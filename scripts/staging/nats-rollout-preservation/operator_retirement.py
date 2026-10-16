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

def media_monitor(role):
    """Fixed read-only monitoring request with safe failure identity for recovery."""
    import commands
    if role not in {'hub',*LEAVES}:fail()
    name=HUB if role=='hub' else 'voice-'+role
    container='nats' if role=='hub' else 'nats-leaf'
    endpoint='varz' if role=='hub' else 'leafz'
    try:
        output=commands.capture(['/usr/local/bin/k3s','kubectl','--namespace',guard.NS,
            'exec','deployment/'+name,'-c',container,'--','/bin/busybox','wget','-q','-O','-',
            'http://127.0.0.1:8222/'+endpoint],timeout=30,limit=2<<20,
            env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
        return json.loads(output)
    except Blocked as error:
        diagnostic=dict(getattr(error,'diagnostic',{}))
        diagnostic['monitor']={'role':role,'endpoint':endpoint}
        error.diagnostic=diagnostic
        raise

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
    if 'operator_media_abort' in state:return verify_media_abort_terminal(base,state)
    if 'operator_reset' in state:
        from operator_reset import verify_terminal as verify_reset
        return verify_reset(base,state)
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


MEDIA_ABORT_OPERATION='0e2be62a3326'
MEDIA_ABORT_CHECKPOINT='157ae4eb5f7b78ded15a2caf5e9eea27f335e3d72a38be0c173f69297a632c56'
MEDIA_ABORT_COMPATIBILITY='36d7e663535c86ff3624b559596db58e5f692d767d5357440d24366f8b58d8a4'
MEDIA_ABORT_ACTIVE='e9d46f0aa438cc5142fb48a363aa0286d3ff71e09736c431665133a36fa54fbf'
MEDIA_ABORT_FINISH='b708d69b20bb6401f628ab9adb6cf4d4cb68cbc1e8d0f360e55475856a62264f.json'
MEDIA_ABORT_JOURNAL='c820f48bb7c2cfcd5b4199fde2584f7ada103e1c51655e7805d5260758a6f7af'
MEDIA_ABORT_PREFIX='operator-media-abort'
MEDIA_TARGET_PREFIX='operator-media-target'
MEDIA_TARGET_BLOCKED='790a980a0f53a031ca1688f7e9f7535661d9b2fb66fc3efe16feda332d5f2fbd'
MEDIA_TARGET_PROGRESS='96965ef71b1f0e787caa20646e9023d804e4736184fd3dd1a84bddc43973c3a9'
MEDIA_TARGET_BASIS='VERIFIED_GATED_TARGET_BASELINE'
MEDIA_RECONCILED_PREFIX='operator-media-target-reconciled'
MEDIA_RECONCILED_SCHEMA='voice-media-target-reconciled-closure-v1'
MEDIA_RECONCILED_REFERENCE='voice-media-target-reconciled-reference-v1'
# Exact failed target apply, not a generic resume or alternative checkpoint.
MEDIA_FAILED_APPLY={
    'operator-media-target-reconciled-origin-runner.py':{'bytes':30352,'sha256':'1fd0fd44572e5dcaea938e6f694d04d32fb5f09982d61ddc06f44dd3be786fb0'},
    'operator-media-target-reconciled-blocked.json':{'bytes':3352567,'sha256':'ab06554cf7f8f39d2c6c347129910c13911c00e94b2ef91acb8b863a6a16500b'},
    'operator-media-target-blocked.json':{'bytes':3342669,'sha256':'790a980a0f53a031ca1688f7e9f7535661d9b2fb66fc3efe16feda332d5f2fbd'},
    'operator-media-target-compiled.json':{'bytes':1076345,'sha256':'aab7efac28191be56b95768e13f2995467360a97af4c38a7f124658760698c83'},
    'operator-media-target-failure-1792043444801386280/failure.json':{'bytes':123,'sha256':'915f5c4237ddffb3f68fd7ad995cfb2ec1c424fb1361907f43f32f88e3f245ba'},
    'operator-media-target-normalized.json':{'bytes':213957,'sha256':'3e873536b81a61a1a9c8310d0a0ec4f9d7f3849b9b5e4828ef24985beeff221b'},
    'operator-media-target-parameters.json':{'bytes':9099,'sha256':'c9dc098d59c8860ff222c87f02458d6fe5bb474e9d185fa65417b31c0cfdc259'},
    'operator-media-target-prepared.json':{'bytes':317635,'sha256':'88025c41c1ca94caf4ab9ed948ecd2b12627c6cde54d946490c9dc2162a26277'},
    'operator-media-target-progress.json':{'bytes':609487,'sha256':'90ffab5b4850d88665116c9094447945d054914daf030914285cd76e30d6024a'},
    'operator-media-target-source-authority.json':{'bytes':704136,'sha256':'eb763ba5a03be9e60796e10ec5541a50ec63fc73fb9f706db4374c320758797b'}}
MEDIA_FAILED_APPLIED=tuple('Service/voice-'+name for name in (
    'social','user','space','chat','file','messaging','matchmaking','search','notification','realtime','bot',
    'subscription','moderation','story','analytics','gateway'))+tuple('NetworkPolicy/voice-'+name for name in (
    'social-privacy-consumers','social-principal-jwks','file-user-principal-listener','file-principal-jwks',
    'search-user-projection','search-projection-jwks'))
MEDIA_FAILED_ADMIN={
    'Deployment/voice-admin':{'uid':'e84f2e89-22e6-4d49-a665-9753131935d8','semantic_sha256':'9bd121c2f96316b8cf56a1ab94267f1ae683e193f909c590de7caf0f985bd11e'},
    'Service/voice-admin':{'uid':'603a3855-f197-4026-b50f-561a95cd0974','semantic_sha256':'9a26a59631e494097267463b98601c5322786681f21f3f4a32dd2477722cb958'},
    'Ingress/voice-admin-http':{'uid':'5fc523bb-106e-45df-8499-0ac6327ba64a','semantic_sha256':'3121a166f08535e241687610eee69a7faf4c49cfa4d95dc57a8daec6b2804f56'}}


def media_failed_apply(base):
    """Validate immutable failed bytes; UNKNOWN is history, never fence authority."""
    base=Path(base)
    for name,pin in MEDIA_FAILED_APPLY.items():
        value=private_bytes(base/name,128<<20)
        if {'bytes':len(value),'sha256':hashlib.sha256(value).hexdigest()}!=pin:fail()
    old=media_target_failed(base)
    state=document(base/(MEDIA_RECONCILED_PREFIX+'-blocked.json'))
    progress=document(base/(MEDIA_TARGET_PREFIX+'-progress.json'))
    if (state.get('operation')!=MEDIA_ABORT_OPERATION or state.get('status')!='BLOCKED'
        or state.get('phase')!='OPERATOR_RETIRING' or state.get('fence_status')!='UNKNOWN'
        or progress.get('phase')!='BLOCKED' or progress.get('context')!=state.get('context')
        or tuple(e.get('resource') for e in progress.get('events',[]) if e.get('kind')=='target_resource_applied')!=MEDIA_FAILED_APPLIED
        or any(state.get(k)!=old.get(k) for k in ('migrations','migration_jobs','active_contract'))):fail()
    for name in ('receipt','verified'):
        if (base/(MEDIA_TARGET_PREFIX+'-'+name+'.json')).exists():fail()
    return state


def media_failed_reconciliation(base,observed):
    """Exact historical partial apply: 22 recorded effects, Auth CAS, untouched rest."""
    from nonnats_plan import semantic
    media_failed_apply(base)
    normalized=document(Path(base)/(MEDIA_TARGET_PREFIX+'-normalized.json'))
    prepared=document(Path(base)/(MEDIA_TARGET_PREFIX+'-prepared.json'))
    media_target_prepared(normalized,prepared)
    keys=[row['kind']+'/'+row['metadata']['name'] for row in normalized]
    if len(keys)!=len(set(keys)) or set(keys)!=set(prepared['before']) or set(keys)!=set(observed):fail()
    if set(MEDIA_FAILED_APPLIED)-set(keys) or keys[len(MEDIA_FAILED_APPLIED)]!='Deployment/voice-auth':fail()
    result={}
    for key,target in zip(keys,normalized):
        before=prepared['before'][key];live=observed[key]
        expected=target if key in MEDIA_FAILED_APPLIED or key=='Deployment/voice-auth' else before
        if (live['metadata']['uid']!=before['metadata']['uid'] or target['metadata']['uid']!=before['metadata']['uid']
            or semantic(live)!=semantic(expected)):fail()
        result[key]={'uid':live['metadata']['uid'],'semantic_sha256':canonical(semantic(live))}
    if len({row['uid'] for row in result.values()})!=len(result):fail()
    return result


def media_failed_admin(base):
    """Derive new semantics only from the hash-bound old preserve descriptors."""
    from nonnats_plan import semantic
    media_failed_apply(base)
    prepared=document(Path(base)/(MEDIA_TARGET_PREFIX+'-prepared.json'))
    if prepared.get('outside_owned')!=MEDIA_FAILED_ADMIN:fail()
    result={}
    for row in prepared['preflight']['objects']:
        key=row['kind']+'/'+row['name']
        if key not in MEDIA_FAILED_ADMIN:continue
        old={'uid':row['uid'],'semantic_sha256':canonical(row['desired'])}
        if key in result or row.get('disposition')!='preserve' or old!=MEDIA_FAILED_ADMIN[key]:fail()
        result[key]={'uid':row['uid'],'semantic_sha256':canonical(semantic(row['desired']))}
    if set(result)!=set(MEDIA_FAILED_ADMIN):fail()
    return result


MEDIA_GATEWAY_PREFIX='operator-media-target-gateway'
MEDIA_GATEWAY_SCHEMA='voice-media-target-gateway-closure-v1'
MEDIA_GATEWAY_REFERENCE='voice-media-target-gateway-reference-v1'
# Exact successfully refenced Gateway startup failure; never retry authority.
MEDIA_FAILED_RESTART={'operator-media-target-failure-1792085273718754214/failure.json': {'bytes': 351,
                                                                    'sha256': 'a49d22e4be642440d7f66cec26bcb7e73f94cd6d5602296dce807dff97b0f667'},
 'operator-media-target-failure-1792085273718754214/voice-gateway-gateway-previous.log': {'bytes': 215,
                                                                                          'sha256': '9df31641e38dca808133933fc5228856f70ca46fae73ab67cf53ce739fdb0da8'},
 'operator-media-target-failure-1792085273718754214/voice-gateway-gateway.log': {'bytes': 215,
                                                                                 'sha256': '9df31641e38dca808133933fc5228856f70ca46fae73ab67cf53ce739fdb0da8'},
 'operator-media-target-gateway-blocked.json': {'bytes': 3385477,
                                                'sha256': 'b3077aef17197008868e7225a0aa42a2a8c247c82bed765a4dee10caae17cae9'},
 'operator-media-target-gateway-origin-runner.py': {'bytes': 32624,
                                                    'sha256': '5ab5a024c19c9250174474f6692aba0a9de21e3a84877efb97188d6350656b82'},
 'operator-media-target-reconciled-blocked.json': {'bytes': 3352567,
                                                   'sha256': 'ab06554cf7f8f39d2c6c347129910c13911c00e94b2ef91acb8b863a6a16500b'},
 'operator-media-target-reconciled-compiled.json': {'bytes': 1076311,
                                                    'sha256': 'cc3650d4cb199c952e88cb33ac7ed20783a76b52af182a82f68b02b62552e794'},
 'operator-media-target-reconciled-normalized.json': {'bytes': 214040,
                                                      'sha256': '122f004a44d03133037d7f9d890224e359ecb5a28832a30c62cda36ac6cdbb21'},
 'operator-media-target-reconciled-origin-runner.py': {'bytes': 30352,
                                                       'sha256': '1fd0fd44572e5dcaea938e6f694d04d32fb5f09982d61ddc06f44dd3be786fb0'},
 'operator-media-target-reconciled-parameters.json': {'bytes': 9099,
                                                      'sha256': 'd8f32cf76f6e966692fa44cddd1651a548db8c904d6f2f512109985766fc090a'},
 'operator-media-target-reconciled-prepared.json': {'bytes': 329800,
                                                    'sha256': '5b8ab75c1fa7fe068db7cb890f6527e9963f3d21d112df8dd0b49804b29f6f5d'},
 'operator-media-target-reconciled-progress.json': {'bytes': 637774,
                                                    'sha256': 'a4157129ea0b1c06a18ed545cb018bbb36a7d24c0dedc615a2c491eaa30fca2f'},
 'operator-media-target-reconciled-source-authority.json': {'bytes': 704276,
                                                            'sha256': '5b7a45c1647bde55d75f8fb4da407d4ade2884fd2513f9afdd221b04034a8ae1'}}


def media_restart_pins():
    result=dict(MEDIA_FAILED_APPLY)
    for name,pin in MEDIA_FAILED_RESTART.items():
        if name in result and result[name]!=pin:fail()
        result[name]=pin
    return result


def media_restart_failed(base):
    base=Path(base)
    for name,pin in media_restart_pins().items():
        value=private_bytes(base/name,128<<20)
        if {'bytes':len(value),'sha256':hashlib.sha256(value).hexdigest()}!=pin:fail()
    old=media_failed_apply(base)
    state=document(base/(MEDIA_GATEWAY_PREFIX+'-blocked.json'))
    progress=document(base/(MEDIA_RECONCILED_PREFIX+'-progress.json'))
    prepared=document(base/(MEDIA_RECONCILED_PREFIX+'-prepared.json'))
    normalized=document(base/(MEDIA_RECONCILED_PREFIX+'-normalized.json'))
    authority=document(base/(MEDIA_RECONCILED_PREFIX+'-source-authority.json'))
    media_target_prepared(normalized,prepared)
    expected=copy.deepcopy(old)
    expected.update(context=progress.get('context'),fence_status='VERIFIED',events=old['events']+progress.get('events',[]))
    final={'kind':'target_baseline_blocked','stage':'target-restart','class':'Blocked',
        'failure':'MEDIA_TARGET_BLOCKED stage=target-restart error=restart_readiness_timeout',
        'refence':{'verified':True,'hub_zero_pods':True,'recovery_failures':[]}}
    if (state!=expected or progress.get('phase')!='BLOCKED' or not progress.get('events')
        or progress['events'][-1]!=final or prepared.get('failed_apply')!=MEDIA_FAILED_APPLY
        or prepared.get('reconciliation')!=media_failed_reconciliation(base,prepared.get('before',{}))
        or prepared.get('outside_owned')!=media_failed_admin(base)
        or prepared.get('source_sha')!='ca7304dcaa613bba4d1e7d08087de2839e0a3b66'
        or prepared.get('binding_sha256')!='8fae034af1c05abed5373f17beba4633e6922ad389e8d7ccb68353a2d3ebaa4e'
        or authority.get('source_sha')!=prepared['source_sha'] or authority.get('run_id')!=38698269484):fail()
    for suffix in ('receipt','verified'):
        if (base/(MEDIA_RECONCILED_PREFIX+'-'+suffix+'.json')).exists():fail()
    snapshots=state['context']['snapshots'];original=state['context']['original_snapshots']
    names=CORE|{'voice-web','voice-developer-portal'}
    if (set(snapshots)!=names or set(original)!=names
        or snapshots[HUB]['metadata']['uid']!=original[HUB]['metadata']['uid']
        or snapshots[HUB]['spec']['template']!=old['context']['snapshots'][HUB]['spec']['template']):fail()
    targets={row['metadata']['name']:row for row in normalized if row['kind']=='Deployment'}
    if set(targets)!=names-{HUB}:fail()
    for name,row in snapshots.items():
        if row['spec'].get('replicas')!=0 or row['metadata']['uid']!=original[name]['metadata']['uid']:fail()
        if name!=HUB and row['spec']['template']!=targets[name]['spec']['template']:fail()
    if 'voice.io/nats-user-space-bootstrap' in snapshots['voice-user']['spec']['template'].get('metadata',{}).get('annotations',{}):fail()
    return state


def media_restart_reconciliation(base,observed):
    from nonnats_plan import semantic
    media_restart_failed(base)
    normalized=document(Path(base)/(MEDIA_RECONCILED_PREFIX+'-normalized.json'))
    prepared=document(Path(base)/(MEDIA_RECONCILED_PREFIX+'-prepared.json'))
    media_target_prepared(normalized,prepared)
    keys=[row['kind']+'/'+row['metadata']['name'] for row in normalized]
    if len(keys)!=46 or len(set(keys))!=46 or set(keys)!=set(prepared['before']) or set(keys)!=set(observed):fail()
    result={}
    for key,row in zip(keys,normalized):
        target=copy.deepcopy(row)
        if target['kind']=='Deployment':target['spec']['replicas']=0
        live=observed[key];uid=prepared['before'][key]['metadata']['uid']
        if live['metadata']['uid']!=uid or target['metadata']['uid']!=uid or semantic(live)!=semantic(target):fail()
        result[key]={'uid':uid,'semantic_sha256':canonical(semantic(live))}
    if len({row['uid'] for row in result.values()})!=46:fail()
    return result


def media_restart_workloads(base,observed):
    state=media_restart_failed(base);expected=state['context']['snapshots']
    if set(observed)!=set(expected) or len(observed)!=22:fail()
    for name,row in observed.items():
        if (row['metadata']['uid']!=expected[name]['metadata']['uid'] or row['spec'].get('replicas')!=0
            or row['spec']['template']!=expected[name]['spec']['template']):fail()
    return {name:{'uid':row['metadata']['uid'],'template_sha256':canonical(row['spec']['template'])} for name,row in observed.items()}



MEDIA_MONITOR_PREFIX='operator-media-target-monitor'
MEDIA_MONITOR_SCHEMA='voice-media-target-monitor-closure-v1'
MEDIA_MONITOR_REFERENCE='voice-media-target-monitor-reference-v1'
# Exact all-ready, monitoring-refused boundary. Historical bytes are immutable.
MEDIA_FAILED_MONITOR={'operator-media-target-failure-1792089890783937891/failure.json': {'bytes': 115,
                                                                    'sha256': 'c54ef0f2f884997303f6a13229329bcee68b559176c95cb98b09943244fc0889'},
 'operator-media-target-gateway-blocked.json': {'bytes': 3385477,
                                                'sha256': 'b3077aef17197008868e7225a0aa42a2a8c247c82bed765a4dee10caae17cae9'},
 'operator-media-target-gateway-compiled.json': {'bytes': 1071733,
                                                 'sha256': 'fdd5f97a1114530575d5d755286000181e269594fbe7234f938a6eb4fdefc83d'},
 'operator-media-target-gateway-normalized.json': {'bytes': 214214,
                                                   'sha256': '595fb73fa265383cadd095f6cb35a4a9fbcfc2a231bdef35406ed39329e62478'},
 'operator-media-target-gateway-origin-runner.py': {'bytes': 32624,
                                                    'sha256': '5ab5a024c19c9250174474f6692aba0a9de21e3a84877efb97188d6350656b82'},
 'operator-media-target-gateway-parameters.json': {'bytes': 9099,
                                                   'sha256': 'cd4b3fb47dffba24b75e18293035a01745e00888eeeea4f3740a7581c25e5f58'},
 'operator-media-target-gateway-prepared.json': {'bytes': 350997,
                                                 'sha256': '14b618229cd8c73d03fc660f4448ee80584a77b8701e0eb9a42b036e60f92624'},
 'operator-media-target-gateway-progress.json': {'bytes': 638223,
                                                 'sha256': '06116fb3e9c1d385144878513b13f04f88da63dd0707d0b015aa6d921fbdbd49'},
 'operator-media-target-gateway-source-authority.json': {'bytes': 704418,
                                                         'sha256': '78abaec5151162a069b0b81976f45a7be944b7fc060739fc61b092e46df9cf64'},
 'operator-media-target-monitor-blocked.json': {'bytes': 3395722,
                                                'sha256': 'cf937983714eacda294218a5e18bdef39f0161a76ba2a70b9a32af80e084858e'},
 'operator-media-target-monitor-origin-runner.py': {'bytes': 32973,
                                                    'sha256': '202e7413e4a77ed5a84d13bb0a55d3e29ce820cb67ac1fc6baa41c01145a06ca'}}

def media_monitor_pins():
    result=media_restart_pins()
    for name,pin in MEDIA_FAILED_MONITOR.items():
        if name in result and result[name]!=pin:fail()
        result[name]=pin
    return result


def media_monitor_failed(base):
    base=Path(base)
    for name,pin in media_monitor_pins().items():
        value=private_bytes(base/name,128<<20)
        if {'bytes':len(value),'sha256':hashlib.sha256(value).hexdigest()}!=pin:fail()
    old=media_restart_failed(base)
    state=document(base/(MEDIA_MONITOR_PREFIX+'-blocked.json'))
    progress=document(base/(MEDIA_GATEWAY_PREFIX+'-progress.json'))
    prepared=document(base/(MEDIA_GATEWAY_PREFIX+'-prepared.json'))
    normalized=document(base/(MEDIA_GATEWAY_PREFIX+'-normalized.json'))
    authority=document(base/(MEDIA_GATEWAY_PREFIX+'-source-authority.json'))
    media_target_prepared(normalized,prepared)
    expected=copy.deepcopy(old)
    expected.update(context=progress.get('context'),fence_status='VERIFIED',events=old['events']+progress.get('events',[]))
    final={'kind':'target_baseline_blocked','stage':'ready-baseline-proof','class':'Blocked',
        'failure':'MEDIA_TARGET_BLOCKED stage=ready-baseline-proof error=command_failed',
        'refence':{'verified':True,'hub_zero_pods':True,'recovery_failures':[]}}
    if (state!=expected or progress.get('phase')!='BLOCKED' or not progress.get('events')
        or progress['events'][-1]!=final or prepared.get('failed_restart')!=media_restart_pins()
        or prepared.get('reconciliation')!=media_restart_reconciliation(base,prepared.get('before',{}))
        or prepared.get('outside_owned')!=media_failed_admin(base)
        or prepared.get('source_sha')!='0917eaeac80830e4e83f0538bd1f06be26b07881'
        or prepared.get('binding_sha256')!='a426e34e1900d81a774a4b102b5da120a0e29035151a74a1495850334d0d9cd1'
        or authority.get('source_sha')!=prepared['source_sha'] or authority.get('run_id')!=38784502351):fail()
    for suffix in ('receipt','verified'):
        if (base/(MEDIA_GATEWAY_PREFIX+'-'+suffix+'.json')).exists():fail()
    snapshots=state['context']['snapshots'];original=state['context']['original_snapshots']
    names=CORE|{'voice-web','voice-developer-portal'}
    if (set(snapshots)!=names or set(original)!=names
        or snapshots[HUB]['metadata']['uid']!=original[HUB]['metadata']['uid']
        or snapshots[HUB]['spec']['template']!=old['context']['snapshots'][HUB]['spec']['template']):fail()
    targets={row['metadata']['name']:row for row in normalized if row['kind']=='Deployment'}
    if set(targets)!=names-{HUB}:fail()
    for name,row in snapshots.items():
        if row['spec'].get('replicas')!=0 or row['metadata']['uid']!=original[name]['metadata']['uid']:fail()
        if name!=HUB and row['spec']['template']!=targets[name]['spec']['template']:fail()
    if 'voice.io/nats-user-space-bootstrap' in snapshots['voice-user']['spec']['template'].get('metadata',{}).get('annotations',{}):fail()
    return state


def media_monitor_reconciliation(base,observed):
    from nonnats_plan import semantic
    media_monitor_failed(base)
    normalized=document(Path(base)/(MEDIA_GATEWAY_PREFIX+'-normalized.json'))
    prepared=document(Path(base)/(MEDIA_GATEWAY_PREFIX+'-prepared.json'))
    media_target_prepared(normalized,prepared)
    keys=[row['kind']+'/'+row['metadata']['name'] for row in normalized]
    if len(keys)!=46 or len(set(keys))!=46 or set(keys)!=set(prepared['before']) or set(keys)!=set(observed):fail()
    result={}
    for key,row in zip(keys,normalized):
        target=copy.deepcopy(row)
        if target['kind']=='Deployment':target['spec']['replicas']=0
        live=observed[key];uid=prepared['before'][key]['metadata']['uid']
        if live['metadata']['uid']!=uid or target['metadata']['uid']!=uid or semantic(live)!=semantic(target):fail()
        result[key]={'uid':uid,'semantic_sha256':canonical(semantic(live))}
    if len({row['uid'] for row in result.values()})!=46:fail()
    return result


def media_monitor_workloads(base,observed):
    state=media_monitor_failed(base);expected=state['context']['snapshots']
    if set(observed)!=set(expected) or len(observed)!=22:fail()
    for name,row in observed.items():
        if (row['metadata']['uid']!=expected[name]['metadata']['uid'] or row['spec'].get('replicas')!=0
            or row['spec']['template']!=expected[name]['spec']['template']):fail()
    return {name:{'uid':row['metadata']['uid'],'template_sha256':canonical(row['spec']['template'])} for name,row in observed.items()}


def media_target_failed(base):
    """The failed original-image attempt remains immutable, not a retry token."""
    raw=private_bytes(Path(base)/(MEDIA_TARGET_PREFIX+'-blocked.json'))
    progress=private_bytes(Path(base)/(MEDIA_ABORT_PREFIX+'-progress.json'))
    if hashlib.sha256(raw).hexdigest()!=MEDIA_TARGET_BLOCKED or hashlib.sha256(progress).hexdigest()!=MEDIA_TARGET_PROGRESS:fail()
    state=json.loads(raw);row=json.loads(progress)
    if (state.get('operation')!=MEDIA_ABORT_OPERATION or state.get('phase')!='OPERATOR_RETIRING'
        or state.get('status')!='BLOCKED' or state.get('fence_status')!='VERIFIED'
        or row.get('phase')!='BLOCKED' or row.get('context')!=state.get('context')
        or row.get('snapshot_sha256')!=MEDIA_ABORT_CHECKPOINT):fail()
    for name in ('receipt.json','verified.json'):
        try:(Path(base)/(MEDIA_ABORT_PREFIX+'-'+name)).lstat()
        except FileNotFoundError:pass
        else:fail()
    return state


def media_target_templates(snapshot,authority,pack,normalized):
    """Bind the manual baseline to independently captured master CI and images."""
    import source_authority
    try:
        target=pack['target'];source=authority['source_sha'];snapshots=snapshot['context']['snapshots']
        if (authority.get('schema')!='voice-source-authority-v1' or authority.get('verified') is not True
            or authority.get('repository')!=source_authority.REPO or authority.get('workflow_id')!=source_authority.WORKFLOW
            or any(type(authority.get(k)) is not int or authority[k]<=0 for k in ('run_id','run_attempt','lock_artifact_id'))
            or not re.fullmatch('[a-f0-9]{40}',source) or not re.fullmatch('[a-f0-9]{40}',authority.get('tree_sha',''))
            or not re.fullmatch('[a-f0-9]{64}',authority.get('lock_sha256',''))
            or authority.get('image_provenance')!='independently resolved trusted CI tag'
            or set(authority['images'])!=set(source_authority.IMAGE_NAMES)
            or target.get('tag')!=source or target.get('mode')!='app-only' or target.get('changed_services')!=[]
            or target.get('manifest_sha256')!=canonical(pack['manifests'])
            or target.get('migration_sha256')!=canonical(pack['migrations'])
            or pack['migrations']!=snapshot['migrations'] or pack['contract']!=snapshot['active_contract']['contract']
            or not target.get('source_hashes')
            or any(authority['source_files'].get(k)!=v for k,v in target['source_hashes'].items())):fail()
        for name,image in authority['images'].items():
            if not re.fullmatch(re.escape(source_authority.REGISTRY+'/'+name)+r'@sha256:[a-f0-9]{64}',image):fail()
        def inventory(rows):
            result={}
            for row in rows:
                key=(row.get('kind'),row.get('metadata',{}).get('name'))
                if (key in result or key[0] not in ('Deployment','Service','ConfigMap','NetworkPolicy','Ingress')
                    or not isinstance(key[1],str) or key[1].startswith('voice-nats')
                    or row.get('metadata',{}).get('namespace')!=guard.NS):fail()
                result[key]=row
            return result
        source_rows=inventory(pack['manifests']);actual_rows=inventory(normalized)
        if source_rows.keys()!=actual_rows.keys():fail()
        result={HUB:copy.deepcopy(snapshots[HUB]['spec']['template'])};used={}
        for (kind,name),row in actual_rows.items():
            if kind!='Deployment':continue
            if name not in snapshots or name==HUB:fail()
            original=source_rows[(kind,name)]['spec']['template'];template=row['spec']['template']
            if target['template_hashes'].get(name)!=canonical(original):fail()
            old={c['name']:c['image'] for c in snapshots[name]['spec']['template']['spec'].get('initContainers',[])+snapshots[name]['spec']['template']['spec']['containers']}
            expected={c['name']:c['image'] for c in original['spec'].get('initContainers',[])+original['spec']['containers']}
            actual={c['name']:c['image'] for c in template['spec'].get('initContainers',[])+template['spec']['containers']}
            if actual!=expected:fail()
            service=name.removeprefix('voice-')
            for container,image in actual.items():
                wanted=authority['images'][service] if image.split('@')[0]==source_authority.REGISTRY+'/'+service else old.get(container)
                if image!=wanted or not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}@sha256:[a-f0-9]{64}',image):fail()
                used[name+'/'+container]=image
            if authority['images'][service] not in actual.values():fail()
            result[name]=template
        if len(result)!=22 or set(result)!=set(snapshots) or used!=target['images']:fail()
        return result
    except (KeyError,TypeError,AttributeError):fail()


