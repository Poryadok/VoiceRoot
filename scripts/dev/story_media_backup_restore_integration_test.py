"""Actual isolated PostgreSQL restore with the pinned golang-migrate tool."""
import hashlib
import io
import json
from pathlib import Path
import secrets
import subprocess
import tempfile
import tarfile
import unittest
from unittest.mock import patch

import story_media_backup as backup
import story_media_backup_integration_test as fixture_module
import story_media_backup_restore as restore

POSTGRES = 'sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'
MIGRATE = 'sha256:9ad5c7b72b43da1857595f48be00e45ed26115103cb4281b5eb96be4a0ce2e8e'


class Journal:
    def __init__(self): self.events = []
    def begin(self, kind, name, labels): self.events.append(('begin', kind)); return secrets.token_hex(16)
    def created(self, kind, nonce, cid, labels): self.events.append(('created', kind, cid))
    def begin_remove(self, kind, cid): self.events.append(('remove', kind, cid))
    def removed(self, kind, cid, absent):
        if absent is not True: raise RuntimeError('not absent')
        self.events.append(('removed', kind, cid))
    def uncertain(self, kind, nonce): self.events.append(('uncertain', kind))


def cleanup_fixture(fixture):
    if not fixture.doCleanups(): raise RuntimeError('fixture cleanup failed')


class ActualRestoreTest(unittest.TestCase):
    def test_populated_restore_with_golang_migrate(self):
        fixture = fixture_module.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture)
        fixture.setUp()
        plans = {}
        for db, (old, final) in fixture_module.VERSIONS.items():
            fixture.sql(fixture.source, 'source_admin', 'postgres', 'CREATE DATABASE ' + db)
            files = {}
            for path in sorted((fixture_module.MIGRATIONS / db).glob('*.up.sql')):
                version = int(path.name[:6])
                if version <= old:
                    fixture.sql(fixture.source, 'source_admin', db, path.read_text())
            for path in sorted((fixture_module.MIGRATIONS / db).glob('*.sql')):
                version = int(path.name[:6])
                if old <= version <= final:
                    files[path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
            plans[db] = {'from': old, 'to': final, 'files': files}
            fixture.sql(fixture.source, 'source_admin', db,
                'CREATE TABLE schema_migrations(version bigint NOT NULL, dirty boolean NOT NULL);'
                'INSERT INTO schema_migrations VALUES (' + str(old) + ', false);')
        fixture.sql(fixture.source, 'source_admin', 'file_db', "INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result) VALUES ('00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000002','fixture','image/png',1,'fixture-key','ready','image','clean');")
        fixture.sql(fixture.source, 'source_admin', 'story_db', "INSERT INTO stories(id,author_profile_id,type,media_file_id,visibility,expires_at,archived_until) VALUES ('00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000002','photo','00000000-0000-0000-0000-000000000001','everyone',now()+interval '1 day',now()+interval '2 days');")
        fixture.sql(fixture.source, 'source_admin', 'messaging_db', "INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content) VALUES ('00000000-0000-0000-0000-000000000004','00000000-0000-0000-0000-000000000005','dm','00000000-0000-0000-0000-000000000002','fixture');")
        catalog = {db: json.loads(fixture.sql(fixture.source, 'source_admin', db, restore.CATALOG))
                   for db in fixture_module.VERSIONS}
        roles = json.loads(fixture.sql(fixture.source, 'source_admin', 'postgres', restore.role_catalog_sql()))
        tool_journal = Journal(); resources = []
        try:
            tool_cid = restore._create(tool_journal, resources, 'migrate', MIGRATE, ['--entrypoint', '/bin/true'])
            raw = restore._docker('cp', tool_cid + ':/usr/local/bin/migrate', '-')
            with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
                items = archive.getmembers()
                self.assertEqual(len(items), 1)
                self.assertEqual(items[0].name, 'migrate')
                self.assertTrue(items[0].isfile())
                tool_sha = hashlib.sha256(archive.extractfile(items[0]).read()).hexdigest()
        finally:
            restore._cleanup(tool_journal, resources)
        self.assertEqual(sum(event[0] == 'removed' for event in tool_journal.events), 1)
        metadata = {'catalog': catalog, 'roles': roles, 'migrations': plans}
        def digest(value):
            return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('ascii')).hexdigest()
        binding = {'operation_id': 'media-c1-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
                   'source_sha': 'ec214b28747a4552eb89f4e431fc018aa82f023a',
                   'dump_receipts': {m: 'd' * 64 for m in backup.MEMBERS},
                   'restore_inputs': {'postgres_major': 16, 'postgres_image': POSTGRES, 'migrate_image': MIGRATE,
                       'migrate_binary_sha256': tool_sha, 'catalog_sha256': digest(catalog),
                       'roles_catalog_sha256': digest(roles), 'migration_catalog_sha256': digest(plans)}}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); key = secrets.token_bytes(32)
            def producer(member, output):
                command = (['pg_dumpall', '-U', 'source_admin', '--roles-only', '--no-role-passwords']
                           if member == 'roles' else ['pg_dump', '-U', 'source_admin', '-Fc', '-d', member])
                result = subprocess.run(['docker', 'exec', fixture.source, *command], stdout=output,
                                        stderr=subprocess.DEVNULL, timeout=90)
                if result.returncode: raise RuntimeError('fixture dump failed')
            manifest = backup.export(root, key, binding, producer)
            original_psql = restore._psql
            verified = []
            def observe(cid, admin, database, sql, timeout=120):
                result = original_psql(cid, admin, database, sql, timeout=timeout)
                if sql.startswith("SELECT json_agg(json_build_object('version'"):
                    table = {'file_db': 'files', 'story_db': 'stories', 'messaging_db': 'messages'}[database]
                    self.assertEqual(original_psql(cid, admin, database, 'SELECT count(*) FROM ' + table).strip(), b'1')
                    verified.append(database)
                return result
            journal = Journal()
            with patch.object(restore, '_psql', side_effect=observe):
                receipt = restore.qualify_restored_set(root, key, binding, manifest, metadata, fixture_module.MIGRATIONS,
                                                       POSTGRES, MIGRATE, journal)
            self.assertEqual(receipt['versions'], {'file_db': 17, 'story_db': 8, 'messaging_db': 38})
            self.assertEqual(receipt['manifest_sha256'], hashlib.sha256(json.dumps(manifest, sort_keys=True,
                             separators=(',', ':'), ensure_ascii=True).encode('ascii')).hexdigest())
            self.assertEqual(receipt['cleanup'], 'verified')
            self.assertEqual(set(verified), set(fixture_module.VERSIONS))
            self.assertEqual(sum(event[0] == 'removed' for event in journal.events), 2)
            self.assertNotIn('uncertain', [event[0] for event in journal.events])
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); key = secrets.token_bytes(32)
            def bad_producer(member, output):
                if member == 'file_db':
                    output.write(b'not-a-postgres-archive-private-fixture')
                else:
                    producer(member, output)
            manifest = backup.export(root, key, binding, bad_producer)
            failed_journal = Journal()
            with self.assertRaises(restore.RestoreError) as error:
                restore.qualify_restored_set(root, key, binding, manifest, metadata, fixture_module.MIGRATIONS,
                                             POSTGRES, MIGRATE, failed_journal)
            self.assertNotIn('private-fixture', str(error.exception))
            self.assertEqual(sum(event[0] == 'removed' for event in failed_journal.events), 2)
            self.assertNotIn('uncertain', [event[0] for event in failed_journal.events])


if __name__ == '__main__': unittest.main()
