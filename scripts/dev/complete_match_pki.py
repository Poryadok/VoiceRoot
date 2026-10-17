#!/usr/bin/env python3
"""Create-only, same-material DEV CompleteMatch TLS provisioning."""
import argparse
import base64
import contextlib
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import secrets
import stat
import subprocess
import sys
import tempfile

NAMESPACE = 'voice-dev'
NAMESPACE_UID = '15950fc8-0392-4053-9156-e95e78b596f9'
OLD_OPERATION = 'msq-3291e91f800a569465ad084a6bb1a61b'
OLD_SOURCE_SHA = '76882140d4f2abda4a40923bc67a6ec78aa69393'
OLD_TEMPLATE_SHA256 = '5ad1c0250574abfadda4bbcf8baa3cd3b3a1525be811f303ea9ea40161e67e0e'
OLD_IMAGE_ID = 'sha256:3f3c5fb9a9b0dd2fcb7660dd6b6c1b48957f380553b838f16c2a9309fac035fd'
OLD_JOURNAL_SHA256 = '09e04ea84d04a55362570aabf082b84f07d70d0e3d170b73047e867b74eb5e33'
OLD_JOURNAL_PATH = Path('/home/pmd/.local/share/voice-dev-match-squad-pki/operation.json')
OLD_VOLUME = 'voice-dev-match-squad-pki'
OLD_CA_NAME = 'voice-match-squad-ca'
OLD_CA_UID = 'cbe47dee-c57e-4cb0-bcf0-e6eeb67c00e6'
OLD_CA_RV = '3381287'
OLD_CA_FINGERPRINT = 'da6dfa9ffbea4903310e624e6e38e411adf2f1353edd264fbe3b7cdd15b33548'
STATE_PATH = Path('/home/pmd/.local/share/voice-dev-complete-match-pki')
KUBECONFIG_PATH = Path('/home/pmd/.kube/config')
KUBECTL_CONTEXT = 'default'
NEW_VOLUME = 'voice-dev-complete-match-pki'
OPERATION_ANNOTATION = 'voice.dev/complete-match-pki-operation'
SOURCE_ANNOTATION = 'voice.dev/complete-match-pki-source-sha256'
RETAINED_CA_FINGERPRINT_ANNOTATION = 'voice.dev/complete-match-retained-ca-sha256'
VOLUME_OPERATION_LABEL = 'voice.dev/complete-match-pki-operation'
MANAGED_LABELS = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}
SERVER_SECRET = 'voice-matchmaking-complete-server-tls'
CLIENT_CA = 'voice-matchmaking-complete-client-ca'
GATEWAY_SECRET = 'voice-gateway-matchmaking-complete-client-tls'
TARGETS = {
    SERVER_SECRET: ('secret', 'kubernetes.io/tls', {'tls.crt', 'tls.key'}),
    CLIENT_CA: ('configmap', None, {'ca.crt'}),
    GATEWAY_SECRET: ('secret', 'kubernetes.io/tls', {'tls.crt', 'tls.key'}),
}
CONSUMER_CONTRACT_PATHS = (
    'deploy/dev/match-squad-trust-overlay/voice-matchmaking.runtime.patch.yaml',
    'deploy/dev/match-squad-trust-overlay/voice-gateway.patch.yaml',
)
STATE_FIELDS = {
    'schema', 'operationId', 'namespace', 'namespaceUid', 'sourceSha',
    'consumerContractSha256', 'issuerImageId', 'volume', 'retained', 'phase',
    'resources', 'stageReceipt',
}
RETAINED_FIELDS = {
    'operationId', 'journalSha256', 'sourceSha', 'templateSha256',
    'issuerImageId', 'volume', 'namespaceUid', 'caUid', 'caResourceVersion', 'fingerprint',
}
PHASES = {'initializing', 'stage-intent', 'staged', 'apply-in-progress', 'complete'}

_ISSUER_PATH = Path(__file__).with_name('complete-match-pki-issuer') / 'issuer.py'
_ISSUER_SPEC = importlib.util.spec_from_file_location('complete_match_pki_issuer', _ISSUER_PATH)
_ISSUER = importlib.util.module_from_spec(_ISSUER_SPEC)
_ISSUER_SPEC.loader.exec_module(_ISSUER)


class Blocked(ValueError):
    """A fail-closed operator guard with a payload-free reason."""


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('utf-8')


def certificate_fingerprint(pem):
    if not isinstance(pem, bytes) or pem.count(b'-----BEGIN CERTIFICATE-----') != 1 or pem.count(b'-----END CERTIFICATE-----') != 1:
        raise Blocked('certificate_pem_invalid')
    der = run_result(['openssl', 'x509', '-outform', 'DER'], input_bytes=pem, limit=65536)
    if der.returncode:
        raise Blocked('certificate_parse_failed')
    return hashlib.sha256(der.stdout).hexdigest()


def consumer_contract(root):
    root = Path(root).resolve(strict=True)
    digest = hashlib.sha256()
    for relative in CONSUMER_CONTRACT_PATHS:
        path = root / relative
        if not path.resolve(strict=True).is_relative_to(root) or not path.is_file():
            raise Blocked('consumer_contract_missing')
        raw = path.read_bytes()
        if not raw:
            raise Blocked('consumer_contract_empty')
        digest.update(relative.encode('utf-8') + b'\0' + len(raw).to_bytes(8, 'big') + raw)
    return digest.hexdigest()


