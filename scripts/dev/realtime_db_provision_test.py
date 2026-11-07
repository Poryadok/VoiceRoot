import unittest
import copy
import base64
import hashlib
from unittest import mock

import realtime_db_provision as provision


class SQLContract(unittest.TestCase):
    def test_catalog_and_transport_pin_system_namespace(self):
        for name in ('pg_roles','pg_auth_members','pg_database'):
            self.assertIn('pg_catalog.'+name,provision.CATALOG_SQL)
        pg=object.__new__(provision.KubePostgres); pg.operation_guard=lambda:None
        with mock.patch.object(provision.subprocess,'run',return_value=mock.Mock(returncode=0,stdout=b'{}')) as run:
            pg.catalog()
        command=run.call_args.args[0]
        self.assertTrue(any('search_path=pg_catalog,public' in str(arg) for arg in command))

    def test_create_only_fixed_role_and_database(self):
        password = b'fixtureCredential_123456789012345678901234'
        role = provision.role_sql(password)
        self.assertIn('CREATE ROLE realtime LOGIN', role)
        self.assertIn('NOSUPERUSER NOCREATEDB NOCREATEROLE', role)
        self.assertNotIn('ALTER ROLE', role)
        self.assertEqual(provision.database_sql(), 'CREATE DATABASE realtime_db OWNER realtime;\n')

    def test_invalid_password_never_reaches_sql(self):
        for value in (b'', b"x'; DROP DATABASE chat_db; --", b'x' * 31, b'x' * 129):
            with self.subTest(value=value), self.assertRaises(ValueError):
                provision.role_sql(value)

    def test_transport_rejects_other_sql(self):
        pg = provision.KubePostgres(lambda: True)
        with mock.patch.object(pg, '_psql') as send:
            for sql in ('DROP DATABASE realtime_db;', 'CREATE DATABASE chat_db;', provision.role_sql(b'x' * 32) + 'DROP ROLE postgres;'):
                with self.assertRaises(provision.RecoveryRequired): pg.execute(sql)
            send.assert_not_called()
            pg.execute(provision.database_sql())
            pg.execute(provision.role_sql(b'x' * 32))
            self.assertEqual(send.call_count, 2)


class PhaseContract(unittest.TestCase):
    def test_secret_url_has_no_override_or_extra_parameters(self):
        password = b'fixtureCredential_123456789012345678901234'
        base = 'postgres://realtime:' + password.decode() + '@voice-postgres:5432/realtime_db?sslmode=disable'
        ref = {'name': 'voice-realtime-db-fixture', 'uid': 'u', 'resourceVersion': '4', 'password_sha256': hashlib.sha256(password).hexdigest()}
        journal = mock.Mock(intent={'realtime_credential': ref})
        client = mock.Mock()
        worker = provision.Provision(journal, mock.Mock(), client, {}, 'ns', mock.Mock(), password)
        def secret(url):
            return {'metadata': {'name': ref['name'], 'namespace': 'voice-dev', 'uid': 'u', 'resourceVersion': '4'},
                    'immutable': True, 'type': 'Opaque', 'data': {'DATABASE_URL': base64.b64encode(url.encode()).decode()}}
        client.call.return_value = secret(base)
        worker._secret()
        for suffix in ('&host=attacker', '&user=other', '&port=123', '&dbname=other', '&service=other', '&options=-cfoo=bar', '&sslmode=disable'):
            client.call.return_value = secret(base + suffix)
            with self.subTest(suffix=suffix), self.assertRaises(provision.RecoveryRequired): worker._secret()

    def test_unknown_create_response_is_not_retried(self):
        password = b'fixtureCredential_123456789012345678901234'
        context = {'operation_id': 'media-c1-' + 'a' * 32}
        credential = {'name': 'voice-realtime-db-fixture', 'uid': 'u', 'resourceVersion': '4', 'password_sha256': hashlib.sha256(password).hexdigest()}
        intent = {'resource_context': context, 'namespace_uid': 'ns', 'postgres': {'pod_uid': 'p'}, 'realtime_credential': credential}
        journal = mock.Mock(poisoned=False, intent=copy.deepcopy(intent), state={'intent': copy.deepcopy(intent), 'steps': {}})
        fence = mock.Mock(context=context)
        ledger = fence.ledger
        row = {'status': 'active'}
        ledger.read.side_effect = lambda: (copy.deepcopy(row), ('uid', '1'))
        def write(_, value): row.clear(); row.update(copy.deepcopy(value))
        ledger.write.side_effect = write
        fence.owned.return_value = True
        pg = mock.Mock()
        pg.snapshot.return_value = {'pod_uid': 'p'}
        pg.catalog.return_value = {'role': None, 'database': None}
        pg.execute.side_effect = TimeoutError('lost response')
        with mock.patch.object(provision, 'guard'), mock.patch.object(provision.Provision, '_secret'):
            worker = provision.Provision(journal, fence, mock.Mock(), {}, 'ns', pg, password)
            with self.assertRaises(provision.RecoveryRequired): worker.run()
            self.assertEqual(pg.execute.call_count, 1)
            fence.uncertain.assert_called_once()
            with self.assertRaises(provision.RecoveryRequired): worker.run()
            self.assertEqual(pg.execute.call_count, 1)


if __name__ == '__main__':
    unittest.main()
