"""Pure DEV Realtime startup planner; the caller verifies all receipts."""
import copy
import hashlib
import json
import re

ASSETS = {
    'voice-app-secrets': {'REALTIME_DATABASE_URL'},
    'voice-app-config': {'REDIS_ADDR'},
    'voice-realtime-principal-signing': {'current.pem', 'next.pem'},
    'voice-p3-principal-public': {'realtime.json'},
    'voice-realtime-p3-tls': {'tls.crt', 'tls.key'},
    'voice-chat-realtime-p3-tls': {'tls.crt', 'tls.key'},
    'voice-p3-principal-jwks-tls': {'tls.crt', 'tls.key'},
    'voice-p3-principal-replay': {'password'},
    'voice-p3-principal-trust': {'chat-server-ca.crt', 'space-client-ca.crt',
                                 'space-jwks-ca.crt', 'realtime-client-ca.crt',
                                 'realtime-jwks-ca.crt'},
}
PUBLIC_HASHES = {'realtime.json', 'space-jwks.json', 'space-jwks-tls.crt', 'realtime-tls.crt', 'chat-tls.crt', 'jwks-tls.crt',
                 'chat-server-ca.crt', 'space-client-ca.crt', 'space-jwks-ca.crt',
                 'realtime-client-ca.crt', 'realtime-jwks-ca.crt'}


def _digest(value):
    return isinstance(value, str) and bool(re.fullmatch(r'[0-9a-f]{64}', value))


def _env(name, value):
    return {'name': name, 'value': value}


def _ref(name, kind, obj, key):
    return {'name': name, 'valueFrom': {kind + 'KeyRef': {'name': obj, 'key': key}}}


