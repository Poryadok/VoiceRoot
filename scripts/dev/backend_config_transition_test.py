import copy
import unittest
from unittest.mock import patch

import backend_config_transition as transition
import writer_cut_test as fixtures


class ServiceTransitionTest(unittest.TestCase):
    def setUp(self):
        self.base = fixtures.WriterCutTest('test_drain_and_resume_use_current_template_not_old')
        self.addCleanup(self.cleanup_base); self.base.setUp()
        original = self.base.admission['controllers'][0]
        del self.base.client.workloads[('Deployment','voice-a')]
        original['metadata']['name'] = 'voice-realtime'
        original['spec']['template']['metadata'] = {'labels':{'app':'realtime'}}
        self.base.client.workloads[('Deployment','voice-realtime')] = copy.deepcopy(original)
        self.service = {'apiVersion':'v1','kind':'Service', 'metadata':{'name':'voice-realtime',
            'namespace':'voice-dev','uid':'service-uid','resourceVersion':'3'},
            'spec':{'type':'ClusterIP','clusterIP':'10.0.0.9','selector':{'app':'realtime'},
                    'ports':[{'name':'grpc','port':9090,'targetPort':9090,'protocol':'TCP'}]}}
        self.base.journal.intent['writer_cut'] = copy.deepcopy(self.base.admission)
        self.base.journal.intent['backend_config_service'] = copy.deepcopy(self.service)
        self.base.journal.state['intent'] = copy.deepcopy(self.base.journal.intent)
        self.base.cut.admission = copy.deepcopy(self.base.admission)
        self.base.cut.intent = copy.deepcopy(self.base.journal.intent)
        self.base.cut.drain(); self.patches = []
        self.effect = transition.ServiceTransition(self.base.journal,self.base.fence,self.base.client,
                                                   self.base.lock,'ns',self.base.cut)

    def cleanup_base(self):
        if not self.base.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def run_effect(self, lost=False, drift=False):
        def get(): return copy.deepcopy(self.service)
        def apply(patch):
            self.patches.append(copy.deepcopy(patch))
            if (patch[0]['value'] != self.service['metadata']['uid'] or
                    patch[1]['value'] != self.service['metadata']['resourceVersion'] or patch[2]['value'] != self.service['spec']):
                raise RuntimeError('CAS conflict')
            self.service['spec'] = copy.deepcopy(patch[-1]['value'])
            self.service['metadata']['resourceVersion'] = '4'
            if lost: raise OSError('lost response')
            response = copy.deepcopy(self.service)
            if drift: self.service['spec']['ports'].pop()
            return response
        with patch.object(self.effect,'_get',side_effect=get), patch.object(self.effect,'_patch',side_effect=apply):
            return self.effect.apply_realtime_service()

    def test_exact_service_transition_preserves_drained_workloads_and_refuses_replay(self):
        receipt = self.run_effect(); self.assertEqual(receipt['uid'],'service-uid')
        self.assertNotIn('generation',receipt); self.assertEqual(len(self.patches),1)
        self.base.cut.verify_drained()
        with self.assertRaises(transition.ConfigError): self.run_effect()
        self.assertEqual(len(self.patches),1)

    def test_lost_response_retains_uncertain_and_never_replays(self):
        with self.assertRaises(transition.ConfigError): self.run_effect(lost=True)
        self.assertEqual(self.base.ledger.row['status'],'uncertain'); self.assertEqual(len(self.patches),1)
        with self.assertRaises(transition.ConfigError): self.run_effect()
        self.assertEqual(len(self.patches),1)

    def test_existing_service_generation_remains_unchanged(self):
        self.service['metadata']['generation'] = 0
        self.base.journal.intent['backend_config_service']['metadata']['generation'] = 0
        self.base.journal.state['intent'] = copy.deepcopy(self.base.journal.intent)
        self.base.cut.intent = copy.deepcopy(self.base.journal.intent)
        self.effect.intent = copy.deepcopy(self.base.journal.intent)
        self.assertEqual(self.run_effect()['generation'],0)

    def test_changed_service_or_cut_rejected_before_patch(self):
        self.service['metadata']['uid'] = 'other'
        with self.assertRaises(transition.ConfigError): self.run_effect()
        self.assertEqual(self.patches,[])

    def test_post_patch_drift_is_uncertain(self):
        with self.assertRaises(transition.ConfigError): self.run_effect(drift=True)
        self.assertEqual(self.base.ledger.row['status'],'uncertain'); self.assertEqual(len(self.patches),1)
        self.assertNotEqual(self.base.ledger.row['backend_config_effects'][transition.STEP]['status'],'done')

    def test_lost_pending_cas_never_submits_patch(self):
        ledger = self.base.ledger; write = ledger.write
        def lost(revision,row):
            write(revision,row)
            if transition.STEP in row.get('backend_config_effects',{}): raise OSError('lost response')
        with patch.object(ledger,'write',side_effect=lost), self.assertRaises(transition.ConfigError): self.run_effect()
        self.assertEqual(self.patches,[]); self.assertTrue(self.base.cut.poisoned)
        with self.assertRaises(transition.ConfigError): self.run_effect()

    def test_drifted_writer_fails_before_service_patch(self):
        item = self.base.client.workloads[('Deployment','voice-realtime')]
        item['spec']['template']['metadata']['labels']['app'] = 'other'
        with self.assertRaises(transition.ConfigError): self.run_effect()
        self.assertEqual(self.patches,[]); self.assertEqual(self.base.ledger.row['status'],'uncertain')


if __name__ == '__main__': unittest.main()
