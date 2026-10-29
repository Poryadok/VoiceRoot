"""Prepare the source-bound staging release policy inside root authority."""
import copy
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import sys
import uuid

import release_policy_wiring as wiring
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
from stage_runtime import Kube

def descriptor(candidate, object_name, key, owners):
    """Construct only the documented alpha values; placement is supplied."""
    policy={'schema_version':2,'profile':'free-invite-alpha','source_sha':candidate,
            'capabilities':{name:'disabled' for name in wiring.CAPABILITIES},
            'rich_intents':{**{name:'qualified' for name in wiring.BASELINE},
                            **{name:'disabled' for name in wiring.RESTRICTED}}}
    raw=json.dumps(policy,sort_keys=True,separators=(',',':'),ensure_ascii=False)
    value={'profile':'free-invite-alpha','source_sha':candidate,
           'policy_sha256':hashlib.sha256(raw.encode('utf-8')).hexdigest(),
           'policy_object':{'apiVersion':'v1','kind':'ConfigMap',
                            'metadata':{'name':object_name,'namespace':'voice-staging'},
                            'immutable':True,'data':{key:raw}},'owners':copy.deepcopy(owners)}
    wiring.validate(value,candidate,'voice-staging')
    _check_owner_paths(value)
    return value

def staging_descriptor(candidate):
    """Use the explicitly selected staging placement after candidate freeze."""
    if not isinstance(candidate,str) or not re.fullmatch(r'[a-f0-9]{40}',candidate) or candidate=='0'*40:
        raise ValueError('release_policy_source_or_profile')
    owner_paths={name:{'policy_directory':'/etc/voice/release',
                       'snapshot_file':'/tmp/voice-release-snapshot.json'} for name in wiring.OWNERS}
    return descriptor(candidate,'voice-release-policy-'+candidate[:12],'policy.json',owner_paths)

def policy_successor(existing, value, candidate):
    """Build a policy update while preserving all unrelated operator fields."""
    if not isinstance(existing,dict):raise ValueError('release_policy_operator_policy_invalid')
    wiring.validate(value,candidate,'voice-staging');_check_owner_paths(value)
    successor=copy.deepcopy(existing)
    successor['release_policy']=copy.deepcopy(value)
    return successor

def _check_owner_paths(value):
    for row in value['owners'].values():
        if row['policy_directory'].startswith('//'):
            raise ValueError('release_policy_absolute_directory')

def _check_policy_mounts(text,value):
    for part in re.split(r'^---\s*$',text,flags=re.M):
        match=re.search(r'^  name: voice-(auth|user|notification|realtime)$',part,re.M)
        if not match or not re.search(r'^kind: Deployment$',part,re.M):continue
        owner=match[1];start='        - name: '+owner+'\n'
        body=part.split(start,1)[1].split('\n        - name:',1)[0]
        desired=value['owners'][owner]['policy_directory'].rstrip('/')
        placeholder='__RELEASE_POLICY_DIRECTORY_'+owner.upper()+'__'
        # This is a closed source syntax contract, not a YAML parser: reject
        # escapes, anchors and aliases before inspecting literal mount paths.
        if re.search(r'[\\&*]',body):raise ValueError('release_policy_mount_syntax_invalid')
        occurrences=re.findall(r'mountPath\s*:',body)
        parsed=re.findall(r'mountPath:\s*(?:"([A-Za-z0-9_./-]+)"|\'([A-Za-z0-9_./-]+)\'|([A-Za-z0-9_./-]+))\s*(?:,|}|$)',body,flags=re.M)
        if body.count('mountPath')!=len(parsed) or len(occurrences)!=len(parsed):
            raise ValueError('release_policy_mount_syntax_invalid')
        for row in parsed:
            mount=next(part for part in row if part)
            if mount==placeholder:continue
            parsed_path=PurePosixPath(mount)
            if mount.startswith('//') or not parsed_path.is_absolute() or '..' in parsed_path.parts or str(parsed_path)!=mount:
                raise ValueError('release_policy_mount_syntax_invalid')
            if mount=='/' or mount==desired or desired.startswith(mount+'/') or mount.startswith(desired+'/'):
                raise ValueError('release_policy_mount_collision')

