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

    def test_apply_provisions_gameintegration_runtime_role_before_schema_migration(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            migration_dir = root / 'src/backend/migrations/game_integration_db'
            migration_dir.mkdir(parents=True)
            (migration_dir / '000031_t58_app_notification_consent.up.sql').write_text('SELECT 1;\n', encoding='utf-8')
            provision_sql = root / 'docker/postgres/provision-gameintegration-runtime-role.sql'
            provision_sql.parent.mkdir(parents=True)
            provision_sql.write_text('-- isolated role-provisioning fixture\n', encoding='utf-8')
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

    def test_unknown_service_cannot_select_arbitrary_database_or_namespace(self):
        for service in ('../../staging', 'federation', 'gateway', ''):
            with self.assertRaises(ValueError):
                migrations.job(service, 'postgres:16-alpine')


if __name__ == '__main__':
    unittest.main()
