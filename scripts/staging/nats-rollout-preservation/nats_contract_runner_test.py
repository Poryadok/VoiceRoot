import copy
import json
import tempfile
import unittest
from pathlib import Path

from nats_contract_plan import digest
from nats_contract_plan_test import Tests as PlanFixtures
from nats_contract_runner import (SCHEMA, ContractRunnerError, NatsActor,
    _canonical, action_readback, execute_action, inspect_action, load_plan,
    validate_envelope, validate_plan)


def context(plan):
    return {
        'operation': '012345abcdef',
        'nonce': '0123456789abcdef0123456789abcdef',
        'pod_uid': '12345678-1234-1234-1234-123456789abc',
        'contract_attempt': 'abcdef12',
        'source_pvc_uid': '22345678-1234-1234-1234-123456789abc',
        'source_pv_uid': '32345678-1234-1234-1234-123456789abc',
        'verification_pvc_uid': '42345678-1234-1234-1234-123456789abc',
        'verification_pv_uid': '52345678-1234-1234-1234-123456789abc',
    }


def envelope(plan):
    return {'schema': SCHEMA, **context(plan), 'plan_sha256': digest(plan), 'plan': plan}


class FakeActor:
    def __init__(self, *, isolation_check):
        self.isolation_check = isolation_check
        self.current = {}
        self.mutations = []

    def assert_closed_isolation(self):
        self.isolation_check()

    def info_config(self, obj):
        self.assert_closed_isolation()
        return copy.deepcopy(self.current.get(obj))

    def mutate(self, api, payload):
        self.assert_closed_isolation()
        self.mutations.append(api)
        if '.STREAM.UPDATE.' in api:
            obj = api.rsplit('.', 1)[1]
            self.current[obj] = copy.deepcopy(payload)
        else:
            stream, durable = api.split('.CREATE.', 1)[1].split('.', 1)
            self.current[stream + '/' + durable] = copy.deepcopy(payload['config'])


