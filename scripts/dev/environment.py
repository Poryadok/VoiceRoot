#!/usr/bin/env python3
"""Render an isolated dev namespace from an explicitly captured image inventory.

Never copy Secrets or application data. Workloads render paused; the operator
must provision independent credentials, initialize databases, then start dev.
"""
import argparse
import copy
import json
from pathlib import Path
import re

NS = 'voice-dev'
DOMAINS = {'voice.comrade.click': 'api-dev.comrade.click',
           'app.comrade.click': 'dev.comrade.click',
           'developers.comrade.click': 'developers-dev.comrade.click',
           'admin.comrade.click': 'admin-dev.comrade.click',
           'livekit.comrade.click': 'livekit-dev.comrade.click'}
LABEL = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}
ALLOWED = {'Deployment', 'StatefulSet', 'Service', 'ConfigMap', 'Ingress', 'Job', 'PersistentVolumeClaim'}


def target_args():
    return ['--namespace', NS]


def validate_target(namespace):
    meta = namespace.get('metadata', {})
    if meta.get('name') != NS or any(meta.get('labels', {}).get(k) != v for k, v in LABEL.items()):
        raise ValueError('dev namespace ownership marker required')


def metadata(name):
    return {'name': name, 'namespace': NS, 'labels': dict(LABEL)}


