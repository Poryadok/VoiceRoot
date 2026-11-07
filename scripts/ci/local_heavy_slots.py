"""Process-held local heavy-run slots; resource ownership is separate."""
import errno
import json
import math
import os
from pathlib import Path
import re
import stat
import sys
import time
import uuid
from contextlib import contextmanager


if os.name == 'nt':
    import msvcrt
else:
    import fcntl


class SlotAcquireTimeout(TimeoutError):
    """A confirmed timeout before this caller owns any slot."""


_RESOURCE_BUDGET_FILE = 'resource-capacity.json'
_RESOURCE_FIELDS = {'nano_cpus', 'memory_bytes', 'pids_limit'}
_ADMISSION_CLOSE_TIMEOUT_SECONDS = 60


def _positive_int_map(value, fields, label):
    if (not isinstance(value, dict) or set(value) != set(fields)
            or any(type(value[key]) is not int or value[key] <= 0 for key in fields)):
        raise ValueError(label + ' must contain positive integer ' + ', '.join(sorted(fields)))
    return {key: value[key] for key in fields}


def measured_host_capacity():
    """Read physical CPU and RAM ceilings; this does not account for other users."""
    if os.name == 'nt':
        import ctypes
        from ctypes import wintypes

        class MemoryStatusEx(ctypes.Structure):
            _fields_ = [('dwLength', wintypes.DWORD), ('dwMemoryLoad', wintypes.DWORD),
                        ('ullTotalPhys', ctypes.c_ulonglong), ('ullAvailPhys', ctypes.c_ulonglong),
                        ('ullTotalPageFile', ctypes.c_ulonglong), ('ullAvailPageFile', ctypes.c_ulonglong),
                        ('ullTotalVirtual', ctypes.c_ulonglong), ('ullAvailVirtual', ctypes.c_ulonglong),
                        ('ullAvailExtendedVirtual', ctypes.c_ulonglong)]

        status = MemoryStatusEx()
        status.dwLength = ctypes.sizeof(status)
        if not ctypes.windll.kernel32.GlobalMemoryStatusEx(ctypes.byref(status)):
            raise ValueError('physical host memory capacity is unavailable')
        processors = ctypes.windll.kernel32.GetActiveProcessorCount(0xFFFF)
        if not processors or not status.ullTotalPhys:
            raise ValueError('physical host capacity is unavailable')
        return {'nano_cpus': int(processors) * 1_000_000_000,
                'memory_bytes': int(status.ullTotalPhys)}
    if sys.platform.startswith('linux'):
        processors = os.cpu_count()
        pages = os.sysconf('SC_PHYS_PAGES')
        page_size = os.sysconf('SC_PAGE_SIZE')
        if not processors or pages <= 0 or page_size <= 0:
            raise ValueError('physical host capacity is unavailable')
        return {'nano_cpus': processors * 1_000_000_000, 'memory_bytes': pages * page_size}
    raise ValueError('physical host capacity is unsupported on this platform')


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


def _owned_slot_marker_written(descriptor, slot, run_id):
    """Test seam immediately after the owner marker is durable."""


class Lease:
    def __init__(self, descriptor, slot, marker, directory, owner_id=None):
        self._descriptor = descriptor
        self.slot = slot
        self._marker = marker
        self._directory = Path(directory)
        self.owner_id = owner_id

    def close(self, *, cleanup_verified=False):
        if self._descriptor is None:
            return
        descriptor = self._descriptor
        try:
            admission = _lock_admission(self._directory,
                                        time.monotonic() + _ADMISSION_CLOSE_TIMEOUT_SECONDS)
        except BaseException:
            self.abandon()
            raise
        self._descriptor = None
        evidence = None
        try:
            if _marker(descriptor) != self._marker:
                raise ValueError('slot ownership marker changed')
            if (self.owner_id is not None and
                    self.owner_id in _resource_state(self._directory)['reservations'] and
                    cleanup_verified is not True):
                raise ValueError('bounded reservation release requires verified cleanup')
            _write_marker(descriptor, b'idle\n')
            if self.owner_id is not None:
                _release_reservation_locked(self._directory, self.owner_id)
            current = _resource_state(self._directory)
            evidence = {'slot': self.slot, 'owner_id': self.owner_id,
                        'marker_idle': _marker(descriptor) == b'idle\n',
                        'reservation_released': (self.owner_id is None or
                                                 self.owner_id not in current['reservations'])}
        finally:
            os.close(descriptor)
            _unlock_close(admission)
        return evidence

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


def _lock_admission(directory, deadline):
    path = Path(directory) / 'capacity.lock'
    flags = os.O_RDWR | getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0)
    try:
        descriptor = os.open(path, flags)
    except OSError as error:
        raise ValueError('capacity admission lock is unavailable') from error
    try:
        if not stat.S_ISREG(os.fstat(descriptor).st_mode) or path.is_symlink():
            raise ValueError('capacity lock must be a regular file')
        while not _try_lock(descriptor):
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise SlotAcquireTimeout('local heavy CI capacity admission is locked')
            time.sleep(min(0.01, remaining))
        return descriptor
    except BaseException:
        os.close(descriptor)
        raise


