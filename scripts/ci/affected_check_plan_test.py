"""Contract tests for the shared CI affected-check planner."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
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

    def test_release_branch_always_requires_history_audit(self):
        rules = planner.load_filters(ROOT / '.github/ci/path-filters.yml')
        matched = planner.match_filters(rules, ['docs/TESTING.md'])
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        plan = planner.make_plan(matched, matrix, 'codex/voice-master-release-example')
        self.assertIn('voice-history-audit', plan['checks'])
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
            self.assertIn('voice-history-audit', plan['checks'])
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
                proposed = set(planner.make_plan(matched, matrix)['checks'])
                self.assertEqual(required - proposed, set())
                expected_extra = ({'backend-go-integration-pr'} if run_go and not integration and
                                  (flags.get('protos') == 'true' or flags.get('ci_global') == 'true')
                                  else set())
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
