"""Exact human-authorized disposable staging reset, never preservation PASS.

The failed a83 migration genuinely expired old records. Its checkpoint, selected
PVC and archives remain historical evidence; a separately proved empty PVC is a
new baseline, not a continuation or rollback of that preservation attempt.
"""
import copy
import hashlib
import json
import re
from pathlib import Path

from controller import Blocked
from rollout_census import validate_snapshot as validate,semantic

OPERATION='a83c5bdb26f1'
CHECKPOINT_SHA='5455398c26fc71012ddcdf2937b8e42cfae653b0744168c86e03431d6d46510a'
CODE_SHA='43f9041e08ad39512800da54a067adb6a7d2a7e9f66f08b13fafea57512e17f0'
ACTIVE_SHA='35668216293161e93876fffe8ee08563b516800c4a0ace82f75baebb821809c9'
CONTRACT_SHA='9557c9cdb4552c644af79f970f7ef881a7c91645eab82d1c267b218a56452808'
POST_ARCHIVE_SHA='cc251232ef2b8e0d019a596221303fab523d7b83b01a7c4ad76308c81135a251'
WAIVER='EXPLICIT_USER_AUTHORIZED_DISPOSABLE_STAGING_RESET'
PREFIX='operator-expiry-reset'


def fail():raise Blocked('operator_expiry_reset_refused')
def canonical(value):return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode()).hexdigest()
def sha(raw):return hashlib.sha256(raw).hexdigest()


def configuration(row):
    value=validate(row)
    return canonical({'account':value['account'],
        'streams':[{k:s[k] for k in ('name','config_sha256')} for s in value['streams']],
        'consumers':[{k:c[k] for k in ('stream','name','durable','config_sha256','inactive_threshold')} for c in value['consumers']]})


def zero_baseline(before,after):
    """All retained configurations, but expressly no old-record preservation."""
    try:
        after=validate(after)
        if configuration(before)!=configuration(after):fail()
        for row in after['streams']:
            state=row['state']
            if (any(state.get(k)!=0 for k in ('messages','bytes','last_seq'))
                or state.get('first_seq') not in (0,1)
                or any(state.get(k) for k in ('deleted','num_deleted','lost','subjects','num_subjects'))):fail()
        for row in after['consumers']:
            if (any(row[k]!=0 for k in ('num_ack_pending','num_redelivered','num_pending'))
                or any(row[k]!={'consumer_seq':0,'stream_seq':0} for k in ('delivered','ack_floor'))):fail()
    except (ValueError,KeyError,TypeError):fail()
    return {'schema':'voice-empty-reset-baseline-v1','configuration_sha256':configuration(after),
        'census_sha256':canonical(after),'streams':len(after['streams']),
        'consumers':len(after['consumers']),'messages':0,'historical_preservation':False}


def restored_baseline(before,captured,restored,native,readback):
    proof=zero_baseline(before,captured);zero_baseline(before,restored)
    if semantic(captured)!=semantic(restored) or native!=readback:fail()
    return dict(proof,restore_verified=True,native_files_verified=True)


def snapshot(raw):
    if sha(raw)!=CHECKPOINT_SHA:fail()
    state=json.loads(raw)
    if (state.get('operation')!=OPERATION or state.get('schema')!='nats-rollout-root-v1'
        or state.get('phase')!='NATS_CONTRACT_MIGRATION' or state.get('status')!='BLOCKED'
        or state.get('fence_status')!='VERIFIED' or state.get('target',{}).get('mode')!='full'
        or state.get('custody',{}).get('verified') is not True
        or canonical(state.get('code_capture'))!=CODE_SHA or canonical(state.get('contract'))!=CONTRACT_SHA
        or any(k in state for k in ('nats_migration','active_contract','migration_jobs','migration_completed_metadata',
            'renderer_applied','authorization','apply','restart','operator_reset'))):fail()
    marker=state['context']['marker']['data']
    if marker.get('phase')!='rollout-capturing' or marker.get('knownRolloutOperation')!=OPERATION:fail()
    return state


def reset_active_contract(original,baseline,claim_uid,namespace_uid,generation,authority_sha):
    if (canonical(original.get('contract'))!=CONTRACT_SHA or original.get('pvc_uid')==claim_uid
        or not re.fullmatch('[a-f0-9-]{36}',claim_uid) or not re.fullmatch('[a-f0-9]{64}',authority_sha)
        or baseline.get('historical_preservation') is not False or baseline.get('messages')!=0
        or baseline.get('restore_verified') is not True or baseline.get('native_files_verified') is not True):fail()
    return {'schema':'voice-nats-reset-active-contract-v1','operation':OPERATION,
        'namespace_uid':namespace_uid,'generation':generation,'pvc_uid':claim_uid,
        'contract':copy.deepcopy(original['contract']),'predecessor':copy.deepcopy(original),
        'baseline':copy.deepcopy(baseline),'reset_authority_sha256':authority_sha,
        'deployment_pass':False,'rollback_eligible':False}


