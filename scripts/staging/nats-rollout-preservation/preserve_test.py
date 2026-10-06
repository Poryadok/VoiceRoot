import copy
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch,Mock
import preserve
import rollout_census
from rollout_census_test import ACCOUNT, Runtime, fixture


class PreservationTest(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.base=Path(self.temp.name);self.source=self.base/'source';self.source.mkdir()
        (self.source/'records.blk').write_bytes(b'known synthetic record bytes')
        self.runtime=type('Runtime',(),{'base':self.base,'no_operation_containers':lambda self,**kw:None})()
        self.row=rollout_census.census(Runtime(fixture()),'owned-copy',ACCOUNT)
        self.fences=[]
    def fence(self):self.fences.append('verified')
    def capture(self):
        with patch.object(preserve,'isolated_census',return_value=copy.deepcopy(self.row)):
            return preserve.capture_cut(self.runtime,self.source,self.fence)
    def test_populated_cut_and_post_apply_preserve_pending_and_ack(self):
        cut=self.capture()
        with patch.object(preserve,'isolated_census',return_value=copy.deepcopy(self.row)):
            result=preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
        self.assertEqual(result['messages'],20);self.assertTrue(result['census_verified'])
        self.assertEqual(cut['census']['consumers'][0]['num_ack_pending'],2)
        self.assertEqual(len(self.fences),5)
    def test_retry_keeps_each_immutable_post_apply_archive(self):
        cut=self.capture()
        with patch.object(preserve,"isolated_census",return_value=copy.deepcopy(self.row)):
            first=preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
            second=preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
        self.assertNotEqual(first["archive"],second["archive"])
        self.assertTrue((self.base/first["archive"]).is_file())
        self.assertTrue((self.base/second["archive"]).is_file())

    def test_native_record_loss_blocks_before_any_census(self):
        cut=self.capture();(self.source/'records.blk').write_bytes(b'')
        with patch.object(preserve,'isolated_census') as read:
            with self.assertRaisesRegex(preserve.Blocked,'native_store_changed'):
                preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
            read.assert_not_called()
    def test_ack_cursor_change_blocks_resume(self):
        cut=self.capture();changed=copy.deepcopy(self.row);changed['consumers'][0]['ack_floor']['stream_seq']=4
        with patch.object(preserve,'isolated_census',return_value=changed):
            with self.assertRaisesRegex(preserve.Blocked,'populated_census_changed'):
                preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
    def test_cut_digest_tamper_blocks_resume(self):
        cut=self.capture();cut['census']['consumers'][0]['num_pending']=0
        with patch.object(preserve,'isolated_census',return_value=copy.deepcopy(self.row)):
            with self.assertRaisesRegex(preserve.Blocked,'populated_census_changed'):
                preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)

    def recovery_fixture(self):
        before=copy.deepcopy(self.row);before['consumers'][0].update({'durable':'','inactive_threshold':300_000_000_000,'created':'2026-01-01T00:00:00Z'})
        after=copy.deepcopy(before);after['consumers'][0]['created']='2026-10-06T00:00:01Z'
        recovery={'server_image':preserve.NATS_IMAGE,'container_id':'owned-fixture-container',
            'started_at':'2026-10-06T00:00:00+00:00','finished_at':'2026-10-06T00:00:02+00:00'}
        row=before['consumers'][0]
        manifest={'files':[{'path':'jetstream/account/streams/'+row['stream']+'/obs/'+row['name']+'/o.dat','sha256':'a'*64}]}
        return before,after,recovery,manifest

    def test_controlled_ephemeral_reopen_records_creation_observation_without_global_waiver(self):
        before,after,recovery,manifest=self.recovery_fixture()
        compared,observations=preserve.recovered_census(before,after,recovery,manifest)
        self.assertEqual(rollout_census.semantic(compared),rollout_census.semantic(before))
        self.assertNotEqual(rollout_census.semantic(after),rollout_census.semantic(before))
        self.assertEqual(observations[0]['recovered_created'],'2026-10-06T00:00:01Z')
        self.assertEqual(after['consumers'][0]['created'],'2026-10-06T00:00:01Z')

    def test_creation_observation_rejects_unbound_or_changed_state(self):
        for changed in ('image','interval','durable','config','ack','missing-state','expiry'):
            before,after,recovery,manifest=self.recovery_fixture()
            if changed=='image':recovery['server_image']='unapproved-image'
            elif changed=='interval':after['consumers'][0]['created']='2026-10-06T00:00:03Z'
            elif changed=='durable':before['consumers'][0]['durable']=after['consumers'][0]['durable']='existing-durable'
            elif changed=='config':after['consumers'][0]['config_sha256']='0'*64
            elif changed=='ack':after['consumers'][0]['ack_floor']['stream_seq']+=1
            elif changed=='missing-state':manifest['files']=[]
            else:before['consumers'][0]['inactive_threshold']=1_000_000_000
            with self.subTest(changed=changed),self.assertRaises(preserve.Blocked):preserve.recovered_census(before,after,recovery,manifest)

    def test_isolated_copy_releases_exact_owned_broker_for_repeat_and_failure(self):
        runtime=Mock();runtime.base=self.base;runtime.owned={};runtime.run.return_value=''
        runtime.inspect.return_value={'Config':{'Image':preserve.NATS_IMAGE},'HostConfig':{'NetworkMode':'none'}}
        def start(label,path):
            if label in runtime.owned:raise RuntimeError('fixture-name-collision')
            runtime.owned[label]={'id':'a'*64};return label
        runtime.start_broker.side_effect=start
        with patch.object(preserve,'verify_archive',return_value=True),patch.object(preserve,'ready'),\
             patch.object(preserve,'census',return_value=self.row) as read:
            for attempt in range(2):
                preserve.isolated_census(runtime,self.base/'archive',{},'same-label')
                self.assertEqual(runtime.owned,{})
            read.side_effect=RuntimeError('fixture-census-interrupted')
            with self.assertRaisesRegex(RuntimeError,'census-interrupted'):
                preserve.isolated_census(runtime,self.base/'archive',{},'same-label')
        self.assertEqual(runtime.owned,{})
        self.assertEqual(runtime.stop.call_count,3)
        self.assertEqual(sum(call.args[0][0]=='rm' for call in runtime.run.call_args_list),3)

    def test_fixed_updated_stream_creation_requires_native_target_and_original_attempt(self):
        for bad in (None,'untouched','missing','image','interval','native','config'):
            before=copy.deepcopy(self.row);old=before['streams'][0]
            old.update(name='chat_events',created='2026-01-01T00:00:00Z')
            after=copy.deepcopy(before);after['streams'][0]['created']='2026-10-06T00:00:01Z'
            evidence={'native_created':after['streams'][0]['created'],'target_config_sha256':old['config_sha256'],
                'attempt':{'server_image':preserve.NATS_IMAGE,'container_id':'original-owned-attempt',
                    'started_at':'2026-10-06T00:00:00+00:00','finished_at':'2026-10-06T00:00:02+00:00'}}
            recovery={'stream_updates':{'chat_events':evidence}}
            if bad=='untouched':old['name']=after['streams'][0]['name']='message_events'
            elif bad=='missing':recovery={}
            elif bad=='image':evidence['attempt']['server_image']='unapproved'
            elif bad=='interval':evidence['attempt']['started_at']='2026-10-06T00:00:02+00:00'
            elif bad=='native':evidence['native_created']='2026-10-06T00:00:00Z'
            elif bad=='config':evidence['target_config_sha256']='0'*64
            with self.subTest(bad=bad):
                if bad:
                    with self.assertRaises(preserve.Blocked):preserve.recovered_census(before,after,recovery,{'files':[]})
                else:
                    compared,observed=preserve.recovered_census(before,after,recovery,{'files':[]})
                    self.assertEqual(compared,before);self.assertEqual(observed[0]['migration_attempt']['container_id'],'original-owned-attempt')
                    self.assertNotEqual(after,before)

if __name__=='__main__':unittest.main()
