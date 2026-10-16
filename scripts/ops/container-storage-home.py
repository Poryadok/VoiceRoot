#!/usr/bin/env python3
"""Single-host, recorded storage relocation; requires root and planned downtime.

Run in the host mount namespace from a host systemd unit, never from a runtime
being migrated. Existing absolute paths and Kubernetes PV identities survive.
Every output and backup is root-only. No source data is deleted by migrate.
"""
import argparse
from contextlib import closing
import hashlib
import json
import os
from pathlib import Path
import shutil
import sqlite3
import subprocess
import time

BASE = Path('/home/container-data')
STATE = BASE / 'migration'
PATHS = (
    '/var/lib/rancher/k3s', '/var/lib/docker', '/var/lib/containerd',
    '/var/lib/kubelet', '/var/log/pods', '/var/lib/voice-nats-preservation',
)
TRACKER = '/var/lib/rancher/k3s/storage/pvc-adbbd51a-a407-496b-beb7-364bfefc26d5_chinchilla_tracker-data/tracker.db'


def run(*args, timeout=600, check=True, body=None):
    result = subprocess.run(args, input=body, capture_output=True, timeout=timeout)
    if check and result.returncode:
        # Output can contain secrets; save it privately instead of echoing it.
        (STATE / 'last-error.txt').write_bytes(result.stderr + result.stdout)
        raise RuntimeError('command failed: ' + args[0])
    return result


def kube(*args):
    return json.loads(run('/usr/local/bin/kubectl', *args, '-o', 'json').stdout)


def write_json(path, value):
    path.write_text(json.dumps(value, sort_keys=True, indent=2) + '\n')
    path.chmod(0o600)


def stage(name):
    (STATE / 'stage').write_text(name + '\n')
    print(name, flush=True)


def target(source):
    if source not in PATHS:
        raise ValueError('source outside migration allowlist')
    return BASE / 'active' / source.removeprefix('/var/')


def require_preserved_volumes(saved, live):
    def identities(items):
        return {(i['metadata'].get('namespace', ''), i['metadata']['name']):
                (i['metadata']['uid'], i.get('spec', {}).get('volumeName', '')) for i in items}
    original, current = identities(saved), identities(live)
    if any(current.get(key) != value for key, value in original.items()):
        raise RuntimeError('persistent volume identity changed')


def rollback_present(source):
    target(source)  # Validate the exact source allowlist before constructing paths.
    return (Path(source + '.before-home').is_dir() or
            (BASE / 'rollback' / source.removeprefix('/var/')).is_dir())


def unit_name(source):
    return run('/usr/bin/systemd-escape', '--path', '--suffix=mount', source).stdout.decode().strip()


def mounts_under(source):
    rows = json.loads(run('findmnt', '--json', '--list', '-o', 'TARGET').stdout)['filesystems']
    return [r['target'] for r in rows if r['target'] == source or r['target'].startswith(source + '/')]


