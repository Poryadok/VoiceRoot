"""Exercise actual workflow predicates and required-job failure behavior."""
import importlib.util
import os
from pathlib import Path
import re
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('acceptance', ROOT / '.github/ci/selected-acceptance-gate_test.py')
policy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(policy)


class FastGateRoutingTests(unittest.TestCase):
    def test_ci_only_changes_cannot_skip_script_matrix(self):
        values = policy.context('pull_request', ci_global='true', code='false')
        self.assertTrue(policy.evaluate(policy.job_condition('ci-script-tests'), values))
        for result in ('skipped', 'failure', 'cancelled'):
            response = policy.run_gate(values, 'ci-script-tests', result)
            self.assertNotEqual(response.returncode, 0, result)

    def test_current_diff_runs_for_all_prs_and_is_required(self):
        for flags in ({'code': 'false'}, {'code': 'true'}, {'ci_global': 'true'}):
            values = policy.context('pull_request', **flags)
            self.assertTrue(policy.evaluate(policy.job_condition('current-diff-contract'), values))
            for result in ('skipped', 'failure', 'cancelled'):
                self.assertNotEqual(policy.run_gate(values, 'current-diff-contract', result).returncode, 0)

    def test_history_audit_is_not_ordinary_pr_work_but_is_release_obligation(self):
        values = policy.context('pull_request')
        values['github.head_ref'] = 'codex/ordinary'
        condition = policy.job_condition('voice-history-audit')
        self.assertFalse(policy.evaluate(condition, values))
        values = policy.context('push')
        values['github.head_ref'] = ''
        self.assertTrue(policy.evaluate(condition, values))
        for result in ('skipped', 'failure', 'cancelled'):
            self.assertNotEqual(policy.run_gate(values, 'voice-history-audit', result).returncode, 0)
        for event in ('schedule', 'workflow_dispatch'):
            values = policy.context(event)
            values['github.head_ref'] = ''
            values['inputs.profile'] = 'full'
            values['needs.changes.result'] = 'skipped'
            self.assertTrue(policy.evaluate(condition, values))

    def test_shards_are_explicit_and_all_required(self):
        block = policy.block('ci-script-tests')
        self.assertIn('fail-fast: false', block)
        for shard in ('policy', 'runtime', 'staging', 'proofs'):
            self.assertRegex(block, r'shard: \[.*\b' + shard + r'\b')
        self.assertIn('VOICE_CONTRACT_PROFILE: current', block)
        self.assertNotIn('continue-on-error', block)

    def test_flutter_shards_preserve_serial_isolation_and_required_coverage(self):
        block = policy.block('flutter-unit-tests')
        self.assertIn('shard: [0, 1, 2, 3]', block)
        self.assertIn('--total-shards=4 --shard-index=${{ matrix.shard }}', block)
        self.assertIn('--concurrency=1', block)
        self.assertNotIn('flutter test --reporter expanded --concurrency=1', policy.block('flutter'))
        values = policy.context('pull_request', code='true', frontend='true')
        self.assertTrue(policy.evaluate(policy.job_condition('flutter-unit-tests'), values))
        for result in ('skipped', 'failure', 'cancelled'):
            self.assertNotEqual(policy.run_gate(values, 'flutter-unit-tests', result).returncode, 0)

    def test_go_jobs_do_not_wait_for_independent_pkg_tests(self):
        for job in ('backend-go', 'backend-go-integration-pr'):
            self.assertIn('    needs: changes\n', policy.block(job))
            self.assertNotIn('needs.backend-go-pkg', policy.job_condition(job))
        values = policy.context('pull_request', run_go='true', run_pkg='true')
        self.assertNotEqual(policy.run_gate(values, 'backend-go-pkg', 'failure').returncode, 0)


if __name__ == '__main__':
    unittest.main()
