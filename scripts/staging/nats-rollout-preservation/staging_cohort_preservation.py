"""Source-owned STAGING preservation before prerequisite effects.

Only the existing root bridge can enter this producer. Private records are not
caller authorization, and a pending write is never reconciled by observation.
"""
import copy
import ast
import builtins
import hashlib
import json
import os
import re
import secrets
import weakref
import sys
import types
import io
import tarfile
import selectors
import subprocess
import signal
import time
import shutil
import datetime as dt
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
    'actual_before','candidate','prerequisite_inputs_sha256','source_inventory_sha256','stores_sha256','preflight_sha256','space_tool_sha256'}
_terminal_reads=weakref.WeakKeyDictionary()
_active_auth=weakref.WeakKeyDictionary()
_shared_cache={}


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
    for filename in ('staging_cohort_preservation.py','staging_cohort_preservation_runtime.py',
        'staging_cohort_preservation_pg.py','staging_cohort_preservation_auxiliary.py','staging_cohort_preservation_auth.py',
        'staging_cohort_preservation_space.py','staging_space_native.py'):
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


def _send(session,effect_id,argv,body,*,limit=4<<20,timeout=60,pass_fds=()):
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
        if len(body)>2<<20:
            if pass_fds:raise Blocked('cohort_restore_binary_copy_invalid')
            from staging_cohort_preservation_runtime import capture_migrate_binary
            result=capture_migrate_binary(session,argv,body,timeout=timeout)
        else:
            result=commands.capture(argv,body=body,limit=limit,timeout=timeout,
                env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'},pass_fds=pass_fds)
        if session._cohort_permit is not None:raise Blocked('cohort_permit_not_consumed')
        session.check()
        return result
    finally:session._cohort_permit=None


def _observation(session,label,argv,body=b'',*,limit=4<<20,timeout=30,pass_fds=()):
    """Private source-fixed observations; callers never supply this transport."""
    _guard(session);_no_pending(session);ident='observe-'+label+'-'+secrets.token_hex(8)
    directory=_base(session)/'effects';private._private_directory(directory)
    intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':ident,
        'binding':session.binding(),'argv':argv,'body_sha256':hashlib.sha256(body).hexdigest(),'status':'PENDING'}
    path=directory/(ident+'.json');_write(path,intent)
    raw=_send(session,ident,argv,body,limit=limit,timeout=timeout,pass_fds=pass_fds)
    if _read(path)!=intent:raise Blocked('cohort_effect_changed')
    save(path,{**intent,'status':'DONE','response_sha256':hashlib.sha256(raw).hexdigest()})
    return raw


def _no_pending(session):
    auth=_base(session)/'auth'
    if auth.exists():
        for path in auth.iterdir():
            value=_read(path)
            if value.get('status')!='COMPLETE' and _active_auth.get(session)!=value.get('nonce'):
                raise Blocked('cohort_auth_reconciliation_required')
    folder=_base(session)/'effects'
    if not folder.exists():return
    paths=list(folder.iterdir())
    if len(paths)>4096:raise Blocked('cohort_effect_inventory_invalid')
    for path in paths:
        value=_read(path)
        allowed=_terminal_reads.get(session)
        if path.suffix!='.json' or value.get('status')!='DONE' and (allowed is None or value!=allowed):
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
    # The existing bridge's two-command workflow sees the same protected early
    # operation. This is not a fabricated final rollout target.
    save(_runtime_base(session)/'checkpoint.json',state)


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
    parent_record=_read(parent/'prepared.json')
    if digest(parent_record)!=prepared['inputs_sha256']:raise Blocked('cohort_prepared_changed')
    parent_plan=parent_record['plan']
    stores=_capture_stores(session,stage,request,approved,workspace,inputs_sha256=prepared['inputs_sha256'])
    _write(base/'stores.json',stores)
    contract=private._json(root_main.public_read(actions.code/'nats-known-baseline'/'deployed-contract.json'))
    preflight=bridge_root.capture_cohort_authorities(actions,request,approved,workspace,stage,contract)
    _preflight_shape(preflight,session)
    _write(base/'preflight.json',preflight);contract=preflight['contract']
    plan={'schema':SCHEMA,'operation':session.operation,'context':session.context,'helper_sha256':session.helper,
        'source_capture_sha256':parent_plan['source_capture_sha256'],'approved_sha256':digest(approved),
        'execution_sha256':digest(execution),'namespace_uid':stage.expected['namespace_uid'],
        'precut_capture_sha256':digest(precut),'actual_before':stage.old_images,
        'candidate':{'commit':session.context['commit'],'images':session.context['images']},
        'prerequisite_inputs_sha256':prepared['inputs_sha256'],'source_inventory_sha256':digest(approved['source_files']),
        'stores_sha256':digest(stores),'preflight_sha256':digest(preflight),'space_tool_sha256':digest(None)}
    validate_plan(plan,session);_source(session,plan)
    try:enrollment=root_main.private_json(guard.ROOT/'bootstrap-enrollment.json')
    except FileNotFoundError:enrollment=None
    provenance=root_main.capture_inputs(stage.kube,runtime,contract,stage.expected['generation'],bootstrap_enrollment=enrollment)
    kernel=_captured_native_kernel(actions.code)
    private._private_write(runtime/'kernel',kernel)
    os.chmod(runtime/'kernel',0o550);os.chown(runtime/'kernel',0,65532)
    state={'schema':'voice-staging-cohort-preservation-checkpoint-v1','operation':session.operation,
        'plan':plan,'status':'PREPARED','context':transaction.context(stage),'events':[],
        'contract':contract,'provenance':provenance,'initial_resource_fence':None}
    _write(base/'checkpoint.json',state)
    # Authenticate the future published tool before the first writer-cut effect.
    # This read-only source/API phase does not require publication during coding.
    import staging_cohort_preservation_space as space
    tool=space.capture_publication(session,request);_write(base/'space-tool.json',tool)
    plan['space_tool_sha256']=digest(tool);state['plan']=plan;save(base/'checkpoint.json',state)
    binding=session.bind_cohort_preservation(plan,stage.old_images)
    state.update(initial_resource_fence=binding,status='CUTTING');_save_state(session,state)
    owned=None
    def journal(event):
        state['events'].append(copy.deepcopy(event))
        if owned is not None:state['context']=transaction.context(owned)
        _save_state(session,state)
    owned=CohortStage.from_captured(session,stage,journal);owned.runtime_base=runtime
    try:
        if tool['selected']:
            space.prepare_tool(session,request['token'])
            import staging_space_native as inspector
            inspected=inspector.inspect_source(session)
            if inspector.load_inspection(session)!=inspected:raise Blocked('cohort_space_inspection_changed')
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
    if row.get('phase')=='realtime-bootstrap':
        rt=_read(guard.ROOT/'installed'/'realtime-bootstrap'/session.operation/'checkpoint.json')
        if (rt['plan'].get('prerequisites_sha256')==digest(captured)
            and row['target_sha256']==digest(rt['plan'])):return
    if row.get('phase')=='prepare':
        target=private._target_checks.get(session)
        if target is None:
            target=_read(_runtime_base(session)/'checkpoint.json').get('target')
        if (isinstance(target,dict) and target.get('prerequisites_sha256')==digest(captured)
            and row['target_sha256']==digest(target)):return
    raise Blocked('cohort_successor_binding_changed')


def revalidate_paused(session):
    """Reobserve owned zero replicas and Pods, never infer a cut from an image map."""
    import root_main
    stage=_paused_stage(session);plan=load_plan(session);_phase_continuity(session,plan)
    namespace=stage.kube.get('namespace',NS)['metadata']
    if namespace.get('uid')!=plan['namespace_uid'] or namespace.get('deletionTimestamp'):
        raise Blocked('cohort_namespace_changed')
    stage.owned_marker()
    auxiliary=None
    if (_base(session)/'auxiliary-successor.json').exists():auxiliary=load_auxiliary_successor(session)
    for name,expected in stage.snapshots.items():
        resumed=auxiliary is not None and name=='voice-p3-principal-jwks'
        if resumed:expected=auxiliary['response']
        current=stage.kube.get('deployment',name)
        if (current['metadata'].get('deletionTimestamp') or current['metadata']['uid']!=expected['metadata']['uid']
            or current['metadata'].get('generation')!=expected['metadata'].get('generation')
            or current['spec']!=expected['spec'] or current['spec'].get('replicas',1)!=(1 if resumed else 0)):
            raise Blocked('cohort_paused_workload_changed')
    stage.no_pods(tuple(name for name in stage.snapshots if not(auxiliary is not None and name=='voice-p3-principal-jwks')))
    stage.verify_final_storage()
    _recheck_stores(session,stage,plan)
    state=_state(session);root_main.revalidate_inputs(stage.kube,state['contract'],state['provenance'])
    session.check()
    return copy.deepcopy(state['context'])


