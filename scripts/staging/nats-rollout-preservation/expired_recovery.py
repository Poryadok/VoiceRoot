"""Exact recovery record validation; no live consumer admits this module yet.

Proof arguments are root-produced evidence, never request fields. Validating
hash links does not perform native proof/readback or admit a caller assertion.
"""
import copy
from datetime import timedelta
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import preserved_upload as upload


def classify_startup(before, after, before_census, after_census,
                     before_records, after_records, read_bytes, checksum, provenance):
    """Classify a CLOSED inventory using actual pinned-server checksum bytes.

    read_bytes(side, path) reads the corresponding verified private archive;
    checksum(stream, consumer, bytes) is the bounded pinned offline checker.
    Neither callback receives a selected store or starts a broker. This pure
    classifier is not authority: the producer must bind archives, full record
    census, observed startup and final selected-store freshness separately.
    """
    from rollout_census import semantic
    from docker_runtime import NATS_IMAGE
    from datetime import datetime
    def veto(): raise RecoveryError('expired_recovery_native_startup_invalid')
    if (set(provenance) != {'server_image','started_at','closed_at'}
            or provenance['server_image'] != NATS_IMAGE): veto()
    try:
        start=datetime.fromisoformat(provenance['started_at'].replace('Z','+00:00'))
        end=datetime.fromisoformat(provenance['closed_at'].replace('Z','+00:00'))
        if start.tzinfo is None or end.tzinfo is None or end < start: veto()
    except (KeyError, TypeError, ValueError): veto()
    if semantic(before_census) != semantic(after_census) or before_records != after_records: veto()
    if before['dirs'] != after['dirs'] or before_census['account'] != after_census['account']: veto()
    old={row['path']:row for row in before['files']}
    new={row['path']:row for row in after['files']}
    if len(old)!=len(before['files']) or len(new)!=len(after['files']) or set(old)!=set(new): veto()
    changed={path for path in old if old[path]!=new[path]}
    if not changed:
        return {'schema':'voice-native-startup-disposition-v1','branch':'EXACT', 'changed_paths':[]}
    account=before_census['account']
    consumers={(row['stream'],row['name']):row for row in before_census['consumers']}
    if len(consumers)!=len(before_census['consumers']): veto()
    accepted=set()
    config_fields={'name','deliver_policy','opt_start_seq','opt_start_time','ack_policy','ack_wait',
        'max_deliver','backoff','filter_subject','filter_subjects','replay_policy','rate_limit_bps',
        'sample_freq','max_waiting','max_ack_pending','headers_only','max_batch','max_expires',
        'max_bytes','inactive_threshold','num_replicas','mem_storage','metadata','pause_until'}
    for path in sorted(changed):
        if path in accepted: continue
        parts=path.split('/')
        if (len(parts)!=7 or parts[:2]!=['jetstream',account] or parts[2]!='streams'
                or parts[4]!='obs' or parts[6]!='meta.inf'): veto()
        stream,name=parts[3],parts[5]
        if not re.fullmatch(r'[A-Za-z0-9_-]{1,255}',stream) or not re.fullmatch(r'[A-Za-z0-9_-]{1,255}',name): veto()
        companion='/'.join(parts[:-1]+['meta.sum'])
        if companion not in changed: veto()
        row=consumers.get((stream,name))
        if not row or row['durable'] or row['inactive_threshold']<=0: veto()
        # Reopening must fit the complete ephemeral lifetime; no expiry waiver.
        if (end-start).total_seconds()*1_000_000_000 >= row['inactive_threshold']: veto()
        blobs=[]
        for side,inventory in (('before',old),('after',new)):
            raw=read_bytes(side,path); summed=read_bytes(side,companion)
            if not isinstance(raw,bytes) or len(raw)>256*1024 or not isinstance(summed,bytes): veto()
            if (inventory[path] != {'path':path,'size':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
                    or inventory[companion] != {'path':companion,'size':len(summed),'sha256':hashlib.sha256(summed).hexdigest()}): veto()
            if len(summed)!=16 or checksum(stream,name,raw)!=summed: veto()
            def unique(pairs):
                result={}
                for key,value in pairs:
                    if key in result: veto()
                    result[key]=value
                return result
            try: value=json.loads(raw,object_pairs_hook=unique,parse_constant=lambda _:veto())
            except (ValueError,UnicodeError): veto()
            if not isinstance(value,dict) or set(value)-config_fields-{'Name','Created'}: veto()
            blobs.append(value)
        original,current=blobs
        if original.get('Name')!=name or original.get('Created')!=row['created']: veto()
        if current.get('Name')!='' or current.get('Created')!='0001-01-01T00:00:00Z': veto()
        config={key:value for key,value in original.items() if key not in ('Name','Created')}
        target={key:value for key,value in current.items() if key not in ('Name','Created')}
        if config != target or config.get('name')!=name or config.get('mem_storage',False): veto()
        if upload.digest(config)!=row['config_sha256']: veto()
        accepted.update((path,companion))
    if accepted != changed: veto()
    return {'schema':'voice-native-startup-disposition-v1','branch':'PINNED_EPHEMERAL_METADATA_REWRITE',
            'changed_paths':sorted(accepted)}

HASH_FIELDS={'checkpoint_sha256','original_cut_sha256','original_cipher_sha256',
    'original_target_sha256','original_execution_sha256','context_sha256',
    'admission_sha256','repair_sha256','source_sha256','selected_inventory_sha256',
    'native_disposition_sha256','full_state_proof_sha256','original_readback_sha256',
    'observation_readback_sha256'}
PROOF_FIELDS=HASH_FIELDS|{'schema','operation','started_at','completed_at'}
V9_BINDING='23a055aca7f2fca8e068810fd5c98ccc6c08b51dfd5d287d6344e2c2787a9b11'
FAILED_REQUEST={'action':'authorize-preserved-upload','operation':upload.OPERATION,
    'artifact_id':11610996097,'upload_nonce':'02f20dcb7549ae659428504c7096900effec0d099bb5e3ae15314c5ad68fe959',
    'run_id':37917461677,'nonce':'f6995aab48da04b205315abb6be906725bb810edefe90b9a36f2b77b100fd60c'}


class RecoveryError(ValueError):pass


def reject(label):raise RecoveryError('expired_recovery_'+label)


def failed_execution(journal_raw,response_raw):
    """Disposition only; original failed journal/response are never rewritten.

    Native pre-intent admission must ALSO verify the actual event history and
    frozen checkpoint. This request record cannot establish absent side effects.
    """
    try:
        journal=json.loads(journal_raw,object_pairs_hook=upload.pairs)
        response=json.loads(response_raw,object_pairs_hook=upload.pairs)
        expected={'schema':'voice-nats-bridge-request-v1','request':FAILED_REQUEST,
            'request_sha256':upload.digest(FAILED_REQUEST),'phase':'STARTED','prepare_error':'unexpected'}
        if journal!=expected or response!={'status':'BLOCKED','error':'blocked_unclassified'}:
            reject('failed_execution_changed')
        return {'schema':'voice-expired-native-failed-execution-v1','operation':upload.OPERATION,
            'request_sha256':upload.digest(FAILED_REQUEST),'journal_sha256':hashlib.sha256(journal_raw).hexdigest(),
            'response_sha256':hashlib.sha256(response_raw).hexdigest(),
            'disposition':'FROZEN_FAILED_NATIVE_PRE_INTENT_NO_REPLAY'}
    except (KeyError,TypeError,AttributeError,json.JSONDecodeError):reject('failed_execution_shape_invalid')


def adoption_record(raw,previous_binding_sha256,replacement_binding):
    """Closed revision of an exact advanced snapshot; installer never saves CP."""
    try:
        state=json.loads(raw,object_pairs_hook=upload.pairs)
        if (previous_binding_sha256!=V9_BINDING or upload.digest(replacement_binding)==V9_BINDING
            or state['operation']!=upload.OPERATION or state['status']!='BLOCKED'
            or state['phase']!='NATS_CONTRACT_MIGRATION' or state['fence_status']!='VERIFIED'
            or state['target']['tag']!=upload.ORIGINAL_TARGET or state['target']['mode']!='images-only'
            or state['target']['changed_services']!=['story'] or state.get('authorization') is not None
            or state.get('nats_migration') is not None
            or state['cipher_binding']['run_id']!=upload.ORIGINAL_RUN
            or state['execution_authority']['nonce']!=upload.ORIGINAL_NONCE):reject('adoption_state_invalid')
        return {'schema':'voice-expired-native-helper-adoption-v1','operation':upload.OPERATION,
            'previous_code_sha256':V9_BINDING,'replacement_code_sha256':upload.digest(replacement_binding),
            'adopted_checkpoint_sha256':hashlib.sha256(raw).hexdigest(),
            'original_expires_at':state['nats_contract_expires_at'],
            'original_cipher_sha256':upload.digest(state['cipher_binding']),
            'original_cut_sha256':upload.digest(state['cut']),
            'original_target_sha256':upload.digest(state['target']),
            'original_execution_sha256':upload.digest(state['execution_authority'])}
    except (KeyError,TypeError,AttributeError,json.JSONDecodeError):reject('adoption_shape_invalid')


def verify_adopted_binding(base,state,binding):
    """Both existing state readers enter through paused_recovery, then here.

    Recheck the complete V9 reader against the immutable adopted snapshot, not
    the progressed checkpoint. Only immutable original authorities are compared
    to current state; phase/context/events must retain truthful later progress.
    """
    import guard
    import root_cli
    base=Path(base)
    if base!=guard.ROOT/('rollout-'+upload.OPERATION):reject('adoption_path_invalid')
    raw=upload.private_read(base/'expired-recovery-adopted-checkpoint.json')
    adopted=json.loads(raw,object_pairs_hook=upload.pairs)
    predecessor=root_cli.code_binding(base/'code-v9-preserved')
    if upload.digest(predecessor)!=V9_BINDING:reject('adoption_predecessor_changed')
    upload.verify_helper_continuity(base,adopted,predecessor)
    record=upload.private_json(base/'expired-recovery-adoption.json',65536)
    if record!=adoption_record(raw,V9_BINDING,binding):reject('adoption_record_changed')
    for key in ('operation','code_capture','cipher_binding','cut','execution_authority',
                'source_authority','nats_contract_expires_at','repair_adoption','repair_revision'):
        if state.get(key)!=adopted.get(key):reject('adoption_original_changed')
    import paused_recovery
    paused_recovery.original_target_unchanged(state,adopted)
    before=adopted.get('events',[]);after=state.get('events',[])
    if not isinstance(before,list) or not isinstance(after,list) or after[:len(before)]!=before:
        reject('adoption_event_history_changed')
    return record


def install_history(state,awaiting):
    """Passive exact pre-intent interval; no claim of absent startup effects."""
    from expired_proof import failed_startup_interval
    return failed_startup_interval(state,awaiting)


def install_helper(code,installed,kube,version):
    """Exact advanced adoption under install_repair ingress/global lock.

    Package-controlled version only. No checkpoint save, input repair, broker
    startup, fresh permission or replay occurs while replacing helper code.
    """
    import root_cli
    import guard
    import installer
    import transaction
    import paused_recovery
    import story_witness
    from docker_runtime import DockerRuntime
    if not isinstance(version,str) or not re.fullmatch('v[0-9]{1,2}',version) or int(version[1:])<=9:
        reject('helper_version_invalid')
    installed=Path(installed);base=root_cli.operation_path(str(guard.ROOT/('rollout-'+upload.OPERATION)))
    checkpoint=base/'checkpoint.json';snapshot=base/'expired-recovery-adopted-checkpoint.json'
    record_path=base/'expired-recovery-adoption.json'
    raw=upload.private_read(snapshot if snapshot.exists() else checkpoint)
    state=json.loads(raw,object_pairs_hook=upload.pairs)
    current_raw=upload.private_read(checkpoint)
    if current_raw!=raw:reject('helper_progressed')
    old_code=base/('code-v9-preserved' if (base/'code-v9-preserved').exists() else 'code')
    old_binding=root_cli.code_binding(old_code)
    if upload.digest(old_binding)!=V9_BINDING:reject('helper_predecessor_changed')
    upload.verify_helper_continuity(base,state,old_binding)
    awaiting=upload.private_json(base/'recovery-v8-awaiting-checkpoint.json',128<<20)
    install_history(state,awaiting)
    new_binding=root_cli.code_binding(Path(code));record=adoption_record(raw,V9_BINDING,new_binding)
    if record_path.exists() and upload.private_json(record_path,65536)!=record:reject('helper_revision_conflict')
    stage=transaction.reconstruct(kube,state,lambda event:None);stage.verify_final_storage()
    DockerRuntime(base,upload.OPERATION).no_operation_containers(running_only=False)
    marker=kube.get('configmap','voice-nats-generation')
    if (marker['metadata']['resourceVersion']!='3073024'
        or marker['data'].get('phase')!='rollout-capturing'
        or marker['data'].get('knownRolloutOperation')!=upload.OPERATION):reject('helper_fence_changed')
    # Binding and source record checks use the original frozen values. Expired
    # permission is retained as history; installer cannot issue fresh authority.
    if not transaction.verify_archive(base/'rollout-before.tar',state['cut']['manifest']):
        reject('helper_original_archive_changed')
    upload.cipher(base/'rollout-backup.cms',state['cipher_binding'])
    upload.cipher(base/'export/rollout-backup.cms',state['cipher_binding'],export=True)
    if (base/'apply-authorization.json').exists() or state.get('nats_migration') is not None:
        reject('helper_advanced_outputs_present')
    journals={p.name:upload.private_read(p,65536) for p in (installed/'journal').iterdir()}
    responses={p.name:paused_recovery.owned_bytes(p,65536,private=False) for p in (installed/'responses').iterdir()}
    failed_name=FAILED_REQUEST['nonce']+'.json'
    disposition=failed_execution(journals[failed_name],responses[failed_name])
    for name,value in journals.items():
        if name==failed_name:continue
        if name==paused_recovery.NONCE+'.json':continue
        if name==__import__('v8_upgrade').FAILED_NONCE+'.json':
            __import__('v8_upgrade').failed_execution(value,responses[name]);continue
        if json.loads(value,object_pairs_hook=upload.pairs).get('phase')!='COMPLETE':
            paused_recovery.verify_existing_prebuild(guard.ROOT,installed/'journal'/name)
    for path in (installed/'inbox',installed/'processing'):
        if any(path.iterdir()):reject('helper_pending_request')
    operations=list(guard.ROOT.glob('rollout-*'))
    if len(operations)>1000:reject('helper_operation_inventory_bound')
    for path in operations:
        if path==base or root_cli.rollout_directory_kind(path)=='capture':continue
        if upload.private_json(path/'checkpoint.json')['status'] not in ('PASS','ROLLED_BACK'):
            reject('helper_other_operation_active')
    policy=upload.private_read(installed/'policy.json',65536);witnesses=story_witness.registry_snapshot()
    keys={n:paused_recovery.owned_bytes(installed/'recovery'/n,65536,private=n=='recovery-key.pem')
        for n in ('recovery-key.pem','recovery-cert.pem')}
    if not snapshot.exists():upload.exclusive(snapshot,raw)
    if not record_path.exists():upload.exclusive(record_path,json.dumps(record,sort_keys=True,separators=(',',':')).encode())
    disposition_path=base/'expired-recovery-failed-execution.json'
    if disposition_path.exists():
        if upload.private_json(disposition_path,65536)!=disposition:reject('helper_failed_disposition_changed')
    else:upload.exclusive(disposition_path,json.dumps(disposition,sort_keys=True,separators=(',',':')).encode())
    installer._upgrade_code(code,installed,1000,V9_BINDING,version,'v9')
    installer._upgrade_code(code,base,1000,V9_BINDING,version,'v9')
    if (upload.private_read(checkpoint)!=current_raw or upload.private_read(snapshot)!=raw
        or {p.name:upload.private_read(p,65536) for p in (installed/'journal').iterdir()}!=journals
        or {p.name:paused_recovery.owned_bytes(p,65536,private=False) for p in (installed/'responses').iterdir()}!=responses
        or upload.private_read(installed/'policy.json',65536)!=policy
        or story_witness.registry_snapshot()!=witnesses
        or any(paused_recovery.owned_bytes(installed/'recovery'/n,65536,private=n=='recovery-key.pem')!=value for n,value in keys.items())
        or kube.get('configmap','voice-nats-generation')!=marker):reject('helper_final_integrity_changed')
    stage.verify_final_storage();verify_adopted_binding(base,state,new_binding)
    return state


def authority(state,execution,proof,*,now):
    """Build a deterministic record only from separately verified root evidence."""
    try:
        if (state['operation']!=upload.OPERATION or state['phase']!='NATS_CONTRACT_MIGRATION'
            or state['status']!='BLOCKED' or state['fence_status']!='VERIFIED'
            or state['target']['tag']!=upload.ORIGINAL_TARGET
            or state['target']['mode']!='images-only' or state['target']['changed_services']!=['story']
            or state.get('authorization') is not None or state.get('nats_migration') is not None):
            reject('state_invalid')
        original=state['execution_authority'];cipher=state['cipher_binding']
        if (original['nonce']!=upload.ORIGINAL_NONCE or original['dispatcher_run_id']!=upload.ORIGINAL_RUN
            or original['head_sha']!=upload.ORIGINAL_HEAD or cipher['operation']!=upload.OPERATION
            or cipher['run_id']!=upload.ORIGINAL_RUN or cipher['head_sha']!=upload.ORIGINAL_HEAD):
            reject('original_identity_changed')
        if (set(execution)!=upload.EXECUTION_FIELDS or execution['repository']!='Poryadok/VoiceRoot'
            or execution['workflow_id']!=263689731 or execution['event']!='workflow_dispatch'
            or execution['path']!='.github/workflows/staging-deploy.yml'
            or type(execution['dispatcher_run_id']) is not int or execution['dispatcher_run_id']<=37917461677
            or type(execution['run_attempt']) is not int or execution['run_attempt']<=0
            or not re.fullmatch('[a-f0-9]{40}',execution['head_sha'])
            or not re.fullmatch('[a-f0-9]{64}',execution['nonce']) or execution['nonce']==upload.ORIGINAL_NONCE):
            reject('dispatcher_invalid')
        if (set(proof)!=PROOF_FIELDS or proof['schema']!='voice-expired-native-proof-v1'
            or proof['operation']!=upload.OPERATION
            or any(not isinstance(proof[k],str) or not re.fullmatch('[a-f0-9]{64}',proof[k]) for k in HASH_FIELDS)):
            reject('proof_shape_invalid')
        bindings={'checkpoint_sha256':upload.digest(state),'original_cut_sha256':upload.digest(state['cut']),
            'original_cipher_sha256':upload.digest(cipher),'original_target_sha256':upload.digest(state['target']),
            'original_execution_sha256':upload.digest(original),'context_sha256':upload.digest(state['context'])}
        if any(proof[k]!=v for k,v in bindings.items()):reject('proof_binding_changed')
        start=upload.timestamp(proof['started_at']);complete=upload.timestamp(proof['completed_at'])
        old_expiry=upload.timestamp(state['nats_contract_expires_at']);deadline=start+timedelta(seconds=600)
        if now.tzinfo is None or not old_expiry<=start<=complete<=now<deadline:
            reject('proof_time_invalid')
        return {'schema':'voice-expired-native-authority-v1','operation':upload.OPERATION,
            'purpose':'native-recovery','proof_sha256':upload.digest(proof),
            'adopted_state_sha256':upload.digest(state),'execution':copy.deepcopy(execution),
            'original_expires_at':state['nats_contract_expires_at'],
            'issued_at':now.isoformat(),'expires_at':deadline.isoformat()}
    except (KeyError,TypeError,AttributeError):reject('shape_invalid')


def verify_authority(record,state,proof,execution,*,now):
    try:
        expected=authority(state,execution,proof,now=upload.timestamp(record['issued_at']))
        if record!=expected:reject('authority_changed')
        if now.tzinfo is None or not upload.timestamp(record['issued_at'])<=now<upload.timestamp(record['expires_at']):
            reject('authority_expired')
    except (KeyError,TypeError,AttributeError):reject('authority_shape_invalid')


def enter(record,state,proof,execution,current_inventory,save_entry,start_broker,*,now,clock=None):
    """Caller holds fence/lock; save_entry must durably save truthful progress.

The inventory callback must inspect the actual CLOSED selected store. Archive
hashes are not a substitute. After save_entry failures retain prospective facts.
"""
    verify_authority(record,state,proof,execution,now=now)
    if current_inventory()!=proof['selected_inventory_sha256']:reject('selected_store_changed')
    verify_authority(record,state,proof,execution,now=clock() if clock is not None else now)
    save_entry({'kind':'expired_recovery_transaction_entered',
        'authority_sha256':upload.digest(record),'proof_sha256':upload.digest(proof),
        'selected_inventory_sha256':proof['selected_inventory_sha256']})
    if current_inventory()!=proof['selected_inventory_sha256']:reject('selected_store_changed_after_entry')
    verify_authority(record,state,proof,execution,now=clock() if clock is not None else now)
    return start_broker()


def runtime_authority(base,state,reference,*,now):
    """Load independent proof from root custody, never from request JSON.

    The immutable admitted checkpoint remains the authority input after durable
    transaction entry. Own later journal/context progression is retained.
    """
    import guard
    import root_cli
    base=Path(base)
    if base!=guard.ROOT/('rollout-'+upload.OPERATION):reject('runtime_path_invalid')
    binding=root_cli.code_binding(base/'code')
    verify_adopted_binding(base,state,binding)
    adopted=upload.private_json(base/'expired-recovery-adopted-checkpoint.json',128<<20)
    try:nonce=reference['execution']['nonce']
    except (KeyError,TypeError):reject('runtime_reference_invalid')
    if not isinstance(nonce,str) or not re.fullmatch(r'[a-f0-9]{64}',nonce):reject('runtime_nonce_invalid')
    slot=base/'expired-recovery-proofs'/nonce
    record=upload.private_json(slot/'authority.json',65536)
    proof=upload.private_json(slot/'proof.json',65536)
    if record!=reference:reject('runtime_authority_changed')
    verify_authority(record,adopted,proof,record['execution'],now=now)
    documents={'admission_sha256':slot/'admission.json',
        'repair_sha256':base/'expired-recovery-input-repair.json',
        'source_sha256':slot/'source.json',
        'full_state_proof_sha256':slot/'rollout-before-manifest.json',
        'original_readback_sha256':slot/'original-readback.json',
        'observation_readback_sha256':slot/'observation-readback.json'}
    for key,path in documents.items():
        if upload.digest(upload.private_json(path,128<<20))!=proof[key]:reject('runtime_evidence_changed')
    observation=upload.private_json(slot/'rollout-before-manifest.json',128<<20)
    if (observation['selected_inventory_sha256']!=proof['selected_inventory_sha256']
        or upload.digest(observation['native_disposition'])!=proof['native_disposition_sha256']
        or observation['original_cut_sha256']!=proof['original_cut_sha256']):reject('runtime_native_evidence_changed')
    for name in ('original-readback.json','observation-readback.json'):
        readback=upload.private_json(slot/name,65536)
        if readback.get('verified') is not True or readback.get('operation')!=upload.OPERATION:
            reject('runtime_readback_unverified')
    entered={'kind':'expired_recovery_transaction_entered','authority_sha256':upload.digest(record),
        'proof_sha256':upload.digest(proof),'selected_inventory_sha256':proof['selected_inventory_sha256']}
    entries=[event for event in state['events'] if event.get('kind')=='expired_recovery_transaction_entered']
    if entries:
        if entries!=[entered] or state.get('expired_recovery_authority')!=record:reject('runtime_entry_changed')
    elif upload.digest(state)!=upload.digest(adopted):reject('runtime_preentry_checkpoint_changed')
    return record,proof


def publish_apply_authority(base,state,reference,receipt,*,now):
    """Root-only fixed public projection; never a substitute proof producer."""
    import guard,root_cli
    base=Path(base)
    record,proof=runtime_authority(base,state,reference,now=now)
    if (receipt.get('expired_recovery_authority_sha256')!=upload.digest(record)
        or receipt['expires_at']!=record['expires_at']
        or proof['original_target_sha256']!=upload.digest(receipt['target'])):
        reject('apply_projection_receipt_changed')
    row={'schema':'voice-expired-native-apply-authority-v1','receipt_sha256':upload.digest(receipt),
        'helper_binding_sha256':upload.digest(root_cli.code_binding(base/'code')),
        'authority':copy.deepcopy(record),'proof':copy.deepcopy(proof)}
    path=base/'apply-recovery-authority.json'
    upload.exclusive(path,json.dumps(row,sort_keys=True,separators=(',',':')).encode())
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        info=os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_nlink!=1:reject('apply_projection_custody_invalid')
        os.fchown(fd,0,1000);os.fchmod(fd,0o440)
    finally:os.close(fd)
    runtime_authority(base,state,reference,now=__import__('datetime').datetime.now(__import__('datetime').timezone.utc))
    return row


class FinishLedger:
    """Root-owned missing-resume obligations; no migration/apply capability.

    Consumers supply independent authority and actual running/closed guards.
    This ledger never produces authority or resets a completed application.
    A safety pause is recorded only after its exact UID/spec pause is verified.
    """
    def __init__(self,record,verify_authority,verify_closed,verify_running,persist):
        if (set(record)!={'schema','operation','authority_sha256','pending','completed','nats_started'}
            or record['schema']!='voice-expired-finish-ledger-v1' or record['operation']!=upload.OPERATION
            or not isinstance(record['pending'],dict) or not isinstance(record['completed'],dict)
            or set(record['pending'])&set(record['completed']) or type(record['nats_started'])!=bool):
            reject('finish_ledger_invalid')
        self.record=copy.deepcopy(record)
        self.verify_authority=verify_authority;self.verify_closed=verify_closed
        self.verify_running=verify_running;self.persist=persist
    def guard(self,action,name=None,replicas=None):
        from stage_runtime import HUB
        self.verify_authority()
        if action=='release':
            if self.record['pending'] or not self.record['nats_started']:reject('finish_obligations_incomplete')
            self.verify_running(HUB);self.verify_authority();return
        if action=='ready' and name in self.record['completed']:
            # Re-observing readiness does not grant another scale/restart.
            self.verify_running(HUB);self.verify_running(name);self.verify_authority();return
        if action not in ('scale','ready') or name not in self.record['pending']:
            reject('finish_resume_obligation_missing')
        if action=='scale' and replicas!=1:reject('finish_resume_scale_invalid')
        obligation=self.record['pending'][name]
        if action=='scale' and obligation.get('resume_entered'):reject('finish_resume_already_entered')
        if action=='ready' and not obligation.get('resume_entered'):reject('finish_resume_not_entered')
        if not self.record['nats_started'] and action=='scale':
            if name!=HUB:reject('finish_nats_not_started')
            self.verify_closed()
        else:self.verify_running(HUB)
        self.verify_authority()
        if action=='scale':
            # Durable prospective transition before the actual scale CAS.
            obligation['resume_entered']=True
            self.persist(copy.deepcopy(self.record))
        else:
            self.verify_running(name)
            self.verify_authority()
            self.record['completed'][name]=self.record['pending'].pop(name)
            if name==HUB:self.record['nats_started']=True
            self.persist(copy.deepcopy(self.record))
    def safety_paused(self,name,verified_pause):
        """New proof-linked obligation, never reuse an old restart approval."""
        from stage_runtime import HUB
        if (name not in self.record['completed'] or set(verified_pause)!={'uid','template_sha256','replicas','proof_sha256'}
            or verified_pause['replicas']!=0
            or any(not isinstance(verified_pause[key],str) or not re.fullmatch(r'[a-f0-9]{64}',verified_pause[key])
                for key in ('template_sha256','proof_sha256'))):reject('finish_safety_pause_unbound')
        previous=self.record['completed'][name]
        if verified_pause['uid']!=previous['uid'] or verified_pause['template_sha256']!=previous['template_sha256']:
            reject('finish_safety_pause_identity_changed')
        del self.record['completed'][name]
        self.record['pending'][name]={**copy.deepcopy(previous),'safety_pause_proof_sha256':verified_pause['proof_sha256']}
        self.record['pending'][name].pop('resume_entered',None)
        if name==HUB:self.record['nats_started']=False
        self.persist(copy.deepcopy(self.record))


INPUT_FILES={'inputs/bootstrap.creds','inputs/social.creds','inputs/realtime.creds',
    'inputs/account.public','server.conf'}


def input_fingerprints(base,expected):
    """Read only fixed private inputs; return metadata, never credential bytes."""
    base=Path(base)
    if set(expected)!=INPUT_FILES:reject('input_binding_shape_invalid')
    rows={}
    for name in sorted(INPUT_FILES):
        parent=base/'inputs' if name.startswith('inputs/') else base
        parent_fd,_=upload.directory(parent)
        fd=None
        try:
            fd=os.open(Path(name).name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=parent_fd)
            row=os.fstat(fd)
            if (not stat.S_ISREG(row.st_mode) or row.st_uid!=0 or row.st_gid!=65532
                or stat.S_IMODE(row.st_mode)!=0o440 or row.st_nlink!=1 or not 1<=row.st_size<=256<<10):
                reject('input_file_custody_invalid')
            hasher=hashlib.sha256();count=0
            while part:=os.read(fd,65536):
                count+=len(part)
                if count>row.st_size:reject('input_file_changed')
                hasher.update(part)
            after=os.fstat(fd);named=os.stat(Path(name).name,dir_fd=parent_fd,follow_symlinks=False)
            if (count!=row.st_size or upload.fingerprint(row)!=upload.fingerprint(after)
                or upload.fingerprint(row)!=upload.fingerprint(named) or hasher.hexdigest()!=expected[name]):
                reject('input_file_binding_changed')
            rows[name]=list(upload.fingerprint(row))
        finally:
            if fd is not None:os.close(fd)
            os.close(parent_fd)
    return rows


def _input_bytes(path):
    """Bounded fixed root-owned runtime input; never emit its contents."""
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        before=os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid!=0 or before.st_gid!=65532
            or stat.S_IMODE(before.st_mode)!=0o440 or before.st_nlink!=1
            or not 1<=before.st_size<=256<<10):reject('input_file_custody_invalid')
        raw=b''
        while part:=os.read(fd,65536):
            raw+=part
            if len(raw)>before.st_size:reject('input_file_changed')
        if (len(raw)!=before.st_size or upload.fingerprint(before)!=upload.fingerprint(os.fstat(fd))
            or upload.fingerprint(before)!=upload.fingerprint(Path(path).lstat())):
            reject('input_file_changed')
        return raw
    finally:os.close(fd)


