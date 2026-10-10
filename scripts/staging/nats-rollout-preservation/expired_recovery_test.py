"""Exact expired recovery authority: old permissions never become fresh proof."""
import copy
from datetime import datetime,timezone,timedelta
import unittest
import json
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).parents[1]/'nats-known-baseline'))
from types import SimpleNamespace
from unittest.mock import patch
import expired_recovery as recovery
from preserved_upload_test import PreservedUploadTests
from preserved_upload import digest


class RecoveryAuthorityTests(unittest.TestCase):
    def setUp(self):
        original=PreservedUploadTests();original.setUp()
        self.state=copy.deepcopy(original.state)
        self.state.update(phase='NATS_CONTRACT_MIGRATION',status='BLOCKED',error='blocked_unclassified')
        self.state['target'].update(mode='images-only',changed_services=['story'])
        self.state['context']={'owned':{'voice-story':{'replicas':0}}}
        self.now=datetime(2026,10,9,13,0,tzinfo=timezone.utc)
        self.execution=copy.deepcopy(original.execution)
        self.execution['dispatcher_run_id']=37930000000
        self.proof={'schema':'voice-expired-native-proof-v1','operation':self.state['operation'],
            'started_at':(self.now-timedelta(seconds=30)).isoformat(),'completed_at':self.now.isoformat(),
            'checkpoint_sha256':digest(self.state),'original_cut_sha256':digest(self.state['cut']),
            'original_cipher_sha256':digest(self.state['cipher_binding']),
            'original_target_sha256':digest(self.state['target']),
            'original_execution_sha256':digest(self.state['execution_authority']),
            'context_sha256':digest(self.state['context']),
            'admission_sha256':'a'*64,'repair_sha256':'b'*64,'source_sha256':'c'*64,
            'selected_inventory_sha256':'d'*64,'native_disposition_sha256':'e'*64,
            'full_state_proof_sha256':'f'*64,'original_readback_sha256':'1'*64,
            'observation_readback_sha256':'2'*64}

    def test_actual_fresh_proof_authority_is_distinct_from_expired_old_permission(self):
        before=copy.deepcopy(self.state)
        record=recovery.authority(self.state,self.execution,self.proof,now=self.now)
        self.assertEqual(self.state,before)
        self.assertEqual(record['original_expires_at'],self.state['nats_contract_expires_at'])
        self.assertEqual(record['expires_at'],(self.now+timedelta(seconds=570)).isoformat())
        self.assertEqual(record['proof_sha256'],digest(self.proof))
        recovery.verify_authority(record,self.state,self.proof,self.execution,now=self.now)

    def test_missing_or_stale_proof_cannot_be_a_clock_only_refresh(self):
        for key in ('repair_sha256','full_state_proof_sha256','original_readback_sha256',
                    'observation_readback_sha256','selected_inventory_sha256'):
            proof=copy.deepcopy(self.proof);proof.pop(key)
            with self.subTest(key=key),self.assertRaises(ValueError):
                recovery.authority(self.state,self.execution,proof,now=self.now)
        proof=copy.deepcopy(self.proof);proof['started_at']=(self.now-timedelta(seconds=600)).isoformat()
        with self.assertRaises(ValueError):recovery.authority(self.state,self.execution,proof,now=self.now)

    def test_selected_drift_after_readback_blocks_before_broker_callback(self):
        calls=[]
        record=recovery.authority(self.state,self.execution,self.proof,now=self.now)
        with self.assertRaises(ValueError):
            recovery.enter(record,self.state,self.proof,self.execution,lambda:'0'*64,
                lambda event:calls.append(event),lambda:calls.append('broker'),now=self.now)
        self.assertEqual(calls,[])

    def test_entry_is_durable_prospective_progress_not_completed_mutation(self):
        calls=[];record=recovery.authority(self.state,self.execution,self.proof,now=self.now)
        def failed_start():
            self.assertEqual(calls[0]['kind'],'expired_recovery_transaction_entered')
            raise RuntimeError('synthetic_broker_not_started')
        with self.assertRaises(RuntimeError):
            recovery.enter(record,self.state,self.proof,self.execution,lambda:self.proof['selected_inventory_sha256'],
                calls.append,failed_start,now=self.now)
        self.assertEqual(len(calls),1)
        self.assertNotIn('mutation_completed',calls[0])

    def test_expiry_during_inventory_or_after_entry_never_starts_broker(self):
        for after_entry in (False,True):
            calls=[];record=recovery.authority(self.state,self.execution,self.proof,now=self.now)
            times=iter([self.now,self.now+timedelta(seconds=600)] if after_entry else [self.now+timedelta(seconds=600)])
            with self.subTest(after_entry=after_entry),self.assertRaises(ValueError):
                recovery.enter(record,self.state,self.proof,self.execution,
                    lambda:self.proof['selected_inventory_sha256'],calls.append,
                    lambda:calls.append('broker'),now=self.now,clock=lambda:next(times))
            self.assertNotIn('broker',calls)
            self.assertEqual(len(calls),int(after_entry))

    def test_adoption_is_exact_blocked_snapshot_and_cannot_reset_progress(self):
        binding={'manifest_sha256':'a'*64}
        raw=json.dumps(self.state,sort_keys=True).encode()
        record=recovery.adoption_record(raw,recovery.V9_BINDING,binding)
        self.assertEqual(record['adopted_checkpoint_sha256'],__import__('hashlib').sha256(raw).hexdigest())
        self.assertEqual(record['original_expires_at'],self.state['nats_contract_expires_at'])
        for key,value in [('phase','PAUSED_APPLY'),('status','WAITING'),('authorization',{'approved':True}),
                          ('nats_migration',{'verified':True})]:
            state=copy.deepcopy(self.state);state[key]=value
            with self.subTest(key=key),self.assertRaises(ValueError):
                recovery.adoption_record(json.dumps(state).encode(),recovery.V9_BINDING,binding)

    def test_common_reader_uses_real_v9_binding_and_retains_progress_and_history(self):
        import paused_recovery
        old=json.loads((Path(__file__).parent/'testdata/expired-recovery-v9-manifest.json').read_text())
        self.assertEqual(digest(old),recovery.V9_BINDING)
        binding={'expired_recovery.py':'a'*64}
        adopted=copy.deepcopy(self.state);adopted['events']=[{'kind':'retained-before-intent'}]
        raw=json.dumps(adopted,sort_keys=True).encode()
        record=recovery.adoption_record(raw,recovery.V9_BINDING,binding)
        base=Path('/root-fixture/rollout-'+adopted['operation'])
        current=copy.deepcopy(adopted)
        current.update(phase='PAUSED_APPLY',status='WAITING',context={'actual-progress':'retained'})
        current['events'].append({'kind':'expired_recovery_transaction_entered'})
        fake_cli=SimpleNamespace(code_binding=lambda code:old)
        # Fixed root filesystem reads and the preexisting V9 physical chain are
        # adapter seams here; actual new shared reader and record logic execute.
        with patch.dict('sys.modules',{'root_cli':fake_cli}),\
             patch('guard.ROOT',base.parent),patch.object(recovery.upload,'private_read',return_value=raw),\
             patch.object(recovery.upload,'private_json',return_value=record),\
             patch.object(recovery.upload,'verify_helper_continuity') as chain,\
             patch.object(paused_recovery,'original_target_unchanged') as target:
            self.assertEqual(recovery.verify_adopted_binding(base,current,binding),record)
            chain.assert_called_once_with(base,adopted,old)
            target.assert_called_once_with(current,adopted)
            for key,value in [('cut',{'replaced':True}),('cipher_binding',{}),('nats_contract_expires_at','renewed'),
                              ('events',[{'kind':'rewritten-history'}])]:
                changed=copy.deepcopy(current);changed[key]=value
                with self.subTest(key=key),self.assertRaises(ValueError):
                    recovery.verify_adopted_binding(base,changed,binding)

    def test_failed_execution_disposition_preserves_exact_known_request_not_replay(self):
        request=copy.deepcopy(recovery.FAILED_REQUEST)
        journal={'schema':'voice-nats-bridge-request-v1','request':request,'request_sha256':digest(request),
            'phase':'STARTED','prepare_error':'unexpected'}
        response={'status':'BLOCKED','error':'blocked_unclassified'}
        raw=json.dumps(journal).encode();answer=json.dumps(response).encode()
        record=recovery.failed_execution(raw,answer)
        self.assertEqual(record['disposition'],'FROZEN_FAILED_NATIVE_PRE_INTENT_NO_REPLAY')
        self.assertEqual(json.loads(raw),journal)
        for key,value in [('run_id',1),('nonce','0'*64),('artifact_id',1),('upload_nonce','0'*64),('operation','0'*12)]:
            changed=copy.deepcopy(journal);changed['request'][key]=value;changed['request_sha256']=digest(changed['request'])
            with self.subTest(key=key),self.assertRaises(ValueError):
                recovery.failed_execution(json.dumps(changed).encode(),answer)


