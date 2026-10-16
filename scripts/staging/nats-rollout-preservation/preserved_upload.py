"""Root-owned preserved-cipher upload identity; never refresh capture authority."""
import copy
from datetime import datetime,timezone,timedelta
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import stat

OPERATION='3340764a7d24'
ORIGINAL_RUN=37900273078
ORIGINAL_NONCE='6138b79b7fc09bbcf978ddf64269251259b9b360357634c49459e0309e2457af'
ORIGINAL_HEAD='8d0a51144b9aae00bd75a919b1be6f90ad3f62a8'
ORIGINAL_TARGET='73ca52699ddf6a9182e3407d5bbdedbc29c06dee'
EXECUTION_FIELDS={'dispatcher_run_id','run_attempt','head_sha','event','workflow_id','path','repository','nonce'}
CIPHER_FIELDS={'operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes','created_at'}
V8_BINDING='9acbc971711997c79ff7adc4a5ba10f603bf0c19ff1b8f99d67e00b802a3d9b6'


class UploadError(ValueError):pass


def reject(label):raise UploadError('preserved_upload_'+label)


def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


def timestamp(value):
    try:
        date=datetime.fromisoformat(value.replace('Z','+00:00'))
        if date.tzinfo is None:reject('timestamp_invalid')
        return date.astimezone(timezone.utc)
    except (AttributeError,TypeError,ValueError):reject('timestamp_invalid')


def identity(state,execution,*,now,challenge):
    """Called only after real dispatcher, source and root custody verification."""
    try:
        if (state['operation']!=OPERATION or state['phase']!='AWAITING_OFF_NODE'
            or state['status']!='WAITING' or state['fence_status']!='VERIFIED'
            or state['target']['tag']!=ORIGINAL_TARGET):reject('state_invalid')
        original=state['execution_authority'];cipher=state['cipher_binding']
        if (original['nonce']!=ORIGINAL_NONCE or original['dispatcher_run_id']!=ORIGINAL_RUN
            or original['head_sha']!=ORIGINAL_HEAD or set(cipher)!=CIPHER_FIELDS
            or cipher['operation']!=OPERATION or cipher['run_id']!=ORIGINAL_RUN
            or cipher['head_sha']!=ORIGINAL_HEAD or not re.fullmatch('[a-f0-9]{32}',cipher['challenge'])
            or not re.fullmatch('[a-f0-9]{64}',cipher['cipher_sha256'])
            or type(cipher['cipher_bytes']) is not int or not 0<cipher['cipher_bytes']<=64*1024**3):
            reject('original_identity_changed')
        if (set(execution)!=EXECUTION_FIELDS or execution['repository']!='Poryadok/VoiceRoot'
            or execution['workflow_id']!=263689731 or execution['event']!='workflow_dispatch'
            or execution['path']!='.github/workflows/staging-deploy.yml'
            or type(execution['dispatcher_run_id']) is not int or execution['dispatcher_run_id']<=ORIGINAL_RUN
            or type(execution['run_attempt']) is not int or execution['run_attempt']<=0
            or not re.fullmatch('[a-f0-9]{40}',execution['head_sha'])
            or not re.fullmatch('[a-f0-9]{64}',execution['nonce'])
            or execution['nonce']==ORIGINAL_NONCE or not re.fullmatch('[a-f0-9]{32}',challenge)):
            reject('dispatcher_invalid')
        original_time=timestamp(cipher['created_at']);expiry=timestamp(state['nats_contract_expires_at'])
        if now.tzinfo is None or original_time>now or now>=expiry:reject('original_proof_expired')
        deadline=min(now+timedelta(seconds=600),expiry)
        artifact={**copy.deepcopy(cipher),'challenge':challenge,'run_id':execution['dispatcher_run_id'],
            'head_sha':execution['head_sha'],'created_at':now.isoformat()}
        return {'schema':'voice-preserved-cipher-upload-identity-v1','operation':OPERATION,
            'original_cipher_binding':copy.deepcopy(cipher),'original_cipher_binding_sha256':digest(cipher),
            'original_cut_sha256':digest(state['cut']),'original_target_sha256':digest(state['target']),
            'original_execution_sha256':digest(original),'original_proof_expires_at':expiry.isoformat(),
            'upload_execution':copy.deepcopy(execution),'artifact_binding':artifact,
            'created_at':now.isoformat(),'deadline':deadline.isoformat()}
    except (KeyError,TypeError,AttributeError):reject('shape_invalid')


