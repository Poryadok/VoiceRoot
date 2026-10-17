"""Fixed tokenless PVC Job protocol for the trusted STAGING runner.

The submitter selects only a phase from PHASES. Every container image, command,
volume, security setting, label, timeout, and cleanup decision is runner-owned.
"""
from __future__ import annotations

import copy
import hashlib
import json
import re
import time


NAMESPACE = 'voice-staging'
LABEL_OPERATION = 'voice.io/nats-operation'
LABEL_NONCE = 'voice.io/nats-job-nonce'
LABEL_PHASE = 'voice.io/nats-job-phase'
PHASES = frozenset(('seed', 'verify-closed', 'drain', 'verify-drained', 'census'))
RECEIPT_SCHEMA = 'voice-nats-tokenless-phase-receipt-v1'
FIELDS = frozenset(('operation', 'nonce', 'phase'))
BINDING_FIELDS = frozenset(('claim_name', 'claim_uid', 'pv_name', 'pv_uid', 'node_name'))


class JobError(ValueError):
    pass


def _fail(code: str) -> None:
    raise JobError('nats_job_' + code)


def validate_request(row: object) -> dict:
    if not isinstance(row, dict) or set(row) != FIELDS:
        _fail('request_shape_invalid')
    if not isinstance(row['operation'], str) or not re.fullmatch('[a-f0-9]{12}', row['operation']):
        _fail('operation_invalid')
    if not isinstance(row['nonce'], str) or not re.fullmatch('[a-f0-9]{32}', row['nonce']):
        _fail('nonce_invalid')
    if row['phase'] not in PHASES:
        _fail('phase_invalid')
    return copy.deepcopy(row)


def validate_binding(value: object) -> dict:
    if not isinstance(value, dict) or set(value) != BINDING_FIELDS:
        _fail('storage_binding_invalid')
    if not isinstance(value['claim_name'], str) or not re.fullmatch('voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}', value['claim_name']):
        _fail('claim_invalid')
    for field in ('claim_uid', 'pv_uid'):
        if not isinstance(value[field], str) or not re.fullmatch('[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', value[field]):
            _fail('storage_identity_invalid')
    for field in ('pv_name', 'node_name'):
        if not isinstance(value[field], str) or not re.fullmatch('[a-z0-9][a-z0-9.-]{0,252}', value[field]):
            _fail('storage_identity_invalid')
    return copy.deepcopy(value)


def validate_runner_image(value: object) -> str:
    if not isinstance(value, str) or not re.fullmatch(r'[a-z0-9./_-]+@sha256:[a-f0-9]{64}', value):
        _fail('runner_image_not_pinned')
    return value


def job_name(request: dict) -> str:
    row = validate_request(request)
    suffix = hashlib.sha256((row['operation'] + ':' + row['nonce']).encode()).hexdigest()[:12]
    return 'voice-nats-rollout-' + row['phase'].replace('-', '') + '-' + suffix


def document(request: dict, binding: dict, runner_image: str) -> dict:
    row = validate_request(request)
    storage = validate_binding(binding)
    image = validate_runner_image(runner_image)
    name = job_name(row)
    return {
        'apiVersion': 'batch/v1',
        'kind': 'Job',
        'metadata': {
            'name': name,
            'namespace': NAMESPACE,
            'labels': {LABEL_OPERATION: row['operation'], LABEL_NONCE: row['nonce'], LABEL_PHASE: row['phase']},
        },
        'spec': {
            'backoffLimit': 0,
            'activeDeadlineSeconds': 120,
            'ttlSecondsAfterFinished': 600,
            'template': {
                'metadata': {'labels': {LABEL_OPERATION: row['operation'], LABEL_NONCE: row['nonce'], LABEL_PHASE: row['phase']}},
                'spec': {
                    'automountServiceAccountToken': False,
            'nodeName': storage['node_name'],
                    'serviceAccountName': 'default',
                    'restartPolicy': 'Never',
                    'securityContext': {'runAsNonRoot': True, 'runAsUser': 65532, 'runAsGroup': 65532, 'seccompProfile': {'type': 'RuntimeDefault'}},
                    'containers': [{
                        'name': 'nats-phase',
                        'image': image,
                        'imagePullPolicy': 'IfNotPresent',
                        'command': ['/voice-nats-runner/phase'],
                        'args': ['--phase', row['phase'], '--nonce', row['nonce']],
                        'securityContext': {'allowPrivilegeEscalation': False, 'readOnlyRootFilesystem': True, 'capabilities': {'drop': ['ALL']}},
                        'resources': {'requests': {'cpu': '10m', 'memory': '16Mi'}, 'limits': {'cpu': '1', 'memory': '512Mi'}},
                        'volumeMounts': [{'name': 'data', 'mountPath': '/data'}],
                    }],
                    'volumes': [{'name': 'data', 'persistentVolumeClaim': {'claimName': storage['claim_name'], 'readOnly': False}}],
                },
            },
        },
    }


