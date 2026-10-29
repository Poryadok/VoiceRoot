"""C2 receipt with real local Journal/Fence and encrypted synthetic cohort."""
import copy
import json
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import auth_live_migration_phase as live
import cohort_readback_test as fixture
import cohort_restore_journal as restore
import cohort_upload_phase as upload
import media_backup_phase_test as base_fixture
import resource_fencing


class C2JournalTest(unittest.TestCase):
    def test_c1_init_runs_real_cohort_verifier_through_durable_reservation(self):
        original=base_fixture.PhaseTest.setUp
        tool={'node_name':'node-one','node_uid':'node-id','flyway_platform_digest':'sha256:'+'c'*64}
        def setup(base):
            original(base)
            base.intent['auth_live_tool']=copy.deepcopy(tool)
        work=fixture.ReadbackTest('test_real_encrypted_readback_and_no_replay')
        try:
            with patch.object(base_fixture.PhaseTest,'setUp',setup): work.setUp()
            _,manifest=work.extract()
            def qualifier(root,key,binding,databases,manifest,metadata,source_root,postgres,migrate,journal):
                name='voice-media-restore-'+'b'*32; cid='c'*64; labels={'voice.test.owner':name}
                nonce=journal.begin('postgres',name,labels); journal.created('postgres',nonce,cid,labels)
                journal.begin_remove('postgres',cid); journal.removed('postgres',cid,True)
                return {'operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                        'source_sha':binding['source_sha'],'manifest_sha256':work.base.ledger.row['cohort_bundle']['manifest_sha256'],
                        'databases':{db:{'mode':'restore-only','from':26,'to':26} for db in databases},
                        'scope':'selected-cohort-roles-catalog-and-ledger-only','cleanup':'verified'}
            with patch.object(upload,'ROOT',work.root),patch.object(restore.restore,'qualify',side_effect=qualifier):
                restore.run(work.phase,work.key,work.destination,manifest,{},'/source','pg','tool',lambda:None)
            work.journal.state['steps']['cohort-auth-flyway-private']={}
            work.journal.state['steps']['cohort-auth-flyway']={}; work.journal.save()
            work.base.ledger.row['cohort_auth_flyway']={}
            history=[{'version':str(n),'script':f'V{n}__fixture.sql','checksum':n,'type':'SQL','success':True} for n in (1,26)]
            metadata={'migrations':{'auth_db':{'files':{}}},'catalog':{'auth_db':{'history':
                [{k:r[k] for k in ('version','script','checksum','success')} for r in history]}}}
            with patch.object(live.restore,'_source'):
                _,provenance=live.verified_sources(work.phase,metadata,26)
            work.journal.state['steps']['cohort-auth-flyway-private']=copy.deepcopy(provenance); work.journal.save()
            observation={'history':history,'sessions':[],'prepared':0}
            cut=SimpleNamespace(verify_drained=lambda:None)
            with patch.object(live,'_cut_identity'),patch.object(live,'_bind_cut_scope'),patch.object(live,'_package_authority'), \
                 patch.object(live.restore,'_source'),patch.object(live,'source_catalog',return_value=({'V26__x.sql':b'sql'},{})), \
                 patch.object(live,'candidate_blob',return_value=b'{}'),patch.object(live,'expected_history',return_value=history), \
                 patch.object(live.eligibility,'observe_candidate',return_value={}),patch.object(live.probe,'inspect',return_value=observation), \
                 patch.object(live,'validate_clone_receipt'),patch.object(live,'_c1_data_snapshot',return_value={}), \
                 patch.object(live,'flyway_platform',return_value=tool['flyway_platform_digest']), \
                 patch.object(live,'observe_node',return_value={'name':'node-one','uid':'node-id'}), \
                 patch.object(live.probe,'verify_login',return_value={'uid':'secret','resourceVersion':'1'}), \
                 patch.object(live,'verify_writer_successor'):
                work.journal.state['steps']['cohort-auth-flyway-private']['manifest_sha256']='f'*64
                work.journal.save()
                with self.assertRaises(live.PhaseError): live.init_c1(work.phase,cut,'.','actor','token',metadata)
                self.assertNotIn('auth_live_c1',work.base.ledger.row)
                work.journal.state['steps']['cohort-auth-flyway-private']=copy.deepcopy(provenance)
                work.journal.save()
                result=live.init_c1(work.phase,cut,'.','actor','token',metadata)
            self.assertEqual(result['job']['state'],'absent')
            self.assertEqual(work.base.ledger.row['auth_live_c1'],result)
        finally:
            if not work.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def test_owned_noop_and_forged_restore_refused(self):
        original=base_fixture.PhaseTest.setUp
        def setup(base):
            original(base)
            base.context['operation_id']='media-c2-'+'a'*32
            base.context['images']['voice-auth/auth']='ghcr.io/poryadok/voiceroot/auth@sha256:'+'d'*64
            base.intent['auth_live_candidate']={'platform_digest':'sha256:'+'e'*64,'deployment':{
                'uid':'deployment-uid','resourceVersion':'1','generation':3,
                'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'auth','image':'old'}]}}}}}
            base.ledger=base_fixture.Ledger(base.context)
            base.fence=resource_fencing.Fence(base.context,lambda:copy.deepcopy(base.card),base.ledger)
            base.lock['data']['resource_context']=json.dumps(base.context,sort_keys=True,separators=(',',':'))
            base.lock['data']['resource_operation']=base.context['operation_id']
        work=fixture.ReadbackTest('test_real_encrypted_readback_and_no_replay')
        try:
            with patch.object(base_fixture.PhaseTest,'setUp',setup): work.setUp()
            _,manifest=work.extract()
            def qualifier(root,key,binding,databases,manifest,metadata,source_root,postgres,migrate,journal):
                name='voice-media-restore-'+'b'*32; cid='c'*64; labels={'voice.test.owner':name}
                nonce=journal.begin('postgres',name,labels); journal.created('postgres',nonce,cid,labels)
                journal.begin_remove('postgres',cid); journal.removed('postgres',cid,True)
                dbs={db:{'mode':'restore-only','from':32,'to':32} for db in databases}
                return {'operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                        'source_sha':binding['source_sha'],'manifest_sha256':work.base.ledger.row['cohort_bundle']['manifest_sha256'],
                        'databases':dbs,'scope':'selected-cohort-roles-catalog-and-ledger-only','cleanup':'verified'}
            with patch.object(upload,'ROOT',work.root),patch.object(restore.restore,'qualify',side_effect=qualifier):
                restore.run(work.phase,work.key,work.destination,manifest,{},'/source','pg','tool',lambda:None)
            binding=work.phase.binding; history=[{'version':str(n),'script':f'V{n}__fixture.sql',
                'checksum':n,'type':'SQL','success':True} for n in (1,26,28,31,32)]
            metadata={'migrations':{'auth_db':{'files':{}}},'catalog':{'auth_db':{'history':
                [{k:r[k] for k in ('version','script','checksum','success')} for r in history]}}}
            observation={'history':history,'sessions':[],'prepared':0}
            cut=SimpleNamespace(verify_drained=lambda:None,_guard=lambda:None,journal=SimpleNamespace(state={'steps':{}}))
            target=live.eligibility.candidate_target(work.media.intent['auth_live_candidate']['deployment']['spec'],work.base.context['images']['voice-auth/auth'])
            candidate={'source_sha':binding['source_sha'],'image':work.base.context['images']['voice-auth/auth'],
                'auth_tree':'a'*40,'platform_digest':'sha256:'+'e'*64,'target_spec_sha256':live._digest(target),
                'deployment':{'uid':'deployment-uid','generation':3,'resourceVersion':'1'},
                'policy':{'flyway':'disabled','persistence':'jdbc','selector':'absent'},
                'route':{'host':'voice-postgres','database':'auth_db','role':'voice'}}
            with patch.object(live,'_cut_identity'),patch.object(live,'_bind_cut_scope'),patch.object(live,'_package_authority'),patch.object(live.restore,'_source'),patch.object(live,'source_catalog',return_value=({},{})), \
                 patch.object(live,'candidate_blob',return_value=b'{}'),patch.object(live,'expected_history',return_value=history), \
                 patch.object(live.eligibility,'observe_candidate',return_value=candidate), \
                 patch.object(live.eligibility,'_source_tree',return_value='a'*40), \
                 patch.object(live.probe,'inspect',return_value=observation):
                bad=copy.deepcopy(work.base.ledger.row['cohort_restore'])
                work.base.ledger.row['cohort_restore']['receipt']['source_sha']='f'*40
                with self.assertRaises(live.PhaseError): live.no_op_c2(work.phase,cut,'.','actor','token',metadata)
                self.assertNotIn('auth_live_c2',work.base.ledger.row)
                work.base.ledger.row['cohort_restore']=bad
                receipt=live.no_op_c2(work.phase,cut,'.','actor','token',metadata)
                self.assertEqual(receipt['operation_id'],binding['operation_id'])
                self.assertEqual(receipt['metadata'],metadata)
                self.assertEqual(work.base.ledger.row['auth_live_c2']['status'],'done')
                self.assertEqual(work.journal.state['steps']['auth-live-c2-private'],receipt)
                with patch.object(cut,'verify_drained',side_effect=AssertionError('resume reader must not drain')):
                    completed=live.verify_completed(work.phase,cut,'.')
                self.assertEqual(completed['status'],'no-op')
                work.journal.state['steps']['auth-live-c2-private']['candidate']['route']['database']='other_db'
                with self.assertRaises(live.PhaseError): live.verify_completed(work.phase,cut,'.')
                work.journal.state['steps']['auth-live-c2-private']=copy.deepcopy(receipt)
                original_row=copy.deepcopy(work.base.ledger.row)
                for kind in ('private','ledger'):
                    calls=[0]
                    def mutate():
                        calls[0]+=1
                        if calls[0]==2:
                            if kind=='private': work.journal.state['steps']['auth-live-c2-private']['candidate']['route']['database']='other_db'
                            else: work.base.ledger.row['auth_live_c2']['receipt_sha256']='f'*64
                    with patch.object(cut,'_guard',side_effect=mutate),self.assertRaises(live.PhaseError):
                        live.verify_completed(work.phase,cut,'.')
                    work.journal.state['steps']['auth-live-c2-private']=copy.deepcopy(receipt)
                    work.base.ledger.row=copy.deepcopy(original_row)
        finally:
            if not work.doCleanups(): raise RuntimeError('fixture cleanup failed')


if __name__=='__main__': unittest.main()
