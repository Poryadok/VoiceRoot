"""Real disposable RSA/TLS verification; no Kubernetes or service calls."""
import base64
import copy
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

import reset_credentials as credentials
import space_creator_runtime_profile_material as material


class CryptoTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp=tempfile.TemporaryDirectory(); cls.root=Path(cls.tmp.name)
        def run(*args): subprocess.run(['openssl',*map(str,args)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=30)
        root=cls.root; run('req','-x509','-newkey','rsa:2048','-nodes','-days','3','-subj','/CN=Fixture CA',
                          '-addext','basicConstraints=critical,CA:TRUE','-keyout',root/'ca.key','-out',root/'ca.crt')
        cls.objects={}
        def secret(name,values):
            cls.objects[name]={'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-dev','uid':name,'resourceVersion':'1'},
                               'data':{k:base64.b64encode(v).decode() for k,v in values.items()}}
        ca=(root/'ca.crt').read_bytes(); secret('voice-media-ca',{'ca.crt':ca,'jwks-ca.crt':ca})
        for name,host,purpose in (('voice-space-media-server-tls','voice-space','serverAuth'),
                                  ('voice-space-media-client-tls','voice-space','clientAuth'),
                                  ('voice-gateway-media-client-tls','voice-gateway','clientAuth'),
                                  ('voice-auth-media-server-tls','voice-auth','serverAuth'),
                                  ('voice-role-media-server-tls','voice-role','serverAuth')):
            key=root/(name+'.key'); csr=root/(name+'.csr'); cert=root/(name+'.crt'); ext=root/(name+'.ext')
            ext.write_text('basicConstraints=critical,CA:FALSE\nsubjectAltName=DNS:'+host+'\nextendedKeyUsage='+purpose+'\n',encoding='ascii')
            run('req','-new','-newkey','rsa:2048','-nodes','-subj','/CN='+host,'-keyout',key,'-out',csr)
            run('x509','-req','-in',csr,'-CA',root/'ca.crt','-CAkey',root/'ca.key','-CAcreateserial','-days','3','-extfile',ext,'-out',cert)
            secret(name,{'tls.crt':cert.read_bytes(),'tls.key':key.read_bytes()})
        for name in ('voice-space-principal-signing','voice-gateway-principal-signing'):
            values={'active-kid':b'current'}
            for kid in ('current','next'):
                key=root/(name+'-'+kid+'.pem'); run('genpkey','-algorithm','RSA','-pkeyopt','rsa_keygen_bits:2048','-out',key)
                values[kid+'.pem']=key.read_bytes()
            secret(name,values)
        cls.jwks=json.dumps(credentials.signer_jwks(cls.objects['voice-space-principal-signing'])).encode()

    @classmethod
    def tearDownClass(cls): cls.tmp.cleanup()

    def test_real_chains_and_signer_match(self):
        receipt=material.validate(self.objects,self.jwks)
        self.assertEqual(receipt['scope'],'public-crypto-only-no-runtime-readiness')
        self.assertNotIn('PRIVATE KEY',str(receipt))

    def test_wrong_leaf_and_wrong_public_issuer_hold(self):
        obj=copy.deepcopy(self.objects)
        obj['voice-gateway-media-client-tls']['data']=copy.deepcopy(obj['voice-auth-media-server-tls']['data'])
        with self.assertRaises(material.MaterialError): material.validate(obj,self.jwks)
        with self.assertRaises(material.MaterialError): material.validate(self.objects,b'{"keys":[]}')

    def test_certificate_payload_cannot_include_private_bytes(self):
        obj=copy.deepcopy(self.objects); value=obj['voice-auth-media-server-tls']['data']
        value['tls.crt']=base64.b64encode(base64.b64decode(value['tls.crt'])+b'private-sentinel').decode()
        with self.assertRaises(material.MaterialError): material.validate(obj,self.jwks)

    def test_certificate_der_and_bundle_cannot_hide_extra_bytes(self):
        obj=copy.deepcopy(self.objects); value=obj['voice-auth-media-server-tls']['data']
        pem=base64.b64decode(value['tls.crt']); der=material._pem(pem,b'CERTIFICATE')+b'private-sentinel'
        encoded=base64.encodebytes(der)
        value['tls.crt']=base64.b64encode(b'-----BEGIN CERTIFICATE-----\n'+encoded+b'-----END CERTIFICATE-----\n').decode()
        with self.assertRaises(material.MaterialError): material.validate(obj,self.jwks)
        obj=copy.deepcopy(self.objects); value=obj['voice-media-ca']['data']
        value['jwks-ca.crt']=base64.b64encode(base64.b64decode(value['jwks-ca.crt'])+b'private-sentinel').decode()
        with self.assertRaises(material.MaterialError): material.validate(obj,self.jwks)

    def test_distinct_gateway_and_space_signers_required(self):
        obj=copy.deepcopy(self.objects)
        obj['voice-gateway-principal-signing']['data']=copy.deepcopy(obj['voice-space-principal-signing']['data'])
        with self.assertRaises(material.MaterialError): material.validate(obj,self.jwks)


if __name__=='__main__': unittest.main()
