#!/usr/bin/env python3
"""Fresh voice-dev bootstrap on pmdebook; never admit writes to staging/prod.

Captures source workload configuration and immutable image identities only.
Secrets are independently issued. Existing dev installations require the
separate image update path; bootstrap never resets or overwrites their data.
"""
import argparse
import base64
import copy
import json
import os
import re
from pathlib import Path
import secrets as random_secrets
import subprocess
import time

import environment as dev
import migrations
import reset_bootstrap
import signal_tls

ROOT = Path(__file__).resolve().parents[2]
WORK = Path('/home/voice-dev/bootstrap')
NATS_JOBS = ['voice-nats-' + role + '-bootstrap' for role in ('realtime', 'notification', 'analytics-chat', 'search')]
INFRA = {'voice-redis', 'voice-nats-pvc-candidate', 'voice-livekit', 'voice-clamav'}


def run(*args, body=None, timeout=600, check=True):
    result = subprocess.run(args, input=body, capture_output=True, timeout=timeout)
    if check and result.returncode:
        (WORK / 'last-error.txt').write_bytes(result.stderr + result.stdout)
        raise RuntimeError('dev command failed: ' + args[0])
    return result


def get(kind, namespace='voice-dev'):
    return json.loads(run('kubectl', '-n', namespace, 'get', kind, '-o', 'json').stdout)['items']


def save(name, objects):
    path = WORK / name
    path.write_text(json.dumps({'apiVersion': 'v1', 'kind': 'List', 'items': objects}))
    path.chmod(0o600)


def apply(objects):
    dynamic = dynamic_storage()
    if dynamic:
        if any(obj['kind'] in ('PersistentVolume', 'StorageClass') for obj in objects):
            raise ValueError('dynamic bootstrap preserves all existing PVs/StorageClasses')
        objects = copy.deepcopy(objects)
        for obj in objects:
            for part in dev.walk(obj):
                if isinstance(part.get('image'), str):
                    image = part['image']
                    part['image'] = dynamic['tool_images'].get(image, image)
                    if (part['image'] not in dynamic['image_references']
                            or not re.fullmatch(r'[A-Za-z0-9._/:\-]+@sha256:[a-f0-9]{64}', part['image'])):
                        raise ValueError('fresh Kubernetes bootstrap requires immutable image: ' + image)
    for obj in objects:
        meta, kind = obj['metadata'], obj['kind']
        if kind == 'Namespace':
            dev.validate_target(obj)
        elif kind == 'PersistentVolume':
            if not meta['name'].startswith('voice-dev-') or obj['spec']['claimRef']['namespace'] != dev.NS:
                raise ValueError('foreign PV target')
        elif kind == 'StorageClass':
            if meta['name'] != 'voice-dev-local':
                raise ValueError('foreign StorageClass')
        elif meta.get('namespace') != dev.NS:
            raise ValueError('foreign namespace mutation forbidden')
    raw = json.dumps({'apiVersion': 'v1', 'kind': 'List', 'items': objects}).encode()
    run('kubectl', 'apply', '--dry-run=server', '-f', '-', body=raw)
    run('kubectl', 'apply', '-f', '-', body=raw)
    print('APPLIED_DEV_OBJECTS=' + str(len(objects)), flush=True)


def refs(objects, field):
    return {p[field]['name'] for obj in objects for p in dev.walk(obj) if field in p and 'name' in p[field]}


def image_key(image):
    return image.removeprefix('docker.io/').removeprefix('library/')


def canonical_bootstrap_data():
    data = {}
    for name in NATS_JOBS:
        raw = (ROOT / 'deploy/templates' / (name.removeprefix('voice-') + '.yaml')).read_text().replace('__NAMESPACE__', 'voice-staging')
        text = run('kubectl', 'create', '--dry-run=client', '-f', '-', '-o', 'json', body=raw.encode()).stdout.decode().lstrip()
        while text:
            parsed, end = json.JSONDecoder().raw_decode(text)
            for item in parsed.get('items', [parsed]):
                if item['kind'] == 'ConfigMap':
                    data[item['metadata']['name']] = item['data']
            text = text[end:].lstrip()
    return data


