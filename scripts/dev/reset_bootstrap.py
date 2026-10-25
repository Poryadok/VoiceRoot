"""Explicit disposable-DEV reset preparation; no implicit cluster mutation."""
import json
from pathlib import Path
import subprocess
import hashlib
import os
import base64
import tempfile
import time
import re
import argparse
import sys
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone

import environment as dev
import signal_tls
import migrations

CLAIMS = {'postgres': 'pgdata-voice-postgres-0', 'minio': 'data-voice-minio-0',
          'clickhouse': 'chdata-voice-clickhouse-0', 'nats': 'voice-nats-jsdata'}
APPS = frozenset('analytics bot chat file gameintegration gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice auth web developer-portal admin'.split())


def old_storage(namespace, pvcs, pvs, *, namespace_uid):
    """Capture retained storage; the fresh-storage route never deletes a PV."""
    dev.validate_target(namespace)
    if not namespace_uid or namespace['metadata'].get('uid') != namespace_uid:
        raise ValueError('DEV namespace identity changed')
    if len(pvcs) != 4 or {p['metadata']['name'] for p in pvcs} != set(CLAIMS.values()):
        raise ValueError('exact four old DEV claims required')
    by_name = {p['metadata']['name']: p for p in pvs}
    retained = []
    for claim in pvcs:
        meta = claim['metadata']
        pv = by_name.get(claim['spec'].get('volumeName'), {})
        spec = pv.get('spec', {})
        if (meta.get('namespace') != dev.NS or not meta.get('uid')
                or claim.get('status', {}).get('phase') != 'Bound'
                or not pv.get('metadata', {}).get('uid')
                or spec.get('persistentVolumeReclaimPolicy') != 'Retain'
                or spec.get('storageClassName') != 'voice-dev-local'
                or {k: spec.get('claimRef', {}).get(k) for k in ('namespace', 'name', 'uid')}
                != {'namespace': dev.NS, 'name': meta['name'], 'uid': meta['uid']}):
            raise ValueError('old DEV storage is not an exact retained binding')
        retained.append({'name': pv['metadata']['name'], 'uid': pv['metadata']['uid'],
                         'claim_uid': meta['uid'], 'spec_sha256': digest(spec)})
    return retained


def storage_class_pin(storage_class, *, available_bytes):
    if (storage_class.get('metadata', {}).get('name') != 'local-path'
            or not storage_class['metadata'].get('uid')
            or storage_class.get('provisioner') != 'rancher.io/local-path'
            or storage_class.get('volumeBindingMode') != 'WaitForFirstConsumer'
            or storage_class.get('reclaimPolicy') != 'Delete'
            or storage_class.get('parameters') not in (None, {})
            or available_bytes < 26 * 1024 ** 3):
        raise ValueError('approved local-path class and fresh-storage headroom required')
    return {'name': 'local-path', 'uid': storage_class['metadata']['uid'],
            'resourceVersion': storage_class['metadata']['resourceVersion']}


def verify_retained(pvs, retained):
    by_name = {p['metadata']['name']: p for p in pvs}
    for pin in retained:
        pv = by_name.get(pin['name'], {})
        if pv.get('metadata', {}).get('uid') != pin['uid'] or digest(pv.get('spec')) != pin['spec_sha256']:
            raise ValueError('old retained PV identity/spec changed')


def verify_fresh(namespace, pvcs, pvs, *, old_namespace_uid, retained):
    dev.validate_target(namespace)
    if not namespace['metadata'].get('uid') or namespace['metadata']['uid'] == old_namespace_uid:
        raise ValueError('new DEV namespace identity required')
    if len(pvcs) != 4 or {p['metadata']['name'] for p in pvcs} != set(CLAIMS.values()):
        raise ValueError('exact four fresh claims required')
    by_name = {p['metadata']['name']: p for p in pvs}
    forbidden_pvs = {p['uid'] for p in retained}
    forbidden_claims = {p['claim_uid'] for p in retained}
    verify_retained(pvs, retained)
    for claim in pvcs:
        meta, spec = claim['metadata'], claim['spec']
        pv = by_name.get(spec.get('volumeName'), {})
        if (meta.get('namespace') != dev.NS or not meta.get('uid') or meta['uid'] in forbidden_claims
                or claim.get('status', {}).get('phase') != 'Bound' or spec.get('storageClassName') != 'local-path'
                or any(spec.get(k) for k in ('selector', 'dataSource', 'dataSourceRef'))
                or not pv.get('metadata', {}).get('uid') or pv['metadata']['uid'] in forbidden_pvs
                or pv['metadata']['name'] != 'pvc-' + meta['uid']
                or pv.get('spec', {}).get('storageClassName') != 'local-path'
                or pv['spec'].get('persistentVolumeReclaimPolicy') != 'Delete'
                or pv['spec'].get('local', {}).get('path') != '/var/lib/rancher/k3s/storage/pvc-' + meta['uid'] + '_' + dev.NS + '_' + meta['name']
                or pv.get('spec', {}).get('claimRef', {}).get('uid') != meta['uid']
                or pv['spec']['claimRef'].get('namespace') != dev.NS
                or pv['spec']['claimRef'].get('name') != meta['name']):
            raise ValueError('fresh dynamic storage binding not established')


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def tls_identity(obj, private_directory, *, validate=signal_tls.validate_certificate):
    """Preserve compatible DEV endpoint identity; private payload never enters logs."""
    signal_tls.owned(obj)
    if (obj['metadata'].get('name') != signal_tls.SECRET or obj.get('type') != 'kubernetes.io/tls'
            or set(obj.get('data', {})) != {'tls.crt', 'tls.key'}):
        raise ValueError('exact existing DEV signaling TLS identity required')
    certificate = base64.b64decode(obj['data']['tls.crt'], validate=True)
    key = base64.b64decode(obj['data']['tls.key'], validate=True)
    with tempfile.TemporaryDirectory(dir=private_directory) as tmp:
        cert_path, key_path = Path(tmp) / 'certificate.pem', Path(tmp) / 'key.pem'
        for path, content in ((cert_path, certificate), (key_path, key)):
            descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(descriptor, 'wb') as stream:
                stream.write(content)
        validate(str(cert_path), str(key_path))
    return signal_tls.secret(certificate, key)


