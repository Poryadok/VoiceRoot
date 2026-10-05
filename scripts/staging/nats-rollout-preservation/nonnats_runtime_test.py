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
    def test_unowned_same_template_rv_change_and_live_restart_are_blocked(self):
        for mutate in (lambda:setitem(self.current['metadata'],'resourceVersion','13'),lambda:setitem(self.current['spec'],'replicas',1)):
            before=copy.deepcopy(self.current);mutate()
            with self.assertRaisesRegex(nonnats_runtime.Blocked,'owned_workload_changed'):
                nonnats_runtime.verify(self.kube,self.plan,self.binding,self.stage)
            self.current=before

def setitem(mapping,key,value):mapping[key]=value
if __name__=='__main__':unittest.main()
