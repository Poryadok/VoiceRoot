"""Isolated PG16/C1 read transport plus real Session; Kube/TT are fixtures."""
import base64
import copy
import json
import shutil
import subprocess
import time
import unittest
import uuid
from unittest.mock import patch

import commands
import realtime_bootstrap as c1
import realtime_bootstrap_existing as existing
import resource_claim
from resource_claim_test import context, card, MemoryJournal


@unittest.skipUnless(shutil.which('docker'), 'isolated docker fixture unavailable')
class ExistingPostgresTest(unittest.TestCase):
    def test_real_pg_auth_catalog_and_current_secret_drift(self):
        name = 'voice-realtime-existing-fixture-' + uuid.uuid4().hex[:16]
        result = subprocess.run(['docker', 'run', '-d', '--name', name, '--network', 'none',
            '--label', 'voice.test.owner=realtime-existing-pg', '-e', 'POSTGRES_USER=voice',
            '-e', 'POSTGRES_PASSWORD=fixture-admin-only', 'postgres:16-alpine'],
            capture_output=True, check=True, timeout=90)
        cid = result.stdout.decode().strip()
        self.addCleanup(lambda: subprocess.run(['docker', 'rm', '-f', '-v', cid], capture_output=True, timeout=30))
        def pg(script, body=b'', check=True):
            return subprocess.run(['docker', 'exec', '-i', cid, 'sh', '-ceu', script], input=body,
                                  capture_output=True, check=check, timeout=30)
        for _ in range(100):
            if pg(c1._PSQL, b'SELECT 1;', check=False).returncode == 0: break
            time.sleep(.2)
        else: self.fail('fixture PostgreSQL unavailable')
        ctx = context(); state = card(ctx); journal = MemoryJournal()
        session = resource_claim.Session(ctx, 'b' * 12, 'b' * 64, lambda: copy.deepcopy(state), journal)
        session.admit(start=True)
        password = b'a' * 32
        secret = {'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-staging',
                               'uid': 'secret', 'resourceVersion': '2'}, 'type': 'Opaque',
                  'data': {'POSTGRES_PASSWORD': base64.b64encode(b'fixture-admin-only').decode(),
                           'REALTIME_DATABASE_URL': base64.b64encode(b'postgres://realtime:' + password +
                               b'@voice-postgres:5432/realtime_db?sslmode=disable').decode(), 'other': 'YQ=='}}
        class Kube:
            def get(self, kind, item):
                if (kind, item) != ('secret', 'voice-app-secrets'): raise AssertionError()
                return copy.deepcopy(secret)
        pg_identity = {'statefulset': {'uid': 'sts', 'resourceVersion': '1'},
                       'pod': {'uid': 'pod', 'resourceVersion': '2', 'container_id': 'ctr', 'restart_count': 0},
                       'pvc': {'name': 'pgdata-voice-postgres-0', 'uid': 'pvc', 'resourceVersion': '3'},
                       'pv': {'name': 'pv', 'uid': 'pvuid', 'resourceVersion': '4'},
                       'service': {'uid': 'svc', 'resourceVersion': '5', 'cluster_ip': '10.0.0.1'},
                       'endpoints': {'uid': 'ep', 'resourceVersion': '6', 'pod_ip': '10.0.0.2'},
                       'slice': {'uid': 'slice', 'resourceVersion': '7'}}
        def capture(args, *, body=b'', **kwargs):
            session.check(args)
            script = args[-1].replace('-h voice-postgres ', '-h 127.0.0.1 ')
            return pg(script, body).stdout
        with patch.object(commands, 'capture', side_effect=capture), \
             patch.object(c1, 'engine_snapshot', return_value=('ns', copy.deepcopy(pg_identity))):
            system = c1._system_identifier()
            c1._create_role(password, system)
            c1._create_database(system, c1._catalog()['role']['oid'])
            catalog = c1._catalog()
            original = copy.deepcopy(pg_identity); original['system_identifier'] = system
            plan = {'namespace_uid': 'ns', 'postgres': original}
            receipt = {'role_oid': catalog['role']['oid'], 'database_oid': catalog['database']['oid'],
                       'owner_oid': catalog['database']['owner_oid'],
                       'secret': {'uid': 'secret', 'resourceVersion': '1', 'data_sha256': 'a' * 64}}
            live = existing._observe(session, Kube(), plan, receipt, password)
            self.assertEqual(live['database_oid'], catalog['database']['oid'])
            self.assertEqual(live['secret']['resourceVersion'], '2')
            c1._verify_runtime_auth(password)
            pg(c1._PSQL, b'ALTER ROLE realtime SUPERUSER;\n')
            with self.assertRaisesRegex(Exception, 'realtime_existing_catalog_changed'):
                existing._observe(session, Kube(), plan, receipt, password)


if __name__ == '__main__': unittest.main()
