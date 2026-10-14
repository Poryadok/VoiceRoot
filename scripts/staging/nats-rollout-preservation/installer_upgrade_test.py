"""Finite V2 predecessor swap; all files are disposable fixture bytes."""
import json,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import stat
from types import SimpleNamespace
import installer
import root_main

class Tests(unittest.TestCase):
    def test_search_bootstrap_idle_route_uses_exact_a025_predecessor(self):
        with patch.object(installer,'_upgrade_idle_transition') as transition:
            installer.upgrade_idle_search_bootstrap_v1(Path('/candidate'))
        transition.assert_called_once_with(Path('/candidate'),
            'a025ffb62b4c0716a44611c4ac00fe9cae60b8bcb5e8b48204b44a62dc541749',
            'idle-search-bootstrap-v1','search-a025',
            'NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_SEARCH_BOOTSTRAP_V1_KEYS_POLICY_HISTORY_PRESERVED')

    def test_search_bootstrap_swap_preserves_history_and_rejects_other_predecessor(self):
        predecessor='a025ffb62b4c0716a44611c4ac00fe9cae60b8bcb5e8b48204b44a62dc541749'
        for actual in (predecessor,'f'*64):
            with self.subTest(actual=actual),tempfile.TemporaryDirectory() as td:
                source,installed=self.fixture(Path(td))
                prior=b'preserved earlier upgrade receipt'
                (installed/'upgrade-idle-source-capture-v1.json').write_bytes(prior)
                def binding(path):return actual if (Path(path)/'binding').read_text()=='old' else 'd'*64
                with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer.os,'chown'),\
                     patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                    args=(source,installed,1000,predecessor,'idle-search-bootstrap-v1','search-a025')
                    if actual!=predecessor:
                        with self.assertRaisesRegex(installer.Blocked,'predecessor_unapproved'):installer._upgrade_code(*args)
                        self.assertFalse((installed/'code-search-a025-preserved').exists())
                    else:
                        installer._upgrade_code(*args);installer._upgrade_code(*args)
                        self.assertEqual((installed/'code-search-a025-preserved/binding').read_text(),'old')
                        self.assertEqual(json.loads((installed/'upgrade-idle-search-bootstrap-v1.json').read_bytes()),
                            {'schema':'voice-nats-code-upgrade-idle-search-bootstrap-v1','from':predecessor,'to':'d'*64})
                self.assertEqual((installed/'upgrade-idle-source-capture-v1.json').read_bytes(),prior)
                self.assertEqual((installed/'policy.json').read_bytes(),b'fixture-policy')
                self.assertEqual((installed/'recovery/recovery-key.pem').read_bytes(),b'fixture-private-key')

    def test_ceccb_idle_transition_uses_exact_receipt_stage_backup_and_is_idempotent(self):
        with tempfile.TemporaryDirectory() as td:
            source,installed=self.fixture(Path(td))
            prior=b'{"schema":"voice-nats-code-upgrade-idle","from":"v15","to":"earlier"}'
            (installed/'upgrade-idle.json').write_bytes(prior)
            def binding(path):
                return installer.CECCB_INSTALLED_BINDING if (Path(path)/'binding').read_text()=='old' else 'd'*64
            with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer.os,'chown'),\
                patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                installer._upgrade_code(source,installed,1000,installer.CECCB_INSTALLED_BINDING,'idle-ceccb-v1','ceccb')
                installer._upgrade_code(source,installed,1000,installer.CECCB_INSTALLED_BINDING,'idle-ceccb-v1','ceccb')
            self.assertEqual((installed/'code/binding').read_text(),'new')
            self.assertEqual((installed/'code-ceccb-preserved/binding').read_text(),'old')
            self.assertEqual((installed/'upgrade-idle.json').read_bytes(),prior)
            self.assertEqual(json.loads((installed/'upgrade-idle-ceccb-v1.json').read_bytes()),
                {'schema':'voice-nats-code-upgrade-idle-ceccb-v1','from':installer.CECCB_INSTALLED_BINDING,'to':'d'*64})
            self.assertFalse((installed/'code-idle-ceccb-v1-staged').exists())
            self.assertEqual((installed/'policy.json').read_bytes(),b'fixture-policy')
            self.assertEqual((installed/'recovery/recovery-key.pem').read_bytes(),b'fixture-private-key')

    def test_ceccb_idle_transition_refuses_wrong_predecessor_receipt_stage_and_backup(self):
        with tempfile.TemporaryDirectory() as td:
            source,installed=self.fixture(Path(td))
            with patch.object(installer,'binding_sha',return_value='c'*64),patch.object(installer.os,'rename') as rename:
                with self.assertRaisesRegex(installer.Blocked,'predecessor_unapproved'):
                    installer._upgrade_code(source,installed,1000,'wrong','idle-ceccb-v1','ceccb')
            rename.assert_not_called()
        for conflict in ('receipt','stage','backup','target'):
            with self.subTest(conflict=conflict),tempfile.TemporaryDirectory() as td:
                source,installed=self.fixture(Path(td))
                def binding(path):
                    marker=(Path(path)/'binding').read_text()
                    return installer.CECCB_INSTALLED_BINDING if marker=='old' else 'd'*64 if marker=='new' else 'f'*64
                receipt=installed/'upgrade-idle-ceccb-v1.json'
                record={'schema':'voice-nats-code-upgrade-idle-ceccb-v1','from':installer.CECCB_INSTALLED_BINDING,'to':'d'*64}
                if conflict=='receipt':record['from']='wrong'
                receipt.write_text(json.dumps(record))
                if conflict=='stage':
                    stage=installed/'code-idle-ceccb-v1-staged';stage.mkdir();(stage/'binding').write_text('bad')
                if conflict=='backup':
                    (installed/'code').rename(installed/'unused')
                    backup=installed/'code-ceccb-preserved';backup.mkdir();(backup/'binding').write_text('bad')
                if conflict=='target':
                    source.joinpath('binding').write_text('changed')
                    (installed/'code').rename(installed/'code-ceccb-preserved')
                    stage=installed/'code-idle-ceccb-v1-staged';stage.mkdir();(stage/'binding').write_text('new')
                with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer.os,'rename') as rename,\
                    patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                    label={'receipt':'receipt_conflict','stage':'staged_unapproved','backup':'backup_conflict','target':'receipt_conflict'}[conflict]
                    with self.assertRaisesRegex(installer.Blocked,label):
                        installer._upgrade_code(source,installed,1000,installer.CECCB_INSTALLED_BINDING,'idle-ceccb-v1','ceccb')
                rename.assert_not_called()

    def test_ceccb_idle_transition_crash_between_renames_resumes_exact_bytes(self):
        with tempfile.TemporaryDirectory() as td:
            source,installed=self.fixture(Path(td));rename=installer.os.rename
            def binding(path):
                marker=(Path(path)/'binding').read_text()
                return installer.CECCB_INSTALLED_BINDING if marker=='old' else 'd'*64
            def crash(old,new):
                if Path(old).name=='code-idle-ceccb-v1-staged':raise RuntimeError('fixture-process-interrupted')
                return rename(old,new)
            args=(source,installed,1000,installer.CECCB_INSTALLED_BINDING,'idle-ceccb-v1','ceccb')
            with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer.os,'chown'),\
                patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                with patch.object(installer.os,'rename',side_effect=crash):
                    with self.assertRaisesRegex(RuntimeError,'process-interrupted'):installer._upgrade_code(*args)
                self.assertFalse((installed/'code').exists())
                self.assertEqual((installed/'code-ceccb-preserved/binding').read_text(),'old')
                self.assertEqual((installed/'code-idle-ceccb-v1-staged/binding').read_text(),'new')
                installer._upgrade_code(*args)
            self.assertEqual((installed/'code/binding').read_text(),'new')
            self.assertEqual((installed/'code-ceccb-preserved/binding').read_text(),'old')

    def test_v13_swap_preserves_v12_code_and_receipt_bytes_and_is_idempotent(self):
        import expired_recovery
        with tempfile.TemporaryDirectory() as td:
            source,installed=self.fixture(Path(td))
            prior=b'{"schema":"voice-nats-code-upgrade-v12","from":"preserved","to":"original"}'
            (installed/'upgrade-v12.json').write_bytes(prior)
            def binding(path):
                return expired_recovery.V12_BINDING if (Path(path)/'binding').read_text()=='old' else 'b'*64
            with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer.os,'chown'),\
                patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                installer._upgrade_code(source,installed,1000,expired_recovery.V12_BINDING,'v13','v12')
                installer._upgrade_code(source,installed,1000,expired_recovery.V12_BINDING,'v13','v12')
            self.assertEqual((installed/'code-v12-preserved/binding').read_text(),'old')
            self.assertEqual((installed/'upgrade-v12.json').read_bytes(),prior)
            self.assertEqual(json.loads((installed/'upgrade-v13.json').read_bytes()),
                {'schema':'voice-nats-code-upgrade-v13','from':expired_recovery.V12_BINDING,'to':'b'*64})
            self.assertEqual((installed/'policy.json').read_bytes(),b'fixture-policy')
            self.assertEqual((installed/'recovery/recovery-key.pem').read_bytes(),b'fixture-private-key')

    def test_v12_swap_preserves_v11_code_and_receipt_bytes_and_is_idempotent(self):
        import expired_recovery
        with tempfile.TemporaryDirectory() as td:
            source,installed=self.fixture(Path(td))
            prior=b'{"schema":"voice-nats-code-upgrade-v11","from":"preserved","to":"original"}'
            (installed/'upgrade-v11.json').write_bytes(prior)
            def binding(path):
                return expired_recovery.V11_BINDING if (Path(path)/'binding').read_text()=='old' else 'b'*64
            with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer.os,'chown'),\
                patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                installer._upgrade_code(source,installed,1000,expired_recovery.V11_BINDING,'v12','v11')
                installer._upgrade_code(source,installed,1000,expired_recovery.V11_BINDING,'v12','v11')
            self.assertEqual((installed/'code-v11-preserved/binding').read_text(),'old')
            self.assertEqual((installed/'upgrade-v11.json').read_bytes(),prior)
            self.assertEqual(json.loads((installed/'upgrade-v12.json').read_bytes()),
                {'schema':'voice-nats-code-upgrade-v12','from':expired_recovery.V11_BINDING,'to':'b'*64})
            self.assertEqual((installed/'policy.json').read_bytes(),b'fixture-policy')
            self.assertEqual((installed/'recovery/recovery-key.pem').read_bytes(),b'fixture-private-key')

    def fixture(self,root):
        source=root/'source';source.mkdir();(source/'binding').write_text('new')
        installed=root/'installed';installed.mkdir();code=installed/'code';code.mkdir()
        (code/'binding').write_text('old')
        (installed/'policy.json').write_bytes(b'fixture-policy')
        recovery=installed/'recovery';recovery.mkdir();(recovery/'recovery-key.pem').write_bytes(b'fixture-private-key')
        return source,installed

    def digest(self,path):
        return installer.V2_BINDING if (Path(path)/'binding').read_text()=='old' else 'b'*64

    def test_swap_retains_old_code_and_policy_key_bytes(self):
        with tempfile.TemporaryDirectory() as td:
            source,installed=self.fixture(Path(td))
            with patch.object(installer,'binding_sha',side_effect=self.digest),patch.object(installer.os,'chown'),patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                installer.upgrade_code(source,installed,1000)
                installer.upgrade_code(source,installed,1000)
            self.assertEqual((installed/'code'/'binding').read_text(),'new')
            self.assertEqual((installed/'code-v2-preserved'/'binding').read_text(),'old')
            self.assertEqual((installed/'policy.json').read_bytes(),b'fixture-policy')
            self.assertEqual((installed/'recovery/recovery-key.pem').read_bytes(),b'fixture-private-key')

    def test_crash_between_renames_resumes_only_exact_staged_and_old_bindings(self):
        with tempfile.TemporaryDirectory() as td:
            source,installed=self.fixture(Path(td));rename=installer.os.rename
            def crash(old,new):
                if Path(old).name=='code-v3-staged':raise RuntimeError('fixture-process-interrupted')
                return rename(old,new)
            with patch.object(installer,'binding_sha',side_effect=self.digest),patch.object(installer.os,'chown'),patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                with patch.object(installer.os,'rename',side_effect=crash):
                    with self.assertRaisesRegex(RuntimeError,'process-interrupted'):installer.upgrade_code(source,installed,1000)
                self.assertFalse((installed/'code').exists())
                self.assertTrue((installed/'code-v2-preserved').exists())
                installer.upgrade_code(source,installed,1000)
            self.assertEqual((installed/'code/binding').read_text(),'new')

    def test_unknown_predecessor_or_changed_resume_target_never_renames(self):
        with tempfile.TemporaryDirectory() as td:
            source,installed=self.fixture(Path(td))
            with patch.object(installer,'binding_sha',return_value='c'*64),patch.object(installer.os,'rename') as rename:
                with self.assertRaisesRegex(installer.Blocked,'predecessor_unapproved'):installer.upgrade_code(source,installed,1000)
            rename.assert_not_called()

