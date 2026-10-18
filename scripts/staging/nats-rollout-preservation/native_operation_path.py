"""Fixed rootless operation-directory adapter for the installed native runner.

The tree is provisioned by the future human bootstrap package. This module
never creates or repairs the installation anchor and never accepts a path from
an operation request.
"""
import os
from pathlib import Path
import re
import stat

from controller import Blocked


UID = 65532
GID = 65532
ANCHOR = Path('/var/lib/voice-staging-runner/operations')
PARENT = Path('/var/lib/voice-staging-runner')
_DIRECTORY = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC


def _blocked():
    raise Blocked('rollout_operation_path_changed')


def _open_chain():
    if os.geteuid() != UID or os.getegid() != GID:
        _blocked()
    fd = os.open('/', _DIRECTORY)
    current = Path('/')
    try:
        for part in ANCHOR.parts[1:]:
            child = os.open(part, _DIRECTORY, dir_fd=fd)
            row = os.fstat(child)
            current /= part
            mode = stat.S_IMODE(row.st_mode)
            if not stat.S_ISDIR(row.st_mode):
                os.close(child)
                _blocked()
            if current == ANCHOR:
                if row.st_uid != UID or row.st_gid != GID or mode != 0o700:
                    os.close(child)
                    _blocked()
            elif current == PARENT:
                if row.st_uid != 0 or row.st_gid != 0 or mode != 0o755:
                    os.close(child)
                    _blocked()
            elif row.st_uid != 0 or mode & 0o022:
                os.close(child)
                _blocked()
            os.close(fd)
            fd = child
        return fd
    except BaseException:
        os.close(fd)
        raise


def _operation_name(operation):
    if not isinstance(operation, str) or re.fullmatch(r'[a-f0-9]{12}', operation) is None:
        _blocked()
    return 'rollout-' + operation


def create_operation(operation):
    """Create one new private operation directory; existing state is never adopted."""
    name = _operation_name(operation)
    anchor_fd = _open_chain()
    try:
        os.mkdir(name, mode=0o700, dir_fd=anchor_fd)
        child = os.open(name, _DIRECTORY, dir_fd=anchor_fd)
        try:
            row = os.fstat(child)
            if (row.st_uid != UID or row.st_gid != GID or stat.S_IMODE(row.st_mode) != 0o700
                or not stat.S_ISDIR(row.st_mode)):
                _blocked()
            os.fsync(anchor_fd)
        finally:
            os.close(child)
    except FileExistsError:
        _blocked()
    finally:
        os.close(anchor_fd)
    return ANCHOR / name


def existing_operation(operation):
    """Open and validate an existing private operation without modifying it."""
    name = _operation_name(operation)
    anchor_fd = _open_chain()
    try:
        child = os.open(name, _DIRECTORY, dir_fd=anchor_fd)
        try:
            row = os.fstat(child)
            if (row.st_uid != UID or row.st_gid != GID or stat.S_IMODE(row.st_mode) != 0o700
                or not stat.S_ISDIR(row.st_mode)):
                _blocked()
        finally:
            os.close(child)
    except OSError:
        _blocked()
    finally:
        os.close(anchor_fd)
    return ANCHOR / name


def operation_root():
    """Validate and return the installed root without creating installation state."""
    fd = _open_chain()
    os.close(fd)
    return ANCHOR


def list_operations():
    """Return every exact private operation directory; unknown entries block without cleanup."""
    fd = _open_chain()
    try:
        names = os.listdir(fd)
    finally:
        os.close(fd)
    if any(re.fullmatch(r'rollout-[a-f0-9]{12}', name) is None for name in names):
        _blocked()
    return [existing_operation(name[len('rollout-'):]) for name in sorted(names)]


def validate_operation(path, operation):
    """Validate an already selected native path against the fixed anchor and operation."""
    expected = existing_operation(operation)
    if Path(path) != expected:
        _blocked()
    return expected


def owner_uid_for_path(path):
    """Return the owner for an already validated artifact path, preserving legacy ROOT."""
    candidate = Path(path)
    for parent in (candidate.parent, *candidate.parents):
        if parent.parent == ANCHOR and re.fullmatch(r'rollout-[a-f0-9]{12}', parent.name):
            existing_operation(parent.name[len('rollout-'):])
            return UID
    return 0
