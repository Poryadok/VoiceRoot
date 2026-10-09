"""Fixed V7 predecessor chain for the original paused recovery operation."""
import hashlib
from pathlib import Path
from controller import Blocked
import paused_recovery as recovery

V7_BINDING='24c7f6c27d6f0f505bca6623a88ce92aa56f5beda25194b7703653878e8d2b62'
FAILED_NONCE='47f8095b54212181374eab46805638659721f575372aecc55c076a67995b6bd5'
FAILED_RUN=37824099794


def revision_record(previous_raw, adoption_raw, previous_binding, replacement):
    """Pure fixed-chain predicate; installer owns custody and immutable writes."""
    try:
        state=recovery.decode(previous_raw);adoption=recovery.decode(adoption_raw)
        if (recovery.digest(previous_binding)!=V7_BINDING
            or state['operation']!=recovery.OPERATION
            or state['phase']!='COLD_BACKUP' or state['status']!='BLOCKED'
            or state['fence_status']!='VERIFIED'
            or state['error']!='rollout_selected_store_custody_invalid'
            or state['code_capture']!=previous_binding
            or state['repair_adoption']!=recovery.digest(adoption)
            or adoption['schema']!='voice-paused-cold-backup-adoption-v1'
            or adoption['operation']!=recovery.OPERATION
            or adoption['original_code_sha256']!=recovery.V6_BINDING
            or adoption['replacement_code_sha256']!=V7_BINDING
            or state['target']['tag']!=recovery.SOURCE
            or state['target']['mode']!='images-only'
            or state['target']['changed_services']!=['story']
            or any(key in state for key in ('cut','cipher_binding','source_authority',
                                          'execution_authority','paused_recovery_authority'))
            or recovery.digest(replacement) in (V7_BINDING,recovery.V6_BINDING)):
            raise ValueError()
        return {'schema':'voice-paused-cold-backup-v8-revision-v1',
                'operation':recovery.OPERATION,
                'previous_code_sha256':V7_BINDING,
                'previous_checkpoint_sha256':hashlib.sha256(previous_raw).hexdigest(),
                'original_adoption_sha256':hashlib.sha256(adoption_raw).hexdigest(),
                'replacement_code_sha256':recovery.digest(replacement)}
    except (KeyError,TypeError,ValueError):
        raise Blocked('paused_recovery_v8_predecessor_invalid') from None


def verify_revision(base,state,binding,adoption_raw):
    """Both later state consumers use this through verify_adopted_binding."""
    import root_cli
    base=Path(base)
    previous_raw=recovery.private_bytes(base/'recovery-v7-checkpoint.json')
    previous=recovery.decode(previous_raw)
    previous_binding=root_cli.code_binding(base/'code-v7-preserved')
    wanted=revision_record(previous_raw,adoption_raw,previous_binding,binding)
    record=recovery.decode(recovery.private_bytes(base/'recovery-v8-revision.json',65536))
    if (record!=wanted or state.get('repair_revision')!=recovery.digest(record)
        or state.get('repair_adoption')!=previous['repair_adoption']
        or state.get('code_capture')!=binding):
        raise Blocked('paused_recovery_v8_revision_changed')
    recovery.original_target_unchanged(state,previous)
    return previous_binding


def failed_execution(raw,response_raw):
    """Only the exact observed pre-authority continuation is preserved."""
    request={'action':'resume-cold-backup','nonce':FAILED_NONCE,
             'operation':recovery.OPERATION,'run_id':FAILED_RUN}
    journal=recovery.decode(raw);response=recovery.decode(response_raw)
    if (journal.get('request')!=request or journal.get('request_sha256')!=recovery.digest(request)
        or journal.get('phase')!='STARTED' or journal.get('prepare_error')!='guard_rejected'
        or response.get('status')!='BLOCKED' or response.get('error')!='exception_Blocked'):
        raise Blocked('paused_recovery_v8_failed_execution_changed')