def _store_policy(session,approved,inputs_sha256):
    """Render the candidate's already prepared policy without committing it."""
    import staging_release_policy_prepare as policy_child
    session.check()
    path=guard.ROOT/'installed'/'prerequisites'/session.operation/'prepared.json'
    record=_read(path)
    if (set(record)!={'schema','plan','request','gis_authority_sha256'}
        or record['schema']!='voice-staging-prerequisites-prepared-v1' or digest(record)!=inputs_sha256):
        raise Blocked('cohort_prepared_changed')
    plan=private.validate_plan(record['plan'],session)
    if plan['approved_sha256']!=digest(approved):raise Blocked('cohort_store_source_changed')
    old=private._read_private(guard.ROOT/'installed'/'policy.json')
    if hashlib.sha256(old).hexdigest()!=plan['policy_before_sha256']:raise Blocked('cohort_store_policy_changed')
    result=policy_child.policy_successor(private._json(old),plan['children']['release_policy']['descriptor'],approved['source_sha'])
    if _read(path)!=record or private._read_private(guard.ROOT/'installed'/'policy.json')!=old:
        raise Blocked('cohort_store_policy_changed')
    session.check();return result


def _capture_stores(session,stage,request,approved,workspace,*,inputs_sha256):
    import compiler
    import source_plan
    import source_authority
    policy=_store_policy(session,approved,inputs_sha256)
    parameters={**policy,'registry':source_authority.REGISTRY,'tag':approved['source_sha'],'mode':request['mode'],
        'changed_services':request['changed_services'],'images':{},'enrolled':list(stage.snapshots),
        'generation':stage.marker['data']['generation'],'dataPVC':stage.marker['data']['dataPVC']}
    projection=source_plan.preservation_projection(workspace,parameters,compiler.decode_yaml,stage.kube)
    if any(row['sha256']!=approved['source_files'].get(row['path']) for row in projection['sources']):
        raise Blocked('cohort_store_source_changed')
    claims={}
    for identity in projection['identities']:
        if identity['kind']!='PersistentVolumeClaim':continue
        claim=stage.kube.get('pvc',identity['name']);meta=claim['metadata'];spec=claim['spec']
        pv=stage.kube.get('pv',spec['volumeName']);ref=pv['spec'].get('claimRef',{})
        if (meta.get('deletionTimestamp') or pv['metadata'].get('deletionTimestamp')
            or meta.get('uid')!=identity['uid'] or claim.get('status',{}).get('phase')!='Bound'
            or ref.get('uid')!=meta['uid'] or ref.get('name')!=meta['name'] or ref.get('namespace')!=NS):
            raise Blocked('cohort_store_storage_changed')
        claims[identity['name']]={'claim':_cut_semantic(claim),'claim_uid':meta['uid'],
            'pv':_cut_semantic(pv),'pv_uid':pv['metadata']['uid']}
    app=stage.kube.get('secret','voice-app-secrets')
    session.check()
    return {'schema':'voice-staging-cohort-unchanged-stores-v1','operation':session.operation,
        'source_sha':approved['source_sha'],'parameters':parameters,'projection':projection,'claims':claims,'app_before':app}


def _app_successor(session,before,current):
    """Only the root-proven additive GIS/Realtime writes may change shared RV."""
    if current['metadata'].get('uid')!=before['metadata']['uid'] or current['metadata'].get('deletionTimestamp'):
        raise Blocked('cohort_store_secret_changed')
    if current['metadata']['resourceVersion']==before['metadata']['resourceVersion'] and _cut_semantic(current)==_cut_semantic(before):return
    state=private._checkpoint(session)
    step=state['steps'].get('gis_material',{}).get('gis-app-credentials')
    if step is None or step.get('status')!='DONE':raise Blocked('cohort_store_secret_successor_required')
    payload=private._json(private.load_payload(session,'gis_material','gis-app-credentials'))
    if payload['before']!=before:raise Blocked('cohort_store_secret_changed')
    key=hashlib.sha256(b'gis_material/gis-app-credentials').hexdigest()
    base=guard.ROOT/'installed'/'prerequisites'/session.operation
    intent=_read(base/'attempts'/(key+'.json'));response=_read(base/'attempts'/(key+'.response.json'))
    effect=private._selected_effect(state,'gis_material','gis-app-credentials')
    desired=_cut_semantic(before);desired['data']=payload['after_data']
    added={'GAME_INTEGRATION_DB_PASSWORD','GAME_INTEGRATION_DATABASE_URL'}
    if (set(payload['after_data'])!=set(before['data'])|added or set(before['data'])&added
        or any(payload['after_data'][k]!=v for k,v in before['data'].items())
        or intent.get('plan_sha256')!=digest(state['plan']) or intent.get('operation')!=session.operation
        or intent.get('payload_sha256')!=effect['payload_sha256'] or response.get('intent_sha256')!=digest(intent)
        or response.get('observation')!=step['observation'] or _cut_semantic(response['object'])!=desired
        or response['object']['metadata']['uid']!=before['metadata']['uid']):raise Blocked('cohort_store_secret_changed')
    if (_cut_semantic(current)==desired and current['metadata']['resourceVersion']==step['observation']['resourceVersion']):return
    if not private._realtime_app_extension(session,state):raise Blocked('cohort_store_secret_changed')


def _recheck_stores(session,stage,plan):
    import compiler
    import source_plan
    record=_read(_base(session)/'stores.json')
    if digest(record)!=plan['stores_sha256']:raise Blocked('cohort_store_custody_changed')
    approved,_,_=_source(session,plan)
    before=record['app_before'];current=stage.kube.get('secret','voice-app-secrets');_app_successor(session,before,current)
    class PausedReads:
        def get(self,kind,name):
            value=stage.kube.get(kind,name)
            if kind.lower()=='secret' and name=='voice-app-secrets':
                _app_successor(session,before,value);return copy.deepcopy(before)
            if kind.lower()=='deployment' and name in stage.original_snapshots:
                expected=stage.snapshots[name]
                if (value['metadata'].get('uid')!=expected['metadata']['uid']
                    or value['metadata'].get('generation')!=expected['metadata'].get('generation')
                    or _cut_semantic(value)!=_cut_semantic(expected)):raise Blocked('cohort_paused_workload_changed')
                return copy.deepcopy(stage.original_snapshots[name])
            return value
        def run(self,*args,**kwargs):return stage.kube.run(*args,**kwargs)
    projected=source_plan.preservation_projection(guard.ROOT/'installed'/'sources'/session.operation,
        record['parameters'],compiler.decode_yaml,PausedReads())
    if projected!=record['projection'] or any(row['sha256']!=approved['source_files'].get(row['path']) for row in projected['sources']):
        raise Blocked('cohort_store_projection_changed')
    for name,wanted in record['claims'].items():
        claim=stage.kube.get('pvc',name);pv=stage.kube.get('pv',claim['spec']['volumeName'])
        if (claim['metadata'].get('uid')!=wanted['claim_uid'] or pv['metadata'].get('uid')!=wanted['pv_uid']
            or _cut_semantic(claim)!=wanted['claim'] or _cut_semantic(pv)!=wanted['pv']
            or claim.get('status',{}).get('phase')!='Bound'):raise Blocked('cohort_store_storage_changed')


def capture_stage(session,receipt):
    if load_qualified(session)!=receipt:raise Blocked('cohort_receipt_changed')
    revalidate_paused(session)
    stage=_paused_stage(session);auxiliary=load_auxiliary_successor(session)
    if auxiliary is not None:stage.snapshots['voice-p3-principal-jwks']=copy.deepcopy(auxiliary['response'])
    return stage


def load_prepared(session):
    """Fixed authorize input loader; never recapture or accept caller source."""
    plan=load_plan(session);_phase_continuity(session,plan);_no_pending(session)
    approved,execution,prepared=_source(session,plan)
    if (set(prepared)!={'schema','plan','request','gis_authority_sha256'}
        or prepared['schema']!='voice-staging-prerequisites-prepared-v1'
        or not isinstance(prepared['gis_authority_sha256'],str)
        or not re.fullmatch('[a-f0-9]{64}',prepared['gis_authority_sha256'])
        or set(prepared['request'])!={'mode','changed_services','gis_first_install'}
        or prepared['request']['mode'] not in ('full','app-only')
        or type(prepared['request']['gis_first_install']) is not bool
        or not isinstance(prepared['request']['changed_services'],list)
        or any(not isinstance(name,str) or not name for name in prepared['request']['changed_services'])):
        raise Blocked('cohort_prepared_inputs_changed')
    return {'request':copy.deepcopy(prepared['request']),'approved':copy.deepcopy(approved),
        'execution':copy.deepcopy(execution),'workspace':str(guard.ROOT/'installed'/'sources'/session.operation)}


def _preflight_shape(value,session):
    if (type(value) is not dict or set(value)!={'schema','operation','services','actor','nats','enrollment','contract'}
        or value['schema']!='voice-staging-cohort-preflight-v1' or value['operation']!=session.operation
        or not isinstance(value['services'],list) or len(set(value['services']))!=len(value['services'])
        or any(not isinstance(name,str) or not re.fullmatch('[a-z0-9-]+',name) for name in value['services'])
        or any(not isinstance(value[key],dict) for key in ('nats','enrollment','contract'))
        or value['actor'] is not None and not isinstance(value['actor'],dict)):
        raise Blocked('cohort_preflight_changed')