def foundation(node):
    if not re.fullmatch(r'[a-z0-9][a-z0-9.-]*', node):
        raise ValueError('invalid node')
    objects = [{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': NS, 'labels': dict(LABEL)}},
               {'apiVersion': 'v1', 'kind': 'LimitRange', 'metadata': metadata('dev-defaults'),
                'spec': {'limits': [{'type': 'Container', 'default': {'cpu': '500m', 'memory': '512Mi'},
                                     'defaultRequest': {'cpu': '20m', 'memory': '64Mi'}}]}},
               {'apiVersion': 'v1', 'kind': 'ResourceQuota', 'metadata': metadata('dev-budget'),
                'spec': {'hard': {'requests.cpu': '6', 'limits.cpu': '40', 'requests.memory': '8Gi',
                                  'limits.memory': '20Gi', 'pods': '60', 'requests.storage': '20Gi'}}},
               {'apiVersion': 'networking.k8s.io/v1', 'kind': 'NetworkPolicy',
                'metadata': metadata('dev-isolation'), 'spec': {
                    'podSelector': {}, 'policyTypes': ['Ingress', 'Egress'],
                    'ingress': [{'from': [{'podSelector': {}}]},
                                {'from': [{'namespaceSelector': {'matchLabels': {'kubernetes.io/metadata.name': 'traefik'}}}],
                                 'ports': [{'port': p, 'protocol': 'TCP'} for p in (80, 8080, 7880)]}],
                    'egress': [{'to': [{'podSelector': {}}]},
                               {'to': [{'namespaceSelector': {'matchLabels': {'kubernetes.io/metadata.name': 'kube-system'}},
                                        'podSelector': {'matchLabels': {'k8s-app': 'kube-dns'}}}],
                                'ports': [{'port': 53, 'protocol': p} for p in ('UDP', 'TCP')]},
                               {'to': [{'ipBlock': {'cidr': '0.0.0.0/0', 'except': ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16', '169.254.0.0/16', '127.0.0.0/8']}}],
                                'ports': [{'port': 443, 'protocol': 'TCP'}, {'port': 80, 'protocol': 'TCP'}]}]}}]
    for name, claim, size in (('postgres', 'pgdata-voice-postgres-0', '5Gi'),
                              ('minio', 'data-voice-minio-0', '5Gi'),
                              ('clickhouse', 'chdata-voice-clickhouse-0', '2Gi'),
                              ('nats', 'voice-nats-jsdata', '1Gi')):
        objects.append({'apiVersion': 'v1', 'kind': 'PersistentVolume',
                        'metadata': {'name': 'voice-dev-' + name, 'labels': dict(LABEL)},
                        'spec': {'capacity': {'storage': size}, 'accessModes': ['ReadWriteOnce'],
                                 'persistentVolumeReclaimPolicy': 'Retain', 'storageClassName': 'voice-dev-local',
                                 'claimRef': {'namespace': NS, 'name': claim},
                                 'local': {'path': '/home/voice-dev/data/' + name},
                                 'nodeAffinity': {'required': {'nodeSelectorTerms': [{'matchExpressions': [
                                     {'key': 'kubernetes.io/hostname', 'operator': 'In', 'values': [node]}]}]}}}})
    return objects


def rewrite(value):
    if isinstance(value, dict):
        return {k: rewrite(v) for k, v in value.items()}
    if isinstance(value, list):
        return [rewrite(v) for v in value]
    if isinstance(value, str):
        value = value.replace('voice-staging', NS)
        for old, new in DOMAINS.items():
            value = value.replace(old, new)
        value = re.sub(r'(voice-nats-(?:service-credentials|bootstrap-credentials|operator|hub-tls))-r[0-9a-z]+', r'\1', value)
        value = re.sub(r'voice-nats-bootstrap-contract-[a-f0-9]+', 'voice-nats-bootstrap-credentials', value)
        value = re.sub(r'voice-nats-jsdata-[a-z0-9]+', 'voice-nats-jsdata', value)
    return value


def walk(value):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def validate_references(objects, secrets):
    available = {}
    for secret in secrets:
        if secret.get('kind') != 'Secret' or secret.get('metadata', {}).get('namespace') != NS:
            raise ValueError('fresh Secret target invalid')
        available[secret['metadata']['name']] = set(secret.get('data', {})) | set(secret.get('stringData', {}))
    for obj in objects:
        for part in walk(obj):
            for field in ('secretKeyRef', 'secretRef', 'secret'):
                if field not in part:
                    continue
                ref = part[field]
                name = ref.get('name') or ref.get('secretName')
                if name not in available or ref.get('key') and ref['key'] not in available[name]:
                    raise ValueError('fresh secret reference unresolved: ' + str(name))
                if any(item['key'] not in available[name] for item in ref.get('items', [])):
                    raise ValueError('fresh secret projection key missing: ' + str(name))


def clean_meta(meta):
    for field in ('uid', 'resourceVersion', 'generation', 'creationTimestamp', 'managedFields', 'ownerReferences'):
        meta.pop(field, None)
    meta['annotations'] = {k: v for k, v in meta.get('annotations', {}).items()
                           if not k.startswith(('kubectl.', 'deployment.kubernetes.io/', 'voice.io/rollout', 'voice.io/nats-', 'pv.kubernetes.io/', 'volume.kubernetes.io/', 'volume.beta.kubernetes.io/'))}
    meta.setdefault('labels', {}).update(LABEL)


def separate_role_jwks_trust(obj):
    """Keep outbound issuer trust separate from Role's inbound client CA."""
    if obj.get('kind') != 'Deployment' or obj.get('metadata', {}).get('name') != 'voice-role':
        return
    containers = obj['spec']['template']['spec'].get('containers', [])
    role = next((container for container in containers if container.get('name') == 'role'), None)
    if role is None:
        raise ValueError('voice-role container missing')
    ca_file = next((env for env in role.get('env', []) if env.get('name') == 'S2S_JWKS_CA_FILE'), None)
    if ca_file is None or ca_file.get('value') != '/etc/voice/media/ca/ca.crt':
        raise ValueError('unexpected voice-role JWKS trust source')
    ca_file['value'] = '/etc/voice/media/ca/jwks-ca.crt'
    volume = next((volume for volume in obj['spec']['template']['spec'].get('volumes', [])
                   if volume.get('name') == 'media-ca'), None)
    if volume is None or volume.get('secret', {}).get('secretName') != 'voice-media-ca':
        raise ValueError('unexpected voice-role media CA volume')
    items = volume['secret'].setdefault('items', [])
    if not any(item.get('key') == 'jwks-ca.crt' and item.get('path') == 'jwks-ca.crt' for item in items):
        items.append({'key': 'jwks-ca.crt', 'path': 'jwks-ca.crt'})


def render(snapshot, node):
    output = []
    for original in snapshot:
        if original.get('kind') not in ALLOWED:
            raise ValueError('source kind not admitted')
        if original.get('metadata', {}).get('namespace') != 'voice-staging':
            raise ValueError('unexpected source namespace')
        obj = rewrite(copy.deepcopy(original))
        kind, meta = obj['kind'], obj['metadata']
        clean_meta(meta)
        obj.pop('status', None)
        separate_role_jwks_trust(obj)
        spec = obj.get('spec', {})
        if kind in ('Deployment', 'StatefulSet', 'Job'):
            pod = spec['template']['spec']
            clean_meta(spec['template'].setdefault('metadata', {}))
            if any(pod.get(k) for k in ('hostNetwork', 'hostPID', 'hostIPC', 'nodeName')) or any('hostPath' in v for v in pod.get('volumes', [])):
                raise ValueError('host access forbidden')
            pod['automountServiceAccountToken'] = False
            pod.pop('imagePullSecrets', None)
            for container in pod.get('containers', []) + pod.get('initContainers', []):
                if container.get('securityContext', {}).get('privileged') or any('hostPort' in p for p in container.get('ports', [])):
                    raise ValueError('privileged container or host port forbidden')
                if ':latest' in container['image'] or not ('@sha256:' in container['image'] or re.search(r':[0-9a-f]{40}$', container['image']) or re.search(r':[v0-9][\w.-]+$', container['image'])):
                    raise ValueError('image must have an explicit version or digest: ' + meta['name'] + '/' + container['name'])
                for env in container.get('env', []):
                    location = (re.search(r'_(?:DIR|PATH|FILE|LOCATION)$', env['name']) and
                                env.get('value', '').startswith(('/', 'file:/', 'classpath:/')))
                    if ('value' in env and re.search(r'PASSWORD|PRIVATE_KEY|API_SECRET|ENCRYPTION_KEY|STATIC_TOKENS|HASH_KEY|API_KEY|ACCESS_KEY|TOKEN_SECRET', env['name'])
                            and not location):
                        raise ValueError('sensitive literal env forbidden: ' + env['name'])
                memory = '1024Mi' if meta['name'] == 'voice-auth' else '384Mi'
                request_memory = '64Mi'
                if meta['name'] == 'voice-clickhouse':
                    memory = '2048Mi'
                    request_memory = '256Mi'
                if meta['name'] == 'voice-clamav':
                    memory = '1536Mi'
                if container['name'] == 'nats-leaf':
                    memory = '128Mi'
                    request_memory = '64Mi'
                container['resources'] = {'requests': {'cpu': '20m', 'memory': request_memory},
                                          'limits': {'cpu': '500m', 'memory': memory}}
            if kind != 'Job':
                spec['replicas'] = 0
                if kind == 'Deployment':
                    spec['strategy'] = {'type': 'Recreate'}
            else:
                spec['suspend'] = True
                spec.pop('selector', None)
                for field in ('controller-uid', 'batch.kubernetes.io/controller-uid', 'job-name', 'batch.kubernetes.io/job-name'):
                    spec['template']['metadata'].get('labels', {}).pop(field, None)
                spec['ttlSecondsAfterFinished'] = 86400
            for claim in spec.get('volumeClaimTemplates', []):
                claim.pop('status', None)
                clean_meta(claim['metadata'])
                claim['spec']['storageClassName'] = 'voice-dev-local'
                claim['spec']['resources']['requests']['storage'] = '2Gi' if meta['name'] == 'voice-clickhouse' else '5Gi'
        elif kind == 'Service':
            for field in ('clusterIP', 'clusterIPs', 'ipFamilies', 'ipFamilyPolicy', 'healthCheckNodePort'):
                spec.pop(field, None)
            for port in spec.get('ports', []):
                if meta['name'] == 'voice-livekit' and port.get('name') in ('rtc-tcp', 'rtc-udp'):
                    port['nodePort'] = port['targetPort'] = 31081 if port['name'] == 'rtc-tcp' else 31082
                else:
                    port.pop('nodePort', None)
        elif kind == 'PersistentVolumeClaim':
            spec.pop('volumeName', None)
            spec['storageClassName'] = 'voice-dev-local'
            spec['resources']['requests']['storage'] = '1Gi'
        serialized = json.dumps(obj)
        if re.search(r'(?:postgres(?:ql)?|redis|clickhouse)://[^/@\s]+:[^/@\s]+@|://(?:10\.|192\.168\.|172\.(?:1[6-9]|2[0-9]|3[01])\.)', serialized):
            raise ValueError('literal credential or private network endpoint forbidden')
        if 'voice-prod' in serialized or 'voice-staging' in serialized or 'PRIVATE KEY' in serialized:
            raise ValueError('foreign environment or private key in rendered object')
        output.append(obj)
    return output


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('snapshot', type=Path)
    parser.add_argument('output', type=Path)
    parser.add_argument('--node', default='pmdebook')
    args = parser.parse_args()
    snapshot = json.loads(args.snapshot.read_text())
    result = foundation(args.node) + render(snapshot['items'], args.node)
    args.output.write_text(json.dumps({'apiVersion': 'v1', 'kind': 'List', 'items': result}))
    args.output.chmod(0o600)
    print('Rendered paused dev resources: ' + str(len(result)))


if __name__ == '__main__':
    main()
