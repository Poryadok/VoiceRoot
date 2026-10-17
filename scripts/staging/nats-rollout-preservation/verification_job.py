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
PHASE = 'census'
FIELDS = frozenset(('operation', 'nonce', 'phase'))
BINDING_FIELDS = frozenset(('source_claim', 'source_uid', 'verification_claim',
    'verification_uid', 'pv_name', 'pv_uid', 'node_name'))
IMAGE_RE = re.compile(r'[a-z0-9./_-]+@sha256:[a-f0-9]{64}')
UUID_RE = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')


class VerificationJobError(ValueError):
    pass


def _fail(code: str) -> None:
    raise VerificationJobError('verification_job_' + code)


def validate_request(value: object) -> dict:
    if (not isinstance(value, dict) or set(value) != FIELDS
        or value.get('phase') != PHASE
        or not isinstance(value.get('operation'), str)
        or not re.fullmatch(r'[a-f0-9]{12}', value['operation'])
        or not isinstance(value.get('nonce'), str)
        or not re.fullmatch(r'[a-f0-9]{32}', value['nonce'])):
        _fail('request_invalid')
    return copy.deepcopy(value)


def validate_binding(value: object) -> dict:
    if not isinstance(value, dict) or set(value) != BINDING_FIELDS:
        _fail('binding_invalid')
    for field in ('source_uid', 'verification_uid', 'pv_uid'):
        if not isinstance(value[field], str) or not UUID_RE.fullmatch(value[field]):
            _fail('storage_uid_invalid')
    if value['source_uid'] == value['verification_uid']:
        _fail('source_verification_same')
    for field in ('source_claim', 'verification_claim'):
        if (not isinstance(value[field], str)
            or not re.fullmatch(r'voice-nats-(?:jsdata|verify)-d[0-9]{8}[a-z0-9]{1,8}', value[field])):
            _fail('claim_name_invalid')
    if value['source_claim'] == value['verification_claim']:
        _fail('claim_name_same')
    for field in ('pv_name', 'node_name'):
        if not isinstance(value[field], str) or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', value[field]):
            _fail('storage_name_invalid')
    return copy.deepcopy(value)


def validate_image(value: object) -> str:
    if not isinstance(value, str) or not IMAGE_RE.fullmatch(value):
        _fail('image_not_digest_pinned')
    return value


def job_name(request: dict) -> str:
    row = validate_request(request)
    suffix = hashlib.sha256((row['operation'] + ':' + row['nonce']).encode()).hexdigest()[:16]
    return 'voice-nats-census-' + suffix


def document(request: dict, binding: dict, trusted_image: str) -> dict:
    row = validate_request(request)
    storage = validate_binding(binding)
    image = validate_image(trusted_image)
    name = job_name(row)
    labels = {LABEL_OPERATION: row['operation'], LABEL_NONCE: row['nonce'], LABEL_PHASE: PHASE}
    return {
        'apiVersion': 'batch/v1',
        'kind': 'Job',
        'metadata': {'name': name, 'namespace': NAMESPACE, 'labels': copy.deepcopy(labels)},
        'spec': {
            'backoffLimit': 0,
            'activeDeadlineSeconds': 14400,
            'manualSelector': True,
            'selector': {'matchLabels': {'voice.io/nats-census-job': name}},
            'template': {
                'metadata': {'labels': labels},
                'spec': {
                    'automountServiceAccountToken': False,
                    'serviceAccountName': 'default',
                    'nodeName': storage['node_name'],
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
                        'args': ['--serve', '--operation', row['operation'], '--nonce', row['nonce']],
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
                            {'name': 'verification', 'mountPath': '/verify'},
                            {'name': 'output', 'mountPath': '/out'},
                            {'name': 'inputs', 'mountPath': '/inputs'},
                        ],
                    }],
                    'volumes': [
                        {'name': 'source', 'persistentVolumeClaim': {
                            'claimName': storage['source_claim'], 'readOnly': True}},
                        {'name': 'verification', 'persistentVolumeClaim': {
                            'claimName': storage['verification_claim']}},
                        {'name': 'output', 'emptyDir': {'sizeLimit': '49Gi'}},
                        {'name': 'inputs', 'emptyDir': {'sizeLimit': '8Mi'}},
                    ],
                },
            },
        },
    }


def exec_vectors(request: dict, pod_uid: str, binding: dict) -> dict:
    """Return the only admitted in-band helper argv/query shapes for census."""
    row = validate_request(request)
    storage = validate_binding(binding)
    if not isinstance(pod_uid, str) or not UUID_RE.fullmatch(pod_uid):
        _fail('pod_uid_invalid')
    binary = ['/usr/bin/python3', '/voice-nats-runner/phase']
    common = ['--operation', row['operation'], '--nonce', row['nonce'], '--pod-uid', pod_uid]
    load = binary + ['--load-inputs'] + common
    census = binary + ['--phase', PHASE] + common + [
        '--source-pvc-uid', storage['source_uid'],
        '--verification-pvc-uid', storage['verification_uid']]
    return {'load_inputs': load, 'census': census}