class RuntimeInputTests(unittest.TestCase):
    def test_rematerialized_mount_is_reverified_at_every_use(self):
        from docker_runtime import DockerRuntime
        base=Path('/root-fixture/rollout-'+recovery.upload.OPERATION)
        runtime=object.__new__(DockerRuntime);runtime.base=base
        with patch.object(Path,'exists',return_value=True),\
             patch.object(recovery,'runtime_inputs',return_value=base/'recovery-runtime-inputs') as verify:
            self.assertEqual(runtime.inputs_directory(),base/'recovery-runtime-inputs')
            self.assertEqual(runtime.inputs_directory(),base/'recovery-runtime-inputs')
            self.assertEqual(verify.call_count,2)
            verify.assert_called_with(base)
        with patch.object(Path,'exists',return_value=False):
            self.assertEqual(runtime.inputs_directory(),base/'inputs')

    def test_changed_original_input_cannot_admit_fresh_runtime_copy(self):
        base=Path('/root-fixture/rollout-'+recovery.upload.OPERATION)
        record={'schema':'voice-expired-input-repair-v1','operation':recovery.upload.OPERATION,
            'input_hashes':{name:'a'*64 for name in recovery.INPUT_FILES},'files':{'bound':True}}
        with patch.object(recovery,'input_fingerprints',return_value={'changed':True}),\
             self.assertRaisesRegex(ValueError,'runtime_inputs_original_changed'):
            recovery.runtime_inputs(base,record)


