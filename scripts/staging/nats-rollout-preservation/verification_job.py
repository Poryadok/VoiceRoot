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
PHASES = frozenset(('archive', 'census', 'bind-verification', 'verification-session'))
VERIFICATION_BINDING_FIELDS = frozenset(('role', 'claim_name', 'target_uid', 'node_name'))
FIELDS = frozenset(('operation', 'nonce', 'phase'))
BINDING_FIELDS = frozenset((
    'source_claim', 'source_uid', 'source_pv_name', 'source_pv_uid', 'source_node_name',
    'verification_claim', 'verification_uid', 'verification_pv_name', 'verification_pv_uid',
    'verification_node_name'))
ARCHIVE_BINDING_FIELDS = frozenset((
    'source_claim', 'source_uid', 'source_pv_name', 'source_pv_uid', 'source_node_name'))
SELECTED_BINDING_FIELDS = BINDING_FIELDS | frozenset(('session_attempt', 'input_pod_uid',
    'input_bundle_sha256', 'input_bundle_bytes', 'kernel_bundle_sha256',
    'server_config_sha256', 'archive_sha256', 'archive_bytes'))
SELECTED_ACTIONS = frozenset((
    'bootstrap-realtime', 'bootstrap-notification', 'bootstrap-analytics-chat',
    'bootstrap-search', 'kernel-census', 'kernel-seed', 'kernel-verify-closed', 'kernel-drain',
    'kernel-verify-drained', 'broker-ready', 'broker-restart-prepare',
    'broker-restart-complete', 'session-close', 'session-stop'))
KERNEL_PHASES = {
    'kernel-census': 'census', 'kernel-seed': 'seed', 'kernel-verify-closed': 'verify-closed',
    'kernel-drain': 'drain', 'kernel-verify-drained': 'verify-drained'}
IMAGE_RE = re.compile(r'[a-z0-9./_-]+@sha256:[a-f0-9]{64}')
UUID_RE = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
_API_METADATA = frozenset(('uid', 'resourceVersion', 'generation', 'creationTimestamp', 'managedFields'))
_JOB_DEFAULTS = {
    ('spec', 'parallelism'): 1,
    ('spec', 'completions'): 1,
    ('spec', 'completionMode'): 'NonIndexed',
    ('spec', 'suspend'): False,
    ('spec', 'podReplacementPolicy'): 'TerminatingOrFailed',
    ('spec', 'template', 'spec', 'dnsPolicy'): 'ClusterFirst',
    ('spec', 'template', 'spec', 'schedulerName'): 'default-scheduler',
    ('spec', 'template', 'spec', 'terminationGracePeriodSeconds'): 30,
    ('spec', 'template', 'spec', 'enableServiceLinks'): True,
    ('spec', 'template', 'spec', 'preemptionPolicy'): 'PreemptLowerPriority',
    ('spec', 'template', 'spec', 'priority'): 0,
    ('spec', 'template', 'spec', 'containers', 0, 'terminationMessagePath'): '/dev/termination-log',
    ('spec', 'template', 'spec', 'containers', 0, 'terminationMessagePolicy'): 'File',
    ('spec', 'template', 'spec', 'containers', 1, 'terminationMessagePath'): '/dev/termination-log',
    ('spec', 'template', 'spec', 'containers', 1, 'terminationMessagePolicy'): 'File',
    ('spec', 'template', 'spec', 'initContainers', 0, 'terminationMessagePath'): '/dev/termination-log',
    ('spec', 'template', 'spec', 'initContainers', 0, 'terminationMessagePolicy'): 'File',
}
_POD_DEFAULTS = {path[3:]: value
    for path, value in _JOB_DEFAULTS.items() if path[:2] == ('spec', 'template')}
_CONTROLLER_LABELS = frozenset((
    'batch.kubernetes.io/controller-uid', 'batch.kubernetes.io/job-name',
    'controller-uid', 'job-name'))


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


