import sys
import os
import hashlib
from pathlib import Path
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import p2_seeded_fixture as fixture


class AdmissionTests(unittest.TestCase):
    def test_hosted_driver_rejects_unapproved_environment_before_paths(self):
        import p2_seeded_identity_hosted as driver
        with patch.dict(fixture.os.environ,{},clear=True):
            with self.assertRaisesRegex(RuntimeError,'owned_fixture_host_required'):
                driver.main('/nonexistent-bundle','/nonexistent-workspace')

    def test_changed_preflight_census_blocks_restore(self):
        import staging_cohort_preservation as cohort, transaction, rollout_census
        seed={'streams':[{'name':'seed'}], 'consumers':[]}
        live={'streams':[{'name':'different'}], 'consumers':[]}
        with patch.object(cohort,'_state',return_value={'events':[]}), \
             patch.object(transaction,'require_original_live_census',return_value=live), \
             patch.object(rollout_census,'validate_snapshot',side_effect=lambda value:value), \
             patch.object(fixture,'_restore_to_selected') as restore:
            check=lambda:fixture._check_preflight_seed(object(),object(),seed)
            with self.assertRaisesRegex(RuntimeError,'owned_fixture_preflight_census_changed'):
                fixture._restore_selected_after_fence(check,'archive','manifest','selected')
            restore.assert_not_called()

    def test_private_archive_or_broker_drift_blocks_use(self):
        from unittest.mock import Mock
        import native_store
        from nats_contract_plan import digest
        handle=fixture.Retained(Path('/unused'),None,lambda _:None)
        handle.private_seed={'manifest':{},'broker':'seed','census':{}}
        handle.private_seed_sha256=digest(handle.private_seed);handle.runtime=Mock()
        for archive_ok,running in ((False,False),(True,True)):
            with self.subTest(archive_ok=archive_ok,running=running), \
                 patch.object(native_store,'verify_archive',return_value=archive_ok):
                handle.runtime.inspect.return_value={'State':{'Running':running}}
                with self.assertRaisesRegex(RuntimeError,'owned_fixture_private_seed_changed'):
                    fixture._check_private_seed(handle)

    def test_lost_fence_prevents_selected_restore(self):
        from unittest.mock import Mock
        for fault in ('session','writer'):
            with self.subTest(fault=fault):
                handle,session,stage=Mock(),Mock(),Mock()
                session.check.return_value={'phase':'changed' if fault=='session' else 'cohort'}
                if fault=='writer':stage.verify_final_storage.side_effect=RuntimeError('writer_reopened')
                check=lambda:fixture._check_seed_boundary(handle,session,{'phase':'cohort'},stage)
                with patch.object(fixture,'_restore_to_selected') as restore:
                    with self.assertRaises(RuntimeError):
                        fixture._restore_selected_after_fence(check,'archive','manifest','selected')
                    restore.assert_not_called()

    @unittest.skipUnless(sys.platform == 'linux' and os.geteuid() == 0, 'Linux root custody')
    def test_seed_rejects_changed_identity_before_selected_store(self):
        sys.path.insert(0, str(Path(__file__).parent.parent/'nats-known-baseline'))
        self.addCleanup(sys.path.pop, 0)
        import bootstrap_selection
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'inputs').mkdir(mode=0o700)
            (root/'inputs'/'bootstrap.creds').write_bytes(b'credential')
            handle=fixture.Retained(root,None,lambda _:None)
            handle.input_sha256={'fixture':'pin'};handle.seal_identity_custody()
            handle.installation=SimpleNamespace(sha256='a'*64,generation='fixture')
            handle.enrollment_v1={'fixture':'v1'}
            handle.enrollment_v4={'credential_sha256':hashlib.sha256(b'credential').hexdigest()}
            handle.kube=object();handle.identity_sha256=handle.identity_digest()
            with patch.object(bootstrap_selection,'select'),patch.object(fixture,'require_hosted'):
                handle.verify_identity()
                import resource_claim, resource_claim_test
                context=resource_claim_test.context();card=resource_claim_test.card(context)
                foreign=resource_claim.Session(context,'a'*12,'b'*64,lambda:card,resource_claim_test.MemoryJournal())
                foreign.admit(start=True)
                for session in (None,foreign):
                    with self.assertRaisesRegex(RuntimeError,'owned_fixture_session_changed'):
                        fixture.seed_prepared(handle,None,session=session)
                    self.assertIsNone(handle.store)
                handle.installation=foreign.installation;handle.identity_sha256=handle.identity_digest()
                with self.assertRaisesRegex(RuntimeError,'owned_fixture_stage_changed'):
                    fixture.seed_prepared(handle,None,session=foreign)
                self.assertIsNone(handle.store)
                from runtime_stage import RolloutStage
                stage=object.__new__(RolloutStage);stage.operation=foreign.operation
                handle.private_seed={'census':{'messages':9}};handle.private_seed_sha256='0'*64
                with self.assertRaisesRegex(RuntimeError,'owned_fixture_private_seed_changed'):
                    fixture.seed_prepared(handle,stage,session=foreign)
                self.assertIsNone(handle.store)
                for change in ('enrollment','credential'):
                    with self.subTest(change=change):
                        if change=='enrollment':handle.enrollment_v1['fixture']='foreign'
                        else:(root/'inputs'/'bootstrap.creds').write_bytes(b'foreign')
                        with self.assertRaisesRegex(RuntimeError,'owned_fixture_identity_changed'):
                            fixture.seed_prepared(handle,None,session=None)
                        self.assertIsNone(handle.store)
                        handle.enrollment_v1['fixture']='v1'
                        (root/'inputs'/'bootstrap.creds').write_bytes(b'credential')

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
                            if '--no-trunc' not in args:raise AssertionError('full IDs required')
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
            second.present={'c'*12}
            with self.assertRaisesRegex(RuntimeError,'owned_fixture_cleanup_uncertain'):
                fixture.reconcile_containers(handle,lambda _:True)
            self.assertEqual(runtime.removed,[])
            second.present={'c'*64}
            fixture.reconcile_containers(handle,lambda _:True)
            self.assertEqual(runtime.removed,['a'*64])
            self.assertEqual(second.removed,['c'*64])
            self.assertTrue(path.is_dir())
            # Identity setup can fail before any selected leaf exists.
            early=fixture.Retained(base,None,lambda _:None);early.input_sha256={}
            early.seal_identity_custody();early_runtime=Runtime('d');early.register(early_runtime)
            fixture.reconcile_containers(early,lambda _:True)
            self.assertEqual(early_runtime.removed,['d'*64])


if __name__ == '__main__':
    unittest.main()
