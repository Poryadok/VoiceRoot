import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import migrations


class MigrationContracts(unittest.TestCase):
    def test_migration_waits_in_same_pod_and_never_embeds_database_password(self):
        job = migrations.job('bot', 'postgres@sha256:' + 'a' * 64)
        pod = job['spec']['template']['spec']
        self.assertEqual(job['metadata']['namespace'], 'voice-dev')
        self.assertEqual(pod['restartPolicy'], 'OnFailure')
        self.assertFalse(pod['automountServiceAccountToken'])
        self.assertIn('pg_isready', json.dumps(pod['initContainers']))
        env = pod['containers'][0]['env'][0]
        self.assertEqual(env['valueFrom']['secretKeyRef'], {'name': 'voice-app-secrets', 'key': 'BOT_DATABASE_URL'})
        self.assertNotIn('postgres://', json.dumps(job))

    def test_auth_uses_own_schema_and_secret_password(self):
        pod = migrations.job('auth', 'postgres:16-alpine')['spec']['template']['spec']
        self.assertEqual(pod['containers'][0]['args'], ['migrate'])
        password = next(e for e in pod['containers'][0]['env'] if e['name'] == 'FLYWAY_PASSWORD')
        self.assertNotIn('value', password)
        self.assertEqual(password['valueFrom']['secretKeyRef']['key'], 'POSTGRES_PASSWORD')

    def test_gameintegration_uses_explicit_db_name_and_restricted_runtime_url(self):
        job_name, config_name = migrations.names('gameintegration')
        self.assertEqual(job_name, 'voice-migrate-gameintegration-db')
        self.assertEqual(config_name, 'voice-gameintegration-db-migrations')
        pod = migrations.job('gameintegration', 'postgres@sha256:' + 'a' * 64)['spec']['template']['spec']
        env = {item['name']: item for item in pod['containers'][0]['env']}
        self.assertEqual(env['POSTGRES_USER']['valueFrom']['configMapKeyRef'],
                         {'name': 'voice-app-config', 'key': 'POSTGRES_USER'})
        self.assertEqual(env['PGPASSWORD']['valueFrom']['secretKeyRef'],
                         {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'})
        self.assertIn('game_integration_db', pod['containers'][0]['command'][-1])
        self.assertNotIn('GAME_INTEGRATION_DATABASE_URL', json.dumps(pod))
        self.assertNotIn('DATABASE_URL', env)
        command = pod['containers'][0]['command'][-1]
        self.assertIn('postgres://${POSTGRES_USER}:${PGPASSWORD}@voice-postgres:5432/game_integration_db', command)
        self.assertNotIn('password=', json.dumps(pod).lower())
        role_job = migrations.gameintegration_runtime_role_job('postgres@sha256:' + 'a' * 64)
        role_pod = role_job['spec']['template']['spec']
        role_env = {item['name']: item for item in role_pod['containers'][0]['env']}
        self.assertEqual(role_env['PGPASSWORD']['valueFrom']['secretKeyRef']['key'], 'POSTGRES_PASSWORD')
        self.assertEqual(role_env['GAME_INTEGRATION_DB_PASSWORD']['valueFrom']['secretKeyRef']['key'],
                         'GAME_INTEGRATION_DB_PASSWORD')
        self.assertFalse(role_pod['automountServiceAccountToken'])

    def test_gameintegration_role_job_is_not_a_migration_target(self):
        job = migrations.gameintegration_runtime_role_job('postgres:16-alpine')
        self.assertEqual(job['metadata']['name'], 'voice-provision-gameintegration-runtime-role')
        self.assertEqual(job['metadata']['namespace'], 'voice-dev')
        rendered = json.dumps(job)
        self.assertNotIn('voice-compose-gameintegration-runtime', rendered)
        self.assertNotIn('password=', rendered.lower())

    def test_gameintegration_prerequisite_contract_binds_fixed_inputs_and_secret_references(self):
        root = Path(__file__).resolve().parents[2]
        contract = migrations.gameintegration_prerequisite_inputs(root)
        self.assertEqual(contract['schema'], 'voice-gis-prerequisite-v1')
        self.assertEqual(contract['database'], 'game_integration_db')
        self.assertEqual([item['name'] for item in contract['migrations']], sorted(
            item['name'] for item in contract['migrations']))
        self.assertTrue(all(len(item['sha256']) == 64 for item in contract['migrations']))
        self.assertEqual(set(contract['jobs']), {'provision', 'migrate', 'verify'})
        self.assertEqual(contract['jobs']['verify'], 'voice-verify-gameintegration-db')
        self.assertEqual(contract['configmaps']['verify'], 'voice-gameintegration-db-verifier-sql')
        self.assertEqual(contract['job_images']['migrate']['migrate'], 'migrate')
        self.assertEqual(contract['job_images']['verify']['verify'], 'postgres')
        self.assertEqual(contract['images']['postgres'],
                         'docker.io/library/postgres@sha256:e013e867e712fec275706a6c51c966f0bb0c93cfa8f51000f85a15f9865a28cb')
        self.assertEqual(contract['images']['migrate'],
                         'migrate/migrate@sha256:f21c436af23c282f4516b00ba3e93bccf5c5fe5cd52530fd5c319a936998f539')
        self.assertEqual(contract['secret_refs']['runtime_database_url'],
                         {'name': 'voice-app-secrets', 'key': 'GAME_INTEGRATION_DATABASE_URL'})
        self.assertNotIn('value', json.dumps(contract))
        for role, template in contract['templates'].items():
            self.assertEqual(len(template), 64, role)
        verifier_cm = migrations.gameintegration_verifier_configmap(root, 'voice-staging')
        self.assertTrue(verifier_cm['immutable'])
        self.assertEqual(verifier_cm['metadata']['name'], 'voice-gameintegration-db-verifier-sql')
        self.assertEqual(verifier_cm['metadata']['namespace'], 'voice-staging')
        self.assertEqual(hashlib.sha256(verifier_cm['data']['verify.sql'].encode()).hexdigest(),
                         contract['verifier_sql_sha256'])
        with self.assertRaises(ValueError):
            migrations.gameintegration_verifier_configmap(root, 'voice-prod')

    def test_gameintegration_verifier_is_fixed_fail_closed_and_not_runtime_credentialed(self):
        root = Path(__file__).resolve().parents[2]
        contract = migrations.gameintegration_prerequisite_inputs(root)
        template = (root / 'deploy/templates/verify-game-integration-db-job.yaml').read_text(encoding='utf-8')
        sql = (root / 'docker/postgres/verify-gameintegration-runtime-role.sql').read_text(encoding='utf-8')
        self.assertIn('voice-gameintegration-db-verifier-sql', template)
        self.assertIn('voice-app-secrets', template)
        self.assertIn('POSTGRES_PASSWORD', template)
        self.assertIn('game_integration_db', template)
        self.assertIn('docker.io/library/postgres@sha256:', template)
        self.assertIn('ON_ERROR_STOP=1', template)
        for invariant in ('migration_version <> 33', 'migration_dirty', 'rolsuper', 'rolcreaterole',
                          'rolcreatedb', 'rolreplication', 'rolbypassrls', "'public', 'CREATE'",
                          "'TRUNCATE'", "'UPDATE'", 'REVOKE ALL PRIVILEGES ON TABLE public.schema_migrations',
                          "'public.schema_migrations', 'UPDATE'", "c.relname <> 'schema_migrations'",
                          "current_database(), 'TEMP'", 'REVOKE TEMPORARY ON DATABASE game_integration_db FROM PUBLIC'):
            self.assertIn(invariant, sql)
        provision_sql = (root / 'docker/postgres/provision-gameintegration-runtime-role.sql').read_text(encoding='utf-8')
        self.assertIn('REVOKE TEMPORARY ON DATABASE game_integration_db FROM PUBLIC', provision_sql)
        self.assertIn('REVOKE TEMPORARY ON DATABASE game_integration_db FROM gameintegration_runtime', provision_sql)
        self.assertNotIn('GAME_INTEGRATION_DB_PASSWORD', template)
        self.assertNotIn('GAME_INTEGRATION_DATABASE_URL', template)
        deployment = (root / 'deploy/staging/services.yaml').read_text(encoding='utf-8')
        gameintegration = deployment.split('name: voice-gameintegration', 1)[1].split('\n---', 1)[0]
        self.assertIn('key: GAME_INTEGRATION_DATABASE_URL', gameintegration)
        self.assertNotIn('POSTGRES_PASSWORD', gameintegration)
        self.assertNotIn('GAME_INTEGRATION_DB_PASSWORD', gameintegration)
        dev_job = migrations.gameintegration_verifier_job()
        dev_pod = dev_job['spec']['template']['spec']
        self.assertEqual(dev_job['metadata']['namespace'], 'voice-dev')
        self.assertEqual(dev_job['metadata']['name'], 'voice-verify-gameintegration-db')
        self.assertFalse(dev_pod['automountServiceAccountToken'])
        self.assertEqual(dev_pod['containers'][0]['image'], contract['images']['postgres'])
        self.assertEqual(dev_pod['containers'][0]['volumeMounts'][0]['mountPath'], '/verify')

    def test_gameintegration_verifier_emits_exact_collector_json_contract(self):
        root = Path(__file__).resolve().parents[2]
        sql = (root / 'docker/postgres/verify-gameintegration-runtime-role.sql').read_text(encoding='utf-8')
        record = sql.split('SELECT json_build_object(', 1)[1].split(')::text', 1)[0]
        fields = [line.strip() for line in record.splitlines() if line.strip()]
        self.assertEqual(fields, [
            "'schema', 'voice-gis-verifier-v3',",
            "'database', current_database(),",
            "'migration_version', version,",
            "'dirty', dirty,",
            "'runtime_role', 'gameintegration_runtime',",
            "'role_restricted', true,",
            "'table_grants', 'select_insert_update_delete',",
            "'sequence_grants', 'usage_select',",
            "'ddl_denied', true,",
            "'schema_ledger_runtime_mutation_denied', true,",
            "'authority_singleton_verified', true,",
            "'authority_triggers_verified', true,",
            "'authority_functions_verified', true",
        ])
        self.assertLess(sql.index('IF has_table_privilege(\'gameintegration_runtime\''),
                        sql.index('SELECT json_build_object('))

    def test_apply_provisions_gameintegration_runtime_role_before_schema_migration(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            migration_dir = root / 'src/backend/migrations/game_integration_db'
            migration_dir.mkdir(parents=True)
            (migration_dir / '000031_t58_app_notification_consent.up.sql').write_text('SELECT 1;\n', encoding='utf-8')
            provision_sql = root / 'docker/postgres/provision-gameintegration-runtime-role.sql'
            provision_sql.parent.mkdir(parents=True)
            provision_sql.write_text('-- isolated role-provisioning fixture\n', encoding='utf-8')
            verifier_sql = root / 'docker/postgres/verify-gameintegration-runtime-role.sql'
            verifier_sql.write_text('-- isolated ledger-finalization fixture\n', encoding='utf-8')
            applied = []
            commands = []
            def get(resource):
                return []
            def apply(objects):
                applied.append(objects)
            def run(*args, **kwargs):
                commands.append(args)
            with patch.object(migrations, 'SERVICES', ('gameintegration',)):
                migrations.apply_all(root, 'postgres:16-alpine', apply, run, get)
            self.assertEqual(applied[0][0]['metadata']['name'], 'voice-gameintegration-runtime-role-sql')
            self.assertEqual(applied[0][1]['metadata']['name'], 'voice-provision-gameintegration-runtime-role')
            self.assertEqual(commands[0][5], 'job/voice-provision-gameintegration-runtime-role')
            self.assertEqual(applied[1][0]['metadata']['name'], 'voice-gameintegration-db-migrations')
            self.assertEqual(applied[1][1]['metadata']['name'], 'voice-migrate-gameintegration-db')
            self.assertEqual(commands[1][5], 'job/voice-migrate-gameintegration-db')
            self.assertEqual(applied[2][0]['metadata']['name'], 'voice-gameintegration-db-verifier-sql')
            self.assertEqual(applied[2][1]['metadata']['name'], 'voice-verify-gameintegration-db')
            self.assertEqual(commands[2][5], 'job/voice-verify-gameintegration-db')

    def test_gameintegration_noop_schema_run_still_reapplies_ledger_privilege_finalizer(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            migration_dir = root / 'src/backend/migrations/game_integration_db'
            migration_dir.mkdir(parents=True)
            migration_file = migration_dir / '000031_t58_app_notification_consent.up.sql'
            migration_file.write_text('SELECT 1;\n', encoding='utf-8')
            provision_sql = root / 'docker/postgres/provision-gameintegration-runtime-role.sql'
            provision_sql.parent.mkdir(parents=True)
            provision_sql.write_text('-- provision fixture\n', encoding='utf-8')
            verifier_sql = root / 'docker/postgres/verify-gameintegration-runtime-role.sql'
            verifier_sql.write_text('-- ledger finalizer fixture\n', encoding='utf-8')
            digest = hashlib.sha256(json.dumps({migration_file.name: 'SELECT 1;\n'}, sort_keys=True).encode()).hexdigest()
            jobs = [
                {'metadata': {'name': 'voice-migrate-gameintegration-db'}, 'status': {'succeeded': 1}},
                {'metadata': {'name': 'voice-verify-gameintegration-db'}, 'status': {'succeeded': 1}},
            ]
            configmaps = [
                {'metadata': {'name': 'voice-gameintegration-db-migrations',
                              'annotations': {'voice.io/dev-migration-hash': digest}}},
                {'metadata': {'name': 'voice-gameintegration-db-verifier-sql'},
                 'data': {'verify.sql': '-- ledger finalizer fixture\n'}},
            ]
            applied, commands = [], []
            def get(resource):
                return jobs if resource == 'jobs' else configmaps
            def apply(objects):
                applied.append(objects)
            def run(*args, **kwargs):
                commands.append(args)
            with patch.object(migrations, 'SERVICES', ('gameintegration',)):
                migrations.apply_all(root, 'postgres:16-alpine', apply, run, get)
            self.assertEqual(len(applied), 2, 'role provisioning and verifier still run; schema migration is skipped')
            self.assertTrue(any('job/voice-verify-gameintegration-db' in command for command in commands))
            self.assertFalse(any('job/voice-migrate-gameintegration-db' in command and 'wait' in command
                                 for command in commands))

    def test_unknown_service_cannot_select_arbitrary_database_or_namespace(self):
        for service in ('../../staging', 'federation', 'gateway', ''):
            with self.assertRaises(ValueError):
                migrations.job(service, 'postgres:16-alpine')


if __name__ == '__main__':
    unittest.main()
