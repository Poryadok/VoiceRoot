"""Prepare and install one fixed, reviewed DEV peer signer bundle.

The root invocation must itself be delivered as a reviewed inline bootstrap
which pins the exact bundle digest. This module never installs on import.
"""
import hashlib
import base64
import io
import os
from pathlib import Path
import re
import stat
import subprocess
import tarfile
import zlib

import dev_peer_issuer_authority as authority
import dev_peer_issuer_source as reviewed
import realtime_p3_create_only_source as git_source
import realtime_p3_missing_issuer as custody

MAX_BUNDLE = 16 * 1024 * 1024
BOOTSTRAP_ARCHIVE = '/home/pmd/voice-dev-peer-issuer-bootstrap.tar'
SOCKET_UNIT = b'''[Unit]
Description=Voice DEV fixed peer issuer socket
[Socket]
ListenSequentialPacket=/run/voice-dev-peer-issuer.sock
Accept=yes
SocketUser=pmd
SocketMode=0600
RemoveOnStop=yes
[Install]
WantedBy=sockets.target
'''
SERVICE_UNIT = b'''[Unit]
Description=Voice DEV fixed peer issuer request
[Service]
Type=oneshot
User=root
StandardInput=socket
StandardOutput=null
StandardError=null
NoNewPrivileges=yes
ExecStart=/usr/bin/env -i PATH=/usr/bin:/bin LANG=C LC_ALL=C /usr/bin/python3 -I -S /opt/voice-dev-peer-issuer/scripts/dev/dev_peer_issuer_service.py
'''


def build(source_root, candidate, output):
    """Create a deterministic private bundle from an independently reviewed cohort."""
    reviewed.verify(source_root, candidate)
    archive = io.BytesIO()
    with tarfile.open(fileobj=archive, mode='w', format=tarfile.USTAR_FORMAT) as tar:
        for path in sorted(authority.FILES):
            value = git_source._blob(source_root, candidate, path)
            if git_source._disk(source_root, path) != value:
                raise ValueError()
            item = tarfile.TarInfo(path)
            item.size, item.mode, item.uid, item.gid, item.mtime = len(value), 0o644, 0, 0, 0
            tar.addfile(item, io.BytesIO(value))
    raw = archive.getvalue()
    if not 0 < len(raw) <= MAX_BUNDLE:
        raise ValueError()
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        view = memoryview(raw)
        while view:
            count = os.write(fd, view)
            if count <= 0:
                raise ValueError()
            view = view[count:]
        os.fsync(fd)
    finally:
        os.close(fd)
    return hashlib.sha256(raw).hexdigest()


