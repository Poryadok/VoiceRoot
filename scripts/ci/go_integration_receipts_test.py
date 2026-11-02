"""Required Go integration cases must execute, not merely be selected."""
import importlib.util
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('receipts', ROOT / 'scripts/ci/go_integration_receipts.py')
receipts = importlib.util.module_from_spec(spec)
spec.loader.exec_module(receipts)


class GoIntegrationReceiptTests(unittest.TestCase):
    def test_native51_freeze_requires_qualified_source_and_exact_pass(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        name = 'TestNative51FreezeRequiresBoundProofAndPreservesMembership'
        inventory = json.loads((ROOT / 'scripts/ci/space_native51_freeze_required.json').read_text(encoding='utf-8'))['space']
        self.assertEqual(inventory, [[package, name]])
        job = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']['space-native-installer-integration']
        step = next(item for item in job['steps'] if item.get('name') == 'Run trusted Space native50 catalog proofs')
        run = step['run']
        self.assertLess(run.index('TestNative51FixtureCommitTrusted50'), run.index(name))
        self.assertLess(run.index(name), run.index('DROP DATABASE space_db'))
        self.assertIn('space_native51_freeze_required.json', run)
        self.assertIn('--output "$RUNNER_TEMP/space-native51-freeze-receipt.json" --reject-skips', run)
        events = [{'Package': package, 'Test': name, 'Action': 'run'},
                  {'Package': package, 'Test': name, 'Action': 'pass'}, {'Package': package, 'Action': 'pass'}]
        self.assertFalse(receipts.verify_required(events, inventory, 0))
        events[1]['Action'] = 'skip'
        self.assertIn((package, name), receipts.verify_required(events, inventory, 0))

    def test_space_over100_requires_real_ordered_fixture_and_consumer(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        cases = (('space_over100_fixture_required.json', 'TestAccountBanFixtureOver100Cohort34'),
                 ('space_over100_consumer_required.json', 'TestAccountBanOver100Cohort50'))
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        for filename, name in cases:
            inventory = json.loads((ROOT / 'scripts/ci' / filename).read_text(encoding='utf-8'))['space']
            self.assertEqual(inventory, [[package, name]])
            entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
            self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
            events = [{'Package': package, 'Test': name, 'Action': 'run'},
                      {'Package': package, 'Test': name, 'Action': 'pass'}, {'Package': package, 'Action': 'pass'}]
            self.assertFalse(receipts.verify_required(events, inventory, 0))
            for mutation in ('skip', 'missing'):
                broken = [event for event in events if mutation != 'missing' or event.get('Action') != 'pass' or event.get('Test') != name]
                if mutation == 'skip':
                    broken = [dict(event, Action='skip') if event.get('Test') == name and event['Action'] == 'pass' else event for event in broken]
                self.assertIn((package, name), receipts.verify_required(broken, inventory, 0))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        job = jobs['space-native-installer-integration']
        step = next(item for item in job['steps'] if item.get('name') == 'Run Space over100 legacy cohort and native Ban proof')
        self.assertIn('space_over100_proof.sh', step['run'])
        self.assertIn('/space_over100?', step['env']['SPACE_BAN_TEST_DATABASE_URL'])
        self.assertIn('space-native-installer-integration', jobs['ci-gate']['needs'])
        script = (ROOT / 'scripts/ci/space_over100_proof.sh').read_text(encoding='utf-8')
        phases = ('goto 34', 'INSERT INTO spaces', 'INSERT INTO community_owner_authority',
                  'INSERT INTO community_roster_members', 'receipt fixture', 'goto 44', 'goto 45',
                  'receipt transition45', 'goto 50', 'receipt transition50', 'receipt consumer')
        positions = [script.index(value) for value in phases]
        self.assertEqual(positions, sorted(positions))
        self.assertIn("00000000000d", script)
        self.assertIn('FROM (VALUES (1),(2),(3),(6))', script)
        self.assertIn('FROM (VALUES (2),(5))', script)
        self.assertIn('-timeout 5m', script)
        self.assertIn('--reject-skips', script)
        self.assertIn('"$actual" != "$owned_oid"', script)
        self.assertIn('trap cleanup EXIT', script)
        self.assertLess(script.index('trap cleanup EXIT'), script.index("-c 'CREATE DATABASE space_over100'"))
        for filename, name in cases:
            self.assertIn(filename + ' ' + name, script)

    def test_no_test_package_requires_discovery_and_real_no_test_marker(self):
        events = [{'Package': 'example/p', 'Action': 'start'},
                  {'Package': 'example/p', 'Action': 'output', 'Output': '?  \texample/p\t[no test files]\n'},
                  {'Package': 'example/p', 'Action': 'skip'}]
        with self.assertRaises(ValueError):
            receipts.validate_test_run(events, ['example/p'])
        result = receipts.validate_test_run(events, ['example/p'], ['example/p'])
        self.assertEqual(result['passed'], 0)
        self.assertEqual(result['skipped'], [])
        for broken in (events[:1] + events[2:], events + [events[1]],
                       [events[0], events[1], {'Package': 'example/p', 'Action': 'pass'}],
                       [events[0], events[1], {'Package': 'example/p', 'Test': 'TestSkip', 'Action': 'run'},
                        {'Package': 'example/p', 'Test': 'TestSkip', 'Action': 'skip'}, events[2]]):
            with self.subTest(events=broken), self.assertRaises(ValueError):
                receipts.validate_test_run(broken, ['example/p'], ['example/p'])

    def test_full_test_stream_requires_complete_unique_results_and_reports_skips(self):
        events = [
            {'Package': 'example/p', 'Action': 'start'},
            {'Package': 'example/p', 'Test': 'TestOne', 'Action': 'run'},
            {'Package': 'example/p', 'Test': 'TestOne', 'Action': 'pass'},
            {'Package': 'example/p', 'Test': 'TestOptional', 'Action': 'run'},
            {'Package': 'example/p', 'Test': 'TestOptional', 'Action': 'skip'},
            {'Package': 'example/p', 'Action': 'pass'},
        ]
        result = receipts.validate_test_run(events, ['example/p'])
        self.assertEqual(result['passed'], 1)
        self.assertEqual(result['skipped'], [['example/p', 'TestOptional']])
        for broken in (events[:-1], events + [events[2]],
                       [e for e in events if e != events[1]],
                       events + [{'Package': 'example/other', 'Action': 'pass'}],
                       [events[0], {'Package': 'example/p', 'Test': 'Ghost', 'Action': 'output'}, *events[1:]],
                       [events[0], {'Package': 'example/p', 'Test': 'Ghost', 'Action': 'pause'}, *events[1:]]):
            with self.subTest(events=broken), self.assertRaises(ValueError):
                receipts.validate_test_run(broken, ['example/p'])

    def test_complete_stream_cli_fails_on_skip_or_nonzero_exit(self):
        with tempfile.TemporaryDirectory() as directory:
            log = Path(directory) / 'go.jsonl'
            events = [{'Package': 'example/p', 'Action': 'start'},
                      {'Package': 'example/p', 'Test': 'TestOne', 'Action': 'run'},
                      {'Package': 'example/p', 'Test': 'TestOne', 'Action': 'pass'},
                      {'Package': 'example/p', 'Action': 'pass'}]
            def run(exit_code):
                log.write_text(''.join(json.dumps(e) + '\n' for e in events), encoding='utf-8')
                return subprocess.run([sys.executable, str(ROOT / 'scripts/ci/go_integration_receipts.py'),
                                       '--events', str(log), '--exit-code', str(exit_code),
                                       '--complete-package', 'example/p'], capture_output=True, text=True)
            self.assertEqual(run(0).returncode, 0)
            self.assertNotEqual(run(1).returncode, 0)
            events[2]['Action'] = 'skip'
            self.assertNotEqual(run(0).returncode, 0)

    def test_space_native_installer_receipt_requires_every_failure_boundary(self):
        inventory = json.loads((ROOT / 'scripts/ci/space_native_installer_required.json').read_text(encoding='utf-8'))['space']
        package = 'voice/backend/space/internal/nativeinstaller'
        parent = 'TestCommittedPhaseFailureNeverAutoResumes'
        phases = ('created', 'migrated44', 'trusted44', 'migrated45', 'trusted45',
                  'migrated50', 'trusted50', 'trusted51', 'complete')
        required = [[package, 'TestProductionFreshChainAndRetainedVerify'], [package, parent]]
        required.extend([package, parent + '/' + phase] for phase in phases)
        self.assertEqual(inventory, required)
        events = [{'Package': p, 'Test': name, 'Action': action}
                  for p, name in required for action in ('run', 'pass')]
        events.append({'Package': package, 'Action': 'pass'})
        self.assertEqual(receipts.verify_required(events, required, 0), [])
        for phase in phases:
            test = parent + '/' + phase
            missing = [event for event in events if not (event.get('Test') == test and event['Action'] == 'pass')]
            self.assertIn((package, test), receipts.verify_required(missing, required, 0))

    def test_role_space_receipt_inventory_excludes_unfinished_interactive_oracle(self):
        inventory = json.loads((ROOT / 'scripts/ci/role_space_receipt_required.json').read_text(encoding='utf-8'))
        cases = ['TestMemberAdmissionPositiveProtectedReceipt', 'TestMemberAdmissionRole25ProtectedReceipt']
        self.assertEqual(inventory['role'], [['voice/backend/role', name] for name in cases])
        source = (ROOT / 'src/backend/role/native_space_integration_test.go').read_text(encoding='utf-8')
        self.assertIn('//go:build role_space_native', source)
        for name in cases:
            self.assertRegex(source, rf'(?m)^func {name}\(t \*testing\.T\)')
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        interactive = next(x for x in exceptions if x['test'] == 'TestNativeAssignmentNeedsQualifiedOperationPublication')
        self.assertEqual(interactive['status'], 'unresolved')
        self.assertNotIn(['voice/backend/role', interactive['test']], inventory['role'])

    def test_space_current_schema_cases_are_classified_by_their_required_receipt(self):
        inventory = json.loads((ROOT / 'scripts/ci/space_account_ban_guard_required.json').read_text(encoding='utf-8'))
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        for name in ('TestAccountBanCannotSucceedWithoutCompleteOwnerCapture',
                     'TestNative51CreatorBornCurrentSchema',
                     'TestAccountBanCatalogRejectsSemanticDrift',
                     'TestNative51ProofDiagnosticEnums'):
            self.assertIn(['voice/backend/space/internal/store', name], inventory['space'])
            entry = next(x for x in exceptions if x['service'] == 'space' and x['test'] == name)
            self.assertEqual((entry['status'], entry['reason']),
                             ('dedicated', 'space-native-installer-integration'))

    def test_chat_auth_java_candidate_proof_has_required_no_skip_receipt(self):
        import yaml
        case = 'TestChatAuthActualJavaInteroperability'
        inventory = json.loads((ROOT / 'scripts/ci/chat_auth_java_required.json').read_text(encoding='utf-8'))
        self.assertEqual(inventory['chat'], [['voice/backend/chat', case]])
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        entry = next(x for x in exceptions if x['service'] == 'chat' and x['test'] == case)
        self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'chat-auth-java-integration'))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        selector = jobs['changes']['outputs']['chat_auth_java']
        for field in ('chat_auth_java_inputs', 'auth', 'protos', 'run_pkg', 'integration_go_services'):
            self.assertIn(field, selector)
        self.assertIn('chat-auth-java-integration', jobs['ci-gate']['needs'])
        self.assertIn('chat-auth-java-integration', jobs['full-validation-aggregation']['needs'])
        job = jobs['chat-auth-java-integration']
        build = next(x for x in job['steps'] if x.get('name') == 'Build candidate Auth fixture')
        self.assertIn('clean test', build['run'])
        proof = next(x for x in job['steps'] if x.get('name') == 'Run candidate Chat to Auth proof')
        self.assertIn('--reject-skips', proof['run'])
        self.assertIn('chat_auth_java_required.json', proof['run'])
        self.assertEqual(proof['env']['BE131_AUTH_CANDIDATE_SHA'], '${{ github.sha }}')
        self.assertEqual(proof['env']['BE131_AUTH_PROOF_DIR'], '${{ runner.temp }}/chat-auth-java-proof')
        artifact = next(x for x in job['steps'] if x.get('name') == 'Upload Chat to Auth proof')
        self.assertIn('${{ runner.temp }}/chat-auth-java-proof/', artifact['with']['path'])

    def test_gateway_live_leave_diagnostic_has_required_parent_and_subprocess_disposition(self):
        import yaml
        name = 'TestComposeSpaceLeaveDiagnosticPreservesStrictStatus'
        helper = 'TestComposeSpaceLeaveDiagnosticProbe'
        inventory = json.loads((ROOT / 'scripts/ci/gateway_leave_diagnostic_required.json').read_text(encoding='utf-8'))
        package = 'voice/backend/gateway'
        self.assertEqual(inventory['gateway'], [[package, name], [package, name + '/204'],
                                                [package, name + '/503']])
        events = [{'Package': package, 'Test': test, 'Action': action}
                  for _, test in inventory['gateway'] for action in ('run', 'pass')]
        events.append({'Package': package, 'Action': 'pass'})
        self.assertFalse(receipts.verify_required(events, inventory['gateway'], 0))
        events[-2]['Action'] = 'skip'
        self.assertTrue(receipts.verify_required(events, inventory['gateway'], 0))
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        parent = next(x for x in exceptions if x['service'] == 'gateway' and x['test'] == name)
        child = next(x for x in exceptions if x['service'] == 'gateway' and x['test'] == helper)
        self.assertEqual((parent['status'], parent['reason']), ('dedicated', 'backend-go-integration-pr'))
        self.assertEqual((child['status'], child['reason']), ('subprocess', name))
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        steps = workflow['jobs']['backend-go-integration-pr']['steps']
        test = next(step for step in steps if step.get('name') == 'Gateway live leave diagnostic')
        self.assertIn("matrix.service == 'gateway'", test['if'])
        self.assertIn(name, test['run'])
        self.assertIn('gateway_leave_diagnostic_required.json', test['run'])
        self.assertIn('--reject-skips', test['run'])
        from go_integration_source_audit import audit
        full = json.loads((ROOT / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        for broken in ([x for x in exceptions if x is not parent],
                       [dict(x, status='unresolved', reason='missing proof') if x is parent else x
                        for x in exceptions],
                       [dict(x, reason='TestUnrelated') if x is child else x for x in exceptions]):
            with self.assertRaises(ValueError):
                audit(ROOT, full, broken)

    def test_space_trusted50_real_boundary_and_required_catalog_cases(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        catalog = 'TestNativeCatalog50LiteralProjectionAndDrift'
        atomic = 'TestNative51AtomicMigrationFailureRollsBackBodySealAndHistory'
        stale = 'TestNative51StaleExpectedParentCannotCommit'
        roots = (catalog, 'TestNative51RawCompositeSessionStability',
                 'TestNative51ClassifierRejectsUnclassifiedMissingDatum',
                 'TestNative50CreatorBornCurrentSchema', atomic,
                 'TestNative51FrameworkUnlockRetainsTransactionFences', stale)
        phases = ('after_dirty_marker', 'after_body', 'after_CAS_and_clean_marker',
                  'engine_unlock_error', 'outer_window_failure')
        cases = ('function_cost', 'function_ACL', 'function_owner', 'escaped_Unicode_body',
                 'unexpected_function', 'private_index_attributes', 'column', 'constraint', 'trigger_disabled')
        inventory = json.loads((ROOT / 'scripts/ci/space_trusted50_required.json').read_text(encoding='utf-8'))
        self.assertEqual(inventory['space'], [[package, name] for name in roots] +
                         [[package, stale + '/' + name] for name in ('epoch', 'seal')] +
                         [[package, atomic + '/' + name] for name in phases] +
                         [[package, catalog + '/' + name] for name in cases])
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        for name in roots:
            entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
            self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        steps = jobs['space-native-installer-integration']['steps']
        step = next(item for item in steps if item.get('name') == 'Run trusted Space native50 catalog proofs')
        self.assertIn('-at migrated50', step['run'])
        self.assertIn('controlled deployment evidence persistence failure', step['run'])
        self.assertIn('space_trusted50_required.json', step['run'])
        self.assertIn('--reject-skips', step['run'])
        self.assertLess(steps.index(step), next(i for i, item in enumerate(steps)
                                               if item.get('name') == 'Run Space account-ban capture guard'))
        events = [{'Package': package, 'Test': test, 'Action': action}
                  for _, test in inventory['space'] for action in ('run', 'pass')]
        events.append({'Package': package, 'Action': 'pass'})
        self.assertFalse(receipts.verify_required(events, inventory['space'], 0))
        events[-2]['Action'] = 'skip'
        self.assertTrue(receipts.verify_required(events, inventory['space'], 0))

    def test_clock_space_observers_require_real_seed_and_no_skips(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        parent = 'TestNativeObserverOtherProfileCommunityGap'
        cases = (parent, parent + '/time_gap_cannot_preserve_old_history',
                 parent + '/continuous_unrelated_write_preserves_history',
                 'TestNativeObserverNoopLeaseGapTuple')
        inventory = json.loads((ROOT / 'scripts/ci/space_clock50_observer_required.json').read_text(encoding='utf-8'))['space']
        self.assertEqual(inventory, [[package, name] for name in cases])
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        for name in (parent, cases[-1]):
            entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
            self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        steps = jobs['space-native-installer-integration']['steps']
        step = next(item for item in steps if item.get('name') == 'Run Space clock-sensitive observer proofs')
        self.assertIn('scripts/ci/space_native_clock_proof.sh', step['run'])
        script = (ROOT / 'scripts/ci/space_native_clock_proof.sh').read_text(encoding='utf-8')
        phases = ('migrate 34', 'INSERT INTO spaces', 'INSERT INTO community_owner_authority',
                  'migrate 44', 'migrate 45', 'receipt transition45', 'migrate 50',
                  'receipt transition50', 'receipt enrollment', 'receipt observer')
        positions = [script.index(value) for value in phases]
        self.assertEqual(positions, sorted(positions))
        self.assertIn('--reject-skips', script)
        self.assertIn('account_ban_wall_clock.c', script)
        self.assertIn('voice.ci.fixture', script)
        self.assertIn('docker container rm -f "$id"', script)
        events = [{'Package': package, 'Test': name, 'Action': action}
                  for _, name in inventory for action in ('run', 'pass')]
        events.append({'Package': package, 'Action': 'pass'})
        self.assertFalse(receipts.verify_required(events, inventory, 0))
        for _, name in inventory:
            missing = [event for event in events if not (event.get('Test') == name and event['Action'] == 'pass')]
            self.assertIn((package, name), receipts.verify_required(missing, inventory, 0))
            skipped = [dict(event, Action='skip') if event.get('Test') == name and event['Action'] == 'pass' else event for event in events]
            self.assertIn((package, name), receipts.verify_required(skipped, inventory, 0))

    def test_clock_roster_expiry_requires_ordered_phases_and_receipts(self):
        package = 'voice/backend/space/internal/store'
        cases = (('space_roster_enrollment_required.json', 'TestAccountBanFixtureRosterEnrollment50'),
                 ('space_gis50_required.json', 'TestAccountBanGIS50'),
                 ('space_gis_expiry_required.json', 'TestAccountBanGISExpiry50'))
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        for filename, name in cases:
            inventory = json.loads((ROOT / 'scripts/ci' / filename).read_text(encoding='utf-8'))['space']
            self.assertEqual(inventory, [[package, name]])
            entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
            self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
            events = [{'Package': package, 'Test': name, 'Action': 'run'},
                      {'Package': package, 'Test': name, 'Action': 'pass'}, {'Package': package, 'Action': 'pass'}]
            self.assertFalse(receipts.verify_required(events, inventory, 0))
            events[1]['Action'] = 'skip'
            self.assertIn((package, name), receipts.verify_required(events, inventory, 0))
        script = (ROOT / 'scripts/ci/space_native_clock_proof.sh').read_text(encoding='utf-8')
        phases = ('native7-fixture-8', 'migrate 44', 'migrate 45', 'receipt transition45',
                  'migrate 50', 'receipt transition50', 'receipt roster-enrollment',
                  'receipt roster space_gis50_required.json', 'receipt roster-expiry')
        positions = [script.index(value) for value in phases]
        self.assertEqual(positions, sorted(positions))
        for filename, name in cases:
            self.assertIn(filename, script)
            self.assertIn(name, script)
        self.assertIn('--reject-skips', script)

    def test_seeded_space_observers_require_real_transition_and_enrollment(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        drift = 'TestNativeCatalogConcurrentRowDrift'
        observers = ('TestNativeObserverNoopTupleAndFrame', 'TestNativeObserverCutPredicateAlgebra',
                     'TestNativeObserverPrivateClosureAndLock')
        inventories = [json.loads((ROOT / ('scripts/ci/space_seeded50_' + name + '_required.json')).read_text(encoding='utf-8'))['space']
                       for name in ('drift', 'observer')]
        self.assertEqual(inventories, [[[package, drift]], [[package, name] for name in observers]])
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        for name in (drift, *observers):
            entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
            self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        steps = jobs['space-native-installer-integration']['steps']
        step = next(item for item in steps if item.get('name') == 'Run seeded Space native50 observer proofs')
        run = step['run']
        phases = ('goto 34', 'INSERT INTO spaces', 'goto 44', 'goto 45',
                  'TestAccountBanFixtureTrusted45Transition', 'goto 50',
                  'TestAccountBanFixtureTrusted50Transition', drift, *observers)
        positions = [run.index(value) for value in phases]
        self.assertEqual(positions, sorted(positions))
        self.assertEqual(run.count('--reject-skips'), 7)
        self.assertIn('owned_oid', run)
        self.assertIn('DROP DATABASE space_db', run)
        for inventory in inventories:
            events = [{'Package': package, 'Test': name, 'Action': action}
                      for _, name in inventory for action in ('run', 'pass')]
            events.append({'Package': package, 'Action': 'pass'})
            self.assertFalse(receipts.verify_required(events, inventory, 0))
            for _, name in inventory:
                missing = [event for event in events if not (event.get('Test') == name and event['Action'] == 'pass')]
                self.assertIn((package, name), receipts.verify_required(missing, inventory, 0))
                skipped = [dict(event, Action='skip') if event.get('Test') == name and event['Action'] == 'pass' else event for event in events]
                self.assertIn((package, name), receipts.verify_required(skipped, inventory, 0))

    def test_space_roster_requires_seeded_enrollment_and_component_proof(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        name = 'TestAccountBanGIS50'
        inventory = json.loads((ROOT / 'scripts/ci/space_gis50_required.json').read_text(encoding='utf-8'))['space']
        self.assertEqual(inventory, [[package, name]])
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
        self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        step = next(item for item in jobs['space-native-installer-integration']['steps']
                    if item.get('name') == 'Run seeded Space native50 observer proofs')
        run = step['run']
        positions = [run.index(value) for value in ('native7-fixture-8', 'goto 44', 'goto 50',
                                                     'TestAccountBanFixtureConcurrencyEnrollment50', name)]
        self.assertEqual(positions, sorted(positions))
        events = [{'Package': package, 'Test': name, 'Action': 'run'},
                  {'Package': package, 'Test': name, 'Action': 'pass'}, {'Package': package, 'Action': 'pass'}]
        self.assertFalse(receipts.verify_required(events, inventory, 0))
        events[1]['Action'] = 'skip'
        self.assertIn((package, name), receipts.verify_required(events, inventory, 0))

    def test_space_concurrency_requires_seeded_enrollment_and_both_proofs(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        enrollment = 'TestAccountBanFixtureConcurrencyEnrollment50'
        cases = ('TestAccountBanIntentConcurrency50', 'TestAccountBanLockWaitExhaustsOriginalBudget50',
                 'TestAccountBanLateCaptureCannotCommitOrSplice50')
        first = json.loads((ROOT / 'scripts/ci/space_concurrency_enrollment_required.json').read_text(encoding='utf-8'))['space']
        second = json.loads((ROOT / 'scripts/ci/space_concurrency_required.json').read_text(encoding='utf-8'))['space']
        self.assertEqual(first, [[package, enrollment]])
        self.assertEqual(second, [[package, case] for case in cases])
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        for name in (enrollment, *cases):
            entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
            self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        step = next(item for item in jobs['space-native-installer-integration']['steps']
                    if item.get('name') == 'Run seeded Space native50 observer proofs')
        run = step['run']
        positions = [run.index(value) for value in ('VALUES (9),(11),(12)', 'goto 44', 'goto 50', enrollment, *cases)]
        self.assertEqual(positions, sorted(positions))
        for inventory in (first, second):
            events = [{'Package': package, 'Test': name, 'Action': action}
                      for _, name in inventory for action in ('run', 'pass')]
            events.append({'Package': package, 'Action': 'pass'})
            self.assertFalse(receipts.verify_required(events, inventory, 0))
            for _, name in inventory:
                missing = [event for event in events if not (event.get('Test') == name and event['Action'] == 'pass')]
                self.assertIn((package, name), receipts.verify_required(missing, inventory, 0))
                skipped = [dict(event, Action='skip') if event.get('Test') == name and event['Action'] == 'pass' else event for event in events]
                self.assertIn((package, name), receipts.verify_required(skipped, inventory, 0))

    def test_space_observer_proof_runs_before_trusted50_transition(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        name = 'TestNativeObserver50DefinitionAndPreimage'
        cases = ('old_body_at50', 'prior_body_at50', 'old_catalog_at50',
                 'catalog_cost_drift', 'catalog_ACL_drift', 'cost_drift',
                 'definer_drift', 'search_path_drift', 'ACL_drift')
        inventory = json.loads((ROOT / 'scripts/ci/space_observer50_required.json').read_text(encoding='utf-8'))['space']
        self.assertEqual(inventory, [[package, name]] + [[package, name + '/' + case] for case in cases])
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
        self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        steps = jobs['space-native-installer-integration']['steps']
        step = next(item for item in steps if item.get('name') == 'Run trusted Space native50 catalog proofs')
        run = step['run']
        self.assertLess(run.index('-at migrated50'), run.index(name))
        self.assertLess(run.index(name), run.index('TestAccountBanFixtureTrusted50Transition'))
        self.assertIn('space_observer50_required.json', run)
        self.assertIn('--output "$RUNNER_TEMP/space-observer50-receipt.json" --reject-skips', run)
        events = [{'Package': package, 'Test': test, 'Action': action}
                  for _, test in inventory for action in ('run', 'pass')]
        events.append({'Package': package, 'Action': 'pass'})
        self.assertFalse(receipts.verify_required(events, inventory, 0))
        for _, case in inventory:
            missing = [event for event in events if not (event.get('Test') == case and event['Action'] == 'pass')]
            self.assertIn((package, case), receipts.verify_required(missing, inventory, 0))
        events[-2]['Action'] = 'skip'
        self.assertTrue(receipts.verify_required(events, inventory, 0))

    def test_space_real_native_transition_proofs_precede_current_catalog(self):
        import yaml
        package = 'voice/backend/space/internal/store'
        names = ('TestAccountBanFixtureTrusted45Transition', 'TestAccountBanFixtureTrusted50Transition')
        for phase, name in zip(('45', '50'), names):
            inventory = json.loads((ROOT / ('scripts/ci/space_transition' + phase + '_required.json')).read_text(encoding='utf-8'))
            self.assertEqual(inventory['space'], [[package, name]])
        inventory = json.loads((ROOT / 'scripts/ci/space_trusted50_required.json').read_text(encoding='utf-8'))
        self.assertIn([package, 'TestNative50CreatorBornCurrentSchema'], inventory['space'])
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        for name in (*names, 'TestNative50CreatorBornCurrentSchema'):
            entry = next(item for item in exceptions if item['service'] == 'space' and item['test'] == name)
            self.assertEqual((entry['status'], entry['reason']), ('dedicated', 'space-native-installer-integration'))
        jobs = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))['jobs']
        steps = jobs['space-native-installer-integration']['steps']
        step45 = next(item for item in steps if item.get('name') == 'Run Space native45 transition proof')
        step50 = next(item for item in steps if item.get('name') == 'Run trusted Space native50 catalog proofs')
        self.assertLess(steps.index(step45), steps.index(step50))
        self.assertIn('-at migrated45', step45['run'])
        self.assertIn('-at migrated50', step50['run'])
        for name in names[:1]:
            self.assertIn(name, step45['run'])
        for name in (names[1], 'TestNative50CreatorBornCurrentSchema'):
            self.assertIn(name, step50['run'])
        for step in (step45, step50):
            self.assertIn('--reject-skips', step['run'])
            self.assertIn('owned_oid', step['run'])

    def test_file_matrix_requires_pdf_runtime_and_real_renderer(self):
        import yaml
        inventory = json.loads((ROOT / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        cases = [['voice/backend/file', 'TestPDFRuntimeMainUploadsRendersAndRevokesThumbnail'],
                 ['voice/backend/file', 'TestPDFRuntimeRequiresInstalledDependencies'],
                 ['voice/backend/file/internal/pdfproc', 'TestRendererFirstPage']]
        for case in cases:
            self.assertIn(case, inventory['file'])
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        install = next(step for step in workflow['jobs']['backend-go-integration-pr']['steps']
                       if step.get('name') == 'Install File codec tools')
        self.assertIn("matrix.service == 'file'", install['if'])
        self.assertIn('poppler-utils', install['run'])
        for package, name in cases:
            skipped = [{'Package': package, 'Test': name, 'Action': action} for action in ('run', 'skip')]
            skipped.append({'Package': package, 'Action': 'pass'})
            self.assertTrue(receipts.verify_required(skipped, [[package, name]], 0))

    def test_file_matrix_requires_real_codec_cases(self):
        import yaml
        inventory = json.loads((ROOT / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        package = 'voice/backend/file/internal/gifproc'
        cases = ('TestRealCodecRunningProcessCancellation', 'TestRealCodecOutputCapIsPermanent',
                 'TestRealCodecAnimationTimingAndDisposal', 'TestRealCodecZeroDelayAndSingleFrame')
        for name in cases:
            self.assertIn([package, name], inventory['file'])
        skipped = [{'Package': package, 'Test': name, 'Action': action}
                   for name in cases for action in ('run', 'skip')]
        skipped.append({'Package': package, 'Action': 'pass'})
        self.assertTrue(receipts.verify_required(skipped, [[package, name] for name in cases], 0))
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        job = workflow['jobs']['backend-go-integration-pr']
        install = next(step for step in job['steps'] if step.get('name') == 'Install File codec tools')
        self.assertIn("matrix.service == 'file'", install['if'])
        self.assertIn('ffmpeg', install['run'])
        test = next(step for step in job['steps'] if step.get('name') == 'Go integration test (${{ matrix.service }})')
        self.assertEqual(test['env']['GIFPROC_REAL_CODEC'], '1')

    def test_realtime_lifecycle_inventory_requires_both_tagged_proofs(self):
        inventory = json.loads((ROOT / 'scripts/ci/realtime_lifecycle_required.json').read_text(encoding='utf-8'))
        cases = [('lifecycle_fanout_integration_test.go', 'TestActualHubFanoutFreezeWaitsForEveryRecipientAndRejectsCapturedLateScope'),
                 ('lifecycle_transport_integration_test.go', 'TestProtectedRealtimeActualPGReceiptsRequireMTLSExactPrincipalAndReplay')]
        self.assertEqual(inventory['realtime'], [['voice/backend/realtime', name] for _, name in cases])
        for filename, name in cases:
            source = (ROOT / 'src/backend/realtime' / filename).read_text(encoding='utf-8')
            self.assertIn('//go:build realtimep3integration', source)
            self.assertRegex(source, rf'(?m)^func {name}\(t \*testing\.T\)')

    def test_chat_lifecycle_inventory_requires_tagged_storage_proof(self):
        inventory = json.loads((ROOT / 'scripts/ci/chat_lifecycle_required.json').read_text(encoding='utf-8'))
        name = 'TestRealtimeNativeLifecycleReadsOnlyActualChatAndFailsClosedWithoutSchema'
        self.assertEqual(inventory['chat'], [['voice/backend/chat/internal/store', name]])
        source = (ROOT / 'src/backend/chat/internal/store/realtime_lifecycle_integration_test.go').read_text(encoding='utf-8')
        self.assertIn('//go:build realtimep3integration', source)
        self.assertRegex(source, rf'(?m)^func {name}\(t \*testing\.T\)')

    def test_space_autovacuum_inventory_requires_real_worker_proof(self):
        inventory = json.loads((ROOT / 'scripts/ci/space_autovacuum_required.json').read_text(encoding='utf-8'))
        self.assertEqual(inventory['space'], [['voice/backend/space/internal/nativeinstaller', 'TestMaintenanceWindowWithRealAutovacuum']])
        source = (ROOT / 'src/backend/space/internal/nativeinstaller/window_integration_test.go').read_text(encoding='utf-8')
        self.assertRegex(source, r'(?m)^func TestMaintenanceWindowWithRealAutovacuum\(t \*testing\.T\)')
        self.assertIn("backend_type='autovacuum worker'", source)
        self.assertIn('pg_stat_progress_vacuum', source)
        self.assertIn('p.heap_blks_scanned < p.heap_blks_total/2', source)

    def test_space_bot_native51_cases_and_subtests_are_required(self):
        inventory = json.loads((ROOT / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        package = 'voice/backend/space/internal/store'
        parent = 'TestBotInstallationScopeTypedAddAndExactPriorInstallationRemove'
        expected = {parent, parent + '/add', parent + '/remove_exact_installed_generation',
                    'TestBotInstallationScopeCancelBeforeAddPreservesTombstone'}
        self.assertTrue({(package, name) for name in expected}.issubset({tuple(c) for c in inventory['space']}))
        source = (ROOT / 'src/backend/space/internal/store/bot_installation_scope_test.go').read_text(encoding='utf-8')
        self.assertIn('name := "add"', source)
        self.assertIn('name = "remove exact installed generation"', source)
        self.assertIn('migrateBotFixture51', source)
        events = [{'Package': package, 'Test': name, 'Action': action}
                  for name in expected for action in ('run', 'pass')]
        events.append({'Package': package, 'Action': 'pass'})
        self.assertEqual(receipts.verify_required(events, [[package, name] for name in expected], 0), [])
        for event in events:
            if event.get('Test') == parent + '/remove_exact_installed_generation' and event['Action'] == 'pass':
                event['Action'] = 'skip'
        self.assertTrue(receipts.verify_required(events, [[package, name] for name in expected], 0))

    def test_integration_source_audit_rejects_unclassified_and_stale_cases(self):
        source = ROOT / 'scripts/ci/go_integration_source_audit.py'
        spec = importlib.util.spec_from_file_location('source_audit', source)
        audit = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(audit)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            package = root / 'src/backend/chat/store'
            package.mkdir(parents=True)
            path = package / 'store_integration_test.go'
            path.write_text('package store\nimport "testing"\nfunc TestCovered(t *testing.T) {}\n'
                            'func TestDeferred(t *testing.T) {}\n', encoding='utf-8')
            inventory = {'chat': [['voice/backend/chat/store', 'TestCovered']]}
            exceptions = [{'service': 'chat', 'package': 'voice/backend/chat/store',
                           'test': 'TestDeferred', 'source': 'src/backend/chat/store/store_integration_test.go',
                           'status': 'unresolved', 'reason': 'requires a fixture'}]
            audit.audit(root, inventory, exceptions)
            with self.assertRaises(ValueError):
                audit.audit(root, inventory, [])
            with self.assertRaises(ValueError):
                audit.audit(root, inventory, exceptions + exceptions)
            with self.assertRaises(ValueError):
                audit.audit(root, inventory, [dict(exceptions[0], source='wrong.go')])
            with self.assertRaises(ValueError):
                audit.audit(root, inventory, [dict(exceptions[0], status='dedicated', reason='made-up-job')])
            with self.assertRaises(ValueError):
                audit.audit(root, inventory, [dict(exceptions[0], status='subprocess', reason='TestCovered')])
            path.write_text(path.read_text(encoding='utf-8') + 'func TestNew(t *testing.T) {}\n', encoding='utf-8')
            with self.assertRaises(ValueError):
                audit.audit(root, inventory, exceptions)

    def test_current_source_integration_entrypoints_are_classified(self):
        from go_integration_source_audit import audit
        inventory = json.loads((ROOT / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        summary = audit(ROOT, inventory, exceptions)
        self.assertGreater(summary['source_cases'], 0)
        self.assertEqual(summary['unresolved'], sum(item['status'] == 'unresolved' for item in exceptions))

    def test_matrix_inventory_names_existing_tests_for_every_service(self):
        inventory = json.loads((ROOT / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        resolver = (ROOT / 'scripts/ci/resolve-go-matrix.sh').read_text(encoding='utf-8')
        names = re.search(r'(?ms)^GO_SERVICES=\(\s*(.*?)\s*\)', resolver).group(1).split()
        self.assertEqual(set(inventory), set(names))
        for service, cases in inventory.items():
            self.assertTrue(cases, service)
            self.assertEqual(len(cases), len({tuple(case) for case in cases}), service)
            by_package = {}
            for package, name in cases:
                self.assertTrue(package == 'voice/backend/' + service or package.startswith('voice/backend/' + service + '/'))
                if package not in by_package:
                    path = ROOT / 'src/backend' / package.removeprefix('voice/backend/')
                    by_package[package] = '\n'.join(f.read_text(encoding='utf-8') for f in path.glob('*_test.go'))
                self.assertRegex(by_package[package], r'(?m)^func ' + re.escape(name.split('/')[0]) + r'\(t \*testing\.T\)')

    def test_downloaded_matrix_requires_exact_members_and_bound_streams(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            inventory = {'chat': [['voice/backend/chat/store', 'TestStore']],
                         'user': [['voice/backend/user/store', 'TestStore']]}
            inv = root / 'inventory.json'
            inv.write_text(json.dumps(inventory), encoding='utf-8')
            exceptions = root / 'exceptions.json'
            exceptions.write_text('[]', encoding='utf-8')
            for service in inventory:
                package_dir = root / 'src/backend' / service / 'store'
                package_dir.mkdir(parents=True)
                (package_dir / 'store_integration_test.go').write_text(
                    'package store\nimport "testing"\nfunc TestStore(t *testing.T) {}\n', encoding='utf-8')
            script_dir = root / 'scripts/ci'
            script_dir.mkdir(parents=True)
            for name in ('go_integration_matrix.py', 'go_integration_receipts.py', 'go_integration_source_audit.py'):
                shutil.copyfile(ROOT / 'scripts/ci' / name, script_dir / name)
            subprocess.run(['git', 'init', '-q', str(root)], check=True, capture_output=True)
            subprocess.run(['git', '-C', str(root), 'add', 'src', 'scripts', 'inventory.json', 'exceptions.json'],
                           check=True, capture_output=True)
            subprocess.run(['git', '-C', str(root), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                            'commit', '-qm', 'fixture'], check=True, capture_output=True)
            artifacts = root / 'artifacts'
            artifacts.mkdir()
            candidate = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
            identity = {'candidate': candidate, 'run': '123', 'attempt': '1'}
            for service, cases in inventory.items():
                folder = artifacts / ('go-integration-' + service + '-1')
                folder.mkdir()
                package, name = cases[0]
                events = [{'Package': package, 'Test': name, 'Action': action}
                          for action in ('run', 'pass')]
                events.append({'Package': package, 'Action': 'pass'})
                raw = ''.join(json.dumps(event) + '\n' for event in events).encode()
                (folder / 'go-integration-events.jsonl').write_bytes(raw)
                summary = receipts.summarize_events(events)
                summary.update(schema='voice-go-integration-diagnostic-v1', service=service,
                               identity=identity, exit_code=0, event_sha256=hashlib.sha256(raw).hexdigest())
                (folder / 'go-integration-summary.json').write_text(json.dumps(summary), encoding='utf-8')
            command = [sys.executable, str(script_dir / 'go_integration_matrix.py'), 'verify-diagnostics',
                       '--inventory', str(inv), '--candidate', identity['candidate'], '--run', '123',
                       '--attempt', '1', '--services', json.dumps(list(inventory)), '--directory', str(artifacts),
                       '--exceptions', str(exceptions), '--source-root', str(root)]
            def run():
                return subprocess.run(command, capture_output=True).returncode
            self.assertEqual(run(), 0)
            tracked = root / 'src/backend/chat/store/store_integration_test.go'
            original_source = tracked.read_bytes()
            tracked.write_bytes(original_source + b'// changed after candidate\n')
            self.assertNotEqual(run(), 0, 'modified source cannot claim the committed candidate')
            tracked.write_bytes(original_source)
            original_inventory = inv.read_bytes()
            inv.write_bytes(original_inventory + b' ')
            self.assertNotEqual(run(), 0, 'modified inventory cannot claim the committed candidate')
            inv.write_bytes(original_inventory)
            untracked_inventory = root / 'other-inventory.json'
            untracked_inventory.write_bytes(original_inventory)
            alternate = command.copy()
            alternate[alternate.index('--inventory') + 1] = str(untracked_inventory)
            self.assertNotEqual(subprocess.run(alternate, capture_output=True).returncode, 0,
                                'untracked inventory cannot claim the committed candidate')
            extra = tracked.with_name('untracked_test.go')
            extra.write_text('package store\n', encoding='utf-8')
            self.assertNotEqual(run(), 0, 'untracked Go source cannot claim the committed candidate')
            extra.unlink()
            self.assertEqual(run(), 0)
            other_candidate = command.copy()
            other_candidate[other_candidate.index('--candidate') + 1] = 'b' * 40
            self.assertNotEqual(subprocess.run(other_candidate, capture_output=True).returncode, 0)
            user = artifacts / 'go-integration-user-1'
            user.rename(artifacts / 'missing')
            self.assertNotEqual(run(), 0)
            (artifacts / 'missing').rename(user)
            summary_path = user / 'go-integration-summary.json'
            original = summary_path.read_text(encoding='utf-8')
            data = json.loads(original)
            data['identity']['attempt'] = '2'
            summary_path.write_text(json.dumps(data), encoding='utf-8')
            self.assertNotEqual(run(), 0)
            summary_path.write_text(original, encoding='utf-8')
            event_path = user / 'go-integration-events.jsonl'
            raw = event_path.read_bytes()
            skipped_raw = raw.replace(b'"pass"', b'"skip"', 1)
            event_path.write_bytes(skipped_raw)
            self.assertNotEqual(run(), 0)
            skipped_summary = receipts.summarize_events(receipts.read_events(skipped_raw.decode()))
            skipped_summary.update(schema='voice-go-integration-diagnostic-v1', service='user',
                                   identity=identity, exit_code=0,
                                   event_sha256=hashlib.sha256(skipped_raw).hexdigest())
            summary_path.write_text(json.dumps(skipped_summary), encoding='utf-8')
            self.assertNotEqual(run(), 0, 'a matching summary cannot turn a skipped required case into a pass')
            event_path.write_bytes(raw)
            summary_path.write_text(original, encoding='utf-8')
            (artifacts / 'go-integration-unexpected-1').mkdir()
            self.assertNotEqual(run(), 0)

    def test_messaging_threadlist_inventory_requires_postgres_case(self):
        inventory = json.loads((ROOT / 'scripts/ci/messaging_threadlist_required.json').read_text(encoding='utf-8'))
        self.assertEqual(inventory['messaging'], [['voice/backend/messaging/internal/threadlist', 'TestPostgresImmutableNodesAndRollback']])
        source = (ROOT / 'src/backend/messaging/internal/threadlist/postgres_test.go').read_text(encoding='utf-8')
        self.assertRegex(source, r'(?m)^func TestPostgresImmutableNodesAndRollback\(t \*testing\.T\)')

    def test_voice_ledger_inventory_requires_real_redis_case(self):
        inventory = json.loads((ROOT / 'scripts/ci/voice_ledger_required.json').read_text(encoding='utf-8'))
        self.assertEqual(inventory['voice'], [['voice/backend/voice/internal/roomlifecycle', 'TestRedisLedger_RealServerAtomicity']])
        source = (ROOT / 'src/backend/voice/internal/roomlifecycle/redis_ledger_test.go').read_text(encoding='utf-8')
        self.assertRegex(source, r'(?m)^func TestRedisLedger_RealServerAtomicity\(t \*testing\.T\)')

    def test_role_bootstrap_inventory_requires_actual_postgres_case(self):
        inventory = json.loads((ROOT / 'scripts/ci/role_bootstrap_required.json').read_text(encoding='utf-8'))
        self.assertEqual(inventory['role'], [['voice/backend/role', 'TestBootstrapActualRole20PG']])
        source = (ROOT / 'src/backend/role/bootstrap_pg_integration_test.go').read_text(encoding='utf-8')
        self.assertRegex(source, r'(?m)^func TestBootstrapActualRole20PG\(t \*testing\.T\)')

    def test_bot_membership_inventory_requires_both_native_boundaries(self):
        inventory = json.loads((ROOT / 'scripts/ci/bot_membership_required.json').read_text(encoding='utf-8'))
        self.assertEqual(inventory['bot'], [
            ['voice/backend/bot/internal/spacemembership', 'TestNativeMembershipTypedCleanupLostReplyAndMalformedReceipt'],
            ['voice/backend/bot/internal/store', 'TestMembershipNativeTransitions'],
        ])
        for package, name in inventory['bot']:
            path = ROOT / 'src/backend/bot/internal' / package.rsplit('/', 1)[1]
            self.assertTrue(any(re.search(r'(?m)^func ' + name + r'\(t \*testing\.T\)',
                                          f.read_text(encoding='utf-8')) for f in path.glob('*_test.go')))

    def test_user_birth_inventory_covers_all_native_birth_tests(self):
        inventory = json.loads((ROOT / 'scripts/ci/user_birth_required.json').read_text(encoding='utf-8'))
        directory = ROOT / 'src/backend/user/internal/store'
        source = '\n'.join((directory / name).read_text(encoding='utf-8') for name in
                           ('profile_birth_capture_native_test.go', 'profile_primary_birth_capture_native_test.go'))
        names = re.findall(r'(?m)^func (Test(?:Secondary|Primary)ProfileBirthCapture\w+)\(t \*testing\.T\)', source)
        self.assertTrue(names)
        self.assertEqual(inventory['user'], [['voice/backend/user/internal/store', name] for name in names])
        package, name = inventory['user'][0]
        skipped = [{'Package': package, 'Test': name, 'Action': 'run'},
                   {'Package': package, 'Test': name, 'Action': 'skip'},
                   {'Package': package, 'Action': 'pass'}]
        self.assertTrue(receipts.verify_required(skipped, inventory['user'], 0))

    def test_story_lfp_inventory_covers_all_current_package_tests(self):
        inventory = json.loads((ROOT / 'scripts/ci/story_lfp_required.json').read_text(encoding='utf-8'))
        cases = inventory['story']
        self.assertEqual(len(cases), len({tuple(case) for case in cases}))
        discovered = set()
        for name in ('lfpconsentstore', 'lfpconsenttransport'):
            package = 'voice/backend/story/internal/' + name
            for path in (ROOT / 'src/backend/story/internal' / name).glob('*_test.go'):
                source = path.read_text(encoding='utf-8')
                self.assertNotRegex(source, r'(?m)^//go:build ', 'review tagged Story LFP tests explicitly')
                for test in re.findall(r'(?m)^func (Test\w+)\(t \*testing\.T\)', source):
                    discovered.add((package, test))
        self.assertTrue(discovered)
        self.assertEqual({tuple(case) for case in cases}, discovered)

    def test_user_association_inventory_covers_all_current_package_tests(self):
        inventory = json.loads((ROOT / 'scripts/ci/user_association_required.json').read_text(encoding='utf-8'))
        cases = inventory['user']
        package = 'voice/backend/user/internal/identityassociation'
        discovered = set()
        for path in (ROOT / 'src/backend/user/internal/identityassociation').glob('*_test.go'):
            source = path.read_text(encoding='utf-8')
            self.assertNotRegex(source, r'(?m)^//go:build ', 'review tagged User association tests explicitly')
            discovered.update((package, test) for test in
                              re.findall(r'(?m)^func (Test\w+)\(t \*testing\.T\)', source))
        self.assertTrue(discovered)
        self.assertEqual(len(cases), len({tuple(case) for case in cases}))
        self.assertEqual({tuple(case) for case in cases}, discovered)

    def test_notification_quota_inventory_covers_all_current_package_tests(self):
        inventory = json.loads((ROOT / 'scripts/ci/notification_quota_required.json').read_text(encoding='utf-8'))
        cases = inventory['notification']
        package = 'voice/backend/notification/internal/chatquota'
        discovered = set()
        for path in (ROOT / 'src/backend/notification/internal/chatquota').glob('*_test.go'):
            source = path.read_text(encoding='utf-8')
            self.assertNotRegex(source, r'(?m)^//go:build ', 'review tagged quota tests explicitly')
            discovered.update((package, test) for test in
                              re.findall(r'(?m)^func (Test\w+)\(t \*testing\.T\)', source))
        self.assertTrue(discovered)
        self.assertEqual(len(cases), len({tuple(case) for case in cases}))
        self.assertEqual({tuple(case) for case in cases}, discovered)

    def test_user_capture_inventory_requires_each_authority_and_sql_cohort(self):
        inventory = json.loads((ROOT / 'scripts/ci/user_capture_required.json').read_text(encoding='utf-8'))
        cases = inventory['user']
        prefix = 'TestSpaceCaptureNativeProtectedTransport'
        required_names = {prefix, *(prefix + '/' + name for name in
                                    ('auth', 'role', 'gateway', 'moderation', 'canonical-SQL-full-cohort'))}
        required_names.update(('TestSpaceCaptureCanonicalProducerNativeCompleteCohort',
                               'TestSpaceCaptureCanonicalProducerNativeArchiveAndRawErasure'))
        self.assertEqual({name for _, name in cases}, required_names)
        events = [{'Package': package, 'Test': name, 'Action': action}
                  for package, name in cases for action in ('run', 'pass')]
        package = 'voice/backend/user/internal/grpcsvc'
        events.append({'Package': package, 'Action': 'pass'})
        self.assertEqual(receipts.verify_required(events, cases, 0), [])
        missing = [event for event in events if event.get('Test') != prefix + '/canonical-SQL-full-cohort']
        self.assertTrue(receipts.verify_required(missing, cases, 0))

    def test_role_migration_inventory_requires_writer_and_snapshot_fences(self):
        inventory = json.loads((ROOT / 'scripts/ci/role_migration_required.json').read_text(encoding='utf-8'))
        cases = inventory['role']
        package = 'voice/backend/role/internal/store'
        names = {'TestRole13EmptyDownUpRoundtrip'}
        for name in ('TestRole13DownRejectsConcurrentFirstWriters', 'TestRole13DownRefusesExistingEvidence'):
            names.add(name)
            names.update(name + '/' + kind for kind in ('operation', 'session', 'grant'))
        name = 'TestRole13DownRejectsStaleTransactionSnapshots'
        names.add(name)
        names.update(name + '/' + kind for kind in ('repeatable_read', 'serializable'))
        self.assertEqual({tuple(case) for case in cases}, {(package, name) for name in names})
        self.assertEqual(len(cases), len(names))
        source = (ROOT / 'src/backend/role/internal/store/migration13_down_native_test.go').read_text(encoding='utf-8')
        discovered = set(re.findall(r'(?m)^func (TestRole13\w+)\(t \*testing\.T\)', source))
        self.assertEqual(discovered, {name for name in names if '/' not in name})

    def test_user_admission_inventory_names_the_native_transport_case(self):
        inventory = json.loads((ROOT / 'scripts/ci/user_admission_required.json').read_text(encoding='utf-8'))
        source = (ROOT / 'src/backend/user/internal/grpcsvc/admission_eligibility_native_test.go').read_text(encoding='utf-8')
        tests = re.findall(r'(?m)^func (Test\w+)\(t \*testing\.T\)', source)
        self.assertEqual(inventory['user'], [['voice/backend/user/internal/grpcsvc', name] for name in tests])
        package, name = inventory['user'][0]
        skipped = [{'Package': package, 'Test': name, 'Action': 'run'},
                   {'Package': package, 'Test': name, 'Action': 'skip'},
                   {'Package': package, 'Action': 'pass'}]
        self.assertTrue(receipts.verify_required(skipped, inventory['user'], 0))

    def test_required_case_must_run_and_pass_exactly_once(self):
        required = [('voice/backend/chat/internal/store', 'TestStoreIntegration')]
        events = [{'Package': required[0][0], 'Test': required[0][1], 'Action': action}
                  for action in ('run', 'pass')]
        events.append({'Package': required[0][0], 'Action': 'pass'})
        self.assertEqual(receipts.verify_required(events, required, 0), [])
        for altered in ([], events[:1], events[:2],
                        [events[0], dict(events[1], Action='skip'), events[2]],
                        [events[0], dict(events[1], Action='fail'), events[2]], events + events):
            with self.subTest(events=altered):
                self.assertTrue(receipts.verify_required(altered, required, 0))
        self.assertTrue(receipts.verify_required(events, required, 1))

    def test_no_skip_scope_rejects_skipped_subtests_and_packages(self):
        package = 'voice/backend/story/internal/lfpconsentstore'
        events = [{'Package': package, 'Test': 'TestRequired', 'Action': 'run'},
                  {'Package': package, 'Test': 'TestRequired', 'Action': 'pass'},
                  {'Package': package, 'Action': 'pass'}]
        self.assertEqual(receipts.verify_no_skips(events, [package]), [])
        for skipped in ({'Package': package, 'Test': 'TestOther/subcase', 'Action': 'skip'},
                        {'Package': package, 'Action': 'skip'}):
            self.assertTrue(receipts.verify_no_skips(events + [skipped], [package]))
        self.assertEqual(receipts.verify_no_skips(events + [
            {'Package': 'voice/backend/other', 'Test': 'TestOptional', 'Action': 'skip'}], [package]), [])
        with self.assertRaises(ValueError):
            receipts.verify_no_skips(events, [])

    def test_package_identity_matters_and_empty_inventory_is_invalid(self):
        required = [('voice/backend/chat/internal/store', 'TestStoreIntegration')]
        events = [{'Package': 'voice/backend/chat/other', 'Test': required[0][1], 'Action': action}
                  for action in ('run', 'pass')]
        self.assertTrue(receipts.verify_required(events, required, 0))
        with self.assertRaises(ValueError):
            receipts.verify_required(events, [], 0)
        with self.assertRaises(ValueError):
            receipts.verify_required(events, required + required, 0)

    def test_event_stream_rejects_malformed_or_truncated_input(self):
        line = json.dumps({'Package': 'voice/backend/chat', 'Test': 'TestCase', 'Action': 'pass'})
        self.assertEqual(receipts.read_events(line + '\n')[0]['Action'], 'pass')
        for text in ('', line, line + '\n{bad json}\n', '[]\n'):
            with self.subTest(text=text), self.assertRaises(ValueError):
                receipts.read_events(text)

    def test_summary_counts_named_cases_and_preserves_skips(self):
        events = [
            {'Package': 'voice/backend/chat/store', 'Test': 'TestA', 'Action': 'run'},
            {'Package': 'voice/backend/chat/store', 'Test': 'TestA', 'Action': 'pass'},
            {'Package': 'voice/backend/chat/store', 'Test': 'TestB', 'Action': 'run'},
            {'Package': 'voice/backend/chat/store', 'Test': 'TestB', 'Action': 'skip'},
            {'Package': 'voice/backend/chat/store', 'Action': 'pass'},
        ]
        summary = receipts.summarize_events(events)
        self.assertEqual(summary['counts'], {'pass': 1, 'skip': 1, 'fail': 0})
        self.assertEqual(summary['skipped'], [['voice/backend/chat/store', 'TestB']])
        self.assertEqual(summary['packages'], {'voice/backend/chat/store': 'pass'})

    def test_cli_rejects_skipped_required_case(self):
        package = 'voice/backend/analytics/internal/store'
        name = 'TestClickHouseInsertBatch'
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'events.jsonl'
            def run(action):
                events = [{'Package': package, 'Test': name, 'Action': 'run'},
                          {'Package': package, 'Test': name, 'Action': action},
                          {'Package': package, 'Action': 'pass'}]
                path.write_text(''.join(json.dumps(e) + '\n' for e in events), encoding='utf-8')
                return subprocess.run([sys.executable, str(ROOT / 'scripts/ci/go_integration_receipts.py'),
                                       '--events', str(path), '--package', package, '--test', name,
                                       '--exit-code', '0'],
                                      capture_output=True, text=True)
            self.assertEqual(run('pass').returncode, 0)
            self.assertNotEqual(run('skip').returncode, 0)

    def test_cli_writes_source_bound_diagnostic_summary_on_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'events.jsonl'
            out = Path(directory) / 'summary.json'
            events = [{'Package': 'voice/backend/chat/store', 'Test': 'TestA', 'Action': 'run'},
                      {'Package': 'voice/backend/chat/store', 'Test': 'TestA', 'Action': 'fail'},
                      {'Package': 'voice/backend/chat/store', 'Action': 'fail'}]
            path.write_text(''.join(json.dumps(e) + '\n' for e in events), encoding='utf-8')
            result = subprocess.run([sys.executable, str(ROOT / 'scripts/ci/go_integration_receipts.py'),
                '--events', str(path), '--exit-code', '1', '--summary', str(out), '--service', 'chat',
                '--candidate', 'a' * 40, '--run', '123', '--attempt', '1'], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            summary = json.loads(out.read_text(encoding='utf-8'))
            self.assertEqual(summary['identity']['candidate'], 'a' * 40)
            self.assertEqual(summary['counts']['fail'], 1)
            self.assertEqual(summary['exit_code'], 1)

    def test_real_go_json_distinguishes_pass_and_skip(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'go.mod').write_text('module example.org/ci-fixture\n\ngo 1.20\n', encoding='utf-8')
            (root / 'case_test.go').write_text(
                'package fixture\nimport "testing"\n'
                'func TestRuns(t *testing.T) {}\n'
                'func TestSkips(t *testing.T) { t.Skip("fixture") }\n', encoding='utf-8')
            env = dict(os.environ, GOWORK='off', GOFLAGS='', GOTOOLCHAIN='local')
            output = subprocess.check_output(['go', 'test', '-json', '-count=1', './...'],
                                             cwd=root, env=env, text=True)
            events = receipts.read_events(output)
            self.assertEqual(receipts.verify_required(events, [('example.org/ci-fixture', 'TestRuns')], 0), [])
            self.assertTrue(receipts.verify_required(events, [('example.org/ci-fixture', 'TestSkips')], 0))

    def test_matrix_requires_each_selected_service_at_exact_candidate(self):
        identity = {'candidate': 'a' * 40, 'run': '123', 'attempt': '2'}
        required = {'chat': [('voice/backend/chat/store', 'TestIntegration')]}
        events = [{'Package': 'voice/backend/chat/store', 'Test': 'TestIntegration', 'Action': a}
                  for a in ('run', 'pass')]
        events.append({'Package': 'voice/backend/chat/store', 'Action': 'pass'})
        receipt = {'service': 'chat', 'identity': identity, 'exit_code': 0, 'events': events}
        self.assertEqual(receipts.verify_matrix(['chat'], required, [receipt], identity), [])
        for submitted in ([], [receipt, receipt],
                          [dict(receipt, identity=dict(identity, candidate='b' * 40))],
                          [dict(receipt, identity=dict(identity, attempt='1'))],
                          [dict(receipt, exit_code=1)], [dict(receipt, events=events[:1])]):
            with self.subTest(submitted=submitted):
                self.assertTrue(receipts.verify_matrix(['chat'], required, submitted, identity))
        self.assertTrue(receipts.verify_matrix(['chat'], {}, [receipt], identity))
        wrong_package = {'chat': [('voice/backend/other/store', 'TestIntegration')]}
        wrong_events = [dict(event, Package='voice/backend/other/store') for event in events]
        self.assertTrue(receipts.verify_matrix(['chat'], wrong_package,
                                               [dict(receipt, events=wrong_events)], identity))
        with self.assertRaises(ValueError):
            receipts.verify_matrix(['../chat'], required, [receipt], identity)
        with self.assertRaises(ValueError):
            receipts.verify_matrix(['chat'], required, [receipt], dict(identity, candidate='0' * 40))

    def test_receipt_requires_exact_selected_case_inventory(self):
        identity = {'candidate': 'a' * 40, 'run': '123', 'attempt': '2'}
        inventory = {'chat': [['voice/backend/chat/store', 'TestIntegration']]}
        events = [{'Package': 'voice/backend/chat/store', 'Test': 'TestIntegration', 'Action': a}
                  for a in ('run', 'pass')]
        events.append({'Package': 'voice/backend/chat/store', 'Action': 'pass'})
        receipt = receipts.make_receipt('chat', identity, inventory, events, 0)
        self.assertEqual(receipts.verify_matrix(['chat'], inventory, [receipt], identity), [])
        unrelated = {'Package': 'voice/backend/chat/other', 'Test': 'TestOther', 'Action': 'pass',
                     'Output': 'unrelated log text'}
        filtered = receipts.make_receipt('chat', identity, inventory, events + [unrelated], 0)
        self.assertNotIn(unrelated, filtered['events'])
        for altered in ([], events[:1]):
            with self.subTest(events=altered), self.assertRaises(ValueError):
                receipts.make_receipt('chat', identity, inventory, altered, 0)
        with self.assertRaises(ValueError):
            receipts.make_receipt('chat', identity, inventory, events, 1)

    def test_matrix_cli_binds_receipts_and_rejects_missing_or_skipped_cases(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            inventory = {'chat': [['voice/backend/chat/store', 'TestStore']],
                         'user': [['voice/backend/user/store', 'TestStore']]}
            inventory_file = root / 'inventory.json'
            inventory_file.write_text(json.dumps(inventory), encoding='utf-8')
            script = str(ROOT / 'scripts/ci/go_integration_matrix.py')
            common = ['--inventory', str(inventory_file), '--candidate', 'a' * 40,
                      '--run', '123', '--attempt', '1']
            files = []
            for service in inventory:
                package, name = inventory[service][0]
                events = [{'Package': package, 'Test': name, 'Action': action}
                          for action in ('run', 'pass')]
                events.append({'Package': package, 'Action': 'pass'})
                path = root / (service + '.jsonl')
                path.write_text(''.join(json.dumps(event) + '\n' for event in events), encoding='utf-8')
                receipt = root / (service + '.json')
                args = [sys.executable, script, 'produce', *common, '--service', service,
                        '--events', str(path), '--exit-code', '0', '--output', str(receipt)]
                self.assertEqual(subprocess.run(args, capture_output=True).returncode, 0)
                files.append((receipt, path))
            verify = [sys.executable, script, 'verify', *common, '--services', '["chat","user"]']
            both = [arg for receipt, events in files
                    for arg in ('--receipt', str(receipt), '--events', str(events))]
            self.assertEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            strict = [sys.executable, script, 'produce', *common, '--service', 'chat',
                      '--events', str(files[0][1]), '--exit-code', '0', '--reject-skips',
                      '--output', str(root / 'strict.json')]
            self.assertEqual(subprocess.run(strict, capture_output=True).returncode, 0)
            strict_verify = [sys.executable, script, 'verify', *common, '--services', '["chat"]',
                             '--require-no-skips', '--events', str(files[0][1]), '--receipt']
            self.assertEqual(subprocess.run(strict_verify + [str(root / 'strict.json')],
                                            capture_output=True).returncode, 0)
            self.assertNotEqual(subprocess.run(strict_verify + [str(files[0][0])],
                                               capture_output=True).returncode, 0)
            self.assertEqual(json.loads(files[0][0].read_text(encoding='utf-8'))['schema'],
                             'voice-go-integration-receipt-v1')
            self.assertNotEqual(subprocess.run(verify + both[:4], capture_output=True).returncode, 0)
            self.assertNotEqual(subprocess.run(verify + both + both[:4], capture_output=True).returncode, 0)
            self.assertNotEqual(subprocess.run(verify + ['--receipt', str(files[0][0])],
                                               capture_output=True).returncode, 0)
            different_attempt = verify.copy()
            different_attempt[different_attempt.index('--attempt') + 1] = '2'
            self.assertNotEqual(subprocess.run(different_attempt + both, capture_output=True).returncode, 0)
            changed = json.loads(files[0][0].read_text(encoding='utf-8'))
            del changed['schema']
            files[0][0].write_text(json.dumps(changed), encoding='utf-8')
            self.assertNotEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            changed['schema'] = 'voice-go-integration-receipt-v1'
            changed['identity']['candidate'] = 'b' * 40
            files[0][0].write_text(json.dumps(changed), encoding='utf-8')
            self.assertNotEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            changed['identity']['candidate'] = 'a' * 40
            files[0][0].write_text(json.dumps(changed), encoding='utf-8')
            original = files[0][1].read_text(encoding='utf-8')
            files[0][1].write_text(original + '{}\n', encoding='utf-8')
            self.assertNotEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            skipped = original.replace('"pass"', '"skip"', 1)
            files[0][1].write_text(skipped, encoding='utf-8')
            changed['event_sha256'] = hashlib.sha256(skipped.encode('utf-8')).hexdigest()
            files[0][0].write_text(json.dumps(changed), encoding='utf-8')
            self.assertNotEqual(subprocess.run(verify + both, capture_output=True).returncode, 0)
            files[0][1].write_text(original, encoding='utf-8')
            bad = root / 'chat.jsonl'
            bad.write_text(bad.read_text(encoding='utf-8').replace('"pass"', '"skip"', 1), encoding='utf-8')
            failed = subprocess.run([sys.executable, script, 'produce', *common, '--service', 'chat',
                                     '--events', str(bad), '--exit-code', '0',
                                     '--output', str(root / 'invalid.json')], capture_output=True)
            self.assertNotEqual(failed.returncode, 0)
            self.assertFalse((root / 'invalid.json').exists())
            bad.write_text(original + json.dumps({'Package': 'voice/backend/chat/store',
                                                  'Test': 'TestOther', 'Action': 'skip'}) + '\n',
                           encoding='utf-8')
            failed = subprocess.run([sys.executable, script, 'produce', *common, '--service', 'chat',
                                     '--events', str(bad), '--exit-code', '0', '--reject-skips',
                                     '--output', str(root / 'invalid-skip.json')], capture_output=True)
            self.assertNotEqual(failed.returncode, 0)
            self.assertFalse((root / 'invalid-skip.json').exists())


if __name__ == '__main__':
    unittest.main()
