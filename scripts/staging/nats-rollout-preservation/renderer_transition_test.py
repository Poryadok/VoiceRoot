import copy
import hashlib
import json
import unittest
import renderer_transition as transition

REPO = 'ghcr.io/poryadok/voiceroot/nats-hub-config-renderer'
OLD = REPO+'@sha256:'+'1'*64
TARGET = REPO+'@sha256:'+'2'*64
SOURCE = 'a'*40
STORAGE = {'pvc_name':'voice-nats-jsdata-d202610040049430b','pvc_uid':'pvc-uid','pv_uid':'pv-uid','generation':'r20260930a4'}

def hub():
    return {'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-nats-pvc-candidate','namespace':'voice-staging','uid':'hub-uid','resourceVersion':'100'},'spec':{'replicas':1,'strategy':{'type':'Recreate'},'selector':{'matchLabels':{'app':'hub'}},'template':{'metadata':{'labels':{'app':'hub'}},'spec':{'initContainers':[{'name':'nats-config-renderer','image':REPO+':old-tag','command':['/renderer'],'args':['--out','/config/nats.conf'],'env':[{'name':'PRIVATE_REF','valueFrom':{'secretKeyRef':{'name':'existing','key':'account.jwt'}}}]}],'containers':[{'name':'nats','image':'nats@sha256:'+'3'*64}],'volumes':[{'name':'js','persistentVolumeClaim':{'claimName':STORAGE['pvc_name']}}]}}}}

def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()).hexdigest()

