#!/usr/bin/env python3
"""Create-only, same-material DEV MatchSquad credential provisioning."""
import argparse
import base64
import copy
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import sys

NAMESPACE = 'voice-dev'
LEGACY_RECOVERY_OPERATION = 'msq-3291e91f800a569465ad084a6bb1a61b'
LEGACY_RECOVERY_SOURCE_SHA = '39828e6f69d5567cbfeadabd9536ea6ef2a8f786'
LEGACY_RECOVERY_TEMPLATE_SHA256 = '5ad1c0250574abfadda4bbcf8baa3cd3b3a1525be811f303ea9ea40161e67e0e'
LEGACY_RECOVERY_IMAGE_ID = 'sha256:9dcea06f038bd866c2386955f8422b210f5630c3cd722513b37d5ce13fe9d299'
LEGACY_RECOVERY_NAMESPACE_UID = '15950fc8-0392-4053-9156-e95e78b596f9'
LEGACY_RECOVERY_JOURNAL_SHA256 = '83aaf4a2931ce687c05baa23bc5f6f4ae9580dfa1f518d61583545e19209376c'
OPERATION_ANNOTATION = 'voice.dev/match-squad-pki-operation'
SOURCE_ANNOTATION = 'voice.dev/match-squad-pki-source-sha256'
MANAGED_LABELS = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}
CA_OBJECT = 'voice-match-squad-ca'
EXPECTED_SECRETS = {
    'voice-matchmaking-principal-signing': ('Opaque', {'current.pem', 'next.pem', 'active-kid'}),
    'voice-matchmaking-client-tls': ('kubernetes.io/tls', {'tls.crt', 'tls.key'}),
    'voice-chat-match-squad-server-tls': ('kubernetes.io/tls', {'tls.crt', 'tls.key'}),
    'voice-voice-match-squad-server-tls': ('kubernetes.io/tls', {'tls.crt', 'tls.key'}),
    'voice-match-squad-replay': ('Opaque', {'password'}),
    'voice-match-squad-member-replay': ('Opaque', {'password'}),
}
EXPECTED_OBJECTS = set(EXPECTED_SECRETS) | {CA_OBJECT}


def _decode_data(data):
    if not isinstance(data, dict):
        raise ValueError('export data must be an object')
    try:
        return {key: base64.b64decode(value, validate=True) for key, value in data.items()}
    except (ValueError, TypeError) as exc:
        raise ValueError('export contains invalid base64 data') from exc


def _decode_configmap_data(data):
    if not isinstance(data, dict) or not all(isinstance(v, str) for v in data.values()):
        raise ValueError('ConfigMap data must contain text values')
    return {key: value.encode('utf-8') for key, value in data.items()}


