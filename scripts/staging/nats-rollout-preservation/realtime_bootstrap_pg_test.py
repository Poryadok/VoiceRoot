"""Disposable isolated PostgreSQL check of the fixed bootstrap SQL."""
import subprocess
import time
import unittest
import uuid
from unittest import mock

import realtime_bootstrap as bootstrap


class PostgreSQLTest(unittest.TestCase):
    def test_scoped_creation_and_foreign_data_preserved(self):
        name = 'voice-stage-realtime-test-' + uuid.uuid4().hex[:16]
        result = subprocess.run(['docker', 'run', '-d', '--name', name, '--network', 'none',
                                 '--label', 'voice.test.owner=staging-realtime-bootstrap',
                                 '-e', 'POSTGRES_USER=voice', '-e', 'POSTGRES_HOST_AUTH_METHOD=trust',
                                 '-e', 'POSTGRES_PASSWORD=synthetic-admin-only', 'postgres:16-alpine'],
                                capture_output=True, timeout=60, check=True)
        container = result.stdout.decode().strip()
        self.addCleanup(lambda: subprocess.run(['docker', 'rm', '-f', '-v', container],
                                               capture_output=True, timeout=30, check=True))
        def execute(body, check=True):
            return subprocess.run(['docker', 'exec', '-i', container, 'sh', '-ceu', bootstrap._PSQL],
                                  input=body, capture_output=True, timeout=30, check=check)
        for _ in range(100):
            if execute(b'SELECT 1;', check=False).returncode == 0: break
            time.sleep(.2)
        else: self.fail('fixture postgres not ready')
        execute(b"CREATE DATABASE foreign_fixture;\n")
        def transport(args, *, body, **kwargs):
            self.assertEqual(args, bootstrap._COMMAND)
            return execute(body).stdout
        with mock.patch.object(bootstrap.commands, 'capture', side_effect=transport):
            self.assertEqual(bootstrap._catalog(), {'role': None, 'database': None})
            system_identifier = bootstrap._system_identifier()
            with self.assertRaises(subprocess.CalledProcessError):
                bootstrap._create_role(b'fixtureCredential_123456789012345678901234', '1')
            self.assertEqual(bootstrap._catalog(), {'role': None, 'database': None})
            bootstrap._create_role(b'fixtureCredential_123456789012345678901234', system_identifier)
            role = bootstrap._catalog()['role']
            self.assertTrue(bootstrap.valid_role(role))
            with self.assertRaises(subprocess.CalledProcessError): bootstrap._create_database(system_identifier, role['oid'] + 1)
            self.assertIsNone(bootstrap._catalog()['database'])
            bootstrap._create_database(system_identifier, role['oid'])
            final = bootstrap._catalog()
            self.assertEqual(final['database']['owner_oid'], role['oid'])
            self.assertNotEqual(execute(b'CREATE DATABASE realtime_db OWNER realtime;', check=False).returncode, 0)
            bootstrap.require_absent({'role': None, 'database': None})
            with self.assertRaises(bootstrap.Blocked): bootstrap.require_absent(final)
        preserved = execute(b"SELECT count(*) FROM pg_database WHERE datname='foreign_fixture';")
        self.assertEqual(preserved.stdout.strip(), b'1')


if __name__ == '__main__':
    unittest.main()
