import base64
import copy
import importlib.util
import json
import hashlib
import re
from pathlib import Path
import unittest
import yaml
import nonnats_plan
import source_plan

SOURCE = Path(__file__).resolve().parents[3]
TAG = 'a' * 40

def decoder(raw):
    return yaml.safe_load(raw)

class Fake:
    def __init__(self):
        self.objects = {}
        self.calls = []
        self.bad_clickhouse = False
    def get(self, kind, name):
        self.calls.append(('get', kind, name))
        return copy.deepcopy(self.objects[(kind, name)])
    def run(self, args, body=None, timeout=30):
        self.calls.append(('run', args))
        if args[0] == 'apply' and '--dry-run=server' in args:
            return copy.deepcopy(body)
        if args[0] == 'exec':
            password = 'private-authority'
            if args[1] == 'voice-postgres-0':
                if args[-2] == 'postgres': catalog = {'databases': sorted(source_plan.DATABASES), 'authenticated': True}
                else: catalog = {'columns': [['platform', 'character varying', 'NO'], ['min_supported', 'character varying', 'NO'],
                    ['latest_version', 'character varying', 'NO'], ['update_url', 'text', 'NO'], ['release_notes', 'text', 'YES'],
                    ['shorebird_patch', 'integer', 'YES'], ['updated_at', 'timestamp with time zone', 'YES']], 'primary_key': ['platform'], 'windows_seed': True}
            else:
                ddl = (SOURCE / 'docker/clickhouse/init/001_events.sql').read_text(encoding='utf-8')
                ddl = re.sub(r'^--.*$', '', ddl, flags=re.M)
                statements = {re.search(r'voice\.(\w+)', statement).group(1): statement.strip() for statement in ddl.split(';') if 'CREATE ' in statement and 'CREATE DATABASE' not in statement}
                catalog = {'data': [{'name': name, 'engine': engine, 'create_table_query': statements[name] + (' SETTINGS index_granularity = 10' if self.bad_clickhouse and name == 'events' else '')}
                    for name, engine in {'events': 'MergeTree', 'events_logical': 'View', 'dau_mv': 'AggregatingMergeTree', 'dau_mv_mv': 'MaterializedView',
                    'events_by_type_mv': 'SummingMergeTree', 'events_by_type_mv_mv': 'MaterializedView'}.items()]}
            return {'password_sha256': hashlib.sha256(password.encode()).hexdigest(), 'catalog': catalog}
        raise AssertionError('unsupported fake query')

def fixture():
    kube = Fake()
    spec = importlib.util.spec_from_file_location('canonical_secret_check', SOURCE / 'scripts/staging/check-resend-key.py')
    helper = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(helper)
    values = {key: 'private-authority' for key in helper.REQUIRED_KEYS}
    values.update(AUTH_RESEND_FROM='Voice <sender@example.invalid>', USER_R2_BUCKET='voice-staging-avatars', FILE_R2_BUCKET='voice-staging-files')
    for key in values:
        if key.endswith('_DATABASE_URL'):
            values[key] = 'postgres://voice:private-authority@voice-postgres:5432/' + key.removesuffix('_DATABASE_URL').lower() + '_db?sslmode=disable'
    values['CLICKHOUSE_DSN'] = 'clickhouse://default:private-authority@voice-clickhouse:9000/voice'
    kube.objects[('Secret', 'voice-app-secrets')] = {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-staging', 'uid': 'app-secret', 'resourceVersion': '1'},
        'data': {key: base64.b64encode(value.encode()).decode() for key, value in values.items()}}
    endpoint = 'https://storage.example.invalid'
    for service in ('file', 'user'):
        name = 'voice-' + service
        kube.objects[('Deployment', name)] = {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': {'name': name, 'namespace': 'voice-staging', 'uid': name, 'resourceVersion': '1'},
            'spec': {'replicas': 1, 'template': {'spec': {'containers': [{'name': service, 'image': 'registry/' + service + ':' + TAG,
                'env': [{'name': service.upper() + '_R2_SIGNING_ENDPOINT', 'value': endpoint}]}]}}}}
    kube.objects[('StatefulSet', 'voice-minio')] = {'apiVersion': 'apps/v1', 'kind': 'StatefulSet',
        'metadata': {'name': 'voice-minio', 'namespace': 'voice-staging', 'uid': 'minio', 'resourceVersion': '1'},
        'spec': {'template': {'spec': {'containers': [{'name': 'minio', 'env': [{'name': 'MINIO_API_CORS_ALLOW_ORIGIN', 'value': 'https://app.example.invalid'}]}]}}}}
    return kube, {'registry': 'registry', 'tag': TAG, 'mode': 'images-only', 'images': {}, 'generation': 'legacy', 'dataPVC': 'voice-nats-jsdata',
        's3_signing_endpoint': endpoint, 'gateway_host': '', 'livekit_host': '', 'storage_host': '', 'web_host': 'app.example.invalid'}

