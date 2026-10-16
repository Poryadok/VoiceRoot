import copy
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).parent))
import update_images as update


def deployment():
    return {'kind':'Deployment', 'metadata':{'name':'voice-user','namespace':'voice-dev',
            'labels':dict(update.LABELS),'uid':'owned-deployment','resourceVersion':'4'},
            'spec':{'replicas':1,'template':{'spec':{'volumes':[{'name':'private'}],
                    'containers':[{'name':'nats-leaf','image':'nats@sha256:'+'a'*64},
                                  {'name':'user','image':'registry/user@sha256:'+'b'*64,
                                   'env':[{'name':'UNCHANGED','value':'true'}]}]}}}}


class UpdateTests(unittest.TestCase):
    def test_only_dev_and_explicit_app_targets(self):
        update.validate_namespace({'metadata':{'name':'voice-dev','labels':dict(update.LABELS)}}, 'voice-dev')
        for name in ('voice-staging','voice-prod'):
            with self.assertRaises(ValueError):
                update.validate_namespace({'metadata':{'name':name,'labels':dict(update.LABELS)}}, name)
        with self.assertRaises(ValueError):
            update.validate_namespace({'metadata':{'name':'voice-dev'}}, 'voice-dev')
        for service in ('nats','postgres','livekit','federation','../user'):
            with self.assertRaises(ValueError):
                update.parse_images([service+'=registry/x@sha256:'+'a'*64])

    def test_immutable_image_requirement(self):
        for image in ('registry/user:latest','registry/user:main','registry/user:abc123','https://registry/user:'+ 'a'*40,
                      'registry/user@sha256:'+'a'*63,'registry/user:latest@sha256:'+'a'*64,
                      'registry/user:'+'a'*40):
            with self.assertRaises(ValueError):
                update.parse_images(['user='+image])
        for image in ('registry/user@sha256:'+ 'a'*64, 'registry:5000/user@sha256:'+'a'*64):
            self.assertEqual(update.parse_images(['user='+image]), {'user':image})
        with self.assertRaises(ValueError):
            update.parse_images(['user=registry/x@sha256:'+'a'*64]*2)

    def test_patch_changes_only_named_main_image_with_cas_guards(self):
        before = deployment()
        original = copy.deepcopy(before)
        target = 'registry/user@sha256:'+ 'c'*64
        patch, expected = update.plan_patch('user', target, before)
        self.assertEqual(before,original)
        self.assertEqual(patch[-1],{'op':'replace','path':'/spec/template/spec/containers/1/image','value':target})
        self.assertEqual(expected['spec']['template']['spec']['containers'][0],before['spec']['template']['spec']['containers'][0])
        expected['spec']['template']['spec']['containers'][1]['image']=original['spec']['template']['spec']['containers'][1]['image']
        self.assertEqual(expected['spec'],original['spec'])
        self.assertTrue(any(x['path']=='/metadata/resourceVersion' and x['op']=='test' for x in patch))
        broken=deployment();broken['spec']['template']['spec']['containers'][1]['name']='different'
        with self.assertRaises(ValueError): update.plan_patch('user',target,broken)

    def test_lock_creation_conflict_is_not_bypassed_and_release_is_conditional(self):
        class Fake:
            def __init__(self): self.calls=[];self.fail=False
            def call(self,*args,body=None):
                self.calls.append((args,body))
                if self.fail: raise RuntimeError('conflict')
                return {'metadata':{'uid':'lock-uid','resourceVersion':'7'}}
        client=Fake();lock=update.acquire_lock(client)
        update.release_lock(client,lock)
        args,body=client.calls[-1]
        self.assertIn('--raw=/api/v1/namespaces/voice-dev/configmaps/'+update.LOCK,args)
        self.assertEqual(body['preconditions'],{'uid':'lock-uid','resourceVersion':'7'})
        client.fail=True
        with self.assertRaises(RuntimeError): update.acquire_lock(client)
        self.assertEqual(client.calls[-1][0][0],'create')

    def test_preview_has_no_lock_or_mutation_and_apply_preserves_spec(self):
        class Fake:
            def __init__(self):
                self.dep=deployment();self.calls=[];self.fail_rollout=False
            def call(self,*args,body=None):
                self.calls.append((args,body))
                if args[:2]==('get','namespace'):
                    return {'metadata':{'name':'voice-dev','labels':dict(update.LABELS)}}
                if args[0]=='create':
                    self.lock=copy.deepcopy(body)
                    self.lock['metadata'].update(uid='lock',resourceVersion='1')
                    return self.lock
                if args[:2]==('get','configmap'): return self.lock
                if args[0]=='get': return copy.deepcopy(self.dep)
                if args[0]=='patch':
                    result=copy.deepcopy(self.dep)
                    result['spec']['template']['spec']['containers'][1]['image']=body[-1]['value']
                    if '--dry-run=server' not in args: self.dep=result
                    return result
                if args[0]=='rollout' and self.fail_rollout: raise RuntimeError('timeout')
                return {}
        image='registry/user@sha256:'+'c'*64
        client=Fake()
        update.update_images(client,{'user':image})
        self.assertFalse(any(a[0] in ('create','delete') for a,b in client.calls))
        self.assertTrue(all('--dry-run=server' in a for a,b in client.calls if a[0]=='patch'))
        client=Fake();before=copy.deepcopy(client.dep)
        update.update_images(client,{'user':image},apply=True)
        before['spec']['template']['spec']['containers'][1]['image']=image
        self.assertEqual(client.dep['spec'],before['spec'])
        self.assertTrue(any(a[0]=='delete' for a,b in client.calls))
        client=Fake();client.fail_rollout=True
        with self.assertRaises(RuntimeError): update.update_images(client,{'user':image},apply=True)
        self.assertFalse(any(a[0]=='delete' for a,b in client.calls))


if __name__=='__main__': unittest.main()
