"""Fixed unprivileged trigger for the delegated Realtime P3 service."""
import array
import fcntl
import json
import os
import socket
import struct

import dev_peer_issuer_client as peer_client
import dev_realtime_p3_issuer_capability as capability
import realtime_p3_create_only as p3
import realtime_p3_root_export as export


class RealtimeTrigger(peer_client._Control):
    def __init__(self, value, token, action, session=None):
        super().__init__()
        if (type(value) is not capability.RealtimeP3Capability or action not in ('p3-capability', 'p3-issue') or
                (action == 'p3-capability' and session is not None) or
                (action == 'p3-issue' and type(session) is not p3.Session)):
            raise ValueError()
        self._value, self._token, self._action, self._session = value, token, action, session

    def run(self):
        self._start()
        return _trigger(self._value, self._token, self._action, self._session, self)


def _trigger(value, token, action, session=None, control=None):
    if type(value) is not capability.RealtimeP3Capability or token != value.token:
        raise ValueError()
    if control is not None: control._check()
    if action == 'p3-capability':
        if value.record is None or value.record.get('status') != 'ready' or session is not None:
            raise ValueError()
        value._guard()
    elif action == 'p3-issue':
        if type(session) is not p3.Session:
            raise ValueError()
        capability.verify_current(value, token)
        session._guard()
    else:
        raise ValueError()
    creator = value.creator_preflight
    if control is not None: control._check()
    package = creator.package.verify_current(token)
    if control is not None: control._check()
    request = {'action': action, 'profile': 'realtime', 'operation_id': creator.fence.context['operation_id'],
               'source_sha': creator.fence.context['commit'], 'run_id': package['run_id'],
               'run_attempt': package['run_attempt']}
    raw, secret = json.dumps(request, sort_keys=True, separators=(',', ':')).encode('ascii'), token.encode('ascii')
    if not secret or len(secret) > 8192:
        raise ValueError()
    if control is not None: control._check()
    fd = (control._memfd() if control is not None else
          os.memfd_create('voice-dev-github-token', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING))
    try:
        os.fchmod(fd, 0o600)
        view = memoryview(secret)
        while view:
            count = os.write(fd, view)
            if count <= 0:
                raise ValueError()
            view = view[count:]
        os.lseek(fd, 0, os.SEEK_SET)
        fcntl.fcntl(fd, fcntl.F_ADD_SEALS, export.REQUIRED_SEALS)
        with socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET) as channel:
            if control is not None: control._register(channel)
            try:
                channel.settimeout(5)
                channel.connect(peer_client.SOCKET)
                channel.settimeout(300)
                if control is not None: control._check()
                _, uid, _ = struct.unpack('3i', channel.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
                if uid != 0:
                    raise ValueError()
                if channel.sendmsg([raw], [(socket.SOL_SOCKET, socket.SCM_RIGHTS, array.array('i', [fd]))]) != len(raw):
                    raise ValueError()
                reply, ancillary, flags, _ = channel.recvmsg(16, 1)
                if ancillary or flags & (socket.MSG_TRUNC | socket.MSG_CTRUNC) or reply != b'OK\n':
                    raise ValueError()
            finally:
                if control is not None: control._unregister(channel)
    finally:
        os.close(fd)
    if control is not None: control._check()
    # The typed receiver completes and rechecks its durable transition after
    # joining this worker; reading its Journal here races that transition.


def trigger_capability(value, token):
    return _trigger(value, token, 'p3-capability')


def trigger_issue(value, token, session):
    return _trigger(value, token, 'p3-issue', session)


def new_capability_trigger(value, token):
    return RealtimeTrigger(value, token, 'p3-capability')


def new_issue_trigger(value, token, session):
    return RealtimeTrigger(value, token, 'p3-issue', session)
