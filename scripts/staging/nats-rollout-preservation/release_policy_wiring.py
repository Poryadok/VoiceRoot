"""Pure source configuration contract; no image, object or runtime attestation.

Executed only from the trusted caller/captured compiler, never target source.
The immutable ConfigMap descriptor is an external prerequisite, not an apply.
"""
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import sys

OWNERS = ('auth', 'user', 'notification', 'realtime')
CAPABILITIES = {'phone_auth', 'paid', 'mobile_push', 'public_discovery', 'unfinished_rich_types', 'federation'}
BASELINE = {'text', 'photo', 'video', 'document', 'music'}
RESTRICTED = {'voice', 'sticker', 'gif', 'article', 'location', 'video_note', 'send_silent', 'schedule', 'when_online'}


def pairs(items):
    result = {}
    for key, value in items:
        if key in result:
            raise ValueError('release_policy_duplicate_key')
        result[key] = value
    return result


def keys(value, expected):
    if not isinstance(value, dict) or set(value) != set(expected):
        raise ValueError('release_policy_input_shape')


def validate(value, candidate, namespace):
    keys(value, ('profile', 'source_sha', 'policy_sha256', 'policy_object', 'owners'))
    if (not isinstance(candidate, str) or not re.fullmatch(r'[a-f0-9]{40}', candidate)
            or candidate == '0' * 40 or value['source_sha'] != candidate
            or value['profile'] != 'free-invite-alpha'):
        raise ValueError('release_policy_source_or_profile')
    obj = value['policy_object']
    keys(obj, ('apiVersion', 'kind', 'metadata', 'immutable', 'data'))
    keys(obj['metadata'], ('name', 'namespace'))
    name = obj['metadata']['name']
    if (obj['apiVersion'] != 'v1' or obj['kind'] != 'ConfigMap' or obj['immutable'] is not True
            or not isinstance(name, str) or not re.fullmatch(r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?', name)
            or obj['metadata']['namespace'] != namespace):
        raise ValueError('release_policy_immutable_object')
    if not isinstance(obj['data'], dict) or len(obj['data']) != 1:
        raise ValueError('release_policy_object_data')
    key, raw = next(iter(obj['data'].items()))
    if (not re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}', key) or not isinstance(raw, str)
            or not 1 <= len(raw.encode('utf-8')) <= 65536
            or hashlib.sha256(raw.encode('utf-8')).hexdigest() != value['policy_sha256']):
        raise ValueError('release_policy_digest')
    policy = json.loads(raw, object_pairs_hook=pairs)
    keys(policy, ('schema_version', 'profile', 'source_sha', 'capabilities', 'rich_intents'))
    if (type(policy['schema_version']) is not int or policy['schema_version'] != 2
            or policy['profile'] != value['profile'] or policy['source_sha'] != candidate):
        raise ValueError('release_policy_v2_binding')
    keys(policy['capabilities'], CAPABILITIES)
    keys(policy['rich_intents'], BASELINE | RESTRICTED)
    if (any(state != 'disabled' for state in policy['capabilities'].values())
            or any(policy['rich_intents'][intent] != 'qualified' for intent in BASELINE)
            or any(policy['rich_intents'][intent] != 'disabled' for intent in RESTRICTED)):
        raise ValueError('release_policy_unapproved_values')
    keys(value['owners'], OWNERS)
    for owner in OWNERS:
        row = value['owners'][owner]
        keys(row, ('policy_directory', 'snapshot_file'))
        directory = row['policy_directory']
        snapshot = row['snapshot_file']
        if (not isinstance(directory, str) or not re.fullmatch(r'/[a-zA-Z0-9_/-]+', directory)
                or str(PurePosixPath(directory)) != directory or '..' in PurePosixPath(directory).parts
                or directory == '/' or directory == '/tmp' or directory.startswith('/tmp/')):
            raise ValueError('release_policy_absolute_directory')
        # No persistent volume, directory allocation or supervisor is introduced.
        # Direct current ENTRYPOINT -> process exit -> fresh container root layer.
        if not isinstance(snapshot, str) or not re.fullmatch(r'/tmp/[a-zA-Z0-9][a-zA-Z0-9._-]{0,120}', snapshot):
            raise ValueError('release_policy_snapshot_container_layer')
    return value


