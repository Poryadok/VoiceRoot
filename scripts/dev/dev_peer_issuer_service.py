"""Socket-activated, fixed DEV peer issuer. Never exposes the retained CA key."""
import array
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import stat
import struct
import tempfile
import sys

sys.path.insert(0, '/opt/voice-dev-peer-issuer/scripts/dev')
sys.path.insert(0, '/opt/voice-dev-peer-issuer/scripts/staging/nats-rollout-preservation')

import credentials
import dev_peer_issuer_authority as authority
import dev_peer_issuer_material as material
import dev_peer_issuer_metadata as metadata
import dev_peer_issuer_packet as packet
import realtime_p3_missing_issuer as existing
import realtime_p3_root_export as root_export
import space_p3_issuer as custody
import space_p3_issuer_preflight_root as preflight

ROOT = Path('/home/voice-dev/issuance')
BASE = Path('/home/pmd/voice-dev-peer-issuer')
FRAME = b'VOICE-DEV-PEER-PACKET\n'
ACK = b'VOICE-DEV-PEER-ACCEPTED\n'


def _unique(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError()
        value[key] = item
    return value


def _read(channel, limit):
    raw, ancillary, flags, _ = channel.recvmsg(limit + 1, 1)
    if not raw or len(raw) > limit or ancillary or flags & (socket.MSG_TRUNC | socket.MSG_CTRUNC):
        raise ValueError()
    return raw


def _path(operation, profile, purpose):
    suffix = operation[-32:]
    directories = (BASE, BASE / purpose, BASE / purpose / suffix)
    if purpose == 'preflight':
        directories += (BASE / purpose / suffix / profile,)
        name = 'custody.sock'
    else:
        name = profile + '.sock'
    uid = root_export._pmd_uid()
    for directory in directories:
        value = directory.lstat()
        if (not stat.S_ISDIR(value.st_mode) or value.st_uid != uid or
                stat.S_IMODE(value.st_mode) != 0o700 or directory.resolve() != directory):
            raise ValueError()
    path = directories[-1] / name
    value = path.lstat()
    if not stat.S_ISSOCK(value.st_mode) or value.st_uid != uid or value.st_mode & 0o077:
        raise ValueError()
    return path


def _connect(operation, profile, purpose, pid, start):
    channel = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
    try:
        channel.settimeout(120)
        channel.connect(str(_path(operation, profile, purpose)))
        peer, uid, _ = struct.unpack('3i', channel.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
        if uid != root_export._pmd_uid() or peer != pid or root_export._start_ticks(peer) != start:
            raise ValueError()
        return channel
    except BaseException:
        channel.close()
        raise


def _guard(request):
    preflight._check_ca(request['ca'])
    if metadata.observe(request['namespace_uid'], request['context'], request['lock'], request['ca']) != request['creator_secrets']:
        raise ValueError()
    preflight._check_ca(request['ca'])


def _exchange(channel, operation, source_sha, profile):
    if channel.send(b'PREFLIGHT-REQUEST\n') != len(b'PREFLIGHT-REQUEST\n'):
        raise ValueError()
    request = json.loads(_read(channel, 65536), object_pairs_hook=_unique)
    if _read(channel, 64) != b'PREFLIGHT-READY\n':
        raise ValueError()
    if (set(request) != {'context', 'namespace_uid', 'lock', 'ca', 'creator_secrets', 'profile', 'preflight_sha256'}
            or request['profile'] != profile or request['context'].get('operation_id') != operation
            or request['context'].get('commit') != source_sha or
            not re.fullmatch(r'[a-f0-9]{64}', request['preflight_sha256']) or
            set(request['creator_secrets']) != set(metadata.MEMBERS)):
        raise ValueError()
    return request


def _sync_dir(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def _assert_no_legacy(profile):
    marker = {'creator': 'space-creator-issuance.once', 'space': 'space-p3-issuance.once', 'role': None}[profile]
    if marker is None:
        return
    try:
        (ROOT / marker).lstat()
    except FileNotFoundError:
        return
    raise ValueError('legacy issuance needs reconciliation')


def check(operation, source_sha, profile, pid, start):
    with _connect(operation, profile, 'preflight', pid, start) as channel:
        request = json.loads(_read(channel, 65536), object_pairs_hook=_unique)
        if (set(request) != {'context', 'namespace_uid', 'nonce', 'ca', 'lock', 'profile'} or
                request['profile'] != profile or request['context'].get('operation_id') != operation or
                request['context'].get('commit') != source_sha or
                not re.fullmatch(r'[a-f0-9]{32}', request['nonce'])):
            raise ValueError()
        preflight._check_ca(request['ca'])
        observations = metadata.observe(request['namespace_uid'], request['context'], request['lock'], request['ca'])
        preflight._check_ca(request['ca'])
        context_hash = hashlib.sha256(json.dumps(request['context'], sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        response = {'schema': 'voice-dev-peer-custody-v1', 'operation_id': operation,
                    'source_sha': request['context']['commit'], 'namespace_uid': request['namespace_uid'],
                    'nonce': request['nonce'], 'context_sha256': context_hash,
                    'lock': {k: request['lock']['metadata'][k] for k in ('uid', 'resourceVersion')},
                    'ca': request['ca'], 'creator_secrets': observations,
                    'scope': 'root-peer-custody-available-not-issued', 'profile': profile}
        raw = json.dumps(response, sort_keys=True, separators=(',', ':')).encode('ascii')
        if len(raw) > 4096 or channel.send(raw) != len(raw):
            raise ValueError()


def issue(operation, source_sha, profile, pid, start):
    root_export._check_root_custody(ROOT)
    with _connect(operation, profile, 'operations', pid, start) as channel:
        first = _exchange(channel, operation, source_sha, profile)
        _guard(first)
        missing = sorted(name for name in metadata.PROFILES[profile]
                         if first['creator_secrets'][name] == {'state': 'absent'})
        if not missing:
            raise ValueError()
        _assert_no_legacy(profile)
        directory = ROOT / ('dev-peer-' + profile + '-' + operation[-32:])
        reservation = ROOT / ('dev-peer-' + profile + '-issuance.once')
        fd = os.open(reservation, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        try:
            os.write(fd, (directory.name + '\n').encode('ascii'))
            os.fsync(fd)
        finally:
            os.close(fd)
        _sync_dir(ROOT)
        directory.mkdir(mode=0o700)
        credentials.write_private(directory / 'pending', b'Generation begun; reconcile before retry.\n')
        existing._sync_file(directory / 'pending')
        _sync_dir(directory)
        cert_fd, cert_id = custody._open('media-ca.crt', False)
        try:
            media = os.read(cert_fd, 65537)
            if not existing._unchanged(cert_fd, cert_id) or len(media) != cert_id[2]:
                raise ValueError()
        finally:
            os.close(cert_fd)
        _guard(first)
        old_umask, old_temp = os.umask(0o077), tempfile.tempdir
        tempfile.tempdir = str(directory)
        try:
            receipt = material.generate(directory, media, profile, missing, operation, source_sha,
                                        first['namespace_uid'], first['preflight_sha256'])
        finally:
            tempfile.tempdir = old_temp
            os.umask(old_umask)
        if _exchange(channel, operation, source_sha, profile) != first:
            raise ValueError()
        _guard(first)
        credentials.write_private(directory / 'receipt.json', json.dumps(receipt, sort_keys=True, separators=(',', ':')) + '\n')
        for path in directory.iterdir():
            existing._sync_file(path)
        credentials.write_private(directory / 'complete', b'Issuance staged; runtime not qualified.\n')
        existing._sync_file(directory / 'complete')
        _sync_dir(directory)
        _sync_dir(ROOT)
        raw = (directory / 'objects.json').read_bytes()
        envelope = json.loads(raw, object_pairs_hook=_unique)
        packet.validate(envelope, profile, operation, source_sha, first['namespace_uid'],
                        first['preflight_sha256'], missing, media)
        fd = os.memfd_create('voice-dev-peer', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING)
        try:
            os.fchmod(fd, 0o600)
            view = memoryview(raw)
            while view:
                count = os.write(fd, view)
                if count <= 0:
                    raise ValueError()
                view = view[count:]
            os.lseek(fd, 0, os.SEEK_SET)
            fcntl.fcntl(fd, fcntl.F_ADD_SEALS, root_export.REQUIRED_SEALS)
            marker = os.open(directory / 'handoff-started', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
            try:
                os.write(marker, b'Potentially sent; reconcile before retry.\n')
                os.fsync(marker)
            finally:
                os.close(marker)
            _sync_dir(directory)
            if channel.sendmsg([FRAME], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', [fd]))]) != len(FRAME):
                raise ValueError()
            if _read(channel, 128) != ACK:
                raise ValueError()
        finally:
            os.close(fd)


def serve(channel):
    """One request from the fixed socket-activated pmd peer."""
    if os.geteuid() != 0 or channel.family != socket.AF_UNIX or channel.type & socket.SOCK_SEQPACKET != socket.SOCK_SEQPACKET:
        raise ValueError()
    pid, uid, _ = struct.unpack('3i', channel.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
    if uid != root_export._pmd_uid() or pid <= 0:
        raise ValueError()
    start = root_export._start_ticks(pid)
    raw, ancillary, flags, _ = channel.recvmsg(4097, socket.CMSG_SPACE(array.array('i').itemsize))
    fds = array.array('i')
    try:
        for level, kind, data in ancillary:
            if level == socket.SOL_SOCKET and kind == socket.SCM_RIGHTS:
                usable = len(data) - len(data) % fds.itemsize
                fds.frombytes(data[:usable])
        if flags & (socket.MSG_TRUNC | socket.MSG_CTRUNC) or not 0 < len(raw) <= 4096 or len(ancillary) != 1 or len(fds) != 1:
            raise ValueError()
        level, kind, data = ancillary[0]
        if level != socket.SOL_SOCKET or kind != socket.SCM_RIGHTS or len(data) != fds.itemsize:
            raise ValueError()
        info = os.fstat(fds[0])
        if (not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= 8192 or
                fcntl.fcntl(fds[0], fcntl.F_GET_SEALS) & root_export.REQUIRED_SEALS != root_export.REQUIRED_SEALS):
            raise ValueError()
        os.lseek(fds[0], 0, os.SEEK_SET)
        token = os.read(fds[0], 8193).decode('ascii')
        if len(token) != info.st_size:
            raise ValueError()
    finally:
        for fd in fds:
            os.close(fd)
    request = json.loads(raw, object_pairs_hook=_unique)
    if (set(request) != {'action', 'profile', 'operation_id', 'source_sha', 'run_id', 'run_attempt'} or
            not ((request['action'] in ('preflight', 'issue') and request['profile'] in metadata.PROFILES) or
                 (request['action'] in ('p3-capability', 'p3-issue') and request['profile'] == 'realtime')) or
            not re.fullmatch(r'media-c1-[a-f0-9]{32}', request['operation_id']) or
            not re.fullmatch(r'[a-f0-9]{40}', request['source_sha'])):
        raise ValueError()
    authority.verify(token, request['source_sha'], request['run_id'], request['run_attempt'])
    if root_export._start_ticks(pid) != start:
        raise ValueError()
    if request['action'] == 'preflight':
        check(request['operation_id'], request['source_sha'], request['profile'], pid, start)
    elif request['action'] == 'issue':
        issue(request['operation_id'], request['source_sha'], request['profile'], pid, start)
    else:
        import dev_realtime_p3_issuer_service as p3_service
        if request['action'] == 'p3-capability':
            p3_service.capability_check(sys.modules[__name__], request['operation_id'], request['source_sha'], pid, start)
        else:
            p3_service.issue(sys.modules[__name__], request['operation_id'], request['source_sha'], pid, start)
    if root_export._start_ticks(pid) != start or channel.send(b'OK\n') != 3:
        raise ValueError()


def main():
    if (Path(__file__).resolve() != Path('/opt/voice-dev-peer-issuer/scripts/dev/dev_peer_issuer_service.py') or
            len(sys.argv) != 1 or set(os.environ) != {'PATH', 'LANG', 'LC_ALL'} or
            os.environ['PATH'] != '/usr/bin:/bin' or os.environ['LANG'] != 'C' or
            os.environ['LC_ALL'] != 'C'):
        raise ValueError()
    with socket.fromfd(0, socket.AF_UNIX, socket.SOCK_SEQPACKET) as channel:
        channel.settimeout(300)
        serve(channel)


if __name__ == '__main__':
    try:
        main()
    except Exception:
        raise SystemExit('DEV peer issuer unavailable; reconcile any attempted issuance') from None
