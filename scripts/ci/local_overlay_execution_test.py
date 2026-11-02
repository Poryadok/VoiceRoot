import unittest
import tempfile
import subprocess
import hashlib
from pathlib import Path
from unittest import mock

import local_overlay_execution as overlay


class OverlayDiagnosticTests(unittest.TestCase):
    def test_pinned_kubectl_is_copied_and_verified_before_offline_tests(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = (Path(directory) / 'kubectl').resolve(); binary.write_bytes(b'fixture binary')
            digest = hashlib.sha256(binary.read_bytes()).hexdigest()
            owner = mock.Mock()
            def execute(name, command, out, err, timeout, stdin=None):
                if stdin is not None:
                    self.assertEqual(stdin.read(), b'fixture binary')
                else:
                    out.write((digest + '  /workspace/tools/kubectl\n').encode())
                return 0
            owner.execute.side_effect = execute
            with mock.patch.object(overlay, 'KUBECTL_SHA256', digest):
                overlay.provision_kubectl(owner, 'loader', binary)
            self.assertEqual(owner.execute.call_count, 2)
            self.assertIn('/workspace/tools/kubectl', owner.execute.call_args.args[1][2])
            owner.reset_mock(); binary.write_bytes(b'tampered')
            with mock.patch.object(overlay, 'KUBECTL_SHA256', digest), self.assertRaises(ValueError):
                overlay.provision_kubectl(owner, 'loader', binary)
            owner.execute.assert_not_called()

    def test_summary_requires_exact_positive_discovered_count(self):
        self.assertEqual(overlay.validate_summary(b'...\n----------------------------------------------------------------------\nRan 3 tests in 0.123s\n\nOK\n', 0, 3), 3)
        for output, code, count in ((b'Ran 3 tests in 1s\n\nOK\n', 1, 3),
                                    (b'Ran 3 tests in 1s\n\nOK (skipped=1)\n', 0, 3),
                                    (b'Ran 3 tests in 1s\n\nOK\n', 0, 4),
                                    (b'Ran 0 tests in 1s\n\nOK\n', 0, 0)):
            with self.subTest(output=output), self.assertRaises(ValueError):
                overlay.validate_summary(output, code, count)

    def test_discovery_output_requires_one_positive_marker(self):
        self.assertEqual(overlay.validate_discovery(b'VOICE_OVERLAY_DISCOVERED=4\n'), 4)
        for output in (b'', b'VOICE_OVERLAY_DISCOVERED=0\n',
                       b'VOICE_OVERLAY_DISCOVERED=1\nVOICE_OVERLAY_DISCOVERED=1\n'):
            with self.subTest(output=output), self.assertRaises(ValueError):
                overlay.validate_discovery(output)

    def test_plan_mismatch_prevents_source_capture_and_worker(self):
        plan = {'source': {'head': 'a' * 40}, 'checks': {'match-squad-overlay': []}}
        with mock.patch.object(overlay, '_windows', return_value=True), \
             mock.patch.object(overlay.auth_broker, '_require_worker_job'), \
             mock.patch.object(overlay.affected_check_plan, 'build_local_plan', return_value={}), \
             mock.patch.object(overlay.local_source_snapshot, 'capture') as capture:
            with self.assertRaisesRegex(ValueError, 'plan differs'):
                overlay.run('C:/source', plan, 'C:/output', overlay.PYTHON_IMAGE, 'kubectl',
                            base='c' * 40, head_ref='test')
        capture.assert_not_called()

    def test_unfenced_call_refuses_before_engine_creation(self):
        with mock.patch.object(overlay.auth_broker, '_require_worker_job', side_effect=ValueError('no job')), \
             mock.patch.object(overlay, 'DockerCLI') as engine:
            with self.assertRaisesRegex(ValueError, 'no job'):
                overlay.run('C:/source', {}, 'C:/output', overlay.PYTHON_IMAGE, 'kubectl', base='c' * 40, head_ref='test')
        engine.assert_not_called()

    def test_owned_linux_client_runs_exact_inventory_and_cleans_up(self):
        with tempfile.TemporaryDirectory() as directory:
            root, output = Path(directory) / 'root', Path(directory) / 'out'
            root.mkdir()
            source = {'head': 'a' * 40}
            plan = {'source': source, 'checks': {'match-squad-overlay': []}}
            owner, engine, scheduled = mock.Mock(), mock.Mock(), mock.Mock()
            scheduled.run, scheduled.slot = owner, 0
            owner.directory = output / 'run' / 'tmp' / 'ci-local' / 'owned'
            owner.create_resource.side_effect = ['network', 'volume']
            owner.create_container.return_value = 'loader'
            owner.create_phase0_container.return_value = 'isolated'
            engine._call.return_value = 'linux/amd64'
            outputs = [(0, (overlay.KUBECTL_SHA256 + '  /workspace/repo/tools/kubectl\n').encode(), b''),
                       (0, b'Python 3.12.7\n', b''), (0, b'VOICE_OVERLAY_DISCOVERED=2\n', b''),
                       (0, b'', b'..\nRan 2 tests in 0.100s\n\nOK\n')]
            with mock.patch.object(overlay.auth_broker, '_require_worker_job'), \
                 mock.patch.object(overlay, '_windows', return_value=True), \
                 mock.patch.object(overlay.affected_check_plan, 'build_local_plan', return_value=plan), \
                 mock.patch.object(overlay.local_source_snapshot, 'capture', return_value=source), \
                 mock.patch.object(overlay.local_source_snapshot, 'materialize', return_value=output / 'source'), \
                 mock.patch.object(overlay, 'DockerCLI', return_value=engine), \
                 mock.patch.object(overlay.ScheduledRun, 'start_host', return_value=scheduled), \
                 mock.patch.object(overlay.local_go_execution, 'transfer_source', return_value='d' * 64), \
                 mock.patch.object(overlay.orchestration, '_verify_file'), \
                 mock.patch.object(overlay, 'provision_kubectl'), \
                 mock.patch.object(overlay.local_go_execution, '_verify_workspace') as verify, \
                 mock.patch.object(overlay, '_command', side_effect=outputs) as command:
                result = overlay.run(root, plan, output, overlay.PYTHON_IMAGE, 'kubectl', base='b' * 40, head_ref='test')
            owner.create_phase0_container.assert_called_once_with(overlay.PYTHON_IMAGE, 'volume')
            verify.assert_called_once()
            scheduled.close.assert_called_once()
            self.assertEqual(result['tests'], {'discovered': 2, 'passed': 2, 'skipped': []})
            self.assertEqual(result['status'], 'incomplete')
            self.assertIn('-m unittest scripts.dev.match_squad_overlay_test scripts.dev.match_squad_runtime_overlay_test',
                          command.call_args_list[3].args[4])

    def test_skipped_owned_test_still_cleans_up(self):
        with tempfile.TemporaryDirectory() as directory:
            root, output = Path(directory) / 'root', Path(directory) / 'out'
            root.mkdir()
            source = {'head': 'a' * 40}
            plan = {'source': source, 'checks': {'match-squad-overlay': []}}
            owner, engine, scheduled = mock.Mock(), mock.Mock(), mock.Mock()
            scheduled.run, scheduled.slot = owner, 0
            owner.create_resource.side_effect = ['network', 'volume']
            engine._call.return_value = 'linux/amd64'
            outputs = [(0, (overlay.KUBECTL_SHA256 + '  /workspace/repo/tools/kubectl\n').encode(), b''),
                       (0, b'Python 3.12.7\n', b''), (0, b'VOICE_OVERLAY_DISCOVERED=2\n', b''),
                       (0, b'', b'..\nRan 2 tests in 0.100s\n\nOK (skipped=1)\n')]
            with mock.patch.object(overlay.auth_broker, '_require_worker_job'), \
                 mock.patch.object(overlay, '_windows', return_value=True), \
                 mock.patch.object(overlay.affected_check_plan, 'build_local_plan', return_value=plan), \
                 mock.patch.object(overlay.local_source_snapshot, 'capture', return_value=source), \
                 mock.patch.object(overlay.local_source_snapshot, 'materialize', return_value=output / 'source'), \
                 mock.patch.object(overlay, 'DockerCLI', return_value=engine), \
                 mock.patch.object(overlay.ScheduledRun, 'start_host', return_value=scheduled), \
                 mock.patch.object(overlay.local_go_execution, 'transfer_source', return_value='d' * 64), \
                 mock.patch.object(overlay.orchestration, '_verify_file'), \
                 mock.patch.object(overlay, 'provision_kubectl'), \
                 mock.patch.object(overlay, '_command', side_effect=outputs):
                with self.assertRaisesRegex(ValueError, 'skipped'):
                    overlay.run(root, plan, output, overlay.PYTHON_IMAGE, 'kubectl', base='b' * 40, head_ref='test')
            scheduled.close.assert_called_once()
            self.assertFalse((output / 'diagnostic.json').exists())

    def test_supervised_path_rejects_forged_report_without_running_inline(self):
        plan = {'source': {'head': 'a' * 40}}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'out'
            result = subprocess.CompletedProcess([], 0, stdout=b'{"status":"pass"}', stderr=b'')
            with mock.patch.object(overlay.supervision, 'run_diagnostic', return_value=result), \
                 mock.patch.object(overlay, 'run') as inline, self.assertRaises(ValueError):
                overlay.run_supervised(Path(directory), plan, output, overlay.PYTHON_IMAGE, 'kubectl',
                                       base='b' * 40, head_ref='test')
            inline.assert_not_called()


if __name__ == '__main__':
    unittest.main()
