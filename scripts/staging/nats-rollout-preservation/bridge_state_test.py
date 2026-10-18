"""Runner state is read-only, narrowly projected and custodian checked."""
import json
import os
from pathlib import Path
import tempfile
import unittest

import bridge_state


OP = 'a' * 12


class BridgeStateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.operations = Path(self.temp.name) / 'operations'
        self.operations.mkdir(mode=0o700)
        self.operation = self.operations / ('rollout-' + OP)
        self.operation.mkdir(mode=0o700)
        self.checkpoint = self.operation / 'checkpoint.json'
        self.write({'operation':OP,'phase':'PAUSED_APPLY','status':'BLOCKED',
            'fence_status':'VERIFIED','error':'custody_pending','secret':'do-not-return'})

    def tearDown(self): self.temp.cleanup()

    def write(self, row):
        self.checkpoint.write_text(json.dumps(row,sort_keys=True));self.checkpoint.chmod(0o600)

    def test_status_reads_exact_operation_and_projects_only_safe_summary(self):
        before=self.checkpoint.read_bytes()
        state=bridge_state.BridgeState(self.operations,owner_uid=os.geteuid())
        self.assertEqual(state.status(OP),{'operation':OP,'phase':'PAUSED_APPLY',
            'status':'BLOCKED','fence_status':'VERIFIED','error':'custody_pending'})
        self.assertEqual(self.checkpoint.read_bytes(),before)

    def test_wrong_operation_duplicate_keys_symlinks_and_mutable_checkpoint_fail_closed(self):
        state=bridge_state.BridgeState(self.operations,owner_uid=os.geteuid())
        with self.assertRaisesRegex(bridge_state.StateError,'operation_unavailable'):
            state.status('b'*12)
        self.checkpoint.write_text('{"operation":"'+OP+'","operation":"'+'b'*12+'"}')
        self.checkpoint.chmod(0o600)
        with self.assertRaisesRegex(bridge_state.StateError,'duplicate_field'):
            state.status(OP)
        self.checkpoint.unlink();self.checkpoint.symlink_to(self.operation/'missing')
        with self.assertRaisesRegex(bridge_state.StateError,'checkpoint_unavailable'):
            state.status(OP)

    def test_errors_are_sanitized_and_invalid_summary_does_not_escape(self):
        state=bridge_state.BridgeState(self.operations,owner_uid=os.geteuid())
        self.write({'operation':OP,'phase':'NATS_CENSUS','status':'RUNNING',
            'fence_status':'VERIFIED','error':'token=secret'})
        self.assertEqual(state.status(OP)['error'],'redacted')
        self.write({'operation':OP,'phase':'/tmp/secret','status':'RUNNING'})
        with self.assertRaisesRegex(bridge_state.StateError,'summary_field_invalid'):
            state.status(OP)


if __name__ == '__main__': unittest.main()
