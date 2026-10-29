import socket
import struct
import threading
import unittest
from unittest.mock import patch

import dev_peer_issuer_client as peer


class ClientCancellationTests(unittest.TestCase):
    def _cancel_during_package(self):
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
        with patch.object(peer.preflight, 'PeerIssuerPreflightPhase', Phase), patch.object(peer.os, 'memfd_create') as memfd:
            trigger = peer.new_preflight_trigger(Phase(), 'token')
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
        self._cancel_during_package()

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

    def test_worker_does_not_read_transitioning_journal_after_service_ok(self):
        class Package:
            def verify_current(self, token): return {'run_id': 1, 'run_attempt': 1}
        class Phase:
            def __init__(self):
                self.record = {'status': 'ready'}
                self.package = Package()
                self.profile = 'creator'
                self.fence = type('Fence', (), {'context': {'operation_id': 'media-c1-' + 'a' * 32, 'commit': 'b' * 40}})()
                self.calls = 0
            def _authority(self):
                self.calls += 1
                if self.calls > 1: raise ValueError('receiver is completing its ledger transition')
        class Channel:
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def settimeout(self, value): pass
            def connect(self, path): pass
            def getsockopt(self, *args): return struct.pack('3i', 1, 0, 0)
            def sendmsg(self, data, ancillary): return len(data[0])
            def recvmsg(self, *args): return b'OK\n', [], 0, None
        phase = Phase()
        with patch.object(peer.preflight, 'PeerIssuerPreflightPhase', Phase), patch.object(peer.socket, 'socket', return_value=Channel()):
            peer.new_preflight_trigger(phase, 'token').run()
        self.assertEqual(phase.calls, 1)


if __name__ == '__main__':
    unittest.main()