def _unlock_close(descriptor):
    if descriptor is not None:
        if os.name == 'nt':
            os.lseek(descriptor, 0, os.SEEK_SET)
            try:
                msvcrt.locking(descriptor, msvcrt.LK_UNLCK, 1)
            except OSError:
                pass
        else:
            fcntl.flock(descriptor, fcntl.LOCK_UN)
        os.close(descriptor)


def _resource_state(directory):
    path = Path(directory) / _RESOURCE_BUDGET_FILE
    if path.is_symlink():
        raise ValueError('resource budget journal is not a regular file')
    if not path.exists():
        return {'version': 1, 'budget': None, 'reservations': {}}
    if not path.is_file():
        raise ValueError('resource budget journal is not a regular file')
    raw = path.read_bytes()
    try:
        state = json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError):
        raise ValueError('resource budget journal is invalid') from None
    if (not isinstance(state, dict) or set(state) != {'version', 'budget', 'reservations'}
            or type(state['version']) is not int or state['version'] != 1
            or not isinstance(state['reservations'], dict)):
        raise ValueError('resource budget journal is invalid')
    if state['budget'] is not None:
        state['budget'] = _positive_int_map(state['budget'], _RESOURCE_FIELDS, 'capacity budget')
    reserved_slots = set()
    for owner_id, reservation in state['reservations'].items():
        if (not isinstance(owner_id, str) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', owner_id)
                or not isinstance(reservation, dict) or set(reservation) != {'claim', 'slot'}
                or type(reservation['slot']) is not int or reservation['slot'] < 0):
            raise ValueError('resource budget journal is invalid')
        if reservation['slot'] in reserved_slots:
            raise ValueError('resource budget journal assigns a slot to multiple owners')
        reserved_slots.add(reservation['slot'])
        reservation['claim'] = _positive_int_map(reservation['claim'], _RESOURCE_FIELDS, 'resource claim')
    canonical = (json.dumps(state, sort_keys=True, separators=(',', ':')) + '\n').encode('utf-8')
    if raw != canonical:
        raise ValueError('resource budget journal is not canonical')
    return state


def _save_resource_state(directory, state):
    path = Path(directory) / _RESOURCE_BUDGET_FILE
    if path.is_symlink() or (path.exists() and not path.is_file()):
        raise ValueError('resource budget journal is not a regular file')
    temporary = Path(directory) / ('.resource-capacity-' + uuid.uuid4().hex + '.tmp')
    data = (json.dumps(state, sort_keys=True, separators=(',', ':')) + '\n').encode('utf-8')
    descriptor = None
    try:
        descriptor = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL |
                             getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0), 0o600)
        if not stat.S_ISREG(os.fstat(descriptor).st_mode):
            raise ValueError('resource journal temporary is not a regular file')
        if os.write(descriptor, data) != len(data):
            raise OSError('short resource budget journal write')
        os.fsync(descriptor)
        os.close(descriptor)
        descriptor = None
        os.replace(temporary, path)
        if os.name != 'nt':
            dir_fd = os.open(directory, os.O_RDONLY | getattr(os, 'O_DIRECTORY', 0))
            try:
                os.fsync(dir_fd)
            finally:
                os.close(dir_fd)
    finally:
        if descriptor is not None:
            os.close(descriptor)
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def _busy_markers(directory, capacity):
    result = []
    for slot in range(capacity):
        path = Path(directory) / ('heavy-' + str(slot) + '.lock')
        if not path.exists():
            continue
        if path.is_symlink() or not path.is_file():
            raise ValueError('slot file must be a regular file')
        descriptor = os.open(path, os.O_RDONLY | getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0))
        try:
            os.lseek(descriptor, 1, os.SEEK_SET)
            marker = os.read(descriptor, 128)
        finally:
            os.close(descriptor)
        if marker != b'idle\n':
            result.append((slot, marker))
    return result


def active_resource_reservation(directory, owner_id):
    if not isinstance(owner_id, str) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', owner_id):
        raise ValueError('invalid resource reservation owner')
    state = _resource_state(directory)
    reservation = state['reservations'].get(owner_id)
    return None if reservation is None else dict(reservation['claim'])


def resource_reservation_slot(directory, owner_id):
    if not isinstance(owner_id, str) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', owner_id):
        raise ValueError('invalid resource reservation owner')
    reservation = _resource_state(directory)['reservations'].get(owner_id)
    return None if reservation is None else reservation['slot']


def _release_reservation_locked(directory, owner_id):
    state = _resource_state(directory)
    if owner_id not in state['reservations']:
        return
    del state['reservations'][owner_id]
    _save_resource_state(directory, state)


def release_resource_reservation(directory, owner_id, *, cleanup_verified):
    if cleanup_verified is not True:
        raise ValueError('reservation release requires verified cleanup')
    if not isinstance(owner_id, str) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', owner_id):
        raise ValueError('invalid resource reservation owner')
    admission = _lock_admission(directory, time.monotonic())
    try:
        _release_reservation_locked(directory, owner_id)
    finally:
        _unlock_close(admission)


