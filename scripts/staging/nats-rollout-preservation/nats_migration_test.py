import copy,tempfile,unittest,tarfile,io,json,hashlib,os
from pathlib import Path
from unittest.mock import Mock,patch
import nats_migration as module
import nats_contract_plan_test as plan_tests
from nats_contract_plan_test import Actor
from nats_contract_plan import digest
from controller import Blocked

class SelectedStoreTests(unittest.TestCase):
    def binding(self,name='voice-nats-jsdata-r20260930a4'):
        uid='11111111-2222-3333-4444-555555555555';pvuid='66666666-7777-8888-9999-aaaaaaaaaaaa'
        path=Path('/var/lib/rancher/k3s/storage/pvc-'+uid+'_voice-staging_'+name)
        claim={'metadata':{'uid':uid,'name':name}}
        pv={'metadata':{'uid':pvuid},'spec':{'claimRef':{'uid':uid},'local':{'path':str(path)},
            'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
        return path,claim,pv
    def test_external_legacy_selected_store_is_bound_before_actual_mount_validation(self):
        path,claim,pv=self.binding();calls=[]
        def run(args,**kwargs):
            calls.append(args)
            if args[:2]==['image','inspect']:return json.dumps([{'Id':'sha256:fixture','RepoDigests':[module.NATS_IMAGE]}])
            if args[0]=='create':return 'a'*64
            return ''
        with tempfile.TemporaryDirectory() as td:
            runtime=module.DockerRuntime(Path(td),'123456abcdef',run);verify=Mock()
            real_resolve=Path.resolve
            def resolved(p,*args,**kwargs):return p if p==path else real_resolve(p,*args,**kwargs)
            with patch.object(Path,'resolve',resolved),patch.object(Path,'lstat',return_value=type('Stat',(),{'st_mode':0o40700,'st_uid':65532})()),patch.object(runtime,'inspect',return_value={}):
                runtime.allow_bound_store(path,claim,pv,verify)
                runtime.create('migration',module.NATS_IMAGE,[(path,'/data',True)],['/usr/local/bin/nats-server'])
            verify.assert_called_once();self.assertIn(str(path),runtime.new_stores)
            self.assertTrue(any(args[0]=='create' and 'type=bind,source='+str(path)+',target=/data' in args for args in calls))
    def test_unbound_external_path_or_fence_failure_never_enrolls_mount(self):
        path,claim,pv=self.binding()
        with tempfile.TemporaryDirectory() as td:
            for wrong in ('path','claim','fence'):
                runtime=module.DockerRuntime(Path(td),'123456abcdef',Mock())
                selected=path if wrong!='path' else path.with_name(path.name+'-other')
                expected=copy.deepcopy(claim)
                if wrong=='claim':expected['metadata']['uid']='aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee'
                verify=Mock(side_effect=Blocked('fixture_fence_open') if wrong=='fence' else None)
                with patch.object(Path,'resolve',lambda p,*a,**k:p),patch.object(Path,'lstat',return_value=type('Stat',(),{'st_mode':0o40700,'st_uid':65532})()):
                    with self.assertRaises(Blocked):runtime.allow_bound_store(selected,expected,pv,verify)
                self.assertEqual(runtime.new_stores,set());runtime.run.assert_not_called()

class Tests(unittest.TestCase):
    def test_issued_update_without_post_info_retains_exact_prior_attempt_evidence(self):
        state,*_=self.fixture();action=state['nats_contract']['actions'][0];obj=action['object']
        attempt={'container_id':'prior-owned-broker','server_image':module.NATS_IMAGE,'started_at':'2026-10-06T12:00:00+00:00'}
        state['events']=[{'kind':'nats_contract_broker_opened',**attempt},
            {'kind':'nats_contract_mutation_issued','object':obj,'api':action['api'],'target_sha256':digest(action['after']),'migration_attempt':attempt['container_id']},
            {'kind':'nats_contract_broker_closed',**attempt,'finished_at':'2026-10-06T12:00:02+00:00'}]
        raw=json.dumps({'Created':'2026-10-06T12:00:01Z'}).encode();name='jetstream/account/streams/'+obj+'/meta.inf'
        manifest={'files':[{'path':name,'size':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}]}
        with tempfile.TemporaryDirectory() as td:
            archive=Path(td)/'current.tar'
            with tarfile.open(archive,'w') as tar:
                member=tarfile.TarInfo(name);member.size=len(raw);tar.addfile(member,io.BytesIO(raw))
            os.chmod(archive,0o600);real_fstat=module.os.fstat
            def custody(fd):
                result=real_fstat(fd);return type('Stat',(),{'st_mode':result.st_mode,'st_uid':0,'st_nlink':result.st_nlink})()
            with patch.object(module.os,'fstat',side_effect=custody):result=module.stream_updates(state,archive,manifest)
        self.assertEqual(result[obj]['target_config_sha256'],digest(action['after']))
        self.assertEqual(result[obj]['native_created'],'2026-10-06T12:00:01Z')
        self.assertEqual(result[obj]['attempt']['container_id'],'prior-owned-broker')
    def fixture(self):
        plan,before,after=plan_tests.CensusTests().proof();actor=Actor(plan)
        manifest={'files':[{'path':'jetstream/account/streams/social_events/msgs/1.blk','size':4,'sha256':'a'*64},
            {'path':'jetstream/account/streams/social_events/obs/old/o.dat','size':9,'sha256':'d'*64}]}
        enrollment={'trusted':'root receipt'}
        state={'operation':'123456abcdef','custody':{'verified':True},'events':[],'nats_contract':plan,'bootstrap_enrollment':enrollment,
            'nats_contract_binding':{'plan_sha256':digest(plan),'server_image':module.NATS_IMAGE,'enrollment_sha256':digest(enrollment),'sources':plan['sources']},
            'cut':{'manifest':manifest,'manifest_sha256':'b'*64,'census':before,'census_sha256':module.canonical(before)}}
        runtime=Mock();runtime.owned={'broker':{'id':'owned-id'}};runtime.start_broker.return_value='broker';runtime.run.return_value=''
        stage=Mock();stage.final_path=Path('/selected-same-store')
        def census(*args):
            row=copy.deepcopy(before)
            for action in plan['actions']:
                config=actor.current[action['object']]
                if config is None:continue
                if '.STREAM.UPDATE.' in action['api']:
                    next(s for s in row['streams'] if s['name']==action['object'])['config_sha256']=digest(config)
                else:
                    _, _, _, _, stream, durable = action['api'].split('.')
                    next(s for s in row['streams'] if s['name']==stream)['state']['consumer_count']+=1
                    consumer=next(c for c in after['consumers'] if c['stream']==stream and c['name']==durable)
                    row['consumers'].append(copy.deepcopy(consumer))
            return row
        return state,runtime,stage,actor,manifest,census
    def run_with(self,base,state,runtime,stage,actor,manifest,census,old_proof):
        with patch.object(module,'DockerRuntime',return_value=runtime),patch.object(module,'Actor',return_value=actor),\
             patch.object(module,'ready'),patch.object(module,'verify_post_apply',old_proof),\
             patch.object(module,'archive_closed_store',return_value=manifest),patch.object(module,'isolated_census',side_effect=census),\
             patch.object(module,'stream_updates',return_value={}),\
             patch.object(module,'file_sha',return_value='c'*64):
            return module.apply_contract(base,state,stage,lambda event:state['events'].append(copy.deepcopy(event)))
    def test_interrupted_update_retry_reproves_old_records_and_exact_partial_delta(self):
        state,runtime,stage,actor,manifest,census=self.fixture();old=Mock(return_value={'verified':True});actor.crash=True
        with tempfile.TemporaryDirectory() as td:
            with self.assertRaisesRegex(RuntimeError,'process_interrupted'):
                self.run_with(Path(td),state,runtime,stage,actor,manifest,census,old)
            self.assertEqual(old.call_count,1)
            result=self.run_with(Path(td),state,runtime,stage,actor,manifest,census,old)
        expected_actions=[
            '$JS.API.STREAM.UPDATE.chat_events',
            '$JS.API.STREAM.UPDATE.social_events',
            '$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed',
            '$JS.API.CONSUMER.CREATE.chat_events.voice_space_media_chat',
            '$JS.API.CONSUMER.CREATE.role_events.voice_space_media_role',
        ]
        self.assertEqual([action['api'] for action in state['nats_contract']['actions']],expected_actions)
        self.assertEqual(old.call_count,1);self.assertTrue(result['verified']);self.assertEqual(actor.calls,expected_actions)
        self.assertEqual(result['proof']['old_consumers_preserved'],1)
        self.assertEqual(result['old_consumer_state_files_verified'],1)

    def test_retry_rejects_old_native_ack_state_change_before_broker_write(self):
        state,runtime,stage,actor,manifest,census=self.fixture()
        state['events'].append({'kind':'nats_contract_old_cut_verified','plan_sha256':digest(state['nats_contract']),
            'old_manifest_sha256':state['cut']['manifest_sha256'],'old_census_sha256':state['cut']['census_sha256']})
        changed=copy.deepcopy(manifest);changed['files'][1]['sha256']='0'*64
        with tempfile.TemporaryDirectory() as td,self.assertRaisesRegex(Blocked,'consumer_state_bytes_changed'):
            self.run_with(Path(td),state,runtime,stage,actor,changed,census,Mock())
        runtime.start_broker.assert_not_called();self.assertEqual(actor.calls,[])
    def test_missing_custody_or_changed_plan_cannot_mount_store(self):
        for changed in ('custody','binding'):
            state,runtime,stage,actor,manifest,census=self.fixture()
            if changed=='custody':state['custody']['verified']=False
            else:state['nats_contract_binding']['plan_sha256']='0'*64
            with tempfile.TemporaryDirectory() as td,self.assertRaises(Blocked):
                self.run_with(Path(td),state,runtime,stage,actor,manifest,census,Mock())
            runtime.start_broker.assert_not_called()
if __name__=='__main__':unittest.main()
