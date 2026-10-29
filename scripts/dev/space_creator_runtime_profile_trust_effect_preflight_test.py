"""Pre-drain protected plan; external Secret and typed evidence are synthetic."""

import unittest
from unittest import mock

import space_creator_runtime_profile_trust_effect as effect
import space_creator_runtime_profile_trust_effect_test as fixture
import writer_cut


class PreflightTest(unittest.TestCase):
    def setUp(self):
        self.fixture = fixture.EffectStateTest('test_undrained_cut_rejects_before_tool_or_secret')
        self.fixture.setUp(); self.addCleanup(self.fixture.doCleanups)
        self.phase = self.fixture.phase

    def test_missing_preflight_is_read_only_and_run_requires_actual_drain(self):
        plan = self.phase.prepare('token')
        self.assertEqual(plan['mode'], 'create'); self.assertEqual(self.fixture.writes, [])
        self.assertEqual(effect.verify_prepared(self.phase, 'token'), plan)
        with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.fixture.writes, [])
        self.fixture.base.cut.drain()
        self.assertEqual(self.phase.run('token')['mode'], 'create')
        self.assertEqual(len(self.fixture.writes), 1)

    def test_changed_secret_after_plan_refuses_before_effect(self):
        self.fixture.existing(b'incompatible')
        self.assertEqual(self.phase.prepare('token')['mode'], 'replace')
        self.assertEqual(self.fixture.writes, [])
        self.fixture.secret['metadata']['resourceVersion'] = '3'
        self.fixture.base.cut.drain()
        with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.fixture.writes, [])

    def test_private_key_store_is_rejected_in_preflight(self):
        self.fixture.existing(b'private')
        with self.assertRaises(effect.TrustEffectError): self.phase.prepare('token')
        self.assertEqual(self.fixture.writes, [])

    def test_missing_plan_refuses_without_effect(self):
        self.fixture.base.cut.drain()
        with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.fixture.writes, []); self.phase._tool.assert_not_called()

    def test_secret_changed_after_verification_refuses(self):
        self.fixture.existing(b'incompatible'); self.phase.prepare('token')
        self.fixture.base.cut.drain()
        original_read = self.phase._read_store
        reads = []
        def changed():
            reads.append(True)
            if len(reads) == 2: self.fixture.secret['metadata']['resourceVersion'] = '3'
            return original_read()
        with mock.patch.object(self.phase, '_read_store', side_effect=changed):
            with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.fixture.writes, [])

    def test_preflight_tamper_refuses_before_write(self):
        self.phase.prepare('token'); self.fixture.base.cut.drain()
        self.phase.journal.state['steps'][effect.PLAN_STEP]['mode'] = 'preserve'
        with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.fixture.writes, [])

    def test_incompatible_immutable_secret_is_not_replaced(self):
        self.fixture.existing(b'incompatible'); self.fixture.secret['immutable'] = True
        with self.assertRaises(effect.TrustEffectError): self.phase.prepare('token')
        self.assertEqual(self.fixture.writes, [])

    def test_target_cannot_generate_store_is_refused_before_drain(self):
        self.phase._tool.side_effect = ValueError('fixture generation unavailable')
        with self.assertRaises(effect.TrustEffectError): self.phase.prepare('token')
        self.assertEqual(self.fixture.writes, [])
        self.assertFalse(self.fixture.base.cut.drained)

    def test_missing_peer_identity_is_rejected_before_any_tool(self):
        source = self.fixture.source
        source['creator_secrets'].pop(next(iter(source['creator_secrets'])))
        peer = self.phase.preflight
        peer.record = {'status': 'done', 'receipt_sha256': writer_cut.digest(source)}
        peer.journal.state['steps'][peer.step] = dict(peer.record, receipt=source.copy())
        peer.journal.save(); self.fixture.base.ledger.row[peer.ledger] = peer.record.copy()
        with self.assertRaises(effect.target.metadata.GuardLost): self.phase.prepare('token')
        self.assertEqual(self.fixture.writes, []); self.phase._tool.assert_not_called()

    def test_compatible_preflight_preserves_secret_after_drain(self):
        self.fixture.existing(b'compatible')
        self.assertEqual(self.phase.prepare('token')['mode'], 'preserve')
        self.fixture.base.cut.drain()
        self.assertEqual(self.phase.run('token')['mode'], 'preserve')
        self.assertEqual(self.fixture.writes, [])


if __name__ == '__main__':
    unittest.main()
