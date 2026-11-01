"""Narrow create-only lifecycle for the protected GIS first install.

This module deliberately has no generic Kubernetes apply path. The caller must
pass an already source-authorized Service and Deployment plus trusted receipts
from the protected workflow. All mutations are create-only or UID/resourceVersion
guarded updates/deletes.
"""
from __future__ import annotations

import copy
import hashlib
import ipaddress
import json
import re
import time

NS = 'voice-staging'
ALLOWED_NAMESPACES = (NS, 'voice-dev')
SERVICE = ('Service', 'voice-gameintegration')
DEPLOYMENT = ('Deployment', 'voice-gameintegration')
IDENTITIES = (SERVICE, DEPLOYMENT)
MARKER = 'voice.poryadok.io/gis-first-install-operation'
SCHEMA = 'voice-gis-first-install-v1'
RESOURCE_RECEIPT_SCHEMA = 'voice-gis-first-install-resources-v1'
TRANSITION_RECEIPT_SCHEMA = 'voice-gis-first-install-transition-v1'
SOURCE_SCHEMA = 'voice-gis-first-install-source-v1'

def baseline_workload_names(*,include_hub):
    from stage_runtime import HUB,LEAVES
    names={'voice-'+name for name in LEAVES}|{'voice-gateway','voice-web','voice-admin',
        'voice-developer-portal','voice-p3-principal-jwks'}
    return names|({HUB} if include_hub else set())

def validate_baseline_snapshots(snapshots):
    if not isinstance(snapshots,dict) or set(snapshots)!=baseline_workload_names(include_hub=True):
        raise Blocked('gis_first_install_baseline_workload_count_invalid')
    uids=set()
    for name,row in snapshots.items():
        if not isinstance(row,dict):raise Blocked('gis_first_install_baseline_workload_count_invalid')
        metadata=row.get('metadata',{})
        if (row.get('kind')!='Deployment' or metadata.get('name')!=name
            or metadata.get('namespace')!=NS or not isinstance(metadata.get('uid'),str)
            or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}',metadata['uid'])
            or metadata['uid'] in uids):raise Blocked('gis_first_install_baseline_workload_count_invalid')
        uids.add(metadata['uid'])
    return True
PREREQUISITE_SCHEMA = 'voice-gis-prerequisite-receipt-v1'
DEV_PREREQUISITE_SCHEMA = 'voice-dev-gis-prerequisite-receipt-v1'
PREREQUISITE_SCHEMA_V2 = 'voice-gis-prerequisite-receipt-v2'
DEV_PREREQUISITE_SCHEMA_V2 = 'voice-dev-gis-prerequisite-receipt-v2'
PREREQUISITE_SCHEMA_V3 = 'voice-gis-prerequisite-receipt-v3'
DEV_PREREQUISITE_SCHEMA_V3 = 'voice-dev-gis-prerequisite-receipt-v3'


def verifier_output(version):
    result={'schema':'voice-gis-verifier-v1','database':'game_integration_db','migration_version':31,
        'dirty':False,'runtime_role':'gameintegration_runtime','role_restricted':True,
        'table_grants':'select_insert_update_delete','sequence_grants':'usage_select','ddl_denied':True,
        'schema_ledger_runtime_mutation_denied':True}
    if version in (32,33):
        result.update(schema='voice-gis-verifier-v'+str(version-30),migration_version=version,
            authority_singleton_verified=True,authority_triggers_verified=True,authority_functions_verified=True)
    elif version!=31:raise Blocked('gis_prerequisite_source_revision_unsupported')
    return result


class Blocked(RuntimeError):
    """Fail-closed operation refusal with a stable, non-sensitive code."""

    def __init__(self, code):
        super().__init__(code)
        self.code = code


def _namespace(plan=None, namespace=NS):
    value = plan.get('namespace', NS) if namespace is None and isinstance(plan, dict) else namespace
    if value not in ALLOWED_NAMESPACES:
        raise Blocked('gis_first_install_namespace_invalid')
    return value


def validate_source_plan(plan, *, namespace=NS):
    """Accept only the source compiler's exact two-object target packet."""
    namespace = _namespace(namespace=namespace)
    if not isinstance(plan, dict) or set(plan) != {'schema', 'objects'} or plan.get('schema') != SOURCE_SCHEMA:
        raise Blocked('gis_first_install_source_plan_invalid')
    objects = plan.get('objects')
    if not isinstance(objects, list) or len(objects) != 2 or {_identity(obj) for obj in objects} != set(IDENTITIES):
        raise Blocked('gis_first_install_source_pair_invalid')
    for obj in objects:
        if obj.get('metadata', {}).get('namespace', namespace) != namespace:
            raise Blocked('gis_first_install_namespace_invalid')
    return True


def bind_operation(source_plan, operation, target, *, namespace=NS):
    """Turn the source-only pair into a private, operation-specific journal."""
    namespace = _namespace(namespace=namespace)
    validate_source_plan(source_plan, namespace=namespace)
    if target.get('mode') not in ('full', 'app-only') or target.get('tag') is None:
        raise Blocked('gis_first_install_admission_invalid')
    objects = {_identity(obj): obj for obj in source_plan['objects']}
    plan = build_plan(service=objects[SERVICE], deployment=objects[DEPLOYMENT], operation=operation,
                      namespace=namespace)
    plan.update(source_sha=target['tag'], source_plan_sha256=digest(source_plan),
                source_hashes=copy.deepcopy(target.get('source_hashes', {})),
                phase='PLANNED')
    if namespace == 'voice-dev':
        build = target.get('source_build_receipt')
        image = next((row.get('image') for row in objects[DEPLOYMENT].get('spec', {}).get('template', {}).get('spec', {}).get('containers', [])
                      if isinstance(row, dict) and row.get('name') == 'gameintegration'), None)
        validate_dev_source_build_receipt(build, target['tag'], image)
        plan['source_build_receipt'] = copy.deepcopy(build)
    return plan


