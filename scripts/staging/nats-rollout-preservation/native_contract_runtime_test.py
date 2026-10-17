"""Focused fake-API tests for the fixed native contract migration adapter."""
from __future__ import annotations

import base64
import copy
import hashlib
import json
import unittest

import nats_contract_plan
import nats_contract_plan_test
import nats_contract_runner
import native_contract_runtime as runtime
import verification_job


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'),
        ensure_ascii=False, allow_nan=False).encode('utf-8')


def fixture():
    plan = nats_contract_plan_test.Tests().plan()
    # One real canonical action is enough to exercise the full durable path.
    plan['actions'] = [copy.deepcopy(plan['actions'][0])]
    operation, nonce, attempt = 'a' * 12, 'b' * 32, 'c' * 8
    source_uid, source_pv_uid = '11111111-1111-4111-8111-111111111111', '22222222-2222-4222-8222-222222222222'
    verify_uid, verify_pv_uid = '33333333-3333-4333-8333-333333333333', '44444444-4444-4444-8444-444444444444'
    creds, server = b'bootstrap credentials fixture', b'operator: {}\nresolver: MEMORY\n'
    plan_bytes = canonical(plan)
    plan_sha = nats_contract_plan.digest(plan)
    archive = b'captured canonical archive fixture'
    base = {
        'schema': nats_contract_runner.INPUT_SCHEMA,
        'operation': operation, 'nonce': nonce, 'contract_attempt': attempt,
        'source_pvc_uid': source_uid, 'source_pv_uid': source_pv_uid,
        'verification_pvc_uid': verify_uid, 'verification_pv_uid': verify_pv_uid,
        'archive_sha256': hashlib.sha256(archive).hexdigest(), 'archive_bytes': len(archive),
        'server_config_sha256': hashlib.sha256(server).hexdigest(),
        'plan_sha256': plan_sha, 'plan': plan,
        'files': {'bootstrap.creds': base64.b64encode(creds).decode(),
            'server.conf': base64.b64encode(server).decode()},
    }
    envelope = canonical(base)
    request = {'operation': operation, 'nonce': nonce, 'phase': 'contract-migration'}
    binding = {
        'source_claim': 'voice-nats-jsdata-d20261017a', 'source_uid': source_uid,
        'source_pv_name': 'pv-source', 'source_pv_uid': source_pv_uid,
        'source_node_name': 'node-a', 'source_hostname': 'node-a',
        'verification_claim': 'voice-nats-verify-' + operation,
        'verification_uid': verify_uid, 'verification_pv_name': 'pv-verify',
        'verification_pv_uid': verify_pv_uid, 'verification_node_name': 'node-a',
        'verification_hostname': 'node-a', 'contract_attempt': attempt,
        'input_bundle_sha256': hashlib.sha256(envelope).hexdigest(),
        'input_bundle_bytes': len(envelope), 'archive_sha256': base['archive_sha256'],
        'archive_bytes': len(archive), 'plan_sha256': plan_sha,
        'plan_bytes': len(plan_bytes), 'server_config_sha256': base['server_config_sha256'],
        'bootstrap_creds_sha256': hashlib.sha256(creds).hexdigest(),
        'bootstrap_creds_bytes': len(creds), 'server_config_bytes': len(server),
    }
    job_uid = '55555555-5555-4555-8555-555555555555'
    pod_uid = '66666666-6666-4666-8666-666666666666'
    pod_name = verification_job.job_name(request) + '-abcde'
    image = 'registry.example/voice-helper@sha256:' + 'a' * 64
    return request, binding, plan, envelope, attempt, job_uid, pod_uid, pod_name, image


def line(value):
    return canonical(value) + b'\n'


