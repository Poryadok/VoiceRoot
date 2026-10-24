"""V6-only all-policy cut and untouched native consumer proof.

These validators are not authority to capture or mutate a live broker. Root
binds their inputs to an original private cold archive and durable journal.
"""
import copy
from datetime import datetime,timezone
import json
from pathlib import Path
import re
import tempfile

from nats_contract_plan import V6_SCHEMA,V6_MIGRATION_ID,V6_DECLARATIONS,digest,ContractError
from nats_contract_actor import persistent_config
from docker_runtime import NATS_IMAGE

STREAM='voice_events'
DURABLE='matchmaking_voice_roster_v1'
SUBJECT='voice.roster_membership_changed'
CUT_SCHEMA='voice-nats-deliver-all-cut-v1'
_SHA=re.compile('[a-f0-9]{64}')
_BINDINGS={'operation','plan_sha256','enrollment_sha256','live_auth_sha256',
    'server_image','account','archive_sha256','manifest_sha256','census_sha256'}
_CUT_KEYS={'schema','bindings','original_stream','durable','filter_subject',
    'normalized_config_sha256','matching_messages','count_response_sha256','restore'}
_ATTEMPT={'container_id','server_image','started_at','finished_at'}
_EVIDENCE={'stream','name','cut_sha256','plan_sha256','operation','action_sha256',
    'target_config_sha256','attempt','issued_event_sha256','native_created','native_members',
    'native_state','restored_row_sha256'}

def fail():raise ContractError('nats_deliver_all_proof_invalid')
def _sha(value):return isinstance(value,str) and _SHA.fullmatch(value) is not None
def _uint(value):return type(value) is int and 0<=value<=2**64-1
def _stamp(value):
    if not isinstance(value,str):fail()
    try:result=datetime.fromisoformat(value.replace('Z','+00:00'))
    except ValueError:fail()
    if result.tzinfo is None:fail()
    return result

def attempt_shape(row):
    if not isinstance(row,dict) or set(row)!=_ATTEMPT:fail()
    if row['server_image']!=NATS_IMAGE or not _sha(row['container_id']):fail()
    if _stamp(row['finished_at'])<_stamp(row['started_at']):fail()

def start_sequence(state):
    f,l,m=(state.get(k) for k in ('first_seq','last_seq','messages'))
    if not all(_uint(v) for v in (f,l,m)):fail()
    if m:
        if not 1<=f<=l or m>l-f+1:fail()
    elif (f,l)!=(0,0) and f!=l+1:fail()
    return 1 if f==0 else min(f,l+1)

def count_response(response,original):
    """Interpret the pinned exact-subject STREAM.INFO reply, never JSZ absence."""
    if (not isinstance(response,dict) or set(response)-{'type','total','offset','limit',
        'config','created','state','cluster','ts'} or response.get('type')!='io.nats.jetstream.api.v1.stream_info_response'
        or response.get('offset')!=0
        or type(response.get('total')) is not int or response['total'] not in (0,1)
        or response.get('limit')!=(0 if response['total']==0 else 100000)):
        fail()
    if (not isinstance(response.get('config'),dict) or response['config'].get('name')!=STREAM
        or digest(persistent_config(response['config']))!=original.get('config_sha256')
        or response.get('created')!=original.get('created') or not isinstance(response.get('state'),dict)):
        fail()
    state=copy.deepcopy(response['state']);subjects=state.pop('subjects',{})
    if state!=original.get('state'):fail()
    start_sequence(state)
    if response['total']==0:
        if subjects!={}:fail()
        return 0
    if (not isinstance(subjects,dict) or set(subjects)!={SUBJECT}
        or not _uint(subjects[SUBJECT]) or not 0<subjects[SUBJECT]<=state['messages']):fail()
    return subjects[SUBJECT]

