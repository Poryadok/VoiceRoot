"""One specific human-root waiver closure; never a deployment or rollback PASS."""
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
sys.path.insert(0,str(Path(__file__).resolve().parent))
import guard
import root_cli
from controller import Blocked,READ,file_sha
from root_main import operation_lock,save
from stage_runtime import Staging,Kube,HUB,LEAVES,MARKER,pv_storage_path

OPERATION='3340764a7d24'
SNAPSHOT_SHA='c776c76492c6266eeaed8eae736224435814f68a76e2a57c892aa9de86925672'
V15_BINDING='10b5e8f3b5b8676bdc8d4e1f5432304933b3205e7a4a820e09730459ba923d2b'
FAILED_RETIREMENT_SHA='5a0e2c2437f3d6b96b57e5793d577d58d85ff0310996cf354dc658774ee5b7b6'
ORIGINAL_RETIREMENT_BINDING='2f90bfbc649b4492c3ca997f8cbaf31932a1033b883bdda06c40a68f658e5173'
CONFIGMAPS={'voice-app-config','voice-nats-analytics-chat-bootstrap','voice-nats-hub-config',
    'voice-nats-leaf-config','voice-nats-realtime-bootstrap'}
CORE={HUB,'voice-gateway',*('voice-'+s for s in LEAVES)}
WAIVER='CURRENT_STAGING_MESSAGES_AND_CONSUMERS_MAY_BE_LOST_SETUP_RETAINED'
FENCE_ABORT_OPERATION='32151c4cb431'
FENCE_ABORT_CHECKPOINT_SHA256='94b78d2ede79b1a899817aef34eee5c1001f75212e0d5b15e1c1cdc722f3e994'
FENCE_ABORT_SOURCE_SHA='e2af6b043b9afa44ea00fb8e564b04ed84e9627a'
FENCE_ABORT_RUN_ID=38380518143
FENCE_ABORT_NONCE='32151c4cb4316a31c57949222e0cf69208c8cd82a0bf32e307c371d34c9b2a59'
FENCE_ABORT_JOURNAL_SHA256='ca6c43b78e57ae5ebf71a9776db320438ee0763b9f94745bda155af0c1264f5e'
FENCE_ABORT_RESPONSE_SHA256='f823049f6a38c4b7c31cf64a19a65e4a611f4c8905df890e2e9291241c335d92'
FENCE_ABORT_POD_UID='c8ac408a-ef62-451b-a8c1-5a4393ec271a'
FENCE_ABORT_POD_NAME='voice-nats-pvc-candidate-cf9757747-dgktg'
FENCE_ABORT_FINALIZER='voice.io/nats-preservation-32151c4cb431'
FENCE_ABORT_NAMESPACE_UID='ac7abaf3-241f-483f-ac33-71086e4dfd32'
PENDING_ABANDONMENT_NAME='operator-pending-abandonment-v1.json'

def fail():raise Blocked('operator_retirement_binding_invalid')
def canonical(row):return hashlib.sha256(json.dumps(row,sort_keys=True,separators=(',',':')).encode()).hexdigest()

def private_bytes(path,limit=128<<20,*,group_read=False):
    path=Path(path);before=path.lstat();fd=os.open(path,READ)
    identity=lambda s:(s.st_dev,s.st_ino,s.st_mode,s.st_uid,s.st_gid,s.st_nlink,s.st_size,s.st_mtime_ns,s.st_ctime_ns)
    try:
        opened=os.fstat(fd)
        if (not stat.S_ISREG(opened.st_mode) or opened.st_uid!=0 or opened.st_mode&(0o037 if group_read else 0o077)
            or opened.st_nlink!=1 or not 0<opened.st_size<=limit or identity(before)!=identity(opened)):fail()
        chunks=[];size=0
        while raw:=os.read(fd,min(1<<20,limit-size+1)):
            chunks.append(raw);size+=len(raw)
            if size>limit:fail()
        if size!=opened.st_size or identity(opened)!=identity(os.fstat(fd)) or identity(opened)!=identity(path.lstat()):fail()
        return b''.join(chunks)
    finally:os.close(fd)

def document(path):return json.loads(private_bytes(path))
def exclusive(path,raw):
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    try:
        with os.fdopen(fd,'wb',closefd=False) as stream:
            stream.write(raw);stream.flush();os.fsync(fd)
    finally:os.close(fd)
    fd=os.open(Path(path).parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(fd)
    finally:os.close(fd)

def terminal_state(snapshot,events,receipt_sha):
    if (snapshot.get('operation')!=OPERATION or snapshot.get('phase')!='PAUSED_APPLY'
        or snapshot.get('status')!='WAITING' or snapshot.get('fence_status')!='VERIFIED'
        or len(snapshot.get('events',[]))!=83 or snapshot.get('nats_migration',{}).get('verified') is not True
        or not isinstance(events,list) or len(events)<2
        or events[0].get('kind')!='operator_retirement_intent'
        or events[-1].get('kind')!='operator_retirement_completed'
        or any(e.get('kind') not in {'operator_retirement_intent','operator_retirement_completed',
            'workload_scaled','user_cycle_override','user_cycle_restored','operator_retirement_continuation_intent'} for e in events)
        or not re.fullmatch('[a-f0-9]{64}',receipt_sha)):fail()
    result=copy.deepcopy(snapshot)
    result.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',
        events=copy.deepcopy(snapshot['events']+events),
        operator_retirement={'schema':'voice-operator-retirement-reference-v1',
            'snapshot_sha256':SNAPSHOT_SHA,'receipt_sha256':receipt_sha})
    return result

def validate_terminal(snapshot,state,events,receipt_sha):
    if state!=terminal_state(snapshot,events,receipt_sha):fail()

def journal_binding(installed,rows):
    if not isinstance(rows,dict) or len(rows)>100:fail()
    for name,wanted in rows.items():
        if not re.fullmatch('[a-f0-9]{64}\\.json',name) or set(wanted)!={'journal_sha256','request_sha256','response_sha256'}:fail()
        raw=private_bytes(installed/'journal'/name,2<<20);row=json.loads(raw)
        if hashlib.sha256(raw).hexdigest()!=wanted['journal_sha256'] or row.get('request_sha256')!=wanted['request_sha256']:fail()
        request=row.get('request',{})
        if canonical(request)!=wanted['request_sha256']:fail()
        if not (request.get('operation')==OPERATION or request.get('action')=='prepare' and request.get('nonce','')[:12]==OPERATION):fail()
        if hashlib.sha256(private_bytes(installed/'responses'/name,2<<20,group_read=True)).hexdigest()!=wanted['response_sha256']:fail()
    return rows

# These labels describe public predicates; exception arguments outside the
# fixed vocabulary never enter a checkpoint or operator receipt.
from errors import CODES as PUBLIC_CODES, TYPES as PUBLIC_TYPES, safe_error
READONLY_GUARDS=PUBLIC_CODES|frozenset(('rollout_old_container_attempt_missing',
    'rollout_old_image_inventory_invalid','rollout_old_image_identity_unusable',
    'rollout_old_revision_unconverged','unclassified_readonly_failure'))
READONLY_KEYS=frozenset(('schema','operation','phase','status','fence_status','events',
    'target','code_capture','contract','input_target','service_actor_authority',
    'renderer_authority','service_actor_services','migrations','migration_secret_metadata',
    'space_preflight','nonnats','nonnats_binding','context','native_capacity',
    'combined_space_capacity','provenance','bootstrap_enrollment','nats_contract',
    'nats_contract_binding','nats_target_scripts','nats_contract_expires_at','kernel_sha256'))
READONLY_ABSENT=('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json',
    'rollout-backup.cms','decrypted-restore.json','authorization.json','apply-authorization.json')


def _readonly_snapshot(base,snapshot):
    operation=snapshot.get('operation','');history=snapshot.get('events')
    if (not re.fullmatch('[a-f0-9]{12}',operation) or Path(base).name!='rollout-'+operation
        or snapshot.get('schema')!='nats-rollout-root-v1' or snapshot.get('phase')!='READ_ONLY_CAPTURE'
        or snapshot.get('status')!='RUNNING' or snapshot.get('fence_status')!='UNKNOWN'
        or not set(snapshot)<=READONLY_KEYS or not isinstance(history,list)
        or [e.get('kind') for e in history] not in ([],['staging_preflight'],
            ['staging_preflight','rollout_original_capture'])
        or any((Path(base)/name).exists() for name in READONLY_ABSENT)):fail()
    if 'context' in snapshot:
        ctx=snapshot['context'];marker=ctx.get('marker',{}).get('data',{})
        original=ctx.get('original_snapshots',{});captured=ctx.get('snapshots',{})
        if (marker.get('phase')!='active' or 'knownRolloutOperation' in marker
            or not original or captured!=original
            or any(row.get('spec',{}).get('replicas',1)!=1 for row in original.values())):fail()


def _readonly_error(error):
    name=type(error).__name__
    kind=name if name in PUBLIC_TYPES|{'SourceError','CustodyError'} else 'OtherError'
    guard_label=(error.args[0] if isinstance(error,Blocked) and len(error.args)==1
        and isinstance(error.args[0],str) and error.args[0] in READONLY_GUARDS
        else 'unclassified_readonly_failure')
    prepare_label=('guard_rejected' if kind=='Blocked' else
        'source_authority_rejected' if kind=='SourceError' else
        'transport_rejected' if kind=='CustodyError' else
        'exception_'+kind if kind in PUBLIC_TYPES else 'unexpected')
    return {'guard':guard_label,'exception_class':kind,
        'response_error':safe_error(error),'prepare_error':prepare_label}


def _readonly_result(snapshot,receipt):
    event={'kind':'read_only_capture_failed',**receipt['failure']}
    result=copy.deepcopy(snapshot)
    result.update(phase='FAILED_READ_ONLY',status='BLOCKED',fence_status='NOT_ACQUIRED',
        events=copy.deepcopy(snapshot['events'])+[event],error=receipt['failure']['guard'])
    result['failed_readonly']={'schema':'voice-failed-readonly-reference-v1',
        'snapshot_sha256':receipt['snapshot_sha256'],'receipt_sha256':canonical(receipt)}
    return result


def record_readonly_failure(base,snapshot,error,*,provenance=None,snapshot_raw=None):
    """Local immutable evidence only; never fence, refence or touch workloads."""
    base=Path(base);_readonly_snapshot(base,snapshot)
    raw=(json.dumps(snapshot,sort_keys=True,separators=(',',':')).encode()
        if snapshot_raw is None else snapshot_raw)
    if not isinstance(raw,bytes) or json.loads(raw)!=snapshot:fail()
    origin={'kind':'automatic_pre_fence'} if provenance is None else copy.deepcopy(provenance)
    if (origin!={'kind':'automatic_pre_fence'} and (set(origin)!={'kind','journals'}
        or origin.get('kind')!='operator_observed_failure' or not isinstance(origin.get('journals'),dict)
        or len(origin['journals'])!=1)):fail()
    receipt={'schema':'voice-failed-readonly-closure-v1','operation':snapshot['operation'],
        'snapshot_sha256':hashlib.sha256(raw).hexdigest(),
        'code_binding_sha256':canonical(snapshot['code_capture']),
        'deployment_pass':False,'rollback_eligible':False,'provenance':origin,
        'failure':_readonly_error(error)}
    exclusive(base/'failed-readonly-snapshot.json',raw)
    exclusive(base/'failed-readonly-receipt.json',json.dumps(receipt,sort_keys=True,separators=(',',':')).encode())
    return _readonly_result(snapshot,receipt)


