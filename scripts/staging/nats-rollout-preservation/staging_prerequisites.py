"""Root-private, finite prerequisite authority for the staging rollout."""
import copy
import hashlib
import importlib
import json
import os
import re
import stat
import secrets
import weakref
import base64
import importlib.util
from pathlib import Path

from controller import Blocked
import encrypted_cut
import guard
import resource_claim
from root_main import save


SLOTS=('release_policy','p3','gis_material')
CHILD_MODULES={'release_policy':'staging_release_policy_prepare','p3':'staging_p3_prerequisites',
               'gis_material':'staging_gis_material'}
EFFECT_IDS={'release_policy':('create-policy-configmap','commit-root-policy'),
    'p3':('Secret/voice-realtime-principal-signing','Secret/voice-realtime-p3-tls',
          'Secret/voice-chat-realtime-p3-tls','Secret/voice-p3-principal-jwks-tls',
          'Secret/voice-p3-principal-replay','Secret/voice-p3-principal-trust',
          'ConfigMap/voice-p3-principal-public','ConfigMap/voice-p3-principal-publisher-config',
          'Service/voice-p3-principal-jwks','Deployment/voice-p3-principal-jwks'),
    'gis_material':('gis-signer','gis-workload','gis-tls','gis-app-credentials')}
BINARY_PINS={'nats-known-baseline/kernel':'9fced2b6eb7ca33d744ceceb1550774ee7a3a6f423c1a3561ab2d3afba107548',
    'nats-rollout-preservation/bootstrap-renewer':'618f1f78e650d19c4948c1d08741ed625c6f9aa706159219dbf11c785a56237c',
    'nats-rollout-preservation/story-current4-probe':'88b6c89b9e57d028c5371db3faaea9eec4624223f54204b59056525f6971f53a'}
P2_BINARY_PINS={**BINARY_PINS,
    'nats-rollout-preservation/story-restored8-probe':'458e4e3e5e940137cddddaf42dda1c22505f92de5792600364907a343b209be0'}
REQUIRED_HELPERS=frozenset('nats-rollout-preservation/'+name+'.py' for name in
    ('staging_prerequisites','staging_release_policy_prepare','staging_p3_prerequisites','staging_p3_prerequisites_crypto',
     'staging_gis_material','gis_existing_update','gis_first_install','gis_prerequisite','source_plan','compiler'))
PLAN_FIELDS={'schema','operation','context','helper_sha256','source_capture_sha256','approved_sha256',
    'execution_sha256','namespace_uid','marker_sha256','baseline_sha256','actual_before',
    'policy_before_sha256','children','auxiliary_images','renderer_authority_sha256','previous_active_sha256'}
SHA=re.compile(r'[a-f0-9]{64}')
OP=re.compile(r'[a-f0-9]{12}')
KUBE_PREFIX=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging']
# A response is usable only in the process and Session which issued the write.
# Durable attempts remain PENDING after a lost response or process interruption.
_responses=weakref.WeakKeyDictionary()
_target_checks=weakref.WeakKeyDictionary()


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


def _validate_source_identity(session,source,approved_sha256,execution_sha256):
    try:
        if type(session) is not resource_claim.Session:raise ValueError()
        resource_claim.validate_source_capture(source,installation=session.installation)
        expected={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,
            'operation':session.operation,'context':session.context,'helper_sha256':session.helper,
            'approved_sha256':approved_sha256,'execution_sha256':execution_sha256}
        if source!=expected:raise ValueError()
    except (KeyError,TypeError,AttributeError,RuntimeError,ValueError):
        raise ValueError('staging_prerequisites_source_changed') from None


