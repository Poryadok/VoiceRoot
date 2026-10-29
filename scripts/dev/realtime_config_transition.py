"""Fixed P3 configuration renderers; rendering is not admission authority."""
import copy
import re

import realtime_p3_adoption as adoption


def verify_chat_service(cut,config_journal):
    from realtime_config_transition_service import verify_chat_service as verify
    return verify(cut,config_journal)


def verify_deployment_successor(cut,config_journal,service):
    from realtime_config_transition_deployment import verify_deployment_successor as verify
    return verify(cut,config_journal,service)


def read_realtime_route(cut,config_journal):
    from realtime_config_transition_deployment import read_realtime_route as read
    return read(cut,config_journal)


def _base(source, service):
    meta = source.get('metadata', {})
    if ((source.get('apiVersion'),source.get('kind'),meta.get('name'),meta.get('namespace'))
            != ('apps/v1','Deployment','voice-'+service,'voice-dev')
            or any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion'))
            or type(source.get('spec',{}).get('replicas')) is not int or source['spec']['replicas'] != 0):
        raise ValueError('drained Deployment required')


def _patch(source,target):
    meta = source['metadata']
    return [{'op':'test','path':'/metadata/uid','value':meta['uid']},
            {'op':'test','path':'/metadata/resourceVersion','value':meta['resourceVersion']},
            {'op':'test','path':'/spec','value':copy.deepcopy(source['spec'])},
            {'op':'replace','path':'/spec','value':target}]


def render_chat_service(source,expected_pod_labels):
    """Plan the fixed Chat lifecycle port; this does not apply or admit it."""
    meta=source.get('metadata',{}); spec=source.get('spec',{})
    if ((source.get('apiVersion'),source.get('kind'),meta.get('name'),meta.get('namespace'))
            != ('v1','Service','voice-chat','voice-dev')
            or any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion'))
            or spec.get('type','ClusterIP')!='ClusterIP' or not isinstance(spec.get('clusterIP'),str)
            or spec['clusterIP'] in ('','None')): raise ValueError('Chat Service identity unavailable')
    selector=spec.get('selector'); ports=spec.get('ports')
    if (not isinstance(expected_pod_labels,dict) or not isinstance(selector,dict) or not selector
            or any(expected_pod_labels.get(k)!=v for k,v in selector.items())
            or not isinstance(ports,list) or any(not isinstance(p,dict) for p in ports)
            or any(p.get('name')=='realtime-lifecycle' or p.get('port')==9099 for p in ports)):
        raise ValueError('Chat Service lifecycle port unavailable')
    target=copy.deepcopy(spec)
    target['ports'].append({'name':'realtime-lifecycle','port':9099,'targetPort':9099,'protocol':'TCP'})
    return _patch(source,target),target


def render_space_service(source,expected_pod_labels):
    """Plan only the documented Space public JWKS Service port."""
    meta=source.get('metadata',{}); spec=source.get('spec',{})
    if ((source.get('apiVersion'),source.get('kind'),meta.get('name'),meta.get('namespace'))
            != ('v1','Service','voice-space','voice-dev')
            or any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion'))
            or spec.get('type','ClusterIP')!='ClusterIP' or not isinstance(spec.get('clusterIP'),str)
            or spec['clusterIP'] in ('','None')): raise ValueError('Space Service identity unavailable')
    selector=spec.get('selector'); ports=spec.get('ports')
    if (not isinstance(expected_pod_labels,dict) or not isinstance(selector,dict) or not selector
            or any(expected_pod_labels.get(k)!=v for k,v in selector.items())
            or not isinstance(ports,list) or any(not isinstance(p,dict) for p in ports)):
        raise ValueError('Space Service JWKS port unavailable')
    target=copy.deepcopy(spec)
    desired={'name':'principal-jwks','port':8443,'targetPort':8443,'protocol':'TCP'}
    matches=[p for p in ports if p.get('name')=='principal-jwks' or p.get('port')==8443]
    if matches and matches!=[desired]: raise ValueError('Space Service JWKS port conflict')
    if not matches: target['ports'].append(desired)
    return _patch(source,target),target


def render_space_creator_service(source,expected_pod_labels):
    """Plan the single Space Service target for JWKS and Creator traffic."""
    _,target=render_space_service(source,expected_pod_labels)
    desired={'name':'gateway-lc','port':9096,'targetPort':9096,'protocol':'TCP'}
    matches=[p for p in target['ports'] if p.get('name')=='gateway-lc' or
             p.get('port')==9096 or p.get('targetPort') in (9096,'gateway-lc')]
    if matches and matches!=[desired]: raise ValueError('Space Service Creator port conflict')
    if not matches:target['ports'].append(desired)
    return _patch(source,target),target


