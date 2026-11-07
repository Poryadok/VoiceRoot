import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import reset_credentials as reset
import credentials
import space_p3_issuer as ca_custody
import space_creator_runtime_profile_issuer as issuer
import space_creator_runtime_profile_issuer_packet as packet


@unittest.skipUnless(os.geteuid()==0,'isolated root-owned CA fixture required')
class PrepareTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup); self.root=Path(self.tmp.name)
        self.archive=self.root/'ca'; self.archive.mkdir(mode=0o700)
        material=reset.ResetMaterial(self.root); key,cert=material.ca('fixture')
        self.media=cert.read_bytes(); (self.archive/'media-ca.crt').write_bytes(self.media)
        (self.archive/'media-ca.key').write_bytes(key.read_bytes()); os.chmod(self.archive/'media-ca.key',0o600)
        self.patch=patch.object(ca_custody,'ARCHIVE',self.archive); self.patch.start(); self.addCleanup(self.patch.stop)
        self.check=patch.object(ca_custody,'_check_ancestors'); self.check.start(); self.addCleanup(self.check.stop)
        self.clean_env=patch.dict(os.environ,{'PATH':'/usr/bin:/bin','LANG':'C','LC_ALL':'C'},clear=True)
        self.clean_env.start(); self.addCleanup(self.clean_env.stop)
        self.dest=self.root/'result'; self.dest.mkdir(mode=0o700)
        self.args=('media-c1-'+'a'*32,'b'*40,'namespace','c'*64)

    def test_only_requested_absent_families_are_generated(self):
        names=sorted(['voice-role-media-server-tls','voice-gateway-principal-signing'])
        receipt=issuer._prepare(self.dest,self.media,names,*self.args)
        envelope=json.loads((self.dest/'objects.json').read_bytes())
        self.assertEqual(receipt['object_names'],names)
        self.assertEqual(set(packet.validate(envelope,*self.args,names,self.media)),set(names))
        self.assertNotIn('voice-space-media-client-tls',receipt['object_names'])

    def test_ca_mismatch_holds_before_generation(self):
        other=reset.ResetMaterial(self.root); _,cert=other.ca('other')
        with self.assertRaises(issuer.IssuerError): issuer._prepare(self.dest,cert.read_bytes(),['voice-space-media-client-tls'],*self.args)
        self.assertFalse((self.dest/'objects.json').exists())

    def test_unknown_or_duplicate_subset_holds(self):
        for names in (['unknown'],['voice-role-media-server-tls']*2):
            with self.assertRaises(issuer.IssuerError): issuer._prepare(self.dest,self.media,names,*self.args)

    def test_leaf_cannot_outlive_existing_ca(self):
        cert=self.archive/'media-ca.crt'; key=self.archive/'media-ca.key'
        cert.unlink()
        credentials.run('openssl','req','-x509','-new','-key',key,'-out',cert,'-days','1',
                        '-subj','/CN=fixture CA','-addext','basicConstraints=critical,CA:TRUE')
        with self.assertRaises(issuer.IssuerError):
            issuer._prepare(self.dest,cert.read_bytes(),['voice-space-media-client-tls'],*self.args)
        self.assertFalse((self.dest/'objects.json').exists())

    def test_inherited_openssl_config_is_refused(self):
        with patch.dict(os.environ,{'OPENSSL_CONF':'/tmp/untrusted'}):
            with self.assertRaises(issuer.IssuerError):
                issuer._prepare(self.dest,self.media,['voice-space-media-client-tls'],*self.args)
        self.assertFalse((self.dest/'objects.json').exists())

if __name__=='__main__': unittest.main()
