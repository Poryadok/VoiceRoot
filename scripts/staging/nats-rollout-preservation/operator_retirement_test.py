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




class CapturedClosureTests(unittest.TestCase):
    def fixture(self):
        helper=PrecutClosureTests();snapshot,receipt,state=helper.fixture()
        operation=snapshot['operation'];self.files={}
        self.files['rollout-before.tar']=b'authenticated native archive'
        manifest={'archive_sha256':hashlib.sha256(self.files['rollout-before.tar']).hexdigest()}
        self.files['rollout-before-manifest.json']=json.dumps(manifest,sort_keys=True,separators=(',',':')).encode()
        self.files['copy-checkpoint.json']=b'{"status":"PASS"}'
        self.files['rollout-backup.cms']=b'authenticated ciphertext'
        self.files['apply-manifests.json']=b'[]'
        cipher={'operation':operation,'challenge':'a'*32,'run_id':1,'head_sha':'b'*40,
            'cipher_sha256':hashlib.sha256(self.files['rollout-backup.cms']).hexdigest(),
            'cipher_bytes':len(self.files['rollout-backup.cms']),'created_at':'2026-10-11T03:51:00+00:00'}
        restored={'schema':'voice-nats-decrypted-restore-v1','decrypted_members_verified':True,
            'remote_equivalence':'FULL_CIPHERTEXT_SHA256_AND_SIZE','cipher_sha256':cipher['cipher_sha256'],
            'cipher_bytes':cipher['cipher_bytes'],'readback':{'native_archive_verified':True},
            'members':{n:hashlib.sha256(self.files[n]).hexdigest() for n in ('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json')}}
        self.files['decrypted-restore.json']=json.dumps(restored,sort_keys=True,separators=(',',':')).encode()
        snapshot.update(phase='AWAITING_OFF_NODE',status='WAITING',cut={'manifest':manifest,
            'manifest_sha256':hashlib.sha256(self.files['rollout-before-manifest.json']).hexdigest()},
            cipher_binding=cipher,custody=dict(cipher,verified=True),
            target={'manifest_sha256':hashlib.sha256(self.files['apply-manifests.json']).hexdigest()},
            nats_contract_expires_at='2026-10-11T07:50:00+00:00')
        snapshot['events'] += [{'kind':k} for k in ('original_hub_shutdown_intent',
            'original_hub_orderly_exit_verified','original_hub_owned_finalizer_removed')]
        events=copy.deepcopy(receipt['events']);events[0]['kind']='operator_captured_abort_intent';events[-1]['kind']='operator_captured_abort_completed'
        receipt.update(schema='voice-operator-captured-abort-closure-v1',snapshot_sha256=retirement.canonical(snapshot),events=events,
            retained_files={n:{'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)} for n,raw in self.files.items()})
        nonce=operation+'a'*52;prepare=helper.journal
        prepare.pop('prepare_error');prepare['phase']='COMPLETE';prepare['result']={'operation':operation,'phase':'AWAITING_OFF_NODE','status':'WAITING','fence_status':'VERIFIED'}
        response=copy.deepcopy(prepare['result'])
        auth_nonce='c'*64;auth={'phase':'STARTED','prepare_error':'guard_rejected',
            'request':{'action':'authorize','operation':operation,'nonce':auth_nonce}}
        auth['request_sha256']=retirement.canonical(auth['request'])
        self.journals={nonce+'.json':prepare,auth_nonce+'.json':auth}
        self.responses={nonce+'.json':response,auth_nonce+'.json':{'status':'BLOCKED','error':'exception_Blocked'}}
        receipt['journals']={n:{'journal_sha256':retirement.canonical(r),'request_sha256':r['request_sha256'],
            'response_sha256':retirement.canonical(self.responses[n])} for n,r in self.journals.items()}
        terminal=copy.deepcopy(snapshot);terminal.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',
            events=snapshot['events']+events,operator_captured_abort={'schema':'voice-operator-captured-abort-reference-v1',
                'snapshot_sha256':receipt['snapshot_sha256'],'receipt_sha256':retirement.canonical(receipt)})
        return snapshot,receipt,terminal

    def verify(self,snapshot,receipt,state):
        base=Path('/private/rollout-'+snapshot['operation'])
        def read(path,*args,**kwargs):
            path=Path(path)
            if path.name=='operator-captured-abort-snapshot.json':return json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()
            if path.parent.name in ('journal','responses'):
                rows=self.journals if path.parent.name=='journal' else self.responses
                return json.dumps(rows[path.name],sort_keys=True,separators=(',',':')).encode()
            return self.files[path.name]
        with patch.object(retirement,'private_bytes',side_effect=read),patch.object(retirement,'document',return_value=receipt),\
             patch.object(retirement.root_cli,'code_binding',return_value=snapshot['code_capture']):
            return retirement.verify_captured_terminal(base,state)

    def test_backup_complete_unapplied_closure_preserves_evidence_and_old_deadlines(self):
        snapshot,receipt,state=self.fixture()
        self.verify(snapshot,receipt,state)
        self.assertEqual(state['events'][:len(snapshot['events'])],snapshot['events'])
        self.assertEqual(state['cut'],snapshot['cut']);self.assertEqual(state['cipher_binding'],snapshot['cipher_binding'])
        self.assertEqual(state['nats_contract_expires_at'],snapshot['nats_contract_expires_at'])
        self.assertFalse(receipt['deployment_pass']);self.assertFalse(receipt['rollback_eligible'])

    def test_effect_evidence_identity_deadline_or_raw_abort_drift_refuses_closure(self):
        for fault in ('authorization','migration','effect_event','archive','cipher','restore','manifest','deadline','code','custody','raw_aborted'):
            snapshot,receipt,state=self.fixture()
            if fault in ('authorization','migration'):
                snapshot['authorization' if fault=='authorization' else 'nats_migration']={'present':True}
            elif fault=='effect_event':snapshot['events'].append({'kind':'migration_complete'})
            elif fault in ('archive','cipher','restore','manifest'):
                name={'archive':'rollout-before.tar','cipher':'rollout-backup.cms','restore':'decrypted-restore.json','manifest':'apply-manifests.json'}[fault]
                self.files[name]+=b'changed'
            elif fault=='deadline':state['nats_contract_expires_at']='renewed'
            elif fault=='code':receipt['code_binding_sha256']='f'*64
            elif fault=='custody':snapshot['custody']['verified']=False
            else:state.pop('operator_captured_abort')
            if fault not in ('deadline','raw_aborted'):
                receipt['snapshot_sha256']=retirement.canonical(snapshot)
                state['operator_captured_abort'].update(snapshot_sha256=receipt['snapshot_sha256'],receipt_sha256=retirement.canonical(receipt))
            with self.subTest(fault=fault),self.assertRaises((Blocked,KeyError,FileNotFoundError)):
                self.verify(snapshot,receipt,state)


class PostmigrationClosureTests(unittest.TestCase):
    def fixture(self):
        helper=CapturedClosureTests();snapshot,receipt,_=helper.fixture();self.files=helper.files
        original=snapshot['context']['original_snapshots']
        snapshot.update(phase='PAUSED_APPLY',renderer_applied={'verified':True},
            nats_migration={'verified':True},migration_jobs=[{'name':'migration','uid':'job','configmap_uid':'cm'}],
            migration_completed_metadata={'bound':True},authorization={'expires_at':'2026-10-11T04:00:00+00:00'})
        snapshot['target']['mode']='full'
        snapshot['events'].append({'kind':'migration_complete',**snapshot['migration_jobs'][0]})
        snapshot['context']['marker']['data']['phase']='rollout-prepared'
        resources={}
        for name,row in original.items():
            if name==retirement.HUB:continue
            r=copy.deepcopy(snapshot['context']['snapshots'][name]);r['kind']='Deployment';r['metadata']['name']=name
            resources['Deployment/'+name]=r
        for i in range(25):
            name='service-'+str(i);resources['Service/'+name]={'kind':'Service','metadata':{'name':name,'uid':str(uuid.uuid4())},'spec':{'ports':[]}}
        resources['ConfigMap/voice-app-config']={'kind':'ConfigMap','metadata':{'name':'voice-app-config','uid':str(uuid.uuid4())},'data':{'retained':'actual'}}
        documents=copy.deepcopy(list(resources.values()))
        for row in documents:
            if row['kind']!='Deployment':row['metadata'].pop('uid')
        self.files['apply-manifests.json']=json.dumps({'apiVersion':'v1','kind':'List','items':documents}).encode()
        snapshot['target']['manifest_sha256']=hashlib.sha256(self.files['apply-manifests.json']).hexdigest()
        self.files['apply-authorization.json']=json.dumps(snapshot['authorization']).encode()
        self.files['operator-postmigration-abort-resources.json']=json.dumps(resources).encode()
        snapshot['nonnats_binding']={'objects':[{'kind':r['kind'],'name':r['metadata']['name'],'uid':r['metadata']['uid']} for r in resources.values()]}
        receipt.update(schema='voice-operator-postmigration-abort-closure-v1',snapshot_sha256=retirement.canonical(snapshot),
            observed_at='2026-10-11T04:01:00+00:00',completed_migrations_sha256=retirement.canonical(snapshot['migration_jobs']),
            configuration_basis='RETAIN_ACTUAL_POST_MIGRATION_CONFIGURATION_NO_ORIGINAL_WRITE_CLAIM',journals={},
            resource_disposition_sha256=hashlib.sha256(self.files['operator-postmigration-abort-resources.json']).hexdigest(),
            retained_files={n:{'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw),'mode':0o600} for n,raw in self.files.items() if n!='operator-postmigration-abort-resources.json'})
        receipt['before_marker']['data']=dict(snapshot['context']['marker']['data'],phase='rollout-applying')
        receipt['consumed_claim']={'uid':receipt['before_marker']['uid'],'resourceVersion':receipt['before_marker']['resourceVersion'],
            'authorization_sha256':receipt['retained_files']['apply-authorization.json']['sha256'],'expires_at':snapshot['authorization']['expires_at']}
        receipt['events']=[{'kind':'operator_postmigration_abort_intent'}]+[
            {'kind':'operator_postmigration_workload_restored','name':n,'uid':r['metadata']['uid'],
             'template_sha256':retirement.canonical(r['spec']['template']),'replicas':1} for n,r in sorted(original.items())]+[{'kind':'operator_postmigration_abort_completed'}]
        return snapshot,receipt,self.terminal(snapshot,receipt)

    def terminal(self,snapshot,receipt):
        result=copy.deepcopy(snapshot);result.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',events=snapshot['events']+receipt['events'])
        result['operator_postmigration_abort']={'schema':'voice-operator-postmigration-abort-reference-v1','snapshot_sha256':receipt['snapshot_sha256'],'receipt_sha256':retirement.canonical(receipt)}
        return result

    def verify(self,snapshot,receipt,state):
        base=Path('/private/rollout-'+snapshot['operation'])
        def read(path,*args,**kwargs):
            if Path(path).name=='operator-postmigration-abort-snapshot.json':return json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()
            return self.files[Path(path).name]
        with patch.object(retirement,'private_bytes',side_effect=read),patch.object(retirement,'document',return_value=receipt),\
             patch.object(retirement.root_cli,'code_binding',return_value=snapshot['code_capture']),\
             patch.object(retirement,'_retained_mode',return_value=0o600):
            return retirement.verify_postmigration_terminal(base,state)

    def test_forward_migrations_and_partial_configuration_retained_without_pass(self):
        snapshot,receipt,state=self.fixture();self.verify(snapshot,receipt,state)
        self.assertEqual(state['authorization'],snapshot['authorization']);self.assertEqual(state['migration_jobs'],snapshot['migration_jobs'])
        self.assertFalse(receipt['deployment_pass']);self.assertFalse(receipt['rollback_eligible'])

    def test_postmigration_closure_refuses_effect_or_binding_drift(self):
        for fault in ('pass','deadline','job','claim','inventory','template','file','history','ready','ledger_uid','explicit_frozen_uid'):
            snapshot,receipt,state=self.fixture()
            if fault=='pass':receipt['deployment_pass']=True
            elif fault=='deadline':receipt['observed_at']='2026-10-11T03:59:00+00:00'
            elif fault=='job':receipt['completed_migrations_sha256']='a'*64
            elif fault=='claim':receipt['consumed_claim']['resourceVersion']='other'
            elif fault=='inventory':receipt['resource_disposition_sha256']='a'*64
            elif fault=='template':snapshot['context']['snapshots']['voice-story']['spec']['template']={'changed':True}
            elif fault=='file':self.files['rollout-backup.cms']+=b'changed'
            elif fault=='history':snapshot['events'].append({'kind':'rollout_target_applied'})
            elif fault=='ready':receipt['setup'][retirement.HUB]['ready']=False
            elif fault=='ledger_uid':snapshot['nonnats_binding']['objects'][0]['uid']='foreign'
            else:
                documents=json.loads(self.files['apply-manifests.json'])
                next(r for r in documents['items'] if r['kind']=='Service')['metadata']['uid']='foreign'
                self.files['apply-manifests.json']=json.dumps(documents).encode()
                pin=hashlib.sha256(self.files['apply-manifests.json']).hexdigest()
                snapshot['target']['manifest_sha256']=pin
                receipt['retained_files']['apply-manifests.json'].update(sha256=pin,bytes=len(self.files['apply-manifests.json']))
            receipt['snapshot_sha256']=retirement.canonical(snapshot)
            with self.subTest(fault=fault),self.assertRaises((Blocked,KeyError)):
                self.verify(snapshot,receipt,self.terminal(snapshot,receipt))


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

    def test_actual_ceccb_idle_update_reuses_verified_closure_and_all_idle_vetoes(self):
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
                    path=installed/'journal'/('d'*64+'.json');path.write_text('{"phase":"STARTED"}');path.chmod(0o600)
                def swap(*args):
                    if failure=='history':
                        state=json.loads((base/'checkpoint.json').read_bytes());state['authorization']={'expires_at':'changed'}
                        (base/'checkpoint.json').write_text(json.dumps(state))
                with patch.object(retirement.guard,'ROOT',root),\
                     patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(raw).hexdigest()),\
                     patch.object(retirement.root_cli,'code_binding',return_value=binding),\
                     patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()),\
                     patch.object(installer,'Kube',return_value=kube),\
                     patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)),\
                     patch('story_witness.registry_snapshot',return_value={'fixed':'witness'}),\
                     patch.object(installer,'_upgrade_code',side_effect=swap) as upgrade:
                    if failure is None:
                        installer.upgrade_idle_ceccb_v1(root/'candidate')
                        upgrade.assert_called_once_with(root/'candidate',installed,1000,
                            installer.CECCB_INSTALLED_BINDING,'idle-ceccb-v1','ceccb')
                    else:
                        with self.assertRaises(Blocked):installer.upgrade_idle_ceccb_v1(root/'candidate')
                        if failure=='unknown':upgrade.assert_not_called()
                    self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

    def test_actual_source_capture_idle_update_uses_only_65717_predecessor(self):
        import installer
        from types import SimpleNamespace
        helper=RetirementActionTests()
        for failure in (None,'unknown','history'):
            with self.subTest(failure=failure),tempfile.TemporaryDirectory() as folder:
                fixture=helper.fixture(folder);kube=FakeKube(fixture[4]);helper.action(fixture,kube)
                root,base,_,_,_,binding,_,_=fixture;installed=root/'installed';(installed/'recovery').mkdir()
                for name in ('recovery-key.pem','recovery-cert.pem'):
                    path=installed/'recovery'/name;path.write_bytes(b'protected existing key material');path.chmod(0o600)
                policy=installed/'policy.json';policy.write_text('{}');policy.chmod(0o600)
                spec=retirement.SOURCE_CAPTURE_REJECTION
                request={'action':'prepare','nonce':spec['nonce'],'source_sha':spec['source_sha'],
                    'run_id':spec['run_id'],'mode':'full','changed_services':[]}
                row={'schema':'voice-nats-bridge-request-v1','request_sha256':retirement.canonical(request),
                    'request':request,'phase':'STARTED','prepare_stage':{'name':'source-capture','status':'STARTED'},
                    'prepare_error':'source_authority_rejected'}
                journal=installed/'journal'/(spec['nonce']+'.json')
                journal.write_bytes(json.dumps(row,sort_keys=True,separators=(',',':')).encode());journal.chmod(0o600)
                response=installed/'responses'/journal.name
                response.write_bytes(b'{"error":"blocked_unclassified","status":"BLOCKED"}');response.chmod(0o440)
                (installed/'sources').chmod(0o700)
                (installed/'sources'/spec['operation']).mkdir(mode=0o700)
                if failure=='unknown':
                    path=installed/'journal'/('e'*64+'.json');path.write_text('{"phase":"STARTED"}');path.chmod(0o600)
                def swap(*args):
                    if failure=='history':
                        state=json.loads((base/'checkpoint.json').read_bytes());state['authorization']={'expires_at':'changed'}
                        (base/'checkpoint.json').write_text(json.dumps(state))
                with patch.object(retirement.guard,'ROOT',root),\
                     patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(fixture[3]).hexdigest()),\
                     patch.object(retirement.root_cli,'code_binding',return_value=binding),\
                     patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()),\
                     patch.object(installer,'Kube',return_value=kube),\
                     patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)),\
                     patch('story_witness.registry_snapshot',return_value={'fixed':'witness'}),\
                     patch.object(installer,'_upgrade_code',side_effect=swap) as upgrade:
                    if failure is None:
                        installer.upgrade_idle_source_capture_v1(root/'candidate')
                        upgrade.assert_called_once_with(root/'candidate',installed,1000,
                            installer.SOURCE_CAPTURE_INSTALLED_BINDING,'idle-source-capture-v1','source-65717')
                    else:
                        with self.assertRaises(Blocked):installer.upgrade_idle_source_capture_v1(root/'candidate')
                        if failure=='unknown':upgrade.assert_not_called()
                    self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'owned Linux root fixture')
