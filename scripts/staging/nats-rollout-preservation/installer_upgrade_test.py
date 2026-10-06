"""Finite V2 predecessor swap; all files are disposable fixture bytes."""
import json,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import installer

class Tests(unittest.TestCase):
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

if __name__=='__main__':unittest.main()