def validate_active(row,expected,baseline_pairs):
    """New-storage authority is explicit; old migration proof stays historical."""
    from nats_active_contract import validate_identity
    keys={'schema','operation','namespace_uid','generation','pvc_uid','contract','predecessor',
          'baseline','reset_authority_sha256','deployment_pass','rollback_eligible'}
    if (set(row)!=keys or row['schema']!='voice-nats-reset-active-contract-v1'
        or row['operation']!=OPERATION or row['deployment_pass'] is not False or row['rollback_eligible'] is not False
        or any(row[k]!=expected[v] for k,v in (('namespace_uid','namespace_uid'),('generation','generation'),('pvc_uid','source_claim_uid')))
        or row['predecessor'].get('schema')!='voice-nats-active-contract-v1'
        or row['predecessor'].get('pvc_uid')==row['pvc_uid'] or row['contract']!=row['predecessor'].get('contract')
        or canonical(row['contract'])!=CONTRACT_SHA or not re.fullmatch('[a-f0-9]{64}',row['reset_authority_sha256'])):fail()
    old=dict(expected,source_claim_uid=row['predecessor']['pvc_uid'])
    validate_identity(row['predecessor'],old,baseline_pairs)
    proof=row['baseline']
    if (set(proof)!={'schema','configuration_sha256','census_sha256','streams','consumers','messages',
            'historical_preservation','restore_verified','native_files_verified'}
        or proof['schema']!='voice-empty-reset-baseline-v1' or proof['streams']!=15 or proof['consumers']!=43
        or proof['messages']!=0 or proof['historical_preservation'] is not False
        or proof['restore_verified'] is not True or proof['native_files_verified'] is not True
        or any(not re.fullmatch('[a-f0-9]{64}',proof[k]) for k in ('configuration_sha256','census_sha256'))):fail()
    validate_active_evidence(row)


def validate_active_evidence(row):
    """The envelope hash must resolve to the protected completed reset proof."""
    import guard
    from operator_retirement import private_bytes
    base=guard.ROOT/('rollout-'+OPERATION)
    state=json.loads(private_bytes(base/'checkpoint.json'))
    receipt=verify_terminal(base,state)
    authority=json.loads(private_bytes(base/(PREFIX+'-authority.json')))
    if (canonical(authority)!=row['reset_authority_sha256']
        or authority!=reset_authority(receipt['new_storage'],receipt['baseline'])
        or row['baseline']!=receipt['baseline']
        or row['pvc_uid']!=receipt['new_storage']['claim']['metadata']['uid']
        or row!=json.loads(private_bytes(base/(PREFIX+'-active-contract-after.json')))):fail()


def terminal(state,receipt):
    value=copy.deepcopy(state)
    value.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED')
    value['events']=state['events']+receipt['events']
    value['operator_reset']={'schema':'voice-operator-expiry-reset-reference-v1',
        'snapshot_sha256':CHECKPOINT_SHA,'receipt_sha256':canonical(receipt)}
    return value


def reset_authority(storage,baseline):
    return {'schema':'voice-operator-expiry-reset-authority-v1','operation':OPERATION,
        'snapshot_sha256':CHECKPOINT_SHA,'predecessor_active_sha256':ACTIVE_SHA,
        'waiver':WAIVER,'new_storage':copy.deepcopy(storage),'baseline':copy.deepcopy(baseline),
        'deployment_pass':False,'rollback_eligible':False}


def verify_journals(installed,rows):
    from operator_retirement import private_bytes
    installed=Path(installed)
    if not isinstance(rows,dict) or not 1<=len(rows)<=100:fail()
    authorizations=0
    for name,wanted in rows.items():
        if not re.fullmatch('[a-f0-9]{64}\\.json',name):fail()
        raw=private_bytes(installed/'journal'/name,2<<20)
        response_raw=private_bytes(installed/'responses'/name,2<<20,group_read=True)
        row=json.loads(raw);request=row.get('request',{});response=json.loads(response_raw)
        if (wanted!={'journal_sha256':sha(raw),'request_sha256':canonical(request),'response_sha256':sha(response_raw)}
            or row.get('request_sha256')!=canonical(request) or request.get('nonce')+'.json'!=name
            or request.get('operation',request.get('nonce','')[:12])!=OPERATION):fail()
        if row.get('phase')=='COMPLETE':
            if row.get('result')!=response or response.get('operation')!=OPERATION:fail()
        elif (row.get('phase')=='STARTED' and request.get('action')=='authorize'
            and row.get('prepare_error')=='guard_rejected'
            and response=={'status':'BLOCKED','error':'exception_Blocked'}):authorizations+=1
        else:fail()
    if authorizations!=1:fail()


