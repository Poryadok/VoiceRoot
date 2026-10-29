"""Source-owned STAGING preservation before prerequisite effects.

Only the existing root bridge can enter this producer. Private records are not
caller authorization, and a pending write is never reconciled by observation.
"""
import copy
import hashlib
import json
import os
import re
import secrets
from pathlib import Path

import commands
import guard
import resource_claim
import staging_prerequisites as private
from controller import Blocked
from root_main import save

NS='voice-staging'
PREFIX=['/usr/local/bin/k3s','kubectl','--namespace',NS]
SCHEMA='voice-staging-cohort-preservation-plan-v1'
PLAN_FIELDS={'schema','operation','context','helper_sha256','source_capture_sha256',
    'approved_sha256','execution_sha256','namespace_uid','precut_capture_sha256',
    'actual_before','candidate','prerequisite_inputs_sha256','source_inventory_sha256'}


def digest(value):
    return resource_claim.digest(value)


def _base(session):
    if not re.fullmatch(r'[a-f0-9]{12}',session.operation):raise Blocked('cohort_operation_invalid')
    return guard.ROOT/'installed'/'cohort-preservation'/session.operation


def _read(path,limit=32<<20):
    return private._json(private._read_private(path,limit=limit))


def _write(path,value):
    private._private_write(path,private._bytes(value))


def validate_plan(plan,session):
    try:
        if (type(plan) is not dict or set(plan)!=PLAN_FIELDS or plan['schema']!=SCHEMA
            or plan['operation']!=session.operation or plan['context']!=session.context
            or plan['helper_sha256']!=session.helper or plan['candidate']!={
                'commit':session.context['commit'],'images':session.context['images']}
            or not isinstance(plan['namespace_uid'],str) or not plan['namespace_uid']
            or not isinstance(plan['actual_before'],dict) or not plan['actual_before']
            or any(not isinstance(plan[name],str) or not re.fullmatch(r'[a-f0-9]{64}',plan[name])
                for name in PLAN_FIELDS if name.endswith('_sha256'))):raise ValueError()
    except (KeyError,TypeError,ValueError):raise Blocked('cohort_plan_invalid') from None


def _source(session,plan):
    parent=guard.ROOT/'installed'/'prerequisites'/session.operation
    approved=_read(parent/'approved.json');execution=_read(parent/'execution.json')
    receipt=_read(guard.ROOT/'installed'/'source-authority'/(session.operation+'.json'))
    prepared=_read(parent/'prepared.json')
    if (digest(receipt)!=plan['source_capture_sha256'] or digest(prepared)!=plan['prerequisite_inputs_sha256']
        or digest(approved)!=plan['approved_sha256'] or digest(execution)!=plan['execution_sha256']
        or receipt!={'schema':'voice-staging-source-capture-v1','operation':session.operation,
            'context':session.context,'helper_sha256':session.helper,'approved_sha256':digest(approved),
            'execution_sha256':digest(execution)}):raise Blocked('cohort_source_changed')
    source=guard.ROOT/'installed'/'sources'/session.operation
    private._verify_helper_source(session,approved,source)
    custody=_read(parent/'code-custody.json')
    if digest(approved['source_files'])!=plan['source_inventory_sha256']:raise Blocked('cohort_source_changed')
    for filename in ('staging_cohort_preservation.py','staging_cohort_preservation_runtime.py'):
        name='scripts/staging/nats-rollout-preservation/'+filename
        wanted=approved['source_files'].get(name)
        if (custody['manifest'].get('nats-rollout-preservation/'+filename)!=wanted or wanted is None
            or Path(__file__).with_name(filename)!=Path(custody['root'])/'nats-rollout-preservation'/filename
            or hashlib.sha256(Path(__file__).with_name(filename).read_bytes()).hexdigest()!=wanted):
            raise Blocked('cohort_executor_source_changed')
    return approved,execution,prepared


def load_plan(session):
    if type(session) is not resource_claim.Session or commands.authority_guard!=session.check:
        raise Blocked('cohort_session_required')
    session.check()
    state=_read(_base(session)/'checkpoint.json');plan=state['plan']
    validate_plan(plan,session);_source(session,plan)
    if state.get('schema')!='voice-staging-cohort-preservation-checkpoint-v1':raise Blocked('cohort_checkpoint_invalid')
    return copy.deepcopy(plan)


