"""Fenced writer retirement and target-aware replica restoration.

The central operator owns writer selection, complete namespace Job/CronJob/HPA
inventory, shared authority and the real database observer. Admission is bound
to journal.intent['writer_cut']: controllers are exact Kubernetes objects,
guards is a dict of complete jobs/cronjobs/hpas lists projected by inventory(),
linked contains job and cronjob UIDs whose effects must be terminal, and
session_scope is the caller-qualified database/role scope. The supplied
probe(scope) must perform actual bounded read-only observation and return
{'scope': scope, 'active_sessions': 0, 'active_transactions': 0}; fake callbacks
are test fixtures only. No claim of generic external I/O quiescence is made.

Use drain() once, then verify_drained() around backup operations under the same
fence. resume(targets) accepts separately caller-admitted CURRENT workload
objects and restores only the original replica counts, never old templates.
Every effect is one-shot; after any uncertain response retain the common lock
and reconcile the private journal, ledger and actual effects without replay.
"""
import copy
import hashlib
import json
import os
import re
import secrets
import subprocess
import time

import frontend_runtime_executor
import media_operation_guard
import media_operation_journal


class RecoveryRequired(RuntimeError):
    pass


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def inventory(items):
    """Retain complete identity, owner references, spec and status privately."""
    out = []
    for item in items:
        m = item['metadata']
        out.append({'metadata': {k: copy.deepcopy(m[k]) for k in
                    ('name', 'namespace', 'uid', 'resourceVersion', 'ownerReferences', 'deletionTimestamp') if k in m},
                    'spec': copy.deepcopy(item.get('spec', {})), 'status': copy.deepcopy(item.get('status', {}))})
    return sorted(out, key=lambda x: x['metadata']['uid'])


def migration_job_inventory(initial, actual, owned_uid):
    """Extract a receipt-bound Job while requiring all prior Jobs unchanged.

    This is only an inventory comparison: the caller must independently verify
    the source-owned plan, receipt, Job specification and owner Pod.
    """
    rows = inventory(actual)
    ids = [item['metadata']['uid'] for item in rows]
    if len(ids) != len(set(ids)) or (owned_uid is not None and
            (not isinstance(owned_uid, str) or not owned_uid or owned_uid in
             {item['metadata']['uid'] for item in initial})):
        raise RecoveryRequired('migration Job inventory identity invalid')
    owned = [item for item in rows if item['metadata']['uid'] == owned_uid] if owned_uid else []
    remaining = [item for item in rows if item['metadata']['uid'] != owned_uid]
    if remaining != initial or (owned_uid is not None and len(owned) != 1):
        raise RecoveryRequired('migration Job inventory changed')
    return copy.deepcopy(owned[0]) if owned else None