def validate_export(raw, operation_id, source_binding):
    """Validate the issuer's exact, namespace-bound, immutable resource packet."""
    if not isinstance(operation_id, str) or not re.fullmatch(r'msq-[0-9a-f]{32}', operation_id):
        raise ValueError('invalid operation identity')
    if not isinstance(source_binding, str) or not re.fullmatch(r'[0-9a-f]{64}', source_binding):
        raise ValueError('invalid source binding')
    try:
        value = json.loads(raw)
    except (TypeError, json.JSONDecodeError) as exc:
        raise ValueError('issuer export is not valid JSON') from exc
    if value.get('apiVersion') != 'v1' or value.get('kind') != 'List' or not isinstance(value.get('items'), list):
        raise ValueError('issuer export must be a v1 List')
    items = {}
    for item in value['items']:
        if not isinstance(item, dict):
            raise ValueError('issuer export contains a non-object resource')
        meta = item.get('metadata', {})
        name = meta.get('name')
        if name in items:
            raise ValueError('issuer export contains duplicate resource names')
        items[name] = item
    if set(items) != EXPECTED_OBJECTS:
        raise ValueError('issuer export resource identities do not match the reviewed set')
    for name, item in items.items():
        meta = item.get('metadata', {})
        expected_meta_keys = {'name', 'namespace', 'labels', 'annotations'}
        expected_item_keys = {'apiVersion', 'kind', 'metadata', 'immutable', 'data'}
        if name != CA_OBJECT:
            expected_item_keys.add('type')
        if (set(item) != expected_item_keys or set(meta) != expected_meta_keys or
                item.get('apiVersion') != 'v1' or meta.get('namespace') != NAMESPACE or
                meta.get('labels') != MANAGED_LABELS or
                meta.get('annotations', {}).get(OPERATION_ANNOTATION) != operation_id or
                meta.get('annotations', {}).get(SOURCE_ANNOTATION) != source_binding or
                set(meta.get('annotations', {})) != {OPERATION_ANNOTATION, SOURCE_ANNOTATION} or
                'type' in meta or item.get('immutable') is not True):
            raise ValueError('issuer resource identity, marker, or immutability is invalid')
        if name == CA_OBJECT:
            decoded = _decode_configmap_data(item.get('data'))
            if (item.get('kind') != 'ConfigMap' or item.get('binaryData') is not None or
                    set(item.get('data', {})) != {'ca.crt'} or
                    not decoded['ca.crt'].startswith(b'-----BEGIN CERTIFICATE-----') or
                    b'PRIVATE KEY' in decoded['ca.crt']):
                raise ValueError('public CA bundle must contain only ca.crt')
        else:
            decoded = _decode_data(item.get('data'))
            expected_type, keys = EXPECTED_SECRETS[name]
            if item.get('kind') != 'Secret' or item.get('type') != expected_type or set(decoded) != keys:
                raise ValueError('Secret type or key schema differs from the reviewed contract')
            if name == 'voice-matchmaking-principal-signing':
                current = decoded['current.pem']
                next_key = decoded['next.pem']
                active = decoded['active-kid'].decode('ascii', errors='strict')
                if not current or not next_key or current == next_key or active not in {'current', 'next'}:
                    raise ValueError('signing key set or active key identifier is invalid')
            if name.endswith('-replay') and not decoded['password']:
                raise ValueError('replay credential is empty')
    if _decode_configmap_data(items[CA_OBJECT]['data'])['ca.crt'] == b'':
        raise ValueError('public CA bundle is empty')
    if decoded_secrets(items['voice-match-squad-replay'])['password'] == decoded_secrets(items['voice-match-squad-member-replay'])['password']:
        raise ValueError('provider and member replay credentials must be independent')
    return items


def decoded_public(item):
    return _decode_data(item['data'])['ca.crt']


def decoded_secrets(item):
    return _decode_data(item['data'])


def is_explicitly_missing(result):
    """Only successful kubectl get --ignore-not-found with empty stdout means absent."""
    if not isinstance(result, subprocess.CompletedProcess):
        raise ValueError('kubectl result must be a subprocess CompletedProcess')
    if result.returncode != 0:
        raise ValueError('kubectl get failed; absence is unproven')
    stdout = result.stdout
    if isinstance(stdout, bytes):
        stdout = stdout.decode('utf-8', errors='strict')
    if not isinstance(stdout, str):
        raise ValueError('kubectl output must be text')
    return stdout == ''


def parse_create_identity(stdout):
    if isinstance(stdout, bytes):
        stdout = stdout.decode('ascii', errors='strict')
    if not isinstance(stdout, str):
        raise ValueError('kubectl create identity output must be text')
    parts = stdout.strip().split('|')
    if len(parts) != 2 or not all(re.fullmatch(r'[A-Za-z0-9._-]{1,128}', part) for part in parts):
        raise ValueError('kubectl create returned incomplete identity pins')
    return parts[0], parts[1]


def _without_server_metadata(value):
    result = copy.deepcopy(value)
    meta = result.setdefault('metadata', {})
    for key in ('uid', 'resourceVersion', 'creationTimestamp', 'managedFields', 'generation'):
        meta.pop(key, None)
    return result


def _adopt_matches(expected, existing, operation_id):
    if not isinstance(existing, dict):
        return False
    meta = existing.get('metadata', {})
    if meta.get('namespace') != NAMESPACE or meta.get('annotations', {}).get(OPERATION_ANNOTATION) != operation_id:
        return False
    return _without_server_metadata(existing) == _without_server_metadata(expected)


