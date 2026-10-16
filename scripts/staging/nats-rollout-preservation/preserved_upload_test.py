"""Preserved cut and fresh upload execution are separate authority domains."""
import copy
from datetime import datetime,timezone,timedelta
import unittest
import preserved_upload


class PreservedUploadTests(unittest.TestCase):
    def setUp(self):
        self.now=datetime(2026,10,9,8,0,tzinfo=timezone.utc)
        self.state={'operation':'3340764a7d24','phase':'AWAITING_OFF_NODE','status':'WAITING','fence_status':'VERIFIED',
            'target':{'tag':'73ca52699ddf6a9182e3407d5bbdedbc29c06dee'},
            'execution_authority':{'nonce':'6138b79b7fc09bbcf978ddf64269251259b9b360357634c49459e0309e2457af',
                'dispatcher_run_id':37900273078,'head_sha':'8d0a51144b9aae00bd75a919b1be6f90ad3f62a8'},
            'cipher_binding':{'operation':'3340764a7d24','run_id':37900273078,'head_sha':'8d0a51144b9aae00bd75a919b1be6f90ad3f62a8',
                'challenge':'a'*32,'cipher_sha256':'b'*64,'cipher_bytes':17,'created_at':'2026-10-09T07:42:00+00:00'},
            'nats_contract_expires_at':'2026-10-09T11:42:00+00:00',
            'cut':{'manifest':{'archive_sha256':'c'*64},'manifest_sha256':'d'*64,'census_sha256':'e'*64}}
        self.execution={'dispatcher_run_id':37910000000,'run_attempt':1,'head_sha':'f'*40,
            'event':'workflow_dispatch','workflow_id':263689731,'path':'.github/workflows/staging-deploy.yml','repository':'Poryadok/VoiceRoot','nonce':'1'*64}

    def test_old_cipher_window_can_expire_but_original_proof_and_bytes_never_change(self):
        original=copy.deepcopy(self.state)
        identity=preserved_upload.identity(self.state,self.execution,now=self.now,challenge='2'*32)
        self.assertEqual(self.state,original)
        self.assertEqual(identity['original_cipher_binding'],original['cipher_binding'])
        self.assertEqual(identity['artifact_binding']['run_id'],37910000000)
        self.assertEqual(identity['artifact_binding']['head_sha'],'f'*40)
        self.assertEqual(identity['artifact_binding']['cipher_sha256'],'b'*64)
        self.assertEqual(identity['deadline'],'2026-10-09T08:10:00+00:00')
        self.assertNotEqual(identity['artifact_binding']['created_at'],original['cipher_binding']['created_at'])

    def test_phase_identity_proof_and_fresh_dispatcher_drifts_reject(self):
        changes=[('phase','PAUSED_APPLY'),('status','BLOCKED'),('fence_status','UNKNOWN'),('operation','0'*12),
            ('nats_contract_expires_at',self.now.isoformat())]
        for key,value in changes:
            state=copy.deepcopy(self.state);state[key]=value
            with self.subTest(key=key),self.assertRaises(preserved_upload.UploadError):
                preserved_upload.identity(state,self.execution,now=self.now,challenge='2'*32)
        for key,value in [('nonce','bad'),('run_attempt',0),('repository','other/repo'),('path','other.yml'),
                          ('workflow_id',1),('event','push'),('dispatcher_run_id',37900273078)]:
            execution={**self.execution,key:value}
            with self.subTest(key=key),self.assertRaises(preserved_upload.UploadError):
                preserved_upload.identity(self.state,execution,now=self.now,challenge='2'*32)

    def test_fresh_upload_window_cannot_extend_original_proof_or_replay_time(self):
        self.state['nats_contract_expires_at']='2026-10-09T08:05:00+00:00'
        identity=preserved_upload.identity(self.state,self.execution,now=self.now,challenge='2'*32)
        self.assertEqual(identity['deadline'],'2026-10-09T08:05:00+00:00')
        preserved_upload.verify_deadline(identity,self.now+timedelta(minutes=4))
        with self.assertRaises(preserved_upload.UploadError):
            preserved_upload.verify_deadline(identity,self.now+timedelta(minutes=5))
        with self.assertRaises(preserved_upload.UploadError):
            preserved_upload.verify_deadline(identity,self.now-timedelta(seconds=1))


if __name__=='__main__':unittest.main()