def prepare(secret_file, revision):
    if len(revision) != 40 or any(c not in '0123456789abcdef' for c in revision):
        raise ValueError('exact source revision required')
    namespaces = json.loads(run('kubectl', 'get', 'namespace', '-o', 'json').stdout)['items']
    if any(i['metadata']['name'] == dev.NS for i in namespaces):
        raise ValueError('dev namespace already exists; bootstrap is fresh-only')
    if (WORK / 'prepared.json').exists():
        raise ValueError('prepared bootstrap exists')
    secrets = json.loads(secret_file.read_text())['items']
    app = next(i for i in secrets if i['metadata']['name'] == 'voice-app-secrets')
    app_values = {k: base64.b64decode(v).decode() for k, v in app['data'].items()}
    # Dev-only Resend protocol sink: no external provider credential is reused.
    app['data']['AUTH_RESEND_API_KEY'] = base64.b64encode(b'voice-dev-mail-fixture').decode()
    app['data']['AUTH_RESEND_FROM'] = base64.b64encode(b'Voice Dev <noreply@voice.test>').decode()
    live = get('deployment,statefulset,service,configmap,ingress,job,pvc', 'voice-staging')
    selected = []
    for obj in live:
        name, kind = obj['metadata']['name'], obj['kind']
        if name in ('voice-federation', 'voice-nats-pvc-candidate') and kind == 'Service' or name == 'voice-federation':
            continue
        if kind in ('Deployment', 'StatefulSet', 'Service') and name.startswith('voice-'):
            selected.append(obj)
        elif kind == 'Ingress' and name.endswith('-http'):
            selected.append(obj)
        elif kind == 'Job' and name in NATS_JOBS + ['voice-minio-create-avatars-bucket', 'voice-minio-create-files-bucket']:
            selected.append(obj)
    needed_cms = refs(selected, 'configMap') | refs(selected, 'configMapRef') | refs(selected, 'configMapKeyRef')
    selected.extend(o for o in live if o['kind'] == 'ConfigMap' and o['metadata']['name'] in needed_cms and o['metadata']['name'] != 'voice-livekit-config')
    needed_pvcs = {p['persistentVolumeClaim']['claimName'] for o in selected for p in dev.walk(o) if 'persistentVolumeClaim' in p}
    selected.extend(o for o in live if o['kind'] == 'PersistentVolumeClaim' and o['metadata']['name'] in needed_pvcs)
    resolved = {}
    for pod in get('pods', 'voice-staging'):
        if pod.get('status', {}).get('phase') != 'Running':
            continue
        for status in pod.get('status', {}).get('containerStatuses', []) + pod.get('status', {}).get('initContainerStatuses', []):
            image_id = status.get('imageID', '').removeprefix('docker-pullable://')
            if '@sha256:' in image_id:
                resolved[image_key(status['image'])] = image_id
    for obj in selected:
        for part in dev.walk(obj):
            if 'image' in part and isinstance(part['image'], str):
                part['image'] = resolved.get(image_key(part['image']), part['image'])
            if part.get('name') in app_values and 'value' in part:
                part.pop('value')
                part['valueFrom'] = {'secretKeyRef': {'name': 'voice-app-secrets', 'key': part['name']}}
            if 'secretKeyRef' in part and part['secretKeyRef']['name'].startswith('voice-rollout-'):
                part['secretKeyRef']['name'] = 'voice-app-secrets'
            if part.get('configMap', {}).get('name') == 'voice-livekit-config':
                source = part.pop('configMap')
                part['secret'] = {'secretName': 'voice-livekit-config', 'defaultMode': source.get('defaultMode', 420)}
    rendered = dev.render(selected, 'pmdebook')
    canonical_data = canonical_bootstrap_data()
    for obj in rendered:
        name, kind = obj['metadata']['name'], obj['kind']
        if kind == 'ConfigMap' and name in canonical_data:
            obj['data'] = canonical_data[name]
        if kind == 'ConfigMap' and name == 'voice-app-config':
            obj['data']['AUTH_RESEND_ENDPOINT'] = 'http://voice-dev-mail:4180/emails'
            obj['data']['GATEWAY_CORS_ALLOWED_ORIGINS'] = 'https://dev.comrade.click,https://admin-dev.comrade.click,https://developers-dev.comrade.click'
            # No inherited mobile push, billing or external game activation.
            obj['data']['AUTH_OAUTH_DEVELOPER_PORTAL_ENABLED'] = 'true'
        if kind == 'Deployment' and name in ('voice-web', 'voice-admin', 'voice-developer-portal'):
            image = {'voice-web': 'web', 'voice-admin': 'admin', 'voice-developer-portal': 'developer-portal'}[name]
            obj['spec']['template']['spec']['containers'][0]['image'] = 'ghcr.io/poryadok/voiceroot/dev-' + image + ':' + revision
        if kind == 'Deployment' and name == 'voice-livekit':
            for port in obj['spec']['template']['spec']['containers'][0].get('ports', []):
                if port['containerPort'] in (7881, 7882):
                    port['containerPort'] += 23200
        if kind == 'Deployment' and name == 'voice-auth':
            obj['spec']['template']['spec']['containers'][0].setdefault('env', []).append({
                'name': 'AUTH_RESEND_ENDPOINT', 'value': 'http://voice-dev-mail:4180/emails'})
        if kind == 'Deployment' and name == 'voice-voice':
            # Public Cloudflare HTTP/2 response bodies stall with the current Go
            # client; HTTP/1.1 restores bounded HTTPS JWKS reads (TLS stays on).
            obj['spec']['template']['spec']['containers'][0].setdefault('env', []).append({
                'name': 'GODEBUG', 'value': 'http2client=0'})
    livekit = ('port: 7880\nrtc:\n  tcp_port: 31081\n  udp_port: 31082\n  use_external_ip: true\nprometheus:\n  port: 6789\nkeys:\n  '
               + app_values['LIVEKIT_API_KEY'] + ': ' + app_values['LIVEKIT_API_SECRET'] + '\n')
    secrets.append({'apiVersion': 'v1', 'kind': 'Secret', 'metadata': dev.metadata('voice-livekit-config'),
                    'stringData': {'livekit.yaml': livekit}})
    dev.validate_references(rendered, secrets)
    save('prepared.json', rendered)
    save('secrets.json', secrets)
    save('foundation.json', dev.foundation('pmdebook') + [{
        'apiVersion': 'storage.k8s.io/v1', 'kind': 'StorageClass',
        'metadata': {'name': 'voice-dev-local', 'labels': dict(dev.LABEL)},
        'provisioner': 'kubernetes.io/no-provisioner', 'volumeBindingMode': 'WaitForFirstConsumer'}])
    (WORK / 'source-revision').write_text(revision + '\n')
    print('PREPARED_PAUSED_DEV_OBJECTS=' + str(len(rendered)), flush=True)


