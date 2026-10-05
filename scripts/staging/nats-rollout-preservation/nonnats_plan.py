"""Private non-NATS target preflight; no mutation is performed here.

Desired non-Secret objects must be API-server-normalized before use. `semantic`
removes identity/status, retaining all other fields for exact preservation.
Secret desired documents contain predicates only, never credential values.
Caller owns canonical coverage construction, guarded action apply and readiness.
"""

import base64
import copy
import hashlib
import importlib.util
import json
from pathlib import Path

ROW_IDS = ('auth-mail', 'minio-credentials', 'principal-secrets', 'user-search-cursor',
           'domains', 'ingress', 'livekit-ingress', 'signing-endpoint', 'minio-cors',
           'frontends', 'app-restart', 'migrations', 'app-config', 'infra', 'db-init',
           'observability')

class PlanError(ValueError):
    pass

SCHEMA = 'nats-rollout-nonnats-v1'
BINDING_SCHEMA = 'nats-rollout-nonnats-binding-v1'
ACTION_KINDS = {'ConfigMap', 'Ingress', 'Deployment', 'StatefulSet', 'Service', 'NetworkPolicy'}
FULL_ONLY = {'app-config', 'infra', 'db-init'}
OPTIONAL = {'ingress', 'livekit-ingress', 'frontends', 'observability'}
SECRET_CONTRACTS = {
    'minio-credentials': {'voice-minio-credentials': {'MINIO_ROOT_USER', 'MINIO_ROOT_PASSWORD'}},
    'user-search-cursor': {'voice-user-search-projection': {'cursor-hmac-key'}},
    'principal-secrets': {
        **{'voice-' + service + '-principal-signing': {'current.pem', 'next.pem', 'active-kid'}
           for service in ('social', 'file', 'search')},
        'voice-principal-ca': {'ca.crt'},
        **{'voice-' + service + '-principal-tls': {'tls.crt', 'tls.key'}
           for service in ('social', 'user', 'space', 'file', 'user-file', 'search')},
    },
}


def _fail(label):
    raise PlanError(label)


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()).hexdigest()


def semantic(obj):
    """Exact resource semantics after removing Kubernetes runtime identity."""
    result = copy.deepcopy(obj)
    result.pop('status', None)
    result.pop('stringData', None)
    metadata = result.get('metadata', {})
    result['metadata'] = {key: value for key, value in metadata.items()
                          if key in ('labels', 'annotations', 'ownerReferences', 'finalizers')}
    # kubectl's bookkeeping is not desired application configuration.
    annotations = result['metadata'].get('annotations', {})
    annotations.pop('kubectl.kubernetes.io/last-applied-configuration', None)
    if not annotations:
        result['metadata'].pop('annotations', None)
    return result


def _secret(obj, desired):
    if set(desired) - {'required_keys', 'min_lengths', 'predicates', 'type'}:
        _fail('secret_predicate_invalid')
    keys = desired.get('required_keys')
    lengths = desired.get('min_lengths', {})
    predicates = desired.get('predicates', {})
    if (not isinstance(keys, list) or not keys or any(not isinstance(k, str) or not k for k in keys)
            or len(set(keys)) != len(keys) or not isinstance(lengths, dict)
            or any(k not in keys or type(v) is not int or v < 1 for k, v in lengths.items())
            or not isinstance(predicates, dict) or set(predicates) - {'auth_mail'}
            or any(v is not True for v in predicates.values())):
        _fail('secret_predicate_invalid')
    if 'type' in desired and obj.get('type', 'Opaque') != desired['type']:
        _fail('secret_type_mismatch')
    values = {}
    try:
        for key in keys:
            value = base64.b64decode(obj['data'][key], validate=True).decode('utf-8')
            if not value.strip() or len(value) < lengths.get(key, 1):
                _fail('required_secret_key_invalid')
            values[key] = value
    except (KeyError, ValueError, TypeError, UnicodeError):
        _fail('required_secret_key_invalid')
    if predicates.get('auth_mail'):
        if not {'AUTH_RESEND_API_KEY', 'AUTH_RESEND_FROM'} <= set(keys):
            _fail('auth_mail_predicate_invalid')
        spec = importlib.util.spec_from_file_location('voice_mail_predicate', Path(__file__).parent.parent / 'mail-only-patch.py')
        helper = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(helper)
        if not helper.valid_sender(values['AUTH_RESEND_FROM']):
            _fail('auth_mail_invalid')


def _read(kube, descriptor):
    try:
        obj = kube.get(descriptor['kind'], descriptor['name'])
        metadata = obj['metadata']
        if (obj['kind'] != descriptor['kind'] or metadata['name'] != descriptor['name']
                or metadata.get('namespace') != descriptor['namespace']
                or any(not isinstance(metadata.get(k), str) or not metadata[k] for k in ('uid', 'resourceVersion'))):
            _fail('live_object_identity_invalid')
        return obj
    except PlanError:
        raise
    except Exception:
        _fail('live_object_read_failed')


def _matches(obj, desired):
    if obj['kind'] == 'Secret':
        _secret(obj, desired)
    elif semantic(obj) != desired:
        _fail('preserve_semantic_mismatch')


