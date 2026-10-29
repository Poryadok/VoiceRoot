"""Isolated synthetic CA/key fixture; no staging custody or admission."""
import copy
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import staging_p3_prerequisites_crypto as crypto


class CryptoFixture(unittest.TestCase):
    def test_staging_issuer_and_peer_certificates_are_bound(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); key=root/'ca.key'; cert=root/'ca.crt'; peerkey=root/'peer.key'; csr=root/'peer.csr'; peer=root/'peer.crt'; ext=root/'ext'
            def run(*args): subprocess.run(['openssl',*map(str,args)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            run('req','-x509','-newkey','rsa:2048','-nodes','-keyout',key,'-out',cert,'-days','40','-subj','/CN=fixture-ca',
                '-addext','basicConstraints=critical,CA:TRUE')
            os.chmod(key,0o600)
            run('req','-newkey','rsa:2048','-nodes','-keyout',peerkey,'-out',csr,'-subj','/CN=voice-space')
            ext.write_text('basicConstraints=critical,CA:FALSE\nsubjectAltName=DNS:voice-space.voice-staging.svc\nextendedKeyUsage=serverAuth,clientAuth\n')
            run('x509','-req','-in',csr,'-CA',cert,'-CAkey',key,'-set_serial','1234','-days','2','-extfile',ext,'-out',peer)
            ca=cert.read_bytes(); peer_cert=peer.read_bytes(); trust={name:ca for name in crypto.TRUST_KEYS}
            fd=os.open(key,os.O_RDONLY|os.O_NOFOLLOW)
            try:
                values=crypto.issue_values(fd,ca,trust,peer_cert,peer_cert,ca,b'fixture-password')
            finally: os.close(fd)
            proof=crypto.validate_values(values,trust,peer_cert,peer_cert,ca)
            self.assertEqual(proof['namespace'],'voice-staging')
            changed=copy.deepcopy(values); changed['voice-p3-principal-public']['realtime.json']=b'{"keys":[]}'
            with self.assertRaises(crypto.CryptoError): crypto.validate_values(changed,trust,peer_cert,peer_cert,ca)
            changed=copy.deepcopy(values); changed['voice-chat-realtime-p3-tls']=copy.deepcopy(values['voice-realtime-p3-tls'])
            with self.assertRaises(crypto.CryptoError): crypto.validate_values(changed,trust,peer_cert,peer_cert,ca)
            with self.assertRaises(crypto.CryptoError): crypto.validate_values(values,{**trust,'space-jwks-ca.crt':b'wrong'},peer_cert,peer_cert,ca)
            os.chmod(key,0o604); fd=os.open(key,os.O_RDONLY|os.O_NOFOLLOW)
            try:
                with self.assertRaises(crypto.CryptoError): crypto.issue_values(fd,ca,trust,peer_cert,peer_cert,ca,b'fixture-password')
            finally: os.close(fd)


if __name__=='__main__': unittest.main()
