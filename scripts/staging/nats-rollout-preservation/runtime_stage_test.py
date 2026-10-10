import copy
import unittest
from unittest.mock import patch
import runtime_stage
from runtime_stage import RolloutStage, Blocked
from apply import digest


class StageTest(unittest.TestCase):
    def test_shutdown_requires_current_exact_container_termination_not_old_state(self):
        before={'metadata':{'uid':'pod'},'status':{'containerStatuses':[{'name':'nats',
            'containerID':'containerd://'+'a'*64,'restartCount':2,'state':{'running':{'startedAt':'2026-10-10T00:00:00Z'}}}]}}
        after=copy.deepcopy(before);after['metadata']['deletionTimestamp']='2026-10-10T00:01:00Z'
        after['status']['containerStatuses'][0]['state']={'terminated':{'exitCode':0,'signal':0,
            'reason':'Completed','startedAt':'2026-10-10T00:00:00Z','finishedAt':'2026-10-10T00:00:31Z'}}
        self.assertEqual(runtime_stage.terminated_hub(before,after)['exit_code'],0)
        for fault in ('old-state','oom','exit137','replacement','restart','missing-deletion'):
            current=copy.deepcopy(after);status=current['status']['containerStatuses'][0]
            if fault=='old-state':status['lastState']=status['state'];status['state']=before['status']['containerStatuses'][0]['state']
            elif fault=='oom':status['state']['terminated']['reason']='OOMKilled'
            elif fault=='exit137':status['state']['terminated']['exitCode']=137
            elif fault=='replacement':status['containerID']='containerd://'+'b'*64
            elif fault=='restart':status['restartCount']=3
            else:current['metadata'].pop('deletionTimestamp')
            with self.subTest(fault=fault),self.assertRaises(Blocked):runtime_stage.terminated_hub(before,current)

    def test_ordinary_restart_cannot_skip_the_same_closed_seal(self):
        self.marker['data']['phase']='rollout-verified'
        with patch.object(runtime_stage.Staging,'restart') as parent:
            with self.assertRaisesRegex(Blocked,'closed_preservation_seal_required'):
                self.stage.restart()
        parent.assert_not_called()

    def test_recovery_restart_cannot_resume_business_without_closed_seal(self):
        self.marker['data']['phase']='rollout-verified'
        self.stage.finish_authority_guard=lambda:None
        calls=[]
        def parent_restart(stage):
            calls.append('hub-start');calls.append('business-start')
        with patch.object(runtime_stage.Staging,'restart',parent_restart):
            with self.assertRaisesRegex(Blocked,'closed_preservation_seal_required'):
                self.stage.restart()
        self.assertEqual(calls,[])

    def test_cold_release_intent_precedes_normal_hub_start_without_live_seal(self):
        self.marker['data']['phase']='rollout-verified'
        calls=[]
        from preserve import ColdRelease
        cold=object.__new__(ColdRelease)
        cold.verify_and_journal=lambda stage:calls.append('verified-release-intent')
        cold.arm_release=unittest.mock.Mock()
        self.stage.closed_preservation_producer=cold
        self.stage.closed_preservation_runtime=cold
        def restart(stage):
            calls.extend(('hub-start','business-start'))
        with patch.object(runtime_stage.Staging,'restart',restart):
            self.stage.restart()
        self.assertEqual(calls,['verified-release-intent','hub-start','business-start'])
        cold.arm_release.assert_called_once_with(self.stage,None)

    def test_failed_cold_release_proof_never_starts_hub(self):
        self.marker['data']['phase']='rollout-verified'
        from preserve import ColdRelease
        cold=object.__new__(ColdRelease)
        cold.verify_and_journal=unittest.mock.Mock(side_effect=Blocked('cold_native_changed'))
        self.stage.closed_preservation_producer=cold
        self.stage.closed_preservation_runtime=cold
        with patch.object(runtime_stage.Staging,'restart') as restart:
            with self.assertRaisesRegex(Blocked,'cold_native_changed'):self.stage.restart()
        restart.assert_not_called()

    def test_hub_renderer_actual_pin_requires_explicit_opt_in(self):
        hub={'metadata':{'uid':'hub'},'spec':{'template':{'spec':{
            'initContainers':[{'name':'nats-config-renderer','image':'renderer:old'}],
            'containers':[{'name':'nats','image':'broker:fixed'}]}}}}
        rows=[{'kind':'ReplicaSet','metadata':{'uid':'rs','ownerReferences':[{'kind':'Deployment','uid':'hub'}]},'spec':{'template':copy.deepcopy(hub['spec']['template'])}},
              {'kind':'Pod','metadata':{'ownerReferences':[{'kind':'ReplicaSet','uid':'rs'}]},'status':{'phase':'Running',
                'initContainerStatuses':[{'name':'nats-config-renderer','imageID':'example.invalid/renderer@sha256:'+'a'*64}],
                'containerStatuses':[{'name':'nats','ready':True,'imageID':'example.invalid/nats@sha256:'+'b'*64}]}}]
        self.assertEqual(runtime_stage.running_image_pins({runtime_stage.HUB:hub},rows),{})
        pins=runtime_stage.running_image_pins({runtime_stage.HUB:hub},rows,include_hub=True)
        self.assertEqual(pins[runtime_stage.HUB+'/nats-config-renderer'],'example.invalid/renderer@sha256:'+'a'*64)
        self.assertEqual(pins[runtime_stage.HUB+'/nats'],'example.invalid/nats@sha256:'+'b'*64)

    def test_renderer_post_start_gate_runs_before_first_app_restart(self):
        self.marker['data']['phase']='rollout-verified'
        self.stage.renderer_transition={'verified':True}
        self.stage.renderer_post_start=unittest.mock.Mock(side_effect=Blocked('renderer_live_output_changed'))
        calls=[]
        def parent_restart(stage):
            stage.scale(runtime_stage.HUB,1);stage.wait_ready(runtime_stage.HUB)
            stage.scale('voice-user',1)
        with patch.object(self.stage,'seal_before_business_resume'),patch.object(runtime_stage.Staging,'restart',parent_restart),patch.object(runtime_stage.Staging,'wait_ready'),patch.object(self.stage,'scale',side_effect=lambda name,n:calls.append((name,n))):
            with self.assertRaisesRegex(Blocked,'renderer_live_output_changed'):self.stage.restart()
        self.assertEqual(calls,[(runtime_stage.HUB,1)])
        self.stage.renderer_post_start.assert_called_once_with(self.stage)

    def test_renderer_restart_without_bound_post_start_gate_is_forbidden(self):
        self.marker['data']['phase']='rollout-verified';self.stage.renderer_transition={'verified':True}
        with patch.object(runtime_stage.Staging,'restart') as restart:
            with self.assertRaisesRegex(Blocked,'renderer_restart_proof_missing'):self.stage.restart()
        restart.assert_not_called()
    def test_user_restore_cas_is_durable_before_readiness_wait_can_interrupt(self):
        original={"metadata":{"annotations":{}},"spec":{"containers":[{"name":"user","env":[]}]}}
        baseline={"metadata":{"uid":"user","resourceVersion":"1"},"spec":{"template":original}}
        overridden=copy.deepcopy(baseline);overridden["metadata"]["resourceVersion"]="2"
        overridden["spec"]["template"]["spec"]["containers"][0]["env"]=[{"name":"SPACE_GRPC_ADDR","value":""}]
        restored=copy.deepcopy(baseline);restored["metadata"]["resourceVersion"]="3"
        saved=[]
        stage=runtime_stage.Staging(unittest.mock.Mock(),"123456abcdef",{},lambda event:saved.append((event,copy.deepcopy(stage.snapshots))))
        stage.snapshots={"voice-user":baseline};stage.final_claim={"metadata":{"name":"selected","uid":"pvc"}}
        stage.kube.get.side_effect=[stage.final_claim,baseline,{"data":{"SPACE_GRPC_ADDR":"space:50051"}},overridden]
        stage.kube.cas.side_effect=[overridden,restored]
        def ready(name):
            if name=="voice-user" and stage.snapshots["voice-user"]["metadata"]["resourceVersion"]=="3":raise KeyboardInterrupt()
        with patch.object(stage,"owned_marker"),patch.object(stage,"no_pods"),patch.object(stage,"scale"),patch.object(stage,"wait_ready",side_effect=ready):
            with self.assertRaises(KeyboardInterrupt):stage.restart()
        self.assertEqual(saved[-1][0]["kind"],"user_cycle_restored")
        self.assertEqual(saved[-1][1]["voice-user"],restored)

    def test_old_ready_replicaset_cannot_supply_images_for_new_desired_template(self):
        template={"metadata":{"labels":{"app":"gateway"}},"spec":{"containers":[{"name":"gateway","image":"new-tag"}]}}
        deployment={"metadata":{"uid":"d"},"spec":{"template":template}}
        previous=copy.deepcopy(template);previous["spec"]["containers"][0]["image"]="old-tag"
        rows=[{"kind":"ReplicaSet","metadata":{"uid":"r","ownerReferences":[{"uid":"d","kind":"Deployment"}]},"spec":{"template":previous}},
              {"kind":"Pod","metadata":{"ownerReferences":[{"uid":"r","kind":"ReplicaSet"}]},"status":{"phase":"Running","containerStatuses":[{"name":"gateway","ready":True,"imageID":"example.invalid/gateway@sha256:"+"a"*64}]}}]
        with self.assertRaisesRegex(Blocked,"old_revision_unconverged"):runtime_stage.running_image_pins({"voice-gateway":deployment},rows)

    def test_old_image_pin_is_actual_running_pod_digest_not_mutable_spec_tag(self):
        uid='deployment-uid';rs='replicaset-uid'
        rows=[{'kind':'ReplicaSet','metadata':{'uid':rs,'ownerReferences':[{'uid':uid,'kind':'Deployment'}]}},
              {'kind':'Pod','metadata':{'ownerReferences':[{'uid':rs,'kind':'ReplicaSet'}]},
               'status':{'phase':'Running','containerStatuses':[{'name':'gateway','ready':True,
                   'imageID':'docker-pullable://example.invalid/gateway@sha256:'+'a'*64}]}}]
        deployment={'metadata':{'uid':uid},'spec':{'template':{'spec':{'containers':[{'name':'gateway','image':'example.invalid/gateway:old'}]}}}}
        rows[0]['spec']={'template':copy.deepcopy(deployment['spec']['template'])}
        self.assertEqual(runtime_stage.running_image_pins({'voice-gateway':deployment},rows),
                         {'voice-gateway/gateway':'example.invalid/gateway@sha256:'+'a'*64})
        rows[1]['status']['containerStatuses'][0]['imageID']='containerd://'+'a'*64
        with self.assertRaisesRegex(Blocked,'old_image_identity'):runtime_stage.running_image_pins({'voice-gateway':deployment},rows)
    def test_ambiguous_actual_old_digest_blocks_before_any_fence(self):
        deployment={'metadata':{'uid':'d'},'spec':{'template':{'spec':{'containers':[{'name':'gateway','image':'tag'}]}}}}
        rows=[{'kind':'ReplicaSet','metadata':{'uid':'r','ownerReferences':[{'uid':'d','kind':'Deployment'}]}}]
        rows[0]['spec']={'template':copy.deepcopy(deployment['spec']['template'])}
        for digest in ('a','b'):
            rows.append({'kind':'Pod','metadata':{'ownerReferences':[{'uid':'r','kind':'ReplicaSet'}]},
                'status':{'phase':'Running','containerStatuses':[{'name':'gateway','ready':True,
                  'imageID':'docker-pullable://example.invalid/gateway@sha256:'+digest*64}]}})
        with self.assertRaisesRegex(Blocked,'old_image_ambiguous'):runtime_stage.running_image_pins({'voice-gateway':deployment},rows)
    def setUp(self):
        self.marker={'metadata':{'uid':'marker','resourceVersion':'101','name':'voice-nats-generation'},
                     'data':{'phase':'rollout-applying','knownRolloutOperation':'123456abcdef','dataPVC':'selected'}}
        self.old={'metadata':{'uid':'deployment','name':'voice-gateway'},'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'gateway','image':'old'}]}}}}
        self.target=copy.deepcopy(self.old);self.target['spec']['template']['spec']['containers'][0]['image']='new'
        self.mutations=[]
        owner=self
        class Kube:
            def get(self,kind,name):return copy.deepcopy(owner.marker if kind=='configmap' else owner.target)
            def cas(self,kind,row,changes):owner.mutations.append(changes);return copy.deepcopy(row)
        self.stage=RolloutStage(Kube(),'123456abcdef',{'source_claim':'selected'},lambda event:None)
        self.stage.marker=copy.deepcopy(self.marker);self.stage.snapshots={'voice-gateway':self.old}
        self.receipt={'target':{'template_hashes':{'voice-gateway':digest(self.target['spec']['template'])}}}
    def test_target_is_adopted_only_with_same_uid_paused_and_exact_claim_rv(self):
        with patch.object(self.stage,'verify_final_storage') as verify:
            self.stage.adopt_applied_target(self.receipt,'101')
        verify.assert_called_once();self.assertEqual(self.stage.snapshots['voice-gateway'],self.target)
        self.assertEqual(self.mutations,[])
    def test_target_uid_reuse_blocks_before_adoption(self):
        self.target['metadata']['uid']='replacement'
        with self.assertRaisesRegex(Blocked,'applied_target_changed'):
            self.stage.adopt_applied_target(self.receipt,'101')
        self.assertEqual(self.stage.snapshots['voice-gateway'],self.old)
    def test_runner_restart_blocks_target_adoption(self):
        self.target['spec']['replicas']=1
        with self.assertRaisesRegex(Blocked,'applied_target_changed'):
            self.stage.adopt_applied_target(self.receipt,'101')
    def test_arbitrary_latest_marker_version_cannot_replace_claim_response(self):
        with self.assertRaisesRegex(Blocked,'apply_owner_changed'):
            self.stage.adopt_applied_target(self.receipt,'102')
    def test_unverified_transaction_cannot_restart_any_workload(self):
        with patch.object(runtime_stage.Staging,'restart') as restart:
            with self.assertRaisesRegex(Blocked,'unverified_resume_forbidden'):self.stage.restart()
        restart.assert_not_called();self.assertEqual(self.mutations,[])
    def test_rollout_has_no_pvc_allocation_or_swap_seam(self):
        with self.assertRaisesRegex(Blocked,'allocation_forbidden'):self.stage.new_claim('20261005')
        with self.assertRaisesRegex(Blocked,'selection_forbidden'):self.stage.select_claim()
        self.assertEqual(self.mutations,[])
    def test_rollback_restores_original_template_with_actual_old_digest_while_paused(self):
        self.stage.snapshots={'voice-gateway':copy.deepcopy(self.target)}
        self.stage.original_snapshots={'voice-gateway':copy.deepcopy(self.old)}
        self.stage.old_images={'voice-gateway/gateway':'example.invalid/gateway@sha256:'+'a'*64}
        with patch.object(self.stage,'verify_final_storage'),patch.object(self.stage.kube,'cas',side_effect=lambda kind,row,changes:dict(row,spec=dict(row['spec'],template=changes[-1]['value']))) as cas:
            self.stage.restore_original_templates()
        self.assertEqual(self.stage.snapshots['voice-gateway']['spec']['replicas'],0)
        self.assertEqual(self.stage.snapshots['voice-gateway']['spec']['template']['spec']['containers'][0]['image'],self.stage.old_images['voice-gateway/gateway'])
        self.assertEqual(cas.call_args.args[2][0],{'op':'test','path':'/spec/replicas','value':0})
    def test_rollback_refuses_live_restart_or_changed_current_template(self):
        self.stage.original_snapshots={'voice-gateway':copy.deepcopy(self.old)}
        self.stage.old_images={'voice-gateway/gateway':'example.invalid/gateway@sha256:'+'a'*64}
        with patch.object(self.stage,'verify_final_storage'):
            with self.assertRaisesRegex(Blocked,'rollback_workload_changed'):self.stage.restore_original_templates()
        self.assertEqual(self.mutations,[])
    def test_frontend_is_physically_fenced_with_core_before_paused_apply(self):
        core=(runtime_stage.HUB,'voice-gateway',*('voice-'+s for s in runtime_stage.LEAVES))
        self.stage.snapshots={name:{} for name in (*core,'voice-web')}
        with patch.object(self.stage,'maintenance'),patch.object(self.stage,'scale') as scale,\
             patch.object(self.stage,'no_pods') as no_pods,patch.object(self.stage,'verify_closed'),\
             patch.object(self.stage,'drain_client_pods'),patch.object(self.stage,'shutdown_original_hub'):
            self.stage.fence()
        self.assertIn(unittest.mock.call('voice-web',0),scale.call_args_list)
        self.assertEqual(set(no_pods.call_args.args[0]),set(self.stage.snapshots))
    def test_frontend_restarts_only_under_verified_ownership_before_marker_release(self):
        self.marker['data']['phase']='rollout-verified'
        self.stage.snapshots['voice-web']=copy.deepcopy(self.old)
        with patch.object(self.stage,'scale') as scale,patch.object(self.stage,'wait_ready') as ready:
            self.stage.release_marker()
        scale.assert_called_once_with('voice-web',1)
        ready.assert_called_once_with('voice-web')

if __name__=='__main__':unittest.main()
