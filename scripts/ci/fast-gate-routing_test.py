"""Exercise actual workflow predicates and required-job failure behavior."""
import importlib.util
import fnmatch
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
import yaml

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('acceptance', ROOT / '.github/ci/selected-acceptance-gate_test.py')
policy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(policy)


class FastGateRoutingTests(unittest.TestCase):
    def test_actual_scope_step_preserves_master_breadth(self):
        workflow = yaml.safe_load(policy.WORKFLOW)
        changes = workflow['jobs']['changes']
        step = next(s for s in changes['steps'] if s.get('id') == 'scope')
        self.assertEqual(changes['outputs']['ci_global'], '${{ steps.scope.outputs.ci_global }}')
        for name in ('Resolve Go service matrix', 'Resolve staging image matrix'):
            matrix = next(s for s in changes['steps'] if s.get('name') == name)
            self.assertEqual(matrix['env']['CI_GLOBAL'], '${{ steps.scope.outputs.ci_global }}')
            self.assertIn('.ci_global = $value', matrix['run'])
        for event, raw, scripts, expected in (
                ('pull_request', 'false', 'true', 'false'),
                ('push', 'false', 'true', 'true'),
                ('workflow_dispatch', 'false', 'true', 'true'),
                ('push', 'false', 'false', 'false'),
                ('pull_request', 'true', 'true', 'true')):
            with tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / 'output'
                env = dict(os.environ, EVENT_NAME=event, CI_GLOBAL=raw,
                           CI_SCRIPTS=scripts, GITHUB_OUTPUT=output.as_posix())
                result = subprocess.run([policy.bash(), '-c', step['run']],
                                        env=env, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(output.read_text().strip(), 'ci_global=' + expected)

    def filters_for(self, *paths):
        workflow = yaml.safe_load(policy.WORKFLOW)
        step = next(s for s in workflow['jobs']['changes']['steps']
                    if s.get('id') == 'filter')
        self.assertEqual(step['with'].get('predicate-quantifier'), 'some-with-excludes')
        filters = yaml.safe_load((ROOT / '.github/ci/path-filters.yml').read_text())
        # These examples use plain paths, * and ** only; no extglob approximation.
        return {name: 'true' if any(
            any(fnmatch.fnmatchcase(path, p) for p in rules if not p.startswith('!'))
            and not any(fnmatch.fnmatchcase(path, p[1:]) for p in rules if p.startswith('!'))
            for path in paths) else 'false' for name, rules in filters.items()}

    def test_docs_exclusions_and_mixed_changes(self):
        self.assertEqual(self.filters_for('docs/TESTING.md')['code'], 'false')
        self.assertEqual(self.filters_for('docs/TESTING.md', 'src/backend/chat/main.go')['code'], 'true')

    def test_leaf_ci_keeps_script_checks_without_product_fanout(self):
        for path in ('scripts/ci/auth_session_floor_diagnostics.py',
                     'scripts/ci/gate-latency_test.py',
                     '.github/workflows/t31-session-events-e2e.yml'):
            flags = self.filters_for(path)
            self.assertEqual(flags['ci_global'], 'false', path)
            self.assertEqual(flags['ci_scripts'], 'true', path)
            values = policy.context('pull_request', **flags)
            self.assertTrue(policy.evaluate(policy.job_condition('ci-script-tests'), values))
            self.assertFalse(policy.evaluate(policy.job_condition('backend-go'), values))
            self.assertFalse(policy.evaluate(policy.job_condition('compose-e2e'), values))
            for result in ('skipped', 'failure', 'cancelled', ''):
                self.assertNotEqual(policy.run_gate(values, 'ci-script-tests', result).returncode, 0)
        for path in ('scripts/ci/unknown-new-validator.py', '.github/workflows/unknown.yml',
                     '.github/workflows/ci.yml', 'Makefile'):
            self.assertEqual(self.filters_for(path)['ci_global'], 'true', path)
        self.assertEqual(self.filters_for('scripts/ci/gate-latency.py', 'Makefile')['ci_global'], 'true')

    def test_attachment_and_renderer_have_actual_input_selectors(self):
        for path in ('src/backend/migrations/matchmaking_db/000020_test.up.sql',
                     'src/backend/notification/internal/store/event.go'):
            values = policy.context('pull_request', **self.filters_for(path))
            self.assertFalse(policy.evaluate(policy.job_condition('a1-attachment-restart-proof'), values), path)
            self.assertFalse(policy.evaluate(policy.job_condition('nats-hub-config-renderer-image'), values), path)
        for path in ('src/backend/file/internal/grpcsvc/file.go',
                     'src/backend/migrations/file_db/000001_test.up.sql'):
            values = policy.context('pull_request', **self.filters_for(path))
            self.assertTrue(policy.evaluate(policy.job_condition('a1-attachment-restart-proof'), values), path)
            self.assertNotEqual(policy.run_gate(values, 'a1-attachment-restart-proof', 'skipped').returncode, 0)
        for path in ('src/backend/pkg/cmd/nats-hub-config-renderer/main.go',
                     'src/backend/chat/pb/voice/chat/v1/chat.pb.go', '.dockerignore'):
            values = policy.context('pull_request', **self.filters_for(path))
            self.assertTrue(policy.evaluate(policy.job_condition('nats-hub-config-renderer-image'), values), path)
            self.assertNotEqual(policy.run_gate(values, 'nats-hub-config-renderer-image', 'skipped').returncode, 0)

    def test_docker_matrix_retains_source_consumers_without_s2s_builds(self):
        for seed, expected in (
                ('file', {'file'}), ('notification', {'notification'}),
                ('chat', {'chat', 'messaging'}), ('role', {'role', 'space', 'messaging'}),
                ('messaging', {'messaging', 'gateway'}), ('social', {'social', 'user'}),
                ('federation', {'federation', 'voice'})):
            env = dict(os.environ, FILTER_JSON=json.dumps({'svc_' + seed: 'true'}))
            result = subprocess.run([sys.executable, str(ROOT / 'scripts/ci/resolve-go-docker-matrix.py')],
                                    env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(set(json.loads(result.stdout)), expected, seed)
        for flag in ('global', 'ci_global', 'protos', 'pkg', 'go_build_shared'):
            env = dict(os.environ, FILTER_JSON=json.dumps({flag: 'true'}))
            result = subprocess.run([sys.executable, str(ROOT / 'scripts/ci/resolve-go-docker-matrix.py')],
                                    env=env, capture_output=True, text=True, check=True)
            self.assertEqual(len(json.loads(result.stdout)), 21, flag)

    def test_pr_build_steps_use_docker_selection_and_keep_consumer_tests(self):
        workflow = yaml.safe_load(policy.WORKFLOW)
        job = workflow['jobs']['backend-go']
        build_steps = [s for s in job['steps'] if s.get('id') == 'dockerctx'
                       or s.get('uses', '').startswith('docker/setup-buildx-action@')
                       or s.get('name', '').startswith('Build ${{ matrix.service }} image')]
        self.assertEqual(len(build_steps), 3)
        call = 'contains(fromJSON(needs.changes.outputs.docker_go_services), matrix.service)'
        for event in ('pull_request', 'push'):
            for service in ('file', 'messaging', 'story', 'gateway'):
                for step in build_steps:
                    self.assertIn(call, step['if'])
                    expression = step['if'].replace(call, 'selected_image')
                    values = policy.context(event, run_go='true')
                    values['selected_image'] = service == 'file'
                    self.assertEqual(bool(policy.evaluate(expression, values)),
                                     event == 'pull_request' and service == 'file')
                self.assertTrue(policy.evaluate(policy.job_condition('backend-go'), values))

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
