"""Fixed Creator/Space receivers for the installed DEV peer issuer.

The public result is operation evidence, not runtime or network readiness.
"""
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
import threading
import time

import canonical_application_package as canonical
import dev_peer_issuer_client as issuer_client
import dev_peer_issuer_metadata as metadata
import dev_peer_issuer_packet as packet
import dev_peer_issuer_preflight as preflight
import frontend_runtime_executor as transport
import full_cohort_package_phase
import media_operation_guard
import media_operation_journal
import realtime_p3_create_only as common
import realtime_p3_create_only_socket as channel
import realtime_p3_create_only_transport as private
import realtime_p3_root_export as root_export
import resource_fencing
import space_native_tool_identity as native
import space_creator_runtime_profile_create as existing_creator
import story_media_backup as backup
import writer_cut

ROOT=Path('/home/pmd/voice-dev-peer-issuer/operations')
FRAME=b'VOICE-DEV-PEER-PACKET\n'
ACK=b'VOICE-DEV-PEER-ACCEPTED\n'


class DelegatedError(RuntimeError): pass


def _get(name,profile):
    if name not in metadata.PROFILES[profile]: raise ValueError()
    raw=private._run(['get','secret',name,'--ignore-not-found=true','-o','json'])
    return None if not raw else json.loads(raw,object_pairs_hook=channel._unique)


def _create(obj,profile,dry):
    if type(dry) is not bool or obj.get('kind')!='Secret' or obj.get('metadata',{}).get('name') not in metadata.PROFILES[profile]: raise ValueError()
    args=['create','-f','-','-o','json']
    if dry: args.insert(1,'--dry-run=server')
    return json.loads(private._run(args,json.dumps(obj,separators=(',',':')).encode('ascii')),object_pairs_hook=channel._unique)


def _same(actual,desired):
    return existing_creator._closed(actual,desired)


def _normalized(obj,name):
    if (not isinstance(obj,dict) or obj.get('apiVersion')!='v1' or obj.get('kind')!='Secret' or obj.get('type')!='Opaque'
            or obj.get('metadata',{}).get('name')!=name or obj['metadata'].get('namespace')!='voice-dev'
            or obj['metadata'].get('deletionTimestamp') or not isinstance(obj.get('data'),dict)
            or set(obj['data'])!=metadata.MEMBERS[name]): raise ValueError()
    return {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-dev'},
            'type':'Opaque','data':copy.deepcopy(obj['data'])}


