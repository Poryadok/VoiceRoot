"""Fenced missing-only Role peer identity handoff and Secret creation."""
import array
import base64
import copy
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import select
import secrets
import socket
import stat
import struct
import threading
import time

import canonical_application_package as canonical
import frontend_runtime_executor as transport
import full_cohort_package_phase
import full_cohort_schema_metadata as schema
import media_operation_guard
import media_operation_journal
import realtime_p3_create_only as common
import realtime_p3_create_only_socket as channel
import realtime_p3_create_only_transport as private
import realtime_p3_root_export as root_export
import resource_fencing
import dev_peer_issuer_packet as packet
import dev_peer_issuer_source as source
import dev_peer_issuer_preflight as preflight
import dev_peer_issuer_metadata as metadata
import space_role_peer_material as role_material
import space_creator_runtime_profile_material as material
import space_native_tool_identity as native
import writer_cut

ROOT=Path('/home/pmd/voice-dev-peer-issuer/operations')
STEP='space-role-peer-material-create'
LEDGER='space_role_peer_material_create'
EFFECTS='space_role_peer_material_effects'
FRAME=b'VOICE-DEV-PEER-PACKET\n'
ACK=b'VOICE-DEV-PEER-ACCEPTED\n'
NAMES=frozenset(('voice-role-media-client-tls','voice-role-principal-signing'))


class CreateError(RuntimeError): pass


def _get(name):
    if name not in NAMES: raise ValueError()
    raw=private._run(['get','secret',name,'--ignore-not-found=true','-o','json'])
    return None if not raw else json.loads(raw)


def _create(obj,dry):
    if type(dry) is not bool or obj.get('kind')!='Secret' or obj.get('metadata',{}).get('name') not in NAMES: raise ValueError()
    args=['create','-f','-','-o','json']
    if dry: args.insert(1,'--dry-run=server')
    return json.loads(private._run(args,json.dumps(obj,separators=(',',':')).encode('ascii')))


def _receive(listener,phase):
    """Accept one root packet only after its exact protected crypto checks."""
    fds=[]
    try:
        connection=getattr(phase,'connection',None)
        if connection is None:
            listener.settimeout(300); connection,_=listener.accept()
        else:
            phase.connection=None
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
            phase.objects=packet.validate(envelope,'role',phase.fence.context['operation_id'],phase.fence.context['commit'],phase.namespace_uid,
                                          writer_cut.digest(phase.initial),phase.missing,phase.media)
            for name,obj in phase.objects.items(): role_material.qualify(name,obj,phase.media)
            phase._distinct_keys(); phase._check_current()
            if root_export._start_ticks(pid)!=start or connection.send(ACK)!=len(ACK): raise ValueError()
            return envelope
    finally:
        for fd in fds:
            try:os.close(fd)
            except OSError:pass


