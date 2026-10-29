import copy
import unittest
from pathlib import Path
from unittest import mock

import frontend_runtime_executor
import full_cohort_image_apply as cohort
import full_cohort_image_apply_test as fixture
import media_operation_journal as journal
import resource_fencing
import writer_cut


class Composition(unittest.TestCase):
    def setUp(self):
        fixture.FullCohortTest.setUp(self)

    def _flow(self, fault=None, config=False):
        self.cut_journal.close()
        admission = copy.deepcopy(self.cut_journal.intent['writer_cut'])
        admission.update(guards={'jobs': [], 'cronjobs': [], 'hpas': []}, linked={'jobs': [], 'cronjobs': []})
        for source in admission['controllers']:
            source['apiVersion'] = 'apps/v1'
        cut_intent = {'resource_context': self.context, 'namespace_uid': 'ns', 'writer_cut': admission}
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
        def kube_call(*args, **kwargs):
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
                        'env_sources': [], 'scope': 'runtime-config-metadata-only-no-provider-validation',
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
            with mock.patch.object(cut, 'current_cut_proof', wraps=cut.current_cut_proof) as proof:
                capture = cohort.prepare_cut(cut)
                self.assertEqual(proof.call_count, 1)
            image_intent = copy.deepcopy(self.intent)
            image_intent['full_cohort']['cut_proof'] = copy.deepcopy(capture.proof)
            for name, obj in self.client.deployments.items():
                snap = image_intent['full_cohort']['workloads'][name]
                snap.update(uid=obj['metadata']['uid'], resourceVersion=obj['metadata']['resourceVersion'],
                            generation=obj['metadata']['generation'], spec=copy.deepcopy(obj['spec']))
            image_journal = journal.Journal(Path(self.temp.name) / 'real-image', image_intent)
            self.addCleanup(image_journal.close)
            self.client.journal = image_journal
            image_phase = cohort.FullCohortImageApply(image_journal, self.fence, self.client, self.lock, 'ns', capture)
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
            image_phase.apply()
            for obj in self.client.deployments.values():
                obj['status']['observedGeneration'] = obj['metadata']['generation']
            targets = [copy.deepcopy(obj) for obj in self.client.deployments.values()]
            self.assertEqual(cut.resume(targets)['status'], 'replicas-restored')
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

    def test_real_writer_cut_full_cohort_resume(self):
        self._flow()

    def test_real_cut_admission_drift_blocks_next_image(self):
        self._flow('admission')

    def test_real_cut_poison_blocks_next_image(self):
        self._flow('poison')

    def test_real_config_producer_cut_image_resume_composition(self):
        self._flow(config=True)

    def test_real_config_producer_ledger_drift_blocks_next_image(self):
        self._flow('config_ledger', config=True)


if __name__ == '__main__': unittest.main()