def load_preflight(session):
    plan=load_plan(session);_phase_continuity(session,plan)
    value=_read(_base(session)/'preflight.json');_preflight_shape(value,session)
    if digest(value)!=plan['preflight_sha256']:raise Blocked('cohort_preflight_changed')
    return copy.deepcopy(value)


def auth_runtime(session,base,kind):
    from staging_cohort_preservation_auth import ScratchRuntime
    return ScratchRuntime(session,base,kind)


def capture_final_stage(session):
    """Add only the completed cold-created P3 publisher to the paused view.

    Existing workloads retain the original pre-cut snapshots. Existing P3 uses
    the separate response-bound replica successor; no active cohort recapture
    or extra workload can enter through this loader.
    """
    import staging_p3_prerequisites as p3
    receipt=load_qualified(session);stage=capture_stage(session,receipt)
    state=private._completed(session);plan=state['plan'];name='voice-p3-principal-jwks';ident='Deployment/'+name
    if (plan.get('schema')!='voice-staging-prerequisites-plan-v2'
        or plan.get('preservation_sha256')!=digest(receipt)):
        raise Blocked('cohort_final_capture_binding_changed')
    images=plan['auxiliary_images'];key=name+'/jwks'
    if set(images)!={key}:raise Blocked('cohort_final_capture_images_changed')
    ready=p3.verify_complete(session)
    current=stage.kube.get('deployment',name);meta=current['metadata'];spec=current['spec']
    proof=ready['proof']['deployment'];identity=state['receipt']['publisher_identity']
    if (ready['receipt'].get('operation')!=session.operation
        or ready['receipt'].get('plan_sha256')!=digest(plan['children']['p3'])
        or ready['receipt'].get('publisher_ready_sha256')!=digest(ready['proof'])
        or meta.get('uid')!=identity['uid'] or meta.get('uid')!=proof['uid']
        or meta.get('generation')!=proof['generation'] or meta.get('deletionTimestamp')
        or digest(p3.private_semantic(current))!=proof['semantic_sha256'] or spec.get('replicas',1)!=1
        or spec['template']['spec'].get('initContainers')
        or [(row.get('name'),row.get('image')) for row in spec['template']['spec'].get('containers',[])]!=[('jwks',images[key])]):
        raise Blocked('cohort_final_publisher_changed')
    if name in stage.snapshots:
        wanted=stage.snapshots[name]
        if (meta['uid']!=wanted['metadata']['uid'] or meta['generation']!=wanted['metadata']['generation']
            or _cut_semantic(current)!=_cut_semantic(wanted)):
            raise Blocked('cohort_final_publisher_changed')
    else:
        step=state['steps'].get('p3',{}).get(ident)
        if (ident in plan['children']['p3']['preserved'] or step is None or step.get('status')!='DONE'
            or step.get('observation')!=identity or name in stage.original_snapshots
            or name in stage.expected['deployment_uids'] or key in stage.old_images):
            raise Blocked('cohort_final_publisher_unowned')
        if private.load_done(session,'p3').get(ident)!=identity:
            raise Blocked('cohort_final_publisher_unowned')
        attempt=hashlib.sha256(('p3/'+ident).encode()).hexdigest()
        response=_read(guard.ROOT/'installed'/'prerequisites'/session.operation/'attempts'/(attempt+'.response.json'))
        original=response['object'];om=original['metadata']
        if (response.get('observation')!=identity or om.get('uid')!=meta['uid']
            or om.get('generation')!=meta['generation'] or om.get('resourceVersion')!=identity['resourceVersion']
            or digest(p3.private_semantic(original))!=identity['semantic_sha256']
            or original.get('spec')!=current.get('spec')):
            raise Blocked('cohort_final_publisher_unowned')
        # Readiness/status and controller revision are independently checked by
        # verify_complete. Pin the acknowledged create, never a later status RV.
        stage.snapshots[name]=copy.deepcopy(original)
        stage.original_snapshots[name]=copy.deepcopy(original)
        stage.expected['deployment_uids'][name]=meta['uid']
        stage.old_images[key]=images[key]
    if stage.old_images!=state['receipt']['actual_after']:
        raise Blocked('cohort_final_capture_images_changed')
    if private._checkpoint(session)!=state:raise Blocked('cohort_final_capture_binding_changed')
    revalidate_paused(session);session.check()
    return stage


def load_auxiliary_successor(session):
    import staging_cohort_preservation_auxiliary as auxiliary
    plan=load_plan(session);_phase_continuity(session,plan)
    value=auxiliary.load(session)
    receipt=_read(_base(session)/'qualified.json')
    if receipt.get('auxiliary_successor_sha256')!=(None if value is None else digest(value)):
        raise Blocked('cohort_auxiliary_receipt_changed')
    return value


def shared_module(session,name):
    """Execute only the captured shared mechanics, with their local import closure."""
    roots={'story_media_backup_cohort','story_media_backup_cohort_restore','story_media_backup_cohort_finalize',
        'story_media_backup_restore','cohort_unmanaged_actual','auth_user_adoption_runtime','space_native_tool_identity_staging'}
    if name not in roots:raise Blocked('cohort_shared_module_forbidden')
    plan=load_plan(session);approved,_,_=_source(session,plan)
    source=guard.ROOT/'installed'/'sources'/session.operation/'scripts'/'dev';closure={}
    import root_main
    def collect(module):
        if module in closure:return
        if len(closure)>=256:raise Blocked('cohort_source_closure_limit')
        relative='scripts/dev/'+module+'.py';path=source/(module+'.py')
        raw=root_main.public_read(path)
        if hashlib.sha256(raw).hexdigest()!=approved['source_files'].get(relative):raise Blocked('cohort_shared_source_changed')
        tree=ast.parse(raw,filename=str(path));closure[module]=(path,raw)
        for node in ast.walk(tree):
            imports=([item.name.split('.')[0] for item in node.names] if isinstance(node,ast.Import)
                else [node.module.split('.')[0]] if isinstance(node,ast.ImportFrom) and node.module and not node.level else [])
            for dependency in imports:
                if (source/(dependency+'.py')).exists():collect(dependency)
    collect(name)
    key=(session.operation,plan['source_inventory_sha256']);cache=_shared_cache.setdefault(key,{})
    def load(module):
        path,raw=closure[module]
        if module in cache:
            if cache[module][0]!=hashlib.sha256(raw).hexdigest():raise Blocked('cohort_shared_source_changed')
            return cache[module][1]
        if module in sys.modules:raise Blocked('cohort_foreign_module_loaded')
        value=types.ModuleType(module);value.__file__=str(path);value.__package__=''
        cache[module]=(hashlib.sha256(raw).hexdigest(),value);sys.modules[module]=value
        def importer(target,globals=None,locals=None,fromlist=(),level=0):
            if level:raise Blocked('cohort_relative_module_forbidden')
            head=target.split('.')[0]
            if head in closure:
                if target!=head:raise Blocked('cohort_shared_module_forbidden')
                return load(head)
            return builtins.__import__(target,globals,locals,fromlist,level)
        value.__dict__['__builtins__']={**vars(builtins),'__import__':importer}
        try:exec(compile(raw,str(path),'exec'),value.__dict__)
        except BaseException:
            cache.pop(module,None);sys.modules.pop(module,None);raise
        return value
    result=load(name);session.check();return result


def _reference(session,value):
    return {'operation':session.operation,'record_sha256':digest(value)}


def _record(session,name,ref,schema):
    plan=load_plan(session)
    if type(ref) is not dict or set(ref)!={'operation','record_sha256'} or ref['operation']!=session.operation:
        raise Blocked('cohort_reference_invalid')
    value=_read(_base(session)/name)
    if (value.get('schema')!=schema or value.get('operation')!=session.operation
        or value.get('plan_sha256')!=digest(plan) or digest(value)!=ref['record_sha256']):
        raise Blocked('cohort_record_changed')
    return value


def load_catalog(session,ref):
    import staging_cohort_preservation_pg as pg
    value=_record(session,'catalog.json',ref,'voice-staging-cohort-catalog-v1')
    if pg.catalog(session)!=value:raise Blocked('cohort_catalog_changed')
    return copy.deepcopy(value)


def guard_postgres(session):
    from staging_cohort_preservation_pg import guard_identity
    return guard_identity(session)


def load_metadata(session,ref):
    value=_record(session,'metadata.json',ref,'voice-staging-cohort-metadata-v1')
    catalog=_read(_base(session)/'catalog.json')
    if value.get('catalog_sha256')!=digest(catalog):raise Blocked('cohort_metadata_changed')
    load_catalog(session,_reference(session,catalog))
    return copy.deepcopy(value)


def validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref):
    """STAGING authority, deliberately separate from the DEV phase validators."""
    _guard(session);catalog=load_catalog(session,catalog_ref);metadata=load_metadata(session,metadata_ref)
    expected=_read(_base(session)/'backup-binding.json')
    plain={k:v for k,v in binding.items() if k!='dump_receipts'}
    if (expected!=plain or databases!=sorted(catalog['databases']) or binding.get('databases')!=databases
        or binding.get('operation_id')!=session.context['operation_id'] or binding.get('source_sha')!=session.context['commit']
        or any(binding.get(k)!=session.context[k] for k in ('project','resource','actor','claim','window','generation','revision'))
        or binding.get('full_catalog')!={'receipt_sha256':digest(catalog),'properties':catalog['properties']}
        or binding.get('sources')!={'roles':{'engine':catalog['engine'],'server_start':catalog['server_start']},
            **{db:{'engine':catalog['engine'],'server_start':catalog['server_start'],'database_oid':oid}
                for db,oid in catalog['databases'].items()}}
        or any(binding.get('restore_inputs',{}).get(field)!=digest(metadata['source'][key]) for field,key in
            (('catalog_sha256','catalog'),('roles_catalog_sha256','roles'),('migration_catalog_sha256','migrations')))):
        raise Blocked('cohort_backup_binding_changed')
    if 'dump_receipts' in binding:
        receipts=dump_receipts(session)
        if binding['dump_receipts']!={member:digest(value) for member,value in receipts.items()}:
            raise Blocked('cohort_dump_receipts_changed')
    return frozenset(databases)|{'roles'}


def backup_custody(session):
    _guard(session)
    base=_base(session);private._private_directory(base/'archive')
    key=private._read_private(base/'archive.key',limit=32)
    if len(key)!=32:raise Blocked('cohort_backup_key_invalid')
    current=base/'restore-attempt.json'
    if current.exists():
        attempt=_read(current)
        if attempt.get('status')=='RUNNING' and attempt.get('phase')=='offnode':
            nonce=restore_attempt(session)
            readback=base/'restore'/nonce/'archive'
            proof=_read(base/'restore'/nonce/'capsule.json')
            if proof.get('members')!=capsule_members(session):raise Blocked('cohort_capsule_changed')
            return readback,key
    return base/'archive',key


def produce_dump(session,member,stream):
    import staging_cohort_preservation_pg as pg
    record=_read(_base(session)/'archive-record.json')
    validate_backup_binding(session,record['binding'],record['databases'],record['catalog_ref'],record['metadata_ref'])
    return pg.stream_dump(session,member,stream,record)


def dump_receipts(session):
    _guard(session);catalog=_read(_base(session)/'catalog.json');result={}
    for member in (*sorted(catalog['databases']),'roles'):
        row=_read(_base(session)/'effects'/('dump-'+member+'.json'))
        if (row.get('status')!='DONE' or row.get('operation')!=session.operation
            or row.get('catalog_sha256')!=digest(catalog) or row.get('binding')!=session.binding()
            or row.get('receipt',{}).get('member')!=member or row['receipt'].get('exit')!=0):
            raise Blocked('cohort_dump_incomplete')
        result[member]=row['receipt']
    return result


def _begin_restore(session,phase):
    if phase not in ('tooling','local','offnode'):raise Blocked('cohort_restore_phase_invalid')
    _guard(session);base=_base(session);private._private_directory(base/'restore')
    current=base/'restore-attempt.json'
    if current.exists():
        previous=_read(current)
        if previous.get('status')!='COMPLETE' or previous.get('phase')==phase:raise Blocked('cohort_restore_reconciliation_required')
    nonce=secrets.token_hex(16);private._private_directory(base/'restore'/nonce)
    _write(base/'restore'/nonce/'resources.json',{'operation':session.operation,'attempt':nonce,'resources':{}})
    value={'schema':'voice-staging-cohort-restore-attempt-v1','operation':session.operation,'phase':phase,
        'nonce':nonce,'status':'RUNNING','plan_sha256':digest(load_plan(session))}
    save(current,value)
    private._private_directory(base/'restore-input')
    return nonce


def restore_attempt(session):
    _guard(session);value=_read(_base(session)/'restore-attempt.json')
    if (value.get('schema')!='voice-staging-cohort-restore-attempt-v1' or value.get('operation')!=session.operation
        or value.get('status')!='RUNNING' or value.get('plan_sha256')!=digest(load_plan(session))
        or not re.fullmatch('[a-f0-9]{32}',str(value.get('nonce')))):
        raise Blocked('cohort_restore_attempt_invalid')
    return value['nonce']


def restore_journal(session):
    from staging_cohort_preservation_runtime import RestoreJournal
    restore_attempt(session)
    return RestoreJournal(session)


def restore_command(session,argv,*,body=b'',timeout=120):
    from staging_cohort_preservation_runtime import restore_command as run
    return run(session,argv,body=body,timeout=timeout)


def native_space_rehearsal(session):
    from staging_cohort_preservation_space import native_space_rehearsal as run
    return run(session)


def load_native_space_rehearsal(session):
    from staging_cohort_preservation_space import load_native_space_rehearsal as load
    return load(session)


def load_native_authority(session):
    from staging_cohort_preservation_space import load_native_authority as load
    return load(session)


def load_native_authority_during_job(session):
    from staging_cohort_preservation_space import load_native_authority_during_job as load
    return load(session)


def _captured_native_kernel(code):
    """Read only the fixed pinned decoder with its existing 32 MiB bound."""
    import root_main
    raw=root_main.public_read(code/'nats-known-baseline'/'kernel',limit=32<<20)
    if hashlib.sha256(raw).hexdigest()!=private.BINARY_PINS['nats-known-baseline/kernel']:
        raise Blocked('cohort_native_kernel_changed')
    return raw


def _end_restore(session,proof):
    nonce=restore_attempt(session);base=_base(session)
    resources=_read(base/'restore'/nonce/'resources.json')
    native=base/'restore'/nonce/'native-space.json'
    attempt=_read(base/'restore-attempt.json')
    if attempt['phase'] in ('local','offnode'):
        metadata=_read(base/'metadata.json')['source']['migrations']
        space=metadata.get('space_db')
        if isinstance(space,dict) and space.get('from')!=space.get('to') and not native.exists():
            raise Blocked('cohort_space_rehearsal_required')
    if native.exists():
        result=load_native_space_rehearsal(session)
        if proof.get('native_space')!=result:raise Blocked('cohort_space_rehearsal_changed')
        proof={**proof,'space_native_rehearsal_sha256':digest(result)}
    if not resources['resources'] or any(row['status']!='REMOVED' for row in resources['resources'].values()):
        raise Blocked('cohort_restore_cleanup_incomplete')
    current=_read(base/'restore-attempt.json')
    result={**current,'status':'COMPLETE','proof':proof,'resources_sha256':digest(resources)}
    _write(base/'restore'/nonce/'receipt.json',result);save(base/'restore-attempt.json',result)
    return result


def _tools(session):
    """Qualify actual immutable clone images and extract the pinned tool bytes."""
    from staging_cohort_preservation_runtime import DOCKER
    constants=shared_module(session,'auth_user_adoption_runtime').QUALIFIED_TOOLS
    images={name:constants[name] for name in ('postgres','migrate')}
    _write(_base(session)/'restore-tools.json',{'images':images,'ids':{}})
    _begin_restore(session,'tooling')
    for image in images.values():restore_command(session,DOCKER+['image','inspect',image])
    tools=_read(_base(session)/'restore-tools.json');journal=restore_journal(session)
    name='voice-media-restore-'+secrets.token_hex(16);labels={'voice.test.owner':name};nonce=journal.begin('migrate',name,labels)
    try:
        cid=restore_command(session,DOCKER+['create','--name',name,'--network','none','--label','voice.test.owner='+name,
            '--entrypoint','/bin/true',tools['ids']['migrate']]).decode().strip()
        restore_command(session,DOCKER+['inspect',cid]);journal.created('migrate',nonce,cid,labels)
        raw=restore_command(session,DOCKER+['cp',cid+':/usr/local/bin/migrate','-'])
        with tarfile.open(fileobj=io.BytesIO(raw),mode='r:') as archive:
            rows=archive.getmembers()
            if len(rows)!=1 or rows[0].name!='migrate' or not rows[0].isfile() or not 0<rows[0].size<=64<<20:
                raise Blocked('cohort_restore_tool_invalid')
            tool=archive.extractfile(rows[0]).read((64<<20)+1)
            if len(tool)!=rows[0].size:raise Blocked('cohort_restore_tool_invalid')
        tools['migrate_binary_sha256']=hashlib.sha256(tool).hexdigest();save(_base(session)/'restore-tools.json',tools)
        journal.begin_remove('migrate',cid);restore_command(session,DOCKER+['rm','-f','-v',cid]);journal.removed('migrate',cid,True)
        _end_restore(session,{'images':tools['ids'],'migrate_binary_sha256':tools['migrate_binary_sha256']})
        return tools
    except BaseException:
        journal.uncertain('migrate',nonce);raise


