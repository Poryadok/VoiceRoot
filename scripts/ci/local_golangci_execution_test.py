import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import local_golangci_execution as lint
import local_golangci_capture as bounded_capture


class LocalGolangciExecutionTests(unittest.TestCase):
    def test_reader_failure_kills_child_and_never_returns_success(self):
        child = mock.Mock()
        child.stdout.read.side_effect = OSError('broken pipe')
        child.stderr.read.return_value = b''
        child.wait.return_value = 0
        with mock.patch.object(bounded_capture.subprocess, 'Popen', return_value=child), \
             mock.patch.object(bounded_capture, 'sys') as fake_sys:
            self.assertEqual(bounded_capture.main(['fake']), 126)
        child.kill.assert_called_once()
        fake_sys.stdout.buffer.write.assert_called_once()

    def test_capture_bounds_each_stream_and_propagates_failure(self):
        wrapper = Path(lint.__file__).with_name('local_golangci_capture.py')
        for stream in ('stdout', 'stderr'):
            code = 'import sys; sys.' + stream + '.buffer.write(b"x" * 1200000)'
            result = subprocess.run([sys.executable, '-B', str(wrapper), sys.executable, '-c', code],
                                    capture_output=True, timeout=30)
            self.assertEqual(result.returncode, 125)
            self.assertLessEqual(len(result.stdout), 1024 * 1024)
            self.assertLessEqual(len(result.stderr), 1024 * 1024)
        result = subprocess.run([sys.executable, '-B', str(wrapper), sys.executable, '-c',
                                 'raise SystemExit(7)'], capture_output=True, timeout=30)
        self.assertEqual(result.returncode, 7)

    def test_modules_are_planner_ordered_deduplicated_and_include_pkg(self):
        self.assertEqual(lint.modules_for({'components': {'go': ['chat', 'pkg', 'user', 'chat']}}),
                         ['pkg', 'chat', 'user'])

    def test_rejects_path_or_missing_inventory(self):
        for value in ({}, {'components': {'go': ['../user']}}, {'components': {'go': 'chat'}}):
            with self.subTest(value=value), self.assertRaises(ValueError):
                lint.modules_for(value)

    def test_fails_on_mismatched_tool_version_and_digest(self):
        with mock.patch.object(lint, '_sha256', return_value='a' * 64), \
             mock.patch.object(lint, '_command', return_value=(b'golangci-lint has version 2.6.0\n', b'')):
            with self.assertRaises(ValueError):
                lint._check_tool('C:/lint.exe', 'a' * 64, 'lint')
        with mock.patch.object(lint, '_sha256', return_value='b' * 64):
            with self.assertRaises(ValueError):
                lint._check_tool('C:/lint.exe', 'a' * 64, 'lint')

    def test_commands_use_exact_ci_order_and_stop_on_failure(self):
        calls = []
        def run(command, **kwargs):
            calls.append((command, kwargs))
            if len(calls) == 3:
                raise subprocess.CalledProcessError(1, command)
            return b'', b''
        with mock.patch.object(lint, '_command', side_effect=run):
            with self.assertRaises(subprocess.CalledProcessError):
                lint._run_modules('C:/source', ['pkg', 'chat'], 'C:/go.exe', 'C:/lint.exe', {})
        self.assertEqual([item[0][1:] for item in calls],
                         [['list', '-test', './...'], ['list', '-test', './...'], ['run', './...']])

    def test_source_or_tool_change_stops_later_commands(self):
        with mock.patch.object(lint, '_command', return_value=(b'ok', b'')) as command:
            with self.assertRaisesRegex(ValueError, 'changed'):
                lint._run_modules('C:/source', ['pkg', 'chat'], 'C:/go.exe', 'C:/lint.exe', {},
                                  lambda *_: (_ for _ in ()).throw(ValueError('source changed')))
        self.assertEqual(command.call_count, 1)

    def test_missing_or_unselected_plan_refuses_before_snapshot(self):
        with tempfile.TemporaryDirectory() as directory, \
             mock.patch.object(lint, '_windows', return_value=True), \
             mock.patch.object(lint.local_source_snapshot, 'capture') as capture:
            root = Path(directory)
            with self.assertRaisesRegex(ValueError, 'planner-selected'):
                lint.run(root / 'source', {'checks': {}, 'source': {}}, root / 'out',
                         root / 'go', 'a' * 64, root / 'lint', 'b' * 64, base='c' * 40, head_ref='test')
        capture.assert_not_called()

    def test_forged_plan_refuses_before_tool_or_snapshot(self):
        with tempfile.TemporaryDirectory() as directory, \
             mock.patch.object(lint, '_windows', return_value=True), \
             mock.patch.object(lint.affected_check_plan, 'build_local_plan', return_value={}), \
             mock.patch.object(lint.local_source_snapshot, 'capture') as capture, \
             mock.patch.object(lint, '_check_tool') as tool:
            root = Path(directory)
            plan = {'source': {'head': 'a' * 40}, 'checks': {'golangci': []}, 'components': {'go': []}}
            with self.assertRaisesRegex(ValueError, 'plan differs'):
                lint.run(root / 'source', plan, root / 'out', root / 'go', 'a' * 64,
                         root / 'lint', 'b' * 64, base='c' * 40, head_ref='test')
        capture.assert_not_called()
        tool.assert_not_called()


if __name__ == '__main__':
    unittest.main()