def validate_export(raw, operation_id, source_sha, retained_ca_cert):
    if not re.fullmatch(r'cmp-[0-9a-f]{32}', operation_id or ''):
        raise Blocked('operation_identity_invalid')
    if not re.fullmatch(r'[0-9a-f]{40}', source_sha or ''):
        raise Blocked('source_identity_invalid')
    try:
        value = json.loads(raw)
    except (TypeError, json.JSONDecodeError) as exc:
        raise Blocked('issuer_packet_invalid') from exc
    if (not isinstance(value, dict) or set(value) != {'apiVersion', 'kind', 'items'} or
            value.get('apiVersion') != 'v1' or value.get('kind') != 'List' or
            not isinstance(value.get('items'), list) or len(value['items']) != len(TARGETS)):
        raise Blocked('issuer_packet_schema_invalid')
    items = {}
    for item in value['items']:
        if not isinstance(item, dict):
            raise Blocked('issuer_resource_invalid')
        meta = item.get('metadata')
        if not isinstance(meta, dict):
            raise Blocked('issuer_metadata_invalid')
        name = meta.get('name')
        if not isinstance(name, str) or name in items:
            raise Blocked('issuer_resource_identity_invalid')
        items[name] = item
    if set(items) != set(TARGETS):
        raise Blocked('issuer_resource_set_invalid')
    retained_fingerprint = certificate_fingerprint(retained_ca_cert)
    for name, (kind, secret_type, expected_keys) in TARGETS.items():
        item = items[name]
        metadata = item.get('metadata', {})
        expected_annotations = {OPERATION_ANNOTATION: operation_id, SOURCE_ANNOTATION: source_sha}
        if name == SERVER_SECRET:
            expected_annotations[RETAINED_CA_FINGERPRINT_ANNOTATION] = retained_fingerprint
        expected_meta = {'name', 'namespace', 'labels', 'annotations'}
        expected_fields = {'apiVersion', 'kind', 'metadata', 'immutable', 'data'}
        if kind == 'secret':
            expected_fields.add('type')
        if (item.get('apiVersion') != 'v1' or item.get('kind') != ('ConfigMap' if kind == 'configmap' else 'Secret') or
                set(item) != expected_fields or
                set(metadata) != expected_meta or metadata.get('namespace') != NAMESPACE or
                metadata.get('labels') != MANAGED_LABELS or metadata.get('annotations') != expected_annotations or
                item.get('immutable') is not True or not isinstance(item.get('data'), dict) or
                set(item['data']) != expected_keys):
            raise Blocked('issuer_resource_contract_invalid')
        if kind == 'configmap':
            if not isinstance(item['data']['ca.crt'], str):
                raise Blocked('client_ca_bundle_invalid')
            cert = item['data']['ca.crt'].encode('ascii')
        else:
            if item.get('type') != secret_type:
                raise Blocked('issuer_secret_type_invalid')
            try:
                for encoded in item['data'].values():
                    base64.b64decode(encoded, validate=True)
            except (ValueError, TypeError) as exc:
                raise Blocked('issuer_secret_data_invalid') from exc
            cert = base64.b64decode(item['data']['tls.crt'], validate=True)
        if name == CLIENT_CA and certificate_fingerprint(cert) == retained_fingerprint:
            raise Blocked('complete_match_client_ca_reuses_retained_root')
    try:
        _ISSUER.verify_packet_profiles(raw, retained_ca_cert)
    except (ValueError, OSError, KeyError, TypeError, json.JSONDecodeError) as exc:
        raise Blocked('issuer_certificate_profile_invalid') from exc
    return items


def _server_fields(value):
    if not isinstance(value, dict):
        raise Blocked('resource_readback_invalid')
    result = json.loads(json.dumps(value))
    metadata = result.get('metadata')
    if isinstance(metadata, dict):
        for key in ('uid', 'resourceVersion', 'creationTimestamp', 'generation', 'managedFields', 'selfLink'):
            metadata.pop(key, None)
    result.pop('status', None)
    return result


def _adopt_matches(expected, existing, operation_id):
    try:
        metadata = existing.get('metadata', {})
        if (metadata.get('namespace') != NAMESPACE or
                metadata.get('annotations', {}).get(OPERATION_ANNOTATION) != operation_id):
            return False
        if not re.fullmatch(r'[0-9a-f-]{1,64}', metadata.get('uid', '')) or not str(metadata.get('resourceVersion', '')).isdigit():
            return False
        return _server_fields(existing) == _server_fields(expected)
    except (AttributeError, TypeError, ValueError):
        return False


def parse_create_identity(stdout):
    if not isinstance(stdout, str):
        raise Blocked('create_identity_invalid')
    pieces = stdout.strip().split('|')
    if len(pieces) != 2 or not re.fullmatch(r'[0-9a-f-]{1,64}', pieces[0]) or not pieces[1].isdigit():
        raise Blocked('create_identity_invalid')
    return {'uid': pieces[0], 'resourceVersion': pieces[1]}


