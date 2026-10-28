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
from auth_flyway_upgrade_qualification_cohort import qualify_cohort
import story_media_backup_cohort as cohort
import story_media_backup_cohort_finalize as finalize
import story_media_backup_cohort_restore as cohort_restore
import story_media_backup_restore as media_restore
from story_media_backup_restore_integration_test import Journal as FixtureRestoreJournal

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
                # The real cohort transport uses raw key bytes and a distinct
                # authenticated manifest. This receipt is synthetic, not DEV evidence.
                for name, raw in migrations.items():
                    (root / 'src/backend/auth/src/main/resources/db/migration' / name).write_bytes(raw)
                gate = 'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql'
                (root / gate).write_bytes(subprocess.run(['git','-C',str(root),'show',source+':'+gate],
                                                       check=True,capture_output=True).stdout)
                cohort_dir = base / 'cohort'; cohort_dir.mkdir(mode=0o700)
                cohort_key = bytes(range(32)); dbs=['auth_db']
                cohort_catalog = {'auth_db':json.loads(psql('auth_db',cohort_restore.AUTH_CATALOG))}
                cohort_roles = json.loads(psql('postgres',media_restore.role_catalog_sql()))
                hashes = {name:hashlib.sha256(raw).hexdigest() for name,raw in migrations.items()}
                plans = {'auth_db':{'engine':'flyway','from':26,'to':26,'files':hashes}}
                metadata={'catalog':cohort_catalog,'roles':cohort_roles,'migrations':plans}
                engine={'namespace_uid':'33333333-3333-3333-3333-333333333333',
                    'pod_uid':'44444444-4444-4444-4444-444444444444','pvc_uid':'55555555-5555-5555-5555-555555555555',
                    'pv_uid':'66666666-6666-6666-6666-666666666666','container_id':'containerd://'+'d'*64,'restart_count':7}
                operation='media-c1-'+'a'*32
                binding={'operation_id':operation,'snapshot_id':'capture-'+'b'*32,'source_sha':source,
                    'project':'voice','resource':'DEV','actor':'scheduled_devops',
                    'claim':'11111111-1111-1111-1111-111111111111','window':'22222222-2222-2222-2222-222222222222',
                    'generation':3,'revision':4,'databases':dbs,
                    'sources':{'auth_db':{'engine':engine,'server_start':'fixture','database_oid':1},
                               'roles':{'engine':engine,'server_start':'fixture'}},
                    'restore_inputs':{'catalog_sha256':digest(cohort_catalog),'roles_catalog_sha256':digest(cohort_roles),
                        'migration_catalog_sha256':digest(plans),'postgres_image':QUALIFIED_TOOLS['postgres'],
                        'migrate_image':QUALIFIED_TOOLS['migrate'],'migrate_binary_sha256':'6'*64,'postgres_major':16}}
                manifest = cohort.export(cohort_dir,cohort_key,binding,dbs,
                    lambda member,out:out.write(blobs[member+'.enc']))
                dump_receipts={}
                for index,member in enumerate(('auth_db','roles')):
                    dump_receipts[member]={'operation_id':operation,'snapshot_id':binding['snapshot_id'],'member':member,
                        'pod_uid':engine['pod_uid'],'pvc_uid':engine['pvc_uid'],'pv_uid':engine['pv_uid'],
                        'container_id':engine['container_id'],'restart_count':7,'server_start':'fixture',
                        'database_oid':binding['sources'][member].get('database_oid'),
                        'attempt_nonce':f'{index+1:032x}','exit':0,'scope':'dump-terminal-only'}
                binding,manifest=finalize.finalize(cohort_dir,cohort_key,binding,dbs,manifest,dump_receipts)
                restore_receipt=cohort_restore.qualify(cohort_dir,cohort_key,binding,dbs,manifest,metadata,root,
                    QUALIFIED_TOOLS['postgres'],QUALIFIED_TOOLS['migrate'],FixtureRestoreJournal())
                custody={'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot',
                    'artifact_id':23,'operation_id':operation,'snapshot_id':binding['snapshot_id'],
                    'challenge':'f'*32,'run_id':42,'head_sha':source,'bundle_bytes':100,'bundle_sha256':'f'*64}
                admitted={'source_sha':source,'target':32,'flyway_enabled':False,'selector_mode':'absent',
                    'persistence':'jdbc','plan_sha256':'a'*64,'custody_sha256':digest(custody),
                    'restore_sha256':digest(restore_receipt)}
                class SyntheticJournal:
                    def __init__(self): self.state={}; self.stages=[]
                    def save(self): pass
                    def guard(self): pass
                    def stage(self,name,status): self.stages.append((name,status))
                    def uncertain(self): raise RuntimeError('fixture outcome uncertain')
                cohort_journal=SyntheticJournal()
                result=qualify_cohort(root,cohort_dir,cohort_key,binding,dbs,manifest,metadata,custody,
                                      restore_receipt,admitted,cohort_journal)
                self.assertEqual(result['expected_after_markers'],result['after_markers'])
                self.assertEqual(result['cleanup'],'confirmed')
                self.assertEqual(cohort_journal.stages,[('flyway-migrate','pending'),('flyway-migrate','done')])
            finally:
                owned.cleanup(); fixture_journal.close()


if __name__ == '__main__':
    unittest.main()
