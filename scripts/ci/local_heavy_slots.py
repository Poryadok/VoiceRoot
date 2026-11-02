"""Process-held local heavy-run slots; resource ownership is separate."""
import errno
import math
import os
from pathlib import Path
import re
import stat
import sys
import time
import uuid


if os.name == 'nt':
    import msvcrt
else:
    import fcntl


def host_slot_directory():
    """Return one OS-wide location for cooperating local CI schedulers.

    Lack of access must fail closed; there is no per-worktree fallback.
    """
    if os.name == 'nt':
        import ctypes
        buffer = ctypes.create_unicode_buffer(32768)
        if ctypes.windll.shell32.SHGetFolderPathW(None, 0x23, None, 0, buffer) != 0 or not buffer.value:
            raise ValueError('Windows common application data directory is unavailable')
        base = Path(buffer.value)
    elif sys.platform.startswith('linux'):
        base = Path('/var/tmp')
    else:
        raise ValueError('host-wide CI scheduling is unsupported on this platform')
    if not base.is_absolute() or not base.is_dir():
        raise ValueError('host-wide CI scheduling base is unavailable')
    return base / 'voice-ci-heavy-slots-v1'


def _marker(descriptor):
    os.lseek(descriptor, 1, os.SEEK_SET)
    return os.read(descriptor, 128)


def _write_marker(descriptor, value):
    os.lseek(descriptor, 1, os.SEEK_SET)
    if os.write(descriptor, value) != len(value):
        raise OSError('short slot marker write')
    os.ftruncate(descriptor, 1 + len(value))
    os.fsync(descriptor)


class Lease:
    def __init__(self, descriptor, slot, marker):
        self._descriptor = descriptor
        self.slot = slot
        self._marker = marker

    def close(self):
        if self._descriptor is None:
            return
        descriptor, self._descriptor = self._descriptor, None
        try:
            if _marker(descriptor) != self._marker:
                raise ValueError('slot ownership marker changed')
            _write_marker(descriptor, b'idle\n')
        finally:
            os.close(descriptor)

    def bind_owner(self, run_id):
        if (self._descriptor is None or not isinstance(run_id, str)
                or not re.fullmatch(r'voiceci-[0-9a-f]{32}', run_id)
                or not re.fullmatch(rb'busy:[0-9a-f]{32}\n', self._marker)
                or _marker(self._descriptor) != self._marker):
            raise ValueError('invalid or changed slot owner binding')
        marker = self._marker[:-1] + b':' + run_id.encode('ascii') + b'\n'
        _write_marker(self._descriptor, marker)
        self._marker = marker

    def __enter__(self):
        return self

    def abandon(self):
        if self._descriptor is not None:
            descriptor, self._descriptor = self._descriptor, None
            os.close(descriptor)

    def __exit__(self, exception_type, *_):
        if exception_type is None:
            self.close()
        else:
            self.abandon()


def _try_lock(descriptor):
    if os.name == 'nt':
        os.lseek(descriptor, 0, os.SEEK_SET)
        try:
            msvcrt.locking(descriptor, msvcrt.LK_NBLCK, 1)
            return True
        except OSError as error:
            if error.errno in (errno.EACCES, errno.EAGAIN, errno.EDEADLK):
                return False
            raise
    try:
        fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        return True
    except BlockingIOError:
        return False


def _configure(directory, capacity, deadline):
    path = directory / 'capacity.lock'
    flags = os.O_RDWR | getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0)
    try:
        descriptor = os.open(path, flags | os.O_CREAT | os.O_EXCL, 0o600)
        created = True
    except FileExistsError:
        descriptor = os.open(path, flags)
        created = False
    try:
        if not stat.S_ISREG(os.fstat(descriptor).st_mode) or path.is_symlink():
            raise ValueError('capacity lock must be a regular file')
        if created:
            if os.write(descriptor, b'\0') != 1:
                raise OSError('short capacity lock write')
            os.fsync(descriptor)
        else:
            while os.fstat(descriptor).st_size == 0:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise ValueError('capacity lock initialization incomplete')
                time.sleep(min(0.05, remaining))
        while not _try_lock(descriptor):
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError('local heavy CI capacity configuration is locked')
            time.sleep(min(0.05, remaining))
        for existing in directory.glob('heavy-*.lock'):
            match = re.fullmatch(r'heavy-(0|[1-9][0-9]*)\.lock', existing.name)
            if not match or int(match.group(1)) >= capacity:
                raise ValueError('capacity would hide an existing slot')
        config = directory / 'capacity'
        if config.is_symlink():
            raise ValueError('capacity configuration must be a regular file')
        try:
            config_fd = os.open(config, os.O_WRONLY | os.O_CREAT | os.O_EXCL |
                                getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0), 0o600)
        except FileExistsError:
            if not config.is_file() or config.read_bytes() != (str(capacity) + '\n').encode('ascii'):
                raise ValueError('capacity configuration differs or is invalid')
        else:
            try:
                value = (str(capacity) + '\n').encode('ascii')
                if os.write(config_fd, value) != len(value):
                    raise OSError('short capacity configuration write')
                os.fsync(config_fd)
            finally:
                os.close(config_fd)
    finally:
        os.close(descriptor)