def create_only(expected, resource, journal, persist, kube):
    """Create one exact resource after durable intent; adopt only same staged bytes."""
    name = expected.get('metadata', {}).get('name')
    if name not in TARGETS or expected.get('metadata', {}).get('namespace') != NAMESPACE:
        raise Blocked('create_target_invalid')
    operation_id = journal.get('operationId')
    if expected.get('metadata', {}).get('annotations', {}).get(OPERATION_ANNOTATION) != operation_id:
        raise Blocked('create_operation_marker_invalid')
    kind = TARGETS[name][0]
    phase = resource.get('phase')
    if phase not in ('planned', 'intent', 'created'):
        raise Blocked('create_phase_invalid')
    existing = kube.get(kind, name)
    if phase == 'planned' and existing is not None:
        raise Blocked('planned_target_already_exists')
    if existing is not None:
        if phase not in ('intent', 'created'):
            raise Blocked('create_phase_invalid')
        if not _adopt_matches(expected, existing, operation_id):
            raise Blocked('existing_target_conflict')
        identity = {'uid': existing['metadata']['uid'], 'resourceVersion': existing['metadata']['resourceVersion']}
        if resource.get('uid') not in (None, identity['uid']) or resource.get('resourceVersion') not in (None, identity['resourceVersion']):
            raise Blocked('existing_target_identity_changed')
        resource.update(phase='created', **identity)
        persist(journal)
        return identity
    if phase == 'created':
        raise Blocked('create_phase_invalid')
    if phase == 'planned':
        resource['phase'] = 'intent'
        persist(journal)
    dry_run = kube.server_dry_run(expected)
    if _server_fields(dry_run) != _server_fields(expected):
        raise Blocked('server_dry_run_admission_differs')
    create_identity = None
    create_error = None
    try:
        create_identity = kube.create(expected)
    except (OSError, ValueError, subprocess.SubprocessError):
        create_error = True
    persisted = kube.get(kind, name)
    if persisted is None or not _adopt_matches(expected, persisted, operation_id):
        if create_error:
            raise Blocked('create_result_ambiguous')
        raise Blocked('created_object_readback_mismatch')
    readback_identity = {'uid': persisted['metadata']['uid'],
                         'resourceVersion': persisted['metadata']['resourceVersion']}
    if create_identity is not None and create_identity != readback_identity:
        raise Blocked('create_readback_identity_changed')
    resource.update(phase='created', **readback_identity)
    persist(journal)
    return readback_identity


def public_receipt(items, journal):
    resources = {}
    for name, item in items.items():
        metadata = item['metadata']
        if name == CLIENT_CA:
            cert = item['data']['ca.crt'].encode('ascii')
            kind, secret_type, keys = 'ConfigMap', None, ['ca.crt']
            proof = {'caFingerprint': certificate_fingerprint(cert), 'role': 'dedicated-gateway-client-ca'}
        else:
            cert = base64.b64decode(item['data']['tls.crt'], validate=True)
            kind, secret_type, keys = 'Secret', item['type'], ['tls.crt', 'tls.key']
            if name == SERVER_SECRET:
                proof = {'caFingerprint': metadata['annotations'][RETAINED_CA_FINGERPRINT_ANNOTATION],
                         'leafFingerprint': certificate_fingerprint(cert), 'san': 'DNS:voice-matchmaking.voice-dev.svc',
                         'eku': 'serverAuth', 'ca': False}
            else:
                ca = items[CLIENT_CA]['data']['ca.crt'].encode('ascii')
                proof = {'caFingerprint': certificate_fingerprint(ca),
                         'leafFingerprint': certificate_fingerprint(cert),
                         'san': 'DNS:voice-gateway.voice-dev.svc', 'eku': 'clientAuth', 'ca': False}
        receipt = journal.get('resources', {}).get(name, {})
        resources[name] = {'kind': kind, 'type': secret_type, 'keyNames': keys,
                           'uid': receipt.get('uid'), 'resourceVersion': receipt.get('resourceVersion'),
                           'proof': proof}
    return {'schema': 'voice-dev-complete-match-pki-receipt-v1',
            'operationId': journal['operationId'], 'phase': journal['phase'],
            'sourceSha': journal['sourceSha'], 'consumerContractSha256': journal['consumerContractSha256'],
            'issuerImageId': journal['issuerImageId'], 'namespace': journal['namespace'],
            'namespaceUid': journal['namespaceUid'],
            'retainedCa': {'name': OLD_CA_NAME, 'uid': journal['retained']['caUid'],
                           'resourceVersion': journal['retained']['caResourceVersion'],
                           'fingerprint': journal['retained']['fingerprint']},
            'resources': resources}


def run_result(args, *, input_bytes=None, timeout=60, limit=1 << 20):
    try:
        result = subprocess.run(args, input=input_bytes, capture_output=True, timeout=timeout,
                                check=False, env={'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': '/nonexistent'})
    except (OSError, subprocess.SubprocessError) as exc:
        raise Blocked('external_command_unavailable') from exc
    if len(result.stdout) > limit or len(result.stderr) > limit:
        raise Blocked('external_command_output_limit')
    return result


