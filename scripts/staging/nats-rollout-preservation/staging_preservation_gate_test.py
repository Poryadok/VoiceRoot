"""Unsafe staging migration paths must stop before any external effect."""
import copy
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import bridge_root
import migrations
import transaction
from controller import Blocked


class PreservationGateTests(unittest.TestCase):
    def test_bridge_refuses_full_and_app_only_before_source_or_parent_effects(self):
        actions = object.__new__(bridge_root.Actions)
        actions._idle = Mock()
        actions._dispatcher = Mock()
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode), self.assertRaisesRegex(Blocked, 'staging_migration_preservation_authority_missing'):
                actions._prepare({'mode':mode,'nonce':'a'*64,'token':'fixture','changed_services':[]})
        actions._idle.assert_not_called()
        actions._dispatcher.assert_not_called()

    def test_existing_checkpoint_and_direct_runner_cannot_start_migrations(self):
        kube=Mock();stage=Mock();journal=Mock()
        with tempfile.TemporaryDirectory() as folder:
            path=Path(folder)
            state={'target':{'mode':'full'},'migrations':[{'database':'space'}]}
            with self.assertRaisesRegex(Blocked, 'staging_migration_preservation_authority_missing'):
                transaction.authorize(kube,path,state,'a'*64,'b'*64,{})
            self.assertEqual(list(path.iterdir()),[])
        with self.assertRaisesRegex(Blocked, 'staging_migration_preservation_authority_missing'):
            migrations.execute(kube,stage,[{'database':'space'}],'full',{},journal)
        self.assertEqual(kube.mock_calls, [])
        self.assertEqual(stage.mock_calls, [])
        journal.assert_not_called()

    def test_bridge_authorization_paths_stop_before_custody_or_restore(self):
        actions=object.__new__(bridge_root.Actions)
        state={'operation':'a'*12,'target':{'mode':'full'},'migrations':[{'database':'space'}]}
        before=copy.deepcopy(state)
        actions.state=Mock(return_value=(Path('/unused'),state))
        actions._dispatcher=Mock();actions._verify_decrypted_restore=Mock()
        actions._authorize_expired_native=Mock()
        with patch.object(bridge_root.github_custody,'verify_artifact') as artifact, \
             patch.object(bridge_root,'save') as save:
            for action in ('authorize','authorize-preserved-upload','authorize-expired-native'):
                with self.subTest(action=action),self.assertRaisesRegex(Blocked,'staging_migration_preservation_authority_missing'):
                    actions.execute({'action':action,'operation':'a'*12})
            with self.assertRaisesRegex(Blocked,'staging_migration_preservation_authority_missing'):
                actions.recover({'action':'authorize','operation':'a'*12})
            artifact.assert_not_called();save.assert_not_called()
        actions._dispatcher.assert_not_called();actions._verify_decrypted_restore.assert_not_called()
        actions._authorize_expired_native.assert_not_called();self.assertEqual(state,before)

    def test_images_only_without_migrations_is_not_held_by_this_gate(self):
        self.assertIsNone(migrations.require_preserving_admission('images-only', []))
        with self.assertRaisesRegex(Blocked, 'staging_migration_preservation_authority_missing'):
            migrations.require_preserving_admission('images-only', [{'database':'space'}])

    def test_empty_migration_plan_does_not_bypass_full_or_app_only_admission(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode):
                kube=Mock();stage=Mock();journal=Mock()
                with self.assertRaisesRegex(Blocked, 'staging_migration_preservation_authority_missing'):
                    migrations.execute(kube,stage,[],mode,{},journal)
                self.assertEqual(kube.mock_calls, [])
                self.assertEqual(stage.mock_calls, [])
                journal.assert_not_called()

    @unittest.skipUnless(__import__('os').name=='posix' and __import__('os').geteuid()==0,
                         'checkpoint custody requires isolated Linux root')
    def test_large_root_checkpoint_records_blocked_without_masking_failure(self):
        import json,root_main
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            base=Path(folder);path=base/'checkpoint.json'
            state={'phase':'READ_ONLY_CAPTURE','status':'RUNNING','fence_status':'UNKNOWN',
                   'source_fixture':'x'*(3<<20)}
            root_main.save(path,state,limit=128<<20)
            with patch.object(transaction,'_prepare',side_effect=RuntimeError('fixture failure')):
                with self.assertRaisesRegex(RuntimeError,'fixture failure'):
                    transaction.prepare(None,base,base,{}, {},'a'*12,{},[],{},
                                        realtime_bootstrap_receipt={'fixture':True})
            result=json.loads(path.read_bytes())
            self.assertEqual((result['status'],result['fence_status']),('BLOCKED','UNKNOWN'))
            self.assertEqual(len(result['source_fixture']),3<<20)


if __name__=='__main__':unittest.main()
