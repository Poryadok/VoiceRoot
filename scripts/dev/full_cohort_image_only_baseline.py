"""Capture source-qualified predecessor evidence before an image-only begin CAS.

The private capture is not authority by itself: the caller must attach its
descriptor in the same UID/resourceVersion CAS that replaces the old row.
"""
import copy
import json
import re
import time
import urllib.request

import frontend_runtime_executor
import full_cohort_predecessor_phase as predecessor
import media_operation_guard as guard
import media_operation_journal
import resource_fencing
import canonical_application_package as package
import full_cohort_package_phase
import media_live_baseline

STEP = 'full-cohort-image-only-baseline'
MARKER = 'image_only_baseline'
QUALIFIED = 'image_only_qualified'


class BaselineError(RuntimeError):
    pass


def _read_history(check):
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args): return None
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result: raise ValueError('duplicate key')
            result[key] = value
        return result
    try:
        with urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect()).open(
                check.endpoint + '/api/projects/voice/resources/DEV', timeout=10) as response:
            raw = response.read((16 << 20) + 1)
        if len(raw) > 16 << 20: raise ValueError('history overflow')
        return json.loads(raw, object_pairs_hook=unique)
    except (OSError, ValueError):
        raise BaselineError('complete TT history unavailable') from None


def history_floors(card):
    """Validate the complete contiguous TT history and extract deployment floors."""
    try:
        if card['code'] != 'DEV' or card['environment'] != 'dev': raise ValueError()
        events = card['events']
        if not isinstance(events, list) or not events or len(events) > 20000: raise ValueError()
        if type(card['revision']) is not int or card['revision'] != len(events): raise ValueError()
        previous = None
        floors = set()
        for index, event in enumerate(events, 1):
            before, after = event['before'], event['after']
            if (event['id'] != 'r' + str(index) or type(before) is not dict or type(after) is not dict or
                    before.get('revision') != index - 1 or after.get('revision') != index or
                    (previous is not None and before != previous)):
                raise ValueError()
            previous = after
            if event['kind'] != 'deployment': continue
            observed, candidate = after['observed'], after['candidate']
            commit = observed['commit']
            if (before.get('state') != 'held' or before.get('phase') != 'deploy' or
                    after.get('phase') != 'verify' or candidate.get('state') != 'known' or
                    observed.get('state') != 'known' or not isinstance(commit, str) or
                    not re.fullmatch(r'[a-f0-9]{40}', commit) or candidate.get('commit') != commit or
                    not isinstance(observed.get('images'), dict) or not observed['images'] or
                    candidate.get('images') != observed['images'] or
                    not isinstance(observed.get('deployment'), str) or not observed['deployment'] or
                    event.get('actor') != before.get('owner') or observed.get('actor') != before.get('owner') or
                    any(after.get(k) != before.get(k) or observed.get(k) != before.get(k)
                        for k in ('claim', 'window', 'operation')) or
                    after.get('generation') != before.get('generation') or
                    type(before.get('operation')) is not int or before['operation'] < 1):
                raise ValueError()
            floors.add(commit)
        return sorted(floors)
    except (KeyError, TypeError, ValueError, AttributeError):
        raise BaselineError('complete deployment history unavailable or inconsistent') from None


