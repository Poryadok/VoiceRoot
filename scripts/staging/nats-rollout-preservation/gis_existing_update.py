"""Identity-bound GIS lifecycle primitives for a protected existing deployment."""
import copy
import re

import gis_first_install as gis
import guard
from root_main import private_json


def load_authority(kube):
    """Resolve only the root-owned active reference and its original PASS."""
    try:
        active = private_json(guard.ROOT / 'installed' / 'active-release.json')
    except FileNotFoundError:
        active = {}
    if not isinstance(active, dict):
        raise gis.Blocked('gis_existing_active_reference_invalid')
    pair = gis._read_pair(kube, None)
    reference = active.get('gis_first_install_transition')
    if reference is None:
        if any(pair.values()):
            raise gis.Blocked('gis_existing_anchor_missing')
        return None
    if ('gis_latest_transition' in active or not isinstance(reference, dict)
            or set(reference) != {'operation', 'receipt_sha256'}
            or not isinstance(reference.get('operation'), str)
            or not re.fullmatch('[a-f0-9]{12}', reference['operation'])
            or not isinstance(reference.get('receipt_sha256'), str)
            or not re.fullmatch('[a-f0-9]{64}', reference['receipt_sha256'])):
        raise gis.Blocked('gis_existing_active_reference_invalid')
    try:
        state = private_json(guard.ROOT / ('rollout-' + reference['operation']) / 'checkpoint.json', limit=128 << 20)
    except FileNotFoundError:
        raise gis.Blocked('gis_existing_anchor_missing') from None
    receipt = state.get('gis_first_install_transition_receipt') if isinstance(state, dict) else None
    if not isinstance(receipt, dict) or receipt.get('receipt_sha256') != reference['receipt_sha256']:
        raise gis.Blocked('gis_existing_anchor_changed')
    return enroll_first_install(receipt, state, kube)


def seal(value):
    result = copy.deepcopy(value)
    result.pop('sha256', None)
    result['sha256'] = gis.digest(result)
    return result


def validate(binding):
    fields = {'schema', 'anchor_sha256', 'namespace_uid', 'operation', 'service_uid',
              'deployment_uid', 'service', 'old', 'target', 'target_service', 'sha256'}
    uid = r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}'
    if (type(binding) is not dict or set(binding) != fields
            or binding.get('schema') != 'voice-gis-existing-update-v1'
            or any(not isinstance(binding.get(key), str) or not re.fullmatch(uid, binding[key])
                   for key in ('namespace_uid', 'service_uid', 'deployment_uid'))
            or not isinstance(binding.get('operation'), str) or not re.fullmatch('[a-f0-9]{12}', binding['operation'])
            or not isinstance(binding.get('anchor_sha256'), str) or not re.fullmatch('[a-f0-9]{64}', binding['anchor_sha256'])
            or not isinstance(binding.get('service'), dict) or not isinstance(binding.get('old'), dict)
            or binding.get('target') is not None and not isinstance(binding['target'], dict)
            or binding.get('target_service') is not None and not isinstance(binding['target_service'], dict)
            or binding.get('sha256') != gis.digest({key: value for key, value in binding.items() if key != 'sha256'})):
        raise gis.Blocked('gis_existing_binding_invalid')


def enroll_first_install(receipt, state, kube):
    """Caller must load receipt/state from the root-owned active reference."""
    gis.validate_next_operation(receipt, state, kube)
    pair = gis._read_pair(kube, None)
    service, deployment = pair[gis.SERVICE], pair[gis.DEPLOYMENT]
    namespace = kube.get('namespace', gis.NS)
    if (namespace.get('metadata', {}).get('deletionTimestamp')
            or namespace.get('metadata', {}).get('uid') != receipt['namespace_uid']):
        raise gis.Blocked('gis_existing_identity_changed')
    for key, row in (('Service/voice-gameintegration', service),
                     ('Deployment/voice-gameintegration', deployment)):
        expected = receipt['resource_receipt']['objects'][key]
        if (row is None or row.get('metadata', {}).get('deletionTimestamp')
                or row.get('metadata', {}).get('uid') != expected['uid']
                or gis.digest(gis.semantic(row)) != expected['spec_sha256']
                or row.get('metadata', {}).get('annotations', {}).get(gis.MARKER) != receipt['operation']):
            raise gis.Blocked('gis_existing_anchor_changed')
    return seal({'schema': 'voice-gis-existing-update-v1',
                 'anchor_sha256': receipt['receipt_sha256'],
                 'namespace_uid': receipt['namespace_uid'],
                 'operation': receipt['operation'],
                 'service_uid': service['metadata']['uid'],
                 'deployment_uid': deployment['metadata']['uid'],
                 'service': gis.semantic(service), 'old': gis.semantic(deployment),
                 'target': None, 'target_service': None})


