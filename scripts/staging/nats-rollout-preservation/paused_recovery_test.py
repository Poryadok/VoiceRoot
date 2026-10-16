import copy
import unittest
import hashlib
import stat
import tempfile
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch,Mock
import paused_recovery as module
from controller import Blocked

class ExactBoundaryTests(unittest.TestCase):
    def test_retained_source_exact_git_bytes_and_inventory(self):
        with tempfile.TemporaryDirectory() as directory:
            workspace=Path(directory);(workspace/'docs').mkdir();file=workspace/'docs'/'proof.txt';file.write_bytes(b'captured')
            tree={'truncated':False,'tree':[{'path':'docs','mode':'040000','type':'tree'},
                {'path':'docs/proof.txt','mode':'100644','type':'blob','size':8,
                 'sha':hashlib.sha1(b'blob 8\0captured').hexdigest()}]}
            # The root reader and ancestor custody are separate physical tests;
            # this fixture exercises the actual canonical byte/inventory logic.
            with patch('encrypted_cut.directory'),patch.object(module,'private_bytes',side_effect=lambda p,limit:Path(p).read_bytes()):
                self.assertEqual(module.retained_source_files(workspace,tree),{'docs/proof.txt':hashlib.sha256(b'captured').hexdigest()})
                file.write_bytes(b'mutated!')
                with self.assertRaises(Blocked):module.retained_source_files(workspace,tree)
                file.write_bytes(b'captured');unexpected=workspace/'unexpected';unexpected.write_bytes(b'x')
                with self.assertRaises(Blocked):module.retained_source_files(workspace,tree)
                unexpected.unlink();file.unlink()
                with self.assertRaises(Blocked):module.retained_source_files(workspace,tree)
                file.symlink_to('/tmp/other-source')
                with self.assertRaises(Blocked):module.retained_source_files(workspace,tree)
            for path in ('../escape','/absolute','docs/../escape'):
                bad=copy.deepcopy(tree);bad['tree'][1]['path']=path
                with patch('encrypted_cut.directory'),self.assertRaises(ValueError):module.retained_source_files(workspace,bad)

    def test_historical_source_route_requires_exact_adopted_operation_before_network(self):
        for state in ({'operation':'other','repair_adoption':'hash'}, {'operation':module.OPERATION}):
            with patch('source_authority._headers') as headers,self.assertRaises(Blocked):
                module.reconstruct_source('token','/base',state,{})
            headers.assert_not_called()

    def test_private_and_public_readers_preserve_separate_mode_policy(self):
        def row(mode,uid=0):
            return SimpleNamespace(st_mode=stat.S_IFREG|mode,st_uid=uid,st_nlink=1,
                st_size=1,st_dev=1,st_ino=2,st_mtime_ns=3,st_ctime_ns=4)
        for mode,private,accepted in ((0o600,True,True),(0o440,False,True),
                (0o440,True,False),(0o660,False,False),(0o604,True,False)):
            with self.subTest(mode=oct(mode),private=private),patch.object(module.os,'open',return_value=9),\
                 patch.object(module.os,'fstat',return_value=row(mode)),\
                 patch.object(module.os,'read',return_value=b'x'),patch.object(module.os,'close') as close:
                if accepted:self.assertEqual(module.owned_bytes('/fixed',private=private),b'x')
                else:
                    with self.assertRaises(Blocked):module.owned_bytes('/fixed',private=private)
                close.assert_called_once_with(9)
        with patch.object(module.os,'open',return_value=9),patch.object(module.os,'fstat',return_value=row(0o440,1000)),patch.object(module.os,'close'):
            with self.assertRaises(Blocked):module.owned_bytes('/fixed',private=False)

    def test_final_journal_compares_preserved_bytes_not_two_current_reads(self):
        record={'original_journal_sha256':hashlib.sha256(b'original').hexdigest()}
        with patch.object(module,'private_bytes',return_value=b'original'):
            module.verify_original_journal('/fixed',record)
        with patch.object(module,'private_bytes',return_value=b'changed'):
            with self.assertRaisesRegex(Blocked,'paused_recovery_journal_changed'):
                module.verify_original_journal('/fixed',record)

    def test_ingress_closed_through_copy_and_restored_after_rejection(self):
        with tempfile.TemporaryDirectory() as directory:
            installed=Path(directory)
            for name in ('inbox','processing','journal','responses','recovery','sources'):
                (installed/name).mkdir(mode=0o700)
            def lstat(path):
                return SimpleNamespace(st_mode=stat.S_IFDIR|0o700,st_uid=0)
            def body(*args):
                self.assertEqual(stat.S_IMODE((installed/'inbox').stat().st_mode),0o700)
                raise Blocked('copy_rejected')
            with patch.object(Path,'lstat',lstat),patch.object(module,'_install_repair_closed',side_effect=body):
                with self.assertRaisesRegex(Blocked,'copy_rejected'):module.install_repair('/code',installed,None)
            self.assertEqual(stat.S_IMODE((installed/'inbox').stat().st_mode),0o1730)

    def fixture(self):
        state={'operation':module.OPERATION,'phase':'COLD_BACKUP','status':'BLOCKED','fence_status':'VERIFIED',
            'error':'rollout_selected_store_custody_invalid','code_capture':{'fixture':'a'*64},
            'target':{'tag':module.SOURCE,'mode':'images-only','changed_services':['story']},
            'input_target':{'exact':'compiler input'},'service_actor_authority':{'root':'captured authority'},
            'service_actor_services':['story'],'context':{'expected':{'marker_uid':module.MARKER_UID,
                'source_claim_uid':module.CLAIM_UID,'source_pv_uid':module.PV_UID}}}
        journal={'phase':'STARTED','request':{'action':'prepare','nonce':module.NONCE,'source_sha':module.SOURCE,
            'run_id':module.RUN_ID,'mode':'images-only','changed_services':['story']}}
        return state,journal

    def test_exact_boundary_and_each_state_drift(self):
        state,journal=self.fixture()
        with patch.object(module,'V6_BINDING',module.digest(state['code_capture'])):
            self.assertEqual(module.initial_checkpoint(state,journal)['operation'],module.OPERATION)
            for key,value in (('operation','other'),('phase','FENCE'),('status','RUNNING'),('fence_status','UNKNOWN'),
                              ('error','other'),('cut',{}),('cipher_binding',{}),('repair_adoption',{})):
                bad=copy.deepcopy(state);bad[key]=value
                with self.subTest(key=key),self.assertRaises(Blocked):module.initial_checkpoint(bad,journal)
            for key in ('marker_uid','source_claim_uid','source_pv_uid'):
                bad=copy.deepcopy(state);bad['context']['expected'][key]='other'
                with self.subTest(key=key),self.assertRaises(Blocked):module.initial_checkpoint(bad,journal)
            for key,value in (('phase','COMPLETE'),):
                bad=copy.deepcopy(journal);bad[key]=value
                with self.assertRaises(Blocked):module.initial_checkpoint(state,bad)
            for key in ('nonce','run_id','source_sha'):
                bad=copy.deepcopy(journal);bad['request'][key]='other'
                with self.subTest(key=key),self.assertRaises(Blocked):module.initial_checkpoint(state,bad)

    def test_original_authority_remains_bound_after_phase_progress(self):
        original,_=self.fixture();state=copy.deepcopy(original);state['phase']='AWAITING_OFF_NODE'
        self.assertEqual(module.original_target_unchanged(state,original),original['code_capture'])
        for key in ('input_target','target','service_actor_authority','service_actor_services'):
            bad=copy.deepcopy(state);bad[key]={}
            with self.subTest(key=key),self.assertRaises(Blocked):module.original_target_unchanged(bad,original)

    def test_any_owned_producer_output_vetoes_recapture(self):
        with tempfile.TemporaryDirectory() as directory:
            base=Path(directory);module.backup_outputs_absent(base,{})
            for name in ('rollout-before.tar','post-apply-123','space-backup.json','space-restore',
                         'out-1','recompile-interrupted','copy-checkpoint.json','rollout-backup.cms','export','apply-authorization.json'):
                path=base/name;path.write_bytes(b'partial')
                with self.subTest(name=name),self.assertRaises(Blocked):module.backup_outputs_absent(base,{})
                path.unlink()