def validate_selected_binding(value: object) -> dict:
    if not isinstance(value, dict) or set(value) != SELECTED_BINDING_FIELDS:
        _fail('selected_binding_invalid')
    storage = validate_binding({key: value[key] for key in BINDING_FIELDS})
    attempt = value.get('session_attempt')
    if not isinstance(attempt, str) or not re.fullmatch(r'[a-f0-9]{8}', attempt):
        _fail('selected_attempt_invalid')
    for field in ('input_pod_uid',):
        if not isinstance(value[field], str) or not UUID_RE.fullmatch(value[field]):
            _fail('selected_input_uid_invalid')
    for field in ('input_bundle_sha256', 'kernel_bundle_sha256', 'server_config_sha256', 'archive_sha256'):
        if not isinstance(value[field], str) or not re.fullmatch(r'[a-f0-9]{64}', value[field]):
            _fail('selected_input_digest_invalid')
    if (type(value['input_bundle_bytes']) is not int or not 0 < value['input_bundle_bytes'] <= (4 << 20)
        or type(value['archive_bytes']) is not int
        or not 0 < value['archive_bytes'] <= (48 << 30) + (262144 * 1024) + (1 << 20)):
        _fail('selected_input_size_invalid')
    return storage | {key: value[key] for key in SELECTED_BINDING_FIELDS - BINDING_FIELDS}


def validate_image(value: object) -> str:
    if not isinstance(value, str) or not IMAGE_RE.fullmatch(value):
        _fail('image_not_digest_pinned')
    return value