def expected_bindings(state):
    plan=state.get('nats_contract',{})
    binding=state.get('nats_contract_binding',{})
    cut=state.get('cut',{})
    if plan.get('schema')!=V6_SCHEMA or plan.get('migration_id')!=V6_MIGRATION_ID:fail()
    result={'operation':state.get('operation'),'plan_sha256':digest(plan),
        'enrollment_sha256':digest(state.get('bootstrap_enrollment',{})),
        'live_auth_sha256':binding.get('live_auth_sha256'),'server_image':NATS_IMAGE,
        'account':cut.get('census',{}).get('account'),'archive_sha256':cut.get('manifest',{}).get('archive_sha256'),
        'manifest_sha256':cut.get('manifest_sha256'),'census_sha256':cut.get('census_sha256')}
    if (not isinstance(result['operation'],str) or not re.fullmatch('[a-z0-9]{6,80}',result['operation'])
        or not isinstance(result['account'],str) or not re.fullmatch('A[A-Z2-7]{55}',result['account'])
        or any(not _sha(result[k]) for k in _BINDINGS-{'operation','server_image','account'})
        or binding.get('plan_sha256')!=result['plan_sha256']
        or binding.get('enrollment_sha256')!=result['enrollment_sha256']
        or binding.get('sources')!=plan.get('sources') or binding.get('server_image')!=NATS_IMAGE
        or ('archive_sha256' in cut and cut['archive_sha256']!=result['archive_sha256'])
        or digest(cut.get('manifest'))!=result['manifest_sha256']
        or digest(cut.get('census'))!=result['census_sha256']):fail()
    return result

def original_stream(state):
    rows=[r for r in state['cut']['census']['streams'] if r.get('name')==STREAM]
    if len(rows)!=1:fail()
    start_sequence(rows[0]['state'])
    return rows[0]

def target_hash(plan):
    rows=[r for r in plan.get('consumer_targets',[]) if r.get('stream')==STREAM and r.get('name')==DURABLE]
    if len(rows)!=1 or not _sha(rows[0].get('config_sha256')):fail()
    return rows[0]['config_sha256']

def original_presence(state):
    rows=[c for c in state['cut']['census']['consumers'] if c.get('stream')==STREAM and c.get('name')==DURABLE]
    if len(rows)>1:fail()
    root='jetstream/'+state['cut']['census']['account']+'/streams/'+STREAM+'/obs/'+DURABLE
    paths=[f['path'] for f in state['cut']['manifest']['files'] if f['path'].startswith(root+'/')]
    dirs=[p for p in state['cut']['manifest'].get('dirs',[]) if p==root or p.startswith(root+'/')]
    if rows:
        if set(paths)!={root+'/'+n for n in ('meta.inf','meta.sum','o.dat')}:fail()
    elif paths or dirs:fail()
    return bool(rows)

def validate_cut(cut,state):
    cut_shape(cut)
    if (cut.get('bindings')!=expected_bindings(state) or cut.get('original_stream')!=original_stream(state)
        or cut.get('durable')!=DURABLE or cut.get('filter_subject')!=SUBJECT
        or cut.get('normalized_config_sha256')!=target_hash(state['nats_contract'])
        or not _sha(cut.get('count_response_sha256')) or not _uint(cut.get('matching_messages'))
        or cut['matching_messages']>cut['original_stream']['state']['messages']):fail()
    restore=cut.get('restore')
    if not isinstance(restore,dict) or set(restore)!=_ATTEMPT|{'native_message_sha256'}:fail()
    attempt_shape({k:restore[k] for k in _ATTEMPT})
    if not _sha(restore['native_message_sha256']):fail()
    original_presence(state)
    return cut

def cut_shape(cut):
    if not isinstance(cut,dict) or set(cut)!=_CUT_KEYS or cut.get('schema')!=CUT_SCHEMA:fail()
    bindings=cut.get('bindings')
    if (not isinstance(bindings,dict) or set(bindings)!=_BINDINGS
        or bindings.get('server_image')!=NATS_IMAGE
        or not isinstance(bindings.get('operation'),str) or not re.fullmatch('[a-z0-9]{6,80}',bindings['operation'])
        or not isinstance(bindings.get('account'),str) or not re.fullmatch('A[A-Z2-7]{55}',bindings['account'])
        or any(not _sha(bindings.get(k)) for k in _BINDINGS-{'operation','server_image','account'})):
        fail()
    original=cut.get('original_stream')
    if (not isinstance(original,dict) or original.get('name')!=STREAM or not _sha(original.get('config_sha256'))
        or not isinstance(original.get('state'),dict)):fail()
    _stamp(original.get('created'));start_sequence(original['state'])
    if (cut.get('durable')!=DURABLE or cut.get('filter_subject')!=SUBJECT
        or not _sha(cut.get('normalized_config_sha256')) or not _sha(cut.get('count_response_sha256'))
        or not _uint(cut.get('matching_messages')) or cut['matching_messages']>original['state']['messages']):fail()
    restore=cut.get('restore')
    if not isinstance(restore,dict) or set(restore)!=_ATTEMPT|{'native_message_sha256'}:fail()
    attempt_shape({k:restore[k] for k in _ATTEMPT})
    if not _sha(restore['native_message_sha256']):fail()
    return cut

