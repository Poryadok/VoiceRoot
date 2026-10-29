"""Pure fixed Space member-observer target; this is not release admission."""
import copy
import json
import re
from urllib.parse import urlsplit

import realtime_config_transition as base
import realtime_p3_adoption as env
import space_creator_runtime_profile as creator


def _unique(pairs):
    result={}
    for key,value in pairs:
        if key in result:raise ValueError('duplicate issuer')
        result[key]=value
    return result


def _https(name,url):
    if (not isinstance(name,str) or not name.strip() or not isinstance(url,str) or
            any(ord(c)<=32 or ord(c)==127 for c in url) or re.search(r'%(?![0-9a-fA-F]{2})',url)):
        raise ValueError('invalid issuer')
    parsed=urlsplit(url); _=parsed.port
    if parsed.scheme!='https' or not parsed.hostname or parsed.username is not None or parsed.password is not None or parsed.fragment:
        raise ValueError('invalid issuer endpoint')


def render_space(source,config_keys):
    """Extend an already rendered Creator target with Role's member observer."""
    _,verified=creator.render_space(source,config_keys)
    if verified!=source['spec']:raise ValueError('complete Creator target required')
    target,pod,container,entries=creator._container(source,'space',config_keys)
    creator._mount(pod,container,'media-server','/etc/voice/media/server','voice-space-media-server-tls',('tls.crt','tls.key'))
    creator._mount(pod,container,'creator-ca','/etc/voice/creator/ca','voice-media-ca',('ca.crt','jwks-ca.crt'))
    creator._mount(pod,container,'creator-client','/etc/voice/creator/client','voice-space-media-client-tls',('tls.crt','tls.key'))
    jwks=[entry for entry in entries if entry.get('name')=='S2S_JWKS_URLS_JSON']
    if (len(jwks)>1 or 'S2S_JWKS_URLS_JSON' in config_keys or
            (jwks and (set(jwks[0])!={'name','value'} or not isinstance(jwks[0]['value'],str)))):
        raise ValueError('Space peer JWKS unavailable')
    endpoints=json.loads(jwks[0]['value'],object_pairs_hook=_unique) if jwks else {}
    if not isinstance(endpoints,dict):raise ValueError('Space peer JWKS unavailable')
    for name,url in endpoints.items():_https(name,url)
    endpoints['role']='https://voice-role:8443/.well-known/jwks.json'
    value=json.dumps(endpoints,sort_keys=True,separators=(',',':'))
    if jwks: jwks[0]['value']=value
    else: entries.append(env._env('S2S_JWKS_URLS_JSON',value))
    observer=any(entry.get('name','').startswith('SPACE_ROLE_OVERRIDE_PRINCIPAL_') for entry in entries)
    exposed=any(port.get('name')=='role-authority' or port.get('containerPort')==9099
                for item in pod['containers']+pod.get('initContainers',[]) for port in item.get('ports',[]))
    if observer!=exposed:raise ValueError('partial Space observer profile')
    values=[env._env('SPACE_ROLE_OVERRIDE_PRINCIPAL_GRPC_LISTEN',':9099'),
            env._env('SPACE_ROLE_OVERRIDE_PRINCIPAL_TLS_CERT_FILE','/etc/voice/media/server/tls.crt'),
            env._env('SPACE_ROLE_OVERRIDE_PRINCIPAL_TLS_KEY_FILE','/etc/voice/media/server/tls.key'),
            env._env('SPACE_ROLE_OVERRIDE_PRINCIPAL_CLIENT_CA_FILE','/etc/voice/creator/ca/ca.crt'),
            env._ref('SPACE_ROLE_OVERRIDE_PRINCIPAL_REPLAY_REDIS_ADDR','configMap','voice-app-config','REDIS_ADDR'),
            env._env('S2S_JWKS_CA_FILE','/etc/voice/creator/ca/jwks-ca.crt'),
            env._env('S2S_JWKS_CLIENT_CERT_FILE','/etc/voice/creator/client/tls.crt'),
            env._env('S2S_JWKS_CLIENT_KEY_FILE','/etc/voice/creator/client/tls.key')]
    creator._ensure_exact(entries,values,config_keys)
    if ('SPACE_ROLE_OVERRIDE_PRINCIPAL_REPLAY_REDIS_PASSWORD' in config_keys or any(
            entry.get('name')=='SPACE_ROLE_OVERRIDE_PRINCIPAL_REPLAY_REDIS_PASSWORD' for entry in entries)):
        raise ValueError('unknown observer Redis authentication')
    desired={'name':'role-authority','containerPort':9099,'protocol':'TCP'}; found=[]
    for item in pod['containers']+pod.get('initContainers',[]):
        for port in item.get('ports',[]):
            if port.get('name')=='role-authority' or port.get('containerPort')==9099:
                if item is not container or port!=desired:raise ValueError('Space observer port conflict')
                found.append(port)
    if len(found)>1:raise ValueError('Space observer port conflict')
    if not found:container.setdefault('ports',[]).append(desired)
    return base._patch(source,target),target


def render_service(source,labels):
    """Plan one Service CAS covering JWKS, Creator, and Role observer ports."""
    _,target=base.render_space_creator_service(source,labels)
    desired={'name':'role-authority','port':9099,'targetPort':9099,'protocol':'TCP'}
    matches=[port for port in target['ports'] if port.get('name')=='role-authority' or
             port.get('port')==9099 or port.get('targetPort') in (9099,'role-authority')]
    if matches and matches!=[desired]:raise ValueError('Space observer Service port conflict')
    if not matches:target['ports'].append(copy.deepcopy(desired))
    return base._patch(source,target),target
