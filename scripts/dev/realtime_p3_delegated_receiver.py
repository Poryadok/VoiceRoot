"""Fixed delegated P3 packet exchange; no Kubernetes effects here."""
import array
import fcntl
import hashlib
import json
import os
import re
import socket
import stat
import struct
import threading
import time

import dev_peer_issuer_preflight as peer
import realtime_p3_create_only as creation
import realtime_p3_create_only_crypto as crypto
import realtime_p3_create_only_packet as packet
import realtime_p3_create_only_socket as channel
import realtime_p3_create_only_transport as transport
import realtime_p3_root_export as root_export
import space_native_tool_identity as native
import writer_cut

FRAME=b'VOICE-DEV-P3-PACKET\n'
ACK=b'VOICE-DEV-P3-ACCEPTED\n'


def _socket(session):
    info=os.stat('packet.sock',dir_fd=session.dirfd,follow_symlinks=False)
    directory=os.fstat(session.dirfd); visible=os.stat(creation.SOCKET_PARENT/session.suffix,follow_symlinks=False)
    if (not stat.S_ISSOCK(info.st_mode) or (info.st_dev,info.st_ino)!=session.inode or info.st_uid!=os.geteuid()
            or stat.S_IMODE(info.st_mode)!=0o600 or (directory.st_dev,directory.st_ino)!=(visible.st_dev,visible.st_ino)
            or session.handoff['pid']!=os.getpid() or session.handoff['start_ticks']!=creation._ticks()): raise ValueError()


def _current(session,custody,capability,token):
    import dev_realtime_p3_issuer_capability as authority
    authority.verify_current(capability,token)
    session._guard(); _socket(session)
    if native.SpaceNativeToolPhase._custody(session.journal)!=custody: raise ValueError()
    session._check_roots()
    for identity in sorted(packet.MEMBERS):
        session._guard()
        if transport.get(*identity) is not None: raise ValueError()
    session._guard()
    if native.SpaceNativeToolPhase._custody(session.journal)!=custody: raise ValueError()


def _request_matches(session,request):
    if (not isinstance(request,dict) or set(request)!={'context','namespace_uid','lock','principal','media','space','objects','capability_sha256','issuer_source_sha'}
            or request['context']!=session.fence.context or request['namespace_uid']!=session.namespace_uid
            or request['lock']!=session.lock or request['issuer_source_sha']!=session.plan['issuer_source_sha']
            or not isinstance(request['capability_sha256'],str) or not re.fullmatch(r'[a-f0-9]{64}',request['capability_sha256'])
            or request['objects']!={kind.lower()+'/'+name:{'state':'absent'} for kind,name in packet.MEMBERS}): raise ValueError()
    for field,identity in zip(('principal','media','space'),creation.PUBLIC):
        current=session.roots[identity]
        expected={'uid':current['uid'],'resourceVersion':current['resourceVersion'],
                  'der_sha256':hashlib.sha256(crypto.der(current['certificate'])).hexdigest()}
        if request[field]!=expected: raise ValueError()
