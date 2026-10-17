"""Native, typed Kubernetes stage for the NATS rollout preservation runner.

This module deliberately has no shell, kubectl, credential-file, or host-path
authority. All reads and the small set of writes go through the injected fixed
KubernetesJSONAPI. Durable recovery authority is supplied by the caller as an
immutable history and is never inferred from a fresh, unjournaled operation.
"""
from __future__ import annotations

import copy
import hashlib
import json
import re
import time
from collections.abc import Mapping, Sequence

from controller import Blocked
from kube_json_api import KubeAPIError, KubeNotFound, NAMESPACE


MARKER = 'voice-nats-generation'
HUB = 'voice-nats-pvc-candidate'
SERVICE = 'voice-nats'
LEAVES = ('analytics', 'auth', 'bot', 'chat', 'file', 'matchmaking', 'messaging',
    'moderation', 'notification', 'realtime', 'role', 'search', 'social', 'space',
    'story', 'subscription', 'user', 'voice')
NATS_IMAGE = 'nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb'
_UID = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
_OP = re.compile(r'[a-z0-9]{8,32}')
_SOURCE = re.compile(r'voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}')
_PHASES = ('rollout-capturing', 'rollout-prepared', 'rollout-applying', 'rollout-verified')


def _blocked(reason):
    raise Blocked(reason)


def _meta(row, *, name=None, namespace=None):
    value = row.get('metadata', {})
    if (not isinstance(value, dict) or (name is not None and value.get('name') != name)
        or (namespace is not None and value.get('namespace') != namespace)
        or not _UID.fullmatch(value.get('uid', ''))
        or not isinstance(value.get('resourceVersion'), str) or not value['resourceVersion']):
        _blocked('rollout_object_identity_invalid')
    return value


def _sha(value):
    try:
        raw = json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False,
            allow_nan=False).encode('utf-8')
    except (TypeError, ValueError, UnicodeError):
        _blocked('rollout_snapshot_invalid')
    return hashlib.sha256(raw).hexdigest()


def _hub_claim(deployment):
    try:
        rows = [v for v in deployment['spec']['template']['spec']['volumes'] if v.get('name') == 'jsdata']
        if (len(rows) != 1 or set(rows[0]) != {'name', 'persistentVolumeClaim'}
            or set(rows[0]['persistentVolumeClaim']) != {'claimName'}):
            raise ValueError()
        claim = rows[0]['persistentVolumeClaim']['claimName']
        if not isinstance(claim, str) or not _SOURCE.fullmatch(claim):
            raise ValueError()
        return claim
    except (KeyError, TypeError, AttributeError, ValueError):
        _blocked('rollout_explicit_selected_claim_required')


def _source_identity(api, claim_name, *, expected=None):
    """Freshly bind the source PVC/PV and singleton eligible scheduler Node."""
    try:
        claim = api.get_pvc(namespace=NAMESPACE, name=claim_name)
        cm = _meta(claim, name=claim_name, namespace=NAMESPACE)
        cs = claim['spec']
        if (claim.get('status', {}).get('phase') != 'Bound'
            or cs.get('accessModes') != ['ReadWriteOnce']
            or cs.get('volumeMode', 'Filesystem') != 'Filesystem'
            or cs.get('storageClassName') != 'local-path'):
            _blocked('rollout_source_claim_invalid')
        pv_name = cs['volumeName']
        if not isinstance(pv_name, str) or not pv_name:
            _blocked('rollout_source_claim_invalid')
        pv = api.get_pv(name=pv_name)
        pm = _meta(pv, name=pv_name)
        ps = pv['spec']
        ref = ps['claimRef']
        if (pv.get('status', {}).get('phase') != 'Bound'
            or ps.get('volumeMode', 'Filesystem') != 'Filesystem'
            or ref.get('namespace') != NAMESPACE or ref.get('name') != claim_name
            or ref.get('uid') != cm['uid']):
            _blocked('rollout_source_pv_identity_changed')
        # Never inspect PV local/hostPath. The node-affinity is the scheduler
        # binding authority needed for the API-only writer fence.
        terms = ps['nodeAffinity']['required']['nodeSelectorTerms']
        if (not isinstance(terms, list) or len(terms) != 1
            or set(terms[0]) != {'matchExpressions'} or len(terms[0]['matchExpressions']) != 1):
            _blocked('rollout_source_node_affinity_invalid')
        expr = terms[0]['matchExpressions'][0]
        if (set(expr) != {'key', 'operator', 'values'}
            or expr.get('key') != 'kubernetes.io/hostname' or expr.get('operator') != 'In'
            or not isinstance(expr.get('values'), list) or len(expr['values']) != 1
            or not isinstance(expr['values'][0], str) or not expr['values'][0]):
            _blocked('rollout_source_node_affinity_invalid')
        hostname = expr['values'][0]
        matches = [n for n in api.list_nodes_by_hostname(hostname)
            if n.get('metadata', {}).get('labels', {}).get('kubernetes.io/hostname') == hostname]
        if len(matches) != 1:
            _blocked('rollout_source_node_ambiguous')
        node = matches[0]
        nm = _meta(node)
        ready = [c for c in node.get('status', {}).get('conditions', [])
            if c.get('type') == 'Ready' and c.get('status') == 'True']
        if node.get('spec', {}).get('unschedulable') is True or len(ready) != 1:
            _blocked('rollout_source_node_not_ready')
        identity = {
            'source_claim_name': claim_name, 'source_claim_uid': cm['uid'],
            'source_claim_resource_version': cm['resourceVersion'],
            'source_pv_name': pv_name, 'source_pv_uid': pm['uid'],
            'source_pv_resource_version': pm['resourceVersion'],
            'source_node_name': nm['name'], 'source_node_uid': nm['uid'],
            'source_node_resource_version': nm['resourceVersion'],
            'source_node_hostname': hostname,
        }
        if expected is not None:
            wanted = {k: expected.get(k) for k in identity}
            if wanted != identity:
                _blocked('rollout_source_binding_changed')
        return identity, claim, pv, node
    except Blocked:
        raise
    except (KubeAPIError, KeyError, TypeError, AttributeError, ValueError):
        _blocked('rollout_source_binding_invalid')


def _fence_rows(api):
    result = api.list_fence_objects(namespace=NAMESPACE)
    if (not isinstance(result, Mapping) or set(result) != {'pods', 'replicasets'}
        or any(not isinstance(result[key], list) or len(result[key]) > 2048
            or any(not isinstance(row, Mapping) for row in result[key])
            for key in ('pods', 'replicasets'))):
        _blocked('rollout_fence_inventory_invalid')
    return ([{'kind': 'Pod', **copy.deepcopy(row)} for row in result['pods']]
        + [{'kind': 'ReplicaSet', **copy.deepcopy(row)} for row in result['replicasets']])


def _control_rows(api):
    result = api.list_control_objects(namespace=NAMESPACE)
    keys = {'deployments', 'horizontalpodautoscalers', 'jobs'}
    if (not isinstance(result, Mapping) or set(result) != keys
        or any(not isinstance(result[key], list) or len(result[key]) > 2048
            or any(not isinstance(row, Mapping) for row in result[key]) for key in keys)):
        _blocked('rollout_control_inventory_invalid')
    kinds = {'deployments': 'Deployment', 'horizontalpodautoscalers': 'HorizontalPodAutoscaler',
        'jobs': 'Job'}
    return [{'kind': kinds[key], **copy.deepcopy(row)} for key in sorted(keys)
        for row in result[key]]


def _revision_template(template):
    value = copy.deepcopy(template)
    value.get('metadata', {}).get('labels', {}).pop('pod-template-hash', None)
    return value


