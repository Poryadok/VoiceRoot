import copy
import unittest
import operator_retirement as retirement
from controller import Blocked

class RetirementTransitionTests(unittest.TestCase):
    def snapshot(self):
        return {'schema':'nats-rollout-root-v1','operation':retirement.OPERATION,
            'phase':'PAUSED_APPLY','status':'WAITING','fence_status':'VERIFIED',
            'events':[{'kind':'historical','index':i} for i in range(83)],
            'cut':{'old':'immutable'},'authorization':{'expires_at':'expired'},
            'nats_migration':{'verified':True},'context':{'original_snapshots':{}}}

    def test_specific_transition_preserves_every_historical_field_and_prefix(self):
        before=self.snapshot();events=[{'kind':'operator_retirement_intent'},
            {'kind':'operator_retirement_completed'}]
        result=retirement.terminal_state(before,events,'a'*64)
        self.assertEqual(result['status'],'ABORTED')
        self.assertEqual(result['phase'],'OPERATOR_RETIRED')
        self.assertEqual(result['fence_status'],'RELEASED')
        self.assertEqual(result['events'][:83],before['events'])
        self.assertEqual(before,self.snapshot())
        for key in set(before)-{'phase','status','fence_status','events'}:
            self.assertEqual(result[key],before[key])
        for field in ('cut','authorization','context'):
            changed=copy.deepcopy(result);changed[field]={'changed':True}
            with self.subTest(field=field),self.assertRaises(Blocked):
                retirement.validate_terminal(before,changed,events,'a'*64)

    def test_raw_aborted_and_malformed_completion_never_become_idle(self):
        before=self.snapshot()
        for events in ([],[{'kind':'operator_retirement_completed'}],
            [{'kind':'operator_retirement_intent'},{'kind':'fake_pass'}]):
            with self.subTest(events=events),self.assertRaises(Blocked):
                retirement.terminal_state(before,events,'a'*64)
        with self.assertRaises((Blocked,FileNotFoundError)):
            retirement.verify_terminal('/nonexistent',{'status':'ABORTED'})


