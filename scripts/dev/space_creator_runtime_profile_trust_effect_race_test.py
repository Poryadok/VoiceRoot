"""Protected preflight custody drift with synthetic external Secrets."""

import base64
import copy
import os
import unittest
from unittest import mock

import space_creator_runtime_profile_trust_effect as effect
import space_creator_runtime_profile_trust_effect_test as fixture
import space_creator_runtime_profile_jdk as jdk
import writer_cut


class PreflightCustodyTest(unittest.TestCase):
    def setUp(self):
        self.fixture = fixture.EffectStateTest('test_undrained_cut_rejects_before_tool_or_secret')
        self.fixture.setUp(); self.addCleanup(self.fixture.doCleanups)
        self.fixture.existing(b'compatible')
        self.phase = self.fixture.phase
        self.plan = self.phase.prepare('token')

    def test_same_uid_and_version_but_changed_bytes_refuse(self):
        original = copy.deepcopy(self.fixture.secret)
        self.fixture.secret['data']['other'] = base64.b64encode(b'foreign').decode()
        with self.assertRaises(effect.TrustEffectError): effect.verify_prepared(self.phase, 'token')
        self.assertEqual(self.fixture.writes, [])
        self.fixture.secret = original
        self.assertEqual(effect.verify_prepared(self.phase, 'token'), self.plan)

    def test_same_bytes_on_replaced_key_inode_refuse(self):
        key = self.phase._key()
        replacement = self.phase.journal.path / 'replacement.key'
        replacement.write_bytes(key); replacement.chmod(0o600)
        os.replace(replacement, self.phase.journal.path / effect.KEY_FILE)
        with self.assertRaises(effect.TrustEffectError): effect.verify_prepared(self.phase, 'token')
        self.assertEqual(self.fixture.writes, [])

    def test_jdk_receipt_changed_between_plan_verification_and_effect_refuses(self):
        self.fixture.base.cut.drain()
        original = self.phase._dependencies; calls = []
        def changed(token):
            calls.append(True)
            if len(calls) == 2:
                self.fixture.roots['root_fingerprints'] = ['c' * 64]
                peer = self.phase.jdk
                peer.record = {'status': 'done', 'receipt_sha256': writer_cut.digest(self.fixture.roots)}
                peer.journal.state['steps'][jdk.STEP] = copy.deepcopy(peer.record) | {'receipt': copy.deepcopy(self.fixture.roots)}
                peer.journal.save(); self.fixture.base.ledger.row[jdk.LEDGER] = copy.deepcopy(peer.record)
            return original(token)
        with mock.patch.object(self.phase, '_dependencies', side_effect=changed):
            with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.fixture.writes, [])


if __name__ == '__main__':
    unittest.main()