def _running_image_pins(api, deployments):
    rows = _fence_rows(api)
    if len(rows) > 4096:
        _blocked('rollout_old_image_inventory_invalid')
    pins = {}
    for name, deployment in deployments.items():
        if name == HUB:
            continue
        deployment_uid = deployment['metadata']['uid']
        owned_rs = {row['metadata']['uid']: row for row in rows if row['kind'] == 'ReplicaSet'
            and any(owner.get('kind') == 'Deployment' and owner.get('uid') == deployment_uid
                and owner.get('controller') is True
                for owner in row.get('metadata', {}).get('ownerReferences', []))}
        pods = [row for row in rows if row['kind'] == 'Pod'
            and row.get('status', {}).get('phase') == 'Running'
            and not row.get('metadata', {}).get('deletionTimestamp')
            and any(owner.get('kind') == 'ReplicaSet' and owner.get('uid') in owned_rs
                and owner.get('controller') is True
                for owner in row.get('metadata', {}).get('ownerReferences', []))]
        ready_rs = {owner['uid'] for pod in pods for owner in pod.get('metadata', {}).get('ownerReferences', [])
            if owner.get('kind') == 'ReplicaSet' and owner.get('uid') in owned_rs}
        for rs_uid in ready_rs:
            listed = owned_rs[rs_uid]
            lm = _meta(listed, namespace=NAMESPACE)
            current = api.get_replicaset(namespace=NAMESPACE, name=lm['name'])
            rm = _meta(current, name=lm['name'], namespace=NAMESPACE)
            if rm['uid'] != rs_uid:
                _blocked('rollout_old_revision_identity_changed')
            if _revision_template(current.get('spec', {}).get('template', {})) != _revision_template(
                    deployment['spec']['template']):
                _blocked('rollout_old_revision_unconverged')
        spec = deployment['spec']['template']['spec']
        containers = spec.get('initContainers', []) + spec['containers']
        for container in containers:
            is_init = container in spec.get('initContainers', [])
            candidates = set()
            for pod in pods:
                statuses = pod.get('status', {}).get(
                    'initContainerStatuses' if is_init else 'containerStatuses', [])
                matches = [row for row in statuses if row.get('name') == container.get('name')]
                for status in matches:
                    if not is_init and status.get('ready') is not True:
                        cid = status.get('containerID', '')
                        state = status.get('state', {})
                        last_state = status.get('lastState', {})
                        running = 'running' in state
                        terminated = state.get('terminated', {}).get('containerID')
                        previous = last_state.get('terminated', {}).get('containerID')
                        if (not re.fullmatch(r'containerd://[a-f0-9]{64}', cid)
                            or not running and terminated != cid and previous != cid):
                            _blocked('rollout_old_container_attempt_missing')
                    image = status.get('imageID', '').removeprefix('docker-pullable://')
                    if not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}@sha256:[a-f0-9]{64}', image):
                        _blocked('rollout_old_image_identity_unusable')
                    candidates.add(image)
            if len(candidates) != 1:
                _blocked('rollout_old_image_ambiguous')
            pins[name + '/' + container['name']] = next(iter(candidates))
    return pins


