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

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or
                self.intent.get('resource_context') != self.fence.context or
                self.intent.get('namespace_uid') != self.namespace_uid or
                (hasattr(self.fence.ledger, 'client') and self.fence.ledger.client is not self.client)):
            raise RecoveryRequired('operation identity changed')
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)

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
            self._guard()
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
            return receipt
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('realtime Secret outcome uncertain')
                except BaseException: pass
            raise RecoveryRequired('realtime Secret not admitted; reconcile if pending') from None
