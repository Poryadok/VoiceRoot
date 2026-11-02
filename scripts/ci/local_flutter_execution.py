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
import local_process_supervision as supervision
import local_source_snapshot


# These Flutter-generated outputs remain explicitly unbound diagnostic inputs.
# Source files outside these exact paths may not appear during a run.
FLUTTER_OUTPUTS = (
    'src/frontend/.dart_tool/', 'src/frontend/build/',
    'src/frontend/linux/flutter/ephemeral/', 'src/frontend/windows/flutter/ephemeral/',
    'src/frontend/macos/Flutter/ephemeral/', 'src/frontend/ios/Flutter/ephemeral/',
    'src/frontend/.flutter-plugins', 'src/frontend/.flutter-plugins-dependencies',
    'src/frontend/android/local.properties',
    'src/frontend/android/app/src/main/java/io/flutter/plugins/GeneratedPluginRegistrant.java',
    'src/frontend/ios/Flutter/Generated.xcconfig', 'src/frontend/ios/Flutter/flutter_export_environment.sh',
    'src/frontend/ios/Runner/GeneratedPluginRegistrant.h', 'src/frontend/ios/Runner/GeneratedPluginRegistrant.m',
    'src/frontend/sqlite3mc.x64.windows.dll', 'src/frontend/libsqlite3mc.x64.linux.so',
    'src/frontend/libsqlite3mc.arm64.linux.so', 'src/frontend/libsqlite3mc.arm64.macos.dylib',
    'src/frontend/libsqlite3mc.x64.macos.dylib',
    # The key-backup test deliberately compares a changed image and writes
    # these diagnostic PNGs even when the negative assertion succeeds.
    'src/frontend/test/failures/e2e_key_backup_h_isolatedDiff.png',
    'src/frontend/test/failures/e2e_key_backup_h_maskedDiff.png',
    'src/frontend/test/failures/e2e_key_backup_h_masterImage.png',
    'src/frontend/test/failures/e2e_key_backup_h_testImage.png',
    'tmp/anton122-accessibility/accessibility-H.png',
    'tmp/anton122-accessibility/accessibility-V.png',
)


FLUTTER_COMMAND_TIMEOUT = 1800


def _invoke(command, **kwargs):
    return supervision.run_diagnostic(command, timeout=FLUTTER_COMMAND_TIMEOUT, **kwargs)


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


FLUTTER_COMMAND_LIMIT = 6000


def suite_batches(flutter, suites, platform):
    """Keep explicit suite invocations below the Windows batch-command limit."""
    if platform not in ('vm', 'chrome') or not suites:
        raise ValueError('invalid Flutter suite batch')
    batches, pending = [], []
    def command(names):
        prefix = [flutter, 'test', '--no-pub']
        if platform == 'chrome':
            prefix += ['--platform', 'chrome']
        prefix += ['--reporter', 'json']
        if platform == 'vm':
            prefix += ['--concurrency=1']
            if 'accessibility_settings_capture_test.dart' in names:
                prefix += ['--dart-define=VOICE_ACCESSIBILITY_CAPTURE=true']
        return prefix + ['test/' + name for name in names]
    def fits(names):
        return len(subprocess.list2cmdline(command(names)).encode('utf-16-le')) // 2 <= FLUTTER_COMMAND_LIMIT
    for name in suites:
        if not fits(pending + [name]):
            if not pending or not fits([name]):
                raise ValueError('Flutter suite path exceeds command limit')
            batches.append(pending)
            pending = []
        pending.append(name)
    if pending:
        batches.append(pending)
    return [('test-' + platform + (('-' + str(index + 1).zfill(3)) if len(batches) > 1 else ''),
             command(batch), batch) for index, batch in enumerate(batches)]


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


def classify_skips(checkout, skipped):
    """Record catalog associations; they never waive a skipped test."""
    try:
        import e2e_manifest
    except SystemExit as error:
        raise ValueError('E2E catalog parser dependency is unavailable') from error

    checkout = Path(checkout).resolve()
    frontend = checkout / 'src/frontend'
    catalog = checkout / '.github/ci/e2e-features.yml'
    sections = ('smoke_flutter', 'full_flutter', 'a1_flutter_profile_handoff')
    try:
        selections = {section: set(e2e_manifest.selection(catalog, section)) for section in sections}
    except (OSError, UnicodeError, ValueError, TypeError, RecursionError, e2e_manifest.yaml.YAMLError) as error:
        raise ValueError('invalid source E2E catalog: ' + str(error)) from error
    classified = []
    for item in skipped:
        if not isinstance(item, dict) or set(item) != {'suite', 'name'} or not isinstance(item['name'], str):
            raise ValueError('invalid skipped Flutter test identity')
        try:
            suite = Path(item['suite']).resolve().relative_to(frontend).as_posix()
        except (KeyError, TypeError, ValueError):
            raise ValueError('skipped Flutter suite is outside the source snapshot') from None
        if not suite.startswith('test/'):
            raise ValueError('skipped Flutter suite is outside the test inventory')
        classified.append({'suite': suite, 'name': item['name'],
                           'catalog_sections': [section for section in sections if suite in selections[section]]})
    return sorted(classified, key=lambda item: (item['suite'], item['name']))


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
        tests.extend(suite_batches(flutter, profiles['vm'], 'vm'))
    if profiles['chrome']:
        tests.extend(suite_batches(flutter, profiles['chrome'], 'chrome'))
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
            local_source_snapshot.verify_materialized(root, source, checkout, allowed_extra=FLUTTER_OUTPUTS)
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
                if label.startswith('test-vm') and 'accessibility_settings_capture_test.dart' in selected_suites:
                    images = {}
                    for orientation in ('H', 'V'):
                        name = 'tmp/anton122-accessibility/accessibility-' + orientation + '.png'
                        raw_image = (checkout / name).read_bytes()
                        if not raw_image.startswith(b'\x89PNG\r\n\x1a\n'):
                            raise ValueError('accessibility capture is missing or invalid')
                        images[name] = hashlib.sha256(raw_image).hexdigest()
                    result['accessibility_captures'] = images
        if local_source_snapshot.capture(root, head) != source:
            raise ValueError('source changed during execution')
        if result['tests']['skipped']:
            result['skip_classification'] = classify_skips(checkout, result['tests']['skipped'])
            raise ValueError('required Flutter tests were skipped')
    except (OSError, ValueError, json.JSONDecodeError, subprocess.TimeoutExpired,
            supervision.SupervisionError, ExceptionGroup) as error:
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
