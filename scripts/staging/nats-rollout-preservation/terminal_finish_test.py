"""Terminal finish admission preserves failed transport history and legacy census."""
import contextlib,copy,hashlib,json,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import operator_retirement as r
from controller import Blocked

class Tests(unittest.TestCase):
    def fixture(self,root,stack):
        installed=root/'installed';base=root/'rollout-5e48553f3105'
        for path in (installed/'journal',installed/'responses',base):path.mkdir(parents=True,mode=0o700)
        failed='1aebb57c08747c2cf9877ad613290b814d1c1b54748a37b57ac8c2d5068a55cb'
        complete='ee72562e3db196e02c872f4f55bbdf626abc77777847ec9f43c95ec2c817d019'
        source='b25087c728cdfeb40b9e4f8cb37cb8e441e122fa';binding={'fixture':'bound'}
        req={'action':'finish','claim_rv':'3320018','operation':base.name[8:],'report_run_id':38806221626,'report_source_sha':source,'run_attempt':1}
        result={'operation':req['operation'],'status':'PASS','phase':'POST_RESTART','fence_status':'RELEASED'}
        marker={'metadata':{'uid':'marker','resourceVersion':req['claim_rv']},'data':{'phase':'rollout-applying','knownRolloutOperation':req['operation']}}
        auth={'schema':'nats-rollout-preservation-v1','operation':req['operation'],'marker':{'uid':'marker','generation':'g2','previousGeneration':'g1','dataPVC':'data'},'target':{'bound':True}}
        state={**result,'schema':'nats-rollout-root-v1','code_capture':binding,'target':auth['target'],'authorization':auth,
            'source_authority':{'source_sha':source,'verified':True},
            'execution_authority':{'dispatcher_run_id':req['report_run_id'],'head_sha':source,'run_attempt':1},
            'events':[{'kind':'rollout_target_observed','marker':marker},{'kind':'rollout_verified','marker':{'metadata':{'uid':'marker'},'data':{'phase':'rollout-verified','knownRolloutOperation':req['operation']}}}],
            'preservation':{'verified':True},'restart':{'verified':True},
            'context':{'marker':{'metadata':{'uid':'marker'},'data':{'phase':'active','generation':'g2','previousGeneration':'g1','dataPVC':'data'}}}}
        files={}
        def put(path,value):
            raw=json.dumps(value,sort_keys=True,separators=(',',':')).encode();path.write_bytes(raw);path.chmod(0o600)
            return {'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)}
        for nonce,phase in ((failed,'STARTED'),(complete,'COMPLETE')):
            request={**req,'nonce':nonce};row={'schema':'voice-nats-bridge-request-v1','request':request,'request_sha256':r.canonical(request),'phase':phase}
            if phase=='STARTED':row['prepare_error']='guard_rejected'
            else:row['result']=result
            label='failed' if phase=='STARTED' else 'completed'
            files[label+'_journal']=put(installed/'journal'/(nonce+'.json'),row)
            files[label+'_response']=put(installed/'responses'/(nonce+'.json'),{'status':'BLOCKED','error':'exception_Blocked'} if phase=='STARTED' else result)
            report={'schema':'voice-deployment-report-v1','request_action':'finish','request_nonce':nonce,'operation':req['operation'],
                'run_id':req['report_run_id'],'run_attempt':1,'source_sha':source,'helper_binding':r.canonical(binding),
                'request_status':'BLOCKED' if phase=='STARTED' else 'COMPLETE','operation_status':'WAITING' if phase=='STARTED' else 'PASS',
                'stage':'PAUSED_APPLY' if phase=='STARTED' else 'POST_RESTART','error_code':'unknown' if phase=='STARTED' else 'none',
                'effects':{'apply':'RECORDED','release':'UNKNOWN' if phase=='STARTED' else 'RECORDED','fence':'VERIFIED' if phase=='STARTED' else 'RELEASED','preservation':'PRESENT','restart':'PRESENT'}}
            files[label+'_report']=put(installed/'responses'/(nonce+'.report.json'),report)
        files['checkpoint']=put(base/'checkpoint.json',state);files['authorization']=put(base/'apply-authorization.json',auth)
        receipt={'schema':'voice-terminal-finish-disposition-v1','operation':req['operation'],'failed_nonce':failed,'completed_nonce':complete,
            'code_binding_sha256':r.canonical(binding),'files':files}
        receipt_path=installed/('terminal-finish-disposition-'+failed+'.json');put(receipt_path,receipt)
        stack.enter_context(patch.object(r.guard,'ROOT',root));stack.enter_context(patch.object(r.root_cli,'code_binding',return_value=binding))
        return installed,installed/'journal'/(failed+'.json'),receipt_path,base,put

    def test_real_request_shapes_admit_without_changing_failed_bytes(self):
        from story_prefence_disposition_test import Tests as StoryTests
        with tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
            root=Path(td);installed,path,receipt,base,put=self.fixture(root,stack)
            legacy=StoryTests().legacy_fixture(root,installed,stack)
            before={p:p.read_bytes() for p in root.rglob('*') if p.is_file()}
            self.assertEqual(r.pending_abandonment_names(installed),{legacy.name})
            for member in sorted((installed/'journal').iterdir()):r.pending_request_accepted(installed,member,{'journals':{}})
            self.assertEqual(before,{p:p.read_bytes() for p in before})

    def test_rehashed_semantic_drift_and_missing_receipt_refuse(self):
        for drift in ('claim','source','run','attempt','action','status','fence','code','missing','unknown','report','marker','file','request_hash'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
                installed,path,receipt_path,base,put=self.fixture(Path(td),stack)
                receipt=json.loads(receipt_path.read_bytes())
                if drift in ('claim','source','run','attempt','action'):
                    p=installed/'journal'/(receipt['completed_nonce']+'.json');row=json.loads(p.read_bytes())
                    key,value={'claim':('claim_rv','999'),'source':('report_source_sha','a'*40),'run':('report_run_id',2),'attempt':('run_attempt',2),'action':('action','status')}[drift]
                    row['request'][key]=value;row['request_sha256']=r.canonical(row['request']);receipt['files']['completed_journal']=put(p,row)
                elif drift in ('status','fence','code'):
                    p=base/'checkpoint.json';row=json.loads(p.read_bytes());row[{'status':'status','fence':'fence_status','code':'code_capture'}[drift]]={'status':'BLOCKED','fence':'UNKNOWN','code':{'other':'binding'}}[drift]
                    receipt['files']['checkpoint']=put(p,row)
                elif drift=='report':
                    p=installed/'responses'/(receipt['completed_nonce']+'.report.json');row=json.loads(p.read_bytes());row['effects']['restart']='UNKNOWN';receipt['files']['completed_report']=put(p,row)
                elif drift=='marker':
                    p=base/'checkpoint.json';row=json.loads(p.read_bytes());row['context']['marker']['data']['knownRolloutOperation']=row['operation'];receipt['files']['checkpoint']=put(p,row)
                elif drift=='file':path.write_bytes(path.read_bytes()+b' ')
                elif drift=='request_hash':
                    row=json.loads(path.read_bytes());row['request_sha256']='0'*64;receipt['files']['failed_journal']=put(path,row)
                elif drift=='missing':receipt_path.unlink()
                else:
                    path=installed/'journal'/('f'*64+'.json');put(path,{'phase':'STARTED','request':{}})
                if drift!='missing':put(receipt_path,receipt)
                with self.assertRaises(Blocked):r.pending_request_accepted(installed,path,{'journals':{}})

    def repair_fixture(self,base,receipt_path,put):
        state=json.loads((base/'checkpoint.json').read_bytes());receipt=json.loads(receipt_path.read_bytes())
        descriptor={'kind':'Service','name':'voice-auth','uid':'auth-uid','disposition':'preserve','resourceVersion':'3310867','desired':{'kind':'Service','spec':{'ports':[{'port':80}]}}}
        descriptors=[]
        for name,rv in (('voice-auth','3310867'),('voice-role','3310882'),('voice-voice','3310907')):
            descriptors.append({**copy.deepcopy(descriptor),'name':name,'resourceVersion':rv})
        descriptors.append(copy.deepcopy(descriptors[0]))
        state['nonnats_binding']={'objects':descriptors};state['nonnats']={'checks':[]}
        before=copy.deepcopy(state);before.update(phase='PAUSED_APPLY',status='WAITING',fence_status='VERIFIED');before['events']=before['events'][:1]
        before_raw=json.dumps(before,sort_keys=True,indent=2).encode()+b'\n'
        p=base/'operator-service-rv-before.json';p.write_bytes(before_raw);p.chmod(0o600)
        receipt['files']['repair_before']={'sha256':hashlib.sha256(before_raw).hexdigest(),'bytes':len(before_raw)}
        changes=[{'index':i,'name':obj['name'],'uid':obj['uid'],'before_rv':obj['resourceVersion'],'after_rv':rv,'semantic_sha256':r.canonical(obj['desired'])}
            for i,(obj,rv) in enumerate(zip(descriptors,('3320022','3320032','3320048','3320022')))]
        event={'kind':'operator_preserved_service_rv_reconciled','before_checkpoint_sha256':receipt['files']['repair_before']['sha256'],
            'authorization_sha256':receipt['files']['authorization']['sha256'],'claim_rv':'3320018','changes':changes,
            'reason':'FULL_APPLY_BOOKKEEPING_ONLY_FULL_SERVICE_SEMANTICS_UNCHANGED'}
        repaired=copy.deepcopy(before)
        for change in changes:repaired['nonnats_binding']['objects'][change['index']]['resourceVersion']=change['after_rv']
        repaired['events'].append(event)
        repair={'schema':'voice-exact-5e-service-rv-reconciliation-v1','operation':state['operation'],
            'before_checkpoint_sha256':event['before_checkpoint_sha256'],
            'after_checkpoint_sha256':hashlib.sha256(json.dumps(repaired,sort_keys=True,indent=2).encode()+b'\n').hexdigest(),
            'authorization_sha256':event['authorization_sha256'],'code_binding_sha256':receipt['code_binding_sha256'],
            'claim_rv':'3320018','changes':changes,'workload_writes':False,'deployment_pass':False}
        receipt['files']['repair_receipt']=put(base/'operator-service-rv-reconciliation.json',repair)
        state['nonnats_binding']=repaired['nonnats_binding'];state['events']=repaired['events']+state['events']
        receipt['files']['checkpoint']=put(base/'checkpoint.json',state);put(receipt_path,receipt)

    def test_repair_reconstruction_and_closed_optional_evidence(self):
        for drift in (None,'missing','before','after','binding','event','extra','traversal'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
                installed,path,receipt_path,base,put=self.fixture(Path(td),stack)
                self.repair_fixture(base,receipt_path,put);receipt=json.loads(receipt_path.read_bytes())
                if drift=='missing':del receipt['files']['repair_before'];del receipt['files']['repair_receipt']
                elif drift in ('before','after'):
                    key='repair_before' if drift=='before' else 'repair_receipt';p=base/('operator-service-rv-before.json' if drift=='before' else 'operator-service-rv-reconciliation.json');row=json.loads(p.read_bytes())
                    if drift=='before':row['target']={'changed':True}
                    else:row['after_checkpoint_sha256']='0'*64
                    receipt['files'][key]=put(p,row)
                elif drift in ('binding','event','extra'):
                    p=base/'checkpoint.json';state=json.loads(p.read_bytes())
                    if drift=='binding':state['nonnats_binding']['objects'][0]['resourceVersion']='999'
                    elif drift=='event':state['events'][1]['changes'][0]['uid']='other'
                    else:state['events']=[e for e in state['events'] if e['kind']!='operator_preserved_service_rv_reconciled']
                    receipt['files']['checkpoint']=put(p,state)
                elif drift=='traversal':receipt['files']['../checkpoint.json']=receipt['files']['checkpoint']
                put(receipt_path,receipt)
                if drift is None:self.assertEqual(r.pending_request_accepted(installed,path,{'journals':{}})['disposition'],'SUPERSEDED_FAILED_FINISH')
                else:
                    with self.assertRaises(Blocked):r.pending_request_accepted(installed,path,{'journals':{}})

    def test_internal_idle_transaction_checks_all_journals_and_rechecks_disposition(self):
        import installer
        from types import SimpleNamespace
        from story_prefence_disposition_test import Tests as StoryTests
        for drift in (None,'receipt'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as td,contextlib.ExitStack() as stack:
                root=Path(td);installed,path,receipt,base,put=self.fixture(root,stack)
                legacy=StoryTests().legacy_fixture(root,installed,stack)
                for name in ('inbox','processing','recovery'):(installed/name).mkdir(mode=0o700)
                (installed/'inbox').chmod(0o1730)
                for name in ('recovery-key.pem','recovery-cert.pem'):put(installed/'recovery'/name,{'key':'fixture'})
                put(installed/'policy.json',{})
                history=root/('rollout-'+r.FENCE_ABORT_OPERATION)/'checkpoint.json';put(history,{'operator_fence_abort':{}})
                stack.enter_context(patch.object(r,'idle_state'));stack.enter_context(patch.object(r,'verify_terminal',return_value={'journals':{}}))
                stack.enter_context(patch.object(installer.root_cli,'rollout_directory_kind',return_value='operation'))
                stack.enter_context(patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()))
                marker={'data':{'phase':'active'}}
                stack.enter_context(patch.object(installer,'Kube',return_value=SimpleNamespace(get=lambda *args:marker)))
                stack.enter_context(patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)))
                stack.enter_context(patch('story_witness.registry_snapshot',return_value=None))
                def swap(*args):
                    if drift:receipt.write_bytes(receipt.read_bytes()+b' ')
                upgrade=stack.enter_context(patch.object(installer,'_upgrade_code',side_effect=swap))
                before={p:p.read_bytes() for p in (installed/'journal').iterdir()}
                if drift is None:installer._upgrade_idle_transition(root/'candidate','reviewed-binding','idle-terminal-finish-v1','previous','upgraded')
                else:
                    with self.assertRaises(Blocked):installer._upgrade_idle_transition(root/'candidate','reviewed-binding','idle-terminal-finish-v1','previous','upgraded')
                upgrade.assert_called_once();self.assertEqual(before,{p:p.read_bytes() for p in before})
                self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

if __name__=='__main__':unittest.main()