def create_only(expected, resource, journal, persist, kube):
    """Create after a durable intent; recover only the exact same staged object."""
    name = expected.get('metadata', {}).get('name')
    operation_id = journal.get('operationId')
    if (expected.get('metadata', {}).get('namespace') != NAMESPACE or
            expected.get('metadata', {}).get('annotations', {}).get(OPERATION_ANNOTATION) != operation_id):
        raise ValueError('resource is outside the pinned operation')
    phase = resource.get('phase')
    if phase not in {'planned', 'create-intent', 'created'}:
        raise ValueError('resource phase is not resumable')
    if phase == 'created':
        existing = kube.get(expected['kind'], name)
        if not _adopt_matches(expected, existing, operation_id):
            raise ValueError('recorded resource is missing or drifted')
        meta = existing.get('metadata', {})
        if meta.get('uid') != resource.get('uid') or meta.get('resourceVersion') != resource.get('resourceVersion'):
            raise ValueError('recorded resource identity changed')
        return resource
    if phase == 'planned':
        resource['phase'] = 'create-intent'
        persist(journal)
    existing = kube.get(expected['kind'], name)
    if existing is not None:
        if not _adopt_matches(expected, existing, operation_id):
            raise ValueError('existing resource is not the exact object for this operation')
        uid = existing.get('metadata', {}).get('uid')
        resource_version = existing.get('metadata', {}).get('resourceVersion')
        if not uid or not resource_version:
            raise ValueError('adopted resource identity is incomplete')
        resource.update({'phase': 'created', 'uid': uid, 'resourceVersion': resource_version})
        persist(journal)
        return resource
    try:
        stdout = kube.create(copy.deepcopy(expected))
        uid, resource_version = parse_create_identity(stdout)
    except Exception as create_error:
        existing = kube.get(expected['kind'], name)
        if not _adopt_matches(expected, existing, operation_id):
            raise ValueError('create outcome is ambiguous and exact operation object is absent') from create_error
        uid = existing.get('metadata', {}).get('uid')
        resource_version = existing.get('metadata', {}).get('resourceVersion')
        if not uid or not resource_version:
            raise ValueError('adopted resource identity is incomplete') from create_error
    else:
        existing = kube.get(expected['kind'], name)
        if not _adopt_matches(expected, existing, operation_id):
            raise ValueError('created resource differs from the exact staged object')
        actual_meta = existing.get('metadata', {})
        if actual_meta.get('uid') != uid or actual_meta.get('resourceVersion') != resource_version:
            raise ValueError('created resource identity differs from the create receipt')
    resource.update({'phase': 'created', 'uid': uid, 'resourceVersion': resource_version})
    persist(journal)
    return resource


def public_receipt(items, journal):
    """Return only public identity and shape; never return Secret data or hashes."""
    resources = []
    for name in sorted(items):
        item = items[name]
        row = {'kind': item['kind'], 'name': name, 'namespace': NAMESPACE,
               'keys': sorted(item['data']), 'secretType': item.get('type')}
        if name == CA_OBJECT:
            row['sha256'] = _certificate_fingerprint(item['data']['ca.crt'].encode('utf-8'))
        elif item.get('type') == 'kubernetes.io/tls':
            row['sha256'] = _certificate_fingerprint(base64.b64decode(item['data']['tls.crt'], validate=True))
        pin = journal.get('resources', {}).get(name, {})
        for field in ('uid', 'resourceVersion'):
            if pin.get(field):
                row[field] = pin[field]
        resources.append(row)
    return {'operationId': journal.get('operationId'), 'namespace': NAMESPACE,
            'sourceSha': journal.get('sourceSha'), 'issuerImageId': journal.get('issuerImageId'),
            'resources': resources}


def _certificate_fingerprint(pem):
    result = subprocess.run(['openssl', 'x509', '-noout', '-fingerprint', '-sha256'],
                            input=pem, capture_output=True, timeout=20)
    if result.returncode:
        raise ValueError('public certificate fingerprint failed')
    match = re.search(rb'Fingerprint=([0-9A-Fa-f:]+)', result.stdout)
    if not match:
        raise ValueError('public certificate fingerprint output was malformed')
    return match.group(1).decode('ascii').upper()


def run_result(args, *, input_text=None, cwd=None):
    return subprocess.run([str(arg) for arg in args], input=input_text, text=True,
                          capture_output=True, timeout=180, check=False, cwd=cwd)


def require_linux_operator():
    if sys.platform != 'linux':
        raise ValueError('operator actions are restricted to the designated Linux host')


