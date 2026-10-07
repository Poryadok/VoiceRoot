"""V5 rename/retry semantics; real root custody is tested in captured fixture."""
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import installer
from controller import Blocked


class InstallerV5Tests(unittest.TestCase):
    def setup_code(self, root):
        installed=root/'installed';installed.mkdir()
        old=installed/'code';old.mkdir();(old/'component').write_bytes(b'old-code')
        new=root/'candidate';new.mkdir();(new/'component').write_bytes(b'new-code')
        (installed/'policy').write_bytes(b'unchanged policy')
        return installed,new

    def binding(self, path):
        data=(Path(path)/'component').read_bytes()
        return installer.V4_BINDING if data==b'old-code' else hashlib.sha256(data).hexdigest()

    def upgrade(self, candidate, installed):
        with patch.object(installer,'binding_sha',side_effect=self.binding), \
                patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_bytes())), \
                patch.object(installer,'save',side_effect=lambda p,row:Path(p).write_text(json.dumps(row))), \
                patch.object(installer,'sync_directory'), \
                patch.object(installer.os,'chown',create=True):
            installer.upgrade_code_v5(candidate,installed,1000)

    def test_fixed_old_code_preserved_and_repeat_idempotent(self):
        with tempfile.TemporaryDirectory() as td:
            installed,candidate=self.setup_code(Path(td))
            self.upgrade(candidate,installed);self.upgrade(candidate,installed)
            self.assertEqual((installed/'code-v4-preserved'/'component').read_bytes(),b'old-code')
            self.assertEqual((installed/'code'/'component').read_bytes(),b'new-code')
            self.assertEqual((installed/'policy').read_bytes(),b'unchanged policy')
            self.assertEqual(json.loads((installed/'upgrade-v5.json').read_bytes()),
                {'schema':'voice-nats-code-upgrade-v5','from':installer.V4_BINDING,'to':self.binding(candidate)})

    def test_interrupt_after_old_rename_resolves_only_known_staged_code(self):
        with tempfile.TemporaryDirectory() as td:
            installed,candidate=self.setup_code(Path(td));rename=os.rename
            def interrupt(source,target):
                if Path(source).name=='code-v5-staged':raise RuntimeError('fixture-interruption')
                return rename(source,target)
            with patch.object(installer.os,'rename',side_effect=interrupt):
                with self.assertRaises(RuntimeError):self.upgrade(candidate,installed)
            self.assertFalse((installed/'code').exists())
            self.assertTrue((installed/'code-v4-preserved').exists())
            self.upgrade(candidate,installed)
            self.assertEqual((installed/'code'/'component').read_bytes(),b'new-code')

    def test_unknown_predecessor_and_conflicting_receipt_veto(self):
        for conflict in ('predecessor','receipt'):
            with self.subTest(conflict=conflict),tempfile.TemporaryDirectory() as td:
                installed,candidate=self.setup_code(Path(td))
                if conflict=='predecessor':(installed/'code'/'component').write_bytes(b'unapproved')
                else:(installed/'upgrade-v5.json').write_text('{}')
                with self.assertRaises(Blocked):self.upgrade(candidate,installed)
                self.assertFalse((installed/'code-v4-preserved').exists())
                self.assertFalse((installed/'code-v5-staged').exists())


if __name__=='__main__':unittest.main()