def mail_fixture():
    code = (ROOT / 'docker/verification-stub/server.mjs').read_text()
    return [
        {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': dev.metadata('voice-dev-mail'), 'data': {'server.mjs': code}},
        {'apiVersion': 'v1', 'kind': 'Service', 'metadata': dev.metadata('voice-dev-mail'),
         'spec': {'selector': {'app': 'voice-dev-mail'}, 'ports': [{'port': 4180, 'targetPort': 4180}]}},
        {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': dev.metadata('voice-dev-mail'),
         'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': 'voice-dev-mail'}},
                  'template': {'metadata': {'labels': {'app': 'voice-dev-mail'}}, 'spec': {
                      'automountServiceAccountToken': False, 'containers': [{
                          'name': 'mail', 'image': 'node:24-alpine', 'command': ['node', '/stub/server.mjs'],
                          'resources': {'requests': {'cpu': '20m', 'memory': '32Mi'}, 'limits': {'cpu': '500m', 'memory': '128Mi'}},
                          'ports': [{'containerPort': 4180}], 'volumeMounts': [{'name': 'script', 'mountPath': '/stub', 'readOnly': True}],
                          'readinessProbe': {'httpGet': {'path': '/health', 'port': 4180}}}],
                      'volumes': [{'name': 'script', 'configMap': {'name': 'voice-dev-mail'}}]}}}}]


def repair_fresh_nats_claim():
    namespaces = json.loads(run('kubectl', 'get', 'namespace', dev.NS, '-o', 'json').stdout)
    dev.validate_target(namespaces)
    if any(Path('/home/voice-dev/data/nats').iterdir()):
        raise ValueError('fresh claim repair forbidden after broker data exists')
    objects = json.loads((WORK / 'prepared.json').read_text())['items']
    pvc = next(o for o in objects if o['kind'] == 'PersistentVolumeClaim' and o['metadata']['name'] == 'voice-nats-jsdata')
    dev.clean_meta(pvc['metadata'])
    pvc['spec']['volumeName'] = 'voice-dev-nats'
    apply([pvc])
    save('prepared.json', objects)
    run('kubectl', '-n', dev.NS, 'scale', 'deployment/voice-nats-pvc-candidate', '--replicas=1')
    print('FRESH_DEV_NATS_CLAIM_BOUND', flush=True)


