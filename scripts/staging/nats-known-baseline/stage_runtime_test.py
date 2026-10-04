import copy
import unittest
import json
from unittest.mock import patch

from controller import Blocked
from stage_runtime import Kube,Staging,MARKER,HUB,LEAVES,fence_objects,pv_storage_path
from docker_runtime import NATS_IMAGE


class StageFenceTests(unittest.TestCase):
    def test_storage_selector_rejects_ambiguity_escape_foreign_node_and_reused_uid(self):
        uid='5fae59b0-aea2-405a-a0f7-287e94a9b6a9';pvuid='4a11a528-81f9-4404-9757-f4f32c411310';name='voice-nats-jsdata-r20260930a4'
        expected='/var/lib/rancher/k3s/storage/pvc-'+uid+'_voice-staging_'+name
        original={'metadata':{'uid':pvuid},'spec':{'claimRef':{'uid':uid},'local':{'path':expected},'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
        for kind in ('local','hostPath'):
            row=copy.deepcopy(original);row['spec'][kind]=row['spec'].pop('local')
            self.assertEqual(pv_storage_path(row,uid,name,pvuid),expected)
        for label in ('both','none','escape','foreign-node','missing-affinity','claim-reuse','pv-reuse','relative'):
            with self.subTest(label=label):
                row=copy.deepcopy(original)
                if label=='both':row['spec']['hostPath']={'path':expected}
                if label=='none':del row['spec']['local']
                if label=='escape':row['spec']['local']['path']=expected+'/../other'
                if label=='relative':row['spec']['local']['path']='relative'
                if label=='foreign-node':row['spec']['nodeAffinity']['required']['nodeSelectorTerms'][0]['matchExpressions'][0]['values']=['other']
                if label=='missing-affinity':del row['spec']['nodeAffinity']
                if label=='claim-reuse':row['spec']['claimRef']['uid']='00000000-0000-0000-0000-000000000001'
                if label=='pv-reuse':row['metadata']['uid']='00000000-0000-0000-0000-000000000001'
                with self.assertRaises(Blocked):pv_storage_path(row,uid,name,pvuid)

    def test_final_storage_accepts_captured_local_pv(self):
        uid='5fae59b0-aea2-405a-a0f7-287e94a9b6a9';pvuid='4a11a528-81f9-4404-9757-f4f32c411310';name='voice-nats-jsdata-r20260930a4'
        path='/var/lib/rancher/k3s/storage/pvc-'+uid+'_voice-staging_'+name
        claim={'metadata':{'uid':uid,'name':name},'spec':{'volumeName':'pv'}}
        pv={'metadata':{'uid':pvuid},'spec':{'claimRef':{'uid':uid},'local':{'path':path},'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
        class ReadOnlyKube:
            def get(self,kind,name):return claim if kind=='pvc' else pv
        stage=Staging(ReadOnlyKube(),'0049430b0dbb',{},lambda _:self.fail('unexpected mutation'))
        stage.final_claim=claim;stage.final_pv=pv;stage.final_path=path
        with patch.object(stage,'verify_closed',return_value={'verified':True}):stage.verify_final_storage()

    def test_fence_projection_rejects_malformed_or_unbounded_metadata(self):
        valid={'kind':'Pod','metadata':{'uid':'00000000-0000-0000-0000-000000000001','ownerReferences':[]},'spec':{'volumes':[]}}
        bad=[]
        for key,value in (('kind','Secret'),('uid','not-uid'),('owners',[{'uid':'bad'}]),('claim','../escape'),('extra',{})):
            row=copy.deepcopy(valid)
            if key=='kind':row['kind']=value
            if key=='uid':row['metadata']['uid']=value
            if key=='owners':row['metadata']['ownerReferences']=value
            if key=='claim':row['spec']['volumes']=[{'persistentVolumeClaim':{'claimName':value}}]
            if key=='extra':row['spec']['containers']=value
            bad.append({'items':[row]})
        bad.extend(({'items':[valid]*2049},{'items':{},'extra':[]}))
        for row in bad:
            with self.subTest(row_count=len(row['items'])):
                with self.assertRaisesRegex(Blocked,'fence_projection_invalid'):fence_objects(Kube(lambda args:row))

    def test_fence_verifies_when_unused_pod_templates_exceed_capture_cap(self):
        def captured(argv,**kwargs):
            if 'pods,replicasets' in argv:
                if argv[-1]=='json':raise Blocked('command_output_limit')
                self.assertTrue(argv[-1].startswith('go-template='))
                self.assertNotIn('containers',argv[-1])
                self.assertNotIn('annotations',argv[-1])
                return b'{"items":[]}'
            return json.dumps({'metadata':{'uid':'hub'},'spec':{'replicas':0,'template':{}}}).encode()
        stage=Staging(Kube(),'abcd1234',{'source_claim':'source'},lambda row:None)
        stage.snapshots={'hub':{'metadata':{'uid':'hub'},'spec':{'replicas':0,'template':{}}}}
        with patch('stage_runtime.capture',side_effect=captured):stage.no_pods(('hub',),timeout=1)

    def test_secret_metadata_projection_maps_atomic_json_values(self):
        calls=[]
        kube=Kube(lambda args:calls.append(args) or ['tls-uid','100'])
        self.assertEqual(kube.secret_meta('tls'),{'uid':'tls-uid','resourceVersion':'100'})
        self.assertEqual(calls,[['get','secret','tls','-o',
            "jsonpath-as-json={.metadata['uid','resourceVersion']}"]])

    def test_secret_metadata_projection_rejects_invalid_shape(self):
        for value in ([],['uid'],['uid','100','extra'],['uid',100],['','100'],{'data':'never'}):
            with self.subTest(value=value):
                with self.assertRaises(Blocked):Kube(lambda args:value).secret_meta('tls')

    def test_second_preflight_rejects_changed_template(self):
        names=(HUB,'voice-gateway',*('voice-'+s for s in LEAVES))
        rows={name:{'metadata':{'name':name,'uid':name,'resourceVersion':'10'},
            'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'nats','image':NATS_IMAGE}],
                'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':'original'}}]}}}} for name in names}
        class Client:
            def get(self,kind,name):
                if kind=='namespace':return {'metadata':{'uid':'namespace'}}
                if kind=='configmap':return {'metadata':{'uid':'marker','resourceVersion':'10'},'data':{'phase':'active','generation':'original'}}
                if kind=='pvc':return {'metadata':{'uid':'pvc'},'spec':{'volumeName':'pv'}}
                if kind=='pv':return {'metadata':{'uid':'pv'}}
                if kind=='service':return {'metadata':{'uid':'svc'}}
                return copy.deepcopy(rows[name])
            def run(self,args):return {'items':[]}
        expected={'namespace_uid':'namespace','marker_uid':'marker','generation':'original',
            'source_claim':'original','source_claim_uid':'pvc','source_pv_uid':'pv',
            'deployment_uids':{name:name for name in names}}
        stage=Staging(Client(),'abcd1234',expected,lambda row:None)
        stage.preflight()
        rows['voice-user']['spec']['template']['spec']['containers'][0]['image']='changed'
        with self.assertRaises(Blocked):stage.preflight()

    def test_rescaled_deployment_is_not_a_verified_fence(self):
        class Client:
            def get(self,kind,name):return {'metadata':{'uid':'hub'},'spec':{'replicas':1,'template':{}}}
            def run(self,args):return {'items':[]}
        stage=Staging(Client(),'abcd1234',{},lambda row:None)
        stage.snapshots={'hub':{'metadata':{'uid':'hub'},'spec':{'replicas':0,'template':{}}}}
        with self.assertRaises(Blocked):stage.no_pods(('hub',),timeout=1)

    def test_manual_source_claim_pod_is_not_a_verified_fence(self):
        class Client:
            def get(self,kind,name):return {'metadata':{'uid':'hub'},'spec':{'replicas':0,'template':{}}}
            def run(self,args):return {'items':[{'kind':'Pod','metadata':{'uid':'00000000-0000-0000-0000-000000000001','ownerReferences':[]},'spec':{'volumes':[{'persistentVolumeClaim':{'claimName':'source'}}]}}]}
        stage=Staging(Client(),'abcd1234',{'source_claim':'source'},lambda row:None)
        stage.snapshots={'hub':{'metadata':{'uid':'hub'},'spec':{'replicas':0,'template':{}}}}
        with self.assertRaisesRegex(Blocked,'staging_fence_timeout'):stage.no_pods(('hub',),timeout=0.01)

    def test_exact_uid_and_rv_bound_in_actual_patch(self):
        calls=[]
        kube=Kube(lambda args,body=None,timeout=30:calls.append(args) or {})
        kube.cas('deployment',{'metadata':{'name':'voice-user','uid':'u','resourceVersion':'10'}},
            [{'op':'replace','path':'/spec/replicas','value':0}])
        import json
        patch=json.loads(calls[0][calls[0].index('--patch')+1])
        self.assertEqual(patch[:2],[{'op':'test','path':'/metadata/uid','value':'u'},
            {'op':'test','path':'/metadata/resourceVersion','value':'10'}])

    def test_marker_race_never_scales_workload(self):
        class Client:
            def __init__(self):self.mutations=[]
            def get(self,kind,name):return {'metadata':{'uid':'marker','resourceVersion':'11'}}
            def cas(self,*args):self.mutations.append(args)
        kube=Client();stage=Staging(kube,'abcd1234',{},lambda row:None)
        stage.marker={'metadata':{'uid':'marker','resourceVersion':'10'}}
        with self.assertRaises(Blocked):stage.fence()
        self.assertEqual(kube.mutations,[])

    def test_lost_maintenance_token_prevents_restart(self):
        class Client:
            def __init__(self):self.mutations=[]
            def get(self,kind,name):return {'metadata':{'uid':'marker','resourceVersion':'11'},
                'data':{'phase':'active','knownBaselineOperation':'other'}}
            def cas(self,*args):self.mutations.append(args)
        kube=Client();stage=Staging(kube,'abcd1234',{},lambda row:None)
        stage.marker={'metadata':{'uid':'marker','resourceVersion':'10'}}
        with self.assertRaises(Blocked):stage.restart()
        self.assertEqual(kube.mutations,[])


if __name__=='__main__':unittest.main()
