import copy
import hashlib
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixtures
import media_operation_journal as journal
import realtime_config_transition_space_plan_phase as phase
import realtime_config_transition_space_plan_test as plan_fixture


class Client(fixtures.Client):
    def call(self,verb,kind,name,*args):
        if kind=='deployment': return copy.deepcopy(self.deployment)
        if kind=='pod': return copy.deepcopy(self.pod)
        raise ValueError()


class Package:
    def verify_current(self,token):
        if token!='token': raise ValueError()


class PlanPhaseTest(unittest.TestCase):
    def setUp(self):
        self.base=fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        intent=copy.deepcopy(self.base.journal.intent)
        intent['realtime_space_public_plan']={'source_root':'/source','space_pod':'voice-space-abc','destination':'/private'}
        self.journal=journal.Journal(Path(self.base.temp.name)/'plan-journal',intent); self.addCleanup(self.journal.close)
        self.client=Client(self.base.lock); self.client.journal=self.journal; self.base.ledger.client=self.client
        podspec={'containers':[{'name':'space','image':'image','env':[],'volumeMounts':[]}],'volumes':[]}
        self.client.deployment={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-space','namespace':'voice-dev',
            'uid':'dep','resourceVersion':'3','generation':4},'spec':{'replicas':1,'template':{'spec':copy.deepcopy(podspec)}}}
        self.client.pod={'metadata':{'uid':'pod','resourceVersion':'2'},'spec':podspec}
        self.cut=SimpleNamespace(fence=self.base.fence,lock=self.base.lock,namespace_uid='ns',_guard=lambda:None,
            admission={'controllers':[copy.deepcopy(self.client.deployment)]})
        self.package=Package(); self.package.fence=self.base.fence; self.package.lock=self.base.lock; self.package.namespace_uid='ns'
        refs={name:{'uid':'uid-'+name,'resource_version':'1'} for name in list(phase.public.PUBLIC)+['voice-app-config']}
        secret={'state':'present','uid':'signer','resource_version':'4','keys':['current.pem','next.pem','active-kid']}
        value=plan_fixture.jwks(); self.observation={'source_sha':self.base.context['commit'],'namespace_uid':'ns',
            'space_identity':{'pod_uid':'pod','pod_rv':'2','deployment_uid':'dep','deployment_rv':'3'},
            'signer':{'configuration':{'signing_dir':'exact','active_kid_ref':'exact','signing_mount':'exact'},'secret':secret},
            'public_sources':refs,'public_der_sha256':{},'jwks_sha256':hashlib.sha256(value).hexdigest(),'jwks_bytes':value,
            'scope':'space-current-http-pod-public-keys-not-https-readiness'}
        self.patches=[patch.object(phase.frontend_runtime_executor,'KubectlTransport',Client),
            patch.object(phase.full_cohort_package_phase,'PackagePhase',Package),patch.object(phase.writer_cut,'WriterCut',SimpleNamespace),
            patch.object(phase.media_operation_guard,'verify'),patch.object(phase.plan,'capture',side_effect=lambda *a:copy.deepcopy(self.observation)),
            patch.object(phase.public,'_signer_identity',return_value=secret),
            patch.object(phase.public,'_meta',side_effect=lambda run,guard,kind,name:refs[name])]
        for item in self.patches: item.start(); self.addCleanup(item.stop)
        self.subject=phase.SpacePlanPhase(self.journal,self.base.fence,self.client,self.base.lock,'ns',self.package,self.cut)

    def cleanup(self,base):
        if not base.doCleanups(): raise RuntimeError('cleanup failed')

    def test_journaled_plan_and_cut_identity(self):
        result=self.subject.run('token'); self.assertEqual(phase.verify(self.subject,self.cut,'token'),result)
        self.cut.admission['controllers'][0]['spec']['replicas']=2
        with self.assertRaises(phase.PlanPhaseError): phase.verify(self.subject,self.cut,'token')

    def test_old_pod_template_and_lost_capture_hold(self):
        self.client.pod['spec']['containers'][0]['env']=[{'name':'changed','value':'x'}]
        with self.assertRaises(phase.PlanPhaseError): self.subject.run('token')
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_standard_service_account_injection_only(self):
        pod=self.client.pod['spec']; name='kube-api-access-abc'
        pod['containers'][0]['volumeMounts'].append({'name':name,'mountPath':'/var/run/secrets/kubernetes.io/serviceaccount','readOnly':True})
        pod['volumes'].append({'name':name,'projected':{'defaultMode':420,'sources':[
            {'serviceAccountToken':{'path':'token','expirationSeconds':3607}},
            {'configMap':{'name':'kube-root-ca.crt','items':[{'key':'ca.crt','path':'ca.crt'}]}},
            {'downwardAPI':{'items':[{'path':'namespace','fieldRef':{'apiVersion':'v1','fieldPath':'metadata.namespace'}}]}}]}})
        self.assertTrue(phase._pod_matches(self.client.deployment['spec']['template']['spec'],pod))
        pod['volumes'][0]['projected']['sources'][0]['serviceAccountToken']['path']='foreign'
        self.assertFalse(phase._pod_matches(self.client.deployment['spec']['template']['spec'],pod))


if __name__=='__main__': unittest.main()