class PrecutClosureTests(unittest.TestCase):
    def fixture(self):
        operation='d802f4190c05';names=retirement.CORE|{'voice-web','voice-admin','voice-developer-portal'}
        originals={name:{'metadata':{'uid':str(uuid.uuid5(uuid.NAMESPACE_DNS,name))},
            'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'main','image':'fixed'}]}}}} for name in names}
        marker={'metadata':{'uid':str(uuid.uuid4()),'resourceVersion':'123'},
            'data':{'phase':'rollout-capturing','knownRolloutOperation':operation,'dataPVC':'selected'}}
        snapshot={'schema':'nats-rollout-root-v1','operation':operation,'phase':'FENCE','status':'BLOCKED',
            'fence_status':'VERIFIED','error':'exception_Blocked','code_capture':{'member':'a'*64},
            'events':[{'kind':'staging_preflight'},{'kind':'rollout_original_capture'},
                      {'kind':'rollout_maintenance_owned'},{'kind':'workload_scaled','name':retirement.HUB,'replicas':0}],
            'context':{'original_snapshots':originals,'snapshots':{n:dict(copy.deepcopy(r),spec=dict(copy.deepcopy(r['spec']),replicas=0)) for n,r in originals.items()},'marker':marker,'expected':{
                'namespace_uid':str(uuid.uuid4()),'source_claim':'selected','source_claim_uid':str(uuid.uuid4()),
                'source_pv_uid':str(uuid.uuid4())},'final_claim':{'spec':{'volumeName':'pv'}},
                'final_pv':{'spec':{'claimRef':{'name':'selected'}}},'final_path':'/selected',
                'service':{'metadata':{'uid':str(uuid.uuid4())},'spec':{'ports':[]}}}}
        setup={n:{'uid':r['metadata']['uid'],'template_sha256':retirement.canonical(r['spec']['template']),
            'replicas':1,'ready':n==retirement.HUB} for n,r in originals.items()}
        events=[{'kind':'operator_precut_abort_intent'}]+[{'kind':'workload_scaled','name':n,'replicas':1} for n in sorted(names)]+[{'kind':'operator_precut_abort_completed'}]
        receipt={'schema':'voice-operator-precut-abort-closure-v1','operation':operation,
            'snapshot_sha256':retirement.canonical(snapshot),'code_binding_sha256':retirement.canonical(snapshot['code_capture']),
            'waiver':retirement.WAIVER,'deployment_pass':False,'rollback_eligible':False,'events':events,
            'setup':setup,'retained_setup':retirement.precut_retained_setup(snapshot),
            'configmaps':{n:{'uid':str(uuid.uuid4()),'data_sha256':'a'*64,'binary_data_sha256':'b'*64} for n in retirement.CONFIGMAPS},
            'before_marker':{'uid':marker['metadata']['uid'],'resourceVersion':'123','data':marker['data']},
            'released_marker':{'uid':marker['metadata']['uid'],'resourceVersion':'124',
                'data':{'phase':'active','dataPVC':'selected'}},'storage_verified':True,'config_verified':True,
            'journals':{}}
        nonce=operation+'a'*52
        self.journal={'phase':'STARTED','prepare_error':'guard_rejected',
            'request':{'action':'prepare','mode':'full','nonce':nonce}}
        self.journal['request_sha256']=retirement.canonical(self.journal['request'])
        self.response={'status':'BLOCKED','error':'exception_Blocked'}
        receipt['journals'][nonce+'.json']={'journal_sha256':retirement.canonical(self.journal),
            'request_sha256':self.journal['request_sha256'],'response_sha256':retirement.canonical(self.response)}
        state=copy.deepcopy(snapshot);state.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',
            events=snapshot['events']+events,operator_precut_abort={'schema':'voice-operator-precut-abort-reference-v1',
                'snapshot_sha256':receipt['snapshot_sha256'],'receipt_sha256':retirement.canonical(receipt)})
        return snapshot,receipt,state

    def verify(self,snapshot,receipt,state):
        base=Path('/private/rollout-'+snapshot['operation'])
        def read(path,*args,**kwargs):
            if Path(path).name=='operator-precut-abort-snapshot.json':return json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()
            if Path(path).parent.name in ('journal','responses'):
                row=self.journal if Path(path).parent.name=='journal' else self.response
                return json.dumps(row,sort_keys=True,separators=(',',':')).encode()
            raise AssertionError(path)
        with patch.object(retirement,'private_bytes',side_effect=read),\
             patch.object(retirement,'document',return_value=receipt),\
             patch.object(retirement.root_cli,'code_binding',return_value=snapshot['code_capture']):
            return retirement.idle_state(base,state)

    def test_bound_precut_closure_is_idle_without_claiming_deployment_success(self):
        snapshot,receipt,state=self.fixture()
        self.assertTrue(self.verify(snapshot,receipt,state))
        self.assertEqual(state['status'],'ABORTED');self.assertFalse(receipt['deployment_pass'])
        self.assertEqual(state['events'][:len(snapshot['events'])],snapshot['events'])

    def test_cut_effects_history_setup_marker_and_provenance_drift_refuse_idle(self):
        for fault in ('cut','effect_event','history','setup','marker','code','reference','receipt','raw_aborted'):
            snapshot,receipt,state=self.fixture()
            if fault=='cut':snapshot['cut']={'present':True}
            elif fault=='effect_event':snapshot['events'].append({'kind':'original_hub_shutdown_intent'})
            elif fault=='history':state['events'][0]['kind']='changed'
            elif fault=='setup':receipt['setup'][retirement.HUB]['ready']=False
            elif fault=='marker':receipt['released_marker']['data']['knownRolloutOperation']=snapshot['operation']
            elif fault=='code':receipt['code_binding_sha256']='b'*64
            elif fault=='reference':state['operator_precut_abort']['snapshot_sha256']='b'*64
            elif fault=='receipt':receipt['deployment_pass']=True
            else:state.pop('operator_precut_abort')
            if fault not in ('reference','raw_aborted'):
                receipt['snapshot_sha256']=retirement.canonical(snapshot)
                state['operator_precut_abort'].update(snapshot_sha256=receipt['snapshot_sha256'],receipt_sha256=retirement.canonical(receipt))
            with self.subTest(fault=fault),self.assertRaises((Blocked,FileNotFoundError)):
                self.verify(snapshot,receipt,state)



import contextlib
import hashlib
import json
import os
from pathlib import Path
import tempfile
import uuid
from unittest.mock import patch

class FakeKube:
    def __init__(self,objects,fail_cycle=False):
        self.objects=copy.deepcopy(objects);self.patches=[];self.fail_cycle=fail_cycle
    def get(self,kind,name):
        row=self.objects[(kind.lower(),name)]
        if self.fail_cycle and name=='voice-user' and row['spec'].get('replicas')==1:
            self.fail_cycle=False
            raise Blocked('fixture_readiness_failure')
        return copy.deepcopy(row)
    def run(self,args,body=None,timeout=30):
        if args[:1]!=['get']:raise AssertionError('unexpected external mutation')
        return {'items':[]}
    def cas(self,kind,row,patches):
        key=(kind.lower(),row['metadata']['name']);current=self.objects[key]
        self.assert_identity(row,current);changed=copy.deepcopy(current)
        for item in patches:
            parts=[p.replace('~1','/').replace('~0','~') for p in item['path'].split('/')[1:]]
            parent=changed
            for part in parts[:-1]:parent=parent[int(part)] if isinstance(parent,list) else parent[part]
            leaf=parts[-1]
            if item['op']=='test':
                actual=parent[int(leaf)] if isinstance(parent,list) else parent[leaf]
                if actual!=item['value']:raise Blocked('fixture_cas_test_failed')
            elif item['op']=='remove':del parent[leaf]
            elif isinstance(parent,list) and leaf=='-':parent.append(copy.deepcopy(item['value']))
            else:parent[leaf]=copy.deepcopy(item['value'])
        changed['metadata']['resourceVersion']=str(int(current['metadata']['resourceVersion'])+1)
        if kind.lower()=='deployment':
            changed['metadata']['generation']=current['metadata']['generation']+1
            desired=changed['spec']['replicas'];changed['status']={
                'observedGeneration':changed['metadata']['generation'],'readyReplicas':desired,
                'updatedReplicas':desired,'availableReplicas':desired}
        self.objects[key]=changed;self.patches.append((kind,row['metadata']['name'],copy.deepcopy(patches)))
        return copy.deepcopy(changed)
    @staticmethod
    def assert_identity(before,current):
        if before['metadata']!=current['metadata']:raise Blocked('fixture_identity_changed')

@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'owned Linux root fixture')
class RetirementActionTests(unittest.TestCase):
    def fixture(self,folder):
        root=Path(folder);installed=root/'installed';installed.mkdir()
        for name in ('inbox','processing','journal','responses','sources','code'):
            (installed/name).mkdir()
        (installed/'inbox').chmod(0o1730)
        base=root/('rollout-'+retirement.OPERATION);base.mkdir()
        marker={'metadata':{'name':'voice-nats-generation','uid':str(uuid.uuid4()),
            'resourceVersion':'3154269'},'data':{'phase':'rollout-prepared',
            'knownRolloutOperation':retirement.OPERATION,'generation':'r20260930a4','dataPVC':'selected'}}
        claim_uid=str(uuid.uuid4());pv_uid=str(uuid.uuid4())
        claim={'metadata':{'name':'selected','uid':claim_uid},'spec':{'volumeName':'pv'},
            'status':{'phase':'Bound'}}
        path='/var/lib/rancher/k3s/storage/pvc-'+claim_uid+'_voice-staging_selected'
        pv={'metadata':{'name':'pv','uid':pv_uid},'spec':{
            'claimRef':{'uid':claim_uid},'hostPath':{'path':path},
            'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[
                {'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
        service={'metadata':{'name':'voice-nats','uid':str(uuid.uuid4())},'spec':{'selector':{'nats':'hub'}}}
        original={};snapshots={};objects={('configmap','voice-nats-generation'):marker,
            ('pvc','selected'):claim,('pv','pv'):pv,('service','voice-nats'):service,
            ('namespace','voice-staging'):{'metadata':{'uid':str(uuid.uuid4())}}}
        for name in sorted(retirement.CORE):
            template={'metadata':{'annotations':{}},'spec':{'containers':[{
                'name':'user' if name=='voice-user' else 'main','env':[]}],
                'volumes':[]}}
            old={'metadata':{'name':name,'uid':str(uuid.uuid4()),'resourceVersion':'1','generation':1},
                'spec':{'replicas':1,'template':template}}
            original[name]=old;snapshots[name]=copy.deepcopy(old);snapshots[name]['spec']['replicas']=0
            objects[('deployment',name)]=snapshots[name]
        configmaps={}
        for name in retirement.CONFIGMAPS:
            row={'metadata':{'name':name,'uid':str(uuid.uuid4())},'data':{'SPACE_GRPC_ADDR':'space:80'}}
            objects[('configmap',name)]=row
            configmaps[name]={'uid':row['metadata']['uid'],'data_sha256':retirement.canonical(row['data'])}
        expected={'namespace_uid':objects[('namespace','voice-staging')]['metadata']['uid'],
            'marker_uid':marker['metadata']['uid'],'generation':'r20260930a4',
            'source_claim':'selected','source_claim_uid':claim_uid,'source_pv_uid':pv_uid}
        snapshot={'schema':'nats-rollout-root-v1','operation':retirement.OPERATION,
            'phase':'PAUSED_APPLY','status':'WAITING','fence_status':'VERIFIED',
            'events':[{'kind':'historical','index':i} for i in range(83)],
            'cut':{'original':'preserved'},'authorization':{'expires_at':'expired'},
            'nats_migration':{'verified':True},'context':{'expected':expected,
                'marker':marker,'service':service,'final_claim':claim,'final_pv':pv,
                'final_path':path,'snapshots':snapshots,'original_snapshots':original}}
        raw=json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()
        checkpoint=base/'checkpoint.json';checkpoint.write_bytes(raw);checkpoint.chmod(0o600)
        nonce=retirement.OPERATION+'a'*52;request={'action':'prepare','nonce':nonce,'run_id':1}
        journal={'schema':'voice-nats-bridge-request-v1','phase':'STARTED','request':request,
            'request_sha256':retirement.canonical(request)}
        journal_raw=json.dumps(journal,sort_keys=True,separators=(',',':')).encode()
        response_raw=b'{"status":"BLOCKED"}'
        (installed/'journal'/(nonce+'.json')).write_bytes(journal_raw)
        (installed/'journal'/(nonce+'.json')).chmod(0o600)
        (installed/'responses'/(nonce+'.json')).write_bytes(response_raw)
        (installed/'responses'/(nonce+'.json')).chmod(0o440)
        binding={'test.py':'a'*64}
        plan={'schema':'voice-specific-operator-retirement-plan-v1','operation':retirement.OPERATION,
            'checkpoint_sha256':hashlib.sha256(raw).hexdigest(),'code_binding_sha256':retirement.canonical(binding),
            'waiver':retirement.WAIVER,'configmaps':configmaps,
            'journals':{nonce+'.json':{'journal_sha256':hashlib.sha256(journal_raw).hexdigest(),
                'request_sha256':retirement.canonical(request),'response_sha256':hashlib.sha256(response_raw).hexdigest()}}}
        plan_raw=json.dumps(plan,sort_keys=True,separators=(',',':')).encode()
        plan_path=root/'plan.json';plan_path.write_bytes(plan_raw);plan_path.chmod(0o600)
        return root,base,snapshot,raw,objects,binding,plan_path,hashlib.sha256(plan_raw).hexdigest()

    def action(self,fixture,kube):
        root,base,snapshot,raw,objects,binding,plan,plan_sha=fixture
        with patch.object(retirement.guard,'ROOT',root),\
             patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(raw).hexdigest()),\
             patch.object(retirement,'V15_BINDING',retirement.canonical(binding)),\
             patch.object(retirement.root_cli,'operation_path',side_effect=Path),\
             patch.object(retirement.root_cli,'code_binding',return_value=binding),\
             patch.object(retirement,'operation_lock',return_value=contextlib.nullcontext()):
            return retirement.retire(root/'candidate',plan,plan_sha,kube=kube)

    def test_real_ordered_startup_closes_only_waived_operation_and_verifier_rejects_drift(self):
        with tempfile.TemporaryDirectory() as folder:
            fixture=self.fixture(folder);kube=FakeKube(fixture[4]);result=self.action(fixture,kube)
            self.assertEqual(result['status'],'ABORTED')
            root,base,snapshot,raw,_,_,_,_=fixture
            state=json.loads((base/'checkpoint.json').read_bytes())
            self.assertEqual((base/'operator-retirement-snapshot.json').read_bytes(),raw)
            self.assertEqual(state['events'][:83],snapshot['events'])
            self.assertEqual(kube.objects[('configmap','voice-nats-generation')]['data']['phase'],'active')
            for name in retirement.CORE:
                self.assertEqual(kube.objects[('deployment',name)]['spec']['template'],
                    snapshot['context']['original_snapshots'][name]['spec']['template'])
                self.assertEqual(kube.objects[('deployment',name)]['spec']['replicas'],1)
            scales=[name for kind,name,items in kube.patches if any(i['path']=='/spec/replicas' for i in items)]
            self.assertEqual(scales[0],retirement.HUB);self.assertEqual(scales[-1],'voice-gateway')
            with patch.object(retirement.guard,'ROOT',root),patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(raw).hexdigest()):
                receipt=retirement.verify_terminal(base,state)
                # Future active marker revisions are not part of historical closure verification.
                kube.objects[('configmap','voice-nats-generation')]['metadata']['resourceVersion']='9999999'
                retirement.verify_terminal(base,state)
                retirement.pending_request_accepted(root/'installed',next((root/'installed/journal').iterdir()),receipt)
                unknown=root/'installed/journal'/('b'*64+'.json')
                unknown.write_text('{"phase":"STARTED"}');unknown.chmod(0o600)
                with self.assertRaises(Blocked):retirement.pending_request_accepted(root/'installed',unknown,receipt)
                for failure in ('snapshot','plan','receipt','history','mode'):
                    with self.subTest(failure=failure):
                        changed=copy.deepcopy(state)
                        target=base/('operator-retirement-'+failure+'.json')
                        if failure=='history':
                            changed['authorization']={'expires_at':'renewed'}
                            with self.assertRaises(Blocked):retirement.verify_terminal(base,changed)
                            continue
                        if failure=='mode':target=base/'operator-retirement-plan.json'
                        previous=target.read_bytes();mode=target.stat().st_mode&0o777
                        if failure=='mode':target.chmod(0o640)
                        elif failure=='receipt':
                            changed_receipt=json.loads(previous);changed_receipt['waiver']='invalid'
                            target.write_text(json.dumps(changed_receipt))
                        else:target.write_bytes(previous+b' ')
                        with self.assertRaises(Blocked):retirement.verify_terminal(base,state)
                        target.write_bytes(previous);target.chmod(mode)

    def test_interrupted_owned_user_override_restores_and_refences_without_idle_receipt(self):
        with tempfile.TemporaryDirectory() as folder:
            fixture=self.fixture(folder);kube=FakeKube(fixture[4],fail_cycle=True)
            with self.assertRaises(Blocked):self.action(fixture,kube)
            state=json.loads((fixture[1]/'checkpoint.json').read_bytes())
            self.assertEqual((state['phase'],state['status'],state['fence_status']),
                ('OPERATOR_RETIRING','BLOCKED','VERIFIED'))
            self.assertFalse((fixture[1]/'operator-retirement-receipt.json').exists())
            for name in retirement.CORE:self.assertEqual(kube.objects[('deployment',name)]['spec']['replicas'],0)
            self.assertEqual(kube.objects[('deployment','voice-user')]['spec']['template'],
                fixture[2]['context']['original_snapshots']['voice-user']['spec']['template'])
            self.assertEqual((fixture[0]/'installed/inbox').stat().st_mode&0o7777,0o1730)

    def test_changed_config_vetoes_before_any_startup_patch(self):
        with tempfile.TemporaryDirectory() as folder:
            fixture=self.fixture(folder);kube=FakeKube(fixture[4])
            kube.objects[('configmap','voice-app-config')]['data']['SPACE_GRPC_ADDR']='changed'
            with self.assertRaises(Blocked):self.action(fixture,kube)
            self.assertEqual(kube.patches,[])



class IdleUpgradeTests(unittest.TestCase):
    def test_actual_idle_update_accepts_verified_closure_and_vetoes_unknown_pending_or_history_write(self):
        import installer
        from types import SimpleNamespace
        helper=RetirementActionTests()
        for failure in (None,'unknown','history'):
            with self.subTest(failure=failure),tempfile.TemporaryDirectory() as folder:
                fixture=helper.fixture(folder);kube=FakeKube(fixture[4]);helper.action(fixture,kube)
                root,base,snapshot,raw,_,binding,_,_=fixture
                installed=root/'installed';(installed/'recovery').mkdir()
                for name in ('recovery-key.pem','recovery-cert.pem'):
                    path=installed/'recovery'/name;path.write_bytes(b'protected existing key material');path.chmod(0o600)
                policy=installed/'policy.json';policy.write_text('{}');policy.chmod(0o600)
                if failure=='unknown':
                    path=installed/'journal'/('b'*64+'.json');path.write_text('{"phase":"STARTED"}');path.chmod(0o600)
                def swap(*args):
                    if failure=='history':
                        state=json.loads((base/'checkpoint.json').read_bytes());state['authorization']={'expires_at':'changed'}
                        (base/'checkpoint.json').write_text(json.dumps(state))
                with patch.object(retirement.guard,'ROOT',root),\
                     patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(raw).hexdigest()),\
                     patch.object(retirement,'V15_BINDING',retirement.canonical(binding)),\
                     patch.object(retirement.root_cli,'code_binding',return_value=binding),\
                     patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()),\
                     patch.object(installer,'Kube',return_value=kube),\
                     patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)),\
                     patch('story_witness.registry_snapshot',return_value={'fixed':'witness'}),\
                     patch.object(installer,'_upgrade_code',side_effect=swap) as upgrade:
                    if failure is None:
                        installer.upgrade_idle(root/'candidate')
                        upgrade.assert_called_once_with(root/'candidate',installed,1000,
                            retirement.canonical(binding),'idle','v15')
                    else:
                        with self.assertRaises(Blocked):installer.upgrade_idle(root/'candidate')
                        if failure=='unknown':upgrade.assert_not_called()
                    self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

    def test_actual_isolated_direct_retirement_entry_imports_before_argument_refusal(self):
        import subprocess
        import sys
        result=subprocess.run([sys.executable,'-I','-S',str(Path(retirement.__file__).resolve())],
            capture_output=True,text=True,timeout=10)
        self.assertNotEqual(result.returncode,0)
        self.assertIn('usage: operator_retirement.py',result.stderr)
        self.assertNotIn('ModuleNotFoundError',result.stderr)

