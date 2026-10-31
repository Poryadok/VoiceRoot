"""Guarded read-only successor for an already anchored Realtime bootstrap."""
import base64
import copy
import hashlib
import importlib
import json
import os
from pathlib import Path
import re
import stat

from controller import Blocked

SOURCE_HELPERS = {
    'realtime_bootstrap': 'nats-rollout-preservation/realtime_bootstrap.py',
    'realtime_bootstrap_existing': 'nats-rollout-preservation/realtime_bootstrap_existing.py',
    'resource_claim': 'nats-rollout-preservation/resource_claim.py',
    'guard': 'nats-rollout-preservation/guard.py',
    'gis_existing_update': 'nats-rollout-preservation/gis_existing_update.py',
    'operator_retirement': 'nats-rollout-preservation/operator_retirement.py',
    'runtime_stage': 'nats-rollout-preservation/runtime_stage.py',
    'bridge_root': 'nats-rollout-preservation/bridge_root.py',
    'source_authority': 'nats-rollout-preservation/source_authority.py',
    'compiler': 'nats-rollout-preservation/compiler.py',
    'commands': 'nats-known-baseline/commands.py',
    'controller': 'nats-known-baseline/controller.py',
    'root_main': 'nats-known-baseline/root_main.py',
    'stage_runtime': 'nats-known-baseline/stage_runtime.py',
}


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def validate_secret(anchor, secret, password):
    """Validate privately and return only identity and protected hashes."""
    try:
        meta, data = secret['metadata'], secret['data']
        if (type(secret) is not dict or meta.get('name') != 'voice-app-secrets'
                or meta.get('namespace') != 'voice-staging' or secret.get('type') != 'Opaque'
                or meta.get('deletionTimestamp')
                or not isinstance(meta.get('uid'), str) or not meta['uid'] or meta['uid'] != anchor['uid']
                or not isinstance(meta.get('resourceVersion'), str) or not meta['resourceVersion']
                or type(data) is not dict or not all(isinstance(k, str) and isinstance(v, str) for k, v in data.items())
                or not isinstance(password, bytes) or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}', password)):
            raise ValueError()
        value = base64.b64decode(data['REALTIME_DATABASE_URL'], validate=True)
        expected = b'postgres://realtime:' + password + b'@voice-postgres:5432/realtime_db?sslmode=disable'
        if value != expected: raise ValueError()
        return {'uid': meta['uid'], 'resourceVersion': meta['resourceVersion'],
                'data_sha256': digest(data), 'realtime_value_sha256': hashlib.sha256(value).hexdigest()}
    except (KeyError, TypeError, ValueError, AttributeError):
        raise Blocked('realtime_existing_secret_changed') from None


def validate_engine(original, current):
    """Bind durable engine identity; transient Pod/RVs are fenced separately."""
    try:
        fields = {'statefulset', 'pod', 'pvc', 'pv', 'service', 'endpoints', 'slice', 'system_identifier'}
        structures = {'statefulset': {'uid', 'resourceVersion'},
                      'pvc': {'name', 'uid', 'resourceVersion'}, 'pv': {'name', 'uid', 'resourceVersion'},
                      'service': {'uid', 'resourceVersion', 'cluster_ip'},
                      'endpoints': {'uid', 'resourceVersion', 'pod_ip'}, 'slice': {'uid', 'resourceVersion'}}
        if (type(current) is not dict or set(current) != fields
                or any(type(current[k]) is not dict or set(current[k]) != keys
                       or any(not isinstance(v, str) or not v for v in current[k].values())
                       for k, keys in structures.items())
                or type(current['pod']) is not dict or set(current['pod']) != {'uid', 'resourceVersion', 'container_id', 'restart_count'}
                or any(not isinstance(current['pod'][k], str) or not current['pod'][k] for k in ('uid', 'resourceVersion', 'container_id'))
                or type(current['pod']['restart_count']) is not int or current['pod']['restart_count'] < 0
                or original['system_identifier'] != current['system_identifier']
                or not re.fullmatch(r'[0-9]{1,20}', current['system_identifier'])
                or any(original[name]['uid'] != current[name]['uid'] for name in ('statefulset', 'pvc', 'pv', 'service'))
                or any(original[name]['name'] != current[name]['name'] for name in ('pvc', 'pv'))):
            raise ValueError()
    except (KeyError, TypeError, ValueError, AttributeError):
        raise Blocked('realtime_existing_engine_changed') from None


