"""Root-owned, source-pinned GIS database prerequisite execution and proof."""
from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
import re
from pathlib import Path

import yaml

import gis_first_install as gis
Blocked = gis.Blocked

NS = 'voice-staging'
ALLOWED_NAMESPACES = (NS, 'voice-dev')
OP_LABEL = 'voice-rollout-operation'
UUID = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
SHA = re.compile(r'[a-f0-9]{64}')
OUTPUT = {
    'schema': 'voice-gis-verifier-v1', 'database': 'game_integration_db',
    'migration_version': 31, 'dirty': False, 'runtime_role': 'gameintegration_runtime',
    'role_restricted': True, 'table_grants': 'select_insert_update_delete',
    'sequence_grants': 'usage_select', 'ddl_denied': True,
    'schema_ledger_runtime_mutation_denied': True,
}
INPUTS_KEYS = {'schema', 'database', 'configmaps', 'migrations', 'migration_inputs_sha256',
               'provision_sql_sha256', 'verifier_sql_sha256', 'templates', 'images',
               'job_images', 'jobs', 'secret_refs'}
JOB_ROLES = ('provision', 'migrate', 'verify')
VERIFIER_V1_SHA256 = '4297373f6d91364da34f46f959d5df8ea07f71553ebaf52d048f48782f5da11c'
VERIFIER_V2_SHA256 = '9924cfc09d6c08c22e0d4b0b412f32cf212476ebfc4806b9fccc2c699f4dfd5e'
VERIFIER_V3_SHA256 = 'bb230fc585ff4d8264e0edc7ec2a395e59c94794a437c13696939206676a8225'


def _canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True)


def _sha(value):
    return hashlib.sha256(value).hexdigest()


def validate_source_revision(inputs):
    """Refuse a verifier and migration set that cannot describe the same revision."""
    try:
        rows=inputs['migrations']
        versions=[int(row['name'].split('_',1)[0]) for row in rows if row['name'].endswith('.up.sql')]
        if not versions or len(set(versions)) != len(versions):
            raise ValueError()
        revision = max(versions)
        if (revision, inputs['verifier_sql_sha256']) not in {
                (31, VERIFIER_V1_SHA256), (32, VERIFIER_V2_SHA256), (33, VERIFIER_V3_SHA256)}:
            raise ValueError()
        return revision
    except (KeyError,TypeError,ValueError,AttributeError):
        raise Blocked('gis_prerequisite_source_revision_unsupported') from None


