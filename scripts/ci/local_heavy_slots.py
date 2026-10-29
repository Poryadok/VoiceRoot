"""Process-held local heavy-run slots; resource ownership is separate."""
import errno
import math
import os
from pathlib import Path
import stat
import time
import uuid


if os.name == 'nt':
    import msvcrt
else:
    import fcntl


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


def acquire(directory, timeout):
    """Acquire one of two host-shared slots or fail without starting work.

    Callers must use the same trusted host-local directory and hold the returned
    lease through their complete resource cleanup. A crashed process leaves its
    slot unavailable until a future owner-aware recovery procedure verifies
    cleanup. This primitive does not label, clean, or isolate Docker resources.
    """
    if type(timeout) not in (int, float) or not math.isfinite(timeout) or timeout < 0:
        raise ValueError('timeout must be finite and nonnegative')
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    if not directory.is_dir() or directory.is_symlink():
        raise ValueError('slot directory must be a real directory')
    deadline = time.monotonic() + timeout
    while True:
        for slot in range(2):
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
            raise TimeoutError('both local heavy CI slots are occupied')
        time.sleep(min(0.05, remaining))