if __name__=='__main__':unittest.main()


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'owned Linux root fixture')
class WaivedContinuationTests(unittest.TestCase):
    def fixture(self,folder):
        helper=RetirementActionTests();fixture=helper.fixture(folder)
        root,base,snapshot,raw,objects,binding,plan,plan_sha=fixture
        (base/'operator-retirement-snapshot.json').write_bytes(raw)
        (base/'operator-retirement-plan.json').write_bytes(plan.read_bytes())
        for name in ('operator-retirement-snapshot.json','operator-retirement-plan.json'):
            (base/name).chmod(0o600)
        failed=copy.deepcopy(snapshot)
        prior=[{'kind':'operator_retirement_intent','waiver':retirement.WAIVER,'plan_sha256':plan_sha}]
        prior += [{'kind':'workload_scaled','name':'voice-space','replicas':0} for _ in range(41)]
        failed.update(phase='OPERATOR_RETIRING',status='BLOCKED',fence_status='VERIFIED',
            events=copy.deepcopy(snapshot['events']+prior))
        failed_raw=json.dumps(failed,sort_keys=True,separators=(',',':')).encode()
        (base/'checkpoint.json').write_bytes(failed_raw)
        current_binding={'test.py':'b'*64}
        attestation={'schema':'voice-specific-retirement-continuation-v1',
            'operation':retirement.OPERATION,'failed_checkpoint_sha256':hashlib.sha256(failed_raw).hexdigest(),
            'original_plan_sha256':plan_sha,'code_binding_sha256':retirement.canonical(current_binding),
            'waiver':retirement.WAIVER}
        path=root/'continuation.json'
        path.write_text(json.dumps(attestation,sort_keys=True,separators=(',',':')))
        path.chmod(0o600)
        return fixture,failed,failed_raw,current_binding,path

    def invoke(self,values,kube):
        fixture,failed,failed_raw,current_binding,path=values
        root,base,snapshot,raw,objects,binding,plan,plan_sha=fixture
        with patch.object(retirement.guard,'ROOT',root),\
             patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(raw).hexdigest()),\
             patch.object(retirement,'FAILED_RETIREMENT_SHA',hashlib.sha256(failed_raw).hexdigest()),\
             patch.object(retirement,'ORIGINAL_RETIREMENT_BINDING',retirement.canonical(binding)),\
             patch.object(retirement,'V15_BINDING',retirement.canonical(binding)),\
             patch.object(retirement.root_cli,'operation_path',side_effect=Path),\
             patch.object(retirement.root_cli,'code_binding',
                side_effect=lambda p:binding if Path(p)==root/'installed/code' else current_binding),\
             patch.object(retirement,'operation_lock',return_value=contextlib.nullcontext()):
            return retirement.continue_retirement(root/'candidate',path,
                hashlib.sha256(path.read_bytes()).hexdigest(),kube=kube)

    def test_actual_continuation_records_unready_setup_without_deployment_pass(self):
        with tempfile.TemporaryDirectory() as folder:
            values=self.fixture(folder);fixture,failed,failed_raw,current_binding,path=values
            class UnreadyKube(FakeKube):
                def cas(self,*args):
                    result=super().cas(*args)
                    if args[0]=='deployment' and args[1]['metadata']['name']=='voice-space':
                        row=self.objects[('deployment','voice-space')]
                        row['status'].update(readyReplicas=0,availableReplicas=0)
                    return result
            kube=UnreadyKube(fixture[4]);result=self.invoke(values,kube)
            self.assertFalse(result['setup_ready'])
            self.assertEqual(result['status'],'ABORTED')
            base=fixture[1];state=json.loads((base/'checkpoint.json').read_bytes())
            receipt=json.loads((base/'operator-retirement-receipt.json').read_bytes())
            self.assertEqual(state['events'][:125],failed['events'])
            self.assertFalse(receipt['setup']['voice-space']['ready'])
            self.assertTrue(receipt['setup'][retirement.HUB]['ready'])
            self.assertFalse(receipt['deployment_pass']);self.assertFalse(receipt['rollback_eligible'])
            self.assertEqual((base/'operator-retirement-failed-attempt.json').read_bytes(),failed_raw)
            self.assertEqual((base/'operator-retirement-plan.json').read_bytes(),fixture[6].read_bytes())
            self.assertEqual(kube.objects[('configmap','voice-nats-generation')]['data']['phase'],'active')
            self.assertTrue(all(kube.objects[('deployment',n)]['spec']['replicas']==1 for n in retirement.CORE))
            with patch.object(retirement.guard,'ROOT',fixture[0]),\
                 patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(fixture[3]).hexdigest()),\
                 patch.object(retirement,'FAILED_RETIREMENT_SHA',hashlib.sha256(failed_raw).hexdigest()),\
                 patch.object(retirement,'ORIGINAL_RETIREMENT_BINDING',retirement.canonical(fixture[5])):
                retirement.verify_terminal(base,state)
                changed=copy.deepcopy(state);changed['events'][100]['replicas']=1
                with self.assertRaises(Blocked):retirement.verify_terminal(base,changed)
                prior=path.read_bytes()
                (base/'operator-retirement-continuation.json').write_bytes(prior+b' ')
                with self.assertRaises(Blocked):retirement.verify_terminal(base,state)

    def test_continuation_refuses_changed_original_setup_before_scaling(self):
        with tempfile.TemporaryDirectory() as folder:
            values=self.fixture(folder);kube=FakeKube(values[0][4])
            kube.objects[('deployment','voice-space')]['spec']['template']['metadata']['annotations']['foreign']='change'
            with self.assertRaises(Blocked):self.invoke(values,kube)
            self.assertEqual(kube.patches,[])
