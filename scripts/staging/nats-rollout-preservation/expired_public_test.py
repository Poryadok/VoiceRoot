"""Actual closed client/dispatcher and extracted workflow routing; no root admission claim."""
import ast,copy,json,os,tempfile,textwrap,unittest
from pathlib import Path
from unittest.mock import Mock,patch
import bridge,bridge_client,bridge_root

class PublicRecoveryTests(unittest.TestCase):
    def environment(self):
        from resource_claim_test import context
        return {'GITHUB_RUN_ID':'99','GITHUB_RUN_ATTEMPT':'1','GITHUB_JOB':'deploy','GITHUB_SHA':'a'*40,'GITHUB_TOKEN':'synthetic-token',
            'VOICE_TTENREC_CONTEXT':json.dumps(context())}
    def rows(self):
        op='3340764a7d24';nonce='b'*64
        return [(['prepare-expired-native',op,'99'],'_prepare_expired_native'),
            (['prepare-expired-finish',op,'99'],'_prepare_expired_finish'),
            (['authorize-expired-native',op,'11','12',nonce,'99'],'_authorize_expired_native'),
            (['authorize-expired-finish',op,'11','12',nonce,'99'],'_authorize_expired_finish'),
            (['finish-expired-native',op,'123','99'],'_finish_expired_native')]
    def test_real_client_closed_schema_nonce_and_actual_actions_dispatch_replay(self):
        for args,method in self.rows():
            with self.subTest(action=args[0]),tempfile.TemporaryDirectory() as directory:
                env=self.environment();row=bridge_client.request(args,env)
                self.assertEqual(row,bridge_client.request(args,env))
                self.assertNotEqual(row['nonce'],bridge_client.request(args,{**env,'GITHUB_RUN_ATTEMPT':'2'})['nonce'])
                self.assertNotEqual(row['nonce'],bridge_client.request(args,{**env,'GITHUB_SHA':'c'*40,'GITHUB_JOB':'other'})['nonce'])
                actions=object.__new__(bridge_root.Actions)
                actions.state=Mock(return_value=(None,{'target':{'mode':'images-only'},'migrations':[]}))
                for _,name in self.rows():setattr(actions,name,Mock(side_effect=AssertionError('wrong action')))
                expected={'status':'WAITING','operation':row['operation']};getattr(actions,method).side_effect=None
                getattr(actions,method).return_value=expected
                result=bridge.dispatch(Path(directory),row,actions.execute,lambda request:None)
                self.assertEqual(result,expected)
                self.assertEqual(result,bridge.dispatch(Path(directory),row,actions.execute,lambda request:None))
                getattr(actions,method).assert_called_once_with(row)
                for changed in ({**row,'path':'/tmp/arbitrary'}, {key:value for key,value in row.items() if key!='run_id'}, {**row,'run_id':True}):
                    with self.assertRaises(bridge.BridgeError):bridge.validate(changed)
                with self.assertRaises(bridge.BridgeError):bridge_client.request(args,{**env,'GITHUB_RUN_ID':'100'})
                if 'upload_nonce' in row:
                    for changed in ({**row,'upload_nonce':'x'}, {**row,'observation_artifact_id':row['original_artifact_id']}):
                        with self.assertRaises(bridge.BridgeError):bridge.validate(changed)
                    with self.assertRaises(bridge.BridgeError):bridge.dispatch(Path(directory),{**row,'observation_artifact_id':13},actions.execute,lambda request:None)
    def test_extracted_actual_workflow_routes_and_rejects_conflicting_or_changed_target(self):
        path=Path(__file__).parent/'testdata/staging-deploy-current.yml'
        if not path.exists():path=Path(__file__).parents[3]/'.github/workflows/staging-deploy.yml'
        raw=path.read_text();start=raw.index("          if action=='--bridge-prepare':")
        end=raw.index("          sys.argv=[str(code/entry),*arguments]",start)
        body=textwrap.dedent(raw[start:end]);ast.parse(body)
        env={**self.environment(),'DEPLOY_MODE':'images-only','CHANGED_SERVICES':'story','VOICE_IMAGE_TAG':'a'*40,
             'ROLLOUT_CLAIM_RV':'123','ROLLOUT_UPLOAD_NONCE':'b'*64,'ROLLOUT_ORIGINAL_ARTIFACT_ID':'11',
             'ROLLOUT_OBSERVATION_ARTIFACT_ID':'12','ROLLOUT_ARTIFACT_ID':'11'}
        for mode in ('native','finish'):
            values={**env,'ROLLOUT_EXPIRED_'+mode.upper()+'_OPERATION':'3340764a7d24'}
            for action,wanted in (('--bridge-prepare','prepare-expired-'+mode),('--bridge-authorize','authorize-expired-'+mode)):
                namespace={'os':os,'action':action,'op':'3340764a7d24','bridge_action':True}
                with patch.dict(os.environ,values,clear=True):exec(body,namespace)
                row=bridge_client.request(namespace['arguments'],values);self.assertEqual(row['action'],wanted)
            if mode=='native':
                namespace={'os':os,'action':'--bridge-finish','op':'3340764a7d24','bridge_action':True}
                with patch.dict(os.environ,values,clear=True):exec(body,namespace)
                self.assertEqual(bridge_client.request(namespace['arguments'],values)['action'],'finish-expired-native')
            for changes in ({'ROLLOUT_COLD_BACKUP_OPERATION':'3340764a7d24'}, {'DEPLOY_MODE':'full'},
                {'CHANGED_SERVICES':'user'}, {'VOICE_IMAGE_TAG':'c'*40}, {'ROLLOUT_EXPIRED_'+mode.upper()+'_OPERATION':'a'*12}):
                with patch.dict(os.environ,{**values,**changes},clear=True),self.assertRaises(SystemExit):
                    exec(body,{'os':os,'action':'--bridge-prepare','op':'','bridge_action':True})
        for action,wanted in (('--bridge-prepare','prepare'),('--bridge-authorize','authorize'),('--bridge-finish','finish')):
            namespace={'os':os,'action':action,'op':'3340764a7d24','bridge_action':True}
            with patch.dict(os.environ,{key:value for key,value in env.items() if key!='ROLLOUT_UPLOAD_NONCE'},clear=True):exec(body,namespace)
            self.assertEqual(bridge_client.request(namespace['arguments'],env)['action'],wanted)
        self.assertIn("if: inputs.expired_finish_operation == ''",raw)
        self.assertIn('ROLLOUT_ORIGINAL_ARTIFACT_ID',raw);self.assertIn('ROLLOUT_OBSERVATION_ARTIFACT_ID',raw)

    def test_real_active_release_feeds_new_latest_rollback_and_ordinary_finish_cannot_bypass(self):
        import resource_claim
        with tempfile.TemporaryDirectory() as folder:
            actions=object.__new__(bridge_root.Actions)
            actions._installation=resource_claim.production_installation()
            state={'schema':'nats-rollout-root-v1','operation':'3340764a7d24','status':'PASS','input_target':{'mode':'images-only','changed_services':['story']},'target':{'manifest_sha256':'a'*64},
                   'context':{'expected':{'namespace_uid':'ns','source_claim_uid':'pvc'}},
                   'expired_recovery_authority':{'purpose':'native-migration'}}
            actions.state=Mock(return_value=(Path(folder),state));actions._prepare=Mock(return_value={'new':'closed capture'})
            with patch.object(bridge_root,'INSTALLED',Path(folder)),patch('transaction.rollback_package') as package,patch('transaction.rollback_services',return_value=['story']) as services:
                actions._active_release(state)
                request={'action':'prepare-rollback','operation':state['operation'],'nonce':'c'*64,'token':'synthetic','run_id':99}
                result=actions.execute(request)
                self.assertEqual(result,{'new':'closed capture'})
                package.assert_called_once_with(state,story_only=False);services.assert_called_once_with(state,story_only=False)
                args,kwargs=actions._prepare.call_args
                self.assertEqual(args[0]['nonce'],'c'*64);self.assertEqual(args[0]['mode'],'images-only')
                self.assertEqual(args[0]['changed_services'],['story']);self.assertIs(args[1],state)
                row=json.loads((Path(folder)/'active-release.json').read_text());row['operation']='a'*12
                bridge_root.save(Path(folder)/'active-release.json',row)
                with self.assertRaises(bridge_root.Blocked):actions.execute(request)
            with patch('transaction.finish') as finish:
                with self.assertRaises(bridge_root.Blocked):actions.execute({'action':'finish','operation':state['operation'],'claim_rv':'1'})
                finish.assert_not_called()

    def test_real_bundle_inventory_and_manifest_reader_new_bound_and_custody_veto(self):
        import bundle,root_cli,hashlib
        added={'expired_recovery.py','expired_proof.py','expired_admission.py','expired_transport.py','expired_finish.py',
            'closed_archive.py','closed_durable.py','hub_bridge_identity.py','hub_bridge_unit.py','operator_retirement.py','operator_reset.py',
            'gis_first_install.py','gis_prerequisite.py'}
        self.assertTrue(added<=set(bundle.ROLLOUT))
        self.assertIn('nats_active_contract.py',bundle.ROLLOUT)
        self.assertIn('deployment_report.py',bundle.ROLLOUT)
        self.assertIn('nats_deliver_all.py',bundle.ROLLOUT)
        self.assertIn('release_policy_wiring.py',bundle.ROLLOUT)
        self.assertIn('gis_existing_update.py',bundle.ROLLOUT)
        self.assertIn('realtime_bootstrap.py',bundle.ROLLOUT)
        self.assertIn('realtime_bootstrap_existing.py',bundle.ROLLOUT)
        self.assertTrue({'staging_prerequisites.py','staging_p3_prerequisites.py','staging_p3_prerequisites_crypto.py',
            'staging_gis_material.py','staging_release_policy_prepare.py'} <= set(bundle.ROLLOUT))
        self.assertEqual(len(bundle.KNOWN)+len(bundle.ROLLOUT)+5,99)
        self.assertIn('provisional.py',bundle.ROLLOUT)
        with tempfile.TemporaryDirectory() as folder:
            code=Path(folder)/'code';code.mkdir(mode=0o700)
            manifest={}
            for index in range(94):
                name='module'+str(index)+'.py';raw=('pinned source '+str(index)).encode()
                path=code/name;path.write_bytes(raw);path.chmod(0o440);manifest[name]=hashlib.sha256(raw).hexdigest()
            def write():
                path=code/'capture-manifest.json'
                if path.exists():path.chmod(0o600)
                path.write_text(json.dumps(manifest));path.chmod(0o440)
            last=[(name,manifest.pop(name)) for name in ('module86.py','module87.py','module88.py','module89.py','module90.py','module91.py','module92.py','module93.py')]
            write();self.assertEqual(root_cli.code_binding(code),manifest)
            manifest.update(last[:1]);write();self.assertEqual(root_cli.code_binding(code),manifest)
            manifest.update(last[1:3]);write();self.assertEqual(root_cli.code_binding(code),manifest)
            manifest.update(last[3:4]);write()
            with self.assertRaises(root_cli.Blocked):root_cli.code_binding(code)
            del manifest[last[3][0]];write()
            path=code/'module0.py';path.chmod(0o460)
            with self.assertRaises(root_cli.Blocked):root_cli.code_binding(code)
            path.chmod(0o600);path.write_bytes(b'changed bytes');path.chmod(0o440)
            with self.assertRaises(root_cli.Blocked):root_cli.code_binding(code)

    def test_actual_bootstrap_prove_optional_get_census_and_cleanup_composition(self):
        import bootstrap_auth as auth,hashlib
        from dataclasses import replace
        from bootstrap_selection_test import owned_fixture
        from docker_runtime import DockerRuntime, LABEL
        from nats_contract_actor import Actor
        from nats_contract_plan import DECLARATIONS,digest
        from rollout_census_test import stream,consumer,ACCOUNT
        seen_operations=set()
        for mode in ('ordinary','get','denied','census-drift'):
            with self.subTest(mode=mode),tempfile.TemporaryDirectory() as folder:
                configs={};requests=[];runs=[];trees=[0]
                class Runtime:
                    no_operation_containers=DockerRuntime.no_operation_containers
                    def __init__(self,*args):
                        self.base=Path(args[0]);self.operation=args[1];self.owned={'proof':{'id':'d'*64}}
                        operations.append(self.operation)
                    def start_broker(self,*args):return 'proof'
                    def stop(self,*args):pass
                    def inspect(self,*args):return {'Id':'d'*64}
                    def run(self,args):
                        runs.append(args)
                        if args[0]=='rm':os.chown(self.base/'auth-store',0,0) # cap-drop synthetic cleanup transport only
                        return ''
                    def monitor_jsz(self,*args):
                        trees[0]+=1
                        streams=[]
                        for stream_name,declaration in DECLARATIONS:
                            item=consumer(stream_name,declaration['durable_name'])
                            item.update(config=copy.deepcopy(declaration),delivered={'consumer_seq':0,'stream_seq':0},
                                ack_floor={'consumer_seq':0,'stream_seq':0},num_ack_pending=0,num_redelivered=0,num_pending=0,num_waiting=0)
                            row=stream(stream_name,[item])
                            row['config']=copy.deepcopy(configs[stream_name]);row['state'].update(messages=0,bytes=0,first_seq=0,last_seq=0)
                            streams.append(row)
                        if mode=='census-drift' and trees[0]>1:streams[1]['consumer_detail'][0]['config']['metadata']={'changed':'proof'}
                        return {'streams':len(streams),'consumers':sum(len(row['consumer_detail']) for row in streams),'messages':0,'bytes':0,'total':1,
                            'account_details':[{'id':ACCOUNT,'name':'fixture','stream_detail':streams}]}
                class FixtureActor:
                    prove_get_permissions=Actor.prove_get_permissions
                    def __init__(self,*args):pass
                    def assert_closed_isolation(self):pass
                    def _request(self,api,payload,**kwargs):
                        requests.append(api)
                        if api.startswith('$JS.API.STREAM.CREATE.'):configs[payload['name']]=copy.deepcopy(payload);return {}
                        if kwargs.get('permission_probe'):
                            if mode=='denied' and api.endswith('social_events'):raise auth.Blocked('fixture_permission_denied')
                            return None
                        raise AssertionError(api)
                    def mutate(self,api,payload):
                        if api.startswith('$JS.API.STREAM.UPDATE.'):configs[payload['name']]=copy.deepcopy(payload)
                        else:configs['consumer']=copy.deepcopy(payload['config'])
                    def info_config(self,name):return copy.deepcopy(configs['consumer' if '/' in name else name])
                tokens={name:(ACCOUNT if name=='account.public' else 'synthetic-'+name).encode() for name in
                    ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
                operations=[]
                installation=replace(owned_fixture()[3],account_sha256=hashlib.sha256(ACCOUNT.encode()).hexdigest())
                with patch.object(auth.guard,'ROOT',Path(folder)),patch.object(auth,'DockerRuntime',Runtime),patch.object(auth,'Actor',FixtureActor),patch.object(auth,'ready'),patch.object(auth,'secret_bytes',side_effect=lambda operator,key:tokens[key]):
                    if mode in ('denied','census-drift'):
                        with self.assertRaises(auth.Blocked):auth.prove(b'synthetic retained identity',{},lambda:None,['chat_events','absent_events','social_events'],installation=installation)
                    else:
                        result=auth.prove(b'synthetic retained identity',{},lambda:None,None if mode=='ordinary' else ['chat_events','absent_events','social_events'],installation=installation)
                        self.assertEqual('get_permissions' in result,mode=='get')
                        if mode=='get':self.assertEqual(result['get_permissions']['streams'],3)
                    self.assertEqual(len(operations),1)
                    self.assertRegex(operations[0],r'^auth[0-9a-f]{12}$')
                    self.assertNotIn(operations[0],seen_operations);seen_operations.add(operations[0])
                    self.assertEqual(runs[-3:], [['rm','-f','d'*64],['ps','-a','--filter','id='+'d'*64,'--format','{{.ID}}'],
                        ['ps','-a','--filter','label='+LABEL+'='+operations[0],'--format','{{.ID}}']])
                    self.assertEqual(len(list(Path(folder).iterdir())),1 if mode in ('denied','census-drift') else 0)
                    get=[api for api in requests if api.startswith('$JS.API.STREAM.MSG.GET.')]
                    self.assertEqual(len(get),0 if mode=='ordinary' else 3)
