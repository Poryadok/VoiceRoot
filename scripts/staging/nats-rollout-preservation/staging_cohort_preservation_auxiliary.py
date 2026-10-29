"""One source-qualified read-only publisher restart after off-node restoration."""
import copy
import hashlib
import ssl
import time

import staging_cohort_preservation as owner
import staging_prerequisites as parent
import staging_p3_prerequisites as p3
import staging_p3_prerequisites_crypto as crypto
from stage_runtime import Kube
from controller import Blocked

NAME='voice-p3-principal-jwks'
IDENT='Deployment/'+NAME


def _inputs(session):
    plan=owner.load_plan(session);approved,_,prepared=owner._source(session,plan)
    child=p3.validate_plan_shape(prepared['plan']['children']['p3'])
    if IDENT not in child['preserved']:return None
    base=owner.guard.ROOT/'installed'/'prerequisites'/session.operation
    location=base/'inputs'/'p3-auxiliary';record=owner._read(location/'record.json')
    nonce=owner._read(location/'nonce.json');kube=Kube()
    source=owner.guard.ROOT/'installed'/'sources'/session.operation
    collector=parent._source_module(approved,source)
    raw={name:parent._read_private(location/(name+'.json'),limit=4<<20) for name in ('index','manifest','config')}
    content=collector.validate(raw['index'],raw['manifest'],raw['config'])
    expected={'schema':'voice-staging-p3-auxiliary-image-v1','operation':session.operation,'nonce':nonce['nonce'],
        'context':session.context,'helper_sha256':session.helper,'source_capture_sha256':plan['source_capture_sha256'],
        'source_module_sha256':approved['source_files']['scripts/deploy/staging_p3_auxiliary_image.py'],
        **content,'node':parent._node(kube)}
    if (record!=expected or nonce!={'operation':session.operation,'nonce':record['nonce']}
        or child['auxiliary_image']['source_authority_sha256']!=owner.digest(record)):
        raise Blocked('cohort_auxiliary_image_changed')
    issuer_raw=parent._read_private(base/'inputs'/'p3-issuer.json');issuer=parent._json(issuer_raw)
    anchors=owner._read(base/'inputs'/'issuer-anchors.json');receipt=issuer['receipt']
    if (hashlib.sha256(issuer_raw).hexdigest()!=child['issuer_input_sha256']
        or owner.digest(receipt)!=child['issuer_authority_sha256']
        or receipt.get('operation')!=session.operation or receipt.get('namespace_uid')!=plan['namespace_uid']
        or receipt.get('source_capture_sha256')!=plan['source_capture_sha256']
        or receipt.get('anchors_sha256')!=owner.digest(anchors)
        or set(anchors)!={'voice-media-ca','voice-space-media-client-tls','voice-space-media-server-tls'}
        or any(parent._secret(kube,name)!=row for name,row in anchors.items())):
        raise Blocked('cohort_auxiliary_issuer_changed')
    current={ident:kube.get(*[value.lower() if index==0 else value for index,value in enumerate(ident.split('/'))])
        for ident in p3.inventory_intent()}
    for ident,row in current.items():
        old=child['preserved'].get(ident)
        if old is None or row['metadata'].get('uid')!=old['observed']['uid'] or row['metadata'].get('deletionTimestamp'):
            raise Blocked('cohort_auxiliary_identity_changed')
        if ident!=IDENT and (p3.private_semantic(row) is None
            or owner.digest(p3.private_semantic(row))!=old['observed']['semantic_sha256']
            or row['metadata'].get('resourceVersion')!=old['observed']['resourceVersion']):
            raise Blocked('cohort_auxiliary_material_changed')
    values=p3._material_values(current);crypto.validate_issuer_materials(values,issuer)
    return child,record,issuer,current,values


def _health(session,response):
    inputs=_inputs(session)
    if inputs is None:raise Blocked('cohort_auxiliary_identity_changed')
    child,record,issuer,current,values=inputs;actual=current[IDENT]
    if (actual['metadata']['uid']!=response['metadata']['uid']
        or actual['metadata'].get('generation')!=response['metadata'].get('generation')
        or owner._cut_semantic(actual)!=owner._cut_semantic(response)):
        raise Blocked('cohort_auxiliary_successor_changed')
    desired=p3.private_semantic(response)
    before=p3._ready_snapshot(session,desired,record)
    leaf=values['voice-p3-principal-jwks-tls']['tls.crt']
    sha=hashlib.sha256(ssl.PEM_cert_to_DER_cert(leaf.decode('ascii'))).hexdigest()
    body=values['voice-p3-principal-public']['realtime.json']
    p3.probe_jwks(before['pod']['ip'],issuer['trust']['realtime-jwks-ca.crt'].encode('ascii'),body,sha)
    after=_inputs(session)
    if p3._ready_snapshot(session,desired,record)!=before or after[:3]!=inputs[:3] or after[4]!=values:
        raise Blocked('cohort_auxiliary_health_changed')
    return {**before,'tls':{'leaf_der_sha256':sha,'jwks_sha256':hashlib.sha256(body).hexdigest()}}


