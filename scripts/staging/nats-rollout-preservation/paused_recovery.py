"""Exact reviewed paused operation adoption; never a general idle waiver."""
import copy
import hashlib
import json
import os
from pathlib import Path
import stat
import time
from controller import Blocked

OPERATION='3340764a7d24'
NONCE='3340764a7d247e69ea1f012f3ac028619fe7fa92a9a8e2c77571fd6b414948d0'
RUN_ID=37683863372
SOURCE='73ca52699ddf6a9182e3407d5bbdedbc29c06dee'
SOURCE_RUN=37652821250
V6_BINDING='9c65bfca72fd433ff45dadf760d448f6fb805c999541d0176043ad721ff5455c'
MARKER_UID='cbafb90c-6974-4642-86d6-09b2eef1c843'
CLAIM_UID='e319328f-1fb4-4a02-9c95-1dfa1997a668'
PV_UID='01ef090e-a221-4aa4-9a6d-7f74304cf4c6'

def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()

def initial_checkpoint(state, journal):
    """Validate the preserved failed boundary before any repair adoption."""
    try:
        expected=state['context']['expected'];request=journal['request']
        if (state['operation']!=OPERATION or state['phase']!='COLD_BACKUP'
            or state['status']!='BLOCKED' or state['fence_status']!='VERIFIED'
            or state.get('error')!='rollout_selected_store_custody_invalid'
            or digest(state['code_capture'])!=V6_BINDING
            or state['target']['tag']!=SOURCE or state['target']['mode']!='images-only'
            or state['target']['changed_services']!=['story']
            or journal.get('phase')!='STARTED' or request!={'action':'prepare','nonce':NONCE,
                'source_sha':SOURCE,'run_id':RUN_ID,'mode':'images-only','changed_services':['story']}
            or expected['marker_uid']!=MARKER_UID or expected['source_claim_uid']!=CLAIM_UID
            or expected['source_pv_uid']!=PV_UID
            or any(key in state for key in ('cut','cipher_binding','custody','restart','preservation','repair_adoption'))):
            raise ValueError()
        return {'operation':OPERATION,'source_sha':SOURCE,'source_run_id':SOURCE_RUN,
                'original_state_sha256':digest(state),'original_code_sha256':V6_BINDING,
                'input_target_sha256':digest(state['input_target']),
                'target_sha256':digest(state['target']),
                'actor_sha256':digest(state['service_actor_authority'])}
    except (KeyError,TypeError,ValueError):
        raise Blocked('paused_recovery_original_boundary_invalid') from None

def original_target_unchanged(state, original):
    """Later state may advance phases; captured target/actor authority may not."""
    if (state.get('operation')!=OPERATION or state.get('input_target')!=original.get('input_target')
        or state.get('target')!=original.get('target')
        or state.get('service_actor_authority')!=original.get('service_actor_authority')
        or state.get('service_actor_services')!=original.get('service_actor_services')):
        raise Blocked('paused_recovery_original_authority_changed')
    return copy.deepcopy(original['code_capture'])

def owned_bytes(path,limit=128<<20,*,private=True):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        row=os.fstat(fd)
        if (not stat.S_ISREG(row.st_mode) or row.st_uid!=0 or row.st_mode&(0o077 if private else 0o022)
            or row.st_nlink!=1 or not 0<row.st_size<=limit):
            raise Blocked('paused_recovery_private_custody_invalid')
        raw=os.read(fd,row.st_size+1);after=os.fstat(fd)
        if len(raw)!=row.st_size or (row.st_dev,row.st_ino,row.st_size,row.st_mtime_ns,row.st_ctime_ns)!=(after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns):
            raise Blocked('paused_recovery_private_input_changed')
        return raw
    finally:os.close(fd)

def private_bytes(path,limit=128<<20):
    return owned_bytes(path,limit,private=True)

def verify_original_journal(path,record):
    if hashlib.sha256(private_bytes(path,65536)).hexdigest()!=record['original_journal_sha256']:
        raise Blocked('paused_recovery_journal_changed')