def _capture_archive(session):
    import staging_cohort_preservation_pg as pg
    tools=_tools(session);catalog=pg.catalog(session);_write(_base(session)/'catalog.json',catalog)
    metadata=pg.metadata(session,catalog);_write(_base(session)/'metadata.json',metadata)
    databases=sorted(catalog['databases']);context=session.context
    binding={key:context[key] for key in ('project','resource','actor','claim','window','generation','revision')}
    binding.update(operation_id=context['operation_id'],snapshot_id='capture-'+secrets.token_hex(16),source_sha=context['commit'],
        databases=databases,sources={'roles':{'engine':catalog['engine'],'server_start':catalog['server_start']},
            **{db:{'engine':catalog['engine'],'server_start':catalog['server_start'],'database_oid':oid} for db,oid in catalog['databases'].items()}},
        full_catalog={'receipt_sha256':digest(catalog),'properties':catalog['properties']},
        restore_inputs={'catalog_sha256':digest(metadata['source']['catalog']),'roles_catalog_sha256':digest(metadata['source']['roles']),
            'migration_catalog_sha256':digest(metadata['source']['migrations']),'postgres_image':tools['images']['postgres'],
            'migrate_image':tools['images']['migrate'],'migrate_binary_sha256':tools['migrate_binary_sha256'],'postgres_major':16})
    _write(_base(session)/'backup-binding.json',binding)
    record={'binding':binding,'databases':databases,'catalog_ref':_reference(session,catalog),
        'metadata_ref':_reference(session,metadata),'manifest':None}
    _write(_base(session)/'archive-record.json',record);private._private_write(_base(session)/'archive.key',secrets.token_bytes(32))
    cohort=shared_module(session,'story_media_backup_cohort');finalize=shared_module(session,'story_media_backup_cohort_finalize')
    manifest=cohort.export_staging(session,binding,databases,record['catalog_ref'],record['metadata_ref'])
    binding,manifest=finalize.finalize_staging(session,binding,databases,manifest,dump_receipts(session),record['catalog_ref'],record['metadata_ref'])
    record.update(binding=binding,manifest=manifest);save(_base(session)/'archive-record.json',record)
    return record


def load_backup_archive(session):
    record=_read(_base(session)/'archive-record.json')
    if type(record) is not dict or set(record)!={'binding','databases','manifest','catalog_ref','metadata_ref'} or record['manifest'] is None:
        raise Blocked('cohort_archive_incomplete')
    validate_backup_binding(session,record['binding'],record['databases'],record['catalog_ref'],record['metadata_ref'])
    return copy.deepcopy(record)


def restore_context(session,catalog_ref,metadata_ref):
    record=load_backup_archive(session)
    if record['catalog_ref']!=catalog_ref or record['metadata_ref']!=metadata_ref:raise Blocked('cohort_restore_source_changed')
    metadata=load_metadata(session,metadata_ref);root,key=backup_custody(session);tools=_read(_base(session)/'restore-tools.json')
    restore_attempt(session)
    return {'root':root,'key':key,'source_root':guard.ROOT/'installed'/'sources'/session.operation,
        'postgres_image':tools['images']['postgres'],'migrate_image':tools['images']['migrate'],
        'journal':restore_journal(session),'source_metadata':metadata['source']}


def native_kernel(session,fd,arguments,raw,limit):
    import stat
    if (type(fd) is not int or not isinstance(raw,bytes) or len(raw)>2<<20
        or not (arguments==['--consumer-state'] and limit==2<<20
            or isinstance(arguments,list) and len(arguments)==3 and arguments[0]=='--metadata-checksum' and limit==16
                and all(isinstance(v,str) and re.fullmatch('[A-Za-z0-9_-]{1,255}',v) for v in arguments[1:]))):
        raise Blocked('cohort_native_kernel_invalid')
    _guard(session);before=os.fstat(fd)
    custody=_read(guard.ROOT/'installed'/'prerequisites'/session.operation/'code-custody.json')
    wanted=custody['manifest']['nats-known-baseline/kernel']
    if not stat.S_ISREG(before.st_mode) or before.st_uid!=0 or before.st_mode&0o022 or before.st_nlink!=1:
        raise Blocked('cohort_native_kernel_changed')
    sha=hashlib.sha256();offset=0
    while chunk:=os.pread(fd,1<<20,offset):sha.update(chunk);offset+=len(chunk)
    if sha.hexdigest()!=wanted or wanted!=private.BINARY_PINS['nats-known-baseline/kernel']:
        raise Blocked('cohort_native_kernel_changed')
    result=_observation(session,'kernel',['/proc/self/fd/'+str(fd),*arguments],raw,limit=limit,timeout=30,pass_fds=(fd,))
    after=os.fstat(fd)
    if any(getattr(before,key)!=getattr(after,key) for key in ('st_dev','st_ino','st_size','st_mtime_ns','st_ctime_ns')):
        raise Blocked('cohort_native_kernel_changed')
    return result


def restore_member(session,member,command):
    from staging_cohort_preservation_runtime import _command,DOCKER
    import encrypted_cut
    record=load_backup_archive(session)
    if member not in set(record['databases'])|{'roles'}:raise Blocked('cohort_restore_member_invalid')
    nonce=restore_attempt(session);cohort=shared_module(session,'story_media_backup_cohort')
    path=cohort.decrypt_staging(session,member)
    expected_path=_base(session)/'restore-input'/(member+'-'+nonce+'.plain')
    if Path(path)!=expected_path:raise Blocked('cohort_restore_plaintext_invalid')
    fd,identity=encrypted_cut.regular(path,private=True)
    child=None;selector=selectors.DefaultSelector()
    try:
        raw_sha=hashlib.sha256();count=0
        while chunk:=os.read(fd,1<<20):raw_sha.update(chunk);count+=len(chunk);session.check()
        dump=_read(_base(session)/'effects'/('dump-'+member+'.json'))
        if dump.get('status')!='DONE' or count!=dump.get('bytes') or raw_sha.hexdigest()!=dump.get('plaintext_sha256'):
            raise Blocked('cohort_restore_plaintext_changed')
        os.lseek(fd,0,os.SEEK_SET)
        argv,_,_,_,_=_command(session,DOCKER+['exec','-i',*command],b'')
        effect='restore-member-'+member+'-'+nonce;effect_path=_base(session)/'effects'/(effect+'.json')
        _no_pending(session)
        intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':effect,
            'binding':session.binding(),'argv':argv,'body_sha256':raw_sha.hexdigest(),'status':'PENDING',
            'member':member,'bytes':count,'restore_attempt':nonce}
        _write(effect_path,intent);_guard(session)
        if _read(effect_path)!=intent or getattr(session,'_cohort_permit',None) is not None:raise Blocked('cohort_effect_changed')
        session._cohort_permit={key:intent[key] for key in ('effect_id','argv','body_sha256')}
        commands.authority_guard(argv)
        if session._cohort_permit is not None:raise Blocked('cohort_permit_not_consumed')
        child=subprocess.Popen(argv,stdin=subprocess.PIPE,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,
            start_new_session=True,env={'PATH':'/usr/bin:/bin','HOME':'/nonexistent'})
        os.set_blocking(child.stdin.fileno(),False);selector.register(child.stdin,selectors.EVENT_WRITE)
        deadline=time.monotonic()+900;pending=b'';written=0
        while selector.get_map():
            if time.monotonic()>=deadline:raise Blocked('cohort_restore_timeout')
            session.check()
            for event,_ in selector.select(.1):
                if not pending:pending=os.read(fd,1<<20)
                if not pending:
                    selector.unregister(child.stdin);child.stdin.close();break
                size=os.write(event.fd,pending);written+=size;pending=pending[size:]
        if child.wait(timeout=max(.01,deadline-time.monotonic()))!=0 or written!=count:raise Blocked('cohort_restore_member_failed')
        after=os.fstat(fd)
        if any(getattr(identity,key)!=getattr(after,key) for key in ('st_dev','st_ino','st_size','st_mtime_ns','st_ctime_ns')):
            raise Blocked('cohort_restore_plaintext_changed')
        _guard(session)
        if _read(effect_path)!=intent:raise Blocked('cohort_effect_changed')
        save(effect_path,{**intent,'status':'DONE','exit':0})
    finally:
        session._cohort_permit=None;selector.close();os.close(fd)
        if child is not None:
            if child.poll() is None:os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=2)
            if not child.stdin.closed:child.stdin.close()


def _qualify_restore(session,phase):
    if phase=='local':_begin_restore(session,phase)
    elif _read(_base(session)/'restore-attempt.json').get('phase')!='offnode':raise Blocked('cohort_restore_phase_invalid')
    record=load_backup_archive(session);restore=shared_module(session,'story_media_backup_cohort_restore')
    proof=restore.qualify_staging(session,record['binding'],record['databases'],record['manifest'],record['catalog_ref'],record['metadata_ref'])
    if (proof.get('cleanup')!='verified' or proof.get('operation_id')!=session.context['operation_id']
        or proof.get('snapshot_id')!=record['binding']['snapshot_id'] or proof.get('source_sha')!=session.context['commit']
        or proof.get('manifest_sha256')!=digest(record['manifest'])):raise Blocked('cohort_restore_proof_invalid')
    receipt=_end_restore(session,proof)
    _write(_base(session)/(phase+'-restore.json'),receipt)
    return receipt