def bind_target(binding, target):
    validate(binding)
    expected = gis.semantic(target)
    if (target.get('metadata', {}).get('uid') != binding['deployment_uid']
            or expected['metadata'] != binding['old']['metadata']
            or expected['spec'].get('replicas', 1) != 1):
        raise gis.Blocked('gis_existing_target_invalid')
    candidate = copy.deepcopy(expected)
    old = binding['old']
    current = candidate['spec']['template']['spec']['containers']
    prior = old['spec']['template']['spec']['containers']
    if len(current) != len(prior):
        raise gis.Blocked('gis_existing_target_invalid')
    selected = [i for i, container in enumerate(current) if container.get('name') == 'gameintegration']
    if (len(selected) != 1 or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/gameintegration@sha256:[a-f0-9]{64}',
                                                current[selected[0]].get('image', ''))):
        raise gis.Blocked('gis_existing_target_invalid')
    current[selected[0]]['image'] = prior[selected[0]]['image']
    if candidate != old:
        raise gis.Blocked('gis_existing_target_invalid')
    value = copy.deepcopy(binding)
    value['target'] = expected
    value['target_service'] = copy.deepcopy(binding['service'])
    return seal(value)


def bind_normalized_target(binding, deployment, service, selected, mode, expected_hash):
    """Bind the root-normalized, TT-target-hashed pair, never caller documents."""
    validate(binding)
    if (type(selected) is not bool or mode not in ('images-only', 'full', 'app-only')
            or not isinstance(expected_hash, str) or not re.fullmatch('[a-f0-9]{64}', expected_hash)
            or not isinstance(deployment, dict) or not isinstance(service, dict)):
        raise gis.Blocked('gis_existing_target_invalid')
    pair = {'deployment_mode': 'selected-paused' if selected else 'preserved-active',
            'deployment_uid': deployment.get('metadata', {}).get('uid'),
            'service_uid': service.get('metadata', {}).get('uid'),
            'deployment': gis.semantic(deployment), 'service': gis.semantic(service)}
    if (gis.digest(pair) != expected_hash or pair['deployment_uid'] != binding['deployment_uid']
            or pair['service_uid'] != binding['service_uid']
            or gis._identity(deployment) != gis.DEPLOYMENT or gis._identity(service) != gis.SERVICE
            or any(row.get('metadata', {}).get('namespace') != gis.NS
                   or row.get('metadata', {}).get('annotations', {}).get(gis.MARKER) != binding['operation']
                   or row.get('metadata', {}).get('deletionTimestamp') for row in (deployment, service))
            or deployment.get('spec', {}).get('replicas', 1) != (0 if selected else 1)):
        raise gis.Blocked('gis_existing_target_invalid')
    ready = copy.deepcopy(pair['deployment'])
    ready['spec']['replicas'] = 1
    if mode == 'images-only':
        if pair['service'] != binding['service']:
            raise gis.Blocked('gis_existing_target_invalid')
        candidate = copy.deepcopy(ready)
        old = copy.deepcopy(binding['old'])
        managed = 'kubectl.kubernetes.io/last-applied-configuration'
        candidate['metadata']['annotations'].pop(managed, None)
        old['metadata']['annotations'].pop(managed, None)
        current = candidate['spec']['template']['spec']['containers']
        prior = old['spec']['template']['spec']['containers']
        indexes = [i for i, c in enumerate(current) if c.get('name') == 'gameintegration']
        if len(current) != len(prior) or len(indexes) != 1:
            raise gis.Blocked('gis_existing_target_invalid')
        index = indexes[0]
        if not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/gameintegration@sha256:[a-f0-9]{64}', current[index].get('image', '')):
            raise gis.Blocked('gis_existing_target_invalid')
        current[index]['image'] = prior[index]['image']
        if candidate != old or not selected and ready != binding['old']:
            raise gis.Blocked('gis_existing_target_invalid')
    value = copy.deepcopy(binding)
    value['target'] = ready
    value['target_service'] = pair['service']
    return seal(value)