def validate_catalog(receipt, catalog):
    """Compare current catalog to immutable original OIDs and role contract.

    This is only a comparison primitive, not source or operation authority.
    """
    try:
        role, database = catalog['role'], catalog['database']
        fields = {'oid', 'login', 'superuser', 'createdb', 'createrole', 'inherit',
                  'replication', 'bypassrls', 'memberships', 'connection_limit', 'valid_until', 'config'}
        if (type(catalog) is not dict or set(catalog) != {'role', 'database'}
                or type(role) is not dict or set(role) != fields
                or type(database) is not dict or set(database) != {'oid', 'owner_oid'}
                or any(type(receipt.get(k)) is not int or receipt[k] <= 0
                       for k in ('role_oid', 'database_oid', 'owner_oid'))
                or type(role['oid']) is not int or role['oid'] != receipt['role_oid']
                or type(database['oid']) is not int or database['oid'] != receipt['database_oid']
                or type(database['owner_oid']) is not int or database['owner_oid'] != receipt['owner_oid']
                or database['owner_oid'] != role['oid'] or role['login'] is not True
                or any(role[k] is not False for k in ('superuser', 'createdb', 'createrole', 'inherit', 'replication', 'bypassrls'))
                or type(role['memberships']) is not int or role['memberships'] != 0
                or type(role['connection_limit']) is not int or role['connection_limit'] != -1
                or role['valid_until'] is not None or role['config'] is not None):
            raise ValueError()
    except (KeyError, TypeError, ValueError, AttributeError):
        raise Blocked('realtime_existing_catalog_changed') from None


def _private(path, limit=2 << 20):
    """Read a fixed root-owned protected file, returning bytes and document."""
    import operator_retirement
    path = Path(path)
    for parent in path.parents:
        info = parent.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
            raise Blocked('realtime_existing_custody_invalid')
    raw = operator_retirement.private_bytes(path, limit)
    return raw, json.loads(raw)


def _rollout(operation,*,installation=None):
    import guard
    import resource_claim
    if not isinstance(operation, str) or not re.fullmatch(r'[a-f0-9]{12}', operation):
        raise Blocked('realtime_existing_anchor_invalid')
    raw, state = _private(guard.ROOT / ('rollout-' + operation) / 'checkpoint.json', 128 << 20)
    if (type(state) is not dict or state.get('status') != 'PASS' or state.get('operation') != operation
            or state.get('phase') != 'POST_RESTART' or state.get('fence_status') != 'RELEASED'):
        raise Blocked('realtime_existing_anchor_invalid')
    binding = resource_claim.historical_binding(state,installation=installation)
    receipt = state.get('realtime_bootstrap')
    scoped = binding['schema']=='voice-staging-resource-fence-v2'
    scoped_schemas = ('voice-staging-realtime-bootstrap-receipt-v2', 'voice-staging-realtime-bootstrap-existing-v3', 'voice-staging-realtime-bootstrap-existing-v4')
    legacy_schemas = ('voice-staging-realtime-bootstrap-receipt-v1', 'voice-staging-realtime-bootstrap-existing-v1', 'voice-staging-realtime-bootstrap-existing-v2')
    if type(receipt) is not dict or receipt.get('schema') not in (scoped_schemas if scoped else legacy_schemas):
        raise Blocked('realtime_existing_anchor_invalid')
    if (type(receipt) is not dict or state['target'].get('realtime_bootstrap_sha256') != digest(receipt)
            or binding['operation'] != operation):
        raise Blocked('realtime_existing_anchor_invalid')
    return state, receipt, binding, hashlib.sha256(raw).hexdigest()


