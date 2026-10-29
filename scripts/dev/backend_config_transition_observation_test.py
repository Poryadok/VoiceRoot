import copy
import unittest
from unittest.mock import patch

import backend_config_transition_observation as observation
import media_backup_phase_test as fixtures


class ObservationTest(unittest.TestCase):
    def setUp(self):
        self.base = fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        self.client = fixtures.Client(self.base.lock); self.client.journal = self.base.journal
        self.base.ledger.client = self.client
        self.plan = {'source_sha':self.base.context['commit'],'targets':[
            {'workload':name,'pod_name':'voice-'+name+'-123','pod_uid':name+'-pod',
             'deployment_uid':name+'-dep','container_id':name+'-container','restart_count':0}
            for name in ('notification','user')]}
        self.base.journal.intent['backend_config_observation'] = copy.deepcopy(self.plan)
        self.base.journal.state['intent'] = copy.deepcopy(self.base.journal.intent)
        self.client_type = patch.object(observation.frontend_runtime_executor,'KubectlTransport',fixtures.Client)
        self.client_type.start(); self.addCleanup(self.client_type.stop)
        self.consumer = observation.Observation(self.base.journal,self.base.fence,self.client,self.base.lock,'ns')

    def cleanup(self, base):
        if not base.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def capture(self, workload, pod, run_bytes, guard):
        guard(); item = next(x for x in self.plan['targets'] if x['workload'] == workload)
        return {'status':'observed','identity':{**{k:item[k] for k in ('pod_name','pod_uid','deployment_uid','container_id','restart_count')},
                'namespace_uid':'ns'},'fields':{'fixture':{'state':'absent','origin':None}},
                'scope':'runtime-config-metadata-only-no-provider-validation'}

    def test_actual_fence_journal_and_ledger_one_shot(self):
        with patch.object(observation.probe,'inspect',side_effect=self.capture):
            receipt = self.consumer.run()
            self.assertEqual(set(receipt['observations']),{'notification','user'})
            self.assertEqual(self.base.ledger.row['backend_config_observation']['status'],'done')
            with self.assertRaises(observation.ObservationError): self.consumer.run()

    def test_drift_and_unknown_are_uncertain_without_replay(self):
        for fault in ('identity','lost'):
            with self.subTest(fault=fault):
                self.setUp()
                def inspect(*args):
                    if fault == 'lost': raise OSError('private sentinel')
                    result = self.capture(*args); result['identity']['pod_uid'] = 'foreign'; return result
                with patch.object(observation.probe,'inspect',side_effect=inspect):
                    with self.assertRaises(observation.ObservationError) as error: self.consumer.run()
                    self.assertNotIn('sentinel',str(error.exception))
                    with self.assertRaises(observation.ObservationError): self.consumer.run()
                self.assertTrue(self.consumer.poisoned); self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_wrong_source_rejected_before_any_probe(self):
        self.plan['source_sha'] = 'f'*40
        self.base.journal.intent['backend_config_observation'] = self.plan
        self.base.journal.state['intent'] = copy.deepcopy(self.base.journal.intent)
        with self.assertRaises(observation.ObservationError):
            observation.Observation(self.base.journal,self.base.fence,self.client,self.base.lock,'ns')

    def test_ledger_observation_drift_holds_before_terminal_receipt(self):
        def drift(*args):
            result = self.capture(*args)
            if args[0] == 'user': self.base.ledger.row['backend_config_observation']['nonce'] = 'f'*32
            return result
        with patch.object(observation.probe,'inspect',side_effect=drift), self.assertRaises(observation.ObservationError):
            self.consumer.run()
        self.assertEqual(self.base.ledger.row['status'],'uncertain')
        self.assertNotEqual(self.base.ledger.row['backend_config_observation']['status'],'done')

    def test_real_probe_is_composed_with_guarded_phase(self):
        def snapshot(workload,pod,run_bytes,guard,namespace):
            item = next(x for x in self.plan['targets'] if x['workload'] == workload)
            value = {'namespace':'voice-dev','namespace_uid':'ns','deployment':'voice-'+workload,
                'deployment_uid':item['deployment_uid'],'deployment_rv':'2','pod_name':pod,'pod_uid':item['pod_uid'],
                'pod_rv':'3','container_name':workload,'container_id':item['container_id'],
                'restart_count':0,'env':[],'envFrom':[]}
            return lambda: copy.deepcopy(value)
        with patch.object(observation.metadata,'kube_workload_snapshot',side_effect=snapshot), \
             patch.object(observation.metadata,'linux_private_bytes',return_value=b'absent'):
            result = self.consumer.run()
        self.assertEqual(result['observations']['user']['fields']['VOICE_RELEASE_PROFILE']['state'],'absent')
        self.assertEqual(result['observations']['notification']['fields']['FCM_CREDENTIALS_JSON']['state'],'absent')

    def test_local_pending_drift_or_removal_is_uncertain(self):
        for fault in ('nonce','remove'):
            with self.subTest(fault=fault):
                self.setUp()
                def drift(*args):
                    result = self.capture(*args)
                    if args[0] == 'user':
                        if fault == 'remove': del self.base.journal.state['steps'][observation.STEP]
                        else: self.base.journal.state['steps'][observation.STEP]['nonce'] = 'f'*32
                    return result
                with patch.object(observation.probe,'inspect',side_effect=drift), self.assertRaises(observation.ObservationError):
                    self.consumer.run()
                self.assertEqual(self.base.ledger.row['status'],'uncertain')
                self.assertNotEqual(self.base.ledger.row['backend_config_observation']['status'],'done')

    def test_local_terminal_receipt_drift_during_cas_is_uncertain(self):
        ledger = self.base.ledger; write = ledger.write
        def drift(revision,row):
            write(revision,row)
            if row.get('backend_config_observation',{}).get('status') == 'done':
                self.base.journal.state['steps'][observation.STEP]['receipt']['source_sha'] = 'f'*40
        with patch.object(observation.probe,'inspect',side_effect=self.capture), patch.object(ledger,'write',side_effect=drift):
            with self.assertRaises(observation.ObservationError): self.consumer.run()
        self.assertEqual(ledger.row['status'],'uncertain')


if __name__ == '__main__': unittest.main()
