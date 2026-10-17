"""Full request-to-Job producer boundary tests; API is deterministic and fake."""
import copy
import json
import sys
import unittest
from unittest.mock import Mock

import tokenless_job as jobs


REQUEST = {
    'operation': 'a' * 12,
    'nonce': 'b' * 32,
    'phase': 'census',
}
BINDING = {
    'claim_name': 'voice-nats-jsdata-d20261017a',
    'claim_uid': '11111111-1111-4111-8111-111111111111',
    'pv_name': 'local-pv-voice-nats',
    'pv_uid': '22222222-2222-4222-8222-222222222222',
    'node_name': 'k3s-worker-1',
    'node_hostname': 'k3s-worker-1.internal',
}
RUNNER_IMAGE = 'ghcr.io/poryadok/voice-nats-rollout-runner@sha256:' + 'a' * 64


class FakeAPI:
    def __init__(self, wanted):
        self.wanted = wanted
        self.calls = []
        self.job = copy.deepcopy(wanted)
        self.job['metadata']['uid'] = '33333333-3333-4333-8333-333333333333'
        uid = self.job['metadata']['uid']; name = wanted['metadata']['name']
        self.job['metadata'].update(resourceVersion='10', generation=1)
        spec = self.job['spec']
        spec.update(completionMode='NonIndexed', completions=1, manualSelector=False,
            parallelism=1, podReplacementPolicy='TerminatingOrFailed', suspend=False,
            selector={'matchLabels': {'batch.kubernetes.io/controller-uid': uid}})
        template = spec['template']
        template['metadata']['labels'].update({
            'batch.kubernetes.io/controller-uid': uid,
            'batch.kubernetes.io/job-name': name,
            'controller-uid': uid, 'job-name': name})
        template_spec = template['spec']
        template_spec.update(dnsPolicy='ClusterFirst', schedulerName='default-scheduler', terminationGracePeriodSeconds=30)
        container = template_spec['containers'][0]
        container.update(terminationMessagePath='/dev/termination-log', terminationMessagePolicy='File')
        self.job['status'] = {'succeeded': 1, 'failed': 0}
        self.pod = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name + '-abcde', 'namespace': jobs.NAMESPACE,
                    'uid': '66666666-6666-4666-8666-666666666666',
                    'labels': copy.deepcopy(template['metadata']['labels']),
                    'ownerReferences': [{'apiVersion': 'batch/v1', 'kind': 'Job', 'name': name, 'uid': uid,
                        'controller': True, 'blockOwnerDeletion': True}]},
            'spec': copy.deepcopy(wanted['spec']['template']['spec']),
            'status': {'phase': 'Succeeded'}}
        self.pod['spec'].update(dnsPolicy='ClusterFirst', schedulerName='default-scheduler', terminationGracePeriodSeconds=30,
            serviceAccountName='default')
        self.claim = {'metadata': {'name': BINDING['claim_name'], 'namespace': jobs.NAMESPACE, 'uid': BINDING['claim_uid']},
            'status': {'phase': 'Bound'}, 'spec': {'volumeName': BINDING['pv_name']}}
        self.pv = {'metadata': {'name': BINDING['pv_name'], 'uid': BINDING['pv_uid']},
            'spec': {'claimRef': {'namespace': jobs.NAMESPACE, 'name': BINDING['claim_name'], 'uid': BINDING['claim_uid']},
                'nodeAffinity': {'required': {'nodeSelectorTerms': [{'matchExpressions': [
                    {'key': 'kubernetes.io/hostname', 'operator': 'In', 'values': [BINDING['node_hostname']]}]}]}}}}
        self.node = {'metadata': {'name': BINDING['node_name'],
            'labels': {'kubernetes.io/hostname': BINDING['node_hostname']}},
            'status': {'conditions': [{'type': 'Ready', 'status': 'True'}]}}

    def get_pvc(self, namespace, name):
        self.calls.append(('get_pvc', namespace, name))
        return copy.deepcopy(self.claim)

    def get_pv(self, name):
        self.calls.append(('get_pv', name))
        return copy.deepcopy(self.pv)

    def get_node(self, name):
        self.calls.append(('get_node', name))
        return copy.deepcopy(self.node)

    def create_job(self, namespace, value):
        self.calls.append(('create_job', namespace, copy.deepcopy(value)))
        return copy.deepcopy(self.job)

    def get_job(self, namespace, name):
        self.calls.append(('get_job', namespace, name))
        return copy.deepcopy(self.job)

    def wait_job(self, namespace, name, uid, timeout):
        self.calls.append(('wait_job', namespace, name, uid, timeout))
        if self.job['metadata']['uid'] != uid: raise jobs.JobError('fixture_uid_changed')
        return copy.deepcopy(self.job)

    def get_job_pod(self, namespace, name):
        self.calls.append(('get_job_pod', namespace, name))
        return copy.deepcopy(self.pod)

    def wait_job_pod(self, namespace, name, uid, timeout):
        self.calls.append(('wait_job_pod', namespace, name, uid, timeout))
        if self.job['metadata']['uid'] != uid: raise jobs.JobError('fixture_uid_changed')
        self.pod['status'] = {'phase': 'Running', 'containerStatuses': [{
            'name': 'nats-phase', 'containerID': 'containerd://' + '7' * 64,
            'ready': True, 'restartCount': 0}]}
        return copy.deepcopy(self.pod)

    def pod_logs(self, namespace, name):
        self.calls.append(('pod_logs', namespace, name))
        return json.dumps({'schema': jobs.RECEIPT_SCHEMA, 'status': 'COMPLETE', 'operation': REQUEST['operation'],
            'nonce': REQUEST['nonce'], 'phase': REQUEST['phase'], 'result_sha256': 'd' * 64}) + '\n'

    def delete_job(self, namespace, name, uid):
        self.calls.append(('delete_job', namespace, name, uid))
        if self.job['metadata']['uid'] != uid: raise jobs.JobError('fixture_uid_changed')
        self.deleted = True
        return True