def _directory(suffix):
    """Open the fixed private operation directory; the role socket is create-only."""
    if os.geteuid()!=1000 or not re.fullmatch(r'[a-f0-9]{32}',suffix): raise ValueError()
    fd=os.open('/home/pmd',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:
        info=os.fstat(fd)
        if info.st_uid!=1000 or info.st_mode & 0o022: raise ValueError()
        for name in ('voice-dev-peer-issuer','operations',suffix):
            try:os.mkdir(name,0o700,dir_fd=fd); os.fsync(fd)
            except FileExistsError:
                pass
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
    if name=='voice-role-principal-signing':
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
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
                type(client) is not transport.KubectlTransport or client.journal is not journal or fence.ledger.client is not client or
                type(package) is not full_cohort_package_phase.PackagePhase or type(issuer_preflight) is not preflight.PeerIssuerPreflightPhase or
                issuer_preflight.profile!='role' or set(metadata.PROFILES['role'])!=NAMES or issuer_preflight.package is not package or any(x.fence.context!=fence.context or x.lock!=lock or x.namespace_uid!=namespace_uid for x in (package,issuer_preflight)) or
                Path(source_root).resolve()!=canonical.ROOT.resolve() or not isinstance(token,str) or not token or
                journal.intent.get('resource_context')!=fence.context or journal.intent.get('namespace_uid')!=namespace_uid): raise CreateError('Role peer identity unavailable')
        match=re.fullmatch(r'media-c1-([a-f0-9]{32})',fence.context.get('operation_id',''))
        if not match: raise CreateError('Role peer identity unavailable')
        self.journal,self.fence,self.client,self.package,self.preflight=journal,fence,client,package,issuer_preflight
        self.lock,self.namespace_uid,self.source_root,self.token=copy.deepcopy(lock),namespace_uid,Path(source_root).resolve(),token
        self.suffix=match[1]; self.intent=copy.deepcopy(journal.intent); self.record=None; self.effects={}; self.pending=False
        self.listener=self.connection=self.dirfd=self.inode=self.initial=self.media=self.objects=self.proof=self.envelope=None
        self.completed=False; self.exchanged=False; self.poisoned=False
        self.issue_thread=self.issue_done=self.issue_result=self.issue_control=None
        self.refs=(journal,fence,client,fence.ledger,package,issuer_preflight); self.custody=native.SpaceNativeToolPhase._custody(journal)
        self.identity=(copy.deepcopy(fence.context),copy.deepcopy(lock),namespace_uid,self.source_root,token,self.suffix)

    def _local(self):
        if (self.poisoned or self.refs!=(self.journal,self.fence,self.client,self.fence.ledger,self.package,self.preflight) or
                self.client.journal is not self.journal or self.fence.ledger.client is not self.client or self.journal.poisoned or
                (self.fence.context,self.lock,self.namespace_uid,self.source_root,self.token,self.suffix)!=self.identity or
                self.preflight.profile!='role' or self.preflight.package is not self.package or self.preflight.fence.context!=self.fence.context or
                self.preflight.lock!=self.lock or self.preflight.namespace_uid!=self.namespace_uid or
                self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                native.SpaceNativeToolPhase._custody(self.journal)!=self.custody): raise ValueError()
        self.preflight._local()
        if self.record is not None and (self.journal.state['steps'].get(STEP)!=self.record or self.record.get('missing')!=self.missing or
                self.record.get('issuer_source_sha')!=source.REVIEWED_SOURCE): raise ValueError()
        local={k.removeprefix('space-role-peer-secret-'):v for k,v in self.journal.state['steps'].items() if k.startswith('space-role-peer-secret-')}
        if local!=self.effects: raise ValueError()

    def _guard(self):
        try:
            self._local(); media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
            self.preflight._authority()
            package_local=copy.deepcopy(self.package.journal.state['steps'].get(full_cohort_package_phase.STEP))
            if not isinstance(package_local,dict) or package_local.get('status')!='done' or not isinstance(package_local.get('receipt'),dict): raise ValueError()
            source.verify(self.source_root,self.fence.context['commit'])
            row,rev=self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status')!='active' or self.preflight.fence.ledger.read()!=(row,rev) or
                    (self.record is not None and row.get(LEDGER)!=self.record) or row.get(EFFECTS,{})!=self.effects or
                    (self.initial is not None and (row.get(self.preflight.ledger)!=self.preflight.record or
                     self.preflight.record.get('receipt_sha256')!=writer_cut.digest(self.initial)))): raise ValueError()
            self._local()
            if self.package.journal.state['steps'].get(full_cohort_package_phase.STEP)!=package_local: raise ValueError()
            return row,rev
        except BaseException: raise schema.GuardLost('Role peer guard unavailable') from None

    def _check_current(self):
        self._guard()
        try:ca=self.preflight._ca(); actual=self.preflight._creator()
        except BaseException:raise schema.GuardLost('Role peer preflight observation unavailable') from None
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
            for name,value in effects.items():self.journal.state['steps']['space-role-peer-secret-'+name]=copy.deepcopy(value)
        self.journal.save()
        try:
            media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
            self.preflight._authority(); source.verify(self.source_root,self.fence.context['commit'])
            if (native.SpaceNativeToolPhase._custody(self.journal)!=self.custody or self.journal.intent!=self.intent or
                    self.journal.state.get('intent')!=self.intent): raise ValueError()
        except BaseException:raise schema.GuardLost('Role peer guard unavailable') from None
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
        try:self.fence.uncertain('Role peer Secret creation uncertain')
        except BaseException:pass

    def _drained(self):
        try:self.preflight.cut.verify_drained()
        except BaseException:raise schema.GuardLost('Role peer writer drain unavailable') from None

    def _socket(self):
        if self.listener is None or self.dirfd is None or self.inode is None: raise ValueError()
        info=os.stat('role.sock',dir_fd=self.dirfd,follow_symlinks=False)
        directory=os.fstat(self.dirfd); visible=os.stat(ROOT/self.suffix,follow_symlinks=False)
        if ((info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode) or info.st_uid!=os.geteuid() or stat.S_IMODE(info.st_mode)!=0o600 or
                (directory.st_dev,directory.st_ino)!=(visible.st_dev,visible.st_ino) or not stat.S_ISDIR(visible.st_mode) or
                directory.st_uid!=os.geteuid() or stat.S_IMODE(directory.st_mode)!=0o700 or
                os.getpid()!=self.record['pid'] or common._ticks()!=self.record['start_ticks']): raise ValueError()

    def _distinct_keys(self):
        keys=[]
        for name in sorted(NAMES):
            obj=self.objects[name] if name in self.objects else _get(name)
            values=_values(obj,name)
            for key in (('current.pem','next.pem') if name=='voice-role-principal-signing' else ('tls.key',)):
                public=role_material.crypto._openssl(['pkey','-in','{0}','-pubout'],[values[key]])
                keys.append(hashlib.sha256(public).digest())
        if len(keys)!=len(set(keys)): raise ValueError()

    def _receipt(self):
        self._check_current(); objects={}; jwks={}
        for name in sorted(NAMES):
            obj=_get(name); public=role_material.qualify(name,obj,self.media)
            expected=self.effects.get(name) or self.initial['creator_secrets'][name]
            identity={key:obj['metadata'].get(key) for key in ('uid','resourceVersion')}
            if any(not isinstance(value,str) or not value for value in identity.values()) or any(identity[k]!=expected[k] for k in identity): raise ValueError()
            objects[name]=identity
            if public is not None: jwks[name]=public
        if set(jwks)!={'voice-role-principal-signing'}: raise ValueError()
        self._check_current()
        return {'schema':'voice-dev-peer-receiver-v1','profile':'role','operation_id':self.fence.context['operation_id'],
                'source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid,
                'preflight_sha256':writer_cut.digest(self.initial),'ca':copy.deepcopy(self.initial['ca']),
                'objects':objects,'public_jwks':jwks,'scope':'fixed-peer-secrets-no-runtime-readiness'}

    def prepare(self):
        try:
            self._guard(); self.initial=preflight.verify_current(self.preflight,self.token)
            source.verify(self.source_root,self.fence.context['commit']); row,_=self._guard()
            if LEDGER in row or EFFECTS in row or STEP in self.journal.state['steps']: raise ValueError()
            if self.initial.get('profile')!='role': raise ValueError()
            missing=sorted(name for name in NAMES if self.initial['creator_secrets'][name]=={'state':'absent'})
            self.missing=missing
            self.media=preflight.public._value(preflight.public.common.linux_private_bytes,self._guard,'secret','voice-media-ca','ca.crt')
            if hashlib.sha256(preflight.crypto.der(self.media)).hexdigest()!=self.initial['ca']['der_sha256']: raise ValueError()
            for name in sorted(NAMES):
                item=self.initial['creator_secrets'][name]
                if item['state']=='present':
                    obj=_get(name); values=_values(obj,name)
                    if any(obj['metadata'].get(k)!=item[k] for k in ('uid','resourceVersion')): raise ValueError()
                    role_material.qualify(name,obj,self.media)
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit'],'issuer_source_sha':source.REVIEWED_SOURCE,
                    'preflight_sha256':writer_cut.digest(self.initial),'missing':missing,'pid':os.getpid(),'start_ticks':common._ticks()}
            self.pending=True; self._write(record=marker)
            if not missing:
                self.objects={}; self._distinct_keys(); self._check_current()
                receipt=self._receipt()
                self.proof=copy.deepcopy(receipt); done=marker|{'status':'done','receipt':copy.deepcopy(receipt)}
                self._write(record=done); self._check_current(); schema._fence(self._local)
                if self.proof!=receipt or self.record!=done or self.journal.state['steps'].get(STEP)!=done: raise schema.GuardLost('Role peer terminal record changed')
                self.completed=True; return copy.deepcopy(done)
            self.dirfd=_directory(self.suffix); self.listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            self.listener.bind(str(ROOT/self.suffix/'role.sock')); os.chmod('role.sock',0o600,dir_fd=self.dirfd,follow_symlinks=False)
            info=os.stat('role.sock',dir_fd=self.dirfd,follow_symlinks=False)
            if not stat.S_ISSOCK(info.st_mode) or info.st_uid!=os.geteuid() or stat.S_IMODE(info.st_mode)!=0o600: raise ValueError()
            self.inode=(info.st_dev,info.st_ino); self.listener.listen(1); os.fsync(self.dirfd); self._check_current()
            ready=marker|{'status':'ready','socket_dev':info.st_dev,'socket_ino':info.st_ino}; self._write(record=ready)
            return copy.deepcopy(ready)
        except BaseException as error:
            if self.pending:self._uncertain()
            self.close()
            if isinstance(error,schema.GuardLost):raise
            raise CreateError('Role peer handoff unavailable; reconcile if reserved') from None

    def serve_preflight(self):
        try:
            if self.exchanged or self.listener is None or self.record.get('status')!='ready': raise ValueError()
            self._socket(); deadline=time.monotonic()+300
            while True:
                self._check_current(); self._drained()
                if self.issue_done is None or self.issue_done.is_set(): raise ValueError()
                remaining=deadline-time.monotonic()
                if remaining<=0: raise ValueError()
                if select.select([self.listener],[],[],min(remaining,1))[0]: break
            connection,_=self.listener.accept()
            self.connection=connection; connection.settimeout(120)
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
                except BaseException:raise schema.GuardLost('Role peer preflight authority unavailable') from None
                if exchanged!=self.initial: raise ValueError()
                if root_export._start_ticks(pid)!=start: raise ValueError()
            self._socket(); self._check_current(); self.exchanged=True
        except BaseException as error:
            self._uncertain()
            if isinstance(error,schema.GuardLost):raise
            raise CreateError('Role peer preflight exchange uncertain') from None

    def run(self):
        """Trigger the fixed signer while servicing its authenticated callbacks."""
        if self.completed and not self.missing: return verify_completed(self,self.token)
        if self.issue_thread is not None or self.record is None or self.record.get('status')!='ready':
            raise CreateError('Role peer issue unavailable')
        import dev_peer_issuer_client
        try:
            self.issue_control=dev_peer_issuer_client.new_issue_trigger(self.preflight,self.token)
            self.issue_done=threading.Event(); self.issue_result=[]
            def trigger():
                try: self.issue_result.append(('ok',self.issue_control.run()))
                except BaseException: self.issue_result.append(('error',None))
                finally: self.issue_done.set()
            self.issue_thread=threading.Thread(target=trigger,daemon=True); self.issue_thread.start()
            self.serve_preflight(); return self.finish()
        except BaseException as error:
            if self.pending and not self.completed: self._uncertain()
            if self.issue_control is not None:self.issue_control.cancel()
            if self.issue_thread is not None and self.issue_thread.ident is not None:self.issue_thread.join(2)
            self.close()
            if isinstance(error,schema.GuardLost):raise
            raise CreateError('Role peer issue unavailable; reconcile') from None

    def _await_issue(self):
        if self.issue_thread is None or self.issue_done is None or not self.issue_done.wait(180): raise ValueError()
        self.issue_thread.join(1)
        if self.issue_thread.is_alive() or self.issue_result!=[('ok',None)]: raise ValueError()

    def finish(self):
        try:
            if not self.exchanged or self.listener is None or self.record.get('status')!='ready': raise ValueError()
            self._socket(); self._check_current(); self.envelope=_receive(self.listener,self); self._await_issue(); self._check_current()
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
            receipt=self._receipt()
            self.proof=copy.deepcopy(receipt); done=self.record|{'status':'done','receipt':copy.deepcopy(receipt)}
            effects=copy.deepcopy(self.effects); self._write(record=done); self._check_current(); schema._fence(self._local)
            if (self.proof!=receipt or self.record!=done or self.effects!=effects or
                    self.journal.state['steps'].get(STEP)!=done): raise schema.GuardLost('Role peer terminal record changed')
            self.completed=True; return copy.deepcopy(receipt)
        except BaseException as error:
            if self.pending and not self.completed:self._uncertain()
            if isinstance(error,schema.GuardLost):raise
            raise CreateError('Role peer creation uncertain; retain lock and reconcile') from None
        finally:self.close()

    def close(self):
        if self.connection is not None:self.connection.close(); self.connection=None
        if self.listener is not None:self.listener.close(); self.listener=None
        if self.dirfd is not None:
            try:
                if self.inode is not None:
                    info=os.stat('role.sock',dir_fd=self.dirfd,follow_symlinks=False)
                    if (info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode): raise ValueError()
                    os.unlink('role.sock',dir_fd=self.dirfd); os.fsync(self.dirfd)
            except Exception:
                if self.pending:self._uncertain()
                raise CreateError('Role peer socket cleanup uncertain') from None
            finally:os.close(self.dirfd); self.dirfd=None