def _pre_begin(fence, client, lock, namespace_uid, previous, revision):
    if (type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
            type(fence.check) is not resource_fencing.ResourceCheck or
            type(client) is not frontend_runtime_executor.KubectlTransport or fence.ledger.client is not client or
            fence.check.endpoint != predecessor.TT_ENDPOINT or fence.check.context != fence.context):
        raise BaselineError('invalid executor')
    predecessor._no_tt_proxy()
    resource_fencing.validate_context(fence.context)
    try:
        keys = {'project', 'resource', 'actor', 'claim', 'generation', 'window', 'revision',
                'operation_id', 'commit', 'images'}
        resource_fencing.validate_context({key: previous[key] for key in keys})
        if (previous['status'] != 'complete' or previous['actor'] != 'scheduled_devops' or
                fence.context['actor'] != 'scheduled_devops' or
                not re.fullmatch(r'media-c1-[a-f0-9]{32}', fence.context['operation_id']) or
                previous['project'] != fence.context['project'] or previous['resource'] != fence.context['resource'] or
                previous['generation'] >= fence.context['generation'] or
                previous['operation_id'] == fence.context['operation_id'] or previous['actual'] != previous['images'] or
                not isinstance(revision, tuple) or len(revision) != 2 or
                any(not isinstance(x, str) or not x for x in revision)):
            raise ValueError()
        meta, data = lock['metadata'], lock['data']
        if (not isinstance(namespace_uid, str) or not namespace_uid or
                meta['name'] != guard.LOCK or meta['namespace'] != 'voice-dev' or
                not meta.get('uid') or not meta.get('resourceVersion') or
                any(meta.get('labels', {}).get(k) != v for k, v in guard.LABELS.items()) or
                data.get('resource_operation') != fence.context['operation_id'] or
                data.get('resource_context') != json.dumps(fence.context, sort_keys=True, separators=(',', ':')) or
                not data.get('operation')): raise ValueError()
        fence.authority()
        if fence.ledger.read() != (previous, revision): raise ValueError()
        namespace = client.call('get', 'namespace', 'voice-dev', '-o', 'json')['metadata']
        actual_lock = client.call('get', 'configmap', guard.LOCK, '-o', 'json')
        am = actual_lock['metadata']
        if (namespace.get('name') != 'voice-dev' or namespace.get('uid') != namespace_uid or
                namespace.get('deletionTimestamp') or
                any(namespace.get('labels', {}).get(k) != v for k, v in guard.LABELS.items()) or
                am.get('name') != guard.LOCK or am.get('namespace') != 'voice-dev' or
                am.get('uid') != meta['uid'] or am.get('resourceVersion') != meta['resourceVersion'] or
                am.get('deletionTimestamp') or
                any(am.get('labels', {}).get(k) != v for k, v in guard.LABELS.items()) or
                actual_lock.get('data') != data): raise ValueError()
        fence.authority()
        if fence.ledger.read() != (previous, revision): raise ValueError()
    except (KeyError, TypeError, ValueError, AttributeError):
        raise BaselineError('pre-begin authority or completed predecessor changed') from None


def capture(fence, client, lock, namespace_uid, journal, previous, revision):
    if (type(journal) is not media_operation_journal.Journal or client.journal is not journal or
            journal.poisoned or journal.dirfd is None or journal.lockfd is None or
            journal.state.get('intent') != journal.intent or
            journal.intent.get('resource_context') != fence.context or
            journal.intent.get('namespace_uid') != namespace_uid or STEP in journal.state['steps']):
        raise BaselineError('private journal invalid')
    _pre_begin(fence, client, lock, namespace_uid, previous, revision)
    history = _read_history(fence.check)
    floors = history_floors(history)
    deployment = predecessor.deployment_event(history, previous)
    if previous['commit'] not in floors: raise BaselineError('completed predecessor floor unavailable')
    card = fence.authority()
    tip = history['events'][-1]['after']
    if (history.get('revision') != card.get('revision') or
            any(tip.get(key) != card.get(key) for key in
                ('code', 'environment', 'state', 'revision', 'generation', 'owner', 'claim',
                 'window', 'operation', 'phase', 'candidate', 'observed'))):
        raise BaselineError('history changed')
    snapshot = {'schema': 'voice-full-cohort-image-only-snapshot-v1',
                'operation_id': fence.context['operation_id'], 'namespace_uid': namespace_uid,
                'ledger_uid': revision[0], 'ledger_resourceVersion': revision[1],
                'old_row_sha256': predecessor.digest(previous), 'previous': copy.deepcopy(previous),
                'history_sha256': predecessor.digest(history), 'deployment': deployment, 'floors': floors}
    descriptor = {'schema': 'voice-full-cohort-image-only-baseline-v1',
                  'snapshot_sha256': predecessor.digest(snapshot),
                  'old_row_sha256': snapshot['old_row_sha256'], 'ledger_uid': revision[0],
                  'ledger_resourceVersion': revision[1]}
    journal.state['steps'][STEP] = {'status': 'captured', 'descriptor': copy.deepcopy(descriptor), 'snapshot': snapshot}
    journal.save()
    _pre_begin(fence, client, lock, namespace_uid, previous, revision)
    if _read_history(fence.check) != history: raise BaselineError('history changed during capture')
    _pre_begin(fence, client, lock, namespace_uid, previous, revision)
    return descriptor


