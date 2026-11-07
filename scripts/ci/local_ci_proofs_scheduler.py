"""Owner-bound, host-shared scheduler for the local CI proofs shard.

The journal is an intent record, written before taking a slot. Recovery only
releases an exact owner-bound slot after every journaled named Job Object is
proven absent. The host directory and named-object namespace are cooperative
same-user resources; this is not protection against a same-user process able
to alter the journal, slot files, or Job Objects.
"""
import json
import os
from pathlib import Path
import re
import tempfile
import uuid

import local_heavy_slots


_RUN_ID = re.compile(r'voiceci-[0-9a-f]{32}\Z')
_JOB_NAME = re.compile(r'Local\\VoiceCIProof-[A-Za-z0-9_-]{1,96}\Z')
_STAGES = {'make-dry-run', 'make'}


def _session_identity():
    r"""Return a boot, Windows session, and logon identity for Local\ Jobs."""
    if os.name != 'nt':
        return {'boot_id': 'nonwindows', 'logon_id': 'nonwindows', 'session_id': 0}
    import ctypes
    from ctypes import wintypes

    class Luid(ctypes.Structure):
        _fields_ = [('LowPart', wintypes.DWORD), ('HighPart', wintypes.LONG)]

    class TokenStatistics(ctypes.Structure):
        _fields_ = [('TokenId', Luid), ('AuthenticationId', Luid),
                    ('ExpirationTime', ctypes.c_longlong), ('TokenType', wintypes.DWORD),
                    ('ImpersonationLevel', wintypes.DWORD), ('DynamicCharged', wintypes.DWORD),
                    ('DynamicAvailable', wintypes.DWORD), ('GroupCount', wintypes.DWORD),
                    ('PrivilegeCount', wintypes.DWORD), ('ModifiedId', Luid)]

    class BootInfo(ctypes.Structure):
        _fields_ = [('BootIdentifier', ctypes.c_ubyte * 16),
                    ('FirmwareType', wintypes.DWORD), ('BootFlags', ctypes.c_ulonglong)]

    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.GetCurrentProcess.restype = wintypes.HANDLE
    kernel.ProcessIdToSessionId.argtypes = [wintypes.DWORD, ctypes.POINTER(wintypes.DWORD)]
    kernel.OpenProcessToken.argtypes = [wintypes.HANDLE, wintypes.DWORD,
                                        ctypes.POINTER(wintypes.HANDLE)]
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    session = wintypes.DWORD()
    if not kernel.ProcessIdToSessionId(os.getpid(), ctypes.byref(session)):
        raise OSError(ctypes.get_last_error(), 'cannot identify Windows session')

    advapi = ctypes.WinDLL('advapi32', use_last_error=True)
    advapi.GetTokenInformation.argtypes = [wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p,
                                           wintypes.DWORD, ctypes.POINTER(wintypes.DWORD)]
    token = wintypes.HANDLE()
    if not kernel.OpenProcessToken(kernel.GetCurrentProcess(), 0x0008, ctypes.byref(token)):
        raise OSError(ctypes.get_last_error(), 'cannot open current process token')
    try:
        stats, returned = TokenStatistics(), wintypes.DWORD()
        if (not advapi.GetTokenInformation(token, 10, ctypes.byref(stats),
                                            ctypes.sizeof(stats), ctypes.byref(returned)) or
                returned.value < ctypes.sizeof(stats)):
            raise OSError(ctypes.get_last_error(), 'cannot identify Windows logon session')
    finally:
        if not kernel.CloseHandle(token):
            raise OSError(ctypes.get_last_error(), 'cannot close process token handle')

    ntdll = ctypes.WinDLL('ntdll')
    ntdll.NtQuerySystemInformation.argtypes = [ctypes.c_int, ctypes.c_void_p,
                                               wintypes.ULONG, ctypes.POINTER(wintypes.ULONG)]
    ntdll.NtQuerySystemInformation.restype = wintypes.LONG
    boot, size = BootInfo(), wintypes.ULONG()
    if ntdll.NtQuerySystemInformation(90, ctypes.byref(boot), ctypes.sizeof(boot),
                                      ctypes.byref(size)) != 0:
        raise OSError('cannot identify Windows boot')
    boot_bytes = bytes(boot.BootIdentifier)
    if not any(boot_bytes):
        raise OSError('Windows boot identifier is empty')
    auth = stats.AuthenticationId
    return {'boot_id': str(uuid.UUID(bytes_le=boot_bytes)),
            'logon_id': f'{auth.HighPart & 0xffffffff:08x}{auth.LowPart:08x}',
            'session_id': int(session.value)}