def verify_readonly_terminal(base,state):
    base=Path(base);raw=private_bytes(base/'failed-readonly-snapshot.json')
    snapshot=json.loads(raw);receipt=document(base/'failed-readonly-receipt.json')
    _readonly_snapshot(base,snapshot)
    failure=receipt.get('failure',{});origin=receipt.get('provenance',{})
    if (set(receipt)!={'schema','operation','snapshot_sha256','code_binding_sha256',
            'deployment_pass','rollback_eligible','provenance','failure'}
        or receipt.get('schema')!='voice-failed-readonly-closure-v1'
        or receipt.get('operation')!=snapshot['operation']
        or receipt.get('snapshot_sha256')!=hashlib.sha256(raw).hexdigest()
        or receipt.get('code_binding_sha256')!=canonical(snapshot['code_capture'])
        or root_cli.code_binding(base/'code')!=snapshot['code_capture']
        or receipt.get('deployment_pass') is not False or receipt.get('rollback_eligible') is not False
        or set(failure)!={'guard','exception_class','response_error','prepare_error'}
        or failure.get('guard') not in READONLY_GUARDS
        or failure.get('exception_class') not in PUBLIC_TYPES|{'SourceError','CustodyError','OtherError'}
        or failure.get('response_error') not in PUBLIC_CODES|{'exception_'+n for n in PUBLIC_TYPES}|{'blocked_unclassified'}
        or failure.get('prepare_error') not in {'guard_rejected','source_authority_rejected','transport_rejected',
            'unexpected'}|{'exception_'+n for n in PUBLIC_TYPES}
        or state!=_readonly_result(snapshot,receipt)):fail()
    kind=failure['exception_class']
    expected_response=(failure['guard'] if kind=='Blocked' and failure['guard'] in PUBLIC_CODES else
        'exception_'+kind if kind in PUBLIC_TYPES else 'blocked_unclassified')
    expected_prepare=('guard_rejected' if kind=='Blocked' else
        'source_authority_rejected' if kind=='SourceError' else
        'transport_rejected' if kind=='CustodyError' else
        'exception_'+kind if kind in PUBLIC_TYPES else 'unexpected')
    if (failure['response_error']!=expected_response or failure['prepare_error']!=expected_prepare
        or kind!='Blocked' and failure['guard']!='unclassified_readonly_failure'):fail()
    if (origin!={'kind':'automatic_pre_fence'} and (set(origin)!={'kind','journals'}
        or origin.get('kind')!='operator_observed_failure' or not isinstance(origin.get('journals'),dict)
        or len(origin['journals'])!=1)):fail()
    installed=guard.ROOT/'installed';journals={}
    paths=list((installed/'journal').glob(snapshot['operation']+'*.json'))
    if len(paths)>100:fail()
    for path in paths:
        journal_raw=private_bytes(path,2<<20);journal=json.loads(journal_raw)
        request=journal.get('request',{})
        action=request.get('action')
        if action not in ('prepare','prepare-rollback'):continue
        import bridge
        if set(request)!={'action','nonce'}|(bridge.FIELDS[action]-{'token'}):fail()
        # Durable bridge journals intentionally omit the credential. Validate
        # the exact public shape with a noncredential temporary placeholder.
        try:bridge.validate(dict(request,token='public-journal-placeholder'))
        except bridge.BridgeError:fail()
        target_mode=snapshot.get('input_target',{}).get('mode')
        selected=(request.get('mode')==target_mode if action=='prepare' else
            target_mode=='images-only' and request['operation']!=snapshot['operation'])
        nonce=request.get('nonce','')
        response_raw=private_bytes(installed/'responses'/path.name,2<<20,group_read=True)
        if (not re.fullmatch('[a-f0-9]{64}',nonce) or path.name!=nonce+'.json' or nonce[:12]!=snapshot['operation']
            or not selected
            or journal.get('phase')!='STARTED' or journal.get('prepare_error')!=failure['prepare_error']
            or journal.get('request_sha256')!=canonical(request)
            or json.loads(response_raw)!={'status':'BLOCKED','error':failure['response_error']}):fail()
        journals[path.name]={'journal_sha256':hashlib.sha256(journal_raw).hexdigest(),
            'request_sha256':canonical(request),'response_sha256':hashlib.sha256(response_raw).hexdigest()}
    if len(journals)!=1 or origin.get('kind')=='operator_observed_failure' and origin['journals']!=journals:fail()
    # The bridge records its failure after the immutable automatic receipt.
    # Return the subsequently authenticated hashes for pending-request admission.
    return dict(receipt,journals=journals)


def verify_terminal(base,state):
    if 'operator_applied_abort' in state:return verify_applied_terminal(base,state)
    if 'operator_postmigration_abort' in state:return verify_postmigration_terminal(base,state)
    if 'operator_fence_abort' in state:return verify_fence_abort_terminal(base,state)
    if 'failed_readonly' in state:return verify_readonly_terminal(base,state)
    if 'operator_captured_abort' in state:return verify_captured_terminal(base,state)
    if 'operator_precut_abort' in state:return verify_precut_terminal(base,state)
    base=Path(base)
    if base.name!='rollout-'+OPERATION or state.get('operation')!=OPERATION:fail()
    raw=private_bytes(base/'operator-retirement-snapshot.json')
    if hashlib.sha256(raw).hexdigest()!=SNAPSHOT_SHA:fail()
    snapshot=json.loads(raw);receipt=document(base/'operator-retirement-receipt.json')
    reference=state.get('operator_retirement',{})
    if (reference.get('receipt_sha256')!=canonical(receipt)
        or receipt.get('schema')!='voice-operator-retirement-closure-v1'
        or receipt.get('operation')!=OPERATION or receipt.get('snapshot_sha256')!=SNAPSHOT_SHA
        or receipt.get('waiver')!=WAIVER or receipt.get('deployment_pass') is not False
        or receipt.get('rollback_eligible') is not False):fail()
    plan_raw=private_bytes(base/'operator-retirement-plan.json',2<<20);plan=json.loads(plan_raw)
    if (hashlib.sha256(plan_raw).hexdigest()!=receipt.get('plan_sha256')
        or set(plan)!={'schema','operation','checkpoint_sha256','code_binding_sha256','waiver','configmaps','journals'}
        or plan.get('schema')!='voice-specific-operator-retirement-plan-v1'
        or plan.get('waiver')!=WAIVER or plan.get('operation')!=OPERATION
        or plan.get('checkpoint_sha256')!=SNAPSHOT_SHA
        or plan.get('code_binding_sha256')!=receipt.get('code_binding_sha256')
        or plan.get('journals')!=receipt.get('journals') or set(plan.get('configmaps',{}))!=CONFIGMAPS):fail()
    validate_terminal(snapshot,state,receipt['events'],canonical(receipt))
    original=snapshot['context']['original_snapshots']
    continuation=receipt.get('continuation')
    if continuation is not None:
        verify_continuation(base,snapshot,plan_raw,receipt)
    elif any(e.get('kind')=='operator_retirement_continuation_intent' for e in receipt['events']):fail()
    setup=receipt.get('setup',{})
    if not isinstance(setup,dict) or set(setup)!=CORE:fail()
    expected={n:{'uid':r['metadata']['uid'],'template_sha256':canonical(r['spec']['template']),
        'replicas':1,'ready':setup[n].get('ready') if continuation is not None else True} for n,r in original.items()}
    if (set(original)!=CORE or setup!=expected or any(type(row.get('ready')) is not bool for row in setup.values())
        or setup[HUB]['ready'] is not True):fail()
    before=snapshot['context']['marker'];after=receipt.get('released_marker',{})
    wanted=copy.deepcopy(before['data']);wanted['phase']='active';wanted.pop('knownRolloutOperation',None)
    if (before['metadata']['resourceVersion']!='3154269' or before['data'].get('phase')!='rollout-prepared'
        or before['data'].get('knownRolloutOperation')!=OPERATION
        or after.get('uid')!=before['metadata']['uid'] or after.get('data')!=wanted
        or not isinstance(after.get('resourceVersion'),str) or after['resourceVersion']==before['metadata']['resourceVersion']
        or receipt.get('storage_verified') is not True or receipt.get('config_verified') is not True
        or receipt.get('user_cycle_restored') is not True):fail()
    journal_binding(guard.ROOT/'installed',receipt['journals'])
    return receipt


def _fence_abort_sequence(names):
    return [HUB,'voice-user-space-cycle','voice-user-template-restored',
        *sorted(names-{HUB,'voice-user','voice-space','voice-web','voice-gateway'}),
        'voice-web','voice-gateway']


def fence_abort_terminal(snapshot,receipt):
    """Construct the one exact ABORTED closure for the retained 32151 incident."""
    receipt_schema=receipt.get('schema')
    if receipt_schema not in ('voice-operator-fence-abort-closure-v1','voice-operator-fence-abort-closure-v2'):fail()
    version='v2' if receipt_schema.endswith('-v2') else 'v1'
    reference={'schema':'voice-operator-fence-abort-reference-'+version,
        'snapshot_sha256':receipt['snapshot_sha256'],'receipt_sha256':canonical(receipt)}
    result=copy.deepcopy(snapshot)
    result.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',
        events=copy.deepcopy(snapshot['events']+receipt['events']),operator_fence_abort=reference)
    return result


def _fence_abort_journal(installed,receipt):
    rows=receipt.get('journals')
    name=FENCE_ABORT_NONCE+'.json'
    wanted={'journal_sha256':FENCE_ABORT_JOURNAL_SHA256,
        'request_sha256':None,'response_sha256':FENCE_ABORT_RESPONSE_SHA256}
    if not isinstance(rows,dict) or set(rows)!={name} or set(rows[name])!=set(wanted):fail()
    raw=private_bytes(Path(installed)/'journal'/name,2<<20)
    row=json.loads(raw);request=row.get('request',{})
    expected={'action':'prepare','changed_services':['web'],'mode':'images-only',
        'nonce':FENCE_ABORT_NONCE,'run_id':FENCE_ABORT_RUN_ID,'source_sha':FENCE_ABORT_SOURCE_SHA}
    import bridge
    try:bridge.validate(dict(request,token='fixed-noncredential-validation-placeholder'))
    except bridge.BridgeError:fail()
    response_raw=private_bytes(Path(installed)/'responses'/name,2<<20,group_read=True)
    if (hashlib.sha256(raw).hexdigest()!=wanted['journal_sha256']
        or set(row)!={'schema','request_sha256','request','phase','prepare_stage','prepare_error'}
        or row.get('schema')!='voice-nats-bridge-request-v1' or request!=expected
        or row.get('request_sha256')!=canonical(request) or row.get('phase')!='STARTED'
        or row.get('prepare_stage')!={'name':'target-build','status':'COMPLETE'}
        or row.get('prepare_error')!='guard_rejected'
        or hashlib.sha256(response_raw).hexdigest()!=wanted['response_sha256']
        or json.loads(response_raw)!={'status':'BLOCKED','error':'command_failed'}
        or rows[name]['request_sha256']!=canonical(request)
        or rows[name]['response_sha256']!=wanted['response_sha256']):fail()
    return {name:{'journal_sha256':wanted['journal_sha256'],
        'request_sha256':canonical(request),'response_sha256':wanted['response_sha256']}}


