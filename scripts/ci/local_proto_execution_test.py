import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

import local_proto_execution as proto


class LocalProtoExecutionTests(unittest.TestCase):
    def test_command_uses_job_broker_and_propagates_timeout(self):
        result = subprocess.CompletedProcess([], 0, b'\x00\xff', b'')
        with mock.patch.object(proto, '_windows', return_value=True), \
             mock.patch.object(proto.supervision, 'run_diagnostic', return_value=result) as broker:
            actual = proto._execute(['git', 'archive'], timeout=120)
        self.assertEqual(actual.stdout, b'\x00\xff')
        self.assertEqual(broker.call_args.kwargs['timeout'], 120)
        self.assertIn('--limit=16777216', broker.call_args.args[0])
        with mock.patch.object(proto, '_windows', return_value=True), \
             mock.patch.object(proto.supervision, 'run_diagnostic', side_effect=subprocess.TimeoutExpired('worker', 1)):
            with self.assertRaises(subprocess.TimeoutExpired):
                proto._execute(['git', 'archive'], timeout=1)

    def test_binary_archive_output_is_preserved_and_oversize_rejected(self):
        binary = 'import sys; sys.stdout.buffer.write(b"\\x00\\xff" * 700000)'
        result = proto._execute([sys.executable, '-c', binary], timeout=30)
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b'\x00\xff' * 700000)
        excess = 'import sys; sys.stdout.buffer.write(b"x" * 17000000)'
        result = proto._execute([sys.executable, '-c', excess], timeout=30)
        self.assertEqual(result.returncode, 125)
        self.assertLessEqual(len(result.stdout), 16777216)

    def test_private_maven_options_allow_spaces_but_reject_unsafe_characters(self):
        options = proto.maven_options(Path('directory with spaces') / 'repository')
        self.assertIn('directory with spaces', options)
        self.assertTrue(options.startswith('-Dmaven.repo.local="'))
        self.assertTrue(options.endswith('"'))
        with self.assertRaises(ValueError):
            proto.maven_options('invalid"path')
        with self.assertRaises(ValueError):
            proto.maven_options('invalid\npath')

    def test_java_consumer_requires_named_pass_and_no_skips(self):
        package = 'voice/r23-contract-test'
        name = 'TestR23GeneratedRuntimeConsumers/Java'
        def stream(action):
            events = [{'Action': 'start', 'Package': package},
                      {'Action': 'run', 'Package': package, 'Test': name},
                      {'Action': action, 'Package': package, 'Test': name},
                      {'Action': 'pass', 'Package': package}]
            return ''.join(json.dumps(item) + '\n' for item in events).encode()
        self.assertEqual(proto.r23_java_summary(stream('pass'), 0)['skipped'], [])
        with self.assertRaises(ValueError):
            proto.r23_java_summary(stream('skip'), 0)
        with self.assertRaises(ValueError):
            proto.r23_java_summary(stream('pass').replace(name.encode(), b'Other'), 0)

    def test_r23_event_validation_records_skip_and_rejects_partial_or_failure(self):
        package = 'voice/r23-contract-test'
        events = [
            {'Action': 'start', 'Package': package},
            {'Action': 'run', 'Package': package, 'Test': 'TestGo'},
            {'Action': 'pass', 'Package': package, 'Test': 'TestGo'},
            {'Action': 'run', 'Package': package, 'Test': 'TestJava'},
            {'Action': 'skip', 'Package': package, 'Test': 'TestJava'},
            {'Action': 'pass', 'Package': package},
        ]
        raw = ''.join(json.dumps(item) + '\n' for item in events).encode()
        result = proto.r23_summary(raw, 0)
        self.assertEqual(result['passed'], 1)
        self.assertEqual(result['skipped'], [[package, 'TestJava']])
        with self.assertRaises(ValueError):
            proto.r23_summary(raw, 1)
        with self.assertRaises(ValueError):
            proto.r23_summary(raw.splitlines(keepends=True)[0], 0)
        with self.assertRaises(ValueError):
            proto.r23_summary(raw + b'{', 0)

    def test_baseline_archive_uses_the_immutable_commit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'repo'
            root.mkdir()
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()
            git('init', '-q')
            (root / 'protos').mkdir()
            file = root / 'protos' / 'buf.yaml'
            file.write_text('old\n', encoding='utf-8')
            git('add', 'protos')
            git('-c', 'user.name=CI', '-c', 'user.email=ci@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture')
            base = git('rev-parse', 'HEAD')
            file.write_text('new\n', encoding='utf-8')
            out = Path(directory) / 'baseline'
            proto.archive_baseline(root, base, out)
            self.assertEqual((out / 'protos/buf.yaml').read_text(encoding='utf-8'), 'old\n')
            with self.assertRaises(ValueError):
                proto.archive_baseline(root, 'not-a-commit', Path(directory) / 'invalid')


if __name__ == '__main__':
    unittest.main()