def media_target_prepared(normalized,prepared):
    """Bind every normalized resource to the immutable prepare-time capture."""
    if prepared.get('normalized_sha256')!=canonical(normalized):fail()


def media_target_resources(normalized):
    """Every applied resource, including service/configuration semantics, at release."""
    from nonnats_plan import semantic
    result={}
    for row in normalized:
        wanted=copy.deepcopy(row);metadata=wanted.get('metadata',{})
        key=wanted.get('kind','')+'/'+metadata.get('name','');uid=metadata.get('uid')
        if key in result or not isinstance(uid,str) or not uid:fail()
        if wanted.get('kind')=='Deployment':wanted['spec']['replicas']=1
        result[key]={'uid':uid,'semantic_sha256':canonical(semantic(wanted))}
    return result


def media_abort_snapshot(raw):
    """Only the independently inspected, refenced failed FULL may be restored."""
    if hashlib.sha256(raw).hexdigest()!=MEDIA_ABORT_CHECKPOINT:fail()
    value=json.loads(raw)
    if (value.get('operation')!=MEDIA_ABORT_OPERATION or value.get('phase')!='RESTART'
        or value.get('status')!='BLOCKED' or value.get('fence_status')!='VERIFIED'
        or value.get('error')!='restart_readiness_timeout' or len(value.get('migration_jobs',[]))!=16
        or canonical(value.get('active_contract'))!=MEDIA_ABORT_ACTIVE
        or value.get('preservation',{}).get('native_files_verified') is not True
        or value.get('nats_migration',{}).get('verified') is not True):fail()
    return value


