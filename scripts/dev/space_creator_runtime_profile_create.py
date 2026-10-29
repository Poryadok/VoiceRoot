"""Fenced missing-only Creator identity handoff and Secret creation."""
import array
import base64
import copy
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import socket
import stat
import struct

import canonical_application_package as canonical
import frontend_runtime_executor as transport
import full_cohort_schema_metadata as schema
import media_operation_guard
import media_operation_journal
import realtime_p3_create_only as common
import realtime_p3_create_only_socket as channel
import realtime_p3_create_only_transport as private
import realtime_p3_root_export as root_export
import resource_fencing
import space_creator_runtime_profile_issuer_packet as packet
import space_creator_runtime_profile_issuer_source as source
import space_creator_runtime_profile_material as material
import space_p3_issuer_preflight as preflight
import space_p3_issuer_preflight_metadata_root as metadata
import space_native_tool_identity as native
import writer_cut

ROOT=Path('/home/pmd/voice-space-creator/operations')
STEP='space-creator-material-create'
LEDGER='space_creator_material_create'
EFFECTS='space_creator_material_effects'
FRAME=b'SPACE-CREATOR-PROFILE\n'
ACK=b'SPACE-CREATOR-ACCEPTED\n'


class CreateError(RuntimeError): pass


def _get(name):
    if name not in metadata.MEMBERS: raise ValueError()
    raw=private._run(['get','secret',name,'--ignore-not-found=true','-o','json'])
    return None if not raw else json.loads(raw)


def _create(obj,dry):
    if type(dry) is not bool or obj.get('kind')!='Secret' or obj.get('metadata',{}).get('name') not in metadata.MEMBERS: raise ValueError()
    args=['create','-f','-','-o','json']
    if dry: args.insert(1,'--dry-run=server')
    return json.loads(private._run(args,json.dumps(obj,separators=(',',':')).encode('ascii')))


