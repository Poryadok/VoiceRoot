import copy
import base64
import hashlib
import json
import os
import tempfile
import unittest
from pathlib import Path
from contextlib import redirect_stderr
from io import StringIO
from unittest.mock import patch

from nats_contract_plan import digest
from nats_contract_plan_test import Tests as PlanFixtures
from nats_contract_runner import (SCHEMA, INPUT_SCHEMA, ContractRunnerError, NatsActor,
    _canonical, _ensure_input_profile, _verify_input_receipt, action_readback, execute_action,
    inspect_action, load_plan, read_plan, main,
    validate_envelope, validate_plan)


def context(plan):
    row = {
        'operation': '012345abcdef',
        'nonce': '0123456789abcdef0123456789abcdef',
        'pod_uid': '12345678-1234-1234-1234-123456789abc',
        'contract_attempt': 'abcdef12',
        'source_pvc_uid': '22345678-1234-1234-1234-123456789abc',
        'source_pv_uid': '32345678-1234-1234-1234-123456789abc',
        'verification_pvc_uid': '42345678-1234-1234-1234-123456789abc',
        'verification_pv_uid': '52345678-1234-1234-1234-123456789abc',
        'archive_sha256': 'a' * 64, 'archive_bytes': 4096,
        'server_config_sha256': hashlib.sha256(b'captured public config').hexdigest(),
        'server_config_bytes': len(b'captured public config'),
        'bootstrap_creds_sha256': hashlib.sha256(b'captured client creds').hexdigest(),
        'bootstrap_creds_bytes': len(b'captured client creds'),
        'plan_sha256': digest(plan), 'plan_bytes': len(_canonical(plan)),
    }
    raw = _canonical(envelope(plan))
    row.update(input_bundle_sha256=hashlib.sha256(raw).hexdigest(), input_bundle_bytes=len(raw))
    return row