class RendererTransitionTests(unittest.TestCase):
    def rejected(self, function, *args, **kwargs):
        with self.assertRaises(transition.TransitionError) as caught:
            function(*args,**kwargs)
        self.assertEqual(str(caught.exception),'renderer_transition_rejected')

    def test_exact_tagged_original_and_only_image_substitution(self):
        h=hub(); saved=copy.deepcopy(h); d=transition.plan(h,OLD,TARGET,SOURCE,STORAGE)
        self.assertEqual(h,saved)
        self.assertEqual(d['schema'],'voice-nats-renderer-transition-v1')
        self.assertEqual(d['hub'],{'name':'voice-nats-pvc-candidate','uid':'hub-uid','resourceVersion':'100'})
        self.assertEqual(d['images'],{'old':OLD,'target':TARGET})
        target=copy.deepcopy(saved['spec']['template']);target['spec']['initContainers'][0]['image']=TARGET
        self.assertEqual(d['original_template'],saved['spec']['template']);self.assertEqual(d['target_template'],target)
        self.assertEqual(d['original_template_sha256'],digest(saved['spec']['template']));self.assertEqual(d['target_template_sha256'],digest(target))
        d['original_template']['spec']['initContainers'][0]['args'].append('changed');self.assertEqual(h,saved)

    def test_paused_ledger_rv_and_target_and_unpaused(self):
        h=hub();d=transition.plan(h,OLD,TARGET,SOURCE,STORAGE);paused=copy.deepcopy(h);paused['metadata']['resourceVersion']='101';paused['spec']['replicas']=0
        transition.validate(d,paused,STORAGE,expected_hub=paused)
        target=copy.deepcopy(paused);target['metadata']['resourceVersion']='102';target['spec']['template']=copy.deepcopy(d['target_template'])
        transition.validate(d,target,STORAGE,expected_hub=target)
        transition.validate(d,h,STORAGE,expected_hub=h,paused=False)
        self.rejected(transition.validate,d,h,STORAGE,expected_hub=h)

    def test_live_or_ledger_drift(self):
        h=hub();h['spec']['replicas']=0;d=transition.plan(h,OLD,TARGET,SOURCE,STORAGE)
        for field in ['uid','resourceVersion','replicas','broker','renderer-args','selector','strategy','pvc']:
            c=copy.deepcopy(h)
            if field in ['uid','resourceVersion']:c['metadata'][field]='999'
            elif field=='replicas':c['spec']['replicas']=1
            elif field=='broker':c['spec']['template']['spec']['containers'][0]['image']='evil'
            elif field=='renderer-args':c['spec']['template']['spec']['initContainers'][0]['args'].append('other')
            elif field=='selector':c['spec']['selector']={'matchLabels':{'app':'other'}}
            elif field=='strategy':c['spec']['strategy']={'type':'RollingUpdate'}
            else:c['spec']['template']['spec']['volumes'][0]['persistentVolumeClaim']['claimName']='other'
            self.rejected(transition.validate,d,c,STORAGE,expected_hub=h)
            if field in ['uid','broker','renderer-args','pvc']:self.rejected(transition.validate,d,c,STORAGE,expected_hub=c)

    def test_descriptor_and_storage_forgery(self):
        h=hub();h['spec']['replicas']=0;d=transition.plan(h,OLD,TARGET,SOURCE,STORAGE)
        for field in ['pvc_name','pvc_uid','pv_uid','generation']:
            s=dict(STORAGE);s[field]='foreign';self.rejected(transition.validate,d,h,s,expected_hub=h)
        for field in ['unknown','hash','non-image','source','image']:
            bad=copy.deepcopy(d)
            if field=='unknown':bad['extra']=True
            elif field=='hash':bad['target_template_sha256']='0'*64
            elif field=='source':bad['source_sha']='not-source'
            elif field=='image':bad['images']['target']=REPO+':tag'
            else:bad['target_template']['spec']['containers'][0]['image']='evil';bad['target_template_sha256']=digest(bad['target_template'])
            self.rejected(transition.validate,bad,h,STORAGE,expected_hub=h)

    def test_malformed_renderer_identity_and_inputs(self):
        for kind in ['missing','duplicate','other-name','other-repo','wrong-hub','wrong-kind','bool-replicas','missing-rv','oversize']:
            h=hub()
            if kind=='missing':h['spec']['template']['spec']['initContainers']=[]
            elif kind=='duplicate':h['spec']['template']['spec']['initContainers']*=2
            elif kind=='other-name':h['spec']['template']['spec']['initContainers'][0]['name']='other'
            elif kind=='other-repo':h['spec']['template']['spec']['initContainers'][0]['image']='evil/renderer:tag'
            elif kind=='wrong-hub':h['metadata']['name']='voice-user'
            elif kind=='wrong-kind':h['kind']='StatefulSet'
            elif kind=='bool-replicas':h['spec']['replicas']=False
            elif kind=='missing-rv':del h['metadata']['resourceVersion']
            else:h['spec']['template']['metadata']['annotations']={'huge':'x'*(2<<20)}
            self.rejected(transition.plan,h,OLD,TARGET,SOURCE,STORAGE)
        for args in [(REPO+':tag',TARGET,SOURCE),(OLD,TARGET.upper(),SOURCE),(OLD,'evil/renderer@sha256:'+'2'*64,SOURCE),(OLD,TARGET,'A'*40)]:self.rejected(transition.plan,hub(),*args,STORAGE)
        for s in [{**STORAGE,'extra':'x'},{**STORAGE,'pv_uid':None},{**STORAGE,'generation':''}]:self.rejected(transition.plan,hub(),OLD,TARGET,SOURCE,s)
        for payload in [float('nan'),object()]:
            h=hub();h['spec']['template']['bad']=payload;self.rejected(transition.plan,h,OLD,TARGET,SOURCE,STORAGE)
        deep={};cursor=deep
        for _ in range(65):cursor['child']={};cursor=cursor['child']
        h=hub();h['spec']['template']['bad']=deep;self.rejected(transition.plan,h,OLD,TARGET,SOURCE,STORAGE)
        cyclic=[];cyclic.append(cyclic);h=hub();h['spec']['template']['bad']=cyclic;self.rejected(transition.plan,h,OLD,TARGET,SOURCE,STORAGE)

    def test_private_output_exact_bytes_and_bounds(self):
        data=b'private output\n';self.assertEqual(transition.compare_outputs(data,data),{'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data),'verified':True})
        data=b'x'*(2<<20);self.assertEqual(transition.compare_outputs(data,data)['bytes'],2<<20)
        for a,b in [(b'',b''),(b'a',b'b'),(b'a\n',b'a'),(b'x'*((2<<20)+1),b'x'*((2<<20)+1)),('text','text'),(bytearray(b'a'),b'a')]:self.rejected(transition.compare_outputs,a,b)

if __name__=='__main__':unittest.main()
