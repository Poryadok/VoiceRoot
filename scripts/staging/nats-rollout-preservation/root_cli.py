"""Captured human-root operator; selected staging data is never reset."""
import grp
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import uuid
sys.path.insert(0,str(Path(__file__).resolve().parent))
import guard
import transaction
import migrations
import nonnats_plan
from errors import safe_error
from apply import digest, paused_documents
from controller import Blocked, file_sha
from root_main import operation_lock,private_json as root_private_json,public_read,save as root_save
from stage_runtime import Kube, HUB

ROOT=guard.ROOT

def private_json(path):return root_private_json(path,limit=128<<20 if Path(path).name=='checkpoint.json' else 2<<20)
def save(path,row):root_save(path,row,limit=128<<20)


def operation_path(raw):
    base=Path(raw)
    if base.parent!=ROOT or not re.fullmatch(r'rollout-[a-f0-9]{12}',base.name):raise Blocked('rollout_path_invalid')
    s=base.lstat()
    if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022:raise Blocked('rollout_directory_untrusted')
    return base


def code_binding(code):
    manifest=json.loads(public_read(code/'capture-manifest.json'))
    if not isinstance(manifest,dict) or not 1<=len(manifest)<=85:raise Blocked('rollout_code_manifest_invalid')
    for relative,wanted in manifest.items():
        p=Path(relative)
        if p.is_absolute() or '..' in p.parts or not guard.SHA.fullmatch(wanted):raise Blocked('rollout_code_manifest_invalid')
        path=code/p
        for parent in (path,*path.parents):
            try:s=parent.lstat()
            except FileNotFoundError:raise Blocked('rollout_code_custody_invalid') from None
            if s.st_uid!=0 or s.st_mode&0o022 or stat.S_ISLNK(s.st_mode):raise Blocked('rollout_code_custody_invalid')
            if parent==code:break
        if file_sha(path)!=wanted:raise Blocked('rollout_captured_code_changed')
    return manifest


def rollout_directory_kind(base):
    """Launcher captures are code, never operations or an idle-state waiver."""
    base=Path(base);s=base.lstat()
    if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022:
        raise Blocked('rollout_inventory_directory_untrusted')
    if re.fullmatch(r'rollout-code-[a-f0-9]{32}',base.name):
        try:code_binding(base)
        except FileNotFoundError:raise Blocked('rollout_capture_incomplete') from None
        return 'capture'
    if not re.fullmatch(r'rollout-[a-f0-9]{12}',base.name):
        raise Blocked('rollout_inventory_name_invalid')
    return 'operation'


def target_input(path,wanted):
    path=Path(path)
    if path.parent!=Path('/home/pmd/voice-nats-rollout') or not re.fullmatch(r'target-[a-f0-9]{40}\.json',path.name) or not guard.SHA.fullmatch(wanted):
        raise Blocked('rollout_target_input_path_invalid')
    for parent in (*path.parents[::-1],):
        s=parent.lstat()
        if not stat.S_ISDIR(s.st_mode):raise Blocked('rollout_target_parent_invalid')
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        s=os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid!=1000 or s.st_mode&0o022 or s.st_nlink!=1 or not 1<=s.st_size<=2<<20:
            raise Blocked('rollout_target_input_custody_invalid')
        raw=os.read(fd,s.st_size+1)
        if len(raw)!=s.st_size or hashlib.sha256(raw).hexdigest()!=wanted:raise Blocked('rollout_target_input_changed')
        value=json.loads(raw,object_pairs_hook=guard.object_pairs)
    finally:os.close(fd)
    if set(value)!={'target','manifests','contract','migrations','nonnats'} or not isinstance(value['manifests'],list) or not 1<=len(value['manifests'])<=128:
        raise Blocked('rollout_target_input_shape_invalid')
    if digest(value['manifests'])!=value['target']['manifest_sha256']:
        raise Blocked('rollout_target_manifest_digest_invalid')
    migrations.validate_plan(value['migrations'],value['target']['mode'])
    if digest(value['migrations'])!=value['target']['migration_sha256']:
        raise Blocked('rollout_target_migration_digest_invalid')
    if digest(value['nonnats'])!=value['target'].get('nonnats_sha256'):
        raise Blocked('rollout_target_nonnats_digest_invalid')
    try:nonnats_plan._validate(value['nonnats'],value['target']['mode'])
    except nonnats_plan.PlanError:raise Blocked('rollout_target_nonnats_plan_invalid') from None
    # Exactly the deployed named INFO contract is supported; never interpret a
    # different/new bootstrap contract as an authorization to reseed the store.
    known=json.loads(public_read(Path(__file__).resolve().parents[1]/'nats-known-baseline'/'deployed-contract.json'))
    if value['contract']!=known or len(known['streams'])!=15 or len(known['consumer_pairs'])!=42:
        raise Blocked('rollout_nats_contract_unsupported')
    return value