def _file_identity(path):
    import encrypted_cut
    fd,before=encrypted_cut.regular(path,private=True);sha=hashlib.sha256();count=0
    with os.fdopen(fd,'rb') as stream:
        while chunk:=stream.read(1<<20):sha.update(chunk);count+=len(chunk)
        after=os.fstat(stream.fileno())
    if count!=before.st_size or any(getattr(before,key)!=getattr(after,key) for key in ('st_dev','st_ino','st_size','st_mtime_ns','st_ctime_ns')):
        raise Blocked('cohort_archive_changed')
    return {'sha256':sha.hexdigest(),'bytes':count}


def _seal_capsule(session):
    import encrypted_cut
    record=load_backup_archive(session);base=_runtime_base(session);root,key=backup_custody(session)
    cohort=shared_module(session,'story_media_backup_cohort')
    cohort.verify_staging(session,record['binding'],record['databases'],record['manifest'],record['catalog_ref'],record['metadata_ref'])
    target=base/'cohort-before.tar'
    with target.open('xb') as output:
        target.chmod(0o600)
        with tarfile.open(fileobj=output,mode='w|',format=tarfile.USTAR_FORMAT) as bundle:
            for member in sorted(set(record['databases'])|{'roles'}):
                fd,row=encrypted_cut.regular(root/(member+'.enc'),private=True)
                with os.fdopen(fd,'rb') as stream:
                    entry=tarfile.TarInfo(member+'.enc');entry.size=row.st_size;entry.mode=0o600;bundle.addfile(entry,stream)
                session.check()
        output.flush();os.fsync(output.fileno())
    _write(base/'cohort-before-manifest.json',record);private._private_write(base/'cohort-before.key',key)
    members={name:_file_identity(base/name) for name in ('cohort-before.tar','cohort-before-manifest.json','cohort-before.key')}
    _write(_base(session)/'capsule.json',{'schema':'voice-staging-cohort-capsule-v1','operation':session.operation,
        'plan_sha256':digest(load_plan(session)),'archive_sha256':digest(record),'members':members})
    return members


def capsule_members(session):
    plan=load_plan(session);_phase_continuity(session,plan)
    value=_read(_base(session)/'capsule.json');record=_read(_base(session)/'archive-record.json')
    names={'cohort-before.tar','cohort-before-manifest.json','cohort-before.key'}
    if (set(value)!={'schema','operation','plan_sha256','archive_sha256','members'}
        or value['schema']!='voice-staging-cohort-capsule-v1' or value['operation']!=session.operation
        or value['plan_sha256']!=digest(plan) or value['archive_sha256']!=digest(record)
        or set(value['members'])!=names):raise Blocked('cohort_capsule_changed')
    current={name:_file_identity(_runtime_base(session)/name) for name in sorted(names)}
    if current!=value['members']:raise Blocked('cohort_capsule_changed')
    return copy.deepcopy(current)


def qualify_decrypted(session,recovered_directory):
    import encrypted_cut
    directory=Path(recovered_directory);base=_runtime_base(session)
    if (directory.resolve(strict=True)!=directory or directory.name!='cohort' or directory.parent.parent!=base
        or not directory.parent.name.startswith('decrypt-')):raise Blocked('cohort_decrypted_directory_invalid')
    encrypted_cut.directory(directory);wanted=capsule_members(session)
    if {p.name for p in directory.iterdir()}!=set(wanted):raise Blocked('cohort_capsule_changed')
    if {name:_file_identity(directory/name) for name in wanted}!=wanted:raise Blocked('cohort_capsule_changed')
    record=_read(directory/'cohort-before-manifest.json')
    if record!=load_backup_archive(session):raise Blocked('cohort_capsule_changed')
    nonce=_begin_restore(session,'offnode');root=_base(session)/'restore'/nonce
    private._private_directory(root/'archive')
    expected={member+'.enc' for member in {*record['databases'],'roles'}};seen=set();total=0
    with tarfile.open(directory/'cohort-before.tar',mode='r|') as bundle:
        for entry in bundle:
            if entry.name not in expected or entry.name in seen or not entry.isfile() or not 0<entry.size<=64<<30:
                raise Blocked('cohort_capsule_member_invalid')
            total+=entry.size
            if total>64<<30:raise Blocked('cohort_capsule_limit')
            seen.add(entry.name);target=root/'archive'/entry.name
            with target.open('xb') as out,bundle.extractfile(entry) as source:
                target.chmod(0o600);shutil.copyfileobj(source,out,1<<20);out.flush();os.fsync(out.fileno())
            session.check()
    if seen!=expected:raise Blocked('cohort_capsule_incomplete')
    _write(root/'capsule.json',{'members':wanted,'archive_sha256':digest(record)})
    receipt=_qualify_restore(session,'offnode')
    return {'schema':'voice-staging-cohort-offnode-restore-v1','operation':session.operation,
        'restore_sha256':digest(receipt),'manifest_sha256':digest(record['manifest']),
        'cipher_members_verified':True,'cleanup':'verified'}


def _closed_copy_authority(session,runtime,broker,tree):
    """Requalify the captured source authority against an owned closed copy."""
    import compiler
    import nats_root_plan
    from stage_runtime import Kube
    frozen=load_preflight(session);revalidate_paused(session)
    current=nats_root_plan.closed_copy_preflight(Kube(),
        guard.ROOT/'installed'/'sources'/session.operation,frozen['enrollment'],
        frozen['contract'],compiler.decode_yaml,runtime,broker,tree)
    keys={'plan','binding','scripts','live_census_sha256'}
    if (set(current)!=keys or set(frozen['nats'])!=keys
        or any(current[key]!=frozen['nats'][key] for key in keys-{'live_census_sha256'})):
        raise Blocked('cohort_closed_copy_authority_changed')
    if load_preflight(session)!=frozen:raise Blocked('cohort_preflight_changed')
    revalidate_paused(session)
    return current


def _capture_native(session):
    import preserve
    import root_main
    import transaction
    from staging_cohort_preservation_runtime import NativeRuntime
    stage=_paused_stage(session);state=_state(session);base=_runtime_base(session)
    revalidate_paused(session)
    cut=preserve.capture_cut(NativeRuntime(session),stage.final_path,
        lambda:revalidate_paused(session),
        validate=lambda runtime,broker,tree,row,manifest:_closed_copy_authority(session,runtime,broker,tree),
        live_catalog=transaction.require_original_live_census(stage,state['events']))
    save(base/'rollout-before-manifest.json',cut['manifest'])
    cut['manifest_sha256']=root_main.file_sha(base/'rollout-before-manifest.json')
    _write(_base(session)/'native-cut.json',cut)
    return cut


def _native_readback(session,archive):
    from preserve import closed_native_states,recovered_census,canonical,native_catalog,verify_catalog
    from rollout_census import semantic
    from expired_proof import observe_copy
    from native_store import verify_archive
    from docker_runtime import NATS_IMAGE
    from staging_cohort_preservation_runtime import NativeRuntime
    cut=_read(_base(session)/'native-cut.json');runtime=NativeRuntime(session)
    revalidate_paused(session)
    if not verify_archive(archive,cut['manifest']):raise Blocked('cohort_native_archive_changed')
    authority=lambda:revalidate_paused(session)
    states=closed_native_states(runtime,archive,cut['manifest'],cut['census'],authority)
    catalog=native_catalog(runtime,archive,cut['manifest'],authority)
    observed=observe_copy(runtime,archive,cut['manifest'],'decrypted-readback',authority,
        validate=lambda runtime,broker,tree,row,manifest,records:_closed_copy_authority(session,runtime,broker,tree),
        native_only=True)
    verify_catalog(catalog,observed['census'])
    compared,ephemeral=recovered_census(cut['census'],observed['census'],observed['recovery'],cut['manifest'])
    if canonical(cut['census'])!=cut['census_sha256'] or semantic(compared)!=semantic(cut['census']):
        raise Blocked('cohort_native_census_changed')
    authority()
    return {'native_archive_verified':True,'complete_durable_sha256':canonical(states),
        'census_sha256':cut['census_sha256'],'native_messages':observed['native_messages'],
        'ephemeral_recovery_observations':ephemeral,'server_image':NATS_IMAGE}


def _encrypt(session,execution,cut):
    import encrypted_cut
    import grp
    base=_runtime_base(session)
    _write(base/'copy-checkpoint.json',{'schema':'nats-rollout-copy-v1','operation':session.operation,
        'archive_sha256':cut['manifest']['archive_sha256'],'manifest_sha256':cut['manifest_sha256'],
        'census_sha256':cut['census_sha256']})
    _seal_capsule(session)
    revalidate_paused(session);session.check()
    cipher=encrypted_cut.encrypt_cut(base,guard.ROOT/'installed'/'recovery',cohort_session=session)
    # The member proof remains private. GitHub custody accepts its closed seven
    # public binding fields, never a Secret/key digest in the workflow result.
    members=cipher['cohort_members']
    if members!=capsule_members(session):raise Blocked('cohort_capsule_changed')
    binding={**cipher,'operation':session.operation,'challenge':secrets.token_hex(16),
        'run_id':execution['dispatcher_run_id'],'head_sha':execution['head_sha'],
        'created_at':dt.datetime.now(dt.timezone.utc).isoformat()}
    _write(_base(session)/'cipher-binding.json',binding)
    export=base/'export';export.mkdir(mode=0o750)
    gid=grp.getgrnam('pmd').gr_gid
    shutil.copyfile(base/'rollout-backup.cms',export/'rollout-backup.cms')
    os.chown(export,0,gid);os.chmod(export,0o750)
    os.chown(export/'rollout-backup.cms',0,gid);os.chmod(export/'rollout-backup.cms',0o440)
    os.chown(base,0,gid);os.chmod(base,0o750)
    session.check();revalidate_paused(session)
    return binding