def resume(session):
    inputs=_inputs(session)
    if inputs is None:return None
    plan=owner._guard(session);owner._no_pending(session)
    if owner._state(session).get('status')!='AWAITING_OFF_NODE':raise Blocked('cohort_auxiliary_phase_invalid')
    owner._offnode_records(session)
    stage=owner._paused_stage(session);original=stage.original_snapshots[NAME];before=stage.kube.get('deployment',NAME)
    paused=stage.snapshots[NAME]
    if (before['metadata'].get('uid')!=paused['metadata']['uid'] or before['metadata'].get('generation')!=paused['metadata']['generation']
        or owner._cut_semantic(before)!=owner._cut_semantic(paused) or before['spec'].get('replicas',1)!=0
        or original['spec'].get('replicas',1)!=1):raise Blocked('cohort_auxiliary_pause_changed')
    desired=copy.deepcopy(before);desired['spec']['replicas']=1
    if (owner._cut_semantic(desired)!=owner._cut_semantic(original)
        or owner.digest(p3.private_semantic(original))!=inputs[0]['preserved'][IDENT]['observed']['semantic_sha256']):
        raise Blocked('cohort_auxiliary_template_changed')
    effect='resume-p3-publisher';path=owner._base(session)/'effects'/(effect+'.json')
    parent._private_directory(path.parent)
    body=parent._bytes([{'op':'test','path':'/metadata/uid','value':before['metadata']['uid']},
        {'op':'test','path':'/metadata/resourceVersion','value':before['metadata']['resourceVersion']},
        {'op':'test','path':'/metadata/generation','value':before['metadata']['generation']},
        {'op':'test','path':'/spec','value':before['spec']},{'op':'replace','path':'/spec/replicas','value':1}])
    argv=owner.PREFIX+['patch','deployment',NAME,'--type=json','--patch-file=/dev/stdin','-o','json']
    intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':effect,
        'binding':session.binding(),'argv':argv,'body_sha256':hashlib.sha256(body).hexdigest(),'status':'PENDING',
        'before':before,'desired':desired}
    owner._write(path,intent)
    actual=parent._json(owner._send(session,effect,argv,body))
    if (actual['metadata'].get('uid')!=before['metadata']['uid'] or actual['metadata'].get('generation')!=before['metadata']['generation']+1
        or not actual['metadata'].get('resourceVersion') or actual['metadata']['resourceVersion']==before['metadata']['resourceVersion']
        or owner._cut_semantic(actual)!=owner._cut_semantic(desired)):raise Blocked('cohort_auxiliary_response_changed')
    owner.save(path,{**intent,'status':'DONE','response':actual})
    deadline=time.monotonic()+90
    while True:
        session.check()
        try:proof=_health(session,actual);break
        except (ValueError,KeyError,p3.PrerequisiteError):
            if time.monotonic()>=deadline:raise Blocked('cohort_auxiliary_health_unqualified') from None
            time.sleep(.5)
    value={'schema':'voice-staging-cohort-auxiliary-successor-v1','operation':session.operation,
        'plan_sha256':owner.digest(plan),'effect_sha256':owner.digest(owner._read(path)),
        'before':before,'response':actual,'proof':proof}
    owner._write(owner._base(session)/'auxiliary-successor.json',value)
    return value


def load(session):
    plan=owner.load_plan(session)
    if NAME not in owner._precut(session)['context']['original_snapshots']:return None
    value=owner._read(owner._base(session)/'auxiliary-successor.json')
    effect=owner._read(owner._base(session)/'effects'/'resume-p3-publisher.json')
    if (set(value)!={'schema','operation','plan_sha256','effect_sha256','before','response','proof'}
        or value['schema']!='voice-staging-cohort-auxiliary-successor-v1' or value['operation']!=session.operation
        or value['plan_sha256']!=owner.digest(plan) or value['effect_sha256']!=owner.digest(effect)
        or effect.get('status')!='DONE' or effect.get('response')!=value['response'] or effect.get('before')!=value['before']):
        raise Blocked('cohort_auxiliary_receipt_changed')
    if _health(session,value['response'])!=value['proof']:raise Blocked('cohort_auxiliary_health_changed')
    return copy.deepcopy(value)
