import io
import json
from pathlib import Path
import tempfile
import unittest
import space_backup as backup

class SnapshotTests(unittest.TestCase):
    def test_dump_uses_retained_snapshot_and_same_observed_precount(self):
        before={'database':'space_db','version':15,'dirty':False,'allow_guests_true':2,'column_default':'true','column_type':'boolean'}
        events=[]
        class Backend:
            def begin(self):events.append('begin');return '00000003-0000001B-1',before
            def alive(self):return True
            def dump(self,snapshot,stream):
                self_test.assertEqual(snapshot,'00000003-0000001B-1')
                events.append('dump');stream.write(b'PGDMPtest-bytes')
            def close(self):events.append('close')
        self_test=self
        with tempfile.TemporaryDirectory() as directory:
            with backup.Snapshot(Backend()) as snapshot:
                receipt=snapshot.dump(Path(directory)/'space-before.dump',1024)
                self.assertEqual(receipt['before'],before)
                self.assertEqual(events,['begin','dump'])
            self.assertEqual(events,['begin','dump','close'])
            self.assertFalse(receipt['restored']);self.assertFalse(receipt['offnode_verified'])

    def test_expired_exporter_or_oversized_dump_does_not_create_valid_receipt(self):
        class Backend:
            def begin(self):return '00000003-0000001B-1',{'database':'space_db','version':15,'dirty':False,'allow_guests_true':0,'column_default':'true','column_type':'boolean'}
            def alive(self):return self.running
            def dump(self,snapshot,stream):stream.write(b'PGDMP'+b'x'*32)
            def close(self):self.closed=True
        for alive in (False,True):
            backend=Backend();backend.running=alive
            with self.subTest(alive=alive),tempfile.TemporaryDirectory() as directory:
                target=Path(directory)/'space-before.dump'
                with self.assertRaises(backup.BackupError):
                    with backup.Snapshot(backend) as snapshot:snapshot.dump(target,16)
                self.assertFalse(target.exists());self.assertTrue(backend.closed)
