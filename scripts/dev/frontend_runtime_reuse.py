"""Plan an image-only change on an already-adopted DEV frontend."""
import copy
import re

import frontend_runtime_adoption as adoption


def plan(app, old, image):
    if app not in adoption.APPS or not isinstance(image, str) or not re.fullmatch(
            r'[a-z0-9][a-z0-9./:_-]*@sha256:[a-f0-9]{64}', image):
        raise adoption.Refused('app or immutable image invalid')
    name, container = adoption.APPS[app]
    try:
        meta = old['metadata']
        if (old['kind'] != 'Deployment' or meta['namespace'] != 'voice-dev' or meta['name'] != name
                or not meta['uid'] or not meta['resourceVersion'] or meta.get('deletionTimestamp')
                or type(meta.get('generation')) is not int or meta['generation'] < 1):
            raise adoption.Refused('deployment identity invalid')
        original = copy.deepcopy(old['spec'])
        target = copy.deepcopy(original)
        pod = target['template']['spec']
        containers = [c for c in pod['containers'] if c['name'] == container]
        if len(containers) != 1:
            raise adoption.Refused('application container is not unique')
        volumes = [v for v in pod.get('volumes', []) if v.get('name') == 'runtime-config']
        mounts = [m for m in containers[0].get('volumeMounts', [])
                  if m.get('name') == 'runtime-config' or m.get('mountPath') == adoption.MOUNT]
        if (volumes != [{'name': 'runtime-config', 'configMap': {'name': name + '-runtime-config', 'defaultMode': 420}}]
                or mounts != [{'name': 'runtime-config', 'mountPath': adoption.MOUNT,
                               'subPath': adoption.KEY, 'readOnly': True}]):
            raise adoption.Refused('existing runtime projection is not exact')
        containers[0]['image'] = image
        patch = [{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
                 {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
                 {'op': 'test', 'path': '/spec', 'value': original},
                 {'op': 'replace', 'path': '/spec', 'value': target}]
        return {'app': app, 'uid': meta['uid'], 'resourceVersion': meta['resourceVersion'],
                'old_spec': original, 'target_spec': target, 'configmap': adoption.configmap(app), 'patch': patch}
    except (KeyError, TypeError, AttributeError) as exc:
        raise adoption.Refused('invalid deployment shape') from exc
