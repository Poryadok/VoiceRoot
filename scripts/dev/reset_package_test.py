import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import shutil
import tempfile
import unittest

import yaml
import reset_package as package

ROOT = Path(os.environ.get('DEV_PACKAGE_TEST_ROOT', Path(__file__).resolve().parents[2]))
SHA = os.environ.get('DEV_PACKAGE_TEST_SOURCE_SHA') or subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT).decode().strip()
MANIFEST = json.loads(Path(os.environ['DEV_PACKAGE_TEST_MANIFEST']).read_text()) if os.environ.get('DEV_PACKAGE_TEST_MANIFEST') else None
PATHS = tuple('deploy/staging/' + p + '.yaml' for p in (
    'infra', 'minio', 'configmap-app', 'services', 'gateway-deployment',
    'flutter-web', 'admin', 'developer-portal')) + tuple(
    'deploy/templates/nats-' + role + '-bootstrap.yaml'
    for role in ('realtime', 'notification', 'analytics-chat', 'search'))
OVERLAY_PATHS = tuple('deploy/dev/match-squad-trust-overlay/' + name for name in (
    'voice-matchmaking.service.patch.yaml', 'voice-chat.service.patch.yaml', 'voice-voice.service.patch.yaml',
    'voice-matchmaking.runtime.patch.yaml', 'voice-chat.runtime.patch.yaml', 'voice-voice.runtime.patch.yaml', 'voice-gateway.patch.yaml'))


def decode(raw):
    return [o for o in yaml.safe_load_all(raw) if o]


def policy():
    raw = json.dumps({'schema_version': 2, 'profile': 'free-invite-alpha',
        'source_sha': SHA, 'capabilities': {k: 'disabled' for k in (
            'phone_auth', 'paid', 'mobile_push', 'public_discovery', 'unfinished_rich_types', 'federation')},
        'rich_intents': dict({k: 'qualified' for k in ('text', 'photo', 'video', 'document', 'music')},
            **{k: 'disabled' for k in ('voice', 'sticker', 'gif', 'article', 'location',
                'video_note', 'send_silent', 'schedule', 'when_online')})}, sort_keys=True)
    return {'profile': 'free-invite-alpha', 'source_sha': SHA,
        'policy_sha256': hashlib.sha256(raw.encode()).hexdigest(),
        'policy_object': {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
            'metadata': {'name': 'voice-release-policy-dev', 'namespace': 'voice-staging'},
            'data': {'policy.json': raw}}, 'owners': {name: {
                'policy_directory': '/etc/voice/release', 'snapshot_file': '/tmp/' + name + '-release.json'}
                for name in ('auth', 'user', 'notification', 'realtime')}}


