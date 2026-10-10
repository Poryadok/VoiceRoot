import copy
import json
import tempfile
from pathlib import Path
import unittest
from unittest.mock import Mock, patch
import preserve
import rollout_census
from rollout_census_test import ACCOUNT, Runtime, fixture


class PreservationTest(unittest.TestCase):
    def setUp(self):
        # These tests isolate archive/census composition; the synthetic block
        # is not a native JetStream layout. Native admission has separate tests.
        native=patch.object(preserve,'native_catalog',return_value={})
        catalog=patch.object(preserve,'verify_catalog')
        states=patch.object(preserve,'closed_native_states',return_value={})
        native.start();states.start();catalog.start()
        self.addCleanup(native.stop);self.addCleanup(states.stop);self.addCleanup(catalog.stop)
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

    def test_closed_snapshot_plan_validation_precedes_cut_acceptance_and_cleans_on_reject(self):
        runtime=Mock();runtime.base=self.base;runtime.operation='fixture123';runtime.owned={};runtime.run.return_value=''
        runtime.account_id.return_value=ACCOUNT;tree=fixture();runtime.monitor_jsz.return_value=tree
        runtime.inspect.return_value={'Id':'a'*64,'Config':{'Image':preserve.NATS_IMAGE},'HostConfig':{'NetworkMode':'none'}}
        def start(label,path):
            name='voice-known-'+runtime.operation+'-'+label
            runtime.owned[name]={'id':'a'*64};return name
        runtime.start_broker.side_effect=start
        seen=[]
        def validate(actual,broker,snapshot,row,manifest):
            self.assertIs(actual,runtime);self.assertIs(snapshot,tree)
            self.assertEqual(row,self.row);self.assertIn(broker,runtime.owned)
            seen.append('validated')
        with patch.object(preserve,'verify_archive',return_value=True),patch.object(preserve,'ready'):
            self.assertEqual(preserve.isolated_census(runtime,self.base/'archive',{},'validated-copy',validate=validate),self.row)
            self.assertEqual(seen,['validated']);self.assertEqual(runtime.owned,{})
            def reject(*args):raise preserve.Blocked('original_plan_changed')
            with self.assertRaisesRegex(preserve.Blocked,'original_plan_changed'):
                preserve.isolated_census(runtime,self.base/'archive',{},'validated-copy',validate=reject)
        self.assertEqual(runtime.monitor_jsz.call_count,2)
        self.assertEqual(runtime.owned,{})
        self.assertEqual(list(self.base.glob('validated-copy-*')),[])

    def test_closed_copy_restore_start_readiness_and_cleanup_failures_cannot_return_a_cut(self):
        for failure in ('restore','start','ready','remove','absence'):
            with self.subTest(failure=failure):
                runtime=Mock();runtime.base=self.base;runtime.operation='fixture123';runtime.owned={}
                name='voice-known-fixture123-checked';cid='a'*64
                runtime.inspect.return_value={'Id':cid,'Config':{'Image':preserve.NATS_IMAGE},'HostConfig':{'NetworkMode':'none'}}
                def reject(*args,**kwargs):raise preserve.Blocked('fixture_'+failure)
                def start(*args):
                    runtime.owned[name]={'id':cid}
                    if failure=='start':reject()
                    return name
                runtime.start_broker.side_effect=start
                if failure=='restore':runtime.restore.side_effect=reject
                def command(args):
                    if args[0]=='rm' and failure=='remove':reject()
                    return cid if args[0]=='ps' and failure=='absence' else ''
                runtime.run.side_effect=command;runtime.monitor_jsz.return_value=fixture();runtime.account_id.return_value=ACCOUNT
                with patch.object(preserve,'verify_archive',return_value=True),patch.object(preserve,'ready',side_effect=reject if failure=='ready' else None):
                    with self.assertRaises(preserve.Blocked):preserve.closed_copy_census(runtime,self.base/'archive',{},'checked',lambda *a:None)
                if failure in ('remove','absence'):
                    # Keep owned ledger/copy on unverifiable cleanup. A caller
                    # cannot receive an accepted cut or retry as output-free.
                    self.assertIn(name,runtime.owned)
                    for path in self.base.glob('checked-*'):
                        import shutil
                        shutil.rmtree(path)
                else:
                    self.assertEqual(runtime.owned,{})
                    self.assertEqual(list(self.base.glob('checked-*')),[])

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
class NativeStateAdmissionTest(unittest.TestCase):
    def test_stream_native_directory_without_metadata_cannot_disappear(self):
        runtime=Mock();runtime.account_id.return_value='ACCOUNT'
        for paths in (['jetstream/ACCOUNT/streams/LOST'],['jetstream/ACCOUNT/streams/LOST/msgs/1.blk']):
            with self.subTest(paths=paths),self.assertRaisesRegex(preserve.Blocked,'stream_metadata_missing'):
                preserve.native_catalog(runtime,Path('/private/cut.tar'),
                    {'files':[{'path':p} for p in paths],'dirs':[]},lambda:None)

    def test_restored_broker_cannot_omit_native_stream(self):
        with self.assertRaisesRegex(preserve.Blocked,'restored_native_inventory_changed'):
            preserve.verify_catalog({'account':'ACCOUNT','streams':['LOST'],'consumers':[]},
                {'account':'ACCOUNT','streams':[],'consumers':[]})

    def test_every_native_consumer_is_decoded_with_complete_pending_state(self):
        path='jetstream/'+('A'+'B'*55)+'/streams/BASE/obs/durable/o.dat'
        row={'account':'A'+'B'*55,'consumers':[{'stream':'BASE','name':'durable'}]}
        root=path.rsplit('/',1)[0]
        manifest={'files':[{'path':root+'/'+name} for name in ('o.dat','meta.inf','meta.sum')],'dirs':[root]}
        decoded={'schema':'nats-closed-durable-consumer-v1','version':2,
            'ack_floor':{'consumer':0,'stream':0},'delivered':{'consumer':2,'stream':3},
            'pending':{'3':{'consumer_sequence':2,'timestamp_ns':1700000000000000000}},
            'redelivered':{'3':1}}
        reader=Mock();reader.member.return_value=b'full native state'
        runtime=Mock();runtime.base=Path('/private');runtime.operation='123456abcdef'
        with patch.object(preserve,'_native_members',return_value={path:b'full native state'}),patch('commands.capture',return_value=json.dumps(decoded).encode()):
            proof=preserve.closed_native_states(runtime,Path('/private/cut.tar'),manifest,row,lambda:None)
        self.assertEqual(proof[path],decoded)

    def test_native_consumer_missing_from_census_or_missing_state_is_rejected(self):
        path='jetstream/ACCOUNT/streams/BASE/obs/durable/o.dat'
        runtime=Mock();runtime.base=Path('/private')
        for files,consumers in (([{'path':path}],[]),([],[{'stream':'BASE','name':'durable'}]),
            ([{'path':'jetstream/ACCOUNT/streams/BASE/obs/omitted/meta.inf'}],[])):
            with self.subTest(files=files),self.assertRaisesRegex(preserve.Blocked,'native_consumer_inventory'):
                preserve.closed_native_states(runtime,Path('/private/cut.tar'),{'files':files},
                    {'account':'ACCOUNT','consumers':consumers},lambda:None)