@contextmanager
def invocation_lock(path):
    """Take a nonblocking, no-symlink Linux lock for the whole invocation."""
    require_linux_operator()
    import fcntl
    flags = os.O_CREAT | os.O_RDWR | getattr(os, 'O_NOFOLLOW', 0)
    fd = os.open(path, flags, 0o600)
    try:
        info = os.fstat(fd)
        if info.st_uid != os.geteuid() or info.st_mode & 0o077:
            raise ValueError('operator lock ownership or mode is unsafe')
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BaseException:
        os.close(fd)
        raise
    try:
        yield
    finally:
        os.close(fd)


def write_private_json(path, value):
    """Atomically persist a private recovery journal with mode 0600 and fsync."""
    require_linux_operator()
    path = Path(path)
    if path.parent.is_symlink() or not path.parent.is_dir():
        raise ValueError('private state directory must be a real directory')
    directory = path.parent.stat()
    if directory.st_uid != os.geteuid() or directory.st_mode & 0o077:
        raise ValueError('private state directory ownership or mode is unsafe')
    temp = path.with_name(path.name + '.tmp')
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, 'O_NOFOLLOW', 0)
    fd = os.open(temp, flags, 0o600)
    try:
        with os.fdopen(fd, 'w', encoding='utf-8') as stream:
            stream.write(json.dumps(value, sort_keys=True, separators=(',', ':')) + '\n')
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temp, path)
        dfd = os.open(path.parent, os.O_RDONLY | getattr(os, 'O_DIRECTORY', 0))
        try:
            os.fsync(dfd)
        finally:
            os.close(dfd)
        os.chmod(path, 0o600, follow_symlinks=False)
    except BaseException:
        try:
            temp.unlink()
        except FileNotFoundError:
            pass
        raise


def validate_source_pins(expected_source_sha):
    if not re.fullmatch(r'[0-9a-f]{40}', expected_source_sha or ''):
        raise ValueError('an exact source commit is required')
    repo_root = Path(__file__).resolve().parents[2]
    result = run_result(['git', 'rev-parse', 'HEAD'], cwd=repo_root)
    if result.returncode or result.stdout.strip() != expected_source_sha:
        raise ValueError('checked-out source commit differs from the approved pin')
    dirty = run_result(['git', 'status', '--porcelain', '--untracked-files=all'], cwd=repo_root)
    if dirty.returncode or dirty.stdout.strip():
        raise ValueError('tracked source changes are present')
    template_path = repo_root / 'deploy' / 'staging' / 'services.yaml'
    if template_path.is_symlink() or not template_path.is_file():
        raise ValueError('canonical staging deployment source is unavailable')
    template_hash = hashlib.sha256(template_path.read_bytes()).hexdigest()
    return {'sourceSha': expected_source_sha, 'templateSha256': template_hash}


def state_dir():
    return Path('/home/pmd/.local/share/voice-dev-match-squad-pki')


def ensure_private_state_directory(directory):
    directory = Path(directory)
    ancestors = [directory, *directory.parents]
    if any(path.is_symlink() for path in ancestors):
        raise ValueError('private operator state path contains a symlink')
    created = not directory.exists()
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    info = directory.stat()
    if not directory.is_dir() or info.st_uid != os.geteuid():
        raise ValueError('private operator state directory ownership is unsafe')
    if created:
        os.chmod(directory, 0o700)
    elif info.st_mode & 0o077:
        raise ValueError('private operator state directory mode is unsafe')
    return directory


def _source_binding(pins):
    return hashlib.sha256((pins['sourceSha'] + ':' + pins['templateSha256']).encode('ascii')).hexdigest()


def _namespace_identity(expected_uid):
    result = run_result(['kubectl', 'get', 'namespace', NAMESPACE, '-o', 'json'])
    if result.returncode:
        raise ValueError('DEV namespace read failed')
    try:
        value = json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise ValueError('DEV namespace metadata is malformed') from exc
    metadata = value.get('metadata', {})
    if (metadata.get('name') != NAMESPACE or metadata.get('uid') != expected_uid or
            any(metadata.get('labels', {}).get(key) != label for key, label in MANAGED_LABELS.items())):
        raise ValueError('DEV namespace identity or ownership does not match')
    return {'name': NAMESPACE, 'uid': expected_uid}


