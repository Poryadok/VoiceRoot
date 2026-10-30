"""Run an owned process tree with bounded execution and verified cleanup.

Windows uses a kill-on-close Job Object. Linux is refused until an external
crash-safe containment and bounded launcher are available. Never treat a
cleanup error as a successful command. A supervisor crash between suspended
process creation and job assignment can leave that process uncontained; this
window is not crash-safe and the caller must not claim crash-safe execution.
"""
import ctypes
import math
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import threading
import time


class SupervisionError(RuntimeError):
    pass


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


def _launch_atomic_windows(kernel, job, accounting, command, cwd, env, stdout, stderr):
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
        def __init__(self, handle):
            self.handle, self.returncode = handle, None
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
    if threading.active_count() != 1:
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
    kernel.InitializeProcThreadAttributeList(None, 2, 0, ctypes.byref(size))
    if not size.value:
        raise SupervisionError('attribute list size unavailable')
    buffer = ctypes.create_string_buffer(size.value)
    if not kernel.InitializeProcThreadAttributeList(buffer, 2, 0, ctypes.byref(size)):
        raise SupervisionError('attribute list initialization failed')
    handles = []
    try:
        with open(os.devnull, 'r+b') as null:
            for file in (null, stdout or null, stderr or null):
                handle = wintypes.HANDLE()
                if not kernel.DuplicateHandle(kernel.GetCurrentProcess(), msvcrt.get_osfhandle(file.fileno()),
                                              kernel.GetCurrentProcess(), ctypes.byref(handle), 0, True, 2):
                    raise SupervisionError('stdio handle duplication failed')
                handles.append(handle)
            handle_list = (wintypes.HANDLE * 3)(*(h.value for h in handles))
            job_list = (wintypes.HANDLE * 1)(job)
            if not kernel.UpdateProcThreadAttribute(buffer, 0, 0x20002, handle_list, ctypes.sizeof(handle_list), None, None):
                raise SupervisionError('stdio allowlist setup failed')
            if not kernel.UpdateProcThreadAttribute(buffer, 0, 0x2000d, job_list, ctypes.sizeof(job_list), None, None):
                raise SupervisionError('atomic job assignment unavailable')
            startup = StartupInfoEx()
            startup.StartupInfo.cb = ctypes.sizeof(startup)
            startup.StartupInfo.dwFlags = 0x100
            startup.StartupInfo.hStdInput, startup.StartupInfo.hStdOutput, startup.StartupInfo.hStdError = (h.value for h in handles)
            startup.lpAttributeList = ctypes.cast(buffer, ctypes.c_void_p)
            args = ctypes.create_unicode_buffer(command_line)
            environment = None if env is None else ctypes.create_unicode_buffer(
                '\0'.join(key + '=' + value for key, value in sorted(env.items(), key=lambda item: item[0].upper())) + '\0\0')
            info = ProcessInfo()
            if not kernel.CreateProcessW(exe, args, None, None, True, 0x80000 | 0x400,
                                         environment, str(cwd) if cwd is not None else None,
                                         ctypes.byref(startup), ctypes.byref(info)):
                raise SupervisionError('atomic process creation failed')
            try:
                _atomic_created_hook(info)
                kernel.CloseHandle(info.hThread)
                return Process(info.hProcess)
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
    try:
        if assigned:
            _settle_windows(kernel, job, Accounting, time.monotonic() + cleanup_timeout)
        elif process is not None:
            process.terminate()
        if process is not None:
            process.wait(timeout=cleanup_timeout)
    except BaseException as cleanup_error:
        if failure is not None:
            raise BaseExceptionGroup('process execution and cleanup failed', [failure, cleanup_error])
        raise SupervisionError('process cleanup failed') from cleanup_error
    finally:
        kernel.CloseHandle(job)
    if failure is not None:
        raise failure
    if process is not None:
        return _result(command, process, stdout, stderr, capture, check)