def _capture_authority(actions,session,approved,execution):
    """Capture the already-issued source and helper authority for this operation."""
    import bridge_root
    import commands
    import root_cli
    if (type(actions) is not bridge_root.Actions or type(session) is not resource_claim.Session
        or actions.tt_session is not session or actions.installation!=session.installation
        or commands.authority_guard!=session.check or session.check().get('phase')!='prepare'):
        raise Blocked('staging_prerequisites_session_invalid')
    source=json.loads(_read_private(guard.ROOT/'installed'/'source-authority'/(session.operation+'.json')),
                      object_pairs_hook=guard.object_pairs)
    try:_validate_source_identity(session,source,digest(approved),digest(execution))
    except (TypeError,ValueError):raise Blocked('staging_prerequisites_source_changed') from None
    if (type(approved) is not dict or approved.get('source_sha')!=session.context['commit']
        or type(approved.get('source_files')) is not dict or type(execution) is not dict
        or digest(approved)!=source['approved_sha256'] or digest(execution)!=source['execution_sha256']
        or digest(actions.binding)!=session.helper):
        raise Blocked('staging_prerequisites_source_changed')
    code=Path(actions.code);installed=guard.ROOT/'installed'/'code'
    if code!=installed and (code.parent!=guard.ROOT or not re.fullmatch(r'rollout-code-[a-f0-9]{32}',code.name)
                            or root_cli.rollout_directory_kind(code)!='capture'):
        raise Blocked('staging_prerequisites_helper_changed')
    info=code.lstat()
    if root_cli.code_binding(code)!=actions.binding:raise Blocked('staging_prerequisites_helper_changed')
    installed_root=guard.ROOT/'installed';encrypted_cut.directory(installed_root)
    parent=installed_root/'prerequisites'
    try:
        parent.mkdir(mode=0o700)
        directory=os.open(installed_root,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
        try:os.fsync(directory)
        finally:os.close(directory)
    except FileExistsError:pass
    encrypted_cut.directory(parent)
    if stat.S_IMODE(parent.lstat().st_mode)!=0o700:raise Blocked('staging_prerequisites_private_invalid')
    base=_base(session);base.mkdir(mode=0o700)
    directory=os.open(parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(directory)
    finally:os.close(directory)
    save(base/'approved.json',approved);save(base/'execution.json',execution)
    save(base/'code-custody.json',{'root':str(code),'manifest':actions.binding,
                                  'directory':{'dev':info.st_dev,'ino':info.st_ino}})
    workspace=guard.ROOT/'installed'/'sources'/session.operation;encrypted_cut.directory(workspace)
    _verify_helper_source(session,approved,workspace)
    session.check()
    return digest(source)


def validate_plan(plan,session):
    """Validate the closed protected identity; child preflight remains mandatory."""
    try:
        version=plan.get('schema') if type(plan) is dict else None
        parented=version in ('voice-staging-prerequisites-plan-v2','voice-staging-prerequisites-plan-v4')
        fields=PLAN_FIELDS|({'preservation_sha256'} if parented else set())
        resource_claim.record_fields(plan,'prerequisites_plan',fields,installation=session.installation)
        if (type(plan) is not dict
            or parented and
                (not isinstance(plan['preservation_sha256'],str) or not SHA.fullmatch(plan['preservation_sha256']))
            or plan['operation']!=session.operation or plan['context']!=session.context
            or plan['helper_sha256']!=session.helper or not OP.fullmatch(plan['operation'])
            or any(not isinstance(plan[k],str) or not SHA.fullmatch(plan[k]) for k in
                ('helper_sha256','source_capture_sha256','approved_sha256','execution_sha256',
                 'marker_sha256','baseline_sha256','policy_before_sha256'))
            or plan['renderer_authority_sha256'] is not None and
                (not isinstance(plan['renderer_authority_sha256'],str) or not SHA.fullmatch(plan['renderer_authority_sha256']))
            or plan['previous_active_sha256'] is not None and
                (not isinstance(plan['previous_active_sha256'],str) or not SHA.fullmatch(plan['previous_active_sha256']))
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
        _validate_source_identity(session,source,plan['approved_sha256'],plan['execution_sha256'])
        if digest(source)!=plan['source_capture_sha256']:raise ValueError()
    except (KeyError,TypeError,ValueError,AttributeError,RuntimeError):
        raise Blocked('staging_prerequisites_plan_invalid') from None
    return copy.deepcopy(plan)


def _checkpoint(session):
    row=session.check()
    state=json.loads(_read_private(_base(session)/'checkpoint.json'),object_pairs_hook=guard.object_pairs)
    if (type(state) is not dict or set(state)!={'schema','plan','initial_resource_fence','status','steps','receipt','preflight_sha256'}
        or state['schema']!='voice-staging-prerequisites-checkpoint-v1' or state['status'] not in ('RUNNING','COMPLETE')
        or type(state['steps']) is not dict or (state['receipt'] is None)!=(state['status']=='RUNNING')):
        raise Blocked('staging_prerequisites_checkpoint_invalid')
    plan=validate_plan(state['plan'],session)
    initial=state['initial_resource_fence'];current=session.binding()
    if (set(initial)!=set(current) or any(initial[k]!=current[k] for k in current if k!='target_sha256')
        or initial['target_sha256']!=digest(plan)):raise Blocked('staging_prerequisites_binding_changed')
    phase=row.get('phase')
    if phase=='prerequisites':wanted=digest(plan)
    elif state['status']=='COMPLETE' and phase in ('prerequisites-captured','realtime-bootstrap','prepare'):
        captured=_json(_read_private(_base(session)/'capture.json',limit=32<<20));capture_sha=digest(captured)
        if phase=='prerequisites-captured':wanted=capture_sha
        elif phase=='realtime-bootstrap':
            rt=_json(_read_private(guard.ROOT/'installed'/'realtime-bootstrap'/session.operation/'checkpoint.json',limit=32<<20))
            if rt['plan'].get('prerequisites_sha256')!=capture_sha:raise Blocked('staging_prerequisites_binding_changed')
            wanted=digest(rt['plan'])
        else:
            target=_target_checks.get(session)
            if target is None or target.get('prerequisites_sha256')!=capture_sha:raise Blocked('staging_prerequisites_target_required')
            wanted=digest(target)
    else:raise Blocked('staging_prerequisites_phase_invalid')
    if row.get('target_sha256')!=wanted:raise Blocked('staging_prerequisites_binding_changed')
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
        if not isinstance(child,dict) or not isinstance(child.get('effects'),list):
            raise Blocked('staging_prerequisites_child_invalid')
        ids=[]
        for effect in child['effects']:
            if not isinstance(effect,dict) or effect.get('id') not in EFFECT_IDS[slot]:
                raise Blocked('staging_prerequisites_effect_invalid')
            ids.append(effect['id'])
        # Fixed child preflight must prove the omitted inventory is preserved.
        if ids!=[i for i in EFFECT_IDS[slot] if i in ids]:
            raise Blocked('staging_prerequisites_effect_invalid')
        result.extend((slot,i) for i in ids)
    return result

def record_pending(session,slot,effect_id):
    state=_checkpoint(session)
    if state['status']!='RUNNING' or state['preflight_sha256']!=digest(state['plan']):raise Blocked('staging_prerequisites_preflight_required')
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
    for prior_slot,prior_id in expected[:len(done)]:
        observation=state['steps'][prior_slot][prior_id]['observation']
        if importlib.import_module(CHILD_MODULES[prior_slot]).verify_effect(session,prior_id,observation)!=observation:
            raise Blocked('staging_prerequisites_observation_changed')
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    after=copy.deepcopy(state);after['steps'].setdefault(slot,{})[effect_id]={'status':'PENDING'}
    _store_state(session,state,after)

def record_done(session,slot,effect_id,observation):
    state=_checkpoint(session)
    if state['preflight_sha256']!=digest(state['plan']) or state['steps'].get(slot,{}).get(effect_id)!={'status':'PENDING'}:
        raise Blocked('staging_prerequisites_reconciliation_required')
    if slot not in SLOTS or (slot,effect_id) not in _effect_order(state['plan']):raise Blocked('staging_prerequisites_effect_invalid')
    response=_responses.get(session,{}).get((slot,effect_id))
    if response is None or response['observation']!=observation or response['checkpoint_sha256']!=digest(state):
        raise Blocked('staging_prerequisites_submit_response_required')
    verified=importlib.import_module(CHILD_MODULES[slot]).verify_effect(session,effect_id,observation)
    load_authority(session)
    if verified!=observation or _checkpoint(session)!=state:raise Blocked('staging_prerequisites_observation_changed')
    after=copy.deepcopy(state);after['steps'][slot][effect_id]={'status':'DONE','observation':copy.deepcopy(verified)}
    _store_state(session,state,after)
    del _responses[session][(slot,effect_id)]


def require_child(session,slot):
    if slot not in SLOTS:raise Blocked('staging_prerequisites_slot_invalid')
    state=_checkpoint(session);plan=state['plan']['children'][slot]
    if type(plan) is not dict:raise Blocked('staging_prerequisites_child_invalid')
    for key in ('operation','namespace_uid','source_capture_sha256'):
        if plan.get(key)!=state['plan'][key]:raise Blocked('staging_prerequisites_child_binding_changed')
    # The fixed child preflight must independently validate its complete plan,
    # payloads and protected issuer/ancestry before any effect is permitted.
    return copy.deepcopy(plan)


def load_payload(session,slot,effect_id):
    plan=require_child(session,slot)
    if not isinstance(effect_id,str) or effect_id not in EFFECT_IDS[slot]:
        raise Blocked('staging_prerequisites_effect_invalid')
    effects=plan.get('effects')
    if type(effects) is not list:raise Blocked('staging_prerequisites_effect_invalid')
    selected=[row for row in effects if type(row) is dict and row.get('id')==effect_id]
    if len(selected)!=1 or not isinstance(selected[0].get('payload_sha256'),str) or not SHA.fullmatch(selected[0]['payload_sha256']):
        raise Blocked('staging_prerequisites_effect_invalid')
    raw=_read_private(_base(session)/'payloads'/slot/(hashlib.sha256(effect_id.encode('ascii')).hexdigest()+'.json'))
    if hashlib.sha256(raw).hexdigest()!=selected[0]['payload_sha256']:
        raise Blocked('staging_prerequisites_payload_changed')
    session.check()
    return raw


def load_done(session,slot):
    if slot not in SLOTS:raise Blocked('staging_prerequisites_slot_invalid')
    state=_checkpoint(session)
    if state['preflight_sha256']!=digest(state['plan']):raise Blocked('staging_prerequisites_preflight_required')
    result={}
    expected={ident for effect_slot,ident in _effect_order(state['plan']) if effect_slot==slot}
    if set(state['steps'].get(slot,{}))!=expected:raise Blocked('staging_prerequisites_incomplete')
    for ident in expected:
        step=state['steps'][slot][ident]
        if set(step)!={'status','observation'} or step['status']!='DONE':raise Blocked('staging_prerequisites_incomplete')
        key=hashlib.sha256((slot+'/'+ident).encode()).hexdigest();base=_base(session)/'attempts'
        intent=_json(_read_private(base/(key+'.json')));response=_json(_read_private(base/(key+'.response.json')))
        effect=_selected_effect(state,slot,ident)
        if (intent.get('plan_sha256')!=digest(state['plan']) or intent.get('slot')!=slot or intent.get('effect_id')!=ident
            or intent.get('operation')!=session.operation or intent.get('payload_sha256')!=effect['payload_sha256']
            or response.get('intent_sha256')!=digest(intent) or response.get('observation')!=step['observation']):
            raise Blocked('staging_prerequisites_response_changed')
        result[ident]=copy.deepcopy(step['observation'])
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    return result

def load_authority(session):
    """Return only the fixed root-captured source and its verified workspace."""
    state=_checkpoint(session);plan=state['plan']
    approved=json.loads(_read_private(_base(session)/'approved.json'),object_pairs_hook=guard.object_pairs)
    execution=json.loads(_read_private(_base(session)/'execution.json'),object_pairs_hook=guard.object_pairs)
    source=guard.ROOT/'installed'/'sources'/session.operation
    encrypted_cut.directory(source)
    if (not isinstance(approved,dict) or approved.get('source_sha')!=session.context['commit']
        or not isinstance(approved.get('source_files'),dict) or digest(approved)!=plan['approved_sha256']
        or not isinstance(execution,dict) or digest(execution)!=plan['execution_sha256']):
        raise Blocked('staging_prerequisites_source_changed')
    _verify_helper_source(session,approved,source)
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    return {'approved':copy.deepcopy(approved),'execution':copy.deepcopy(execution),'source':str(source)}


def _verify_helper_source(session,approved,source):
    """Revalidate the actual captured helper and each source-backed member."""
    import root_cli
    record=json.loads(_read_private(_base(session)/'code-custody.json'),object_pairs_hook=guard.object_pairs)
    if (type(record) is not dict or set(record)!={'root','manifest','directory'}
        or type(record['root']) is not str or type(record['manifest']) is not dict
        or type(record['directory']) is not dict or set(record['directory'])!={'dev','ino'}):
        raise Blocked('staging_prerequisites_helper_changed')
    code=Path(record['root']); installed=guard.ROOT/'installed'/'code'
    if code!=installed:
        if code.parent!=guard.ROOT or not re.fullmatch(r'rollout-code-[a-f0-9]{32}',code.name):
            raise Blocked('staging_prerequisites_helper_changed')
        if root_cli.rollout_directory_kind(code)!='capture':raise Blocked('staging_prerequisites_helper_changed')
    info=code.lstat()
    if (not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o022
        or {'dev':info.st_dev,'ino':info.st_ino}!=record['directory']
        or root_cli.code_binding(code)!=record['manifest'] or digest(record['manifest'])!=session.helper
        or not REQUIRED_HELPERS.issubset(record['manifest'])):
        raise Blocked('staging_prerequisites_helper_changed')
    for relative,wanted in record['manifest'].items():
        if relative in P2_BINARY_PINS:
            if wanted!=P2_BINARY_PINS[relative]:raise Blocked('staging_prerequisites_helper_changed')
            continue
        name='scripts/staging/'+relative
        if approved['source_files'].get(name)!=wanted:raise Blocked('staging_prerequisites_source_changed')
        path=source/name
        for parent in path.parents:
            if parent==source:break
            row=parent.lstat()
            if not stat.S_ISDIR(row.st_mode) or row.st_uid!=0 or row.st_mode&0o022:
                raise Blocked('staging_prerequisites_source_changed')
        if root_cli.public_code_sha(path)!=wanted:raise Blocked('staging_prerequisites_source_changed')
    if set(BINARY_PINS)-set(record['manifest']):raise Blocked('staging_prerequisites_helper_changed')


def _bytes(value):
    return json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode()


def _json(raw):
    return json.loads(raw,object_pairs_hook=guard.object_pairs,parse_constant=lambda _: (_ for _ in ()).throw(ValueError()))


def _private_write(path,raw,*,exclusive=True):
    """Only callers deriving fixed paths may use this private custody primitive."""
    encrypted_cut.directory(path.parent)
    temporary=path if exclusive else path.with_name('.'+path.name+'.'+secrets.token_hex(16))
    fd=os.open(temporary,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    try:
        with os.fdopen(fd,'wb') as stream:stream.write(raw);stream.flush();os.fsync(stream.fileno())
        if not exclusive:os.replace(temporary,path)
        directory=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
        try:os.fsync(directory)
        finally:os.close(directory)
    except BaseException:
        # Never delete an intent or successful effect to make a retry possible.
        raise


def _private_directory(path):
    encrypted_cut.directory(path.parent)
    try:path.mkdir(mode=0o700)
    except FileExistsError:pass
    encrypted_cut.directory(path)
    if stat.S_IMODE(path.lstat().st_mode)!=0o700:raise Blocked('staging_prerequisites_private_invalid')
    fd=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(fd)
    finally:os.close(fd)


def _semantic(row):
    if type(row) is not dict or type(row.get('metadata')) is not dict or row['metadata'].get('deletionTimestamp'):
        raise Blocked('staging_prerequisites_object_invalid')
    result=copy.deepcopy(row);result.pop('status',None)
    for key in ('uid','resourceVersion','generation','creationTimestamp','managedFields'):
        result['metadata'].pop(key,None)
    return result


def _selected_effect(state,slot,effect_id):
    if slot not in SLOTS or effect_id not in EFFECT_IDS[slot]:raise Blocked('staging_prerequisites_effect_invalid')
    rows=[row for row in state['plan']['children'][slot]['effects'] if row.get('id')==effect_id]
    if len(rows)!=1 or set(rows[0])!={'id','kind','name','namespace','before','desired_sha256','payload_sha256'}:
        raise Blocked('staging_prerequisites_effect_invalid')
    return rows[0]


def _read_command(session,args):
    import commands
    if commands.authority_guard!=session.check:raise Blocked('staging_prerequisites_command_scope_required')
    return _json(commands.capture(KUBE_PREFIX+args,timeout=30,env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'}))


def _observe_named(session,kind,name):
    if not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}',name):raise Blocked('staging_prerequisites_identity_invalid')
    plurals={'Secret':'secrets','ConfigMap':'configmaps','Service':'services','Deployment':'deployments'}
    if kind not in plurals:raise Blocked('staging_prerequisites_identity_invalid')
    if kind=='Deployment':row=_read_command(session,['get','deployments','-o','json'])
    else:
        path='/api/v1/namespaces/voice-staging/'+plurals[kind]+'?fieldSelector=metadata.name%3D'+name+'&limit=2'
        row=_read_command(session,['get','--raw',path])
    meta=row.get('metadata',{})
    if (row.get('kind')!=kind+'List' or row.get('apiVersion')!=('apps/v1' if kind=='Deployment' else 'v1')
        or not isinstance(meta.get('resourceVersion'),str) or not re.fullmatch(r'[0-9]+',meta['resourceVersion'])
        or meta.get('continue') or type(row.get('items')) is not list or len(row['items'])>2048):
        raise Blocked('staging_prerequisites_absence_invalid')
    matches=[];names=set()
    for item in row['items']:
        if item.get('metadata',{}).get('namespace')!='voice-staging':raise Blocked('staging_prerequisites_absence_invalid')
        current=item['metadata'].get('name')
        if not isinstance(current,str) or current in names:raise Blocked('staging_prerequisites_absence_invalid')
        names.add(current)
        if kind!='Deployment' and current!=name:raise Blocked('staging_prerequisites_absence_invalid')
        if current==name:
            if item.get('apiVersion',row['apiVersion'])!=row['apiVersion'] or item.get('kind',kind)!=kind:
                raise Blocked('staging_prerequisites_absence_invalid')
            matches.append({'apiVersion':row['apiVersion'],'kind':kind,**item})
    if len(matches)>1:raise Blocked('staging_prerequisites_absence_invalid')
    return meta['resourceVersion'],matches[0] if matches else None


def _absence(session,kind,name):
    rv,row=_observe_named(session,kind,name)
    if row is not None:raise Blocked('staging_prerequisites_create_collision')
    return rv


def check_effect(session,slot,effect_id):
    import commands
    from stage_runtime import Kube
    state=_checkpoint(session);effect=_selected_effect(state,slot,effect_id)
    if (state['status']!='RUNNING' or commands.authority_guard!=session.check or state['preflight_sha256']!=digest(state['plan'])
        or state['steps'].get(slot,{}).get(effect_id)!={'status':'PENDING'}):
        raise Blocked('staging_prerequisites_effect_not_admitted')
    namespace=Kube().get('namespace','voice-staging')
    if namespace['metadata'].get('uid')!=state['plan']['namespace_uid'] or namespace['metadata'].get('deletionTimestamp'):
        raise Blocked('staging_prerequisites_namespace_changed')
    load_authority(session)
    _recheck_baseline(session,state)
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    return state,effect


def _recheck_baseline(session,state):
    import realtime_bootstrap
    from stage_runtime import Kube,pv_storage_path
    baseline=_json(_read_private(_base(session)/'baseline.json',limit=32<<20))
    if digest(baseline)!=state['plan']['baseline_sha256']:
        raise Blocked('staging_prerequisites_baseline_changed')
    if 'preservation_sha256' in state['plan']:
        import staging_cohort_preservation
        staging_cohort_preservation.revalidate_paused(session)
        return
    kube=Kube();realtime_bootstrap._check_stage(kube,baseline['workloads'])
    namespace=kube.get('namespace','voice-staging');expected=baseline['expected'];storage=baseline['storage']
    claim=kube.get('pvc',expected['source_claim']);pv=kube.get('pv',claim['spec']['volumeName'])
    if (namespace['metadata'].get('uid')!=state['plan']['namespace_uid'] or namespace['metadata'].get('deletionTimestamp')
        or claim['metadata'].get('uid')!=expected['source_claim_uid'] or claim['metadata'].get('deletionTimestamp')
        or pv['metadata'].get('deletionTimestamp') or claim['spec']!=storage['claim']['spec'] or pv['spec']!=storage['pv']['spec']
        or pv_storage_path(pv,claim['metadata']['uid'],claim['metadata']['name'],expected['source_pv_uid'])!=storage['path']):
        raise Blocked('staging_prerequisites_storage_changed')
    if digest(baseline['workloads']['marker'])!=state['plan']['marker_sha256']:
        raise Blocked('staging_prerequisites_marker_changed')


def submit_effect(session,slot,effect_id):
    """The only writer: fixed transport/body, one attempt, response-bound completion."""
    import commands
    from stage_runtime import Kube
    state,effect=check_effect(session,slot,effect_id)
    raw=load_payload(session,slot,effect_id);payload=_json(raw)
    if effect['namespace']!='voice-staging':raise Blocked('staging_prerequisites_identity_invalid')
    root_policy=slot=='release_policy' and effect_id=='commit-root-policy'
    app=slot=='gis_material' and effect_id=='gis-app-credentials'
    if root_policy:
        path=guard.ROOT/'installed'/'policy.json'
        before=_read_private(path)
        previous=_json(before)
        if (effect['kind']!='RootPolicy' or effect['name']!='installed-policy'
            or hashlib.sha256(before).hexdigest()!=effect['before']['sha256']
            or payload!={**previous,'release_policy':state['plan']['children']['release_policy']['descriptor']}
            or digest(payload)!=effect['desired_sha256']):raise Blocked('staging_prerequisites_policy_changed')
        identity=path.lstat();argv=None;body=raw;desired=payload
    elif app:
        if set(payload)!={'before','after_data'} or effect['kind']!='Secret' or effect['name']!='voice-app-secrets':
            raise Blocked('staging_prerequisites_app_delta_invalid')
        before=payload['before'];meta=before['metadata'];old=before['data'];new=payload['after_data']
        added={'GAME_INTEGRATION_DB_PASSWORD','GAME_INTEGRATION_DATABASE_URL'}
        if (set(old)&added or set(new)!=set(old)|added or any(new[k]!=v for k,v in old.items())
            or effect['before']!={'uid':meta['uid'],'resourceVersion':meta['resourceVersion'],'data_sha256':digest(old)}):
            raise Blocked('staging_prerequisites_app_delta_invalid')
        actual=Kube().get('secret','voice-app-secrets')
        if (actual['metadata'].get('uid')!=meta['uid'] or actual['metadata'].get('resourceVersion')!=meta['resourceVersion']
            or _semantic(actual)!=_semantic(before)):raise Blocked('staging_prerequisites_app_changed')
        desired=_semantic(before);desired['data']=new
        if digest(desired)!=effect['desired_sha256']:raise Blocked('staging_prerequisites_payload_changed')
        patch=[{'op':'test','path':'/metadata/uid','value':meta['uid']},
               {'op':'test','path':'/metadata/resourceVersion','value':meta['resourceVersion']},
               {'op':'test','path':'/data','value':old}]
        patch.extend({'op':'add','path':'/data/'+key,'value':new[key]} for key in sorted(added))
        argv=KUBE_PREFIX+['patch','secret','voice-app-secrets','--type=json','--patch-file=/dev/stdin','-o','json'];body=_bytes(patch)
    else:
        desired=_semantic(payload)
        if (payload!=desired or payload.get('kind')!=effect['kind'] or payload['metadata'].get('name')!=effect['name']
            or payload['metadata'].get('namespace')!='voice-staging' or digest(desired)!=effect['desired_sha256']
            or set(effect['before'])!={'absent','list_rv'} or effect['before']['absent'] is not True):
            raise Blocked('staging_prerequisites_payload_changed')
        _absence(session,effect['kind'],effect['name'])
        argv=KUBE_PREFIX+['create','-f','/dev/stdin','-o','json'];body=raw
    attempts=_base(session)/'attempts';_private_directory(attempts)
    key=hashlib.sha256((slot+'/'+effect_id).encode()).hexdigest()
    intent={'schema':'voice-staging-prerequisite-attempt-v1','operation':session.operation,'plan_sha256':digest(state['plan']),
        'slot':slot,'effect_id':effect_id,'payload_sha256':effect['payload_sha256'],'body_sha256':hashlib.sha256(body).hexdigest(),
        'argv':argv,'before':effect['before']}
    # Exclusive intent survives every failure; no second transport invocation.
    _private_write(attempts/(key+'.json'),_bytes(intent))
    if check_effect(session,slot,effect_id)[0]!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    if root_policy:
        latest=path.lstat()
        if ((latest.st_dev,latest.st_ino)!=(identity.st_dev,identity.st_ino) or _read_private(path)!=before):
            raise Blocked('staging_prerequisites_policy_changed')
        _private_write(path,raw,exclusive=False)
        observation={'kind':'RootPolicy','name':'installed-policy','sha256':hashlib.sha256(raw).hexdigest()}
        actual=payload
    else:
        if session._prerequisite_permit is not None:raise Blocked('staging_prerequisites_permit_busy')
        session._prerequisite_permit=list(argv)
        try:
            actual=_json(commands.capture(argv,body=body,timeout=30,env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'}))
            if session._prerequisite_permit is not None:raise Blocked('staging_prerequisites_permit_not_consumed')
        finally:session._prerequisite_permit=None
        semantic=_semantic(actual);meta=actual['metadata']
        if (not isinstance(meta.get('uid'),str) or not meta['uid'] or not isinstance(meta.get('resourceVersion'),str)
            or not meta['resourceVersion']):raise Blocked('staging_prerequisites_response_invalid')
        if slot=='p3' and effect['kind']=='Service':
            import staging_p3_prerequisites
            staging_p3_prerequisites.service_allocation(desired,actual)
        elif semantic!=desired:raise Blocked('staging_prerequisites_response_changed')
        if app and (meta['uid']!=effect['before']['uid'] or meta['resourceVersion']==effect['before']['resourceVersion']):
            raise Blocked('staging_prerequisites_response_changed')
        observation={'kind':effect['kind'],'name':effect['name'],'namespace':'voice-staging','uid':meta['uid'],
            'resourceVersion':meta['resourceVersion'],'semantic_sha256':digest(semantic)}
        if effect['kind']=='Deployment':
            if type(meta.get('generation')) is not int or meta['generation']<1:raise Blocked('staging_prerequisites_response_invalid')
            observation['generation']=meta['generation']
    session.check();load_authority(session)
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    _private_write(attempts/(key+'.response.json'),_bytes({'intent_sha256':digest(intent),'object':actual,'observation':observation}))
    _responses.setdefault(session,{})[(slot,effect_id)]={'checkpoint_sha256':digest(state),'observation':copy.deepcopy(observation)}
    return copy.deepcopy(observation)


def _source_module(approved,source):
    """One reviewed source-captured collector; never a caller-selected import."""
    import root_cli
    relative='scripts/deploy/staging_p3_auxiliary_image.py';path=Path(source)/relative
    encrypted_cut.directory(path.parent)
    if root_cli.public_code_sha(path)!=approved['source_files'].get(relative):
        raise Blocked('staging_prerequisites_auxiliary_source_changed')
    fd,info=encrypted_cut.regular(path)
    try:
        if not 0<info.st_size<=2<<20:raise Blocked('staging_prerequisites_auxiliary_source_changed')
        raw=bytearray()
        while len(raw)<info.st_size:
            chunk=os.read(fd,info.st_size-len(raw))
            if not chunk:raise Blocked('staging_prerequisites_auxiliary_source_changed')
            raw.extend(chunk)
        if os.read(fd,1) or hashlib.sha256(raw).hexdigest()!=approved['source_files'][relative]:
            raise Blocked('staging_prerequisites_auxiliary_source_changed')
    finally:os.close(fd)
    spec=importlib.util.spec_from_file_location('_staging_prerequisites_nginx',path)
    module=importlib.util.module_from_spec(spec)
    # A source digest must bind the executed program, not a timestamp/size pyc.
    exec(compile(bytes(raw),str(path),'exec'),module.__dict__)
    if root_cli.public_code_sha(path)!=approved['source_files'][relative]:
        raise Blocked('staging_prerequisites_auxiliary_source_changed')
    return module


def _node(kube):
    # The selected staging local-path storage contract already fixes this node.
    row=kube.get('node','pmdebook');meta=row['metadata'];info=row.get('status',{}).get('nodeInfo',{})
    if (meta.get('name')!='pmdebook' or not meta.get('uid') or meta.get('deletionTimestamp')
        or info.get('operatingSystem')!='linux' or info.get('architecture')!='amd64'
        or not any(r.get('type')=='Ready' and r.get('status')=='True' for r in row.get('status',{}).get('conditions',[]))):
        raise Blocked('staging_prerequisites_node_changed')
    return {'name':'pmdebook','uid':meta['uid'],'os':'linux','architecture':'amd64'}


def _collect_auxiliary(session,approved,source,source_capture_sha256,kube):
    collector=_source_module(approved,source);session.check()
    node=_node(kube);nonce=secrets.token_hex(16)
    location=_base(session)/'inputs'/'p3-auxiliary';_private_directory(location)
    _private_write(location/'nonce.json',_bytes({'operation':session.operation,'nonce':nonce}))
    raw=collector.collect()
    if type(raw) is not dict or set(raw)!={'index','manifest','config'}:raise Blocked('staging_prerequisites_auxiliary_invalid')
    content=collector.validate(raw['index'],raw['manifest'],raw['config'])
    record={'schema':'voice-staging-p3-auxiliary-image-v1','operation':session.operation,'nonce':nonce,
        'context':session.context,'helper_sha256':session.helper,'source_capture_sha256':source_capture_sha256,
        'source_module_sha256':approved['source_files']['scripts/deploy/staging_p3_auxiliary_image.py'],**content,'node':node}
    if _node(kube)!=node:raise Blocked('staging_prerequisites_node_changed')
    session.check();_verify_helper_source(session,approved,source)
    for name,value in raw.items():_private_write(location/(name+'.json'),value)
    _private_write(location/'record.json',_bytes(record))
    return record


def load_auxiliary_image(session,slot):
    from stage_runtime import Kube
    if slot!='p3':raise Blocked('staging_prerequisites_slot_invalid')
    state=_checkpoint(session);authority=load_authority(session);location=_base(session)/'inputs'/'p3-auxiliary'
    record=_json(_read_private(location/'record.json'));nonce=_json(_read_private(location/'nonce.json'))
    collector=_source_module(authority['approved'],authority['source'])
    raw={name:_read_private(location/(name+'.json'),limit=4<<20) for name in ('index','manifest','config')}
    content=collector.validate(raw['index'],raw['manifest'],raw['config'])
    expected={'schema':'voice-staging-p3-auxiliary-image-v1','operation':session.operation,'nonce':nonce['nonce'],
        'context':session.context,'helper_sha256':session.helper,'source_capture_sha256':state['plan']['source_capture_sha256'],
        'source_module_sha256':authority['approved']['source_files']['scripts/deploy/staging_p3_auxiliary_image.py'],
        **content,'node':_node(Kube())}
    if (nonce!={'operation':session.operation,'nonce':record.get('nonce')} or not re.fullmatch(r'[a-f0-9]{32}',nonce['nonce'])
        or record!=expected or state['plan']['children']['p3']['auxiliary_image']['source_authority_sha256']!=digest(record)):
        raise Blocked('staging_prerequisites_auxiliary_changed')
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    return copy.deepcopy(record)


def _secret(kube,name):
    row=kube.get('secret',name);meta=row['metadata']
    if (row.get('apiVersion')!='v1' or row.get('kind')!='Secret' or meta.get('name')!=name
        or meta.get('namespace')!='voice-staging' or not meta.get('uid') or not meta.get('resourceVersion')
        or meta.get('deletionTimestamp') or type(row.get('data')) is not dict):
        raise Blocked('staging_prerequisites_anchor_invalid')
    return row


def _qualify_issuer_inputs(session,source_capture_sha256,namespace_uid,kube):
    """Qualify operator-provided bytes against actual STAGING public anchors."""
    import staging_p3_prerequisites_crypto as crypto
    root=guard.ROOT/'installed'/'prerequisite-inputs'
    # Read BOTH inputs before any generation or cluster effect.
    raw={slot:_read_private(root/(name+'-issuer.json')) for slot,name in (('gis_material','gis'),('p3','p3'))}
    values={slot:_json(value) for slot,value in raw.items()}
    anchors={name:_secret(kube,name) for name in ('voice-media-ca','voice-space-media-client-tls','voice-space-media-server-tls')}
    decode=lambda name,key:base64.b64decode(anchors[name]['data'][key],validate=True)
    media=decode('voice-media-ca','jwks-ca.crt');server_ca=decode('voice-media-ca','ca.crt')
    gis=values['gis_material'];p3=values['p3']
    if (set(gis)!={'schema','receipt','ca_certificate_pem','ca_private_key_pem','jwks_ca_bundle_pem','ca_secret'}
        or gis['schema']!='voice-staging-gis-issuer-input-v1' or gis['jwks_ca_bundle_pem'].encode('ascii')!=media
        or set(p3)!={'schema','receipt','ca_certificate_pem','ca_private_key_pem','trust',
            'space_client_certificate_pem','space_jwks_certificate_pem','space_server_ca_pem','replay_password_b64'}
        or p3['schema']!='voice-staging-p3-issuer-input-v1'
        or p3['space_client_certificate_pem'].encode('ascii')!=decode('voice-space-media-client-tls','tls.crt')
        or p3['space_jwks_certificate_pem'].encode('ascii')!=decode('voice-space-media-server-tls','tls.crt')
        or p3['space_server_ca_pem'].encode('ascii')!=server_ca):
        raise Blocked('staging_prerequisites_issuer_anchor_changed')
    # Valid chains are not permission to add independent trust roots. Every
    # installed trust bundle is exactly one of the current source-wired anchors.
    expected_trust={name:(server_ca if name=='space-client-ca.crt' else media).decode('ascii')
        for name in crypto.TRUST_KEYS}
    if p3['trust']!=expected_trust:raise Blocked('staging_prerequisites_issuer_trust_changed')
    for slot,item in values.items():
        ca=item['ca_certificate_pem'].encode('ascii');key=item['ca_private_key_pem'].encode('ascii')
        crypto.validate_issuer_key(key,ca)
        crypto._run(['verify','-trusted','{0}','-partial_chain','{1}'],[media,ca])
        # The supplied receipt is data, never a producer authority.
        item['receipt']={'schema':'voice-staging-issuer-qualification-v1','operation':session.operation,
            'namespace_uid':namespace_uid,'source_capture_sha256':source_capture_sha256,
            'input_sha256':hashlib.sha256(raw[slot]).hexdigest(),'anchors_sha256':digest(anchors)}
    gis['ca_secret']=copy.deepcopy(anchors['voice-media-ca'])
    if any(_secret(kube,name)!=row for name,row in anchors.items()):raise Blocked('staging_prerequisites_issuer_anchor_changed')
    session.check()
    _private_write(_base(session)/'inputs'/'issuer-anchors.json',_bytes(anchors))
    for slot,name in (('gis_material','gis'),('p3','p3')):
        _private_write(_base(session)/'inputs'/(name+'-issuer.json'),_bytes(values[slot]))
    return values


def load_issuer(session,slot):
    from stage_runtime import Kube
    if slot not in ('gis_material','p3'):raise Blocked('staging_prerequisites_slot_invalid')
    state=_checkpoint(session);child=require_child(session,slot)
    name='gis' if slot=='gis_material' else 'p3';raw=_read_private(_base(session)/'inputs'/(name+'-issuer.json'))
    issuer=_json(raw);anchors=_json(_read_private(_base(session)/'inputs'/'issuer-anchors.json'))
    wanted=child['issuer']['private_input_sha256'] if slot=='gis_material' else child['issuer_input_sha256']
    receipt_sha=child['issuer']['input_receipt_sha256'] if slot=='gis_material' else child['issuer_authority_sha256']
    receipt=issuer['receipt']
    if (hashlib.sha256(raw).hexdigest()!=wanted or digest(receipt)!=receipt_sha
        or receipt.get('schema')!='voice-staging-issuer-qualification-v1' or receipt.get('operation')!=session.operation
        or receipt.get('namespace_uid')!=state['plan']['namespace_uid']
        or receipt.get('source_capture_sha256')!=state['plan']['source_capture_sha256'] or receipt.get('anchors_sha256')!=digest(anchors)
        or set(anchors)!={'voice-media-ca','voice-space-media-client-tls','voice-space-media-server-tls'}):
        raise Blocked('staging_prerequisites_issuer_changed')
    kube=Kube()
    if any(_secret(kube,name)!=row for name,row in anchors.items()):raise Blocked('staging_prerequisites_issuer_anchor_changed')
    load_authority(session)
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    return copy.deepcopy(issuer)


def _save_payloads(session,slot,payloads):
    directory=_base(session)/'payloads'/slot;_private_directory(directory)
    for effect_id,raw in payloads.items():
        if effect_id not in EFFECT_IDS[slot] or type(raw) is not bytes:raise Blocked('staging_prerequisites_payload_invalid')
        _private_write(directory/(hashlib.sha256(effect_id.encode()).hexdigest()+'.json'),raw)


def _normal_create(kube,obj):
    normalized=kube.run(['create','--dry-run=server','-f','/dev/stdin','-o','json'],body=obj)
    semantic=_semantic(normalized)
    if (normalized.get('kind')!=obj['kind'] or normalized.get('apiVersion')!=obj['apiVersion']
        or semantic['metadata'].get('name')!=obj['metadata']['name'] or semantic['metadata'].get('namespace')!='voice-staging'):
        raise Blocked('staging_prerequisites_normalized_identity_changed')
    return semantic


def _candidate_images(session,request,approved,source,stage,policy,auxiliary,renderer,code,planned_public):
    import bridge_root
    import compiler
    import source_authority
    from stage_runtime import HUB
    images={}
    for name,obj in stage.snapshots.items():
        if name==HUB:continue
        service=name.removeprefix('voice-')
        for container in obj['spec']['template']['spec'].get('initContainers',[])+obj['spec']['template']['spec']['containers']:
            key=name+'/'+container['name'];image=stage.old_images[key]
            if image.split('@',1)[0]==source_authority.REGISTRY+'/'+service:image=approved['images'][service]
            images[key]=image
    images['voice-p3-principal-jwks/jwks']=auxiliary['image_ref']
    first=request.get('gis_first_install') is True
    if first:images['voice-gameintegration/gameintegration']=approved['images']['gameintegration']
    parameters={**policy,'registry':source_authority.REGISTRY,'tag':approved['source_sha'],'mode':request['mode'],
        'changed_services':request['changed_services'],'images':images,'contract':_json((Path(code)/'nats-known-baseline'/'deployed-contract.json').read_bytes()),
        'enrolled':list(stage.snapshots)+(['voice-gameintegration'] if first else [])+['voice-p3-principal-jwks'],
        'generation':stage.marker['data']['generation'],'dataPVC':stage.marker['data']['dataPVC']}
    class Preview:
        # Only this already source/crypto-bound future immutable public object
        # is projected for the read-only image inventory, never for admission.
        def get(self,kind,name):
            if kind.lower()=='configmap' and name=='voice-p3-principal-public':return copy.deepcopy(planned_public)
            return stage.kube.get(kind,name)
        def run(self,*args,**kwargs):return stage.kube.run(*args,**kwargs)
    ordinary=None if first else compiler.prebootstrap_image_inventory(source,parameters,Preview())
    bridge_root.verify_bootstrap_candidate_images(session,images,renderer,first,ordinary)


def prepare_inputs(actions,request,approved,execution,workspace):
    """Prepare immutable, private prerequisite inputs before the writer cut."""
    import commands
    import compiler
    import gis_existing_update
    import realtime_bootstrap
    import renderer_root
    import source_plan
    import staging_gis_material as gis
    import staging_p3_prerequisites as p3
    import staging_p3_prerequisites_crypto as crypto
    import staging_release_policy_prepare as policy_child
    from runtime_stage import RolloutStage
    from stage_runtime import Kube,HUB
    session=actions.tt_session
    if (request.get('mode') not in ('full','app-only') or commands.authority_guard!=session.check
        or Path(workspace)!=guard.ROOT/'installed'/'sources'/session.operation):
        raise Blocked('staging_prerequisites_prepare_invalid')
    source_capture_sha256=_capture_authority(actions,session,approved,execution)
    base=_base(session)
    for name in ('inputs','payloads','attempts'):_private_directory(base/name)
    kube=Kube();namespace=kube.get('namespace','voice-staging');namespace_uid=namespace['metadata']['uid']
    if namespace['metadata'].get('deletionTimestamp'):raise Blocked('staging_prerequisites_namespace_changed')
    # Source contracts, operator policy and BOTH issuer inputs are checked before
    # binding the finite mutation phase. Existing identities are never adopted.
    _,gis_source_sha=gis.source_contract(approved,workspace)
    service_raw=(Path(workspace)/'deploy/staging/services.yaml').read_bytes()
    p3_source_sha=digest(p3.source_contract(approved,service_raw))
    inventory={ident:_observe_named(session,*ident.split('/')) for ident in EFFECT_IDS['p3']}
    existing=any(row is not None for _,row in inventory.values())
    previous=prior_rollout=None
    if existing:
        if any(row is None for _,row in inventory.values()):raise Blocked('staging_prerequisites_partial_inventory')
        active,previous,_,prior_rollout=_previous_pass(namespace_uid,installation=session.installation)
        _private_write(base/'previous-active.json',_bytes(active))
    before={ident:{'absent':True,'list_rv':rv} for ident,(rv,row) in inventory.items() if row is None}
    gis_before={} if existing else {ident:{'absent':True,'list_rv':_absence(session,'Secret',name)} for ident,name in
        (('gis-signer','voice-gameintegration-principal-signing'),('gis-workload','voice-gameintegration-workload-credentials'),
         ('gis-tls','voice-gameintegration-jwks-tls'))}
    app_before=_secret(kube,'voice-app-secrets');app_identity=gis.inspect_app_secret(app_before)
    if app_identity['credentials']!=('present' if existing else 'absent'):raise Blocked('staging_prerequisites_existing_authority_required')
    if not existing:gis_before['gis-app-credentials']={key:app_identity[key] for key in ('uid','resourceVersion','data_sha256')}
    old_policy=_read_private(guard.ROOT/'installed'/'policy.json');old_policy_value=_json(old_policy)
    descriptor=policy_child.staging_descriptor(approved['source_sha'])
    policy_child.validate_source(approved,workspace,descriptor)
    policy_rv,policy_live=_observe_named(session,'ConfigMap',descriptor['policy_object']['metadata']['name'])
    if policy_live is not None:
        if not existing:raise Blocked('staging_prerequisites_existing_authority_required')
        preserved={}
        for ident in EFFECT_IDS['release_policy']:
            qualified,observation=_qualified_preserved(session,'release_policy',ident,previous,prior_rollout,approved)
            preserved[ident]={'authority':qualified['authority'],'observed':observation}
        policy_plan,policy_payloads=policy_child.build_preserved_plan(approved,workspace,old_policy,descriptor,operation=session.operation,
            namespace_uid=namespace_uid,source_capture_sha256=source_capture_sha256,preserved=preserved)
    else:
        policy_plan,policy_payloads=policy_child.plan(approved,workspace,old_policy,descriptor,operation=session.operation,
            namespace_uid=namespace_uid,source_capture_sha256=source_capture_sha256,list_rv=policy_rv)
    _private_write(base/'policy-before.json',old_policy)
    binding=None if request.get('gis_first_install') is True else gis_existing_update.load_authority(kube)
    gis_active=None
    if binding is not None:
        gis_active=_json(_read_private(guard.ROOT/'installed'/'active-release.json'))
        if gis_active.get('gis_first_install_transition')!={'operation':binding['operation'],'receipt_sha256':binding['anchor_sha256']}:
            raise Blocked('staging_prerequisites_gis_active_changed')
    gis_authority={'binding':copy.deepcopy(binding),'active':copy.deepcopy(gis_active)}
    _private_write(base/'gis-authority.json',_bytes(gis_authority))
    extras=['voice-web','voice-admin','voice-developer-portal']+([HUB] if request['mode']=='full' else [])
    if binding is not None:extras.append('voice-gameintegration')
    if existing:extras.append('voice-p3-principal-jwks')
    stage=RolloutStage.capture(kube,session.operation,lambda _:None,extras,**({'gis_binding':binding} if binding is not None else {}))
    baseline={'expected':stage.expected,'workloads':realtime_bootstrap._stage_view(stage),'original_snapshots':stage.original_snapshots,
        'storage':{'claim':stage.final_claim,'pv':stage.final_pv,'path':str(stage.final_path)}}
    _private_write(base/'baseline.json',_bytes(baseline))
    renderer=None
    if request['mode']=='full':
        renderer=renderer_root.prove(kube,stage,approved['source_sha'],approved['images']['nats-hub-config-renderer'],installation=session.installation)
        _private_write(base/'renderer-authority.json',_bytes(renderer))
    issuers=_qualify_issuer_inputs(session,source_capture_sha256,namespace_uid,kube)
    auxiliary=_collect_auxiliary(session,approved,workspace,source_capture_sha256,kube)
    gi=issuers['gis_material'];raw_gi=_read_private(base/'inputs'/'gis-issuer.json')
    gis_preserved={}
    if existing:
        for ident in EFFECT_IDS['gis_material']:
            qualified,observation=_qualified_preserved(session,'gis_material',ident,previous,prior_rollout,approved)
            gis_preserved[ident]={'authority':qualified['authority'],'observed':observation}
        gis_payloads={}
    else:
        gis_payloads=gis.build_create_payloads(session.operation,app_before,gi['ca_certificate_pem'].encode('ascii'),
            gi['ca_private_key_pem'].encode('ascii'),gi['jwks_ca_bundle_pem'].encode('ascii'))
    issuer={'input_receipt_sha256':digest(gi['receipt']),'ca_secret_uid':gi['ca_secret']['metadata']['uid'],
        'ca_public_sha256':hashlib.sha256(gi['jwks_ca_bundle_pem'].encode('ascii')).hexdigest(),
        'issuer_certificate_sha256':hashlib.sha256(gi['ca_certificate_pem'].encode('ascii')).hexdigest(),
        'private_input_sha256':hashlib.sha256(raw_gi).hexdigest()}
    gis_plan,gis_payloads=gis.build_plan(session.operation,namespace_uid,source_capture_sha256,gis_source_sha,issuer,gis_before,gis_preserved,gis_payloads)
    pi=issuers['p3']
    if not existing:
        values=crypto.issue_private_bytes(pi['ca_private_key_pem'].encode('ascii'),pi['ca_certificate_pem'].encode('ascii'),
            {k:v.encode('ascii') for k,v in pi['trust'].items()},pi['space_client_certificate_pem'].encode('ascii'),
            pi['space_jwks_certificate_pem'].encode('ascii'),pi['space_server_ca_pem'].encode('ascii'),base64.b64decode(pi['replay_password_b64'],validate=True))
        crypto.validate_issuer_materials(values,pi)
    rows=[compiler.decode_yaml(piece.encode()) for piece in re.split(r'^---\s*$',service_raw.decode(),flags=re.M)
          if re.search(r'^  name: (?:voice-p3-principal-jwks|voice-p3-principal-publisher-config)\s*$',piece,re.M)]
    publisher=[row for row in rows if row.get('metadata',{}).get('name') in ('voice-p3-principal-jwks','voice-p3-principal-publisher-config')]
    objects=[] if existing else p3.mark_operation(p3.material_objects(values)+p3.publisher_objects(publisher,auxiliary['image_ref'],'sha256:'+auxiliary['manifest_sha256']),session.operation)
    normalized=[];p3_preserved={}
    if existing:
        for ident in EFFECT_IDS['p3']:
            qualified,observation=_qualified_preserved(session,'p3',ident,previous,prior_rollout,approved)
            p3_preserved[ident]={'authority':qualified['authority'],'observed':observation};normalized.append(qualified['semantic'])
    for obj in objects:
        value=_normal_create(kube,obj)
        if value['kind']=='Service':
            # Only the source-absent Service allocations are deferred to CREATE.
            if any(k in obj['spec'] for k in ('clusterIP','clusterIPs')):raise Blocked('staging_prerequisites_service_allocation_invalid')
            value['spec'].pop('clusterIP',None);value['spec'].pop('clusterIPs',None)
        normalized.append(value)
    p3_payloads={obj['kind']+'/'+obj['metadata']['name']:_bytes(obj) for obj in normalized}
    aux={key:auxiliary[key] for key in ('source_ref','image_ref','platform','manifest_sha256','config_sha256','index_sha256')}
    aux.update(workload='voice-p3-principal-jwks',container='jwks',source_authority_sha256=digest(auxiliary))
    object_rows=[{'id':ident,'kind':ident.split('/')[0],'name':ident.split('/')[1],'namespace':'voice-staging',
        'desired_sha256':hashlib.sha256(p3_payloads[ident]).hexdigest(),'payload_sha256':hashlib.sha256(p3_payloads[ident]).hexdigest()} for ident in EFFECT_IDS['p3']]
    p3_plan={'schema':'voice-staging-p3-prerequisites-plan-v1','operation':session.operation,'namespace_uid':namespace_uid,
        'source_capture_sha256':source_capture_sha256,'source_contract_sha256':p3_source_sha,
        'issuer_authority_sha256':digest(pi['receipt']),'issuer_input_sha256':hashlib.sha256(_read_private(base/'inputs'/'p3-issuer.json')).hexdigest(),
        'auxiliary_image':aux,'objects':object_rows,'effects':[{**row,'before':before[row['id']]} for row in object_rows if row['id'] in before],
        'preserved':p3_preserved}
    p3.validate_plan_shape(p3_plan)
    public=next(row for row in normalized if row['metadata']['name']=='voice-p3-principal-public')
    _candidate_images(session,request,approved,workspace,stage,policy_child.policy_successor(old_policy_value,descriptor,approved['source_sha']),auxiliary,renderer,actions.code,public)
    plan={'schema':'voice-staging-prerequisites-plan-v3','installation_sha256':session.installation.sha256,
        'operation':session.operation,'context':session.context,'helper_sha256':session.helper,
        'source_capture_sha256':source_capture_sha256,'approved_sha256':digest(approved),'execution_sha256':digest(execution),
        'namespace_uid':namespace_uid,'marker_sha256':digest(stage.marker),'baseline_sha256':digest(baseline),'actual_before':stage.old_images,
        'policy_before_sha256':hashlib.sha256(old_policy).hexdigest(),'children':{'release_policy':policy_plan,'p3':p3_plan,'gis_material':gis_plan},
        'auxiliary_images':{'voice-p3-principal-jwks/jwks':auxiliary['image_ref']},'renderer_authority_sha256':None if renderer is None else digest(renderer),
        'previous_active_sha256':digest(active) if existing else None}
    for slot,payloads in (('release_policy',policy_payloads),('p3',{} if existing else p3_payloads),('gis_material',gis_payloads)):_save_payloads(session,slot,payloads)
    _recheck_baseline(session,{'plan':plan});session.check()
    prepared={'schema':'voice-staging-prerequisites-prepared-v1','plan':plan,'gis_authority_sha256':digest(gis_authority),
              'request':{'mode':request['mode'],'changed_services':copy.deepcopy(request['changed_services']),
                         'gis_first_install':request.get('gis_first_install') is True}}
    _private_write(base/'prepared.json',_bytes(prepared))
    session.check()
    return {'stage':stage,'inputs_sha256':digest(prepared)}


def _bind_auxiliary_successor(session,plan):
    import staging_cohort_preservation
    auxiliary=staging_cohort_preservation.load_auxiliary_successor(session)
    publisher=plan['children']['p3'].get('preserved',{}).get('Deployment/voice-p3-principal-jwks')
    if auxiliary is not None:
        if publisher is None:raise Blocked('staging_prerequisites_auxiliary_changed')
        observed=publisher['observed'];response=auxiliary['response'];meta=response['metadata']
        if (meta.get('uid')!=observed.get('uid') or type(meta.get('generation')) is not int
            or not isinstance(meta.get('resourceVersion'),str) or not meta['resourceVersion']
            or digest(_semantic(response))!=observed.get('semantic_sha256')):
            raise Blocked('staging_prerequisites_auxiliary_changed')
        observed['generation']=meta['generation'];observed['resourceVersion']=meta['resourceVersion']
    elif publisher is not None:
        raise Blocked('staging_prerequisites_auxiliary_missing')


def run_prepare(actions,request,approved,execution,workspace,*,preservation_ref):
    """Continue only from the root-loaded, restore-qualified preservation receipt."""
    import commands
    import staging_cohort_preservation
    import staging_p3_prerequisites as p3
    session=actions.tt_session
    if (commands.authority_guard!=session.check or Path(workspace)!=guard.ROOT/'installed'/'sources'/session.operation):
        raise Blocked('staging_prerequisites_prepare_invalid')
    base=_base(session)
    receipt=staging_cohort_preservation.load_qualified(session)
    if receipt!=preservation_ref:raise Blocked('staging_prerequisites_preservation_changed')
    prepared=_json(_read_private(_base(session)/'prepared.json',limit=32<<20))
    if (type(prepared) is not dict or set(prepared)!={'schema','plan','request','gis_authority_sha256'}
        or prepared['schema']!='voice-staging-prerequisites-prepared-v1'
        or digest(prepared)!=receipt.get('prerequisite_inputs_sha256')):
        raise Blocked('staging_prerequisites_prepared_changed')
    plan=copy.deepcopy(prepared['plan'])
    if (plan['approved_sha256']!=digest(approved) or plan['execution_sha256']!=digest(execution)
        or request.get('mode') not in ('full','app-only')
        or prepared['request']!={'mode':request.get('mode'),'changed_services':request.get('changed_services'),
                                  'gis_first_install':request.get('gis_first_install') is True}):
        raise Blocked('staging_prerequisites_prepared_changed')
    plan['schema']='voice-staging-prerequisites-plan-v4';plan['preservation_sha256']=digest(receipt)
    _bind_auxiliary_successor(session,plan)
    stage=staging_cohort_preservation.capture_stage(session,receipt)
    _recheck_baseline(session,{'plan':plan});session.check()
    fence=session.bind_prerequisites(plan,stage.old_images)
    save(base/'checkpoint.json',{'schema':'voice-staging-prerequisites-checkpoint-v1','plan':plan,'initial_resource_fence':fence,
        'status':'RUNNING','steps':{},'receipt':None,'preflight_sha256':None})
    preflight_all(session)
    # Readiness is a fixed child implementation, not a supplied callback.
    if not callable(getattr(p3,'verify_complete',None)):raise Blocked('staging_prerequisites_publisher_verifier_missing')
    for slot in SLOTS:importlib.import_module(CHILD_MODULES[slot]).execute(session)
    return _complete(session)


def load_gis_authority(session):
    """Load only the pre-cut GIS authority bound to this preservation receipt."""
    import staging_cohort_preservation
    import gis_existing_update
    from stage_runtime import Kube
    receipt=staging_cohort_preservation.load_qualified(session)
    prepared=_json(_read_private(_base(session)/'prepared.json',limit=32<<20))
    record=_json(_read_private(_base(session)/'gis-authority.json',limit=32<<20))
    if (type(prepared) is not dict or set(prepared)!={'schema','plan','request','gis_authority_sha256'}
        or digest(prepared)!=receipt.get('prerequisite_inputs_sha256') or type(record) is not dict
        or set(record)!={'binding','active'} or digest(record)!=prepared['gis_authority_sha256']):
        raise Blocked('staging_prerequisites_gis_authority_changed')
    if record['binding'] is None:
        if record['active'] is not None:raise Blocked('staging_prerequisites_gis_authority_changed')
    else:
        current=_json(_read_private(guard.ROOT/'installed'/'active-release.json'))
        if current!=record['active'] or current.get('gis_first_install_transition')!={
            'operation':record['binding']['operation'],'receipt_sha256':record['binding']['anchor_sha256']}:
            raise Blocked('staging_prerequisites_gis_active_changed')
        gis_existing_update.verify_phase(record['binding'],Kube(),'paused-old')
    session.check()
    return copy.deepcopy(record['binding'])


def _verify_children(session,state):
    expected=_effect_order(state['plan'])
    actual={(slot,ident) for slot,steps in state['steps'].items() for ident in steps}
    if actual!=set(expected):raise Blocked('staging_prerequisites_incomplete')
    children={}
    for slot in SLOTS:
        module=importlib.import_module(CHILD_MODULES[slot]);observations={}
        for effect_slot,ident in expected:
            if effect_slot!=slot:continue
            step=state['steps'][slot][ident]
            if set(step)!={'status','observation'} or step['status']!='DONE':raise Blocked('staging_prerequisites_incomplete')
            if slot=='gis_material' and ident=='gis-app-credentials' and _realtime_app_extension(session,state):
                observations[ident]=step['observation']
                continue
            verified=module.verify_effect(session,ident,step['observation'])
            if verified!=step['observation']:raise Blocked('staging_prerequisites_observation_changed')
            observations[ident]=verified
        for ident,value in state['plan']['children'][slot].get('preserved',{}).items():
            if slot=='p3':
                load_preserved(session,slot,ident)
                observations[ident]=copy.deepcopy(value['observed'])
            else:observations[ident]=module.verify_effect(session,ident,value['observed'])
        children[slot]=observations
    return children


def _realtime_app_extension(session,state):
    """Recognize only the completed, response-bound additive Realtime CAS.

    This is local custody validation, not a recursive SQL/parent admission.
    PENDING, even with the desired key live, never grants the exception.
    """
    import realtime_bootstrap as rt
    from stage_runtime import Kube
    base=guard.ROOT/'installed'/'realtime-bootstrap'/session.operation
    try:checkpoint=_json(_read_private(base/'checkpoint.json',limit=32<<20))
    except FileNotFoundError:return False
    if checkpoint.get('status')!='COMPLETE':return False
    plan,receipt,_,_=rt._records(checkpoint,session)
    if checkpoint['resource_fence']['schema']!=session.binding()['schema']:
        raise Blocked('staging_prerequisites_realtime_custody_changed')
    captured=_json(_read_private(_base(session)/'capture.json',limit=32<<20))
    before=_json(_read_private(base/'before-secret.json'));credential=_read_private(base/'credential',limit=128)
    payload=_json(load_payload(session,'gis_material','gis-app-credentials'))
    desired=_semantic(payload['before']);desired['data']=payload['after_data']
    original=state['steps']['gis_material']['gis-app-credentials']['observation']
    if (plan.get('schema') not in ('voice-staging-realtime-bootstrap-v2','voice-staging-realtime-bootstrap-v4') or plan.get('prerequisites_sha256')!=digest(captured)
        or _semantic(before)!=desired or rt._secret_identity(before)!=plan['secret']
        or any(before['metadata'].get(key)!=original[key] for key in ('uid','resourceVersion'))
        or hashlib.sha256(credential).hexdigest()!=plan['credential_sha256']):
        raise Blocked('staging_prerequisites_realtime_custody_changed')
    actual=_secret(Kube(),'voice-app-secrets')
    if rt._verify_secret_after(before,actual,credential)!=receipt['secret']:
        raise Blocked('staging_prerequisites_realtime_secret_changed')
    authority=_json(_read_private(base/'authority.json'))
    rt._verify_source_capture(session,authority['approved'],authority['execution'])
    if (digest(authority['approved'])!=plan['source_authority_sha256']
        or digest(authority['execution'])!=plan['execution_sha256']):raise Blocked('staging_prerequisites_realtime_custody_changed')
    return True


def _complete(session):
    import staging_p3_prerequisites as p3
    state=_checkpoint(session)
    if state['status']!='RUNNING':raise Blocked('staging_prerequisites_reconciliation_required')
    children=_verify_children(session,state);ready=p3.verify_complete(session)
    if (type(ready) is not dict or set(ready)!={'receipt','proof'}
        or ready['receipt'].get('publisher_ready_sha256')!=digest(ready['proof'])
        or ready['receipt'].get('operation')!=session.operation
        or ready['receipt'].get('plan_sha256')!=digest(state['plan']['children']['p3'])):
        raise Blocked('staging_prerequisites_publisher_proof_invalid')
    children['p3']=ready['receipt']
    _private_write(_base(session)/'publisher-ready.json',_bytes(ready))
    plan=state['plan'];actual_after={**plan['actual_before'],**plan['auxiliary_images']}
    receipt={'schema':'voice-staging-prerequisites-receipt-v1','operation':session.operation,'plan_sha256':digest(plan),
        'steps_sha256':digest(state['steps']),'namespace_uid':plan['namespace_uid'],'children_receipts':children,
        'actual_before':plan['actual_before'],'actual_after':actual_after,
        'publisher_identity':_publisher_identity(state),
        'policy_after_sha256':plan['children']['release_policy']['policy_after_sha256']}
    _recheck_baseline(session,state)
    after=copy.deepcopy(state);after.update(status='COMPLETE',receipt=receipt)
    _store_state(session,state,after)
    return copy.deepcopy(receipt)


def _publisher_identity(state):
    ident='Deployment/voice-p3-principal-jwks'
    step=state['steps'].get('p3',{}).get(ident)
    if step is not None:return copy.deepcopy(step['observation'])
    return copy.deepcopy(state['plan']['children']['p3']['preserved'][ident]['observed'])


def _completed(session):
    state=_checkpoint(session);plan=state['plan'];receipt=state['receipt']
    if (state['status']!='COMPLETE' or type(receipt) is not dict
        or set(receipt)!={'schema','operation','plan_sha256','steps_sha256','namespace_uid','children_receipts',
            'actual_before','actual_after','publisher_identity','policy_after_sha256'}
        or receipt['schema']!='voice-staging-prerequisites-receipt-v1' or receipt['operation']!=session.operation
        or receipt['plan_sha256']!=digest(plan) or receipt['steps_sha256']!=digest(state['steps'])
        or receipt['namespace_uid']!=plan['namespace_uid'] or receipt['actual_before']!=plan['actual_before']
        or receipt['actual_after']!={**plan['actual_before'],**plan['auxiliary_images']}):
        raise Blocked('staging_prerequisites_receipt_changed')
    children=_verify_children(session,state)
    original=_json(_read_private(_base(session)/'publisher-ready.json',limit=8<<20))
    if (set(original)!={'receipt','proof'} or original['receipt'].get('publisher_ready_sha256')!=digest(original['proof'])
        or original['receipt'].get('operation')!=session.operation or original['receipt'].get('plan_sha256')!=digest(plan['children']['p3'])
        or receipt['children_receipts']!={**children,'p3':original['receipt']}
        or receipt['publisher_identity']!=_publisher_identity(state)
        or receipt['policy_after_sha256']!=plan['children']['release_policy']['policy_after_sha256']):
        raise Blocked('staging_prerequisites_receipt_changed')
    import staging_p3_prerequisites as p3
    fresh=p3.verify_complete(session)
    if (fresh['receipt'].get('plan_sha256')!=digest(plan['children']['p3'])
        or fresh['receipt'].get('publisher_ready_sha256')!=digest(fresh['proof'])):
        raise Blocked('staging_prerequisites_publisher_proof_invalid')
    _recheck_baseline(session,state);load_authority(session)
    if _checkpoint(session)!=state:raise Blocked('staging_prerequisites_checkpoint_changed')
    return state


def capture(session,stage):
    import realtime_bootstrap
    from runtime_stage import RolloutStage
    if type(stage) is not RolloutStage or stage.operation!=session.operation:
        raise Blocked('staging_prerequisites_capture_invalid')
    state=_completed(session);receipt=state['receipt'];plan=state['plan']
    view=realtime_bootstrap._stage_view(stage)
    if plan['schema'] in ('voice-staging-prerequisites-plan-v2','voice-staging-prerequisites-plan-v4'):
        import staging_cohort_preservation
        protected=staging_cohort_preservation.capture_final_stage(session)
        if (stage.expected!=protected.expected or stage.old_images!=protected.old_images
            or stage.original_snapshots!=protected.original_snapshots or view!=realtime_bootstrap._stage_view(protected)):
            raise Blocked('staging_prerequisites_capture_changed')
    else:
        baseline=_json(_read_private(_base(session)/'baseline.json',limit=32<<20))
        expected=copy.deepcopy(baseline['expected'])
        publisher=receipt['publisher_identity'];expected['deployment_uids']['voice-p3-principal-jwks']=publisher['uid']
        if stage.expected!=expected or stage.old_images!=receipt['actual_after']:
            raise Blocked('staging_prerequisites_capture_changed')
        if any(view['snapshots'].get(name)!=row for name,row in baseline['workloads']['snapshots'].items()):
            raise Blocked('staging_prerequisites_capture_changed')
        if view['marker']!=baseline['workloads']['marker']:raise Blocked('staging_prerequisites_marker_changed')
    if stage.old_images!=receipt['actual_after']:raise Blocked('staging_prerequisites_capture_changed')
    record={'schema':'voice-staging-prerequisites-capture-v1','operation':session.operation,'context':session.context,
        'helper_sha256':session.helper,'source_capture_sha256':plan['source_capture_sha256'],'plan_sha256':digest(plan),
        'receipt_sha256':digest(receipt),'namespace_uid':plan['namespace_uid'],'actual_before':receipt['actual_before'],
        'actual_after':receipt['actual_after'],'stage':{'expected':stage.expected,'workloads':view,
            'original_snapshots_sha256':digest(stage.original_snapshots)}}
    _private_write(_base(session)/'capture-snapshots.json',_bytes(stage.original_snapshots))
    _private_write(_base(session)/'capture.json',_bytes(record))
    return record


def load_capture(session,*,actual=None,rollout_target=None):
    import realtime_bootstrap
    from stage_runtime import Kube
    if rollout_target is not None:_target_checks[session]=copy.deepcopy(rollout_target)
    try:
        state=_completed(session);plan=state['plan'];record=_json(_read_private(_base(session)/'capture.json',limit=32<<20))
        if (set(record)!={'schema','operation','context','helper_sha256','source_capture_sha256','plan_sha256','receipt_sha256',
                'namespace_uid','actual_before','actual_after','stage'}
            or record['schema']!='voice-staging-prerequisites-capture-v1' or record['operation']!=session.operation
            or record['context']!=session.context or record['helper_sha256']!=session.helper
            or record['source_capture_sha256']!=plan['source_capture_sha256'] or record['plan_sha256']!=digest(plan)
            or record['receipt_sha256']!=digest(state['receipt']) or record['namespace_uid']!=plan['namespace_uid']
            or record['actual_before']!=plan['actual_before'] or record['actual_after']!=state['receipt']['actual_after']
            or actual is not None and actual!=record['actual_after']
            or rollout_target is not None and rollout_target.get('prerequisites_sha256')!=digest(record)
            or digest(_json(_read_private(_base(session)/'capture-snapshots.json',limit=32<<20)))!=record['stage']['original_snapshots_sha256']):
            raise Blocked('staging_prerequisites_capture_changed')
        if plan['schema'] in ('voice-staging-prerequisites-plan-v2','voice-staging-prerequisites-plan-v4'):
            import staging_cohort_preservation
            protected=staging_cohort_preservation.capture_final_stage(session)
            if (record['stage']['expected']!=protected.expected or record['actual_after']!=protected.old_images
                or record['stage']['workloads']!=realtime_bootstrap._stage_view(protected)
                or record['stage']['original_snapshots_sha256']!=digest(protected.original_snapshots)):
                raise Blocked('staging_prerequisites_capture_changed')
        realtime_bootstrap._check_stage(Kube(),record['stage']['workloads'])
        session.check()
        return record
    finally:_target_checks.pop(session,None)


def load_renderer(session,stage):
    import renderer_root
    state=_checkpoint(session);wanted=state['plan']['renderer_authority_sha256']
    if wanted is None:return None
    authority=_json(_read_private(_base(session)/'renderer-authority.json',limit=8<<20))
    if digest(authority)!=wanted:raise Blocked('staging_prerequisites_renderer_changed')
    renderer_root.revalidate(stage.kube,stage,authority,installation=session.installation)
    session.check()
    return authority


def _historical(operation,seen,*,installation=None):
    installation=resource_claim.require_installation(installation)
    if not isinstance(operation,str) or not OP.fullmatch(operation) or operation in seen or len(seen)>=64:
        raise Blocked('staging_prerequisites_history_invalid')
    seen=seen|{operation};base=guard.ROOT/'installed'/'prerequisites'/operation
    rollout=_json(_read_private(guard.ROOT/('rollout-'+operation)/'checkpoint.json',limit=128<<20))
    if (rollout.get('status')!='PASS' or rollout.get('phase')!='POST_RESTART' or rollout.get('fence_status')!='RELEASED'
        or rollout.get('operation')!=operation):raise Blocked('staging_prerequisites_history_not_pass')
    binding=resource_claim.historical_binding(rollout,installation=installation)
    state=_json(_read_private(base/'checkpoint.json',limit=32<<20));plan=state['plan'];receipt=state['receipt']
    record=_json(_read_private(base/'capture.json',limit=32<<20))
    source=_json(_read_private(guard.ROOT/'installed'/'source-authority'/(operation+'.json')))
    approved=_json(_read_private(base/'approved.json',limit=32<<20));execution=_json(_read_private(base/'execution.json',limit=8<<20))
    import staging_cohort_preservation
    try:
        staging_cohort_preservation._validate_historical_source_receipt(binding,source,approved,execution,installation=installation)
        source_valid=True
    except Blocked:
        source_valid=False
    v2=plan.get('schema') in ('voice-staging-prerequisites-plan-v2','voice-staging-prerequisites-plan-v4')
    try:
        resource_claim.record_fields(plan,'prerequisites_plan',PLAN_FIELDS|({'preservation_sha256'} if v2 else set()),installation=installation)
    except (RuntimeError,ValueError):raise Blocked('staging_prerequisites_history_changed') from None
    if v2:
        preserved=staging_cohort_preservation.validate_historical_capture(operation,installation=installation)
        if plan.get('preservation_sha256')!=digest(preserved):raise Blocked('staging_prerequisites_history_preservation_changed')
    if (state.get('status')!='COMPLETE' or state.get('preflight_sha256')!=digest(plan)
        or set(state)!={'schema','plan','initial_resource_fence','status','steps','receipt','preflight_sha256'}
        or state['schema']!='voice-staging-prerequisites-checkpoint-v1'
        or state['initial_resource_fence']!={**binding,'target_sha256':digest(plan)}
        or plan['operation']!=operation or plan['context']!=binding['context'] or plan['helper_sha256']!=binding['helper_sha256']
        or approved.get('source_sha')!=binding['context']['commit']
        or plan['source_capture_sha256']!=digest(source) or plan['approved_sha256']!=digest(approved)
        or plan['execution_sha256']!=digest(execution)
        or not source_valid
        or record!=rollout.get('prerequisites') or rollout['target'].get('prerequisites_sha256')!=digest(record)
        or record.get('plan_sha256')!=digest(plan) or record.get('receipt_sha256')!=digest(receipt)
        or receipt.get('operation')!=operation or receipt.get('plan_sha256')!=digest(plan)
        or receipt.get('steps_sha256')!=digest(state['steps']) or receipt.get('namespace_uid')!=plan['namespace_uid']
        or record.get('actual_before')!=plan['actual_before']
        or record.get('actual_after')!={**plan['actual_before'],**plan['auxiliary_images']}
        or record.get('operation')!=operation or record.get('context')!=plan['context'] or record.get('helper_sha256')!=plan['helper_sha256']
        or record.get('source_capture_sha256')!=plan['source_capture_sha256'] or record.get('namespace_uid')!=plan['namespace_uid']
        or rollout['context']['expected']['namespace_uid']!=plan['namespace_uid']
        or receipt.get('actual_before')!=record['actual_before'] or receipt.get('actual_after')!=record['actual_after']
        or receipt.get('schema')!='voice-staging-prerequisites-receipt-v1'
        or receipt.get('publisher_identity')!=_publisher_identity(state)
        or receipt.get('policy_after_sha256')!=plan['children']['release_policy']['policy_after_sha256']
        or digest(_json(_read_private(base/'capture-snapshots.json',limit=32<<20)))!=record['stage']['original_snapshots_sha256']):
        raise Blocked('staging_prerequisites_history_changed')
    ready=_json(_read_private(base/'publisher-ready.json',limit=8<<20))
    baseline=_json(_read_private(base/'baseline.json',limit=32<<20))
    if (set(ready)!={'receipt','proof'} or ready['receipt']!=receipt['children_receipts']['p3']
        or ready['receipt'].get('operation')!=operation or ready['receipt'].get('plan_sha256')!=digest(plan['children']['p3'])
        or ready['receipt'].get('publisher_ready_sha256')!=digest(ready['proof'])
        or digest(baseline)!=plan['baseline_sha256'] or digest(baseline['workloads']['marker'])!=plan['marker_sha256']
        or baseline['expected']['namespace_uid']!=plan['namespace_uid']
        or baseline['expected']['source_claim_uid']!=rollout['context']['expected']['source_claim_uid']):
        raise Blocked('staging_prerequisites_history_changed')
    manifest=_json(_read_private(base/'code-custody.json'))['manifest']
    if digest(manifest)!=binding['helper_sha256'] or not REQUIRED_HELPERS<=set(manifest):
        raise Blocked('staging_prerequisites_history_helper_changed')
    # Historical helpers are checked as source data; never execute old code.
    for relative,wanted in manifest.items():
        if relative in P2_BINARY_PINS:
            if wanted!=P2_BINARY_PINS[relative]:raise Blocked('staging_prerequisites_history_helper_changed')
        elif approved['source_files'].get('scripts/staging/'+relative)!=wanted:
            raise Blocked('staging_prerequisites_history_source_changed')
    expected=set(_effect_order(plan))
    if {(slot,ident) for slot,steps in state['steps'].items() for ident in steps}!=expected:
        raise Blocked('staging_prerequisites_history_incomplete')
    if set(receipt['children_receipts'])!=set(SLOTS):raise Blocked('staging_prerequisites_history_incomplete')
    for slot in SLOTS:
        observations={ident:step['observation'] for ident,step in state['steps'].get(slot,{}).items()}
        observations.update({ident:value['observed'] for ident,value in plan['children'][slot].get('preserved',{}).items()})
        if slot!='p3':
            if receipt['children_receipts'][slot]!=observations:raise Blocked('staging_prerequisites_history_receipt_changed')
        else:
            stored=ready['receipt'].get('objects',{})
            if set(stored)!=set(EFFECT_IDS['p3']) or set(stored)!=set(observations):
                raise Blocked('staging_prerequisites_history_receipt_changed')
            for ident,value in observations.items():
                keys=set(value)-({'resourceVersion'} if value.get('kind')=='Deployment' else set())
                if set(stored[ident])!=set(value) or any(stored[ident][key]!=value[key] for key in keys):
                    raise Blocked('staging_prerequisites_history_receipt_changed')
    for slot,ident in expected:
        step=state['steps'][slot][ident];effect=_selected_effect(state,slot,ident)
        raw=_read_private(base/'payloads'/slot/(hashlib.sha256(ident.encode()).hexdigest()+'.json'))
        key=hashlib.sha256((slot+'/'+ident).encode()).hexdigest()
        intent=_json(_read_private(base/'attempts'/(key+'.json')));result=_json(_read_private(base/'attempts'/(key+'.response.json')))
        if (step.get('status')!='DONE' or set(step)!={'status','observation'}
            or hashlib.sha256(raw).hexdigest()!=effect['payload_sha256'] or intent.get('plan_sha256')!=digest(plan)
            or intent.get('operation')!=operation or intent.get('slot')!=slot or intent.get('effect_id')!=ident
            or intent.get('payload_sha256')!=effect['payload_sha256'] or result.get('intent_sha256')!=digest(intent)
            or result.get('observation')!=step['observation']):raise Blocked('staging_prerequisites_history_effect_changed')
        payload=_json(raw);observation=step['observation'];root_policy=slot=='release_policy' and ident=='commit-root-policy'
        app=slot=='gis_material' and ident=='gis-app-credentials'
        if root_policy:
            if (result['object']!=payload or observation!={'kind':'RootPolicy','name':'installed-policy','sha256':hashlib.sha256(raw).hexdigest()}
                or digest(payload)!=effect['desired_sha256'] or intent['argv'] is not None or intent['body_sha256']!=hashlib.sha256(raw).hexdigest()):
                raise Blocked('staging_prerequisites_history_effect_changed')
        else:
            desired=_semantic(payload['before'] if app else payload)
            body=raw;argv=KUBE_PREFIX+['create','-f','/dev/stdin','-o','json']
            if app:
                before=payload['before'];old=before['data'];new=payload['after_data'];keys={'GAME_INTEGRATION_DB_PASSWORD','GAME_INTEGRATION_DATABASE_URL'}
                if set(old)&keys or set(new)!=set(old)|keys or any(new[k]!=v for k,v in old.items()):raise Blocked('staging_prerequisites_history_effect_changed')
                desired['data']=new
                patch=[{'op':'test','path':'/metadata/uid','value':before['metadata']['uid']},
                    {'op':'test','path':'/metadata/resourceVersion','value':before['metadata']['resourceVersion']},{'op':'test','path':'/data','value':old}]
                patch.extend({'op':'add','path':'/data/'+k,'value':new[k]} for k in sorted(keys));body=_bytes(patch)
                argv=KUBE_PREFIX+['patch','secret','voice-app-secrets','--type=json','--patch-file=/dev/stdin','-o','json']
            actual=result['object'];semantic=_semantic(actual);meta=actual['metadata']
            expected={'kind':effect['kind'],'name':effect['name'],'namespace':'voice-staging','uid':meta['uid'],
                'resourceVersion':meta['resourceVersion'],'semantic_sha256':digest(semantic)}
            if effect['kind']=='Deployment':expected['generation']=meta['generation']
            if effect['kind']=='Service':
                import staging_p3_prerequisites
                staging_p3_prerequisites.service_allocation(desired,actual)
            elif semantic!=desired:raise Blocked('staging_prerequisites_history_effect_changed')
            if (digest(desired)!=effect['desired_sha256'] or observation!=expected
                or intent['argv']!=argv or intent['body_sha256']!=hashlib.sha256(body).hexdigest()
                or app and meta['uid']!=payload['before']['metadata']['uid']):
                raise Blocked('staging_prerequisites_history_effect_changed')
    previous=None
    if plan['previous_active_sha256'] is not None:
        active=_json(_read_private(base/'previous-active.json'))
        if digest(active)!=plan['previous_active_sha256']:raise Blocked('staging_prerequisites_history_predecessor_changed')
        previous,_,prior_rollout=_historical(active.get('operation'),seen,installation=installation)
        expected=prior_rollout['context']['expected']
        if (active.get('namespace_uid')!=plan['namespace_uid'] or expected['namespace_uid']!=plan['namespace_uid']
            or active.get('pvc_uid')!=expected['source_claim_uid'] or active.get('target_sha256')!=prior_rollout['target']['manifest_sha256']):
            raise Blocked('staging_prerequisites_history_predecessor_changed')
    for slot in SLOTS:
        for ident,preserved in plan['children'][slot].get('preserved',{}).items():
            if previous is None:raise Blocked('staging_prerequisites_history_predecessor_changed')
            expected=({'operation':previous['plan']['operation'],'receipt_sha256':digest(previous['receipt'])}
                if ident in previous['steps'].get(slot,{}) else previous['plan']['children'][slot]['preserved'][ident]['authority'])
            if preserved['authority']!=expected:
                raise Blocked('staging_prerequisites_history_predecessor_changed')
    return state,record,rollout


def validate_historical_capture(operation,*,installation=None):
    """Protected PASS history only, never current Session or live authority."""
    return copy.deepcopy(_historical(operation,set(),installation=installation)[1])


def _previous_pass(namespace_uid,*,installation=None):
    installation=resource_claim.require_installation(installation)
    active=_json(_read_private(guard.ROOT/'installed'/'active-release.json'))
    resource_claim.validate_active_release(active,installation=installation)
    state,record,rollout=_historical(active.get('operation'),set(),installation=installation)
    expected=rollout['context']['expected']
    if (active.get('namespace_uid')!=namespace_uid or state['plan']['namespace_uid']!=namespace_uid
        or expected['namespace_uid']!=namespace_uid or active.get('pvc_uid')!=expected['source_claim_uid']
        or active.get('target_sha256')!=rollout['target']['manifest_sha256']):
        raise Blocked('staging_prerequisites_active_changed')
    return active,state,record,rollout


def _origin(state,slot,ident,seen=None,*,installation=None):
    """Resolve exactly the resource referenced by each protected successor."""
    seen=set() if seen is None else seen
    operation=state['plan']['operation']
    if operation in seen or len(seen)>=64:raise Blocked('staging_prerequisites_history_invalid')
    step=state['steps'].get(slot,{}).get(ident)
    if step is not None:
        key=hashlib.sha256((slot+'/'+ident).encode()).hexdigest()
        response=_json(_read_private(guard.ROOT/'installed'/'prerequisites'/operation/'attempts'/(key+'.response.json')))
        if response.get('observation')!=step['observation']:raise Blocked('staging_prerequisites_history_effect_changed')
        return state,response
    preserved=state['plan']['children'][slot].get('preserved',{}).get(ident)
    if preserved is None:raise Blocked('staging_prerequisites_preserved_invalid')
    ref=preserved['authority'];prior,_,_=_historical(ref['operation'],set(),installation=installation)
    if digest(prior['receipt'])!=ref['receipt_sha256']:raise Blocked('staging_prerequisites_history_predecessor_changed')
    origin,response=_origin(prior,slot,ident,seen|{operation},installation=installation)
    if origin['plan']['operation']!=ref['operation']:
        raise Blocked('staging_prerequisites_preserved_reference_not_origin')
    return origin,response


def _publisher_source(operation,approved):
    import compiler
    raw=_read_private(guard.ROOT/'installed'/'sources'/operation/'deploy/staging/services.yaml')
    if hashlib.sha256(raw).hexdigest()!=approved['source_files'].get('deploy/staging/services.yaml'):
        raise Blocked('staging_prerequisites_preserved_source_changed')
    return [compiler.decode_yaml(piece.encode()) for piece in re.split(r'^---\s*$',raw.decode(),flags=re.M)
        if re.search(r'^  name: (?:voice-p3-principal-jwks|voice-p3-principal-publisher-config)\s*$',piece,re.M)]


def _qualified_preserved(session,slot,ident,previous,rollout,approved):
    """Current private object plus its successful origin; no existing adoption."""
    from stage_runtime import Kube
    origin,response=_origin(previous,slot,ident,installation=session.installation);origin_op=origin['plan']['operation']
    ref={'operation':origin_op,'receipt_sha256':digest(origin['receipt'])}
    if slot=='release_policy' and ident=='commit-root-policy':
        raw=_read_private(guard.ROOT/'installed'/'policy.json')
        payload=_read_private(guard.ROOT/'installed'/'prerequisites'/origin_op/'payloads'/slot/(hashlib.sha256(ident.encode()).hexdigest()+'.json'))
        if raw!=payload or response['object']!=_json(raw):raise Blocked('staging_prerequisites_preserved_policy_changed')
        return {'authority':ref,'semantic':_json(raw),'sha256':hashlib.sha256(raw).hexdigest()},response['observation']
    semantic=_semantic(response['object']);uid=response['observation']['uid']
    if slot=='p3':
        old_approved=_json(_read_private(guard.ROOT/'installed'/'prerequisites'/origin_op/'approved.json',limit=32<<20))
        if _publisher_source(origin_op,old_approved)!=_publisher_source(session.operation,approved):
            raise Blocked('staging_prerequisites_preserved_source_changed')
        old_aux=origin['plan']['children']['p3']['auxiliary_image']
        current_aux=_json(_read_private(_base(session)/'inputs'/'p3-auxiliary'/'record.json'))
        if any(old_aux[k]!=current_aux[k] for k in ('image_ref','manifest_sha256','index_sha256','config_sha256','platform')):
            raise Blocked('staging_prerequisites_preserved_image_changed')
        if ident=='Deployment/voice-p3-principal-jwks':
            row=rollout['context']['snapshots']['voice-p3-principal-jwks']
            selected=rollout['target'].get('template_hashes',{}).get('voice-p3-principal-jwks')
            if (row['metadata']['uid']!=uid or row['spec']['template']!=semantic['spec']['template']
                or selected is not None and selected!=digest(row['spec']['template'])):
                raise Blocked('staging_prerequisites_preserved_template_changed')
            semantic=_semantic(row)
    live=Kube().get(semantic['kind'].lower(),semantic['metadata']['name'])
    if slot=='gis_material' and ident=='gis-app-credentials' and _semantic(live)!=semantic:
        # Only the original completed, target-bound Realtime additive CAS may
        # extend the shared Secret. The loader verifies protected PASS ancestry.
        import realtime_bootstrap_existing as existing
        import realtime_bootstrap as rt
        _,plan,receipt,directory=existing._anchor()
        credential=existing._original_credential(directory,plan,receipt)
        before=_json(_read_private(directory/'before-secret.json'))
        if _semantic(before)!=semantic or before['metadata']['uid']!=uid:
            raise Blocked('staging_prerequisites_preserved_app_changed')
        extended=copy.deepcopy(before);extended['data']['REALTIME_DATABASE_URL']=rt.secret_patch(before,credential)[-1]['value']
        semantic=_semantic(extended)
    if live['metadata'].get('uid')!=uid or _semantic(live)!=semantic:
        raise Blocked('staging_prerequisites_preserved_live_changed')
    meta=live['metadata'];observation={'kind':live['kind'],'name':meta['name'],'namespace':'voice-staging','uid':uid,
        'resourceVersion':meta['resourceVersion'],'semantic_sha256':digest(semantic)}
    if live['kind']=='Deployment':observation['generation']=meta['generation']
    session.check()
    return {'authority':ref,'semantic':semantic,'uid':uid},observation


def load_preserved(session,slot,effect_id):
    state=_checkpoint(session);child=require_child(session,slot)
    if effect_id not in child.get('preserved',{}):raise Blocked('staging_prerequisites_preserved_invalid')
    current_active,previous,_,rollout=_previous_pass(state['plan']['namespace_uid'],installation=session.installation)
    expected_active=_json(_read_private(_base(session)/'previous-active.json'))
    if current_active!=expected_active or digest(expected_active)!=state['plan']['previous_active_sha256']:
        raise Blocked('staging_prerequisites_active_changed')
    authority=load_authority(session)
    result,observed=_qualified_preserved(session,slot,effect_id,previous,rollout,authority['approved'])
    expected=child['preserved'][effect_id]['observed']
    same=observed==expected
    if observed.get('kind')=='Deployment':same={k:v for k,v in observed.items() if k!='resourceVersion'}=={k:v for k,v in expected.items() if k!='resourceVersion'}
    if result['authority']!=child['preserved'][effect_id]['authority'] or not same:
        raise Blocked('staging_prerequisites_preserved_authority_changed')
    if _json(_read_private(guard.ROOT/'installed'/'active-release.json'))!=current_active or _checkpoint(session)!=state:
        raise Blocked('staging_prerequisites_active_changed')
    return result