def _pinned_kubeconfig():
    """Select only pmd's documented kubeconfig after a metadata-only custody check."""
    path = KUBECONFIG_PATH
    try:
        parent = path.parent.lstat()
        info = path.lstat()
    except OSError as exc:
        raise Blocked('kubeconfig_unavailable') from exc
    mode = stat.S_IMODE(info.st_mode)
    if (stat.S_ISLNK(parent.st_mode) or not stat.S_ISDIR(parent.st_mode) or
            parent.st_uid != os.geteuid() or parent.st_mode & 0o022 or
            stat.S_ISLNK(info.st_mode) or not stat.S_ISREG(info.st_mode) or
            info.st_uid != os.geteuid() or info.st_nlink != 1 or
            not 1 <= info.st_size <= (1 << 20) or mode & 0o077 or
            not mode & 0o400 or mode & 0o100):
        raise Blocked('kubeconfig_custody_invalid')
    return str(path)


def kubectl_result(args, *, input_bytes=None, timeout=60, limit=1 << 20):
    """Invoke kubectl with one pinned private config and the sanitized process env."""
    return run_result(['kubectl', '--kubeconfig', _pinned_kubeconfig(),
                       '--context', KUBECTL_CONTEXT, *args],
                      input_bytes=input_bytes, timeout=timeout, limit=limit)


def _kubectl_json(args, *, input_bytes=None):
    result = kubectl_result(args, input_bytes=input_bytes)
    if result.returncode:
        raise Blocked('kubectl_operation_failed')
    try:
        return json.loads(result.stdout)
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        raise Blocked('kubectl_json_invalid') from exc


class Kubectl:
    def __init__(self, namespace_uid):
        self.namespace_uid = namespace_uid

    def namespace(self):
        row = _kubectl_json(['get', 'namespace', NAMESPACE, '-o', 'json'])
        metadata = row.get('metadata', {})
        if (metadata.get('uid') != self.namespace_uid or
                any(metadata.get('labels', {}).get(key) != value for key, value in MANAGED_LABELS.items())):
            raise Blocked('namespace_identity_changed')
        return {'name': NAMESPACE, 'uid': metadata['uid']}

    def get(self, kind, name):
        result = kubectl_result(['--namespace', NAMESPACE, 'get', kind, name,
                                 '--ignore-not-found', '-o', 'json'])
        if result.returncode:
            raise Blocked('kubectl_get_failed')
        output = result.stdout.decode('utf-8', errors='strict').strip()
        if output in ('', 'null'):
            return None
        try:
            row = json.loads(output)
        except json.JSONDecodeError as exc:
            raise Blocked('kubectl_get_json_invalid') from exc
        if not isinstance(row, dict):
            raise Blocked('kubectl_get_identity_invalid')
        return row

    def server_dry_run(self, value):
        raw = canonical(value)
        return _kubectl_json(['--namespace', NAMESPACE, 'create', '--dry-run=server', '-f', '-', '-o', 'json'],
                             input_bytes=raw)

    def create(self, value):
        result = kubectl_result(['--namespace', NAMESPACE, 'create', '-f', '-',
                                 '-o', 'go-template={{.metadata.uid}}|{{.metadata.resourceVersion}}'],
                                input_bytes=canonical(value))
        if result.returncode:
            raise Blocked('kubectl_create_failed')
        return parse_create_identity(result.stdout.decode('utf-8', errors='strict'))


def _namespace_identity(expected_uid):
    if expected_uid != NAMESPACE_UID:
        raise Blocked('namespace_uid_pin_invalid')
    return Kubectl(expected_uid).namespace()


def _load_retained_journal():
    path = OLD_JOURNAL_PATH
    parent = path.parent
    if parent.is_symlink() or not parent.is_dir():
        raise Blocked('retained_operation_directory_unavailable')
    parent_info = parent.stat()
    if parent_info.st_uid != os.geteuid() or parent_info.st_mode & 0o077:
        raise Blocked('retained_operation_directory_custody_invalid')
    if path.is_symlink() or not path.is_file():
        raise Blocked('retained_operation_journal_unavailable')
    info = path.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o077:
        raise Blocked('retained_operation_journal_custody_invalid')
    raw = path.read_bytes()
    if hashlib.sha256(raw).hexdigest() != OLD_JOURNAL_SHA256:
        raise Blocked('retained_operation_journal_changed')
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise Blocked('retained_operation_journal_invalid') from exc
    expected = {'operationId': OLD_OPERATION, 'namespace': NAMESPACE, 'namespaceUid': NAMESPACE_UID,
                'sourceSha': OLD_SOURCE_SHA, 'templateSha256': OLD_TEMPLATE_SHA256,
                'issuerImageId': OLD_IMAGE_ID, 'volume': OLD_VOLUME, 'phase': 'complete'}
    if any(value.get(key) != expected_value for key, expected_value in expected.items()):
        raise Blocked('retained_operation_binding_changed')
    return {'operationId': OLD_OPERATION, 'journalSha256': OLD_JOURNAL_SHA256,
            'sourceSha': OLD_SOURCE_SHA, 'templateSha256': OLD_TEMPLATE_SHA256,
            'issuerImageId': OLD_IMAGE_ID, 'volume': OLD_VOLUME, 'namespaceUid': NAMESPACE_UID}