class FinishLedgerTests(unittest.TestCase):
    def ledger(self):
        from stage_runtime import HUB
        self.hub=HUB;self.calls=[];self.running=set();self.expired=False
        def authority():
            if self.expired:raise recovery.RecoveryError('expired')
            self.calls.append('authority')
        def closed():
            self.calls.append('closed')
            if self.running:raise recovery.RecoveryError('not_closed')
        def running(name):
            self.calls.append(('running',name))
            if name not in self.running:raise recovery.RecoveryError('not_running')
        record={'schema':'voice-expired-finish-ledger-v1','operation':recovery.upload.OPERATION,
            'authority_sha256':'a'*64,'pending':{name:{'uid':name+'-uid','template_sha256':'b'*64}
                for name in (HUB,'voice-story')},'completed':{},'nats_started':False}
        return recovery.FinishLedger(record,authority,closed,running,lambda value:self.calls.append(('saved',value)))
    def test_first_ready_uses_running_guard_and_later_workload_does_not_require_closed(self):
        ledger=self.ledger();ledger.guard('scale',self.hub,1);self.running.add(self.hub)
        ledger.guard('ready',self.hub);ledger.guard('scale','voice-story',1)
        self.running.add('voice-story');ledger.guard('ready','voice-story')
        ledger.guard('ready','voice-story') # A repeated readiness observation is not another scale action.
        ledger.guard('release');self.assertEqual(self.calls.count('closed'),1)
        with self.assertRaises(ValueError):ledger.guard('scale','voice-story',1)
    def test_verified_safety_pause_creates_new_obligation_and_does_not_replay_application(self):
        ledger=self.ledger();ledger.guard('scale',self.hub,1);self.running.add(self.hub);ledger.guard('ready',self.hub)
        ledger.guard('scale','voice-story',1);self.running.add('voice-story');ledger.guard('ready','voice-story')
        self.running.remove('voice-story')
        ledger.safety_paused('voice-story',{'uid':'voice-story-uid','template_sha256':'b'*64,
            'replicas':0,'proof_sha256':'c'*64})
        ledger.guard('scale','voice-story',1);self.running.add('voice-story');ledger.guard('ready','voice-story')
        self.assertEqual(self.calls.count('closed'),1)
        self.assertEqual(ledger.record['completed']['voice-story']['safety_pause_proof_sha256'],'c'*64)
    def test_expiry_after_final_running_check_cannot_release(self):
        ledger=self.ledger();ledger.guard('scale',self.hub,1);self.running.add(self.hub);ledger.guard('ready',self.hub)
        ledger.guard('scale','voice-story',1);self.running.add('voice-story');ledger.guard('ready','voice-story')
        def expire(name):self.expired=True
        ledger.verify_running=expire
        with self.assertRaises(ValueError):ledger.guard('release')


if __name__=='__main__':unittest.main()


class InstallHistoryTests(unittest.TestCase):
    def test_exact_preintent_history_rejects_issued_unknown_or_unclosed_before_upgrade(self):
        from docker_runtime import NATS_IMAGE
        state={'nats_contract':{'fixed':'plan'},'cut':{'manifest_sha256':'a'*64,'census_sha256':'b'*64}}
        awaiting={'events':[{'kind':'immutable-awaiting'}]}
        state['events']=copy.deepcopy(awaiting['events'])+[
            {'kind':'nats_contract_old_cut_verified','plan_sha256':digest(state['nats_contract']),
             'old_manifest_sha256':'a'*64,'old_census_sha256':'b'*64,
             'proof':{'native_files_verified':True,'census_verified':True}},
            {'kind':'nats_contract_broker_opened','container_id':'c'*64,'server_image':NATS_IMAGE,'started_at':'2026-10-09T10:00:00Z'},
            {'kind':'nats_contract_broker_closed','container_id':'c'*64,'server_image':NATS_IMAGE,
             'started_at':'2026-10-09T10:00:00Z','finished_at':'2026-10-09T10:01:00Z'}]
        before=copy.deepcopy(state)
        self.assertEqual(recovery.install_history(state,awaiting)['server_image'],NATS_IMAGE)
        self.assertEqual(state,before)
        for drift in ('issued','unknown','missing-close','prefix','proof','time'):
            changed=copy.deepcopy(state)
            if drift=='issued':changed['events'].append({'kind':'nats_contract_mutation_issued'})
            elif drift=='unknown':changed['events'].append({'kind':'nats_contract_unknown'})
            elif drift=='missing-close':changed['events'].pop()
            elif drift=='prefix':changed['events'][0]['kind']='substitute'
            elif drift=='proof':changed['events'][1]['proof']['census_verified']=False
            else:changed['events'][-1]['finished_at']='2026-10-09T09:59:00Z'
            with self.subTest(drift=drift),self.assertRaises(ValueError):recovery.install_history(changed,awaiting)
