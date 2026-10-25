"""Source-bound, paused DEV package preparation. No cluster/build/issuer calls.

The caller supplies freshly issued private DEV material, registry receipts and a
YAML decoder. A verified source export may replace Git with a trusted manifest.
This module does not establish credential issuance or runtime qualification.
"""
import base64
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess

import environment as dev

STAGING = 'voice-staging'
REGISTRY = 'ghcr.io/poryadok/voiceroot'
FRONTENDS = ('web', 'admin', 'developer-portal')
JOBS = ('realtime', 'notification', 'analytics-chat', 'search')
SOURCE_PATHS = tuple('deploy/staging/' + name + '.yaml' for name in (
    'infra', 'minio', 'configmap-app', 'services', 'gateway-deployment',
    'flutter-web', 'admin', 'developer-portal')) + tuple(
    'deploy/templates/nats-' + role + '-bootstrap.yaml' for role in JOBS)
AUTHORITY_PATHS = ('scripts/dev/environment.py', 'scripts/dev/credentials.py',
    'scripts/dev/bootstrap.py', 'scripts/dev/migrations.py',
    'scripts/staging/nats-rollout-preservation/release_policy_wiring.py',
    'scripts/staging/init-postgres-databases.sh', 'scripts/staging/ensure-gateway-schema.sh',
    'scripts/staging/apply-clickhouse-init.sh', 'scripts/staging/lib/kubectl-configmap.sh',
    'docker/verification-stub/server.mjs')
REQUIRED_TOOLS = ('space-native-installer', 'migrate/migrate:v4.18.1',
    'flyway/flyway:11.3.0', 'node:24-alpine')
DIGEST = re.compile(r'[a-zA-Z0-9][a-zA-Z0-9./:_-]*@sha256:[a-f0-9]{64}')
TOKEN = re.compile(r'__[A-Z0-9_]+__|IMAGE_PLACEHOLDER')


def _fail(reason):
    raise ValueError('dev_package_' + reason)


def _read(root, path):
    target = root / path
    if (not target.is_file() or target.is_symlink() or target.resolve() != target
            or not target.resolve().is_relative_to(root)
            or not 0 < target.stat().st_size <= 2 << 20):
        _fail('source_file_invalid:' + path)
    return target.read_bytes()


def _source(root, source_sha, source_manifest):
    if not isinstance(source_sha, str) or not re.fullmatch(r'[a-f0-9]{40}', source_sha) or source_sha == '0' * 40:
        _fail('source_revision')
    policies = tuple(p.relative_to(root).as_posix() for p in sorted((root / 'deploy/templates').glob('network-policy-*.yaml'))
        if p.name != 'network-policy-nats-live-acl-proof.yaml')
    migrations = tuple(p.relative_to(root).as_posix() for p in sorted((root / 'src/backend/migrations').rglob('*')) if p.is_file())
    sql = tuple(p.relative_to(root).as_posix() for p in sorted((root / 'docker/clickhouse/init').glob('*.sql')))
    native = tuple(p.relative_to(root).as_posix() for p in sorted((root / 'src/backend').rglob('*.go')) if not p.name.endswith('_test.go'))
    auth_sql = tuple(p.relative_to(root).as_posix() for p in sorted((root / 'src/backend/auth/src/main/resources/db/migration').glob('*.sql')))
    postgres_sql = tuple(p.relative_to(root).as_posix() for p in sorted((root / 'docker/postgres').rglob('*.sql')))
    templates = tuple(p.relative_to(root).as_posix() for p in sorted((root / 'deploy/templates').glob('*.yaml')))
    required = set(SOURCE_PATHS + AUTHORITY_PATHS + policies + migrations + sql + native + auth_sql + postgres_sql + templates)
    if (root / '.git').exists():
        def git(*args):
            try:
                return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.DEVNULL, timeout=30)
            except (OSError, subprocess.SubprocessError):
                _fail('source_git_binding')
        if git('rev-parse', 'HEAD').decode().strip() != source_sha:
            _fail('source_head_mismatch')
        entries = {}
        for row in git('ls-tree', '-r', '-z', source_sha).split(b'\0'):
            if row:
                meta, path = row.split(b'\t', 1)
                mode, kind, digest = meta.decode().split()
                if mode in ('100644', '100755') and kind == 'blob':
                    entries[path.decode()] = digest
        expected = entries
    elif (isinstance(source_manifest, dict) and set(source_manifest) == {'source_sha', 'files'}
            and source_manifest['source_sha'] == source_sha and isinstance(source_manifest['files'], dict)
            and required <= set(source_manifest['files'])):
        expected = source_manifest['files']
        for path, digest in expected.items():
            if (not isinstance(path, str) or not path or str(PurePosixPath(path)) != path
                    or PurePosixPath(path).is_absolute() or '..' in PurePosixPath(path).parts
                    or '\\' in path or ':' in path or not isinstance(digest, str) or not re.fullmatch(r'[a-f0-9]{64}', digest)):
                _fail('archive_manifest_shape')
        required.update(expected)
    else:
        _fail('archive_manifest_required')
    raw, manifest = {}, []
    for path in sorted(required):
        data = _read(root, path)
        digest = hashlib.sha256(data).hexdigest()
        actual = hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest() if (root / '.git').exists() else digest
        if expected.get(path) != actual:
            _fail('source_content_mismatch:' + path)
        raw[path] = data
        manifest.append({'path': path, 'sha256': digest})
    return raw, manifest, policies


