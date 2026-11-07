"""Opt-in fixed Compose fixture cannot satisfy a planned Compose check."""
import json
import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import local_compose_bridge as subject


class ComposeBridgeTests(unittest.TestCase):
    def inputs(self, root):
        return {'helper': str(root / 'creator.exe'), 'helper_sha256': 'a' * 64,
                'postgres_archive': str(root / 'postgres.tar'), 'postgres_archive_sha256': 'b' * 64,
                'postgres_image_id': 'sha256:' + 'c' * 64}

    def test_opt_in_requires_selected_compose_and_complete_inputs(self):
        with tempfile.TemporaryDirectory() as path:
            root = Path(path)
            self.assertIsNone(subject.run({'checks': {'compose-e2e': ['fixture']}}, None, root / 'no-run'))
            self.assertFalse((root / 'no-run').exists())
            with self.assertRaises(ValueError):
                subject.run({'checks': {'backend-go': ['changed.go']}}, self.inputs(root), root / 'not-selected')
            partial = self.inputs(root)
            partial.pop('helper')
            with self.assertRaises(ValueError):
                subject.run({'checks': {'compose-e2e': ['fixture']}}, partial, root / 'invalid')
            self.assertFalse((root / 'invalid').exists())

    def test_executes_broker_and_returns_only_separate_incomplete_report(self):
        with tempfile.TemporaryDirectory() as path:
            root = Path(path)
            inputs = self.inputs(root)
            plan = {'checks': {'compose-e2e': ['test/live.dart'], 'compose-config': ['docker-compose.yml']}}
            output = root / 'bridge'
            def broker(command, *, timeout, stdout, stderr):
                self.assertEqual(timeout, 1800)
                self.assertEqual(command[1:4], ['-B', str(Path(subject.__file__).resolve()), '--worker'])
                self.assertEqual(command[4], str(output / 'fixture'))
                return mock.Mock(returncode=0)
            expected = {'status': 'incomplete', 'schema': 'voice-local-compose-private-v1'}
            with mock.patch.object(subject.supervision, 'run_diagnostic', side_effect=broker), \
                 mock.patch.object(subject, 'validate_diagnostic', return_value=expected):
                result = subject.run(plan, inputs, output)
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['fixture_report'], 'fixture/diagnostic.json')
            self.assertEqual(result['selected_checks'], ['compose-config', 'compose-e2e'])
            self.assertIn('journal is trusted producer evidence, not independent Docker or slot observation',
                          result['limitations'])
            self.assertNotIn('checks', result)
            self.assertEqual(plan, {'checks': {'compose-e2e': ['test/live.dart'], 'compose-config': ['docker-compose.yml']}})

    def test_failed_worker_does_not_validate_or_report_success(self):
        with tempfile.TemporaryDirectory() as path:
            root = Path(path)
            with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=mock.Mock(returncode=1)), \
                 mock.patch.object(subject, 'validate_diagnostic') as validate, self.assertRaises(ValueError):
                subject.run({'checks': {'compose-config': ['docker-compose.yml']}}, self.inputs(root), root / 'bridge')
            validate.assert_not_called()

    def test_validates_retained_fixed_model_and_removed_journal(self):
        with tempfile.TemporaryDirectory() as path:
            root = Path(path)
            inputs = self.inputs(root)
            fixture = root / 'fixture'
            project = 'voiceci-' + 'd' * 32
            scratch = fixture / 'run' / 'tmp' / 'ci-local' / project
            scratch.mkdir(parents=True)
            model = json.dumps(subject.local_compose_private.model(project)).encode()
            (fixture / 'compose-model.json').write_bytes(model)
            (fixture / 'compose.stdout').write_bytes(subject.local_compose_private.MARKER)
            journal_path = scratch / 'ownership.json'
            journal = {'run_id': project, 'resources': [{'kind': kind, 'state': 'removed'} for kind in
                       ('network', 'volume', 'volume', 'volume', 'volume', 'container', 'container')]}
            journal_path.write_text(json.dumps(journal), encoding='utf-8')
            (fixture / 'owner.json').write_text(json.dumps({'run_id': project, 'slot': 0, 'directory': str(scratch)}))
            diagnostic = {'schema': 'voice-local-compose-private-v1', 'status': 'incomplete', 'project': project,
                          'slot': 0, 'archive_sha256': inputs['postgres_archive_sha256'],
                          'postgres_image_id': inputs['postgres_image_id'], 'client_image_id': 'sha256:' + 'e' * 64,
                          'ownership_journal': str(journal_path), 'model_sha256': hashlib.sha256(model).hexdigest()}
            (fixture / 'diagnostic.json').write_text(json.dumps(diagnostic))
            self.assertEqual(subject.validate_diagnostic(fixture, inputs), diagnostic)
            journal['resources'][0]['state'] = 'ready'
            journal_path.write_text(json.dumps(journal), encoding='utf-8')
            with self.assertRaisesRegex(ValueError, 'cleanup'):
                subject.validate_diagnostic(fixture, inputs)
            journal['resources'][0]['state'] = 'removed'
            journal_path.write_text(json.dumps(journal), encoding='utf-8')
            (fixture / 'compose-model.json').write_bytes(model + b' ')
            with self.assertRaisesRegex(ValueError, 'digest'):
                subject.validate_diagnostic(fixture, inputs)


if __name__ == '__main__':
    unittest.main()
