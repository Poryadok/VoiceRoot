"""Private DEV credential union checks. No API calls and no private diagnostics.

Reference completeness is a structural prerequisite, never a PKI/reset verdict.
"""
import base64
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import secrets
import stat
import sys
import tempfile
from urllib.parse import quote
import credentials as canonical

NS = 'voice-dev'


# Exact current DEV transport families. Separate roots preserve the dedicated
# CompleteMatch client boundary; public HTTPS roots never enter inbound trust.
TLS_FAMILIES = {
    **{'voice-' + s + '-principal-tls': ('principal', 'voice-' + s, 'serverAuth,clientAuth')
       for s in ('social', 'user', 'space', 'file', 'search', 'notification')},
    'voice-user-file-principal-tls': ('principal', 'voice-user', 'serverAuth,clientAuth'),
    **{'voice-' + s + '-media-server-tls': ('media', 'voice-' + s, 'serverAuth')
       for s in ('auth', 'role', 'space', 'voice')},
    **{'voice-' + s + '-media-client-tls': ('media', 'voice-' + s, 'clientAuth')
       for s in ('role', 'space', 'chat', 'voice', 'gateway')},
    'voice-chat-media-jwks-tls': ('media', 'voice-chat', 'serverAuth'),
    'voice-voice-media-jwks-tls': ('media', 'voice-voice', 'serverAuth'),
    'voice-gameintegration-jwks-tls': ('media', 'voice-gameintegration', 'serverAuth'),
    'voice-realtime-p3-tls': ('principal', 'voice-realtime.voice-dev.svc', 'serverAuth,clientAuth'),
    'voice-chat-realtime-p3-tls': ('principal', 'voice-chat.voice-dev.svc', 'serverAuth'),
    'voice-p3-principal-jwks-tls': ('principal', 'voice-p3-principal-jwks.voice-dev.svc', 'serverAuth'),
    'voice-chat-match-squad-server-tls': ('match-squad', 'voice-chat', 'serverAuth'),
    'voice-voice-match-squad-server-tls': ('match-squad', 'voice-voice', 'serverAuth'),
    'voice-matchmaking-client-tls': ('match-squad', 'voice-matchmaking', 'clientAuth'),
    'voice-matchmaking-principal-jwks-tls': ('match-squad', 'voice-matchmaking.voice-dev.svc', 'serverAuth'),
    'voice-matchmaking-complete-server-tls': ('match-squad', 'voice-matchmaking.voice-dev.svc', 'serverAuth'),
    'voice-gateway-matchmaking-complete-client-tls': ('matchmaking-complete-client', 'voice-gateway', 'clientAuth'),
}
SIGNERS = ('social', 'file', 'search', 'gateway', 'role', 'space', 'chat',
           'notification', 'matchmaking', 'realtime', 'voice', 'gameintegration')
REPLAY_NAMES = ('voice-p3-principal-replay', 'voice-match-squad-replay', 'voice-match-squad-member-replay')


def public_config(name, data, immutable=False):
    return {'apiVersion': 'v1', 'kind': 'ConfigMap',
            'metadata': {'name': name, 'namespace': NS}, 'data': data,
            **({'immutable': True} if immutable else {})}


class ResetMaterial(canonical.Material):
    def tls(self, name, ca, hostname, purpose):
        # Existing consumers use both service-short and namespace DNS names.
        # Use the canonical Material recipe with explicit SAN alternatives.
        key = self.key(name)
        csr, cert, ext = [self.root / (name + suffix) for suffix in ('.csr', '.crt', '.ext')]
        canonical.run('openssl', 'req', '-new', '-key', key, '-out', csr, '-subj', '/CN=' + hostname)
        names = [hostname]
        if '.' not in hostname:
            names.append(hostname + '.voice-dev.svc')
        canonical.write_private(ext, 'basicConstraints=critical,CA:FALSE\n'
                                'keyUsage=critical,digitalSignature,keyEncipherment\n'
                                'extendedKeyUsage=' + purpose + '\nsubjectAltName=' +
                                ','.join('DNS:' + host for host in names) + '\n')
        canonical.run('openssl', 'x509', '-req', '-in', csr, '-CA', ca[1], '-CAkey', ca[0],
                      '-set_serial', '0x' + secrets.token_hex(16), '-days', '90', '-extfile', ext, '-out', cert)
        self.add(name, {'tls.crt': cert.read_bytes(), 'tls.key': key.read_bytes()})
        csr.unlink()
        ext.unlink()


