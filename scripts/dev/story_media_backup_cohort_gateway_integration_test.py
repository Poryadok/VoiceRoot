"""Populated gateway logical backup and isolated restore; no live authority."""
import hashlib
import json
from pathlib import Path
import secrets
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as restore
import story_media_backup_restore as media
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_integration_test import cleanup_fixture
from story_media_backup_restore_integration_test import Journal, POSTGRES, MIGRATE


class GatewayRestoreTest(unittest.TestCase):
    def test_populated_gateway_without_ledger_restores_exact_catalog_and_row(self):
        fixture = fixtures.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture); fixture.setUp()
        fixture.sql(fixture.source, 'source_admin', 'postgres', 'CREATE DATABASE gateway_db')
        folder = Path('/repo/src/backend/migrations/gateway_db')
        for path in sorted(folder.glob('*.up.sql')):
            fixture.sql(fixture.source, 'source_admin', 'gateway_db', path.read_text())
        fixture.sql(fixture.source, 'source_admin', 'gateway_db',
                    "UPDATE client_versions SET latest_version='9.8.7' WHERE platform='windows'")
        catalog = {'gateway_db': json.loads(fixture.sql(fixture.source, 'source_admin', 'gateway_db', restore.GATEWAY_CATALOG))}
        self.assertEqual(catalog['gateway_db']['history'], {'schema_migrations': False, 'flyway_schema_history': False})
        roles = json.loads(fixture.sql(fixture.source, 'source_admin', 'postgres', media.role_catalog_sql()))
        plans = {'gateway_db': {'engine': 'unmanaged', 'from': None, 'to': None,
                 'files': {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in folder.glob('*.sql')}}}
        metadata = {'catalog': catalog, 'roles': roles, 'migrations': plans}
        def digest(value):
            return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode()).hexdigest()
        source = {'engine': {'fixture': 'same'}, 'server_start': 'fixture'}
        binding = {'operation_id': 'media-c2-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
            'source_sha': 'ec214b28747a4552eb89f4e431fc018aa82f023a', 'databases': ['gateway_db'],
            'sources': {'gateway_db': {**source, 'database_oid': 1}, 'roles': source},
            'dump_receipts': {'gateway_db': 'd' * 64, 'roles': 'e' * 64},
            'restore_inputs': {'catalog_sha256': digest(catalog), 'roles_catalog_sha256': digest(roles),
                'migration_catalog_sha256': digest(plans), 'postgres_image': POSTGRES, 'migrate_image': MIGRATE,
                'migrate_binary_sha256': 'f' * 64, 'postgres_major': 16}}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); key = secrets.token_bytes(32)
            def producer(member, output):
                cmd = (['pg_dumpall', '-U', 'source_admin', '--roles-only', '--no-role-passwords'] if member == 'roles'
                       else ['pg_dump', '-U', 'source_admin', '-Fc', '-d', member])
                result = subprocess.run(['docker', 'exec', fixture.source, *cmd], stdout=output,
                                        stderr=subprocess.DEVNULL, timeout=90)
                if result.returncode: raise RuntimeError('fixture dump failed')
            manifest = cohort.export(root, key, binding, ['gateway_db'], producer)
            original = media._psql; observations = []
            def observe(cid, admin, db, sql, timeout=120):
                result = original(cid, admin, db, sql, timeout=timeout)
                if sql == restore.GATEWAY_CATALOG:
                    self.assertEqual(original(cid, admin, db, "SELECT latest_version FROM client_versions WHERE platform='windows'").strip(), b'9.8.7')
                    observations.append(db)
                return result
            journal = Journal()
            with patch.object(media, '_psql', side_effect=observe), patch.object(media, '_tool_bytes', side_effect=AssertionError('unexpected migration')):
                receipt = restore.qualify(root, key, binding, ['gateway_db'], manifest, metadata, Path('/repo'), POSTGRES, MIGRATE, journal)
            self.assertEqual(receipt['databases']['gateway_db'], {'mode': 'restore-only', 'from': None, 'to': None})
            self.assertEqual(receipt['cleanup'], 'verified'); self.assertEqual(observations, ['gateway_db', 'gateway_db'])
            self.assertEqual([event[1] for event in journal.events if event[0] == 'created'], ['postgres'])


if __name__ == '__main__': unittest.main()