def validate_source(approved, source, value):
    """Require the protected capture's exact template and candidate binding."""
    if not isinstance(approved,dict) or not isinstance(approved.get('source_files'),dict):
        raise ValueError('release_policy_source_authority_invalid')
    candidate=approved.get('source_sha')
    wiring.validate(value,candidate,'voice-staging');_check_owner_paths(value)
    relative='deploy/staging/services.yaml'
    path=Path(source)/relative
    if path.is_symlink() or not path.is_file() or path.stat().st_size>2<<20:
        raise ValueError('release_policy_source_template')
    expected=approved['source_files'].get(relative)
    if hashlib.sha256(path.read_bytes()).hexdigest()!=expected:
        raise ValueError('release_policy_source_template_changed')
    result=wiring.source_input(source,{'tag':candidate,'release_policy':value})
    if result!=value or hashlib.sha256(path.read_bytes()).hexdigest()!=expected:
        raise ValueError('release_policy_source_template_changed')
    _check_policy_mounts(path.read_text(encoding='utf-8-sig'),value)
    if hashlib.sha256(path.read_bytes()).hexdigest()!=expected:
        raise ValueError('release_policy_source_template_changed')
    return copy.deepcopy(value)

def _canonical(value):
    return json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode('utf-8')

def plan(approved, source, old_policy_bytes, value, *, operation, namespace_uid, source_capture_sha256, list_rv):
    """Create a finite plan and exact payloads; this function grants no authority."""
    if (not isinstance(operation,str) or not re.fullmatch(r'[a-f0-9]{12}',operation)
        or not isinstance(source_capture_sha256,str) or not re.fullmatch(r'[a-f0-9]{64}',source_capture_sha256)
        or not isinstance(list_rv,str) or not list_rv or not isinstance(old_policy_bytes,bytes)
        or not 1<=len(old_policy_bytes)<=2<<20):
        raise ValueError('release_policy_plan_input_invalid')
    try:
        if str(uuid.UUID(namespace_uid))!=namespace_uid:raise ValueError('namespace_uid')
        existing=json.loads(old_policy_bytes,object_pairs_hook=wiring.pairs)
    except (ValueError,TypeError,AttributeError,UnicodeDecodeError) as exc:
        raise ValueError('release_policy_plan_input_invalid') from exc
    validate_source(approved,source,value)
    successor=policy_successor(existing,value,approved['source_sha'])
    payloads={'create-policy-configmap':_canonical(value['policy_object']),
              'commit-root-policy':_canonical(successor)}
    hashes={name:hashlib.sha256(raw).hexdigest() for name,raw in payloads.items()}
    before=hashlib.sha256(old_policy_bytes).hexdigest()
    effects=[{'id':'create-policy-configmap','kind':'ConfigMap','name':value['policy_object']['metadata']['name'],
              'namespace':'voice-staging','before':{'absent':True,'list_rv':list_rv},
              'desired_sha256':hashes['create-policy-configmap'],'payload_sha256':hashes['create-policy-configmap']},
             {'id':'commit-root-policy','kind':'RootPolicy','name':'installed-policy','namespace':'voice-staging',
              'before':{'sha256':before},'desired_sha256':hashes['commit-root-policy'],
              'payload_sha256':hashes['commit-root-policy']}]
    record={'schema':'voice-staging-release-policy-plan-v1','operation':operation,'namespace_uid':namespace_uid,
            'source_capture_sha256':source_capture_sha256,'source_sha':approved['source_sha'],
            'descriptor':copy.deepcopy(value),'policy_before_sha256':before,
            'policy_after_sha256':hashes['commit-root-policy'],'effects':effects}
    return record,payloads