def make_cut(state,response,restore):
    original=original_stream(state)
    result={'schema':CUT_SCHEMA,'bindings':expected_bindings(state),
        'original_stream':copy.deepcopy(original),'durable':DURABLE,'filter_subject':SUBJECT,
        'normalized_config_sha256':target_hash(state['nats_contract']),
        'matching_messages':count_response(response,original),
        'count_response_sha256':digest(response),'restore':copy.deepcopy(restore)}
    return validate_cut(result,state)

def untouched_initial(cut,row,native):
    """Exact source-derived and pinned-native-qualified CLOSED initializer."""
    s=start_sequence(cut['original_stream']['state'])
    if (row.get('stream')!=STREAM or row.get('name')!=DURABLE or row.get('durable')!=DURABLE
        or row.get('config_sha256')!=cut['normalized_config_sha256']
        or row.get('inactive_threshold')!=0
        or row.get('delivered')!={'consumer_seq':0,'stream_seq':s-1}
        or row.get('ack_floor')!={'consumer_seq':0,'stream_seq':0}
        or row.get('num_pending')!=cut['matching_messages']
        or row.get('num_ack_pending')!=0 or row.get('num_redelivered')!=0):fail()
    observations=row.get('observations')
    if (not isinstance(observations,dict) or observations.get('num_waiting')!=0
        or observations.get('push_bound',False) is not False or observations.get('paused',False) is not False
        or observations.get('pause_remaining',0)!=0
        or any(k in observations for k in ('delivered_last_active','ack_floor_last_active','priority_groups'))):fail()
    from closed_durable import state_shape
    state_shape(native)
    if (native['delivered']!={'consumer':0,'stream':s-1} or native['ack_floor']!={'consumer':0,'stream':0}
        or native['pending']!={} or native['redelivered']!={}):fail()

def capture_original_cut(runtime,state,stage,journal):
    """Root-only capture from the original immutable archive, never live INFO."""
    from preserve import (closed_copy_census,native_catalog,closed_native_states,
        native_message_files)
    from native_store import archive_closed_store
    from nats_contract_actor import Actor
    from scenario import ready
    authority=stage.verify_final_storage
    saved=state.get('nats_deliver_all_cut')
    if saved is not None:
        validate_cut(saved,state)
        if not any(e.get('kind')=='nats_deliver_all_cut_bound' and e.get('sha256')==digest(saved)
            and e.get('descriptor')==saved for e in state['events']):fail()
        return saved
    expected_bindings(state);original_presence(state);authority()
    archive=Path(state['cut'].get('archive',Path(runtime.base)/'rollout-before.tar'))
    if not archive.is_absolute():archive=Path(runtime.base)/archive
    manifest=state['cut']['manifest']
    # Inventory includes incomplete directories; API absence cannot hide them.
    native_catalog(runtime,archive,manifest,authority)
    old_states=closed_native_states(runtime,archive,manifest,state['cut']['census'],authority)
    result={};started=datetime.now(timezone.utc).isoformat()
    def validate(r,broker,tree,row,unused):
        authority();owned=r.inspect(broker)
        if r.account_id()!=state['cut']['census']['account']:fail()
        stores=[m for m in owned['Mounts'] if m.get('Destination')=='/data']
        if len(stores)!=1 or stores[0].get('Type')!='bind' or not stores[0].get('RW'):fail()
        actor=Actor(r,broker,owned['Id'],stores[0]['Source'],authority,plan=state['nats_contract'])
        actor.assert_closed_isolation()
        response=actor._request('$JS.API.STREAM.INFO.voice_events',{'subjects_filter':SUBJECT,'offset':0})
        count_response(response,original_stream(state))
        actual=[s for s in row['streams'] if s.get('name')==STREAM]
        if len(actual)!=1 or any(actual[0].get(k)!=original_stream(state).get(k) for k in ('created','state','config_sha256')):fail()
        r.stop(broker);authority()
        if r.inspect(broker)['State']['Running']:fail()
        closed=Path(tempfile.mkdtemp(prefix='all-cut-closed-',dir=r.base))/'copy.tar'
        current=archive_closed_store(stores[0]['Source'],closed)
        messages=native_message_files(current)
        if messages!=native_message_files(manifest):fail()
        if closed_native_states(r,closed,current,row,authority)!=old_states:fail()
        restore={'container_id':owned['Id'],'server_image':NATS_IMAGE,
            'started_at':started,'finished_at':datetime.now(timezone.utc).isoformat(),
            'native_message_sha256':messages['sha256']}
        result['descriptor']=make_cut(state,response,restore)
    closed_copy_census(runtime,archive,manifest,'all-original',validate)
    descriptor=result['descriptor'];state['nats_deliver_all_cut']=copy.deepcopy(descriptor)
    journal({'kind':'nats_deliver_all_cut_bound','sha256':digest(descriptor),'descriptor':descriptor})
    return descriptor

