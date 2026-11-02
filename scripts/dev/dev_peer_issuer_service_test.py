import array
import fcntl
import json
import os
import socket
import threading
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import dev_peer_issuer_service as service
from dev_peer_issuer_packet_test import PacketTest


class ServiceTests(unittest.TestCase):
    def invoke(self, sealed, source='a' * 40):
        left, right = socket.socketpair(socket.AF_UNIX, socket.SOCK_SEQPACKET)
        fd = os.memfd_create('test-token', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING)
        os.write(fd, b'test-token')
        if sealed:
            fcntl.fcntl(fd, fcntl.F_ADD_SEALS, service.root_export.REQUIRED_SEALS)
        result = []
        def worker():
            try:
                service.serve(right)
                result.append('ok')
            except Exception:
                result.append('rejected')
            finally:
                right.close()
        with patch.object(service.os, 'geteuid', return_value=0), \
             patch.object(service.root_export, '_pmd_uid', return_value=os.getuid()), \
             patch.object(service.root_export, '_start_ticks', return_value=1), \
             patch.object(service.authority, 'verify') as verify, \
             patch.object(service, 'check') as check:
            thread = threading.Thread(target=worker)
            thread.start()
            request = {'action': 'preflight', 'profile': 'role', 'operation_id': 'media-c1-' + 'b' * 32,
                       'source_sha': source, 'run_id': 17, 'run_attempt': 2}
            left.sendmsg([json.dumps(request).encode()], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', [fd]))])
            left.settimeout(3)
            reply = left.recv(32)
            thread.join(3)
            os.close(fd)
            left.close()
            self.assertFalse(thread.is_alive())
            return result, reply, verify.call_count, check.call_count

    def test_fixed_request_and_sealed_token(self):
        result, reply, verify, check = self.invoke(True)
        self.assertEqual((result, reply, verify, check), (['ok'], b'OK\n', 1, 1))

    def test_unsealed_token_rejected_before_authority_or_callback(self):
        result, reply, verify, check = self.invoke(False)
        self.assertEqual((result, reply, verify, check), (['rejected'], b'', 0, 0))

    def test_bad_source_rejected_before_authority_or_callback(self):
        result, reply, verify, check = self.invoke(True, 'bad')
        self.assertEqual((result, reply, verify, check), (['rejected'], b'', 0, 0))

    def test_legacy_reservations_hold_even_if_names_are_now_absent(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(service, 'ROOT', Path(temp)):
            for profile, name in [('creator', 'space-creator-issuance.once'), ('space', 'space-p3-issuance.once')]:
                marker = Path(temp) / name
                service._assert_no_legacy(profile)
                marker.write_text('unreconciled')
                with self.assertRaises(ValueError):
                    service._assert_no_legacy(profile)
                marker.unlink()


class IssuanceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        PacketTest.setUpClass()
        cls.fixture = PacketTest()

    @classmethod
    def tearDownClass(cls):
        PacketTest.tearDownClass()

    def test_lost_ack_keeps_reservation_and_forbids_replay(self):
        profile = 'role'
        operation = 'media-c1-' + 'a' * 32
        source = 'b' * 40
        name = 'voice-role-media-client-tls'
        request = {'namespace_uid': 'ns', 'preflight_sha256': 'c' * 64,
                   'creator_secrets': {name: {'state': 'absent'},
                                       'voice-role-principal-signing': {'state': 'present'}}}
        class Channel:
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def sendmsg(self, payload, ancillary):
                sent = array.array('i')
                sent.frombytes(ancillary[0][2].tobytes())
                self.assert_sealed = (fcntl.fcntl(sent[0], fcntl.F_GET_SEALS) & service.root_export.REQUIRED_SEALS ==
                                      service.root_export.REQUIRED_SEALS)
                return len(payload[0])
            def recvmsg(self, *args): return b'', [], 0, None
        channel = Channel()
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            cert = root / 'fixture.crt'
            cert.write_bytes(self.fixture.media)
            def opened(*args):
                return os.open(cert, os.O_RDONLY), (0, 0, len(self.fixture.media), 0, 0)
            def generate(directory, media, selected, missing, op, sha, namespace, digest):
                envelope = self.fixture.envelope(profile, [name])
                (directory / 'objects.json').write_text(json.dumps(envelope))
                return envelope['issuer_receipt']
            with patch.object(service, 'ROOT', root), patch.object(service, '_connect', return_value=channel), \
                 patch.object(service, '_exchange', return_value=request), patch.object(service, '_guard'), \
                 patch.object(service.root_export, '_check_root_custody'), patch.object(service.existing, '_unchanged', return_value=True), \
                 patch.object(service.custody, '_open', side_effect=opened), patch.object(service.material, 'generate', side_effect=generate):
                with self.assertRaises(ValueError):
                    service.issue(operation, source, profile, 7, 8)
                self.assertTrue(channel.assert_sealed)
                self.assertTrue((root / 'dev-peer-role-issuance.once').exists())
                self.assertTrue((root / ('dev-peer-role-' + 'a' * 32) / 'handoff-started').exists())
                with self.assertRaises(FileExistsError):
                    service.issue(operation, source, profile, 7, 8)


if __name__ == '__main__':
    unittest.main()
