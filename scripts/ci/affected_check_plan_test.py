"""Contract tests for the shared CI affected-check planner."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import shlex
import tempfile
import subprocess
import sys
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location('planner', ROOT / 'scripts/ci/affected_check_plan.py')
planner = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(planner)


class AffectedCheckPlanTests(unittest.TestCase):
    def test_missing_resolver_dependency_is_actionable(self):
        error = subprocess.CalledProcessError(1, ['bash'], stderr='resolve-go-matrix: jq is required\n')
        with mock.patch.object(planner.subprocess, 'run', side_effect=error):
            with self.assertRaises(subprocess.CalledProcessError) as caught:
                planner.resolve_go_matrix(ROOT, {})
        self.assertIn('jq is required', caught.exception.stderr)

    def test_current_filter_rules_cover_direct_shared_and_docs(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        for path, expected in (
            ('src/backend/chat/main.go', 'svc_chat'),
            ('src/backend/pkg/config/config.go', 'pkg'),
            ('protos/voice/chat/v1/chat.proto', 'protos'),
            ('src/backend/migrations/chat_db/000999_test.up.sql', 'svc_chat'),
            ('scripts/ci/unknown.py', 'ci_global'),
        ):
            with self.subTest(path=path):
                self.assertIn(path, planner.match_filters(rules, [path])[expected])
        self.assertFalse(planner.match_filters(rules, ['docs/TESTING.md'])['code'])
        mixed = planner.match_filters(rules, ['docs/TESTING.md', 'src/backend/chat/main.go'])
        self.assertEqual(mixed['code'], ['src/backend/chat/main.go'])

    def test_unsupported_filter_syntax_is_rejected(self):
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / 'filters.yml'
            for pattern in ('src/{one,two}/**', '!src/foo**bar'):
                path.write_text('code:\n  - "' + pattern + '"\n', encoding='utf-8')
                with self.subTest(pattern=pattern), self.assertRaises(ValueError):
                    planner.load_filters(path)

    def test_plan_selects_checks_with_paths_as_reasons(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        path = 'src/backend/chat/main.go'
        matched = planner.match_filters(rules, [path])
        matrix = {'go_services': ['chat', 'messaging', 'file', 'gateway'],
                  'docker_go_services': ['chat', 'messaging'],
                  'integration_go_services': ['chat'], 'run_go': True, 'run_pkg': True}
        plan = planner.make_plan(matched, matrix)
        self.assertIn('backend-go', plan['checks'])
        self.assertIn('backend-go-integration-pr', plan['checks'])
        self.assertIn(path, plan['checks']['backend-go-integration-pr'])
        self.assertEqual(plan['components']['go'], matrix['go_services'])
        self.assertNotIn('flutter', plan['checks'])

    def test_unknown_code_widens_checks(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        path = 'new-area/executable.sh'
        matched = planner.match_filters(rules, [path])
        self.assertEqual(planner.unclassified_paths(matched), [path])
        self.assertTrue(planner.effective_filters(matched)['ci_global'])
        self.assertFalse(planner.effective_filters(
            planner.match_filters(rules, ['docs/TESTING.md']))['ci_global'])
        future_rule = {'code': ['src/backend/newservice/main.go'],
                       'svc_newservice': ['src/backend/newservice/main.go']}
        self.assertTrue(planner.effective_filters(future_rule)['ci_global'])
        future_rule['frontend'] = future_rule['code']
        self.assertTrue(planner.effective_filters(future_rule)['ci_global'])

    def test_shared_inputs_propose_all_go_integration_consumers(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        for path in ('src/backend/pkg/config/config.go',
                     'protos/voice/chat/v1/chat.proto', 'new-area/executable.sh'):
            with self.subTest(path=path):
                matched = planner.match_filters(rules, [path])
                matrix = {'go_services': ['chat', 'voice'], 'docker_go_services': ['chat', 'voice'],
                          'integration_go_services': [], 'run_go': True, 'run_pkg': True}
                plan = planner.make_plan(matched, matrix)
                self.assertEqual(plan['components']['go_integration'], ['chat', 'voice'])
                self.assertIn('backend-go-integration-pr', plan['checks'])

    def test_required_results_must_all_be_successful(self):
        plan = {'checks': {'current-diff-contract': ['every pull request'],
                           'backend-go': ['src/backend/chat/main.go']}}
        self.assertEqual(planner.verify_results(plan, {'current-diff-contract': 'success',
                                                       'backend-go': 'success'}), [])
        for result in ('skipped', 'cancelled', 'failure', ''):
            with self.subTest(result=result):
                self.assertTrue(planner.verify_results(plan, {'current-diff-contract': 'success',
                                                              'backend-go': result}))
        self.assertTrue(planner.verify_results(plan, {'current-diff-contract': 'success'}))

    def test_plan_gate_is_bound_to_candidate_and_requires_receipts(self):
        candidate = 'a' * 40
        identity = {'base': 'b' * 40, 'head': 'c' * 40, 'candidate': candidate}
        plan = {'schema': 'voice-affected-check-plan-v1',
                'identity': identity,
                'checks': {'current-diff-contract': ['every pull request'],
                           'flutter-unit-tests': ['src/frontend/lib/main.dart']}}
        needs = {'current-diff-contract': {'result': 'success'},
                 'flutter-unit-tests': {'result': 'success'},
                 'flutter-unit-receipts': {'result': 'success'}}
        self.assertEqual(planner.verify_gate(plan, needs, identity), [])
        for value in ('failure', 'cancelled', 'skipped', ''):
            with self.subTest(result=value):
                changed = dict(needs, **{'flutter-unit-receipts': {'result': value}})
                self.assertIn('flutter-unit-tests', planner.verify_gate(plan, changed, identity))
        self.assertIn('flutter-unit-tests', planner.verify_gate(plan, {}, identity))
        with self.assertRaises(ValueError):
            planner.verify_gate(plan, needs, dict(identity, base='d' * 40))
        with self.assertRaises(ValueError):
            planner.verify_gate(dict(plan, checks={}), needs, identity)

    def test_release_branch_does_not_add_nightly_history_audit_to_pr_plan(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matched = planner.match_filters(rules, ['docs/TESTING.md'])
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        plan = planner.make_plan(matched, matrix, 'codex/voice-master-release-example')
        self.assertNotIn('voice-history-audit', plan['checks'])
        self.assertNotIn('voice-history-audit', planner.make_plan(matched, matrix, 'codex/ordinary')['checks'])

    def test_hosted_comparison_reports_unknown_path_widening(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matched = planner.match_filters(rules, ['new-area/executable.sh'])
        empty = {'go_services': [], 'docker_go_services': [],
                 'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        broad = dict(empty, go_services=['chat'], run_go=True, run_pkg=True)
        proposed = planner.make_plan(matched, broad)
        hosted = {key: str(bool(paths)).lower() for key, paths in matched.items()}
        with mock.patch.object(planner, 'resolve_go_matrix', return_value=empty):
            diff = planner.compare_hosted(ROOT, proposed, hosted, '')
        self.assertIn('backend-go', diff['planner_only_checks'])
        self.assertIn('ci_global', diff['filter_disagreements'])
        self.assertEqual(diff['hosted_only_checks'], [])
        with self.assertRaises(ValueError):
            planner.compare_hosted(ROOT, proposed, {}, '')

    def test_go_matrix_reuses_the_actual_resolver(self):
        matrix = planner.resolve_go_matrix(ROOT, {'code': True, 'svc_chat': True})
        self.assertEqual(set(matrix['go_services']), {'chat', 'messaging', 'file', 'gateway'})
        self.assertEqual(set(matrix['docker_go_services']), {'chat', 'messaging'})
        self.assertTrue({'chat', 'messaging', 'file', 'gateway', 'analytics',
                         'notification', 'realtime'} <= set(matrix['integration_go_services']))
        self.assertTrue(matrix['run_go'])
        self.assertTrue(matrix['run_pkg'])

    def test_go_resolver_rejects_malformed_filter_json(self):
        bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else 'bash'
        env = dict(os.environ, FILTER_JSON='{invalid', FORCE_FULL='false')
        with tempfile.TemporaryDirectory() as directory:
            env['GITHUB_OUTPUT'] = str(Path(directory) / 'output')
            result = subprocess.run([bash, str(ROOT / 'scripts/ci/resolve-go-matrix.sh')],
                                    cwd=ROOT, env=env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0, result.stdout)

    def test_go_resolver_refuses_missing_jq(self):
        bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else shutil.which('bash')
        with tempfile.TemporaryDirectory() as directory:
            env = dict(os.environ, PATH=directory, FILTER_JSON='{"svc_chat":"true"}', FORCE_FULL='false')
            result = subprocess.run([bash, str(ROOT / 'scripts/ci/resolve-go-matrix.sh')],
                                    cwd=ROOT, env=env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn('jq is required', result.stderr)

    def test_go_resolver_handles_crlf_and_rejects_jq_failure(self):
        bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else 'bash'
        real_jq = subprocess.check_output([bash, '-c', 'command -v jq'], text=True).strip()
        with tempfile.TemporaryDirectory() as directory:
            shim = Path(directory) / 'jq'
            env = dict(os.environ, PATH=directory + os.pathsep + os.environ['PATH'])
            shim.write_text('#!/usr/bin/env bash\nset -o pipefail\n' + shlex.quote(real_jq) +
                            ' "$@" | tr -d "\\r" | sed "s/$/\\r/"\n', encoding='utf-8')
            shim.chmod(0o755)
            with mock.patch.dict(os.environ, env):
                matrix = planner.resolve_go_matrix(ROOT, {'code': True, 'svc_chat': True})
            self.assertIn('chat', matrix['integration_go_services'])
            shim.write_text('#!/usr/bin/env bash\nif [[ "$1" == -r && "$2" == --arg ]]; then exit 42; fi\n'
                            'exec ' + shlex.quote(real_jq) + ' "$@"\n', encoding='utf-8')
            with mock.patch.dict(os.environ, env):
                with self.assertRaises(subprocess.CalledProcessError):
                    planner.resolve_go_matrix(ROOT, {'code': True, 'svc_chat': True})
            shim.write_text('#!/usr/bin/env bash\nif [[ "$1" == -r && "$2" == ".[]" ]]; then exit 42; fi\n'
                            'exec ' + shlex.quote(real_jq) + ' "$@"\n', encoding='utf-8')
            with mock.patch.dict(os.environ, env):
                with self.assertRaises(subprocess.CalledProcessError):
                    planner.resolve_go_matrix(ROOT, {'code': True, 'svc_chat': True})

    def test_migration_only_selects_owner_and_event_consumers(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        path = 'src/backend/migrations/chat_db/000999_example.up.sql'
        matched = planner.match_filters(rules, [path])
        matrix = planner.resolve_go_matrix(ROOT, planner.effective_filters(matched))
        integrations = set(matrix['integration_go_services'])
        self.assertTrue({'chat', 'messaging', 'realtime', 'notification', 'analytics'} <= integrations)
        self.assertNotIn('story', integrations)

    def test_plan_binds_source_and_is_deterministic(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            def git(*args):
                return subprocess.check_output(['git', '-C', d, *args], text=True).strip()
            git('init', '-q')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            (root / '.github/ci').mkdir(parents=True)
            (root / '.github/ci/path-filters.yml').write_text(
                'code:\n  - "**"\nsvc_chat:\n  - src/backend/chat/**\n', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'base')
            base = git('rev-parse', 'HEAD')
            (root / 'src/backend/chat').mkdir(parents=True)
            (root / 'src/backend/chat/main.go').write_text('package main\n', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'head')
            head = git('rev-parse', 'HEAD')
            matrix = {'go_services': ['chat'], 'docker_go_services': [],
                      'integration_go_services': ['chat'], 'run_go': True, 'run_pkg': True}
            with mock.patch.object(planner, 'resolve_go_matrix', return_value=matrix):
                plan = planner.build_plan(root, base, head, head)
                self.assertEqual(plan, planner.build_plan(root, base, head, head))
            self.assertEqual(plan['identity'], {'base': base, 'head': head, 'candidate': head})
            self.assertEqual(plan['changed_paths'], ['src/backend/chat/main.go'])
            self.assertIn('backend-go-integration-pr', plan['checks'])

    def test_unsupported_filter_configuration_widens_the_plan(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            def git(*args):
                return subprocess.check_output(['git', '-C', d, *args], text=True).strip()
            git('init', '-q')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            (root / '.github/ci').mkdir(parents=True)
            (root / '.github/ci/path-filters.yml').write_text('code:\n  - "{a,b}/**"\n', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'base')
            base = git('rev-parse', 'HEAD')
            (root / 'change.txt').write_text('change', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'head')
            head = git('rev-parse', 'HEAD')
            matrix = {'go_services': ['chat'], 'docker_go_services': ['chat'],
                      'integration_go_services': ['chat'], 'run_go': True, 'run_pkg': True}
            with mock.patch.object(planner, 'resolve_go_matrix', return_value=matrix) as resolver:
                plan = planner.build_plan(root, base, head, head)
            self.assertTrue(resolver.call_args.args[1]['ci_global'])
            self.assertTrue(plan['unresolved_filter_configuration'])
            self.assertNotIn('voice-history-audit', plan['checks'])
            self.assertIn('flutter-windows', plan['checks'])

    def test_hosted_comparison_runs_without_replacing_existing_selection(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['affected-check-plan']
        self.assertIn("github.event_name == 'pull_request'", job['if'])
        selector = next(step for step in job['steps'] if step.get('id') == 'filter')
        self.assertEqual(selector['with']['filters'], '.github/ci/path-filters.yml')
        self.assertEqual(selector['with']['predicate-quantifier'], 'some-with-excludes')
        command = next(step for step in job['steps'] if step.get('name') == 'Compare affected checks')
        self.assertIn('--compare-hosted', command['run'])
        self.assertIn('--event-base', command['run'])
        self.assertEqual(command['env']['CANDIDATE_SHA'], '${{ github.sha }}')
        self.assertNotIn('affected-check-plan', workflow['jobs']['ci-gate']['needs'])

    def test_required_gate_recomputes_plan_and_unknown_scope_widens_before_routing(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        steps = workflow['jobs']['changes']['steps']
        scope = next(step for step in steps if step.get('id') == 'scope')
        affected = next(step for step in steps if step.get('id') == 'affected_scope')
        self.assertLess(steps.index(affected), steps.index(scope))
        self.assertIn('--event-base', affected['run'])
        self.assertIn('steps.affected_scope.outputs.ci_global', scope['env']['AFFECTED_GLOBAL'])
        self.assertIn('AFFECTED_GLOBAL', scope['run'])
        for output in ('global', 'code', 'history_audit', 'windows_desktop'):
            self.assertIn('steps.affected_scope.outputs.unresolved',
                          workflow['jobs']['changes']['outputs'][output])
        self.assertIn('unresolved_filter_configuration != null', affected['run'])
        gate = next(step for step in workflow['jobs']['ci-gate']['steps']
                    if step.get('name') == 'Verify affected PR checks')
        self.assertIn('--verify-needs', gate['run'])
        self.assertIn('--event-base', gate['run'])
        self.assertIn('toJson(needs)', gate['env']['NEEDS_JSON'])

    def test_story_lfp_proof_follows_affected_integration_and_rejects_skips(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['story-lfp-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'story')", job['if'])
        self.assertIn('story-lfp-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Story LFP proofs')
        self.assertIn('./internal/lfpconsentstore', test['run'])
        self.assertIn('./internal/lfpconsenttransport', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.postgres.ports', test['env']['LFP_TEST_DATABASE_DSN'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['story'], 'docker_go_services': [],
                  'integration_go_services': ['story'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/story/main.go']), matrix)
        self.assertIn('story-lfp-integration', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        self.assertEqual(planner.verify_gate(selected, needs, identity), [])
        for result in ('skipped', 'failure', 'cancelled'):
            needs['story-lfp-integration']['result'] = result
            self.assertIn('story-lfp-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('story-lfp-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_user_association_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['user-association-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'user')", job['if'])
        self.assertIn('user-association-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run User association proofs')
        self.assertIn('./internal/identityassociation', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.postgres.ports', test['env']['USER_ASSOCIATION_TEST_DSN'])
        capture = next(step for step in job['steps'] if step.get('name') == 'Run User capture transport proof')
        for name in ('NativeProtectedTransport', 'CanonicalProducerNativeCompleteCohort',
                     'CanonicalProducerNativeArchiveAndRawErasure'):
            self.assertIn(name, capture['run'])
        self.assertIn('--reject-skips', capture['run'])
        self.assertIn('job.services.redis.ports', capture['env']['USER_ASSOCIATION_TEST_REDIS'])
        self.assertIn('job.services.postgres.ports', capture['env']['USER_ASSOCIATION_TEST_DSN'])
        birth = next(step for step in job['steps'] if step.get('name') == 'Run User profile birth proof')
        self.assertIn('user_birth_required.json', birth['run'])
        self.assertIn('--reject-skips', birth['run'])
        self.assertIn('job.services.postgres.ports', birth['env']['USER_ASSOCIATION_TEST_DSN'])
        admission = next(step for step in job['steps'] if step.get('name') == 'Run User admission proof')
        self.assertIn('TestAdmissionEligibility_NativeExactPairAndRevisionWriters', admission['run'])
        self.assertIn('--reject-skips', admission['run'])
        self.assertIn('job.services.redis.ports', admission['env']['USER_ADMISSION_TEST_REDIS_ADDR'])
        self.assertIn('job.services.postgres.ports', admission['env']['USER_ADMISSION_TEST_DSN'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['user'], 'docker_go_services': [],
                  'integration_go_services': ['user'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/user/main.go']), matrix)
        self.assertIn('user-association-integration', selected['checks'])
        matrix['integration_go_services'] = []
        self.assertNotIn('user-association-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_notification_quota_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['notification-quota-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'notification')", job['if'])
        self.assertIn('notification-quota-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Notification quota proofs')
        self.assertIn('./internal/chatquota', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.redis.ports', test['env']['BE195_REDIS_ADDR'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['notification'], 'docker_go_services': [],
                  'integration_go_services': ['notification'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/notification/main.go']), matrix)
        self.assertIn('notification-quota-integration', selected['checks'])
        matrix['integration_go_services'] = []
        self.assertNotIn('notification-quota-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_go_integration_evidence_is_required_for_selected_matrix(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['backend-go-integration-evidence']
        self.assertIn('backend-go-integration-pr', job['needs'])
        self.assertIn('backend-go-integration-evidence', workflow['jobs']['ci-gate']['needs'])
        verify = next(step for step in job['steps'] if step.get('name') == 'Verify selected Go integration evidence')
        self.assertIn('verify-diagnostics', verify['run'])
        self.assertIn('go_integration_required.json', verify['run'])
        self.assertIn('integration_go_services', verify['env']['SERVICES'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['chat'], 'docker_go_services': [],
                  'integration_go_services': ['chat'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)
        self.assertIn('backend-go-integration-evidence', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        for outcome in ('skipped', 'failure', 'cancelled'):
            needs['backend-go-integration-evidence']['result'] = outcome
            self.assertIn('backend-go-integration-evidence', planner.verify_gate(selected, needs, identity))
        del needs['backend-go-integration-evidence']
        self.assertIn('backend-go-integration-evidence', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('backend-go-integration-evidence', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_role_space_receipt_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['role-space-receipt-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'role')", job['if'])
        self.assertIn('role-space-receipt-integration', workflow['jobs']['ci-gate']['needs'])
        step = next(step for step in job['steps'] if step.get('name') == 'Run Role Space receipt proofs')
        self.assertIn('-tags=role_space_native', step['run'])
        self.assertIn('role_space_receipt_required.json', step['run'])
        self.assertIn('--reject-skips', step['run'])
        self.assertIn('github.workspace', step['env']['ROLE_NATIVE_SPACE_SOURCE_ROOT'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['role'], 'docker_go_services': [],
                  'integration_go_services': ['role'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/role/main.go']), matrix)
        self.assertIn('role-space-receipt-integration', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        for outcome in ('skipped', 'failure', 'cancelled'):
            needs['role-space-receipt-integration']['result'] = outcome
            self.assertIn('role-space-receipt-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('role-space-receipt-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_t31_shared_and_contract_inputs_are_required_by_gate(self):
        import yaml
        workflow = yaml.load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'), Loader=yaml.BaseLoader)
        standalone = yaml.load((ROOT / '.github/workflows/t31-session-events-e2e.yml').read_text(encoding='utf-8'), Loader=yaml.BaseLoader)
        self.assertIn('workflow_call', standalone['on'])
        self.assertIn('github.workflow', standalone['concurrency']['group'])
        job = workflow['jobs']['t31-critical']
        self.assertEqual(job['uses'], './.github/workflows/t31-session-events-e2e.yml')
        self.assertIn('needs.changes.outputs.t31', job['if'])
        self.assertIn('t31-critical', workflow['jobs']['ci-gate']['needs'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        self.assertEqual(set(rules['t31']), set(standalone['on']['pull_request']['paths']))
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        for path in ('src/backend/pkg/principal/verifier.go', 'protos/voice/chat/v1/chat.proto',
                     'src/backend/migrations/chat_db/000001_initial.up.sql',
                     'docs/testing/game-integrations-acceptance.md', 'new-area/executable.sh'):
            with self.subTest(path=path):
                selected = planner.make_plan(planner.match_filters(rules, [path]), matrix)
                self.assertIn('t31-critical', selected['checks'])
                identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
                selected.update(schema='voice-affected-check-plan-v1', identity=identity)
                needs = {name: {'result': 'success'} for name in selected['checks']}
                for outcome in ('skipped', 'failure', 'cancelled'):
                    needs['t31-critical']['result'] = outcome
                    self.assertIn('t31-critical', planner.verify_gate(selected, needs, identity))
                del needs['t31-critical']
                self.assertIn('t31-critical', planner.verify_gate(selected, needs, identity))
        selected = planner.make_plan(planner.match_filters(rules, ['docs/TESTING.md']), matrix)
        self.assertNotIn('t31-critical', selected['checks'])

    def test_t31_backend_catchall_does_not_hide_unknown_executable(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        path = 'src/backend/new-global-tool.py'
        matched = planner.match_filters(rules, [path])
        self.assertIn(path, matched['t31'])
        self.assertIn(path, planner.unclassified_paths(matched))
        self.assertTrue(planner.effective_filters(matched)['ci_global'])

    def test_local_only_flutter_diagnostics_do_not_widen_hosted_scope(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        for path in ('scripts/ci/local_source_snapshot.py',
                     'scripts/ci/local_source_snapshot_test.py',
                     'scripts/ci/local_flutter_execution.py',
                     'scripts/ci/local_flutter_execution_test.py'):
            with self.subTest(path=path):
                matched = planner.match_filters(rules, [path])
                plan = planner.make_plan(matched, matrix)
                self.assertFalse(plan['filters']['ci_global'])
                self.assertIn('ci-script-tests', plan['checks'])
                self.assertNotIn('flutter', plan['checks'])
                self.assertNotIn('backend-go', plan['checks'])
        mixed = planner.match_filters(rules, ['scripts/ci/local_flutter_execution.py',
                                              'scripts/ci/new_unclassified_helper.py'])
        self.assertTrue(planner.effective_filters(mixed)['ci_global'])

    def test_chat_auth_java_proof_follows_auth_chat_shared_and_contract_inputs(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        for path, integration, run_pkg in (
                ('src/backend/auth/pom.xml', [], False),
                ('src/backend/chat/main.go', ['chat'], False),
                ('src/backend/pkg/go.mod', ['chat'], True),
                ('protos/voice/auth/v1/auth.proto', ['chat'], True),
                ('scripts/ci/auth_candidate_classpath.py', [], False)):
            matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': integration,
                      'run_go': bool(integration), 'run_pkg': run_pkg}
            with self.subTest(path=path):
                plan = planner.make_plan(planner.match_filters(rules, [path]), matrix)
                self.assertIn('chat-auth-java-integration', plan['checks'])
                if path.startswith('scripts/ci/auth_candidate_'):
                    self.assertFalse(plan['filters']['ci_global'])
                    self.assertNotIn('f13-historical-replay', plan['checks'])
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        plan = planner.make_plan(planner.match_filters(rules, ['src/frontend/lib/main.dart']), matrix)
        self.assertNotIn('chat-auth-java-integration', plan['checks'])

    def test_f13_replay_is_selected_for_its_inputs_and_unknowns_only(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        def checks(path):
            return planner.make_plan(planner.match_filters(rules, [path]), matrix)['checks']
        for path in ('scripts/ci/f13_exact_j_historical_red_test.py',
                     'scripts/ci/f13_f54_history.py',
                     'scripts/ci/testdata/f13-exact-j-547/source-unit.json',
                     'scripts/ci/voice-db-runtime-provisioning-contract_test.sh',
                     '.agent/execplans/native-p3-master-source.md', 'Makefile',
                     '.github/workflows/ci.yml', 'scripts/ci/new_unknown_tool.py'):
            with self.subTest(path=path):
                self.assertIn('f13-historical-replay', checks(path))
        self.assertNotIn('f13-historical-replay', checks('scripts/ci/local_flutter_execution.py'))
        for name in ('local_process_supervision.py', 'local_process_supervision_test.py'):
            with self.subTest(process_supervision=name):
                self.assertNotIn('f13-historical-replay', checks('scripts/ci/' + name))
                self.assertIn('ci-script-tests', checks('scripts/ci/' + name))
        for name in ('local_go_dependencies.py', 'local_go_dependencies_test.py',
                     'local_go_execution.py', 'local_go_execution_test.py',
                     'local_go_plan.py', 'local_go_plan_test.py', 'local_source_snapshot.py',
                     'local_current_diff.py', 'local_current_diff_test.py',
                     'local_ci_routing.py', 'local_ci_routing_test.py',
                     'local_ci_matrix_inventory.py', 'local_ci_matrix_inventory_test.py',
                     'local_ci_staging_dispatch.py', 'local_ci_staging_dispatch_test.py',
                     'local_ci_staging_network.py', 'local_ci_staging_network_test.py',
                     'local_ci_staging_request.py', 'local_ci_staging_request_test.py',
                     'local_ci_staging_worker.py', 'local_ci_staging_worker_test.py',
                     'local_ci_staging_execution.py', 'local_ci_staging_execution_test.py',
                     'local_preflight_staging_test.py',
                     'local_nats_hosted_diagnostic.py', 'local_nats_hosted_diagnostic_test.py',
                     'local_staging_owned_tmp.py', 'local_staging_owned_tmp_test.py'):
            with self.subTest(local_go=name):
                self.assertNotIn('f13-historical-replay', checks('scripts/ci/' + name))
                self.assertIn('ci-script-tests', checks('scripts/ci/' + name))
        for name in ('local_go_execution.py', 'local_go_dependencies.py', 'local_go_plan.py', 'local_source_snapshot.py',
                     'local_rootless_profile.py', 'local_rootless_profile_test.py'):
            with self.subTest(docker_consumer=name):
                self.assertIn('local-docker-isolation', checks('scripts/ci/' + name))
        for name in ('local_rootless_profile.py', 'local_rootless_profile_test.py',
                     'local_chat_private.py', 'local_chat_private_test.py',
                     'local_chat_orchestration.py', 'local_chat_orchestration_test.py',
                     'local_auth_private.py', 'local_auth_private_test.py', 'local_auth_orchestration.py',
                     'local_compose_private.py', 'local_compose_private_test.py',
                     'local_nats_hosted_diagnostic.py', 'local_nats_hosted_diagnostic_test.py',
                     'local_ci_staging_dispatch.py', 'local_ci_staging_dispatch_test.py',
                     'local_ci_staging_network.py', 'local_ci_staging_network_test.py',
                     'local_ci_staging_request.py', 'local_ci_staging_request_test.py',
                     'local_ci_staging_worker.py', 'local_ci_staging_worker_test.py',
                     'local_ci_staging_execution.py', 'local_ci_staging_execution_test.py',
                     'local_preflight_staging_test.py',
                     'local_staging_owned_tmp.py', 'local_staging_owned_tmp_test.py',
                     'local_compose_bridge.py', 'local_compose_bridge_test.py',
                     'local_image_archive.py', 'local_image_archive_test.py',
                     'local_docker_owned.py', 'local_docker_owned_test.py',
                     'local_docker_scheduled.py', 'local_docker_scheduled_test.py',
                     'local_docker_scheduled_exec_integration_test.py'):
            with self.subTest(unrelated=name):
                self.assertNotIn('f13-historical-replay', checks('scripts/ci/' + name))
                self.assertIn('local-docker-isolation', checks('scripts/ci/' + name))

    def test_browser_event_parser_selects_hosted_flutter_checks(self):
        import yaml
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        for path in ('scripts/ci/flutter_test_events.py', 'scripts/ci/flutter_test_events_test.py'):
            plan = planner.make_plan(planner.match_filters(rules, [path]), matrix)
            self.assertIn('flutter', plan['checks'])
            self.assertIn('ci-script-tests', plan['checks'])
            self.assertNotIn('backend-go', plan['checks'])
            self.assertNotIn('flutter-music-proof', plan['checks'])
            self.assertNotIn('flutter-help-proof', plan['checks'])
            self.assertNotIn('flutter-startup-proof', plan['checks'])
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        step = next(s for s in workflow['jobs']['flutter']['steps'] if s.get('name') == 'Run browser-only Flutter tests')
        for name in ('app_shell_icon_web_test.dart', 'linked_oauth_browser_web_test.dart',
                     'presigned_upload_web_test.dart', 'chat_draft_browser_storage_test.dart'):
            self.assertIn(name, step['run'])
        self.assertIn('flutter_test_events.py', step['run'])
        self.assertNotIn('--allow-skips', step['run'])
        self.assertNotIn('--concurrency', step['run'])

    def test_a1_proofs_select_their_inputs_without_widening_unrelated_frontend_tests(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        def checks(path):
            return planner.make_plan(planner.match_filters(rules, [path]), matrix)['checks']
        for file in ('compose_a1_multi_account_live_test.go', 'compose_session_epoch_live_test.go',
                     'compose_threads_e2e_live_test.go', 'compose_live_helpers_test.go',
                     'compose_users_search_live_test.go', 'compose_auth_guest_nats_sessions_live_test.go'):
            with self.subTest(file=file):
                self.assertIn('a1-e2e', checks('src/backend/gateway/' + file))
        for file in ('t055_profile_switch_reconnect_inbox_e2e_live_test.dart',
                     't106_account_soft_delete_e2e_live_test.dart',
                     't107_folders_quick_access_e2e_live_test.dart',
                     'a1_pending_email_reentry_e2e_live_test.dart'):
            with self.subTest(file=file):
                self.assertIn('a1-flutter-profile-handoff', checks('src/frontend/test/' + file))
        for path in ('src/frontend/lib/backend/auth_client.dart',
                     'src/frontend/lib/state/profile_switch_coordinator.dart',
                     'src/frontend/lib/ui/chat/chat_room_panel.dart',
                     'src/frontend/test/support/live_gateway_harness.dart', 'src/frontend/pubspec.lock'):
            with self.subTest(path=path):
                self.assertIn('a1-flutter-profile-handoff', checks(path))
        for path in ('src/backend/auth/src/main/Auth.java', 'src/backend/user/main.go',
                     'src/backend/chat/main.go', 'src/backend/messaging/main.go',
                     'src/backend/social/main.go', 'src/backend/gateway/main.go',
                     'src/backend/realtime/main.go', 'src/backend/pkg/principal/verifier.go',
                     'src/backend/migrations/messaging_db/000001_initial.up.sql',
                     'protos/voice/chat/v1/chat.proto'):
            with self.subTest(path=path):
                self.assertIn('a1-e2e', checks(path))
                self.assertIn('a1-flutter-profile-handoff', checks(path))
        unrelated = 'src/frontend/test/bot_install_page_test.dart'
        matched = planner.match_filters(rules, [unrelated])
        self.assertEqual(planner.unclassified_paths(matched), [])
        self.assertFalse(planner.effective_filters(matched)['ci_global'])
        self.assertNotIn('a1-e2e', checks(unrelated))
        self.assertNotIn('a1-flutter-profile-handoff', checks(unrelated))
        self.assertIn('a1-flutter-profile-handoff', checks('scripts/ci/new_unknown_tool.py'))
        self.assertTrue(planner.effective_filters(planner.match_filters(
            rules, [unrelated, 'scripts/ci/new_unknown_tool.py']))['ci_global'])

    def test_every_catalogued_live_flutter_suite_has_premerge_coverage_for_its_own_input(self):
        import yaml
        catalog = yaml.safe_load((ROOT / '.github/ci/e2e-features.yml').read_text(encoding='utf-8'))
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        for suite in catalog['full_flutter']:
            with self.subTest(suite=suite):
                matched = planner.match_filters(rules, ['src/frontend/' + suite])
                checks = planner.make_plan(matched, matrix)['checks']
                self.assertTrue('compose-e2e' in checks or
                                (suite in catalog['a1_flutter_profile_handoff'] and
                                 'a1-flutter-profile-handoff' in checks), suite)

    def test_a1_workflow_and_gate_consume_shared_selectors(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        jobs = workflow['jobs']
        self.assertIn('a1_multi_account_pr', jobs['changes']['outputs'])
        self.assertIn('a1_multi_account_pr', jobs['a1-e2e']['if'])
        self.assertIn('a1_t106_live', jobs['a1-flutter-profile-handoff']['if'])
        self.assertIn('ci_global', jobs['a1-flutter-profile-handoff']['if'])
        gate = next(step for step in jobs['ci-gate']['steps'] if 'RUN_A1_E2E' in step.get('env', {}))
        self.assertIn('a1_multi_account_pr', gate['env']['RUN_A1_E2E'])
        self.assertIn('ci_global', gate['env']['RUN_A1_FLUTTER_PROFILE_HANDOFF'])
        bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else 'bash'
        for selector, result in (('RUN_A1_E2E', 'JOB_A1_E2E'),
                                 ('RUN_A1_FLUTTER_PROFILE_HANDOFF', 'JOB_A1_FLUTTER_PROFILE_HANDOFF')):
            for outcome, expected in (('success', 0), ('skipped', 1), ('failure', 1), ('cancelled', 1)):
                env = {'PATH': os.environ['PATH'], 'JOB_CURRENT_DIFF_CONTRACT': 'success',
                       selector: 'true', result: outcome}
                run = subprocess.run([bash, str(ROOT / '.github/ci/verify-required-jobs.sh'), 'pull_request', 'false'],
                                     env=env, capture_output=True)
                self.assertEqual(run.returncode, expected, (selector, outcome, run.stderr))

    def test_music_selector_keeps_shared_and_unknown_inputs_but_excludes_independent_browser_tests(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        browser_tests = ['src/frontend/test/' + name for name in
                         ('app_shell_icon_web_test.dart', 'chat_draft_browser_storage_test.dart',
                          'linked_oauth_browser_web_test.dart', 'presigned_upload_web_test.dart')]
        for path in browser_tests:
            plan = planner.make_plan(planner.match_filters(rules, [path]), matrix)
            self.assertIn('flutter', plan['checks'])
            self.assertNotIn('flutter-music-proof', plan['checks'])
            self.assertNotIn('flutter-help-proof', plan['checks'])
            self.assertNotIn('flutter-startup-proof', plan['checks'])
        for paths in (['src/frontend/lib/ui/chat/music_playback.dart'],
                      ['src/frontend/test/support/e2e_attachment_test_fixture.dart'],
                      ['src/frontend/pubspec.lock'], ['protos/voice/message/v1/message.proto'],
                      ['src/frontend/test/new_unknown_test.dart'], ['scripts/ci/new_unknown_helper.py'],
                      [browser_tests[0], 'src/frontend/lib/ui/chat/music_playback.dart']):
            with self.subTest(paths=paths):
                plan = planner.make_plan(planner.match_filters(rules, paths), matrix)
                self.assertIn('flutter-music-proof', plan['checks'])
                self.assertIn('flutter-help-proof', plan['checks'])
                self.assertIn('flutter-startup-proof', plan['checks'])

    def test_full_dispatch_forces_code_even_after_docs_only_changes(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        changes = workflow['jobs']['changes']
        required = "github.event_name == 'workflow_dispatch' && inputs.profile == 'full'"
        self.assertIn(required, changes['outputs']['code'])
        matrix = next(step for step in changes['steps'] if step.get('id') == 'stagingmatrix')
        self.assertIn(required, matrix['env']['FILTER_CODE'])
        self.assertIn(required, matrix['env']['FORCE_FULL'])

    def test_help_and_startup_proofs_preserve_checks_and_are_required(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        original = workflow['jobs']['flutter']
        names = (('flutter-help-proof', 'Help real Chrome external handoff (H/V)'),
                 ('flutter-startup-proof', 'Web startup lifecycle and real browser retry (H/V)'))
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        plan = planner.make_plan(planner.match_filters(rules, ['src/frontend/lib/main.dart']), matrix)
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        plan.update(schema='voice-affected-check-plan-v1', identity=identity)
        for job_name, step_name in names:
            self.assertFalse(any(step.get('name') == step_name for step in original['steps']))
            comparison = workflow['jobs'][job_name]
            new_step = next(step for step in comparison['steps'] if step.get('name') == step_name)
            self.assertEqual(new_step['env']['CHROME_BIN'], 'google-chrome')
            self.assertIn(job_name, workflow['jobs']['ci-gate']['needs'])
            self.assertIn(job_name, plan['checks'])
            needs = {name: {'result': 'success'} for name in plan['checks']}
            for result in ('failure', 'skipped', 'cancelled'):
                needs[job_name] = {'result': result}
                self.assertIn(job_name, planner.verify_gate(plan, needs, identity))
            del needs[job_name]
            self.assertIn(job_name, planner.verify_gate(plan, needs, identity))
        startup = workflow['jobs']['flutter-startup-proof']
        build = next(step['run'] for step in startup['steps']
                     if step.get('name') == 'Build the Help launcher fixture for startup')
        help_step = next(step['run'] for step in workflow['jobs']['flutter-help-proof']['steps']
                         if step.get('name') == 'Help real Chrome external handoff (H/V)')
        for source in (build, help_step):
            self.assertIn('--target test/fixtures/settings_help_web_launcher_smoke.dart', source)
            self.assertIn('--output build/help-launcher-web', source)
            self.assertIn('--no-web-resources-cdn', source)
        self.assertIn('flutter pub get', build)
        self.assertIn('scripts/ci/settings-help-browser-handoff.mjs', help_step)
        startup_step = next(step['run'] for step in startup['steps']
                            if step.get('name') == 'Web startup lifecycle and real browser retry (H/V)')
        self.assertIn('node --test scripts/ci/web-startup.test.mjs', startup_step)
        self.assertIn('scripts/ci/web-startup-browser.mjs', startup_step)
        for name, artifact in (('flutter-help-proof', 'settings-help-browser-${{ github.run_id }}'),
                               ('flutter-startup-proof', 'web-startup-browser-${{ github.run_id }}')):
            self.assertTrue(any(step.get('with', {}).get('name') == artifact
                                for step in workflow['jobs'][name]['steps']))

    def test_isolated_music_proof_preserves_steps_and_is_required(self):
        import yaml
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        plan = planner.make_plan(planner.match_filters(rules, ['src/frontend/lib/ui/chat/music_playback.dart']), matrix)
        self.assertIn('flutter', plan['checks'])
        self.assertIn('flutter-music-proof', plan['checks'])
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        original = workflow['jobs']['flutter']
        comparison = workflow['jobs']['flutter-music-proof']
        for name in ('Capture Music inline controls', 'Verify Music codecs and E2E local playback in Chrome'):
            self.assertFalse(any(step.get('name') == name for step in original['steps']))
            self.assertTrue(any(step.get('name') == name for step in comparison['steps']))
        self.assertTrue(any(step.get('name') == 'Retain hosted Music dependency resolution'
                            for step in comparison['steps']))
        self.assertIn('flutter-music-proof', workflow['jobs']['ci-gate']['needs'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        plan.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in plan['checks']}
        needs['flutter-unit-receipts'] = {'result': 'success'}
        self.assertEqual(planner.verify_gate(plan, needs, identity), [])
        for result in ('skipped', 'cancelled', 'failure'):
            needs['flutter-music-proof'] = {'result': result}
            self.assertIn('flutter-music-proof', planner.verify_gate(plan, needs, identity))
        del needs['flutter-music-proof']
        self.assertIn('flutter-music-proof', planner.verify_gate(plan, needs, identity))

    def test_realtime_lifecycle_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['realtime-lifecycle-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'realtime')", job['if'])
        self.assertIn('realtime-lifecycle-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Realtime lifecycle proofs')
        self.assertIn('-tags=realtimep3integration', test['run'])
        self.assertIn('realtime_lifecycle_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.postgres.ports', test['env']['REALTIME_P3_TEST_DATABASE_URL'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['realtime'], 'docker_go_services': [],
                  'integration_go_services': ['realtime'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/realtime/main.go']), matrix)
        self.assertIn('realtime-lifecycle-integration', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        for outcome in ('skipped', 'failure', 'cancelled'):
            needs['realtime-lifecycle-integration']['result'] = outcome
            self.assertIn('realtime-lifecycle-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('realtime-lifecycle-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/realtime/main.go']), matrix)['checks'])

    def test_chat_lifecycle_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['chat-lifecycle-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'chat')", job['if'])
        self.assertIn('chat-lifecycle-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Chat lifecycle proof')
        self.assertIn('-tags=realtimep3integration', test['run'])
        self.assertIn('chat_lifecycle_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.postgres.ports', test['env']['REALTIME_P3_TEST_DATABASE_URL'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['chat'], 'docker_go_services': [],
                  'integration_go_services': ['chat'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)
        self.assertIn('chat-lifecycle-integration', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        for outcome in ('skipped', 'failure', 'cancelled'):
            needs['chat-lifecycle-integration']['result'] = outcome
            self.assertIn('chat-lifecycle-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('chat-lifecycle-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_space_autovacuum_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['space-autovacuum-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'space')", job['if'])
        self.assertIn('space-autovacuum-integration', workflow['jobs']['ci-gate']['needs'])
        prepare = next(step for step in job['steps'] if step.get('name') == 'Configure owned PostgreSQL autovacuum')
        self.assertIn('job.services.postgres.id', prepare['env']['PG_CONTAINER'])
        self.assertIn('autovacuum_naptime', prepare['run'])
        self.assertIn("autovacuum_vacuum_cost_delay = '100ms'", prepare['run'])
        self.assertIn("autovacuum_vacuum_cost_limit = '1'", prepare['run'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Space real-autovacuum proof')
        self.assertIn('-tags=integration', test['run'])
        self.assertIn('space_autovacuum_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.postgres.ports', test['env']['NATIVE_INSTALLER_WINDOW_TEST_ADMIN_URL'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['space'], 'docker_go_services': [],
                  'integration_go_services': ['space'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/space/main.go']), matrix)
        self.assertIn('space-autovacuum-integration', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        needs['space-autovacuum-integration']['result'] = 'skipped'
        self.assertIn('space-autovacuum-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('space-autovacuum-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_space_native_installer_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['space-native-installer-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'space')", job['if'])
        self.assertIn('space-native-installer-integration', workflow['jobs']['ci-gate']['needs'])
        build = next(step for step in job['steps'] if step.get('name') == 'Build source-bound Space installer')
        self.assertIn('Dockerfile.native-migrate', build['run'])
        self.assertIn('SOURCE_REVISION', build['run'])
        self.assertIn('testdata/evidencefailure', build['run'])
        self.assertIn('BuildSourceSHA256', build['run'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Space native installer proof')
        self.assertIn('TestProductionFreshChainAndRetainedVerify', test['run'])
        self.assertIn('TestCommittedPhaseFailureNeverAutoResumes', test['run'])
        self.assertIn('NATIVE_INSTALLER_TEST_FAILURE_HELPER', test['env'])
        self.assertIn('space_native_installer_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        inventory = json.loads((ROOT / 'scripts/ci/space_native_installer_required.json').read_text(encoding='utf-8'))
        required = {name for _, name in inventory['space']}
        for phase in ('created', 'migrated44', 'trusted44', 'migrated45', 'trusted45',
                      'migrated50', 'trusted50', 'trusted51', 'complete'):
            self.assertIn('TestCommittedPhaseFailureNeverAutoResumes/' + phase, required)
        self.assertIn('job.services.postgres.ports', test['env']['NATIVE_INSTALLER_TEST_ADMIN_URL'])

    def test_over100_proof_inputs_select_the_gated_space_job(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        for path in ('src/backend/space/internal/store/account_ban.go',
                     'src/backend/migrations/space_db/000044_native_membership_readiness.up.sql',
                     'src/backend/pkg/spaceaccountcapture/capture.go',
                     'protos/voice/space/v1/space.proto',
                     'scripts/ci/space_over100_proof.sh',
                     'scripts/ci/space_over100_consumer_required.json'):
            with self.subTest(path=path):
                matched = planner.match_filters(rules, [path])
                matrix = planner.resolve_go_matrix(ROOT, planner.effective_filters(matched))
                selected = planner.make_plan(matched, matrix)
                self.assertIn('space-native-installer-integration', selected['checks'])
                self.assertIn('space', selected['components']['go_integration'])

    def test_space_trusted50_requires_atomic_rollback_and_fences(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['space-native-installer-integration']
        step = next(step for step in job['steps'] if step.get('name') == 'Run trusted Space native50 catalog proofs')
        inventory = json.loads((ROOT / 'scripts/ci/space_trusted50_required.json').read_text(encoding='utf-8'))
        required = {name for package, name in inventory['space'] if package == 'voice/backend/space/internal/store'}
        root = 'TestNative51AtomicMigrationFailureRollsBackBodySealAndHistory'
        for name in (root, 'TestNative51FrameworkUnlockRetainsTransactionFences'):
            self.assertIn(name, step['run'])
            self.assertIn(name, required)
        for phase in ('after_dirty_marker', 'after_body', 'after_CAS_and_clean_marker',
                      'engine_unlock_error', 'outer_window_failure'):
            self.assertIn(root + '/' + phase, required)
        stale = 'TestNative51StaleExpectedParentCannotCommit'
        self.assertIn(stale, step['run'])
        for name in (stale, stale + '/epoch', stale + '/seal'):
            self.assertIn(name, required)
        self.assertIn('--reject-skips', step['run'])
        commit = 'TestNative51FixtureCommitTrusted50'
        self.assertIn(commit, step['run'])
        self.assertIn('space_native51_commit_required.json', step['run'])
        self.assertLess(step['run'].index('--output "$RUNNER_TEMP/space-trusted50-receipt.json"'),
                        step['run'].index(commit))
        commit_inventory = json.loads((ROOT / 'scripts/ci/space_native51_commit_required.json').read_text(encoding='utf-8'))
        self.assertEqual(commit_inventory['space'], [['voice/backend/space/internal/store', commit]])

    def test_space_native_installer_runs_missing_capture_guard(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['space-native-installer-integration']
        test = next(step for step in job['steps'] if step.get('name') == 'Run Space account-ban capture guard')
        self.assertIn('-mode provision', test['run'])
        self.assertIn('-tags=spacebanintegration', test['run'])
        self.assertIn('TestAccountBanCannotSucceedWithoutCompleteOwnerCapture', test['run'])
        self.assertIn('TestNative51CreatorBornCurrentSchema', test['run'])
        self.assertIn('TestAccountBanCatalogRejectsSemanticDrift', test['run'])
        self.assertIn('TestNative51ProofDiagnosticEnums', test['run'])
        self.assertIn('space_account_ban_guard_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('SPACE_NATIVE_ADMIN_URL', test['env'])
        self.assertIn('SPACE_BAN_TEST_DATABASE_URL', test['env'])
        inventory = json.loads((ROOT / 'scripts/ci/space_account_ban_guard_required.json').read_text(encoding='utf-8'))
        required = {name for package, name in inventory['space']
                    if package == 'voice/backend/space/internal/store'}
        self.assertIn('TestAccountBanCannotSucceedWithoutCompleteOwnerCapture', required)
        self.assertIn('TestNative51CreatorBornCurrentSchema', required)
        root = 'TestAccountBanCatalogRejectsSemanticDrift'
        self.assertIn(root, required)
        for case in ('serving_login_needs_no_private_schema_access', 'guard_predicate',
                     'guard_event_columns', 'internal_foreign_key_disabled',
                     'column_public_privilege', 'table_public_privilege',
                     'private_schema_relation', 'unexpected_prefixed_relation'):
            self.assertIn(root + '/' + case, required)
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['space'], 'docker_go_services': [],
                  'integration_go_services': ['space'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/space/main.go']), matrix)
        self.assertIn('space-native-installer-integration', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        for outcome in ('skipped', 'failure', 'cancelled'):
            needs['space-native-installer-integration']['result'] = outcome
            self.assertIn('space-native-installer-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('space-native-installer-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_local_docker_lifecycle_proof_is_selected_and_gated(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['local-docker-isolation']
        self.assertIn('local_docker_isolation', job['if'])
        self.assertIn('local-docker-isolation', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Exercise owned Docker lifecycle')
        self.assertEqual(test['env']['VOICE_LOCAL_DOCKER_TEST'], '1')
        self.assertIn('local_docker_scheduled_integration_test.py', test['run'])
        self.assertIn('local_docker_owned_cancel_test.py', test['run'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        matched = planner.match_filters(rules, ['scripts/ci/local_docker_scheduled.py'])
        self.assertEqual(matched['local_docker_isolation'], ['scripts/ci/local_docker_scheduled.py'])
        creator = 'src/backend/pkg/cmd/ci-rootless-create/main.go'
        self.assertEqual(planner.match_filters(rules, [creator])['local_docker_isolation'], [creator])
        selected = planner.make_plan(matched, matrix)
        self.assertIn('local-docker-isolation', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        for outcome in ('skipped', 'failure', 'cancelled'):
            needs['local-docker-isolation']['result'] = outcome
            self.assertIn('local-docker-isolation', planner.verify_gate(selected, needs, identity))
        unknown = planner.match_filters(rules, ['scripts/ci/local_docker_unknown.py'])
        self.assertTrue(planner.effective_filters(unknown)['ci_global'])

    def test_messaging_threadlist_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['messaging-threadlist-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'messaging')", job['if'])
        self.assertIn('messaging-threadlist-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Messaging thread-list proof')
        self.assertIn('messaging_threadlist_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.postgres.ports', test['env']['THREADLIST_TEST_DATABASE_URL'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['messaging'], 'docker_go_services': [],
                  'integration_go_services': ['messaging'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/messaging/main.go']), matrix)
        self.assertIn('messaging-threadlist-integration', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        needs['messaging-threadlist-integration']['result'] = 'skipped'
        self.assertIn('messaging-threadlist-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('messaging-threadlist-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_voice_ledger_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['voice-ledger-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'voice')", job['if'])
        self.assertIn('voice-ledger-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Voice Redis ledger proof')
        self.assertIn('voice_ledger_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.redis.ports', test['env']['VOICE_R18_REDIS_TEST_ADDR'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['voice'], 'docker_go_services': [],
                  'integration_go_services': ['voice'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/voice/main.go']), matrix)
        self.assertIn('voice-ledger-integration', selected['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        needs['voice-ledger-integration']['result'] = 'skipped'
        self.assertIn('voice-ledger-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('voice-ledger-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_bot_membership_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['bot-membership-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'bot')", job['if'])
        self.assertIn('bot-membership-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Bot membership proofs')
        self.assertIn('bot_membership_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.postgres.ports', test['env']['BOT_MEMBERSHIP_TEST_DATABASE_URL'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['bot'], 'docker_go_services': [],
                  'integration_go_services': ['bot'], 'run_go': True, 'run_pkg': True}
        self.assertIn('bot-membership-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/bot/main.go']), matrix)['checks'])
        matrix['integration_go_services'] = []
        self.assertNotIn('bot-membership-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_role_migration_proof_follows_affected_integration(self):
        import yaml
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['role-migration-integration']
        self.assertIn("contains(fromJSON(needs.changes.outputs.integration_go_services), 'role')", job['if'])
        self.assertIn('role-migration-integration', workflow['jobs']['ci-gate']['needs'])
        test = next(step for step in job['steps'] if step.get('name') == 'Run Role migration proofs')
        self.assertIn('TestRole13', test['run'])
        self.assertIn('--reject-skips', test['run'])
        self.assertIn('job.services.postgres.ports', test['env']['ROLE_MIGRATION_TEST_DSN'])
        bootstrap = next(step for step in job['steps'] if step.get('name') == 'Run Role bootstrap proof')
        self.assertIn('role_bootstrap_required.json', bootstrap['run'])
        self.assertIn('--reject-skips', bootstrap['run'])
        self.assertIn('job.services.postgres.ports', bootstrap['env']['VOICE_ROLE_BOOTSTRAP_PG_DSN'])
        self.assertIn('job.services.redis.ports', bootstrap['env']['VOICE_ROLE_BOOTSTRAP_REDIS_ADDR'])
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matrix = {'go_services': ['role'], 'docker_go_services': [],
                  'integration_go_services': ['role'], 'run_go': True, 'run_pkg': True}
        selected = planner.make_plan(planner.match_filters(rules, ['src/backend/role/main.go']), matrix)
        self.assertIn('role-migration-integration', selected['checks'])
        migration = planner.match_filters(rules, ['src/backend/migrations/role_db/000013_game_session_grants.down.sql'])
        self.assertTrue(migration['svc_role'])
        self.assertIn('role-migration-integration', planner.make_plan(migration, matrix)['checks'])
        identity = {'base': 'a' * 40, 'head': 'b' * 40, 'candidate': 'c' * 40}
        selected.update(schema='voice-affected-check-plan-v1', identity=identity)
        needs = {name: {'result': 'success'} for name in selected['checks']}
        self.assertEqual(planner.verify_gate(selected, needs, identity), [])
        needs['role-migration-integration']['result'] = 'skipped'
        self.assertIn('role-migration-integration', planner.verify_gate(selected, needs, identity))
        matrix['integration_go_services'] = []
        self.assertNotIn('role-migration-integration', planner.make_plan(
            planner.match_filters(rules, ['src/backend/chat/main.go']), matrix)['checks'])

    def test_cli_uses_committed_candidate_and_actual_go_resolver(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            files = [ROOT / '.github/ci/path-filters.yml',
                     ROOT / 'scripts/ci/affected_check_plan.py',
                     ROOT / 'scripts/ci/resolve-go-matrix.sh',
                     ROOT / 'scripts/ci/resolve-go-docker-matrix.py']
            files += list((ROOT / 'src/backend').glob('*/Dockerfile'))
            files += list((ROOT / 'src/backend').glob('*/go.mod'))
            for source in files:
                target = root / source.relative_to(ROOT)
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(source, target)
            def git(*args):
                return subprocess.check_output(['git', '-C', d, *args], text=True).strip()
            git('init', '-q')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            git('add', '.')
            git('commit', '-qm', 'base')
            base = git('rev-parse', 'HEAD')
            (root / 'src/backend/chat/example.go').write_text('package chat\n', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'head')
            head = git('rev-parse', 'HEAD')
            env = dict(os.environ, PYTHONDONTWRITEBYTECODE='1')
            output = subprocess.check_output([sys.executable, str(root / 'scripts/ci/affected_check_plan.py'),
                                              '--base', base, '--head', head, '--candidate', head,
                                              '--head-ref', 'codex/example'],
                                             cwd=root, env=env, text=True)
            plan = json.loads(output)
            self.assertEqual(plan['identity']['candidate'], head)
            self.assertEqual(plan['changed_paths'], ['src/backend/chat/example.go'])
            self.assertIn('chat', plan['components']['go_integration'])
            self.assertIn('backend-go-integration-pr', plan['checks'])
            matched = planner.match_filters(planner.load_filters(root / '.github/ci/path-filters.yml'),
                                            plan['changed_paths'])
            env['HOSTED_FILTER_JSON'] = json.dumps({name: str(bool(paths)).lower()
                                                    for name, paths in matched.items()})
            hosted = json.loads(subprocess.check_output(
                [sys.executable, str(root / 'scripts/ci/affected_check_plan.py'), '--base', base,
                 '--head', head, '--candidate', head, '--head-ref', 'codex/example', '--compare-hosted'],
                cwd=root, env=env, text=True))
            comparison = hosted.pop('hosted_comparison')
            self.assertEqual(hosted, plan)
            self.assertEqual(comparison['filter_disagreements'], [])
            self.assertEqual(comparison['hosted_only_checks'], [])
            needs = {name: {'result': 'success'} for name in plan['checks']}
            needs['flutter-unit-receipts'] = {'result': 'success'}
            env['NEEDS_JSON'] = json.dumps(needs)
            verify_cmd = [sys.executable, str(root / 'scripts/ci/affected_check_plan.py'),
                          '--base', base, '--head', head, '--candidate', head,
                          '--head-ref', 'codex/example', '--verify-needs']
            subprocess.run(verify_cmd, cwd=root, env=env, check=True, capture_output=True)
            needs['backend-go-integration-pr'] = {'result': 'skipped'}
            env['NEEDS_JSON'] = json.dumps(needs)
            self.assertNotEqual(subprocess.run(verify_cmd, cwd=root, env=env,
                                               capture_output=True).returncode, 0)

    def test_advisory_plan_matches_existing_pr_gate_for_representative_inputs(self):
        spec = importlib.util.spec_from_file_location(
            'gate_policy', ROOT / '.github/ci/selected-acceptance-gate_test.py')
        gate = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(gate)
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        cases = (
            (['docs/TESTING.md'], [], [], False, False),
            (['src/backend/chat/main.go'], ['chat', 'messaging', 'file', 'gateway'], ['chat'], True, True),
            (['src/frontend/lib/main.dart'], [], [], False, False),
            (['protos/voice/chat/v1/chat.proto'], ['chat'], [], True, True),
            (['scripts/ci/unknown.py'], ['chat'], [], True, True),
            (['src/backend/migrations/auth_db/000999_test.up.sql'], [], [], False, False),
            (['docker/clickhouse/init/001_events.sql'], [], [], False, False),
        )
        for paths, go, integration, run_go, run_pkg in cases:
            with self.subTest(paths=paths):
                matched = planner.match_filters(rules, paths)
                matrix = {'go_services': go, 'docker_go_services': [],
                          'integration_go_services': integration,
                          'run_go': run_go, 'run_pkg': run_pkg}
                flags = {k: str(v).lower() for k, v in planner.effective_filters(matched).items()}
                flags['chat_auth_java'] = str(
                    flags.get('chat_auth_java_inputs') == 'true' or flags.get('auth') == 'true'
                    or flags.get('protos') == 'true' or run_pkg or 'chat' in integration).lower()
                flags.update(run_go=str(run_go).lower(), run_pkg=str(run_pkg).lower(),
                             run_auth=flags['auth'], run_web=flags['frontend'],
                             run_admin=flags['admin'], run_developer_portal=flags['developer-portal'],
                             integration_analytics=str('analytics' in integration or
                                                       flags['analytics_integration_inputs'] == 'true').lower(),
                             integration_go_services=json.dumps(integration, separators=(',', ':')))
                values = gate.context('pull_request', **flags)
                self.assertEqual(gate.run_gate(values).returncode, 0)
                env = gate.gate_env(values)
                required = set()
                for key in env:
                    if not key.startswith('JOB_'):
                        continue
                    name = key[4:].lower().replace('_', '-')
                    if gate.run_gate(values, name, 'skipped').returncode:
                        required.add(name)
                plan = planner.make_plan(matched, matrix)
                proposed = set(plan['checks'])
                self.assertEqual(required - proposed, set())
                expected_extra = ({'backend-go-integration-pr'} if run_go and not integration and
                                  (flags.get('protos') == 'true' or flags.get('ci_global') == 'true')
                                  else set())
                if integration or (go and any(flags.get(key) == 'true' for key in
                                               ('global', 'ci_global', 'pkg', 'protos', 'go_build_shared'))):
                    expected_extra.add('backend-go-integration-evidence')
                if 'chat' in plan['components']['go_integration']:
                    expected_extra.add('chat-lifecycle-integration')
                if 'realtime' in plan['components']['go_integration']:
                    expected_extra.add('realtime-lifecycle-integration')
                if 'role' in plan['components']['go_integration']:
                    expected_extra.add('role-space-receipt-integration')
                if 'space' in plan['components']['go_integration']:
                    expected_extra.add('space-native-installer-integration')
                if 't31-critical' in proposed:
                    expected_extra.add('t31-critical')
                if 'local-docker-isolation' in proposed:
                    expected_extra.add('local-docker-isolation')
                self.assertEqual(proposed - required, expected_extra)

    def test_git_diff_includes_both_rename_paths_and_deletions(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            def git(*args):
                return subprocess.check_output(['git', '-C', d, *args], text=True).strip()
            git('init', '-q')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            (root / 'old.txt').write_text('rename me\n', encoding='utf-8')
            (root / 'deleted.txt').write_text('delete me\n', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'base')
            base = git('rev-parse', 'HEAD')
            planner.validate_checkout(root, base)
            (root / 'untracked.txt').write_text('untracked', encoding='utf-8')
            with self.assertRaises(ValueError):
                planner.validate_checkout(root, base)
            (root / 'untracked.txt').unlink()
            (root / 'old.txt').rename(root / 'new.txt')
            (root / 'deleted.txt').unlink()
            git('add', '-A')
            git('commit', '-qm', 'head')
            head = git('rev-parse', 'HEAD')
            self.assertEqual(planner.changed_paths(root, base, head),
                             ['deleted.txt', 'new.txt', 'old.txt'])
            self.assertEqual(planner.validate_identity(root, base, head, head),
                             {'base': base, 'head': head, 'candidate': head})
            with self.assertRaises(ValueError):
                planner.validate_identity(root, base, head, base)
            with self.assertRaises(ValueError):
                planner.validate_identity(root, 'HEAD', head, head)

    def test_candidate_must_be_the_clean_merge_of_exact_parents(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            def git(*args):
                return subprocess.check_output(['git', '-C', d, *args], text=True).strip()
            git('init', '-q')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            (root / 'base.txt').write_text('base', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'initial')
            event_base = git('rev-parse', 'HEAD')
            git('switch', '-qc', 'feature')
            (root / 'feature.txt').write_text('feature', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'feature')
            head = git('rev-parse', 'HEAD')
            git('switch', '-q', '--detach', 'HEAD^')
            (root / 'other.txt').write_text('other', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'base')
            base = git('rev-parse', 'HEAD')
            git('merge', '--no-ff', '-qm', 'merge', head)
            candidate = git('rev-parse', 'HEAD')
            self.assertEqual(planner.candidate_base(root, event_base, head, candidate), base)
            self.assertEqual(planner.candidate_base(root, base, head, candidate), base)
            with self.assertRaises(ValueError):
                planner.candidate_base(root, head, head, candidate)
            with self.assertRaises(ValueError):
                planner.candidate_base(root, event_base, base, candidate)
            self.assertEqual(planner.validate_identity(root, base, head, candidate)['candidate'], candidate)
            with self.assertRaises(ValueError):
                planner.validate_identity(root, head, base, candidate)
            with self.assertRaises(ValueError):
                planner.validate_identity(root, base, head, head)


if __name__ == '__main__':
    unittest.main()
