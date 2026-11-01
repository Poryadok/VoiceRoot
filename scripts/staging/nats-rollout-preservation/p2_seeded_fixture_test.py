import sys
import os
import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import p2_seeded_fixture as fixture


class AdmissionTests(unittest.TestCase):
    def test_unapproved_host_is_rejected_before_touching_paths(self):
        for platform, uid, env in (('win32', 0, {}), ('linux', 1, {}),
                                   ('linux', 0, {}),
                                   ('linux', 0, {'GITHUB_ACTIONS': 'true', 'RUNNER_ENVIRONMENT': 'self-hosted'})):
            with self.subTest(platform=platform, uid=uid, env=env):
                with patch.object(fixture.sys, 'platform', platform), patch.object(fixture.os, 'geteuid', return_value=uid, create=True), patch.dict(fixture.os.environ, env, clear=True):
                    with self.assertRaisesRegex(RuntimeError, 'owned_fixture_host_required'):
                        fixture.require_hosted()

    def test_no_context_is_returned_when_selected_path_is_private(self):
        with self.assertRaisesRegex(RuntimeError, 'owned_fixture_store_not_external'):
            fixture.require_external('/private/base', '/private/base/store')

    def test_runtime_identity_is_checkpointed_before_caller_can_create(self):
        checkpoints=[]
        handle=fixture.Retained(Path('/base'),Path('/selected'),lambda state:checkpoints.append(
            [runtime.operation for runtime in state.runtimes]))
        runtime=type('Runtime',(),{'operation':'ownedp2fixture'})()
        handle.register(runtime)
        self.assertEqual(checkpoints,[['ownedp2fixture']])
        self.assertIs(handle.runtimes[0],runtime)

    def test_cleanup_requires_explicit_successful_reconciliation(self):
        handle=fixture.Retained(Path('/base'),Path('/selected'),lambda _:None)
        for callback in (None,lambda _:False):
            with self.subTest(callback=callback), self.assertRaisesRegex(RuntimeError,'owned_fixture_reconciliation_required'):
                fixture.reconcile_containers(handle,callback)

    def test_delayed_create_cannot_be_declared_clean_by_empty_inventory(self):
        class Pending:
            operation='pendingfixture'
            owned={}
            calls=0
            def run(self,*args,**kwargs):
                self.calls+=1
                return ''
        handle=fixture.Retained(Path('/base'),Path('/selected'),lambda _:None)
        runtime=Pending();handle.register(runtime)
        # An unresolved create is not quiescent even if a daemon snapshot is empty.
        with self.assertRaisesRegex(RuntimeError,'owned_fixture_reconciliation_required'):
            fixture.reconcile_containers(handle,lambda _:False)
        self.assertEqual(runtime.calls,0)

    @unittest.skipUnless(sys.platform == 'linux' and os.geteuid() == 0, 'Linux root custody')
    def test_real_native_archive_is_restored_without_changing_selected_leaf(self):
        sys.path.insert(0, str(Path(__file__).parent.parent/'nats-known-baseline'))
        self.addCleanup(sys.path.pop, 0)
        from native_store import archive_closed_store
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); source = root/'source'; source.mkdir()
            (source/'jetstream').mkdir(); (source/'jetstream'/'record').write_bytes(b'fixture-record')
            archive = root/'archive.tar'; manifest = archive_closed_store(source,archive)
            selected = root/'selected'; selected.mkdir();os.chown(selected,65532,10000);selected.chmod(0o2770)
            before = selected.stat()
            fixture._restore_to_selected(archive,manifest,selected)
            after = selected.stat()
            self.assertEqual((after.st_dev,after.st_ino,after.st_uid,after.st_gid,after.st_mode & 0o7777),
                (before.st_dev,before.st_ino,65532,10000,0o2770))
            self.assertEqual((selected/'jetstream'/'record').read_bytes(),b'fixture-record')
            self.assertEqual((selected/'jetstream'/'record').stat().st_uid,65532)
            self.assertEqual((selected/'jetstream'/'record').stat().st_gid,10000)

    @unittest.skipUnless(sys.platform == 'linux' and os.geteuid() == 0, 'Linux root custody')
    def test_pins_reject_changed_or_symlinked_input(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);good=root/'helper';good.write_bytes(b'known');good.chmod(0o444)
            digest=hashlib.sha256(b'known').hexdigest()
            self.assertEqual(fixture._checked_file(root,'helper',digest),good)
            with self.assertRaisesRegex(RuntimeError,'owned_fixture_input_changed'):
                fixture._checked_file(root,'helper','0'*64)
            (root/'link').symlink_to(good)
            with self.assertRaisesRegex(RuntimeError,'owned_fixture_input_untrusted'):
                fixture._checked_file(root,'link',digest)

    @unittest.skipUnless(sys.platform == 'linux' and os.geteuid() == 0, 'Linux root custody')
    def test_actual_selected_leaf_custody_rejects_wrong_group(self):
        sys.path.insert(0, str(Path(__file__).parent.parent/'nats-known-baseline'))
        self.addCleanup(sys.path.pop, 0)
        from controller import Blocked
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            path=Path(directory)/'selected';path.mkdir();os.chown(path,65532,10000);path.chmod(0o2770)
            descriptor={'schema':'voice-selected-store-custody-v1','hub_uid':'h','claim_uid':'c','pv_uid':'p',
                'path':str(path),'uid':65532,'gid':10000,'mode':0o2770}
            fixture._verify_selected_path(path,descriptor)
            os.chown(path,65532,65532)
            with self.assertRaises(Blocked):fixture._verify_selected_path(path,descriptor)

    @unittest.skipUnless(sys.platform == 'linux' and os.geteuid() == 0, 'Linux root custody')
    def test_partial_cleanup_checks_custody_and_rejects_unknown_ids(self):
        sys.path.insert(0, str(Path(__file__).parent.parent/'nats-known-baseline'))
        self.addCleanup(sys.path.pop, 0)
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);base=root/'base';base.mkdir(mode=0o700)
            path=root/'selected';path.mkdir();os.chown(path,65532,10000);path.chmod(0o2770)
            descriptor={'schema':'voice-selected-store-custody-v1','hub_uid':'h','claim_uid':'c','pv_uid':'p',
                'path':str(path),'uid':65532,'gid':10000,'mode':0o2770}
            handle=fixture.Retained(base,path,lambda _:None);handle.descriptor=descriptor;handle.input_sha256={}
            handle.seal_custody()
            class Runtime:
                operation='prior123456789abc'
                def __init__(self,identity='a'):
                    self.owned={'owned':{'id':identity*64}}
                    self.present={identity*64}
                    self.removed=[]
                def run(self,args,**kwargs):
                    if args[0]=='ps':
                        if args[3]=='label='+__import__('docker_runtime').LABEL+'='+self.operation:
                            return '\n'.join(self.present)
                        return ''
                    if args[0]=='rm':self.removed.append(args[-1]);self.present.remove(args[-1]);return ''
                    raise AssertionError(args)
                def inspect(self,name):return {'State':{'Running':False}}
                def no_operation_containers(self):
                    if self.present:raise AssertionError('containers remain')
            runtime=Runtime();handle.register(runtime)
            second=Runtime('c');second.operation='other123456789abc';handle.register(second)
            second.present.add('b'*64)
            with self.assertRaisesRegex(RuntimeError,'owned_fixture_cleanup_uncertain'):
                fixture.reconcile_containers(handle,lambda _:True)
            self.assertEqual(runtime.removed,[])
            self.assertEqual(second.removed,[])
            second.present.remove('b'*64)
            fixture.reconcile_containers(handle,lambda _:True)
            self.assertEqual(runtime.removed,['a'*64])
            self.assertEqual(second.removed,['c'*64])
            self.assertTrue(path.is_dir())


if __name__ == '__main__':
    unittest.main()