def validate_dev_source_build_receipt(build, source_sha, image):
    """Validate the complete producer receipt shared by DEV planning and execution."""
    expected = {'schema', 'source_sha', 'image_ref', 'build_context_sha256',
        'image_config_sha256', 'working_tree_clean', 'builder_kind', 'builder_run',
        'builder_job', 'builder_sha', 'builder_tree', 'builder_artifact_id',
        'builder_artifact_sha256', 'builder_output_index_sha256', 'builder_provenance_sha256'}
    digest = re.compile(r'[a-f0-9]{64}')
    commit = re.compile(r'[a-f0-9]{40}')
    if (not isinstance(build, dict) or set(build) != expected
        or build.get('schema') != 'voice-dev-source-build-v2'
        or build.get('source_sha') != source_sha or build.get('image_ref') != image
        or build.get('working_tree_clean') is not True
        or not isinstance(image, str)
        or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/gameintegration@sha256:[a-f0-9]{64}', image)
        or not isinstance(build.get('builder_sha'), str) or not commit.fullmatch(build['builder_sha'])
        or not isinstance(build.get('builder_tree'), str) or not commit.fullmatch(build['builder_tree'])
        or build.get('build_context_sha256') != hashlib.sha256(build['builder_tree'].encode('ascii')).hexdigest()
        or not isinstance(build.get('build_context_sha256'), str) or not digest.fullmatch(build['build_context_sha256'])
        or not isinstance(build.get('image_config_sha256'), str) or not digest.fullmatch(build['image_config_sha256'])):
        raise Blocked('gis_dev_source_build_receipt_invalid')
    if build['builder_kind'] == 'github-actions':
        if (type(build.get('builder_run')) is not int or build['builder_run'] <= 0
            or type(build.get('builder_job')) is not int or build['builder_job'] <= 0
            or type(build.get('builder_artifact_id')) is not int or build['builder_artifact_id'] <= 0
            or not isinstance(build.get('builder_artifact_sha256'), str)
            or not digest.fullmatch(build['builder_artifact_sha256'])
            or build.get('builder_output_index_sha256') != image.rsplit('@sha256:', 1)[-1]
            or not isinstance(build.get('builder_output_index_sha256'), str)
            or not digest.fullmatch(build['builder_output_index_sha256'])
            or not isinstance(build.get('builder_provenance_sha256'), str)
            or not digest.fullmatch(build['builder_provenance_sha256'])):
            raise Blocked('gis_dev_source_build_receipt_invalid')
    elif build['builder_kind'] == 'local-build':
        if (build.get('builder_run') is not None or build.get('builder_job') is not None
            or build['builder_sha'] != source_sha or build.get('builder_artifact_id') is not None
            or build.get('builder_artifact_sha256') is not None
            or build.get('builder_output_index_sha256') is not None
            or build.get('builder_provenance_sha256') is not None):
            raise Blocked('gis_dev_source_build_receipt_invalid')
    else:
        raise Blocked('gis_dev_source_build_receipt_invalid')
    return True