def verify_fence_abort_terminal(base,state):
    """Verify the exact restored 32151 pre-cut abort; this is never rollout PASS."""
    base=Path(base)
    if base.name!='rollout-'+FENCE_ABORT_OPERATION or state.get('operation')!=FENCE_ABORT_OPERATION:fail()
    raw=private_bytes(base/'operator-fence-abort-snapshot.json')
    if hashlib.sha256(raw).hexdigest()!=FENCE_ABORT_CHECKPOINT_SHA256:fail()
    snapshot=json.loads(raw);receipt=document(base/'operator-fence-abort-receipt.json')
    reference_version='v2' if receipt.get('schema')=='voice-operator-fence-abort-closure-v2' else 'v1'
    reference={'schema':'voice-operator-fence-abort-reference-'+reference_version,
        'snapshot_sha256':FENCE_ABORT_CHECKPOINT_SHA256,'receipt_sha256':canonical(receipt)}
    names=CORE|{'voice-web'}
    context=snapshot.get('context',{});expected=context.get('expected',{})
    original=context.get('original_snapshots',{});captured=context.get('snapshots',{})
    marker=context.get('marker',{});marker_data=marker.get('data',{})
    if (snapshot.get('schema')!='nats-rollout-root-v1' or snapshot.get('phase')!='FENCE'
        or snapshot.get('status')!='BLOCKED' or snapshot.get('fence_status')!='UNKNOWN'
        or snapshot.get('target',{}).get('mode')!='images-only'
        or snapshot.get('target',{}).get('tag')!=FENCE_ABORT_SOURCE_SHA
        or set(snapshot)&{'cut','custody','authorization','apply','restart','preservation','closed_preservation'}
        or set(original)!=names or set(captured)!=names
        or expected.get('namespace_uid')!=FENCE_ABORT_NAMESPACE_UID
        or not isinstance(expected.get('source_claim'),str) or not re.fullmatch(r'[a-z0-9-]{1,63}',expected.get('source_claim',''))
        or expected.get('source_claim')!=context.get('final_claim',{}).get('metadata',{}).get('name')
        or expected.get('source_claim_uid')!=context.get('final_claim',{}).get('metadata',{}).get('uid')
        or expected.get('source_pv_uid')!=context.get('final_pv',{}).get('metadata',{}).get('uid')
        or expected.get('deployment_uids')!={n:r.get('metadata',{}).get('uid') for n,r in original.items()}
        or marker_data.get('phase')!='rollout-capturing'
        or marker_data.get('knownRolloutOperation')!=FENCE_ABORT_OPERATION
        or marker_data.get('dataPVC')!=expected.get('source_claim')):fail()
    history=snapshot.get('events')
    if not isinstance(history,list):fail()
    intents=[e for e in history if e.get('kind')=='original_hub_shutdown_intent']
    exits=[e for e in history if e.get('kind')=='original_hub_orderly_exit_verified']
    removals=[e for e in history if e.get('kind')=='original_hub_owned_finalizer_removed']
    allowed_history={'staging_preflight','rollout_original_capture','rollout_maintenance_owned',
        'workload_scaled','workload_fenced','original_hub_shutdown_intent','original_hub_orderly_exit_verified'}
    scaled=[e for e in history if e.get('kind')=='workload_scaled']
    if (any(not isinstance(e,dict) or e.get('kind') not in allowed_history for e in history)
        or len(intents)!=1 or len(exits)!=1 or removals
        or {e.get('name') for e in scaled}!=names or any(e.get('replicas')!=0 for e in scaled)
        or intents[0].get('pod_uid')!=FENCE_ABORT_POD_UID
        or intents[0].get('finalizer')!=FENCE_ABORT_FINALIZER
        or not re.fullmatch(r'containerd://[a-f0-9]{64}',intents[0].get('container_id',''))
        or not re.fullmatch('[a-f0-9]{64}',intents[0].get('live_census_sha256',''))
        or exits[0].get('proof',{}).get('pod_uid')!=FENCE_ABORT_POD_UID
        or exits[0].get('proof',{}).get('schema')!='voice-original-hub-orderly-exit-v1'
        or exits[0].get('proof',{}).get('source')!='KUBELET_CURRENT_TERMINATED_RETAINED_POD'
        or exits[0].get('proof',{}).get('exit_code')!=0
        or exits[0].get('proof',{}).get('container_id')!=intents[0].get('container_id')
        or type(exits[0].get('proof',{}).get('restart_count')) is not int
        or not isinstance(exits[0].get('proof',{}).get('started_at'),str)
        or not isinstance(exits[0].get('proof',{}).get('finished_at'),str)):fail()
    for name,row in original.items():
        current=captured[name]
        if (row.get('metadata',{}).get('uid')!=current.get('metadata',{}).get('uid')
            or row.get('spec',{}).get('replicas',1)!=1
            or current.get('spec',{}).get('replicas')!=0
            or row.get('spec',{}).get('template')!=current.get('spec',{}).get('template')):fail()
    if (set(receipt)!={'schema','operation','snapshot_sha256','code_binding_sha256','authorization',
        'deployment_pass','rollback_eligible','pod_finalizer','storage','restored','restore_sequence',
        'before_marker','released_marker','events','journals'}
        or receipt.get('schema') not in ('voice-operator-fence-abort-closure-v1','voice-operator-fence-abort-closure-v2')
        or receipt.get('operation')!=FENCE_ABORT_OPERATION
        or receipt.get('snapshot_sha256')!=FENCE_ABORT_CHECKPOINT_SHA256
        or receipt.get('code_binding_sha256')!=canonical(snapshot.get('code_capture'))
        or receipt.get('authorization')!='EXPLICIT_USER_AUTHORIZED_DISPOSABLE_STAGING_RECOVERY'
        or receipt.get('deployment_pass') is not False or receipt.get('rollback_eligible') is not False
        or state.get('operator_fence_abort')!=reference):fail()
    binding=root_cli.code_binding(base/'code')
    if binding!=snapshot.get('code_capture'):fail()
    pod=receipt.get('pod_finalizer',{});proof=exits[0]['proof']
    common={'pod_uid','finalizer','container_id','exit_code','before','after','resource_version_before','resource_version_after'}
    if (pod.get('pod_uid')!=FENCE_ABORT_POD_UID or pod.get('finalizer')!=FENCE_ABORT_FINALIZER
        or pod.get('container_id')!=proof.get('container_id') or pod.get('exit_code')!=0):fail()
    if receipt['schema']=='voice-operator-fence-abort-closure-v1':
        finals=pod.get('before')
        if (set(pod)!=common or not isinstance(finals,list) or finals.count(FENCE_ABORT_FINALIZER)!=1
            or pod.get('after')!=[f for f in finals if f!=FENCE_ABORT_FINALIZER]
            or not re.fullmatch('[0-9]+',pod.get('resource_version_before',''))
            or not re.fullmatch('[0-9]+',pod.get('resource_version_after',''))
            or int(pod['resource_version_after'])<=int(pod['resource_version_before'])):fail()
        finalizer_event={'kind':'operator_nats_finalizer_removed','pod_uid':FENCE_ABORT_POD_UID,
            'finalizer':FENCE_ABORT_FINALIZER}
    else:
        observation={'namespace_uid':FENCE_ABORT_NAMESPACE_UID,'pod_name':FENCE_ABORT_POD_NAME,
            'pod_uid':FENCE_ABORT_POD_UID}
        if (set(pod)!=common|{'outcome','pod_absence_observation'}
            or pod.get('outcome')!='CAS_RESPONSE_UNAVAILABLE_UID_ABSENT'
            or pod.get('before') is not None or pod.get('after') is not None
            or pod.get('resource_version_before') is not None or pod.get('resource_version_after') is not None
            or pod.get('pod_absence_observation')!=observation):fail()
        finalizer_event={'kind':'operator_nats_pod_uid_absence_verified','cas_response':'UNAVAILABLE',
            **observation}
    storage=receipt.get('storage',{});claim=context.get('final_claim',{});pv=context.get('final_pv',{})
    if (storage!={'namespace_uid':FENCE_ABORT_NAMESPACE_UID,'claim_uid':expected.get('source_claim_uid'),
        'pv_uid':expected.get('source_pv_uid'),'claim_spec_sha256':canonical(claim.get('spec')),
        'pv_spec_sha256':canonical(pv.get('spec')),'path':context.get('final_path')}):fail()
    restored=receipt.get('restored',{})
    wanted={name:{'uid':row['metadata']['uid'],'template_sha256':canonical(row['spec']['template']),
        'replicas':1,'ready':True} for name,row in original.items()}
    if restored!=wanted or receipt.get('restore_sequence')!=_fence_abort_sequence(names):fail()
    before=receipt.get('before_marker',{});released=receipt.get('released_marker',{})
    active=copy.deepcopy(marker_data);active['phase']='active';active.pop('knownRolloutOperation',None)
    rv=marker.get('metadata',{}).get('resourceVersion','')
    if (before!={'uid':marker.get('metadata',{}).get('uid'),'resourceVersion':rv,'data':marker_data}
        or set(released)!={'uid','resourceVersion','data'} or released.get('uid')!=before['uid']
        or released.get('data')!=active or not re.fullmatch('[0-9]+',rv)
        or not re.fullmatch('[0-9]+',released.get('resourceVersion',''))
        or int(released['resourceVersion'])<=int(rv)):fail()
    events=receipt.get('events');workload_events=[e for e in events[2:-2] if e.get('kind')=='operator_workload_restored'] if isinstance(events,list) else []
    expected_events=[{'kind':'operator_workload_restored','name':n,'uid':row['uid'],
        'template_sha256':row['template_sha256'],'replicas':1,'ready':True} for n,row in sorted(wanted.items())]
    if (not isinstance(events,list) or len(events)!=len(names)+4
        or events[0]!={'kind':'operator_fence_abort_intent'}
        or events[1]!=finalizer_event
        or workload_events!=expected_events
        or events[-2]!={'kind':'operator_marker_released','uid':before['uid']}
        or events[-1]!={'kind':'operator_fence_abort_completed'}):fail()
    journals=_fence_abort_journal(guard.ROOT/'installed',receipt)
    if state!=fence_abort_terminal(snapshot,dict(receipt,journals=journals)):fail()
    # Return the exact accepted closure augmented with journal pins for the idle installer.
    return dict(receipt,journals=journals)


def write_fence_abort_closure(base,snapshot_raw,receipt):
    """Persist the reviewed operation-specific root closure after observed recovery."""
    if os.geteuid()!=0 or sys.platform!='linux':fail()
    base=Path(base)
    if base.name!='rollout-'+FENCE_ABORT_OPERATION or not isinstance(snapshot_raw,bytes):fail()
    checkpoint_raw=private_bytes(base/'checkpoint.json')
    if (hashlib.sha256(checkpoint_raw).hexdigest()!=FENCE_ABORT_CHECKPOINT_SHA256
        or checkpoint_raw!=snapshot_raw):fail()
    snapshot=json.loads(snapshot_raw)
    receipt_raw=json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()
    exclusive(base/'operator-fence-abort-snapshot.json',snapshot_raw)
    exclusive(base/'operator-fence-abort-receipt.json',receipt_raw)
    terminal=fence_abort_terminal(snapshot,receipt)
    verify_fence_abort_terminal(base,terminal)
    save(base/'checkpoint.json',terminal,limit=128<<20)
    return terminal

def precut_retained_setup(snapshot):
    ctx=snapshot['context'];expected=ctx['expected']
    return {'namespace_uid':expected['namespace_uid'],
        'selected_claim':{'name':expected['source_claim'],'uid':expected['source_claim_uid'],'spec_sha256':canonical(ctx['final_claim']['spec'])},
        'selected_pv':{'uid':expected['source_pv_uid'],'spec_sha256':canonical(ctx['final_pv']['spec']),'path':ctx['final_path']},
        'service':{'uid':ctx['service']['metadata']['uid'],'object_sha256':canonical(ctx['service'])}}


def precut_configmaps(original):
    names=set(CONFIGMAPS)
    for row in original.values():
        spec=row['spec']['template']['spec']
        names.update(v['configMap']['name'] for v in spec.get('volumes',[]) if 'configMap' in v)
        names.update(s['configMap']['name'] for v in spec.get('volumes',[])
            for s in v.get('projected',{}).get('sources',[]) if 'configMap' in s)
        for c in spec.get('initContainers',[])+spec.get('containers',[]):
            names.update(v['configMapRef']['name'] for v in c.get('envFrom',[]) if 'configMapRef' in v)
            names.update(v['valueFrom']['configMapKeyRef']['name'] for v in c.get('env',[]) if 'configMapKeyRef' in v.get('valueFrom',{}))
    return names


def verify_precut_terminal(base,state):
    """Historical root-attested abort before any cold cut or application effect."""
    return _verify_unapplied_terminal(base,state,captured=False)


def verify_captured_terminal(base,state):
    """Abort an unapplied captured rollout; retained backup is never release PASS."""
    return _verify_unapplied_terminal(base,state,captured=True)