def plan_service(original_service, expected_pod_labels):
    """Plan only the missing lifecycle port; the caller owns apply and recovery."""
    meta = original_service.get('metadata', {})
    if (original_service.get('apiVersion'), original_service.get('kind'), meta.get('name'), meta.get('namespace')) != ('v1', 'Service', 'voice-realtime', 'voice-dev'):
        raise ValueError('unexpected Service')
    if not all(isinstance(meta.get(k), str) and meta[k] for k in ('uid', 'resourceVersion')):
        raise ValueError('Service identity required')
    spec = original_service['spec']
    if spec.get('type', 'ClusterIP') != 'ClusterIP' or not isinstance(spec.get('clusterIP'), str) or spec['clusterIP'] in ('', 'None'):
        raise ValueError('routable ClusterIP Service required')
    selector = spec.get('selector')
    if not isinstance(expected_pod_labels, dict) or not isinstance(selector, dict) or not selector or any(expected_pod_labels.get(k) != v for k, v in selector.items()):
        raise ValueError('Service selector does not select realtime')
    ports = spec.get('ports')
    if not isinstance(ports, list) or not all(isinstance(p, dict) for p in ports) or any(p.get('name') == 'realtime-lifecycle' or p.get('port') == 9091 for p in ports):
        raise ValueError('Service lifecycle port collision')
    target = copy.deepcopy(spec)
    target['ports'].append({'name': 'realtime-lifecycle', 'port': 9091, 'targetPort': 9091, 'protocol': 'TCP'})
    return ([{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
             {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
             {'op': 'test', 'path': '/spec', 'value': copy.deepcopy(spec)},
             {'op': 'replace', 'path': '/spec', 'value': target}], target)


def plan(original_deployment, admitted_image, assets_receipts):
    """Return (JSON Patch, target spec, receipt digest); caller verifies evidence."""
    if not isinstance(assets_receipts, dict) or set(assets_receipts) != {'assets', 'schema', 'public_hashes', 'service', 'transport_evidence_sha256'}:
        raise ValueError('unexpected receipt fields')
    if not isinstance(admitted_image, str) or not re.fullmatch(r'[^\s@]+@sha256:[0-9a-f]{64}', admitted_image):
        raise ValueError('image must be digest pinned')
    meta = original_deployment.get('metadata', {})
    if (original_deployment.get('apiVersion'), original_deployment.get('kind'), meta.get('name'), meta.get('namespace')) != ('apps/v1', 'Deployment', 'voice-realtime', 'voice-dev'):
        raise ValueError('unexpected deployment')
    if not all(isinstance(meta.get(k), str) and meta[k] for k in ('uid', 'resourceVersion')):
        raise ValueError('deployment identity required')
    assets = assets_receipts.get('assets', {})
    if set(assets) != set(ASSETS):
        raise ValueError('asset inventory incomplete')
    for name, keys in ASSETS.items():
        item = assets[name]
        if not isinstance(item, dict) or not all(isinstance(item.get(k), str) and item[k] for k in ('uid', 'resourceVersion')):
            raise ValueError('asset identity required')
        expected_fields = {'uid', 'resourceVersion', 'keys', 'kind', 'namespace'}
        if name == 'voice-p3-principal-public':
            expected_fields.add('immutable')
        if set(item) != expected_fields:
            raise ValueError('unexpected receipt fields')
        kind = 'ConfigMap' if name in ('voice-app-config', 'voice-p3-principal-public') else 'Secret'
        if item['kind'] != kind or item['namespace'] != 'voice-dev':
            raise ValueError('unexpected asset identity')
        actual = item.get('keys')
        if not isinstance(actual, list) or not all(isinstance(k, str) for k in actual) or len(actual) != len(set(actual)) or not keys.issubset(actual):
            raise ValueError('asset keys incomplete')
        if name not in ('voice-app-secrets', 'voice-app-config') and set(actual) != keys:
            raise ValueError('unexpected asset keys')
    if assets['voice-p3-principal-public'].get('immutable') is not True:
        raise ValueError('public issuer set must be immutable')
    public, schema = assets_receipts.get('public_hashes', {}), assets_receipts.get('schema', {})
    if set(public) != PUBLIC_HASHES or not all(_digest(x) for x in public.values()):
        raise ValueError('public identity hashes required')
    if set(schema) != {'database', 'migration', 'evidence_sha256'} or (schema.get('database'), schema.get('migration')) != ('realtime_db', '000001_admission') or not _digest(schema.get('evidence_sha256')):
        raise ValueError('schema evidence required')
    if not _digest(assets_receipts.get('transport_evidence_sha256')):
        raise ValueError('transport evidence required')
    env = [
        _ref('DATABASE_URL', 'secret', 'voice-app-secrets', 'REALTIME_DATABASE_URL'),
        _env('REALTIME_PRINCIPAL_SIGNING_KEYS_DIR', '/etc/voice/p3/keys'),
        _env('REALTIME_PRINCIPAL_ACTIVE_KID', 'current'),
        _env('REALTIME_PRINCIPAL_JWKS_FILE', '/etc/voice/p3/public/realtime.json'),
        _env('REALTIME_CHAT_LIFECYCLE_ADDR', 'voice-chat:9099'),
        _env('REALTIME_CHAT_LIFECYCLE_SERVER_NAME', 'voice-chat.voice-dev.svc'),
        _env('REALTIME_CHAT_LIFECYCLE_CA_FILE', '/etc/voice/p3/trust/chat-server-ca.crt'),
        _env('REALTIME_CHAT_LIFECYCLE_CERT_FILE', '/etc/voice/p3/tls/tls.crt'),
        _env('REALTIME_CHAT_LIFECYCLE_KEY_FILE', '/etc/voice/p3/tls/tls.key'),
        _env('REALTIME_SPACE_LIFECYCLE_GRPC_LISTEN', ':9091'),
        _env('REALTIME_SPACE_LIFECYCLE_TLS_CERT_FILE', '/etc/voice/p3/tls/tls.crt'),
        _env('REALTIME_SPACE_LIFECYCLE_TLS_KEY_FILE', '/etc/voice/p3/tls/tls.key'),
        _env('REALTIME_SPACE_LIFECYCLE_CLIENT_CA_FILE', '/etc/voice/p3/trust/space-client-ca.crt'),
        _env('REALTIME_SPACE_PRINCIPAL_JWKS_URL', 'https://voice-space.voice-dev.svc:8443/.well-known/jwks.json'),
        _env('REALTIME_SPACE_PRINCIPAL_JWKS_CA_FILE', '/etc/voice/p3/trust/space-jwks-ca.crt'),
        _ref('REALTIME_SPACE_PRINCIPAL_REPLAY_REDIS_ADDR', 'configMap', 'voice-app-config', 'REDIS_ADDR'),
        _ref('REALTIME_SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD', 'secret', 'voice-p3-principal-replay', 'password'),
    ]
    mounts = [{'name': n, 'mountPath': '/etc/voice/p3/' + p, 'readOnly': True} for n, p in [('p3-signing', 'keys'), ('p3-public', 'public'), ('p3-server', 'tls'), ('p3-trust', 'trust')]]
    def items(keys):
        return [{'key': key, 'path': key} for key in keys]
    volumes = [
        {'name': 'p3-signing', 'secret': {'secretName': 'voice-realtime-principal-signing', 'defaultMode': 288, 'items': items(['current.pem', 'next.pem'])}},
        {'name': 'p3-public', 'configMap': {'name': 'voice-p3-principal-public', 'items': items(['realtime.json'])}},
        {'name': 'p3-server', 'secret': {'secretName': 'voice-realtime-p3-tls', 'defaultMode': 288}},
        {'name': 'p3-trust', 'secret': {'secretName': 'voice-p3-principal-trust', 'defaultMode': 288, 'items': items(['chat-server-ca.crt', 'space-client-ca.crt', 'space-jwks-ca.crt'])}},
    ]
    original = original_deployment['spec']
    service = assets_receipts['service']
    if not isinstance(service, dict) or set(service) != {'name', 'namespace', 'uid', 'resourceVersion', 'selector', 'ports', 'type', 'clusterIP'} or not all(isinstance(service.get(k), str) and service[k] for k in ('uid', 'resourceVersion')) or (service.get('name'), service.get('namespace')) != ('voice-realtime', 'voice-dev'):
        raise ValueError('Service identity required')
    if service['type'] != 'ClusterIP' or not isinstance(service['clusterIP'], str) or service['clusterIP'] in ('', 'None'):
        raise ValueError('routable ClusterIP Service required')
    selector = service['selector']
    labels = original['template']['metadata']['labels']
    if not isinstance(selector, dict) or not selector or any(labels.get(k) != v for k, v in selector.items()):
        raise ValueError('Service selector does not select realtime')
    ports = service['ports']
    if not isinstance(ports, list) or sum(p.get('name') == 'realtime-lifecycle' and p.get('port') == 9091 and p.get('targetPort', 9091) == 9091 and p.get('protocol', 'TCP') == 'TCP' for p in ports if isinstance(p, dict)) != 1:
        raise ValueError('Service lifecycle port missing')
    target = copy.deepcopy(original)
    if target.get('replicas', 1) != 1:
        raise ValueError('realtime requires one replica')
    target['strategy'] = {'type': 'Recreate'}
    pod = target['template']['spec']
    containers = [c for c in pod['containers'] if c.get('name') == 'realtime']
    if len(containers) != 1:
        raise ValueError('exactly one realtime container required')
    container = containers[0]
    forbidden = {'S2S_SIGNING_KEY_PEM', 'S2S_SIGNING_KID', 'REALTIME_SPACE_PRINCIPAL_JWKS_CLIENT_CERT_FILE', 'REALTIME_SPACE_PRINCIPAL_JWKS_CLIENT_KEY_FILE'}
    for source in container.get('envFrom', []):
        if source != {'configMapRef': {'name': 'voice-app-config'}}:
            raise ValueError('unknown envFrom source')
        if set(assets['voice-app-config']['keys']) & ({entry['name'] for entry in env} | forbidden):
            raise ValueError('envFrom conflicts with lifecycle configuration')
    if {entry.get('name') for entry in container.get('env', [])} & forbidden:
        raise ValueError('unsupported principal transport configuration')
    existing_paths = [m.get('mountPath', '') for m in container.get('volumeMounts', [])]
    if any(a == b or a.startswith(b + '/') or b.startswith(a + '/') for a in existing_paths for b in [m['mountPath'] for m in mounts]):
        raise ValueError('mount path collision')
    for field, additions in [('env', env), ('volumeMounts', mounts)]:
        current = container.setdefault(field, [])
        if len({entry['name'] for entry in current}) != len(current) or {entry['name'] for entry in current} & {entry['name'] for entry in additions}:
            raise ValueError('container configuration collision')
        current.extend(additions)
    current_volumes = pod.setdefault('volumes', [])
    if len({v['name'] for v in current_volumes}) != len(current_volumes) or {v['name'] for v in current_volumes} & {v['name'] for v in volumes}:
        raise ValueError('volume collision')
    current_volumes.extend(volumes)
    security = pod.setdefault('securityContext', {})
    if 'fsGroup' in security and security['fsGroup'] != 65532:
        raise ValueError('incompatible fsGroup')
    security['fsGroup'] = 65532
    container['image'] = admitted_image
    digest = hashlib.sha256(json.dumps(assets_receipts, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
    return ([{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
             {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
             {'op': 'test', 'path': '/spec', 'value': copy.deepcopy(original)},
             {'op': 'replace', 'path': '/spec', 'value': target}], target, digest)