class FixedSourceCaptureRejectionTests(unittest.TestCase):
    def fixture(self,folder):
        import hashlib,json
        from pathlib import Path
        spec=retirement.SOURCE_CAPTURE_REJECTION
        installed=Path(folder)/'installed';journal_dir=installed/'journal';response_dir=installed/'responses'
        sources=installed/'sources';workspace=sources/spec['operation']
        installed.mkdir(mode=0o700)
        for path in (journal_dir,response_dir,sources,workspace):path.mkdir(mode=0o700)
        request={'action':'prepare','nonce':spec['nonce'],'source_sha':spec['source_sha'],
            'run_id':spec['run_id'],'mode':'full','changed_services':[]}
        row={'schema':'voice-nats-bridge-request-v1','request_sha256':retirement.canonical(request),
            'request':request,'phase':'STARTED','prepare_stage':{'name':'source-capture','status':'STARTED'},
            'prepare_error':'source_authority_rejected'}
        journal_raw=json.dumps(row,sort_keys=True,separators=(',',':')).encode()
        response={'status':'BLOCKED','error':'blocked_unclassified'}
        response_raw=json.dumps(response,sort_keys=True,separators=(',',':')).encode()
        self.assertEqual(hashlib.sha256(journal_raw).hexdigest(),spec['journal_sha256'])
        self.assertEqual(hashlib.sha256(response_raw).hexdigest(),spec['response_sha256'])
        path=journal_dir/(spec['nonce']+'.json');path.write_bytes(journal_raw);path.chmod(0o600)
        response_path=response_dir/path.name;response_path.write_bytes(response_raw);response_path.chmod(0o440)
        return installed,path,row,journal_raw,response_raw,workspace

    def test_exact_precheckpoint_source_failure_is_admitted_and_excluded_from_old_inventory(self):
        from pathlib import Path
        with tempfile.TemporaryDirectory() as folder:
            installed,path,row,journal_raw,response_raw,workspace=self.fixture(folder)
            root=Path(folder)/'root';root.mkdir()
            def private(target,*args,**kwargs):
                target=Path(target)
                if target==path:return journal_raw
                if target==installed/'responses'/path.name:return response_raw
                raise AssertionError(target)
            with patch.object(retirement,'private_bytes',side_effect=private),\
                 patch.object(retirement,'document',return_value=row),patch.object(retirement.guard,'ROOT',root):
                witness=retirement.pending_request_accepted(installed,path,{'journals':{}})
                self.assertEqual(witness,retirement._source_capture_rejection_workspace(installed))
                self.assertEqual(retirement.pending_abandonment_names(installed),set())

    def test_source_failure_disposition_rejects_drift_and_new_operation_artifacts(self):
        from pathlib import Path
        for drift in ('stage','workspace','checkpoint','build'):
            with self.subTest(drift=drift),tempfile.TemporaryDirectory() as folder:
                installed,path,row,journal_raw,response_raw,workspace=self.fixture(folder)
                root=Path(folder)/'root';root.mkdir()
                if drift=='workspace':(workspace/'unexpected').write_text('not empty')
                if drift=='checkpoint':
                    base=root/('rollout-'+retirement.SOURCE_CAPTURE_REJECTION['operation']);base.mkdir()
                    (base/'checkpoint.json').write_text('{}')
                if drift=='build':
                    (installed/'sources'/(retirement.SOURCE_CAPTURE_REJECTION['operation']+'-build')).mkdir()
                if drift=='stage':row['prepare_stage']={'name':'target-build','status':'COMPLETE'}
                changed=json.dumps(row,sort_keys=True,separators=(',',':')).encode()
                def private(target,*args,**kwargs):
                    target=Path(target)
                    if target==path:return changed if drift=='stage' else journal_raw
                    if target==installed/'responses'/path.name:return response_raw
                    raise AssertionError(target)
                with patch.object(retirement,'private_bytes',side_effect=private),\
                     patch.object(retirement,'document',return_value=row),patch.object(retirement.guard,'ROOT',root):
                    with self.assertRaises(Blocked):retirement.pending_request_accepted(installed,path,{'journals':{}})

