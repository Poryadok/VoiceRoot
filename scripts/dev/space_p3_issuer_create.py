"""Fenced, missing-only DEV Space signing and HTTPS Secret creation."""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import socket
import stat

import frontend_runtime_executor
import media_operation_guard
import realtime_p3_create_only as common
import realtime_p3_create_only_crypto as crypto
import realtime_p3_create_only_socket as channel
import realtime_p3_create_only_transport as transport
import space_p3_issuer_packet as packet
import space_p3_issuer_source as source
import story_media_backup as backup

SOCKET_PARENT=Path('/home/pmd/voice-space-p3/operations')
ROOTS=(('voice-principal-ca','ca.crt'),('voice-media-ca','ca.crt'))
STEP='space-p3-handoff'
LEDGER='space_p3_handoff'
EFFECTS='space_p3_objects'


class CreateError(RuntimeError): pass


def _get(name):
    if name not in packet.MEMBERS: raise ValueError()
    raw=transport._run(['get','secret',name,'--ignore-not-found=true','-o','json'])
    return None if not raw else json.loads(raw)


def _create(obj,dry):
    if type(dry) is not bool or obj.get('kind')!='Secret' or obj.get('metadata',{}).get('name') not in packet.MEMBERS: raise ValueError()
    payload=json.dumps(obj,separators=(',',':'),ensure_ascii=True).encode('ascii')
    args=['create','-f','-','-o','json']
    if dry: args.insert(1,'--dry-run=server')
    return json.loads(transport._run(args,payload))


