"""Private local journal primitive, not a cluster lock or rollout authority.

An executor must additionally hold the common Kubernetes lock, verify its TT
claim and active permanent CAS ledger before every effect, and reconcile any
uncertain save or remote response without retrying from absence alone. All
ancestor directories must be trusted against rename or replacement by other
users; pathname symlink checks alone do not enforce this boundary.
"""
import copy
import fcntl
import json
import os
from pathlib import Path
import secrets
import stat


class RecoveryRequired(RuntimeError):
    pass


class Journal:
    def __init__(self, directory, intent):
        self.path = Path(directory)
        self.dirfd = self.lockfd = None
        self.poisoned = False
        self.intent = copy.deepcopy(intent)
        try:
            if not isinstance(intent, dict) or not intent:
                raise RecoveryRequired('immutable operation intent required')
            encoded = json.dumps(intent, sort_keys=True, separators=(',', ':')).encode()
            if len(encoded) > 262144:
                raise RecoveryRequired('operation intent too large')
            if self.path.is_symlink() or any(parent.is_symlink() for parent in self.path.parents):
                raise RecoveryRequired('journal path contains a symlink')
            try:
                self.path.mkdir(mode=0o700)
                new_directory = True
            except FileExistsError:
                new_directory = False
            parentfd = os.open(self.path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            try:
                os.fsync(parentfd)
            finally:
                os.close(parentfd)
            self.dirfd = os.open(self.path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            self._private(os.fstat(self.dirfd), 0o700, directory=True)
            self.lockfd = os.open('lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW | os.O_NONBLOCK, 0o600, dir_fd=self.dirfd)
            self._private(os.fstat(self.lockfd), 0o600)
            fcntl.flock(self.lockfd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            inventory = set(os.listdir(self.dirfd))
            expected_inventory = {'lock'} if new_directory else {'lock', 'state.json'}
            if inventory != expected_inventory:
                raise RecoveryRequired('journal inventory requires reconciliation')
            os.fsync(self.dirfd)
            try:
                fd = os.open('state.json', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=self.dirfd)
            except FileNotFoundError:
                self.state = {'format': 1, 'intent': copy.deepcopy(intent), 'steps': {}}
                self.save()
            else:
                with os.fdopen(fd, 'rb') as stream:
                    self._private(os.fstat(stream.fileno()), 0o600)
                    raw = stream.read(1048577)
                if len(raw) > 1048576:
                    raise RecoveryRequired('journal is too large')
                self.state = json.loads(raw)
                self._validate()
        except BaseException:
            self.close()
            raise RecoveryRequired('private journal unavailable or requires reconciliation') from None

    @staticmethod
    def _private(info, mode, directory=False):
        correct = stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)
        if (not correct or stat.S_IMODE(info.st_mode) != mode or info.st_uid != os.geteuid() or
                (not directory and info.st_nlink != 1)):
            raise RecoveryRequired('journal custody invalid')

    def _validate(self):
        if (not isinstance(self.state, dict) or set(self.state) != {'format', 'intent', 'steps'} or
                self.state['format'] != 1 or self.state['intent'] != self.intent or
                not isinstance(self.state['steps'], dict)):
            raise RecoveryRequired('journal intent or schema changed')

    def save(self):
        if self.poisoned or self.dirfd is None or self.lockfd is None:
            raise RecoveryRequired('journal closed or save outcome uncertain')
        temporary = '.state-' + secrets.token_hex(16)
        created = False
        try:
            self._validate()
            raw = json.dumps(self.state, sort_keys=True, separators=(',', ':')).encode()
            if len(raw) > 1048576:
                raise RecoveryRequired('journal is too large')
            fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                         0o600, dir_fd=self.dirfd)
            created = True
            with os.fdopen(fd, 'wb') as stream:
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, 'state.json', src_dir_fd=self.dirfd, dst_dir_fd=self.dirfd)
            os.fsync(self.dirfd)
        except BaseException:
            self.poisoned = True
            raise RecoveryRequired('journal save outcome uncertain; reconcile fence') from None
        finally:
            if created:
                try:
                    os.unlink(temporary, dir_fd=self.dirfd)
                except FileNotFoundError:
                    pass
                except OSError:
                    self.poisoned = True
                    raise RecoveryRequired('journal cleanup uncertain; reconcile fence') from None

    def close(self):
        for attr in ('lockfd', 'dirfd'):
            fd = getattr(self, attr, None)
            if fd is not None:
                os.close(fd)
                setattr(self, attr, None)

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()
