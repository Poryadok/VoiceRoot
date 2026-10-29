"""DEV protected Gateway/Space Creator profile; renderer is not admission."""
import copy
from pathlib import PurePosixPath

import realtime_config_transition as base
import realtime_p3_adoption as env


PORT=9096
GATEWAY_JWKS='https://api-dev.comrade.click/.well-known/voice-principal-jwks.json'


def _check_path(mounts,path,exclude=None):
    def valid(value):
        return (isinstance(value,str) and value.startswith('/') and not value.startswith('//') and str(PurePosixPath(value))==value and
                '..' not in PurePosixPath(value).parts)
    if not valid(path): raise ValueError('invalid mount path')
    for item in mounts:
        if item.get('name')==exclude: continue
        other=item.get('mountPath')
        if not valid(other) or other=='/' or other==path or other.startswith(path.rstrip('/')+'/') or path.startswith(other.rstrip('/')+'/'):
            raise ValueError('mount collision')


def _mount(pod,container,name,path,secret,keys):
    mounts=container.setdefault('volumeMounts',[]); volumes=pod.setdefault('volumes',[])
    desired={'name':name,'mountPath':path,'readOnly':True}
    volume={'name':name,'secret':{'secretName':secret,'defaultMode':288,
            'items':[{'key':k,'path':k} for k in keys]}}
    existing=[x for x in mounts if x.get('name')==name]
    found=[x for x in volumes if x.get('name')==name]
    if existing or found:
        if existing!=[desired] or found!=[volume]: raise ValueError('conflicting material mount')
        _check_path(mounts,path,exclude=name)
        return
    _check_path(mounts,path)
    mounts.append(desired); volumes.append(volume)


def _container(source, service, config_keys):
    base._base(source,service)
    if not isinstance(config_keys,list) or any(not isinstance(x,str) for x in config_keys): raise ValueError()
    target=copy.deepcopy(source['spec']); pod=target['template']['spec']
    containers=[x for x in pod['containers'] if x.get('name')==service]
    if len(containers)!=1: raise ValueError()
    container=containers[0]
    mounts=container.get('volumeMounts',[])
    if (not isinstance(mounts,list) or any(not isinstance(x,dict) or not isinstance(x.get('name'),str) for x in mounts) or
            len({x['name'] for x in mounts})!=len(mounts)): raise ValueError('mount identity collision')
    if any(x!={'configMapRef':{'name':'voice-app-config'}} for x in container.get('envFrom',[])): raise ValueError()
    entries=container.setdefault('env',[])
    if (not isinstance(entries,list) or any(not isinstance(x,dict) or not isinstance(x.get('name'),str) for x in entries)
            or len({x['name'] for x in entries})!=len(entries)): raise ValueError()
    return target,pod,container,entries


def _append(entries, values, config_keys):
    names={x['name'] for x in values}
    if len(names)!=len(values) or names & set(config_keys) or names & {x['name'] for x in entries}: raise ValueError('environment collision')
    entries.extend(values)


def _ensure_exact(entries, values, config_keys):
    names={x['name'] for x in values}
    existing=[x for x in entries if x['name'] in names]
    if names & set(config_keys): raise ValueError('shared config collision')
    if existing:
        if len(existing)!=len(values) or {x['name']:x for x in existing}!={x['name']:x for x in values}:
            raise ValueError('partial or different existing profile')
    else:
        entries.extend(values)


def _whole_profile(pod, container, entries, prefixes, port=False):
    """Only a wholly absent or wholly present Creator transport may be rendered."""
    names={entry['name'] for entry in entries}
    selected={name for name in names if any(name.startswith(p) for p in prefixes)}
    mount_names={item['name'] for item in container.get('volumeMounts',[])}
    volume_names={item.get('name') for item in pod.get('volumes',[])}
    parts=[bool(selected), 'creator-ca' in mount_names, 'creator-client' in mount_names,
           'creator-ca' in volume_names, 'creator-client' in volume_names]
    if port:
        parts.append(any(p.get('name')=='gateway-lc' for p in container.get('ports',[])))
    if any(parts) and not all(parts): raise ValueError('partial existing profile')
    return all(parts)


