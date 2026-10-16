import os
from pathlib import Path
import tempfile
import unittest
import native_store
from controller import Blocked

class NativeTests(unittest.TestCase):
    def test_streaming_store_larger_than_reset_fixture_roundtrips(self):
        with tempfile.TemporaryDirectory() as directory:
            base=Path(directory);source=base/'source';source.mkdir()
            with (source/'records.blk').open('wb') as stream:stream.truncate(65<<20)
            manifest=native_store.archive_closed_store(source,base/'backup.tar')
            self.assertEqual(manifest['bytes'],65<<20)
            native_store.restore_closed_store(base/'backup.tar',base/'restored',manifest)
            self.assertEqual((base/'restored/records.blk').stat().st_size,65<<20)
            self.assertTrue(native_store.verify_archive(base/'backup.tar',manifest))

    def test_dynamic_native_entry_count_exceeds_reset_8192(self):
        with tempfile.TemporaryDirectory() as directory:
            base=Path(directory);source=base/'source';source.mkdir()
            for number in range(8193):(source/('consumer-'+str(number))).touch()
            manifest=native_store.archive_closed_store(source,base/'backup.tar')
            self.assertEqual(manifest['file_count'],8193)
            self.assertTrue(native_store.verify_archive(base/'backup.tar',manifest))

    def test_capacity_and_disk_policy_veto_before_fence(self):
        with tempfile.TemporaryDirectory() as directory:
            base=Path(directory);source=base/'source';source.mkdir();(source/'data').write_bytes(b'x')
            self.assertEqual(native_store.preflight(source,base,'20Gi')['files'],1)
            with self.assertRaises(Blocked):native_store.preflight(source,base,'64Gi')
            with self.assertRaises(Blocked):native_store.preflight(source,base,'20Gi',free_bytes=1)
            (source/'unsafe').symlink_to(source/'data')
            with self.assertRaises(Blocked):native_store.preflight(source,base,'20Gi')