def _validate(plan, mode):
    if (not isinstance(plan, dict) or set(plan) != {'schema', 'mode', 'checks', 'actions'}
            or plan['schema'] != SCHEMA or plan['mode'] != mode
            or mode not in ('full', 'app-only', 'images-only')):
        _fail('plan_schema_invalid')
    checks, actions = plan['checks'], plan['actions']
    expected = {'auth-mail', 'signing-endpoint'} if mode == 'images-only' else set(ROW_IDS)
    if (not isinstance(checks, list) or any(not isinstance(r, dict) for r in checks)
            or len(checks) != len(expected) or {r.get('id') for r in checks} != expected
            or not isinstance(actions, list) or (mode == 'images-only' and actions)):
        _fail('plan_coverage_invalid')
    indexed = {}
    for action in actions:
        if not isinstance(action, dict) or set(action) != {'id', 'row', 'manifest'}:
            _fail('unsupported_action')
        manifest = action['manifest']
        if (not isinstance(action['id'], str) or not action['id'] or action['id'] in indexed
                or action['row'] not in expected or not isinstance(manifest, dict)
                or manifest.get('kind') not in ACTION_KINDS
                or manifest.get('metadata', {}).get('namespace') != 'voice-staging'
                or 'nats' in manifest.get('metadata', {}).get('name', '').lower()
                or not manifest.get('metadata', {}).get('name')
                or 'data' in manifest and manifest['kind'] != 'ConfigMap'):
            _fail('unsupported_action')
        indexed[action['id']] = action
    descriptors, used = [], set()
    for row in checks:
        if set(row) - {'id', 'disposition', 'objects', 'reason', 'enabled'}:
            _fail('plan_row_invalid')
        disposition = row.get('disposition')
        objects = row.get('objects')
        if disposition == 'skip':
            if objects != [] or not ((mode == 'app-only' and row['id'] in FULL_ONLY
                    and row.get('reason') == 'mode-inapplicable') or (row['id'] in OPTIONAL
                    and row.get('enabled') is False and row.get('reason') == 'not-requested')):
                _fail('plan_skip_invalid')
            continue
        if disposition not in ('preserve', 'action') or not isinstance(objects, list) or not objects:
            _fail('plan_row_invalid')
        for name, keys in SECRET_CONTRACTS.get(row['id'], {}).items():
            matching = [obj for obj in objects if isinstance(obj, dict)
                        and obj.get('kind') == 'Secret' and obj.get('name') == name]
            if (len(matching) != 1 or not keys <= set(matching[0].get('desired', {}).get('required_keys', []))):
                _fail('required_secret_contract_missing')
        if mode == 'app-only' and row['id'] in FULL_ONLY:
            _fail('plan_app_only_infra_invalid')
        for obj in objects:
            allowed = {'kind', 'name', 'namespace', 'desired', 'action_id'}
            if (not isinstance(obj, dict) or set(obj) - allowed
                    or not {'kind', 'name', 'desired'} <= set(obj)
                    or not isinstance(obj['desired'], dict) or not obj['desired']
                    or any(not isinstance(obj.get(k), str) or not obj[k] for k in ('kind', 'name'))):
                _fail('plan_object_invalid')
            obj = copy.deepcopy(obj)
            obj.setdefault('namespace', 'voice-staging')
            if obj['namespace'] != 'voice-staging':
                _fail('plan_namespace_invalid')
            if row['id'] == 'auth-mail' and (obj['kind'] != 'Secret' or obj['name'] != 'voice-app-secrets'
                    or obj['desired'].get('predicates', {}).get('auth_mail') is not True):
                _fail('auth_mail_predicate_invalid')
            if disposition == 'action':
                action = indexed.get(obj.get('action_id'))
                if (not action or action['row'] != row['id'] or obj['kind'] != action['manifest']['kind']
                        or obj['name'] != action['manifest']['metadata']['name']
                        or obj['desired'] != semantic(action['manifest'])):
                    _fail('action_target_mismatch')
                used.add(obj['action_id'])
            elif 'action_id' in obj:
                _fail('preserve_action_invalid')
            descriptors.append((disposition, obj))
    if used != set(indexed):
        _fail('unconsumed_action')
    return descriptors


def preflight(kube, plan, mode):
    """Return a credential-free JSON binding, or veto before any mutation."""
    try:
        descriptors = _validate(plan, mode)  # Validate every action before any live read.
        binding = {'schema': BINDING_SCHEMA, 'mode': mode, 'plan_sha256': digest(plan), 'objects': []}
        for disposition, descriptor in descriptors:
            obj = _read(kube, descriptor)
            current = descriptor['desired'] if disposition == 'preserve' else semantic(obj)
            _matches(obj, current)
            bound = {k: descriptor[k] for k in ('kind', 'name', 'namespace')}
            bound.update({'uid': obj['metadata']['uid'], 'resourceVersion': obj['metadata']['resourceVersion'],
                          'desired': copy.deepcopy(current), 'disposition': disposition})
            binding['objects'].append(bound)
        return binding
    except PlanError:
        raise
    except Exception:
        _fail('plan_invalid')

def revalidate(kube, binding):
    """Strictly verify the pre-fence binding immediately before paused apply."""
    try:
        if binding.get('schema') != BINDING_SCHEMA or not binding.get('objects'):
            _fail('binding_invalid')
        for descriptor in binding['objects']:
            obj = _read(kube, descriptor)
            if any(obj['metadata'][k] != descriptor[k] for k in ('uid', 'resourceVersion')):
                _fail('live_object_identity_changed')
            _matches(obj, descriptor['desired'])
    except PlanError:
        raise
    except Exception:
        _fail('binding_invalid')
    return True
