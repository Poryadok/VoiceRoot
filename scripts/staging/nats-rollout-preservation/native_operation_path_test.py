import os
from pathlib import Path
import secrets
import tempfile
import unittest
from unittest.mock import patch

import native_operation_path as paths
from controller import Blocked
import root_cli


class NativeOperationPathTests(unittest.TestCase):
    def test_fixed_anchor_creates_and_reads_only_a_new_private_operation(self):
        if os.geteuid()!=paths.UID or os.getegid()!=paths.GID:
            self.skipTest('native operation path positive requires UID:GID 65532')
        if Path(os.environ.get('NATIVE_PRIVATE_TEST_ANCHOR',''))!=paths.ANCHOR:
            self.skipTest('native operation path positive requires the fixed test mount')
        operation=secrets.token_hex(6)
        base=paths.create_operation(operation)
        self.assertEqual(base,paths.ANCHOR/('rollout-'+operation))
        self.assertEqual(paths.existing_operation(operation),base)
        self.assertEqual(root_cli.operation_path(str(base),owner_uid=paths.UID),base)
        self.assertEqual(paths.owner_uid_for_path(base/'checkpoint.json'),paths.UID)
        with self.assertRaises(Blocked):paths.create_operation(operation)
        self.assertEqual(stat_mode(base),0o700)

    def test_existing_operation_rejects_mode_and_symlink_drift(self):
        if os.geteuid()!=paths.UID or os.getegid()!=paths.GID:
            self.skipTest('native operation path custody requires UID:GID 65532')
        if Path(os.environ.get('NATIVE_PRIVATE_TEST_ANCHOR',''))!=paths.ANCHOR:
            self.skipTest('native operation path custody requires the fixed test mount')
        operation=secrets.token_hex(6);base=paths.create_operation(operation)
        base.chmod(0o750)
        with self.assertRaises(Blocked):paths.existing_operation(operation)
        base.chmod(0o700)
        alias=paths.ANCHOR/('rollout-'+secrets.token_hex(6))
        alias.symlink_to(base,target_is_directory=True)
        alias_operation=alias.name[len('rollout-'):]
        with self.assertRaises((Blocked,OSError)):paths.existing_operation(alias_operation)
        alias.unlink()

    def test_wrong_uid_invalid_names_and_caller_anchor_override_fail_closed(self):
        if os.geteuid()==paths.UID and os.getegid()==paths.GID:
            self.skipTest('wrong UID is exercised by the UID 1000 suite')
        with self.assertRaises(Blocked):paths.operation_root()
        for operation in ('../etc/passwd','A'*12,'a'*11,'a'*13):
            with self.assertRaises(Blocked):paths.create_operation(operation)
        with self.assertRaises(TypeError):paths.create_operation('a17c93e4d205',anchor=Path(tempfile.gettempdir()))

    def test_native_path_is_not_accepted_by_legacy_root_owner_mode(self):
        with self.assertRaises(Blocked):root_cli.operation_path(
            str(paths.ANCHOR/'rollout-a17c93e4d205'),owner_uid=0)

    def test_native_actions_select_fixed_root_owned_group_readable_source_mode(self):
        if os.geteuid()!=paths.UID:
            with self.assertRaises(Blocked):
                root_cli.code_binding(root_cli.NATIVE_INSTALLED_CODE,owner_uid=os.geteuid(),native_source=True)
            return
        fixture=Path(os.environ.get('NATIVE_SOURCE_FIXTURE',''))
        if fixture!=root_cli.NATIVE_INSTALLED_CODE or not fixture.is_dir():
            self.skipTest('native source binding requires build-time immutable fixture at fixed installed prefix')
        binding=root_cli.code_binding(fixture,owner_uid=paths.UID,native_source=True)
        self.assertTrue(binding)
        from bridge_root import Actions
        actions=Actions(fixture,owner_uid=paths.UID,native_api=object(),native_config={})
        self.assertEqual(actions.binding,binding)
        with self.assertRaises(Blocked):
            root_cli.code_binding(fixture.parent,owner_uid=paths.UID,native_source=True)
        negatives=Path(os.environ.get('NATIVE_SOURCE_NEGATIVE_ROOT',''))
        if not negatives.is_dir():
            self.skipTest('native source custody negatives require build-time fixture variants')
        for name in ('runner-owned','foreign-owner','foreign-group','group-writable','symlink','source-drift'):
            candidate=negatives/name
            with self.subTest(source=name),patch.object(root_cli,'NATIVE_INSTALLED_CODE',candidate):
                with self.assertRaises(Blocked):
                    root_cli.code_binding(candidate,owner_uid=paths.UID,native_source=True)

    def test_native_prepare_value_uses_fixed_anchor_and_uid_owned_checkpoint(self):
        if os.geteuid()!=paths.UID or os.getegid()!=paths.GID:
            self.skipTest('native prepare path requires UID:GID 65532')
        if Path(os.environ.get('NATIVE_PRIVATE_TEST_ANCHOR',''))!=paths.ANCHOR:
            self.skipTest('native prepare path requires the fixed test mount')
        operation=secrets.token_hex(6)
        value={'manifests':[],'target':{},'contract':{},'migrations':[],'nonnats':{}}
        with tempfile.TemporaryDirectory() as source:
            source_path=Path(source)
            state={'operation':operation,'phase':'READ_ONLY_CAPTURE','events':[]}
            with patch.object(root_cli.transaction,'prepare',return_value=state) as prepare:
                result=root_cli.prepare_value(value,source_path,{},operation,native_api=object(),native_config={})
            self.assertIs(result,state)
            self.assertEqual(prepare.call_args.args[1],paths.ANCHOR/('rollout-'+operation))
            self.assertTrue((paths.ANCHOR/('rollout-'+operation)/'checkpoint.json').is_file())
            self.assertFalse((paths.ANCHOR/('rollout-'+operation)/'code').exists())


def stat_mode(path):
    import stat
    return stat.S_IMODE(path.lstat().st_mode)


if __name__=='__main__':unittest.main()