class Session:
    def __init__(self,journal,fence,client,lock,namespace_uid):
        self.journal,self.fence,self.client=journal,fence,client
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.intent=copy.deepcopy(journal.intent); self.plan=self.intent.get('space_p3_create')
        self.listener=self.dirfd=self.inode=None; self.handoff=None; self.effects={}; self.roots=None
        self.objects=None; self.envelope=None; self.proof=None; self.workload=None; self.pending=False; self.completed=False
        match=re.fullmatch(r'media-c[12]-([a-f0-9]{32})',fence.context.get('operation_id',''))
        if (not match or type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
            or getattr(fence.ledger,'client',None) is not client or self.intent.get('resource_context')!=fence.context
            or self.intent.get('namespace_uid')!=namespace_uid or type(self.plan) is not dict
            or set(self.plan)!={'source_repo','source_sha','roots'} or self.plan['source_sha']!=fence.context.get('commit')
            or not isinstance(self.plan['source_repo'],str) or not Path(self.plan['source_repo']).is_absolute()
            or type(self.plan['roots']) is not dict or set(self.plan['roots'])!={name for name,_ in ROOTS}): raise CreateError('Space creation identity unavailable')
        for identity in self.plan['roots'].values():
            if (type(identity) is not dict or set(identity)!={'uid','resourceVersion','der_sha256'}
                or any(not isinstance(identity[k],str) or not identity[k] for k in ('uid','resourceVersion'))
                or not isinstance(identity['der_sha256'],str) or not re.fullmatch(r'[a-f0-9]{64}',identity['der_sha256'])): raise CreateError('Space public root identity invalid')
        self.suffix=match[1]

    def _guard(self):
        if self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent: raise ValueError()
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        row,_=self.fence.ledger.read()
        if self.handoff is not None and (row.get(LEDGER)!=self.handoff or self.journal.state['steps'].get(STEP)!=self.handoff): raise ValueError()
        local={k.removeprefix('space-p3-object-'):v for k,v in self.journal.state['steps'].items() if k.startswith('space-p3-object-')}
        if row.get(EFFECTS,{})!=self.effects or local!=self.effects: raise ValueError()
        source.verify(self.plan['source_repo'],self.plan['source_sha'])

    def _public(self):
        result={}
        for name,key in ROOTS:
            template=('{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}'
                      '{{.metadata.deletionTimestamp}}{{"\\n"}}{{index .data "'+key+'"}}')
            self._guard(); raw=transport._run(['get','secret',name,'-o','go-template='+template]); self._guard(); lines=raw.split(b'\n',3)
            if len(lines)!=4 or not lines[0] or not lines[1] or lines[2] not in (b'',b'<no value>') or not lines[3]: raise ValueError()
            cert=base64.b64decode(lines[3],validate=True)
            if not re.fullmatch(rb'-----BEGIN CERTIFICATE-----\r?\n(?:[A-Za-z0-9+/=]+\r?\n)+-----END CERTIFICATE-----\r?\n?',cert): raise ValueError()
            identity={'uid':lines[0].decode('ascii'),'resourceVersion':lines[1].decode('ascii'),
                      'der_sha256':hashlib.sha256(crypto.der(cert)).hexdigest()}
            if identity!=self.plan['roots'][name] or b'CA:TRUE' not in crypto._call(['x509','-in','{0}','-noout','-ext','basicConstraints'],[cert]): raise ValueError()
            result[name]={'identity':identity,'certificate':cert}
        return result

    def _workload(self):
        self._guard(); obj=self.client.call('get','deployment','voice-space','-o','json'); self._guard()
        meta=obj['metadata']; spec=obj['spec']
        if (obj.get('apiVersion')!='apps/v1' or obj.get('kind')!='Deployment' or meta.get('name')!='voice-space'
            or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp') or not meta.get('uid')
            or type(meta.get('generation')) is not int or meta['generation']<1): raise ValueError()
        def contains(value):
            if isinstance(value,dict): return any(contains(k) or contains(v) for k,v in value.items())
            if isinstance(value,list): return any(contains(v) for v in value)
            return isinstance(value,str) and value in packet.MEMBERS
        if contains(spec): raise ValueError()
        return {'uid':meta['uid'],'generation':meta['generation'],
                'spec_sha256':hashlib.sha256(json.dumps(spec,sort_keys=True,separators=(',',':')).encode()).hexdigest()}

    def _check_workload(self):
        if self._workload()!=self.workload: raise ValueError()

    def _check_roots(self):
        if self._public()!=self.roots: raise ValueError()

    def _check_created(self):
        for name,record in self.effects.items():
            if record['status']!='done': continue
            self._guard(); current=_get(name); self._guard()
            if not common._same_object(current,self.objects[name]) or any(current['metadata'][k]!=record[k] for k in ('uid','resourceVersion')): raise ValueError()

    def _write(self,*,handoff=None,effects=None):
        self._guard(); row,rev=self.fence.ledger.read()
        if handoff is not None: self.journal.state['steps'][STEP]=copy.deepcopy(handoff)
        if effects is not None:
            for name,value in effects.items(): self.journal.state['steps']['space-p3-object-'+name]=copy.deepcopy(value)
        self.journal.save(); current,version=self.fence.ledger.read()
        if current!=row or version!=rev: raise ValueError()
        if handoff is not None: current[LEDGER]=copy.deepcopy(handoff)
        if effects is not None: current[EFFECTS]=copy.deepcopy(effects)
        current['step']=STEP; self.fence.ledger.write(rev,current)
        if handoff is not None: self.handoff=copy.deepcopy(handoff)
        if effects is not None: self.effects=copy.deepcopy(effects)
        self._guard()

    def _poison(self):
        try: self.fence.uncertain('Space create-only outcome uncertain')
        except BaseException: pass

    def _proof(self,jwks):
        signing=self.effects['voice-space-principal-signing']; tls=self.effects['voice-space-media-server-tls']
        if any(record.get('status')!='done' for record in (signing,tls)): raise ValueError()
        cert=base64.b64decode(self.objects['voice-space-media-server-tls']['data']['tls.crt'],validate=True)
        return {'schema':'voice-dev-space-p3-issuer-v1','operation_id':self.fence.context['operation_id'],
                'source_sha':self.plan['source_sha'],'namespace_uid':self.namespace_uid,
                'signer':{k:signing[k] for k in ('uid','resourceVersion')},
                'tls':{k:tls[k] for k in ('uid','resourceVersion')}|{'certificate_sha256':hashlib.sha256(crypto.der(cert)).hexdigest()},
                'media_ca':copy.deepcopy(self.roots['voice-media-ca']['identity']),
                'public_jwks':copy.deepcopy(jwks),'scope':'space-two-secret-create-only'}

    def prepare(self):
        try:
            self._guard(); self.roots=self._public(); self.workload=self._workload()
            for name in sorted(packet.MEMBERS):
                self._guard()
                if _get(name) is not None: raise ValueError()
            row,_=self.fence.ledger.read()
            if LEDGER in row or STEP in self.journal.state['steps'] or EFFECTS in row: raise ValueError()
            initial={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                     'source_sha':self.plan['source_sha'],'namespace_uid':self.namespace_uid,'pid':os.getpid(),
                     'start_ticks':common._ticks(),'workload':copy.deepcopy(self.workload)}
            self.pending=True; self._write(handoff=initial)
            directory=SOCKET_PARENT/self.suffix; manager=backup.private_directory(directory)
            self.dirfd=manager.__enter__(); self._dir_manager=manager
            if os.geteuid()!=1000: raise ValueError()
            self.listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            self.listener.bind(str(directory/'packet.sock')); os.chmod('packet.sock',0o600,dir_fd=self.dirfd,follow_symlinks=False)
            info=os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
            if not stat.S_ISSOCK(info.st_mode) or info.st_uid!=os.geteuid(): raise ValueError()
            self.inode=(info.st_dev,info.st_ino); self.listener.listen(1); os.fsync(self.dirfd); self._guard(); self._check_roots(); self._check_workload()
            ready=initial|{'status':'ready','socket_dev':info.st_dev,'socket_ino':info.st_ino}
            self._write(handoff=ready); return copy.deepcopy(ready)
        except BaseException:
            if self.pending: self._poison()
            self.close(); raise CreateError('Space handoff unavailable; reconcile if reserved') from None

    def finish(self):
        try:
            if self.listener is None or self.handoff is None or self.handoff['status']!='ready': raise ValueError()
            self._guard(); info=os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
            if ((info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode)
                or os.getpid()!=self.handoff['pid'] or common._ticks()!=self.handoff['start_ticks']): raise ValueError()
            envelope=channel.receive(self.listener,self.fence.context['operation_id'],source.ISSUER_SHA); self.envelope=copy.deepcopy(envelope)
            self._guard(); self._check_roots(); self._check_workload()
            self.objects,jwks=packet.validate(envelope,self.fence.context['operation_id'],source.ISSUER_SHA,self.roots['voice-media-ca']['certificate'])
            for name in sorted(packet.MEMBERS):
                self._guard(); self._check_roots(); self._check_workload()
                if _get(name) is not None or not common._same_object(_create(self.objects[name],True),self.objects[name],False): raise ValueError()
            for name in sorted(packet.MEMBERS):
                self._guard(); self._check_roots(); self._check_workload(); self._check_created()
                if _get(name) is not None: raise ValueError()
                pending={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                         'kind':'Secret','name':name,'keys':sorted(packet.MEMBERS[name])}
                self._write(effects=self.effects|{name:pending}); self._guard(); self._check_roots(); self._check_workload(); self._check_created()
                response=_create(self.objects[name],False); actual=_get(name)
                if (not common._same_object(response,self.objects[name]) or not common._same_object(actual,self.objects[name])
                    or any(response['metadata'][k]!=actual['metadata'][k] for k in ('uid','resourceVersion'))): raise ValueError()
                self._write(effects=self.effects|{name:pending|{'status':'done','uid':actual['metadata']['uid'],
                                                                  'resourceVersion':actual['metadata']['resourceVersion']}})
                self._check_created()
            self._guard(); self._check_roots(); self._check_workload(); self._check_created()
            self.proof=self._proof(jwks)
            self._write(handoff=self.handoff|{'status':'done','receipt':copy.deepcopy(self.proof)})
            self._check_roots(); self._check_created(); self._guard(); self.completed=True
            return copy.deepcopy(self.proof)
        except BaseException:
            if self.pending and not self.completed: self._poison()
            raise CreateError('Space creation uncertain; retain lock and reconcile') from None
        finally: self.close()

    def close(self):
        if self.listener is not None: self.listener.close(); self.listener=None
        if self.dirfd is not None:
            try:
                if self.inode is not None:
                    info=os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
                    if (info.st_dev,info.st_ino)!=self.inode or not stat.S_ISSOCK(info.st_mode): raise ValueError()
                    os.unlink('packet.sock',dir_fd=self.dirfd); os.fsync(self.dirfd)
            except Exception:
                if self.pending: self._poison()
                raise CreateError('Space socket cleanup uncertain; reconcile') from None
            finally: self._dir_manager.__exit__(None,None,None); self.dirfd=None


def verify_completed(phase):
    try:
        if type(phase) is not Session or not phase.completed or phase.handoff.get('status')!='done' or phase.handoff.get('receipt')!=phase.proof: raise ValueError()
        phase._guard(); phase._check_roots(); phase._check_created()
        objects,jwks=packet.validate(phase.envelope,phase.fence.context['operation_id'],source.ISSUER_SHA,
                                     phase.roots['voice-media-ca']['certificate'])
        if objects!=phase.objects or phase._proof(jwks)!=phase.proof: raise ValueError()
        phase._guard(); phase._check_roots(); phase._check_created(); phase._guard()
        return copy.deepcopy(phase.proof)
    except Exception:
        if type(phase) is Session and phase.pending: phase._poison()
        raise CreateError('Space creation proof unavailable') from None
