"""Fixed DEV Realtime seven-object metadata and root-held principal CA checks."""
import base64
import copy
import hashlib
import os
from pathlib import Path
import re

import realtime_p3_create_only_packet as packet
import realtime_p3_missing_issuer as custody
import reset_credentials as reset
import space_p3_issuer_preflight_metadata_root as base

MEMBERS = {(kind.lower(), name): keys for (kind, name), keys in packet.MEMBERS.items()}
if len(MEMBERS) != 7:
    raise RuntimeError('Realtime P3 inventory differs')
CERT_TEMPLATE = '{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}{{.metadata.deletionTimestamp}}{{"\\n"}}{{index .data "%s"}}'


def _item(raw, kind, name):
    if not raw:
        return {'state': 'absent'}
    lines = raw.decode('ascii').splitlines()
    if (len(lines) < 3 or any(not re.fullmatch(r'[A-Za-z0-9.-]{1,128}', value) for value in lines[:2]) or
            lines[2] not in ('', '<no value>') or len(lines[3:]) != len(set(lines[3:])) or
            set(lines[3:]) != MEMBERS[kind, name]):
        raise ValueError()
    return {'state': 'present', 'uid': lines[0], 'resourceVersion': lines[1], 'keys': sorted(lines[3:])}


def _cert(binary, config, name, key, expected):
    raw = base._read(binary, config, ['-n', 'voice-dev', 'get', 'secret', name, '-o',
                                      'go-template=' + CERT_TEMPLATE % key])
    parts = raw.split(b'\n', 3)
    if (len(parts) != 4 or parts[0].decode('ascii') != expected['uid'] or
            parts[1].decode('ascii') != expected['resourceVersion'] or parts[2] not in (b'', b'<no value>')):
        raise ValueError()
    pem = base64.b64decode(parts[3], validate=True)
    if not re.fullmatch(rb'-----BEGIN CERTIFICATE-----\r?\n(?:[A-Za-z0-9+/=]+\r?\n)+-----END CERTIFICATE-----\r?\n?', pem):
        raise ValueError()
    fd = os.memfd_create('p3-public', os.MFD_CLOEXEC)
    try:
        os.write(fd, pem)
        der = reset._openssl('x509', '-in', Path('/proc') / str(os.getpid()) / 'fd' / str(fd), '-outform', 'DER')
        if hashlib.sha256(der).hexdigest() != expected['der_sha256']:
            raise ValueError()
    finally:
        os.close(fd)
    return pem


def check_principal(principal):
    custody._check_ancestors(custody.ARCHIVE)
    info = custody.ARCHIVE.lstat()
    if not __import__('stat').S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o077:
        raise ValueError()
    cert_fd, cert_id = custody._open_archive('principal-ca.crt', False)
    try:
        key_fd, key_id = custody._open_archive('principal-ca.key', True)
        try:
            cert = Path('/proc') / str(os.getpid()) / 'fd' / str(cert_fd)
            key = Path('/proc') / str(os.getpid()) / 'fd' / str(key_fd)
            if (hashlib.sha256(reset._openssl('x509', '-in', cert, '-outform', 'DER')).hexdigest() != principal['der_sha256'] or
                    reset._openssl('x509', '-in', cert, '-pubkey', '-noout') != reset._openssl('pkey', '-in', key, '-pubout') or
                    b'CA:TRUE' not in reset._openssl('x509', '-in', cert, '-noout', '-ext', 'basicConstraints') or
                    not custody._unchanged(cert_fd, cert_id) or not custody._unchanged(key_fd, key_id)):
                raise ValueError()
            for name, expected in [('principal-ca.crt', cert_id), ('principal-ca.key', key_id)]:
                current = (custody.ARCHIVE / name).lstat()
                if (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns, current.st_ctime_ns) != expected:
                    raise ValueError()
        finally:
            os.close(key_fd)
    finally:
        os.close(cert_fd)


def observe(namespace_uid, context, lock, media, principal, space=None):
    """Recheck fixed TT/lock/namespace, public roots and seven metadata twice."""
    base.observe(namespace_uid, context, lock, media)
    check_principal(principal)
    binary, bid = base._open(base.K3S, 512 * 1024 * 1024)
    try:
        config, cid = base._open(base.KUBECONFIG, 1024 * 1024)
        try:
            if base._digest(binary, bid[2]) != base.K3S_SHA256:
                raise ValueError()
            def snapshot():
                roots = {'principal': _cert(binary, config, 'voice-principal-ca', 'ca.crt', principal),
                         'media': _cert(binary, config, 'voice-media-ca', 'ca.crt', media)}
                if space is not None:
                    roots['space'] = _cert(binary, config, 'voice-space-media-server-tls', 'tls.crt', space)
                    packet.crypto._call(['verify', '-trusted', '{1}', '-purpose', 'sslserver',
                                         '-verify_hostname', 'voice-space.voice-dev.svc', '{0}'],
                                        [roots['space'], roots['media']])
                    packet.crypto.space_certificate(roots['space'], roots['media'])
                values = {}
                for kind, name in sorted(MEMBERS):
                    raw = base._read(binary, config, ['-n', 'voice-dev', 'get', kind, name,
                                                       '--ignore-not-found=true', '-o', 'go-template=' + base.TEMPLATE])
                    values[kind + '/' + name] = _item(raw, kind, name)
                return roots, values
            first = snapshot()
            base.observe(namespace_uid, context, lock, media)
            second = snapshot()
            base.observe(namespace_uid, context, lock, media)
            check_principal(principal)
            if (first != second or not custody._unchanged(binary, bid) or not custody._unchanged(config, cid) or
                    (base.K3S.lstat().st_dev, base.K3S.lstat().st_ino) != bid[:2] or
                    (base.KUBECONFIG.lstat().st_dev, base.KUBECONFIG.lstat().st_ino) != cid[:2] or
                    base._digest(binary, bid[2]) != base.K3S_SHA256):
                raise ValueError()
            return copy.deepcopy(first[1]), first[0]
        finally:
            os.close(config)
    finally:
        os.close(binary)
