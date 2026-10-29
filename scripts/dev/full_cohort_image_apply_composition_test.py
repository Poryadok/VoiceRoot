import copy
import unittest
from pathlib import Path
from unittest import mock

import frontend_runtime_executor
import full_cohort_image_apply as cohort
import full_cohort_image_apply_test as fixture
import media_operation_journal as journal
import writer_cut


class Composition(unittest.TestCase):
    def setUp(self):
        fixture.FullCohortTest.setUp(self)

    def test_real_writer_cut_full_cohort_resume(self):
        self.cut_journal.close()
        admission = copy.deepcopy(self.cut_journal.intent['writer_cut'])
        admission.update(guards={'jobs': [], 'cronjobs': [], 'hpas': []}, linked={'jobs': [], 'cronjobs': []})
        cut_intent = {'resource_context': self.context, 'namespace_uid': 'ns', 'writer_cut': admission}
        cut_journal = journal.Journal(Path(self.temp.name) / 'real-cut', cut_intent)
        self.addCleanup(cut_journal.close)
        self.ledger.row.pop('writer_cut_invocation', None)
        self.ledger.row.pop('writer_cut_effects', None)
        for source in admission['controllers']:
            obj = self.client.deployments[source['metadata']['name']]
            obj['spec'] = copy.deepcopy(source['spec'])
            obj['metadata'] = copy.deepcopy(source['metadata'])
            obj['status'] = {'observedGeneration': 1, 'replicas': 1, 'readyReplicas': 1}
        call = self.client.call
        def kube_call(*args, **kwargs):
            if args[1] in ('jobs', 'cronjobs', 'horizontalpodautoscalers'):
                return {'items': []}
            return call(*args, **kwargs)
        self.client.call = kube_call
        self.client.journal = cut_journal
        def cut_patch(kind, name, operations):
            obj = self.client.deployments[name]
            self.assertEqual((operations[0]['value'], operations[1]['value'], operations[2]['value']),
                             (obj['metadata']['uid'], obj['metadata']['resourceVersion'], obj['spec']))
            changed = obj['spec']['replicas'] != operations[3]['value']
            obj['spec']['replicas'] = operations[3]['value']
            obj['metadata']['generation'] += changed
            obj['metadata']['resourceVersion'] = str(int(obj['metadata']['resourceVersion']) + 1)
            obj['status'] = {'observedGeneration': obj['metadata']['generation'], 'replicas': obj['spec']['replicas'],
                             'readyReplicas': obj['spec']['replicas']}
            return copy.deepcopy(obj)
        with mock.patch.object(frontend_runtime_executor, 'KubectlTransport', fixture.Cluster):
            cut = writer_cut.WriterCut(cut_journal, self.fence, self.client, self.lock, 'ns', admission,
                lambda scope: {'scope': scope, 'active_sessions': 0, 'active_transactions': 0})
            cut._patch = cut_patch
            cut.drain()
            image_intent = copy.deepcopy(self.intent)
            for name, obj in self.client.deployments.items():
                snap = image_intent['full_cohort']['workloads'][name]
                snap.update(uid=obj['metadata']['uid'], resourceVersion=obj['metadata']['resourceVersion'],
                            generation=obj['metadata']['generation'], spec=copy.deepcopy(obj['spec']))
            image_journal = journal.Journal(Path(self.temp.name) / 'real-image', image_intent)
            self.addCleanup(image_journal.close)
            self.client.journal = image_journal
            image_phase = cohort.FullCohortImageApply(image_journal, self.fence, self.client, self.lock, 'ns', cut_journal)
            image_phase.apply()
            for obj in self.client.deployments.values():
                obj['status']['observedGeneration'] = obj['metadata']['generation']
            targets = [copy.deepcopy(obj) for obj in self.client.deployments.values()]
            self.client.journal = cut_journal
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
            self.client.journal = image_journal
            self.assertEqual(len(image_phase.verify_after_resume()), 23)


if __name__ == '__main__': unittest.main()