def prepared_result(session):
    """Only upload instructions; never authorization to mutate prerequisites."""
    plan=load_plan(session);_phase_continuity(session,plan);_no_pending(session)
    state=_state(session);binding=_read(_base(session)/'cipher-binding.json')
    _,_,prepared=_source(session,plan)
    if state.get('status')!='AWAITING_OFF_NODE' or state.get('cipher_binding')!=binding:
        raise Blocked('cohort_upload_not_ready')
    if _file_identity(_runtime_base(session)/'rollout-backup.cms')!={
        'sha256':binding['cipher_sha256'],'bytes':binding['cipher_bytes']}:
        raise Blocked('cohort_cipher_changed')
    revalidate_paused(session)
    request=prepared['request']
    return {'operation':session.operation,'phase':'AWAITING_OFF_NODE','status':'WAITING',
        'fence_status':'VERIFIED','cipher_path':str(_runtime_base(session)/'export'/'rollout-backup.cms'),
        'artifact_name':'voice-nats-rollout-'+session.operation+'-'+binding['challenge'],
        'challenge':binding['challenge'],'changed_services':','.join(request['changed_services']),
        'deploy_mode':request['mode'],'source_sha':plan['context']['commit']}


def prepare(actions,request,approved,execution,workspace):
    """Existing bridge entry: stop writers, qualify actual copies, then upload."""
    session=actions.tt_session
    _start_cut(actions,request,approved,execution,workspace)
    try:
        state=_state(session);state.update(status='BACKUP',phase='COHORT_BACKUP',fence_status='VERIFIED')
        _save_state(session,state)
        _capture_archive(session)
        _qualify_restore(session,'local')
        cut=_capture_native(session)
        binding=_encrypt(session,execution,cut)
        state=_state(session);state.update(status='AWAITING_OFF_NODE',phase='AWAITING_OFF_NODE',
            cut=cut,cipher_binding=binding,execution_authority=execution)
        _save_state(session,state)
        return prepared_result(session)
    except BaseException:
        state=_state(session);state.update(status='BLOCKED',fence_status='UNKNOWN')
        _save_state(session,state);session.uncertain('cohort_preservation_backup_uncertain')
        raise


def _space_record_hashes(base,plan):
    tool=_read(base/'space-tool.json')
    if (set(tool)!={'schema','operation','source_capture_sha256','approved_sha256','selected','authenticated_publication'}
        or tool['schema']!='voice-staging-cohort-space-tool-v1' or tool['operation']!=plan['operation']
        or tool['source_capture_sha256']!=plan['source_capture_sha256'] or tool['approved_sha256']!=plan['approved_sha256']
        or digest(tool)!=plan['space_tool_sha256'] or type(tool['selected']) is not bool):
        raise Blocked('cohort_space_tool_changed')
    fields={'space_tool_available_sha256':'space-tool-available.json','space_inspection_sha256':'space-inspection.json'}
    if not tool['selected']:
        if tool['authenticated_publication'] is not None or any((base/name).exists() for name in fields.values()):
            raise Blocked('cohort_space_proof_changed')
        return {key:None for key in fields}
    return {key:digest(_read(base/name)) for key,name in fields.items()}


def _verify_space_proofs(session):
    plan=load_plan(session);expected=_space_record_hashes(_base(session),plan)
    if expected['space_inspection_sha256'] is not None:
        import staging_cohort_preservation_space as space
        import staging_space_native as inspector
        actual={'space_tool_available_sha256':digest(space.load_available_tool(session)),
            'space_inspection_sha256':digest(inspector.load_retained_inspection(session))}
        if actual!=expected:raise Blocked('cohort_space_proof_changed')
    return expected