def _inventory_absent():
    names = [("configmap", CA_OBJECT)] + [("secret", name) for name in sorted(EXPECTED_SECRETS)]
    for kind, name in names:
        result = run_result(['kubectl', '--namespace', NAMESPACE, 'get', kind, name,
                             '--ignore-not-found', '-o', 'name'])
        if not is_explicitly_missing(result):
            raise ValueError('a target DEV resource already exists; create-only preparation refused')


def _check_local_image(image_id, source_sha):
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id or ''):
        raise ValueError('issuer image must be pinned by immutable local image ID')
    result = run_result(['docker', 'image', 'inspect',
                         '--format={{.Id}}|{{index .Config.Labels "voice.source.commit"}}', image_id])
    expected = image_id + '|' + source_sha
    if result.returncode or result.stdout.strip() != expected:
        raise ValueError('pinned issuer image is unavailable or bound to different source')


def _journal_path(directory):
    return Path(directory) / 'operation.json'


def _load_journal(directory):
    return _load_journal_snapshot(directory)[0]


def _load_journal_snapshot(directory):
    path = _journal_path(directory)
    if path.is_symlink() or not path.is_file():
        raise ValueError('private operation journal is unavailable')
    info = path.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o077:
        raise ValueError('private operation journal ownership or mode is unsafe')
    raw = path.read_bytes()
    value = json.loads(raw.decode('utf-8'))
    if (value.get('namespace') != NAMESPACE or value.get('volume') != 'voice-dev-match-squad-pki' or
            not re.fullmatch(r'msq-[0-9a-f]{32}', value.get('operationId', ''))):
        raise ValueError('private operation journal binding is invalid')
    return value, hashlib.sha256(raw).hexdigest()


def _save_journal(directory, journal):
    write_private_json(_journal_path(directory), journal)


def _issuer_output(image_id, volume, action, operation_id, binding):
    result = run_result(['docker', 'run', '--rm', '--network', 'none', '--read-only',
                         '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges',
                         '--user', '10002:10002', '--tmpfs',
                         '/tmp:rw,noexec,nosuid,nodev,size=64m,mode=0700,uid=10002,gid=10002',
                         '--mount', f'type=volume,source={volume},target=/state',
                         image_id, action, '--state-dir', '/state', '--operation-id', operation_id,
                         '--source-sha256', binding])
    if result.returncode:
        raise ValueError('offline issuer operation failed; staged state is preserved')
    return result.stdout


def _read_staged_manifest(journal, *, stage_first=False):
    stage_receipt = None
    if stage_first:
        raw_receipt = _issuer_output(journal['issuerImageId'], journal['volume'], 'stage',
                                     journal['operationId'], journal['sourceBinding'])
        try:
            stage_receipt = json.loads(raw_receipt)
        except json.JSONDecodeError as exc:
            raise ValueError('issuer stage receipt is malformed; staged state is preserved') from exc
        if (stage_receipt.get('operationId') != journal['operationId'] or
                stage_receipt.get('sourceSha256') != journal['sourceBinding'] or
                not re.fullmatch(r'[0-9a-f]{64}', stage_receipt.get('manifestSha256', ''))):
            raise ValueError('issuer stage receipt is not bound to this operation')
    raw = _issuer_output(journal['issuerImageId'], journal['volume'], 'export',
                         journal['operationId'], journal['sourceBinding'])
    items = validate_export(raw, journal['operationId'], journal['sourceBinding'])
    digest = hashlib.sha256(raw.encode('utf-8')).hexdigest()
    if stage_receipt and digest != stage_receipt['manifestSha256']:
        raise ValueError('issuer stage receipt differs from staged material')
    if journal.get('manifestSha256') and digest != journal['manifestSha256']:
        raise ValueError('exported staged material differs from the pinned operation')
    journal.update({'phase': 'staged', 'manifestSha256': digest})
    return items


