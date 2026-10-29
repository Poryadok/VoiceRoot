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
REQUIRED_HELPERS=frozenset('nats-rollout-preservation/'+name+'.py' for name in
    ('staging_prerequisites','staging_release_policy_prepare','staging_p3_prerequisites','staging_p3_prerequisites_crypto',
     'staging_gis_material','gis_first_install','gis_prerequisite','source_plan','compiler'))
PLAN_FIELDS={'schema','operation','context','helper_sha256','source_capture_sha256','approved_sha256',
    'execution_sha256','namespace_uid','marker_sha256','baseline_sha256','actual_before',
    'policy_before_sha256','children','auxiliary_images','renderer_authority_sha256'}
SHA=re.compile(r'[a-f0-9]{64}')
OP=re.compile(r'[a-f0-9]{12}')
KUBE_PREFIX=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging']
# A response is usable only in the process and Session which issued the write.
# Durable attempts remain PENDING after a lost response or process interruption.
_responses=weakref.WeakKeyDictionary()


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


def _capture_authority(actions,session,approved,execution):
    """Capture the already-issued source and helper authority for this operation."""
    import bridge_root
    import commands
    import root_cli
    if (type(actions) is not bridge_root.Actions or actions.tt_session is not session
        or commands.authority_guard!=session.check or session.check().get('phase')!='prepare'):
        raise Blocked('staging_prerequisites_session_invalid')
    source=json.loads(_read_private(guard.ROOT/'installed'/'source-authority'/(session.operation+'.json')),
                      object_pairs_hook=guard.object_pairs)
    if (type(source) is not dict or set(source)!={'schema','operation','context','helper_sha256','approved_sha256','execution_sha256'}
        or source['schema']!='voice-staging-source-capture-v1' or source['operation']!=session.operation
        or source['context']!=session.context or source['helper_sha256']!=session.helper
        or type(approved) is not dict or approved.get('source_sha')!=session.context['commit']
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
    if verified!=observation or _checkpoint(session)!=state:raise Blocked('staging_prerequisites_observation_changed')
    after=copy.deepcopy(state);after['steps'][slot][effect_id]={'status':'DONE','observation':copy.deepcopy(verified)}
    _store_state(session,state,after)
    del _responses[session][(slot,effect_id)]


def require_child(session,slot):
    if slot not in SLOTS:raise Blocked('staging_prerequisites_slot_invalid')
    state=_checkpoint(session);plan=state['plan']['children'][slot]
    if type(plan) is not dict:raise Blocked('staging_prerequisites_child_invalid')
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
        if relative in BINARY_PINS:
            if wanted!=BINARY_PINS[relative]:raise Blocked('staging_prerequisites_helper_changed')
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


def _absence(session,kind,name):
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
    for item in row['items']:
        if item.get('metadata',{}).get('namespace')!='voice-staging':raise Blocked('staging_prerequisites_absence_invalid')
        if kind!='Deployment' or item['metadata'].get('name')==name:raise Blocked('staging_prerequisites_create_collision')
    return meta['resourceVersion']


def check_effect(session,slot,effect_id):
    import commands
    from stage_runtime import Kube
    state=_checkpoint(session);effect=_selected_effect(state,slot,effect_id)
    if (commands.authority_guard!=session.check or state['preflight_sha256']!=digest(state['plan'])
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
    from stage_runtime import Kube
    baseline=_json(_read_private(_base(session)/'baseline.json',limit=32<<20))
    if digest(baseline)!=state['plan']['baseline_sha256']:
        raise Blocked('staging_prerequisites_baseline_changed')
    realtime_bootstrap._check_stage(Kube(),baseline['workloads'])
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