def receive(session,capability,token):
    """Return a validated private packet only after the root service terminal OK."""
    import dev_realtime_p3_issuer_capability as authority
    import dev_realtime_p3_issuer_client as client
    fds=[]; connection=None; worker=None; handle=None
    try:
        if (type(session) is not creation.Session or type(capability) is not authority.RealtimeP3Capability
                or not isinstance(token,str) or not token or session.listener is None
                or session.handoff is None or session.handoff.get('status')!='ready'): raise ValueError()
        preflight=capability.creator_preflight; cut=preflight.cut
        if (type(preflight) is not peer.PeerIssuerPreflightPhase or type(cut) is not writer_cut.WriterCut
                or capability.token!=token or preflight.fence.context!=session.fence.context
                or preflight.lock!=session.lock or preflight.namespace_uid!=session.namespace_uid
                or cut.fence.context!=session.fence.context or cut.lock!=session.lock
                or cut.namespace_uid!=session.namespace_uid or session._delegated_cut is not None): raise ValueError()
        authority.verify_current(capability,token)
        if session.fence.ledger.read()!=preflight.fence.ledger.read() or session.fence.ledger.read()!=cut.fence.ledger.read(): raise ValueError()
        session._delegated_cut=cut; session._delegated_refs=(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger)
        custody=native.SpaceNativeToolPhase._custody(session.journal)
        _current(session,custody,capability,token)
        result=[]; handle=client.new_issue_trigger(capability,token,session)
        def invoke():
            try:handle.run(); result.append(True)
            except BaseException:result.append(False)
        worker=threading.Thread(target=invoke); worker.start(); deadline=time.monotonic()+300
        session.listener.settimeout(300); connection,_=session.listener.accept(); connection.settimeout(30)
        pid,uid,_=struct.unpack('3i',connection.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
        if uid!=0 or pid<=0: raise ValueError()
        start=root_export._start_ticks(pid); previous=None
        for _ in range(2):
            data,ancillary,flags,_=connection.recvmsg(64,socket.CMSG_SPACE(8*array.array('i').itemsize),socket.MSG_CMSG_CLOEXEC)
            for level,typ,payload in ancillary:
                if level==socket.SOL_SOCKET and typ==socket.SCM_RIGHTS:
                    numbers=array.array('i'); numbers.frombytes(payload[:len(payload)-len(payload)%numbers.itemsize]); fds.extend(numbers)
            if data!=b'PREFLIGHT-REQUEST\n' or ancillary or flags & (socket.MSG_TRUNC|socket.MSG_CTRUNC): raise ValueError()
            _current(session,custody,capability,token)
            observed=authority.exchange_root_request(capability,token,session,connection)
            _request_matches(session,observed)
            if previous is not None and observed!=previous: raise ValueError()
            previous=observed
            if root_export._start_ticks(pid)!=start: raise ValueError()
        data,ancillary,flags,_=connection.recvmsg(len(FRAME)+1,socket.CMSG_SPACE(8*array.array('i').itemsize),socket.MSG_CMSG_CLOEXEC)
        invalid=False
        for level,typ,payload in ancillary:
            if level!=socket.SOL_SOCKET or typ!=socket.SCM_RIGHTS: invalid=True; continue
            numbers=array.array('i')
            if len(payload)%numbers.itemsize: invalid=True
            numbers.frombytes(payload[:len(payload)-len(payload)%numbers.itemsize]); fds.extend(numbers)
        if (invalid or data!=FRAME or len(ancillary)!=1 or len(fds)!=1 or flags & (socket.MSG_TRUNC|socket.MSG_CTRUNC)
                or root_export._start_ticks(pid)!=start): raise ValueError()
        fd=fds[0]; info=os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_nlink!=0 or stat.S_IMODE(info.st_mode)!=0o600
                or not 0<info.st_size<=channel.MAX_PACKET or fcntl.fcntl(fd,fcntl.F_GET_SEALS)&channel.REQUIRED_SEALS!=channel.REQUIRED_SEALS): raise ValueError()
        raw=bytearray(); offset=0
        while offset<info.st_size:
            chunk=os.pread(fd,min(65536,info.st_size-offset),offset)
            if not chunk: raise ValueError()
            raw.extend(chunk); offset+=len(chunk)
        after=os.fstat(fd)
        if (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)!=(after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns): raise ValueError()
        envelope=json.loads(raw,object_pairs_hook=channel._unique); _current(session,custody,capability,token)
        principal=session.roots[creation.PUBLIC[0]]['certificate']; media=session.roots[creation.PUBLIC[1]]['certificate']
        space=session.roots[creation.PUBLIC[2]]['certificate']
        packet.validate(envelope,session.fence.context['operation_id'],session.plan['issuer_source_sha'],principal,media,space)
        _current(session,custody,capability,token)
        if root_export._start_ticks(pid)!=start or connection.send(ACK)!=len(ACK): raise ValueError()
        connection.close(); connection=None
        worker.join(timeout=max(0,deadline-time.monotonic()))
        if worker.is_alive() or result!=[True]: raise ValueError()
        _current(session,custody,capability,token)
        return envelope
    except BaseException:
        raise creation.CreateError('delegated P3 handoff unavailable') from None
    finally:
        if connection is not None:connection.close()
        if worker is not None and worker.is_alive():
            try:handle.cancel()
            except BaseException:pass
            while worker.is_alive():worker.join(timeout=5)
        for fd in fds:
            try:os.close(fd)
            except OSError:pass
