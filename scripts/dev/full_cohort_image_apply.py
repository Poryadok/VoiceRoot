"""One-shot fenced DEV image application; caller owns admission and resume.

Construct a separate private image Journal after WriterCut.drain using actual
CUT-state observations. Intent `full_cohort` has `images`, `platform_digests`,
`workloads` (UID, resourceVersion, generation, full spec, resume_replicas),
`cut`, `service_accounts` and `cut_proof`. Call `prepare_cut(actual_cut)` once
before creating the image Journal; copy the complete private `admission.proof`
into its intent, and pass the admission handle to FullCohortImageApply. Supply
separate fixed per-journal clients and fences on the same physical ledger/lock.
The exact 23 refs and platform digests must already be authenticated by the
central operator. `apply()` returns private post-patch UID/RV/generation/spec;
the caller restores replicas through WriterCut using those current targets,
then calls `verify_after_resume()`. It verifies only Kubernetes workload
readiness and writer receipts. Business smoke remains the caller's obligation.
An uncertain outcome retains the lock; never replay an invocation.
Only successors already validated by the source-owned WriterCut are admitted;
the backend producer remains responsible for its own admission and evidence.
"""
import copy
import hashlib
import json
from pathlib import Path
import re
import secrets

from frontend_runtime_executor import KubectlTransport
import media_operation_guard
import update_images
from writer_cut import WriterCut


class RecoveryRequired(RuntimeError):
    pass


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


CATALOG = {'voice-' + name: container for name, container in update_images.SERVICES.items()}


class CutAdmission:
    """Private in-process capture; not reconstructible from caller-supplied hashes."""
    def __init__(self, cut):
        if type(cut) is not WriterCut:
            raise RecoveryRequired('actual writer cut required')
        self.cut = cut
        self.proof = copy.deepcopy(cut.current_cut_proof())
        cut._guard()
        cut._recheck_config_successors()
        row, revision = cut.fence.ledger.read()
        if not cut.fence.owned(row) or row.get('status') != 'active' or not isinstance(revision, tuple) or len(revision) != 2:
            raise RecoveryRequired('writer ledger unavailable')
        self.ledger_uid = revision[0]
        self.cut_intent = copy.deepcopy(cut.journal.intent)
        self.cut_object_intent = copy.deepcopy(cut.intent)
        self.cut_object_admission = copy.deepcopy(cut.admission)
        self.cut_invocation = cut.invocation
        self.retired_rs = copy.deepcopy(cut.retired_rs)
        self.probe = cut.probe
        self.cut_steps = copy.deepcopy(cut.journal.state['steps'])
        self.writer_effects = copy.deepcopy(row.get('writer_cut_effects', {}))
        self.writer_invocation = copy.deepcopy(row.get('writer_cut_invocation'))
        self.config_journal = cut.config_journal
        self.config_snapshot = copy.deepcopy(cut.config_snapshot)
        self.successors = copy.deepcopy(cut.successors)
        self.config_intent = copy.deepcopy(cut.config_journal.intent) if cut.config_journal is not None else None
        self.config_steps = copy.deepcopy(cut.config_journal.state['steps']) if cut.config_journal is not None else None
        self.config_effects = copy.deepcopy(row.get('backend_config_effects', {}))
        cut._guard()
        cut._recheck_config_successors()
        final, final_revision = cut.fence.ledger.read()
        if final != row or final_revision != revision:
            raise RecoveryRequired('writer proof changed during capture')


def prepare_cut(cut):
    return CutAdmission(cut)


def admission_changed(admission, snapshot, cut, config_journal):
    return (admission.cut is not cut or admission.config_journal is not config_journal
            or any(getattr(admission, key, None) != value for key, value in snapshot.items()))