def restore_tls(kube, original, private_directory):
    """Create if absent; never roll back an independently renewed certificate."""
    preserved = tls_identity(original, private_directory)
    current = kube.get('secret', signal_tls.SECRET)
    if current is not None:
        return tls_identity(current, private_directory)
    kube.call('create', '-f', '-', body=preserved)
    return preserved


class Journal:
    """Private single-operation forward-recovery receipt, never an old root journal."""
    def __init__(self, directory, binding):
        self.directory = private_directory(Path(directory))
        self.binding = digest(binding)
        self.path = self.directory / 'reset-state.json'
        if self.path.is_symlink():
            raise ValueError('journal symlink forbidden')
        if self.path.exists():
            self.state = json.loads(self.path.read_text())
            if self.state['binding'] != self.binding:
                raise ValueError('operation/package binding changed')
        else:
            self.state = {'schema': 'voice-dev-reset-v1', 'binding': self.binding, 'steps': {}}
            self.save()

    def save(self):
        temporary = self.directory / 'reset-state.next'
        fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        try:
            with os.fdopen(fd, 'w', encoding='utf-8') as stream:
                json.dump(self.state, stream, sort_keys=True)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, self.path)
            if os.name == 'posix':
                directory = os.open(self.directory, os.O_RDONLY | os.O_DIRECTORY)
                try:
                    os.fsync(directory)
                finally:
                    os.close(directory)
        finally:
            if temporary.exists():
                temporary.unlink()

    def begin(self, step):
        if step not in ('stop-writers', 'delete-namespace', 'bootstrap', 'smoke'):
            raise ValueError('unknown DEV reset step')
        previous = self.state['steps'].get(step)
        if previous:
            raise ValueError('step already started; reconcile observed outcome before continuation')
        self.state['steps'][step] = {'status': 'started'}
        self.save()

    def complete(self, step, evidence):
        if self.state['steps'].get(step) != {'status': 'started'} or not evidence:
            raise ValueError('started step and observed proof required')
        # Store only a digest; private credential objects never enter this receipt.
        self.state['steps'][step] = {'status': 'complete', 'evidence_sha256': digest(evidence)}
        self.save()


def quiet_window(observed, original, *, now, remaining_seconds):
    """Reject destructive entry on drift or a due/queued DEV TLS renewal.

    Daily base midnight MSK is conservative: randomized next-trigger time is
    deliberately not used as the exclusion deadline.
    """
    if now.tzinfo is None or not isinstance(remaining_seconds, int) or remaining_seconds <= 0:
        raise ValueError('measured positive recovery budget required')
    stamp = datetime.fromisoformat(observed['observed_at'])
    if (stamp.tzinfo is None or abs((now - stamp).total_seconds()) > 15
            or observed['boot_id'] != original['boot_id']
            or observed['schedule'] != original['schedule']
            or observed['schedule'] != {'calendar': 'daily', 'randomized_seconds': 3600, 'persistent': True}
            or observed['timezone'] != 'Europe/Moscow'
            or observed['service_active'] != 'inactive' or observed['main_pid'] != 0
            or observed['timer_substate'] != 'waiting' or observed['pending_jobs']
            or observed['dev_updaters'] or observed['catch_up_due']):
        raise ValueError('DEV renewal/writer quiet interval not established')
    # Compare wall and monotonic progress against the pinned host boot.
    elapsed = (stamp - datetime.fromisoformat(original['observed_at'])).total_seconds()
    if abs(elapsed - (observed['monotonic_seconds'] - original['monotonic_seconds'])) > 2:
        raise ValueError('host clock changed')
    local = now.astimezone(timezone(timedelta(hours=3)))
    deadline = (local + timedelta(days=1)).replace(hour=0, minute=0, second=0, microsecond=0)
    if now + timedelta(seconds=remaining_seconds) >= deadline:
        raise ValueError('insufficient pre-renewal recovery budget')
    return deadline.isoformat()


