"""Cross-stage GIS contract using actual capture, compiler and normalizer."""
import copy
import contextlib
import datetime as dt
import hashlib
import inspect
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

import bridge_root
import compiler
import expired_recovery
import guard
import gis_existing_update as existing
import gis_first_install as gis
import gis_existing_update_test as fixtures
import normalize
import paused_recovery
import preserve
import resource_claim
import resource_claim_test
import runtime_stage
import source_plan
import source_plan_test
import source_authority
import transaction


class AdmissionKube(fixtures.CaptureKube):
    def get(self, kind, name):
        if kind == 'serviceaccount' and name == 'gis-target':
            return copy.deepcopy(self.target_account)
        return super().get(kind, name)

    def run(self, args, body=None):
        if args[:2] == ['apply', '--dry-run=server']:
            result = copy.deepcopy(body)
            current = self.get(result['kind'].lower(), result['metadata']['name'])
            result['metadata']['uid'] = current['metadata']['uid']
            for key in ('labels', 'annotations'):
                merged = copy.deepcopy(current['metadata'].get(key, {}))
                merged.update(result['metadata'].get(key, {}))
                result['metadata'][key] = merged
            return result
        return super().run(args)


class GISLifecycleContract(unittest.TestCase):
    def setUp(self):
        kube, state, receipt = fixtures.first_install()
        self.binding = existing.enroll_first_install(receipt, state, kube)
        self.kube = AdmissionKube(kube)
        self.kube.target_account = {'metadata': {'name': 'gis-target', 'namespace': gis.NS,
            'uid': '00000000-0000-0000-0000-000000000103', 'resourceVersion': '2'}}
        self.old = copy.deepcopy(self.kube.deployments['voice-gameintegration'])
        self.old_image = self.old['spec']['template']['spec']['containers'][0]['image']
        self.new_image = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'b' * 64

    def capture(self):
        return runtime_stage.RolloutStage.capture(self.kube, '123456abcdef', lambda _: None,
            ['voice-gameintegration'], gis_binding=self.binding)

    def source_fixture(self):
        return {'schema': 'voice-source-authority-v1',
            'repository': source_authority.REPO, 'source_sha': 'a' * 40,
            'workflow_id': source_authority.WORKFLOW, 'run_id': 17, 'run_attempt': 1,
            'tree_sha': 'b' * 40, 'source_files': {'deploy/staging/services.yaml': 'c' * 64},
            'images': {name: source_authority.REGISTRY + '/' + name + '@sha256:' + 'b' * 64
                for name in source_authority.IMAGE_NAMES}, 'lock_artifact_id': 19,
            'lock_sha256': 'd' * 64,
            'image_provenance': 'independently resolved trusted CI tag', 'verified': True}

    def compiled_target(self, stage, target_sa=False):
        source = Path(__file__).resolve().parents[3]
        desired = copy.deepcopy(self.old)
        desired['spec']['template']['spec']['containers'][0]['image'] = self.new_image
        if target_sa:
            desired['spec']['template']['spec']['serviceAccountName'] = 'gis-target'
        return compiler.compile_target(source, [desired], 'ghcr.io/poryadok/voiceroot',
            'a' * 40, 'images-only', ['gameintegration'],
            {'voice-gameintegration/gameintegration': self.new_image}, {},
            set(stage.snapshots), ('deploy/staging/services.yaml',))

    def test_two_real_captures_and_compiler_bind_paused_pair_before_mutation(self):
        first, second = self.capture(), self.capture()
        for stage in (first, second):
            self.assertEqual(stage.snapshots['voice-gameintegration']['metadata']['uid'],
                self.binding['deployment_uid'])
            self.assertEqual(stage.old_images['voice-gameintegration/gameintegration'], self.old_image)
        compiled = self.compiled_target(second)
        documents, image_target = normalize.image_only_documents(
            second, compiled['manifests'], compiled['target'])
        emitted = []
        target = normalize.normalize_target(self.kube, second, documents,
            image_target, gis_binding=self.binding,
            gis_target_sink=lambda *pair: emitted.append(pair))
        self.assertEqual(len(emitted), 1)
        deployment, service, selected = emitted[0]
        self.assertTrue(selected)
        self.assertEqual(deployment['spec']['replicas'], 0)
        self.assertEqual(service['metadata']['uid'], self.binding['service_uid'])
        bound = existing.bind_normalized_target(self.binding, deployment, service, selected,
            'images-only', target['gis_existing_normalized_sha256'])
        self.kube.deployments['voice-gameintegration']['spec']['replicas'] = 0
        existing.verify_phase(bound, self.kube, 'paused-old')
        self.kube.deployments['voice-gameintegration'] = copy.deepcopy(deployment)
        existing.verify_phase(bound, self.kube, 'paused-target')
        self.kube.deployments['voice-gameintegration']['spec']['replicas'] = 1
        existing.verify_phase(bound, self.kube, 'ready-target')

    def test_second_capture_blocks_identity_or_runtime_drift_before_compile(self):
        self.capture()
        for failure in ('deployment_uid', 'service_uid', 'runtime_image', 'namespace_terminating'):
            with self.subTest(failure=failure):
                saved = copy.deepcopy(self.kube)
                if failure == 'deployment_uid':
                    self.kube.deployments['voice-gameintegration']['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                elif failure == 'service_uid':
                    self.kube.gis_kube.objects[gis.SERVICE]['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                elif failure == 'runtime_image':
                    self.kube.runtime_gis_image = 'example.invalid/foreign@sha256:' + 'c' * 64
                else:
                    self.kube.gis_kube.objects[('Namespace', gis.NS)] = {'metadata': {
                        'uid': self.binding['namespace_uid'], 'deletionTimestamp': '2026-10-28T00:00:00Z'}}
                with self.assertRaises((gis.Blocked, runtime_stage.Blocked)):
                    self.capture()
                self.kube = saved

    def ready_fixture(self):
        kube, state, receipt = fixtures.first_install()
        binding = existing.enroll_first_install(receipt, state, kube)
        deployment = copy.deepcopy(kube.objects[gis.DEPLOYMENT])
        deployment['spec']['template']['spec']['containers'][0]['image'] = self.new_image
        binding = existing.bind_target(binding, deployment)
        kube.objects[gis.DEPLOYMENT] = deployment
        kube._ready_objects()
        for obj in (kube.rs, kube.pod, kube.slice):
            obj['metadata']['resourceVersion'] = '1'
        kube.rs['spec'] = {'template': copy.deepcopy(deployment['spec']['template'])}
        pod = kube.pod
        pod['spec'] = copy.deepcopy(deployment['spec']['template']['spec'])
        pod['spec'].update({'serviceAccountName': 'default', 'serviceAccount': 'default',
            'schedulerName': 'default-scheduler', 'restartPolicy': 'Always',
            'dnsPolicy': 'ClusterFirst', 'enableServiceLinks': True, 'priority': 0,
            'preemptionPolicy': 'PreemptLowerPriority', 'terminationGracePeriodSeconds': 30,
            'hostNetwork': False, 'hostPID': False, 'hostIPC': False, 'securityContext': {},
            'nodeName': 'fixture-node', 'tolerations': [
                {'key': 'node.kubernetes.io/not-ready', 'operator': 'Exists',
                 'effect': 'NoExecute', 'tolerationSeconds': 300},
                {'key': 'node.kubernetes.io/unreachable', 'operator': 'Exists',
                 'effect': 'NoExecute', 'tolerationSeconds': 300}]})
        pod['spec']['volumes'] = [{'name': 'kube-api-access-abc12', 'projected': {
            'defaultMode': 420, 'sources': [
                {'serviceAccountToken': {'expirationSeconds': 3607, 'path': 'token'}},
                {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}},
                {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {
                    'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}]}}]
        pod['spec']['containers'][0]['volumeMounts'] = [{
            'name': 'kube-api-access-abc12',
            'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True}]
        pod['status']['phase'] = 'Running'
        pod['status']['containerStatuses'] = [{'name': 'gameintegration', 'ready': True,
            'imageID': self.new_image}]
        kube.objects[('Serviceaccount', 'default')] = {'metadata': {
            'name': 'default', 'namespace': gis.NS, 'resourceVersion': '1',
            'uid': '00000000-0000-0000-0000-000000000101'}}
        account = existing.service_account_preflight(kube, deployment['spec']['template']['spec'])
        return kube, binding, account

    def test_restart_runs_hub_first_and_gis_proof_before_marker(self):
        # Existing retirement CAS fixture models the core workloads; GIS owned
        # readiness comes from ReadyKube. Storage sealing and pod drain are I/O seams.
        import operator_retirement_test as retirement_fixture
        with tempfile.TemporaryDirectory() as folder:
            fixture = retirement_fixture.RetirementActionTests().fixture(folder)
            context = fixture[2]['context']
            objects = copy.deepcopy(fixture[4])
            marker_key = ('configmap', 'voice-nats-generation')
            objects[marker_key]['data']['phase'] = 'rollout-verified'
            ready, binding, account = self.ready_fixture()
            deployment = copy.deepcopy(ready.objects[gis.DEPLOYMENT])
            deployment['spec']['replicas'] = 0
            ready.objects[gis.DEPLOYMENT] = copy.deepcopy(deployment)
            objects[('deployment', 'voice-gameintegration')] = deployment
            ready_proof = []

            class CombinedKube(retirement_fixture.FakeKube):
                def get(inner, kind, name):
                    if (kind.lower(), name) == ('deployment', 'voice-gameintegration'):
                        return super().get(kind, name)
                    if kind.lower() in ('namespace', 'serviceaccount', 'pod') or (kind.lower(), name) == ('service', 'voice-gameintegration'):
                        return ready.get(kind, name)
                    return super().get(kind, name)
                def run(inner, args, body=None, timeout=30):
                    return ready.run(args, body=body)
                def cas(inner, kind, row, patches):
                    if kind.lower() == 'configmap' and row['metadata']['name'] == 'voice-nats-generation':
                        self.assertEqual(len(ready_proof), 1)
                    result = super().cas(kind, row, patches)
                    if (kind.lower(), row['metadata']['name']) == ('deployment', 'voice-gameintegration'):
                        ready.objects[gis.DEPLOYMENT] = copy.deepcopy(result)
                    return result

            kube = CombinedKube(objects)
            stage = object.__new__(runtime_stage.RolloutStage)
            stage.kube = kube
            stage.operation = fixture[2]['operation']
            stage.expected = context['expected']
            stage.marker = copy.deepcopy(objects[marker_key])
            stage.final_claim = context['final_claim']
            stage.snapshots = copy.deepcopy(context['snapshots'])
            stage.snapshots['voice-gameintegration'] = copy.deepcopy(deployment)
            stage.no_pods = lambda _names: None
            stage.seal_before_business_resume = lambda: None
            stage.save = lambda _event: None
            progress = []
            stage.closed_preservation_release = lambda _stage, action, name=None: progress.append((action, name))
            def start_guard(resumed):
                existing.verify_phase(binding, resumed.kube, 'paused-target')
                self.assertEqual(existing.service_account_preflight(resumed.kube,
                    binding['target']['spec']['template']['spec']), account)
            stage.gis_start_guard = start_guard
            stage.gis_ready_proof = lambda resumed: ready_proof.append(existing.verify_ready(binding, resumed.kube, account))
            result = stage.restart()
            self.assertTrue(result['verified'])
            scales = [name for kind, name, patches in kube.patches if kind == 'deployment'
                and any(item['path'] == '/spec/replicas' for item in patches)]
            self.assertEqual(scales[0], runtime_stage.HUB)
            self.assertIn('voice-gameintegration', scales)
            self.assertEqual(progress[0], ('scale-intent', runtime_stage.HUB))
            self.assertEqual(progress[-2:], [('release-intent', None), ('release-result', None)])
            self.assertEqual(kube.objects[marker_key]['data']['phase'], 'active')

    def test_release_requires_fresh_owned_ready_endpoint_and_runtime_image(self):
        kube, binding, account = self.ready_fixture()
        stage = object.__new__(runtime_stage.RolloutStage)
        stage.snapshots = {'voice-gameintegration': {}}
        stage.expected = {'source_claim': 'selected'}
        stage.owned_marker = Mock(return_value={'data': {'phase': 'rollout-verified'}})
        stage.scale = Mock()
        stage.wait_ready = Mock()
        stage.kube = Mock()
        stage.save = Mock()
        stage.gis_start_guard = lambda _stage: None
        stage.kube.cas.return_value = {'data': {'phase': 'active'}}
        for fault in ('runtime', 'extra-env', 'extra-volume', 'host-network', 'service-account',
                      'foreign-endpoint', 'stale-replicaset', 'terminating-pod', 'terminating-deployment'):
            with self.subTest(fault=fault):
                modified = copy.deepcopy(kube)
                if fault == 'runtime':
                    modified.pod['status']['containerStatuses'][0]['imageID'] = self.old_image
                elif fault == 'extra-env':
                    modified.pod['spec']['containers'][0]['env'] = [{'name': 'EVIL', 'value': '1'}]
                elif fault == 'extra-volume':
                    modified.pod['spec']['volumes'].append({'name': 'extra', 'emptyDir': {}})
                elif fault == 'host-network':
                    modified.pod['spec']['hostNetwork'] = True
                elif fault == 'service-account':
                    modified.objects[('Serviceaccount', 'default')]['metadata']['uid'] = '00000000-0000-0000-0000-000000000102'
                elif fault == 'foreign-endpoint':
                    modified.slice['endpoints'][0]['targetRef']['uid'] = '00000000-0000-0000-0000-000000000199'
                elif fault == 'stale-replicaset':
                    modified.rs['spec']['template']['spec']['containers'][0]['image'] = self.old_image
                elif fault == 'terminating-pod':
                    modified.pod['metadata']['deletionTimestamp'] = '2026-10-28T00:00:00Z'
                else:
                    modified.objects[gis.DEPLOYMENT]['metadata']['deletionTimestamp'] = '2026-10-28T00:00:00Z'
                stage.kube.cas.reset_mock()
                stage.gis_ready_proof = lambda _: existing.verify_ready(binding, modified, account)
                with self.assertRaises(gis.Blocked):
                    stage.release_marker()
                stage.kube.cas.assert_not_called()
        del stage.gis_ready_proof
        with self.assertRaises(runtime_stage.Blocked):
            stage.release_marker()
        stage.kube.cas.assert_not_called()
        stage.gis_ready_proof = lambda _: existing.verify_ready(binding, kube, account)
        stage.release_marker()
        stage.kube.cas.assert_called_once()

    def test_release_rechecks_target_service_account_immediately_before_gis_scale(self):
        target = copy.deepcopy(self.old)
        target['spec']['template']['spec']['containers'][0]['image'] = self.new_image
        binding = existing.bind_target(self.binding, target)
        for fault in ('none', 'missing_guard', 'uid', 'resourceVersion'):
            with self.subTest(fault=fault):
                kube = copy.deepcopy(self.kube)
                kube.deployments['voice-gameintegration'] = copy.deepcopy(target)
                kube.deployments['voice-gameintegration']['spec']['replicas'] = 0
                account = existing.service_account_preflight(kube, target['spec']['template']['spec'])
                if fault == 'uid':
                    kube.service_account['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                elif fault == 'resourceVersion':
                    kube.service_account['metadata']['resourceVersion'] = '2'
                stage = object.__new__(runtime_stage.RolloutStage)
                stage.snapshots = {'voice-gameintegration': {}}
                stage.expected = {'source_claim': 'selected'}
                stage.owned_marker = Mock(return_value={'data': {'phase': 'rollout-verified'}})
                stage.scale = Mock()
                stage.wait_ready = Mock()
                stage.kube = Mock()
                stage.kube.cas.return_value = {'data': {'phase': 'active'}}
                stage.save = Mock()
                stage.gis_ready_proof = Mock()
                if fault != 'missing_guard':
                    def start_guard(_stage):
                        self.assertIs(_stage, stage)
                        existing.verify_phase(binding, kube, 'paused-target')
                        if existing.service_account_preflight(kube, binding['target']['spec']['template']['spec']) != account:
                            raise gis.Blocked('gis_existing_service_account_changed')
                    stage.gis_start_guard = start_guard
                if fault == 'none':
                    stage.release_marker()
                    stage.scale.assert_called_once_with('voice-gameintegration', 1)
                    stage.kube.cas.assert_called_once()
                else:
                    expected = ('gis_existing_start_guard_missing' if fault == 'missing_guard'
                                else 'gis_existing_service_account_changed')
                    with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), expected):
                        stage.release_marker()
                    stage.scale.assert_not_called()
                    stage.kube.cas.assert_not_called()

    def test_unproved_wiring_finish_cannot_destroy_first_install_anchor(self):
        kube, state, receipt = fixtures.first_install()
        state['schema'] = 'nats-rollout-root-v1'
        state['context'] = {'expected': {'namespace_uid': receipt['namespace_uid'],
            'source_claim_uid': '00000000-0000-0000-0000-000000000012'}}
        state['target']['manifest_sha256'] = '9' * 64
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            installed = root / 'installed'
            installed.mkdir()
            operation = root / ('rollout-' + state['operation'])
            operation.mkdir()
            (operation / 'checkpoint.json').write_text(json.dumps(state), encoding='utf-8')
            def read(path, **kwargs):
                return json.loads(Path(path).read_text(encoding='utf-8'))
            def save(path, value):
                Path(path).write_text(json.dumps(value), encoding='utf-8')
            actions = object.__new__(bridge_root.Actions)
            actions._installation = resource_claim.production_installation()
            with patch.object(bridge_root, 'INSTALLED', installed), patch.object(guard, 'ROOT', root), \
                    patch.object(bridge_root, 'private_json', side_effect=read), \
                    patch.object(bridge_root, 'save', side_effect=save):
                actions._active_release(state)
                anchor = read(installed / 'active-release.json')['gis_first_install_transition']
                binding = actions._previous_gis_transition(kube)
                incomplete = {'operation': 'b' * 12, 'status': 'PASS', 'context': state['context'],
                    'target': {'manifest_sha256': 'a' * 64}, 'gis_transition': binding}
                try:
                    actions._active_release(incomplete)
                except (bridge_root.Blocked, gis.Blocked):
                    pass
                active = read(installed / 'active-release.json')
                self.assertIn('gis_first_install_transition', active)
                self.assertEqual(active['gis_first_install_transition'], anchor)

    def test_replaying_old_finish_does_not_replace_later_active_release(self):
        _, first, receipt = fixtures.first_install()
        first['schema'] = 'nats-rollout-root-v1'
        first['context'] = {'expected': {'namespace_uid': receipt['namespace_uid'],
            'source_claim_uid': '00000000-0000-0000-0000-000000000012'}}
        first['target']['manifest_sha256'] = '9' * 64
        later = {'operation': 'c' * 12, 'status': 'PASS', 'context': first['context'],
            'target': {'mode': 'images-only', 'manifest_sha256': '8' * 64}}
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            installed = root / 'installed'
            installed.mkdir()
            for state in (first, later):
                directory = root / ('rollout-' + state['operation'])
                directory.mkdir()
                (directory / 'checkpoint.json').write_text(json.dumps(state), encoding='utf-8')
            def read(path, **kwargs):
                return json.loads(Path(path).read_text(encoding='utf-8'))
            def save(path, value):
                Path(path).write_text(json.dumps(value), encoding='utf-8')
            actions = object.__new__(bridge_root.Actions)
            actions._installation = resource_claim.production_installation()
            with patch.object(bridge_root, 'INSTALLED', installed), patch.object(guard, 'ROOT', root), \
                    patch.object(bridge_root, 'private_json', side_effect=read), \
                    patch.object(bridge_root, 'save', side_effect=save):
                actions._active_release(first)
                active = read(installed / 'active-release.json')
                self.assertIn('gis_first_install_transition', active)
                # Fixture begins after B was published; it does not claim to
                # exercise or authorize publication of this abbreviated B.
                active['operation'] = later['operation']
                active['target_sha256'] = later['target']['manifest_sha256']
                save(installed / 'active-release.json', active)
                try:
                    actions._active_release(first)
                except (bridge_root.Blocked, gis.Blocked):
                    pass
                self.assertEqual(read(installed / 'active-release.json'), active)

    def test_same_operation_replay_is_noop_only_for_exact_release(self):
        _, state, receipt = fixtures.first_install()
        state['schema'] = 'nats-rollout-root-v1'
        state['context'] = {'expected': {'namespace_uid': receipt['namespace_uid'],
            'source_claim_uid': '00000000-0000-0000-0000-000000000012'}}
        state['target']['manifest_sha256'] = '9' * 64
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            installed = root / 'installed'
            installed.mkdir()
            directory = root / ('rollout-' + state['operation'])
            directory.mkdir()
            (directory / 'checkpoint.json').write_text(json.dumps(state), encoding='utf-8')
            def read(path, **kwargs):
                return json.loads(Path(path).read_text(encoding='utf-8'))
            def save(path, value):
                Path(path).write_text(json.dumps(value), encoding='utf-8')
            actions = object.__new__(bridge_root.Actions)
            actions._installation = resource_claim.production_installation()
            with patch.object(bridge_root, 'INSTALLED', installed), patch.object(guard, 'ROOT', root), \
                    patch.object(bridge_root, 'private_json', side_effect=read), \
                    patch.object(bridge_root, 'save', side_effect=save) as save_mock:
                actions._active_release(state)
                active = read(installed / 'active-release.json')
                save_mock.reset_mock()
                actions._active_release(copy.deepcopy(state))
                self.assertEqual(read(installed / 'active-release.json'), active)
                save_mock.assert_not_called()
                for field in ('namespace_uid', 'pvc_uid', 'target_sha256'):
                    with self.subTest(field=field):
                        altered = copy.deepcopy(state)
                        if field == 'namespace_uid':
                            altered['context']['expected']['namespace_uid'] = '00000000-0000-0000-0000-000000000099'
                        elif field == 'pvc_uid':
                            altered['context']['expected']['source_claim_uid'] = '00000000-0000-0000-0000-000000000099'
                        else:
                            altered['target']['manifest_sha256'] = '8' * 64
                        with self.assertRaises((bridge_root.Blocked, gis.Blocked)):
                            actions._active_release(altered)
                        self.assertEqual(read(installed / 'active-release.json'), active)
                        save_mock.assert_not_called()

    def check_transaction_captures_qualified_gis_and_normalizes_before_fence(self, mode, fault=None, target_sa=False, reject_target_sa=False):
        # Stop at the next external prerequisite, after the real capture and
        # normalizer. The fake API never sends a mutating Kubernetes request.
        class ReachedPrerequisite(Exception):
            pass
        _, first, receipt = fixtures.first_install()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'installed').mkdir()
            original = root / ('rollout-' + first['operation'])
            original.mkdir()
            (original / 'checkpoint.json').write_text(json.dumps(first), encoding='utf-8')
            (root / 'installed' / 'active-release.json').write_text(json.dumps({
                'gis_first_install_transition': {'operation': first['operation'],
                    'receipt_sha256': receipt['receipt_sha256']}}), encoding='utf-8')
            def read(path, **kwargs):
                return json.loads(Path(path).read_text(encoding='utf-8'))
            with patch.object(guard, 'ROOT', root), patch.object(existing, 'private_json', side_effect=read):
                binding = existing.load_authority(self.kube)
                stage = self.capture()
                pack = self.compiled_target(stage, target_sa=target_sa)
                pack['target']['mode'] = mode
                base = root / 'rollout-123456abcdef'
                base.mkdir()
                (base / 'apply-manifests.json').write_text(json.dumps(pack['manifests']), encoding='utf-8')
                authority = self.source_fixture()
                kw = {'gis_source_authority': authority}
                if 'gis_binding' in inspect.signature(transaction.prepare).parameters:
                    kw['gis_binding'] = binding
                if fault == 'missing':
                    kw['gis_source_authority'] = None
                elif fault == 'source_sha':
                    authority['source_sha'] = 'f' * 40
                elif fault == 'image':
                    authority['images']['gameintegration'] = source_authority.REGISTRY + '/gameintegration@sha256:' + 'e' * 64
                elif fault == 'image_type':
                    authority['images']['gameintegration'] = None
                elif fault == 'provenance':
                    authority['image_provenance'] = 'caller assertion'
                checkpoints = []
                def save_snapshot(path, value):
                    if Path(path).name == 'checkpoint.json':
                        checkpoints.append(copy.deepcopy(value))
                with patch.object(transaction, 'private_json', side_effect=read), \
                        patch.object(transaction, 'save', side_effect=save_snapshot), \
                        patch.object(transaction.migrations, 'preflight', side_effect=ReachedPrerequisite) as preflight, \
                        patch.object(runtime_stage.RolloutStage, 'fence') as fence, \
                        patch('operator_retirement.record_readonly_failure', side_effect=lambda _b, state, _e: state), \
                        patch.object(transaction, 'normalize_target', wraps=normalize.normalize_target) as normalizer:
                    if reject_target_sa:
                        with self.assertRaisesRegex(gis.Blocked, 'gis_existing_service_account_unsupported'):
                            transaction.prepare(self.kube, base, root, pack['target'], {},
                                '123456abcdef', {}, [], {'actions': []}, **kw)
                        preflight.assert_not_called()
                        fence.assert_not_called()
                        return
                    if fault:
                        expected_error = ('gis_existing_source_image_changed' if fault == 'image'
                                          else 'gis_existing_source_authority_invalid')
                        with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), expected_error):
                            transaction.prepare(self.kube, base, root, pack['target'], {},
                                '123456abcdef', {}, [], {'actions': []}, **kw)
                        preflight.assert_not_called()
                        fence.assert_not_called()
                        return
                    with self.assertRaises(ReachedPrerequisite):
                        transaction.prepare(self.kube, base, root, pack['target'], {},
                            '123456abcdef', {}, [], {'actions': []}, **kw)
                    self.assertEqual(normalizer.call_args.kwargs['gis_binding'], binding)
                    self.assertTrue(callable(normalizer.call_args.kwargs['gis_target_sink']))
                    recorded = checkpoints[-1]
                    self.assertIn('gis_normalized_pair', recorded)
                    self.assertEqual(recorded['gis_source_authority'], authority)
                    pair = recorded['gis_normalized_pair']
                    self.assertEqual(pair['deployment_mode'], 'selected-paused')
                    self.assertEqual(pair['service_uid'], binding['service_uid'])
                    self.assertEqual(pair['deployment_uid'], binding['deployment_uid'])
                    self.assertEqual(pair['deployment']['spec']['replicas'], 0)
                    self.assertEqual(recorded['target']['gis_existing_normalized_sha256'], gis.digest(pair))
                    self.assertEqual(recorded['gis_binding']['service_uid'], binding['service_uid'])
                    self.assertEqual(recorded['gis_binding']['deployment_uid'], binding['deployment_uid'])
                    self.assertEqual(recorded['gis_binding']['target_service'], pair['service'])
                    expected = copy.deepcopy(pair['deployment'])
                    expected['spec']['replicas'] = 1
                    self.assertEqual(recorded['gis_binding']['target'], expected)
                    account = existing.service_account_preflight(self.kube, expected['spec']['template']['spec'])
                    self.assertEqual(recorded['gis_service_account'], account)
                    if target_sa:
                        self.assertEqual(account['name'], 'gis-target')
                        self.assertEqual(account['uid'], self.kube.target_account['metadata']['uid'])

    def test_transaction_captures_qualified_gis_and_normalizes_before_fence(self):
        for mode in ('images-only', 'full', 'app-only'):
            with self.subTest(mode=mode):
                self.check_transaction_captures_qualified_gis_and_normalizes_before_fence(mode)

    def test_transaction_rejects_missing_or_mismatched_source_authority(self):
        for fault in ('missing', 'source_sha', 'image', 'image_type', 'provenance'):
            with self.subTest(fault=fault):
                self.check_transaction_captures_qualified_gis_and_normalizes_before_fence('images-only', fault)

    def test_pre_fence_rechecks_target_service_account_after_capture(self):
        class ReachedFence(Exception):
            pass
        _, first, receipt = fixtures.first_install()
        for fault in ('none', 'uid', 'resourceVersion'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / 'installed').mkdir()
                original = root / ('rollout-' + first['operation'])
                original.mkdir()
                (original / 'checkpoint.json').write_text(json.dumps(first), encoding='utf-8')
                (root / 'installed' / 'active-release.json').write_text(json.dumps({
                    'gis_first_install_transition': {'operation': first['operation'],
                        'receipt_sha256': receipt['receipt_sha256']}}), encoding='utf-8')
                def read(path, **kwargs):
                    return json.loads(Path(path).read_text(encoding='utf-8'))
                kube = copy.deepcopy(self.kube)
                original_get = kube.get
                def get(kind, name):
                    row = original_get(kind, name)
                    if kind == 'pvc':
                        row['status'] = {'capacity': {'storage': '1Gi'}}
                    return row
                kube.get = get
                stage = runtime_stage.RolloutStage.capture(kube, '123456abcdef', lambda _: None,
                    ['voice-gameintegration'], gis_binding=self.binding)
                pack = self.compiled_target(stage)
                nonnats = {'actions': [], 'checks': []}
                pack['target']['nonnats_sha256'] = gis.digest(nonnats)
                base = root / 'rollout-123456abcdef'
                base.mkdir()
                (base / 'apply-manifests.json').write_text(json.dumps(pack['manifests']), encoding='utf-8')
                (root / 'kernel').write_bytes(b'fixture')
                def capture_inputs(*_args, **_kwargs):
                    if fault == 'uid':
                        kube.service_account['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                    elif fault == 'resourceVersion':
                        kube.service_account['metadata']['resourceVersion'] = '2'
                    return {}
                resource = Mock()
                resource.installation = resource_claim.production_installation()
                pack['target']['installation_sha256'] = resource.installation.sha256
                resource.binding.return_value = {'fixture': 'tt-bound'}
                with patch.object(guard, 'ROOT', root), patch.object(existing, 'private_json', side_effect=read), \
                        patch.object(transaction, 'private_json', side_effect=read), \
                        patch.object(transaction.migrations, 'preflight', return_value={}), \
                        patch.object(transaction.space_safeguards, 'preflight', return_value=None), \
                        patch.object(transaction.space_safeguards, 'combined_capacity', return_value={}), \
                        patch.object(transaction.nonnats_runtime, 'preflight', return_value={}), \
                        patch.object(transaction.nonnats_plan, 'revalidate'), \
                        patch.object(source_plan, 'verify_db_init'), \
                        patch.object(transaction, 'native_preflight', return_value={}), \
                        patch.object(transaction, 'capture_inputs', side_effect=capture_inputs), \
                        patch.object(runtime_stage.RolloutStage, 'fence', side_effect=ReachedFence) as fence, \
                        patch('operator_retirement.record_readonly_failure', side_effect=lambda _b, value, _e: value):
                    if fault == 'none':
                        with self.assertRaises(ReachedFence):
                            transaction.prepare(kube, base, root, pack['target'], {}, '123456abcdef',
                                {}, [], nonnats, gis_binding=self.binding, gis_source_authority=self.source_fixture(),
                                resource_session=resource)
                        fence.assert_called_once()
                    else:
                        with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked),
                                'gis_existing_service_account_changed'):
                            transaction.prepare(kube, base, root, pack['target'], {}, '123456abcdef',
                                {}, [], nonnats, gis_binding=self.binding, gis_source_authority=self.source_fixture(),
                                resource_session=resource)
                        fence.assert_not_called()

    def test_transaction_rechecks_pair_and_sa_after_authority_and_tt_bind(self):
        class ReachedFence(Exception):
            pass
        _, first, receipt = fixtures.first_install()
        for fault in ('none', 'missing_tt', 'before_sa', 'before_service', 'before_active',
                      'after_sa', 'after_deployment', 'after_active'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / 'installed').mkdir()
                original = root / ('rollout-' + first['operation'])
                original.mkdir()
                (original / 'checkpoint.json').write_text(json.dumps(first), encoding='utf-8')
                (root / 'installed' / 'active-release.json').write_text(json.dumps({
                    'gis_first_install_transition': {'operation': first['operation'],
                        'receipt_sha256': receipt['receipt_sha256']}}), encoding='utf-8')
                def read(path, **kwargs):
                    return json.loads(Path(path).read_text(encoding='utf-8'))
                kube = copy.deepcopy(self.kube)
                original_get = kube.get
                def get(kind, name):
                    row = original_get(kind, name)
                    if kind == 'pvc': row['status'] = {'capacity': {'storage': '1Gi'}}
                    return row
                kube.get = get
                kube.cas = Mock()
                stage = runtime_stage.RolloutStage.capture(kube, '123456abcdef', lambda _: None,
                    ['voice-gameintegration'], gis_binding=self.binding)
                pack = self.compiled_target(stage)
                nonnats = {'actions': [], 'checks': []}
                pack['target']['nonnats_sha256'] = gis.digest(nonnats)
                base = root / 'rollout-123456abcdef'
                base.mkdir()
                (base / 'apply-manifests.json').write_text(json.dumps(pack['manifests']), encoding='utf-8')
                (root / 'kernel').write_bytes(b'fixture')
                order = []
                def before_fence():
                    order.append('authority')
                    if fault == 'before_sa':
                        kube.service_account['metadata']['resourceVersion'] = '2'
                    elif fault == 'before_service':
                        kube.gis_kube.objects[gis.SERVICE]['spec']['selector']['app'] = 'foreign'
                    elif fault == 'before_active':
                        (root / 'installed' / 'active-release.json').write_text(json.dumps({'changed': True}), encoding='utf-8')
                context = resource_claim_test.context()
                context.update(commit=pack['target']['tag'], images=copy.deepcopy(pack['target']['images']))
                card = resource_claim_test.card(context)
                card['observed']['images'] = copy.deepcopy(stage.old_images)
                journal = resource_claim_test.MemoryJournal()
                resource = resource_claim.Session(context, '123456abcdef', 'a' * 64,
                    lambda: copy.deepcopy(card), journal)
                resource.admit(start=True)
                pack['target']['installation_sha256'] = resource.installation.sha256
                bind_target = resource.bind_target
                def bind(tag, images, old_images, target_hash):
                    order.append('tt-bind')
                    bind_target(tag, images, old_images, target_hash)
                    if fault == 'after_sa':
                        kube.service_account['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                    elif fault == 'after_deployment':
                        kube.deployments['voice-gameintegration']['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                    elif fault == 'after_active':
                        (root / 'installed' / 'active-release.json').write_text(json.dumps({'changed': True}), encoding='utf-8')
                resource.bind_target = bind
                def fence(_stage):
                    order.append('fence')
                    raise ReachedFence()
                with patch.object(guard, 'ROOT', root), patch.object(existing, 'private_json', side_effect=read), \
                        patch.object(transaction, 'private_json', side_effect=read), \
                        patch.object(transaction.migrations, 'preflight', return_value={}), \
                        patch.object(transaction.space_safeguards, 'preflight', return_value=None), \
                        patch.object(transaction.space_safeguards, 'combined_capacity', return_value={}), \
                        patch.object(transaction.nonnats_runtime, 'preflight', return_value={}), \
                        patch.object(transaction.nonnats_plan, 'revalidate'), \
                        patch.object(source_plan, 'verify_db_init'), \
                        patch.object(transaction, 'native_preflight', return_value={}), \
                        patch.object(transaction, 'capture_inputs', return_value={}), \
                        patch.object(runtime_stage.RolloutStage, 'fence', autospec=True, side_effect=fence) as fence_call, \
                        patch.object(runtime_stage.RolloutStage, 'scale', autospec=True) as scale, \
                        patch('operator_retirement.record_readonly_failure', side_effect=lambda _b, value, _e: value):
                    if fault == 'none':
                        with self.assertRaises(ReachedFence):
                            transaction.prepare(kube, base, root, pack['target'], {}, '123456abcdef',
                                {}, [], nonnats, before_fence=before_fence, resource_session=resource,
                                gis_binding=self.binding, gis_source_authority=self.source_fixture())
                        self.assertEqual(order, ['authority', 'tt-bind', 'fence'])
                        fence_call.assert_called_once()
                        checkpoint = json.loads((base / 'checkpoint.json').read_text(encoding='utf-8'))
                        self.assertEqual(resource.binding()['target_sha256'], gis.digest(checkpoint['target']))
                        self.assertEqual(resource.check()['target_sha256'], gis.digest(checkpoint['target']))
                        self.assertEqual(checkpoint['resource_fence'], resource.binding())
                        resource.require_binding(checkpoint['resource_fence'])
                        self.assertEqual(resource.context['commit'], checkpoint['target']['tag'])
                        self.assertEqual(resource.context['images'], checkpoint['target']['images'])
                    else:
                        expected = ('gis_existing_resource_fence_required' if fault == 'missing_tt'
                            else 'gis_existing_service_account_changed' if fault.endswith('_sa')
                            else 'gis_existing_authority_changed' if fault.endswith('_active')
                            else 'gis_existing_identity_changed')
                        with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), expected):
                            transaction.prepare(kube, base, root, pack['target'], {}, '123456abcdef',
                                {}, [], nonnats, before_fence=before_fence,
                                resource_session=None if fault == 'missing_tt' else resource,
                                gis_binding=self.binding, gis_source_authority=self.source_fixture())
                        self.assertEqual(order, ['authority'] if fault.startswith('before_') or fault == 'missing_tt'
                            else ['authority', 'tt-bind'])
                        fence_call.assert_not_called()
                    checkpoint = json.loads((base / 'checkpoint.json').read_text(encoding='utf-8'))
                    self.assertEqual(checkpoint['phase'], 'FENCE')
                    self.assertEqual(checkpoint['status'], 'BLOCKED')
                    self.assertEqual(checkpoint['fence_status'], 'UNKNOWN')
                    self.assertEqual('resource_fence' in checkpoint, 'tt-bind' in order)
                    scale.assert_not_called()
                    kube.cas.assert_not_called()

    def test_prepare_and_authorize_share_qualified_binding_and_tt_target(self):
        # Actual capture, normalization, TT Session and authorization. Fence,
        # prepared and storage guards plus archive/migration I/O are simulated.
        _, first, receipt = fixtures.first_install()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'installed').mkdir()
            original = root / ('rollout-' + first['operation'])
            original.mkdir()
            (original / 'checkpoint.json').write_text(json.dumps(first), encoding='utf-8')
            (root / 'installed' / 'active-release.json').write_text(json.dumps({
                'gis_first_install_transition': {'operation': first['operation'],
                    'receipt_sha256': receipt['receipt_sha256']}}), encoding='utf-8')
            def read(path, **_kwargs):
                return json.loads(Path(path).read_text(encoding='utf-8'))
            kube = copy.deepcopy(self.kube)
            get = kube.get
            def capacity(kind, name):
                row = get(kind, name)
                if kind == 'pvc': row['status'] = {'capacity': {'storage': '1Gi'}}
                return row
            kube.get = capacity
            captured = runtime_stage.RolloutStage.capture(kube, '123456abcdef', lambda _: None,
                ['voice-gameintegration'], gis_binding=self.binding)
            pack = self.compiled_target(captured)
            nonnats = {'actions': [], 'checks': []}
            pack['target']['nonnats_sha256'] = gis.digest(nonnats)
            base = root / 'rollout-123456abcdef'
            base.mkdir()
            (base / 'apply-manifests.json').write_text(json.dumps(pack['manifests']), encoding='utf-8')
            (root / 'kernel').write_bytes(b'fixture')
            ctx = resource_claim_test.context()
            ctx.update(commit=pack['target']['tag'], images=copy.deepcopy(pack['target']['images']))
            card = resource_claim_test.card(ctx)
            card['observed']['images'] = copy.deepcopy(captured.old_images)
            helper = {'nats-rollout-preservation/resource_claim.py': 'e' * 64}
            session = resource_claim.Session(ctx, '123456abcdef', gis.digest(helper),
                lambda: copy.deepcopy(card), resource_claim_test.MemoryJournal())
            session.admit(start=True)
            pack['target']['installation_sha256'] = session.installation.sha256
            cut = {'manifest': {'archive_sha256': '6' * 64}, 'census': {'fixture': 'cut'}}
            cut['census_sha256'] = gis.digest(cut['census'])
            def fence(stage):
                kube.deployments['voice-gameintegration']['spec']['replicas'] = 0
                stage.marker['data'].setdefault('previousGeneration', 'previous')
            with contextlib.ExitStack() as stack:
                for seam in (
                    patch.object(guard, 'ROOT', root), patch.object(existing, 'private_json', side_effect=read),
                    patch.object(transaction, 'private_json', side_effect=read),
                    patch.object(transaction.migrations, 'preflight', return_value={}),
                    patch.object(transaction.migrations, 'execute', return_value=[]),
                    patch.object(transaction.space_safeguards, 'preflight', return_value=None),
                    patch.object(transaction.space_safeguards, 'combined_capacity', return_value={}),
                    patch.object(transaction.space_safeguards, 'before_migrate'),
                    patch.object(transaction.space_safeguards, 'after_migrate', return_value={}),
                    patch.object(transaction.nonnats_runtime, 'preflight', return_value={}),
                    patch.object(transaction.nonnats_runtime, 'verify'),
                    patch.object(transaction.nonnats_plan, 'revalidate'), patch.object(source_plan, 'verify_db_init'),
                    patch.object(transaction, 'native_preflight', return_value={}),
                    patch.object(transaction, 'capture_inputs', return_value={}),
                    patch.object(transaction, 'require_original_live_census', return_value={}),
                    patch.object(transaction, 'capture_cut', return_value=copy.deepcopy(cut)),
                    patch.object(transaction, 'DockerRuntime'), patch.object(transaction, 'verify_archive', return_value=True),
                    patch.object(transaction, 'revalidate_inputs'),
                    patch.object(runtime_stage.RolloutStage, 'fence', autospec=True, side_effect=fence),
                    patch.object(runtime_stage.RolloutStage, 'verify_final_storage'),
                    patch.object(runtime_stage.RolloutStage, 'prepared')):
                    stack.enter_context(seam)
                state = transaction.prepare(kube, base, root, pack['target'], {}, '123456abcdef',
                    helper, [], nonnats, resource_session=session, gis_binding=self.binding,
                    gis_source_authority=self.source_fixture())
                self.assertEqual((state['phase'], state['status']), ('AWAITING_OFF_NODE', 'WAITING'))
                bound = copy.deepcopy(state['gis_binding'])
                self.assertEqual(state['target']['gis_existing_normalized_sha256'], gis.digest(state['gis_normalized_pair']))
                self.assertEqual(state['resource_fence'], session.binding())
                self.assertEqual(session.binding()['target_sha256'], gis.digest(state['target']))
                existing.verify_phase(bound, kube, 'paused-old')
                authorization = transaction.authorize(kube, base, state, cut['manifest']['archive_sha256'],
                    state['cut']['manifest_sha256'], {}, resource_session=session)
                self.assertEqual((state['phase'], state['status']), ('PAUSED_APPLY', 'WAITING'))
                self.assertEqual(state['gis_binding'], bound)
                self.assertEqual(authorization['target'], state['target'])
                self.assertEqual(authorization['resource_fence'], session.binding())
                session.require_binding(authorization['resource_fence'])
                self.assertEqual(resource_claim.historical_binding(state), session.binding())
                self.assertEqual(json.loads((base / 'apply-authorization.json').read_text(encoding='utf-8')), authorization)
                self.assertEqual(json.loads((base / 'checkpoint.json').read_text(encoding='utf-8')), state)

    def test_full_update_records_source_selected_service_account(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode):
                self.check_transaction_captures_qualified_gis_and_normalizes_before_fence(mode, target_sa=True)

    def test_full_update_rejects_unsupported_target_service_account(self):
        self.kube.target_account['automountServiceAccountToken'] = False
        self.check_transaction_captures_qualified_gis_and_normalizes_before_fence(
            'full', target_sa=True, reject_target_sa=True)

    def test_full_and_app_only_compile_real_source_plan_with_gis_and_callers(self):
        source = Path(os.environ.get('VOICE_TEST_SOURCE_ROOT', Path(__file__).resolve().parents[3]))
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as tmp, \
                    patch.object(source_plan_test, 'SOURCE', source):
                kube, params = source_plan_test.SourceTests().canonical_fixture(mode)
                params.update(generation='r20260930a4', dataPVC='voice-nats-jsdata-d202610040049430b',
                    changed_services=[], enrolled=[], contract={'scripts': []})
                for part in ('realtime', 'notification', 'search', 'analytics-chat'):
                    raw = (source / ('deploy/templates/nats-' + part + '-bootstrap.yaml')).read_bytes().split(b'---')[0]
                    script = source_plan_test.decoder(raw)['data']['bootstrap.sh']
                    params['contract']['scripts'].append({'part': part, 'sha256': hashlib.sha256(script.encode()).hexdigest()})
                rows = compiler.rendered_source(source, params, source_plan_test.decoder)
                deployments = [row for row in rows if row.get('kind') == 'Deployment']
                params['enrolled'] = sorted(row['metadata']['name'] for row in deployments)
                images = {}
                for row in deployments:
                    name = row['metadata']['name']
                    spec = row['spec']['template']['spec']
                    for container in spec.get('initContainers', []) + spec.get('containers', []):
                        key = name + '/' + container['name']
                        images[key] = ('docker.io/library/nginx@sha256:' + 'e' * 64 if name == source_plan.PRINCIPAL_PUBLISHER
                                       else 'registry/' + container['name'] + '@sha256:' + 'a' * 64)
                params['images'] = images
                parameter_path = Path(tmp) / 'params.json'
                parameter_path.write_text(json.dumps(params), encoding='utf-8')
                output = Path(tmp) / ('target-' + params['tag'] + '.json')
                with patch.object(compiler, 'decode_yaml', side_effect=source_plan_test.decoder):
                    compiler.main([str(source), str(parameter_path), str(output)], producer=kube)
                pack = json.loads(output.read_text(encoding='utf-8'))
                identities = {(row['kind'], row['metadata']['name']) for row in pack['manifests']}
                for kind, name in (('Service', 'voice-gameintegration'), ('Deployment', 'voice-gameintegration'),
                                   ('Deployment', 'voice-role'), ('Deployment', 'voice-notification'),
                                   ('Deployment', 'voice-realtime')):
                    self.assertIn((kind, name), identities)
                self.assertEqual(pack['target']['mode'], mode)
                self.assertIn('voice-gameintegration/gameintegration', pack['target']['images'])
                self.assertTrue(pack['nonnats']['checks'])
                def object_for(kind, name):
                    return next(row for row in pack['manifests']
                        if row['kind'] == kind and row['metadata']['name'] == name)
                service = object_for('Service', 'voice-gameintegration')
                self.assertEqual(service['spec']['selector'], {'app': 'voice-gameintegration'})
                self.assertEqual(len(service['spec']['ports']), 2)
                self.assertEqual({row['name']: row['port'] for row in service['spec']['ports']},
                    {'http': 8080, 'principal-jwks': 8443})
                def env_for(name):
                    deployment = object_for('Deployment', 'voice-' + name)
                    container = next(row for row in deployment['spec']['template']['spec']['containers']
                        if row['name'] == name)
                    self.assertEqual(len(container.get('env', [])),
                        len({row['name'] for row in container.get('env', [])}))
                    return {row['name']: row for row in container.get('env', [])}
                role = env_for('role')
                issuers = json.loads(role['S2S_JWKS_URLS_JSON']['value'])
                self.assertEqual(issuers['gameintegration'],
                    'https://voice-gameintegration:8443/internal/v1/principal/jwks.json')
                gis_env = env_for('gameintegration')
                for caller in ('notification', 'realtime'):
                    key = 'GAME_INTEGRATION_' + caller.upper() + '_WORKLOAD_KEY_B64'
                    expected = {'secretKeyRef': {'name': 'voice-gameintegration-workload-credentials', 'key': key}}
                    self.assertEqual(gis_env[key]['valueFrom'], expected)
                    caller_env = env_for(caller)
                    self.assertEqual(caller_env[key]['valueFrom'], expected)
                    self.assertEqual(caller_env['GAME_INTEGRATION_CONSENT_URL']['value'],
                        'http://voice-gameintegration:8080')

    def test_transaction_blocks_drift_between_real_captures_before_fence(self):
        class ReachedPrerequisite(Exception):
            pass
        _, state, receipt = fixtures.first_install()
        first = self.capture()
        pack = self.compiled_target(first)
        for failure in ('service_uid', 'deployment_uid', 'runtime_image', 'detached_binding'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as tmp:
                kube = copy.deepcopy(self.kube)
                binding = copy.deepcopy(self.binding)
                if failure == 'service_uid':
                    kube.gis_kube.objects[gis.SERVICE]['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                elif failure == 'deployment_uid':
                    kube.deployments['voice-gameintegration']['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                elif failure == 'runtime_image':
                    kube.runtime_gis_image = 'example.invalid/foreign@sha256:' + 'c' * 64
                else:
                    binding['anchor_sha256'] = '0' * 64
                    binding = existing.seal(binding)
                root = Path(tmp)
                (root / 'installed').mkdir()
                original = root / ('rollout-' + state['operation'])
                original.mkdir()
                (original / 'checkpoint.json').write_text(json.dumps(state), encoding='utf-8')
                (root / 'installed' / 'active-release.json').write_text(json.dumps({
                    'gis_first_install_transition': {'operation': state['operation'],
                        'receipt_sha256': receipt['receipt_sha256']}}), encoding='utf-8')
                def read(path, **kwargs):
                    return json.loads(Path(path).read_text(encoding='utf-8'))
                (root / 'apply-manifests.json').write_text(json.dumps(pack['manifests']), encoding='utf-8')
                with patch.object(guard, 'ROOT', root), patch.object(existing, 'private_json', side_effect=read), \
                        patch.object(transaction, 'save'), \
                        patch.object(transaction.migrations, 'preflight', side_effect=ReachedPrerequisite) as preflight, \
                        patch.object(runtime_stage.RolloutStage, 'fence') as fence, \
                        patch('operator_retirement.record_readonly_failure', side_effect=lambda _b, state, _e: state):
                    with self.assertRaises((gis.Blocked, runtime_stage.Blocked)):
                        transaction.prepare(kube, root, root, pack['target'], {},
                            '123456abcdef', {}, [], {'actions': []}, gis_binding=binding,
                            gis_source_authority=self.source_fixture())
                    preflight.assert_not_called()
                    fence.assert_not_called()

    def test_conflicting_or_invalid_binding_never_reaches_capture_or_fence(self):
        _, state, receipt = fixtures.first_install()
        legacy = {'operation': state['operation'], 'receipt_sha256': receipt['receipt_sha256'],
            'receipt': receipt}
        self.assertTrue(gis.validate_transition_binding(legacy, self.kube))
        for failure in ('conflicting_legacy', 'invalid_binding'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / 'installed').mkdir()
                original = root / ('rollout-' + state['operation'])
                original.mkdir()
                (original / 'checkpoint.json').write_text(json.dumps(state), encoding='utf-8')
                (root / 'installed' / 'active-release.json').write_text(json.dumps({
                    'gis_first_install_transition': {'operation': state['operation'],
                        'receipt_sha256': receipt['receipt_sha256']}}), encoding='utf-8')
                def read(path, **kwargs):
                    return json.loads(Path(path).read_text(encoding='utf-8'))
                first = self.capture()
                pack = self.compiled_target(first)
                (root / 'apply-manifests.json').write_text(json.dumps(pack['manifests']), encoding='utf-8')
                binding = copy.deepcopy(self.binding)
                options = {'gis_binding': binding, 'gis_source_authority': self.source_fixture()}
                if failure == 'conflicting_legacy':
                    options['gis_transition'] = legacy
                else:
                    binding['sha256'] = '0' * 64
                with patch.object(guard, 'ROOT', root), patch.object(existing, 'private_json', side_effect=read), \
                        patch.object(transaction, 'save'), \
                        patch.object(runtime_stage.RolloutStage, 'capture') as capture, \
                        patch.object(runtime_stage.RolloutStage, 'fence') as fence, \
                        patch('operator_retirement.record_readonly_failure', side_effect=lambda _b, value, _e: value):
                    expected = ('gis_existing_admission_conflict' if failure == 'conflicting_legacy'
                                else 'gis_existing_authority_changed')
                    with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), expected):
                        transaction.prepare(self.kube, root, root, pack['target'], {},
                            '123456abcdef', {}, [], {'actions': []}, **options)
                    capture.assert_not_called()
                    fence.assert_not_called()

    def test_authorize_requires_exact_paused_old_before_prerequisites(self):
        class ReachedPrerequisite(Exception):
            pass
        target = copy.deepcopy(self.old)
        target['spec']['template']['spec']['containers'][0]['image'] = self.new_image
        binding = existing.bind_target(self.binding, target)
        for fault in ('none', 'active_replicas', 'deployment_uid', 'service_drift'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                kube = copy.deepcopy(self.kube)
                kube.deployments['voice-gameintegration']['spec']['replicas'] = 0
                if fault == 'active_replicas':
                    kube.deployments['voice-gameintegration']['spec']['replicas'] = 1
                elif fault == 'deployment_uid':
                    kube.deployments['voice-gameintegration']['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                elif fault == 'service_drift':
                    kube.gis_kube.objects[gis.SERVICE]['spec']['selector']['app'] = 'foreign'
                state = {'operation': '123456abcdef', 'phase': 'AWAITING_OFF_NODE', 'status': 'WAITING',
                    'gis_binding': binding, 'events': [], 'provenance': {},
                    'cut': {'manifest': {'archive_sha256': '6' * 64},
                            'manifest_sha256': '7' * 64, 'census_sha256': '8' * 64}}
                stage = Mock()
                with patch.object(transaction, 'file_sha', return_value='7' * 64), \
                        patch.object(transaction, 'verify_archive', return_value=True), \
                        patch.object(transaction, 'reconstruct', return_value=stage), \
                        patch.object(transaction, 'revalidate_inputs', side_effect=ReachedPrerequisite) as preflight, \
                        patch.object(transaction, 'save') as save:
                    if fault == 'none':
                        with self.assertRaises(ReachedPrerequisite):
                            transaction.authorize(kube, Path(tmp), state, '6' * 64, '7' * 64, {})
                        preflight.assert_called_once()
                    else:
                        expected = ('gis_existing_deployment_changed' if fault == 'active_replicas'
                                    else 'gis_existing_identity_changed')
                        with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), expected):
                            transaction.authorize(kube, Path(tmp), state, '6' * 64, '7' * 64, {})
                        preflight.assert_not_called()
                        save.assert_not_called()

    def test_finish_requires_exact_paused_target_before_adoption(self):
        class ReachedAdoption(Exception):
            pass
        target = copy.deepcopy(self.old)
        target['spec']['template']['spec']['containers'][0]['image'] = self.new_image
        binding = existing.bind_target(self.binding, target)
        for fault in ('none', 'active_replicas', 'deployment_uid', 'service_drift', 'wrong_image'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                kube = copy.deepcopy(self.kube)
                kube.deployments['voice-gameintegration'] = copy.deepcopy(target)
                kube.deployments['voice-gameintegration']['spec']['replicas'] = 0
                if fault == 'active_replicas':
                    kube.deployments['voice-gameintegration']['spec']['replicas'] = 1
                elif fault == 'deployment_uid':
                    kube.deployments['voice-gameintegration']['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                elif fault == 'service_drift':
                    kube.gis_kube.objects[gis.SERVICE]['spec']['selector']['app'] = 'foreign'
                elif fault == 'wrong_image':
                    kube.deployments['voice-gameintegration']['spec']['template']['spec']['containers'][0]['image'] = self.old_image
                receipt = {'operation': '123456abcdef', 'target': {'mode': 'images-only'},
                    'expires_at': (dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5)).isoformat()}
                state = {'operation': receipt['operation'], 'phase': 'PAUSED_APPLY', 'status': 'WAITING',
                    'target': receipt['target'], 'authorization': receipt, 'gis_binding': binding,
                    'events': [], 'provenance': {}}
                stage = Mock()
                stage.adopt_applied_target.side_effect = ReachedAdoption
                with patch.object(transaction, 'reconstruct', return_value=stage), \
                        patch.object(transaction, 'revalidate_inputs'), \
                        patch.object(transaction, 'save') as save:
                    if fault == 'none':
                        with self.assertRaises(ReachedAdoption):
                            transaction.finish(kube, Path(tmp), state, receipt, 'claim-rv', {})
                        stage.adopt_applied_target.assert_called_once()
                    else:
                        expected = ('gis_existing_identity_changed' if fault in ('deployment_uid', 'service_drift')
                                    else 'gis_existing_deployment_changed')
                        with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), expected):
                            transaction.finish(kube, Path(tmp), state, receipt, 'claim-rv', {})
                        stage.adopt_applied_target.assert_not_called()
                        stage.restart.assert_not_called()
                        save.assert_not_called()

    def test_finish_rechecks_target_service_account_before_restart(self):
        class ReachedRestart(Exception):
            pass
        target = copy.deepcopy(self.old)
        target['spec']['template']['spec']['containers'][0]['image'] = self.new_image
        binding = existing.bind_target(self.binding, target)
        for fault in ('none', 'uid', 'resourceVersion'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                kube = copy.deepcopy(self.kube)
                kube.deployments['voice-gameintegration'] = copy.deepcopy(target)
                kube.deployments['voice-gameintegration']['spec']['replicas'] = 0
                account = existing.service_account_preflight(kube, target['spec']['template']['spec'])
                receipt = {'operation': '123456abcdef', 'target': {'mode': 'images-only'},
                    'expires_at': (dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5)).isoformat()}
                state = {'operation': receipt['operation'], 'phase': 'PAUSED_APPLY', 'status': 'WAITING',
                    'target': receipt['target'], 'authorization': receipt, 'gis_binding': binding,
                    'gis_service_account': account, 'events': [], 'provenance': {}, 'cut': {'manifest': {}}}
                class Stage:
                    final_path = Path('/fixture-store')
                    def adopt_applied_target(self, *_args): pass
                    def verify_final_storage(self): pass
                    def verified(self): pass
                    def refence(self): return {'verified': True}
                    def restart(self): raise ReachedRestart()
                stage = Stage()
                stage.kube = kube
                def post_apply(*_args):
                    if fault == 'uid':
                        kube.service_account['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                    elif fault == 'resourceVersion':
                        kube.service_account['metadata']['resourceVersion'] = '2'
                    return {'verified': True}
                with patch.object(transaction, 'reconstruct', return_value=stage), \
                        patch.object(transaction, 'revalidate_inputs'), \
                        patch.object(transaction, 'context', return_value={}), \
                        patch.object(transaction, 'DockerRuntime'), \
                        patch.object(transaction, 'verify_post_apply', side_effect=post_apply), \
                        patch.object(transaction, 'attach_closed_preservation'), \
                        patch.object(transaction, 'save'):
                    if fault == 'none':
                        with self.assertRaises(ReachedRestart):
                            transaction.finish(kube, Path(tmp), state, receipt, 'claim-rv', {})
                    else:
                        with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked),
                                'gis_existing_service_account_changed'):
                            transaction.finish(kube, Path(tmp), state, receipt, 'claim-rv', {})

    def test_finish_installs_start_guard_for_late_service_account_drift(self):
        class ReachedScale(Exception):
            pass
        target = copy.deepcopy(self.old)
        target['spec']['template']['spec']['containers'][0]['image'] = self.new_image
        binding = existing.bind_target(self.binding, target)
        for fault in ('none', 'uid', 'resourceVersion'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                kube = copy.deepcopy(self.kube)
                kube.deployments['voice-gameintegration'] = copy.deepcopy(target)
                kube.deployments['voice-gameintegration']['spec']['replicas'] = 0
                account = existing.service_account_preflight(kube, target['spec']['template']['spec'])
                receipt = {'operation': '123456abcdef', 'target': {'mode': 'images-only'},
                    'expires_at': (dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5)).isoformat()}
                state = {'operation': receipt['operation'], 'phase': 'PAUSED_APPLY', 'status': 'WAITING',
                    'target': receipt['target'], 'authorization': receipt, 'gis_binding': binding,
                    'gis_service_account': account, 'events': [], 'provenance': {}, 'cut': {'manifest': {}}}
                test = self
                class Stage:
                    final_path = Path('/fixture-store')
                    def adopt_applied_target(self, *_args): pass
                    def verify_final_storage(self): pass
                    def verified(self): pass
                    def refence(self): return {'verified': True}
                    def restart(self):
                        if fault == 'uid':
                            kube.service_account['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                        elif fault == 'resourceVersion':
                            kube.service_account['metadata']['resourceVersion'] = '2'
                        test.assertTrue(callable(getattr(self, 'gis_start_guard', None)))
                        self.gis_start_guard(self)
                        raise ReachedScale()
                stage = Stage()
                stage.kube = kube
                with patch.object(transaction, 'reconstruct', return_value=stage), \
                        patch.object(transaction, 'revalidate_inputs'), \
                        patch.object(transaction, 'context', return_value={}), \
                        patch.object(transaction, 'DockerRuntime'), \
                        patch.object(transaction, 'verify_post_apply', return_value={'verified': True}), \
                        patch.object(transaction, 'attach_closed_preservation'), \
                        patch.object(transaction, 'save'):
                    if fault == 'none':
                        with self.assertRaises(ReachedScale):
                            transaction.finish(kube, Path(tmp), state, receipt, 'claim-rv', {})
                    else:
                        with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked),
                                'gis_existing_service_account_changed'):
                            transaction.finish(kube, Path(tmp), state, receipt, 'claim-rv', {})

    def test_finish_installs_real_ready_proof_and_persists_it_before_release(self):
        class BeforeRelease(Exception):
            pass
        kube, binding, account = self.ready_fixture()
        kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 0
        receipt = {'operation': '123456abcdef', 'target': {'mode': 'images-only'},
            'expires_at': (dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5)).isoformat()}
        state = {'operation': receipt['operation'], 'phase': 'PAUSED_APPLY', 'status': 'WAITING',
            'target': receipt['target'], 'authorization': receipt, 'gis_binding': binding,
            'gis_service_account': account, 'events': [], 'provenance': {},
            'cut': {'manifest': {}}}
        saved = []
        observed = []
        actual_verify_ready = existing.verify_ready
        def record_ready(*args):
            proof = actual_verify_ready(*args)
            observed.append(copy.deepcopy(proof))
            return proof
        def save(path, value):
            if Path(path).name == 'checkpoint.json':
                saved.append(copy.deepcopy(value))
        class Stage:
            final_path = Path('/fixture-store')
            def adopt_applied_target(self, *_args): pass
            def verify_final_storage(self): pass
            def verified(self): pass
            def refence(self): return {'verified': True}
            def restart(self):
                kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 1
                self_test.assertTrue(callable(getattr(self, 'gis_ready_proof', None)),
                    'finish must install the production ready proof before restart')
                try:
                    self.gis_ready_proof(self)
                except gis.Blocked:
                    self_test.assertTrue(saved and 'gis_ready_proof' in saved[-1],
                        'ready proof must be checkpointed before evidence rejection')
                    self_test.assertEqual(saved[-1]['gis_ready_proof'], observed[-1])
                    self_test.assertFalse(any(row.get('status') in ('PASS', 'PROVISIONAL_READY') for row in saved))
                    raise
                raise BeforeRelease()
        self_test = self
        stage = Stage()
        stage.kube = kube
        with tempfile.TemporaryDirectory() as tmp, \
                patch.object(transaction, 'reconstruct', return_value=stage), \
                patch.object(transaction, 'revalidate_inputs'), \
                patch.object(transaction, 'context', return_value={}), \
                patch.object(transaction, 'DockerRuntime'), \
                patch.object(transaction, 'verify_post_apply', return_value={'verified': True}), \
                patch.object(transaction, 'attach_closed_preservation'), \
                patch.object(transaction, 'save', side_effect=save), \
                patch.object(existing, 'verify_ready', side_effect=record_ready) as ready:
            with self.assertRaises(gis.Blocked):
                transaction.finish(kube, Path(tmp), state, receipt, 'claim-rv', {})
            ready.assert_called_once_with(binding, kube, account)
            self.assertEqual(saved[-1]['gis_ready_proof'], observed[-1])
            self.assertFalse(any(row.get('status') in ('PASS', 'PROVISIONAL_READY') for row in saved))

    def test_missing_successor_evidence_blocks_release_and_pass(self):
        class ReachedRelease(Exception):
            pass
        kube, binding, account = self.ready_fixture()
        kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 0
        receipt = {'operation': '123456abcdef', 'target': {'mode': 'images-only'},
            'expires_at': (dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5)).isoformat()}
        state = {'operation': receipt['operation'], 'phase': 'PAUSED_APPLY', 'status': 'WAITING',
            'target': receipt['target'], 'authorization': receipt, 'gis_binding': binding,
            'gis_service_account': account, 'events': [], 'provenance': {}, 'cut': {'manifest': {}}}
        saved = []
        def save(path, value):
            if Path(path).name == 'checkpoint.json': saved.append(copy.deepcopy(value))
        release = Mock(side_effect=ReachedRelease)
        publication = Mock()
        class Stage:
            final_path = Path('/fixture-store')
            def adopt_applied_target(self, *_args): pass
            def verify_final_storage(self): pass
            def verified(self): pass
            def refence(self): return {'verified': True}
            def restart(self):
                kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 1
                self.gis_ready_proof(self)
                release()
        stage = Stage()
        stage.kube = kube
        with tempfile.TemporaryDirectory() as tmp, \
                patch.object(transaction, 'reconstruct', return_value=stage), \
                patch.object(transaction, 'revalidate_inputs'), \
                patch.object(transaction, 'context', return_value={}), \
                patch.object(transaction, 'DockerRuntime'), \
                patch.object(transaction, 'verify_post_apply', return_value={'verified': True}), \
                patch.object(transaction, 'attach_closed_preservation'), \
                patch.object(transaction, 'save', side_effect=save), \
                patch.object(existing, 'verify_ready', wraps=existing.verify_ready) as ready:
            with self.assertRaises((gis.Blocked, runtime_stage.Blocked)):
                transaction.finish(kube, Path(tmp), state, receipt, 'claim-rv', {}, publication=publication)
            ready.assert_called_once_with(binding, kube, account)
            release.assert_not_called()
            publication.assert_not_called()
            self.assertFalse(any(row.get('status') in ('PASS', 'PROVISIONAL_READY') for row in saved))

    def test_finish_only_with_existing_gis_is_blocked_before_reconstruction(self):
        class ReachedReconstruction(Exception):
            pass
        receipt = {'operation': '123456abcdef', 'target': {'mode': 'images-only'}}
        state = {'operation': receipt['operation'], 'target': receipt['target'],
            'authorization': receipt, 'gis_binding': self.binding, 'events': []}
        for entry in ('direct', 'finish_dispatch'):
            with self.subTest(entry=entry), tempfile.TemporaryDirectory() as tmp, \
                    patch.object(transaction, 'reconstruct', side_effect=ReachedReconstruction) as reconstruct, \
                    patch.object(transaction, 'save') as save:
                publication = Mock()
                with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), 'gis_existing_finish_only_unsupported'):
                    if entry == 'direct':
                        transaction.finish_only(self.kube, Path(tmp), copy.deepcopy(state),
                            receipt, {}, {}, publication=publication)
                    else:
                        transaction.finish(self.kube, Path(tmp), copy.deepcopy(state),
                            receipt, 'rv', {}, finish_authority={}, publication=publication)
                reconstruct.assert_not_called()
                save.assert_not_called()
                publication.assert_not_called()

    def test_paused_rollback_rejects_foreign_pair_before_adoption(self):
        class ReachedAdoption(Exception):
            pass
        target = copy.deepcopy(self.old)
        target['spec']['template']['spec']['containers'][0]['image'] = self.new_image
        binding = existing.bind_target(self.binding, target)
        for fault in ('third_image', 'deployment_uid', 'service_drift'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                kube = copy.deepcopy(self.kube)
                kube.deployments['voice-gameintegration'] = copy.deepcopy(target)
                kube.deployments['voice-gameintegration']['spec']['replicas'] = 0
                if fault == 'third_image':
                    kube.deployments['voice-gameintegration']['spec']['template']['spec']['containers'][0]['image'] = (
                        'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'f' * 64)
                elif fault == 'deployment_uid':
                    kube.deployments['voice-gameintegration']['metadata']['uid'] = '00000000-0000-0000-0000-000000000999'
                else:
                    kube.gis_kube.objects[gis.SERVICE]['spec']['selector']['app'] = 'foreign'
                receipt = {'operation': '123456abcdef', 'target': {'mode': 'images-only'},
                    'expires_at': (dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5)).isoformat()}
                state = {'operation': receipt['operation'], 'phase': 'PAUSED_APPLY', 'status': 'WAITING',
                    'target': receipt['target'], 'authorization': receipt, 'gis_binding': binding,
                    'events': [], 'provenance': {}, 'fence_status': 'VERIFIED'}
                stage = Mock()
                stage.kube = kube
                stage.finish_released_marker = None
                stage.adopt_rollback_target.side_effect = ReachedAdoption
                with patch.object(transaction, 'reconstruct', return_value=stage), \
                        patch.object(transaction, 'revalidate_inputs'), \
                        patch.object(transaction, 'context', return_value={}), \
                        patch.object(transaction, 'save'):
                    with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), 'gis_existing_rollback_pair_changed'):
                        transaction.rollback(kube, Path(tmp), state, receipt, 'claim-rv', {})
                    stage.adopt_rollback_target.assert_not_called()
                    stage.restore_original_templates.assert_not_called()
                    stage.restart.assert_not_called()

    def test_root_authority_rejects_dangling_or_nonpass_successor(self):
        _, first, receipt = fixtures.first_install()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            installed = root / 'installed'
            installed.mkdir()
            original = root / ('rollout-' + first['operation'])
            original.mkdir()
            (original / 'checkpoint.json').write_text(json.dumps(first), encoding='utf-8')
            active_path = installed / 'active-release.json'
            active = {'gis_first_install_transition': {'operation': first['operation'],
                'receipt_sha256': receipt['receipt_sha256']}}
            def read(path, **kwargs):
                return json.loads(Path(path).read_text(encoding='utf-8'))
            active_path.write_text(json.dumps(active), encoding='utf-8')
            with patch.object(guard, 'ROOT', root), patch.object(existing, 'private_json', side_effect=read):
                self.assertEqual(existing.load_authority(self.kube), self.binding)
                active['gis_latest_transition'] = {'operation': 'b' * 12, 'receipt_sha256': 'c' * 64}
                active_path.write_text(json.dumps(active), encoding='utf-8')
                for status in ('missing', 'WAITING', 'ROLLED_BACK', 'PASS'):
                    with self.subTest(status=status):
                        successor = root / ('rollout-' + 'b' * 12)
                        if status != 'missing':
                            successor.mkdir(exist_ok=True)
                            (successor / 'checkpoint.json').write_text(json.dumps({
                                'operation': 'b' * 12, 'status': status}), encoding='utf-8')
                        before = active_path.read_bytes()
                        with self.assertRaises(gis.Blocked):
                            existing.load_authority(self.kube)
                        self.assertEqual(active_path.read_bytes(), before)

    def test_unsupported_recovery_paths_reject_gis_before_external_effects(self):
        class ReachedBoundary(Exception):
            pass
        state = {'operation': paused_recovery.OPERATION, 'gis_binding': self.binding}
        request = {'operation': paused_recovery.OPERATION, 'nonce': 'a' * 64,
            'token': 'fixture', 'run_id': 1}
        actions = object.__new__(bridge_root.Actions)
        for method in ('_expired_source_admission', '_finish_source_admission'):
            with self.subTest(method=method), tempfile.TemporaryDirectory() as tmp, \
                    patch.object(actions, 'state', return_value=(Path(tmp), copy.deepcopy(state))), \
                    patch.object(actions, '_dispatcher', side_effect=ReachedBoundary) as dispatcher, \
                    patch.object(bridge_root.source_authority, 'capture_source') as source:
                with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), 'gis_existing_recovery_unsupported'):
                    getattr(actions, method)(request)
                dispatcher.assert_not_called()
                source.assert_not_called()
        with tempfile.TemporaryDirectory() as tmp, \
                patch.object(expired_recovery, 'runtime_authority', side_effect=ReachedBoundary) as authority, \
                patch.object(transaction, 'reconstruct') as reconstruct, \
                patch.object(transaction, 'save') as save:
            with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), 'gis_existing_recovery_unsupported'):
                transaction.authorize(self.kube, Path(tmp), copy.deepcopy(state), '6' * 64, '7' * 64, {},
                    expired_authority={'fixture': True},
                    authorization_deadline=dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=1))
            authority.assert_not_called()
            reconstruct.assert_not_called()
            save.assert_not_called()
        receipt = {'operation': paused_recovery.OPERATION, 'target': {'mode': 'images-only'},
            'expires_at': (dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5)).isoformat()}
        finish_state = {**state, 'phase': 'PAUSED_APPLY', 'status': 'WAITING',
            'target': receipt['target'], 'authorization': receipt,
            'expired_recovery_authority': {'fixture': True}, 'events': []}
        with tempfile.TemporaryDirectory() as tmp, \
                patch.object(transaction, 'reconstruct', side_effect=ReachedBoundary) as reconstruct, \
                patch.object(transaction, 'save') as save:
            publication = Mock()
            with self.assertRaisesRegex((gis.Blocked, runtime_stage.Blocked), 'gis_existing_recovery_unsupported'):
                transaction.finish(self.kube, Path(tmp), finish_state, receipt, 'rv', {},
                    publication=publication)
            reconstruct.assert_not_called()
            save.assert_not_called()
            publication.assert_not_called()

    def test_successor_evidence_uses_owned_cold_intent_and_exact_custody(self):
        # Real ColdRelease constructs and journals the intent. Archive/content
        # I/O is simulated: this fixture does not prove off-node custody.
        from types import SimpleNamespace
        import root_cli
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp) / 'rollout-123456abcdef'
            base.mkdir()
            census = {'fixture': 'cold-state'}
            cut = {'manifest': {'archive_sha256': '6' * 64}, 'census': census,
                'census_sha256': gis.digest(census)}
            raw = b'synthetic native manifest'
            (base / 'rollout-before-manifest.json').write_bytes(raw)
            cut['manifest_sha256'] = hashlib.sha256(raw).hexdigest()
            pair = {'fixture': 'normalized-pair'}
            target = {'tag': 'a' * 40, 'mode': 'images-only',
                'images': {'voice-gameintegration/gameintegration': self.new_image},
                'gis_existing_normalized_sha256': gis.digest(pair), 'manifest_sha256': '9' * 64}
            backup = {'archive_sha256': cut['manifest']['archive_sha256'],
                'manifest_sha256': cut['manifest_sha256'], 'census_sha256': cut['census_sha256'],
                'off_node_verified': True}
            binding = {'operation': '123456abcdef', 'challenge': 'b' * 32, 'run_id': 17,
                'head_sha': 'a' * 40, 'cipher_sha256': 'c' * 64, 'cipher_bytes': 100,
                'created_at': '2026-10-28T00:00:00+00:00'}
            custody = {'schema': 'voice-nats-custody-v1', 'destination': 'github:Poryadok/VoiceRoot',
                'verified': True, 'artifact_id': 19,
                **{key: value for key, value in binding.items() if key != 'created_at'}}
            restore = {'schema': 'voice-nats-decrypted-restore-v1', 'cipher_sha256': binding['cipher_sha256'],
                'cipher_bytes': binding['cipher_bytes'], 'decrypted_members_verified': True,
                'readback': {'native_archive_verified': True, 'census_sha256': cut['census_sha256']},
                'members': {'rollout-before.tar': backup['archive_sha256'],
                    'rollout-before-manifest.json': backup['manifest_sha256']}}
            restore_path = base / 'decrypted-restore.json'
            restore_path.write_text(json.dumps(restore), encoding='utf-8')
            restore_path.chmod(0o600)
            proof = {'archive': 'synthetic.tar', 'archive_sha256': 'd' * 64,
                'native_files_verified': True, 'census_verified': True, 'census_sha256': cut['census_sha256']}
            helper = {'nats-rollout-preservation/resource_claim.py': 'e' * 64}
            context = resource_claim_test.context()
            context.update(commit=target['tag'], images=copy.deepcopy(target['images']))
            fence = {'schema': 'voice-staging-resource-fence-v1', 'context': context,
                'operation': binding['operation'], 'helper_sha256': gis.digest(helper),
                'target_sha256': gis.digest(target)}
            authorization = {'operation': binding['operation'], 'target': target, 'backup': backup,
                'namespace_uid': self.binding['namespace_uid']}
            nonce = 'f' * 64
            state = {'schema': 'nats-rollout-root-v1', 'operation': binding['operation'], 'cut': cut, 'target': target,
                'authorization': authorization, 'cipher_binding': binding, 'custody': custody,
                'gis_binding': copy.deepcopy(self.binding),
                'context': {'expected': {'namespace_uid': authorization['namespace_uid'],
                    'source_claim_uid': '00000000-0000-0000-0000-000000000012'}},
                'preservation': proof, 'resource_fence': fence, 'code_capture': helper,
                'gis_normalized_pair': pair, 'events': [], 'closed_preservation_attempt_nonce': nonce,
                'closed_preservation_attempts': {nonce: {'nonce': nonce,
                    'authorization_sha256': gis.digest(authorization), 'admitted_cut_sha256': gis.digest(cut)}}}
            stage = SimpleNamespace(final_path=base, verify_final_storage=lambda: None)
            stage.save = lambda event: state['events'].append(copy.deepcopy(event))
            runtime = preserve.ColdRelease(base, state, stage, lambda: None, cut, nonce,
                dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5))
            stage.closed_preservation_runtime = runtime
            resource_claim.historical_binding(state)
            with patch.object(preserve, 'DockerRuntime', create=True), \
                    patch('docker_runtime.DockerRuntime'), \
                    patch.object(preserve, 'verify_post_apply', return_value=proof), \
                    patch.object(preserve, 'closed_native_states', return_value={'fixture': 'same'}), \
                    patch('native_store.verify_archive', return_value=True):
                intent = runtime.verify_and_journal(stage)
                runtime.arm_release(stage, intent)
                evidence = existing.successor_evidence(base, state, stage)
                self.assertEqual(evidence['cold_intent'], intent)
                self.assertEqual(evidence['custody'], custody)
                self.assertEqual(evidence['restore'], restore)
                for fault in ('custody', 'restore', 'target', 'normalized', 'intent', 'fence_context'):
                    with self.subTest(fault=fault):
                        original = copy.deepcopy(state)
                        original_restore = restore_path.read_bytes()
                        if fault == 'custody':
                            state['custody']['run_id'] += 1
                        elif fault == 'restore':
                            changed = copy.deepcopy(restore)
                            changed['cipher_sha256'] = '0' * 64
                            restore_path.write_text(json.dumps(changed), encoding='utf-8')
                        elif fault == 'target':
                            state['resource_fence']['target_sha256'] = '0' * 64
                        elif fault == 'normalized':
                            state['gis_normalized_pair']['fixture'] = 'changed'
                        elif fault == 'intent':
                            state['events'].clear()
                        else:
                            state['resource_fence']['context']['commit'] = 'f' * 40
                        with self.assertRaises((gis.Blocked, runtime_stage.Blocked, RuntimeError)):
                            existing.successor_evidence(base, state, stage)
                        state.clear()
                        state.update(original)
                        restore_path.write_bytes(original_restore)
                with self.assertRaises((gis.Blocked, runtime_stage.Blocked, RuntimeError)):
                    existing.successor_evidence(base, copy.deepcopy(state), stage)
                with self.assertRaises((gis.Blocked, runtime_stage.Blocked, RuntimeError)):
                    existing.successor_evidence(base / 'other', state, stage)
                hook = stage.closed_preservation_release
                stage.closed_preservation_release = lambda *_args: None
                with self.assertRaises((gis.Blocked, runtime_stage.Blocked, RuntimeError)):
                    existing.successor_evidence(base, state, stage)
                stage.closed_preservation_release = hook
                prior_cut = copy.deepcopy(runtime.cut)
                runtime.cut['census_sha256'] = '0' * 64
                with self.assertRaises((gis.Blocked, runtime_stage.Blocked, RuntimeError)):
                    existing.successor_evidence(base, state, stage)
                runtime.cut = prior_cut
                state['code_capture']['nats-rollout-preservation/resource_claim.py'] = '0' * 64
                with self.assertRaises((gis.Blocked, runtime_stage.Blocked, RuntimeError)):
                    existing.successor_evidence(base, state, stage)
                state['code_capture'] = copy.deepcopy(helper)

                # Genesis uses real first-install receipt, live-ready fixture
                # and an actual cold intent; only native archive I/O is mocked.
                first_kube, first_state, first_receipt = fixtures.first_install()
                ready_kube, ready_binding, account = self.ready_fixture()
                deployment = copy.deepcopy(ready_kube.objects[gis.DEPLOYMENT])
                service = copy.deepcopy(ready_kube.objects[gis.SERVICE])
                normalized_deployment = gis.semantic(deployment)
                normalized_deployment['spec']['replicas'] = 0
                normalized_pair = {'deployment_mode': 'selected-paused',
                    'deployment_uid': ready_binding['deployment_uid'], 'service_uid': ready_binding['service_uid'],
                    'deployment': normalized_deployment, 'service': gis.semantic(service)}
                state['gis_normalized_pair'] = normalized_pair
                state['target']['gis_existing_normalized_sha256'] = gis.digest(normalized_pair)
                state['target']['template_hashes'] = {'voice-gameintegration':
                    gis.digest(normalized_deployment['spec']['template'])}
                state['resource_fence']['target_sha256'] = gis.digest(state['target'])
                state['authorization']['target'] = copy.deepcopy(state['target'])
                state['closed_preservation_attempts'][nonce]['authorization_sha256'] = gis.digest(state['authorization'])
                state['gis_binding'] = ready_binding
                state['gis_source_authority'] = self.source_fixture()
                state['gis_service_account'] = account
                state['gis_ready_proof'] = existing.verify_ready(ready_binding, ready_kube, account)
                state['authorization']['expires_at'] = (dt.datetime.now(dt.timezone.utc)
                    + dt.timedelta(minutes=5)).isoformat()
                state['closed_preservation_attempts'][nonce]['authorization_sha256'] = gis.digest(state['authorization'])
                state.update(phase='PAUSED_APPLY', status='WAITING', fence_status='VERIFIED', provenance={})
                anchor = {'operation': first_state['operation'], 'receipt_sha256': first_receipt['receipt_sha256']}
                active = {'operation': first_state['operation'],
                    'namespace_uid': first_receipt['namespace_uid'],
                    'pvc_uid': '00000000-0000-0000-0000-000000000012',
                    'target_sha256': '9' * 64, 'gis_first_install_transition': anchor}
                state['gis_expected_active'] = copy.deepcopy(active)
                root = Path(tmp)
                original_dir = root / ('rollout-' + first_state['operation'])
                original_dir.mkdir()
                checkpoint = original_dir / 'checkpoint.json'
                checkpoint.write_text(json.dumps(first_state), encoding='utf-8')
                checkpoint.chmod(0o600)
                installed = root / 'installed'
                installed.mkdir()
                active_path = installed / 'active-release.json'
                active_path.write_text(json.dumps(active), encoding='utf-8')
                active_path.chmod(0o600)
                stage.kube = ready_kube
                state['events'] = []
                runtime = preserve.ColdRelease(base, state, stage, lambda: None, cut, nonce,
                    dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5))
                stage.closed_preservation_runtime = runtime
                intent = runtime.verify_and_journal(stage)
                runtime.arm_release(stage, intent)
                snapshots = []
                durable = base / 'checkpoint.json'
                state['gis_ready_proof'] = None
                ready_kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 0
                stage.snapshots = {'voice-gameintegration': {}}
                stage.expected = {'source_claim': 'fixture-claim'}
                stage.owned_marker = lambda: {'data': {'phase': 'rollout-verified'}}
                stage.scale = lambda _name, replicas: ready_kube.objects[gis.DEPLOYMENT]['spec'].update(replicas=replicas)
                stage.wait_ready = lambda _name: None
                stage.verify_selected_storage_identity = lambda: None
                stage.adopt_applied_target = lambda *_args: None
                stage.verified = lambda: None
                stage.restart = lambda: runtime_stage.RolloutStage.release_marker(stage)
                stage.refence = lambda: {'verified': False}
                failure = {'point': None, 'marker_applied': False}
                def marker_cas(*_args):
                    checkpoint_at_cas = json.loads(durable.read_text(encoding='utf-8'))
                    self.assertIn('receipt', checkpoint_at_cas['gis_existing_update'])
                    self.assertEqual(checkpoint_at_cas['gis_ready_proof'],
                        existing.verify_ready(ready_binding, ready_kube, account))
                    self.assertNotEqual(checkpoint_at_cas['status'], 'PASS')
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), active)
                    if failure['point'] == 'cas-before':
                        raise RuntimeError('injected CAS before effect')
                    failure['marker_applied'] = True
                    if failure['point'] == 'cas-response':
                        raise RuntimeError('injected lost CAS response')
                    return {'data': {'phase': 'active'}}
                ready_kube.cas = Mock(side_effect=marker_cas)
                def persist(_path, row):
                    if failure['point'] == 'save-pass' and row['status'] == 'PASS':
                        failure['point'] = None
                        raise RuntimeError('injected PASS save failure')
                    snapshots.append(copy.deepcopy(row))
                    durable.write_text(json.dumps(row), encoding='utf-8')
                    durable.chmod(0o600)
                with patch.object(guard, 'ROOT', root), \
                        patch.object(transaction, 'save', side_effect=persist), \
                        patch.object(transaction, 'reconstruct', return_value=stage), \
                        patch.object(transaction, 'context', side_effect=lambda _stage: copy.deepcopy(state['context'])), \
                        patch.object(transaction, 'revalidate_inputs'), \
                        patch.object(transaction, 'DockerRuntime'), \
                        patch.object(transaction, 'verify_post_apply', return_value=proof), \
                        patch.object(transaction, 'attach_closed_preservation'):
                    baseline_state = copy.deepcopy(state)
                    for fault in ('cas-before', 'cas-response', 'save-pass'):
                        with self.subTest(fault=fault):
                            failure.update(point=fault, marker_applied=False)
                            first_snapshot = len(snapshots)
                            with self.assertRaisesRegex(RuntimeError, 'injected'):
                                transaction.finish(ready_kube, base, state, state['authorization'], 'fixture-rv', {})
                            saved = json.loads(durable.read_text(encoding='utf-8'))
                            self.assertEqual(saved['status'], 'BLOCKED')
                            self.assertEqual(saved['fence_status'], 'UNKNOWN')
                            self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), active)
                            self.assertEqual(failure['marker_applied'], fault != 'cas-before')
                            self.assertTrue(all(row['status'] != 'PASS' for row in snapshots[first_snapshot:]))
                            ready_kube.cas.assert_called_once()
                            state.clear()
                            state.update(copy.deepcopy(baseline_state))
                            ready_kube.objects[gis.DEPLOYMENT]['spec']['replicas'] = 0
                            if hasattr(stage, 'marker'):
                                del stage.marker
                            ready_kube.cas.reset_mock()
                    failure.update(point=None, marker_applied=False)
                    transaction.finish(ready_kube, base, state, state['authorization'], 'fixture-rv', {})
                    receipt = state['gis_existing_update']['receipt']
                    self.assertEqual(receipt['anchor'], anchor)
                    self.assertEqual(receipt['predecessor'], anchor)
                    self.assertEqual(receipt['old_pair_sha256'], gis.digest({
                        'deployment_uid': ready_binding['deployment_uid'], 'service_uid': ready_binding['service_uid'],
                        'deployment': ready_binding['old'], 'service': ready_binding['service']}))
                    self.assertEqual(receipt['ready_pair_sha256'], gis.digest({
                        'deployment_uid': ready_binding['deployment_uid'], 'service_uid': ready_binding['service_uid'],
                        'deployment': gis.semantic(deployment), 'service': gis.semantic(service)}))
                    self.assertEqual(receipt['receipt_sha256'], gis.digest({
                        key: value for key, value in receipt.items() if key != 'receipt_sha256'}))
                    self.assertEqual(snapshots[-1]['gis_existing_update']['receipt'], receipt)
                    self.assertEqual(snapshots[-1]['status'], 'PASS')
                    self.assertEqual(snapshots[-1]['phase'], 'POST_RESTART')
                    self.assertEqual(snapshots[-1]['fence_status'], 'RELEASED')
                    ready_kube.cas.assert_called_once()
                # Finish executed with controlled external Kube/NATS/storage seams.
                self.assertEqual(json.loads(durable.read_text(encoding='utf-8')), state)
                actions = object.__new__(bridge_root.Actions)
                actions._installation = resource_claim.production_installation()
                actions.binding = copy.deepcopy(helper)
                with patch.object(guard, 'ROOT', root), patch.object(bridge_root, 'INSTALLED', installed), \
                        patch.object(root_cli, 'ROOT', root), patch.object(bridge_root, 'Kube', return_value=ready_kube):
                    # TT transport and journal storage are simulated, but the
                    # Session and root_scope state transitions are production code.
                    tt_context = state['resource_fence']['context']
                    journal = resource_claim_test.MemoryJournal()
                    journal.save(dict(state['resource_fence'], status='active', phase='apply-ready'))
                    check = Mock(return_value=resource_claim_test.card(tt_context))
                    request = {'action': 'finish', 'operation': state['operation'], 'resource_context': tt_context}
                    with patch.object(resource_claim, 'require_helper'), \
                            patch.object(resource_claim, 'RootJournal', return_value=journal), \
                            patch.object(resource_claim, 'Check', return_value=check):
                        with self.assertRaisesRegex(OSError, 'publication unavailable'):
                            with resource_claim.root_scope(actions, request) as session:
                                session.admit(start=False)
                                with patch.object(bridge_root, 'save', side_effect=OSError('publication unavailable')):
                                    actions._active_release(state)
                        self.assertEqual(journal.read()['status'], 'uncertain')
                        with self.assertRaisesRegex(RuntimeError, 'staging_unfenced_or_uncertain_operation'):
                            with resource_claim.root_scope(actions, request) as session:
                                session.admit(start=False)
                        check.uncertain.assert_called_once_with('staging_execution_interrupted_or_partial')
                    self.assertEqual(json.loads(durable.read_text(encoding='utf-8')), state)
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), active)
                    # Handler idempotence only: real root_scope may mark TT uncertain,
                    # in which case a later admission must not be assumed possible.
                    actions.state(state['operation'])
                    result = actions.recover({'action': 'finish', 'operation': state['operation']})
                    self.assertEqual(result['status'], 'PASS')
                    published = json.loads(active_path.read_text(encoding='utf-8'))
                    self.assertEqual(published['gis_first_install_transition'], anchor)
                    self.assertEqual(published['gis_latest_transition'], {
                        'operation': state['operation'], 'receipt_sha256': receipt['receipt_sha256']})
                    with patch.object(bridge_root, 'save') as replay_save:
                        self.assertEqual(actions.recover({'action': 'finish', 'operation': state['operation']})['status'], 'PASS')
                        replay_save.assert_not_called()
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), published)
                    loaded = existing.load_authority(ready_kube)
                    self.assertEqual(loaded['anchor_sha256'], anchor['receipt_sha256'])
                    self.assertEqual(loaded['old'], gis.semantic(deployment))
                    # Re-seal target, authority and TT records consistently,
                    # but retain the original normalized/bound GIS template.
                    forged = copy.deepcopy(state)
                    changed_image = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'e' * 64
                    forged['target']['images']['voice-gameintegration/gameintegration'] = changed_image
                    forged['gis_source_authority']['images']['gameintegration'] = changed_image
                    forged['authorization']['target'] = copy.deepcopy(forged['target'])
                    forged['closed_preservation_attempts'][nonce]['authorization_sha256'] = gis.digest(forged['authorization'])
                    forged['resource_fence']['context']['images'] = copy.deepcopy(forged['target']['images'])
                    forged['resource_fence']['target_sha256'] = gis.digest(forged['target'])
                    section = forged['gis_existing_update']
                    section['source_authority'] = copy.deepcopy(forged['gis_source_authority'])
                    section['resource_fence'] = copy.deepcopy(forged['resource_fence'])
                    forged_receipt = section['receipt']
                    forged_receipt['source_authority_sha256'] = gis.digest(section['source_authority'])
                    forged_receipt['resource_fence_sha256'] = gis.digest(section['resource_fence'])
                    forged_receipt['target_sha256'] = gis.digest(forged['target'])
                    forged_receipt['receipt_sha256'] = gis.digest({
                        key: value for key, value in forged_receipt.items() if key != 'receipt_sha256'})
                    durable.write_text(json.dumps(forged), encoding='utf-8')
                    changed_active = copy.deepcopy(published)
                    changed_active['gis_latest_transition']['receipt_sha256'] = forged_receipt['receipt_sha256']
                    active_path.write_text(json.dumps(changed_active), encoding='utf-8')
                    with self.assertRaisesRegex(gis.Blocked, 'gis_existing_successor_binding_changed'):
                        existing.load_authority(ready_kube)
                    durable.write_text(json.dumps(state), encoding='utf-8')
                    active_path.write_text(json.dumps(published), encoding='utf-8')
                    legacy = {'operation': 'c' * 12, 'status': 'PASS', 'context': state['context'],
                        'target': {'mode': 'images-only', 'manifest_sha256': '8' * 64}}
                    legacy_dir = root / ('rollout-' + legacy['operation'])
                    legacy_dir.mkdir()
                    legacy_checkpoint = legacy_dir / 'checkpoint.json'
                    legacy_checkpoint.write_text(json.dumps(legacy), encoding='utf-8')
                    legacy_checkpoint.chmod(0o600)
                    try:
                        actions._active_release(legacy)
                    except (gis.Blocked, runtime_stage.Blocked):
                        pass
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), published)

                    # B is independently produced and journaled against A's
                    # published root reference. Completion is still simulated.
                    second = copy.deepcopy(state)
                    second.pop('gis_existing_update')
                    second['operation'] = 'b' * 12
                    second['status'] = 'WAITING'
                    second['phase'] = 'RESTART'
                    second['fence_status'] = 'VERIFIED'
                    second['gis_expected_active'] = copy.deepcopy(published)
                    second_image = 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'c' * 64
                    next_deployment = copy.deepcopy(ready_kube.objects[gis.DEPLOYMENT])
                    next_deployment['spec']['template']['spec']['containers'][0]['image'] = second_image
                    next_binding = existing.bind_target(loaded, next_deployment)
                    ready_kube.objects[gis.DEPLOYMENT] = copy.deepcopy(next_deployment)
                    ready_kube.rs['spec']['template'] = copy.deepcopy(next_deployment['spec']['template'])
                    ready_kube.pod['spec']['containers'][0]['image'] = second_image
                    ready_kube.pod['status']['containerStatuses'][0]['imageID'] = second_image
                    next_pair = {'deployment_mode': 'selected-paused',
                        'deployment_uid': next_binding['deployment_uid'], 'service_uid': next_binding['service_uid'],
                        'deployment': gis.semantic(next_deployment), 'service': gis.semantic(service)}
                    next_pair['deployment']['spec']['replicas'] = 0
                    second['gis_binding'] = next_binding
                    second['gis_normalized_pair'] = next_pair
                    second['gis_ready_proof'] = existing.verify_ready(next_binding, ready_kube, account)
                    second['target']['images']['voice-gameintegration/gameintegration'] = second_image
                    second['target']['manifest_sha256'] = '8' * 64
                    second['target']['gis_existing_normalized_sha256'] = gis.digest(next_pair)
                    second['target']['template_hashes']['voice-gameintegration'] = gis.digest(next_pair['deployment']['spec']['template'])
                    second['gis_source_authority']['images']['gameintegration'] = second_image
                    second['resource_fence']['operation'] = second['operation']
                    second['resource_fence']['context']['images'] = copy.deepcopy(second['target']['images'])
                    second['resource_fence']['target_sha256'] = gis.digest(second['target'])
                    second['authorization']['operation'] = second['operation']
                    second['authorization']['target'] = copy.deepcopy(second['target'])
                    second['cipher_binding']['operation'] = second['operation']
                    second['custody']['operation'] = second['operation']
                    second['cipher_binding']['challenge'] = '1' * 32
                    second['custody']['challenge'] = second['cipher_binding']['challenge']
                    second['cipher_binding']['cipher_sha256'] = '2' * 64
                    second['custody']['cipher_sha256'] = second['cipher_binding']['cipher_sha256']
                    second['custody']['artifact_id'] = 20
                    second['events'] = []
                    second['closed_preservation_attempts'][nonce]['authorization_sha256'] = gis.digest(second['authorization'])
                    second_base = root / ('rollout-' + second['operation'])
                    second_base.mkdir()
                    (second_base / 'rollout-before-manifest.json').write_bytes(raw)
                    second_restore = second_base / 'decrypted-restore.json'
                    restored_b = copy.deepcopy(restore)
                    restored_b['cipher_sha256'] = second['cipher_binding']['cipher_sha256']
                    second_restore.write_text(json.dumps(restored_b), encoding='utf-8')
                    second_restore.chmod(0o600)
                    second_stage = SimpleNamespace(final_path=second_base, kube=ready_kube,
                        verify_final_storage=lambda: None)
                    second_stage.save = lambda event: second['events'].append(copy.deepcopy(event))
                    second_runtime = preserve.ColdRelease(second_base, second, second_stage, lambda: None, cut, nonce,
                        dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5))
                    second_stage.closed_preservation_runtime = second_runtime
                    next_intent = second_runtime.verify_and_journal(second_stage)
                    second_runtime.arm_release(second_stage, next_intent)
                    with patch.object(transaction, 'save'):
                        next_receipt = transaction.gis_successor_checkpoint(second_base, second, second_stage)
                    self.assertEqual(next_receipt['predecessor'], published['gis_latest_transition'])
                    self.assertEqual(next_receipt['old_pair_sha256'], receipt['ready_pair_sha256'])
                    second['status'] = 'PASS'
                    second['phase'] = 'POST_RESTART'
                    second['fence_status'] = 'RELEASED'
                    second_checkpoint = second_base / 'checkpoint.json'
                    second_checkpoint.write_text(json.dumps(second), encoding='utf-8')
                    second_checkpoint.chmod(0o600)
                    actions._active_release(second)
                    published_b = json.loads(active_path.read_text(encoding='utf-8'))
                    self.assertEqual(published_b['gis_first_install_transition'], anchor)
                    self.assertEqual(published_b['gis_latest_transition']['receipt_sha256'], next_receipt['receipt_sha256'])
                    self.assertEqual(existing.load_authority(ready_kube)['old'], gis.semantic(next_deployment))
                    with patch.object(bridge_root, 'save', wraps=bridge_root.save) as publish_save:
                        actions._active_release(copy.deepcopy(second))
                        publish_save.assert_not_called()
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), published_b)
                    try:
                        actions._active_release(state)
                    except (gis.Blocked, runtime_stage.Blocked):
                        pass
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), published_b)
                    for fault in ('missing_receipt', 'nonpass', 'context_namespace', 'authorization_namespace'):
                        with self.subTest(chain_fault=fault):
                            damaged = copy.deepcopy(second)
                            if fault == 'missing_receipt':
                                damaged['gis_existing_update'].pop('receipt')
                            elif fault == 'nonpass':
                                damaged['status'] = 'ROLLED_BACK'
                            elif fault == 'context_namespace':
                                damaged['context']['expected']['namespace_uid'] = '00000000-0000-0000-0000-000000000099'
                            else:
                                damaged['authorization']['namespace_uid'] = '00000000-0000-0000-0000-000000000099'
                                damaged['closed_preservation_attempts'][nonce]['authorization_sha256'] = gis.digest(damaged['authorization'])
                            second_checkpoint.write_text(json.dumps(damaged), encoding='utf-8')
                            with self.assertRaises((gis.Blocked, runtime_stage.Blocked, RuntimeError)):
                                existing.load_authority(ready_kube)
                            self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), published_b)
                            second_checkpoint.write_text(json.dumps(second), encoding='utf-8')
                    altered_pair = copy.deepcopy(second)
                    altered_pair['gis_binding']['old']['spec']['template']['spec']['containers'][0]['image'] = (
                        'ghcr.io/poryadok/voiceroot/gameintegration@sha256:' + 'f' * 64)
                    altered_pair['gis_binding'] = existing.seal(altered_pair['gis_binding'])
                    altered_section = altered_pair['gis_existing_update']
                    altered_section['old_pair']['deployment'] = copy.deepcopy(altered_pair['gis_binding']['old'])
                    altered_receipt = altered_section['receipt']
                    altered_receipt['old_pair_sha256'] = gis.digest(altered_section['old_pair'])
                    altered_receipt['receipt_sha256'] = gis.digest({
                        key: value for key, value in altered_receipt.items() if key != 'receipt_sha256'})
                    second_checkpoint.write_text(json.dumps(altered_pair), encoding='utf-8')
                    altered_active = copy.deepcopy(published_b)
                    altered_active['gis_latest_transition']['receipt_sha256'] = altered_receipt['receipt_sha256']
                    active_path.write_text(json.dumps(altered_active), encoding='utf-8')
                    with self.assertRaisesRegex(gis.Blocked, 'gis_existing_successor_predecessor_changed'):
                        existing.load_authority(ready_kube)
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), altered_active)
                    second_checkpoint.write_text(json.dumps(second), encoding='utf-8')
                    active_path.write_text(json.dumps(published_b), encoding='utf-8')
                    # Forged C claims B was active but links directly to A.
                    # Re-seal its local evidence to isolate predecessor skip.
                    forged_c = copy.deepcopy(second)
                    forged_c['operation'] = 'e' * 12
                    forged_c['gis_expected_active'] = copy.deepcopy(published_b)
                    forged_c['authorization']['operation'] = forged_c['operation']
                    forged_c['cipher_binding']['operation'] = forged_c['operation']
                    forged_c['custody']['operation'] = forged_c['operation']
                    forged_c['resource_fence']['operation'] = forged_c['operation']
                    forged_c['closed_preservation_attempts'][nonce]['authorization_sha256'] = gis.digest(forged_c['authorization'])
                    section_c = forged_c['gis_existing_update']
                    section_c['custody'] = copy.deepcopy(forged_c['custody'])
                    section_c['resource_fence'] = copy.deepcopy(forged_c['resource_fence'])
                    evidence_c = section_c['preservation']
                    evidence_c['custody'] = copy.deepcopy(forged_c['custody'])
                    intent_c = evidence_c['cold_intent']
                    intent_c['operation'] = forged_c['operation']
                    intent_c['custody_sha256'] = gis.digest(forged_c['custody'])
                    forged_c['events'] = [copy.deepcopy(intent_c)]
                    receipt_c = section_c['receipt']
                    receipt_c['operation'] = forged_c['operation']
                    receipt_c['predecessor'] = copy.deepcopy(published['gis_latest_transition'])
                    receipt_c['custody_sha256'] = gis.digest(section_c['custody'])
                    receipt_c['resource_fence_sha256'] = gis.digest(section_c['resource_fence'])
                    receipt_c['preservation_sha256'] = gis.digest(evidence_c)
                    receipt_c['receipt_sha256'] = gis.digest({
                        key: value for key, value in receipt_c.items() if key != 'receipt_sha256'})
                    third_base = root / ('rollout-' + forged_c['operation'])
                    third_base.mkdir()
                    (third_base / 'rollout-before-manifest.json').write_bytes(raw)
                    third_restore = third_base / 'decrypted-restore.json'
                    third_restore.write_text(json.dumps(restored_b), encoding='utf-8')
                    third_restore.chmod(0o600)
                    third_checkpoint = third_base / 'checkpoint.json'
                    third_checkpoint.write_text(json.dumps(forged_c), encoding='utf-8')
                    third_checkpoint.chmod(0o600)
                    with self.assertRaisesRegex(runtime_stage.Blocked, 'gis_existing_publication_stale'):
                        actions._active_release(forged_c)
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), published_b)
                    active_c = copy.deepcopy(published_b)
                    active_c['operation'] = forged_c['operation']
                    active_c['gis_latest_transition'] = {'operation': forged_c['operation'],
                        'receipt_sha256': receipt_c['receipt_sha256']}
                    active_path.write_text(json.dumps(active_c), encoding='utf-8')
                    with self.assertRaisesRegex(gis.Blocked, 'gis_existing_successor_invalid'):
                        existing.load_authority(ready_kube)
                    self.assertEqual(json.loads(active_path.read_text(encoding='utf-8')), active_c)
                    active_path.write_text(json.dumps(published_b), encoding='utf-8')

    def test_bridge_qualifies_gis_before_real_workload_capture(self):
        class Captured(Exception):
            pass
        _, first, receipt = fixtures.first_install()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            installed = root / 'installed'
            (installed / 'sources').mkdir(parents=True)
            original = root / ('rollout-' + first['operation'])
            original.mkdir()
            (original / 'checkpoint.json').write_text(json.dumps(first), encoding='utf-8')
            (installed / 'active-release.json').write_text(json.dumps({
                'gis_first_install_transition': {'operation': first['operation'],
                    'receipt_sha256': receipt['receipt_sha256']}}), encoding='utf-8')
            def read(path, **kwargs):
                return json.loads(Path(path).read_text(encoding='utf-8'))
            actual_capture = runtime_stage.RolloutStage.capture
            captures = []
            def observe(*args, **kwargs):
                stage = actual_capture(*args, **kwargs)
                captures.append((copy.deepcopy(kwargs), stage))
                raise Captured()
            actions = object.__new__(bridge_root.Actions)
            actions.binding = {}
            request = {'nonce': 'a' * 64, 'token': 'fixture', 'mode': 'images-only',
                'changed_services': ['gameintegration']}
            execution = {'head_sha': 'a' * 40}
            with patch.object(guard, 'ROOT', root), patch.object(bridge_root, 'INSTALLED', installed), \
                    patch.object(existing, 'private_json', side_effect=read), \
                    patch.object(actions, '_idle'), patch.object(actions, '_dispatcher', return_value=(1, execution)), \
                    patch.object(bridge_root.source_authority, 'capture_source', return_value={}), \
                    patch.object(bridge_root, 'Kube', return_value=self.kube), \
                    patch.object(runtime_stage.RolloutStage, 'capture', side_effect=observe):
                with self.assertRaises(Captured):
                    actions._prepare(request)
                self.assertEqual(len(captures), 1)
                kwargs, stage = captures[0]
                self.assertEqual(kwargs.get('gis_binding'), self.binding)
                self.assertIn('voice-gameintegration', stage.snapshots)
                self.assertEqual(stage.old_images['voice-gameintegration/gameintegration'], self.old_image)


if __name__ == '__main__':
    unittest.main()
