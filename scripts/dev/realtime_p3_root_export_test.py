import array
import fcntl
import json
import os
from pathlib import Path
import socket
import tempfile
import threading
import unittest
from unittest.mock import patch

import realtime_p3_root_export as exporter


class ExportTests(unittest.TestCase):
    def test_rejects_list_or_nonstring_data(self):
        items = []
        for name, keys in exporter.ASSET_KEYS.items():
            public = name == 'voice-p3-principal-public'
            items.append({'apiVersion': 'v1', 'kind': 'ConfigMap' if public else 'Secret',
                'metadata': {'name': name, 'namespace': 'voice-dev'}, 'data': dict.fromkeys(keys, ''),
                **({'immutable': True} if public else {'type': 'Opaque'})})
        bundle = {'apiVersion': 'v1', 'kind': 'List', 'items': items}
        items[0]['data'] = list(exporter.ASSET_KEYS[items[0]['metadata']['name']])
        with self.assertRaises(ValueError): exporter._check_objects(bundle)
        items[0]['data'] = dict.fromkeys(exporter.ASSET_KEYS[items[0]['metadata']['name']], 1)
        with self.assertRaises(ValueError): exporter._check_objects(bundle)

    def test_sealed_fd_transfers_only_completed_packet(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            opid = 'media-c1-' + '1' * 32
            packet = root / ('p3-' + '1' * 32)
            packet.mkdir(mode=0o700)
            (root / 'p3-issuance.once').write_text(packet.name + '\n')
            (packet / 'complete').write_text('Packet complete; runtime and cluster are not qualified.\n')
            (packet / 'receipt.json').write_text(json.dumps({'status': 'staged', 'namespace': 'voice-dev', 'cluster_applied': False, 'runtime_qualified': False, 'object_names': sorted(exporter.ASSET_KEYS), 'principal_ca_der_sha256': 'a' * 64, 'media_ca_der_sha256': 'b' * 64, 'space_jwks_cert_der_sha256': 'c' * 64}))
            items = []
            for name, keys in exporter.ASSET_KEYS.items():
                public = name == 'voice-p3-principal-public'
                items.append({'apiVersion': 'v1', 'kind': 'ConfigMap' if public else 'Secret',
                    'metadata': {'name': name, 'namespace': 'voice-dev'}, 'data': dict.fromkeys(keys, ''),
                    **({'immutable': True} if public else {'type': 'Opaque'})})
            (packet / 'objects.json').write_text(json.dumps({'kind': 'List', 'apiVersion': 'v1', 'items': items}))
            for path in (root / 'p3-issuance.once', *packet.iterdir()): path.chmod(0o600)
            socket_path = root / 'packet.sock'
            listener = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
            listener.bind(str(socket_path)); listener.listen(1)
            received = []
            def consume():
                for _ in range(2):
                    refused, _ = listener.accept()
                    with refused:
                        self.assertEqual(refused.recv(64), b'')
                connection, _ = listener.accept()
                with connection:
                    msg, anc, _, _ = connection.recvmsg(64, socket.CMSG_SPACE(array.array('i').itemsize))
                    fd = array.array('i', anc[0][2][:array.array('i').itemsize])[0]
                    try:
                        received.append((msg, fcntl.fcntl(fd, fcntl.F_GET_SEALS), json.loads(os.read(fd, 65536))))
                    finally: os.close(fd)
            thread = threading.Thread(target=consume); thread.start()
            ticks = exporter._start_ticks(os.getpid())
            with patch.object(exporter, 'ROOT', root), patch.object(exporter, '_socket_path', return_value=socket_path), patch.object(exporter, '_check_root_custody'), patch.object(exporter, '_pmd_uid', return_value=os.getuid()):
                with self.assertRaises(exporter.ExportError):
                    exporter.export(opid, 'a' * 40, os.getpid() + 1, ticks)
                with self.assertRaises(exporter.ExportError):
                    exporter.export(opid, 'a' * 40, os.getpid(), ticks + 1)
                self.assertFalse((packet / 'handoff-started').exists())
                exporter.export(opid, 'a' * 40, os.getpid(), ticks)
                with self.assertRaises(exporter.ExportError):
                    exporter.export(opid, 'a' * 40, os.getpid(), ticks)
            thread.join(timeout=5); listener.close()
            self.assertFalse(thread.is_alive())
            self.assertEqual(received[0][0], b'P3-PACKET\n')
            self.assertEqual(received[0][1] & exporter.REQUIRED_SEALS, exporter.REQUIRED_SEALS)
            self.assertEqual(received[0][2]['operation_id'], opid)
            self.assertEqual(received[0][2]['source_sha'], 'a' * 40)

    def test_unfinished_packet_fails_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            with patch.object(exporter, 'ROOT', Path(temporary)), patch.object(exporter, '_check_root_custody'):
                with self.assertRaises(exporter.ExportError):
                    exporter.export('media-c1-' + '1' * 32, 'a' * 40, os.getpid(), exporter._start_ticks(os.getpid()))


if __name__ == '__main__': unittest.main()
