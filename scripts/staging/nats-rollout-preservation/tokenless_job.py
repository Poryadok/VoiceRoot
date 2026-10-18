"""Fixed tokenless PVC Job protocol for the trusted STAGING runner.

The submitter selects only a phase from PHASES. Every container image, command,
volume, security setting, label, timeout, and cleanup decision is runner-owned.
"""
from __future__ import annotations

import copy
import hashlib
import json
import os
import re
import stat
import time
from pathlib import Path


NAMESPACE = 'voice-staging'
LABEL_OPERATION = 'voice.io/nats-operation'
LABEL_NONCE = 'voice.io/nats-job-nonce'
LABEL_PHASE = 'voice.io/nats-job-phase'
PHASES = frozenset(('seed', 'verify-closed', 'drain', 'verify-drained', 'census', 'archive'))
RECEIPT_SCHEMA = 'voice-nats-tokenless-phase-receipt-v1'
FIELDS = frozenset(('operation', 'nonce', 'phase'))
BINDING_FIELDS = frozenset(('claim_name', 'claim_uid', 'pv_name', 'pv_uid', 'node_name', 'node_hostname'))


class JobError(ValueError):
    pass


def _fail(code: str) -> None:
    raise JobError('nats_job_' + code)


def _quantity_bytes(value):
    match = re.fullmatch(r'([1-9][0-9]*)(Ki|Mi|Gi|Ti)?', value) if isinstance(value, str) else None
    if not match:
        return None
    return int(match[1]) * {'Ki':1<<10,'Mi':1<<20,'Gi':1<<30,'Ti':1<<40,None:1}[match[2]]


def _quantity_at_least(value, required):
    actual = _quantity_bytes(value)
    return actual is not None and actual >= required


def _pv_hostname(spec):
    terms = spec.get('nodeAffinity', {}).get('required', {}).get('nodeSelectorTerms')
    if (not isinstance(terms, list) or len(terms) != 1
        or set(terms[0]) != {'matchExpressions'} or len(terms[0]['matchExpressions']) != 1):
        _fail('verification_node_affinity_invalid')
    expression = terms[0]['matchExpressions'][0]
    values = expression.get('values')
    if (set(expression) != {'key','operator','values'}
        or expression.get('key') != 'kubernetes.io/hostname'
        or expression.get('operator') != 'In' or not isinstance(values,list) or len(values)!=1
        or not isinstance(values[0],str)
        or not re.fullmatch(r'[a-z0-9](?:[a-z0-9.-]{0,61}[a-z0-9])?',values[0])):
        _fail('verification_node_affinity_invalid')
    return values[0]


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
    if not isinstance(value['node_hostname'], str) or not re.fullmatch(
        r'[a-z0-9](?:[a-z0-9.-]{0,61}[a-z0-9])?', value['node_hostname']):
        _fail('storage_hostname_invalid')
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
    archive = row['phase'] == 'archive'
    mounts = [{'name': 'data', 'mountPath': '/data', 'readOnly': archive}]
    volumes = [{'name': 'data', 'persistentVolumeClaim': {'claimName': storage['claim_name'], 'readOnly': archive}}]
    if archive:
        # Only the helper's transient work product is writable. The selected
        # source claim is mounted read-only; the trusted runner receives the
        # verified result over the fixed pods/exec binary stream protocol.
        mounts.append({'name': 'scratch', 'mountPath': '/out'})
        volumes.append({'name': 'scratch', 'emptyDir': {'sizeLimit': '49Gi'}})
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
            'activeDeadlineSeconds': 43200 if archive else 120,
            **({'ttlSecondsAfterFinished': 600} if not archive else {}),
            'template': {
                'metadata': {'labels': {LABEL_OPERATION: row['operation'], LABEL_NONCE: row['nonce'], LABEL_PHASE: row['phase']}},
                'spec': {
                    'automountServiceAccountToken': False,
                    'nodeName': storage['node_name'],
                    'serviceAccountName': 'default',
                    'restartPolicy': 'Never',
                    # Match the existing STAGING broker's supported local-PV
                    # ownership contract. Kubelet applies this group on mount;
                    # the runner never chowns a host/PV path.
                    'securityContext': {'runAsNonRoot': True, 'runAsUser': 65532, 'runAsGroup': 65532,
                        'fsGroup': 10000, 'fsGroupChangePolicy': 'OnRootMismatch',
                        'seccompProfile': {'type': 'RuntimeDefault'}},
                    'containers': [{
                        'name': 'nats-phase',
                        'image': image,
                        'imagePullPolicy': 'IfNotPresent',
                        'command': ['/voice-nats-runner/phase'],
                        'args': (['--serve', '--operation', row['operation'], '--nonce', row['nonce']]
                            if archive else ['--phase', row['phase'], '--nonce', row['nonce']]),
                        **({'env': [{'name': 'POD_UID', 'valueFrom': {'fieldRef': {'fieldPath': 'metadata.uid'}}}]}
                            if archive else {}),
                        'securityContext': {'allowPrivilegeEscalation': False, 'readOnlyRootFilesystem': True, 'capabilities': {'drop': ['ALL']}},
                        'resources': ({'requests': {'cpu': '500m', 'memory': '512Mi', 'ephemeral-storage': '49Gi'},
                            'limits': {'cpu': '2', 'memory': '2Gi', 'ephemeral-storage': '49Gi'}} if archive else
                            {'requests': {'cpu': '10m', 'memory': '16Mi'},
                             'limits': {'cpu': '1', 'memory': '512Mi'}}),
                        'volumeMounts': mounts,
                    }],
                    'volumes': volumes,
                },
            },
        },
    }