def _action(state):
    actions=[a for a in state['nats_contract'].get('actions',[]) if a.get('object')==STREAM+'/'+DURABLE]
    if len(actions)!=1:fail()
    action=actions[0]
    if (action.get('api')!='$JS.API.CONSUMER.CREATE.'+STREAM+'.'+DURABLE or action.get('before') is not None
        or action.get('after')!={'stream_name':STREAM,'action':'create','config':V6_DECLARATIONS[-1][1]}
        or action.get('normalized_config_sha256')!=target_hash(state['nats_contract'])):fail()
    return action

def collect_created(runtime,state,archive,manifest,row,authority):
    """Prove original absence and one owned issued+closed native creation."""
    from preserve import (native_catalog,verify_catalog,closed_native_states,
        _native_members,checksum_current_metadata)
    cut=validate_cut(state.get('nats_deliver_all_cut'),state)
    if original_presence(state):return []
    action=_action(state)
    catalog=native_catalog(runtime,archive,manifest,authority);verify_catalog(catalog,row)
    matches=[c for c in row['consumers'] if c.get('stream')==STREAM and c.get('name')==DURABLE]
    if not matches:return []  # The remaining CREATE may still be unissued.
    if len(matches)!=1:fail()
    root='jetstream/'+row['account']+'/streams/'+STREAM+'/obs/'+DURABLE
    members={f['path']:f['sha256'] for f in manifest['files'] if f['path'].startswith(root+'/')}
    if set(members)!={root+'/'+n for n in ('meta.inf','meta.sum','o.dat')}:fail()
    raw=_native_members(runtime,archive,manifest,set(members),authority)
    if checksum_current_metadata(STREAM,DURABLE,raw[root+'/meta.inf'])!=raw[root+'/meta.sum']:fail()
    def pairs(items):
        result={}
        for k,v in items:
            if k in result:fail()
            result[k]=v
        return result
    meta=json.loads(raw[root+'/meta.inf'],object_pairs_hook=pairs)
    if not isinstance(meta,dict) or meta.get('Name')!=DURABLE or meta.get('Created')!=matches[0].get('created'):fail()
    config={k:v for k,v in meta.items() if k not in ('Name','Created')}
    if digest(config)!=cut['normalized_config_sha256']:fail()
    native=closed_native_states(runtime,archive,manifest,row,authority)[root+'/o.dat']
    untouched_initial(cut,matches[0],native)
    opened={e['container_id']:e for e in state['events'] if e.get('kind')=='nats_contract_broker_opened'}
    closed={e['container_id']:e for e in state['events'] if e.get('kind')=='nats_contract_broker_closed'}
    issued=[]
    created=_stamp(meta['Created'])
    for e in state['events']:
        cid=e.get('migration_attempt')
        if (e.get('kind')!='nats_contract_mutation_issued' or e.get('object')!=action['object']
            or e.get('api')!=action['api'] or e.get('target_sha256')!=cut['normalized_config_sha256']
            or cid not in opened or cid not in closed):continue
        start,end=opened[cid],closed[cid]
        attempt={'container_id':cid,'server_image':start.get('server_image'),
            'started_at':start.get('started_at'),'finished_at':end.get('finished_at')}
        attempt_shape(attempt)
        if end.get('started_at')!=attempt['started_at'] or end.get('server_image')!=NATS_IMAGE:fail()
        if _stamp(attempt['started_at'])<=created<=_stamp(attempt['finished_at']):issued.append((e,attempt))
    if len(issued)!=1:fail()
    event,attempt=issued[0]
    evidence={'stream':STREAM,'name':DURABLE,'cut_sha256':digest(cut),'plan_sha256':digest(state['nats_contract']),
        'operation':state['operation'],'action_sha256':digest(action),'target_config_sha256':cut['normalized_config_sha256'],
        'attempt':attempt,'issued_event_sha256':digest(event),'native_created':meta['Created'],
        'native_members':members,'native_state':native,'restored_row_sha256':digest(matches[0])}
    authority();return [evidence]