def retained_source_files(workspace,tree):
    """Exact root-retained bytes against the canonical Git blob inventory."""
    import source_authority as authority
    from encrypted_cut import directory
    workspace=Path(workspace);directory(workspace)
    if tree.get('truncated') is not False or not isinstance(tree.get('tree'),list) or len(tree['tree'])>authority.MAX_ITEMS:
        raise Blocked('paused_recovery_source_tree_invalid')
    expected={};directories=set();total=0
    for row in tree['tree']:
        name=str(authority._path(row['path']))
        if name in expected or name in directories:raise Blocked('paused_recovery_source_tree_invalid')
        if row['type']=='tree' and row['mode']=='040000':directories.add(name);continue
        if row['type']!='blob' or row['mode'] not in ('100644','100755') or type(row['size']) is not int or not 0<=row['size']<=authority.MAX_FILE:
            raise Blocked('paused_recovery_source_tree_invalid')
        expected[name]=row;total+=row['size']
    if not expected or total>authority.MAX_SOURCE:raise Blocked('paused_recovery_source_tree_invalid')
    files={};seen=set()
    for path in workspace.rglob('*'):
        name=path.relative_to(workspace).as_posix();row=path.lstat()
        if stat.S_ISDIR(row.st_mode):
            if name not in directories:raise Blocked('paused_recovery_retained_source_changed')
            directory(path);continue
        if name not in expected or not stat.S_ISREG(row.st_mode):raise Blocked('paused_recovery_retained_source_changed')
        wanted=expected[name]
        # Canonical Git may contain empty files; custody remains exact.
        if wanted['size']==0:
            fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
            try:
                metadata=os.fstat(fd)
                if not stat.S_ISREG(metadata.st_mode) or metadata.st_uid!=0 or metadata.st_mode&0o077 or metadata.st_nlink!=1 or metadata.st_size!=0:
                    raise Blocked('paused_recovery_retained_source_changed')
                raw=b''
            finally:os.close(fd)
        else:raw=private_bytes(path,authority.MAX_FILE)
        if len(raw)!=wanted['size'] or hashlib.sha1(b'blob '+str(len(raw)).encode()+b'\0'+raw).hexdigest()!=wanted['sha']:
            raise Blocked('paused_recovery_retained_source_changed')
        files[name]=hashlib.sha256(raw).hexdigest();seen.add(name)
    if seen!=set(expected):raise Blocked('paused_recovery_retained_source_changed')
    return dict(sorted(files.items()))

