"""Read-only DEV baseline collection; no admission or rollout authority."""
import argparse
import hashlib
import json
import re
import subprocess

RESOURCES = ('namespace', 'deployments', 'statefulsets', 'pods', 'replicasets', 'pvcs', 'jobs')
PVCS = frozenset(('pgdata-voice-postgres-0', 'data-voice-minio-0',
                  'chdata-voice-clickhouse-0', 'voice-nats-jsdata'))


def _read(context, namespace, resource):
    if resource == 'namespace':
        path = '/api/v1/namespaces/' + namespace
    else:
        paths = {'deployments': '/apis/apps/v1', 'statefulsets': '/apis/apps/v1',
                 'replicasets': '/apis/apps/v1', 'jobs': '/apis/batch/v1',
                 'pods': '/api/v1', 'pvcs': '/api/v1'}
        names = {'pvcs': 'persistentvolumeclaims'}
        path = paths[resource] + '/namespaces/' + namespace + '/' + names.get(resource, resource) + '?limit=500'
    command = ['kubectl', '--context', context, 'get', '--raw', path]
    try:
        result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                check=True, timeout=30)
        data = json.loads(result.stdout)
        if resource != 'namespace':
            kinds = {'deployments': 'Deployment', 'statefulsets': 'StatefulSet', 'pods': 'Pod',
                     'replicasets': 'ReplicaSet', 'pvcs': 'PersistentVolumeClaim', 'jobs': 'Job'}
            if not isinstance(data, dict) or data.get('kind') != kinds[resource] + 'List':
                raise ValueError('unexpected list kind')
            for row in data.get('items', []):
                if not isinstance(row, dict) or row.get('kind', kinds[resource]) != kinds[resource]:
                    raise ValueError('unexpected list kind')
                row['kind'] = kinds[resource]
        return data
    except (OSError, subprocess.SubprocessError, ValueError) as exc:
        raise ValueError('Kubernetes read failed') from exc


def _metadata(obj):
    meta = obj.get('metadata')
    if not isinstance(meta, dict) or any(not isinstance(meta.get(k), str) or not meta[k]
                                          for k in ('name', 'uid', 'resourceVersion')):
        raise ValueError('invalid object identity')
    return meta


def _items(doc):
    if not isinstance(doc, dict) or not isinstance(doc.get('items'), list):
        raise ValueError('invalid Kubernetes list')
    if doc.get('metadata', {}).get('continue'):
        raise ValueError('Kubernetes list exceeds bounded page')
    rows = doc['items']
    seen, uids = set(), set()
    for row in rows:
        if not isinstance(row, dict):
            raise ValueError('invalid object')
        meta = _metadata(row)
        key = (row.get('kind'), meta['name'])
        if key in seen or meta['uid'] in uids:
            raise ValueError('duplicate identity')
        seen.add(key); uids.add(meta['uid'])
    return rows


def _fingerprint(data):
    namespace = _metadata(data['namespace'])
    return ((namespace['name'], namespace['uid'], namespace['resourceVersion']), *(
        tuple(sorted((x.get('kind'), _metadata(x)['name'], _metadata(x)['uid'],
                      _metadata(x)['resourceVersion']) for x in _items(data[name])))
        for name in RESOURCES[1:]))


def _list_version(doc):
    version = doc.get('metadata', {}).get('resourceVersion') if isinstance(doc, dict) else None
    if not isinstance(version, str) or not version:
        raise ValueError('list resourceVersion missing')
    return version


def _owner(obj):
    refs = _metadata(obj).get('ownerReferences', [])
    if not isinstance(refs, list):
        raise ValueError('invalid controller')
    refs = [r for r in refs if isinstance(r, dict) and r.get('controller') is True]
    if len(refs) != 1 or any(not isinstance(refs[0].get(k), str) or not refs[0][k]
                              for k in ('kind', 'name', 'uid')):
        raise ValueError('unknown controller')
    return refs[0]


