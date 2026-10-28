"""Synthetic populated 26→32 rehearsal with real pinned PostgreSQL and Flyway."""
import hashlib
import hmac
import io
import json
import os
import pathlib
import shutil
import subprocess
import tarfile
import tempfile
import time
import unittest

from auth_user_adoption import Journal
from auth_user_adoption_runtime import QUALIFIED_TOOLS, backup_catalog_queries, digest, operator_fingerprint, source_config
from auth_flyway_upgrade_qualification import qualify, source_catalog
from auth_flyway_upgrade_qualification_docker import OwnedDocker

ROOT = pathlib.Path(__file__).resolve().parents[2]


@unittest.skipUnless(os.environ.get('RUN_DOCKER_INTEGRATION') == '1', 'explicit Linux Docker integration')
class RealFlywayQualification(unittest.TestCase):
    def test_populated_actual_source_and_tamper(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = pathlib.Path(temporary)
            root = base / 'candidate'
            root.mkdir()
            for path in ('scripts/dev/auth_user_adoption.py', 'scripts/dev/auth_user_adoption_runtime.py',
                         'scripts/dev/auth_user_adoption_backup.py', 'scripts/dev/auth_user_adoption_smoke.py',
                         'scripts/dev/auth_user_adoption_restore.py', 'docker/postgres/adopt-auth-user-runtime-roles.sql',
                         'docker/postgres/provision-auth-user-runtime-roles.sql',
                         'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql',
                         'src/backend/auth/src/main/resources/db/migration', 'src/backend/migrations/user_db'):
                src, dst = ROOT / path, root / path
                dst.parent.mkdir(parents=True, exist_ok=True)
                shutil.copytree(src, dst) if src.is_dir() else shutil.copy2(src, dst)
            for path in root.rglob('*'):
                if path.is_file():
                    path.chmod(0o644)
            def git(*args):
                return subprocess.run(['git', '-C', str(root), *args], check=True, capture_output=True).stdout.decode().strip()
            git('init', '-q'); git('config', 'user.name', 'fixture'); git('config', 'user.email', 'fixture@example.invalid')
            git('add', '.'); git('commit', '-qm', 'real source fixture')
            source = git('rev-parse', 'HEAD'); operation = 'fixture'
            admission = {'source_sha': source, 'target': 32, 'flyway_enabled': False, 'selector_mode': 'absent', 'persistence': 'jdbc',
                         'plan_sha256': 'a' * 64, 'operator_sha256': operator_fingerprint(root),
                         'sources_sha256': digest(source_config(root, operation))}
            tools_image = subprocess.run(['docker', 'image', 'inspect', 'python:3.12-alpine', '--format', '{{.Id}}'],
                                         check=True, capture_output=True).stdout.decode().strip()
            # The separate tools image must include openssl; its local immutable ID is bound in the report.
            tools_image = os.environ['QUALIFICATION_TOOLS_IMAGE']
            password = 'fixture-password'; fixture_journal = Journal(base / 'fixture-journal', {'operation': operation})
            owned = OwnedDocker(fixture_journal); call = owned.call
            try:
                pg = owned.create(QUALIFIED_TOOLS['postgres'], ['--network', 'none', '-e', 'POSTGRES_USER=voice',
                                  '-e', 'POSTGRES_PASSWORD=' + password], [])
                call('start', pg)
                def psql(db, sql):
                    return call('exec', '-i', '-e', 'PGPASSWORD=' + password, pg, 'psql', '-h', '127.0.0.1',
                                '-U', 'voice', '-d', db, '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', data=sql.encode())
                for _ in range(100):
                    try:
                        psql('postgres', 'SELECT 1'); break
                    except ValueError:
                        time.sleep(.2)
                else:
                    self.fail('fixture database did not start')
                psql('postgres', 'CREATE DATABASE auth_db OWNER voice; CREATE DATABASE user_db OWNER voice;')
                migrations, _ = source_catalog(root, source)
                archive = io.BytesIO()
                with tarfile.open(fileobj=archive, mode='w') as tar:
                    for name, raw in sorted(migrations.items()):
                        entry = tarfile.TarInfo('sql/' + name); entry.mode = 0o644; entry.size = len(raw)
                        tar.addfile(entry, io.BytesIO(raw))
                fly = owned.create(QUALIFIED_TOOLS['flyway'], ['--network', 'container:' + pg,
                    '-e', 'FLYWAY_USER=voice', '-e', 'FLYWAY_PASSWORD=' + password,
                    '-e', 'FLYWAY_URL=jdbc:postgresql://127.0.0.1:5432/auth_db'],
                    ['-locations=filesystem:/flyway/sql', '-target=26', 'migrate'])
                call('cp', '-', fly + ':/flyway', data=archive.getvalue()); call('start', '-a', fly)
                psql('auth_db', "INSERT INTO accounts(id,password_hash,type,status,deleted_at) VALUES "
                    "('00000000-0000-0000-0000-000000000001','x','guest','active',NULL),"
                    "('00000000-0000-0000-0000-000000000002','x','guest','active','2020-01-01');")
                psql('user_db', 'CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(21,false);')
                catalog = {db: json.loads(psql(db, sql)) for db, sql in backup_catalog_queries().items()}
                encrypted = base / 'backup'; encrypted.mkdir(mode=0o700)
                key = '12' * 32; (encrypted / 'backup.key').write_text(key); os.chmod(encrypted / 'backup.key', 0o600)
                blobs = {'roles.enc': call('exec', '-e', 'PGPASSWORD=' + password, pg, 'pg_dumpall', '-h', '127.0.0.1', '-U', 'voice', '--roles-only', '--no-role-passwords')}
                for db in ('auth_db', 'user_db'):
                    blobs[db + '.enc'] = call('exec', '-e', 'PGPASSWORD=' + password, pg, 'pg_dump', '-h', '127.0.0.1', '-U', 'voice', '-Fc', db)
                files = {}
                for name, raw in blobs.items():
                    env = dict(os.environ, FIXTURE_KEY=key)
                    ciphertext = subprocess.run(['openssl', 'enc', '-aes-256-cbc', '-pbkdf2', '-iter', '200000',
                                                 '-salt', '-pass', 'env:FIXTURE_KEY'], input=raw, env=env,
                                                check=True, capture_output=True).stdout
                    path = encrypted / name; path.write_bytes(ciphertext); os.chmod(path, 0o600)
                    files[name] = {'sha256': hashlib.sha256(ciphertext).hexdigest(),
                                   'hmac': hmac.new(bytes.fromhex(key), ciphertext, hashlib.sha256).hexdigest()}
                binding = {key: admission[key] for key in ('source_sha', 'operator_sha256', 'sources_sha256')}
                manifest = {'format': 1, 'operation': operation, 'binding': binding, 'files': files, 'catalog': catalog}
                report = {'protocol': 'voice-auth-user-backup-v1', 'operation': operation, 'result': 'PASS',
                    'files': files, 'binding': binding, 'restored_catalog': catalog, 'off_node': True,
                    'restore_exit': {'auth_db': 0, 'user_db': 0, 'roles': 0}, 'postgres_major': 16,
                    'verifier_sha256': 'b' * 64, 'tools_image': tools_image, 'fixture_cleanup': True,
                    'scope': 'actual-existing-dev-backup'}
                journal = Journal(base / 'qualifier-journal', {'operation': operation})
                try:
                    original_save = journal.save
                    def mutate_caller_and_save():
                        admission['plan_sha256'] = 'f' * 64
                        manifest['binding']['source_sha'] = '0' * 40
                        report['scope'] = 'changed-after-call'
                        (root / 'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql').write_text('changed')
                        (root / 'src/backend/auth/src/main/resources/db/migration/V31__auth_identity_ddl_barrier.sql').write_text('changed')
                        original_save()
                    journal.save = mutate_caller_and_save
                    receipt = qualify(root, admission, encrypted, manifest, report, operation, tools_image, journal)
                    self.assertEqual([x['version'] for x in receipt['after_history'][-3:]], ['28', '31', '32'])
                    self.assertEqual(receipt['expected_after_markers'], receipt['after_markers'])
                    self.assertNotEqual(receipt['before_markers']['accounts'], receipt['after_markers']['accounts'])
                    self.assertNotEqual(receipt['before_markers']['auth_authority_revision'], receipt['after_markers']['auth_authority_revision'])
                    self.assertEqual(receipt['cleanup'], 'confirmed')
                    self.assertEqual(receipt['plan_sha256'], 'a' * 64)
                    self.assertEqual(receipt['backup_binding']['source_sha'], source)
                finally:
                    journal.close()
            finally:
                owned.cleanup(); fixture_journal.close()


if __name__ == '__main__':
    unittest.main()