def reconstruct_source(token,base,state,binding):
    """Only this preserved operation may use its historical approved target."""
    import source_authority as authority
    import github_custody as transport
    import guard
    import tempfile
    import zipfile
    if state.get('operation')!=OPERATION or state.get('repair_adoption') is None:
        raise Blocked('paused_recovery_adoption_required')
    verify_adopted_binding(base,state,binding)
    workspace=guard.ROOT/'installed'/'sources'/OPERATION
    headers=authority._headers(token);deadline=time.monotonic()+600
    repo=authority._get(authority.API,headers,deadline)
    if repo.get('full_name')!=authority.REPO or repo.get('private') is not False or repo.get('default_branch')!='master':
        raise Blocked('paused_recovery_source_repository_invalid')
    run,jobs=authority._ci(headers,SOURCE_RUN,SOURCE,repo,deadline)
    if run['status']!='completed' or run['conclusion']!='success':raise Blocked('paused_recovery_source_ci_invalid')
    commit=authority._get(authority.API+'/git/commits/'+SOURCE,headers,deadline)
    if commit.get('sha')!=SOURCE:raise Blocked('paused_recovery_source_commit_invalid')
    tree=authority._get(authority.API+'/git/trees/'+commit['tree']['sha']+'?recursive=1',headers,deadline)
    if tree.get('sha')!=commit['tree']['sha']:raise Blocked('paused_recovery_source_tree_invalid')
    files=retained_source_files(workspace,tree)
    artifacts=authority._pages(authority.API+'/actions/runs/'+str(SOURCE_RUN)+'/artifacts','artifacts',headers,deadline)
    matches=[a for a in artifacts if a['name']=='staging-stack-lock']
    if len(matches)!=1 or type(matches[0]['id']) is not int:raise Blocked('paused_recovery_source_artifact_invalid')
    artifact=authority._get(authority.API+'/actions/artifacts/'+str(matches[0]['id']),headers,deadline)
    workflow=artifact['workflow_run'];job=jobs['staging-stack-lock']
    if (artifact['id']!=matches[0]['id'] or artifact['name']!='staging-stack-lock' or artifact['expired'] is not False
        or workflow['id']!=SOURCE_RUN or workflow['head_sha']!=SOURCE or workflow['repository_id']!=repo['id']
        or workflow['head_repository_id']!=repo['id']
        or not transport._timestamp(job['started_at'])<=transport._timestamp(artifact['created_at'])<=transport._timestamp(job['completed_at'])
        or transport._timestamp(artifact['expires_at'])<=transport.datetime.now(transport.timezone.utc)):
        raise Blocked('paused_recovery_source_artifact_invalid')
    with tempfile.TemporaryFile() as archive:
        count,_=authority._fetch(authority.API+'/actions/artifacts/'+str(artifact['id'])+'/zip',headers,4<<20,deadline,target=archive)
        if type(artifact.get('size_in_bytes')) is not int or count!=artifact['size_in_bytes']:raise Blocked('paused_recovery_source_artifact_invalid')
        archive.seek(0)
        with zipfile.ZipFile(archive) as zipped:
            members=zipped.infolist()
            if (len(members)!=1 or members[0].filename!='stack.lock.yaml' or members[0].is_dir()
                or members[0].file_size>1<<20 or stat.S_IFMT(members[0].external_attr>>16) not in (0,stat.S_IFREG)
                or members[0].flag_bits&1):raise Blocked('paused_recovery_source_artifact_invalid')
            lock=zipped.read(members[0])
    tags=authority._lock(lock,SOURCE,workspace)
    images={name:authority._resolve_image(name,tag,deadline) for name,tag in sorted(tags.items())}
    if retained_source_files(workspace,tree)!=files:raise Blocked('paused_recovery_retained_source_changed')
    return {'schema':'voice-source-authority-v1','repository':authority.REPO,'source_sha':SOURCE,
        'workflow_id':authority.WORKFLOW,'run_id':SOURCE_RUN,'run_attempt':run['run_attempt'],
        'tree_sha':tree['sha'],'source_files':files,'images':images,'lock_artifact_id':artifact['id'],
        'lock_sha256':hashlib.sha256(lock).hexdigest(),'image_provenance':'independently resolved trusted CI tag',
        'verified':True,'paused_operation':OPERATION}