def _ensure_volume_absent_or_owned(operation_id):
    name = 'voice-dev-match-squad-pki'
    listing = run_result(['docker', 'volume', 'ls', '--filter', 'name=' + name, '--format={{.Name}}'])
    if listing.returncode:
        raise ValueError('issuer volume inventory failed')
    existing_names = listing.stdout.splitlines()
    if existing_names:
        if existing_names != [name]:
            raise ValueError('issuer volume inventory returned an unexpected identity')
        inspect = run_result(['docker', 'volume', 'inspect', '--format={{json .Labels}}', name])
        if inspect.returncode:
            raise ValueError('issuer volume ownership inspection failed')
        try:
            labels = json.loads(inspect.stdout)
        except json.JSONDecodeError as exc:
            raise ValueError('issuer volume ownership is unproven') from exc
        if labels != {'voice.environment': 'dev', 'voice.dev/match-squad-pki-operation': operation_id}:
            raise ValueError('dedicated MatchSquad issuer volume belongs to another operation')
        return name
    create = run_result(['docker', 'volume', 'create', '--label', 'voice.environment=dev',
                         '--label', 'voice.dev/match-squad-pki-operation=' + operation_id, name])
    if create.returncode or create.stdout.strip() != name:
        raise ValueError('dedicated MatchSquad issuer volume creation failed')
    return _ensure_volume_absent_or_owned(operation_id)


def _verify_volume_owned(operation_id):
    name = 'voice-dev-match-squad-pki'
    listing = run_result(['docker', 'volume', 'ls', '--filter', 'name=' + name, '--format={{.Name}}'])
    if listing.returncode or listing.stdout.splitlines() != [name]:
        raise ValueError('staged issuer volume is missing or ambiguous')
    inspect = run_result(['docker', 'volume', 'inspect', '--format={{json .Labels}}', name])
    if inspect.returncode:
        raise ValueError('issuer volume ownership inspection failed')
    try:
        labels = json.loads(inspect.stdout)
    except json.JSONDecodeError as exc:
        raise ValueError('issuer volume ownership is unproven') from exc
    if labels != {'voice.environment': 'dev', 'voice.dev/match-squad-pki-operation': operation_id}:
        raise ValueError('staged issuer volume belongs to another operation')
    return name


class KubectlCreateOnly:
    def __init__(self, namespace_uid):
        self.namespace_uid = namespace_uid

    def _check_namespace(self):
        _namespace_identity(self.namespace_uid)

    def get(self, kind, name):
        self._check_namespace()
        result = run_result(['kubectl', '--namespace', NAMESPACE, 'get', kind.lower(), name,
                             '--ignore-not-found', '-o', 'json'])
        if is_explicitly_missing(result):
            return None
        try:
            return json.loads(result.stdout)
        except json.JSONDecodeError as exc:
            raise ValueError('kubectl resource read was malformed') from exc

    def create(self, value):
        self._check_namespace()
        result = run_result(['kubectl', '--namespace', NAMESPACE, 'create', '-f', '-',
                             '-o', 'go-template={{.metadata.uid}}|{{.metadata.resourceVersion}}'],
                            input_text=json.dumps(value, separators=(',', ':')))
        if result.returncode:
            raise ValueError('kubectl create failed; exact-object recovery will be checked')
        return result.stdout


def _verify_journal_pins(journal, pins, image_id, namespace):
    if (journal.get('sourceSha') != pins['sourceSha'] or
            journal.get('templateSha256') != pins['templateSha256'] or
            journal.get('issuerImageId') != image_id or journal.get('namespaceUid') != namespace['uid'] or
            journal.get('sourceBinding') != _source_binding(pins)):
        raise ValueError('resumed operation pins differ from staged operation')