def qualify_offnode(actions,request):
    """Fixed authorize continuation: verify remote bytes and restore before effects."""
    import bridge_root
    import github_custody
    import encrypted_cut
    if type(actions) is not bridge_root.Actions:raise Blocked('cohort_root_actions_required')
    session=actions.tt_session;plan=_guard(session);_no_pending(session)
    state=_state(session);base=_runtime_base(session)
    if request.get('operation')!=session.operation or request.get('action')!='authorize':
        raise Blocked('cohort_continuation_invalid')
    if state.get('status')=='QUALIFIED':return load_qualified(session)
    if state.get('status')!='AWAITING_OFF_NODE':raise Blocked('cohort_continuation_not_ready')
    binding=_read(_base(session)/'cipher-binding.json')
    if state.get('cipher_binding')!=binding:raise Blocked('cohort_cipher_changed')
    revalidate_paused(session)
    try:
        public_binding={key:binding[key] for key in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes','created_at')}
        custody=github_custody.verify_artifact(request['token'],public_binding,base/'rollout-backup.cms',request['artifact_id'])
        _write(_base(session)/'offnode-custody.json',custody)
        session.check();revalidate_paused(session)
        restored=encrypted_cut.decrypt_verify(base,guard.ROOT/'installed'/'recovery',binding,
            lambda archive:_native_readback(session,archive),cohort_session=session)
        _write(_base(session)/'decrypted-restore.json',restored)
        offnode=_read(_base(session)/'offnode-restore.json');local=_read(_base(session)/'local-restore.json')
        archive=load_backup_archive(session);_no_pending(session);revalidate_paused(session)
        _offnode_records(session)
        import staging_cohort_preservation_auxiliary as auxiliary
        successor=auxiliary.resume(session)
        receipt={'schema':'voice-staging-cohort-preservation-receipt-v1','operation':session.operation,
            'context':session.context,'helper_sha256':session.helper,'plan_sha256':digest(plan),
            'prerequisite_inputs_sha256':plan['prerequisite_inputs_sha256'],
            'actual_before':plan['actual_before'],'namespace_uid':plan['namespace_uid'],
            'paused_sha256':digest(_read(_base(session)/'paused.json')),'archive_sha256':digest(archive),
            'native_cut_sha256':digest(_read(_base(session)/'native-cut.json')),
            'local_restore_sha256':digest(local),'offnode_restore_sha256':digest(offnode),
            'custody_sha256':digest(custody),'decrypted_restore_sha256':digest(restored),
            'cipher_binding_sha256':digest(binding),'resource_fence':state['initial_resource_fence'],
            'auxiliary_successor_sha256':None if successor is None else digest(successor)}
        receipt.update(_verify_space_proofs(session))
        _write(_base(session)/'qualified.json',receipt)
        state.update(status='QUALIFIED',phase='COHORT_QUALIFIED');_save_state(session,state)
        return load_qualified(session)
    except BaseException:
        state.update(status='BLOCKED',fence_status='UNKNOWN');_save_state(session,state)
        session.uncertain('cohort_offnode_qualification_uncertain')
        raise


def load_qualified(session,*,actual=None,rollout_target=None):
    plan=load_plan(session);_phase_continuity(session,plan);_no_pending(session)
    state=_state(session);receipt=_read(_base(session)/'qualified.json')
    fixed={'schema':'voice-staging-cohort-preservation-receipt-v1','operation':session.operation,
        'context':session.context,'helper_sha256':session.helper,'plan_sha256':digest(plan),
        'prerequisite_inputs_sha256':plan['prerequisite_inputs_sha256'],
        'actual_before':plan['actual_before'],'namespace_uid':plan['namespace_uid'],
        'resource_fence':state['initial_resource_fence']}
    auxiliary=_base(session)/'auxiliary-successor.json'
    fixed['auxiliary_successor_sha256']=digest(_read(auxiliary)) if auxiliary.exists() else None
    fixed.update(_verify_space_proofs(session))
    for name,file in {'paused_sha256':'paused.json','archive_sha256':'archive-record.json',
        'native_cut_sha256':'native-cut.json','local_restore_sha256':'local-restore.json',
        'offnode_restore_sha256':'offnode-restore.json','custody_sha256':'offnode-custody.json',
        'decrypted_restore_sha256':'decrypted-restore.json','cipher_binding_sha256':'cipher-binding.json'}.items():
        fixed[name]=digest(_read(_base(session)/file))
    if receipt!=fixed or state.get('status')!='QUALIFIED':raise Blocked('cohort_qualification_changed')
    _offnode_records(session)
    if actual is not None and actual!=plan['actual_before']:raise Blocked('cohort_actual_changed')
    if rollout_target is not None:
        capture=_read(guard.ROOT/'installed'/'prerequisites'/session.operation/'capture.json')
        if (rollout_target.get('prerequisites_sha256')!=digest(capture)
            or rollout_target.get('cohort_preservation_sha256')!=digest(receipt)):
            raise Blocked('cohort_target_changed')
    revalidate_paused(session)
    return copy.deepcopy(receipt)


def load_rollout_continuation(session):
    """Transfer the already qualified cut into the fixed transaction caller."""
    import root_main
    from native_store import verify_archive
    receipt=load_qualified(session);state=_state(session);preflight=load_preflight(session)
    paused=_read(_base(session)/'paused.json');cut=_read(_base(session)/'native-cut.json')
    if (state.get('cut')!=cut or state.get('events')!=paused['events']
        or state.get('contract')!=preflight['contract']
        or root_main.file_sha(_runtime_base(session)/'rollout-before-manifest.json')!=cut['manifest_sha256']
        or not verify_archive(_runtime_base(session)/'rollout-before.tar',cut['manifest'])):
        raise Blocked('cohort_continuation_changed')
    result={'cut':cut,'custody':_offnode_records(session),'provenance':state['provenance'],
        'contract':preflight['contract'],'events':paused['events']}
    if load_qualified(session)!=receipt or _state(session)!=state:
        raise Blocked('cohort_continuation_changed')
    return copy.deepcopy(result)


def _offnode_records(session):
    """No auxiliary or prerequisite effect can substitute local backup for custody."""
    plan=load_plan(session);base=_base(session);binding=_read(base/'cipher-binding.json')
    custody=_read(base/'offnode-custody.json');restored=_read(base/'decrypted-restore.json')
    archive=_read(base/'archive-record.json');offnode=_read(base/'offnode-restore.json')
    local=_read(base/'local-restore.json')
    expected={'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',
        **{key:binding[key] for key in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')},
        'artifact_id':custody.get('artifact_id'),'verified':True}
    if (custody!=expected or type(custody['artifact_id']) is not int or custody['artifact_id']<=0
        or binding['operation']!=session.operation or binding['head_sha']!=session.context['commit']
        or restored.get('schema')!='voice-nats-decrypted-restore-v1'
        or restored.get('cipher_sha256')!=binding['cipher_sha256'] or restored.get('cipher_bytes')!=binding['cipher_bytes']
        or restored.get('decrypted_members_verified') is not True
        or restored.get('remote_equivalence')!='FULL_CIPHERTEXT_SHA256_AND_SIZE'
        or restored.get('cohort_readback')!={'schema':'voice-staging-cohort-offnode-restore-v1','operation':session.operation,
            'restore_sha256':digest(offnode),'manifest_sha256':digest(archive['manifest']),
            'cipher_members_verified':True,'cleanup':'verified'}):raise Blocked('cohort_offnode_receipt_changed')
    for phase,value in (('local',local),('offnode',offnode)):
        nonce=value.get('nonce')
        if (not isinstance(nonce,str) or not re.fullmatch('[a-f0-9]{32}',nonce) or value.get('phase')!=phase
            or value.get('status')!='COMPLETE' or value.get('plan_sha256')!=digest(plan)
            or value.get('proof',{}).get('cleanup')!='verified'
            or value.get('proof',{}).get('manifest_sha256')!=digest(archive['manifest'])
            or value!=_read(base/'restore'/nonce/'receipt.json')):raise Blocked('cohort_restore_receipt_changed')
        resources=_read(base/'restore'/nonce/'resources.json')
        if (value['resources_sha256']!=digest(resources) or not resources['resources']
            or any(row['status']!='REMOVED' for row in resources['resources'].values())):
            raise Blocked('cohort_restore_cleanup_changed')
    if _file_identity(_runtime_base(session)/'rollout-backup.cms')!={
        'sha256':binding['cipher_sha256'],'bytes':binding['cipher_bytes']}:raise Blocked('cohort_cipher_changed')
    return copy.deepcopy(custody)


def validate_historical_capture(operation):
    """Validate a completed root-owned predecessor as data, never execute it."""
    if not isinstance(operation,str) or not re.fullmatch('[a-f0-9]{12}',operation):
        raise Blocked('cohort_history_operation_invalid')
    root=guard.ROOT/'installed'/'cohort-preservation'/operation
    state=_read(root/'checkpoint.json');plan=state['plan'];receipt=_read(root/'qualified.json')
    rollout=_read(guard.ROOT/('rollout-'+operation)/'checkpoint.json',limit=128<<20)
    if (rollout.get('status')!='PASS' or rollout.get('phase')!='POST_RESTART'
        or rollout.get('fence_status')!='RELEASED' or rollout.get('operation')!=operation):
        raise Blocked('cohort_history_not_pass')
    binding=resource_claim.historical_binding(rollout)
    parent=guard.ROOT/'installed'/'prerequisites'/operation
    prepared=_read(parent/'prepared.json');successor=_read(parent/'checkpoint.json');captured=_read(parent/'capture.json')
    approved=_read(parent/'approved.json');execution=_read(parent/'execution.json')
    source=_read(guard.ROOT/'installed'/'source-authority'/(operation+'.json'))
    fixed={'schema':'voice-staging-cohort-preservation-receipt-v1','operation':operation,
        'context':binding['context'],'helper_sha256':binding['helper_sha256'],'plan_sha256':digest(plan),
        'prerequisite_inputs_sha256':digest(prepared),'actual_before':plan['actual_before'],
        'namespace_uid':plan['namespace_uid'],'resource_fence':state['initial_resource_fence']}
    for key,file in {'paused_sha256':'paused.json','archive_sha256':'archive-record.json','native_cut_sha256':'native-cut.json',
        'local_restore_sha256':'local-restore.json','offnode_restore_sha256':'offnode-restore.json',
        'custody_sha256':'offnode-custody.json','decrypted_restore_sha256':'decrypted-restore.json',
        'cipher_binding_sha256':'cipher-binding.json'}.items():fixed[key]=digest(_read(root/file))
    fixed['auxiliary_successor_sha256']=digest(_read(root/'auxiliary-successor.json')) if (root/'auxiliary-successor.json').exists() else None
    fixed.update(_space_record_hashes(root,plan))
    if (state.get('status')!='QUALIFIED' or set(plan)!=PLAN_FIELDS or plan['schema']!=SCHEMA or receipt!=fixed
        or plan['operation']!=operation or plan['context']!=binding['context'] or plan['helper_sha256']!=binding['helper_sha256']
        or state['initial_resource_fence']!={**binding,'target_sha256':digest(plan)}
        or plan['prerequisite_inputs_sha256']!=digest(prepared) or plan['approved_sha256']!=digest(approved)
        or plan['execution_sha256']!=digest(execution) or plan['source_capture_sha256']!=digest(source)
        or plan['source_inventory_sha256']!=digest(approved['source_files'])
        or source!={'schema':'voice-staging-source-capture-v1','operation':operation,'context':binding['context'],
            'helper_sha256':binding['helper_sha256'],'approved_sha256':digest(approved),'execution_sha256':digest(execution)}
        or successor['plan'].get('schema')!='voice-staging-prerequisites-plan-v2'
        or successor['plan'].get('preservation_sha256')!=digest(receipt)
        or captured.get('plan_sha256')!=digest(successor['plan']) or rollout.get('prerequisites')!=captured
        or rollout['target'].get('prerequisites_sha256')!=digest(captured)
        or rollout['target'].get('cohort_preservation_sha256')!=digest(receipt)
        or digest(_read(root/'precut.json'))!=plan['precut_capture_sha256']
        or digest(_read(root/'stores.json'))!=plan['stores_sha256']
        or digest(_read(root/'preflight.json'))!=plan['preflight_sha256']
        or digest(_read(root/'space-tool.json'))!=plan['space_tool_sha256']):raise Blocked('cohort_history_changed')
    for phase in ('local','offnode'):
        restored=_read(root/(phase+'-restore.json'))
        nonce=restored.get('nonce')
        if (not isinstance(nonce,str) or not re.fullmatch('[a-f0-9]{32}',nonce)
            or restored.get('status')!='COMPLETE' or restored.get('phase')!=phase
            or restored.get('plan_sha256')!=digest(plan) or restored.get('proof',{}).get('cleanup')!='verified'
            or restored!=_read(root/'restore'/nonce/'receipt.json')
            or restored['resources_sha256']!=digest(_read(root/'restore'/nonce/'resources.json'))):
            raise Blocked('cohort_history_restore_changed')
    if any(_read(path).get('status')!='DONE' for path in (root/'effects').iterdir()):
        raise Blocked('cohort_history_incomplete')
    if fixed['space_inspection_sha256'] is not None:
        import staging_cohort_preservation_space as space
        import staging_space_native as inspector
        tool=_read(root/'space-tool.json')['authenticated_publication']['publication']
        if (digest(space._available_record(operation,plan,tool))!=fixed['space_tool_available_sha256']
            or digest(inspector.validate_historical_inspection(operation))!=fixed['space_inspection_sha256']):
            raise Blocked('cohort_space_proof_changed')
    return copy.deepcopy(receipt)
