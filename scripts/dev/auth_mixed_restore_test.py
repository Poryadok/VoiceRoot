"""Mixed restore envelope checks; source and transport boundaries are synthetic."""
import copy
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import auth_live_migration_phase as live
import cohort_restore_journal
import auth_flyway_upgrade_qualification_cohort as qualification
from auth_flyway_upgrade_qualification_cohort_test import CohortInputs, digest


class MixedRestore(unittest.TestCase):
    def test_clone_qualification_requires_protected_mixed_binding(self):
        fixture=CohortInputs('test_valid_and_wrong_operation_source_hmac_or_restore_rejected')
        self.addCleanup(lambda: self.assertTrue(fixture.doCleanups()))
        fixture.setUp()
        binding=copy.deepcopy(fixture.binding); binding['full_catalog']={}
        receipt=copy.deepcopy(fixture.restore); receipt['migration_logins']={}
        admission=copy.deepcopy(fixture.admission); admission['restore_sha256']=digest(receipt)
        provenance={'custody_sha256':digest(fixture.custody),'restore_sha256':digest(receipt)}
        phase=object()
        args=(fixture.root,fixture.key,binding,fixture.dbs,fixture.manifest,fixture.metadata,fixture.custody,receipt,admission)
        with patch.object(qualification.finalize,'validate_binding',return_value={'auth_db','roles'}), \
             patch.object(qualification.cohort,'verify'),patch.object(qualification.restore_engine,'_source'), \
             patch.object(live,'verified_sources',return_value=(binding,provenance)) as verifier:
            qualification.validate_cohort(*args,phase=phase)
            verifier.assert_called_once_with(phase,fixture.metadata,26)
            verifier.return_value=(binding,provenance | {'restore_sha256':'0'*64})
            with self.assertRaises(ValueError): qualification.validate_cohort(*args,phase=phase)

    def test_protected_roles_and_exact_receipt_are_required(self):
        operation='media-c1-'+'a'*32; snapshot='capture-'+'b'*32; source='c'*40
        binding={'operation_id':operation,'snapshot_id':snapshot,'source_sha':source,
                 'databases':['auth_db','voice_db'],'full_catalog':{},'restore_inputs':{'postgres_image':'p','migrate_image':'m'}}
        planned={'operation_id':operation,'snapshot_id':snapshot,'challenge':'d'*32,
                 'bundle_bytes':10,'bundle_sha256':'e'*64,'run_id':'5'}
        marker={'manifest_sha256':'f'*64}; artifact={**planned,'run_id':5,'head_sha':source,'created_at':'now'}
        read_intent={'status':'pending','artifact_id':7,'binding':artifact,'manifest_sha256':marker['manifest_sha256'],'invocation_nonce':'nonce'}
        read_receipt={'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot','artifact_id':7,
                      **{k:v for k,v in artifact.items() if k!='created_at'}}
        roles={'voice_db':{'role':'voice'}}
        logins={'voice_db':{'role':'voice','from':14,'to':15}}
        intent={'status':'pending','operation_id':operation,'snapshot_id':snapshot,'invocation_nonce':'nonce',
                'manifest_sha256':marker['manifest_sha256'],'artifact_id':7,'migration_roles_sha256':live._digest(roles)}
        receipt={'operation_id':operation,'snapshot_id':snapshot,'source_sha':source,'manifest_sha256':marker['manifest_sha256'],
                 'databases':{'auth_db':{'mode':'restore-only','from':26,'to':26},'voice_db':{'mode':'migrate','from':14,'to':15}},
                 'scope':'selected-cohort-roles-catalog-and-ledger-only','cleanup':'verified','migration_logins':logins}
        rd={'status':'done','intent':read_intent,'receipt':read_receipt}; rs={'status':'done','intent':intent,'receipt':receipt}
        resource={'status':'removed','operation_id':operation,'invocation_nonce':'nonce'}
        row={'cohort_readback':rd,'cohort_restore':rs,'cohort_restore_resources':{'postgres':resource}}
        ledger=SimpleNamespace(read=lambda:(row,('uid','rv')))
        p=SimpleNamespace(fence=SimpleNamespace(ledger=ledger),invocation='nonce',journal=SimpleNamespace(state={'steps':{
            'cohort-readback':rd,'cohort-restore':rs,'cohort-restore-resource-postgres':copy.deepcopy(resource)}}))
        metadata_phase=SimpleNamespace(fence=SimpleNamespace(ledger=ledger))
        phase=SimpleNamespace(phases=p,catalog_phase=object(),_auth_metadata_binding=(metadata_phase,'token'))
        metadata={'migrations':{'auth_db':{'from':26,'to':26},'voice_db':{'from':14,'to':15}}}
        proof={'migration_roles_sha256':live._digest(roles),'source_sha':source,'snapshot_id':snapshot,
               'manifest_sha256':marker['manifest_sha256'],**logins['voice_db']}
        with patch.object(live.readback,'_snapshot',return_value=(binding,planned,marker,None)), \
             patch.object(live.restore,'classify_plan',side_effect=lambda db,*_: 'migrate' if db=='voice_db' else 'restore-only'), \
             patch.object(live.restore,'_qualified_roles',return_value=roles),patch.object(live.restore,'_source'), \
             patch.object(cohort_restore_journal,'verify_go_role',return_value=proof) as verify:
            live.verified_sources(phase,metadata,26)
            for field,value in [('migration_roles_sha256','0'*64)]:
                saved=intent[field]; intent[field]=value
                with self.assertRaises(ValueError): live.verified_sources(phase,metadata,26)
                intent[field]=saved
            original=copy.deepcopy(receipt['migration_logins']); receipt['migration_logins']['voice_db']['role']='other'
            with self.assertRaises(ValueError): live.verified_sources(phase,metadata,26)
            receipt['migration_logins']=original
            def drift(*_):
                p.journal.state['steps'].pop('cohort-readback')
                return proof
            verify.side_effect=drift
            with self.assertRaises(ValueError): live.verified_sources(phase,metadata,26)
            p.journal.state['steps']['cohort-readback']=rd
            verify.side_effect=None
            def revision_drift(*_):
                ledger.read=lambda:(row,('uid','changed'))
                return proof
            verify.side_effect=revision_drift
            with self.assertRaises(ValueError): live.verified_sources(phase,metadata,26)
            ledger.read=lambda:(row,('uid','rv'))
            verify.side_effect=None
            calls=[0]
            def resource_drift():
                calls[0]+=1
                if calls[0]==4:
                    p.journal.state['steps']['cohort-restore-resource-postgres']['container_id']='changed'
                return row,('uid','rv')
            ledger.read=resource_drift
            with self.assertRaises(ValueError): live.verified_sources(phase,metadata,26)
            ledger.read=lambda:(row,('uid','rv'))
            p.journal.state['steps']['cohort-restore-resource-postgres']=copy.deepcopy(resource)
            del phase._auth_metadata_binding
            with self.assertRaises(AttributeError): live.verified_sources(phase,metadata,26)


if __name__ == '__main__': unittest.main()