def _volume_inspect(name, expected_labels):
    result = run_result(['docker', 'volume', 'inspect', name])
    if result.returncode:
        raise Blocked('issuer_volume_inspect_failed')
    try:
        rows = json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise Blocked('issuer_volume_inspect_invalid') from exc
    if (not isinstance(rows, list) or len(rows) != 1 or not isinstance(rows[0], dict) or
            rows[0].get('Name') != name or rows[0].get('Driver') != 'local' or
            rows[0].get('Scope') != 'local' or rows[0].get('Labels') != expected_labels):
        raise Blocked('issuer_volume_identity_invalid')
    return rows[0]


def _volume_list():
    result = run_result(['docker', 'volume', 'ls', '--format={{.Name}}'])
    if result.returncode:
        raise Blocked('issuer_volume_inventory_failed')
    names = result.stdout.decode('utf-8', errors='strict').splitlines()
    if len(names) != len(set(names)):
        raise Blocked('issuer_volume_inventory_ambiguous')
    return names


def _no_volume_containers(volume):
    result = run_result(['docker', 'ps', '-a', '--filter', 'volume=' + volume, '--format={{.ID}}'])
    if result.returncode or result.stdout.decode('ascii', errors='strict').strip():
        raise Blocked('issuer_volume_container_reference_present')


def _verify_retained_volume():
    labels = {'voice.environment': 'dev', 'voice.dev/match-squad-pki-operation': OLD_OPERATION}
    if OLD_VOLUME not in _volume_list():
        raise Blocked('retained_issuer_volume_missing')
    _volume_inspect(OLD_VOLUME, labels)
    _no_volume_containers(OLD_VOLUME)


def _new_volume_labels(operation_id):
    return {'voice.environment': 'dev', VOLUME_OPERATION_LABEL: operation_id}


def _ensure_new_volume_absent_or_owned(operation_id, *, create=False):
    names = _volume_list()
    if NEW_VOLUME not in names:
        if not create:
            raise Blocked('complete_match_issuer_volume_missing')
        result = run_result(['docker', 'volume', 'create', '--label', 'voice.environment=dev',
                             '--label', VOLUME_OPERATION_LABEL + '=' + operation_id, NEW_VOLUME])
        if result.returncode or result.stdout.decode('utf-8', errors='strict').strip() != NEW_VOLUME:
            raise Blocked('complete_match_issuer_volume_create_failed')
        names = _volume_list()
    if NEW_VOLUME not in names:
        raise Blocked('complete_match_issuer_volume_missing')
    _volume_inspect(NEW_VOLUME, _new_volume_labels(operation_id))
    _no_volume_containers(NEW_VOLUME)
    return NEW_VOLUME


def _check_local_image(image_id, source_sha):
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id or ''):
        raise Blocked('issuer_image_pin_invalid')
    result = run_result(['docker', 'image', 'inspect', '--format={{.Id}}|{{index .Config.Labels "voice.source.commit"}}', image_id])
    expected = image_id + '|' + source_sha
    if result.returncode or result.stdout.decode('ascii', errors='strict').strip() != expected:
        raise Blocked('issuer_image_source_binding_invalid')


def _verify_source(expected_source_sha, expected_contract_sha):
    if not re.fullmatch(r'[0-9a-f]{40}', expected_source_sha or ''):
        raise Blocked('source_pin_invalid')
    root = Path(__file__).resolve().parents[2]
    result = run_result(['git', '-C', str(root), 'rev-parse', 'HEAD'], timeout=20)
    if result.returncode or result.stdout.decode('ascii', errors='strict').strip() != expected_source_sha:
        raise Blocked('source_commit_changed')
    status = run_result(['git', '-C', str(root), 'status', '--porcelain', '--untracked-files=all'], timeout=20)
    if status.returncode or status.stdout.strip():
        raise Blocked('source_worktree_not_clean')
    contract = consumer_contract(root)
    if not re.fullmatch(r'[0-9a-f]{64}', expected_contract_sha or '') or contract != expected_contract_sha:
        raise Blocked('consumer_contract_changed')
    return {'sourceSha': expected_source_sha, 'consumerContractSha256': contract}


def _retained_ca_object(kube):
    row = kube.get('configmap', OLD_CA_NAME)
    if row is None:
        raise Blocked('retained_public_ca_missing')
    metadata = row.get('metadata', {})
    expected_annotations = {'voice.dev/match-squad-pki-operation': OLD_OPERATION,
                            'voice.dev/match-squad-pki-source-sha256': OLD_SOURCE_SHA}
    if (metadata.get('uid') != OLD_CA_UID or metadata.get('resourceVersion') != OLD_CA_RV or
            metadata.get('namespace') != NAMESPACE or metadata.get('labels') != MANAGED_LABELS or
            metadata.get('annotations') != expected_annotations or row.get('immutable') is not True or
            set(row.get('data', {})) != {'ca.crt'} or not isinstance(row['data']['ca.crt'], str)):
        raise Blocked('retained_public_ca_identity_changed')
    cert = row['data']['ca.crt'].encode('ascii')
    if certificate_fingerprint(cert) != OLD_CA_FINGERPRINT:
        raise Blocked('retained_public_ca_fingerprint_changed')
    return {'uid': OLD_CA_UID, 'resourceVersion': OLD_CA_RV,
            'fingerprint': OLD_CA_FINGERPRINT, 'certificate': cert}


