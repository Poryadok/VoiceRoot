import array
import fcntl
import json
import os
import socket
import threading
import unittest
from unittest.mock import patch

import dev_peer_issuer_service as service


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


if __name__ == '__main__':
    unittest.main()