if __name__=='__main__':unittest.main()

class ContinuationTests(unittest.TestCase):
    def fixture(self):return ExactBoundaryTests.fixture(self)

    def test_actual_compiler_and_normalization_reconstruct_retained_story_target(self):
        import json
        import compiler
        import guard
        import normalize
        import root_main
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);workspace=root/'installed'/'sources'/module.OPERATION;workspace.mkdir(parents=True)
            build=workspace.parent/(module.OPERATION+'-build');build.mkdir();base=root/'operation';base.mkdir()
            image='example.invalid/story@sha256:'+'a'*64
            live={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-story','namespace':guard.NS,
                'uid':'0b53e628-0c09-425f-be37-7b67e8628ba4'},'spec':{'replicas':1,'template':{'spec':{
                'containers':[{'name':'story','image':'example.invalid/story@sha256:'+'b'*64}]}}}}
            for index,relative in enumerate(compiler.SOURCE_MANIFESTS):
                path=workspace/relative;path.parent.mkdir(parents=True,exist_ok=True)
                row=copy.deepcopy(live) if index==0 else {'kind':'Service','metadata':{'name':'voice-test-'+str(index),'namespace':guard.NS}}
                path.write_text(json.dumps(row))
            acl=workspace/'deploy/nats/acl-intent.yaml';acl.parent.mkdir(parents=True);acl.write_text('source-bound fixture')
            contract={'scripts':[]}
            for part in ('realtime','notification','search','analytics-chat'):
                script='echo fixture\n';path=workspace/('deploy/templates/nats-'+part+'-bootstrap.yaml')
                path.write_text(json.dumps({'kind':'ConfigMap','metadata':{'name':'voice-nats-'+part+'-bootstrap'},'data':{'bootstrap.sh':script}}))
                contract['scripts'].append({'part':part,'sha256':hashlib.sha256(script.encode()).hexdigest()})
            parameters={'registry':'example.invalid','tag':module.SOURCE,'mode':'images-only','changed_services':['story'],
                'images':{'voice-story/story':image},'contract':contract,'enrolled':['voice-story'],
                'generation':'r20260930a4','dataPVC':'voice-nats-jsdata-d202610040049430b','s3_signing_endpoint':'https://storage.example.invalid'}
            root_main.save(build/'parameters.json',parameters);output=build/('target-'+module.SOURCE+'.json')
            # JSON stands in for external YAML decoding; producer auxiliary
            # planning is isolated. Compiler/render/bootstrap and both target
            # transformations below execute their production implementations.
            import source_plan
            # This fixture covers rendering/normalization only. The real owned
            # producer and canonical source plan are covered by paused_recompile_plan_test.
            with patch.object(module,'PausedOriginalProducer',return_value=SimpleNamespace(verify=lambda:None)),\
                 patch.object(compiler,'decode_yaml',side_effect=json.loads),patch.object(source_plan,'compile_plan',return_value={'checks':[],'actions':[]}):
                compiler.main([str(workspace),str(build/'parameters.json'),str(output)])
                compiled=json.loads(output.read_bytes())
                class Kube:
                    def get(self,kind,name):return copy.deepcopy(live)
                    def run(self,args,body):return copy.deepcopy(body)
                stage=SimpleNamespace(kube=Kube(),expected={'source_claim':parameters['dataPVC']},
                    snapshots={'voice-story':live},original_snapshots={'voice-story':live},verify_final_storage=lambda:None)
                rows,target=normalize.image_only_documents(stage,compiled['manifests'],compiled['target'])
                root_main.save(base/'apply-manifests.json',rows)
                target['manifest_sha256']=hashlib.sha256((base/'apply-manifests.json').read_bytes()).hexdigest()
                target=normalize.normalize_target(stage.kube,stage,rows,target)
                state={'input_target':compiled['target'],'target':target,'contract':contract,'migrations':compiled['migrations'],
                    'nonnats':compiled['nonnats'],'context':{'expected':{'generation':parameters['generation'],'source_claim':parameters['dataPVC']}}}
                with patch.object(guard,'ROOT',root),patch.object(module,'private_bytes',side_effect=lambda p,*a:Path(p).read_bytes()),\
                     patch.object(module,'owned_bytes',side_effect=lambda p,*a,**kw:Path(p).read_bytes()):
                    proof=module.recompile_target(base,state,stage,{'images':{'story':image}})
                    self.assertEqual(proof['normalized_sha256'],module.digest(target))
                    changed=copy.deepcopy(state);changed['input_target']['images']['voice-story/story']='unapproved'
                    with self.assertRaises(Blocked):module.recompile_target(base,changed,stage,{'images':{'story':image}})
                    acl.write_text('changed canonical source')
                    with self.assertRaisesRegex(Blocked,'compiled_target_changed'):module.recompile_target(base,state,stage,{'images':{'story':image}})

    def test_target_reconstruction_matches_actual_root_save_representation(self):
        import json
        import root_main
        import compiler
        import normalize
        import guard
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);workspace=root/'installed'/'sources'/module.OPERATION;workspace.mkdir(parents=True)
            build=workspace.parent/(module.OPERATION+'-build');build.mkdir();base=root/'operation';base.mkdir()
            rows=[{'kind':'Deployment','spec':{'replicas':0,'template':{'captured':'same'}}}]
            raw=json.dumps(rows,sort_keys=True,indent=2).encode()+b'\n';sha=hashlib.sha256(raw).hexdigest()
            parameters={'tag':module.SOURCE,'mode':'images-only','changed_services':['story'],'contract':{},
                        'generation':'same','dataPVC':'selected','images':{'voice-story/story':'exact-approved-image'}}
            compiled={'target':{'original':'compiler'},'contract':{},'migrations':[],'nonnats':{},'manifests':rows}
            state={'input_target':compiled['target'],'contract':{},'migrations':[],'nonnats':{},
                'target':{'manifest_sha256':sha},'context':{'expected':{'generation':'same','source_claim':'selected'}}}
            root_main.save(build/'parameters.json',parameters);root_main.save(build/('target-'+module.SOURCE+'.json'),compiled)
            root_main.save(base/'apply-manifests.json',rows)
            self.assertEqual((base/'apply-manifests.json').read_bytes(),raw)
            stage=Mock();stage.original_snapshots={};stage.kube=Mock()
            def compile_again(args,*,producer=None):
                self.assertIsNotNone(producer)
                root_main.save(Path(args[2]),compiled)
            def normalize_again(kube,actual,documents,target):
                self.assertEqual(target['manifest_sha256'],sha)
                return target
            with patch.object(guard,'ROOT',root),patch.object(module,'private_bytes',side_effect=lambda p,*a:Path(p).read_bytes()),\
                 patch.object(module,'owned_bytes',side_effect=lambda p,*a,**kw:Path(p).read_bytes()),\
                 patch.object(module,'PausedOriginalProducer',return_value=SimpleNamespace(verify=lambda:None)),\
                 patch.object(compiler,'main',side_effect=compile_again),\
                 patch.object(normalize,'image_only_documents',side_effect=lambda *args:(copy.deepcopy(rows),{'manifest_sha256':module.digest(rows)})),\
                 patch.object(normalize,'normalize_target',side_effect=normalize_again):
                result=module.recompile_target(base,state,stage,{'images':{'story':'exact-approved-image'}})
                self.assertEqual(result['apply_sha256'],sha)
                for changed in (json.dumps(rows,sort_keys=True,separators=(',',':')).encode(),raw+b'\n',raw.replace(b'captured',b'changed')):
                    (base/'apply-manifests.json').write_bytes(changed)
                    with self.subTest(bytes=len(changed)),self.assertRaises(Blocked):module.recompile_target(base,state,stage,{'images':{'story':'exact-approved-image'}})
                (base/'apply-manifests.json').write_bytes(raw)
                state['target']['manifest_sha256']=module.digest(rows)
                with self.assertRaises(Blocked):module.recompile_target(base,state,stage,{'images':{'story':'exact-approved-image'}})

    def run_capture(self,failure=None):
        import transaction
        import json
        state,journal=self.fixture()
        state.update(repair_adoption='approved',kernel_sha256='b'*64,contract={'scripts':[]},migrations=[],migration_secret_metadata={},
            nonnats={'checks':[]},nonnats_binding={},provenance={},bootstrap_enrollment={},
            nats_contract={'plan':'original'},nats_contract_binding={'binding':'original'},nats_target_scripts={'scripts':'original'})
        current={'plan':state['nats_contract'],'binding':state['nats_contract_binding'],'scripts':state['nats_target_scripts']}
        if failure in current:current[failure]={'unapproved':'drift'}
        with tempfile.TemporaryDirectory() as directory:
            base=Path(directory);stage=Mock();stage.final_path=base/'store';runtime=Mock()
            original=copy.deepcopy(state)
            def capture(actual,source,fence,validate):
                self.assertIs(actual,runtime);self.assertNotIn('cut',state)
                validate(actual,'owned-broker',{'exact':'snapshot'},{'census':'actual'},{'archive':'actual'})
                return {'manifest':{'archive_sha256':'a'*64}}
            def reject(*args,**kwargs):raise Blocked('fixture_authority_drift')
            with patch.object(module,'verify_adopted_binding'),patch.object(module,'initial_checkpoint'),\
                 patch.object(module,'private_bytes',side_effect=lambda path,*args:json.dumps(original if Path(path).name=='recovery-original-checkpoint.json' else journal).encode()),\
                 patch.object(module,'backup_outputs_absent'),patch.object(module,'recompile_target',return_value={'verified':'exact-target'},side_effect=reject if failure=='apply-target' else None),\
                 patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value=original['context']),\
                 patch.object(transaction,'save') as save,patch.object(transaction,'file_sha',return_value='b'*64),\
                 patch('docker_runtime.DockerRuntime',return_value=runtime),patch('root_main.revalidate_inputs',side_effect=reject if failure=='inputs' else None),\
                 patch('actor_root.revalidate',side_effect=reject if failure=='actor-credential' else None) as actor,patch('migrations.preflight',return_value={'changed':'PG'} if failure=='PG' else {}),\
                 patch('nonnats_runtime.verify',side_effect=reject if failure=='nonnats' else None),patch('preserve.capture_cut',side_effect=capture),\
                 patch('nats_root_plan.closed_copy_preflight',return_value=current):
                if failure:
                    with self.assertRaises(Blocked):module.resume_capture(None,base,state,base,{}, {'source_sha':module.SOURCE},{'dispatcher_run_id':99},verify_execution=reject if failure=='execution' else lambda:None)
                    self.assertNotIn('cut',state)
                    self.assertNotIn('rollout-before-manifest.json',[call.args[0].name for call in save.call_args_list])
                    self.assertNotIn('cipher_binding',state)
                    return
                result=module.resume_capture(None,base,state,base,{}, {'source_sha':module.SOURCE},{'dispatcher_run_id':99},verify_execution=lambda:None)
                self.assertEqual(result['phase'],'AWAITING_OFF_NODE');self.assertEqual(result['status'],'WAITING')
                self.assertEqual(result['cut']['manifest_sha256'],'b'*64)
                self.assertGreaterEqual(actor.call_count,3);runtime.no_operation_containers.assert_called_with(running_only=False)
                saved=[call.args[0].name for call in save.call_args_list]
                self.assertIn('rollout-before-manifest.json',saved)

    def test_capture_requires_original_closed_plan_and_fresh_authority_before_cut(self):
        self.run_capture()

    def test_each_original_plan_and_fresh_authority_drift_blocks_cut_acceptance(self):
        for failure in ('plan','binding','scripts','apply-target','inputs','actor-credential','PG','nonnats','execution'):
            with self.subTest(failure=failure):self.run_capture(failure)

    def test_bridge_fixed_route_and_completed_cipher_recovery_never_replay_capture(self):
        import bridge
        import bridge_root
        action=object.__new__(bridge_root.Actions);action.code=Path('/approved-code');action.binding={'nats-rollout-preservation/paused_recovery.py':'a'*64}
        request={'action':'resume-cold-backup','operation':module.OPERATION,'nonce':'b'*64,'run_id':99,'token':'synthetic-token'}
        self.assertEqual(bridge.validate(request),request)
        for nonce in (module.NONCE,module.OPERATION+'b'*52):
            with patch.object(action,'state') as state,self.assertRaises(Blocked):
                action.execute(dict(request,nonce=nonce))
            state.assert_not_called()
        interrupted={'phase':'COLD_BACKUP','status':'BLOCKED','operation':module.OPERATION}
        with patch.object(action,'state',return_value=(Path('/base'),interrupted)),patch.object(action,'execute') as execute:
            self.assertIsNone(action.recover(request));execute.assert_not_called()
        completed={'phase':'AWAITING_OFF_NODE','operation':module.OPERATION,
            'execution_authority':{'nonce':request['nonce'],'dispatcher_run_id':99,'head_sha':'c'*40},
            'cipher_binding':{'challenge':'fixed','operation':module.OPERATION,'run_id':99,'head_sha':'c'*40}}
        with patch.object(action,'state',return_value=(Path('/base'),completed)),patch.object(action,'copy_result',return_value={'cipher':'same'}) as result,patch.object(action,'execute') as execute:
            self.assertEqual(action.recover(request),{'cipher':'same'});execute.assert_not_called();result.assert_called_once()
        for section,key,value in (('execution_authority','nonce','d'*64),('execution_authority','dispatcher_run_id',100),
                ('cipher_binding','operation','other'),('cipher_binding','run_id',100),('cipher_binding','head_sha','e'*40)):
            bad=copy.deepcopy(completed);bad[section][key]=value
            with self.subTest(section=section,key=key),patch.object(action,'state',return_value=(Path('/base'),bad)),\
                 patch.object(action,'copy_result') as result,patch.object(action,'execute') as execute,self.assertRaises(Blocked):
                action.recover(request)
            result.assert_not_called();execute.assert_not_called()
