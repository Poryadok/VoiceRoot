"""Fenced, journalled DEV frontend migration. Run only under the common owner.

Construct KubectlTransport(journal), bind KubeLedger and Fence to this same
client, then construct FrontendExecutor(journal, fence, client, lock,
namespace_uid, images, platform_digests) and run each of web/admin/portal once.
The images are the exact three deployment/container references in the fence;
platform_digests are authenticated OCI platform manifest digests keyed by the
same names and also pinned in journal.intent['frontend_platform_digests'].
The caller must qualify their package/source and architecture before invoking.
An index digest is not assumed to equal the runtime imageID. Missing mapping
blocks admission. For an existing adopted config, immutable intent must also
contain frontend_existing_config[app] with the qualified actual ConfigMap
UID/resourceVersion/content SHA256. For all three apps, frontend_deployment_baseline
must bind the separately qualified UID/generation/full spec before any change.
The executor rereads source-owned public bytes and exact deployment under the fence. The receipt is current inventory,
not a claim about any previous release source. This phase neither acquires nor releases the claim or lock,
and never retries an effect after an unknown response. Full plans are private.
"""
import copy
import hashlib
import json
import os
import re
import secrets
import selectors
import subprocess
import time

import frontend_runtime_adoption as adoption
import frontend_runtime_reuse as reuse
import media_operation_guard


class RecoveryRequired(RuntimeError):
    pass


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


class KubectlTransport:
    """Fixed host transport: full specifications never appear in command argv."""
    PREFIX = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
              '--namespace', 'voice-dev', '--request-timeout=15s']

    def __init__(self, journal):
        self.journal = journal

    def call(self, *args, body=None):
        payload = None if body is None else json.dumps(body).encode()
        if args and args[0] == 'patch' and '--patch-file=-' in args:
            if not isinstance(body, list):
                raise RecoveryRequired('ledger JSON patch required')
            args = tuple('--patch=' + json.dumps(body) if arg == '--patch-file=-' else arg for arg in args)
            payload = None
        result = subprocess.run(self.PREFIX + list(args), input=payload, stdout=subprocess.PIPE,
                                stderr=subprocess.DEVNULL, timeout=25, check=True)
        return json.loads(result.stdout) if result.stdout.strip() else {}

    def patch(self, name, patch):
        filename = '.frontend-patch-' + secrets.token_hex(16)
        fd = os.open(filename, os.O_CREAT | os.O_EXCL | os.O_WRONLY | os.O_NOFOLLOW,
                     0o600, dir_fd=self.journal.dirfd)
        try:
            with os.fdopen(fd, 'wb') as stream:
                stream.write(json.dumps(patch, separators=(',', ':')).encode())
                stream.flush()
                os.fsync(stream.fileno())
            os.fsync(self.journal.dirfd)
            path = str(self.journal.path / filename)
            result = subprocess.run(self.PREFIX + ['patch', 'deployment', name, '--type=json',
                '--patch-file=' + path, '-o', 'json'], stdout=subprocess.PIPE,
                stderr=subprocess.DEVNULL, timeout=25, check=True)
            return json.loads(result.stdout)
        finally:
            os.unlink(filename, dir_fd=self.journal.dirfd)
            os.fsync(self.journal.dirfd)

    def read_pod(self, name):
        if not re.fullmatch(r'[a-z0-9]([-a-z0-9]*[a-z0-9])?', name) or len(name) > 253:
            raise RecoveryRequired('invalid pod name')
        path = '/api/v1/namespaces/voice-dev/pods/' + name + ':80/proxy/voice-runtime-config.json'
        process = subprocess.Popen(self.PREFIX + ['get', '--raw=' + path], stdout=subprocess.PIPE,
                                   stderr=subprocess.DEVNULL)
        selector = selectors.DefaultSelector()
        try:
            selector.register(process.stdout, selectors.EVENT_READ)
            deadline = time.monotonic() + 25
            output = bytearray()
            while True:
                remaining = deadline - time.monotonic()
                if remaining <= 0 or not selector.select(remaining):
                    raise RecoveryRequired('pod config read timed out')
                chunk = os.read(process.stdout.fileno(), 65537 - len(output))
                if not chunk:
                    break
                output.extend(chunk)
                if len(output) > 65536:
                    raise RecoveryRequired('pod config response too large')
            if process.wait(timeout=max(0.1, deadline-time.monotonic())) != 0:
                raise RecoveryRequired('pod config read failed')
            return bytes(output)
        finally:
            selector.close()
            if process.poll() is None:
                process.kill()
                process.wait()
            process.stdout.close()


