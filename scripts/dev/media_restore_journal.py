"""Fence owned isolated restore resource attempts; never adopt unknown resources."""
import copy
import re
import secrets
from datetime import datetime


class RestoreJournalError(RuntimeError):
    pass


class RestoreJournal:
    def __init__(self, phases):
        self.phases = phases

    def _fail(self):
        self.phases.poisoned = True
        try:
            self.phases.fence.uncertain('isolated restore resource outcome uncertain')
        except BaseException:
            pass
        raise RestoreJournalError('retain fence and reconcile restore resources') from None

    def _transition(self, kind, expected, target):
        started = False
        try:
            if kind not in ('postgres', 'migrate'):
                raise RestoreJournalError('unknown resource kind')
            p = self.phases
            p._guard()
            row, revision = p.fence.ledger.read()
            readback = p.journal.state['steps'].get('backup-readback')
            receipt = readback.get('receipt') if isinstance(readback, dict) else None
            readback_intent = readback.get('intent') if isinstance(readback, dict) else None
            upload = row.get('backup_upload')
            local_upload = p.journal.state['steps'].get('backup-upload')
            records = row.get('restore_resources', {})
            name = 'restore-resource-' + kind
            local = p.journal.state['steps'].get(name)
            if (not p.fence.owned(row) or row.get('status') != 'active' or not p._owned_invocation(row) or
                    not isinstance(readback, dict) or readback.get('status') != 'done' or
                    row.get('backup_readback') != readback or set(readback) != {'status', 'intent', 'receipt'} or
                    not isinstance(readback_intent, dict) or set(readback_intent) != {'status','artifact_id','binding'} or
                    readback_intent.get('status') != 'pending' or not isinstance(readback_intent.get('binding'), dict) or
                    set(readback_intent['binding']) != {'operation_id','snapshot_id','challenge','run_id',
                                                       'head_sha','created_at','bundle_bytes','bundle_sha256'} or
                    type(readback_intent.get('artifact_id')) is not int or
                    type(readback_intent['binding'].get('run_id')) is not int or
                    type(readback_intent['binding'].get('bundle_bytes')) is not int or
                    not isinstance(receipt, dict) or set(receipt) != {'schema', 'destination', 'artifact_id',
                        'operation_id', 'snapshot_id', 'challenge', 'run_id', 'head_sha', 'bundle_bytes', 'bundle_sha256'} or
                    receipt.get('schema') != 'voice-media-custody-v1' or
                    receipt.get('destination') != 'github:Poryadok/VoiceRoot' or
                    type(receipt.get('artifact_id')) is not int or receipt['artifact_id'] <= 0 or
                    receipt['artifact_id'] != readback_intent.get('artifact_id') or
                    not isinstance(upload, dict) or upload.get('status') != 'upload-running' or
                    type(upload.get('run_attempt')) is not int or upload.get('run_attempt') != 1 or
                    not isinstance(local_upload, dict) or
                    local_upload.get('status') != 'dispatch-pending' or
                    any(upload.get(k) != v for k, v in local_upload.items() if k != 'status') or
                    type(receipt.get('run_id')) is not int or str(receipt['run_id']) != upload.get('run_id') or
                    receipt.get('head_sha') != p.fence.context.get('commit') or
                    receipt.get('head_sha') != upload.get('source_sha') or
                    type(receipt.get('bundle_bytes')) is not int or
                    any(receipt.get(k) != upload.get(k) for k in
                        ('operation_id', 'snapshot_id', 'challenge', 'bundle_bytes', 'bundle_sha256')) or
                    receipt.get('operation_id') != p.fence.context.get('operation_id') or
                    {k: v for k, v in readback_intent['binding'].items() if k != 'created_at'} !=
                        {k: v for k, v in receipt.items() if k not in ('schema', 'destination', 'artifact_id')} or
                    not isinstance(records, dict) or
                    local != expected or records.get(kind) != expected):
                raise RestoreJournalError('restore resource state changed')
            created = readback_intent['binding']['created_at']
            if not isinstance(created, str) or datetime.fromisoformat(created.replace('Z', '+00:00')).tzinfo is None:
                raise RestoreJournalError('readback timestamp invalid')
            started = True
            p.journal.state['steps'][name] = copy.deepcopy(target)
            p.journal.save()
            current, actual = p.fence.ledger.read()
            if actual != revision or current != row:
                raise RestoreJournalError('restore ledger changed')
            current.setdefault('restore_resources', {})[kind] = copy.deepcopy(target)
            current['step'] = name + '-' + target['status']
            p.fence.ledger.write(actual, current)
            p._guard()
            checked, checked_revision = p.fence.ledger.read()
            if (not p.fence.owned(checked) or checked.get('status') != 'active' or
                    not p._owned_invocation(checked) or checked.get('backup_readback') != readback or
                    checked.get('backup_upload') != upload or
                    checked.get('restore_resources', {}).get(kind) != target or
                    not isinstance(checked_revision, tuple) or len(checked_revision) != 2 or
                    any(not isinstance(v, str) or not v for v in checked_revision)):
                raise RestoreJournalError('restore reservation changed')
        except BaseException:
            if started:
                self._fail()
            raise RestoreJournalError('restore resource not admitted') from None

    def begin(self, kind, name, labels):
        if (not isinstance(name, str) or not re.fullmatch(r'voice-media-restore-[0-9a-f]{32}', name) or
                labels != {'voice.test.owner': name}):
            raise RestoreJournalError('restore resource identity invalid')
        nonce = secrets.token_hex(16)
        target = {'status': 'creating', 'nonce': nonce, 'name': name, 'labels': copy.deepcopy(labels)}
        self._transition(kind, None, target)
        return nonce

    def created(self, kind, nonce, cid, labels):
        current = self.phases.journal.state['steps'].get('restore-resource-' + kind)
        if (not isinstance(current, dict) or current.get('status') != 'creating' or current.get('nonce') != nonce or
                current.get('labels') != labels or not isinstance(cid, str) or not re.fullmatch(r'[0-9a-f]{64}', cid)):
            self._fail()
        self._transition(kind, current, current | {'status': 'created', 'container_id': cid})

    def begin_remove(self, kind, cid):
        current = self.phases.journal.state['steps'].get('restore-resource-' + kind)
        if not isinstance(current, dict) or current.get('status') != 'created' or current.get('container_id') != cid:
            self._fail()
        self._transition(kind, current, current | {'status': 'removing'})

    def removed(self, kind, cid, absent):
        current = self.phases.journal.state['steps'].get('restore-resource-' + kind)
        if (not isinstance(current, dict) or current.get('status') != 'removing' or
                current.get('container_id') != cid or absent is not True):
            self._fail()
        self._transition(kind, current, current | {'status': 'removed'})

    def uncertain(self, kind, nonce):
        self._fail()
