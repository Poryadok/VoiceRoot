"""Disposable Git tests for the local current-diff diagnostic."""
import json
import os
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path
from unittest import mock

import local_current_diff
import local_source_snapshot


class BoundedCurrentDiffProcessTest(unittest.TestCase):
    def test_windows_routes_input_and_timeout_through_job_broker(self):
        completed = subprocess.CompletedProcess([], 0, b'output', b'')
        with mock.patch.object(local_current_diff, '_windows', return_value=True), \
             mock.patch.object(local_current_diff.supervision, 'run_diagnostic', return_value=completed) as broker:
            result = local_current_diff._execute(['git', 'commit-tree', 'tree'], input=b'message', timeout=30)
        self.assertEqual(result.stdout, b'output')
        self.assertEqual(broker.call_args.kwargs['input'], b'message')
        self.assertEqual(broker.call_args.kwargs['timeout'], 30)
        self.assertIn('local_golangci_capture.py', broker.call_args.args[0][2])

    def test_oversized_output_fails_closed(self):
        with mock.patch.object(local_current_diff, '_windows', return_value=False):
            result = local_current_diff._execute([sys.executable, '-c',
                'import sys; sys.stdout.buffer.write(b"x" * 1200000)'], timeout=30)
        self.assertEqual(result.returncode, 125)
        self.assertLessEqual(len(result.stdout), 1024 * 1024)

    def test_failure_is_not_treated_as_success(self):
        with mock.patch.object(local_current_diff, '_windows', return_value=False):
            result = local_current_diff._execute([sys.executable, '-c', 'raise SystemExit(7)'], timeout=30)
        self.assertEqual(result.returncode, 7)

    def test_timed_out_command_cannot_leave_descendant_running(self):
        if os.name != 'nt':
            with mock.patch.object(local_current_diff, '_windows', return_value=True), \
                 mock.patch.object(local_current_diff.supervision, 'run_diagnostic',
                                   side_effect=subprocess.TimeoutExpired('worker', 1)):
                with self.assertRaises(subprocess.TimeoutExpired):
                    local_current_diff._execute([sys.executable, '-c', 'pass'], timeout=1)
            return
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / 'survived'
            ready = Path(directory) / 'ready'
            grandchild = ('import pathlib,sys,time; pathlib.Path(sys.argv[2]).write_text("ready"); '
                          'time.sleep(8); pathlib.Path(sys.argv[1]).write_text("alive")')
            child = ('import pathlib,subprocess,sys,time; '
                     'subprocess.Popen([sys.executable,"-c",sys.argv[1],sys.argv[2],sys.argv[3]]); '
                     'p=pathlib.Path(sys.argv[3]); '
                     'exec("while not p.exists(): time.sleep(0.02)"); time.sleep(30)')
            with self.assertRaises(subprocess.TimeoutExpired):
                local_current_diff._execute([sys.executable, '-c', child, grandchild, str(marker), str(ready)], timeout=5)
            self.assertTrue(ready.exists())
            time.sleep(4)
            self.assertFalse(marker.exists())


class CurrentDiffDiagnosticTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'repo'
        self.root.mkdir()
        self.output = Path(self.tmp.name) / 'output'
        self.git('init', '-q')
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')
        script = self.root / 'scripts/ci/current-diff-contract.py'
        script.parent.mkdir(parents=True)
        script.write_bytes((Path(__file__).parent / 'current-diff-contract.py').read_bytes())
        self.migration = self.root / 'src/backend/migrations/chat/001_init.sql'
        self.migration.parent.mkdir(parents=True)
        self.migration.write_text('SELECT 1;\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'base')
        self.head = self.git('rev-parse', 'HEAD').decode().strip()

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE)

    def test_source_bound_snapshot_runs_canonical_validator(self):
        (self.root / 'new.txt').write_text('new\n')
        source = local_source_snapshot.capture(self.root, self.head)
        report = local_current_diff.run(self.root, source, self.head, self.output)
        self.assertEqual(report['status'], 'diagnostic')
        self.assertEqual(report['source'], source)
        self.assertIn('new.txt', report['contract']['changed_paths'])
        self.assertEqual(self.git('rev-parse', 'HEAD').decode().strip(), self.head)
        self.assertEqual(json.loads((self.output / 'diagnostic.json').read_text()), report)

    def test_modified_existing_migration_fails(self):
        self.migration.write_text('SELECT 2;\n')
        source = local_source_snapshot.capture(self.root, self.head)
        with self.assertRaisesRegex(ValueError, 'current-diff-contract'):
            local_current_diff.run(self.root, source, self.head, self.output)

    def test_changed_source_after_capture_is_rejected(self):
        source = local_source_snapshot.capture(self.root, self.head)
        (self.root / 'later.txt').write_text('later\n')
        with self.assertRaisesRegex(ValueError, 'source changed'):
            local_current_diff.run(self.root, source, self.head, self.output)


if __name__ == '__main__':
    unittest.main()