class TokenlessJobTests(unittest.TestCase):
    def test_candidate_cannot_choose_command_resource_or_extra_fields(self):
        for update in (
            {'command': ['/bin/sh', '-c', 'id']},
            {'image': 'attacker.invalid/x:latest'},
            {'path': '/etc/kubernetes/admin.conf'},
            {'phase': 'exec'},
            {'operation': 'g' * 12},
            {'operation': 'a' * 10},
        ):
            with self.subTest(update=update), self.assertRaises(jobs.JobError):
                jobs.validate_request(REQUEST | update)
        with self.assertRaises(jobs.JobError):
            jobs.validate_request(REQUEST | {'extra': 'field'})

    def test_complete_producer_persists_intent_then_readback_then_owned_cleanup(self):
        wanted = jobs.document(REQUEST, BINDING, RUNNER_IMAGE)
        api = FakeAPI(wanted)
        journal = []
        result = jobs.run(api, REQUEST, BINDING, RUNNER_IMAGE, persist_intent=journal.append, persist_result=journal.append)
        self.assertEqual(result['phase'], 'census')
        self.assertEqual(journal[0]['state'], 'JOB_CREATE_INTENT')
        self.assertEqual(journal[1]['state'], 'JOB_RESULT_READBACK')
        self.assertEqual([call[0] for call in api.calls], ['get_pvc', 'get_pv', 'get_node', 'create_job', 'wait_job', 'get_job_pod', 'pod_logs', 'delete_job'])
        created = next(call[2] for call in api.calls if call[0] == 'create_job')
        pod = created['spec']['template']['spec']
        self.assertFalse(pod['automountServiceAccountToken'])
        self.assertEqual(pod['nodeName'], BINDING['node_name'])
        self.assertEqual(pod['securityContext'], {
            'runAsNonRoot': True, 'runAsUser': 65532, 'runAsGroup': 65532,
            'fsGroup': 10000, 'fsGroupChangePolicy': 'OnRootMismatch',
            'seccompProfile': {'type': 'RuntimeDefault'}})
        self.assertEqual(pod['volumes'][0]['persistentVolumeClaim']['claimName'], BINDING['claim_name'])
        self.assertEqual(created['spec']['ttlSecondsAfterFinished'], 600)
        self.assertEqual(api.job['spec']['template']['spec']['serviceAccountName'], 'default')
        self.assertEqual(api.calls[-1][-1], api.job['metadata']['uid'])

    def test_wrong_created_uid_or_failed_job_never_reads_result_or_cleans_foreign_job(self):
        wanted = jobs.document(REQUEST, BINDING, RUNNER_IMAGE)
        api = FakeAPI(wanted)
        api.created_uid = api.job['metadata']['uid']
        api.job['metadata']['uid'] = '44444444-4444-4444-8444-444444444444'
        # The API's immediate create response binds the original UID, then a
        # subsequent GET showing another UID must stop before result/cleanup.
        def created_then_changed(namespace, value):
            api.calls.append(('create_job', namespace, copy.deepcopy(value)))
            api.job['metadata']['uid'] = api.created_uid
            result = copy.deepcopy(api.job)
            api.job['metadata']['uid'] = '44444444-4444-4444-8444-444444444444'
            return result
        api.create_job = created_then_changed
        with self.assertRaises(jobs.JobError):
            jobs.run(api, REQUEST, BINDING, RUNNER_IMAGE, persist_intent=Mock(), persist_result=Mock())
        self.assertFalse(any(call[0] in ('get_job_pod', 'pod_logs', 'delete_job') for call in api.calls))

        api = FakeAPI(wanted)
        api.job['status'] = {'failed': 1}
        with self.assertRaises(jobs.JobError):
            jobs.run(api, REQUEST, BINDING, RUNNER_IMAGE, persist_intent=Mock(), persist_result=Mock())
        self.assertFalse(any(call[0] in ('get_job_pod', 'pod_logs', 'delete_job') for call in api.calls))

    def test_pod_must_be_owned_tokenless_and_on_pinned_node(self):
        for mutate in (
            lambda api: api.pod['metadata']['ownerReferences'][0].update(uid='55555555-5555-4555-8555-555555555555'),
            lambda api: api.pod['spec'].update(automountServiceAccountToken=True),
            lambda api: api.pod['spec'].update(nodeName='other-node'),
            lambda api: api.pod['spec']['securityContext'].update(fsGroup=2000),
            lambda api: api.pod['spec']['securityContext'].pop('fsGroup'),
            lambda api: api.pod['status'].update(phase='Running'),
        ):
            api = FakeAPI(jobs.document(REQUEST, BINDING, RUNNER_IMAGE))
            mutate(api)
            with self.assertRaises(jobs.JobError):
                jobs.run(api, REQUEST, BINDING, RUNNER_IMAGE, persist_intent=Mock(), persist_result=Mock())
            self.assertNotIn('delete_job', [call[0] for call in api.calls])

    def test_pv_or_claim_uid_drift_fails_before_job_creation(self):
        for mutate in (
            lambda api: api.claim['metadata'].update(uid='55555555-5555-4555-8555-555555555555'),
            lambda api: api.pv['spec']['claimRef'].update(uid='55555555-5555-4555-8555-555555555555'),
            lambda api: api.pv['metadata'].update(uid='55555555-5555-4555-8555-555555555555'),
            lambda api: api.pv['spec']['nodeAffinity']['required']['nodeSelectorTerms'][0]['matchExpressions'][0].update(values=['wrong-host']),
            lambda api: api.node['metadata']['labels'].update({'kubernetes.io/hostname':'wrong-host'}),
        ):
            api = FakeAPI(jobs.document(REQUEST, BINDING, RUNNER_IMAGE))
            mutate(api)
            with self.assertRaises(jobs.JobError):
                jobs.run(api, REQUEST, BINDING, RUNNER_IMAGE, persist_intent=Mock(), persist_result=Mock())
            self.assertNotIn('create_job', [call[0] for call in api.calls])


if __name__ == '__main__':
    unittest.main()