def _anchor(*,installation=None):
    """Follow protected active PASS and, for successors, its original C1 anchor."""
    import guard
    import realtime_bootstrap as c1
    import resource_claim
    installation=resource_claim.require_installation(installation)
    _, active = _private(guard.ROOT / 'installed' / 'active-release.json')
    resource_claim.validate_active_release(active,installation=installation)
    operation = active.get('operation')
    state, receipt, binding, checkpoint_sha = _rollout(operation,installation=installation)
    if receipt.get('schema') in ('voice-staging-realtime-bootstrap-existing-v2','voice-staging-realtime-bootstrap-existing-v4'):
        import staging_prerequisites
        historical=staging_prerequisites.validate_historical_capture(operation,installation=installation)
        if digest(historical)!=receipt.get('prerequisites_sha256'):
            raise Blocked('realtime_existing_anchor_changed')
    expected = state['context']['expected']
    if (active.get('namespace_uid') != expected['namespace_uid']
            or active.get('pvc_uid') != expected['source_claim_uid']
            or active.get('target_sha256') != state['target']['manifest_sha256']):
        raise Blocked('realtime_existing_anchor_invalid')
    if receipt.get('schema') in ('voice-staging-realtime-bootstrap-receipt-v1','voice-staging-realtime-bootstrap-receipt-v2'):
        anchor = {'operation': operation, 'receipt_sha256': digest(receipt)}
        original_state, original_receipt = state, receipt
    elif receipt.get('schema') in ('voice-staging-realtime-bootstrap-existing-v1','voice-staging-realtime-bootstrap-existing-v2','voice-staging-realtime-bootstrap-existing-v3','voice-staging-realtime-bootstrap-existing-v4'):
        _validate_record(receipt,installation=installation)
        if (receipt['operation'] != operation or receipt['context'] != binding['context']
                or receipt['helper_sha256'] != binding['helper_sha256']):
            raise Blocked('realtime_existing_anchor_invalid')
        _, saved = _private(guard.ROOT / 'installed' / 'realtime-bootstrap' / operation / 'existing.json')
        if saved != receipt: raise Blocked('realtime_existing_anchor_invalid')
        anchor = receipt['anchor']
        original_state, original_receipt, _, _ = _rollout(anchor['operation'],installation=installation)
        if (original_receipt.get('schema') not in ('voice-staging-realtime-bootstrap-receipt-v1','voice-staging-realtime-bootstrap-receipt-v2')
                or digest(original_receipt) != anchor['receipt_sha256']):
            raise Blocked('realtime_existing_anchor_invalid')
    else:
        raise Blocked('realtime_existing_anchor_invalid')
    original_dir = guard.ROOT / 'installed' / 'realtime-bootstrap' / anchor['operation']
    _, checkpoint = _private(original_dir / 'checkpoint.json')
    plan, validated_receipt = c1.validate_historical_checkpoint(checkpoint, original_state,installation=installation)
    if (validated_receipt != original_receipt or plan['namespace_uid'] != active['namespace_uid']
            or original_state['context']['expected']['namespace_uid'] != plan['namespace_uid']):
        raise Blocked('realtime_existing_anchor_invalid')
    return {'anchor': anchor, 'active_release': {'operation': operation, 'row_sha256': digest(active),
            'checkpoint_sha256': checkpoint_sha}}, plan, validated_receipt, original_dir


def _validate_original_provenance(plan,provenance,*,installation=None):
    import resource_claim
    try:
        installation=resource_claim.require_installation(installation)
        schema=plan['schema']
        if schema in ('voice-staging-realtime-bootstrap-v1','voice-staging-realtime-bootstrap-v2'):
            resource_claim.require_production(installation)
            if 'installation_sha256' in plan:raise ValueError()
            source_schema='voice-staging-source-capture-v1'
        elif schema in ('voice-staging-realtime-bootstrap-v3','voice-staging-realtime-bootstrap-v4'):
            if plan.get('installation_sha256')!=installation.sha256:raise ValueError()
            source_schema='voice-staging-source-capture-v2'
        else:raise ValueError()
        resource_claim.validate_source_capture(provenance,installation=installation)
        expected={'schema':source_schema,'operation':plan['operation'],'context':plan['context'],
                  'helper_sha256':plan['helper_sha256'],'approved_sha256':plan['source_authority_sha256'],
                  'execution_sha256':plan['execution_sha256']}
        if source_schema=='voice-staging-source-capture-v2':expected['installation_sha256']=installation.sha256
        if provenance!=expected:raise ValueError()
    except (ValueError,RuntimeError,TypeError,KeyError,AttributeError):
        raise Blocked('realtime_existing_anchor_invalid') from None