def validate_plan(record,payloads,approved,source,old_policy_bytes):
    """Rebuild the closed child plan from protected parent-provided inputs."""
    try:
        if not isinstance(record,dict) or not isinstance(payloads,dict):raise ValueError('shape')
        first=record['effects'][0]
        rebuilt,expected=plan(approved,source,old_policy_bytes,record['descriptor'],
            operation=record['operation'],namespace_uid=record['namespace_uid'],
            source_capture_sha256=record['source_capture_sha256'],list_rv=first['before']['list_rv'])
        if record!=rebuilt or payloads!=expected:raise ValueError('mismatch')
    except (KeyError,IndexError,TypeError,ValueError) as exc:
        raise ValueError('release_policy_child_plan_invalid') from exc
    return True

def preflight_payloads(session):
    """Check protected inputs and live preimages once before any child effect."""
    import guard
    import operator_retirement
    import staging_prerequisites as parent
    current=parent.require_child(session,'release_policy')
    authority=parent.load_authority(session)
    if not isinstance(authority,dict) or set(authority)!={'approved','source'}:
        raise ValueError('release_policy_source_authority_invalid')
    payloads={effect:parent.load_payload(session,'release_policy',effect)
              for effect in ('create-policy-configmap','commit-root-policy')}
    policy_path=guard.ROOT/'installed'/'policy.json'
    before_info=os.lstat(policy_path)
    before_identity=(before_info.st_dev,before_info.st_ino,before_info.st_mode,before_info.st_uid,before_info.st_nlink)
    old=operator_retirement.private_bytes(policy_path,2<<20)
    validate_plan(current,payloads,authority['approved'],authority['source'],old)
    kube=Kube();namespace=kube.get('namespace','voice-staging');metadata=namespace.get('metadata',{})
    if (metadata.get('name')!='voice-staging' or metadata.get('uid')!=current['namespace_uid']
        or metadata.get('deletionTimestamp')):
        raise ValueError('release_policy_namespace_changed')
    name=current['effects'][0]['name']
    path='/api/v1/namespaces/voice-staging/configmaps?fieldSelector=metadata.name%3D'+name+'&limit=2'
    listing=kube.run(['get','--raw',path]);metadata=listing.get('metadata',{})
    if (listing.get('apiVersion')!='v1' or listing.get('kind')!='ConfigMapList'
        or listing.get('items')!=[] or metadata.get('continue')
        or not isinstance(metadata.get('resourceVersion'),str)
        or not re.fullmatch(r'[0-9]{1,20}',metadata['resourceVersion'])):
        raise ValueError('release_policy_configmap_absence_changed')
    final=operator_retirement.private_bytes(policy_path,2<<20)
    after_info=os.lstat(policy_path)
    after_identity=(after_info.st_dev,after_info.st_ino,after_info.st_mode,after_info.st_uid,after_info.st_nlink)
    if (after_identity!=before_identity or final!=old
        or parent.require_child(session,'release_policy')!=current
        or parent.load_authority(session)!=authority):
        raise ValueError('release_policy_preflight_changed')
    validate_plan(current,payloads,authority['approved'],authority['source'],old)
    return True

def execute(session):
    """Ask the fixed coordinator to run only the two planned effects."""
    import staging_prerequisites as parent
    parent.require_child(session,'release_policy')
    for effect in ('create-policy-configmap','commit-root-policy'):
        parent.record_pending(session,'release_policy',effect)
        observation=parent.submit_effect(session,'release_policy',effect)
        parent.record_done(session,'release_policy',effect,observation)

