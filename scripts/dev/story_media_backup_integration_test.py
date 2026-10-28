"""Disposable Linux Docker test; never connects to an environment database."""
import io
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
import unittest
import uuid

import story_media_backup as backup

PG_IMAGE = 'sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'
MIGRATIONS = Path('/repo/src/backend/migrations')
VERSIONS = {'file_db': (13, 17), 'story_db': (4, 8), 'messaging_db': (27, 38)}
CATALOG = """SELECT json_build_object(
 'database_owner',(SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname=current_database()),
 'schema_owner',(SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname='public'),
 'schema_acl',(SELECT nspacl::text FROM pg_namespace WHERE nspname='public'),
 'objects',(SELECT coalesce(json_agg(json_build_object('name',c.relname,'kind',c.relkind,
 'owner',pg_get_userbyid(c.relowner),'acl',c.relacl::text) ORDER BY c.relname),'[]'::json)
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relkind IN('r','p','S','v','m')),
 'defaults',(SELECT coalesce(json_agg(json_build_object('owner',pg_get_userbyid(d.defaclrole),
 'schema',n.nspname,'type',d.defaclobjtype,'acl',d.defaclacl::text)
 ORDER BY pg_get_userbyid(d.defaclrole),n.nspname,d.defaclobjtype),'[]'::json)
 FROM pg_default_acl d LEFT JOIN pg_namespace n ON n.oid=d.defaclnamespace));"""