class PausedOriginalProducer:
    """Canonical desired inputs for this exact already-owned fenced operation.

    Only file/user signing descriptors need their original desired replicas.
    Fresh live metadata is retained; all other reads and dry-runs remain live.
    """
    NAMES=('voice-file','voice-user')

    def __init__(self,stage,state):
        import nonnats_plan
        self.stage=stage;self.state=state;self.plan=nonnats_plan
        if (state.get('operation')!=OPERATION or state.get('repair_adoption') is None
            or state.get('target',{}).get('tag')!=SOURCE
            or state['target'].get('mode')!='images-only'
            or state['target'].get('changed_services')!=['story']):
            raise Blocked('paused_recovery_original_producer_boundary_invalid')
        self.verify()

    def _project(self,name):
        try:
            matches=[d for d in self.state['nonnats_binding']['objects']
                if d.get('kind')=='Deployment' and d.get('name')==name]
            if len(matches)!=1:raise ValueError()
            descriptor=matches[0];owned=self.stage.snapshots[name]
            original=self.stage.original_snapshots[name]
            current=self.stage.kube.get('Deployment',name)
            for row in (original,owned,current):
                if (row.get('kind')!='Deployment' or row['metadata']['name']!=name
                    or row['metadata'].get('namespace')!='voice-staging'
                    or not isinstance(row.get('spec'),dict)):
                    raise ValueError()
            if (descriptor.get('namespace')!='voice-staging'
                or descriptor.get('disposition')!='preserve'
                or any(row['metadata']['uid']!=descriptor['uid'] for row in (original,owned,current))
                or current['metadata']['resourceVersion']!=owned['metadata']['resourceVersion']
                or self.plan.semantic(current)!=self.plan.semantic(owned)
                or self.plan.semantic(original)!=descriptor['desired']
                or original['spec'].get('replicas',1)!=1
                or owned['spec'].get('replicas',1)!=0):
                raise ValueError()
            scaled=self.plan.semantic(original);scaled['spec']['replicas']=0
            if self.plan.semantic(owned)!=scaled:raise ValueError()
            # Server dry-run needs the CURRENT RV. Restore only desired spec;
            # source semantics exclude controller status and runtime metadata.
            projected=copy.deepcopy(current);projected['spec']=copy.deepcopy(original['spec'])
            return projected
        except (KeyError,TypeError,ValueError):
            raise Blocked('paused_recovery_owned_desired_changed') from None

    def verify(self):
        for name in self.NAMES:self._project(name)

    def get(self,kind,name):
        self.verify()
        if kind=='Deployment' and name in self.NAMES:return self._project(name)
        return self.stage.kube.get(kind,name)

    def run(self,args,body=None,timeout=30):
        self.verify()
        return self.stage.kube.run(args,body=body,timeout=timeout)

    def secret_meta(self,name):
        self.verify();return self.stage.kube.secret_meta(name)


def recompile_target(base,state,stage,approved):
    """Rebuild only retained canonical target inputs; compare all three forms."""
    import compiler
    import guard
    import tempfile
    from normalize import image_only_documents,normalize_target
    workspace=guard.ROOT/'installed'/'sources'/OPERATION
    build=workspace.parent/(OPERATION+'-build')
    parameters=decode(private_bytes(build/'parameters.json',2<<20))
    retained=decode(private_bytes(build/('target-'+SOURCE+'.json'),2<<20))
    if (parameters.get('tag')!=SOURCE or parameters.get('mode')!='images-only'
        or parameters.get('changed_services')!=['story'] or parameters.get('contract')!=state['contract']
        or parameters.get('generation')!=state['context']['expected']['generation']
        or parameters.get('dataPVC')!=state['context']['expected']['source_claim']
        or parameters.get('images',{}).get('voice-story/story')!=approved['images']['story']
        or retained.get('target')!=state['input_target'] or retained.get('contract')!=state['contract']
        or retained.get('migrations')!=state['migrations'] or retained.get('nonnats')!=state['nonnats']):
        raise Blocked('paused_recovery_retained_target_changed')
    producer=PausedOriginalProducer(stage,state)
    with tempfile.TemporaryDirectory(prefix='recompile-',dir=base) as temporary:
        output=Path(temporary)/('target-'+SOURCE+'.json')
        compiler.main([str(workspace),str(build/'parameters.json'),str(output)],producer=producer)
        output.chmod(0o600);compiled=decode(private_bytes(output,2<<20))
    if compiled!=retained:raise Blocked('paused_recovery_compiled_target_changed')
    original_stage=copy.copy(stage);original_stage.snapshots=copy.deepcopy(stage.original_snapshots)
    rows,target=image_only_documents(original_stage,compiled['manifests'],compiled['target'])
    actual_manifest=owned_bytes(Path(base)/'apply-manifests.json',2<<20,private=False)
    expected_manifest=json.dumps(rows,sort_keys=True,indent=2).encode()+b'\n'
    if expected_manifest!=actual_manifest:raise Blocked('paused_recovery_apply_target_changed')
    target['manifest_sha256']=hashlib.sha256(actual_manifest).hexdigest()
    normalized=normalize_target(stage.kube,stage,rows,target)
    if normalized!=state['target']:raise Blocked('paused_recovery_normalized_target_changed')
    if decode(private_bytes(build/'parameters.json',2<<20))!=parameters:
        raise Blocked('paused_recovery_retained_target_changed')
    producer.verify();stage.verify_final_storage()
    return {'parameters_sha256':digest(parameters),'compiled_sha256':digest(compiled),
            'apply_sha256':hashlib.sha256(actual_manifest).hexdigest(),'normalized_sha256':digest(normalized)}