class FullCohortImageApply:
    def __init__(self, journal, fence, client, lock, namespace_uid, admission=None):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.intent = copy.deepcopy(journal.intent)
        if type(admission) is not CutAdmission:
            raise RecoveryRequired('source-owned cut admission required')
        self.admission = admission
        self.capture_values = copy.deepcopy({k: getattr(admission, k) for k in (
            'proof', 'ledger_uid', 'cut_intent', 'cut_steps', 'writer_effects', 'writer_invocation',
            'cut_object_intent', 'cut_object_admission', 'cut_invocation', 'retired_rs',
            'config_snapshot', 'successors', 'config_intent', 'config_steps', 'config_effects')})
        self.capture_cut = admission.cut
        self.capture_config_journal = admission.config_journal
        self.capture_probe = admission.probe
        self.cut_journal = admission.cut.journal
        self.cut_intent = copy.deepcopy(admission.cut_intent)
        self.invocation = secrets.token_hex(16)
        self.started = self.poisoned = self.verifying = self.activation_checking = False
        if (type(client) is not KubectlTransport or client.journal is not journal
                or getattr(fence.ledger, 'client', None) is not client):
            raise RecoveryRequired('single fixed Kubernetes client required')
        self._identity()

    def _identity(self):
        try:
            plan = self.intent['full_cohort']
            images, platforms, workloads, cut, accounts = (plan[k] for k in ('images', 'platform_digests', 'workloads', 'cut', 'service_accounts'))
            keys = {name + '/' + container for name, container in CATALOG.items()}
            catalog = json.loads((Path(__file__).resolve().parents[1] / 'ci' / 'staging-image-catalog.json').read_text(encoding='utf-8'))
            canonical = {entry['deployment']: update_images.SERVICES[entry['name']] for entry in catalog['images']}
            if (len(CATALOG) != 23 or self.poisoned or self.journal.poisoned
                    or type(self.client) is not KubectlTransport or self.client.journal is not self.journal
                    or getattr(self.fence.ledger, 'client', None) is not self.client
                    or self.admission.probe is not self.capture_probe
                    or admission_changed(self.admission, self.capture_values, self.capture_cut, self.capture_config_journal)
                    or catalog.get('version') != 1 or len(catalog['images']) != 23 or canonical != CATALOG
                    or self.journal.intent != self.intent or self.journal.state['intent'] != self.intent
                    or (self.started and self.journal.state['steps'].get('full-cohort-invocation') !=
                        {'status': 'owned', 'nonce': self.invocation})
                    or self.intent['resource_context'] != self.fence.context
                    or self.intent['namespace_uid'] != self.namespace_uid
                    or set(plan) != {'images', 'platform_digests', 'workloads', 'cut', 'service_accounts', 'cut_proof'}
                    or plan['cut_proof'] != self.admission.proof
                    or set(images) != keys or images != self.fence.context['images']
                    or set(platforms) != keys or set(workloads) != set(CATALOG)
                    or not isinstance(cut, list) or len(cut) != len(set(cut)) or not set(cut) <= set(CATALOG)):
                raise RecoveryRequired('cohort identity invalid')
            for name, container in CATALOG.items():
                key = name + '/' + container
                ref, platform, snap = images[key], platforms[key], workloads[name]
                if (not isinstance(ref, str) or not re.fullmatch(r'[a-z0-9][a-z0-9./:_-]*@sha256:[a-f0-9]{64}', ref)
                        or not isinstance(platform, str) or not re.fullmatch(r'sha256:[a-f0-9]{64}', platform)
                        or set(snap) != {'uid', 'resourceVersion', 'generation', 'spec', 'resume_replicas'}
                        or any(not isinstance(snap[k], str) or not snap[k] for k in ('uid', 'resourceVersion'))
                        or type(snap['generation']) is not int or snap['generation'] < 1
                        or type(snap['resume_replicas']) is not int or snap['resume_replicas'] < 1):
                    raise RecoveryRequired('cohort plan invalid')
                spec = snap['spec']
                replicas = spec.get('replicas', 1)
                if (type(replicas) is not int or (name in cut and replicas != 0)
                        or (name not in cut and replicas != snap['resume_replicas'])):
                    raise RecoveryRequired('cut or resume replicas invalid')
                containers = spec['template']['spec']['containers']
                if sum(c['name'] == container for c in containers) != 1:
                    raise RecoveryRequired('application container invalid')
            account_names = {snap['spec']['template']['spec'].get('serviceAccountName', 'default') for snap in workloads.values()}
            if set(accounts) != account_names:
                raise RecoveryRequired('service account inventory invalid')
            for name, account in accounts.items():
                meta = account['metadata']
                if (account.get('kind') != 'ServiceAccount' or meta.get('name') != name
                        or meta.get('namespace') != 'voice-dev' or not meta.get('uid')
                        or not meta.get('resourceVersion') or meta.get('deletionTimestamp')):
                    raise RecoveryRequired('service account identity invalid')
            for snap in workloads.values():
                pod_spec = snap['spec']['template']['spec']
                account = accounts[pod_spec.get('serviceAccountName', 'default')]
                if account.get('imagePullSecrets') and not pod_spec.get('imagePullSecrets'):
                    raise RecoveryRequired('inherited image pull secrets require prior admission')
            if cut:
                if (self.cut_journal is None or self.cut_journal is self.journal or self.cut_journal.poisoned
                        or self.cut_journal.dirfd is None or self.cut_journal.lockfd is None
                        or self.cut_journal.intent != self.cut_intent or self.cut_journal.state['intent'] != self.cut_intent
                        or self.cut_intent['resource_context'] != self.fence.context
                        or self.cut_intent['namespace_uid'] != self.namespace_uid):
                    raise RecoveryRequired('writer journal custody invalid')
                controllers = self.cut_intent['writer_cut']['controllers']
                selected = [c for c in controllers if c.get('kind') == 'Deployment'
                            and c.get('metadata', {}).get('name') in CATALOG]
                if len(selected) != len(cut) or {c['metadata']['name'] for c in selected} != set(cut):
                    raise RecoveryRequired('writer cut admission differs')
                for item in selected:
                    name = item['metadata']['name']
                    if (item['metadata'].get('namespace') != 'voice-dev' or item['metadata'].get('uid') != workloads[name]['uid']
                            or item['spec'].get('replicas') != workloads[name]['resume_replicas']):
                        raise RecoveryRequired('writer cut original identity differs')
            proof = self.admission.proof
            writers = [w for w in proof['writers'] if w['kind'] == 'Deployment' and w['name'] in CATALOG]
            if (proof['operation_id'] != self.fence.context['operation_id'] or proof['namespace_uid'] != self.namespace_uid
                    or len(writers) != len(cut) or {w['name'] for w in writers} != set(cut)):
                raise RecoveryRequired('cut proof scope differs')
            for writer in writers:
                snap = workloads[writer['name']]
                if (writer['uid'] != snap['uid'] or writer['generation'] != snap['generation']
                        or writer['spec'] != snap['spec'] or writer['spec'].get('replicas') != 0):
                    raise RecoveryRequired('cut proof and image admission differ')
                if writer['successor'] is not None:
                    successor = self.admission.successors.get(writer['name'], {})
                    accepted = self.cut_journal.state['steps'].get(writer['successor'].get('step'), {})
                    if (successor.get('target_spec') != snap['spec'] or successor.get('receipt', {}).get('uid') != snap['uid']
                            or successor.get('receipt', {}).get('generation') != snap['generation']
                            or accepted.get('status') != 'done' or accepted.get('nonce') != writer['successor'].get('nonce')
                            or digest(accepted.get('receipt')) != writer['successor'].get('receipt_sha256')):
                        raise RecoveryRequired('config successor and cut proof differ')
        except (KeyError, TypeError, AttributeError, ValueError) as exc:
            raise RecoveryRequired('cohort identity invalid') from exc

    def _guard(self):
        self._identity()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        self._lineage()

    def _lineage(self):
        capture, cut = self.admission, self.admission.cut
        if (type(cut) is not WriterCut or cut.journal is not self.cut_journal
                or cut.poisoned or cut.intent != capture.cut_object_intent or cut.admission != capture.cut_object_admission
                or cut.invocation != capture.cut_invocation or cut.retired_rs != capture.retired_rs or cut.probe is not capture.probe
                or cut.journal.poisoned or cut.journal.dirfd is None or cut.journal.lockfd is None
                or cut.journal.intent != capture.cut_intent or cut.journal.state.get('intent') != capture.cut_intent
                or cut.fence.context != self.fence.context or cut.namespace_uid != self.namespace_uid or cut.lock != self.lock
                or type(cut.client) is not KubectlTransport or cut.client.journal is not cut.journal
                or getattr(cut.fence.ledger, 'client', None) is not cut.client):
            raise RecoveryRequired('cut custody changed')
        cut._guard()
        cut._recheck_config_successors()
        for key, value in capture.cut_steps.items():
            if cut.journal.state['steps'].get(key) != value:
                raise RecoveryRequired('cut journal lineage changed')
        for key in cut.journal.state['steps']:
            allowed = key.startswith('writer-resume-') or key in ('writer-cut-resume-inventory',
                'writer-cut-activation-plan', 'writer-cut-space-health', 'writer-cut-publisher-health',
                'writer-cut-chat-health', 'writer-cut-realtime-scope', 'writer-cut-provision-scope')
            if key not in capture.cut_steps and (not self.verifying or not allowed):
                raise RecoveryRequired('unexpected cut journal step')
        config = capture.config_journal
        if (config is not cut.config_journal or cut.config_snapshot != capture.config_snapshot
                or cut.successors != capture.successors):
            raise RecoveryRequired('config custody changed')
        if config is not None and (config.poisoned or config.dirfd is None or config.lockfd is None
                or config.intent != capture.config_intent or config.state.get('intent') != capture.config_intent
                or config.state['steps'] != capture.config_steps):
            raise RecoveryRequired('config journal lineage changed')
        first, first_rev = self.fence.ledger.read()
        second, second_rev = cut.fence.ledger.read()
        if (first != second or first_rev != second_rev or not isinstance(first_rev, tuple)
                or len(first_rev) != 2 or first_rev[0] != capture.ledger_uid or not self.fence.owned(first)
                or first.get('status') != 'active' or first.get('writer_cut_invocation') != capture.writer_invocation
                or first.get('backend_config_effects', {}) != capture.config_effects):
            raise RecoveryRequired('shared ledger lineage changed')
        effects = first.get('writer_cut_effects', {})
        if not isinstance(effects, dict) or any(effects.get(k) != v for k, v in capture.writer_effects.items()):
            raise RecoveryRequired('writer ledger lineage changed')
        if not self.verifying and effects != capture.writer_effects:
            raise RecoveryRequired('writer effects changed during image apply')

    def _uncertain(self):
        self.poisoned = True
        try:
            self.fence.uncertain('full cohort image apply outcome uncertain')
        except BaseException:
            pass
        raise RecoveryRequired('cohort outcome uncertain; retain lock and reconcile') from None

    def _start(self):
        steps = self.journal.state['steps']
        if self.started or 'full-cohort-invocation' in steps:
            raise RecoveryRequired('invocation already attempted')
        steps['full-cohort-invocation'] = {'status': 'pending', 'nonce': self.invocation}
        self.journal.save()
        self.fence.authority()
        row, rev = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or 'full_cohort_invocation' in row
                or row.get('full_cohort_effects', {}) != {}):
            raise RecoveryRequired('cohort invocation owned elsewhere')
        owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
        row['full_cohort_invocation'] = owner
        self.fence.ledger.write(rev, row)
        self._guard()
        current, _ = self.fence.ledger.read()
        if not self.fence.owned(current) or current.get('status') != 'active' or current.get('full_cohort_invocation') != owner:
            raise RecoveryRequired('cohort invocation changed')
        self._records(current)
        steps['full-cohort-invocation']['status'] = 'owned'
        self.journal.save()
        self.started = True

    def _records(self, row, omit=None):
        prefix = 'full-cohort-image-'
        allowed = {prefix + name for name in CATALOG}
        expected = {}
        for step, item in self.journal.state['steps'].items():
            if not step.startswith(prefix) or step == omit:
                continue
            if step not in allowed or not isinstance(item, dict) or item.get('status') not in ('pending', 'done'):
                raise RecoveryRequired('journal effect inventory invalid')
            if (not isinstance(item.get('nonce'), str) or not item['nonce']
                    or item.get('details_sha256') != digest(item.get('details'))):
                raise RecoveryRequired('journal effect identity invalid')
            record = {k: item[k] for k in ('status', 'nonce', 'details_sha256')}
            if item['status'] == 'done':
                if not isinstance(item.get('receipt'), dict):
                    raise RecoveryRequired('journal receipt invalid')
                record['receipt_sha256'] = digest(item['receipt'])
            expected[step] = record
        if row.get('full_cohort_effects', {}) != expected:
            raise RecoveryRequired('cohort effect map changed')

    def _reserve(self, name, details):
        steps = self.journal.state['steps']
        step = 'full-cohort-image-' + name
        if step in steps:
            raise RecoveryRequired('effect already attempted')
        record = {'status': 'pending', 'nonce': secrets.token_hex(16), 'details_sha256': digest(details)}
        steps[step] = record | {'details': copy.deepcopy(details)}
        self.journal.save()
        self.fence.authority()
        row, rev = self.fence.ledger.read()
        owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
        records = row.get('full_cohort_effects', {})
        if (not self.fence.owned(row) or row.get('status') != 'active' or row.get('full_cohort_invocation') != owner
                or not isinstance(records, dict) or step in records or any(v.get('status') != 'done' for v in records.values())):
            raise RecoveryRequired('cohort reservation invalid')
        self._records(row, omit=step)
        row['full_cohort_effects'] = copy.deepcopy(records) | {step: record}
        row['step'] = step
        self.fence.ledger.write(rev, row)
        self._guard()
        current, _ = self.fence.ledger.read()
        if (not self.fence.owned(current) or current.get('status') != 'active'
                or current.get('full_cohort_invocation') != owner or current.get('full_cohort_effects', {}).get(step) != record):
            raise RecoveryRequired('cohort reservation changed')
        self._records(current)
        return step, record

    def _complete(self, step, record, receipt):
        self._guard()
        row, rev = self.fence.ledger.read()
        owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
        if (not self.fence.owned(row) or row.get('status') != 'active' or row.get('full_cohort_invocation') != owner
                or row.get('full_cohort_effects', {}).get(step) != record):
            raise RecoveryRequired('cohort reservation lost')
        self._records(row)
        self.journal.state['steps'][step].update(status='done', receipt=copy.deepcopy(receipt))
        self.journal.save()
        row['full_cohort_effects'][step] = record | {'status': 'done', 'receipt_sha256': digest(receipt)}
        row['step'] = step + '-done'
        self.fence.ledger.write(rev, row)
        self._guard()
        current, _ = self.fence.ledger.read()
        if not self.fence.owned(current) or current.get('status') != 'active' or current.get('full_cohort_invocation') != owner:
            raise RecoveryRequired('cohort completion changed')
        self._records(current)

    def _get(self, kind, name):
        return self.client.call('get', kind, name, '--ignore-not-found=true', '-o', 'json')

    def _accounts(self):
        for name, expected in self.intent['full_cohort']['service_accounts'].items():
            if self._get('serviceaccount', name) != expected:
                raise RecoveryRequired('service account changed')

    def _writer_records(self, row):
        steps = self.cut_journal.state['steps']
        invocation = steps.get('writer-cut-invocation', {})
        owner = {'nonce': invocation.get('nonce'), 'operation_id': self.fence.context['operation_id']}
        if (invocation.get('status') != 'owned' or not isinstance(invocation.get('nonce'), str)
                or not re.fullmatch(r'[0-9a-f]{32}', invocation['nonce']) or row.get('writer_cut_invocation') != owner):
            raise RecoveryRequired('writer invocation unavailable')
        expected = {}
        for step, item in steps.items():
            if not (step.startswith('writer-drain-') or step.startswith('writer-resume-')
                    or step.startswith('writer-cut-config-successor-')
                    or re.fullmatch(r'writer-cut-migration-(?:plan|finish|cleanup)-[0-9a-f]{32}', step)
                    or step in ('writer-cut-drained', 'writer-cut-config-plan', 'writer-cut-resume-inventory',
                                'writer-cut-auth-successor', 'writer-cut-activation-plan', 'writer-cut-space-health',
                                'writer-cut-publisher-health', 'writer-cut-chat-health', 'writer-cut-realtime-scope',
                                'writer-cut-provision-scope', 'writer-cut-space-bind', 'writer-cut-space-finish')):
                continue
            pending_resume = (self.activation_checking and step.startswith('writer-resume-') and
                              isinstance(item, dict) and item.get('status') == 'pending')
            if pending_resume and getattr(self.admission.cut, 'activation', None) is None:
                raise RecoveryRequired('pending resume has no activation plan')
            if (not isinstance(item, dict) or (item.get('status') != 'done' and not pending_resume)
                    or not isinstance(item.get('nonce'), str) or not re.fullmatch(r'[0-9a-f]{32}', item['nonce'])
                    or item.get('details_sha256') != digest(item.get('details')) or
                    (not pending_resume and not isinstance(item.get('receipt'), dict)) or
                    (pending_resume and 'receipt' in item)):
                raise RecoveryRequired('writer effect incomplete or changed')
            expected[step] = {k: item[k] for k in ('status', 'nonce', 'details_sha256')}
            if not pending_resume:
                expected[step]['receipt_sha256'] = digest(item['receipt'])
        if row.get('writer_cut_effects') != expected or 'writer-cut-drained' not in expected:
            raise RecoveryRequired('writer effect map changed')
        cut = self.admission.cut
        space=getattr(cut,'space_fresh',None)
        if space is None:
            if 'writer-cut-space-bind' in steps or 'writer-cut-space-finish' in steps:
                raise RecoveryRequired('Space successor custody changed')
        else:
            import writer_cut_space
            if writer_cut_space.verify_binding(cut,require_finished=True)!=space:
                raise RecoveryRequired('Space successor custody changed')
        provision = getattr(cut, 'provision_successor', None)
        provision_step = steps.get('writer-cut-provision-scope')
        if (provision is None) != (provision_step is None):
            raise RecoveryRequired('provision scope custody changed')
        if provision is not None and (provision_step['details'] != provision['details'] or
                provision_step['receipt'] != provision['details'] or not cut._provision_scope_matches(row)):
            raise RecoveryRequired('provision scope receipt changed')
        effective = getattr(cut, 'effective_session', None)
        scope_step = steps.get('writer-cut-realtime-scope')
        if (effective is None) != (scope_step is None):
            raise RecoveryRequired('Realtime scope custody changed')
        if effective is not None:
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': cut.invocation,
                       'original_scope_sha256': digest(cut.admission['session_scope']),
                       'scope_sha256': digest(effective['scope']), 'migration_terminal_sha256': effective['terminal']}
            if (scope_step['details'] != details or scope_step['receipt'] != details or
                    not cut._effective_scope_matches(row)):
                raise RecoveryRequired('Realtime scope receipt changed')
        activation = getattr(cut, 'activation', None)
        activation_step = steps.get('writer-cut-activation-plan')
        if (activation is None) != (activation_step is None):
            raise RecoveryRequired('activation plan custody changed')
        if activation is not None:
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': cut.invocation,
                       'plan_sha256': activation['plan_sha256']}
            if (activation_step['details'] != details or activation_step['receipt'] != details or
                    not cut._activation_matches(row)):
                raise RecoveryRequired('activation plan receipt changed')
            cut.verify_activation_stage()
            pending = [step for step, item in steps.items() if step.startswith('writer-resume-') and item.get('status') == 'pending']
            if pending:
                ordered = activation['plan']['ordered']
                completed = len(cut.current_resumed_proof()['controllers'])
                if len(pending) != 1 or completed >= len(ordered):
                    raise RecoveryRequired('activation pending resume invalid')
                target = ordered[completed]
                step = 'writer-resume-' + target['kind'].lower() + '-' + target['name']
                source = steps[step]['details']['source'] if pending[0] == step else {}
                originals = [o for o in cut.admission['controllers'] if o['kind'] == target['kind'] and
                             o['metadata']['name'] == target['name']]
                resumed_spec = copy.deepcopy(target['spec'])
                if len(originals) != 1: raise RecoveryRequired('activation writer scope changed')
                resumed_spec['replicas'] = originals[0]['spec']['replicas']
                if (pending[0] != step or source.get('kind') != target['kind'] or
                        source.get('metadata', {}).get('name') != target['name'] or
                        source.get('metadata', {}).get('namespace') != 'voice-dev' or
                        source.get('metadata', {}).get('uid') != target['uid'] or
                        source.get('metadata', {}).get('generation') != target['generation'] or
                        source.get('spec') != target['spec'] or steps[step]['details'].get('target_spec') != resumed_spec):
                    raise RecoveryRequired('activation pending target changed')
            for name in ('space_health', 'publisher_health', 'chat_health'):
                step = 'writer-cut-' + name.replace('_', '-')
                health = activation['health'].get(name)
                if (health is None) != (step not in steps):
                    raise RecoveryRequired('activation health custody changed')
                if health is not None:
                    details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': cut.invocation,
                               'activation_plan_sha256': activation['plan_sha256'],
                               'health_receipt_sha256': health['receipt_sha256']}
                    if steps[step]['details'] != details or steps[step]['receipt'] != details:
                        raise RecoveryRequired('activation health receipt changed')
        auth = getattr(cut, 'auth_successor', None)
        auth_step = steps.get('writer-cut-auth-successor')
        if auth is None or auth_step is None:
            raise RecoveryRequired('Auth successor custody absent')
        if auth is not None:
            details = {'operation_id': self.fence.context['operation_id'], 'cut_invocation': cut.invocation,
                       'receipt_sha256': auth['receipt_sha256']}
            if (auth_step['details'] != details or auth_step['receipt'] != details or
                    not cut._auth_successor_matches(row)):
                raise RecoveryRequired('Auth successor receipt changed')
            candidate = auth.get('proof', {}).get('candidate', {})
            plan = self.intent['full_cohort']
            if ('voice-auth' not in plan['cut'] or self._target('voice-auth').get('replicas') != 0 or
                    digest(auth['proof']) != auth['receipt_sha256'] or
                    candidate.get('target_spec_sha256') != digest(self._target('voice-auth')) or
                    candidate.get('image') != plan['images']['voice-auth/auth'] or
                    candidate.get('platform_digest') != plan['platform_digests']['voice-auth/auth']):
                raise RecoveryRequired('Auth target is not source-qualified')
        inventory = getattr(cut, 'resume_inventory', None)
        inventory_step = steps.get('writer-cut-resume-inventory')
        if (inventory is None) != (inventory_step is None):
            raise RecoveryRequired('resume inventory custody changed')
        if inventory is not None:
            details = {'operation_id': self.fence.context['operation_id'],
                       'cut_invocation': cut.invocation, 'inventory_sha256': inventory['receipt_sha256']}
            if (inventory_step['details'] != details or inventory_step['receipt'] != details or
                    not cut._resume_inventory_matches(row)):
                raise RecoveryRequired('resume inventory receipt changed')
        histories = getattr(cut, 'migration_history', [])
        active = getattr(cut, 'migration', None)
        bound = list(histories) + ([active] if active is not None else [])
        expected_migrations = set()
        for binding in bound:
            nonce = binding['plan']['stable']['nonce']
            if not binding['finished']:
                raise RecoveryRequired('migration not finished')
            plan_step = 'writer-cut-migration-plan-' + nonce
            finish_step = 'writer-cut-migration-finish-' + nonce
            expected_migrations.update((plan_step, finish_step))
            plan_hash = digest(binding['plan'])
            if (steps[plan_step]['details'] != {'kind': binding['kind'], 'plan': binding['plan']} or
                    steps[plan_step]['receipt'] != {'operation_id': self.fence.context['operation_id'], 'plan_sha256': plan_hash}):
                raise RecoveryRequired('migration plan receipt changed')
            details = {'plan_sha256': plan_hash, 'job_uid': binding['job_uid'], 'terminal_sha256': binding['terminal']}
            if (steps[finish_step]['details'] != details or steps[finish_step]['receipt'] !=
                    {'operation_id': self.fence.context['operation_id'], **details}):
                raise RecoveryRequired('migration terminal receipt changed')
            if binding in histories:
                cleanup_step = 'writer-cut-migration-cleanup-' + nonce
                expected_migrations.add(cleanup_step)
                details = {'plan_sha256': plan_hash, 'job_uid': binding['job_uid'],
                           'config_uid': binding['config_uid'], 'terminal_sha256': binding['terminal']}
                if (steps[cleanup_step]['details'] != details or steps[cleanup_step]['receipt'] !=
                        {'operation_id': self.fence.context['operation_id'], **details}):
                    raise RecoveryRequired('migration cleanup receipt changed')
        if {step for step in expected if step.startswith('writer-cut-migration-')} != expected_migrations:
            raise RecoveryRequired('migration effect lineage changed')
        return expected

    def _drained(self, row):
        cut = set(self.intent['full_cohort']['cut'])
        if not cut:
            return
        effects = self._writer_records(row)
        if any(step.startswith('writer-resume-') for step in effects):
            raise RecoveryRequired('writer already resumed')
        drained = self.cut_journal.state['steps']['writer-cut-drained']
        scope = self.cut_intent['writer_cut']['session_scope']
        if (drained['details'] != {'session_scope': scope}
                or drained['receipt'] != {'scope': scope, 'active_sessions': 0, 'active_transactions': 0}
                or type(drained['receipt']['active_sessions']) is not int
                or type(drained['receipt']['active_transactions']) is not int):
            raise RecoveryRequired('writer drain receipt invalid')
        controllers = {c['metadata']['name']: c for c in self.cut_intent['writer_cut']['controllers']
                       if c.get('kind') == 'Deployment' and c.get('metadata', {}).get('name') in cut}
        for name in cut:
            step = 'writer-drain-deployment-' + name
            item = self.cut_journal.state['steps'].get(step, {})
            details, receipt = item.get('details', {}), item.get('receipt', {})
            source = details.get('source', {})
            original = controllers[name]
            target = copy.deepcopy(original['spec'])
            target['replicas'] = 0
            generation = original['metadata']['generation'] + (original['spec'].get('replicas') != 0)
            snap = self.intent['full_cohort']['workloads'][name]
            writer = next(w for w in self.admission.proof['writers'] if w['kind'] == 'Deployment' and w['name'] == name)
            if (step not in effects or not self._match(name, source, original['metadata']['uid'], original['spec'],
                                                       generation=original['metadata']['generation'])
                    or details.get('target_spec') != target or receipt.get('uid') != original['metadata']['uid']
                    or receipt.get('spec_sha256') != digest(target) or receipt.get('generation') != generation
                    or not receipt.get('responseResourceVersion') or not receipt.get('resourceVersion')
                    or writer['drain_step'] != step or writer['drain_nonce'] != item.get('nonce')
                    or writer['drain_receipt_sha256'] != digest(receipt)
                    or (writer['successor'] is None and (snap['generation'] != generation or snap['spec'] != target))):
                raise RecoveryRequired('writer drain evidence invalid')

    def _target(self, name):
        plan = self.intent['full_cohort']
        target = copy.deepcopy(plan['workloads'][name]['spec'])
        container = next(c for c in target['template']['spec']['containers'] if c['name'] == CATALOG[name])
        container['image'] = plan['images'][name + '/' + CATALOG[name]]
        return target

    def _partial(self, completed):
        """Recheck exact per-image predecessor/successor without reusing old cut spec."""
        self._guard()
        for name, snap in self.intent['full_cohort']['workloads'].items():
            receipt = completed.get(name)
            spec = receipt['spec'] if receipt else snap['spec']
            generation = receipt['generation'] if receipt else snap['generation']
            if not self._match(name, self._get('deployment', name), snap['uid'], spec, generation=generation):
                raise RecoveryRequired('partial cohort workload changed')
        cut = self.admission.cut
        cut._guards()
        if not cut._pods_drained():
            raise RecoveryRequired('writer pods remain')
        scope = (cut.current_provisioned_session_scope() if getattr(cut, 'provision_successor', None) is not None
                 else self.cut_intent['writer_cut']['session_scope'])
        observed = cut._session_observation()
        if (not isinstance(observed, dict)
                or observed != {'scope': scope, 'active_sessions': 0, 'active_transactions': 0}
                or type(observed.get('active_sessions')) is not int or type(observed.get('active_transactions')) is not int):
            raise RecoveryRequired('writer sessions remain')
        for writer in self.admission.proof['writers']:
            if writer['kind'] == 'Deployment' and writer['name'] in CATALOG:
                continue
            obj = self._get(writer['kind'].lower(), writer['name'])
            meta, status = obj.get('metadata', {}), obj.get('status', {})
            if (obj.get('kind') != writer['kind'] or meta.get('name') != writer['name']
                    or meta.get('namespace') != 'voice-dev' or meta.get('uid') != writer['uid']
                    or meta.get('generation') != writer['generation'] or meta.get('deletionTimestamp')
                    or obj.get('spec') != writer['spec'] or status.get('replicas', 0) != 0
                    or status.get('readyReplicas', 0) != 0):
                raise RecoveryRequired('other writer changed')
        self._guard()

    @staticmethod
    def _match(name, obj, uid, spec, rv=None, generation=None):
        meta = obj.get('metadata', {})
        return (obj.get('kind') == 'Deployment' and meta.get('namespace') == 'voice-dev' and meta.get('name') == name
                and meta.get('uid') == uid and not meta.get('deletionTimestamp') and bool(meta.get('resourceVersion'))
                and (generation is None or meta.get('generation') == generation)
                and (rv is None or meta.get('resourceVersion') == rv) and obj.get('spec') == spec)

    def apply(self):
        started = False
        try:
            self._guard()
            started = True
            self._start()
            receipts = {}
            self._accounts()
            row, _ = self.fence.ledger.read()
            self._drained(row)
            self._partial(receipts)
            for name in sorted(CATALOG):
                snap = self.intent['full_cohort']['workloads'][name]
                if not self._match(name, self._get('deployment', name), snap['uid'], snap['spec'], generation=snap['generation']):
                    raise RecoveryRequired('cohort preflight changed')
            for name in sorted(CATALOG):
                snap = self.intent['full_cohort']['workloads'][name]
                old, target = snap['spec'], self._target(name)
                current = self._get('deployment', name)
                if not self._match(name, current, snap['uid'], old, generation=snap['generation']):
                    raise RecoveryRequired('admitted workload changed')
                old_rv = current['metadata']['resourceVersion']
                target_generation = snap['generation'] + (old != target)
                changed = old != target
                patch = [{'op': 'test', 'path': '/metadata/uid', 'value': snap['uid']},
                         {'op': 'test', 'path': '/metadata/resourceVersion', 'value': old_rv},
                         {'op': 'test', 'path': '/metadata/generation', 'value': snap['generation']},
                         {'op': 'test', 'path': '/spec', 'value': old},
                         {'op': 'replace', 'path': '/spec', 'value': target}] if changed else None
                mode = 'patch' if changed else 'already-target'
                step, record = self._reserve(name, {'uid': snap['uid'], 'old': old, 'target': target, 'patch': patch,
                                                    'mode': mode})
                self._accounts()
                self._guard()
                row, _ = self.fence.ledger.read()
                owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
                if (not self.fence.owned(row) or row.get('status') != 'active' or row.get('full_cohort_invocation') != owner
                        or row.get('full_cohort_effects', {}).get(step) != record):
                    raise RecoveryRequired('cohort reservation changed before patch')
                self._records(row)
                self._drained(row)
                self._partial(receipts)
                response = self.client.patch(name, patch) if changed else self._get('deployment', name)
                if not self._match(name, response, snap['uid'], target, generation=target_generation):
                    raise RecoveryRequired('patch response differs')
                observed = self._get('deployment', name)
                if not self._match(name, observed, snap['uid'], target, generation=target_generation):
                    raise RecoveryRequired('patch readback differs')
                receipt = {'uid': snap['uid'], 'resourceVersion': observed['metadata']['resourceVersion'],
                           'generation': target_generation, 'spec': target, 'mode': mode}
                self._complete(step, record, receipt)
                receipts[name] = receipt
                self._partial(receipts)
            self._guard()
            self._accounts()
            for name, receipt in receipts.items():
                observed = self._get('deployment', name)
                if not self._match(name, observed, receipt['uid'], receipt['spec'], generation=receipt['generation']):
                    raise RecoveryRequired('cohort readback changed')
                receipt['resourceVersion'] = observed['metadata']['resourceVersion']
            self._guard()
            row, _ = self.fence.ledger.read()
            owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
            if not self.fence.owned(row) or row.get('status') != 'active' or row.get('full_cohort_invocation') != owner:
                raise RecoveryRequired('cohort completion ownership changed')
            self._records(row)
            self._drained(row)
            return receipts
        except BaseException:
            if started:
                self._uncertain()
            raise RecoveryRequired('cohort admission failed') from None

    def _resume_generations(self, row):
        cut = set(self.intent['full_cohort']['cut'])
        generations = {}
        for name in CATALOG:
            receipt = self.journal.state['steps']['full-cohort-image-' + name]['receipt']
            snap = self.intent['full_cohort']['workloads'][name]
            expected = snap['generation'] + (snap['spec'] != self._target(name))
            if receipt.get('generation') != expected:
                raise RecoveryRequired('image generation receipt invalid')
            generations[name] = expected
        if not cut:
            return generations
        self._writer_records(row)
        for name in cut:
            step = 'writer-resume-deployment-' + name
            item = self.cut_journal.state['steps'].get(step, {})
            details, receipt = item.get('details', {}), item.get('receipt', {})
            source = details.get('source', {})
            before, after = self._target(name), self._target(name)
            after['replicas'] = self.intent['full_cohort']['workloads'][name]['resume_replicas']
            expected_generation = generations[name] + (before != after)
            if (item.get('status') != 'done' or details.get('target_spec') != after
                    or not self._match(name, source, self.intent['full_cohort']['workloads'][name]['uid'], before,
                                       generation=generations[name])
                    or set(receipt) != {'uid', 'spec_sha256', 'responseResourceVersion', 'resourceVersion', 'generation'}
                    or receipt.get('uid') != self.intent['full_cohort']['workloads'][name]['uid']
                    or receipt.get('spec_sha256') != digest(after) or receipt.get('generation') != expected_generation
                    or not receipt.get('responseResourceVersion') or not receipt.get('resourceVersion')):
                raise RecoveryRequired('writer resume receipt invalid')
            generations[name] = expected_generation
        return generations

    def verify_activation_lineage(self):
        """Read the completed source-owned image receipts for activation admission."""
        entered = False
        try:
            if not self.started or self.verifying:
                raise RecoveryRequired('image invocation not available')
            self.verifying = True
            self.activation_checking = True
            entered = True
            self._guard()
            row, _ = self.fence.ledger.read()
            self._records(row)
            self._writer_records(row)
            receipts = {}
            for name in CATALOG:
                local = self.journal.state['steps'].get('full-cohort-image-' + name, {})
                receipt = local.get('receipt', {})
                if (local.get('status') != 'done' or receipt.get('uid') != self.intent['full_cohort']['workloads'][name]['uid']
                        or receipt.get('spec') != self._target(name) or not receipt.get('resourceVersion')):
                    raise RecoveryRequired('image receipt incomplete')
                receipts[name] = copy.deepcopy(receipt)
            self._guard()
            row, _ = self.fence.ledger.read()
            self._records(row)
            self._writer_records(row)
            if any(self.journal.state['steps'].get('full-cohort-image-' + name, {}).get('receipt') != receipt
                   for name, receipt in receipts.items()):
                raise RecoveryRequired('image receipts changed during activation check')
            return {'operation_id': self.fence.context['operation_id'], 'source_sha': self.fence.context['commit'],
                    'images': receipts}
        except BaseException:
            self._uncertain()
        finally:
            if entered:
                self.verifying = False
                self.activation_checking = False

    def verify_after_resume(self):
        entered = False
        try:
            if self.verifying:
                raise RecoveryRequired('image verification already in progress')
            self.verifying = True
            entered = True
            self._guard()
            if not self.started:
                raise RecoveryRequired('invocation not owned')
            row, _ = self.fence.ledger.read()
            owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
            if (not self.fence.owned(row) or row.get('status') != 'active'
                    or row.get('full_cohort_invocation') != owner):
                raise RecoveryRequired('cohort invocation changed')
            self._records(row)
            for name in CATALOG:
                step = 'full-cohort-image-' + name
                local = self.journal.state['steps'].get(step, {})
                remote = row.get('full_cohort_effects', {}).get(step, {})
                if (local.get('status') != 'done' or remote.get('status') != 'done'
                        or remote.get('nonce') != local.get('nonce')
                        or remote.get('details_sha256') != local.get('details_sha256')
                        or remote.get('receipt_sha256') != digest(local.get('receipt'))
                        or local.get('receipt', {}).get('spec') != self._target(name)
                        or local.get('receipt', {}).get('uid') != self.intent['full_cohort']['workloads'][name]['uid']
                        or not local.get('receipt', {}).get('resourceVersion')):
                    raise RecoveryRequired('cohort receipts incomplete')
            generations = self._resume_generations(row)
            self._accounts()
            replicasets = self.client.call('get', 'replicasets', '-o', 'json')['items']
            pods = self.client.call('get', 'pods', '-o', 'json')['items']
            deployment_ids = {snap['uid'] for snap in self.intent['full_cohort']['workloads'].values()}
            owned_rs = [r for r in replicasets if self._controller(r.get('metadata', {}), 'Deployment') in deployment_ids]
            rs_ids = {r['metadata']['uid'] for r in owned_rs}
            cohort_pods = [p for p in pods if self._controller(p.get('metadata', {}), 'ReplicaSet') in rs_ids]
            observations = {}
            for name, container in CATALOG.items():
                snap = self.intent['full_cohort']['workloads'][name]
                target = self._target(name)
                target['replicas'] = snap['resume_replicas']
                deployment = self._get('deployment', name)
                if not self._match(name, deployment, snap['uid'], target, generation=generations[name]):
                    raise RecoveryRequired('resumed deployment differs')
                meta, status = deployment['metadata'], deployment.get('status', {})
                replicas = snap['resume_replicas']
                if (not isinstance(meta.get('generation'), int)
                        or status.get('observedGeneration', 0) < meta['generation']
                        or status.get('updatedReplicas') != replicas or status.get('availableReplicas') != replicas):
                    raise RecoveryRequired('resumed deployment not ready')
                owned = [r for r in replicasets if self._controller(r.get('metadata', {}), 'Deployment') == snap['uid']]
                desired_template = target['template']
                def matches(rs):
                    template = copy.deepcopy(rs.get('spec', {}).get('template'))
                    if not isinstance(template, dict):
                        return False
                    template.get('metadata', {}).get('labels', {}).pop('pod-template-hash', None)
                    return template == desired_template
                current = [r for r in owned if matches(r)]
                owned_ids = {r['metadata']['uid'] for r in owned}
                owned_pods = [p for p in pods if self._controller(p.get('metadata', {}), 'ReplicaSet') in owned_ids]
                current_meta = current[0]['metadata'] if len(current) == 1 else {}
                if (len(current) != 1 or current_meta.get('namespace') != 'voice-dev'
                        or not current_meta.get('name') or not current_meta.get('uid')
                        or not current_meta.get('resourceVersion') or current_meta.get('deletionTimestamp')
                        or len(owned_pods) != replicas
                        or any(self._controller(p['metadata'], 'ReplicaSet') != current[0]['metadata']['uid']
                               or not self._valid_pod(name, p, target) for p in owned_pods)):
                    raise RecoveryRequired('resumed pods or ownership differ')
                observations[name] = {'uid': snap['uid'], 'resourceVersion': meta['resourceVersion'],
                    'spec_sha256': digest(target), 'pods': sorted((p['metadata']['name'], p['metadata']['uid'],
                                                                   p['metadata']['resourceVersion']) for p in owned_pods)}
            self._guard()
            self._accounts()
            for name, receipt in observations.items():
                target = self._target(name)
                target['replicas'] = self.intent['full_cohort']['workloads'][name]['resume_replicas']
                if not self._match(name, self._get('deployment', name), receipt['uid'], target, generation=generations[name]):
                    raise RecoveryRequired('resumed deployment changed')
            final_rs = [r for r in self.client.call('get', 'replicasets', '-o', 'json')['items']
                        if self._controller(r.get('metadata', {}), 'Deployment') in deployment_ids]
            final_rs_ids = {r['metadata']['uid'] for r in final_rs}
            final_pods = [p for p in self.client.call('get', 'pods', '-o', 'json')['items']
                          if self._controller(p.get('metadata', {}), 'ReplicaSet') in rs_ids | final_rs_ids]
            if (sorted(final_rs, key=lambda r: r['metadata']['uid']) != sorted(owned_rs, key=lambda r: r['metadata']['uid'])
                    or sorted(final_pods, key=lambda p: p['metadata']['uid']) != sorted(cohort_pods, key=lambda p: p['metadata']['uid'])):
                raise RecoveryRequired('resumed pod inventory changed')
            self._guard()
            row, _ = self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status') != 'active'
                    or row.get('full_cohort_invocation') != owner):
                raise RecoveryRequired('cohort verification ownership changed')
            self._records(row)
            if self._resume_generations(row) != generations:
                raise RecoveryRequired('writer resume identity changed')
            return observations
        except BaseException:
            self._uncertain()
        finally:
            if entered:
                self.verifying = False

    @staticmethod
    def _controller(meta, kind):
        owners = [o for o in meta.get('ownerReferences', []) if o.get('controller') is True and o.get('kind') == kind]
        return owners[0].get('uid') if len(owners) == 1 and owners[0].get('uid') else None

    def _valid_pod(self, name, pod, target):
        meta, status = pod.get('metadata', {}), pod.get('status', {})
        template_meta = target['template'].get('metadata', {})
        def bound_fields(value):
            if isinstance(value, dict):
                field = value.get('fieldPath')
                match = re.fullmatch(r"metadata\.(annotations|labels)\['([^']+)'\]", field) if isinstance(field, str) else None
                if match and meta.get(match.group(1), {}).get(match.group(2)) != template_meta.get(match.group(1), {}).get(match.group(2)):
                    return False
                return all(bound_fields(v) for v in value.values())
            return not isinstance(value, list) or all(bound_fields(v) for v in value)
        if (status.get('phase') != 'Running' or meta.get('namespace') != 'voice-dev' or not meta.get('name') or not meta.get('uid')
                or not meta.get('resourceVersion') or meta.get('deletionTimestamp')
                or any(meta.get('labels', {}).get(k) != v for k, v in target['template'].get('metadata', {}).get('labels', {}).items())
                or any(meta.get('annotations', {}).get(k) != v for k, v in target['template'].get('metadata', {}).get('annotations', {}).items())
                or not bound_fields(target['template']['spec'])
                or not any(c.get('type') == 'Ready' and c.get('status') == 'True' for c in status.get('conditions', []))):
            return False
        desired = target['template']['spec']
        spec = copy.deepcopy(pod.get('spec', {}))
        app = CATALOG[name]
        statuses = [c for c in status.get('containerStatuses', []) if c.get('name') == app]
        image_id = statuses[0].get('imageID', '') if len(statuses) == 1 else ''
        match = re.search(r'(sha256:[a-f0-9]{64})$', image_id) if isinstance(image_id, str) else None
        if (len(statuses) != 1 or statuses[0].get('ready') is not True or match is None
                or match.group(1) != self.intent['full_cohort']['platform_digests'][name + '/' + app]):
            return False
        account_name = desired.get('serviceAccountName', 'default')
        account = self.intent['full_cohort']['service_accounts'][account_name]
        enabled = desired.get('automountServiceAccountToken', account.get('automountServiceAccountToken', True))
        injected_names = set()
        for group in ('containers', 'initContainers'):
            for container in spec.get(group, []):
                mounts = container.get('volumeMounts', [])
                injected = [m for m in mounts if m.get('mountPath') == '/var/run/secrets/kubernetes.io/serviceaccount'
                            and m.get('name', '').startswith('kube-api-access-') and m.get('readOnly') is True]
                if len(injected) > 1 or (injected and enabled is not True):
                    return False
                if injected:
                    mount = injected[0]
                    if set(mount) != {'name', 'mountPath', 'readOnly'}:
                        return False
                    injected_names.add(mount['name'])
                    container['volumeMounts'] = [m for m in mounts if m != mount]
                    expected_containers = desired.get(group, [])
                    expected = next((c for c in expected_containers if c.get('name') == container.get('name')), {})
                    if not container['volumeMounts'] and 'volumeMounts' not in expected:
                        container.pop('volumeMounts')
        if len(injected_names) > 1:
            return False
        if injected_names:
            injected_name = next(iter(injected_names))
            volumes = [v for v in spec.get('volumes', []) if v.get('name') == injected_name]
            if len(volumes) != 1 or set(volumes[0]) != {'name', 'projected'}:
                return False
            projected = volumes[0]['projected']
            sources = projected.get('sources', [])
            if (set(projected) != {'sources', 'defaultMode'} or projected.get('defaultMode') != 420
                    or len(sources) != 3 or [set(s) for s in sources] != [{'serviceAccountToken'}, {'configMap'}, {'downwardAPI'}]
                    or set(sources[0]['serviceAccountToken']) != {'path', 'expirationSeconds'}
                    or sources[0]['serviceAccountToken'].get('path') != 'token'
                    or type(sources[0]['serviceAccountToken'].get('expirationSeconds')) is not int
                    or not 600 <= sources[0]['serviceAccountToken']['expirationSeconds'] <= 86400
                    or sources[1]['configMap'] != {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}
                    or sources[2]['downwardAPI'] != {'items': [{'path': 'namespace', 'fieldRef': {
                        'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}):
                return False
            spec['volumes'] = [v for v in spec.get('volumes', []) if v.get('name') != injected_name]
            if not spec['volumes'] and 'volumes' not in desired:
                spec.pop('volumes')
        defaults = {'serviceAccountName': 'default', 'serviceAccount': account_name, 'dnsPolicy': 'ClusterFirst',
                    'restartPolicy': 'Always', 'schedulerName': 'default-scheduler', 'terminationGracePeriodSeconds': 30,
                    'enableServiceLinks': True, 'preemptionPolicy': 'PreemptLowerPriority', 'priority': 0, 'securityContext': {}}
        for key, value in defaults.items():
            if key not in desired and spec.get(key) == value:
                spec.pop(key)
        if 'nodeName' not in desired:
            spec.pop('nodeName', None)
        if 'tolerations' not in desired and spec.get('tolerations') == [
                {'key': 'node.kubernetes.io/not-ready', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300},
                {'key': 'node.kubernetes.io/unreachable', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300}]:
            spec.pop('tolerations')
        return spec == desired
