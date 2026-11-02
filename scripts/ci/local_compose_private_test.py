"""The private Compose diagnostic has one fixed model and no host cleanup."""
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import local_compose_private as subject


class PrivateComposeTests(unittest.TestCase):
    def test_fixture_is_audited_and_exact_model_is_sent_to_compose(self):
        with tempfile.TemporaryDirectory() as directory:
            checkout = Path(directory)
            project = 'voiceci-' + 'a' * 32
            scratch = checkout / 'tmp' / 'ci-local' / project
            scratch.mkdir(parents=True)
            owner = mock.Mock(run_id=project, directory=scratch)
            def execute(client, command, out, err, timeout, stdin=None):
                self.assertEqual(client, 'client')
                if 'config' in command:
                    self.assertEqual(json.load(stdin), subject.model(project))
                    out.write(json.dumps(subject.model(project), indent=2).encode())
                    return 0
                self.assertEqual(command, subject.COMMAND + ['-p', project, '-f', '-', 'run', '--rm',
                                                             '--no-deps', '--pull', 'never', '-T', 'fixture'])
                self.assertEqual(stdin.read(), json.dumps(subject.model(project), indent=2).encode())
                out.write(subject.MARKER)
                return 0
            owner.execute.side_effect = execute
            with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='private'):
                result = subject.run_fixture(owner, 'daemon', 'client', checkout, io.BytesIO(), io.BytesIO(), checkout / 'model.json')
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['project'], project)
            self.assertEqual((checkout / 'model.json').read_bytes(), json.dumps(subject.model(project), indent=2).encode())
            self.assertEqual(result['model_sha256'], subject.hashlib.sha256((checkout / 'model.json').read_bytes()).hexdigest())

    def test_audit_failure_never_executes_compose(self):
        owner = mock.Mock(run_id='voiceci-' + 'a' * 32)
        with mock.patch.object(subject.local_compose_audit, 'audit', return_value=['unsafe']), \
             self.assertRaisesRegex(ValueError, 'audit'):
            subject.run_fixture(owner, 'daemon', 'client', Path('.'), io.BytesIO(), io.BytesIO(), Path('unused'))
        self.assertEqual(owner.execute.call_count, 0)

    def test_failure_wrong_output_and_changed_peer_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            checkout = Path(directory)
            project = 'voiceci-' + 'b' * 32
            scratch = checkout / 'tmp' / 'ci-local' / project
            scratch.mkdir(parents=True)
            for index, (code, payload, addresses) in enumerate(((1, subject.MARKER, ['private']),
                                              (0, b'wrong', ['private', 'private']),
                                              (0, subject.MARKER, ['private', 'changed']))):
                owner = mock.Mock(run_id=project, directory=scratch)
                def execute(client, command, out, err, timeout, stdin=None):
                    if 'config' in command:
                        out.write(json.dumps(subject.model(project)).encode())
                        return 0
                    out.write(payload)
                    return code
                owner.execute.side_effect = execute
                with self.subTest(code=code, payload=payload, addresses=addresses), \
                     mock.patch.object(subject.local_chat_private, 'inspect_pair', side_effect=addresses), \
                     self.assertRaises(ValueError):
                    subject.run_fixture(owner, 'daemon', 'client', checkout, io.BytesIO(), io.BytesIO(), checkout / ('model-' + str(index) + '.json'))

    def test_rejects_rendered_drift_duplicate_keys_and_interpolation(self):
        with tempfile.TemporaryDirectory() as directory:
            checkout = Path(directory)
            project = 'voiceci-' + 'c' * 32
            scratch = checkout / 'tmp' / 'ci-local' / project
            scratch.mkdir(parents=True)
            rendered = json.dumps(subject.model(project)).encode()
            for value in (rendered.replace(b'postgres:16-bookworm', b'postgres:latest'),
                          b'{"name":"wrong","name":"' + project.encode() + b'"}',
                          rendered.replace(b'postgres:16-bookworm', b'postgres:$TAG'),
                          b' ' * 65537):
                owner = mock.Mock(run_id=project, directory=scratch)
                def execute(client, command, out, err, timeout, stdin=None):
                    out.write(value)
                    return 0
                owner.execute.side_effect = execute
                with self.subTest(value=value), \
                     mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='private'), \
                     self.assertRaises(ValueError):
                    subject.run_fixture(owner, 'daemon', 'client', checkout, io.BytesIO(), io.BytesIO(), checkout / 'model.json')
                self.assertEqual(owner.execute.call_count, 1)

    def test_setup_interruption_always_invokes_owned_cleanup(self):
        scheduled = mock.Mock()
        scheduled.run.create_resource.side_effect = KeyboardInterrupt()
        with self.assertRaises(KeyboardInterrupt):
            subject.run_owned(scheduled, mock.Mock(), Path('.'), Path('.'),
                              'helper', 'a' * 64, 'archive', 'b' * 64, 'sha256:' + 'c' * 64)
        scheduled.close.assert_called_once_with()

    def test_publishes_exact_owner_and_slot_before_work(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            owner = mock.Mock(run_id='voiceci-' + 'd' * 32,
                              directory=output / 'run' / 'tmp' / 'ci-local' / ('voiceci-' + 'd' * 32))
            scheduled = mock.Mock(run=owner, slot=1)
            subject.publish_owner(scheduled, output)
            self.assertEqual(json.loads((output / 'owner.json').read_text(encoding='utf-8')),
                             {'run_id': owner.run_id, 'slot': 1, 'directory': str(owner.directory)})
            with self.assertRaises(FileExistsError):
                subject.publish_owner(scheduled, output)

    def test_rejects_changed_retained_model(self):
        with tempfile.TemporaryDirectory() as directory:
            checkout = Path(directory)
            project = 'voiceci-' + 'e' * 32
            scratch = checkout / 'tmp' / 'ci-local' / project
            scratch.mkdir(parents=True)
            owner = mock.Mock(run_id=project, directory=scratch)
            def execute(client, command, out, err, timeout, stdin=None):
                if 'config' in command:
                    out.write(json.dumps(subject.model(project)).encode())
                else:
                    stdin.seek(0)
                    stdin.write(b'x')
                    stdin.flush()
                    out.write(subject.MARKER)
                return 0
            owner.execute.side_effect = execute
            with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='private'), \
                 self.assertRaisesRegex(ValueError, 'model changed'):
                subject.run_fixture(owner, 'daemon', 'client', checkout, io.BytesIO(), io.BytesIO(), checkout / 'model.json')


if __name__ == '__main__':
    unittest.main()