def central_bootstrap():
    dev.validate_target(json.loads(run('kubectl', 'get', 'namespace', dev.NS, '-o', 'json').stdout))
    objects = json.loads((WORK / 'prepared.json').read_text())['items']
    canonical = canonical_bootstrap_data()
    for obj in objects:
        if obj['kind'] == 'ConfigMap' and obj['metadata']['name'] in canonical:
            obj['data'] = canonical[obj['metadata']['name']]
    apply([o for o in objects if o['kind'] == 'ConfigMap' and o['metadata']['name'] in canonical])
    save('prepared.json', objects)
    job = next(copy.deepcopy(o) for o in objects if o['kind'] == 'Job' and o['metadata']['name'] == NATS_JOBS[0])
    job['spec']['suspend'] = False
    apply([job])
    run('kubectl', '-n', dev.NS, 'wait', '--for=condition=complete', 'job/' + NATS_JOBS[0], '--timeout=180s')
    print('DEV_CENTRAL_STREAMS_BOOTSTRAPPED', flush=True)


def create_namespace(namespace, *, readback=False):
    dev.validate_target(namespace)
    args = ('kubectl', 'create', '-f', '-', '-o', 'json') if readback else ('kubectl', 'create', '-f', '-')
    result = run(*args, body=json.dumps(namespace).encode())
    return json.loads(result.stdout) if readback else None


def dynamic_storage():
    path = WORK / 'fresh-storage.json'
    if path.is_symlink():
        raise ValueError('fresh storage receipt symlink forbidden')
    if not path.exists():
        return None
    value = json.loads(path.read_text())
    if (value.get('schema') != 'voice-dev-fresh-storage-v1' or not value.get('old_namespace_uid')
            or not value.get('operation') or not value.get('retained') or not value.get('image_references')
            or not isinstance(value.get('tool_images'), dict)):
        raise ValueError('fresh storage receipt invalid')
    return value


def configure_work(path):
    """Explicit operator-owned work directory; never adopt protected old journals."""
    global WORK
    path = path.absolute()
    if path.is_symlink() or path.resolve() != path:
        raise ValueError('canonical work directory required')
    path.mkdir(mode=0o700, exist_ok=True)
    if os.name == 'posix' and (path.stat().st_uid != os.getuid() or path.stat().st_mode & 0o077):
        raise ValueError('caller-owned mode0700 work directory required')
    WORK = path


