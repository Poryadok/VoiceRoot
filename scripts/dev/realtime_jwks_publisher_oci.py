"""Source-qualified fixed nginx metadata for the fenced DEV publisher.

No OCI bytes are supplied by the caller. This phase does not establish image
runtime readiness and does not add the auxiliary image to the application map.
"""
import copy
import hashlib
import re
import secrets
import types
from pathlib import Path

import frontend_runtime_executor
import full_cohort_package_phase
import full_cohort_predecessor_phase
import media_operation_guard
import media_operation_journal
import realtime_jwks_publisher as publisher_module
import realtime_p3_create_only_source as source
import resource_fencing

COLLECTOR_PATH = 'scripts/deploy/staging_p3_auxiliary_image.py'
COLLECTOR_SHA256 = 'f996c63220bde5c4d2622784c62bdbbcb6c3cabc95c34fd725180edc78e1fcad'
STEP = 'realtime-jwks-publisher-oci'
MARKER = 'realtime_jwks_publisher_oci'


class OCIError(RuntimeError): pass


def _source_module(repo, commit):
    try:
        if not isinstance(repo, str) or not isinstance(commit, str) or not re.fullmatch(r'[a-f0-9]{40}', commit):
            raise ValueError()
        blob = source._blob(repo, commit, COLLECTOR_PATH)
        if source._disk(repo, COLLECTOR_PATH) != blob or hashlib.sha256(blob).hexdigest() != COLLECTOR_SHA256:
            raise ValueError()
        module = types.ModuleType('_reviewed_staging_p3_auxiliary_image')
        exec(compile(blob, COLLECTOR_PATH, 'exec'), module.__dict__)
        return module
    except Exception:
        raise OCIError('reviewed publisher OCI source unavailable') from None


