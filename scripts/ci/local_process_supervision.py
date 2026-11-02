"""Run an owned process tree with bounded execution and verified cleanup.

Windows uses a kill-on-close Job Object. Linux is refused until an external
crash-safe containment and bounded launcher are available. Never treat a
cleanup error as a successful command. A supervisor crash between suspended
process creation and job assignment can leave that process uncontained; this
window is not crash-safe and the caller must not claim crash-safe execution.
"""
import ctypes
from contextlib import ExitStack
import io
import math
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import tempfile
import threading
import time
import uuid
import sys


class SupervisionError(RuntimeError):
    pass


MAX_CAPTURE_BYTES = 64 * 1024 * 1024
MAX_INPUT_BYTES = 64 * 1024 * 1024


def _windows_job():
    from ctypes import wintypes
    class BasicLimit(ctypes.Structure):
        _fields_ = [('PerProcessUserTimeLimit', ctypes.c_longlong), ('PerJobUserTimeLimit', ctypes.c_longlong),
                    ('LimitFlags', wintypes.DWORD), ('MinimumWorkingSetSize', ctypes.c_size_t),
                    ('MaximumWorkingSetSize', ctypes.c_size_t), ('ActiveProcessLimit', wintypes.DWORD),
                    ('Affinity', ctypes.c_size_t), ('PriorityClass', wintypes.DWORD),
                    ('SchedulingClass', wintypes.DWORD)]
    class IO(ctypes.Structure):
        _fields_ = [(name, ctypes.c_ulonglong) for name in
                    ('ReadOperationCount', 'WriteOperationCount', 'OtherOperationCount',
                     'ReadTransferCount', 'WriteTransferCount', 'OtherTransferCount')]
    class Extended(ctypes.Structure):
        _fields_ = [('BasicLimitInformation', BasicLimit), ('IoInfo', IO),
                    ('ProcessMemoryLimit', ctypes.c_size_t), ('JobMemoryLimit', ctypes.c_size_t),
                    ('PeakProcessMemoryUsed', ctypes.c_size_t), ('PeakJobMemoryUsed', ctypes.c_size_t)]
    class Accounting(ctypes.Structure):
        _fields_ = [('TotalUserTime', ctypes.c_longlong), ('TotalKernelTime', ctypes.c_longlong),
                    ('ThisPeriodTotalUserTime', ctypes.c_longlong), ('ThisPeriodTotalKernelTime', ctypes.c_longlong),
                    ('TotalPageFaultCount', wintypes.DWORD), ('TotalProcesses', wintypes.DWORD),
                    ('ActiveProcesses', wintypes.DWORD), ('TotalTerminatedProcesses', wintypes.DWORD)]
    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.CreateJobObjectW.argtypes = [ctypes.c_void_p, wintypes.LPCWSTR]
    kernel.CreateJobObjectW.restype = wintypes.HANDLE
    kernel.SetInformationJobObject.argtypes = [wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p, wintypes.DWORD]
    kernel.AssignProcessToJobObject.argtypes = [wintypes.HANDLE, wintypes.HANDLE]
    kernel.TerminateJobObject.argtypes = [wintypes.HANDLE, wintypes.UINT]
    kernel.QueryInformationJobObject.argtypes = [wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p, wintypes.DWORD, ctypes.c_void_p]
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    job = kernel.CreateJobObjectW(None, None)
    if not job:
        raise SupervisionError('cannot create job object')
    limits = Extended()
    limits.BasicLimitInformation.LimitFlags = 0x2000  # KILL_ON_JOB_CLOSE
    if not kernel.SetInformationJobObject(job, 9, ctypes.byref(limits), ctypes.sizeof(limits)):
        kernel.CloseHandle(job)
        raise SupervisionError('cannot configure job object')
    return kernel, job, Accounting


def _settle_windows(kernel, job, Accounting, deadline):
    if not kernel.TerminateJobObject(job, 1):
        raise SupervisionError('cannot terminate owned job')
    while time.monotonic() < deadline:
        info = Accounting()
        if not kernel.QueryInformationJobObject(job, 1, ctypes.byref(info), ctypes.sizeof(info), None):
            raise SupervisionError('cannot verify owned job cleanup')
        if info.ActiveProcesses == 0:
            return
        time.sleep(0.02)
    raise SupervisionError('owned job cleanup timed out')