def _inspect_existing(phase,token,profile):
        row,rev=phase._guard()
        package_local=copy.deepcopy(phase.package.journal.state['steps'].get(full_cohort_package_phase.STEP))
        if not isinstance(package_local,dict) or package_local.get('status')!='done' or not isinstance(package_local.get('receipt'),dict): raise ValueError()
        initial=preflight.verify_current(phase.preflight,token)
        if initial.get('profile')!=profile: raise ValueError()
        media=preflight.public._value(preflight.public.common.linux_private_bytes,phase._guard,'secret','voice-media-ca','ca.crt')
        if hashlib.sha256(preflight.crypto.der(media)).hexdigest()!=initial['ca']['der_sha256']: raise ValueError()
        present={}; missing=[]; jwks={}; keys=[]
        for name in sorted(NAMES):
            identity=initial['creator_secrets'][name]
            if identity=={'state':'absent'}:
                missing.append(name); continue
            obj=_get(name); values=_values(obj,name)
            if any(obj['metadata'].get(k)!=identity[k] for k in ('uid','resourceVersion')): raise ValueError()
            public=role_material.qualify(name,obj,media)
            present[name]=copy.deepcopy(identity)
            if public is not None:jwks[name]=public
            for key in (('current.pem','next.pem') if name=='voice-role-principal-signing' else ('tls.key',)):
                keys.append(hashlib.sha256(role_material.crypto._openssl(['pkey','-in','{0}','-pubout'],[values[key]])).digest())
        if len(keys)!=len(set(keys)) or preflight.verify_current(phase.preflight,token)!=initial: raise ValueError()
        if phase._guard()!=(row,rev): raise ValueError()
        phase._local()
        if (phase.record is not None or phase.pending or
                phase.package.journal.state['steps'].get(full_cohort_package_phase.STEP)!=package_local): raise ValueError()
        return {'schema':'voice-dev-peer-existing-v1','profile':'role','operation_id':phase.fence.context['operation_id'],
                'source_sha':phase.fence.context['commit'],'namespace_uid':phase.namespace_uid,
                'preflight_sha256':writer_cut.digest(initial),'ca':copy.deepcopy(initial['ca']),
                'present':present,'missing':missing,'public_jwks':jwks,'scope':'predrain-peer-material-observation-not-runtime-readiness'}