class _Receiver:
    PROFILE=None
    def __init__(self,journal,fence,client,lock,namespace_uid,package,cut,issuer_preflight,source_root,token):
        if (self.PROFILE not in ('creator','space') or type(journal) is not media_operation_journal.Journal
                or type(fence) is not resource_fencing.Fence or type(client) is not transport.KubectlTransport
                or client.journal is not journal or fence.ledger.client is not client
                or type(package) is not full_cohort_package_phase.PackagePhase or type(cut) is not writer_cut.WriterCut
                or type(issuer_preflight) is not preflight.PeerIssuerPreflightPhase or issuer_preflight.profile!=self.PROFILE
                or issuer_preflight.package is not package or issuer_preflight.cut is not cut
                or any(x.fence.context!=fence.context or x.lock!=lock or x.namespace_uid!=namespace_uid for x in (package,cut,issuer_preflight))
                or Path(source_root).resolve()!=canonical.ROOT.resolve() or not isinstance(token,str) or not token
                or journal.intent.get('resource_context')!=fence.context or journal.intent.get('namespace_uid')!=namespace_uid
                or journal.intent.get('peer_receiver')!={'profile':self.PROFILE,'source_sha':fence.context['commit']}): raise DelegatedError('peer receiver identity unavailable')
        match=re.fullmatch(r'media-c1-([a-f0-9]{32})',fence.context.get('operation_id',''))
        if not match: raise DelegatedError('peer receiver identity unavailable')
        self.journal,self.fence,self.client,self.package,self.cut,self.preflight=journal,fence,client,package,cut,issuer_preflight
        self.lock,self.namespace_uid,self.source_root,self.token=copy.deepcopy(lock),namespace_uid,Path(source_root).resolve(),token
        self.suffix=match[1]; self.intent=copy.deepcopy(journal.intent); self.identity=(copy.deepcopy(fence.context),copy.deepcopy(lock),namespace_uid,self.source_root,token,self.suffix)
        self.step='dev-peer-receiver-'+self.PROFILE; self.ledger='dev_peer_receiver_'+self.PROFILE; self.effects_key=self.ledger+'_effects'
        self.refs=(journal,fence,client,fence.ledger,package,cut,issuer_preflight); self.custody=native.SpaceNativeToolPhase._custody(journal)
        self.record=None; self.effects={}; self.initial=None; self.objects={}; self.pending=False; self.completed=False; self.poisoned=False
        self.listener=self.dirfd=self.inode=None; self.envelope=None; self.media=None; self.proof=None

    def _local(self):
        if (self.poisoned or any(a is not b for a,b in zip(self.refs,(self.journal,self.fence,self.client,self.fence.ledger,self.package,self.cut,self.preflight)))
                or self.client.journal is not self.journal or self.fence.ledger.client is not self.client or self.journal.poisoned
                or (self.fence.context,self.lock,self.namespace_uid,self.source_root,self.token,self.suffix)!=self.identity
                or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent
                or native.SpaceNativeToolPhase._custody(self.journal)!=self.custody): raise ValueError()
        self.preflight._local()
        if self.record is not None and self.journal.state['steps'].get(self.step)!=self.record: raise ValueError()
        local={k.removeprefix(self.step+'-secret-'):v for k,v in self.journal.state['steps'].items() if k.startswith(self.step+'-secret-')}
        if local!=self.effects: raise ValueError()

    def _guard(self):
        self._local(); dependencies=copy.deepcopy(tuple((p.journal.intent,p.journal.state) for p in
                                                       (self.package,self.cut,self.preflight)))
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        self.preflight._authority(); self.cut._guard(); row,rev=self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status')!='active' or self.preflight.fence.ledger.read()!=(row,rev)
                or self.cut.fence.ledger.read()!=(row,rev) or self.package.fence.ledger.read()!=(row,rev)
                or (self.record is not None and row.get(self.ledger)!=self.record) or row.get(self.effects_key,{})!=self.effects
                or (self.initial is not None and (row.get(self.preflight.ledger)!=self.preflight.record
                    or self.preflight.record.get('receipt_sha256')!=writer_cut.digest(self.initial)))): raise ValueError()
        self._local()
        if dependencies!=tuple((p.journal.intent,p.journal.state) for p in (self.package,self.cut,self.preflight)): raise ValueError()
        return row,rev

    def _drained(self):
        self._guard(); self.cut.verify_drained(); self._guard()

    def _current(self):
        self._guard(); ca=self.preflight._ca(); actual=self.preflight._creator()
        if ca!=self.initial['ca']: raise ValueError()
        expected=copy.deepcopy(self.initial['creator_secrets'])
        for name,effect in self.effects.items():
            if effect['status']=='done': expected[name]={'state':'present','uid':effect['uid'],
                'resourceVersion':effect['resourceVersion'],'keys':sorted(metadata.MEMBERS[name])}
        if actual!=expected: raise ValueError()
        for name,effect in self.effects.items():
            if effect['status']=='done':
                obj=_get(name,self.PROFILE)
                if not _same(obj,self.objects[name]) or any(obj['metadata'].get(k)!=effect[k] for k in ('uid','resourceVersion')): raise ValueError()
        self._guard()

    def _write(self,record=None,effects=None):
        row,rev=self._guard()
        if record is not None:self.journal.state['steps'][self.step]=copy.deepcopy(record)
        if effects is not None:
            for name,value in effects.items():self.journal.state['steps'][self.step+'-secret-'+name]=copy.deepcopy(value)
        self.journal.save(); media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        self.preflight._authority(); self.cut._guard()
        if native.SpaceNativeToolPhase._custody(self.journal)!=self.custody or self.journal.intent!=self.intent: raise ValueError()
        now,version=self.fence.ledger.read()
        if now!=row or version!=rev: raise ValueError()
        if record is not None:now[self.ledger]=copy.deepcopy(record)
        if effects is not None:now[self.effects_key]=copy.deepcopy(effects)
        self.fence.ledger.write(version,now)
        if record is not None:self.record=copy.deepcopy(record)
        if effects is not None:self.effects=copy.deepcopy(effects)
        self._guard()

    def _crypto(self,names,incoming=None,*,space_inspection=False):
        if space_inspection and (type(self) is not CreatorReceiver or incoming is not None): raise ValueError()
        profile='space' if space_inspection else self.PROFILE
        if not names: return {}
        if names!=sorted(set(names)) or set(names)-set(metadata.PROFILES[profile]): raise ValueError()
        objects=[]; jwks={}; public_keys=[]; identities=self.preflight._creator(); incoming=incoming or {}
        for name in names:
            current=incoming.get(name); identity=identities[name]
            if current is None:
                current=_get(name,profile)
                if current is None or identity.get('state')!='present' or any(current['metadata'].get(k)!=identity[k] for k in ('uid','resourceVersion')): raise ValueError()
            elif identity!={'state':'absent'}: raise ValueError()
            obj=_normalized(current,name); objects.append(obj)
            values={k:base64.b64decode(v,validate=True) for k,v in obj['data'].items()}
            key_names=('current.pem','next.pem') if name.endswith('-principal-signing') else ('tls.key',)
            if name.endswith('-principal-signing'):
                if values['active-kid'] not in (b'current',b'next'): raise ValueError()
                jwks[name]=packet.crypto._jwks(values)
                # Packet validator's issued-key shape uses current; this private
                # projection validates existing keys without altering a Secret.
                obj['data']['active-kid']=base64.b64encode(b'current').decode('ascii')
            for key in key_names:public_keys.append(hashlib.sha256(packet.crypto._openssl(['pkey','-in','{0}','-pubout'],[values[key]])).digest())
        if len(public_keys)!=len(set(public_keys)): raise ValueError()
        receipt={'schema':'voice-dev-peer-issuer-receipt-v1','status':'staged','profile':profile,'object_names':names,
                 'media_ca_der_sha256':self.initial['ca']['der_sha256'],'public_jwks':jwks,'runtime_qualified':False,'cluster_applied':False}
        envelope={'schema':'voice-dev-peer-issuer-packet-v1','profile':profile,'operation_id':self.fence.context['operation_id'],
                  'source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid,'preflight_sha256':writer_cut.digest(self.initial),
                  'issuer_receipt':receipt,'objects':{'apiVersion':'v1','kind':'List','items':objects}}
        packet.validate(envelope,profile,self.fence.context['operation_id'],self.fence.context['commit'],self.namespace_uid,
                        writer_cut.digest(self.initial),names,self.media)
        self._current(); return jwks

    def _socket(self):
        if self.listener is None or self.dirfd is None or self.inode is None: raise ValueError()
        info=os.stat(self.PROFILE+'.sock',dir_fd=self.dirfd,follow_symlinks=False)
        directory=os.fstat(self.dirfd); visible=os.stat(ROOT/self.suffix,follow_symlinks=False)
        if ((info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode)
                or info.st_uid!=os.geteuid() or stat.S_IMODE(info.st_mode)!=0o600
                or (directory.st_dev,directory.st_ino)!=(visible.st_dev,visible.st_ino)
                or os.getpid()!=self.record['pid'] or common._ticks()!=self.record['start_ticks']): raise ValueError()

    def _packet(self,connection,pid,start):
        fds=[]
        try:
            data,ancillary,flags,_=connection.recvmsg(len(FRAME)+1,socket.CMSG_SPACE(8*array.array('i').itemsize),socket.MSG_CMSG_CLOEXEC)
            bad=False
            for level,kind,payload in ancillary:
                if level!=socket.SOL_SOCKET or kind!=socket.SCM_RIGHTS: bad=True; continue
                numbers=array.array('i')
                if len(payload)%numbers.itemsize: bad=True
                numbers.frombytes(payload[:len(payload)-len(payload)%numbers.itemsize]); fds.extend(numbers)
            if (bad or flags & (socket.MSG_TRUNC|socket.MSG_CTRUNC) or data!=FRAME or len(ancillary)!=1 or len(fds)!=1
                    or root_export._start_ticks(pid)!=start): raise ValueError()
            fd=fds[0]; info=os.fstat(fd)
            if (not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_nlink!=0 or stat.S_IMODE(info.st_mode)!=0o600
                    or not 0<info.st_size<=channel.MAX_PACKET
                    or fcntl.fcntl(fd,fcntl.F_GET_SEALS)&channel.REQUIRED_SEALS!=channel.REQUIRED_SEALS): raise ValueError()
            raw=bytearray(); offset=0
            while offset<info.st_size:
                chunk=os.pread(fd,min(65536,info.st_size-offset),offset)
                if not chunk: raise ValueError()
                raw.extend(chunk); offset+=len(chunk)
            end=os.fstat(fd)
            if (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)!=(end.st_dev,end.st_ino,end.st_size,end.st_mtime_ns,end.st_ctime_ns): raise ValueError()
            envelope=json.loads(raw,object_pairs_hook=channel._unique); self._current(); self._drained()
            objects=packet.validate(envelope,self.PROFILE,self.fence.context['operation_id'],self.fence.context['commit'],
                                    self.namespace_uid,writer_cut.digest(self.initial),self.missing,self.media)
            self._crypto(sorted(metadata.PROFILES[self.PROFILE]),objects)
            self._current(); self._drained()
            if root_export._start_ticks(pid)!=start or connection.send(ACK)!=len(ACK): raise ValueError()
            return envelope,objects
        finally:
            for fd in fds:
                try:os.close(fd)
                except OSError:pass

    def _obtain(self):
        directory=ROOT/self.suffix; self._manager=backup.private_directory(directory); self.dirfd=self._manager.__enter__()
        if os.geteuid()!=1000: raise ValueError()
        self.listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
        self.listener.bind(str(directory/(self.PROFILE+'.sock'))); os.chmod(self.PROFILE+'.sock',0o600,dir_fd=self.dirfd,follow_symlinks=False)
        info=os.stat(self.PROFILE+'.sock',dir_fd=self.dirfd,follow_symlinks=False)
        if not stat.S_ISSOCK(info.st_mode) or info.st_uid!=os.geteuid(): raise ValueError()
        self.inode=(info.st_dev,info.st_ino); self.listener.listen(1); os.fsync(self.dirfd); self._socket(); self._current(); self._drained()
        result=[]; handle=issuer_client.new_issue_trigger(self.preflight,self.token)
        def invoke():
            try:handle.run(); result.append(True)
            except BaseException:result.append(False)
        worker=threading.Thread(target=invoke); worker.start(); deadline=time.monotonic()+300
        try:
            while True:
                self._current(); self._drained()
                if time.monotonic()>=deadline or (result and result!=[True]): raise ValueError()
                self.listener.settimeout(min(2,max(0.01,deadline-time.monotonic())))
                try:connection,_=self.listener.accept(); break
                except socket.timeout: continue
            with connection:
                connection.settimeout(180)
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
                    self._current(); self._drained()
                    if preflight.exchange_root_request(self.preflight,self.token,connection)!=self.initial or root_export._start_ticks(pid)!=start: raise ValueError()
                self.envelope,self.objects=self._packet(connection,pid,start)
            worker.join(timeout=max(0,deadline-time.monotonic()))
            if worker.is_alive() or result!=[True]: raise ValueError()
            self._socket(); self._current(); self._drained()
        finally:
            if worker.is_alive():
                try:handle.cancel()
                except BaseException:pass
                while worker.is_alive():worker.join(timeout=5)

    def run(self):
        try:
            self._guard(); self._drained(); self.initial=preflight.verify_current(self.preflight,self.token)
            if (self.initial.get('profile')!=self.PROFILE or set(self.initial.get('creator_secrets',{}))!=set(metadata.MEMBERS)): raise ValueError()
            self.missing=sorted(name for name in metadata.PROFILES[self.PROFILE] if self.initial['creator_secrets'][name]=={'state':'absent'})
            self.media=preflight.public._value(preflight.public.common.linux_private_bytes,self._guard,'secret','voice-media-ca','ca.crt')
            if hashlib.sha256(preflight.crypto.der(self.media)).hexdigest()!=self.initial['ca']['der_sha256']: raise ValueError()
            row,_=self._guard()
            if self.ledger in row or self.effects_key in row or self.step in self.journal.state['steps']: raise ValueError()
            present=sorted(set(metadata.PROFILES[self.PROFILE])-set(self.missing)); self._crypto(present)
            record={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit'],'preflight_sha256':writer_cut.digest(self.initial),
                    'missing':copy.deepcopy(self.missing),'pid':os.getpid(),'start_ticks':common._ticks()}
            self.pending=True; self._write(record=record)
            if self.missing:
                self._obtain()
                for name in self.missing:
                    self._current(); self._drained()
                    if _get(name,self.PROFILE) is not None or not existing_creator._closed(_create(self.objects[name],self.PROFILE,True),self.objects[name],False): raise ValueError()
                for name in self.missing:
                    self._current(); self._drained()
                    if _get(name,self.PROFILE) is not None: raise ValueError()
                    pending={'status':'pending','nonce':secrets.token_hex(16),'kind':'Secret','name':name,'keys':sorted(metadata.MEMBERS[name])}
                    self._write(effects=self.effects|{name:pending}); self._current(); self._drained()
                    response=_create(self.objects[name],self.PROFILE,False); self._drained(); actual=_get(name,self.PROFILE)
                    if (not _same(response,self.objects[name]) or not _same(actual,self.objects[name])
                            or any(response['metadata'].get(k)!=actual['metadata'].get(k) for k in ('uid','resourceVersion'))): raise ValueError()
                    self._write(effects=self.effects|{name:pending|{'status':'done','uid':actual['metadata']['uid'],
                                                                    'resourceVersion':actual['metadata']['resourceVersion']}})
            self._current(); self._drained(); jwks=self._crypto(sorted(metadata.PROFILES[self.PROFILE])); self._current()
            identities={}
            for name in sorted(metadata.PROFILES[self.PROFILE]):
                item=self.preflight._creator()[name]; identities[name]={'uid':item['uid'],'resourceVersion':item['resourceVersion']}
            receipt={'schema':'voice-dev-peer-receiver-v1','profile':self.PROFILE,'operation_id':self.fence.context['operation_id'],
                     'source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid,'preflight_sha256':writer_cut.digest(self.initial),
                     'ca':copy.deepcopy(self.initial['ca']),'objects':identities,'public_jwks':jwks,
                     'scope':'fixed-peer-secrets-no-runtime-readiness'}
            done=self.record|{'status':'done','receipt':copy.deepcopy(receipt)}; self._write(record=done); self._current(); self._drained(); self._current()
            if self.record!=done or self.journal.state['steps'].get(self.step)!=done: raise ValueError()
            self.proof=copy.deepcopy(receipt); self.completed=True; return copy.deepcopy(receipt)
        except BaseException:
            if self.pending:self._uncertain()
            raise DelegatedError('peer Secret outcome unavailable; reconcile if reserved') from None
        finally:self.close()

    def _uncertain(self):
        self.poisoned=True
        try:self.fence.uncertain('DEV peer Secret outcome uncertain')
        except BaseException:pass

    def close(self):
        if self.listener is not None:self.listener.close(); self.listener=None
        if self.dirfd is not None:
            try:
                if self.inode is not None:
                    info=os.stat(self.PROFILE+'.sock',dir_fd=self.dirfd,follow_symlinks=False)
                    if (info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode): raise ValueError()
                    os.unlink(self.PROFILE+'.sock',dir_fd=self.dirfd); os.fsync(self.dirfd)
            except Exception:
                if self.pending:self._uncertain()
                raise DelegatedError('peer socket cleanup uncertain') from None
            finally:self._manager.__exit__(None,None,None); self.dirfd=None


