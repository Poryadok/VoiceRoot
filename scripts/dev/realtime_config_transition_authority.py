"""Reconstruct P3 configuration inputs from concrete fenced phase objects."""
import base64
import copy
from dataclasses import dataclass
import hashlib
import json
import re

import realtime_config_transition_inputs as inputs
import realtime_config_transition_redis as redis
import realtime_config_transition_space_plan_phase as space_plan
import realtime_p3_adoption as adoption
import realtime_p3_create_only_transport as transport
import writer_cut


class AuthorityError(RuntimeError): pass


@dataclass(frozen=True,eq=False,repr=False)
class BoundFixed9SpaceAuthority:
    cut: object
    trust: object
    creator: object
    space: object
    role: object
    token: str


def bind_delegated_space(cut,trust,creator,space,role,token):
    """Bind the exact completed fixed-nine chain to its Space receiver."""
    import space_creator_runtime_profile_delegated as delegated
    import space_creator_runtime_profile_target_trust as target
    import space_role_peer_create as role_module
    try:
        if (type(cut) is not writer_cut.WriterCut or type(trust) is not target.TargetTrustPhase or
                type(creator) is not delegated.CreatorReceiver or type(space) is not delegated.SpaceReceiver or
                type(role) is not role_module.Session or space.cut is not cut or creator.cut is not cut or
                role.preflight.cut is not cut): raise ValueError()
        target.verify_fixed9_successor(trust,token,creator,space,role)
        binding=BoundFixed9SpaceAuthority(cut,trust,creator,space,role,token)
        target.verify_fixed9_successor(trust,token,creator,space,role)
        return binding
    except BaseException: raise AuthorityError('Space authority unavailable') from None


def _delegated_space(cut,binding,token):
    import realtime_p3_create_only_crypto as crypto
    import space_creator_runtime_profile_delegated as delegated
    import space_creator_runtime_profile_target_trust as target
    import space_role_peer_create as role_module
    if type(binding) is not BoundFixed9SpaceAuthority or binding.cut is not cut or token!=binding.token: raise ValueError()
    phase=binding.space
    refs=(binding.cut,binding.trust,binding.creator,binding.space,binding.role); bound_token=binding.token
    cut_refs=(cut.fence,cut.fence.ledger,cut.client,cut.journal)
    cut_fixed=copy.deepcopy((cut.fence.context,cut.lock,cut.namespace_uid))
    def outer():
        if (type(cut) is not writer_cut.WriterCut or type(refs[1]) is not target.TargetTrustPhase or
                type(refs[2]) is not delegated.CreatorReceiver or type(phase) is not delegated.SpaceReceiver or
                type(refs[4]) is not role_module.Session or refs[2].cut is not cut or phase.cut is not cut or
                refs[4].preflight.cut is not cut or refs[1].preflight.cut is not cut or
                any(p.fence.context!=cut.fence.context or p.lock!=cut.lock or p.namespace_uid!=cut.namespace_uid
                    for p in (refs[1],refs[2],phase,refs[4]))): raise ValueError()
        cut._guard(); row,revision=cut.fence.ledger.read()
        if any(p.fence.ledger.read()!=(row,revision) for p in (refs[1],refs[2],phase,refs[4])): raise ValueError()
        return row,revision
    before=outer()
    target.verify_fixed9_successor(refs[1],token,refs[2],phase,refs[4])
    if outer()!=before: raise ValueError()
    original=copy.deepcopy(phase.proof); name='voice-space-media-server-tls'
    identity=original['objects'][name]; jwks=original['public_jwks']['voice-space-principal-signing']
    raw=transport.public_certificate(name,'tls.crt').split(b'\n',2)
    if (len(raw)!=3 or raw[0].decode()!=identity['uid'] or raw[1].decode()!=identity['resourceVersion']): raise ValueError()
    der=hashlib.sha256(crypto.der(base64.b64decode(raw[2],validate=True))).hexdigest()
    result={'schema':'voice-dev-space-delegated-v1','operation_id':cut.fence.context['operation_id'],
            'source_sha':cut.fence.context['commit'],'namespace_uid':cut.namespace_uid,
            'tls':identity|{'certificate_sha256':der},'media_ca':copy.deepcopy(original['ca']),
            'public_jwks':copy.deepcopy(jwks),'scope':'fixed-peer-space-material-no-runtime-readiness'}
    target.verify_fixed9_successor(refs[1],token,refs[2],phase,refs[4])
    import full_cohort_package_phase as package
    trust_local=copy.deepcopy(refs[1].journal.state['steps'][target.STEP]); trust_record=copy.deepcopy(refs[1].record)
    package_local=copy.deepcopy(refs[1].package.journal.state['steps'][package.STEP])
    cut_local=copy.deepcopy((cut.journal.intent,cut.journal.state))
    anchors=tuple(copy.deepcopy((p.initial,p.record,p.effects,p.proof)) for p in (refs[2],phase,refs[4]))
    if outer()!=before: raise ValueError()
    receipt=trust_local['receipt']
    refs[1]._locals(before[0],receipt['jdk_receipt_sha256'],receipt['preflight_receipt_sha256'])
    if (refs[1].journal.state['steps'].get(target.STEP)!=trust_local or refs[1].record!=trust_record or
            refs[1].package.journal.state['steps'].get(package.STEP)!=package_local): raise ValueError()
    for p,anchor in zip((refs[2],phase,refs[4]),anchors):
        p._local()
        if (p.initial,p.record,p.effects,p.proof)!=anchor: raise ValueError()
    cut._identity()
    if ((cut.journal.intent,cut.journal.state)!=cut_local or phase.proof!=original or binding.token!=bound_token or (cut.fence.context,cut.lock,cut.namespace_uid)!=cut_fixed or
            any(a is not b for a,b in zip((cut.fence,cut.fence.ledger,cut.client,cut.journal),cut_refs)) or
            any(a is not b for a,b in zip((binding.cut,binding.trust,binding.creator,binding.space,binding.role),refs))): raise ValueError()
    return result,json.dumps(jwks,sort_keys=True,separators=(',',':'),allow_nan=False).encode()


