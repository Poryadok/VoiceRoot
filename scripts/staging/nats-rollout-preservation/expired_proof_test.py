import copy
import unittest
import expired_proof
import sys
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch
from unittest.mock import Mock
sys.path.insert(0,str(Path(__file__).parents[1]/'nats-known-baseline'))

class RecordProofTests(unittest.TestCase):
    def test_current_copy_orderly_close_archives_only_exact_successful_exit(self):
        from docker_runtime import NATS_IMAGE
        from datetime import datetime, timezone, timedelta
        for failure in (None,'exit','running','identity','started','future'):
            with self.subTest(failure=failure),TemporaryDirectory() as folder:
                calls=[];now=datetime.now(timezone.utc)
                stamp=lambda value:value.isoformat().replace('+00:00','Z')
                class Runtime:
                    base=Path(folder);owned={};stopped=False
                    def restore(self,archive,target,manifest):target.mkdir()
                    def start_broker(self,label,target):
                        self.owned['copy']={'id':'1'*64};self.started=stamp(datetime.now(timezone.utc));return 'copy'
                    def inspect(self,name):
                        state={'Running':not self.stopped,'StartedAt':self.started,'FinishedAt':self.finished if self.stopped else '',
                            'ExitCode':0,'OOMKilled':False,'Dead':False,'Error':''}
                        cid='1'*64
                        if self.stopped:
                            if failure=='exit':state['ExitCode']=137
                            if failure=='running':state['Running']=True
                            if failure=='identity':cid='2'*64
                            if failure=='started':state['StartedAt']=stamp(now-timedelta(days=1))
                            if failure=='future':state['FinishedAt']=stamp(now+timedelta(days=1))
                        return {'Id':cid,'Config':{'Image':NATS_IMAGE},'HostConfig':{'NetworkMode':'none'},'State':state}
                    def stop(self,name):calls.append('stop');self.stopped=True;self.finished=stamp(datetime.now(timezone.utc))
                    def monitor_jsz(self,broker):return {'server_id':'owned'}
                    def account_id(self):return 'account'
                    def run(self,args):calls.append(args);return ''
                class Actor:
                    def __init__(self,*args):pass
                def archive(source,target):
                    calls.append('archive');self.assertTrue(runtime.stopped)
                    target.write_bytes(b'closed native');return {'archive_sha256':'a'*64}
                runtime=Runtime();target=Path(folder)/'current-copy-closed.tar'
                with patch('native_store.verify_archive',return_value=True),patch('native_store.archive_closed_store',side_effect=archive),\
                    patch('scenario.ready'),patch('rollout_census.census',return_value={'streams':[]}),patch('nats_contract_actor.Actor',Actor):
                    if failure is None:
                        result=expired_proof.observe_copy(runtime,Path(folder)/'archive',{},'current',lambda:None,
                            closed_archive=target)
                        self.assertEqual(result['closed_manifest'],{'archive_sha256':'a'*64})
                        self.assertEqual(result['orderly_exit']['container_id'],'1'*64)
                        self.assertEqual(result['orderly_exit']['exit_code'],0)
                        self.assertLess(calls.index('stop'),calls.index('archive'))
                        self.assertEqual(target.read_bytes(),b'closed native')
                    else:
                        with self.assertRaises(ValueError):
                            expired_proof.observe_copy(runtime,Path(folder)/'archive',{},'current',lambda:None,closed_archive=target)
                        self.assertNotIn('archive',calls);self.assertFalse(target.exists())
                self.assertEqual(runtime.owned,{})

    def test_current_copy_exports_full_private_index_without_duplicate_get(self):
        from docker_runtime import NATS_IMAGE
        with TemporaryDirectory() as folder:
            class Runtime:
                base=Path(folder);owned={}
                def restore(self,archive,target,manifest):target.mkdir()
                def start_broker(self,label,target):self.owned['copy']={'id':'1'*64};return 'copy'
                def inspect(self,name):return {'Id':self.owned[name]['id'],'Config':{'Image':NATS_IMAGE},'HostConfig':{'NetworkMode':'none'}}
                def monitor_jsz(self,broker):return {'captured':'private config tree'}
                def account_id(self):return 'account'
                def run(self,args):return ''
            calls=[];saved=[]
            row={'streams':[{'name':'events','state':{'messages':1,'first_seq':1,'last_seq':1}}]}
            record={'seq':1,'subject':'events.created','time':'2026-10-09T00:00:00Z','data':'eA==','hdrs':'TkFUUy8xLjANCg0K'}
            class Actor:
                def __init__(self,*args):pass
                def get_record(self,stream,sequence):calls.append((stream,sequence));return copy.deepcopy(record)
            def sink(stream,value):saved.append((stream,copy.deepcopy(value)));return {'file':'root-private-reference'}
            runtime=Runtime()
            with patch('native_store.verify_archive',return_value=True),patch('scenario.ready'),\
                patch('rollout_census.census',return_value=row),patch('nats_contract_actor.Actor',Actor):
                result=expired_proof.observe_copy(runtime,Path(folder)/'archive',{},'current',lambda:None,record_sink=sink)
            self.assertEqual(calls,[('events',1)])
            self.assertEqual(saved,[('events',record)])
            self.assertEqual(result['private_record_ledger']['streams']['events']['records']['1'],{'file':'root-private-reference'})
            self.assertEqual(result['private_record_ledger']['record_proof'],result['records'])
            self.assertEqual(result['private_monitor'],{'captured':'private config tree'})
            self.assertEqual(runtime.owned,{})

    def test_get_record_rejects_unknown_response_wrapper_fields(self):
        from nats_contract_actor import Actor
        message={'seq':1,'subject':'events','data':'YQ==','time':'2026-10-09T13:00:00Z'}
        actor=Actor(None,'broker','a'*64,'/copy',lambda:None)
        response={'type':'io.nats.jetstream.api.v1.stream_msg_get_response','message':message}
        with patch.object(actor,'_request',return_value=response):
            self.assertEqual(actor.get_record('events',1),message)
        for changed in ({**response,'unknown':'not approved'}, {**response,'type':'wrong-response'}):
            with patch.object(actor,'_request',return_value=changed),self.assertRaises(ValueError):
                actor.get_record('events',1)
    def test_actual_migration_freshness_hook_vetoes_before_mount_or_start(self):
        import nats_migration as migration
        from nats_contract_plan import digest
        plan={'sources':{},'actions':[{'fixture':'not executed'}]};enrollment={'fixture':'bound'}
        state={'operation':'123456abcdef','events':[],'custody':{'verified':True},
            'nats_contract':plan,'bootstrap_enrollment':enrollment,
            'nats_contract_binding':{'plan_sha256':digest(plan),'server_image':migration.NATS_IMAGE,
                'enrollment_sha256':digest(enrollment),'sources':{}},
            'cut':{'manifest':{'files':[]},'manifest_sha256':'a'*64,'census_sha256':'b'*64}}
        runtime=Mock();stage=Mock()
        def reject():raise ValueError('fixture_closed_inventory_or_expiry_drift')
        with TemporaryDirectory() as folder,patch.object(migration,'DockerRuntime',return_value=runtime),\
             patch.object(migration,'verify_post_apply',return_value={'verified':True}):
            with self.assertRaisesRegex(ValueError,'closed_inventory_or_expiry_drift'):
                migration.apply_contract(Path(folder),state,stage,state['events'].append,before_selected_start=reject)
        runtime.allow_bound_store.assert_not_called();runtime.start_broker.assert_not_called()

    def test_full_records_cover_undispatched_sequence_and_reconcile_deleted_gap(self):
        calls=[]
        class Actor:
            def get_record(self,stream,seq):
                calls.append((stream,seq))
                return None if seq==2 else {'subject':'events.one','seq':seq,'data':'YQ==','time':'2026-10-09T13:00:00Z'}
        row={'streams':[{'name':'events','state':{'messages':2,'first_seq':1,'last_seq':3}}]}
        result=expired_proof.record_digest(Actor(),row)
        self.assertEqual(calls,[('events',1),('events',2),('events',3)])
        self.assertEqual(result['messages'],2)
    def test_record_identity_count_and_unknown_fields_are_not_waived(self):
        row={'streams':[{'name':'events','state':{'messages':1,'first_seq':1,'last_seq':1}}]}
        for record in (None,{'seq':2,'subject':'events','data':'YQ==','time':'2026-10-09T13:00:00Z'},
                       {'seq':1,'subject':'events','data':'YQ==','time':'2026-10-09T13:00:00Z','unknown':1}):
            class Actor:
                def get_record(self,stream,seq):return copy.deepcopy(record)
            with self.subTest(record=record),self.assertRaises(ValueError):expired_proof.record_digest(Actor(),row)

    def test_private_copy_rejection_removes_exact_new_ids_and_preserves_prior_ledger(self):
        from docker_runtime import NATS_IMAGE
        with TemporaryDirectory() as folder:
            class Runtime:
                base=Path(folder);owned={'prior':{'id':'0'*64}}
                def restore(self,archive,target,manifest):target.mkdir()
                def start_broker(self,label,target):
                    self.owned['copy']={'id':'1'*64};return 'copy'
                def inspect(self,name):return {'Id':self.owned[name]['id'],
                    'Config':{'Image':NATS_IMAGE},'HostConfig':{'NetworkMode':'none'}}
                def monitor_jsz(self,broker):return {'captured':'one'}
                def account_id(self):return 'account'
                def run(self,args):
                    self.calls.append(args);return ''
                calls=[]
            runtime=Runtime();row={'streams':[]}
            class Actor:
                def __init__(self,*args):pass
            seen=[]
            def reject(*args):seen.append(args[2]);raise ValueError('synthetic_plan_drift')
            with patch('native_store.verify_archive',return_value=True),patch('scenario.ready'),\
                 patch('rollout_census.census',return_value=row),patch('nats_contract_actor.Actor',Actor):
                with self.assertRaises(ValueError):
                    expired_proof.observe_copy(runtime,Path(folder)/'archive',{},'copy',lambda:None,reject)
            self.assertEqual(seen,[{'captured':'one'}])
            self.assertEqual(runtime.owned,{'prior':{'id':'0'*64}})
            self.assertEqual(runtime.calls[0],['rm','-f','1'*64])
            self.assertEqual(list(Path(folder).iterdir()),[])

    def test_failed_interval_never_accepts_missing_close_intent_or_modified_prefix(self):
        from docker_runtime import NATS_IMAGE
        from nats_contract_plan import digest
        awaiting={'events':[{'kind':'original'}]}
        state={'nats_contract':{'exact':'plan'},'cut':{'manifest_sha256':'a'*64,'census_sha256':'b'*64},
            'events':awaiting['events']+[{'kind':'nats_contract_old_cut_verified',
                'plan_sha256':digest({'exact':'plan'}),'old_manifest_sha256':'a'*64,
                'old_census_sha256':'b'*64,'proof':{'native_files_verified':True,'census_verified':True}},
                {'kind':'nats_contract_broker_opened','server_image':NATS_IMAGE,'container_id':'c'*64,
                 'started_at':'2026-10-09T13:00:00Z'},
                {'kind':'nats_contract_broker_closed','server_image':NATS_IMAGE,'container_id':'c'*64,
                 'started_at':'2026-10-09T13:00:00Z','finished_at':'2026-10-09T13:00:20Z'}]}
        self.assertEqual(expired_proof.failed_startup_interval(state,awaiting)['server_image'],NATS_IMAGE)
        for change in ('prefix','intent','close','identity','native-proof'):
            wrong=copy.deepcopy(state)
            if change=='prefix':wrong['events'][0]['kind']='substitute'
            elif change=='intent':wrong['events'].append({'kind':'nats_contract_intent'})
            elif change=='close':wrong['events'].pop()
            elif change=='identity':wrong['events'][-1]['container_id']='d'*64
            else:wrong['events'][1]['proof']['native_files_verified']=False
            with self.subTest(change=change),self.assertRaises(ValueError):expired_proof.failed_startup_interval(wrong,awaiting)

if __name__=='__main__':unittest.main()
