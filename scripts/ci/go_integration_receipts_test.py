"""Required Go integration cases must execute, not merely be selected."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('receipts', ROOT / 'scripts/ci/go_integration_receipts.py')
receipts = importlib.util.module_from_spec(spec)
spec.loader.exec_module(receipts)


class GoIntegrationReceiptTests(unittest.TestCase):
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


if __name__ == '__main__':
    unittest.main()
