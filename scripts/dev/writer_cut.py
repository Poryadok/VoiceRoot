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
        self.probe = probe
        self.invocation = secrets.token_hex(16)
        self.started = self.drained = self.poisoned = False
        self.retired_rs = set()
        self.successors = {}
        self.config_journal = None
        self.config_snapshot = None
        self.migration = None
        self.migration_history = []
        self._identity()

    def _identity(self):
        a = self.admission
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or self.intent.get('resource_context') != self.fence.context or
                self.intent.get('namespace_uid') != self.namespace_uid or self.intent.get('writer_cut') != a or
                type(self.client) is not frontend_runtime_executor.KubectlTransport or
                self.client.journal is not self.journal or getattr(self.fence.ledger, 'client', None) is not self.client or
                not callable(self.probe) or not isinstance(a, dict) or set(a) != {'controllers', 'guards', 'linked', 'session_scope'} or
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
                    name.startswith('writer-cut-config-successor-') or name.startswith('writer-cut-migration-')):
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
                if self.migration['finished']:
                    receipt = writer_cut_migration.sessions(self, state, pod)
                    if receipt['attributed_sessions']: raise RecoveryRequired('terminal migration sessions remain')
                uid = state['job']['uid'] if state['job']['state'] in ('owned', 'done', 'removing') else None
                actual = [item for item in actual if item['metadata']['uid'] != uid]
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
                _, state = writer_cut_migration.normalized(self, binding['kind'], binding['phase'], binding['root'])
                receipt = writer_cut_migration.sessions(self, state, None)
                if receipt['attributed_sessions']: raise RecoveryRequired('cleaned migration sessions remain')

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
        self._guard()
        row, _ = self.fence.ledger.read()
        if (not self._owned(row) or row.get('writer_cut_effects', {}) != self._effect_map() or
                not self._config_effects_match(row) or
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

    def _pods_drained(self, controllers=None):
        controllers = self.admission['controllers'] if controllers is None else controllers
        deployment_ids = {o['metadata']['uid'] for o in controllers if o['kind'] == 'Deployment'}
        stateful_ids = {o['metadata']['uid'] for o in controllers if o['kind'] == 'StatefulSet'}
        rs = self.client.call('get', 'replicasets', '-o', 'json')['items']
        all_rs = {r['metadata']['uid'] for r in rs}
        rs_ids = self.retired_rs | {r['metadata']['uid'] for r in rs if self._owner(r['metadata'], 'Deployment') in deployment_ids}
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
            elif service in ('chat', 'realtime'):
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
                    set(plan['services']) - {'user', 'notification', 'realtime', 'chat'}):
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
            state, pod = migration.observe(self, binding)
            receipt = migration.sessions(self, state, pod)
            if receipt['attributed_sessions']:
                raise RecoveryRequired('migration sessions precede Job')
            step = 'writer-cut-migration-plan-' + state['stable']['nonce']
            record = self._reserve(step, {'kind': kind, 'plan': plan})
            self.migration = binding
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
            receipt = migration.sessions(self, state, pod)
            if self.migration['finished'] and receipt['attributed_sessions']:
                raise RecoveryRequired('terminal migration sessions remain')
            self._guard()
            self._guards()
            self._recheck_config_successors()
            after, next_pod = migration.observe(self, self.migration, allow_removed=self.migration['finished'])
            if state != after or pod != next_pod or not self._retired():
                raise RecoveryRequired('migration changed during session observation')
            return receipt
        except BaseException:
            self._uncertain()

    def _migration_resume_guard(self):
        """Check terminal migration without assuming already-resumed writers are zero."""
        import writer_cut_migration as migration
        if self.migration is None:
            return
        if not self.migration['finished']:
            raise RecoveryRequired('migration is not terminal')
        state, pod = migration.observe(self, self.migration, allow_removed=True)
        receipt = migration.sessions(self, state, pod)
        if receipt['attributed_sessions']:
            raise RecoveryRequired('terminal migration sessions remain')
        after, next_pod = migration.observe(self, self.migration, allow_removed=True)
        if state != after or pod != next_pod:
            raise RecoveryRequired('migration changed before writer resume')

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
            if binding['kind'] == 'auth':
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
            if binding is None or not binding['finished'] or binding['kind'] != 'auth':
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
            scope = copy.deepcopy(self.admission['session_scope'])
            receipt = self.probe(scope)
            if (not isinstance(receipt, dict) or set(receipt) != {'scope', 'active_sessions', 'active_transactions'} or
                    receipt['scope'] != self.admission['session_scope'] or
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
                    set(plan['services']) - {'user', 'notification', 'realtime', 'chat'}):
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

    def resume(self, targets):
        try:
            if not self.drained or not isinstance(targets, list) or len(targets) != len(self.admission['controllers']):
                raise RecoveryRequired('current target admission required')
            if self.migration is not None:
                self.verify_migration_running()
                if not self.migration['finished']: raise RecoveryRequired('migration is not terminal')
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