def acquire(directory, timeout, capacity=2):
    """Acquire a configured host-shared slot or fail without starting work.

    Callers must use the same trusted host-local directory and hold the returned
    lease through their complete resource cleanup. A crashed process leaves its
    slot unavailable until a future owner-aware recovery procedure verifies
    cleanup. This primitive does not label, clean, or isolate Docker resources.
    """
    if type(timeout) not in (int, float) or not math.isfinite(timeout) or timeout < 0:
        raise ValueError('timeout must be finite and nonnegative')
    if type(capacity) is not int or not 1 <= capacity <= 64:
        raise ValueError('capacity must be an integer between 1 and 64')
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    if not directory.is_dir() or directory.is_symlink() or directory.resolve() != directory.absolute():
        raise ValueError('slot directory must be a real directory')
    deadline = time.monotonic() + timeout
    _configure(directory, capacity, deadline)
    while True:
        for slot in range(capacity):
            path = directory / ('heavy-' + str(slot) + '.lock')
            flags = os.O_RDWR | getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0)
            try:
                descriptor = os.open(path, flags | os.O_CREAT | os.O_EXCL, 0o600)
                created = True
            except FileExistsError:
                descriptor = os.open(path, flags)
                created = False
            try:
                if not stat.S_ISREG(os.fstat(descriptor).st_mode) or path.is_symlink():
                    raise ValueError('slot file must be a regular file')
                if created:
                    if os.write(descriptor, b'\0idle\n') != 6:
                        raise OSError('short initial slot marker write')
                    os.fsync(descriptor)
                if _try_lock(descriptor):
                    if _marker(descriptor) == b'idle\n':
                        marker = ('busy:' + uuid.uuid4().hex + '\n').encode('ascii')
                        _write_marker(descriptor, marker)
                        return Lease(descriptor, slot, marker)
            except BaseException:
                os.close(descriptor)
                raise
            os.close(descriptor)
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError('all local heavy CI slots are occupied')
        time.sleep(min(0.05, remaining))


def recover_bound(directory, slot, run_id, capacity=2):
    """Lock an abandoned slot only when it names the exact owned run."""
    if type(capacity) is not int or not 1 <= capacity <= 64 or type(slot) is not int or not 0 <= slot < capacity:
        raise ValueError('invalid recovery slot or capacity')
    if not isinstance(run_id, str) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', run_id):
        raise ValueError('invalid recovery run identity')
    directory = Path(directory)
    if not directory.is_dir() or directory.is_symlink() or directory.resolve() != directory.absolute():
        raise ValueError('slot directory must be a real directory')
    _configure(directory, capacity, time.monotonic())
    path = directory / ('heavy-' + str(slot) + '.lock')
    flags = os.O_RDWR | getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0)
    descriptor = os.open(path, flags)
    try:
        if path.is_symlink() or not stat.S_ISREG(os.fstat(descriptor).st_mode):
            raise ValueError('slot file must be a regular file')
        if not _try_lock(descriptor):
            raise ValueError('slot owner is still active')
        marker = _marker(descriptor)
        expected = rb'busy:[0-9a-f]{32}:' + run_id.encode('ascii') + rb'\n'
        if not re.fullmatch(expected, marker):
            raise ValueError('slot does not name the requested owner')
        return Lease(descriptor, slot, marker)
    except BaseException:
        os.close(descriptor)
        raise
