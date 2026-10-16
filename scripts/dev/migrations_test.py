import json
import unittest

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

    def test_unknown_service_cannot_select_arbitrary_database_or_namespace(self):
        for service in ('../../staging', 'federation', 'gateway', ''):
            with self.assertRaises(ValueError):
                migrations.job(service, 'postgres:16-alpine')


if __name__ == '__main__':
    unittest.main()
