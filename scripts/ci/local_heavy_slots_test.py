"""Cross-process limits for local heavy CI runs."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

from local_heavy_slots import acquire, host_slot_directory


class HeavySlotsTest(unittest.TestCase):
    def test_host_slot_directory_is_absolute_and_not_selected_by_environment(self):
        expected = host_slot_directory()
        self.assertTrue(expected.is_absolute())
        self.assertEqual(expected.name, 'voice-ci-heavy-slots-v1')
        with mock.patch.dict(os.environ, {'TMP': 'different-tmp', 'TEMP': 'different-temp',
                                        'PROGRAMDATA': 'different-data'}):
            self.assertEqual(host_slot_directory(), expected)

    def test_capacity_is_shared_and_conflicting_configuration_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            leases = [acquire(root, timeout=0, capacity=3) for _ in range(3)]
            try:
                self.assertEqual({lease.slot for lease in leases}, {0, 1, 2})
                with self.assertRaises(TimeoutError):
                    acquire(root, timeout=0, capacity=3)
                with self.assertRaisesRegex(ValueError, 'capacity'):
                    acquire(root, timeout=0, capacity=2)
            finally:
                for lease in leases:
                    lease.close()

    def test_invalid_capacity_fails_without_allocating_slots(self):
        with tempfile.TemporaryDirectory() as directory:
            for invalid in (0, -1, True, 2.5, 1000):
                with self.subTest(capacity=invalid), self.assertRaises(ValueError):
                    acquire(directory, timeout=0, capacity=invalid)
            self.assertEqual(list(Path(directory).iterdir()), [])

    def test_concurrent_conflicting_capacity_initialization_has_one_winner(self):
        child = '''import sys
from local_heavy_slots import acquire
sys.stdin.readline()
try:
    lease = acquire(sys.argv[1], timeout=5, capacity=int(sys.argv[2]))
except ValueError:
    print('rejected', flush=True)
else:
    print('acquired', flush=True)
    sys.stdin.readline()
    lease.close()
'''
        env = dict(os.environ, PYTHONPATH=str(Path(__file__).resolve().parent))
        for _ in range(3):
            with tempfile.TemporaryDirectory() as directory:
                workers = [subprocess.Popen([sys.executable, '-u', '-c', child, directory, str(capacity)],
                                            env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                            stderr=subprocess.PIPE, text=True) for capacity in (1, 3)]
                try:
                    for process in workers:
                        process.stdin.write('\n')
                        process.stdin.flush()
                    self.assertEqual(sorted(process.stdout.readline().strip() for process in workers),
                                     ['acquired', 'rejected'])
                finally:
                    for process in workers:
                        if process.poll() is None:
                            process.stdin.write('\n')
                            process.stdin.flush()
                            process.wait(timeout=6)
                        process.stdin.close()
                        process.stdout.close()
                        process.stderr.close()

    def test_invalid_capacity_file_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            (Path(directory) / 'capacity').write_bytes(b'invalid\n')
            with self.assertRaisesRegex(ValueError, 'capacity'):
                acquire(directory, timeout=0)

    def test_incomplete_capacity_lock_initialization_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            (Path(directory) / 'capacity.lock').write_bytes(b'')
            with self.assertRaisesRegex(ValueError, 'initialization incomplete'):
                acquire(directory, timeout=0)

    def test_initial_capacity_does_not_hide_existing_slots(self):
        with tempfile.TemporaryDirectory() as directory:
            (Path(directory) / 'heavy-2.lock').write_bytes(b'\0busy:orphan\n')
            try:
                lease = acquire(directory, timeout=0, capacity=2)
            except ValueError as error:
                self.assertIn('capacity', str(error))
            else:
                lease.close()
                self.fail('capacity hid an existing slot')

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