def verify_effect(session,effect_id,observation):
    """Re-read the fixed target before the coordinator records completion."""
    import staging_prerequisites as parent
    current=parent.require_child(session,'release_policy')
    if (not isinstance(current,dict) or set(current)!={'schema','operation','namespace_uid','source_capture_sha256',
            'source_sha','descriptor','policy_before_sha256','policy_after_sha256','effects'}
        or current.get('schema')!='voice-staging-release-policy-plan-v1'
        or not isinstance(current.get('effects'),list)
        or [row.get('id') for row in current['effects'] if isinstance(row,dict)]!=['create-policy-configmap','commit-root-policy']
        or effect_id not in ('create-policy-configmap','commit-root-policy')):
        raise ValueError('release_policy_child_plan_invalid')
    value=current['descriptor'];wiring.validate(value,current['source_sha'],'voice-staging');_check_owner_paths(value)
    effect=next(row for row in current['effects'] if row['id']==effect_id)
    if set(effect)!={'id','kind','name','namespace','before','desired_sha256','payload_sha256'}:
        raise ValueError('release_policy_child_effect_invalid')
    payload=parent.load_payload(session,'release_policy',effect_id)
    if not isinstance(payload,bytes) or hashlib.sha256(payload).hexdigest()!=effect['payload_sha256']:
        raise ValueError('release_policy_child_payload_changed')
    if effect_id=='create-policy-configmap':
        expected=value['policy_object']
        if (effect['kind']!='ConfigMap' or effect['name']!=expected['metadata']['name']
            or effect['namespace']!='voice-staging' or payload!=_canonical(expected)
            or effect['desired_sha256']!=hashlib.sha256(payload).hexdigest()
            or not isinstance(observation,dict) or set(observation)!={'kind','name','namespace','uid','resourceVersion','semantic_sha256'}):
            raise ValueError('release_policy_configmap_receipt_invalid')
        kube=Kube();namespace=kube.get('namespace','voice-staging')
        if (namespace.get('metadata',{}).get('name')!='voice-staging'
            or namespace.get('metadata',{}).get('uid')!=current['namespace_uid']):
            raise ValueError('release_policy_namespace_changed')
        live=kube.get('configmap',effect['name']);metadata=live.get('metadata',{})
        semantic={'apiVersion':live.get('apiVersion'),'kind':live.get('kind'),
                  'metadata':{'name':metadata.get('name'),'namespace':metadata.get('namespace')},
                  'immutable':live.get('immutable'),'data':live.get('data')}
        if (set(live)-{'apiVersion','kind','metadata','immutable','data'} or metadata.get('deletionTimestamp')
            or metadata.get('labels') or metadata.get('annotations') or metadata.get('ownerReferences')
            or metadata.get('finalizers') or not isinstance(metadata.get('uid'),str) or not metadata['uid']
            or not isinstance(metadata.get('resourceVersion'),str) or not metadata['resourceVersion']
            or semantic!=expected or observation!={'kind':'ConfigMap','name':effect['name'],'namespace':'voice-staging',
                'uid':metadata['uid'],'resourceVersion':metadata['resourceVersion'],
                'semantic_sha256':hashlib.sha256(_canonical(semantic)).hexdigest()}):
            raise ValueError('release_policy_configmap_readback_changed')
    else:
        import guard
        import operator_retirement
        if (effect['kind']!='RootPolicy' or effect['name']!='installed-policy' or effect['namespace']!='voice-staging'
            or effect['desired_sha256']!=current['policy_after_sha256']
            or effect['payload_sha256']!=current['policy_after_sha256']
            or observation!={'kind':'RootPolicy','name':'installed-policy','sha256':current['policy_after_sha256']}):
            raise ValueError('release_policy_root_receipt_invalid')
        raw=operator_retirement.private_bytes(guard.ROOT/'installed'/'policy.json',2<<20)
        if raw!=payload or hashlib.sha256(raw).hexdigest()!=current['policy_after_sha256']:
            raise ValueError('release_policy_root_readback_changed')
        policy=json.loads(raw,object_pairs_hook=wiring.pairs)
        if not isinstance(policy,dict) or policy.get('release_policy')!=value:
            raise ValueError('release_policy_root_readback_changed')
    if parent.require_child(session,'release_policy')!=current:
        raise ValueError('release_policy_child_plan_changed')
    return copy.deepcopy(observation)