def render_realtime(source,receipts,operation_id):
    """Render only; all receipt identity and authority checks belong to the phase."""
    _base(source,'realtime')
    staged = copy.deepcopy(source); staged['spec']['replicas'] = 1
    containers = [c for c in source['spec']['template']['spec']['containers'] if c.get('name') == 'realtime']
    if len(containers) != 1 or not isinstance(containers[0].get('image'),str): raise ValueError('container required')
    original_image = containers[0]['image']
    _, target, _ = adoption.plan(staged,'source.invalid/realtime@sha256:'+'0'*64,receipts,operation_id=operation_id)
    target['replicas'] = 0
    next(c for c in target['template']['spec']['containers'] if c.get('name') == 'realtime')['image'] = original_image
    return _patch(source,target), target


def render_chat(source,app_config_keys):
    """Render the canonical Chat listener; caller rechecks actual config references."""
    _base(source,'chat')
    if not isinstance(app_config_keys,list) or any(not isinstance(k,str) for k in app_config_keys): raise ValueError()
    target = copy.deepcopy(source['spec']); pod = target['template']['spec']
    containers = [c for c in pod['containers'] if c.get('name') == 'chat']
    if len(containers) != 1: raise ValueError()
    container = containers[0]
    env = [
        adoption._env('CHAT_REALTIME_LIFECYCLE_GRPC_LISTEN',':9099'),
        adoption._env('CHAT_REALTIME_LIFECYCLE_TLS_CERT_FILE','/etc/voice/realtime-p3/tls/tls.crt'),
        adoption._env('CHAT_REALTIME_LIFECYCLE_TLS_KEY_FILE','/etc/voice/realtime-p3/tls/tls.key'),
        adoption._env('CHAT_REALTIME_LIFECYCLE_CLIENT_CA_FILE','/etc/voice/realtime-p3/trust/realtime-client-ca.crt'),
        adoption._env('CHAT_REALTIME_PRINCIPAL_JWKS_URL','https://voice-p3-principal-jwks.voice-dev.svc:8443/realtime.json'),
        adoption._env('CHAT_REALTIME_PRINCIPAL_JWKS_CA_FILE','/etc/voice/realtime-p3/trust/realtime-jwks-ca.crt'),
        adoption._ref('CHAT_REALTIME_PRINCIPAL_REPLAY_REDIS_ADDR','configMap','voice-app-config','REDIS_ADDR'),
        adoption._ref('CHAT_REALTIME_PRINCIPAL_REPLAY_REDIS_PASSWORD','secret','voice-p3-principal-replay','password')]
    forbidden = {'CHAT_REALTIME_PRINCIPAL_JWKS_CLIENT_CERT_FILE','CHAT_REALTIME_PRINCIPAL_JWKS_CLIENT_KEY_FILE'}
    names = {e['name'] for e in env} | forbidden
    if names & set(app_config_keys): raise ValueError('conflicting config keys')
    if any(x != {'configMapRef':{'name':'voice-app-config'}} for x in container.get('envFrom',[])): raise ValueError('unknown envFrom')
    existing = container.setdefault('env',[])
    if (not isinstance(existing,list) or any(not isinstance(x,dict) or not isinstance(x.get('name'),str) for x in existing)
            or len({x['name'] for x in existing}) != len(existing) or names & {x['name'] for x in existing}): raise ValueError('env collision')
    mounts = [{'name':'realtime-p3-server','mountPath':'/etc/voice/realtime-p3/tls','readOnly':True},
              {'name':'realtime-p3-trust','mountPath':'/etc/voice/realtime-p3/trust','readOnly':True}]
    current = container.setdefault('volumeMounts',[])
    if any(a.get('name') in {m['name'] for m in mounts} or any(
            a.get('mountPath','') == b['mountPath'] or a.get('mountPath','').startswith(b['mountPath']+'/')
            or b['mountPath'].startswith(a.get('mountPath','')+'/') for b in mounts) for a in current): raise ValueError('mount collision')
    volumes = [{'name':'realtime-p3-server','secret':{'secretName':'voice-chat-realtime-p3-tls','defaultMode':288}},
               {'name':'realtime-p3-trust','secret':{'secretName':'voice-p3-principal-trust','defaultMode':288,
                  'items':[{'key':x,'path':x} for x in ('realtime-client-ca.crt','realtime-jwks-ca.crt')]}}]
    prior = pod.setdefault('volumes',[])
    if any(v.get('name') in {x['name'] for x in volumes} for v in prior): raise ValueError('volume collision')
    security = pod.setdefault('securityContext',{})
    if 'fsGroup' in security and security['fsGroup'] != 65532: raise ValueError('fsGroup collision')
    security['fsGroup'] = 65532; existing.extend(env); current.extend(mounts); prior.extend(volumes)
    return _patch(source,target), target


