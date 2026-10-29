"""Real two-Journal/Fence cut composition with synthetic Kubernetes metadata."""
import copy
from pathlib import Path
import unittest
from unittest.mock import patch

import backend_config_transition_deployment as deployment
import backend_config_transition_probe as probe
import media_operation_journal as journal
import writer_cut
import writer_cut_test as fixtures


class CompositionTest(unittest.TestCase):
    def setUp(self, two=False):
        self.base = fixtures.WriterCutTest('test_drain_and_resume_use_current_template_not_old')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        source = self.base.admission['controllers'][0]
        del self.base.client.workloads[('Deployment','voice-a')]
        source['metadata']['name'] = 'voice-user'
        source['spec']['template']['spec']['containers'][0]['name'] = 'user'
        self.base.client.workloads[('Deployment','voice-user')] = copy.deepcopy(source)
        if two:
            other = self.base.admission['controllers'][1]
            del self.base.client.workloads[('StatefulSet','voice-b')]
            other['kind'] = 'Deployment'; other['metadata']['name'] = 'voice-notification'
            other['spec']['template']['spec']['containers'][0]['name'] = 'notification'
            self.base.client.workloads[('Deployment','voice-notification')] = copy.deepcopy(other)
        self.base.journal.intent['writer_cut'] = copy.deepcopy(self.base.admission)
        self.base.journal.state['intent'] = copy.deepcopy(self.base.journal.intent)
        self.base.cut.admission = copy.deepcopy(self.base.admission); self.base.cut.intent = copy.deepcopy(self.base.journal.intent)
        self.base.cut.drain()
        targets = [{'workload':name,'pod_name':'voice-'+name+'-123','pod_uid':name+'-pod',
                    'deployment_uid':source['metadata']['uid'] if name == 'user' else ('uid-b' if two else name+'-dep'),
                    'container_id':name+'-container','restart_count':0} for name in deployment.SERVICES]
        observations = {}
        for item in targets:
            name = item['workload']; observations[name] = {'status':'observed',
                'identity':{**{k:item[k] for k in ('pod_name','pod_uid','deployment_uid','container_id','restart_count')},
                            'namespace_uid':'ns','deployment_rv':source['metadata']['resourceVersion']},
                'fields':{key:{'state':'absent','origin':None} for key in probe.FIELDS[name]},'env_sources':[],
                'scope':'runtime-config-metadata-only-no-provider-validation',
                'template_binding':'pod-env-and-deployment-template-v1'}
        receipt = {'operation_id':self.base.context['operation_id'],'source_sha':self.base.context['commit'],
                   'nonce':'c'*32,'observations':observations}
        self.base.ledger.row['backend_config_observation'] = {'status':'done','nonce':receipt['nonce'],
            'operation_id':receipt['operation_id'],'source_sha':receipt['source_sha'],
            'targets_sha256':writer_cut.digest(targets),'receipt_sha256':writer_cut.digest(receipt)}
        names = ['notification','user'] if two else ['user']
        sources = {name:copy.deepcopy(self.base.client.workloads[('Deployment','voice-'+name)]) for name in names}
        intent = {'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns',
            'backend_config_deployments':{'source_sha':self.base.context['commit'],'cut_invocation':self.base.cut.invocation,
                'services':names,'sources':sources,'observation':receipt,
                'observation_plan':{'source_sha':self.base.context['commit'],'targets':targets}}}
        self.config = journal.Journal(Path(self.base.temp.name)/'backend-config-journal',intent); self.addCleanup(self.config.close)
        self.base.cut.bind_config_plan(self.config); self.calls = []

    def cleanup(self,base):
        if not base.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def execute(self, lost=False, service='user', lag=False, drift=False):
        def apply(kind,name,patch_data):
            self.calls.append(copy.deepcopy(patch_data))
            current = self.base.client.workloads[(kind,name)]
            if (patch_data[0]['value'] != current['metadata']['uid'] or
                    patch_data[1]['value'] != current['metadata']['resourceVersion'] or patch_data[2]['value'] != current['spec']):
                raise RuntimeError('CAS failed')
            current['spec'] = copy.deepcopy(patch_data[3]['value'])
            current['metadata']['resourceVersion'] = str(int(current['metadata']['resourceVersion'])+1)
            current['metadata']['generation'] += 1
            current['status'] = {'observedGeneration':current['metadata']['generation']-int(lag),'replicas':0,'readyReplicas':0}
            if lost: raise OSError('lost response')
            return copy.deepcopy(current)
        def tick(_):
            current = self.base.client.workloads[('Deployment','voice-'+service)]
            if drift: current['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
            current['status']['observedGeneration'] = current['metadata']['generation']
        with patch.object(self.base.cut,'_patch',side_effect=apply), patch.object(deployment.time,'sleep',side_effect=tick):
            return deployment.apply_development(self.base.cut,self.config,service)

    def test_actual_cut_accepts_exact_profile_successor(self):
        receipt = self.execute(); self.assertEqual(receipt['generation'],3)
        self.assertEqual(self.base.cut.verify_drained()['active_sessions'],0)
        spec = self.base.client.workloads[('Deployment','voice-user')]['spec']
        self.assertEqual(spec['replicas'],0); self.assertEqual(spec['template']['spec']['containers'][0]['image'],'old-a')
        self.assertEqual(spec['template']['spec']['containers'][0]['env'],[{'name':'VOICE_RELEASE_PROFILE','value':'development'}])
        self.assertEqual(len(self.calls),1)

    def test_lost_apply_is_uncertain_without_replay(self):
        with self.assertRaises(deployment.DeploymentError): self.execute(lost=True)
        self.assertEqual(self.base.ledger.row['status'],'uncertain'); self.assertEqual(len(self.calls),1)
        with self.assertRaises(deployment.DeploymentError): self.execute()
        self.assertEqual(len(self.calls),1)

    def test_ordered_two_services_and_delayed_generation(self):
        self.setUp(two=True)
        with self.assertRaises(deployment.DeploymentError): self.execute(service='user')
        self.assertEqual(self.calls,[])
        self.execute(service='notification',lag=True); self.execute(service='user')
        self.assertEqual(len(self.calls),2)
        self.assertEqual(self.base.cut.verify_drained()['active_sessions'],0)

    def test_target_drift_during_controller_wait_is_uncertain(self):
        with self.assertRaises(deployment.DeploymentError): self.execute(lag=True,drift=True)
        self.assertEqual(self.base.ledger.row['status'],'uncertain')
        self.assertEqual(self.base.ledger.row['backend_config_effects']['backend-config-deployment-voice-user']['status'],'pending')


if __name__ == '__main__': unittest.main()
