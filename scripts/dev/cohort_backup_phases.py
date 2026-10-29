"""Fenced sequential cohort dump attempts; selection/drain remain external."""
import copy
import hashlib
import json
import re
import secrets
import story_media_backup_cohort as cohort


class CohortPhaseError(RuntimeError):
    pass


class CohortPhases:
    def __init__(self, phases):
        self.phases = phases
        self.binding = copy.deepcopy(phases.intent['cohort_binding'])
        self.members = cohort._members(self.binding, self.binding['databases'])
        source = self.binding['sources']['roles']['engine']
        capture = phases.intent['backup_capture']
        if (self.binding['operation_id'] != phases.fence.context['operation_id'] or
                self.binding['source_sha'] != phases.fence.context['commit'] or
                self.binding['snapshot_id'] != capture['snapshot_id'] or
                source.get('namespace_uid') != phases.namespace_uid or
                type(source.get('restart_count')) is not int or
                any(not isinstance(source.get(k), str) or not source[k] for k in
                    ('namespace_uid','pod_uid','pvc_uid','pv_uid','container_id')) or
                any(source.get(k) != capture[k] for k in
                    ('pod_uid','pvc_uid','pv_uid','container_id','restart_count'))):
            raise CohortPhaseError('cohort identity mismatch')

    def _guard(self):
        if self.phases.intent.get('cohort_binding') != self.binding:
            raise CohortPhaseError('cohort binding changed')
        self.phases._guard()

    def uncertain(self, member):
        self.phases._uncertain('cohort-' + str(member))

    def begin(self, member):
        if member not in self.members: raise CohortPhaseError('cohort member not admitted')
        pending = False
        try:
            p = self.phases
            self._guard()
            pending = True
            p._start(member)
            name = 'cohort-dump-' + member
            if name in p.journal.state['steps']:
                raise CohortPhaseError('cohort member already attempted')
            nonce = secrets.token_hex(16)
            reservation = {'status':'pending', 'nonce':nonce, 'snapshot_id':self.binding['snapshot_id'],
                           'operation_id':self.binding['operation_id']}
            p.journal.state['steps'][name] = copy.deepcopy(reservation)
            p.journal.save()
            p.fence.authority()
            row, revision = p.fence.ledger.read()
            records = row.get('cohort_members', {})
            if (not p.fence.owned(row) or row.get('status') != 'active' or not p._owned_invocation(row) or
                    'backup_members' in row or
                    not isinstance(records, dict) or set(records) - self.members or member in records or
                    any(not isinstance(v, dict) or v.get('status') != 'done' for v in records.values())):
                raise CohortPhaseError('cohort reservation changed')
            row.setdefault('cohort_members', {})[member] = copy.deepcopy(reservation)
            row['step'] = name
            p.fence.ledger.write(revision, row)
            self._guard()
            actual, revision = p.fence.ledger.read()
            if (not p.fence.owned(actual) or actual.get('status') != 'active' or not p._owned_invocation(actual) or
                    actual.get('cohort_members', {}).get(member) != reservation or not isinstance(revision, tuple) or
                    len(revision) != 2 or any(not isinstance(v,str) or not v for v in revision)):
                raise CohortPhaseError('cohort reservation lost')
            return nonce
        except BaseException:
            if pending:
                try: self.uncertain(member)
                except BaseException: pass
            raise CohortPhaseError('cohort dump not admitted; reconcile') from None

    def complete(self, member, receipt):
        try:
            if member not in self.members: raise CohortPhaseError('cohort member not admitted')
            p = self.phases
            self._guard()
            name = 'cohort-dump-' + member
            entry = p.journal.state['steps'][name]
            source = self.binding['sources'][member]
            expected = {'operation_id':self.binding['operation_id'], 'snapshot_id':self.binding['snapshot_id'],
                        'member':member, **{k:p.intent['backup_capture'][k] for k in
                        ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')},
                        'server_start':source['server_start'], 'database_oid':source.get('database_oid'),
                        'attempt_nonce':entry['nonce'], 'exit':0, 'scope':'dump-terminal-only'}
            if (entry.get('status') != 'pending' or receipt != expected or type(receipt.get('exit')) is not int or
                    type(receipt.get('restart_count')) is not int or
                    (member != 'roles' and type(receipt.get('database_oid')) is not int)):
                raise CohortPhaseError('cohort receipt differs')
            row, revision = p.fence.ledger.read()
            if (not p.fence.owned(row) or row.get('status') != 'active' or not p._owned_invocation(row) or
                    row.get('cohort_members', {}).get(member) != entry):
                raise CohortPhaseError('cohort reservation changed')
            p.journal.state['steps'][name] = {'status':'done', 'nonce':entry['nonce'], 'receipt':copy.deepcopy(receipt)}
            p.journal.save()
            digest = hashlib.sha256(json.dumps(receipt, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            row['cohort_members'][member] = entry | {'status':'done', 'receipt_sha256':digest}
            completed = copy.deepcopy(row['cohort_members'][member])
            row['step'] = name + '-done'
            p.fence.ledger.write(revision, row)
            self._guard()
            actual, actual_revision = p.fence.ledger.read()
            if (not p.fence.owned(actual) or actual.get('status') != 'active' or not p._owned_invocation(actual) or
                    actual.get('cohort_members', {}).get(member) != completed or
                    not isinstance(actual_revision, tuple) or len(actual_revision) != 2 or
                    any(not isinstance(v, str) or not v for v in actual_revision)):
                raise CohortPhaseError('cohort terminal receipt changed')
        except BaseException:
            try: self.uncertain(member)
            except BaseException: pass
            raise CohortPhaseError('cohort dump outcome uncertain') from None
