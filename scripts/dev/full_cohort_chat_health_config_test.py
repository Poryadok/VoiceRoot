"""Real ChatHealth source verifier composition, synthetic upstream observations."""
import unittest
from pathlib import Path

import full_cohort_chat_health as health
import full_cohort_schema_metadata as common
import realtime_config_transition_deployment as deployment
import realtime_config_transition_joint_test


class ConfigIntegrationTest(unittest.TestCase):
    def test_chat_health_reads_owned_config_and_rejects_ledger_drift(self):
        base = realtime_config_transition_joint_test.MixedTest('test_ordered_real_producers_with_shared_cut')
        base.setUp(); self.addCleanup(base.doCleanups)
        deployment.apply(base.base.cut, base.config, 'chat')
        deps = base.config._realtime_config_dependencies
        phase = object.__new__(health.ChatHealthPhase)
        phase.fence, phase.client = base.base.fence, base.base.client
        phase.lock, phase.namespace_uid = base.base.lock, 'ns'
        phase.config_journal, phase.source_root = base.config, Path('/source')
        phase.p3_session, phase.migration = deps.session, deps.migration
        source = phase._source(base.base.cut)
        self.assertEqual(source['deployment']['source']['metadata']['name'], 'voice-chat')
        self.assertEqual(source['service']['receipt']['uid'], 'cs')
        self.assertEqual(source['secret']['uid'], 'uid-voice-realtime-p3-tls')
        base.base.ledger.row['backend_config_effects']['backend-config-deployment-voice-chat']['nonce'] = 'f' * 32
        with self.assertRaises(common.GuardLost): phase._source(base.base.cut)


if __name__ == '__main__': unittest.main()