def envelope(plan):
    return {'schema': INPUT_SCHEMA, 'operation': '012345abcdef',
        'nonce': '0123456789abcdef0123456789abcdef', 'contract_attempt': 'abcdef12',
        'source_pvc_uid': '22345678-1234-1234-1234-123456789abc',
        'source_pv_uid': '32345678-1234-1234-1234-123456789abc',
        'verification_pvc_uid': '42345678-1234-1234-1234-123456789abc',
        'verification_pv_uid': '52345678-1234-1234-1234-123456789abc',
        'archive_sha256': 'a' * 64, 'archive_bytes': 4096,
        'server_config_sha256': hashlib.sha256(b'captured public config').hexdigest(),
        'plan_sha256': digest(plan), 'plan': plan,
        'files': {'bootstrap.creds': base64.b64encode(b'captured client creds').decode('ascii'),
            'server.conf': base64.b64encode(b'captured public config').decode('ascii')}}


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
        self.raw = _canonical(self.input)
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.state_root = Path(self.tmp.name)
        self.state_root.chmod(0o777)
        self.config_root = self.state_root / 'broker-config'
        self.config_root.mkdir(mode=0o777)
        import nats_contract_runner as module
        self.config_patch = patch.object(module, 'SERVER_CONFIG', self.config_root / 'server.conf')
        self.config_patch.start()
        self.addCleanup(self.config_patch.stop)

    def test_envelope_requires_exact_context_canonical_bytes_and_plan_digest(self):
        self.assertEqual(validate_envelope(self.raw, expected=self.ctx)['plan'], self.plan)
        with self.assertRaises(ContractRunnerError):
            validate_envelope(self.raw + b' ', expected=self.ctx)
        with self.assertRaises(ContractRunnerError):
            validate_envelope(self.raw, expected={**self.ctx, 'plan_sha256': 'b' * 64})
        changed = copy.deepcopy(self.input)
        changed['plan']['actions'][0]['after']['max_msgs'] = 10
        with self.assertRaises(ContractRunnerError):
            validate_envelope(_canonical(changed), expected=self.ctx)

    def test_duplicate_keys_extra_fields_and_arbitrary_api_are_rejected(self):
        duplicate = self.raw[:-1] + b',"schema":"voice-nats-contract-inputs-v1"}'
        with self.assertRaises(ContractRunnerError):
            validate_envelope(duplicate, expected=self.ctx)
        bad = copy.deepcopy(self.input); bad['command'] = ['/bin/sh']
        with self.assertRaises(ContractRunnerError):
            validate_envelope(_canonical(bad), expected=self.ctx)
        bad = copy.deepcopy(self.input); bad['plan']['actions'][0]['api'] = '$JS.API.STREAM.DELETE.chat_events'
        bad['plan_sha256'] = digest(bad['plan'])
        with self.assertRaises(ContractRunnerError):
            validate_envelope(_canonical(bad), expected=self.ctx)
        bad = copy.deepcopy(self.input); bad['files']['client.key'] = 'cHJpdmF0ZQ=='
        with self.assertRaises(ContractRunnerError):
            validate_envelope(_canonical(bad), expected=self.ctx)

    def test_load_plan_receipt_binds_same_pod_and_exact_two_files(self):
        receipt = load_plan(self.raw, expected=self.ctx, state_root=self.state_root)
        self.assertEqual(receipt['schema'], 'voice-nats-contract-input-receipt-v1')
        self.assertEqual(receipt['status'], 'COMPLETE')
        self.assertEqual(receipt['file_count'], 2)
        directory = self.state_root / self.ctx['contract_attempt']
        self.assertEqual((directory.stat().st_uid, directory.stat().st_gid,
            directory.stat().st_mode & 0o777), (os.geteuid(), os.getegid(), 0o700))
        cred = directory / 'bootstrap.creds'
        self.assertEqual(cred.read_bytes(), b'captured client creds')
        self.assertEqual(cred.stat().st_mode & 0o777, 0o400)
        config = self.config_root / 'server.conf'
        self.assertEqual(config.read_bytes(), b'captured public config')
        self.assertEqual(config.stat().st_mode & 0o777, 0o440)
        self.assertEqual(_verify_input_receipt(self.ctx, state_root=self.state_root), receipt)
        self.assertEqual(read_plan(state_root=self.state_root, binding=self.ctx), receipt)
        with self.assertRaises(ContractRunnerError):
            load_plan(self.raw, expected=self.ctx, state_root=self.state_root)

    def test_bad_pod_state_root_or_capture_binding_fails_before_filesystem_write(self):
        wrong = {**self.ctx, 'server_config_sha256': 'b' * 64}
        with self.assertRaises(ContractRunnerError):
            load_plan(self.raw, expected=wrong, state_root=self.state_root)
        self.assertFalse((self.state_root / self.ctx['contract_attempt']).exists())
        wrong_root = self.state_root / 'wrong-root'
        wrong_root.mkdir(mode=0o700)
        with self.assertRaises(ContractRunnerError):
            load_plan(self.raw, expected=self.ctx, state_root=wrong_root)
        linked_root = self.state_root / 'linked-root'
        linked_root.symlink_to(self.state_root, target_is_directory=True)
        with self.assertRaises(ContractRunnerError):
            load_plan(self.raw, expected=self.ctx, state_root=linked_root)
        self.assertFalse((self.state_root / self.ctx['contract_attempt']).exists())

    def test_emptydir_profile_accepts_kubelet_mode_without_root_identity(self):
        # A non-root helper may create its exclusive attempt directory below
        # kubelet's 0777 emptyDir root; no UID0 init or fsGroup is needed.
        self.assertEqual(load_plan(self.raw, expected=self.ctx,
            state_root=self.state_root)['status'], 'COMPLETE')
        expected_bytes = {**self.ctx, 'server_config_bytes': self.ctx['server_config_bytes'] + 1}
        with self.assertRaises(ContractRunnerError):
            _verify_input_receipt(expected_bytes, state_root=self.state_root)

    def test_new_pod_uid_cannot_read_or_replay_existing_attempt(self):
        load_plan(self.raw, expected=self.ctx, state_root=self.state_root)
        wrong = {**self.ctx, 'pod_uid': '62345678-1234-1234-1234-123456789abc'}
        with self.assertRaises(ContractRunnerError):
            _verify_input_receipt(wrong, state_root=self.state_root)
        with self.assertRaises(ContractRunnerError):
            load_plan(self.raw, expected=wrong, state_root=self.state_root)

    def test_action_is_one_exact_mutation_after_host_journal_and_same_attempt_receipt(self):
        action_index = 0
        action = self.plan['actions'][action_index]
        actor = FakeActor(isolation_check=lambda: None)
        actor.current[action['object']] = copy.deepcopy(action['before'])
        load_plan(self.raw, expected=self.ctx, state_root=self.state_root)
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
                issued_event_sha256=hashlib_sha(issued), state_root=self.state_root, actor=actor)
            self.assertEqual(receipt['status'], 'COMPLETE')
            self.assertEqual(actor.mutations, [action['api']])
            self.assertEqual(action_readback(state_root=self.state_root, binding={**self.ctx,
                'plan_sha256': digest(self.plan)}, index=action_index), receipt)

    def test_interrupted_action_is_unknown_and_cannot_be_replayed(self):
        action = self.plan['actions'][0]
        actor = FakeActor(isolation_check=lambda: None)
        actor.current[action['object']] = copy.deepcopy(action['before'])
        actor.mutate = lambda *_args: (_ for _ in ()).throw(RuntimeError('interrupted'))
        load_plan(self.raw, expected=self.ctx, state_root=self.state_root)
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
                    state_root=self.state_root, actor=actor)
            with self.assertRaises(ContractRunnerError):
                action_readback(state_root=self.state_root, binding=binding, index=0)
            with self.assertRaises(ContractRunnerError):
                execute_action(self.plan, 0, binding=binding,
                    before_event_sha256=hashlib_sha(intent), issued_event_sha256=hashlib_sha(issued),
                    state_root=self.state_root, actor=actor)
            self.assertEqual(actor.mutations, [])

    def test_actor_uses_fixed_cli_without_shell_or_secret_payload(self):
        captured = []
        class Result:
            returncode = 0
            stdout = b'{"config":{"name":"chat_events"}}'
        actor = NatsActor(isolation_check=lambda: None,
            creds_path=Path('/state/abcdef12/bootstrap.creds'),
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
        self.assertEqual(command[command.index('--creds') + 1], '/state/abcdef12/bootstrap.creds')

    def test_input_profile_detects_config_or_credential_drift(self):
        load_plan(self.raw, expected=self.ctx, state_root=self.state_root)
        cred = self.state_root / self.ctx['contract_attempt'] / 'bootstrap.creds'
        cred.chmod(0o600)
        cred.write_bytes(b'changed')
        with self.assertRaises(ContractRunnerError):
            _ensure_input_profile(self.ctx, state_root=self.state_root)

    def test_actor_isolation_requires_running_contract_supervisor(self):
        actor = NatsActor(isolation_check=lambda: None, creds_path=Path('/state/x/bootstrap.creds'))
        running = {'schema': 'voice-nats-supervisor-v1', 'ok': True, 'running': True,
            'pid': 123, 'started_at_ns': 456}
        with patch('nats_contract_runner._contract_supervisor', return_value=running):
            actor.assert_closed_isolation()
        with patch('nats_contract_runner._contract_supervisor', return_value={**running, 'running': False}):
            with self.assertRaises(ContractRunnerError):
                actor.assert_closed_isolation()
        bound = NatsActor(isolation_check=lambda: None, creds_path=Path('/state/x/bootstrap.creds'),
            expected_broker=lambda: {'pid': 123, 'started_at_ns': 456})
        with patch('nats_contract_runner._contract_supervisor', return_value=running):
            bound.assert_closed_isolation()
        with patch('nats_contract_runner._contract_supervisor', return_value={**running, 'pid': 124}):
            with self.assertRaises(ContractRunnerError):
                bound.assert_closed_isolation()

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
            '--input-bundle-sha256', self.ctx['input_bundle_sha256'],
            '--input-bundle-bytes', str(self.ctx['input_bundle_bytes']),
            '--archive-sha256', self.ctx['archive_sha256'], '--archive-bytes', str(self.ctx['archive_bytes']),
            '--server-config-sha256', self.ctx['server_config_sha256'],
            '--server-config-bytes', str(self.ctx['server_config_bytes']),
            '--bootstrap-creds-sha256', self.ctx['bootstrap_creds_sha256'],
            '--bootstrap-creds-bytes', str(self.ctx['bootstrap_creds_bytes']),
            '--plan-sha256', digest(self.plan), '--plan-bytes', str(self.ctx['plan_bytes']),
            '--action-index', '0']
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
