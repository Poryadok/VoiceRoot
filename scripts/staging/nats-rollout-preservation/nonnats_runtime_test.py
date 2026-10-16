import copy
from types import SimpleNamespace
import unittest
import nonnats_plan
import nonnats_runtime

class OwnedPlanTest(unittest.TestCase):
    def setUp(self):
        self.current={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-web','namespace':'voice-staging','uid':'u','resourceVersion':'12'},
            'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'web','image':'new'}]}}}}
        self.stage=SimpleNamespace(snapshots={'voice-web':copy.deepcopy(self.current)})
        self.plan={'checks':[{'objects':[{'kind':'Deployment','name':'voice-web','desired':{'target':'new'}}]}]}
        self.binding={'plan_sha256':nonnats_plan.digest(self.plan),'objects':[{'kind':'Deployment','name':'voice-web','uid':'u','resourceVersion':'10','disposition':'action'}]}
        self.kube=SimpleNamespace(get=lambda *args:copy.deepcopy(self.current))
    def test_exact_owned_fence_rv_is_allowed_instead_of_original_rv(self):
        self.assertTrue(nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage)['verified'])
    def test_controller_status_and_resource_version_churn_keeps_owned_semantics(self):
        self.current['metadata']['resourceVersion']='13'
        self.current['metadata']['generation']=7
        self.current['metadata']['managedFields']=[{'manager':'kube-controller-manager'}]
        self.current['status']={'observedGeneration':7,'replicas':0,'conditions':[]}
        receipt=nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage)
        self.assertTrue(receipt['verified'])
        self.assertEqual(receipt['owned_resources'][0]['resourceVersion'],'13')
        self.assertEqual(receipt['owned_resources'][0]['semantic_sha256'],
            nonnats_plan.digest(nonnats_plan.semantic(self.stage.snapshots['voice-web'])))

    def test_owned_identity_configuration_and_live_restart_changes_are_blocked(self):
        mutations=(
            lambda:setitem(self.current['metadata'],'uid','replacement'),
            lambda:setitem(self.current['spec'],'replicas',1),
            lambda:setitem(self.current['spec']['template']['spec']['containers'][0],'image','foreign'),
            lambda:setitem(self.current['spec']['template']['spec']['containers'][0],'env',[{'name':'CONFIG','value':'changed'}]),
            lambda:setitem(self.current['metadata'],'labels',{'changed':'true'}),
            lambda:setitem(self.current['metadata'],'annotations',{'changed':'true'}),
            lambda:setitem(self.current['metadata'],'ownerReferences',[{'uid':'foreign'}]),
            lambda:setitem(self.current['metadata'],'finalizers',['foreign']),
        )
        for number,mutate in enumerate(mutations):
            with self.subTest(mutation=number):
                before=copy.deepcopy(self.current);mutate()
                with self.assertRaisesRegex(nonnats_runtime.Blocked,'owned_workload_changed'):
                    nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage)
                self.current=before

    def test_owned_controller_revision_after_target_adoption_is_not_config_drift(self):
        self.stage.snapshots['voice-web']['metadata']['annotations']={
            'deployment.kubernetes.io/revision':'5','voice.io/config':'unchanged'}
        self.current['metadata']['annotations']={
            'deployment.kubernetes.io/revision':'6','voice.io/config':'unchanged'}
        self.assertTrue(nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage,applied=True)['verified'])
        self.current['metadata']['annotations']['voice.io/config']='changed'
        with self.assertRaisesRegex(nonnats_runtime.Blocked,'owned_workload_changed'):
            nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage,applied=True)

class PreservedServiceTest(unittest.TestCase):
    def setUp(self):
        self.current={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-auth','namespace':'voice-staging',
            'uid':'same-uid','resourceVersion':'3320022','labels':{'app':'voice-auth'}},
            'spec':{'selector':{'app':'voice-auth'},'ports':[{'port':8080,'targetPort':8080}],
                    'clusterIP':'10.43.1.2','type':'ClusterIP'}}
        desired=nonnats_plan.semantic(self.current)
        self.plan={'checks':[{'objects':[{'kind':'Service','name':'voice-auth','desired':desired}]}]}
        self.binding={'schema':nonnats_plan.BINDING_SCHEMA,'plan_sha256':nonnats_plan.digest(self.plan),
            'objects':[{'kind':'Service','name':'voice-auth','namespace':'voice-staging','uid':'same-uid',
                'resourceVersion':'3310867','disposition':'preserve','desired':desired}]}
        self.stage=SimpleNamespace(snapshots={})
        self.kube=SimpleNamespace(get=lambda *args:copy.deepcopy(self.current))

    def test_full_apply_bookkeeping_rv_change_keeps_preserved_service(self):
        self.current['metadata']['annotations']={'kubectl.kubernetes.io/last-applied-configuration':'updated'}
        before=copy.deepcopy(self.binding)
        result=nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage,applied=True)
        self.assertTrue(result['verified'])
        self.assertEqual(result['owned_resources'][0]['resourceVersion'],'3320022')
        self.assertEqual(self.binding,before)

    def test_preapply_and_secret_version_checks_remain_strict(self):
        with self.assertRaisesRegex(nonnats_runtime.Blocked,'live_object_identity_changed'):
            nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage)
        self.current['kind']='Secret';self.binding['objects'][0]['kind']='Secret'
        self.plan['checks'][0]['objects'][0]['kind']='Secret'
        self.binding['plan_sha256']=nonnats_plan.digest(self.plan)
        with self.assertRaisesRegex(nonnats_runtime.Blocked,'live_object_identity_changed'):
            nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage,applied=True)

    def test_service_identity_and_full_semantic_drift_still_refuse(self):
        for field in ('uid','namespace','name','labels','annotations','ownerReferences','finalizers','selector','ports','clusterIP'):
            with self.subTest(field=field):
                before=copy.deepcopy(self.current)
                if field in ('selector','ports','clusterIP'):self.current['spec'][field]='changed'
                elif field in ('annotations','labels'):self.current['metadata'][field]={'foreign':'changed'}
                elif field=='ownerReferences':self.current['metadata'][field]=[{'uid':'foreign'}]
                elif field=='finalizers':self.current['metadata'][field]=['foreign']
                else:self.current['metadata'][field]='changed'
                with self.assertRaises(nonnats_runtime.Blocked):
                    nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage,applied=True)
                self.current=before

def setitem(mapping,key,value):mapping[key]=value
if __name__=='__main__':unittest.main()