class SourceTests(unittest.TestCase):
    def canonical_fixture(self, mode='app-only'):
        kube, params = fixture()
        params.update(mode=mode, gateway_host='voice.comrade.click', livekit_host='livekit.comrade.click', web_host='app.comrade.click',
            developer_portal_host='developers.comrade.click', admin_host='admin.comrade.click',
            s3_signing_endpoint='https://voice.comrade.click', minio_image='minio@sha256:' + 'b' * 64,
            minio_mc_image='mc@sha256:' + 'c' * 64, minio_storage_class='local-path', minio_storage_size='20Gi')
        replacements = {'__IMAGE_REGISTRY__': 'registry', '__IMAGE_TAG__': TAG, 'IMAGE_PLACEHOLDER': 'registry/gateway:' + TAG,
            '__K_NAMESPACE__': 'voice-staging', '__NAMESPACE__': 'voice-staging', '__S3_SIGNING_ENDPOINT__': params['s3_signing_endpoint'],
            '__GATEWAY_INGRESS_HOST__': params['gateway_host'], '__DEVELOPER_PORTAL_INGRESS_HOST__': params['developer_portal_host'],
            '__WEB_INGRESS_HOST__': params['web_host'], '__ADMIN_INGRESS_HOST__': params['admin_host'], '__LIVEKIT_INGRESS_HOST__': params['livekit_host'],
            '__INGRESS_HOST__': params['gateway_host'], '__TLS_SECRET_NAME__': 'voice-gateway-tls',
            '__FILE_BUCKET__': 'voice-staging-files', '__AVATAR_BUCKET__': 'voice-staging-avatars', '__VOICE_MINIO_IMAGE__': params['minio_image'],
            '__VOICE_MINIO_MC_IMAGE__': params['minio_mc_image'], '__VOICE_MINIO_STORAGE_CLASS__': 'local-path', '__VOICE_MINIO_STORAGE_SIZE__': '20Gi',
            '__WEB_ORIGIN__': 'https://' + params['web_host'], '__LIVEKIT_API_KEY__': 'private-authority', '__LIVEKIT_API_SECRET__': 'private-authority'}
        paths = list(source_plan.APP_SOURCES) + ['deploy/staging/developer-portal.yaml', 'deploy/staging/flutter-web.yaml', 'deploy/staging/admin.yaml',
            'deploy/staging/configmap-app.yaml', 'deploy/staging/minio.yaml', 'deploy/gateway/ingress.yaml', 'deploy/livekit/ingress.yaml', 'deploy/staging/infra.yaml']
        for path in paths:
            raw = (SOURCE / path).read_text(encoding='utf-8')
            for token, value in replacements.items(): raw = raw.replace(token, value)
            if path == 'deploy/livekit/ingress.yaml': raw = raw.replace(params['gateway_host'], params['livekit_host'])
            for obj in yaml.safe_load_all(raw):
                if obj['metadata']['name'].startswith('voice-nats'): continue
                obj['metadata'].update(uid=obj['metadata']['name'], resourceVersion='1')
                if obj['kind'] == 'Job': obj['status'] = {'succeeded': 1}
                kube.objects[(obj['kind'], obj['metadata']['name'])] = obj
        for contract in nonnats_plan.SECRET_CONTRACTS.values():
            for name, keys in contract.items():
                kube.objects[('Secret', name)] = {'apiVersion': 'v1', 'kind': 'Secret',
                    'metadata': {'name': name, 'namespace': 'voice-staging', 'uid': name, 'resourceVersion': '1'},
                    'data': {key: base64.b64encode(b'private-authority').decode() for key in keys}}
        for name in ('voice-postgres', 'voice-clickhouse'):
            kube.objects[('Pod', name + '-0')] = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name + '-0', 'namespace': 'voice-staging', 'uid': name + '-0', 'resourceVersion': '1'}, 'spec': {}}
        for (kind, name), obj in list(kube.objects.items()):
            if kind != 'StatefulSet': continue
            for template in obj['spec'].get('volumeClaimTemplates', []):
                pvc = template['metadata']['name'] + '-' + name + '-0'
                kube.objects[('PersistentVolumeClaim', pvc)] = {'apiVersion': 'v1', 'kind': 'PersistentVolumeClaim',
                    'metadata': {'name': pvc, 'namespace': 'voice-staging', 'uid': pvc, 'resourceVersion': '1'},
                    'spec': {**copy.deepcopy(template['spec']), 'volumeName': 'pv-' + pvc}, 'status': {'phase': 'Bound'}}
        ddl = (SOURCE / 'docker/clickhouse/init/001_events.sql').read_text(encoding='utf-8')
        kube.objects[('ConfigMap', 'voice-clickhouse-init')] = {'apiVersion': 'v1', 'kind': 'ConfigMap',
            'metadata': {'name': 'voice-clickhouse-init', 'namespace': 'voice-staging', 'uid': 'ch-init', 'resourceVersion': '1'}, 'data': {'001_events.sql': ddl}}
        return kube, params

    def test_full_missing_canonical_auth_secret_key_veto(self):
        kube, params = self.canonical_fixture('full')
        del kube.objects[('Secret', 'voice-app-secrets')]['data']['ACCOUNT_DELETE_TOKEN_SECRET']
        with self.assertRaises(nonnats_plan.PlanError):
            source_plan.compile_plan(SOURCE, params, decoder, kube)

    def test_full_unbound_existing_minio_pvc_veto(self):
        kube, params = self.canonical_fixture('full')
        pvc = next(obj for (kind, name), obj in kube.objects.items() if kind == 'PersistentVolumeClaim' and 'minio' in name)
        pvc['status']['phase'] = 'Pending'
        with self.assertRaises(nonnats_plan.PlanError):
            source_plan.compile_plan(SOURCE, params, decoder, kube)

    def test_full_existing_database_url_password_sync_change_veto(self):
        kube, params = self.canonical_fixture('full')
        kube.objects[('Secret', 'voice-app-secrets')]['data']['USER_DATABASE_URL'] = base64.b64encode(b'postgres://wrong').decode()
        with self.assertRaises(nonnats_plan.PlanError):
            source_plan.compile_plan(SOURCE, params, decoder, kube)

    def test_wrong_clickhouse_ddl_veto_even_all_expected_engines_present(self):
        kube, params = self.canonical_fixture('full')
        kube.bad_clickhouse = True
        with self.assertRaisesRegex(nonnats_plan.PlanError, 'clickhouse'):
            source_plan.compile_plan(SOURCE, params, decoder, kube)

    def test_canonical_app_and_full_bind_specific_source_resources(self):
        for mode in ('app-only', 'full'):
            kube, params = self.canonical_fixture(mode)
            with self.subTest(mode=mode):
                plan = source_plan.compile_plan(SOURCE, params, decoder, kube)
                self.assertEqual({r['id'] for r in plan['checks']}, set(nonnats_plan.ROW_IDS))
                frontends = next(r for r in plan['checks'] if r['id'] == 'frontends')
                self.assertEqual({o['name'] for o in frontends['objects'] if o['kind'] == 'Deployment'}, {'voice-web', 'voice-admin', 'voice-developer-portal'})
                self.assertTrue(frontends['sources'])
                self.assertNotIn('private-authority', json.dumps(plan))
                if mode == 'full': self.assertTrue(source_plan.verify_db_init(kube, plan)['verified'])

    def test_present_frontend_change_is_planned_with_requested_image(self):
        kube, params = self.canonical_fixture()
        params['images']['voice-web/web'] = 'registry/web@sha256:' + 'd' * 64
        plan = source_plan.compile_plan(SOURCE, params, decoder, kube)
        action = next(a for a in plan['actions'] if a['manifest']['metadata']['name'] == 'voice-web' and a['manifest']['kind'] == 'Deployment')
        self.assertEqual(action['manifest']['spec']['template']['spec']['containers'][0]['image'], params['images']['voice-web/web'])

    def test_canonical_app_every_container_identity_pinned_before_action(self):
        kube, params = self.canonical_fixture()
        name = 'voice-user'
        template = kube.objects[('Deployment', name)]['spec']['template']['spec']
        for container in template.get('containers', []) + template.get('initContainers', []):
            params['images'][name + '/' + container['name']] = 'registry/' + container['name'] + '@sha256:' + 'e' * 64
        plan = source_plan.compile_plan(SOURCE, params, decoder, kube)
        action = next(a for a in plan['actions'] if a['manifest']['kind'] == 'Deployment' and a['manifest']['metadata']['name'] == name)
        actual = action['manifest']['spec']['template']['spec']
        for container in actual.get('containers', []) + actual.get('initContainers', []):
            self.assertEqual(container['image'], params['images'][name + '/' + container['name']])

    def test_missing_present_frontend_veto_before_fence(self):
        kube, params = self.canonical_fixture()
        del kube.objects[('Deployment', 'voice-web')]
        with self.assertRaisesRegex(nonnats_plan.PlanError, 'voice-web'): source_plan.compile_plan(SOURCE, params, decoder, kube)

    def test_full_init_incomplete_bucket_job_veto(self):
        kube, params = self.canonical_fixture('full')
        kube.objects[('Job', 'voice-minio-create-files-bucket')]['status'] = {'active': 1}
        with self.assertRaisesRegex(nonnats_plan.PlanError, 'bucket_job_incomplete'): source_plan.compile_plan(SOURCE, params, decoder, kube)

    def test_images_changed_cors_veto(self):
        kube, params = fixture()
        params['web_host'] = 'changed.example.invalid'
        with self.assertRaisesRegex(nonnats_plan.PlanError, 'cors'): source_plan.compile_plan(SOURCE, params, decoder, kube)

    def test_images_has_required_mail_and_actual_signing_contract(self):
        kube, params = fixture()
        plan = source_plan.compile_plan(SOURCE, params, decoder, kube)
        self.assertEqual({row['id'] for row in plan['checks']}, {'auth-mail', 'signing-endpoint'})
        names = {obj['name'] for row in plan['checks'] for obj in row['objects']}
        self.assertIn('voice-app-secrets', names)
        self.assertIn('voice-file', names)
        self.assertIn('voice-user', names)
        self.assertNotIn('private-authority', json.dumps(plan))
    def test_images_signing_change_veto_is_specific(self):
        kube, params = fixture()
        params['s3_signing_endpoint'] = 'https://changed.example.invalid'
        with self.assertRaisesRegex(nonnats_plan.PlanError, 'signing'):
            source_plan.compile_plan(SOURCE, params, decoder, kube)
    def test_explicit_authority_update_not_discarded(self):
        kube, params = fixture()
        params['secret_updates'] = ['voice-app-secrets']
        with self.assertRaisesRegex(nonnats_plan.PlanError, 'authority'):
            source_plan.compile_plan(SOURCE, params, decoder, kube)
    def test_invalid_mail_actual_veto(self):
        kube, params = fixture()
        kube.objects[('Secret', 'voice-app-secrets')]['data']['AUTH_RESEND_FROM'] = base64.b64encode(b'bad-email').decode()
        with self.assertRaises(nonnats_plan.PlanError): source_plan.compile_plan(SOURCE, params, decoder, kube)

if __name__ == '__main__': unittest.main()
