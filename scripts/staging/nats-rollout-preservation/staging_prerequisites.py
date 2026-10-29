"""Root-private, finite prerequisite authority for the staging rollout."""
import copy
import hashlib
import importlib
import json
import os
import re
from pathlib import Path

from controller import Blocked
import encrypted_cut
import guard
import resource_claim
from root_main import save


SLOTS=('release_policy','p3','gis_material')
CHILD_MODULES={'release_policy':'staging_release_policy_prepare','p3':'staging_p3_prerequisites',
               'gis_material':'staging_gis_material'}
PLAN_FIELDS={'schema','operation','context','helper_sha256','source_capture_sha256','approved_sha256',
    'execution_sha256','namespace_uid','marker_sha256','baseline_sha256','actual_before',
    'policy_before_sha256','children','auxiliary_images','renderer_authority_sha256'}
SHA=re.compile(r'[a-f0-9]{64}')
OP=re.compile(r'[a-f0-9]{12}')


def digest(value):
    return resource_claim.digest(value)


def _read_private(path,limit=2<<20):
    fd,info=encrypted_cut.regular(path,private=True)
    try:
        if not 1<=info.st_size<=limit:raise Blocked('staging_prerequisites_private_invalid')
        chunks=[];remaining=info.st_size
        while remaining:
            chunk=os.read(fd,remaining)
            if not chunk:raise Blocked('staging_prerequisites_private_changed')
            chunks.append(chunk);remaining-=len(chunk)
        after=os.fstat(fd)
        if (os.read(fd,1) or (after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns)!=
                (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns)):
            raise Blocked('staging_prerequisites_private_changed')
        return b''.join(chunks)
    finally:os.close(fd)


def _base(session):
    if not OP.fullmatch(session.operation):raise Blocked('staging_prerequisites_operation_invalid')
    return guard.ROOT/'installed'/'prerequisites'/session.operation


def validate_plan(plan,session):
    """Validate the closed protected identity; child preflight remains mandatory."""
    try:
        if (type(plan) is not dict or set(plan)!=PLAN_FIELDS
            or plan['schema']!='voice-staging-prerequisites-plan-v1'
            or plan['operation']!=session.operation or plan['context']!=session.context
            or plan['helper_sha256']!=session.helper or not OP.fullmatch(plan['operation'])
            or any(not isinstance(plan[k],str) or not SHA.fullmatch(plan[k]) for k in
                ('helper_sha256','source_capture_sha256','approved_sha256','execution_sha256',
                 'marker_sha256','baseline_sha256','policy_before_sha256'))
            or plan['renderer_authority_sha256'] is not None and
                (not isinstance(plan['renderer_authority_sha256'],str) or not SHA.fullmatch(plan['renderer_authority_sha256']))
            or not isinstance(plan['namespace_uid'],str) or not plan['namespace_uid']
            or type(plan['actual_before']) is not dict or not plan['actual_before']
            or type(plan['children']) is not dict or set(plan['children'])!=set(SLOTS)
            or type(plan['auxiliary_images']) is not dict
            or set(plan['auxiliary_images'])-{'voice-p3-principal-jwks/jwks'}):
            raise ValueError()
        for image in plan['auxiliary_images'].values():
            if not isinstance(image,str) or not re.fullmatch(r'[^\s@]+@sha256:[a-f0-9]{64}',image):raise ValueError()
        source=json.loads(_read_private(guard.ROOT/'installed'/'source-authority'/(session.operation+'.json')),
                          object_pairs_hook=guard.object_pairs)
        if (type(source) is not dict or set(source)!={'schema','operation','context','helper_sha256','approved_sha256','execution_sha256'}
            or source['schema']!='voice-staging-source-capture-v1' or source['operation']!=session.operation
            or source['context']!=session.context or source['helper_sha256']!=session.helper
            or source['approved_sha256']!=plan['approved_sha256'] or source['execution_sha256']!=plan['execution_sha256']
            or digest(source)!=plan['source_capture_sha256']):raise ValueError()
    except (KeyError,TypeError,ValueError,AttributeError):
        raise Blocked('staging_prerequisites_plan_invalid') from None
    return copy.deepcopy(plan)


def _checkpoint(session):
    row=session.check()
    if row.get('phase')!='prerequisites':raise Blocked('staging_prerequisites_phase_invalid')
    state=json.loads(_read_private(_base(session)/'checkpoint.json'),object_pairs_hook=guard.object_pairs)
    if (type(state) is not dict or set(state)!={'schema','plan','initial_resource_fence','status','steps','receipt','preflight_sha256'}
        or state['schema']!='voice-staging-prerequisites-checkpoint-v1' or state['status']!='RUNNING'
        or type(state['steps']) is not dict or state['receipt'] is not None):
        raise Blocked('staging_prerequisites_checkpoint_invalid')
    plan=validate_plan(state['plan'],session)
    session.require_binding(state['initial_resource_fence'])
    if row.get('target_sha256')!=digest(plan):raise Blocked('staging_prerequisites_binding_changed')
    return state

