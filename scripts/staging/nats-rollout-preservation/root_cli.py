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


def public_code_sha(path,*,content=False):
    """Hash public captured code as the runner, without root-only O_NOATIME."""
    path=Path(path)
    limit=(32 if path.parent.name=='nats-rollout-preservation' and path.name=='story-restored8-probe' else 16)<<20
    fingerprint=lambda s:(s.st_dev,s.st_ino,s.st_mode,s.st_uid,s.st_gid,
        s.st_nlink,s.st_size,s.st_mtime_ns,s.st_ctime_ns)
    parent_before=path.parent.lstat()
    parent=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        if (not stat.S_ISDIR(parent_before.st_mode) or parent_before.st_uid!=0
            or parent_before.st_mode&0o022
            or fingerprint(parent_before)!=fingerprint(os.fstat(parent))):
            raise Blocked('rollout_code_custody_invalid')
        before=os.stat(path.name,dir_fd=parent,follow_symlinks=False)
        fd=os.open(path.name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=parent)
        try:
            opened=os.fstat(fd)
            if (not stat.S_ISREG(opened.st_mode) or opened.st_uid!=0
                or opened.st_mode&0o022 or opened.st_nlink!=1
                or not 0<=opened.st_size<=limit or fingerprint(before)!=fingerprint(opened)):
                raise Blocked('rollout_code_custody_invalid')
            digest=hashlib.sha256();count=0;chunks=[] if content else None
            while chunk:=os.read(fd,1<<20):
                count+=len(chunk)
                if count>opened.st_size:raise Blocked('rollout_captured_code_changed')
                digest.update(chunk)
                if chunks is not None:chunks.append(chunk)
            if (count!=opened.st_size or fingerprint(opened)!=fingerprint(os.fstat(fd))
                or fingerprint(opened)!=fingerprint(os.stat(path.name,dir_fd=parent,follow_symlinks=False))
                or fingerprint(parent_before)!=fingerprint(os.fstat(parent))
                or fingerprint(parent_before)!=fingerprint(path.parent.lstat())):
                raise Blocked('rollout_captured_code_changed')
            return b''.join(chunks) if chunks is not None else digest.hexdigest()
        finally:os.close(fd)
    finally:os.close(parent)


def code_binding(code):
    manifest=json.loads(public_read(code/'capture-manifest.json'),object_pairs_hook=guard.object_pairs)
    # Historical captures retain their bound; admit only the exact successor inventory.
    if (not isinstance(manifest,dict) or not (1<=len(manifest)<=89 or
        len(manifest)==94 and hashlib.sha256(json.dumps(sorted(manifest),separators=(',',':')).encode()).hexdigest()
        == '13397edd8fae921ff480cd366fbcb63439fe00e55e3f8bfe9c00ea0c9790fe20' or
        len(manifest)==99 and hashlib.sha256(json.dumps(sorted(manifest),separators=(',',':')).encode()).hexdigest()
        == '9e08bb7df3c0dcdb9f4bf6287895eff08a4451062ca41199b79ca131da7465e4' or
        len(manifest)==100 and hashlib.sha256(json.dumps(sorted(manifest),separators=(',',':')).encode()).hexdigest()
        == '6d3ce2a5893bbb4d340a3a97daddb2e024f8ebfdb358db6641b0258e621bb002')):
        raise Blocked('rollout_code_manifest_invalid')
    for relative,wanted in manifest.items():
        p=Path(relative)
        if p.is_absolute() or '..' in p.parts or not guard.SHA.fullmatch(wanted):raise Blocked('rollout_code_manifest_invalid')
        path=code/p
        for parent in (path,*path.parents):
            try:s=parent.lstat()
            except FileNotFoundError:raise Blocked('rollout_code_custody_invalid') from None
            if s.st_uid!=0 or s.st_mode&0o022 or stat.S_ISLNK(s.st_mode):raise Blocked('rollout_code_custody_invalid')
            if parent==code:break
        if public_code_sha(path)!=wanted:raise Blocked('rollout_captured_code_changed')
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
    allowed={'target','manifests','contract','migrations','nonnats','gis_first_install'}
    if not {'target','manifests','contract','migrations','nonnats'}<=set(value) or set(value)-allowed or not isinstance(value['manifests'],list) or not 1<=len(value['manifests'])<=128:
        raise Blocked('rollout_target_input_shape_invalid')
    if 'gis_first_install' in value:
        # This legacy root CLI accepts a runner-created file plus its own hash.
        # GIS creation is admitted only by bridge_root's source/CI/lock-bound
        # compiler path, never by a caller-supplied target document.
        raise Blocked('gis_first_install_protected_bridge_required')
    if digest(value['manifests'])!=value['target']['manifest_sha256']:
        raise Blocked('rollout_target_manifest_digest_invalid')
    migrations.validate_plan(value['migrations'],value['target']['mode'])
    if digest(value['migrations'])!=value['target']['migration_sha256']:
        raise Blocked('rollout_target_migration_digest_invalid')
    if digest(value['nonnats'])!=value['target'].get('nonnats_sha256'):
        raise Blocked('rollout_target_nonnats_digest_invalid')
    try:nonnats_plan._validate(value['nonnats'],value['target']['mode'])
    except nonnats_plan.PlanError:raise Blocked('rollout_target_nonnats_plan_invalid') from None
    if 'gis_first_install' in value:
        import gis_first_install
        if (value['target']['mode'] not in ('full','app-only')
            or value['target'].get('gis_first_install_sha256')!=digest(value['gis_first_install'])):
            raise Blocked('gis_first_install_target_binding_invalid')
        gis_first_install.validate_source_plan(value['gis_first_install'])
    # Exactly the deployed named INFO contract is supported; never interpret a
    # different/new bootstrap contract as an authorization to reseed the store.
    known=json.loads(public_read(Path(__file__).resolve().parents[1]/'nats-known-baseline'/'deployed-contract.json'))
    if value['contract']!=known or len(known['streams'])!=15 or len(known['consumer_pairs'])!=42:
        raise Blocked('rollout_nats_contract_unsupported')
    return value


