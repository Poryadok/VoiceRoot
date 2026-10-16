"""Actual canonical compiler must preserve owned pre-fence desired objects."""
import copy
import json
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
import compiler
import guard
import nonnats_plan
import paused_recovery
import source_plan
from controller import Blocked


def deployment(name, replicas=1):
    return {'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-'+name,
        'namespace':guard.NS,'uid':name+'-uid','resourceVersion':'original'},
        'spec':{'replicas':replicas,'template':{'spec':{'containers':[{'name':name,
            'image':'example.invalid/'+name+'@sha256:'+'b'*64,
            'env':[{'name':name.upper()+'_R2_SIGNING_ENDPOINT','value':'https://safe.invalid'}]}]}}}}


class Kube:
    def __init__(self):
        self.rows={('Deployment','voice-'+n):deployment(n) for n in ('file','user')}
        self.rows[('Secret','voice-app-secrets')]={'kind':'Secret','metadata':{'name':'voice-app-secrets',
            'namespace':guard.NS,'uid':'secret-uid','resourceVersion':'1'},'data':{}}
        self.rows[('StatefulSet','voice-minio')]={'kind':'StatefulSet','metadata':{'name':'voice-minio',
            'namespace':guard.NS,'uid':'minio-uid','resourceVersion':'1'},'spec':{'template':{'spec':{
                'containers':[{'name':'minio','env':[{'name':'MINIO_API_CORS_ALLOW_ORIGIN','value':'https://safe.invalid'}]}]}}}}
    def get(self,kind,name):return copy.deepcopy(self.rows[(kind,name)])
    def run(self,args,body=None,**kwargs):
        if args!=['apply','--dry-run=server','-f','/dev/stdin','-o','json']:
            raise AssertionError('only modeled server dry-run permitted')
        return copy.deepcopy(body)