def repair_inputs(base,expected,admission_sha256):
    """Root holds lock and verified exact adoption/fence before calling.

    Re-materialize exact bound bytes in a new private runtime directory.
    Original input bytes AND directory custody remain immutable. An exclusive
    receipt closes repeats; incomplete copies are never adopted implicitly.
    """
    base=Path(base)
    if base.name!='rollout-'+upload.OPERATION or not re.fullmatch('[a-f0-9]{64}',admission_sha256):
        reject('input_repair_admission_invalid')
    base_fd,_=upload.directory(base,gid=1000,modes={0o750});inputs_fd=None
    try:
        inputs_fd,before=upload.directory(base/'inputs',gid=65532,modes={0o700,0o750})
        files=input_fingerprints(base,expected);receipt_path=base/'expired-recovery-input-repair.json'
        if receipt_path.exists():
            record=upload.private_json(receipt_path,65536)
            if (set(record)!={'schema','operation','admission_sha256','input_hashes','files','directory'}
                or record['schema']!='voice-expired-input-repair-v1' or record['operation']!=upload.OPERATION
                or record['admission_sha256']!=admission_sha256 or record['input_hashes']!=expected
                or record['files']!=files or record['directory']!=list(upload.fingerprint(before))
                ):reject('input_repair_repeat_changed')
            runtime_inputs(base,record)
            return record
        if stat.S_IMODE(before.st_mode)!=0o700:reject('input_repair_unrecorded_mode')
        if upload.fingerprint(before)!=upload.fingerprint(os.fstat(inputs_fd)):
            reject('input_directory_changed')
        runtime=base/'recovery-runtime-inputs';runtime.mkdir(mode=0o700)
        for name in sorted(expected):
            if not name.startswith('inputs/'):continue
            source=base/name;target=runtime/Path(name).name
            raw=_input_bytes(source)
            if hashlib.sha256(raw).hexdigest()!=expected[name]:reject('input_repair_integrity_changed')
            fd=os.open(target,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o440)
            try:
                with os.fdopen(fd,'wb',closefd=False) as output:output.write(raw);output.flush();os.fsync(output.fileno())
                os.fchown(fd,0,65532);os.fchmod(fd,0o440)
            finally:os.close(fd)
        os.chown(runtime,0,65532);runtime.chmod(0o750)
        after=os.fstat(inputs_fd)
        if (upload.fingerprint(after)!=upload.fingerprint(before)
            or upload.fingerprint(after)!=upload.fingerprint((base/'inputs').lstat())
            or input_fingerprints(base,expected)!=files):reject('input_repair_integrity_changed')
        record={'schema':'voice-expired-input-repair-v1','operation':upload.OPERATION,
            'admission_sha256':admission_sha256,'input_hashes':copy.deepcopy(expected),
            'files':files,'directory':list(upload.fingerprint(after))}
        import json
        upload.exclusive(receipt_path,json.dumps(record,sort_keys=True,separators=(',',':')).encode())
        return record
    finally:
        if inputs_fd is not None:os.close(inputs_fd)
        os.close(base_fd)