def _safe_directory(directory, *, create):
    path = Path(directory)
    if create:
        path.mkdir(parents=True, exist_ok=True)
    if (not path.is_absolute() or not path.is_dir() or path.is_symlink() or
            path.resolve() != path.absolute()):
        raise ValueError('proof journal directory must be a real absolute directory')
    return path


def _durable_write(path, record):
    """Replace a journal atomically after flushing its complete contents."""
    if path.is_symlink():
        raise ValueError('proof journal must not be a symlink')
    payload = (json.dumps(record, sort_keys=True, separators=(',', ':')) + '\n').encode('utf-8')
    descriptor, temporary = tempfile.mkstemp(prefix='.proof-run-', suffix='.tmp', dir=path.parent)
    try:
        with os.fdopen(descriptor, 'wb') as stream:
            if os.name != 'nt':
                os.fchmod(stream.fileno(), 0o600)
            stream.write(payload)
            stream.flush()
            os.fsync(stream.fileno())
        if os.name == 'nt':
            import ctypes
            from ctypes import wintypes
            kernel = ctypes.WinDLL('kernel32', use_last_error=True)
            kernel.MoveFileExW.argtypes = [wintypes.LPCWSTR, wintypes.LPCWSTR, wintypes.DWORD]
            kernel.MoveFileExW.restype = wintypes.BOOL
            if not kernel.MoveFileExW(str(temporary), str(path), 0x1 | 0x8):
                raise OSError(ctypes.get_last_error(), 'write-through journal replacement failed')
        else:
            os.replace(temporary, path)
            directory_fd = os.open(path.parent, os.O_RDONLY | getattr(os, 'O_DIRECTORY', 0))
            try:
                os.fsync(directory_fd)
            finally:
                os.close(directory_fd)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def _durable_unlink(path):
    path.unlink()
    if os.name != 'nt':
        directory_fd = os.open(path.parent, os.O_RDONLY | getattr(os, 'O_DIRECTORY', 0))
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)


def _read_record(path, run_id):
    if path.is_symlink() or not path.is_file():
        raise ValueError('proof journal must be a regular file')
    if path.stat().st_size > 16384:
        raise ValueError('proof journal is too large')
    record = json.loads(path.read_text(encoding='utf-8'))
    identity = record.get('session_identity') if type(record) is dict else None
    if (type(record) is not dict or set(record) != {
            'schema', 'run_id', 'slot', 'job_names', 'session_identity'} or
            record.get('schema') != 1 or record.get('run_id') != run_id or
            type(identity) is not dict or set(identity) != {'boot_id', 'logon_id', 'session_id'} or
            not isinstance(identity.get('boot_id'), str) or not identity['boot_id'] or
            not isinstance(identity.get('logon_id'), str) or not identity['logon_id'] or
            type(identity.get('session_id')) is not int or identity['session_id'] < 0 or
            (record.get('slot') is not None and
             (type(record['slot']) is not int or not 0 <= record['slot'] < 64)) or
            type(record.get('job_names')) is not list or
            any(not isinstance(name, str) or not _JOB_NAME.fullmatch(name)
                for name in record['job_names']) or
            len(set(record['job_names'])) != len(record['job_names'])):
        raise ValueError('invalid proof journal contents')
    return record


