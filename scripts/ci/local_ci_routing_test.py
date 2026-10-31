"""Safe-subset CI routing diagnostic tests."""
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import local_ci_routing as routing


class RoutingDiagnosticTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'root'
        self.root.mkdir()
        self.output = Path(self.tmp.name) / 'output'
        self.source = {'head': 'a' * 40, 'tree': 'b' * 40}

    def execute(self, *, stderr=b'Ran 4 tests in 0.1s\n\nOK\n', exit_code=0, changed=False):
        def invoke(command, **kwargs):
            content = stderr.replace(b'Ran 4 tests', b'Ran 8 tests') if 'selected-acceptance-gate_test.py' in command[2] else stderr
            kwargs['stderr'].write(content)
            return subprocess.CompletedProcess(command, exit_code)
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root), \
             patch.object(routing.local_source_snapshot, 'verify_materialized'), \
             patch.object(routing.local_source_snapshot, 'capture', return_value=({'head': 'changed'} if changed else self.source)), \
             patch.object(routing.local_process_supervision, 'run_diagnostic', side_effect=invoke):
            return routing.run(self.root, self.source, self.output)

    def test_partial_evidence_preserves_source_and_never_claims_full_matrix(self):
        report = self.execute()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['source'], self.source)
        self.assertEqual(len(report['commands']), 2)
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
            count = 8 if 'selected-acceptance-gate_test.py' in command[2] else 4
            kwargs['stderr'].write(('Ran %s tests in 0.1s\n\nOK\n' % count).encode())
            return subprocess.CompletedProcess(command, 0)
        with patch.object(routing.local_source_snapshot, 'materialize', return_value=self.root) as materialize, \
             patch.object(routing.local_source_snapshot, 'verify_materialized') as verify, \
             patch.object(routing.local_source_snapshot, 'capture', return_value=self.source) as capture, \
             patch.object(routing.local_process_supervision, 'run_diagnostic', side_effect=invoke):
            routing.run(self.root, self.source, self.output)
        materialize.assert_called_once()
        self.assertEqual(verify.call_count, 2)
        self.assertEqual(capture.call_count, 2)

    def test_failed_materialization_runs_no_command(self):
        with patch.object(routing.local_source_snapshot, 'materialize', side_effect=ValueError('source changed')), \
             patch.object(routing.local_process_supervision, 'run_diagnostic') as invoke:
            with self.assertRaisesRegex(ValueError, 'source changed'):
                routing.run(self.root, self.source, self.output)
        invoke.assert_not_called()


if __name__ == '__main__':
    unittest.main()