def issue_identities(material, public_roots):
    """Replace whole connected families using existing DEV Material primitives.

    Does not apply objects or settle old P3 admissions. Caller must keep old
    storage quarantined under the authorized whole-DEV fresh-dataset reset.
    """
    roots = {family: material.ca(family) for family in
             ('principal', 'media', 'match-squad', 'matchmaking-complete-client')}
    certs = {family: pair[1].read_bytes() for family, pair in roots.items()}
    material.add('voice-principal-ca', {'ca.crt': certs['principal']})
    material.add('voice-media-ca', {'ca.crt': certs['media'],
                                  'jwks-ca.crt': public_roots + b'\n' + certs['principal'] + b'\n' + certs['media']})
    for service in SIGNERS:
        material.signer('voice-' + service + '-principal-signing',
                        active=service not in ('realtime', 'voice', 'gameintegration'))
    for name, (family, host, purpose) in TLS_FAMILIES.items():
        material.tls(name, roots[family], host, purpose)
    p3_trust = {key: certs['principal'] for key in (
        'chat-server-ca.crt', 'space-client-ca.crt', 'space-jwks-ca.crt',
        'realtime-client-ca.crt', 'realtime-jwks-ca.crt')}
    # Space publishes its JWKS with media-server, independently of its
    # principal transport leaf and private signer mount.
    p3_trust['space-jwks-ca.crt'] = certs['media']
    material.add('voice-p3-principal-trust', p3_trust)
    material.add('voice-matchmaking-principal-jwks-ca', {'ca.crt': certs['match-squad']})
    for name in REPLAY_NAMES:
        material.add(name, {'password': ''})
    material.items.extend([
        public_config('voice-match-squad-ca', {'ca.crt': certs['match-squad'].decode()}),
        public_config('voice-matchmaking-complete-client-ca', {'ca.crt': certs['matchmaking-complete-client'].decode()}),
    ])
    return roots


def fail(code):
    raise ValueError('dev_credentials_' + code)


def _directory_info(path):
    return path.lstat()


