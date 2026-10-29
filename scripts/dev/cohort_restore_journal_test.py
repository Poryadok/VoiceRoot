import copy
from unittest.mock import patch
import unittest

import cohort_readback_test as fixtures
import cohort_restore_journal as restore
import cohort_upload_phase as upload


class RestorePhaseTest(unittest.TestCase):
    def setUp(self):
        self.base = fixtures.ReadbackTest('test_real_encrypted_readback_and_no_replay')
        self.addCleanup(fixtures.cleanup,self.base); self.base.setUp()
        _, self.manifest = self.base.extract()

    def invoke(self, qualifier):
        with patch.object(upload,'ROOT',self.base.root), patch.object(restore.restore,'qualify',side_effect=qualifier):
            return restore.run(self.base.phase,self.base.key,self.base.destination,self.manifest,{},'/source','pg','tool',lambda:None)

    def qualifier(self, root, key, binding, databases, manifest, metadata, source_root, postgres, migrate, journal):
        name = 'voice-media-restore-' + 'a'*32; labels = {'voice.test.owner':name}; cid = 'b'*64
        nonce = journal.begin('postgres',name,labels)
        journal.created('postgres',nonce,cid,labels)
        journal.begin_remove('postgres',cid); journal.removed('postgres',cid,True)
        return {'operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],'source_sha':binding['source_sha'],
                'manifest_sha256':self.base.base.ledger.row['cohort_bundle']['manifest_sha256'],
                'databases':{db:{'mode':'restore-only'} for db in databases},'cleanup':'verified'}

    def test_durable_owned_resources_and_no_replay(self):
        result = self.invoke(self.qualifier)
        self.assertEqual(result['cleanup'],'verified')
        record = self.base.base.ledger.row['cohort_restore_resources']['postgres']
        self.assertEqual(record['status'],'removed'); self.assertEqual(record['invocation_nonce'],self.base.media.invocation)
        self.assertEqual(self.base.base.ledger.row['cohort_restore']['status'],'done')
        with self.assertRaises(restore.RestoreError): self.invoke(self.qualifier)

    def test_unknown_create_is_uncertain_and_no_replay(self):
        def failure(root,key,binding,databases,manifest,metadata,source_root,pg,migrate,journal):
            name = 'voice-media-restore-'+'a'*32; nonce = journal.begin('postgres',name,{'voice.test.owner':name})
            journal.uncertain('postgres',nonce)
        with self.assertRaises(restore.RestoreError): self.invoke(failure)
        self.assertEqual(self.base.base.ledger.row['status'],'uncertain')
        self.assertEqual(self.base.base.ledger.row['cohort_restore_resources']['postgres']['status'],'creating')
        with self.assertRaises(restore.RestoreError): self.invoke(self.qualifier)

    def test_foreign_readback_and_manifest_rejected_before_restore(self):
        for fault in ('receipt','manifest'):
            with self.subTest(fault=fault):
                self.setUp()
                if fault == 'receipt': self.base.base.ledger.row['cohort_readback']['receipt']['run_id'] = 124
                else: self.manifest = copy.deepcopy(self.manifest); self.manifest['mac'] = '0'*64
                called = []
                with self.assertRaises(restore.RestoreError): self.invoke(lambda *args: called.append(True))
                self.assertFalse(called)

    def test_terminal_resource_drift_is_uncertain(self):
        for fault in ('nonce','container_id','foreign'):
            with self.subTest(fault=fault):
                self.setUp()
                def changed(*args):
                    result = self.qualifier(*args)
                    records = self.base.base.ledger.row['cohort_restore_resources']
                    if fault == 'foreign': records['foreign'] = {'status':'removed'}
                    else: records['postgres'][fault] = 'f'*64
                    return result
                with self.assertRaises(restore.RestoreError): self.invoke(changed)
                self.assertEqual(self.base.base.ledger.row['status'],'uncertain')
                self.assertNotEqual(self.base.base.ledger.row['cohort_restore']['status'],'done')

    def test_resource_drift_during_final_cas_is_uncertain(self):
        ledger = self.base.base.ledger; write = ledger.write
        def drift(revision,row):
            write(revision,row)
            if row.get('cohort_restore',{}).get('status') == 'done':
                ledger.row['cohort_restore_resources']['postgres']['container_id'] = 'f'*64
        with patch.object(ledger,'write',side_effect=drift), self.assertRaises(restore.RestoreError):
            self.invoke(self.qualifier)
        self.assertEqual(ledger.row['status'],'uncertain')

    def test_previous_resource_drift_blocks_next_resource(self):
        def changed(*args):
            journal = args[-1]; self.qualifier(*args)
            self.base.base.ledger.row['cohort_restore_resources']['postgres']['nonce'] = 'f'*32
            name = 'voice-media-restore-'+'b'*32
            journal.begin('migrate',name,{'voice.test.owner':name})
        with self.assertRaises(restore.RestoreError): self.invoke(changed)
        self.assertEqual(self.base.base.ledger.row['status'],'uncertain')
        self.assertNotIn('migrate',self.base.base.ledger.row['cohort_restore_resources'])


if __name__ == '__main__': unittest.main()