class PopulatedRestoreTest(unittest.TestCase):
    def docker(self, *args, input=None):
        result = subprocess.run(['docker', *args], input=input, stdout=subprocess.PIPE,
                                stderr=subprocess.DEVNULL, timeout=120)
        if result.returncode:
            raise RuntimeError('fixture docker operation failed')
        return result.stdout

    def setUp(self):
        self.owner = 'media-backup-' + uuid.uuid4().hex
        self.ids = []
        self.addCleanup(self.cleanup)
        self.assertEqual(self.docker('info', '--format', '{{.OSType}}').strip(), b'linux')
        self.password = secrets.token_hex(24)
        self.source = self.create('source_admin')
        self.target = self.create('target_admin')

    def create(self, admin):
        result = self.docker('create', '--network', 'none', '--label', 'voice.test.owner=' + self.owner,
                             '-e', 'POSTGRES_USER=' + admin, '-e', 'POSTGRES_PASSWORD=' + self.password,
                             PG_IMAGE)
        cid = result.decode().strip()
        if len(cid) != 64 or any(c not in '0123456789abcdef' for c in cid):
            raise RuntimeError('fixture creation identity unknown')
        info = json.loads(self.docker('inspect', cid))[0]
        if info['Id'] != cid or info['Config']['Labels'].get('voice.test.owner') != self.owner:
            raise RuntimeError('fixture ownership unknown')
        if info['HostConfig']['NetworkMode'] != 'none' or any(m['Type'] != 'volume' for m in info['Mounts']):
            raise RuntimeError('fixture isolation changed')
        volumes = [m['Name'] for m in info['Mounts']]
        self.ids.append((cid, volumes))
        self.docker('start', cid)
        for _ in range(120):
            result = subprocess.run(['docker', 'exec', cid, 'pg_isready', '-U', admin],
                                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if result.returncode == 0:
                return cid
            time.sleep(.25)
        raise RuntimeError('fixture postgres did not start')

    def cleanup(self):
        for cid, volumes in reversed(self.ids):
            info = json.loads(self.docker('inspect', cid))[0]
            if info['Id'] != cid or info['Config']['Labels'].get('voice.test.owner') != self.owner:
                raise RuntimeError('fixture cleanup ownership changed')
            self.docker('rm', '-f', '-v', cid)
            remaining = set(self.docker('volume', 'ls', '--format', '{{.Name}}').decode().splitlines())
            if remaining.intersection(volumes):
                raise RuntimeError('fixture volume cleanup incomplete')

    def sql(self, cid, admin, db, sql):
        return self.docker('exec', '-i', cid, 'psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1',
                           '-U', admin, '-d', db, input=sql.encode()).decode().strip()

    def test_actual_populated_restore_and_source_migrations(self):
        for db, (old, _) in VERSIONS.items():
            self.sql(self.source, 'source_admin', 'postgres', 'CREATE DATABASE ' + db)
            for path in sorted((MIGRATIONS / db).glob('*.up.sql')):
                if int(path.name[:6]) <= old:
                    self.sql(self.source, 'source_admin', db, path.read_text())
            self.sql(self.source, 'source_admin', db,
                     'CREATE TABLE schema_migrations(version bigint NOT NULL, dirty boolean NOT NULL);'
                     'INSERT INTO schema_migrations VALUES (' + str(old) + ', false);')
        self.sql(self.source, 'source_admin', 'file_db', "INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result) VALUES ('00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000002','fixture','image/png',1,'fixture-key','ready','image','clean');")
        self.sql(self.source, 'source_admin', 'story_db', "INSERT INTO stories(id,author_profile_id,type,media_file_id,visibility,expires_at,archived_until) VALUES ('00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000002','photo','00000000-0000-0000-0000-000000000001','everyone',now()+interval '1 day',now()+interval '2 days');")
        self.sql(self.source, 'source_admin', 'messaging_db', "INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,attachments) VALUES ('00000000-0000-0000-0000-000000000004','00000000-0000-0000-0000-000000000005','dm','00000000-0000-0000-0000-000000000002','fixture','[{\"file_id\":\"00000000-0000-0000-0000-000000000001\"}]');")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            key = secrets.token_bytes(32)
            binding = {'operation': self.owner, 'scope': 'synthetic-fixture-only'}
            def producer(member, output):
                command = (['pg_dumpall', '-U', 'source_admin', '--roles-only', '--no-role-passwords']
                           if member == 'roles' else ['pg_dump', '-U', 'source_admin', '-Fc', '-d', member])
                result = subprocess.run(['docker', 'exec', self.source, *command], stdout=output,
                                        stderr=subprocess.DEVNULL, timeout=90)
                if result.returncode:
                    raise RuntimeError('fixture dump failed')
            manifest = backup.export(root, key, binding, producer)
            roles = io.BytesIO()
            backup.decrypt(root, key, binding, manifest, 'roles', roles)
            self.sql(self.target, 'target_admin', 'postgres', roles.getvalue().decode())
            for db, (old, target) in VERSIONS.items():
                self.sql(self.target, 'target_admin', 'postgres', 'CREATE DATABASE ' + db + ' OWNER source_admin')
                restore = subprocess.Popen(['docker', 'exec', '-i', self.target, 'pg_restore',
                                            '-U', 'target_admin', '--exit-on-error', '-d', db],
                                           stdin=subprocess.PIPE, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                try:
                    backup.decrypt(root, key, binding, manifest, db, restore.stdin)
                    restore.stdin.close()
                    self.assertEqual(restore.wait(timeout=120), 0)
                finally:
                    if restore.poll() is None:
                        restore.kill(); restore.wait()
                self.assertEqual(self.sql(self.target, 'target_admin', db,
                                          'SELECT version || \':\' || dirty FROM schema_migrations'), str(old) + ':false')
                self.assertEqual(json.loads(self.sql(self.source, 'source_admin', db, CATALOG)),
                                 json.loads(self.sql(self.target, 'target_admin', db, CATALOG)))
                table = {'file_db': 'files', 'story_db': 'stories', 'messaging_db': 'messages'}[db]
                self.assertEqual(self.sql(self.source, 'source_admin', db, 'SELECT row_to_json(t) FROM ' + table + ' t'),
                                 self.sql(self.target, 'target_admin', db, 'SELECT row_to_json(t) FROM ' + table + ' t'))
                for path in sorted((MIGRATIONS / db).glob('*.up.sql')):
                    version = int(path.name[:6])
                    if old < version <= target:
                        self.sql(self.target, 'target_admin', db, path.read_text())
                self.sql(self.target, 'target_admin', db, 'UPDATE schema_migrations SET version=' + str(target))
            for db, table in [('file_db', 'files'), ('story_db', 'stories'), ('messaging_db', 'messages')]:
                self.assertEqual(self.sql(self.target, 'target_admin', db, 'SELECT count(*) FROM ' + table), '1')


if __name__ == '__main__':
    unittest.main()