class IdleUpgradeRemainderTests(unittest.TestCase):
    def test_ceccb_unclosed_operation_vetoes_despite_old_closure_and_complete_journal(self):
        import installer
        from types import SimpleNamespace
        helper=RetirementActionTests()
        with tempfile.TemporaryDirectory() as folder:
            fixture=helper.fixture(folder);kube=FakeKube(fixture[4]);helper.action(fixture,kube)
            root,_,_,_,_,binding,_,_=fixture;installed=root/'installed'
            # Keep the historical closure and add a separate unclosed d4-style operation.
            operation='d4d4d4d4d4d4';base=root/('rollout-'+operation);base.mkdir()
            checkpoint={'schema':'nats-rollout-root-v1','operation':operation,
                'phase':'PREPARE','status':'BLOCKED','fence_status':'VERIFIED','events':[]}
            (base/'checkpoint.json').write_text(json.dumps(checkpoint));(base/'checkpoint.json').chmod(0o600)
            request_name=operation+'a'*52+'.json'
            (installed/'journal'/request_name).write_text('{"phase":"COMPLETE"}')
            (installed/'journal'/request_name).chmod(0o600)
            (installed/'responses'/request_name).write_text('{"status":"BLOCKED"}')
            (installed/'responses'/request_name).chmod(0o640)
            (installed/'recovery').mkdir()
            for name in ('recovery-key.pem','recovery-cert.pem'):
                (installed/'recovery'/name).write_bytes(b'protected existing key material')
                (installed/'recovery'/name).chmod(0o600)
            policy=installed/'policy.json';policy.write_text('{}');policy.chmod(0o600)
            with patch.object(retirement.guard,'ROOT',root),\
                 patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(fixture[3]).hexdigest()),\
                 patch.object(retirement.root_cli,'code_binding',return_value=binding),\
                 patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()),\
                 patch.object(installer,'Kube',return_value=kube),\
                 patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)),\
                 patch('story_witness.registry_snapshot',return_value={'fixed':'witness'}),\
                 patch.object(installer,'_upgrade_code') as upgrade:
                with self.assertRaises(Blocked):
                    installer.upgrade_idle_ceccb_v1(root/'candidate')
            upgrade.assert_not_called()
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


