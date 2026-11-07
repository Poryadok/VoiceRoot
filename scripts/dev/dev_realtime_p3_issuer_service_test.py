import array
import fcntl
import json
import os
from pathlib import Path
import shutil
import socket
import tempfile
import threading
import unittest
from unittest.mock import patch

import dev_peer_issuer_service as parent
import dev_realtime_p3_issuer_service as service
import realtime_p3_create_only_packet as packet
import reset_credentials as reset


@unittest.skipUnless(os.name == 'posix' and os.geteuid() == 0, 'isolated Linux root fixture required')
class RealtimeServiceTest(unittest.TestCase):
    def test_real_p3_generation_sealed_ack_and_old_reservation_no_replay(self):
        with tempfile.TemporaryDirectory(dir='/root', prefix='p3-service-test-') as temp:
            root = Path(temp)
            archive = root / 'generation1'
            archive.mkdir(mode=0o700)
            material = reset.ResetMaterial(root)
            principal_key, principal_cert = material.ca('principal')
            media_key, media_cert = material.ca('media')
            shutil.copyfile(principal_key, archive / 'principal-ca.key')
            shutil.copyfile(principal_cert, archive / 'principal-ca.crt')
            (archive / 'principal-ca.key').chmod(0o600)
            leaf = reset.ResetMaterial(root)
            leaf.tls('space-fixture', (media_key, media_cert), 'voice-space', 'serverAuth')
            import base64
            space = base64.b64decode(leaf.items[0]['data']['tls.crt'])
            roots = {'principal': principal_cert.read_bytes(), 'media': media_cert.read_bytes(), 'space': space}
            operation = 'media-c1-' + 'a' * 32
            source_sha = 'b' * 40
            request = {'context': {'operation_id': operation, 'commit': source_sha}, 'namespace_uid': 'ns',
                       'lock': {}, 'principal': {}, 'media': {}, 'space': {}, 'objects': {},
                       'capability_sha256': 'c' * 64, 'issuer_source_sha': service.source.REVIEWED_ISSUER_SHA}
            endpoint = root / 'packet.sock'
            listener = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
            listener.bind(str(endpoint))
            listener.listen(1)
            self.addCleanup(listener.close)
            result = []
            def receiver():
                try:
                    channel, _ = listener.accept()
                    with channel:
                        for _ in range(2):
                            if channel.recv(64) != b'PREFLIGHT-REQUEST\n': raise ValueError()
                            channel.send(json.dumps(request).encode())
                            channel.send(b'PREFLIGHT-READY\n')
                        frame, ancillary, _, _ = channel.recvmsg(128, socket.CMSG_SPACE(array.array('i').itemsize))
                        if frame != service.FRAME or len(ancillary) != 1: raise ValueError()
                        fd = array.array('i')
                        fd.frombytes(ancillary[0][2])
                        try:
                            if fcntl.fcntl(fd[0], fcntl.F_GET_SEALS) & service.export.REQUIRED_SEALS != service.export.REQUIRED_SEALS:
                                raise ValueError()
                            envelope = json.loads(os.read(fd[0], 300000))
                            objects = packet.validate(envelope, operation, service.source.REVIEWED_ISSUER_SHA,
                                                      roots['principal'], roots['media'], roots['space'])
                            if set(objects) != set(packet.MEMBERS): raise ValueError()
                        finally:
                            os.close(fd[0])
                        channel.send(service.ACK)
                    result.append('verified')
                except Exception as error:
                    result.append(type(error).__name__)
            thread = threading.Thread(target=receiver)
            thread.start()
            with patch.object(service.export, 'ROOT', root), patch.object(service.issuer, 'PARENT', root), \
                 patch.object(service.issuer, 'ARCHIVE', archive), patch.object(service.export, '_socket_path', return_value=endpoint), \
                 patch.object(service.export, '_pmd_uid', return_value=0), patch.object(service.export, '_start_ticks', return_value=7), \
                 patch.object(service, '_guard', return_value=roots):
                try:
                    service.issue(parent, operation, source_sha, os.getpid(), 7)
                finally:
                    thread.join(30)
                self.assertFalse(thread.is_alive())
                self.assertEqual(result, ['verified'])
                self.assertTrue((root / 'p3-issuance.once').exists())
                self.assertTrue((root / ('p3-' + 'a' * 32) / 'handoff-started').exists())
                with self.assertRaises(service.issuer.IssueError):
                    service.issuer.stage(root / ('p3-' + 'd' * 32), roots['principal'], roots['media'], roots['space'])


if __name__ == '__main__':
    unittest.main()