def _validate_capacity_budget(capacity_budget):
    budget = _positive_int_map(capacity_budget, _RESOURCE_FIELDS, 'capacity budget')
    measured = measured_host_capacity()
    if budget['nano_cpus'] > measured['nano_cpus'] or budget['memory_bytes'] > measured['memory_bytes']:
        raise ValueError('capacity budget exceeds measured host capacity')
    return budget


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


def acquire(directory, timeout, capacity=2, *, owner_id=None, resource_claim=None, capacity_budget=None):
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
    bounded = resource_claim is not None or capacity_budget is not None
    if bounded and (owner_id is None or resource_claim is None or capacity_budget is None):
        raise ValueError('bounded admission requires owner, resource claim and capacity budget')
    claim = _positive_int_map(resource_claim, _RESOURCE_FIELDS, 'resource claim') if resource_claim is not None else None
    budget = _validate_capacity_budget(capacity_budget) if capacity_budget is not None else None
    if owner_id is not None and (not isinstance(owner_id, str) or
                                 not re.fullmatch(r'voiceci-[0-9a-f]{32}', owner_id)):
        raise ValueError('invalid slot owner identity')
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    if not directory.is_dir() or directory.is_symlink() or directory.resolve() != directory.absolute():
        raise ValueError('slot directory must be a real directory')
    deadline = time.monotonic() + timeout
    try:
        _configure(directory, capacity, deadline)
    except TimeoutError as error:
        raise SlotAcquireTimeout(str(error)) from error
    while True:
        wait_seconds = None
        admission = _lock_admission(directory, deadline)
        try:
            state = _resource_state(directory)
            busy = _busy_markers(directory, capacity)
            if bounded:
                if state['budget'] is not None and state['budget'] != budget:
                    raise ValueError('capacity budget differs from persisted host budget')
                if any(claim[key] > budget[key] for key in _RESOURCE_FIELDS):
                    raise ValueError('resource claim exceeds capacity budget')
                if any(not re.fullmatch(rb'busy:[0-9a-f]{32}:voiceci-[0-9a-f]{32}\n', marker)
                       or marker.split(b':', 2)[2].rstrip(b'\n').decode('ascii') not in state['reservations']
                       for _, marker in busy):
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        raise SlotAcquireTimeout('unbounded or unaccounted local heavy run is active')
                    wait_seconds = min(0.05, remaining)
                if wait_seconds is None:
                    total = {key: sum(row['claim'][key] for row in state['reservations'].values()) + claim[key]
                             for key in _RESOURCE_FIELDS}
                if wait_seconds is None and any(total[key] > budget[key] for key in _RESOURCE_FIELDS):
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        raise SlotAcquireTimeout('local heavy CI resource capacity budget is fully reserved')
                    wait_seconds = min(0.05, remaining)
                if wait_seconds is None and owner_id in state['reservations']:
                    raise ValueError('resource reservation owner is already active')
            elif state['reservations']:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise SlotAcquireTimeout('bounded local heavy run is active')
                wait_seconds = min(0.05, remaining)

            if wait_seconds is None:
                selected = None
                reserved_slots = {row['slot'] for row in state['reservations'].values()}
                for slot in range(capacity):
                    if slot in reserved_slots:
                        continue
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
                        if _try_lock(descriptor) and _marker(descriptor) == b'idle\n':
                            selected = (slot, descriptor)
                            break
                    except BaseException:
                        os.close(descriptor)
                        raise
                    os.close(descriptor)
                if selected is None:
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        raise SlotAcquireTimeout('all local heavy CI slots are occupied')
                    wait_seconds = min(0.05, remaining)
                else:
                    slot, descriptor = selected
                    try:
                        if bounded:
                            if state['budget'] is None:
                                state['budget'] = budget
                            state['reservations'][owner_id] = {'claim': claim, 'slot': slot}
                            _save_resource_state(directory, state)
                        marker_value = 'busy:' + uuid.uuid4().hex
                        if owner_id is not None:
                            marker_value += ':' + owner_id
                        marker = (marker_value + '\n').encode('ascii')
                        _write_marker(descriptor, marker)
                        if owner_id is not None:
                            _owned_slot_marker_written(descriptor, slot, owner_id)
                        return Lease(descriptor, slot, marker, directory, owner_id)
                    except BaseException:
                        os.close(descriptor)
                        raise
        finally:
            _unlock_close(admission)
        if wait_seconds is None:
            raise AssertionError('slot admission neither acquired nor scheduled a retry')
        time.sleep(wait_seconds)


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
    admission = _lock_admission(directory, time.monotonic())
    try:
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
            state = _resource_state(directory)
            reservation = state['reservations'].get(run_id)
            reserved_idle = (reservation is not None and reservation['slot'] == slot and marker == b'idle\n')
            if reservation is not None and reservation['slot'] != slot:
                raise ValueError('resource reservation names a different slot')
            if not (re.fullmatch(expected, marker) or reserved_idle):
                raise ValueError('slot does not name the requested owner')
            return Lease(descriptor, slot, marker, directory, run_id)
        except BaseException:
            os.close(descriptor)
            raise
    finally:
        _unlock_close(admission)
