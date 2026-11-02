import io
import hashlib
import os
import shlex
import shutil
import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import local_auth_private as subject


class AuthPrivateTests(unittest.TestCase):
    def report(self, name=subject.CASE, classname=subject.CLASS, counts='tests="1" failures="0" errors="0" skipped="0"', child=''):
        return (f'<testsuite name="{subject.CLASS}" {counts}>'
                f'<testcase name="{name}" classname="{classname}">{child}</testcase></testsuite>').encode()

    def test_exact_case_must_pass_without_skip_or_duplicate(self):
        subject.validate_report(self.report())
        for raw in (self.report(name='other'), self.report(classname='other'),
                    self.report(counts='tests="1" failures="0" errors="0" skipped="1"'),
                    self.report(child='<skipped/>'), self.report(child='<failure/>'),
                    self.report().replace(b'</testsuite>', b'<testcase/></testsuite>'), b'<broken'):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                subject.validate_report(raw)

    def test_cache_archive_rejects_traversal_links_and_duplicates(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'cache.tar'
            for names, link in ((['ok/a'], False), (['../escape'], False), (['ok/a', 'ok/a'], False), (['ok/a'], True)):
                with tarfile.open(path, 'w') as tar:
                    for name in names:
                        info = tarfile.TarInfo(name)
                        if link:
                            info.type, info.linkname = tarfile.SYMTYPE, '../outside'
                            tar.addfile(info)
                        else:
                            info.size = 1
                            tar.addfile(info, io.BytesIO(b'x'))
                if names == ['ok/a'] and not link:
                    self.assertEqual(len(subject.cache_digest(path)), 64)
                else:
                    with self.assertRaises(ValueError):
                        subject.cache_digest(path)

    def test_source_drift_refuses_before_compile(self):
        owner = mock.Mock()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
                 mock.patch.object(subject.snapshot, 'verify_materialized'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value={'head': 'different'}), \
                 self.assertRaisesRegex(ValueError, 'source changed'):
                subject.run_selected(owner, 'daemon', 'client', root, {'head': 'a' * 40}, root / 'copy',
                                     Path(directory) / 'cache.tar', Path(directory) / 'out')
        owner.execute.assert_not_called()

    def test_selected_build_runs_offline_and_returns_incomplete(self):
        owner = mock.Mock()
        def execute(name, command, out, err, timeout, **kwargs):
            if command[0] == 'sha256sum':
                digest = 'd' * 64 if command[1].endswith('build-source.tar') else 'b' * 64
                out.write((digest + '  ' + command[1] + '\n').encode())
            elif command[0] == 'cat':
                out.write(self.report())
            return 0
        owner.execute.side_effect = execute
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            cache = Path(directory) / 'cache'; cache.write_bytes(b'cache')
            source = {'head': 'a' * 40}
            with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
                 mock.patch.object(subject.snapshot, 'verify_materialized'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value=source), \
                 mock.patch.object(subject, 'cache_digest', return_value='b' * 64), \
                 mock.patch.object(subject, 'normalize_archive', side_effect=lambda src, dest, digest: (dest.write_bytes(b'build'), 'd' * 64)[1]), \
                 mock.patch.object(subject.local_go_execution, 'transfer_source', return_value='c' * 64), \
                 mock.patch.object(subject.local_go_execution, '_verify_workspace'), \
                 mock.patch.object(subject, 'verify_build') as verify_build:
                result = subject.run_selected(owner, 'daemon', 'client', root, source, root / 'copy',
                                              cache, Path(directory) / 'out')
        self.assertEqual(result['status'], 'incomplete')
        self.assertIn('cd /workspace/build/src/backend/auth && exec env', result['command'][2])
        self.assertIn('mvn -o -B -ntp', result['command'][2])
        self.assertIn('VOICE_RELEASE_PROFILE=development', result['command'][2])
        executed = [call.args[1] for call in owner.execute.call_args_list if call.args[1] == result['command']]
        self.assertEqual(executed, [result['command']])
        self.assertTrue(result['command'][2].endswith(' test'))
        self.assertTrue(any('test-classes-in-classpath' in call.args[1][2]
                            for call in owner.execute.call_args_list if call.args[1][0] == 'sh'))
        fixture = next(call.args[1][2] for call in owner.execute.call_args_list
                       if call.args[1][0] == 'sh' and 'test-classes-in-classpath' in call.args[1][2])
        self.assertTrue(fixture.startswith('set -eu; '))
        self.assertIn('Maven cache and tool provenance', result['unbound_inputs'])
        self.assertEqual(verify_build.call_count, 2)

    def test_normalized_build_archive_preserves_file_bytes_mode_and_paths(self):
        with tempfile.TemporaryDirectory() as directory:
            source, output = Path(directory) / 'source.tar', Path(directory) / 'build.tar'
            with tarfile.open(source, 'w') as tar:
                info = tarfile.TarInfo('src/test/resources/application-integration.properties')
                info.mode, info.mtime, info.size = 0o644, 0, 7
                tar.addfile(info, io.BytesIO(b'fixture'))
            self.assertEqual(len(subject.normalize_archive(source, output, hashlib.sha256(source.read_bytes()).hexdigest())), 64)
            with tarfile.open(output, 'r:') as tar:
                member = tar.next()
                self.assertEqual(member.name, 'src/test/resources/application-integration.properties')
                self.assertEqual(member.mode, 0o644)
                self.assertEqual(member.mtime, subject.BUILD_MTIME)
                self.assertEqual(tar.extractfile(member).read(), b'fixture')

    def test_normalize_rejects_bad_digest_unsafe_entry_and_changed_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); source = root / 'source.tar'
            with tarfile.open(source, 'w') as tar:
                info = tarfile.TarInfo('src/file'); info.size = 1
                tar.addfile(info, io.BytesIO(b'x'))
            with self.assertRaisesRegex(ValueError, 'source archive'):
                subject.normalize_archive(source, root / 'wrong.tar', '0' * 64)
            digest = hashlib.sha256(source.read_bytes()).hexdigest()
            real_hash = hashlib.file_digest
            output = root / 'changed.tar'
            def change_during_hash(stream, algorithm):
                if getattr(stream, 'name', None) == str(output):
                    with output.open('ab') as changed:
                        changed.write(b'changed')
                return real_hash(stream, algorithm)
            with mock.patch.object(subject.hashlib, 'file_digest', side_effect=change_during_hash):
                with self.assertRaisesRegex(ValueError, 'identity changed'):
                    subject.normalize_archive(source, output, digest)
            unsafe = root / 'unsafe.tar'
            with tarfile.open(unsafe, 'w') as tar:
                info = tarfile.TarInfo('../escape'); info.size = 1
                tar.addfile(info, io.BytesIO(b'x'))
            with self.assertRaisesRegex(ValueError, 'unsafe'):
                subject.normalize_archive(unsafe, root / 'unsafe-output.tar', hashlib.sha256(unsafe.read_bytes()).hexdigest())

    def test_build_source_tampering_fails_closed(self):
        owner = mock.Mock()
        def execute(name, command, out, err, timeout):
            if command[0] == 'sha256sum':
                out.write((('d' * 64) + '  /workspace/build-source.tar\n').encode())
            if command[0] == 'tar':
                return 1  # tar detects a modified captured file
            return 0
        owner.execute.side_effect = execute
        with mock.patch.object(subject.snapshot, '_snapshot_entries', return_value=iter([(b'src/file', [])])):
            with self.assertRaisesRegex(ValueError, 'build source'):
                subject.verify_build(owner, 'client', Path('.'), {}, 'd' * 64)
        self.assertTrue(any(call.args[1][0] == 'tar' for call in owner.execute.call_args_list))

    def test_changed_build_reference_is_rejected_without_trusting_candidate(self):
        owner = mock.Mock()
        def execute(name, command, out, err, timeout):
            if command[0] == 'sha256sum':
                out.write((('e' * 64) + '  /workspace/build-source.tar\n').encode())
            return 0
        owner.execute.side_effect = execute
        with mock.patch.object(subject.snapshot, '_snapshot_entries', return_value=iter([(b'src/file', [])])):
            with self.assertRaisesRegex(ValueError, 'build source'):
                subject.verify_build(owner, 'client', Path('.'), {}, 'd' * 64)
        self.assertFalse(any('candidate.tar' in ' '.join(call.args[1]) for call in owner.execute.call_args_list))

    def test_build_reference_changed_during_comparison_is_rejected(self):
        owner = mock.Mock(); hashes = iter(['d' * 64, 'e' * 64])
        def execute(name, command, out, err, timeout):
            if command[0] == 'sha256sum':
                out.write((next(hashes) + '  /workspace/build-source.tar\n').encode())
            elif command[0] == 'sh' and 'sort -z' in command[2]:
                out.write(b'./src/file\0')
            return 0
        owner.execute.side_effect = execute
        with mock.patch.object(subject.snapshot, '_snapshot_entries', return_value=iter([(b'src/file', [])])):
            with self.assertRaisesRegex(ValueError, 'build source changed'):
                subject.verify_build(owner, 'client', Path('.'), {}, 'd' * 64)

    def test_fixture_script_fails_on_missing_or_changed_resource(self):
        shell = shutil.which('sh')
        if shell is None and os.name == 'nt':
            shell = r'C:\Program Files\Git\bin\bash.exe'
        if not shell or not Path(shell).is_file():
            self.skipTest('POSIX shell unavailable')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            original = root / 'src/test/resources/application-integration.properties'
            copied = root / 'target/test-classes/application-integration.properties'
            original.parent.mkdir(parents=True)
            copied.parent.mkdir(parents=True)
            original.write_text('fixture')
            script = subject.fixture_script().replace('cd /workspace/build/src/backend/auth', 'cd ' + shlex.quote(root.as_posix()))
            missing = subprocess.run([shell, '-c', script], capture_output=True, timeout=15)
            self.assertNotEqual(missing.returncode, 0)
            self.assertIn(b'test-fixture=missing', missing.stdout)
            copied.write_text('changed')
            changed = subprocess.run([shell, '-c', script], capture_output=True, timeout=15)
            self.assertNotEqual(changed.returncode, 0)
            self.assertIn(b'fixture-compare=mismatch-or-error', changed.stdout)
            copied.write_text('fixture')
            matched = subprocess.run([shell, '-c', script], capture_output=True, timeout=15)
            self.assertEqual(matched.returncode, 0)
            self.assertIn(b'fixture-compare=match', matched.stdout)
            reports = root / 'target/surefire-reports'; reports.mkdir()
            report = reports / ('TEST-' + subject.CLASS + '.xml')
            report.write_text('<property name="java.class.path" value="/workspace/build/src/backend/auth/target/test-classes:/other"/>')
            self.assertIn(b'test-classes-in-classpath=yes', subprocess.run(
                [shell, '-c', script], capture_output=True, timeout=15).stdout)
            for near in ('/prefix/workspace/build/src/backend/auth/target/test-classes',
                         '/workspace/build/src/backend/auth/target/test-classes-extra'):
                report.write_text('<property name="java.class.path" value="' + near + '"/>')
                self.assertIn(b'test-classes-in-classpath=no-or-unparsed', subprocess.run(
                    [shell, '-c', script], capture_output=True, timeout=15).stdout)
            report.unlink()
            broken = subprocess.run([shell, '-c', 'cmp() { return 2; }; ' + script],
                                    capture_output=True, timeout=15)
            self.assertNotEqual(broken.returncode, 0)
            self.assertIn(b'fixture-compare=mismatch-or-error', broken.stdout)


if __name__ == '__main__':
    unittest.main()