def bootstrap(bundle_sha256):
    """Return the one-time inline bootstrap for one already-reviewed bundle."""
    if not isinstance(bundle_sha256, str) or not re.fullmatch(r'[a-f0-9]{64}', bundle_sha256):
        raise ValueError()
    prefix = ('EXPECTED = ' + repr(bundle_sha256) + '\n'
              'FILES = ' + repr(tuple(sorted(authority.FILES))) + '\n'
              'ARCHIVE = ' + repr(BOOTSTRAP_ARCHIVE) + '\n')
    program = r'''
import hashlib, io, os, pwd, stat, sys, tarfile, tempfile
from pathlib import Path
if os.geteuid() != 0:
    raise SystemExit('root required for one-time bootstrap')
fd = os.open(ARCHIVE, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
try:
    before = os.fstat(fd)
    if (not stat.S_ISREG(before.st_mode) or before.st_uid != pwd.getpwnam('pmd').pw_uid
            or before.st_nlink != 1 or stat.S_IMODE(before.st_mode) != 0o600
            or not 0 < before.st_size <= 16 * 1024 * 1024):
        raise ValueError('invalid bundle custody')
    raw = bytearray()
    while len(raw) < before.st_size:
        part = os.read(fd, min(65536, before.st_size - len(raw)))
        if not part:
            raise ValueError('truncated bundle')
        raw.extend(part)
    def identity(value):
        return (value.st_dev, value.st_ino, value.st_size, value.st_mtime_ns, value.st_ctime_ns, value.st_mode, value.st_uid, value.st_nlink)
    if identity(before) != identity(os.fstat(fd)) or identity(before) != identity(os.stat(ARCHIVE, follow_symlinks=False)):
        raise ValueError('bundle changed')
finally:
    os.close(fd)
raw = bytes(raw)
if hashlib.sha256(raw).hexdigest() != EXPECTED:
    raise ValueError('bundle digest differs')
with tempfile.TemporaryDirectory(prefix='voice-dev-peer-bootstrap-') as temp:
    base = Path(temp)
    names = set()
    with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
        for item in archive:
            if (item.name not in FILES or item.name in names or not item.isfile()
                    or item.issym() or item.islnk() or not 0 < item.size <= 1024 * 1024):
                raise ValueError('invalid bundle member')
            names.add(item.name)
            value = archive.extractfile(item).read(item.size + 1)
            if len(value) != item.size:
                raise ValueError('truncated bundle member')
            path = base / item.name
            path.parent.mkdir(parents=True, exist_ok=True)
            with path.open('xb') as output:
                output.write(value)
    if names != set(FILES):
        raise ValueError('missing bundle member')
    sys.path[:0] = [str(base / 'scripts/dev'), str(base / 'scripts/staging/nats-rollout-preservation')]
    import dev_peer_issuer_installer
    dev_peer_issuer_installer.install(raw, EXPECTED)
'''
    encoded = base64.b64encode(zlib.compress((prefix + program).encode('utf-8'), 9)).decode('ascii')
    return ("sudo /usr/bin/env -i PATH=/usr/bin:/bin LANG=C LC_ALL=C /usr/bin/python3 -I -S -c "
            + "'import base64,zlib;exec(zlib.decompress(base64.b64decode(\"" + encoded + "\")))'\n")


def install(raw, expected_sha256):
    """Install only bytes authenticated by the reviewed one-time bootstrap."""
    if (os.geteuid() != 0 or not isinstance(raw, bytes) or not 0 < len(raw) <= MAX_BUNDLE or
            not isinstance(expected_sha256, str) or not re.fullmatch(r'[a-f0-9]{64}', expected_sha256) or
            hashlib.sha256(raw).hexdigest() != expected_sha256):
        raise ValueError()
    files = {}
    with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
        for item in archive:
            if (item.name not in authority.FILES or item.name in files or not item.isfile() or
                    item.issym() or item.islnk() or item.size <= 0 or item.size > 1024 * 1024):
                raise ValueError()
            value = archive.extractfile(item).read(item.size + 1)
            if len(value) != item.size:
                raise ValueError()
            files[item.name] = value
    if set(files) != set(authority.FILES):
        raise ValueError()
    custody._check_ancestors(Path('/opt'))
    custody._check_ancestors(Path('/etc/systemd/system'))
    authority.INSTALLED.mkdir(mode=0o755)
    for name, value in sorted(files.items()):
        target = authority.INSTALLED / name
        target.parent.mkdir(mode=0o755, parents=True, exist_ok=True)
        fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o644)
        try:
            view = memoryview(value)
            while view:
                count = os.write(fd, view)
                if count <= 0:
                    raise ValueError()
                view = view[count:]
            os.fsync(fd)
        finally:
            os.close(fd)
    for name, value in (('voice-dev-peer-issuer.socket', SOCKET_UNIT),
                        ('voice-dev-peer-issuer@.service', SERVICE_UNIT)):
        fd = os.open(Path('/etc/systemd/system') / name,
                     os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o644)
        try:
            if os.write(fd, value) != len(value):
                raise ValueError()
            os.fsync(fd)
        finally:
            os.close(fd)
    directories = {authority.INSTALLED, authority.INSTALLED.parent, Path('/etc/systemd/system')}
    for name in files:
        parent = (authority.INSTALLED / name).parent
        while parent != authority.INSTALLED:
            directories.add(parent)
            parent = parent.parent
    for path in sorted(directories, key=lambda value: len(value.parts), reverse=True):
        fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    for command in (['/usr/bin/systemctl', 'daemon-reload'],
                    ['/usr/bin/systemctl', 'enable', '--now', 'voice-dev-peer-issuer.socket']):
        subprocess.run(command, check=True, timeout=30, stdin=subprocess.DEVNULL,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                       env={'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C'})
