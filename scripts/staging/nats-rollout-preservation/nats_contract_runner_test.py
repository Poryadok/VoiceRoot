import copy
import json
import tempfile
import unittest
from pathlib import Path
from contextlib import redirect_stderr
from io import StringIO
from unittest.mock import patch

from nats_contract_plan import digest
from nats_contract_plan_test import Tests as PlanFixtures
from nats_contract_runner import (SCHEMA, ContractRunnerError, NatsActor,
    _canonical, _ensure_input_profile, _verify_input_receipt, action_readback, execute_action,
    inspect_action, load_plan, main,
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
        'input_pod_uid': '62345678-1234-1234-1234-123456789abc',
        'input_attempt': 'abcde123', 'archive_sha256': 'a' * 64, 'archive_bytes': 4096,
        'input_bundle_sha256': '7' * 64,
        'input_bundle_bytes': 1024,
        'kernel_bundle_sha256': '8' * 64,
        'server_config_sha256': '9' * 64,
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
            state = Path(tmp) / self.ctx['operation']; state.mkdir()
            load_plan(self.input, expected=self.ctx, state_root=state)
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
                issued_event_sha256=hashlib_sha(issued), state_root=state, actor=actor)
            self.assertEqual(receipt['status'], 'COMPLETE')
            self.assertEqual(actor.mutations, [action['api']])
            self.assertEqual(action_readback(state_root=state, binding={**self.ctx,
                'plan_sha256': digest(self.plan)}, index=action_index), receipt)

    def test_interrupted_action_is_unknown_and_cannot_be_replayed(self):
        action = self.plan['actions'][0]
        actor = FakeActor(isolation_check=lambda: None)
        actor.current[action['object']] = copy.deepcopy(action['before'])
        actor.mutate = lambda *_args: (_ for _ in ()).throw(RuntimeError('interrupted'))
        with tempfile.TemporaryDirectory() as tmp:
            state = Path(tmp) / self.ctx['operation']; state.mkdir()
            load_plan(self.input, expected=self.ctx, state_root=state)
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
                    state_root=state, actor=actor)
            with self.assertRaises(ContractRunnerError):
                action_readback(state_root=state, binding=binding, index=0)
            with self.assertRaises(ContractRunnerError):
                execute_action(self.plan, 0, binding=binding,
                    before_event_sha256=hashlib_sha(intent), issued_event_sha256=hashlib_sha(issued),
                    state_root=state, actor=actor)
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
            'schema': 'voice-nats-supervisor-v1', 'ok': True, 'running': True,
            'pid': 123, 'started_at_ns': 456}
        try:
            self.assertEqual(actor.info_config('chat_events'), {'name': 'chat_events'})
        finally:
            nats_contract_runner._supervisor = original
        command, kwargs = captured[0]
        self.assertNotIn('/bin/sh', command)
        self.assertEqual(command[-2], '$JS.API.STREAM.INFO.chat_events')
        self.assertEqual(kwargs['stderr'], -3)
        self.assertEqual(command[command.index('--creds') + 1], '/inputs/private/bootstrap.creds')

    def test_input_profile_requires_exact_files_and_bound_digests(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rows = {'bootstrap.creds': b'boot', 'social.creds': b'social',
                'realtime.creds': b'realtime', 'fixture.json': b'{}'}
            for name, raw in rows.items():
                path = root / name
                path.write_bytes(raw)
                path.chmod(0o440)
            import hashlib
            bundle_hash = hashlib.sha256(json.dumps(
                {name: hashlib.sha256(rows[name]).hexdigest()
                 for name in sorted(set(rows) - {'server.conf'})},
                sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            expected = {'kernel_bundle_sha256': bundle_hash,
                'server_config_sha256': 'a' * 64}
            import nats_contract_runner as module
            with patch.object(module, 'INPUT_ROOT', root), patch.object(module, 'CREDS', root / 'bootstrap.creds'):
                _ensure_input_profile(expected)
                (root / 'client.key').write_bytes(b'private')
                with self.assertRaises(ContractRunnerError):
                    _ensure_input_profile(expected)

    def test_actor_isolation_requires_running_contract_supervisor(self):
        actor = NatsActor(isolation_check=lambda: None)
        running = {'schema': 'voice-nats-supervisor-v1', 'ok': True, 'running': True,
            'pid': 123, 'started_at_ns': 456}
        with patch('nats_contract_runner._contract_supervisor', return_value=running):
            actor.assert_closed_isolation()
        with patch('nats_contract_runner._contract_supervisor', return_value={**running, 'running': False}):
            with self.assertRaises(ContractRunnerError):
                actor.assert_closed_isolation()
        bound = NatsActor(isolation_check=lambda: None,
            expected_broker=lambda: {'pid': 123, 'started_at_ns': 456})
        with patch('nats_contract_runner._contract_supervisor', return_value=running):
            bound.assert_closed_isolation()
        with patch('nats_contract_runner._contract_supervisor', return_value={**running, 'pid': 124}):
            with self.assertRaises(ContractRunnerError):
                bound.assert_closed_isolation()

    def test_input_receipt_is_bound_to_exact_loader_and_archive(self):
        with tempfile.TemporaryDirectory() as tmp:
            state = Path(tmp) / self.ctx['input_attempt']
            state.mkdir(mode=0o700)
            receipt = {'schema': 'voice-nats-input-receipt-v1', 'phase': 'inputs',
                'purpose': 'verification', 'operation': self.ctx['operation'],
                'nonce': self.ctx['nonce'], 'pod_uid': self.ctx['input_pod_uid'],
                'source_pvc_uid': self.ctx['source_pvc_uid'], 'source_pv_uid': self.ctx['source_pv_uid'],
                'verification_pvc_uid': self.ctx['verification_pvc_uid'],
                'verification_pv_uid': self.ctx['verification_pv_uid'],
                'archive_sha256': self.ctx['archive_sha256'], 'archive_bytes': self.ctx['archive_bytes'],
                'session_attempt': self.ctx['input_attempt'],
                'input_bundle_sha256': self.ctx['input_bundle_sha256'],
                'input_bundle_bytes': self.ctx['input_bundle_bytes'], 'status': 'COMPLETE',
                'file_count': 5, 'kernel_bundle_sha256': self.ctx['kernel_bundle_sha256'],
                'server_config_sha256': self.ctx['server_config_sha256']}
            (state / 'inputs.receipt.json').write_bytes(_canonical(receipt))
            (state / 'inputs.receipt.json').chmod(0o600)
            self.assertEqual(_verify_input_receipt(self.ctx, state_root=Path(tmp)), receipt)
            receipt['archive_sha256'] = 'b' * 64
            (state / 'inputs.receipt.json').write_bytes(_canonical(receipt))
            with self.assertRaises(ContractRunnerError):
                _verify_input_receipt(self.ctx, state_root=Path(tmp))

    def test_cli_rejects_arbitrary_command_without_echoing_arguments(self):
        stderr = StringIO()
        with redirect_stderr(stderr):
            result = main(['--load-plan', '--operation', '012345abcdef', '--command', '/bin/sh'])
        self.assertEqual(result, 2)
        self.assertEqual(stderr.getvalue(), 'contract_runner_failed\n')

    def test_cli_binds_request_to_downward_api_pod_uid_before_state_access(self):
        args = ['--readback-action', '--operation', self.ctx['operation'], '--nonce', self.ctx['nonce'],
            '--pod-uid', self.ctx['pod_uid'], '--contract-attempt', self.ctx['contract_attempt'],
            '--source-pvc-uid', self.ctx['source_pvc_uid'], '--source-pv-uid', self.ctx['source_pv_uid'],
            '--verification-pvc-uid', self.ctx['verification_pvc_uid'],
            '--verification-pv-uid', self.ctx['verification_pv_uid'],
            '--input-pod-uid', self.ctx['input_pod_uid'], '--input-attempt', self.ctx['input_attempt'],
            '--input-bundle-sha256', self.ctx['input_bundle_sha256'],
            '--input-bundle-bytes', str(self.ctx['input_bundle_bytes']),
            '--archive-sha256', self.ctx['archive_sha256'], '--archive-bytes', str(self.ctx['archive_bytes']),
            '--kernel-bundle-sha256', self.ctx['kernel_bundle_sha256'],
            '--server-config-sha256', self.ctx['server_config_sha256'],
            '--plan-sha256', digest(self.plan), '--plan-bytes', '100', '--action-index', '0']
        stderr = StringIO()
        with patch.dict('os.environ', {'POD_UID': '72345678-1234-1234-1234-123456789abc'}), \
                redirect_stderr(stderr):
            result = main(args)
        self.assertEqual(result, 2)
        self.assertEqual(stderr.getvalue(), 'contract_runner_failed\n')


def hashlib_sha(value):
    import hashlib
    return hashlib.sha256(_canonical(value)).hexdigest()


if __name__ == '__main__':
    unittest.main()
