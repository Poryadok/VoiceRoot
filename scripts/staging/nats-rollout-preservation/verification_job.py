"""Closed native census Job ABI for the trusted STAGING runner.

The image digest and storage identities come from the installed runner's
trusted configuration and its current transaction, never from the app bundle.
The Job is only a tokenless holder for fixed ``pods/exec`` helper calls.
"""
from __future__ import annotations

import copy
import hashlib
import re


NAMESPACE = 'voice-staging'
LABEL_OPERATION = 'voice.io/nats-operation'
LABEL_NONCE = 'voice.io/nats-job-nonce'
LABEL_PHASE = 'voice.io/nats-job-phase'
LABEL_JOB_SELECTOR = 'voice.io/nats-phase-job'
PHASES = frozenset(('archive', 'census', 'bind-verification', 'bind-restore'))
VERIFICATION_BINDING_FIELDS = frozenset(('role', 'claim_name', 'target_uid', 'node_name'))
FIELDS = frozenset(('operation', 'nonce', 'phase'))
BINDING_FIELDS = frozenset((
    'source_claim', 'source_uid', 'source_pv_name', 'source_pv_uid', 'source_node_name',
    'verification_claim', 'verification_uid', 'verification_pv_name', 'verification_pv_uid',
    'verification_node_name'))
ARCHIVE_BINDING_FIELDS = frozenset((
    'source_claim', 'source_uid', 'source_pv_name', 'source_pv_uid', 'source_node_name'))
IMAGE_RE = re.compile(r'[a-z0-9./_-]+@sha256:[a-f0-9]{64}')
UUID_RE = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')


class VerificationJobError(ValueError):
    pass


def _fail(code: str) -> None:
    raise VerificationJobError('verification_job_' + code)


def validate_request(value: object) -> dict:
    if (not isinstance(value, dict) or set(value) != FIELDS
        or value.get('phase') not in PHASES
        or not isinstance(value.get('operation'), str)
        or not re.fullmatch(r'[a-f0-9]{12}', value['operation'])
        or not isinstance(value.get('nonce'), str)
        or not re.fullmatch(r'[a-f0-9]{32}', value['nonce'])):
        _fail('request_invalid')
    return copy.deepcopy(value)


def validate_binding(value: object) -> dict:
    if not isinstance(value, dict) or set(value) != BINDING_FIELDS:
        _fail('binding_invalid')
    for field in ('source_uid', 'verification_uid', 'source_pv_uid', 'verification_pv_uid'):
        if not isinstance(value[field], str) or not UUID_RE.fullmatch(value[field]):
            _fail('storage_uid_invalid')
    if (value['source_uid'] == value['verification_uid']
        or value['source_pv_uid'] == value['verification_pv_uid']
        or value['source_pv_name'] == value['verification_pv_name']):
        _fail('source_verification_same')
    if (not isinstance(value['source_claim'], str)
        or not re.fullmatch(r'voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}', value['source_claim'])):
        _fail('source_claim_invalid')
    if (not isinstance(value['verification_claim'], str)
        or not re.fullmatch(r'voice-nats-verify-[a-f0-9]{12}', value['verification_claim'])):
        _fail('verification_claim_invalid')
    if value['source_claim'] == value['verification_claim']:
        _fail('claim_name_same')
    for field in ('source_pv_name', 'verification_pv_name'):
        if not isinstance(value[field], str) or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', value[field]):
            _fail('storage_name_invalid')
    for field in ('source_node_name', 'verification_node_name'):
        if not isinstance(value[field], str) or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', value[field]):
            _fail('storage_name_invalid')
    if value['source_node_name'] != value['verification_node_name']:
        _fail('storage_node_mismatch')
    return copy.deepcopy(value)


def validate_archive_binding(value: object) -> dict:
    if not isinstance(value, dict) or set(value) != ARCHIVE_BINDING_FIELDS:
        _fail('archive_binding_invalid')
    for field in ('source_uid', 'source_pv_uid'):
        if not isinstance(value[field], str) or not UUID_RE.fullmatch(value[field]):
            _fail('storage_uid_invalid')
    if (not isinstance(value['source_claim'], str)
        or not re.fullmatch(r'voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}', value['source_claim'])):
        _fail('source_claim_invalid')
    for field in ('source_pv_name', 'source_node_name'):
        if not isinstance(value[field], str) or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', value[field]):
            _fail('storage_name_invalid')
    return copy.deepcopy(value)


