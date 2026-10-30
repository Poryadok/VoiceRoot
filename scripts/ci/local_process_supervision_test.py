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
    def test_atomic_job_launch_is_contained_before_first_instruction(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / 'survived'
            kernel, job, accounting = supervision._windows_job()
            process = None
            try:
                process = supervision._launch_atomic_windows(
                    kernel, job, accounting, [sys.executable, '-c', 'import pathlib,sys,time; time.sleep(2); pathlib.Path(sys.argv[1]).write_text("alive")', str(marker)],
                    None, None, None, None)
                supervision._settle_windows(kernel, job, accounting, time.monotonic() + 5)
                process.wait(timeout=5)
            finally:
                kernel.CloseHandle(job)
                if process is not None:
                    process.close()
            time.sleep(2.5)
            self.assertFalse(marker.exists())

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_atomic_launcher_crash_immediately_after_create_kills_child(self):
        import ctypes
        from ctypes import wintypes
        with tempfile.TemporaryDirectory() as directory:
            ready, release = Path(directory) / 'ready', Path(directory) / 'release'
            child = ('import os,pathlib,sys,time; pathlib.Path(sys.argv[1]).write_text(str(os.getpid())); '
                     'time.sleep(20)')
            harness = ('import os,pathlib,sys,time; sys.path.insert(0,sys.argv[1]); '
                       'import local_process_supervision as s; '
                       'exec("def hook(info):\\n p=pathlib.Path(sys.argv[4])\\n while not p.exists(): time.sleep(.02)\\n os._exit(42)"); '
                       's._atomic_created_hook=hook; k,j,a=s._windows_job(); '
                       's._launch_atomic_windows(k,j,a,[sys.executable,"-c",sys.argv[2],sys.argv[3]],None,None,None,None)')
            owner = subprocess.Popen([sys.executable, '-B', '-c', harness, str(Path(__file__).parent),
                                      child, str(ready), str(release)])
            kernel = ctypes.WinDLL('kernel32', use_last_error=True)
            kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
            kernel.OpenProcess.restype = wintypes.HANDLE
            kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
            kernel.CloseHandle.argtypes = [wintypes.HANDLE]
            handle = None
            try:
                deadline = time.monotonic() + 5
                while not ready.exists() and time.monotonic() < deadline:
                    time.sleep(0.02)
                self.assertTrue(ready.exists())
                handle = kernel.OpenProcess(0x100000, False, int(ready.read_text()))
                self.assertTrue(handle)
                release.write_text('release')
                self.assertEqual(owner.wait(timeout=5), 42)
                self.assertEqual(kernel.WaitForSingleObject(handle, 5000), 0)
            finally:
                if owner.poll() is None:
                    owner.terminate()
                    owner.wait(timeout=5)
                if handle:
                    kernel.CloseHandle(handle)

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_atomic_launcher_forwards_cwd_environment_and_separate_output(self):
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            kernel, job, accounting = supervision._windows_job()
            process = None
            try:
                env = dict(os.environ, VOICE_PROCESS_TEST='значение', PYTHONIOENCODING='utf-8')
                process = supervision._launch_atomic_windows(
                    kernel, job, accounting, [sys.executable, '-c',
                                  'import os,sys; print(os.getcwd()); sys.stderr.buffer.write(os.environ["VOICE_PROCESS_TEST"].encode()); print(sys.argv[1])',
                                  'аргумент with spaces'],
                    directory, env, out, err)
                self.assertEqual(process.wait(timeout=5), 0)
                supervision._settle_windows(kernel, job, accounting, time.monotonic() + 5)
                out.seek(0); err.seek(0)
                lines = out.read().decode('utf-8').splitlines()
                self.assertEqual(Path(lines[0]), Path(directory))
                self.assertEqual(lines[1], 'аргумент with spaces')
                self.assertEqual(err.read().decode('utf-8'), 'значение')
            finally:
                kernel.CloseHandle(job)
                if process is not None:
                    process.close()

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_batch_launch_restricts_shell_metacharacters(self):
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryFile() as out:
            batch = Path(directory) / 'flutter fixture.cmd'
            batch.write_text('@echo off\r\necho %~1\r\n', encoding='ascii')
            kernel, job, accounting = supervision._windows_job()
            process = None
            try:
                process = supervision._launch_atomic_windows(kernel, job, accounting,
                    [str(batch), 'safe argument'], directory, None, out, None)
                self.assertEqual(process.wait(timeout=5), 0)
                supervision._settle_windows(kernel, job, accounting, time.monotonic() + 5)
                out.seek(0)
                self.assertEqual(out.read().decode().strip(), 'safe argument')
                with self.assertRaisesRegex(supervision.SupervisionError, 'unsafe batch'):
                    supervision._launch_atomic_windows(kernel, job, accounting,
                        [str(batch), 'x&echo injected'], directory, None, out, None)
            finally:
                kernel.CloseHandle(job)
                if process is not None:
                    process.close()

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_atomic_post_create_exception_settles_and_closes_handles(self):
        import ctypes
        from ctypes import wintypes
        kernel, job, accounting = supervision._windows_job()
        handles = []
        def fail(info):
            handles.extend((info.hProcess, info.hThread))
            raise ValueError('injected post-create error')
        try:
            with patch.object(supervision, '_atomic_created_hook', side_effect=fail), \
                 self.assertRaisesRegex(ValueError, 'injected'):
                supervision._launch_atomic_windows(kernel, job, accounting,
                    [sys.executable, '-c', 'import time; time.sleep(8)'], None, None, None, None)
            info = accounting()
            self.assertTrue(kernel.QueryInformationJobObject(job, 1, ctypes.byref(info), ctypes.sizeof(info), None))
            self.assertEqual(info.ActiveProcesses, 0)
            flags = wintypes.DWORD()
            kernel.GetHandleInformation.argtypes = [wintypes.HANDLE, ctypes.POINTER(wintypes.DWORD)]
            for handle in handles:
                self.assertFalse(kernel.GetHandleInformation(handle, ctypes.byref(flags)))
        finally:
            kernel.CloseHandle(job)

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_atomic_timeout_reaps_ready_grandchild_and_preserves_sentinel(self):
        import ctypes
        from ctypes import wintypes
        with tempfile.TemporaryDirectory() as directory:
            ready = Path(directory) / 'ready'
            grandchild = 'import os,pathlib,sys,time; pathlib.Path(sys.argv[1]).write_text(str(os.getpid())); time.sleep(20)'
            child = 'import subprocess,sys,time; subprocess.Popen([sys.executable,"-c",sys.argv[1],sys.argv[2]]); time.sleep(20)'
            sentinel = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(20)'])
            kernel, job, accounting = supervision._windows_job()
            kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
            kernel.OpenProcess.restype = wintypes.HANDLE
            process = handle = None
            try:
                process = supervision._launch_atomic_windows(kernel, job, accounting,
                    [sys.executable, '-c', child, grandchild, str(ready)], None, None, None, None)
                deadline = time.monotonic() + 5
                while not ready.exists() and time.monotonic() < deadline:
                    time.sleep(.02)
                self.assertTrue(ready.exists())
                handle = kernel.OpenProcess(0x100000, False, int(ready.read_text()))
                self.assertTrue(handle)
                with self.assertRaises(subprocess.TimeoutExpired):
                    process.wait(timeout=.1)
                supervision._settle_windows(kernel, job, accounting, time.monotonic() + 5)
                process.wait(timeout=5)
                self.assertEqual(kernel.WaitForSingleObject(handle, 5000), 0)
                self.assertIsNone(sentinel.poll())
            finally:
                kernel.CloseHandle(job)
                if handle:
                    kernel.CloseHandle(handle)
                if process:
                    process.close()
                sentinel.terminate()
                sentinel.wait(timeout=5)

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