def _verify_unapplied_terminal(base,state,*,captured):
    kind='captured' if captured else 'precut';prefix='operator-'+kind+'-abort'
    reference_key='operator_'+kind+'_abort' 
    base=Path(base);operation=state.get('operation','')
    if not re.fullmatch('[a-f0-9]{12}',operation) or base.name!='rollout-'+operation:fail()
    raw=private_bytes(base/(prefix+'-snapshot.json'));snapshot=json.loads(raw)
    sha=hashlib.sha256(raw).hexdigest();receipt=document(base/(prefix+'-receipt.json'))
    reference={'schema':'voice-'+prefix+'-reference-v1','snapshot_sha256':sha,'receipt_sha256':canonical(receipt)}
    forbidden={'authorization','nats_migration','migration_jobs','migration_completed_metadata','space_postcondition',
        'renderer_applied','apply','restart','preservation','closed_preservation','closed_preservation_sealed',
        'operator_retirement','operator_precut_abort','operator_captured_abort'}
    if not captured:forbidden.update({'cut','custody','space_gate','space_backup'})
    absent=('authorization.json','apply-authorization.json') if captured else (
        'rollout-before.tar','rollout-before-manifest.json','authorization.json','apply-authorization.json','copy-checkpoint.json')
    if (state.get(reference_key)!=reference or snapshot.get('schema')!='nats-rollout-root-v1'
        or snapshot.get('operation')!=operation or snapshot.get('phase')!=('AWAITING_OFF_NODE' if captured else 'FENCE')
        or snapshot.get('status')!=('WAITING' if captured else 'BLOCKED') or snapshot.get('fence_status')!='VERIFIED'
        or any(k in snapshot for k in forbidden) or any((base/n).exists() for n in absent)
        or receipt.get('schema')!='voice-'+prefix+'-closure-v1' or receipt.get('operation')!=operation
        or receipt.get('snapshot_sha256')!=sha or receipt.get('waiver')!=WAIVER
        or receipt.get('deployment_pass') is not False or receipt.get('rollback_eligible') is not False):fail()
    binding=root_cli.code_binding(base/'code')
    if binding!=snapshot.get('code_capture') or canonical(binding)!=receipt.get('code_binding_sha256'):fail()
    if captured:verify_captured_evidence(base,snapshot,receipt)
    history=snapshot.get('events');events=receipt.get('events')
    allowed={'staging_preflight','rollout_original_capture','rollout_maintenance_owned','workload_scaled'}
    if captured:allowed.update({'original_hub_shutdown_intent','original_hub_orderly_exit_verified','original_hub_owned_finalizer_removed'})
    if (not isinstance(history,list) or not history or any(e.get('kind') not in allowed for e in history)
        or not isinstance(events,list) or len(events)<2 or events[0].get('kind')!='operator_'+kind+'_abort_intent'
        or events[-1].get('kind')!='operator_'+kind+'_abort_completed'):fail()
    original=snapshot['context']['original_snapshots'];names=CORE|{'voice-web','voice-admin','voice-developer-portal'}
    setup=receipt.get('setup',{})
    if set(original)!=names or not isinstance(setup,dict) or set(setup)!=names:fail()
    captured_snapshots=snapshot['context']['snapshots']
    if (set(captured_snapshots)!=names or any(r['spec'].get('replicas',1)!=1
        or captured_snapshots[n]['spec'].get('replicas',1)!=0 or captured_snapshots[n]['metadata']['uid']!=r['metadata']['uid']
        or captured_snapshots[n]['spec']['template']!=r['spec']['template'] for n,r in original.items())
        or any(e.get('kind')=='workload_scaled' and (e.get('name') not in names or e.get('replicas')!=0) for e in history)
        or {e['name'] for e in events[1:-1]}!=names or len(events)!=len(names)+2):fail()
    expected={n:{'uid':r['metadata']['uid'],'template_sha256':canonical(r['spec']['template']),
        'replicas':1,'ready':setup[n].get('ready')} for n,r in original.items()}
    if (setup!=expected or any(type(r['ready']) is not bool for r in setup.values()) or setup[HUB]['ready'] is not True
        or any(e.get('kind')!='workload_scaled' or e.get('name') not in names or e.get('replicas')!=1 for e in events[1:-1])
        or receipt.get('retained_setup')!=precut_retained_setup(snapshot)
        or receipt.get('storage_verified') is not True or receipt.get('config_verified') is not True):fail()
    configs=receipt.get('configmaps',{})
    if (not isinstance(configs,dict) or set(configs)!=precut_configmaps(original)
        or any(set(r)!={'uid','data_sha256','binary_data_sha256'} or not re.fullmatch('[a-f0-9-]{36}',r['uid'])
            or any(not re.fullmatch('[a-f0-9]{64}',r[k]) for k in ('data_sha256','binary_data_sha256')) for r in configs.values())):fail()
    marker=snapshot['context']['marker'];before={'uid':marker['metadata']['uid'],'resourceVersion':marker['metadata']['resourceVersion'],'data':marker['data']}
    after=receipt.get('released_marker',{});data=copy.deepcopy(before['data']);data['phase']='active';data.pop('knownRolloutOperation',None)
    if (receipt.get('before_marker')!=before or before['data'].get('phase')!='rollout-capturing'
        or before['data'].get('knownRolloutOperation')!=operation or set(after)!={'uid','resourceVersion','data'}
        or after.get('uid')!=before['uid'] or after.get('data')!=data or not isinstance(after.get('resourceVersion'),str)
        or after['resourceVersion']==before['resourceVersion']):fail()
    journals=receipt.get('journals',{})
    if not isinstance(journals,dict) or not 1<=len(journals)<=100:fail()
    for name,wanted in journals.items():
        if not re.fullmatch('[a-f0-9]{64}\\.json',name) or set(wanted)!={'journal_sha256','request_sha256','response_sha256'}:fail()
        installed=guard.ROOT/'installed';raw=private_bytes(installed/'journal'/name,2<<20);journal=json.loads(raw)
        request=journal.get('request',{});response_raw=private_bytes(installed/'responses'/name,2<<20,group_read=True)
        response=json.loads(response_raw);action=request.get('action')
        failed=(journal.get('phase')=='STARTED' and journal.get('prepare_error')=='guard_rejected'
            and response=={'status':'BLOCKED','error':'exception_Blocked'})
        complete=(journal.get('phase')=='COMPLETE' and journal.get('result')==response
            and response.get('operation')==operation and response.get('phase')=='AWAITING_OFF_NODE'
            and response.get('status')=='WAITING' and response.get('fence_status')=='VERIFIED')
        identity=(request.get('nonce')==name[:-5] and request.get('operation',operation)==operation)
        admitted=(action=='prepare' and request.get('mode')=='full' and name[:12]==operation
            and (complete if captured else failed))
        if captured:admitted=admitted or action=='authorize' and request.get('operation')==operation and failed or action=='status' and request.get('operation')==operation and complete
        if (hashlib.sha256(raw).hexdigest()!=wanted['journal_sha256'] or journal.get('request_sha256')!=wanted['request_sha256']
            or canonical(request)!=wanted['request_sha256'] or not identity or not admitted
            or hashlib.sha256(response_raw).hexdigest()!=wanted['response_sha256']):fail()
    if captured and not any(json.loads(private_bytes((guard.ROOT/'installed/journal'/n),2<<20))['request']['action']=='authorize' for n in journals):fail()
    terminal=copy.deepcopy(snapshot);terminal.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',events=history+events)
    terminal[reference_key]=reference
    if state!=terminal:fail()
    return receipt



def verify_captured_evidence(base,snapshot,receipt):
    names={'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json',
        'rollout-backup.cms','decrypted-restore.json','apply-manifests.json'}
    files=receipt.get('retained_files',{})
    if not isinstance(files,dict) or set(files)!=names:fail()
    raw={}
    for name,row in files.items():
        if (set(row)!={'sha256','bytes'} or not re.fullmatch('[a-f0-9]{64}',row['sha256'])
            or type(row['bytes']) is not int or not 0<row['bytes']<=128<<20):fail()
        raw[name]=private_bytes(Path(base)/name,group_read=True)
        if len(raw[name])!=row['bytes'] or hashlib.sha256(raw[name]).hexdigest()!=row['sha256']:fail()
    cut=snapshot['cut'];cipher=snapshot['cipher_binding'];custody=snapshot['custody']
    hashes={'rollout-before.tar':cut['manifest']['archive_sha256'],
        'rollout-before-manifest.json':cut['manifest_sha256'],'rollout-backup.cms':cipher['cipher_sha256'],
        'apply-manifests.json':snapshot['target']['manifest_sha256']}
    if (any(files[n]['sha256']!=h for n,h in hashes.items())
        or json.loads(raw['rollout-before-manifest.json'])!=cut['manifest']
        or files['rollout-backup.cms']['bytes']!=cipher['cipher_bytes'] or cipher['operation']!=snapshot['operation']
        or custody.get('verified') is not True or any(custody.get(k)!=cipher[k] for k in (
            'operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes'))):fail()
    restored=json.loads(raw['decrypted-restore.json'])
    members={n:files[n]['sha256'] for n in ('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json')}
    members.update(snapshot.get('cipher_space_members',{}));members.update(cipher.get('space_members',{}));members.update(cipher.get('postseal_members',{}))
    if (restored.get('schema')!='voice-nats-decrypted-restore-v1' or restored.get('decrypted_members_verified') is not True
        or restored.get('remote_equivalence')!='FULL_CIPHERTEXT_SHA256_AND_SIZE'
        or restored.get('cipher_sha256')!=cipher['cipher_sha256'] or restored.get('cipher_bytes')!=cipher['cipher_bytes']
        or restored.get('members')!=members or restored.get('readback',{}).get('native_archive_verified') is not True):fail()

def _retained_mode(path):return stat.S_IMODE(Path(path).lstat().st_mode)


