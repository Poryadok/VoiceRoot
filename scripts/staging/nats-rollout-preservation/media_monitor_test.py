"""Fixed monitoring proof diagnostics; private command output never becomes an error."""
import hashlib
import json
import unittest
from unittest.mock import patch
import commands
import operator_retirement as r
from controller import Blocked


class MonitorTests(unittest.TestCase):
    def test_fixed_monitor_endpoint_and_json(self):
        for role, endpoint, container in (('hub', 'varz', 'nats'), ('analytics', 'leafz', 'nats-leaf')):
            with patch.object(commands, 'capture', return_value=b'{"leafnodes":1}') as run:
                self.assertEqual(r.media_monitor(role), {'leafnodes': 1})
                args = run.call_args.args[0]
                self.assertEqual(args[-1], 'http://127.0.0.1:8222/' + endpoint)
                self.assertEqual(args[args.index('-c') + 1], container)
                self.assertEqual(run.call_args.kwargs['timeout'], 30)
        with patch.object(commands, 'capture') as run:
            for role in ('../auth', 'gateway', 'http://example.invalid', ''):
                with self.assertRaises(Blocked): r.media_monitor(role)
            run.assert_not_called()

    def test_failure_has_safe_role_and_reason_without_stderr(self):
        error = Blocked('command_failed')
        error.diagnostic = {'exit_code': 1, 'timeout': False, 'cause': 'nonzero_exit',
                            'action': 'command', 'stderr_reason': 'connection_refused',
                            'stderr_sha256': 'a' * 64, 'stderr_bytes': 73}
        with patch.object(commands, 'capture', side_effect=error):
            with self.assertRaises(Blocked) as caught: r.media_monitor('analytics')
        self.assertEqual(caught.exception.diagnostic['monitor'], {'role': 'analytics', 'endpoint': 'leafz'})
        self.assertEqual(str(caught.exception), 'command_failed')

    def test_real_capture_classifies_bounded_stderr(self):
        private = b"wget: connection refused private-not-for-public-output"
        with self.assertRaises(Blocked) as caught:
            commands.capture(['/bin/sh', '-c', 'printf %s "$1" >&2; exit 7', '--', private.decode()])
        diagnostic = caught.exception.diagnostic
        self.assertEqual(diagnostic['exit_code'], 7)
        self.assertEqual(diagnostic['stderr_reason'], 'connection_refused')
        self.assertEqual(diagnostic['stderr_sha256'], hashlib.sha256(private).hexdigest())
        self.assertNotIn('private-not-for-public-output', json.dumps(diagnostic))

    def test_timeout_keeps_monitor_identity(self):
        error = commands.failure('command_timeout', timeout=True, cause='timeout')
        with patch.object(commands, 'capture', side_effect=error):
            with self.assertRaises(Blocked) as caught: r.media_monitor('hub')
        self.assertTrue(caught.exception.diagnostic['timeout'])
        self.assertEqual(caught.exception.diagnostic['monitor'], {'role': 'hub', 'endpoint': 'varz'})


if __name__ == '__main__': unittest.main()
