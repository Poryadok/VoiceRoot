"""Root-only fixed migration authority from approved passive source + live INFO.

No arbitrary caller plan, seed, publication or live API mutation is accepted.
"""
import hashlib,re
from pathlib import Path
from bootstrap_selection import select
from bootstrap_root import secret_bytes
from controller import Blocked
from docker_runtime import NATS_IMAGE
from nats_contract_plan import DECLARATIONS,V3_DECLARATIONS,V4_SCHEMA,SCHEMA,V5_SOURCE_HASHES,V6_SCHEMA,V6_DECLARATIONS,V6_SOURCE_HASHES,compile_plan,digest
from rollout_census import census
from stage_runtime import HUB

TARGET_FILES={'realtime':'1122d97dca3b185aecb43e0ae41d1c255d0adc46090f9f1c3dd8faf4ee294d31',
    'analytics-chat':'7f37cdee8a59a245998371a4eddb0d4c80be965086ee06a5f30e8bd75b05d4b1',
    'search':'997b35cc0526eda5bead2a6ecfba95df59423c800fa7e960599466b3ee51334a'}
V4_TARGET_FILES=TARGET_FILES
V5_TARGET_FILES={path.removeprefix('deploy/templates/nats-').removesuffix('-bootstrap.yaml'):sha for path,sha in V5_SOURCE_HASHES.items()}
V6_TARGET_FILES={path.removeprefix('deploy/templates/nats-').removesuffix('-bootstrap.yaml'):sha for path,sha in V6_SOURCE_HASHES.items()}
def fail():raise Blocked('nats_migration_root_preflight_refused')

_LEGACY_AUTH_KEYS={'verified','server_image','account_sha256','reply_prefix','normalized_consumer','normalized_consumer_sha256'}
_V3_AUTH_KEYS=_LEGACY_AUTH_KEYS|{'normalized_consumers','normalized_consumers_sha256'}
_V4_AUTH_KEYS=_V3_AUTH_KEYS|{'schema'}
def _validated_normalized_consumers(live_auth,enrolled_auth,*,version=V4_SCHEMA):
    """Extend exact prior defaults without changing immutable enrollment bytes."""
    if not isinstance(enrolled_auth,dict) or set(enrolled_auth) not in (_LEGACY_AUTH_KEYS,_V3_AUTH_KEYS,_V4_AUTH_KEYS):fail()
    v6=version==V6_SCHEMA
    if not isinstance(live_auth,dict) or set(live_auth)!=_V4_AUTH_KEYS or live_auth.get('schema')!=('voice-bootstrap-auth-v5' if v6 else 'voice-bootstrap-auth-v4'):fail()
    if {key:live_auth[key] for key in _LEGACY_AUTH_KEYS}!={key:enrolled_auth[key] for key in _LEGACY_AUTH_KEYS}:fail()
    def normalized_map(auth,declarations):
        rows=auth['normalized_consumers']
        if not isinstance(rows,list) or len(rows)!=len(declarations) or digest(rows)!=auth['normalized_consumers_sha256']:fail()
        result={}
        for row,(stream,declaration) in zip(rows,declarations):
            if (not isinstance(row,dict) or set(row)!={'stream','durable','config'}
                or row['stream']!=stream or row['durable']!=declaration['durable_name']
                or not isinstance(row['config'],dict)
                or any(row['config'].get(key)!=value for key,value in declaration.items())):fail()
            key=(stream,declaration['durable_name'])
            if key in result:fail()
            result[key]=row['config']
        legacy=result.get(('social_events',DECLARATIONS[0][1]['durable_name']))
        if (legacy!=auth['normalized_consumer']
            or digest(legacy)!=auth['normalized_consumer_sha256']):fail()
        return result
    if v6:
        # A new live proof extends normalization; it never rewrites enrollment.
        rows=live_auth.get('normalized_consumers')
        if not isinstance(rows,list) or len(rows)!=len(V6_DECLARATIONS):fail()
        projection={**live_auth,'schema':'voice-bootstrap-auth-v4',
            'normalized_consumers':rows[:len(DECLARATIONS)],
            'normalized_consumers_sha256':digest(rows[:len(DECLARATIONS)])}
        if enrolled_auth.get('schema')=='voice-bootstrap-auth-v5':
            if enrolled_auth!=live_auth:fail()
        else:
            _validated_normalized_consumers(projection,enrolled_auth)
        result=normalized_map(live_auth,V6_DECLARATIONS)
        roster=result[('voice_events','matchmaking_voice_roster_v1')]
        if roster.get('deliver_subject','')!='' or roster.get('deliver_group','')!='' or roster.get('filter_subjects',[])!=[]:fail()
        return result
    if set(enrolled_auth)==_V3_AUTH_KEYS:
        normalized_map(enrolled_auth,V3_DECLARATIONS)
        if live_auth['normalized_consumers'][:len(V3_DECLARATIONS)]!=enrolled_auth['normalized_consumers']:fail()
    elif set(enrolled_auth)==_V4_AUTH_KEYS:
        if enrolled_auth!=live_auth:fail()
    result=normalized_map(live_auth,DECLARATIONS)
    return result
