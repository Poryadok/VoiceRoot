import copy
import unittest
from unittest.mock import patch
from pathlib import Path
import tempfile
import json
from unittest.mock import Mock
import datetime as dt
import transaction
import deployment_report
from contextlib import ExitStack
import provisional as module
from controller import Blocked

class Tests(unittest.TestCase):
    def setUp(self):
        self.source={'verified':True,'source_sha':'a'*40,'run_id':123,'run_attempt':2,
            'provisional_staging':True,'lock_sha256':'b'*64,'images':{'web':'image@sha256:'+'c'*64}}
        self.execution={'event':'workflow_dispatch','path':'.github/workflows/staging-deploy.yml',
            'workflow_id':263689731,'head_sha':'a'*40,'dispatcher_run_id':456,'run_attempt':1}
        self.binding={'helper.py':'d'*64}
        self.admission=module.admission(self.source,self.execution,self.binding)
        self.state={'operation':'1'*12,'status':'PROVISIONAL_READY','phase':'POST_RESTART',
            'fence_status':'RELEASED','provisional':self.admission,'source_authority':self.source,
            'code_capture':self.binding,'target':{'tag':'a'*40,'mode':'images-only'},'events':[]}

    def test_only_explicit_dispatch_admission(self):
        self.assertEqual(self.admission['run_attempt'],2)
        for field,value in [('event','push'),('workflow_id',123),('head_sha','f'*40)]:
            execution=dict(self.execution,**{field:value})
            with self.assertRaises(Blocked):module.admission(self.source,execution,self.binding)

    def test_receipt_cannot_drop_or_mix_mode(self):
        receipt={'operation':self.state['operation'],'provisional':copy.deepcopy(self.admission)}
        module.receipt_binding(self.state,receipt)
        for mutation in ({}, {'provisional':dict(self.admission,run_attempt=1)},
                         {'provisional':self.admission,'operation':'2'*12}):
            with self.assertRaises(Blocked):module.receipt_binding(self.state,mutation)
        with self.assertRaises(Blocked):module.receipt_binding({'operation':'1'*12},receipt)

    def test_failed_acceptance_does_not_modify_checkpoint_or_state(self):
        before=copy.deepcopy(self.state)
        with patch.object(module.source_authority,'revalidate_ci',side_effect=module.source_authority.SourceError('source_authority_rejected')),\
             patch.object(module,'verify_runtime') as runtime,patch.object(module,'save') as save:
            with self.assertRaises(module.source_authority.SourceError):
                module.accept(Path('/private'),self.state,'token',None,self.binding,{})
        self.assertEqual(self.state,before);save.assert_not_called();runtime.assert_not_called()

    def test_acceptance_rechecks_exact_original_attempt_and_runtime_without_restart(self):
        with patch.object(module.source_authority,'revalidate_ci',return_value={'id':123}) as ci,\
             patch.object(module,'verify_runtime') as runtime,patch.object(module,'save') as save:
            result=module.accept(Path('/private'),self.state,'token',None,self.binding,{})
        ci.assert_called_once_with('token',self.source,final=True)
        runtime.assert_called_once();save.assert_called_once()
        self.assertEqual(result['status'],'PASS');self.assertEqual(result['provisional'],self.admission)
        self.assertEqual(result['phase'],'POST_RESTART')

    def test_failed_runtime_acceptance_preserves_actual_checkpoint_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            base=Path(directory);checkpoint=base/'checkpoint.json'
            checkpoint.write_text(json.dumps(self.state));before=checkpoint.read_bytes()
            with patch.object(module.source_authority,'revalidate_ci'),\
                 patch.object(module,'verify_runtime',side_effect=Blocked('provisional_runtime_changed')):
                with self.assertRaises(Blocked):
                    module.accept(base,self.state,'token',None,self.binding,{})
            self.assertEqual(checkpoint.read_bytes(),before)
            self.assertEqual(self.state['status'],'PROVISIONAL_READY')

    def test_blocked_unknown_and_fenced_parents_are_not_repair_candidates(self):
        for key,value in [('status','BLOCKED'),('status','UNKNOWN'),('phase','PAUSED_APPLY'),
                          ('fence_status','VERIFIED')]:
            state=dict(self.state,**{key:value})
            with self.assertRaises(Blocked):module.require_ready(state,self.binding)

    def test_helper_drift_and_missing_bound_admission_rejected(self):
        with self.assertRaises(Blocked):module.require_ready(self.state,{'helper.py':'e'*64})
        state=dict(self.state);state.pop('provisional')
        with self.assertRaises(Blocked):module.require_ready(state,self.binding)

    def test_real_finish_retains_provisional_not_pass_after_preservation_restart(self):
        state=copy.deepcopy(self.state);state.update(phase='PAUSED_APPLY',status='WAITING',
            provenance={},cut={'native':'same'},context={})
        receipt={'operation':state['operation'],'target':state['target'],'provisional':state['provisional'],
            'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(minutes=5)).isoformat()}
        state['authorization']=copy.deepcopy(receipt)
        stage=Mock();stage.final_path=Path('/same');stage.restart.return_value={'verified':True}
        with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'save'),patch.object(transaction,'revalidate_inputs'),\
             patch.object(transaction,'DockerRuntime'),patch.object(transaction,'verify_post_apply',return_value={'native':'preserved'}) as native,\
             patch.object(transaction,'attach_closed_preservation') as sealed:
            result=transaction.finish(None,Path('/private'),state,receipt,'7',{})
        self.assertEqual(result['status'],'PROVISIONAL_READY')
        self.assertEqual((result['phase'],result['fence_status']),('POST_RESTART','RELEASED'))
        native.assert_called_once();sealed.assert_called_once();stage.restart.assert_called_once()

    def test_repair_link_preserves_parent_bytes_and_failed_child_blocks(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);base=root/('rollout-'+self.state['operation']);base.mkdir()
            checkpoint=base/'checkpoint.json';checkpoint.write_text(json.dumps(self.state))
            before=checkpoint.read_bytes();child_op='2'*12
            reference=module.link_parent(base,self.state,child_op)
            self.assertEqual(checkpoint.read_bytes(),before)
            with self.assertRaises(Blocked):module.link_parent(base,self.state,'3'*12)
            child_base=root/('rollout-'+child_op);child_base.mkdir()
            child={'operation':child_op,'repair_parent':reference,'status':'BLOCKED',
                'phase':'FENCE','fence_status':'UNKNOWN'}
            module.save(child_base/'checkpoint.json',child)
            self.assertFalse(module.covered(base,self.state))
            self.assertFalse(module.covered(base,self.state,admitted=child_op))
            child.update(status='PROVISIONAL_READY',phase='POST_RESTART',fence_status='RELEASED')
            module.save(child_base/'checkpoint.json',child)
            self.assertFalse(module.covered(base,self.state))
            self.assertTrue(module.covered(base,self.state,admitted=child_op))
            child.update(status='PASS');module.save(child_base/'checkpoint.json',child)
            self.assertTrue(module.covered(base,self.state))
            self.assertEqual(checkpoint.read_bytes(),before)
            changed=copy.deepcopy(self.state);changed['events'].append({'kind':'foreign'})
            with self.assertRaises(Blocked):module.covered(base,changed)

    def test_repair_parent_does_not_reuse_expired_application_or_failed_candidate_ci(self):
        state=copy.deepcopy(self.state)
        state['authorization']={'expires_at':'2000-01-01T00:00:00+00:00'}
        # Admission uses runtime readiness; the new child obtains its own source
        # and custody authority. Failed candidate CI is never asserted accepted.
        with patch.object(module.source_authority,'revalidate_ci') as ci:
            module.require_ready(state,self.binding)
        ci.assert_not_called()
        self.assertEqual(state['status'],'PROVISIONAL_READY')

    def test_actual_idle_requires_named_parent_or_complete_reciprocal_child(self):
        import bridge_root
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);base=root/('rollout-'+self.state['operation']);base.mkdir()
            module.save(base/'checkpoint.json',self.state)
            actions=object.__new__(bridge_root.Actions)
            with patch.object(bridge_root.guard,'ROOT',root),\
                 patch.object(bridge_root.root_cli,'rollout_directory_kind',return_value='operation'):
                with self.assertRaises(Blocked):actions._idle()
                actions._idle(admitted_parent=self.state['operation'])
                reference=module.link_parent(base,self.state,'2'*12)
                child_base=root/('rollout-'+'2'*12);child_base.mkdir()
                child={'operation':'2'*12,'repair_parent':reference,'status':'BLOCKED',
                    'phase':'FENCE','fence_status':'UNKNOWN'}
                module.save(child_base/'checkpoint.json',child)
                with self.assertRaises(Blocked):actions._idle()
                child.update(status='PASS',phase='POST_RESTART',fence_status='RELEASED')
                module.save(child_base/'checkpoint.json',child)
                actions._idle()
                child['repair_parent']['checkpoint_sha256']='f'*64
                module.save(child_base/'checkpoint.json',child)
                with self.assertRaises(Blocked):actions._idle()

    def test_actual_prepare_creates_checkpoint_before_link_and_partial_fence_stays_blocked(self):
        with tempfile.TemporaryDirectory() as directory,ExitStack() as mocks:
            root=Path(directory);parent=root/('rollout-'+self.state['operation']);parent.mkdir()
            (parent/'checkpoint.json').write_text(json.dumps(self.state));before=(parent/'checkpoint.json').read_bytes()
            child_op='2'*12;base=root/('rollout-'+child_op);base.mkdir();code=base/'source';code.mkdir()
            (code/'kernel').write_bytes(b'public-fixture');(base/'apply-manifests.json').write_text('[]')
            plan={'actions':[]};target={'mode':'images-only','migration_sha256':module.canonical([]),
                'nonnats_sha256':module.canonical(plan),'tag':'b'*40}
            stage=Mock();stage.expected={'source_claim':'same','generation':'generation'}
            stage.final_path=Path('/same');stage.fence.side_effect=Blocked('synthetic_partial_fence')
            stage.refence.return_value={'verified':False}
            kube=Mock();kube.get.return_value={'status':{'capacity':{'storage':'1Gi'}}}
            replacements={
                'context':lambda current:{'expected':current.expected},
                'normalize_target':lambda *a,**k:target,
                'image_only_documents':lambda *a,**k:([],target),
                'native_preflight':lambda *a:{},'capture_inputs':lambda *a,**k:{},
                'private_json':Mock(side_effect=FileNotFoundError),
            }
            for name,value in replacements.items():mocks.enter_context(patch.object(transaction,name,value))
            mocks.enter_context(patch.object(transaction.RolloutStage,'capture',return_value=stage))
            mocks.enter_context(patch.object(transaction.migrations,'preflight',return_value=None))
            mocks.enter_context(patch.object(transaction.space_safeguards,'preflight',return_value=None))
            mocks.enter_context(patch.object(transaction.space_safeguards,'combined_capacity',return_value={}))
            mocks.enter_context(patch.object(transaction.nonnats_runtime,'preflight',return_value={}))
            mocks.enter_context(patch.object(transaction.nonnats_plan,'revalidate'))
            import source_plan
            mocks.enter_context(patch.object(source_plan,'verify_db_init'))
            def before_fence():
                checkpoint=json.loads((base/'checkpoint.json').read_text())
                self.assertEqual(checkpoint['phase'],'FENCE')
                self.assertEqual(checkpoint['repair_parent'],module.parent_reference(self.state))
                stage.fence.assert_not_called()
                module.link_parent(parent,self.state,child_op)
            with self.assertRaisesRegex(Blocked,'synthetic_partial_fence'):
                transaction.prepare(kube,base,code,target,{},child_op,self.binding,[],plan,
                    before_fence=before_fence,lifecycle={'repair_parent':module.parent_reference(self.state)})
            child=json.loads((base/'checkpoint.json').read_text())
            self.assertEqual((child['status'],child['phase'],child['fence_status']),('BLOCKED','FENCE','UNKNOWN'))
            self.assertFalse(module.covered(parent,self.state))
            self.assertEqual((parent/'checkpoint.json').read_bytes(),before)

    def runtime_fixture(self):
        state=copy.deepcopy(self.state)
        image='ghcr.io/poryadok/voiceroot/voice@sha256:'+'c'*64
        expected={'namespace_uid':'namespace','source_claim_uid':'claim'}
        marker={'metadata':{'uid':'marker','resourceVersion':'7'},'data':{'phase':'active'}}
        workload={'metadata':{'uid':'deployment','generation':2},
            'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'voice','image':image}]}}},
            'status':{'observedGeneration':2,'readyReplicas':1,'updatedReplicas':1,'availableReplicas':1}}
        state['target'].update(manifest_sha256='e'*64,images={'voice-voice/voice':image})
        state['context']={'expected':expected,'marker':marker,'snapshots':{'voice-voice':copy.deepcopy(workload)},
            'old_images':{'voice-voice/voice':image}}
        state['events']=[{'kind':'rollout_released','marker':copy.deepcopy(marker)}]
        state['nonnats']={'checks':[]}
        state['nonnats_binding']={'plan_sha256':module.canonical(state['nonnats']),'objects':[]}
        active={'operation':state['operation'],'namespace_uid':'namespace','pvc_uid':'claim','target_sha256':'e'*64}
        captured=Mock();captured.expected=copy.deepcopy(expected);captured.old_images={'voice-voice/voice':image}
        rows={'configmap':copy.deepcopy(marker),'deployment':copy.deepcopy(workload)}
        kube=Mock();kube.get.side_effect=lambda kind,name:rows[kind]
        return state,active,captured,rows,kube

    def test_exact_unhealthy_released_parent_can_be_repaired_but_not_accepted(self):
        state,active,captured,rows,kube=self.runtime_fixture()
        rows['deployment']['status'].update(readyReplicas=0,availableReplicas=0)
        with patch.object(module.RolloutStage,'capture',return_value=captured):
            module.verify_runtime(state,kube,self.binding,active,require_healthy=False)
            with self.assertRaises(Blocked):module.verify_runtime(state,kube,self.binding,active)
            rows['deployment']['metadata']['uid']='foreign'
            with self.assertRaises(Blocked):module.verify_runtime(state,kube,self.binding,active,require_healthy=False)

    def test_preserved_resource_drift_blocks_acceptance_and_preserves_checkpoint(self):
        import nonnats_plan
        from nonnats_runtime_test import PreservedServiceTest
        for kind in ('Service','ConfigMap','Secret'):
            with self.subTest(kind=kind),tempfile.TemporaryDirectory() as directory:
                state,active,captured,rows,kube=self.runtime_fixture()
                fixture=PreservedServiceTest();fixture.setUp()
                fixture.current['kind']=kind
                fixture.binding['objects'][0]['kind']=kind
                fixture.plan['checks'][0]['objects'][0]['kind']=kind
                fixture.binding['plan_sha256']=nonnats_plan.digest(fixture.plan)
                # Changed Service semantics, or changed strict config/secret RV.
                if kind=='Service':fixture.current['spec']['selector']={'app':'foreign'}
                state['nonnats']=fixture.plan;state['nonnats_binding']=fixture.binding
                kube.get.side_effect=lambda resource,name:fixture.current if resource==kind else rows[resource]
                base=Path(directory);checkpoint=base/'checkpoint.json';checkpoint.write_text(json.dumps(state))
                before=checkpoint.read_bytes()
                with patch.object(module.RolloutStage,'capture',return_value=captured),\
                     patch.object(module.source_authority,'revalidate_ci'):
                    with self.assertRaises(Blocked):module.accept(base,state,'token',kube,self.binding,active)
                self.assertEqual(checkpoint.read_bytes(),before)
                self.assertEqual(state['status'],'PROVISIONAL_READY')

    def test_actual_runtime_proof_rejects_marker_target_storage_image_and_readiness_drift(self):
        for fault in ('none','active','owner','template','uid','storage','image','ready'):
            with self.subTest(fault=fault):
                state,active,captured,rows,kube=self.runtime_fixture()
                if fault=='active':active['target_sha256']='f'*64
                elif fault=='owner':rows['configmap']['data']['knownRolloutOperation']='foreign'
                elif fault=='template':rows['deployment']['spec']['template']['extra']='drift'
                elif fault=='uid':rows['deployment']['metadata']['uid']='foreign'
                elif fault=='storage':captured.expected['source_claim_uid']='foreign'
                elif fault=='image':captured.old_images['voice-voice/voice']='foreign'
                elif fault=='ready':rows['deployment']['status']['readyReplicas']=0
                with patch.object(module.RolloutStage,'capture',return_value=captured),\
                     patch.object(module.source_authority,'same_image_content',return_value=False):
                    if fault=='none':module.verify_runtime(state,kube,self.binding,active)
                    else:
                        with self.assertRaises(Blocked):module.verify_runtime(state,kube,self.binding,active)
                kube.cas.assert_not_called();captured.restart.assert_not_called();captured.fence.assert_not_called()

    def test_safe_report_distinguishes_runtime_ready_and_release_acceptance(self):
        request={'nonce':'a'*64,'action':'finish','operation':self.state['operation']}
        report=deployment_report.validate(deployment_report.build(request,self.state))
        self.assertEqual(report['operation_status'],'PROVISIONAL_READY')
        self.assertEqual(report['release_acceptance'],'UNACCEPTED')
        self.assertEqual(report['effects']['release'],'RECORDED')
        self.assertFalse(report['recovery_authorized'])
        state=dict(self.state,status='PASS',ci_acceptance={'verified':True})
        report=deployment_report.validate(deployment_report.build(request,state))
        self.assertEqual(report['release_acceptance'],'ACCEPTED')

if __name__=='__main__':unittest.main()