class NativeRolloutStage:
    """One rollout operation, bound to a typed API and durable journal."""

    def __init__(self, kube, operation, expected_identity, journal, *, history=(), account_id=None):
        if not isinstance(operation, str) or not _OP.fullmatch(operation):
            _blocked('operation_identity_invalid')
        if not callable(journal) or not isinstance(expected_identity, Mapping):
            _blocked('rollout_stage_constructor_invalid')
        if account_id is not None and (not isinstance(account_id, str)
            or not re.fullmatch(r'A[A-Z2-7]{55}', account_id)):
            _blocked('rollout_account_identity_invalid')
        self.kube = kube
        self.operation = operation
        self.expected_identity = copy.deepcopy(dict(expected_identity))
        self.expected = copy.deepcopy(dict(expected_identity))
        self.journal = journal
        self.history = tuple(copy.deepcopy(dict(row)) for row in history
            if isinstance(row, Mapping))
        self.account_id = account_id
        self.snapshots = {}
        self.original_snapshots = {}
        self.old_images = {}
        self.marker = None
        self.service = None
        self.final_claim = None
        self.final_pv = None
        self.original_live_census = None
        self.source_identity = {key: copy.deepcopy(value) for key, value in expected_identity.items()
            if key.startswith('source_')}
        self.final_identity = None
        self.active_claim = expected_identity.get('source_claim')
        self.hub_pod_identity = None
        for event in self.history:
            if event.get('kind') == 'rollout_original_capture':
                self.hub_pod_identity = copy.deepcopy(event.get('hub_pod_identity'))
            if event.get('kind') == 'rollout_marker_write_complete':
                marker = event.get('marker', {})
                selected = marker.get('data', {}).get('dataPVC')
                if selected == self.expected.get('source_claim'):
                    self.active_claim = selected
                    self.marker = copy.deepcopy(marker)

    @classmethod
    def capture(cls, kube, operation, journal, extra_deployments=(), *,
            expected_identity=None, history=(), account_id):
        extras = set(extra_deployments)
        allowed_extras = {'voice-web', 'voice-admin', 'voice-developer-portal'}
        if extras - allowed_extras:
            _blocked('rollout_extra_workload_unsupported')
        try:
            namespace = kube.get_namespace(name=NAMESPACE)
            marker = kube.get_configmap(namespace=NAMESPACE, name=MARKER)
            nsm = _meta(namespace, name=NAMESPACE)
            mm = _meta(marker, name=MARKER, namespace=NAMESPACE)
            data = marker['data']
            if (data.get('phase') != 'active'
                or any(k in data for k in ('knownBaselineOperation', 'knownRolloutOperation'))):
                _blocked('rollout_active_marker_required')
            names = (HUB, 'voice-gateway', *('voice-' + x for x in LEAVES), *sorted(extras))
            rows = {name: kube.get_deployment(namespace=NAMESPACE, name=name) for name in names}
            for name, row in rows.items():
                _meta(row, name=name, namespace=NAMESPACE)
            if rows[HUB].get('spec', {}).get('selector', {}).get('matchLabels') != {'app': HUB}:
                _blocked('rollout_hub_selector_unsupported')
            claim_name = _hub_claim(rows[HUB])
            if data.get('dataPVC') != claim_name:
                _blocked('rollout_explicit_selected_claim_required')
            identity, claim, pv, node = _source_identity(kube, claim_name)
            expected = {
                'namespace_uid': nsm['uid'], 'marker_uid': mm['uid'],
                'marker_resource_version': mm['resourceVersion'],
                'generation': data.get('generation'), 'source_claim': claim_name,
                **identity,
                'deployment_uids': {name: _meta(row)['uid'] for name, row in rows.items()},
                'deployment_resource_versions': {name: _meta(row)['resourceVersion'] for name, row in rows.items()},
            }
            if expected_identity is not None and dict(expected_identity) != expected:
                _blocked('rollout_expected_identity_changed')
            stage = cls(kube, operation, expected, journal, history=history, account_id=account_id)
            stage.marker = copy.deepcopy(marker)
            stage.source_identity = copy.deepcopy(identity)
            stage.source_claim = copy.deepcopy(claim)
            stage.final_claim = copy.deepcopy(claim)
            stage.final_pv = {'metadata': {
                'name': identity['source_pv_name'], 'uid': identity['source_pv_uid'],
                'resourceVersion': identity['source_pv_resource_version']}}
            stage.final_identity = {
                'claim_name': identity['source_claim_name'], 'claim_uid': identity['source_claim_uid'],
                'claim_resource_version': identity['source_claim_resource_version'],
                'pv_name': identity['source_pv_name'], 'pv_uid': identity['source_pv_uid'],
                'pv_resource_version': identity['source_pv_resource_version'],
                'node_name': identity['source_node_name'], 'node_uid': identity['source_node_uid'],
                'node_resource_version': identity['source_node_resource_version'],
                'node_hostname': identity['source_node_hostname'],
            }
            stage.service = copy.deepcopy(kube.get_service(namespace=NAMESPACE, name=SERVICE))
            for name, row in rows.items():
                if name in extras and row['spec'].get('replicas', 1) != 1:
                    _blocked('rollout_extra_replica_baseline_unsupported')
                stage.snapshots[name] = copy.deepcopy(row)
            stage.original_snapshots = copy.deepcopy(stage.snapshots)
            # Only the typed fence projection is used for ownership and writer
            # checks. Historical image detail is reconstructed from snapshots.
            stage.old_images = _running_image_pins(kube, stage.original_snapshots)
            stage.original_live_census = stage._read_live_census()
            stage.journal({'kind': 'rollout_original_capture', 'snapshots': stage.original_snapshots,
                'marker': stage.marker, 'service': stage.service,
                'expected': expected, 'source_identity': identity, 'old_images': stage.old_images,
                'original_live_census_sha256': _sha(stage.original_live_census),
                'hub_pod_identity': stage.hub_pod_identity})
            stage.preflight()
            stage.original_snapshots = copy.deepcopy(stage.snapshots)
            return stage
        except Blocked:
            raise
        except (KubeAPIError, KeyError, TypeError, AttributeError, ValueError):
            _blocked('rollout_capture_invalid')

    def _read_live_census(self):
        pods = self.kube.list_hub_pods(namespace=NAMESPACE, labels={'app': HUB})
        active = [p for p in pods if not p.get('metadata', {}).get('deletionTimestamp')]
        if len(active) != 1:
            _blocked('rollout_original_hub_inventory_invalid')
        pod = active[0]
        pm = _meta(pod, namespace=NAMESPACE)
        node_name = pod.get('spec', {}).get('nodeName')
        if node_name and node_name != self.expected['source_node_name']:
            _blocked('rollout_original_hub_node_changed')
        containers = pod['spec'].get('containers', [])
        statuses = pod.get('status', {}).get('containerStatuses', [])
        owners = pm.get('ownerReferences', [])
        if (pod.get('status', {}).get('phase') != 'Running' or len(owners) != 1
            or owners[0].get('kind') != 'ReplicaSet' or owners[0].get('controller') is not True
            or len(containers) != 1 or containers[0].get('name') != 'nats'
            or containers[0].get('image') != NATS_IMAGE or len(statuses) != 1):
            _blocked('rollout_original_hub_inventory_invalid')
        self.hub_pod_identity = {'name': pm['name'], 'uid': pm['uid'],
            'resource_version': pm['resourceVersion'], 'node_name': node_name}
        pod_claims = [v.get('persistentVolumeClaim', {}).get('claimName')
            for v in pod['spec'].get('volumes', []) if 'persistentVolumeClaim' in v]
        if pod_claims != [self.expected['source_claim']]:
            _blocked('rollout_original_hub_claim_changed')
        rs = self.kube.get_replicaset(namespace=NAMESPACE, name=owners[0]['name'])
        rsm = _meta(rs, name=owners[0]['name'], namespace=NAMESPACE)
        if (rsm['uid'] != owners[0].get('uid') or not any(o.get('kind') == 'Deployment'
            and o.get('uid') == self.snapshots[HUB]['metadata']['uid']
            and o.get('controller') is True for o in rsm.get('ownerReferences', []))):
            _blocked('rollout_original_hub_owner_changed')
        node = self.kube.get_node(namespace=NAMESPACE,
            name=node_name or self.expected['source_node_name'])
        nm = _meta(node)
        if nm['uid'] != self.expected['source_node_uid'] or not any(
            c.get('type') == 'Ready' and c.get('status') == 'True'
            for c in node.get('status', {}).get('conditions', [])):
            _blocked('rollout_original_hub_node_not_ready')
        if (statuses[0].get('name') != 'nats' or statuses[0].get('ready') is not True
            or not re.fullmatch(r'containerd://[a-f0-9]{64}', statuses[0].get('containerID', ''))
            or statuses[0].get('imageID', '').removeprefix('docker-pullable://') not in
                (NATS_IMAGE.split('@', 1)[1], 'docker.io/library/nats@' + NATS_IMAGE.split('@', 1)[1])
            or type(statuses[0].get('restartCount')) is not int
            or set(statuses[0].get('state', {})) != {'running'}):
            _blocked('rollout_original_hub_container_changed')
        if self.account_id is None:
            _blocked('rollout_account_identity_required')
        raw = self.kube.exec_hub_jsz(namespace=NAMESPACE, pod_name=pm['name'], pod_uid=pm['uid'],
            container='nats', timeout_seconds=30)
        live = self.kube.get_pod(namespace=NAMESPACE, name=pm['name'])
        lm = _meta(live, name=pm['name'], namespace=NAMESPACE)
        live_statuses = live.get('status', {}).get('containerStatuses', [])
        if (lm['uid'] != pm['uid'] or live.get('metadata', {}).get('deletionTimestamp')
            or live.get('spec', {}).get('nodeName') != node_name
            or live_statuses != statuses or live.get('status', {}).get('phase') != 'Running'):
            _blocked('rollout_original_hub_changed')
        self.hub_pod_identity.update({'container_id': statuses[0]['containerID'],
            'image_id': statuses[0]['imageID'], 'restart_count': statuses[0]['restartCount']})
        if not isinstance(raw, (bytes, bytearray, str)):
            _blocked('rollout_original_live_census_invalid')
        # Keep the raw bounded JSZ result transient. Domain census code accepts
        # a tiny object implementing monitor_jsz; the durable journal stores a
        # digest-only evidence marker, never account credentials.
        from rollout_census import census
        tree = json.loads(raw if isinstance(raw, str) else bytes(raw).decode('utf-8'))

        class _Snapshot:
            def monitor_jsz(self, _name):
                return tree

        # Keep the complete census transient for independent archive
        # comparison. It is never copied into the journal; only digest/counts
        # cross the durable boundary.
        return census(_Snapshot(), 'original', self.account_id)

    def _fresh_marker(self):
        row = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
        meta = _meta(row, name=MARKER, namespace=NAMESPACE)
        if (meta['uid'] != self.marker['metadata']['uid']
            or meta['resourceVersion'] != self.marker['metadata']['resourceVersion']
            or row.get('data', {}).get('dataPVC') != self.active_claim):
            _blocked('rollout_marker_race')
        return row

    def _assert_source_identity(self):
        _identity, _claim, _pv, _node = _source_identity(self.kube,
            self.expected['source_claim'], expected=self.source_identity or self.expected)

    def _marker_cas(self, row, old_phase, new_phase, *, release=False):
        authority = getattr(self, 'finish_authority_guard', None)
        if authority is not None:
            authority()
        self._assert_source_identity()
        event = {'kind': 'rollout_marker_write_intent', 'uid': row['metadata']['uid'],
            'resource_version': row['metadata']['resourceVersion'], 'old_phase': old_phase,
            'new_phase': new_phase, 'claim': self.expected['source_claim'],
            'release': release}
        event['intent_id'] = _sha(event)
        self.journal(event)
        try:
            result = self.kube.cas_rollout_marker(name=MARKER, operation=self.operation,
                expected_uid=row['metadata']['uid'], expected_rv=row['metadata']['resourceVersion'],
                expected_phase=old_phase, new_phase=new_phase,
                expected_claim=self.expected['source_claim'], release=release)
        except KubeAPIError:
            # The request may have committed. Exact read-only reconciliation is
            # the only safe response; never replay a write on ambiguity.
            try:
                current = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
                md, data = _meta(current, name=MARKER, namespace=NAMESPACE), current['data']
                expected_op = None if release else self.operation
                if (md['uid'] == row['metadata']['uid'] and data.get('phase') == new_phase
                    and data.get('knownRolloutOperation') == expected_op
                    and data.get('dataPVC') == self.expected['source_claim']):
                    result = current
                else:
                    self.journal({'kind': 'rollout_marker_write_unknown',
                        'intent_id': event['intent_id']})
                    _blocked('rollout_marker_write_unknown')
            except (KubeAPIError, KeyError, TypeError, AttributeError, ValueError):
                self.journal({'kind': 'rollout_marker_write_unknown',
                    'intent_id': event['intent_id']})
                _blocked('rollout_marker_write_unknown')
        self.marker = copy.deepcopy(result)
        self.journal({'kind': 'rollout_marker_write_complete', 'marker': self.marker,
            'new_phase': new_phase, 'intent_id': event['intent_id']})
        self.active_claim = self.expected['source_claim']
        return self.marker

    def preflight(self):
        ns = self.kube.get_namespace(name=NAMESPACE)
        marker = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
        nm, mm = _meta(ns, name=NAMESPACE), _meta(marker, name=MARKER, namespace=NAMESPACE)
        if nm['uid'] != self.expected['namespace_uid'] or mm['uid'] != self.expected['marker_uid']:
            _blocked('staging_identity_changed')
        data = marker['data']
        if data.get('generation') != self.expected['generation']:
            _blocked('staging_marker_changed')
        if data.get('phase') == 'active':
            if any(k in data for k in ('knownBaselineOperation', 'knownRolloutOperation')):
                _blocked('rollout_active_marker_required')
        elif (data.get('phase') not in _PHASES
            or data.get('knownRolloutOperation') != self.operation):
            _blocked('rollout_ownership_changed')
        self.marker = copy.deepcopy(marker)
        for name, captured in tuple(self.snapshots.items()):
            row = self.kube.get_deployment(namespace=NAMESPACE, name=name)
            meta = _meta(row, name=name, namespace=NAMESPACE)
            if meta['uid'] != self.expected['deployment_uids'][name]:
                _blocked('staging_deployment_changed')
            if data.get('phase') == 'active':
                if row['spec'].get('replicas', 1) != 1:
                    _blocked('staging_replica_baseline_changed')
                if row['spec']['template'] != captured['spec']['template']:
                    _blocked('staging_template_changed_during_proof')
            self.snapshots[name] = copy.deepcopy(row)
        if _hub_claim(self.snapshots[HUB]) != self.expected['source_claim']:
            _blocked('staging_claim_changed')
        identity, claim, pv, node = _source_identity(self.kube, self.expected['source_claim'],
            expected=self.source_identity)
        self.source_identity = identity
        self.source_claim = claim
        controls = _control_rows(self.kube)
        for row in controls:
            kind = row.get('kind')
            meta = row.get('metadata', {})
            if kind == 'HorizontalPodAutoscaler' and meta.get('name') in self.snapshots:
                if row.get('spec', {}).get('scaleTargetRef', {}).get('name') in self.snapshots:
                    _blocked('fenced_workload_autoscaler_present')
            if kind == 'Job' and row.get('status', {}).get('active', 0) and 'nats' in meta.get('name', ''):
                _blocked('active_nats_job_present')
            if kind == 'Deployment' and meta.get('name') not in self.snapshots and row.get('spec', {}).get('replicas', 1):
                images = [c.get('image') for c in row.get('spec', {}).get('template', {}).get('spec', {}).get('containers', [])]
                if NATS_IMAGE in images:
                    _blocked('alternate_nats_hub_present')
        service = self.kube.get_service(namespace=NAMESPACE, name=SERVICE)
        if self.service is not None and service != self.service:
            _blocked('nats_service_changed')
        self.service = copy.deepcopy(service)
        if data.get('phase') == 'active':
            self.journal({'kind': 'staging_preflight', 'source_claim': self.expected['source_claim'],
                'source_claim_uid': self.expected['source_claim_uid'],
                'replicas': {n: r['spec'].get('replicas', 1) for n, r in self.snapshots.items()}})
        return {'verified': True}

    def maintenance(self):
        row = self._fresh_marker()
        if row['data'].get('phase') != 'active':
            _blocked('rollout_maintenance_phase_invalid')
        return self._marker_cas(row, 'active', 'rollout-capturing')

    def owned_marker(self):
        row = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
        meta, data = _meta(row, name=MARKER, namespace=NAMESPACE), row.get('data', {})
        if (meta['uid'] != self.marker['metadata']['uid']
            or meta['resourceVersion'] != self.marker['metadata']['resourceVersion']
            or data.get('knownRolloutOperation') != self.operation
            or data.get('phase') not in _PHASES
            or data.get('dataPVC') != self.active_claim):
            _blocked('rollout_ownership_changed')
        return row

    def prepared(self):
        self.verify_final_storage()
        row = self.owned_marker()
        if row['data'].get('phase') != 'rollout-capturing':
            _blocked('rollout_prepare_phase_invalid')
        return self._marker_cas(row, 'rollout-capturing', 'rollout-prepared')

    def verified(self):
        self.verify_final_storage()
        row = self.owned_marker()
        if row['data'].get('phase') == 'rollout-verified':
            self.marker = copy.deepcopy(row)
            return
        if row['data'].get('phase') != 'rollout-applying':
            _blocked('rollout_verify_phase_invalid')
        return self._marker_cas(row, 'rollout-applying', 'rollout-verified')

    def verify_selected_storage_identity(self):
        if self.active_claim != self.expected['source_claim']:
            _blocked('rollout_claim_selection_forbidden')
        identity, claim, pv, node = _source_identity(self.kube, self.expected['source_claim'],
            expected=self.source_identity or self.expected)
        namespace = self.kube.get_namespace(name=NAMESPACE)
        marker = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
        data = marker.get('data', {})
        if (_meta(namespace, name=NAMESPACE)['uid'] != self.expected['namespace_uid']
            or marker['metadata']['uid'] != self.expected['marker_uid']
            or data.get('generation') != self.expected['generation']
            or data.get('dataPVC') != self.active_claim):
            _blocked('rollout_marker_race')
        selected_identity = identity
        if self.active_claim != self.expected['source_claim']:
            selected_identity, _selected_claim, _selected_pv, _selected_node = _source_identity(
                self.kube, self.active_claim, expected={
                    'source_claim_name': self.final_identity['claim_name'],
                    'source_claim_uid': self.final_identity['claim_uid'],
                    'source_claim_resource_version': self.final_identity['claim_resource_version'],
                    'source_pv_name': self.final_identity['pv_name'],
                    'source_pv_uid': self.final_identity['pv_uid'],
                    'source_pv_resource_version': self.final_identity['pv_resource_version'],
                    'source_node_name': self.final_identity['node_name'],
                    'source_node_uid': self.final_identity['node_uid'],
                    'source_node_resource_version': self.final_identity['node_resource_version'],
                    'source_node_hostname': self.final_identity['node_hostname'],
                })
        pods = self.kube.list_hub_pods(namespace=NAMESPACE, labels={'app': HUB})
        live = [p for p in pods if not p.get('metadata', {}).get('deletionTimestamp')]
        if data.get('phase') == 'active':
            if len(live) != 1:
                _blocked('rollout_source_hub_pod_invalid')
        elif (data.get('knownRolloutOperation') != self.operation or data.get('phase') not in _PHASES
            or (data.get('phase') != 'rollout-verified' and live) or len(live) > 1):
            _blocked('rollout_source_hub_pod_invalid')
        for pod in live:
            claims = [v.get('persistentVolumeClaim', {}).get('claimName')
                for v in pod.get('spec', {}).get('volumes', []) if 'persistentVolumeClaim' in v]
            owners = pod.get('metadata', {}).get('ownerReferences', [])
            if (claims != [self.expected['source_claim']]
                or len(owners) != 1 or owners[0].get('kind') != 'ReplicaSet'
                or owners[0].get('controller') is not True):
                _blocked('rollout_source_hub_pod_changed')
            if (pod.get('spec', {}).get('nodeName') not in (None, '', selected_identity['source_node_name'])
                or not _UID.fullmatch(pod.get('metadata', {}).get('uid', ''))):
                _blocked('rollout_source_hub_pod_changed')
            owner = owners[0]
            rs = self.kube.get_replicaset(namespace=NAMESPACE, name=owner.get('name'))
            rm = _meta(rs, name=owner.get('name'), namespace=NAMESPACE)
            if (rm['uid'] != owner.get('uid') or not any(
                ref.get('kind') == 'Deployment' and ref.get('uid') == self.snapshots[HUB]['metadata']['uid']
                and ref.get('controller') is True for ref in rm.get('ownerReferences', []))):
                _blocked('rollout_source_hub_owner_changed')
        self.source_identity = identity
        self.source_claim = claim
        return identity

    def verify_final_storage(self):
        self.verify_closed()
        return self.verify_selected_storage_identity()

    def bind_final_storage(self, claim, pv):
        _blocked('rollout_pvc_allocation_forbidden')

    def new_claim(self, *args, **kwargs):
        _blocked('rollout_pvc_allocation_forbidden')

    def _fresh_final_identity(self):
        if self.final_claim is None:
            _blocked('rollout_final_claim_missing')
        identity, _claim, _pv, _node = _source_identity(self.kube,
            self.final_claim['metadata']['name'])
        return {
            'claim_name': identity['source_claim_name'],
            'claim_uid': identity['source_claim_uid'],
            'claim_resource_version': identity['source_claim_resource_version'],
            'pv_name': identity['source_pv_name'],
            'pv_uid': identity['source_pv_uid'],
            'pv_resource_version': identity['source_pv_resource_version'],
            'node_name': identity['source_node_name'],
            'node_uid': identity['source_node_uid'],
            'node_resource_version': identity['source_node_resource_version'],
            'node_hostname': identity['source_node_hostname'],
        }

    def select_claim(self):
        _blocked('rollout_pvc_selection_forbidden')

    def _fresh_final_identity_check(self):
        return self.verify_selected_storage_identity()

    def _check_fenced(self, names):
        self.owned_marker()
        for name in names:
            row = self.kube.get_deployment(namespace=NAMESPACE, name=name)
            old = self.snapshots[name]
            if (_meta(row)['uid'] != old['metadata']['uid']
                or row['spec'].get('replicas', 1) != 0
                or row['spec']['template'] != old['spec']['template']):
                _blocked('fenced_workload_changed')
        objects = _fence_rows(self.kube)
        claims = {self.expected['source_claim']}
        if self.final_claim is not None:
            claims.add(self.final_claim['metadata']['name'])
        pods_by_rs = set()
        deployment_uids = {self.snapshots[n]['metadata']['uid'] for n in names}
        rs_uids = {r['metadata']['uid'] for r in objects if r.get('kind') == 'ReplicaSet'
            and any(o.get('uid') in deployment_uids for o in r.get('metadata', {}).get('ownerReferences', []))}
        for row in objects:
            if row.get('kind') != 'Pod':
                continue
            spec = row.get('spec', {})
            if any(v.get('persistentVolumeClaim', {}).get('claimName') in claims for v in spec.get('volumes', [])):
                _blocked('rollout_source_writer_present')
            if any(o.get('uid') in rs_uids for o in row.get('metadata', {}).get('ownerReferences', [])):
                pods_by_rs.add(row['metadata']['uid'])
        if pods_by_rs:
            _blocked('rollout_owned_pods_remain')
        return objects

    def no_pods(self, names, timeout=90):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            self._check_fenced(names)
            return
        _blocked('staging_fence_timeout')

    def competing_controllers(self):
        for row in _control_rows(self.kube):
            kind, meta = row.get('kind'), row.get('metadata', {})
            if kind == 'HorizontalPodAutoscaler' and row.get('spec', {}).get('scaleTargetRef', {}).get('name') in self.snapshots:
                _blocked('fenced_workload_autoscaler_present')
            if kind == 'Job' and row.get('status', {}).get('active', 0) and 'nats' in meta.get('name', ''):
                _blocked('active_nats_job_present')

    def verify_closed(self):
        names = tuple(self.snapshots)
        self.owned_marker()
        self.competing_controllers()
        self._check_fenced(names)
        if self.kube.get_service(namespace=NAMESPACE, name=SERVICE) != self.service:
            _blocked('nats_service_changed')
        return {'verified': True, 'hub_zero_pods': True}

    def fence(self):
        self.maintenance()
        names = tuple(n for n in self.snapshots if n != HUB)
        for name in names:
            self._scale(name, 0)
            self.journal({'kind': 'workload_fenced', 'name': name, 'replicas': 0})
        self._drain_clients(names)
        self.shutdown_original_hub()
        all_names = names + (HUB,)
        self.no_pods(all_names)
        self.verify_closed()
        return {'verified': True, 'workloads': list(all_names), 'hub_zero_pods': True}

    def _scale(self, name, replicas):
        authority = getattr(self, 'finish_authority_guard', None)
        if authority is not None and replicas != 0:
            authority()
        finish_guard = getattr(self, 'finish_guard', None)
        if finish_guard is not None and replicas != 0:
            finish_guard('scale', name, replicas)
        release = getattr(self, 'closed_preservation_release', None)
        if release is not None and replicas != 0:
            release(self, 'scale-intent', name)
        self._assert_source_identity()
        self.owned_marker()
        row = self.kube.get_deployment(namespace=NAMESPACE, name=name)
        old = self.snapshots[name]
        digest = _sha(row['spec']['template'])
        if (_meta(row)['uid'] != old['metadata']['uid'] or digest != _sha(old['spec']['template'])):
            _blocked('staging_workload_race')
        event = {'kind': 'rollout_deployment_write_intent', 'name': name,
            'uid': old['metadata']['uid'], 'resource_version': row['metadata']['resourceVersion'],
            'replicas': replicas, 'template_sha256': digest}
        event['intent_id'] = _sha(event)
        self.journal(event)
        try:
            result = self.kube.cas_rollout_deployment(namespace=NAMESPACE, name=name,
                expected_uid=old['metadata']['uid'],
                expected_rv=row['metadata']['resourceVersion'], expected_replicas=row['spec'].get('replicas', 1),
                expected_template_sha256=digest, new_replicas=replicas,
                expected_jsdata_claim=self.expected['source_claim'] if name == HUB else None)
        except KubeAPIError:
            self._assert_source_identity()
            self.owned_marker()
            current = self.kube.get_deployment(namespace=NAMESPACE, name=name)
            if (_meta(current)['uid'] == old['metadata']['uid']
                and current['spec'].get('replicas', 1) == replicas
                and _sha(current['spec']['template']) == digest):
                result = current
            else:
                self.journal({'kind': 'rollout_deployment_write_unknown', 'name': name,
                    'uid': old['metadata']['uid'], 'replicas': replicas})
                _blocked('rollout_deployment_write_unknown')
        self.snapshots[name] = copy.deepcopy(result)
        self.journal({'kind': 'workload_scaled', 'name': name, 'replicas': replicas,
            'intent_id': event['intent_id'],
            'uid': result['metadata']['uid'], 'resource_version': result['metadata']['resourceVersion']})
        if release is not None and replicas != 0:
            release(self, 'scale-result', name)

    def scale(self, name, replicas):
        """Compatibility entry point; the API still restricts the exact CAS."""
        return self._scale(name, replicas)

    def _drain_clients(self, names, timeout=90):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            self.owned_marker()
            try:
                objects = _fence_rows(self.kube)
                wanted = {self.snapshots[n]['metadata']['uid'] for n in names}
                rs_uids = {r['metadata']['uid'] for r in objects if r.get('kind') == 'ReplicaSet'
                    and any(o.get('uid') in wanted for o in r.get('metadata', {}).get('ownerReferences', []))}
                active = [p for p in objects if p.get('kind') == 'Pod'
                    and any(o.get('uid') in rs_uids for o in p.get('metadata', {}).get('ownerReferences', []))]
                if not active:
                    return
            except KubeAPIError:
                _blocked('rollout_client_fence_read_failed')
            time.sleep(0.2)
        _blocked('rollout_client_fence_timeout')

    def shutdown_original_hub(self):
        # This high-risk exit is intentionally based only on UID-bound Pod and
        # fixed finalizer/delete endpoints. The typed API owns all payload shape.
        pods = self.kube.list_hub_pods(namespace=NAMESPACE, labels={'app': HUB})
        active = [p for p in pods if not p.get('metadata', {}).get('deletionTimestamp')]
        if len(active) != 1:
            _blocked('rollout_original_hub_inventory_invalid')
        pod = active[0]
        pm = _meta(pod, namespace=NAMESPACE)
        captured_pod = self.hub_pod_identity
        if (not isinstance(captured_pod, Mapping) or captured_pod.get('uid') != pm['uid']
            or captured_pod.get('name') != pm['name']
            or pod.get('spec', {}).get('nodeName') != self.expected['source_node_name']):
            _blocked('rollout_original_hub_identity_changed')
        self.owned_marker()
        statuses = pod.get('status', {}).get('containerStatuses', [])
        containers = pod.get('spec', {}).get('containers', [])
        if (pod.get('status', {}).get('phase') != 'Running' or len(containers) != 1
            or containers[0].get('name') != 'nats' or containers[0].get('image') != NATS_IMAGE
            or len(statuses) != 1 or statuses[0].get('name') != 'nats'
            or statuses[0].get('ready') is not True
            or statuses[0].get('containerID') != captured_pod.get('container_id')
            or statuses[0].get('imageID') != captured_pod.get('image_id')
            or statuses[0].get('restartCount') != captured_pod.get('restart_count')):
            _blocked('rollout_original_hub_container_changed')
        finalizer = 'voice.io/nats-preservation-' + self.operation
        finals = pm.get('finalizers', [])
        if finalizer in finals:
            _blocked('rollout_original_hub_finalizer_already_present')
        add_intent = {'kind': 'original_hub_finalizer_add_intent', 'pod_name': pm['name'],
            'pod_uid': pm['uid'], 'resource_version': pm['resourceVersion'],
            'expected_finalizers': list(finals), 'finalizer': finalizer,
            'live_census_sha256': hashlib.sha256(json.dumps(self.original_live_census,
                sort_keys=True, separators=(',', ':')).encode()).hexdigest()}
        add_intent['intent_id'] = _sha(add_intent)
        self._assert_source_identity()
        self.owned_marker()
        self.journal(add_intent)
        try:
            self.kube.add_owned_pod_finalizer(namespace=NAMESPACE, name=pm['name'], pod_uid=pm['uid'],
                expected_rv=pm['resourceVersion'], expected_finalizers=finals, finalizer=finalizer)
        except KubeAPIError:
            self._assert_source_identity()
            self.owned_marker()
            live = self.kube.get_pod(namespace=NAMESPACE, name=pm['name'])
            lm = _meta(live, name=pm['name'], namespace=NAMESPACE)
            if lm['uid'] == pm['uid'] and finalizer in lm.get('finalizers', []):
                pass
            else:
                self.journal({'kind': 'original_hub_finalizer_add_unknown',
                    'intent_id': add_intent['intent_id']})
                _blocked('rollout_original_hub_finalizer_add_unknown')
        self.journal({'kind': 'original_hub_finalizer_add_complete',
            'intent_id': add_intent['intent_id'], 'pod_uid': pm['uid']})
        self._scale(HUB, 0)
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            self.owned_marker()
            live = self.kube.get_pod(namespace=NAMESPACE, name=pm['name'])
            lm = _meta(live, name=pm['name'], namespace=NAMESPACE)
            if lm['uid'] != pm['uid'] or finalizer not in lm.get('finalizers', []):
                _blocked('rollout_original_hub_retained_pod_changed')
            status = live.get('status', {}).get('containerStatuses', [])
            if len(status) != 1 or status[0].get('containerID') != statuses[0].get('containerID'):
                _blocked('rollout_original_hub_container_changed')
            if 'terminated' in status[0].get('state', {}):
                if (status[0].get('restartCount') != statuses[0].get('restartCount')
                    or status[0].get('containerID') != statuses[0].get('containerID')):
                    _blocked('rollout_original_hub_container_changed')
                remove_intent = {'kind': 'original_hub_finalizer_remove_intent',
                    'pod_name': pm['name'], 'pod_uid': pm['uid'],
                    'resource_version': lm['resourceVersion'],
                    'expected_finalizers': list(lm['finalizers']), 'finalizer': finalizer}
                remove_intent['intent_id'] = _sha(remove_intent)
                self._assert_source_identity()
                self.owned_marker()
                self.journal(remove_intent)
                try:
                    self.kube.remove_owned_pod_finalizer(namespace=NAMESPACE, name=pm['name'],
                        pod_uid=pm['uid'], expected_rv=lm['resourceVersion'],
                        expected_finalizers=lm['finalizers'], finalizer=finalizer)
                except KubeAPIError:
                    self._assert_source_identity()
                    self.owned_marker()
                    after = self.kube.get_pod(namespace=NAMESPACE, name=pm['name'])
                    am = _meta(after, name=pm['name'], namespace=NAMESPACE)
                    if am['uid'] == pm['uid'] and finalizer not in am.get('finalizers', []):
                        pass
                    else:
                        self.journal({'kind': 'original_hub_finalizer_remove_unknown',
                            'intent_id': remove_intent['intent_id']})
                        _blocked('rollout_original_hub_finalizer_remove_unknown')
                self.journal({'kind': 'original_hub_finalizer_remove_complete',
                    'intent_id': remove_intent['intent_id'], 'pod_uid': pm['uid']})
                self.journal({'kind': 'original_hub_owned_finalizer_removed', 'pod_uid': pm['uid']})
                return
            time.sleep(0.2)
        _blocked('rollout_original_hub_shutdown_timeout')

    def refence(self):
        released = getattr(self, 'finish_released_marker', None)
        if released is not None:
            current = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
            if (current != released or current.get('data', {}).get('phase') != 'active'
                or current.get('data', {}).get('knownRolloutOperation') is not None
                or current.get('data', {}).get('dataPVC') != self.expected['source_claim']):
                _blocked('finish_released_marker_changed')
            self.verify_selected_storage_identity()
            self.marker = copy.deepcopy(current)
            self._marker_cas(current, 'active', 'rollout-capturing')
            del self.finish_released_marker
            self.journal({'kind': 'expired_finish_release_refenced', 'marker': self.marker})
        self.owned_marker()
        names = tuple(self.snapshots)
        for name in names:
            self._scale(name, 0)
        self.no_pods(names)
        self.verify_closed()
        return {'verified': True, 'hub_zero_pods': True}

    def adopt_applied_target(self, receipt, claim_rv):
        row = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
        meta, data = _meta(row), row['data']
        if (meta['uid'] != self.marker['metadata']['uid'] or meta['resourceVersion'] != claim_rv
            or data.get('phase') != 'rollout-applying' or data.get('knownRolloutOperation') != self.operation):
            _blocked('rollout_apply_owner_changed')
        replacement = {}
        hashes = receipt['target']['template_hashes']
        for name, old in self.snapshots.items():
            current = self.kube.get_deployment(namespace=NAMESPACE, name=name)
            wanted = hashes.get(name, _sha(old['spec']['template']))
            if (_meta(current)['uid'] != old['metadata']['uid'] or current['spec'].get('replicas', 1) != 0
                or _sha(current['spec']['template']) != wanted):
                _blocked('rollout_applied_target_changed')
            replacement[name] = current
        self.marker, self.snapshots = row, replacement
        self.verify_final_storage()
        self.journal({'kind': 'rollout_target_observed', 'snapshots': replacement, 'marker': row})

    def adopt_rollback_target(self, receipt, claim_rv):
        row = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
        meta, data = _meta(row), row['data']
        if (meta['uid'] != self.marker['metadata']['uid'] or meta['resourceVersion'] != claim_rv
            or data.get('phase') != 'rollout-applying' or data.get('knownRolloutOperation') != self.operation):
            _blocked('rollout_rollback_owner_changed')
        replacement = {}
        hashes = receipt['target']['template_hashes']
        for name, old in self.original_snapshots.items():
            current = self.kube.get_deployment(namespace=NAMESPACE, name=name)
            allowed = {_sha(old['spec']['template']), hashes.get(name, _sha(old['spec']['template']))}
            if (_meta(current)['uid'] != old['metadata']['uid'] or current['spec'].get('replicas', 1) != 0
                or _sha(current['spec']['template']) not in allowed):
                _blocked('rollout_rollback_workload_changed')
            replacement[name] = current
        self.marker, self.snapshots = row, replacement
        self.verify_final_storage()
        self.journal({'kind': 'rollout_partial_target_observed'})

    def restore_original_templates(self):
        self.verify_final_storage()
        for name, snapshot in self.snapshots.items():
            current = self.kube.get_deployment(namespace=NAMESPACE, name=name)
            if (_meta(current)['uid'] != snapshot['metadata']['uid'] or current['spec'].get('replicas', 1) != 0
                or current['spec']['template'] != snapshot['spec']['template']):
                _blocked('rollout_rollback_workload_changed')
        for name, original in self.original_snapshots.items():
            if name == HUB:
                continue
            self.verify_final_storage()
            current, snapshot = self.kube.get_deployment(namespace=NAMESPACE, name=name), self.snapshots[name]
            if (_meta(current)['uid'] != snapshot['metadata']['uid'] or current['spec'].get('replicas', 1) != 0
                or current['spec']['template'] != snapshot['spec']['template']):
                _blocked('rollout_rollback_workload_changed')
            template = copy.deepcopy(original['spec']['template'])
            for container in template['spec'].get('initContainers', []) + template['spec'].get('containers', []):
                image = self.old_images.get(name + '/' + container['name'], '')
                if not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}@sha256:[a-f0-9]{64}', image):
                    _blocked('rollout_rollback_image_unpinned')
                container['image'] = image
            authority = getattr(self, 'finish_authority_guard', None)
            if authority is not None:
                authority()
            intent = {'kind': 'rollout_template_restore_intent', 'name': name,
                'uid': current['metadata']['uid'], 'resource_version': current['metadata']['resourceVersion'],
                'replicas': 0, 'expected_template_sha256': _sha(current['spec']['template']),
                'template_sha256': _sha(template)}
            intent['intent_id'] = _sha(intent)
            self._assert_source_identity()
            self.owned_marker()
            self.journal(intent)
            try:
                result = self.kube.cas_rollout_deployment(namespace=NAMESPACE, name=name,
                    expected_uid=current['metadata']['uid'],
                    expected_rv=current['metadata']['resourceVersion'], expected_replicas=0,
                    expected_template_sha256=_sha(current['spec']['template']), new_template=template,
                    expected_jsdata_claim=self.expected['source_claim'] if name == HUB else None)
            except KubeAPIError:
                self._assert_source_identity()
                self.owned_marker()
                after = self.kube.get_deployment(namespace=NAMESPACE, name=name)
                if (_meta(after)['uid'] == current['metadata']['uid']
                    and after['spec'].get('replicas', 1) == 0
                    and _sha(after['spec']['template']) == intent['template_sha256']):
                    result = after
                else:
                    self.journal({'kind': 'rollout_template_restore_unknown',
                        'intent_id': intent['intent_id']})
                    _blocked('rollout_template_restore_unknown')
            self.snapshots[name] = result
            self.journal({'kind': 'rollout_template_restore_complete',
                'intent_id': intent['intent_id'], 'name': name,
                'template_sha256': intent['template_sha256']})
        self.journal({'kind': 'rollout_original_templates_restored'})

    def restart(self):
        if self.owned_marker()['data'].get('phase') != 'rollout-verified':
            _blocked('rollout_unverified_resume_forbidden')
        self.seal_before_business_resume()
        self.verify_selected_storage_identity()
        self.no_pods(tuple(self.snapshots))
        claim = self.kube.get_pvc(namespace=NAMESPACE, name=self.expected['source_claim'])
        if _meta(claim)['uid'] != self.expected['source_claim_uid']:
            _blocked('restart_source_claim_changed')
        self._scale(HUB, 1)
        self._wait_ready(HUB)
        self.verify_selected_storage_identity()

        # Reproduce the reviewed User/Space bootstrap order with a fixed API
        # action. Restore the exact target template captured before restart.
        original = copy.deepcopy(self.snapshots['voice-user']['spec']['template'])
        user = self.kube.get_deployment(namespace=NAMESPACE, name='voice-user')
        um = _meta(user, name='voice-user', namespace=NAMESPACE)
        if (um['uid'] != self.snapshots['voice-user']['metadata']['uid']
            or user['spec'].get('replicas', 1) != 0
            or user['spec']['template'] != original):
            _blocked('user_cycle_shape_invalid')
        config = self.kube.get_configmap(namespace=NAMESPACE, name='voice-app-config')
        if not isinstance(config.get('data'), dict) or 'SPACE_GRPC_ADDR' not in config['data']:
            _blocked('user_cycle_config_missing')
        override = self._user_bootstrap_override(original)
        self._user_template_cas('install', original, override)
        for service in LEAVES:
            self._scale('voice-' + service, 1)
        self._wait_ready('voice-user')
        self._wait_ready('voice-space')
        self.owned_marker()
        self._user_template_cas('restore', override, original)
        self._wait_ready('voice-user')
        for service in LEAVES:
            if service not in ('user', 'space'):
                self._wait_ready('voice-' + service)
        self._scale('voice-gateway', 1)
        self._wait_ready('voice-gateway')
        self.verify_selected_storage_identity()
        self.release_marker()
        return {'verified': True, 'active_claim': self.active_claim,
            'generation': self.expected['generation'], 'user_cycle_restored': True}

    def _user_bootstrap_override(self, template):
        result = copy.deepcopy(template)
        try:
            spec = result['spec']
            containers = spec['containers']
            indexes = [i for i, row in enumerate(containers) if row.get('name') == 'user']
            annotations = result['metadata'].get('annotations')
            env = containers[indexes[0]].get('env') if len(indexes) == 1 else None
            if (not isinstance(env, list) or not isinstance(annotations, dict)
                or any(row.get('name') == 'SPACE_GRPC_ADDR' for row in env)
                or 'voice.io/nats-user-space-bootstrap' in annotations):
                _blocked('user_cycle_already_owned')
            env.append({'name': 'SPACE_GRPC_ADDR', 'value': ''})
            annotations['voice.io/nats-user-space-bootstrap'] = self.operation
            return result
        except Blocked:
            raise
        except (KeyError, TypeError, AttributeError, IndexError):
            _blocked('user_cycle_shape_invalid')

    def _user_template_cas(self, action, expected_template, target_template):
        row = self.kube.get_deployment(namespace=NAMESPACE, name='voice-user')
        old = self.snapshots['voice-user']
        meta = _meta(row, name='voice-user', namespace=NAMESPACE)
        expected_replicas = 0 if action == 'install' else 1
        if (meta['uid'] != old['metadata']['uid']
            or row['spec'].get('replicas', 1) != expected_replicas
            or row['spec']['template'] != expected_template):
            _blocked('user_cycle_workload_changed')
        intent = {'kind': 'rollout_user_template_write_intent', 'action': action,
            'uid': meta['uid'], 'resource_version': meta['resourceVersion'],
            'replicas': expected_replicas,
            'expected_template_sha256': _sha(expected_template),
            'new_template_sha256': _sha(target_template)}
        intent['intent_id'] = _sha(intent)
        authority = getattr(self, 'finish_authority_guard', None)
        if authority is not None:
            authority()
        self._assert_source_identity()
        self.owned_marker()
        self.journal(intent)
        try:
            result = self.kube.cas_user_bootstrap_template(namespace=NAMESPACE,
                name='voice-user', expected_uid=meta['uid'], expected_rv=meta['resourceVersion'],
                expected_replicas=expected_replicas, expected_template_sha256=_sha(expected_template),
                operation=self.operation,
                original_template=target_template if action == 'restore' else None,
                action=action)
        except KubeAPIError:
            self._assert_source_identity()
            self.owned_marker()
            current = self.kube.get_deployment(namespace=NAMESPACE, name='voice-user')
            if (_meta(current)['uid'] == meta['uid']
                and current['spec'].get('replicas', 1) == expected_replicas
                and current['spec']['template'] == target_template):
                result = current
            else:
                self.journal({'kind': 'rollout_user_template_write_unknown',
                    'intent_id': intent['intent_id']})
                _blocked('rollout_user_template_write_unknown')
        self.snapshots['voice-user'] = copy.deepcopy(result)
        self.journal({'kind': 'rollout_user_template_write_complete',
            'intent_id': intent['intent_id'], 'template_sha256': _sha(target_template)})

    def _wait_ready(self, name, timeout=300):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            self.owned_marker()
            result = self.kube.get_deployment_ready(name=name,
                expected_uid=self.snapshots[name]['metadata']['uid'])
            if result:
                if name == HUB and getattr(self, 'renderer_transition', None) is not None:
                    post_start = getattr(self, 'renderer_post_start', None)
                    if not callable(post_start):
                        _blocked('renderer_restart_proof_missing')
                    post_start(self)
                    self.journal({'kind': 'rollout_renderer_live_verified'})
                finish_guard = getattr(self, 'finish_guard', None)
                if finish_guard is not None:
                    finish_guard('ready', name, None)
                release = getattr(self, 'closed_preservation_release', None)
                if release is not None:
                    release(self, 'ready', name)
                return
            time.sleep(0.5)
        _blocked('restart_readiness_timeout')

    def wait_ready(self, name, timeout=300):
        return self._wait_ready(name, timeout)

    def release_marker(self):
        row = self.owned_marker()
        if row['data'].get('phase') != 'rollout-verified':
            _blocked('rollout_release_phase_invalid')
        core = {HUB, 'voice-gateway', *('voice-' + service for service in LEAVES)}
        for name in sorted(set(self.snapshots) - core):
            ledger = getattr(self, 'finish_ledger', None)
            if ledger is None or name in ledger.record['pending']:
                self._scale(name, 1)
            self._wait_ready(name)
        self.verify_selected_storage_identity()
        row = self.owned_marker()
        finish_guard = getattr(self, 'finish_guard', None)
        if finish_guard is not None:
            finish_guard('release', None, None)
        release = getattr(self, 'closed_preservation_release', None)
        if release is not None:
            release(self, 'release-intent')
        self._marker_cas(row, 'rollout-verified', 'active', release=True)
        if getattr(self, 'finish_authority_guard', None) is not None:
            self.finish_released_marker = copy.deepcopy(self.marker)
        self.journal({'kind': 'rollout_released', 'marker': self.marker})
        if release is not None:
            release(self, 'release-result')
        final_guard = getattr(self, 'finish_authority_guard', None)
        if final_guard is not None:
            final_guard()

    def seal_before_business_resume(self):
        producer = getattr(self, 'closed_preservation_producer', None)
        if producer is None:
            _blocked('closed_preservation_seal_required')
        result = producer.seal_before_resume(self)
        runtime = getattr(self, 'closed_preservation_runtime', None)
        if runtime is None:
            _blocked('closed_preservation_release_source_required')
        runtime.arm_release(self, result)
        authority = getattr(self, 'finish_authority_guard', None)
        if authority is not None:
            authority()
        return result

    def restart_missing(self, ledger):
        """Resume only this fresh proof's exact pending readiness obligations."""
        if self.owned_marker()['data'].get('phase') != 'rollout-verified':
            _blocked('rollout_unverified_resume_forbidden')
        pending = ledger.record.get('pending')
        completed = ledger.record.get('completed')
        if (not isinstance(pending, Mapping) or not isinstance(completed, Mapping)
            or set(pending) | set(completed) != set(self.snapshots)
            or set(pending) & set(completed)):
            _blocked('finish_resume_inventory_changed')
        self.finish_ledger = ledger
        self.seal_before_business_resume()

        def resume(name):
            if name in pending:
                row = self.kube.get_deployment(namespace=NAMESPACE, name=name)
                if row['spec'].get('replicas', 1) != 0:
                    _blocked('finish_resume_not_paused')
                self._scale(name, 1)
            self._wait_ready(name)

        resume(HUB)
        user_name, space_name, original = 'voice-user', 'voice-space', None
        if user_name in pending and space_name in pending:
            user = self.kube.get_deployment(namespace=NAMESPACE, name=user_name)
            original = copy.deepcopy(user['spec']['template'])
            override = self._user_bootstrap_override(original)
            config = self.kube.get_configmap(namespace=NAMESPACE, name='voice-app-config')
            if 'SPACE_GRPC_ADDR' not in config.get('data', {}):
                _blocked('user_cycle_config_missing')
            authority = getattr(self, 'finish_authority_guard', None)
            if authority is not None:
                authority()
            self.journal({'kind': 'expired_finish_cycle_entered', 'name': user_name,
                'original_template_sha256': _sha(original)})
            self._user_template_cas('install', original, override)
            self.journal({'kind': 'user_cycle_override', 'owner': self.operation})
        for service in LEAVES:
            name = 'voice-' + service
            if name in pending:
                row = self.kube.get_deployment(namespace=NAMESPACE, name=name)
                if row['spec'].get('replicas', 1) != 0:
                    _blocked('finish_resume_not_paused')
                self._scale(name, 1)
        self._wait_ready(user_name)
        self._wait_ready(space_name)
        if original is not None:
            authority = getattr(self, 'finish_authority_guard', None)
            if authority is not None:
                authority()
            user = self.kube.get_deployment(namespace=NAMESPACE, name=user_name)
            override = copy.deepcopy(user['spec']['template'])
            self._user_template_cas('restore', override, original)
            self.journal({'kind': 'user_cycle_restored', 'owner': self.operation})
            if authority is not None:
                authority()
            self._wait_ready(user_name)
        for service in LEAVES:
            self._wait_ready('voice-' + service)
        resume('voice-gateway')
        self.release_marker()
        return {'verified': True, 'active_claim': self.expected['source_claim'],
            'generation': self.expected['generation'], 'user_cycle_restored': True,
            'finish_only': True}

    def recover(self):
        """Read-only reconciliation for every journaled incomplete write."""
        intents = {}
        completed = set()
        for event in self.history:
            kind = event.get('kind')
            if kind in ('rollout_marker_write_intent', 'rollout_deployment_write_intent',
                'rollout_template_restore_intent', 'rollout_user_template_write_intent',
                'original_hub_finalizer_add_intent', 'original_hub_finalizer_remove_intent'):
                intent_id = event.get('intent_id')
                if not isinstance(intent_id, str) or not re.fullmatch(r'[a-f0-9]{64}', intent_id):
                    _blocked('rollout_recovery_journal_invalid')
                if intent_id in intents:
                    _blocked('rollout_recovery_journal_ambiguous')
                intents[intent_id] = event
            elif kind in ('rollout_marker_write_complete', 'workload_scaled',
                'rollout_template_restore_complete', 'rollout_user_template_write_complete',
                'original_hub_finalizer_add_complete', 'original_hub_finalizer_remove_complete'):
                completed.add(event.get('intent_id'))
        incomplete = [event for key, event in intents.items() if key not in completed]
        for intent in incomplete:
            kind = intent['kind']
            if kind == 'rollout_marker_write_intent':
                current = self.kube.get_configmap(namespace=NAMESPACE, name=MARKER)
                data, meta = current['data'], _meta(current)
                expected_op = None if intent['new_phase'] == 'active' else self.operation
                if (meta['uid'] != intent['uid'] or data.get('phase') != intent['new_phase']
                    or data.get('knownRolloutOperation') != expected_op
                    or data.get('dataPVC') != intent['claim']):
                    _blocked('rollout_recovery_unknown')
                self.marker = current
                continue
            if kind in ('original_hub_finalizer_add_intent', 'original_hub_finalizer_remove_intent'):
                current = self.kube.get_pod(namespace=NAMESPACE, name=intent['pod_name'])
                meta = _meta(current, name=intent['pod_name'], namespace=NAMESPACE)
                finals = meta.get('finalizers', [])
                if meta['uid'] != intent['pod_uid']:
                    _blocked('rollout_recovery_unknown')
                present = intent['finalizer'] in finals
                if (kind.endswith('_add_intent') and not present
                    or kind.endswith('_remove_intent') and present):
                    _blocked('rollout_recovery_unknown')
                continue
            if kind == 'rollout_user_template_write_intent':
                current = self.kube.get_deployment(namespace=NAMESPACE, name='voice-user')
                if (_meta(current)['uid'] != intent['uid']
                    or current['spec'].get('replicas', 1) != intent['replicas']
                    or _sha(current['spec']['template']) != intent['new_template_sha256']):
                    _blocked('rollout_recovery_unknown')
                self.snapshots['voice-user'] = current
                continue
            name = intent['name']
            current = self.kube.get_deployment(namespace=NAMESPACE, name=name)
            expected_replicas = intent.get('replicas', 0)
            expected_template = intent.get('template_sha256')
            if kind == 'rollout_template_restore_intent':
                expected_template = intent['template_sha256']
            if (_meta(current)['uid'] != intent['uid']
                or current['spec'].get('replicas', 1) != expected_replicas
                or expected_template is not None and _sha(current['spec']['template']) != expected_template):
                _blocked('rollout_recovery_unknown')
            self.snapshots[name] = current
        return {'verified': True, 'read_only': True, 'reconciled': len(incomplete),
            'unresolved': 0}