def media_abort_templates(snapshot):
    """Restore reviewed application images; retain the already proved HUB target."""
    result={}
    for name,row in snapshot['context']['original_snapshots'].items():
        template=copy.deepcopy((snapshot['context']['snapshots'][name] if name==HUB else row)['spec']['template'])
        if name!=HUB:
            for container in template['spec'].get('initContainers',[])+template['spec']['containers']:
                image=snapshot['context']['old_images'].get(name+'/'+container['name'],'')
                if not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}@sha256:[a-f0-9]{64}',image):fail()
                container['image']=image
        result[name]=template
    if len(result)!=22 or HUB not in result:fail()
    return result


def media_abort_journal(installed):
    path=Path(installed)/'journal'/MEDIA_ABORT_FINISH
    raw=private_bytes(path,2<<20);row=json.loads(raw);request=row.get('request',{})
    if (hashlib.sha256(raw).hexdigest()!=MEDIA_ABORT_JOURNAL or row.get('phase')!='STARTED'
        or request.get('action')!='finish' or request.get('operation')!=MEDIA_ABORT_OPERATION
        or request.get('nonce')+'.json'!=MEDIA_ABORT_FINISH or canonical(request)!=row.get('request_sha256')):fail()
    # The root-direct continuation wrote no public response. Preserve that fact.
    try:(Path(installed)/'responses'/MEDIA_ABORT_FINISH).lstat()
    except FileNotFoundError:pass
    else:fail()
    return {'name':MEDIA_ABORT_FINISH,'journal_sha256':MEDIA_ABORT_JOURNAL,'response_absent':True}