def _guard(session):
    plan=load_plan(session);row=session.check()
    if row.get('phase')!='cohort-preservation' or row.get('target_sha256')!=digest(plan):
        raise Blocked('cohort_binding_changed')
    return plan


def _send(session,effect_id,argv,body,*,limit=4<<20,timeout=60):
    _guard(session)
    permit={'effect_id':effect_id,'argv':list(argv),'body_sha256':hashlib.sha256(body).hexdigest()}
    pending=_read(_base(session)/'effects'/(effect_id+'.json'))
    if (pending.get('status')!='PENDING' or pending.get('operation')!=session.operation
        or pending.get('effect_id')!=effect_id or pending.get('binding')!=session.binding()
        or pending.get('argv')!=permit['argv'] or pending.get('body_sha256')!=permit['body_sha256']):
        raise Blocked('cohort_effect_changed')
    if getattr(session,'_cohort_permit',None) is not None:raise Blocked('cohort_permit_busy')
    session._cohort_permit=permit
    try:
        result=commands.capture(argv,body=body,limit=limit,timeout=timeout,
            env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
        if session._cohort_permit is not None:raise Blocked('cohort_permit_not_consumed')
        session.check()
        return result
    finally:session._cohort_permit=None


def _observation(session,label,argv,body=b'',*,limit=4<<20,timeout=30):
    """Private source-fixed observations; callers never supply this transport."""
    _guard(session);_no_pending(session);ident='observe-'+label+'-'+secrets.token_hex(8)
    directory=_base(session)/'effects';private._private_directory(directory)
    intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':ident,
        'binding':session.binding(),'argv':argv,'body_sha256':hashlib.sha256(body).hexdigest(),'status':'PENDING'}
    path=directory/(ident+'.json');_write(path,intent)
    raw=_send(session,ident,argv,body,limit=limit,timeout=timeout)
    if _read(path)!=intent:raise Blocked('cohort_effect_changed')
    save(path,{**intent,'status':'DONE','response_sha256':hashlib.sha256(raw).hexdigest()})
    return raw


def _no_pending(session):
    folder=_base(session)/'effects'
    if not folder.exists():return
    paths=list(folder.iterdir())
    if len(paths)>4096:raise Blocked('cohort_effect_inventory_invalid')
    for path in paths:
        if path.suffix!='.json' or _read(path).get('status')!='DONE':
            raise Blocked('cohort_reconciliation_required')


def version_read(session):
    return private._json(_observation(session,'version',PREFIX+['version','-o','json']))


def _precut(session):
    plan=load_plan(session);value=_read(_base(session)/'precut.json')
    if digest(value)!=plan['precut_capture_sha256']:raise Blocked('cohort_precut_changed')
    return value


def monitor_capture(session,pod_name):
    original=_precut(session)['hub_pod']
    if original['metadata']['name']!=pod_name:raise Blocked('cohort_hub_identity_changed')
    return _observation(session,'nats-monitor',PREFIX+['exec',pod_name,'-c','nats','--','/bin/busybox','wget','-q','-O','-',
        'http://127.0.0.1:8222/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048'],limit=64<<20)


def _cut_semantic(row):
    value=copy.deepcopy(row);value.pop('status',None)
    for key in ('uid','resourceVersion','generation','creationTimestamp','managedFields'):
        value['metadata'].pop(key,None)
    return value


