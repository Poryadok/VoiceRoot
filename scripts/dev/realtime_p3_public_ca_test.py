import copy
from pathlib import Path
import unittest
from unittest.mock import patch

import realtime_p3_public_ca as ca
import realtime_p3_public_inputs_test as fixture


class PublicCATest(unittest.TestCase):
    def setUp(self):
        self.base=fixture.CaptureTests('test_real_chain_and_private_files'); self.base.setUp()
        self.addCleanup(self.base.tearDown)
        self.destination=self.base.root/'ca-only'; self.destination.mkdir(mode=0o700)
        self.snapshot={'source_sha':'a'*40,'files':{'deploy/staging/services.yaml':self.base.manifest_sha}}

    def invoke(self,runner=None):
        with patch.object(ca,'source_snapshot',return_value=copy.deepcopy(self.snapshot)), \
             patch.object(ca.public,'SPACE_DOCUMENT_SHA',self.base.document_sha):
            return ca.capture(self.base.root,'a'*40,lambda _:None,'ns',lambda:None,self.destination,runner or self.base.runner)

    def test_two_real_public_cas_no_space_certificate(self):
        def runner(args):
            self.assertNotIn('voice-space-media-server-tls',args)
            return self.base.runner(args)
        result=self.invoke(runner)
        self.assertEqual(result['scope'],'public-ca-preparation-only'); self.assertFalse(result['admission_qualified'])
        self.assertEqual(sorted(p.name for p in self.destination.iterdir()),['media-ca.pem','principal-ca.pem'])
        self.assertTrue(all(p.stat().st_mode & 0o777==0o600 for p in self.destination.iterdir()))
        self.assertNotIn('BEGIN CERTIFICATE',str(result))

    def test_drift_holds_without_receipt_and_preserves_partial_files(self):
        count=0
        def runner(args):
            nonlocal count
            if 'voice-media-ca' in args and any('.metadata.uid' in x for x in args):
                count+=1
                if count>1: return b'uid-voice-media-ca\n2'
            return self.base.runner(args)
        with self.assertRaises(ca.common.Hold): self.invoke(runner)
        self.assertEqual(list(self.destination.iterdir()),[])

    def test_private_or_invalid_certificate_never_written(self):
        self.base.values['voice-principal-ca']=b'-----BEGIN PRIVATE KEY-----\nprivate-sentinel\n-----END PRIVATE KEY-----\n'
        with self.assertRaises(ca.common.Hold) as error: self.invoke()
        self.assertNotIn('private-sentinel',str(error.exception)); self.assertEqual(list(self.destination.iterdir()),[])


if __name__=='__main__': unittest.main()