def verify_job(actual: object, wanted: dict, *, created_uid: str | None = None) -> dict:
    if not isinstance(actual, dict):
        _fail('readback_invalid')
    expected = wanted['metadata']
    metadata = actual.get('metadata', {})
    if actual.get('apiVersion') != 'batch/v1' or actual.get('kind') != 'Job':
        _fail('job_readback_kind_invalid')
    if metadata.get('name') != expected['name']:
        _fail('job_name_changed')
    if metadata.get('namespace') != NAMESPACE:
        _fail('job_namespace_changed')
    if metadata.get('labels') != expected['labels']:
        _fail('job_labels_changed')
    if not isinstance(metadata.get('uid'), str) or not re.fullmatch('[a-f0-9-]{36}', metadata['uid']):
        _fail('job_uid_invalid')
    if created_uid is not None and metadata['uid'] != created_uid:
        _fail('job_uid_changed')
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


def _verify_pod(pod, wanted, job_uid, *, phase='Succeeded'):
    metadata = pod.get('metadata', {})
    expected_name = wanted['metadata']['name']
    if (pod.get('apiVersion') != 'v1' or pod.get('kind') != 'Pod'
        or metadata.get('namespace') != NAMESPACE
        or not isinstance(metadata.get('name'), str) or not re.fullmatch(re.escape(expected_name) + r'-[a-z0-9]{5}', metadata['name'])
        or not isinstance(metadata.get('uid'), str) or not re.fullmatch('[a-f0-9-]{36}', metadata['uid'])):
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
    if pod.get('status', {}).get('phase') != phase:
        _fail('pod_not_' + phase.lower())
    if phase == 'Running':
        statuses = pod.get('status', {}).get('containerStatuses', [])
        if (len(statuses) != 1 or statuses[0].get('name') != 'nats-phase'
            or not isinstance(statuses[0].get('containerID'), str)
            or not re.fullmatch('containerd://[a-f0-9]{64}', statuses[0]['containerID'])
            or statuses[0].get('ready') is not True or statuses[0].get('restartCount') != 0):
            _fail('pod_container_identity_invalid')


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
    node_affinity = pv.get('spec', {}).get('nodeAffinity', {}).get('required', {}).get('nodeSelectorTerms')
    if (not isinstance(node_affinity, list) or len(node_affinity) != 1
        or set(node_affinity[0]) != {'matchExpressions'} or len(node_affinity[0]['matchExpressions']) != 1):
        _fail('storage_node_affinity_changed')
    expression = node_affinity[0]['matchExpressions'][0]
    conditions = [row for row in node.get('status', {}).get('conditions', []) if row.get('type') == 'Ready']
    if (claim_meta.get('name') != binding['claim_name'] or claim_meta.get('namespace') != NAMESPACE
        or claim_meta.get('uid') != binding['claim_uid'] or claim.get('status', {}).get('phase') != 'Bound'
        or claim_spec.get('volumeName') != binding['pv_name']
        or pv_meta.get('name') != binding['pv_name'] or pv_meta.get('uid') != binding['pv_uid']
        or ref.get('namespace') != NAMESPACE or ref.get('name') != binding['claim_name']
        or ref.get('uid') != binding['claim_uid'] or node.get('metadata', {}).get('name') != binding['node_name']
        or node.get('metadata', {}).get('labels', {}).get('kubernetes.io/hostname') != binding['node_hostname']
        or set(expression) != {'key', 'operator', 'values'}
        or expression.get('key') != 'kubernetes.io/hostname' or expression.get('operator') != 'In'
        or expression.get('values') != [binding['node_hostname']]
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


def run_archive(api, request: dict, binding: dict, runner_image: str, artifact, *,
                persist_intent, persist_result, verify_archive):
    """Create the single fixed read-only-PVC archive Job and stream its result.

    If anything fails after Job creation, the Job is retained. Recovery must
    call ``reconcile_archive`` with the journaled Job and Pod UIDs; this entry
    point never blindly creates the same archive operation again.
    """
    import archive_stream
    import verification_job
    row = verification_job.validate_request(request)
    if row['phase'] != 'archive': _fail('archive_phase_required')
    storage = validate_binding(binding)
    producer_binding = {'source_claim': storage['claim_name'], 'source_uid': storage['claim_uid'],
        'source_pv_name': storage['pv_name'], 'source_pv_uid': storage['pv_uid'],
        'source_node_name': storage['node_name'], 'source_hostname': storage['node_hostname']}
    image = verification_job.validate_image(runner_image)
    _verify_storage(api, storage)
    wanted = verification_job.document(row, producer_binding, image)
    name = wanted['metadata']['name']
    template_sha = hashlib.sha256(json.dumps(wanted['spec'], sort_keys=True,
        separators=(',', ':')).encode()).hexdigest()
    persist_intent({'phase': 'archive', 'nonce': row['nonce'], 'job_name': name,
        'template_sha256': template_sha, 'state': 'JOB_CREATE_INTENT'})
    created = verification_job.verify_job(api.create_job(NAMESPACE, copy.deepcopy(wanted)), wanted)
    uid = created['metadata']['uid']
    persist_intent({'phase': 'archive', 'nonce': row['nonce'], 'job_name': name,
        'job_uid': uid, 'template_sha256': template_sha, 'state': 'JOB_CREATED'})
    pod = api.wait_job_pod(NAMESPACE, name, uid, timeout=120)
    verification_job.verify_pod(pod, wanted['spec']['template'], uid, 'archive',
        expected_node_name=storage['node_name'])
    pod_meta = pod['metadata']
    container_id = pod['status']['containerStatuses'][0]['containerID']
    persist_intent({'phase': 'archive', 'nonce': row['nonce'], 'job_name': name,
        'job_uid': uid, 'pod_name': pod_meta['name'], 'pod_uid': pod_meta['uid'],
        'container_id': container_id, 'template_sha256': template_sha, 'state': 'ARCHIVE_STREAM_UNKNOWN'})
    result = _consume_archive(api, row, storage, wanted, uid, pod, artifact,
                              persist_intent, persist_result, verify_archive)
    if api.delete_job(NAMESPACE, name, uid) is not True:
        _fail('archive_job_cleanup_unconfirmed')
    return result


def reconcile_archive(api, request: dict, binding: dict, runner_image: str,
                      journal: dict, artifact, *, persist_intent, persist_result,
                      verify_archive):
    """Read back and re-stream only the journaled completed Job/Pod identity."""
    import verification_job
    row = verification_job.validate_request(request)
    if row['phase'] != 'archive' or not isinstance(journal, dict) or journal.get('phase') != 'archive':
        _fail('archive_recovery_binding_invalid')
    storage = validate_binding(binding)
    producer_binding = {'source_claim': storage['claim_name'], 'source_uid': storage['claim_uid'],
        'source_pv_name': storage['pv_name'], 'source_pv_uid': storage['pv_uid'],
        'source_node_name': storage['node_name'], 'source_hostname': storage['node_hostname']}
    wanted = verification_job.document(row, producer_binding, verification_job.validate_image(runner_image))
    name, uid = wanted['metadata']['name'], journal.get('job_uid')
    pod_name, pod_uid = journal.get('pod_name'), journal.get('pod_uid')
    if (journal.get('job_name') != name or journal.get('template_sha256') != hashlib.sha256(
        json.dumps(wanted['spec'], sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        or journal.get('state') not in ('ARCHIVE_STREAM_UNKNOWN', 'ARCHIVE_STREAM_INTENT')
        or not isinstance(uid, str) or not re.fullmatch('[a-f0-9-]{36}', uid)
        or not isinstance(pod_name, str) or not isinstance(pod_uid, str)
        or not re.fullmatch('[a-f0-9-]{36}', pod_uid)
        or not isinstance(journal.get('container_id'), str)
        or not re.fullmatch('containerd://[a-f0-9]{64}', journal['container_id'])):
        _fail('archive_recovery_binding_invalid')
    verification_job.verify_job(api.get_job(NAMESPACE, name), wanted, expected_uid=uid)
    pod = api.get_job_pod(NAMESPACE, name)
    verification_job.verify_pod(pod, wanted['spec']['template'], uid, 'archive',
        expected_node_name=storage['node_name'], expected_uid=pod_uid)
    if pod.get('metadata', {}).get('name') != pod_name or pod.get('metadata', {}).get('uid') != pod_uid:
        _fail('archive_pod_changed')
    if pod['status']['containerStatuses'][0]['containerID'] != journal['container_id']:
        _fail('archive_container_changed')
    result = _consume_archive(api, row, storage, wanted, uid, pod, artifact,
                              persist_intent, persist_result, verify_archive, existing=True)
    if api.delete_job(NAMESPACE, name, uid) is not True:
        _fail('archive_job_cleanup_unconfirmed')
    return result


def _consume_archive(api, row, storage, wanted, job_uid, pod, artifact,
                     persist_intent, persist_result, verify_archive, *, existing=False):
    import archive_stream
    import verification_job
    pod_meta = pod['metadata']
    _verify_storage(api, storage)
    producer_binding = {
        'source_claim': storage['claim_name'], 'source_uid': storage['claim_uid'],
        'source_pv_name': storage['pv_name'], 'source_pv_uid': storage['pv_uid'],
        'source_node_name': storage['node_name'], 'source_hostname': storage['node_hostname'],
    }
    action = 'archive_same_pod_readback' if existing else 'archive_create_and_stream'
    result = archive_stream.consume(api.exec_pod_verification(NAMESPACE, pod_meta['name'], pod_meta['uid'],
        row, producer_binding, action),
        artifact, operation=row['operation'], nonce=row['nonce'], pod_uid=pod_meta['uid'],
        source_pvc_uid=storage['claim_uid'], source_pv_uid=storage['pv_uid'],
        allow_existing=existing,
        persist_intent=lambda value: persist_intent(value | {'phase':'archive','job_name': wanted['metadata']['name'],
            'job_uid': job_uid, 'pod_name': pod_meta['name'], 'pod_uid': pod_meta['uid'],
            'container_id': pod['status']['containerStatuses'][0]['containerID'],
            'template_sha256': hashlib.sha256(json.dumps(wanted['spec'], sort_keys=True,
                separators=(',', ':')).encode()).hexdigest()}),
        persist_result=persist_result, verify_archive=verify_archive)
    current = api.get_job_pod(NAMESPACE, wanted['metadata']['name'])
    verification_job.verify_pod(current, wanted['spec']['template'], job_uid, 'archive',
        expected_node_name=storage['node_name'], expected_uid=pod_meta['uid'])
    if (current.get('metadata', {}).get('uid') != pod_meta.get('uid')
        or current['status']['containerStatuses'][0]['containerID'] != pod['status']['containerStatuses'][0]['containerID']):
        _fail('archive_container_changed')
    return result


class PvcArchiveRuntime:
    """Internal ``capture_cut`` adapter for a single trusted archive Job.

    It intentionally exposes no command, path, image or manifest selector.
    The surrounding transaction binds operation and storage identities before
    constructing this object. Non-archive DockerRuntime operations are not
    implemented by this adapter and must not fall back to the host runtime.
    """
    def __init__(self, api, base, binding, runner_image, operation, request_nonce,
                 *, persist_intent, persist_result, account_id=None,
                 verification_pvc_size_bytes=None, verification_storage_class=None):
        self.api = api
        self.base = Path(base)
        self.binding = validate_binding(binding)
        self.runner_image = validate_runner_image(runner_image)
        if account_id is not None and (not isinstance(account_id,str)
            or not re.fullmatch(r'A[A-Z2-7]{55}',account_id)):
            _fail('archive_runtime_account_invalid')
        self._account_id=account_id
        if (not isinstance(operation, str) or not re.fullmatch('[a-f0-9]{12}', operation)
            or not isinstance(request_nonce, str) or not re.fullmatch('[a-f0-9]{64}', request_nonce)):
            _fail('archive_runtime_identity_invalid')
        directory = self.base.lstat()
        if (not stat.S_ISDIR(directory.st_mode) or stat.S_ISLNK(directory.st_mode)
            or directory.st_uid != os.geteuid() or stat.S_IMODE(directory.st_mode) != 0o700):
            _fail('archive_runtime_directory_invalid')
        self.operation = operation
        self.rootless = True
        self.request = {'operation': operation,
            'nonce': hashlib.sha256((request_nonce + ':cold-cut-archive').encode()).hexdigest()[:32],
            'phase': 'archive'}
        self.persist_intent = persist_intent
        self.persist_result = persist_result
        if verification_pvc_size_bytes is not None and (
            type(verification_pvc_size_bytes) is not int or verification_pvc_size_bytes <= 0
            or verification_pvc_size_bytes > (1 << 63) - 1):
            _fail('verification_storage_config_invalid')
        if verification_storage_class is not None and (
            not isinstance(verification_storage_class, str) or not verification_storage_class):
            _fail('verification_storage_config_invalid')
        self.verification_pvc_size_bytes = verification_pvc_size_bytes
        self.verification_storage_class = verification_storage_class

    def account_id(self):
        if self._account_id is None:
            _fail('archive_runtime_account_unbound')
        return self._account_id

    def no_operation_containers(self, *, running_only=False):
        rows = self.api.list_operation_jobs(NAMESPACE, self.operation)
        if rows:
            _fail('operation_job_writer_present')

    def archive_selected_store(self, artifact):
        import native_store
        target = Path(artifact)
        if target.parent != self.base or target.name not in ('rollout-before.tar','rollout-after.tar') \
            or target.exists() or target.is_symlink():
            _fail('archive_target_invalid')
        archive_request = dict(self.request)
        if target.name != 'rollout-before.tar':
            archive_request['nonce'] = hashlib.sha256((self.request['nonce'] + ':' + target.name).encode()).hexdigest()[:32]
        result = run_archive(self.api, archive_request, self.binding, self.runner_image, target,
            persist_intent=self.persist_intent, persist_result=self.persist_result,
            verify_archive=native_store.verify_archive)
        self._archive_result = result
        capacity=self.api.get_pvc(NAMESPACE,self.binding['claim_name']).get('status',{}).get('capacity',{}).get('storage')
        match=re.fullmatch(r'([1-9][0-9]*)(Ki|Mi|Gi|Ti)?',capacity) if isinstance(capacity,str) else None
        if not match:
            _fail('source_capacity_readback_invalid')
        capacity_bytes=int(match[1])*{'Ki':1<<10,'Mi':1<<20,'Gi':1<<30,'Ti':1<<40,None:1}[match[2]]
        if type(result.get('bytes')) is not int or result['bytes']>capacity_bytes:
            _fail('archive_exceeds_reported_source_capacity')
        return result['manifest']

    def capture_census(self, archive, manifest, label, *, validate=None):
        """Create a disposable verified scratch claim and run the fixed census ABI."""
        import base64
        import census_stream
        import kube_json_api
        import verification_job
        import native_store
        if label != 'rollout-before' or not isinstance(manifest, dict):
            _fail('census_request_invalid')
        # Revalidate the retained source artifact and canonical manifest before
        # allocating scratch; no archive bytes are read into memory.
        verified = native_store.verify_archive(archive, manifest)
        if verified is not True:
            _fail('census_archive_invalid')
        archive_path = Path(archive)
        archive_stat = archive_path.lstat()
        if (not stat.S_ISREG(archive_stat.st_mode) or stat.S_ISLNK(archive_stat.st_mode)
            or archive_stat.st_uid != os.geteuid() or stat.S_IMODE(archive_stat.st_mode) != 0o600
            or archive_stat.st_nlink != 1 or archive_stat.st_size <= 0):
            _fail('census_archive_custody_invalid')
        archive_bytes = archive_stat.st_size
        archive_sha = getattr(self, '_archive_result', {}).get('archive_sha256')
        if not isinstance(archive_sha, str) or not re.fullmatch('[a-f0-9]{64}', archive_sha):
            _fail('census_archive_digest_invalid')
        # Freshly resolve the selected source for every mutating phase. The
        # journaled source is an expectation, never an authority.
        self._fresh_source()
        attempt = hashlib.sha256((self.operation + ':' + self.request['nonce'] + ':census').encode()).hexdigest()[:8]
        census_nonce = hashlib.sha256((self.request['nonce'] + ':prepare-census').encode()).hexdigest()[:32]
        request = {'operation': self.operation, 'nonce': census_nonce, 'phase': 'census'}
        name = 'voice-nats-verify-' + self.operation
        pvc, pvc_uid, pv, pv_uid, node_name, hostname, sc_identity = self._ensure_verification_claim(name)
        binding = {
            'source_claim': self.binding['claim_name'], 'source_uid': self.binding['claim_uid'],
            'source_pv_name': self.binding['pv_name'], 'source_pv_uid': self.binding['pv_uid'],
            'source_node_name': self.binding['node_name'],
            'source_hostname': self.binding['node_hostname'],
            'verification_claim': name, 'verification_uid': pvc_uid,
            'verification_pv_name': pv['metadata']['name'], 'verification_pv_uid': pv_uid,
            'verification_node_name': node_name, 'verification_hostname': hostname,
        }
        binding = verification_job.validate_binding(binding)
        self.persist_intent({'phase':'census','state':'PVC_BOUND','operation':self.operation,
            'nonce':request['nonce'],'session_attempt':attempt,'verification_claim':name,
            'verification_uid':pvc_uid,'verification_pv_name':pv['metadata']['name'],
            'verification_pv_uid':pv_uid,'verification_node_name':node_name,
            'verification_hostname':hostname,'storage_class':sc_identity})
        if validate is not None: validate()
        # A single fixed census Pod performs the two-file loader, same-Pod
        # consumption proof, archive restore, census, and durable readback.
        wanted = verification_job.document(request, binding, self.runner_image)
        job_name = wanted['metadata']['name']
        template_sha = hashlib.sha256(json.dumps(wanted['spec'], sort_keys=True,
            separators=(',', ':')).encode()).hexdigest()
        self._fresh_source()
        self._fresh_verification_storage(name, pvc_uid, pv_uid, sc_identity, node_name, hostname)
        try:
            self.api.get_job(kube_json_api.NAMESPACE, job_name)
        except kube_json_api.KubeNotFound:
            pass
        else:
            _fail('census_job_already_exists')
        self.persist_intent({'phase':'census','state':'JOB_CREATE_INTENT','operation':self.operation,
            'nonce':request['nonce'],'job_name':job_name,'template_sha256':template_sha,
            'verification_uid':pvc_uid,'session_attempt':attempt})
        try:
            created = verification_job.verify_job(
                self.api.create_job(kube_json_api.NAMESPACE, copy.deepcopy(wanted)), wanted)
        except Exception:
            # An API error after CREATE is an unknown outcome. Reconcile only
            # the deterministic name and exact operation template; never POST again.
            try: observed = self.api.get_job(kube_json_api.NAMESPACE, job_name)
            except kube_json_api.KubeNotFound: _fail('census_job_create_unknown')
            created = verification_job.verify_job(observed, wanted)
        job_uid = created['metadata']['uid']
        self.persist_intent({'phase':'census','state':'JOB_CREATED','operation':self.operation,
            'nonce':request['nonce'],'job_name':job_name,'job_uid':job_uid,
            'template_sha256':template_sha,'verification_uid':pvc_uid,'session_attempt':attempt})
        pod = self.api.wait_job_pod(kube_json_api.NAMESPACE, job_name, job_uid, timeout=120)
        verification_job.verify_pod(pod, wanted['spec']['template'], job_uid, 'census',
            expected_node_name=node_name)
        pod_name, pod_uid = pod['metadata']['name'], pod['metadata']['uid']
        container_id = pod['status']['containerStatuses'][0]['containerID']
        archive_binding = {'sha256':archive_sha,'bytes':archive_bytes}
        try:
            bootstrap, server_conf = self._read_census_inputs()
            bundle = {'schema':'voice-nats-census-inputs-v1','operation':self.operation,
                'nonce':request['nonce'],'pod_uid':pod_uid,
                'files':{'bootstrap.creds':base64.b64encode(bootstrap).decode('ascii'),
                    'server.conf':base64.b64encode(server_conf).decode('ascii')}}
            bundle_bytes = json.dumps(bundle,sort_keys=True,separators=(',',':'),
                ensure_ascii=False,allow_nan=False).encode('utf-8')
            bundle_info = {'attempt':attempt,'sha256':hashlib.sha256(bundle_bytes).hexdigest(),
                'bytes':len(bundle_bytes)}
            self._assert_census_pod(request,binding,wanted,job_uid,pod_name,pod_uid,container_id,
                sc_identity,validate)
            self.persist_intent({'phase':'census','state':'INPUT_LOAD_INTENT','job_name':job_name,
                'job_uid':job_uid,'pod_name':pod_name,'pod_uid':pod_uid,'container_id':container_id,
                'session_attempt':attempt,'input_bundle_sha256':bundle_info['sha256'],
                'input_bundle_bytes':bundle_info['bytes'],'archive_sha256':archive_sha,
                'archive_bytes':archive_bytes,'template_sha256':template_sha})
            loaded = self.api.exec_pod_verification_input(kube_json_api.NAMESPACE,pod_name,pod_uid,
                request,binding,archive_binding,bundle_bytes,bundle_info)
            load_receipt = self.api.read_census_input_receipt(kube_json_api.NAMESPACE,pod_name,pod_uid,
                request,binding,archive_binding,bundle_info,input_pod_uid=pod_uid)
            if loaded != (json.dumps(load_receipt,sort_keys=True,separators=(',',':'),
                    ensure_ascii=False,allow_nan=False).encode('utf-8')+b'\n'):
                _fail('census_input_receipt_mismatch')
            self.persist_result({'phase':'census','state':'INPUTS_COMPLETE','pod_uid':pod_uid,
                'job_uid':job_uid,'session_attempt':attempt,'input_bundle_sha256':bundle_info['sha256'],
                'input_bundle_bytes':bundle_info['bytes'],'kernel_bundle_sha256':load_receipt['kernel_bundle_sha256'],
                'server_config_sha256':load_receipt['server_config_sha256']})
            self._assert_census_pod(request,binding,wanted,job_uid,pod_name,pod_uid,container_id,
                sc_identity,validate)
            self.persist_intent({'phase':'census','state':'INPUT_CONSUME_INTENT','pod_uid':pod_uid,
                'job_uid':job_uid,'session_attempt':attempt})
            consumption = self.api.execute_census_input_consumption(kube_json_api.NAMESPACE,
                pod_name,pod_uid,request,binding,archive_binding,bundle_info,input_pod_uid=pod_uid)
            self.persist_result({'phase':'census','state':'INPUTS_CONSUMED','pod_uid':pod_uid,
                'job_uid':job_uid,'session_attempt':attempt,'receipt':consumption})
            self._assert_census_pod(request,binding,wanted,job_uid,pod_name,pod_uid,container_id,
                sc_identity,validate)
            self.persist_intent({'phase':'census','state':'ARCHIVE_INGEST_INTENT','pod_uid':pod_uid,
                'job_uid':job_uid,'session_attempt':attempt,'archive_sha256':archive_sha,
                'archive_bytes':archive_bytes})
            self.api.exec_pod_census_input(kube_json_api.NAMESPACE,pod_name,pod_uid,request,binding,
                archive,bytes(json.dumps(manifest,sort_keys=True,separators=(',',':'),
                    ensure_ascii=False,allow_nan=False).encode('utf-8')),archive_sha,archive_bytes,bundle_info)
            self.persist_result({'phase':'census','state':'CENSUS_EXECUTION_COMPLETE','pod_uid':pod_uid,
                'job_uid':job_uid,'session_attempt':attempt,'archive_sha256':archive_sha,
                'archive_bytes':archive_bytes})
            self._assert_census_pod(request,binding,wanted,job_uid,pod_name,pod_uid,container_id,
                sc_identity,validate)
            result = self.api.read_verification_census(kube_json_api.NAMESPACE,pod_name,pod_uid,
                request,binding,archive_binding,bundle_info)
            census = result.get('census') if isinstance(result,dict) else None
            if not isinstance(census,dict) or census.get('account') != self.account_id():
                _fail('census_account_mismatch')
            self.persist_result({'phase':'census','state':'CENSUS_RECEIPT_VALIDATED','pod_uid':pod_uid,
                'job_uid':job_uid,'session_attempt':attempt,
                'census_sha256':result.get('receipt',{}).get('census_sha256')})
            self._assert_census_pod(request,binding,wanted,job_uid,pod_name,pod_uid,container_id,
                sc_identity,validate)
            if self.api.delete_job(kube_json_api.NAMESPACE,job_name,job_uid) is not True:
                _fail('census_job_cleanup_unconfirmed')
            self.persist_result({'phase':'census','state':'JOB_CLEANED','job_name':job_name,
                'job_uid':job_uid,'pod_uid':pod_uid,'verification_uid':pvc_uid})
            return census
        except Exception:
            # Keep the Job, Pod and PVC for immutable same-Pod read-only
            # reconciliation. No failure path cleans or recreates a writer.
            raise

    def _fresh_source(self):
        import kube_json_api
        expected = {'binding':{'source_claim':self.binding['claim_name'],
            'source_uid':self.binding['claim_uid'],'source_pv_name':self.binding['pv_name'],
            'source_pv_uid':self.binding['pv_uid'],'source_node_name':self.binding['node_name']},
            'proof':None}
        # The transaction callback performs the stronger full captured identity
        # comparison and final storage fence before each phase/action.
        self.persist_intent({'phase':'census','state':'SOURCE_READBACK_REQUIRED'})

    def _read_census_inputs(self):
        import base64
        files = {}
        for name, limit, path in (('bootstrap.creds',256 << 10,self.base/'inputs'/'bootstrap.creds'),
                                  ('server.conf',256 << 10,self.base/'server.conf')):
            try: fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
            except OSError: _fail('census_input_unavailable')
            try:
                before=os.fstat(fd)
                if (not stat.S_ISREG(before.st_mode) or before.st_uid!=os.geteuid()
                    or before.st_gid!=os.getegid() or stat.S_IMODE(before.st_mode)!=0o440
                    or before.st_nlink!=1 or not 0<before.st_size<=limit):
                    _fail('census_input_custody_invalid')
                raw=os.read(fd,limit+1); after=os.fstat(fd)
                if len(raw)!=before.st_size or (before.st_dev,before.st_ino,before.st_size,
                    before.st_mtime_ns,before.st_ctime_ns)!=(after.st_dev,after.st_ino,after.st_size,
                    after.st_mtime_ns,after.st_ctime_ns): _fail('census_input_changed')
                files[name]=raw
            finally: os.close(fd)
        return files['bootstrap.creds'],files['server.conf']

    def _ensure_verification_claim(self,name):
        import kube_json_api
        if self.verification_pvc_size_bytes is None or self.verification_storage_class is None:
            _fail('verification_storage_config_missing')
        # StorageClass identity is pinned before any CREATE and rechecked before
        # binding adoption; no default class or implicit binding mode is used.
        sc=self.api.get_storage_class(self.verification_storage_class)
        sm=sc.get('metadata',{}); spec=sc.get('provisioner') and sc
        mode=sc.get('volumeBindingMode')
        if (sm.get('name')!=self.verification_storage_class or not isinstance(sm.get('uid'),str)
            or not sm.get('resourceVersion') or not isinstance(sc.get('provisioner'),str)
            or mode not in ('Immediate','WaitForFirstConsumer')):
            _fail('verification_storage_class_invalid')
        sc_identity={'name':sm['name'],'uid':sm['uid'],'resourceVersion':sm['resourceVersion'],
            'provisioner':sc['provisioner'],'volumeBindingMode':mode}
        nonce=self.request['nonce']
        expected={'apiVersion':'v1','kind':'PersistentVolumeClaim','metadata':{'name':name,
            'namespace':kube_json_api.NAMESPACE,'labels':{'voice.io/nats-operation':self.operation,
                'voice.io/nats-job-nonce':nonce,'voice.io/nats-pvc-role':'verification'}},
            'spec':{'accessModes':['ReadWriteOnce'],'volumeMode':'Filesystem',
                'storageClassName':self.verification_storage_class,
                'resources':{'requests':{'storage':str(self.verification_pvc_size_bytes)}}}}
        self._fresh_source()
        self.persist_intent({'phase':'verification-pvc','state':'PVC_CREATE_INTENT','name':name,
            'operation':self.operation,'nonce':nonce,'storage_class':sc_identity,
            'size_bytes':self.verification_pvc_size_bytes})
        try:
            existing=self.api.get_pvc(kube_json_api.NAMESPACE,name)
        except kube_json_api.KubeNotFound:
            existing=None
        if existing is not None:
            _fail('verification_pvc_preexisting')
        current_sc=self.api.get_storage_class(self.verification_storage_class)
        current_meta=current_sc.get('metadata',{})
        if {'name':current_meta.get('name'),'uid':current_meta.get('uid'),
            'resourceVersion':current_meta.get('resourceVersion'),'provisioner':current_sc.get('provisioner'),
            'volumeBindingMode':current_sc.get('volumeBindingMode')}!=sc_identity:
            _fail('verification_storage_class_changed')
        try: existing=self.api.create_verification_pvc(self.operation,nonce,
            self.verification_pvc_size_bytes,self.verification_storage_class)
        except Exception:
            try: existing=self.api.get_pvc(kube_json_api.NAMESPACE,name)
            except kube_json_api.KubeNotFound: _fail('verification_pvc_create_unknown')
        self._verify_claim_projection(existing,expected)
        pvc_uid=existing['metadata']['uid']
        if mode=='WaitForFirstConsumer':
            self._bind_verification_claim(name,pvc_uid,nonce)
        deadline=time.monotonic()+600
        while time.monotonic()<deadline:
            current=self.api.get_pvc(kube_json_api.NAMESPACE,name)
            self._verify_claim_projection(current,expected,expected_uid=pvc_uid)
            if current.get('status',{}).get('phase')=='Bound':break
            if mode=='Immediate' and current.get('status',{}).get('phase') not in ('Pending',None):
                _fail('verification_pvc_phase_invalid')
            time.sleep(0.5)
        else: _fail('verification_pvc_bind_timeout')
        pv_name=current.get('spec',{}).get('volumeName')
        pv=self.api.get_pv(pv_name)
        pmeta,pvspec=pv.get('metadata',{}),pv.get('spec',{})
        ref=pvspec.get('claimRef',{})
        if (not isinstance(pv_name,str) or not pmeta.get('uid') or not re.fullmatch('[a-f0-9-]{36}',pmeta['uid'])
            or pv.get('status',{}).get('phase')!='Bound' or ref.get('namespace')!=kube_json_api.NAMESPACE
            or ref.get('name')!=name or ref.get('uid')!=pvc_uid
            or pvspec.get('storageClassName')!=self.verification_storage_class
            or pvspec.get('accessModes')!=['ReadWriteOnce'] or pvspec.get('volumeMode','Filesystem')!='Filesystem'):
            _fail('verification_pv_binding_invalid')
        cap=pvspec.get('capacity',{}).get('storage')
        if not _quantity_at_least(cap,self.verification_pvc_size_bytes):_fail('verification_pv_capacity_invalid')
        hostname=_pv_hostname(pvspec)
        nodes=self.api.list_nodes_by_hostname(hostname)
        nodes=[row for row in nodes if row.get('metadata',{}).get('labels',{}).get('kubernetes.io/hostname')==hostname]
        if len(nodes)!=1:_fail('verification_node_ambiguous')
        node=nodes[0]; nmeta=node.get('metadata',{})
        if (nmeta.get('name')!=self.binding['node_name'] or nmeta.get('uid') is None
            or node.get('spec',{}).get('unschedulable') is True
            or sum(1 for row in node.get('status',{}).get('conditions',[])
                if row.get('type')=='Ready' and row.get('status')=='True')!=1):
            _fail('verification_node_invalid')
        sc_now=self.api.get_storage_class(self.verification_storage_class)
        sm_now=sc_now.get('metadata',{})
        if ({'name':sm_now.get('name'),'uid':sm_now.get('uid'),'resourceVersion':sm_now.get('resourceVersion'),
            'provisioner':sc_now.get('provisioner'),'volumeBindingMode':sc_now.get('volumeBindingMode')} != sc_identity):
            _fail('verification_storage_class_changed')
        return current,pvc_uid,pv,pmeta['uid'],nmeta['name'],hostname,sc_identity

    def _verify_claim_projection(self,actual,wanted,expected_uid=None):
        if not isinstance(actual,dict):_fail('verification_pvc_identity_invalid')
        meta=actual.get('metadata',{}) if isinstance(actual,dict) else {}
        spec=actual.get('spec',{}) if isinstance(actual,dict) else {}
        if (actual.get('apiVersion')!='v1' or actual.get('kind')!='PersistentVolumeClaim'
            or meta.get('name')!=wanted['metadata']['name'] or meta.get('namespace')!=wanted['metadata']['namespace']
            or not isinstance(meta.get('uid'),str) or not re.fullmatch('[a-f0-9-]{36}',meta['uid'])
            or (expected_uid is not None and meta['uid']!=expected_uid)
            or meta.get('labels')!=wanted['metadata']['labels'] or meta.get('annotations',{}) not in ({},None)
            or spec.get('accessModes')!=['ReadWriteOnce'] or spec.get('volumeMode','Filesystem')!='Filesystem'
            or spec.get('storageClassName')!=wanted['spec']['storageClassName']
            or spec.get('resources',{}).get('requests',{}).get('storage')!=wanted['spec']['resources']['requests']['storage']):
            _fail('verification_pvc_identity_invalid')
        return actual

    def _bind_verification_claim(self,name,uid,nonce):
        import kube_json_api
        import verification_job
        request={'operation':self.operation,'nonce':nonce,'phase':'bind-verification'}
        binding={'role':'verification','claim_name':name,'target_uid':uid,
            'node_name':self.binding['node_name'],'hostname':self.binding['node_hostname']}
        wanted=verification_job.document(request,binding,self.runner_image)
        job_name=wanted['metadata']['name']
        self._fresh_source()
        try:self.api.get_job(kube_json_api.NAMESPACE,job_name)
        except kube_json_api.KubeNotFound:pass
        else:_fail('verification_binder_already_exists')
        self.persist_intent({'phase':'bind-verification','state':'JOB_CREATE_INTENT','job_name':job_name,
            'verification_uid':uid,'nonce':nonce})
        try:created=verification_job.verify_job(self.api.create_job(kube_json_api.NAMESPACE,wanted),wanted)
        except Exception:
            try:created=verification_job.verify_job(self.api.get_job(kube_json_api.NAMESPACE,job_name),wanted)
            except Exception:_fail('verification_binder_create_unknown')
        job_uid=created['metadata']['uid']
        self.persist_intent({'phase':'bind-verification','state':'JOB_CREATED','job_name':job_name,'job_uid':job_uid})
        completed=self.api.wait_job(kube_json_api.NAMESPACE,job_name,job_uid,timeout=600)
        verification_job.verify_job(completed,wanted,expected_uid=job_uid)
        if completed.get('status',{}).get('succeeded')!=1:
            _fail('verification_binder_failed')
        pod=self.api.get_job_pod(kube_json_api.NAMESPACE,job_name)
        verification_job.verify_pod(pod,wanted['spec']['template'],job_uid,'bind-verification',
            expected_node_name=self.binding['node_name'])
        if pod.get('status',{}).get('phase')!='Succeeded':_fail('verification_binder_pod_failed')
        self.persist_result({'phase':'bind-verification','state':'BIND_COMPLETE','job_name':job_name,
            'job_uid':job_uid,'pod_uid':pod.get('metadata',{}).get('uid'),'target_uid':uid})
        if self.api.delete_job(kube_json_api.NAMESPACE,job_name,job_uid) is not True:
            _fail('verification_binder_cleanup_unconfirmed')

    def _fresh_verification_storage(self,name,uid,pv_uid,sc_identity,node_name,hostname):
        import kube_json_api
        pvc=self.api.get_pvc(kube_json_api.NAMESPACE,name)
        pspec=pvc.get('spec',{})
        if (pvc.get('metadata',{}).get('uid')!=uid or pvc.get('metadata',{}).get('name')!=name
            or pvc.get('metadata',{}).get('namespace')!=kube_json_api.NAMESPACE
            or pvc.get('status',{}).get('phase')!='Bound'
            or pspec.get('accessModes')!=['ReadWriteOnce'] or pspec.get('volumeMode','Filesystem')!='Filesystem'
            or pspec.get('storageClassName')!=self.verification_storage_class
            or pspec.get('resources',{}).get('requests',{}).get('storage')!=str(self.verification_pvc_size_bytes)):
            _fail('verification_pvc_changed')
        pv=self.api.get_pv(pspec.get('volumeName'))
        pvspec=pv.get('spec',{}); claim_ref=pvspec.get('claimRef',{})
        if (pv.get('metadata',{}).get('uid')!=pv_uid
            or _pv_hostname(pvspec)!=hostname
            or pv.get('status',{}).get('phase')!='Bound'
            or claim_ref.get('namespace')!=kube_json_api.NAMESPACE
            or claim_ref.get('name')!=name or claim_ref.get('uid')!=uid
            or pvspec.get('storageClassName')!=self.verification_storage_class
            or pvspec.get('accessModes')!=['ReadWriteOnce']
            or pvspec.get('volumeMode','Filesystem')!='Filesystem'
            or not _quantity_at_least(pvspec.get('capacity',{}).get('storage'),self.verification_pvc_size_bytes)):
            _fail('verification_pv_changed')
        sc=self.api.get_storage_class(self.verification_storage_class)
        sm=sc.get('metadata',{})
        if {'name':sm.get('name'),'uid':sm.get('uid'),'resourceVersion':sm.get('resourceVersion'),
            'provisioner':sc.get('provisioner'),'volumeBindingMode':sc.get('volumeBindingMode')}!=sc_identity:
            _fail('verification_storage_class_changed')
        nodes=self.api.list_nodes_by_hostname(hostname)
        if (len(nodes)!=1 or nodes[0].get('metadata',{}).get('name')!=node_name
            or nodes[0].get('spec',{}).get('unschedulable') is True
            or sum(1 for row in nodes[0].get('status',{}).get('conditions',[])
                if row.get('type')=='Ready' and row.get('status')=='True')!=1):
            _fail('verification_node_changed')

    def _assert_census_pod(self,request,binding,wanted,job_uid,pod_name,pod_uid,container_id,sc_identity,validate):
        import verification_job
        self._fresh_source()
        self._fresh_verification_storage(binding['verification_claim'],binding['verification_uid'],
            binding['verification_pv_uid'],sc_identity,binding['verification_node_name'],binding['verification_hostname'])
        job=verification_job.verify_job(self.api.get_job('voice-staging',wanted['metadata']['name']),
            wanted,expected_uid=job_uid)
        if job.get('status',{}).get('failed',0)>0:
            _fail('census_job_failed')
        pod=self.api.get_pod('voice-staging',pod_name)
        verification_job.verify_pod(pod,wanted['spec']['template'],job_uid,'census',
            expected_node_name=binding['verification_node_name'],expected_uid=pod_uid)
        statuses=pod.get('status',{}).get('containerStatuses',[])
        if (len(statuses)!=1 or statuses[0].get('containerID')!=container_id
            or statuses[0].get('restartCount')!=0 or statuses[0].get('ready') is not True):
            _fail('census_pod_replaced')
        if validate is not None:validate()