def _original_credential(directory, plan, receipt,*,installation=None):
    import guard
    import realtime_bootstrap as c1
    import resource_claim
    installation=resource_claim.require_installation(installation)
    _, before = _private(directory / 'before-secret.json')
    _, authority = _private(directory / 'authority.json')
    _, stage = _private(directory / 'stage.json')
    _, provenance = _private(guard.ROOT / 'installed' / 'source-authority' / (plan['operation'] + '.json'))
    _validate_original_provenance(plan,provenance,installation=installation)
    if (plan['schema'] in ('voice-staging-realtime-bootstrap-v3','voice-staging-realtime-bootstrap-v4')
        and stage.get('marker',{}).get('data',{}).get('generation')!=installation.generation):
        raise Blocked('realtime_existing_generation_changed')
    # Check trusted parents through _private before reading the private credential.
    password = c1._read_private(directory / 'credential', limit=128)
    if (hashlib.sha256(password).hexdigest() != plan['credential_sha256']
            or c1._secret_identity(before) != plan['secret']
            or digest(authority['approved']) != plan['source_authority_sha256']
            or digest(authority['approved']['source_files']) != plan['source_files_sha256']
            or digest(authority['execution']) != plan['execution_sha256']
            or digest(stage) != plan['stage_sha256'] or digest(stage['marker']) != plan['active_marker_sha256']):
        raise Blocked('realtime_existing_anchor_invalid')
    patch = c1.secret_patch(before, password)
    data = copy.deepcopy(before['data']); data['REALTIME_DATABASE_URL'] = patch[-1]['value']
    if (digest(data) != receipt['secret']['data_sha256']
            or before['metadata']['uid'] != receipt['secret']['uid']):
        raise Blocked('realtime_existing_anchor_invalid')
    return password


def _validate_record(record,*,installation=None):
    import resource_claim
    installation=resource_claim.require_installation(installation)
    fields = {'schema', 'operation', 'context', 'helper_sha256', 'source_authority_sha256',
              'execution_sha256', 'stage_sha256', 'actual_images', 'namespace_uid', 'anchor',
              'active_release', 'live'}
    parented=isinstance(record,dict) and record.get('schema') in ('voice-staging-realtime-bootstrap-existing-v2','voice-staging-realtime-bootstrap-existing-v4')
    if parented:fields.add('prerequisites_sha256')
    try:
        resource_claim.record_fields(record,'realtime_existing',fields,installation=installation)
        if (type(record) is not dict
                or parented and (not isinstance(record.get('prerequisites_sha256'),str)
                    or not re.fullmatch(r'[a-f0-9]{64}',record['prerequisites_sha256']))
                or not re.fullmatch(r'[a-f0-9]{12}', record['operation'])
                or type(record['context']) is not dict or type(record['actual_images']) is not dict
                or not record['actual_images'] or not isinstance(record['namespace_uid'], str) or not record['namespace_uid']
                or any(not isinstance(record[k], str) or not re.fullmatch(r'[a-f0-9]{64}', record[k])
                       for k in ('helper_sha256', 'source_authority_sha256', 'execution_sha256', 'stage_sha256'))):
            raise ValueError()
        anchor, active, live = record['anchor'], record['active_release'], record['live']
        if (type(anchor) is not dict or set(anchor) != {'operation', 'receipt_sha256'}
                or not re.fullmatch(r'[a-f0-9]{12}', anchor['operation'])
                or not re.fullmatch(r'[a-f0-9]{64}', anchor['receipt_sha256'])
                or type(active) is not dict or set(active) != {'operation', 'row_sha256', 'checkpoint_sha256'}
                or not re.fullmatch(r'[a-f0-9]{12}', active['operation'])
                or any(not re.fullmatch(r'[a-f0-9]{64}', active[k]) for k in ('row_sha256', 'checkpoint_sha256'))
                or type(live) is not dict or set(live) != {'postgres', 'role_oid', 'database_oid', 'owner_oid', 'secret'}
                or type(live['postgres']) is not dict
                or any(type(live[k]) is not int or live[k] <= 0 for k in ('role_oid', 'database_oid', 'owner_oid'))
                or type(live['secret']) is not dict
                or set(live['secret']) != {'uid', 'resourceVersion', 'data_sha256', 'realtime_value_sha256'}
                or any(not isinstance(live['secret'][k], str) or not live['secret'][k] for k in ('uid', 'resourceVersion'))
                or any(not re.fullmatch(r'[a-f0-9]{64}', live['secret'][k]) for k in ('data_sha256', 'realtime_value_sha256'))):
            raise ValueError()
    except (KeyError, TypeError, ValueError, AttributeError,RuntimeError):
        raise Blocked('realtime_existing_record_invalid') from None