def private_destination(destination, work_dir):
    """Reset-only caller-owned private workspace; canonical bootstrap unchanged.

    User's fresh DEV mandate and the reset adapter's private --work-dir permit
    an explicitly selected 0700 subdirectory, including one owned by pmd.
    Its parent shared home is never itself an output directory.
    """
    if sys.platform != 'linux' or os.geteuid() == 0:
        fail('nonroot_linux')
    destination, work_dir = Path(destination), Path(work_dir)
    if (not destination.is_absolute() or not work_dir.is_absolute()
            or destination.parent != work_dir or destination == work_dir
            or work_dir.resolve() != work_dir or work_dir.is_symlink()
            or destination.is_symlink() or destination.exists()):
        fail('destination')
    info = _directory_info(work_dir)
    if not stat.S_ISDIR(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o700 or info.st_uid != os.geteuid():
        fail('private_workdir')


def _openssl(*args):
    try:
        result = subprocess.run(['openssl', *map(str, args)], capture_output=True, timeout=60)
    except (OSError, subprocess.SubprocessError):
        fail('certificate_tool_unavailable')
    if result.returncode:
        fail('certificate_verification_failed')
    return result.stdout


def signer_jwks(signer):
    """Canonical export-jwks.py recipe, without its absolute-volume effects."""
    data = values(signer)
    realtime = signer['metadata']['name'] == 'voice-realtime-principal-signing'
    expected = {'current.pem', 'next.pem'}
    if realtime and set(data) != expected:
        fail('realtime_signer_keys')
    if not expected <= set(data) or (not realtime and data.get('active-kid', b'current') not in (b'current', b'next')):
        fail('signer_keys')
    result = []
    with tempfile.TemporaryDirectory(prefix='voice-dev-signer-') as scratch:
        root = Path(scratch)
        root.chmod(0o700)
        for kid in ('current', 'next'):
            raw = data[kid + '.pem']
            if not raw.splitlines() or raw.splitlines()[0] != b'-----BEGIN PRIVATE KEY-----' or raw.count(b'-----BEGIN ') != 1:
                fail('signer_pkcs8')
            path = root / (kid + '.pem')
            fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
            with os.fdopen(fd, 'wb') as stream:
                stream.write(raw)
            _openssl('pkey', '-in', path, '-check', '-noout')
            modulus = _openssl('rsa', '-in', path, '-noout', '-modulus').decode().strip().removeprefix('Modulus=')
            try:
                number = int(modulus, 16)
                if number.bit_length() < 2048:
                    fail('signer_bits')
                text = _openssl('pkey', '-in', path, '-pubout', '-text_pub', '-noout').decode()
                exponent = int(re.search(r'^Exponent: ([0-9]+)', text, re.MULTILINE)[1])
                encode = lambda value: base64.urlsafe_b64encode(value.to_bytes((value.bit_length() + 7) // 8, 'big')).decode().rstrip('=')
                result.append(dict(kty='RSA', kid=kid, use='sig', alg='RS256', n=encode(number), e=encode(exponent)))
            except (ValueError, TypeError, AttributeError):
                fail('signer_rsa')
    if result[0]['n'] == result[1]['n']:
        fail('duplicate_rotation')
    return {'keys': result}


def verify_p3_public(signer, public):
    objects_by_identity([signer, public])
    if (public['kind'] != 'ConfigMap' or public['metadata']['name'] != 'voice-p3-principal-public'
            or public.get('immutable') is not True or set(values(public)) != {'realtime.json'}):
        fail('p3_public_contract')
    try:
        published = json.loads(values(public)['realtime.json'])
    except (ValueError, TypeError):
        fail('p3_public_encoding')
    if published != signer_jwks(signer):
        fail('jwks_generation')


def verify_replay(prepared, objects):
    index = objects_by_identity(objects)
    redis = [o for o in prepared if o.get('kind') in ('Deployment', 'StatefulSet')
             and o.get('metadata', {}).get('name') == 'voice-redis']
    if len(redis) != 1:
        fail('redis_contract')
    pod = redis[0]['spec']['template']['spec']
    containers = pod.get('containers', [])
    if (len(containers) != 1 or containers[0].get('command')
            or containers[0].get('args') != ['redis-server', '--appendonly', 'yes']):
        fail('redis_contract')
    for name in REPLAY_NAMES:
        obj = index.get(('Secret', name))
        if obj is None or values(obj) != {'password': b''}:
            fail('replay_password')
    # Inspect effective values, including envFrom and individual key refs.
    for obj in prepared:
        if obj.get('kind') not in ('Deployment', 'StatefulSet', 'Job'):
            continue
        pod = obj['spec']['template']['spec']
        for container in pod.get('containers', []) + pod.get('initContainers', []):
            env = effective_environment(container, index)
            for name, value in env.items():
                if (name.endswith('_REPLAY_REDIS_PASSWORD') or name == 'REDIS_PASSWORD') and value != '':
                    fail('replay_password')
                if (name.endswith('_REPLAY_REDIS_ADDR') or name == 'REDIS_ADDR') and value != 'voice-redis:6379':
                    fail('replay_endpoint')


def verify_identities(objects):
    """Check the entire newly issued graph; no isolated retained CA/leaf mixing."""
    index = objects_by_identity(objects)
    def data(kind, name):
        if (kind, name) not in index:
            fail('missing_object:' + kind + '/' + name)
        return values(index[kind, name])
    roots = {
        'principal': data('Secret', 'voice-principal-ca')['ca.crt'],
        'media': data('Secret', 'voice-media-ca')['ca.crt'],
        'match-squad': data('ConfigMap', 'voice-match-squad-ca')['ca.crt'],
        'matchmaking-complete-client': data('ConfigMap', 'voice-matchmaking-complete-client-ca')['ca.crt'],
    }
    if len(set(roots.values())) != len(roots):
        fail('trust_separation')
    p3 = data('Secret', 'voice-p3-principal-trust')
    expected = {'chat-server-ca.crt', 'space-client-ca.crt', 'space-jwks-ca.crt',
                'realtime-client-ca.crt', 'realtime-jwks-ca.crt'}
    if set(p3) != expected or any(value != roots['media' if key == 'space-jwks-ca.crt' else 'principal']
                                   for key, value in p3.items()):
        fail('p3_trust_generation')
    if data('Secret', 'voice-matchmaking-principal-jwks-ca') != {'ca.crt': roots['match-squad']}:
        fail('matchmaking_trust_generation')
    jwks_ca = data('Secret', 'voice-media-ca')['jwks-ca.crt']
    if any(cert not in jwks_ca for cert in (roots['principal'], roots['media'])) or b'PRIVATE KEY' in jwks_ca:
        fail('media_jwks_trust')
    receipts = {}
    for name, (family, host, purposes) in TLS_FAMILIES.items():
        leaf = data('Secret', name)
        if set(leaf) != {'tls.crt', 'tls.key'}:
            fail('tls_keys')
        for purpose in purposes.split(','):
            receipts[name + '/' + purpose] = verify_certificate(leaf['tls.crt'], leaf['tls.key'], roots[family], host, purpose)
    for service in SIGNERS:
        signer_jwks(index['Secret', 'voice-' + service + '-principal-signing'])
    verify_p3_public(index['Secret', 'voice-realtime-principal-signing'], index['ConfigMap', 'voice-p3-principal-public'])
    return receipts


def verify_certificate(cert, key, ca, hostname, purpose):
    """Canonical issuer chain/purpose/SAN/key checks plus signal-TLS 24h margin.

    The caller must supply the EXACT final consumer trust bundle and DNS name;
    this helper does not select a CA, mint identities, or qualify reset state.
    """
    if purpose not in ('serverAuth', 'clientAuth') or not re.fullmatch(r'[a-zA-Z0-9.-]+', hostname):
        fail('certificate_contract')
    with tempfile.TemporaryDirectory(prefix='voice-dev-cert-') as scratch:
        root = Path(scratch)
        root.chmod(0o700)
        for name, value in (('tls.crt', cert), ('tls.key', key), ('ca.crt', ca)):
            fd = os.open(root / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(fd, 'wb') as stream:
                stream.write(value)
        _openssl('verify', '-CAfile', root / 'ca.crt', '-purpose', 'sslserver' if purpose == 'serverAuth' else 'sslclient',
                 '-verify_hostname', hostname, root / 'tls.crt')
        _openssl('x509', '-in', root / 'tls.crt', '-noout', '-checkend', '86400')
        sans = _openssl('x509', '-in', root / 'tls.crt', '-noout', '-ext', 'subjectAltName').decode()
        if hostname not in re.findall(r'DNS:([A-Za-z0-9.-]+)', sans):
            fail('certificate_dns_san')
        eku = _openssl('x509', '-in', root / 'tls.crt', '-noout', '-ext', 'extendedKeyUsage')
        expected_eku = b'TLS Web Server Authentication' if purpose == 'serverAuth' else b'TLS Web Client Authentication'
        if expected_eku not in eku:
            fail('certificate_eku')
        constraints = _openssl('x509', '-in', root / 'tls.crt', '-noout', '-ext', 'basicConstraints')
        if b'CA:FALSE' not in constraints or b'CA:TRUE' in constraints:
            fail('certificate_leaf_constraints')
        cert_public = _openssl('x509', '-in', root / 'tls.crt', '-pubkey', '-noout')
        key_public = _openssl('pkey', '-in', root / 'tls.key', '-pubout')
        if cert_public != key_public:
            fail('certificate_private_key_mismatch')
        return {'certificate_sha256': hashlib.sha256(cert).hexdigest(),
                'public_key_sha256': hashlib.sha256(cert_public).hexdigest()}


def objects_by_identity(objects):
    result = {}
    for obj in objects:
        meta = obj.get('metadata', {})
        identity = (obj.get('kind'), meta.get('name'))
        if (identity[0] not in ('Secret', 'ConfigMap') or obj.get('apiVersion') != 'v1'
                or meta.get('namespace') != NS or not isinstance(identity[1], str)
                or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', identity[1]) or identity in result):
            fail('object_identity')
        result[identity] = obj
    return result


def values(obj):
    result = {}
    try:
        if obj['kind'] == 'Secret':
            result.update({k: base64.b64decode(v, validate=True) for k, v in obj.get('data', {}).items()})
            result.update({k: v.encode() for k, v in obj.get('stringData', {}).items()})
        else:
            result.update({k: v.encode() for k, v in obj.get('data', {}).items()})
            result.update({k: base64.b64decode(v, validate=True) for k, v in obj.get('binaryData', {}).items()})
    except (ValueError, TypeError, AttributeError):
        fail('object_encoding')
    return result


def effective_environment(container, index, loader=None):
    """Resolve Kubernetes envFrom order and explicit env override semantics."""
    def read(kind, name, keys, reason):
        if loader:
            return loader(kind, name, keys, reason)
        if (kind, name) not in index:
            fail('missing_object:' + kind + '/' + str(name))
        result = values(index[kind, name])
        if keys is not None and not set(keys) <= set(result):
            fail('missing_keys:' + kind + '/' + str(name))
        return result
    def decode(raw):
        try:
            return raw.decode('utf-8')
        except (UnicodeError, AttributeError):
            fail('environment_encoding')
    result = {}
    for source in container.get('envFrom', []):
        refs = [(field, kind) for field, kind in (('secretRef', 'Secret'), ('configMapRef', 'ConfigMap')) if field in source]
        if len(refs) != 1:
            fail('environment_source')
        field, kind = refs[0]
        prefix = source.get('prefix', '')
        for key, raw in read(kind, source[field]['name'], None, 'envFrom').items():
            result[prefix + key] = decode(raw)
    seen = set()
    for env in container.get('env', []):
        name = env['name']
        if name in seen:
            fail('environment_duplicate')
        seen.add(name)
        if 'value' in env and 'valueFrom' in env:
            fail('environment_source')
        if 'valueFrom' in env:
            refs = [(field, kind) for field, kind in (('secretKeyRef', 'Secret'), ('configMapKeyRef', 'ConfigMap')) if field in env['valueFrom']]
            if len(refs) != 1:
                # Non-identity downward fields cannot resolve a credential setting.
                result[name] = None
                continue
            field, kind = refs[0]
            ref = env['valueFrom'][field]
            value = decode(read(kind, ref['name'], [ref['key']], 'env/' + name)[ref['key']])
        else:
            value = env.get('value', '')
        if not isinstance(value, str):
            fail('environment_shape')
        result[name] = value
    return result


def complete_application(app):
    """Fill Realtime's omitted DSN and bind Notification to the SAME fresh PG key."""
    objects_by_identity([app])
    if app['kind'] != 'Secret' or app['metadata']['name'] != 'voice-app-secrets':
        fail('application_identity')
    data = values(app)
    try:
        password = data['POSTGRES_PASSWORD'].decode()
        if not password:
            fail('database_password')
    except (KeyError, UnicodeError):
        fail('database_password')
    result = copy.deepcopy(app)
    for service in ('realtime', 'notification'):
        key = service.upper() + '_DATABASE_URL'
        expected = ('postgres://voice:' + quote(password, safe='') + '@voice-postgres:5432/' + service + '_db?sslmode=disable').encode()
        if key in data and data[key] != expected:
            fail('database_generation:' + key)
        result.setdefault('data', {})[key] = base64.b64encode(expected).decode()
        result.get('stringData', {}).pop(key, None)
    return result


def verify_application(objects):
    index = objects_by_identity(objects)
    app = values(index['Secret', 'voice-app-secrets'])
    minio = values(index['Secret', 'voice-minio-credentials'])
    try:
        password = quote(app['POSTGRES_PASSWORD'].decode(), safe='')
        if not password:
            fail('application_database')
        for service in 'social user chat space messaging file role matchmaking search notification bot story moderation subscription gateway voice realtime'.split():
            expected = f'postgres://voice:{password}@voice-postgres:5432/{service}_db?sslmode=disable'.encode()
            if app[service.upper() + '_DATABASE_URL'] != expected:
                fail('application_database')
        game_password = quote(app['GAME_INTEGRATION_DB_PASSWORD'].decode(), safe='')
        if not game_password or app['GAME_INTEGRATION_DATABASE_URL'] != f'postgres://gameintegration_runtime:{game_password}@voice-postgres:5432/game_integration_db?sslmode=disable'.encode():
            fail('application_database')
        ch = quote(app['CLICKHOUSE_PASSWORD'].decode(), safe='')
        if not ch or app['CLICKHOUSE_DSN'] != f'clickhouse://default:{ch}@voice-clickhouse:9000/voice'.encode():
            fail('application_database')
        for service in ('USER', 'FILE'):
            if (app[service + '_R2_ACCESS_KEY_ID'] != minio['MINIO_ROOT_USER']
                    or app[service + '_R2_SECRET_ACCESS_KEY'] != minio['MINIO_ROOT_PASSWORD']):
                fail('application_object_store')
    except (KeyError, UnicodeError):
        fail('application_contract')


def verify_runtime_binding(prepared, secrets, source_inputs):
    """Bind every final consumer to a fresh canonical source-qualified render.

    Identity graph checks alone cannot qualify final URLs, server names, trust
    mounts or effective client identities. Reuse the canonical reset renderer's
    source/archive, image, overlay and policy checks instead of inferring that
    consumer contract from Secret names. Inputs are exactly render()'s keyword
    arguments and must retain the original fresh credential union.
    """
    required = {'root', 'source_sha', 'secret_objects', 'image_pins', 'release_policy', 'decoder'}
    if not isinstance(source_inputs, dict) or not required <= set(source_inputs):
        fail('runtime_source_binding_required')
    import reset_package
    expected = reset_package.render(**source_inputs)
    if prepared != expected['prepared'] or secrets != expected['secrets']:
        fail('runtime_source_mismatch')
    return {key: expected['receipt'][key] for key in ('source_sha', 'renderer_sha256')}


def qualify(prepared, secrets, *, source_inputs=None):
    """Offline checks only. Runtime HTTPS/listeners/migrations remain a gate.

    No qualification result is returned without the canonical final source
    render and identical credentials. This does not qualify live API readback.
    """
    source_binding = verify_runtime_binding(prepared, secrets, source_inputs)
    objects = [o for o in prepared if o.get('kind') == 'ConfigMap'] + secrets
    rows = references(prepared, secrets)
    verify_application(objects)
    verify_replay(prepared, objects)
    certificates = verify_identities(objects)
    return {'schema': 'voice-dev-offline-credential-qualification-v1', 'namespace': NS,
            'references': rows, 'certificate_checks': certificates,
            'source_binding': source_binding, 'runtime_qualified': False}


def build_union(fresh, captured, retained_names):
    """Select complete retained objects explicitly; never merge same-name families."""
    fresh_index = objects_by_identity(fresh)
    captured_index = objects_by_identity(captured)
    if not isinstance(retained_names, (set, frozenset)) or not all(isinstance(n, str) for n in retained_names):
        fail('retained_selection')
    selected = [obj for (_, name), obj in captured_index.items() if name in retained_names]
    if {o['metadata']['name'] for o in selected} != retained_names:
        fail('missing_retained')
    if any(identity[1] in retained_names for identity in fresh_index):
        fail('generation_collision')
    result = copy.deepcopy(fresh + selected)
    for obj in result:
        meta = obj['metadata']
        for key in ('uid', 'resourceVersion', 'generation', 'creationTimestamp', 'managedFields', 'ownerReferences', 'deletionTimestamp', 'deletionGracePeriodSeconds', 'finalizers'):
            meta.pop(key, None)
        meta['annotations'] = {k: v for k, v in meta.get('annotations', {}).items()
                               if not k.startswith(('kubectl.', 'deployment.kubernetes.io/'))}
        obj.pop('status', None)
    return result


def references(prepared, secrets):
    """Return only object/key/consumer metadata; resolve whole-volume runtime files."""
    available = objects_by_identity([o for o in prepared if o.get('kind') == 'ConfigMap'] + secrets)
    rows = {}

    def require(kind, name, keys, consumer, reason):
        identity = (kind, name)
        if identity not in available:
            fail('missing_object:' + kind + '/' + str(name))
        data = values(available[identity])
        selected = set(data) if keys is None else set(keys)
        if not selected or not selected <= set(data):
            fail('missing_keys:' + kind + '/' + str(name))
        row = rows.setdefault(identity, {'kind': kind, 'name': name, 'keys': set(), 'consumers': set()})
        row['keys'].update(selected)
        row['consumers'].add(consumer + ':' + reason)
        return data

    def source_files(source, consumer):
        for field, kind in (('secret', 'Secret'), ('configMap', 'ConfigMap')):
            if field in source:
                ref = source[field]
                name = ref.get('secretName') or ref.get('name')
                items = ref.get('items')
                data = require(kind, name, [i['key'] for i in items] if items is not None else None, consumer, 'volume')
                if items is not None and len({i['path'] for i in items}) != len(items):
                    fail('projection_collision')
                files = {i['path']: (kind, name, i['key']) for i in items} if items is not None else {key: (kind, name, key) for key in data}
                if any(PurePosixPath(path).is_absolute() or '..' in PurePosixPath(path).parts for path in files):
                    fail('projection_path')
                return files
        if 'projected' in source:
            files = {}
            for projection in source['projected'].get('sources', []):
                if not set(projection) <= {'secret', 'configMap'}:
                    # ServiceAccountToken/downwardAPI are not retained identity inputs.
                    continue
                addition = source_files(projection, consumer)
                if set(files) & set(addition):
                    fail('projection_collision')
                files.update(addition)
            return files
        return {}

    for obj in prepared:
        if obj.get('kind') not in ('Deployment', 'StatefulSet', 'Job'):
            continue
        if obj.get('metadata', {}).get('namespace') != NS:
            fail('workload_namespace')
        consumer = obj['kind'] + '/' + obj['metadata']['name']
        pod = obj['spec']['template']['spec']
        volumes = {v['name']: source_files(v, consumer) for v in pod.get('volumes', [])}
        for ref in pod.get('imagePullSecrets', []):
            data = require('Secret', ref['name'], ['.dockerconfigjson'], consumer, 'imagePullSecret')
            try:
                document = json.loads(data['.dockerconfigjson'])
                if not isinstance(document.get('auths'), dict):
                    fail('pull_secret_format')
            except (ValueError, TypeError):
                fail('pull_secret_format')
        for container in pod.get('containers', []) + pod.get('initContainers', []):
            actor = consumer + '/' + container['name']
            mounted = {}
            for mount in container.get('volumeMounts', []):
                files = volumes.get(mount['name'], {})
                if 'subPathExpr' in mount:
                    fail('dynamic_subpath_contract')
                if mount.get('subPath'):
                    if mount['subPath'] not in files:
                        fail('subpath_missing:' + actor)
                    mounted[mount['mountPath']] = files[mount['subPath']]
                else:
                    for path, identity in files.items():
                        mounted[str(PurePosixPath(mount['mountPath']) / path)] = identity
            environment = effective_environment(container, available,
                lambda kind, name, keys, reason: require(kind, name, keys, actor, reason))
            for name, value in environment.items():
                if value is None:
                    if name.endswith(('_FILE', '_DIR', '_DIRECTORY')):
                        fail('dynamic_runtime_file')
                    continue
                paths = re.findall(r'/etc/voice/[A-Za-z0-9_.\-/]+', value)
                for path in paths:
                    is_directory = name.endswith(('_DIR', '_DIRECTORY'))
                    resolved = [p for p in mounted if p.startswith(path.rstrip('/') + '/')] if is_directory else [path] if path in mounted else []
                    if not resolved:
                        fail('runtime_file_unresolved:' + actor + '/' + name)
                    for location in resolved:
                        kind, object_name, key = mounted[location]
                        require(kind, object_name, [key], actor, 'runtime/' + name)
    return [{**row, 'keys': sorted(row['keys']), 'consumers': sorted(row['consumers'])}
            for identity, row in sorted(rows.items())]


def issue(destination, work_dir, issuer, public_roots, java_cacerts, gateway_host, acl, source_sha):
    """Offline fresh DEV generation. Never reads/applies Kubernetes objects."""
    private_destination(destination, work_dir)
    destination = Path(destination)
    if (gateway_host != 'api-dev.comrade.click' or not re.fullmatch('[0-9a-f]{40}', source_sha)):
        fail('source_contract')
    roots = Path(public_roots).read_bytes()
    if b'PRIVATE KEY' in roots or b'BEGIN CERTIFICATE' not in roots:
        fail('public_roots')
    _openssl('crl2pkcs7', '-nocrl', '-certfile', public_roots)
    old_umask = os.umask(0o077)
    try:
        with tempfile.TemporaryDirectory(prefix='.voice-reset-issue-', dir=work_dir) as scratch:
            root = Path(scratch)
            material = ResetMaterial(root)
            families = issue_identities(material, roots)
            material.add('voice-media-jvm-trust', {'truststore.p12': canonical.make_truststore(root, java_cacerts, families['media'][1])})
            material.add('voice-media-resolver', {'token': secrets.token_hex(32)})
            material.add('voice-user-search-projection', {'cursor-hmac-key': secrets.token_hex(32)})
            material.add('voice-gameintegration-workload-credentials', {
                name: base64.b64encode(secrets.token_bytes(32)).decode() for name in (
                    'GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64', 'GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64')})
            minio = {'MINIO_ROOT_USER': 'dev' + secrets.token_hex(12), 'MINIO_ROOT_PASSWORD': secrets.token_hex(32)}
            material.add('voice-minio-credentials', minio)
            app = canonical.application_values(material.key('auth-jwt').read_bytes(), gateway_host)
            for service in ('USER', 'FILE'):
                app[service + '_R2_ACCESS_KEY_ID'] = minio['MINIO_ROOT_USER']
                app[service + '_R2_SECRET_ACCESS_KEY'] = minio['MINIO_ROOT_PASSWORD']
            material.add('voice-app-secrets', app)
            material.items[-1] = complete_application(material.items[-1])
            signer = next(o for o in material.items if o['metadata']['name'] == 'voice-realtime-principal-signing')
            material.items.append(public_config('voice-p3-principal-public',
                {'realtime.json': json.dumps(signer_jwks(signer), separators=(',', ':')) + '\n'}, True))
            nats = material.ca('nats')
            material.tls('nats-input', nats, 'voice-nats', 'serverAuth')
            material.items.pop()
            canonical.run(issuer, '--namespace', NS, '--tls-cert', root / 'nats-input.crt',
                          '--tls-key', root / 'nats-input.key', '--tls-ca', nats[1], acl, root / 'nats-issued')
            issued = json.loads((root / 'nats-issued/secrets.json').read_text())
            canonical.validate_bundle(issued)
            material.items += issued['items']
            objects_by_identity(material.items)
            verify_application(material.items)
            verdict = verify_identities(material.items)
            provenance = {'schema': 'voice-dev-offline-credentials-v1', 'namespace': NS,
                          'source_sha': source_sha, 'runtime_qualified': False,
                          'inputs_sha256': {name: hashlib.sha256(Path(path).read_bytes()).hexdigest()
                              for name, path in (('public_roots', public_roots), ('java_cacerts', java_cacerts),
                                                 ('nats_issuer', issuer), ('nats_acl', acl))},
                          'certificate_checks': verdict}
            canonical.write_private(root / 'credentials.json', json.dumps({'apiVersion': 'v1', 'kind': 'List', 'items': material.items}, separators=(',', ':')) + '\n')
            canonical.write_private(root / 'issuance.json', json.dumps(provenance, separators=(',', ':')) + '\n')
            for path in root.rglob('*'):
                path.chmod(0o700 if path.is_dir() else 0o600)
            private_destination(destination, work_dir)
            root.rename(destination)
    finally:
        os.umask(old_umask)


def main():
    parser = argparse.ArgumentParser(description='Offline DEV credential issuance and final rendered reference checks; never applies.')
    parser.add_argument('--work-dir', required=True, type=Path)
    parser.add_argument('--destination', required=True, type=Path)
    parser.add_argument('--nats-issuer', required=True, type=Path)
    parser.add_argument('--public-roots', required=True, type=Path)
    parser.add_argument('--java-cacerts', required=True, type=Path)
    parser.add_argument('--gateway-host', default='api-dev.comrade.click')
    parser.add_argument('--source-sha', required=True)
    parser.add_argument('--acl', type=Path, default=Path(__file__).resolve().parents[2] / 'deploy/nats/acl-intent.yaml')
    args = parser.parse_args()
    try:
        issue(args.destination, args.work_dir, args.nats_issuer, args.public_roots,
              args.java_cacerts, args.gateway_host, args.acl, args.source_sha)
    except (ValueError, OSError, KeyError, TypeError, subprocess.SubprocessError):
        print('Offline DEV issuance failed; private diagnostics suppressed', file=sys.stderr)
        return 1
    print('Offline DEV credential generation verified; final render and runtime qualification still required')
    return 0


if __name__ == '__main__':
    sys.exit(main())