def _cut_patch(session,kind,before,changes):
    """Closed cut operations; no template, Secret, storage or arbitrary CAS."""
    meta=before['metadata'];name=meta['name']
    if meta.get('namespace')!=NS or meta.get('deletionTimestamp') and kind!='pod':
        raise Blocked('cohort_cut_identity_changed')
    desired=copy.deepcopy(before)
    if kind=='deployment':
        if changes not in ([{'op':op,'path':'/spec/replicas','value':0}] for op in ('add','replace')):
            raise Blocked('cohort_cut_effect_forbidden')
        desired['spec']['replicas']=0
    elif kind=='configmap' and name=='voice-nats-generation':
        expected=[{'op':'test','path':'/data/phase','value':'active'},
            {'op':'replace','path':'/data/phase','value':'rollout-capturing'},
            {'op':'add','path':'/data/knownRolloutOperation','value':session.operation}]
        if changes!=expected or before['data'].get('phase')!='active':raise Blocked('cohort_cut_effect_forbidden')
        desired['data'].update(phase='rollout-capturing',knownRolloutOperation=session.operation)
    elif kind=='pod':
        finalizer='voice.io/nats-preservation-'+session.operation
        finals=meta.get('finalizers',[])
        if finalizer not in finals:
            if meta.get('deletionTimestamp') or changes!=[{'op':'add','path':'/metadata/finalizers','value':finals+[finalizer]}]:
                raise Blocked('cohort_cut_effect_forbidden')
            desired['metadata']['finalizers']=finals+[finalizer]
        else:
            expected=[{'op':'test','path':'/metadata/finalizers','value':finals},
                {'op':'remove','path':'/metadata/finalizers/'+str(finals.index(finalizer))}]
            if changes!=expected or not meta.get('deletionTimestamp'):raise Blocked('cohort_cut_effect_forbidden')
            from runtime_stage import terminated_hub
            terminated_hub(_precut(session)['hub_pod'],before)
            desired['metadata']['finalizers']=[v for v in finals if v!=finalizer]
    else:raise Blocked('cohort_cut_effect_forbidden')
    return desired


def cut_cas(session,kind,before,changes):
    """One fixed cut CAS, bound to its terminal API response and private intent."""
    plan=_guard(session)
    precut=_precut(session)
    expected=(precut['context']['original_snapshots'].get(before.get('metadata',{}).get('name'))
        if kind=='deployment' else precut['context']['marker'] if kind=='configmap'
        else precut.get('hub_pod') if kind=='pod' else None)
    if kind=='pod':
        if (expected is None or before['metadata']['uid']!=expected['metadata']['uid'] or before['spec']!=expected['spec']
            or before['metadata'].get('ownerReferences')!=expected['metadata'].get('ownerReferences')):
            raise Blocked('cohort_cut_scope_changed')
        if 'voice.io/nats-preservation-'+session.operation in before['metadata'].get('finalizers',[]):
            added=_read(_base(session)/'effects'/('cut-pod-'+before['metadata']['name']+'-retain.json'))
            if added.get('status')!='DONE' or added.get('response',{}).get('metadata',{}).get('uid')!=before['metadata']['uid']:
                raise Blocked('cohort_cut_scope_changed')
            expected=copy.deepcopy(before)
    if (expected is None or before.get('metadata',{}).get('uid')!=expected['metadata']['uid']
        or before['metadata'].get('generation')!=expected['metadata'].get('generation')
        or _cut_semantic(before)!=_cut_semantic(expected)):
        raise Blocked('cohort_cut_scope_changed')
    desired=_cut_patch(session,kind,before,changes);meta=before['metadata'];name=meta['name']
    if not re.fullmatch(r'voice-[a-z0-9-]{1,100}',name):raise Blocked('cohort_cut_identity_changed')
    directory=_base(session)/'effects';private._private_directory(directory)
    suffix=('-release' if before['metadata'].get('deletionTimestamp') else '-retain') if kind=='pod' else ''
    effect_id='cut-'+kind+'-'+name+suffix;path=directory/(effect_id+'.json')
    if path.exists():raise Blocked('cohort_reconciliation_required')
    _no_pending(session)
    body=private._bytes([{'op':'test','path':'/metadata/uid','value':meta['uid']},
        {'op':'test','path':'/metadata/resourceVersion','value':meta['resourceVersion']},*changes])
    argv=PREFIX+['patch',kind,name,'--type=json','--patch-file=/dev/stdin','-o','json']
    intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':effect_id,
        'binding':session.binding(),'before':before,'desired':desired,'argv':argv,
        'body_sha256':hashlib.sha256(body).hexdigest(),'status':'PENDING'}
    _write(path,intent)
    raw=_send(session,effect_id,argv,body);result=private._json(raw)
    if (result.get('metadata',{}).get('uid')!=meta['uid']
        or not result['metadata'].get('resourceVersion') or result['metadata']['resourceVersion']==meta['resourceVersion']
        or _cut_semantic(result)!=_cut_semantic(desired)
        or kind=='deployment' and result['metadata'].get('generation')!=meta['generation']+(before['spec'].get('replicas',1)!=0)):
        raise Blocked('cohort_cut_response_changed')
    _guard(session)
    if _read(path)!=intent:raise Blocked('cohort_effect_changed')
    save(path,{**intent,'status':'DONE','response':result})
    return result


