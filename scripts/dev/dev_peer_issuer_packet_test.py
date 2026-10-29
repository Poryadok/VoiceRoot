import copy
import hashlib
from pathlib import Path
import tempfile
import unittest

import dev_peer_issuer_metadata as metadata
import dev_peer_issuer_packet as packet
import reset_credentials as reset

class PacketTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp=tempfile.TemporaryDirectory(); cls.root=Path(cls.tmp.name)
        material=reset.ResetMaterial(cls.root); key,cert=material.ca('fixture'); cls.media=cert.read_bytes()
        for name in metadata.MEMBERS:
            if name.endswith('-principal-signing'): material.signer(name,active=True)
            else: material.tls(name,(key,cert),*packet.TLS[name])
        cls.objects={obj['metadata']['name']:obj for obj in material.items}
        cls.der=hashlib.sha256(packet.crypto._openssl(['x509','-in','{0}','-outform','DER'],[cls.media])).hexdigest()
    @classmethod
    def tearDownClass(cls): cls.tmp.cleanup()

    def envelope(self,profile,names):
        names=sorted(names); objects=[copy.deepcopy(self.objects[n]) for n in names]
        jwks={name:reset.signer_jwks(self.objects[name]) for name in names if name.endswith('-principal-signing')}
        return {'schema':'voice-dev-peer-issuer-packet-v1','profile':profile,'operation_id':'media-c1-'+'a'*32,
            'source_sha':'b'*40,'namespace_uid':'ns','preflight_sha256':'c'*64,
            'issuer_receipt':{'schema':'voice-dev-peer-issuer-receipt-v1','status':'staged','profile':profile,
                'object_names':names,'media_ca_der_sha256':self.der,'public_jwks':jwks,
                'runtime_qualified':False,'cluster_applied':False},'objects':{'apiVersion':'v1','kind':'List','items':objects}}

    def test_three_exact_profiles_and_subsets(self):
        for profile,names in metadata.PROFILES.items():
            for selected in (names,names[:1]):
                result=packet.validate(self.envelope(profile,selected),profile,'media-c1-'+'a'*32,'b'*40,'ns','c'*64,sorted(selected),self.media)
                self.assertEqual(set(result),set(selected))

    def test_foreign_profile_or_wrong_leaf_refused(self):
        names=['voice-role-media-client-tls']; env=self.envelope('role',names)
        with self.assertRaises(packet.PacketError): packet.validate(env,'creator','media-c1-'+'a'*32,'b'*40,'ns','c'*64,names,self.media)
        env['objects']['items'][0]['data']=copy.deepcopy(self.objects['voice-space-media-client-tls']['data'])
        with self.assertRaises(packet.PacketError): packet.validate(env,'role','media-c1-'+'a'*32,'b'*40,'ns','c'*64,names,self.media)

    def test_space_server_requires_short_and_full_names(self):
        name='voice-space-media-server-tls'
        material=reset.ResetMaterial(self.root)
        material.tls(name,(self.root/'fixture-ca.key',self.root/'fixture-ca.crt'),
                     'voice-space.voice-dev.svc','serverAuth')
        env=self.envelope('space',[name])
        env['objects']['items'][0]=material.items[0]
        with self.assertRaises(packet.PacketError):
            packet.validate(env,'space','media-c1-'+'a'*32,'b'*40,'ns','c'*64,[name],self.media)

if __name__=='__main__': unittest.main()