def _copy_record(state):
    import resource_claim
    installation=resource_claim.production_installation()
    schema=state.get('schema')
    if schema=='nats-rollout-root-v2':
        binding=resource_claim.historical_binding(state,installation=installation)
        if binding['schema']!='voice-staging-resource-fence-v2':raise Blocked('rollout_copy_checkpoint_changed')
    elif schema=='nats-rollout-root-v1':
        if 'installation_sha256' in state or 'installation_sha256' in state.get('target',{}):
            raise Blocked('rollout_copy_checkpoint_changed')
        if 'resource_fence' in state:
            binding=resource_claim.historical_binding(state,installation=installation)
            if binding['schema']!='voice-staging-resource-fence-v1':raise Blocked('rollout_copy_checkpoint_changed')
    else:raise Blocked('rollout_copy_checkpoint_changed')
    cut=state['cut'];row={'schema':'nats-rollout-copy-v2' if schema=='nats-rollout-root-v2' else 'nats-rollout-copy-v1','operation':state['operation'],
       'phase':state['phase'],'fence_status':state['fence_status'],
       'archive_sha256':cut['manifest']['archive_sha256'],'manifest_sha256':cut['manifest_sha256'],
       'census_sha256':cut['census_sha256'],'messages':sum(s['state']['messages'] for s in cut['census']['streams']),
       'streams':len(cut['census']['streams']),'consumers':len(cut['census']['consumers'])}
    if schema=='nats-rollout-root-v2':row['installation_sha256']=installation.sha256
    return row


def publish_copy(base,state):
    row=_copy_record(state)
    save(base/'copy-checkpoint.json',row)
    gid=grp.getgrnam('pmd').gr_gid
    for name in ('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json'):
        os.chown(base/name,0,gid);(base/name).chmod(0o440)
    os.chown(base,0,gid);base.chmod(0o750)
    print('NATS_ROLLOUT_COPY_CHECKPOINT='+str(base/'copy-checkpoint.json'))


def main_unlocked(args,code):
    binding=code_binding(code)
    if args and args[0] in ('--prepare','--prepare-rollback','--authorize','--finish','--rollback'):
        raise Blocked('rollout_resource_fenced_bridge_required')
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