def publish_copy(base,state):
    cut=state['cut'];row={'schema':'nats-rollout-copy-v1','operation':state['operation'],
       'phase':state['phase'],'fence_status':state['fence_status'],
       'archive_sha256':cut['manifest']['archive_sha256'],'manifest_sha256':cut['manifest_sha256'],
       'census_sha256':cut['census_sha256'],'messages':sum(s['state']['messages'] for s in cut['census']['streams']),
       'streams':len(cut['census']['streams']),'consumers':len(cut['census']['consumers'])}
    save(base/'copy-checkpoint.json',row)
    gid=grp.getgrnam('pmd').gr_gid
    for name in ('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json'):
        os.chown(base/name,0,gid);(base/name).chmod(0o440)
    os.chown(base,0,gid);base.chmod(0o750)
    print('NATS_ROLLOUT_COPY_CHECKPOINT='+str(base/'copy-checkpoint.json'))


def main_unlocked(args,code):
    binding=code_binding(code)
    if args==['--enroll-existing-bootstrap']:
        # Explicit human command only; CI bridge has no issuer action.
        from bridge_root import Actions
        from bootstrap_root import enroll_current
        from bootstrap_auth import prove
        enroll_current(Kube(),code,binding,Actions(code)._idle,private_json,save,prove)
        print('NATS_EXISTING_BOOTSTRAP_ENROLLMENT=READY');return
    previous=None
    if len(args)==2 and args[0]=='--prepare-rollback':
        prior=operation_path(args[1]);previous=private_json(prior/'checkpoint.json')
        import paused_recovery
        paused_recovery.verify_adopted_binding(prior,previous,binding)
        if previous.get('renderer_authority') is not None:raise Blocked('renderer_rollback_protected_bridge_required')
        value=transaction.rollback_package(previous)
        expected=previous['context']['expected'];kube=Kube()
        namespace=kube.get('namespace',guard.NS);marker=kube.get('configmap','voice-nats-generation')
        claim=kube.get('pvc',expected['source_claim']);pv=kube.get('pv',claim['spec']['volumeName'])
        if namespace['metadata']['uid']!=expected['namespace_uid'] or marker['metadata']['uid']!=expected['marker_uid'] or marker['data'].get('phase')!='active' or marker['data'].get('dataPVC')!=expected['source_claim'] or claim['metadata']['uid']!=expected['source_claim_uid'] or pv['metadata']['uid']!=expected['source_pv_uid']:
            raise Blocked('rollout_rollback_current_storage_changed')
    elif len(args)==3 and args[0]=='--prepare':
        value=target_input(args[1],args[2])
    else:value=None
    if value is not None:
        return prepare_value(value,code,binding,uuid.uuid4().hex[:12],previous,publish=True)
    if len(args)>=2 and args[0] in ('--authorize','--finish','--rollback','--status'):
        base=operation_path(args[1]);state=private_json(base/'checkpoint.json')
        if state.get('operation')!=base.name[len('rollout-'):]:raise Blocked('rollout_checkpoint_identity_invalid')
        import paused_recovery
        paused_recovery.verify_adopted_binding(base,state,binding)
        if args[0]=='--status' and len(args)==2:
            print(json.dumps({k:state[k] for k in ('schema','operation','phase','status','fence_status','error','restart','preservation') if k in state},sort_keys=True));return
        if args[0]=='--authorize' and len(args)==4:
            receipt=transaction.authorize(Kube(),base,state,args[2],args[3],state['contract'])
            gid=grp.getgrnam('pmd').gr_gid
            for name in ('apply-authorization.json','apply-manifests.json'):
                os.chown(base/name,0,gid);(base/name).chmod(0o440)
            print('NATS_ROLLOUT_AUTHORIZATION='+str(base/'apply-authorization.json'));return
        if args[0]=='--finish' and len(args)==3 and re.fullmatch(r'[0-9]{1,20}',args[2]):
            receipt=guard.protected_receipt(base/'apply-authorization.json')
            result=transaction.finish(Kube(),base,state,receipt,args[2],state['contract'])
            print('NATS_ROLLOUT='+result['status']);return
        if args[0]=='--rollback' and len(args)==3 and re.fullmatch(r'[0-9]{1,20}',args[2]):
            receipt=guard.protected_receipt(base/'apply-authorization.json')
            result=transaction.rollback(Kube(),base,state,receipt,args[2],state['contract'])
            print('NATS_ROLLOUT='+result['status']);return
    raise Blocked('rollout_root_arguments_invalid')


