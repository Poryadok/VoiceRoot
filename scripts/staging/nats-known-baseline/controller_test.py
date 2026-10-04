import io
import json
import os
from pathlib import Path
import tempfile
import tarfile
import unittest

import controller


class ColdArchiveTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.source = self.base / 'owned-closed-store'
        self.source.mkdir()
        (self.source / 'jetstream').mkdir()
        (self.source / 'jetstream' / 'meta.inf').write_bytes(b'controlled-native-metadata')
        self.archive = self.base / 'closed.tar'

    def test_closed_copy_exact_bytes_and_hashes(self):
        manifest = controller.archive_closed_store(self.source, self.archive)
        target = self.base / 'restore'
        controller.restore_closed_store(self.archive, target, manifest)
        self.assertEqual((target / 'jetstream' / 'meta.inf').read_bytes(), b'controlled-native-metadata')
        self.assertEqual(manifest['file_count'], 1)
        self.assertEqual(manifest['bytes'], 26)
        self.assertTrue(controller.verify_archive(self.archive, manifest))

    def test_symlink_never_read(self):
        (self.source / 'jetstream' / 'external').symlink_to('/etc/passwd')
        with self.assertRaises(controller.Blocked):
            controller.archive_closed_store(self.source, self.archive)

    def test_fifo_never_opened_blocking(self):
        os.mkfifo(self.source / 'jetstream' / 'block')
        with self.assertRaises(controller.Blocked):
            controller.archive_closed_store(self.source, self.archive)

    def test_credentials_not_archived(self):
        (self.source / 'social.creds').write_bytes(b'not-archive-input')
        with self.assertRaises(controller.Blocked):
            controller.archive_closed_store(self.source, self.archive)

    def test_tampered_archive_not_extracted(self):
        manifest = controller.archive_closed_store(self.source, self.archive)
        with self.archive.open('ab') as output:
            output.write(b'changed')
        target = self.base / 'restore'
        with self.assertRaises(controller.Blocked):
            controller.restore_closed_store(self.archive, target, manifest)
        self.assertFalse(target.exists())

    def test_untrusted_tar_path_cannot_escape(self):
        manifest = controller.archive_closed_store(self.source, self.archive)
        with tarfile.open(self.archive, 'w') as archive:
            info = tarfile.TarInfo('../escape')
            info.size = 1
            archive.addfile(info, io.BytesIO(b'x'))
        manifest['archive_sha256'] = controller.file_sha(self.archive)
        with self.assertRaises(controller.Blocked):
            controller.restore_closed_store(self.archive, self.base / 'restore', manifest)
        self.assertFalse((self.base / 'escape').exists())

    def test_fixed_bounds_fail_no_partial_success(self):
        with self.assertRaises(controller.Blocked):
            controller.archive_closed_store(self.source, self.archive, max_bytes=1)


if __name__ == '__main__':
    unittest.main()