class WriterCut:
    def __init__(self, journal, fence, client, lock, namespace_uid, admission, probe):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.intent, self.admission = copy.deepcopy(journal.intent), copy.deepcopy(admission)
        self.probe = self._initial_probe = probe
        self.invocation = secrets.token_hex(16)
        self.started = self.drained = self.poisoned = False
        self.retired_rs = set()
        self.successors = {}
        self.config_journal = None
        self.config_snapshot = None
        self.migration = None
        self.migration_history = []
        self.space_fresh = None
        self.resume_inventory = None
        self.auth_successor = None
        self.effective_session = None
        self.provision_successor = None
        self.pending_provision = None
        self.activation = None
        self.pending_activation_health = None
        self._identity()

    def _identity(self):
        a = self.admission
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or self.intent.get('resource_context') != self.fence.context or
                self.intent.get('namespace_uid') != self.namespace_uid or self.intent.get('writer_cut') != a or
                type(self.client) is not frontend_runtime_executor.KubectlTransport or
                self.client.journal is not self.journal or getattr(self.fence.ledger, 'client', None) is not self.client or
                not callable(self.probe) or self.probe is not self._initial_probe or
                not isinstance(a, dict) or set(a) != {'controllers', 'guards', 'linked', 'session_scope'} or
                not isinstance(a['controllers'], list) or not a['controllers'] or
                not isinstance(a['session_scope'], dict) or not a['session_scope'] or
                not isinstance(a['guards'], dict) or set(a['guards']) != {'jobs', 'cronjobs', 'hpas'} or
                not isinstance(a['linked'], dict) or set(a['linked']) != {'jobs', 'cronjobs'}):
            raise RecoveryRequired('writer cut identity invalid')
        keys = set()
        for obj in a['controllers']:
            self._workload(obj)
            key = (obj['kind'], obj['metadata']['name'])
            if key in keys: raise RecoveryRequired('duplicate writer')
            keys.add(key)
        if len({o['metadata']['uid'] for o in a['controllers']}) != len(keys):
            raise RecoveryRequired('duplicate writer UID')
        for kind in ('jobs', 'cronjobs', 'hpas'):
            rows = a['guards'][kind]
            if not isinstance(rows, list) or inventory(rows) != rows:
                raise RecoveryRequired('guard inventory invalid or unordered')
            ids = [r['metadata']['uid'] for r in rows]
            if len(ids) != len(set(ids)):
                raise RecoveryRequired('duplicate guard UID')
        for kind in ('jobs', 'cronjobs'):
            linked = a['linked'][kind]
            ids = {r['metadata']['uid'] for r in a['guards'][kind]}
            if not isinstance(linked, list) or len(linked) != len(set(linked)) or not set(linked) <= ids:
                raise RecoveryRequired('linked guard set invalid')

    @staticmethod
    def _workload(obj):
        try:
            meta, spec = obj['metadata'], obj['spec']
            name = meta['name']
            if (obj['kind'] not in ('Deployment', 'StatefulSet') or meta['namespace'] != 'voice-dev' or
                    not isinstance(name, str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9]*[a-z0-9])?', name) or
                    len(name) > 253 or not meta['uid'] or not meta['resourceVersion'] or meta.get('deletionTimestamp') or
                    type(meta.get('generation')) is not int or meta['generation'] < 1 or
                    type(spec['replicas']) is not int or spec['replicas'] < 0):
                raise RecoveryRequired('invalid writer identity')
        except (KeyError, TypeError):
            raise RecoveryRequired('invalid writer identity') from None

    def _guard(self):
        self._identity()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        if self.started:
            row, _ = self.fence.ledger.read()
            if not self._owned(row) or row.get('writer_cut_effects', {}) != self._effect_map():
                raise RecoveryRequired('writer invocation or effect map changed')

    def _effect_map(self, exclude=None):
        effects = {}
        for name, entry in self.journal.state['steps'].items():
            if not (name.startswith('writer-drain-') or name.startswith('writer-resume-') or
                    name in ('writer-cut-drained', 'writer-cut-config-plan') or
                    name.startswith('writer-cut-config-successor-') or name.startswith('writer-cut-migration-') or
                    name in ('writer-cut-resume-inventory', 'writer-cut-auth-successor', 'writer-cut-activation-plan',
                             'writer-cut-space-health', 'writer-cut-publisher-health', 'writer-cut-chat-health',
                             'writer-cut-realtime-scope', 'writer-cut-provision-scope',
                             'writer-cut-space-bind', 'writer-cut-space-finish')):
                continue
            if name == exclude: continue
            if (not isinstance(entry, dict) or entry.get('status') not in ('pending', 'done') or
                    not isinstance(entry.get('nonce'), str) or not re.fullmatch(r'[0-9a-f]{32}', entry['nonce']) or
                    entry.get('details_sha256') != digest(entry.get('details'))):
                raise RecoveryRequired('private writer effect record changed')
            record = {k: entry[k] for k in ('status', 'nonce', 'details_sha256')}
            if entry['status'] == 'done':
                if 'receipt' not in entry: raise RecoveryRequired('writer receipt missing')
                record['receipt_sha256'] = digest(entry['receipt'])
            effects[name] = record
        return effects

    def _uncertain(self):
        self.poisoned = True
        try: self.fence.uncertain('writer cut outcome uncertain')
        except BaseException: pass
        raise RecoveryRequired('writer cut uncertain; retain lock and reconcile') from None

    def _owned(self, row):
        return (self.fence.owned(row) and row.get('status') == 'active' and
                row.get('writer_cut_invocation') == {'nonce': self.invocation,
                                                      'operation_id': self.fence.context['operation_id']})

    def _start(self):
        if self.started: return
        steps = self.journal.state['steps']
        if 'writer-cut-invocation' in steps:
            raise RecoveryRequired('previous invocation requires reconciliation')
        steps['writer-cut-invocation'] = {'status': 'pending', 'nonce': self.invocation}
        self.journal.save()
        self.fence.authority()
        row, revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or 'writer_cut_invocation' in row or
                row.get('writer_cut_effects', {})):
            raise RecoveryRequired('writer invocation already owned')
        row['writer_cut_invocation'] = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
        self.fence.ledger.write(revision, row)
        self._guard()
        current, _ = self.fence.ledger.read()
        if not self._owned(current): raise RecoveryRequired('writer invocation changed')
        steps['writer-cut-invocation']['status'] = 'owned'
        self.journal.save()
        self.started = True

    def _reserve(self, name, details):
        steps = self.journal.state['steps']
        if name in steps: raise RecoveryRequired('writer effect already attempted')
        record = {'status': 'pending', 'nonce': secrets.token_hex(16), 'details_sha256': digest(details)}
        steps[name] = record | {'details': copy.deepcopy(details)}
        self.journal.save()
        self.fence.authority()
        row, rev = self.fence.ledger.read()
        effects = row.get('writer_cut_effects', {})
        if (not self._owned(row) or not isinstance(effects, dict) or name in effects or
                effects != self._effect_map(exclude=name) or
                any(not isinstance(v, dict) or v.get('status') != 'done' for v in effects.values())):
            raise RecoveryRequired('writer effect already pending')
        row['writer_cut_effects'] = copy.deepcopy(effects) | {name: record}
        row['step'] = name
        self.fence.ledger.write(rev, row)
        self._guard()
        current, _ = self.fence.ledger.read()
        if not self._owned(current) or current.get('writer_cut_effects', {}).get(name) != record:
            raise RecoveryRequired('writer reservation changed')
        return record

    def _complete(self, name, record, receipt):
        self._guard()
        row, revision = self.fence.ledger.read()
        if (not self._owned(row) or row.get('writer_cut_effects', {}) != self._effect_map() or
                not self._config_effects_match(row) or
                (self.pending_activation_health is not None and not self._health_matches(row, self.pending_activation_health)) or
                (self.pending_provision is not None and not self._provision_source_matches(row, self.pending_provision)) or
                row.get('writer_cut_effects', {}).get(name) != record):
            raise RecoveryRequired('writer reservation changed')
        if (name == 'writer-cut-config-plan' and
                (any(k.startswith('backend-config-deployment-') for k in row.get('backend_config_effects', {})) or
                 any(k.startswith('backend-config-deployment-') for k in self.config_journal.state['steps']))):
            raise RecoveryRequired('config deployment preceded bound plan')
        self.journal.state['steps'][name].update(status='done', receipt=copy.deepcopy(receipt))
        self.journal.save()
        row['writer_cut_effects'][name] = record | {'status': 'done', 'receipt_sha256': digest(receipt)}
        row['step'] = name + '-done'
        self.fence.ledger.write(revision, row)
        self._guard()

    def _get(self, kind, name):
        return self.client.call('get', kind.lower(), name, '-o', 'json')

    def _guards(self):
        mapping = {'jobs': 'jobs', 'cronjobs': 'cronjobs', 'hpas': 'horizontalpodautoscalers'}
        selected = {(o['kind'], o['metadata']['name']) for o in self.admission['controllers']}
        for kind, resource in mapping.items():
            actual = inventory(self.client.call('get', resource, '-o', 'json')['items'])
            if kind == 'jobs' and self.migration is not None:
                import writer_cut_migration
                state, pod = writer_cut_migration.observe(self, self.migration, allow_removed=self.migration['finished'])
                if not self._has_resumed_effect():
                    receipt = writer_cut_migration.sessions(self, state, pod)
                    if self.migration['finished'] and receipt['attributed_sessions']:
                        raise RecoveryRequired('terminal migration sessions remain')
                uid = state['job']['uid'] if state['job']['state'] in ('owned', 'done', 'removing') else None
                actual = [item for item in actual if item['metadata']['uid'] != uid]
            if kind == 'jobs' and self.space_fresh is not None:
                import writer_cut_space
                actual=writer_cut_space.guard_jobs(self,actual)
            if actual != self.admission['guards'][kind]:
                raise RecoveryRequired('guard inventory changed')
            for item in actual:
                uid, spec, status = item['metadata']['uid'], item['spec'], item['status']
                if kind == 'hpas' and (spec.get('scaleTargetRef', {}).get('kind'),
                                       spec.get('scaleTargetRef', {}).get('name')) in selected:
                    raise RecoveryRequired('writer HPA must be separately retired')
                if kind == 'cronjobs' and uid in self.admission['linked']['cronjobs']:
                    if spec.get('suspend') is not True or status.get('active'):
                        raise RecoveryRequired('linked CronJob not suspended')
                if kind == 'jobs' and uid in self.admission['linked']['jobs']:
                    terminal = any(c.get('type') in ('Complete', 'Failed') and c.get('status') == 'True'
                                   for c in status.get('conditions', []))
                    if status.get('active', 0) or not terminal:
                        raise RecoveryRequired('linked Job not terminal')
        if self.migration_history:
            import writer_cut_migration
            for binding in self.migration_history:
                writer_cut_migration.verify_cleaned(self, binding)
                if self.migration is not None and not self._has_resumed_effect():
                    _, state = writer_cut_migration.normalized(self, binding['kind'], binding['phase'], binding['root'])
                    database = state['execution']['database']
                    if self.admission['session_scope'].get('database_oids', {}).get(database) != state['execution']['database_oid']:
                        raise RecoveryRequired('previous migration database is outside pinned scope')
        if self._has_resumed_effect() and (self.migration is not None or self.migration_history):
            self._resumed_sessions()
        if self.auth_successor is not None:
            self._recheck_auth_successor()

    def _patch(self, kind, name, patch):
        filename = '.writer-patch-' + secrets.token_hex(16)
        fd = os.open(filename, os.O_CREAT | os.O_EXCL | os.O_WRONLY | os.O_NOFOLLOW, 0o600, dir_fd=self.journal.dirfd)
        try:
            with os.fdopen(fd, 'wb') as stream:
                stream.write(json.dumps(patch, separators=(',', ':')).encode())
                stream.flush()
                os.fsync(stream.fileno())
            os.fsync(self.journal.dirfd)
            path = str(self.journal.path / filename)
            result = subprocess.run(frontend_runtime_executor.KubectlTransport.PREFIX +
                ['patch', kind.lower(), name, '--type=json', '--patch-file=' + path, '-o', 'json'],
                stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=25, check=True)
            return json.loads(result.stdout)
        finally:
            os.unlink(filename, dir_fd=self.journal.dirfd)
            os.fsync(self.journal.dirfd)

    def _effect(self, phase, source, replicas):
        kind, meta = source['kind'], source['metadata']
        original = copy.deepcopy(source['spec'])
        target = copy.deepcopy(original)
        target['replicas'] = replicas
        patch = [{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
                 {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
                 {'op': 'test', 'path': '/spec', 'value': original},
                 {'op': 'replace', 'path': '/spec/replicas', 'value': replicas}]
        name = 'writer-' + phase + '-' + kind.lower() + '-' + meta['name']
        record = self._reserve(name, {'source': source, 'target_spec': target})
        self._guards()
        self._recheck_config_successors()
        if phase == 'resume':
            self._migration_resume_guard()
            if self.activation is not None:
                self._activation_verify_sources()
        self._guard()
        row, _ = self.fence.ledger.read()
        if (not self._owned(row) or row.get('writer_cut_effects', {}) != self._effect_map() or
                not self._config_effects_match(row) or
                (phase == 'resume' and not self._resume_inventory_matches(row)) or
                (phase == 'resume' and not self._auth_successor_matches(row)) or
                (phase == 'resume' and not self._effective_scope_matches(row)) or
                (phase == 'resume' and not self._provision_scope_matches(row)) or
                (phase == 'resume' and not self._activation_matches(row)) or
                row.get('writer_cut_effects', {}).get(name) != record):
            raise RecoveryRequired('writer reservation changed before submission')
        result = self._patch(kind, meta['name'], patch)
        actual = self._get(kind, meta['name'])
        for item in (result, actual):
            if (item.get('kind') != kind or item.get('metadata', {}).get('uid') != meta['uid'] or
                    item.get('metadata', {}).get('namespace') != 'voice-dev' or
                    item.get('metadata', {}).get('name') != meta['name'] or
                    item.get('metadata', {}).get('deletionTimestamp') or
                    not item.get('metadata', {}).get('resourceVersion') or
                    type(item.get('metadata', {}).get('generation')) is not int or item.get('spec') != target):
                raise RecoveryRequired('writer effect identity mismatch')
        if result['metadata']['generation'] != actual['metadata']['generation']:
            raise RecoveryRequired('writer generation changed before reobservation')
        if actual['metadata']['generation'] != meta['generation'] + (original['replicas'] != replicas):
            raise RecoveryRequired('unexpected writer generation transition')
        receipt = {'uid': meta['uid'], 'spec_sha256': digest(target),
                   'responseResourceVersion': result['metadata']['resourceVersion'],
                   'resourceVersion': actual['metadata']['resourceVersion'], 'generation': actual['metadata']['generation']}
        self._complete(name, record, receipt)
        self._recheck_config_successors()
        return receipt

    @staticmethod
    def _owner(meta, kind):
        owners = [o for o in meta.get('ownerReferences', []) if o.get('controller') is True and o.get('kind') == kind]
        return owners[0].get('uid') if len(owners) == 1 else None

    def _pods_drained(self, controllers=None, include_retired=True):
        controllers = self.admission['controllers'] if controllers is None else controllers
        deployment_ids = {o['metadata']['uid'] for o in controllers if o['kind'] == 'Deployment'}
        stateful_ids = {o['metadata']['uid'] for o in controllers if o['kind'] == 'StatefulSet'}
        rs = self.client.call('get', 'replicasets', '-o', 'json')['items']
        all_rs = {r['metadata']['uid'] for r in rs}
        rs_ids = (self.retired_rs if include_retired else set()) | {r['metadata']['uid'] for r in rs if self._owner(r['metadata'], 'Deployment') in deployment_ids}
        pods = self.client.call('get', 'pods', '-o', 'json')['items']
        for pod in pods:
            meta = pod['metadata']
            owners = [o for o in meta.get('ownerReferences', []) if o.get('controller') is True]
            if (pod.get('status', {}).get('phase') not in ('Succeeded', 'Failed') and
                    (len(owners) != 1 or owners[0].get('kind') not in ('ReplicaSet', 'StatefulSet', 'Job', 'DaemonSet')
                     or not owners[0].get('uid'))):
                return False
            rs_owner = self._owner(meta, 'ReplicaSet')
            if rs_owner and rs_owner not in all_rs and pod.get('status', {}).get('phase') not in ('Succeeded', 'Failed'):
                return False
            if self._owner(meta, 'ReplicaSet') in rs_ids or self._owner(meta, 'StatefulSet') in stateful_ids:
                return False
            if self._owner(meta, 'Job') in self.admission['linked']['jobs']:
                if pod.get('status', {}).get('phase') not in ('Succeeded', 'Failed'):
                    return False
        return True

    def _retired(self):
        for original in self.admission['controllers']:
            actual = self._get(original['kind'], original['metadata']['name'])
            expected = copy.deepcopy(original['spec'])
            expected['replicas'] = 0
            meta, status = actual.get('metadata', {}), actual.get('status', {})
            name = 'writer-drain-' + original['kind'].lower() + '-' + original['metadata']['name']
            saved = self.journal.state['steps'].get(name, {})
            successor = self.successors.get(original['metadata']['name']) if original['kind'] == 'Deployment' else None
            if successor:
                expected = copy.deepcopy(successor['target_spec'])
                expected_generation = successor['receipt']['generation']
            else:
                expected_generation = saved.get('receipt', {}).get('generation')
            if (meta.get('uid') != original['metadata']['uid'] or meta.get('namespace') != 'voice-dev'
                    or actual.get('spec') != expected or meta.get('deletionTimestamp') or
                    saved.get('status') != 'done' or meta.get('generation') != expected_generation):
                raise RecoveryRequired('retired writer changed')
            generation, observed = meta.get('generation'), status.get('observedGeneration')
            if (type(generation) is not int or type(observed) is not int or observed < generation
                    or status.get('replicas', 0) != 0 or status.get('readyReplicas', 0) != 0):
                return False
        return self._pods_drained()

    def _validate_config_snapshot(self, config_journal, snapshot, selected):
        import backend_config_transition

        successors = {}
        for service in snapshot['services']:
            name = 'voice-' + service
            step = 'backend-config-deployment-' + name
            entry = snapshot['steps'].get(step)
            ledger = snapshot['effects'].get(step)
            if service in ('notification', 'user'):
                proof = backend_config_transition.verify_deployment_successor(self, config_journal, service)
            elif service in ('chat', 'realtime', 'space'):
                import realtime_config_transition
                proof = realtime_config_transition.verify_deployment_successor(self, config_journal, service)
            else:
                raise RecoveryRequired('unknown config successor')
            if (not isinstance(entry, dict) or set(entry) != {'status', 'nonce', 'details', 'details_sha256', 'receipt'} or
                    entry['status'] != 'done' or not isinstance(entry['nonce'], str) or
                    not re.fullmatch(r'[0-9a-f]{32}', entry['nonce']) or
                    entry['details_sha256'] != digest(entry['details']) or
                    not isinstance(entry['details'], dict) or set(entry['details']) !=
                    {'operation_id', 'invocation_nonce', 'source', 'target_spec', 'patch'} or
                    not isinstance(entry['receipt'], dict) or set(entry['receipt']) !=
                    {'uid', 'spec_sha256', 'responseResourceVersion', 'resourceVersion', 'generation'}):
                raise RecoveryRequired('config effect journal record invalid')
            details, receipt = entry['details'], entry['receipt']
            expected_ledger = {'status': 'done', 'nonce': entry['nonce'], 'details_sha256': entry['details_sha256'],
                               'receipt_sha256': digest(receipt)}
            if (ledger != expected_ledger or not isinstance(proof, dict) or
                    set(proof) != {'source', 'target_spec', 'receipt', 'ledger_record'} or
                    proof != {'source': details['source'], 'target_spec': details['target_spec'],
                              'receipt': receipt, 'ledger_record': ledger} or
                    details['operation_id'] != self.fence.context['operation_id'] or
                    details['invocation_nonce'] != self.invocation):
                raise RecoveryRequired('config effect ownership or receipt changed')
            source, target = details['source'], details['target_spec']
            self._workload(source)
            original = selected[name]
            cut_step = self.journal.state['steps'].get('writer-drain-deployment-' + name, {})
            cut_receipt, cut_details = cut_step.get('receipt', {}), cut_step.get('details', {})
            meta = source['metadata']
            patch = [{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
                     {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
                     {'op': 'test', 'path': '/spec', 'value': source['spec']},
                     {'op': 'replace', 'path': '/spec', 'value': target}]
            if (source['kind'] != 'Deployment' or meta['name'] != name or meta['uid'] != original['metadata']['uid'] or
                    cut_step.get('status') != 'done' or cut_receipt.get('uid') != meta['uid'] or
                    cut_receipt.get('generation') != meta['generation'] or cut_details.get('target_spec') != source['spec'] or
                    cut_receipt.get('spec_sha256') != digest(source['spec']) or details['patch'] != patch or
                    not isinstance(target, dict) or target == source['spec'] or target.get('replicas') != 0 or
                    receipt['uid'] != meta['uid'] or receipt['spec_sha256'] != digest(target) or
                    type(receipt['generation']) is not int or receipt['generation'] != meta['generation'] + 1 or
                    not isinstance(receipt['responseResourceVersion'], str) or not receipt['responseResourceVersion'] or
                    not isinstance(receipt['resourceVersion'], str) or not receipt['resourceVersion']):
                raise RecoveryRequired('config successor does not follow original cut')
            successors[name] = copy.deepcopy(proof)
        return successors

    def _recheck_config_successors(self):
        if self.config_journal is None:
            if self.successors or self.config_snapshot is not None:
                raise RecoveryRequired('config successor custody missing')
            return
        config = self.config_journal
        snapshot = self.config_snapshot
        if (config.poisoned or config.dirfd is None or config.lockfd is None or
                config.intent != snapshot['intent'] or config.state.get('intent') != snapshot['intent'] or
                any(config.state.get('steps', {}).get(k) != v for k, v in snapshot['steps'].items())):
            raise RecoveryRequired('config journal changed')
        row, _ = self.fence.ledger.read()
        effects = row.get('backend_config_effects', {})
        if not self._owned(row) or not self._config_effects_match(row):
            raise RecoveryRequired('config ledger changed')
        selected = {o['metadata']['name']: o for o in self.admission['controllers'] if o['kind'] == 'Deployment'}
        if self._validate_config_snapshot(config, snapshot, selected) != self.successors:
            raise RecoveryRequired('config successor proof changed')

    def _config_effects_match(self, row):
        if self.config_snapshot is None:
            return True
        config, snapshot = self.config_journal, self.config_snapshot
        effects = row.get('backend_config_effects', {})
        matches = (config is not None and not config.poisoned and config.dirfd is not None and config.lockfd is not None
                and config.intent == snapshot['intent'] and config.state.get('intent') == snapshot['intent']
                and isinstance(effects, dict) and
                all(config.state.get('steps', {}).get(k) == v for k, v in snapshot['steps'].items()) and
                all(effects.get(k) == v for k, v in snapshot['effects'].items()))
        if not matches:
            return False
        if snapshot['services'] == snapshot['intent']['backend_config_deployments']['services']:
            expected = set(snapshot['steps'])
            prefix = 'backend-config-deployment-'
            return ({k for k in config.state['steps'] if k.startswith(prefix)} == expected and
                    {k for k in effects if k.startswith(prefix)} == expected)
        return True

    def bind_config_plan(self, config_journal):
        """Durably require a fixed producer plan before its first config effect."""
        try:
            if not self.drained or self.config_journal is not None:
                raise RecoveryRequired('drained writers and unused config plan required')
            self._guard()
            self._guards()
            if not self._retired(): raise RecoveryRequired('original cut no longer retired')
            if (type(config_journal) is not media_operation_journal.Journal or config_journal is self.journal or
                    config_journal.poisoned or config_journal.dirfd is None or config_journal.lockfd is None or
                    config_journal.intent != config_journal.state.get('intent')):
                raise RecoveryRequired('open separate config journal required')
            intent = config_journal.intent
            plan = intent.get('backend_config_deployments')
            if (intent.get('resource_context') != self.fence.context or intent.get('namespace_uid') != self.namespace_uid or
                    not isinstance(plan, dict) or plan.get('source_sha') != self.fence.context['commit'] or
                    plan.get('cut_invocation') != self.invocation or not isinstance(plan.get('services'), list) or
                    not plan['services'] or len(plan['services']) != len(set(plan['services'])) or
                    set(plan['services']) - {'user', 'notification', 'realtime', 'chat', 'space'}):
                raise RecoveryRequired('config source or cut identity changed')
            selected = {o['metadata']['name'] for o in self.admission['controllers'] if o['kind'] == 'Deployment'}
            if any('voice-' + s not in selected for s in plan['services']):
                raise RecoveryRequired('config plan includes nonwriter')
            row, _ = self.fence.ledger.read()
            effects = row.get('backend_config_effects', {})
            if (not self._owned(row) or not isinstance(effects, dict) or
                    any(k.startswith('backend-config-deployment-') for k in effects) or
                    any(k.startswith('backend-config-deployment-') for k in config_journal.state['steps'])):
                raise RecoveryRequired('config deployment already attempted')
            self.verify_drained()
            snapshot = {'intent': copy.deepcopy(intent), 'steps': {}, 'effects': {}, 'services': []}
            record = self._reserve('writer-cut-config-plan', snapshot)
            self.config_journal, self.config_snapshot = config_journal, snapshot
            receipt = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': self.invocation,
                       'services': copy.deepcopy(plan['services']), 'config_intent_sha256': digest(intent)}
            self._complete('writer-cut-config-plan', record, receipt)
            return copy.deepcopy(receipt)
        except BaseException:
            self._uncertain()

    def bind_space_fresh(self, phase, token):
        import writer_cut_space
        return writer_cut_space.bind(self,phase,token)

    def verify_space_pending(self):
        import writer_cut_space
        return writer_cut_space.pending(self)

    def verify_space_running(self):
        import writer_cut_space
        return writer_cut_space.running(self)

    def finish_space_fresh(self):
        import writer_cut_space
        return writer_cut_space.finish(self)

    def bind_migration(self, kind, phase, source_root):
        """Bind a source-owned migration plan before its Job is submitted."""
        import writer_cut_migration as migration
        try:
            if self.migration is not None:
                raise RecoveryRequired('previous migration not cleaned up')
            self.verify_drained()
            _, state = migration.normalized(self, kind, phase, source_root)
            if state['job']['state'] != 'absent':
                raise RecoveryRequired('migration Job already attempted')
            config_plan = state['configmap'].get('authored', state['configmap'].get('receipt'))
            plan = {'authored': copy.deepcopy(state['job']['authored']), 'config': copy.deepcopy(config_plan),
                    'execution': copy.deepcopy(state['execution']),
                    'stable': copy.deepcopy(state['stable'])}
            binding = {'kind': kind, 'phase': phase, 'root': source_root, 'plan': plan, 'finished': False,
                       'job_uid': None, 'pod_uid': None, 'config_uid': state['configmap'].get('uid')}
            self.migration = binding
            state, pod = migration.observe(self, binding)
            receipt = migration.sessions(self, state, pod)
            if receipt['attributed_sessions']:
                raise RecoveryRequired('migration sessions precede Job')
            step = 'writer-cut-migration-plan-' + state['stable']['nonce']
            record = self._reserve(step, {'kind': kind, 'plan': plan})
            self._complete(step, record, {'operation_id': self.fence.context['operation_id'],
                                           'plan_sha256': digest(plan)})
            return copy.deepcopy(plan)
        except BaseException:
            self._uncertain()

    def verify_migration_running(self):
        """Reobserve the registered Job, all writer guards and scoped sessions."""
        import writer_cut_migration as migration
        try:
            if not self.drained or self.migration is None:
                raise RecoveryRequired('migration plan absent')
            self._guard()
            self._guards()
            self._recheck_config_successors()
            if not self._retired(): raise RecoveryRequired('writers changed')
            state, pod = migration.observe(self, self.migration, allow_removed=self.migration['finished'])
            job_uid, config_uid = state['job'].get('uid'), state['configmap'].get('uid')
            for key, uid in (('job_uid', job_uid), ('config_uid', config_uid)):
                old = self.migration[key]
                if old is not None and uid != old: raise RecoveryRequired('migration UID changed')
                if uid is not None: self.migration[key] = uid
            if pod:
                if self.migration['pod_uid'] not in (None, pod['uid']):
                    raise RecoveryRequired('migration Pod changed')
                self.migration['pod_uid'] = pod['uid']
            # Identity has only three startup ranks: absent, Pending, runtime.
            # Every resample must strictly advance, so at most two are possible.
            startup_rank = 0 if pod is None else 1 if not pod.get('ip') else 2
            while True:
                receipt = migration.sessions(self, state, pod)
                if self.migration['finished'] and receipt['attributed_sessions']:
                    raise RecoveryRequired('terminal migration sessions remain')
                self._guard()
                self._guards()
                self._recheck_config_successors()
                after, next_pod = migration.observe(self, self.migration, allow_removed=self.migration['finished'])
                if state == after and pod == next_pod and self._retired():
                    return receipt
                # Keep the complete sample stable; only validated monotonic startup
                # edges permit another full sample, never a UID or established-IP change.
                next_rank = 0 if next_pod is None else 1 if not next_pod.get('ip') else 2
                progressing = (self.migration['kind'] == 'realtime' and not self.migration['finished']
                    and state == after and state['job']['state'] == 'owned' and not state['terminal']
                    and next_rank > startup_rank and next_pod and ((pod is None and self.migration['pod_uid'] is None)
                        or (pod and not pod.get('ip') and next_pod['uid'] == pod['uid'] and next_pod.get('ip'))))
                if not progressing or not self._retired():
                    raise RecoveryRequired('migration changed during session observation')
                self.migration['pod_uid'] = next_pod['uid']
                state, pod, startup_rank = after, next_pod, next_rank

        except BaseException:
            self._uncertain()

    def current_resumed_proof(self):
        """Return only completed resume effects with exact current controller state."""
        self._guard()
        row, _ = self.fence.ledger.read()
        if not self._owned(row) or row.get('writer_cut_effects') != self._effect_map():
            raise RecoveryRequired('resume effect map changed')
        admitted = {(item['kind'], item['metadata']['name']): item for item in self.admission['controllers']}
        result = []
        for step, entry in sorted(self.journal.state['steps'].items()):
            if not step.startswith('writer-resume-'):
                continue
            if entry.get('status') == 'pending':
                continue
            if entry.get('status') != 'done': raise RecoveryRequired('resume effect invalid')
            source = entry['details']['source']; spec = entry['details']['target_spec']; receipt = entry['receipt']
            key = (source['kind'], source['metadata']['name'])
            if (key not in admitted or step != 'writer-resume-' + key[0].lower() + '-' + key[1] or
                    source['spec'].get('replicas') != 0 or spec.get('replicas') != admitted[key]['spec']['replicas'] or
                    receipt.get('uid') != source['metadata']['uid'] or receipt.get('spec_sha256') != digest(spec)):
                raise RecoveryRequired('resume receipt differs from admission')
            target = copy.deepcopy(source['spec']); target['replicas'] = admitted[key]['spec']['replicas']
            current = self._get(*key); meta = current.get('metadata', {})
            if (target != spec or current.get('kind') != key[0] or meta.get('name') != key[1] or
                    meta.get('namespace') != 'voice-dev' or meta.get('uid') != receipt['uid'] or
                    meta.get('generation') != receipt['generation'] or meta.get('deletionTimestamp') or
                    not meta.get('resourceVersion') or current.get('spec') != spec):
                raise RecoveryRequired('resumed controller changed')
            result.append({'kind': key[0], 'name': key[1], 'uid': receipt['uid'],
                           'generation': receipt['generation'], 'spec': copy.deepcopy(spec),
                           'replicas': spec['replicas'], 'nonce': entry['nonce'], 'receipt_sha256': digest(receipt)})
        self._guard()
        return {'operation_id': self.fence.context['operation_id'], 'namespace_uid': self.namespace_uid,
                'invocation': self.invocation, 'controllers': result}

    def bind_auth_successor(self, phase, source_root):
        """Pin the source-owned completed Auth proof for C1 or C2."""
        import auth_live_migration_phase
        try:
            if self.auth_successor is not None or not self.drained or any(
                    k.startswith('writer-resume-') for k in self.journal.state['steps']):
                raise RecoveryRequired('Auth successor binding unavailable')
            self._guard()
            proof = auth_live_migration_phase.verify_completed(phase, self, source_root)
            row, _ = self.fence.ledger.read()
            markers = {k: copy.deepcopy(row.get(k)) for k in ('auth_live_c1', 'auth_live_c2', 'cohort_readback',
                'cohort_restore', 'cohort_restore_resources', 'cohort_auth_flyway', 'full_cohort_package')}
            steps = copy.deepcopy(phase.phases.journal.state['steps'])
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': self.invocation,
                       'receipt_sha256': digest(proof)}
            record = self._reserve('writer-cut-auth-successor', details)
            self._complete('writer-cut-auth-successor', record, details)
            self.auth_successor = {'phase': phase, 'root': source_root, 'receipt_sha256': digest(proof),
                                   'proof': copy.deepcopy(proof), 'steps': steps, 'markers': markers}
            self._recheck_auth_successor()
            return copy.deepcopy(details)
        except BaseException:
            self._uncertain()

    def _auth_successor_matches(self, row):
        if self.auth_successor is None:
            return True
        binding = self.auth_successor
        journal = binding['phase'].phases.journal
        return (not journal.poisoned and journal.dirfd is not None and journal.lockfd is not None and
                digest(binding['proof']) == binding['receipt_sha256'] and
                journal.state['steps'] == binding['steps'] and
                all(row.get(k) == value for k, value in binding['markers'].items()))

    def _recheck_auth_successor(self):
        import auth_live_migration_phase
        binding = self.auth_successor
        row, _ = self.fence.ledger.read()
        if not self._auth_successor_matches(row): raise RecoveryRequired('Auth successor custody changed')
        proof = auth_live_migration_phase.verify_completed(binding['phase'], self, binding['root'])
        if digest(proof) != binding['receipt_sha256']:
            raise RecoveryRequired('Auth successor proof changed')
        row, _ = self.fence.ledger.read()
        if not self._auth_successor_matches(row): raise RecoveryRequired('Auth successor changed during check')

    def bind_provision_scope(self, phase):
        """Bind an actual create-only Realtime DB before migration starts."""
        import realtime_db_provision
        import resource_fencing
        try:
            if (not self.drained or self.provision_successor is not None or self.migration is not None or
                    type(phase) is not realtime_db_provision.Provision or
                    type(phase.pg) is not realtime_db_provision.KubePostgres or
                    type(phase.journal) is not media_operation_journal.Journal or
                    type(phase.client) is not frontend_runtime_executor.KubectlTransport or
                    type(phase.fence) is not resource_fencing.Fence or
                    type(phase.fence.ledger) is not resource_fencing.KubeLedger or
                    phase.client.journal is not phase.journal or phase.fence.ledger.client is not phase.client or
                    phase.fence.context != self.fence.context or phase.lock != self.lock or phase.namespace_uid != self.namespace_uid):
                raise RecoveryRequired('provision phase identity invalid')
            self._guard(); self._guards()
            if not self._retired(): raise RecoveryRequired('writers are not retired')
            left, lrev = self.fence.ledger.read(); right, rrev = phase.fence.ledger.read()
            if left != right or lrev != rrev or not self._owned(left): raise RecoveryRequired('provision ledger differs')
            proof = realtime_db_provision.verify_completed(phase)
            scope = self.current_base_session_scope()
            if (proof.get('operation_id') != self.fence.context['operation_id'] or proof.get('postgres') != scope['engine'] or
                    proof.get('database') != 'realtime_db' or proof.get('role') != 'realtime' or
                    proof.get('scope') != 'create-only' or type(proof.get('database_oid')) is not int or proof['database_oid'] <= 0):
                raise RecoveryRequired('provision proof differs')
            existing = scope['database_oids'].get('realtime_db')
            if existing is not None and existing != proof['database_oid']: raise RecoveryRequired('Realtime OID changed')
            scope['database_oids']['realtime_db'] = proof['database_oid']; scope['databases'] = sorted(scope['database_oids'])
            if len(set(scope['database_oids'].values())) != len(scope['database_oids']): raise RecoveryRequired('database OID collision')
            row, rev = self.fence.ledger.read(); other, other_rev = phase.fence.ledger.read()
            if row != other or rev != other_rev or not self._owned(row): raise RecoveryRequired('provision authority changed')
            markers = {k: copy.deepcopy(row.get(k)) for k in ('realtime_db_provision', 'realtime_secret')}
            steps = copy.deepcopy(phase.journal.state['steps'])
            source_binding = {'phase': phase, 'steps': steps, 'markers': markers, 'intent': copy.deepcopy(phase.intent),
                              'client': phase.client, 'fence': phase.fence, 'postgres': phase.pg, 'nonce': phase.nonce}
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': self.invocation,
                       'original_scope_sha256': digest(self.admission['session_scope']), 'scope_sha256': digest(scope),
                       'provision_receipt_sha256': digest(proof)}
            record = self._reserve('writer-cut-provision-scope', details)
            if realtime_db_provision.verify_completed(phase) != proof:
                raise RecoveryRequired('provision proof changed before binding')
            self.pending_provision = source_binding
            self._complete('writer-cut-provision-scope', record, details)
            self.pending_provision = None
            self.provision_successor = source_binding | {'scope': scope, 'proof': copy.deepcopy(proof),
                                                        'details': copy.deepcopy(details)}
            self.current_provisioned_session_scope()
            receipt = self._session_observation()
            if receipt != {'scope': scope, 'active_sessions': 0, 'active_transactions': 0} or \
                    type(receipt['active_sessions']) is not int or type(receipt['active_transactions']) is not int:
                raise RecoveryRequired('provisioned catalog is not drained')
            return copy.deepcopy(details)
        except BaseException:
            self._uncertain()

    def current_base_session_scope(self):
        """Return original scope plus only a completed source-owned Space replacement."""
        scope=copy.deepcopy(self.admission['session_scope'])
        if self.space_fresh is None:return scope
        import writer_cut_space
        import space_fresh_install_phase
        binding=writer_cut_space.verify_binding(self,require_finished=True)
        proof=space_fresh_install_phase.verify_historical(binding['phase'],binding['token'])
        if (proof['operation_id']!=self.fence.context['operation_id'] or proof['source_sha']!=self.fence.context['commit'] or
                proof['namespace_uid']!=self.namespace_uid or proof['receipt_sha256']!=binding['receipt_sha256'] or
                scope['database_oids'].get('space_db')!=proof['old_oid'] or type(proof['new_oid']) is not int or
                proof['new_oid']<=0 or proof['new_oid']==proof['old_oid']):
            raise RecoveryRequired('Space session scope changed')
        scope['database_oids']['space_db']=proof['new_oid'];scope['databases']=sorted(scope['database_oids'])
        if len(set(scope['database_oids'].values()))!=len(scope['database_oids']):raise RecoveryRequired('database OID collision')
        if writer_cut_space.verify_binding(self,require_finished=True)!=binding:raise RecoveryRequired('Space binding changed')
        return scope

    @staticmethod
    def _provision_source_matches(row, binding):
        phase = binding['phase']; source = phase.journal
        return (not source.poisoned and source.dirfd is not None and source.lockfd is not None and
                phase.client is binding['client'] and phase.fence is binding['fence'] and phase.pg is binding['postgres'] and
                phase.nonce == binding['nonce'] and phase.intent == binding['intent'] and
                source.intent == binding['intent'] and source.state.get('intent') == binding['intent'] and
                source.state['steps'] == binding['steps'] and
                all(row.get(k) == value for k, value in binding['markers'].items()))

    def _provision_scope_matches(self, row):
        if self.provision_successor is None: return True
        binding = self.provision_successor
        step = self.journal.state['steps'].get('writer-cut-provision-scope', {})
        return (self._provision_source_matches(row, binding) and step.get('status') == 'done' and
                step.get('details') == binding['details'] and step.get('receipt') == binding['details'] and
                binding['details']['original_scope_sha256'] == digest(self.admission['session_scope']) and
                binding['details']['scope_sha256'] == digest(binding['scope']) and
                binding['details']['provision_receipt_sha256'] == digest(binding['proof']))

    def current_provisioned_session_scope(self):
        import realtime_db_provision
        self._guard(); row, _ = self.fence.ledger.read()
        if self.provision_successor is None or not self._provision_scope_matches(row):
            raise RecoveryRequired('provision scope unavailable')
        binding = self.provision_successor
        proof = realtime_db_provision.verify_completed(binding['phase'])
        expected = self.current_base_session_scope()
        expected['database_oids']['realtime_db'] = proof['database_oid']; expected['databases'] = sorted(expected['database_oids'])
        if proof != binding['proof'] or expected != binding['scope']:
            raise RecoveryRequired('provision scope changed')
        self._guard(); row, _ = self.fence.ledger.read()
        if not self._provision_scope_matches(row): raise RecoveryRequired('provision scope changed during observation')
        return copy.deepcopy(binding['scope'])

    def bind_realtime_scope(self):
        """Bind a newly provisioned Realtime DB from its completed migration."""
        import writer_cut_migration as migration
        try:
            binding = self.migration
            if (self.effective_session is not None or self.provision_successor is None or
                    binding is None or binding['kind'] != 'realtime' or
                    not binding['finished'] or any(k.startswith('writer-resume-') for k in self.journal.state['steps'])):
                raise RecoveryRequired('Realtime scope successor unavailable')
            self._guard(); self._guards()
            _, state = migration.normalized(self, 'realtime', binding['phase'], binding['root'])
            execution = state['execution']; phase = binding['phase']; scope = self.current_base_session_scope()
            if (phase.invocation_status != 'done' or state['job']['state'] != 'done' or
                    state['terminal'] != binding['terminal'] or execution['database'] != 'realtime_db' or
                    execution['role'] != 'realtime' or type(execution['database_oid']) is not int or
                    execution['database_oid'] <= 0 or phase.intent['realtime_migration']['engine'] != scope['engine']):
                raise RecoveryRequired('Realtime scope authority differs')
            prior = scope['database_oids'].get('realtime_db')
            if prior is not None and prior != execution['database_oid']:
                raise RecoveryRequired('Realtime database OID changed')
            scope['database_oids']['realtime_db'] = execution['database_oid']
            scope['databases'] = sorted(scope['database_oids'])
            if len(set(scope['database_oids'].values())) != len(scope['database_oids']):
                raise RecoveryRequired('database OID collision')
            if self.current_provisioned_session_scope() != scope:
                raise RecoveryRequired('migration and provision scopes differ')
            row, _ = self.fence.ledger.read()
            markers = {k: copy.deepcopy(row.get(k)) for k in ('realtime_migration', 'realtime_secret', 'realtime_db_provision')}
            steps = copy.deepcopy(phase.journal.state['steps'])
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': self.invocation,
                       'original_scope_sha256': digest(self.admission['session_scope']), 'scope_sha256': digest(scope),
                       'migration_terminal_sha256': state['terminal']}
            record = self._reserve('writer-cut-realtime-scope', details)
            self._complete('writer-cut-realtime-scope', record, details)
            self.effective_session = {'scope': scope, 'phase': phase, 'steps': steps, 'markers': markers,
                                      'terminal': state['terminal'], 'details': copy.deepcopy(details)}
            self.current_effective_session_scope()
            return copy.deepcopy(details)
        except BaseException:
            self._uncertain()

    def _effective_scope_matches(self, row):
        if self.effective_session is None:
            return (self.provision_successor is None and
                    (self.migration is None or self.migration.get('kind') != 'realtime') and
                    not any(o['metadata']['name'] == 'voice-realtime' for o in self.admission['controllers']))
        binding = self.effective_session; source = binding['phase'].journal
        step = self.journal.state['steps'].get('writer-cut-realtime-scope', {})
        return (not source.poisoned and source.dirfd is not None and source.lockfd is not None and
                source.state['steps'] == binding['steps'] and
                step.get('status') == 'done' and step.get('details') == binding['details'] and
                step.get('receipt') == binding['details'] and
                binding['details']['scope_sha256'] == digest(binding['scope']) and
                binding['details']['original_scope_sha256'] == digest(self.admission['session_scope']) and
                all(row.get(k) == value for k, value in binding['markers'].items()))

    def current_effective_session_scope(self):
        import writer_cut_migration as migration
        self._guard()
        row, _ = self.fence.ledger.read()
        if not self._effective_scope_matches(row):
            raise RecoveryRequired('Realtime session scope is not qualified')
        if self.effective_session is None:
            return self.current_base_session_scope()
        binding = self.effective_session
        _, state = migration.normalized(self, 'realtime', binding['phase'], self.migration['root'])
        expected = self.current_base_session_scope()
        expected['database_oids']['realtime_db'] = state['execution']['database_oid']
        expected['databases'] = sorted(expected['database_oids'])
        if (not self.migration['finished'] or state['terminal'] != binding['terminal'] or
                binding['details']['migration_terminal_sha256'] != state['terminal'] or binding['scope'] != expected or
                self.current_provisioned_session_scope() != expected):
            raise RecoveryRequired('Realtime scope successor changed')
        self._guard()
        row, _ = self.fence.ledger.read()
        if not self._effective_scope_matches(row): raise RecoveryRequired('Realtime scope changed during observation')
        return copy.deepcopy(binding['scope'])

    def bind_resume_inventory(self, inventory_phase, token):
        """Bind authenticated pre-cut routes without persisting the token."""
        import full_cohort_inventory_phase
        try:
            if (not self.drained or self.resume_inventory is not None or
                    type(inventory_phase) is not full_cohort_inventory_phase.InventoryPhase or
                    any(k.startswith('writer-resume-') for k in self.journal.state['steps']) or
                    inventory_phase.fence.context != self.fence.context or inventory_phase.lock != self.lock or
                    inventory_phase.namespace_uid != self.namespace_uid or
                    inventory_phase.fence.ledger.client is not inventory_phase.client):
                raise RecoveryRequired('resume inventory identity invalid')
            self._guard()
            left, lrev = self.fence.ledger.read()
            right, rrev = inventory_phase.fence.ledger.read()
            if left != right or lrev != rrev or not self._owned(left):
                raise RecoveryRequired('resume inventory ledger differs')
            receipt = inventory_phase.verify_receipt(token)
            left, lrev = self.fence.ledger.read()
            right, rrev = inventory_phase.fence.ledger.read()
            if left != right or lrev != rrev or not self._owned(left):
                raise RecoveryRequired('resume inventory ledger changed')
            local = copy.deepcopy(inventory_phase.journal.state['steps'][full_cohort_inventory_phase.STEP])
            marker = {k: v for k, v in local.items() if k != 'receipt'}
            if local.get('receipt') != receipt or left.get('full_cohort_inventory') != marker:
                raise RecoveryRequired('resume inventory marker changed')
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': self.invocation,
                       'inventory_sha256': digest(receipt)}
            record = self._reserve('writer-cut-resume-inventory', details)
            self._complete('writer-cut-resume-inventory', record, details)
            self.resume_inventory = {'phase': inventory_phase, 'token': token, 'receipt_sha256': digest(receipt),
                                     'local': local, 'marker': marker}
            observed = self._resumed_sessions()
            if observed['attributed_sessions']:
                raise RecoveryRequired('sessions precede writer resume')
            return copy.deepcopy(details)
        except BaseException:
            self._uncertain()

    def _has_resumed_effect(self):
        return any(k.startswith('writer-resume-') and v.get('status') == 'done'
                   for k, v in self.journal.state['steps'].items() if isinstance(v, dict))

    def _resume_inventory_matches(self, row):
        if self.resume_inventory is None:
            return self.migration is None and not self.migration_history
        import full_cohort_inventory_phase
        binding = self.resume_inventory
        phase = binding['phase']
        return (not phase.journal.poisoned and phase.journal.dirfd is not None and phase.journal.lockfd is not None and
                phase.journal.state['steps'].get(full_cohort_inventory_phase.STEP) == binding['local'] and
                row.get('full_cohort_inventory') == binding['marker'] and
                digest(binding['local']['receipt']) == binding['receipt_sha256'])

    def _resumed_sessions(self):
        import writer_cut_pg_probe
        bound = self.resume_inventory
        if bound is None: raise RecoveryRequired('resume inventory not bound')
        phase = bound['phase']
        def same_ledger():
            left, lrev = self.fence.ledger.read()
            right, rrev = phase.fence.ledger.read()
            if left != right or lrev != rrev or not self._owned(left):
                raise RecoveryRequired('resume inventory ledger changed')
        same_ledger()
        receipt = phase.verify_receipt(bound['token'])
        if digest(receipt) != bound['receipt_sha256']:
            raise RecoveryRequired('resume inventory receipt changed')
        observed = writer_cut_pg_probe.inspect_resumed(self, phase, bound['token'])
        if (not isinstance(observed, dict) or set(observed) !=
                {'scope', 'active_sessions', 'active_transactions', 'attributed_sessions'} or
                observed['scope'] != self.current_effective_session_scope() or
                type(observed['active_sessions']) is not int or observed['active_sessions'] != 0 or
                type(observed['active_transactions']) is not int or observed['active_transactions'] != 0 or
                not isinstance(observed['attributed_sessions'], list)):
            raise RecoveryRequired('unattributed resumed sessions remain')
        if digest(phase.verify_receipt(bound['token'])) != bound['receipt_sha256']:
            raise RecoveryRequired('resume inventory changed during observation')
        same_ledger()
        return observed

    def _migration_resume_guard(self):
        """Check terminal migration without assuming already-resumed writers are zero."""
        import writer_cut_migration as migration
        if self.migration is None:
            if self.migration_history or self.provision_successor is not None:
                if self._has_resumed_effect():
                    self._resumed_sessions()
                else:
                    scope = (self.current_provisioned_session_scope() if self.provision_successor is not None
                             else self.current_base_session_scope())
                    receipt = self._session_observation()
                    if (receipt != {'scope': scope, 'active_sessions': 0, 'active_transactions': 0} or
                            type(receipt['active_sessions']) is not int or type(receipt['active_transactions']) is not int):
                        raise RecoveryRequired('writer sessions remain')
            return
        if not self.migration['finished']:
            raise RecoveryRequired('migration is not terminal')
        state, pod = migration.observe(self, self.migration, allow_removed=True)
        if self._has_resumed_effect():
            self._resumed_sessions()
        else:
            receipt = migration.sessions(self, state, pod)
            if receipt['attributed_sessions']:
                raise RecoveryRequired('terminal migration sessions remain')
        after, next_pod = migration.observe(self, self.migration, allow_removed=True)
        if state != after or pod != next_pod:
            raise RecoveryRequired('migration changed before writer resume')

    def verify_migration_pending(self):
        """Recheck the source-owned pending Job and zero sessions before POST."""
        import writer_cut_migration as migration
        try:
            if self.migration is None or self.migration['finished']:
                raise RecoveryRequired('pending migration unavailable')
            self._guard()
            before, pod = migration.observe(self, self.migration)
            if before['job']['state'] != 'pending' or before['job']['uid'] is not None or pod is not None:
                raise RecoveryRequired('pending migration Job is not absent')
            receipt = self.verify_migration_running()
            if (not isinstance(receipt, dict) or set(receipt) !=
                    {'scope', 'active_sessions', 'active_transactions', 'attributed_sessions'} or
                    receipt['active_sessions'] != 0 or receipt['active_transactions'] != 0 or
                    type(receipt['active_sessions']) is not int or type(receipt['active_transactions']) is not int or
                    receipt['attributed_sessions'] != []):
                raise RecoveryRequired('pending migration sessions remain')
            after, pod = migration.observe(self, self.migration)
            self._guard()
            if after != before or pod is not None:
                raise RecoveryRequired('pending migration changed')
            return copy.deepcopy(receipt)
        except BaseException:
            self._uncertain()

    def finish_migration(self):
        import writer_cut_migration as migration
        try:
            binding = self.migration
            if binding is None or binding['finished']: raise RecoveryRequired('migration not active')
            receipt = self.verify_migration_running()
            if receipt['attributed_sessions']: raise RecoveryRequired('migration sessions remain')
            module, state = migration.normalized(self, binding['kind'], binding['phase'], binding['root'])
            if not state['terminal'] or state['job']['state'] not in ('owned', 'done'):
                raise RecoveryRequired('migration terminal receipt absent')
            if binding['kind'] in ('auth', 'go'):
                terminal = module.verify_writer_terminal(binding['phase'], binding['root'])
            else:
                terminal = binding['phase'].journal.state['steps']['job']['receipt']
            _, pod = migration.observe(self, binding)
            if not pod or terminal.get('pod_uid') != pod['uid'] or terminal.get('job_uid') != state['job']['uid']:
                raise RecoveryRequired('terminal Pod or Job changed')
            step = 'writer-cut-migration-finish-' + state['stable']['nonce']
            details = {'plan_sha256': digest(binding['plan']), 'job_uid': state['job']['uid'],
                       'terminal_sha256': state['terminal']}
            record = self._reserve(step, details)
            self._complete(step, record, {'operation_id': self.fence.context['operation_id'], **details})
            binding['terminal'] = state['terminal']
            binding['finished'] = True
            self.verify_migration_running()
            return copy.deepcopy(details)
        except BaseException:
            self._uncertain()

    def accept_migration_cleanup(self):
        import writer_cut_migration as migration
        try:
            binding = self.migration
            if binding is None or not binding['finished'] or binding['kind'] not in ('auth', 'go'):
                raise RecoveryRequired('migration cleanup unavailable')
            module, state = migration.normalized(self, binding['kind'], binding['phase'], binding['root'])
            module.verify_writer_cleanup(binding['phase'], binding['root'])
            receipt = self.verify_migration_running()
            if receipt['attributed_sessions'] or state['job']['state'] != 'removed' or state['configmap']['state'] != 'removed':
                raise RecoveryRequired('migration cleanup incomplete')
            step = 'writer-cut-migration-cleanup-' + state['stable']['nonce']
            details = {'plan_sha256': digest(binding['plan']), 'job_uid': binding['job_uid'],
                       'config_uid': binding['config_uid'], 'terminal_sha256': state['terminal']}
            record = self._reserve(step, details)
            self._complete(step, record, {'operation_id': self.fence.context['operation_id'], **details})
            self.migration_history.append(binding)
            self.migration = None
            self.verify_drained()
            return copy.deepcopy(details)
        except BaseException:
            self._uncertain()

    def _session_observation(self):
        if self.provision_successor is None:
            if self.space_fresh is not None:
                import full_cohort_affected_plan
                return full_cohort_affected_plan.inspect_space_completed(self.probe,self)
            return self.probe(copy.deepcopy(self.admission['session_scope']))
        import full_cohort_affected_plan
        return full_cohort_affected_plan.inspect_provisioned(self.probe, self)

    def verify_drained(self):
        if not self.drained: raise RecoveryRequired('writer drain not completed')
        try:
            if self.migration is not None:
                if not self.migration['finished']: raise RecoveryRequired('migration is not terminal')
                self.verify_migration_running()
            self._guard()
            self._guards()
            self._recheck_config_successors()
            if not self._retired(): raise RecoveryRequired('writer controllers or pods not retired')
            scope = (self.current_provisioned_session_scope() if self.provision_successor is not None
                     else self.current_base_session_scope())
            receipt = self._session_observation()
            if (not isinstance(receipt, dict) or set(receipt) != {'scope', 'active_sessions', 'active_transactions'} or
                    receipt['scope'] != scope or
                    type(receipt['active_sessions']) is not int or receipt['active_sessions'] != 0 or
                    type(receipt['active_transactions']) is not int or receipt['active_transactions'] != 0):
                raise RecoveryRequired('writer sessions remain or observation invalid')
            self._guard()
            self._guards()
            self._recheck_config_successors()
            if not self._retired(): raise RecoveryRequired('writer changed during observation')
            return copy.deepcopy(receipt)
        except BaseException:
            self._uncertain()

    def current_cut_proof(self):
        """Return private current specs plus journal-bound proof for image admission."""
        try:
            if self.migration is not None and not self.migration['finished']:
                raise RecoveryRequired('migration is not terminal')
            if (self.config_snapshot is not None and
                    self.config_snapshot['services'] != self.config_snapshot['intent']['backend_config_deployments']['services']):
                raise RecoveryRequired('config successor plan is incomplete')
            session = self.verify_drained()
            writers = []
            for original in self.admission['controllers']:
                name, kind = original['metadata']['name'], original['kind']
                drain_step = 'writer-drain-' + kind.lower() + '-' + name
                drain = self.journal.state['steps'][drain_step]
                successor = self.successors.get(name) if kind == 'Deployment' else None
                if successor:
                    service = name.removeprefix('voice-')
                    step = 'writer-cut-config-successor-' + service
                    accepted = self.journal.state['steps'][step]
                    if accepted.get('status') != 'done':
                        raise RecoveryRequired('config successor not complete')
                    spec = successor['target_spec']
                    receipt = successor['receipt']
                    successor_ref = {'step': step, 'nonce': accepted['nonce'],
                                     'receipt_sha256': digest(accepted['receipt'])}
                else:
                    spec = drain['details']['target_spec']
                    receipt = drain['receipt']
                    successor_ref = None
                writers.append({'kind': kind, 'name': name, 'uid': receipt['uid'],
                                'generation': receipt['generation'], 'spec': copy.deepcopy(spec),
                                'drain_step': drain_step, 'drain_nonce': drain['nonce'],
                                'drain_receipt_sha256': digest(drain['receipt']), 'successor': successor_ref})
            self._guard()
            self._recheck_config_successors()
            return {'operation_id': self.fence.context['operation_id'], 'namespace_uid': self.namespace_uid,
                    'invocation_nonce': self.invocation, 'writers': writers, 'session': session}
        except BaseException:
            self._uncertain()

    def accept_config_successor(self, config_journal, service):
        """Bind the next completed fixed producer effect to the original cut."""
        try:
            if not self.drained or self.config_journal is not config_journal or self.config_snapshot is None:
                raise RecoveryRequired('bound config plan required')
            self._guard()
            self._guards()
            self._recheck_config_successors()
            if (type(config_journal) is not media_operation_journal.Journal or config_journal is self.journal or
                    config_journal.poisoned or config_journal.dirfd is None or config_journal.lockfd is None or
                    config_journal.intent != config_journal.state.get('intent')):
                raise RecoveryRequired('open separate config journal required')
            intent = config_journal.intent
            plan = intent.get('backend_config_deployments')
            if (intent.get('resource_context') != self.fence.context or intent.get('namespace_uid') != self.namespace_uid or
                    not isinstance(plan, dict) or plan.get('source_sha') != self.fence.context['commit'] or
                    plan.get('cut_invocation') != self.invocation or not isinstance(plan.get('services'), list) or
                    not plan['services'] or len(plan['services']) != len(set(plan['services'])) or
                    set(plan['services']) - {'user', 'notification', 'realtime', 'chat', 'space'}):
                raise RecoveryRequired('config source or cut identity changed')
            selected = {o['metadata']['name']: o for o in self.admission['controllers'] if o['kind'] == 'Deployment'}
            if any('voice-' + s not in selected for s in plan['services']):
                raise RecoveryRequired('config plan includes nonwriter')
            previous = [] if self.config_snapshot is None else self.config_snapshot['services']
            if len(previous) >= len(plan['services']) or service != plan['services'][len(previous)]:
                raise RecoveryRequired('config successor is skipped or replayed')
            services = previous + [service]
            row, _ = self.fence.ledger.read()
            if not self._owned(row) or not isinstance(row.get('backend_config_effects'), dict):
                raise RecoveryRequired('config ledger unavailable')
            config_effects = copy.deepcopy(row['backend_config_effects'])
            expected_steps = {'backend-config-deployment-voice-' + s for s in services}
            if ({name for name in config_effects if name.startswith('backend-config-deployment-')} != expected_steps or
                    {name for name in config_journal.state['steps'] if name.startswith('backend-config-deployment-')} != expected_steps or
                    any(config_effects[name].get('status') != 'done' or
                        config_journal.state['steps'][name].get('status') != 'done' for name in expected_steps)):
                raise RecoveryRequired('config deployment set incomplete')
            snapshot = {'intent': copy.deepcopy(intent),
                        'steps': {k: copy.deepcopy(config_journal.state['steps'][k]) for k in expected_steps},
                        'effects': {k: copy.deepcopy(config_effects[k]) for k in expected_steps}, 'services': services}
            successors = self._validate_config_snapshot(config_journal, snapshot, selected)
            self._guard()
            step_name = 'writer-cut-config-successor-' + service
            record = self._reserve(step_name, snapshot)
            self.config_journal, self.config_snapshot, self.successors = config_journal, snapshot, successors
            probe = self.verify_drained()
            output = {'invocation': {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']},
                      'successor': {'service': service, 'uid': successors['voice-' + service]['receipt']['uid'],
                          'generation': successors['voice-' + service]['receipt']['generation'],
                          'spec_sha256': successors['voice-' + service]['receipt']['spec_sha256'],
                          'config_step': 'backend-config-deployment-voice-' + service,
                          'nonce': successors['voice-' + service]['ledger_record']['nonce'],
                          'receipt_sha256': successors['voice-' + service]['ledger_record']['receipt_sha256']},
                      'probe': probe}
            self._complete(step_name, record, output)
            return copy.deepcopy(output)
        except BaseException:
            self._uncertain()

    def drain(self):
        try:
            self._guard()
            self._start()
            self._guards()
            deployment_ids = {o['metadata']['uid'] for o in self.admission['controllers'] if o['kind'] == 'Deployment'}
            replicasets = self.client.call('get', 'replicasets', '-o', 'json')['items']
            self.retired_rs = {r['metadata']['uid'] for r in replicasets if self._owner(r['metadata'], 'Deployment') in deployment_ids}
            self.journal.state['steps']['writer-cut-old-replicasets'] = {'uids': sorted(self.retired_rs)}
            self.journal.save()
            for source in self.admission['controllers']:
                current = self._get(source['kind'], source['metadata']['name'])
                self._workload(current)
                if (current.get('metadata', {}).get('uid') != source['metadata']['uid'] or
                        current.get('metadata', {}).get('resourceVersion') != source['metadata']['resourceVersion'] or
                        current.get('spec') != source['spec']):
                    raise RecoveryRequired('writer source changed')
            for source in self.admission['controllers']:
                self._effect('drain', source, 0)
            self.drained = True
            deadline = time.monotonic() + 180
            while not self._retired():
                self._guard()
                self._guards()
                if time.monotonic() >= deadline: raise RecoveryRequired('writer pods not retired')
                time.sleep(2)
            record = self._reserve('writer-cut-drained', {'session_scope': self.admission['session_scope']})
            receipt = self.verify_drained()
            self._complete('writer-cut-drained', record, receipt)
            return receipt
        except BaseException:
            self._uncertain()

    def _health_matches(self, row, health):
        phase = health['phase']
        return (not phase.journal.poisoned and phase.journal.dirfd is not None and phase.journal.lockfd is not None
                and phase.journal.state['steps'].get(health['step']) == health['local']
                and row.get(health['ledger']) == health['marker']
                and digest(health['local']['receipt']) == health['receipt_sha256'] and
                ('cut_step' not in health or (self.journal.state['steps'].get(health['cut_step']) == health['cut_local']
                 and row.get('writer_cut_effects', {}).get(health['cut_step']) == health['cut_effect'])))

    def _activation_matches(self, row):
        if self.activation is None:
            return not any(o['metadata']['name'] in ('voice-space', 'voice-chat', 'voice-realtime')
                           for o in self.admission['controllers'])
        binding = self.activation
        phase = binding['phase']
        return (not phase.journal.poisoned and phase.journal.dirfd is not None and phase.journal.lockfd is not None
                and phase.journal.state['steps'].get('full-cohort-activation-plan') == binding['local']
                and row.get('full_cohort_activation_plan') == binding['marker']
                and digest(binding['local']['receipt']) == binding['plan_sha256']
                and self.journal.state['steps'].get('writer-cut-activation-plan') == binding['cut_local']
                and row.get('writer_cut_effects', {}).get('writer-cut-activation-plan') == binding['cut_effect']
                and all(self._health_matches(row, health) for health in binding['health'].values()))

    def _activation_stage(self):
        binding = self.activation
        ordered = binding['plan']['ordered']
        names = [x['name'] for x in ordered]
        allowed = {'writer-resume-' + item['kind'].lower() + '-' + item['name'] for item in ordered}
        if any(k.startswith('writer-resume-') and k not in allowed for k in self.journal.state['steps']):
            raise RecoveryRequired('activation includes an unselected writer')
        done = [i for i, item in enumerate(ordered) if self.journal.state['steps'].get(
            'writer-resume-' + item['kind'].lower() + '-' + item['name'], {}).get('status') == 'done']
        if done != list(range(len(done))): raise RecoveryRequired('activation prefix changed')
        index = len(done)
        pending = [k for k, v in self.journal.state['steps'].items() if k.startswith('writer-resume-')
                   and isinstance(v, dict) and v.get('status') == 'pending']
        expected_pending = ('writer-resume-' + ordered[index]['kind'].lower() + '-' + ordered[index]['name']) if index < len(ordered) else None
        if len(pending) > 1 or (pending and pending[0] != expected_pending):
            raise RecoveryRequired('activation pending effect changed')
        health = binding['health']
        if 'voice-space' in names and names.index('voice-space') < index and 'space_health' not in health:
            return 'space_health'
        if 'voice-chat' in names and names.index('voice-chat') < index and 'chat_health' not in health:
            return 'chat_health'
        if index == len(ordered): return 'done'
        name = names[index]
        if name == 'voice-chat' and 'publisher_health' not in health: return 'publisher_health'
        if name == 'voice-realtime':
            for barrier in ('space_health', 'publisher_health', 'chat_health'):
                if barrier not in health: return barrier
        return {'voice-space': 'space', 'voice-chat': 'chat', 'voice-realtime': 'realtime'}.get(name, 'remaining')

    def verify_activation_stage(self):
        if self.activation is None: raise RecoveryRequired('activation plan unavailable')
        self._guard()
        row, _ = self.fence.ledger.read()
        if not self._activation_matches(row): raise RecoveryRequired('activation plan changed')
        stage = self._activation_stage()
        if self.activation['stage'] != stage: raise RecoveryRequired('activation stage changed')
        self._guard()
        row, _ = self.fence.ledger.read()
        if not self._activation_matches(row) or self._activation_stage() != stage:
            raise RecoveryRequired('activation changed during stage check')
        return {'operation_id': self.fence.context['operation_id'], 'cut_invocation': self.invocation,
                'plan_sha256': self.activation['plan_sha256'], 'stage': stage}

    def prepare_activation(self, phase, token):
        """Bind the fixed source-owned post-image plan before any writer CAS."""
        import full_cohort_activation_plan
        try:
            if (self.activation is not None or not self.drained or self.resume_inventory is None or
                    (self.provision_successor is not None and self.effective_session is None) or
                    type(phase) is not full_cohort_activation_plan.ActivationPhase or
                    any(k.startswith('writer-resume-') for k in self.journal.state['steps']) or
                    phase.fence.context != self.fence.context or phase.lock != self.lock or
                    phase.namespace_uid != self.namespace_uid or phase.fence.ledger.client is not phase.client):
                raise RecoveryRequired('activation phase identity invalid')
            self._guard(); self._guards(); self._migration_resume_guard()
            if self.migration is None and self.migration_history:
                scope = (self.current_provisioned_session_scope() if self.provision_successor is not None
                         else self.current_base_session_scope())
                receipt = self._session_observation()
                if (receipt != {'scope': scope, 'active_sessions': 0, 'active_transactions': 0} or
                        type(receipt['active_sessions']) is not int or type(receipt['active_transactions']) is not int):
                    raise RecoveryRequired('writer sessions remain')
            plan = full_cohort_activation_plan.verify_completed(phase, self, token)
            if (not isinstance(plan, dict) or set(plan) != {'operation_id', 'namespace_uid', 'source_sha',
                    'cut_invocation', 'package_sha256', 'ordered', 'barriers'} or
                    plan['operation_id'] != self.fence.context['operation_id'] or plan['namespace_uid'] != self.namespace_uid or
                    plan['source_sha'] != self.fence.context['commit'] or plan['cut_invocation'] != self.invocation or
                    plan['barriers'] != ['space_health', 'publisher_health', 'chat_health'] or
                    not isinstance(plan['ordered'], list)):
                raise RecoveryRequired('activation source plan invalid')
            selected = {(x['kind'], x['metadata']['name']): x for x in self.admission['controllers']}
            keys = [(x.get('kind'), x.get('name')) for x in plan['ordered']]
            if len(keys) != len(set(keys)) or set(keys) != set(selected):
                raise RecoveryRequired('activation selected set changed')
            names = [x[1] for x in keys]
            if ('voice-realtime' in names and any(x in names and names.index(x) > names.index('voice-realtime')
                                                   for x in ('voice-space', 'voice-chat'))):
                raise RecoveryRequired('activation source order violates prerequisites')
            for item in plan['ordered']:
                if set(item) != {'kind', 'name', 'uid', 'generation', 'spec'}: raise RecoveryRequired('target invalid')
                actual = self._get(item['kind'], item['name']); self._workload(actual)
                meta = actual['metadata']
                if (meta['uid'] != item['uid'] or meta['generation'] != item['generation'] or
                        actual['spec'] != item['spec'] or item['spec']['replicas'] != 0):
                    raise RecoveryRequired('activation target changed')
            if not self._pods_drained(): raise RecoveryRequired('writer Pods remain')
            left, lrev = self.fence.ledger.read(); right, rrev = phase.fence.ledger.read()
            local = copy.deepcopy(phase.journal.state['steps']['full-cohort-activation-plan'])
            marker = {k: v for k, v in local.items() if k != 'receipt'}
            if left != right or lrev != rrev or not self._owned(left) or left.get('full_cohort_activation_plan') != marker or local.get('receipt') != plan:
                raise RecoveryRequired('activation source ledger changed')
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': self.invocation,
                       'plan_sha256': digest(plan)}
            record = self._reserve('writer-cut-activation-plan', details)
            self._complete('writer-cut-activation-plan', record, details)
            row, _ = self.fence.ledger.read()
            self.activation = {'phase': phase, 'token': token, 'plan': copy.deepcopy(plan), 'plan_sha256': digest(plan),
                               'local': local, 'marker': marker, 'health': {}, 'stage': None,
                               'cut_local': copy.deepcopy(self.journal.state['steps']['writer-cut-activation-plan']),
                               'cut_effect': copy.deepcopy(row['writer_cut_effects']['writer-cut-activation-plan'])}
            self.activation['stage'] = self._activation_stage()
            return self.verify_activation_stage()
        except BaseException:
            self._uncertain()

    def _activation_verify_sources(self):
        import full_cohort_activation_plan
        binding = self.activation
        plan = full_cohort_activation_plan.verify_completed(binding['phase'], self, binding['token'])
        if plan != binding['plan']: raise RecoveryRequired('activation source plan changed')
        for name, item in binding['health'].items():
            module = self._activation_health_module(name)
            receipt = module.verify_completed(item['phase'], self, item['token'])
            if digest(receipt) != item['receipt_sha256']:
                raise RecoveryRequired('activation health proof changed')

    @staticmethod
    def _activation_health_module(name):
        if name == 'space_health':
            import realtime_config_transition_space_health as module
        elif name == 'publisher_health':
            import realtime_jwks_publisher_health as module
        elif name == 'chat_health':
            import full_cohort_chat_health as module
        else:
            raise RecoveryRequired('unknown activation health')
        return module

    def accept_activation_health(self, name, phase, token):
        """Accept exactly the next fixed source-owned health barrier."""
        try:
            stage = self.verify_activation_stage()['stage']
            if name != stage or name not in ('space_health', 'publisher_health', 'chat_health'):
                raise RecoveryRequired('activation health is out of order')
            module = self._activation_health_module(name)
            classes = {'space_health': 'SpaceHealthPhase', 'publisher_health': 'PublisherHealthPhase',
                       'chat_health': 'ChatHealthPhase'}
            if (type(phase) is not getattr(module, classes[name]) or phase.fence.context != self.fence.context or
                    phase.lock != self.lock or phase.namespace_uid != self.namespace_uid or
                    phase.fence.ledger.client is not phase.client):
                raise RecoveryRequired('activation health identity differs')
            proof = module.verify_completed(phase, self, token)
            row, revision = self.fence.ledger.read(); other, other_revision = phase.fence.ledger.read()
            local = copy.deepcopy(phase.journal.state['steps'][module.STEP])
            marker = {k: v for k, v in local.items() if k != 'receipt'}
            if (row != other or revision != other_revision or not self._owned(row) or
                    row.get(module.LEDGER) != marker or local.get('receipt') != proof):
                raise RecoveryRequired('activation health ledger differs')
            health = {'phase': phase, 'token': token, 'step': module.STEP, 'ledger': module.LEDGER,
                      'local': local, 'marker': marker, 'receipt_sha256': digest(proof)}
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': self.invocation,
                       'activation_plan_sha256': self.activation['plan_sha256'], 'health_receipt_sha256': digest(proof)}
            step = 'writer-cut-' + name.replace('_', '-')
            record = self._reserve(step, details)
            self.pending_activation_health = health
            self._complete(step, record, details)
            self.pending_activation_health = None
            row, _ = self.fence.ledger.read()
            health.update(cut_step=step, cut_local=copy.deepcopy(self.journal.state['steps'][step]),
                          cut_effect=copy.deepcopy(row['writer_cut_effects'][step]))
            self.activation['health'][name] = health
            self.activation['stage'] = self._activation_stage()
            return self.verify_activation_stage()
        except BaseException:
            self._uncertain()

    def activate_next(self):
        """Restore the next source-ordered writer once its fixed barriers are done."""
        try:
            stage = self.verify_activation_stage()['stage']
            if stage in ('space_health', 'publisher_health', 'chat_health', 'done'):
                raise RecoveryRequired('activation health or completion required')
            self._activation_verify_sources()
            binding = self.activation
            done = len(self.current_resumed_proof()['controllers'])
            item = binding['plan']['ordered'][done]
            selected = {(x['kind'], x['metadata']['name']): x for x in self.admission['controllers']}
            key = (item['kind'], item['name']); actual = self._get(*key); self._workload(actual)
            meta, status = actual['metadata'], actual.get('status', {})
            if (meta['uid'] != item['uid'] or meta['generation'] != item['generation'] or actual['spec'] != item['spec'] or
                    status.get('observedGeneration', 0) < meta['generation'] or status.get('replicas', 0) != 0 or
                    status.get('readyReplicas', 0) != 0 or
                    not self._pods_drained([selected[key], actual], include_retired=False)):
                raise RecoveryRequired('activation target no longer retired')
            self._effect('resume', actual, selected[key]['spec']['replicas'])
            binding['stage'] = self._activation_stage()
            return self.verify_activation_stage()
        except BaseException:
            self._uncertain()

    def finish_activation(self):
        try:
            if self.verify_activation_stage()['stage'] != 'done':
                raise RecoveryRequired('activation remains incomplete')
            self._activation_verify_sources()
            self._guard(); self._guards(); self._recheck_config_successors()
            proof = self.current_resumed_proof()
            if len(proof['controllers']) != len(self.admission['controllers']):
                raise RecoveryRequired('activation controller set incomplete')
            self.drained = False
            return {'status': 'replicas-restored', 'invocation': proof['invocation'],
                    'controllers': copy.deepcopy(proof['controllers'])}
        except BaseException:
            self._uncertain()

    def resume(self, targets):
        try:
            if not self.drained or not isinstance(targets, list) or len(targets) != len(self.admission['controllers']):
                raise RecoveryRequired('current target admission required')
            if any(x['metadata']['name'] in ('voice-space', 'voice-chat', 'voice-realtime') for x in self.admission['controllers']):
                raise RecoveryRequired('source-owned staged activation required')
            if (self.migration is not None or self.migration_history or self.provision_successor is not None) and self.resume_inventory is None:
                raise RecoveryRequired('qualified resume inventory required')
            if self.provision_successor is not None and self.effective_session is None:
                raise RecoveryRequired('Realtime migration scope is not completed')
            if self.migration is None and self.migration_history:
                scope = (self.current_provisioned_session_scope() if self.provision_successor is not None
                         else self.current_base_session_scope())
                receipt = self._session_observation()
                if (receipt != {'scope': scope, 'active_sessions': 0, 'active_transactions': 0} or
                        type(receipt['active_sessions']) is not int or type(receipt['active_transactions']) is not int):
                    raise RecoveryRequired('writer sessions remain')
            if self.migration is not None:
                if not self.migration['finished']: raise RecoveryRequired('migration is not terminal')
                self._migration_resume_guard()
            self._guard()
            self._guards()
            self._recheck_config_successors()
            if (self.config_snapshot is not None and
                    self.config_snapshot['services'] != self.config_snapshot['intent']['backend_config_deployments']['services']):
                raise RecoveryRequired('config successor plan is incomplete')
            original = {(x['kind'], x['metadata']['name']): x for x in self.admission['controllers']}
            seen, fresh = set(), []
            for target in targets:
                self._workload(target)
                key = (target['kind'], target['metadata']['name'])
                if key not in original or key in seen or target['spec']['replicas'] != 0:
                    raise RecoveryRequired('target scope or paused replica count invalid')
                seen.add(key)
                current = self._get(*key)
                self._workload(current)
                if (current.get('kind') != target['kind'] or current.get('metadata', {}).get('name') != key[1] or
                        current.get('metadata', {}).get('namespace') != 'voice-dev' or
                        current.get('metadata', {}).get('uid') != target['metadata']['uid'] or
                        current.get('metadata', {}).get('generation') != target['metadata']['generation'] or
                        not current.get('metadata', {}).get('resourceVersion') or
                        current.get('spec') != target['spec']):
                    raise RecoveryRequired('current target changed')
                generation = current.get('metadata', {}).get('generation')
                status = current.get('status', {})
                if (type(generation) is not int or type(status.get('observedGeneration')) is not int or
                        status['observedGeneration'] < generation or status.get('replicas', 0) != 0 or
                        status.get('readyReplicas', 0) != 0):
                    raise RecoveryRequired('current target not retired')
                fresh.append(current)
            if not self._pods_drained() or not self._pods_drained(targets):
                raise RecoveryRequired('current or predecessor writer pods remain')
            effects = {}
            for target in fresh:
                key = (target['kind'], target['metadata']['name'])
                receipt = self._effect('resume', target, original[key]['spec']['replicas'])
                step = 'writer-resume-' + target['kind'].lower() + '-' + target['metadata']['name']
                saved = self.journal.state['steps'][step]
                effects[step] = {'kind': key[0], 'name': key[1], 'replicas': original[key]['spec']['replicas'],
                                 'nonce': saved['nonce'], 'receipt_sha256': digest(receipt), **receipt}
            self._guard()
            self._recheck_config_successors()
            self.drained = False
            return {'status': 'replicas-restored', 'count': len(seen), 'effects': effects,
                    'invocation': {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}}
        except BaseException:
            self._uncertain()
