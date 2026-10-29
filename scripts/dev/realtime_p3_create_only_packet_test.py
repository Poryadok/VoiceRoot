import base64
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import realtime_p3_create_only_crypto as crypto
import realtime_p3_create_only_packet as packet
import reset_credentials as canonical


class PacketTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(); cls.addClassCleanup(cls.temp.cleanup); root = Path(cls.temp.name)
        def run(*args):
            subprocess.run(['openssl',*map(str,args)],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True,timeout=30)
        def ca(name):
            key, cert = root/(name+'.key'),root/(name+'.crt')
            run('req','-x509','-newkey','rsa:2048','-nodes','-keyout',key,'-out',cert,'-days','3',
                '-subj','/CN='+name,'-addext','basicConstraints=critical,CA:TRUE','-addext','keyUsage=critical,keyCertSign,cRLSign')
            return key,cert
        principal_key,principal = ca('principal'); media_key,media = ca('media')
        def leaf(name,host,purposes,key,cert):
            private,csr,out,ext = [root/(name+x) for x in ('.key','.csr','.crt','.ext')]
            run('req','-new','-newkey','rsa:2048','-nodes','-keyout',private,'-out',csr,'-subj','/CN='+host)
            ext.write_text('basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\n'
                           'extendedKeyUsage='+purposes+'\nsubjectAltName=DNS:'+host+'\n')
            run('x509','-req','-in',csr,'-CA',cert,'-CAkey',key,'-set_serial','0x'+hashlib.sha256(name.encode()).hexdigest()[:16],
                '-days','2','-extfile',ext,'-out',out)
            return {'tls.crt':out.read_bytes(),'tls.key':private.read_bytes()}
        signing = {}
        for kid in ('current','next'):
            path = root/(kid+'.pem'); run('genpkey','-algorithm','RSA','-pkeyopt','rsa_keygen_bits:2048','-out',path)
            signing[kid+'.pem'] = path.read_bytes()
        cls.principal,cls.media = principal.read_bytes(),media.read_bytes()
        space = leaf('space','voice-space.voice-dev.svc','serverAuth',media_key,media); cls.space = space['tls.crt']
        values = {
            'voice-realtime-principal-signing':signing,
            'voice-realtime-p3-tls':leaf('realtime','voice-realtime.voice-dev.svc','serverAuth,clientAuth',principal_key,principal),
            'voice-chat-realtime-p3-tls':leaf('chat','voice-chat.voice-dev.svc','serverAuth',principal_key,principal),
            'voice-p3-principal-jwks-tls':leaf('jwks','voice-p3-principal-jwks.voice-dev.svc','serverAuth',principal_key,principal),
            'voice-p3-principal-replay':{'password':b''},
            'voice-p3-principal-trust':{'chat-server-ca.crt':cls.principal,'space-client-ca.crt':cls.principal,
                'space-jwks-ca.crt':cls.media,'realtime-client-ca.crt':cls.principal,'realtime-jwks-ca.crt':cls.principal}}
        def secret(name,data):
            return {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-dev'},'type':'Opaque',
                    'data':{k:base64.b64encode(v).decode() for k,v in data.items()}}
        signer = secret('voice-realtime-principal-signing',signing)
        if crypto.signer_jwks(signing) != canonical.signer_jwks(signer): raise AssertionError('canonical JWKS differs')
        items = [secret(name,data) for name,data in values.items()]
        items.append({'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'voice-p3-principal-public','namespace':'voice-dev'},
                      'immutable':True,'data':{'realtime.json':json.dumps(crypto.signer_jwks(signing))}})
        cls.operation = 'media-c1-'+'a'*32; cls.source = 'b'*40
        cls.envelope = {'operation_id':cls.operation,'source_sha':cls.source,'objects':{'apiVersion':'v1','kind':'List','items':items},
            'issuer_receipt':{'status':'staged','namespace':'voice-dev','object_names':sorted(name for _,name in packet.MEMBERS),
                'principal_ca_der_sha256':hashlib.sha256(crypto.der(cls.principal)).hexdigest(),
                'media_ca_der_sha256':hashlib.sha256(crypto.der(cls.media)).hexdigest(),
                'space_jwks_cert_der_sha256':hashlib.sha256(crypto.der(cls.space)).hexdigest(),
                'runtime_qualified':False,'cluster_applied':False}}

    def validate(self,value):
        return packet.validate(value,self.operation,self.source,self.principal,self.media,self.space)

    def test_real_crypto_exact_packet(self):
        self.assertEqual(set(self.validate(self.envelope)),set(packet.MEMBERS))

    def test_wrong_trust_public_key_scope_and_operation_are_refused(self):
        for fault in ('trust','jwks','extra','operation','replay'):
            with self.subTest(fault=fault):
                value = copy.deepcopy(self.envelope); items = value['objects']['items']
                if fault == 'trust': value['issuer_receipt']['principal_ca_der_sha256'] = 'f'*64
                if fault == 'jwks': items[-1]['data']['realtime.json'] = '{"keys":[]}'
                if fault == 'extra': items.append(copy.deepcopy(items[0]))
                if fault == 'operation': value['operation_id'] = 'media-c2-'+'a'*32
                if fault == 'replay': next(x for x in items if x['metadata']['name']=='voice-p3-principal-replay')['data']['password'] = 'c2VjcmV0'
                with self.assertRaises(packet.PacketError) as error: self.validate(value)
                self.assertNotIn('secret',str(error.exception))

    def test_input_mutation_during_crypto_cannot_rebind_return(self):
        value = copy.deepcopy(self.envelope); original = crypto.space_certificate
        def mutate(cert,ca):
            value['objects']['items'][0]['metadata']['name'] = 'foreign'
            return original(cert,ca)
        with patch.object(crypto,'space_certificate',side_effect=mutate):
            result = self.validate(value)
        self.assertEqual(set(result),set(packet.MEMBERS))
        self.assertEqual(result['Secret','voice-realtime-principal-signing']['metadata']['name'],
                         'voice-realtime-principal-signing')


if __name__ == '__main__': unittest.main()