class ProofRun:
    """One durable proof run, with its host-shared lease and named Jobs."""

    def __init__(self, journal_path, run_id, slot, lease, record):
        self.journal_path = journal_path
        self.run_id = run_id
        self.slot = slot
        self._lease = lease
        self._record = record

    @classmethod
    def start(cls, directory=None, *, timeout=300, capacity=2):
        directory = local_heavy_slots.host_slot_directory() if directory is None else directory
        directory = _safe_directory(directory, create=True)
        run_id = 'voiceci-' + uuid.uuid4().hex
        path = directory / ('proof-run-' + run_id + '.json')
        identity = _session_identity()
        record = {'schema': 1, 'run_id': run_id, 'slot': None, 'job_names': [],
                  'session_identity': identity}
        _durable_write(path, record)
        # acquire writes the owner into the busy marker while holding the slot
        # lock; there is no ownerless-busy/bind_owner crash window.
        try:
            lease = local_heavy_slots.acquire(local_heavy_slots.host_slot_directory(),
                                              timeout, capacity=capacity, owner_id=run_id)
        except local_heavy_slots.SlotAcquireTimeout:
            # This exception is raised only before acquire has written an
            # owner marker. Remove the exact untouched intent on normal
            # contention; preserve it for every uncertain post-acquire error.
            current = _read_record(path, run_id)
            if current['slot'] is not None or current['job_names']:
                raise ValueError('timed-out proof acquisition changed its intent')
            _durable_unlink(path)
            raise
        record['slot'] = lease.slot
        try:
            _durable_write(path, record)
        except BaseException:
            lease.abandon()
            raise
        return cls(path, run_id, lease.slot, lease, record)

    @property
    def job_names(self):
        return tuple(self._record['job_names'])

    def reserve_job_name(self, stage):
        if self._lease is None:
            raise ValueError('proof run no longer owns its slot')
        if stage not in _STAGES:
            raise ValueError('unsupported proof stage')
        name = 'Local\\VoiceCIProof-' + uuid.uuid4().hex + '-' + stage.replace('-', '_')
        if not _JOB_NAME.fullmatch(name):
            raise ValueError('generated invalid proof Job Object name')
        updated = dict(self._record)
        updated['job_names'] = list(self._record['job_names']) + [name]
        _durable_write(self.journal_path, updated)
        self._record = updated
        return name

    def close(self, *, job_exists=None):
        if self._lease is None:
            return
        probe = _default_job_exists if job_exists is None else job_exists
        # Even the normal clean path rechecks the exact named objects after
        # the broker has closed its handles. Query uncertainty keeps the lease.
        lease = self._lease
        try:
            if any(probe(name) for name in self._record['job_names']):
                raise ValueError('a journaled proof Job Object is still present')
        except BaseException:
            # Drop the process-held byte lock but retain the owner-bound busy
            # marker and journal so a later recovery can retry safely.
            lease.abandon()
            self._lease = None
            raise
        # Keep the durable journal if either operation is uncertain. The slot
        # itself is released only after the caller has verified child cleanup.
        try:
            lease.close()
        except BaseException:
            lease.abandon()
            self._lease = None
            raise
        self._lease = None
        _durable_unlink(self.journal_path)

    def abandon(self):
        if self._lease is not None:
            self._lease.abandon()
            self._lease = None


def _default_job_exists(name):
    import local_process_supervision
    return local_process_supervision.named_job_exists(name)


def recover_proof_runs(directory=None, *, capacity=2, job_exists=None):
    """Recover stale runs only after exact owner and Job absence are proven.

    Any malformed journal, active owner, present Job, or failed Job query is
    retained as unresolved. No Job is opened with write rights or terminated.
    """
    directory = local_heavy_slots.host_slot_directory() if directory is None else directory
    # An absent host scheduler directory means no journals exist yet; creating
    # it is safe and lets the first run share the same initialization path.
    directory = _safe_directory(directory, create=True)
    if type(capacity) is not int or not 1 <= capacity <= 64:
        raise ValueError('invalid proof scheduler capacity')
    job_exists = _default_job_exists if job_exists is None else job_exists
    identity = _session_identity()
    recovered = []
    for path in sorted(directory.glob('proof-run-*.json')):
        match = re.fullmatch(r'proof-run-(voiceci-[0-9a-f]{32})\.json', path.name)
        if not match:
            raise ValueError('unrecognized proof journal name')
        run_id = match.group(1)
        record = _read_record(path, run_id)
        if record['session_identity'] != identity:
            # Local\ Job Objects are session-scoped, while the slot directory
            # is host-shared. Never infer absence from a different session.
            continue
        slots = ([record['slot']] if record['slot'] is not None else list(range(capacity)))
        lease = None
        for slot in slots:
            if slot >= capacity:
                raise ValueError('journal slot exceeds configured capacity')
            try:
                lease = local_heavy_slots.recover_bound(
                    local_heavy_slots.host_slot_directory(), slot, run_id, capacity=capacity)
                break
            except (OSError, ValueError, TimeoutError):
                continue
        if lease is None:
            # Do not infer that an intent-only journal is stale: it may belong
            # to a live process between journal fsync and slot acquisition.
            continue
        try:
            # The owner may have durably reserved a new Job name after our
            # first read but before we acquired its slot lock. Re-read only
            # after exact-owner reacquisition so recovery probes the latest
            # committed inventory, never a stale prefix.
            latest = _read_record(path, run_id)
            if ((latest['slot'] is not None and latest['slot'] != lease.slot) or
                    (record['slot'] is not None and latest['slot'] != record['slot']) or
                    latest['session_identity'] != identity or
                    latest['job_names'][:len(record['job_names'])] != record['job_names']):
                lease.abandon()
                continue
            record = latest
        except BaseException:
            lease.abandon()
            raise
        try:
            if any(job_exists(name) for name in record['job_names']):
                lease.abandon()
                continue
        except BaseException:
            lease.abandon()
            raise
        try:
            lease.close()
        except BaseException:
            lease.abandon()
            raise
        _durable_unlink(path)
        recovered.append(run_id)
    return tuple(recovered)
