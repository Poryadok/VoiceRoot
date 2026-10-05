import copy
import datetime as dt
import json
import unittest
from unittest.mock import patch
import guard

OP='123456abcdef'
MARKER_UID='aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
PVC_UID='bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'
PV_UID='cccccccc-cccc-cccc-cccc-cccccccccccc'
NS_UID='dddddddd-dddd-dddd-dddd-dddddddddddd'
NAME='voice-nats-jsdata-d202610040049430b'
def receipt():
    return {'operation':OP,'namespace_uid':NS_UID,
        'marker':{'uid':MARKER_UID,'resourceVersion':'100','generation':'r20260930a4','previousGeneration':'r20260930a3','dataPVC':NAME},
        'pvc':{'name':NAME,'uid':PVC_UID,'pv_name':'pvc-'+PVC_UID},
        'pv':{'uid':PV_UID,'kind':'local','path':'/var/lib/rancher/k3s/storage/pvc-'+PVC_UID+'_voice-staging_'+NAME,'node':'pmdebook'}}

class FrontendScopeTest(unittest.TestCase):
    def target(self):return {"mode":"images-only","changed_services":["web"],"template_hashes":{"voice-web":"a"*64},"images":{"voice-web/web":"registry/web@sha256:"+"b"*64}}
    def test_frontend_image_only_is_derived_from_all_target_workloads(self):
        self.assertTrue(guard.frontend_image_only(self.target()))
        for change in ({"changed_services":[]},{"changed_services":["gateway"]},{"mode":"app-only"},{"template_hashes":{"voice-gateway":"a"*64}},{"images":{"voice-gateway/gateway":"registry/gateway@sha256:"+"b"*64}}):
            target=self.target();target.update(change)
            with self.subTest(change=change):self.assertFalse(guard.frontend_image_only(target))

class ClaimTest(unittest.TestCase):
    def setUp(self):
        self.r=receipt();self.calls=[]
        self.marker={'metadata':{'uid':MARKER_UID,'resourceVersion':'100'},'data':dict(self.r['marker'],phase='rollout-prepared',knownRolloutOperation=OP)}
        self.pvc={'metadata':{'uid':PVC_UID,'name':NAME},'spec':{'volumeName':'pvc-'+PVC_UID},'status':{'phase':'Bound'}}
        self.pv={'metadata':{'uid':PV_UID},'spec':{'claimRef':{'uid':PVC_UID,'name':NAME,'namespace':'voice-staging'},'local':{'path':self.r['pv']['path']},'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
        self.ns={'metadata':{'uid':NS_UID}}
        self.hub={'metadata':{'uid':'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'},'spec':{'replicas':0,'template':{'spec':{'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':NAME}}]}}}}
    def kube(self,args,body=None):
        self.calls.append((args,body))
        if args[0]=='patch':
            self.marker['metadata']['resourceVersion']='101'
            self.marker['data']['phase']='rollout-applying'
            return json.dumps(self.marker).encode()
        resource=args[1]
        return json.dumps({'configmap':self.marker,'pvc':self.pvc,'pv':self.pv,'namespace':self.ns,'deployment':self.hub}[resource]).encode()
    def assert_no_claim(self):
        with patch.object(guard,'kubectl',self.kube):
            with self.assertRaises(guard.Blocked):guard.claim(self.r)
        self.assertFalse(any(args[0]=='patch' for args,_ in self.calls))
    def test_same_name_pvc_uid_reuse_never_claims(self):
        self.pvc['metadata']['uid']='ffffffff-ffff-ffff-ffff-ffffffffffff';self.assert_no_claim()
    def test_namespace_recreation_never_claims(self):
        self.ns['metadata']['uid']='ffffffff-ffff-ffff-ffff-ffffffffffff';self.assert_no_claim()
    def test_stale_marker_rv_never_claims(self):
        self.marker['metadata']['resourceVersion']='101';self.assert_no_claim()
    def test_second_use_never_claims(self):
        self.marker['data']['phase']='rollout-applying';self.assert_no_claim()
    def test_exact_marker_claim_uses_uid_rv_cas(self):
        with patch.object(guard,'kubectl',self.kube):guard.claim(self.r)
        patches=[body for args,body in self.calls if args[0]=='patch']
        self.assertEqual(len(patches),1)
        self.assertIn({'op':'test','path':'/metadata/uid','value':MARKER_UID},patches[0])
        self.assertIn({'op':'test','path':'/metadata/resourceVersion','value':'100'},patches[0])
        self.assertEqual(patches[0][-1],{'op':'replace','path':'/data/phase','value':'rollout-applying'})

if __name__=='__main__':unittest.main()
