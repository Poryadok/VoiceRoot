"""Real shared Journal/Fence integration; no Kubernetes or PostgreSQL network."""
import base64
import copy
import hashlib
from pathlib import Path
import unittest

import media_backup_phase_test as fixtures
import media_operation_journal
import realtime_db_provision as provision


class JournalIntegration(unittest.TestCase):
    def setUp(self):
        self.base = fixtures.PhaseTest()
        self.base.setUp()
        self.addCleanup(self.base.doCleanups)
        self.password = b'fixtureCredential_123456789012345678901234'
        self.secret = {'name': 'voice-realtime-db-fixture', 'uid': 'secret', 'resourceVersion': '10',
                       'password_sha256': hashlib.sha256(self.password).hexdigest()}
        self.intent = {'resource_context': self.base.context, 'namespace_uid': 'ns',
                       'postgres': {'pod_uid': 'p'}, 'realtime_credential': self.secret}
        self.journal = media_operation_journal.Journal(Path(self.base.temp.name) / 'realtime', self.intent)
        self.addCleanup(self.journal.close)
        base = self.base
        secret = self.secret
        url = 'postgres://realtime:' + self.password.decode() + '@voice-postgres:5432/realtime_db?sslmode=disable'
        class Client(fixtures.Client):
            def call(self, *args):
                if args[1] == 'secret':
                    return {'metadata': {'name': secret['name'], 'namespace': 'voice-dev', 'uid': secret['uid'],
                                         'resourceVersion': secret['resourceVersion']}, 'immutable': True, 'type': 'Opaque',
                            'data': {'DATABASE_URL': base64.b64encode(url.encode()).decode()}}
                return super().call(*args)
        self.client = Client(base.lock)
        self.role = {'oid': 12, 'login': True, 'superuser': False, 'createdb': False, 'createrole': False,
                     'inherit': False, 'replication': False, 'bypassrls': False, 'memberships': 0,
                     'connection_limit': -1, 'valid_until': None, 'config': None}
        test = self
        class PG:
            def __init__(self): self.state = {'role': None, 'database': None}; self.lost = False
            def snapshot(self): return {'pod_uid': 'p'}
            def catalog(self): return copy.deepcopy(self.state)
            def execute(self, sql):
                if sql == provision.database_sql(): self.state['database'] = {'oid': 13, 'owner_oid': 12}
                else: self.state['role'] = copy.deepcopy(test.role)
                if self.lost: raise TimeoutError()
        self.pg = PG()
        self.worker = provision.Provision(self.journal, base.fence, self.client, base.lock, 'ns', self.pg, self.password)

    def test_real_guard_and_durable_ledger(self):
        receipt = self.worker.run()
        self.assertEqual((receipt['role_oid'], receipt['database_oid']), (12, 13))
        self.assertEqual(self.journal.state['steps']['realtime-db-database']['status'], 'done')
        self.assertEqual(self.base.ledger.row['realtime_db_provision']['steps']['database']['status'], 'done')
        self.assertNotIn(self.password.decode(), str(self.journal.state))
        self.assertNotIn(self.password.decode(), str(self.base.ledger.row))

    def test_lost_response_retains_uncertain_and_never_retries(self):
        self.pg.lost = True
        with self.assertRaises(provision.RecoveryRequired): self.worker.run()
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')
        self.assertEqual(self.journal.state['steps']['realtime-db-role']['status'], 'pending')
        with self.assertRaises(provision.RecoveryRequired): self.worker.run()
        self.assertIsNone(self.pg.state['database'])


if __name__ == '__main__': unittest.main()