def sqlite_facts(path):
    with closing(sqlite3.connect('file:' + str(path) + '?mode=ro', uri=True)) as db:
        integrity = db.execute('PRAGMA integrity_check').fetchall()
        if integrity != [('ok',)]:
            raise RuntimeError('tracker integrity check failed')
        names = [r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")]
        facts = {}
        for name in names:
            quoted = '"' + name.replace('"', '""') + '"'
            digest = hashlib.sha256()
            rows = sorted(repr(r) for r in db.execute('SELECT * FROM ' + quoted))
            for row in rows:
                digest.update(row.encode() + b'\n')
            facts[name] = {'rows': len(rows), 'sha256': digest.hexdigest()}
        return facts


def backup_tracker(name):
    backup = STATE / (name + '.sqlite')
    if backup.exists():
        raise RuntimeError('refusing to overwrite tracker backup')
    with closing(sqlite3.connect('file:' + TRACKER + '?mode=ro', uri=True)) as src:
        with closing(sqlite3.connect(backup)) as dst:
            src.backup(dst)
    backup.chmod(0o600)
    facts = sqlite_facts(backup)
    write_json(STATE / (name + '.json'), facts)
    return facts


def capture():
    if (STATE / 'before.json').exists():
        raise RuntimeError('capture already exists')
    if os.stat('/home').st_dev == os.stat('/var').st_dev:
        raise RuntimeError('expected separate /home and /var filesystems')
    if shutil.disk_usage('/home').free < 150 * 1024**3:
        raise RuntimeError('insufficient copy and rollback space')
    for source in PATHS:
        if Path(source).is_symlink() or target(source).exists() or Path(source + '.before-home').exists():
            raise RuntimeError('unexpected existing relocation')
        if (Path('/etc/systemd/system') / unit_name(source)).exists():
            raise RuntimeError('mount unit already exists')
    for service in ('k3s', 'docker', 'containerd'):
        if Path('/etc/systemd/system/' + service + '.service.d/container-storage-home.conf').exists():
            raise RuntimeError('service drop-in already exists')
    workloads = kube('get', 'deployment,statefulset,daemonset,cronjob', '-A')['items']
    jobs = kube('get', 'jobs', '-A')['items']
    if any(x.get('status', {}).get('active', 0) for x in jobs):
        raise RuntimeError('active Kubernetes jobs must finish first')
    before = {'workloads': workloads, 'pvs': kube('get', 'pv')['items'],
              'pvcs': kube('get', 'pvc', '-A')['items'],
              'docker': run('docker', 'ps', '--format', '{{.ID}}').stdout.decode().split(),
              'runner_active': run('systemctl', 'is-active', 'actions.runner.Poryadok-VoiceRoot.pmdebook.service', check=False).returncode == 0}
    write_json(STATE / 'before.json', before)
    facts = backup_tracker('tracker-online-backup')
    print('TRACKER_INTEGRITY=ok TABLES=' + str(len(facts)), flush=True)
    stage('CAPTURED')


def wait_no_running_pods():
    deadline = time.monotonic() + 300
    while time.monotonic() < deadline:
        pods = kube('get', 'pods', '-A')['items']
        if not any(p.get('status', {}).get('phase') in ('Running', 'Pending', 'Unknown') for p in pods):
            return
        time.sleep(3)
    raise RuntimeError('pods did not quiesce')


def scale(saved, replicas):
    for item in saved['workloads']:
        kind = item['kind']
        meta = item['metadata']
        ns, name = meta['namespace'], meta['name']
        if kind in ('Deployment', 'StatefulSet'):
            count = item['spec'].get('replicas', 1) if replicas is None else replicas
            run('kubectl', '-n', ns, 'scale', kind.lower() + '/' + name, '--replicas=' + str(count))
        elif kind == 'DaemonSet':
            selector = item['spec']['template']['spec'].get('nodeSelector', {}) if replicas is None else {'voice-storage-migration': 'paused'}
            run('kubectl', '-n', ns, 'patch', 'daemonset', name, '--type=json', '-p', json.dumps([
                {'op': 'add', 'path': '/spec/template/spec/nodeSelector', 'value': selector}]))
        elif kind == 'CronJob':
            suspend = item['spec'].get('suspend', False) if replicas is None else True
            run('kubectl', '-n', ns, 'patch', 'cronjob', name, '--type=merge', '-p', json.dumps({'spec': {'suspend': suspend}}))


def migrate():
    saved = json.loads((STATE / 'before.json').read_text())
    if (STATE / 'stage').read_text().strip() != 'CAPTURED':
        raise RuntimeError('migration requires captured state; use explicit recovery for partial execution')
    # The temporary Docker launcher may have exited since host capture.
    saved['docker'] = [identifier for identifier in saved['docker']
                       if run('docker', 'inspect', identifier, check=False).returncode == 0]
    write_json(STATE / 'before.json', saved)
    stage('QUIESCING')
    run('systemctl', 'stop', 'actions.runner.Poryadok-VoiceRoot.pmdebook.service')
    scale(saved, 0)
    wait_no_running_pods()
    backup_tracker('tracker-cold-before')
    if saved['docker']:
        run('docker', 'stop', '--time', '60', *saved['docker'])
    run('systemctl', 'stop', 'k3s.service', 'docker.socket', 'docker.service', 'containerd.service')
    time.sleep(3)
    for p in Path('/proc').iterdir():
        if p.name.isdigit():
            try:
                comm = (p / 'comm').read_text().strip()
                if comm in ('containerd', 'containerd-shim', 'dockerd', 'k3s-server', 'kubelet'):
                    raise RuntimeError('runtime process still alive: ' + comm)
            except FileNotFoundError:
                pass
    for source in PATHS:
        if mounts_under(source):
            raise RuntimeError('nested mounts remain: ' + source)
    stage('OFFLINE_COPY')
    for source in PATHS:
        destination = target(source)
        destination.mkdir(parents=True, mode=0o700)
        run('rsync', '-aHAXx', '--numeric-ids', source + '/', str(destination) + '/', timeout=3600)
        comparison = run('rsync', '-aHAXxnic', '--numeric-ids', source + '/', str(destination) + '/', timeout=3600)
        if comparison.stdout:
            (STATE / 'copy-difference.txt').write_bytes(comparison.stdout)
            raise RuntimeError('offline copy mismatch')
        print('VERIFIED_COPY=' + source, flush=True)
    stage('INSTALLING_BINDS')
    for source in PATHS:
        destination = target(source)
        name = unit_name(source)
        unit = ('[Unit]\nDescription=Container storage on home\nRequiresMountsFor=/home /var\n'
                'After=home.mount var.mount\nBefore=k3s.service docker.service containerd.service\n'
                '[Mount]\nWhat=' + str(destination) + '\nWhere=' + source + '\nType=none\nOptions=bind\n'
                '[Install]\nWantedBy=local-fs.target\n')
        path = Path('/etc/systemd/system') / name
        if path.exists():
            raise RuntimeError('mount unit already exists')
        path.write_text(unit)
        os.rename(source, source + '.before-home')
        Path(source).mkdir(mode=0o700)
    for service in ('k3s', 'docker', 'containerd'):
        directory = Path('/etc/systemd/system/' + service + '.service.d')
        directory.mkdir(exist_ok=True)
        (directory / 'container-storage-home.conf').write_text('[Unit]\nRequiresMountsFor=' + ' '.join(PATHS) + '\n')
    run('systemctl', 'daemon-reload')
    for source in PATHS:
        run('systemctl', 'enable', '--now', unit_name(source))
        if os.stat(source).st_dev != os.stat('/home').st_dev:
            raise RuntimeError('bind is not backed by home')
    stage('RESTARTING')
    run('systemctl', 'start', 'containerd.service', 'docker.socket', 'docker.service', 'k3s.service')
    for _ in range(60):
        if run('kubectl', 'get', 'nodes', check=False).returncode == 0:
            break
        time.sleep(2)
    else:
        raise RuntimeError('Kubernetes API did not recover')
    scale(saved, None)
    # Resume CI only after independent application and persistence verification.
    stage('AWAITING_VERIFICATION')


def recover_user_space():
    saved = json.loads((STATE / 'before.json').read_text())
    original = next(i for i in saved['workloads'] if i['kind'] == 'Deployment' and
                    i['metadata']['namespace'] == 'voice-staging' and i['metadata']['name'] == 'voice-user')
    containers = original['spec']['template']['spec']['containers']
    index = next(i for i, c in enumerate(containers) if c['name'] == 'user')
    run('kubectl', '-n', 'voice-staging', 'set', 'env', 'deployment/voice-user', 'SPACE_GRPC_ADDR=')
    try:
        for name in ('voice-user', 'voice-space'):
            run('kubectl', '-n', 'voice-staging', 'rollout', 'status', 'deployment/' + name, '--timeout=240s')
    finally:
        run('kubectl', '-n', 'voice-staging', 'patch', 'deployment', 'voice-user', '--type=json', '-p',
            json.dumps([{'op': 'add', 'path': '/spec/template/spec/containers/' + str(index) + '/env',
                         'value': containers[index].get('env', [])}]))
    run('kubectl', '-n', 'voice-staging', 'rollout', 'status', 'deployment/voice-user', '--timeout=240s')
    print('USER_SPACE_ORIGINAL_ENV_RESTORED', flush=True)


def verify():
    saved = json.loads((STATE / 'before.json').read_text())
    live_workloads = kube('get', 'deployment,statefulset,daemonset', '-A')['items']
    live_by_key = {(i['kind'], i['metadata']['namespace'], i['metadata']['name']): i for i in live_workloads}
    for item in saved['workloads']:
        if item['kind'] == 'CronJob':
            continue
        key = (item['kind'], item['metadata']['namespace'], item['metadata']['name'])
        current = live_by_key[key]
        status = current.get('status', {})
        expected = item['spec'].get('replicas', 1)
        ready = status.get('readyReplicas', 0)
        if item['kind'] == 'DaemonSet':
            expected = status.get('desiredNumberScheduled', 0)
            ready = status.get('numberReady', 0)
        if ready < expected or status.get('observedGeneration', 0) < current['metadata']['generation']:
            raise RuntimeError('workload not ready: ' + '/'.join(key))
    for resource, key in (('pv', 'pvs'), ('pvc', 'pvcs')):
        args = ('get', resource, '-A') if resource == 'pvc' else ('get', resource)
        live = kube(*args)['items']
        require_preserved_volumes(saved[key], live)
    expected = json.loads((STATE / 'tracker-cold-before.json').read_text())
    actual = sqlite_facts(TRACKER)
    write_json(STATE / 'tracker-after.json', actual)
    if actual != expected:
        raise RuntimeError('tracker content changed; inspect legitimate writes versus data loss')
    for source in PATHS:
        if os.stat(source).st_dev != os.stat('/home').st_dev or not rollback_present(source):
            raise RuntimeError('unexpected storage or missing rollback')
        row = json.loads(run('findmnt', '--json', '--mountpoint', source, '-o', 'TARGET,FSROOT').stdout)['filesystems'][0]
        expected_root = '/' + str(target(source)).removeprefix('/home/')
        if row['target'] != source or row['fsroot'] != expected_root:
            raise RuntimeError('unexpected bind source')
    print('PERSISTENT_IDENTITIES=unchanged TRACKER_INTEGRITY=ok CONTENT=unchanged', flush=True)
    if (STATE / 'stage').read_text().strip() != 'COMPLETE':
        stage('VERIFIED')
    if saved['runner_active']:
        run('systemctl', 'start', 'actions.runner.Poryadok-VoiceRoot.pmdebook.service')
    if saved['docker']:
        run('docker', 'start', *saved['docker'])


def retire():
    if (STATE / 'stage').read_text().strip() not in ('VERIFIED', 'RETIRING'):
        raise RuntimeError('retirement requires verified live services and data')
    stage('RETIRING')
    ledger = STATE / 'retired.json'
    completed = json.loads(ledger.read_text()) if ledger.exists() else []
    for source in PATHS:
        if source in completed:
            continue
        original = Path(source + '.before-home')
        backup = BASE / 'rollback' / source.removeprefix('/var/')
        if not original.is_dir() or original.is_symlink() or mounts_under(str(original)):
            raise RuntimeError('unexpected original rollback path')
        if os.stat(original).st_dev != os.stat('/var').st_dev or os.stat(source).st_dev != os.stat('/home').st_dev:
            raise RuntimeError('unexpected original or live filesystem')
        backup.mkdir(mode=0o700, parents=True, exist_ok=True)
        run('rsync', '-aHAXx', '--numeric-ids', str(original) + '/', str(backup) + '/', timeout=3600)
        difference = run('rsync', '-aHAXxnic', '--numeric-ids', str(original) + '/', str(backup) + '/', timeout=3600)
        if difference.stdout:
            (STATE / 'rollback-difference.txt').write_bytes(difference.stdout)
            raise RuntimeError('rollback copy does not match original')
        # All content remains recoverable under /home; remove only the exact
        # allowlisted offline duplicate on /var after its checksum comparison.
        shutil.rmtree(original)
        completed.append(source)
        write_json(ledger, completed)
        print('ROLLBACK_RETAINED_ON_HOME=' + source, flush=True)
    stage('COMPLETE')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('capture', 'migrate', 'recover-user-space', 'verify', 'retire'))
    args = parser.parse_args()
    if os.geteuid() != 0 or os.readlink('/proc/self/ns/mnt') != os.readlink('/proc/1/ns/mnt'):
        raise SystemExit('root in host mount namespace required')
    os.umask(0o077)
    STATE.mkdir(parents=True, mode=0o700, exist_ok=True)
    try:
        globals()[args.action.replace('-', '_')]()
    except Exception as exc:
        (STATE / 'failure.txt').write_text(type(exc).__name__ + ': ' + str(exc) + '\n')
        print('FAILED; services remain in recorded state. Inspect root-only migration/failure.txt.', flush=True)
        raise SystemExit(1)


if __name__ == '__main__':
    main()
