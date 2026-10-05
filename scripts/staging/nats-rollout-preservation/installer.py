"""One human-root installation of immutable code and the fixed inbox service."""
import grp
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
sys.path.insert(0,str(Path(__file__).resolve().parent))
import encrypted_cut
import guard
import root_cli
from root_main import operation_lock,private_json,save
from controller import Blocked
from stage_runtime import Kube

SERVICE='''[Unit]
Description=Voice NATS preservation fixed request bridge
After=docker.service k3s.service
[Service]
Type=oneshot
User=root
Group=root
UMask=0077
Environment=PATH=/usr/local/bin:/usr/bin:/bin
ExecStart=/usr/bin/python3 -I -S /var/lib/voice-nats-preservation/installed/code/nats-rollout-preservation/bridge.py
TimeoutStartSec=2400
KillMode=control-group
NoNewPrivileges=true
MemoryMax=2G
PrivateTmp=true
ProtectHome=read-only
'''
PATH_UNIT='''[Unit]
Description=Watch Voice fixed rollout requests
[Path]
DirectoryNotEmpty=/var/lib/voice-nats-preservation/installed/inbox
Unit=voice-nats-preservation.service
[Install]
WantedBy=multi-user.target
'''
TIMER='''[Unit]
Description=Recover pending Voice rollout requests
[Timer]
OnBootSec=30
OnUnitInactiveSec=60
Unit=voice-nats-preservation.service
[Install]
WantedBy=timers.target
'''

def install(code,policy_path):
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('bridge_install_human_root_required')
    code=Path(code);root_cli.code_binding(code)
    encrypted_cut.directory(guard.ROOT)
    policy_path=Path(policy_path);fd,_=encrypted_cut.regular(policy_path,private=True)
    try:policy=json.loads(os.read(fd,65537))
    finally:os.close(fd)
    allowed={'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host','developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability','minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin'}
    if not isinstance(policy,dict) or not set(policy)<=allowed or not isinstance(policy.get('s3_signing_endpoint'),str):raise Blocked('bridge_install_policy_invalid')
    with operation_lock():
        marker=Kube().get('configmap','voice-nats-generation')
        if marker['data'].get('phase')!='active':raise Blocked('bridge_install_live_operation_present')
        for base in guard.ROOT.glob('rollout-*'):
            if root_cli.rollout_directory_kind(base)=='capture':continue
            if not (base/'checkpoint.json').exists() or private_json(base/'checkpoint.json').get('status') not in ('PASS','ROLLED_BACK'):raise Blocked('bridge_install_live_operation_present')
        installed=guard.ROOT/'installed'
        if installed.exists():raise Blocked('bridge_already_installed_no_replacement')
        installed.mkdir(mode=0o750);gid=grp.getgrnam('pmd').gr_gid;os.chown(installed,0,gid)
        os.chown(guard.ROOT,0,gid);guard.ROOT.chmod(0o750)
        for name,mode in (('inbox',0o1730),('processing',0o700),('journal',0o700),('responses',0o750),('sources',0o700),('recovery',0o700)):
            path=installed/name;path.mkdir(mode=mode)
            if name in ('inbox','responses'):os.chown(path,0,gid)
            path.chmod(mode)
        shutil.copytree(code,installed/'code')
        for path in [installed/'code',*(installed/'code').rglob('*')]:
            os.chown(path,0,gid);path.chmod(0o750 if path.is_dir() else 0o550 if path.name=='kernel' else 0o440)
        save(installed/'policy.json',policy)
        encrypted_cut.initialize_recovery_key(installed/'recovery')
        for suffix,text in (('service',SERVICE),('path',PATH_UNIT),('timer',TIMER)):
            path=Path('/etc/systemd/system')/('voice-nats-preservation.'+suffix)
            fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o644)
            with os.fdopen(fd,'w') as stream:stream.write(text);stream.flush();os.fsync(stream.fileno())
        subprocess.run(['/usr/bin/systemctl','daemon-reload'],check=True,timeout=30)
        subprocess.run(['/usr/bin/systemctl','enable','--now','voice-nats-preservation.path','voice-nats-preservation.timer'],check=True,timeout=30)
    print('NATS_ROLLOUT_BRIDGE=INSTALLED')

if __name__=='__main__':
    if len(sys.argv)!=2:raise SystemExit('usage: installer.py ROOT_PRIVATE_POLICY_JSON')
    install(Path(__file__).resolve().parents[1],sys.argv[1])
