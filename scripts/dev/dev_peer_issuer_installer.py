"""Prepare and install one fixed, reviewed DEV peer signer bundle.

The root invocation must itself be delivered as a reviewed inline bootstrap
which pins the exact bundle digest. This module never installs on import.
"""
import hashlib
import io
import os
from pathlib import Path
import re
import stat
import subprocess
import tarfile

import dev_peer_issuer_authority as authority
import dev_peer_issuer_source as reviewed
import realtime_p3_create_only_source as git_source
import realtime_p3_missing_issuer as custody

MAX_BUNDLE = 16 * 1024 * 1024
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
    for command in (['/usr/bin/systemctl', 'daemon-reload'],
                    ['/usr/bin/systemctl', 'enable', '--now', 'voice-dev-peer-issuer.socket']):
        subprocess.run(command, check=True, timeout=30, stdin=subprocess.DEVNULL,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                       env={'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C'})
