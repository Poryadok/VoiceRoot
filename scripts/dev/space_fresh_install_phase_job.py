"""Identity checks for the one source-rendered Space native Job and its Pod."""
import copy
import ipaddress
import re

import auth_user_candidate_probe as admission


class JobError(RuntimeError):
    pass


def job(authored, actual):
    try:
        meta = actual['metadata']; expected = authored['metadata']
        if (meta.get('name') != expected['name'] or meta.get('namespace') != 'voice-dev' or
                meta.get('deletionTimestamp') or meta.get('ownerReferences') or meta.get('finalizers') or
                not isinstance(meta.get('uid'), str) or not meta['uid'] or
                not isinstance(meta.get('resourceVersion'), str) or not meta['resourceVersion'] or
                meta.get('annotations') != expected['annotations']): raise ValueError()
        return admission.admitted_job(authored, actual)
    except BaseException:
        raise JobError('Space installer Job identity unavailable') from None


def pod(authored, actual_job, actual, platform_digest):
    try:
        admitted = job(authored, actual_job)
        meta, spec, status = actual['metadata'], actual['spec'], actual['status']
        owners = [x for x in meta.get('ownerReferences', []) if x.get('controller') is True]
        uid = actual_job['metadata']['uid']; name = actual_job['metadata']['name']
        if (actual.get('kind') != 'Pod' or actual.get('apiVersion') != 'v1' or meta.get('namespace') != 'voice-dev' or
                meta.get('deletionTimestamp') or not all(isinstance(meta.get(k), str) and meta[k] for k in ('name','uid','resourceVersion')) or
                len(owners) != 1 or owners[0].get('kind') != 'Job' or owners[0].get('uid') != uid or
                owners[0].get('name') != name or spec.get('hostNetwork', False) is not False): raise ValueError()
        if meta.get('labels') != admitted['template']['metadata']['labels']: raise ValueError()
        generated = {'batch.kubernetes.io/controller-uid':uid, 'controller-uid':uid,
                     'batch.kubernetes.io/job-name':name, 'job-name':name}
        labels = copy.deepcopy(meta['labels'])
        for key, value in generated.items():
            if key in labels and labels.pop(key) != value: raise ValueError()
        template = authored['spec']['template']
        if labels != template['metadata']['labels']: raise ValueError()
        intended = {'apiVersion':'v1', 'kind':'Pod', 'metadata':{'name':'template','namespace':'voice-dev',
                    'labels':template['metadata']['labels']}, 'spec':template['spec']}
        observed = copy.deepcopy(intended); observed['spec'] = spec
        admission.admitted_pod(intended, observed)
        containers = status.get('containerStatuses', [])
        if (not re.fullmatch(r'sha256:[0-9a-f]{64}', platform_digest) or status.get('phase') not in ('Running','Succeeded') or
                not isinstance(status.get('podIP'), str) or str(ipaddress.ip_address(status['podIP'])) != status['podIP'] or
                len(containers) != 1 or containers[0].get('name') != 'migrate' or
                not isinstance(containers[0].get('containerID'), str) or not containers[0]['containerID'] or
                type(containers[0].get('restartCount')) is not int or containers[0]['restartCount'] != 0 or
                not re.fullmatch(r'(?:docker-pullable://[^@]+@|containerd://|docker://)?' + platform_digest,
                                 containers[0].get('imageID', ''))): raise ValueError()
        return {'uid':meta['uid'], 'resourceVersion':meta['resourceVersion'], 'name':meta['name'],
                'pod_ip':status['podIP'], 'container_id':containers[0]['containerID']}
    except BaseException:
        raise JobError('Space installer Pod identity unavailable') from None
