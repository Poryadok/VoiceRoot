"""Run Flutter checks against an isolated source copy, without claiming preflight PASS.

Package and SDK inputs are not yet bound to this diagnostic receipt. This is
not the mandatory local preflight and cannot authorize a PR handoff.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

import flutter_test_events
import flutter_test_shards
import local_source_snapshot


def _invoke(command, **kwargs):
    return subprocess.run(command, **kwargs)


def run(root, head, output, flutter, selected=None):
    root = Path(root).resolve()
    output = Path(output).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('output must be a new directory outside the source tree')
    source = local_source_snapshot.capture(root, head)
    output.mkdir(parents=True, exist_ok=False)
    checkout = local_source_snapshot.materialize(root, source, output / 'source')
    frontend = checkout / 'src/frontend'
    inventory = flutter_test_shards.discover(frontend / 'test')
    if selected is not None and (not selected or len(set(selected)) != len(selected)
                                 or not set(selected).issubset(inventory)):
        raise ValueError('selected suites must be distinct discovered test paths')
    selected = inventory if selected is None else selected
    suites = [str(frontend / 'test' / name) for name in selected]
    if not suites:
        raise ValueError('Flutter test inventory is empty')
    result = {'schema': 'voice-local-flutter-diagnostic-v1', 'source': source,
              'status': 'incomplete', 'commands': [], 'tests': None,
              'full_inventory': selected == inventory,
              'unbound_inputs': ['Flutter SDK and external packages', 'generated and ambient inputs']}
    test_args = [] if result['full_inventory'] else ['test/' + name for name in selected]
    env = os.environ.copy()
    env['PUB_CACHE'] = str(output / 'pub-cache')
    private_temp = output / 'temp'
    private_temp.mkdir()
    env.update(TEMP=str(private_temp), TMP=str(private_temp), TMPDIR=str(private_temp))
    if Path(flutter).is_absolute():
        env['PATH'] = str(Path(flutter).parent) + os.pathsep + env.get('PATH', '')
    try:
        version = _invoke([flutter, '--version', '--machine'], cwd=frontend, env=env,
                          capture_output=True, check=False)
        if version.returncode != 0:
            raise ValueError('Flutter version command failed')
        result['toolchain'] = json.loads(version.stdout)
        captures = output / 'music-captures'
        captures.mkdir()
        env['VOICE_MUSIC_CAPTURE_DIR'] = str(captures)
        if result['toolchain'].get('flutterRoot'):
            env['FLUTTER_ROOT'] = result['toolchain']['flutterRoot']
        prefetch = (['powershell', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File',
                     str(checkout / 'scripts/ci/flutter-windows-prefetch-sqlite3.ps1'), '-RootDir', str(checkout)]
                    if os.name == 'nt' else ['bash', str(checkout / 'scripts/ci/flutter-linux-prefetch-sqlite3.sh')])
        for label, command in [('pub-get', [flutter, 'pub', 'get', '--enforce-lockfile']),
                               ('sqlite-prefetch', prefetch),
                               ('analyze', [flutter, 'analyze', '--no-fatal-infos']),
                               ('test', [flutter, 'test', '--no-pub', '--reporter', 'json',
                                         '--concurrency=1', *test_args])]:
            out = output / (label + '.stdout')
            err = output / (label + '.stderr')
            with out.open('wb') as stdout, err.open('wb') as stderr:
                completed = _invoke(command, cwd=frontend, env=env, stdout=stdout,
                                    stderr=stderr, check=False)
            result['commands'].append({'argv': command, 'exit_code': completed.returncode,
                                       'stdout': out.name, 'stderr': err.name})
            if completed.returncode != 0:
                raise ValueError(label + ' failed')
            local_source_snapshot.verify_materialized(root, source, checkout)
            if label == 'test':
                raw = out.read_bytes()
                result['tests'] = flutter_test_events.summarize(flutter_test_events.read_events(raw), suites)
                result['tests']['event_sha256'] = hashlib.sha256(raw).hexdigest()
        if local_source_snapshot.capture(root, head) != source:
            raise ValueError('source changed during execution')
    except (OSError, ValueError, json.JSONDecodeError) as error:
        result['status'] = 'failed'
        result['error'] = str(error)
    (output / 'diagnostic.json').write_text(json.dumps(result, indent=2, sort_keys=True), encoding='utf-8')
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--head', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--flutter', default='flutter')
    parser.add_argument('--suite', action='append', help='test path relative to test/; default is full inventory')
    args = parser.parse_args()
    result = run(Path(__file__).resolve().parents[2], args.head, args.output, args.flutter, args.suite)
    print(json.dumps(result, sort_keys=True))
    # Incomplete diagnostics must not be accepted as a successful preflight.
    return 1 if result['status'] == 'failed' else 2


if __name__ == '__main__':
    raise SystemExit(main())
