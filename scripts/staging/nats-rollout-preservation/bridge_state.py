"""Read-only projection of runner-custodied operation checkpoints."""
from __future__ import annotations

import json
import os
from pathlib import Path
import re
import stat


MAX_CHECKPOINT = 128 << 20


class StateError(ValueError):
    pass


def _fail(code):
    raise StateError('runner_state_' + code)


def _pairs(items):
    row = {}
    for key, value in items:
        if key in row:
            _fail('duplicate_field')
        row[key] = value
    return row


def _read_checkpoint(path: Path, operation: str, *, owner_uid: int) -> dict:
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    except OSError:
        _fail('checkpoint_unavailable')
    try:
        before = os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != owner_uid
            or stat.S_IMODE(before.st_mode) != 0o600 or before.st_nlink != 1
            or not 1 <= before.st_size <= MAX_CHECKPOINT):
            _fail('checkpoint_custody_invalid')
        raw = bytearray()
        while len(raw) <= before.st_size:
            chunk = os.read(fd, min(1 << 20, before.st_size + 1 - len(raw)))
            if not chunk:
                break
            raw.extend(chunk)
        after = os.fstat(fd)
        if (len(raw) != before.st_size or (before.st_dev, before.st_ino, before.st_size,
            before.st_mtime_ns, before.st_ctime_ns) != (after.st_dev, after.st_ino,
            after.st_size, after.st_mtime_ns, after.st_ctime_ns)):
            _fail('checkpoint_changed')
    finally:
        os.close(fd)
    try:
        state = json.loads(raw, object_pairs_hook=_pairs)
    except StateError:
        raise
    except (ValueError, TypeError, UnicodeError):
        _fail('checkpoint_invalid')
    if not isinstance(state, dict) or state.get('operation') != operation:
        _fail('operation_custody_invalid')
    return state


class BridgeState:
    """Only reads runner-owned, bootstrap-copied checkpoint data; never writes it."""
    def __init__(self, operations_dir: str | Path, *, owner_uid: int | None = None):
        self.operations_dir = Path(operations_dir)
        self.owner_uid = os.geteuid() if owner_uid is None else owner_uid
        try:
            directory = self.operations_dir.lstat()
        except OSError:
            _fail('operations_directory_unavailable')
        if (not stat.S_ISDIR(directory.st_mode) or stat.S_ISLNK(directory.st_mode)
            or directory.st_uid != self.owner_uid or stat.S_IMODE(directory.st_mode) != 0o700):
            _fail('operations_directory_custody_invalid')
        try:
            parent = self.operations_dir.parent.lstat()
        except OSError:
            _fail('state_directory_unavailable')
        if (not stat.S_ISDIR(parent.st_mode) or stat.S_ISLNK(parent.st_mode)
            or parent.st_uid != self.owner_uid or stat.S_IMODE(parent.st_mode) != 0o700):
            _fail('state_directory_custody_invalid')

    def status(self, operation: str) -> dict:
        if not isinstance(operation, str) or not re.fullmatch('[a-f0-9]{12}', operation):
            _fail('operation_invalid')
        base = self.operations_dir / ('rollout-' + operation)
        try:
            directory = base.lstat()
        except OSError:
            _fail('operation_unavailable')
        if (not stat.S_ISDIR(directory.st_mode) or stat.S_ISLNK(directory.st_mode)
            or directory.st_uid != self.owner_uid or stat.S_IMODE(directory.st_mode) != 0o700):
            _fail('operation_directory_custody_invalid')
        checkpoint = _read_checkpoint(base / 'checkpoint.json', operation, owner_uid=self.owner_uid)
        result = {'operation': operation}
        for key in ('phase', 'status', 'fence_status'):
            value = checkpoint.get(key)
            if value is not None:
                if not isinstance(value, str) or not re.fullmatch('[A-Z][A-Z0-9_]{0,63}', value):
                    _fail('summary_field_invalid')
                result[key] = value
        error = checkpoint.get('error')
        if error is not None:
            if not isinstance(error, str):
                _fail('summary_error_invalid')
            result['error'] = error if re.fullmatch('[a-z][a-z0-9_]{0,63}', error) else 'redacted'
        return result