def monitor(kube):
    hub=kube.get('deployment',HUB)
    pods=kube.run(['get','pods','-l','app='+HUB,'-o','json'])['items']
    if len(pods)!=1:fail()
    pod=pods[0];name=pod['metadata']['name']
    if not re.fullmatch(HUB+'-[a-z0-9-]+',name) or pod['status'].get('phase')!='Running' or not any(c.get('type')=='Ready' and c.get('status')=='True' for c in pod['status'].get('conditions',[])):fail()
    owners=pod['metadata'].get('ownerReferences',[])
    if len(owners)!=1 or owners[0].get('kind')!='ReplicaSet':fail()
    rs=kube.get('replicaset',owners[0]['name'])
    if rs['metadata']['uid']!=owners[0]['uid'] or not any(o.get('uid')==hub['metadata']['uid'] and o.get('kind')=='Deployment' for o in rs['metadata'].get('ownerReferences',[])):fail()
    url='/api/v1/namespaces/voice-staging/pods/'+name+':8222/proxy/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048'
    tree=kube.run(['get','--raw',url])
    if kube.get('deployment',HUB)!=hub:fail()
    return tree
def preflight(kube,source,enrollment,contract,decoder,*,installation=None):
    from resource_claim import require_installation
    installation=require_installation(installation)
    select(kube,enrollment,installation.generation,installation=installation)
    return _from_tree(kube,source,enrollment,contract,decoder,monitor(kube),installation=installation)

def closed_copy_preflight(kube,source,enrollment,contract,decoder,runtime,broker,tree,*,installation=None):
    """Root recovery callback: actual owned restored broker snapshot only."""
    from resource_claim import require_installation
    installation=require_installation(installation)
    select(kube,enrollment,installation.generation,installation=installation)
    owned=runtime.inspect(broker)
    if (owned['Config']['Image']!=NATS_IMAGE or owned['HostConfig']['NetworkMode']!='none'
        or runtime.owned[broker]['id']!=owned['Id']):fail()
    generation=enrollment['binding']['generation']
    account=secret_bytes(kube.get('secret','voice-nats-operator-'+generation),'account.public').decode().strip()
    if runtime.account_id()!=account:fail()
    result=_from_tree(kube,source,enrollment,contract,decoder,tree,installation=installation)
    if runtime.inspect(broker)['Id']!=owned['Id']:fail()
    return result