def _receive(listener,phase):
    """Accept one root packet only after its exact protected crypto checks."""
    fds=[]
    try:
        listener.settimeout(300); connection,_=listener.accept()
        with connection:
            connection.settimeout(30)
            pid,uid,_=struct.unpack('3i',connection.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
            if uid!=0 or pid<=0: raise ValueError()
            start=root_export._start_ticks(pid)
            data,ancillary,flags,_=connection.recvmsg(len(FRAME)+1,socket.CMSG_SPACE(8*array.array('i').itemsize),socket.MSG_CMSG_CLOEXEC)
            invalid=False
            for level,kind,payload in ancillary:
                numbers=array.array('i')
                if level!=socket.SOL_SOCKET or kind!=socket.SCM_RIGHTS: invalid=True; continue
                if len(payload)%numbers.itemsize: invalid=True
                numbers.frombytes(payload[:len(payload)-len(payload)%numbers.itemsize]); fds.extend(numbers)
            if invalid or flags & (socket.MSG_TRUNC|socket.MSG_CTRUNC) or data!=FRAME or len(ancillary)!=1 or len(fds)!=1 or root_export._start_ticks(pid)!=start: raise ValueError()
            fd=fds[0]; info=os.fstat(fd)
            if (not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_nlink!=0 or stat.S_IMODE(info.st_mode)!=0o600 or
                    not 0<info.st_size<=channel.MAX_PACKET or fcntl.fcntl(fd,fcntl.F_GET_SEALS)&channel.REQUIRED_SEALS!=channel.REQUIRED_SEALS): raise ValueError()
            raw=bytearray(); offset=0
            while offset<info.st_size:
                value=os.pread(fd,min(65536,info.st_size-offset),offset)
                if not value: raise ValueError()
                raw.extend(value); offset+=len(value)
            after=os.fstat(fd)
            if (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)!=(after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns): raise ValueError()
            envelope=json.loads(raw,object_pairs_hook=channel._unique); phase._check_current()
            phase.objects=packet.validate(envelope,phase.fence.context['operation_id'],phase.fence.context['commit'],phase.namespace_uid,
                                          writer_cut.digest(phase.initial),phase.missing,phase.media)
            phase._distinct_keys(); phase._check_current()
            if root_export._start_ticks(pid)!=start or connection.send(ACK)!=len(ACK): raise ValueError()
            return envelope
    finally:
        for fd in fds:
            try:os.close(fd)
            except OSError:pass


def _directory(suffix):
    """Create the fixed owned operation directory exactly once, without adoption."""
    if os.geteuid()!=1000 or not re.fullmatch(r'[a-f0-9]{32}',suffix): raise ValueError()
    fd=os.open('/home/pmd',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:
        info=os.fstat(fd)
        if info.st_uid!=1000 or info.st_mode & 0o022: raise ValueError()
        for name in ('voice-space-creator','operations',suffix):
            try:os.mkdir(name,0o700,dir_fd=fd); os.fsync(fd)
            except FileExistsError:
                if name==suffix: raise
            new=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            info=os.fstat(new)
            if info.st_uid!=1000 or stat.S_IMODE(info.st_mode)!=0o700: os.close(new); raise ValueError()
            os.close(fd); fd=new
        return fd
    except BaseException:
        os.close(fd); raise


def _values(obj,name):
    if (not isinstance(obj,dict) or obj.get('apiVersion')!='v1' or obj.get('kind')!='Secret' or obj.get('type')!='Opaque' or
            obj.get('metadata',{}).get('name')!=name or obj['metadata'].get('namespace')!='voice-dev' or
            obj['metadata'].get('deletionTimestamp') or not isinstance(obj.get('data'),dict) or set(obj['data'])!=metadata.MEMBERS[name]): raise ValueError()
    values={key:base64.b64decode(value,validate=True) for key,value in obj['data'].items()}
    if any(not value or len(value)>131072 for value in values.values()): raise ValueError()
    if name=='voice-gateway-principal-signing':
        for key in ('current.pem','next.pem'):material._pem(values[key],b'PRIVATE KEY')
    else:
        material._pem(values['tls.crt'],b'CERTIFICATE'); material._pem(values['tls.key'],b'PRIVATE KEY')
    return values


def _closed(actual,desired,require_identity=True):
    if (not isinstance(actual,dict) or set(actual)-{'apiVersion','kind','metadata','type','data'} or
            not isinstance(actual.get('metadata'),dict) or
            set(actual['metadata'])-{'name','namespace','uid','resourceVersion','creationTimestamp','managedFields'} or
            not common._same_object(actual,desired,require_identity)): return False
    return True


class Session:
    def __init__(self,journal,fence,client,lock,namespace_uid,package,issuer_preflight,source_root,token):
        import full_cohort_package_phase
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
                type(client) is not transport.KubectlTransport or client.journal is not journal or fence.ledger.client is not client or
                type(package) is not full_cohort_package_phase.PackagePhase or type(issuer_preflight) is not preflight.IssuerPreflightPhase or
                issuer_preflight.package is not package or any(x.fence.context!=fence.context or x.lock!=lock or x.namespace_uid!=namespace_uid for x in (package,issuer_preflight)) or
                Path(source_root).resolve()!=canonical.ROOT.resolve() or not isinstance(token,str) or not token or
                journal.intent.get('resource_context')!=fence.context or journal.intent.get('namespace_uid')!=namespace_uid): raise CreateError('Creator identity unavailable')
        match=re.fullmatch(r'media-c1-([a-f0-9]{32})',fence.context.get('operation_id',''))
        if not match: raise CreateError('Creator identity unavailable')
        self.journal,self.fence,self.client,self.package,self.preflight=journal,fence,client,package,issuer_preflight
        self.lock,self.namespace_uid,self.source_root,self.token=copy.deepcopy(lock),namespace_uid,Path(source_root).resolve(),token
        self.suffix=match[1]; self.intent=copy.deepcopy(journal.intent); self.record=None; self.effects={}; self.pending=False
        self.listener=self.dirfd=self.inode=self.initial=self.media=self.objects=self.proof=self.envelope=None
        self.completed=False; self.exchanged=False; self.poisoned=False
        self.refs=(journal,fence,client,fence.ledger,package,issuer_preflight); self.custody=native.SpaceNativeToolPhase._custody(journal)
        self.identity=(copy.deepcopy(fence.context),copy.deepcopy(lock),namespace_uid,self.source_root,token,self.suffix)

    def _local(self):
        if (self.poisoned or self.refs!=(self.journal,self.fence,self.client,self.fence.ledger,self.package,self.preflight) or
                self.client.journal is not self.journal or self.fence.ledger.client is not self.client or self.journal.poisoned or
                (self.fence.context,self.lock,self.namespace_uid,self.source_root,self.token,self.suffix)!=self.identity or
                self.preflight.package is not self.package or self.preflight.fence.context!=self.fence.context or
                self.preflight.lock!=self.lock or self.preflight.namespace_uid!=self.namespace_uid or
                self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                native.SpaceNativeToolPhase._custody(self.journal)!=self.custody): raise ValueError()
        self.preflight._local()
        if self.record is not None and (self.journal.state['steps'].get(STEP)!=self.record or self.record.get('missing')!=self.missing or
                self.record.get('issuer_source_sha')!=source.REVIEWED_SOURCE): raise ValueError()
        local={k.removeprefix('space-creator-secret-'):v for k,v in self.journal.state['steps'].items() if k.startswith('space-creator-secret-')}
        if local!=self.effects: raise ValueError()

    def _guard(self):
        try:
            self._local(); media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
            self.preflight._authority(); source.verify(self.source_root,self.fence.context['commit'])
            row,rev=self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status')!='active' or self.preflight.fence.ledger.read()!=(row,rev) or
                    (self.record is not None and row.get(LEDGER)!=self.record) or row.get(EFFECTS,{})!=self.effects or
                    (self.initial is not None and (row.get(preflight.LEDGER)!=self.preflight.record or
                     self.preflight.record.get('receipt_sha256')!=writer_cut.digest(self.initial)))): raise ValueError()
            self._local(); return row,rev
        except BaseException: raise schema.GuardLost('Creator guard unavailable') from None

    def _check_current(self):
        self._guard()
        try:ca=self.preflight._ca(); actual=self.preflight._creator()
        except BaseException:raise schema.GuardLost('Creator preflight observation unavailable') from None
        if ca!=self.initial['ca']: raise ValueError()
        expected=copy.deepcopy(self.initial['creator_secrets'])
        for name,effect in self.effects.items():
            if effect['status']=='done': expected[name]={'state':'present','uid':effect['uid'],'resourceVersion':effect['resourceVersion'],'keys':sorted(metadata.MEMBERS[name])}
        if actual!=expected: raise ValueError()
        for name,obj in (self.objects or {}).items():
            if name in self.effects and self.effects[name]['status']=='done':
                current=_get(name)
                if not _closed(current,obj) or any(current['metadata'].get(k)!=self.effects[name][k] for k in ('uid','resourceVersion')): raise ValueError()
        self._guard()

    def _write(self,record=None,effects=None):
        row,rev=self._guard()
        if record is not None:self.journal.state['steps'][STEP]=copy.deepcopy(record)
        if effects is not None:
            for name,value in effects.items():self.journal.state['steps']['space-creator-secret-'+name]=copy.deepcopy(value)
        self.journal.save()
        try:
            media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
            self.preflight._authority(); source.verify(self.source_root,self.fence.context['commit'])
            if (native.SpaceNativeToolPhase._custody(self.journal)!=self.custody or self.journal.intent!=self.intent or
                    self.journal.state.get('intent')!=self.intent): raise ValueError()
        except BaseException:raise schema.GuardLost('Creator guard unavailable') from None
        now,version=self.fence.ledger.read()
        if now!=row or version!=rev: raise ValueError()
        if record is not None:now[LEDGER]=copy.deepcopy(record)
        if effects is not None:now[EFFECTS]=copy.deepcopy(effects)
        self.fence.ledger.write(version,now)
        if record is not None:self.record=copy.deepcopy(record)
        if effects is not None:self.effects=copy.deepcopy(effects)
        self._guard()

    def _uncertain(self):
        self.poisoned=True
        try:self.fence.uncertain('Creator Secret creation uncertain')
        except BaseException:pass

    def _drained(self):
        try:self.preflight.cut.verify_drained()
        except BaseException:raise schema.GuardLost('Creator writer drain unavailable') from None

    def _socket(self):
        if self.listener is None or self.dirfd is None or self.inode is None: raise ValueError()
        info=os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
        directory=os.fstat(self.dirfd); visible=os.stat(ROOT/self.suffix,follow_symlinks=False)
        if ((info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode) or info.st_uid!=os.geteuid() or stat.S_IMODE(info.st_mode)!=0o600 or
                (directory.st_dev,directory.st_ino)!=(visible.st_dev,visible.st_ino) or not stat.S_ISDIR(visible.st_mode) or
                directory.st_uid!=os.geteuid() or stat.S_IMODE(directory.st_mode)!=0o700 or
                os.getpid()!=self.record['pid'] or common._ticks()!=self.record['start_ticks']): raise ValueError()

    def _distinct_keys(self):
        keys=[]
        for name in sorted(metadata.MEMBERS):
            obj=self.objects[name] if name in self.objects else _get(name)
            values=_values(obj,name)
            for key in (('current.pem','next.pem') if name=='voice-gateway-principal-signing' else ('tls.key',)):
                public=packet._openssl(['pkey','-in','{0}','-pubout'],[values[key]])
                keys.append(hashlib.sha256(public).digest())
        if len(keys)!=len(set(keys)): raise ValueError()

    def prepare(self):
        try:
            self._guard(); self.initial=preflight.verify_current(self.preflight,self.token)
            source.verify(self.source_root,self.fence.context['commit']); row,_=self._guard()
            if LEDGER in row or EFFECTS in row or STEP in self.journal.state['steps']: raise ValueError()
            missing=sorted(name for name,item in self.initial['creator_secrets'].items() if item=={'state':'absent'})
            if not missing: raise ValueError()
            self.missing=missing
            self.media=preflight.public._value(preflight.public.common.linux_private_bytes,self._guard,'secret','voice-media-ca','ca.crt')
            if hashlib.sha256(preflight.crypto.der(self.media)).hexdigest()!=self.initial['ca']['der_sha256']: raise ValueError()
            for name,item in self.initial['creator_secrets'].items():
                if item['state']=='present':
                    obj=_get(name); values=_values(obj,name)
                    if any(obj['metadata'].get(k)!=item[k] for k in ('uid','resourceVersion')): raise ValueError()
                    if name=='voice-gateway-principal-signing':
                        if values['active-kid'] not in (b'current',b'next'): raise ValueError()
                        packet._jwks(values)
                    else:packet._certificate(values,self.media,*packet.FAMILIES[name])
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit'],'issuer_source_sha':source.REVIEWED_SOURCE,
                    'preflight_sha256':writer_cut.digest(self.initial),'missing':missing,'pid':os.getpid(),'start_ticks':common._ticks()}
            self.pending=True; self._write(record=marker)
            self.dirfd=_directory(self.suffix); self.listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            self.listener.bind(str(ROOT/self.suffix/'packet.sock')); os.chmod('packet.sock',0o600,dir_fd=self.dirfd,follow_symlinks=False)
            info=os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
            if not stat.S_ISSOCK(info.st_mode) or info.st_uid!=os.geteuid() or stat.S_IMODE(info.st_mode)!=0o600: raise ValueError()
            self.inode=(info.st_dev,info.st_ino); self.listener.listen(1); os.fsync(self.dirfd); self._check_current()
            ready=marker|{'status':'ready','socket_dev':info.st_dev,'socket_ino':info.st_ino}; self._write(record=ready)
            return copy.deepcopy(ready)
        except BaseException as error:
            if self.pending:self._uncertain()
            self.close()
            if isinstance(error,schema.GuardLost):raise
            raise CreateError('Creator handoff unavailable; reconcile if reserved') from None

    def serve_preflight(self):
        try:
            if self.exchanged or self.listener is None or self.record.get('status')!='ready': raise ValueError()
            self._socket(); self._check_current(); self.listener.settimeout(300); connection,_=self.listener.accept()
            with connection:
                connection.settimeout(120)
                pid,uid,_=struct.unpack('3i',connection.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
                if uid!=0 or pid<=0: raise ValueError()
                start=root_export._start_ticks(pid)
                for _ in range(2):
                    data,ancillary,flags,_=connection.recvmsg(64,socket.CMSG_SPACE(8*array.array('i').itemsize),socket.MSG_CMSG_CLOEXEC)
                    for level,kind,payload in ancillary:
                        if level==socket.SOL_SOCKET and kind==socket.SCM_RIGHTS:
                            numbers=array.array('i'); numbers.frombytes(payload[:len(payload)-len(payload)%numbers.itemsize])
                            for fd in numbers:os.close(fd)
                    if data!=b'PREFLIGHT-REQUEST\n' or ancillary or flags & (socket.MSG_TRUNC|socket.MSG_CTRUNC): raise ValueError()
                    try:exchanged=preflight.exchange_root_request(self.preflight,self.token,connection)
                    except BaseException:raise schema.GuardLost('Creator preflight authority unavailable') from None
                    if exchanged!=self.initial: raise ValueError()
                    if root_export._start_ticks(pid)!=start: raise ValueError()
            self._socket(); self._check_current(); self.exchanged=True
        except BaseException as error:
            self._uncertain()
            if isinstance(error,schema.GuardLost):raise
            raise CreateError('Creator preflight exchange uncertain') from None

    def finish(self):
        try:
            if not self.exchanged or self.listener is None or self.record.get('status')!='ready': raise ValueError()
            self._socket(); self._check_current(); self.envelope=_receive(self.listener,self); self._check_current()
            for name in self.missing:
                self._check_current(); self._drained()
                if _get(name) is not None or not _closed(_create(self.objects[name],True),self.objects[name],False): raise ValueError()
            for name in self.missing:
                self._check_current(); self._drained()
                if _get(name) is not None: raise ValueError()
                pending={'status':'pending','nonce':secrets.token_hex(16),'kind':'Secret','name':name,'keys':sorted(metadata.MEMBERS[name])}
                self._write(effects=self.effects|{name:pending}); self._check_current(); self._drained()
                response=_create(self.objects[name],False); self._drained(); current=_get(name)
                if (not _closed(response,self.objects[name]) or not _closed(current,self.objects[name]) or
                        any(response['metadata'].get(k)!=current['metadata'].get(k) for k in ('uid','resourceVersion'))): raise ValueError()
                self._write(effects=self.effects|{name:pending|{'status':'done','uid':current['metadata']['uid'],'resourceVersion':current['metadata']['resourceVersion']}})
                self._check_current()
            receipt={'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                     'issuer_source_sha':source.REVIEWED_SOURCE,'namespace_uid':self.namespace_uid,
                     'preflight_sha256':writer_cut.digest(self.initial),'objects':copy.deepcopy(self.effects),
                     'scope':'creator-missing-only-secret-create-no-runtime-readiness'}
            self.proof=copy.deepcopy(receipt); done=self.record|{'status':'done','receipt':copy.deepcopy(receipt)}
            effects=copy.deepcopy(self.effects); self._write(record=done); self._check_current(); schema._fence(self._local)
            if (self.proof!=receipt or self.record!=done or self.effects!=effects or
                    self.journal.state['steps'].get(STEP)!=done): raise schema.GuardLost('Creator terminal record changed')
            self.completed=True; return copy.deepcopy(receipt)
        except BaseException as error:
            if self.pending and not self.completed:self._uncertain()
            if isinstance(error,schema.GuardLost):raise
            raise CreateError('Creator creation uncertain; retain lock and reconcile') from None
        finally:self.close()

    def close(self):
        if self.listener is not None:self.listener.close(); self.listener=None
        if self.dirfd is not None:
            try:
                if self.inode is not None:
                    info=os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
                    if (info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode): raise ValueError()
                    os.unlink('packet.sock',dir_fd=self.dirfd); os.fsync(self.dirfd)
            except Exception:
                if self.pending:self._uncertain()
                raise CreateError('Creator socket cleanup uncertain') from None
            finally:os.close(self.dirfd); self.dirfd=None


def verify_completed(phase,token):
    try:
        if type(phase) is not Session or token!=phase.token or not phase.completed or phase.record.get('status')!='done' or phase.record.get('receipt')!=phase.proof: raise ValueError()
        phase._check_current()
        objects=packet.validate(phase.envelope,phase.fence.context['operation_id'],phase.fence.context['commit'],phase.namespace_uid,
                                writer_cut.digest(phase.initial),phase.missing,phase.media)
        if objects!=phase.objects: raise ValueError()
        phase._distinct_keys(); phase._check_current(); row,_=phase._guard(); phase._local()
        if (row.get(LEDGER)!=phase.record or row.get(EFFECTS)!=phase.effects or phase.journal.state['steps'].get(STEP)!=phase.record or
                phase.record.get('receipt')!=phase.proof): raise ValueError()
        return copy.deepcopy(phase.proof)
    except BaseException as error:
        if type(phase) is Session and phase.pending:phase._uncertain()
        if isinstance(error,schema.GuardLost):raise
        raise CreateError('Creator creation proof unavailable') from None
