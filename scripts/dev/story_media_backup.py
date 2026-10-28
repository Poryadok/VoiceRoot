"""Private encrypted media-database backup primitives.

This module is not an admission or restore qualification. Its caller must hold
and revalidate the operation fence and establish independent off-node custody.
"""
import hashlib
import hmac
import json
import os
import shutil
import stat
import subprocess
import tempfile
from contextlib import contextmanager

DOMAIN = b'voice-story-media-backup-manifest-v1\0'
MEMBERS = frozenset(('file_db', 'story_db', 'messaging_db', 'roles'))


class BackupError(Exception):
    pass


@contextmanager
def private_directory(root):
    """Walk without symlinks, then anchor a private owner-only directory."""
    parts = os.path.abspath(os.fspath(root)).split(os.sep)[1:]
    fd = None
    try:
        if not parts or not all(parts):
            raise BackupError('backup directory custody invalid')
        fd = os.open(os.sep, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        for index, part in enumerate(parts):
            next_fd = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
            os.close(fd)
            fd = next_fd
            info = os.fstat(fd)
            final = index == len(parts) - 1
            if final:
                if info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != 0o700:
                    raise BackupError('backup directory custody invalid')
            elif (info.st_uid not in (0, os.geteuid()) or
                  (info.st_mode & 0o022 and not (info.st_uid == 0 and info.st_mode & stat.S_ISVTX))):
                raise BackupError('backup directory custody invalid')
        yield fd
    except OSError:
        raise BackupError('backup directory custody invalid') from None
    finally:
        if fd is not None:
            os.close(fd)


def export(root, key, binding, producer):
    """Encrypt four producer streams; producer must return only after checked exit.

    The caller owns source identity, deadlines and continuous writer fencing.
    Failure leaves a partial private set, never a completed manifest.
    """
    if len(key) != 32 or not callable(producer) or not isinstance(binding, dict):
        raise BackupError('invalid backup input')
    with private_directory(root) as dirfd:
        return _export(dirfd, key, binding, producer)


def _export(dirfd, key, binding, producer):
    files = {}
    inodes = {}
    for name in sorted(MEMBERS):
        filename = name + '.enc'
        fd = os.open(filename, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=dirfd)
        read_fd, write_fd = os.pipe()
        try:
            os.write(write_fd, key.hex().encode() + b'\n')
            os.close(write_fd)
            write_fd = -1
            with os.fdopen(fd, 'w+b') as output:
                process = subprocess.Popen(
                    ['openssl', 'enc', '-aes-256-cbc', '-pbkdf2', '-iter', '200000',
                     '-salt', '-pass', 'file:/proc/self/fd/' + str(read_fd)],
                    stdin=subprocess.PIPE, stdout=output, stderr=subprocess.DEVNULL,
                    pass_fds=(read_fd,))
                try:
                    producer(name, process.stdin)
                    process.stdin.close()
                    if process.wait(timeout=20) != 0:
                        raise BackupError('backup encryption failed')
                    output.flush()
                    os.fsync(output.fileno())
                    info = os.fstat(output.fileno())
                    current = os.stat(filename, dir_fd=dirfd, follow_symlinks=False)
                    if (info.st_dev, info.st_ino) != (current.st_dev, current.st_ino):
                        raise BackupError('backup member identity changed')
                    output.seek(0)
                    files[name] = stream_identity(output, key)
                    current = os.stat(filename, dir_fd=dirfd, follow_symlinks=False)
                    if (info.st_dev, info.st_ino) != (current.st_dev, current.st_ino):
                        raise BackupError('backup member identity changed')
                    inodes[name] = (info.st_dev, info.st_ino)
                finally:
                    if process.poll() is None:
                        process.kill()
                        process.wait()
                    if not process.stdin.closed:
                        try:
                            process.stdin.close()
                        except OSError:
                            pass
        except Exception:
            raise BackupError('backup encryption failed') from None
        finally:
            os.close(read_fd)
            if write_fd >= 0:
                os.close(write_fd)
    for name in sorted(MEMBERS):
        info = os.stat(name + '.enc', dir_fd=dirfd, follow_symlinks=False)
        if ((info.st_dev, info.st_ino) != inodes[name]
                or identity_at(dirfd, name + '.enc', key) != files[name]):
            raise BackupError('backup member identity changed')
    manifest = {'format': 1, 'binding': binding, 'files': files}
    manifest['mac'] = manifest_mac(key, manifest)
    return manifest


def verify(root, key, binding, manifest):
    with private_directory(root) as dirfd:
        return _verify(dirfd, key, binding, manifest)


def _verify(dirfd, key, binding, manifest):
    if len(key) != 32 or not isinstance(manifest, dict):
        raise BackupError('backup integrity failed')
    if set(manifest) != {'format', 'binding', 'files', 'mac'}:
        raise BackupError('backup integrity failed')
    unsigned = {name: manifest[name] for name in ('format', 'binding', 'files')}
    if (manifest['format'] != 1 or manifest['binding'] != binding
            or not isinstance(manifest['mac'], str)
            or not hmac.compare_digest(manifest_mac(key, unsigned), manifest['mac'])):
        raise BackupError('backup integrity failed')
    files = manifest['files']
    if not isinstance(files, dict) or set(files) != MEMBERS:
        raise BackupError('backup integrity failed')
    for name, expected in files.items():
        if identity_at(dirfd, name + '.enc', key) != expected:
            raise BackupError('backup integrity failed')
    return set(files)


def manifest_mac(key, manifest):
    encoded = json.dumps(manifest, sort_keys=True, separators=(',', ':'), ensure_ascii=True,
                         allow_nan=False).encode('ascii')
    return hmac.new(key, DOMAIN + encoded, hashlib.sha256).hexdigest()


def identity(path, key):
    with private_directory(path.parent) as dirfd:
        return identity_at(dirfd, path.name, key)


def stream_identity(stream, key):
    sha = hashlib.sha256()
    mac = hmac.new(key, digestmod=hashlib.sha256)
    info = os.fstat(stream.fileno())
    if (not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077
            or info.st_uid != os.geteuid() or info.st_nlink != 1):
        raise BackupError('backup member custody invalid')
    for chunk in iter(lambda: stream.read(1024 * 1024), b''):
        sha.update(chunk)
        mac.update(chunk)
    return {'sha256': sha.hexdigest(), 'hmac': mac.hexdigest()}


def identity_at(dirfd, filename, key):
    try:
        fd = os.open(filename, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=dirfd)
        with os.fdopen(fd, 'rb') as stream:
            return stream_identity(stream, key)
    except OSError:
        raise BackupError('backup member unavailable') from None


def decrypt(root, key, binding, manifest, member, destination):
    """Authenticate a private ciphertext snapshot before sending any plaintext.

    A failed decrypt may have written a prefix; callers must discard the entire
    isolated restore target when any stage fails.
    """
    with private_directory(root) as dirfd:
        _verify(dirfd, key, binding, manifest)
        if member not in manifest['files']:
            raise BackupError('backup member missing')
        try:
            source_fd = os.open(member + '.enc', os.O_RDONLY | os.O_NOFOLLOW, dir_fd=dirfd)
        except OSError:
            raise BackupError('backup member unavailable') from None
    _decrypt_snapshot(source_fd, key, manifest['files'][member], destination)


def _decrypt_snapshot(source_fd, key, expected, destination):
    with os.fdopen(source_fd, 'rb') as source, tempfile.TemporaryFile() as snapshot:
        info = os.fstat(source.fileno())
        if (not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077
                or info.st_uid != os.geteuid() or info.st_nlink != 1):
            raise BackupError('backup member custody invalid')
        shutil.copyfileobj(source, snapshot, length=1024 * 1024)
        snapshot.seek(0)
        sha = hashlib.sha256()
        mac = hmac.new(key, digestmod=hashlib.sha256)
        for chunk in iter(lambda: snapshot.read(1024 * 1024), b''):
            sha.update(chunk)
            mac.update(chunk)
        actual = {'sha256': sha.hexdigest(), 'hmac': mac.hexdigest()}
        if actual != expected:
            raise BackupError('backup integrity failed')
        snapshot.seek(0)
        read_fd, write_fd = os.pipe()
        try:
            os.write(write_fd, key.hex().encode() + b'\n')
            os.close(write_fd)
            write_fd = -1
            process = subprocess.Popen(
                ['openssl', 'enc', '-d', '-aes-256-cbc', '-pbkdf2', '-iter', '200000',
                 '-pass', 'file:/proc/self/fd/' + str(read_fd)],
                stdin=snapshot, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                pass_fds=(read_fd,))
            try:
                shutil.copyfileobj(process.stdout, destination, length=1024 * 1024)
                process.stdout.close()
                if process.wait() != 0:
                    raise BackupError('backup decryption failed')
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
        except (OSError, subprocess.SubprocessError) as error:
            raise BackupError('backup decryption failed') from error
        finally:
            os.close(read_fd)
            if write_fd >= 0:
                os.close(write_fd)
