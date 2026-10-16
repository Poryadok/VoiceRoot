import os
from pathlib import Path
import tempfile
import unittest
from bootstrap_renewal import invoke,RenewalError

@unittest.skipUnless(os.name=='posix','private inherited fd transport requires Linux')
class Tests(unittest.TestCase):
    def executable(self,body):
        td=tempfile.TemporaryDirectory();self.addCleanup(td.cleanup)
        path=Path(td.name)/'synthetic-issuer'
        path.write_text('#!/usr/bin/python3\nimport os,sys,json,time\njson.load(sys.stdin)\nfd=int(sys.argv[2])\n'+body+'\n');path.chmod(0o700)
        return path
    def test_large_private_result_drains_while_child_runs(self):
        result=invoke(self.executable('os.write(fd,b"x"*240000)'),{'synthetic':True},timeout=5)
        self.assertEqual(result,b'x'*240000)
    def test_oversize_private_result_refused(self):
        with self.assertRaises(RenewalError):invoke(self.executable('os.write(fd,b"x"*262145)'),{},timeout=5)
    def test_stalled_child_bounded_and_sanitized(self):
        started=__import__('time').monotonic()
        with self.assertRaisesRegex(RenewalError,'^existing_bootstrap_private_renewal_refused$'):invoke(self.executable('time.sleep(10)'),{},timeout=0.3)
        self.assertLess(__import__('time').monotonic()-started,3)
if __name__=='__main__':unittest.main()