def observe_host():
    """Read only the concrete installed DEV timer/process control plane."""
    def command(*args):
        return subprocess.check_output(args, env=dict(os.environ, LC_ALL='C'), timeout=15).decode()
    def properties(unit, names):
        raw = command('systemctl', 'show', unit, '--property=' + ','.join(names))
        return dict(line.split('=', 1) for line in raw.splitlines() if '=' in line)
    timer = properties('voice-dev-signaling-tls-renew.timer',
                       ('ActiveState', 'SubState', 'UnitFileState', 'LastTriggerUSec', 'TimersCalendar', 'Persistent', 'RandomizedDelayUSec'))
    service = properties('voice-dev-signaling-tls-renew.service', ('ActiveState', 'MainPID', 'Result'))
    now = datetime.now(timezone.utc)
    local = now.astimezone(timezone(timedelta(hours=3)))
    last = re.fullmatch(r'\w+ (\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) MSK', timer.get('LastTriggerUSec', ''))
    catch_up = not last or last[1] != local.date().isoformat()
    if timer.get('ActiveState') != 'active' or timer.get('UnitFileState') != 'enabled' or service.get('Result') != 'success':
        catch_up = True
    schedule = {'calendar': 'daily' if timer.get('TimersCalendar', '').startswith('{ OnCalendar=*-*-* 00:00:00 ;') else 'unknown',
                'randomized_seconds': 3600 if timer.get('RandomizedDelayUSec') == '1h' else -1,
                'persistent': timer.get('Persistent') == 'yes'}
    pending = [line for line in command('systemctl', 'list-jobs', '--no-legend', '--plain').splitlines()
               if 'voice-dev-signaling-tls-renew.' in line]
    updaters = []
    for line in command('ps', '-eo', 'pid=,args=').splitlines():
        if '/home/voice-dev/operator/' in line or ('update_images.py' in line and 'voice-dev' in line):
            updaters.append(line.strip().split()[0])  # Record PIDs only, never private argv.
    return {'observed_at': now.isoformat(), 'boot_id': Path('/proc/sys/kernel/random/boot_id').read_text().strip(),
            'monotonic_seconds': float(Path('/proc/uptime').read_text().split()[0]),
            'timezone': command('timedatectl', 'show', '--property=Timezone', '--value').strip(),
            'service_active': service.get('ActiveState'), 'main_pid': int(service.get('MainPID', '-1')),
            'timer_substate': timer.get('SubState'), 'pending_jobs': pending, 'dev_updaters': updaters,
            'catch_up_due': catch_up, 'schedule': schedule}


