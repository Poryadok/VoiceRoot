from pathlib import Path
import sys
import tempfile
import time
import unittest

from commands import capture
from controller import Blocked


class CommandBoundsTests(unittest.TestCase):
    def test_output_limit_kills_before_later_side_effect(self):
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'late'
            with self.assertRaises(Blocked):
                capture([sys.executable,'-I','-S','-c',
                    'import time;from pathlib import Path;print("x"*65536,flush=True);time.sleep(0.3);Path('+repr(str(path))+').touch()'],limit=16)
            self.assertFalse(path.exists())

    def test_timeout_kills_and_returns_within_bound(self):
        started=time.monotonic()
        with self.assertRaises(Blocked):
            capture([sys.executable,'-I','-S','-c','import time;time.sleep(30)'],timeout=0.1)
        self.assertLess(time.monotonic()-started,2)

    def test_bounded_stdin_and_json_output(self):
        output=capture([sys.executable,'-I','-S','-c','import sys;print(len(sys.stdin.buffer.read()))'],body=b'known')
        self.assertEqual(output,b'5\n')


if __name__=='__main__':unittest.main()