def validate_prerequisite_receipt(receipt, plan):
    """Validate the private root-collected, non-secret database proof."""
    if _namespace(plan, namespace=None) == 'voice-dev':
        return validate_dev_prerequisite_receipt(receipt, plan)
    if not isinstance(receipt, dict) or set(receipt) != {
        'schema', 'operation', 'source_sha', 'source_plan_sha256', 'source_ci', 'inputs_sha256',
        'inputs', 'namespace_uid', 'database', 'database_identity', 'configmaps', 'jobs',
        'verifier', 'revalidation', 'custody_binding_sha256'
    }:
        raise Blocked('gis_prerequisite_receipt_invalid')
    sha = re.compile(r'[a-f0-9]{64}')
    if (receipt['schema'] not in (PREREQUISITE_SCHEMA,PREREQUISITE_SCHEMA_V2,PREREQUISITE_SCHEMA_V3) or receipt['operation'] != plan.get('operation')
        or receipt['source_sha'] != plan.get('source_sha')
        or receipt['source_plan_sha256'] != plan.get('source_plan_sha256')
        or receipt['source_ci'] != plan.get('source_ci')
        or receipt['custody_binding_sha256'] != plan.get('custody_binding_sha256')
        or not isinstance(receipt['inputs_sha256'], str) or not sha.fullmatch(receipt['inputs_sha256'])
        or receipt['inputs'] != plan.get('inputs')
        or receipt['inputs_sha256'] != digest(receipt['inputs'])
        or not isinstance(receipt['custody_binding_sha256'], str)
        or not sha.fullmatch(receipt['custody_binding_sha256'])
        or not isinstance(receipt['namespace_uid'], str) or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', receipt['namespace_uid'])
        or receipt['database'] != 'game_integration_db'):
        raise Blocked('gis_prerequisite_receipt_binding_invalid')
    if (not isinstance(receipt['source_ci'], dict) or set(receipt['source_ci']) != {
        'run_id', 'run_attempt', 'workflow_id', 'tree_sha', 'lock_artifact_id', 'lock_sha256'}
        or type(receipt['source_ci']['run_id']) is not int or receipt['source_ci']['run_id'] <= 0
        or type(receipt['source_ci']['run_attempt']) is not int or receipt['source_ci']['run_attempt'] <= 0
        or type(receipt['source_ci']['workflow_id']) is not int
        or not isinstance(receipt['source_ci']['tree_sha'], str)
        or not re.fullmatch(r'[a-f0-9]{40}', receipt['source_ci']['tree_sha'])
        or type(receipt['source_ci']['lock_artifact_id']) is not int
        or receipt['source_ci']['lock_artifact_id'] <= 0
        or not isinstance(receipt['source_ci']['lock_sha256'], str) or not sha.fullmatch(receipt['source_ci']['lock_sha256'])):
        raise Blocked('gis_prerequisite_source_ci_invalid')
    expected_jobs = {'provision': 'voice-provision-gameintegration-runtime-role',
                     'migrate': 'voice-migrate-gameintegration-db',
                     'verify': 'voice-verify-gameintegration-db'}
    jobs = receipt['jobs']
    if not isinstance(jobs, dict) or set(jobs) != set(expected_jobs):
        raise Blocked('gis_prerequisite_jobs_invalid')
    for role, name in expected_jobs.items():
        row = jobs[role]
        if (not isinstance(row, dict) or set(row) != {'name', 'uid', 'spec_sha256', 'pod_uid', 'image_ids',
            'complete', 'succeeded', 'completion_time'}
            or row['name'] != name or not isinstance(row['uid'], str) or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', row['uid'])
            or not isinstance(row['pod_uid'], str) or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', row['pod_uid'])
            or not isinstance(row['spec_sha256'], str) or not sha.fullmatch(row['spec_sha256'])
            or not isinstance(row['image_ids'], dict) or not row['image_ids']
            or any(not isinstance(value, str) or '@sha256:' not in value for value in row['image_ids'].values())
            or row['complete'] is not True or row['succeeded'] != 1
            or not isinstance(row['completion_time'], str) or not row['completion_time']):
            raise Blocked('gis_prerequisite_job_binding_invalid')
        expected_images = plan.get('inputs', {}).get('job_images', {}).get(role, {})
        if set(row['image_ids']) != set(expected_images):
            raise Blocked('gis_prerequisite_job_binding_invalid')
        for container, image_role in expected_images.items():
            if not row['image_ids'][container].endswith(plan['inputs']['images'][image_role].split('@', 1)[1]):
                raise Blocked('gis_prerequisite_job_binding_invalid')
    verifier = receipt['verifier']
    import gis_prerequisite
    revision={PREREQUISITE_SCHEMA:31,PREREQUISITE_SCHEMA_V2:32,PREREQUISITE_SCHEMA_V3:33}[receipt['schema']]
    if gis_prerequisite.validate_source_revision(receipt['inputs'])!=revision:
        raise Blocked('gis_prerequisite_source_revision_unsupported')
    required_verifier = verifier_output(revision)
    if verifier != required_verifier:
        raise Blocked('gis_prerequisite_verifier_failed')
    expected_configmaps = plan.get('inputs', {}).get('configmaps', {})
    configmaps = receipt['configmaps']
    if not isinstance(configmaps, dict) or set(configmaps) != set(expected_configmaps):
        raise Blocked('gis_prerequisite_configmaps_invalid')
    for role, name in expected_configmaps.items():
        row = configmaps[role]
        if (not isinstance(row, dict) or set(row) != {'name', 'uid', 'spec_sha256'}
            or row['name'] != name
            or not isinstance(row['uid'], str)
            or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', row['uid'])
            or not isinstance(row['spec_sha256'], str) or not sha.fullmatch(row['spec_sha256'])):
            raise Blocked('gis_prerequisite_configmaps_invalid')
    database_identity = receipt['database_identity']
    if (not isinstance(database_identity, dict) or set(database_identity) != {'namespace_uid', 'service_uid', 'service_spec_sha256', 'endpoints'}
        or database_identity['namespace_uid'] != receipt['namespace_uid']
        or database_identity != plan.get('database_identity')
        or not all(isinstance(database_identity.get(key), str) and re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', database_identity[key])
                   for key in ('service_uid',))
        or not isinstance(database_identity['service_spec_sha256'], str) or not sha.fullmatch(database_identity['service_spec_sha256'])
        or not isinstance(database_identity['endpoints'], list) or not database_identity['endpoints']):
        raise Blocked('gis_prerequisite_database_identity_invalid')
    for endpoint in database_identity['endpoints']:
        if (not isinstance(endpoint, dict) or set(endpoint) != {'uid', 'pod_uids'}
            or not isinstance(endpoint['uid'], str) or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', endpoint['uid'])
            or not isinstance(endpoint['pod_uids'], list) or not endpoint['pod_uids']
            or endpoint['pod_uids'] != sorted(set(endpoint['pod_uids']))
            or any(not isinstance(uid, str) or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', uid) for uid in endpoint['pod_uids'])):
            raise Blocked('gis_prerequisite_database_identity_invalid')
    revalidation = receipt['revalidation']
    if (not isinstance(revalidation, dict) or set(revalidation) != {'namespace_uid', 'service_uid',
        'service_spec_sha256', 'endpoints', 'secret_metadata', 'config_metadata'}
        or revalidation['namespace_uid'] != receipt['namespace_uid']
        or revalidation['service_uid'] != database_identity['service_uid']
        or revalidation['service_spec_sha256'] != database_identity['service_spec_sha256']
        or revalidation['endpoints'] != database_identity['endpoints']
        or revalidation['secret_metadata'] != {'name': 'voice-app-secrets', **plan.get('secret_metadata', {})}
        or revalidation['config_metadata'] != plan.get('config_metadata')
        or not isinstance(revalidation['secret_metadata'], dict)
        or set(revalidation['secret_metadata']) != {'name', 'uid', 'resourceVersion'}
        or revalidation['secret_metadata']['name'] != 'voice-app-secrets'
        or not isinstance(revalidation['secret_metadata']['uid'], str)
        or not revalidation['secret_metadata']['uid']
        or not isinstance(revalidation['secret_metadata']['resourceVersion'], str)
        or not revalidation['secret_metadata']['resourceVersion']
        or not isinstance(revalidation['config_metadata'], dict)
        or set(revalidation['config_metadata']) != {'uid', 'resourceVersion'}
        or not all(isinstance(value, str) and value for value in revalidation['config_metadata'].values())):
        raise Blocked('gis_prerequisite_live_revalidation_invalid')
    return True


def validate_dev_prerequisite_receipt(receipt, plan):
    """Bind DEV DB execution to its local source-build receipt, never a CI receipt."""
    expected = {'schema', 'operation', 'source_sha', 'source_plan_sha256', 'source_build_receipt',
        'source_build_receipt_sha256', 'inputs_sha256', 'inputs', 'namespace_uid', 'database',
        'database_identity', 'configmaps', 'jobs', 'verifier', 'revalidation'}
    if not isinstance(receipt, dict) or set(receipt) != expected:
        raise Blocked('gis_dev_prerequisite_receipt_invalid')
    build = plan.get('source_build_receipt')
    deployment = next((obj for obj in plan.get('objects', []) if _identity(obj) == DEPLOYMENT), {})
    images = {row.get('image') for row in deployment.get('spec', {}).get('template', {}).get('spec', {}).get('containers', [])
              if isinstance(row, dict)}
    sha = re.compile(r'[a-f0-9]{64}')
    uid = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
    if (receipt['schema'] not in (DEV_PREREQUISITE_SCHEMA,DEV_PREREQUISITE_SCHEMA_V2,DEV_PREREQUISITE_SCHEMA_V3) or receipt['operation'] != plan.get('operation')
        or receipt['source_sha'] != plan.get('source_sha') or receipt['source_plan_sha256'] != plan.get('source_plan_sha256')
        or receipt['source_build_receipt'] != build or not isinstance(build, dict)
        or not _dev_source_build_receipt_valid(build, plan.get('source_sha'), build.get('image_ref'))
        or receipt['source_build_receipt_sha256'] != digest(build)
        or build.get('image_ref') not in images
        or not isinstance(build.get('image_ref'), str)
        or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/gameintegration@sha256:[a-f0-9]{64}', build['image_ref'])
        or not isinstance(build.get('build_context_sha256'), str) or not sha.fullmatch(build['build_context_sha256'])
        or not isinstance(build.get('image_config_sha256'), str) or not sha.fullmatch(build['image_config_sha256'])
        or not isinstance(receipt['inputs_sha256'], str) or not sha.fullmatch(receipt['inputs_sha256'])
        or receipt['inputs'] != plan.get('inputs') or receipt['inputs_sha256'] != digest(receipt['inputs'])
        or not isinstance(receipt['namespace_uid'], str) or not uid.fullmatch(receipt['namespace_uid'])
        or receipt['database'] != 'game_integration_db'):
        raise Blocked('gis_dev_prerequisite_receipt_binding_invalid')
    identity = receipt['database_identity']
    if (not isinstance(identity, dict) or set(identity) != {'namespace_uid', 'service_uid', 'service_spec_sha256', 'endpoints'}
        or identity.get('namespace_uid') != receipt['namespace_uid'] or identity != plan.get('database_identity')
        or not isinstance(identity.get('service_uid'), str) or not uid.fullmatch(identity['service_uid'])
        or not isinstance(identity.get('service_spec_sha256'), str) or not sha.fullmatch(identity['service_spec_sha256'])
        or not isinstance(identity.get('endpoints'), list) or not identity['endpoints']):
        raise Blocked('gis_dev_prerequisite_database_identity_invalid')
    expected_configmaps = {'provision': plan.get('inputs', {}).get('configmaps', {}).get('provision'),
        'migrations': plan.get('inputs', {}).get('configmaps', {}).get('migrations'),
        'verify': plan.get('inputs', {}).get('configmaps', {}).get('verify')}
    configmaps = receipt['configmaps']
    if not isinstance(configmaps, dict) or set(configmaps) != set(expected_configmaps):
        raise Blocked('gis_dev_prerequisite_configmaps_invalid')
    for role, name in expected_configmaps.items():
        row = configmaps[role]
        if (not isinstance(row, dict) or set(row) != {'name', 'uid', 'spec_sha256'} or row['name'] != name
            or not isinstance(row['uid'], str) or not uid.fullmatch(row['uid'])
            or not isinstance(row['spec_sha256'], str) or not sha.fullmatch(row['spec_sha256'])):
            raise Blocked('gis_dev_prerequisite_configmaps_invalid')
    expected_jobs = {'provision': 'voice-provision-gameintegration-runtime-role',
        'migrate': 'voice-migrate-gameintegration-db', 'verify': 'voice-verify-gameintegration-db'}
    jobs = receipt['jobs']
    if not isinstance(jobs, dict) or set(jobs) != set(expected_jobs):
        raise Blocked('gis_dev_prerequisite_jobs_invalid')
    for role, name in expected_jobs.items():
        row = jobs[role]
        if (not isinstance(row, dict) or set(row) != {'name', 'uid', 'spec_sha256', 'pod_uid', 'image_ids',
            'complete', 'succeeded', 'completion_time'} or row['name'] != name
            or not isinstance(row['uid'], str) or not uid.fullmatch(row['uid'])
            or not isinstance(row['pod_uid'], str) or not uid.fullmatch(row['pod_uid'])
            or not isinstance(row['spec_sha256'], str) or not sha.fullmatch(row['spec_sha256'])
            or not isinstance(row['image_ids'], dict) or not row['image_ids']
            or any(not isinstance(value, str) or '@sha256:' not in value for value in row['image_ids'].values())
            or row['complete'] is not True or type(row['succeeded']) is not int or row['succeeded'] != 1
            or not isinstance(row['completion_time'], str) or not row['completion_time']):
            raise Blocked('gis_dev_prerequisite_job_binding_invalid')
        wanted = plan.get('inputs', {}).get('job_images', {}).get(role, {})
        pinned_images = plan.get('inputs', {}).get('images', {})
        if set(row['image_ids']) != set(wanted) or any(
            not isinstance(pinned_images.get(logical), str)
            or '@sha256:' not in pinned_images[logical]
            or not row['image_ids'][key].endswith(pinned_images[logical].split('@', 1)[1])
            for key, logical in wanted.items()):
            raise Blocked('gis_dev_prerequisite_job_image_invalid')
    verifier = receipt['verifier']
    import gis_prerequisite
    revision={DEV_PREREQUISITE_SCHEMA:31,DEV_PREREQUISITE_SCHEMA_V2:32,DEV_PREREQUISITE_SCHEMA_V3:33}[receipt['schema']]
    if gis_prerequisite.validate_source_revision(receipt['inputs'])!=revision:
        raise Blocked('gis_prerequisite_source_revision_unsupported')
    required_verifier = verifier_output(revision)
    if verifier != required_verifier:
        raise Blocked('gis_dev_prerequisite_verifier_invalid')
    revalidation = receipt['revalidation']
    if (not isinstance(revalidation, dict) or revalidation != {
        'namespace_uid': identity['namespace_uid'], 'service_uid': identity['service_uid'],
        'service_spec_sha256': identity['service_spec_sha256'], 'endpoints': identity['endpoints'],
        'secret_metadata': {'name': 'voice-app-secrets', **plan.get('secret_metadata', {})},
        'config_metadata': plan.get('config_metadata')}):
        raise Blocked('gis_dev_prerequisite_revalidation_invalid')
    return True


def _dev_source_build_receipt_valid(build, source_sha, image):
    try:
        validate_dev_source_build_receipt(build, source_sha, image)
        return True
    except (Blocked, KeyError, TypeError, ValueError):
        return False


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True)


def digest(value):
    return hashlib.sha256(canonical(value).encode()).hexdigest()


def semantic(obj):
    """Ignore API server metadata while retaining the complete object spec."""
    return {'apiVersion': obj.get('apiVersion'), 'kind': obj.get('kind'),
            'metadata': {'name': obj.get('metadata', {}).get('name'),
                         'namespace': obj.get('metadata', {}).get('namespace'),
                         'labels': copy.deepcopy(obj.get('metadata', {}).get('labels', {})),
                         'annotations': copy.deepcopy(obj.get('metadata', {}).get('annotations', {}))},
            'spec': copy.deepcopy(obj.get('spec', {}))}


def _identity(obj):
    if not isinstance(obj, dict) or not isinstance(obj.get('metadata'), dict):
        raise Blocked('gis_first_install_manifest_invalid')
    return obj.get('kind'), obj['metadata'].get('name')


def _not_found(error):
    """Accept only explicit Kubernetes 404 signals, never message matching."""
    status = getattr(error, 'status', None)
    code = getattr(error, 'status_code', None)
    return status == 404 or code == 404 or type(error).__name__ in ('NotFound', 'ApiException404')


def _conflict(error):
    return getattr(error, 'status', None) == 409 or getattr(error, 'status_code', None) == 409


def build_plan(*, service, deployment, operation, namespace=NS):
    """Bind only the two approved resource specs.

    This helper is not admission. The root bridge must complete source/CI,
    stack-lock, preservation, NATS custody, and GIS DB prerequisite checks
    before calling it, and keep those verified records in its private journal.
    """
    namespace = _namespace(namespace=namespace)
    if not isinstance(operation, str) or not re.fullmatch(r'[a-f0-9]{12}', operation):
        raise Blocked('gis_first_install_operation_invalid')
    if _identity(service) != SERVICE or _identity(deployment) != DEPLOYMENT:
        raise Blocked('gis_first_install_resource_scope_invalid')
    service = copy.deepcopy(service)
    deployment = copy.deepcopy(deployment)
    for obj in (service, deployment):
        metadata = obj['metadata']
        if metadata.get('namespace', namespace) != namespace:
            raise Blocked('gis_first_install_namespace_invalid')
        metadata['namespace'] = namespace
        annotations = metadata.setdefault('annotations', {})
        if MARKER in annotations:
            raise Blocked('gis_first_install_source_marker_forbidden')
        annotations[MARKER] = operation
    if deployment.get('apiVersion') != 'apps/v1' or service.get('apiVersion') != 'v1':
        raise Blocked('gis_first_install_api_version_invalid')
    spec = deployment.setdefault('spec', {})
    replicas = spec.get('replicas', 1)
    if type(replicas) is not int or replicas != 1:
        raise Blocked('gis_first_install_source_replicas_invalid')
    spec['replicas'] = 0
    objects = {SERVICE: service, DEPLOYMENT: deployment}
    plan = {'schema': SCHEMA, 'operation': operation,
            'spec_sha256': {f'{kind}/{name}': digest(semantic(obj)) for (kind, name), obj in objects.items()},
            'objects': [service, deployment], 'phase': 'PLANNED', 'created': {}}
    if namespace != NS:
        plan['namespace'] = namespace
    return plan


def _read_pair(kube, plan):
    if hasattr(kube, 'run'):
        try:
            response = kube.run(['get', 'services,deployments', '-o', 'json'])
            items = response.get('items') if isinstance(response, dict) and set(response) == {'apiVersion', 'items', 'kind', 'metadata'} else None
            if not isinstance(items, list) or len(items) > 2048:
                raise ValueError()
            found = {(row.get('kind'), row.get('metadata', {}).get('name')): row for row in items
                     if isinstance(row, dict) and isinstance(row.get('metadata'), dict)}
            return {identity: found.get(identity) for identity in IDENTITIES}
        except Blocked:
            raise
        except Exception:
            raise Blocked('gis_first_install_live_read_failed') from None
    found = {}
    for kind, name in IDENTITIES:
        try:
            found[(kind, name)] = kube.get(kind, name)
        except Exception as error:
            if _not_found(error):
                found[(kind, name)] = None
            else:
                raise Blocked('gis_first_install_live_read_failed') from None
    return found


def _read_one(kube, kind, name):
    return _read_pair(kube, None)[(kind, name)]


def preflight(kube, plan):
    if not isinstance(plan, dict) or plan.get('schema') != SCHEMA:
        raise Blocked('gis_first_install_plan_invalid')
    current = _read_pair(kube, plan)
    for identity, obj in current.items():
        if obj is not None:
            raise Blocked('gis_first_install_identity_already_present')
    return True


def _matches_owned(live, intended, operation, allow_service_allocation=False):
    metadata = live.get('metadata', {})
    actual = semantic(live)
    wanted = semantic(intended)
    if allow_service_allocation and _identity(intended)==SERVICE:
        try:
            if (wanted['metadata']['namespace']!=NS or 'clusterIP' in wanted['spec']
                or 'clusterIPs' in wanted['spec'] or wanted['spec'].get('type')!='ClusterIP'):
                return False
            primary=actual['spec'].pop('clusterIP');addresses=actual['spec'].pop('clusterIPs')
            if (not isinstance(addresses,list) or not 1<=len(addresses)<=2 or primary!=addresses[0]
                or len(set(addresses))!=len(addresses)):
                return False
            parsed=[ipaddress.ip_address(address) for address in addresses]
            if any(address.is_unspecified or address.is_multicast for address in parsed):return False
            families=actual['spec'].get('ipFamilies')
            if families is not None and families!=['IPv4' if address.version==4 else 'IPv6' for address in parsed]:return False
        except (KeyError,ValueError,TypeError):return False
    # The only permitted post-create mutation is this operation's guarded
    # 0-to-1 start. Compare all other fields against the paused spec.
    if _identity(intended) == DEPLOYMENT:
        actual['spec']['replicas'] = 0
        wanted['spec']['replicas'] = 0
    return (metadata.get('uid') and metadata.get('resourceVersion')
            and metadata.get('annotations', {}).get(MARKER) == operation
            and actual == wanted)


def _record(plan, save, kind, name, live):
    if not _matches_owned(live, next(obj for obj in plan['objects'] if _identity(obj) == (kind, name)), plan['operation'],plan.get('service_allocation')=='server'):
        raise Blocked('gis_first_install_created_object_mismatch')
    plan['created'][f'{kind}/{name}'] = {
        'uid': live['metadata']['uid'], 'resourceVersion': str(live['metadata']['resourceVersion']),
        'spec_sha256': digest(semantic(live))}
    plan['phase'] = 'CREATING'
    save(copy.deepcopy(plan))


def _create(kube, intended):
    if hasattr(kube, 'create'):
        return kube.create(copy.deepcopy(intended))
    return kube.run(['create', '-f', '/dev/stdin', '-o', 'json'], body=intended)


def _start(kube, deployment):
    if hasattr(kube, 'start'):
        return kube.start(deployment, replicas=1,
                          expected_uid=deployment['metadata']['uid'],
                          expected_resource_version=str(deployment['metadata']['resourceVersion']))
    patch = [{'op': 'test', 'path': '/metadata/uid', 'value': deployment['metadata']['uid']},
             {'op': 'test', 'path': '/metadata/resourceVersion', 'value': deployment['metadata']['resourceVersion']},
             {'op': 'replace', 'path': '/spec/replicas', 'value': 1}]
    return kube.run(['patch', 'deployment', deployment['metadata']['name'], '--type=json',
                     '--patch', json.dumps(patch), '-o', 'json'])


def _delete(kube, kind, name, uid, resource_version):
    if hasattr(kube, 'delete'):
        return kube.delete(kind, name, uid=uid, resource_version=resource_version)
    preconditions = 'uid=' + uid + ',resourceVersion=' + str(resource_version)
    return kube.run(['delete', kind.lower(), name, '--preconditions=' + preconditions, '-o', 'json'])


def execute(kube, plan, save, *, start=True):
    """Create the pair, adopt only exact same-operation crash leftovers, optionally start."""
    if plan.get('schema') != SCHEMA or plan.get('phase') not in ('PLANNED', 'CREATING', 'STARTING'):
        raise Blocked('gis_first_install_phase_invalid')
    validate_prerequisite_receipt(plan.get('prerequisite_receipt'), plan)
    if not plan.get('created'):
        save(copy.deepcopy(plan))  # durable intent before the first mutation
    # The pair is checked again as a unit immediately before any create. Each
    # create then has its own last-moment GET and create-only API request.
    pair = _read_pair(kube, plan)
    for kind, name in IDENTITIES:
        key = f'{kind}/{name}'; live = pair[(kind, name)]
        record = plan.get('created', {}).get(key)
        intended = next(obj for obj in plan['objects'] if _identity(obj) == (kind, name))
        if live is None:
            if record is not None:
                raise Blocked('gis_first_install_owned_object_missing')
            continue
        if not _matches_owned(live, intended, plan['operation'],plan.get('service_allocation')=='server'):
            raise Blocked('gis_first_install_create_race')
        if record is not None and live['metadata']['uid'] != record['uid']:
            raise Blocked('gis_first_install_owned_object_drift')
        if record is not None and plan.get('phase') != 'STARTING' and digest(semantic(live)) != record.get('spec_sha256'):
            raise Blocked('gis_first_install_owned_object_drift')
    for kind, name in IDENTITIES:
        key = f'{kind}/{name}'
        intended = next(obj for obj in plan['objects'] if _identity(obj) == (kind, name))
        live = _read_one(kube, kind, name)
        if key in plan['created']:
            recorded = plan['created'][key]
            if (kind, name) == DEPLOYMENT and plan.get('phase') == 'STARTING' and live is not None:
                if (not _matches_owned(live, intended, plan['operation'],plan.get('service_allocation')=='server')
                    or live['metadata']['uid'] != recorded['uid']
                    or live.get('spec', {}).get('replicas') != 1):
                    raise Blocked('gis_first_install_owned_object_drift')
                recorded['resourceVersion'] = str(live['metadata']['resourceVersion'])
                recorded['spec_sha256'] = digest(semantic(live))
                save(copy.deepcopy(plan))
                continue
            if (live is None or not _matches_owned(live, intended, plan['operation'],plan.get('service_allocation')=='server')
                or live['metadata']['uid'] != recorded['uid']
                or str(live['metadata']['resourceVersion']) != recorded['resourceVersion']
                or digest(semantic(live)) != recorded.get('spec_sha256')):
                raise Blocked('gis_first_install_owned_object_drift')
            continue
        if live is not None:
            # Recover a response lost after create, but only in the same journal.
            if not _matches_owned(live, intended, plan['operation'],plan.get('service_allocation')=='server'):
                raise Blocked('gis_first_install_create_race')
            _record(plan, save, kind, name, live)
            continue
        try:
            created = _create(kube, intended)
        except Exception as error:
            # A failed response is ambiguous. Re-read; only exact operation-owned
            # content can be enrolled into this same operation.
            try:
                observed = _read_one(kube, kind, name)
            except Exception:
                raise Blocked('gis_first_install_create_outcome_unknown') from None
            if observed is None:
                raise Blocked('gis_first_install_create_outcome_unknown') from None
            if not _matches_owned(observed, intended, plan['operation'],plan.get('service_allocation')=='server'):
                code = 'gis_first_install_create_race' if _conflict(error) else 'gis_first_install_create_outcome_unknown'
                raise Blocked(code) from None
            created = observed
        _record(plan, save, kind, name, created)
    current = _read_pair(kube, plan)
    for (kind, name), live in current.items():
        key = f'{kind}/{name}'
        recorded = plan['created'].get(key, {})
        intended = next(obj for obj in plan['objects'] if _identity(obj) == (kind, name))
        if (live is None or not _matches_owned(live, intended, plan['operation'],plan.get('service_allocation')=='server')
            or live['metadata']['uid'] != recorded.get('uid')
            or str(live['metadata']['resourceVersion']) != recorded.get('resourceVersion')):
            raise Blocked('gis_first_install_pair_revalidation_failed')
    if not start:
        plan['phase'] = 'CREATED'
        plan['receipt'] = make_receipt(plan, current[SERVICE], current[DEPLOYMENT])
        save(copy.deepcopy(plan))
        return copy.deepcopy(plan['receipt'])
    plan['phase'] = 'CREATED'
    save(copy.deepcopy(plan))
    return start_owned(kube, plan, save)


def start_owned(kube, plan, save):
    """Start only the same paused pair after standard apply, under UID/RV custody."""
    if plan.get('schema') != SCHEMA or plan.get('phase') not in ('CREATED', 'STARTING', 'STARTED'):
        raise Blocked('gis_first_install_start_phase_invalid')
    validate_prerequisite_receipt(plan.get('prerequisite_receipt'), plan)
    current = _read_pair(kube, plan)
    for kind, name in IDENTITIES:
        key=f'{kind}/{name}'; live=current[(kind,name)]
        record=plan.get('created',{}).get(key)
        intended=next(obj for obj in plan['objects'] if _identity(obj)==(kind,name))
        if (live is None or record is None or live.get('metadata',{}).get('uid')!=record.get('uid')
            or not _matches_owned(live,intended,plan['operation'],plan.get('service_allocation')=='server')):
            raise Blocked('gis_first_install_start_object_changed')
        replicas=live.get('spec',{}).get('replicas',0)
        if (kind,name)==DEPLOYMENT:
            if plan.get('phase') in ('STARTING', 'STARTED') and replicas!=1:
                raise Blocked('gis_first_install_start_object_changed')
            if plan.get('phase')=='CREATED' and replicas!=0:
                raise Blocked('gis_first_install_start_object_changed')
        elif digest(semantic(live))!=digest(semantic(intended)):
            raise Blocked('gis_first_install_start_object_changed')
        # Standard apply may advance resourceVersion, but only while preserving
        # this exact source object and UID. Persist the observed baseline before
        # the guarded start mutation.
        record['resourceVersion']=str(live['metadata']['resourceVersion'])
        record['spec_sha256']=digest(semantic(live))
    save(copy.deepcopy(plan))
    deployment=current[DEPLOYMENT]
    if plan.get('phase') == 'STARTED':
        if plan.get('receipt') != make_receipt(plan, current[SERVICE], deployment):
            raise Blocked('gis_first_install_start_result_invalid')
        return copy.deepcopy(plan['receipt'])
    plan['phase'] = 'STARTING'
    save(copy.deepcopy(plan))
    if deployment.get('spec', {}).get('replicas') == 1:
        # Recover a crash after the guarded start but before its journal save.
        started = deployment
    else:
        started = _start(kube, deployment)
    if (started.get('metadata', {}).get('uid') != deployment['metadata']['uid']
        or started.get('spec', {}).get('replicas') != 1
        or started.get('metadata', {}).get('annotations', {}).get(MARKER) != plan['operation']):
        raise Blocked('gis_first_install_start_result_invalid')
    plan['created'][f'{DEPLOYMENT[0]}/{DEPLOYMENT[1]}']['resourceVersion'] = str(started['metadata']['resourceVersion'])
    plan['created'][f'{DEPLOYMENT[0]}/{DEPLOYMENT[1]}']['spec_sha256'] = digest(semantic(started))
    plan['phase'] = 'STARTED'
    plan['receipt'] = make_receipt(plan, current[SERVICE], started)
    save(copy.deepcopy(plan))
    return copy.deepcopy(plan['receipt'])


def make_receipt(plan, service, deployment):
    """Return resource ownership evidence, not a full authorization receipt."""
    return {'schema': RESOURCE_RECEIPT_SCHEMA, 'operation': plan['operation'],
            'objects': {f'{kind}/{name}': {'uid': obj['metadata']['uid'],
                'spec_sha256': digest(semantic(obj))} for (kind, name), obj in
                ((SERVICE, service), (DEPLOYMENT, deployment))}}


def validate_receipt(receipt, kube, *, operation):
    """Revalidate the owned object pair; root separately validates full receipt."""
    if (not isinstance(receipt, dict) or receipt.get('schema') != RESOURCE_RECEIPT_SCHEMA
        or receipt.get('operation') != operation
        or set(receipt.get('objects', {})) != {f'{k}/{n}' for k, n in IDENTITIES}):
        raise Blocked('gis_first_install_receipt_invalid')
    for kind, name in IDENTITIES:
        key = f'{kind}/{name}'; expected = receipt['objects'][key]
        try: live = _read_one(kube, kind, name)
        except Exception: raise Blocked('gis_first_install_receipt_object_missing') from None
        if live is None: raise Blocked('gis_first_install_receipt_object_missing')
        if (live.get('metadata', {}).get('uid') != expected.get('uid')
            or digest(semantic(live)) != expected.get('spec_sha256')
            or live.get('metadata', {}).get('annotations', {}).get(MARKER) != operation):
            raise Blocked('gis_first_install_receipt_object_changed')
    return True


def verify_ready(kube, plan, *, timeout=300, clock=time.monotonic, sleep=time.sleep):
    """Wait for the operation-owned GIS Deployment and its Service endpoint."""
    if (not isinstance(plan, dict) or plan.get('schema') != SCHEMA
        or plan.get('phase') != 'STARTED' or timeout <= 0 or timeout > 600):
        raise Blocked('gis_first_install_ready_plan_invalid')
    resource_receipt = plan.get('receipt')
    validate_receipt(resource_receipt, kube, operation=plan.get('operation'))
    service_spec = next(obj for obj in plan['objects'] if _identity(obj) == SERVICE)
    deployment_spec = next(obj for obj in plan['objects'] if _identity(obj) == DEPLOYMENT)
    resource_rows = resource_receipt['objects']
    service_uid = resource_rows['Service/voice-gameintegration']['uid']
    deployment_uid = resource_rows['Deployment/voice-gameintegration']['uid']
    deadline = clock() + timeout
    while clock() < deadline:
        pair = _read_pair(kube, plan)
        service, deployment = pair[SERVICE], pair[DEPLOYMENT]
        if (service is None or deployment is None
            or service.get('metadata', {}).get('uid') != service_uid
            or deployment.get('metadata', {}).get('uid') != deployment_uid
            or not _matches_owned(service, service_spec, plan['operation'],plan.get('service_allocation')=='server')
            or not _matches_owned(deployment, deployment_spec, plan['operation'],plan.get('service_allocation')=='server')):
            raise Blocked('gis_first_install_ready_resource_changed')
        status = deployment.get('status', {})
        generation = deployment.get('metadata', {}).get('generation')
        if (type(generation) is int and status.get('observedGeneration', 0) >= generation
            and status.get('readyReplicas', 0) == 1 and status.get('updatedReplicas', 0) == 1
            and status.get('availableReplicas', 0) == 1):
            endpoints = _ready_service_endpoints(kube, service, deployment_uid,
                                                 namespace=_namespace(plan, namespace=None))
            if endpoints:
                return {'schema': 'voice-gis-first-install-ready-v1',
                    'namespace': _namespace(plan, namespace=None), 'service_uid': service_uid,
                    'service_spec_sha256': digest(semantic(service)),
                    'deployment_uid': deployment_uid,
                    'deployment_spec_sha256': digest(semantic(deployment)),
                    'endpoints': endpoints}
        sleep(min(0.5, max(0, deadline-clock())))
    raise Blocked('gis_first_install_ready_timeout')


def _ready_service_endpoints(kube, service, deployment_uid, *, namespace=NS):
    namespace = _namespace(namespace=namespace)
    response = kube.run(['get', 'endpointslices', '-l',
        'kubernetes.io/service-name=voice-gameintegration', '-o', 'json'])
    rows = response.get('items') if isinstance(response, dict) else None
    if not isinstance(rows, list) or len(rows) > 64:
        raise Blocked('gis_first_install_endpoint_inventory_invalid')
    selector = service.get('spec', {}).get('selector')
    if not isinstance(selector, dict) or not selector:
        raise Blocked('gis_first_install_service_selector_invalid')
    replica_rows = kube.run(['get', 'replicasets', '-o', 'json']).get('items')
    if not isinstance(replica_rows, list) or len(replica_rows) > 2048:
        raise Blocked('gis_first_install_replicaset_inventory_invalid')
    owned_rs = {row.get('metadata', {}).get('uid') for row in replica_rows
        if any(owner.get('kind') == 'Deployment' and owner.get('uid') == deployment_uid
               and owner.get('controller') is True for owner in row.get('metadata', {}).get('ownerReferences', []))}
    result = []
    for row in rows:
        slice_uid = row.get('metadata', {}).get('uid')
        if not isinstance(slice_uid, str) or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', slice_uid):
            raise Blocked('gis_first_install_endpoint_identity_invalid')
        for endpoint in row.get('endpoints', []):
            if endpoint.get('conditions', {}).get('ready') is not True:
                continue
            target = endpoint.get('targetRef', {})
            if (target.get('kind') != 'Pod' or not isinstance(target.get('uid'), str)
                or not isinstance(target.get('name'), str)):
                raise Blocked('gis_first_install_endpoint_identity_invalid')
            pod = kube.get('pod', target['name'])
            owners = pod.get('metadata', {}).get('ownerReferences', [])
            rs_uids = {owner.get('uid') for owner in owners
                       if owner.get('kind') == 'ReplicaSet' and owner.get('controller') is True}
            if (pod.get('metadata', {}).get('uid') != target['uid']
                or pod.get('metadata', {}).get('namespace') != namespace
                or any(pod.get('metadata', {}).get('labels', {}).get(key) != value for key, value in selector.items())
                or not any(condition.get('type') == 'Ready' and condition.get('status') == 'True'
                           for condition in pod.get('status', {}).get('conditions', []))
                or len(rs_uids) != 1 or not rs_uids.issubset(owned_rs)):
                raise Blocked('gis_first_install_endpoint_pod_invalid')
            result.append({'slice_uid': slice_uid, 'pod_uid': target['uid'],
                           'replicaset_uid': next(iter(rs_uids))})
    if len(result) > 1:
        identities = {(row['slice_uid'], row['pod_uid'], row['replicaset_uid']) for row in result}
        if len(identities) != len(result):
            raise Blocked('gis_first_install_endpoint_duplicate')
    return sorted(result, key=lambda row: (row['slice_uid'], row['pod_uid']))


def normalized_binding(plan):
    return digest({'objects':plan['objects'],'service_allocation':plan.get('service_allocation')})


def validate_normalized_binding(target, plan):
    try:
        if (target.get('gis_first_install_normalized_sha256') != normalized_binding(plan)
                or target.get('gis_first_install_sha256') != plan['source_plan_sha256']):
            raise ValueError()
    except (KeyError,TypeError,ValueError):
        raise Blocked('gis_first_install_normalized_binding_changed') from None


def transition_receipt(plan, ready, *, source_ci, custody, target_mode):
    """Bind the 22-to-23 addition to the database, NATS and workload proofs."""
    validate_prerequisite_receipt(plan.get('prerequisite_receipt'), plan)
    if (not isinstance(source_ci, dict) or source_ci != plan.get('source_ci')
        or target_mode not in ('full', 'app-only')
        or not isinstance(custody, dict)
        or custody != plan.get('custody')
        or digest(custody) != plan.get('custody_binding_sha256')
        or not isinstance(ready, dict) or ready.get('schema') != 'voice-gis-first-install-ready-v1'
        or ready.get('service_uid') != plan.get('receipt', {}).get('objects', {}).get('Service/voice-gameintegration', {}).get('uid')
        or ready.get('deployment_uid') != plan.get('receipt', {}).get('objects', {}).get('Deployment/voice-gameintegration', {}).get('uid')):
        raise Blocked('gis_first_install_transition_binding_invalid')
    payload = {'schema': TRANSITION_RECEIPT_SCHEMA, 'operation': plan['operation'],
        'source_sha': plan['source_sha'], 'source_plan_sha256': plan['source_plan_sha256'],
        'source_ci': copy.deepcopy(source_ci), 'target_mode': target_mode,
        'namespace': _namespace(plan, namespace=None), 'namespace_uid': plan['prerequisite_receipt']['namespace_uid'],
        'database_prerequisite_sha256': digest(plan['prerequisite_receipt']),
        'resource_receipt': copy.deepcopy(plan['receipt']), 'ready': copy.deepcopy(ready),
        'custody': copy.deepcopy(custody), 'workload_transition': {'from': 22, 'to': 23}}
    return {**payload, 'receipt_sha256': digest(payload)}


def validate_transition_receipt(receipt, state):
    expected_keys = {'schema', 'operation', 'source_sha', 'source_plan_sha256', 'source_ci',
        'target_mode', 'namespace', 'namespace_uid', 'database_prerequisite_sha256',
        'resource_receipt', 'ready', 'custody', 'workload_transition', 'receipt_sha256'}
    if not isinstance(receipt, dict) or set(receipt) != expected_keys:
        raise Blocked('gis_first_install_transition_receipt_invalid')
    gis_plan = state.get('gis_first_install')
    payload = {key: value for key, value in receipt.items() if key != 'receipt_sha256'}
    if (gis_plan is None or receipt['schema'] != TRANSITION_RECEIPT_SCHEMA
        or receipt['operation'] != state.get('operation')
        or receipt['source_sha'] != gis_plan.get('source_sha')
        or receipt['source_plan_sha256'] != gis_plan.get('source_plan_sha256')
        or receipt['source_ci'] != gis_plan.get('source_ci')
        or receipt['target_mode'] != state.get('target', {}).get('mode')
        or receipt['namespace'] != _namespace(gis_plan, namespace=None)
        or receipt['namespace_uid'] != gis_plan.get('prerequisite_receipt', {}).get('namespace_uid')
        or receipt['database_prerequisite_sha256'] != digest(gis_plan.get('prerequisite_receipt'))
        or receipt['resource_receipt'] != gis_plan.get('receipt')
        or receipt['custody'] != gis_plan.get('custody')
        or digest(receipt['custody']) != gis_plan.get('custody_binding_sha256')
        or receipt['workload_transition'] != {'from': 22, 'to': 23}
        or not isinstance(receipt['receipt_sha256'], str)
        or not re.fullmatch(r'[a-f0-9]{64}', receipt['receipt_sha256'])
        or receipt['receipt_sha256'] != digest(payload)):
        raise Blocked('gis_first_install_transition_receipt_binding_invalid')
    custody = receipt['custody']
    custody_uid = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
    custody_sha = re.compile(r'[a-f0-9]{64}')
    if (not isinstance(custody, dict) or set(custody) != {'operation', 'namespace_uid', 'marker_uid',
        'claim_uid', 'pv_uid', 'target_sha', 'archive_sha256', 'manifest_sha256', 'census_sha256',
        'baseline_workload_count', 'baseline_snapshots_sha256', 'off_node_verified'}
        or custody['operation'] != receipt['operation']
        or any(not isinstance(custody.get(key), str) or not custody_uid.fullmatch(custody[key])
               for key in ('namespace_uid', 'marker_uid', 'claim_uid', 'pv_uid'))
        or custody['namespace_uid'] != receipt['namespace_uid']
        or not isinstance(custody['target_sha'], str) or not re.fullmatch(r'[a-f0-9]{40}', custody['target_sha'])
        or any(not isinstance(custody.get(key), str) or not custody_sha.fullmatch(custody[key])
               for key in ('archive_sha256', 'manifest_sha256', 'census_sha256', 'baseline_snapshots_sha256'))
        or custody['baseline_workload_count'] not in (22,24) or type(custody['baseline_workload_count']) is not int
        or custody['off_node_verified'] is not True):
        raise Blocked('gis_first_install_transition_custody_invalid')
    if custody['baseline_workload_count']==24:
        snapshots=gis_plan.get('baseline_snapshots')
        validate_baseline_snapshots(snapshots)
        if (snapshots!=state.get('context',{}).get('original_snapshots')
                or digest(snapshots)!=custody['baseline_snapshots_sha256']):
            raise Blocked('gis_first_install_transition_custody_invalid')
        validate_normalized_binding(state.get('target',{}),gis_plan)
    ready = receipt['ready']
    if (not isinstance(ready, dict) or set(ready) != {'schema', 'namespace', 'service_uid',
        'service_spec_sha256', 'deployment_uid', 'deployment_spec_sha256', 'endpoints'}
        or ready['schema'] != 'voice-gis-first-install-ready-v1'
        or ready['namespace'] != _namespace(gis_plan, namespace=None)
        or ready['service_uid'] != receipt['resource_receipt']['objects']['Service/voice-gameintegration']['uid']
        or ready['deployment_uid'] != receipt['resource_receipt']['objects']['Deployment/voice-gameintegration']['uid']
        or not isinstance(ready['endpoints'], list) or not ready['endpoints']):
        raise Blocked('gis_first_install_transition_readiness_invalid')
    return True


def validate_next_operation(receipt, source_state, kube):
    """Authorize the ordinary follow-up only from the completed first install."""
    if (not isinstance(source_state, dict) or source_state.get('status') != 'PASS'
        or source_state.get('gis_first_install_transition_receipt') != receipt):
        raise Blocked('gis_first_install_transition_source_invalid')
    validate_transition_receipt(receipt, source_state)
    try:
        namespace = kube.get('namespace', receipt.get('namespace', NS))
    except Exception:
        raise Blocked('gis_first_install_transition_namespace_unavailable') from None
    if namespace.get('metadata', {}).get('uid') != receipt['namespace_uid']:
        raise Blocked('gis_first_install_transition_namespace_changed')
    validate_receipt(receipt['resource_receipt'], kube, operation=receipt['operation'])
    return {'operation': receipt['operation'], 'receipt_sha256': receipt['receipt_sha256']}


def validate_transition_binding(binding, kube):
    """Recheck a previously admitted receipt during authorize and finish."""
    if (not isinstance(binding, dict) or set(binding) != {'operation', 'receipt_sha256', 'receipt'}
        or not isinstance(binding.get('receipt'), dict)
        or binding['operation'] != binding['receipt'].get('operation')
        or binding['receipt_sha256'] != binding['receipt'].get('receipt_sha256')):
        raise Blocked('gis_first_install_transition_binding_invalid')
    receipt = binding['receipt']
    payload = {key: value for key, value in receipt.items() if key != 'receipt_sha256'}
    if digest(payload) != binding['receipt_sha256']:
        raise Blocked('gis_first_install_transition_binding_invalid')
    try:
        namespace = kube.get('namespace', receipt.get('namespace', NS))
    except Exception:
        raise Blocked('gis_first_install_transition_namespace_unavailable') from None
    if namespace.get('metadata', {}).get('uid') != receipt.get('namespace_uid'):
        raise Blocked('gis_first_install_transition_namespace_changed')
    validate_receipt(receipt.get('resource_receipt'), kube, operation=binding['operation'])
    return True


def rollback(kube, plan, save):
    """Delete only exact unchanged objects journaled by this operation, Deployment first."""
    if plan.get('schema') != SCHEMA or plan.get('phase') not in ('PLANNED', 'CREATING', 'CREATED', 'STARTING', 'STARTED'):
        raise Blocked('gis_first_install_rollback_phase_invalid')
    if not plan.get('created'):
        if any(_read_pair(kube, plan).values()):
            raise Blocked('gis_first_install_rollback_unowned_object')
        plan['phase'] = 'ROLLED_BACK'
        save(copy.deepcopy(plan))
        return True
    for kind, name in (DEPLOYMENT, SERVICE):
        key = f'{kind}/{name}'; record = plan.get('created', {}).get(key)
        if record is None:
            continue
        try: live = _read_one(kube, kind, name)
        except Exception: raise Blocked('gis_first_install_rollback_read_failed') from None
        if live is None:
            plan['created'].pop(key); save(copy.deepcopy(plan)); continue
        intended = next(obj for obj in plan['objects'] if _identity(obj) == (kind, name))
        actual_semantic=semantic(live) if live is not None else None
        expected_semantic=copy.deepcopy(actual_semantic)
        if (kind, name)==DEPLOYMENT and expected_semantic is not None:
            expected_semantic['spec']['replicas']=0
        if (not _matches_owned(live, intended, plan['operation'],plan.get('service_allocation')=='server')
            or live['metadata']['uid'] != record['uid']
            or digest(expected_semantic) != digest(semantic(intended))
            or (plan.get('phase') not in ('CREATED','STARTING','STARTED')
                and str(live['metadata']['resourceVersion']) != record.get('resourceVersion'))
            or (plan.get('phase') not in ('STARTING','STARTED')
                and digest(actual_semantic) != record.get('spec_sha256'))
            or ((kind, name)==DEPLOYMENT and plan.get('phase') in ('STARTING','STARTED')
                and live.get('spec',{}).get('replicas') != 1)):
            raise Blocked('gis_first_install_rollback_object_changed')
        if str(live['metadata']['resourceVersion']) != record['resourceVersion']:
            # Only resourceVersion may drift across the standard apply. The
            # exact UID, operation marker, labels, annotations and spec above
            # must still match before updating the journal and deleting.
            record['resourceVersion']=str(live['metadata']['resourceVersion'])
        record['spec_sha256']=digest(semantic(live))
        save(copy.deepcopy(plan))
        try:
            _delete(kube, kind, name, record['uid'], record['resourceVersion'])
        except Exception:
            raise Blocked('gis_first_install_rollback_delete_conflict') from None
        plan['created'].pop(key)
        save(copy.deepcopy(plan))
    plan['phase'] = 'ROLLED_BACK'
    save(copy.deepcopy(plan))
