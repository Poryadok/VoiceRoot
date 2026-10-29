"""Reconstruct P3 configuration inputs from concrete fenced phase objects."""
import base64
import copy
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


def _space(cut,phase,token):
    if type(phase) is space_plan.SpacePlanPhase:
        proof=space_plan.verify(phase,cut,token)
        return proof,base64.b64decode(proof['observation']['jwks_bytes'],validate=True)
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
        if public_plan.get('schema')=='voice-dev-space-p3-issuer-v1':
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
