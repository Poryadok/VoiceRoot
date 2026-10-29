"""Fixed source-derived DEV Realtime principal JWKS publisher."""
import copy
import base64
import hashlib
import ipaddress
import json
from pathlib import Path
import re
import secrets
import time

import frontend_runtime_executor
import full_cohort_oci as oci
import media_operation_guard
import publish_candidate
import realtime_p3_create_only as p3
import realtime_p3_create_only_crypto as crypto
import realtime_p3_create_only_transport as p3_transport

INDEX = 'sha256:0e52bd054126cdc040b1276c0af347aeb02a41b9cfe305d6094767ac30e97949'
PLATFORM = 'sha256:84ed8c0dd296c188bfc24ed506f46589e9285fb0b11cf3a04d91d524d3067e11'
CONFIG = 'sha256:88f52f3d58a505b7e1ad96ca6969478517bcccb58418e75a54223271a64da7df'
IMAGE = 'docker.io/library/nginx@' + INDEX
NAMESPACE = 'voice-dev'
NAME = 'voice-p3-principal-jwks'
CONFIG_NAME = 'voice-p3-principal-publisher-config'
PUBLIC_NAME = 'voice-p3-principal-public'
TLS_NAME = 'voice-p3-principal-jwks-tls'
REVIEWED_SOURCE = '7d4221d2355e9f6019beaf7c154b082dc4c5e783'
SOURCE_PATH = 'deploy/staging/services.yaml'
CONFIG_TEXT = ('server {\n'
    '  listen 8443 ssl;\n'
    '  ssl_certificate /etc/voice/tls/tls.crt;\n'
    '  ssl_certificate_key /etc/voice/tls/tls.key;\n'
    '  ssl_protocols TLSv1.2 TLSv1.3;\n'
    '  server_tokens off;\n'
    '  root /srv/jwks;\n'
    '  location = /realtime.json { limit_except GET { deny all; } try_files $uri =404; }\n'
    '  location / { return 404; }\n'
    '}\n')


class PublisherError(RuntimeError): pass
class _NotReady(Exception): pass


def _source_documents(raw):
    result = {}
    for piece in re.split(r'^---\s*$', raw.decode('utf-8'), flags=re.M):
        kind = re.search(r'^kind: (ConfigMap|Deployment|Service)\s*$', piece, flags=re.M)
        name = re.search(r'^  name: (voice-p3-principal-publisher-config|voice-p3-principal-jwks)\s*$', piece, flags=re.M)
        if not kind or not name: continue
        key = (kind.group(1), name.group(1))
        if key in result: raise ValueError()
        result[key] = piece
    if set(result) != {('ConfigMap', CONFIG_NAME), ('Deployment', NAME), ('Service', NAME)}: raise ValueError()
    config = result['ConfigMap', CONFIG_NAME].split('  default.conf: |\n', 1)[1]
    lines = config.splitlines()
    actual = '\n'.join(line[4:] for line in lines if line.startswith('    ')) + '\n'
    if actual != CONFIG_TEXT: raise ValueError()
    return result


def verify_source(repo, candidate_sha):
    """Require exact reviewed publisher documents in captured candidate source."""
    import realtime_p3_create_only_source as source
    try:
        if not Path(repo).is_absolute() or not isinstance(candidate_sha, str) or not re.fullmatch(r'[0-9a-f]{40}', candidate_sha):
            raise ValueError()
        reviewed = source._blob(repo, REVIEWED_SOURCE, SOURCE_PATH)
        candidate = source._blob(repo, candidate_sha, SOURCE_PATH)
        if source._disk(repo, SOURCE_PATH) != candidate or _source_documents(reviewed) != _source_documents(candidate):
            raise ValueError()
    except Exception:
        raise PublisherError('publisher source qualification failed') from None