def _targets_absent(kube):
    for name, (kind, _, _) in TARGETS.items():
        if kube.get(kind, name) is not None:
            raise Blocked('complete_match_target_not_absent')


def _private_dir(path, *, create=False):
    path = Path(path)
    if create:
        path.mkdir(mode=0o700, parents=True, exist_ok=True)
    if path.is_symlink() or not path.is_dir():
        raise Blocked('private_state_directory_unavailable')
    info = path.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o777 != 0o700:
        raise Blocked('private_state_directory_custody_invalid')
    return path


@contextlib.contextmanager
def invocation_lock(path):
    path = Path(path)
    flags = os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW
    fd = os.open(path, flags, 0o600)
    try:
        info = os.fstat(fd)
        if info.st_uid != os.geteuid() or info.st_mode & 0o777 != 0o600:
            raise Blocked('operator_lock_custody_invalid')
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        yield
    except BlockingIOError as exc:
        raise Blocked('operator_invocation_already_running') from exc
    finally:
        os.close(fd)


def write_private_json(path, value):
    path = Path(path)
    parent = _private_dir(path.parent)
    if path.is_symlink() or path.exists() and (not path.is_file() or path.stat().st_uid != os.geteuid() or path.stat().st_mode & 0o077):
        raise Blocked('private_journal_custody_invalid')
    temporary = parent / ('.' + path.name + '.' + secrets.token_hex(8) + '.tmp')
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW
    fd = os.open(temporary, flags, 0o600)
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(canonical(value))
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        directory_fd = os.open(parent, os.O_RDONLY | getattr(os, 'O_DIRECTORY', 0))
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
    except BaseException:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass
        raise


def _journal_path(directory):
    return Path(directory) / 'operation.json'


def _load_journal(directory):
    directory = _private_dir(directory)
    path = _journal_path(directory)
    if path.is_symlink() or not path.is_file():
        raise Blocked('private_operation_journal_unavailable')
    info = path.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o077:
        raise Blocked('private_operation_journal_custody_invalid')
    raw = path.read_bytes()
    try:
        journal = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise Blocked('private_operation_journal_invalid') from exc
    if (not isinstance(journal, dict) or set(journal) != STATE_FIELDS or
            journal.get('schema') != 'voice-dev-complete-match-pki-operation-v1' or
            journal.get('namespace') != NAMESPACE or journal.get('namespaceUid') != NAMESPACE_UID or
            journal.get('volume') != NEW_VOLUME or journal.get('phase') not in PHASES or
            not re.fullmatch(r'cmp-[0-9a-f]{32}', journal.get('operationId', '')) or
            not re.fullmatch(r'[0-9a-f]{40}', journal.get('sourceSha', '')) or
            not re.fullmatch(r'[0-9a-f]{64}', journal.get('consumerContractSha256', '')) or
            not re.fullmatch(r'sha256:[0-9a-f]{64}', journal.get('issuerImageId', '')) or
            not isinstance(journal.get('resources'), dict) or set(journal['resources']) != set(TARGETS) or
            not isinstance(journal.get('retained'), dict) or set(journal['retained']) != RETAINED_FIELDS or
            journal['retained'].get('operationId') != OLD_OPERATION or
            journal['retained'].get('journalSha256') != OLD_JOURNAL_SHA256 or
            journal['retained'].get('sourceSha') != OLD_SOURCE_SHA or
            journal['retained'].get('templateSha256') != OLD_TEMPLATE_SHA256 or
            journal['retained'].get('issuerImageId') != OLD_IMAGE_ID or
            journal['retained'].get('volume') != OLD_VOLUME or
            journal['retained'].get('namespaceUid') != NAMESPACE_UID or
            journal['retained'].get('caUid') != OLD_CA_UID or
            journal['retained'].get('caResourceVersion') != OLD_CA_RV or
            journal['retained'].get('fingerprint') != OLD_CA_FINGERPRINT):
        raise Blocked('private_operation_journal_binding_invalid')
    for name, row in journal['resources'].items():
        if not isinstance(row, dict) or row.get('phase') not in ('planned', 'intent', 'created'):
            raise Blocked('private_operation_resource_state_invalid')
        expected_row_fields = {'phase'} if row['phase'] in ('planned', 'intent') else {
            'phase', 'uid', 'resourceVersion'}
        if set(row) != expected_row_fields:
            raise Blocked('private_operation_resource_state_invalid')
        if row.get('phase') == 'created' and (
                not re.fullmatch(r'[0-9a-f-]{1,64}', row.get('uid', '')) or
                not str(row.get('resourceVersion', '')).isdigit()):
            raise Blocked('private_operation_resource_receipt_invalid')
    stage_receipt = {'state': 'staged', 'resourceNames': sorted(TARGETS)}
    if (journal.get('stageReceipt') is not None and journal['stageReceipt'] != stage_receipt or
            journal.get('phase') in {'initializing', 'stage-intent'} and journal.get('stageReceipt') is not None or
            journal.get('phase') in {'staged', 'apply-in-progress', 'complete'} and journal.get('stageReceipt') != stage_receipt):
        raise Blocked('private_operation_stage_receipt_invalid')
    resource_phases = {row['phase'] for row in journal['resources'].values()}
    phase = journal['phase']
    if ((phase in {'initializing', 'stage-intent', 'staged'} and resource_phases != {'planned'}) or
            phase == 'staged' and resource_phases != {'planned'} or
            phase == 'apply-in-progress' and not resource_phases <= {'planned', 'intent', 'created'} or
            phase == 'complete' and resource_phases != {'created'}):
        raise Blocked('private_operation_completion_receipt_invalid')
    return journal