class FakeAPI:
    def __init__(self, data):
        request, binding, plan, _envelope, attempt, _job_uid, pod_uid, _pod_name, _image = data
        self.request, self.binding, self.plan, self.attempt, self.pod_uid = request, binding, plan, attempt, pod_uid
        self.calls = []
        self.current = {action['object']: copy.deepcopy(action['before']) for action in plan['actions']}
        self.receipts = {}
        self.started = None
        self.stopped = None
        self.fail_after_execute = False
        self.actual_pod_uid = pod_uid

    def _assert_pod(self, pod_uid):
        if pod_uid != self.actual_pod_uid:
            raise RuntimeError('typed API rejected changed Pod UID')

    def read_contract_receipt(self, namespace, pod_name, pod_uid, request, binding, attempt, kind, action_index=None):
        self._assert_pod(pod_uid)
        self.calls.append(('read', kind, action_index))
        if kind in self.receipts:
            return line(self.receipts[kind])
        if kind == 'action' and action_index in self.receipts:
            return line(self.receipts[action_index])
        raise RuntimeError('receipt absent')

    def exec_pod_contract_input(self, namespace, pod_name, pod_uid, request, binding, bundle_bytes):
        self._assert_pod(pod_uid)
        self.calls.append(('input',))
        expected = expected_binding(self)
        receipt = runtime._input_receipt(expected)
        self.receipts['input'] = receipt
        return line(receipt)

    def exec_pod_contract(self, namespace, pod_name, pod_uid, request, binding, action,
            *, action_index=None, journal_hashes=None):
        self._assert_pod(pod_uid)
        self.calls.append((action, action_index))
        expected = expected_binding(self)
        if action == 'broker-start':
            self.started = {'schema': 'voice-nats-supervisor-v1', 'ok': True, 'running': True,
                'pid': 912, 'started_at_ns': 1000000000}
            row = broker_receipt(expected, 'start', self.started)
            self.receipts['broker-start'] = row
            return line(row)
        if action == 'broker-stop':
            self.stopped = {**self.started, 'running': False, 'finished_at_ns': 2000000000}
            row = broker_receipt(expected, 'stop', self.stopped)
            self.receipts['broker-stop'] = row
            return line(row)
        if action == 'broker-status':
            broker = self.started if self.started and self.stopped is None else (
                {**self.started, 'running': False, 'pid': None, 'started_at_ns': None}
                if self.started else {'running': False, 'pid': None, 'started_at_ns': None})
            return line({'schema': nats_contract_runner.RECEIPT_SCHEMA, **expected,
                'status': 'READ_ONLY', 'action': 'broker-status', 'broker': broker})
        assert action_index is not None
        contract_action = self.plan['actions'][action_index]
        target = contract_action.get('normalized_config_sha256', nats_contract_plan.digest(contract_action['after']))
        if action == 'inspect-action':
            current = copy.deepcopy(self.current[contract_action['object']])
            before_hash = nats_contract_plan.digest(current)
            state = 'TARGET' if before_hash == target else ('BEFORE' if current == contract_action['before'] else 'DRIFT')
            return line({'schema': 'voice-nats-contract-inspection-v1', 'action_index': action_index,
                'object': contract_action['object'], 'api': contract_action['api'],
                'before_sha256': before_hash, 'target_sha256': target, 'current': current, 'state': state})
        if action == 'execute-action':
            rows = self.journal_events[-2:]
            if [event.get('kind') for event in rows] != [
                'nats_contract_intent', 'nats_contract_mutation_issued']:
                raise AssertionError('host journal was not durable before execute')
            wanted_hashes = {
                'intent_sha256': hashlib.sha256(nats_contract_runner._canonical(rows[0])).hexdigest(),
                'issued_sha256': hashlib.sha256(nats_contract_runner._canonical(rows[1])).hexdigest(),
            }
            if journal_hashes != wanted_hashes:
                raise AssertionError('execute did not bind exact durable journal rows')
            self.current[contract_action['object']] = copy.deepcopy(contract_action['after'])
            base_intent = {'kind': 'nats_contract_intent', 'object': contract_action['object'],
                'api': contract_action['api'], 'before_sha256': nats_contract_plan.digest(contract_action['before']),
                'target_sha256': target, 'migration_attempt': self.attempt}
            base_issued = {'kind': 'nats_contract_mutation_issued', 'object': contract_action['object'],
                'api': contract_action['api'], 'target_sha256': target, 'migration_attempt': self.attempt}
            receipt = {'schema': nats_contract_runner.RECEIPT_SCHEMA, **expected,
                'status': 'COMPLETE', 'action_index': action_index, 'object': contract_action['object'],
                'api': contract_action['api'],
                'before_event_sha256': hashlib.sha256(nats_contract_runner._canonical(base_intent)).hexdigest(),
                'issued_event_sha256': hashlib.sha256(nats_contract_runner._canonical(base_issued)).hexdigest(),
                'target_sha256': target, 'config_sha256': target, 'recovered_exact_target': False}
            self.receipts[action_index] = receipt
            if self.fail_after_execute:
                self.fail_after_execute = False
                raise TimeoutError('simulated disconnect after mutation')
            return line(receipt)
        raise AssertionError('unexpected API action ' + action)