def qualify_oci(index_raw, child_raw, config_raw):
    """Authenticate the fixed image index, unique amd64 child and config bytes."""
    try:
        index = oci._decode(index_raw, INDEX)
        if type(index.get('schemaVersion')) is not int or index['schemaVersion'] != 2 or index.get('mediaType') not in oci.INDEX_TYPES:
            raise ValueError()
        if publish_candidate.platform_manifest(index) != PLATFORM: raise ValueError()
        selected = next(item for item in index['manifests'] if item.get('digest') == PLATFORM)
        oci._descriptor(selected, oci.MANIFEST_TYPES)
        if len(child_raw) != selected['size']: raise ValueError()
        child = oci._decode(child_raw, PLATFORM)
        if (type(child.get('schemaVersion')) is not int or child['schemaVersion'] != 2
            or child.get('mediaType') not in (None, selected['mediaType']) or type(child.get('layers')) is not list): raise ValueError()
        for layer in child['layers']: oci._descriptor(layer, oci.LAYER_TYPES)
        descriptor = oci._descriptor(child['config'], oci.CONFIG_TYPES)
        if descriptor['digest'] != CONFIG or len(config_raw) != descriptor['size']: raise ValueError()
        config = oci._decode(config_raw, CONFIG)
        if config.get('os') != 'linux' or config.get('architecture') != 'amd64' or config.get('variant'): raise ValueError()
        return {'index': INDEX, 'platform': PLATFORM, 'config': CONFIG}
    except Exception:
        raise PublisherError('publisher image qualification failed') from None


