import tempfile
from pathlib import Path
import unittest
import contextlib
import json
import os
from unittest.mock import patch
import bridge

class ReplayTests(unittest.TestCase):
    def test_completed_request_replays_without_second_mutation(self):
        with tempfile.TemporaryDirectory() as directory:
            calls=[]; request={'action':'finish','operation':'a'*12,'claim_rv':'17','nonce':'b'*64}
            execute=lambda row: calls.append(row) or {'status':'PASS','operation':'a'*12}
            first=bridge.dispatch(Path(directory),request,execute,lambda row:None)
            self.assertEqual(first,bridge.dispatch(Path(directory),request,execute,lambda row:None))
            self.assertEqual(len(calls),1)

    def test_crash_after_pass_recovers_checkpoint_without_finish_again(self):
        with tempfile.TemporaryDirectory() as directory:
            request={'action':'finish','operation':'a'*12,'claim_rv':'17','nonce':'b'*64}
            def crash(row):raise KeyboardInterrupt()
            with self.assertRaises(KeyboardInterrupt):bridge.dispatch(Path(directory),request,crash,lambda row:None)
            result=bridge.dispatch(Path(directory),request,lambda row:self.fail('finish rerun'),lambda row:{'status':'PASS','operation':'a'*12})
            self.assertEqual(result['status'],'PASS')

    def test_changed_nonce_body_and_interrupted_unknown_never_execute(self):
        with tempfile.TemporaryDirectory() as directory:
            request={'action':'finish','operation':'a'*12,'claim_rv':'17','nonce':'b'*64}
            def crash(row):raise KeyboardInterrupt()
            with self.assertRaises(KeyboardInterrupt):bridge.dispatch(Path(directory),request,crash,lambda row:None)
            with self.assertRaises(bridge.BridgeError):bridge.dispatch(Path(directory),dict(request,claim_rv='18'),lambda row:self.fail(),lambda row:None)
            with self.assertRaises(bridge.BridgeError):bridge.dispatch(Path(directory),request,lambda row:self.fail(),lambda row:None)

    def test_fixed_action_schema_rejects_extra_path_and_command(self):
        for row in ({'nonce':'b'*64,'action':'shell','command':'id'}, {'nonce':'b'*64,'action':'status','operation':'a'*12,'path':'/tmp/x'}):
            with self.assertRaises(bridge.BridgeError):bridge.validate(row)

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'root-owned disposable Linux fixture')
    def test_actual_inbox_claim_copies_runner_inode_and_delivers_private_response(self):
        import root_main
        import bridge_root
        with tempfile.TemporaryDirectory() as directory:
            installed=Path(directory)
            for name in ('inbox','processing','journal','responses'):(installed/name).mkdir(mode=0o700)
            nonce='d'*64;request={'action':'status','operation':'a'*12,'nonce':nonce}
            inbox=installed/'inbox'/(nonce+'.json');inbox.write_text(json.dumps(request));inbox.chmod(0o600);os.chown(inbox,1000,1000)
            old_inode=inbox.stat().st_ino
            class FakeActions:
                def __init__(self,code):pass
                def execute(self,row):
                    claimed=installed/'processing'/(nonce+'.json')
                    self_test.assertNotEqual(claimed.stat().st_ino,old_inode)
                    self_test.assertEqual(claimed.stat().st_uid,0)
                    return {'status':'PASS','operation':row['operation']}
                def recover(self,row):return None
            self_test=self
            with patch.object(root_main,'operation_lock',contextlib.nullcontext),patch.object(bridge_root,'Actions',FakeActions),patch('grp.getgrnam',return_value=type('G',(),{'gr_gid':65532})()):bridge.drain(installed)
            response=installed/'responses'/(nonce+'.json')
            self.assertEqual(json.loads(response.read_text())['status'],'PASS')
            self.assertEqual(response.stat().st_mode&0o777,0o440)
            self.assertFalse(inbox.exists());self.assertEqual(list((installed/'processing').iterdir()),[])