def resume_capture(kube,base,state,code,binding,approved,execution,*,verify_execution):
    """Continue the preserved closed boundary, never fence a new operation."""
    import datetime as dt
    import actor_root
    import compiler
    import guard
    import transaction
    import migrations
    import nonnats_runtime
    import nats_root_plan
    from root_main import revalidate_inputs
    from docker_runtime import DockerRuntime
    from preserve import capture_cut
    verify_adopted_binding(base,state,binding)
    original=decode(private_bytes(Path(base)/'recovery-original-checkpoint.json'))
    initial_checkpoint(original,decode(private_bytes(guard.ROOT/'installed'/'journal'/(NONCE+'.json'),65536)))
    if (state.get('phase')!='COLD_BACKUP' or state.get('status')!='BLOCKED'
        or state.get('fence_status')!='VERIFIED' or state.get('error')!='rollout_selected_store_custody_invalid'):
        raise Blocked('paused_recovery_current_boundary_invalid')
    original_target_unchanged(state,original);backup_outputs_absent(base,state)
    stage=transaction.reconstruct(kube,state,lambda event:None)
    stage.verify_final_storage()
    runtime=DockerRuntime(base,OPERATION);runtime.no_operation_containers(running_only=False)
    workspace=guard.ROOT/'installed'/'sources'/OPERATION
    target_proof=recompile_target(base,state,stage,approved)
    def fresh():
        verify_execution()
        verify_adopted_binding(base,state,binding);stage.verify_final_storage()
        if transaction.file_sha(Path(base)/'kernel')!=original['kernel_sha256']:
            raise Blocked('paused_recovery_kernel_changed')
        if recompile_target(base,state,stage,approved)!=target_proof:
            raise Blocked('paused_recovery_target_proof_changed')
        revalidate_inputs(kube,state['contract'],state['provenance'])
        actor_root.revalidate(kube,workspace,code,binding,stage,state['service_actor_services'],
                              compiler.decode_yaml,state['service_actor_authority'])
        if migrations.preflight(kube,state['migrations'],state['target']['mode'])!=state['migration_secret_metadata']:
            raise Blocked('paused_recovery_database_authority_changed')
        nonnats_runtime.verify(kube,state['nonnats'],state['nonnats_binding'],stage)
    fresh()
    accepted={}
    def validate(actual,broker,tree,row,manifest):
        current=nats_root_plan.closed_copy_preflight(kube,workspace,state['bootstrap_enrollment'],
            state['contract'],compiler.decode_yaml,actual,broker,tree)
        if (current['plan']!=original['nats_contract'] or current['binding']!=original['nats_contract_binding']
            or current['scripts']!=original['nats_target_scripts']):
            raise Blocked('paused_recovery_original_nats_authority_changed')
        fresh();accepted.update(current)
    state['paused_recovery_authority']={'source':{k:v for k,v in approved.items() if k!='source_files'},
        'execution':copy.deepcopy(execution),'target':target_proof,'original_operation':OPERATION}
    transaction.save(Path(base)/'checkpoint.json',state)
    try:
        cut=capture_cut(runtime,stage.final_path,stage.verify_final_storage,validate=validate)
        if not accepted:raise Blocked('paused_recovery_closed_proof_missing')
        runtime.no_operation_containers(running_only=False);fresh()
        state['cut']=cut
        transaction.save(Path(base)/'rollout-before-manifest.json',cut['manifest'])
        cut['manifest_sha256']=transaction.file_sha(Path(base)/'rollout-before-manifest.json')
        state['nats_contract_expires_at']=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=4)).isoformat()
        state['source_authority']={k:v for k,v in approved.items() if k!='source_files'}
        state['execution_authority']=copy.deepcopy(execution)
        state['context']=transaction.context(stage);state['phase']='AWAITING_OFF_NODE';state['status']='WAITING'
        state.pop('error',None);transaction.save(Path(base)/'checkpoint.json',state)
        return state
    except Exception as error:
        state['status']='BLOCKED';state['fence_status']='UNKNOWN';state['error']=transaction.safe_error(error)
        try:
            stage.verify_final_storage();state['fence_status']='VERIFIED'
        except Exception:pass
        state['context']=transaction.context(stage);transaction.save(Path(base)/'checkpoint.json',state)
        raise

