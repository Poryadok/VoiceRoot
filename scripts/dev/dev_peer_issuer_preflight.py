"""Fixed nine-name protected DEV peer-issuer preflight."""
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

import frontend_runtime_executor
import full_cohort_package_phase
import media_operation_guard
import media_operation_journal
import realtime_p3_create_only as common
import realtime_p3_create_only_crypto as crypto
import realtime_p3_public_inputs as public
import dev_peer_issuer_source as source
import dev_peer_issuer_metadata as metadata
import story_media_backup as backup
import writer_cut

SOCKET_ROOT=Path('/home/pmd/voice-dev-peer-issuer/preflight')


class PreflightError(RuntimeError): pass


class PeerIssuerPreflightPhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,package,cut,source_root,token,profile):
        self.journal,self.fence,self.client,self.package,self.cut=journal,fence,client,package,cut
        if profile not in metadata.PROFILES: raise PreflightError('fixed peer profile required')
        self.profile=profile; self.step='dev-peer-issuer-preflight-'+profile; self.ledger='dev_peer_issuer_preflight_'+profile
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.source_root,self.token=source_root,token; self.intent=copy.deepcopy(journal.intent)
        self._identity=(profile,self.step,self.ledger,source_root,token,copy.deepcopy(lock),namespace_uid)
        self.record=self.listener=self.dirfd=self.inode=None; self.pending=False
        match=re.fullmatch(r'media-c1-([a-f0-9]{32})',fence.context.get('operation_id',''))
        if (not match or type(journal) is not media_operation_journal.Journal
                or type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
                or getattr(fence.ledger,'client',None) is not client or type(package) is not full_cohort_package_phase.PackagePhase
                or type(cut) is not writer_cut.WriterCut or journal is cut.journal or package.fence.context!=fence.context
                or cut.fence.context!=fence.context or any(x.lock!=lock or x.namespace_uid!=namespace_uid for x in (cut,package))
                or self.intent.get('resource_context')!=fence.context or self.intent.get('namespace_uid')!=namespace_uid
                or self.intent.get('dev_peer_issuer_preflight')!={'source_root':source_root,'source_sha':fence.context.get('commit'),'profile':profile}
                or not isinstance(token,str) or not token or not Path(source_root).is_absolute()): raise PreflightError('preflight identity unavailable')
        self.suffix=match[1]
        self._phase_refs=(journal,fence,client,package,package.journal,package.fence,package.client,cut,cut.journal,cut.fence,cut.client)
        journals=(journal,package.journal,cut.journal)
        if len({id(item) for item in journals})!=3: raise PreflightError('preflight journals must be distinct')
        self._journal_custody=tuple(self._custody(item) for item in journals)

    @staticmethod
    def _custody(journal):
        if type(journal) is not media_operation_journal.Journal or type(journal.dirfd) is not int or type(journal.lockfd) is not int: raise ValueError()
        directory=os.fstat(journal.dirfd); lock=os.fstat(journal.lockfd)
        media_operation_journal.Journal._private(directory,0o700,directory=True)
        media_operation_journal.Journal._private(lock,0o600)
        path_dir=os.stat(journal.path,follow_symlinks=False); path_lock=os.stat(journal.path/'lock',follow_symlinks=False)
        if ((directory.st_dev,directory.st_ino)!=(path_dir.st_dev,path_dir.st_ino)
                or (lock.st_dev,lock.st_ino)!=(path_lock.st_dev,path_lock.st_ino)): raise ValueError()
        return (journal.dirfd,journal.lockfd,directory.st_dev,directory.st_ino,lock.st_dev,lock.st_ino)

    def _local(self):
        refs=(self.journal,self.fence,self.client,self.package,self.package.journal,self.package.fence,self.package.client,
              self.cut,self.cut.journal,self.cut.fence,self.cut.client)
        if (any(actual is not expected for actual,expected in zip(refs,self._phase_refs))
                or (self.profile,self.step,self.ledger,self.source_root,self.token,self.lock,self.namespace_uid)!=self._identity
                or any(x.client.journal is not x.journal or x.fence.ledger.client is not x.client for x in (self,self.package,self.cut))
                or tuple(self._custody(j) for j in (self.journal,self.package.journal,self.cut.journal))!=self._journal_custody
                or self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent): raise ValueError()
        if self.record is not None:
            local=self.journal.state['steps'].get(self.step)
            if not isinstance(local,dict) or {k:v for k,v in local.items() if k!='receipt'}!=self.record: raise ValueError()
            if self.record['status']=='done' and writer_cut.digest(local.get('receipt'))!=self.record['receipt_sha256']: raise ValueError()

    def _guard(self):
        self._local(); media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid); self._local()
        if self.record is not None:
            row,_=self.fence.ledger.read()
            if row.get(self.ledger)!=self.record: raise ValueError()
        self._local()

    def _authority(self):
        self._guard(); self.cut._guard(); self.package.verify_current(self.token)
        source.verify(self.source_root,self.fence.context['commit'])
        row,rev=self.fence.ledger.read()
        for other in (self.cut,self.package):
            if other.fence.ledger.read()!=(row,rev): raise ValueError()
        self._guard()

    def _ca(self):
        self._guard(); identity=public._meta(public.common.linux_private_bytes,self._guard,'secret','voice-media-ca')
        value=public._value(public.common.linux_private_bytes,self._guard,'secret','voice-media-ca','ca.crt')
        public._public_pem(value); digest=hashlib.sha256(crypto.der(value)).hexdigest()
        if public._meta(public.common.linux_private_bytes,self._guard,'secret','voice-media-ca')!=identity: raise ValueError()
        return {'uid':identity['uid'],'resourceVersion':identity['resource_version'],'der_sha256':digest}

    def _creator(self):
        result={}
        for name,keys in metadata.MEMBERS.items():
            raw=public._call(public.common.linux_private_bytes,self._guard,'get','secret',name,
                             '--ignore-not-found=true','-o','go-template='+metadata.TEMPLATE)
            if not raw: result[name]={'state':'absent'}; continue
            lines=raw.decode('ascii').splitlines()
            if (len(lines)<3 or any(not re.fullmatch(r'[A-Za-z0-9.-]{1,128}',x) for x in lines[:2])
                    or lines[2] not in ('','<no value>') or len(lines[3:])!=len(set(lines[3:]))
                    or set(lines[3:])!=keys): raise ValueError()
            result[name]={'state':'present','uid':lines[0],'resourceVersion':lines[1],'keys':sorted(lines[3:])}
        return result

    def prepare(self):
        try:
            self._authority(); ca=self._ca(); creator=self._creator(); self._authority(); row,rev=self.fence.ledger.read()
            if self.step in self.journal.state['steps'] or self.ledger in row: raise ValueError()
            record={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid,
                    'pid':os.getpid(),'start_ticks':common._ticks(),'ca':ca,'creator_secrets':creator}
            self.pending=True; self.journal.state['steps'][self.step]=copy.deepcopy(record); self.journal.save()
            now,version=self.fence.ledger.read()
            if now!=row or version!=rev: raise ValueError()
            now[self.ledger]=copy.deepcopy(record); now['step']=self.step+'-pending'; self.fence.ledger.write(rev,now)
            self.record=copy.deepcopy(record); self._authority()
            directory=SOCKET_ROOT/self.suffix/self.profile; self._manager=backup.private_directory(directory)
            self.dirfd=self._manager.__enter__()
            self.listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            self.listener.bind(str(directory/'custody.sock')); os.chmod('custody.sock',0o600,dir_fd=self.dirfd,follow_symlinks=False)
            info=os.stat('custody.sock',dir_fd=self.dirfd,follow_symlinks=False)
            if not stat.S_ISSOCK(info.st_mode) or info.st_uid!=os.geteuid(): raise ValueError()
            self.inode=(info.st_dev,info.st_ino); self.listener.listen(1); os.fsync(self.dirfd); self._authority()
            if self._ca()!=ca or self._creator()!=creator: raise ValueError()
            row,rev=self.fence.ledger.read()
            if row.get(self.ledger)!=record or self.journal.state['steps'].get(self.step)!=record: raise ValueError()
            ready=record|{'status':'ready','socket_dev':info.st_dev,'socket_ino':info.st_ino}
            self.journal.state['steps'][self.step]=copy.deepcopy(ready); self.journal.save()
            now,version=self.fence.ledger.read()
            if now!=row or version!=rev: raise ValueError()
            now[self.ledger]=copy.deepcopy(ready); now['step']=self.step+'-ready'; self.fence.ledger.write(rev,now)
            self.record=copy.deepcopy(ready); self._authority(); return copy.deepcopy(ready)
        except BaseException:
            if self.pending: self._uncertain()
            self.close(); raise PreflightError('Space custody preflight unavailable') from None

    def finish(self):
        try:
            if self.listener is None or self.record is None or self.record['status']!='ready': raise ValueError()
            info=os.stat('custody.sock',dir_fd=self.dirfd,follow_symlinks=False)
            if ((info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode)
                    or os.getpid()!=self.record['pid'] or common._ticks()!=self.record['start_ticks']): raise ValueError()
            self._authority(); self.listener.settimeout(300); connection,_=self.listener.accept()
            with connection:
                connection.settimeout(180)
                _,uid,_=struct.unpack('3i',connection.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
                if uid!=0: raise ValueError()
                self._authority()
                request={'context':self.fence.context,'namespace_uid':self.namespace_uid,'nonce':self.record['nonce'],
                         'ca':self.record['ca'],'lock':self.lock,'profile':self.profile}
                payload=json.dumps(request,sort_keys=True,separators=(',',':')).encode('ascii')
                if len(payload)>65536 or connection.send(payload)!=len(payload): raise ValueError()
                raw,ancillary,flags,_=connection.recvmsg(4097,1)
                if ancillary or flags & (socket.MSG_TRUNC|socket.MSG_CTRUNC) or len(raw)>4096: raise ValueError()
            response=json.loads(raw,object_pairs_hook=common.channel._unique)
            expected={'schema':'voice-dev-peer-custody-v1','operation_id':self.record['operation_id'],
                      'source_sha':self.record['source_sha'],'namespace_uid':self.namespace_uid,'nonce':self.record['nonce'],
                      'context_sha256':writer_cut.digest(self.fence.context),
                      'lock':{k:self.lock['metadata'][k] for k in ('uid','resourceVersion')},
                      'ca':self.record['ca'],'creator_secrets':self.record['creator_secrets'],
                      'scope':'root-peer-custody-available-not-issued','profile':self.profile}
            if response!=expected or self._ca()!=self.record['ca'] or self._creator()!=self.record['creator_secrets']: raise ValueError()
            self._authority(); row,rev=self.fence.ledger.read(); pending=copy.deepcopy(self.record)
            done=pending|{'status':'done','receipt_sha256':writer_cut.digest(response)}
            local=done|{'receipt':copy.deepcopy(response)}; self.journal.state['steps'][self.step]=copy.deepcopy(local); self.journal.save()
            now,version=self.fence.ledger.read()
            if now!=row or version!=rev or row.get(self.ledger)!=pending: raise ValueError()
            now[self.ledger]=copy.deepcopy(done); now['step']=self.step+'-done'; self.fence.ledger.write(rev,now); self.record=copy.deepcopy(done)
            self._authority(); after,end=self.fence.ledger.read()
            if self._ca()!=pending['ca'] or self._creator()!=pending['creator_secrets']: raise ValueError()
            self._authority(); final,final_rev=self.fence.ledger.read()
            self._local()
            if final!=after or final_rev!=end or self.journal.state['steps'].get(self.step)!=local: raise ValueError()
            return copy.deepcopy(response)
        except BaseException:
            if self.pending: self._uncertain()
            raise PreflightError('Space custody preflight uncertain') from None
        finally: self.close()

    def _uncertain(self):
        try: self.fence.uncertain('Space custody preflight uncertain')
        except BaseException: pass

    def close(self):
        if self.listener is not None: self.listener.close(); self.listener=None
        if self.dirfd is not None:
            try:
                if self.inode is not None:
                    info=os.stat('custody.sock',dir_fd=self.dirfd,follow_symlinks=False)
                    if (info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode): raise ValueError()
                    os.unlink('custody.sock',dir_fd=self.dirfd); os.fsync(self.dirfd)
            except Exception:
                if self.pending: self._uncertain()
                raise PreflightError('Space preflight cleanup uncertain') from None
            finally: self._manager.__exit__(None,None,None); self.dirfd=None


def verify_current(phase,token):
    try:
        if type(phase) is not PeerIssuerPreflightPhase or token!=phase.token: raise ValueError()
        phase._authority(); row,rev=phase.fence.ledger.read(); local=copy.deepcopy(phase.journal.state['steps'][phase.step])
        if (phase.record is None or phase.record.get('status')!='done' or phase._ca()!=phase.record['ca']
                or phase._creator()!=phase.record['creator_secrets']): raise ValueError()
        phase._authority(); after,end=phase.fence.ledger.read()
        phase._local()
        if after!=row or end!=rev or phase.journal.state['steps'][phase.step]!=local: raise ValueError()
        return copy.deepcopy(local['receipt'])
    except Exception: raise PreflightError('Space custody preflight unavailable') from None


def exchange_root_request(phase,token,connection):
    """Send this phase's current public evidence to an authenticated root peer."""
    try:
        if (type(phase) is not PeerIssuerPreflightPhase or connection.family!=socket.AF_UNIX
                or connection.type & socket.SOCK_SEQPACKET!=socket.SOCK_SEQPACKET): raise ValueError()
        _,uid,_=struct.unpack('3i',connection.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
        if uid!=0: raise ValueError()
        receipt=verify_current(phase,token)
        request={'context':copy.deepcopy(phase.fence.context),'namespace_uid':phase.namespace_uid,
                 'lock':copy.deepcopy(phase.lock),'ca':copy.deepcopy(receipt['ca']),
                 'creator_secrets':copy.deepcopy(receipt['creator_secrets']), 'profile':phase.profile,
                 'preflight_sha256':writer_cut.digest(receipt)}
        payload=json.dumps(request,sort_keys=True,separators=(',',':')).encode('ascii')
        if len(payload)>65536 or connection.send(payload)!=len(payload): raise ValueError()
        if verify_current(phase,token)!=receipt: raise ValueError()
        if connection.send(b'PREFLIGHT-READY\n')!=len(b'PREFLIGHT-READY\n'): raise ValueError()
        return copy.deepcopy(receipt)
    except Exception: raise PreflightError('Space custody exchange unavailable') from None
