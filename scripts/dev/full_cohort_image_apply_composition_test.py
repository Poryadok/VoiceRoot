import copy
import sys
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

import frontend_runtime_executor
import frontend_runtime_adoption as adoption
import full_cohort_image_apply as cohort
import full_cohort_image_apply_test as fixture
import media_operation_journal as journal
import resource_fencing
import writer_cut
import writer_cut_pg_probe
import full_cohort_inventory_phase
import full_cohort_activation_plan
import full_cohort_package_phase


class Composition(unittest.TestCase):
    def setUp(self):
        fixture.FullCohortTest.setUp(self)

    def _activate(self, cut, targets, image_phase):
        """Exercise real staged cut with synthetic source phase evidence."""
        class Inventory:
            def __init__(inner):
                inner.fence, inner.client, inner.lock, inner.namespace_uid = cut.fence, cut.client, cut.lock, 'ns'
                inner.journal = journal.Journal(Path(self.temp.name) / 'fake-activation-inventory',
                    {'resource_context': self.context, 'namespace_uid': 'ns'})
                self.addCleanup(inner.journal.close)
                value = {'operation_id': self.context['operation_id'], 'synthetic': True}
                marker = {'status': 'done', 'nonce': '1' * 32, 'receipt_sha256': writer_cut.digest(value)}
                inner.journal.state['steps'][full_cohort_inventory_phase.STEP] = marker | {'receipt': value}
                inner.journal.save(); self.ledger.row['full_cohort_inventory'] = marker
            def verify_receipt(inner, token):
                return copy.deepcopy(inner.journal.state['steps'][full_cohort_inventory_phase.STEP]['receipt'])
        zero = {'scope': cut.admission['session_scope'], 'active_sessions': 0, 'active_transactions': 0, 'attributed_sessions': []}
        inventory_patch = mock.patch.object(full_cohort_inventory_phase, 'InventoryPhase', Inventory)
        observer_patch = mock.patch.object(writer_cut_pg_probe, 'inspect_resumed', return_value=zero, create=True)
        inventory_patch.start(); observer_patch.start()
        self.addCleanup(inventory_patch.stop); self.addCleanup(observer_patch.stop)
        cut.bind_resume_inventory(Inventory(), 'token')
        class Package:
            def __init__(inner):
                inner.fence, inner.lock, inner.namespace_uid = cut.fence, cut.lock, 'ns'
            def verify_current(inner, token): return {'package_sha256': 'a' * 64}
        package_patch = mock.patch.object(full_cohort_package_phase, 'PackagePhase', Package)
        package_patch.start(); self.addCleanup(package_patch.stop)
        source_journal = journal.Journal(Path(self.temp.name) / 'actual-activation-plan',
            {'resource_context': self.context, 'namespace_uid': 'ns'})
        self.addCleanup(source_journal.close)
        source_client = copy.copy(cut.client); source_client.journal = source_journal
        backing = self.ledger
        class LedgerView:
            client = source_client
            def read(_): return backing.read()
            def write(_, revision, row): return backing.write(revision, row)
        source_fence = resource_fencing.Fence(self.context, lambda: copy.deepcopy(self.card), LedgerView())
        phase = full_cohort_activation_plan.ActivationPhase(source_journal, source_fence, source_client,
                                                            self.lock, 'ns', Package(), image_phase, cut)
        phase.run('token')
        cut.prepare_activation(phase, 'token')
        for barrier, module_name, class_name in (
                ('space_health', 'realtime_config_transition_space_health', 'SpaceHealthPhase'),
                ('publisher_health', 'realtime_jwks_publisher_health', 'PublisherHealthPhase'),
                ('chat_health', 'full_cohort_chat_health', 'ChatHealthPhase')):
            proof = {'barrier': barrier, 'operation_id': self.context['operation_id']}
            def factory(name, value):
                class Health:
                    def __init__(inner):
                        inner.fence, inner.client, inner.lock, inner.namespace_uid = cut.fence, cut.client, cut.lock, 'ns'
                        inner.journal = journal.Journal(Path(self.temp.name) / ('fake-' + name),
                            {'resource_context': self.context, 'namespace_uid': 'ns'})
                        self.addCleanup(inner.journal.close)
                        marker = {'status': 'done', 'receipt_sha256': writer_cut.digest(value)}
                        inner.journal.state['steps'][name] = marker | {'receipt': copy.deepcopy(value)}
                        inner.journal.save(); self.ledger.row[name] = marker
                return Health
            cls = factory(barrier, proof)
            source = SimpleNamespace(STEP=barrier, LEDGER=barrier,
                                     verify_completed=lambda phase, cut, token: copy.deepcopy(phase.journal.state['steps'][phase.key]['receipt']))
            setattr(source, class_name, cls)
            patch = mock.patch.dict(sys.modules, {module_name: source}); patch.start(); self.addCleanup(patch.stop)
            setattr(self, '_phase_' + barrier, cls())
            getattr(self, '_phase_' + barrier).key = barrier
        while True:
            stage = cut.verify_activation_stage()['stage']
            if stage == 'done': break
            if stage in ('space_health', 'publisher_health', 'chat_health'):
                cut.accept_activation_health(stage, getattr(self, '_phase_' + stage), 'token')
            else:
                cut.activate_next()
        return cut.finish_activation()

    def _flow(self, fault=None, config=False, frontend=False):
        self.cut_journal.close()
        admission = copy.deepcopy(self.cut_journal.intent['writer_cut'])
        ui_names = {name for name, _ in adoption.APPS.values()}
        if frontend:
            admission['controllers'] = [o for o in admission['controllers'] if o['metadata']['name'] not in ui_names]
        admission.update(guards={'jobs': [], 'cronjobs': [], 'hpas': []}, linked={'jobs': [], 'cronjobs': []})
        for source in admission['controllers']:
            source['apiVersion'] = 'apps/v1'
        cut_intent = {'resource_context': self.context, 'namespace_uid': 'ns', 'writer_cut': admission}
        if frontend:
            for name in ui_names:
                obj = self.client.deployments[name]
                obj['spec']['replicas'] = 1
                obj['status'] = {'observedGeneration': obj['metadata']['generation'], 'replicas': 1, 'readyReplicas': 1}
            keys = {name + '/' + container for name, container in adoption.APPS.values()}
            cut_intent['frontend_platform_digests'] = {k: self.intent['full_cohort']['platform_digests'][k] for k in keys}
            cut_intent['frontend_deployment_baseline'] = {app: {
                'uid': self.client.deployments[name]['metadata']['uid'],
                'generation': self.client.deployments[name]['metadata']['generation'],
                'spec': copy.deepcopy(self.client.deployments[name]['spec'])}
                for app, (name, _) in adoption.APPS.items()}
        cut_journal = journal.Journal(Path(self.temp.name) / 'real-cut', cut_intent)
        self.addCleanup(cut_journal.close)
        self.ledger.row.pop('writer_cut_invocation', None)
        self.ledger.row.pop('writer_cut_effects', None)
        for source in admission['controllers']:
            obj = self.client.deployments[source['metadata']['name']]
            obj['apiVersion'] = 'apps/v1'
            obj['spec'] = copy.deepcopy(source['spec'])
            obj['metadata'] = copy.deepcopy(source['metadata'])
            obj['status'] = {'observedGeneration': 1, 'replicas': 1, 'readyReplicas': 1}
        call = self.client.call
        configs = {}
        def kube_call(*args, **kwargs):
            if args[0] == 'create':
                obj = copy.deepcopy(kwargs['body'])
                obj['metadata'].update(uid=obj['metadata']['name'] + '-uid', resourceVersion='1')
                configs[obj['metadata']['name']] = obj
                return copy.deepcopy(obj)
            if args[1] == 'configmap' and args[2] != self.lock['metadata']['name']:
                return copy.deepcopy(configs.get(args[2], {}))
            if args[1] in ('jobs', 'cronjobs', 'horizontalpodautoscalers'):
                return {'items': []}
            return call(*args, **kwargs)
        self.client.call = kube_call
        cut_client = copy.copy(self.client)
        cut_client.journal = cut_journal
        backing = self.ledger
        class LedgerView:
            client = cut_client
            def read(_): return backing.read()
            def write(_, revision, row): return backing.write(revision, row)
        cut_fence = resource_fencing.Fence(self.context, lambda: copy.deepcopy(self.card), LedgerView())
        def ui_patch(name, operations):
            obj = self.client.deployments[name]
            self.assertEqual([operations[i]['value'] for i in range(3)],
                             [obj['metadata']['uid'], obj['metadata']['resourceVersion'], obj['spec']])
            obj['spec'] = copy.deepcopy(operations[3]['value'])
            obj['metadata']['generation'] += 1
            obj['metadata']['resourceVersion'] = str(int(obj['metadata']['resourceVersion']) + 1)
            obj['status'] = {'observedGeneration': obj['metadata']['generation'], 'updatedReplicas': 1, 'availableReplicas': 1}
            template = copy.deepcopy(obj['spec']['template'])
            template['metadata']['labels']['pod-template-hash'] = 'abc'
            rsuid = name + '-rs'
            self.client.replicasets.append({'metadata': {'uid': rsuid, 'ownerReferences': [
                {'controller': True, 'kind': 'Deployment', 'uid': obj['metadata']['uid']}]}, 'spec': {'template': template}})
            c = obj['spec']['template']['spec']['containers'][0]
            self.client.pods.append({'metadata': {'name': name + '-pod', 'namespace': 'voice-dev', 'uid': name + '-pod-uid',
                'resourceVersion': '34', 'ownerReferences': [{'controller': True, 'kind': 'ReplicaSet', 'uid': rsuid}]},
                'spec': copy.deepcopy(obj['spec']['template']['spec']), 'status': {'phase': 'Running',
                'conditions': [{'type': 'Ready', 'status': 'True'}], 'containerStatuses': [{'name': c['name'],
                'ready': True, 'imageID': 'containerd://sha256:' + 'c' * 64}]}})
            return copy.deepcopy(obj)
        cut_client.patch = ui_patch
        cut_client.read_pod = lambda name: configs[name.removesuffix('-pod') + '-runtime-config']['data'][adoption.KEY].encode()
        def cut_patch(kind, name, operations):
            obj = self.client.deployments[name]
            self.assertEqual((operations[0]['value'], operations[1]['value'], operations[2]['value']),
                             (obj['metadata']['uid'], obj['metadata']['resourceVersion'], obj['spec']))
            if operations[3]['path'] == '/spec':
                changed = obj['spec'] != operations[3]['value']
                obj['spec'] = copy.deepcopy(operations[3]['value'])
            else:
                changed = obj['spec']['replicas'] != operations[3]['value']
                obj['spec']['replicas'] = operations[3]['value']
            obj['metadata']['generation'] += changed
            obj['metadata']['resourceVersion'] = str(int(obj['metadata']['resourceVersion']) + 1)
            obj['status'] = {'observedGeneration': obj['metadata']['generation'], 'replicas': obj['spec']['replicas'],
                             'readyReplicas': obj['spec']['replicas']}
            return copy.deepcopy(obj)
        with mock.patch.object(frontend_runtime_executor, 'KubectlTransport', fixture.Cluster), \
             mock.patch.object(cohort, 'WriterCut', writer_cut.WriterCut):
            if frontend:
                phase = frontend_runtime_executor.FrontendExecutor(cut_journal, cut_fence, cut_client, self.lock, 'ns',
                    {k: self.context['images'][k] for k in keys}, cut_intent['frontend_platform_digests'])
                for app in adoption.APPS:
                    phase.run(app)
            cut = writer_cut.WriterCut(cut_journal, cut_fence, cut_client, self.lock, 'ns', admission,
                lambda scope: {'scope': scope, 'active_sessions': 0, 'active_transactions': 0})
            cut._patch = cut_patch
            cut.drain()
            if config:
                import backend_config_transition_deployment as producer
                import backend_config_transition_probe as probe
                targets = []
                for service in producer.SERVICES:
                    original = next(o for o in admission['controllers'] if o['metadata']['name'] == 'voice-' + service)
                    targets.append({'workload': service, 'pod_name': 'voice-' + service + '-pod', 'pod_uid': service + '-pod',
                        'deployment_uid': original['metadata']['uid'], 'container_id': service + '-container', 'restart_count': 0})
                observations = {}
                for target in targets:
                    service = target['workload']
                    original = next(o for o in admission['controllers'] if o['metadata']['name'] == 'voice-' + service)
                    observations[service] = {'status': 'observed', 'identity': {
                        **{k: target[k] for k in ('pod_name', 'pod_uid', 'deployment_uid', 'container_id', 'restart_count')},
                        'namespace_uid': 'ns', 'deployment_rv': original['metadata']['resourceVersion']},
                        'fields': {key: {'state': 'absent', 'origin': None} for key in probe.FIELDS[service]},
                        'env_sources': [], 'explicit_sources': [], 'scope': 'runtime-config-metadata-only-no-provider-validation',
                        'template_binding': 'pod-env-and-deployment-template-v1'}
                observation = {'operation_id': self.context['operation_id'], 'source_sha': self.context['commit'],
                               'nonce': 'c' * 32, 'observations': observations}
                self.ledger.row['backend_config_observation'] = {'status': 'done', 'nonce': observation['nonce'],
                    'operation_id': observation['operation_id'], 'source_sha': observation['source_sha'],
                    'targets_sha256': writer_cut.digest(targets), 'receipt_sha256': writer_cut.digest(observation)}
                config_intent = {'resource_context': self.context, 'namespace_uid': 'ns', 'backend_config_deployments': {
                    'source_sha': self.context['commit'], 'cut_invocation': cut.invocation, 'services': list(producer.SERVICES),
                    'sources': {service: copy.deepcopy(self.client.deployments['voice-' + service]) for service in producer.SERVICES},
                    'observation': observation,
                    'observation_plan': {'source_sha': self.context['commit'], 'targets': targets}}}
                config_journal = journal.Journal(Path(self.temp.name) / 'real-config', config_intent)
                self.addCleanup(config_journal.close)
                cut.bind_config_plan(config_journal)
                for service in producer.SERVICES:
                    producer.apply_development(cut, config_journal, service)
            import auth_live_migration_phase
            auth_journal = journal.Journal(Path(self.temp.name) / 'fake-auth-qualification', {
                'resource_context': self.context, 'namespace_uid': 'ns'})
            self.addCleanup(auth_journal.close)
            auth_target = copy.deepcopy(self.client.deployments['voice-auth']['spec'])
            auth_target['template']['spec']['containers'][0]['image'] = self.context['images']['voice-auth/auth']
            auth_proof = {'candidate': {'target_spec_sha256': cohort.digest(auth_target),
                'image': self.context['images']['voice-auth/auth'],
                'platform_digest': self.intent['full_cohort']['platform_digests']['voice-auth/auth']}}
            qualified = mock.patch.object(auth_live_migration_phase, 'verify_completed', return_value=auth_proof)
            qualified.start(); self.addCleanup(qualified.stop)
            cut.bind_auth_successor(SimpleNamespace(phases=SimpleNamespace(journal=auth_journal)), '.')
            with mock.patch.object(cut, 'current_cut_proof', wraps=cut.current_cut_proof) as proof:
                capture = cohort.prepare_cut(cut)
                self.assertEqual(proof.call_count, 1)
            image_intent = copy.deepcopy(self.intent)
            if frontend:
                image_intent['full_cohort']['cut'] = sorted(set(image_intent['full_cohort']['cut']) - ui_names)
            image_intent['full_cohort']['cut_proof'] = copy.deepcopy(capture.proof)
            for name, obj in self.client.deployments.items():
                snap = image_intent['full_cohort']['workloads'][name]
                snap.update(uid=obj['metadata']['uid'], resourceVersion=obj['metadata']['resourceVersion'],
                            generation=obj['metadata']['generation'], spec=copy.deepcopy(obj['spec']))
            image_journal = journal.Journal(Path(self.temp.name) / 'real-image', image_intent)
            self.addCleanup(image_journal.close)
            self.client.journal = image_journal
            image_phase = cohort.FullCohortImageApply(image_journal, self.fence, self.client, self.lock, 'ns', capture)
            if fault == 'frontend_drift':
                original_get = image_phase._get
                def drift_noop(kind, name):
                    obj = original_get(kind, name)
                    step = image_journal.state['steps'].get('full-cohort-image-' + name, {})
                    if name in ui_names and step.get('status') == 'pending':
                        obj['metadata']['generation'] += 1
                    return obj
                with mock.patch.object(image_phase, '_get', side_effect=drift_noop):
                    with self.assertRaises(cohort.RecoveryRequired): image_phase.apply()
                self.assertEqual(self.ledger.row['status'], 'uncertain')
                self.assertTrue(all(not (s.startswith('full-cohort-image-') and s.removeprefix('full-cohort-image-') in ui_names
                                         and item.get('status') == 'done')
                                    for s, item in image_journal.state['steps'].items()))
                return
            if fault:
                original_patch = self.client.patch
                def drift(name, patch_data):
                    result = original_patch(name, patch_data)
                    if fault == 'admission': cut.admission['linked']['jobs'].append('foreign')
                    if fault == 'poison': cut.poisoned = True
                    if fault == 'config_ledger':
                        self.ledger.row['backend_config_effects']['backend-config-deployment-voice-user']['nonce'] = 'f' * 32
                        self.ledger.version += 1
                    return result
                with mock.patch.object(self.client, 'patch', side_effect=drift):
                    with self.assertRaises(cohort.RecoveryRequired): image_phase.apply()
                self.assertEqual(self.client.patches, 1)
                return
            image_result = image_phase.apply()
            if frontend:
                self.assertTrue(all(image_result[name]['mode'] == 'already-target' for name in ui_names))
                self.assertEqual(self.client.patches, 20)
            for obj in self.client.deployments.values():
                obj['status']['observedGeneration'] = obj['metadata']['generation']
            targets = [copy.deepcopy(obj) for name, obj in self.client.deployments.items() if not frontend or name not in ui_names]
            self.assertEqual(self._activate(cut, targets, image_phase)['status'], 'replicas-restored')
            self.client.replicasets, self.client.pods = [], []
            for name, obj in self.client.deployments.items():
                obj['status'].update(updatedReplicas=1, availableReplicas=1)
                template = copy.deepcopy(obj['spec']['template'])
                template.setdefault('metadata', {}).setdefault('labels', {})['pod-template-hash'] = 'abc'
                rsuid = name + '-rs'
                self.client.replicasets.append({'metadata': {'name': name + '-rs', 'namespace': 'voice-dev', 'uid': rsuid,
                    'resourceVersion': '33', 'ownerReferences': [{'kind': 'Deployment', 'controller': True,
                    'uid': obj['metadata']['uid']}]}, 'spec': {'template': template}})
                c = obj['spec']['template']['spec']['containers'][0]
                self.client.pods.append({'metadata': {'name': name + '-pod', 'namespace': 'voice-dev', 'uid': name + '-pod-uid',
                    'resourceVersion': '34', 'labels': {'app': name}, 'ownerReferences': [
                        {'kind': 'ReplicaSet', 'controller': True, 'uid': rsuid}]},
                    'spec': copy.deepcopy(obj['spec']['template']['spec']), 'status': {'phase': 'Running',
                    'conditions': [{'type': 'Ready', 'status': 'True'}], 'containerStatuses': [{'name': c['name'],
                    'ready': True, 'imageID': 'containerd://sha256:' + 'c' * 64}]}})
            self.assertEqual(len(image_phase.verify_after_resume()), 23)
            if frontend:
                for app in adoption.APPS:
                    phase.verify_current(app)

    def test_real_writer_cut_full_cohort_resume(self):
        self._flow()

    def test_real_cut_admission_drift_blocks_next_image(self):
        self._flow('admission')

    def test_real_cut_poison_blocks_next_image(self):
        self._flow('poison')

    def test_real_config_producer_cut_image_resume_composition(self):
        self._flow(config=True)

    def test_frontend_cut_config_image_resume_and_served_bytes(self):
        self._flow(config=True, frontend=True)

    def test_frontend_already_target_generation_drift_holds(self):
        self._flow('frontend_drift', config=True, frontend=True)

    def test_real_config_producer_ledger_drift_blocks_next_image(self):
        self._flow('config_ledger', config=True)


if __name__ == '__main__': unittest.main()