def decode(raw):
    import guard
    try:return json.loads(raw,object_pairs_hook=guard.object_pairs)
    except (ValueError,TypeError):raise Blocked('paused_recovery_private_json_invalid') from None

def immutable_bytes(path,raw):
    if Path(path).exists():
        if private_bytes(path)!=raw:raise Blocked('paused_recovery_preserved_bytes_changed')
        return
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    try:
        with os.fdopen(fd,'wb') as stream:
            stream.write(raw);stream.flush();os.fsync(stream.fileno())
    finally:
        directory=os.open(Path(path).parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
        try:os.fsync(directory)
        finally:os.close(directory)

def preserve_original(base,journal_path,new_binding):
    """Caller holds global lock; no checkpoint/journal rewrite at this step."""
    import root_cli
    base=Path(base)
    if base!=root_cli.ROOT/('rollout-'+OPERATION):raise Blocked('paused_recovery_operation_path_invalid')
    preserved=base/'recovery-original-checkpoint.json'
    raw=private_bytes(preserved if preserved.exists() else base/'checkpoint.json')
    original=decode(raw);journal_raw=private_bytes(journal_path,65536)
    boundary=initial_checkpoint(original,decode(journal_raw))
    new_sha=digest(new_binding)
    if new_sha==V6_BINDING:raise Blocked('paused_recovery_replacement_unapproved')
    record={'schema':'voice-paused-cold-backup-adoption-v1',**boundary,
            'original_checkpoint_sha256':hashlib.sha256(raw).hexdigest(),
            'original_journal_sha256':hashlib.sha256(journal_raw).hexdigest(),
            'replacement_code_sha256':new_sha}
    immutable_bytes(preserved,raw)
    immutable_bytes(base/'recovery-adoption.json',json.dumps(record,sort_keys=True,separators=(',',':')).encode()+b'\n')
    return record,original

def verify_adopted_binding(base,state,binding):
    """Every later state consumer rechecks immutable originals, not a waiver."""
    import root_cli
    import guard
    base=Path(base)
    expired_revision=base/'expired-recovery-adoption.json'
    if expired_revision.exists() and decode(private_bytes(expired_revision,65536)).get('replacement_code_sha256')==digest(binding):
        import expired_recovery
        expired_recovery.verify_adopted_binding(base,state,binding)
        return
    if state.get('code_capture')!=binding and (base/'recovery-v9-revision.json').exists():
        import preserved_upload
        predecessor,_=preserved_upload.verify_helper_continuity(base,state,binding)
        return verify_adopted_binding(base,state,predecessor)
    if state.get('repair_adoption') is None:
        if state.get('code_capture')!=binding:raise Blocked('rollout_operation_code_changed')
        return
    if base!=root_cli.ROOT/('rollout-'+OPERATION):raise Blocked('paused_recovery_operation_path_invalid')
    raw=private_bytes(base/'recovery-original-checkpoint.json');original=decode(raw)
    journal_raw=private_bytes(guard.ROOT/'installed'/'journal'/(NONCE+'.json'),65536)
    boundary=initial_checkpoint(original,decode(journal_raw))
    adoption_raw=private_bytes(base/'recovery-adoption.json',65536)
    record=decode(adoption_raw)
    predecessor=binding
    if state.get('repair_revision') is not None:
        import v8_upgrade
        predecessor=v8_upgrade.verify_revision(base,state,binding,adoption_raw)
    wanted={'schema':'voice-paused-cold-backup-adoption-v1',**boundary,
            'original_checkpoint_sha256':hashlib.sha256(raw).hexdigest(),
            'original_journal_sha256':hashlib.sha256(journal_raw).hexdigest(),
            'replacement_code_sha256':digest(predecessor)}
    if record!=wanted or state['repair_adoption']!=digest(record) or state.get('code_capture')!=binding:
        raise Blocked('paused_recovery_adoption_binding_changed')
    original_target_unchanged(state,original)
    if digest(root_cli.code_binding(base/'code-v6-preserved'))!=V6_BINDING:
        raise Blocked('paused_recovery_original_code_changed')

def backup_outputs_absent(base,state):
    if state.get('space_preflight') is not None or any(k in state for k in ('cut','space_backup','space_gate','cipher_binding','cipher_space_members')):
        raise Blocked('paused_recovery_partial_backup_present')
    for path in Path(base).iterdir():
        if (path.name.startswith(('rollout-before','post-apply-','space-backup','space-restore','out-','recompile-'))
            or path.name in ('copy-checkpoint.json','rollout-backup.cms','export','apply-authorization.json')):
            raise Blocked('paused_recovery_partial_backup_present')

def verify_existing_prebuild(root,path):
    """Verify previously enrolled fixed receipts; current marker is separate."""
    import prebuild_disposition as v3
    import prebuild_v5_disposition as v4
    from encrypted_cut import directory
    root=Path(root);path=Path(path);nonce=path.stem
    if nonce in v4.CASES:
        code=v4.V4_BINDING;run=v4.CASES[nonce];source=v4.SOURCE;services=['story']
        schema='voice-nats-v4-prebuild-disposition-v1'
    elif nonce.startswith(v3.OPERATION) and len(nonce)==64:
        code=v3.V3_BINDING;run=v3.RUN;source=v3.SOURCE;services=['user']
        schema='voice-nats-known-prebuild-disposition-v1'
    else:raise Blocked('paused_recovery_unknown_unfinished_request')
    raw=private_bytes(path,65536);journal=decode(raw)
    receipt=decode(private_bytes(root/'installed'/('prebuild-disposition-'+nonce[:12]+'.json'),65536))
    response_raw=owned_bytes(root/'installed'/'responses'/path.name,65536,private=False);response=decode(response_raw)
    wanted={'action':'prepare','nonce':nonce,'run_id':run,'source_sha':source,'mode':'images-only','changed_services':services}
    if (receipt.get('schema')!=schema or receipt.get('operation')!=nonce[:12] or receipt.get('run_id')!=run
        or receipt.get('original_journal')!=journal or journal.get('request')!=wanted
        or journal.get('phase')!='STARTED' or journal.get('request_sha256')!=digest(wanted)
        or receipt.get('code_sha256')!=code or receipt.get('marker')!=v3.MARKER
        or receipt.get('generation')!='r20260930a4' or receipt.get('pvc')!='voice-nats-jsdata-d202610040049430b'
        or receipt.get('operation_absent') is not True or receipt.get('build_absent') is not True
        or receipt.get('disposition')!='FAILED_BEFORE_TRANSACTION_NO_REPLAY'
        or receipt.get('journal_sha256')!=(hashlib.sha256(raw).hexdigest() if nonce in v4.CASES else digest(journal))
        or receipt.get('response_sha256')!=(hashlib.sha256(response_raw).hexdigest() if nonce in v4.CASES else digest(response))):
        raise Blocked('paused_recovery_prior_disposition_changed')
    directory(root/'installed'/'sources'/nonce[:12])
    v3.absent(root/('rollout-'+nonce[:12]));v3.absent(root/'installed'/'sources'/(nonce[:12]+'-build'))

def install_repair(code,installed,kube,version='v7'):
    """Close ingress for the entire exact adoption, including final rereads."""
    installed=Path(installed)
    for path in (installed,installed/'inbox',installed/'processing',installed/'journal',
                 installed/'responses',installed/'recovery',installed/'sources'):
        row=path.lstat()
        if (not stat.S_ISDIR(row.st_mode) or row.st_uid!=0 or row.st_mode&0o002
            or path.name!='inbox' and row.st_mode&0o020):
            raise Blocked('bridge_upgrade_directory_untrusted')
    inbox=installed/'inbox';inbox.chmod(0o700)
    try:
        if version=='v10':
            import expired_recovery
            return expired_recovery.install_helper(code,installed,kube,version)
        if version=='v9':
            import preserved_upload
            return preserved_upload.install_helper(code,installed,kube)
        if version=='v8':
            import v8_upgrade
            return v8_upgrade.install(code,installed,kube)
        if version!='v7':raise Blocked('bridge_upgrade_version_invalid')
        return _install_repair_closed(code,installed,kube)
    finally:inbox.chmod(0o1730)

def _install_repair_closed(code,installed,kube):
    """Exact human V6→V7 paused adoption under the existing global lock."""
    import grp
    import guard
    import root_cli
    import installer
    import transaction
    import story_witness
    from docker_runtime import DockerRuntime
    from root_main import save
    installed=Path(installed);base=root_cli.operation_path(str(guard.ROOT/('rollout-'+OPERATION)))
    binding=root_cli.code_binding(Path(code));record,original=preserve_original(base,installed/'journal'/(NONCE+'.json'),binding)
    current=decode(private_bytes(base/'checkpoint.json'))
    if current.get('repair_adoption') is None:
        if current!=original:raise Blocked('paused_recovery_checkpoint_changed_before_adoption')
    else:verify_adopted_binding(base,current,binding)
    stage=transaction.reconstruct(kube,original,lambda event:None)
    stage.verify_final_storage();backup_outputs_absent(base,original)
    DockerRuntime(base,OPERATION).no_operation_containers(running_only=False)
    marker=kube.get('configmap','voice-nats-generation')
    if marker['metadata']['uid']!=MARKER_UID or marker['data'].get('phase')!='rollout-capturing' or marker['data'].get('knownRolloutOperation')!=OPERATION:
        raise Blocked('paused_recovery_marker_changed')
    rows=list(guard.ROOT.glob('rollout-*'))
    if len(rows)>1000:raise Blocked('bridge_operation_inventory_bound')
    for path in rows:
        if root_cli.rollout_directory_kind(path)=='capture' or path==base:continue
        if decode(private_bytes(path/'checkpoint.json')).get('status') not in ('PASS','ROLLED_BACK'):
            raise Blocked('paused_recovery_other_operation_present')
    for path in installed.iterdir():
        if path.name in ('inbox','processing') and any(path.iterdir()):raise Blocked('bridge_upgrade_pending_request')
    for path in (installed/'journal').iterdir():
        if path.name==NONCE+'.json':continue
        if decode(private_bytes(path,65536)).get('phase')!='COMPLETE':verify_existing_prebuild(guard.ROOT,path)
    witnesses=story_witness.registry_snapshot();policy=private_bytes(installed/'policy.json',65536)
    key_bytes={name:owned_bytes(installed/'recovery'/name,65536,private=name=='recovery-key.pem') for name in ('recovery-key.pem','recovery-cert.pem')}
    gid=grp.getgrnam('pmd').gr_gid
    installer._upgrade_code(code,installed,gid,V6_BINDING,'v7','v6')
    installer._upgrade_code(code,base,gid,V6_BINDING,'v7','v6')
    if (story_witness.registry_snapshot()!=witnesses or private_bytes(installed/'policy.json',65536)!=policy
        or any(owned_bytes(installed/'recovery'/name,65536,private=name=='recovery-key.pem')!=raw for name,raw in key_bytes.items())
        or kube.get('configmap','voice-nats-generation')!=marker):
        raise Blocked('paused_recovery_upgrade_custody_changed')
    stage.verify_final_storage()
    verify_original_journal(installed/'journal'/(NONCE+'.json'),record)
    adopted=copy.deepcopy(original);adopted['code_capture']=binding;adopted['repair_adoption']=digest(record)
    save(base/'checkpoint.json',adopted,limit=128<<20)
    verify_adopted_binding(base,adopted,binding)
    return adopted
