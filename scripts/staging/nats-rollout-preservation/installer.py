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
import hub_bridge_identity
import hub_bridge_unit
from root_main import checkpoint_json,operation_lock,private_json,save
from controller import Blocked
from stage_runtime import Kube

V2_BINDING='57457481c7af3148f501083f941dfac0f1c17e75d75b879e377d060a7f5fee77'
V3_BINDING='b0742fec4732b769944e5b849e39229d5f614989c4a439050ec48dd2ed20ab26'
V4_BINDING='426a6edf16ccb8aba30145cfde458c9b2ca5d468cdf4015ea8bdf525e049f47f'
V5_BINDING='23b26ad5478e01d4b00abf171d29a5d210c6c2fbf6284826cf9e3a84cfd73a9a'
CECCB_INSTALLED_BINDING='ceccb8a607957d9c436247fb183d8467d48ac33ece8b3e6c70850404c569e032'
SOURCE_CAPTURE_INSTALLED_BINDING='65717bd51213624f7c0890e97b2cdf1ef9149f57fb27f22f3bdbebca5fb963e5'
SEARCH_BOOTSTRAP_INSTALLED_BINDING='a025ffb62b4c0716a44611c4ac00fe9cae60b8bcb5e8b48204b44a62dc541749'

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

def upgrade_code_v5(code,installed,gid):
    return _upgrade_code(code,installed,gid,V4_BINDING,'v5','v4')

def upgrade_code_v6(code,installed,gid):
    return _upgrade_code(code,installed,gid,V5_BINDING,'v6','v5')

def disposition_predecessor_v6(code,installed):
    installed=Path(installed);current=installed/'code';old=installed/'code-v5-preserved'
    if current.exists() and binding_sha(current)==V5_BINDING:return V5_BINDING
    wanted=binding_sha(code)
    try:record=private_json(installed/'upgrade-v6.json')
    except FileNotFoundError:raise Blocked('bridge_upgrade_predecessor_unapproved')
    if (record!={'schema':'voice-nats-code-upgrade-v6','from':V5_BINDING,'to':wanted}
        or not old.exists() or binding_sha(old)!=V5_BINDING
        or current.exists() and binding_sha(current)!=wanted):raise Blocked('bridge_upgrade_predecessor_unapproved')
    return V5_BINDING

def disposition_predecessor_v5(code,installed):
    installed=Path(installed);current=installed/'code';old=installed/'code-v4-preserved'
    if current.exists() and binding_sha(current)==V4_BINDING:return V4_BINDING
    wanted=binding_sha(code)
    try:record=private_json(installed/'upgrade-v5.json')
    except FileNotFoundError:raise Blocked('bridge_upgrade_predecessor_unapproved')
    if (record!={'schema':'voice-nats-code-upgrade-v5','from':V4_BINDING,'to':wanted}
        or not old.exists() or binding_sha(old)!=V4_BINDING
        or current.exists() and binding_sha(current)!=wanted):
        raise Blocked('bridge_upgrade_predecessor_unapproved')
    return V4_BINDING

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

