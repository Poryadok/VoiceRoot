import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import root_cli
import staging_cohort_preservation as cohort
import transaction
import guard
import resource_claim
import resource_claim_test


class PreservedRootCLITest(unittest.TestCase):
    def test_authorization_rejects_cohort_hash_without_prerequisite_link(self):
        target={'cohort_preservation_sha256':'a'*64};context=resource_claim_test.context()
        receipt={'schema':'nats-rollout-preservation-v1','operation':'a'*12,'namespace_uid':'11111111-1111-4111-8111-111111111111',
                 'marker':{},'pvc':{},'pv':{},'target':target,'backup':{},'expires_at':'fixture',
                 'resource_fence':{'schema':'voice-staging-resource-fence-v1','context':context,'operation':'a'*12,
                                   'helper_sha256':'b'*64,'target_sha256':resource_claim.digest(target)}}
        with self.assertRaisesRegex(guard.Blocked,'cohort_preservation_target_invalid'):
            guard.validate(receipt,'registry',context['commit'],'full',[],Path('/fixture'))

    def test_transaction_rejects_foreign_receipt_before_checkpoint(self):
        session=SimpleNamespace(operation='a'*12,installation=resource_claim.production_installation())
        with tempfile.TemporaryDirectory(dir='/root') as folder,patch.object(cohort,'load_qualified',return_value={'receipt':'actual'}) as load:
            with self.assertRaisesRegex(transaction.Blocked,'cohort_preserved_prepare_invalid'):
                transaction.prepare_preserved(None,Path(folder),Path('/code'),{'cohort_preservation_sha256':'f'*64,'installation_sha256':session.installation.sha256},{},'a'*12,{},[],{},
                    preservation_ref={'receipt':'foreign'},resource_session=session,
                    prerequisites_capture={},realtime_bootstrap_receipt={})
            load.assert_called_once_with(session)
            self.assertEqual(list(Path(folder).iterdir()),[])

    def test_missing_or_foreign_receipt_is_rejected_before_any_files(self):
        args=dict(resource_session=SimpleNamespace(operation='a'*12),nats_authority=None,actor_authority=None,
                  actor_services=[],renderer_authority=None,gis_source_authority=None,gis_source_root=None,
                  gis_source_capture=None,gis_transition=None,gis_binding=None,realtime_bootstrap_receipt=None,
                  prerequisites_capture=None)
        value={'target':{'cohort_preservation_sha256':'f'*64}}
        with tempfile.TemporaryDirectory(dir='/root') as folder,patch.object(root_cli,'ROOT',Path(folder)):
            with self.assertRaisesRegex(root_cli.Blocked,'cohort_preserved_prepare_invalid'):
                root_cli.prepare_preserved_value(value,Path('/source'),{},'a'*12,preservation_ref=None,**args)
            with patch.object(cohort,'load_qualified',return_value={'receipt':'different'}):
                with self.assertRaisesRegex(root_cli.Blocked,'cohort_preserved_prepare_invalid'):
                    root_cli.prepare_preserved_value(value,Path('/source'),{},'a'*12,preservation_ref={'receipt':'claimed'},**args)
            self.assertEqual(list(Path(folder).iterdir()),[])


if __name__=='__main__':unittest.main()
