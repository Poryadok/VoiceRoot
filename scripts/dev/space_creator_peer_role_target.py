"""Pure Role/User/Social peer target checks; not release admission."""
import copy
import json

import realtime_config_transition as base
import realtime_p3_adoption as env
import space_creator_peer_target as peer
import space_creator_runtime_profile as creator


def render_role(source,config_keys):
    """Plan Role's protected Space client and existing principal listener."""
    target,pod,container,entries=creator._container(source,'role',config_keys)
    jwks=[entry for entry in entries if entry.get('name')=='S2S_JWKS_URLS_JSON']
    if (len(jwks)!=1 or set(jwks[0])!={'name','value'} or not isinstance(jwks[0]['value'],str) or
            'S2S_JWKS_URLS_JSON' in config_keys):raise ValueError('Role peer JWKS unavailable')
    endpoints=json.loads(jwks[0]['value'],object_pairs_hook=peer._unique)
    if not isinstance(endpoints,dict) or not {'gameintegration','voice'}<=set(endpoints):
        raise ValueError('Role issuer set unavailable')
    endpoints['space']='https://voice-space:8443/.well-known/jwks.json'
    allowed={'space','gameintegration','voice'} | ({'chat'} if 'chat' in endpoints else set()) | ({'messaging'} if 'messaging' in endpoints else set())
    for flag,issuer in (('ROLE_BOT_LIFECYCLE_ENABLED','bot'),('ROLE_DELEGATED_USER_ENABLED','gateway'),
                        ('ROLE_AUTHORITY_SOURCE_ENABLED','federation')):
        matches=[entry for entry in entries if entry.get('name')==flag]
        if flag in config_keys or len(matches)>1:raise ValueError('Role issuer flag unavailable')
        if matches:
            if set(matches[0])!={'name','value'} or matches[0]['value'] not in ('true','false'):
                raise ValueError('Role issuer flag invalid')
            if matches[0]['value']=='true':allowed.add(issuer)
    if set(endpoints)!=allowed:raise ValueError('Role issuer set invalid')
    for name,url in endpoints.items():peer._https(name,url)
    jwks[0]['value']=json.dumps(endpoints,sort_keys=True,separators=(',',':'))
    groups=[
        [env._env('ROLE_PRINCIPAL_GRPC_LISTEN',':9091'),
         env._env('ROLE_PRINCIPAL_TLS_CERT_FILE','/etc/voice/media/server/tls.crt'),
         env._env('ROLE_PRINCIPAL_TLS_KEY_FILE','/etc/voice/media/server/tls.key'),
         env._env('ROLE_PRINCIPAL_CLIENT_CA_FILE','/etc/voice/media/ca/ca.crt')],
        [env._env('ROLE_SPACE_MEMBER_AUTHORITY_GRPC_ADDR','voice-space:9099'),
         env._env('ROLE_SPACE_MEMBER_AUTHORITY_TLS_SERVER_NAME','voice-space'),
         env._env('ROLE_SPACE_MEMBER_AUTHORITY_TLS_CA_FILE','/etc/voice/media/ca/ca.crt'),
         env._env('ROLE_SPACE_MEMBER_AUTHORITY_CLIENT_CERT_FILE','/etc/voice/media/client/tls.crt'),
         env._env('ROLE_SPACE_MEMBER_AUTHORITY_CLIENT_KEY_FILE','/etc/voice/media/client/tls.key')],
        [env._env('ROLE_PRINCIPAL_SIGNING_KEYS_DIR','/etc/voice/media/signing'),
         env._ref('ROLE_PRINCIPAL_ACTIVE_KID','secret','voice-role-principal-signing','active-kid')],
        [env._env('ROLE_PRINCIPAL_JWKS_HTTPS_LISTEN',':8443'),
         env._env('ROLE_PRINCIPAL_JWKS_TLS_CERT_FILE','/etc/voice/media/server/tls.crt'),
         env._env('ROLE_PRINCIPAL_JWKS_TLS_KEY_FILE','/etc/voice/media/server/tls.key')],
        [env._ref('ROLE_PRINCIPAL_REPLAY_REDIS_ADDR','configMap','voice-app-config','REDIS_ADDR')],
        [env._env('S2S_JWKS_CA_FILE','/etc/voice/media/ca/jwks-ca.crt')],
    ]
    for group in groups:creator._ensure_exact(entries,group,config_keys)
    if ('ROLE_PRINCIPAL_REPLAY_REDIS_PASSWORD' in config_keys or any(
            entry.get('name')=='ROLE_PRINCIPAL_REPLAY_REDIS_PASSWORD' for entry in entries)):
        raise ValueError('unknown Role Redis authentication')
    creator._mount(pod,container,'media-server','/etc/voice/media/server','voice-role-media-server-tls',('tls.crt','tls.key'))
    creator._mount(pod,container,'media-client','/etc/voice/media/client','voice-role-media-client-tls',('tls.crt','tls.key'))
    creator._mount(pod,container,'media-ca','/etc/voice/media/ca','voice-media-ca',('ca.crt','jwks-ca.crt'))
    creator._mount(pod,container,'media-signing','/etc/voice/media/signing','voice-role-principal-signing',('current.pem','next.pem'))
    for number,name in ((9091,'role-principal'),(8443,'principal-jwks')):
        found=[]
        for item in pod['containers']+pod.get('initContainers',[]):
            for port in item.get('ports',[]):
                if port.get('containerPort')==number or port.get('name')==name:
                    if (item is not container or port.get('containerPort')!=number or port.get('protocol','TCP')!='TCP' or
                            set(port)-{'name','containerPort','protocol'}):raise ValueError('Role listener port conflict')
                    found.append(port)
        if len(found)>1:raise ValueError('Role listener port conflict')
        if not found:container.setdefault('ports',[]).append({'name':name,'containerPort':number,'protocol':'TCP'})
    security=pod.setdefault('securityContext',{})
    if 'fsGroup' in security and security['fsGroup']!=65532:raise ValueError('Role group conflict')
    security['fsGroup']=65532
    return base._patch(source,target),target