def _migrate_legacy_journal(directory, journal, pins, image_id, namespace, journal_sha256):
    if journal.get('legacyRecovery') is not None:
        recovery = journal['legacyRecovery']
        expected_recovery = {
            'operationId': LEGACY_RECOVERY_OPERATION,
            'journalSha256': LEGACY_RECOVERY_JOURNAL_SHA256,
            'previousSourceSha': LEGACY_RECOVERY_SOURCE_SHA,
            'previousTemplateSha256': LEGACY_RECOVERY_TEMPLATE_SHA256,
            'previousSourceBinding': _source_binding({'sourceSha': LEGACY_RECOVERY_SOURCE_SHA,
                                                      'templateSha256': LEGACY_RECOVERY_TEMPLATE_SHA256}),
            'previousIssuerImageId': LEGACY_RECOVERY_IMAGE_ID,
            'previousNamespaceUid': LEGACY_RECOVERY_NAMESPACE_UID,
        }
        if (recovery != expected_recovery or journal.get('sourceSha') != pins['sourceSha'] or
                journal.get('templateSha256') != pins['templateSha256'] or
                journal.get('sourceBinding') != _source_binding(pins) or
                journal.get('issuerImageId') != image_id or namespace.get('uid') != LEGACY_RECOVERY_NAMESPACE_UID):
            raise ValueError('saved legacy recovery binding is invalid')
        return False
    if (journal.get('sourceSha') == pins['sourceSha'] and
            journal.get('templateSha256') == pins['templateSha256'] and
            journal.get('issuerImageId') == image_id):
        return False
    resources = journal.get('resources')
    planned = {name: {'phase': 'planned'} for name in sorted(EXPECTED_OBJECTS)}
    old_pins = {'sourceSha': LEGACY_RECOVERY_SOURCE_SHA,
                'templateSha256': LEGACY_RECOVERY_TEMPLATE_SHA256}
    if (journal_sha256 != LEGACY_RECOVERY_JOURNAL_SHA256 or
            journal.get('operationId') != LEGACY_RECOVERY_OPERATION or
            journal.get('namespace') != NAMESPACE or
            journal.get('namespaceUid') != LEGACY_RECOVERY_NAMESPACE_UID or
            namespace.get('uid') != LEGACY_RECOVERY_NAMESPACE_UID or
            journal.get('sourceSha') != LEGACY_RECOVERY_SOURCE_SHA or
            journal.get('templateSha256') != LEGACY_RECOVERY_TEMPLATE_SHA256 or
            journal.get('sourceBinding') != _source_binding(old_pins) or
            journal.get('issuerImageId') != LEGACY_RECOVERY_IMAGE_ID or
            journal.get('volume') != 'voice-dev-match-squad-pki' or
            journal.get('phase') != 'stage-intent' or resources != planned or
            'manifestSha256' in journal or
            not re.fullmatch(r'[0-9a-f]{40}', pins.get('sourceSha', '')) or
            not re.fullmatch(r'[0-9a-f]{64}', pins.get('templateSha256', '')) or
            not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id or '')):
        raise ValueError('saved operation does not match the sole approved legacy recovery transition')
    _inventory_absent()
    _verify_volume_owned(LEGACY_RECOVERY_OPERATION)
    inspected = _issuer_output(image_id, 'voice-dev-match-squad-pki', 'inspect-empty-building',
                               LEGACY_RECOVERY_OPERATION, _source_binding(pins))
    try:
        state = json.loads(inspected)
    except json.JSONDecodeError as exc:
        raise ValueError('same-operation issuer partial state is unverified') from exc
    if state != {'operationId': LEGACY_RECOVERY_OPERATION, 'state': 'empty-building'}:
        raise ValueError('same-operation issuer partial state is not safely retryable')
    journal['legacyRecovery'] = {
        'operationId': LEGACY_RECOVERY_OPERATION,
        'journalSha256': LEGACY_RECOVERY_JOURNAL_SHA256,
        'previousSourceSha': LEGACY_RECOVERY_SOURCE_SHA,
        'previousTemplateSha256': LEGACY_RECOVERY_TEMPLATE_SHA256,
        'previousSourceBinding': journal['sourceBinding'],
        'previousIssuerImageId': LEGACY_RECOVERY_IMAGE_ID,
        'previousNamespaceUid': LEGACY_RECOVERY_NAMESPACE_UID,
    }
    journal.update(sourceSha=pins['sourceSha'], templateSha256=pins['templateSha256'],
                   sourceBinding=_source_binding(pins), issuerImageId=image_id)
    _save_journal(directory, journal)
    return True


def _run_preflight(args):
    pins = validate_source_pins(args.expected_source_sha)
    if pins['templateSha256'] != args.expected_template_sha256:
        raise ValueError('staging template hash differs from the approved pin')
    _check_local_image(args.issuer_image, pins['sourceSha'])
    namespace = _namespace_identity(args.expected_namespace_uid)
    _inventory_absent()
    return {'namespace': namespace, **pins, 'issuerImageId': args.issuer_image,
            'targetResources': sorted(EXPECTED_OBJECTS)}


