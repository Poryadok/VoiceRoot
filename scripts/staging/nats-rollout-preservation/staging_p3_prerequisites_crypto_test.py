"""Isolated synthetic CA/key fixture; no staging custody or admission."""
import copy
import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import socket
import ssl
import threading
import unittest
from unittest import mock
from types import SimpleNamespace

import staging_p3_prerequisites_crypto as crypto
import staging_p3_prerequisites as p3


class CryptoFixture(unittest.TestCase):
    def test_openssl_does_not_inherit_host_configuration(self):
        with mock.patch.object(crypto.subprocess,'run',wraps=subprocess.run) as invoke:
            crypto._run(['version'])
            self.assertEqual(invoke.call_args.args[0][0],'/usr/bin/openssl')
            self.assertEqual(invoke.call_args.kwargs['env'],{'PATH':'/usr/bin:/bin','LANG':'C'})

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
            crypto.validate_issuer_key(key.read_bytes(),ca)
            with self.assertRaises(crypto.CryptoError): crypto.validate_issuer_key(peerkey.read_bytes(),ca)
            crypto._validity_within(peer_cert,ca)
            with self.assertRaises(crypto.CryptoError): crypto._validity_within(ca,peer_cert)
            with mock.patch.object(crypto,'_run',wraps=crypto._run) as invoke:
                crypto._peer(peer_cert,ca,'serverAuth','voice-space.voice-staging.svc')
                verify=invoke.call_args_list[0].args[0]
                self.assertIn('-trusted',verify); self.assertNotIn('-CAfile',verify)
            fd=os.open(key,os.O_RDONLY|os.O_NOFOLLOW)
            try:
                values=crypto.issue_values(fd,ca,trust,peer_cert,peer_cert,ca,b'fixture-password')
            finally: os.close(fd)
            generated=crypto.issue_private_bytes(key.read_bytes(),ca,trust,peer_cert,peer_cert,ca,b'fixture-password')
            self.assertEqual(crypto.validate_values(generated,trust,peer_cert,peer_cert,ca)['scope'],'offline-material-only')
            issuer={'schema':'voice-staging-p3-issuer-input-v1','receipt':{'fixture':'not-authority'},
                    'ca_certificate_pem':ca.decode(),'ca_private_key_pem':key.read_text(),
                    'trust':{k:v.decode() for k,v in trust.items()},'space_client_certificate_pem':peer_cert.decode(),
                    'space_jwks_certificate_pem':peer_cert.decode(),'space_server_ca_pem':ca.decode(),
                    'replay_password_b64':base64.b64encode(b'fixture-password').decode()}
            self.assertEqual(crypto.validate_issuer_materials(generated,issuer)['scope'],'offline-material-only')
            rows=p3.material_objects(generated)
            for row in rows: row['metadata']['annotations']={'voice.poryadok.io/prerequisites-operation':'fixture'}
            payloads={row['kind']+'/'+row['metadata']['name']:json.dumps(row,sort_keys=True,separators=(',',':')).encode() for row in rows}
            restored=p3.decode_material_payloads(payloads,{ident:'fixture' for ident in payloads})
            self.assertEqual(crypto.validate_issuer_materials(restored,issuer)['scope'],'offline-material-only')
            import resource_claim, resource_claim_test, staging_prerequisites as parent, stage_runtime
            context_data=resource_claim_test.context(); card=resource_claim_test.card(context_data)
            session=resource_claim.Session(context_data,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal()); session.admit(start=True)
            source_rows=json.loads(Path(__file__).with_name('staging_p3_prerequisites_source_fixture.json').read_text())
            source_rows=[r for r in source_rows if r['metadata']['name'] in ('voice-p3-principal-jwks','voice-p3-principal-publisher-config')]
            image='docker.io/library/nginx@sha256:'+'d'*64
            publisher=p3.publisher_objects(source_rows,image,'sha256:'+'d'*64)
            all_rows=p3.mark_operation(p3.material_objects(generated)+publisher,session.operation)
            for row in all_rows:
                if row['kind']=='Service': row['spec']['type']='ClusterIP'
            rawmap={row['kind']+'/'+row['metadata']['name']:json.dumps(row,sort_keys=True,separators=(',',':')).encode() for row in all_rows}
            descriptors=[{'id':i,'kind':i.split('/')[0],'name':i.split('/')[1],'namespace':'voice-staging',
                          'payload_sha256':hashlib.sha256(rawmap[i]).hexdigest(),
                          'desired_sha256':resource_claim.digest(p3.private_semantic(json.loads(rawmap[i])))} for i in p3.inventory_intent()]
            aux={'workload':'voice-p3-principal-jwks','container':'jwks','source_ref':'nginx:stable-alpine','image_ref':image,
                 'platform':{'os':'linux','architecture':'amd64','variant':None},'index_sha256':'e'*64,
                 'manifest_sha256':'d'*64,'config_sha256':'f'*64,'source_authority_sha256':'1'*64}
            plan={'schema':'voice-staging-p3-prerequisites-plan-v1','operation':session.operation,'namespace_uid':'ns',
                  'source_capture_sha256':'2'*64,'source_contract_sha256':'3'*64,'issuer_authority_sha256':'4'*64,
                  'issuer_input_sha256':'5'*64,'auxiliary_image':aux,'objects':descriptors,'preserved':{},
                  'effects':[{**r,'before':{'absent':True,'list_rv':'10'}} for r in descriptors]}
            def kube_run(_self,args,body=None,timeout=30):
                if args[0]=='get': return {'metadata':{'resourceVersion':'11'},'items':[]}
                self.assertEqual(args,['create','--dry-run=server','-f','/dev/stdin','-o','json'])
                result=copy.deepcopy(body)
                if result['kind']=='Service': result['spec'].update(type='ClusterIP',clusterIP='10.2.0.1',clusterIPs=['10.2.0.1'])
                return result
            with mock.patch.object(parent,'require_child',return_value=plan),mock.patch.object(parent,'load_payload',side_effect=lambda s,slot,i:rawmap[i]), \
                 mock.patch.object(parent,'load_issuer',return_value=issuer,create=True),mock.patch.object(p3,'_source_and_image',return_value=(publisher,{'fixture':'not-authority'})), \
                 mock.patch.object(stage_runtime,'Kube',return_value=SimpleNamespace(run=lambda args,body=None,timeout=30:kube_run(None,args,body,timeout))):
                self.assertTrue(p3.preflight_payloads(session))
                bad=copy.deepcopy(issuer); bad['replay_password_b64']=base64.b64encode(b'wrong').decode()
                with mock.patch.object(parent,'load_issuer',return_value=bad),self.assertRaises(p3.PrerequisiteError): p3.preflight_payloads(session)
                with mock.patch.object(parent,'load_issuer',side_effect=[issuer,bad]),self.assertRaises(p3.PrerequisiteError): p3.preflight_payloads(session)
                calls={'lists':0}
                def late(args,body=None,timeout=30):
                    result=kube_run(None,args,body,timeout)
                    if args[0]=='get':
                        calls['lists']+=1
                        if calls['lists']==5: result['items']=[{'metadata':{'name':'voice-realtime-principal-signing'}}]
                    return result
                with mock.patch.object(stage_runtime,'Kube',return_value=SimpleNamespace(run=late)),self.assertRaises(p3.PrerequisiteError):
                    p3.preflight_payloads(session)
            bad=copy.deepcopy(issuer); bad['replay_password_b64']=base64.b64encode(b'wrong').decode()
            with self.assertRaises(crypto.CryptoError): crypto.validate_issuer_materials(generated,bad)
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