def inspect_existing(phase,token):
    """Read-only qualification using the fresh Role preflight."""
    try:
        if type(phase) is not Session or token!=phase.token or phase.record is not None or phase.pending: raise ValueError()
        return _inspect_existing(phase,token,'role')
    except schema.GuardLost: raise
    except BaseException: raise CreateError('Role peer inspection unavailable') from None


def inspect_role_from_creator(phase,token):
    """Qualify Role's current materials using the single guarded Creator preflight."""
    import space_creator_runtime_profile_delegated as delegated
    try:
        if (type(phase) is not delegated.CreatorReceiver or token!=phase.token or phase.preflight.profile!='creator' or
                phase.record is not None or phase.pending): raise ValueError()
        return _inspect_existing(phase,token,'creator')
    except schema.GuardLost: raise
    except BaseException: raise CreateError('Role peer inspection unavailable') from None


def verify_completed(phase,token):
    try:
        if type(phase) is not Session or token!=phase.token or not phase.completed or phase.record.get('status')!='done' or phase.record.get('receipt')!=phase.proof: raise ValueError()
        phase._check_current()
        objects={} if not phase.missing else packet.validate(phase.envelope,'role',phase.fence.context['operation_id'],phase.fence.context['commit'],phase.namespace_uid,
                                writer_cut.digest(phase.initial),phase.missing,phase.media)
        if objects!=phase.objects: raise ValueError()
        for name in sorted(NAMES):
            role_material.qualify(name,phase.objects.get(name) or _get(name),phase.media)
        phase._distinct_keys()
        if phase._receipt()!=phase.proof: raise ValueError()
        phase._check_current(); row,_=phase._guard(); phase._local()
        if (row.get(LEDGER)!=phase.record or row.get(EFFECTS,{})!=phase.effects or phase.journal.state['steps'].get(STEP)!=phase.record or
                phase.record.get('receipt')!=phase.proof): raise ValueError()
        return copy.deepcopy(phase.proof)
    except BaseException as error:
        if type(phase) is Session and phase.pending:phase._uncertain()
        if isinstance(error,schema.GuardLost):raise
        raise CreateError('Role peer creation proof unavailable') from None