def _project(data):
    if _metadata(data['namespace']).get('deletionTimestamp') is not None:
        raise ValueError('namespace terminating')
    deployments, statefulsets, pods, replicasets, pvcs, jobs = (_items(data[k]) for k in RESOURCES[1:])
    workloads = deployments + statefulsets
    if any(x.get('kind') != 'Deployment' for x in deployments) or any(x.get('kind') != 'StatefulSet' for x in statefulsets):
        raise ValueError('workload list kind mismatch')
    by_workload, images, init_images, uids, hashes = {}, {}, {}, {}, {}
    for row in workloads:
        meta = _metadata(row); kind = row.get('kind'); name = meta['name']
        if kind not in ('Deployment', 'StatefulSet') or not re.fullmatch(r'voice-[a-z0-9-]+', name) or name in by_workload or meta.get('deletionTimestamp') is not None:
            raise ValueError('unexpected workload')
        spec = row.get('spec')
        if not isinstance(spec, dict):
            raise ValueError('missing workload spec')
        try:
            podspec = spec['template']['spec']
            containers = podspec['containers']
            inits = podspec.get('initContainers', [])
            if not isinstance(containers, list) or not containers or not isinstance(inits, list):
                raise ValueError('invalid containers')
            for target, entries in ((images, containers), (init_images, inits)):
                for entry in entries:
                    cname, image = entry['name'], entry['image']
                    if (not isinstance(cname, str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9]*[a-z0-9])?', cname) or
                            not isinstance(image, str) or not (re.fullmatch(r'[^\s@]+@sha256:[0-9a-f]{64}', image) or
                            (name == 'voice-dev-mail' and cname == 'mail' and image == 'node:24-alpine'))):
                        raise ValueError('invalid image')
                    key = name + '/' + cname
                    if key in target:
                        raise ValueError('duplicate container')
                    target[key] = image
        except (KeyError, TypeError) as exc:
            raise ValueError('invalid pod template') from exc
        key = kind + '/' + name
        by_workload[name] = (kind, meta['uid'])
        uids[key] = meta['uid']
        hashes[key] = hashlib.sha256(json.dumps(spec, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
    if len(uids) != 31 or len(set(uids.values())) != 31 or len(images) != 49 or images.get('voice-dev-mail/mail') != 'node:24-alpine':
        raise ValueError('unexpected workload inventory')
    pvc_uids = {}
    for row in pvcs:
        meta = _metadata(row)
        if row.get('kind') != 'PersistentVolumeClaim' or meta['name'] not in PVCS or meta.get('deletionTimestamp') is not None or row.get('status', {}).get('phase') != 'Bound':
            raise ValueError('unexpected or unbound PVC')
        pvc_uids[meta['name']] = meta['uid']
    if set(pvc_uids) != PVCS:
        raise ValueError('incomplete PVC inventory')
    rs_by_uid = {}
    for row in replicasets:
        meta = _metadata(row); owner = _owner(row)
        if row.get('kind') != 'ReplicaSet' or meta.get('deletionTimestamp') is not None or by_workload.get(owner['name']) != ('Deployment', owner['uid']) or owner['kind'] != 'Deployment':
            raise ValueError('unrecognized ReplicaSet')
        rs_by_uid[meta['uid']] = (meta['name'], owner['name'], owner['uid'])
    jobs_by_uid = {}
    for row in jobs:
        meta = _metadata(row); job_status = row.get('status', {})
        conditions = job_status.get('conditions', []) if isinstance(job_status, dict) else []
        terminal = [x for x in conditions if isinstance(x, dict) and x.get('status') == 'True' and x.get('type') in ('Complete', 'Failed')]
        if (row.get('kind') != 'Job' or meta.get('deletionTimestamp') is not None or
                type(job_status.get('active', 0)) is not int or job_status.get('active', 0) != 0 or len(terminal) != 1):
            raise ValueError('active or unknown Job')
        jobs_by_uid[meta['uid']] = meta['name']
    mail_pods = []
    for row in pods:
        meta = _metadata(row); owner = _owner(row)
        if row.get('kind') != 'Pod':
            raise ValueError('unexpected pod')
        if owner['kind'] == 'ReplicaSet':
            chain = rs_by_uid.get(owner['uid'])
            if not chain or chain[0] != owner['name']:
                raise ValueError('unknown pod controller')
            workload_name = chain[1]
        elif owner['kind'] == 'StatefulSet' and by_workload.get(owner['name']) == ('StatefulSet', owner['uid']):
            workload_name = owner['name']
        elif owner['kind'] == 'Job' and jobs_by_uid.get(owner['uid']) == owner['name']:
            if meta.get('deletionTimestamp') is not None or row.get('status', {}).get('phase') not in ('Succeeded', 'Failed'):
                raise ValueError('active or unknown Job pod')
            continue
        else:
            raise ValueError('unknown pod controller')
        if workload_name == 'voice-dev-mail' or meta['name'].startswith('voice-dev-mail-'):
            if workload_name != 'voice-dev-mail' or owner['kind'] != 'ReplicaSet':
                raise ValueError('mail owner mismatch')
            mail_pods.append((row, owner, chain))
    if len(mail_pods) != 1:
        raise ValueError('mail pod count invalid')
    pod, owner, chain = mail_pods[0]; pm = _metadata(pod)
    statuses = pod.get('status', {}).get('containerStatuses', [])
    if pm.get('deletionTimestamp') is not None or pod.get('status', {}).get('phase') != 'Running' or not isinstance(statuses, list) or len(statuses) != 1 or statuses[0].get('name') != 'mail':
        raise ValueError('mail pod not stable')
    status = statuses[0]; node = pod.get('spec', {}).get('nodeName')
    cid, iid, restarts = status.get('containerID'), status.get('imageID'), status.get('restartCount')
    if (not isinstance(node, str) or not node or not isinstance(cid, str) or not re.fullmatch(r'containerd://[0-9a-f]{64}', cid) or
            not isinstance(iid, str) or not re.fullmatch(r'docker\.io/library/node@sha256:[0-9a-f]{64}', iid) or
            type(restarts) is not int or restarts < 0 or status.get('ready') is not True or status.get('started') is not True):
        raise ValueError('mail runtime invalid')
    mail = {'pod_uid': pm['uid'], 'replicaset_uid': owner['uid'], 'deployment_uid': chain[2],
            'node': node, 'container_id': cid, 'image_id': iid, 'restart_count': restarts,
            'ready': True, 'started': True, 'pod_count': 1, 'terminating_count': 0,
            'owner_chain_verified': True}
    return {'namespace_uid': _metadata(data['namespace'])['uid'], 'images': images, 'init_images': init_images,
            'pvc_uids': pvc_uids, 'workload_uids': uids, 'workload_spec_sha256': hashes, 'mail_runtime': mail}


def collect(context, namespace, retries=3, reader=None):
    if not isinstance(context, str) or not context.strip() or context.startswith('-') or namespace != 'voice-dev' or type(retries) is not int or not 1 <= retries <= 5:
        raise ValueError('explicit context and DEV namespace required')
    reader = reader or _read
    for _ in range(retries):
        first = {name: reader(context, namespace, name) for name in RESOURCES}
        second = {name: reader(context, namespace, name) for name in RESOURCES}
        for name in RESOURCES[1:]:
            _list_version(first[name]); _list_version(second[name])
        if _fingerprint(first) == _fingerprint(second):
            if _metadata(second['namespace'])['name'] != namespace:
                raise ValueError('namespace identity mismatch')
            projected = _project(second)
            if _project(first) == projected:
                return projected
    raise ValueError('Kubernetes snapshot changed')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--context', required=True)
    parser.add_argument('--namespace', required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(collect(args.context, args.namespace), sort_keys=True, separators=(',', ':')))
    except Exception:
        parser.exit(1, 'baseline collection failed\n')


if __name__ == '__main__':
    main()
