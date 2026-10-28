"""Pure DEV frontend adoption plans; execution and authority belong to caller.

Consumer contract: under the shared claim/fence, capture the exact Deployment
and admitted immutable image, then call plan(app, deployment, image). Store the
complete plan privately: it contains the old and target specifications and may
contain sensitive environment references. Create the returned ConfigMap only
after an authoritative absence check, or refuse and reconcile an existing
object. Read it back and validate_configmap to record UID and public bytes hash
before applying the JSON patch. The caller must guard ConfigMap UID, resourceVersion
and contents before rollout and after Pod replacement, verify the Pod owner
chain and served runtime document, enforce the admitted image/source, journal
intent before every effect and reconcile unknown responses; this module cannot grant
authority, execute effects, prove ownership, retry or roll back.

classify is observation only. 'target' does not establish who committed it or
that pods are ready; 'old' after a lost response never authorizes replay. Rollout
and recovery must retain the complete private plan, verify actual workload and
ConfigMap identities, and hold on any unowned, changed or uncertain state.
"""
import copy
import hashlib
import json
import re


class Refused(ValueError):
    pass


APPS = {'web': ('voice-web', 'web'), 'admin': ('voice-admin', 'admin'),
        'portal': ('voice-developer-portal', 'portal')}
ORIGINS = {'web': 'https://dev.comrade.click', 'admin': 'https://admin-dev.comrade.click',
           'portal': 'https://developers-dev.comrade.click'}
MOUNT = '/usr/share/nginx/html/voice-runtime-config.json'
KEY = 'voice-runtime-config.json'


def configmap(app):
    if app not in APPS:
        raise Refused('app outside DEV scope')
    data = {'version': 1, 'app_origin': ORIGINS[app], 'api_base_url': 'https://api-dev.comrade.click'}
    if app == 'web':
        data['livekit_url'] = 'wss://livekit-dev.comrade.click'
    return {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
            'metadata': {'namespace': 'voice-dev', 'name': APPS[app][0] + '-runtime-config'},
            'data': {KEY: json.dumps(data, separators=(',', ':')) + '\n'}}


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise Refused('duplicate config key')
        result[key] = value
    return result


def validate_configmap(app, actual):
    expected = configmap(app)
    try:
        metadata = actual['metadata']
        data = actual['data']
        raw = data[KEY]
        parsed = json.loads(raw, object_pairs_hook=_unique)
        wanted = json.loads(expected['data'][KEY])
        if (actual['kind'] != 'ConfigMap' or metadata['namespace'] != 'voice-dev'
                or metadata['name'] != expected['metadata']['name'] or not metadata.get('uid')
                or not metadata.get('resourceVersion') or actual.get('immutable') is not True
                or metadata.get('deletionTimestamp') or set(data) != {KEY}
                or not isinstance(parsed, dict) or type(parsed.get('version')) is not int
                or parsed != wanted or actual.get('binaryData')):
            raise Refused('configmap identity or public tuple differs')
        return {'uid': metadata['uid'], 'resourceVersion': metadata['resourceVersion'],
                'sha256': hashlib.sha256(raw.encode('utf-8')).hexdigest()}
    except (KeyError, TypeError, ValueError) as exc:
        raise Refused('configmap identity or public tuple differs') from exc


def plan(app, old, image):
    if app not in APPS or not isinstance(image, str) or not re.fullmatch(
            r'[a-z0-9][a-z0-9./:_-]*@sha256:[a-f0-9]{64}', image):
        raise Refused('app or immutable image invalid')
    name, container = APPS[app]
    try:
        meta = old['metadata']
        if (old['kind'] != 'Deployment' or meta['namespace'] != 'voice-dev'
                or meta['name'] != name or not meta['uid'] or not meta['resourceVersion']
                or meta.get('deletionTimestamp')):
            raise Refused('deployment identity invalid')
        original = copy.deepcopy(old['spec'])
        target = copy.deepcopy(original)
        pod = target['template']['spec']
        containers = [c for c in pod['containers'] if c['name'] == container]
        if len(containers) != 1:
            raise Refused('application container is not unique')
        volumes = pod.setdefault('volumes', [])
        mounts = containers[0].setdefault('volumeMounts', [])
        if (any(v.get('name') == 'runtime-config' for v in volumes)
                or any(m.get('name') == 'runtime-config' or m.get('mountPath') == MOUNT for m in mounts)):
            raise Refused('runtime config projection collision')
        volumes.append({'name': 'runtime-config', 'configMap': {'name': name + '-runtime-config'}})
        mounts.append({'name': 'runtime-config', 'mountPath': MOUNT, 'subPath': KEY, 'readOnly': True})
        containers[0]['image'] = image
        patch = [{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
                 {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
                 {'op': 'test', 'path': '/spec', 'value': original},
                 {'op': 'replace', 'path': '/spec', 'value': target}]
        return {'app': app, 'uid': meta['uid'], 'resourceVersion': meta['resourceVersion'],
                'old_spec': original, 'target_spec': target, 'configmap': configmap(app), 'patch': patch}
    except (KeyError, TypeError, AttributeError) as exc:
        raise Refused('invalid deployment shape') from exc


def classify(plan, actual):
    """Classify observed state, not ownership or permission to retry an effect."""
    try:
        meta = actual['metadata']
        if (actual['kind'] != 'Deployment' or meta['namespace'] != 'voice-dev'
                or meta['name'] != APPS[plan['app']][0] or meta['uid'] != plan['uid']
                or meta.get('deletionTimestamp')):
            return 'conflict'
        if actual['spec'] == plan['target_spec']:
            return 'target'
        if actual['spec'] == plan['old_spec'] and meta['resourceVersion'] == plan['resourceVersion']:
            return 'old'
    except (KeyError, TypeError):
        pass
    return 'conflict'
