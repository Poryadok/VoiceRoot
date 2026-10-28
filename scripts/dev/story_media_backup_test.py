import copy
import contextlib
import io
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import story_media_backup as backup


class EncryptedBackupTest(unittest.TestCase):
    def test_unsafe_and_symlink_directories_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            unsafe = root / 'unsafe'
            unsafe.mkdir(mode=0o777)
            unsafe.chmod(0o777)
            link = root / 'link'
            link.symlink_to(unsafe, target_is_directory=True)
            for candidate in (unsafe, link):
                with self.subTest(candidate=candidate), self.assertRaises(backup.BackupError):
                    backup.export(candidate, bytes(32), {'operation': 'fixture'}, lambda n, o: o.write(b'x'))

    def test_replaced_created_member_is_not_signed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def producer(name, output):
                output.write(b'fixture')
                if name == 'file_db':
                    os.rename(root / 'file_db.enc', root / 'detached')
                    replacement = root / 'file_db.enc'
                    replacement.write_bytes(b'replacement')
                    replacement.chmod(0o600)
            with self.assertRaises(backup.BackupError):
                backup.export(root, bytes(32), {'operation': 'fixture'}, producer)

    def test_replacement_after_verify_is_rejected_before_plaintext(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            key = bytes(32)
            binding = {'operation': 'fixture'}
            manifest = backup.export(root, key, binding, lambda n, o: o.write(b'private fixture'))
            verify = backup._verify
            def replaced(dirfd, key, binding, manifest):
                result = verify(dirfd, key, binding, manifest)
                (root / 'file_db.enc').write_bytes(b'replacement')
                return result
            output = io.BytesIO()
            with patch.object(backup, '_verify', side_effect=replaced):
                with self.assertRaises(backup.BackupError):
                    backup.decrypt(root, key, binding, manifest, 'file_db', output)
            self.assertEqual(output.getvalue(), b'')

    def test_failed_producer_cannot_publish_manifest_or_leak_error(self):
        with tempfile.TemporaryDirectory() as directory:
            def fail(name, out):
                out.write(b'private fixture')
                raise RuntimeError('secret-producer-error')
            with self.assertRaises(backup.BackupError) as error:
                backup.export(Path(directory), bytes(32), {'operation': 'fixture'}, fail)
            self.assertNotIn('secret-producer-error', str(error.exception))

    def test_invalid_producer_is_rejected_without_creating_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaises(backup.BackupError):
                backup.export(root, bytes(32), {'operation': 'fixture'}, None)
            self.assertEqual(list(root.iterdir()), [])

    def test_ciphertext_and_binding_must_authenticate_before_decryption(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            key = bytes(range(32))
            binding = {'operation': 'fixture-operation', 'source_sha': 'a' * 40}
            manifest = backup.export(root, key, binding, lambda name, out: out.write(b'private fixture'))
            self.assertEqual(backup.verify(root, key, binding, manifest), backup.MEMBERS)
            restored = io.BytesIO()
            backup.decrypt(root, key, binding, manifest, 'file_db', restored)
            self.assertEqual(restored.getvalue(), b'private fixture')
            for change in ('key', 'binding', 'manifest', 'version', 'missing', 'ciphertext', 'truncated'):
                with self.subTest(change=change):
                    altered = copy.deepcopy(manifest)
                    expected = copy.deepcopy(binding)
                    candidate_key = key
                    path = root / 'file_db.enc'
                    original = path.read_bytes()
                    if change == 'key': candidate_key = bytes(reversed(key))
                    if change == 'binding': expected['operation'] = 'other'
                    if change == 'manifest': altered['binding']['operation'] = 'other'
                    if change == 'version': altered['format'] = 2
                    if change == 'missing': del altered['files']['roles']
                    if change == 'ciphertext': path.write_bytes(original[:-1] + bytes([original[-1] ^ 1]))
                    if change == 'truncated': path.write_bytes(original[:-1])
                    try:
                        with self.assertRaises(backup.BackupError):
                            restored = io.BytesIO()
                            backup.decrypt(root, candidate_key, expected, altered, 'file_db', restored)
                        self.assertEqual(restored.getvalue(), b'')
                    finally:
                        path.write_bytes(original)

    def test_authenticated_but_invalid_ciphertext_fails_without_secret_diagnostics(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            key = bytes(range(32))
            binding = {'operation': 'fixture-operation'}
            manifest = backup.export(root, key, binding, lambda name, out: out.write(b'fixture'))
            path = root / 'file_db.enc'
            path.write_bytes(b'invalid ciphertext with fixture secret')
            manifest['files']['file_db'] = backup.identity(path, key)
            unsigned = {name: manifest[name] for name in ('format', 'binding', 'files')}
            manifest['mac'] = backup.manifest_mac(key, unsigned)
            stdout, stderr = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                with self.assertRaises(backup.BackupError) as error:
                    backup.decrypt(root, key, binding, manifest, 'file_db', io.BytesIO())
            self.assertNotIn('fixture secret', str(error.exception) + stdout.getvalue() + stderr.getvalue())


if __name__ == '__main__':
    unittest.main()