def prepare_value(value,code,binding,op,previous=None,publish=False,before_fence=None,nats_authority=None,actor_authority=None,actor_services=None,renderer_authority=None):
        """Trusted root compiler entrypoint; never accepts a runner target file."""
        if not re.fullmatch(r'[a-f0-9]{12}',op):raise Blocked('rollout_operation_invalid')
        base=ROOT/('rollout-'+op);base.mkdir(mode=0o700)
        # Runner code remains outside the overwritten target checkout. Copy
        # only the manifest's verified code members, never inputs/credentials.
        runner_code=base/'code';runner_code.mkdir(mode=0o750)
        gid=grp.getgrnam('pmd').gr_gid;os.chown(runner_code,0,gid)
        for relative in binding:
            destination=runner_code/relative;destination.parent.mkdir(parents=True,exist_ok=True,mode=0o750)
            for parent in (destination.parent,*destination.parents):
                os.chown(parent,0,gid);parent.chmod(0o750)
                if parent==runner_code:break
            shutil.copyfile(code/relative,destination);os.chown(destination,0,gid);destination.chmod(0o440)
        save(runner_code/'capture-manifest.json',binding)
        os.chown(runner_code/'capture-manifest.json',0,gid);(runner_code/'capture-manifest.json').chmod(0o440)
        raw=json.dumps(value['manifests'],sort_keys=True,separators=(',',':')).encode()
        (base/'apply-manifests.json').write_bytes(raw);(base/'apply-manifests.json').chmod(0o600)
        state=transaction.prepare(Kube(),base,code/'nats-known-baseline',value['target'],value['contract'],op,binding,value['migrations'],value['nonnats'],before_fence=before_fence,nats_authority=nats_authority,actor_authority=actor_authority,actor_services=actor_services,renderer_authority=renderer_authority)
        if previous is not None:state['rollback_from']={'operation':previous['operation'],'images':value['target']['images']}
        state['code_capture']=binding;state['contract']=value['contract'];save(base/'checkpoint.json',state)
        if publish:publish_copy(base,state)
        return state


def main(args):
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('rollout_human_root_linux_required')
    s=ROOT.lstat()
    if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022:raise Blocked('rollout_root_untrusted')
    # Same root-private lock as reset recovery; process death releases it.
    with operation_lock():main_unlocked(args,Path(__file__).resolve().parents[1])


if __name__=='__main__':
    try:main(sys.argv[1:])
    except Exception as error:
        print('NATS_ROLLOUT=BLOCKED',file=sys.stderr)
        print('NATS_ROLLOUT_ERROR='+safe_error(error),file=sys.stderr)
        sys.exit(1)
