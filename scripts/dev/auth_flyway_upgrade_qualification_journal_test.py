import copy
import hashlib
import json
import unittest
from unittest.mock import patch

import auth_flyway_upgrade_qualification_journal as adapter
import auth_user_adoption_runtime as runtime
import cohort_restore_journal_test as fixtures
import story_media_backup_cohort_restore as restore


def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()


class CloneJournalTest(unittest.TestCase):
    def setUp(self):
        self.fix = fixtures.RestorePhaseTest('test_durable_owned_resources_and_no_replay')
        self.fix.setUp()
        self.addCleanup(self.fix.doCleanups)
        def qualify(*args):
            value = self.fix.qualifier(*args)
            value['scope'] = 'selected-cohort-roles-catalog-and-ledger-only'
            value['databases']['auth_db'] = {'mode':'restore-only','from':26,'to':26}
            return value
        self.restore = self.fix.invoke(qualify)
        self.base = self.fix.base
        self.custody = self.base.journal.state['steps']['cohort-readback']['receipt']
        self.manifest_sha = digest(self.fix.manifest)
        self.metadata = {'private': 'never-copy-this-private-payload', 'migrations': {'auth_db': {'files': {'V26__fixture.sql': 'a'*64}}}}
        self.plan_sha = '9' * 64

    def create(self):
        with patch.object(restore, '_source'):
            return adapter.AuthFlywayJournal(self.base.phase, lambda: None, self.base.binding,
                self.manifest_sha, digest(self.custody), digest(self.restore), self.plan_sha, self.metadata)

    def progress(self, journal):
        journal.state['flyway_clone'] = {'status':'starting','resources':[]}; journal.save()
        def add(i, image):
            resources=journal.state['flyway_clone']['resources']
            resource={'name_hint':'voice-auth-flyway-'+str(i)*32,'image':image,
                      'image_id':'sha256:'+('a' if i==1 else 'b')*64,'state':'creating'}
            resources.append(copy.deepcopy(resource)); journal.save()
            resource.update(id=str(i)*64,state='created'); resources[-1]=copy.deepcopy(resource); journal.save()
            resource.update(volumes=['fixture-volume'] if i==1 else [],state='owned'); resources[-1]=copy.deepcopy(resource); journal.save()
        add(1,runtime.QUALIFIED_TOOLS['postgres']); add(2,runtime.QUALIFIED_TOOLS['flyway'])
        journal.stage('flyway-migrate','pending'); add(3,runtime.QUALIFIED_TOOLS['flyway']); journal.stage('flyway-migrate','done')
        add(4,runtime.QUALIFIED_TOOLS['flyway'])
        resources=journal.state['flyway_clone']['resources']
        for i in range(3,-1,-1):
            resources[i]['state']='removing'; journal.save()
            resources[i]['state']='removed'; journal.save()
        return copy.deepcopy(resources)

    def receipt(self, resources):
        b=self.base.binding
        return {'protocol':'voice-auth-flyway-cohort-clone-v1','operation_id':b['operation_id'],
                'snapshot_id':b['snapshot_id'],'source_sha':b['source_sha'],'target':32,
                'plan_sha256':self.plan_sha,'manifest_sha256':self.manifest_sha,
                'custody_sha256':digest(self.custody),'restore_sha256':digest(self.restore),
                'metadata_sha256':digest(self.metadata),'postgres_image':runtime.QUALIFIED_TOOLS['postgres'],
                'flyway_image':runtime.QUALIFIED_TOOLS['flyway'],'source_catalog':self.metadata['migrations']['auth_db']['files'],'before_history':[],
                'after_history':[],'before_markers':{'private':'never-copy-this-private-payload'},
                'expected_after_markers':{},'after_markers':{},'original_table_columns':{},
                'before_catalog_sha256':'1'*64,'after_catalog_sha256':'2'*64,'roles_sha256':'3'*64,
                'scope':'isolated-auth-clone-only','cleanup':'confirmed','resources':resources}

    def test_full_fenced_resource_lifecycle_redacts_private_receipt(self):
        journal=self.create(); resources=self.progress(journal)
        journal.state['flyway_clone']['status']='complete'; journal.state['flyway_clone']['receipt']=self.receipt(resources)
        with patch.object(adapter,'_assert_gone') as gone: journal.save()
        gone.assert_called_once()
        row=self.base.base.ledger.row
        self.assertEqual(row['cohort_auth_flyway']['status'],'done')
        self.assertNotIn('never-copy-this-private-payload',str(row))
        self.assertEqual(row['cohort_restore_resources']['postgres']['status'],'removed')

    def test_drift_after_pending_is_uncertain(self):
        journal=self.create(); journal.state['flyway_clone']={'status':'starting','resources':[]}; journal.save()
        self.base.base.ledger.row['cohort_readback']['receipt']['artifact_id'] = 999
        with self.assertRaises(adapter.CloneJournalError): journal.guard()
        self.assertEqual(self.base.base.ledger.row['status'],'uncertain')

    def test_no_second_clone_and_no_complete_without_cleanup(self):
        journal=self.create()
        with self.assertRaises(adapter.CloneJournalError): self.create()
        journal.state['flyway_clone']={'status':'complete','resources':[],'receipt':{}}
        with self.assertRaises(adapter.CloneJournalError): journal.save()

    def test_lost_ledger_response_poisoned_without_retry(self):
        journal=self.create(); ledger=self.base.base.ledger; write=ledger.write
        def lost(revision,row):
            write(revision,row)
            raise OSError('synthetic acknowledgement lost')
        journal.state['flyway_clone']={'status':'starting','resources':[]}
        with patch.object(ledger,'write',side_effect=lost), self.assertRaises(adapter.CloneJournalError): journal.save()
        self.assertTrue(self.base.media.poisoned)
        with self.assertRaises(adapter.CloneJournalError): journal.guard()

    def test_full_set_readback_failure_never_completes(self):
        journal=self.create(); resources=self.progress(journal)
        journal.state['flyway_clone']['status']='complete'; journal.state['flyway_clone']['receipt']=self.receipt(resources)
        with patch.object(adapter,'_assert_gone',side_effect=ValueError('remaining resource')), self.assertRaises(adapter.CloneJournalError): journal.save()
        self.assertNotEqual(self.base.base.ledger.row['cohort_auth_flyway']['status'],'done')
        self.assertEqual(self.base.base.ledger.row['status'],'uncertain')

    def test_resource_identity_drift_never_advances(self):
        journal=self.create(); journal.state['flyway_clone']={'status':'starting','resources':[]}; journal.save()
        self.base.base.ledger.row['cohort_auth_flyway']['nonce']='0'*32
        with self.assertRaises(adapter.CloneJournalError): journal.guard()
        self.assertEqual(self.base.base.ledger.row['status'],'uncertain')


if __name__ == '__main__': unittest.main()
