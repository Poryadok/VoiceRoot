import copy
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixtures
import media_operation_journal as journal
import realtime_config_transition_space_phase as phase


class Client(fixtures.Client):
    def call(self,verb,kind,name,*args):
        if kind=='deployment': return copy.deepcopy(self.deployment)
        if kind=='service': return {'metadata':{'uid':'svc','resourceVersion':'4'}}
        raise ValueError()


class Package:
    def verify_current(self,token):
        if token!='token': raise ValueError()


class SpacePhaseTest(unittest.TestCase):
    def setUp(self):
        self.base=fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        intent=copy.deepcopy(self.base.journal.intent)
        intent['realtime_space_https']={'source_root':'/source','space_pod':'space-0','destination':'/private'}
        self.journal=journal.Journal(Path(self.base.temp.name)/'space-journal',intent); self.addCleanup(self.journal.close)
        self.client=Client(self.base.lock); self.client.journal=self.journal; self.base.ledger.client=self.client
        self.client.deployment={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-space','namespace':'voice-dev',
            'uid':'dep','resourceVersion':'3','generation':4},'spec':{'replicas':1,'template':{'spec':{}}}}
        package=Package(); package.fence=self.base.fence; package.lock=self.base.lock; package.namespace_uid='ns'
        self.context=self.base.context; self.workload={'deployment_uid':'dep','deployment_rv':'3'}
        self.public_objects={name:{'uid':'uid-'+name,'resource_version':'1'} for name in phase.public.PUBLIC}
        import hashlib
        raw=b'{"keys":[]}'; self.observation={'source_sha':self.context['commit'],'namespace_uid':'ns',
            'identity':{'workload':self.workload,'service':{'uid':'svc','resource_version':'4'}},
            'public_objects':self.public_objects,'jwks_sha256':hashlib.sha256(raw).hexdigest(),
            'jwks_bytes':raw,'scope':'space-pod-https-through-kubernetes-tunnel'}
        self.patches=[patch.object(phase.frontend_runtime_executor,'KubectlTransport',Client),
            patch.object(phase.full_cohort_package_phase,'PackagePhase',Package),
            patch.object(phase.media_operation_guard,'verify'),
            patch.object(phase.space,'capture',side_effect=lambda *a:copy.deepcopy(self.observation)),
            patch.object(phase.public,'_meta',side_effect=lambda run,guard,kind,name:self.public_objects[name])]
        for item in self.patches: item.start(); self.addCleanup(item.stop)
        self.subject=phase.SpacePhase(self.journal,self.base.fence,self.client,self.base.lock,'ns',package)

    def cleanup(self,base):
        if not base.doCleanups(): raise RuntimeError('cleanup failed')

    def test_journaled_capture_and_cut_predecessor(self):
        receipt=self.subject.run('token'); self.assertEqual(receipt['observation']['jwks_sha256'],self.observation['jwks_sha256'])
        cut=SimpleNamespace(fence=self.base.fence,lock=self.base.lock,namespace_uid='ns',_guard=lambda:None,
                            admission={'controllers':[copy.deepcopy(self.client.deployment)]})
        with patch.object(phase.writer_cut,'WriterCut',SimpleNamespace):
            self.assertEqual(phase.verify(self.subject,cut,'token'),receipt)
            cut.admission['controllers'][0]['spec']['replicas']=2
            with self.assertRaises(phase.SpacePhaseError): phase.verify(self.subject,cut,'token')

    def test_capture_failure_is_uncertain_and_no_retry(self):
        with patch.object(phase.space,'capture',side_effect=OSError('private-sentinel')):
            with self.assertRaises(phase.SpacePhaseError) as error: self.subject.run('token')
        self.assertNotIn('private-sentinel',str(error.exception)); self.assertEqual(self.base.ledger.row['status'],'uncertain')
        with self.assertRaises(phase.SpacePhaseError): self.subject.run('token')


if __name__=='__main__': unittest.main()