def _from_tree(kube,source,enrollment,contract,decoder,tree,*,installation=None):
    from resource_claim import require_installation
    installation=require_installation(installation)
    generation=installation.generation;select(kube,enrollment,generation,installation=installation)
    actual={}
    for role in TARGET_FILES:
        path=Path(source)/('deploy/templates/nats-'+role+'-bootstrap.yaml')
        if path.is_symlink() or not path.is_file():fail()
        actual[role]=hashlib.sha256(path.read_bytes()).hexdigest()
    # Select the complete reviewed source tuple. No per-file old/new mixing.
    notification=Path(source)/'deploy/templates/nats-notification-bootstrap.yaml'
    v5_actual=dict(actual)
    if not notification.is_symlink() and notification.is_file():v5_actual['notification']=hashlib.sha256(notification.read_bytes()).hexdigest()
    if v5_actual==V6_TARGET_FILES:version=V6_SCHEMA
    elif v5_actual==V5_TARGET_FILES:version=SCHEMA
    elif actual==TARGET_FILES:version=V4_SCHEMA
    else:fail()
    account=secret_bytes(kube.get('secret','voice-nats-operator-'+generation),'account.public').decode().strip()
    class Runtime:
        def monitor_jsz(self,broker):return tree
    row=census(Runtime(),'root-live-monitor',account)
    detail=next(a for a in tree['account_details'] if a['id']==account)['stream_detail']
    streams={s['name']:{'config':s['config'],'state':s['state'],'consumer_detail':s.get('consumer_detail',[])} for s in detail}
    current={}
    for stream in detail:
        for consumer in stream.get('consumer_detail',[]):
            key=(stream['name'],consumer.get('durable') or consumer['name'])
            if key in current:fail()
            current[key]=consumer
    from bootstrap_auth import prove
    def unchanged():select(kube,enrollment,generation,installation=installation)
    live_auth=prove(secret_bytes(kube.get('secret',enrollment['secret']['name']),'bootstrap.creds'),kube.get('secret','voice-nats-operator-'+generation),unchanged,
        installation=installation,**({'version':'voice-bootstrap-auth-v5'} if version==V6_SCHEMA else {}))
    normalized=_validated_normalized_consumers(live_auth,enrollment['auth_proof'],version=version)
    plan=compile_plan(source,streams,current,normalized,version=version)
    selected=V6_TARGET_FILES if version==V6_SCHEMA else V5_TARGET_FILES if version==SCHEMA else TARGET_FILES
    expected_sources={'deploy/templates/nats-'+part+'-bootstrap.yaml':sha for part,sha in selected.items()}
    planned_paths=set(expected_sources) if version in (SCHEMA,V6_SCHEMA) else set(expected_sources)-{'deploy/templates/nats-search-bootstrap.yaml'}
    if plan['sources']!={path:expected_sources[path] for path in planned_paths}:fail()
    scripts={}
    for part in selected:
        if sum(old['part']==part for old in contract['scripts'])!=1:fail()
        path=Path(source)/('deploy/templates/nats-'+part+'-bootstrap.yaml')
        if path.is_symlink() or not path.is_file():fail()
        raw=path.read_bytes()
        if hashlib.sha256(raw).hexdigest()!=selected[part]:fail()
        text=raw.decode('utf8').replace('__NAMESPACE__','voice-staging').replace('__K_NAMESPACE__','voice-staging')
        cm=decoder(re.split(r'^---\s*$',text,flags=re.MULTILINE)[0].encode())
        if cm['kind']!='ConfigMap' or cm['metadata']['name']!='voice-nats-'+part+'-bootstrap':fail()
        script=cm['data']['bootstrap.sh'];scripts[part]={'sha256':hashlib.sha256(script.encode()).hexdigest(),'script':script}
    binding={'plan_sha256':digest(plan),'server_image':NATS_IMAGE,'enrollment_sha256':digest(enrollment),'sources':plan['sources']}
    if version==V6_SCHEMA:binding['live_auth_sha256']=digest(live_auth)
    binding.update(schema='voice-nats-root-plan-binding-v2',installation_sha256=installation.sha256)
    return {'plan':plan,'binding':binding,'scripts':scripts,'live_census_sha256':digest(row)}