def verify_deadline(record,now):
    start=timestamp(record['created_at']);deadline=timestamp(record['deadline'])
    expiry=timestamp(record['original_proof_expires_at'])
    if now.tzinfo is None or not start<=now<deadline or deadline>min(start+timedelta(seconds=600),expiry):
        reject('deadline_expired')


def fingerprint(row):
    return (row.st_dev,row.st_ino,row.st_uid,row.st_gid,row.st_mode,row.st_nlink,
        row.st_size,row.st_mtime_ns,row.st_ctime_ns)


def pairs(values):
    row={}
    for key,value in values:
        if key in row:reject('duplicate_field')
        row[key]=value
    return row


def private_read(path,limit=128<<20):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        before=os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid!=0 or before.st_mode&0o077
            or before.st_nlink!=1 or not 1<=before.st_size<=limit):reject('private_custody_invalid')
        raw=b''
        while len(raw)<=before.st_size:
            part=os.read(fd,min(1<<20,before.st_size+1-len(raw)))
            if not part:break
            raw+=part
        if len(raw)!=before.st_size or fingerprint(before)!=fingerprint(os.fstat(fd)):
            reject('private_file_changed')
        return raw
    finally:os.close(fd)


def private_json(path,limit=128<<20):
    return json.loads(private_read(path,limit),object_pairs_hook=pairs)


