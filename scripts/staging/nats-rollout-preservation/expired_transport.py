"""Root-owned dual ciphertext export/readback; immutable original cut retained."""
import copy
from datetime import datetime,timezone
import hashlib
import json
import os
from pathlib import Path
import stat
import expired_recovery as recovery
import preserved_upload as upload


def put(path,row):
    upload.exclusive(path,json.dumps(row,sort_keys=True,separators=(',',':')).encode())


def cipher_copy(source,target,binding):
    """Only ciphertext crosses the root-private/public export boundary."""
    before=upload.cipher(source,binding)
    source_fd=os.open(source,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    output_fd=None
    try:
        if upload.fingerprint(os.fstat(source_fd))!=before:recovery.reject('export_source_replaced')
        output_fd=os.open(target,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
        sha=hashlib.sha256();count=0
        while part:=os.read(source_fd,1<<20):
            count+=len(part)
            if count>binding['cipher_bytes']:recovery.reject('export_size_changed')
            sha.update(part);view=memoryview(part)
            while view:
                written=os.write(output_fd,view)
                if written<=0:recovery.reject('export_write_failed')
                view=view[written:]
        if count!=binding['cipher_bytes'] or sha.hexdigest()!=binding['cipher_sha256']:
            recovery.reject('export_cipher_changed')
        os.fchown(output_fd,0,1000);os.fchmod(output_fd,0o440);os.fsync(output_fd)
        if (upload.fingerprint(os.fstat(source_fd))!=before
            or upload.cipher(source,binding)!=before):recovery.reject('export_source_changed')
    finally:
        if output_fd is not None:os.close(output_fd)
        os.close(source_fd)
    upload.cipher(target,binding,export=True)


def export(base,state,slot,execution,observation_cipher):
    """No repeated/partial export is guessed complete or silently overwritten."""
    base=Path(base);slot=Path(slot);nonce=execution['nonce']
    if (base.name!='rollout-'+upload.OPERATION or slot.parent not in (base/'expired-recovery-proofs',base/'expired-finish-proofs')
        or slot.name!=nonce
        or not isinstance(nonce,str) or not __import__('re').fullmatch('[a-f0-9]{64}',nonce)):
        recovery.reject('export_path_invalid')
    checkpoint=upload.private_read(base/'checkpoint.json')
    if json.loads(checkpoint)!=state:recovery.reject('export_checkpoint_changed')
    producer=upload.private_json(slot/'producer.json')
    if (producer['cipher']!=observation_cipher or producer['operation']!=state['operation']
        or producer['original_checkpoint_sha256']!=hashlib.sha256(checkpoint).hexdigest()):
        recovery.reject('export_producer_changed')
    parent=base/'expired-export'
    if not parent.exists():parent.mkdir(mode=0o700);os.chown(parent,0,1000);parent.chmod(0o750)
    parent_fd,_=upload.directory(parent,gid=1000,modes={0o750});os.close(parent_fd)
    destination=parent/nonce;destination.mkdir(mode=0o700);os.chown(destination,0,1000);destination.chmod(0o750)
    bindings={}
    for role,source,binding in (('original',base/'rollout-backup.cms',state['cipher_binding']),
        ('observation',slot/'rollout-backup.cms',observation_cipher)):
        folder=destination/role;folder.mkdir(mode=0o700);os.chown(folder,0,1000);folder.chmod(0o750)
        cipher_copy(source,folder/'rollout-backup.cms',binding)
        # Actual fresh uploader identity is distinct from the retained capture.
        # Artifact custody has a closed transport schema. Keep the complete
        # authenticated cipher/payload descriptor in the ROOT record below;
        # never pass its extra private proof fields as remote artifact metadata.
        fresh={**{key:copy.deepcopy(binding[key]) for key in ('operation','cipher_sha256','cipher_bytes')},'run_id':execution['dispatcher_run_id'],
            'head_sha':execution['head_sha'],'created_at':datetime.now(timezone.utc).isoformat(),
            'challenge':__import__('secrets').token_hex(16)}
        bindings[role]=fresh
    record={'schema':'voice-expired-dual-upload-v1','operation':state['operation'],
        'execution':copy.deepcopy(execution),'original_cipher_binding':copy.deepcopy(state['cipher_binding']),
        'observation_cipher_binding':copy.deepcopy(observation_cipher),'artifact_bindings':bindings,
        'checkpoint_sha256':hashlib.sha256(checkpoint).hexdigest(),'producer_sha256':upload.digest(producer)}
    if upload.private_read(base/'checkpoint.json')!=checkpoint:recovery.reject('export_checkpoint_changed')
    put(slot/'upload.json',record)
    return record,{role:str(destination/role/'rollout-backup.cms') for role in bindings}


def readback(token,base,state,slot,execution,artifact_ids,clock=None):
    """Actual full ZIP bytes for both immutable root-bound ciphertexts."""
    import github_custody
    clock=clock or (lambda:datetime.now(timezone.utc))
    base=Path(base);slot=Path(slot);record_raw=upload.private_read(slot/'upload.json')
    record=json.loads(record_raw,object_pairs_hook=upload.pairs)
    checkpoint=upload.private_read(base/'checkpoint.json');producer=upload.private_json(slot/'producer.json')
    if (record['execution']!=execution or record['operation']!=state['operation']
        or record['checkpoint_sha256']!=hashlib.sha256(checkpoint).hexdigest()
        or json.loads(checkpoint)!=state or record['producer_sha256']!=upload.digest(producer)
        or record['original_cipher_binding']!=state['cipher_binding']
        or set(artifact_ids)!={'original','observation'}
        or any(type(value)!=int or value<=0 for value in artifact_ids.values())
        or artifact_ids['original']==artifact_ids['observation']):recovery.reject('readback_identity_changed')
    start=datetime.fromisoformat(producer['started_at'])
    deadline=start+__import__('datetime').timedelta(seconds=600)
    def fresh():
        if clock()>=deadline:recovery.reject('readback_expired')
        if (upload.private_read(base/'checkpoint.json')!=checkpoint
            or upload.private_read(slot/'upload.json')!=record_raw
            or upload.digest(upload.private_json(slot/'producer.json'))!=record['producer_sha256']):
            recovery.reject('readback_integrity_changed')
    receipts={}
    for role,source in (('original',base/'rollout-backup.cms'),('observation',slot/'rollout-backup.cms')):
        fresh();binding=record['artifact_bindings'][role]
        if (binding['run_id']!=execution['dispatcher_run_id'] or binding['head_sha']!=execution['head_sha']):
            recovery.reject('readback_fresh_upload_changed')
        receipts[role]=github_custody.verify_artifact(token,binding,source,artifact_ids[role])
        fresh()
        if receipts[role].get('verified') is not True:recovery.reject('readback_unverified')
    fresh()
    for role in receipts:
        put(slot/(role+'-readback.json'),receipts[role])
    return receipts


def commit_authority(base,state,slot,execution,*,clock=None):
    """Root builds short authority only from completed dual byte readback."""
    clock=clock or (lambda:datetime.now(timezone.utc))
    base=Path(base);slot=Path(slot)
    adopted=upload.private_json(base/'expired-recovery-adopted-checkpoint.json')
    if state!=adopted or upload.private_json(base/'checkpoint.json')!=adopted:
        recovery.reject('authority_checkpoint_progressed')
    record=upload.private_json(slot/'upload.json');producer=upload.private_json(slot/'producer.json')
    observation=upload.private_json(slot/'rollout-before-manifest.json')
    if (record['execution']!=execution or record['producer_sha256']!=upload.digest(producer)
        or producer['observation_sha256']!=upload.digest(observation)
        or record['original_cipher_binding']!=adopted['cipher_binding']):
        recovery.reject('authority_producer_changed')
    receipts={}
    for role in ('original','observation'):
        receipt=upload.private_json(slot/(role+'-readback.json'))
        binding=record['artifact_bindings'][role]
        if (receipt.get('schema')!='voice-nats-custody-v1' or receipt.get('verified') is not True
            or receipt.get('destination')!='github:Poryadok/VoiceRoot'
            or any(receipt.get(key)!=binding[key] for key in
                ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes'))
            or type(receipt.get('artifact_id'))!=int or receipt['artifact_id']<=0
            or binding['run_id']!=execution['dispatcher_run_id'] or binding['head_sha']!=execution['head_sha']):
            recovery.reject('authority_readback_unbound')
        receipts[role]=receipt
    if receipts['original']['artifact_id']==receipts['observation']['artifact_id']:
        recovery.reject('authority_artifacts_not_distinct')
    proof={'schema':'voice-expired-native-proof-v1','operation':state['operation'],
        'started_at':producer['started_at'],'completed_at':clock().isoformat(),
        'checkpoint_sha256':upload.digest(adopted),'original_cut_sha256':upload.digest(adopted['cut']),
        'original_cipher_sha256':upload.digest(adopted['cipher_binding']),
        'original_target_sha256':upload.digest(adopted['target']),
        'original_execution_sha256':upload.digest(adopted['execution_authority']),
        'context_sha256':upload.digest(adopted['context']),
        'admission_sha256':upload.digest(upload.private_json(slot/'admission.json')),
        'repair_sha256':upload.digest(upload.private_json(base/'expired-recovery-input-repair.json')),
        'source_sha256':upload.digest(upload.private_json(slot/'source.json')),
        'selected_inventory_sha256':observation['selected_inventory_sha256'],
        'native_disposition_sha256':upload.digest(observation['native_disposition']),
        'full_state_proof_sha256':upload.digest(observation),
        **{role+'_readback_sha256':upload.digest(receipts[role]) for role in receipts}}
    authority=recovery.authority(adopted,execution,proof,now=clock())
    put(slot/'proof.json',proof);put(slot/'authority.json',authority)
    return authority
