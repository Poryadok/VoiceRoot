"""Fenced sequential cohort dump attempts; selection/drain remain external."""
import copy
import hashlib
import json
import re
import secrets
import story_media_backup_cohort as cohort
import cohort_unmanaged_actual as unmanaged


class CohortPhaseError(RuntimeError):
    pass


class CohortPhases:
    def __init__(self, phases, catalog_phase=None, catalog_token=None):
        self.phases = phases
        self.binding = copy.deepcopy(phases.intent['cohort_binding'])
        self.catalog_phase, self.catalog_token = catalog_phase, catalog_token
        self._catalog_phase, self._catalog_token = catalog_phase, catalog_token
        self._catalog_cut = self._bound_catalog_cut = None
        if 'full_catalog' in self.binding:
            try:
                unmanaged.verify_catalog_phase(catalog_phase,catalog_token,self.binding)
                self._catalog_identity()
            except Exception: raise CohortPhaseError('full catalog authority unavailable') from None
        elif catalog_phase is not None or catalog_token is not None:
            raise CohortPhaseError('unexpected catalog authority')
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
        if 'full_catalog' in self.binding:
            if self.catalog_phase is not self._catalog_phase or self.catalog_token != self._catalog_token:
                raise CohortPhaseError('full catalog authority changed')
            if self._catalog_cut is not self._bound_catalog_cut:
                raise CohortPhaseError('full catalog cut changed')
            try:
                self._catalog_identity()
                unmanaged.verify_catalog_phase(self.catalog_phase,self.catalog_token,self.binding,self._catalog_cut)
                self._catalog_identity()
            except unmanaged.UnmanagedError: raise CohortPhaseError('full catalog authority changed') from None

    def _catalog_identity(self):
        catalog=self.catalog_phase; phase=self.phases
        if (catalog.fence.context!=phase.fence.context or catalog.lock!=phase.lock or
                catalog.namespace_uid!=phase.namespace_uid or catalog.client.journal is not catalog.journal or
                phase.client.journal is not phase.journal or catalog.fence.ledger.client is not catalog.client or
                phase.fence.ledger.client is not phase.client or catalog.fence.ledger.read()!=phase.fence.ledger.read()):
            raise CohortPhaseError('full catalog phase identity differs')

    def bind_catalog_cut(self,cut):
        """Bind the original writer cut before any provision successor exists."""
        try:
            import full_cohort_affected_plan as affected
            if ('full_catalog' not in self.binding or self._catalog_cut is not None or
                    type(cut) is not affected.writer_cut.WriterCut or cut.provision_successor is not None or
                    cut.effective_session is not None or cut.migration is not None):
                raise ValueError()
            self._guard();cut._guard()
            if (cut.fence.context!=self.phases.fence.context or cut.lock!=self.phases.lock or
                    cut.namespace_uid!=self.phases.namespace_uid or
                    cut.fence.ledger.read()!=self.phases.fence.ledger.read()):
                raise ValueError()
            receipt=unmanaged.verify_catalog_phase(self.catalog_phase,self.catalog_token,self.binding,cut)
            scope={'engine':receipt['engine'],'server_start':receipt['server_start'],
                   'databases':sorted(receipt['all_databases']),'database_oids':receipt['all_databases']}
            if cut.admission['session_scope']!=scope or cut.fence.ledger.read()!=self.phases.fence.ledger.read():
                raise ValueError()
            self._catalog_cut=self._bound_catalog_cut=cut
        except Exception:
            raise CohortPhaseError('full catalog cut binding unavailable') from None

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
