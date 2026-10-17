import copy
import hashlib
import json
import unittest

from controller import Blocked
from kube_json_api import KubeAPIError
from native_runtime_stage import HUB, LEAVES, NativeRolloutStage


NS = 'voice-staging'
HUB = 'voice-nats-pvc-candidate'
CLAIM = 'voice-nats-jsdata-d20261017abc12345'
ACCOUNT = 'A' + 'B' * 55
PINNED_IMAGE = 'nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb'


def metadata(name, uid, rv='1', namespace=NS):
    return {'name': name, 'namespace': namespace, 'uid': uid, 'resourceVersion': rv}


class FakeAPI:
    def __init__(self):
        self.writes = []
        self.fail_marker = None
        self.fail_deployment = None
        self.mutate_pod_on_readback = False
        self.namespace = {'metadata': metadata(NS, '00000000-0000-0000-0000-000000000001', namespace=None)}
        self.marker = {'metadata': metadata('voice-nats-generation', '00000000-0000-0000-0000-000000000002'),
            'data': {'phase': 'active', 'generation': 'r20261017a', 'dataPVC': CLAIM}}
        self.hub = {'metadata': metadata(HUB, '00000000-0000-0000-0000-000000000003'),
            'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': HUB}},
                'template': {'metadata': {'labels': {'app': HUB}}, 'spec': {
                    'volumes': [{'name': 'jsdata', 'persistentVolumeClaim': {'claimName': CLAIM}}],
                    'containers': [{'name': 'nats', 'image': PINNED_IMAGE}]}}},
            'status': {'readyReplicas': 1, 'updatedReplicas': 1, 'availableReplicas': 1,
                'observedGeneration': 1}}
        self.claim = {'metadata': metadata(CLAIM, '00000000-0000-0000-0000-000000000004'),
            'spec': {'volumeName': 'pvc-source', 'accessModes': ['ReadWriteOnce'],
                'storageClassName': 'local-path', 'volumeMode': 'Filesystem'},
            'status': {'phase': 'Bound'}}
        self.pv = {'metadata': metadata('pvc-source', '00000000-0000-0000-0000-000000000005', namespace=None),
            'spec': {'storageClassName': 'local-path', 'volumeMode': 'Filesystem',
                'claimRef': {'namespace': NS, 'name': CLAIM, 'uid': self.claim['metadata']['uid']},
                'local': {'path': '/must-not-be-read'},
                'nodeAffinity': {'required': {'nodeSelectorTerms': [{'matchExpressions': [
                    {'key': 'kubernetes.io/hostname', 'operator': 'In', 'values': ['host-label']}]}]}}},
            'status': {'phase': 'Bound'}}
        self.node = {'metadata': {'name': 'node-object-name', 'namespace': None,
                'uid': '00000000-0000-0000-0000-000000000006', 'resourceVersion': '1',
                'labels': {'kubernetes.io/hostname': 'host-label'}},
            'spec': {}, 'status': {'conditions': [{'type': 'Ready', 'status': 'True'}]}}
        self.pod = {'metadata': {**metadata(HUB + '-abc', '00000000-0000-0000-0000-000000000007'),
                'labels': {'app': HUB}},
            'spec': {'nodeName': 'node-object-name', 'volumes': [
                {'name': 'jsdata', 'persistentVolumeClaim': {'claimName': CLAIM}}],
                'containers': [{'name': 'nats', 'image': PINNED_IMAGE}]},
            'status': {'phase': 'Running', 'containerStatuses': [{'name': 'nats', 'ready': True,
                'image': PINNED_IMAGE, 'imageID': 'docker-pullable://docker.io/library/nats@sha256:' + PINNED_IMAGE.split('@sha256:', 1)[1],
                'containerID': 'containerd://' + 'a' * 64, 'restartCount': 0,
                'state': {'running': {'startedAt': '2026-10-17T00:00:00Z'}}}]}}
        self.rs = {'metadata': {'name': HUB + '-abc', 'namespace': NS,
                'uid': '00000000-0000-0000-0000-000000000008', 'resourceVersion': '1',
                'ownerReferences': [{'kind': 'Deployment', 'name': HUB,
                    'uid': self.hub['metadata']['uid'], 'controller': True}]}}
        self.pod['metadata']['ownerReferences'] = [{'kind': 'ReplicaSet',
            'name': self.rs['metadata']['name'], 'uid': self.rs['metadata']['uid'],
            'controller': True}]
        self.service = {'metadata': metadata('voice-nats', '00000000-0000-0000-0000-000000000009'),
            'spec': {'type': 'ClusterIP', 'selector': {'app': HUB}}}
        self.deployments = [self.hub]
        self.pods = [self.pod]
        self.replicasets = [self.rs]
        self.pod_rs = {}
        for index, name in enumerate(('voice-gateway', *('voice-' + item for item in LEAVES)), start=10):
            deployment = {'metadata': metadata(name,
                    f'00000000-0000-0000-0000-{index:012d}'),
                'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': name}},
                    'template': {'metadata': {'labels': {'app': name}}, 'spec': {
                        'containers': [{'name': 'user' if name == 'voice-user' else name,
                            'image': 'example@sha256:' + 'b' * 64,
                            **({'env': []} if name == 'voice-user' else {})}],
                        'volumes': []}}}}
            if name == 'voice-user':
                deployment['spec']['template']['metadata']['annotations'] = {}
            self.deployments.append(deployment)
            pod_name = name + '-abc'
            rs = {'metadata': {'name': pod_name, 'namespace': NS,
                    'uid': f'00000000-0000-0000-0000-{index + 100:012d}', 'resourceVersion': '1',
                    'ownerReferences': [{'kind': 'Deployment', 'name': name,
                        'uid': deployment['metadata']['uid'], 'controller': True}]},
                'spec': {'template': copy.deepcopy(deployment['spec']['template'])}}
            pod = {'metadata': {**metadata(pod_name, f'00000000-0000-0000-0000-{index + 200:012d}'),
                    'labels': {'app': name},
                    'ownerReferences': [{'kind': 'ReplicaSet', 'name': pod_name,
                        'uid': rs['metadata']['uid'], 'controller': True}]},
                'spec': {'nodeName': 'node-object-name', 'volumes': [],
                    'containers': copy.deepcopy(deployment['spec']['template']['spec']['containers'])},
                'status': {'phase': 'Running', 'containerStatuses': [{
                    'name': 'user' if name == 'voice-user' else name,
                    'ready': True, 'imageID': 'docker.io/library/app@sha256:' + 'b' * 64,
                    'containerID': 'containerd://' + 'b' * 64,
                    'state': {'running': {'startedAt': '2026-10-17T00:00:00Z'}}}]}}
            self.pods.append(pod)
            self.replicasets.append(rs)

    def get_namespace(self, name):
        assert name == NS
        return copy.deepcopy(self.namespace)

    def get_configmap(self, *, namespace, name):
        assert namespace == NS
        if name == 'voice-nats-generation':
            return copy.deepcopy(self.marker)
        assert name == 'voice-app-config'
        return {'metadata': metadata(name, '00000000-0000-0000-0000-000000000600'),
            'data': {'SPACE_GRPC_ADDR': 'voice-space:50051'}}

    def get_deployment(self, *, namespace, name):
        assert namespace == NS
        return copy.deepcopy(next(row for row in self.deployments if row['metadata']['name'] == name))

    def get_pvc(self, *, namespace, name):
        assert (namespace, name) == (NS, CLAIM)
        return copy.deepcopy(self.claim)

    def get_pv(self, name):
        assert name == 'pvc-source'
        return copy.deepcopy(self.pv)

    def list_nodes_by_hostname(self, hostname):
        assert hostname == 'host-label'
        return [copy.deepcopy(self.node)]

    def get_node(self, *, namespace, name):
        assert namespace is None or namespace == NS
        assert name == 'node-object-name'
        return copy.deepcopy(self.node)

    def list_hub_pods(self, *, namespace, labels):
        assert namespace == NS and labels == {'app': HUB}
        return copy.deepcopy([row for row in self.pods
            if row.get('metadata', {}).get('labels', {}).get('app') == HUB])

    def get_pod(self, *, namespace, name):
        assert namespace == NS
        row = next(row for row in self.pods if row['metadata']['name'] == name)
        if self.mutate_pod_on_readback:
            row['status']['containerStatuses'][0]['restartCount'] += 1
            self.mutate_pod_on_readback = False
        return copy.deepcopy(row)

    def get_replicaset(self, *, namespace, name):
        assert namespace == NS
        return copy.deepcopy(next(row for row in self.replicasets if row['metadata']['name'] == name))

    def get_service(self, *, namespace, name):
        assert (namespace, name) == (NS, 'voice-nats')
        return copy.deepcopy(self.service)

    def exec_hub_jsz(self, *, namespace, pod_name, pod_uid, container, timeout_seconds):
        assert namespace == NS and pod_name == self.pod['metadata']['name']
        assert pod_uid == self.pod['metadata']['uid']
        assert container == 'nats' and timeout_seconds == 30
        return ('{"account_details":[{"id":"' + ACCOUNT + '","stream_detail":[]}],'
            '"total":1,"streams":0,"consumers":0,"messages":0,"bytes":0}').encode()

    def list_control_objects(self, namespace):
        assert namespace == NS
        return {'deployments': copy.deepcopy(self.deployments),
            'horizontalpodautoscalers': [], 'jobs': []}

    def list_fence_objects(self, namespace):
        assert namespace == NS
        return {'pods': copy.deepcopy(self.pods), 'replicasets': copy.deepcopy(self.replicasets)}

    def cas_rollout_marker(self, name, operation, expected_uid, expected_rv, expected_phase, new_phase,
            *, expected_claim, release=False):
        self.writes.append(('marker', name, new_phase))
        if self.marker['metadata']['uid'] != expected_uid or self.marker['metadata']['resourceVersion'] != expected_rv:
            raise KubeAPIError('kube_api_conflict')
        if self.marker['data'].get('phase') != expected_phase:
            raise KubeAPIError('kube_api_conflict')
        if (self.marker['data'].get('dataPVC') != expected_claim
            or (expected_phase == 'active' and 'knownRolloutOperation' in self.marker['data'])
            or (expected_phase != 'active'
                and self.marker['data'].get('knownRolloutOperation') != operation)):
            raise KubeAPIError('kube_api_conflict')
        if self.fail_marker == 'before':
            self.fail_marker = None
            raise KubeAPIError('kube_api_transport_failed')
        row = self.marker
        row['metadata']['resourceVersion'] = str(int(expected_rv) + 1)
        row['data']['phase'] = new_phase
        if new_phase == 'active':
            row['data'].pop('knownRolloutOperation', None)
        else:
            row['data']['knownRolloutOperation'] = operation
        if self.fail_marker == 'after':
            self.fail_marker = None
            raise KubeAPIError('kube_api_transport_failed')
        return copy.deepcopy(row)

    def cas_rollout_deployment(self, namespace, name, expected_uid, expected_rv,
            expected_replicas, expected_template_sha256, *, new_replicas=None,
            new_template=None, expected_jsdata_claim=None, new_jsdata_claim=None):
        assert namespace == NS
        self.writes.append(('deployment', name, new_replicas))
        row = next(row for row in self.deployments if row['metadata']['name'] == name)
        digest = hashlib.sha256(json.dumps(row['spec']['template'], sort_keys=True,
            separators=(',', ':'), ensure_ascii=False).encode()).hexdigest()
        if (row['metadata']['uid'] != expected_uid or row['metadata']['resourceVersion'] != expected_rv
            or row['spec'].get('replicas', 1) != expected_replicas
            or digest != expected_template_sha256 or new_jsdata_claim is not None
            or name == HUB and row['spec']['template']['spec']['volumes'][0]['persistentVolumeClaim']['claimName']
                != expected_jsdata_claim):
            raise KubeAPIError('kube_api_conflict')
        if self.fail_deployment == 'before':
            self.fail_deployment = None
            raise KubeAPIError('kube_api_transport_failed')
        if self.fail_deployment == 'before-drift':
            self.fail_deployment = None
            row['spec']['template']['metadata']['labels']['external-drift'] = 'yes'
            row['metadata']['resourceVersion'] = str(int(expected_rv) + 1)
            raise KubeAPIError('kube_api_transport_failed')
        if new_replicas is not None:
            row['spec']['replicas'] = new_replicas
        if new_template is not None:
            row['spec']['template'] = copy.deepcopy(new_template)
        row['metadata']['resourceVersion'] = str(int(expected_rv) + 1)
        if self.fail_deployment == 'after':
            self.fail_deployment = None
            raise KubeAPIError('kube_api_transport_failed')
        return copy.deepcopy(row)

    def cas_user_bootstrap_template(self, namespace, name, expected_uid, expected_rv,
            expected_replicas, expected_template_sha256, *, operation,
            original_template=None, action):
        assert (namespace, name) == (NS, 'voice-user')
        row = next(row for row in self.deployments if row['metadata']['name'] == name)
        self.writes.append(('user-template', action))
        digest = hashlib.sha256(json.dumps(row['spec']['template'], sort_keys=True,
            separators=(',', ':'), ensure_ascii=False).encode()).hexdigest()
        if (row['metadata']['uid'] != expected_uid or row['metadata']['resourceVersion'] != expected_rv
            or row['spec'].get('replicas', 1) != expected_replicas
            or digest != expected_template_sha256):
            raise KubeAPIError('kube_api_conflict')
        template = row['spec']['template']
        template = copy.deepcopy(template)
        if action == 'install':
            env = next(container for container in template['spec']['containers']
                if container['name'] == 'user')['env']
            env.append({'name': 'SPACE_GRPC_ADDR', 'value': ''})
            template['metadata']['annotations']['voice.io/nats-user-space-bootstrap'] = operation
        elif action == 'restore':
            template = copy.deepcopy(original_template)
        else:
            raise AssertionError(action)
        row['spec']['template'] = template
        row['metadata']['resourceVersion'] = str(int(expected_rv) + 1)
        return copy.deepcopy(row)

    def get_deployment_ready(self, *, name, expected_uid):
        row = next(row for row in self.deployments if row['metadata']['name'] == name)
        assert row['metadata']['uid'] == expected_uid
        return True