def _prepare(args, directory):
    pins = validate_source_pins(args.expected_source_sha)
    if pins['templateSha256'] != args.expected_template_sha256:
        raise ValueError('staging template hash differs from the approved pin')
    _check_local_image(args.issuer_image, pins['sourceSha'])
    namespace = _namespace_identity(args.expected_namespace_uid)
    _inventory_absent()
    directory = ensure_private_state_directory(directory)
    if _journal_path(directory).exists() or _journal_path(directory).is_symlink():
        raise ValueError('an operation journal already exists; do not start a new operation')
    operation_id = 'msq-' + secrets.token_hex(16)
    volume = 'voice-dev-match-squad-pki'
    journal = {'operationId': operation_id, 'namespace': NAMESPACE, 'namespaceUid': namespace['uid'],
               'sourceSha': pins['sourceSha'], 'templateSha256': pins['templateSha256'],
               'sourceBinding': _source_binding(pins), 'issuerImageId': args.issuer_image,
               'volume': volume, 'phase': 'initializing', 'resources': {
                   name: {'phase': 'planned'} for name in sorted(EXPECTED_OBJECTS)}}
    _save_journal(directory, journal)
    _ensure_volume_absent_or_owned(operation_id)
    journal['phase'] = 'stage-intent'
    _save_journal(directory, journal)
    items = _read_staged_manifest(journal, stage_first=True)
    _save_journal(directory, journal)
    return public_receipt(items, journal)


def _apply(args, directory):
    pins = validate_source_pins(args.expected_source_sha)
    _check_local_image(args.issuer_image, pins['sourceSha'])
    namespace = _namespace_identity(args.expected_namespace_uid)
    journal, journal_sha256 = _load_journal_snapshot(directory)
    _migrate_legacy_journal(directory, journal, pins, args.issuer_image, namespace, journal_sha256)
    _verify_journal_pins(journal, pins, args.issuer_image, namespace)
    if journal.get('phase') not in {'initializing', 'stage-intent', 'staged', 'apply-in-progress', 'complete'}:
        raise ValueError('operation phase is not eligible for create-only application')
    if journal.get('phase') == 'initializing':
        # A volume-create receipt may be lost before any issuer generation begins.
        _ensure_volume_absent_or_owned(journal['operationId'])
        journal['phase'] = 'stage-intent'
        _save_journal(directory, journal)
        items = _read_staged_manifest(journal, stage_first=True)
        _save_journal(directory, journal)
    else:
        _verify_volume_owned(journal['operationId'])
        items = _read_staged_manifest(journal, stage_first=journal.get('phase') == 'stage-intent')
        if journal.get('phase') == 'stage-intent':
            _save_journal(directory, journal)
    completed = journal.get('phase') == 'complete'
    if not completed:
        journal['phase'] = 'apply-in-progress'
        _save_journal(directory, journal)
    kube = KubectlCreateOnly(journal['namespaceUid'])
    for name in [CA_OBJECT] + sorted(EXPECTED_SECRETS):
        create_only(items[name], journal['resources'][name], journal, lambda value: _save_journal(directory, value), kube)
    if not completed:
        journal['phase'] = 'complete'
        _save_journal(directory, journal)
    return public_receipt(items, journal)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    for command in ('preflight', 'prepare', 'apply', 'resume'):
        child = sub.add_parser(command)
        child.add_argument('--issuer-image', required=True)
        child.add_argument('--expected-source-sha', required=True)
        child.add_argument('--expected-namespace-uid', required=True)
        child.add_argument('--expected-template-sha256', required=True)
        if command in {'prepare', 'apply', 'resume'}:
            child.add_argument('--confirm-dev-match-squad-pki-change', action='store_true')
    args = parser.parse_args(argv)
    if args.command in {'prepare', 'apply', 'resume'} and not args.confirm_dev_match_squad_pki_change:
        parser.error(f'{args.command} requires --confirm-dev-match-squad-pki-change')
    if sys.platform != 'linux':
        parser.error('operator commands are restricted to the designated Linux host')
    try:
        if args.command == 'preflight':
            print(json.dumps(_run_preflight(args), sort_keys=True))
            return 0
        if args.command == 'prepare':
            directory = ensure_private_state_directory(state_dir())
            with invocation_lock(directory / 'operator.lock'):
                print(json.dumps(_prepare(args, directory), sort_keys=True))
            return 0
        if args.command in {'apply', 'resume'}:
            directory = state_dir()
            with invocation_lock(directory / 'operator.lock'):
                print(json.dumps(_apply(args, directory), sort_keys=True))
            return 0
    except (ValueError, OSError, json.JSONDecodeError) as exc:
        parser.error(str(exc))


if __name__ == '__main__':
    raise SystemExit(main())
