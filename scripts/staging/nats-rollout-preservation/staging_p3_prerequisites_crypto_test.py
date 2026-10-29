"""Isolated synthetic CA/key fixture; no staging custody or admission."""
import copy
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import socket
import ssl
import threading
import unittest

import staging_p3_prerequisites_crypto as crypto
import staging_p3_prerequisites as p3


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
            servercert=root/'server.crt'; serverkey=root/'server.key'
            servercert.write_bytes(values['voice-p3-principal-jwks-tls']['tls.crt'])
            serverkey.write_bytes(values['voice-p3-principal-jwks-tls']['tls.key'])
            context=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER); context.load_cert_chain(servercert,serverkey)
            expected=values['voice-p3-principal-public']['realtime.json']
            leaf_hash=hashlib.sha256(ssl.PEM_cert_to_DER_cert(servercert.read_text())).hexdigest()
            def exchange(body,status=b'200 OK'):
                listener=socket.socket(); listener.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1)
                listener.bind(('127.0.0.1',8443)); listener.listen(1)
                def server():
                    try:
                        conn,_=listener.accept()
                        with context.wrap_socket(conn,server_side=True) as tls:
                            tls.recv(4096); tls.sendall(b'HTTP/1.0 '+status+b'\r\nContent-Length: '+str(len(body)).encode()+b'\r\n\r\n'+body)
                    except OSError: pass
                    finally: listener.close()
                thread=threading.Thread(target=server,daemon=True); thread.start()
                return thread
            thread=exchange(expected); p3.probe_jwks('127.0.0.1',ca,expected,leaf_hash); thread.join(5); self.assertFalse(thread.is_alive())
            thread=exchange(b'wrong')
            with self.assertRaises(p3.PrerequisiteError): p3.probe_jwks('127.0.0.1',ca,expected,leaf_hash)
            thread.join(5); self.assertFalse(thread.is_alive())
            thread=exchange(expected,b'302 Found')
            with self.assertRaises(p3.PrerequisiteError): p3.probe_jwks('127.0.0.1',ca,expected,leaf_hash)
            thread.join(5); self.assertFalse(thread.is_alive())
            thread=exchange(expected)
            with self.assertRaises(p3.PrerequisiteError): p3.probe_jwks('127.0.0.1',ca,expected,'0'*64)
            thread.join(5); self.assertFalse(thread.is_alive())
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
