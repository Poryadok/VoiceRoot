"""Space-only offline issuance: exact existing CA and one-shot ownership."""
import json
import copy
import array
import fcntl
import os
from pathlib import Path
import subprocess
import socket
import tempfile
import threading
import unittest
from unittest.mock import patch

import space_p3_issuer as issuer
import reset_credentials as reset
import space_p3_issuer_packet as packet
import space_p3_issuer_export as exporter


class IssuerTest(unittest.TestCase):
    def test_existing_media_ca_only_and_distinct_signers(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch); archive = root / 'generation1'; archive.mkdir(mode=0o700)
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','120',
                '-subj','/CN=fixture-media','-addext','basicConstraints=critical,CA:TRUE',
                '-keyout',str(archive/'media-ca.key'),'-out',str(archive/'media-ca.crt')],
                check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            (archive/'media-ca.key').chmod(0o600)
            media=(archive/'media-ca.crt').read_bytes(); dest=root/('space-p3-'+'a'*32)
            with patch.object(issuer,'ARCHIVE',archive), patch.object(issuer,'PARENT',root), patch.object(issuer,'_check_ancestors'):
                with self.assertRaises(issuer.IssueError): issuer.stage(root/('space-p3-'+'b'*32), media+media)
            self.assertTrue((root/'space-p3-issuance.once').exists())
            (root/'space-p3-issuance.once').unlink()
            with patch.object(issuer,'ARCHIVE',archive), patch.object(issuer,'PARENT',root), patch.object(issuer,'_check_ancestors'):
                receipt=issuer.stage(dest,media)
                with self.assertRaises(issuer.IssueError): issuer.stage(root/('space-p3-'+'c'*32),media)
            objects=json.loads((dest/'objects.json').read_bytes())['items']
            self.assertEqual({x['metadata']['name'] for x in objects}, {'voice-space-principal-signing','voice-space-media-server-tls'})
            signer=next(x for x in objects if x['metadata']['name']=='voice-space-principal-signing')
            self.assertEqual(reset.values(signer)['active-kid'],b'current')
            self.assertEqual(receipt['public_jwks'],reset.signer_jwks(signer))
            self.assertEqual((dest/'objects.json').stat().st_mode & 0o777,0o600)
            self.assertNotIn('PRIVATE KEY',json.dumps(receipt))
            self.assertFalse((dest/'media-ca.key').exists())
            envelope={'operation_id':'media-c1-'+'a'*32,'source_sha':'b'*40,
                      'issuer_receipt':receipt,'objects':{'apiVersion':'v1','kind':'List','items':objects}}
            qualified,jwks=packet.validate(envelope,envelope['operation_id'],envelope['source_sha'],media)
            self.assertEqual(jwks,receipt['public_jwks']); self.assertEqual(len(qualified),2)
            changed=copy.deepcopy(envelope)
            changed['objects']['items'][0]['data']['active-kid']='bmV4dA=='
            with self.assertRaises(packet.PacketError): packet.validate(changed,envelope['operation_id'],envelope['source_sha'],media)
            path=root/'packet.sock'; listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            listener.bind(str(path)); listener.listen(1); received=[]
            def consume():
                connection,_=listener.accept()
                with connection:
                    data,anc,_,_=connection.recvmsg(64,socket.CMSG_SPACE(array.array('i').itemsize))
                    fd=array.array('i',anc[0][2][:array.array('i').itemsize])[0]
                    try: received.append((data,fcntl.fcntl(fd,fcntl.F_GET_SEALS),json.loads(os.read(fd,524288))))
                    finally: os.close(fd)
            thread=threading.Thread(target=consume); thread.start()
            ticks=exporter.existing._start_ticks(os.getpid())
            try:
                with patch.object(exporter,'ROOT',root), patch.object(exporter,'_socket_path',return_value=path), \
                     patch.object(exporter.existing,'_check_root_custody'), patch.object(exporter.existing,'_pmd_uid',return_value=os.getuid()):
                    exporter.export(envelope['operation_id'],envelope['source_sha'],os.getpid(),ticks)
                    with self.assertRaises(exporter.ExportError): exporter.export(envelope['operation_id'],envelope['source_sha'],os.getpid(),ticks)
            finally: thread.join(timeout=5); listener.close()
            self.assertFalse(thread.is_alive()); self.assertEqual(received[0][0],b'P3-PACKET\n')
            self.assertEqual(received[0][1]&exporter.existing.REQUIRED_SEALS,exporter.existing.REQUIRED_SEALS)
            self.assertEqual(received[0][2],envelope)


if __name__ == '__main__': unittest.main()
