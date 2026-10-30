import unittest
from pathlib import Path

import full_validation_gate as gate


class FullValidationGateTests(unittest.TestCase):
    def setUp(self):
        self.sha = 'a' * 40
        self.context = {'event': 'workflow_dispatch', 'profile': 'full',
                        'ref': 'refs/heads/master', 'sha': self.sha,
                        'checkout': self.sha, 'run': '123', 'attempt': '1'}
        self.needs = {name: {'result': 'success'} for name in gate.REQUIRED_JOBS}
        self.services = ['chat', 'space']
        self.needs['changes']['outputs'] = {
            'go_services': '["chat","space"]', 'integration_go_services': '["chat","space"]',
            'run_go': 'true', 'run_pkg': 'true', 'code': 'true', 'run_flutter_tier2': 'true'}
        self.audit = {'source_cases': 100, 'unresolved': 0}

    def test_complete_exact_candidate_has_receipt(self):
        result = gate.qualify(self.context, self.needs, self.audit, self.services)
        self.assertEqual(result['identity']['sha'], self.sha)
        self.assertEqual(result['jobs'], list(gate.REQUIRED_JOBS))
        self.assertEqual(result['identity']['run'], '123')

    def test_master_push_requires_the_same_complete_exact_sha(self):
        context = dict(self.context, event='push', profile='full')
        result = gate.qualify(context, self.needs, self.audit, self.services)
        self.assertEqual(result['identity']['sha'], self.sha)
        for changes in ({'checkout': 'b' * 40}, {'ref': 'refs/heads/develop'},
                        {'profile': 'auto'}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                gate.qualify(dict(context, **changes), self.needs, self.audit, self.services)
        needs = dict(self.needs, protobuf={'result': 'skipped'})
        with self.assertRaises(ValueError):
            gate.qualify(context, needs, self.audit, self.services)

    def test_missing_and_non_success_jobs_fail(self):
        for name in gate.REQUIRED_JOBS:
            for status in ('skipped', 'failure', 'cancelled', None):
                needs = dict(self.needs)
                if status is None:
                    del needs[name]
                else:
                    needs[name] = {'result': status}
                with self.subTest(name=name, status=status), self.assertRaises(ValueError):
                    gate.qualify(self.context, needs, self.audit, self.services)

    def test_rejects_stale_identity_partial_profile_and_unresolved_inventory(self):
        for changes in ({'checkout': 'b' * 40}, {'sha': 'bad'}, {'profile': 'auto'},
                        {'event': 'pull_request'}, {'ref': 'refs/heads/develop'},
                        {'run': '0'}, {'attempt': ''}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                gate.qualify(dict(self.context, **changes), self.needs, self.audit, self.services)
        for audit in ({'source_cases': 0, 'unresolved': 0},
                      {'source_cases': 100, 'unresolved': 1}, {}):
            with self.subTest(audit=audit), self.assertRaises(ValueError):
                gate.qualify(self.context, self.needs, audit, self.services)

    def test_partial_or_duplicate_matrices_and_disabled_full_scope_fail(self):
        for key, value in (('go_services', '["chat"]'),
                           ('integration_go_services', '["chat","chat","space"]'),
                           ('integration_go_services', 'not-json'), ('run_go', 'false'),
                           ('run_pkg', 'false'), ('code', 'false'), ('run_flutter_tier2', 'false')):
            needs = dict(self.needs)
            outputs = dict(needs['changes']['outputs'], **{key: value})
            needs['changes'] = {'result': 'success', 'outputs': outputs}
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                gate.qualify(self.context, needs, self.audit, self.services)

    def test_reports_inventory_and_failed_jobs_together(self):
        needs = dict(self.needs)
        needs['protobuf'] = {'result': 'skipped'}
        outputs = dict(needs['changes']['outputs'], go_services='["chat"]')
        needs['changes'] = {'result': 'success', 'outputs': outputs}
        with self.assertRaises(ValueError) as error:
            gate.qualify(self.context, needs, {'source_cases': 100, 'unresolved': 3}, self.services)
        self.assertIn('unresolved', str(error.exception))
        self.assertIn('protobuf', str(error.exception))
        self.assertIn('go_services', str(error.exception))

    def test_workflow_binds_exact_inventory_and_full_dispatch(self):
        import yaml
        root = Path(__file__).resolve().parents[2]
        workflow = yaml.safe_load((root / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['full-validation-aggregation']
        self.assertEqual(len(gate.REQUIRED_JOBS), len(set(gate.REQUIRED_JOBS)))
        self.assertTrue(set(gate.REQUIRED_JOBS).issubset(workflow['jobs']))
        self.assertEqual(set(job['needs']), set(gate.REQUIRED_JOBS))
        self.assertEqual(len(job['needs']), len(gate.REQUIRED_JOBS))
        self.assertIn('always()', job['if'])
        selection = workflow['jobs']['changes']['outputs']['full_validation']
        self.assertIn("github.event_name == 'workflow_dispatch'", selection)
        self.assertIn("inputs.profile == 'full'", selection)
        self.assertIn("github.event_name == 'push'", selection)
        self.assertIn("github.ref == 'refs/heads/master'", selection)
        step = next(step for step in job['steps'] if step.get('name') == 'Aggregate full validation')
        self.assertIn('full_validation_gate.py', step['run'])
        self.assertEqual(step['env']['NEEDS_JSON'], '${{ toJSON(needs) }}')
        self.assertIn('full_validation_gate_test.py', (root / 'Makefile').read_text(encoding='utf-8'))

    def test_master_promotion_and_deployment_require_full_aggregation(self):
        import yaml
        root = Path(__file__).resolve().parents[2]
        jobs = yaml.safe_load((root / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        aggregate = jobs['full-validation-aggregation']
        self.assertIn("needs.changes.outputs.full_validation == 'true'", aggregate['if'])
        for name in ('staging-images-promote', 'deploy-staging'):
            with self.subTest(job=name):
                self.assertIn('full-validation-aggregation', jobs[name]['needs'])
                self.assertIn("needs.full-validation-aggregation.result == 'success'", jobs[name]['if'])
        self.assertIn('staging-go-matrix', [step.get('id') for step in jobs['changes']['steps']])
        staging = next(step for step in jobs['changes']['steps'] if step.get('id') == 'stagingmatrix')
        self.assertEqual(staging['env']['GO_SERVICES_JSON'], '${{ steps.staging-go-matrix.outputs.go_services }}')

    def test_full_master_routing_covers_the_required_jobs_without_a_cycle(self):
        import yaml
        root = Path(__file__).resolve().parents[2]
        jobs = yaml.safe_load((root / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        already_selected_on_master = {'changes', 'flutter-unit-receipts',
                                      'flutter-windows', 'flutter-android-smoke',
                                      'flutter-web-integration', 'flutter-ios'}
        for name in set(gate.REQUIRED_JOBS) - already_selected_on_master:
            with self.subTest(job=name):
                self.assertIn("needs.changes.outputs.full_validation == 'true'", jobs[name]['if'])
        self.assertIn("github.event_name == 'schedule'", jobs['voice-history-audit']['if'])
        self.assertNotIn('voice-history-audit', jobs['ci-gate']['needs'])
        for name in ('flutter-windows', 'flutter-android-smoke', 'flutter-web-integration', 'flutter-ios'):
            self.assertIn("github.event_name == 'push'", jobs[name]['if'])
            self.assertIn("needs.changes.outputs.run_flutter_tier2 == 'true'", jobs[name]['if'])
        self.assertIn("github.event_name == 'push'", jobs['changes']['outputs']['run_flutter_tier2'])
        for name in ('web', 'admin', 'developer-portal'):
            step = next(step for step in jobs[name]['steps']
                        if step.get('name') == 'Build ' + name + ' image (PR verify)')
            self.assertIn("needs.changes.outputs.full_validation == 'true'", step['if'])
            self.assertIs(step['with']['push'], False)
        active, done = set(), set()
        def visit(name):
            if name in active:
                self.fail('workflow dependency cycle involving ' + name)
            if name in done:
                return
            active.add(name)
            dependencies = jobs[name].get('needs', [])
            if isinstance(dependencies, str):
                dependencies = [dependencies]
            for dependency in dependencies:
                self.assertIn(dependency, jobs)
                visit(dependency)
            active.remove(name)
            done.add(name)
        for name in jobs:
            visit(name)

    def test_duplicate_json_keys_are_rejected(self):
        import json
        with self.assertRaises(ValueError):
            json.loads('{"x":1,"x":2}', object_pairs_hook=gate._unique_object)

    def test_full_dispatch_selects_previously_pr_only_proofs(self):
        import yaml
        root = Path(__file__).resolve().parents[2]
        jobs = yaml.safe_load((root / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        names = ('backend-go-integration-pr', 'backend-go-integration-evidence',
                 'local-docker-isolation', 'backend-auth-image', 't31-critical',
                 'nats-hub-config-renderer-image', 'story-lfp-integration',
                 'user-association-integration', 'notification-quota-integration',
                 'role-migration-integration', 'role-space-receipt-integration',
                 'bot-membership-integration', 'voice-ledger-integration',
                 'messaging-threadlist-integration', 'space-autovacuum-integration',
                 'space-native-installer-integration', 'chat-lifecycle-integration',
                 'realtime-lifecycle-integration')
        for name in names:
            with self.subTest(name=name):
                self.assertIn("inputs.profile == 'full'", jobs[name]['if'])
        step = next(s for s in jobs['nats-hub-config-renderer-image']['steps']
                    if s.get('name') == 'Build renderer image (PR verify)')
        self.assertIn("inputs.profile == 'full'", step['if'])

    def test_static_full_matrix_and_checkout_bindings(self):
        import json
        import yaml
        root = Path(__file__).resolve().parents[2]
        jobs = yaml.safe_load((root / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        inventory = json.loads((root / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        services = jobs['backend-go-integration']['strategy']['matrix']['service']
        self.assertEqual(len(services), len(set(services)))
        self.assertEqual(set(services), set(inventory))
        self.assertEqual(jobs['flutter-unit-tests']['strategy']['matrix']['shard'], [0, 1, 2, 3])
        for name in gate.REQUIRED_JOBS:
            if name == 't31-critical':
                self.assertEqual(jobs[name]['uses'], './.github/workflows/t31-session-events-e2e.yml')
                nested = yaml.safe_load((root / '.github/workflows/t31-session-events-e2e.yml').read_text(encoding='utf-8'))['jobs']
                checkouts = [step for job in nested.values() for step in job.get('steps', [])
                             if str(step.get('uses', '')).startswith('actions/checkout@')]
            else:
                checkouts = [step for step in jobs[name].get('steps', [])
                             if str(step.get('uses', '')).startswith('actions/checkout@')]
            with self.subTest(job=name):
                self.assertTrue(checkouts)
                self.assertTrue(all('ref' not in step.get('with', {}) for step in checkouts))

    def test_go_validation_does_not_rewrite_committed_module_files(self):
        import yaml
        root = Path(__file__).resolve().parents[2]
        jobs = yaml.safe_load((root / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        for name in ('golangci', 'backend-go-pkg', 'backend-go', 'backend-go-integration-pr',
                     'backend-go-integration', 'local-ci-parity', 'local-docker-isolation',
                     'story-lfp-integration', 'user-association-integration',
                     'notification-quota-integration', 'role-space-receipt-integration',
                     'realtime-lifecycle-integration', 'chat-lifecycle-integration',
                     'space-native-installer-integration', 'space-autovacuum-integration',
                     'messaging-threadlist-integration', 'voice-ledger-integration',
                     'bot-membership-integration', 'role-migration-integration'):
            with self.subTest(job=name):
                self.assertEqual(jobs[name].get('env', {}).get('GOFLAGS'), '-mod=readonly')
                self.assertFalse(any('go mod tidy' in step.get('run', '')
                                     for step in jobs[name].get('steps', [])))
                guard = next(step for step in jobs[name]['steps']
                             if step.get('name') == 'Verify Go module source inputs')
                self.assertIn('check_go_modules_clean.py', guard['run'])
                self.assertIn('always()', guard['if'])
        makefile = (root / 'Makefile').read_text(encoding='utf-8')
        for target in ('backend-test-ci:', 'backend-test-ci-short:'):
            line = next(line for line in makefile.splitlines() if line.startswith(target))
            self.assertIn('go-mod-resolve-all', line)
            self.assertNotIn('go-mod-tidy-all', line)


if __name__ == '__main__':
    unittest.main()