def run(command, *, timeout, cwd=None, env=None, stdout=None, stderr=None,
        capture_output=False, check=False, platform=None, cgroup_parent=None, cleanup_timeout=10,
        allow_unverified_crash_window=False):
    """Return CompletedProcess; raise on timeout or unverified cleanup.

    Linux is not supported yet; `cgroup_parent` does not enable execution.
    """
    if (not isinstance(timeout, (int, float)) or isinstance(timeout, bool) or not math.isfinite(timeout)
            or timeout <= 0 or not isinstance(cleanup_timeout, (int, float)) or
            not math.isfinite(cleanup_timeout) or cleanup_timeout <= 0):
        raise ValueError('positive finite timeouts required')
    if not isinstance(command, (list, tuple)) or not command or capture_output and (stdout is not None or stderr is not None):
        raise ValueError('invalid command or output selection')
    platform = platform or ('windows' if os.name == 'nt' else 'linux' if os.name == 'posix' and Path('/proc').is_dir() else 'unsupported')
    if platform not in ('windows', 'linux') or (platform == 'windows' and os.name != 'nt'):
        raise SupervisionError('unsupported process containment platform')
    if platform == 'linux':
        raise SupervisionError('Linux crash-safe containment is unavailable')
    if allow_unverified_crash_window is not True:
        raise SupervisionError('atomic Windows job assignment is unavailable')
    files = []
    try:
        if capture_output:
            stdout = tempfile.TemporaryFile(); stderr = tempfile.TemporaryFile()
            files.extend((stdout, stderr))
        if platform == 'windows':
            return _run_windows(command, timeout, cleanup_timeout, cwd, env, stdout, stderr, capture_output, check)
    finally:
        for file in files:
            file.close()


def _result(command, process, stdout, stderr, capture, check):
    out = err = None
    if capture:
        stdout.seek(0); stderr.seek(0)
        out, err = stdout.read(), stderr.read()
    result = subprocess.CompletedProcess(command, process.returncode, out, err)
    if check:
        result.check_returncode()
    return result


def _assign_windows(kernel, job, process):
    return kernel.AssignProcessToJobObject(job, int(process._handle))


def _resume_windows(process):
    resume = ctypes.WinDLL('ntdll').NtResumeProcess
    resume.argtypes = [ctypes.c_void_p]
    resume.restype = ctypes.c_long
    return resume(int(process._handle))


def _wait_windows(process, timeout):
    process.wait(timeout=timeout)


def _atomic_created_hook(info):
    """Test seam immediately after CreateProcessW returns."""


def _atomic_command_line(command, env):
    """Limit batch arguments to characters that cmd.exe cannot interpret."""
    executable = shutil.which(str(command[0]), path=(env or os.environ).get('PATH'))
    if not executable:
        raise SupervisionError('executable not found')
    args = [str(arg) for arg in command]
    if executable.lower().endswith(('.bat', '.cmd')):
        args[0] = executable
        if any(not arg or any(not (ch.isalnum() or ch in ' ._:-=/\\') for ch in arg) for arg in args):
            raise SupervisionError('unsafe batch argument')
        shell = (env or os.environ).get('COMSPEC')
        if not shell or not Path(shell).is_file():
            raise SupervisionError('cmd.exe unavailable')
        line = subprocess.list2cmdline([shell, '/d', '/s', '/c']) + ' "' + ' '.join('"' + arg + '"' for arg in args) + '"'
        return shell, line
    if not executable.lower().endswith('.exe'):
        raise SupervisionError('atomic launcher requires an executable path')
    return executable, subprocess.list2cmdline(args)


