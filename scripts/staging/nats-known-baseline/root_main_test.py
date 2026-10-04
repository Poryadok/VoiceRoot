import datetime as dt
import unittest
import tempfile
import json
from pathlib import Path
from unittest.mock import patch

from controller import Blocked
import root_main
from controller import archive_closed_store


class ResumeGuardTests(unittest.TestCase):
    def test_second_operator_never_enters_locked_operation(self):
        with tempfile.TemporaryDirectory() as d,patch('root_main.ROOT',Path(d)):
            with root_main.operation_lock():
                with self.assertRaisesRegex(Blocked,'operation_already_running'):
                    with root_main.operation_lock():self.fail('concurrent operation entered')

    def test_failed_refence_never_claims_verified_fence(self):
        class Stage:
            operation='abcd1234'
            marker={'data':{'knownBaselineOperation':'abcd1234'}}
            def refence(self):raise Blocked('ownership_lost')
        state={'stage_fenced':True,'fence_status':'VERIFIED'}
        root_main.observe_refence(state,Stage())
        self.assertFalse(state['stage_fenced'])
        self.assertEqual(state['fence_status'],'UNKNOWN')
        self.assertFalse(state['refence']['verified'])

    def test_changed_captured_code_is_rejected(self):
        with patch('root_main.public_read',return_value=b'{"files":{"kernel":"new"}}'):
            with self.assertRaisesRegex(Blocked,'checkpoint_code_capture_changed'):
                root_main.bind_code(Path('/captured'),{'code_capture':{'files':{'kernel':'old'}}})

    def test_modified_closed_final_store_rejects_before_restart(self):
        with tempfile.TemporaryDirectory() as d:
            base=Path(d);store=base/'store';store.mkdir();(store/'blk').write_bytes(b'old')
            manifest=archive_closed_store(store,base/'closed.tar')
            path=base/'manifest.json';path.write_text(json.dumps(manifest))
            (store/'blk').write_bytes(b'changed')
            class Stage:
                final_path=store
                def verify_final_storage(self):pass
            with self.assertRaisesRegex(Blocked,'closed_final_store_changed'):
                root_main.verify_store(base,{'staging_backup':{'manifest':str(path)}},Stage())
            self.assertEqual(list(base.glob('store-verification-*')),[])

    def state(self):
        return {'status':'AWAITING_OFF_NODE_COPY',
            'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat(),
            'fixture_backup':{'archive_sha256':'a'*64},
            'staging_backup':{'archive_sha256':'b'*64}}

    def test_wrong_off_node_hash_never_opens_kubernetes(self):
        with patch('root_main.private_json',return_value=self.state()),patch('root_main.Kube') as kube:
            with self.assertRaises(Blocked):
                root_main.resume(Path('/captured-code'),root_main.ROOT/'known-baseline-abcd1234abcd',['c'*64,'b'*64])
            kube.assert_not_called()

    def test_expired_checkpoint_never_restarts(self):
        state=self.state();state['expires_at']=(dt.datetime.now(dt.timezone.utc)-dt.timedelta(seconds=1)).isoformat()
        with patch('root_main.private_json',return_value=state),patch('root_main.Kube') as kube:
            with self.assertRaises(Blocked):
                root_main.resume(Path('/captured-code'),root_main.ROOT/'known-baseline-abcd1234abcd',['a'*64,'b'*64])
            kube.assert_not_called()

    def test_credentials_rotation_rejects_before_script_reuse(self):
        class Kube:
            def get(self,kind,name):return {'metadata':{'uid':'current','resourceVersion':'11'}}
            def secret_meta(self,name):return self.get('secret',name)['metadata']
        with self.assertRaises(Blocked):
            root_main.revalidate_inputs(Kube(),{'scripts':[]},
                {'social.creds':{'resource':'known','uid':'original','resource_version':'10'}})


if __name__=='__main__':unittest.main()
