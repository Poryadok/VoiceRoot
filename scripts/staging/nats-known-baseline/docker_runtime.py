"""Owned, network-none broker execution. Never attaches staging or its PVC.

Inputs and executables must already be captured by the root controller. This
adapter accepts no arbitrary image, address, Docker endpoint, or container.
"""
import json
import os
from pathlib import Path
import re
import subprocess
import stat
import tempfile
import time

from controller import Blocked, archive_closed_store, restore_closed_store
from commands import capture

NATS_IMAGE = 'nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb'
BOX_IMAGE = 'natsio/nats-box@sha256:abdc9f9f0120bb8adfbf674eb037d1551db55356eb198b7bd4ffed377f6950a6'
LABEL = 'voice.known-nats.operation'


class DockerRuntime:
    def __init__(self, base, operation, run=None):
        if not re.fullmatch(r'[a-z0-9]{8,32}', operation):
            raise Blocked('operation_identity_invalid')
        self.base = Path(base).resolve(strict=True)
        self.operation = operation
        self.run = run or self._run
        self.owned = {}
        self.new_stores = set()

    def allow_new_store(self, path):
        path=Path(path)
        if not re.fullmatch(r'/var/lib/rancher/k3s/storage/pvc-[a-f0-9-]{36}_voice-staging_voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}',str(path)) or path.resolve(strict=True)!=path or any(path.iterdir()):
            raise Blocked('new_store_identity_invalid')
        # Called only after Staging.new_claim bound PVC/PV UID and proved zero
        # Pod mounts. Existing identity-derived stores cannot match this path.
        os.chmod(path,0o700); os.chown(path,65532,65532)
        self.new_stores.add(str(path))

    def allow_existing_store(self, path):
        # Caller has revalidated the exact recorded PVC/PV identity, physical
        # fence and paused native inventory. Never harden/reset existing data.
        path=Path(path)
        if not re.fullmatch(r'/var/lib/rancher/k3s/storage/pvc-[a-f0-9-]{36}_voice-staging_voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}',str(path)) or path.resolve(strict=True)!=path:
            raise Blocked('existing_store_identity_invalid')
        s=path.lstat()
        if not stat.S_ISDIR(s.st_mode) or s.st_uid!=65532 or s.st_mode&0o077:
            raise Blocked('existing_store_custody_invalid')
        self.new_stores.add(str(path))

    def no_operation_containers(self, *, running_only=False):
        args=['ps'] if running_only else ['ps','-a']
        if self.run([*args,'--filter','label='+LABEL+'='+self.operation,'--format','{{.ID}}']):
            raise Blocked('operation_container_writer_present')

    def allow_bound_store(self, path, claim, pv, verify_closed):
        """Enroll one already-verified selected PV, never an arbitrary mount.

        The rollout caller must have verified original native records/census
        before invoking this, and the supplied stage check revalidates the
        physical fence, captured UID/PV and trusted filesystem ancestors.
        Existing reset-store authorization remains independently unchanged.
        """
        verify_closed()
        path=Path(path)
        if path.resolve(strict=True)!=path:raise Blocked('selected_store_path_changed')
        if path.is_relative_to(self.base):return # private owned proof/fixture store
        from stage_runtime import pv_storage_path
        if pv_storage_path(pv,claim['metadata']['uid'],claim['metadata']['name'],pv['metadata']['uid'])!=str(path):
            raise Blocked('selected_store_binding_changed')
        row=path.lstat()
        if not stat.S_ISDIR(row.st_mode) or row.st_uid!=65532 or stat.S_IMODE(row.st_mode)!=0o700:
            raise Blocked('selected_store_custody_invalid')
        self.new_stores.add(str(path))

    @staticmethod
    def _run(args, timeout=60, limit=1<<20):
        output=capture(['/usr/bin/docker','--host=unix:///var/run/docker.sock',*args],
            timeout=timeout,limit=limit,env={'PATH':'/usr/bin:/bin','HOME':'/nonexistent'})
        return output.decode('utf-8',errors='strict').strip()

    def account_id(self):
        path=self.base/'inputs/account.public'
        fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
        try:
            row=os.fstat(fd)
            if not stat.S_ISREG(row.st_mode) or row.st_uid!=0 or row.st_mode&0o022 or row.st_nlink!=1 or row.st_size!=56:
                raise Blocked('rollout_account_custody_invalid')
            value=os.read(fd,57).decode('ascii')
            if not re.fullmatch(r'A[A-Z2-7]{55}',value):raise Blocked('rollout_account_identity_invalid')
            return value
        finally:os.close(fd)

    def monitor_jsz(self,broker):
        if not self.inspect(broker)['State']['Running']:raise Blocked('rollout_monitor_broker_not_running')
        raw=self._run(['exec',broker,'/bin/busybox','wget','-q','-O','-',
            'http://127.0.0.1:8222/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048'],
            timeout=30,limit=64<<20)
        self.inspect(broker)
        return json.loads(raw)

    def inspect(self, name):
        rows = json.loads(self.run(['inspect', name]))
        if len(rows) != 1:
            raise Blocked('container_identity_invalid')
        row = rows[0]
        expected = self.owned.get(name)
        host = row['HostConfig']; config = row['Config']
        if (not expected or row['Id'] != expected['id'] or
                config['Labels'].get(LABEL) != self.operation or
                config['Image'] != expected['image'] or
                row['Image'] != expected['image_id'] or
                config['User'] != '65532:65532' or
                host['NetworkMode'] != expected['network'] or
                host['Privileged'] or not host['ReadonlyRootfs'] or
                host.get('CapAdd') or host.get('CapDrop') != ['ALL'] or
                host.get('PortBindings') or
                'no-new-privileges:true' not in host.get('SecurityOpt', [])):
            raise Blocked('container_isolation_invalid')
        mounts = {(m['Source'], m['Destination'], bool(m['RW'])) for m in row['Mounts'] if m['Type'] == 'bind'}
        if mounts != expected['mounts'] or any(m['Type'] not in ('bind', 'tmpfs') for m in row['Mounts']):
            raise Blocked('container_mount_invalid')
        return row

    def create(self, suffix, image, mounts, command, broker=None):
        if image not in (NATS_IMAGE, BOX_IMAGE) or not re.fullmatch(r'[a-z0-9-]{1,32}', suffix):
            raise Blocked('container_spec_invalid')
        image_rows = json.loads(self.run(['image', 'inspect', image]))
        if len(image_rows) != 1 or image.split('@')[1] not in [d.split('@')[-1] for d in image_rows[0]['RepoDigests']]:
            raise Blocked('image_digest_invalid')
        name = 'voice-known-' + self.operation + '-' + suffix
        network = 'none'
        if broker is not None:
            if not self.inspect(broker)['State']['Running']:
                raise Blocked('broker_not_running')
            network = 'container:' + self.owned[broker]['id']
        args = ['create', '--name', name, '--label', LABEL+'='+self.operation,
                '--network', network, '--user', '65532:65532', '--read-only',
                '--env', 'HOME=/tmp', '--env', 'XDG_CACHE_HOME=/tmp/cache', '--workdir', '/tmp',
                '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges:true',
                '--memory', '256m', '--pids-limit', '64', '--tmpfs', '/tmp:rw,noexec,nosuid,size=16m']
        expected_mounts = set()
        for source, target, writable in mounts:
            source = Path(source).resolve(strict=True)
            if (not source.is_relative_to(self.base) and not (str(source) in self.new_stores and target=='/data')) or ',' in str(source) or target not in ('/data', '/inputs', '/out', '/kernel', '/server.conf', '/bootstrap.sh'):
                raise Blocked('container_mount_invalid')
            expected_mounts.add((str(source), target, writable))
            args += ['--mount', 'type=bind,source='+str(source)+',target='+target+('' if writable else ',readonly')]
        args += ['--entrypoint', command[0], image, *command[1:]]
        cid = self.run(args)
        if not re.fullmatch(r'[a-f0-9]{64}', cid):
            raise Blocked('container_identity_invalid')
        self.owned[name] = {'id': cid, 'image': image, 'image_id': image_rows[0]['Id'], 'network': network, 'mounts': expected_mounts}
        self.inspect(name)
        return name

    def start_broker(self, suffix, store):
        name = self.create(suffix, NATS_IMAGE,
            [(store, '/data', True), (self.base/'server.conf', '/server.conf', False)],
            ['/usr/local/bin/nats-server', '-c', '/server.conf'])
        self.run(['start', name]); self.inspect(name)
        return name

    def stop(self, name):
        self.inspect(name)
        self.run(['stop', '--time', '10', name], timeout=20)
        if self.inspect(name)['State']['Running']:
            raise Blocked('broker_stop_failed')

    def restart(self, name):
        if self.inspect(name)['State']['Running']:
            raise Blocked('broker_not_closed')
        self.run(['start', name]); self.inspect(name)

    def kernel(self, broker, phase):
        if phase not in ('seed', 'verify-closed', 'drain', 'verify-drained', 'census'):
            raise Blocked('kernel_phase_invalid')
        # A reconstructed operation has an empty owned-container ledger but
        # retains previous proof files. Never reuse or overwrite those outputs.
        out = Path(tempfile.mkdtemp(prefix='out-', dir=self.base))
        os.chown(out, 65532, 65532)
        name = self.create('kernel-'+str(len(self.owned)), NATS_IMAGE,
            [(self.base/'kernel', '/kernel', False), (self.base/'inputs', '/inputs', False), (out, '/out', True)],
            ['/kernel', '--phase', phase], broker)
        self.run(['start', name])
        if self.run(['wait', name], timeout=60) != '0' or self.inspect(name)['State']['Running']:
            raise Blocked('kernel_failed')
        return out

    def bootstrap(self, broker):
        for role in ('realtime', 'notification', 'analytics-chat', 'search'):
            script = self.base / ('bootstrap-'+role+'.sh')
            name = self.create('bootstrap-'+role+'-'+str(len(self.owned)), BOX_IMAGE,
                [(script, '/bootstrap.sh', False), (self.base/'inputs', '/inputs', False)],
                ['/bin/sh', '-c', 'NATS_URL=nats://127.0.0.1:4222 NATS_CREDS=/inputs/bootstrap.creds exec /bin/sh /bootstrap.sh'], broker)
            self.run(['start', name])
            if self.run(['wait', name], timeout=120) != '0' or self.inspect(name)['State']['Running']:
                raise Blocked('bootstrap_failed')

    def cold_archive(self, broker, store, archive):
        # The exact owned broker must be stopped before any source byte read.
        if self.inspect(broker)['State']['Running']:
            raise Blocked('archive_broker_running')
        return archive_closed_store(store, archive)

    def restore(self, archive, target, manifest):
        if manifest.get('mechanism')=='closed-jetstream-store-tar-v2':
            from native_store import restore_closed_store as rollout_restore
            rollout_restore(archive,target,manifest)
        else:restore_closed_store(archive,target,manifest)
        for root, dirs, files in os.walk(target, followlinks=False):
            os.chown(root, 65532, 65532)
            for name in files:
                os.chown(Path(root)/name, 65532, 65532, follow_symlinks=False)