def _space(cut,phase,token):
    if type(phase) is space_plan.SpacePlanPhase:
        proof=space_plan.verify(phase,cut,token)
        return proof,base64.b64decode(proof['observation']['jwks_bytes'],validate=True)
    if type(phase) is BoundFixed9SpaceAuthority:
        return _delegated_space(cut,phase,token)
    import space_p3_issuer_create as issuer
    if type(phase) is not issuer.Session or phase.fence.context!=cut.fence.context or phase.lock!=cut.lock or phase.namespace_uid!=cut.namespace_uid: raise ValueError()
    proof=issuer.verify_completed(phase); row,rev=cut.fence.ledger.read()
    if (phase.fence.ledger.read()!=(row,rev) or proof.get('schema')!='voice-dev-space-p3-issuer-v1'
            or proof.get('operation_id')!=cut.fence.context['operation_id'] or proof.get('source_sha')!=cut.fence.context['commit']
            or proof.get('namespace_uid')!=cut.namespace_uid or proof.get('scope')!='space-two-secret-create-only'): raise ValueError()
    return proof,json.dumps(proof['public_jwks'],sort_keys=True,separators=(',',':'),allow_nan=False).encode()


def _realtime_service(cut):
    entry=cut.journal.state['steps']['backend-config-service-voice-realtime']
    row,_=cut.fence.ledger.read(); record=row['backend_config_effects']['backend-config-service-voice-realtime']
    source=cut.journal.intent['backend_config_service']
    matches=[x for x in cut.admission['controllers'] if x.get('kind')=='Deployment' and x.get('metadata',{}).get('name')=='voice-realtime']
    if len(matches)!=1: raise ValueError()
    patch,target=adoption.plan_service(source,matches[0]['spec']['template']['metadata']['labels'])
    details={'operation_id':cut.fence.context['operation_id'],'invocation_nonce':cut.invocation,
             'source':source,'target_spec':target,'patch':patch}
    receipt=entry['receipt']
    if (entry.get('status')!='done' or entry.get('details')!=details or entry.get('details_sha256')!=writer_cut.digest(details)
            or record!={'status':'done','nonce':entry['nonce'],'details_sha256':entry['details_sha256'],
                       'receipt_sha256':writer_cut.digest(receipt)}): raise ValueError()
    actual=cut.client.call('get','service','voice-realtime','-o','json'); meta=actual['metadata']
    if (actual.get('kind')!='Service' or actual.get('apiVersion')!='v1' or actual.get('spec')!=target
            or meta.get('name')!='voice-realtime' or meta.get('namespace')!='voice-dev'
            or meta.get('uid')!=receipt['uid'] or meta.get('resourceVersion')!=receipt['resourceVersion']
            or meta.get('deletionTimestamp') or receipt['spec_sha256']!=writer_cut.digest(target)): raise ValueError()
    return {'name':'voice-realtime','namespace':'voice-dev','uid':meta['uid'],'resourceVersion':meta['resourceVersion'],
            'selector':copy.deepcopy(target['selector']),'ports':copy.deepcopy(target['ports']),
            'type':target.get('type','ClusterIP'),'clusterIP':target['clusterIP']}


