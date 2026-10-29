import copy
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import media_backup_upload_phase
import space_native_tool_identity as tool
import space_native_tool_identity_custody as custody
import publish_candidate
import full_cohort_predecessor_phase as predecessor
from space_native_tool_identity_phase_test import ToolPhaseTest


class ToolCustodyTest(ToolPhaseTest):
    def test_actual_phase_seal_and_completed_readback(self):
        source={'files':{'src/backend/space/go.mod':'1'*64,'src/backend/space/Dockerfile.native-migrate':'2'*64}}
        with tempfile.TemporaryDirectory() as temp, patch.object(media_backup_upload_phase,'ROOT',Path(temp)), \
             patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)), \
             patch.object(publish_candidate,'source_snapshot',return_value=source):
            root=Path(temp)/self.fence.context['operation_id']; root.mkdir(mode=0o700)
            receipt=self.phase.run('token')
            marker=custody.seal(self.phase,'token')
            row,_=self.fence.ledger.read(); row['status']='complete'
            snapshot=custody.read_completed(row,'ns')
            self.assertEqual(snapshot['receipt'],receipt)
            self.assertEqual(marker['snapshot_sha256'],custody.digest(snapshot))
            changed=copy.deepcopy(row); changed['space_native_tool']['receipt_sha256']='0'*64
            with self.assertRaises(custody.CustodyError): custody.read_completed(changed,'ns')
            with self.assertRaises(custody.CustodyError): custody.seal(self.phase,'token')

    def test_seal_refuses_late_receipt_change_before_write(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(media_backup_upload_phase,'ROOT',Path(temp)):
            root=Path(temp)/self.fence.context['operation_id']; root.mkdir(mode=0o700)
            with patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
                self.phase.run('token')
            bad=copy.deepcopy(self.capture); bad['publication']['content']['executable_sha256']='0'*64
            with patch.object(tool,'authenticated_publication',return_value=bad),self.assertRaises(custody.CustodyError):
                custody.seal(self.phase,'token')
            self.assertFalse((root/custody.FILENAME).exists())

    def test_lost_seal_marker_reply_sets_uncertain_and_no_replay(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(media_backup_upload_phase,'ROOT',Path(temp)), \
             patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
            root=Path(temp)/self.fence.context['operation_id']; root.mkdir(mode=0o700)
            self.phase.run('token'); write=self.ledger.write
            def lost(revision,row):
                write(revision,row)
                if custody.MARKER in row and row.get('status')=='active': raise OSError('lost reply')
            with patch.object(self.ledger,'write',side_effect=lost),self.assertRaises(custody.CustodyError):
                custody.seal(self.phase,'token')
            self.assertEqual(self.pkg.base.ledger.row['status'],'uncertain')
            with self.assertRaises(custody.CustodyError): custody.seal(self.phase,'token')

    def test_c2_reuses_original_tool_only_for_matching_native_closure(self):
        self.fence.context['operation_id']='media-c2-'+'a'*32
        self.pkg.phase.fence.context['operation_id']=self.fence.context['operation_id']
        package={'source_sha':'b'*40}; inventory={'source_sha':'a'*40,'build_source_sha256':'1'*64,'sql_files':{'1.sql':'2'*64}}
        contract={'schema':'voice-space-native-local-closure-v1','packages':{'src/backend/space/cmd/native-migrate':['src/backend/space/cmd/native-migrate/a.go']},
                  'files':{'src/backend/space/cmd/native-migrate/a.go':'3'*64,'src/backend/space/go.mod':'4'*64},
                  'workspace':[],'raw_sha256':'5'*64}
        receipt={'source_sha':'a'*40,'immutable_image':'qualified-original'}
        previous={'tool_completion':{'snapshot':{'inventory':inventory,'contract':contract,'receipt':receipt}}}
        source={'files':dict(contract['files'],**{'src/backend/user/unrelated.go':'6'*64})}
        candidate=dict(inventory,source_sha='b'*40,build_source_sha256='7'*64)
        with patch.object(predecessor,'load',return_value=copy.deepcopy(previous)), \
             patch.object(self.pkg.phase,'verify_current',return_value=package), \
             patch.object(publish_candidate,'source_snapshot',return_value=source), \
             patch.object(tool,'source_inventory',return_value=candidate):
            self.assertEqual(custody.reuse(self.fence,self.client,self.phase.lock,'ns',self.journal,self.pkg.phase,'token'),receipt)
            source['files']['src/backend/space/cmd/native-migrate/new.go']='8'*64
            with self.assertRaises(custody.CustodyError):
                custody.reuse(self.fence,self.client,self.phase.lock,'ns',self.journal,self.pkg.phase,'token')

if __name__=='__main__': unittest.main()