class PublisherOCIPhase:
    """Use fixed collector bytes from the pinned source and protected facades."""
    def __init__(self, journal, fence, client, lock, namespace_uid, package, publisher):
        if (type(journal) is not media_operation_journal.Journal or
                type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
                type(fence.check) is not resource_fencing.ResourceCheck or
                fence.check.endpoint != full_cohort_predecessor_phase.TT_ENDPOINT or
                type(client) is not frontend_runtime_executor.KubectlTransport or
                type(package) is not full_cohort_package_phase.PackagePhase or
                type(publisher) is not publisher_module.PublisherPhase or
                type(publisher.p3) is not publisher_module.p3.Session or client.journal is not journal or
                fence.ledger.client is not client or
                fence.context != package.fence.context or fence.context != publisher.fence.context or
                package.namespace_uid != namespace_uid or publisher.namespace_uid != namespace_uid or
                package.lock != lock or publisher.lock != lock or
                journal.intent.get('resource_context') != fence.context or
                journal.intent.get('namespace_uid') != namespace_uid or
                journal.intent.get('realtime_jwks_publisher') != publisher.plan or
                type(publisher.plan) is not dict or set(publisher.plan) != {'source_repo','source_sha','node'} or
                publisher.plan.get('source_sha') != fence.context.get('commit') or
                not isinstance(publisher.plan.get('source_repo'),str) or
                not Path(publisher.plan['source_repo']).is_absolute()):
            raise OCIError('publisher OCI phase identity invalid')
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.package, self.publisher = package, publisher
        self.context, self.intent = copy.deepcopy(fence.context), copy.deepcopy(journal.intent)
        self.plan = copy.deepcopy(publisher.plan)
        self.package_refs = (package.journal, package.fence, package.client)
        self.publisher_refs = (publisher.journal, publisher.fence, publisher.client)
        self.p3 = publisher.p3
        self.p3_refs = (self.p3.journal, self.p3.fence, self.p3.client)
        self.fence_refs = (fence.ledger, fence.check)
        self.dependency_fences = tuple((f.ledger, f.check) for _, f, _ in
                                       (self.package_refs, self.publisher_refs, self.p3_refs))
        self.poisoned = False

    def _local(self):
        if (self.poisoned or self.journal.poisoned or self.journal.dirfd is None or self.journal.lockfd is None or
                self.journal.intent != self.intent or self.journal.state.get('intent') != self.intent or
                (self.fence.ledger, self.fence.check) != self.fence_refs or
                self.fence.context != self.context or self.fence.check.context != self.context or
                self.fence.check.endpoint != full_cohort_predecessor_phase.TT_ENDPOINT or
                self.fence.ledger.client is not self.client or self.client.journal is not self.journal or
                (self.package.journal, self.package.fence, self.package.client) != self.package_refs or
                (self.publisher.journal, self.publisher.fence, self.publisher.client) != self.publisher_refs or
                self.publisher.p3 is not self.p3 or (self.p3.journal,self.p3.fence,self.p3.client) != self.p3_refs or
                self.publisher.plan != self.plan or self.package.fence.context != self.context or
                self.publisher.fence.context != self.context or self.package.lock != self.lock or
                self.publisher.lock != self.lock or self.package.namespace_uid != self.namespace_uid or
                self.publisher.namespace_uid != self.namespace_uid):
            raise OCIError('publisher OCI authority changed')
        for (journal, fence, client), refs in zip((self.package_refs, self.publisher_refs, self.p3_refs),
                                                  self.dependency_fences):
            if (type(journal) is not media_operation_journal.Journal or journal.poisoned or
                    journal.dirfd is None or journal.lockfd is None or journal.state.get('intent') != journal.intent or
                    journal.intent.get('resource_context') != self.context or
                    journal.intent.get('namespace_uid') != self.namespace_uid or
                    type(fence) is not resource_fencing.Fence or (fence.ledger, fence.check) != refs or
                    type(fence.ledger) is not resource_fencing.KubeLedger or
                    fence.context != self.context or type(fence.check) is not resource_fencing.ResourceCheck or
                    fence.check.context != self.context or
                    fence.check.endpoint != self.fence.check.endpoint or fence.ledger.client is not client or
                    type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal):
                raise OCIError('publisher OCI dependent custody changed')
    def _guard(self):
        self._local()
        full_cohort_predecessor_phase._no_tt_proxy()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        self.publisher._guard()
        row, revision = self.fence.ledger.read()
        if (self.package.fence.ledger.read() != (row, revision) or
                self.publisher.fence.ledger.read() != (row, revision) or
                self.p3.fence.ledger.read() != (row, revision)):
            raise OCIError('publisher OCI physical ledger differs')
        self._local()
        return row, revision

    def _inputs(self, token):
        row, revision = self._guard()
        package = self.package.verify_current(token)
        node = self.plan['node']
        wanted = {'name':node['name'], 'uid':node['uid'], 'hostname':node['hostname'], 'architecture':'amd64'}
        if package.get('nodes', []).count(wanted) != 1:
            raise OCIError('publisher node not in package')
        self.publisher._node()
        _source_module(self.plan['source_repo'], self.context['commit'])
        if self.package.verify_current(token) != package:
            raise OCIError('publisher package changed')
        module = _source_module(self.plan['source_repo'], self.context['commit'])
        if self._guard() != (row, revision): raise OCIError('publisher OCI ledger changed')
        return module, row, revision

    def _collect(self, module):
        value = module.collect()
        if type(value) is not dict or set(value) != {'index', 'manifest', 'config'}:
            raise OCIError('publisher OCI collection invalid')
        raw = (value['index'], value['manifest'], value['config'])
        if any(type(item) is not bytes for item in raw): raise OCIError('publisher OCI bytes invalid')
        module.validate(*raw)
        if publisher_module.qualify_oci(*raw) != {'index':publisher_module.INDEX,
                'platform':publisher_module.PLATFORM, 'config':publisher_module.CONFIG}:
            raise OCIError('publisher OCI pin changed')
        return raw

    def run(self, token):
        module, row, revision = self._inputs(token)
        if MARKER in row or STEP in self.journal.state['steps']:
            raise OCIError('publisher OCI already attempted')
        pending = {'status':'pending', 'nonce':secrets.token_hex(16),
                   'operation_id':self.context['operation_id'], 'source_sha':self.context['commit'],
                   'collector_sha256':COLLECTOR_SHA256}
        try:
            self.journal.state['steps'][STEP] = copy.deepcopy(pending); self.journal.save()
            if self._guard() != (row, revision): raise OCIError('publisher OCI reservation changed')
            row[MARKER] = copy.deepcopy(pending)
            self.fence.ledger.write(revision, row)
            module, row, revision = self._inputs(token)
            if row.get(MARKER) != pending: raise OCIError('publisher OCI reservation lost')
            first = self._collect(module)
            module, current, current_revision = self._inputs(token)
            if (current, current_revision) != (row, revision): raise OCIError('publisher OCI changed')
            second = self._collect(module)
            module, current, current_revision = self._inputs(token)
            if first != second or (current, current_revision) != (row, revision):
                raise OCIError('publisher OCI content changed')
            if self.journal.state['steps'].get(STEP) != pending:
                raise OCIError('publisher OCI private reservation changed')
            done = pending | {'status':'done', 'index_sha256':hashlib.sha256(first[0]).hexdigest(),
                              'manifest_sha256':hashlib.sha256(first[1]).hexdigest(),
                              'config_sha256':hashlib.sha256(first[2]).hexdigest()}
            row[MARKER] = copy.deepcopy(done)
            self.fence.ledger.write(revision, row)
            confirmed, _ = self._guard()
            if confirmed.get(MARKER) != done or self.journal.state['steps'].get(STEP) != pending:
                raise OCIError('publisher OCI completion changed')
            self.journal.state['steps'][STEP] = copy.deepcopy(done); self.journal.save()
            _, confirmed, _ = self._inputs(token)
            if confirmed.get(MARKER) != done or self.journal.state['steps'].get(STEP) != done:
                raise OCIError('publisher OCI terminal authority changed')
            return first
        except BaseException:
            self.poisoned = True
            try: self.fence.uncertain('publisher OCI admission uncertain')
            except BaseException: pass
            raise

    def verify_current(self, token):
        module, row, revision = self._inputs(token)
        done = copy.deepcopy(self.journal.state['steps'].get(STEP))
        if (not isinstance(done, dict) or set(done) != {'status','nonce','operation_id','source_sha',
                    'collector_sha256','index_sha256','manifest_sha256','config_sha256'} or
                not isinstance(done.get('nonce'), str) or not re.fullmatch(r'[a-f0-9]{32}', done['nonce']) or
                done.get('status') != 'done' or row.get(MARKER) != done or
                done.get('operation_id') != self.context['operation_id'] or
                done.get('source_sha') != self.context['commit'] or done.get('collector_sha256') != COLLECTOR_SHA256 or
                done.get('index_sha256') != publisher_module.INDEX[7:] or
                done.get('manifest_sha256') != publisher_module.PLATFORM[7:] or
                done.get('config_sha256') != publisher_module.CONFIG[7:]):
            raise OCIError('publisher OCI receipt unavailable')
        first = self._collect(module)
        module, current, current_revision = self._inputs(token)
        if (current, current_revision) != (row, revision): raise OCIError('publisher OCI ledger changed')
        second = self._collect(module)
        module, current, current_revision = self._inputs(token)
        if (current, current_revision) != (row, revision) or first != second or \
                self.journal.state['steps'].get(STEP) != done or any(
                done.get(key) != hashlib.sha256(raw).hexdigest() for key, raw in zip(
                    ('index_sha256','manifest_sha256','config_sha256'), first)):
            raise OCIError('publisher OCI proof changed')
        return first
