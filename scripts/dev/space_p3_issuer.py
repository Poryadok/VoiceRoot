"""Root-only offline issuance of the two missing DEV Space identities.

The caller must independently bind the public CA to the live fenced capture.
No Kubernetes access, root key export, CA rotation or cluster authority here.
"""
import hashlib
import argparse
import json
import os
from pathlib import Path
import re
import stat
import sys
import tempfile

import credentials
import realtime_p3_missing_issuer as existing
import reset_credentials as reset

ARCHIVE = Path('/home/voice-dev/issuance/generation1')
PARENT = Path('/home/voice-dev/issuance')
_check_ancestors = existing._check_ancestors


class IssueError(RuntimeError): pass


def _open(name, private):
    fd = os.open(ARCHIVE / name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    info = os.fstat(fd)
    if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1
        or info.st_mode & (0o077 if private else 0o022) or not 0 < info.st_size <= 65536):
        os.close(fd); raise ValueError()
    return fd, (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def _prepare(destination, media):
    if (not isinstance(media, bytes) or not re.fullmatch(
        rb'-----BEGIN CERTIFICATE-----\r?\n(?:[A-Za-z0-9+/=]+\r?\n)+-----END CERTIFICATE-----\r?\n?', media)): raise ValueError()
    _check_ancestors(ARCHIVE)
    info = ARCHIVE.lstat()
    if (not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o077 or ARCHIVE.resolve() != ARCHIVE): raise ValueError()
    cert_fd, cert_id = _open('media-ca.crt', False)
    try:
        key_fd, key_id = _open('media-ca.key', True)
        try:
            cert = Path('/proc') / str(os.getpid()) / 'fd' / str(cert_fd)
            key = Path('/proc') / str(os.getpid()) / 'fd' / str(key_fd)
            public = destination / 'media-public.crt'; credentials.write_private(public, media)
            cert_der = reset._openssl('x509','-in',cert,'-outform','DER')
            if (cert_der != reset._openssl('x509','-in',public,'-outform','DER')
                or reset._openssl('x509','-in',cert,'-pubkey','-noout') != reset._openssl('pkey','-in',key,'-pubout')
                or b'CA:TRUE' not in reset._openssl('x509','-in',cert,'-noout','-ext','basicConstraints')): raise ValueError()
            material = reset.ResetMaterial(destination)
            material.signer('voice-space-principal-signing', active=True)
            material.tls('voice-space-media-server-tls', (key,cert), 'voice-space.voice-dev.svc', 'serverAuth')
            by_name = {obj['metadata']['name']:obj for obj in material.items}
            if set(by_name) != {'voice-space-principal-signing','voice-space-media-server-tls'}: raise ValueError()
            signer = by_name['voice-space-principal-signing']; values = reset.values(signer)
            if set(values) != {'current.pem','next.pem','active-kid'} or values['active-kid'] != b'current': raise ValueError()
            jwks = reset.signer_jwks(signer)
            leaf = reset.values(by_name['voice-space-media-server-tls'])
            reset.verify_certificate(leaf['tls.crt'],leaf['tls.key'],media,'voice-space.voice-dev.svc','serverAuth')
            if not existing._unchanged(cert_fd,cert_id) or not existing._unchanged(key_fd,key_id): raise ValueError()
            credentials.write_private(destination/'objects.json',json.dumps({'apiVersion':'v1','kind':'List','items':material.items},separators=(',',':'))+'\n')
            return {'status':'staged','namespace':'voice-dev','object_names':sorted(by_name),
                    'media_ca_der_sha256':hashlib.sha256(cert_der).hexdigest(), 'public_jwks':jwks,
                    'runtime_qualified':False,'cluster_applied':False}
        finally: os.close(key_fd)
    finally: os.close(cert_fd)


def stage(destination, media):
    """Durably reserve once; a failed attempt always requires reconciliation."""
    try:
        destination = Path(destination); parent = destination.parent
        if parent != PARENT or not re.fullmatch(r'space-p3-[a-f0-9]{32}',destination.name): raise ValueError()
        _check_ancestors(parent); info = parent.stat()
        if (sys.platform != 'linux' or os.geteuid() != 0 or not destination.is_absolute() or parent.resolve() != parent
            or not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o700
            or destination.exists() or destination.is_symlink()): raise ValueError()
        reservation = parent/'space-p3-issuance.once'
        credentials.write_private(reservation,destination.name+'\n'); existing._sync_file(reservation)
        fd = os.open(parent,os.O_RDONLY|os.O_DIRECTORY)
        try: os.fsync(fd)
        finally: os.close(fd)
        destination.mkdir(mode=0o700); credentials.write_private(destination/'pending','Started; preserve for reconciliation.\n')
        old_umask = os.umask(0o077); old_tempdir = tempfile.tempdir; tempfile.tempdir = str(destination)
        try:
            receipt = _prepare(destination,media)
            credentials.write_private(destination/'receipt.json',json.dumps(receipt,separators=(',',':'))+'\n')
            for path in destination.iterdir(): existing._sync_file(path)
            fd = os.open(destination,os.O_RDONLY|os.O_DIRECTORY)
            try: os.fsync(fd)
            finally: os.close(fd)
            credentials.write_private(destination/'complete','Offline issuance complete; runtime not qualified.\n')
            existing._sync_file(destination/'complete')
            for path in (destination,parent):
                fd = os.open(path,os.O_RDONLY|os.O_DIRECTORY)
                try: os.fsync(fd)
                finally: os.close(fd)
        finally: tempfile.tempdir = old_tempdir; os.umask(old_umask)
        return receipt
    except Exception:
        raise IssueError('Space issuance refused or incomplete; preserve operation directory') from None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--operation-id',required=True)
    parser.add_argument('--media-public',required=True,type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r'media-c[12]-[a-f0-9]{32}',args.operation_id): raise IssueError('invalid operation')
    print(json.dumps(stage(PARENT/('space-p3-'+args.operation_id[-32:]),existing._public_input(args.media_public)),sort_keys=True))


if __name__ == '__main__':
    try: main()
    except Exception:
        print('Space issuance refused or incomplete; preserve operation directory',file=sys.stderr)
        sys.exit(1)