class CreatorReceiver(_Receiver): PROFILE='creator'
class SpaceReceiver(_Receiver): PROFILE='space'


def inspect_existing(phase,token):
    """Privately qualify fixed current profile before any writer drain/effect."""
    return _inspect(phase,token,False)


def inspect_space_from_creator(phase,token):
    """Qualify only Space's existing materials against the current creator preflight."""
    return _inspect(phase,token,True)


def _inspect(phase,token,space_inspection):
    try:
        if type(phase) not in (CreatorReceiver,SpaceReceiver) or token!=phase.token or phase.record is not None or phase.pending: raise ValueError()
        if space_inspection and type(phase) is not CreatorReceiver: raise ValueError()
        profile='space' if space_inspection else phase.PROFILE
        row,rev=phase._guard(); initial=preflight.verify_current(phase.preflight,token)
        if initial.get('profile')!=phase.PROFILE or set(initial.get('creator_secrets',{}))!=set(metadata.MEMBERS): raise ValueError()
        phase.initial=copy.deepcopy(initial)
        phase.media=preflight.public._value(preflight.public.common.linux_private_bytes,phase._guard,'secret','voice-media-ca','ca.crt')
        if hashlib.sha256(preflight.crypto.der(phase.media)).hexdigest()!=initial['ca']['der_sha256']: raise ValueError()
        missing=sorted(name for name in metadata.PROFILES[profile] if initial['creator_secrets'][name]=={'state':'absent'})
        present=sorted(set(metadata.PROFILES[profile])-set(missing)); jwks=phase._crypto(present,space_inspection=space_inspection)
        if preflight.verify_current(phase.preflight,token)!=initial: raise ValueError()
        after,end=phase._guard(); phase._local()
        if after!=row or end!=rev or phase.record is not None: raise ValueError()
        return {'schema':'voice-dev-peer-existing-v1','profile':profile,'operation_id':phase.fence.context['operation_id'],
                'source_sha':phase.fence.context['commit'],'namespace_uid':phase.namespace_uid,
                'preflight_sha256':writer_cut.digest(initial),'ca':copy.deepcopy(initial['ca']),
                'present':{name:copy.deepcopy(initial['creator_secrets'][name]) for name in present},'missing':missing,
                'public_jwks':jwks,'scope':'predrain-peer-material-observation-not-runtime-readiness'}
    except BaseException: raise DelegatedError('peer predrain observation unavailable') from None


def verify_completed(phase,token):
    try:
        if type(phase) not in (CreatorReceiver,SpaceReceiver) or token!=phase.token or not phase.completed or phase.record.get('status')!='done': raise ValueError()
        phase._current(); jwks=phase._crypto(sorted(metadata.PROFILES[phase.PROFILE])); phase._current(); row,_=phase._guard(); phase._local()
        if (phase.record.get('receipt')!=phase.proof or phase.proof['public_jwks']!=jwks or row.get(phase.ledger)!=phase.record
                or row.get(phase.effects_key,{})!=phase.effects or phase.journal.state['steps'].get(phase.step)!=phase.record): raise ValueError()
        return copy.deepcopy(phase.proof)
    except BaseException:
        if type(phase) in (CreatorReceiver,SpaceReceiver) and phase.pending:phase._uncertain()
        raise DelegatedError('peer Secret proof unavailable') from None