def _save_journal(directory, journal):
    if set(journal) != STATE_FIELDS:
        raise Blocked('private_operation_journal_schema_invalid')
    write_private_json(_journal_path(directory), journal)


def _issuer_output(image_id, volume, operation_id, source_sha, action, retained_cert):
    with tempfile.TemporaryDirectory(prefix='complete-match-public-input-') as temp_name:
        ca_path = Path(temp_name) / 'retained-ca.crt'
        ca_path.write_bytes(retained_cert)
        ca_path.chmod(0o644)
        command = [
            'docker', 'run', '--rm', '--network', 'none', '--read-only', '--cap-drop', 'ALL',
            '--security-opt', 'no-new-privileges', '--user', '10002:10002', '--tmpfs',
            '/tmp:rw,noexec,nosuid,nodev,size=64m,mode=0700,uid=10002,gid=10002',
            '--mount', f'type=volume,source={volume},target=/state',
            '--mount', f'type=volume,source={OLD_VOLUME},target=/retained,readonly',
            '--mount', f'type=bind,source={ca_path},target=/input/retained-ca.crt,readonly',
            image_id, action, '--state-dir', '/state', '--operation-id', operation_id,
            '--source-sha256', source_sha,
        ]
        result = run_result(command, timeout=180, limit=8 << 20)
    if result.returncode:
        raise Blocked('offline_issuer_failed_staged_material_preserved')
    return result.stdout


def _read_export(journal, retained_ca):
    raw = _issuer_output(journal['issuerImageId'], NEW_VOLUME, journal['operationId'],
                         journal['sourceSha'], 'export', retained_ca['certificate'])
    items = validate_export(raw, journal['operationId'], journal['sourceSha'], retained_ca['certificate'])
    return raw, items


def _record_stage(journal, output):
    try:
        value = json.loads(output)
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        raise Blocked('issuer_stage_receipt_invalid') from exc
    if value != {'operationId': journal['operationId'], 'sourceSha256': journal['sourceSha'],
                 'resourceNames': sorted(TARGETS), 'state': 'staged'}:
        raise Blocked('issuer_stage_receipt_binding_invalid')
    journal['stageReceipt'] = {'state': 'staged', 'resourceNames': sorted(TARGETS)}
    journal['phase'] = 'staged'


def _check_operation_inputs(journal, args, *, create=False):
    pins = _verify_source(args.expected_source_sha, args.expected_consumer_contract_sha256)
    if journal['sourceSha'] != pins['sourceSha'] or journal['consumerContractSha256'] != pins['consumerContractSha256']:
        raise Blocked('operation_source_or_contract_changed')
    _check_local_image(args.issuer_image, pins['sourceSha'])
    if journal['issuerImageId'] != args.issuer_image:
        raise Blocked('operation_issuer_image_changed')
    namespace = _namespace_identity(args.expected_namespace_uid)
    if journal['namespaceUid'] != namespace['uid']:
        raise Blocked('operation_namespace_changed')
    retained = _load_retained_journal()
    if (journal['retained'].get('journalSha256') != retained['journalSha256'] or
            journal['retained'].get('sourceSha') != retained['sourceSha'] or
            journal['retained'].get('templateSha256') != retained['templateSha256'] or
            journal['retained'].get('issuerImageId') != retained['issuerImageId']):
        raise Blocked('retained_operation_binding_changed')
    kube = Kubectl(namespace['uid'])
    ca = _retained_ca_object(kube)
    if (journal['retained'].get('caUid') != ca['uid'] or
            journal['retained'].get('caResourceVersion') != ca['resourceVersion'] or
            journal['retained'].get('fingerprint') != ca['fingerprint']):
        raise Blocked('retained_public_ca_binding_changed')
    _verify_retained_volume()
    _ensure_new_volume_absent_or_owned(journal['operationId'])
    if create:
        _targets_absent(kube)
    return namespace, retained, ca, kube


def _preflight(args):
    pins = _verify_source(args.expected_source_sha, args.expected_consumer_contract_sha256)
    _check_local_image(args.issuer_image, pins['sourceSha'])
    namespace = _namespace_identity(args.expected_namespace_uid)
    retained_journal = _load_retained_journal()
    _verify_retained_volume()
    retained_ca = _retained_ca_object(Kubectl(namespace['uid']))
    if NEW_VOLUME in _volume_list():
        raise Blocked('complete_match_issuer_volume_already_exists')
    _targets_absent(Kubectl(namespace['uid']))
    if STATE_PATH.exists() or STATE_PATH.is_symlink():
        directory = _private_dir(STATE_PATH, create=False)
        if list(directory.iterdir()):
            raise Blocked('complete_match_operation_state_already_exists')
    return {'namespace': namespace, 'sourceSha': pins['sourceSha'],
            'consumerContractSha256': pins['consumerContractSha256'], 'issuerImageId': args.issuer_image,
            'retainedOperationId': retained_journal['operationId'],
            'retainedJournalSha256': retained_journal['journalSha256'],
            'retainedCa': {key: retained_ca[key] for key in ('uid', 'resourceVersion', 'fingerprint')},
            'targets': sorted(TARGETS), 'newVolumeExpectedAbsent': NEW_VOLUME}