def render_space(source,config_keys):
    """Render one drained Space target; service 9096 is separately guarded."""
    target,pod,container,entries=_container(source,'space',config_keys)
    present=_whole_profile(pod,container,entries,('SPACE_GATEWAY_', 'SPACE_CREATOR_AUTH_'),port=True)
    if present and (pod.get('securityContext',{}).get('fsGroup')!=65532 or not any(
            e['name']=='ROLE_PRINCIPAL_GRPC_ADDR' for e in entries)):
        raise ValueError('partial existing profile')
    required=[env._env('SPACE_PRINCIPAL_SIGNING_KEYS_DIR','/etc/voice/media/signing'),
              env._ref('SPACE_PRINCIPAL_ACTIVE_KID','secret','voice-space-principal-signing','active-kid')]
    if any(x not in entries for x in required): raise ValueError('Space signer missing')
    _mount(pod,container,'media-server','/etc/voice/media/server','voice-space-media-server-tls',('tls.crt','tls.key'))
    _mount(pod,container,'media-signing','/etc/voice/media/signing','voice-space-principal-signing',('current.pem','next.pem'))
    _mount(pod,container,'creator-ca','/etc/voice/creator/ca','voice-media-ca',('ca.crt','jwks-ca.crt'))
    _mount(pod,container,'creator-client','/etc/voice/creator/client','voice-space-media-client-tls',('tls.crt','tls.key'))
    values=[env._env('SPACE_GATEWAY_LIFECYCLE_GRPC_LISTEN',':9096'),
      env._env('SPACE_GATEWAY_LIFECYCLE_TLS_CERT_FILE','/etc/voice/media/server/tls.crt'),
      env._env('SPACE_GATEWAY_LIFECYCLE_TLS_KEY_FILE','/etc/voice/media/server/tls.key'),
      env._env('SPACE_GATEWAY_LIFECYCLE_CLIENT_CA_FILE','/etc/voice/creator/ca/ca.crt'),
      env._env('SPACE_GATEWAY_PRINCIPAL_JWKS_URL',GATEWAY_JWKS),
      env._env('SPACE_GATEWAY_PRINCIPAL_JWKS_CA_FILE','/etc/voice/creator/ca/jwks-ca.crt'),
      env._ref('SPACE_GATEWAY_PRINCIPAL_REDIS_ADDR','configMap','voice-app-config','REDIS_ADDR'),
      env._env('SPACE_CREATOR_AUTH_GRPC_ADDR','voice-auth:9092'),
      env._env('SPACE_CREATOR_AUTH_TLS_SERVER_NAME','voice-auth'),
      env._env('SPACE_CREATOR_AUTH_TLS_CA_FILE','/etc/voice/creator/ca/ca.crt'),
      env._env('SPACE_CREATOR_AUTH_CLIENT_CERT_FILE','/etc/voice/creator/client/tls.crt'),
      env._env('SPACE_CREATOR_AUTH_CLIENT_KEY_FILE','/etc/voice/creator/client/tls.key')]
    _ensure_exact(entries,values,config_keys)
    _ensure_exact(entries,[env._env('ROLE_PRINCIPAL_GRPC_ADDR','voice-role:9091'),
        env._env('ROLE_PRINCIPAL_TLS_SERVER_NAME','voice-role'),
        env._env('ROLE_PRINCIPAL_TLS_CA_FILE','/etc/voice/creator/ca/ca.crt'),
        env._env('SPACE_ROLE_CLIENT_CERT_FILE','/etc/voice/creator/client/tls.crt'),
        env._env('SPACE_ROLE_CLIENT_KEY_FILE','/etc/voice/creator/client/tls.key')],config_keys)
    if 'SPACE_GATEWAY_PRINCIPAL_REDIS_PASSWORD' in config_keys or any(x['name']=='SPACE_GATEWAY_PRINCIPAL_REDIS_PASSWORD' for x in entries):
        raise ValueError('unknown Redis authentication')
    desired_port={'name':'gateway-lc','containerPort':PORT,'protocol':'TCP'}; found=[]
    for c in pod['containers']+pod.get('initContainers',[]):
        for p in c.get('ports',[]):
            if p.get('containerPort')==PORT or p.get('name')=='gateway-lc':
                if c is not container or p!=desired_port: raise ValueError('port collision')
                found.append(p)
    if len(found)>1: raise ValueError('port collision')
    if not found: container.setdefault('ports',[]).append(desired_port)
    security=pod.setdefault('securityContext',{})
    if 'fsGroup' in security and security['fsGroup']!=65532: raise ValueError()
    security['fsGroup']=65532
    return base._patch(source,target),target