def upgrade(code,version='v4'):
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('bridge_install_human_root_required')
    if version not in ('v4','v5','v6','v7','v8','v9','v10','v11','v12','v13','v14','v15'):raise Blocked('bridge_upgrade_version_unapproved')
    root_cli.code_binding(Path(code));installed=guard.ROOT/'installed'
    with operation_lock():
        if version in ('v7','v8','v9','v10','v11','v12','v13','v14','v15'):
            import paused_recovery
            paused_recovery.install_repair(code,installed,Kube(),version=version)
            print('NATS_ROLLOUT_BRIDGE=UPGRADED_'+version.upper()+'_KEYS_POLICY_PRESERVED')
            return
        marker=Kube().get('configmap','voice-nats-generation')
        if marker['data'].get('phase')!='active':raise Blocked('bridge_upgrade_live_operation_present')
        for base in guard.ROOT.glob('rollout-*'):
            if root_cli.rollout_directory_kind(base)=='capture':continue
            if checkpoint_json(base/'checkpoint.json').get('status') not in ('PASS','ROLLED_BACK'):raise Blocked('bridge_upgrade_live_operation_present')
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
                    if version=='v6':
                        from prebuild_v5_disposition import enroll
                        disposition_predecessor_v6(code,installed)
                        # V6 only verifies the already enrolled V5 dispositions;
                        # no new exception or journal rewrite is permitted.
                        try:private_json(installed/('prebuild-disposition-'+path.stem[:12]+'.json'))
                        except FileNotFoundError:raise Blocked('bridge_upgrade_interrupted_request') from None
                        enroll(guard.ROOT,path,marker,V4_BINDING)
                    elif version=='v5':
                        from prebuild_v5_disposition import enroll
                        enroll(guard.ROOT,path,marker,disposition_predecessor_v5(code,installed))
                    else:
                        from prebuild_disposition import enroll
                        enroll(guard.ROOT,path,marker,disposition_predecessor(code,installed))
            policy=private_json(installed/'policy.json')
            for name in ('recovery-key.pem','recovery-cert.pem'):
                fd,_=encrypted_cut.regular(installed/'recovery'/name,private=name=='recovery-key.pem')
                os.close(fd)
            if version=='v6':
                import story_witness
                witnesses=story_witness.registry_snapshot()
                if Kube().get('configmap','voice-nats-generation')!=marker:raise Blocked('bridge_upgrade_marker_changed')
                upgrade_code_v6(code,installed,grp.getgrnam('pmd').gr_gid)
                if story_witness.registry_snapshot()!=witnesses:raise Blocked('bridge_upgrade_story_witness_changed')
                if Kube().get('configmap','voice-nats-generation')!=marker:raise Blocked('bridge_upgrade_marker_changed')
            elif version=='v5':
                if Kube().get('configmap','voice-nats-generation')!=marker:raise Blocked('bridge_upgrade_marker_changed')
                upgrade_code_v5(code,installed,grp.getgrnam('pmd').gr_gid)
                if Kube().get('configmap','voice-nats-generation')!=marker:raise Blocked('bridge_upgrade_marker_changed')
            else:upgrade_code_v4(code,installed,grp.getgrnam('pmd').gr_gid)
            if private_json(installed/'policy.json')!=policy:raise Blocked('bridge_upgrade_policy_changed')
        finally:inbox.chmod(0o1730)
    print('NATS_ROLLOUT_BRIDGE=UPGRADED_'+version.upper()+'_KEYS_POLICY_PRESERVED')


def _upgrade_idle_transition(code,predecessor,version,previous_version,result):
    # The retired operation is historical evidence, never repaired or adopted.
    import operator_retirement as retirement
    import story_witness
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('bridge_install_human_root_required')
    code=Path(code);root_cli.code_binding(code);installed=guard.ROOT/'installed';kube=Kube()
    with operation_lock():
        marker=kube.get('configmap','voice-nats-generation')
        if marker['data'].get('phase')!='active' or any(k in marker['data'] for k in ('knownRolloutOperation','knownBaselineOperation')):
            raise Blocked('bridge_upgrade_live_operation_present')
        closure=None;checkpoints={}
        for base in guard.ROOT.glob('rollout-*'):
            if root_cli.rollout_directory_kind(base)=='capture':continue
            state=checkpoint_json(base/'checkpoint.json')
            retirement.idle_state(base,state)
            checkpoints[str(base/'checkpoint.json')]=hashlib.sha256(retirement.private_bytes(base/'checkpoint.json')).hexdigest()
            if any(state.get(key) is not None for key in ('operator_retirement','operator_fence_abort')):
                closure=retirement.verify_terminal(base,state)
        if closure is None:raise Blocked('bridge_upgrade_retirement_required')
        if version=='idle-story-fixture-v1':
            retirement.pending_request_accepted(installed,installed/'journal'/(retirement.STORY_PREFENCE_NONCE+'.json'),closure)
        for path in (installed,installed/'inbox',installed/'processing',installed/'journal',installed/'responses',installed/'recovery'):
            row=path.lstat()
            if not path.is_dir() or path.is_symlink() or row.st_uid!=0 or row.st_mode&0o002 or path.name!='inbox' and row.st_mode&0o020:
                raise Blocked('bridge_upgrade_directory_untrusted')
        inbox=installed/'inbox'
        if __import__('stat').S_IMODE(inbox.lstat().st_mode)!=0o1730:raise Blocked('bridge_upgrade_directory_untrusted')
        inbox.chmod(0o700)
        try:
            if any(inbox.iterdir()) or any((installed/'processing').iterdir()):raise Blocked('bridge_upgrade_pending_request')
            journals={str(path):hashlib.sha256(retirement.private_bytes(path,2<<20,group_read=directory=='responses')).hexdigest()
                for directory in ('journal','responses') for path in (installed/directory).iterdir()}
            source_workspaces={}
            for path in (installed/'journal').iterdir():
                accepted=retirement.pending_request_accepted(installed,path,closure)
                if isinstance(accepted,dict) and accepted.get('disposition')=='SUPERSEDED_FAILED_FINISH':source_workspaces[path.name]=accepted
                if isinstance(accepted,dict) and accepted.get('disposition')=='ABANDONED_COMPILER_INVENTORY_REJECTION':source_workspaces[path.name]=accepted
                if path.name==retirement.SOURCE_CAPTURE_REJECTION['nonce']+'.json':source_workspaces[path.name]=accepted
                if path.stem[:12] in retirement.SEARCH_BOOTSTRAP_REJECTIONS:source_workspaces[path.name]=accepted
                if path.name==retirement.STORY_PREFENCE_NONCE+'.json':source_workspaces[path.name]=accepted
            protected={str(installed/'policy.json'):hashlib.sha256(retirement.private_bytes(installed/'policy.json',2<<20)).hexdigest()}
            abandonment=installed/retirement.PENDING_ABANDONMENT_NAME
            try:abandonment.lstat()
            except FileNotFoundError:protected[str(abandonment)]=None
            else:protected[str(abandonment)]=hashlib.sha256(retirement.private_bytes(abandonment,2<<20)).hexdigest()
            for name in ('recovery-key.pem','recovery-cert.pem'):
                path=installed/'recovery'/name
                fd,_=encrypted_cut.regular(path,private=name=='recovery-key.pem');os.close(fd)
                protected[str(path)]=retirement.file_sha(path)
            witnesses=story_witness.registry_snapshot()
            if kube.get('configmap','voice-nats-generation')!=marker:raise Blocked('bridge_upgrade_marker_changed')
            _upgrade_code(code,installed,grp.getgrnam('pmd').gr_gid,predecessor,version,previous_version)
            if story_witness.registry_snapshot()!=witnesses or kube.get('configmap','voice-nats-generation')!=marker:
                raise Blocked('bridge_upgrade_marker_changed')
            for name,wanted in {**checkpoints,**journals,**protected}.items():
                if wanted is None:
                    try:Path(name).lstat()
                    except FileNotFoundError:continue
                    raise Blocked('bridge_upgrade_history_changed')
                if retirement.file_sha(Path(name))!=wanted:raise Blocked('bridge_upgrade_history_changed')
            for name,wanted in source_workspaces.items():
                if name==retirement.SOURCE_CAPTURE_REJECTION['nonce']+'.json':
                    current=retirement._source_capture_rejection_workspace(installed)
                else:current=retirement.pending_request_accepted(installed,installed/'journal'/name,closure)
                if current!=wanted:
                    raise Blocked('bridge_upgrade_source_workspace_changed')
        finally:inbox.chmod(0o1730)
    print(result)