def _prepare(args, directory):
    pins = _verify_source(args.expected_source_sha, args.expected_consumer_contract_sha256)
    _check_local_image(args.issuer_image, pins['sourceSha'])
    namespace = _namespace_identity(args.expected_namespace_uid)
    retained = _load_retained_journal()
    _verify_retained_volume()
    kube = Kubectl(namespace['uid'])
    ca = _retained_ca_object(kube)
    if NEW_VOLUME in _volume_list():
        raise Blocked('complete_match_issuer_volume_already_exists')
    _targets_absent(kube)
    directory = _private_dir(directory, create=True)
    if {entry.name for entry in directory.iterdir()} != {'operator.lock'}:
        raise Blocked('complete_match_operation_journal_already_exists')
    operation_id = 'cmp-' + secrets.token_hex(16)
    journal = {'schema': 'voice-dev-complete-match-pki-operation-v1',
               'operationId': operation_id, 'namespace': NAMESPACE, 'namespaceUid': namespace['uid'],
               'sourceSha': pins['sourceSha'], 'consumerContractSha256': pins['consumerContractSha256'],
               'issuerImageId': args.issuer_image, 'volume': NEW_VOLUME,
               'retained': {**retained, 'caUid': ca['uid'], 'caResourceVersion': ca['resourceVersion'],
                            'fingerprint': ca['fingerprint']},
               'phase': 'initializing', 'resources': {name: {'phase': 'planned'} for name in sorted(TARGETS)},
               'stageReceipt': None}
    _save_journal(directory, journal)
    _ensure_new_volume_absent_or_owned(operation_id, create=True)
    journal['phase'] = 'stage-intent'
    _save_journal(directory, journal)
    output = _issuer_output(args.issuer_image, NEW_VOLUME, operation_id, pins['sourceSha'], 'stage', ca['certificate'])
    _record_stage(journal, output)
    _save_journal(directory, journal)
    raw, items = _read_export(journal, ca)
    del raw
    return public_receipt(items, journal)


def _resume(args, directory):
    directory = _private_dir(directory)
    journal = _load_journal(directory)
    _, retained, ca, kube = _check_operation_inputs(journal, args)
    if journal['phase'] == 'initializing':
        journal['phase'] = 'stage-intent'
        _save_journal(directory, journal)
    if journal['phase'] == 'stage-intent':
        output = _issuer_output(journal['issuerImageId'], NEW_VOLUME, journal['operationId'],
                                journal['sourceSha'], 'stage', ca['certificate'])
        _record_stage(journal, output)
        _save_journal(directory, journal)
    _, items = _read_export(journal, ca)
    if journal['phase'] == 'complete':
        for name, resource in journal['resources'].items():
            kind = TARGETS[name][0]
            current = kube.get(kind, name)
            if current is None or not _adopt_matches(items[name], current, journal['operationId']) or (
                    current['metadata']['uid'] != resource.get('uid') or
                    current['metadata']['resourceVersion'] != resource.get('resourceVersion')):
                raise Blocked('completed_target_changed')
        return public_receipt(items, journal)
    if journal['phase'] not in ('staged', 'apply-in-progress'):
        raise Blocked('operation_phase_not_resumable')
    journal['phase'] = 'apply-in-progress'
    _save_journal(directory, journal)
    for name in [CLIENT_CA, SERVER_SECRET, GATEWAY_SECRET]:
        _check_operation_inputs(journal, args)
        create_only(items[name], journal['resources'][name], journal,
                    lambda value: _save_journal(directory, value), kube)
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
        child.add_argument('--expected-consumer-contract-sha256', required=True)
        child.add_argument('--expected-namespace-uid', required=True)
        if command in {'prepare', 'apply', 'resume'}:
            child.add_argument('--confirm-dev-complete-match-pki-change', action='store_true')
    args = parser.parse_args(argv)
    if args.command in {'prepare', 'apply', 'resume'} and not args.confirm_dev_complete_match_pki_change:
        parser.error(f'{args.command} requires --confirm-dev-complete-match-pki-change')
    if sys.platform != 'linux':
        parser.error('CompleteMatch PKI operations require the designated Linux host')
    try:
        if args.command == 'preflight':
            print(json.dumps(_preflight(args), sort_keys=True))
            return 0
        directory = _private_dir(STATE_PATH, create=args.command == 'prepare')
        with invocation_lock(directory / 'operator.lock'):
            if args.command == 'prepare':
                receipt = _prepare(args, directory)
            elif args.command in {'apply', 'resume'}:
                receipt = _resume(args, directory)
            else:
                raise Blocked('command_invalid')
        print(json.dumps(receipt, sort_keys=True))
        return 0
    except (Blocked, OSError, json.JSONDecodeError, UnicodeDecodeError) as exc:
        parser.error(str(exc) if isinstance(exc, Blocked) else 'CompleteMatch PKI operation failed closed')


if __name__ == '__main__':
    raise SystemExit(main())