class Kube:
    """Explicit Kubernetes transport; never shell-interpolate resource names."""
    def __init__(self, *, prefix=('kubectl',), runner=subprocess.run):
        self.prefix, self.runner = list(prefix), runner

    def command(self, args, body=None):
        result = self.runner(self.prefix + args, input=None if body is None else json.dumps(body).encode(),
                             capture_output=True, timeout=330, check=True)
        return json.loads(result.stdout) if result.stdout.strip() else None

    def get(self, kind, name):
        return self.command(['--namespace', dev.NS, 'get', kind, name, '--ignore-not-found', '-o', 'json'])

    def call(self, *args, body=None):
        args = list(args)
        if args and args[0] in ('create', 'replace', 'patch') and '-o' not in args:
            args += ['-o', 'json']
        return self.command(['--namespace', dev.NS, *args], body=body)

    def objects(self, kinds):
        return self.call('get', kinds, '-o', 'json')['items']

    def pause(self, obj):
        meta, kind = obj['metadata'], obj['kind']
        resource = {'Deployment': 'deployment', 'StatefulSet': 'statefulset',
                    'Job': 'job', 'CronJob': 'cronjob'}.get(kind)
        if not resource or meta.get('namespace') != dev.NS or not meta.get('uid') or not meta.get('resourceVersion'):
            raise ValueError('exact DEV writer identity required')
        field, value = ('replicas', 0) if kind in ('Deployment', 'StatefulSet') else ('suspend', True)
        patch = [{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
                 {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
                 {'op': 'add', 'path': '/spec/' + field, 'value': value}]
        return self.call('patch', resource, meta['name'], '--type=json', '-p', json.dumps(patch))

    def delete_old(self, kind, name, uid, revision):
        if kind == 'Namespace' and name == dev.NS:
            path = '/api/v1/namespaces/' + dev.NS
        else:
            raise ValueError('foreign deletion forbidden')
        if not uid or not revision:
            raise ValueError('exact deletion preconditions required')
        return self.command(['delete', '--raw', path, '-f', '-'], {
            'apiVersion': 'v1', 'kind': 'DeleteOptions',
            'preconditions': {'uid': uid, 'resourceVersion': revision}, 'propagationPolicy': 'Foreground'})


WRITERS = 'deployments,statefulsets,jobs,cronjobs'


def writers_paused(objects):
    for obj in objects:
        if obj['metadata'].get('namespace') != dev.NS:
            raise ValueError('foreign writer inventory')
        spec = obj.get('spec', {})
        if obj['kind'] in ('Deployment', 'StatefulSet'):
            if spec.get('replicas', 1) != 0:
                return False
        elif obj['kind'] in ('Job', 'CronJob'):
            if spec.get('suspend') is not True:
                return False
        else:
            raise ValueError('unknown DEV writer kind')
    return True


def active_pods(pods):
    active = []
    for pod in pods:
        if pod['metadata'].get('namespace') != dev.NS:
            raise ValueError('foreign Pod inventory')
        status = pod.get('status', {})
        containers = status.get('containerStatuses', []) + status.get('initContainerStatuses', [])
        terminated = (status.get('phase') in ('Succeeded', 'Failed') and containers
                      and all('terminated' in c.get('state', {}) for c in containers))
        if not terminated:
            active.append(pod)
    return active


def stop_writers(kube, expected, journal, *, check_window, timeout=300):
    """Fence known controllers with UID/RV patches and await actual Pod exit."""
    namespace = kube.get('namespace', dev.NS)
    dev.validate_target(namespace)
    if namespace['metadata']['uid'] != expected['namespace_uid']:
        raise ValueError('DEV namespace identity changed before stop')
    current = kube.objects(WRITERS)
    identity = lambda rows: sorted((r['kind'], r['metadata']['name'], r['metadata']['uid'], r['metadata']['resourceVersion']) for r in rows)
    resuming = journal.state['steps'].get('stop-writers', {}).get('status') == 'started'
    if resuming:
        expected_ids = {row[:3] for row in identity(expected['writers'])}
        actual_ids = {row[:3] for row in identity(current)}
        if actual_ids != expected_ids:
            raise ValueError('DEV writer identity changed during interrupted stop')
    elif identity(current) != identity(expected['writers']):
        raise ValueError('DEV writer inventory changed; prepare again')
    # Autoscalers and extension controllers require an explicit known stop route.
    if kube.objects('hpa'):
        raise ValueError('active autoscaler requires explicit existing stop route')
    check_window()
    if not resuming:
        journal.begin('stop-writers')
    for obj in current:
        if not writers_paused([obj]):
            kube.pause(obj)
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if not writers_paused(kube.objects(WRITERS)):
            raise ValueError('DEV writer resumed during stop')
        if not active_pods(kube.objects('pods')):
            journal.complete('stop-writers', {'namespace_uid': expected['namespace_uid'], 'pods_absent': True})
            return
        time.sleep(1)
    raise ValueError('DEV Pods remain; no force deletion permitted')


def delete_namespace(kube, expected, journal, *, check_window, timeout=600):
    """Delete only the pinned namespace; leave all old Retain PVs untouched."""
    if journal.state['steps'].get('stop-writers', {}).get('status') != 'complete':
        raise ValueError('completed writer stop required')
    current = kube.get('namespace', dev.NS)
    resuming = journal.state['steps'].get('delete-namespace', {}).get('status') == 'started'
    if current is None:
        if not resuming:
            raise ValueError('old namespace disappeared before this operation delete')
        verify_retained(kube.command(['get', 'pv', '-o', 'json'])['items'], expected['retained'])
        journal.complete('delete-namespace', {'old_namespace_uid': expected['namespace_uid'], 'absent': True})
        return
    dev.validate_target(current)
    if current['metadata']['uid'] != expected['namespace_uid']:
        raise ValueError('DEV namespace changed before delete')
    if active_pods(kube.objects('pods')) or not writers_paused(kube.objects(WRITERS)) or kube.objects('hpa'):
        raise ValueError('DEV writers reappeared before delete')
    verify_retained(kube.command(['get', 'pv', '-o', 'json'])['items'], expected['retained'])
    if not current['metadata'].get('deletionTimestamp'):
        check_window()
        if not resuming:
            journal.begin('delete-namespace')
        # A retry still targets the same observed old UID; never a recreated name.
        kube.delete_old('Namespace', dev.NS, current['metadata']['uid'], current['metadata']['resourceVersion'])
    elif not resuming:
        raise ValueError('namespace deletion was started outside this operation')
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        observed = kube.get('namespace', dev.NS)
        if observed is None:
            verify_retained(kube.command(['get', 'pv', '-o', 'json'])['items'], expected['retained'])
            journal.complete('delete-namespace', {'old_namespace_uid': expected['namespace_uid'], 'absent': True})
            return
        if observed['metadata']['uid'] != expected['namespace_uid']:
            raise ValueError('unexpected namespace recreation; do not adopt')
        time.sleep(1)
    raise ValueError('DEV deletion incomplete; do not strip finalizers')


PACKAGE_FILES = ('prepared.json', 'secrets.json', 'foundation.json', 'package-receipt.json', 'source-revision')


def adapter_binding():
    directory = Path(__file__).resolve().parent
    return {name: hashlib.sha256((directory / name).read_bytes()).hexdigest()
            for name in ('reset_bootstrap.py', 'bootstrap.py', 'reset_package.py', 'migrations.py', 'environment.py', 'signal_tls.py')}


def private_directory(path):
    if path.is_symlink() or not path.is_dir() or path.resolve() != path.absolute():
        raise ValueError('canonical existing private operation directory required')
    if os.name == 'posix' and (path.stat().st_uid != os.getuid() or path.stat().st_mode & 0o077):
        raise ValueError('operation directory must be caller-owned mode0700')
    return path


def private_read(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 4 << 20:
        raise ValueError('bounded private package file required')
    if os.name == 'posix' and (path.stat().st_uid != os.getuid() or path.stat().st_mode & 0o077):
        raise ValueError('caller-owned mode0600 package file required')
    return path.read_bytes()


def write_private(path, value):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w', encoding='utf-8') as stream:
        json.dump(value, stream, sort_keys=True)
        stream.flush()
        os.fsync(stream.fileno())


@contextmanager
def operator_lock(directory):
    """One live process per private reset package; a crash releases the OS lock."""
    path = directory / 'operator.lock'
    if path.is_symlink():
        raise ValueError('operation lock symlink forbidden')
    descriptor = os.open(path, os.O_RDWR | os.O_CREAT | getattr(os, 'O_NOFOLLOW', 0), 0o600)
    with os.fdopen(descriptor, 'a+b') as stream:
        if os.name == 'posix':
            import fcntl
            fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
        else:
            import msvcrt
            stream.seek(0)
            if not stream.read(1):
                stream.write(b'0')
                stream.flush()
            stream.seek(0)
            msvcrt.locking(stream.fileno(), msvcrt.LK_NBLCK, 1)
        try:
            yield
        finally:
            if os.name != 'posix':
                stream.seek(0)
                msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)


def validate_package(directory, root):
    raw = {name: private_read(directory / name) for name in PACKAGE_FILES}
    receipt = json.loads(raw['package-receipt.json'])
    revision = raw['source-revision'].decode().strip()
    if (not re.fullmatch(r'[a-f0-9]{40}', revision) or receipt.get('schema') != 'voice-dev-reset-package-v1'
            or receipt.get('source_sha') != revision or receipt.get('storage_class') != 'local-path'
            or receipt.get('storage_mode') != 'fresh-dynamic-no-retained-pv-binding'):
        raise ValueError('source-bound fresh package required')
    if receipt.get('node') != 'pmdebook':
        raise ValueError('approved DEV node placement required')
    source_paths = {source['path'] for source in receipt['sources']}
    if not {'deploy/staging/services.yaml', 'deploy/staging/infra.yaml', 'deploy/staging/configmap-app.yaml'} <= source_paths:
        raise ValueError('complete canonical source manifest required')
    for source in receipt['sources']:
        path = root / source['path']
        if (not path.resolve().is_relative_to(root.resolve()) or path.is_symlink()
                or hashlib.sha256(path.read_bytes()).hexdigest() != source['sha256']):
            raise ValueError('canonical package source changed')
    objects = json.loads(raw['prepared.json'])['items']
    secrets = json.loads(raw['secrets.json'])['items']
    foundation = json.loads(raw['foundation.json'])['items']
    if (len([o for o in foundation if o['kind'] == 'Namespace']) != 1
            or any(o['kind'] not in ('Namespace', 'LimitRange', 'ResourceQuota', 'NetworkPolicy') for o in foundation)):
        raise ValueError('fresh dynamic foundation required')
    dev.validate_target(next(o for o in foundation if o['kind'] == 'Namespace'))
    identities = {(o['kind'], o['metadata']['name']) for o in objects}
    required = {('Deployment', 'voice-' + name) for name in APPS | {'redis', 'nats-pvc-candidate', 'livekit', 'clamav', 'p3-principal-jwks'}}
    required |= {('StatefulSet', 'voice-' + name) for name in ('postgres', 'minio', 'clickhouse')}
    required |= {('Job', 'voice-nats-' + role + '-bootstrap') for role in ('realtime', 'notification', 'analytics-chat', 'search')}
    required |= {('Job', 'voice-minio-create-' + bucket + '-bucket') for bucket in ('avatars', 'files')}
    required |= {('ConfigMap', 'voice-app-config'), ('PersistentVolumeClaim', 'voice-nats-jsdata')}
    if not required <= identities or len(identities) != len(objects):
        raise ValueError('complete unique canonical DEV startup inventory required')
    for obj in objects:
        if obj['kind'] not in dev.ALLOWED | {'NetworkPolicy'}:
            raise ValueError('unexpected fresh package kind')
        if obj['kind'] in ('Deployment', 'StatefulSet', 'Job'):
            pod = obj.get('spec', {}).get('template', {}).get('spec', {})
            if pod.get('nodeSelector', {}).get('kubernetes.io/hostname') != receipt['node']:
                raise ValueError('package workload placement differs from approved node')
            if (not pod.get('containers') or any(pod.get(k) for k in ('hostNetwork', 'hostPID', 'hostIPC', 'nodeName'))
                    or any('hostPath' in v for v in pod.get('volumes', []))
                    or any(c.get('securityContext', {}).get('privileged') for c in pod.get('containers', []) + pod.get('initContainers', []))):
                raise ValueError('valid isolated fresh workload required')
    dev.validate_references(objects, secrets)
    if not any(s['metadata']['name'] == signal_tls.SECRET for s in secrets):
        raise ValueError('preserved DEV endpoint TLS required before reset')
    images = {row['reference'] for row in receipt['images']} | set(receipt['tool_images'].values())
    for obj in objects + secrets + [o for o in foundation if o['kind'] != 'Namespace']:
        if obj['metadata'].get('namespace') != dev.NS:
            raise ValueError('foreign package target')
        for part in dev.walk(obj):
            if isinstance(part.get('image'), str) and (part['image'] not in images
                    or not re.fullmatch(r'[A-Za-z0-9._/:\-]+@sha256:[a-f0-9]{64}', part['image'])):
                raise ValueError('package image is not a receipt-bound immutable image')
    tools = receipt['tool_images']
    for name in ('node:24-alpine', 'migrate/migrate:v4.18.1', 'flyway/flyway:11.3.0', 'space-native-installer'):
        if not re.fullmatch(r'[A-Za-z0-9._/:\-]+@sha256:[a-f0-9]{64}', tools.get(name, '')):
            raise ValueError('missing immutable bootstrap tool: ' + name)
    return receipt, {name: hashlib.sha256(value).hexdigest() for name, value in raw.items()}


def prepare_plan(kube, directory, root, *, namespace_uid, staging_uid, actor, claim, budget_seconds, budget_evidence):
    receipt, hashes = validate_package(directory, root)
    if not actor or not claim or budget_seconds < 1800 or len(budget_evidence.strip()) < 40:
        raise ValueError('actual operator claim and reviewed recovery budget evidence required')
    objects = [o for name in ('foundation.json', 'prepared.json', 'secrets.json')
               for o in json.loads(private_read(directory / name))['items']]
    kube.command(['create', '--dry-run=client', '--validate=strict', '-f', '-', '-o', 'json'],
                 body={'apiVersion': 'v1', 'kind': 'List', 'items': objects})
    namespace = kube.get('namespace', dev.NS)
    pvcs = kube.objects('pvc')
    pvs = kube.command(['get', 'pv', '-o', 'json'])['items']
    retained = old_storage(namespace, pvcs, pvs, namespace_uid=namespace_uid)
    staging = staging_storage(kube)
    if staging['namespace_uid'] != staging_uid:
        raise ValueError('STAGING namespace identity changed')
    sc = kube.command(['get', 'storageclass', 'local-path', '-o', 'json'])
    storage_pin = storage_class_pin(sc, available_bytes=shutil_disk_free())
    provider = provider_pin(kube)
    original = observe_host()
    quiet_window(original, original, now=datetime.now(timezone.utc), remaining_seconds=budget_seconds)
    existing_tls = kube.get('secret', signal_tls.SECRET)
    preserved = tls_identity(existing_tls, directory)
    bundled = next(o for o in json.loads(private_read(directory / 'secrets.json'))['items'] if o['metadata']['name'] == signal_tls.SECRET)
    if bundled.get('data') != preserved['data']:
        raise ValueError('DEV TLS identity changed since private package preparation')
    writers = [{'kind': o['kind'], 'metadata': {k: o['metadata'][k] for k in ('name', 'namespace', 'uid', 'resourceVersion')}}
               for o in kube.objects(WRITERS)]
    plan = {'schema': 'voice-dev-reset-plan-v1', 'operation': os.urandom(6).hex(), 'actor': actor,
            'resource_claim': claim, 'namespace_uid': namespace_uid, 'staging_uid': staging_uid,
            'writers': writers, 'retained': retained, 'storage_class': storage_pin,
            'provider': provider,
            'host': original, 'package_files': hashes, 'source_sha': receipt['source_sha'],
            'adapter_code': adapter_binding(),
            'staging_storage': staging,
            # Reserve observer/TERM/KILL time inside the reviewed window.
            'deadline': (datetime.now(timezone.utc) + timedelta(seconds=budget_seconds - 15)).isoformat(),
            'budget_seconds': budget_seconds, 'budget_evidence': budget_evidence}
    write_private(directory / 'reset-plan.json', plan)
    return plan


def shutil_disk_free():
    import shutil
    return shutil.disk_usage('/home').free


def provider_pin(kube):
    config = kube.command(['-n', 'kube-system', 'get', 'configmap', 'local-path-config', '-o', 'json'])
    deployment = kube.command(['-n', 'kube-system', 'get', 'deployment', 'local-path-provisioner', '-o', 'json'])
    node = kube.command(['get', 'node', 'pmdebook', '-o', 'json'])
    expected = {'nodePathMap': [{'node': 'DEFAULT_PATH_FOR_NON_LISTED_NODES', 'paths': ['/var/lib/rancher/k3s/storage']}]}
    if (json.loads(config.get('data', {}).get('config.json', '{}')) != expected
            or not deployment.get('status', {}).get('availableReplicas')
            or node.get('spec', {}).get('unschedulable')
            or not any(c.get('type') == 'Ready' and c.get('status') == 'True' for c in node.get('status', {}).get('conditions', []))):
        raise ValueError('approved ready DEV local-path provider/node required')
    return {'config_uid': config['metadata']['uid'], 'config_rv': config['metadata']['resourceVersion'],
            'config_sha256': digest(config['data']), 'deployment_uid': deployment['metadata']['uid'],
            'deployment_spec_sha256': digest(deployment['spec']), 'node_uid': node['metadata']['uid']}


def staging_storage(kube):
    """Metadata/storage bindings only; never request STAGING Secrets or data."""
    namespace = kube.command(['get', 'namespace', 'voice-staging', '-o', 'json'])
    claims = kube.command(['-n', 'voice-staging', 'get', 'pvc', '-o', 'json'])['items']
    volumes = kube.command(['get', 'pv', '-o', 'json'])['items']
    workloads = kube.command(['-n', 'voice-staging', 'get', 'deployment,statefulset', '-o', 'json'])['items']
    return {'namespace_uid': namespace['metadata']['uid'],
            'claims': sorted((p['metadata']['name'], p['metadata']['uid'], p['spec'].get('volumeName')) for p in claims),
            'volumes': sorted((p['metadata']['name'], p['metadata']['uid'], digest(p['spec'])) for p in volumes
                              if p.get('spec', {}).get('claimRef', {}).get('namespace') == 'voice-staging'),
            'workload_bindings': sorted((w['kind'], w['metadata']['name'], w['metadata']['uid'],
                digest({'claims': w.get('spec', {}).get('volumeClaimTemplates', []),
                        'volumes': w.get('spec', {}).get('template', {}).get('spec', {}).get('volumes', [])})) for w in workloads)}


def execute_plan(kube, directory, root, *, actor, claim):
    private_directory(directory)
    with operator_lock(directory):
        return execute_locked(kube, directory, root, actor=actor, claim=claim)


def submitted_work(kube):
    """Accepted server-side work survives client death; record exact identities."""
    namespace = kube.get('namespace', dev.NS)
    return {'namespace_uid': namespace['metadata']['uid'] if namespace else None,
            'workloads': sorted((o['kind'], o['metadata']['name'], o['metadata']['uid'], digest(o.get('spec', {})))
                                for o in kube.objects(WRITERS)) if namespace else []}


def run_bootstrap(args, *, env, timeout, check_window, log, interval=2, observer_timeout=5, grace=5):
    """Supervise only our local POSIX session; accepted K8s work stays unresolved."""
    if os.name != 'posix':
        raise ValueError('bootstrap supervision requires the actual Linux execution host')
    import signal
    import queue
    import threading
    deadline = time.monotonic() + timeout
    def observe():
        responses = queue.Queue()
        def check():
            try:
                check_window()
                responses.put(None)
            except BaseException as error:
                responses.put(error)
        threading.Thread(target=check, daemon=True).start()
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise ValueError('bootstrap operation deadline exhausted')
        try:
            error = responses.get(timeout=min(observer_timeout, remaining))
        except queue.Empty:
            raise ValueError('bootstrap observer timeout; outcome unresolved') from None
        if error:
            raise error
    observe()
    descriptor = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_APPEND | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'ab') as stream:
        process = subprocess.Popen(args, env=env, stdout=stream, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            while True:
                observe()
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise ValueError('bootstrap operation deadline exhausted; reconcile accepted Kubernetes work')
                try:
                    result = process.wait(timeout=min(interval, remaining))
                    observe()
                    return subprocess.CompletedProcess(args, result, b'', b'')
                except subprocess.TimeoutExpired:
                    continue
        finally:
            # Also clean descendants after a normal leader exit. Never signal
            # another session or equate local death with cancelled K8s work.
            for sig in (signal.SIGTERM, signal.SIGKILL):
                try:
                    os.killpg(process.pid, sig)
                except ProcessLookupError:
                    pass
                try:
                    process.wait(timeout=grace)
                except subprocess.TimeoutExpired:
                    continue
                if sig == signal.SIGTERM:
                    end = time.monotonic() + grace
                    while time.monotonic() < end:
                        try:
                            os.killpg(process.pid, 0)
                        except ProcessLookupError:
                            break
                        time.sleep(0.01)


def execute_locked(kube, directory, root, *, actor, claim):
    plan = json.loads(private_read(directory / 'reset-plan.json'))
    receipt, hashes = validate_package(directory, root)
    if (actor != plan['actor'] or claim != plan['resource_claim'] or hashes != plan['package_files']
            or adapter_binding() != plan['adapter_code']):
        raise ValueError('operator claim/package changed; no mutation')
    journal = Journal(directory, plan)
    def check_window():
        now = datetime.now(timezone.utc)
        remaining = int((datetime.fromisoformat(plan['deadline']) - now).total_seconds())
        if remaining <= 0:
            raise ValueError('original operation deadline exhausted; preserve interrupted state')
        quiet_window(observe_host(), plan['host'], now=now, remaining_seconds=remaining)
        if digest(staging_storage(kube)) != digest(plan['staging_storage']):
            raise ValueError('STAGING storage/identity changed')
        current_sc = kube.command(['get', 'storageclass', 'local-path', '-o', 'json'])
        if storage_class_pin(current_sc, available_bytes=shutil_disk_free()) != plan['storage_class']:
            raise ValueError('fresh storage provider changed')
        if provider_pin(kube) != plan['provider']:
            raise ValueError('fresh storage provisioner/node identity changed')
    runtime = directory / 'runtime'
    runtime_binding = {'operation': plan['operation'], 'package_files': plan['package_files']}
    if not runtime.exists():
        runtime.mkdir(mode=0o700)
        for name in PACKAGE_FILES:
            descriptor = os.open(runtime / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(descriptor, 'wb') as stream:
                stream.write(private_read(directory / name))
        write_private(runtime / 'package-binding.json', runtime_binding)
    else:
        private_directory(runtime)
        if json.loads(private_read(runtime / 'package-binding.json')) != runtime_binding:
            raise ValueError('interrupted runtime package binding changed')
    dynamic = runtime / 'fresh-storage.json'
    dynamic_binding = {'schema': 'voice-dev-fresh-storage-v1', 'operation': plan['operation'],
        'old_namespace_uid': plan['namespace_uid'], 'retained': plan['retained'],
        'host': plan['host'], 'deadline': plan['deadline'],
        'storage_class': plan['storage_class'], 'tool_images': receipt['tool_images'],
        'image_references': sorted({r['reference'] for r in receipt['images']} | set(receipt['tool_images'].values())
                                   | set(migrations.GAMEINTEGRATION_IMAGES.values()))}
    if not dynamic.exists():
        write_private(dynamic, dynamic_binding)
    else:
        current_binding = json.loads(private_read(dynamic))
        if ({k: current_binding.get(k) for k in dynamic_binding} != dynamic_binding
                or set(current_binding) - set(dynamic_binding) - {'new_namespace_uid'}):
            raise ValueError('interrupted runtime authority/image binding changed')
    if journal.state['steps'].get('stop-writers', {}).get('status') != 'complete':
        stop_writers(kube, plan, journal, check_window=check_window)
    if journal.state['steps'].get('delete-namespace', {}).get('status') != 'complete':
        delete_namespace(kube, plan, journal, check_window=check_window)
    if journal.state['steps'].get('delete-namespace', {}).get('status') != 'complete':
        raise ValueError('interrupted namespace deletion needs observed reconciliation')
    if journal.state['steps'].get('bootstrap', {}).get('status') != 'complete':
        check_window()
        interruptions = sorted(runtime.glob('bootstrap-interrupted-*.json'))
        if interruptions:
            previous = json.loads(private_read(interruptions[-1]))
            if not previous.get('observed') or previous['work'] != json.loads(json.dumps(submitted_work(kube))):
                raise ValueError('accepted Kubernetes work changed or remains unknown; exact operator reconciliation required')
        if not journal.state['steps'].get('bootstrap'):
            journal.begin('bootstrap')
        # Only forward bootstrap may resume; old namespace deletion never replays.
        environment = dict(os.environ, SPACE_NATIVE_INSTALLER_IMAGE=receipt['tool_images']['space-native-installer'])
        log = runtime / 'bootstrap.log'
        try:
            result = run_bootstrap([sys.executable, str(Path(__file__).with_name('bootstrap.py')), 'start', '--work-dir', str(runtime), '--source-root', str(root)],
                                   env=environment, timeout=max(1, int((datetime.fromisoformat(plan['deadline']) - datetime.now(timezone.utc)).total_seconds())),
                                   check_window=check_window, log=log)
            if result.returncode:
                raise ValueError('forward bootstrap incomplete; inspect private bootstrap.log, preserve state')
        except BaseException:
            try:
                observation = {'observed': True, 'work': submitted_work(kube), 'accepted_work_cancelled': False}
            except Exception:
                observation = {'observed': False, 'accepted_work_cancelled': False}
            write_private(runtime / ('bootstrap-interrupted-%06d.json' % len(interruptions)), observation)
            raise
        journal.complete('bootstrap', {'namespace_uid': kube.get('namespace', dev.NS)['metadata']['uid']})
    if digest(staging_storage(kube)) != digest(plan['staging_storage']):
        raise ValueError('STAGING storage/identity changed during DEV operation')
    return journal.state


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('plan', 'run'))
    parser.add_argument('--work-dir', required=True, type=Path)
    parser.add_argument('--source-root', required=True, type=Path)
    parser.add_argument('--actor', required=True)
    parser.add_argument('--resource-claim', required=True)
    parser.add_argument('--namespace-uid')
    parser.add_argument('--staging-uid')
    parser.add_argument('--budget-seconds', type=int, default=0)
    parser.add_argument('--budget-evidence', default='')
    args = parser.parse_args()
    os.umask(0o077)
    directory = private_directory(args.work_dir.absolute())
    root = args.source_root.resolve(strict=True)
    if args.action == 'plan':
        if (directory / 'reset-state.json').exists():
            raise ValueError('existing operation state must never be reset')
        prepare_plan(Kube(), directory, root, namespace_uid=args.namespace_uid, staging_uid=args.staging_uid,
                     actor=args.actor, claim=args.resource_claim, budget_seconds=args.budget_seconds, budget_evidence=args.budget_evidence)
        print('DEV_RESET_PLAN_READY; no cluster mutation')
    else:
        execute_plan(Kube(), directory, root, actor=args.actor, claim=args.resource_claim)
        print('DEV_BOOTSTRAP_READY_FOR_APPLICATION_SMOKE; acceptance is still required')


if __name__ == '__main__':
    main()
