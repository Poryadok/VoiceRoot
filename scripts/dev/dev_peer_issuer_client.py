"""Unprivileged fixed DEV peer issuer service trigger."""
import array
import fcntl
import json
import os
import socket
import struct

import dev_peer_issuer_preflight as preflight
import realtime_p3_root_export as existing

SOCKET = '/run/voice-dev-peer-issuer.sock'


def _trigger(phase, token, action):
    if type(phase) is not preflight.PeerIssuerPreflightPhase or action not in ('preflight', 'issue'):
        raise ValueError()
    if action == 'issue':
        preflight.verify_current(phase, token)
    else:
        phase._authority()
        if phase.record is None or phase.record.get('status') != 'ready':
            raise ValueError()
    package = phase.package.verify_current(token)
    request = {'action': action, 'profile': phase.profile, 'operation_id': phase.fence.context['operation_id'],
               'source_sha': phase.fence.context['commit'], 'run_id': package['run_id'],
               'run_attempt': package['run_attempt']}
    raw = json.dumps(request, sort_keys=True, separators=(',', ':')).encode('ascii')
    secret = token.encode('ascii')
    if not secret or len(secret) > 8192:
        raise ValueError()
    fd = os.memfd_create('voice-dev-github-token', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING)
    try:
        os.fchmod(fd, 0o600)
        remaining = memoryview(secret)
        while remaining:
            count = os.write(fd, remaining)
            if count <= 0:
                raise ValueError()
            remaining = remaining[count:]
        os.lseek(fd, 0, os.SEEK_SET)
        fcntl.fcntl(fd, fcntl.F_ADD_SEALS, existing.REQUIRED_SEALS)
        with socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET) as channel:
            channel.settimeout(300)
            channel.connect(SOCKET)
            _, uid, _ = struct.unpack('3i', channel.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
            if uid != 0:
                raise ValueError()
            if channel.sendmsg([raw], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', [fd]))]) != len(raw):
                raise ValueError()
            response, ancillary, flags, _ = channel.recvmsg(16, 1)
            if ancillary or flags & (socket.MSG_TRUNC | socket.MSG_CTRUNC) or response != b'OK\n':
                raise ValueError()
    finally:
        os.close(fd)
    phase._authority()


def trigger_preflight(phase, token):
    """Run while the same phase's prepared listener accepts the root check."""
    return _trigger(phase, token, 'preflight')


def trigger_issue(phase, token):
    """Run while the typed profile receiver accepts the root callback."""
    return _trigger(phase, token, 'issue')