def start():
    rendered = json.loads((WORK / 'prepared.json').read_text())['items']
    secrets = json.loads((WORK / 'secrets.json').read_text())['items']
    dynamic = dynamic_storage()
    if dynamic:
        foundation = json.loads((WORK / 'foundation.json').read_text())['items']
        if any(o['kind'] not in ('Namespace', 'LimitRange', 'ResourceQuota', 'NetworkPolicy') for o in foundation):
            raise ValueError('dynamic foundation must not create or change PVs/StorageClasses')
        reset_bootstrap.verify_retained(json.loads(run('kubectl', 'get', 'pv', '-o', 'json').stdout)['items'], dynamic['retained'])
        for obj in rendered:
            for part in dev.walk(obj):
                if 'storageClassName' in part and (part['storageClassName'] != 'local-path'
                        or any(part.get(k) for k in ('volumeName', 'selector', 'dataSource', 'dataSourceRef'))):
                    raise ValueError('fresh dynamic claims required')
        tls = next((s for s in secrets if s['metadata']['name'] == signal_tls.SECRET), None)
        reset_bootstrap.tls_identity(tls, WORK)
    else:
        for directory in ('postgres', 'minio', 'clickhouse', 'nats'):
            path = Path('/home/voice-dev/data') / directory
            if path.exists() and any(path.iterdir()):
                raise ValueError('fresh bootstrap data directory not empty')
            path.mkdir(parents=True, exist_ok=True)
        foundation = dev.foundation('pmdebook') + [o for o in json.loads((WORK / 'foundation.json').read_text())['items'] if o['kind'] == 'StorageClass']
    save('foundation.json', foundation)
    # Exclusive creation closes the prepare/start race; never adopt a namespace.
    namespace = next(o for o in foundation if o['kind'] == 'Namespace')
    if dynamic:
        namespace['metadata'].setdefault('annotations', {})['voice.dev/reset-operation'] = dynamic['operation']
        current = reset_bootstrap.Kube().get('namespace', dev.NS)
        if current:
            dev.validate_target(current)
            if (current['metadata']['uid'] == dynamic['old_namespace_uid']
                    or current['metadata'].get('annotations', {}).get('voice.dev/reset-operation') != dynamic['operation']
                    or dynamic.get('new_namespace_uid', current['metadata']['uid']) != current['metadata']['uid']):
                raise ValueError('existing namespace is not this interrupted fresh bootstrap')
        else:
            if dynamic.get('new_namespace_uid'):
                raise ValueError('fresh namespace disappeared; do not recreate automatically')
            current = create_namespace(namespace, readback=True)
        dynamic['new_namespace_uid'] = current['metadata']['uid']
        receipt = WORK / 'fresh-storage.json'
        temporary = WORK / 'fresh-storage.next'
        with temporary.open('x', encoding='utf-8') as stream:
            json.dump(dynamic, stream, sort_keys=True)
            stream.flush()
            os.fsync(stream.fileno())
        temporary.chmod(0o600)
        os.replace(temporary, receipt)
    else:
        create_namespace(namespace)
    apply([o for o in foundation if o['kind'] != 'Namespace'])
    if dynamic:
        apply([s for s in secrets if s['metadata']['name'] != signal_tls.SECRET])
        reset_bootstrap.restore_tls(reset_bootstrap.Kube(), tls, WORK)
    else:
        apply(secrets)
    # LiveKit public media is separately scoped; no datastore ingress is exposed.
    media_policy = {'apiVersion': 'networking.k8s.io/v1', 'kind': 'NetworkPolicy',
                    'metadata': dev.metadata('dev-livekit-media'), 'spec': {
                        'podSelector': {'matchLabels': {'app': 'voice-livekit'}}, 'policyTypes': ['Ingress'],
                        'ingress': [{'ports': [{'port': 31081, 'protocol': 'TCP'}, {'port': 31082, 'protocol': 'UDP'}]}]}}
    apply([media_policy])
    apply([o for o in rendered if o['kind'] not in ('Deployment', 'StatefulSet', 'Job', 'Ingress')])
    apply(mail_fixture())
    if dynamic:
        # Restore the known TLS renewal surface before slow data migrations.
        web = next(copy.deepcopy(o) for o in rendered if o['kind'] == 'Deployment' and o['metadata']['name'] == 'voice-web')
        web['spec']['replicas'] = 1
        apply([web])
        run('kubectl', '-n', dev.NS, 'rollout', 'status', 'deployment/voice-web', '--timeout=300s')
        apply([signal_tls.ingress(secure, host) for host in signal_tls.ROUTES for secure in (False, True)])
    infra = [copy.deepcopy(o) for o in rendered if o['kind'] == 'StatefulSet' or o['kind'] == 'Deployment' and o['metadata']['name'] in INFRA]
    for obj in infra:
        obj['spec']['replicas'] = 1
    apply(infra)
    for name in ('voice-postgres', 'voice-minio', 'voice-clickhouse'):
        run('kubectl', '-n', dev.NS, 'rollout', 'status', 'statefulset/' + name, '--timeout=300s')
    for name in ('voice-nats-pvc-candidate', 'voice-redis', 'voice-livekit'):
        run('kubectl', '-n', dev.NS, 'rollout', 'status', 'deployment/' + name, '--timeout=300s')
    if dynamic:
        reset_bootstrap.verify_fresh(json.loads(run('kubectl', 'get', 'namespace', dev.NS, '-o', 'json').stdout),
            get('pvc'), json.loads(run('kubectl', 'get', 'pv', '-o', 'json').stdout)['items'],
            old_namespace_uid=dynamic['old_namespace_uid'], retained=dynamic['retained'])
    ordered_jobs = sorted([copy.deepcopy(o) for o in rendered if o['kind'] == 'Job'],
                          key=lambda o: NATS_JOBS.index(o['metadata']['name']) if o['metadata']['name'] in NATS_JOBS else 10)
    for job in ordered_jobs:
        job['spec']['suspend'] = False
        apply([job])
        run('kubectl', '-n', dev.NS, 'wait', '--for=condition=complete', 'job/' + job['metadata']['name'], '--timeout=300s')
    activate()