def _launch_atomic_windows(kernel, job, accounting, command, cwd, env, stdout, stderr, *, broker=False):
    """Experimental only: requires no concurrent handle-inheriting launchers.

    Python cannot prove the absence of unmanaged native threads; this must run
    in a dedicated single-purpose broker before it is connected to run().
    """
    import msvcrt
    from ctypes import wintypes
    class StartupInfo(ctypes.Structure):
        _fields_ = [('cb', wintypes.DWORD), ('lpReserved', wintypes.LPWSTR), ('lpDesktop', wintypes.LPWSTR),
                    ('lpTitle', wintypes.LPWSTR), ('dwX', wintypes.DWORD), ('dwY', wintypes.DWORD),
                    ('dwXSize', wintypes.DWORD), ('dwYSize', wintypes.DWORD), ('dwXCountChars', wintypes.DWORD),
                    ('dwYCountChars', wintypes.DWORD), ('dwFillAttribute', wintypes.DWORD), ('dwFlags', wintypes.DWORD),
                    ('wShowWindow', wintypes.WORD), ('cbReserved2', wintypes.WORD), ('lpReserved2', ctypes.c_void_p),
                    ('hStdInput', wintypes.HANDLE), ('hStdOutput', wintypes.HANDLE), ('hStdError', wintypes.HANDLE)]
    class StartupInfoEx(ctypes.Structure):
        _fields_ = [('StartupInfo', StartupInfo), ('lpAttributeList', ctypes.c_void_p)]
    class ProcessInfo(ctypes.Structure):
        _fields_ = [('hProcess', wintypes.HANDLE), ('hThread', wintypes.HANDLE),
                    ('dwProcessId', wintypes.DWORD), ('dwThreadId', wintypes.DWORD)]
    class Process:
        def __init__(self, handle, pid):
            self.handle, self.pid, self.returncode = handle, pid, None
        def wait(self, timeout):
            code = kernel.WaitForSingleObject(self.handle, min(int(timeout * 1000), 0xfffffffe))
            if code == 258:
                raise subprocess.TimeoutExpired(command, timeout)
            if code != 0:
                raise SupervisionError('process wait failed')
            result = wintypes.DWORD()
            if not kernel.GetExitCodeProcess(self.handle, ctypes.byref(result)):
                raise SupervisionError('process exit status unavailable')
            self.returncode = result.value
            return self.returncode
        def close(self):
            kernel.CloseHandle(self.handle)
    if not broker and threading.active_count() != 1:
        raise SupervisionError('atomic launcher requires an isolated single-threaded broker')
    exe, command_line = _atomic_command_line(command, env)
    kernel.GetCurrentProcess.restype = wintypes.HANDLE
    kernel.DuplicateHandle.argtypes = [wintypes.HANDLE, wintypes.HANDLE, wintypes.HANDLE,
                                       ctypes.POINTER(wintypes.HANDLE), wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
    kernel.InitializeProcThreadAttributeList.argtypes = [ctypes.c_void_p, wintypes.DWORD, wintypes.DWORD, ctypes.POINTER(ctypes.c_size_t)]
    kernel.UpdateProcThreadAttribute.argtypes = [ctypes.c_void_p, wintypes.DWORD, ctypes.c_size_t,
                                                 ctypes.c_void_p, ctypes.c_size_t, ctypes.c_void_p, ctypes.c_void_p]
    kernel.DeleteProcThreadAttributeList.argtypes = [ctypes.c_void_p]
    kernel.CreateProcessW.argtypes = [wintypes.LPCWSTR, wintypes.LPWSTR, ctypes.c_void_p, ctypes.c_void_p,
                                      wintypes.BOOL, wintypes.DWORD, ctypes.c_void_p, wintypes.LPCWSTR,
                                      ctypes.POINTER(StartupInfoEx), ctypes.POINTER(ProcessInfo)]
    kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
    kernel.GetExitCodeProcess.argtypes = [wintypes.HANDLE, ctypes.POINTER(wintypes.DWORD)]
    size = ctypes.c_size_t()
    attributes = 1 if broker else 2
    kernel.InitializeProcThreadAttributeList(None, attributes, 0, ctypes.byref(size))
    if not size.value:
        raise SupervisionError('attribute list size unavailable')
    buffer = ctypes.create_string_buffer(size.value)
    if not kernel.InitializeProcThreadAttributeList(buffer, attributes, 0, ctypes.byref(size)):
        raise SupervisionError('attribute list initialization failed')
    handles = []
    try:
        with open(os.devnull, 'r+b') as null:
            for file in (() if broker else (null, stdout or null, stderr or null)):
                handle = wintypes.HANDLE()
                if not kernel.DuplicateHandle(kernel.GetCurrentProcess(), msvcrt.get_osfhandle(file.fileno()),
                                              kernel.GetCurrentProcess(), ctypes.byref(handle), 0, True, 2):
                    raise SupervisionError('stdio handle duplication failed')
                handles.append(handle)
            handle_list = (wintypes.HANDLE * len(handles))(*(h.value for h in handles))
            job_list = (wintypes.HANDLE * 1)(job)
            if not broker and not kernel.UpdateProcThreadAttribute(buffer, 0, 0x20002, handle_list, ctypes.sizeof(handle_list), None, None):
                raise SupervisionError('stdio allowlist setup failed')
            if not kernel.UpdateProcThreadAttribute(buffer, 0, 0x2000d, job_list, ctypes.sizeof(job_list), None, None):
                raise SupervisionError('atomic job assignment unavailable')
            startup = StartupInfoEx()
            startup.StartupInfo.cb = ctypes.sizeof(startup)
            if not broker:
                startup.StartupInfo.dwFlags = 0x100
                startup.StartupInfo.hStdInput, startup.StartupInfo.hStdOutput, startup.StartupInfo.hStdError = (h.value for h in handles)
            startup.lpAttributeList = ctypes.cast(buffer, ctypes.c_void_p)
            args = ctypes.create_unicode_buffer(command_line)
            environment = None if env is None else ctypes.create_unicode_buffer(
                '\0'.join(key + '=' + value for key, value in sorted(env.items(), key=lambda item: item[0].upper())) + '\0\0')
            info = ProcessInfo()
            if not kernel.CreateProcessW(exe, args, None, None, not broker, 0x80000 | 0x400,
                                         environment, str(cwd) if cwd is not None else None,
                                         ctypes.byref(startup), ctypes.byref(info)):
                raise SupervisionError('atomic process creation failed')
            try:
                _atomic_created_hook(info)
                kernel.CloseHandle(info.hThread)
                return Process(info.hProcess, info.dwProcessId)
            except BaseException as original:
                try:
                    _settle_windows(kernel, job, accounting, time.monotonic() + 10)
                except BaseException as cleanup:
                    raise BaseExceptionGroup('atomic launch and cleanup failed', [original, cleanup])
                finally:
                    kernel.CloseHandle(info.hThread)
                    kernel.CloseHandle(info.hProcess)
                raise
    finally:
        for handle in handles:
            kernel.CloseHandle(handle)
        kernel.DeleteProcThreadAttributeList(buffer)


def _run_windows(command, timeout, cleanup_timeout, cwd, env, stdout, stderr, capture, check):
    kernel, job, Accounting = _windows_job()
    process = None
    assigned = False
    failure = None
    try:
        process = subprocess.Popen(command, cwd=cwd, env=env, stdout=stdout, stderr=stderr,
                                   creationflags=0x00000004)  # CREATE_SUSPENDED
        if not _assign_windows(kernel, job, process):
            raise SupervisionError('cannot assign suspended process to job')
        assigned = True
        if _resume_windows(process) != 0:
            raise SupervisionError('cannot resume contained process')
        _wait_windows(process, timeout)
    except BaseException as error:
        failure = error
    cleanup_error = None
    try:
        if assigned:
            _settle_windows(kernel, job, Accounting, time.monotonic() + cleanup_timeout)
        elif process is not None:
            process.terminate()
        if process is not None:
            process.wait(timeout=cleanup_timeout)
    except BaseException as error:
        cleanup_error = error
    finally:
        if not kernel.CloseHandle(job):
            close_error = SupervisionError('cannot close owned job')
            cleanup_error = (BaseExceptionGroup('job cleanup and close failed', [cleanup_error, close_error])
                             if cleanup_error is not None else close_error)
    if cleanup_error is not None and process is not None and process.returncode is None:
        try:
            process.wait(timeout=cleanup_timeout)
        except BaseException as reap_error:
            cleanup_error = BaseExceptionGroup('cleanup and child reap failed', [cleanup_error, reap_error])
    if cleanup_error is not None:
        if failure is not None:
            raise BaseExceptionGroup('process execution and cleanup failed', [failure, cleanup_error])
        raise SupervisionError('process cleanup failed') from cleanup_error
    if failure is not None:
        raise failure
    if process is not None:
        return _result(command, process, stdout, stderr, capture, check)


def _broker_entry(address):
    """Single-purpose broker: receives handles already duplicated into itself."""
    import msvcrt
    from multiprocessing.connection import PipeClient
    connection = PipeClient(address)
    try:
        payload = json.loads(_pipe_io(connection.fileno(), None, time.monotonic() + 15, 32768))
        if set(payload) != {'command', 'stdin', 'stdout', 'stderr', 'cwd'}:
            raise ValueError('invalid broker request')
        with ExitStack() as streams:
            out = streams.enter_context(os.fdopen(msvcrt.open_osfhandle(payload['stdout'], os.O_WRONLY | os.O_BINARY), 'wb'))
            err = streams.enter_context(os.fdopen(msvcrt.open_osfhandle(payload['stderr'], os.O_WRONLY | os.O_BINARY), 'wb'))
            source = (subprocess.DEVNULL if payload['stdin'] is None else
                      streams.enter_context(os.fdopen(msvcrt.open_osfhandle(payload['stdin'], os.O_RDONLY | os.O_BINARY), 'rb')))
            executable, line = _atomic_command_line(payload['command'], os.environ)
            child = subprocess.Popen(line, executable=executable, cwd=payload['cwd'],
                                     stdin=source, stdout=out, stderr=err, close_fds=True)
            code = child.wait()
        _pipe_io(connection.fileno(), json.dumps({'exit_code': code}).encode(), time.monotonic() + 15, 1024)
    finally:
        connection.close()


def _duplicate_broker_output(kernel, process, file):
    import msvcrt
    from ctypes import wintypes
    value = wintypes.HANDLE()
    if not kernel.DuplicateHandle(kernel.GetCurrentProcess(), msvcrt.get_osfhandle(file.fileno()),
                                  process.handle, ctypes.byref(value), 0, False, 2):
        raise SupervisionError('cannot bind broker output handle')
    return value.value


def _pipe_io(handle, data, deadline, limit):
    """Transfer one bounded named-pipe message with an overlapped deadline."""
    import _winapi
    if data is not None and (not isinstance(data, bytes) or not 0 < len(data) <= limit):
        raise ValueError('invalid pipe message')
    try:
        ov, error = (_winapi.ReadFile(handle, limit, overlapped=True) if data is None else
                     _winapi.WriteFile(handle, data, overlapped=True))
        try:
            if error == _winapi.ERROR_IO_PENDING:
                remaining = max(0, int((deadline - time.monotonic()) * 1000))
                if _winapi.WaitForMultipleObjects([ov.event], False, remaining) != _winapi.WAIT_OBJECT_0:
                    raise subprocess.TimeoutExpired('broker pipe', max(0, remaining / 1000))
            count, error = ov.GetOverlappedResult(False)
            if error != 0 or count == 0 or (data is not None and count != len(data)):
                raise SupervisionError('incomplete broker pipe message')
            return bytes(ov.getbuffer()) if data is None else None
        finally:
            ov.cancel()
    except OSError as error:
        raise SupervisionError('broker pipe failed') from error


def _require_broker_pid(kernel, pipe, expected):
    from ctypes import wintypes
    pid = wintypes.DWORD()
    kernel.GetNamedPipeClientProcessId.argtypes = [wintypes.HANDLE, ctypes.POINTER(wintypes.DWORD)]
    if not kernel.GetNamedPipeClientProcessId(pipe, ctypes.byref(pid)) or pid.value != expected:
        raise SupervisionError('broker pipe client identity mismatch')


def _run_broker_experimental(command, *, timeout, cwd, env, stdout, stderr, stdin=None, cleanup_timeout=10):
    """Experimental broker; public API stays closed pending independent review.

    The first-instance random pipe checks the connected process PID before any
    handle values are sent. Same-user processes able to debug or alter either
    participant remain within the host trust boundary.
    The deadline bounds pipe I/O and process waits; process creation, handle
    duplication and filesystem operations have no independently proven bound.
    """
    import _winapi
    from ctypes import wintypes
    if os.name != 'nt' or stdout is None or stderr is None:
        raise SupervisionError('Windows and explicit owned output files required')
    if not isinstance(timeout, (int, float)) or not math.isfinite(timeout) or timeout <= 0:
        raise ValueError('positive finite timeout required')
    _atomic_command_line(command, env)
    encoded = json.dumps({'command': list(map(str, command)), 'cwd': str(cwd) if cwd is not None else None,
                          'stdin': 0, 'stdout': 0, 'stderr': 0}).encode()
    if len(encoded) > 16000:
        raise ValueError('broker request too large')
    address = r'\\.\pipe\voice-ci-' + uuid.uuid4().hex + uuid.uuid4().hex
    pipe = _winapi.CreateNamedPipe(address, _winapi.PIPE_ACCESS_DUPLEX | _winapi.FILE_FLAG_OVERLAPPED |
                                  _winapi.FILE_FLAG_FIRST_PIPE_INSTANCE,
                                  _winapi.PIPE_TYPE_MESSAGE | _winapi.PIPE_READMODE_MESSAGE | _winapi.PIPE_WAIT,
                                  1, 32768, 32768, 0, _winapi.NULL)
    try:
        kernel, job, accounting = _windows_job()
    except BaseException:
        _winapi.CloseHandle(pipe)
        raise
    process = None
    failure = None
    started = time.monotonic()
    result = None
    try:
        bootstrap = ('import runpy,sys;sys.argv=sys.argv[1:];'
                     'runpy.run_path(sys.argv[0],run_name="__main__")')
        process = _launch_atomic_windows(kernel, job, accounting,
            [sys.executable, '-I', '-S', '-B', '-c', bootstrap,
             str(Path(__file__).resolve()), '--broker', address], None, env, None, None, broker=True)
        ov = _winapi.ConnectNamedPipe(pipe, overlapped=True)
        try:
            remaining = max(0, int((timeout - (time.monotonic() - started)) * 1000))
            if _winapi.WaitForMultipleObjects([ov.event], False, remaining) != _winapi.WAIT_OBJECT_0:
                raise subprocess.TimeoutExpired(command, timeout)
            ov.GetOverlappedResult(True)
        finally:
            ov.cancel()
        _require_broker_pid(kernel, pipe, process.pid)
        remote = [_duplicate_broker_output(kernel, process, file) for file in (stdout, stderr)]
        remote_input = _duplicate_broker_output(kernel, process, stdin) if stdin is not None else None
        payload = {'command': list(map(str, command)), 'cwd': str(cwd) if cwd is not None else None,
                   'stdin': remote_input, 'stdout': remote[0], 'stderr': remote[1]}
        deadline = started + timeout
        _pipe_io(pipe, json.dumps(payload).encode(), deadline, 32768)
        response = json.loads(_pipe_io(pipe, None, deadline, 1024))
        if type(response) is not dict or set(response) != {'exit_code'} or type(response['exit_code']) is not int:
            raise SupervisionError('invalid broker response')
        remaining = timeout - (time.monotonic() - started)
        if remaining <= 0:
            raise subprocess.TimeoutExpired(command, timeout)
        process.wait(remaining)
        if process.returncode != 0:
            raise SupervisionError('broker failed')
        result = subprocess.CompletedProcess(command, response['exit_code'])
    except BaseException as error:
        failure = error
    try:
        _settle_windows(kernel, job, accounting, time.monotonic() + cleanup_timeout)
        if process is not None:
            process.wait(cleanup_timeout)
    except BaseException as cleanup:
        if failure is not None:
            raise BaseExceptionGroup('broker execution and cleanup failed', [failure, cleanup])
        raise SupervisionError('broker cleanup failed') from cleanup
    finally:
        _winapi.CloseHandle(pipe)
        if process is not None:
            process.close()
        kernel.CloseHandle(job)
    if failure is not None:
        raise failure
    return result


def run_diagnostic(command, *, timeout, cwd=None, env=None, stdout=None, stderr=None,
                   capture_output=False, check=False, input=None):
    """Run a Windows-only diagnostic with caller-owned binary output streams.

    This does not establish complete preflight or generic daemon cancellation.
    Input must be bytes of at most 64 MiB and is staged in a temporary file.
    Capture is limited to 64 MiB per stream. Explicit outputs must be fresh,
    distinct binary files exclusively owned by the caller for the run; external
    sharing cannot be verified. Pipe and process waits are bounded; OS process creation, handle duplication,
    filesystem operations and the same-user host trust boundary remain separate.
    """
    if os.name != 'nt':
        raise SupervisionError('Windows diagnostic broker required; Linux is unsupported')
    if (not isinstance(command, (list, tuple)) or not command or
            any(not isinstance(arg, str) or not arg or '\0' in arg for arg in command)):
        raise ValueError('command must contain nonempty string arguments')
    if (isinstance(timeout, bool) or not isinstance(timeout, (int, float)) or
            not math.isfinite(timeout) or timeout <= 0):
        raise ValueError('positive finite timeout required')
    if check is not False or type(capture_output) is not bool:
        raise ValueError('unsupported subprocess options')
    if input is not None and (type(input) is not bytes or len(input) > MAX_INPUT_BYTES):
        raise ValueError('input must be bounded bytes')
    if env is not None and (not isinstance(env, dict) or
                            any(not isinstance(k, str) or not isinstance(v, str) or
                                not k or '\0' in k or '=' in k or '\0' in v for k, v in env.items())):
        raise ValueError('environment must contain valid string entries')
    with ExitStack() as stack:
        source = None
        if input is not None:
            source = stack.enter_context(_private_windows_input())
            source.write(input)
            source.flush()
            source.seek(0)
        return _run_diagnostic_with_input(command, timeout, cwd, env, stdout, stderr, capture_output, source)


def _private_windows_input():
    """Create an exclusive, delete-on-last-close input file or fail closed."""
    import msvcrt
    from ctypes import wintypes
    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.CreateFileW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD,
                                   ctypes.c_void_p, wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    kernel.CreateFileW.restype = wintypes.HANDLE
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    path = str(Path(tempfile.gettempdir()) / ('voice-ci-input-' + uuid.uuid4().hex))
    # No sharing; CREATE_NEW; FILE_ATTRIBUTE_TEMPORARY | FILE_FLAG_DELETE_ON_CLOSE.
    handle = kernel.CreateFileW(path, 0xc0000000, 0, None, 1, 0x04000100, None)
    if handle == ctypes.c_void_p(-1).value:
        raise SupervisionError('cannot create private diagnostic input')
    try:
        fd = msvcrt.open_osfhandle(handle, os.O_RDWR | os.O_BINARY)
    except BaseException:
        kernel.CloseHandle(handle)
        raise
    try:
        return os.fdopen(fd, 'w+b')
    except BaseException:
        os.close(fd)
        raise


def _run_diagnostic_with_input(command, timeout, cwd, env, stdout, stderr, capture_output, source):
    if capture_output:
        if stdout is not None or stderr is not None:
            raise ValueError('capture_output cannot be combined with explicit streams')
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            _validate_output_files(out, err)
            result = _run_broker_experimental(command, timeout=timeout, cwd=cwd, env=env,
                                              stdout=out, stderr=err, stdin=source)
            out.seek(0); err.seek(0)
            output, errors = out.read(MAX_CAPTURE_BYTES + 1), err.read(MAX_CAPTURE_BYTES + 1)
            if len(output) > MAX_CAPTURE_BYTES or len(errors) > MAX_CAPTURE_BYTES:
                raise ValueError('capture exceeds per-stream limit')
            return subprocess.CompletedProcess(command, result.returncode, output, errors)
    if stdout is None or stderr is None:
        raise ValueError('explicit stdout and stderr are required')
    _validate_output_files(stdout, stderr)
    return _run_broker_experimental(command, timeout=timeout, cwd=cwd, env=env,
                                    stdout=stdout, stderr=stderr, stdin=source)


def _validate_output_files(stdout, stderr):
    identities = []
    for stream in (stdout, stderr):
        if isinstance(stream, io.TextIOBase) or not stream.writable() or not stream.seekable():
            raise ValueError('fresh writable binary output files are required')
        stream.flush()
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_size != 0 or stream.tell() != 0:
            raise ValueError('fresh writable binary output files are required')
        if not info.st_ino:
            raise ValueError('output identity unavailable')
        identities.append((info.st_dev, info.st_ino))
    if identities[0] == identities[1]:
        raise ValueError('output files must be distinct')


if __name__ == '__main__' and len(sys.argv) == 3 and sys.argv[1] == '--broker':
    _broker_entry(sys.argv[2])
