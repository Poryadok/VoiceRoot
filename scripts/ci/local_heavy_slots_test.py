"""Cross-process limits for local heavy CI runs."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from local_heavy_slots import acquire


class HeavySlotsTest(unittest.TestCase):
    def test_simultaneous_first_start_initializes_distinct_slots(self):
        child = ('import sys; from pathlib import Path; from local_heavy_slots import acquire; '
                 'sys.stdin.readline(); lease=acquire(Path(sys.argv[1]), timeout=5); '
                 'print(lease.slot, flush=True); sys.stdin.readline(); lease.close()')
        env = dict(os.environ, PYTHONPATH=str(Path(__file__).resolve().parent))
        for _ in range(4):
            with tempfile.TemporaryDirectory() as directory:
                workers = [subprocess.Popen([sys.executable, '-u', '-c', child, directory], env=env,
                                            stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                            stderr=subprocess.PIPE, text=True) for _ in range(2)]
                try:
                    for process in workers:
                        process.stdin.write('\n')
                        process.stdin.flush()
                    self.assertEqual({process.stdout.readline().strip() for process in workers}, {'0', '1'})
                finally:
                    for process in workers:
                        if process.poll() is None:
                            process.stdin.write('\n')
                            process.stdin.flush()
                            process.wait(timeout=6)
                        process.stdin.close()
                        process.stdout.close()
                        process.stderr.close()

    def test_invalid_timeout_and_unknown_marker_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for invalid in (-1, float('nan'), float('inf'), True):
                with self.subTest(timeout=invalid), self.assertRaises(ValueError):
                    acquire(root, timeout=invalid)
            (root / 'heavy-0.lock').write_bytes(b'\0unknown\n')
            lease = acquire(root, timeout=0)
            try:
                self.assertEqual(lease.slot, 1)
                with self.assertRaises(TimeoutError):
                    acquire(root, timeout=0)
            finally:
                lease.close()

    def test_exception_retains_slot_for_recovery(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaisesRegex(RuntimeError, 'interrupted'):
                with acquire(root, timeout=0):
                    raise RuntimeError('interrupted')
            with acquire(root, timeout=0):
                try:
                    unexpected = acquire(root, timeout=0)
                except TimeoutError:
                    pass
                else:
                    unexpected.close()
                    self.fail('exception released a slot without cleanup')

    def test_two_processes_exclude_third_and_crash_retains_its_slot(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            child = ('import sys; from pathlib import Path; from local_heavy_slots import acquire; '
                     'lease=acquire(Path(sys.argv[1]), timeout=5); print("acquired", flush=True); '
                     'sys.stdin.readline(); lease.close()')
            env = dict(os.environ)
            env['PYTHONPATH'] = str(Path(__file__).resolve().parent)
            workers = []
            def assert_unavailable():
                try:
                    unexpected = acquire(root, timeout=0.2)
                except TimeoutError:
                    return
                unexpected.close()
                self.fail('an occupied or abandoned slot was reused')
            try:
                for _ in range(2):
                    process = subprocess.Popen([sys.executable, '-u', '-c', child, str(root)],
                                               env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                               stderr=subprocess.PIPE, text=True)
                    workers.append(process)
                    self.assertEqual(process.stdout.readline().strip(), 'acquired')
                assert_unavailable()
                workers[0].kill()
                workers[0].wait(timeout=5)
                self.assertIsNone(workers[1].poll())
                assert_unavailable()
                workers[1].stdin.write('\n')
                workers[1].stdin.flush()
                workers[1].wait(timeout=5)
                lease = acquire(root, timeout=2)
                try:
                    assert_unavailable()
                finally:
                    lease.close()
            finally:
                for process in workers:
                    if process.poll() is None:
                        process.stdin.write('\n')
                        process.stdin.flush()
                        process.wait(timeout=5)
                    process.stdin.close()
                    process.stdout.close()
                    process.stderr.close()


if __name__ == '__main__':
    unittest.main()