def _load_producer(root):
    path = Path(root) / 'scripts/dev/migrations.py'
    if not path.is_file():
        raise Blocked('gis_prerequisite_source_missing')
    spec = importlib.util.spec_from_file_location('_voice_gis_migration_source', path)
    if spec is None or spec.loader is None:
        raise Blocked('gis_prerequisite_source_invalid')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def source_inputs(root, code_capture):
    """Recompute the source owner’s metadata from the immutable root capture."""
    root = Path(root)
    required_paths = {
        'scripts/dev/migrations.py',
        'docker/postgres/provision-gameintegration-runtime-role.sql',
        'docker/postgres/verify-gameintegration-runtime-role.sql',
        'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
        'deploy/templates/migrate-game-integration-db-job.yaml',
        'deploy/templates/verify-game-integration-db-job.yaml',
    }
    if not isinstance(code_capture, dict) or not required_paths.issubset(code_capture):
        raise Blocked('gis_prerequisite_capture_incomplete')
    for relative in required_paths:
        path = root / relative
        if not path.is_file() or code_capture[relative] != _sha(path.read_bytes()):
            raise Blocked('gis_prerequisite_capture_changed')
    try:
        producer = _load_producer(root)
        inputs = producer.gameintegration_prerequisite_inputs(root)
    except Exception:
        raise Blocked('gis_prerequisite_source_contract_invalid') from None
    if not isinstance(inputs, dict) or set(inputs) != INPUTS_KEYS:
        raise Blocked('gis_prerequisite_source_contract_invalid')
    for row in inputs.get('migrations', []):
        if not isinstance(row, dict) or not isinstance(row.get('name'), str):
            raise Blocked('gis_prerequisite_source_contract_invalid')
        relative = 'src/backend/migrations/game_integration_db/' + row.get('name', '')
        path = root / relative
        if (relative not in code_capture or not path.is_file()
            or code_capture[relative] != _sha(path.read_bytes())):
            raise Blocked('gis_prerequisite_capture_incomplete')
    if (inputs['schema'] != 'voice-gis-prerequisite-v1'
        or inputs['database'] != 'game_integration_db'
        or inputs['configmaps'] != {'provision': 'voice-gameintegration-runtime-role-sql',
            'migrations': 'voice-gameintegration-db-migrations',
            'verify': 'voice-gameintegration-db-verifier-sql'}
        or inputs['jobs'] != {'provision': 'voice-provision-gameintegration-runtime-role',
            'migrate': 'voice-migrate-gameintegration-db',
            'verify': 'voice-verify-gameintegration-db'}
        or not isinstance(inputs['migrations'], list) or not inputs['migrations']
        or len(inputs['migrations']) > 512
        or inputs['migration_inputs_sha256'] != _sha(_canonical(inputs['migrations']).encode())
        or any(not isinstance(row, dict) or set(row) != {'name', 'sha256'}
               or not re.fullmatch(r'[0-9]{1,20}_[a-zA-Z0-9_-]{1,100}\.(up|down)\.sql', row['name'])
               or not isinstance(row['sha256'], str) or not SHA.fullmatch(row['sha256'])
               for row in inputs['migrations'])
        or inputs['migrations'] != sorted(inputs['migrations'], key=lambda row: row['name'])
        or any(not isinstance(inputs[key], str) or not SHA.fullmatch(inputs[key])
               for key in ('provision_sql_sha256', 'verifier_sql_sha256'))):
        raise Blocked('gis_prerequisite_source_contract_invalid')
    validate_source_revision(inputs)
    total = 0
    for row in inputs['migrations']:
        raw = (root / 'src/backend/migrations/game_integration_db' / row['name']).read_bytes()
        total += len(raw)
        if not 1 <= len(raw) <= 262144 or total > 1048576:
            raise Blocked('gis_prerequisite_source_size_invalid')
    for role in JOB_ROLES:
        if not isinstance(inputs['templates'].get(role), str) or not SHA.fullmatch(inputs['templates'][role]):
            raise Blocked('gis_prerequisite_template_binding_invalid')
    if inputs['templates'] != {role: _sha((root / path).read_bytes()) for role, path in {
        'provision': 'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
        'migrate': 'deploy/templates/migrate-game-integration-db-job.yaml',
        'verify': 'deploy/templates/verify-game-integration-db-job.yaml'}.items()}:
        raise Blocked('gis_prerequisite_template_binding_invalid')
    if (not isinstance(inputs['images'], dict) or set(inputs['images']) != {'postgres', 'migrate'}
        or any(not isinstance(image, str) or '@sha256:' not in image for image in inputs['images'].values())
        or not isinstance(inputs['job_images'], dict) or set(inputs['job_images']) != set(JOB_ROLES)
        or inputs['secret_refs'] != {
            'provision_admin': {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'},
            'runtime_password': {'name': 'voice-app-secrets', 'key': 'GAME_INTEGRATION_DB_PASSWORD'},
            'migration_admin': {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'},
            'verification_admin': {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'},
            'runtime_database_url': {'name': 'voice-app-secrets', 'key': 'GAME_INTEGRATION_DATABASE_URL'}}):
        raise Blocked('gis_prerequisite_source_contract_invalid')
    return copy.deepcopy(inputs)


def _documents(root, plan, code_capture):
    root = Path(root)
    namespace = plan.get('namespace', NS)
    if namespace not in ALLOWED_NAMESPACES:
        raise Blocked('gis_prerequisite_namespace_invalid')
    inputs = plan.get('inputs')
    if not isinstance(inputs, dict) or set(inputs) != INPUTS_KEYS or gis.digest(inputs) != plan.get('inputs_sha256'):
        raise Blocked('gis_prerequisite_input_binding_invalid')
    # Recompute from the same root-owned capture on every resume, not from a
    # caller-provided bundle or stored success claim.
    producer = _load_producer(root)
    if source_inputs(root, code_capture) != inputs:
        raise Blocked('gis_prerequisite_source_changed')
    migration_dir = root / 'src/backend/migrations/game_integration_db'
    migration_data = {row['name']: (migration_dir / row['name']).read_text(encoding='utf-8')
                      for row in inputs['migrations']}
    provision_sql = (root / 'docker/postgres/provision-gameintegration-runtime-role.sql').read_text(encoding='utf-8')
    if (_sha(provision_sql.encode()) != inputs['provision_sql_sha256']
        or len(migration_data) != len(inputs['migrations'])
        or any(_sha(migration_data[row['name']].encode()) != row['sha256'] for row in inputs['migrations'])):
        raise Blocked('gis_prerequisite_sql_binding_invalid')
    configmaps = {
        'provision': {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
            'metadata': {'name': inputs['configmaps']['provision'], 'namespace': namespace},
            'data': {'provision.sql': provision_sql}},
        'migrate': {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
            'metadata': {'name': inputs['configmaps']['migrations'], 'namespace': namespace},
            'data': migration_data},
        'verify': producer.gameintegration_verifier_configmap(root, namespace),
    }
    if (_sha(configmaps['verify']['data']['verify.sql'].encode()) != inputs['verifier_sql_sha256']
        or configmaps['verify']['metadata']['name'] != inputs['configmaps']['verify']):
        raise Blocked('gis_prerequisite_verifier_binding_invalid')
    template_paths = {
        'provision': 'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
        'migrate': 'deploy/templates/migrate-game-integration-db-job.yaml',
        'verify': 'deploy/templates/verify-game-integration-db-job.yaml',
    }
    jobs = {}
    for role, relative in template_paths.items():
        raw = (root / relative).read_text(encoding='utf-8')
        if raw.count('__K_NAMESPACE__') != 1:
            raise Blocked('gis_prerequisite_template_invalid')
        obj = yaml.safe_load(raw.replace('__K_NAMESPACE__', namespace))
        if (not isinstance(obj, dict) or obj.get('kind') != 'Job'
            or obj.get('metadata', {}).get('name') != inputs['jobs'][role]
            or obj['metadata'].get('namespace') != namespace):
            raise Blocked('gis_prerequisite_template_invalid')
        jobs[role] = obj
    # Bind immutable operation ownership labels before calculating the actual
    # created API specs. These labels do not alter source-controlled templates.
    for role, obj in jobs.items():
        obj['metadata'].setdefault('labels', {})[OP_LABEL] = plan['operation']
        obj['spec']['template']['metadata'].setdefault('labels', {})[OP_LABEL] = plan['operation']
    for role, obj in configmaps.items():
        obj['metadata'].setdefault('labels', {})[OP_LABEL] = plan['operation']
    for role in JOB_ROLES:
        expected = inputs['job_images'].get(role)
        spec = jobs[role]['spec']['template']['spec']
        actual = {container['name']: container['image'] for group in ('initContainers', 'containers')
                  for container in spec.get(group, [])}
        if not isinstance(expected, dict) or set(expected) != set(actual):
            raise Blocked('gis_prerequisite_image_contract_invalid')
        if any(actual[name] != inputs['images'][logical] for name, logical in expected.items()):
            raise Blocked('gis_prerequisite_image_contract_invalid')
        names = {item.get('configMap', {}).get('name') for item in spec.get('volumes', [])
                 if item.get('configMap', {}).get('name')}
        wanted_cm = inputs['configmaps']['migrations' if role == 'migrate' else role]
        if wanted_cm not in names:
            raise Blocked('gis_prerequisite_template_invalid')
    secret_sets = {
        'provision': {('voice-app-secrets', 'POSTGRES_PASSWORD'),
                      ('voice-app-secrets', 'GAME_INTEGRATION_DB_PASSWORD')},
        'migrate': {('voice-app-secrets', 'POSTGRES_PASSWORD')},
        'verify': {('voice-app-secrets', 'POSTGRES_PASSWORD')},
    }
    for role, job in jobs.items():
        refs = set()
        for container in job['spec']['template']['spec'].get('initContainers', []) + job['spec']['template']['spec'].get('containers', []):
            for env in container.get('env', []):
                reference = env.get('valueFrom', {}).get('secretKeyRef')
                if reference is not None:
                    refs.add((reference.get('name'), reference.get('key')))
        if refs != secret_sets[role]:
            raise Blocked('gis_prerequisite_secret_contract_invalid')
    return configmaps, jobs


def _endpoint_identity(kube, namespace_name=NS):
    if namespace_name not in ALLOWED_NAMESPACES:
        raise Blocked('gis_prerequisite_namespace_invalid')
    namespace = kube.get('namespace', namespace_name)
    service = kube.get('service', 'voice-postgres')
    service_spec = service.get('spec', {})
    if (namespace.get('metadata', {}).get('uid') is None or service.get('metadata', {}).get('uid') is None
        or service.get('metadata', {}).get('namespace') != namespace_name
        or service_spec.get('type', 'ClusterIP') != 'ClusterIP'
        or not any(port.get('port') == 5432 for port in service_spec.get('ports', []))
        or not isinstance(service_spec.get('selector'), dict) or not service_spec['selector']):
        raise Blocked('gis_prerequisite_database_identity_invalid')
    response = kube.run(['get', 'endpointslices', '-l', 'kubernetes.io/service-name=voice-postgres', '-o', 'json'])
    rows = response.get('items') if isinstance(response, dict) else None
    if not isinstance(rows, list) or not rows or len(rows) > 64:
        raise Blocked('gis_prerequisite_database_endpoints_invalid')
    result = []
    for row in rows:
        uid = row.get('metadata', {}).get('uid')
        if not isinstance(uid, str) or not UUID.fullmatch(uid):
            raise Blocked('gis_prerequisite_database_endpoints_invalid')
        ready = []
        for endpoint in row.get('endpoints', []):
            conditions = endpoint.get('conditions', {})
            target = endpoint.get('targetRef', {})
            if conditions.get('ready') is True:
                if target.get('kind') != 'Pod' or not isinstance(target.get('uid'), str) or not UUID.fullmatch(target['uid']):
                    raise Blocked('gis_prerequisite_database_endpoints_invalid')
                pod = kube.get('pod', target.get('name'))
                if (pod.get('metadata', {}).get('uid') != target['uid']
                    or pod.get('metadata', {}).get('namespace') != namespace_name
                    or pod.get('status', {}).get('phase') != 'Running'
                    or any(pod.get('metadata', {}).get('labels', {}).get(key) != value
                           for key, value in service_spec['selector'].items())
                    or not any(condition.get('type') == 'Ready' and condition.get('status') == 'True'
                               for condition in pod.get('status', {}).get('conditions', []))):
                    raise Blocked('gis_prerequisite_database_endpoints_invalid')
                ready.append(target['uid'])
        if ready:
            result.append({'uid': uid, 'pod_uids': sorted(ready)})
    if not result:
        raise Blocked('gis_prerequisite_database_endpoints_invalid')
    return {'namespace_uid': namespace['metadata']['uid'], 'service_uid': service['metadata']['uid'],
            'service_spec_sha256': gis.digest({'spec': service_spec}),
            'endpoints': sorted(result, key=lambda row: row['uid'])}


def preflight(kube, plan):
    if (not isinstance(plan, dict) or plan.get('phase') not in ('PLANNED', 'CREATING', 'STARTING')
        or not isinstance(plan.get('inputs_sha256'), str) or not SHA.fullmatch(plan['inputs_sha256'])):
        raise Blocked('gis_prerequisite_plan_invalid')
    namespace = plan.get('namespace', NS)
    if namespace not in ALLOWED_NAMESPACES:
        raise Blocked('gis_prerequisite_namespace_invalid')
    live = _endpoint_identity(kube, namespace)
    if plan.get('database_identity') is not None and plan['database_identity'] != live:
        raise Blocked('gis_prerequisite_database_identity_changed')
    secret = kube.secret_meta('voice-app-secrets')
    if plan.get('secret_metadata') is not None and plan['secret_metadata'] != secret:
        raise Blocked('gis_prerequisite_secret_changed')
    config = kube.get('configmap', 'voice-app-config')
    config_meta = {key: config.get('metadata', {}).get(key) for key in ('uid', 'resourceVersion')}
    if not all(isinstance(value, str) and value for value in config_meta.values()):
        raise Blocked('gis_prerequisite_config_metadata_invalid')
    if plan.get('config_metadata') is not None and plan['config_metadata'] != config_meta:
        raise Blocked('gis_prerequisite_config_changed')
    username = config.get('data', {}).get('POSTGRES_USER')
    if not isinstance(username, str) or not re.fullmatch(r'[a-z_][a-z0-9_]{0,31}', username):
        raise Blocked('gis_prerequisite_config_invalid')
    secret_keys = {'POSTGRES_PASSWORD', 'GAME_INTEGRATION_DB_PASSWORD', 'GAME_INTEGRATION_DATABASE_URL'}
    for key in sorted(secret_keys):
        present = kube.run(['get', 'secret', 'voice-app-secrets', '-o',
            'go-template={{if index .data "' + key + '"}}true{{else}}false{{end}}'])
        if present is not True:
            raise Blocked('gis_prerequisite_secret_key_missing')
    plan['database_identity'] = live
    plan['secret_metadata'] = secret
    plan['config_metadata'] = config_meta
    return copy.deepcopy(live)


def _spec_digest(obj):
    spec = copy.deepcopy(obj.get('spec'))
    if obj.get('kind') == 'Job' and isinstance(spec, dict):
        # Remove only Kubernetes-generated defaults/selector fields. All
        # explicit workload fields remain part of the digest.
        for key in ('selector', 'manualSelector', 'completions', 'parallelism'):
            spec.pop(key, None)
        template = spec.get('template', {})
        metadata = template.get('metadata', {})
        labels = metadata.get('labels', {})
        for key in ('controller-uid', 'job-name', 'batch.kubernetes.io/controller-uid',
                    'batch.kubernetes.io/job-name'):
            labels.pop(key, None)
        pod = template.get('spec', {})
        defaults = {'dnsPolicy': 'ClusterFirst', 'schedulerName': 'default-scheduler',
                    'restartPolicy': 'Never', 'terminationGracePeriodSeconds': 30,
                    'serviceAccountName': 'default', 'enableServiceLinks': True,
                    'preemptionPolicy': 'PreemptLowerPriority'}
        for key, value in defaults.items():
            if pod.get(key) == value:
                pod.pop(key, None)
        for group in ('initContainers', 'containers'):
            for container in pod.get(group, []):
                cdefaults = {'terminationMessagePath': '/dev/termination-log',
                             'terminationMessagePolicy': 'File',
                             'imagePullPolicy': ('Always' if ':latest' in container.get('image', '')
                                                 or ':' not in container.get('image', '').split('/')[-1]
                                                 else 'IfNotPresent')}
                for key, value in cdefaults.items():
                    if container.get(key) == value:
                        container.pop(key, None)
    return _sha(_canonical({'apiVersion': obj['apiVersion'], 'kind': obj['kind'],
        'metadata': {'name': obj['metadata']['name'], 'namespace': obj['metadata']['namespace'],
                     'labels': obj['metadata'].get('labels', {}),
                     'annotations': obj['metadata'].get('annotations', {})},
        'spec': spec, 'immutable': obj.get('immutable'), 'data': obj.get('data')}).encode())


def _read_named(kube, kind, name):
    response = kube.run(['get', kind + 's', '-o', 'json'])
    items = response.get('items') if isinstance(response, dict) else None
    if not isinstance(items, list) or len(items) > 2048:
        raise Blocked('gis_prerequisite_existing_read_failed')
    matches = [row for row in items if row.get('metadata', {}).get('name') == name]
    if len(matches) > 1:
        raise Blocked('gis_prerequisite_existing_read_failed')
    return matches[0] if matches else None


def _job_live_matches(actual, wanted, uid):
    actual_meta = actual.get('metadata', {})
    wanted_meta = wanted.get('metadata', {})
    if (actual.get('kind') != 'Job' or wanted.get('kind') != 'Job'
        or actual_meta.get('name') != wanted_meta.get('name')
        or actual_meta.get('namespace') != wanted_meta.get('namespace')
        or actual_meta.get('uid') != uid
        or actual_meta.get('labels', {}).get(OP_LABEL) != wanted_meta.get('labels', {}).get(OP_LABEL)):
        return False
    spec = copy.deepcopy(actual.get('spec'))
    wanted_spec = copy.deepcopy(wanted.get('spec'))
    if not isinstance(spec, dict) or not isinstance(wanted_spec, dict):
        return False
    labels = wanted['spec']['template']['metadata'].get('labels', {})
    generated = {'batch.kubernetes.io/controller-uid': uid,
                 'batch.kubernetes.io/job-name': wanted['metadata']['name']}
    legacy = {'controller-uid': uid, 'job-name': wanted['metadata']['name']}
    selector_object = spec.get('selector')
    if ('selector' in wanted_spec or not isinstance(selector_object, dict)
        or set(selector_object) != {'matchLabels'}):
        return False
    selector = selector_object.get('matchLabels')
    if not isinstance(selector, dict) or selector != {'batch.kubernetes.io/controller-uid': uid}:
        return False
    actual_labels = spec.get('template', {}).get('metadata', {}).get('labels', {})
    if actual_labels not in ({**labels, **generated}, {**labels, **generated, **legacy}):
        return False
    spec.pop('selector', None)
    wanted_spec.pop('selector', None)
    for key, value in {'completionMode': 'NonIndexed', 'completions': 1,
            'manualSelector': False, 'parallelism': 1,
            'podReplacementPolicy': 'TerminatingOrFailed', 'suspend': False}.items():
        if wanted_spec.get(key) is None:
            if spec.get(key) != value:
                return False
            spec.pop(key, None)
    template = spec.get('template', {})
    wanted_template = wanted_spec.get('template', {})
    template_metadata = template.get('metadata', {})
    wanted_metadata = wanted_template.get('metadata', {})
    template_metadata['labels'] = copy.deepcopy(labels)
    wanted_metadata['labels'] = copy.deepcopy(labels)
    pod = template.get('spec', {})
    wanted_pod = wanted_template.get('spec', {})
    for key, value in {'dnsPolicy': 'ClusterFirst', 'schedulerName': 'default-scheduler',
            'terminationGracePeriodSeconds': 30}.items():
        if wanted_pod.get(key) is None:
            if pod.get(key) != value:
                return False
            pod.pop(key, None)
    if wanted_pod.get('securityContext') is None:
        if pod.get('securityContext') != {}:
            return False
        pod.pop('securityContext', None)
    for group in ('containers', 'initContainers'):
        actual_rows = pod.get(group, [])
        wanted_rows = wanted_pod.get(group, [])
        if not isinstance(actual_rows, list) or len(actual_rows) != len(wanted_rows):
            return False
        for actual_row, wanted_row in zip(actual_rows, wanted_rows):
            defaults = {'imagePullPolicy': 'IfNotPresent',
                'terminationMessagePath': '/dev/termination-log',
                'terminationMessagePolicy': 'File'}
            for key, value in defaults.items():
                if wanted_row.get(key) is None:
                    if actual_row.get(key) != value:
                        return False
                    actual_row.pop(key, None)
    actual_volumes = pod.get('volumes', [])
    wanted_volumes = wanted_pod.get('volumes', [])
    if not isinstance(actual_volumes, list) or len(actual_volumes) != len(wanted_volumes):
        return False
    for actual_volume, wanted_volume in zip(actual_volumes, wanted_volumes):
        actual_cm = actual_volume.get('configMap')
        wanted_cm = wanted_volume.get('configMap')
        if isinstance(actual_cm, dict) and isinstance(wanted_cm, dict) and wanted_cm.get('defaultMode') is None:
            if actual_cm.get('defaultMode') != 420:
                return False
            actual_cm.pop('defaultMode', None)
    return _canonical(spec) == _canonical(wanted_spec)


def _create_once(kube, kind, obj, plan, saved, save):
    name = obj['metadata']['name']
    key = kind + '/' + name
    wanted_sha = _spec_digest(obj)
    record = saved.get(key)
    existing = _read_named(kube, kind.lower(), name)
    if existing is not None:
        if record is None:
            metadata = existing.get('metadata', {})
            uid = metadata.get('uid')
            # A create response can be lost after the API committed the exact
            # operation-owned object. Adopt it only while this same operation
            # is durably in CREATING and every identity/spec guard matches.
            if (plan.get('prerequisite_phase') != 'CREATING'
                or metadata.get('namespace') != plan.get('namespace', NS)
                or metadata.get('name') != name
                or not isinstance(uid, str) or not UUID.fullmatch(uid)
                or metadata.get('labels', {}).get(OP_LABEL) != plan.get('operation')
                or (kind == 'job' and not _job_live_matches(existing, obj, uid))
                or (kind != 'job' and _spec_digest(existing) != wanted_sha)):
                raise Blocked('gis_prerequisite_foreign_resource_present')
            record = {'uid': uid, 'spec_sha256': wanted_sha}
            saved[key] = record
            save(copy.deepcopy(plan))
        if (existing.get('metadata', {}).get('uid') != record.get('uid')
            or existing.get('metadata', {}).get('labels', {}).get(OP_LABEL) != plan['operation']
            or (kind == 'job' and not _job_live_matches(existing, obj, record.get('uid')))
            or (kind != 'job' and _spec_digest(existing) != record.get('spec_sha256'))):
            raise Blocked('gis_prerequisite_resource_changed')
        return existing, record
    if record is not None:
        raise Blocked('gis_prerequisite_recorded_resource_missing')
    verify_storage = getattr(kube, 'verify_final_storage', None)
    if verify_storage is not None:
        verify_storage()
    try:
        # The comma selector is rejected by kubectl's JSONPath parser before it
        # can return a server receipt. Parse the full JSON object privately and
        # retain only its validated identity below.
        created = kube.run(['create', '-f', '/dev/stdin', '-o', 'json'], body=obj)
    except Exception as error:
        returncode = getattr(error, 'returncode', None)
        category = getattr(error, 'category', 'transport_error')
        if type(returncode) is not int or returncode < 0:
            returncode = None
        if category not in {'invalid_output_format', 'forbidden', 'unauthorized',
                            'not_found', 'conflict', 'timeout', 'api_error', 'transport_error'}:
            category = 'transport_error'
        plan.setdefault('create_failures', {})[key] = {
            'returncode': returncode, 'category': category}
        try:
            observed = _read_named(kube, kind.lower(), name)
        except Exception:
            raise Blocked('gis_prerequisite_create_outcome_unknown') from None
        if observed is None:
            raise Blocked('gis_prerequisite_create_outcome_unknown') from None
        metadata = observed.get('metadata', {})
        uid = metadata.get('uid')
        if (metadata.get('namespace') != plan.get('namespace', NS)
            or metadata.get('name') != name
            or not isinstance(uid, str) or not UUID.fullmatch(uid)
            or metadata.get('labels', {}).get(OP_LABEL) != plan.get('operation')
            or (kind == 'job' and not _job_live_matches(observed, obj, uid))
            or (kind != 'job' and _spec_digest(observed) != wanted_sha)):
            raise Blocked('gis_prerequisite_create_outcome_unknown') from None
        created = observed
        existing = observed
    metadata = created.get('metadata', {}) if isinstance(created, dict) else {}
    uid = metadata.get('uid')
    if (not isinstance(created, dict) or created.get('kind') != obj.get('kind')
        or metadata.get('name') != name
        or metadata.get('namespace') != plan.get('namespace', NS)
        or metadata.get('labels', {}).get(OP_LABEL) != plan.get('operation')
        or not isinstance(uid, str) or not UUID.fullmatch(uid)):
        raise Blocked('gis_prerequisite_create_receipt_invalid')
    record = {'uid': uid, 'spec_sha256': wanted_sha}
    saved[key] = record
    save(copy.deepcopy(plan))
    fresh = existing if existing is not None else _read_named(kube, kind.lower(), name)
    if (fresh.get('metadata', {}).get('uid') != record['uid']
        or fresh.get('metadata', {}).get('labels', {}).get(OP_LABEL) != plan['operation']
        or (kind == 'job' and not _job_live_matches(fresh, obj, record['uid']))
        or (kind != 'job' and _spec_digest(fresh) != wanted_sha)):
        raise Blocked('gis_prerequisite_created_resource_changed')
    return fresh, record


def reconcile_existing_jobs(kube, plan, source_root, code_capture, save):
    """Adopt only exact operation-owned Jobs after an interrupted create."""
    if plan.get('prerequisite_phase') not in ('CREATING', 'VERIFIED'):
        return False
    _, wanted_jobs = _documents(source_root, plan, code_capture)
    created = plan.setdefault('created', {})
    observed_any = False
    for role in JOB_ROLES:
        wanted = wanted_jobs[role]
        name = wanted['metadata']['name']
        key = 'job/' + name
        live = _read_named(kube, 'job', name)
        record = created.get(key) or plan.setdefault('jobs', {}).get(role)
        if live is None:
            if record is not None:
                raise Blocked('gis_prerequisite_recorded_resource_missing')
            continue
        observed_any = True
        metadata = live.get('metadata', {})
        uid = metadata.get('uid')
        if (metadata.get('namespace') != plan.get('namespace', NS)
            or metadata.get('name') != name or not isinstance(uid, str) or not UUID.fullmatch(uid)
            or metadata.get('labels', {}).get(OP_LABEL) != plan.get('operation')
            or not _job_live_matches(live, wanted, uid)):
            raise Blocked('gis_prerequisite_foreign_resource_present')
        if record is None:
            record = {'uid': uid, 'spec_sha256': _spec_digest(wanted)}
            created[key] = record
            save(copy.deepcopy(plan))
        elif (record.get('uid') != uid
            or record.get('spec_sha256') != _spec_digest(wanted)):
            raise Blocked('gis_prerequisite_resource_changed')
        proof, _ = _verify_job(kube, role, wanted, record, plan['inputs'], plan)
        plan.setdefault('jobs', {})[role] = proof
        save(copy.deepcopy(plan))
    if plan.get('prerequisite_phase') == 'VERIFIED' and not observed_any:
        raise Blocked('gis_prerequisite_recorded_resource_missing')
    return observed_any


def _verify_job(kube, role, obj, record, inputs, plan):
    name = obj['metadata']['name']
    try:
        waited = kube.run(['wait', '--for=condition=complete', 'job/' + name, '--timeout=600s', '-o', 'json'], timeout=610)
        live = kube.get('job', name)
    except Exception:
        raise Blocked('gis_prerequisite_job_wait_failed') from None
    conditions = live.get('status', {}).get('conditions', [])
    waited_metadata = waited.get('metadata', {}) if isinstance(waited, dict) else {}
    if (not isinstance(waited, dict) or waited.get('kind') != 'Job'
        or waited_metadata.get('name') != name
        or waited_metadata.get('namespace') != plan.get('namespace', NS)
        or waited_metadata.get('uid') != record['uid']
        or live.get('metadata', {}).get('uid') != record['uid']
        or live.get('metadata', {}).get('labels', {}).get(OP_LABEL) != plan['operation']
        or not _job_live_matches(live, obj, record['uid'])
        or live.get('status', {}).get('succeeded') != 1
        or live.get('status', {}).get('failed', 0) != 0
        or not any(row.get('type') == 'Complete' and row.get('status') == 'True' for row in conditions)):
        raise Blocked('gis_prerequisite_job_not_complete')
    pods = kube.run(['get', 'pods', '-l', 'job-name=' + name, '-o', 'json']).get('items')
    if not isinstance(pods, list) or len(pods) != 1:
        raise Blocked('gis_prerequisite_job_pod_invalid')
    pod = pods[0]
    owners = pod.get('metadata', {}).get('ownerReferences', [])
    if (pod.get('status', {}).get('phase') != 'Succeeded'
        or not any(owner.get('kind') == 'Job' and owner.get('uid') == record['uid'] and owner.get('controller') is True for owner in owners)):
        raise Blocked('gis_prerequisite_job_pod_invalid')
    statuses = pod.get('status', {}).get('initContainerStatuses', []) + pod.get('status', {}).get('containerStatuses', [])
    expected = inputs['job_images'][role]
    expected_names = set(expected)
    if {row.get('name') for row in statuses} != expected_names:
        raise Blocked('gis_prerequisite_job_image_invalid')
    image_ids = {}
    containers = {row['name']: row for group in ('initContainers', 'containers')
                  for row in obj['spec']['template']['spec'].get(group, [])}
    for status in statuses:
        wanted = inputs['images'][expected[status['name']]]
        image_id = status.get('imageID', '')
        if (not isinstance(image_id, str) or not image_id.endswith(wanted.split('@', 1)[1])
            or containers[status['name']]['image'] != wanted
            or status.get('state', {}).get('terminated', {}).get('exitCode') != 0):
            raise Blocked('gis_prerequisite_job_image_invalid')
        image_ids[status['name']] = image_id
    pod_uid = pod.get('metadata', {}).get('uid')
    if not isinstance(pod_uid, str) or not UUID.fullmatch(pod_uid):
        raise Blocked('gis_prerequisite_job_pod_invalid')
    result = None
    if role == 'verify':
        try:
            raw = kube.run(['logs', 'job/' + name, '-c', 'verify'])
            if not isinstance(raw, str) or not 1 <= len(raw) <= 4096 or not raw.endswith('\n'):
                raise ValueError()
            lines = raw.splitlines()
            if len(lines) != 1:
                raise ValueError()
            result = json.loads(lines[0])
        except Exception:
            raise Blocked('gis_prerequisite_verifier_output_invalid') from None
        if result != gis.verifier_output(validate_source_revision(inputs)):
            raise Blocked('gis_prerequisite_verifier_failed')
    completion_time = live.get('status', {}).get('completionTime')
    if not isinstance(completion_time, str) or not completion_time:
        raise Blocked('gis_prerequisite_job_not_complete')
    return {'name': name, 'uid': record['uid'], 'spec_sha256': record['spec_sha256'],
            'pod_uid': pod_uid, 'image_ids': image_ids, 'complete': True,
            'succeeded': 1, 'completion_time': completion_time}, result


def execute(kube, plan, source_root, code_capture, save, *, verify_storage=None):
    """Create provision→migration→verification inputs/jobs, then journal proof."""
    configmaps, jobs = _documents(source_root, plan, code_capture)
    current = preflight(kube, plan)
    if plan.get('phase') not in ('PLANNED', 'CREATING', 'STARTING'):
        raise Blocked('gis_prerequisite_phase_invalid')
    saved = plan.setdefault('created', {})
    completions = plan.setdefault('jobs', {})
    plan['prerequisite_phase'] = 'CREATING'
    save(copy.deepcopy(plan))
    job_records = {}
    verifier_result = None
    for role in JOB_ROLES:
        if verify_storage is not None:
            verify_storage()
        cm, cm_record = _create_once(kube, 'configmap', configmaps[role], plan, saved, save)
        if verify_storage is not None:
            verify_storage()
        job, job_record = _create_once(kube, 'job', jobs[role], plan, saved, save)
        # If a previous root process completed the job but died before the
        # completion record, accept only its original UID and source-bound spec.
        observed, verifier_result = _verify_job(kube, role, jobs[role], job_record,
                                                plan['inputs'], plan)
        if verify_storage is not None:
            verify_storage()
        if preflight(kube, plan) != current:
            raise Blocked('gis_prerequisite_live_inputs_changed')
        job_records[role] = observed
        completions[role] = observed
        save(copy.deepcopy(plan))
    if source_inputs(source_root, code_capture) != plan['inputs']:
        raise Blocked('gis_prerequisite_source_changed')
    after = preflight(kube, plan)
    if after != current:
        raise Blocked('gis_prerequisite_live_inputs_changed')
    revision = validate_source_revision(plan['inputs'])
    if verifier_result != gis.verifier_output(revision):
        raise Blocked('gis_prerequisite_verifier_failed')
    dev = plan.get('namespace', gis.NS) == 'voice-dev'
    receipt = {
        'schema': ({31: gis.DEV_PREREQUISITE_SCHEMA, 32: gis.DEV_PREREQUISITE_SCHEMA_V2, 33: gis.DEV_PREREQUISITE_SCHEMA_V3} if dev else
                   {31: gis.PREREQUISITE_SCHEMA, 32: gis.PREREQUISITE_SCHEMA_V2, 33: gis.PREREQUISITE_SCHEMA_V3})[revision],
        'operation': plan['operation'],
        'source_sha': plan['source_sha'], 'source_plan_sha256': plan['source_plan_sha256'],
        'inputs_sha256': plan['inputs_sha256'], 'inputs': copy.deepcopy(plan['inputs']),
        'namespace_uid': current['namespace_uid'],
        'database': 'game_integration_db', 'database_identity': copy.deepcopy(after),
        'configmaps': {('migrations' if role == 'migrate' else role): {
            'name': plan['inputs']['configmaps']['migrations' if role == 'migrate' else role],
            'uid': saved['configmap/' + plan['inputs']['configmaps']['migrations' if role == 'migrate' else role]]['uid'],
            'spec_sha256': saved['configmap/' + plan['inputs']['configmaps']['migrations' if role == 'migrate' else role]]['spec_sha256']}
            for role in JOB_ROLES},
        'jobs': job_records, 'verifier': verifier_result,
        'revalidation': {'namespace_uid': after['namespace_uid'],
            'service_uid': after['service_uid'],
            'service_spec_sha256': after['service_spec_sha256'],
            'endpoints': copy.deepcopy(after['endpoints']),
            'secret_metadata': {'name': 'voice-app-secrets', **plan['secret_metadata']},
            'config_metadata': copy.deepcopy(plan['config_metadata'])},
    }
    if dev:
        build = copy.deepcopy(plan['source_build_receipt'])
        receipt['source_build_receipt'] = build
        receipt['source_build_receipt_sha256'] = gis.digest(build)
    else:
        # Preserve the established STAGING evidence contract exactly.
        receipt['source_ci'] = copy.deepcopy(plan['source_ci'])
        receipt['custody_binding_sha256'] = plan['custody_binding_sha256']
    gis.validate_prerequisite_receipt(receipt, plan)
    plan['prerequisite_receipt'] = receipt
    plan['prerequisite_phase'] = 'VERIFIED'
    save(copy.deepcopy(plan))
    return copy.deepcopy(receipt)
