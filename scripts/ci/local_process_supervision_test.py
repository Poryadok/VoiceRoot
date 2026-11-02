"""Owned process supervision tests; no foreign process is terminated."""
import os
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time
import unittest
import uuid
from unittest.mock import patch

import local_process_supervision as supervision


class SupervisionTest(unittest.TestCase):
    def test_job_limits_are_explicit_positive_and_within_windows_ranges(self):
        limits = supervision.JobLimits(cpu_rate=2500, job_memory_bytes=1024 * 1024,
                                       active_process_limit=4)
        self.assertEqual((limits.cpu_rate, limits.job_memory_bytes,
                          limits.active_process_limit), (2500, 1024 * 1024, 4))
        for values in (
                {'cpu_rate': 0, 'job_memory_bytes': 1024, 'active_process_limit': 1},
                {'cpu_rate': 10001, 'job_memory_bytes': 1024, 'active_process_limit': 1},
                {'cpu_rate': True, 'job_memory_bytes': 1024, 'active_process_limit': 1},
                {'cpu_rate': 1000, 'job_memory_bytes': 0, 'active_process_limit': 1},
                {'cpu_rate': 1000, 'job_memory_bytes': 1024, 'active_process_limit': 0},
                {'cpu_rate': 1000, 'job_memory_bytes': 1024, 'active_process_limit': True}):
            with self.subTest(values=values), self.assertRaises((TypeError, ValueError)):
                supervision.JobLimits(**values)

    def test_windows_job_applies_cpu_memory_and_process_limits(self):
        limits = supervision.JobLimits(cpu_rate=2500, job_memory_bytes=8 * 1024 * 1024,
                                       active_process_limit=3)
        calls = []

        class Api:
            def __init__(self, name):
                self.name = name
                self.argtypes = None
                self.restype = None

            def __call__(self, *args):
                calls.append((self.name, args))
                if self.name == 'CreateJobObjectW':
                    return 101
                if self.name == 'SetInformationJobObject':
                    if not hasattr(kernel, 'configured'):
                        kernel.configured = {}
                    kernel.configured[args[1]] = (ctypes.string_at(args[2], args[3]), args[3])
                    return 1
                if self.name == 'QueryInformationJobObject':
                    source, size = kernel.configured[args[1]]
                    ctypes.memmove(args[2], source, size)
                    return 1
                if self.name == 'CloseHandle':
                    return 1
                raise AssertionError('unexpected WinAPI call: ' + self.name)

        class Kernel:
            def __getattr__(self, name):
                value = Api(name)
                setattr(self, name, value)
                return value

        kernel = Kernel()
        kernel.configured = {}
        import ctypes
        with patch.object(supervision.ctypes, 'WinDLL', return_value=kernel, create=True), \
             patch.object(supervision.ctypes, 'set_last_error', return_value=None, create=True):
            returned_kernel, job, accounting = supervision._windows_job(limits)

        self.assertIs(returned_kernel, kernel)
        self.assertEqual(job, 101)
        configured = [(args[1], args[2]._obj) for name, args in calls
                      if name == 'SetInformationJobObject']
        self.assertEqual({info_class for info_class, _ in configured}, {9, 15})
        extended = next(value for info_class, value in configured if info_class == 9)
        basic = extended.BasicLimitInformation
        self.assertTrue(basic.LimitFlags & 0x2000)  # kill on close
        self.assertTrue(basic.LimitFlags & 0x8)     # active process limit
        self.assertTrue(basic.LimitFlags & 0x200)   # job memory limit
        self.assertEqual(basic.ActiveProcessLimit, limits.active_process_limit)
        self.assertEqual(extended.JobMemoryLimit, limits.job_memory_bytes)
        cpu = next(value for info_class, value in configured if info_class == 15)
        self.assertEqual(cpu.ControlFlags, 0x5)  # ENABLE | HARD_CAP
        self.assertEqual(cpu.CpuRate, limits.cpu_rate)
        query_calls = [(index, args[1]) for index, (name, args) in enumerate(calls)
                       if name == 'QueryInformationJobObject']
        self.assertEqual({info_class for _, info_class in query_calls}, {9, 15})
        self.assertGreater(min(index for index, _ in query_calls),
                           max(index for index, (name, _) in enumerate(calls)
                               if name == 'SetInformationJobObject'))
        queried = [(args[1], args[2]._obj) for name, args in calls
                   if name == 'QueryInformationJobObject']
        queried_extended = next(value for info_class, value in queried if info_class == 9)
        self.assertEqual(queried_extended.BasicLimitInformation.ActiveProcessLimit,
                         limits.active_process_limit)
        self.assertEqual(queried_extended.JobMemoryLimit, limits.job_memory_bytes)
        queried_cpu = next(value for info_class, value in queried if info_class == 15)
        self.assertEqual(queried_cpu.CpuRate, limits.cpu_rate)

    def test_diagnostic_requires_job_limits_and_returns_verified_job_evidence(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=16 * 1024 * 1024,
                                       active_process_limit=5)
        evidence = {'cpu_rate': 1800, 'job_memory_bytes': 16 * 1024 * 1024,
                    'active_process_limit': 5, 'peak_job_memory_bytes': 4096,
                    'total_processes': 2, 'active_processes_after_cleanup': 0}
        completed = subprocess.CompletedProcess(['probe'], 0)
        completed.job_evidence = evidence
        with patch.object(supervision.os, 'name', 'nt'), \
             patch.object(supervision, '_run_diagnostic_with_input', return_value=completed) as run:
            result = supervision.run_diagnostic(['probe'], timeout=1, capture_output=True,
                                                 job_limits=limits)
        self.assertIs(result, completed)
        self.assertEqual(result.job_evidence, evidence)
        self.assertEqual(run.call_args.args[0], ['probe'])
        self.assertEqual(run.call_args.kwargs['job_limits'], limits)

    def test_diagnostic_refuses_invalid_supplied_limits_before_broker_launch(self):
        with patch.object(supervision.os, 'name', 'nt'), \
             patch.object(supervision, '_run_diagnostic_with_input') as run:
            with self.assertRaises((TypeError, ValueError, supervision.SupervisionError)):
                supervision.run_diagnostic(['probe'], timeout=1, capture_output=True,
                                           job_limits={'cpu_rate': 0})
        run.assert_not_called()

    def test_windows_job_configuration_failure_closes_job_and_fails_closed(self):
        limits = supervision.JobLimits(cpu_rate=2500, job_memory_bytes=8192,
                                       active_process_limit=3)
        calls = []

        class Api:
            def __init__(self, name):
                self.name = name
                self.argtypes = None
                self.restype = None

            def __call__(self, *args):
                calls.append(self.name)
                if self.name == 'CreateJobObjectW':
                    return 101
                if self.name == 'SetInformationJobObject':
                    return 0
                if self.name == 'CloseHandle':
                    return 1
                raise AssertionError('unexpected WinAPI call: ' + self.name)

        class Kernel:
            def __getattr__(self, name):
                value = Api(name)
                setattr(self, name, value)
                return value

        kernel = Kernel()
        with patch.object(supervision.ctypes, 'WinDLL', return_value=kernel, create=True), \
             patch.object(supervision.ctypes, 'set_last_error', return_value=None, create=True):
            with self.assertRaises(supervision.SupervisionError):
                supervision._windows_job(limits)
        self.assertEqual(calls.count('CreateJobObjectW'), 1)
        self.assertEqual(calls.count('SetInformationJobObject'), 1)
        self.assertEqual(calls.count('CloseHandle'), 1)
        self.assertNotIn('QueryInformationJobObject', calls)

    def test_named_job_collision_fails_before_configuration_or_process_launch(self):
        limits = supervision.JobLimits(cpu_rate=2000, job_memory_bytes=8192,
                                       active_process_limit=3)
        calls = []

        class Api:
            def __init__(self, name):
                self.name = name
                self.argtypes = None
                self.restype = None

            def __call__(self, *args):
                calls.append((self.name, args))
                if self.name == 'CreateJobObjectW':
                    return 101
                if self.name == 'CloseHandle':
                    return 1
                raise AssertionError('collision must stop before configuration')

        class Kernel:
            def __getattr__(self, name):
                value = Api(name)
                setattr(self, name, value)
                return value

        kernel = Kernel()
        name = r'Local\VoiceCIProof-test-job'
        with patch.object(supervision.ctypes, 'WinDLL', return_value=kernel, create=True), \
             patch.object(supervision.ctypes, 'set_last_error', return_value=None, create=True), \
             patch.object(supervision.ctypes, 'get_last_error', return_value=183, create=True):
            with self.assertRaisesRegex(supervision.SupervisionError, 'already exists|collision'):
                supervision._windows_job(limits, job_name=name)
        create = next(args for method, args in calls if method == 'CreateJobObjectW')
        self.assertEqual(create[1], name)
        self.assertEqual(sum(method == 'CloseHandle' for method, _ in calls), 1)
        self.assertFalse(any(method == 'SetInformationJobObject' for method, _ in calls))

    @unittest.skipUnless(os.name == 'nt', 'Windows named Job Object acceptance')
    def test_named_job_presence_query_is_read_only_and_exact(self):
        name = r'Local\VoiceCIProof-' + uuid.uuid4().hex
        kernel, job, _accounting = supervision._windows_job(job_name=name)
        try:
            self.assertTrue(supervision.named_job_exists(name))
        finally:
            self.assertTrue(kernel.CloseHandle(job))
        self.assertFalse(supervision.named_job_exists(name))

    def test_broker_cleanup_attempts_every_close_and_preserves_execution_failure(self):
        calls = []

        class PipeApi:
            @staticmethod
            def CloseHandle(_handle):
                calls.append('pipe')
                raise OSError('pipe close failed')

        class Kernel:
            @staticmethod
            def CloseHandle(_handle):
                calls.append('job')
                return 0

        class Process:
            @staticmethod
            def close():
                calls.append('process')
                raise RuntimeError('process close failed')

        job_name = r'Local\VoiceCIProof-cleanup-' + uuid.uuid4().hex
        with patch.object(supervision, 'named_job_exists',
                          side_effect=OSError('job query failed')):
            errors = supervision._close_broker_resources(
                PipeApi, Kernel, 'pipe-handle', Process(), 'job-handle', job_name)
        self.assertEqual(calls, ['pipe', 'process', 'job'])
        self.assertEqual(len(errors), 4)

        execution_failure = TimeoutError('execution timed out')
        with self.assertRaises(BaseExceptionGroup) as raised:
            supervision._finish_broker(object(), execution_failure, errors)
        self.assertIn(execution_failure, raised.exception.exceptions)
        self.assertEqual(len(raised.exception.exceptions), 2)

    def test_broker_cleanup_rejects_job_that_remains_after_close(self):
        calls = []

        class PipeApi:
            @staticmethod
            def CloseHandle(_handle):
                calls.append('pipe')

        class Kernel:
            @staticmethod
            def CloseHandle(_handle):
                calls.append('job')
                return 1

        class Process:
            @staticmethod
            def close():
                calls.append('process')

        job_name = r'Local\VoiceCIProof-remains-' + uuid.uuid4().hex
        with patch.object(supervision, 'named_job_exists', return_value=True):
            errors = supervision._close_broker_resources(
                PipeApi, Kernel, 'pipe-handle', Process(), 'job-handle', job_name)
        self.assertEqual(calls, ['pipe', 'process', 'job'])
        with self.assertRaisesRegex(supervision.SupervisionError, 'cleanup failed'):
            supervision._finish_broker(object(), None, errors)

    def test_windows_job_rejects_failed_or_mismatched_limit_readback(self):
        import ctypes
        limits = supervision.JobLimits(cpu_rate=2500, job_memory_bytes=8192,
                                       active_process_limit=3)
        for failure in ('second_set', 'query', 'cpu_rate', 'memory',
                        'active_process_limit', 'limit_flags'):
            with self.subTest(failure=failure):
                calls = []

                class Api:
                    def __init__(self, name):
                        self.name = name
                        self.argtypes = None
                        self.restype = None

                    def __call__(self, *args):
                        calls.append((self.name, args))
                        if self.name == 'CreateJobObjectW':
                            return 101
                        if self.name == 'SetInformationJobObject':
                            if failure == 'second_set' and sum(
                                    name == 'SetInformationJobObject' for name, _ in calls) == 2:
                                return 0
                            if not hasattr(kernel, 'configured'):
                                kernel.configured = {}
                            kernel.configured[args[1]] = (
                                ctypes.string_at(args[2], args[3]), args[3])
                            return 1
                        if self.name == 'QueryInformationJobObject':
                            if failure == 'query':
                                return 0
                            raw, size = kernel.configured[args[1]]
                            ctypes.memmove(args[2], raw, size)
                            value = args[2]._obj
                            if failure == 'cpu_rate' and args[1] == 15:
                                value.CpuRate += 1
                            elif failure == 'memory' and args[1] == 9:
                                value.JobMemoryLimit += 1
                            elif failure == 'active_process_limit' and args[1] == 9:
                                value.BasicLimitInformation.ActiveProcessLimit += 1
                            elif failure == 'limit_flags' and args[1] == 9:
                                value.BasicLimitInformation.LimitFlags ^= 0x200
                            return 1
                        if self.name == 'CloseHandle':
                            return 1
                        raise AssertionError('unexpected WinAPI call: ' + self.name)

                class Kernel:
                    def __getattr__(self, name):
                        value = Api(name)
                        setattr(self, name, value)
                        return value

                kernel = Kernel()
                kernel.configured = {}
                with patch.object(supervision.ctypes, 'WinDLL', return_value=kernel, create=True), \
                     patch.object(supervision.ctypes, 'set_last_error', return_value=None, create=True):
                    with self.assertRaises(supervision.SupervisionError):
                        supervision._windows_job(limits)
                self.assertEqual(sum(name == 'CloseHandle' for name, _ in calls), 1)

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_real_broker_ignores_python_environment_and_adjacent_shadows(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            probe = ('if __name__ == "__main__":\n'
                     ' try:\n  import pwd\n except ImportError:\n  pass\n'
                     ' try:\n  import grp\n except ImportError:\n  pass\n')
            (root / 'local_process_supervision.py').write_text(
                probe + Path(supervision.__file__).read_text(encoding='utf-8'), encoding='utf-8')
            marker = root / 'injected'
            for name in ('json.py', 'pathlib.py', 'sitecustomize.py', 'pwd.py', 'grp.py'):
                (root / name).write_text('open(' + repr(str(marker)) + ', "w").write("bad")\n', encoding='utf-8')
            program = ('import sys;sys.path.append(sys.argv[1]);import local_process_supervision as s;'
                       'r=s.run_diagnostic([sys.executable,"-I","-S","-c","print(123)"],'
                       'timeout=20,capture_output=True);assert r.returncode==0 and r.stdout.strip()==b"123"')
            env = dict(os.environ, PYTHONPATH=str(root), PYTHONHOME=str(root))
            result = subprocess.run([sys.executable, '-I', '-S', '-B', '-c', program, str(root)],
                                    cwd=root, env=env, capture_output=True, timeout=30, check=False)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse(marker.exists())

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_private_input_is_exclusive_and_deleted_on_close(self):
        with tempfile.TemporaryDirectory() as directory:
            with patch.object(supervision.tempfile, 'gettempdir', return_value=directory):
                with supervision._private_windows_input() as source:
                    source.write(b'secret')
                    source.flush()
                    paths = list(Path(directory).iterdir())
                    self.assertEqual(len(paths), 1)
                    with self.assertRaises(OSError):
                        paths[0].read_bytes()
                self.assertEqual(list(Path(directory).iterdir()), [])

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_private_input_is_deleted_after_creator_crashes(self):
        with tempfile.TemporaryDirectory() as directory:
            program = ('import os, tempfile, local_process_supervision as s; '
                       'tempfile.tempdir = ' + repr(directory) + '; '
                       'f = s._private_windows_input(); f.write(b"secret"); f.flush(); os._exit(7)')
            result = subprocess.run([sys.executable, '-c', program], cwd=Path(__file__).parent,
                                    timeout=5, capture_output=True)
            self.assertEqual(result.returncode, 7, result.stderr)
            self.assertEqual(list(Path(directory).iterdir()), [])

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_diagnostic_binary_input_and_eof(self):
        data = bytes(range(256)) * 1024 + b'\x00\xff'
        result = supervision.run_diagnostic([sys.executable, '-c',
            'import sys; sys.stdout.buffer.write(sys.stdin.buffer.read())'],
            timeout=10, capture_output=True, input=data)
        self.assertEqual((result.returncode, result.stdout), (0, data))

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_diagnostic_input_early_exit_and_no_read_timeout(self):
        data = b'x' * (1024 * 1024)
        result = supervision.run_diagnostic([sys.executable, '-c', 'pass'],
                                            timeout=5, capture_output=True, input=data)
        self.assertEqual(result.returncode, 0)
        start = time.monotonic()
        with self.assertRaises(subprocess.TimeoutExpired):
            supervision.run_diagnostic([sys.executable, '-c', 'import time; time.sleep(10)'],
                                       timeout=.5, capture_output=True, input=data)
        self.assertLess(time.monotonic() - start, 5)

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_diagnostic_rejects_unsafe_input(self):
        for value in ('text', bytearray(b'x'), b'x' * 33):
            with self.subTest(value_type=type(value), size=len(value)):
                with patch.object(supervision, 'MAX_INPUT_BYTES', 32, create=True):
                    with self.assertRaises(ValueError):
                        supervision.run_diagnostic([sys.executable], timeout=5,
                                                   capture_output=True, input=value)

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_public_diagnostic_capture_returns_bytes_and_exit_code(self):
        result = supervision.run_diagnostic([sys.executable, '-c',
            'import sys; sys.stdout.buffer.write(b"out"); sys.stderr.buffer.write(b"err"); sys.exit(7)'],
            timeout=5, cwd=None, env=dict(os.environ), capture_output=True, check=False)
        self.assertIsInstance(result, subprocess.CompletedProcess)
        self.assertEqual((result.returncode, result.stdout, result.stderr), (7, b'out', b'err'))

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_public_diagnostic_keeps_caller_output_files_open(self):
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            result = supervision.run_diagnostic([sys.executable, '-c', 'print("owned")'],
                timeout=5, cwd=None, env=dict(os.environ), stdout=out, stderr=err, check=False)
            self.assertEqual(result.returncode, 0)
            self.assertIsNone(result.stdout)
            self.assertFalse(out.closed)
            out.seek(0)
            self.assertEqual(out.read().strip(), b'owned')

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_public_diagnostic_rejects_buffered_existing_output_and_aliases(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'output'
            with path.open('w+b') as out, tempfile.TemporaryFile() as err:
                out.write(b'pending')
                with self.assertRaisesRegex(ValueError, 'fresh'):
                    supervision.run_diagnostic([sys.executable, '-c', 'print("unsafe")'], timeout=5, stdout=out, stderr=err)
                self.assertFalse(out.closed)
            path.write_bytes(b'')
            with path.open('r+b') as out, path.open('r+b') as alias:
                with self.assertRaisesRegex(ValueError, 'distinct'):
                    supervision.run_diagnostic([sys.executable, '-c', 'print("unsafe")'], timeout=5, stdout=out, stderr=alias)

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_public_diagnostic_rejects_capture_above_limit(self):
        with patch.object(supervision, 'MAX_CAPTURE_BYTES', 32):
            with self.assertRaisesRegex(ValueError, 'capture exceeds'):
                supervision.run_diagnostic([sys.executable, '-c', 'print("x" * 33)'],
                                           timeout=5, capture_output=True)

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_public_diagnostic_times_out(self):
        with self.assertRaises(subprocess.TimeoutExpired):
            supervision.run_diagnostic([sys.executable, '-c', 'import time; time.sleep(10)'],
                                      timeout=.3, capture_output=True)

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object active-process limit acceptance')
    def test_real_diagnostic_job_rejects_grandchild_and_reports_exact_limits_and_cleanup(self):
        limits = supervision.JobLimits(cpu_rate=2500, job_memory_bytes=256 * 1024 * 1024,
                                       active_process_limit=2)
        child = ('import subprocess,sys\n'
                 'try:\n'
                 ' p=subprocess.Popen([sys.executable,"-c","pass"]); p.wait()\n'
                 ' print("GRANDCHILD_CREATED")\n'
                 'except OSError as error:\n'
                 ' if error.winerror != 1816: raise\n'
                 ' print("GRANDCHILD_REJECTED")')
        job_name = r'Local\VoiceCIProof-' + uuid.uuid4().hex
        result = supervision.run_diagnostic([sys.executable, '-I', '-S', '-c', child],
                                             timeout=15, capture_output=True,
                                              job_limits=limits, job_name=job_name)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(b'GRANDCHILD_REJECTED', result.stdout)
        self.assertNotIn(b'GRANDCHILD_CREATED', result.stdout)
        self.assertEqual(result.job_evidence['cpu_rate'], limits.cpu_rate)
        self.assertEqual(result.job_evidence['job_memory_bytes'], limits.job_memory_bytes)
        self.assertEqual(result.job_evidence['active_process_limit'], limits.active_process_limit)
        self.assertGreaterEqual(result.job_evidence['peak_job_memory_bytes'], 0)
        self.assertLessEqual(result.job_evidence['peak_job_memory_bytes'], limits.job_memory_bytes)
        self.assertEqual(result.job_evidence['total_processes'], 2)
        self.assertEqual(result.job_evidence['active_processes_after_cleanup'], 0)
        self.assertEqual(result.job_evidence['job_name'], job_name)
        self.assertFalse(supervision.named_job_exists(job_name))
        control = supervision.run_diagnostic([sys.executable, '-I', '-S', '-c', child],
            timeout=15, capture_output=True,
            job_limits=supervision.JobLimits(cpu_rate=2500,
                job_memory_bytes=256 * 1024 * 1024, active_process_limit=3))
        self.assertEqual(control.returncode, 0, control.stderr)
        self.assertIn(b'GRANDCHILD_CREATED', control.stdout)
        self.assertNotIn(b'GRANDCHILD_REJECTED', control.stdout)
        self.assertEqual(control.job_evidence['total_processes'], 3)
        self.assertEqual(control.job_evidence['active_processes_after_cleanup'], 0)

    def test_public_diagnostic_refuses_unsupported_arguments_and_platform(self):
        with self.assertRaises((ValueError, supervision.SupervisionError)):
            supervision.run_diagnostic([sys.executable], timeout=1, capture_output=True, check=True)
        with self.assertRaises((ValueError, supervision.SupervisionError)):
            supervision.run_diagnostic([sys.executable], timeout=0, capture_output=True)
        with self.assertRaises(TypeError):
            supervision.run_diagnostic([sys.executable], timeout=1, capture_output=True, shell=True)
        if os.name == 'nt':
            with tempfile.TemporaryFile() as out:
                with self.assertRaises(ValueError):
                    supervision.run_diagnostic([sys.executable], timeout=1, capture_output=True, stdout=out)
            with self.assertRaises(ValueError):
                supervision.run_diagnostic([sys.executable], timeout=1)
            with self.assertRaises(ValueError):
                supervision.run_diagnostic([], timeout=1, capture_output=True)
        with patch.object(supervision.os, 'name', 'posix'):
            with self.assertRaises(supervision.SupervisionError):
                supervision.run_diagnostic([sys.executable], timeout=1, capture_output=True)
    @unittest.skipUnless(os.name == 'nt', 'Windows named-pipe acceptance')
    def test_pipe_timeout_partial_message_disconnect_and_wrong_pid(self):
        import ctypes
        from multiprocessing.connection import Listener, Client
        address = r'\\.\pipe\voice-test-' + uuid.uuid4().hex
        listener = Listener(address, family='AF_PIPE')
        client = Client(address, family='AF_PIPE')
        server = listener.accept()
        try:
            kernel = ctypes.WinDLL('kernel32', use_last_error=True)
            with self.assertRaisesRegex(supervision.SupervisionError, 'identity mismatch'):
                supervision._require_broker_pid(kernel, server.fileno(), os.getpid() + 1)
            with self.assertRaises(subprocess.TimeoutExpired):
                supervision._pipe_io(server.fileno(), None, time.monotonic() + .15, 1024)
            supervision._pipe_io(client.fileno(), b'{', time.monotonic() + 1, 1024)
            partial = supervision._pipe_io(server.fileno(), None, time.monotonic() + 1, 1024)
            with self.assertRaises(json.JSONDecodeError):
                json.loads(partial)
            client.close()
            with self.assertRaises(supervision.SupervisionError):
                supervision._pipe_io(server.fileno(), None, time.monotonic() + 1, 1024)
        finally:
            client.close(); server.close(); listener.close()
        address = r'\\.\pipe\voice-test-' + uuid.uuid4().hex
        listener = Listener(address, family='AF_PIPE')
        client = Client(address, family='AF_PIPE')
        server = listener.accept()
        try:
            supervision._pipe_io(client.fileno(), b'x' * 2048, time.monotonic() + 2, 2048)
            with self.assertRaises(supervision.SupervisionError):
                supervision._pipe_io(server.fileno(), None, time.monotonic() + 2, 128)
        finally:
            client.close(); server.close(); listener.close()

    @unittest.skipUnless(os.name == 'nt', 'Windows named-pipe acceptance')
    def test_pipe_blocked_write_and_oversized_message_fail_bounded(self):
        from multiprocessing.connection import Listener, Client
        address = r'\\.\pipe\voice-test-' + uuid.uuid4().hex
        listener = Listener(address, family='AF_PIPE')
        client = Client(address, family='AF_PIPE')
        server = listener.accept()
        try:
            start = time.monotonic()
            with self.assertRaises(subprocess.TimeoutExpired):
                supervision._pipe_io(server.fileno(), b'x' * (2 * 1024 * 1024), start + .2, 2 * 1024 * 1024)
            self.assertLess(time.monotonic() - start, 3)
        finally:
            client.close(); server.close(); listener.close()

    @unittest.skipUnless(os.name == 'nt', 'Windows named-pipe acceptance')
    def test_competing_pipe_client_cannot_impersonate_broker_pid(self):
        import ctypes
        from multiprocessing.connection import Listener
        address = r'\\.\pipe\voice-test-' + uuid.uuid4().hex
        listener = Listener(address, family='AF_PIPE')
        child = subprocess.Popen([sys.executable, '-B', '-c',
            'import sys,time; from multiprocessing.connection import Client; c=Client(sys.argv[1],family="AF_PIPE"); time.sleep(10)', address])
        server = None
        try:
            server = listener.accept()
            kernel = ctypes.WinDLL('kernel32', use_last_error=True)
            with self.assertRaisesRegex(supervision.SupervisionError, 'identity mismatch'):
                supervision._require_broker_pid(kernel, server.fileno(), os.getpid())
        finally:
            if server:
                server.close()
            listener.close()
            child.terminate()
            child.wait(timeout=5)
    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_broker_binds_exact_output_handles_without_parent_inheritable_handles(self):
        import msvcrt
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            handles = [msvcrt.get_osfhandle(file.fileno()) for file in (out, err)]
            before = [os.get_handle_inheritable(handle) for handle in handles]
            self.assertEqual(before, [False, False])
            result = supervision._run_broker_experimental(
                [sys.executable, '-c', 'import sys; print("owned out"); print("owned err", file=sys.stderr)'],
                timeout=5, cwd=None, env=dict(os.environ), stdout=out, stderr=err)
            self.assertEqual(result.returncode, 0)
            self.assertEqual([os.get_handle_inheritable(handle) for handle in handles], before)
            out.seek(0); err.seek(0)
            self.assertEqual(out.read().decode().strip(), 'owned out')
            self.assertEqual(err.read().decode().strip(), 'owned err')

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_broker_runs_constrained_flutter_batch_and_rejects_metacharacters(self):
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            batch = Path(directory) / 'flutter fixture.bat'
            batch.write_text('@echo off\r\necho %~1 %~2 %~3\r\n', encoding='ascii')
            command = [str(batch), 'test', '--no-pub', '--concurrency=1']
            result = supervision._run_broker_experimental(command, timeout=5, cwd=directory,
                                                          env=dict(os.environ), stdout=out, stderr=err)
            self.assertEqual(result.returncode, 0)
            out.seek(0)
            self.assertEqual(out.read().decode().strip(), 'test --no-pub --concurrency=1')
            with self.assertRaisesRegex(supervision.SupervisionError, 'unsafe batch'):
                supervision._run_broker_experimental([str(batch), 'x&echo injected'], timeout=5,
                                                      cwd=directory, env=dict(os.environ), stdout=out, stderr=err)

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_broker_forwards_unicode_argv_cwd_and_environment(self):
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            env = dict(os.environ, VOICE_PROCESS_TEST='значение', PYTHONIOENCODING='utf-8')
            command = [sys.executable, '-c',
                       'import os,sys; print(os.getcwd()); print(sys.argv[1]); print(os.environ["VOICE_PROCESS_TEST"], file=sys.stderr)',
                       'аргумент with spaces']
            result = supervision._run_broker_experimental(command, timeout=5, cwd=directory, env=env, stdout=out, stderr=err)
            self.assertEqual(result.returncode, 0)
            out.seek(0); err.seek(0)
            lines = out.read().decode('utf-8').splitlines()
            self.assertEqual(Path(lines[0]), Path(directory))
            self.assertEqual(lines[1], 'аргумент with spaces')
            self.assertEqual(err.read().decode('utf-8').strip(), 'значение')

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_broker_timeout_kills_ready_grandchild_and_preserves_sentinel(self):
        import ctypes
        from ctypes import wintypes
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            ready = Path(directory) / 'ready'
            grandchild = 'import os,pathlib,sys,time; pathlib.Path(sys.argv[1]).write_text(str(os.getpid())); time.sleep(20)'
            child = 'import subprocess,sys,time; subprocess.Popen([sys.executable,"-c",sys.argv[1],sys.argv[2]]); time.sleep(20)'
            sentinel = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(20)'])
            kernel = ctypes.WinDLL('kernel32', use_last_error=True)
            kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
            kernel.OpenProcess.restype = wintypes.HANDLE
            kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
            kernel.CloseHandle.argtypes = [wintypes.HANDLE]
            handles = []
            def observe():
                deadline = time.monotonic() + 1
                while time.monotonic() < deadline:
                    if ready.exists() and ready.read_text().strip().isdigit():
                        handles.append(kernel.OpenProcess(0x100000, False, int(ready.read_text())))
                        return
                    time.sleep(.01)
            observer = threading.Thread(target=observe)
            observer.start()
            try:
                with self.assertRaises(subprocess.TimeoutExpired):
                    supervision._run_broker_experimental([sys.executable, '-c', child, grandchild, str(ready)],
                        timeout=1.5, cwd=None, env=dict(os.environ), stdout=out, stderr=err)
                self.assertTrue(ready.exists())
                observer.join(timeout=2)
                self.assertEqual(len(handles), 1)
                self.assertTrue(handles[0])
                self.assertEqual(kernel.WaitForSingleObject(handles[0], 5000), 0)
                self.assertIsNone(sentinel.poll())
            finally:
                observer.join(timeout=2)
                for handle in handles:
                    if handle:
                        kernel.CloseHandle(handle)
                sentinel.terminate()
                sentinel.wait(timeout=5)

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_supervisor_crash_terminates_live_broker_descendant(self):
        import ctypes
        from ctypes import wintypes
        with tempfile.TemporaryDirectory() as directory:
            ready = Path(directory) / 'ready'
            input_dir = Path(directory) / 'input'; input_dir.mkdir()
            child = ('import os,pathlib,sys,time; assert sys.stdin.buffer.read(1)==b"x"; '
                     'pathlib.Path(sys.argv[1]).write_text(str(os.getpid())); time.sleep(20)')
            harness = ('import os,sys,tempfile; sys.path.insert(0,sys.argv[1]); import local_process_supervision as s; '
                       'o=tempfile.TemporaryFile(); e=tempfile.TemporaryFile(); '
                       'tempfile.tempdir=sys.argv[4]; '
                       's.run_diagnostic([sys.executable,"-c",sys.argv[2],sys.argv[3]],'
                       'timeout=15,env=dict(os.environ),stdout=o,stderr=e,input=b"x"*1000)')
            owner = subprocess.Popen([sys.executable, '-B', '-c', harness, str(Path(__file__).parent), child, str(ready), str(input_dir)])
            kernel = ctypes.WinDLL('kernel32', use_last_error=True)
            kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
            kernel.OpenProcess.restype = wintypes.HANDLE
            kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
            kernel.CloseHandle.argtypes = [wintypes.HANDLE]
            handle = None
            try:
                deadline = time.monotonic() + 5
                while (not ready.exists() or not ready.read_text().strip().isdigit()) and time.monotonic() < deadline:
                    time.sleep(.02)
                self.assertTrue(ready.exists())
                handle = kernel.OpenProcess(0x100000, False, int(ready.read_text()))
                self.assertTrue(handle)
                self.assertEqual(len(list(input_dir.iterdir())), 1)
                owner.terminate()
                owner.wait(timeout=5)
                self.assertEqual(kernel.WaitForSingleObject(handle, 5000), 0)
                deadline = time.monotonic() + 5
                while list(input_dir.iterdir()) and time.monotonic() < deadline:
                    time.sleep(.02)
                self.assertEqual(list(input_dir.iterdir()), [])
            finally:
                if owner.poll() is None:
                    owner.terminate()
                    owner.wait(timeout=5)
                if handle:
                    kernel.CloseHandle(handle)

    @unittest.skipUnless(os.name == 'nt', 'Windows broker acceptance')
    def test_broker_crash_fails_and_reaps_its_child(self):
        import ctypes
        from ctypes import wintypes
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            ready = Path(directory) / 'ready'
            release = Path(directory) / 'release'
            input_dir = Path(directory) / 'input'; input_dir.mkdir()
            child = ('import ctypes,os,pathlib,sys,time; '
                     'assert sys.stdin.buffer.read(1)==b"x"; '
                     'pathlib.Path(sys.argv[1]).write_text(str(os.getpid())); '
                     'p=pathlib.Path(sys.argv[2]); '
                     'exec("while not p.exists():\\n time.sleep(.02)"); '
                     'h=ctypes.windll.kernel32.OpenProcess(1,False,os.getppid()); '
                     'ctypes.windll.kernel32.TerminateProcess(h,42); time.sleep(20)')
            kernel = ctypes.WinDLL('kernel32', use_last_error=True)
            kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
            kernel.OpenProcess.restype = wintypes.HANDLE
            kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
            kernel.CloseHandle.argtypes = [wintypes.HANDLE]
            handles = []
            def observe():
                deadline = time.monotonic() + 5
                while (not ready.exists() or not ready.read_text().strip().isdigit()) and time.monotonic() < deadline:
                    time.sleep(.02)
                if ready.exists():
                    handles.append(kernel.OpenProcess(0x100000, False, int(ready.read_text())))
                    release.write_text('release')
            observer = threading.Thread(target=observe)
            observer.start()
            try:
                with patch.object(supervision.tempfile, 'gettempdir', return_value=str(input_dir)):
                    with self.assertRaises((EOFError, supervision.SupervisionError)):
                        supervision.run_diagnostic([sys.executable, '-c', child, str(ready), str(release)],
                            timeout=7, stdout=out, stderr=err, input=b'x' * 1000)
                observer.join(timeout=2)
                self.assertEqual(len(handles), 1)
                self.assertTrue(handles[0])
                self.assertEqual(kernel.WaitForSingleObject(handles[0], 5000), 0)
                self.assertEqual(list(input_dir.iterdir()), [])
            finally:
                observer.join(timeout=6)
                for handle in handles:
                    if handle:
                        kernel.CloseHandle(handle)
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
        seen = []
        real_assign = supervision._assign_windows
        def assign(kernel, job, process):
            seen.append(process)
            return real_assign(kernel, job, process)
        try:
            with patch.object(supervision, '_assign_windows', side_effect=assign), \
                 patch.object(supervision, '_wait_windows', side_effect=KeyboardInterrupt()), \
                 patch.object(supervision, '_settle_windows', side_effect=supervision.SupervisionError('cleanup evidence missing')):
                with self.assertRaises(BaseExceptionGroup) as caught:
                    supervision.run([sys.executable, '-c', 'import time; time.sleep(8)'], timeout=2,
                                    allow_unverified_crash_window=True)
            self.assertIsInstance(caught.exception.exceptions[0], KeyboardInterrupt)
            self.assertIn('cleanup evidence missing', str(caught.exception.exceptions[1]))
            self.assertEqual(len(seen), 1)
            self.assertIsNotNone(seen[0].returncode, 'owned child must be reaped even when job verification fails')
        finally:
            for process in seen:
                process.wait(timeout=5)

    @unittest.skipUnless(os.name == 'nt', 'Windows Job Object acceptance')
    def test_reap_failure_preserves_execution_and_cleanup_uncertainty(self):
        seen = []
        real_assign = supervision._assign_windows
        def assign(kernel, job, process):
            seen.append((process, process.wait))
            process.wait = lambda timeout: (_ for _ in ()).throw(subprocess.TimeoutExpired('reap', timeout))
            return real_assign(kernel, job, process)
        try:
            with patch.object(supervision, '_assign_windows', side_effect=assign), \
                 patch.object(supervision, '_wait_windows', side_effect=KeyboardInterrupt()), \
                 patch.object(supervision, '_settle_windows', side_effect=supervision.SupervisionError('job uncertain')):
                with self.assertRaises(BaseExceptionGroup) as caught:
                    supervision.run([sys.executable, '-c', 'import time; time.sleep(8)'], timeout=2,
                                    allow_unverified_crash_window=True)
            self.assertIsInstance(caught.exception.exceptions[0], KeyboardInterrupt)
            cleanup_group = caught.exception.exceptions[1]
            self.assertIn('job uncertain', str(cleanup_group.exceptions[0]))
            self.assertIsInstance(cleanup_group.exceptions[1], subprocess.TimeoutExpired)
        finally:
            for process, wait in seen:
                process.wait = wait
                wait(timeout=5)

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
