"""Safe-subset CI routing diagnostic tests."""
import subprocess
import hashlib
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import local_ci_routing as routing


class RoutingDiagnosticTest(unittest.TestCase):
    def test_workflow_is_part_of_the_pinned_safety_surface(self):
        self.assertIn('.github/workflows/ci.yml', routing.SAFETY_SURFACE)

    def test_workflow_with_bash_env_injection_refuses_before_execution(self):
        path = self.root / '.github/workflows/ci.yml'
        path.parent.mkdir(parents=True)
        path.write_text('jobs:\n  ci-gate:\n    env:\n      BASH_ENV: malicious\n')
        expected = routing.SAFETY_SURFACE['.github/workflows/ci.yml']
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root), \
             patch.object(routing, 'SAFETY_SURFACE', {'.github/workflows/ci.yml': expected}), \
             patch.object(routing.local_process_supervision, 'run_diagnostic') as invoke:
            with self.assertRaisesRegex(ValueError, 'safety surface changed'):
                routing.run(self.root, self.source, self.output)
        invoke.assert_not_called()

    def test_ambient_gate_variables_do_not_reach_tests(self):
        captured = []
        def invoke(command, **kwargs):
            captured.append(kwargs['env'])
            if len(command) == 2:
                kwargs['stdout'].write(b'All verify-required-jobs tests passed.\n')
            else:
                count = 8 if any('selected-acceptance-gate_test.py' in arg for arg in command) else 4
                kwargs['stderr'].write(('Ran %s tests in 0.1s\n\nOK\n' % count).encode())
            return subprocess.CompletedProcess(command, 0)
        ambient = {'JOB_BACKEND_GO': 'success', 'RUN_GO': 'true', 'FILTER_AUTH': 'true',
                   'GLOBAL': 'true', 'PROTOS': 'true', 'COMPOSE': 'true', 'CODE': 'true',
                   'BASH_FUNC_rm%%': '() { :; }', 'BASHOPTS': 'extglob'}
        with patch.dict(os.environ, ambient), \
             patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root), \
             patch.object(routing, '_verify_safety'), \
             patch.object(routing.local_source_snapshot, 'verify_materialized'), \
             patch.object(routing.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(routing.local_process_supervision, 'run_diagnostic', side_effect=invoke):
            routing.run(self.root, self.source, self.output)
        for env in captured:
            self.assertFalse(set(ambient) & set(env))

    @unittest.skipUnless(os.name == 'nt', 'Windows owned process acceptance')
    def test_python_commands_cannot_import_adjacent_shadow_modules(self):
        marker = self.root / 'shadow-executed'
        for directory, name, suite in ((self.root / '.github/ci', 'selected-acceptance-gate_test.py', 'SelectedAcceptanceGateTests'),
                                       (self.root / 'scripts/ci', 'fast-gate-routing_test.py', 'FastGateRoutingTests')):
            directory.mkdir(parents=True)
            (directory / 'yaml.py').write_text('from pathlib import Path\nPath(' + repr(str(marker)) + ').write_text("bad")\nraise RuntimeError("shadow")\n')
            (directory / name).write_text('import unittest, yaml\nclass ' + suite + '(unittest.TestCase):\n def test_ok(self): self.assertTrue(hasattr(yaml, "safe_load"))\nif __name__ == "__main__": unittest.main()\n')
        (self.root / '.github/ci/verify-required-jobs_test.sh').write_text('echo "All verify-required-jobs tests passed."\n')
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root), \
             patch.object(routing, '_shell_path', return_value=Path('C:/Program Files/Git/bin/bash.exe').resolve(strict=True)), \
             patch.object(routing, '_verify_safety'), patch.object(routing, 'GATE_CASES', ('test_ok',)), \
             patch.object(routing, 'ROUTER_CASES', ('test_ok',)), \
             patch.object(routing.local_source_snapshot, 'verify_materialized'), \
             patch.object(routing.local_source_snapshot, 'capture', return_value=self.source):
            report = routing.run(self.root, self.source, self.output)
        self.assertEqual(report['status'], 'incomplete')
        self.assertFalse(marker.exists())
    def test_unreviewed_executable_drift_refuses_before_execution(self):
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root), \
             patch.object(routing.local_process_supervision, 'run_diagnostic') as invoke:
            with self.assertRaisesRegex(ValueError, 'safety surface'):
                routing.run(self.root, self.source, self.output)
        invoke.assert_not_called()

    def test_missing_shell_completion_cannot_satisfy_gate(self):
        def invoke(command, **kwargs):
            return subprocess.CompletedProcess(command, 0)
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root), \
             patch.object(routing, '_verify_safety'), \
             patch.object(routing.local_process_supervision, 'run_diagnostic', side_effect=invoke):
            with self.assertRaisesRegex(ValueError, 'gate-shell'):
                routing.run(self.root, self.source, self.output)

    def test_modified_audited_file_refuses_before_any_process(self):
        path = self.root / 'audited-test'
        path.write_bytes(b'changed')
        expected = hashlib.sha256(b'reviewed').hexdigest()
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root), \
             patch.object(routing, 'SAFETY_SURFACE', {'audited-test': expected}), \
             patch.object(routing.local_process_supervision, 'run_diagnostic') as invoke:
            with self.assertRaisesRegex(ValueError, 'safety surface changed'):
                routing.run(self.root, self.source, self.output)
        invoke.assert_not_called()
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'root'
        self.root.mkdir()
        self.fixture_shell = self.root / 'fixture-shell'
        self.fixture_shell.write_bytes(b'owned fake shell for mocked execution')
        shell_patch = patch.object(routing, '_shell_path', return_value=self.fixture_shell)
        shell_patch.start()
        self.addCleanup(shell_patch.stop)
        self.output = Path(self.tmp.name) / 'output'
        self.source = {'head': 'a' * 40, 'tree': 'b' * 40}

    def execute(self, *, stderr=b'Ran 4 tests in 0.1s\n\nOK\n', exit_code=0, changed=False):
        def invoke(command, **kwargs):
            if len(command) == 2:
                kwargs['stdout'].write(b'All verify-required-jobs tests passed.\n')
                return subprocess.CompletedProcess(command, exit_code)
            content = stderr.replace(b'Ran 4 tests', b'Ran 8 tests') if any('selected-acceptance-gate_test.py' in arg for arg in command) else stderr
            kwargs['stderr'].write(content)
            return subprocess.CompletedProcess(command, exit_code)
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root), \
             patch.object(routing, '_verify_safety'), \
             patch.object(routing.local_source_snapshot, 'verify_materialized'), \
             patch.object(routing.local_source_snapshot, 'capture', return_value=({'head': 'changed'} if changed else self.source)), \
             patch.object(routing.local_process_supervision, 'run_diagnostic', side_effect=invoke):
            return routing.run(self.root, self.source, self.output)

    def test_partial_evidence_preserves_source_and_never_claims_full_matrix(self):
        report = self.execute()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['source'], self.source)
        self.assertEqual(len(report['commands']), 3)
        self.assertEqual(report['bash']['path'], str(self.fixture_shell))
        self.assertEqual(report['bash']['sha256'], hashlib.sha256(self.fixture_shell.read_bytes()).hexdigest())
        self.assertTrue(report['omitted'])

    def test_failed_skipped_or_missing_tests_are_rejected(self):
        for stderr, code in ((b'Ran 0 tests in 0s\n\nOK\n', 0),
                             (b'Ran 4 tests in 0s\n\nOK (skipped=1)\n', 0),
                             (b'Ran 4 tests in 0s\n\nFAILED (failures=1)\n', 1)):
            with self.subTest(stderr=stderr):
                with tempfile.TemporaryDirectory() as directory:
                    self.output = Path(directory) / 'report'
                    with self.assertRaises(ValueError):
                        self.execute(stderr=stderr, exit_code=code)

    def test_source_change_invalidates_diagnostic(self):
        with self.assertRaisesRegex(ValueError, 'source changed'):
            self.execute(changed=True)

    def test_materialization_provides_precheck_and_every_command_has_postcheck(self):
        def invoke(command, **kwargs):
            if len(command) == 2:
                kwargs['stdout'].write(b'All verify-required-jobs tests passed.\n')
                return subprocess.CompletedProcess(command, 0)
            count = 8 if any('selected-acceptance-gate_test.py' in arg for arg in command) else 4
            kwargs['stderr'].write(('Ran %s tests in 0.1s\n\nOK\n' % count).encode())
            return subprocess.CompletedProcess(command, 0)
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root) as materialize, \
             patch.object(routing, '_verify_safety'), \
             patch.object(routing.local_source_snapshot, 'verify_materialized') as verify, \
             patch.object(routing.local_source_snapshot, 'capture', return_value=self.source) as capture, \
             patch.object(routing.local_process_supervision, 'run_diagnostic', side_effect=invoke):
            routing.run(self.root, self.source, self.output)
        materialize.assert_called_once()
        self.assertEqual(verify.call_count, 3)
        self.assertEqual(capture.call_count, 3)

    def test_failed_materialization_runs_no_command(self):
        with patch.object(routing.local_source_snapshot, 'materialize', side_effect=ValueError('source changed')), \
             patch.object(routing.local_process_supervision, 'run_diagnostic') as invoke:
            with self.assertRaisesRegex(ValueError, 'source changed'):
                routing.run(self.root, self.source, self.output)
        invoke.assert_not_called()


if __name__ == '__main__':
    unittest.main()
