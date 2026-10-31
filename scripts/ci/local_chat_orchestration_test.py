"""Selected Chat diagnostic never treats failed work or cleanup as a pass."""
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import local_chat_orchestration as subject


class OrchestrationTests(unittest.TestCase):
    def test_creator_uses_broker_binary_input_and_preserves_uncertainty(self):
        request = {'name': 'owned'}
        completed = mock.Mock(returncode=0, stdout=('a' * 64 + '\n').encode())
        with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=completed) as run, \
             mock.patch.object(subject, '_verify_file'):
            self.assertEqual(subject._create_daemon('helper.exe', 'f' * 64, {'DOCKER_HOST': 'local'}, request), 'a' * 64)
        self.assertEqual(run.call_args.kwargs['input'], b'{"name":"owned"}')
        self.assertEqual(run.call_args.kwargs['timeout'], 90)
        completed.returncode = 1
        with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=completed), \
             mock.patch.object(subject, '_verify_file'), self.assertRaises(subject.OwnershipError):
            subject._create_daemon('helper.exe', 'f' * 64, {}, request)

    def test_both_execution_and_cleanup_failures_are_preserved(self):
        scheduled = mock.Mock()
        scheduled.close.side_effect = RuntimeError('cleanup uncertain')
        with self.assertRaises(BaseExceptionGroup) as caught:
            subject._finish(scheduled, ValueError('work failed'))
        self.assertEqual([str(e) for e in caught.exception.exceptions], ['work failed', 'cleanup uncertain'])

    def test_preflight_status_stays_incomplete(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            result = subject._report({'head': 'a' * 40}, {'status': 'incomplete'}, 'run/journal.json', 0)
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['diagnostic']['status'], 'incomplete')
            self.assertEqual(result['slot'], 0)

    def test_interrupt_during_owned_work_still_closes_scheduled_run(self):
        scheduled = mock.Mock()
        scheduled.run.create_resource.side_effect = KeyboardInterrupt()
        with mock.patch.object(subject, '_verify_file'):
            with self.assertRaises(KeyboardInterrupt):
                subject._run_owned(scheduled, mock.Mock(), {}, Path('.'), Path('.'), Path('.'),
                                   'helper', 'a' * 64, 'archive', 'b' * 64, 'sha256:' + 'c' * 64)
        scheduled.close.assert_called_once_with()

    def test_socket_readiness_retries_and_verifies_owned_peers(self):
        owner = mock.Mock()
        def execute(name, command, out, err, timeout):
            if owner.execute.call_count == 1:
                return 7
            out.write(b'OK')
            return 0
        owner.execute.side_effect = execute
        with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='172.18.0.2') as inspect, \
             mock.patch.object(subject.time, 'sleep'):
            subject._wait_daemon(owner, 'daemon', 'client', 5)
        self.assertEqual(owner.execute.call_count, 2)
        self.assertEqual(inspect.call_count, 3)
        self.assertIn('--unix-socket', owner.execute.call_args.args[1])

    def test_socket_readiness_rejects_changed_peer_and_uncertain_execution(self):
        owner = mock.Mock()
        owner.execute.side_effect = subject.OwnershipError('uncertain')
        with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
             self.assertRaises(subject.OwnershipError):
            subject._wait_daemon(owner, 'daemon', 'client', 5)
        owner.execute.side_effect = None
        owner.execute.return_value = 0
        with mock.patch.object(subject.local_chat_private, 'inspect_pair', side_effect=['172.18.0.2', '172.18.0.3']), \
             self.assertRaisesRegex(ValueError, 'endpoint changed'):
            subject._wait_daemon(owner, 'daemon', 'client', 5)

    def test_readiness_budget_exhaustion_cleans_owned_run_without_image_load(self):
        scheduled = mock.Mock()
        owner = scheduled.run
        owner.create_resource.side_effect = ['network', 'data', 'legacy', 'socket', 'workspace']
        owner.create_container.return_value = 'client'
        owner.create_rootless_container.return_value = 'daemon'
        owner.execute.side_effect = [0, 7]
        engine = mock.Mock()
        with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
             mock.patch.object(subject.time, 'monotonic', side_effect=[0, 0, 61]), \
             self.assertRaisesRegex(ValueError, 'did not become ready'):
            subject._run_owned(scheduled, engine, {}, Path('.'), Path('.'), Path('.'),
                               'helper', 'a' * 64, 'archive', 'b' * 64, 'sha256:' + 'c' * 64)
        self.assertEqual(owner.execute.call_count, 2)
        owner.load_canonical_rootless_image.assert_not_called()
        scheduled.close.assert_called_once_with()


if __name__ == '__main__':
    unittest.main()