class FailedReadOnlyClosureTests(unittest.TestCase):
    def fixture(self,folder):
        root=Path(folder);operation='669fd496166b';base=root/('rollout-'+operation);base.mkdir()
        installed=root/'installed';(installed/'journal').mkdir(parents=True);(installed/'responses').mkdir()
        nonce=operation+'a'*52
        request={'action':'prepare','mode':'full','nonce':nonce,'source_sha':'a'*40,
            'changed_services':[],'run_id':1}
        row={'phase':'STARTED','prepare_error':'guard_rejected','request':request,
             'request_sha256':retirement.canonical(request)}
        response={'status':'BLOCKED','error':'exception_Blocked'}
        for path,value in ((installed/'journal'/(nonce+'.json'),row),
                           (installed/'responses'/(nonce+'.json'),response)):
            path.write_text(json.dumps(value,sort_keys=True,separators=(',',':')));path.chmod(0o600)
        binding={'member':'a'*64}
        state={'schema':'nats-rollout-root-v1','operation':operation,'phase':'READ_ONLY_CAPTURE',
               'status':'RUNNING','fence_status':'UNKNOWN','events':[{'kind':'staging_preflight'}],
               'code_capture':binding,'target':{'mode':'full','immutable':'source'},'input_target':{'mode':'full','immutable':'source'},
               'contract':{'immutable':'contract'}}
        return root,base,state,binding,nonce

    def test_automatic_no_effect_failure_authenticates_idle_and_pending_request(self):
        with tempfile.TemporaryDirectory() as folder:
            root,base,state,binding,nonce=self.fixture(folder)
            with patch.object(retirement.guard,'ROOT',root),patch.object(retirement.root_cli,'code_binding',return_value=binding):
                result=retirement.record_readonly_failure(base,state,Blocked('rollout_old_container_attempt_missing'))
                self.assertEqual((result['phase'],result['status'],result['fence_status']),
                    ('FAILED_READ_ONLY','BLOCKED','NOT_ACQUIRED'))
                self.assertEqual(result['events'][:-1],state['events'])
                self.assertEqual(result['events'][-1]['guard'],'rollout_old_container_attempt_missing')
                self.assertEqual(state['phase'],'READ_ONLY_CAPTURE')
                (base/'checkpoint.json').write_text(json.dumps(result));(base/'checkpoint.json').chmod(0o600)
                closure=retirement.verify_readonly_terminal(base,result)
                self.assertTrue(retirement.idle_state(base,result))
                retirement.pending_request_accepted(root/'installed',root/'installed/journal'/(nonce+'.json'),closure)
                for fault in ('scale','cut','context_marker','snapshot','journal','response'):
                    changed=copy.deepcopy(result)
                    if fault=='scale':changed['events'].insert(0,{'kind':'workload_scaled','name':'voice-user','replicas':0})
                    elif fault=='cut':changed['cut']={'unexpected':True}
                    elif fault=='context_marker':changed['context']={'marker':{'data':{'phase':'rollout-capturing'}}}
                    elif fault=='snapshot':changed['contract']={'changed':True}
                    elif fault=='journal':
                        path=root/'installed/journal'/(nonce+'.json');raw=path.read_bytes();row=json.loads(raw);row['request']['nonce']='b'*64;path.write_text(json.dumps(row))
                    else:
                        path=root/'installed/responses'/(nonce+'.json');raw=path.read_bytes();path.write_text(json.dumps({'status':'PASS'}))
                    with self.subTest(fault=fault),self.assertRaises(Blocked):retirement.verify_readonly_terminal(base,changed)
                    if fault in ('journal','response'):path.write_bytes(raw)

    def test_mutation_evidence_or_arbitrary_error_text_never_becomes_a_readonly_closure(self):
        with tempfile.TemporaryDirectory() as folder:
            root,base,state,binding,nonce=self.fixture(folder)
            for key in ('authorization','cut','cipher_binding','custody','nats_migration','apply','restart'):
                changed=copy.deepcopy(state);changed[key]={}
                with self.subTest(key=key),self.assertRaises(Blocked):
                    retirement.record_readonly_failure(base,changed,Blocked('rollout_old_image_ambiguous'))
            with patch.object(retirement.guard,'ROOT',root),patch.object(retirement.root_cli,'code_binding',return_value=binding):
                result=retirement.record_readonly_failure(base,state,RuntimeError('do not persist arbitrary private text'))
                self.assertEqual(result['events'][-1]['guard'],'unclassified_readonly_failure')
                self.assertNotIn('private text',json.dumps(result))


    def test_failed_rollback_binds_new_nonce_and_distinct_prior_without_fake_mode(self):
        with tempfile.TemporaryDirectory() as folder:
            root,base,state,binding,nonce=self.fixture(folder)
            state['target']['mode']=state['input_target']['mode']='images-only'
            path=root/'installed/journal'/(nonce+'.json');row=json.loads(path.read_bytes())
            request={'action':'prepare-rollback','operation':'123456abcdef','nonce':nonce,
                'run_id':1}
            row['request']=request;row['request_sha256']=retirement.canonical(request)
            path.write_text(json.dumps(row));path.chmod(0o600)
            with patch.object(retirement.guard,'ROOT',root),patch.object(retirement.root_cli,'code_binding',return_value=binding):
                result=retirement.record_readonly_failure(base,state,Blocked('rollout_old_container_attempt_missing'))
                retirement.verify_readonly_terminal(base,result)
                for fault in ('same_prior','malformed_prior','invented_mode','wrong_target_mode'):
                    changed=copy.deepcopy(row)
                    if fault=='same_prior':changed['request']['operation']=state['operation']
                    elif fault=='malformed_prior':changed['request']['operation']='not-an-operation'
                    elif fault=='invented_mode':changed['request']['mode']='images-only'
                    else:changed['request']['action']='prepare';changed['request'].pop('operation');changed['request'].update(mode='full',source_sha='a'*40,changed_services=[])
                    changed['request_sha256']=retirement.canonical(changed['request']);path.write_text(json.dumps(changed))
                    with self.subTest(fault=fault),self.assertRaises(Blocked):retirement.verify_readonly_terminal(base,result)
                path.write_text(json.dumps(row))


