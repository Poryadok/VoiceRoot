"""Unprivileged fixed DEV peer issuer service trigger."""
import array
import fcntl
import json
import os
import socket
import struct
import threading

import dev_peer_issuer_preflight as preflight
import realtime_p3_root_export as existing

SOCKET = '/run/voice-dev-peer-issuer.sock'


class _Control:
    def __init__(self):
        self._lock = threading.Lock()
        self._cancelled = False
        self._channel = None
        self._started = False

    def _start(self):
        with self._lock:
            if self._started or self._cancelled:
                raise ValueError()
            self._started = True

    def _check(self):
        with self._lock:
            if self._cancelled:
                raise ValueError()

    def _register(self, channel):
        with self._lock:
            if self._cancelled or self._channel is not None:
                raise ValueError()
            self._channel = channel

    def _memfd(self):
        with self._lock:
            if self._cancelled:
                raise ValueError()
            return os.memfd_create('voice-dev-github-token', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING)

    def _unregister(self, channel):
        with self._lock:
            if self._channel is channel:
                self._channel = None

    def cancel(self):
        with self._lock:
            self._cancelled = True
            channel = self._channel
            if channel is not None:
                try:
                    channel.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass
                channel.close()


class PeerTrigger(_Control):
    def __init__(self, phase, token, action):
        super().__init__()
        if type(phase) is not preflight.PeerIssuerPreflightPhase or action not in ('preflight', 'issue'):
            raise ValueError()
        self._phase, self._token, self._action = phase, token, action

    def run(self):
        self._start()
        return _trigger(self._phase, self._token, self._action, self)


def _trigger(phase, token, action, control=None):
    if type(phase) is not preflight.PeerIssuerPreflightPhase or action not in ('preflight', 'issue'):
        raise ValueError()
    if control is not None: control._check()
    if action == 'issue':
        preflight.verify_current(phase, token)
    else:
        phase._authority()
        if phase.record is None or phase.record.get('status') != 'ready':
            raise ValueError()
    if control is not None: control._check()
    package = phase.package.verify_current(token)
    if control is not None: control._check()
    request = {'action': action, 'profile': phase.profile, 'operation_id': phase.fence.context['operation_id'],
               'source_sha': phase.fence.context['commit'], 'run_id': package['run_id'],
               'run_attempt': package['run_attempt']}
    raw = json.dumps(request, sort_keys=True, separators=(',', ':')).encode('ascii')
    secret = token.encode('ascii')
    if not secret or len(secret) > 8192:
        raise ValueError()
    if control is not None: control._check()
    fd = (control._memfd() if control is not None else
          os.memfd_create('voice-dev-github-token', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING))
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
            if control is not None: control._register(channel)
            try:
                channel.settimeout(5)
                channel.connect(SOCKET)
                channel.settimeout(300)
                if control is not None: control._check()
                _, uid, _ = struct.unpack('3i', channel.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
                if uid != 0:
                    raise ValueError()
                if channel.sendmsg([raw], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', [fd]))]) != len(raw):
                    raise ValueError()
                response, ancillary, flags, _ = channel.recvmsg(16, 1)
                if ancillary or flags & (socket.MSG_TRUNC | socket.MSG_CTRUNC) or response != b'OK\n':
                    raise ValueError()
            finally:
                if control is not None: control._unregister(channel)
    finally:
        os.close(fd)
    if control is not None: control._check()
    # The receiver owns the durable transition and final authority recheck.
    # A worker-side guard here can race that local-to-ledger transition.


def trigger_preflight(phase, token):
    """Run while the same phase's prepared listener accepts the root check."""
    return _trigger(phase, token, 'preflight')


def trigger_issue(phase, token):
    """Run while the typed profile receiver accepts the root callback."""
    return _trigger(phase, token, 'issue')


def new_preflight_trigger(phase, token):
    return PeerTrigger(phase, token, 'preflight')


def new_issue_trigger(phase, token):
    return PeerTrigger(phase, token, 'issue')