def verify_postmigration_terminal(base,state):
    """Historical, effects-aware operator retirement; never release authority."""
    from datetime import datetime
    base=Path(base);operation=state.get('operation','')
    if not re.fullmatch('[a-f0-9]{12}',operation) or base.name!='rollout-'+operation:fail()
    raw=private_bytes(base/'operator-postmigration-abort-snapshot.json');snapshot=json.loads(raw)
    sha=hashlib.sha256(raw).hexdigest();receipt=document(base/'operator-postmigration-abort-receipt.json')
    reference={'schema':'voice-operator-postmigration-abort-reference-v1','snapshot_sha256':sha,'receipt_sha256':canonical(receipt)}
    if (state.get('operator_postmigration_abort')!=reference or snapshot.get('schema')!='nats-rollout-root-v1'
        or snapshot.get('operation')!=operation or snapshot.get('phase')!='PAUSED_APPLY'
        or snapshot.get('status')!='WAITING' or snapshot.get('fence_status')!='VERIFIED'
        or snapshot.get('target',{}).get('mode')!='full' or not snapshot.get('renderer_applied')
        or snapshot.get('nats_migration',{}).get('verified') is not True
        or not snapshot.get('migration_completed_metadata') or not snapshot.get('migration_jobs')
        or any(k in snapshot for k in ('apply','restart','preservation','closed_preservation','closed_preservation_sealed',
            'operator_retirement','operator_precut_abort','operator_captured_abort','failed_readonly','operator_postmigration_abort'))
        or receipt.get('schema')!='voice-operator-postmigration-abort-closure-v1'
        or receipt.get('operation')!=operation or receipt.get('snapshot_sha256')!=sha
        or receipt.get('waiver')!=WAIVER or receipt.get('deployment_pass') is not False
        or receipt.get('rollback_eligible') is not False or receipt.get('journals')!={}
        or receipt.get('configuration_basis')!='RETAIN_ACTUAL_POST_MIGRATION_CONFIGURATION_NO_ORIGINAL_WRITE_CLAIM'
        or receipt.get('completed_migrations_sha256')!=canonical(snapshot['migration_jobs'])):fail()
    binding=root_cli.code_binding(base/'code')
    if binding!=snapshot.get('code_capture') or canonical(binding)!=receipt.get('code_binding_sha256'):fail()
    names=CORE|{'voice-web','voice-admin','voice-developer-portal'}
    ctx=snapshot['context'];original=ctx['original_snapshots'];captured=ctx['snapshots'];setup=receipt.get('setup',{})
    if (set(original)!=names or set(captured)!=names or set(setup)!=names
        or any(r['spec'].get('replicas',1)!=1 or captured[n]['spec'].get('replicas',1)!=0
            or captured[n]['metadata']['uid']!=r['metadata']['uid'] for n,r in original.items())):fail()
    for name,row in original.items():
        if name!=HUB and captured[name]['spec']['template']!=row['spec']['template']:fail()
    if captured[HUB]['spec']['template']!=original[HUB]['spec']['template']:
        descriptor=snapshot['renderer_authority']['descriptor']
        if (descriptor['original_template']!=original[HUB]['spec']['template']
            or descriptor['target_template']!=captured[HUB]['spec']['template']):fail()
    wanted={n:{'uid':r['metadata']['uid'],'template_sha256':canonical(r['spec']['template']),
        'replicas':1,'ready':setup[n].get('ready')} for n,r in original.items()}
    if (setup!=wanted or any(type(r['ready']) is not bool for r in setup.values()) or setup[HUB]['ready'] is not True
        or receipt.get('retained_setup')!=precut_retained_setup(snapshot)
        or receipt.get('storage_verified') is not True or receipt.get('config_verified') is not True):fail()
    allowed={'staging_preflight','rollout_original_capture','rollout_maintenance_owned','workload_scaled',
        'original_hub_shutdown_intent','original_hub_orderly_exit_verified','original_hub_owned_finalizer_removed',
        'nats_contract_old_cut_verified','nats_contract_cm_applied','migration_authority_copy_created',
        'migration_resource_created','migration_complete','operator_renderer_recovery_reentered',
        'operator_unconsumed_authorization_revoked','rollout_prepared','rollout_renderer_template_applied','rollout_renderer_template_intent'}
    history=snapshot.get('events');events=receipt.get('events')
    if (not isinstance(history,list) or not history or any(e.get('kind') not in allowed for e in history)
        or any(e.get('kind')=='workload_scaled' and (e.get('name') not in names or e.get('replicas')!=0) for e in history)
        or [e for e in history if e.get('kind')=='migration_complete']!=[dict(kind='migration_complete',**r) for r in snapshot['migration_jobs']]
        or not isinstance(events,list) or len(events)!=len(names)+2
        or events[0].get('kind')!='operator_postmigration_abort_intent'
        or events[-1].get('kind')!='operator_postmigration_abort_completed'
        or {e.get('name') for e in events[1:-1]}!=names
        or any(e!={'kind':'operator_postmigration_workload_restored','name':e.get('name'),
            'uid':wanted[e['name']]['uid'],'template_sha256':wanted[e['name']]['template_sha256'],'replicas':1} for e in events[1:-1])):fail()
    files=receipt.get('retained_files',{})
    required={'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json','rollout-backup.cms',
        'decrypted-restore.json','apply-manifests.json','apply-authorization.json'}
    retained={}
    if not isinstance(files,dict) or not required<=set(files):fail()
    for name,row in files.items():
        if (name not in required and not re.fullmatch(r'operator-(?:renderer-(?:before|correction)|authorization-before)(?:-attempt[0-9]+)?\.json',name)
            or set(row)!={'sha256','bytes','mode'} or type(row['bytes']) is not int or not 0<row['bytes']<=128<<20
            or not re.fullmatch('[a-f0-9]{64}',row['sha256']) or type(row['mode']) is not int
            or row['mode']&0o022 or _retained_mode(base/name)!=row['mode']):fail()
        retained[name]=private_bytes(base/name,group_read=True)
        if len(retained[name])!=row['bytes'] or hashlib.sha256(retained[name]).hexdigest()!=row['sha256']:fail()
    actual_names={p.name for pattern in ('operator-renderer-before*.json','operator-renderer-correction*.json','operator-authorization-before*.json') for p in base.glob(pattern)}
    if actual_names-set(files):fail()
    evidence={n:{k:files[n][k] for k in ('sha256','bytes')} for n in required-{'apply-authorization.json'}}
    verify_captured_evidence(base,snapshot,{'retained_files':evidence})
    authorization=json.loads(retained['apply-authorization.json']);claim=receipt.get('consumed_claim',{})
    before=receipt.get('before_marker',{});marker=ctx['marker']
    expected_data=dict(marker['data'],phase='rollout-applying')
    if (authorization!=snapshot.get('authorization') or before.get('data')!=expected_data
        or expected_data.get('knownRolloutOperation')!=operation or before.get('uid')!=marker['metadata']['uid']
        or set(before)!={'uid','resourceVersion','data'} or not re.fullmatch('[0-9]+',before.get('resourceVersion',''))
        or claim!={'uid':before['uid'],'resourceVersion':before['resourceVersion'],
            'authorization_sha256':files['apply-authorization.json']['sha256'],'expires_at':authorization['expires_at']}):fail()
    try:
        observed=datetime.fromisoformat(receipt['observed_at']);expired=datetime.fromisoformat(authorization['expires_at'])
        contract=datetime.fromisoformat(snapshot['nats_contract_expires_at'])
        if any(d.tzinfo is None for d in (observed,expired,contract)) or not expired<=observed or expired>contract:fail()
    except (ValueError,TypeError,KeyError):fail()
    after=receipt.get('released_marker',{});active=dict(expected_data,phase='active');active.pop('knownRolloutOperation')
    if (set(after)!={'uid','resourceVersion','data'} or after['uid']!=before['uid'] or after['data']!=active
        or not re.fullmatch('[0-9]+',after.get('resourceVersion','')) or after['resourceVersion']==before['resourceVersion']):fail()
    configs=receipt.get('configmaps',{})
    if (set(configs)!=precut_configmaps(original) or any(set(r)!={'uid','data_sha256','binary_data_sha256'}
        or not re.fullmatch('[a-f0-9-]{36}',r['uid']) or any(not re.fullmatch('[a-f0-9]{64}',r[k]) for k in ('data_sha256','binary_data_sha256')) for r in configs.values())):fail()
    resource_raw=private_bytes(base/'operator-postmigration-abort-resources.json');resources=json.loads(resource_raw)
    documents=json.loads(retained['apply-manifests.json'])
    if isinstance(documents,dict):
        if documents.get('kind')!='List' or not isinstance(documents.get('items'),list):fail()
        documents=documents['items']
    if not isinstance(documents,list):fail()
    frozen={r['kind']+'/'+r['metadata']['name']:r for r in documents}
    ledger={r['kind']+'/'+r['name']:r for r in snapshot['nonnats_binding']['objects']}
    if (hashlib.sha256(resource_raw).hexdigest()!=receipt.get('resource_disposition_sha256')
        or not isinstance(resources,dict) or len(documents)!=48 or len(frozen)!=48 or set(resources)!=set(frozen)
        or {r['metadata']['name'] for r in documents if r['kind']=='Deployment'}!=names-{HUB}):fail()
    for key,row in resources.items():
        kind,name=key.split('/',1)
        if (row.get('kind')!=kind or row.get('metadata',{}).get('name')!=name
            or 'uid' in frozen[key]['metadata'] and row['metadata'].get('uid')!=frozen[key]['metadata']['uid']
            or key not in ledger or row['metadata']['uid']!=ledger[key]['uid']):fail()
        if kind=='Deployment':
            if name not in names-{HUB} or row['spec']!=captured[name]['spec'] or row['spec']['replicas']!=0:fail()
        elif kind not in ('Service','NetworkPolicy','ConfigMap'):fail()
    terminal=copy.deepcopy(snapshot);terminal.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',events=history+events)
    terminal['operator_postmigration_abort']=reference
    if state!=terminal:fail()
    return receipt

def applied_journal_binding(installed,operation,claim,rows):
    import bridge
    if not isinstance(rows,dict) or not 1<=len(rows)<=100:fail()
    for name,pins in rows.items():
        if not re.fullmatch('[a-f0-9]{64}\\.json',name) or set(pins)!={'journal_sha256','request_sha256','response_sha256'}:fail()
        raw=private_bytes(Path(installed)/'journal'/name,2<<20);row=json.loads(raw)
        request=row.get('request',{})
        try:bridge.validate(request)
        except bridge.BridgeError:fail()
        response_raw=private_bytes(Path(installed)/'responses'/name,2<<20,group_read=True);response=json.loads(response_raw)
        if (hashlib.sha256(raw).hexdigest()!=pins['journal_sha256']
            or canonical(request)!=pins['request_sha256'] or row.get('request_sha256')!=pins['request_sha256']
            or request['nonce']+'.json'!=name or request.get('action')!='finish'
            or request.get('operation')!=operation or request.get('claim_rv')!=claim.get('resourceVersion')
            or row.get('phase')!='STARTED' or row.get('prepare_error')!='guard_rejected'
            or hashlib.sha256(response_raw).hexdigest()!=pins['response_sha256']
            or set(response)!={'status','error'} or response['status']!='BLOCKED'
            or response['error'] not in PUBLIC_CODES|{'exception_'+name for name in PUBLIC_TYPES}|{'blocked_unclassified'}):fail()
    return rows


