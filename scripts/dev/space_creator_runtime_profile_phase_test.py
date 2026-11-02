"""Real local Journal/Fence plus real crypto; Kube/source/Redis authority synthetic."""
import copy
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import canonical_application_package as canonical
import frontend_runtime_executor as transport
import full_cohort_package_phase as package
import media_backup_phase_test as fixture
import media_operation_guard as guard
import media_operation_journal as journal_module
import resource_fencing
import space_creator_runtime_profile_material as material
import space_creator_runtime_profile_material_test as crypto_fixture
import space_creator_runtime_profile_phase as phase_module
import writer_cut


class PhaseTest(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        crypto=crypto_fixture.CryptoTest
        self.addCleanup(lambda: crypto.tmp.cleanup() if hasattr(crypto,'tmp') else None)
        crypto.setUpClass(); self.objects=copy.deepcopy(crypto.objects); self.jwks=crypto.jwks
        self.context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
          'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
          'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'a'*40,
          'images':{'voice-space/space':'ghcr.io/x@sha256:'+'b'*64}}
        self.journal=journal_module.Journal(Path(self.temp.name)/'journal',{'resource_context':self.context,'namespace_uid':'ns'})
        self.addCleanup(self.journal.close)
        self.backing=fixture.Ledger(self.context); self.client=transport.KubectlTransport(self.journal)
        self.ledger=resource_fencing.KubeLedger(self.client); self.ledger.read=self.backing.read; self.ledger.write=self.backing.write
        self.fence=resource_fencing.Fence(self.context,lambda:{},self.ledger)
        self.cut=writer_cut.WriterCut.__new__(writer_cut.WriterCut); self.package=package.PackagePhase.__new__(package.PackagePhase)
        self.lock={'metadata':{'uid':'lock'}}
        for obj in (self.cut,self.package):
            obj.fence=self.fence; obj.client=self.client; obj.lock=copy.deepcopy(self.lock); obj.namespace_uid='ns'
        self.cut._guard=mock.Mock(); self.package.verify_current=mock.Mock(return_value={})
        self.phase=phase_module.CreatorProfilePhase(self.journal,self.fence,self.client,self.lock,'ns',self.cut,
                                                    self.package,object(),canonical.ROOT,'token')
        self.phase._source=mock.Mock(); self.phase._redis=mock.Mock(return_value={'scope':'synthetic'})
        self.phase._get_secret=mock.Mock(side_effect=lambda name:copy.deepcopy(self.objects[name]))
        public=material.validate(self.objects,self.jwks)
        self.proof={'schema':'voice-dev-space-p3-issuer-v1','scope':'space-two-secret-create-only',
            'signer':public['objects']['voice-space-principal-signing'],
            'tls':public['objects']['voice-space-media-server-tls']|{'certificate_sha256':public['certificate_sha256']['voice-space-media-server-tls']},
            'media_ca':public['objects']['voice-media-ca']|{'der_sha256':public['media_ca_sha256']}}
        self.patches=[mock.patch.object(guard,'verify'),
                      mock.patch.object(phase_module.authority,'_space',side_effect=lambda *a:(copy.deepcopy(self.proof),self.jwks))]
        for patcher in self.patches: patcher.start(); self.addCleanup(patcher.stop)

    def test_real_crypto_protected_receipt_and_secret_identity_drift(self):
        receipt=self.phase.run()
        self.assertFalse(receipt['runtime_qualified'])
        self.assertNotIn('PRIVATE KEY',json.dumps(receipt))
        self.assertEqual(phase_module.verify_completed(self.phase,'token'),receipt)
        self.objects['voice-auth-media-server-tls']['metadata']['resourceVersion']='2'
        with self.assertRaises(phase_module.ProfileError): phase_module.verify_completed(self.phase,'token')

    def test_missing_material_holds_and_poisoned_fence_never_replays(self):
        del self.objects['voice-role-media-server-tls']
        with self.assertRaises(phase_module.ProfileError): self.phase.run()
        self.assertTrue(self.phase.poisoned)
        with self.assertRaises(phase_module.metadata.GuardLost): self.phase.run()

    def test_transient_cut_guard_loss_is_not_masked(self):
        self.cut._guard.side_effect=RuntimeError('private-sentinel')
        with self.assertRaises(phase_module.metadata.GuardLost): self.phase.run()
        self.assertNotIn(phase_module.LEDGER,self.backing.row)


if __name__=='__main__': unittest.main()
