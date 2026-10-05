"""Authenticated OpenSSL CMS backup; recovery keys never leave root custody."""
import hashlib
import os
from pathlib import Path
import stat
import subprocess
import tarfile
import tempfile


class CryptoError(RuntimeError):
    pass


def directory(path):
    path = Path(path)
    for parent in (*path.parents[::-1], path):
        row = parent.lstat()
        if not stat.S_ISDIR(row.st_mode) or row.st_uid != 0 or row.st_mode & 0o022:
            raise CryptoError('backup_directory_custody_invalid')
    return path


def regular(path, private=False):
    path = Path(path)
    directory(path.parent)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    row = os.fstat(fd)
    if (not stat.S_ISREG(row.st_mode) or row.st_uid != 0 or row.st_nlink != 1 or
            row.st_mode & (0o077 if private else 0o022)):
        os.close(fd)
        raise CryptoError('backup_file_custody_invalid')
    return fd, row


def openssl(args):
    try:
        subprocess.run(['/usr/bin/openssl', *args], check=True, timeout=600,
            stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            env={'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C'})
    except (OSError, subprocess.SubprocessError):
        raise CryptoError('backup_openssl_failed') from None


def initialize_recovery_key(key_directory):
    key_directory = directory(key_directory)
    private = key_directory / 'recovery-key.pem'
    certificate = key_directory / 'recovery-cert.pem'
    if private.exists() or certificate.exists():
        for path in (private, certificate):
            fd, _ = regular(path, private=True)
            os.close(fd)
        openssl(['pkey', '-in', str(private), '-check', '-noout'])
        openssl(['x509', '-in', str(certificate), '-noout'])
        scratch=Path(tempfile.mkdtemp(prefix='key-check-',dir=key_directory))
        try:
            openssl(['pkey','-in',str(private),'-pubout','-out',str(scratch/'private.pub')])
            openssl(['x509','-in',str(certificate),'-pubkey','-noout','-out',str(scratch/'certificate.pub')])
            if (scratch/'private.pub').read_bytes()!=(scratch/'certificate.pub').read_bytes():raise CryptoError('backup_recovery_key_certificate_mismatch')
        finally:
            for path in scratch.iterdir():path.unlink()
            scratch.rmdir()
        return
    # Unique private workspace: a failed generation never becomes an installed
    # recovery key. Existing keys are checked and retained, never replaced.
    scratch = Path(tempfile.mkdtemp(prefix='key-', dir=key_directory))
    saved_umask = os.umask(0o077)
    try:
        openssl(['req', '-x509', '-newkey', 'rsa:3072', '-nodes', '-sha256',
            '-days', '36500', '-subj', '/CN=Voice rollout backup recovery',
            '-keyout', str(scratch / 'key.pem'), '-out', str(scratch / 'cert.pem')])
        for source, target in ((scratch / 'key.pem', private), (scratch / 'cert.pem', certificate)):
            os.link(source, target)
            target.chmod(0o600)
        for path in (private, certificate):
            with path.open('rb') as stream:
                os.fsync(stream.fileno())
        fd = os.open(key_directory, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    finally:
        os.umask(saved_umask)
        for path in scratch.iterdir():
            path.unlink()
        scratch.rmdir()


def encrypt_cut(base, key_directory):
    base, key_directory = directory(base), directory(key_directory)
    for path in (key_directory / 'recovery-key.pem', key_directory / 'recovery-cert.pem'):
        fd, _ = regular(path, private=True)
        os.close(fd)
    target = base / 'rollout-backup.cms'
    # O_EXCL reservation binds this attempt to one immutable ciphertext.
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    os.close(fd)
    scratch = Path(tempfile.mkdtemp(prefix='encrypt-', dir=base))
    try:
        payload = scratch / 'payload.tar'
        with tarfile.open(payload, 'w') as bundle:
            payload.chmod(0o600)
            for name in ('rollout-before.tar', 'rollout-before-manifest.json', 'copy-checkpoint.json'):
                fd, row = regular(base / name)
                with os.fdopen(fd, 'rb') as stream:
                    member = tarfile.TarInfo(name)
                    member.size = row.st_size
                    member.mode = 0o600
                    bundle.addfile(member, stream)
        ciphertext = scratch / 'cipher.cms'
        openssl(['cms', '-encrypt', '-binary', '-stream', '-aes-256-gcm',
            '-outform', 'DER', '-in', str(payload), '-out', str(ciphertext),
            '-recip', str(key_directory / 'recovery-cert.pem')])
        ciphertext.chmod(0o600)
        with ciphertext.open('rb') as source, target.open('wb') as output:
            import shutil
            shutil.copyfileobj(source, output, length=1 << 20)
            output.flush()
            os.fsync(output.fileno())
        sha = hashlib.sha256()
        with target.open('rb') as stream:
            while chunk := stream.read(1 << 20):
                sha.update(chunk)
        return {'cipher_sha256': sha.hexdigest(), 'cipher_bytes': target.stat().st_size}
    except BaseException:
        target.unlink()
        raise
    finally:
        for path in scratch.iterdir():
            path.unlink()
        scratch.rmdir()
