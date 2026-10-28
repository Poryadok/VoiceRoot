"""Existing GIS transitions preserve the first-install identity and receipt."""
import copy
import unittest
from unittest.mock import Mock, patch

import gis_first_install as gis
import gis_first_install_test as first_test
from gis_prerequisite_test import ReadyKube
import gis_existing_update as existing
import runtime_stage
from stage_runtime import NATS_IMAGE


def first_install():
    kube = ReadyKube()
    plan = first_test.plan()
    gis.execute(kube, plan, lambda _event: None)
    ready = gis.verify_ready(kube, plan, timeout=1, sleep=lambda _delay: None)
    receipt = gis.transition_receipt(plan, ready, source_ci=plan['source_ci'],
                                     custody=plan['custody'], target_mode='app-only')
    state = {'operation': plan['operation'], 'status': 'PASS', 'target': {'mode': 'app-only'},
             'gis_first_install': plan, 'gis_first_install_transition_receipt': receipt}
    return kube, state, receipt


class ExistingGISTest(unittest.TestCase):
    def test_normalized_pair_requires_target_hash_and_preserves_images_only_fields(self):
        kube, state, receipt = first_install()
        binding = existing.enroll_first_install(receipt, state, kube)
        deployment = copy.deepcopy(kube.objects[gis.DEPLOYMENT])
        service = copy.deepcopy(kube.objects[gis.SERVICE])
        deployment['spec']['replicas'] = 0
        deployment['spec']['template']['spec']['containers'][0]['image'] = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'b' * 64
        deployment['metadata']['annotations']['kubectl.kubernetes.io/last-applied-configuration'] = 'approved-normalized-document'
        pair = {'deployment_mode': 'selected-paused', 'deployment_uid': binding['deployment_uid'],
                'service_uid': binding['service_uid'], 'deployment': gis.semantic(deployment), 'service': gis.semantic(service)}
        digest = gis.digest(pair)
        bound = existing.bind_normalized_target(binding, deployment, service, True, 'images-only', digest)
        kube.objects[gis.DEPLOYMENT] = copy.deepcopy(deployment)
        self.assertTrue(existing.verify_phase(bound, kube, 'paused-target'))
        with self.assertRaises(gis.Blocked):
            existing.bind_normalized_target(binding, deployment, service, True, 'images-only', '0' * 64)
        changed = copy.deepcopy(deployment)
        changed['spec']['template']['spec']['containers'][0]['env'] = [{'name': 'UNAPPROVED', 'value': 'true'}]
        pair['deployment'] = gis.semantic(changed)
        with self.assertRaises(gis.Blocked):
            existing.bind_normalized_target(binding, changed, service, True, 'images-only', gis.digest(pair))

    def test_full_target_binds_exact_source_normalized_service_and_deployment(self):
        kube, state, receipt = first_install()
        binding = existing.enroll_first_install(receipt, state, kube)
        deployment = copy.deepcopy(kube.objects[gis.DEPLOYMENT])
        service = copy.deepcopy(kube.objects[gis.SERVICE])
        deployment['spec']['replicas'] = 0
        deployment['spec']['template']['spec']['containers'][0]['env'] = [{'name': 'SOURCE_APPROVED', 'value': 'true'}]
        service['spec']['ports'][0]['port'] = 8081
        pair = {'deployment_mode': 'selected-paused', 'deployment_uid': binding['deployment_uid'],
                'service_uid': binding['service_uid'], 'deployment': gis.semantic(deployment), 'service': gis.semantic(service)}
        bound = existing.bind_normalized_target(binding, deployment, service, True, 'app-only', gis.digest(pair))
        kube.objects[gis.DEPLOYMENT] = copy.deepcopy(deployment)
        kube.objects[gis.SERVICE] = copy.deepcopy(service)
        self.assertTrue(existing.verify_phase(bound, kube, 'paused-target'))
        kube.objects[gis.SERVICE]['spec']['ports'][0]['port'] = 8082
        with self.assertRaises(gis.Blocked): existing.verify_phase(bound, kube, 'paused-target')

    def test_deployment_revision_only_accepts_owned_matching_replica_set(self):
        kube, state, receipt = first_install()
        binding = existing.enroll_first_install(receipt, state, kube)
        target = copy.deepcopy(kube.objects[gis.DEPLOYMENT])
        target['spec']['template']['spec']['containers'][0]['image'] = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'b' * 64
        binding = existing.bind_target(binding, target)
        kube.objects[gis.DEPLOYMENT] = copy.deepcopy(target)
        kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 0
        kube.objects[gis.DEPLOYMENT]['metadata']['annotations']['deployment.kubernetes.io/revision'] = '2'
        kube.rs['metadata']['annotations'] = {'deployment.kubernetes.io/revision': '2'}
        kube.rs['spec'] = {'template': copy.deepcopy(target['spec']['template'])}
        self.assertTrue(existing.verify_phase(binding, kube, 'paused-target'))
        for fault in ('wrong-revision', 'foreign-owner', 'wrong-template'):
            altered = copy.deepcopy(kube)
            if fault == 'wrong-revision': altered.rs['metadata']['annotations']['deployment.kubernetes.io/revision'] = '3'
            elif fault == 'foreign-owner': altered.rs['metadata']['ownerReferences'][0]['uid'] = 'foreign'
            else: altered.rs['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
            with self.subTest(fault=fault), self.assertRaises(gis.Blocked):
                existing.verify_phase(binding, altered, 'paused-target')

    def test_gis_ready_proof_is_required_before_marker_release(self):
        stage = object.__new__(runtime_stage.RolloutStage)
        stage.snapshots = {'voice-gameintegration': {}}
        stage.expected = {'source_claim': 'selected'}
        stage.owned_marker = Mock(return_value={'data': {'phase': 'rollout-verified'}})
        stage.scale = Mock()
        stage.wait_ready = Mock()
        stage.kube = Mock()
        stage.save = Mock()
        with self.assertRaises(runtime_stage.Blocked):
            stage.release_marker()
        stage.kube.cas.assert_not_called()
        stage.gis_ready_proof = Mock(side_effect=gis.Blocked('unsafe-ready'))
        with self.assertRaises(gis.Blocked):
            stage.release_marker()
        stage.kube.cas.assert_not_called()
        order = []
        stage.gis_ready_proof = lambda _stage: order.append('gis-proof')
        stage.kube.cas.side_effect = lambda *_args: order.append('release-cas') or {'data': {'phase': 'active'}}
        stage.release_marker()
        self.assertEqual(order, ['gis-proof', 'release-cas'])

    def test_service_account_policy_is_checked_before_fence(self):
        account = {'metadata': {'name': 'default', 'namespace': gis.NS,
                                'uid': '00000000-0000-0000-0000-000000000101', 'resourceVersion': '1'}}
        class Reader:
            def get(self, kind, name):
                self_kind = (kind, name)
                if self_kind != ('serviceaccount', 'default'): raise AssertionError(self_kind)
                return copy.deepcopy(account)
        binding = existing.service_account_preflight(Reader(), {})
        self.assertEqual(binding['uid'], account['metadata']['uid'])
        for fault in ('disabled', 'inherited-pull-secret', 'terminating'):
            modified = copy.deepcopy(account)
            if fault == 'disabled': modified['automountServiceAccountToken'] = False
            elif fault == 'inherited-pull-secret': modified['imagePullSecrets'] = [{'name': 'foreign'}]
            else: modified['metadata']['deletionTimestamp'] = '2026-10-28T00:00:00Z'
            class Other:
                def get(self, _kind, _name): return modified
            with self.subTest(fault=fault), self.assertRaises(gis.Blocked):
                existing.service_account_preflight(Other(), {})

    def test_service_account_admission_preserves_existing_principal_mounts(self):
        template = {'containers': [{'name': 'gameintegration', 'image': 'example.invalid/gis', 'volumeMounts': [
            {'name': 'principal-signing', 'mountPath': '/etc/voice/principal/keys', 'readOnly': True},
            {'name': 'principal-tls', 'mountPath': '/etc/voice/principal/tls', 'readOnly': True}]}],
            'volumes': [{'name': 'principal-signing', 'secret': {'secretName': 'voice-gameintegration-principal-signing'}},
                        {'name': 'principal-tls', 'secret': {'secretName': 'voice-gameintegration-jwks-tls'}}]}
        pod = copy.deepcopy(template)
        pod['serviceAccountName'] = 'default'
        pod['volumes'].append({'name': 'kube-api-access-abc12', 'projected': {'defaultMode': 420, 'sources': [
            {'serviceAccountToken': {'expirationSeconds': 3607, 'path': 'token'}},
            {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}},
            {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}]}})
        pod['containers'][0]['volumeMounts'].append({'name': 'kube-api-access-abc12', 'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True})
        existing.verify_pod_spec(template, pod)
        for fault in ('missing-token', 'inherited-pull-secret', 'changed-principal'):
            changed = copy.deepcopy(pod)
            if fault == 'missing-token': changed['volumes'].pop()
            elif fault == 'inherited-pull-secret': changed['imagePullSecrets'] = [{'name': 'unapproved'}]
            else: changed['containers'][0]['volumeMounts'][0]['mountPath'] = '/foreign'
            with self.subTest(fault=fault), self.assertRaises(gis.Blocked):
                existing.verify_pod_spec(template, changed)

    def test_root_authority_never_accepts_detached_or_missing_anchor(self):
        kube, state, receipt = first_install()
        active = {'gis_first_install_transition': {'operation': state['operation'], 'receipt_sha256': receipt['receipt_sha256']}}
        def read(path, **_kwargs):
            return copy.deepcopy(active if path.name == 'active-release.json' else state)
        with patch.object(existing, 'private_json', side_effect=read):
            binding = existing.load_authority(kube)
            self.assertEqual(binding['anchor_sha256'], receipt['receipt_sha256'])
        for fault in ('missing-anchor', 'wrong-reference', 'nonpass', 'unverified-successor'):
            active = {'gis_first_install_transition': {'operation': state['operation'], 'receipt_sha256': receipt['receipt_sha256']}}
            candidate = copy.deepcopy(state)
            if fault == 'missing-anchor': active = {}
            elif fault == 'wrong-reference': active['gis_first_install_transition']['receipt_sha256'] = '0' * 64
            elif fault == 'nonpass': candidate['status'] = 'WAITING'
            else: active['gis_latest_transition'] = {'operation': 'f' * 12, 'receipt_sha256': 'a' * 64}
            def read_case(path, **_kwargs):
                return copy.deepcopy(active if path.name == 'active-release.json' else candidate)
            with self.subTest(fault=fault), patch.object(existing, 'private_json', side_effect=read_case), self.assertRaises(gis.Blocked):
                existing.load_authority(kube)

    def test_enrollment_requires_completed_root_bound_first_install(self):
        kube, state, receipt = first_install()
        binding = existing.enroll_first_install(receipt, state, kube)
        self.assertEqual(binding['anchor_sha256'], receipt['receipt_sha256'])
        self.assertTrue(existing.verify_phase(binding, kube, 'active'))
        for fault in ('not-pass', 'receipt-tampered', 'namespace', 'service-uid', 'deployment-uid', 'marker', 'spec', 'namespace-terminating', 'service-terminating', 'deployment-terminating'):
            kube, state, receipt = first_install()
            if fault == 'not-pass': state['status'] = 'WAITING'
            elif fault == 'receipt-tampered': receipt['source_sha'] = '0' * 40
            elif fault == 'namespace': kube.objects[('Namespace', gis.NS)] = {'metadata': {'uid': 'replacement'}}
            elif fault == 'service-uid': kube.objects[gis.SERVICE]['metadata']['uid'] = 'replacement'
            elif fault == 'deployment-uid': kube.objects[gis.DEPLOYMENT]['metadata']['uid'] = 'replacement'
            elif fault == 'marker': kube.objects[gis.DEPLOYMENT]['metadata']['annotations'][gis.MARKER] = 'foreign'
            elif fault == 'spec': kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 0
            elif fault == 'namespace-terminating': kube.objects[('Namespace', gis.NS)] = {'metadata': {'uid': '00000000-0000-0000-0000-000000000001', 'deletionTimestamp': '2026-10-28T00:00:00Z'}}
            elif fault == 'service-terminating': kube.objects[gis.SERVICE]['metadata']['deletionTimestamp'] = '2026-10-28T00:00:00Z'
            else: kube.objects[gis.DEPLOYMENT]['metadata']['deletionTimestamp'] = '2026-10-28T00:00:00Z'
            with self.subTest(fault=fault), self.assertRaises(gis.Blocked):
                existing.enroll_first_install(receipt, state, kube)

    def test_paused_old_and_target_are_exact_and_service_never_drifts(self):
        kube, state, receipt = first_install()
        binding = existing.enroll_first_install(receipt, state, kube)
        old = copy.deepcopy(kube.objects[gis.DEPLOYMENT])
        target = copy.deepcopy(old)
        target['spec']['template']['spec']['containers'][0]['image'] = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'b' * 64
        binding = existing.bind_target(binding, target)
        kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 0
        self.assertTrue(existing.verify_phase(binding, kube, 'paused-old'))
        kube.objects[gis.DEPLOYMENT] = copy.deepcopy(target)
        kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 0
        self.assertTrue(existing.verify_phase(binding, kube, 'paused-target'))
        for fault in ('replicas', 'image', 'uid', 'service'):
            altered = copy.deepcopy(kube)
            if fault == 'replicas': altered.objects[gis.DEPLOYMENT]['spec']['replicas'] = 1
            elif fault == 'image': altered.objects[gis.DEPLOYMENT]['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
            elif fault == 'uid': altered.objects[gis.DEPLOYMENT]['metadata']['uid'] = 'replacement'
            else: altered.objects[gis.SERVICE]['metadata']['uid'] = 'replacement'
            with self.subTest(fault=fault), self.assertRaises(gis.Blocked):
                existing.verify_phase(binding, altered, 'paused-target')

        for fault in ('foreign-repository', 'sidecar', 'environment', 'marker'):
            altered = copy.deepcopy(target)
            container = altered['spec']['template']['spec']['containers'][0]
            if fault == 'foreign-repository': container['image'] = 'example.invalid/gis@sha256:' + 'b' * 64
            elif fault == 'sidecar': altered['spec']['template']['spec']['containers'].append({'name': 'extra', 'image': 'foreign'})
            elif fault == 'environment': container['env'] = [{'name': 'CHANGED', 'value': 'true'}]
            else: altered['metadata']['annotations'][gis.MARKER] = 'foreign'
            with self.subTest(fault=fault), self.assertRaises(gis.Blocked):
                existing.bind_target(binding, altered)

    def test_replacement_between_anchor_validation_and_capture_is_rejected(self):
        kube, state, receipt = first_install()
        original = kube.run
        count = [0]
        def changing(args, *other, **kwargs):
            if args[:2] == ['get', 'services,deployments']:
                count[0] += 1
                if count[0] == 3:
                    kube.objects[gis.DEPLOYMENT]['metadata']['uid'] = 'replacement'
            return original(args, *other, **kwargs)
        kube.run = changing
        with self.assertRaises(gis.Blocked):
            existing.enroll_first_install(receipt, state, kube)

    def test_ready_requires_exact_target_owned_pod_and_runtime_image(self):
        kube, state, receipt = first_install()
        binding = existing.enroll_first_install(receipt, state, kube)
        target = copy.deepcopy(kube.objects[gis.DEPLOYMENT])
        target['spec']['template']['spec']['containers'][0]['image'] = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'b' * 64
        binding = existing.bind_target(binding, target)
        kube.objects[gis.DEPLOYMENT] = copy.deepcopy(target)
        kube._ready_objects()
        kube.rs['metadata']['resourceVersion'] = '1'
        kube.pod['metadata']['resourceVersion'] = '1'
        kube.slice['metadata']['resourceVersion'] = '1'
        kube.rs['spec'] = {'template': copy.deepcopy(target['spec']['template'])}
        kube.pod['spec'] = copy.deepcopy(target['spec']['template']['spec'])
        kube.pod['spec']['serviceAccountName'] = 'default'
        kube.pod['spec']['volumes'] = [{'name': 'kube-api-access-abc12', 'projected': {'defaultMode': 420, 'sources': [
            {'serviceAccountToken': {'expirationSeconds': 3607, 'path': 'token'}},
            {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}},
            {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}]}}]
        kube.pod['spec']['containers'][0]['volumeMounts'] = [{'name': 'kube-api-access-abc12',
            'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True}]
        kube.pod['status']['phase'] = 'Running'
        kube.pod['status']['containerStatuses'] = [{'name': 'gameintegration', 'ready': True,
            'imageID': target['spec']['template']['spec']['containers'][0]['image']}]
        kube.objects[('Serviceaccount', 'default')] = {'metadata': {'name': 'default', 'namespace': gis.NS,
            'uid': '00000000-0000-0000-0000-000000000101', 'resourceVersion': '1'}}
        account = existing.service_account_preflight(kube, target['spec']['template']['spec'])
        proof = existing.verify_ready(binding, kube, account)
        self.assertEqual(proof['deployment_uid'], binding['deployment_uid'])
        altered = copy.deepcopy(kube)
        original_get = altered.get
        calls = [0]
        def replace_observation(kind, name):
            if kind == 'pod':
                calls[0] += 1
                if calls[0] == 3:
                    altered.pod['metadata']['resourceVersion'] = '2'
                    altered.pod['status']['containerStatuses'][0]['imageID'] = 'foreign'
            return original_get(kind, name)
        altered.get = replace_observation
        with self.assertRaises(gis.Blocked):
            existing.verify_ready(binding, altered, account)
        for fault in ('wrong-runtime', 'wrong-template', 'foreign-endpoint', 'terminating-pod', 'pod-env', 'sa-projection', 'sa-mount', 'extra-volume', 'host-network', 'host-pid', 'service-account'):
            altered = copy.deepcopy(kube)
            if fault == 'wrong-runtime': altered.pod['status']['containerStatuses'][0]['imageID'] = 'example.invalid/gis@sha256:' + 'c' * 64
            elif fault == 'wrong-template': altered.rs['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
            elif fault == 'foreign-endpoint': altered.slice['endpoints'][0]['targetRef']['uid'] = '00000000-0000-0000-0000-000000000199'
            elif fault == 'terminating-pod': altered.pod['metadata']['deletionTimestamp'] = '2026-10-28T00:00:00Z'
            elif fault == 'pod-env': altered.pod['spec']['containers'][0]['env'] = [{'name': 'MUTATED', 'value': 'true'}]
            elif fault == 'sa-projection': altered.pod['spec']['volumes'][0]['projected']['sources'][1]['configMap']['name'] = 'foreign-ca'
            elif fault == 'sa-mount': altered.pod['spec']['containers'][0]['volumeMounts'][0]['mountPath'] = '/foreign'
            elif fault == 'extra-volume': altered.pod['spec']['volumes'].append({'name': 'extra', 'emptyDir': {}})
            elif fault == 'host-network': altered.pod['spec']['hostNetwork'] = True
            elif fault == 'host-pid': altered.pod['spec']['hostPID'] = True
            else: altered.objects[('Serviceaccount', 'default')]['metadata']['resourceVersion'] = '2'
            with self.subTest(fault=fault), self.assertRaises(gis.Blocked):
                existing.verify_ready(binding, altered, account)

    def test_real_capture_requires_qualified_fixed_gis_binding(self):
        kube, state, receipt = first_install()
        binding = existing.enroll_first_install(receipt, state, kube)
        client = CaptureKube(kube)
        with self.assertRaises(runtime_stage.Blocked):
            runtime_stage.RolloutStage.capture(client, '123456abcdef', lambda _event: None,
                                               ['voice-gameintegration'])
        stage = runtime_stage.RolloutStage.capture(client, '123456abcdef', lambda _event: None,
                                                   ['voice-gameintegration'], gis_binding=binding)
        self.assertEqual(stage.snapshots['voice-gameintegration']['metadata']['uid'], binding['deployment_uid'])
        self.assertEqual(stage.old_images['voice-gameintegration/gameintegration'],
                         binding['old']['spec']['template']['spec']['containers'][0]['image'])
        second = runtime_stage.RolloutStage.capture(client, '123456abcdef', lambda _event: None,
                                                    ['voice-gameintegration'], gis_binding=binding)
        self.assertEqual(stage.old_images['voice-gameintegration/gameintegration'], second.old_images['voice-gameintegration/gameintegration'])
        for fault in ('uid', 'template', 'service', 'runtime-image'):
            changed_client = copy.deepcopy(client)
            if fault == 'uid': changed_client.deployments['voice-gameintegration']['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
            elif fault == 'template': changed_client.deployments['voice-gameintegration']['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
            elif fault == 'service': changed_client.gis_kube.objects[gis.SERVICE]['spec']['selector']['app'] = 'foreign'
            else: changed_client.runtime_gis_image = 'example.invalid/gis@sha256:' + 'd' * 64
            with self.subTest(fault=fault), self.assertRaises((gis.Blocked,runtime_stage.Blocked)):
                runtime_stage.RolloutStage.capture(changed_client, '123456abcdef', lambda _event: None,
                                                   ['voice-gameintegration'], gis_binding=binding)
        client.service_account['automountServiceAccountToken'] = False
        with self.assertRaises(gis.Blocked):
            runtime_stage.RolloutStage.capture(client, '123456abcdef', lambda _event: None,
                                               ['voice-gameintegration'], gis_binding=binding)
        client.service_account.pop('automountServiceAccountToken')
        changed = copy.deepcopy(binding)
        changed['deployment_uid'] = 'replacement'
        with self.assertRaises((runtime_stage.Blocked, gis.Blocked)):
            runtime_stage.RolloutStage.capture(client, '123456abcdef', lambda _event: None,
                                               ['voice-gameintegration'], gis_binding=changed)


class CaptureKube:
    """Actual capture/preflight/pin traversal over an in-memory Kubernetes API."""
    def __init__(self, gis_kube):
        self.gis_kube = gis_kube
        self.claim_uid = '5fae59b0-aea2-405a-a0f7-287e94a9b6a9'
        self.pv_uid = '4a11a528-81f9-4404-9757-f4f32c411310'
        self.claim = 'voice-nats-jsdata-r20260930a4'
        self.runtime_gis_image = None
        self.service_account = {'metadata': {'name': 'default', 'namespace': gis.NS,
            'uid': '00000000-0000-0000-0000-000000000101', 'resourceVersion': '1'}}
        self.deployments = {}
        for name in (runtime_stage.HUB, 'voice-gateway', *('voice-' + x for x in runtime_stage.LEAVES)):
            container = name.removeprefix('voice-')
            image = NATS_IMAGE if name == runtime_stage.HUB else 'example.invalid/' + container + '@sha256:' + 'c' * 64
            spec = {'containers': [{'name': container, 'image': image}]}
            if name == runtime_stage.HUB:
                spec['volumes'] = [{'name': 'jsdata', 'persistentVolumeClaim': {'claimName': self.claim}}]
            self.deployments[name] = {'metadata': {'name': name, 'uid': name, 'resourceVersion': '1'},
                                      'spec': {'replicas': 1, 'template': {'spec': spec}}}
        self.deployments['voice-gameintegration'] = copy.deepcopy(gis_kube.objects[gis.DEPLOYMENT])

    def get(self, kind, name):
        if kind == 'serviceaccount' and name == 'default': return copy.deepcopy(self.service_account)
        if kind == 'namespace': return self.gis_kube.get(kind, name)
        if kind == 'configmap': return {'metadata': {'uid': 'marker', 'resourceVersion': '1'}, 'data': {'phase': 'active', 'generation': 'r20260930a4', 'dataPVC': self.claim}}
        if kind == 'pvc': return {'metadata': {'uid': self.claim_uid, 'name': self.claim}, 'spec': {'volumeName': 'pv'}}
        if kind == 'pv': return {'metadata': {'uid': self.pv_uid}, 'spec': {'claimRef': {'uid': self.claim_uid}, 'local': {'path': '/var/lib/rancher/k3s/storage/pvc-' + self.claim_uid + '_voice-staging_' + self.claim}, 'nodeAffinity': {'required': {'nodeSelectorTerms': [{'matchExpressions': [{'key': 'kubernetes.io/hostname', 'operator': 'In', 'values': ['pmdebook']}]}]}}}}
        if kind == 'service' and name == 'voice-nats': return {'metadata': {'uid': 'nats-service'}}
        if kind == 'service': return self.gis_kube.get(kind, name)
        if kind == 'deployment': return copy.deepcopy(self.deployments[name])
        if kind == 'replicaset': return copy.deepcopy(next(x for x in self.image_rows() if x['kind'] == 'ReplicaSet' and x['metadata']['name'] == name))
        raise AssertionError((kind, name))

    def image_rows(self):
        rows = []
        for name, deployment in self.deployments.items():
            rs = name + '-rs'
            rows.append({'kind': 'ReplicaSet', 'metadata': {'name': rs, 'uid': rs, 'ownerReferences': [{'kind': 'Deployment', 'uid': deployment['metadata']['uid']}]}, 'spec': {'template': copy.deepcopy(deployment['spec']['template'])}})
            rows.append({'kind': 'Pod', 'metadata': {'ownerReferences': [{'kind': 'ReplicaSet', 'uid': rs}]}, 'status': {'phase': 'Running', 'containerStatuses': [{'name': c['name'], 'ready': True, 'imageID': self.runtime_gis_image if name == 'voice-gameintegration' and self.runtime_gis_image else c['image']} for c in deployment['spec']['template']['spec']['containers']]}})
        return rows

    def run(self, args):
        if args[:2] == ['get', 'services,deployments']:
            return {'apiVersion': 'v1', 'kind': 'List', 'metadata': {}, 'items': [
                copy.deepcopy(self.gis_kube.objects[gis.SERVICE]),
                copy.deepcopy(self.deployments['voice-gameintegration'])]}
        return {'items': self.image_rows() if args[:2] == ['get', 'pods,replicasets'] else []}


if __name__ == '__main__':
    unittest.main()
