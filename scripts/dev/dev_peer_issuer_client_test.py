import socket
import threading
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import dev_peer_issuer_client as peer
import dev_realtime_p3_issuer_client as realtime


class ClientCancellationTests(unittest.TestCase):
    def _cancel_during_package(self, realtime_mode):
        entered, release, result = threading.Event(), threading.Event(), []
        class Package:
            def verify_current(self, token):
                entered.set()
                if not release.wait(5):
                    raise RuntimeError('fixture timeout')
                return {'run_id': 1, 'run_attempt': 1}
        class Phase:
            def __init__(self):
                self.record = {'status': 'ready'}
                self.package = Package()
            def _authority(self): pass
        class Capability:
            def __init__(self):
                self.record = {'status': 'ready'}
                self.token = 'token'
                self.creator_preflight = SimpleNamespace(package=Package())
            def _guard(self): pass
        target_class = realtime.capability if realtime_mode else peer.preflight
        target_name = 'RealtimeP3Capability' if realtime_mode else 'PeerIssuerPreflightPhase'
        fake_type = Capability if realtime_mode else Phase
        with patch.object(target_class, target_name, fake_type), patch.object(peer.os, 'memfd_create') as memfd:
            value = fake_type()
            trigger = realtime.new_capability_trigger(value, 'token') if realtime_mode else peer.new_preflight_trigger(value, 'token')
            def run():
                try:
                    trigger.run()
                except ValueError:
                    result.append('cancelled')
            thread = threading.Thread(target=run)
            thread.start()
            self.assertTrue(entered.wait(5))
            trigger.cancel()
            release.set()
            thread.join(5)
            self.assertFalse(thread.is_alive())
            self.assertEqual(result, ['cancelled'])
            memfd.assert_not_called()
            with self.assertRaises(ValueError):
                trigger.run()

    def test_cancel_during_package_never_opens_token_or_socket(self):
        for realtime_mode in (False, True):
            with self.subTest(realtime=realtime_mode):
                self._cancel_during_package(realtime_mode)

    def test_cancel_closes_registered_socket_and_wakes_reader(self):
        left, right = socket.socketpair()
        self.addCleanup(right.close)
        control = peer._Control()
        control._start()
        control._register(left)
        ended = threading.Event()
        def read():
            try:
                left.recv(1)
            except OSError:
                pass
            finally:
                ended.set()
        thread = threading.Thread(target=read)
        thread.start()
        control.cancel()
        self.assertTrue(ended.wait(5))
        thread.join(5)
        self.assertEqual(left.fileno(), -1)
        with self.assertRaises(ValueError):
            control._register(right)


if __name__ == '__main__':
    unittest.main()
