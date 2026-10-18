"""The production bridge status caller reaches only runner-owned read state."""
import json
import os
from pathlib import Path
import socket
import sys
import tempfile
import threading
import types
import unittest
from unittest.mock import patch

import bridge_state
import runner_actions
import runner_rpc as rpc


OPERATION = 'a' * 12
NONCE = 'b' * 64
ENV = {'GITHUB_RUN_ATTEMPT':'1','GITHUB_RUN_ID':'42','GITHUB_SHA':'c'*40,'GITHUB_JOB':'status','GITHUB_TOKEN':'secret'}


def bridge_client_module():
    report=types.ModuleType('deployment_report')
    report.code=lambda value: value if isinstance(value,str) and value.isascii() and value.replace('_','').isalnum() else 'blocked_unclassified'
    errors=types.ModuleType('errors')
    errors.safe_error=lambda error: 'blocked_unclassified'
    sys.modules.setdefault('deployment_report',report)
    sys.modules.setdefault('errors',errors)
    import bridge_client
    return bridge_client


def checkpoint_tree(parent, *, operation=OPERATION, status='CONTINUING', error=None):
    operations = Path(parent) / 'operations'
    operations.mkdir(mode=0o700)
    base = operations / ('rollout-' + operation)
    base.mkdir(mode=0o700)
    path = base / 'checkpoint.json'
    path.write_text(json.dumps({'operation':operation,'phase':'NATS_CENSUS',
        'status':status,'fence_status':'VERIFIED','error':error},sort_keys=True))
    path.chmod(0o600)
    return operations


class RunnerRpcTests(unittest.TestCase):
    def test_request_is_exact_status_only_contract(self):
        request = {'schema':rpc.SCHEMA,'protocol':1,'action':'status','operation':OPERATION,'nonce':NONCE}
        self.assertEqual(rpc.validate(request), {'action':'status','operation':OPERATION,'nonce':NONCE})
        for update in ({'action':'nats-phase','phase':'drain'}, {'action':'prepare'},
            {'path':'/etc/shadow'}, {'token':'secret'}, {'nonce':'b'*32}):
            with self.subTest(update=update), self.assertRaises(rpc.RpcError):
                rpc.validate(request | update)

    def test_candidate_cannot_choose_checkpoint_path_or_change_status(self):
        with tempfile.TemporaryDirectory() as root:
            operations = checkpoint_tree(root)
            engine = bridge_state.BridgeState(operations,owner_uid=os.geteuid())
            runner = rpc.Runner(runner_actions.Actions(engine))
            response = runner.handle({'schema':rpc.SCHEMA,'protocol':1,'action':'status',
                'operation':OPERATION,'nonce':NONCE})
            self.assertEqual(response['result'], {'operation':OPERATION,'phase':'NATS_CENSUS',
                'status':'CONTINUING','fence_status':'VERIFIED'})
            self.assertEqual(sorted(p.name for p in Path(root).rglob('*') if p.is_file()),['checkpoint.json'])
            with self.assertRaisesRegex(rpc.RpcError,'runner_request_invalid'):
                runner.handle({'schema':rpc.SCHEMA,'protocol':1,'action':'status',
                    'operation':OPERATION,'nonce':NONCE,'checkpoint':'/etc/shadow'})

    def test_status_checks_operation_custody_and_redacts_untrusted_error_text(self):
        with tempfile.TemporaryDirectory() as root:
            operations = checkpoint_tree(root,error='token=secret')
            state = bridge_state.BridgeState(operations,owner_uid=os.geteuid())
            self.assertEqual(state.status(OPERATION)['error'],'redacted')
            with self.assertRaisesRegex(bridge_state.StateError,'checkpoint_unavailable'):
                bridge_state._read_checkpoint(operations / ('rollout-'+'d'*12) / 'checkpoint.json',
                    OPERATION,owner_uid=os.geteuid())

    def test_real_bridge_status_submit_uses_socket_and_not_root_inbox(self):
        with tempfile.TemporaryDirectory() as root:
            operations = checkpoint_tree(root)
            runner = rpc.Runner(runner_actions.Actions(bridge_state.BridgeState(operations,owner_uid=os.geteuid())))
            socket_path = str(Path(root)/'runner.sock')
            listener = socket.socket(socket.AF_UNIX,socket.SOCK_STREAM)
            listener.bind(socket_path);listener.listen(1)
            done = threading.Event()
            def serve():
                connection,_=listener.accept()
                rpc.serve_connection(runner,connection)
                done.set()
            thread=threading.Thread(target=serve);thread.start()
            try:
                bridge_client=bridge_client_module()
                request=bridge_client.request(['status',OPERATION],ENV)
                with patch.object(bridge_client,'RUNNER_SOCKET',Path(socket_path)), \
                    patch.object(rpc,'SOCKET_OWNER_UID',os.geteuid()):
                    result=bridge_client.submit(request,installed=Path(root)/'must-not-be-read',timeout=2)
                self.assertEqual(result['status'],'CONTINUING')
                self.assertEqual(result['phase'],'NATS_CENSUS')
                self.assertFalse((Path(root)/'must-not-be-read').exists())
                self.assertTrue(done.wait(2))
            finally:
                listener.close();thread.join(timeout=2)
            self.assertFalse(thread.is_alive())

    def test_mutating_action_is_rejected_without_socket_or_file_fallback(self):
        bridge_client=bridge_client_module()
        request=bridge_client.request(['prepare','full','messaging','c'*40,'42'],ENV)
        with tempfile.TemporaryDirectory() as root:
            with patch.object(bridge_client,'RUNNER_SOCKET',Path(root)/'absent.sock'):
                with self.assertRaisesRegex(bridge_client.bridge.BridgeError,'bridge_runner_action_not_migrated'):
                    bridge_client.submit(request,installed=Path(root)/'inbox',timeout=1)
            self.assertFalse((Path(root)/'inbox').exists())


if __name__ == '__main__': unittest.main()