def verify_phase(binding, kube, phase):
    validate(binding)
    if phase not in ('active', 'paused-old', 'paused-target', 'ready-target'):
        raise gis.Blocked('gis_existing_phase_invalid')
    namespace = kube.get('namespace', gis.NS)
    service = gis._read_one(kube, *gis.SERVICE)
    deployment = gis._read_one(kube, *gis.DEPLOYMENT)
    if (namespace['metadata']['uid'] != binding['namespace_uid']
            or any(row.get('metadata', {}).get('deletionTimestamp') for row in (namespace, service, deployment))
            or service['metadata']['uid'] != binding['service_uid']
            or deployment['metadata']['uid'] != binding['deployment_uid']
            or gis.semantic(service) != (binding['target_service'] if phase in ('paused-target','ready-target') else binding['service'])):
        raise gis.Blocked('gis_existing_identity_changed')
    expected = copy.deepcopy(binding['target'] if phase in ('paused-target', 'ready-target') else binding['old'])
    if not isinstance(expected, dict):
        raise gis.Blocked('gis_existing_target_invalid')
    if phase in ('paused-old', 'paused-target'):
        expected['spec']['replicas'] = 0
    actual = gis.semantic(deployment)
    if actual != expected:
        # The Deployment controller alone advances this one annotation after
        # creating the owned ReplicaSet; never discard other metadata drift.
        if phase not in ('paused-target', 'ready-target'):
            raise gis.Blocked('gis_existing_deployment_changed')
        revision_key = 'deployment.kubernetes.io/revision'
        actual_copy, expected_copy = copy.deepcopy(actual), copy.deepcopy(expected)
        actual_revision = actual_copy['metadata']['annotations'].pop(revision_key, None)
        previous_revision = expected_copy['metadata']['annotations'].pop(revision_key, None)
        if (actual_copy != expected_copy or not isinstance(actual_revision, str)
                or not re.fullmatch(r'[1-9][0-9]{0,18}', actual_revision)
                or previous_revision is not None and (not isinstance(previous_revision, str)
                    or not re.fullmatch(r'[1-9][0-9]{0,18}', previous_revision)
                    or int(actual_revision) < int(previous_revision))):
            raise gis.Blocked('gis_existing_deployment_changed')
        rows = kube.run(['get', 'replicasets', '-o', 'json']).get('items')
        if not isinstance(rows, list) or len(rows) > 2048:
            raise gis.Blocked('gis_existing_ready_inventory_invalid')
        from runtime_stage import revision_template
        matching = [row for row in rows if row.get('metadata', {}).get('annotations', {}).get(revision_key) == actual_revision
                    and not row.get('metadata', {}).get('deletionTimestamp')
                    and any(owner.get('kind') == 'Deployment' and owner.get('uid') == binding['deployment_uid']
                            and owner.get('controller') is True for owner in row.get('metadata', {}).get('ownerReferences', []))
                    and revision_template(row.get('spec', {}).get('template', {})) == revision_template(expected['spec']['template'])]
        if len(matching) != 1:
            raise gis.Blocked('gis_existing_revision_unproved')
    return True


def service_account_preflight(kube, template):
    """Reject unsupported effective Pod admission before any fence."""
    if not isinstance(template, dict) or (template.get('automountServiceAccountToken') is not None
                                          and template['automountServiceAccountToken'] is not True):
        raise gis.Blocked('gis_existing_service_account_unsupported')
    name = template.get('serviceAccountName') or 'default'
    if not isinstance(name, str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]{0,251}[a-z0-9])?', name):
        raise gis.Blocked('gis_existing_service_account_invalid')
    account = kube.get('serviceaccount', name)
    metadata = account.get('metadata', {}) if isinstance(account, dict) else {}
    uid = metadata.get('uid')
    rv = metadata.get('resourceVersion')
    if (metadata.get('name') != name or metadata.get('namespace') != gis.NS
            or metadata.get('deletionTimestamp') or not isinstance(uid, str)
            or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', uid)
            or not isinstance(rv, str) or not re.fullmatch('[0-9]+', rv)
            or account.get('automountServiceAccountToken') is not None and account['automountServiceAccountToken'] is not True
            or not template.get('imagePullSecrets') and account.get('imagePullSecrets')):
        raise gis.Blocked('gis_existing_service_account_unsupported')
    return {'name': name, 'uid': uid, 'resourceVersion': rv}