class AppliedRestartClosureTests(unittest.TestCase):
    def fixture(self):
        helper=PostmigrationClosureTests();snapshot,receipt,_=helper.fixture();self.files=helper.files
        snapshot.update(phase='RESTART',status='BLOCKED',error='restart_readiness_timeout',
            preservation={'native_files_verified':True,'census_verified':True},
            nonnats_applied={'verified':True},closed_preservation_attempt_nonce='a'*64,closed_preservation_attempts={'a'*64:{'retained':True}})
        snapshot['context']['marker']['data']['phase']='rollout-verified'
        names=set(snapshot['context']['snapshots'])
        for name,row in snapshot['context']['snapshots'].items():
            row['spec']['template']['metadata']={'annotations':{'approved-target':'immutable'}}
        snapshot['target']['template_hashes']={n:retirement.canonical(r['spec']['template']) for n,r in snapshot['context']['snapshots'].items() if n!=retirement.HUB}
        resources=json.loads(self.files.pop('operator-postmigration-abort-resources.json'))
        for key,row in resources.items():
            if row['kind']=='Deployment':row['spec']=copy.deepcopy(snapshot['context']['snapshots'][row['metadata']['name']]['spec'])
        self.files['operator-applied-abort-resources.json']=json.dumps(resources).encode()
        snapshot['events'] += [{'kind':'rollout_target_adopted'}]+[{'kind':'workload_scaled','name':n,'replicas':0} for n in sorted(names)]
        receipt.update(schema='voice-operator-applied-abort-closure-v1',snapshot_sha256=retirement.canonical(snapshot),
            configuration_basis='RETAIN_APPLIED_TARGET_CONFIGURATION_AND_FORWARD_MIGRATIONS',
            resource_disposition_sha256=hashlib.sha256(self.files['operator-applied-abort-resources.json']).hexdigest())
        receipt['before_marker']['data']=copy.deepcopy(snapshot['context']['marker']['data'])
        receipt['consumed_claim']['resourceVersion']='100'
        receipt['setup']={n:{'uid':r['metadata']['uid'],'template_sha256':retirement.canonical(r['spec']['template']),'replicas':1,'ready':n==retirement.HUB} for n,r in snapshot['context']['snapshots'].items()}
        receipt['events']=[{'kind':'operator_applied_abort_intent','waiver':retirement.WAIVER,'snapshot_sha256':receipt['snapshot_sha256']}]+[{'kind':'workload_scaled','name':n,'replicas':1} for n in [retirement.HUB,*sorted(names-{retirement.HUB})]]+[{'kind':'operator_applied_abort_completed'}]
        nonce='c'*64;request={'action':'finish','operation':snapshot['operation'],'nonce':nonce,'claim_rv':'100'}
        self.journal={'phase':'STARTED','prepare_error':'guard_rejected','request':request,'request_sha256':retirement.canonical(request)}
        self.response={'status':'BLOCKED','error':'exception_Blocked'}
        receipt['journals']={nonce+'.json':{'journal_sha256':retirement.canonical(self.journal),'request_sha256':retirement.canonical(request),'response_sha256':retirement.canonical(self.response)}}
        return snapshot,receipt,self.terminal(snapshot,receipt)

    def terminal(self,snapshot,receipt):
        result=copy.deepcopy(snapshot);result.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',events=snapshot['events']+receipt['events'])
        result['operator_applied_abort']={'schema':'voice-operator-applied-abort-reference-v1','snapshot_sha256':receipt['snapshot_sha256'],'receipt_sha256':retirement.canonical(receipt)}
        return result

    def verify(self,snapshot,receipt,state):
        base=Path('/private/rollout-'+snapshot['operation'])
        def read(path,*args,**kwargs):
            path=Path(path)
            if path.name=='operator-applied-abort-snapshot.json':return json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()
            if path.parent.name in ('journal','responses'):return json.dumps(self.journal if path.parent.name=='journal' else self.response,sort_keys=True,separators=(',',':')).encode()
            return self.files[path.name]
        with patch.object(retirement,'private_bytes',side_effect=read),patch.object(retirement,'document',return_value=receipt),patch.object(retirement.root_cli,'code_binding',return_value=snapshot['code_capture']),patch.object(retirement,'_retained_mode',return_value=0o600):
            return retirement.verify_applied_terminal(base,state)

    def test_applied_failed_restart_retains_target_forward_data_and_original_authority(self):
        snapshot,receipt,state=self.fixture();self.verify(snapshot,receipt,state)
        self.assertNotIn('restart',snapshot)
        self.assertEqual(state['authorization'],snapshot['authorization'])
        self.assertEqual(state['context'],snapshot['context'])
        self.assertEqual(state['events'][:len(snapshot['events'])],snapshot['events'])
        self.assertFalse(receipt['deployment_pass']);self.assertFalse(receipt['rollback_eligible'])

    def test_no_pass_original_target_rewrite_proof_marker_or_journal_drift(self):
        for fault in ('pass','target','proof','marker','claim','journal','history','ready','expired_rewrite','file'):
            snapshot,receipt,state=self.fixture()
            if fault=='pass':receipt['deployment_pass']=True
            elif fault=='target':snapshot['target']['template_hashes']['voice-story']='f'*64
            elif fault=='proof':snapshot.pop('preservation')
            elif fault=='marker':receipt['before_marker']['data']['phase']='rollout-applying'
            elif fault=='claim':receipt['consumed_claim']['resourceVersion']='101'
            elif fault=='journal':self.journal['request']['operation']='123456abcdef'
            elif fault=='history':state['events'][0]['kind']='changed'
            elif fault=='ready':receipt['setup'][retirement.HUB]['ready']=False
            elif fault=='expired_rewrite':state['authorization']['expires_at']='renewed'
            else:self.files['rollout-backup.cms']+=b'changed'
            receipt['snapshot_sha256']=retirement.canonical(snapshot)
            receipt['events'][0]['snapshot_sha256']=receipt['snapshot_sha256']
            if fault not in ('history','expired_rewrite'):state=self.terminal(snapshot,receipt)
            with self.subTest(fault=fault),self.assertRaises((Blocked,KeyError)):
                self.verify(snapshot,receipt,state)


    def test_operator_closure_preserves_distinct_later_apply_authority_deadline(self):
        snapshot,receipt,_=self.fixture()
        snapshot['authorization']['expires_at']='2026-10-11T12:34:11.549999+00:00'
        snapshot['nats_contract_expires_at']='2026-10-11T12:26:51.376312+00:00'
        receipt['observed_at']='2026-10-11T10:36:48.233176+00:00'
        self.files['apply-authorization.json']=json.dumps(snapshot['authorization']).encode()
        raw=self.files['apply-authorization.json']
        receipt['retained_files']['apply-authorization.json'].update(sha256=hashlib.sha256(raw).hexdigest(),bytes=len(raw))
        receipt['consumed_claim'].update(authorization_sha256=hashlib.sha256(raw).hexdigest(),expires_at=snapshot['authorization']['expires_at'])
        receipt['snapshot_sha256']=retirement.canonical(snapshot)
        receipt['events'][0]['snapshot_sha256']=receipt['snapshot_sha256']
        state=self.terminal(snapshot,receipt);self.verify(snapshot,receipt,state)
        self.assertEqual(state['authorization']['expires_at'],snapshot['authorization']['expires_at'])
        self.assertEqual(state['nats_contract_expires_at'],snapshot['nats_contract_expires_at'])
        self.assertFalse(receipt['deployment_pass']);self.assertFalse(receipt['rollback_eligible'])