def _stage_view(stage):
    snapshots = {}
    for name, obj in stage.snapshots.items():
        meta = obj['metadata']
        if (not isinstance(meta.get('uid'), str) or not meta['uid']
                or meta.get('deletionTimestamp') or type(meta.get('generation')) is not int or meta['generation'] <= 0):
            raise Blocked('realtime_existing_stage_invalid')
        snapshots[name] = {'uid': meta['uid'], 'generation': meta['generation'], 'spec': copy.deepcopy(obj['spec'])}
    if not snapshots or type(stage.old_images) is not dict or not stage.old_images:
        raise Blocked('realtime_existing_stage_invalid')
    return {'snapshots': snapshots, 'marker': copy.deepcopy(stage.marker), 'actual_images': copy.deepcopy(stage.old_images)}


def _check_stage(kube, view):
    if kube.get('configmap', 'voice-nats-generation') != view['marker']:
        raise Blocked('realtime_existing_stage_changed')
    for name, expected in view['snapshots'].items():
        actual = kube.get('deployment', name)
        if (actual['metadata'].get('deletionTimestamp') or actual['metadata'].get('uid') != expected['uid']
                or actual['metadata'].get('generation') != expected['generation']
                or actual['spec'] != expected['spec']):
            raise Blocked('realtime_existing_stage_changed')


def _observe(session, kube, plan, receipt, password):
    import realtime_bootstrap as c1
    session.check()
    namespace, pg = c1.engine_snapshot(kube)
    pg['system_identifier'] = c1._system_identifier()
    if namespace != plan['namespace_uid']: raise Blocked('realtime_existing_engine_changed')
    validate_engine(plan['postgres'], pg)
    secret = kube.get('secret', 'voice-app-secrets')
    secret_identity = validate_secret(receipt['secret'], secret, password)
    c1._verify_admin(secret)
    catalog = c1._catalog(); validate_catalog(receipt, catalog)
    c1._verify_runtime_auth(password)
    namespace2, pg2 = c1.engine_snapshot(kube)
    pg2['system_identifier'] = c1._system_identifier()
    if (namespace2 != namespace or pg2 != pg or c1._catalog() != catalog
            or validate_secret(receipt['secret'], kube.get('secret', 'voice-app-secrets'), password) != secret_identity):
        raise Blocked('realtime_existing_live_changed')
    session.check()
    return {'postgres': pg, 'role_oid': catalog['role']['oid'], 'database_oid': catalog['database']['oid'],
            'owner_oid': catalog['database']['owner_oid'], 'secret': secret_identity}


def _session(session):
    import commands
    import resource_claim
    if (not isinstance(session, resource_claim.Session) or not session.admitted
            or getattr(commands.authority_guard, '__self__', None) is not session):
        raise Blocked('realtime_existing_authority_invalid')
    row = session.check()
    if row.get('phase') not in ('prepare','prerequisites-captured'): raise Blocked('realtime_existing_phase_invalid')
    return row


def _source(session, approved, execution):
    import guard
    import gis_existing_update
    import realtime_bootstrap as c1
    from root_main import public_read
    if (type(approved) is not dict or approved.get('schema') != 'voice-source-authority-v1'
            or approved.get('verified') is not True or approved.get('source_sha') != session.context['commit']
            or type(execution) is not dict or execution.get('head_sha') != session.context['commit']):
        raise Blocked('realtime_existing_source_invalid')
    _session(session)
    _, provenance = _private(guard.ROOT / 'installed' / 'source-authority' / (session.operation + '.json'))
    expected = {'schema': 'voice-staging-source-capture-v2', 'installation_sha256': session.installation.sha256,
                'operation': session.operation,
                'context': session.context, 'helper_sha256': session.helper,
                'approved_sha256': digest(approved), 'execution_sha256': digest(execution)}
    import resource_claim
    try:resource_claim.validate_source_capture(provenance,installation=session.installation)
    except (ValueError,RuntimeError,TypeError,AttributeError):
        raise Blocked('realtime_existing_source_invalid') from None
    if provenance != expected: raise Blocked('realtime_existing_source_invalid')
    gis_existing_update.validate_source_authority(approved, {'tag': session.context['commit'],
                                                             'images': session.context['images']})
    root = guard.ROOT / 'installed' / 'sources' / session.operation
    for module, relative in SOURCE_HELPERS.items():
        source_path = 'scripts/staging/' + relative
        installed = Path(importlib.import_module(module).__file__)
        actual = hashlib.sha256(public_read(installed, limit=1 << 20)).hexdigest()
        if (approved.get('source_files', {}).get(source_path) != actual
                or hashlib.sha256(public_read(root / source_path, limit=1 << 20)).hexdigest() != actual):
            raise Blocked('realtime_existing_source_changed')
    c1._source_contract(root, approved)
    _session(session)