class ContractRunnerTests(unittest.TestCase):
    def setUp(self):
        self.plan = PlanFixtures().plan()
        self.ctx = context(self.plan)
        self.input = envelope(self.plan)

    def test_envelope_requires_exact_context_canonical_bytes_and_plan_digest(self):
        raw = json.dumps(self.input, sort_keys=True, separators=(',', ':')).encode()
        self.assertEqual(validate_envelope(raw, expected=self.ctx)['plan'], self.plan)
        with self.assertRaises(ContractRunnerError):
            validate_envelope(raw + b' ', expected=self.ctx)
        with self.assertRaises(ContractRunnerError):
            validate_envelope(raw, expected={**self.ctx, 'pod_uid': '62345678-1234-1234-1234-123456789abc'})
        changed = copy.deepcopy(self.input)
        changed['plan']['actions'][0]['after']['max_msgs'] = 10
        with self.assertRaises(ContractRunnerError):
            validate_envelope(json.dumps(changed, sort_keys=True, separators=(',', ':')).encode(), expected=self.ctx)

    def test_duplicate_keys_extra_fields_and_arbitrary_api_are_rejected(self):
        raw = json.dumps(self.input, sort_keys=True, separators=(',', ':')).encode()
        duplicate = raw[:-1] + b',"schema":"voice-nats-contract-execution-v1"}'
        with self.assertRaises(ContractRunnerError):
            validate_envelope(duplicate, expected=self.ctx)
        bad = copy.deepcopy(self.input); bad['command'] = ['/bin/sh']
        with self.assertRaises(ContractRunnerError):
            validate_envelope(json.dumps(bad, sort_keys=True, separators=(',', ':')).encode(), expected=self.ctx)
        bad = copy.deepcopy(self.input); bad['plan']['actions'][0]['api'] = '$JS.API.STREAM.DELETE.chat_events'
        bad['plan_sha256'] = digest(bad['plan'])
        with self.assertRaises(ContractRunnerError):
            validate_envelope(json.dumps(bad, sort_keys=True, separators=(',', ':')).encode(), expected=self.ctx)

    def test_action_is_one_exact_mutation_after_host_journal_and_same_attempt_receipt(self):
        action_index = 0
        action = self.plan['actions'][action_index]
        actor = FakeActor(isolation_check=lambda: None)
        actor.current[action['object']] = copy.deepcopy(action['before'])
        with tempfile.TemporaryDirectory() as tmp:
            load_plan(self.input, expected=self.ctx, state_root=Path(tmp))
            inspection = inspect_action(self.plan, action_index, actor)
            self.assertEqual(inspection['state'], 'BEFORE')
            self.assertEqual(inspection['before_sha256'], digest(action['before']))
            intent = {'kind': 'nats_contract_intent', 'object': action['object'], 'api': action['api'],
                'before_sha256': digest(action['before']), 'target_sha256': digest(action['after']),
                'migration_attempt': self.ctx['contract_attempt']}
            issued = {'kind': 'nats_contract_mutation_issued', 'object': action['object'],
                'api': action['api'], 'target_sha256': digest(action['after']),
                'migration_attempt': self.ctx['contract_attempt']}
            receipt = execute_action(self.plan, action_index, binding={**self.ctx,
                'plan_sha256': digest(self.plan)}, before_event_sha256=hashlib_sha(intent),
                issued_event_sha256=hashlib_sha(issued), state_root=Path(tmp), actor=actor)
            self.assertEqual(receipt['status'], 'COMPLETE')
            self.assertEqual(actor.mutations, [action['api']])
            self.assertEqual(action_readback(state_root=Path(tmp), binding={**self.ctx,
                'plan_sha256': digest(self.plan)}, index=action_index), receipt)

    def test_interrupted_action_is_unknown_and_cannot_be_replayed(self):
        action = self.plan['actions'][0]
        actor = FakeActor(isolation_check=lambda: None)
        actor.current[action['object']] = copy.deepcopy(action['before'])
        actor.mutate = lambda *_args: (_ for _ in ()).throw(RuntimeError('interrupted'))
        with tempfile.TemporaryDirectory() as tmp:
            load_plan(self.input, expected=self.ctx, state_root=Path(tmp))
            intent = {'kind': 'nats_contract_intent', 'object': action['object'], 'api': action['api'],
                'before_sha256': digest(action['before']), 'target_sha256': digest(action['after']),
                'migration_attempt': self.ctx['contract_attempt']}
            issued = {'kind': 'nats_contract_mutation_issued', 'object': action['object'],
                'api': action['api'], 'target_sha256': digest(action['after']),
                'migration_attempt': self.ctx['contract_attempt']}
            binding = {**self.ctx, 'plan_sha256': digest(self.plan)}
            with self.assertRaises(RuntimeError):
                execute_action(self.plan, 0, binding=binding,
                    before_event_sha256=hashlib_sha(intent), issued_event_sha256=hashlib_sha(issued),
                    state_root=Path(tmp), actor=actor)
            with self.assertRaises(ContractRunnerError):
                action_readback(state_root=Path(tmp), binding=binding, index=0)
            with self.assertRaises(ContractRunnerError):
                execute_action(self.plan, 0, binding=binding,
                    before_event_sha256=hashlib_sha(intent), issued_event_sha256=hashlib_sha(issued),
                    state_root=Path(tmp), actor=actor)
            self.assertEqual(actor.mutations, [])

    def test_actor_uses_fixed_cli_without_shell_or_secret_payload(self):
        captured = []
        class Result:
            returncode = 0
            stdout = b'{"config":{"name":"chat_events"}}'
        actor = NatsActor(isolation_check=lambda: None,
            run=lambda command, **kwargs: (captured.append((command, kwargs)) or Result()))
        # Avoid contacting a real supervisor in this unit test.
        import nats_contract_runner
        original = nats_contract_runner._supervisor
        nats_contract_runner._supervisor = lambda _action: {
            'schema': 'voice-nats-supervisor-v1', 'ok': True, 'running': True}
        try:
            self.assertEqual(actor.info_config('chat_events'), {'name': 'chat_events'})
        finally:
            nats_contract_runner._supervisor = original
        command, kwargs = captured[0]
        self.assertNotIn('/bin/sh', command)
        self.assertEqual(command[-2], '$JS.API.STREAM.INFO.chat_events')
        self.assertEqual(kwargs['stderr'], -3)
        self.assertEqual(command[command.index('--creds') + 1], '/inputs/private/bootstrap.creds')


def hashlib_sha(value):
    import hashlib
    return hashlib.sha256(_canonical(value)).hexdigest()


if __name__ == '__main__':
    unittest.main()