class FenceAbortClosureTests(unittest.TestCase):
    def fixture(self):
        import hashlib,json
        helper=PrecutClosureTests();snapshot,_,_=helper.fixture()
        op='32151c4cb431';names=retirement.CORE|{'voice-web'}
        original=snapshot['context']['original_snapshots'];captured=snapshot['context']['snapshots']
        original={n:r for n,r in original.items() if n in names};captured={n:r for n,r in captured.items() if n in names}
        for row in captured.values():row['spec']['replicas']=0
        marker=snapshot['context']['marker'];marker['data'].update(phase='rollout-capturing',knownRolloutOperation=op,
            generation='generation-1',dataPVC='voice-nats-pvc-candidate')
        snapshot.update(operation=op,phase='FENCE',status='BLOCKED',fence_status='UNKNOWN',
            code_capture={'capture-manifest.json':'b'*64},target={'mode':'images-only','tag':'e2af6b043b9afa44ea00fb8e564b04ed84e9627a'},
            events=[{'kind':'staging_preflight'},{'kind':'rollout_original_capture'},{'kind':'rollout_maintenance_owned'}]+
                [{'kind':'workload_scaled','name':n,'replicas':0} for n in sorted(names)]+
                [{'kind':'original_hub_shutdown_intent','pod_uid':'c8ac408a-ef62-451b-a8c1-5a4393ec271a',
                  'container_id':'containerd://'+'1'*64,'finalizer':'voice.io/nats-preservation-32151c4cb431',
                  'live_census_sha256':'2'*64},
                 {'kind':'original_hub_orderly_exit_verified','proof':{'pod_uid':'c8ac408a-ef62-451b-a8c1-5a4393ec271a',
                    'schema':'voice-original-hub-orderly-exit-v1','container_id':'containerd://'+'1'*64,
                    'restart_count':0,'started_at':'2026-10-13T10:00:00Z','finished_at':'2026-10-13T10:01:00Z',
                    'exit_code':0,'source':'KUBELET_CURRENT_TERMINATED_RETAINED_POD'} }])
        snapshot['context']['original_snapshots']=original;snapshot['context']['snapshots']=captured
        claim_name='voice-nats-jsdata-d202610040049430b'
        expected=snapshot['context']['expected'];expected.update(namespace_uid='ac7abaf3-241f-483f-ac33-71086e4dfd32',
            source_claim=claim_name,source_claim_uid='claim-uid',source_pv_uid='pv-uid',generation='generation-1')
        expected['deployment_uids']={n:r['metadata']['uid'] for n,r in original.items()}
        marker['data']['dataPVC']=expected['source_claim']
        snapshot['context']['final_claim'].setdefault('metadata',{}).update(name=claim_name,uid='claim-uid')
        snapshot['context']['final_pv'].setdefault('metadata',{})['uid']='pv-uid'
        snapshot_raw=json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode();snapshot_sha=hashlib.sha256(snapshot_raw).hexdigest()
        nonce='32151c4cb4316a31c57949222e0cf69208c8cd82a0bf32e307c371d34c9b2a59'
        request={'action':'prepare','changed_services':['web'],'mode':'images-only','nonce':nonce,
            'run_id':38380518143,'source_sha':'e2af6b043b9afa44ea00fb8e564b04ed84e9627a'}
        journal={'schema':'voice-nats-bridge-request-v1','request_sha256':retirement.canonical(request),'request':request,
            'phase':'STARTED','prepare_stage':{'name':'target-build','status':'COMPLETE'},'prepare_error':'guard_rejected'}
        response={'status':'BLOCKED','error':'command_failed'}
        jname=nonce+'.json';jraw=json.dumps(journal,sort_keys=True,separators=(',',':')).encode()
        rraw=json.dumps(response,sort_keys=True,separators=(',',':')).encode()
        jsha=hashlib.sha256(jraw).hexdigest();rsha=hashlib.sha256(rraw).hexdigest()
        finals=['kubernetes.io/pvc-protection','voice.io/nats-preservation-32151c4cb431']
        receipt={'schema':'voice-operator-fence-abort-closure-v1','operation':op,'snapshot_sha256':snapshot_sha,
            'code_binding_sha256':retirement.canonical(snapshot['code_capture']),'deployment_pass':False,'rollback_eligible':False,
            'authorization':'EXPLICIT_USER_AUTHORIZED_DISPOSABLE_STAGING_RECOVERY',
            'pod_finalizer':{'pod_uid':'c8ac408a-ef62-451b-a8c1-5a4393ec271a','finalizer':finals[-1],
                'container_id':'containerd://'+'1'*64,'exit_code':0,'before':finals,'after':finals[:-1],
                'resource_version_before':'301','resource_version_after':'302'},
            'storage':{'namespace_uid':expected['namespace_uid'],'claim_uid':'claim-uid','pv_uid':'pv-uid',
                'claim_spec_sha256':retirement.canonical(snapshot['context']['final_claim']['spec']),
                'pv_spec_sha256':retirement.canonical(snapshot['context']['final_pv']['spec']),
                'path':snapshot['context']['final_path']},
            'restored':{n:{'uid':r['metadata']['uid'],'template_sha256':retirement.canonical(r['spec']['template']),
                'replicas':1,'ready':True} for n,r in original.items()},
            'restore_sequence':['voice-nats-pvc-candidate','voice-user-space-cycle','voice-user-template-restored',
                *sorted(names-{'voice-nats-pvc-candidate','voice-user','voice-space','voice-web','voice-gateway'}),
                'voice-web','voice-gateway'],
            'before_marker':{'uid':marker['metadata']['uid'],'resourceVersion':marker['metadata']['resourceVersion'],'data':marker['data']},
            'released_marker':{'uid':marker['metadata']['uid'],'resourceVersion':'124',
                'data':{k:v for k,v in dict(marker['data'],phase='active').items() if k!='knownRolloutOperation'}},
            'events':[{'kind':'operator_fence_abort_intent'},
                {'kind':'operator_nats_finalizer_removed','pod_uid':'c8ac408a-ef62-451b-a8c1-5a4393ec271a','finalizer':finals[-1]},
                *[{'kind':'operator_workload_restored','name':n,'uid':r['metadata']['uid'],
                    'template_sha256':retirement.canonical(r['spec']['template']),'replicas':1,'ready':True}
                    for n,r in sorted(original.items())],
                {'kind':'operator_marker_released','uid':marker['metadata']['uid']},
                {'kind':'operator_fence_abort_completed'}],
            'journals':{jname:{'journal_sha256':jsha,'request_sha256':journal['request_sha256'],'response_sha256':rsha}}}
        state=copy.deepcopy(snapshot);state.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',
            events=snapshot['events']+receipt['events'],operator_fence_abort={'schema':'voice-operator-fence-abort-reference-v1',
                'snapshot_sha256':snapshot_sha,'receipt_sha256':retirement.canonical(receipt)})
        return snapshot,receipt,state,journal,response

    def verify(self,snapshot,receipt,state,journal,response):
        import hashlib,json
        base=Path('/private/rollout-'+snapshot['operation'])
        jname=next(iter(receipt['journals']))
        def private(path,*args,**kwargs):
            path=Path(path)
            if path.name=='operator-fence-abort-snapshot.json':return json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()
            if path.parent.name=='journal':return json.dumps(journal,sort_keys=True,separators=(',',':')).encode()
            if path.parent.name=='responses':return json.dumps(response,sort_keys=True,separators=(',',':')).encode()
            raise AssertionError(path)
        with patch.object(retirement,'private_bytes',side_effect=private),patch.object(retirement,'document',return_value=receipt),\
             patch.object(retirement.root_cli,'code_binding',return_value=snapshot['code_capture']),\
             patch.object(retirement,'FENCE_ABORT_CHECKPOINT_SHA256',hashlib.sha256(json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()).hexdigest()),\
             patch.object(retirement,'FENCE_ABORT_JOURNAL_SHA256',hashlib.sha256(json.dumps(journal,sort_keys=True,separators=(',',':')).encode()).hexdigest()),\
             patch.object(retirement,'FENCE_ABORT_RESPONSE_SHA256',hashlib.sha256(json.dumps(response,sort_keys=True,separators=(',',':')).encode()).hexdigest()):
            return retirement.verify_fence_abort_terminal(base,state)

    def admit_pending(self,snapshot,receipt,state,journal,response):
        import hashlib,json
        installed=Path('/private/installed');name=retirement.FENCE_ABORT_NONCE+'.json'
        def document(path):
            path=Path(path)
            if path.name==name:return journal
            if path.name=='checkpoint.json':return state
            if path.name=='operator-fence-abort-receipt.json':return receipt
            raise AssertionError(path)
        def private(path,*args,**kwargs):
            path=Path(path)
            if path.name=='operator-fence-abort-snapshot.json':return json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()
            if path.parent.name=='journal':return json.dumps(journal,sort_keys=True,separators=(',',':')).encode()
            if path.parent.name=='responses':return json.dumps(response,sort_keys=True,separators=(',',':')).encode()
            raise AssertionError(path)
        with patch.object(retirement,'private_bytes',side_effect=private),patch.object(retirement,'document',side_effect=document),\
             patch.object(retirement,'FENCE_ABORT_CHECKPOINT_SHA256',hashlib.sha256(json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()).hexdigest()),\
             patch.object(retirement,'FENCE_ABORT_JOURNAL_SHA256',hashlib.sha256(json.dumps(journal,sort_keys=True,separators=(',',':')).encode()).hexdigest()),\
             patch.object(retirement,'FENCE_ABORT_RESPONSE_SHA256',hashlib.sha256(json.dumps(response,sort_keys=True,separators=(',',':')).encode()).hexdigest()),\
             patch.object(retirement.guard,'ROOT',Path('/private')),patch.object(retirement.root_cli,'code_binding',return_value=snapshot['code_capture']):
            return retirement.pending_request_accepted(installed,installed/'journal'/name,receipt)

    def test_exact_fence_abort_closure_is_idle_and_preserves_unknown_historical_fence(self):
        snapshot,receipt,state,journal,response=self.fixture()
        self.assertEqual(self.verify(snapshot,receipt,state,journal,response)['schema'],'voice-operator-fence-abort-closure-v1')
        self.assertEqual(snapshot['fence_status'],'UNKNOWN');self.assertEqual(state['fence_status'],'RELEASED')
        self.assertFalse(receipt['deployment_pass']);self.assertFalse(receipt['rollback_eligible'])

    def test_unavailable_patch_response_requires_exact_uid_absence_and_never_fabricates_resource_version(self):
        snapshot,receipt,state,journal,response=self.fixture()
        receipt['schema']='voice-operator-fence-abort-closure-v2'
        pod=receipt['pod_finalizer']
        pod.update(outcome='CAS_RESPONSE_UNAVAILABLE_UID_ABSENT',before=None,after=None,
            resource_version_before=None,resource_version_after=None,
            pod_absence_observation={'namespace_uid':'ac7abaf3-241f-483f-ac33-71086e4dfd32',
                'pod_name':'voice-nats-pvc-candidate-cf9757747-dgktg',
                'pod_uid':'c8ac408a-ef62-451b-a8c1-5a4393ec271a'})
        receipt['events'][1]={'kind':'operator_nats_pod_uid_absence_verified','cas_response':'UNAVAILABLE',
            **pod['pod_absence_observation']}
        state=retirement.fence_abort_terminal(snapshot,receipt)
        self.assertEqual(state['operator_fence_abort']['schema'],'voice-operator-fence-abort-reference-v2')
        self.assertEqual(self.verify(snapshot,receipt,state,journal,response)['schema'],'voice-operator-fence-abort-closure-v2')
        self.assertIsNone(self.admit_pending(snapshot,receipt,state,journal,response))
        for fault in ('invented_resource_version','wrong_uid','wrong_namespace','wrong_event'):
            bad_snapshot,bad_receipt,bad_state,journal,response=self.fixture()
            bad_receipt['schema']='voice-operator-fence-abort-closure-v2'
            bad_pod=bad_receipt['pod_finalizer']
            bad_pod.update(outcome='CAS_RESPONSE_UNAVAILABLE_UID_ABSENT',before=None,after=None,
                resource_version_before=None,resource_version_after=None,
                pod_absence_observation={'namespace_uid':'ac7abaf3-241f-483f-ac33-71086e4dfd32',
                    'pod_name':'voice-nats-pvc-candidate-cf9757747-dgktg',
                    'pod_uid':'c8ac408a-ef62-451b-a8c1-5a4393ec271a'})
            bad_receipt['events'][1]={'kind':'operator_nats_pod_uid_absence_verified','cas_response':'UNAVAILABLE',
                **bad_pod['pod_absence_observation']}
            if fault=='invented_resource_version':bad_pod['resource_version_after']='999999'
            elif fault=='wrong_uid':bad_pod['pod_absence_observation']['pod_uid']='foreign'
            elif fault=='wrong_namespace':bad_pod['pod_absence_observation']['namespace_uid']='foreign'
            else:bad_receipt['events'][1]['kind']='operator_nats_finalizer_removed'
            bad_state=retirement.fence_abort_terminal(bad_snapshot,bad_receipt)
            with self.subTest(fault=fault),self.assertRaises((Blocked,KeyError,FileNotFoundError)):
                self.verify(bad_snapshot,bad_receipt,bad_state,journal,response)

    def test_pending_request_admission_is_exact_and_closure_drift_is_rejected(self):
        snapshot,receipt,state,journal,response=self.fixture()
        self.assertIsNone(self.admit_pending(snapshot,receipt,state,journal,response))
        for fault in ('pass','pod','restored','marker','sequence','journal','response','request','fence'):
            snapshot,receipt,state,journal,response=self.fixture()
            if fault=='pass':receipt['deployment_pass']=True
            elif fault=='pod':receipt['pod_finalizer']['pod_uid']='foreign'
            elif fault=='restored':receipt['restored']['voice-user']['ready']=False
            elif fault=='marker':receipt['released_marker']['data']['knownRolloutOperation']=snapshot['operation']
            elif fault=='sequence':receipt['restore_sequence'][-1]='gateway-before-web'
            elif fault=='journal':journal['prepare_error']='unexpected'
            elif fault=='response':response['error']='different'
            elif fault=='request':journal['request']['changed_services']=['gateway']
            else:state['fence_status']='VERIFIED'
            receipt['snapshot_sha256']=retirement.canonical(snapshot)
            state['operator_fence_abort'].update(snapshot_sha256=receipt['snapshot_sha256'],receipt_sha256=retirement.canonical(receipt))
            with self.subTest(fault=fault),self.assertRaises((Blocked,KeyError,FileNotFoundError)):
                self.verify(snapshot,receipt,state,journal,response)


