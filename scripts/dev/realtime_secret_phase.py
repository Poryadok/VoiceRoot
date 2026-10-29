"""Create the operation-owned Realtime credential under the shared DEV fence.

No Secret payload is persisted in the journal or ledger. A lost effect requires
reconciliation; an existing object is never adopted. The caller owns private
password custody and must bind the returned receipt to the provision journal.
"""
import base64
import copy
import hashlib
import re
import secrets

import media_operation_guard


class RecoveryRequired(RuntimeError):
    pass


def valid_revision(revision):
    return (isinstance(revision, tuple) and len(revision) == 2 and
            all(isinstance(value, str) and value for value in revision))


class SecretPhase:
    def __init__(self, journal, fence, client, lock, namespace_uid):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.intent = copy.deepcopy(journal.intent)
        self.poisoned = False
        self.cut = None
        self._cut_refs = None
        self._cut_custody = None

    def bind_cut(self, cut):
        """Bind the exact drained writer cut before a C1 Secret attempt."""
        import writer_cut
        import space_native_tool_identity as custody
        if (self.cut is not None or type(cut) is not writer_cut.WriterCut or cut.fence.context!=self.fence.context or
                cut.fence.check is not self.fence.check or cut.fence.on_uncertain is not self.fence.on_uncertain or
                cut.lock!=self.lock or cut.namespace_uid!=self.namespace_uid):
            raise RecoveryRequired('writer cut identity invalid')
        self._guard(); cut.verify_drained()
        if cut.fence.ledger.read()!=self.fence.ledger.read():raise RecoveryRequired('writer cut ledger differs')
        self.cut=cut; self._cut_refs=(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger,
                                     self.journal,self.fence,self.client,self.fence.ledger,self.fence.check,self.fence.on_uncertain)
        self._cut_custody=(custody.SpaceNativeToolPhase._custody(self.journal),custody.SpaceNativeToolPhase._custody(cut.journal))
        self._before_effect()

    def _before_effect(self):
        import writer_cut
        import space_native_tool_identity as custody
        if self.fence.context['operation_id'].startswith('media-c1-') or self.cut is not None:
            cut=self.cut
            if (type(cut) is not writer_cut.WriterCut or self._cut_refs is None or
                    any(a is not b for a,b in zip(self._cut_refs,(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger,
                        self.journal,self.fence,self.client,self.fence.ledger,self.fence.check,self.fence.on_uncertain))) or
                    cut.fence.check is not self.fence.check or cut.fence.on_uncertain is not self.fence.on_uncertain or
                    self._cut_custody!=(custody.SpaceNativeToolPhase._custody(self.journal),custody.SpaceNativeToolPhase._custody(cut.journal)) or
                    cut.fence.context!=self.fence.context or cut.lock!=self.lock or cut.namespace_uid!=self.namespace_uid):
                raise RecoveryRequired('writer cut unavailable')
            own=copy.deepcopy(self.journal.state)
            self._guard(); cut.verify_drained(); cut_state=copy.deepcopy(cut.journal.state)
            if cut.fence.ledger.read()!=self.fence.ledger.read():raise RecoveryRequired('writer cut ledger differs')
            self._guard()
            if (self.cut is not cut or any(a is not b for a,b in zip(self._cut_refs,(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger,
                    self.journal,self.fence,self.client,self.fence.ledger,self.fence.check,self.fence.on_uncertain))) or
                    cut.fence.check is not self.fence.check or cut.fence.on_uncertain is not self.fence.on_uncertain or
                    cut.fence.context!=self.fence.context or self.fence.context!=self.intent.get('resource_context') or
                    self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                    cut.lock!=self.lock or cut.namespace_uid!=self.namespace_uid or
                    self._cut_custody!=(custody.SpaceNativeToolPhase._custody(self.journal),custody.SpaceNativeToolPhase._custody(cut.journal)) or
                    self.journal.state!=own or cut.journal.state!=cut_state):
                raise RecoveryRequired('writer cut evidence changed')

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or
                self.intent.get('resource_context') != self.fence.context or
                self.intent.get('namespace_uid') != self.namespace_uid or
                (hasattr(self.fence.ledger, 'client') and self.fence.ledger.client is not self.client)):
            raise RecoveryRequired('operation identity changed')
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)

    def _cut_local(self):
        import writer_cut
        import space_native_tool_identity as custody
        if self.fence.context['operation_id'].startswith('media-c1-') or self.cut is not None:
            cut=self.cut
            if (type(cut) is not writer_cut.WriterCut or self._cut_refs is None or
                    any(a is not b for a,b in zip(self._cut_refs,(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger,
                        self.journal,self.fence,self.client,self.fence.ledger,self.fence.check,self.fence.on_uncertain))) or
                    cut.fence.context!=self.fence.context or self.fence.context!=self.intent.get('resource_context') or
                    self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                    cut.lock!=self.lock or cut.namespace_uid!=self.namespace_uid or
                    cut.fence.check is not self.fence.check or cut.fence.on_uncertain is not self.fence.on_uncertain or
                    self.poisoned or self.journal.poisoned or cut.journal.poisoned or
                    self._cut_custody!=(custody.SpaceNativeToolPhase._custody(self.journal),custody.SpaceNativeToolPhase._custody(cut.journal))):
                raise RecoveryRequired('writer cut custody changed')

    def run(self, password):
        pending = False
        try:
            if not isinstance(password, bytes) or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}', password):
                raise RecoveryRequired('credential format invalid')
            operation = self.fence.context['operation_id']
            match = re.fullmatch(r'media-c[12]-([0-9a-f]{32})', operation)
            if not match: raise RecoveryRequired('operation invalid')
            name = 'voice-realtime-db-' + match.group(1)
            password_sha256 = hashlib.sha256(password).hexdigest()
            nonce = secrets.token_hex(16)
            record = {'status':'pending','operation_id':operation,'nonce':nonce,'name':name}
            self._guard(); self._before_effect()
            if 'realtime-secret' in self.journal.state['steps']:
                raise RecoveryRequired('attempt already recorded')
            pending = True
            self.journal.state['steps']['realtime-secret'] = copy.deepcopy(record)
            self.journal.save()
            row, revision = self.fence.ledger.read()
            if not self.fence.owned(row) or row.get('status') != 'active' or 'realtime_secret' in row:
                raise RecoveryRequired('secret operation already reserved')
            row['realtime_secret'] = copy.deepcopy(record)
            self.fence.ledger.write(revision,row)
            self._guard()
            row, revision = self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status') != 'active' or
                    row.get('realtime_secret') != record or not valid_revision(revision)):
                raise RecoveryRequired('reservation changed')
            url = b'postgres://realtime:' + password + b'@voice-postgres:5432/realtime_db?sslmode=disable'
            obj = {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-dev'},
                   'immutable':True,'type':'Opaque','data':{'DATABASE_URL':base64.b64encode(url).decode('ascii')}}
            self._before_effect()
            created = self.client.call('create','-f','-','-o','json',body=obj)
            self._guard()
            current = self.client.call('get','secret',name,'-o','json')
            meta = created.get('metadata',{})
            if (created != current or meta.get('name') != name or meta.get('namespace') != 'voice-dev' or
                    not all(isinstance(meta.get(k),str) and meta[k] for k in ('uid','resourceVersion')) or
                    created.get('immutable') is not True or created.get('type') != 'Opaque' or
                    created.get('data') != obj['data'] or meta.get('deletionTimestamp')):
                raise RecoveryRequired('created Secret differs')
            receipt = {'name':name,'uid':meta['uid'],'resourceVersion':meta['resourceVersion'],
                       'password_sha256':password_sha256}
            self._guard()
            row, revision = self.fence.ledger.read()
            if not self.fence.owned(row) or row.get('status') != 'active' or row.get('realtime_secret') != record:
                raise RecoveryRequired('secret ownership changed')
            done = record | {'status':'done','receipt':{k:receipt[k] for k in ('name','uid','resourceVersion')}}
            self.journal.state['steps']['realtime-secret'] = copy.deepcopy(done) | {'password_sha256':password_sha256}
            self.journal.save()
            row['realtime_secret'] = copy.deepcopy(done)
            self.fence.ledger.write(revision,row)
            self._guard()
            confirmed, revision = self.fence.ledger.read()
            if (not self.fence.owned(confirmed) or confirmed.get('status') != 'active' or
                    confirmed.get('realtime_secret') != done or not valid_revision(revision)):
                raise RecoveryRequired('secret receipt changed')
            self._cut_local()
            if self.journal.state['steps'].get('realtime-secret')!=done | {'password_sha256':password_sha256}:
                raise RecoveryRequired('secret receipt changed')
            return receipt
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('realtime Secret outcome uncertain')
                except BaseException: pass
            raise RecoveryRequired('realtime Secret not admitted; reconcile if pending') from None