def media_abort_terminal(snapshot,receipt):
    state=copy.deepcopy(snapshot)
    state.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED')
    state['operator_media_abort']={'schema':'voice-media-original-restore-reference-v1',
        'snapshot_sha256':MEDIA_ABORT_CHECKPOINT,'receipt_sha256':canonical(receipt)}
    if receipt.get('configuration_basis')==MEDIA_TARGET_BASIS:
        state['operator_media_abort'].update(schema='voice-media-target-baseline-reference-v1',
            blocked_sha256=MEDIA_TARGET_BLOCKED,progress_sha256=MEDIA_TARGET_PROGRESS)
        if receipt.get('schema')==MEDIA_RECONCILED_SCHEMA:
            state['operator_media_abort'].update(schema=MEDIA_RECONCILED_REFERENCE,
                blocked_sha256=MEDIA_FAILED_APPLY[MEDIA_RECONCILED_PREFIX+'-blocked.json']['sha256'],
                progress_sha256=MEDIA_FAILED_APPLY[MEDIA_TARGET_PREFIX+'-progress.json']['sha256'])
        if receipt.get('schema')==MEDIA_GATEWAY_SCHEMA:
            state['operator_media_abort'].update(schema=MEDIA_GATEWAY_REFERENCE,
                blocked_sha256=MEDIA_FAILED_RESTART[MEDIA_GATEWAY_PREFIX+'-blocked.json']['sha256'],
                progress_sha256=MEDIA_FAILED_RESTART[MEDIA_RECONCILED_PREFIX+'-progress.json']['sha256'])
        if receipt.get('schema')==MEDIA_MONITOR_SCHEMA:
            state['operator_media_abort'].update(schema=MEDIA_MONITOR_REFERENCE,
                blocked_sha256=MEDIA_FAILED_MONITOR[MEDIA_MONITOR_PREFIX+'-blocked.json']['sha256'],
                progress_sha256=MEDIA_FAILED_MONITOR[MEDIA_GATEWAY_PREFIX+'-progress.json']['sha256'])
    state['events']=snapshot['events']+[{'kind':'operator_media_abort_completed',
        'deployment_pass':False,'rollback_eligible':False,'forward_migrations_retained':True}]
    return state


