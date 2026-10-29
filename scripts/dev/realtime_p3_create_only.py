"""One fenced, create-only P3 handoff and exact seven-object application."""
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
import realtime_p3_create_only_packet as packet
import realtime_p3_create_only_socket as channel
import realtime_p3_create_only_transport as transport
import space_native_tool_identity as native
import story_media_backup as backup

SOCKET_PARENT = Path('/home/pmd/voice-realtime-p3/operations')
PUBLIC = (('voice-principal-ca','ca.crt'),('voice-media-ca','ca.crt'),
          ('voice-space-media-server-tls','tls.crt'))


class CreateError(RuntimeError): pass


def _ticks():
    raw = Path('/proc/self/stat').read_text()
    fields = raw.rsplit(')',1)[1].split()
    result = fields[19]
    if not re.fullmatch(r'[0-9]+',result): raise ValueError()
    return result


def _same_object(actual, desired, require_identity=True):
    if not isinstance(actual,dict) or actual.get('apiVersion') != 'v1' or actual.get('kind') != desired['kind']: return False
    meta = actual.get('metadata',{})
    if (meta.get('name') != desired['metadata']['name'] or meta.get('namespace') != 'voice-dev'
            or meta.get('deletionTimestamp')): return False
    if require_identity and (not isinstance(meta.get('uid'),str) or not meta['uid']
            or not isinstance(meta.get('resourceVersion'),str) or not meta['resourceVersion']): return False
    if desired['kind'] == 'Secret':
        if actual.get('type') != 'Opaque' or actual.get('immutable',False) is not False: return False
        try:
            got = {k:base64.b64decode(v,validate=True) for k,v in actual.get('data',{}).items()}
            wanted = {k:base64.b64decode(v,validate=True) for k,v in desired['data'].items()}
        except Exception: return False
        return got == wanted
    return actual.get('immutable') is True and actual.get('data') == desired['data'] and not actual.get('binaryData')