def desired(hostname):
    if not isinstance(hostname, str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?', hostname):
        raise PublisherError('publisher node identity invalid')
    meta = lambda name: {'name': name, 'namespace': NAMESPACE}
    config = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': meta(CONFIG_NAME), 'data': {'default.conf': CONFIG_TEXT}}
    container = {'name': 'jwks', 'image': IMAGE, 'ports': [{'containerPort': 8443}],
                 'volumeMounts': [{'name': 'public', 'mountPath': '/srv/jwks', 'readOnly': True},
                                  {'name': 'tls', 'mountPath': '/etc/voice/tls', 'readOnly': True},
                                  {'name': 'config', 'mountPath': '/etc/nginx/conf.d', 'readOnly': True}],
                 'readinessProbe': {'tcpSocket': {'port': 8443}, 'periodSeconds': 5}}
    pod = {'containers': [container], 'automountServiceAccountToken': False, 'nodeSelector': {'kubernetes.io/hostname': hostname,
            'kubernetes.io/os': 'linux', 'kubernetes.io/arch': 'amd64'}, 'volumes': [
            {'name': 'public', 'configMap': {'name': PUBLIC_NAME, 'items': [{'key': 'realtime.json', 'path': 'realtime.json'}]}},
            {'name': 'tls', 'secret': {'secretName': TLS_NAME, 'defaultMode': 288,
             'items': [{'key': 'tls.crt', 'path': 'tls.crt'}, {'key': 'tls.key', 'path': 'tls.key'}]}},
            {'name': 'config', 'configMap': {'name': CONFIG_NAME}}]}
    labels = {'app': NAME}
    deployment = {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': meta(NAME),
                  'spec': {'replicas': 1, 'selector': {'matchLabels': copy.deepcopy(labels)},
                           'template': {'metadata': {'labels': copy.deepcopy(labels)}, 'spec': pod}}}
    service = {'apiVersion': 'v1', 'kind': 'Service', 'metadata': meta(NAME),
               'spec': {'selector': copy.deepcopy(labels), 'ports': [{'name': 'https', 'port': 8443}]}}
    return config, deployment, service


def check_admission(actual, wanted):
    """Accept only source shape plus finite Kubernetes API defaults."""
    try:
        if (actual.get('apiVersion') != wanted['apiVersion'] or actual.get('kind') != wanted['kind']
            or actual.get('metadata', {}).get('name') != wanted['metadata']['name']
            or actual.get('metadata', {}).get('namespace') != NAMESPACE
            or actual.get('metadata', {}).get('deletionTimestamp')): raise ValueError()
        if wanted['kind'] == 'ConfigMap':
            if actual.get('data') != wanted['data'] or actual.get('binaryData') or actual.get('immutable', False) is not False:
                raise ValueError()
            return
        spec = copy.deepcopy(actual['spec']); expected = copy.deepcopy(wanted['spec'])
        def default(row, key, value):
            if key in row and row[key] == value: row.pop(key)
        if wanted['kind'] == 'Deployment':
            default(spec, 'revisionHistoryLimit', 10); default(spec, 'progressDeadlineSeconds', 600)
            default(spec, 'strategy', {'type': 'RollingUpdate', 'rollingUpdate': {'maxSurge': '25%', 'maxUnavailable': '25%'}})
            template = spec['template']; default(template['metadata'], 'creationTimestamp', None)
            pod = template['spec']; defaults = {'dnsPolicy': 'ClusterFirst', 'restartPolicy': 'Always',
                'schedulerName': 'default-scheduler', 'terminationGracePeriodSeconds': 30, 'securityContext': {}}
            for key, value in defaults.items(): default(pod, key, value)
            for c in pod['containers']:
                for key, value in {'imagePullPolicy':'IfNotPresent','terminationMessagePath':'/dev/termination-log',
                                   'terminationMessagePolicy':'File','resources':{}}.items(): default(c,key,value)
                for p in c.get('ports', []): default(p,'protocol','TCP')
                probe = c.get('readinessProbe', {})
                for key,value in {'timeoutSeconds':1,'successThreshold':1,'failureThreshold':3}.items(): default(probe,key,value)
            for volume in pod.get('volumes', []):
                if 'configMap' in volume: default(volume['configMap'], 'defaultMode', 420)
        elif wanted['kind'] == 'Service':
            ip = spec.pop('clusterIP', None)
            if ip is not None:
                address = ipaddress.ip_address(ip)
                if address.is_unspecified or address.is_multicast or address.is_loopback or address.is_link_local: raise ValueError()
                if 'clusterIPs' in spec and spec.pop('clusterIPs') != [ip]: raise ValueError()
                family = 'IPv4' if address.version == 4 else 'IPv6'
                if 'ipFamilies' in spec and spec.pop('ipFamilies') != [family]: raise ValueError()
            elif 'clusterIPs' in spec or 'ipFamilies' in spec: raise ValueError()
            default(spec, 'ipFamilyPolicy', 'SingleStack'); default(spec, 'internalTrafficPolicy', 'Cluster')
            default(spec, 'type', 'ClusterIP'); default(spec, 'sessionAffinity', 'None')
            for port in spec.get('ports', []):
                default(port, 'protocol', 'TCP'); default(port, 'targetPort', 8443)
        else: raise ValueError()
        if spec != expected: raise ValueError()
    except Exception:
        raise PublisherError('publisher server admission differs') from None


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def _public_tls():
    template = ('{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}'
                '{{.metadata.deletionTimestamp}}{{"\\n"}}{{.type}}{{"\\n"}}{{len .data}}{{"\\n"}}'
                '{{if index .data "tls.key"}}present{{end}}{{"\\n"}}{{index .data "tls.crt"}}')
    raw = p3_transport._run(['get', 'secret', TLS_NAME, '-o', 'go-template=' + template])
    fields = raw.split(b'\n')
    if len(fields) != 7 or fields[2] not in (b'', b'<no value>') or fields[3:6] != [b'Opaque', b'2', b'present']:
        raise ValueError()
    cert = base64.b64decode(fields[6], validate=True)
    return {'uid': fields[0].decode('ascii'), 'resourceVersion': fields[1].decode('ascii')}, cert


def _verify_cert(cert, ca):
    host = 'voice-p3-principal-jwks.voice-dev.svc'
    crypto._call(['verify', '-CAfile', '{1}', '-purpose', 'sslserver', '-verify_hostname', host, '{0}'], [cert, ca])
    crypto._call(['x509', '-in', '{0}', '-noout', '-checkend', '86400'], [cert])
    sans = crypto._call(['x509', '-in', '{0}', '-noout', '-ext', 'subjectAltName'], [cert]).decode('ascii')
    if host not in re.findall(r'DNS:([A-Za-z0-9.-]+)', sans): raise ValueError()
    if b'TLS Web Server Authentication' not in crypto._call(['x509', '-in', '{0}', '-noout', '-ext', 'extendedKeyUsage'], [cert]): raise ValueError()
    constraints = crypto._call(['x509', '-in', '{0}', '-noout', '-ext', 'basicConstraints'], [cert])
    if b'CA:FALSE' not in constraints or b'CA:TRUE' in constraints: raise ValueError()
    return hashlib.sha256(crypto.der(cert)).hexdigest()


class PublisherPhase:
    """One create-only attempt under the same physical P3 operation fence."""
    def __init__(self, journal, fence, client, lock, namespace_uid, p3_phase):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid, self.p3 = copy.deepcopy(lock), namespace_uid, p3_phase
        self.intent = copy.deepcopy(journal.intent); self.plan = self.intent.get('realtime_jwks_publisher')
        self.record = None; self.pending = False; self.poisoned = False
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
            or getattr(fence.ledger, 'client', None) is not client or not isinstance(p3_phase, p3.Session)
            or p3_phase.fence.context != fence.context or p3_phase.namespace_uid != namespace_uid
            or p3_phase.lock != lock or self.intent.get('resource_context') != fence.context
            or self.intent.get('namespace_uid') != namespace_uid or type(self.plan) is not dict
            or set(self.plan) != {'source_repo', 'source_sha', 'node'}
            or self.plan['source_sha'] != fence.context.get('commit') or not Path(self.plan['source_repo']).is_absolute()):
            raise PublisherError('publisher operation identity invalid')
        node = self.plan['node']
        if (type(node) is not dict or set(node) != {'name', 'uid', 'resourceVersion', 'hostname'}
            or not all(isinstance(v, str) and v for v in node.values())): raise PublisherError('publisher node invalid')
        desired(node['hostname'])

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent
            or self.journal.state.get('intent') != self.intent): raise ValueError()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        verify_source(self.plan['source_repo'], self.plan['source_sha'])
        self.p3._guard()
        first, revision = self.fence.ledger.read(); second, other_revision = self.p3.fence.ledger.read()
        if (first != second or revision != other_revision or not self.fence.owned(first)
            or first.get('status') != 'active' or not self.p3.completed
            or first.get('realtime_p3_handoff') != self.p3.handoff
            or self.p3.journal.state['steps'].get('realtime-p3-handoff') != self.p3.handoff
            or self.p3.handoff.get('status') != 'done'):
            raise ValueError()
        local = self.journal.state['steps'].get('realtime-jwks-publisher')
        if first.get('realtime_jwks_publisher') != self.record or local != self.record: raise ValueError()
        if self.record is not None: self._validate_record()

    def _validate_record(self):
        value = self.record; fields = {'status', 'nonce', 'operation_id', 'source_sha', 'node', 'image',
                                      'p3_handoff_sha256', 'public', 'tls', 'objects'}
        if value.get('status') == 'done': fields.add('runtime')
        hex64 = lambda x: isinstance(x, str) and bool(re.fullmatch(r'[0-9a-f]{64}', x))
        text = lambda x: isinstance(x, str) and bool(x)
        if (type(value) is not dict or set(value) != fields or value.get('status') not in ('pending', 'done')
            or not isinstance(value.get('nonce'), str) or not re.fullmatch(r'[0-9a-f]{32}', value['nonce'])
            or value.get('operation_id') != self.fence.context['operation_id'] or value.get('source_sha') != self.plan['source_sha']
            or value.get('node') != self.plan['node'] or value.get('image') != {'index': INDEX, 'platform': PLATFORM, 'config': CONFIG}
            or value.get('p3_handoff_sha256') != digest(self.p3.handoff)): raise ValueError()
        for key, hashkey in (('public', 'sha256'), ('tls', 'certificate_sha256')):
            row = value[key]
            if (type(row) is not dict or set(row) != {'uid', 'resourceVersion', hashkey}
                or not text(row['uid']) or not text(row['resourceVersion']) or not hex64(row[hashkey])): raise ValueError()
        objects = value['objects']; keys = ('configmap', 'deployment', 'service')
        names = {'configmap': CONFIG_NAME, 'deployment': NAME, 'service': NAME}
        if type(objects) is not dict or set(objects) != set(keys[:len(objects)]): raise ValueError()
        for key in keys[:len(objects)]:
            row = objects[key]; expected = {'status', 'nonce', 'name'}
            if row.get('status') == 'done':
                expected |= {'uid', 'resourceVersion', 'spec_sha256'}
                if key == 'deployment': expected.add('generation')
            elif row.get('status') != 'pending' or key != keys[len(objects) - 1]: raise ValueError()
            if (type(row) is not dict or set(row) != expected or row['name'] != names[key]
                or not isinstance(row['nonce'], str) or not re.fullmatch(r'[0-9a-f]{32}', row['nonce'])): raise ValueError()
            if row['status'] == 'done' and (not text(row['uid']) or not text(row['resourceVersion']) or not hex64(row['spec_sha256'])): raise ValueError()
            if key == 'deployment' and row['status'] == 'done' and (type(row['generation']) is not int or row['generation'] < 1): raise ValueError()
        if value['status'] == 'done':
            runtime = value['runtime']; rfields = {'replicaset_uid', 'replicaset_resourceVersion', 'pod_name', 'pod_uid',
                'pod_resourceVersion', 'node_name', 'container_id', 'image_id', 'restart_count', 'deployment_resourceVersion'}
            if (set(objects) != set(keys) or any(objects[k]['status'] != 'done' for k in keys)
                or type(runtime) is not dict or set(runtime) != rfields or runtime['node_name'] != self.plan['node']['name']
                or not all(text(runtime[k]) for k in rfields - {'restart_count'})
                or type(runtime['restart_count']) is not int or runtime['restart_count'] < 0
                or not runtime['image_id'].endswith(PLATFORM)
                or runtime['deployment_resourceVersion'] != objects['deployment']['resourceVersion']): raise ValueError()

    def _node(self):
        self._guard(); selected = self.client.call('get', 'node', self.plan['node']['name'], '-o', 'json')
        rows = self.client.call('get', 'nodes', '-o', 'json')['items']; self._guard()
        node = self.plan['node']; meta = selected['metadata']; labels = meta.get('labels', {})
        if (meta.get('name') != node['name'] or meta.get('uid') != node['uid'] or meta.get('resourceVersion') != node['resourceVersion']
            or meta.get('deletionTimestamp') or labels.get('kubernetes.io/hostname') != node['hostname']
            or labels.get('kubernetes.io/os') != 'linux' or labels.get('kubernetes.io/arch') != 'amd64'
            or not any(x.get('type') == 'Ready' and x.get('status') == 'True' for x in selected.get('status', {}).get('conditions', []))
            or sum(r.get('metadata', {}).get('labels', {}).get('kubernetes.io/hostname') == node['hostname'] for r in rows) != 1):
            raise ValueError()

    def _inputs(self):
        self._guard(); self.p3._check_roots(); receipt = self.p3.handoff['receipt']
        if (receipt.get('operation_id') != self.fence.context['operation_id']
            or receipt.get('candidate_source_sha') != self.fence.context['commit']
            or receipt.get('namespace_uid') != self.namespace_uid): raise ValueError()
        objects = receipt['objects']; pub = objects['configmap-' + PUBLIC_NAME]; tls = objects['secret-' + TLS_NAME]
        if (pub.get('status') != 'done' or tls.get('status') != 'done'
            or pub.get('kind') != 'ConfigMap' or tls.get('kind') != 'Secret'
            or pub.get('name') != PUBLIC_NAME or tls.get('name') != TLS_NAME
            or pub.get('keys') != ['realtime.json'] or tls.get('keys') != ['tls.crt', 'tls.key']): raise ValueError()
        current = self.client.call('get', 'configmap', PUBLIC_NAME, '-o', 'json')
        meta = current['metadata']; values = current['data']
        if (current.get('immutable') is not True or current.get('binaryData') or set(values) != {'realtime.json'}
            or meta.get('namespace') != NAMESPACE or meta.get('name') != PUBLIC_NAME or meta.get('deletionTimestamp')
            or any(meta.get(k) != pub.get(k) for k in ('uid', 'resourceVersion'))): raise ValueError()
        raw = values['realtime.json'].encode('utf-8')
        if not 0 < len(raw) <= 131072 or not isinstance(json.loads(raw), dict): raise ValueError()
        tls_meta, cert = _public_tls(); self._guard()
        if any(tls_meta.get(k) != tls.get(k) for k in ('uid', 'resourceVersion')): raise ValueError()
        ca = self.p3.roots[('voice-principal-ca', 'ca.crt')]['certificate']
        cert_sha = _verify_cert(cert, ca); self.p3._check_roots(); self._guard()
        return ({'uid': pub['uid'], 'resourceVersion': pub['resourceVersion'], 'sha256': hashlib.sha256(raw).hexdigest()},
                {'uid': tls['uid'], 'resourceVersion': tls['resourceVersion'], 'certificate_sha256': cert_sha})

    def _get(self, kind, name):
        result = self.client.call('get', kind.lower(), name, '--ignore-not-found=true', '-o', 'json')
        return result or None

    def _selector(self):
        self._guard()
        args = ('-l', 'app=' + NAME, '-o', 'json')
        replicasets = self.client.call('get', 'replicasets', *args)['items']
        pods = self.client.call('get', 'pods', *args)['items']
        deployment = (self.record or {}).get('objects', {}).get('deployment', {})
        uid = deployment.get('uid') if deployment.get('status') == 'done' else None
        def owner(row, kind):
            owners = [x for x in row.get('metadata', {}).get('ownerReferences', []) if x.get('controller') is True]
            if len(owners) != 1 or owners[0].get('kind') != kind: raise ValueError()
            return owners[0].get('uid')
        rs_uids = set()
        for row in replicasets:
            meta = row['metadata']
            if (uid is None or owner(row, 'Deployment') != uid or meta.get('namespace') != NAMESPACE
                or meta.get('deletionTimestamp') or not meta.get('uid') or meta.get('labels', {}).get('app') != NAME): raise ValueError()
            rs_uids.add(meta['uid'])
        for row in pods:
            meta = row['metadata']
            if (owner(row, 'ReplicaSet') not in rs_uids or meta.get('namespace') != NAMESPACE
                or meta.get('deletionTimestamp') or meta.get('labels', {}).get('app') != NAME): raise ValueError()
        self._guard()
        return replicasets, pods

    def _write(self, new):
        self._guard(); row, revision = self.fence.ledger.read()
        self.journal.state['steps']['realtime-jwks-publisher'] = copy.deepcopy(new); self.journal.save()
        current, current_revision = self.fence.ledger.read()
        if current != row or current_revision != revision: raise ValueError()
        current['realtime_jwks_publisher'] = copy.deepcopy(new)
        self.fence.ledger.write(revision, current); self.record = copy.deepcopy(new); self._guard()

    def _check_objects(self, expected):
        for kind, record in self.record['objects'].items():
            wanted = expected[kind]; current = self._get(wanted['kind'], wanted['metadata']['name'])
            if record['status'] != 'done' or current is None: raise ValueError()
            check_admission(current, wanted); meta = current['metadata']
            value = current['data'] if kind == 'configmap' else current['spec']
            if (meta.get('uid') != record['uid']
                or (kind != 'deployment' or self.record['status'] == 'done') and meta.get('resourceVersion') != record['resourceVersion']
                or digest(value) != record['spec_sha256']): raise ValueError()
            if kind == 'deployment' and meta.get('generation') != record['generation']: raise ValueError()
            if kind == 'service' and not current['spec'].get('clusterIP'): raise ValueError()

    def _runtime(self, expected):
        self._guard(); self._node(); replicasets, pods = self._selector()
        if len(replicasets) <= 1 and not pods: raise _NotReady()
        if len(replicasets) != 1 or len(pods) != 1: raise ValueError()
        rs, pod = replicasets[0], pods[0]; meta = pod['metadata']; rmeta = rs['metadata']
        deployment = self._get('Deployment', NAME); dmeta = deployment['metadata']; status = deployment.get('status', {})
        if (type(dmeta.get('generation')) is not int or not dmeta.get('resourceVersion')
            or any(type(status.get(k, 0)) is not int for k in ('observedGeneration', 'readyReplicas', 'updatedReplicas', 'availableReplicas'))):
            raise ValueError()
        if (status.get('observedGeneration', 0) < dmeta['generation']
            or any(status.get(k, 0) != 1 for k in ('readyReplicas', 'updatedReplicas', 'availableReplicas'))): raise _NotReady()
        if (not all(isinstance(meta.get(k), str) and meta[k] for k in ('name', 'uid', 'resourceVersion'))
            or not all(isinstance(rmeta.get(k), str) and rmeta[k] for k in ('uid', 'resourceVersion'))): raise ValueError()
        spec = copy.deepcopy(pod['spec'])
        if spec.pop('nodeName', None) != self.plan['node']['name']: raise ValueError()
        defaults = {'serviceAccountName': 'default', 'serviceAccount': 'default', 'enableServiceLinks': True,
                    'preemptionPolicy': 'PreemptLowerPriority', 'priority': 0}
        for key, value in defaults.items():
            if spec.get(key) == value: spec.pop(key)
        tolerations = [{'key':'node.kubernetes.io/not-ready','operator':'Exists','effect':'NoExecute','tolerationSeconds':300},
                       {'key':'node.kubernetes.io/unreachable','operator':'Exists','effect':'NoExecute','tolerationSeconds':300}]
        if spec.get('tolerations') == tolerations: spec.pop('tolerations')
        synthetic = copy.deepcopy(expected['deployment']); synthetic['spec']['template']['spec'] = spec
        check_admission(synthetic, expected['deployment'])
        conditions = pod.get('status', {}).get('conditions', [])
        statuses = pod.get('status', {}).get('containerStatuses', [])
        if len(statuses) != 1 or statuses[0].get('name') != 'jwks': raise ValueError()
        container = statuses[0]
        if (not isinstance(container.get('containerID'), str) or not container['containerID']
            or not isinstance(container.get('imageID'), str) or not container['imageID'].endswith(PLATFORM)
            or type(container.get('restartCount')) is not int or container['restartCount'] < 0): raise ValueError()
        if (container.get('ready') is not True or 'running' not in container.get('state', {})
            or not any(c.get('type') == 'Ready' and c.get('status') == 'True' for c in conditions)): raise _NotReady()
        self._guard()
        return {'replicaset_uid': rmeta['uid'], 'replicaset_resourceVersion': rmeta['resourceVersion'],
                'pod_name': meta['name'], 'pod_uid': meta['uid'], 'pod_resourceVersion': meta['resourceVersion'],
                'node_name': spec.get('nodeName', self.plan['node']['name']), 'container_id': container['containerID'],
                'image_id': container['imageID'], 'restart_count': container['restartCount'],
                'deployment_resourceVersion': dmeta['resourceVersion']}

    def run(self, index_raw, child_raw, config_raw):
        try:
            self._guard(); verify_source(self.plan['source_repo'], self.plan['source_sha'])
            image = qualify_oci(index_raw, child_raw, config_raw); self._node(); public, tls = self._inputs(); self._selector()
            values = desired(self.plan['node']['hostname'])
            expected = dict(zip(('configmap', 'deployment', 'service'), values))
            for obj in values:
                self._guard()
                if self._get(obj['kind'], obj['metadata']['name']) is not None: raise ValueError()
            record = {'status': 'pending', 'nonce': secrets.token_hex(16), 'operation_id': self.fence.context['operation_id'],
                      'source_sha': self.plan['source_sha'], 'node': copy.deepcopy(self.plan['node']), 'image': image,
                      'p3_handoff_sha256': digest(self.p3.handoff), 'public': public, 'tls': tls, 'objects': {}}
            self.pending = True; self._write(record)
            for obj in values:
                self._guard(); self._node()
                if self._inputs() != (public, tls) or self._get(obj['kind'], obj['metadata']['name']) is not None: raise ValueError()
                dry = self.client.call('create', '--dry-run=server', '-f', '-', '-o', 'json', body=obj)
                check_admission(dry, obj)
            for key, obj in expected.items():
                self._guard(); self._node(); self._check_objects(expected); self._selector()
                if self._inputs() != (public, tls) or self._get(obj['kind'], obj['metadata']['name']) is not None: raise ValueError()
                pending = copy.deepcopy(self.record); pending['objects'][key] = {'status': 'pending', 'nonce': secrets.token_hex(16), 'name': obj['metadata']['name']}
                self._write(pending); self._node()
                if self._inputs() != (public, tls) or self._get(obj['kind'], obj['metadata']['name']) is not None: raise ValueError()
                response = self.client.call('create', '-f', '-', '-o', 'json', body=obj)
                self._guard(); actual = self._get(obj['kind'], obj['metadata']['name'])
                check_admission(response, obj); check_admission(actual, obj)
                meta = actual['metadata']; returned = response['metadata']
                if any(not isinstance(meta.get(k), str) or not meta[k] or meta[k] != returned.get(k) for k in ('uid', 'resourceVersion')): raise ValueError()
                done = copy.deepcopy(self.record)
                done['objects'][key] = {'status': 'done', 'nonce': pending['objects'][key]['nonce'], 'name': obj['metadata']['name'],
                                        'uid': meta['uid'], 'resourceVersion': meta['resourceVersion'],
                                        'spec_sha256': digest(actual['data'] if key == 'configmap' else actual['spec'])}
                if key == 'deployment':
                    if type(meta.get('generation')) is not int or meta['generation'] < 1: raise ValueError()
                    done['objects'][key]['generation'] = meta['generation']
                self._write(done)
            deadline = time.monotonic() + 45
            while True:
                try:
                    runtime = self._runtime(expected); break
                except _NotReady:
                    if time.monotonic() >= deadline: raise ValueError()
                    time.sleep(1)
            done = copy.deepcopy(self.record); done['status'] = 'done'; done['runtime'] = runtime
            done['objects']['deployment']['resourceVersion'] = runtime['deployment_resourceVersion']
            self._write(done)
            return verify_publisher(self)
        except BaseException:
            if self.pending:
                self.poisoned = True
                try: self.fence.uncertain('Realtime JWKS publisher outcome uncertain')
                except BaseException: pass
            raise PublisherError('publisher not admitted; reconcile if pending') from None