def install(code,installed,kube):
    """Called only under installer global lock with inbox already closed."""
    import copy
    import grp
    import guard
    import root_cli
    import installer
    import transaction
    import story_witness
    from docker_runtime import DockerRuntime
    from root_main import save
    installed=Path(installed);base=root_cli.operation_path(str(guard.ROOT/('rollout-'+recovery.OPERATION)))
    binding=root_cli.code_binding(Path(code))
    checkpoint=base/'checkpoint.json';preserved=base/'recovery-v7-checkpoint.json'
    previous_raw=recovery.private_bytes(preserved if preserved.exists() else checkpoint)
    previous=recovery.decode(previous_raw)
    # Before/after partial code replacement the immutable V7 directory is the
    # only valid predecessor; never accept arbitrary latest code or checkpoint.
    prior_code=base/('code-v7-preserved' if (base/'code-v7-preserved').exists() else 'code')
    previous_binding=root_cli.code_binding(prior_code)
    recovery.verify_adopted_binding(base,previous,previous_binding)
    adoption_raw=recovery.private_bytes(base/'recovery-adoption.json',65536)
    record=revision_record(previous_raw,adoption_raw,previous_binding,binding)
    adopted=copy.deepcopy(previous);adopted['code_capture']=binding;adopted['repair_revision']=recovery.digest(record)
    current_raw=recovery.private_bytes(checkpoint);current=recovery.decode(current_raw)
    repeated=current.get('repair_revision') is not None
    if not repeated:
        if current_raw!=previous_raw:raise Blocked('paused_recovery_v8_checkpoint_changed')
    else:
        recovery.verify_adopted_binding(base,current,binding)
        # The reader accepts later rollout progression; the installer may
        # repeat only the exact initial adoption, never replace advanced state.
        if current!=adopted:raise Blocked('paused_recovery_v8_install_progressed')
    recovery.backup_outputs_absent(base,current)
    stage=transaction.reconstruct(kube,current,lambda event:None);stage.verify_final_storage()
    DockerRuntime(base,recovery.OPERATION).no_operation_containers(running_only=False)
    marker=kube.get('configmap','voice-nats-generation')
    journals={}
    for path in (installed/'journal').iterdir():
        raw=recovery.private_bytes(path,65536);journals[path.name]=raw
        if path.name==recovery.NONCE+'.json':continue
        if path.name==FAILED_NONCE+'.json':
            failed_execution(raw,recovery.owned_bytes(installed/'responses'/path.name,65536,private=False));continue
        if recovery.decode(raw).get('phase')!='COMPLETE':recovery.verify_existing_prebuild(guard.ROOT,path)
    if FAILED_NONCE+'.json' not in journals:raise Blocked('paused_recovery_v8_failed_execution_missing')
    for path in (installed/'inbox',installed/'processing'):
        if any(path.iterdir()):raise Blocked('bridge_upgrade_pending_request')
    rows=list(guard.ROOT.glob('rollout-*'))
    if len(rows)>1000:raise Blocked('bridge_operation_inventory_bound')
    for path in rows:
        if path==base or root_cli.rollout_directory_kind(path)=='capture':continue
        if recovery.decode(recovery.private_bytes(path/'checkpoint.json')).get('status') not in ('PASS','ROLLED_BACK'):
            raise Blocked('paused_recovery_other_operation_present')
    witnesses=story_witness.registry_snapshot();policy=recovery.private_bytes(installed/'policy.json',65536)
    keys={n:recovery.owned_bytes(installed/'recovery'/n,65536,private=n=='recovery-key.pem')
          for n in ('recovery-key.pem','recovery-cert.pem')}
    recovery.immutable_bytes(preserved,previous_raw)
    import json
    recovery.immutable_bytes(base/'recovery-v8-revision.json',json.dumps(record,sort_keys=True,separators=(',',':')).encode()+b'\n')
    gid=grp.getgrnam('pmd').gr_gid
    installer._upgrade_code(code,installed,gid,V7_BINDING,'v8','v7')
    installer._upgrade_code(code,base,gid,V7_BINDING,'v8','v7')
    if (story_witness.registry_snapshot()!=witnesses or recovery.private_bytes(installed/'policy.json',65536)!=policy
        or kube.get('configmap','voice-nats-generation')!=marker
        or any(recovery.owned_bytes(installed/'recovery'/n,65536,private=n=='recovery-key.pem')!=raw for n,raw in keys.items())
        or {p.name:recovery.private_bytes(p,65536) for p in (installed/'journal').iterdir()}!=journals
        or recovery.private_bytes(base/'recovery-adoption.json',65536)!=adoption_raw
        or recovery.private_bytes(checkpoint)!=current_raw):
        raise Blocked('paused_recovery_v8_upgrade_custody_changed')
    stage.verify_final_storage();recovery.backup_outputs_absent(base,current)
    if repeated:
        recovery.verify_adopted_binding(base,current,binding)
        return current # retain the exact existing checkpoint bytes/inode
    save(checkpoint,adopted,limit=128<<20)
    recovery.verify_adopted_binding(base,adopted,binding)
    return adopted