def validate_evidence(cut,evidence,plan=None,row=None):
    cut_shape(cut)
    if not isinstance(evidence,dict) or set(evidence)!=_EVIDENCE:fail()
    if (evidence.get('stream')!=STREAM or evidence.get('name')!=DURABLE
        or evidence.get('cut_sha256')!=digest(cut)
        or evidence.get('plan_sha256')!=cut['bindings']['plan_sha256']
        or evidence.get('operation')!=cut['bindings']['operation']
        or evidence.get('target_config_sha256')!=cut['normalized_config_sha256']
        or any(not _sha(evidence.get(k)) for k in ('action_sha256','issued_event_sha256','restored_row_sha256'))):fail()
    attempt=evidence.get('attempt');attempt_shape(attempt)
    created=_stamp(evidence.get('native_created'))
    if not _stamp(attempt['started_at'])<=created<=_stamp(attempt['finished_at']):fail()
    root='jetstream/'+cut['bindings']['account']+'/streams/'+STREAM+'/obs/'+DURABLE
    members=evidence.get('native_members')
    if (not isinstance(members,dict) or set(members)!={root+'/'+n for n in ('meta.inf','meta.sum','o.dat')}
        or any(not _sha(value) for value in members.values())):fail()
    native=evidence.get('native_state')
    if not isinstance(native,dict):fail()
    # Receipt validation retains the exact native initializer even without the
    # current API row. Root census validation additionally checks that full row.
    if row is None:
        row={'stream':STREAM,'name':DURABLE,'durable':DURABLE,'inactive_threshold':0,
            'config_sha256':cut['normalized_config_sha256'],
            'delivered':{'consumer_seq':0,'stream_seq':start_sequence(cut['original_stream']['state'])-1},
            'ack_floor':{'consumer_seq':0,'stream_seq':0},'num_pending':cut['matching_messages'],
            'num_ack_pending':0,'num_redelivered':0,'observations':{'num_waiting':0}}
    else:
        if row.get('created')!=evidence['native_created'] or digest(row)!=evidence['restored_row_sha256']:fail()
    untouched_initial(cut,row,native)
    if plan is not None:
        if (plan.get('schema')!=V6_SCHEMA or plan.get('migration_id')!=V6_MIGRATION_ID
            or digest(plan)!=cut['bindings']['plan_sha256'] or target_hash(plan)!=cut['normalized_config_sha256']):fail()
        actions=[a for a in plan.get('actions',[]) if a.get('object')==STREAM+'/'+DURABLE]
        if len(actions)!=1 or digest(actions[0])!=evidence['action_sha256']:fail()
    return evidence

def census_inputs(plan,before,after,proof):
    """Validate V6 evidence before generic semantic() removes observations."""
    if not isinstance(proof,dict) or set(proof)!={'cut','created','authority_plan'}:fail()
    cut=cut_shape(proof['cut']);created=proof['created']
    authority=proof['authority_plan']
    if (not isinstance(authority,dict) or {k:v for k,v in plan.items() if k!='actions'}!={k:v for k,v in authority.items() if k!='actions'}
        or not isinstance(plan.get('actions'),list) or not isinstance(authority.get('actions'),list)
        or any(a not in authority['actions'] for a in plan['actions'])):fail()
    if (cut['bindings']['plan_sha256']!=digest(authority) or cut['bindings']['census_sha256']!=digest(before)
        or cut['bindings']['account']!=before.get('account') or before.get('account')!=after.get('account')):fail()
    originals=[r for r in before.get('streams',[]) if r.get('name')==STREAM]
    if originals!=[cut['original_stream']]:fail()
    if target_hash(plan)!=cut['normalized_config_sha256']:fail()
    actions=[a for a in plan.get('actions',[]) if a.get('object')==STREAM+'/'+DURABLE]
    old=[r for r in before.get('consumers',[]) if r.get('stream')==STREAM and r.get('name')==DURABLE]
    if old:
        if len(old)!=1 or actions or created!=[]:fail()
    elif actions:
        if len(actions)!=1 or not isinstance(created,list) or len(created)!=1:fail()
        rows=[r for r in after.get('consumers',[]) if r.get('stream')==STREAM and r.get('name')==DURABLE]
        if len(rows)!=1:fail()
        validate_evidence(cut,created[0],authority,rows[0])
    else:
        if created!=[] or any(r.get('stream')==STREAM and r.get('name')==DURABLE for r in after.get('consumers',[])):fail()
    return cut,created
