import json
import os
from pathlib import Path
import tempfile
import types
import unittest
from unittest.mock import patch

import media_operation_journal
import staging_core_credentials as credentials


def sample():
    return {'schema': 'voice-staging-test-accounts-v1', 'provenance': 'fixture', 'accounts': [
        {'email': 'owned+2@example.test', 'password': 'fixture-only-2'},
        {'email': 'owned+3@example.test', 'password': 'fixture-only-3'}]}


class Session:
    def __init__(self):
        self.context = {'resource': 'STAGING', 'operation_id': 'fixture-operation', 'commit': 'a' * 40}
        self.checks = 0
    def check(self): self.checks += 1


class CredentialInputTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        self.session = Session()
        self.journal = media_operation_journal.Journal(Path(self.temp.name) / 'journal', {'resource_context': self.session.context})
        self.addCleanup(self.journal.close)
        self.file = Path(self.temp.name) / 'accounts.json'
        self.file.write_text(json.dumps(sample()), encoding='utf-8'); self.file.chmod(0o600)
        self.release = object()
        receipt = {'operation_id': 'fixture-operation', 'source_sha': 'a' * 40,
                   'namespace_uid': 'fixture-ns', 'target_sha256': 'b' * 64}
        fake_release = types.SimpleNamespace(verify_completed=lambda release, session: receipt if release is self.release and session is self.session else None)
        patches = [patch.dict('sys.modules', {'resource_claim': types.SimpleNamespace(Session=Session), 'staging_core_release': fake_release}),
                   patch.object(credentials, '_open_input', side_effect=lambda: os.open(self.file, os.O_RDONLY))]
        for item in patches: item.start(); self.addCleanup(item.stop)
        self.phase = credentials.CredentialInput(self.session, self.release, self.journal)

    def test_one_private_capture_and_no_replay(self):
        accounts = self.phase.capture()
        self.assertEqual(len(accounts), 2)
        record = self.journal.state['steps']['staging-core-credentials']
        self.assertEqual(len(record['hmac']), 64)
        self.assertNotEqual(record['hmac'], record['hmac_key'])
        self.assertGreaterEqual(self.session.checks, 6)
        self.assertEqual(self.phase.verify(), accounts)
        record['input']['accounts'][0]['password'] = 'changed-fixture'
        self.journal.save()
        with self.assertRaises(credentials.CredentialError): self.phase.verify()
        with self.assertRaises(credentials.CredentialError): self.phase.capture()

    def test_unknown_fields_duplicate_and_wrong_slots_rejected(self):
        for value in (sample() | {'unexpected': 1}, sample() | {'accounts': list(reversed(sample()['accounts']))}):
            with self.assertRaises(credentials.CredentialError): credentials.decode(json.dumps(value).encode())
        with self.assertRaises(credentials.CredentialError):
            credentials.decode(b'{"schema":"a","schema":"b"}')

    def test_closed_journal_cannot_capture(self):
        self.journal.close()
        with self.assertRaises(credentials.CredentialError): self.phase.capture()


if __name__ == '__main__': unittest.main()