def collect(cut,journal,session,migration,space_phase,publisher_phase,source_root,token):
    """Return a renderer input derived from actual producer proofs, not caller hashes."""
    try:
        import realtime_jwks_publisher as publisher
        import realtime_config_transition_service as chat_service
        if not isinstance(token,str) or not token: raise ValueError()
        cut._guard(); before,revision=cut.fence.ledger.read()
        p3=inputs.observe(cut,session,migration,source_root)
        public_plan,space_jwks=_space(cut,space_phase,token)
        if public_plan.get('schema') in ('voice-dev-space-p3-issuer-v1','voice-dev-space-delegated-v1'):
            tls=p3['public_sources']['voice-space-media-server-tls']
            ca=p3['public_sources']['voice-media-ca']
            if (any(public_plan['tls'][k]!=tls[k] for k in ('uid','resourceVersion'))
                    or public_plan['tls']['certificate_sha256']!=tls['der_sha256']
                    or public_plan['media_ca']!=ca): raise ValueError()
        published=publisher.verify_publisher(publisher_phase)
        if (publisher_phase.fence.context!=cut.fence.context or publisher_phase.lock!=cut.lock
                or publisher_phase.namespace_uid!=cut.namespace_uid or published['operation_id']!=cut.fence.context['operation_id']
                or publisher_phase.fence.ledger.read()!=(before,revision)): raise ValueError()
        redis_proof=redis.observe(cut.fence,cut.client,cut.lock,cut.namespace_uid)
        chat=chat_service.verify_chat_service(cut,journal)
        realtime=_realtime_service(cut)
        config=cut.client.call('get','configmap','voice-app-config','-o','json'); cm=config['metadata']; config_data=config.get('data',{})
        if (config.get('kind')!='ConfigMap' or config.get('apiVersion')!='v1' or cm.get('name')!='voice-app-config'
                or cm.get('namespace')!='voice-dev' or cm.get('deletionTimestamp') or not isinstance(config_data,dict)
                or config_data.get('REDIS_ADDR')!='voice-redis:6379' or not cm.get('uid') or not cm.get('resourceVersion')): raise ValueError()
        credential=migration.intent['realtime_migration']['credential']
        operation=cut.fence.context['operation_id']
        if (not isinstance(operation,str) or not re.fullmatch(r'media-c[12]-[0-9a-f]{32}',operation)
                or credential.get('name')!='voice-realtime-db-'+operation[-32:]): raise ValueError()
        cut._guard(); secret=cut.client.call('get','secret',credential['name'],'-o','json'); cut._guard(); sm=secret['metadata']
        if (secret.get('apiVersion')!='v1' or secret.get('kind')!='Secret' or sm.get('name')!=credential['name']
                or sm.get('namespace')!='voice-dev' or sm.get('deletionTimestamp')
                or sm.get('uid')!=credential['uid'] or sm.get('resourceVersion')!=credential['resourceVersion']
                or secret.get('immutable') is not True or secret.get('type')!='Opaque' or set(secret.get('data',{}))!={'DATABASE_URL'}): raise ValueError()
        assets={'voice-app-config':{'kind':'ConfigMap','namespace':'voice-dev','uid':cm['uid'],'resourceVersion':cm['resourceVersion'],'keys':sorted(config_data)},
                credential['name']:{'kind':'Secret','namespace':'voice-dev','uid':sm['uid'],'resourceVersion':sm['resourceVersion'],
                                    'keys':['DATABASE_URL'],'immutable':True}}
        values={}
        for name,identity in p3['objects'].items():
            obj=transport.get(identity['kind'],name); meta=obj['metadata']
            if meta.get('uid')!=identity['uid'] or meta.get('resourceVersion')!=identity['resourceVersion']: raise ValueError()
            data=obj['data']; item={'kind':identity['kind'],'namespace':'voice-dev','uid':meta['uid'],
                'resourceVersion':meta['resourceVersion'],'keys':sorted(data)}
            if identity['kind']=='ConfigMap': item['immutable']=obj.get('immutable')
            assets[name]=item; values[name]=data
        def secret_value(name,key): return base64.b64decode(values[name][key],validate=True)
        raw_space=transport.public_certificate('voice-space-media-server-tls','tls.crt').split(b'\n',2)
        if len(raw_space)!=3: raise ValueError()
        expected_space=p3['public_sources']['voice-space-media-server-tls']
        if raw_space[0].decode()!=expected_space['uid'] or raw_space[1].decode()!=expected_space['resourceVersion']: raise ValueError()
        public_values={'realtime.json':values['voice-p3-principal-public']['realtime.json'].encode(),
            'space-jwks.json':space_jwks,
            'space-jwks-tls.crt':base64.b64decode(raw_space[2],validate=True),
            'realtime-tls.crt':secret_value('voice-realtime-p3-tls','tls.crt'),
            'chat-tls.crt':secret_value('voice-chat-realtime-p3-tls','tls.crt'),
            'jwks-tls.crt':secret_value('voice-p3-principal-jwks-tls','tls.crt')}
        for key in ('chat-server-ca.crt','space-client-ca.crt','space-jwks-ca.crt','realtime-client-ca.crt','realtime-jwks-ca.crt'):
            public_values[key]=secret_value('voice-p3-principal-trust',key)
        if set(public_values)!=adoption.PUBLIC_HASHES: raise ValueError()
        result={'assets':assets,'schema':{'database':'realtime_db','migration':'000001_admission',
                    'evidence_sha256':writer_cut.digest(p3['migration'])},
                'public_hashes':{k:hashlib.sha256(v).hexdigest() for k,v in public_values.items()},'service':realtime,
                'transport_evidence_sha256':writer_cut.digest({'publisher':published,'redis':redis_proof,'chat_service':chat['receipt'],
                                                               'space_plan':public_plan})}
        if (inputs.observe(cut,session,migration,source_root)!=p3 or _space(cut,space_phase,token)!=(public_plan,space_jwks)
                or publisher.verify_publisher(publisher_phase)!=published or redis.observe(cut.fence,cut.client,cut.lock,cut.namespace_uid)!=redis_proof
                or chat_service.verify_chat_service(cut,journal)!=chat or _realtime_service(cut)!=realtime): raise ValueError()
        latest=cut.client.call('get','configmap','voice-app-config','-o','json')
        if latest.get('metadata',{}).get('uid')!=cm['uid'] or latest.get('metadata',{}).get('resourceVersion')!=cm['resourceVersion'] or latest.get('data')!=config_data: raise ValueError()
        cut._guard(); latest_secret=cut.client.call('get','secret',credential['name'],'-o','json'); cut._guard()
        if latest_secret.get('metadata',{}).get('uid')!=sm['uid'] or latest_secret.get('metadata',{}).get('resourceVersion')!=sm['resourceVersion']: raise ValueError()
        cut._guard(); after,end=cut.fence.ledger.read()
        if after!=before or end!=revision or publisher_phase.fence.ledger.read()!=(before,revision): raise ValueError()
        return result
    except Exception: raise AuthorityError('P3 configuration authority unavailable') from None