def verify_applied_terminal(base,state):
    """Historical, effects-aware operator retirement; never release authority."""
    from datetime import datetime
    base=Path(base);operation=state.get('operation','')
    if not re.fullmatch('[a-f0-9]{12}',operation) or base.name!='rollout-'+operation:fail()
    raw=private_bytes(base/'operator-applied-abort-snapshot.json');snapshot=json.loads(raw)
    sha=hashlib.sha256(raw).hexdigest();receipt=document(base/'operator-applied-abort-receipt.json')
    reference={'schema':'voice-operator-applied-abort-reference-v1','snapshot_sha256':sha,'receipt_sha256':canonical(receipt)}
    if (state.get('operator_applied_abort')!=reference or snapshot.get('schema')!='nats-rollout-root-v1'
        or snapshot.get('operation')!=operation or snapshot.get('phase')!='RESTART'
        or snapshot.get('status')!='BLOCKED' or snapshot.get('fence_status')!='VERIFIED'
        or snapshot.get('target',{}).get('mode')!='full' or not snapshot.get('renderer_applied')
        or snapshot.get('nats_migration',{}).get('verified') is not True
        or not snapshot.get('migration_completed_metadata') or not snapshot.get('migration_jobs')
        or snapshot.get('nonnats_applied',{}).get('verified') is not True
        or snapshot.get('preservation',{}).get('native_files_verified') is not True
        or snapshot.get('preservation',{}).get('census_verified') is not True
        or not snapshot.get('closed_preservation_attempts')
        or snapshot.get('closed_preservation_attempt_nonce') not in snapshot['closed_preservation_attempts']
        or any(k in snapshot for k in ('restart',
            'operator_retirement','operator_precut_abort','operator_captured_abort','failed_readonly','operator_postmigration_abort','operator_applied_abort'))
        or receipt.get('schema')!='voice-operator-applied-abort-closure-v1'
        or receipt.get('operation')!=operation or receipt.get('snapshot_sha256')!=sha
        or receipt.get('waiver')!=WAIVER or receipt.get('deployment_pass') is not False
        or receipt.get('rollback_eligible') is not False
        or receipt.get('configuration_basis')!='RETAIN_APPLIED_TARGET_CONFIGURATION_AND_FORWARD_MIGRATIONS'
        or receipt.get('completed_migrations_sha256')!=canonical(snapshot['migration_jobs'])):fail()
    binding=root_cli.code_binding(base/'code')
    if binding!=snapshot.get('code_capture') or canonical(binding)!=receipt.get('code_binding_sha256'):fail()
    names=CORE|{'voice-web','voice-admin','voice-developer-portal'}
    ctx=snapshot['context'];original=ctx['original_snapshots'];captured=ctx['snapshots'];setup=receipt.get('setup',{})
    if (set(original)!=names or set(captured)!=names or set(setup)!=names
        or any(r['spec'].get('replicas',1)!=1 or captured[n]['spec'].get('replicas',1)!=0
            or captured[n]['metadata']['uid']!=r['metadata']['uid'] for n,r in original.items())):fail()
    for name,row in captured.items():
        if name!=HUB and canonical(row['spec']['template'])!=snapshot['target']['template_hashes'].get(name):fail()
    wanted={n:{'uid':r['metadata']['uid'],'template_sha256':canonical(r['spec']['template']),
        'replicas':1,'ready':setup[n].get('ready')} for n,r in captured.items()}
    if (setup!=wanted or any(type(r['ready']) is not bool for r in setup.values()) or setup[HUB]['ready'] is not True
        or receipt.get('retained_setup')!=precut_retained_setup(snapshot)
        or receipt.get('storage_verified') is not True or receipt.get('config_verified') is not True):fail()
    history=snapshot.get('events');events=receipt.get('events')
    if (not isinstance(history,list) or len(history)<len(names)
        or {e.get('name') for e in history[-len(names):]}!=names
        or any(e!={'kind':'workload_scaled','name':e.get('name'),'replicas':0} for e in history[-len(names):])
        or [e for e in history if e.get('kind')=='migration_complete']!=[dict(kind='migration_complete',**r) for r in snapshot['migration_jobs']]
        or not isinstance(events,list) or events!=[{'kind':'operator_applied_abort_intent','waiver':WAIVER,'snapshot_sha256':sha}]+
            [{'kind':'workload_scaled','name':n,'replicas':1} for n in [HUB,*sorted(names-{HUB})]]+[{'kind':'operator_applied_abort_completed'}]):fail()
    files=receipt.get('retained_files',{})
    required={'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json','rollout-backup.cms',
        'decrypted-restore.json','apply-manifests.json','apply-authorization.json'}
    retained={}
    if not isinstance(files,dict) or not required<=set(files):fail()
    for name,row in files.items():
        if (name not in required and not re.fullmatch(r'operator-(?:restart-(?:controller|readiness)-(?:snapshot|record)\.json|readiness-diagnostic-(?:file|messaging)-1\.log)',name)
            or set(row)!={'sha256','bytes','mode'} or type(row['bytes']) is not int or not 0<row['bytes']<=128<<20
            or not re.fullmatch('[a-f0-9]{64}',row['sha256']) or type(row['mode']) is not int
            or row['mode']&0o022 or _retained_mode(base/name)!=row['mode']):fail()
        retained[name]=private_bytes(base/name,group_read=True)
        if len(retained[name])!=row['bytes'] or hashlib.sha256(retained[name]).hexdigest()!=row['sha256']:fail()
    actual_names={p.name for pattern in ('operator-restart-*.json','operator-readiness-diagnostic-*-1.log') for p in base.glob(pattern)}
    if actual_names-set(files):fail()
    evidence={n:{k:files[n][k] for k in ('sha256','bytes')} for n in required-{'apply-authorization.json'}}
    verify_captured_evidence(base,snapshot,{'retained_files':evidence})
    authorization=json.loads(retained['apply-authorization.json']);claim=receipt.get('consumed_claim',{})
    before=receipt.get('before_marker',{});marker=ctx['marker']
    expected_data=marker['data']
    if expected_data.get('phase')!='rollout-verified':fail()
    if (authorization!=snapshot.get('authorization') or before.get('data')!=expected_data
        or expected_data.get('knownRolloutOperation')!=operation or before.get('uid')!=marker['metadata']['uid']
        or before.get('resourceVersion')!=marker['metadata']['resourceVersion']
        or set(before)!={'uid','resourceVersion','data'} or not re.fullmatch('[0-9]+',before.get('resourceVersion',''))
        or claim!={'uid':before['uid'],'resourceVersion':claim.get('resourceVersion'),
            'authorization_sha256':files['apply-authorization.json']['sha256'],'expires_at':authorization['expires_at']}):fail()
    try:
        observed=datetime.fromisoformat(receipt['observed_at']);expired=datetime.fromisoformat(authorization['expires_at'])
        contract=datetime.fromisoformat(snapshot['nats_contract_expires_at'])
        if any(d.tzinfo is None for d in (observed,expired,contract)):fail()
    except (ValueError,TypeError,KeyError):fail()
    after=receipt.get('released_marker',{});active=dict(expected_data,phase='active');active.pop('knownRolloutOperation')
    if (set(after)!={'uid','resourceVersion','data'} or after['uid']!=before['uid'] or after['data']!=active
        or not re.fullmatch('[0-9]+',after.get('resourceVersion','')) or after['resourceVersion']==before['resourceVersion']):fail()
    configs=receipt.get('configmaps',{})
    if (set(configs)!=precut_configmaps(original) or any(set(r)!={'uid','data_sha256','binary_data_sha256'}
        or not re.fullmatch('[a-f0-9-]{36}',r['uid']) or any(not re.fullmatch('[a-f0-9]{64}',r[k]) for k in ('data_sha256','binary_data_sha256')) for r in configs.values())):fail()
    resource_raw=private_bytes(base/'operator-applied-abort-resources.json');resources=json.loads(resource_raw)
    documents=json.loads(retained['apply-manifests.json'])
    if isinstance(documents,dict):
        if documents.get('kind')!='List' or not isinstance(documents.get('items'),list):fail()
        documents=documents['items']
    if not isinstance(documents,list):fail()
    frozen={r['kind']+'/'+r['metadata']['name']:r for r in documents}
    ledger={r['kind']+'/'+r['name']:r for r in snapshot['nonnats_binding']['objects']}
    if (hashlib.sha256(resource_raw).hexdigest()!=receipt.get('resource_disposition_sha256')
        or not isinstance(resources,dict) or len(documents)!=48 or len(frozen)!=48 or set(resources)!=set(frozen)
        or {r['metadata']['name'] for r in documents if r['kind']=='Deployment'}!=names-{HUB}):fail()
    for key,row in resources.items():
        kind,name=key.split('/',1)
        if (row.get('kind')!=kind or row.get('metadata',{}).get('name')!=name
            or 'uid' in frozen[key]['metadata'] and row['metadata'].get('uid')!=frozen[key]['metadata']['uid']
            or key not in ledger or row['metadata']['uid']!=ledger[key]['uid']):fail()
        if kind=='Deployment':
            if name not in names-{HUB} or row['spec']!=captured[name]['spec'] or row['spec']['replicas']!=0:fail()
        elif kind not in ('Service','NetworkPolicy','ConfigMap'):fail()
    applied_journal_binding(guard.ROOT/'installed',operation,claim,receipt.get('journals'))
    terminal=copy.deepcopy(snapshot);terminal.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',events=history+events)
    terminal['operator_applied_abort']=reference
    if state!=terminal:fail()
    return receipt


def idle_state(base,state):
    if state.get('status') in ('PASS','ROLLED_BACK'):return True
    verify_terminal(base,state)
    return True


PREBUILD_RECEIPTS={
    '161c282144ec':'ded7b5fb7a1342653b2067157b0b2410b4d888f5a4fc23cd6050d8fb8ebd3db3',
    '65607cfc9397':'707e8b70d15f0ead23da0f87ed1aac519b40435ff2774266fc7a0eb553a63594',
    'c8862dbc23a8':'ce3b0e55845c8bc898d02577c9e850f61de0063c435af65cfe4e8ccc1add4110'}

# One known full prepare stopped in source capture before workload capture or
# checkpoint creation. Its immutable journal/response identity is admitted
# separately from the ordinary disposable-abandonment inventory.
SOURCE_CAPTURE_REJECTION={
    'operation':'fc2e710d33e6',
    'nonce':'fc2e710d33e6431e6c65158c629e3cc726ebb461e1a16098a322f995984748ec',
    'source_sha':'fa4198bc54810c6b1d837fee2cd5436d5a690f54',
    'run_id':38469787490,
    'journal_sha256':'689c87bf3907f6cf673bccc3cf11b8ed395274803588ed788d232fc3d729de36',
    'response_sha256':'83fb20c9621d18d737818eb030b72008076d2afff606a37dac0c37ee8daa7a28',
}

def _source_capture_rejection_workspace(installed):
    """Require the exact empty workspace left before the first source write."""
    installed=Path(installed);operation=SOURCE_CAPTURE_REJECTION['operation']
    sources=installed/'sources';workspace=sources/operation
    parent=sources.lstat();row=workspace.lstat()
    if (not stat.S_ISDIR(parent.st_mode) or stat.S_ISLNK(parent.st_mode) or parent.st_uid!=0 or parent.st_gid!=0
        or stat.S_IMODE(parent.st_mode)!=0o700 or not stat.S_ISDIR(row.st_mode) or stat.S_ISLNK(row.st_mode)
        or row.st_uid!=0 or row.st_gid!=0 or stat.S_IMODE(row.st_mode)!=0o700 or any(workspace.iterdir())):fail()
    return (parent.st_dev,parent.st_ino,parent.st_uid,parent.st_gid,stat.S_IMODE(parent.st_mode),
        row.st_dev,row.st_ino,row.st_uid,row.st_gid,stat.S_IMODE(row.st_mode))

def _verify_source_capture_rejection(installed,path,row):
    spec=SOURCE_CAPTURE_REJECTION;operation=spec['operation'];nonce=spec['nonce']
    if path.name!=nonce+'.json' or path.stem[:12]!=operation:fail()
    journal_raw=private_bytes(path,2<<20);response_raw=private_bytes(Path(installed)/'responses'/path.name,2<<20,group_read=True)
    try:response=json.loads(response_raw)
    except (TypeError,ValueError):fail()
    request=row.get('request')
    expected={'action':'prepare','nonce':nonce,'source_sha':spec['source_sha'],'run_id':spec['run_id'],
        'mode':'full','changed_services':[]}
    if (hashlib.sha256(journal_raw).hexdigest()!=spec['journal_sha256']
        or hashlib.sha256(response_raw).hexdigest()!=spec['response_sha256']
        or set(row)!={'schema','request_sha256','request','phase','prepare_stage','prepare_error'}
        or row.get('schema')!='voice-nats-bridge-request-v1' or request!=expected
        or row.get('request_sha256')!=canonical(request) or row.get('phase')!='STARTED'
        or row.get('prepare_stage')!={'name':'source-capture','status':'STARTED'}
        or row.get('prepare_error')!='source_authority_rejected'
        or response!={'status':'BLOCKED','error':'blocked_unclassified'}):fail()
    for candidate in (guard.ROOT/('rollout-'+operation),Path(installed)/'sources'/(operation+'-build')):
        try:candidate.lstat()
        except FileNotFoundError:continue
        fail()
    return _source_capture_rejection_workspace(installed)

def pending_request_accepted(installed,path,closure):
    installed=Path(installed);path=Path(path)
    if path.parent!=installed/'journal' or not re.fullmatch('[a-f0-9]{64}\\.json',path.name):fail()
    row=document(path)
    if row.get('phase')=='COMPLETE':return
    if path.name==FENCE_ABORT_NONCE+'.json':
        base=guard.ROOT/('rollout-'+FENCE_ABORT_OPERATION)
        state=document(base/'checkpoint.json')
        verified=verify_fence_abort_terminal(base,state)
        if path.name not in verified.get('journals',{}):fail()
        return
    if path.name in closure['journals']:
        if closure.get('schema') in ('voice-operator-fence-abort-closure-v1','voice-operator-fence-abort-closure-v2'):
            request=row.get('request',{});operation=request.get('nonce','')[:12]
            base=guard.ROOT/('rollout-'+operation)
            if verify_fence_abort_terminal(base,document(base/'checkpoint.json'))!=closure:fail()
            return
        if closure.get('schema')=='voice-failed-readonly-closure-v1':
            base=guard.ROOT/('rollout-'+path.stem[:12])
            if verify_readonly_terminal(base,document(base/'checkpoint.json'))!=closure:fail()
            return
        if closure.get('schema')=='voice-operator-applied-abort-closure-v1':
            request=row.get('request',{});operation=request.get('operation','')
            base=guard.ROOT/('rollout-'+operation)
            if verify_applied_terminal(base,document(base/'checkpoint.json'))!=closure:fail()
            return
        journal_binding(installed,{path.name:closure['journals'][path.name]})
        return
    operation=path.stem[:12];wanted=PREBUILD_RECEIPTS.get(operation)
    if operation==SOURCE_CAPTURE_REJECTION['operation']:
        return _verify_source_capture_rejection(installed,path,row)
    if wanted is not None:
        raw=private_bytes(installed/('prebuild-disposition-'+operation+'.json'),2<<20)
        if hashlib.sha256(raw).hexdigest()!=wanted:fail()
        receipt=json.loads(raw)
        if (receipt.get('operation')!=operation or receipt.get('original_journal')!=row
            or receipt.get('journal_sha256')!=hashlib.sha256(private_bytes(path,2<<20)).hexdigest()
            or receipt.get('response_sha256')!=hashlib.sha256(private_bytes(installed/'responses'/path.name,2<<20,group_read=True)).hexdigest()
            or receipt.get('operation_absent') is not True or receipt.get('build_absent') is not True):fail()
        for candidate in (guard.ROOT/('rollout-'+operation),installed/'sources'/(operation+'-build')):
            try:candidate.lstat()
            except FileNotFoundError:continue
            fail()
        return
    verify_pending_abandonment(installed,path)