def upgrade_idle(code):
    import operator_retirement as retirement
    _upgrade_idle_transition(code,retirement.V15_BINDING,'idle','v15',
        'NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_KEYS_POLICY_HISTORY_PRESERVED')

def upgrade_idle_ceccb_v1(code):
    """One exact, separate transition from the reviewed installed ceccb helper."""
    _upgrade_idle_transition(code,CECCB_INSTALLED_BINDING,'idle-ceccb-v1','ceccb',
        'NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_CECCB_V1_KEYS_POLICY_HISTORY_PRESERVED')

def upgrade_idle_source_capture_v1(code):
    """Exact successor for the installed 65717 helper after source capture failed."""
    _upgrade_idle_transition(code,SOURCE_CAPTURE_INSTALLED_BINDING,'idle-source-capture-v1','source-65717',
        'NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_SOURCE_CAPTURE_V1_KEYS_POLICY_HISTORY_PRESERVED')

def upgrade_idle_search_bootstrap_v1(code):
    """Exact a025 successor; failed prepares remain abandoned historical evidence."""
    _upgrade_idle_transition(code,SEARCH_BOOTSTRAP_INSTALLED_BINDING,'idle-search-bootstrap-v1','search-a025',
        'NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_SEARCH_BOOTSTRAP_V1_KEYS_POLICY_HISTORY_PRESERVED')

def upgrade_idle_expiry_reset_v1(code):
    """Exact 43f904 successor after the explicit failed-migration reset closure."""
    from operator_reset import CODE_SHA
    _upgrade_idle_transition(code,CODE_SHA,'idle-expiry-reset-v1','expiry-43f904',
        'NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_EXPIRY_RESET_V1_KEYS_POLICY_HISTORY_PRESERVED')

def upgrade_idle_media_v1(code):
    """Exact 1295 successor after the independently verified original-app abort."""
    _upgrade_idle_transition(code,'1295ba61d4603a18790c33cfb9b12ae5e3d337c799d9853bd0fca3a80cc00c20',
        'idle-media-v1','media-1295',
        'NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_MEDIA_V1_KEYS_POLICY_HISTORY_PRESERVED')