def _runtime_base(session):
    return guard.ROOT/('rollout-'+session.operation)


def _state(session):
    load_plan(session)
    return _read(_base(session)/'checkpoint.json')


def _save_state(session,state):
    save(_base(session)/'checkpoint.json',state)


def _start_cut(actions,request,approved,execution,workspace):
    """Capture source and real active stage once, then take the physical cut."""
    import bridge_root
    import root_main
    import transaction
    from stage_runtime import HUB
    from staging_cohort_preservation_runtime import CohortStage
    if type(actions) is not bridge_root.Actions:raise Blocked('cohort_root_actions_required')
    session=actions.tt_session
    if (type(session) is not resource_claim.Session or commands.authority_guard!=session.check
        or session.check().get('phase')!='prepare' or session.check().get('target_sha256') is not None):
        raise Blocked('cohort_prepare_session_invalid')
    prepared=private.prepare_inputs(actions,request,approved,execution,workspace)
    stage=prepared['stage'];base=_base(session);private._private_directory(base.parent)
    # An existing operation is evidence, never a request to replay the writer cut.
    base.mkdir(mode=0o700);private._private_directory(base)
    runtime=_runtime_base(session);runtime.mkdir(mode=0o700);private._private_directory(runtime)
    hub_pods=stage.kube.run(['get','pods','-o','json'])['items']
    hub_pods=[p for p in hub_pods if any(v.get('persistentVolumeClaim',{}).get('claimName')==stage.expected['source_claim']
        for v in p.get('spec',{}).get('volumes',[]))]
    if len(hub_pods)!=1:raise Blocked('cohort_hub_identity_changed')
    precut={'schema':'voice-staging-cohort-precut-v1','operation':session.operation,
        'context':transaction.context(stage),'hub_pod':hub_pods[0]}
    _write(base/'precut.json',precut)
    parent=guard.ROOT/'installed'/'prerequisites'/session.operation
    parent_plan=_read(parent/'prepared.json')['plan']
    plan={'schema':SCHEMA,'operation':session.operation,'context':session.context,'helper_sha256':session.helper,
        'source_capture_sha256':parent_plan['source_capture_sha256'],'approved_sha256':digest(approved),
        'execution_sha256':digest(execution),'namespace_uid':stage.expected['namespace_uid'],
        'precut_capture_sha256':digest(precut),'actual_before':stage.old_images,
        'candidate':{'commit':session.context['commit'],'images':session.context['images']},
        'prerequisite_inputs_sha256':prepared['inputs_sha256'],'source_inventory_sha256':digest(approved['source_files'])}
    validate_plan(plan,session);_source(session,plan)
    contract=private._json(root_main.public_read(actions.code/'nats-known-baseline'/'deployed-contract.json'))
    try:enrollment=root_main.private_json(guard.ROOT/'bootstrap-enrollment.json')
    except FileNotFoundError:enrollment=None
    provenance=root_main.capture_inputs(stage.kube,runtime,contract,stage.expected['generation'],bootstrap_enrollment=enrollment)
    state={'schema':'voice-staging-cohort-preservation-checkpoint-v1','operation':session.operation,
        'plan':plan,'status':'PREPARED','context':transaction.context(stage),'events':[],
        'contract':contract,'provenance':provenance,'initial_resource_fence':None}
    _write(base/'checkpoint.json',state)
    binding=session.bind_cohort_preservation(plan,stage.old_images)
    state.update(initial_resource_fence=binding,status='CUTTING');_save_state(session,state)
    owned=None
    def journal(event):
        state['events'].append(copy.deepcopy(event))
        if owned is not None:state['context']=transaction.context(owned)
        _save_state(session,state)
    owned=CohortStage.from_captured(session,stage,journal);owned.runtime_base=runtime
    try:
        root_main.revalidate_inputs(owned.kube,contract,provenance)
        owned.fence()
        state.update(status='PAUSED',context=transaction.context(owned))
        _save_state(session,state)
        _write(base/'paused.json',{'schema':'voice-staging-cohort-paused-v1','operation':session.operation,
            'plan_sha256':digest(plan),'context':state['context'],'events':state['events']})
        revalidate_paused(session)
        return owned
    except BaseException:
        state.update(status='BLOCKED',context=transaction.context(owned),fence_status='UNKNOWN')
        _save_state(session,state);session.uncertain('cohort_preservation_cut_uncertain')
        raise