def verify_media_abort_terminal(base,state):
    """Exact operator abort, not successful deployment or generic rollback authority."""
    base=Path(base);reference=state.get('operator_media_abort',{}).get('schema')
    reconciled=reference==MEDIA_RECONCILED_REFERENCE
    gateway=reference==MEDIA_GATEWAY_REFERENCE
    monitor=reference==MEDIA_MONITOR_REFERENCE
    target_baseline=monitor or gateway or reconciled or reference=='voice-media-target-baseline-reference-v1'
    prefix=MEDIA_MONITOR_PREFIX if monitor else MEDIA_GATEWAY_PREFIX if gateway else MEDIA_RECONCILED_PREFIX if reconciled else MEDIA_TARGET_PREFIX if target_baseline else MEDIA_ABORT_PREFIX
    if base!=guard.ROOT/('rollout-'+MEDIA_ABORT_OPERATION):fail()
    snapshot=media_abort_snapshot(private_bytes(base/(MEDIA_ABORT_PREFIX+'-snapshot.json')))
    blocked=media_monitor_failed(base) if monitor else media_restart_failed(base) if gateway else media_failed_apply(base) if reconciled else media_target_failed(base) if target_baseline else snapshot
    receipt=document(base/(prefix+'-receipt.json'))
    if (set(receipt)!={'schema','operation','waiver','snapshot_sha256','compatibility_sha256',
        'deployment_pass','rollback_eligible','configuration_basis','migration_jobs_sha256',
        'active_contract_sha256','setup','released_marker','retained_files','journal'}
        or receipt.get('schema')!=(MEDIA_MONITOR_SCHEMA if monitor else MEDIA_GATEWAY_SCHEMA if gateway else MEDIA_RECONCILED_SCHEMA if reconciled else 'voice-media-target-baseline-closure-v1' if target_baseline else 'voice-media-original-restore-closure-v1')
        or receipt.get('operation')!=MEDIA_ABORT_OPERATION or receipt.get('waiver')!=WAIVER
        or receipt.get('snapshot_sha256')!=MEDIA_ABORT_CHECKPOINT
        or receipt.get('compatibility_sha256')!=MEDIA_ABORT_COMPATIBILITY
        or receipt.get('deployment_pass') is not False or receipt.get('rollback_eligible') is not False
        or receipt.get('configuration_basis')!=(MEDIA_TARGET_BASIS if target_baseline else 'ORIGINAL_APPLICATION_IMAGES_RETAIN_MIGRATED_NATS_AND_DATABASES')
        or receipt.get('migration_jobs_sha256')!=canonical(snapshot['migration_jobs'])
        or receipt.get('active_contract_sha256')!=MEDIA_ABORT_ACTIVE
        or root_cli.code_binding(base/'code')!=snapshot['code_capture']
        or receipt.get('journal')!=media_abort_journal(guard.ROOT/'installed')
        or state!=media_abort_terminal(blocked,receipt)):fail()
    if target_baseline:
        authority=document(base/(prefix+'-source-authority.json'))
        pack=document(base/(prefix+'-compiled.json'))
        normalized=document(base/(prefix+'-normalized.json'))
        prepared=document(base/(prefix+'-prepared.json'))
        media_target_prepared(normalized,prepared)
        if reconciled:
            if (prepared.get('failed_apply')!=MEDIA_FAILED_APPLY
                or prepared.get('reconciliation')!=media_failed_reconciliation(base,prepared.get('before',{}))
                or prepared.get('outside_owned')!=media_failed_admin(base)):fail()
        if gateway:
            if (prepared.get('failed_restart')!=media_restart_pins()
                or prepared.get('reconciliation')!=media_restart_reconciliation(base,prepared.get('before',{}))
                or prepared.get('outside_owned')!=media_failed_admin(base)):fail()
        if monitor:
            if (prepared.get('failed_monitor')!=media_monitor_pins()
                or prepared.get('reconciliation')!=media_monitor_reconciliation(base,prepared.get('before',{}))
                or prepared.get('outside_owned')!=media_failed_admin(base)):fail()
        if (blocked.get('migrations')!=snapshot.get('migrations') or blocked.get('migration_jobs')!=snapshot.get('migration_jobs')
            or blocked.get('active_contract')!=snapshot.get('active_contract')
            or precut_retained_setup(blocked)!=precut_retained_setup(snapshot)):fail()
        templates=media_target_templates(blocked,authority,pack,normalized)
    else:templates=media_abort_templates(snapshot)
    expected={name:{'uid':snapshot['context']['original_snapshots'][name]['metadata']['uid'],
        'template_sha256':canonical(template),'replicas':1,'ready':True} for name,template in templates.items()}
    if receipt.get('setup')!=expected:fail()
    marker=snapshot['context']['marker'];after=receipt.get('released_marker',{})
    data=dict(marker['data'],phase='active');data.pop('knownRolloutOperation')
    if (set(after)!={'uid','resourceVersion','data'} or after.get('uid')!=marker['metadata']['uid']
        or after.get('data')!=data or not re.fullmatch('[0-9]+',after.get('resourceVersion',''))
        or after['resourceVersion']==marker['metadata']['resourceVersion']):fail()
    required={'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json','rollout-backup.cms',
        'decrypted-restore.json','apply-manifests.json','apply-authorization.json',
        prefix+'-compatibility.md',prefix+'-active-contract.json',prefix+'-verified.json',snapshot['preservation']['archive']}
    if target_baseline:
        required|={prefix+'-blocked.json',MEDIA_ABORT_PREFIX+'-progress.json',MEDIA_ABORT_PREFIX+'-snapshot.json',
            prefix+'-source-authority.json',prefix+'-compiled.json',prefix+'-normalized.json',prefix+'-parameters.json',prefix+'-prepared.json'}
    if reconciled:required|=set(MEDIA_FAILED_APPLY)
    if gateway:required|=set(media_restart_pins())
    if monitor:required|=set(media_monitor_pins())
    files=receipt.get('retained_files',{})
    if set(files)!=required:fail()
    for name,pin in files.items():
        path=base/name
        if path.resolve()!=path or not path.is_relative_to(base):fail()
        raw=private_bytes(path,128<<20,group_read=True)
        if pin!={'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)}:fail()
    if files[prefix+'-compatibility.md']['sha256']!=MEDIA_ABORT_COMPATIBILITY:fail()
    if canonical(document(base/(prefix+'-active-contract.json')))!=MEDIA_ABORT_ACTIVE:fail()
    proof=document(base/(prefix+'-verified.json'))
    if (reconciled or gateway or monitor) and proof.get('outside_owned')!=media_failed_admin(base):fail()
    if target_baseline and proof.get('resources')!=media_target_resources(normalized):fail()
    if (proof.get('setup')!=expected or proof.get('storage')!=precut_retained_setup(snapshot)
        or proof.get('active_contract_sha256')!=MEDIA_ABORT_ACTIVE
        or proof.get('migration_jobs_sha256')!=receipt['migration_jobs_sha256']
        or proof.get('no_migration_replay') is not True):fail()
    actors=proof.get('actors',{});connections=proof.get('leaf_connections',{})
    roles=set(LEAVES)|{'gateway'}
    if actors.get('roles')!=sorted(roles) or set(actors.get('proofs',{}))!=roles or set(connections)!=set(LEAVES):fail()
    for role in roles:
        actor=actors['proofs'][role].get('proof',{})
        if (any(actor.get(key) is not True for key in ('signed_chain_verified','server_authentication_verified',
                'signed_effective_permissions_verified','broker_grant_compatibility_verified'))):fail()
    for role in LEAVES:
        if (connections[role].get('connected') is not True
            or not re.fullmatch('[a-f0-9]{64}',connections[role].get('remote_server_sha256',''))):fail()
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

# Explicit dispositions of two reviewed failed prepares. These are abandoned
# requests, not deployment success or proof that no external effects occurred.
SEARCH_BOOTSTRAP_REJECTIONS={
    'cd248d0e3ef8':{
        'nonce':'cd248d0e3ef8b90b2509c90ec40005467b9bef1a9f07bee3f13ffc78e5adcd87',
        'source_sha':'8e4aa9d1eb5441cf4ba21f18082a6b71f8564457','run_id':38478016683,
        'request_sha256':'f282c735b51818f6324114da5b7a51e3bb4ed24f65d0e4ad8e3a6f9e605d10b3',
        'journal_sha256':'8f75c21377a2aefb913f2b6c74af2fad8af3cfc82037994c89fcf45d1c889326',
        'response_sha256':'83fb20c9621d18d737818eb030b72008076d2afff606a37dac0c37ee8daa7a28',
        'stage':'nats-preflight','prepare_error':'unexpected','response_error':'blocked_unclassified',
        'source_tree':{'files':5590,'bytes':68147962,'sha256':'15dd40b1a71ab05c4ff919f207c0bbd23fcf704f5490b22c59d537d699fd7f11'},
        'build_tree':None,
    },
    'bc0e9541ff37':{
        'nonce':'bc0e9541ff37d449c225c862d87c87750e24b5b3d815c9774a4f7d4a54663f68',
        'source_sha':'8e4aa9d1eb5441cf4ba21f18082a6b71f8564457','run_id':38482268019,
        'request_sha256':'e076e899cfefc493557a2608e5961758d329b95a903390b99438fb591754f0be',
        'journal_sha256':'0c22603765346fa057a4b2c1469f8b757cfab74b35b8ee5d4f4e9492e56dabd1',
        'response_sha256':'35afa798fcd405b6568e043d48fedb94aefc41e7ca9fd00eec39a40fe1cec3a5',
        'stage':'target-build','prepare_error':'guard_rejected','response_error':'exception_Blocked',
        'source_tree':{'files':5590,'bytes':68147962,'sha256':'15dd40b1a71ab05c4ff919f207c0bbd23fcf704f5490b22c59d537d699fd7f11'},
        'build_tree':{'files':1,'bytes':11590,'sha256':'8d638dda10ac1af98ce1d682a51ecfb3e7b7062ad9ca63bb37de90d06b80eb5b'},
    },
}

def _retained_prepare_tree(path,expected):
    """Bounded public-source tree census plus custody witness for the swap."""
    path=Path(path)
    if expected is None:
        try:path.lstat()
        except FileNotFoundError:return None
        fail()
    if path.resolve(strict=True)!=path:fail()
    rows=[];custody=[];size=0;directories=0
    identity=lambda s:(s.st_dev,s.st_ino,s.st_mode,s.st_uid,s.st_gid,s.st_nlink,s.st_size,s.st_mtime_ns,s.st_ctime_ns)
    def directory(candidate):
        st=candidate.lstat()
        if not stat.S_ISDIR(st.st_mode) or st.st_uid!=0 or st.st_mode&0o022:fail()
        return identity(st)
    parent=directory(path.parent)
    for current,dirs,files in os.walk(path,followlinks=False,onerror=lambda error:fail()):
        current=Path(current);dirs.sort();files.sort();directories+=1
        if directories>10000 or len(rows)+len(files)>10000:fail()
        before=directory(current)
        # Reject linked directories even though walk would not descend them.
        for name in dirs:directory(current/name)
        for name in files:
            candidate=current/name;observed=candidate.lstat()
            if not stat.S_ISREG(observed.st_mode):fail()
            fd=os.open(candidate,READ)
            try:
                opened=os.fstat(fd)
                if (not stat.S_ISREG(opened.st_mode) or opened.st_uid!=0 or opened.st_mode&0o022
                    or opened.st_nlink!=1 or not 0<=opened.st_size<=16<<20 or identity(observed)!=identity(opened)):fail()
                size+=opened.st_size
                if size>128<<20:fail()
                digest=hashlib.sha256();count=0
                while raw:=os.read(fd,1<<20):
                    count+=len(raw)
                    if count>opened.st_size:fail()
                    digest.update(raw)
                if count!=opened.st_size or identity(opened)!=identity(os.fstat(fd)) or identity(opened)!=identity(candidate.lstat()):fail()
            finally:os.close(fd)
            relative=candidate.relative_to(path).as_posix()
            rows.append({'path':relative,'kind':'file','bytes':count,'sha256':digest.hexdigest()})
            custody.append((relative,identity(opened)))
        if directory(current)!=before:fail()
        custody.append((current.relative_to(path).as_posix(),before))
    if directory(path.parent)!=parent:fail()
    tree={'files':len(rows),'bytes':size,'sha256':canonical(rows)}
    if tree!=expected:fail()
    return {'tree':tree,'custody_sha256':canonical([parent,custody])}

def _verify_search_bootstrap_rejection(installed,path,row):
    installed=Path(installed);operation=path.stem[:12];spec=SEARCH_BOOTSTRAP_REJECTIONS[operation]
    request={'action':'prepare','nonce':spec['nonce'],'source_sha':spec['source_sha'],
        'run_id':spec['run_id'],'mode':'full','changed_services':[]}
    journal=private_bytes(path,2<<20);response=private_bytes(installed/'responses'/path.name,2<<20,group_read=True)
    if (path.name!=spec['nonce']+'.json' or canonical(request)!=spec['request_sha256']
        or hashlib.sha256(journal).hexdigest()!=spec['journal_sha256']
        or hashlib.sha256(response).hexdigest()!=spec['response_sha256']
        or json.loads(journal)!=row or row!={'schema':'voice-nats-bridge-request-v1','request_sha256':spec['request_sha256'],
            'request':request,'phase':'STARTED','prepare_stage':{'name':spec['stage'],'status':'STARTED'},'prepare_error':spec['prepare_error']}
        or json.loads(response)!={'status':'BLOCKED','error':spec['response_error']}):fail()
    try:(guard.ROOT/('rollout-'+operation)).lstat()
    except FileNotFoundError:pass
    else:fail()
    return {'disposition':'ABANDONED_PRECHECKPOINT','operation':operation,
        'source':_retained_prepare_tree(installed/'sources'/operation,spec['source_tree']),
        'build':_retained_prepare_tree(installed/'sources'/(operation+'-build'),spec['build_tree'])}

STORY_PREFENCE_NONCE='8d65ee47ce1bb71bbe325eb9b020749ec015fdf5c920270b54ecb0cdd5bc4cf0'
STORY_PREFENCE_FILES={
    'journal':('bfafa1fcaaaa787f21ae1d6f46fd4c5c67f25f78cd87650e1277cf680c5f0a5b',563),
    'responses':('35afa798fcd405b6568e043d48fedb94aefc41e7ca9fd00eec39a40fe1cec3a5',48),
    'reports':('79d0eeae729d92906f55cfee786c083a2e4de63343169d4ba7b85d2fb63f9c2f',928)}
STORY_PREFENCE_SOURCE={'files':5607,'bytes':68552714,'sha256':'298f19aedec66b164f2408ae40598736c259496ec662d9c152a5f934e0b30e90'}
STORY_PREFENCE_CUSTODY='4fae73c6dbd74980ff694cc1e596072c94a143414cc44937eb7f20792c3ce9e9'
STORY_PREFENCE_CHECKPOINT='bfa1426b858a4cb9c367dc7ec81a3418b194a75985542899b6e932d2056f86c2'
STORY_PREFENCE_RECEIPT='f951ce8711b383a9e7c2bb9da33869b79798b5b447225d5e31779e503d20bd4b'

def _verify_story_prefence_rejection(installed,path,row):
    """One failed pre-fence request remains BLOCKED evidence, never a rollout."""
    if path.name!=STORY_PREFENCE_NONCE+'.json':fail()
    for directory,(wanted,length) in STORY_PREFENCE_FILES.items():
        member=installed/'responses'/(STORY_PREFENCE_NONCE+'.report.json') if directory=='reports' else installed/directory/path.name
        raw=private_bytes(member,2<<20,group_read=directory!='journal')
        if len(raw)!=length or hashlib.sha256(raw).hexdigest()!=wanted:fail()
    sha='b25087c728cdfeb40b9e4f8cb37cb8e441e122fa'
    request={'action':'prepare','nonce':STORY_PREFENCE_NONCE,'source_sha':sha,'run_id':38798032967,
        'run_attempt':1,'report_run_id':38798032967,'report_source_sha':sha,'mode':'full','changed_services':[]}
    if row!={'schema':'voice-nats-bridge-request-v1','request_sha256':'4f69074c5bbf08f05a327f7e3e00fd69a8c6fbdbb631271a8c02e8755d673311',
        'request':request,'phase':'STARTED','prepare_stage':{'name':'nats-preflight','status':'STARTED'},'prepare_error':'guard_rejected'}:fail()
    operation=STORY_PREFENCE_NONCE[:12]
    for candidate in (guard.ROOT/('rollout-'+operation),installed/'sources'/(operation+'-build')):
        try:candidate.lstat()
        except FileNotFoundError:continue
        fail()
    source=_retained_prepare_tree(installed/'sources'/operation,STORY_PREFENCE_SOURCE)
    # The shared sources parent's timestamps change when later operations add
    # siblings. Content/policy are historical invariants; this full metadata
    # fingerprint is transient and the installer compares it across the swap.
    base=guard.ROOT/('rollout-'+MEDIA_ABORT_OPERATION)
    if (hashlib.sha256(private_bytes(base/'checkpoint.json')).hexdigest()!=STORY_PREFENCE_CHECKPOINT
        or hashlib.sha256(private_bytes(base/(MEDIA_MONITOR_PREFIX+'-receipt.json'))).hexdigest()!=STORY_PREFENCE_RECEIPT):fail()
    verify_media_abort_terminal(base,document(base/'checkpoint.json'))
    return {'disposition':'ABANDONED_PRECHECKPOINT','status':'BLOCKED','operation':operation,
        'files':STORY_PREFENCE_FILES,'source':source,'checkpoint_sha256':STORY_PREFENCE_CHECKPOINT,
        'receipt_sha256':STORY_PREFENCE_RECEIPT}

def terminal_finish_disposition(installed,path):
    """Read-only admission of a failed finish superseded by the same completed finish.

    A root-attested, closed file map preserves both transport outcomes. PASS alone
    never admits a pending request, and this receipt grants no replay authority.
    """
    import bridge
    installed=Path(installed);path=Path(path)
    receipt_path=installed/('terminal-finish-disposition-'+path.stem+'.json')
    try:receipt_path.lstat()
    except FileNotFoundError:return None
    try:return _terminal_finish_disposition(installed,path,receipt_path)
    except (KeyError,IndexError,TypeError,ValueError,OSError,bridge.BridgeError):fail()


def _terminal_finish_disposition(installed,path,receipt_path):
    import bridge
    receipt_raw=private_bytes(receipt_path,2<<20);receipt=json.loads(receipt_raw)
    if set(receipt)!={'schema','operation','failed_nonce','completed_nonce','code_binding_sha256','files'}:fail()
    if receipt['schema']!='voice-terminal-finish-disposition-v1':fail()
    operation=receipt['operation'];failed=receipt['failed_nonce'];completed=receipt['completed_nonce']
    if (not re.fullmatch('[a-f0-9]{12}',operation) or not re.fullmatch('[a-f0-9]{64}',completed)
        or failed!=path.stem or failed==completed or path!=installed/'journal'/(failed+'.json')):fail()
    base=guard.ROOT/('rollout-'+operation)
    paths={'checkpoint':base/'checkpoint.json','authorization':base/'apply-authorization.json'}
    for label,nonce in (('failed',failed),('completed',completed)):
        paths[label+'_journal']=installed/'journal'/(nonce+'.json')
        paths[label+'_response']=installed/'responses'/(nonce+'.json')
        paths[label+'_report']=installed/'responses'/(nonce+'.report.json')
    repair_names={'repair_before':base/'operator-service-rv-before.json','repair_receipt':base/'operator-service-rv-reconciliation.json'}
    pins=receipt['files']
    if set(pins) not in (set(paths),set(paths)|set(repair_names)):fail()
    paths.update({k:v for k,v in repair_names.items() if k in pins})
    raw={};values={}
    for name,member in paths.items():
        raw[name]=private_bytes(member,group_read=name=='authorization' or member.parent==installed/'responses')
        if pins[name]!={'sha256':hashlib.sha256(raw[name]).hexdigest(),'bytes':len(raw[name])}:fail()
        values[name]=json.loads(raw[name])
    old=values['failed_journal'];new=values['completed_journal']
    if (set(old)!={'schema','request','request_sha256','phase','prepare_error'} or old['phase']!='STARTED'
        or old['prepare_error']!='guard_rejected' or set(new)!={'schema','request','request_sha256','phase','result'}
        or new['phase']!='COMPLETE'):fail()
    request=old['request']
    for row,nonce in ((old,failed),(new,completed)):
        bridge.validate(row['request'])
        if (row['schema']!='voice-nats-bridge-request-v1' or row['request_sha256']!=canonical(row['request'])
            or row['request']!={**request,'nonce':nonce}):fail()
    if set(request)!={'action','nonce','operation','claim_rv','report_run_id','report_source_sha','run_attempt'} or request['action']!='finish' or request['operation']!=operation:fail()
    state=values['checkpoint'];auth=values['authorization']
    result={'operation':operation,'phase':'POST_RESTART','status':'PASS','fence_status':'RELEASED'}
    if (state.get('schema')!='nats-rollout-root-v1' or any(state.get(k)!=v for k,v in result.items())
        or state.get('error') is not None or new['result']!=result or values['completed_response']!=result
        or values['failed_response']!={'status':'BLOCKED','error':'exception_Blocked'}):fail()
    binding=root_cli.code_binding(base/'code')
    if state['code_capture']!=binding or receipt['code_binding_sha256']!=canonical(binding):fail()
    execution=state['execution_authority'];source=state['source_authority']
    if (execution['dispatcher_run_id']!=request['report_run_id'] or execution['run_attempt']!=request['run_attempt']
        or execution['head_sha']!=request['report_source_sha'] or source['source_sha']!=request['report_source_sha']
        or source.get('verified') is not True or state['authorization']!=auth
        or auth['schema']!='nats-rollout-preservation-v1' or auth['operation']!=operation or auth['target']!=state['target']):fail()
    observed=[e['marker'] for e in state['events'] if e.get('kind')=='rollout_target_observed']
    verified=[e['marker'] for e in state['events'] if e.get('kind')=='rollout_verified']
    if not observed or not verified:fail()
    for marker in observed:
        if (marker['metadata']['uid']!=auth['marker']['uid'] or marker['metadata']['resourceVersion']!=request['claim_rv']
            or marker['data'].get('phase')!='rollout-applying' or marker['data'].get('knownRolloutOperation')!=operation):fail()
    for marker in verified:
        if marker['metadata']['uid']!=auth['marker']['uid'] or marker['data'].get('phase')!='rollout-verified' or marker['data'].get('knownRolloutOperation')!=operation:fail()
    released=state['context']['marker']
    if (released['metadata']['uid']!=auth['marker']['uid'] or released['data'].get('phase')!='active'
        or any(k in released['data'] for k in ('knownRolloutOperation','knownBaselineOperation'))
        or any(released['data'].get(k)!=auth['marker'][k] for k in ('generation','previousGeneration','dataPVC'))):fail()
    if not state.get('preservation') or not state.get('restart'):fail()
    for label,nonce,status in (('failed',failed,'BLOCKED'),('completed',completed,'COMPLETE')):
        report=values[label+'_report']
        expected={'schema':'voice-deployment-report-v1','request_action':'finish','request_nonce':nonce,'request_status':status,
            'operation':operation,'run_id':request['report_run_id'],'run_attempt':request['run_attempt'],
            'source_sha':request['report_source_sha'],'helper_binding':canonical(binding)}
        if any(report.get(k)!=v for k,v in expected.items()) or report['effects']['apply']!='RECORDED':fail()
    if (values['completed_report']['operation_status']!='PASS' or values['completed_report']['stage']!='POST_RESTART'
        or values['completed_report']['error_code']!='none'
        or any(values['completed_report']['effects'].get(k)!=v for k,v in {'release':'RECORDED','preservation':'PRESENT','restart':'PRESENT','fence':'RELEASED'}.items())
        or values['failed_report']['operation_status']!='WAITING' or values['failed_report']['stage']!='PAUSED_APPLY'
        or values['failed_report']['effects'].get('fence')!='VERIFIED'):fail()
    repairs=[e for e in state['events'] if e.get('kind')=='operator_preserved_service_rv_reconciled']
    if bool(repairs)!=('repair_before' in pins) or len(repairs)>1:fail()
    if repairs:_terminal_finish_service_repair(values,raw,repairs[0],request,receipt)
    if private_bytes(receipt_path,2<<20)!=receipt_raw:fail()
    return {'disposition':'SUPERSEDED_FAILED_FINISH','status':'BLOCKED','operation':operation,
        'receipt_sha256':hashlib.sha256(receipt_raw).hexdigest(),'files':pins}


def _terminal_finish_service_repair(values,raw,event,request,disposition):
    before=values['repair_before'];repair=values['repair_receipt'];after=copy.deepcopy(before)
    versions={'voice-auth':('3310867','3320022'),'voice-role':('3310882','3320032'),'voice-voice':('3310907','3320048')}
    if (repair.get('schema')!='voice-exact-5e-service-rv-reconciliation-v1' or repair.get('operation')!=request['operation']
        or request['operation']!='5e48553f3105' or before.get('operation')!=request['operation']
        or repair.get('before_checkpoint_sha256')!=hashlib.sha256(raw['repair_before']).hexdigest()
        or repair.get('authorization_sha256')!=hashlib.sha256(raw['authorization']).hexdigest()
        or repair.get('code_binding_sha256')!=disposition['code_binding_sha256'] or repair.get('claim_rv')!=request['claim_rv']
        or repair.get('workload_writes') is not False or repair.get('deployment_pass') is not False
        or before.get('phase')!='PAUSED_APPLY' or before.get('status')!='WAITING'
        or before.get('authorization')!=values['authorization'] or before.get('target')!=values['authorization']['target']
        or before.get('code_capture')!=values['checkpoint']['code_capture']):fail()
    wanted={'kind':'operator_preserved_service_rv_reconciled','before_checkpoint_sha256':repair['before_checkpoint_sha256'],
        'authorization_sha256':repair['authorization_sha256'],'claim_rv':repair['claim_rv'],'changes':repair['changes'],
        'reason':'FULL_APPLY_BOOKKEEPING_ONLY_FULL_SERVICE_SEMANTICS_UNCHANGED'}
    if event!=wanted or not repair['changes']:fail()
    seen=set()
    for change in repair['changes']:
        if set(change)!={'index','name','uid','before_rv','after_rv','semantic_sha256'} or type(change['index']) is not int or change['index'] in seen:fail()
        seen.add(change['index']);obj=after['nonnats_binding']['objects'][change['index']]
        if (change['index']<0 or obj['kind']!='Service' or obj['disposition']!='preserve' or obj['name']!=change['name']
            or versions.get(change['name'])!=(change['before_rv'],change['after_rv'])
            or obj['uid']!=change['uid'] or obj['resourceVersion']!=change['before_rv']
            or canonical(obj['desired'])!=change['semantic_sha256'] or not re.fullmatch('[0-9]{1,20}',change['after_rv'])):fail()
        obj['resourceVersion']=change['after_rv']
    required={i for i,obj in enumerate(before['nonnats_binding']['objects']) if obj['kind']=='Service' and obj['name'] in versions}
    if seen!=required or {change['name'] for change in repair['changes']}!=set(versions):fail()
    after['events'].append(event)
    deterministic=json.dumps(after,sort_keys=True,indent=2).encode()+b'\n'
    if hashlib.sha256(deterministic).hexdigest()!=repair['after_checkpoint_sha256']:fail()
    if after['nonnats_binding']!=values['checkpoint']['nonnats_binding'] or after['nonnats']!=values['checkpoint']['nonnats']:fail()
    if values['checkpoint']['events'][:len(after['events'])]!=after['events']:fail()


def compiler_inventory_disposition(installed,path,receipt=None):
    """Verify a separate root attestation; never replay or deployment authority."""
    installed=Path(installed);path=Path(path)
    if path.parent!=installed/'journal' or not re.fullmatch(r'[a-f0-9]{64}\.json',path.name):fail()
    destination=installed/('compiler-inventory-rejection-'+path.stem+'.json')
    receipt_raw=None
    if receipt is None:
        try:destination.lstat()
        except FileNotFoundError:return None
        receipt_raw=private_bytes(destination,2<<20);receipt=json.loads(receipt_raw)
    keys={'schema','nonce','disposition','deployment_pass','recovery_authorized','helper_binding',
        'files','old_receipt_sha256','source_tree','build_tree'}
    if (not isinstance(receipt,dict) or set(receipt)!=keys
        or receipt['schema']!='voice-compiler-inventory-rejection-v1' or receipt['nonce']!=path.stem
        or receipt['disposition']!='ABANDONED_PRECHECKPOINT' or receipt['deployment_pass'] is not False
        or receipt['recovery_authorized'] is not False
        or not re.fullmatch('[a-f0-9]{64}',receipt['helper_binding'])
        or not isinstance(receipt['files'],dict) or set(receipt['files'])!={'journal','response','report'}):fail()
    old=private_bytes(installed/PENDING_ABANDONMENT_NAME,2<<20)
    if hashlib.sha256(old).hexdigest()!=receipt['old_receipt_sha256'] or path.name in json.loads(old).get('entries',{}):fail()
    locations={'journal':path,'response':installed/'responses'/path.name,
        'report':installed/'responses'/(path.stem+'.report.json')}
    values={}
    for key,candidate in locations.items():
        raw=private_bytes(candidate,2<<20,group_read=key!='journal');pin=receipt['files'][key]
        if pin!={'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)}:fail()
        values[key]=json.loads(raw)
    row=values['journal'];request=row.get('request',{});operation=path.stem[:12]
    import bridge
    import deployment_report
    expected_fields={'action','nonce','source_sha','run_id','mode','changed_services','report_run_id','report_source_sha','run_attempt'}
    if (set(request)!=expected_fields or request.get('action')!='prepare' or request.get('nonce')!=path.stem
        or request.get('mode')!='images-only' or not request.get('changed_services')
        or request.get('run_id')!=request.get('report_run_id') or request.get('source_sha')!=request.get('report_source_sha')):fail()
    try:bridge.validate(dict(request,token='disposition-schema-check'))
    except bridge.BridgeError:fail()
    if (row!={'schema':'voice-nats-bridge-request-v1','request':request,'request_sha256':canonical(request),
            'phase':'STARTED','prepare_stage':{'name':'target-build','status':'STARTED'},'prepare_error':'guard_rejected'}
        or values['response']!={'status':'BLOCKED','error':'rollout_compile_image_inventory_changed'}):fail()
    report=values['report']
    try:deployment_report.validate(report,nonce=path.stem,source_sha=request['source_sha'],
        run_id=request['run_id'],run_attempt=request['run_attempt'])
    except (ValueError,TypeError,KeyError):fail()
    expected={'request_action':'prepare','request_status':'BLOCKED','operation':None,'operation_status':'UNKNOWN',
        'stage':'target-build','error_code':'rollout_compile_image_inventory_changed','helper_binding':receipt['helper_binding']}
    if any(report.get(k)!=v for k,v in expected.items()) or any(v!='UNKNOWN' for v in report['effects'].values()):fail()
    try:(guard.ROOT/('rollout-'+operation)).lstat()
    except FileNotFoundError:pass
    else:fail()
    source=_retained_prepare_tree(installed/'sources'/operation,receipt['source_tree'])
    build_path=installed/'sources'/(operation+'-build')
    if source is None or receipt['build_tree'] is None or {p.name for p in build_path.iterdir()}!={'parameters.json'}:fail()
    build=_retained_prepare_tree(build_path,receipt['build_tree'])
    if receipt_raw is not None and private_bytes(destination,2<<20)!=receipt_raw:fail()
    return {'disposition':'ABANDONED_COMPILER_INVENTORY_REJECTION','operation':operation,
        'receipt_sha256':hashlib.sha256(receipt_raw).hexdigest() if receipt_raw is not None else canonical(receipt),
        'files':receipt['files'],'source':source,'build':build,'old_receipt_sha256':receipt['old_receipt_sha256']}


def attest_compiler_inventory_rejection(reviewed_path,reviewed_sha):
    """Root-only, create-only attestation of an independently reviewed failure."""
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('operator_retirement_human_root_required')
    with operation_lock():
        installed=guard.ROOT/'installed'
        if any((installed/'inbox').iterdir()) or any((installed/'processing').iterdir()):fail()
        raw=private_bytes(reviewed_path,2<<20)
        if hashlib.sha256(raw).hexdigest()!=reviewed_sha:fail()
        receipt=json.loads(raw);nonce=receipt.get('nonce','')
        if not re.fullmatch('[a-f0-9]{64}',nonce):fail()
        path=installed/'journal'/(nonce+'.json')
        compiler_inventory_disposition(installed,path,receipt)
        destination=installed/('compiler-inventory-rejection-'+nonce+'.json')
        exclusive(destination,raw)
        return compiler_inventory_disposition(installed,path)


def pending_request_accepted(installed,path,closure):
    installed=Path(installed);path=Path(path)
    if path.parent!=installed/'journal' or not re.fullmatch('[a-f0-9]{64}\\.json',path.name):fail()
    row=document(path)
    if row.get('phase')=='COMPLETE':return
    compiler=compiler_inventory_disposition(installed,path)
    if compiler is not None:return compiler
    terminal=terminal_finish_disposition(installed,path)
    if terminal is not None:return terminal
    request=row.get('request',{})
    if request.get('operation')==MEDIA_ABORT_OPERATION:
        base=guard.ROOT/('rollout-'+MEDIA_ABORT_OPERATION)
        verify_media_abort_terminal(base,document(base/'checkpoint.json'))
        if path.name!=MEDIA_ABORT_FINISH:fail()
        return
    if request.get('operation')=='a83c5bdb26f1':
        from operator_reset import verify_terminal as verify_reset,verify_journals
        base=guard.ROOT/'rollout-a83c5bdb26f1'
        receipt=verify_reset(base,document(base/'checkpoint.json'))
        verify_journals(installed,receipt['journals'])
        if path.name not in receipt['journals']:fail()
        return
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
    if operation==STORY_PREFENCE_NONCE[:12]:
        return _verify_story_prefence_rejection(installed,path,row)
    if operation==SOURCE_CAPTURE_REJECTION['operation']:
        return _verify_source_capture_rejection(installed,path,row)
    if operation in SEARCH_BOOTSTRAP_REJECTIONS:
        return _verify_search_bootstrap_rejection(installed,path,row)
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
        if compiler_inventory_disposition(installed,path) is not None:continue
        if terminal_finish_disposition(installed,path) is not None:continue
        if path.name==STORY_PREFENCE_NONCE+'.json':
            _verify_story_prefence_rejection(installed,path,row)
            continue
        if row.get('request',{}).get('operation') in ('a83c5bdb26f1',MEDIA_ABORT_OPERATION):
            pending_request_accepted(installed,path,{'journals':{}})
            continue
        # Separately verified exact dispositions do not rewrite the old inventory.
        separate={FENCE_ABORT_NONCE+'.json',SOURCE_CAPTURE_REJECTION['nonce']+'.json',
            *(spec['nonce']+'.json' for spec in SEARCH_BOOTSTRAP_REJECTIONS.values())}
        if path.name not in separate:
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
