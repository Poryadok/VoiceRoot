"""Owned, network-none broker execution. Never attaches staging or its PVC.

Inputs and executables must already be captured by the root controller. This
adapter accepts no arbitrary image, address, Docker endpoint, or container.
"""
import json
import copy
import hashlib
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
        self.selected_stores = {}

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

    def allow_bound_store(self, path, claim, pv, verify_closed, *, descriptor=None):
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
        from stage_runtime import pv_storage_path, verify_selected_store_leaf
        if pv_storage_path(pv,claim['metadata']['uid'],claim['metadata']['name'],pv['metadata']['uid'])!=str(path):
            raise Blocked('selected_store_binding_changed')
        if (not isinstance(descriptor,dict) or descriptor.get('path')!=str(path)
            or descriptor.get('claim_uid')!=claim['metadata']['uid'] or descriptor.get('pv_uid')!=pv['metadata']['uid']):
            raise Blocked('selected_store_custody_invalid')
        verify_selected_store_leaf(path.lstat(),descriptor)
        self.new_stores.add(str(path))
        if not hasattr(self,'selected_stores'):self.selected_stores={}
        self.selected_stores[str(path)]=copy.deepcopy(descriptor)

    @staticmethod
    def selected_config_bytes(path,gid):
        fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
        try:
            row=os.fstat(fd)
            if (not stat.S_ISREG(row.st_mode) or row.st_uid!=0 or row.st_gid!=gid
                or stat.S_IMODE(row.st_mode)!=0o440 or row.st_nlink!=1 or not 0<row.st_size<=1<<20):
                raise Blocked('selected_broker_config_custody_invalid')
            raw=os.read(fd,row.st_size+1);after=os.fstat(fd)
            if len(raw)!=row.st_size or (row.st_dev,row.st_ino,row.st_size,row.st_mtime_ns,row.st_ctime_ns)!=(after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns):
                raise Blocked('selected_broker_config_changed')
            return raw
        finally:os.close(fd)

    def selected_broker_config(self):
        # Copy only the already-captured configuration. Never widen the
        # credential tree or change the selected store's permissions.
        original=self.base/'server.conf';raw=self.selected_config_bytes(original,65532)
        folder=Path(tempfile.mkdtemp(prefix='selected-config-',dir=self.base));folder.chmod(0o700)
        target=folder/'server.conf'
        fd=os.open(target,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
        with os.fdopen(fd,'wb') as stream:stream.write(raw);stream.flush();os.fsync(stream.fileno())
        os.chown(target,0,10000);target.chmod(0o440)
        if self.selected_config_bytes(original,65532)!=raw or self.selected_config_bytes(target,10000)!=raw:
            raise Blocked('selected_broker_config_changed')
        return target,hashlib.sha256(raw).hexdigest()

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
                config['User'] != expected.get('user','65532:65532') or host.get('GroupAdd') or
                host['NetworkMode'] != expected['network'] or
                host['Privileged'] or not host['ReadonlyRootfs'] or
                host.get('CapAdd') or host.get('CapDrop') != ['ALL'] or
                host.get('PortBindings') or
                'no-new-privileges:true' not in host.get('SecurityOpt', [])):
            raise Blocked('container_isolation_invalid')
        mounts = {(m['Source'], m['Destination'], bool(m['RW'])) for m in row['Mounts'] if m['Type'] == 'bind'}
        if mounts != expected['mounts'] or any(m['Type'] not in ('bind', 'tmpfs') for m in row['Mounts']):
            raise Blocked('container_mount_invalid')
        if expected.get('selected') is not None:
            selected=expected['selected']
            if self.selected_stores.get(selected['path'])!=selected['descriptor']:
                raise Blocked('selected_broker_descriptor_changed')
            for path,gid in ((self.base/'server.conf',65532),(Path(selected['config']),10000)):
                if hashlib.sha256(self.selected_config_bytes(path,gid)).hexdigest()!=selected['config_sha256']:
                    raise Blocked('selected_broker_config_changed')
        return row

    def create(self, suffix, image, mounts, command, broker=None,*,selected=None):
        if image not in (NATS_IMAGE, BOX_IMAGE) or not re.fullmatch(r'[a-z0-9-]{1,32}', suffix):
            raise Blocked('container_spec_invalid')
        if selected is None and any(str(path) in self.selected_stores for path,_,_ in mounts):
            raise Blocked('selected_broker_route_required')
        image_rows = json.loads(self.run(['image', 'inspect', image]))
        if len(image_rows) != 1 or image.split('@')[1] not in [d.split('@')[-1] for d in image_rows[0]['RepoDigests']]:
            raise Blocked('image_digest_invalid')
        name = 'voice-known-' + self.operation + '-' + suffix
        network = 'none'
        user='65532:65532'
        if selected is not None:
            if (broker is not None or image!=NATS_IMAGE or command!=['/usr/local/bin/nats-server','-c','/server.conf']
                or self.selected_stores.get(selected['path'])!=selected['descriptor']
                or {(str(p),t,w) for p,t,w in mounts}!={(selected['path'],'/data',True),(selected['config'],'/server.conf',False)}):
                raise Blocked('selected_broker_spec_invalid')
            from stage_runtime import verify_selected_store_leaf
            verify_selected_store_leaf(Path(selected['path']).lstat(),selected['descriptor'])
            user='10000:10000'
        if broker is not None:
            if not self.inspect(broker)['State']['Running']:
                raise Blocked('broker_not_running')
            network = 'container:' + self.owned[broker]['id']
        args = ['create', '--name', name, '--label', LABEL+'='+self.operation,
                '--network', network, '--user', user, '--read-only',
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
        self.owned[name] = {'id': cid, 'image': image, 'image_id': image_rows[0]['Id'], 'network': network, 'mounts': expected_mounts,'user':user,'selected':copy.deepcopy(selected)}
        self.inspect(name)
        return name

    def start_broker(self, suffix, store):
        selected=None;config=self.base/'server.conf'
        descriptor=getattr(self,'selected_stores',{}).get(str(store))
        if descriptor is not None:
            config,sha=self.selected_broker_config()
            selected={'path':str(store),'descriptor':copy.deepcopy(descriptor),'config':str(config),'config_sha256':sha}
        arguments=(suffix,NATS_IMAGE,[(store,'/data',True),(config,'/server.conf',False)],
                   ['/usr/local/bin/nats-server','-c','/server.conf'])
        name=self.create(*arguments) if selected is None else self.create(*arguments,selected=selected)
        self.run(['start', name]); self.inspect(name)
        return name

    def stop(self, name):
        self.inspect(name)
        self.run(['stop', '--time', '10', name], timeout=20)
        stopped=self.inspect(name)['State']
        if stopped['Running']:
            raise Blocked('broker_stop_failed')
        if self.owned[name].get('selected') is not None and (
            type(stopped.get('ExitCode')) is not int or stopped['ExitCode']!=0
            or stopped.get('OOMKilled') is not False or stopped.get('Error','')!=''):
            raise Blocked('broker_orderly_shutdown_failed')

    def restart(self, name):
        if self.inspect(name)['State']['Running']:
            raise Blocked('broker_not_closed')
        self.run(['start', name]); self.inspect(name)

    def inputs_directory(self):
        # Only the separately admitted historical recovery uses a fresh copy;
        # every consumer rechecks both original and rematerialized custody.
        receipt=self.base/'expired-recovery-input-repair.json'
        if receipt.exists():
            from expired_recovery import runtime_inputs
            return runtime_inputs(self.base)
        return self.base/'inputs'

    def kernel(self, broker, phase):
        if phase not in ('seed', 'verify-closed', 'drain', 'verify-drained', 'census'):
            raise Blocked('kernel_phase_invalid')
        # A reconstructed operation has an empty owned-container ledger but
        # retains previous proof files. Never reuse or overwrite those outputs.
        out = Path(tempfile.mkdtemp(prefix='out-', dir=self.base))
        os.chown(out, 65532, 65532)
        name = self.create('kernel-'+str(len(self.owned)), NATS_IMAGE,
            [(self.base/'kernel', '/kernel', False), (self.inputs_directory(), '/inputs', False), (out, '/out', True)],
            ['/kernel', '--phase', phase], broker)
        self.run(['start', name])
        if self.run(['wait', name], timeout=60) != '0' or self.inspect(name)['State']['Running']:
            raise Blocked('kernel_failed')
        return out

    def bootstrap(self, broker):
        for role in ('realtime', 'notification', 'analytics-chat', 'search'):
            script = self.base / ('bootstrap-'+role+'.sh')
            name = self.create('bootstrap-'+role+'-'+str(len(self.owned)), BOX_IMAGE,
                [(script, '/bootstrap.sh', False), (self.inputs_directory(), '/inputs', False)],
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