def _store_state(session,before,after):
    if _checkpoint(session)!=before:raise Blocked('staging_prerequisites_checkpoint_changed')
    save(_base(session)/'checkpoint.json',after)
    if _checkpoint(session)!=after:raise Blocked('staging_prerequisites_checkpoint_changed')

def preflight_all(session):
    state=_checkpoint(session)
    if state['preflight_sha256'] is not None or state['steps']:
        raise Blocked('staging_prerequisites_reconciliation_required')
    for slot in SLOTS:
        importlib.import_module(CHILD_MODULES[slot]).preflight_payloads(session)
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    after=copy.deepcopy(state);after['preflight_sha256']=digest(state['plan'])
    _store_state(session,state,after)

def _effect_order(plan):
    result=[]
    for slot in SLOTS:
        child=plan['children'][slot]
        if not isinstance(child,dict) or not isinstance(child.get('effects'),list) or not child['effects']:
            raise Blocked('staging_prerequisites_child_invalid')
        for effect in child['effects']:
            if not isinstance(effect,dict) or not isinstance(effect.get('id'),str) or not re.fullmatch(r'[a-z0-9-]{1,64}',effect['id']):
                raise Blocked('staging_prerequisites_effect_invalid')
            result.append((slot,effect['id']))
    if len(result)!=len(set(result)):raise Blocked('staging_prerequisites_effect_invalid')
    return result

def record_pending(session,slot,effect_id):
    state=_checkpoint(session)
    if state['preflight_sha256']!=digest(state['plan']):raise Blocked('staging_prerequisites_preflight_required')
    expected=_effect_order(state['plan']);done=[]
    if set(state['steps'])-set(SLOTS):raise Blocked('staging_prerequisites_checkpoint_invalid')
    for s in SLOTS:
        items=state['steps'].get(s,{})
        if not isinstance(items,dict):raise Blocked('staging_prerequisites_checkpoint_invalid')
        for key,value in items.items():
            if not isinstance(value,dict) or set(value)!={'status','observation'} or value.get('status')!='DONE':
                raise Blocked('staging_prerequisites_reconciliation_required')
            done.append((s,key))
    if set(done)!=set(expected[:len(done)]) or len(done)>=len(expected) or expected[len(done)]!=(slot,effect_id):
        raise Blocked('staging_prerequisites_effect_order')
    after=copy.deepcopy(state);after['steps'].setdefault(slot,{})[effect_id]={'status':'PENDING'}
    _store_state(session,state,after)

def record_done(session,slot,effect_id,observation):
    state=_checkpoint(session)
    if state['preflight_sha256']!=digest(state['plan']) or state['steps'].get(slot,{}).get(effect_id)!={'status':'PENDING'}:
        raise Blocked('staging_prerequisites_reconciliation_required')
    if slot not in SLOTS or (slot,effect_id) not in _effect_order(state['plan']):raise Blocked('staging_prerequisites_effect_invalid')
    verified=importlib.import_module(CHILD_MODULES[slot]).verify_effect(session,effect_id,observation)
    if verified!=observation or _checkpoint(session)!=state:raise Blocked('staging_prerequisites_observation_changed')
    after=copy.deepcopy(state);after['steps'][slot][effect_id]={'status':'DONE','observation':copy.deepcopy(verified)}
    _store_state(session,state,after)


def require_child(session,slot):
    if slot not in SLOTS:raise Blocked('staging_prerequisites_slot_invalid')
    state=_checkpoint(session);plan=state['plan']['children'][slot]
    if type(plan) is not dict:raise Blocked('staging_prerequisites_child_invalid')
    # The fixed child preflight must independently validate its complete plan,
    # payloads and protected issuer/ancestry before any effect is permitted.
    return copy.deepcopy(plan)


def load_payload(session,slot,effect_id):
    plan=require_child(session,slot)
    if not isinstance(effect_id,str) or not re.fullmatch(r'[a-z0-9-]{1,64}',effect_id):
        raise Blocked('staging_prerequisites_effect_invalid')
    effects=plan.get('effects')
    if type(effects) is not list:raise Blocked('staging_prerequisites_effect_invalid')
    selected=[row for row in effects if type(row) is dict and row.get('id')==effect_id]
    if len(selected)!=1 or not isinstance(selected[0].get('payload_sha256'),str) or not SHA.fullmatch(selected[0]['payload_sha256']):
        raise Blocked('staging_prerequisites_effect_invalid')
    raw=_read_private(_base(session)/'payloads'/slot/(effect_id+'.json'))
    if hashlib.sha256(raw).hexdigest()!=selected[0]['payload_sha256']:
        raise Blocked('staging_prerequisites_payload_changed')
    session.check()
    return raw
