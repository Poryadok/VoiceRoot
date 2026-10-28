"""Run the real DEV updater and durable ledger together on a CAS Kubernetes fake."""
import copy
import unittest
from unittest.mock import patch

import resource_fencing as fencing
from resource_fencing_test import card, identity
import update_images as update
from update_images_test import deployment


class Kube:
    def __init__(self):
        self.maps={}; self.dep=deployment(); self.writes=[]; self.lost_patch_response=False
        self.fail_rollout=False; self.lost_lock_response=False
        self.on_fence_write=lambda:None

    def call(self,*args,body=None):
        verb=args[0]
        if args[:2]==('get','namespace'):
            return {'metadata':{'name':update.NS,'labels':update.LABELS}}
        if args[:2]==('get','configmap'): return copy.deepcopy(self.maps.get(args[2],{}))
        if args[:2]==('get','deployment'): return copy.deepcopy(self.dep)
        if verb=='create':
            name=body['metadata']['name']
            if name in self.maps: raise RuntimeError('already exists')
            row=copy.deepcopy(body); row['metadata'].update(uid=name+'-uid',resourceVersion='1')
            self.maps[name]=row; self.writes.append(('create',name))
            if name==update.LOCK and self.lost_lock_response: raise RuntimeError('lock response lost')
            if name==fencing.LEDGER: self.on_fence_write()
            return copy.deepcopy(row)
        if args[:2]==('patch','configmap'):
            row=self.maps[args[2]]
            for operation in body[:-1]:
                key=operation['path'].rsplit('/',1)[-1]
                if row['metadata'][key]!=operation['value']: raise RuntimeError('CAS conflict')
            row['data']=copy.deepcopy(body[-1]['value'])
            row['metadata']['resourceVersion']=str(int(row['metadata']['resourceVersion'])+1)
            self.writes.append(('patch',args[2])); self.on_fence_write()
            return copy.deepcopy(row)
        if args[:2]==('patch','deployment'):
            if body[0]['value']!=self.dep['metadata']['uid'] or body[1]['value']!=self.dep['metadata']['resourceVersion']:
                raise RuntimeError('workload CAS conflict')
            row=copy.deepcopy(self.dep)
            row['spec']['template']['spec']['containers'][1]['image']=body[-1]['value']
            if '--dry-run=server' not in args:
                self.dep=row; self.writes.append(('patch','deployment'))
                if self.lost_patch_response: raise RuntimeError('response lost after server committed patch')
            return row
        if verb=='delete':
            row=self.maps[update.LOCK]
            if body['preconditions']!={k:row['metadata'][k] for k in ('uid','resourceVersion')}:
                raise RuntimeError('delete CAS conflict')
            del self.maps[update.LOCK]; self.writes.append(('delete',update.LOCK)); return {}
        if verb=='rollout':
            if self.fail_rollout: raise RuntimeError('deployment not available')
            return {}
        raise AssertionError('unexpected Kubernetes command '+str(args))