def rotate_postgres():
    dev.validate_target(json.loads(run('kubectl', 'get', 'namespace', dev.NS, '-o', 'json').stdout))
    items = json.loads((WORK / 'secrets.json').read_text())['items']
    app = next(o for o in items if o['metadata']['name'] == 'voice-app-secrets')
    old = base64.b64decode(app['data']['POSTGRES_PASSWORD']).decode()
    password = random_secrets.token_hex(32)
    for key in app['data']:
        value = base64.b64decode(app['data'][key]).decode()
        if key == 'POSTGRES_PASSWORD' or key.endswith('_DATABASE_URL'):
            app['data'][key] = base64.b64encode(value.replace(old, password).encode()).decode()
    run('kubectl', '-n', dev.NS, 'exec', '-i', 'voice-postgres-0', '--', 'psql', '-U', 'voice', '-d', 'voice', '-v', 'ON_ERROR_STOP=1',
        body=("ALTER USER voice WITH PASSWORD '" + password + "';\n").encode())
    apply([app])
    save('secrets.json', items)
    print('DEV_POSTGRES_CREDENTIALS_ROTATED', flush=True)


def activate():
    dev.validate_target(json.loads(run('kubectl', 'get', 'namespace', dev.NS, '-o', 'json').stdout))
    rendered = json.loads((WORK / 'prepared.json').read_text())['items']
    env = dict(os.environ, VOICE_K8S_NAMESPACE=dev.NS, VOICE_ENABLE_GAME_INTEGRATION_DB='true')
    initial = subprocess.run(['bash', str(ROOT / 'scripts/staging/init-postgres-databases.sh')], env=env, capture_output=True, timeout=600)
    (WORK / 'init-postgres-databases.sh.log').write_bytes(initial.stdout + initial.stderr)
    if initial.returncode:
        raise RuntimeError('dev database creation failed')
    postgres_image = next(o for o in rendered if o['kind'] == 'StatefulSet' and o['metadata']['name'] == 'voice-postgres')['spec']['template']['spec']['containers'][0]['image']
    migrations.apply_all(ROOT, postgres_image, apply, run, get)
    for script in ('ensure-gateway-schema.sh', 'apply-clickhouse-init.sh'):
        result = subprocess.run(['bash', str(ROOT / 'scripts/staging' / script)], env=env, capture_output=True, timeout=1800)
        (WORK / (script + '.log')).write_bytes(result.stdout + result.stderr)
        if result.returncode:
            raise RuntimeError('dev database initialization failed: ' + script)
    apps = [copy.deepcopy(o) for o in rendered if o['kind'] == 'Deployment' and o['metadata']['name'] not in INFRA]
    user = next(o for o in apps if o['metadata']['name'] == 'voice-user')
    original_env = copy.deepcopy(user['spec']['template']['spec']['containers'][0].get('env', []))
    user['spec']['template']['spec']['containers'][0]['env'] = [e for e in original_env if e['name'] != 'SPACE_GRPC_ADDR'] + [{'name': 'SPACE_GRPC_ADDR', 'value': ''}]
    for obj in apps:
        obj['spec']['replicas'] = 1
    apply(apps)
    for name in ('voice-auth', 'voice-user', 'voice-space'):
        run('kubectl', '-n', dev.NS, 'rollout', 'status', 'deployment/' + name, '--timeout=600s', timeout=660)
    user['spec']['template']['spec']['containers'][0]['env'] = original_env
    apply([user])
    for obj in apps:
        run('kubectl', '-n', dev.NS, 'rollout', 'status', 'deployment/' + obj['metadata']['name'], '--timeout=600s', timeout=660)
    apply([o for o in rendered if o['kind'] == 'Ingress'])
    configure_routes()
    (WORK / 'stage').write_text('READY_FOR_APPLICATION_SMOKE\n')
    print('DEV_READY_FOR_APPLICATION_SMOKE', flush=True)