def _paused_stage(session):
    import transaction
    from stage_runtime import Kube
    plan=load_plan(session);state=_state(session);record=_read(_base(session)/'paused.json')
    if (record.get('schema')!='voice-staging-cohort-paused-v1' or record.get('operation')!=session.operation
        or record.get('plan_sha256')!=digest(plan) or record.get('context')!=state['context']
        or record.get('events')!=state['events'] or state.get('status') not in ('PAUSED','BACKUP','AWAITING_OFF_NODE','QUALIFIED')
        or record['context']['original_snapshots']!=_precut(session)['context']['original_snapshots']
        or record['context']['old_images']!=plan['actual_before']):raise Blocked('cohort_paused_custody_changed')
    return transaction.reconstruct(Kube(),{'operation':session.operation,'context':record['context'],'events':record['events']},lambda _:None)


def _phase_continuity(session,plan):
    row=session.check()
    if row.get('phase')=='cohort-preservation' and row.get('target_sha256')==digest(plan):return
    # After the sole typed advance the root parent plan carries this receipt.
    parent=_read(guard.ROOT/'installed'/'prerequisites'/session.operation/'checkpoint.json')
    receipt=_read(_base(session)/'qualified.json')
    if (parent['plan'].get('schema')!='voice-staging-prerequisites-plan-v2'
        or parent['plan'].get('preservation_sha256')!=digest(receipt)
        or parent['plan'].get('operation')!=session.operation or parent['plan'].get('context')!=session.context
        or parent['plan'].get('helper_sha256')!=session.helper):raise Blocked('cohort_successor_binding_changed')
    if row.get('phase')=='prerequisites' and row['target_sha256']==digest(parent['plan']):return
    captured=_read(guard.ROOT/'installed'/'prerequisites'/session.operation/'capture.json')
    if captured.get('plan_sha256')!=digest(parent['plan']):raise Blocked('cohort_successor_binding_changed')
    if row.get('phase')=='prerequisites-captured' and row['target_sha256']==digest(captured):return
    raise Blocked('cohort_successor_binding_changed')


def revalidate_paused(session):
    """Reobserve owned zero replicas and Pods, never infer a cut from an image map."""
    import root_main
    stage=_paused_stage(session);plan=load_plan(session);_phase_continuity(session,plan)
    namespace=stage.kube.get('namespace',NS)['metadata']
    if namespace.get('uid')!=plan['namespace_uid'] or namespace.get('deletionTimestamp'):
        raise Blocked('cohort_namespace_changed')
    stage.owned_marker()
    for name,expected in stage.snapshots.items():
        current=stage.kube.get('deployment',name)
        if (current['metadata'].get('deletionTimestamp') or current['metadata']['uid']!=expected['metadata']['uid']
            or current['metadata'].get('generation')!=expected['metadata'].get('generation')
            or current['spec']!=expected['spec'] or current['spec'].get('replicas',1)!=0):
            raise Blocked('cohort_paused_workload_changed')
    stage.no_pods(tuple(stage.snapshots));stage.verify_final_storage()
    state=_state(session);root_main.revalidate_inputs(stage.kube,state['contract'],state['provenance'])
    session.check()
    return copy.deepcopy(state['context'])


def capture_stage(session,receipt):
    if load_qualified(session)!=receipt:raise Blocked('cohort_receipt_changed')
    revalidate_paused(session)
    return _paused_stage(session)