def load_input(path, candidate, namespace):
    if not path:
        raise ValueError('release_policy_input_required')
    target = Path(path)
    if target.is_symlink() or not target.is_file() or not 1 <= target.stat().st_size <= 131072:
        raise ValueError('release_policy_input_file')
    return validate(json.loads(target.read_bytes(), object_pairs_hook=pairs), candidate, namespace)


def source_input(source, parameters):
    """Historical source stays data; only new source opts into this contract.

    No environment variable or target checkout code supplies protected inputs.
    The captured caller must explicitly carry the release_policy parameter.
    """
    path = Path(source) / 'deploy/staging/services.yaml'
    if path.is_symlink():
        raise ValueError('release_policy_source_template')
    # Historical frontend-only sources may omit application manifests. Their
    # existing compiler checks still decide their own completeness.
    if not path.is_file():
        return None
    if path.stat().st_size > 2 << 20:
        raise ValueError('release_policy_source_template')
    text = path.read_text(encoding='utf-8-sig')
    if '__RELEASE_POLICY_' not in text:
        return None
    check_template(text, 'voice-staging')
    return validate(parameters.get('release_policy'), parameters['tag'], 'voice-staging')


def replacements(value, candidate, namespace):
    validate(value, candidate, namespace)
    obj = value['policy_object']
    key = next(iter(obj['data']))
    result = {'__RELEASE_POLICY_OBJECT__': obj['metadata']['name'],
              '__RELEASE_POLICY_KEY__': key, '__RELEASE_POLICY_SHA256__': value['policy_sha256']}
    for owner, row in value['owners'].items():
        result['__RELEASE_POLICY_DIRECTORY_' + owner.upper() + '__'] = row['policy_directory']
        result['__RELEASE_SNAPSHOT_' + owner.upper() + '__'] = row['snapshot_file']
    return result


def render(text, value, candidate, namespace):
    validate(value, candidate, namespace)
    check_template(text, namespace)
    for token, replacement in replacements(value, candidate, namespace).items():
        text = text.replace(token, replacement)
    if re.search(r'__RELEASE_[A-Z0-9_]+__', text):
        raise ValueError('release_policy_unresolved_token')
    return text


def check_template(text, namespace):
    """Passive, deliberately narrow canonical syntax before any YAML subprocess.

    Decoded semantic CI additionally verifies full objects. Unknown formatting
    fails closed rather than executing a target parser/helper or guessing YAML.
    """
    seen = []
    for part in re.split(r'^---\s*$', text, flags=re.M):
        match = re.search(r'^  name: voice-(auth|user|notification|realtime)$', part, re.M)
        if not match or not re.search(r'^kind: Deployment$', part, re.M):
            continue
        owner = match[1]
        upper = owner.upper()
        if owner in seen or not re.search(r'^  namespace: ' + re.escape(namespace) + r'$', part, re.M):
            raise ValueError('release_policy_template_identity')
        seen.append(owner)
        start = '        - name: ' + owner + '\n'
        if part.count(start) != 1:
            raise ValueError('release_policy_template_container')
        body = part.split(start, 1)[1].split('\n        - name:', 1)[0]
        expected = {
            'VOICE_RELEASE_PROFILE': 'free-invite-alpha',
            'VOICE_RELEASE_POLICY_FILE': '__RELEASE_POLICY_DIRECTORY_' + upper + '__/policy.json',
            'VOICE_RELEASE_POLICY_SHA256': '__RELEASE_POLICY_SHA256__',
            'VOICE_RELEASE_SNAPSHOT_FILE': '__RELEASE_SNAPSHOT_' + upper + '__'}
        for name, token in expected.items():
            line = '            - {name: ' + name + ', value: "' + token + '"}'
            if line not in body or body.count('name: ' + name + ',') != 1:
                raise ValueError('release_policy_template_environment')
        mount = '            - {name: release-policy, mountPath: "__RELEASE_POLICY_DIRECTORY_' + upper + '__", readOnly: true}'
        volume = '          configMap: {name: "__RELEASE_POLICY_OBJECT__", items: [{key: "__RELEASE_POLICY_KEY__", path: policy.json}]}'
        if mount not in body or volume not in part:
            raise ValueError('release_policy_template_readonly_object')
        if re.search(r'readOnlyRootFilesystem:\s*true|^          (?:command|args):', body, re.M):
            raise ValueError('release_policy_template_snapshot_runtime')
        for path in re.findall(r'mountPath:\s*[\"\']?([^,\s\"\'}]+)', body):
            if path == '/' or path == '/tmp' or path.startswith('/tmp/'):
                raise ValueError('release_policy_template_snapshot_volume')
    if set(seen) != set(OWNERS):
        raise ValueError('release_policy_template_owner_inventory')