class Session:
    def __init__(self, journal, fence, client, lock, namespace_uid):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.intent = copy.deepcopy(journal.intent); self.listener = None; self.dirfd = None; self.inode = None
        self.pending = False; self.handoff = None; self.effects = {}; self.roots = None; self.completed = False
        self._delegated_cut = None; self._delegated_refs = None
        self.plan = self.intent.get('realtime_p3_create')
        match = re.fullmatch(r'media-c[12]-([0-9a-f]{32})',fence.context.get('operation_id',''))
        if (not match or self.intent.get('resource_context') != fence.context or self.intent.get('namespace_uid') != namespace_uid
                or type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
                or getattr(fence.ledger,'client',None) is not client or not isinstance(self.plan,dict)
                or set(self.plan) != {'issuer_source_sha','source_repo','issuer_suffix'}
                or self.plan['issuer_suffix'] != match[1]
                or not isinstance(self.plan['issuer_source_sha'],str) or not re.fullmatch(r'[0-9a-f]{40}',self.plan['issuer_source_sha'])
                or not isinstance(self.plan['source_repo'],str) or not Path(self.plan['source_repo']).is_absolute()):
            raise CreateError('P3 creation identity unavailable')
        self.suffix = match[1]
        self._refs=(journal,fence,client,fence.ledger)
        self._identity=(copy.deepcopy(fence.context),copy.deepcopy(lock),namespace_uid)
        self._custody=native.SpaceNativeToolPhase._custody(journal)

    def _local(self):
        if (any(a is not b for a,b in zip(self._refs,(self.journal,self.fence,self.client,self.fence.ledger)))
                or self.client.journal is not self.journal or self.fence.ledger.client is not self.client
                or (self.fence.context,self.lock,self.namespace_uid)!=self._identity
                or self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent
                or native.SpaceNativeToolPhase._custody(self.journal)!=self._custody): raise ValueError()

    def _guard(self):
        self._local()
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        if self._delegated_refs is not None and self._delegated_cut is None: raise ValueError()
        cut_snapshot=cut_custody=None
        if self._delegated_cut is not None:
            cut=self._delegated_cut
            if (self._delegated_refs is None or any(a is not b for a,b in zip(self._delegated_refs,(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger)))
                    or cut.fence.context!=self.fence.context or cut.lock!=self.lock or cut.namespace_uid!=self.namespace_uid): raise ValueError()
            cut_snapshot=copy.deepcopy((cut.journal.intent,cut.journal.state))
            cut_custody=native.SpaceNativeToolPhase._custody(cut.journal)
            cut.verify_drained()
            if self._delegated_refs is None or any(a is not b for a,b in zip(self._delegated_refs,(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger))): raise ValueError()
        row, rev = self.fence.ledger.read()
        if cut_snapshot is not None and cut.fence.ledger.read()!=(row,rev): raise ValueError()
        if self.handoff is not None and (row.get('realtime_p3_handoff') != self.handoff
                or self.journal.state['steps'].get('realtime-p3-handoff') != self.handoff): raise ValueError()
        local = {k.removeprefix('realtime-p3-object-'):v for k,v in self.journal.state['steps'].items()
                 if k.startswith('realtime-p3-object-')}
        if row.get('realtime_p3_objects',{}) != self.effects or local != self.effects: raise ValueError()
        self._local()
        if cut_snapshot is not None and (self._delegated_cut is not cut or
                self._delegated_refs is None or any(a is not b for a,b in zip(self._delegated_refs,(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger))) or
                cut.fence.context!=self.fence.context or cut.lock!=self.lock or cut.namespace_uid!=self.namespace_uid or
                (cut.journal.intent,cut.journal.state)!=cut_snapshot or
                native.SpaceNativeToolPhase._custody(cut.journal)!=cut_custody): raise ValueError()

    def _poison(self):
        try: self.fence.uncertain('P3 create-only outcome uncertain')
        except BaseException: pass

    def _public(self):
        result = {}
        for name,key in PUBLIC:
            self._guard(); raw = transport.public_certificate(name,key); self._guard()
            lines = raw.split(b'\n',2)
            if len(lines) != 3 or not lines[0] or not lines[1] or not lines[2]: raise ValueError()
            cert = base64.b64decode(lines[2],validate=True)
            if not cert or len(cert)>131072: raise ValueError()
            result[(name,key)] = {'uid':lines[0].decode('ascii'),'resourceVersion':lines[1].decode('ascii'),'certificate':cert}
        return result

    def _check_roots(self):
        if self._public() != self.roots: raise ValueError()

    def _check_created(self,objects):
        for record in self.effects.values():
            if record['status'] != 'done': continue
            self._guard(); current = transport.get(record['kind'],record['name']); self._guard()
            if (not _same_object(current,objects[record['kind'],record['name']])
                    or any(current['metadata'][k] != record[k] for k in ('uid','resourceVersion'))): raise ValueError()

    def _reserve(self, step, value):
        self._guard(); row, rev = self.fence.ledger.read()
        if row.get('realtime_p3_objects',{}) != self.effects or step in self.effects: raise ValueError()
        self.effects[step] = copy.deepcopy(value)
        self.journal.state['steps']['realtime-p3-object-'+step] = copy.deepcopy(value); self.journal.save()
        current, actual = self.fence.ledger.read()
        if current != row or actual != rev: raise ValueError()
        current['realtime_p3_objects'] = copy.deepcopy(self.effects); current['step'] = 'realtime-p3-object-'+step
        self.fence.ledger.write(rev,current); self._guard()

    def _complete(self, step, value):
        self._guard(); row, rev = self.fence.ledger.read()
        self.effects[step] = copy.deepcopy(value)
        self.journal.state['steps']['realtime-p3-object-'+step] = copy.deepcopy(value); self.journal.save()
        row['realtime_p3_objects'] = copy.deepcopy(self.effects); row['step'] = 'realtime-p3-object-'+step+'-done'
        self.fence.ledger.write(rev,row); self._guard()

    def prepare(self):
        """Reserve the one-shot root handoff only after all current inputs qualify."""
        try:
            self._guard()
            from realtime_p3_create_only_source import verify as verify_source
            verify_source(self.plan['source_repo'],self.plan['issuer_source_sha'],self.fence.context['commit'])
            self.roots = self._public()
            for kind,name in sorted(packet.MEMBERS):
                self._guard()
                if transport.get(kind,name) is not None: raise ValueError()
            self._guard(); row, rev = self.fence.ledger.read()
            if 'realtime_p3_handoff' in row or 'realtime-p3-handoff' in self.journal.state['steps']: raise ValueError()
            intent = {'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                'issuer_suffix':self.suffix,'issuer_source_sha':self.plan['issuer_source_sha'],
                'candidate_source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid,
                'pid':os.getpid(),'start_ticks':_ticks()}
            self.pending = True; self.handoff = copy.deepcopy(intent)
            self.journal.state['steps']['realtime-p3-handoff'] = copy.deepcopy(intent); self.journal.save()
            current, actual = self.fence.ledger.read()
            if current != row or actual != rev: raise ValueError()
            current['realtime_p3_handoff'] = copy.deepcopy(intent); current['step'] = 'realtime-p3-handoff-pending'
            self.fence.ledger.write(rev,current); self._guard()
            directory = SOCKET_PARENT/self.suffix
            manager = backup.private_directory(directory); self.dirfd = manager.__enter__(); self._dir_manager = manager
            if os.geteuid() != 1000: raise ValueError()
            self.listener = socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            self.listener.bind(str(directory/'packet.sock')); os.chmod('packet.sock',0o600,dir_fd=self.dirfd,follow_symlinks=False)
            info = os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
            if not stat.S_ISSOCK(info.st_mode) or info.st_uid != os.geteuid(): raise ValueError()
            self.inode = (info.st_dev,info.st_ino); self.listener.listen(1); os.fsync(self.dirfd); self._guard()
            ready = intent | {'status':'ready','socket_dev':info.st_dev,'socket_ino':info.st_ino}
            row, rev = self.fence.ledger.read()
            if row.get('realtime_p3_handoff') != self.handoff: raise ValueError()
            self.journal.state['steps']['realtime-p3-handoff'] = copy.deepcopy(ready); self.journal.save()
            current, actual = self.fence.ledger.read()
            if current != row or actual != rev: raise ValueError()
            current['realtime_p3_handoff'] = copy.deepcopy(ready); current['step'] = 'realtime-p3-handoff-ready'
            self.fence.ledger.write(rev,current); self.handoff = copy.deepcopy(ready); self._guard()
            return copy.deepcopy(ready)
        except BaseException:
            if self.pending: self._poison()
            self.close()
            raise CreateError('P3 handoff not ready; reconcile if reserved') from None

    def finish(self):
        """Receive root packet, dry-run, then create and observe each fixed object."""
        return self._finish(False,None,None)

    def finish_delegated(self,capability,token):
        """Use the fixed delegated issuer; its terminal success precedes all effects."""
        return self._finish(True,capability,token)

    def _finish(self,delegated,capability,token):
        try:
            if self.listener is None or self.handoff is None: raise ValueError()
            self._guard()
            info = os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
            if (not stat.S_ISSOCK(info.st_mode) or (info.st_dev,info.st_ino) != self.inode
                    or os.getpid() != self.handoff['pid'] or _ticks() != self.handoff['start_ticks']): raise ValueError()
            if not delegated:
                envelope = channel.receive(self.listener,self.fence.context['operation_id'],self.plan['issuer_source_sha'])
            else:
                from realtime_p3_delegated_receiver import receive
                envelope = receive(self,capability,token)
            self._guard(); self._check_roots()
            principal = self.roots[PUBLIC[0]]['certificate']; media = self.roots[PUBLIC[1]]['certificate']; space = self.roots[PUBLIC[2]]['certificate']
            objects = packet.validate(envelope,self.fence.context['operation_id'],self.plan['issuer_source_sha'],principal,media,space)
            for identity in sorted(packet.MEMBERS):
                self._guard(); self._check_roots()
                if transport.get(*identity) is not None: raise ValueError()
                result = transport.create(objects[identity],True)
                if not _same_object(result,objects[identity],require_identity=False): raise ValueError()
            for kind,name in sorted(packet.MEMBERS):
                self._guard(); self._check_roots()
                if transport.get(kind,name) is not None: raise ValueError()
                step = kind.lower()+'-'+name
                pending = {'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                           'kind':kind,'name':name,'keys':sorted(packet.MEMBERS[kind,name])}
                self._reserve(step,pending); self._guard(); self._check_roots(); self._check_created(objects)
                response = transport.create(objects[kind,name],False)
                actual = transport.get(kind,name)
                if not _same_object(response,objects[kind,name]) or not _same_object(actual,objects[kind,name]): raise ValueError()
                if any(response['metadata'][k] != actual['metadata'][k] for k in ('uid','resourceVersion')): raise ValueError()
                done = pending | {'status':'done','uid':actual['metadata']['uid'],'resourceVersion':actual['metadata']['resourceVersion']}
                self._complete(step,done)
                self._check_created(objects)
            self._guard(); self._check_roots(); self._check_created(objects)
            public_receipt = {name:{'uid':self.roots[(name,key)]['uid'],
                'resourceVersion':self.roots[(name,key)]['resourceVersion'],
                'der_sha256':hashlib.sha256(__import__('realtime_p3_create_only_crypto').der(self.roots[(name,key)]['certificate'])).hexdigest()}
                for name,key in PUBLIC}
            receipt = {'operation_id':self.fence.context['operation_id'],'issuer_source_sha':self.plan['issuer_source_sha'],
                'candidate_source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid,
                'objects':copy.deepcopy(self.effects),'public_sources':public_receipt,'scope':'p3-create-only'}
            done = self.handoff | {'status':'done','receipt':copy.deepcopy(receipt)}
            row, rev = self.fence.ledger.read()
            if row.get('realtime_p3_handoff') != self.handoff: raise ValueError()
            self.journal.state['steps']['realtime-p3-handoff'] = copy.deepcopy(done); self.journal.save()
            current, actual = self.fence.ledger.read()
            if current != row or actual != rev: raise ValueError()
            current['realtime_p3_handoff'] = copy.deepcopy(done); current['step'] = 'realtime-p3-handoff-done'
            self.fence.ledger.write(rev,current); self.handoff = copy.deepcopy(done); self._guard()
            self._check_roots(); self._check_created(objects); self._guard()
            self.completed = True; return receipt
        except BaseException:
            if self.pending and not self.completed: self._poison()
            raise CreateError('P3 creation uncertain; retain lock and reconcile') from None
        finally:
            self.close()

    def close(self):
        if self.listener is not None: self.listener.close(); self.listener = None
        if self.dirfd is not None:
            try:
                if self.inode is not None:
                    info = os.stat('packet.sock',dir_fd=self.dirfd,follow_symlinks=False)
                    if self.inode != (info.st_dev,info.st_ino) or not stat.S_ISSOCK(info.st_mode): raise ValueError()
                    os.unlink('packet.sock',dir_fd=self.dirfd); os.fsync(self.dirfd)
            except Exception:
                if self.pending: self._poison()
                raise CreateError('P3 socket cleanup uncertain; retain lock and reconcile') from None
            finally:
                self._dir_manager.__exit__(None,None,None); self.dirfd = None