def _load_existing(session, actual, rollout_target):
    import guard
    from stage_runtime import Kube
    row = _session(session)
    directory = guard.ROOT / 'installed' / 'realtime-bootstrap' / session.operation
    _, record = _private(directory / 'existing.json')
    _, authority = _private(directory / 'authority.json')
    _, stage = _private(directory / 'stage.json')
    _validate_record(record,installation=session.installation)
    parent=None
    if record['schema'] in ('voice-staging-realtime-bootstrap-existing-v2','voice-staging-realtime-bootstrap-existing-v4'):
        import staging_prerequisites
        parent=staging_prerequisites.load_capture(session,actual=actual,rollout_target=rollout_target)
        if digest(parent)!=record['prerequisites_sha256']:raise Blocked('realtime_existing_parent_changed')
    if (record['operation'] != session.operation or record['context'] != session.context
            or record['helper_sha256'] != session.helper or type(authority) is not dict
            or set(authority) != {'approved', 'execution'}
            or digest(authority['approved']) != record['source_authority_sha256']
            or digest(authority['execution']) != record['execution_sha256']
            or digest(stage) != record['stage_sha256'] or stage['actual_images'] != record['actual_images']
            or actual is not None and actual != record['actual_images']):
        raise Blocked('realtime_existing_record_changed')
    if rollout_target is None:
        if parent is None and row.get('target_sha256') is not None: raise Blocked('realtime_existing_target_changed')
        if parent is not None and row.get('phase')!='prerequisites-captured':raise Blocked('realtime_existing_target_changed')
    elif (type(rollout_target) is not dict or session.binding()['target_sha256'] != digest(rollout_target)
          or rollout_target.get('realtime_bootstrap_sha256') != digest(record)):
        raise Blocked('realtime_existing_target_changed')
    _source(session, authority['approved'], authority['execution'])
    ancestry, plan, receipt, directory0 = _anchor(installation=session.installation)
    if (ancestry['anchor'] != record['anchor'] or ancestry['active_release'] != record['active_release']
            or plan['namespace_uid'] != record['namespace_uid']):
        raise Blocked('realtime_existing_anchor_changed')
    password = _original_credential(directory0, plan, receipt,installation=session.installation)
    kube = Kube()
    _check_stage(kube, stage)
    if _observe(session, kube, plan, receipt, password) != record['live']:
        raise Blocked('realtime_existing_live_changed')
    _check_stage(kube, stage)
    ancestry2, plan2, receipt2, directory2 = _anchor(installation=session.installation)
    if ((ancestry2, plan2, receipt2, directory2) != (ancestry, plan, receipt, directory0)
            or _original_credential(directory2, plan2, receipt2,installation=session.installation) != password):
        raise Blocked('realtime_existing_anchor_changed')
    _source(session, authority['approved'], authority['execution'])
    _session(session)
    return copy.deepcopy(record)


def load_existing(session, *, actual=None, rollout_target=None):
    """Load only an authenticated current-operation admission and reobserve it."""
    try:
        return _load_existing(session, actual, rollout_target)
    except Blocked:
        raise
    except Exception:
        raise Blocked('realtime_existing_observation_failed') from None