class ActualUpgradeDispatchTests(unittest.TestCase):
    def test_idle_checkpoint_reader_accepts_payload_above_generic_json_limit_and_keeps_128m_cap(self):
        payload={'fixture':'x'*((2<<20)+1)}
        raw=(json.dumps(payload,sort_keys=True,indent=2)+'\n').encode()
        with tempfile.TemporaryDirectory() as td:
            path=Path(td)/'checkpoint.json';root_main.save_checkpoint(path,payload)
            self.assertEqual(path.stat().st_size,len(raw));self.assertGreater(len(raw),2<<20)
            def file_stat(_fd,size=len(raw)):
                return SimpleNamespace(st_mode=stat.S_IFREG|0o600,st_uid=0,st_size=size)
            with patch.object(installer.os,'fstat',side_effect=file_stat):
                self.assertEqual(installer.checkpoint_json(path),payload)
                with self.assertRaisesRegex(installer.Blocked,'checkpoint_custody_invalid'):
                    installer.private_json(path)
            with patch.object(installer.os,'fstat',return_value=file_stat(None,(128<<20)+1)):
                with self.assertRaisesRegex(installer.Blocked,'checkpoint_custody_invalid'):
                    installer.checkpoint_json(path)

    def test_actual_v13_upgrade_reaches_expired_helper_and_restores_ingress(self):
        self.exercise_upgrade('v13')

    def test_actual_v14_upgrade_reaches_expired_helper_and_restores_ingress(self):
        self.exercise_upgrade('v14')

    def test_actual_v15_upgrade_reaches_expired_helper_and_restores_ingress(self):
        self.exercise_upgrade('v15')

    def exercise_upgrade(self,version):
        import contextlib,expired_recovery
        with tempfile.TemporaryDirectory() as td:
            root=Path(td); installed=root/'installed'; installed.mkdir()
            for name in ('inbox','processing','journal','responses','recovery','sources'):
                (installed/name).mkdir(mode=0o700)
            kube=object(); code=root/'candidate'; code.mkdir()
            with patch.object(installer.os,'geteuid',return_value=0), \
                patch.object(installer.sys,'platform','linux'), \
                patch.object(installer.root_cli,'code_binding',return_value={}), \
                patch.object(installer.guard,'ROOT',root), \
                patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()), \
                patch.object(installer,'Kube',return_value=kube), \
                patch.object(expired_recovery,'install_helper',autospec=True) as helper:
                installer.upgrade(code,version=version)
            helper.assert_called_once_with(code,installed,kube,version)
            self.assertEqual((installed/'inbox').stat().st_mode & 0o7777,0o1730)

    def test_actual_v13_helper_refusal_restores_ingress(self):
        import contextlib,expired_recovery
        with tempfile.TemporaryDirectory() as td:
            root=Path(td); installed=root/'installed'; installed.mkdir()
            for name in ('inbox','processing','journal','responses','recovery','sources'):
                (installed/name).mkdir(mode=0o700)
            code=root/'candidate'; code.mkdir()
            with patch.object(installer.os,'geteuid',return_value=0), \
                patch.object(installer.sys,'platform','linux'), \
                patch.object(installer.root_cli,'code_binding',return_value={}), \
                patch.object(installer.guard,'ROOT',root), \
                patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()), \
                patch.object(installer,'Kube',return_value=object()), \
                patch.object(expired_recovery,'install_helper',autospec=True,side_effect=installer.Blocked('fixture_helper_refused')):
                with self.assertRaisesRegex(installer.Blocked,'fixture_helper_refused'):
                    installer.upgrade(code,version='v13')
            self.assertEqual((installed/'inbox').stat().st_mode & 0o7777,0o1730)

if __name__=='__main__':unittest.main()