def verify_pending_abandonment(installed,path):
    """Admit only this exact root-attested abandonment, never a new pending nonce."""
    installed=Path(installed);path=Path(path)
    try:
        raw=private_bytes(installed/PENDING_ABANDONMENT_NAME,2<<20)
        receipt=json.loads(raw)
    except (OSError,ValueError):fail()
    if (type(receipt) is not dict or set(receipt)!={'schema','authorization','fence_abort','entries','inventory_sha256'}
        or receipt.get('schema')!='voice-nats-pending-abandonment-v1'
        or receipt.get('authorization')!='EXPLICIT_USER_AUTHORIZED_DISPOSABLE_STAGING_RECOVERY'
        or not isinstance(receipt.get('entries'),dict) or not 1<=len(receipt['entries'])<=100
        or receipt.get('inventory_sha256')!=canonical(receipt['entries'])
        or FENCE_ABORT_NONCE+'.json' in receipt['entries']):fail()
    if set(receipt['entries'])!=pending_abandonment_names(installed):fail()
    fence_base=guard.ROOT/('rollout-'+FENCE_ABORT_OPERATION)
    fence_state=document(fence_base/'checkpoint.json')
    fence=verify_fence_abort_terminal(fence_base,fence_state)
    if receipt.get('fence_abort')!={'operation':FENCE_ABORT_OPERATION,
        'checkpoint_sha256':FENCE_ABORT_CHECKPOINT_SHA256,
        'receipt_sha256':canonical(document(fence_base/'operator-fence-abort-receipt.json'))}:fail()
    name=path.name
    if name not in receipt['entries']:raise Blocked('bridge_upgrade_interrupted_request')
    pins=receipt['entries'][name]
    if (not re.fullmatch(r'[a-f0-9]{64}\.json',name)
        or set(pins)!={'journal_sha256','request_sha256','response_sha256','operation','build_key',
            'checkpoint_sha256','checkpoint_phase','checkpoint_status','operation_state','build_present',
            'build_fingerprint','disposition'}
        or pins.get('disposition')!='ABANDONED_BY_EXPLICIT_USER_AUTHORIZATION'
        or pins.get('operation_state') not in ('TERMINAL_CHECKPOINT','CHECKPOINT_ABSENT','PREBUILD_ONLY')
        or type(pins.get('build_present')) is not bool):fail()
    try:
        journal_raw=private_bytes(path,2<<20)
        response_raw=private_bytes(installed/'responses'/name,2<<20,group_read=True)
        journal=json.loads(journal_raw);response=json.loads(response_raw)
    except (OSError,ValueError):fail()
    request=journal.get('request',{})
    if (journal.get('phase')!='STARTED' or journal.get('request_sha256')!=pins['request_sha256']
        or canonical(request)!=pins['request_sha256'] or request.get('nonce')!=path.stem
        or hashlib.sha256(journal_raw).hexdigest()!=pins['journal_sha256']
        or hashlib.sha256(response_raw).hexdigest()!=pins['response_sha256']
        or response.get('status')!='BLOCKED' or set(response)-{'status','error','operation'}
        or response.get('error') not in PUBLIC_CODES|{'exception_'+kind for kind in PUBLIC_TYPES}|{'blocked_unclassified'}):fail()
    import bridge
    validated=dict(request)
    if 'token' in bridge.FIELDS.get(request.get('action'),set()) and 'token' not in validated:
        validated['token']='disposition-schema-check'
    try:bridge.validate(validated)
    except bridge.BridgeError:fail()
    request_operation=request.get('operation');response_operation=response.get('operation');prefix=path.stem[:12]
    if (request_operation is not None and not re.fullmatch('[a-f0-9]{12}',request_operation)
        or response_operation is not None and not re.fullmatch('[a-f0-9]{12}',response_operation)
        or request_operation is not None and response_operation is not None and request_operation!=response_operation):fail()
    requested=request_operation if request_operation is not None else response_operation
    build_key=requested if isinstance(requested,str) else prefix
    candidate=requested if isinstance(requested,str) else prefix
    if not re.fullmatch('[a-f0-9]{12}',candidate):fail()
    operation_path=guard.ROOT/('rollout-'+candidate) if candidate else None
    checkpoint_sha=checkpoint_phase=checkpoint_status=None
    operation_present=False
    if operation_path is not None:
        try:operation_stat=operation_path.lstat()
        except FileNotFoundError:pass
        else:
            if not stat.S_ISDIR(operation_stat.st_mode) or stat.S_ISLNK(operation_stat.st_mode):fail()
            operation_present=True
    if operation_present:
        state=document(operation_path/'checkpoint.json')
        if state.get('operation')!=candidate:fail()
        idle_state(operation_path,state)
        checkpoint_sha=hashlib.sha256(private_bytes(operation_path/'checkpoint.json')).hexdigest()
        checkpoint_phase=state.get('phase');checkpoint_status=state.get('status')
    build_path=installed/'sources'/(build_key+'-build') if re.fullmatch('[a-f0-9]{12}',build_key) else None
    build_present=False;build_fingerprint=None
    if build_path is not None:
        try:build_stat=build_path.lstat()
        except FileNotFoundError:pass
        else:
            if (not stat.S_ISDIR(build_stat.st_mode) or stat.S_ISLNK(build_stat.st_mode)
                or build_stat.st_uid!=0):fail()
            build_present=True
            build_fingerprint={'device':build_stat.st_dev,'inode':build_stat.st_ino,
                'mode':stat.S_IMODE(build_stat.st_mode),'uid':build_stat.st_uid,'gid':build_stat.st_gid,
                'bytes':build_stat.st_size,'mtime_ns':build_stat.st_mtime_ns,'ctime_ns':build_stat.st_ctime_ns}
    operation_state=('TERMINAL_CHECKPOINT' if operation_present else
        'PREBUILD_ONLY' if build_present else 'CHECKPOINT_ABSENT')
    if (pins.get('operation')!=candidate or pins.get('build_key')!=build_key or pins.get('checkpoint_sha256')!=checkpoint_sha
        or pins.get('checkpoint_phase')!=checkpoint_phase or pins.get('checkpoint_status')!=checkpoint_status
        or pins.get('operation_state')!=operation_state or pins.get('build_present') is not build_present
        or pins.get('build_fingerprint')!=build_fingerprint):fail()
    return {'receipt_sha256':hashlib.sha256(raw).hexdigest(),'fence':fence['schema'],'name':name}


def pending_abandonment_names(installed):
    """Bind the administrative receipt to the complete current STARTED set."""
    directory=Path(installed)/'journal'
    rows=sorted(directory.iterdir(),key=lambda item:item.name)
    if len(rows)>100:fail()
    names=set()
    for path in rows:
        if not re.fullmatch(r'[a-f0-9]{64}\.json',path.name):fail()
        try:row=json.loads(private_bytes(path,2<<20))
        except (OSError,ValueError):fail()
        if type(row) is not dict:fail()
        phase=row.get('phase')
        if phase=='COMPLETE':continue
        if phase!='STARTED':fail()
        # These two requests are independently fixed and verified by the
        # terminal fence closure / source-capture rejection disposition.
        if path.name not in (FENCE_ABORT_NONCE+'.json',SOURCE_CAPTURE_REJECTION['nonce']+'.json'):
            names.add(path.name)
    return names

class RetirementStage(Staging):
    def __init__(self,kube,snapshot,plan,journal):
        ctx=snapshot['context'];super().__init__(kube,OPERATION,copy.deepcopy(ctx['expected']),journal)
        for key in ('snapshots','marker','service','final_claim','final_pv'):
            setattr(self,key,copy.deepcopy(ctx[key]))
        self.final_path=Path(ctx['final_path']);self.original=ctx['original_snapshots'];self.plan=plan
        self.released=None;self.setup=None

    def owned_marker(self):
        marker=self.kube.get('configmap',MARKER)
        if (marker['metadata']['uid']!=self.marker['metadata']['uid']
            or marker['metadata']['resourceVersion']!=self.marker['metadata']['resourceVersion']
            or marker['data']!=self.marker['data']
            or marker['data'].get('phase')!='rollout-prepared'
            or marker['data'].get('knownRolloutOperation')!=OPERATION):fail()
        self.storage_config()
        return marker

    def storage_config(self):
        namespace=self.kube.get('namespace',guard.NS)
        claim=self.kube.get('pvc',self.expected['source_claim']);pv=self.kube.get('pv',claim['spec']['volumeName'])
        if (namespace['metadata']['uid']!=self.expected['namespace_uid']
            or claim['metadata']['uid']!=self.expected['source_claim_uid'] or claim['status']['phase']!='Bound'
            or pv['metadata']['uid']!=self.expected['source_pv_uid']
            or pv_storage_path(pv,claim['metadata']['uid'],claim['metadata']['name'],pv['metadata']['uid'])!=str(self.final_path)
            or claim['spec']!=self.final_claim['spec'] or pv['spec']!=self.final_pv['spec']
            or self.kube.get('service','voice-nats')!=self.service):fail()
        for name,wanted in self.plan['configmaps'].items():
            row=self.kube.get('configmap',name)
            if row['metadata']['uid']!=wanted['uid'] or canonical(row.get('data',{}))!=wanted['data_sha256']:fail()

    def release_marker(self):
        self.owned_marker();setup={}
        for name,original in self.original.items():
            if not getattr(self,'waived_continuation',False):self.wait_ready(name)
            row=self.kube.get('deployment',name)
            if (row['metadata']['uid']!=original['metadata']['uid'] or row['spec']['template']!=original['spec']['template']
                or row['spec'].get('replicas',1)!=1):fail()
            setup[name]={'uid':row['metadata']['uid'],'template_sha256':canonical(row['spec']['template']),
                'replicas':1,'ready':row.get('status',{}).get('observedGeneration',0)>=row['metadata']['generation']
                    and all(row.get('status',{}).get(k,0)==1 for k in ('readyReplicas','updatedReplicas','availableReplicas'))}
        if setup[HUB]['ready'] is not True:fail()
        marker=self.owned_marker()
        self.released=self.kube.cas('configmap',marker,[
            {'op':'test','path':'/data/phase','value':'rollout-prepared'},
            {'op':'test','path':'/data/knownRolloutOperation','value':OPERATION},
            {'op':'replace','path':'/data/phase','value':'active'},
            {'op':'remove','path':'/data/knownRolloutOperation'}])
        wanted=copy.deepcopy(marker['data']);wanted['phase']='active';wanted.pop('knownRolloutOperation')
        if self.released['metadata']['uid']!=marker['metadata']['uid'] or self.released['data']!=wanted:fail()
        self.setup=setup

    def restore_owned_cycle(self):
        row=self.kube.get('deployment','voice-user');original=self.original['voice-user']['spec']['template']
        if row['spec']['template']==original:return
        owned=self.snapshots['voice-user']['spec']['template']
        if (row['metadata']['uid']!=self.original['voice-user']['metadata']['uid']
            or row['spec']['template']!=owned
            or owned.get('metadata',{}).get('annotations',{}).get('voice.io/nats-user-space-bootstrap')!=OPERATION):fail()
        self.snapshots['voice-user']=self.kube.cas('deployment',row,[
            {'op':'test','path':'/spec/template','value':owned},
            {'op':'replace','path':'/spec/template','value':original}])
        self.save({'kind':'user_cycle_restored','owner':OPERATION})