class NativeRuntimeStageTest(unittest.TestCase):
    def setUp(self):
        self.api = FakeAPI()
        self.events = []
        self.stage = NativeRolloutStage.capture(self.api, 'a' * 12, self.events.append,
            account_id=ACCOUNT)

    def test_capture_records_selected_source_and_never_pv_host_path(self):
        self.assertEqual(self.stage.expected['source_claim'], CLAIM)
        self.assertEqual(self.stage.expected['source_node_name'], 'node-object-name')
        self.assertEqual(self.stage.source_identity['source_node_hostname'], 'host-label')
        self.assertEqual(HUB, 'voice-nats-pvc-candidate')
        self.assertEqual(LEAVES, ('analytics', 'auth', 'bot', 'chat', 'file', 'matchmaking',
            'messaging', 'moderation', 'notification', 'realtime', 'role', 'search', 'social',
            'space', 'story', 'subscription', 'user', 'voice'))
        self.assertEqual(set(self.stage.snapshots), {HUB, 'voice-gateway',
            *('voice-' + name for name in LEAVES)})
        self.assertEqual(len(self.stage.old_images), len(LEAVES) + 1)
        self.assertNotIn(HUB + '/nats', self.stage.old_images)
        self.assertIn('voice-gateway/voice-gateway', self.stage.old_images)
        self.assertNotIn('path', repr(self.stage.expected).lower())
        self.assertNotIn('path', repr(self.stage.source_identity).lower())
        self.assertNotIn('path', repr(self.stage.final_pv).lower())

    def test_claim_allocation_and_selection_keep_existing_stage_policy_forbidden(self):
        for method, args in ((self.stage.new_claim, ('20261017',)), (self.stage.select_claim, ())):
            with self.subTest(method=method.__name__), self.assertRaises(Blocked):
                method(*args)
        self.assertEqual(self.api.writes, [])

    def test_restart_preserves_captured_target_template_and_bootstrap_order(self):
        operation = 'a' * 12
        self.api.marker['metadata']['resourceVersion'] = '2'
        self.api.marker['data'].update({'phase': 'rollout-verified',
            'knownRolloutOperation': operation})
        self.stage.marker = copy.deepcopy(self.api.marker)
        for row in self.api.deployments:
            row['spec']['replicas'] = 0
        self.api.pods.clear()
        self.stage.snapshots = {row['metadata']['name']: copy.deepcopy(row)
            for row in self.api.deployments}
        user = next(row for row in self.api.deployments if row['metadata']['name'] == 'voice-user')
        user['spec']['template']['spec']['containers'][0]['image'] = 'target@sha256:' + 'c' * 64
        self.stage.snapshots['voice-user'] = copy.deepcopy(user)
        self.stage.verify_selected_storage_identity = lambda: {'verified': True}
        self.stage.no_pods = lambda *_args, **_kwargs: None
        self.stage.seal_before_business_resume = lambda: {'sealed': True}
        trace = []
        scale = self.stage._scale
        wait = self.stage._wait_ready
        user_cas = self.stage._user_template_cas

        def recording_scale(name, replicas):
            trace.append(('scale', name, replicas))
            return scale(name, replicas)

        def recording_wait(name, timeout=300):
            trace.append(('ready', name))
            return wait(name, timeout)

        def recording_user_cas(action, old, new):
            trace.append(('user-template', action))
            return user_cas(action, old, new)

        self.stage._scale = recording_scale
        self.stage._wait_ready = recording_wait
        self.stage._user_template_cas = recording_user_cas
        self.stage.restart()
        self.assertEqual(user['spec']['template'], self.stage.snapshots['voice-user']['spec']['template'])
        self.assertEqual(self.stage.snapshots['voice-user']['spec']['template']['spec']['containers'][0]['image'],
            'target@sha256:' + 'c' * 64)
        self.assertEqual(trace[0], ('scale', HUB, 1))
        self.assertEqual(trace[1], ('ready', HUB))
        self.assertEqual(trace[2], ('user-template', 'install'))
        self.assertEqual(trace[3:3 + len(LEAVES)], [
            ('scale', 'voice-' + item, 1) for item in LEAVES])
        user_ready = trace.index(('ready', 'voice-user'))
        space_ready = trace.index(('ready', 'voice-space'))
        restore = trace.index(('user-template', 'restore'))
        self.assertLess(user_ready, restore)
        self.assertLess(space_ready, restore)
        self.assertEqual(trace[-2:], [('scale', 'voice-gateway', 1), ('ready', 'voice-gateway')])
        self.assertEqual(self.api.marker['data']['phase'], 'active')
        self.assertNotIn('knownRolloutOperation', self.api.marker['data'])
        self.assertEqual(self.api.marker['data']['dataPVC'], CLAIM)

    def test_restart_missing_scales_only_pending_and_observes_completed(self):
        operation = 'a' * 12
        self.api.marker['metadata']['resourceVersion'] = '2'
        self.api.marker['data'].update({'phase': 'rollout-verified',
            'knownRolloutOperation': operation})
        self.stage.marker = copy.deepcopy(self.api.marker)
        for row in self.api.deployments:
            row['spec']['replicas'] = 0
        self.api.pods.clear()
        self.stage.snapshots = {row['metadata']['name']: copy.deepcopy(row)
            for row in self.api.deployments}
        pending = {HUB: {'ready': False}, 'voice-gateway': {'ready': False}}
        completed = {name: {'ready': True} for name in self.stage.snapshots if name not in pending}
        trace = []
        self.stage.seal_before_business_resume = lambda: trace.append(('seal',))
        self.stage._scale = lambda name, replicas: trace.append(('scale', name, replicas))
        self.stage._wait_ready = lambda name, timeout=300: trace.append(('ready', name))
        self.stage.release_marker = lambda: trace.append(('release',))
        ledger = type('Ledger', (), {'record': {'pending': pending, 'completed': completed}})()

        result = self.stage.restart_missing(ledger)

        self.assertTrue(result['finish_only'])
        self.assertEqual(result['active_claim'], CLAIM)
        self.assertEqual(trace[0], ('seal',))
        self.assertEqual([entry for entry in trace if entry[0] == 'scale'],
            [('scale', HUB, 1), ('scale', 'voice-gateway', 1)])
        self.assertEqual(len([entry for entry in trace if entry == ('ready', HUB)]), 1)
        self.assertEqual(len([entry for entry in trace if entry == ('ready', 'voice-gateway')]), 1)
        self.assertEqual(trace[-1], ('release',))
        self.assertEqual(self.api.writes, [])

    def test_restart_missing_rejects_overlapping_ledger_partitions_before_seal(self):
        operation = 'a' * 12
        self.api.marker['metadata']['resourceVersion'] = '2'
        self.api.marker['data'].update({'phase': 'rollout-verified',
            'knownRolloutOperation': operation})
        self.stage.marker = copy.deepcopy(self.api.marker)
        sealed = []
        self.stage.seal_before_business_resume = lambda: sealed.append(True)
        names = set(self.stage.snapshots)
        ledger = type('Ledger', (), {'record': {
            'pending': {name: {} for name in names},
            'completed': {next(iter(names)): {}},
        }})()
        with self.assertRaises(Blocked):
            self.stage.restart_missing(ledger)
        self.assertEqual(sealed, [])
        self.assertEqual(self.api.writes, [])

    def test_capture_fails_when_marker_claim_does_not_match_hub_claim(self):
        self.api.marker['data']['dataPVC'] = 'voice-nats-jsdata-d20261016ffffeeee'
        with self.assertRaises(Blocked):
            NativeRolloutStage.capture(self.api, 'a' * 12, self.events.append,
                account_id=ACCOUNT)
        self.assertEqual(self.api.writes, [])

    def test_live_pod_container_change_after_jsz_readback_blocks_capture(self):
        self.api.mutate_pod_on_readback = True
        with self.assertRaises(Blocked):
            NativeRolloutStage.capture(self.api, 'a' * 12, self.events.append,
                account_id=ACCOUNT)
        self.assertEqual(self.api.writes, [])

    def test_fresh_readback_fails_closed_on_pv_claim_uid_drift(self):
        self.api.pv['spec']['claimRef']['uid'] = '00000000-0000-0000-0000-000000000099'
        with self.assertRaises(Blocked):
            self.stage.verify_selected_storage_identity()
        self.assertEqual(self.api.writes, [])

    def test_marker_uid_and_resource_version_drift_block_before_cas(self):
        for field, value in (('uid', '00000000-0000-0000-0000-000000000099'),
                ('resourceVersion', '99')):
            with self.subTest(field=field):
                self.api.marker['metadata'][field] = value
                with self.assertRaises(Blocked):
                    self.stage.maintenance()
                self.assertEqual(self.api.writes, [])
                self.api.marker['metadata'][field] = ('00000000-0000-0000-0000-000000000002'
                    if field == 'uid' else '1')

    def test_node_affinity_hostname_must_resolve_to_one_ready_node(self):
        original = self.api.list_nodes_by_hostname
        self.api.list_nodes_by_hostname = lambda _hostname: [self.api.node, copy.deepcopy(self.api.node)]
        with self.assertRaises(Blocked):
            NativeRolloutStage.capture(self.api, 'a' * 12, self.events.append, account_id=ACCOUNT)
        self.assertEqual(self.api.writes, [])
        self.api.list_nodes_by_hostname = original

    def test_wrong_replicaset_owner_blocks_live_census(self):
        self.api.rs['metadata']['ownerReferences'][0]['uid'] = '00000000-0000-0000-0000-000000000099'
        with self.assertRaises(Blocked):
            NativeRolloutStage.capture(self.api, 'a' * 12, self.events.append, account_id=ACCOUNT)
        self.assertEqual(self.api.writes, [])

    def test_live_hub_pod_after_rollout_fence_is_rejected(self):
        self.api.marker['data'].update({'phase': 'rollout-capturing',
            'knownRolloutOperation': 'a' * 12})
        self.stage.marker = copy.deepcopy(self.api.marker)
        with self.assertRaises(Blocked):
            self.stage.verify_selected_storage_identity()
        self.assertEqual(self.api.writes, [])

    def test_unknown_marker_cas_reconciles_exact_applied_state_without_retry(self):
        self.api.fail_marker = 'after'
        self.stage.maintenance()
        self.assertEqual(self.api.marker['data']['phase'], 'rollout-capturing')
        self.assertEqual(len(self.api.writes), 1)
        self.assertEqual(self.stage.marker['metadata']['resourceVersion'], '2')

    def test_uncertain_marker_cas_that_did_not_apply_stays_blocked_and_is_not_replayed(self):
        self.api.fail_marker = 'before'
        with self.assertRaises(Blocked):
            self.stage.maintenance()
        self.assertEqual(self.api.marker['data']['phase'], 'active')
        self.assertEqual(len(self.api.writes), 1)

    def test_unknown_deployment_cas_exact_readback_is_accepted_without_retry(self):
        self.stage.maintenance()
        before = len(self.api.writes)
        self.api.fail_deployment = 'after'
        self.stage._scale('voice-user', 0)
        self.assertEqual(self.api.writes[before:], [('deployment', 'voice-user', 0)])
        self.assertEqual(self.stage.snapshots['voice-user']['spec']['replicas'], 0)

    def test_unknown_deployment_cas_with_template_drift_blocks_without_retry(self):
        self.stage.maintenance()
        before = len(self.api.writes)
        self.api.fail_deployment = 'before-drift'
        with self.assertRaises(Blocked):
            self.stage._scale('voice-user', 0)
        self.assertEqual(self.api.writes[before:], [('deployment', 'voice-user', 0)])

    def test_recovery_observes_unfinished_marker_intent_without_retrying_it(self):
        self.api.fail_marker = 'before'
        with self.assertRaises(Blocked):
            self.stage.maintenance()
        history = copy.deepcopy(self.events)
        resumed = NativeRolloutStage(self.api, 'a' * 12, self.stage.expected_identity,
            self.events.append, history=history)
        with self.assertRaises(Blocked):
            resumed.recover()
        self.assertEqual(len(self.api.writes), 1)
        self.assertEqual(self.api.marker['data']['phase'], 'active')

    def test_unknown_marker_intent_with_identity_drift_is_not_replayed(self):
        self.api.fail_marker = 'before'
        with self.assertRaises(Blocked):
            self.stage.maintenance()
        self.api.marker['metadata']['uid'] = '00000000-0000-0000-0000-000000000099'
        resumed = NativeRolloutStage(self.api, 'a' * 12, self.stage.expected_identity,
            self.events.append, history=copy.deepcopy(self.events))
        with self.assertRaises(Blocked):
            resumed.recover()
        self.assertEqual(len(self.api.writes), 1)

    def test_recovery_proves_applied_marker_with_opaque_rv_without_replaying(self):
        event = {'kind': 'rollout_marker_write_intent',
            'uid': self.api.marker['metadata']['uid'], 'resource_version': '1',
            'old_phase': 'active', 'new_phase': 'rollout-capturing',
            'claim': CLAIM, 'release': False}
        event['intent_id'] = hashlib.sha256(json.dumps(event, sort_keys=True,
            separators=(',', ':'), ensure_ascii=False, allow_nan=False).encode()).hexdigest()
        self.api.marker['metadata']['resourceVersion'] = 'opaque-rv:cas-applied'
        self.api.marker['data'].update({'phase': 'rollout-capturing',
            'knownRolloutOperation': 'a' * 12})
        resumed = NativeRolloutStage(self.api, 'a' * 12, self.stage.expected_identity,
            self.events.append, history=[event])
        receipt = resumed.recover()
        self.assertEqual(receipt, {'verified': True, 'read_only': True,
            'reconciled': 1, 'unresolved': 0})
        self.assertEqual(self.api.writes, [])
        self.assertEqual(resumed.marker['metadata']['resourceVersion'], 'opaque-rv:cas-applied')


if __name__ == '__main__':
    unittest.main()