def run_existing(actions, request, approved, execution, source_root, stage):
    """Capture once under the current root Session; no Kubernetes/SQL writes."""
    import bridge_root
    import guard
    import operator_retirement
    import resource_claim
    from stage_runtime import Kube
    try:
        session = actions.tt_session
        if type(actions) is not bridge_root.Actions or type(stage) is not __import__('runtime_stage').RolloutStage:
            raise Blocked('realtime_existing_authority_invalid')
        resource_claim.require_session(session,installation=actions.installation)
        row = _session(session)
        parent=None
        if row.get('phase')=='prerequisites-captured':
            import staging_prerequisites
            parent=staging_prerequisites.load_capture(session,actual=stage.old_images)
        if (request.get('action') != 'prepare' or request.get('mode') not in ('full', 'app-only')
                or request.get('provisional_staging') is True or request.get('nonce', '')[:12] != session.operation
                or parent is None and (row.get('phase') != 'prepare' or row.get('target_sha256') is not None)
                or Path(source_root) != guard.ROOT / 'installed' / 'sources' / session.operation
                or digest(actions.binding) != session.helper):
            raise Blocked('realtime_existing_authority_invalid')
        key = 'nats-rollout-preservation/realtime_bootstrap_existing.py'
        if actions.binding.get(key) != hashlib.sha256(Path(__file__).read_bytes()).hexdigest():
            raise Blocked('realtime_existing_source_changed')
        _source(session, approved, execution)
        card = session.authority(); observed = card.get('observed', {})
        if (observed.get('state') != 'known' or observed.get('claim') != session.context['claim']
                or observed.get('window') != session.context['window']
                or observed.get('operation') != card.get('operation', 0) - 1
                or observed.get('images') != (parent['actual_before'] if parent is not None else stage.old_images)):
            raise Blocked('realtime_existing_baseline_changed')
        directory = guard.ROOT / 'installed' / 'realtime-bootstrap' / session.operation
        if directory.exists():
            record = load_existing(session, actual=stage.old_images)
            if (record['source_authority_sha256'] != digest(approved)
                    or record['execution_sha256'] != digest(execution)
                    or record['stage_sha256'] != digest(_stage_view(stage))):
                raise Blocked('realtime_existing_record_changed')
            return record
        ancestry, plan, receipt, original_dir = _anchor(installation=session.installation)
        password = _original_credential(original_dir, plan, receipt,installation=session.installation)
        view = _stage_view(stage); kube = Kube(); _check_stage(kube, view)
        live = _observe(session, kube, plan, receipt, password)
        _check_stage(kube, view)
        ancestry2, plan2, receipt2, directory2 = _anchor(installation=session.installation)
        if ((ancestry2, plan2, receipt2, directory2) != (ancestry, plan, receipt, original_dir)
                or _original_credential(directory2, plan2, receipt2,installation=session.installation) != password):
            raise Blocked('realtime_existing_anchor_changed')
        _source(session, approved, execution); _session(session)
        record = {'schema': 'voice-staging-realtime-bootstrap-existing-v3','installation_sha256':session.installation.sha256, 'operation': session.operation,
                  'context': copy.deepcopy(session.context), 'helper_sha256': session.helper,
                  'source_authority_sha256': digest(approved), 'execution_sha256': digest(execution),
                  'stage_sha256': digest(view), 'actual_images': copy.deepcopy(stage.old_images),
                  'namespace_uid': plan['namespace_uid'], 'anchor': ancestry['anchor'],
                  'active_release': ancestry['active_release'], 'live': live}
        if parent is not None:
            record['schema']='voice-staging-realtime-bootstrap-existing-v4'
            record['prerequisites_sha256']=digest(parent)
        _validate_record(record,installation=session.installation)
        parent = directory.parent
        # The operation directory is root-owned, exclusive and never adopted.
        for path in (parent, *parent.parents):
            info = path.lstat()
            if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
                raise Blocked('realtime_existing_custody_invalid')
        directory.mkdir(mode=0o700)
        fd = os.open(parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try: os.fsync(fd)
        finally: os.close(fd)
        for name, value in (('authority.json', {'approved': approved, 'execution': execution}),
                            ('stage.json', view), ('existing.json', record)):
            raw = json.dumps(value, sort_keys=True, separators=(',', ':')).encode() + b'\n'
            operator_retirement.exclusive(directory / name, raw)
        return load_existing(session, actual=stage.old_images)
    except Blocked:
        raise
    except Exception:
        raise Blocked('realtime_existing_observation_failed') from None
