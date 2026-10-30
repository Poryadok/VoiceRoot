"""Synthetic GitHub transport with actual isolated PostgreSQL restore/migration."""
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
from pathlib import Path
import secrets
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import story_media_backup as backup
import story_media_backup_custody as custody
import story_media_backup_integration_test as fixture_module


def cleanup_fixture(fixture):
    if not fixture.doCleanups():
        raise RuntimeError('fixture cleanup failed')


class CustodyRestoreTest(unittest.TestCase):
    def test_fake_github_readback_then_real_populated_restore_and_migrations(self):
        fixture = fixture_module.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture)
        fixture.setUp()
        for db, (old, _) in fixture_module.VERSIONS.items():
            fixture.sql(fixture.source, 'source_admin', 'postgres', 'CREATE DATABASE ' + db)
            for path in sorted((fixture_module.MIGRATIONS / db).glob('*.up.sql')):
                if int(path.name[:6]) <= old:
                    fixture.sql(fixture.source, 'source_admin', db, path.read_text())
            fixture.sql(fixture.source, 'source_admin', db,
                'CREATE TABLE schema_migrations(version bigint NOT NULL, dirty boolean NOT NULL);'
                'INSERT INTO schema_migrations VALUES (' + str(old) + ', false);')
        fixture.sql(fixture.source, 'source_admin', 'file_db', "INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result) VALUES ('00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000002','fixture','image/png',1,'fixture-key','ready','image','clean');")
        fixture.sql(fixture.source, 'source_admin', 'story_db', "INSERT INTO stories(id,author_profile_id,type,media_file_id,visibility,expires_at,archived_until) VALUES ('00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000002','photo','00000000-0000-0000-0000-000000000001','everyone',now()+interval '1 day',now()+interval '2 days');")
        fixture.sql(fixture.source, 'source_admin', 'messaging_db', "INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,attachments) VALUES ('00000000-0000-0000-0000-000000000004','00000000-0000-0000-0000-000000000005','dm','00000000-0000-0000-0000-000000000002','fixture','[{\"file_id\":\"00000000-0000-0000-0000-000000000001\"}]');")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'; root.mkdir(mode=0o700)
            target = Path(directory) / 'readback'; target.mkdir(mode=0o700)
            key = secrets.token_bytes(32)
            operation = 'media-c1-' + 'a' * 32
            snapshot = 'capture-' + 'b' * 32
            binding = {'operation_id': operation, 'snapshot_id': snapshot, 'source_sha': 'c' * 40}
            def producer(member, output):
                command = (['pg_dumpall', '-U', 'source_admin', '--roles-only', '--no-role-passwords']
                           if member == 'roles' else ['pg_dump', '-U', 'source_admin', '-Fc', '-d', member])
                result = subprocess.run(['docker', 'exec', fixture.source, *command], stdout=output,
                                        stderr=subprocess.DEVNULL, timeout=90)
                if result.returncode: raise RuntimeError('fixture dump failed')
            manifest = backup.export(root, key, binding, producer)
            bundle = custody.build_bundle(root, key, binding, manifest)
            raw = bundle.read_bytes()
            archive_bytes = io.BytesIO()
            with zipfile.ZipFile(archive_bytes, 'w', compression=zipfile.ZIP_STORED) as archive:
                archive.writestr('media-backup.tar', raw)
            wire = archive_bytes.getvalue()
            now = datetime.now(timezone.utc)
            artifact = {'operation_id': operation, 'snapshot_id': snapshot, 'challenge': 'd' * 32,
                        'run_id': 42, 'head_sha': 'e' * 40, 'created_at': (now-timedelta(seconds=1)).isoformat(),
                        'bundle_bytes': len(raw), 'bundle_sha256': hashlib.sha256(raw).hexdigest()}
            metadata = {'id': 23, 'url': custody.API + '23', 'name': 'voice-media-backup-' + operation + '-' + 'd' * 32,
                        'expired': False, 'created_at': now.isoformat(), 'expires_at': (now+timedelta(days=1)).isoformat(),
                        'workflow_run': {'id': 42, 'head_sha': 'e' * 40}, 'size_in_bytes': len(wire)}
            def download(url, headers, output, limit, deadline, redirects=False):
                value = wire if redirects else json.dumps(metadata).encode()
                output.write(value)
                return len(value)
            with patch.object(custody.transport, '_download', side_effect=download):
                _, restored_manifest = custody.readback_extract('fixture-token', artifact, 23, bundle, key, binding, target)
            roles = io.BytesIO()
            backup.decrypt(target, key, binding, restored_manifest, 'roles', roles)
            fixture.sql(fixture.target, 'target_admin', 'postgres', roles.getvalue().decode())
            for db, (old, final) in fixture_module.VERSIONS.items():
                fixture.sql(fixture.target, 'target_admin', 'postgres', 'CREATE DATABASE ' + db + ' OWNER source_admin')
                process = subprocess.Popen(['docker', 'exec', '-i', fixture.target, 'pg_restore',
                    '-U', 'target_admin', '--exit-on-error', '-d', db], stdin=subprocess.PIPE,
                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                try:
                    backup.decrypt(target, key, binding, restored_manifest, db, process.stdin)
                    process.stdin.close()
                    self.assertEqual(process.wait(timeout=120), 0)
                finally:
                    if process.poll() is None: process.kill(); process.wait()
                self.assertEqual(fixture.sql(fixture.target, 'target_admin', db,
                    'SELECT version || \':\' || dirty FROM schema_migrations'), str(old) + ':false')
                self.assertEqual(json.loads(fixture.sql(fixture.source, 'source_admin', db, fixture_module.CATALOG)),
                                 json.loads(fixture.sql(fixture.target, 'target_admin', db, fixture_module.CATALOG)))
                table = {'file_db': 'files', 'story_db': 'stories', 'messaging_db': 'messages'}[db]
                self.assertEqual(fixture.sql(fixture.source, 'source_admin', db, 'SELECT row_to_json(t) FROM ' + table + ' t'),
                                 fixture.sql(fixture.target, 'target_admin', db, 'SELECT row_to_json(t) FROM ' + table + ' t'))
                for path in sorted((fixture_module.MIGRATIONS / db).glob('*.up.sql')):
                    if old < int(path.name[:6]) <= final:
                        fixture.sql(fixture.target, 'target_admin', db, path.read_text())
                self.assertEqual(fixture.sql(fixture.target, 'target_admin', db, 'SELECT count(*) FROM ' + table), '1')


if __name__ == '__main__': unittest.main()