def validate_binder_binding(request: dict, value: object) -> dict:
    row = validate_request(request)
    if not isinstance(value, dict) or set(value) != VERIFICATION_BINDING_FIELDS:
        _fail('binder_binding_invalid')
    if value.get('role') != 'verification':
        _fail('binder_role_invalid')
    if not isinstance(value.get('target_uid'), str) or not UUID_RE.fullmatch(value['target_uid']):
        _fail('binder_target_uid_invalid')
    expected_claim = 'voice-nats-verify-' + row['operation']
    if value.get('claim_name') != expected_claim:
        _fail('binder_claim_invalid')
    if (not isinstance(value.get('node_name'), str)
        or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', value['node_name'])):
        _fail('binder_node_invalid')
    return copy.deepcopy(value)


def job_name(request: dict) -> str:
    row = validate_request(request)
    suffix = hashlib.sha256((row['operation'] + ':' + row['nonce'] + ':' + row['phase']).encode()).hexdigest()[:16]
    return 'voice-nats-' + row['phase'] + '-' + suffix


def _compare_api_projection(actual: object, wanted: object, path: tuple = (), defaults=None) -> None:
    """Require an API object to match its immutable producer template.

    Only documented Kubernetes defaults in the phase allowlist are accepted;
    admission-injected containers, mounts, env, labels, or security fields fail.
    """
    defaults = _JOB_DEFAULTS if defaults is None else defaults
    if isinstance(wanted, dict):
        if not isinstance(actual, dict) or not set(wanted).issubset(actual):
            _fail('api_object_shape')
        for key, value in wanted.items():
            _compare_api_projection(actual[key], value, path + (key,), defaults)
        extras = set(actual) - set(wanted)
        for key in extras:
            default = defaults.get(path + (key,), None)
            if default is None or actual[key] != default:
                _fail('api_object_extra_field')
        return
    if isinstance(wanted, list):
        if not isinstance(actual, list) or len(actual) != len(wanted):
            _fail('api_object_array')
        for index, (actual_row, wanted_row) in enumerate(zip(actual, wanted)):
            _compare_api_projection(actual_row, wanted_row, path + (index,), defaults)
        return
    if actual != wanted or type(actual) is not type(wanted):
        _fail('api_object_value')


def _api_metadata(actual: object, wanted: dict, *, pod=False) -> dict:
    if not isinstance(actual, dict):
        _fail('api_metadata_invalid')
    projection = copy.deepcopy(actual)
    for key in _API_METADATA:
        projection.pop(key, None)
    if pod:
        projection.pop('ownerReferences', None)
        namespace = projection.pop('namespace', None)
        if namespace not in (None, NAMESPACE):
            _fail('api_pod_namespace')
        name = projection.get('name', '')
        expected_name = wanted.get('labels', {}).get(LABEL_JOB_SELECTOR, '')
        if (not isinstance(name, str) or not expected_name
            or not re.fullmatch(re.escape(expected_name) + r'-[a-z0-9]{5}', name)):
            _fail('api_pod_name')
        projection.pop('name', None)
        projection.pop('generateName', None)
        labels = projection.get('labels')
        if not isinstance(labels, dict):
            _fail('api_pod_labels')
        expected_labels = wanted.get('labels', {})
        controller = {key: value for key, value in labels.items() if key in _CONTROLLER_LABELS}
        if set(labels) - set(expected_labels) - set(_CONTROLLER_LABELS):
            _fail('api_pod_label_extra')
        for key, value in expected_labels.items():
            if labels.get(key) != value:
                _fail('api_pod_label_binding')
        if controller:
            job = expected_labels.get(LABEL_JOB_SELECTOR)
            expected_controller = {
                'batch.kubernetes.io/job-name': job,
                'job-name': job,
            }
            for key, value in controller.items():
                if key in ('batch.kubernetes.io/controller-uid', 'controller-uid'):
                    if not isinstance(value, str) or not UUID_RE.fullmatch(value):
                        _fail('api_pod_controller_label')
                elif value != expected_controller[key]:
                    _fail('api_pod_controller_label')
        projection['labels'] = expected_labels
    else:
        for key in _API_METADATA:
            value = actual.get(key)
            if value is not None and key == 'uid' and (not isinstance(value, str) or not UUID_RE.fullmatch(value)):
                _fail('api_job_uid')
        extra = set(projection) - set(wanted)
        if extra:
            _fail('api_metadata_extra_field')
    _compare_api_projection(projection, wanted, defaults={})
    return actual


def verify_job(actual: object, wanted: dict, expected_uid: str | None = None) -> dict:
    """Validate a created/read-back Job against its exact producer document."""
    if not isinstance(wanted, dict):
        _fail('api_job_template')
    request = {'operation': wanted.get('metadata', {}).get('labels', {}).get(LABEL_OPERATION),
        'nonce': wanted.get('metadata', {}).get('labels', {}).get(LABEL_NONCE),
        'phase': wanted.get('metadata', {}).get('labels', {}).get(LABEL_PHASE)}
    validate_request(request)
    if (not isinstance(actual, dict) or actual.get('apiVersion') != 'batch/v1'
        or actual.get('kind') != 'Job' or not isinstance(actual.get('metadata'), dict)
        or not isinstance(actual.get('spec'), dict)):
        _fail('api_job_invalid')
    uid = actual['metadata'].get('uid')
    if (not isinstance(uid, str) or not UUID_RE.fullmatch(uid)
        or (expected_uid is not None and uid != expected_uid)):
        _fail('api_job_uid')
    metadata = copy.deepcopy(actual['metadata'])
    for key in _API_METADATA:
        metadata.pop(key, None)
    status = actual.get('status', {})
    if not isinstance(status, dict) or set(status) - {'active', 'succeeded', 'failed', 'conditions',
        'ready', 'terminating', 'uncountedTerminatedPods', 'startTime', 'completionTime',
        'completedIndexes', 'failedIndexes'}:
        _fail('api_job_status')
    projection = {'apiVersion': actual['apiVersion'], 'kind': actual['kind'],
        'metadata': metadata, 'spec': actual['spec']}
    _compare_api_projection(projection, wanted, defaults=_JOB_DEFAULTS)
    return copy.deepcopy(actual)


def verify_pod(actual: object, wanted_template: dict, job_uid: str, phase: str,
               expected_node_name: str | None = None, expected_uid: str | None = None) -> dict:
    """Validate one controller-owned Pod and its scheduled node against the Job template."""
    if phase not in PHASES or not isinstance(wanted_template, dict):
        _fail('api_pod_template')
    if not isinstance(job_uid, str) or not UUID_RE.fullmatch(job_uid):
        _fail('api_pod_owner')
    request = {'operation': wanted_template.get('metadata', {}).get('labels', {}).get(LABEL_OPERATION),
        'nonce': wanted_template.get('metadata', {}).get('labels', {}).get(LABEL_NONCE),
        'phase': phase}
    validate_request(request)
    if (not isinstance(actual, dict) or actual.get('apiVersion') != 'v1'
        or actual.get('kind') != 'Pod' or not isinstance(actual.get('metadata'), dict)
        or not isinstance(actual.get('spec'), dict)):
        _fail('api_pod_invalid')
    uid = actual['metadata'].get('uid')
    if (not isinstance(uid, str) or not UUID_RE.fullmatch(uid)
        or (expected_uid is not None and uid != expected_uid)):
        _fail('api_pod_uid')
    wanted_metadata = wanted_template.get('metadata', {})
    wanted_spec = wanted_template.get('spec', {})
    metadata = copy.deepcopy(actual['metadata'])
    owners = metadata.pop('ownerReferences', None)
    if (not isinstance(owners, list) or len(owners) != 1 or owners[0] != {
        'apiVersion': 'batch/v1', 'kind': 'Job', 'name': wanted_metadata['labels'][LABEL_JOB_SELECTOR],
        'uid': job_uid, 'controller': True, 'blockOwnerDeletion': True}):
        _fail('api_pod_owner')
    labels = metadata.get('labels')
    if not isinstance(labels, dict):
        _fail('api_pod_labels')
    job_name_value = wanted_metadata['labels'][LABEL_JOB_SELECTOR]
    for key in ('batch.kubernetes.io/controller-uid', 'controller-uid'):
        if key in labels and labels[key] != job_uid:
            _fail('api_pod_controller_label')
    for key in ('batch.kubernetes.io/job-name', 'job-name'):
        if key in labels and labels[key] != job_name_value:
            _fail('api_pod_controller_label')
    # API server metadata and Job-controller labels are checked separately.
    _api_metadata(metadata, wanted_metadata, pod=True)
    pod_spec = copy.deepcopy(actual['spec'])
    scheduled = pod_spec.pop('nodeName', None)
    if scheduled is not None:
        if (not isinstance(expected_node_name, str)
            or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', expected_node_name)
            or scheduled != expected_node_name):
            _fail('api_pod_node')
    _compare_api_projection(pod_spec, wanted_spec, defaults=_POD_DEFAULTS)
    return copy.deepcopy(actual)


def document(request: dict, binding: dict, trusted_image: str) -> dict:
    row = validate_request(request)
    image = validate_image(trusted_image)
    if row['phase'] == 'bind-verification':
        storage = validate_binder_binding(row, binding)
        return _binder_document(row, storage, image)
    if row['phase'] == 'archive':
        return _archive_document(row, validate_archive_binding(binding), image)
    if row['phase'] == 'census':
        return _census_document(row, validate_binding(binding), image)
    if row['phase'] == 'verification-session':
        return _selected_store_document(row, validate_selected_binding(binding), image)
    _fail('phase_unsupported')


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


def _census_document(request: dict, storage: dict, image: str) -> dict:
    """Run a disposable census on its own target PVC; the selected source is metadata only."""
    name = job_name(request)
    labels = {LABEL_OPERATION: request['operation'], LABEL_NONCE: request['nonce'],
        LABEL_PHASE: 'census', LABEL_JOB_SELECTOR: name}
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
                            'operator': 'In', 'values': [storage['verification_node_name']]}]}]}}},
                    'restartPolicy': 'Never',
                    'securityContext': {
                        'runAsNonRoot': True,
                        'runAsUser': 65532,
                        'runAsGroup': 65532,
                        'fsGroup': 65532,
                        'fsGroupChangePolicy': 'OnRootMismatch',
                        'seccompProfile': {'type': 'RuntimeDefault'},
                    },
                    'containers': [{
                        'name': 'nats-phase',
                        'image': image,
                        'imagePullPolicy': 'IfNotPresent',
                        'command': ['/usr/bin/python3', '/voice-nats-runner/phase'],
                        'args': ['--serve-census', '--operation', request['operation'], '--nonce', request['nonce']],
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
                            {'name': 'target', 'mountPath': '/verify'},
                            {'name': 'output', 'mountPath': '/out'},
                            {'name': 'inputs', 'mountPath': '/inputs'},
                        ],
                    }],
                    'volumes': [
                        {'name': 'target', 'persistentVolumeClaim': {
                            'claimName': storage['verification_claim']}},
                        {'name': 'output', 'emptyDir': {'sizeLimit': '49Gi'}},
                        {'name': 'inputs', 'emptyDir': {'sizeLimit': '16Mi'}},
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


def _selected_store_document(request: dict, storage: dict, image: str) -> dict:
    """Create a scratch-only broker session; source IDs are metadata, never mounts."""
    name = job_name(request)
    labels = {LABEL_OPERATION: request['operation'], LABEL_NONCE: request['nonce'],
        LABEL_PHASE: 'verification-session', LABEL_JOB_SELECTOR: name}
    common = ['--operation', request['operation'], '--nonce', request['nonce'],
        '--session-attempt', storage['session_attempt'], '--source-pvc-uid', storage['source_uid'],
        '--source-pv-uid', storage['source_pv_uid'], '--verification-pvc-uid', storage['verification_uid'],
        '--verification-pv-uid', storage['verification_pv_uid'],
        '--input-pod-uid', storage['input_pod_uid'],
        '--input-bundle-sha256', storage['input_bundle_sha256'],
        '--input-bundle-bytes', str(storage['input_bundle_bytes']),
        '--kernel-bundle-sha256', storage['kernel_bundle_sha256'],
        '--server-config-sha256', storage['server_config_sha256'],
        '--archive-sha256', storage['archive_sha256'], '--archive-bytes', str(storage['archive_bytes'])]
    return {
        'apiVersion': 'batch/v1', 'kind': 'Job',
        'metadata': {'name': name, 'namespace': NAMESPACE, 'labels': copy.deepcopy(labels)},
        'spec': {
            'backoffLimit': 0, 'activeDeadlineSeconds': 14400, 'manualSelector': True,
            'podReplacementPolicy': 'Never',
            'selector': {'matchLabels': {LABEL_JOB_SELECTOR: name}},
            'template': {'metadata': {'labels': labels}, 'spec': {
                'automountServiceAccountToken': False, 'serviceAccountName': 'default',
                'affinity': {'nodeAffinity': {'requiredDuringSchedulingIgnoredDuringExecution': {
                    'nodeSelectorTerms': [{'matchExpressions': [{'key': 'kubernetes.io/hostname',
                        'operator': 'In', 'values': [storage['source_node_name']]}]}]}}},
                'restartPolicy': 'Never',
                'securityContext': {'runAsNonRoot': True, 'runAsUser': 65532, 'runAsGroup': 65532,
                    'fsGroup': 65532, 'fsGroupChangePolicy': 'OnRootMismatch',
                    'seccompProfile': {'type': 'RuntimeDefault'}},
                'initContainers': [{
                    'name': 'nats', 'restartPolicy': 'Always', 'image': image,
                    'imagePullPolicy': 'IfNotPresent',
                    'command': ['/usr/bin/python3', '/voice-nats-runner/nats-supervisor'],
                    'args': [],
                    'securityContext': {'runAsNonRoot': True, 'runAsUser': 65532, 'runAsGroup': 65532,
                        'allowPrivilegeEscalation': False, 'readOnlyRootFilesystem': True,
                        'capabilities': {'drop': ['ALL']},
                        'seccompProfile': {'type': 'RuntimeDefault'}},
                    'resources': {'requests': {'cpu': '500m', 'memory': '512Mi', 'ephemeral-storage': '1Gi'},
                        'limits': {'cpu': '2', 'memory': '2Gi', 'ephemeral-storage': '2Gi'}},
                    'volumeMounts': [
                        {'name': 'scratch', 'mountPath': '/data', 'subPath': 'store'},
                        {'name': 'scratch', 'mountPath': '/server.conf', 'subPath': 'inputs/public/server.conf',
                            'readOnly': True},
                    ],
                }],
                'containers': [{
                    'name': 'control', 'image': image, 'imagePullPolicy': 'IfNotPresent',
                    'command': ['/usr/bin/python3', '/voice-nats-runner/phase'],
                    'args': ['--serve-selected-session'] + common,
                    'env': [{'name': 'POD_UID', 'valueFrom': {'fieldRef': {'fieldPath': 'metadata.uid'}}}],
                    'securityContext': {'allowPrivilegeEscalation': False, 'readOnlyRootFilesystem': True,
                        'capabilities': {'drop': ['ALL']}},
                    'resources': {'requests': {'cpu': '250m', 'memory': '256Mi', 'ephemeral-storage': '256Mi'},
                        'limits': {'cpu': '1', 'memory': '1Gi', 'ephemeral-storage': '512Mi'}},
                    'volumeMounts': [
                        {'name': 'scratch', 'mountPath': '/inputs', 'subPath': 'kernelinputbundle', 'readOnly': True},
                        {'name': 'scratch', 'mountPath': '/state',
                            'subPath': 'state/' + request['operation'] + '/' + storage['session_attempt']},
                        {'name': 'output', 'mountPath': '/out'},
                    ],
                }],
                'volumes': [
                    {'name': 'scratch', 'persistentVolumeClaim': {'claimName': storage['verification_claim']}},
                    {'name': 'output', 'emptyDir': {'sizeLimit': '256Mi'}},
                ],
            }},
        },
    }


def exec_vectors(request: dict, pod_uid: str, binding: dict, archive: dict | None = None,
        action: str | None = None, session_inputs: dict | None = None) -> dict:
    """Return the only admitted in-band helper argv/query shapes for a fixed phase."""
    row = validate_request(request)
    if row['phase'] == 'bind-verification':
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
    if row['phase'] == 'census':
        storage = validate_binding(binding)
        if (not isinstance(archive, dict) or set(archive) != {'sha256', 'bytes'}
            or not isinstance(archive['sha256'], str)
            or not re.fullmatch(r'[a-f0-9]{64}', archive['sha256'])
            or type(archive['bytes']) is not int or not 0 < archive['bytes'] <= (48 << 30) + (262144 * 1024) + (1 << 20)):
            _fail('archive_input_invalid')
        if (not isinstance(session_inputs, dict) or set(session_inputs) != {'attempt', 'sha256', 'bytes'}
            or not isinstance(session_inputs['attempt'], str)
            or not re.fullmatch(r'[a-f0-9]{8}', session_inputs['attempt'])
            or not isinstance(session_inputs['sha256'], str)
            or not re.fullmatch(r'[a-f0-9]{64}', session_inputs['sha256'])
            or type(session_inputs['bytes']) is not int
            or not 0 < session_inputs['bytes'] <= (4 << 20)):
            _fail('session_inputs_invalid')
        common += ['--source-pvc-uid', storage['source_uid'], '--source-pv-uid', storage['source_pv_uid'],
            '--verification-pvc-uid', storage['verification_uid'],
            '--verification-pv-uid', storage['verification_pv_uid'],
            '--archive-sha256', archive['sha256'], '--archive-bytes', str(archive['bytes']),
            '--session-attempt', session_inputs['attempt'],
            '--input-bundle-sha256', session_inputs['sha256'],
            '--input-bundle-bytes', str(session_inputs['bytes'])]
        return {
            'census_load_inputs': binary + ['--load-inputs'] + common,
            'census_read_input_receipt': binary + ['--read-input-receipt'] + common,
            'census_receive_archive': binary + ['--receive-archive'] + common,
            'census_same_pod_readback': binary + ['--read-receipt'] + common + ['--phase', 'census'],
        }
    if row['phase'] == 'verification-session':
        storage = validate_selected_binding(binding)
        if action is not None and action not in SELECTED_ACTIONS:
            _fail('selected_action_invalid')
        session = common + ['--session-attempt', storage['session_attempt'],
            '--source-pvc-uid', storage['source_uid'], '--source-pv-uid', storage['source_pv_uid'],
            '--verification-pvc-uid', storage['verification_uid'],
            '--verification-pv-uid', storage['verification_pv_uid'],
            '--input-pod-uid', storage['input_pod_uid'],
            '--input-bundle-sha256', storage['input_bundle_sha256'],
            '--input-bundle-bytes', str(storage['input_bundle_bytes']),
            '--kernel-bundle-sha256', storage['kernel_bundle_sha256'],
            '--server-config-sha256', storage['server_config_sha256'],
            '--archive-sha256', storage['archive_sha256'], '--archive-bytes', str(storage['archive_bytes'])]
        names = ('bootstrap-realtime', 'bootstrap-notification', 'bootstrap-analytics-chat',
            'bootstrap-search', 'kernel-census', 'kernel-seed', 'kernel-verify-closed', 'kernel-drain',
            'broker-ready', 'broker-restart-prepare', 'broker-restart-complete',
            'kernel-verify-drained', 'session-close')
        if action is not None:
            action_vector = binary + ['--session-action', action] + session
            if action == 'session-stop':
                return {'action': action_vector}
            return {
                'action': action_vector,
                'readback': binary + ['--session-readback', '--action', action] + session,
            }
        vectors = {
            'actions': {name: binary + ['--session-action', name] + session for name in names},
            'readbacks': {name: binary + ['--session-readback', '--action', name] + session for name in names},
            'nats_health': ['/bin/busybox', 'wget', '-q', '-O', '-', 'http://127.0.0.1:8222/healthz'],
            'session_close': binary + ['--session-action', 'session-close'] + session,
            'session_close_readback': binary + ['--session-readback', '--action', 'session-close'] + session,
            'session_stop': binary + ['--session-action', 'session-stop'] + session,
        }
        return vectors
    _fail('phase_transport_unsupported')


def selected_exec_contracts(request: dict, pod_uid: str, binding: dict) -> dict:
    """Return exact Kubernetes exec parameters; callers cannot vary stream flags or container."""
    row = validate_request(request)
    if row['phase'] != 'verification-session':
        _fail('selected_phase_required')
    vectors = exec_vectors(row, pod_uid, binding)
    contracts = {}
    for name, command in vectors['actions'].items():
        contracts['action:' + name] = {'container': 'control', 'command': command,
            'stdin': False, 'stdout': True, 'stderr': False, 'tty': False}
    for name, command in vectors['readbacks'].items():
        contracts['readback:' + name] = {'container': 'control', 'command': command,
            'stdin': False, 'stdout': True, 'stderr': False, 'tty': False}
    contracts['nats-health'] = {'container': 'nats', 'command': vectors['nats_health'],
        'stdin': False, 'stdout': True, 'stderr': False, 'tty': False}
    contracts['session-close'] = {'container': 'control', 'command': vectors['session_close'],
        'stdin': False, 'stdout': True, 'stderr': False, 'tty': False}
    contracts['session-stop'] = {'container': 'control', 'command': vectors['session_stop'],
        'stdin': False, 'stdout': True, 'stderr': False, 'tty': False}
    return contracts
