"""Durable fenced dump attempts; not a backup, restore or rollout authority.

The owner must hold the common lock throughout and perform separate actual
drain, identity and storage checks around the producer. Never restart this
operation automatically after an unknown outcome: reconcile its journal,
processes, effects and permanent ledger first, retaining the cluster lock.
One sequential caller owns each instance; concurrent use of the same instance
is not supported. Cluster CAS excludes independent invocations.
"""
import copy
import hashlib
import json
import re
import secrets

import media_operation_guard

MEMBERS = frozenset(('file_db', 'story_db', 'messaging_db', 'roles'))
CAPTURE_FIELDS = frozenset(('snapshot_id', 'pod_uid', 'pvc_uid', 'pv_uid', 'container_id', 'restart_count'))


class RecoveryRequired(RuntimeError):
    pass


class BackupPhases:
    def __init__(self, journal, fence, client, lock, namespace_uid):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.intent = copy.deepcopy(journal.intent)
        self.poisoned = False
        self.invocation = secrets.token_hex(16)
        self.started = False
        self._identity()

    def _identity(self):
        intent = self.intent
        capture = intent.get('backup_capture') if isinstance(intent, dict) else None
        if (self.poisoned or self.journal.poisoned or self.journal.intent != intent or
                self.journal.state.get('intent') != intent or
                intent.get('resource_context') != self.fence.context or intent.get('namespace_uid') != self.namespace_uid or
                not isinstance(capture, dict) or set(capture) != CAPTURE_FIELDS or
                any(not isinstance(capture.get(k), str) or not capture[k] for k in CAPTURE_FIELDS - {'restart_count'}) or
                not re.fullmatch(r'containerd://[0-9a-f]{64}', capture['container_id']) or
                type(capture.get('restart_count')) is not int or capture['restart_count'] < 0):
            raise RecoveryRequired('backup operation identity invalid or unavailable')

    def _guard(self):
        self._identity()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)

    def _uncertain(self, member):
        self.poisoned = True
        try:
            self.fence.uncertain('backup dump outcome uncertain: ' + member)
        except BaseException:
            raise RecoveryRequired('fence outcome unknown; retain lock and reconcile') from None
        raise RecoveryRequired('dump outcome uncertain; retain lock and reconcile') from None

    def _start(self, member):
        if self.started:
            return
        steps = self.journal.state['steps']
        if 'backup-invocation' in steps:
            raise RecoveryRequired('existing backup invocation requires reconciliation')
        steps['backup-invocation'] = {'status': 'pending', 'nonce': self.invocation}
        self.journal.save()
        self.fence.authority()
        row, revision = self.fence.ledger.read()
        if not self.fence.owned(row) or row.get('status') != 'active' or 'backup_invocation' in row:
            raise RecoveryRequired('backup invocation already owned or fence changed')
        owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
        row['backup_invocation'] = owner
        self.fence.ledger.write(revision, row)
        self._guard()
        current, current_revision = self.fence.ledger.read()
        if (not self.fence.owned(current) or current.get('status') != 'active' or
                not isinstance(current_revision, tuple) or len(current_revision) != 2 or
                any(not isinstance(v, str) or not v for v in current_revision) or
                current.get('backup_invocation') != owner):
            raise RecoveryRequired('backup invocation ownership changed')
        steps['backup-invocation'] = {'status': 'owned', 'nonce': self.invocation}
        self.journal.save()
        self.started = True

    def _owned_invocation(self, row):
        return row.get('backup_invocation') == {'nonce': self.invocation,
                    'operation_id': self.fence.context['operation_id']}

    def begin(self, member):
        if member not in MEMBERS:
            raise RecoveryRequired('backup member not admitted')
        pending = False
        try:
            self._guard()
            pending = True
            self._start(member)
            steps = self.journal.state['steps']
            name = 'dump-' + member
            if name in steps or any(k.startswith('dump-') and v.get('status') == 'pending'
                                    for k, v in steps.items() if isinstance(v, dict)):
                raise RecoveryRequired('dump already attempted or pending; reconcile')
            nonce = secrets.token_hex(16)
            steps[name] = {'status': 'pending', 'snapshot_id': self.intent['backup_capture']['snapshot_id'],
                           'nonce': nonce}
            self.journal.save()
            self.fence.authority()
            row, revision = self.fence.ledger.read()
            records = row.get('backup_members', {}) if isinstance(row, dict) else None
            if (not self.fence.owned(row) or row.get('status') != 'active' or not self._owned_invocation(row) or
                    not isinstance(records, dict) or
                    set(records) - MEMBERS or member in records or
                    any(not isinstance(value, dict) or value.get('status') != 'done' for value in records.values())):
                raise RecoveryRequired('another dump is pending or member already attempted')
            records = copy.deepcopy(records)
            records[member] = {'status': 'pending', 'nonce': nonce,
                               'snapshot_id': self.intent['backup_capture']['snapshot_id'],
                               'operation_id': self.fence.context['operation_id']}
            row['backup_members'] = records
            row['step'] = name
            self.fence.ledger.write(revision, row)
            self.fence.authority()
            self._guard()
            current, current_revision = self.fence.ledger.read()
            if (not self.fence.owned(current) or current.get('status') != 'active' or
                    not self._owned_invocation(current) or not isinstance(current_revision, tuple) or
                    len(current_revision) != 2 or any(not isinstance(v, str) or not v for v in current_revision) or
                    current.get('backup_members', {}).get(member) != records[member]):
                raise RecoveryRequired('dump reservation changed')
            return nonce
        except BaseException:
            if pending:
                self._uncertain(member)
            raise RecoveryRequired('dump admission failed; no request issued') from None

    def complete(self, member, receipt):
        if member not in MEMBERS:
            raise RecoveryRequired('backup member not admitted')
        try:
            self._guard()
            name = 'dump-' + member
            row = self.journal.state['steps'].get(name)
            expected = self.intent['backup_capture'] | {'operation_id': self.fence.context['operation_id'],
                        'member': member, 'exit': 0, 'scope': 'dump-terminal-only',
                        'attempt_nonce': row.get('nonce') if isinstance(row, dict) else None}
            if (not isinstance(row, dict) or set(row) != {'status', 'snapshot_id', 'nonce'} or
                    row['status'] != 'pending' or row['snapshot_id'] != expected['snapshot_id'] or
                    not isinstance(row['nonce'], str) or not re.fullmatch(r'[0-9a-f]{32}', row['nonce']) or
                    not isinstance(receipt, dict) or receipt != expected or
                    type(receipt.get('exit')) is not int or type(receipt.get('restart_count')) is not int):
                raise RecoveryRequired('dump receipt mismatch')
            ledger_row, revision = self.fence.ledger.read()
            reservation = {'status': 'pending', 'nonce': row['nonce'], 'snapshot_id': expected['snapshot_id'],
                           'operation_id': expected['operation_id']}
            if (not self.fence.owned(ledger_row) or ledger_row.get('status') != 'active' or
                    not self._owned_invocation(ledger_row) or
                    ledger_row.get('backup_members', {}).get(member) != reservation):
                raise RecoveryRequired('dump reservation changed')
            self.journal.state['steps'][name] = {'status': 'done', 'nonce': row['nonce'],
                                                 'receipt': copy.deepcopy(receipt)}
            self.journal.save()
            digest = hashlib.sha256(json.dumps(receipt, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            ledger_row['backup_members'][member] = reservation | {'status': 'done', 'receipt_sha256': digest}
            ledger_row['step'] = name + '-done'
            self.fence.ledger.write(revision, ledger_row)
            self._guard()
        except BaseException:
            self._uncertain(member)

    def uncertain(self, member):
        if member not in MEMBERS:
            raise RecoveryRequired('backup member not admitted')
        self._uncertain(member)
