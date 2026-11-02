"""Create a private staging-fixture directory below a caller-owned root.

Caller must retain exclusive custody of the root until cleanup. This helper
does not authenticate a Docker daemon, bind mount or source checkout.
"""
import os
from pathlib import Path
import stat
import sys
import uuid


def create(root):
    if (os.name != 'posix' or not hasattr(os, 'geteuid') or os.geteuid() == 0 or
            not isinstance(root, str) or not root.startswith('/') or root == '/' or
            os.path.normpath(root) != root or os.path.realpath(root) != root or
            not hasattr(os, 'O_NOFOLLOW') or not hasattr(os, 'O_DIRECTORY')):
        raise ValueError('a canonical non-root owned directory is required')
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
    try:
        descriptor = os.open(root, flags)
    except OSError as error:
        raise ValueError('owned temporary root is unavailable') from error
    name = None
    created = False
    try:
        opened = os.fstat(descriptor)
        current = os.stat(root, follow_symlinks=False)
        if (not stat.S_ISDIR(opened.st_mode) or opened.st_uid != os.geteuid() or
                stat.S_IMODE(opened.st_mode) != 0o700 or
                (opened.st_dev, opened.st_ino) != (current.st_dev, current.st_ino)):
            raise ValueError('owned temporary root identity or permissions differ')
        name = 'voice-staging-pg-ci.' + uuid.uuid4().hex
        os.mkdir(name, 0o700, dir_fd=descriptor)
        created = True
        child = os.open(name, flags, dir_fd=descriptor)
        try:
            os.fchmod(child, 0o700)
            item = os.fstat(child)
            current = os.stat(root, follow_symlinks=False)
            if (item.st_uid != os.geteuid() or stat.S_IMODE(item.st_mode) != 0o700 or
                    (opened.st_dev, opened.st_ino) != (current.st_dev, current.st_ino)):
                raise ValueError('owned temporary fixture identity changed')
        finally:
            os.close(child)
        return Path(root) / name
    except BaseException as error:
        if created:
            try:
                os.rmdir(name, dir_fd=descriptor)
            except OSError as cleanup:
                error.add_note('Failed to remove the newly created fixture directory: ' + str(cleanup))
        raise
    finally:
        os.close(descriptor)


if __name__ == '__main__':
    if len(sys.argv) != 2:
        raise SystemExit('expected one owned temporary root')
    print(create(sys.argv[1]))
