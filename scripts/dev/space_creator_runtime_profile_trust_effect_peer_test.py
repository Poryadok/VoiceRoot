"""Actual peer preflight verification with synthetic CA/Secret observations."""

import copy
import unittest
from unittest import mock

import dev_peer_issuer_preflight as peer
import space_creator_runtime_profile_trust_effect as effect
import space_creator_runtime_profile_trust_effect_test as fixture
import writer_cut


REAL_VERIFY_CURRENT = peer.verify_current


class PeerPreflightTest(unittest.TestCase):
    def test_actual_peer_verifier_rechecks_ca_and_all_nine_before_drain(self):
        env = fixture.EffectStateTest('test_undrained_cut_rejects_before_tool_or_secret')
        env.setUp(); self.addCleanup(env.doCleanups)
        phase = env.phase; source = env.source; issuer = phase.preflight
        issuer.record = {'status': 'done', 'ca': copy.deepcopy(source['ca']),
                         'creator_secrets': copy.deepcopy(source['creator_secrets']),
                         'receipt_sha256': writer_cut.digest(source)}
        issuer.journal.state['steps'][issuer.step] = copy.deepcopy(issuer.record) | {'receipt': copy.deepcopy(source)}
        issuer.journal.save(); env.base.ledger.row[issuer.ledger] = copy.deepcopy(issuer.record)
        with mock.patch.object(peer, 'verify_current', REAL_VERIFY_CURRENT), \
             mock.patch.object(peer.source, 'verify'), \
             mock.patch.object(issuer.package, 'verify_current', return_value={}), \
             mock.patch.object(issuer, '_ca', return_value=copy.deepcopy(source['ca'])) as ca, \
             mock.patch.object(issuer, '_creator', return_value=copy.deepcopy(source['creator_secrets'])) as members:
            self.assertFalse(env.base.cut.drained)
            plan = phase.prepare('token')
            self.assertEqual(plan['media_ca'], source['ca']); self.assertEqual(plan['mode'], 'create')
            self.assertEqual(effect.verify_prepared(phase, 'token'), plan)
            self.assertEqual(env.writes, [])
            ca.return_value = dict(source['ca'], resourceVersion='changed')
            with self.assertRaises(effect.target.metadata.GuardLost): effect.verify_prepared(phase, 'token')
            ca.return_value = copy.deepcopy(source['ca'])
            members.return_value.pop(next(iter(members.return_value)))
            with self.assertRaises(effect.target.metadata.GuardLost): effect.verify_prepared(phase, 'token')
            self.assertEqual(env.writes, [])


if __name__ == '__main__':
    unittest.main()
