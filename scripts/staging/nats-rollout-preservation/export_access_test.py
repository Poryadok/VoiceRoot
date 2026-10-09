"""Real filesystem export modes; physical UID1000 proof is a separate fixture."""
import hashlib
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import bridge_root


class ExportAccessTests(unittest.TestCase):
    def test_cipher_export_explicit_modes_under_service_and_normal_umask(self):
        for mask in (0o022,0o077):
            with self.subTest(umask=oct(mask)),tempfile.TemporaryDirectory() as directory:
                base=Path(directory)/'operation';base.mkdir(mode=0o700)
                private=base/'rollout-backup.cms';private.write_bytes(b'synthetic ciphertext');private.chmod(0o600)
                before=private.stat();digest=hashlib.sha256(private.read_bytes()).hexdigest()
                state={'operation':'3340764a7d24','cut':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64,'census_sha256':'c'*64}}
                actions=object.__new__(bridge_root.Actions)
                old=os.umask(mask)
                try:
                    with patch.object(bridge_root.encrypted_cut,'encrypt_cut',return_value={'cipher_sha256':digest,'cipher_bytes':private.stat().st_size}),\
                         patch.object(bridge_root.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)),\
                         patch.object(bridge_root.os,'chown') as ownership,patch.object(bridge_root,'save'),\
                         patch.object(actions,'copy_result',side_effect=lambda base,state:{'cipher_path':str(base/'export/rollout-backup.cms')}):
                        result=actions._encrypt(base,state,{'dispatcher_run_id':37900273078,'head_sha':'8'*40})
                finally:os.umask(old)
                exported=Path(result['cipher_path'])
                self.assertEqual(exported.parent.stat().st_mode&0o777,0o750)
                self.assertEqual(exported.stat().st_mode&0o777,0o440)
                self.assertEqual(base.stat().st_mode&0o777,0o750)
                ownership.assert_any_call(exported.parent,0,1000)
                ownership.assert_any_call(exported,0,1000)
                self.assertEqual(private.stat().st_mode&0o777,0o600)
                self.assertEqual(private.stat().st_ino,before.st_ino)
                self.assertEqual(hashlib.sha256(private.read_bytes()).hexdigest(),digest)
                self.assertEqual(hashlib.sha256(exported.read_bytes()).hexdigest(),digest)


if __name__=='__main__':unittest.main()
