"""Fenced, identity-bound DEV PostgreSQL dump producer.

The supplied guard, drain and durable phase journal are authority adapters,
not data flags. This producer does not implement those adapters or admission.
"""
import json
import posixpath
import re
import subprocess
import tempfile

from story_media_backup import MEMBERS

BASE = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
        '--namespace', 'voice-dev', '--request-timeout=15s']
POD = 'voice-postgres-0'
PVC = 'pgdata-voice-postgres-0'


class CaptureError(Exception):
    pass


class CaptureUncertain(CaptureError):
    pass


class Producer:
    def __init__(self, expected, guard, drain, journal, runner=None):
        keys = {'operation', 'snapshot_id', 'namespace_uid', 'pod_uid', 'statefulset_uid',
                'pvc_uid', 'pv_name', 'pv_uid', 'container', 'container_id', 'restart_count',
                'mount_path', 'data_directory', 'pg_major'}
        strings = keys - {'restart_count', 'pg_major'}
        if (not isinstance(expected, dict) or set(expected) != keys
                or any(not isinstance(expected[k], str) or not expected[k] for k in strings)
                or type(expected['restart_count']) is not int or expected['restart_count'] < 0
                or type(expected['pg_major']) is not int or expected['pg_major'] < 1
                or expected['container'] != 'postgres'
                or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?', expected['pv_name'])
                or not callable(guard) or not callable(drain)
                or any(not callable(getattr(journal, name, None)) for name in ('begin', 'uncertain', 'complete'))):
            raise CaptureError('capture authority or identity missing')
        self.expected = dict(expected)
        self.guard = guard
        self.drain = drain
        self.journal = journal
        self.runner = runner or subprocess.run

    def _get(self, kind, name):
        result = self.runner(BASE + ['get', kind, name, '-o', 'json'],
                             stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=20)
        if result.returncode:
            raise CaptureError('capture identity unavailable')
        return json.loads(result.stdout)

    def _container_status(self, pod):
        statuses = [s for s in pod['status']['containerStatuses'] if s.get('name') == 'postgres']
        if (pod['metadata']['uid'] != self.expected['pod_uid'] or len(statuses) != 1
                or statuses[0].get('containerID') != self.expected['container_id']
                or statuses[0].get('restartCount') != self.expected['restart_count']
                or statuses[0].get('ready') is not True or 'running' not in statuses[0].get('state', {})):
            raise CaptureError('postgres container identity changed')

    def _physical_source(self):
        sql = ("SELECT json_build_object('data_directory',current_setting('data_directory'),"
               "'version',current_setting('server_version_num'),"
               "'mountinfo',pg_read_file('/proc/self/mountinfo'))")
        script = ('export PGPASSWORD="$POSTGRES_PASSWORD"; '
                  'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "$1"')
        result = self.runner(BASE + ['exec', POD, '-c', 'postgres', '--', 'sh', '-ec', script, '--', sql],
                             stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=20)
        if result.returncode or len(result.stdout) > 1024 * 1024:
            raise CaptureError('postgres physical identity unavailable')
        observed = json.loads(result.stdout)
        data_dir = observed['data_directory']
        mount_path = self.expected['mount_path']
        if (data_dir != self.expected['data_directory'] or not data_dir.startswith('/')
                or posixpath.normpath(data_dir) != data_dir or not mount_path.startswith('/')
                or posixpath.normpath(mount_path) != mount_path
                or int(observed['version']) // 10000 != self.expected['pg_major']):
            raise CaptureError('postgres physical identity changed')
        resolve = 'readlink -f -- "$1"; readlink -f -- "$2"; cat /proc/self/mountinfo'
        result = self.runner(BASE + ['exec', POD, '-c', 'postgres', '--', 'sh', '-ec', resolve, '--', data_dir, mount_path],
                             stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=20)
        if result.returncode or len(result.stdout) > 1024 * 1024:
            raise CaptureError('postgres physical identity unavailable')
        resolved_data, resolved_mount, mountinfo = result.stdout.decode().split('\n', 2)
        if (resolved_data != data_dir or resolved_mount != mount_path
                or mountinfo != observed['mountinfo'] or '\\' in mountinfo):
            raise CaptureError('postgres mount namespace changed')
        mountpoints = []
        for line in mountinfo.splitlines():
            fields = line.split(' - ', 1)[0].split()
            if len(fields) < 6 or not fields[4].startswith('/'):
                raise CaptureError('postgres mount information invalid')
            point = fields[4]
            if data_dir == point or data_dir.startswith(point.rstrip('/') + '/'):
                mountpoints.append(point)
        if (not mountpoints or max(mountpoints, key=len) != mount_path
                or mountpoints.count(mount_path) != 1):
            raise CaptureError('postgres data directory is outside the PVC mount')

    def _check(self):
        self.guard()
        self.drain()
        ns = self._get('namespace', 'voice-dev')
        pod = self._get('pod', POD)
        pvc = self._get('pvc', PVC)
        pv = self._get('pv', self.expected['pv_name'])
        exp = self.expected
        owner = pod['metadata'].get('ownerReferences', [])
        containers = [c for c in pod['spec']['containers'] if c.get('name') == 'postgres']
        volumes = [v['name'] for v in pod['spec']['volumes']
                   if v.get('persistentVolumeClaim', {}).get('claimName') == PVC]
        mounts = [m for m in containers[0].get('volumeMounts', [])
                  if m.get('name') in volumes and m.get('mountPath') == exp['mount_path']] if len(containers) == 1 else []
        if (ns['metadata']['uid'] != exp['namespace_uid']
                or pod['metadata']['uid'] != exp['pod_uid'] or pod['metadata']['namespace'] != 'voice-dev'
                or not any(o.get('uid') == exp['statefulset_uid'] and o.get('kind') == 'StatefulSet'
                           and o.get('controller') is True for o in owner)
                or len(volumes) != 1 or len(mounts) != 1
                or pvc['metadata']['uid'] != exp['pvc_uid'] or pvc['metadata']['namespace'] != 'voice-dev'
                or pvc['status']['phase'] != 'Bound' or pvc['spec']['volumeName'] != exp['pv_name']
                or pv['metadata']['uid'] != exp['pv_uid']
                or any(pv['spec']['claimRef'].get(k) != v for k, v in
                       {'uid': exp['pvc_uid'], 'namespace': 'voice-dev', 'name': PVC}.items())):
            raise CaptureError('capture identity changed')
        self._container_status(pod)
        self._physical_source()
        self._container_status(self._get('pod', POD))
        self.guard()
        self.drain()

    def __call__(self, member, output):
        if member not in MEMBERS:
            raise CaptureError('capture member outside scope')
        try:
            self._check()
        except Exception:
            raise CaptureError('capture precondition unavailable') from None
        try:
            # begin must guard, persist pending and fence before permitting exec.
            self.journal.begin(member)
            self._check()
            script = ('export PGPASSWORD="$POSTGRES_PASSWORD"; export PGAPPNAME=voice-media-backup; '
                      'export PGOPTIONS="-c statement_timeout=60000 -c lock_timeout=5000"; ')
            script += ('exec pg_dumpall -h 127.0.0.1 -U "$POSTGRES_USER" --roles-only --no-role-passwords'
                       if member == 'roles' else
                       'exec pg_dump -h 127.0.0.1 -U "$POSTGRES_USER" --format=custom --lock-wait-timeout=5s -d "$1"')
            command = BASE + ['exec', POD, '-c', 'postgres', '--', 'sh', '-ec', script, '--', member]
            # Diagnostics may contain private database details; never journal or print them.
            with tempfile.TemporaryFile() as stderr:
                result = self.runner(command, stdout=output, stderr=stderr, timeout=90)
            if result.returncode:
                raise CaptureUncertain('capture stream uncertain')
            self._check()
            self.journal.complete(member, {'operation_id': self.expected['operation'], 'member': member,
                'snapshot_id': self.expected['snapshot_id'], 'pod_uid': self.expected['pod_uid'],
                'pvc_uid': self.expected['pvc_uid'], 'pv_uid': self.expected['pv_uid'],
                'container_id': self.expected['container_id'], 'restart_count': self.expected['restart_count'],
                'exit': 0, 'scope': 'dump-terminal-only'})
        except BaseException:
            try:
                self.journal.uncertain(member)
            except BaseException:
                pass
            raise CaptureUncertain('capture uncertain; retain fence and reconcile') from None
