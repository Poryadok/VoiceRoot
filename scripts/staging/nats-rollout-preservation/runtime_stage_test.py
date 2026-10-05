import copy
import unittest
from unittest.mock import patch
import runtime_stage
from runtime_stage import RolloutStage, Blocked
from apply import digest


class StageTest(unittest.TestCase):
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
             patch.object(self.stage,'no_pods') as no_pods,patch.object(self.stage,'verify_closed'):
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
