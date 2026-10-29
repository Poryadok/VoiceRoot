"""Capture three source-bound DEV public PEM inputs under a prepare claim.

The caller supplies a real reference authority and claim guard. No Secret is
listed or emitted; only allowlisted public keys enter process memory. This is
an observation, not deployment, runtime or durable admission authority.
The caller owns a fresh private single-writer directory. On failure any
partial public files remain quarantined: never consume them without a receipt.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess

import full_cohort_schema_metadata as common
from publish_candidate import source_snapshot

PREFIX = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
          '--namespace', 'voice-dev', '--request-timeout=15s']
PUBLIC = {'voice-principal-ca': ('ca.crt', 'principal-ca.pem'),
          'voice-media-ca': ('ca.crt', 'media-ca.pem'),
          'voice-space-media-server-tls': ('tls.crt', 'space-jwks.pem')}
CERT_PATH = '/etc/voice/media/server/tls.crt'
# Exact reviewed Space Deployment document, not the surrounding multi-service file.
SPACE_DOCUMENT_SHA = '13ba97e8c2249127f3cb06ea4d3e552b11657a17d6e32defa433380ca296cf18'
SOURCE_LINES = (
    b'{name: SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE, value: "/etc/voice/media/server/tls.crt"}',
    b'{name: SPACE_PRINCIPAL_JWKS_HTTPS_LISTEN, value: ":8443"}',
    b'{name: media-server, mountPath: /etc/voice/media/server, readOnly: true}',
    b'secret: {secretName: voice-space-media-server-tls, defaultMode: 0440, items: [{key: tls.crt, path: tls.crt}, {key: tls.key, path: tls.key}]}',
)

# Only these fixed codes may leave capture; never include exception text.
KNOWN_HOLDS = frozenset({'source_identity_invalid','source_authority_failed','space_source_contract_unknown',
    'space_identity_invalid','space_identity_changed','space_mount_invalid','space_configuration_unknown',
    'space_service_invalid','public_input_identity','public_input_missing','public_input_changed',
    'public_certificate_invalid','public_crypto_invalid','public_ca_invalid','space_certificate_invalid',
    'source_snapshot_changed','destination_invalid','destination_changed','public_write_failed'})
STAGE_HOLDS = frozenset({'p3_stage_source','p3_stage_workload','p3_stage_pod','p3_stage_mount',
    'p3_stage_service','p3_stage_public_identity','p3_stage_public_value','p3_stage_public_pem',
    'p3_stage_config','p3_stage_recheck','p3_stage_destination','p3_stage_write','p3_stage_crypto',
    'p3_stage_postcheck'})


def _source_contract(root, snapshot):
    path = Path(root) / 'deploy/staging/services.yaml'
    if path.is_symlink() or not path.is_file(): raise common.Hold('space_source_contract_unknown')
    data = path.read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    if len(data) > 1048576 or digest != snapshot.get('files', {}).get('deploy/staging/services.yaml'):
        raise common.Hold('space_source_contract_unknown')
    documents = re.split(rb'(?m)^---[ \t]*\r?$', data)
    space = [doc for doc in documents if re.search(rb'(?m)^kind: Deployment\r?$', doc)
             and re.search(rb'(?m)^metadata:\r?\n  name: voice-space\r?$', doc)]
    if len(space) != 1:
        raise common.Hold('space_source_contract_unknown')
    doc = space[0]
    if hashlib.sha256(doc).hexdigest() != SPACE_DOCUMENT_SHA:
        raise common.Hold('space_source_contract_unknown')
    for line in (b'spec:', b'  template:', b'    spec:', b'      containers:', b'      volumes:'):
        if len(re.findall(rb'(?m)^' + re.escape(line) + rb'\r?$', doc)) != 1:
            raise common.Hold('space_source_contract_unknown')
    section = re.search(rb'(?ms)^      containers:\r?\n(.*?)^      volumes:\r?$', doc)
    if section is None: raise common.Hold('space_source_contract_unknown')
    containers = re.split(rb'(?m)^        - name: ', section.group(1))
    selected = [part for part in containers[1:] if re.match(rb'space\r?\n', part)]
    if len(selected) != 1: raise common.Hold('space_source_contract_unknown')
    container = selected[0]
    def field(name):
        pattern = rb'(?ms)^          ' + name + rb':\r?\n(.*?)(?=^          [A-Za-z][^\r\n]*:|\Z)'
        matches = re.findall(pattern, container)
        if len(matches) != 1: raise common.Hold('space_source_contract_unknown')
        return matches[0]
    env, mounts = field(b'env'), field(b'volumeMounts')
    for segment, line in ((env, SOURCE_LINES[0]), (env, SOURCE_LINES[1]), (mounts, SOURCE_LINES[2])):
        if len(re.findall(rb'(?m)^            - ' + re.escape(line) + rb'\r?$', segment)) != 1:
            raise common.Hold('space_source_contract_unknown')
    volume = rb'(?m)^        - name: media-server\r?\n          ' + re.escape(SOURCE_LINES[3]) + rb'\r?$'
    if len(re.findall(volume, doc[section.end():])) != 1:
        raise common.Hold('space_source_contract_unknown')
    return digest


def _call(run, guard, *args):
    common._fence(guard)
    raw = common._external(run, PREFIX + list(args))
    common._fence(guard)
    if not isinstance(raw, bytes) or len(raw) > 65536: raise common.Hold('public_input_transport')
    return raw


def _get(run, guard, kind, name):
    return json.loads(_call(run, guard, 'get', kind, name, '-o', 'json'))


def _meta(run, guard, kind, name):
    raw = _call(run, guard, 'get', kind, name, '-o', 'jsonpath={.metadata.uid}{"\\n"}{.metadata.resourceVersion}')
    parts = raw.decode('utf-8').splitlines()
    if len(parts) != 2 or not all(re.fullmatch(r'[A-Za-z0-9.-]{1,128}', p) for p in parts):
        raise common.Hold('public_input_identity')
    return {'uid': parts[0], 'resource_version': parts[1]}


def _value(run, guard, kind, name, key):
    expression = 'jsonpath={.data.' + key.replace('.', '\\.') + '}'
    raw = _call(run, guard, 'get', kind, name, '-o', expression)
    if kind == 'secret': raw = base64.b64decode(raw, validate=True)
    if not raw or len(raw) > 32768: raise common.Hold('public_input_missing')
    return raw


def _mounted(pod):
    containers = [c for c in pod['spec']['containers'] if c.get('name') == 'space']
    if len(containers) != 1: raise common.Hold('space_mount_invalid')
    container = containers[0]
    for name, value in [('SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE', CERT_PATH),
                        ('SPACE_PRINCIPAL_JWKS_HTTPS_LISTEN', ':8443')]:
        entries = [e for e in container.get('env', []) if e.get('name') == name]
        if len(entries) != 1 or set(entries[0]) != {'name', 'value'} or entries[0]['value'] != value:
            raise common.Hold('space_configuration_unknown')
    mounts = [m for m in container.get('volumeMounts', []) if CERT_PATH == m.get('mountPath') or
              CERT_PATH.startswith(m.get('mountPath', '').rstrip('/') + '/')]
    if len(mounts) != 1: raise common.Hold('space_mount_invalid')
    mount = mounts[0]
    if (mount.get('mountPath') != '/etc/voice/media/server' or mount.get('readOnly') is not True
            or 'subPath' in mount or 'subPathExpr' in mount): raise common.Hold('space_mount_invalid')
    volumes = [v for v in pod['spec']['volumes'] if v.get('name') == mount.get('name')]
    if len(volumes) != 1: raise common.Hold('space_mount_invalid')
    secret = volumes[0].get('secret', {})
    if secret.get('secretName') != 'voice-space-media-server-tls': raise common.Hold('space_mount_invalid')
    if sum(item.get('key') == 'tls.crt' and item.get('path') == 'tls.crt' for item in secret.get('items', [])) != 1:
        raise common.Hold('space_mount_invalid')


def _service(service, pod):
    meta, spec = service['metadata'], service['spec']
    if meta.get('name') != 'voice-space' or meta.get('namespace') != 'voice-dev': raise common.Hold('space_service_invalid')
    if not all(isinstance(meta.get(key), str) and meta[key] for key in ('uid', 'resourceVersion')):
        raise common.Hold('space_service_invalid')
    selector = spec.get('selector')
    if (spec.get('type', 'ClusterIP') != 'ClusterIP' or not spec.get('clusterIP') or spec.get('clusterIP') == 'None'
            or not isinstance(selector, dict) or not selector or
            any(pod['metadata'].get('labels', {}).get(k) != v for k, v in selector.items()) or
            sum(p.get('port') == 8443 and p.get('targetPort', 8443) == 8443 and p.get('protocol', 'TCP') == 'TCP' for p in spec.get('ports', [])) != 1):
        raise common.Hold('space_service_invalid')
    return {'uid': meta['uid'], 'resource_version': meta['resourceVersion']}


def _openssl(*args, input_bytes=None):
    result = subprocess.run(['openssl', *map(str, args)], input=input_bytes, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=15)
    if result.returncode or len(result.stdout) > 65536: raise common.Hold('public_crypto_invalid')
    return result.stdout


def _public_pem(raw):
    match = re.fullmatch(rb'\s*-----BEGIN CERTIFICATE-----\s+([A-Za-z0-9+/=\s]+)-----END CERTIFICATE-----\s*', raw)
    if match is None:
        raise common.Hold('public_certificate_invalid')
    der = base64.b64decode(b''.join(match.group(1).split()), validate=True)
    if _openssl('x509', '-inform', 'DER', '-outform', 'DER', input_bytes=der) != der:
        raise common.Hold('public_certificate_invalid')


def _verify(paths):
    for path in paths.values():
        _openssl('x509', '-in', path, '-noout', '-checkend', '86400')
    for name in ('principal-ca.pem', 'media-ca.pem'):
        if b'CA:TRUE' not in _openssl('x509', '-in', paths[name], '-noout', '-ext', 'basicConstraints'):
            raise common.Hold('public_ca_invalid')
    _openssl('verify', '-CAfile', paths['media-ca.pem'], '-purpose', 'sslserver', '-verify_hostname',
             'voice-space.voice-dev.svc', paths['space-jwks.pem'])
    if b'CA:FALSE' not in _openssl('x509', '-in', paths['space-jwks.pem'], '-noout', '-ext', 'basicConstraints'):
        raise common.Hold('space_certificate_invalid')
    if b'TLS Web Server Authentication' not in _openssl('x509', '-in', paths['space-jwks.pem'], '-noout', '-ext', 'extendedKeyUsage'):
        raise common.Hold('space_certificate_invalid')
    sans = _openssl('x509', '-in', paths['space-jwks.pem'], '-noout', '-ext', 'subjectAltName')
    dns = re.findall(rb'DNS:([A-Za-z0-9.-]+)', sans)
    if b'voice-space.voice-dev.svc' not in dns: raise common.Hold('space_certificate_invalid')
    return {name: hashlib.sha256(_openssl('x509', '-in', path, '-outform', 'DER')).hexdigest()
            for name, path in paths.items()}


def capture(source_root, source_sha, source_authority, space_pod, namespace_uid, claim_guard, destination,
            run_bytes=common.linux_private_bytes):
    """One prepare-only capture. The caller owns claim and source authority."""
    paths, filefds, dirfd = {}, {}, None
    stage = 'p3_stage_source'
    try:
        if not re.fullmatch(r'[a-f0-9]{40}', source_sha) or not namespace_uid: raise common.Hold('source_identity_invalid')
        def guard():
            common._fence(claim_guard)
            raw = common._external(run_bytes, ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                    '--request-timeout=15s', 'get', 'namespace', 'voice-dev', '-o', 'json'])
            try:
                if not isinstance(raw, bytes) or len(raw) > 65536: raise ValueError()
                meta = json.loads(raw)['metadata']
                if meta.get('name') != 'voice-dev' or meta.get('uid') != namespace_uid: raise ValueError()
            except Exception: raise common.GuardLost('namespace_guard_failed') from None
            common._fence(claim_guard)
        def authorize():
            if common._external(source_authority, source_sha) is False: raise common.Hold('source_authority_failed')
        common._fence(guard); authorize()
        snapshot = source_snapshot(Path(source_root), source_sha)
        manifest_digest = _source_contract(source_root, snapshot)
        stage = 'p3_stage_workload'
        workload = common.kube_workload_snapshot('space', space_pod, run_bytes, guard)
        before = workload()
        if before['namespace_uid'] != namespace_uid: raise common.Hold('space_identity_invalid')
        stage = 'p3_stage_pod'; pod = _get(run_bytes, guard, 'pod', space_pod)
        if pod['metadata'].get('uid') != before['pod_uid'] or pod['metadata'].get('resourceVersion') != before['pod_rv']:
            raise common.Hold('space_identity_changed')
        stage = 'p3_stage_mount'; _mounted(pod)
        stage = 'p3_stage_service'; service = _service(_get(run_bytes, guard, 'service', 'voice-space'), pod)
        identities, values = {}, {}
        for name, (key, _) in PUBLIC.items():
            stage = 'p3_stage_public_identity'; identities[name] = _meta(run_bytes, guard, 'secret', name)
            stage = 'p3_stage_public_value'; values[name] = _value(run_bytes, guard, 'secret', name, key)
            stage = 'p3_stage_public_pem'; _public_pem(values[name])
        stage = 'p3_stage_config'
        identities['voice-app-config'] = _meta(run_bytes, guard, 'configmap', 'voice-app-config')
        redis = _value(run_bytes, guard, 'configmap', 'voice-app-config', 'REDIS_ADDR')
        def recheck():
            for name in PUBLIC:
                if _meta(run_bytes, guard, 'secret', name) != identities[name]: raise common.Hold('public_input_changed')
            if _meta(run_bytes, guard, 'configmap', 'voice-app-config') != identities['voice-app-config']:
                raise common.Hold('public_input_changed')
            if workload() != before or _service(_get(run_bytes, guard, 'service', 'voice-space'), pod) != service:
                raise common.Hold('space_identity_changed')
            authorize()
            if source_snapshot(Path(source_root), source_sha) != snapshot: raise common.Hold('source_snapshot_changed')
            common._fence(guard)
        stage = 'p3_stage_recheck'; recheck()
        stage = 'p3_stage_destination'
        directory = Path(destination)
        if not directory.is_absolute() or directory.resolve() != directory: raise common.Hold('destination_invalid')
        dirfd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        info = os.fstat(dirfd)
        if info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != 0o700 or os.listdir(dirfd):
            raise common.Hold('destination_invalid')
        stage = 'p3_stage_write'
        for name, (_, filename) in PUBLIC.items():
            fd = os.open(filename, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=dirfd)
            filefds[filename] = fd
            paths[filename] = name
            remaining = memoryview(values[name])
            while remaining:
                count = os.write(fd, remaining)
                if count <= 0: raise common.Hold('public_write_failed')
                remaining = remaining[count:]
            os.fsync(fd)
        procpaths = {name: Path('/proc') / str(os.getpid()) / 'fd' / str(fd) for name, fd in filefds.items()}
        stage = 'p3_stage_crypto'; hashes = _verify(procpaths)
        os.fsync(dirfd)
        stage = 'p3_stage_postcheck'; recheck()
        current_dir = os.stat(directory, follow_symlinks=False)
        if (current_dir.st_dev, current_dir.st_ino) != (info.st_dev, info.st_ino): raise common.Hold('destination_changed')
        for filename, fd in filefds.items():
            current, held = os.stat(filename, dir_fd=dirfd, follow_symlinks=False), os.fstat(fd)
            if ((current.st_dev, current.st_ino) != (held.st_dev, held.st_ino) or held.st_nlink != 1
                    or stat.S_IMODE(held.st_mode) != 0o600 or held.st_uid != os.geteuid()
                    or os.pread(fd, 32769, 0) != values[paths[filename]]):
                raise common.Hold('destination_changed')
        common._fence(guard)
        return {'status': 'observed', 'source_sha': source_sha, 'namespace_uid': namespace_uid,
                'source_manifest_sha256': manifest_digest,
                'space_identity': {k: before[k] for k in ('pod_uid', 'pod_rv', 'deployment_uid', 'deployment_rv', 'container_id', 'restart_count')},
                'space_service_spec': service, 'objects': identities, 'public_der_sha256': hashes,
                'redis_endpoint': 'expected' if redis == b'voice-redis:6379' else 'other',
                'admission_qualified': False,
                'unobserved': ['projected_mount_sync', 'service_endpoint_routing', 'runtime_peer_transport', 'redis_auth', 'schema_job', 'rollout']}
    except common.GuardLost:
        raise
    except common.Hold as error:
        if str(error) in KNOWN_HOLDS: raise
        raise common.Hold(stage) from None
    except Exception:
        raise common.Hold(stage) from None
    finally:
        if dirfd is not None:
            os.close(dirfd)
        for fd in filefds.values(): os.close(fd)
