"""Protected pre-drain principal-CA capability and fixed seven-object snapshot."""
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import socket
import stat
import struct

import dev_peer_issuer_preflight as peer
import dev_realtime_p3_issuer_metadata as metadata
import realtime_p3_create_only as p3
import realtime_p3_create_only_source as source
import story_media_backup as backup
import writer_cut

SOCKET_ROOT = Path('/home/pmd/voice-dev-peer-issuer/preflight')
STEP = 'dev-realtime-p3-capability'
LEDGER = 'dev_realtime_p3_capability'


class CapabilityError(RuntimeError):
    pass


class RealtimeP3Capability:
    def __init__(self, creator_preflight, token):
        if (type(creator_preflight) is not peer.PeerIssuerPreflightPhase or
                creator_preflight.profile != 'creator' or token != creator_preflight.token):
            raise CapabilityError('typed creator authority required')
        self.creator_preflight, self.token = creator_preflight, token
        self.record = self.listener = self.dirfd = self.inode = None
        self.pending = False
        self._refs = (creator_preflight, creator_preflight.journal, creator_preflight.fence,
                      creator_preflight.client, creator_preflight.package, creator_preflight.cut)

    def _guard(self):
        c = self.creator_preflight
        if (self.token != c.token or any(a is not b for a, b in zip(
                (c, c.journal, c.fence, c.client, c.package, c.cut), self._refs))):
            raise ValueError()
        c._authority()
        row, rev = c.fence.ledger.read()
        if self.record is not None:
            local = c.journal.state['steps'].get(STEP)
            if (not isinstance(local, dict) or {k: v for k, v in local.items() if k != 'receipt'} != self.record or
                    row.get(LEDGER) != self.record or
                    self.record.get('status') == 'done' and writer_cut.digest(local.get('receipt')) != self.record['receipt_sha256']):
                raise ValueError()
        c._local()
        return row, rev

    def _principal(self):
        c = self.creator_preflight
        identity = peer.public._meta(peer.public.common.linux_private_bytes, self._guard, 'secret', 'voice-principal-ca')
        value = peer.public._value(peer.public.common.linux_private_bytes, self._guard, 'secret', 'voice-principal-ca', 'ca.crt')
        peer.public._public_pem(value)
        digest = hashlib.sha256(peer.crypto.der(value)).hexdigest()
        if peer.public._meta(peer.public.common.linux_private_bytes, self._guard, 'secret', 'voice-principal-ca') != identity:
            raise ValueError()
        return {'uid': identity['uid'], 'resourceVersion': identity['resource_version'], 'der_sha256': digest}

    def _seven(self):
        result = {}
        for (kind, name) in sorted(metadata.MEMBERS):
            raw = peer.public._call(peer.public.common.linux_private_bytes, self._guard, 'get', kind, name,
                                    '--ignore-not-found=true', '-o', 'go-template=' + metadata.base.TEMPLATE)
            result[kind + '/' + name] = metadata._item(raw, kind, name)
        return result

    def prepare(self):
        try:
            c = self.creator_preflight
            creator = peer.verify_current(c, self.token)
            principal, media, seven = self._principal(), c._ca(), self._seven()
            if peer.verify_current(c, self.token) != creator:
                raise ValueError()
            row, rev = self._guard()
            if STEP in c.journal.state['steps'] or LEDGER in row:
                raise ValueError()
            pending = {'status': 'pending', 'nonce': secrets.token_hex(16),
                       'operation_id': c.fence.context['operation_id'], 'source_sha': c.fence.context['commit'],
                       'namespace_uid': c.namespace_uid, 'creator_receipt_sha256': writer_cut.digest(creator),
                       'principal': principal, 'media': media, 'objects': seven}
            self.pending = True
            c.journal.state['steps'][STEP] = copy.deepcopy(pending)
            c.journal.save()
            current, actual = c.fence.ledger.read()
            if current != row or actual != rev:
                raise ValueError()
            current[LEDGER] = copy.deepcopy(pending)
            current['step'] = STEP + '-pending'
            c.fence.ledger.write(rev, current)
            self.record = copy.deepcopy(pending)
            directory = SOCKET_ROOT / c.suffix / 'realtime'
            self._manager = backup.private_directory(directory)
            self.dirfd = self._manager.__enter__()
            self.listener = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
            self.listener.bind(str(directory / 'custody.sock'))
            os.chmod('custody.sock', 0o600, dir_fd=self.dirfd, follow_symlinks=False)
            info = os.stat('custody.sock', dir_fd=self.dirfd, follow_symlinks=False)
            if not stat.S_ISSOCK(info.st_mode) or info.st_uid != os.geteuid():
                raise ValueError()
            self.inode = (info.st_dev, info.st_ino)
            self.listener.listen(1)
            os.fsync(self.dirfd)
            self._guard()
            if self._principal() != principal or c._ca() != media or self._seven() != seven:
                raise ValueError()
            row, rev = self._guard()
            ready = pending | {'status': 'ready', 'socket_dev': info.st_dev, 'socket_ino': info.st_ino}
            c.journal.state['steps'][STEP] = copy.deepcopy(ready)
            c.journal.save()
            current, actual = c.fence.ledger.read()
            if current != row or actual != rev:
                raise ValueError()
            current[LEDGER] = copy.deepcopy(ready)
            current['step'] = STEP + '-ready'
            c.fence.ledger.write(rev, current)
            self.record = copy.deepcopy(ready)
            self._guard()
            return copy.deepcopy(ready)
        except BaseException:
            if self.pending:
                self._uncertain()
            self.close()
            raise CapabilityError('Realtime P3 capability unavailable') from None

    def finish(self):
        try:
            c = self.creator_preflight
            if self.listener is None or self.record is None or self.record.get('status') != 'ready':
                raise ValueError()
            self._guard()
            info = os.stat('custody.sock', dir_fd=self.dirfd, follow_symlinks=False)
            if (info.st_dev, info.st_ino) != self.inode or not stat.S_ISSOCK(info.st_mode):
                raise ValueError()
            self.listener.settimeout(300)
            connection, _ = self.listener.accept()
            with connection:
                connection.settimeout(180)
                _, uid, _ = struct.unpack('3i', connection.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
                if uid != 0:
                    raise ValueError()
                self._guard()
                request = {'context': c.fence.context, 'namespace_uid': c.namespace_uid, 'lock': c.lock,
                           'nonce': self.record['nonce'], 'principal': self.record['principal'],
                           'media': self.record['media'], 'objects': self.record['objects']}
                raw = json.dumps(request, sort_keys=True, separators=(',', ':')).encode('ascii')
                if len(raw) > 65536 or connection.send(raw) != len(raw):
                    raise ValueError()
                reply, ancillary, flags, _ = connection.recvmsg(65537, 1)
                if ancillary or flags & (socket.MSG_TRUNC | socket.MSG_CTRUNC) or len(reply) > 65536:
                    raise ValueError()
            receipt = json.loads(reply, object_pairs_hook=peer.common.channel._unique)
            expected = {'schema': 'voice-dev-realtime-p3-capability-v1',
                        'operation_id': self.record['operation_id'], 'source_sha': self.record['source_sha'],
                        'namespace_uid': c.namespace_uid, 'nonce': self.record['nonce'],
                        'context_sha256': writer_cut.digest(c.fence.context), 'principal': self.record['principal'],
                        'media': self.record['media'], 'objects': self.record['objects'],
                        'scope': 'root-principal-custody-available-not-issued'}
            if receipt != expected or self._principal() != self.record['principal'] or c._ca() != self.record['media'] or self._seven() != self.record['objects']:
                raise ValueError()
            row, rev = self._guard()
            done = self.record | {'status': 'done', 'receipt_sha256': writer_cut.digest(receipt)}
            local = done | {'receipt': copy.deepcopy(receipt)}
            c.journal.state['steps'][STEP] = copy.deepcopy(local)
            c.journal.save()
            current, actual = c.fence.ledger.read()
            if current != row or actual != rev:
                raise ValueError()
            current[LEDGER] = copy.deepcopy(done)
            current['step'] = STEP + '-done'
            c.fence.ledger.write(rev, current)
            self.record = copy.deepcopy(done)
            self._guard()
            verify_current(self, self.token)
            return copy.deepcopy(receipt)
        except BaseException:
            if self.pending:
                self._uncertain()
            raise CapabilityError('Realtime P3 capability uncertain') from None
        finally:
            self.close()

    def _uncertain(self):
        try:
            self.creator_preflight.fence.uncertain('Realtime P3 capability uncertain')
        except BaseException:
            pass

    def close(self):
        if self.listener is not None:
            self.listener.close()
            self.listener = None
        if self.dirfd is not None:
            try:
                if self.inode is not None:
                    info = os.stat('custody.sock', dir_fd=self.dirfd, follow_symlinks=False)
                    if (info.st_dev, info.st_ino) != self.inode or not stat.S_ISSOCK(info.st_mode):
                        raise ValueError()
                    os.unlink('custody.sock', dir_fd=self.dirfd)
                    os.fsync(self.dirfd)
            finally:
                self._manager.__exit__(None, None, None)
                self.dirfd = None


def verify_current(capability, token):
    try:
        if type(capability) is not RealtimeP3Capability or token != capability.token:
            raise ValueError()
        c = capability.creator_preflight
        row, rev = capability._guard()
        local = copy.deepcopy(c.journal.state['steps'][STEP])
        if (capability.record is None or capability.record.get('status') != 'done' or
                capability._principal() != capability.record['principal'] or c._ca() != capability.record['media'] or
                capability._seven() != capability.record['objects']):
            raise ValueError()
        after, end = capability._guard()
        if (row != after or rev != end or c.journal.state['steps'][STEP] != local or
                writer_cut.digest(c.journal.state['steps'][c.step]['receipt']) != capability.record['creator_receipt_sha256']):
            raise ValueError()
        return copy.deepcopy(local['receipt'])
    except Exception:
        raise CapabilityError('Realtime P3 capability unavailable') from None


def exchange_root_request(capability, token, session, connection):
    """Send a fixed, freshly guarded post-Space issue snapshot to root."""
    try:
        if (type(capability) is not RealtimeP3Capability or type(session) is not p3.Session or
                connection.family != socket.AF_UNIX or connection.type & socket.SOCK_SEQPACKET != socket.SOCK_SEQPACKET):
            raise ValueError()
        _, uid, _ = struct.unpack('3i', connection.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))
        if uid != 0:
            raise ValueError()
        c = capability.creator_preflight
        receipt = verify_current(capability, token)
        if (session.fence.context != c.fence.context or session.namespace_uid != c.namespace_uid or
                session.lock != c.lock or session.handoff is None or session.handoff.get('status') != 'ready' or
                session.plan['issuer_source_sha'] != source.REVIEWED_ISSUER_SHA):
            raise ValueError()
        source.verify(session.plan['source_repo'], session.plan['issuer_source_sha'], c.fence.context['commit'])
        session._guard()
        session._check_roots()
        if session.fence.ledger.read() != c.fence.ledger.read():
            raise ValueError()
        refs = {}
        for key, label in zip(p3.PUBLIC, ('principal', 'media', 'space')):
            value = session.roots[key]
            refs[label] = {'uid': value['uid'], 'resourceVersion': value['resourceVersion'],
                           'der_sha256': hashlib.sha256(p3.packet.crypto.der(value['certificate'])).hexdigest()}
        if refs['principal'] != receipt['principal'] or refs['media'] != receipt['media']:
            raise ValueError()
        p3.packet.crypto.space_certificate(session.roots[p3.PUBLIC[2]]['certificate'],
                                           session.roots[p3.PUBLIC[1]]['certificate'])
        current = capability._seven()
        if current != receipt['objects'] or any(item != {'state': 'absent'} for item in current.values()):
            raise ValueError()
        request = {'context': copy.deepcopy(c.fence.context), 'namespace_uid': c.namespace_uid,
                   'lock': copy.deepcopy(c.lock), 'principal': refs['principal'], 'media': refs['media'],
                   'space': refs['space'], 'objects': current, 'capability_sha256': writer_cut.digest(receipt),
                   'issuer_source_sha': session.plan['issuer_source_sha']}
        raw = json.dumps(request, sort_keys=True, separators=(',', ':')).encode('ascii')
        if len(raw) > 65536 or connection.send(raw) != len(raw):
            raise ValueError()
        session._guard()
        session._check_roots()
        if verify_current(capability, token) != receipt or capability._seven() != current:
            raise ValueError()
        if connection.send(b'PREFLIGHT-READY\n') != len(b'PREFLIGHT-READY\n'):
            raise ValueError()
        return copy.deepcopy(request)
    except Exception:
        raise CapabilityError('Realtime P3 issue exchange unavailable') from None
