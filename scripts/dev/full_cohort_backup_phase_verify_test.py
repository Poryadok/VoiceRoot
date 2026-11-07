"""Completed receipt boundary with synthetic external authority."""
import copy
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import cohort_backup_phases
import full_cohort_backup_metadata as metadata
import full_cohort_backup_phase as backup
import cohort_readback
import cohort_upload_runner as runner


class CompletedTest(unittest.TestCase):
    def test_completed_restore_and_final_guard_drift(self):
        phase=object.__new__(backup.BackupPhase);cohort=object.__new__(cohort_backup_phases.CohortPhases)
        binding={'operation_id':'op','snapshot_id':'snap','source_sha':'source','databases':['voice_db'],
                 'sources':{},'restore_inputs':{},'full_catalog':{}}
        cohort.binding=copy.deepcopy(binding);phase.cohort=cohort;phase.metadata=object();phase._auth_plan=None
        phase.supervisor=SimpleNamespace(namespace_uid='ns',catalog=object());cohort.catalog_phase=phase.supervisor.catalog
        plan={'engine':'golang-migrate','from':14,'to':14,'files':{'000014_test.up.sql':'x'}}
        source={'operation_id':'op','source_sha':'source','databases':['voice_db'],'sources':{},'restore_inputs':{},
                'full_catalog':{},'migration_roles':{},'metadata':{'migrations':{'voice_db':plan}}}
        marker={'manifest_sha256':'f'*64};planned={'operation_id':'op','snapshot_id':'snap','challenge':'c'*32,
             'bundle_bytes':1,'bundle_sha256':'b'*64,'run_id':'123'}
        artifact=planned|{'run_id':123,'head_sha':'source','created_at':'time'}
        ri={'status':'pending','artifact_id':1,'binding':artifact,'manifest_sha256':'f'*64,'invocation_nonce':'inv'}
        custody={'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot','artifact_id':1,
                 **{k:v for k,v in artifact.items() if k!='created_at'}}
        readback={'status':'done','intent':ri,'receipt':custody}
        intent={'status':'pending','operation_id':'op','snapshot_id':'snap','invocation_nonce':'inv','manifest_sha256':'f'*64,'artifact_id':1}
        receipt={'operation_id':'op','snapshot_id':'snap','source_sha':'source','manifest_sha256':'f'*64,'cleanup':'verified',
                 'scope':'selected-cohort-roles-catalog-and-ledger-only','databases':{'voice_db':{'mode':'restore-only','from':14,'to':14}}}
        done={'status':'done','intent':intent,'receipt':receipt}
        name='voice-media-restore-'+'a'*32
        resources={'postgres':{'status':'removed','nonce':'a'*32,'name':name,'labels':{'voice.test.owner':name},
                                'operation_id':'op','invocation_nonce':'inv','container_id':'a'*64}}
        row={'full_cohort_backup_route':runner.route_marker(cohort.binding,'ns'),'cohort_readback':readback,
             'cohort_restore':done,'cohort_restore_resources':resources};revision=('uid','rv')
        steps={'cohort-readback':copy.deepcopy(readback),'cohort-restore':copy.deepcopy(done),
               'cohort-restore-resource-postgres':copy.deepcopy(resources['postgres'])}
        journal=SimpleNamespace(state={'steps':steps});phase.journal=journal
        cohort.phases=SimpleNamespace(journal=journal,invocation='inv',intent={},fence=SimpleNamespace(ledger=SimpleNamespace(read=lambda:(copy.deepcopy(row),revision))))
        with patch.object(backup.BackupPhase,'_guard') as guard,patch.object(cohort_backup_phases.CohortPhases,'_guard'), \
             patch.object(metadata,'verify_completed',return_value=source), \
             patch.object(cohort_readback,'_snapshot',return_value=(binding,planned,marker,{})):
            self.assertEqual(backup.verify_completed(phase,'token')['snapshot_id'],'snap')
            calls=[]
            def drift():
                calls.append(1)
                if len(calls)==2:steps.pop('cohort-restore-resource-postgres')
            guard.side_effect=drift
            with self.assertRaises(backup.BackupError):backup.verify_completed(phase,'token')


if __name__=='__main__':unittest.main()