def expected_binding(api):
    request, b = api.request, api.binding
    return {
        'operation': request['operation'], 'nonce': request['nonce'], 'pod_uid': api.pod_uid,
        'contract_attempt': api.attempt, 'source_pvc_uid': b['source_uid'], 'source_pv_uid': b['source_pv_uid'],
        'verification_pvc_uid': b['verification_uid'], 'verification_pv_uid': b['verification_pv_uid'],
        **{key: b[key] for key in ('input_bundle_sha256','input_bundle_bytes','archive_sha256','archive_bytes',
            'server_config_sha256','server_config_bytes','plan_sha256','plan_bytes',
            'bootstrap_creds_sha256','bootstrap_creds_bytes')},
    }


def broker_receipt(expected, action, broker):
    return {'schema': nats_contract_runner.RECEIPT_SCHEMA, **expected,
        'status': 'COMPLETE', 'action': 'broker-' + action, 'broker': broker}


class NativeContractRuntimeTests(unittest.TestCase):
    def invoke(self, data, api, events):
        request, binding, plan, envelope, attempt, job_uid, pod_uid, pod_name, image = data
        api.journal_events = events
        return runtime.apply_contract(api, request, binding, pod_name=pod_name, pod_uid=pod_uid,
            job_uid=job_uid, runner_image=image, validated_plan=plan, plan_envelope_bytes=envelope,
            contract_attempt=attempt, journal_event=events.append, revalidate=lambda: None,
            journal_events=events)

    def test_complete_contract_migration_journals_before_mutation_and_stops_broker(self):
        data = fixture(); api = FakeAPI(data); events = []
        result = self.invoke(data, api, events)
        self.assertEqual(result, [{'object': data[2]['actions'][0]['object'],
            'config_sha256': nats_contract_plan.digest(data[2]['actions'][0]['after'])}])
        kinds = [event['kind'] for event in events]
        self.assertLess(kinds.index('nats_contract_intent'), kinds.index('nats_contract_mutation_issued'))
        self.assertLess(kinds.index('nats_contract_mutation_issued'), kinds.index('nats_contract_applied'))
        self.assertEqual([name for name, *_ in api.calls if name in ('execute-action','broker-stop')],
            ['execute-action','broker-stop'])

    def test_transport_unknown_after_issued_reconciles_receipt_and_never_replays(self):
        data = fixture(); api = FakeAPI(data); api.fail_after_execute = True; events = []
        self.invoke(data, api, events)
        self.assertEqual(sum(call[0] == 'execute-action' for call in api.calls), 1)
        self.assertTrue(any(call[:2] == ('read', 'action') for call in api.calls))
        self.assertTrue(any(event['kind'] == 'nats_contract_applied' for event in events))

    def test_completed_same_pod_session_is_readback_only_on_reentry(self):
        data = fixture(); api = FakeAPI(data); events = []
        first = self.invoke(data, api, events)
        loaded_event = next(event for event in events if event['kind'] == 'nats_contract_input_loaded')
        expected_receipt = runtime._input_receipt(expected_binding(api))
        self.assertEqual(loaded_event['receipt_sha256'], hashlib.sha256(canonical(expected_receipt)).hexdigest())
        expected_loaded = {'kind': 'nats_contract_input_loaded', 'contract_attempt': data[4],
            'pod_uid': data[6], 'plan_sha256': data[1]['plan_sha256'],
            'input_bundle_sha256': data[1]['input_bundle_sha256'],
            'receipt_sha256': hashlib.sha256(canonical(expected_receipt)).hexdigest()}
        self.assertEqual(loaded_event, expected_loaded)
        first_call_count = len(api.calls)
        second = self.invoke(data, api, events)
        self.assertEqual(second, first)
        self.assertEqual([call[0] for call in api.calls[first_call_count:]],
            ['read', 'read', 'read', 'read', 'read'])

    def test_issued_input_without_exact_receipt_never_replays_load(self):
        data = fixture(); api = FakeAPI(data)
        expected = expected_binding(api)
        attempt = data[4]
        plan_digest = expected['plan_sha256']
        events = [
            {'kind': 'nats_contract_input_load_intent', 'contract_attempt': attempt,
                'pod_uid': data[6], 'plan_sha256': plan_digest,
                'input_bundle_sha256': expected['input_bundle_sha256'],
                'input_bundle_bytes': expected['input_bundle_bytes']},
            {'kind': 'nats_contract_input_load_issued', 'contract_attempt': attempt,
                'pod_uid': data[6], 'plan_sha256': plan_digest,
                'input_bundle_sha256': expected['input_bundle_sha256']},
        ]
        with self.assertRaises(runtime.NativeContractRuntimeError): self.invoke(data, api, events)
        self.assertFalse(any(call[0] == 'input' for call in api.calls))
        self.assertFalse(any(call[0] in ('broker-start', 'execute-action') for call in api.calls))

    def test_unknown_issued_action_without_same_pod_receipt_fails_without_replay(self):
        data = fixture(); api = FakeAPI(data)
        action = data[2]['actions'][0]
        target = nats_contract_plan.digest(action['after'])
        intent = {'kind':'nats_contract_intent','object':action['object'],'api':action['api'],
            'before_sha256':nats_contract_plan.digest(action['before']),'target_sha256':target,
            'migration_attempt':data[4]}
        issued = {'kind':'nats_contract_mutation_issued','object':action['object'],'api':action['api'],
            'target_sha256':target,'migration_attempt':data[4]}
        events = [intent, issued]
        api.current[action['object']] = copy.deepcopy(action['before'])
        with self.assertRaises(runtime.NativeContractRuntimeError): self.invoke(data, api, events)
        self.assertEqual(sum(call[0] == 'execute-action' for call in api.calls), 0)

    def test_replacement_identity_cannot_reconcile_or_execute(self):
        data = fixture(); api = FakeAPI(data); events = []
        bad = list(data); bad[6] = '77777777-7777-4777-8777-777777777777'
        with self.assertRaises(runtime.NativeContractRuntimeError): self.invoke(tuple(bad), api, events)
        self.assertEqual(api.calls, [])

    def test_candidate_plan_envelope_mismatch_fails_before_api(self):
        data = list(fixture()); data[3] = data[3].replace(b'contract_attempt', b'contract_attemptX')
        api = FakeAPI(tuple(data)); events = []
        with self.assertRaises(runtime.NativeContractRuntimeError): self.invoke(tuple(data), api, events)
        self.assertEqual(api.calls, [])


if __name__ == '__main__':
    unittest.main()