def retire(code,plan_path,plan_sha,kube=None):
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('operator_retirement_human_root_required')
    code=Path(code);binding=root_cli.code_binding(code)
    plan_raw=private_bytes(plan_path,2<<20)
    if hashlib.sha256(plan_raw).hexdigest()!=plan_sha:fail()
    plan=json.loads(plan_raw)
    if (set(plan)!={'schema','operation','checkpoint_sha256','code_binding_sha256','waiver','configmaps','journals'}
        or plan['schema']!='voice-specific-operator-retirement-plan-v1' or plan['operation']!=OPERATION
        or plan['checkpoint_sha256']!=SNAPSHOT_SHA or plan['code_binding_sha256']!=canonical(binding)
        or plan['waiver']!=WAIVER or set(plan['configmaps'])!=CONFIGMAPS):fail()
    installed=guard.ROOT/'installed';base=root_cli.operation_path(str(guard.ROOT/('rollout-'+OPERATION)))
    with operation_lock():
        if canonical(root_cli.code_binding(installed/'code'))!=V15_BINDING:fail()
        inbox=installed/'inbox';old_mode=stat.S_IMODE(inbox.lstat().st_mode)
        if old_mode!=0o1730:fail()
        inbox.chmod(0o700)
        try:
            if any(inbox.iterdir()) or any((installed/'processing').iterdir()):fail()
            raw=private_bytes(base/'checkpoint.json')
            if hashlib.sha256(raw).hexdigest()!=SNAPSHOT_SHA:fail()
            snapshot=json.loads(raw)
            # Exact checkpoint bytes pin all private authority/contract inputs.
            terminal_state(snapshot,[{'kind':'operator_retirement_intent'},{'kind':'operator_retirement_completed'}],'a'*64)
            ctx=snapshot['context'];original=ctx['original_snapshots']
            if set(original)!=CORE or set(ctx['snapshots'])!=CORE:fail()
            for name,row in original.items():
                current=ctx['snapshots'][name]
                if (row['spec'].get('replicas',1)!=1 or current['spec'].get('replicas',1)!=0
                    or current['metadata']['uid']!=row['metadata']['uid'] or current['spec']['template']!=row['spec']['template']):fail()
            journal_binding(installed,plan['journals'])
            stage=None;events=[{'kind':'operator_retirement_intent','waiver':WAIVER,'plan_sha256':plan_sha}]
            progress=copy.deepcopy(snapshot)
            def journal(event):
                events.append(copy.deepcopy(event))
                progress.update(phase='OPERATOR_RETIRING',status='RUNNING',
                    events=copy.deepcopy(snapshot['events']+events))
                save(base/'checkpoint.json',progress,limit=128<<20)
            stage=RetirementStage(kube or Kube(),snapshot,plan,journal)
            stage.verify_closed()
            if stage.marker['metadata']['resourceVersion']!='3154269':fail()
            exclusive(base/'operator-retirement-snapshot.json',raw)
            exclusive(base/'operator-retirement-plan.json',plan_raw)
            progress.update(phase='OPERATOR_RETIRING',status='RUNNING',events=snapshot['events']+events)
            save(base/'checkpoint.json',progress,limit=128<<20)
            try:
                stage.restart()
                if stage.released is None or stage.setup is None:fail()
                events.append({'kind':'operator_retirement_completed'})
                receipt={'schema':'voice-operator-retirement-closure-v1','operation':OPERATION,
                    'snapshot_sha256':SNAPSHOT_SHA,'waiver':WAIVER,'plan_sha256':plan_sha,
                    'code_binding_sha256':canonical(binding),'deployment_pass':False,'rollback_eligible':False,
                    'events':events,'setup':stage.setup,'journals':plan['journals'],
                    'released_marker':{'uid':stage.released['metadata']['uid'],
                        'resourceVersion':stage.released['metadata']['resourceVersion'],'data':stage.released['data']},
                    'storage_verified':True,'config_verified':True,'user_cycle_restored':True}
                exclusive(base/'operator-retirement-receipt.json',
                    json.dumps(receipt,sort_keys=True,separators=(',',':')).encode())
                state=terminal_state(snapshot,events,canonical(receipt))
                verify_terminal(base,state);save(base/'checkpoint.json',state,limit=128<<20)
                return {'operation':OPERATION,'status':'ABORTED','phase':'OPERATOR_RETIRED',
                    'receipt_sha256':canonical(receipt),'setup_ready':True}
            except Exception:
                progress.update(phase='OPERATOR_RETIRING',status='BLOCKED',fence_status='UNKNOWN')
                try:
                    if stage.released is None:
                        stage.restore_owned_cycle()
                        if stage.refence().get('verified') is True:progress['fence_status']='VERIFIED'
                finally:
                    progress.update(phase='OPERATOR_RETIRING',status='BLOCKED')
                    save(base/'checkpoint.json',progress,limit=128<<20)
                raise
        finally:inbox.chmod(0o1730)


def failed_snapshot(snapshot,failed):
    expected=copy.deepcopy(snapshot)
    expected.update(phase='OPERATOR_RETIRING',status='BLOCKED',fence_status='VERIFIED',
        events=failed.get('events'))
    if (failed!=expected or len(failed.get('events',[]))!=125
        or failed['events'][:83]!=snapshot['events']
        or failed['events'][83].get('kind')!='operator_retirement_intent'
        or any(e.get('kind') not in {'operator_retirement_intent','workload_scaled',
            'user_cycle_override','user_cycle_restored'} for e in failed['events'][83:])):fail()

def continuation_document(row,plan_sha):
    if (not isinstance(row,dict) or set(row)!={'schema','operation','failed_checkpoint_sha256',
        'original_plan_sha256','code_binding_sha256','waiver'}
        or row['schema']!='voice-specific-retirement-continuation-v1'
        or row['operation']!=OPERATION or row['failed_checkpoint_sha256']!=FAILED_RETIREMENT_SHA
        or row['original_plan_sha256']!=plan_sha or row['waiver']!=WAIVER
        or not re.fullmatch('[a-f0-9]{64}',row['code_binding_sha256'])):fail()

def verify_continuation(base,snapshot,plan_raw,receipt):
    failed_raw=private_bytes(base/'operator-retirement-failed-attempt.json')
    if hashlib.sha256(failed_raw).hexdigest()!=FAILED_RETIREMENT_SHA:fail()
    failed=json.loads(failed_raw);failed_snapshot(snapshot,failed)
    raw=private_bytes(base/'operator-retirement-continuation.json',2<<20)
    row=json.loads(raw);continuation_document(row,hashlib.sha256(plan_raw).hexdigest())
    if (json.loads(plan_raw)['code_binding_sha256']!=ORIGINAL_RETIREMENT_BINDING
        or receipt.get('continuation')!={'attestation_sha256':hashlib.sha256(raw).hexdigest(),
            'code_binding_sha256':row['code_binding_sha256']}
        or receipt['events'][:42]!=failed['events'][83:]
        or receipt['events'][42]!={'kind':'operator_retirement_continuation_intent',
            'attestation_sha256':hashlib.sha256(raw).hexdigest(),
            'code_binding_sha256':row['code_binding_sha256']}):fail()
    return row

def continue_retirement(code,attestation_path,attestation_sha,kube=None):
    """One exact failed-attempt continuation under the explicit current-data waiver."""
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('operator_retirement_human_root_required')
    code=Path(code);binding=root_cli.code_binding(code)
    installed=guard.ROOT/'installed';base=root_cli.operation_path(str(guard.ROOT/('rollout-'+OPERATION)))
    with operation_lock():
        if canonical(root_cli.code_binding(installed/'code'))!=V15_BINDING:fail()
        inbox=installed/'inbox'
        if stat.S_IMODE(inbox.lstat().st_mode)!=0o1730:fail()
        inbox.chmod(0o700)
        try:
            if any(inbox.iterdir()) or any((installed/'processing').iterdir()):fail()
            raw=private_bytes(base/'checkpoint.json')
            if hashlib.sha256(raw).hexdigest()!=FAILED_RETIREMENT_SHA:fail()
            failed=json.loads(raw)
            snapshot_raw=private_bytes(base/'operator-retirement-snapshot.json')
            if hashlib.sha256(snapshot_raw).hexdigest()!=SNAPSHOT_SHA:fail()
            snapshot=json.loads(snapshot_raw);failed_snapshot(snapshot,failed)
            plan_raw=private_bytes(base/'operator-retirement-plan.json',2<<20);plan=json.loads(plan_raw)
            if (plan.get('schema')!='voice-specific-operator-retirement-plan-v1'
                or plan.get('operation')!=OPERATION or plan.get('checkpoint_sha256')!=SNAPSHOT_SHA
                or plan.get('code_binding_sha256')!=ORIGINAL_RETIREMENT_BINDING
                or plan.get('waiver')!=WAIVER or set(plan.get('configmaps',{}))!=CONFIGMAPS):fail()
            att_raw=private_bytes(attestation_path,2<<20)
            if hashlib.sha256(att_raw).hexdigest()!=attestation_sha:fail()
            attestation=json.loads(att_raw);continuation_document(attestation,hashlib.sha256(plan_raw).hexdigest())
            if attestation['code_binding_sha256']!=canonical(binding):fail()
            journal_binding(installed,plan['journals'])
            events=copy.deepcopy(failed['events'][83:])
            events.append({'kind':'operator_retirement_continuation_intent',
                'attestation_sha256':attestation_sha,'code_binding_sha256':canonical(binding)})
            progress=copy.deepcopy(failed)
            def journal(event):
                events.append(copy.deepcopy(event))
                progress.update(phase='OPERATOR_RETIRING',status='RUNNING',
                    events=copy.deepcopy(snapshot['events']+events))
                save(base/'checkpoint.json',progress,limit=128<<20)
            stage=RetirementStage(kube or Kube(),snapshot,plan,journal)
            stage.waived_continuation=True
            stage.verify_closed()
            # Authenticate every unchanged template before the first startup effect.
            for name,original in stage.original.items():
                row=stage.kube.get('deployment',name)
                if (row['metadata']['uid']!=original['metadata']['uid']
                    or row['spec']['template']!=original['spec']['template']
                    or row['spec'].get('replicas',1)!=0):fail()
            exclusive(base/'operator-retirement-failed-attempt.json',raw)
            exclusive(base/'operator-retirement-continuation.json',att_raw)
            progress.update(phase='OPERATOR_RETIRING',status='RUNNING',
                events=copy.deepcopy(snapshot['events']+events))
            save(base/'checkpoint.json',progress,limit=128<<20)
            try:
                stage.scale(HUB,1);stage.wait_ready(HUB)
                for name in sorted(CORE-{HUB}):stage.scale(name,1)
                stage.release_marker()
                events.append({'kind':'operator_retirement_completed'})
                receipt={'schema':'voice-operator-retirement-closure-v1','operation':OPERATION,
                    'snapshot_sha256':SNAPSHOT_SHA,'waiver':WAIVER,
                    'plan_sha256':hashlib.sha256(plan_raw).hexdigest(),
                    'code_binding_sha256':plan['code_binding_sha256'],
                    'continuation':{'attestation_sha256':attestation_sha,'code_binding_sha256':canonical(binding)},
                    'deployment_pass':False,'rollback_eligible':False,
                    'events':events,'setup':stage.setup,'journals':plan['journals'],
                    'released_marker':{'uid':stage.released['metadata']['uid'],
                        'resourceVersion':stage.released['metadata']['resourceVersion'],'data':stage.released['data']},
                    'storage_verified':True,'config_verified':True,'user_cycle_restored':True}
                exclusive(base/'operator-retirement-receipt.json',
                    json.dumps(receipt,sort_keys=True,separators=(',',':')).encode())
                state=terminal_state(snapshot,events,canonical(receipt))
                verify_terminal(base,state);save(base/'checkpoint.json',state,limit=128<<20)
                return {'operation':OPERATION,'status':'ABORTED','phase':'OPERATOR_RETIRED',
                    'receipt_sha256':canonical(receipt),'setup_ready':all(r['ready'] for r in stage.setup.values())}
            except Exception:
                progress['fence_status']='UNKNOWN'
                try:
                    if stage.released is None and stage.refence().get('verified') is True:
                        progress['fence_status']='VERIFIED'
                finally:
                    progress.update(phase='OPERATOR_RETIRING',status='BLOCKED')
                    save(base/'checkpoint.json',progress,limit=128<<20)
                raise
        finally:inbox.chmod(0o1730)

if __name__=='__main__':
    sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
    continuation=len(sys.argv)==4 and sys.argv[1]=='--continue'
    if not continuation and len(sys.argv)!=3:raise SystemExit('usage: operator_retirement.py [--continue] REVIEWED_PRIVATE_DOCUMENT SHA256')
    try:
        code=Path(__file__).resolve().parents[1]
        result=(continue_retirement(code,sys.argv[2],sys.argv[3]) if continuation else retire(code,sys.argv[1],sys.argv[2]))
        print(json.dumps(result,sort_keys=True))
    except Exception:
        print('NATS_OPERATOR_RETIREMENT=BLOCKED',file=sys.stderr);sys.exit(1)
