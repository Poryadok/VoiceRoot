"""One human-root installation of immutable code and the fixed inbox service."""
import grp
import hashlib
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

V2_BINDING='57457481c7af3148f501083f941dfac0f1c17e75d75b879e377d060a7f5fee77'
V3_BINDING='b0742fec4732b769944e5b849e39229d5f614989c4a439050ec48dd2ed20ab26'

def binding_sha(code):
    return hashlib.sha256(json.dumps(root_cli.code_binding(code),sort_keys=True,separators=(',',':')).encode()).hexdigest()

def sync_directory(path):
    fd=os.open(path,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(fd)
    finally:os.close(fd)

def upgrade_code(code,installed,gid):
    """Fixed reviewed V2 predecessor, preserved backup and resumable renames.

    Caller holds the global operation lock and has proved host/queue idle.
    Policy, recovery, request journals and units are never replaced.
    """
    return _upgrade_code(code,installed,gid,V2_BINDING,'v3','v2')

def upgrade_code_v4(code,installed,gid):
    return _upgrade_code(code,installed,gid,V3_BINDING,'v4','v3')

def disposition_predecessor(code,installed):
    """Retain the exact predecessor identity through either owned rename."""
    installed=Path(installed);current=installed/'code';old=installed/'code-v3-preserved'
    if current.exists() and binding_sha(current)==V3_BINDING:return V3_BINDING
    wanted=binding_sha(code)
    try:record=private_json(installed/'upgrade-v4.json')
    except FileNotFoundError:raise Blocked('bridge_upgrade_predecessor_unapproved')
    if (record!={'schema':'voice-nats-code-upgrade-v4','from':V3_BINDING,'to':wanted}
        or not old.exists() or binding_sha(old)!=V3_BINDING
        or current.exists() and binding_sha(current)!=wanted):
        raise Blocked('bridge_upgrade_predecessor_unapproved')
    return V3_BINDING

def _upgrade_code(code,installed,gid,predecessor,version,previous_version):
    code=Path(code);installed=Path(installed);wanted=binding_sha(code)
    record=installed/('upgrade-'+version+'.json');current=installed/'code'
    staged=installed/('code-'+version+'-staged');old=installed/('code-'+previous_version+'-preserved')
    if record.exists():
        receipt=private_json(record)
        if receipt!={'schema':'voice-nats-code-upgrade-'+version,'from':predecessor,'to':wanted}:
            raise Blocked('bridge_upgrade_receipt_conflict')
    else:
        if binding_sha(current)!=predecessor or old.exists() or staged.exists():
            raise Blocked('bridge_upgrade_predecessor_unapproved')
        save(record,{'schema':'voice-nats-code-upgrade-'+version,'from':predecessor,'to':wanted})
    if current.exists() and binding_sha(current)==wanted:
        if not old.exists() or binding_sha(old)!=predecessor:raise Blocked('bridge_upgrade_backup_unapproved')
        return
    if old.exists():
        if binding_sha(old)!=predecessor or current.exists():raise Blocked('bridge_upgrade_backup_conflict')
    elif binding_sha(current)!=predecessor:raise Blocked('bridge_upgrade_predecessor_unapproved')
    if staged.exists():
        if binding_sha(staged)!=wanted:raise Blocked('bridge_upgrade_staged_unapproved')
    else:
        shutil.copytree(code,staged)
        for path in [staged,*staged.rglob('*')]:
            os.chown(path,0,gid);path.chmod(0o750 if path.is_dir() else 0o550 if path.name in ('kernel','bootstrap-renewer') else 0o440)
        if binding_sha(staged)!=wanted:raise Blocked('bridge_upgrade_staged_unapproved')
        sync_directory(installed)
    if not old.exists():
        os.rename(current,old);sync_directory(installed)
    os.rename(staged,current);sync_directory(installed)
    if binding_sha(current)!=wanted or binding_sha(old)!=predecessor:raise Blocked('bridge_upgrade_verification_failed')

def upgrade(code):
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('bridge_install_human_root_required')
    root_cli.code_binding(Path(code));installed=guard.ROOT/'installed'
    with operation_lock():
        marker=Kube().get('configmap','voice-nats-generation')
        if marker['data'].get('phase')!='active':raise Blocked('bridge_upgrade_live_operation_present')
        for base in guard.ROOT.glob('rollout-*'):
            if root_cli.rollout_directory_kind(base)=='capture':continue
            if private_json(base/'checkpoint.json').get('status') not in ('PASS','ROLLED_BACK'):raise Blocked('bridge_upgrade_live_operation_present')
        for path in (installed,installed/'inbox',installed/'processing',installed/'journal',installed/'responses',installed/'recovery'):
            row=path.lstat()
            if not path.is_dir() or path.is_symlink() or row.st_uid!=0 or row.st_mode&0o002 or path.name!='inbox' and row.st_mode&0o020:raise Blocked('bridge_upgrade_directory_untrusted')
        # Close the runner ingress while checking and replacing code. Requests
        # are preserved; an existing queued request vetoes, never gets removed.
        inbox=installed/'inbox';inbox.chmod(0o700)
        try:
            if any(inbox.iterdir()) or any((installed/'processing').iterdir()):raise Blocked('bridge_upgrade_pending_request')
            for path in (installed/'journal').iterdir():
                if private_json(path).get('phase')!='COMPLETE':
                    from prebuild_disposition import enroll
                    enroll(guard.ROOT,path,marker,disposition_predecessor(code,installed))
            policy=private_json(installed/'policy.json')
            for name in ('recovery-key.pem','recovery-cert.pem'):
                fd,_=encrypted_cut.regular(installed/'recovery'/name,private=name=='recovery-key.pem')
                os.close(fd)
            upgrade_code_v4(code,installed,grp.getgrnam('pmd').gr_gid)
            if private_json(installed/'policy.json')!=policy:raise Blocked('bridge_upgrade_policy_changed')
        finally:inbox.chmod(0o1730)
    print('NATS_ROLLOUT_BRIDGE=UPGRADED_V4_KEYS_POLICY_PRESERVED')

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
            os.chown(path,0,gid);path.chmod(0o750 if path.is_dir() else 0o550 if path.name in ('kernel','bootstrap-renewer') else 0o440)
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
    if len(sys.argv)!=2:raise SystemExit('usage: installer.py ROOT_PRIVATE_POLICY_JSON | --upgrade-v4')
    if sys.argv[1]=='--upgrade-v4':upgrade(Path(__file__).resolve().parents[1])
    else:install(Path(__file__).resolve().parents[1],sys.argv[1])