def prepare_value(value,code,binding,op,previous=None,publish=False,before_fence=None,nats_authority=None,actor_authority=None,actor_services=None,renderer_authority=None,lifecycle=None,gis_source_authority=None,gis_source_root=None,gis_source_capture=None,gis_transition=None,resource_session=None,gis_binding=None,realtime_bootstrap_receipt=None,prerequisites_capture=None,_preservation_ref=None):
        """Trusted root compiler entrypoint; never accepts a runner target file."""
        if not re.fullmatch(r'[a-f0-9]{12}',op):raise Blocked('rollout_operation_invalid')
        base=ROOT/('rollout-'+op)
        if _preservation_ref is None:base.mkdir(mode=0o700)
        else:
            import staging_cohort_preservation
            if (resource_session is None or resource_session.operation!=op
                or staging_cohort_preservation.load_qualified(resource_session)!=_preservation_ref
                or value['target'].get('cohort_preservation_sha256')!=transaction.canonical(_preservation_ref)):
                raise Blocked('cohort_preserved_prepare_invalid')
            if not callable(getattr(transaction,'prepare_preserved',None)):
                raise Blocked('cohort_preserved_transaction_missing')
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
        if _preservation_ref is None:
            (base/'apply-manifests.json').write_bytes(raw);(base/'apply-manifests.json').chmod(0o600)
        else:
            fd=os.open(base/'apply-manifests.json',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
            with os.fdopen(fd,'wb') as stream:
                stream.write(raw);stream.flush();os.fsync(stream.fileno())
        lifecycle_args={'lifecycle':lifecycle} if lifecycle is not None else {}
        if resource_session is not None:lifecycle_args['resource_session']=resource_session
        if gis_binding is not None:lifecycle_args['gis_binding']=gis_binding
        if realtime_bootstrap_receipt is not None:lifecycle_args['realtime_bootstrap_receipt']=realtime_bootstrap_receipt
        if prerequisites_capture is not None:lifecycle_args['prerequisites_capture']=prerequisites_capture
        if _preservation_ref is not None:lifecycle_args['preservation_ref']=_preservation_ref
        gis_first_install=(None if 'gis_first_install' not in value else
            __import__('gis_first_install').bind_operation(value['gis_first_install'],op,value['target']))
        prepare_transaction=transaction.prepare if _preservation_ref is None else transaction.prepare_preserved
        state=prepare_transaction(Kube(),base,code/'nats-known-baseline',value['target'],value['contract'],op,binding,value['migrations'],value['nonnats'],before_fence=before_fence,nats_authority=nats_authority,actor_authority=actor_authority,actor_services=actor_services,renderer_authority=renderer_authority,gis_first_install=gis_first_install,gis_source_root=(gis_source_root if gis_first_install is not None else None),gis_source_capture=(gis_source_capture if gis_first_install is not None else None),gis_source_authority=(gis_source_authority if gis_first_install is not None or gis_binding is not None else None),gis_transition=gis_transition,**lifecycle_args)
        if previous is not None:state['rollback_from']={'operation':previous['operation'],'images':value['target']['images']}
        state['code_capture']=binding;state['contract']=value['contract'];save(base/'checkpoint.json',state)
        if publish:publish_copy(base,state)
        return state


def prepare_preserved_value(value,code,binding,op,*,resource_session,preservation_ref,
                            nats_authority,actor_authority,actor_services,renderer_authority,
                            gis_source_authority,gis_source_root,gis_source_capture,gis_transition,
                            gis_binding,realtime_bootstrap_receipt,prerequisites_capture):
    """Continue only an already qualified, root-owned preservation operation."""
    if preservation_ref is None:raise Blocked('cohort_preserved_prepare_invalid')
    return prepare_value(value,code,binding,op,resource_session=resource_session,_preservation_ref=preservation_ref,
        nats_authority=nats_authority,actor_authority=actor_authority,actor_services=actor_services,
        renderer_authority=renderer_authority,gis_source_authority=gis_source_authority,
        gis_source_root=gis_source_root,gis_source_capture=gis_source_capture,gis_transition=gis_transition,
        gis_binding=gis_binding,realtime_bootstrap_receipt=realtime_bootstrap_receipt,prerequisites_capture=prerequisites_capture)


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