def check_documents(documents, value, candidate, namespace):
    """Semantic CI/parsed-source check; does not inspect a live image filesystem."""
    validate(value, candidate, namespace)
    seen = []
    for doc in documents:
        if not doc or doc.get('kind') != 'Deployment':
            continue
        pod = doc['spec']['template']['spec']
        for container in pod['containers']:
            owner = container['name']
            if owner not in OWNERS:
                continue
            if owner in seen:
                raise ValueError('release_policy_duplicate_owner')
            if doc['metadata'].get('name') != 'voice-' + owner or doc['metadata'].get('namespace') != namespace:
                raise ValueError('release_policy_owner_identity')
            seen.append(owner)
            if 'command' in container or 'args' in container:
                raise ValueError('release_policy_snapshot_entrypoint_override')
            entries = [e for e in container.get('env', []) if e['name'].startswith('VOICE_RELEASE_')]
            env = {e['name']: e.get('value') for e in entries}
            row = value['owners'][owner]
            expected = {'VOICE_RELEASE_PROFILE': value['profile'], 'VOICE_RELEASE_POLICY_SHA256': value['policy_sha256'],
                        'VOICE_RELEASE_POLICY_FILE': row['policy_directory'] + '/policy.json',
                        'VOICE_RELEASE_SNAPSHOT_FILE': row['snapshot_file']}
            if len(entries) != 4 or env != expected:
                raise ValueError('release_policy_owner_environment')
            mounts = container.get('volumeMounts', [])
            policy_mount = [m for m in mounts if m.get('name') == 'release-policy']
            if policy_mount != [{'name': 'release-policy', 'mountPath': row['policy_directory'], 'readOnly': True}]:
                raise ValueError('release_policy_owner_readonly_mount')
            volumes = [v for v in pod.get('volumes', []) if v.get('name') == 'release-policy']
            expected_volume = {'name': 'release-policy', 'configMap': {'name': value['policy_object']['metadata']['name'],
                'items': [{'key': next(iter(value['policy_object']['data'])), 'path': 'policy.json'}]}}
            if volumes != [expected_volume]:
                raise ValueError('release_policy_owner_object_binding')
            if container.get('securityContext', {}).get('readOnlyRootFilesystem') is True:
                raise ValueError('release_policy_snapshot_readonly_root')
            for mount in mounts:
                path = mount.get('mountPath', '').rstrip('/') or '/'
                if path == '/' or row['snapshot_file'] == path or row['snapshot_file'].startswith(path + '/'):
                    raise ValueError('release_policy_snapshot_persistent_or_readonly_mount')
    if set(seen) != set(OWNERS):
        raise ValueError('release_policy_owner_inventory')
    return seen


def main(args):
    if len(args) != 4:
        raise ValueError('release_policy_render_arguments')
    descriptor, candidate, namespace, template = args
    value = load_input(descriptor, candidate, namespace)
    sys.stdout.write(render(Path(template).read_text(encoding='utf-8-sig'), value, candidate, namespace))


if __name__ == '__main__':
    try:
        main(sys.argv[1:])
    except (ValueError, OSError, TypeError, KeyError):
        print('ERROR: invalid or missing bound release policy input', file=sys.stderr)
        sys.exit(1)