def runtime_inputs(base,record=None):
    """At-use integrity of the fixed rematerialized old-operation inputs."""
    base=Path(base);record=record or upload.private_json(base/'expired-recovery-input-repair.json',65536)
    if (base.name!='rollout-'+upload.OPERATION or record.get('schema')!='voice-expired-input-repair-v1'
        or record.get('operation')!=upload.OPERATION or set(record.get('input_hashes',{}))!=INPUT_FILES):
        reject('runtime_inputs_binding_invalid')
    if input_fingerprints(base,record['input_hashes'])!=record['files']:reject('runtime_inputs_original_changed')
    source_fd,original=upload.directory(base/'inputs',gid=65532,modes={0o700});os.close(source_fd)
    if record['directory']!=list(upload.fingerprint(original)):reject('runtime_inputs_original_directory_changed')
    target=base/'recovery-runtime-inputs';fd,_=upload.directory(target,gid=65532,modes={0o750});os.close(fd)
    wanted={Path(name).name for name in INPUT_FILES if name.startswith('inputs/')}
    if {p.name for p in target.iterdir()}!=wanted:reject('runtime_inputs_inventory_changed')
    for name in wanted:
        raw=_input_bytes(target/name)
        if hashlib.sha256(raw).hexdigest()!=record['input_hashes']['inputs/'+name]:reject('runtime_inputs_bytes_changed')
    return target
