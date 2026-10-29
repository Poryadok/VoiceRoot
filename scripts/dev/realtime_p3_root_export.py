"""Send an already completed P3 packet to the same-host operator once.

The receiver owns the actual operation fence and independently verifies the
packet, source, namespace, live CA and peer context. This is not admission.
The human root must authenticate consumer_pid against the live operation claim,
including process start time and PID reuse; the integer is not authority.
source_sha is likewise independently verified by the receiver.
"""
import argparse
import array
import fcntl
import json
import os
from pathlib import Path
import pwd
import re
import socket
import stat
import struct
import sys

ROOT = Path('/home/voice-dev/issuance')
SOCKET_ROOT = Path('/home/pmd/voice-realtime-p3/operations')
REQUIRED_SEALS = fcntl.F_SEAL_WRITE | fcntl.F_SEAL_GROW | fcntl.F_SEAL_SHRINK | fcntl.F_SEAL_SEAL
ASSET_KEYS = {
    'voice-realtime-principal-signing': {'current.pem', 'next.pem'},
    'voice-p3-principal-public': {'realtime.json'},
    'voice-realtime-p3-tls': {'tls.crt', 'tls.key'},
    'voice-chat-realtime-p3-tls': {'tls.crt', 'tls.key'},
    'voice-p3-principal-jwks-tls': {'tls.crt', 'tls.key'},
    'voice-p3-principal-replay': {'password'},
    'voice-p3-principal-trust': {'chat-server-ca.crt', 'space-client-ca.crt', 'space-jwks-ca.crt', 'realtime-client-ca.crt', 'realtime-jwks-ca.crt'},
}


class ExportError(RuntimeError): pass


def _check_root_custody(path):
    for directory in (path, *path.parents):
        info = directory.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022 or directory.resolve() != directory:
            raise ValueError()
    info = path.stat()
    if stat.S_IMODE(info.st_mode) != 0o700:
        raise ValueError()


def _read(path, limit):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        before = os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != 0 or before.st_nlink != 1
                or stat.S_IMODE(before.st_mode) != 0o600 or not 0 < before.st_size <= limit):
            raise ValueError()
        chunks, remaining = [], before.st_size + 1
        while remaining:
            chunk = os.read(fd, min(65536, remaining))
            if not chunk: break
            chunks.append(chunk); remaining -= len(chunk)
        after = os.fstat(fd)
        if ((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) !=
                (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns)):
            raise ValueError()
        raw = b''.join(chunks)
        if len(raw) != before.st_size: raise ValueError()
        return raw
    finally: os.close(fd)


def _pmd_uid(): return pwd.getpwnam('pmd').pw_uid


def _start_ticks(pid):
    data = Path('/proc').joinpath(str(pid), 'stat').read_bytes()
    if len(data) > 8192 or b') ' not in data: raise ValueError()
    return int(data.rsplit(b') ', 1)[1].split()[19])


def _socket_path(suffix):
    uid = _pmd_uid()
    for directory in (SOCKET_ROOT.parent, SOCKET_ROOT, SOCKET_ROOT / suffix):
        info = directory.lstat()
        if (not stat.S_ISDIR(info.st_mode) or info.st_uid != uid
                or stat.S_IMODE(info.st_mode) != 0o700 or directory.resolve() != directory):
            raise ValueError()
    path = SOCKET_ROOT / suffix / 'packet.sock'
    info = path.lstat()
    if not stat.S_ISSOCK(info.st_mode) or info.st_uid != uid or info.st_mode & 0o077:
        raise ValueError()
    return path


def _check_objects(bundle):
    if (not isinstance(bundle, dict) or set(bundle) != {'apiVersion', 'kind', 'items'}
            or bundle.get('apiVersion') != 'v1' or bundle.get('kind') != 'List'):
        raise ValueError()
    items = bundle.get('items')
    if not isinstance(items, list) or len(items) != len(ASSET_KEYS): raise ValueError()
    seen = set()
    for item in items:
        metadata = item['metadata']; name = metadata['name']
        kind = 'ConfigMap' if name == 'voice-p3-principal-public' else 'Secret'
        if (name not in ASSET_KEYS or name in seen or item.get('apiVersion') != 'v1' or item.get('kind') != kind
                or set(metadata) != {'name', 'namespace'} or metadata.get('namespace') != 'voice-dev'
                or set(item) != ({'apiVersion', 'kind', 'metadata', 'data', 'immutable'} if kind == 'ConfigMap'
                                 else {'apiVersion', 'kind', 'metadata', 'data', 'type'})
                or (kind == 'Secret' and item.get('type') != 'Opaque')
                or not isinstance(item.get('data'), dict)
                or any(not isinstance(value, str) for value in item['data'].values())
                or set(item.get('data', {})) != ASSET_KEYS[name]
                or (kind == 'ConfigMap' and item.get('immutable') is not True)):
            raise ValueError()
        seen.add(name)