def validate_image(value: object) -> str:
    if not isinstance(value, str) or not IMAGE_RE.fullmatch(value):
        _fail('image_not_digest_pinned')
    return value


def validate_binder_binding(request: dict, value: object) -> dict:
    row = validate_request(request)
    if not isinstance(value, dict) or set(value) != VERIFICATION_BINDING_FIELDS:
        _fail('binder_binding_invalid')
    expected_role = 'verification' if row['phase'] == 'bind-verification' else 'restore'
    if value.get('role') != expected_role:
        _fail('binder_role_invalid')
    if not isinstance(value.get('target_uid'), str) or not UUID_RE.fullmatch(value['target_uid']):
        _fail('binder_target_uid_invalid')
    if expected_role == 'verification':
        expected_claim = 'voice-nats-verify-' + row['operation']
        if value.get('claim_name') != expected_claim:
            _fail('binder_claim_invalid')
    elif (not isinstance(value.get('claim_name'), str)
        or not re.fullmatch(r'voice-nats-restore-' + row['operation'] + r'-[a-f0-9]{8}', value['claim_name'])):
        _fail('binder_claim_invalid')
    if (not isinstance(value.get('node_name'), str)
        or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', value['node_name'])):
        _fail('binder_node_invalid')
    return copy.deepcopy(value)


def job_name(request: dict) -> str:
    row = validate_request(request)
    suffix = hashlib.sha256((row['operation'] + ':' + row['nonce'] + ':' + row['phase']).encode()).hexdigest()[:16]
    return 'voice-nats-' + row['phase'] + '-' + suffix


def document(request: dict, binding: dict, trusted_image: str) -> dict:
    row = validate_request(request)
    image = validate_image(trusted_image)
    if row['phase'] in ('bind-verification', 'bind-restore'):
        storage = validate_binder_binding(row, binding)
        return _binder_document(row, storage, image)
    if row['phase'] == 'archive':
        return _archive_document(row, validate_archive_binding(binding), image)
    # Do not resurrect the former source+scratch co-mount. The census upload
    # contract is a separate target-only producer and remains fail-closed here.
    _fail('census_source_split_pending')


def _archive_document(request: dict, storage: dict, image: str) -> dict:
    name = job_name(request)
    labels = {LABEL_OPERATION: request['operation'], LABEL_NONCE: request['nonce'],
        LABEL_PHASE: 'archive', LABEL_JOB_SELECTOR: name}
    return {
        'apiVersion': 'batch/v1',
        'kind': 'Job',
        'metadata': {'name': name, 'namespace': NAMESPACE, 'labels': copy.deepcopy(labels)},
        'spec': {
            'backoffLimit': 0,
            'activeDeadlineSeconds': 14400,
            'manualSelector': True,
            'selector': {'matchLabels': {LABEL_JOB_SELECTOR: name}},
            'template': {
                'metadata': {'labels': labels},
                'spec': {
                    'automountServiceAccountToken': False,
                    'serviceAccountName': 'default',
                    'affinity': {'nodeAffinity': {'requiredDuringSchedulingIgnoredDuringExecution': {
                        'nodeSelectorTerms': [{'matchExpressions': [{'key': 'kubernetes.io/hostname',
                            'operator': 'In', 'values': [storage['source_node_name']]}]}]}}},
                    'restartPolicy': 'Never',
                    'securityContext': {
                        'runAsNonRoot': True,
                        'runAsUser': 10000,
                        'runAsGroup': 10000,
                        'seccompProfile': {'type': 'RuntimeDefault'},
                    },
                    'containers': [{
                        'name': 'nats-phase',
                        'image': image,
                        'imagePullPolicy': 'IfNotPresent',
                        'command': ['/usr/bin/python3', '/voice-nats-runner/phase'],
                        'args': ['--serve', '--operation', request['operation'], '--nonce', request['nonce'],
                            '--source-pvc-uid', storage['source_uid'], '--source-pv-uid', storage['source_pv_uid']],
                        'env': [{'name': 'POD_UID', 'valueFrom': {'fieldRef': {'fieldPath': 'metadata.uid'}}}],
                        'securityContext': {
                            'allowPrivilegeEscalation': False,
                            'readOnlyRootFilesystem': True,
                            'capabilities': {'drop': ['ALL']},
                        },
                        'resources': {
                            'requests': {'cpu': '500m', 'memory': '512Mi', 'ephemeral-storage': '49Gi'},
                            'limits': {'cpu': '2', 'memory': '2Gi', 'ephemeral-storage': '49Gi'},
                        },
                        'volumeMounts': [
                            {'name': 'source', 'mountPath': '/data', 'readOnly': True},
                            {'name': 'output', 'mountPath': '/out'},
                        ],
                    }],
                    'volumes': [
                        {'name': 'source', 'persistentVolumeClaim': {
                            'claimName': storage['source_claim'], 'readOnly': True}},
                        {'name': 'output', 'emptyDir': {'sizeLimit': '49Gi'}},
                    ],
                },
            },
        },
    }


def _binder_document(request: dict, binding: dict, image: str) -> dict:
    name = job_name(request)
    labels = {LABEL_OPERATION: request['operation'], LABEL_NONCE: request['nonce'],
        LABEL_PHASE: request['phase'], LABEL_JOB_SELECTOR: name}
    return {
        'apiVersion': 'batch/v1',
        'kind': 'Job',
        'metadata': {'name': name, 'namespace': NAMESPACE, 'labels': copy.deepcopy(labels)},
        'spec': {
            'backoffLimit': 0,
            'activeDeadlineSeconds': 600,
            'manualSelector': True,
            'selector': {'matchLabels': {LABEL_JOB_SELECTOR: name}},
            'template': {
                'metadata': {'labels': labels},
                'spec': {
                    'automountServiceAccountToken': False,
                    'serviceAccountName': 'default',
                    'restartPolicy': 'Never',
                    'securityContext': {
                        'runAsNonRoot': True,
                        'runAsUser': 65532,
                        'runAsGroup': 65532,
                        'seccompProfile': {'type': 'RuntimeDefault'},
                    },
                    'affinity': {'nodeAffinity': {'requiredDuringSchedulingIgnoredDuringExecution': {
                        'nodeSelectorTerms': [{'matchExpressions': [{'key': 'kubernetes.io/hostname',
                            'operator': 'In', 'values': [binding['node_name']]}]}]}}},
                    'containers': [{
                        'name': 'nats-phase',
                        'image': image,
                        'imagePullPolicy': 'IfNotPresent',
                        'command': ['/usr/bin/python3', '/voice-nats-runner/phase'],
                        'args': ['--bind-only', '--operation', request['operation'],
                            '--nonce', request['nonce'], '--role', binding['role']],
                        'env': [{'name': 'POD_UID', 'valueFrom': {'fieldRef': {'fieldPath': 'metadata.uid'}}}],
                        'securityContext': {
                            'allowPrivilegeEscalation': False,
                            'readOnlyRootFilesystem': True,
                            'capabilities': {'drop': ['ALL']},
                        },
                        'resources': {
                            'requests': {'cpu': '50m', 'memory': '32Mi', 'ephemeral-storage': '16Mi'},
                            'limits': {'cpu': '250m', 'memory': '128Mi', 'ephemeral-storage': '32Mi'},
                        },
                        'volumeMounts': [{'name': 'target', 'mountPath': '/verify', 'readOnly': True}],
                    }],
                    'volumes': [{'name': 'target', 'persistentVolumeClaim': {
                        'claimName': binding['claim_name'], 'readOnly': True}}],
                },
            },
        },
    }


def exec_vectors(request: dict, pod_uid: str, binding: dict) -> dict:
    """Return the only admitted in-band helper argv/query shapes for a fixed phase."""
    row = validate_request(request)
    if row['phase'] in ('bind-verification', 'bind-restore'):
        _fail('binder_has_no_exec')
    if not isinstance(pod_uid, str) or not UUID_RE.fullmatch(pod_uid):
        _fail('pod_uid_invalid')
    binary = ['/usr/bin/python3', '/voice-nats-runner/phase']
    common = ['--operation', row['operation'], '--nonce', row['nonce'], '--pod-uid', pod_uid]
    if row['phase'] == 'archive':
        storage = validate_archive_binding(binding)
        common += ['--source-pvc-uid', storage['source_uid'], '--source-pv-uid', storage['source_pv_uid']]
        return {
            'archive_create_and_stream': binary + ['--create-and-stream'] + common,
            'archive_same_pod_readback': binary + ['--stream-existing'] + common,
        }
    _fail('phase_transport_unsupported')