class UpdaterFenceTests(unittest.TestCase):
    def fixture(self):
        context=identity(); current=card(context); kube=Kube(); reports=[]
        fence=fencing.Fence(context,lambda:copy.deepcopy(current),fencing.KubeLedger(kube),on_uncertain=reports.append)
        return kube,fence,current,reports

    def apply(self,kube,fence):
        with patch.object(update,'is_ancestor',return_value=True):
            update.update_images(kube,{'user':identity()['images']['voice-user/user']},apply=True,fence=fence)

    def test_real_updater_closes_durable_generation_but_retains_tt_window(self):
        kube,fence,current,reports=self.fixture(); self.apply(kube,fence)
        self.assertNotIn(update.LOCK,kube.maps)
        row=fencing.KubeLedger(kube).read()[0]
        self.assertEqual(row['status'],'complete')
        self.assertEqual(row['actual'],identity()['images'])
        self.assertEqual(current['state'],'held')
        self.assertEqual(reports,[])
        with self.assertRaises(RuntimeError): self.apply(kube,fence)
        self.assertEqual(kube.writes.count(('patch','deployment')),1)

    def test_takeover_after_check_before_patch_blocks_workload_and_retains_lock(self):
        kube,fence,current,reports=self.fixture()
        def takeover():
            row=fencing.KubeLedger(kube).read()[0]
            if row.get('step')=='patch-user': current['claim']='33333333-3333-4333-8333-333333333333'
        kube.on_fence_write=takeover
        with self.assertRaises(RuntimeError): self.apply(kube,fence)
        self.assertNotIn(('patch','deployment'),kube.writes)
        self.assertIn(update.LOCK,kube.maps)
        self.assertEqual(fencing.KubeLedger(kube).read()[0]['status'],'uncertain')

    def test_lost_response_after_actual_patch_cannot_be_retried_as_safe_failure(self):
        kube,fence,_,reports=self.fixture(); kube.lost_patch_response=True
        with self.assertRaises(RuntimeError): self.apply(kube,fence)
        self.assertEqual(kube.dep['spec']['template']['spec']['containers'][1]['image'],identity()['images']['voice-user/user'])
        self.assertIn(update.LOCK,kube.maps)
        self.assertEqual(fencing.KubeLedger(kube).read()[0]['status'],'uncertain')
        self.assertEqual(len(reports),1)
        with self.assertRaises(RuntimeError): self.apply(kube,fence)
        self.assertEqual(kube.writes.count(('patch','deployment')),1)

    def test_fresh_observation_drift_and_rollback_stop_before_durable_admission(self):
        kube,fence,_,_=self.fixture()
        kube.dep['spec']['template']['spec']['containers'][1]['image']='registry/user@sha256:'+'d'*64
        with self.assertRaises(RuntimeError): self.apply(kube,fence)
        self.assertNotIn(fencing.LEDGER,kube.maps)
        self.assertNotIn(update.LOCK,kube.maps)
        kube,fence,_,_=self.fixture()
        with patch.object(update,'is_ancestor',return_value=False), self.assertRaises(RuntimeError):
            update.update_images(kube,{'user':identity()['images']['voice-user/user']},apply=True,fence=fence)
        self.assertNotIn(('patch','deployment'),kube.writes)

    def test_existing_updater_lock_remains_mandatory(self):
        kube,fence,_,_=self.fixture(); kube.maps[update.LOCK]={'foreign':'do not touch'}
        with self.assertRaises(RuntimeError): self.apply(kube,fence)
        self.assertEqual(kube.maps[update.LOCK],{'foreign':'do not touch'})
        self.assertNotIn(fencing.LEDGER,kube.maps)

    def test_unchanged_unready_workload_cannot_close_as_verified_rollout(self):
        kube,fence,current,reports=self.fixture()
        current['observed']['images']=copy.deepcopy(identity()['images'])
        kube.dep['spec']['template']['spec']['containers'][1]['image']=identity()['images']['voice-user/user']
        kube.fail_rollout=True
        with self.assertRaises(RuntimeError): self.apply(kube,fence)
        self.assertIn(update.LOCK,kube.maps)
        self.assertEqual(fencing.KubeLedger(kube).read()[0]['status'],'uncertain')

    def test_unknown_lock_create_response_keeps_lock_identity_and_reports_recovery(self):
        kube,fence,_,reports=self.fixture(); kube.lost_lock_response=True
        with self.assertRaises(RuntimeError): self.apply(kube,fence)
        self.assertIn(update.LOCK,kube.maps)
        self.assertEqual(len(reports),1)
        self.assertEqual(kube.maps[update.LOCK]['data']['resource_operation'],identity()['operation_id'])

    def test_mixed_batch_checks_unchanged_member_rollout_after_changed_member(self):
        class MixedKube(Kube):
            def call(self,*args,body=None):
                if args[:3]==('get','deployment','voice-story'):
                    row=deployment(); row['metadata'].update(name='voice-story',uid='story-uid')
                    row['spec']['template']['spec']['containers'][1].update(name='story',image='registry/story@sha256:'+'c'*64)
                    return row
                if args[:3]==('rollout','status','deployment/voice-story'):
                    raise RuntimeError('unchanged story unavailable')
                return super().call(*args,body=body)
        context=identity(); story='registry/story@sha256:'+'c'*64
        context['images']['voice-story/story']=story
        current=card(context); current['observed']['images']['voice-story/story']=story
        kube=MixedKube(); fence=fencing.Fence(context,lambda:current,fencing.KubeLedger(kube))
        with patch.object(update,'is_ancestor',return_value=True),self.assertRaises(RuntimeError):
            update.update_images(kube,{'user':context['images']['voice-user/user'],'story':story},apply=True,fence=fence)
        self.assertIn(('patch','deployment'),kube.writes)
        self.assertIn(update.LOCK,kube.maps)
        self.assertEqual(fencing.KubeLedger(kube).read()[0]['status'],'uncertain')


if __name__=='__main__': unittest.main()