def render_space(source,app_config_keys):
    """Add only the documented Space signer and HTTPS JWKS identity."""
    _base(source,'space')
    if not isinstance(app_config_keys,list) or any(not isinstance(k,str) for k in app_config_keys): raise ValueError()
    target=copy.deepcopy(source['spec']); pod=target['template']['spec']
    containers=[c for c in pod['containers'] if c.get('name')=='space']
    if len(containers)!=1: raise ValueError()
    container=containers[0]
    env=[adoption._env('SPACE_PRINCIPAL_SIGNING_KEYS_DIR','/etc/voice/media/signing'),
         adoption._ref('SPACE_PRINCIPAL_ACTIVE_KID','secret','voice-space-principal-signing','active-kid'),
         adoption._env('SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE','/etc/voice/media/server/tls.key'),
         adoption._env('SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE','/etc/voice/media/server/tls.crt'),
         adoption._env('SPACE_PRINCIPAL_JWKS_HTTPS_LISTEN',':8443')]
    names={e['name'] for e in env}
    if names & set(app_config_keys) or any(x!={'configMapRef':{'name':'voice-app-config'}} for x in container.get('envFrom',[])): raise ValueError()
    existing=container.setdefault('env',[])
    if (not isinstance(existing,list) or any(not isinstance(x,dict) or not isinstance(x.get('name'),str) for x in existing)
            or len({x['name'] for x in existing})!=len(existing) or names & {x['name'] for x in existing}): raise ValueError()
    mounts=[{'name':'media-signing','mountPath':'/etc/voice/media/signing','readOnly':True},
            {'name':'media-server','mountPath':'/etc/voice/media/server','readOnly':True}]
    existing_mounts=container.setdefault('volumeMounts',[])
    if any(m.get('name') in {x['name'] for x in mounts} or any(
            m.get('mountPath','')==x['mountPath'] or m.get('mountPath','').startswith(x['mountPath']+'/')
            or x['mountPath'].startswith(m.get('mountPath','')+'/') for x in mounts) for m in existing_mounts): raise ValueError()
    volumes=[{'name':'media-signing','secret':{'secretName':'voice-space-principal-signing','defaultMode':288,
        'items':[{'key':k,'path':k} for k in ('current.pem','next.pem')]}},
        {'name':'media-server','secret':{'secretName':'voice-space-media-server-tls','defaultMode':288,
        'items':[{'key':k,'path':k} for k in ('tls.crt','tls.key')]}}]
    existing_volumes=pod.setdefault('volumes',[])
    if any(v.get('name') in {x['name'] for x in volumes} for v in existing_volumes): raise ValueError()
    security=pod.setdefault('securityContext',{})
    if 'fsGroup' in security and security['fsGroup']!=65532: raise ValueError()
    security['fsGroup']=65532; existing.extend(env); existing_mounts.extend(mounts); existing_volumes.extend(volumes)
    return _patch(source,target),target


def verify_existing(source, predecessor_source, predecessor_receipts, predecessor_operation, service, app_config_keys):
    """Check a C1-rendered config exactly, allowing only separately admitted images.

    The caller must authenticate the predecessor and current WriterCut source;
    this pure comparison is not ownership or C2 admission authority.
    """
    if not isinstance(predecessor_operation,str) or not re.fullmatch(r'media-c1-[0-9a-f]{32}',predecessor_operation): raise ValueError()
    _base(source,service)
    if service == 'realtime':
        _, expected = render_realtime(predecessor_source,predecessor_receipts,predecessor_operation)
    elif service == 'chat':
        _, expected = render_chat(predecessor_source,app_config_keys)
    elif service == 'space':
        _, expected = render_space(predecessor_source,app_config_keys)
    else: raise ValueError()
    current = source['spec']; old = expected['template']['spec']['containers']; new = current['template']['spec']['containers']
    if (not isinstance(old,list) or not isinstance(new,list)
            or any(not isinstance(c,dict) or not isinstance(c.get('name'),str) or not isinstance(c.get('image'),str) for c in old+new)
            or len({c['name'] for c in old}) != len(old) or len({c['name'] for c in new}) != len(new)
            or [c['name'] for c in old] != [c['name'] for c in new]): raise ValueError()
    for previous,actual in zip(old,new): previous['image'] = actual['image']
    if expected != current: raise ValueError('existing configuration differs')
    return copy.deepcopy(current)
