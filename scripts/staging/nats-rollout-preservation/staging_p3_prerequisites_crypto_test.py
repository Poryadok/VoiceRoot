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
                 mock.patch.object(stage_runtime,'Kube',return_value=SimpleNamespace(run=lambda args,body=None,timeout=30:kube_run(None,args,body,timeout),
                     get=lambda kind,name:{'kind':'Namespace','metadata':{'name':'voice-staging','uid':'ns'}})):
                self.assertTrue(p3.preflight_payloads(session))
                def no_allocation(args,body=None,timeout=30):
                    result=kube_run(None,args,body,timeout)
                    if args[0]=='create' and result['kind']=='Service':
                        result['spec'].pop('clusterIP'); result['spec'].pop('clusterIPs')
                    return result
                with mock.patch.object(stage_runtime,'Kube',return_value=SimpleNamespace(run=no_allocation,get=lambda kind,name:{'kind':'Namespace','metadata':{'name':'voice-staging','uid':'ns'}})):
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
                with mock.patch.object(stage_runtime,'Kube',return_value=SimpleNamespace(run=late,get=lambda kind,name:{'kind':'Namespace','metadata':{'name':'voice-staging','uid':'ns'}})),self.assertRaises(p3.PrerequisiteError):
                    p3.preflight_payloads(session)
                with mock.patch.object(stage_runtime,'Kube',return_value=SimpleNamespace(run=late,get=lambda kind,name:{'kind':'Namespace','metadata':{'name':'voice-staging','uid':'foreign'}})),self.assertRaises(p3.PrerequisiteError):
                    p3.preflight_payloads(session)
            live={ident:json.loads(rawmap[ident]) for ident in rawmap}; done={}
            for ident,row in live.items():
                kind,name=ident.split('/'); row['metadata'].update(uid='uid-'+name,resourceVersion='1')
                if kind=='Deployment': row['metadata']['generation']=1
                if kind=='Service': row['spec'].update(clusterIP='10.2.0.1',clusterIPs=['10.2.0.1'])
                done[ident]={'kind':kind,'name':name,'namespace':'voice-staging','uid':row['metadata']['uid'],
                             'resourceVersion':'1','semantic_sha256':resource_claim.digest(p3.private_semantic(row))}
                if kind=='Deployment': done[ident]['generation']=1
            deployment=live['Deployment/voice-p3-principal-jwks']; deployment['metadata']['annotations']['deployment.kubernetes.io/revision']='1'
            deployment['status']={'observedGeneration':1,'readyReplicas':1,'availableReplicas':1}
            template=deployment['spec']['template']; rs_template=copy.deepcopy(template)
            rs_template['metadata']['labels']['pod-template-hash']='abc12'
            rs={'metadata':{'namespace':'voice-staging','uid':'rsuid','annotations':{'deployment.kubernetes.io/revision':'1'},
                            'ownerReferences':[{'kind':'Deployment','uid':deployment['metadata']['uid'],'controller':True}]},
                'spec':{'replicas':1,'selector':{'matchLabels':{'app':'voice-p3-principal-jwks','pod-template-hash':'abc12'}},'template':rs_template}}
            podspec=copy.deepcopy(template['spec']); podspec['nodeName']='node'; podspec['serviceAccountName']='default'
            podspec['volumes'].append({'name':'kube-api-access-abc12','projected':{'defaultMode':420,'sources':[
                {'serviceAccountToken':{'expirationSeconds':3607,'path':'token'}},
                {'configMap':{'name':'kube-root-ca.crt','items':[{'key':'ca.crt','path':'ca.crt'}]}},
                {'downwardAPI':{'items':[{'path':'namespace','fieldRef':{'apiVersion':'v1','fieldPath':'metadata.namespace'}}]}}]}})
            podspec['containers'][0]['volumeMounts'].append({'name':'kube-api-access-abc12','mountPath':'/var/run/secrets/kubernetes.io/serviceaccount','readOnly':True})
            pod={'metadata':{'namespace':'voice-staging','uid':'poduid','labels':{'app':'voice-p3-principal-jwks'},
                             'ownerReferences':[{'kind':'ReplicaSet','uid':'rsuid','controller':True}]},'spec':podspec,
                 'status':{'phase':'Running','podIP':'10.2.0.8','conditions':[{'type':'Ready','status':'True'}],
                           'containerStatuses':[{'name':'jwks','ready':True,'state':{'running':{}},'imageID':'docker-pullable://nginx@sha256:'+'d'*64}]}}
            svc_uid=live['Service/voice-p3-principal-jwks']['metadata']['uid']
            endpoint={'metadata':{'uid':'sliceuid','namespace':'voice-staging','labels':{'kubernetes.io/service-name':'voice-p3-principal-jwks'},
                                  'ownerReferences':[{'kind':'Service','uid':svc_uid,'controller':True}]},
                      'ports':[{'name':'https','port':8443,'protocol':'TCP'}],
                      'endpoints':[{'addresses':['10.2.0.8'],'conditions':{'ready':True},'targetRef':{'kind':'Pod','uid':'poduid','namespace':'voice-staging'}}]}
            node={'metadata':{'name':'node','uid':'nodeuid'},'status':{'nodeInfo':{'operatingSystem':'linux','architecture':'amd64'},'conditions':[{'type':'Ready','status':'True'}]}}
            account={'metadata':{'name':'default','namespace':'voice-staging','uid':'00000000-0000-0000-0000-000000000101','resourceVersion':'1'}}
            evidence={'node':{'name':'node','uid':'nodeuid','os':'linux','architecture':'amd64'},'manifest_sha256':'d'*64}
            def get(kind,name):
                if kind=='namespace': return {'kind':'Namespace','metadata':{'name':'voice-staging','uid':'ns'}}
                if kind=='node': return copy.deepcopy(node)
                if kind=='serviceaccount': return copy.deepcopy(account)
                key=next(i for i in live if i.lower()==kind+'/'+name)
                return copy.deepcopy(live[key])
            def list_ready(args,body=None,timeout=30):
                return {'metadata':{'resourceVersion':'11'},'items':copy.deepcopy({'replicasets':[rs],'replicaset':[rs],'pods':[pod],'endpointslices':[endpoint]}[args[1]])}
            reader=SimpleNamespace(get=get,run=list_ready)
            with mock.patch.object(parent,'require_child',return_value=plan),mock.patch.object(parent,'load_payload',side_effect=lambda s,slot,i:rawmap[i]), \
                 mock.patch.object(parent,'load_issuer',return_value=issuer,create=True),mock.patch.object(parent,'load_done',return_value=done,create=True), \
                 mock.patch.object(p3,'_source_and_image',return_value=(publisher,evidence)),mock.patch.object(stage_runtime,'Kube',return_value=reader), \
                 mock.patch.object(p3,'probe_jwks',return_value=True) as tls:
                completed=p3.verify_complete(session)
                self.assertEqual(completed['receipt']['objects'],done)
                self.assertEqual(completed['proof']['pod']['uid'],'poduid'); tls.assert_called_once()
                pod['spec']['containers'][0]['image']='foreign'
                with self.assertRaises(p3.PrerequisiteError): p3.verify_complete(session)
            preserved_live=copy.deepcopy(live); prior='9'*12; provenances={}; preserved={}
            for ident,row in preserved_live.items():
                row['metadata']['annotations']['voice.poryadok.io/prerequisites-operation']=prior
                kind,name=ident.split('/'); semantic=p3.private_semantic(row)
                authority={'operation':prior,'receipt_sha256':'a'*64}
                observation={'kind':kind,'name':name,'namespace':'voice-staging','uid':row['metadata']['uid'],
                             'resourceVersion':row['metadata']['resourceVersion'],'semantic_sha256':resource_claim.digest(semantic)}
                if kind=='Deployment': observation['generation']=row['metadata']['generation']
                preserved[ident]={'authority':authority,'observed':observation}
                provenances[ident]={'authority':authority,'semantic':semantic,'uid':row['metadata']['uid']}
            c2plan={**copy.deepcopy(plan),'effects':[],'preserved':preserved}
            def list_preserved(args,body=None,timeout=30):
                self.assertEqual(args[0],'get'); return {'metadata':{'resourceVersion':'22'},
                    'items':[copy.deepcopy(row) for row in preserved_live.values() if row['kind'].lower()==args[1]]}
            reader2=SimpleNamespace(run=list_preserved,get=lambda kind,name:{'kind':'Namespace','metadata':{'name':'voice-staging','uid':'ns'}})
            with mock.patch.object(parent,'require_child',return_value=c2plan),mock.patch.object(parent,'load_issuer',return_value=issuer,create=True), \
                 mock.patch.object(parent,'load_preserved',side_effect=lambda s,slot,i:copy.deepcopy(provenances[i]),create=True), \
                 mock.patch.object(p3,'_source_and_image',return_value=(publisher,evidence)),mock.patch.object(stage_runtime,'Kube',return_value=reader2):
                self.assertTrue(p3.preflight_payloads(session))
                preserved_live['Secret/voice-realtime-principal-signing']['data']['current.pem']='eA=='
                with self.assertRaises(p3.PrerequisiteError): p3.preflight_payloads(session)
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
