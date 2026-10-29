import base64
import copy
import hashlib
import sys
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixtures
import media_operation_journal as journal
import realtime_config_transition_space_health as health


class Client(fixtures.Client):
    def call(self,verb,kind,name,*args):
        if kind=='pod': return copy.deepcopy(self.pod)
        if kind=='service': return {'metadata':{'name':'voice-space','namespace':'voice-dev','uid':'svc','resourceVersion':'4'}}
        if kind=='endpoints': return {'metadata':{'name':'voice-space','namespace':'voice-dev','uid':'ep','resourceVersion':'5'}}
        raise ValueError()


class Package:
    def verify_current(self,token):
        if token!='token': raise ValueError()
        return {'package':'fixed'}


class Plan: pass


class Image:
    def _guard(self): pass
    def _valid_pod(self,name,pod,target): return pod['status']['ready'] is True


class Cut:
    def current_resumed_proof(self): return copy.deepcopy(self.proof)


class HealthTest(unittest.TestCase):
    def setUp(self):
        self.base=fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        intent=copy.deepcopy(self.base.journal.intent)
        intent['realtime_space_health']={'source_root':'/source','space_pod':'voice-space-abc','destination':'/private'}
        self.journal=journal.Journal(Path(self.base.temp.name)/'health-journal',intent); self.addCleanup(self.journal.close)
        self.client=Client(self.base.lock); self.client.journal=self.journal; self.base.ledger.client=self.client
        self.client.pod={'metadata':{'name':'voice-space-abc','uid':'pod','resourceVersion':'2'},
            'status':{'ready':True,'containerStatuses':[{'name':'space','containerID':'ctr','imageID':'image','restartCount':0}]}}
        self.cut=Cut(); self.cut.fence=self.base.fence; self.cut.lock=self.base.lock; self.cut.namespace_uid='ns'
        self.cut.proof={'controllers':[{'kind':'Deployment','name':'voice-space','uid':'dep','generation':4,'spec':{},'replicas':1}]}
        self.package=Package(); self.package.fence=self.base.fence; self.package.lock=self.base.lock; self.package.namespace_uid='ns'
        self.plan=Plan(); self.plan.cut=self.cut; self.plan.plan={'source_root':'/source'}
        self.image=Image(); self.image.fence=self.base.fence; self.image.lock=self.base.lock; self.image.namespace_uid='ns'
        data=b'{"keys":[]}'; self.expected={'observation':{'jwks_bytes':base64.b64encode(data).decode()}}
        self.observation={'identity':{'workload':{'pod_uid':'pod'},'service':{'uid':'svc','resource_version':'4'},
            'endpoints':{'uid':'ep','resourceVersion':'5'}},'jwks_bytes':data,'jwks_sha256':hashlib.sha256(data).hexdigest(),
            'public_objects':{},'scope':'space-pod-https-through-kubernetes-tunnel'}
        self.patches=[patch.object(health.frontend_runtime_executor,'KubectlTransport',Client),
            patch.object(health.full_cohort_package_phase,'PackagePhase',Package),patch.object(health.writer_cut,'WriterCut',Cut),
            patch.object(health.planned,'SpacePlanPhase',Plan),patch.object(health.full_cohort_image_apply,'FullCohortImageApply',Image),
            patch.object(health.media_operation_guard,'verify'),patch.object(health.planned,'verify',side_effect=lambda *a:copy.deepcopy(self.expected)),
            patch.object(health.space_service,'verify_space_service',return_value={'receipt':{'uid':'svc','resourceVersion':'4'}}),
            patch.object(health.space,'capture',side_effect=lambda *a:copy.deepcopy(self.observation))]
        for item in self.patches: item.start(); self.addCleanup(item.stop)
        self.config=SimpleNamespace(intent={'fixed':True},state={'intent':{'fixed':True},'steps':{health.space_service.STEP:{'status':'done'}}})
        self.subject=health.SpaceHealthPhase(self.journal,self.base.fence,self.client,self.base.lock,'ns',self.package,self.cut,self.plan,self.image,self.config)

    def cleanup(self,base):
        if not base.doCleanups(): raise RuntimeError('cleanup failed')

    def test_journaled_post_resume_health(self):
        receipt=self.subject.run('token')
        self.assertEqual(health.verify_completed(self.subject,self.cut,'token'),receipt)
        self.client.pod['status']['ready']=False
        with self.assertRaises(health.HealthError): health.verify_completed(self.subject,self.cut,'token')

    def test_fixed9_health_binds_live_ca_and_server_secret(self):
        bound=health.peer_authority.BoundFixed9SpaceAuthority(self.cut,object(),object(),object(),object(),'token')
        self.subject.plan_phase=bound
        expected={'schema':'voice-dev-space-delegated-v1','operation_id':self.base.context['operation_id'],
                  'source_sha':self.base.context['commit'],'namespace_uid':'ns','public_jwks':{'keys':[]},
                  'media_ca':{'uid':'ca','resourceVersion':'2'},'tls':{'uid':'tls','resourceVersion':'3'}}
        self.observation['public_objects']={'voice-media-ca':{'uid':'ca','resource_version':'2'},
            'voice-space-media-server-tls':{'uid':'tls','resource_version':'3'}}
        with patch.object(health.peer_authority,'_space',return_value=(expected,b'{"keys":[]}')), \
             patch.object(health.public,'_meta',side_effect=lambda run,guard,kind,name:copy.deepcopy(self.observation['public_objects'][name])):
            receipt=self.subject.run('token'); self.assertEqual(health.verify_completed(self.subject,self.cut,'token'),receipt)
            expected['tls']['resourceVersion']='4'
            with self.assertRaises(health.HealthError):health.verify_completed(self.subject,self.cut,'token')

    def test_wrong_public_keys_hold_without_receipt(self):
        self.observation['jwks_bytes']=b'{"keys":[1]}'
        with self.assertRaises(health.HealthError): self.subject.run('token')
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_foreign_service_receipt_cannot_qualify_health(self):
        with patch.object(health.space_service,'verify_space_service',return_value={'receipt':{'uid':'foreign','resourceVersion':'4'}}):
            with self.assertRaises(health.HealthError): self.subject.run('token')
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_service_record_drift_during_final_guard_holds(self):
        original=self.subject._guard; count=0
        def mutate():
            nonlocal count
            count+=1; original()
        with patch.object(self.subject,'_guard',side_effect=mutate): self.subject.run('token')
        self.subject.record=None; self.journal.state['steps'].clear(); self.base.ledger.row.pop(health.LEDGER)
        calls=0
        def drift():
            nonlocal calls
            calls+=1; original()
            if calls==count: self.config.state['steps'][health.space_service.STEP]['status']='foreign'
        with patch.object(self.subject,'_guard',side_effect=drift):
            with self.assertRaises(health.HealthError): self.subject.run('token')
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_reopened_health_rejects_service_record_drift_at_last_guard(self):
        self.subject.run('token'); original=self.subject._guard; count=0
        def observe():
            nonlocal count
            count+=1; original()
        with patch.object(self.subject,'_guard',side_effect=observe): health.verify_completed(self.subject,self.cut,'token')
        calls=0
        def drift():
            nonlocal calls
            calls+=1; original()
            if calls==count: self.config.state['steps'][health.space_service.STEP]['status']='foreign'
        with patch.object(self.subject,'_guard',side_effect=drift):
            with self.assertRaises(health.HealthError): health.verify_completed(self.subject,self.cut,'token')

    def test_pod_drift_during_final_source_recheck_holds(self):
        original=self.subject._sources; calls=0
        def sources(token):
            nonlocal calls
            calls+=1; result=original(token)
            if calls==3: self.client.pod['metadata']['resourceVersion']='9'
            return result
        with patch.object(self.subject,'_sources',side_effect=sources):
            with self.assertRaises(health.HealthError): self.subject.run('token')
        self.assertEqual(calls,3); self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_newly_issued_public_jwks_is_expected_not_old_http(self):
        class Issuer: pass
        issuer=Issuer(); issuer.fence=self.base.fence; issuer.lock=self.base.lock; issuer.namespace_uid='ns'
        proof={'schema':'voice-dev-space-p3-issuer-v1','operation_id':self.base.context['operation_id'],
            'source_sha':self.base.context['commit'],'namespace_uid':'ns','public_jwks':{'keys':[]},
            'scope':'space-two-secret-create-only'}
        module=SimpleNamespace(Session=Issuer,verify_completed=lambda _:copy.deepcopy(proof))
        with patch.dict(sys.modules,{'space_p3_issuer_create':module}):
            subject=health.SpaceHealthPhase(self.journal,self.base.fence,self.client,self.base.lock,'ns',self.package,self.cut,issuer,self.image,self.config)
            receipt=subject.run('token')
            self.assertEqual(health.verify_completed(subject,self.cut,'token'),receipt)


if __name__=='__main__': unittest.main()
