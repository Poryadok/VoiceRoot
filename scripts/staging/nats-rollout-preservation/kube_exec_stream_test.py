"""Fixed Kubernetes pods/exec WebSocket binary transport tests."""
import base64
import hashlib
import io
import json
import unittest
from urllib.parse import parse_qs, urlsplit
from unittest.mock import patch

import kube_json_api as kube


POD_UID = '66666666-6666-4666-8666-666666666666'


def ws_frame(opcode, payload=b''):
    size = len(payload)
    if size < 126:
        header = bytes((0x80 | opcode, size))
    elif size < 65536:
        header = bytes((0x80 | opcode, 126)) + size.to_bytes(2, 'big')
    else:
        header = bytes((0x80 | opcode, 127)) + size.to_bytes(8, 'big')
    return header + payload


class FakeSocket:
    def __init__(self, incoming):
        self.incoming = incoming
        self.stream = None
        self.sent = bytearray()

    def sendall(self, value): self.sent.extend(value)
    def makefile(self, *_args, **_kwargs):
        key = next(line.split(b':',1)[1].strip() for line in self.sent.split(b'\r\n')
                   if line.lower().startswith(b'sec-websocket-key:'))
        accept = base64.b64encode(hashlib.sha1(key + b'258EAFA5-E914-47DA-95CA-C5AB0DC85B11').digest())
        handshake = (b'HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\n'
            b'Connection: Upgrade\r\nSec-WebSocket-Protocol: v5.channel.k8s.io\r\n'
            b'Sec-WebSocket-Accept: ' + accept + b'\r\n\r\n')
        self.stream = io.BytesIO(handshake + self.incoming)
        return self.stream
    def close(self): pass


class Context:
    def wrap_socket(self, raw, *, server_hostname):
        self.server_hostname = server_hostname
        return raw


class ExecStreamTests(unittest.TestCase):
    def test_fixed_exec_command_returns_binary_stdout_and_binds_uid(self):
        archive_frames = b'{"kind":"data"}\n' * 700
        wire = ws_frame(2, b'\x01' + archive_frames)
        wire += ws_frame(2, b'\x03' + b'{"status":"Success"}')
        wire += ws_frame(8)
        sock = FakeSocket(wire)
        api = kube.KubernetesJSONAPI.__new__(kube.KubernetesJSONAPI)
        api.origin = 'https://127.0.0.1:6443'
        api.context = Context()
        api.timeout = 2
        api._request = lambda method, path: {'metadata': {'uid': POD_UID, 'namespace': kube.NAMESPACE}}
        api._operation_token = lambda: 'private-token'
        with patch.object(kube.socket, 'create_connection', return_value=sock) as connect:
            payload = b''.join(api.exec_pod_archive(kube.NAMESPACE, 'pod', POD_UID,
                'a' * 12, 'b' * 32))
        self.assertEqual(payload, archive_frames)
        connect.assert_called_once_with(('127.0.0.1', 6443), timeout=120)
        request = bytes(sock.sent).decode('ascii')
        self.assertIn('GET /api/v1/namespaces/voice-staging/pods/pod/exec?', request)
        self.assertIn('Authorization: Bearer private-token', request)
        self.assertIn('Sec-WebSocket-Protocol: v5.channel.k8s.io', request)
        query = parse_qs(urlsplit(request.split(' ')[1]).query)
        self.assertEqual(query['container'], ['nats-phase'])
        self.assertEqual(query['command'], ['/voice-nats-runner/phase', '--create-and-stream',
            '--operation', 'a' * 12, '--nonce', 'b' * 32, '--pod-uid', POD_UID])
        self.assertEqual(query['stdin'], ['false'])
        self.assertEqual(query['stdout'], ['true'])
        self.assertEqual(query['stderr'], ['false'])
        self.assertEqual(query['tty'], ['false'])

    def test_uid_mismatch_prevents_exec_connection(self):
        api = kube.KubernetesJSONAPI.__new__(kube.KubernetesJSONAPI)
        api._request = lambda *_args: {'metadata': {'uid': '11111111-1111-1111-1111-111111111111',
            'namespace': kube.NAMESPACE}}
        with patch.object(kube.socket, 'create_connection') as connect:
            with self.assertRaisesRegex(kube.KubeAPIError, 'archive_stream_pod_changed'):
                list(api.exec_pod_archive(kube.NAMESPACE, 'pod', POD_UID, 'a' * 12, 'b' * 32))
        connect.assert_not_called()

    def test_bad_channel_or_transport_limit_fails_closed(self):
        bad_channel = FakeSocket(ws_frame(2, b'\x02bad') + ws_frame(8))
        api = kube.KubernetesJSONAPI.__new__(kube.KubernetesJSONAPI)
        api.origin = 'https://127.0.0.1:6443'; api.context = Context(); api.timeout = 2
        api._request = lambda *_args: {'metadata': {'uid': POD_UID, 'namespace': kube.NAMESPACE}}
        api._operation_token = lambda: 'private-token'
        with patch.object(kube.socket, 'create_connection', return_value=bad_channel):
            with self.assertRaisesRegex(kube.KubeAPIError, 'exec_unexpected_channel_data'):
                list(api.exec_pod_archive(kube.NAMESPACE, 'pod', POD_UID, 'a' * 12, 'b' * 32))

        huge = FakeSocket(ws_frame(2, b'\x01' + b'x' * 129) + ws_frame(8))
        with patch.object(kube.socket, 'create_connection', return_value=huge):
            with self.assertRaisesRegex(kube.KubeAPIError, 'archive_stream_too_large'):
                list(api.exec_pod_archive(kube.NAMESPACE, 'pod', POD_UID, 'a' * 12, 'b' * 32,
                    max_bytes=128))


if __name__ == '__main__':
    unittest.main()