def render_gateway(source,config_keys):
    target,pod,container,entries=_container(source,'gateway',config_keys)
    present=_whole_profile(pod,container,entries,('GATEWAY_SPACE_LIFECYCLE_',))
    if present and (pod.get('securityContext',{}).get('fsGroup')!=65532 or not all(
            any(e['name']==name for e in entries) for name in
            ('GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR','GATEWAY_PRINCIPAL_ACTIVE_KID','GATEWAY_SESSION_EPOCH_STRICT'))):
        raise ValueError('partial existing profile')
    _mount(pod,container,'creator-ca','/etc/voice/creator/ca','voice-media-ca',('ca.crt',))
    _mount(pod,container,'creator-client','/etc/voice/creator/client','voice-gateway-media-client-tls',('tls.crt','tls.key'))
    values=[env._env('GATEWAY_SPACE_LIFECYCLE_GRPC_ADDR','voice-space:9096'),
      env._env('GATEWAY_SPACE_LIFECYCLE_TLS_SERVER_NAME','voice-space'),
      env._env('GATEWAY_SPACE_LIFECYCLE_TLS_CA_FILE','/etc/voice/creator/ca/ca.crt'),
      env._env('GATEWAY_SPACE_LIFECYCLE_CLIENT_CERT_FILE','/etc/voice/creator/client/tls.crt'),
      env._env('GATEWAY_SPACE_LIFECYCLE_CLIENT_KEY_FILE','/etc/voice/creator/client/tls.key')]
    _ensure_exact(entries,values,config_keys)
    signer_names={'GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR','GATEWAY_PRINCIPAL_ACTIVE_KID'}
    existing=[x for x in entries if x['name'] in signer_names]
    if signer_names & set(config_keys): raise ValueError('unknown signer source')
    if not existing:
        _mount(pod,container,'gateway-signing','/etc/voice/gateway-principal/keys',
               'voice-gateway-principal-signing',('current.pem','next.pem'))
        entries.extend([env._env('GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR','/etc/voice/gateway-principal/keys'),
                        env._ref('GATEWAY_PRINCIPAL_ACTIVE_KID','secret','voice-gateway-principal-signing','active-kid')])
    else:
        mapping={x['name']:x for x in existing}
        if set(mapping)!=signer_names or mapping['GATEWAY_PRINCIPAL_ACTIVE_KID']!=env._ref(
                'GATEWAY_PRINCIPAL_ACTIVE_KID','secret','voice-gateway-principal-signing','active-kid'): raise ValueError('partial signer')
        directory=mapping['GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR'].get('value')
        mounts=[x for x in container.get('volumeMounts',[]) if x.get('mountPath')==directory and x.get('readOnly') is True]
        if not isinstance(directory,str) or len(mounts)!=1 or mounts[0]!={'name':mounts[0].get('name'),'mountPath':directory,'readOnly':True}: raise ValueError('signer mount unknown')
        _check_path(container.get('volumeMounts',[]),directory,exclude=mounts[0].get('name'))
        volumes=[x for x in pod.get('volumes',[]) if x.get('name')==mounts[0].get('name')]
        expected={'secretName':'voice-gateway-principal-signing','defaultMode':288,
                  'items':[{'key':k,'path':k} for k in ('current.pem','next.pem')]}
        if len(volumes)!=1 or volumes[0].get('secret')!=expected: raise ValueError('signer material unknown')
    _ensure_exact(entries,[env._env('GATEWAY_SESSION_EPOCH_STRICT','true')],config_keys)
    security=pod.setdefault('securityContext',{})
    if 'fsGroup' in security and security['fsGroup']!=65532: raise ValueError()
    security['fsGroup']=65532
    return base._patch(source,target),target
