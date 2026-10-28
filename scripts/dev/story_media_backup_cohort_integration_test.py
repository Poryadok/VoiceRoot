"""Synthetic selected cohort: actual no-op restores without migration runner."""
import hashlib
import json
from pathlib import Path
import secrets
import subprocess
import tarfile
import tempfile
import unittest
from contextlib import nullcontext
import io
from unittest.mock import patch

import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as restore
import story_media_backup_restore as media_restore
import story_media_backup_integration_test as fixtures
from story_media_backup_restore_integration_test import Journal, POSTGRES, MIGRATE, cleanup_fixture


class CohortNoopTest(unittest.TestCase):
    def test_populated_auth_and_voice_restore_without_migration_execution(self):
        self.exercise(15, 15)

    def test_populated_changed_voice_and_unchanged_auth(self):
        self.exercise(14, 15)

    def exercise(self, voice_from, voice_to):
        fixture = fixtures.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture); fixture.setUp()
        databases = ['auth_db', 'voice_db']
        for db in databases:
            fixture.sql(fixture.source, 'source_admin', 'postgres', 'CREATE DATABASE ' + db)
        fixture.sql(fixture.source, 'source_admin', 'auth_db',
            "CREATE TABLE flyway_schema_history(installed_rank int PRIMARY KEY, version varchar(50), script varchar(1000) NOT NULL, checksum int, success bool NOT NULL);"
            "INSERT INTO flyway_schema_history VALUES (1,'32','V32__linked_oauth_transport.sql',123,true);")
        voice_folder = Path('/repo/src/backend/migrations/voice_db')
        for path in sorted(voice_folder.glob('*.up.sql')):
            if int(path.name[:6]) <= voice_from:
                fixture.sql(fixture.source, 'source_admin', 'voice_db', path.read_text())
        fixture.sql(fixture.source, 'source_admin', 'voice_db',
                    'CREATE TABLE schema_migrations(version bigint NOT NULL, dirty boolean NOT NULL);'
                    'INSERT INTO schema_migrations VALUES (' + str(voice_from) + ',false);')
        for db in databases:
            fixture.sql(fixture.source, 'source_admin', db, 'CREATE TABLE fixture_marker(id int PRIMARY KEY); INSERT INTO fixture_marker VALUES (1);')
        auth_folder = Path('/repo/src/backend/auth/src/main/resources/db/migration')
        plans = {}
        for db, folder, engine, current, target in [('auth_db', auth_folder, 'flyway', 32, 32),
                                                     ('voice_db', voice_folder, 'golang-migrate', voice_from, voice_to)]:
            files = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in folder.glob('*.sql')}
            plans[db] = {'engine': engine, 'from': current, 'to': target, 'files': files}
        catalog = {db: json.loads(fixture.sql(fixture.source, 'source_admin', db,
                     restore.AUTH_CATALOG if db == 'auth_db' else media_restore.CATALOG)) for db in databases}
        roles = json.loads(fixture.sql(fixture.source, 'source_admin', 'postgres', media_restore.role_catalog_sql()))
        metadata = {'catalog': catalog, 'roles': roles, 'migrations': plans}
        def digest(value):
            return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('ascii')).hexdigest()
        tool_sha = 'e' * 64
        if voice_from != voice_to:
            tool_journal = Journal(); resources = []
            try:
                cid = media_restore._create(tool_journal, resources, 'migrate', MIGRATE, ['--entrypoint', '/bin/true'])
                raw = media_restore._docker('cp', cid + ':/usr/local/bin/migrate', '-')
                with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
                    items = archive.getmembers(); self.assertEqual(len(items), 1)
                    self.assertEqual(items[0].name, 'migrate'); self.assertTrue(items[0].isfile())
                    tool_sha = hashlib.sha256(archive.extractfile(items[0]).read()).hexdigest()
            finally:
                media_restore._cleanup(tool_journal, resources)
            self.assertEqual(sum(event[0] == 'removed' for event in tool_journal.events), 1)
        binding = {'operation_id': 'media-c1-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
            'source_sha': 'ec214b28747a4552eb89f4e431fc018aa82f023a', 'databases': databases,
            'sources': {'auth_db': {'engine': {'fixture': 'same'}, 'server_start': 'fixture', 'database_oid': 1},
                        'voice_db': {'engine': {'fixture': 'same'}, 'server_start': 'fixture', 'database_oid': 2},
                        'roles': {'engine': {'fixture': 'same'}, 'server_start': 'fixture'}},
            'dump_receipts': {m: 'd' * 64 for m in (*databases, 'roles')},
            'restore_inputs': {'catalog_sha256': digest(catalog), 'roles_catalog_sha256': digest(roles),
                'migration_catalog_sha256': digest(plans), 'postgres_image': POSTGRES, 'migrate_image': MIGRATE,
                'migrate_binary_sha256': tool_sha, 'postgres_major': 16}}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); key = secrets.token_bytes(32)
            def producer(member, output):
                command = (['pg_dumpall', '-U', 'source_admin', '--roles-only', '--no-role-passwords']
                           if member == 'roles' else ['pg_dump', '-U', 'source_admin', '-Fc', '-d', member])
                result = subprocess.run(['docker', 'exec', fixture.source, *command], stdout=output,
                                        stderr=subprocess.DEVNULL, timeout=90)
                if result.returncode: raise RuntimeError('fixture dump failed')
            manifest = cohort.export(root, key, binding, databases, producer)
            original_psql = media_restore._psql; observed = []
            def observe(cid, admin, db, sql, timeout=120):
                result = original_psql(cid, admin, db, sql, timeout=timeout)
                if sql in (restore.AUTH_CATALOG, media_restore.CATALOG):
                    self.assertEqual(original_psql(cid, admin, db, 'SELECT count(*) FROM fixture_marker').strip(), b'1')
                    observed.append(db)
                return result
            journal = Journal()
            no_tool = (patch.object(media_restore, '_tool_bytes', side_effect=AssertionError('no-op must not start migrator'))
                       if voice_from == voice_to else nullcontext())
            with patch.object(media_restore, '_psql', side_effect=observe), no_tool:
                receipt = restore.qualify(root, key, binding, databases, manifest, metadata, Path('/repo'),
                                          POSTGRES, MIGRATE, journal)
            self.assertEqual(receipt['databases'], {
                'auth_db': {'mode': 'restore-only', 'from': 32, 'to': 32},
                'voice_db': {'mode': 'restore-only' if voice_from == voice_to else 'migrate',
                             'from': voice_from, 'to': voice_to}})
            self.assertEqual(observed.count('auth_db'), 2)
            self.assertEqual(observed.count('voice_db'), 2)
            self.assertEqual(receipt['cleanup'], 'verified')
            self.assertEqual(receipt['snapshot_id'], binding['snapshot_id'])
            self.assertEqual([event[1] for event in journal.events if event[0] == 'created'],
                             ['postgres'] if voice_from == voice_to else ['postgres', 'migrate'])
            self.assertNotIn('uncertain', [event[0] for event in journal.events])


if __name__ == '__main__': unittest.main()
