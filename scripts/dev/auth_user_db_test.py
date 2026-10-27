"""DEV credential ownership regression; execute on Linux, never a live namespace."""
import base64
import copy
import json
from pathlib import Path
import unittest
from urllib.parse import urlparse

import credentials
import environment
import migrations
import reset_credentials
from unittest.mock import patch


def deployment(service):
    env = ([{'name': 'SPRING_DATASOURCE_USERNAME', 'value': 'voice'},
            {'name': 'SPRING_DATASOURCE_PASSWORD', 'valueFrom': {'secretKeyRef': {
                'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'}}},
            {'name': 'AUTH_FLYWAY_ENABLED', 'value': 'false'}] if service == 'auth' else
           [{'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {
               'name': 'voice-app-secrets', 'key': 'USER_DATABASE_URL'}}}])
    return {'kind': 'Deployment', 'metadata': {'name': 'voice-' + service, 'namespace': 'voice-staging'},
            'spec': {'template': {'spec': {'containers': [{'name': service,
                'image': 'registry/' + service + '@sha256:' + 'a' * 64, 'env': env}]}}}}


class AuthUserDatabaseContract(unittest.TestCase):
    def test_provisioning_job_keeps_credentials_in_secret_refs_and_waits(self):
        applied, commands = [], []
        root = Path(__file__).resolve().parents[2]
        migrations.apply_auth_user_roles(root, 'postgres:16-alpine', applied.extend,
            lambda *args, **kwargs: commands.append(args), lambda kind: [])
        pod = applied[1]['spec']['template']['spec']
        self.assertEqual(applied[0]['data']['provision.sql'],
            (root / 'docker/postgres/provision-auth-user-runtime-roles.sql').read_text())
        env = {e['name']: e['valueFrom']['secretKeyRef']['key'] for e in pod['containers'][0]['env']}
        self.assertEqual(env, {'PGPASSWORD': 'POSTGRES_PASSWORD',
            'AUTH_DB_PASSWORD': 'AUTH_DB_PASSWORD', 'USER_DB_PASSWORD': 'USER_DB_PASSWORD'})
        self.assertFalse(pod['automountServiceAccountToken'])
        self.assertTrue(any('wait' in command for command in commands))

    def test_fresh_roles_and_passwords_are_distinct_from_each_other_and_admin(self):
        values = credentials.application_values(b'key', 'api-dev.comrade.click')
        user = urlparse(values['USER_DATABASE_URL'])
        self.assertEqual(user.username, 'user_runtime')
        self.assertEqual(values['AUTH_DB_USERNAME'], 'auth_runtime')
        self.assertEqual(user.password, values['USER_DB_PASSWORD'])
        self.assertEqual(len({values['POSTGRES_PASSWORD'], values['AUTH_DB_PASSWORD'], user.password}), 3)

    def test_auth_render_does_not_receive_administrator_or_user_credentials(self):
        source = deployment('auth'); before = copy.deepcopy(source)
        rendered = environment.render([source], 'pmdebook')[0]
        env = {e['name']: e for e in rendered['spec']['template']['spec']['containers'][0]['env']}
        for name, key in [('SPRING_DATASOURCE_USERNAME', 'AUTH_DB_USERNAME'),
                          ('SPRING_DATASOURCE_PASSWORD', 'AUTH_DB_PASSWORD')]:
            self.assertEqual(env[name]['valueFrom']['secretKeyRef'], {'name': 'voice-app-secrets', 'key': key})
        self.assertEqual(source, before)
        self.assertNotIn('POSTGRES_PASSWORD', json.dumps(rendered))
        self.assertNotIn('USER_DATABASE_URL', json.dumps(rendered))

    def test_auth_broad_secret_import_and_in_process_migration_fail_closed(self):
        for mutation in ['broad', 'flyway']:
            source = deployment('auth'); container = source['spec']['template']['spec']['containers'][0]
            if mutation == 'broad':container['envFrom'] = [{'secretRef': {'name': 'voice-app-secrets'}}]
            else:container['env'][-1]['value'] = 'true'
            with self.assertRaises(ValueError):environment.render([source], 'pmdebook')

    def test_auth_cannot_import_user_credentials_by_alias_or_shared_volume(self):
        for mutation in ['alias', 'volume']:
            source = deployment('auth')
            pod = source['spec']['template']['spec']
            if mutation == 'alias':
                pod['containers'][0]['env'].append({'name': 'UNRELATED_ALIAS', 'valueFrom': {
                    'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'USER_DATABASE_URL'}}})
            else:
                pod['volumes'] = [{'name': 'shared', 'secret': {'secretName': 'voice-app-secrets'}}]
                pod['containers'][0]['volumeMounts'] = [{'name': 'shared', 'mountPath': '/shared'}]
            with self.assertRaises(ValueError):
                environment.render([source], 'pmdebook')

    def test_user_migrations_keep_admin_authority_out_of_runtime_dsn(self):
        pod = migrations.job('user', 'postgres:16-alpine')['spec']['template']['spec']
        container = pod['containers'][0]
        self.assertNotIn('USER_DATABASE_URL', json.dumps(container))
        self.assertIn('POSTGRES_PASSWORD', json.dumps(container))
        self.assertIn('user_db', json.dumps(container))

    def test_reset_qualifies_only_distinct_auth_user_credentials(self):
        values = credentials.application_values(b'key', 'api-dev.comrade.click')
        values['REALTIME_DATABASE_URL'] = values['NOTIFICATION_DATABASE_URL'].replace('notification_db', 'realtime_db')
        values.update(USER_R2_ACCESS_KEY_ID='minio', USER_R2_SECRET_ACCESS_KEY='minio-password',
                      FILE_R2_ACCESS_KEY_ID='minio', FILE_R2_SECRET_ACCESS_KEY='minio-password')
        def secret(name, vals):return {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': name, 'namespace': 'voice-dev'},
            'data': {k: base64.b64encode(v if isinstance(v, bytes) else v.encode()).decode() for k,v in vals.items()}}
        objects = [secret('voice-app-secrets', values), secret('voice-minio-credentials',
                   {'MINIO_ROOT_USER': 'minio', 'MINIO_ROOT_PASSWORD': 'minio-password'})]
        reset_credentials.verify_application(objects)
        bad = copy.deepcopy(objects)
        bad[0]['data']['AUTH_DB_PASSWORD'] = bad[0]['data']['POSTGRES_PASSWORD']
        with self.assertRaises(ValueError):reset_credentials.verify_application(bad)


if __name__ == '__main__':unittest.main()