def fixture():
    """Reference-complete synthetic material, never an issued trust claim."""
    secrets, public = {}, {}
    images = {}
    for path in PATHS + OVERLAY_PATHS:
        text = (ROOT / path).read_text(encoding='utf-8')
        text = re.sub(r'__NATS_(?:APP|SYS)_RESOLVER_PRELOAD__', '', text)
        text = re.sub(r'__[A-Z0-9_]+__', 'fixture', text)
        for obj in decode(text):
            if obj.get('metadata', {}).get('name') == 'voice-nats' and obj['kind'] in ('Deployment', 'Service'):
                continue
            for part in package.dev.walk(obj):
                if 'image' in part:
                    image = part['image']
                    name = image.split('/')[-1].split(':')[0]
                    images[image] = 'registry.example/' + name + '@sha256:' + 'a' * 64
                    images[name] = images[image]
                for field in ('secretKeyRef', 'secretRef', 'secret', 'configMapKeyRef', 'configMapRef', 'configMap'):
                    ref = part.get(field)
                    if not isinstance(ref, dict):
                        continue
                    name = ref.get('name') or ref.get('secretName')
                    if not name:
                        continue
                    target = secrets if field.startswith('secret') else public
                    keys = target.setdefault(name, set())
                    if ref.get('key'):
                        keys.add(ref['key'])
                    keys.update(i['key'] for i in ref.get('items', []))
    secrets.setdefault('voice-app-secrets', set()).update(('LIVEKIT_API_KEY', 'LIVEKIT_API_SECRET',
        'POSTGRES_PASSWORD', 'AUTH_RESEND_API_KEY', 'AUTH_RESEND_FROM',
        'AUTH_DB_USERNAME', 'AUTH_DB_PASSWORD', 'USER_DB_PASSWORD'))
    secrets.setdefault('voice-nats-operator', set()).update(('account.public', 'account.jwt',
        'system-account.public', 'system-account.jwt'))
    objects = [{'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': name, 'namespace': 'voice-dev'},
        'data': {k: base64.b64encode(('DEV-' + k).encode()).decode() for k in keys}}
        for name, keys in secrets.items()]
    # Canonical source/config and the release descriptor supply these themselves.
    provided = {'voice-app-config', 'voice-nats-leaf-config', 'voice-nats-hub-config',
        'voice-p3-principal-publisher-config', 'voice-livekit-config'} | {
        'voice-nats-' + r + '-bootstrap' for r in ('realtime', 'notification', 'analytics-chat', 'search')}
    objects += [{'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': name, 'namespace': 'voice-dev'},
        'data': {k: 'synthetic-public' for k in keys}} for name, keys in public.items()
        if name not in provided and name != 'fixture']
    frontends = {name: {'source_sha': SHA, 'api_base_url': 'https://api-dev.comrade.click',
        'reference': 'registry.example/dev-' + name + '@sha256:' + 'b' * 64}
        for name in ('web', 'admin', 'developer-portal')}
    frontends['web']['livekit_url'] = 'wss://livekit-dev.comrade.click'
    images['minio'] = 'registry.example/minio@sha256:' + 'a' * 64
    images['minio-mc'] = 'registry.example/mc@sha256:' + 'a' * 64
    images['gateway'] = 'registry.example/gateway@sha256:' + 'a' * 64
    tools = {name: 'registry.example/tool@sha256:' + 'c' * 64 for name in (
        'node:24-alpine', 'migrate/migrate:v4.18.1', 'flyway/flyway:11.3.0', 'space-native-installer')}
    return objects, {'images': images, 'tools': tools, 'frontends': frontends}


class SourceInventoryTests(unittest.TestCase):
    def test_deleted_tracked_scoped_inputs_cannot_disappear_from_receipt(self):
        scoped = ('src/backend/migrations/example/001.up.sql',
            'src/backend/example/main.go', 'docker/clickhouse/init/001.sql',
            'src/backend/auth/src/main/resources/db/migration/V1__fixture.sql',
            'docker/postgres/fixture.sql', 'deploy/templates/network-policy-fixture.yaml',
            'deploy/templates/extra-fixture.yaml')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            paths = set(package.SOURCE_PATHS + package.AUTHORITY_PATHS + package.OVERLAY_PATHS + scoped
                + ('deploy/dev/match-squad-trust-overlay/README.md',))
            for path in paths:
                (root / path).parent.mkdir(parents=True, exist_ok=True)
                (root / path).write_bytes(b'synthetic source\n')
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.DEVNULL, timeout=10)
            git('init', '-q')
            git('add', '.')
            git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.test',
                'commit', '-q', '-m', 'Synthetic source inventory')
            sha = git('rev-parse', 'HEAD').decode().strip()
            _, receipt, policies = package._source(root, sha, None)
            self.assertTrue(set(scoped) <= {row['path'] for row in receipt})
            self.assertIn('deploy/templates/network-policy-fixture.yaml', policies)
            for path in scoped:
                with self.subTest(path=path):
                    target = root / path
                    data = target.read_bytes()
                    target.unlink()
                    try:
                        with self.assertRaisesRegex(ValueError, 'source_file_invalid:' + re.escape(path)):
                            package._source(root, sha, None)
                    finally:
                        target.write_bytes(data)
            extra = root / 'src/backend/migrations/example/002.up.sql'
            extra.write_bytes(b'synthetic extra SQL\n')
            with self.assertRaisesRegex(ValueError, 'source_content_mismatch:.*002.up.sql'):
                package._source(root, sha, None)


