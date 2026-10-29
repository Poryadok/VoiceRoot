"""Unsafe staging migration paths must stop before any external effect."""
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock

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
        kube.assert_not_called();stage.verify_final_storage.assert_not_called();journal.assert_not_called()

    def test_images_only_without_migrations_is_not_held_by_this_gate(self):
        self.assertIsNone(migrations.require_preserving_admission('images-only', []))
        with self.assertRaisesRegex(Blocked, 'staging_migration_preservation_authority_missing'):
            migrations.require_preserving_admission('images-only', [{'database':'space'}])


if __name__=='__main__':unittest.main()
