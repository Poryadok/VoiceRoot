"""Tests for the fixed idle-before-validation NATS sidecar supervisor."""
import importlib.machinery
import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import Mock


path = Path(__file__).with_name('nats-supervisor')
loader = importlib.machinery.SourceFileLoader('nats_supervisor', str(path))
spec = importlib.util.spec_from_loader('nats_supervisor', loader)
supervisor_module = importlib.util.module_from_spec(spec)
loader.exec_module(supervisor_module)


class Child:
    def __init__(self):
        self.pid = 4242
        self.code = None
        self.terminated = False
        self.waited = []

    def poll(self):
        return self.code

    def terminate(self):
        self.terminated = True
        self.code = 0

    def wait(self, timeout=None):
        self.waited.append(timeout)
        return self.code


class BrokerSupervisorTests(unittest.TestCase):
    def test_starts_only_after_explicit_start_and_uses_fixed_nats_argv(self):
        children = []
        def popen(command, **kwargs):
            child = Child(); children.append(child)
            self.assertEqual(command, ['/usr/local/bin/nats-server', '--config', '/broker-config/server.conf'])
            self.assertIs(kwargs['stdin'], subprocess.DEVNULL)
            self.assertIs(kwargs['stdout'], subprocess.DEVNULL)
            self.assertIs(kwargs['stderr'], subprocess.DEVNULL)
            self.assertTrue(kwargs['close_fds'])
            return child
        broker = supervisor_module.BrokerSupervisor(popen=popen)
        self.assertIsNone(broker.child)
        self.assertEqual(broker.handle('status'), {'running': False})
        self.assertEqual(broker.handle('start'), {'running': True})
        self.assertEqual(len(children), 1)
        with self.assertRaisesRegex(ValueError, 'already_running'):
            broker.handle('start')

    def test_canonical_restart_is_stop_then_new_child_start(self):
        children = []
        def popen(command, **_kwargs):
            child = Child(); children.append(child); return child
        broker = supervisor_module.BrokerSupervisor(popen=popen)
        broker.handle('start')
        first = children[0]
        self.assertEqual(broker.handle('stop'), {'running': False})
        self.assertTrue(first.terminated)
        self.assertEqual(first.waited, [30])
        self.assertEqual(broker.handle('start'), {'running': True})
        self.assertEqual(len(children), 2)
        self.assertIsNot(children[0], children[1])

    def test_stop_timeout_and_shutdown_while_running_fail_closed(self):
        child = Mock()
        child.poll.return_value = None
        child.wait.side_effect = subprocess.TimeoutExpired('nats-server', 30)
        broker = supervisor_module.BrokerSupervisor(popen=Mock(return_value=child))
        broker.handle('start')
        with self.assertRaisesRegex(ValueError, 'stop_incomplete'):
            broker.handle('stop')
        with self.assertRaisesRegex(ValueError, 'shutdown_while_running'):
            broker.handle('shutdown')

    def test_preflight_reports_fixed_binary_and_same_child_orderly_stop(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / 'nats-server'
            binary.write_bytes(b'fixed nats executable')
            binary.chmod(0o555)
            child = Child()
            def popen(command, **_kwargs):
                self.assertEqual(command, [str(binary), '--config', '/broker-config/server.conf'])
                return child
            broker = supervisor_module.BrokerSupervisor(popen=popen, preflight=True,
                server=str(binary))
            self.assertEqual(broker.handle('status'), {'running': False,
                'pid': None, 'started_at_ns': None, 'exit_code': None,
                'finished_at_ns': None, 'nats_binary_sha256': __import__('hashlib').sha256(
                    b'fixed nats executable').hexdigest()})
            started = broker.handle('start')
            self.assertTrue(started['running'])
            self.assertEqual(started['pid'], child.pid)
            self.assertEqual(started['nats_binary_sha256'], __import__('hashlib').sha256(
                b'fixed nats executable').hexdigest())
            stopped = broker.handle('stop')
            self.assertFalse(stopped['running'])
            self.assertEqual(stopped['pid'], child.pid)
            self.assertEqual(stopped['exit_code'], 0)
            self.assertGreaterEqual(stopped['finished_at_ns'], stopped['started_at_ns'])
            self.assertEqual(stopped['nats_binary_sha256'], started['nats_binary_sha256'])

    def test_protocol_is_closed_and_malformed_requests_do_not_start_child(self):
        broker = supervisor_module.BrokerSupervisor(popen=Mock())
        valid = b'{"action":"status","schema":"voice-nats-supervisor-v1"}'
        self.assertEqual(supervisor_module._response(broker, valid), {
            'schema': 'voice-nats-supervisor-v1', 'ok': True, 'running': False})
        for raw in (
            b'{"action":"start","schema":"voice-nats-supervisor-v1","argv":["x"]}',
            b'{"schema":"voice-nats-supervisor-v1"}',
            b'{"action":"start","schema":"voice-nats-supervisor-v1"} trailing',
        ):
            with self.subTest(raw=raw):
                self.assertEqual(supervisor_module._response(broker, raw), {
                    'schema': 'voice-nats-supervisor-v1', 'ok': False})
        broker._popen.assert_not_called()


if __name__ == '__main__':
    unittest.main()
