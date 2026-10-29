"""Offline missing-only DEV P3 issuance. No Kubernetes operations.

Inputs are public trust certificates captured and authenticated by the
operator. The caller must establish their live provenance separately: this
offline stage does not constitute an admission or create-only operation.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import pwd
import stat
import sys
import tempfile

import credentials
import reset_credentials as reset

ARCHIVE = Path('/home/voice-dev/issuance/generation1')
PARENT = Path('/home/voice-dev/issuance')


class IssueError(RuntimeError):
    pass


def _check_ancestors(path):
    for directory in (path, *path.parents):
        info = directory.lstat()
        if (not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022
                or directory.resolve() != directory):
            raise ValueError()


def _open_archive(name, private):
    fd = os.open(ARCHIVE / name, os.O_RDONLY | os.O_NOFOLLOW)
    info = os.fstat(fd)
    if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1
            or info.st_mode & (0o077 if private else 0o022)):
        os.close(fd)
        raise ValueError()
    return fd, (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def _unchanged(fd, identity):
    info = os.fstat(fd)
    return (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns) == identity


def _sync_file(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1 or info.st_mode & 0o077:
            raise ValueError()
        os.fsync(fd)
    finally:
        os.close(fd)


def _public_input(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        before = os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != pwd.getpwnam('pmd').pw_uid
                or before.st_nlink != 1 or before.st_mode & 0o077 or not 0 < before.st_size <= 65536):
            raise ValueError()
        chunks, remaining = [], before.st_size + 1
        while remaining:
            chunk = os.read(fd, min(8192, remaining))
            if not chunk: break
            chunks.append(chunk); remaining -= len(chunk)
        after = os.fstat(fd)
        if ((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) !=
                (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns)):
            raise ValueError()
        raw = b''.join(chunks)
        if len(raw) != before.st_size: raise ValueError()
        return raw
    finally:
        os.close(fd)


def _prepare(root, principal, media, space_jwks_cert):
    if any(b'PRIVATE KEY' in cert or b'BEGIN CERTIFICATE' not in cert for cert in (principal, media, space_jwks_cert)):
        raise ValueError()
    _check_ancestors(ARCHIVE)
    archive_info = ARCHIVE.lstat()
    if (not stat.S_ISDIR(archive_info.st_mode) or archive_info.st_uid != 0
            or archive_info.st_mode & 0o077 or ARCHIVE.resolve() != ARCHIVE
            ):
        raise ValueError()
    cert_fd, cert_identity = _open_archive('principal-ca.crt', False)
    try:
        key_fd, key_identity = _open_archive('principal-ca.key', True)
    except Exception:
        os.close(cert_fd)
        raise
    try:
        return _issue_with_open_archive(root, principal, media, space_jwks_cert,
                                        cert_fd, cert_identity, key_fd, key_identity)
    finally:
        os.close(key_fd)
        os.close(cert_fd)


def _issue_with_open_archive(root, principal, media, space_jwks_cert, cert_fd, cert_identity, key_fd, key_identity):
    # Refer to held file descriptors, never reopen attacker-selected pathnames.
    archived_cert = Path('/proc') / str(os.getpid()) / 'fd' / str(cert_fd)
    archived_key = Path('/proc') / str(os.getpid()) / 'fd' / str(key_fd)
    principal_input = root / 'principal-public.crt'
    media_input = root / 'media-public.crt'
    credentials.write_private(principal_input, principal)
    credentials.write_private(media_input, media)
    space_input = root / 'space-jwks-public.crt'
    credentials.write_private(space_input, space_jwks_cert)
    # Compare exact certificate identity and match the private key without export.
    if reset._openssl('x509', '-in', archived_cert, '-outform', 'DER') != reset._openssl('x509', '-in', principal_input, '-outform', 'DER'):
        raise ValueError()
    if reset._openssl('x509', '-in', archived_cert, '-pubkey', '-noout') != reset._openssl('pkey', '-in', archived_key, '-pubout'):
        raise ValueError()
    for path in (principal_input, media_input):
        constraints = reset._openssl('x509', '-in', path, '-noout', '-ext', 'basicConstraints')
        if b'CA:TRUE' not in constraints:
            raise ValueError()
    reset._openssl('verify', '-CAfile', media_input, '-purpose', 'sslserver',
                   '-verify_hostname', 'voice-space.voice-dev.svc', space_input)
    reset._openssl('x509', '-in', space_input, '-noout', '-checkend', '86400')
    material = reset.ResetMaterial(root)
    material.signer('voice-realtime-principal-signing', active=False)
    ca = (archived_key, archived_cert)
    leaves = {
        'voice-realtime-p3-tls': ('voice-realtime.voice-dev.svc', 'serverAuth,clientAuth'),
        'voice-chat-realtime-p3-tls': ('voice-chat.voice-dev.svc', 'serverAuth'),
        'voice-p3-principal-jwks-tls': ('voice-p3-principal-jwks.voice-dev.svc', 'serverAuth'),
    }
    for name, (host, purpose) in leaves.items():
        material.tls(name, ca, host, purpose)
    material.add('voice-p3-principal-replay', {'password': ''})
    material.add('voice-p3-principal-trust', {
        'chat-server-ca.crt': principal, 'space-client-ca.crt': principal,
        'space-jwks-ca.crt': media, 'realtime-client-ca.crt': principal,
        'realtime-jwks-ca.crt': principal,
    })
    signer = material.items[0]
    public = reset.public_config('voice-p3-principal-public',
        {'realtime.json': json.dumps(reset.signer_jwks(signer), separators=(',', ':')) + '\n'}, True)
    reset.verify_p3_public(signer, public)
    material.items.append(public)
    by_name = {item['metadata']['name']: item for item in material.items}
    if len(by_name) != 7:
        raise ValueError()
    for name, (host, purposes) in leaves.items():
        data = reset.values(by_name[name])
        for purpose in purposes.split(','):
            reset.verify_certificate(data['tls.crt'], data['tls.key'], principal, host, purpose)
    if not _unchanged(cert_fd, cert_identity) or not _unchanged(key_fd, key_identity):
        raise ValueError()
    bundle = {'apiVersion': 'v1', 'kind': 'List', 'items': material.items}
    credentials.write_private(root / 'objects.json', json.dumps(bundle, separators=(',', ':')) + '\n')
    return {'status': 'staged', 'namespace': 'voice-dev', 'object_names': sorted(by_name),
            'principal_ca_der_sha256': hashlib.sha256(reset._openssl('x509', '-in', principal_input, '-outform', 'DER')).hexdigest(),
            'media_ca_der_sha256': hashlib.sha256(reset._openssl('x509', '-in', media_input, '-outform', 'DER')).hexdigest(),
            'space_jwks_cert_der_sha256': hashlib.sha256(reset._openssl('x509', '-in', space_input, '-outform', 'DER')).hexdigest(),
            'runtime_qualified': False, 'cluster_applied': False}


def stage(destination, principal, media, space_jwks_cert):
    """Create a private packet once; failure leaves it for operator recovery."""
    try:
        destination = Path(destination)
        parent = destination.parent
        if parent != PARENT or not re.fullmatch(r'p3-[a-f0-9]{32}', destination.name):
            raise ValueError()
        _check_ancestors(parent)
        info = parent.stat()
        if (sys.platform != 'linux' or os.geteuid() != 0 or not destination.is_absolute()
                or parent.resolve() != parent or not stat.S_ISDIR(info.st_mode)
                or info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o700):
            raise ValueError()
        if destination.exists() or destination.is_symlink():
            raise ValueError()
        # One durable reservation across all operation IDs; uncertain attempts
        # require explicit operator reconciliation, never automatic reissuance.
        reservation = parent / 'p3-issuance.once'
        credentials.write_private(reservation, destination.name + '\n')
        _sync_file(reservation)
        parent_fd = os.open(parent, os.O_RDONLY | os.O_DIRECTORY)
        try: os.fsync(parent_fd)
        finally: os.close(parent_fd)
        destination.mkdir(mode=0o700)
        credentials.write_private(destination / 'pending', 'Started; completion requires the complete marker.\n')
        previous = os.umask(0o077)
        old_tempdir = tempfile.tempdir
        tempfile.tempdir = str(destination)
        try:
            receipt = _prepare(destination, principal, media, space_jwks_cert)
            credentials.write_private(destination / 'receipt.json', json.dumps(receipt) + '\n')
            for path in destination.iterdir():
                _sync_file(path)
            dirfd = os.open(destination, os.O_RDONLY | os.O_DIRECTORY)
            try: os.fsync(dirfd)
            finally: os.close(dirfd)
            credentials.write_private(destination / 'complete', 'Packet complete; runtime and cluster are not qualified.\n')
            _sync_file(destination / 'complete')
            dirfd = os.open(destination, os.O_RDONLY | os.O_DIRECTORY)
            try: os.fsync(dirfd)
            finally: os.close(dirfd)
            parent_fd = os.open(parent, os.O_RDONLY | os.O_DIRECTORY)
            try: os.fsync(parent_fd)
            finally: os.close(parent_fd)
        finally:
            tempfile.tempdir = old_tempdir
            os.umask(previous)
        return receipt
    except Exception:
        raise IssueError('issuance refused or incomplete; preserve operation directory') from None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--operation-id', required=True)
    parser.add_argument('--principal-public', required=True, type=Path)
    parser.add_argument('--media-public', required=True, type=Path)
    parser.add_argument('--space-jwks-cert', required=True, type=Path)
    args = parser.parse_args()
    if not re.fullmatch('[a-f0-9]{32}', args.operation_id):
        raise IssueError('invalid operation id')
    destination = PARENT / ('p3-' + args.operation_id)
    result = stage(destination, _public_input(args.principal_public), _public_input(args.media_public),
                   _public_input(args.space_jwks_cert))
    print(json.dumps(result, sort_keys=True))


if __name__ == '__main__':
    try:
        main()
    except Exception:
        print('issuance refused or incomplete; preserve operation directory', file=sys.stderr)
        sys.exit(1)
