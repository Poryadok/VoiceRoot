import copy
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch
import preserve


class PreservationTest(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.base=Path(self.temp.name);self.source=self.base/'source';self.source.mkdir()
        (self.source/'records.blk').write_bytes(b'known synthetic record bytes')
        self.runtime=type('Runtime',(),{'base':self.base,'no_operation_containers':lambda self,**kw:None})()
        self.row={'streams':[{'messages':3,'first_seq':1,'last_seq':3}],
                  'consumers':[{'ack_stream':1,'ack_consumer':1,'ack_pending':1,'pending':1,'delivered_stream':2,'delivered_consumer':2}]}
        self.fences=[]
    def fence(self):self.fences.append('verified')
    def capture(self):
        with patch.object(preserve,'isolated_census',return_value=copy.deepcopy(self.row)):
            return preserve.capture_cut(self.runtime,self.source,self.fence)
    def test_populated_cut_and_post_apply_preserve_pending_and_ack(self):
        cut=self.capture()
        with patch.object(preserve,'isolated_census',return_value=copy.deepcopy(self.row)):
            result=preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
        self.assertEqual(result['messages'],3);self.assertTrue(result['census_verified'])
        self.assertEqual(cut['census']['consumers'][0]['ack_pending'],1)
        self.assertEqual(len(self.fences),5)
    def test_retry_keeps_each_immutable_post_apply_archive(self):
        cut=self.capture()
        with patch.object(preserve,"isolated_census",return_value=copy.deepcopy(self.row)):
            first=preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
            second=preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
        self.assertNotEqual(first["archive"],second["archive"])
        self.assertTrue((self.base/first["archive"]).is_file())
        self.assertTrue((self.base/second["archive"]).is_file())

    def test_native_record_loss_blocks_before_any_census(self):
        cut=self.capture();(self.source/'records.blk').write_bytes(b'')
        with patch.object(preserve,'isolated_census') as read:
            with self.assertRaisesRegex(preserve.Blocked,'native_store_changed'):
                preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
            read.assert_not_called()
    def test_ack_cursor_change_blocks_resume(self):
        cut=self.capture();changed=copy.deepcopy(self.row);changed['consumers'][0]['ack_stream']=2
        with patch.object(preserve,'isolated_census',return_value=changed):
            with self.assertRaisesRegex(preserve.Blocked,'populated_census_changed'):
                preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)
    def test_cut_digest_tamper_blocks_resume(self):
        cut=self.capture();cut['census']['consumers'][0]['pending']=0
        with patch.object(preserve,'isolated_census',return_value=copy.deepcopy(self.row)):
            with self.assertRaisesRegex(preserve.Blocked,'populated_census_changed'):
                preserve.verify_post_apply(self.runtime,self.source,cut,self.fence)

if __name__=='__main__':unittest.main()