def verify_service(source,deployment,port):
    """Check one fixed peer Service and target selector without claiming dataplane health."""
    name=deployment.get('metadata',{}).get('name')
    expected={'voice-role':{9091,8443},'voice-user':{9090},'voice-social':{9090},'voice-auth':{9092}}
    if name not in expected or port not in expected[name]:raise ValueError('peer service invalid')
    meta=source.get('metadata',{}); spec=source.get('spec',{})
    labels=deployment.get('spec',{}).get('template',{}).get('metadata',{}).get('labels',{})
    selector=spec.get('selector'); ports=spec.get('ports')
    if ((source.get('apiVersion'),source.get('kind'),meta.get('name'),meta.get('namespace'))!=('v1','Service',name,'voice-dev') or
            any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion')) or meta.get('deletionTimestamp') or
            spec.get('type','ClusterIP')!='ClusterIP' or not isinstance(spec.get('clusterIP'),str) or spec['clusterIP'] in ('','None') or
            not isinstance(selector,dict) or not selector or not isinstance(labels,dict) or
            any(not isinstance(k,str) or not isinstance(v,str) for k,v in selector.items()) or
            any(labels.get(k)!=v for k,v in selector.items()) or not isinstance(ports,list)):
        raise ValueError('peer service identity unavailable')
    matched=[item for item in ports if isinstance(item,dict) and item.get('port')==port]
    if len(matched)!=1 or matched[0].get('protocol','TCP')!='TCP' or matched[0].get('targetPort',port)!=port:
        raise ValueError('peer service route unavailable')
    return {'uid':meta['uid'],'resourceVersion':meta['resourceVersion'],'port':port,'selector':dict(selector)}


def render_role_service(source,deployment):
    """Plan Role JWKS port in one UID/RV/full-spec Service CAS."""
    verify_service(source,deployment,9091)
    target=copy.deepcopy(source['spec'])
    desired={'name':'principal-jwks','port':8443,'targetPort':8443,'protocol':'TCP'}
    matches=[port for port in target['ports'] if port.get('name')=='principal-jwks' or
             port.get('port')==8443 or port.get('targetPort') in (8443,'principal-jwks')]
    if matches and matches!=[desired]:raise ValueError('Role JWKS Service port conflict')
    if not matches:target['ports'].append(desired)
    return base._patch(source,target),target