class PackageTests(unittest.TestCase):
    def setUp(self):
        self.material, self.pins = fixture()

    def render(self, **kw):
        args = dict(root=ROOT, source_sha=SHA, secret_objects=self.material,
            image_pins=self.pins, release_policy=policy(), decoder=decode, source_manifest=MANIFEST)
        args.update(kw)
        return package.render(**args)

    def test_actual_current_source_is_complete_immutable_and_paused(self):
        before = copy.deepcopy((self.material, self.pins))
        result = self.render()
        self.assertGreater(len(result['prepared']), 60)
        self.assertEqual(before, (self.material, self.pins))
        by_name = {(o['kind'], o['metadata']['name']): o for o in result['prepared']}
        self.assertNotIn(('Deployment', 'voice-nats'), by_name)
        self.assertIn(('Deployment', 'voice-nats-pvc-candidate'), by_name)
        for obj in result['prepared']:
            self.assertEqual(obj['metadata']['namespace'], 'voice-dev')
            if obj['kind'] in ('Deployment', 'StatefulSet'):
                self.assertEqual(obj['spec']['replicas'], 0)
            if obj['kind'] == 'Job':
                self.assertIs(obj['spec']['suspend'], True)
            for part in package.dev.walk(obj):
                if 'image' in part:
                    self.assertRegex(part['image'], r'@sha256:[a-f0-9]{64}$')
        ch = by_name['StatefulSet', 'voice-clickhouse']['spec']['template']['spec']['containers'][0]
        self.assertEqual(ch['resources']['limits']['memory'], '4096Mi')
        self.assertEqual(ch['resources']['requests']['memory'], '256Mi')
        for obj in result['prepared']:
            checked = copy.deepcopy(obj)
            if checked['kind'] == 'ConfigMap' and checked['metadata']['name'] == 'voice-nats-hub-config':
                checked['data'].pop('nats.conf')
            self.assertNotRegex(json.dumps(checked), r'__[A-Z0-9_]+__|IMAGE_PLACEHOLDER')
        self.assertNotIn('DEV-LIVEKIT_API_SECRET', json.dumps(result['receipt']))
        self.assertFalse(any(o['kind'] in ('PersistentVolume', 'StorageClass') for o in result['foundation']))
        for obj in result['prepared']:
            claims = obj.get('spec', {}).get('volumeClaimTemplates', []) + ([obj] if obj['kind'] == 'PersistentVolumeClaim' else [])
            for claim in claims:
                self.assertEqual(claim['spec']['storageClassName'], 'local-path')
                self.assertFalse({'volumeName', 'selector', 'dataSource', 'dataSourceRef'} & set(claim['spec']))

    def test_source_archive_manifest_binds_revision_and_every_input_byte(self):
        receipt = self.render()['receipt']
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            files = {r['path']: r['sha256'] for r in receipt['sources']}
            for path in files:
                (root / path).parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / path, root / path)
            envelope = {'source_sha': SHA, 'files': files}
            self.assertGreater(len(self.render(root=root, source_manifest=envelope)['prepared']), 60)
            with self.assertRaises(ValueError):
                self.render(root=root, source_manifest={'source_sha': '1' * 40, 'files': files})
            bad = copy.deepcopy(envelope)
            bad['files'].pop('scripts/dev/credentials.py')
            with self.assertRaises(ValueError):
                self.render(root=root, source_manifest=bad)
            (root / 'deploy/staging/infra.yaml').write_text('changed')
            with self.assertRaises(ValueError):
                self.render(root=root, source_manifest=envelope)

    def test_wrong_source_revision_or_missing_archive_manifest_rejected(self):
        with self.assertRaises(ValueError):
            self.render(source_sha='0' * 40)
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ValueError):
                self.render(root=Path(directory))

    def test_all_workload_pods_are_pinned_to_selected_node_and_conflicts_reject(self):
        result = self.render(node='pmdebook')
        self.assertEqual(result['receipt']['node'], 'pmdebook')
        for obj in result['prepared']:
            if obj['kind'] in ('Deployment', 'StatefulSet', 'Job'):
                self.assertEqual(obj['spec']['template']['spec']['nodeSelector']['kubernetes.io/hostname'], 'pmdebook')
        def conflicting(raw):
            documents = decode(raw)
            for obj in documents:
                if obj['kind'] in ('Deployment', 'StatefulSet', 'Job'):
                    obj['spec']['template']['spec']['nodeSelector'] = {'kubernetes.io/hostname': 'other-node'}
                    break
            return documents
        with self.assertRaisesRegex(ValueError, 'node'):
            self.render(decoder=conflicting)

    def test_only_canonical_nats_runtime_template_keeps_its_three_tokens(self):
        result = self.render()
        hub = next(o for o in result['prepared'] if o['kind'] == 'ConfigMap' and o['metadata']['name'] == 'voice-nats-hub-config')
        tokens = ('__NATS_APP_RESOLVER_PRELOAD__', '__NATS_SYS_RESOLVER_PRELOAD__', '__NATS_SYSTEM_ACCOUNT__')
        for token in tokens:
            self.assertEqual(hub['data']['nats.conf'].count(token), 1)
        for mutation in ('elsewhere', 'duplicate', 'missing', 'unknown'):
            bad = copy.deepcopy(result)
            config = next(o for o in bad['prepared'] if o['kind'] == 'ConfigMap' and o['metadata']['name'] == 'voice-nats-hub-config')
            if mutation == 'elsewhere':
                config['metadata']['annotations'] = {'unexpected': tokens[0]}
            elif mutation == 'duplicate':
                config['data']['nats.conf'] += tokens[0]
            elif mutation == 'missing':
                config['data']['nats.conf'] = config['data']['nats.conf'].replace(tokens[0], '')
            else:
                config['data']['nats.conf'] += '__UNKNOWN_RENDER_TOKEN__'
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                package._validate(bad)

    def test_final_dev_overlay_is_source_bound_and_keeps_directional_trust(self):
        result = self.render()
        self.assertEqual(set(result['receipt']['overlay_sources']), set(OVERLAY_PATHS))
        by_name = {(o['kind'], o['metadata']['name']): o for o in result['prepared']}
        mm = by_name['Deployment', 'voice-matchmaking']['spec']['template']['spec']
        env = {e['name']: e for e in next(c for c in mm['containers'] if c['name'] == 'matchmaking')['env']}
        self.assertEqual(env['MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD'], {'name': 'MATCHMAKING_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD', 'value': ''})
        self.assertEqual(json.loads(env['S2S_JWKS_URLS_JSON']['value']), {'gateway': 'https://api-dev.comrade.click/.well-known/voice-principal-jwks.json'})
        volumes = {v['name']: v for v in mm['volumes']}
        self.assertEqual(volumes['matchmaking-complete-client-ca']['configMap']['name'], 'voice-matchmaking-complete-client-ca')
        self.assertEqual(volumes['match-squad-ca']['configMap']['name'], 'voice-match-squad-ca')
        gateway = by_name['Deployment', 'voice-gateway']['spec']['template']['spec']
        gateway_env = {e['name']: e for e in next(c for c in gateway['containers'] if c['name'] == 'gateway')['env']}
        self.assertEqual(gateway_env['GATEWAY_MATCH_SQUAD_MEMBER_TLS_CA_FILE']['value'], '/etc/voice/match-squad/ca/ca.crt')
        self.assertEqual(gateway_env['GATEWAY_MATCHMAKING_COMPLETE_TLS_SERVER_NAME']['value'], 'voice-matchmaking.voice-dev.svc')

    def test_mutable_unmapped_and_wrong_dev_frontend_inputs_rejected(self):
        for name, key, value in [('web', 'api_base_url', 'https://voice.comrade.click'),
                ('web', 'livekit_url', 'wss://livekit.comrade.click'),
                ('admin', 'source_sha', '1' * 40), ('admin', 'reference', 'registry/admin:latest')]:
            with self.subTest(name=name, key=key):
                pins = copy.deepcopy(self.pins)
                pins['frontends'][name][key] = value
                with self.assertRaises(ValueError):
                    self.render(image_pins=pins)
        with self.assertRaises(ValueError):
            self.render(image_pins={**self.pins, 'images': {}})
        for tool in ('space-native-installer', 'migrate/migrate:v4.18.1', 'flyway/flyway:11.3.0', 'node:24-alpine'):
            with self.subTest(tool=tool):
                pins = copy.deepcopy(self.pins)
                del pins['tools'][tool]
                with self.assertRaises(ValueError):
                    self.render(image_pins=pins)

    def test_missing_fresh_secret_configmap_and_key_references_rejected(self):
        for name in ('voice-app-secrets', 'voice-p3-principal-public'):
            with self.subTest(name=name):
                with self.assertRaises(ValueError):
                    self.render(secret_objects=[o for o in self.material if o['metadata']['name'] != name])
        material = copy.deepcopy(self.material)
        next(o for o in material if o['metadata']['name'] == 'voice-app-secrets')['data'].pop('POSTGRES_PASSWORD')
        with self.assertRaises(ValueError):
            self.render(secret_objects=material)

    def test_decoder_host_access_foreign_namespace_and_unknown_tokens_rejected(self):
        for mutation in ('host', 'namespace', 'token', 'storage'):
            def hostile(raw):
                docs = decode(raw)
                for o in docs:
                    if o.get('kind') == 'Deployment':
                        if mutation == 'host':
                            o['spec']['template']['spec']['hostNetwork'] = True
                        elif mutation == 'namespace':
                            o['metadata']['namespace'] = 'voice-prod'
                        elif mutation == 'token':
                            o['metadata']['annotations'] = {'unexpected': '__UNKNOWN_RENDER_TOKEN__'}
                        else:
                            continue
                        break
                    if mutation == 'storage' and o.get('kind') == 'PersistentVolumeClaim':
                        o['spec']['dataSource'] = {'name': 'old-retained-data', 'kind': 'VolumeSnapshot', 'apiGroup': 'snapshot.storage.k8s.io'}
                        break
                return docs
            with self.subTest(mutation=mutation):
                with self.assertRaises(ValueError):
                    self.render(decoder=hostile)

    @unittest.skipIf(os.name == 'nt', 'POSIX ownership/mode contract requires Linux')
    def test_private_package_is_create_only_and_invalid_result_leaves_no_output(self):
        result = self.render()
        with tempfile.TemporaryDirectory() as scratch:
            parent = Path(scratch)
            parent.chmod(0o700)
            target = parent / 'package'
            package.write_package(target, result)
            self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o700)
            for name in ('prepared.json', 'secrets.json', 'foundation.json', 'source-revision', 'package-receipt.json'):
                self.assertEqual(stat.S_IMODE((target / name).stat().st_mode), 0o600)
            with self.assertRaises(ValueError):
                package.write_package(target, result)
            link = parent / 'alias'
            link.symlink_to(target, target_is_directory=True)
            with self.assertRaises(ValueError):
                package.write_package(link, result)
            exposed = parent / 'exposed'
            exposed.mkdir(mode=0o755)
            exposed.chmod(0o755)
            with self.assertRaises(ValueError):
                package.write_package(exposed / 'invalid', result)
            bad = copy.deepcopy(result)
            bad['prepared'][0]['metadata']['namespace'] = 'voice-staging'
            with self.assertRaises(ValueError):
                package.write_package(parent / 'invalid', bad)
            self.assertFalse((parent / 'invalid').exists())


if __name__ == '__main__':
    unittest.main()