def export(operation_id, source_sha, consumer_pid, consumer_start_ticks):
    try:
        if (os.geteuid() != 0 or type(consumer_pid) is not int or consumer_pid <= 0
                or type(consumer_start_ticks) is not int or consumer_start_ticks <= 0
                or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}', operation_id) or not re.fullmatch(r'[a-f0-9]{40}', source_sha)):
            raise ValueError()
        suffix = operation_id[-32:]
        _check_root_custody(ROOT)
        directory = ROOT / ('p3-' + suffix)
        _check_root_custody(directory)
        if (directory / 'handoff-started').exists() or (directory / 'handoff-started').is_symlink(): raise ValueError()
        if _read(ROOT / 'p3-issuance.once', 128) != (directory.name + '\n').encode(): raise ValueError()
        if _read(directory / 'complete', 1024) != b'Packet complete; runtime and cluster are not qualified.\n': raise ValueError()
        receipt = json.loads(_read(directory / 'receipt.json', 16384))
        if (set(receipt) != {'status', 'namespace', 'object_names', 'principal_ca_der_sha256', 'media_ca_der_sha256',
                             'space_jwks_cert_der_sha256', 'runtime_qualified', 'cluster_applied'}
                or any(not isinstance(receipt.get(key), str) or not re.fullmatch(r'[a-f0-9]{64}', receipt[key])
                       for key in ('principal_ca_der_sha256', 'media_ca_der_sha256', 'space_jwks_cert_der_sha256'))
                or receipt.get('status') != 'staged' or receipt.get('namespace') != 'voice-dev'
                or receipt.get('runtime_qualified') is not False or receipt.get('cluster_applied') is not False):
            raise ValueError()
        objects = json.loads(_read(directory / 'objects.json', 262144))
        _check_objects(objects)
        if receipt.get('object_names') != sorted(ASSET_KEYS): raise ValueError()
        envelope = json.dumps({'operation_id': operation_id, 'source_sha': source_sha,
                               'issuer_receipt': receipt, 'objects': objects}, separators=(',', ':')).encode()
        if len(envelope) > 280000: raise ValueError()
        memfd = os.memfd_create('voice-p3-packet', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING)
        try:
            os.fchmod(memfd, 0o600)
            remaining = memoryview(envelope)
            while remaining:
                size = os.write(memfd, remaining)
                if size <= 0: raise ValueError()
                remaining = remaining[size:]
            os.lseek(memfd, 0, os.SEEK_SET)
            fcntl.fcntl(memfd, fcntl.F_ADD_SEALS, REQUIRED_SEALS)
            if fcntl.fcntl(memfd, fcntl.F_GET_SEALS) & REQUIRED_SEALS != REQUIRED_SEALS: raise ValueError()
            with socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET) as channel:
                channel.settimeout(10)
                channel.connect(str(_socket_path(suffix)))
                pid, uid, _ = struct.unpack('3i', channel.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, struct.calcsize('3i')))
                if uid != _pmd_uid() or pid != consumer_pid or _start_ticks(pid) != consumer_start_ticks: raise ValueError()
                marker = os.open(directory / 'handoff-started', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
                try:
                    os.write(marker, b'Potentially sent; reconcile before any further handoff.\n')
                    os.fsync(marker)
                finally: os.close(marker)
                dirfd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY)
                try: os.fsync(dirfd)
                finally: os.close(dirfd)
                frame = b'P3-PACKET\n'
                if channel.sendmsg([frame], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', [memfd]))]) != len(frame):
                    raise ValueError()
        finally: os.close(memfd)
        return {'status': 'send_attempted', 'receiver_verified': False, 'admission_qualified': False}
    except Exception:
        raise ExportError('handoff failed or uncertain; retain packet and reconcile') from None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--operation-id', required=True)
    parser.add_argument('--source-sha', required=True)
    parser.add_argument('--consumer-pid', required=True, type=int)
    parser.add_argument('--consumer-start-ticks', required=True, type=int)
    args = parser.parse_args()
    print(json.dumps(export(args.operation_id, args.source_sha, args.consumer_pid, args.consumer_start_ticks), sort_keys=True))


if __name__ == '__main__':
    try: main()
    except Exception:
        print('handoff failed or uncertain; retain packet and reconcile', file=sys.stderr)
        sys.exit(1)