class FrontendExecutor:
    def __init__(self, journal, fence, client, lock, namespace_uid, images, platform_digests, transport=None):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.intent = copy.deepcopy(journal.intent)
        self.images = copy.deepcopy(images)
        self.platform_digests = copy.deepcopy(platform_digests)
        self.transport = transport if transport is not None else client
        if (self.transport is not client or getattr(fence.ledger, 'client', None) is not client
                or type(client) is not KubectlTransport or client.journal is not journal):
            raise RecoveryRequired('single fixed Kubernetes client required')
        self.invocation = secrets.token_hex(16)
        self.started = self.poisoned = False
        self._identity()

    def _identity(self):
        expected = {name + '/' + container for name, container in adoption.APPS.values()}
        existing = self.intent.get('frontend_existing_config', {})
        baselines = self.intent.get('frontend_deployment_baseline')
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent
                or self.journal.state.get('intent') != self.intent
                or self.intent.get('resource_context') != self.fence.context
                or self.intent.get('namespace_uid') != self.namespace_uid
                or self.intent.get('frontend_platform_digests') != self.platform_digests
                or not isinstance(self.images, dict) or set(self.images) != expected
                or not isinstance(self.platform_digests, dict) or set(self.platform_digests) != expected
                or not isinstance(existing, dict) or set(existing) - set(adoption.APPS)
                or not isinstance(baselines, dict) or set(baselines) != set(adoption.APPS)
                or any(not isinstance(v, dict) or set(v) != {'uid', 'generation', 'spec'}
                       or not isinstance(v['uid'], str) or not v['uid']
                       or type(v['generation']) is not int or v['generation'] < 1
                       or not isinstance(v['spec'], dict) for v in baselines.values())
                or any(not isinstance(v, dict) or set(v) != {'uid', 'resourceVersion', 'sha256'}
                       or not isinstance(v['uid'], str) or not v['uid']
                       or not isinstance(v['resourceVersion'], str) or not v['resourceVersion']
                       or not isinstance(v['sha256'], str) or not re.fullmatch(r'[a-f0-9]{64}', v['sha256'])
                       for v in existing.values())
                or any(not isinstance(v, str) or not re.fullmatch(r'sha256:[a-f0-9]{64}', v)
                       for v in self.platform_digests.values())
                or any(self.fence.context.get('images', {}).get(k) != v for k, v in self.images.items())):
            raise RecoveryRequired('frontend operation identity invalid')

    def _guard(self):
        self._identity()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        if self.started:
            row, _ = self.fence.ledger.read()
            if not self._owned(row) or row.get('frontend_effects', {}) != self._effect_map():
                raise RecoveryRequired('frontend invocation or effect map changed')

    def _owned(self, row):
        return (self.fence.owned(row) and row.get('status') == 'active' and row.get('frontend_invocation') ==
                {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']})

    def _effect_map(self, exclude=None):
        effects = {}
        admitted = {'frontend-' + app + '-' + stage for app in adoption.APPS for stage in ('config', 'deployment')}
        for name, entry in self.journal.state['steps'].items():
            if name not in admitted or name == exclude:
                continue
            if (not isinstance(entry, dict) or entry.get('status') not in ('pending', 'done')
                    or not isinstance(entry.get('nonce'), str) or not re.fullmatch(r'[0-9a-f]{32}', entry['nonce'])
                    or entry.get('details_sha256') != digest(entry.get('details'))):
                raise RecoveryRequired('frontend journal record changed')
            record = {key: entry[key] for key in ('status', 'nonce', 'details_sha256')}
            if entry['status'] == 'done':
                if 'receipt' not in entry:
                    raise RecoveryRequired('frontend receipt missing')
                record['receipt_sha256'] = digest(entry['receipt'])
            effects[name] = record
        return effects

    def _uncertain(self):
        self.poisoned = True
        try:
            self.fence.uncertain('frontend adoption outcome uncertain')
        except BaseException:
            pass
        raise RecoveryRequired('frontend outcome uncertain; retain lock and reconcile') from None

    def _start(self):
        if self.started:
            return
        steps = self.journal.state['steps']
        if 'frontend-invocation' in steps:
            raise RecoveryRequired('frontend invocation already attempted')
        steps['frontend-invocation'] = {'status': 'pending', 'nonce': self.invocation}
        self.journal.save()
        self.fence.authority()
        row, revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or 'frontend_invocation' in row
                or row.get('frontend_effects', {})):
            raise RecoveryRequired('frontend invocation owned elsewhere')
        owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
        row['frontend_invocation'] = owner
        self.fence.ledger.write(revision, row)
        self._guard()
        current, _ = self.fence.ledger.read()
        if not self.fence.owned(current) or current.get('status') != 'active' or current.get('frontend_invocation') != owner:
            raise RecoveryRequired('frontend invocation changed')
        steps['frontend-invocation'] = {'status': 'owned', 'nonce': self.invocation}
        self.journal.save()
        self.started = True

    def _reservation(self, name, details):
        steps = self.journal.state['steps']
        if name in steps:
            raise RecoveryRequired('effect already attempted')
        nonce = secrets.token_hex(16)
        record = {'status': 'pending', 'nonce': nonce, 'details_sha256': digest(details)}
        steps[name] = record | {'details': copy.deepcopy(details)}
        self.journal.save()
        self.fence.authority()
        row, revision = self.fence.ledger.read()
        owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
        records = row.get('frontend_effects', {})
        if (not self.fence.owned(row) or row.get('status') != 'active'
                or row.get('frontend_invocation') != owner or not isinstance(records, dict)
                or name in records or records != self._effect_map(exclude=name)
                or any(not isinstance(v, dict) or v.get('status') != 'done' for v in records.values())):
            raise RecoveryRequired('another frontend effect pending')
        row['frontend_effects'] = copy.deepcopy(records) | {name: record}
        row['step'] = name
        self.fence.ledger.write(revision, row)
        self._guard()
        current, _ = self.fence.ledger.read()
        if (not self.fence.owned(current) or current.get('status') != 'active'
                or current.get('frontend_invocation') != owner
                or current.get('frontend_effects', {}).get(name) != record):
            raise RecoveryRequired('frontend reservation changed')
        return record

    def _complete(self, name, record, receipt):
        self._guard()
        row, revision = self.fence.ledger.read()
        owner = {'nonce': self.invocation, 'operation_id': self.fence.context['operation_id']}
        if (not self.fence.owned(row) or row.get('status') != 'active'
                or row.get('frontend_invocation') != owner
                or row.get('frontend_effects', {}) != self._effect_map()
                or row.get('frontend_effects', {}).get(name) != record):
            raise RecoveryRequired('frontend reservation lost')
        self.journal.state['steps'][name]['status'] = 'done'
        self.journal.state['steps'][name]['receipt'] = copy.deepcopy(receipt)
        self.journal.save()
        row['frontend_effects'][name] = record | {'status': 'done', 'receipt_sha256': digest(receipt)}
        row['step'] = name + '-done'
        self.fence.ledger.write(revision, row)
        self._guard()

    def _get(self, kind, name):
        return self.client.call('get', kind, name, '--ignore-not-found=true', '-o', 'json')

    def _config(self, app):
        name = adoption.APPS[app][0] + '-runtime-config'
        actual = self._get('configmap', name)
        receipt = adoption.validate_configmap(app, actual)
        if actual['data'][adoption.KEY] != adoption.configmap(app)['data'][adoption.KEY]:
            raise RecoveryRequired('public config bytes differ from source')
        return receipt

    def run(self, app):
        if app not in adoption.APPS:
            raise RecoveryRequired('app outside frontend scope')
        started = False
        try:
            self._guard()
            started = True
            self._start()
            name, container = adoption.APPS[app]
            old = self._get('deployment', name)
            old_generation = old.get('metadata', {}).get('generation')
            replicas = old.get('spec', {}).get('replicas', 1)
            if type(old_generation) is not int or old_generation < 1 or type(replicas) is not int or replicas < 1:
                raise RecoveryRequired('live frontend generation or replicas invalid')
            baseline = self.intent['frontend_deployment_baseline'][app]
            if (old['metadata']['uid'] != baseline['uid'] or old_generation != baseline['generation']
                    or old['spec'] != baseline['spec']):
                raise RecoveryRequired('qualified frontend deployment baseline changed')
            existing = self._get('configmap', name + '-runtime-config')
            pinned = self.intent.get('frontend_existing_config', {}).get(app)
            if existing:
                if pinned is None:
                    raise RecoveryRequired('qualified existing config receipt required')
                plan = reuse.plan(app, old, self.images[name + '/' + container])
                if self._config(app) != pinned:
                    raise RecoveryRequired('existing config identity or contents changed')
                config_details = {'mode': 'reuse', 'qualified_receipt': copy.deepcopy(pinned),
                                  'configmap': plan['configmap']}
            else:
                if pinned is not None:
                    raise RecoveryRequired('qualified config is missing')
                plan = adoption.plan(app, old, self.images[name + '/' + container])
                # Kubernetes defaults ConfigMap volume mode during admission.
                plan['target_spec']['template']['spec']['volumes'][-1]['configMap']['defaultMode'] = 420
                plan['patch'][-1]['value'] = copy.deepcopy(plan['target_spec'])
                config_details = plan['configmap']
            changed = plan['target_spec'] != plan['old_spec']
            plan['expected_generation'] = old_generation + int(changed)
            old_pods = self._owned_pods(old)
            config_step = 'frontend-' + app + '-config'
            record = self._reservation(config_step, config_details)
            self._guard()
            row, _ = self.fence.ledger.read()
            if not self._owned(row) or row.get('frontend_effects', {}) != self._effect_map():
                raise RecoveryRequired('frontend config reservation changed')
            if existing:
                config_receipt = self._config(app)
                if config_receipt != pinned:
                    raise RecoveryRequired('existing config changed during qualification')
            else:
                created = self.client.call('create', '-f', '-', '-o', 'json', body=plan['configmap'])
                created_receipt = adoption.validate_configmap(app, created)
                config_receipt = self._config(app)
                if config_receipt != created_receipt:
                    raise RecoveryRequired('created config identity changed')
            self._complete(config_step, record, config_receipt)
            account_name = plan['target_spec']['template']['spec'].get('serviceAccountName', 'default')
            service_account = self._get('serviceaccount', account_name)
            account_meta = service_account.get('metadata', {})
            if (account_meta.get('name') != account_name or account_meta.get('namespace') != 'voice-dev'
                    or not account_meta.get('uid') or not account_meta.get('resourceVersion')
                    or account_meta.get('deletionTimestamp')):
                raise RecoveryRequired('service account identity invalid')
            deployment_step = 'frontend-' + app + '-deployment'
            record = self._reservation(deployment_step, {'plan': plan, 'config': config_receipt, 'old_pods': old_pods,
                                                         'mode': 'patch' if changed else 'already-target',
                                                         'service_account': service_account})
            if self._config(app) != config_receipt or self._get('serviceaccount', account_name) != service_account:
                raise RecoveryRequired('config or service account changed before patch')
            self._guard()
            row, _ = self.fence.ledger.read()
            if (not self._owned(row) or row.get('frontend_effects', {}) != self._effect_map()
                    or row.get('frontend_effects', {}).get(deployment_step) != record):
                raise RecoveryRequired('frontend deployment reservation changed')
            response = self.transport.patch(name, plan['patch']) if changed else self._get('deployment', name)
            if (adoption.classify(plan, response) != 'target' or
                    response.get('metadata', {}).get('generation') != plan['expected_generation']):
                raise RecoveryRequired('patch response does not match target')
            receipt = self._rollout(app, plan, old_pods if changed else [], config_receipt, service_account)
            receipt['mode'] = 'patch' if changed else 'already-target'
            self._complete(deployment_step, record, receipt)
            return receipt
        except BaseException:
            if started:
                self._uncertain()
            raise RecoveryRequired('frontend admission failed') from None

    def verify_current(self, app):
        """Recheck actual live Pods and served bytes after the cohort rollout."""
        if app not in adoption.APPS:
            raise RecoveryRequired('app outside frontend scope')
        try:
            self._guard()
            config_step = self.journal.state['steps'].get('frontend-' + app + '-config', {})
            deployment_step = self.journal.state['steps'].get('frontend-' + app + '-deployment', {})
            if config_step.get('status') != 'done' or deployment_step.get('status') != 'done':
                raise RecoveryRequired('frontend rollout receipt missing')
            details = deployment_step['details']
            if details['config'] != config_step['receipt']:
                raise RecoveryRequired('frontend receipt mismatch')
            result = self._rollout(app, details['plan'], [], details['config'], details['service_account'])
            self._guard()
            result['mode'] = 'verify-current'
            return result
        except BaseException:
            self._uncertain()

    @staticmethod
    def _controller(meta, kind):
        owners = [o for o in meta.get('ownerReferences', []) if o.get('controller') is True and o.get('kind') == kind]
        if len(owners) != 1 or not owners[0].get('uid'):
            return None
        return owners[0]['uid']

    def _owned_pods(self, deployment):
        dep_uid = deployment['metadata']['uid']
        replicasets = self.client.call('get', 'replicasets', '-o', 'json')['items']
        rs_uids = {r['metadata']['uid'] for r in replicasets if self._controller(r['metadata'], 'Deployment') == dep_uid}
        pods = self.client.call('get', 'pods', '-o', 'json')['items']
        return sorted(p['metadata']['uid'] for p in pods if self._controller(p['metadata'], 'ReplicaSet') in rs_uids)

    def _rollout(self, app, plan, old_pods, config_receipt, service_account):
        name, container = adoption.APPS[app]
        deadline = time.monotonic() + 180
        while time.monotonic() < deadline:
            self._guard()
            if (self._config(app) != config_receipt or
                    self._get('serviceaccount', service_account['metadata']['name']) != service_account):
                raise RecoveryRequired('config or service account identity changed')
            deployment = self._get('deployment', name)
            if (adoption.classify(plan, deployment) != 'target' or
                    deployment.get('metadata', {}).get('generation') != plan['expected_generation']):
                raise RecoveryRequired('deployment identity or spec changed')
            meta, status = deployment['metadata'], deployment.get('status', {})
            replicas = plan['target_spec'].get('replicas', 1)
            if (type(replicas) is not int or replicas < 1):
                raise RecoveryRequired('frontend replicas invalid')
            replicasets = self.client.call('get', 'replicasets', '-o', 'json')['items']
            owned = [r for r in replicasets if self._controller(r['metadata'], 'Deployment') == plan['uid']]
            target_template = copy.deepcopy(plan['target_spec']['template'])
            def matches(rs):
                template = copy.deepcopy(rs.get('spec', {}).get('template'))
                if not isinstance(template, dict):
                    return False
                template.get('metadata', {}).get('labels', {}).pop('pod-template-hash', None)
                return template == target_template
            current = [r for r in owned if matches(r)]
            owned_ids = {r['metadata']['uid'] for r in owned}
            pods = self.client.call('get', 'pods', '-o', 'json')['items']
            pods = [p for p in pods if self._controller(p['metadata'], 'ReplicaSet') in owned_ids]
            def valid_pod(pod):
                spec = pod.get('spec', {})
                template_spec = plan['target_spec']['template']['spec']
                matches = [c for c in spec.get('containers', []) if c.get('name') == container]
                wanted = [c for c in plan['target_spec']['template']['spec']['containers'] if c.get('name') == container]
                volumes = [v for v in spec.get('volumes', []) if v.get('name') == 'runtime-config']
                mounts = [m for m in matches[0].get('volumeMounts', []) if m.get('name') == 'runtime-config'] if len(matches) == 1 else []
                wanted_mounts = [m for m in wanted[0].get('volumeMounts', []) if m.get('name') == 'runtime-config']
                statuses = [c for c in pod.get('status', {}).get('containerStatuses', []) if c.get('name') == container]
                image_id = statuses[0].get('imageID', '') if len(statuses) == 1 else ''
                image_digest = re.search(r'(sha256:[a-f0-9]{64})$', image_id) if isinstance(image_id, str) else None
                actual_container = copy.deepcopy(matches[0]) if len(matches) == 1 else {}
                expected_container = copy.deepcopy(wanted[0])
                # Kubernetes may inject its own projected service-account mount.
                actual_mounts = actual_container.get('volumeMounts', [])
                injected = [m for m in actual_mounts if m.get('mountPath') == '/var/run/secrets/kubernetes.io/serviceaccount'
                            and m.get('name', '').startswith('kube-api-access-') and m.get('readOnly') is True]
                if len(injected) > 1:
                    return False
                if injected:
                    account_name = template_spec.get('serviceAccountName', 'default')
                    account = service_account
                    account_meta = account.get('metadata', {})
                    if (account_meta.get('name') != account_name or account_meta.get('namespace') != 'voice-dev'
                            or not account_meta.get('uid') or not account_meta.get('resourceVersion')
                            or account_meta.get('deletionTimestamp')):
                        return False
                    enabled = template_spec.get('automountServiceAccountToken', account.get('automountServiceAccountToken', True))
                    if enabled is not True:
                        return False
                    if set(injected[0]) != {'name', 'mountPath', 'readOnly'}:
                        return False
                    injected_volumes = [v for v in spec.get('volumes', []) if v.get('name') == injected[0]['name']]
                    if len(injected_volumes) != 1 or set(injected_volumes[0]) != {'name', 'projected'}:
                        return False
                    projected = injected_volumes[0]['projected']
                    if set(projected) != {'sources', 'defaultMode'} or projected['defaultMode'] != 420:
                        return False
                    sources = projected.get('sources', [])
                    if (len(sources) != 3 or [set(s) for s in sources] !=
                            [{'serviceAccountToken'}, {'configMap'}, {'downwardAPI'}]
                            or set(sources[0]['serviceAccountToken']) != {'path', 'expirationSeconds'}
                            or sources[0]['serviceAccountToken']['path'] != 'token'
                            or type(sources[0]['serviceAccountToken']['expirationSeconds']) is not int
                            or sources[1]['configMap'] != {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}
                            or sources[2]['downwardAPI'] != {'items': [{'path': 'namespace', 'fieldRef': {
                                'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}):
                        return False
                actual_spec = copy.deepcopy(spec)
                desired_spec = copy.deepcopy(template_spec)
                if injected:
                    actual_spec['volumes'] = [v for v in actual_spec.get('volumes', []) if v.get('name') != injected[0]['name']]
                    actual_spec['containers'][0]['volumeMounts'] = [m for m in actual_mounts if m != injected[0]]
                # These are API/scheduler defaults, not arbitrary admission mutations.
                defaults = {'serviceAccountName': 'default', 'serviceAccount': 'default',
                            'dnsPolicy': 'ClusterFirst', 'restartPolicy': 'Always', 'schedulerName': 'default-scheduler',
                            'terminationGracePeriodSeconds': 30, 'enableServiceLinks': True,
                            'preemptionPolicy': 'PreemptLowerPriority', 'priority': 0, 'securityContext': {}}
                for key, value in defaults.items():
                    if key not in desired_spec and actual_spec.get(key) == value:
                        actual_spec.pop(key)
                if 'nodeName' not in desired_spec:
                    actual_spec.pop('nodeName', None)
                # The default admission plugin adds two bounded NoExecute tolerations.
                if 'tolerations' not in desired_spec and 'tolerations' in actual_spec:
                    allowed = [{'key': 'node.kubernetes.io/not-ready', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300},
                               {'key': 'node.kubernetes.io/unreachable', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300}]
                    if actual_spec['tolerations'] == allowed:
                        actual_spec.pop('tolerations')
                actual_container['volumeMounts'] = [m for m in actual_mounts if not (
                    m.get('mountPath') == '/var/run/secrets/kubernetes.io/serviceaccount'
                    and m.get('name', '').startswith('kube-api-access-') and m.get('readOnly') is True)]
                expected_container.setdefault('volumeMounts', [])
                if actual_container.get('volumeMounts') == [] and 'volumeMounts' not in wanted[0]:
                    actual_container.pop('volumeMounts', None)
                    expected_container.pop('volumeMounts', None)
                return (len(matches) == 1 and matches[0].get('image') == wanted[0]['image']
                        and actual_spec == desired_spec
                        and len(spec.get('containers', [])) == len(template_spec.get('containers', [])) == 1
                        and spec.get('initContainers', []) == template_spec.get('initContainers', [])
                        and not spec.get('ephemeralContainers') and actual_container == expected_container
                        and mounts == wanted_mounts and len(volumes) == 1
                        and volumes[0] == next(v for v in template_spec['volumes'] if v.get('name') == 'runtime-config')
                        and len(statuses) == 1 and statuses[0].get('ready') is True and image_digest is not None
                        and image_digest.group(1) == self.platform_digests[name + '/' + container])
            if (len(current) == 1 and status.get('observedGeneration', 0) >= meta.get('generation', 0)
                    and status.get('updatedReplicas') == replicas and status.get('availableReplicas') == replicas
                    and len(pods) == replicas and not set(old_pods).intersection(p['metadata']['uid'] for p in pods)
                    and all(self._controller(p['metadata'], 'ReplicaSet') == current[0]['metadata']['uid']
                            and p['metadata'].get('namespace') == 'voice-dev' and not p['metadata'].get('deletionTimestamp')
                            and valid_pod(p)
                            and any(c.get('type') == 'Ready' and c.get('status') == 'True'
                                    for c in p.get('status', {}).get('conditions', [])) for p in pods)):
                identities = sorted((p['metadata']['name'], p['metadata']['uid'], p['metadata']['resourceVersion']) for p in pods)
                expected = adoption.configmap(app)['data'][adoption.KEY].encode()
                for pod_name, _, _ in identities:
                    if self.transport.read_pod(pod_name) != expected:
                        raise RecoveryRequired('served config differs')
                self._guard()
                final_deployment = self._get('deployment', name)
                if (self._config(app) != config_receipt or adoption.classify(plan, final_deployment) != 'target' or
                        final_deployment.get('metadata', {}).get('generation') != plan['expected_generation']):
                    raise RecoveryRequired('rollout identity changed')
                final_rs = self.client.call('get', 'replicasets', '-o', 'json')['items']
                final_owned = [r for r in final_rs if self._controller(r['metadata'], 'Deployment') == plan['uid']]
                final_pods = self.client.call('get', 'pods', '-o', 'json')['items']
                final_pods = [p for p in final_pods if self._controller(p['metadata'], 'ReplicaSet') in owned_ids]
                if (sorted(final_owned, key=lambda r: r['metadata']['uid']) != sorted(owned, key=lambda r: r['metadata']['uid'])
                        or sorted(final_pods, key=lambda p: p['metadata']['uid']) != sorted(pods, key=lambda p: p['metadata']['uid'])
                        or self._get('serviceaccount', service_account['metadata']['name']) != service_account):
                    raise RecoveryRequired('pod inventory changed')
                return {'deployment_uid': plan['uid'], 'spec_sha256': digest(plan['target_spec']),
                        'config': config_receipt, 'pods': identities}
            time.sleep(2)
        raise RecoveryRequired('rollout not terminal')