def verify_job(actual: object, wanted: dict, *, created_uid: str | None = None) -> dict:
    if not isinstance(actual, dict):
        _fail('readback_invalid')
    expected = wanted['metadata']
    metadata = actual.get('metadata', {})
    if (actual.get('apiVersion') != 'batch/v1' or actual.get('kind') != 'Job'
        or metadata.get('name') != expected['name'] or metadata.get('namespace') != NAMESPACE
        or metadata.get('labels') != expected['labels']
        or not isinstance(metadata.get('uid'), str) or not re.fullmatch('[a-f0-9-]{36}', metadata['uid'])
        or created_uid is not None and metadata['uid'] != created_uid):
        _fail('job_identity_changed')
    if set(actual) - {'apiVersion', 'kind', 'metadata', 'spec', 'status'}:
        _fail('job_shape_invalid')
    if set(metadata) - {'name', 'namespace', 'labels', 'uid', 'resourceVersion', 'generation', 'creationTimestamp', 'managedFields'}:
        _fail('job_metadata_shape_invalid')
    if (not isinstance(metadata.get('resourceVersion'), str) or not metadata['resourceVersion'].isdigit()
        or type(metadata.get('generation')) is not int or metadata['generation'] < 1):
        _fail('job_metadata_invalid')
    spec = copy.deepcopy(actual.get('spec', {}))
    template = spec.get('template')
    if not isinstance(template, dict): _fail('job_template_changed')
    job_defaults = {'completionMode': 'NonIndexed', 'completions': 1, 'manualSelector': False,
        'parallelism': 1, 'podReplacementPolicy': 'TerminatingOrFailed', 'suspend': False}
    for field, value in job_defaults.items():
        if field in spec:
            if spec[field] != value: _fail('job_default_changed')
            del spec[field]
    selector = spec.pop('selector', None)
    if selector != {'matchLabels': {'batch.kubernetes.io/controller-uid': metadata['uid']}}:
        _fail('job_selector_invalid')
    template_metadata = template.get('metadata', {})
    labels = template_metadata.get('labels', {})
    expected_labels = copy.deepcopy(wanted['spec']['template']['metadata']['labels'])
    controller_labels = {'batch.kubernetes.io/controller-uid': metadata['uid'],
        'batch.kubernetes.io/job-name': expected['name'], 'controller-uid': metadata['uid'],
        'job-name': expected['name']}
    for key, value in controller_labels.items():
        if labels.get(key) != value: _fail('job_controller_label_invalid')
    if set(labels) != set(expected_labels) | set(controller_labels): _fail('job_template_labels_invalid')
    template_metadata['labels'] = expected_labels
    if set(template_metadata) != {'labels'}: _fail('job_template_metadata_invalid')
    pod_spec = template.get('spec', {})
    for field, value in {'dnsPolicy': 'ClusterFirst', 'schedulerName': 'default-scheduler',
                         'terminationGracePeriodSeconds': 30}.items():
        if field in pod_spec:
            if pod_spec[field] != value: _fail('pod_default_changed')
            del pod_spec[field]
    containers = pod_spec.get('containers')
    if not isinstance(containers, list) or len(containers) != 1:
        _fail('job_container_shape_invalid')
    container_defaults = {'terminationMessagePath': '/dev/termination-log', 'terminationMessagePolicy': 'File'}
    for field, value in container_defaults.items():
        if field in containers[0]:
            if containers[0][field] != value: _fail('container_default_changed')
            del containers[0][field]
    if set(spec) != set(wanted['spec']) or spec != wanted['spec']:
        _fail('job_template_changed')
    return actual


def _verify_pod(pod, wanted, job_uid):
    metadata = pod.get('metadata', {})
    expected_name = wanted['metadata']['name']
    if (pod.get('apiVersion') != 'v1' or pod.get('kind') != 'Pod'
        or metadata.get('namespace') != NAMESPACE
        or not isinstance(metadata.get('name'), str) or not re.fullmatch(re.escape(expected_name) + r'-[a-z0-9]{5}', metadata['name'])):
        _fail('pod_identity_invalid')
    if set(metadata) - {'name', 'namespace', 'labels', 'uid', 'resourceVersion', 'creationTimestamp',
                        'managedFields', 'ownerReferences'}:
        _fail('pod_metadata_shape_invalid')
    owners = metadata.get('ownerReferences', [])
    if owners != [{'apiVersion': 'batch/v1', 'kind': 'Job', 'name': expected_name,
                   'uid': job_uid, 'controller': True, 'blockOwnerDeletion': True}]:
        _fail('pod_owner_invalid')
    expected_labels = copy.deepcopy(wanted['spec']['template']['metadata']['labels'])
    controller_labels = {'batch.kubernetes.io/controller-uid': job_uid,
        'batch.kubernetes.io/job-name': expected_name, 'controller-uid': job_uid, 'job-name': expected_name}
    labels = metadata.get('labels', {})
    if any(labels.get(key) != value for key, value in controller_labels.items()) or set(labels) != set(expected_labels) | set(controller_labels):
        _fail('pod_labels_invalid')
    pod_spec = copy.deepcopy(pod.get('spec', {}))
    for field, value in {'dnsPolicy': 'ClusterFirst', 'schedulerName': 'default-scheduler',
                         'terminationGracePeriodSeconds': 30}.items():
        if field in pod_spec:
            if pod_spec[field] != value: _fail('pod_default_changed')
            del pod_spec[field]
    containers = pod_spec.get('containers')
    if not isinstance(containers, list) or len(containers) != 1:
        _fail('pod_container_shape_invalid')
    for field, value in {'terminationMessagePath': '/dev/termination-log', 'terminationMessagePolicy': 'File'}.items():
        if field in containers[0]:
            if containers[0][field] != value: _fail('container_default_changed')
            del containers[0][field]
    if pod_spec != wanted['spec']['template']['spec']:
        _fail('pod_spec_changed')
    if pod.get('status', {}).get('phase') != 'Succeeded':
        _fail('pod_not_succeeded')


