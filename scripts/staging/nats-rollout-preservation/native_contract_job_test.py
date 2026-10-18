import copy
import unittest
from unittest.mock import patch

import kube_json_api
import transaction
import native_contract_runtime
import verification_job


JOB_UID = '11111111-1111-4111-8111-111111111111'
POD_UID = '22222222-2222-4222-8222-222222222222'
OP = 'a' * 12
NONCE = 'b' * 32
ATTEMPT = 'c' * 8
IMAGE = 'registry.example/phase@sha256:' + 'd' * 64


class FakeAPI:
    def __init__(self, *, create_unknown=False, pod_uid=POD_UID, replacement_uid=None):
        self.calls = []
        self.job = None
        self.create_unknown = create_unknown
        self.pod_uid = pod_uid
        self.replacement_uid = replacement_uid

    def get_job(self, namespace, name):
        self.calls.append(('get_job', namespace, name))
        if self.job is None:
            raise kube_json_api.KubeNotFound('missing')
        return copy.deepcopy(self.job)

    def create_job(self, namespace, wanted):
        self.calls.append(('create_job', namespace, wanted['metadata']['name']))
        self.job = copy.deepcopy(wanted)
        self.job['metadata']['uid'] = JOB_UID
        if self.create_unknown:
            raise TimeoutError('lost create response')
        return copy.deepcopy(self.job)

    def get_job_pod(self, namespace, name):
        self.calls.append(('get_job_pod', namespace, name))
        pod = {'metadata': {'uid': self.pod_uid, 'name': name + '-abcde'}, 'status': {
            'phase': 'Running',
            'initContainerStatuses': [{'name': 'nats', 'containerID': 'containerd://nats',
                'restartCount': 0, 'ready': True, 'state': {'running': {}}}],
            'containerStatuses': [{'name': 'control', 'containerID': 'containerd://control',
                'restartCount': 0, 'ready': True, 'state': {'running': {}}}]}}
        return pod

    def get_pod(self, namespace, name):
        pod = self.get_job_pod(namespace, name)
        if self.replacement_uid is not None:
            pod['metadata']['uid'] = self.replacement_uid
        return pod

    def delete_job(self, namespace, name, uid):
        self.calls.append(('delete_job', namespace, name, uid))
        if self.job is None or self.job['metadata'].get('uid') != uid:
            raise RuntimeError('job identity changed')
        self.job = None
        return True


def inputs():
    request = {'operation': OP, 'nonce': NONCE, 'phase': 'contract-migration'}
    binding = {'verification_attempt': ATTEMPT, 'contract_attempt': 'f' * 8,
        'source_uid': 'source-pvc-uid', 'source_pv_uid': 'source-pv-uid',
        'verification_uid': 'verify-pvc-uid', 'verification_pv_uid': 'verify-pv-uid',
        'source_node_name': 'node-a'}
    wanted = {'metadata': {'name': 'voice-nats-contract-aabbccddeeff', 'namespace': 'voice-staging'},
        'spec': {'template': {'metadata': {}, 'spec': {}}}}
    plan = {'actions': [{'object': 'chat_events'}]}
    envelope = b'{}'
    return request, binding, wanted, plan, envelope


class NativeContractJobTests(unittest.TestCase):
    def invoke(self, api, *, events=None, adapter=None):
        request, binding, wanted, plan, envelope = inputs()
        events = [] if events is None else events
        adapter = (lambda *args, **kwargs: [{'object': 'chat_events', 'config_sha256': 'e' * 64}]) if adapter is None else adapter
        with patch.object(verification_job, 'document', return_value=wanted), \
             patch.object(verification_job, 'validate_contract_binding', return_value=binding), \
             patch.object(verification_job, 'verify_job', side_effect=lambda actual, expected, expected_uid=None: actual), \
             patch.object(verification_job, 'verify_pod', side_effect=lambda actual, *args, **kwargs: actual), \
             patch.object(native_contract_runtime, 'apply_contract', side_effect=adapter):
            return transaction.run_native_contract_job(api, request, binding, runner_image=IMAGE,
                plan=plan, plan_envelope=envelope, journal_events=events,
                journal_event=events.append, revalidate=lambda: None)

    def test_create_timeout_adopts_only_exact_readback_and_executes_once(self):
        api = FakeAPI(create_unknown=True)
        events = []
        result = self.invoke(api, events=events)
        self.assertEqual(result['applied'], [{'object': 'chat_events', 'config_sha256': 'e' * 64}])
        self.assertEqual([call[0] for call in api.calls].count('create_job'), 1)
        kinds = [call[0] for call in api.calls]
        self.assertEqual(kinds.count('create_job'), 1)
        self.assertTrue(any(kind == 'get_job' for kind in kinds[kinds.index('create_job') + 1:]))
        self.assertEqual(events[0]['kind'], 'nats_contract_job_create_intent')
        self.assertEqual(events[1]['kind'], 'nats_contract_job_create_issued')
        self.assertEqual(events[2]['kind'], 'nats_contract_job_created')
        self.assertEqual(events[-3]['kind'], 'nats_contract_job_close_intent')
        self.assertEqual(events[-2]['kind'], 'nats_contract_job_close_issued')
        self.assertEqual(events[-1]['kind'], 'nats_contract_job_closed')
        self.assertEqual(kinds.count('delete_job'), 1)

    def test_replaced_pod_fails_before_contract_adapter(self):
        api = FakeAPI(replacement_uid='33333333-3333-4333-8333-333333333333')
        events = []
        with self.assertRaises(Exception):
            self.invoke(api, events=events, adapter=lambda *args, **kwargs: self.fail('adapter ran'))
        self.assertFalse(any(call[0] == 'delete_job' for call in api.calls))


if __name__ == '__main__':
    unittest.main()