class PausedCompilerTests(unittest.TestCase):
    def fixture(self, root):
        for relative in (*compiler.SOURCE_MANIFESTS,'deploy/nats/acl-intent.yaml',
                         'scripts/staging/preflight-resend-key.sh','scripts/storage/apply-signing-env.sh',
                         'scripts/storage/apply-minio-cors.sh'):
            path=root/relative;path.parent.mkdir(parents=True,exist_ok=True);path.write_text('public fixture\n')
        p={'registry':'example.invalid','tag':paused_recovery.SOURCE,'mode':'images-only','changed_services':['story'],
           'images':{'voice-story/story':'example.invalid/story@sha256:'+'a'*64},'contract':{'scripts':[]},
           'enrolled':['voice-story'],'generation':'r20260930a4','dataPVC':'voice-nats-jsdata-d202610040049430b',
           's3_signing_endpoint':'https://safe.invalid','web_origin':'https://safe.invalid','livekit_host':''}
        parameters=root/'parameters.json';parameters.write_text(json.dumps(p))
        return parameters

    def compile(self,root,parameters,kube,producer=None):
        out=root/('target-'+paused_recovery.SOURCE+'.json')
        with patch.object(compiler,'ProducerKube',return_value=kube), \
             patch.object(compiler,'compatible_bootstrap',return_value=[]), \
             patch.object(compiler,'rendered_source',return_value=[deployment('story')]), \
             patch.object(source_plan,'_configuration'),patch.object(source_plan,'_ingress'), \
             patch.object(nonnats_plan,'_secret'):
            # Canonical compile_plan, _Source.add, descriptor validation and
            # actual compile_target execute. Only unrelated private values,
            # optional ingress, YAML/bootstrap and server normalization modeled.
            if producer is None:compiler.main([str(root),str(parameters),str(out)])
            else:compiler.main([str(root),str(parameters),str(out)],producer=producer)
        value=json.loads(out.read_bytes());out.unlink();return value

    def captured(self,kube,plan):
        original={n:kube.get('Deployment',n) for n in ('voice-file','voice-user')}
        for n in original:
            kube.rows[('Deployment',n)]['spec']['replicas']=0
            kube.rows[('Deployment',n)]['metadata']['resourceVersion']='owned'
        owned={n:kube.get('Deployment',n) for n in original}
        with patch.object(nonnats_plan,'_secret'):
            # Capture actual production nonnats bindings from original objects.
            class OriginalKube:
                def get(self,kind,name):return copy.deepcopy(original[name]) if name in original else kube.get(kind,name)
            binding=nonnats_plan.preflight(OriginalKube(),plan,'images-only')
        stage=SimpleNamespace(kube=kube,snapshots=owned,original_snapshots=original)
        state={'operation':paused_recovery.OPERATION,'repair_adoption':'root-adoption',
               'target':{'mode':'images-only','changed_services':['story'],'tag':paused_recovery.SOURCE},
               'nonnats_binding':binding}
        return stage,state

    def test_actual_compiler_owned_fence_preserves_exact_original_target(self):
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);parameters=self.fixture(root);kube=Kube()
            original=self.compile(root,parameters,kube)
            stage,state=self.captured(kube,original['nonnats'])
            live=self.compile(root,parameters,kube)
            self.assertNotEqual(live,original) # demonstrated old normal producer bug
            producer=paused_recovery.PausedOriginalProducer(stage,state)
            fixed=self.compile(root,parameters,kube,producer)
            producer.verify()
            self.assertEqual(fixed,original)
            self.assertEqual(kube.get('Deployment','voice-file')['spec']['replicas'],0)

    def test_owned_drift_and_wrong_original_descriptor_are_rejected(self):
        changes={
            'UID':lambda k,s,t:k.rows[('Deployment','voice-file')]['metadata'].__setitem__('uid','changed'),
            'RV':lambda k,s,t:k.rows[('Deployment','voice-file')]['metadata'].__setitem__('resourceVersion','changed'),
            'template':lambda k,s,t:k.rows[('Deployment','voice-file')]['spec']['template']['spec']['containers'][0].__setitem__('image','changed'),
            'replicas':lambda k,s,t:k.rows[('Deployment','voice-file')]['spec'].__setitem__('replicas',1),
            'desired':lambda k,s,t:t['nonnats_binding']['objects'][1].__setitem__('desired',{}),
            'unowned':lambda k,s,t:s.snapshots.pop('voice-file'),
            'original-nonscale':lambda k,s,t:s.original_snapshots['voice-file']['spec']['template']['spec']['containers'][0].__setitem__('image','changed'),
            'operation':lambda k,s,t:t.__setitem__('operation','other'),
        }
        for label,change in changes.items():
            with self.subTest(label=label),tempfile.TemporaryDirectory() as td:
                root=Path(td);p=self.fixture(root);kube=Kube();pack=self.compile(root,p,kube)
                stage,state=self.captured(kube,pack['nonnats']);change(kube,stage,state)
                with self.assertRaises(Blocked):paused_recovery.PausedOriginalProducer(stage,state)

    def test_projection_rechecks_and_leaves_unowned_reads_live(self):
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);p=self.fixture(root);kube=Kube();pack=self.compile(root,p,kube)
            stage,state=self.captured(kube,pack['nonnats']);producer=paused_recovery.PausedOriginalProducer(stage,state)
            self.assertEqual(producer.get('StatefulSet','voice-minio'),kube.get('StatefulSet','voice-minio'))
            projected=producer.get('Deployment','voice-file')
            self.assertEqual(projected['metadata']['resourceVersion'],'owned')
            self.assertEqual(projected['spec']['replicas'],1)
            kube.rows[('Deployment','voice-user')]['metadata']['resourceVersion']='new'
            with self.assertRaises(Blocked):producer.verify()
            with self.assertRaises(Blocked):producer.get('Deployment','voice-file')


if __name__=='__main__':unittest.main()