class ImageOnlyPhase:
    """Qualify the captured predecessor and actual inventory after begin."""
    def __init__(self, journal, fence, client, lock, namespace_uid, package_phase):
        if (type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
                type(fence.check) is not resource_fencing.ResourceCheck or
                fence.check.endpoint != predecessor.TT_ENDPOINT or fence.check.context != fence.context or
                type(client) is not frontend_runtime_executor.KubectlTransport or fence.ledger.client is not client or
                type(journal) is not media_operation_journal.Journal or client.journal is not journal or
                type(package_phase) is not full_cohort_package_phase.PackagePhase or
                package_phase.fence.context != fence.context or package_phase.namespace_uid != namespace_uid or
                package_phase.lock != lock):
            raise BaselineError('phase identity invalid')
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.package_phase = package_phase
        self.intent = copy.deepcopy(journal.intent)
        self.context = copy.deepcopy(fence.context)
        self.package_journal = package_phase.journal
        self.package_fence = package_phase.fence
        self.package_client = package_phase.client
        self.package_intent = copy.deepcopy(self.package_journal.intent)
        self.poisoned = False

    def _guard(self):
        if (self.poisoned or self.fence.context != self.context or self.fence.check.context != self.context or
                self.fence.check.endpoint != predecessor.TT_ENDPOINT or self.fence.ledger.client is not self.client or
                self.client.journal is not self.journal or self.journal.poisoned or
                self.journal.dirfd is None or self.journal.lockfd is None or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or
                self.intent.get('resource_context') != self.fence.context or
                self.intent.get('namespace_uid') != self.namespace_uid or
                self.package_phase.journal is not self.package_journal or
                self.package_phase.fence is not self.package_fence or self.package_phase.client is not self.package_client or
                type(self.package_journal) is not media_operation_journal.Journal or self.package_journal.poisoned or
                self.package_journal.dirfd is None or self.package_journal.lockfd is None or
                self.package_journal.intent != self.package_intent or
                self.package_journal.state.get('intent') != self.package_intent or
                self.package_intent.get('resource_context') != self.context or
                self.package_intent.get('namespace_uid') != self.namespace_uid or
                type(self.package_fence) is not resource_fencing.Fence or
                type(self.package_fence.ledger) is not resource_fencing.KubeLedger or
                type(self.package_fence.check) is not resource_fencing.ResourceCheck or
                self.package_fence.context != self.context or self.package_fence.check.context != self.context or
                self.package_fence.check.endpoint != predecessor.TT_ENDPOINT or
                type(self.package_client) is not frontend_runtime_executor.KubectlTransport or
                self.package_fence.ledger.client is not self.package_client or
                self.package_client.journal is not self.package_journal or
                self.package_phase.lock != self.lock or self.package_phase.namespace_uid != self.namespace_uid):
            raise BaselineError('private identity changed')
        predecessor._no_tt_proxy()
        guard.verify(self.fence, self.client, self.lock, self.namespace_uid)

    def _snapshot(self):
        self._guard()
        row, revision = self.fence.ledger.read()
        entry = self.journal.state['steps'].get(STEP)
        if not isinstance(entry, dict) or set(entry) != {'status', 'descriptor', 'snapshot'} or entry['status'] != 'captured':
            raise BaselineError('capture missing')
        descriptor, snapshot = entry['descriptor'], entry['snapshot']
        if (not isinstance(descriptor, dict) or set(descriptor) != {'schema','snapshot_sha256',
                    'old_row_sha256','ledger_uid','ledger_resourceVersion'} or
                not isinstance(snapshot, dict) or set(snapshot) != {'schema','operation_id','namespace_uid',
                    'ledger_uid','ledger_resourceVersion','old_row_sha256','previous','history_sha256','deployment','floors'} or
                snapshot.get('schema') != 'voice-full-cohort-image-only-snapshot-v1' or
                not self.fence.owned(row) or row.get('status') != 'active' or row.get(MARKER) != descriptor or
                descriptor.get('schema') != 'voice-full-cohort-image-only-baseline-v1' or
                descriptor.get('snapshot_sha256') != predecessor.digest(snapshot) or
                descriptor.get('old_row_sha256') != predecessor.digest(snapshot.get('previous')) or
                descriptor.get('old_row_sha256') != snapshot.get('old_row_sha256') or
                descriptor.get('ledger_uid') != snapshot.get('ledger_uid') or
                descriptor.get('ledger_resourceVersion') != snapshot.get('ledger_resourceVersion') or
                revision[0] != descriptor.get('ledger_uid') or
                snapshot.get('operation_id') != self.fence.context['operation_id'] or
                snapshot.get('namespace_uid') != self.namespace_uid):
            raise BaselineError('captured predecessor changed')
        if self.package_phase.fence.ledger.read() != (row, revision):
            raise BaselineError('package ledger differs')
        self._guard()
        return copy.deepcopy(snapshot), row, revision

    def _read(self, _context, namespace, resource):
        if namespace != 'voice-dev' or resource not in media_live_baseline.RESOURCES:
            raise BaselineError('inventory request invalid')
        roots = {'deployments':'/apis/apps/v1', 'statefulsets':'/apis/apps/v1',
                 'replicasets':'/apis/apps/v1', 'jobs':'/apis/batch/v1',
                 'pods':'/api/v1', 'pvcs':'/api/v1'}
        kinds = {'deployments':'Deployment', 'statefulsets':'StatefulSet', 'replicasets':'ReplicaSet',
                 'jobs':'Job', 'pods':'Pod', 'pvcs':'PersistentVolumeClaim'}
        path = '/api/v1/namespaces/voice-dev' if resource == 'namespace' else (
            roots[resource] + '/namespaces/voice-dev/' +
            ('persistentvolumeclaims' if resource == 'pvcs' else resource) + '?limit=500')
        self._guard()
        data = self.client.call('get', '--raw', path)
        self._guard()
        if resource != 'namespace':
            kind = kinds[resource]
            if not isinstance(data, dict) or data.get('kind') != kind + 'List':
                raise BaselineError('inventory list kind invalid')
            for item in data.get('items', []):
                if not isinstance(item, dict) or item.get('kind', kind) != kind:
                    raise BaselineError('inventory item kind invalid')
                item['kind'] = kind
        return data

    def _current(self, token):
        snapshot, row, revision = self._snapshot()
        receipt = self.package_phase.verify_current(token)
        self._guard()
        if self.package_phase.fence.ledger.read() != (row, revision):
            raise BaselineError('package ledger changed')
        actual = media_live_baseline.collect('default', 'voice-dev', reader=self._read)
        card = self.fence.authority(); observed = card.get('observed', {})
        c = self.fence.context
        if (actual.get('namespace_uid') != self.namespace_uid or observed.get('state') != 'known' or
                observed.get('actor') != c['actor'] or observed.get('claim') != c['claim'] or
                observed.get('window') != c['window'] or observed.get('operation') != card.get('operation', 0) - 1 or
                observed.get('images') != actual.get('images')):
            raise BaselineError('fresh image-only inventory differs')
        self._guard()
        if self.fence.ledger.read() != (row, revision): raise BaselineError('ledger changed during inventory')
        if self.package_phase.verify_current(token) != receipt:
            raise BaselineError('package changed during inventory')
        self._guard()
        if self.fence.ledger.read() != (row, revision): raise BaselineError('ledger changed after package verification')
        return snapshot, receipt, actual, row, revision

    def run(self, token):
        snapshot, package_receipt, actual, row, revision = self._current(token)
        if QUALIFIED in row or QUALIFIED in self.journal.state['steps']:
            raise BaselineError('qualification already recorded')
        headers = package.source._headers(token)
        deadline = time.monotonic() + 30
        package.source._head(headers, self.fence.context['commit'], deadline)
        for floor in snapshot['floors']:
            candidate = self.fence.context['commit']
            comparison = package.source._get(package.source.API + '/compare/' + floor + '...' + candidate,
                                             headers, deadline)
            if (comparison.get('base_commit', {}).get('sha') != floor or
                    comparison.get('merge_base_commit', {}).get('sha') != floor or
                    type(comparison.get('ahead_by')) is not int or
                    (floor == candidate and (comparison.get('status') != 'identical' or comparison['ahead_by'] != 0)) or
                    (floor != candidate and (comparison.get('status') != 'ahead' or comparison['ahead_by'] <= 0))):
                raise BaselineError('candidate does not descend from protected deployment')
        again, pkg_again, actual_again, current, rev = self._current(token)
        if (again != snapshot or pkg_again != package_receipt or actual_again != actual or current != row or rev != revision):
            raise BaselineError('qualification inputs changed')
        proof = {'schema':'voice-full-cohort-image-only-qualification-v1',
                 'operation_id':self.fence.context['operation_id'], 'snapshot_sha256':row[MARKER]['snapshot_sha256'],
                 'package_sha256':predecessor.digest(package_receipt), 'inventory_sha256':predecessor.digest(actual),
                 'floors':copy.deepcopy(snapshot['floors']), 'candidate':self.fence.context['commit']}
        pending = {'status':'pending', 'proof_sha256':predecessor.digest(proof)}
        marker = {'status':'done', 'proof_sha256':pending['proof_sha256']}
        try:
            self.journal.state['steps'][QUALIFIED] = {'marker':copy.deepcopy(pending), 'proof':proof}
            self.journal.save()
            self._guard()
            current, rev = self.fence.ledger.read()
            if current != row or rev != revision: raise BaselineError('qualification ledger changed')
            current[QUALIFIED] = pending
            self.fence.ledger.write(rev, current)
            self._guard()
            reserved, rev = self.fence.ledger.read()
            if reserved.get(QUALIFIED) != pending or self.journal.state['steps'][QUALIFIED] != {'marker':pending, 'proof':proof}:
                raise BaselineError('qualification reservation changed')
            reserved[QUALIFIED] = marker
            self.fence.ledger.write(rev, reserved)
            self._guard()
            confirmed, _ = self.fence.ledger.read()
            if confirmed.get(QUALIFIED) != marker:
                raise BaselineError('qualification completion changed')
            self.journal.state['steps'][QUALIFIED] = {'marker':copy.deepcopy(marker), 'proof':proof}
            self.journal.save()
            self.verify_current(token)
            return copy.deepcopy(proof)
        except BaseException:
            self.poisoned = True
            try: self.fence.uncertain('image-only baseline qualification uncertain')
            except BaseException: pass
            raise

    def verify_current(self, token):
        snapshot, receipt, actual, row, _ = self._current(token)
        entry = self.journal.state['steps'].get(QUALIFIED)
        if not isinstance(entry, dict) or set(entry) != {'marker', 'proof'}:
            raise BaselineError('qualification missing')
        marker, proof = entry['marker'], entry['proof']
        if (row.get(QUALIFIED) != marker or marker != {'status':'done', 'proof_sha256':predecessor.digest(proof)} or
                proof.get('schema') != 'voice-full-cohort-image-only-qualification-v1' or
                proof.get('operation_id') != self.fence.context['operation_id'] or
                proof.get('snapshot_sha256') != row[MARKER]['snapshot_sha256'] or
                proof.get('package_sha256') != predecessor.digest(receipt) or
                proof.get('inventory_sha256') != predecessor.digest(actual) or
                proof.get('floors') != snapshot['floors'] or proof.get('candidate') != self.fence.context['commit']):
            raise BaselineError('qualification changed')
        self._guard()
        return copy.deepcopy(proof)