def configure_routes():
    dev.validate_target(json.loads(run('kubectl', 'get', 'namespace', dev.NS, '-o', 'json').stdout))
    run('kubectl', '-n', dev.NS, 'set', 'env', 'deployment/voice-auth', '--containers=auth', 'AUTH_RESEND_ENDPOINT=http://voice-dev-mail:4180/emails')
    run('kubectl', '-n', dev.NS, 'rollout', 'status', 'deployment/voice-auth', '--timeout=180s')
    run('kubectl', '-n', dev.NS, 'set', 'env', 'deployment/voice-voice', '--containers=voice', 'GODEBUG=http2client=0')
    run('kubectl', '-n', dev.NS, 'rollout', 'status', 'deployment/voice-voice', '--timeout=180s')
    items = json.loads((WORK / 'secrets.json').read_text())['items']
    app = next(o for o in items if o['metadata']['name'] == 'voice-app-secrets')
    values = {k: base64.b64decode(v).decode() for k, v in app['data'].items()}
    livekit = next(o for o in items if o['metadata']['name'] == 'voice-livekit-config')
    livekit['stringData']['livekit.yaml'] = ('port: 7880\nrtc:\n  tcp_port: 31081\n  udp_port: 31082\n'
        '  use_external_ip: false\n  node_ip: 95.31.10.177\nprometheus:\n  port: 6789\nkeys:\n  '
        + values['LIVEKIT_API_KEY'] + ': ' + values['LIVEKIT_API_SECRET'] + '\n')
    apply([livekit])
    save('secrets.json', items)
    apply([{'apiVersion': 'networking.k8s.io/v1', 'kind': 'NetworkPolicy',
            'metadata': dev.metadata('dev-livekit-media'), 'spec': {
                'podSelector': {'matchLabels': {'app': 'voice-livekit'}}, 'policyTypes': ['Ingress', 'Egress'],
                'ingress': [{'ports': [{'port': 31081, 'protocol': 'TCP'}, {'port': 31082, 'protocol': 'UDP'}]}],
                'egress': [{'to': [{'ipBlock': {'cidr': '0.0.0.0/0', 'except': ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16', '169.254.0.0/16', '127.0.0.0/8']}}],
                            'ports': [{'protocol': 'UDP', 'port': 1, 'endPort': 65535}]},
                           # Real LAN clients advertise these ICE candidates;
                           # admit only LiveKit media to their unprivileged UDP ports.
                           {'to': [{'ipBlock': {'cidr': '192.168.0.0/24'}}],
                            'ports': [{'protocol': 'UDP', 'port': 1024, 'endPort': 65535}]}]}},
           {'apiVersion': 'networking.k8s.io/v1', 'kind': 'Ingress', 'metadata': dev.metadata('voice-dev-avatars-http'),
            'spec': {'ingressClassName': 'traefik', 'rules': [{'host': 'api-dev.comrade.click', 'http': {'paths': [
                {'path': '/voice-dev-avatars', 'pathType': 'Prefix', 'backend': {'service': {'name': 'voice-minio', 'port': {'number': 9000}}}}]}}]}}])
    run('kubectl', '-n', dev.NS, 'rollout', 'restart', 'deployment/voice-livekit')
    run('kubectl', '-n', dev.NS, 'rollout', 'status', 'deployment/voice-livekit', '--timeout=180s')
    print('DEV_ROUTES_AND_MEDIA_CONFIGURED', flush=True)


def main():
    global ROOT
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('prepare', 'start', 'repair-fresh-nats-claim', 'central-bootstrap', 'rotate-postgres', 'activate', 'configure-routes'))
    parser.add_argument('--secrets', type=Path)
    parser.add_argument('--revision')
    parser.add_argument('--work-dir', type=Path, default=WORK)
    parser.add_argument('--source-root', type=Path, default=ROOT)
    args = parser.parse_args()
    os.umask(0o077)
    configure_work(args.work_dir)
    ROOT = args.source_root.resolve(strict=True)
    try:
        if args.action == 'prepare':
            prepare(args.secrets, args.revision)
        elif args.action == 'start':
            start()
        elif args.action == 'repair-fresh-nats-claim':
            repair_fresh_nats_claim()
        elif args.action == 'central-bootstrap':
            central_bootstrap()
        elif args.action == 'rotate-postgres':
            rotate_postgres()
        elif args.action == 'activate':
            activate()
        else:
            configure_routes()
    except Exception as exc:
        (WORK / 'failure.txt').write_text(type(exc).__name__ + ': ' + str(exc) + '\n')
        print('Dev bootstrap failed; inspect private bootstrap/failure.txt and last-error.txt', flush=True)
        raise SystemExit(1)


if __name__ == '__main__':
    main()
