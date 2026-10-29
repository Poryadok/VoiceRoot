"""Protected preflight custody drift with synthetic external Secrets."""

import base64
import copy
import os
import unittest

import space_creator_runtime_profile_trust_effect as effect
import space_creator_runtime_profile_trust_effect_test as fixture


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


if __name__ == '__main__':
    unittest.main()
