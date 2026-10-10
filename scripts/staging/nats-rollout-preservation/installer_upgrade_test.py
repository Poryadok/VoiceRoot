"""Finite V2 predecessor swap; all files are disposable fixture bytes."""
import json,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import installer

class Tests(unittest.TestCase):
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
    def test_actual_v13_upgrade_reaches_expired_helper_and_restores_ingress(self):
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
                installer.upgrade(code,version='v13')
            helper.assert_called_once_with(code,installed,kube,'v13')
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
