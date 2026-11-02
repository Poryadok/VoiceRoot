"""Private key creation is exclusive and never replaces existing data."""
import os
from pathlib import Path
import stat
import tempfile
import unittest

import full_cohort_backup_phase as phase
import story_media_backup as backup


class KeyCustodyTest(unittest.TestCase):
    def test_exclusive_private_key_and_no_overwrite(self):
        with tempfile.TemporaryDirectory() as root:
            path=Path(root);value=phase.BackupPhase._write_key(path);key=path/'backup.key'
            self.assertEqual(len(value),32);self.assertEqual(key.read_bytes(),value)
            self.assertEqual(stat.S_IMODE(key.stat().st_mode),0o600)
            with self.assertRaises((FileExistsError,backup.BackupError)):phase.BackupPhase._write_key(path)
            self.assertEqual(key.read_bytes(),value)

    def test_symlink_key_is_not_followed(self):
        with tempfile.TemporaryDirectory() as root:
            path=Path(root);target=path/'untouched';target.write_bytes(b'keep')
            os.symlink(target,path/'backup.key')
            with self.assertRaises((OSError,backup.BackupError)):phase.BackupPhase._write_key(path)
            self.assertEqual(target.read_bytes(),b'keep')


if __name__=='__main__':unittest.main()
