"""Fixed root-side Realtime P3 capability and one-shot handoff."""
import array
import fcntl
import hashlib
import json
import os
import re
import socket
import struct

import dev_realtime_p3_issuer_metadata as metadata
import realtime_p3_create_only_packet as packet
import realtime_p3_create_only_source as source
import realtime_p3_missing_issuer as issuer
import realtime_p3_root_export as export

FRAME = b'VOICE-DEV-P3-PACKET\n'
ACK = b'VOICE-DEV-P3-ACCEPTED\n'


def capability_check(service, operation, source_sha, pid, start):
    with service._connect(operation, 'realtime', 'preflight', pid, start) as channel:
        request = json.loads(service._read(channel, 65536), object_pairs_hook=service._unique)
        if (set(request) != {'context', 'namespace_uid', 'lock', 'nonce', 'principal', 'media', 'objects'} or
                request['context'].get('operation_id') != operation or request['context'].get('commit') != source_sha or
                not re.fullmatch(r'[a-f0-9]{32}', request['nonce'])):
            raise ValueError()
        objects, _ = metadata.observe(request['namespace_uid'], request['context'], request['lock'],
                                      request['media'], request['principal'])
        if objects != request['objects']:
            raise ValueError()
        reply = {'schema': 'voice-dev-realtime-p3-capability-v1', 'operation_id': operation,
                 'source_sha': source_sha, 'namespace_uid': request['namespace_uid'], 'nonce': request['nonce'],
                 'context_sha256': hashlib.sha256(json.dumps(request['context'], sort_keys=True, separators=(',', ':')).encode()).hexdigest(), 'principal': request['principal'],
                 'media': request['media'], 'objects': objects, 'scope': 'root-principal-custody-available-not-issued'}
        raw = json.dumps(reply, sort_keys=True, separators=(',', ':')).encode('ascii')
        if len(raw) > 65536 or channel.send(raw) != len(raw):
            raise ValueError()


def _exchange(service, channel, operation, source_sha):
    if channel.send(b'PREFLIGHT-REQUEST\n') != len(b'PREFLIGHT-REQUEST\n'):
        raise ValueError()
    request = json.loads(service._read(channel, 65536), object_pairs_hook=service._unique)
    if service._read(channel, 64) != b'PREFLIGHT-READY\n':
        raise ValueError()
    if (set(request) != {'context', 'namespace_uid', 'lock', 'principal', 'media', 'space', 'objects',
                         'capability_sha256', 'issuer_source_sha'} or
            request['context'].get('operation_id') != operation or request['context'].get('commit') != source_sha or
            request['issuer_source_sha'] != source.REVIEWED_ISSUER_SHA or
            not re.fullmatch(r'[a-f0-9]{64}', request['capability_sha256'])):
        raise ValueError()
    return request


def _guard(request):
    objects, roots = metadata.observe(request['namespace_uid'], request['context'], request['lock'],
                                      request['media'], request['principal'], request['space'])
    if (objects != request['objects'] or set(objects) != {kind + '/' + name for kind, name in metadata.MEMBERS} or
            any(value != {'state': 'absent'} for value in objects.values())):
        raise ValueError()
    return roots


def issue(service, operation, source_sha, pid, start):
    export._check_root_custody(export.ROOT)
    channel = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
    try:
        channel.settimeout(120)
        channel.connect(str(export._socket_path(operation[-32:])))
        peer, uid, _ = struct.unpack('3i', channel.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
        if uid != export._pmd_uid() or peer != pid or export._start_ticks(peer) != start:
            raise ValueError()
        first = _exchange(service, channel, operation, source_sha)
        roots = _guard(first)
        directory = export.ROOT / ('p3-' + operation[-32:])
        issuer.stage(directory, roots['principal'], roots['media'], roots['space'])
        if _exchange(service, channel, operation, source_sha) != first:
            raise ValueError()
        if _guard(first) != roots:
            raise ValueError()
        receipt = json.loads(export._read(directory / 'receipt.json', 16384), object_pairs_hook=service._unique)
        objects = json.loads(export._read(directory / 'objects.json', 262144), object_pairs_hook=service._unique)
        envelope = {'operation_id': operation, 'source_sha': source.REVIEWED_ISSUER_SHA,
                    'issuer_receipt': receipt, 'objects': objects}
        packet.validate(envelope, operation, source.REVIEWED_ISSUER_SHA, roots['principal'], roots['media'], roots['space'])
        raw = json.dumps(envelope, sort_keys=True, separators=(',', ':')).encode('ascii')
        if not 0 < len(raw) <= 280000:
            raise ValueError()
        fd = os.memfd_create('voice-dev-realtime-p3', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING)
        try:
            os.fchmod(fd, 0o600)
            view = memoryview(raw)
            while view:
                count = os.write(fd, view)
                if count <= 0:
                    raise ValueError()
                view = view[count:]
            os.lseek(fd, 0, os.SEEK_SET)
            fcntl.fcntl(fd, fcntl.F_ADD_SEALS, export.REQUIRED_SEALS)
            marker = os.open(directory / 'handoff-started', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
            try:
                os.write(marker, b'Potentially sent; reconcile before retry.\n')
                os.fsync(marker)
            finally:
                os.close(marker)
            service._sync_dir(directory)
            if channel.sendmsg([FRAME], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', [fd]))]) != len(FRAME):
                raise ValueError()
            if service._read(channel, 128) != ACK:
                raise ValueError()
        finally:
            os.close(fd)
    finally:
        channel.close()
