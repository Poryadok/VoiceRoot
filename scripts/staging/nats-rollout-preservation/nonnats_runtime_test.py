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

def setitem(mapping,key,value):mapping[key]=value
if __name__=='__main__':unittest.main()