def verify_pod_spec(template, pod):
    """Allow only the standard ServiceAccount injection and fixed Pod defaults."""
    wanted = copy.deepcopy(template)
    actual = copy.deepcopy(pod)
    if not isinstance(wanted, dict) or not isinstance(actual, dict):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    volumes = actual.get('volumes', [])
    expected_volumes = wanted.get('volumes', [])
    if not isinstance(volumes, list) or not isinstance(expected_volumes, list):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    extra = [v for v in volumes if v not in expected_volumes]
    if len(extra) != 1 or not isinstance(extra[0], dict):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    volume = extra[0]
    name = volume.get('name', '')
    if not isinstance(name, str) or not re.fullmatch(r'kube-api-access-[a-z0-9]{5}', name):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    projected = volume.get('projected', {})
    sources = projected.get('sources') if isinstance(projected, dict) else None
    if (set(volume) != {'name', 'projected'} or not isinstance(projected, dict)
            or set(projected) != {'defaultMode', 'sources'}
            or projected['defaultMode'] != 420 or not isinstance(sources, list) or len(sources) != 3):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    token = sources[0].get('serviceAccountToken', {}) if isinstance(sources[0], dict) else {}
    if (not isinstance(sources[0], dict) or set(sources[0]) != {'serviceAccountToken'}
            or not isinstance(token, dict) or set(token) != {'path', 'expirationSeconds'}
            or token['path'] != 'token' or type(token['expirationSeconds']) is not int
            or not 600 <= token['expirationSeconds'] <= 86400
            or sources[1] != {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}}
            or sources[2] != {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    volumes.remove(volume)
    if volumes != expected_volumes:
        raise gis.Blocked('gis_existing_ready_pod_changed')
    if 'volumes' not in wanted:
        actual.pop('volumes', None)
    for field in ('containers', 'initContainers'):
        rows = actual.get(field, [])
        if not isinstance(rows, list): raise gis.Blocked('gis_existing_ready_pod_changed')
        for row in rows:
            mounts = row.get('volumeMounts', [])
            mount = {'name': name, 'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True}
            if not isinstance(mounts, list) or mounts.count(mount) != 1:
                raise gis.Blocked('gis_existing_ready_pod_changed')
            mounts.remove(mount)
            if not mounts: row.pop('volumeMounts', None)
    defaults = {'serviceAccountName': wanted.get('serviceAccountName') or 'default',
                'serviceAccount': wanted.get('serviceAccountName') or 'default',
                'schedulerName': 'default-scheduler', 'restartPolicy': 'Always',
                'dnsPolicy': 'ClusterFirst', 'enableServiceLinks': True,
                'priority': 0, 'preemptionPolicy': 'PreemptLowerPriority',
                'terminationGracePeriodSeconds': 30, 'hostNetwork': False,
                'hostPID': False, 'hostIPC': False, 'securityContext': {},
                'tolerations': [
                    {'key': 'node.kubernetes.io/not-ready', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300},
                    {'key': 'node.kubernetes.io/unreachable', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300}]}
    for key in set(actual) - set(wanted):
        if key == 'nodeName' and isinstance(actual[key], str) and re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', actual[key]):
            continue
        if key not in defaults or type(actual[key]) is not type(defaults[key]) or actual[key] != defaults[key]:
            raise gis.Blocked('gis_existing_ready_pod_changed')
    if any(actual.get(key) != value for key, value in wanted.items()):
        raise gis.Blocked('gis_existing_ready_pod_changed')


def verify_ready(binding, kube, service_account):
    """Prove the endpoint belongs to the captured UID and exact target image."""
    verify_phase(binding, kube, 'ready-target')
    if service_account_preflight(kube, binding['target']['spec']['template']['spec']) != service_account:
        raise gis.Blocked('gis_existing_service_account_changed')
    pair = gis._read_pair(kube, None)
    service, deployment = pair[gis.SERVICE], pair[gis.DEPLOYMENT]
    status = deployment.get('status', {})
    generation = deployment.get('metadata', {}).get('generation')
    if (type(generation) is not int or status.get('observedGeneration', 0) < generation
            or any(status.get(key) != 1 for key in ('readyReplicas', 'updatedReplicas', 'availableReplicas'))):
        raise gis.Blocked('gis_existing_not_ready')
    endpoints = gis._ready_service_endpoints(kube, service, binding['deployment_uid'])
    if not endpoints:
        raise gis.Blocked('gis_existing_endpoint_missing')
    slices = kube.run(['get', 'endpointslices', '-l',
                       'kubernetes.io/service-name=voice-gameintegration', '-o', 'json']).get('items')
    replicasets = kube.run(['get', 'replicasets', '-o', 'json']).get('items')
    if not isinstance(slices, list) or len(slices) > 64 or not isinstance(replicasets, list) or len(replicasets) > 2048:
        raise gis.Blocked('gis_existing_ready_inventory_invalid')
    from runtime_stage import revision_template
    template = binding['target']['spec']['template']
    observed = []
    for endpoint in endpoints:
        matches = [rs for rs in replicasets if rs.get('metadata', {}).get('uid') == endpoint['replicaset_uid']]
        if (len(matches) != 1 or not any(owner.get('kind') == 'Deployment' and owner.get('uid') == binding['deployment_uid']
                and owner.get('controller') is True for owner in matches[0].get('metadata', {}).get('ownerReferences', []))
                or revision_template(matches[0].get('spec', {}).get('template', {})) != revision_template(template)):
            raise gis.Blocked('gis_existing_ready_revision_changed')
        refs = [item.get('targetRef', {}) for row in slices if row.get('metadata', {}).get('uid') == endpoint['slice_uid']
                for item in row.get('endpoints', []) if item.get('conditions', {}).get('ready') is True
                and item.get('targetRef', {}).get('uid') == endpoint['pod_uid']]
        if len(refs) != 1:
            raise gis.Blocked('gis_existing_ready_endpoint_changed')
        pod = kube.get('pod', refs[0]['name'])
        if (pod.get('metadata', {}).get('uid') != endpoint['pod_uid']
                or pod.get('metadata', {}).get('deletionTimestamp')
                or pod.get('status', {}).get('phase') != 'Running'
                or not any(owner.get('kind') == 'ReplicaSet' and owner.get('uid') == endpoint['replicaset_uid']
                           and owner.get('controller') is True for owner in pod.get('metadata', {}).get('ownerReferences', []))):
            raise gis.Blocked('gis_existing_ready_pod_changed')
        slice_rows = [row for row in slices if row.get('metadata', {}).get('uid') == endpoint['slice_uid']]
        versions = [pod.get('metadata', {}).get('resourceVersion'), matches[0].get('metadata', {}).get('resourceVersion'),
                    slice_rows[0].get('metadata', {}).get('resourceVersion') if len(slice_rows) == 1 else None]
        if any(not isinstance(value, str) or not re.fullmatch(r'[0-9]+', value) for value in versions):
            raise gis.Blocked('gis_existing_ready_version_missing')
        observed.append((refs[0]['name'], endpoint, versions))
        pod_spec = pod.get('spec', {})
        verify_pod_spec(template['spec'], pod_spec)
        for spec_key, status_key in (('containers', 'containerStatuses'), ('initContainers', 'initContainerStatuses')):
            expected = template['spec'].get(spec_key, [])
            actual = pod.get('spec', {}).get(spec_key, [])
            states = pod.get('status', {}).get(status_key, [])
            if ({c.get('name'): c.get('image') for c in actual} != {c.get('name'): c.get('image') for c in expected}
                    or len(actual) != len(expected) or len(states) != len(expected)):
                raise gis.Blocked('gis_existing_ready_pod_changed')
            for container in expected:
                matched = [row for row in states if row.get('name') == container['name']]
                if (len(matched) != 1 or spec_key == 'containers' and matched[0].get('ready') is not True
                        or matched[0].get('imageID', '').removeprefix('docker-pullable://') != container['image']):
                    raise gis.Blocked('gis_existing_ready_image_changed')
    verify_phase(binding, kube, 'ready-target')
    if service_account_preflight(kube, binding['target']['spec']['template']['spec']) != service_account:
        raise gis.Blocked('gis_existing_service_account_changed')
    if gis._ready_service_endpoints(kube, service, binding['deployment_uid']) != endpoints:
        raise gis.Blocked('gis_existing_ready_endpoint_changed')
    current_slices = kube.run(['get', 'endpointslices', '-l',
                               'kubernetes.io/service-name=voice-gameintegration', '-o', 'json']).get('items')
    current_rs = kube.run(['get', 'replicasets', '-o', 'json']).get('items')
    if not isinstance(current_slices, list) or len(current_slices) > 64 or not isinstance(current_rs, list) or len(current_rs) > 2048:
        raise gis.Blocked('gis_existing_ready_inventory_invalid')
    for pod_name, endpoint, versions in observed:
        pod = kube.get('pod', pod_name)
        rs = [row for row in current_rs if row.get('metadata', {}).get('uid') == endpoint['replicaset_uid']]
        slices = [row for row in current_slices if row.get('metadata', {}).get('uid') == endpoint['slice_uid']]
        if (pod.get('metadata', {}).get('uid') != endpoint['pod_uid'] or len(rs) != 1 or len(slices) != 1
                or [pod.get('metadata', {}).get('resourceVersion'), rs[0]['metadata'].get('resourceVersion'),
                    slices[0]['metadata'].get('resourceVersion')] != versions):
            raise gis.Blocked('gis_existing_ready_observation_changed')
    return {'deployment_uid': binding['deployment_uid'], 'service_uid': binding['service_uid'],
            'target_sha256': gis.digest(binding['target']), 'endpoints': endpoints}
