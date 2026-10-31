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
    def test_historical_replay_is_separate_but_retained_in_full_local_suite(self):
        def commands(target):
            return subprocess.run(['make', '-n', target], cwd=ROOT, capture_output=True,
                                  text=True, check=True).stdout
        runtime = commands('ci-script-tests-runtime')
        full = commands('ci-script-tests')
        name = 'f13_exact_j_historical_red_test.py'
        self.assertNotIn(name, runtime)
        self.assertEqual(full.count(name), 1)
        self.assertIn('voice-db-runtime-provisioning-contract_test.sh', runtime)

    def test_full_validation_scope_distinguishes_master_code_and_docs(self):
        workflow = yaml.safe_load(policy.WORKFLOW)
        expression = workflow['jobs']['changes']['outputs']['full_validation']
        for event, ref, code, request, profile, expected in (
                ('push', 'refs/heads/master', 'true', 'false', 'auto', True),
                ('push', 'refs/heads/master', 'false', 'false', 'auto', False),
                ('push', 'refs/heads/master', 'false', 'true', 'auto', True),
                ('push', 'refs/heads/develop', 'true', 'false', 'auto', False),
                ('pull_request', 'refs/heads/master', 'true', 'false', 'auto', False),
                ('workflow_dispatch', 'refs/heads/master', 'false', 'false', 'full', True),
                ('workflow_dispatch', 'refs/heads/master', 'true', 'false', 'auto', False)):
            values = {'github.event_name': event, 'github.ref': ref, 'inputs.profile': profile,
                      'steps.filter.outputs.code': code, 'steps.fullrequest.outputs.requested': request}
            with self.subTest(event=event, ref=ref, code=code, request=request, profile=profile):
                self.assertEqual(policy.evaluate(expression, values), expected)

    def test_master_full_validation_actually_selects_required_jobs(self):
        import full_validation_gate
        workflow = yaml.safe_load(policy.WORKFLOW)
        values = policy.context('push', code='true', full_validation='true',
                                run_go='true', run_pkg='true', run_flutter_tier2='true',
                                integration_go_services='["analytics","bot","chat","controlledgame","federation","file","gameintegration","gateway","matchmaking","messaging","moderation","notification","realtime","role","search","social","space","story","subscription","user","voice"]')
        for name in workflow['jobs']:
            values['needs.' + name + '.result'] = 'success'
        for name in full_validation_gate.REQUIRED_JOBS:
            if name == 'changes':
                continue
            expression = workflow['jobs'][name]['if']
            expression = re.sub(
                r"contains\(fromJSON\(needs\.changes\.outputs\.integration_go_services\), '([a-z]+)'\)",
                lambda match: 'true' if match.group(1) in json.loads(values['needs.changes.outputs.integration_go_services']) else 'false',
                expression)
            with self.subTest(job=name):
                self.assertTrue(policy.evaluate(expression, values), expression)

    def test_master_promotion_and_deployment_reject_incomplete_prerequisites(self):
        workflow = yaml.safe_load(policy.WORKFLOW)
        values = policy.context('push', code='true', full_validation='true',
                                build_go_services='["chat"]', promote_services='["space"]',
                                ci_only_change='false', full_application_request='false')
        values['vars.STAGING_DEPLOY_ENABLED'] = 'true'
        for name in workflow['jobs']:
            values['needs.' + name + '.result'] = 'success'
        for name in ('staging-images-promote', 'deploy-staging'):
            condition = workflow['jobs'][name]['if']
            self.assertTrue(policy.evaluate(condition, values), name)
            for status in ('failure', 'cancelled', 'skipped'):
                changed = dict(values, **{'needs.full-validation-aggregation.result': status})
                self.assertFalse(policy.evaluate(condition, changed), (name, status))
                changed = dict(values, **{'needs.staging-images-push.result': status})
                self.assertFalse(policy.evaluate(condition, changed), (name, status))
            if name == 'deploy-staging':
                for status in ('failure', 'cancelled', 'skipped'):
                    changed = dict(values, **{'needs.staging-images-promote.result': status})
                    self.assertFalse(policy.evaluate(condition, changed), (name, status))
        empty = dict(values, **{'needs.changes.outputs.build_go_services': '[]',
                                'needs.staging-images-push.result': 'skipped',
                                'needs.changes.outputs.promote_services': '[]',
                                'needs.staging-images-promote.result': 'skipped'})
        self.assertTrue(policy.evaluate(workflow['jobs']['deploy-staging']['if'], empty))

    def test_actual_scope_step_preserves_master_breadth(self):
        workflow = yaml.safe_load(policy.WORKFLOW)
        changes = workflow['jobs']['changes']
        step = next(s for s in changes['steps'] if s.get('id') == 'scope')
        self.assertEqual(changes['outputs']['ci_global'], '${{ steps.scope.outputs.ci_global }}')
        for name in ('Resolve Go service matrix', 'Resolve staging image matrix'):
            matrix = next(s for s in changes['steps'] if s.get('name') == name)
            self.assertEqual(matrix['env']['CI_GLOBAL'], '${{ steps.scope.outputs.ci_global }}')
            self.assertIn('.ci_global = $value', matrix['run'])
        for event, raw, scripts, affected, expected in (
                ('pull_request', 'false', 'true', 'false', 'false'),
                ('pull_request', 'false', 'false', 'true', 'true'),
                ('push', 'false', 'true', '', 'true'),
                ('workflow_dispatch', 'false', 'true', '', 'true'),
                ('push', 'false', 'false', '', 'false'),
                ('pull_request', 'true', 'true', 'false', 'true')):
            with tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / 'output'
                env = dict(os.environ, EVENT_NAME=event, CI_GLOBAL=raw,
                           CI_SCRIPTS=scripts, AFFECTED_GLOBAL=affected,
                           GITHUB_OUTPUT=output.as_posix())
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

    def test_database_migrations_select_their_own_service_tests(self):
        for database in (ROOT / 'src/backend/migrations').iterdir():
            if not database.is_dir():
                continue
            service = database.name.removesuffix('_db')
            service = {'game_integration': 'gameintegration'}.get(service, service)
            selector = 'auth' if service == 'auth' else 'svc_' + service
            flags = self.filters_for(database.relative_to(ROOT).as_posix() + '/000999_test.up.sql')
            self.assertEqual(flags[selector], 'true', database.name)

    def test_auth_consumer_checks_follow_subscription_and_contract_inputs(self):
        for path in ('src/backend/subscription/internal/events/consumer.go',
                     'src/backend/migrations/subscription_db/000999_test.up.sql',
                     'protos/voice/subscription/v1/subscription.proto'):
            with self.subTest(path=path):
                flags = self.filters_for(path)
                self.assertEqual(flags.get('auth_test_impact'), 'true')
                values = policy.context('pull_request', **flags)
                self.assertTrue(policy.evaluate(policy.job_condition('backend-auth'), values))
                self.assertFalse(policy.evaluate(policy.job_condition('backend-auth-image'), values))
                self.assertNotEqual(policy.run_gate(values, 'backend-auth', 'skipped').returncode, 0)

    def test_selected_analytics_integration_is_a_required_pr_job(self):
        values = policy.context('pull_request', code='true', integration_analytics='true')
        self.assertTrue(policy.evaluate(policy.job_condition('analytics-clickhouse-integration'), values))
        for result in ('skipped', 'failure', 'cancelled', ''):
            self.assertNotEqual(policy.run_gate(values, 'analytics-clickhouse-integration', result).returncode, 0)
        values['needs.changes.outputs.integration_analytics'] = 'false'
        self.assertFalse(policy.evaluate(policy.job_condition('analytics-clickhouse-integration'), values))
        workflow = yaml.safe_load(policy.WORKFLOW)
        steps = workflow['jobs']['analytics-clickhouse-integration']['steps']
        run = next(s['run'] for s in steps if s.get('name') == 'Analytics ClickHouse integration tests')
        self.assertIn('go test -json -count=1 -tags=integration', run)
        self.assertIn('go_integration_receipts.py', run)
        self.assertIn('TestClickHouseInsertBatch', run)

    def test_clickhouse_schema_alone_requires_analytics_integration(self):
        flags = self.filters_for('docker/clickhouse/init/001_events.sql')
        self.assertEqual(flags.get('analytics_integration_inputs'), 'true')
        workflow = yaml.safe_load(policy.WORKFLOW)
        output = workflow['jobs']['changes']['outputs']['integration_analytics']
        self.assertTrue(policy.evaluate(output, {'steps.filter.outputs.analytics_integration_inputs': 'true',
                                                  'steps.gomatrix.outputs.integration_analytics': 'false'}))
        values = policy.context('pull_request', integration_analytics='true', **flags)
        self.assertTrue(policy.evaluate(policy.job_condition('analytics-clickhouse-integration'), values))
        self.assertNotEqual(policy.run_gate(values, 'analytics-clickhouse-integration', 'skipped').returncode, 0)

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

    def test_selected_chat_auth_java_proof_cannot_be_skipped(self):
        values = policy.context('pull_request', code='true', chat_auth_java='true')
        self.assertTrue(policy.evaluate(policy.job_condition('chat-auth-java-integration'), values))
        for result in ('skipped', 'failure', 'cancelled'):
            self.assertNotEqual(policy.run_gate(values, 'chat-auth-java-integration', result).returncode, 0)

    def test_history_audit_is_scheduled_and_excluded_from_fast_gate(self):
        values = policy.context('pull_request')
        values['github.head_ref'] = 'codex/ordinary'
        condition = policy.job_condition('voice-history-audit')
        self.assertFalse(policy.evaluate(condition, values))
        values = policy.context('push')
        values['github.head_ref'] = ''
        self.assertFalse(policy.evaluate(condition, values))
        self.assertNotIn('      - voice-history-audit', policy.block('ci-gate'))
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
        self.assertIn('flutter_test_shards.py run --plan', block)
        self.assertIn('--concurrency=1', (ROOT / 'scripts/ci/flutter_test_shards.py').read_text())
        self.assertNotIn('flutter test --reporter expanded --concurrency=1', policy.block('flutter'))
        values = policy.context('pull_request', code='true', frontend='true')
        values['needs.flutter-unit-plan.result'] = 'success'
        self.assertTrue(policy.evaluate(policy.job_condition('flutter-unit-tests'), values))
        values['needs.flutter-unit-plan.result'] = 'failure'
        self.assertFalse(policy.evaluate(policy.job_condition('flutter-unit-tests'), values))
        self.assertIn('needs: [flutter-unit-plan, flutter-unit-tests]', policy.block('flutter-unit-receipts'))
        gate = policy.block('ci-gate')
        self.assertIn('JOB_FLUTTER_UNIT_TESTS: ${{ needs.flutter-unit-receipts.result }}', gate)
        self.assertIn('      - flutter-unit-receipts', gate)
        for result in ('skipped', 'failure', 'cancelled'):
            self.assertNotEqual(policy.run_gate(values, 'flutter-unit-tests', result).returncode, 0)

    def test_go_jobs_do_not_wait_for_independent_pkg_tests(self):
        for job in ('backend-go', 'backend-go-integration-pr'):
            self.assertIn('    needs: changes\n', policy.block(job))
            self.assertNotIn('needs.backend-go-pkg', policy.job_condition(job))
        values = policy.context('pull_request', run_go='true', run_pkg='true')
        self.assertNotEqual(policy.run_gate(values, 'backend-go-pkg', 'failure').returncode, 0)

    def test_go_integrations_require_docker_and_execute_without_test_cache(self):
        workflow = yaml.safe_load(policy.WORKFLOW)
        steps = workflow['jobs']['backend-go-integration-pr']['steps']
        docker_step = next(s for s in steps if s.get('name') == 'Require Docker for Go integration')
        test_step = next(s for s in steps if s.get('name', '').startswith('Go integration test'))
        self.assertLess(steps.index(docker_step), steps.index(test_step))
        self.assertIn('go test -json -count=1', test_step['run'])
        self.assertIn('go_integration_receipts.py', test_step['run'])
        artifact = next(s for s in steps if s.get('name') == 'Upload Go integration diagnostics')
        self.assertEqual(artifact['if'], 'always()')
        self.assertIn('matrix.service', artifact['with']['name'])
        with tempfile.TemporaryDirectory() as directory:
            command = Path(directory) / 'docker'
            command.write_text('#!/bin/sh\nexit 1\n', encoding='utf-8')
            command.chmod(0o755)
            env = dict(os.environ, PATH=directory + os.pathsep + os.environ['PATH'])
            result = subprocess.run([policy.bash(), '-c', docker_step['run']], env=env,
                                    text=True, capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            command.write_text('#!/bin/sh\nexit 0\n', encoding='utf-8')
            result = subprocess.run([policy.bash(), '-c', docker_step['run']], env=env,
                                    text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_go_integration_diagnostics_preserve_failure(self):
        workflow = yaml.safe_load(policy.WORKFLOW)
        steps = workflow['jobs']['backend-go-integration-pr']['steps']
        run = next(s['run'] for s in steps if s.get('name', '').startswith('Go integration test'))
        with tempfile.TemporaryDirectory() as directory:
            go = Path(directory) / 'go'
            lines = [
                {'Package': 'voice/backend/chat/store', 'Test': 'TestCase', 'Action': 'run'},
                {'Package': 'voice/backend/chat/store', 'Test': 'TestCase', 'Action': 'fail'},
                {'Package': 'voice/backend/chat/store', 'Action': 'fail'},
            ]
            payload = ''.join(json.dumps(line) + '\n' for line in lines)
            go.write_text('#!/bin/sh\ncat <<\'EVENTS\'\n' + payload + 'EVENTS\nexit 1\n', encoding='utf-8')
            go.chmod(0o755)
            env = dict(os.environ, PATH=directory + os.pathsep + os.environ['PATH'],
                       RUNNER_TEMP=Path(directory).as_posix(), SERVICE='chat',
                       CANDIDATE_SHA='a' * 40, GITHUB_RUN_ID='123', GITHUB_RUN_ATTEMPT='1',
                       PYTHONDONTWRITEBYTECODE='1')
            result = subprocess.run([policy.bash(), '-c', run], cwd=ROOT / 'src/backend/chat',
                                    env=env, text=True, capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            summary = json.loads((Path(directory) / 'go-integration-summary.json').read_text())
            self.assertEqual(summary['counts']['fail'], 1)
            self.assertEqual(summary['exit_code'], 1)
            successful = payload.replace('"Action": "fail"', '"Action": "pass"')
            go.write_text('#!/bin/sh\ncat <<\'EVENTS\'\n' + successful + 'EVENTS\nexit 0\n', encoding='utf-8')
            result = subprocess.run([policy.bash(), '-c', run], cwd=ROOT / 'src/backend/chat',
                                    env=env, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == '__main__':
    unittest.main()