def _receipt(logs, request):
    def pairs(items):
        row = {}
        for key, value in items:
            if key in row: _fail('result_duplicate_field')
            row[key] = value
        return row
    try:
        row = json.loads(logs, object_pairs_hook=pairs)
    except (ValueError, TypeError):
        _fail('result_invalid')
    if (not isinstance(row, dict) or set(row) != {'schema', 'status', 'operation', 'nonce', 'phase', 'result_sha256'}
        or row.get('schema') != RECEIPT_SCHEMA or row.get('status') != 'COMPLETE'
        or row.get('operation') != request['operation'] or row.get('nonce') != request['nonce']
        or row.get('phase') != request['phase'] or not isinstance(row.get('result_sha256'), str)
        or not re.fullmatch('[a-f0-9]{64}', row['result_sha256'])):
        _fail('result_identity_invalid')
    return row


def _verify_storage(api, binding: dict) -> None:
    claim = api.get_pvc(NAMESPACE, binding['claim_name'])
    pv = api.get_pv(binding['pv_name'])
    node = api.get_node(binding['node_name'])
    claim_meta = claim.get('metadata', {})
    pv_meta = pv.get('metadata', {})
    claim_spec = claim.get('spec', {})
    ref = pv.get('spec', {}).get('claimRef', {})
    conditions = [row for row in node.get('status', {}).get('conditions', []) if row.get('type') == 'Ready']
    if (claim_meta.get('name') != binding['claim_name'] or claim_meta.get('namespace') != NAMESPACE
        or claim_meta.get('uid') != binding['claim_uid'] or claim.get('status', {}).get('phase') != 'Bound'
        or claim_spec.get('volumeName') != binding['pv_name']
        or pv_meta.get('name') != binding['pv_name'] or pv_meta.get('uid') != binding['pv_uid']
        or ref.get('namespace') != NAMESPACE or ref.get('name') != binding['claim_name']
        or ref.get('uid') != binding['claim_uid'] or node.get('metadata', {}).get('name') != binding['node_name']
        or len(conditions) != 1 or conditions[0].get('status') != 'True'):
        _fail('storage_binding_changed')


def run(api, request: dict, binding: dict, runner_image: str, *, persist_intent, persist_result) -> dict:
    """Create one operation-owned Job, read its exact identity/result, then clean it up.

    `api` exposes storage reads, create_job, wait_job, get_job_pod, pod_logs,
    delete_job (which must confirm UID-bound absence).
    The durable callbacks must persist before create and after verified readback.
    """
    row = validate_request(request)
    storage = validate_binding(binding)
    image = validate_runner_image(runner_image)
    _verify_storage(api, storage)
    wanted = document(row, storage, image)
    name = wanted['metadata']['name']
    persist_intent({'phase': row['phase'], 'nonce': row['nonce'], 'job_name': name,
                    'template_sha256': hashlib.sha256(json.dumps(wanted['spec'], sort_keys=True, separators=(',', ':')).encode()).hexdigest(),
                    'state': 'JOB_CREATE_INTENT'})
    created = api.create_job(NAMESPACE, copy.deepcopy(wanted))
    created = verify_job(created, wanted)
    uid = created['metadata']['uid']
    observed = api.wait_job(NAMESPACE, name, uid, timeout=120)
    verify_job(observed, wanted, created_uid=uid)
    if observed.get('status', {}).get('succeeded') != 1 or observed.get('status', {}).get('failed', 0) != 0:
        _fail('job_not_successful')
    pod = api.get_job_pod(NAMESPACE, name)
    _verify_pod(pod, wanted, uid)
    pod_meta = pod['metadata']
    logs = api.pod_logs(NAMESPACE, pod_meta['name'])
    if not isinstance(logs, str) or not 1 <= len(logs) <= 65536:
        _fail('result_invalid')
    receipt = _receipt(logs, row)
    result = {'job_name': name, 'job_uid': uid, 'pod_name': pod_meta['name'],
        'phase': row['phase'], 'result_sha256': receipt['result_sha256']}
    persist_result({'phase': row['phase'], 'nonce': row['nonce'], 'job_name': name, 'job_uid': uid,
                    'pod_name': pod_meta['name'], 'state': 'JOB_RESULT_READBACK',
                    'result_sha256': receipt['result_sha256']})
    if api.delete_job(NAMESPACE, name, uid) is not True:
        _fail('job_cleanup_unconfirmed')
    return result