def _policy_module():
    # Execute the trusted installed renderer, never arbitrary code from root.
    path = Path(__file__).resolve().parents[1] / 'staging/nats-rollout-preservation/release_policy_wiring.py'
    spec = importlib.util.spec_from_file_location('dev_package_release_policy', path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _immutable(value):
    if not isinstance(value, str) or not DIGEST.fullmatch(value):
        _fail('immutable_image_required')
    return value


def _pins(value, source_sha):
    if not isinstance(value, dict) or set(value) != {'images', 'tools', 'frontends'}:
        _fail('image_pins_shape')
    for key in ('images', 'tools'):
        if not isinstance(value[key], dict):
            _fail('image_pins_shape')
        for image in value[key].values():
            _immutable(image)
    for name in REQUIRED_TOOLS:
        _immutable(value['tools'].get(name))
    if not isinstance(value['frontends'], dict) or set(value['frontends']) != set(FRONTENDS):
        _fail('frontend_inventory')
    for name, row in value['frontends'].items():
        fields = {'source_sha', 'api_base_url', 'reference'} | ({'livekit_url'} if name == 'web' else set())
        if (not isinstance(row, dict) or set(row) != fields or row['source_sha'] != source_sha
                or row['api_base_url'] != 'https://api-dev.comrade.click'
                or name == 'web' and row['livekit_url'] != 'wss://livekit-dev.comrade.click'):
            _fail('frontend_dev_binding:' + name)
        _immutable(row['reference'])


def _credential_objects(objects):
    if not isinstance(objects, list) or not objects:
        _fail('fresh_credentials_required')
    result = copy.deepcopy(objects)
    seen = set()
    for obj in result:
        meta = obj.get('metadata', {})
        name = meta.get('name', '')
        identity = (obj.get('kind'), name)
        if (obj.get('apiVersion') != 'v1' or obj.get('kind') not in ('Secret', 'ConfigMap')
                or meta.get('namespace') != dev.NS or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', name)
                or identity in seen or any(k in meta for k in ('uid', 'resourceVersion', 'managedFields', 'ownerReferences'))):
            _fail('fresh_credential_identity')
        seen.add(identity)
        if obj['kind'] == 'Secret':
            if (not isinstance(obj.get('data', {}), dict) or not isinstance(obj.get('stringData', {}), dict)
                    or any(not isinstance(v, str) for v in obj.get('stringData', {}).values())):
                _fail('fresh_secret_encoding')
            for value in obj.get('data', {}).values():
                try:
                    base64.b64decode(value, validate=True)
                except (ValueError, TypeError):
                    _fail('fresh_secret_encoding')
        elif obj.get('binaryData'):
            _fail('public_configmap_encoding')
    return result


def _value(objects, name, key):
    try:
        obj = next(o for o in objects if o['kind'] == 'Secret' and o['metadata']['name'] == name)
        if key in obj.get('stringData', {}):
            value = obj['stringData'][key]
        else:
            value = base64.b64decode(obj['data'][key], validate=True).decode()
        value = value.strip()
        if not value or '\n' in value or '\r' in value:
            _fail('fresh_secret_value')
        return value
    except (StopIteration, KeyError, UnicodeError, TypeError):
        _fail('missing_secret_key:' + name + '/' + key)


def _references(objects, secrets):
    dev.validate_references(objects, secrets)
    cms = {o['metadata']['name']: set(o.get('data', {})) for o in objects if o['kind'] == 'ConfigMap'}
    pvcs = {o['metadata']['name'] for o in objects if o['kind'] == 'PersistentVolumeClaim'}
    for obj in objects:
        for part in dev.walk(obj):
            for field in ('configMapKeyRef', 'configMapRef', 'configMap'):
                if field not in part:
                    continue
                ref = part[field]
                name = ref.get('name')
                if (name not in cms or ref.get('key') and ref['key'] not in cms[name]
                        or any(i['key'] not in cms[name] for i in ref.get('items', []))):
                    _fail('unresolved_configmap:' + str(name))
            if 'persistentVolumeClaim' in part and part['persistentVolumeClaim'].get('claimName') not in pvcs:
                _fail('unresolved_claim')


def _validate(result):
    if not isinstance(result, dict) or set(result) != {'prepared', 'secrets', 'foundation', 'receipt'}:
        _fail('package_shape')
    if any(o.get('kind') not in dev.ALLOWED | {'NetworkPolicy'} for o in result['prepared']):
        _fail('prepared_kind')
    if any(o.get('kind') != 'Secret' for o in result['secrets']):
        _fail('secret_kind')
    for obj in result['prepared'] + result['secrets']:
        if obj.get('metadata', {}).get('namespace') != dev.NS:
            _fail('output_namespace')
        if obj.get('kind') in ('Deployment', 'StatefulSet') and obj['spec'].get('replicas') != 0:
            _fail('output_not_paused')
        if obj.get('kind') == 'Job' and obj['spec'].get('suspend') is not True:
            _fail('output_job_not_suspended')
        if obj in result['prepared'] and TOKEN.search(json.dumps(obj)):
            _fail('unresolved_token')
        for part in dev.walk(obj):
            if 'image' in part:
                _immutable(part['image'])
            if any(part.get(k) for k in ('hostNetwork', 'hostPID', 'hostIPC', 'hostPath', 'privileged', 'hostPort')):
                _fail('output_host_access')
        claims = obj.get('spec', {}).get('volumeClaimTemplates', [])
        if obj.get('kind') == 'PersistentVolumeClaim':
            claims = claims + [obj]
        for claim in claims:
            spec = claim['spec']
            if spec.get('storageClassName') != 'local-path' or any(k in spec for k in ('volumeName', 'selector', 'dataSource', 'dataSourceRef')):
                _fail('fresh_dynamic_storage_required')
    for obj in result['foundation']:
        if obj['kind'] in ('PersistentVolume', 'StorageClass'):
            _fail('static_storage_forbidden')
        if obj['kind'] == 'Namespace':
            dev.validate_target(obj)
        elif obj['metadata'].get('namespace') != dev.NS:
            _fail('foundation_namespace')
    _references(result['prepared'], result['secrets'])


def render(root, source_sha, secret_objects, image_pins, release_policy, decoder,
           node='pmdebook', source_manifest=None):
    root = Path(root).absolute()
    if root.resolve() != root:
        _fail('source_root_symlink')
    raw, manifest, policies = _source(root, source_sha, source_manifest)
    _pins(image_pins, source_sha)
    material = _credential_objects(secret_objects)
    wiring = _policy_module()
    wiring.validate(release_policy, source_sha, STAGING)
    replacements = {'__NAMESPACE__': STAGING, '__K_NAMESPACE__': STAGING, '__IMAGE_REGISTRY__': REGISTRY,
        '__IMAGE_TAG__': source_sha, '__NATS_STORAGE_CLASS__': 'local-path', '__NATS_STORAGE_SIZE__': '1Gi',
        '__VOICE_MINIO_STORAGE_CLASS__': 'local-path', '__VOICE_MINIO_STORAGE_SIZE__': '5Gi',
        '__GATEWAY_INGRESS_HOST__': 'api-dev.comrade.click', '__LIVEKIT_INGRESS_HOST__': 'livekit-dev.comrade.click',
        '__WEB_INGRESS_HOST__': 'dev.comrade.click', '__ADMIN_INGRESS_HOST__': 'admin-dev.comrade.click',
        '__DEVELOPER_PORTAL_INGRESS_HOST__': 'developers-dev.comrade.click',
        '__WEB_ORIGIN__': 'https://dev.comrade.click', '__S3_SIGNING_ENDPOINT__': 'https://api-dev.comrade.click'}
    # The LiveKit source ConfigMap is replaced by a freshly rendered Secret below.
    replacements.update({'__LIVEKIT_API_KEY__': 'dev-input-placeholder', '__LIVEKIT_API_SECRET__': 'dev-input-placeholder'})
    for token, key in (('__NATS_APP_RESOLVER_PRELOAD__', 'account'), ('__NATS_SYS_RESOLVER_PRELOAD__', 'system-account')):
        public = _value(material, 'voice-nats-operator', key + '.public')
        jwt = _value(material, 'voice-nats-operator', key + '.jwt')
        replacements[token] = json.dumps(public) + ': ' + json.dumps(jwt) + ','
    replacements['__NATS_SYSTEM_ACCOUNT__'] = json.dumps(_value(material, 'voice-nats-operator', 'system-account.public'))
    for token, name in (('__VOICE_MINIO_IMAGE__', 'minio'), ('__VOICE_MINIO_MC_IMAGE__', 'minio-mc')):
        replacements[token] = _immutable(image_pins['images'].get(name) or image_pins['tools'].get(name))
    selected, network = [], []
    bindings = []
    for path in SOURCE_PATHS + policies:
        text = raw[path].decode('utf-8-sig')
        if path == 'deploy/staging/services.yaml':
            text = wiring.render(text, release_policy, source_sha, STAGING)
        for token, value in replacements.items():
            text = text.replace(token, value)
        text = text.replace('IMAGE_PLACEHOLDER', REGISTRY + '/gateway:' + source_sha)
        if TOKEN.search(text):
            _fail('unresolved_source_token:' + path)
        documents = decoder(text.encode())
        if not isinstance(documents, list):
            _fail('decoder_list_required')
        for original in documents:
            obj = copy.deepcopy(original)
            if obj.get('metadata', {}).get('namespace') != STAGING:
                _fail('source_namespace')
            name, kind = obj['metadata']['name'], obj['kind']
            if name == 'voice-federation' or kind == 'Deployment' and name == 'voice-nats':
                continue
            if kind == 'ConfigMap' and name == 'voice-livekit-config':
                continue
            if kind == 'Service' and name == 'voice-nats':
                obj['spec']['selector'] = {'app': 'voice-nats-pvc-candidate'}
            if kind == 'Deployment' and name == 'voice-role':
                # The canonical source already separates outbound trust. The
                # older shared DEV renderer accepts the pre-separation spelling
                # and deterministically restores the same qualified final path.
                role = next(c for c in obj['spec']['template']['spec']['containers'] if c['name'] == 'role')
                for env in role.get('env', []):
                    if env['name'] == 'S2S_JWKS_CA_FILE' and env.get('value') == '/etc/voice/media/ca/jwks-ca.crt':
                        env['value'] = '/etc/voice/media/ca/ca.crt'
            for part in dev.walk(obj):
                if 'image' in part:
                    old = part['image']
                    frontend = {'voice-' + n: n for n in FRONTENDS}.get(name) if kind == 'Deployment' else None
                    short = old.rsplit('/', 1)[-1].split(':')[0]
                    if frontend:
                        new = image_pins['frontends'][frontend]['reference']
                    else:
                        new = image_pins['images'].get(old) or image_pins['images'].get(short) or image_pins['tools'].get(old) or image_pins['tools'].get(short)
                        # Already pinned source digests need no mutable tag resolution.
                        if new is None and DIGEST.fullmatch(old):
                            new = old
                    part['image'] = _immutable(new)
                    bindings.append({'workload': name, 'source': old, 'reference': new})
                if part.get('configMap', {}).get('name') == 'voice-livekit-config':
                    cm = part.pop('configMap')
                    part['secret'] = {'secretName': 'voice-livekit-config', 'defaultMode': cm.get('defaultMode', 420)}
            if kind == 'NetworkPolicy':
                obj = dev.rewrite(obj)
                dev.clean_meta(obj['metadata'])
                network.append(obj)
            else:
                selected.append(obj)
    prepared = dev.render(selected, node) + network
    wiring.check_documents(selected, release_policy, source_sha, STAGING)
    public = [o for o in material if o['kind'] == 'ConfigMap']
    needed_public = {part[field]['name'] for obj in prepared for part in dev.walk(obj)
        for field in ('configMap', 'configMapKeyRef', 'configMapRef') if field in part}
    if any(o['metadata']['name'] not in needed_public for o in public):
        _fail('unreferenced_public_configmap')
    if any(o['metadata']['name'] in {p['metadata']['name'] for p in prepared if p['kind'] == 'ConfigMap'} for o in public):
        _fail('public_configmap_source_override')
    prepared += public + [dev.rewrite(copy.deepcopy(release_policy['policy_object']))]
    secrets = [o for o in material if o['kind'] == 'Secret']
    for obj in prepared:
        name = obj['metadata']['name']
        if obj['kind'] == 'ConfigMap' and name == 'voice-app-config':
            obj['data'].update(AUTH_RESEND_ENDPOINT='http://voice-dev-mail:4180/emails',
                GATEWAY_CORS_ALLOWED_ORIGINS='https://dev.comrade.click,https://admin-dev.comrade.click,https://developers-dev.comrade.click',
                AUTH_OAUTH_DEVELOPER_PORTAL_ENABLED='true')
        if obj['kind'] in ('Deployment', 'StatefulSet'):
            pod = obj['spec']['template']['spec']
            for c in pod['containers']:
                if name == 'voice-livekit':
                    for port in c.get('ports', []):
                        if port.get('containerPort') in (7881, 7882):
                            port['containerPort'] += 23200
                if name in ('voice-auth', 'voice-voice'):
                    key, value = ('AUTH_RESEND_ENDPOINT', 'http://voice-dev-mail:4180/emails') if name == 'voice-auth' else ('GODEBUG', 'http2client=0')
                    c['env'] = [e for e in c.get('env', []) if e['name'] != key] + [{'name': key, 'value': value}]
            for claim in obj['spec'].get('volumeClaimTemplates', []):
                claim['spec']['storageClassName'] = 'local-path'
        if obj['kind'] == 'PersistentVolumeClaim':
            obj['spec']['storageClassName'] = 'local-path'
    livekit = 'port: 7880\nrtc:\n  tcp_port: 31081\n  udp_port: 31082\n  use_external_ip: true\nprometheus:\n  port: 6789\nkeys:\n  '
    livekit += json.dumps(_value(secrets, 'voice-app-secrets', 'LIVEKIT_API_KEY')) + ': ' + json.dumps(_value(secrets, 'voice-app-secrets', 'LIVEKIT_API_SECRET')) + '\n'
    if any(o['metadata']['name'] == 'voice-livekit-config' for o in secrets):
        _fail('livekit_secret_override')
    secrets.append({'apiVersion': 'v1', 'kind': 'Secret', 'metadata': dev.metadata('voice-livekit-config'), 'stringData': {'livekit.yaml': livekit}})
    app = next(o for o in secrets if o['metadata']['name'] == 'voice-app-secrets')
    for key, value in {'AUTH_RESEND_API_KEY': 'voice-dev-mail-fixture', 'AUTH_RESEND_FROM': 'Voice Dev <noreply@voice.test>'}.items():
        app.setdefault('data', {})[key] = base64.b64encode(value.encode()).decode()
        app.get('stringData', {}).pop(key, None)
    # Canonical no-provider mail sink, generated without importing live bootstrap.
    mail = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': dev.metadata('voice-dev-mail'),
        'data': {'server.mjs': raw['docker/verification-stub/server.mjs'].decode()}}
    prepared += [mail, {'apiVersion': 'v1', 'kind': 'Service', 'metadata': dev.metadata('voice-dev-mail'),
        'spec': {'selector': {'app': 'voice-dev-mail'}, 'ports': [{'port': 4180, 'targetPort': 4180}]}},
        {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': dev.metadata('voice-dev-mail'),
        'spec': {'replicas': 0, 'selector': {'matchLabels': {'app': 'voice-dev-mail'}}, 'template': {
            'metadata': {'labels': {'app': 'voice-dev-mail'}}, 'spec': {'automountServiceAccountToken': False,
            'containers': [{'name': 'mail', 'image': _immutable(image_pins['tools'].get('node:24-alpine')),
                'command': ['node', '/stub/server.mjs'], 'ports': [{'containerPort': 4180}],
                'volumeMounts': [{'name': 'script', 'mountPath': '/stub', 'readOnly': True}],
                'readinessProbe': {'httpGet': {'path': '/health', 'port': 4180}},
                'resources': {'requests': {'cpu': '20m', 'memory': '32Mi'}, 'limits': {'cpu': '500m', 'memory': '128Mi'}}}],
            'volumes': [{'name': 'script', 'configMap': {'name': 'voice-dev-mail'}}]}}}}]
    result = {'prepared': prepared, 'secrets': secrets,
        'foundation': [o for o in dev.foundation(node) if o['kind'] != 'PersistentVolume'],
        'receipt': {'schema': 'voice-dev-reset-package-v1', 'source_sha': source_sha,
            'sources': manifest, 'images': bindings, 'tool_images': copy.deepcopy(image_pins['tools']),
            'frontends': copy.deepcopy(image_pins['frontends']), 'storage_class': 'local-path',
            'renderer_sha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
            'environment_helper_sha256': hashlib.sha256(Path(dev.__file__).read_bytes()).hexdigest(),
            'policy_helper_sha256': hashlib.sha256(Path(wiring.__file__).read_bytes()).hexdigest(),
            'storage_mode': 'fresh-dynamic-no-retained-pv-binding'}}
    _validate(result)
    return result


def write_package(output, result):
    _validate(result)
    output = Path(output)
    if os.name != 'posix' or not output.is_absolute() or output.exists() or output.is_symlink() or output.parent.resolve() != output.parent:
        _fail('private_fresh_output_required')
    info = output.parent.stat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != 0o700:
        _fail('private_parent_required')
    files = {name + '.json': json.dumps({'apiVersion': 'v1', 'kind': 'List', 'items': result[name]}, separators=(',', ':'))
        for name in ('prepared', 'secrets', 'foundation')}
    files['source-revision'] = result['receipt']['source_sha'] + '\n'
    files['package-receipt.json'] = json.dumps(result['receipt'], sort_keys=True)
    # Validate and serialize everything before an exclusive directory creation.
    # Never rename over a raced-in empty directory or overwrite an existing file.
    try:
        output.mkdir(mode=0o700)
    except FileExistsError:
        _fail('output_appeared')
    directory = os.open(output, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        for name, raw in files.items():
            fd = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=directory)
            with os.fdopen(fd, 'w', encoding='utf-8') as stream:
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())
        os.fsync(directory)
    finally:
        os.close(directory)