def upgrade_idle_story_fixture_v1(code):
    """Exact monitor-success helper successor; failed preflight bytes stay immutable."""
    _upgrade_idle_transition(code,'7f5b901d8429f2d560bb7f71481baabfd90ad2303cfd1b06c8569d94ff87920b',
        'idle-story-fixture-v1','story-7f5b',
        'NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_STORY_FIXTURE_V1_KEYS_POLICY_HISTORY_PRESERVED')

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
''' + hub_bridge_identity.UNIT_DIRECTIVES
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
    allowed={'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host','developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability','minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin','release_policy'}
    if not isinstance(policy,dict) or not set(policy)<=allowed or not isinstance(policy.get('s3_signing_endpoint'),str):raise Blocked('bridge_install_policy_invalid')
    if 'release_policy' in policy:
        import release_policy_wiring
        value=policy['release_policy']
        try:release_policy_wiring.validate(value,value.get('source_sha') if isinstance(value,dict) else None,guard.NS)
        except ValueError as exc:raise Blocked('bridge_install_release_policy_invalid') from exc
    with operation_lock():
        marker=Kube().get('configmap','voice-nats-generation')
        if marker['data'].get('phase')!='active':raise Blocked('bridge_install_live_operation_present')
        for base in guard.ROOT.glob('rollout-*'):
            if root_cli.rollout_directory_kind(base)=='capture':continue
            if not (base/'checkpoint.json').exists() or checkpoint_json(base/'checkpoint.json').get('status') not in ('PASS','ROLLED_BACK'):raise Blocked('bridge_install_live_operation_present')
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
            hub_bridge_unit.verify(suffix,text)
        subprocess.run(['/usr/bin/systemctl','daemon-reload'],check=True,timeout=30)
        subprocess.run(['/usr/bin/systemctl','enable','--now','voice-nats-preservation.path','voice-nats-preservation.timer'],check=True,timeout=30)
    print('NATS_ROLLOUT_BRIDGE=INSTALLED')

if __name__=='__main__':
    if len(sys.argv)!=2:raise SystemExit('usage: installer.py ROOT_PRIVATE_POLICY_JSON | --upgrade-v4 | --upgrade-v5 | --upgrade-v6 | --upgrade-v7 | --upgrade-v8 | --upgrade-idle | --upgrade-idle-ceccb-v1 | --upgrade-idle-source-capture-v1 | --upgrade-idle-search-bootstrap-v1 | --upgrade-idle-expiry-reset-v1 | --upgrade-idle-media-v1 | --upgrade-idle-story-fixture-v1')
    if sys.argv[1]=='--upgrade-idle-story-fixture-v1':upgrade_idle_story_fixture_v1(Path(__file__).resolve().parents[1])
    elif sys.argv[1]=='--upgrade-idle-media-v1':upgrade_idle_media_v1(Path(__file__).resolve().parents[1])
    elif sys.argv[1]=='--upgrade-idle-expiry-reset-v1':upgrade_idle_expiry_reset_v1(Path(__file__).resolve().parents[1])
    elif sys.argv[1]=='--upgrade-idle':upgrade_idle(Path(__file__).resolve().parents[1])
    elif sys.argv[1]=='--upgrade-idle-ceccb-v1':upgrade_idle_ceccb_v1(Path(__file__).resolve().parents[1])
    elif sys.argv[1]=='--upgrade-idle-source-capture-v1':upgrade_idle_source_capture_v1(Path(__file__).resolve().parents[1])
    elif sys.argv[1]=='--upgrade-idle-search-bootstrap-v1':upgrade_idle_search_bootstrap_v1(Path(__file__).resolve().parents[1])
    elif sys.argv[1]=='--upgrade-v4':upgrade(Path(__file__).resolve().parents[1])
    elif sys.argv[1]=='--upgrade-v5':upgrade(Path(__file__).resolve().parents[1],version='v5')
    elif sys.argv[1]=='--upgrade-v6':upgrade(Path(__file__).resolve().parents[1],version='v6')
    elif sys.argv[1]=='--upgrade-v7':upgrade(Path(__file__).resolve().parents[1],version='v7')
    elif sys.argv[1]=='--upgrade-v8':upgrade(Path(__file__).resolve().parents[1],version='v8')
    elif sys.argv[1]=='--upgrade-v9':upgrade(Path(__file__).resolve().parents[1],version='v9')
    elif sys.argv[1]=='--upgrade-v10':upgrade(Path(__file__).resolve().parents[1],version='v10')
    elif sys.argv[1]=='--upgrade-v11':upgrade(Path(__file__).resolve().parents[1],version='v11')
    elif sys.argv[1]=='--upgrade-v12':upgrade(Path(__file__).resolve().parents[1],version='v12')
    elif sys.argv[1]=='--upgrade-v13':upgrade(Path(__file__).resolve().parents[1],version='v13')
    elif sys.argv[1]=='--upgrade-v14':upgrade(Path(__file__).resolve().parents[1],version='v14')
    elif sys.argv[1]=='--upgrade-v15':upgrade(Path(__file__).resolve().parents[1],version='v15')
    else:install(Path(__file__).resolve().parents[1],sys.argv[1])
