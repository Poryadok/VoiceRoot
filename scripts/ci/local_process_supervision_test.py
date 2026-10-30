"""Owned process supervision tests; no foreign process is terminated."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

import local_process_supervision as supervision


class SupervisionTest(unittest.TestCase):
    def test_invalid_timeout_refuses_before_launch(self):
        with self.assertRaises(ValueError):
            supervision.run([sys.executable, '-c', 'raise SystemExit(0)'], timeout=0)

    def test_unsupported_platform_refuses_before_launch(self):
        with self.assertRaises(supervision.SupervisionError):
            supervision.run(['never-executed'], timeout=1, platform='unsupported')

    def test_linux_without_delegated_cgroup_refuses_before_launch(self):
        with self.assertRaises(supervision.SupervisionError):
            supervision.run(['never-executed'], timeout=1, platform='linux')

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_default_refuses_unproven_crash_window(self):
        with self.assertRaisesRegex(supervision.SupervisionError, 'atomic'):
            supervision.run(['never-executed'], timeout=1)

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_assignment_resume_and_interrupt_reap_owned_child(self):
        for point in ('assignment', 'resume', 'interrupt'):
            with self.subTest(point=point):
                seen = []
                real_assign = supervision._assign_windows
                def assign(kernel, job, process):
                    seen.append(process)
                    return False if point == 'assignment' else real_assign(kernel, job, process)
                def resume(process):
                    if point == 'resume':
                        return -1
                    return real_resume(process)
                real_resume = supervision._resume_windows
                def wait(process, timeout):
                    if point == 'interrupt':
                        raise KeyboardInterrupt()
                    process.wait(timeout=timeout)
                expected = KeyboardInterrupt if point == 'interrupt' else supervision.SupervisionError
                with patch.object(supervision, '_assign_windows', side_effect=assign), \
                     patch.object(supervision, '_resume_windows', side_effect=resume), \
                     patch.object(supervision, '_wait_windows', side_effect=wait), \
                     self.assertRaises(expected):
                    supervision.run([sys.executable, '-c', 'import time; time.sleep(8)'], timeout=2,
                                    allow_unverified_crash_window=True)
                self.assertEqual(len(seen), 1)
                self.assertIsNotNone(seen[0].poll())

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_execution_and_cleanup_failures_are_both_preserved(self):
        with patch.object(supervision, '_wait_windows', side_effect=KeyboardInterrupt()), \
             patch.object(supervision, '_settle_windows', side_effect=supervision.SupervisionError('cleanup evidence missing')):
            with self.assertRaises(BaseExceptionGroup) as caught:
                supervision.run([sys.executable, '-c', 'import time; time.sleep(8)'], timeout=2,
                                allow_unverified_crash_window=True)
        self.assertIsInstance(caught.exception.exceptions[0], KeyboardInterrupt)
        self.assertIn('cleanup evidence missing', str(caught.exception.exceptions[1]))

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_timeout_kills_child_and_grandchild_without_touching_sentinel(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / 'grandchild-survived'
            ready = Path(directory) / 'grandchild-ready'
            child = ('import subprocess,sys,time; '
                     'subprocess.Popen([sys.executable,"-c",sys.argv[1],sys.argv[2],sys.argv[3]]); '
                     'time.sleep(8)')
            grandchild = ('import pathlib,sys,time; pathlib.Path(sys.argv[2]).write_text("ready"); '
                          'time.sleep(2); pathlib.Path(sys.argv[1]).write_text("alive")')
            sentinel = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(8)'])
            try:
                with self.assertRaises(subprocess.TimeoutExpired):
                    supervision.run([sys.executable, '-c', child, grandchild, str(marker), str(ready)], timeout=1.2,
                                    allow_unverified_crash_window=True)
                self.assertTrue(ready.exists(), 'grandchild must have started before timeout')
                self.assertIsNone(sentinel.poll())
                time.sleep(2.5)
                self.assertFalse(marker.exists())
                self.assertIsNone(sentinel.poll())
            finally:
                sentinel.terminate()
                sentinel.wait(timeout=5)

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_completed_parent_cannot_leave_live_descendant(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / 'survived'
            child = ('import subprocess,sys; subprocess.Popen([sys.executable,"-c",sys.argv[1],sys.argv[2]])')
            grandchild = 'import pathlib,sys,time; time.sleep(2); pathlib.Path(sys.argv[1]).write_text("alive")'
            result = supervision.run([sys.executable, '-c', child, grandchild, str(marker)], timeout=4,
                                     allow_unverified_crash_window=True)
            self.assertEqual(result.returncode, 0)
            time.sleep(2.5)
            self.assertFalse(marker.exists())

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_supervisor_crash_closes_job_and_kills_grandchild(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / 'survived'
            ready = Path(directory) / 'ready'
            grandchild = ('import pathlib,sys,time; pathlib.Path(sys.argv[2]).write_text("ready"); '
                          'time.sleep(3); pathlib.Path(sys.argv[1]).write_text("alive")')
            child = ('import pathlib,subprocess,sys,time; '
                     'subprocess.Popen([sys.executable,"-c",sys.argv[1],sys.argv[2],sys.argv[3]]); '
                     'p=pathlib.Path(sys.argv[3]); '
                     'exec("while not p.exists(): time.sleep(0.02)")')
            harness = ('import os,sys; sys.path.insert(0,sys.argv[1]); import local_process_supervision as s; '
                       's._settle_windows=lambda *args: os._exit(42); '
                       's.run([sys.executable,"-c",sys.argv[2],sys.argv[3],sys.argv[4],sys.argv[5]],timeout=5,allow_unverified_crash_window=True)')
            outcome = subprocess.run([sys.executable, '-B', '-c', harness, str(Path(__file__).parent), child,
                                      grandchild, str(marker), str(ready)], timeout=10)
            self.assertEqual(outcome.returncode, 42)
            self.assertTrue(ready.exists())
            time.sleep(3.5)
            self.assertFalse(marker.exists())


if __name__ == '__main__':
    unittest.main()
