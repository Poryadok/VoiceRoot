import importlib
import json
import os
from pathlib import Path
import tempfile
import threading
import types
import unittest
from http.server import ThreadingHTTPServer
from unittest.mock import patch

import core_postrollout_smoke_test as fixture
import staging_core_credentials as credentials
import staging_core_smoke_http as http
from staging_core_credentials_test import sample


class Session:
    def __init__(self):
        self.operation = 'abcdef123456'
        self.context = {'resource': 'STAGING', 'operation_id': 'fixture-operation', 'commit': 'a' * 40}
        self.uncertain_state = False
    def check(self):
        if self.uncertain_state: raise RuntimeError('uncertain')
    def uncertain(self, reason): self.uncertain_state = True


class StagingSmokeTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name); (root / 'rollout-abcdef123456').mkdir(mode=0o700)
        source = root / 'input'; source.write_text(json.dumps(sample())); source.chmod(0o600)
        self.session = Session(); self.release = object()
        receipt = {'operation_id': 'fixture-operation', 'source_sha': 'a' * 40,
                   'namespace_uid': 'fixture-ns', 'target_sha256': 'b' * 64}
        fake_release = types.SimpleNamespace(verify_completed=lambda phase, session: receipt if phase is self.release and session is self.session else None)
        self.fake_release = fake_release
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), fixture.Server)
        self.server.calls = []; self.server.registered = {'owned+2@example.test': 0, 'owned+3@example.test': 1}
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close); self.addCleanup(self.server.shutdown)
        patches = [patch.dict('sys.modules', {'resource_claim': types.SimpleNamespace(Session=Session),
                                             'guard': types.SimpleNamespace(ROOT=root), 'staging_core_release': fake_release}),
                   patch.object(credentials, '_open_input', side_effect=lambda: os.open(source, os.O_RDONLY)),
                   patch.object(http, 'ORIGIN', 'http://127.0.0.1:' + str(self.server.server_port))]
        for item in patches: item.start(); self.addCleanup(item.stop)
        import staging_core_smoke
        self.module = importlib.reload(staging_core_smoke)
        self.phase = self.module.StagingCoreSmoke(self.session, self.release)
        self.addCleanup(lambda: self.phase.journal.close() if self.phase.journal else None)

    def test_real_http_existing_accounts_and_owned_space(self):
        result = self.phase.run()
        self.assertEqual(result['space_id'], fixture.Server.space)
        self.assertEqual(self.module.verify_completed(self.phase), result)
        self.assertEqual([row[0] for row in self.server.calls], ['/api/v1/auth/login'] * 2 + [
            '/api/v1/spaces', '/api/v1/spaces/' + fixture.Server.space + '/invites',
            '/api/v1/invites/fixture-code/join', '/api/v1/spaces'])
        self.assertNotIn('unrelated-not-recorded', json.dumps(self.phase.journal.state))

    def test_lost_create_ack_marks_uncertain_and_no_replay(self):
        actual = http.create_space
        def lost(*args):
            actual(*args)
            raise TimeoutError('lost response')
        with patch.object(http, 'create_space', side_effect=lost):
            with self.assertRaises(self.module.SmokeError): self.phase.run()
        self.assertTrue(self.session.uncertain_state)
        self.assertEqual(sum(row[0] == '/api/v1/spaces' for row in self.server.calls), 1)
        with self.assertRaises(self.module.SmokeError): self.phase.run()
        self.assertEqual(sum(row[0] == '/api/v1/spaces' for row in self.server.calls), 1)

    def test_wrong_membership_holds(self):
        with patch.object(http, 'join', return_value={'space_id': fixture.Server.space,
                                                     'profile_id': fixture.Server.accounts[0][1]}):
            with self.assertRaises(self.module.SmokeError): self.phase.run()
        self.assertTrue(self.session.uncertain_state)

    def test_missing_release_prevents_public_effect(self):
        self.fake_release.verify_completed = lambda phase, session: None
        with self.assertRaises(self.module.SmokeError): self.phase.run()
        self.assertEqual(self.server.calls, [])

    def test_replaced_private_journal_refuses_completed_proof(self):
        self.phase.run()
        original = self.phase.journal
        self.phase.journal = object()
        try:
            with self.assertRaises(self.module.SmokeError): self.module.verify_completed(self.phase)
        finally:
            self.phase.journal = original


if __name__ == '__main__': unittest.main()