def verify_publisher(phase):
    try:
        if type(phase) is not PublisherPhase or phase.record is None or phase.record.get('status') != 'done': raise ValueError()
        phase._guard(); verify_source(phase.plan['source_repo'], phase.plan['source_sha']); phase._node()
        public, tls = phase._inputs(); expected = dict(zip(('configmap', 'deployment', 'service'), desired(phase.plan['node']['hostname'])))
        if public != phase.record['public'] or tls != phase.record['tls'] or phase.record['p3_handoff_sha256'] != digest(phase.p3.handoff): raise ValueError()
        if set(phase.record['objects']) != set(expected): raise ValueError()
        phase._check_objects(expected)
        if phase._runtime(expected) != phase.record['runtime']: raise ValueError()
        phase._guard()
        return {'schema': 'voice-realtime-jwks-publisher-v1', 'operation_id': phase.fence.context['operation_id'],
                'candidate_source_sha': phase.plan['source_sha'], 'namespace_uid': phase.namespace_uid,
                'image': copy.deepcopy(phase.record['image']), 'node': copy.deepcopy(phase.record['node']),
                'public': public, 'tls': tls, 'objects': copy.deepcopy(phase.record['objects']),
                'runtime': copy.deepcopy(phase.record['runtime']), 'scope': 'create-only'}
    except Exception:
        if type(phase) is PublisherPhase and phase.pending:
            phase.poisoned = True
            try: phase.fence.uncertain('Realtime JWKS publisher proof changed')
            except BaseException: pass
        raise PublisherError('publisher proof unavailable') from None