class PendingAbandonmentTests(unittest.TestCase):
    def test_verified_reset_authorization_preserves_legacy_pending_inventory(self):
        import hashlib,json,tempfile
        from pathlib import Path
        from unittest.mock import patch
        import operator_reset
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);installed=root/'installed'
            (installed/'journal').mkdir(parents=True);(installed/'responses').mkdir()
            base=root/('rollout-'+operator_reset.OPERATION);base.mkdir()
            (base/'checkpoint.json').write_text('{}',encoding='utf-8')
            (base/'checkpoint.json').chmod(0o600)
            legacy={format(n,'064x')+'.json' for n in range(29)}
            def write(path,value):
                raw=json.dumps(value,sort_keys=True,separators=(',',':')).encode()
                path.write_bytes(raw);path.chmod(0o600)
                return raw
            for name in legacy:
                write(installed/'journal'/name,{'phase':'STARTED','request':{'operation':'legacy'}})
            nonce='4c708f1c8fc76c129fa8088d0f08284bda7fa2283ceb9b176b9847d8f7058006'
            request={'action':'authorize','nonce':nonce,'operation':operator_reset.OPERATION}
            journal={'phase':'STARTED','request':request,'request_sha256':retirement.canonical(request),'prepare_error':'guard_rejected'}
            raw=write(installed/'journal'/(nonce+'.json'),journal)
            response=write(installed/'responses'/(nonce+'.json'),{'status':'BLOCKED','error':'exception_Blocked'})
            receipt={'journals':{nonce+'.json':{'journal_sha256':hashlib.sha256(raw).hexdigest(),
                'request_sha256':retirement.canonical(request),'response_sha256':hashlib.sha256(response).hexdigest()}}}
            before={p:p.read_bytes() for p in (installed/'journal').iterdir()}
            with patch.object(retirement.guard,'ROOT',root),patch.object(operator_reset,'verify_terminal',return_value=receipt) as verify:
                self.assertEqual(retirement.pending_abandonment_names(installed),legacy)
                verify.assert_called_once_with(base,{})
                self.assertEqual({p:p.read_bytes() for p in before},before)
                # A new unrelated request still changes the legacy inventory.
                unknown=installed/'journal'/('f'*64+'.json')
                write(unknown,{'phase':'STARTED','request':{'operation':'foreign'}})
                self.assertEqual(retirement.pending_abandonment_names(installed),legacy|{unknown.name})
                # Claiming the reset operation does not bind a new nonce to it.
                write(unknown,{'phase':'STARTED','request':{'operation':operator_reset.OPERATION}})
                with self.assertRaises(Blocked):retirement.pending_abandonment_names(installed)
                unknown.unlink()
                # Real journal/response verification remains active after closure lookup.
                write(installed/'responses'/(nonce+'.json'),{'status':'BLOCKED','error':'changed'})
                with self.assertRaises(Blocked):retirement.pending_abandonment_names(installed)
            with patch.object(retirement.guard,'ROOT',root),patch.object(operator_reset,'verify_terminal',side_effect=Blocked('invalid_reset_closure')):
                with self.assertRaises(Blocked):retirement.pending_abandonment_names(installed)

    def fixture(self):
        import hashlib,json
        nonce='a'*64;name=nonce+'.json';operation='85e0b83bb353'
        request={'action':'status','nonce':nonce,'operation':operation}
        journal={'schema':'voice-nats-bridge-request-v1','request_sha256':retirement.canonical(request),
            'request':request,'phase':'STARTED','prepare_error':'guard_rejected'}
        response={'status':'BLOCKED','error':'exception_Blocked'}
        jraw=json.dumps(journal,sort_keys=True,separators=(',',':')).encode()
        rraw=json.dumps(response,sort_keys=True,separators=(',',':')).encode()
        fence_receipt={'schema':'voice-operator-fence-abort-closure-v1','operation':retirement.FENCE_ABORT_OPERATION}
        fence={'schema':'voice-operator-fence-abort-closure-v1'}
        pins={'journal_sha256':hashlib.sha256(jraw).hexdigest(),'request_sha256':retirement.canonical(request),
            'response_sha256':hashlib.sha256(rraw).hexdigest(),'operation':operation,'build_key':operation,'checkpoint_sha256':None,
            'checkpoint_phase':None,'checkpoint_status':None,'operation_state':'CHECKPOINT_ABSENT',
            'build_present':False,'build_fingerprint':None,
            'disposition':'ABANDONED_BY_EXPLICIT_USER_AUTHORIZATION'}
        receipt={'schema':'voice-nats-pending-abandonment-v1',
            'authorization':'EXPLICIT_USER_AUTHORIZED_DISPOSABLE_STAGING_RECOVERY',
            'fence_abort':{'operation':retirement.FENCE_ABORT_OPERATION,
                'checkpoint_sha256':retirement.FENCE_ABORT_CHECKPOINT_SHA256,
                'receipt_sha256':retirement.canonical(fence_receipt)},'entries':{name:pins}}
        receipt['inventory_sha256']=retirement.canonical(receipt['entries'])
        state={'operation':retirement.FENCE_ABORT_OPERATION,'operator_fence_abort':{'schema':'reference'}}
        return name,journal,response,jraw,rraw,receipt,fence_receipt,fence,state

    def check(self,fixture,path_name=None,*,root=None,installed_root=None,changed=None,idle_failure=False,inventory_drift=False):
        import json
        from pathlib import Path
        from unittest.mock import patch
        name,journal,response,jraw,rraw,receipt,fence_receipt,fence,state=fixture
        installed=Path(installed_root) if installed_root is not None else Path('/private/installed')
        path=installed/'journal'/(path_name or name)
        def private(target,*args,**kwargs):
            target=Path(target)
            if target.name==retirement.PENDING_ABANDONMENT_NAME:
                return json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()
            if target.name=='checkpoint.json':return b'{}'
            if target.parent.name=='journal':return changed if changed is not None else jraw
            if target.parent.name=='responses':return rraw
            raise AssertionError(target)
        def document(target):
            target=Path(target)
            if target.name=='checkpoint.json' and target.parent.name.startswith('rollout-'):
                operation=journal['request']['operation']
                if target.parent.name=='rollout-'+operation:
                    return {'operation':operation,'phase':'FENCE','status':'BLOCKED'}
            if target.name=='checkpoint.json':return state
            if target.name=='operator-fence-abort-receipt.json':return fence_receipt
            raise AssertionError(target)
        root=root or Path('/private')
        with patch.object(retirement,'private_bytes',side_effect=private),patch.object(retirement,'document',side_effect=document),\
             patch.object(retirement.guard,'ROOT',root),patch.object(retirement,'verify_fence_abort_terminal',return_value=fence),\
             patch.object(retirement,'pending_abandonment_names',return_value=set(receipt['entries'])|({'c'*64+'.json'} if inventory_drift else set())),\
             patch.object(retirement,'idle_state',side_effect=Blocked('operation_not_idle') if idle_failure else None,return_value=True):
            return retirement.verify_pending_abandonment(installed,path)

    def test_exact_user_disposition_is_bound_to_321_closure_and_current_request_bytes(self):
        fixture=self.fixture();result=self.check(fixture)
        self.assertEqual(result['fence'],'voice-operator-fence-abort-closure-v1')
        self.assertEqual(result['name'],'a'*64+'.json')

    def test_unknown_request_hash_change_and_nonterminal_operation_fail_closed(self):
        fixture=self.fixture();_,_,_,jraw,_,_,_,_,_=fixture
        changed=bytearray(jraw);changed[-2]^=1
        with self.assertRaises(Blocked):self.check(fixture,changed=bytes(changed))
        with self.assertRaises(Blocked):self.check(fixture,path_name='b'*64+'.json')
        with self.assertRaises(Blocked):self.check(self.fixture(),inventory_drift=True)
        import tempfile
        from pathlib import Path
        fixture=self.fixture();name,journal=fixture[:2];pins=fixture[5]['entries'][name]
        operation=journal['request']['operation']
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);base=root/('rollout-'+operation);base.mkdir()
            checkpoint=base/'checkpoint.json';checkpoint.write_text('{}',encoding='utf-8')
            pins.update(checkpoint_sha256=__import__('hashlib').sha256(b'{}').hexdigest(),
                checkpoint_phase='FENCE',checkpoint_status='BLOCKED')
            with self.assertRaises(Blocked):self.check(fixture,root=root,idle_failure=True)

    def test_conflicting_request_and_response_operation_ids_are_rejected(self):
        import hashlib,json
        fixture=list(self.fixture());name=fixture[0];response=fixture[2]
        response['operation']='c'*12
        raw=json.dumps(response,sort_keys=True,separators=(',',':')).encode();fixture[4]=raw
        pins=fixture[5]['entries'][name];pins['response_sha256']=hashlib.sha256(raw).hexdigest()
        fixture[5]['inventory_sha256']=retirement.canonical(fixture[5]['entries'])
        with self.assertRaises(Blocked):self.check(tuple(fixture))

    def test_prebuild_only_disposition_binds_source_directory_identity(self):
        import stat,tempfile
        from pathlib import Path
        fixture=self.fixture();name,journal=fixture[:2];pins=fixture[5]['entries'][name]
        with tempfile.TemporaryDirectory() as directory:
            installed=Path(directory)/'installed';build=installed/'sources'/(journal['request']['operation']+'-build')
            build.mkdir(parents=True)
            st=build.lstat()
            fingerprint={'device':st.st_dev,'inode':st.st_ino,'mode':stat.S_IMODE(st.st_mode),
                'uid':st.st_uid,'gid':st.st_gid,'bytes':st.st_size,'mtime_ns':st.st_mtime_ns,'ctime_ns':st.st_ctime_ns}
            original_mode=stat.S_IMODE(st.st_mode)
            pins.update(operation_state='PREBUILD_ONLY',build_present=True,build_fingerprint=fingerprint)
            fixture[5]['inventory_sha256']=retirement.canonical(fixture[5]['entries'])
            self.assertEqual(self.check(fixture,installed_root=installed)['name'],name)
            (build/'changed').write_text('preserved source build state',encoding='utf-8')
            # A child write does not necessarily change directory timestamps on
            # the fixture filesystem. Also mutate a field held in the exact
            # directory fingerprint so this identity-drift assertion is stable.
            build.chmod(original_mode ^ stat.S_IXUSR)
            try:
                with self.assertRaises(Blocked):self.check(fixture,installed_root=installed)
            finally:
                build.chmod(original_mode)

    def test_absent_checkpoint_disposition_rejects_a_checkpoint_that_appears(self):
        import tempfile
        from pathlib import Path
        fixture=self.fixture();operation=fixture[1]['request']['operation']
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            self.assertEqual(self.check(fixture,root=root)['name'],'a'*64+'.json')
            base=root/('rollout-'+operation);base.mkdir()
            (base/'checkpoint.json').write_text(json.dumps({'operation':operation,'phase':'FENCE','status':'BLOCKED'}),encoding='utf-8')
            with self.assertRaises(Blocked):self.check(fixture,root=root,idle_failure=True)