def verify_terminal(base,state):
    from operator_retirement import private_bytes
    import root_cli
    base=Path(base)
    if base.name!='rollout-'+OPERATION:fail()
    old=snapshot(private_bytes(base/(PREFIX+'-snapshot.json')))
    receipt=json.loads(private_bytes(base/(PREFIX+'-receipt.json')))
    if (receipt.get('schema')!='voice-operator-expiry-reset-closure-v1'
        or receipt.get('operation')!=OPERATION or receipt.get('waiver')!=WAIVER
        or receipt.get('snapshot_sha256')!=CHECKPOINT_SHA
        or receipt.get('deployment_pass') is not False or receipt.get('rollback_eligible') is not False
        or receipt.get('old_store_retained') is not True
        or root_cli.code_binding(base/'code')!=old['code_capture'] or state!=terminal(old,receipt)):fail()
    progress=json.loads(private_bytes(base/(PREFIX+'-progress.json')))
    observed=receipt.get('old_store_inventory_sha256')
    if (not isinstance(observed,str) or not re.fullmatch('[a-f0-9]{64}',observed)
        or observed!=progress.get('old_store_inventory_sha256')
        or progress.get('operation')!=OPERATION or progress.get('phase') not in ('FINISHING','COMPLETE')):fail()
    retained=receipt.get('retained_files',{})
    required={PREFIX+'-active-contract-before.json',PREFIX+'-active-release-before.json',PREFIX+'-active-release-retired.json',
        'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json','rollout-backup.cms',
        'contract-post-sm7mqed5/nats-post-migration.tar'}
    if not isinstance(retained,dict) or set(retained)!=required:fail()
    for name,wanted in retained.items():
        raw=private_bytes(base/name,128<<20,group_read=True)
        if wanted!={'sha256':sha(raw),'bytes':len(raw)}:fail()
    if retained[PREFIX+'-active-contract-before.json']['sha256']!=ACTIVE_SHA:fail()
    if retained[PREFIX+'-active-release-retired.json']!=retained[PREFIX+'-active-release-before.json']:fail()
    if retained['contract-post-sm7mqed5/nats-post-migration.tar']['sha256']!=POST_ARCHIVE_SHA:fail()
    reset=json.loads(private_bytes(base/(PREFIX+'-baseline.json')))
    proof=restored_baseline(old['cut']['census'],reset['census'],reset['restored_census'],reset['native'],reset['native_readback'])
    if proof!=receipt.get('baseline') or reset.get('off_node_sha256')!=reset['native'].get('archive_sha256'):fail()
    if sha(private_bytes(base/(PREFIX+'-baseline.tar'),128<<20,group_read=True))!=reset['native']['archive_sha256']:fail()
    oldctx=old['context'];new=receipt.get('new_storage',{})
    if (set(new)!={'claim','pv','path','descriptor'} or new['claim']['metadata']['uid']==oldctx['final_claim']['metadata']['uid']
        or new['pv']['metadata']['uid']==oldctx['final_pv']['metadata']['uid']):fail()
    from stage_runtime import HUB,selected_store_descriptor
    originals=oldctx['original_snapshots'];setup=receipt.get('setup',{})
    if set(setup)!=set(originals):fail()
    for name,original in originals.items():
        expected=copy.deepcopy(original['spec']['template'])
        if name==HUB:
            volumes=[v for v in expected['spec']['volumes'] if v['name']=='jsdata']
            if len(volumes)!=1:fail()
            volumes[0]['persistentVolumeClaim']={'claimName':new['claim']['metadata']['name']}
        if setup[name]!={'uid':original['metadata']['uid'],'template_sha256':canonical(expected),'replicas':1,'ready':True}:fail()
    hub=copy.deepcopy(originals[HUB]);hub['spec']['template']['spec']['volumes']=[
        dict(v,persistentVolumeClaim={'claimName':new['claim']['metadata']['name']}) if v['name']=='jsdata' else v
        for v in hub['spec']['template']['spec']['volumes']]
    if selected_store_descriptor(hub,new['claim'],new['pv'])!=new['descriptor'] or new['path']!=new['descriptor']['path']:fail()
    before=oldctx['marker'];after=receipt.get('released_marker',{})
    data=dict(before['data'],phase='active',dataPVC=new['claim']['metadata']['name']);data.pop('knownRolloutOperation')
    if (set(after)!={'uid','resourceVersion','data'} or after['uid']!=before['metadata']['uid'] or after['data']!=data
        or not re.fullmatch('[0-9]+',after['resourceVersion']) or after['resourceVersion']==before['metadata']['resourceVersion']):fail()
    expected_events=[{'kind':'operator_expiry_reset_intent','waiver':WAIVER},
        {'kind':'operator_expiry_reset_completed','old_store_retained':True,'deployment_pass':False}]
    if receipt.get('events')!=expected_events:fail()
    import guard
    verify_journals(guard.ROOT/'installed',receipt.get('journals'))
    original=json.loads(private_bytes(base/(PREFIX+'-active-contract-before.json')))
    authority=reset_authority(new,proof)
    if json.loads(private_bytes(base/(PREFIX+'-authority.json')))!=authority:fail()
    active=reset_active_contract(original,proof,new['claim']['metadata']['uid'],
        oldctx['expected']['namespace_uid'],oldctx['expected']['generation'],canonical(authority))
    if json.loads(private_bytes(base/(PREFIX+'-active-contract-after.json')))!=active:fail()
    return receipt
