"""Cross-stage GIS contract using actual capture, compiler and normalizer."""
import copy
import datetime as dt
import inspect
import json
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
import runtime_stage
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
            set(stage.snapshots), ())

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

    def test_unproved_wiring_finish_cannot_destroy_first_install_anchor(self):
        kube, state, receipt = fixtures.first_install()
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

    def test_full_update_records_source_selected_service_account(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode):
                self.check_transaction_captures_qualified_gis_and_normalizes_before_fence(mode, target_sa=True)

    def test_full_update_rejects_unsupported_target_service_account(self):
        self.kube.target_account['automountServiceAccountToken'] = False
        self.check_transaction_captures_qualified_gis_and_normalizes_before_fence(
            'full', target_sa=True, reject_target_sa=True)

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
