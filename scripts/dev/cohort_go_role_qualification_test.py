"""Authority-boundary unit checks; PG privileges are tested separately on PG16."""
import copy
import hashlib
import json
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import full_cohort_backup_metadata as source
import story_media_backup_cohort_restore as restore
import cohort_restore_journal as journal
import cohort_backup_phases as phases


class RoleReceiptTest(unittest.TestCase):
    def setUp(self):
        self.row=({'operation':'fixture'},('uid','rv'))
        fence=SimpleNamespace(context={'commit':'a'*40},ledger=SimpleNamespace(read=lambda:self.row))
        self.catalog=SimpleNamespace(fence=fence,lock={'lease':'fixture'},namespace_uid='fixture')
        self.phase=object.__new__(source.BackupMetadataPhase)
        self.phase.fence=fence;self.phase.catalog=self.catalog;self.phase.lock=self.catalog.lock;self.phase.namespace_uid='fixture'
        self.metadata={'roles':{'roles':[{'name':'service_login','login':True}]}}
        self.binding={'operation_id':'fixture','source_sha':'a'*40,'sources':{'db':'oid'},
                      'full_catalog':{'receipt_sha256':'b'*64},'restore_inputs':{'migrate':'pinned'}}
        self.result={'operation_id':'fixture','source_sha':'a'*40,'databases':['voice_db'],
                     'sources':self.binding['sources'],'full_catalog':self.binding['full_catalog'],
                     'restore_inputs':self.binding['restore_inputs'],'metadata':self.metadata,
                     'migration_roles':{'voice_db':{'role':'service_login'}}}

    def verify(self, result):
        with patch.object(source,'verify_completed',return_value=result):
            return restore._qualified_roles(self.phase,'token',self.catalog,self.binding,['voice_db'],self.metadata,['voice_db'])

    def test_exact_receipt_and_source_login_required(self):
        self.assertEqual(self.verify(self.result),self.result['migration_roles'])
        for field,value in [('source_sha','c'*40),('databases',[]),('sources',{}),('full_catalog',{}),
                            ('restore_inputs',{}),('metadata',{}),('migration_roles',{})]:
            altered=copy.deepcopy(self.result);altered[field]=value
            with self.subTest(field=field),self.assertRaises(restore.CohortRestoreError):self.verify(altered)
        self.metadata['roles']['roles'][0]['login']=False
        with self.assertRaises(restore.CohortRestoreError):self.verify(self.result)

    def test_actual_phase_and_same_catalog_are_required(self):
        with patch.object(source,'verify_completed',return_value=self.result):
            with self.assertRaises(restore.CohortRestoreError):
                restore._qualified_roles(object(),'token',self.catalog,self.binding,['voice_db'],self.metadata,['voice_db'])
            self.phase.catalog=object()
            with self.assertRaises(restore.CohortRestoreError):
                restore._qualified_roles(self.phase,'token',self.catalog,self.binding,['voice_db'],self.metadata,['voice_db'])

    def test_completed_clone_receipt_requires_matching_protected_history(self):
        self.binding.update(snapshot_id='snap',databases=['voice_db'])
        self.metadata['migrations']={'voice_db':{'from':14,'to':15}}
        roles=self.result['migration_roles']
        digest=hashlib.sha256(json.dumps(roles,sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()
        marker={'manifest_sha256':'c'*64}
        def terminal(ch):
            name='voice-media-restore-'+ch*32
            return {'status':'removed','nonce':ch*32,'name':name,'labels':{'voice.test.owner':name},
                    'operation_id':'fixture','invocation_nonce':'invocation','container_id':ch*64}
        resource={'postgres':terminal('a'),'migrate':terminal('b')}
        intent={'status':'pending','operation_id':'fixture','snapshot_id':'snap','invocation_nonce':'invocation',
                'manifest_sha256':marker['manifest_sha256'],'artifact_id':1,'migration_roles_sha256':digest}
        receipt={'operation_id':'fixture','snapshot_id':'snap','source_sha':'a'*40,
                 'manifest_sha256':marker['manifest_sha256'],'cleanup':'verified',
                 'databases':{'voice_db':{'mode':'migrate','from':14,'to':15}},
                 'migration_logins':{'voice_db':{'role':'service_login','from':14,'to':15}}}
        done={'status':'done','intent':intent,'receipt':receipt}
        readback={'status':'done','intent':{'manifest_sha256':marker['manifest_sha256']},'receipt':{'artifact_id':1}}
        row={'cohort_restore':done,'cohort_restore_resources':resource,'cohort_readback':readback};revision=('uid','rv')
        self.phase.fence.ledger.read=lambda:(row,revision)
        p=SimpleNamespace(fence=self.phase.fence,lock=self.phase.lock,namespace_uid='fixture',invocation='invocation',
                          journal=SimpleNamespace(state={'steps':{'cohort-restore':copy.deepcopy(done),'cohort-readback':readback,
                              'cohort-restore-resource-postgres':resource['postgres'],
                              'cohort-restore-resource-migrate':resource['migrate']}}))
        cp=object.__new__(phases.CohortPhases);cp.phases=p;cp.catalog_phase=self.catalog
        with patch.object(phases.CohortPhases,'_guard') as guard,patch.object(journal.readback,'_snapshot',return_value=(self.binding,None,marker,None)), \
             patch.object(source,'verify_completed',return_value=self.result), \
             patch.object(restore,'classify_plan',return_value='migrate'):
            value=journal.verify_go_role(cp,self.phase,'token','voice_db')
            self.assertEqual((value['role'],value['from'],value['to']),('service_login',14,15))
            for field in ('cohort-restore','cohort-readback','cohort-restore-resource-postgres'):
                original=copy.deepcopy(p.journal.state['steps'])
                calls=[]
                def mutate():
                    calls.append(1)
                    if len(calls)==2: p.journal.state['steps'].pop(field)
                guard.side_effect=mutate
                with self.subTest(field=field),self.assertRaises(journal.RestoreError):
                    journal.verify_go_role(cp,self.phase,'token','voice_db')
                p.journal.state['steps']=original
            guard.side_effect=None
            for altered in ({'status':'removed'}, terminal('c') | {'invocation_nonce':'foreign'},
                            {k:v for k,v in terminal('c').items() if k!='container_id'}):
                original=copy.deepcopy(resource['postgres'])
                row['cohort_restore_resources']['postgres']=altered
                p.journal.state['steps']['cohort-restore-resource-postgres']=copy.deepcopy(altered)
                with self.subTest(resource=altered),self.assertRaises(journal.RestoreError):
                    journal.verify_go_role(cp,self.phase,'token','voice_db')
                row['cohort_restore_resources']['postgres']=original
                p.journal.state['steps']['cohort-restore-resource-postgres']=copy.deepcopy(original)
            row['cohort_restore']['receipt']['migration_logins']['voice_db']['role']='forged'
            with self.assertRaises(journal.RestoreError):journal.verify_go_role(cp,self.phase,'token','voice_db')


if __name__ == '__main__': unittest.main()
