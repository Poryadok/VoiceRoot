"""Own disposable Linux sandboxes only. No host ports/listeners or Docker socket mounts.

The host command is a trusted fixture supervisor; peers execute inside Linux.
This records raw capability observations, never a native/supervisor FenceReceipt.
"""
import json
import re
import subprocess
import sys
import time
import uuid
from pathlib import Path

IMAGE = 'python@sha256:1b668429b3511ab407d8e00648891631b0b1a4d7e15e3ca70f38ab5b91ad4ab4'
HERE = Path(__file__).resolve().parent


def docker(*args, timeout=25):
    result = subprocess.run(['rtk', 'proxy', 'docker', *map(str, args)], capture_output=True, text=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError(f'Docker {args[0]} failed: {result.stderr.strip()} {result.stdout.strip()}')
    return result.stdout.strip()


def inspection(cid):
    if not re.fullmatch('[a-f0-9]{64}', cid):
        raise RuntimeError('runtime UID required')
    result = json.loads(docker('inspect', cid))[0]
    if result['Id'] != cid:
        raise RuntimeError('runtime UID substitution')
    return result


PID_INSPECT = '''import json,os,sys,time
expected=json.loads(sys.argv[1]);out=[]
for entry in expected:
 p=entry['pid'];path='/proc/'+str(p)+'/stat'
 try:
  raw=open(path).read();tail=raw[raw.rfind(') ')+2:].split();start=tail[19]
  out.append({'pid':p,'start':start,'same_birth':entry.get('start',start)==start})
 except FileNotFoundError:out.append({'pid':p,'same_birth':False})
print(json.dumps({'boot':open('/proc/sys/kernel/random/boot_id').read().strip(),'observed_ns':time.monotonic_ns(),'processes':out}))'''


def pid_observation(processes):
    # Trusted, short-lived read-only observer. Reads ONLY exact owned PIDs from
    # Docker top, never enumerates/controls other host processes. cgroup v1 is
    # supported by checking actual PID births plus namespace-init termination.
    return json.loads(docker('run', '--rm', '--network', 'none', '--pid', 'host', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges', IMAGE, 'python', '-c', PID_INSPECT, json.dumps(processes)))


def state(cid):
    return json.loads(docker('exec', cid, 'cat', '/tmp/state.json'))


def wait_state(cid, predicate, seconds=8):
    deadline = time.monotonic() + seconds
    last = None
    while time.monotonic() < deadline:
        try:
            last = state(cid)
            if predicate(last):
                return last
        except RuntimeError:
            pass
        time.sleep(.1)
    raise RuntimeError(f'bounded peer observation failed: {last}')


def main(output):
    operation = str(uuid.uuid4())
    prefix = 'rudeus260-fence-' + operation[:8]
    created, networks = [], []
    report = {'operation': operation, 'kind': 'RAW_PRIVATE_COMPONENT_OBSERVATION', 'image': IMAGE,
              'profile': 'responsive trusted Docker/Linux host and forwarder; paused worker parent; descendant may continue; whole-host faults excluded',
              'not_qualified': ['LiveKit media/ICE/TURN', 'native authority journal', 'durable supervisor recovery', 'production topology', 'exact participant eviction']}
    output = output.resolve()

    def save():
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(json.dumps(report, indent=2))

    def mutate(action, cid, *options):
        if cid not in created:
            raise RuntimeError('foreign runtime mutation refused')
        inspection(cid)
        return docker(action, *options, cid)

    def new_network(suffix):
        nid = docker('network', 'create', '--internal', prefix + suffix)
        value = json.loads(docker('network', 'inspect', nid))[0]
        if value['Id'] != nid or not value['Internal']:
            raise RuntimeError('private network mismatch')
        networks.append(nid)
        save()
        return nid

    def create(suffix, network, *command):
        cid = docker('create', '--name', prefix + suffix, '--network', network, '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges', '--pids-limit', '32', '--memory', '128m', '--restart', 'no', IMAGE, 'python', '/fixture.py', *command)
        created.append(cid)
        report.setdefault('births', []).append({'runtime_uid': cid, 'role': suffix, 'created': inspection(cid)['Created']})
        save()  # Preserve exact returned runtime UID before starting it.
        docker('cp', HERE / 'peer.py', cid + ':/fixture.py')
        return cid

    try:
        info = json.loads(docker('info', '--format', '{{json .}}'))
        if info['OSType'] != 'linux':
            raise RuntimeError('native Linux daemon required')
        report['host_profile'] = {k: info.get(k) for k in ('ID', 'OSType', 'KernelVersion', 'CgroupVersion', 'Driver')}
        save()
        worker_net, client_net = new_network('-worker'), new_network('-client')
        worker = create('-worker', worker_net, 'worker')
        mutate('start', worker)
        wait_state(worker, lambda value: value.get('child', 0) > 1)
        worker_ip = inspection(worker)['NetworkSettings']['Networks'][prefix + '-worker']['IPAddress']
        edge = create('-edge', worker_net, 'relay', worker_ip)
        docker('network', 'connect', client_net, edge)
        mutate('start', edge)
        edge_ip = inspection(edge)['NetworkSettings']['Networks'][prefix + '-client']['IPAddress']
        nonce = uuid.uuid4().hex
        client = create('-client', client_net, 'client', edge_ip, worker_ip, nonce)
        mutate('start', client)
        before = wait_state(client, lambda value: value.get('received', 0) >= 10)
        if not all(before.get(k) for k in ('tcp_control', 'direct_tcp_denied', 'direct_udp_denied')):
            raise RuntimeError('mandatory attachment controls failed')
        docker('exec', worker, 'python', '-c', 'from pathlib import Path;import sys;sys.exit(1 if Path("/tmp/direct-bypass").exists() else 0)')

        rows = docker('top', worker, '-eo', 'pid,ppid,comm').splitlines()[1:]
        pids = [{'pid': int(row.split()[0])} for row in rows]
        if len(pids) < 2 or inspection(worker)['State']['Pid'] not in [x['pid'] for x in pids]:
            raise RuntimeError('parent and descendant census required')
        births = pid_observation(pids)
        if not all(x.get('same_birth') for x in births['processes']):
            raise RuntimeError('owned process birth observation missing')

        # Stop only PID1 first: an independent process-group child still emits.
        mutate('kill', worker, '--signal', 'STOP')
        continued = wait_state(client, lambda value: value.get('received', 0) >= before['received'] + 10)
        report['parent_pause_child_continued'] = continued['received'] - before['received']
        report['before'] = before
        report['owned_process_births'] = births
        save()

        fence_started = time.monotonic_ns()
        mutate('stop', edge, '--time', '1')
        for nid in (worker_net, client_net):
            docker('network', 'disconnect', '--force', nid, edge)
        endpoint_stopped = time.monotonic_ns()
        mutate('stop', worker, '--time', '1')
        docker('network', 'disconnect', '--force', worker_net, worker)
        terminated = time.monotonic_ns()
        for cid in (edge, worker):
            value = inspection(cid)
            if value['State']['Running'] or value['State']['Pid'] != 0 or value['NetworkSettings']['Networks']:
                raise RuntimeError('runtime or forwarding attachment still exists')

        after_pids = pid_observation(births['processes'])
        if after_pids['boot'] != births['boot'] or any(x['same_birth'] for x in after_pids['processes']):
            raise RuntimeError('old parent/descendant incarnation still alive or host replaced')
        marker = after_pids['observed_ns']
        docker('exec', client, 'python', '-c', 'from pathlib import Path;import sys;Path("/tmp/fence.marker").write_text(sys.argv[1])', marker)
        time.sleep(1)
        after = state(client)
        if after['after_fence'] != 0 or after['last_received_ns'] >= marker:
            raise RuntimeError('new peer delivery after final owned fence')
        report.update({'component_fence_observed': True, 'after': after, 'owned_processes_after': after_pids,
                       'observed_control_ms': {'endpoint_close': (endpoint_stopped-fence_started)/1e6, 'worker_termination': (terminated-endpoint_stopped)/1e6},
                       'covered_transport': ['bounded fixture UDP stream', 'fixture TCP forwarding'], 'full_provider_qualification': False})
        save()
        print(json.dumps({'component_fence_observed': True, 'runtime_count': len(created), 'whole_host_fault_covered': False, 'full_provider_qualification': False}))
    except Exception as exc:
        report['failure'] = str(exc)
        report['peer_failure_logs'] = {}
        for cid in created:
            result = subprocess.run(['rtk', 'proxy', 'docker', 'logs', '--tail', '25', cid], capture_output=True, text=True, timeout=10)
            report['peer_failure_logs'][cid] = result.stdout + result.stderr
        save()
        raise
    finally:
        # Known immutable UID custody only; never discover/delete by labels.
        failures = []
        for cid in reversed(created):
            try:
                if inspection(cid)['State']['Running']:
                    mutate('stop', cid, '--time', '1')
                mutate('rm', cid)
            except Exception as exc:
                failures.append(str(exc))
        for nid in reversed(networks):
            try:
                docker('network', 'rm', nid)
            except Exception as exc:
                failures.append(str(exc))
        report['cleanup_failures'] = failures
        save()
        if failures:
            raise RuntimeError('own fixture cleanup incomplete: ' + '; '.join(failures))


if __name__ == '__main__':
    main(Path(sys.argv[1]))
