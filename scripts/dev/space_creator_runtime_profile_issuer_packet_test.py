import copy
import hashlib
import tempfile
from pathlib import Path
import unittest

import credentials
import reset_credentials as reset
import space_creator_runtime_profile_issuer_packet as packet


class PacketTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp=tempfile.TemporaryDirectory(); cls.root=Path(cls.tmp.name)
        cls.material=reset.ResetMaterial(cls.root); cls.ca=cls.material.ca('fixture')
        for name,spec in packet.FAMILIES.items():
            if spec is None: cls.material.signer(name,active=True)
            else: cls.material.tls(name,cls.ca,*spec)
        cls.media=cls.ca[1].read_bytes(); cls.objects=cls.material.items
        cls.der=hashlib.sha256(packet._openssl(['x509','-in','{0}','-outform','DER'],[cls.media])).hexdigest()
        cls.jwks=reset.signer_jwks(next(x for x in cls.objects if x['metadata']['name']=='voice-gateway-principal-signing'))
    @classmethod
    def tearDownClass(cls): cls.tmp.cleanup()

    def envelope(self,names):
        names=sorted(names)
        return {'schema':'voice-space-creator-profile-packet-v1','operation_id':'media-c1-'+'a'*32,
                'source_sha':'b'*40,'namespace_uid':'namespace','preflight_sha256':'c'*64,
                'issuer_receipt':{'schema':'voice-space-creator-profile-issuer-v1','status':'staged','namespace':'voice-dev',
                    'object_names':names,'media_ca_der_sha256':self.der,
                    'public_jwks':self.jwks if 'voice-gateway-principal-signing' in names else None,
                    'runtime_qualified':False,'cluster_applied':False},
                'objects':{'apiVersion':'v1','kind':'List','items':[copy.deepcopy(x) for x in self.objects if x['metadata']['name'] in names]}}

    def check(self,env,names,media=None):
        return packet.validate(env,'media-c1-'+'a'*32,'b'*40,'namespace','c'*64,sorted(names),self.media if media is None else media)

    def test_all_and_missing_only_subset(self):
        for names in (list(packet.FAMILIES),['voice-space-media-client-tls'],['voice-gateway-principal-signing']):
            self.assertEqual(set(self.check(self.envelope(names),names)),set(names))

    def test_binding_unknown_and_partial_fail(self):
        names=['voice-space-media-client-tls']; env=self.envelope(names)
        for field,value in [('source_sha','0'*40),('namespace_uid','other'),('preflight_sha256','0'*64)]:
            bad=copy.deepcopy(env); bad[field]=value
            with self.assertRaises(packet.PacketError): self.check(bad,names)
        bad=copy.deepcopy(env); del bad['objects']['items'][0]['data']['tls.key']
        with self.assertRaises(packet.PacketError): self.check(bad,names)
        with self.assertRaises(packet.PacketError): self.check(env,['not-an-authorized-name'])

    def test_wrong_ca_or_swapped_identity_fail(self):
        names=['voice-space-media-client-tls']; env=self.envelope(names)
        with self.assertRaises(packet.PacketError): self.check(env,names,b'invalid')
        other=next(x for x in self.objects if x['metadata']['name']=='voice-gateway-media-client-tls')
        bad=copy.deepcopy(env); bad['objects']['items'][0]['data']=copy.deepcopy(other['data'])
        with self.assertRaises(packet.PacketError): self.check(bad,names)

if __name__=='__main__': unittest.main()
