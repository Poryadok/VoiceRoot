"""C2 receipt with real local Journal/Fence and encrypted synthetic cohort."""
import copy
import json
import unittest
from unittest.mock import patch

import auth_live_migration_phase as live
import cohort_readback_test as fixture
import cohort_restore_journal as restore
import cohort_upload_phase as upload
import media_backup_phase_test as base_fixture
import resource_fencing


class C2JournalTest(unittest.TestCase):
    def test_owned_noop_and_forged_restore_refused(self):
        original=base_fixture.PhaseTest.setUp
        def setup(base):
            original(base)
            base.context['operation_id']='media-c2-'+'a'*32
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
            with patch.object(live,'_phase_identity'),patch.object(live,'_package_authority'),patch.object(live.restore,'_source'),patch.object(live,'source_catalog',return_value=({},{})), \
                 patch.object(live,'candidate_blob',return_value=b'{}'),patch.object(live,'expected_history',return_value=history), \
                 patch.object(live.eligibility,'observe_candidate',return_value={'policy':'checked'}), \
                 patch.object(live.probe,'inspect',return_value=observation):
                bad=copy.deepcopy(work.base.ledger.row['cohort_restore'])
                work.base.ledger.row['cohort_restore']['receipt']['source_sha']='f'*40
                with self.assertRaises(live.PhaseError): live.no_op_c2(work.phase,lambda:None,'.','actor','token',metadata)
                self.assertNotIn('auth_live_c2',work.base.ledger.row)
                work.base.ledger.row['cohort_restore']=bad
                receipt=live.no_op_c2(work.phase,lambda:None,'.','actor','token',metadata)
                self.assertEqual(receipt['operation_id'],binding['operation_id'])
                self.assertEqual(work.base.ledger.row['auth_live_c2']['status'],'done')
                self.assertEqual(work.journal.state['steps']['auth-live-c2-private'],receipt)
        finally:
            if not work.doCleanups(): raise RuntimeError('fixture cleanup failed')


if __name__=='__main__': unittest.main()
