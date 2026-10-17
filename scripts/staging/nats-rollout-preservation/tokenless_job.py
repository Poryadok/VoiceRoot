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


def run_archive(api, request: dict, binding: dict, runner_image: str, artifact, *,
                persist_intent, persist_result, verify_archive):
    """Create the single fixed read-only-PVC archive Job and stream its result.

    If anything fails after Job creation, the Job is retained. Recovery must
    call ``reconcile_archive`` with the journaled Job and Pod UIDs; this entry
    point never blindly creates the same archive operation again.
    """
    import archive_stream
    row = validate_request(request)
    if row['phase'] != 'archive': _fail('archive_phase_required')
    storage = validate_binding(binding)
    image = validate_runner_image(runner_image)
    _verify_storage(api, storage)
    wanted = document(row, storage, image)
    name = wanted['metadata']['name']
    template_sha = hashlib.sha256(json.dumps(wanted['spec'], sort_keys=True,
        separators=(',', ':')).encode()).hexdigest()
    persist_intent({'phase': 'archive', 'nonce': row['nonce'], 'job_name': name,
        'template_sha256': template_sha, 'state': 'JOB_CREATE_INTENT'})
    created = verify_job(api.create_job(NAMESPACE, copy.deepcopy(wanted)), wanted)
    uid = created['metadata']['uid']
    persist_intent({'phase': 'archive', 'nonce': row['nonce'], 'job_name': name,
        'job_uid': uid, 'template_sha256': template_sha, 'state': 'JOB_CREATED'})
    pod = api.wait_job_pod(NAMESPACE, name, uid, timeout=120)
    _verify_pod(pod, wanted, uid, phase='Running')
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
    row = validate_request(request)
    if row['phase'] != 'archive' or not isinstance(journal, dict) or journal.get('phase') != 'archive':
        _fail('archive_recovery_binding_invalid')
    storage = validate_binding(binding)
    wanted = document(row, storage, validate_runner_image(runner_image))
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
    verify_job(api.get_job(NAMESPACE, name), wanted, created_uid=uid)
    pod = api.get_job_pod(NAMESPACE, name)
    _verify_pod(pod, wanted, uid, phase='Running')
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
    pod_meta = pod['metadata']
    _verify_storage(api, storage)
    result = archive_stream.consume(api.exec_pod_archive(NAMESPACE, pod_meta['name'], pod_meta['uid'],
        row['operation'], row['nonce'], existing=existing),
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
    _verify_pod(current, wanted, job_uid, phase='Running')
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
                 *, persist_intent, persist_result):
        self.api = api
        self.base = Path(base)
        self.binding = validate_binding(binding)
        self.runner_image = validate_runner_image(runner_image)
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

    def no_operation_containers(self, *, running_only=False):
        rows = self.api.list_operation_jobs(NAMESPACE, self.operation)
        if rows:
            _fail('operation_job_writer_present')

    def archive_selected_store(self, artifact):
        import native_store
        target = Path(artifact)
        if target != self.base / 'rollout-before.tar' or target.exists() or target.is_symlink():
            _fail('archive_target_invalid')
        result = run_archive(self.api, self.request, self.binding, self.runner_image, target,
            persist_intent=self.persist_intent, persist_result=self.persist_result,
            verify_archive=native_store.verify_archive)
        return result['manifest']

    def capture_census(self, archive, manifest, label, *, validate=None):
        # Until the source-split, target-only census ABI is installed, refusing
        # PREPARE is safer than falling through to DockerRuntime's host-backed
        # isolated_census path.
        _fail('rootless_census_phase_unavailable')
