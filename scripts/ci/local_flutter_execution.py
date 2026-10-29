"""Run Flutter checks against an isolated source copy, without claiming preflight PASS.

Package and SDK inputs are not yet bound to this diagnostic receipt. This is
not the mandatory local preflight and cannot authorize a PR handoff.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess

import flutter_test_events
import flutter_test_shards
import local_source_snapshot


def _invoke(command, **kwargs):
    return subprocess.run(command, **kwargs)


def classify_suites(test_root, selected):
    """Separate known browser-only files; refuse unknown platform annotations."""
    profiles = {'vm': [], 'chrome': []}
    for name in selected:
        text = (Path(test_root) / name).read_text(encoding='utf-8')
        count = text.count('@TestOn')
        if count == 0:
            profiles['vm'].append(name)
        elif count == 1 and re.match(
                r"\A(?:[ \t]*(?://[^\r\n]*)?\r?\n)*[ \t]*@TestOn\s*\(\s*(['\"])browser\1\s*\)\s*\r?\n[ \t]*library\s*;", text):
            profiles['chrome'].append(name)
        else:
            raise ValueError('unsupported TestOn annotation: ' + name)
    return profiles


def directory_digest(root):
    """Hash all regular files in a private dependency cache, refusing links."""
    root = Path(root)
    if root.is_symlink() or not root.is_dir():
        raise ValueError('dependency cache must be a real directory')
    digest = hashlib.sha256()
    def walk(directory):
        with os.scandir(directory) as entries:
            children = sorted(entries, key=lambda entry: entry.name.encode('utf-8'))
        for entry in children:
            path = Path(entry.path)
            info = path.lstat()
            raw_name = path.relative_to(root).as_posix().encode('utf-8')
            digest.update(len(raw_name).to_bytes(8, 'big'))
            digest.update(raw_name)
            digest.update(stat.S_IMODE(info.st_mode).to_bytes(4, 'big'))
            if stat.S_ISDIR(info.st_mode):
                digest.update(b'D')
                walk(path)
            elif stat.S_ISREG(info.st_mode):
                digest.update(b'F' + info.st_size.to_bytes(8, 'big'))
                with path.open('rb') as stream:
                    for block in iter(lambda: stream.read(1024 * 1024), b''):
                        digest.update(block)
                after = path.stat()
                if (info.st_size, info.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
                    raise ValueError('dependency cache changed during hashing')
            else:
                raise ValueError('dependency cache contains a link or unsupported file')
    walk(root)
    return digest.hexdigest()


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
    if not selected:
        raise ValueError('Flutter test inventory is empty')
    profiles = classify_suites(frontend / 'test', selected)
    result = {'schema': 'voice-local-flutter-diagnostic-v1', 'source': source,
              'status': 'incomplete', 'commands': [], 'tests': None,
              'full_inventory': selected == inventory,
              'unbound_inputs': ['Flutter SDK and external package provenance', 'generated and ambient inputs']}
    tests = []
    if profiles['vm']:
        vm_args = [] if result['full_inventory'] else ['test/' + name for name in profiles['vm']]
        tests.append(('test-vm', [flutter, 'test', '--no-pub', '--reporter', 'json',
                                  '--concurrency=1', *vm_args], profiles['vm']))
    if profiles['chrome']:
        tests.append(('test-chrome', [flutter, 'test', '--no-pub', '--platform', 'chrome',
                                      '--reporter', 'json', '--concurrency=1',
                                      *('test/' + name for name in profiles['chrome'])], profiles['chrome']))
    # Captures and live-test switches are caller data, not this run's outputs.
    env = {key: value for key, value in os.environ.items()
           if not key.upper().startswith('VOICE_') and not key.upper().endswith('_CAPTURE_DIR')}
    env['PUB_CACHE'] = str(output / 'pub-cache')
    Path(env['PUB_CACHE']).mkdir()
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
        commands = [('pub-get', [flutter, 'pub', 'get', '--enforce-lockfile'], []),
                    ('sqlite-prefetch', prefetch, []),
                    ('analyze', [flutter, 'analyze', '--no-fatal-infos', '--no-pub'], []), *tests]
        for label, command, selected_suites in commands:
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
            if label == 'sqlite-prefetch':
                result['pub_cache_sha256'] = directory_digest(env['PUB_CACHE'])
            elif (label == 'analyze' or label.startswith('test-')) and directory_digest(env['PUB_CACHE']) != result['pub_cache_sha256']:
                raise ValueError('dependency cache changed during execution')
            if selected_suites:
                raw = out.read_bytes()
                summary = flutter_test_events.summarize(
                    flutter_test_events.read_events(raw),
                    [str(frontend / 'test' / name) for name in selected_suites], reject_skips=False)
                if result['tests'] is None:
                    result['tests'] = {'tests': 0, 'passed': 0, 'hidden': 0,
                                       'skipped': [], 'suites': [], 'event_sha256': {}}
                for key in ('tests', 'passed', 'hidden'):
                    result['tests'][key] += summary[key]
                result['tests']['skipped'].extend(summary['skipped'])
                result['tests']['suites'].extend(summary['suites'])
                result['tests']['event_sha256'][label] = hashlib.sha256(raw).hexdigest()
        if local_source_snapshot.capture(root, head) != source:
            raise ValueError('source changed during execution')
        if result['tests']['skipped']:
            raise ValueError('required Flutter tests were skipped')
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