def directory(path,*,gid=None,modes=None):
    path=Path(path)
    for parent in (*path.parents[::-1],path):
        row=parent.lstat()
        if not stat.S_ISDIR(row.st_mode) or row.st_uid!=0 or row.st_mode&0o022:
            reject('directory_custody_invalid')
    fd=os.open(path,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    row=os.fstat(fd)
    if fingerprint(row)!=fingerprint(path.lstat()) or gid is not None and row.st_gid!=gid or modes is not None and stat.S_IMODE(row.st_mode) not in modes:
        os.close(fd);reject('directory_custody_invalid')
    return fd,row


def cipher(path,binding,*,export=False):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        before=os.fstat(fd);mode=0o440 if export else 0o600
        if (not stat.S_ISREG(before.st_mode) or before.st_uid!=0 or before.st_nlink!=1
            or stat.S_IMODE(before.st_mode)!=mode or export and before.st_gid!=1000
            or before.st_size!=binding['cipher_bytes']):reject('cipher_custody_invalid')
        hasher=hashlib.sha256();count=0
        while part:=os.read(fd,1<<20):
            count+=len(part)
            if count>binding['cipher_bytes']:reject('cipher_size_changed')
            hasher.update(part)
        if count!=binding['cipher_bytes'] or hasher.hexdigest()!=binding['cipher_sha256']:
            reject('cipher_bytes_changed')
        if fingerprint(before)!=fingerprint(os.fstat(fd)) or fingerprint(before)!=fingerprint(Path(path).lstat()):
            reject('cipher_identity_changed')
        return fingerprint(before)
    finally:os.close(fd)


def exclusive(path,raw):
    """Commit one immutable root-private inode without replacing an existing file."""
    path=Path(path);pending=path.with_name(path.name+'.'+secrets.token_hex(16)+'.pending')
    fd=os.open(pending,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    try:
        with os.fdopen(fd,'wb') as stream:stream.write(raw);stream.flush();os.fsync(stream.fileno())
        os.link(pending,path,follow_symlinks=False)
    finally:
        pending.unlink()
    fd=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(fd)
    finally:os.close(fd)


def prepare(base,state,execution,installed,continuity,source_proof,*,now=None):
    """Caller holds global lock and has verified exact helper continuity/live fence.

    continuation/source proofs are derived by root, never fields in a request.
    Existing partial slots reject; no capture, encryption or checkpoint save here.
    """
    now=now or datetime.now(timezone.utc);base=Path(base);installed=Path(installed)
    if base.name!='rollout-'+OPERATION:reject('operation_path_invalid')
    before=private_read(base/'checkpoint.json');before_hash=hashlib.sha256(before).hexdigest()
    if json.loads(before,object_pairs_hook=pairs)!=state or continuity['original_checkpoint_sha256']!=before_hash:
        reject('checkpoint_changed')
    journal_path=installed/'journal'/(ORIGINAL_NONCE+'.json')
    response_path=installed/'responses'/(ORIGINAL_NONCE+'.json')
    journal_raw=private_read(journal_path,65536)
    journal=json.loads(journal_raw,object_pairs_hook=pairs)
    expected={'action':'resume-cold-backup','operation':OPERATION,'run_id':ORIGINAL_RUN,'nonce':ORIGINAL_NONCE}
    if journal.get('phase')!='COMPLETE' or journal.get('request')!=expected or journal.get('request_sha256')!=digest(expected):
        reject('original_journal_changed')
    # Public response is root:G1000/0440; read through stable FD, no raw output.
    fd=os.open(response_path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        row=os.fstat(fd)
        if not stat.S_ISREG(row.st_mode) or row.st_uid!=0 or row.st_gid!=1000 or stat.S_IMODE(row.st_mode)!=0o440 or row.st_nlink!=1 or not 1<=row.st_size<=65536:
            reject('original_response_custody_invalid')
        response_raw=os.read(fd,row.st_size+1)
        if len(response_raw)!=row.st_size or fingerprint(row)!=fingerprint(os.fstat(fd)):
            reject('original_response_changed')
    finally:os.close(fd)
    response=json.loads(response_raw,object_pairs_hook=pairs)
    if (response!=journal.get('result') or response.get('operation')!=OPERATION
        or response.get('phase')!='AWAITING_OFF_NODE' or response.get('status')!='WAITING'
        or response.get('fence_status')!='VERIFIED'):reject('original_response_changed')
    def original_unchanged():
        if private_read(base/'checkpoint.json')!=before or private_read(journal_path,65536)!=journal_raw:
            reject('original_state_changed')
        response_fd=os.open(response_path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
        try:
            current=os.fstat(response_fd)
            if (fingerprint(current)!=fingerprint(row) or fingerprint(response_path.lstat())!=fingerprint(row)
                or os.read(response_fd,current.st_size+1)!=response_raw
                or fingerprint(os.fstat(response_fd))!=fingerprint(row)):
                reject('original_response_changed')
        finally:os.close(response_fd)
    record=identity(state,execution,now=now,challenge=secrets.token_hex(16))
    root_fd,_=directory(base,gid=1000,modes={0o750});export_fd=None
    try:
        export_fd,export_row=directory(base/'export',gid=1000,modes={0o700,0o750})
        private_identity=cipher(base/'rollout-backup.cms',state['cipher_binding'])
        export_identity=cipher(base/'export/rollout-backup.cms',state['cipher_binding'],export=True)
        if private_identity[:2]==export_identity[:2]:reject('cipher_export_alias')
        record.update({'original_checkpoint_sha256':before_hash,'original_journal_sha256':hashlib.sha256(journal_raw).hexdigest(),
            'original_response_sha256':hashlib.sha256(response_raw).hexdigest(),'continuity_sha256':digest(continuity),
            'source_proof_sha256':digest(source_proof),'original_context_sha256':digest(state['context'])})
        uploads=base/'cipher-upload'
        if not uploads.exists():uploads.mkdir(mode=0o700)
        upload_fd,_=directory(uploads,modes={0o700})
        try:
            slots=list(uploads.iterdir())
            if len(slots)>1:reject('upload_inventory_conflict')
            slot=uploads/execution['nonce']
            if slots:
                if slots!=[slot]:reject('upload_execution_conflict')
                existing=private_json(slot/'record.json',65536)
                if existing['upload_execution']!=execution or existing['original_checkpoint_sha256']!=before_hash:
                    reject('upload_record_changed')
                verify_deadline(existing,now)
                if private_read(slot/'original-checkpoint.json')!=before:reject('upload_snapshot_changed')
                for key in ('original_journal_sha256','original_response_sha256','continuity_sha256','source_proof_sha256','original_context_sha256'):
                    if existing.get(key)!=record[key]:reject('upload_record_changed')
                original_unchanged()
                return existing
            verify_deadline(record,datetime.now(timezone.utc))
            if fingerprint(export_row)!=fingerprint(os.fstat(export_fd)) or fingerprint(export_row)!=fingerprint((base/'export').lstat()):
                reject('export_directory_changed')
            os.fchmod(export_fd,0o750)
            after=os.fstat(export_fd)
            if (after.st_dev,after.st_ino,after.st_uid,after.st_gid,stat.S_IMODE(after.st_mode))!=(export_row.st_dev,export_row.st_ino,0,1000,0o750):
                reject('export_permission_repair_failed')
            if cipher(base/'rollout-backup.cms',state['cipher_binding'])!=private_identity or cipher(base/'export/rollout-backup.cms',state['cipher_binding'],export=True)!=export_identity:
                reject('cipher_changed_during_export')
            original_unchanged()
            slot.mkdir(mode=0o700)
            exclusive(slot/'original-checkpoint.json',before)
            exclusive(slot/'record.json',json.dumps(record,sort_keys=True,separators=(',',':')).encode())
            return record
        finally:os.close(upload_fd)
    finally:
        if export_fd is not None:os.close(export_fd)
        os.close(root_fd)


def load(base,nonce,state,continuity,*,execution=None,now=None):
    if not isinstance(nonce,str) or not re.fullmatch('[a-f0-9]{64}',nonce):reject('nonce_invalid')
    base=Path(base);slot=base/'cipher-upload'/nonce
    directory_fd,_=directory(slot,modes={0o700})
    try:
        raw=private_read(slot/'record.json',65536);record=json.loads(raw,object_pairs_hook=pairs)
        snapshot_raw=private_read(slot/'original-checkpoint.json')
        snapshot=json.loads(snapshot_raw,object_pairs_hook=pairs)
        if (record['original_checkpoint_sha256']!=hashlib.sha256(snapshot_raw).hexdigest()
            or record['continuity_sha256']!=digest(continuity)
            or record['upload_execution']['nonce']!=nonce
            or state['operation']!=OPERATION or state['cipher_binding']!=record['original_cipher_binding']
            or digest(state['cut'])!=record['original_cut_sha256'] or digest(state['target'])!=record['original_target_sha256']
            or digest(state['execution_authority'])!=record['original_execution_sha256']):
            reject('record_original_binding_changed')
        expected=identity(snapshot,record['upload_execution'],now=timestamp(record['created_at']),
            challenge=record['artifact_binding']['challenge'])
        extra={'original_checkpoint_sha256','original_journal_sha256','original_response_sha256','continuity_sha256',
               'source_proof_sha256','original_context_sha256'}
        if set(record)!=set(expected)|extra or any(not isinstance(record[key],str) or not re.fullmatch('[a-f0-9]{64}',record[key]) for key in extra):
            reject('record_schema_changed')
        if any(record.get(key)!=value for key,value in expected.items()):reject('record_identity_changed')
        if digest(state['context'])!=record['original_context_sha256'] or state.get('nats_contract_expires_at')!=snapshot.get('nats_contract_expires_at'):
            reject('record_original_binding_changed')
        if execution is not None and execution!=record['upload_execution']:reject('upload_dispatcher_changed')
        verify_deadline(record,now or datetime.now(timezone.utc))
        return record,raw,snapshot_raw
    finally:os.close(directory_fd)


def result(base,state,record):
    binding=record['artifact_binding']
    return {'operation':OPERATION,'phase':'AWAITING_OFF_NODE','status':'WAITING','fence_status':'VERIFIED',
        'cipher_path':str(Path(base)/'export/rollout-backup.cms'),
        'artifact_name':'voice-nats-rollout-'+OPERATION+'-'+binding['challenge'],
        'challenge':binding['challenge'],'upload_nonce':record['upload_execution']['nonce'],
        'changed_services':','.join(state['target']['changed_services']),
        'deploy_mode':state['target']['mode'],'source_sha':state['target']['tag']}


def verify_artifact(token,base,state,continuity,nonce,execution,artifact_id,*,now=None):
    """Full byte readback of fresh upload against unchanged original private cipher."""
    import github_custody
    base=Path(base);clock=now or (lambda:datetime.now(timezone.utc))
    record,record_raw,snapshot_raw=load(base,nonce,state,continuity,execution=execution,now=clock())
    checkpoint=private_read(base/'checkpoint.json')
    if checkpoint!=snapshot_raw:reject('checkpoint_progressed_before_custody')
    original=cipher(base/'rollout-backup.cms',record['original_cipher_binding'])
    exported=cipher(base/'export/rollout-backup.cms',record['original_cipher_binding'],export=True)
    receipt=github_custody.verify_artifact(token,record['artifact_binding'],base/'rollout-backup.cms',artifact_id)
    # Recheck after complete readback; a transport deadline cannot extend authority.
    load(base,nonce,state,continuity,execution=execution,now=clock())
    if (private_read(base/'checkpoint.json')!=checkpoint
        or private_read(base/'cipher-upload'/nonce/'record.json',65536)!=record_raw
        or cipher(base/'rollout-backup.cms',record['original_cipher_binding'])!=original
        or cipher(base/'export/rollout-backup.cms',record['original_cipher_binding'],export=True)!=exported):
        reject('readback_integrity_changed')
    verify_deadline(record,clock())
    return {**receipt,'schema':'voice-preserved-cipher-custody-v1',
        'original_cipher_binding_sha256':record['original_cipher_binding_sha256'],
        'upload_record_sha256':hashlib.sha256(record_raw).hexdigest(),
        'upload_nonce':nonce,'upload_attempt':execution['run_attempt'],
        'original_capture_run_id':record['original_cipher_binding']['run_id'],
        'original_capture_head_sha':record['original_cipher_binding']['head_sha']}


def helper_revision(original_raw,old_binding,new_binding,journal_raw):
    """Only the exact completed V8 preserved cut may enroll the new helper."""
    state=json.loads(original_raw,object_pairs_hook=pairs)
    journal=json.loads(journal_raw,object_pairs_hook=pairs)
    expected={'action':'resume-cold-backup','operation':OPERATION,'run_id':ORIGINAL_RUN,'nonce':ORIGINAL_NONCE}
    try:
        if (digest(old_binding)!=V8_BINDING or digest(new_binding)==V8_BINDING
            or state['code_capture']!=old_binding or state['operation']!=OPERATION
            or state['phase']!='AWAITING_OFF_NODE' or state['status']!='WAITING' or state['fence_status']!='VERIFIED'
            or state['target']['tag']!=ORIGINAL_TARGET or state['target']['mode']!='images-only'
            or state['target']['changed_services']!=['story'] or state.get('custody') is not None
            or state.get('authorization') is not None or journal.get('phase')!='COMPLETE'
            or journal.get('request')!=expected or journal.get('request_sha256')!=digest(expected)
            or journal['result']['operation']!=OPERATION or journal['result']['phase']!='AWAITING_OFF_NODE'
            or journal['result']['status']!='WAITING' or journal['result']['fence_status']!='VERIFIED'
            or state['execution_authority']['nonce']!=ORIGINAL_NONCE
            or state['execution_authority']['dispatcher_run_id']!=ORIGINAL_RUN
            or state['execution_authority']['head_sha']!=ORIGINAL_HEAD
            or state['cipher_binding']['run_id']!=ORIGINAL_RUN or state['cipher_binding']['head_sha']!=ORIGINAL_HEAD
            or state['cipher_binding']['operation']!=OPERATION or set(state['cipher_binding'])!=CIPHER_FIELDS):
            reject('helper_predecessor_invalid')
        return {'schema':'voice-preserved-cipher-helper-revision-v1','operation':OPERATION,
            'previous_code_sha256':V8_BINDING,'replacement_code_sha256':digest(new_binding),
            'original_checkpoint_sha256':hashlib.sha256(original_raw).hexdigest(),
            'original_journal_sha256':hashlib.sha256(journal_raw).hexdigest(),
            'original_cipher_binding_sha256':digest(state['cipher_binding']),
            'original_cut_sha256':digest(state['cut']),'original_target_sha256':digest(state['target']),
            'original_execution_sha256':digest(state['execution_authority']),
            'original_source_authority_sha256':digest(state['source_authority']),
            'original_proof_expires_at':state['nats_contract_expires_at']}
    except (KeyError,TypeError):reject('helper_predecessor_invalid')


def verify_helper_continuity(base,state,binding):
    import root_cli
    import guard
    import paused_recovery
    base=Path(base)
    if base!=guard.ROOT/('rollout-'+OPERATION):reject('helper_operation_path_invalid')
    original_raw=private_read(base/'recovery-v8-awaiting-checkpoint.json')
    original=json.loads(original_raw,object_pairs_hook=pairs)
    old_binding=root_cli.code_binding(base/'code-v8-preserved')
    # This recurses only through the original V6→V7→V8 reader, never this revision.
    paused_recovery.verify_adopted_binding(base,original,old_binding)
    journal_raw=private_read(guard.ROOT/'installed'/'journal'/(ORIGINAL_NONCE+'.json'),65536)
    expected=helper_revision(original_raw,old_binding,binding,journal_raw)
    record=private_json(base/'recovery-v9-revision.json',65536)
    if record!=expected or state.get('code_capture')!=old_binding:reject('helper_revision_changed')
    for key in ('cipher_binding','cut','target','execution_authority','source_authority','nats_contract_expires_at'):
        if state.get(key)!=original.get(key):reject('helper_original_authority_changed')
    paused_recovery.original_target_unchanged(state,original)
    return old_binding,record


def install_helper(code,installed,kube):
    """Installer lock/inbox closure required. Never save/rebuild the checkpoint."""
    import guard
    import root_cli
    import paused_recovery as recovery
    import installer
    import transaction
    import story_witness
    from docker_runtime import DockerRuntime
    installed=Path(installed);base=root_cli.operation_path(str(guard.ROOT/('rollout-'+OPERATION)))
    new_binding=root_cli.code_binding(Path(code));checkpoint=base/'checkpoint.json'
    snapshot=base/'recovery-v8-awaiting-checkpoint.json';revision_path=base/'recovery-v9-revision.json'
    original_raw=private_read(snapshot if snapshot.exists() else checkpoint)
    original=json.loads(original_raw,object_pairs_hook=pairs)
    old_code=base/('code-v8-preserved' if (base/'code-v8-preserved').exists() else 'code')
    old_binding=root_cli.code_binding(old_code)
    recovery.verify_adopted_binding(base,original,old_binding)
    journal_path=installed/'journal'/(ORIGINAL_NONCE+'.json');journal_raw=private_read(journal_path,65536)
    record=helper_revision(original_raw,old_binding,new_binding,journal_raw)
    current_raw=private_read(checkpoint)
    if current_raw!=original_raw:reject('helper_install_progressed')
    if revision_path.exists() and private_json(revision_path,65536)!=record:reject('helper_partial_revision_changed')
    stage=transaction.reconstruct(kube,original,lambda event:None);stage.verify_final_storage()
    DockerRuntime(base,OPERATION).no_operation_containers(running_only=False)
    marker=kube.get('configmap','voice-nats-generation')
    if (marker['data'].get('phase')!='rollout-capturing' or marker['data'].get('knownRolloutOperation')!=OPERATION
        or marker['metadata']['resourceVersion']!='3073024'):reject('helper_fence_changed')
    for leaf in ('copy-checkpoint.json','rollout-before-manifest.json'):
        private_read(base/leaf)
    if private_json(base/'copy-checkpoint.json')['archive_sha256']!=original['cut']['manifest']['archive_sha256']:
        reject('helper_copy_binding_changed')
    if hashlib.sha256(private_read(base/'rollout-before-manifest.json')).hexdigest()!=original['cut']['manifest_sha256']:
        reject('helper_manifest_changed')
    if not transaction.verify_archive(base/'rollout-before.tar',original['cut']['manifest']):reject('helper_archive_changed')
    cipher(base/'rollout-backup.cms',original['cipher_binding'])
    cipher(base/'export/rollout-backup.cms',original['cipher_binding'],export=True)
    if (base/'apply-authorization.json').exists() or (base/'cipher-upload').exists():reject('helper_advanced_outputs_present')
    journals={path.name:private_read(path,65536) for path in (installed/'journal').iterdir()}
    responses={path.name:recovery.owned_bytes(path,65536,private=False) for path in (installed/'responses').iterdir()}
    for name,raw in journals.items():
        if name==recovery.NONCE+'.json':continue
        if name==__import__('v8_upgrade').FAILED_NONCE+'.json':
            __import__('v8_upgrade').failed_execution(raw,responses[name]);continue
        if json.loads(raw,object_pairs_hook=pairs).get('phase')!='COMPLETE':recovery.verify_existing_prebuild(guard.ROOT,installed/'journal'/name)
    for path in (installed/'inbox',installed/'processing'):
        if any(path.iterdir()):reject('helper_pending_request')
    rows=list(guard.ROOT.glob('rollout-*'))
    if len(rows)>1000:reject('helper_operation_inventory_bound')
    for path in rows:
        if path==base or root_cli.rollout_directory_kind(path)=='capture':continue
        if private_json(path/'checkpoint.json')['status'] not in ('PASS','ROLLED_BACK'):reject('helper_other_operation_active')
    witnesses=story_witness.registry_snapshot();policy=private_read(installed/'policy.json',65536)
    keys={name:recovery.owned_bytes(installed/'recovery'/name,65536,private=name=='recovery-key.pem')
        for name in ('recovery-key.pem','recovery-cert.pem')}
    if not snapshot.exists():exclusive(snapshot,original_raw)
    if not revision_path.exists():exclusive(revision_path,json.dumps(record,sort_keys=True,separators=(',',':')).encode())
    installer._upgrade_code(code,installed,1000,V8_BINDING,'v9','v8')
    installer._upgrade_code(code,base,1000,V8_BINDING,'v9','v8')
    if (private_read(checkpoint)!=current_raw or private_read(journal_path,65536)!=journal_raw
        or private_read(installed/'policy.json',65536)!=policy or story_witness.registry_snapshot()!=witnesses
        or kube.get('configmap','voice-nats-generation')!=marker
        or {p.name:private_read(p,65536) for p in (installed/'journal').iterdir()}!=journals
        or {p.name:recovery.owned_bytes(p,65536,private=False) for p in (installed/'responses').iterdir()}!=responses
        or any(recovery.owned_bytes(installed/'recovery'/n,65536,private=n=='recovery-key.pem')!=raw for n,raw in keys.items())):
        reject('helper_upgrade_integrity_changed')
    stage.verify_final_storage();verify_helper_continuity(base,original,new_binding)
    return original
