"""Bounded fixed unit-file custody/readback before installer activation."""
import os
import stat
from controller import Blocked

KEYS = ('st_dev', 'st_ino', 'st_uid', 'st_gid', 'st_mode', 'st_nlink',
        'st_size', 'st_mtime_ns', 'st_ctime_ns')


def fail(): raise Blocked('hub_bridge_unit_readback_unbound')


def verify(suffix, expected):
    if (os.geteuid() != 0 or suffix not in ('service', 'path', 'timer')
            or type(expected) is not str or not 0 < len(expected.encode('utf-8')) <= 16384): fail()
    wanted = expected.encode('utf-8')
    parent = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC)
    try:
        for part in ('etc', 'systemd', 'system'):
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent)
            row = os.fstat(child)
            if row.st_uid != 0 or row.st_gid != 0 or row.st_mode & 0o022:
                os.close(child); fail()
            os.close(parent); parent = child
        name = 'voice-nats-preservation.' + suffix
        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=parent)
        try:
            before = os.fstat(fd)
            if (not stat.S_ISREG(before.st_mode) or before.st_uid != 0 or before.st_gid != 0
                    or before.st_nlink != 1 or stat.S_IMODE(before.st_mode) != 0o644
                    or before.st_size != len(wanted)): fail()
            raw = os.read(fd, len(wanted) + 1)
            after = os.fstat(fd); path = os.stat(name, dir_fd=parent, follow_symlinks=False)
            if raw != wanted or any(getattr(before, key) != getattr(after, key)
                                   or getattr(before, key) != getattr(path, key) for key in KEYS): fail()
        finally: os.close(fd)
    finally: os.close(parent)
