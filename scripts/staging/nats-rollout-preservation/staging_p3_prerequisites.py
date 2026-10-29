"""Fixed STAGING P3 object contract; pure helpers do not grant authority."""
import base64
import copy
import ipaddress
import re
import socket
import ssl
import time
import source_plan
import gis_first_install


class PrerequisiteError(RuntimeError): pass


NAMESPACE='voice-staging'
ASSET_KEYS={
    'voice-realtime-principal-signing':frozenset(('current.pem','next.pem')),
    'voice-realtime-p3-tls':frozenset(('tls.crt','tls.key')),
    'voice-chat-realtime-p3-tls':frozenset(('tls.crt','tls.key')),
    'voice-p3-principal-jwks-tls':frozenset(('tls.crt','tls.key')),
    'voice-p3-principal-replay':frozenset(('password',)),
    'voice-p3-principal-trust':frozenset(('chat-server-ca.crt','space-client-ca.crt','space-jwks-ca.crt',
                                          'realtime-client-ca.crt','realtime-jwks-ca.crt')),
    'voice-p3-principal-public':frozenset(('realtime.json',)),
}
TLS_NAMES=frozenset(('voice-realtime-p3-tls','voice-chat-realtime-p3-tls','voice-p3-principal-jwks-tls'))
PUBLISHER=('ConfigMap/voice-p3-principal-publisher-config','Service/voice-p3-principal-jwks',
           'Deployment/voice-p3-principal-jwks')


def publisher_objects(source_rows,image,platform_digest):
    """Materialize the fixed source triple; callers must authenticate source/OCI."""
    try:
        rows=copy.deepcopy(source_rows)
        if not isinstance(rows,list) or len(rows)!=3 or source_plan.principal_publisher_documents(rows) is not True:
            raise ValueError()
        if not isinstance(platform_digest,str) or not re.fullmatch(r'sha256:[0-9a-f]{64}',platform_digest): raise ValueError()
        source_plan.principal_publisher_image(image)
        deployment=next(row for row in rows if row['kind']=='Deployment')
        if deployment['spec'].get('replicas') != 1: raise ValueError()
        container=deployment['spec']['template']['spec']['containers'][0]
        if container['image']!='nginx:stable-alpine': raise ValueError()
        container['image']=image
        return rows
    except Exception: raise PrerequisiteError('staging P3 publisher source invalid') from None


def inventory_intent():
    """Ordered fixed names only; it is not an authorization or live snapshot."""
    return [('ConfigMap/' if name=='voice-p3-principal-public' else 'Secret/')+name
            for name in ASSET_KEYS]+list(PUBLISHER)


def service_allocation(intended,actual):
    """Compare normalized Service intent and response; not a readback or authority."""
    try:
        wanted=gis_first_install.semantic(intended); live=gis_first_install.semantic(actual)
        identity={'name':'voice-p3-principal-jwks','namespace':NAMESPACE}
        if any(wanted['metadata'].get(k)!=v or live['metadata'].get(k)!=v for k,v in identity.items()): raise ValueError()
        if (wanted['kind']!='Service' or wanted['apiVersion']!='v1' or wanted['spec'].get('type')!='ClusterIP'
            or 'clusterIP' in wanted['spec'] or 'clusterIPs' in wanted['spec']): raise ValueError()
        meta=actual['metadata']
        if not all(isinstance(meta.get(k),str) and meta[k] for k in ('uid','resourceVersion')): raise ValueError()
        if (meta.get('ownerReferences') or meta.get('finalizers') or meta.get('deletionTimestamp')
            or set(actual)-{'apiVersion','kind','metadata','spec','status'}): raise ValueError()
        primary=live['spec'].pop('clusterIP'); addresses=live['spec'].pop('clusterIPs')
        if (not isinstance(addresses,list) or not 1<=len(addresses)<=2 or len(set(addresses))!=len(addresses)
            or primary!=addresses[0]): raise ValueError()
        parsed=[ipaddress.ip_address(address) for address in addresses]
        if any(ip.is_unspecified or ip.is_multicast for ip in parsed): raise ValueError()
        families=live['spec'].get('ipFamilies')
        if families is not None and families!=['IPv4' if ip.version==4 else 'IPv6' for ip in parsed]: raise ValueError()
        if live!=wanted: raise ValueError()
        return list(addresses)
    except Exception: raise PrerequisiteError('staging P3 Service allocation invalid') from None


def publisher_endpoint(deployment,replicasets,pods,slices,node,node_uid,platform_digest):
    """Correlate snapshots only; full spec, source and fresh reads are separate."""
    try:
        def controller(row,kind,uid):
            refs=row['metadata'].get('ownerReferences',[])
            owners=[ref for ref in refs if ref.get('controller') is True]
            return len(owners)==1 and owners[0].get('kind')==kind and owners[0].get('uid')==uid
        meta=deployment['metadata']; status=deployment['status']; generation=meta['generation']; dep_uid=meta['uid']
        if (meta.get('namespace')!=NAMESPACE or meta.get('name')!='voice-p3-principal-jwks' or meta.get('deletionTimestamp')
            or not dep_uid or type(generation) is not int or generation<1 or status.get('observedGeneration')!=generation
            or status.get('readyReplicas')!=1 or status.get('availableReplicas')!=1): raise ValueError()
        owned=[r for r in replicasets if controller(r,'Deployment',dep_uid) and r.get('spec',{}).get('replicas',0)!=0]
        if len(owned)!=1: raise ValueError()
        rs_uid=owned[0]['metadata']['uid']
        if (not rs_uid or owned[0]['metadata'].get('namespace')!=NAMESPACE or owned[0]['metadata'].get('deletionTimestamp')
            or owned[0]['spec']['replicas']!=1): raise ValueError()
        selected=[p for p in pods if p.get('metadata',{}).get('labels',{}).get('app')=='voice-p3-principal-jwks']
        if len(selected)!=1: raise ValueError()
        pod=selected[0]; pm=pod['metadata']; ps=pod['status']; pod_uid=pm['uid']
        if (not pod_uid or pm.get('namespace')!=NAMESPACE or pm.get('deletionTimestamp') or not controller(pod,'ReplicaSet',rs_uid)
            or ps.get('phase')!='Running' or [{'type':'Ready','status':'True'}][0] not in
               [{'type':v.get('type'),'status':v.get('status')} for v in ps.get('conditions',[])]): raise ValueError()
        statuses=ps['containerStatuses']
        if (len(statuses)!=1 or statuses[0].get('name')!='jwks' or statuses[0].get('ready') is not True
            or 'running' not in statuses[0].get('state',{}) or not re.fullmatch(r'sha256:[0-9a-f]{64}',platform_digest)
            or not re.fullmatch(r'(?:docker-pullable|containerd)://[^@\s]+@'+re.escape(platform_digest),statuses[0].get('imageID',''))): raise ValueError()
        if (not isinstance(node_uid,str) or not node_uid or node['metadata'].get('uid')!=node_uid
            or pod['spec'].get('nodeName')!=node['metadata'].get('name')
            or node['status'].get('nodeInfo',{}).get('operatingSystem')!='linux'
            or node['status'].get('nodeInfo',{}).get('architecture')!='amd64'
            or not any(c.get('type')=='Ready' and c.get('status')=='True' for c in node['status'].get('conditions',[]))): raise ValueError()
        matching=[s for s in slices if s.get('metadata',{}).get('labels',{}).get('kubernetes.io/service-name')=='voice-p3-principal-jwks']
        if (len(matching)!=1 or matching[0].get('metadata',{}).get('namespace')!=NAMESPACE
            or matching[0].get('ports')!=[{'name':'https','port':8443,'protocol':'TCP'}]): raise ValueError()
        endpoints=matching[0]['endpoints']
        if len(endpoints)!=1: raise ValueError()
        endpoint=endpoints[0]; address=ps['podIP']; ip=ipaddress.ip_address(address)
        if (ip.is_unspecified or ip.is_multicast or ip.is_loopback or ip.is_link_local or endpoint.get('addresses')!=[address]
            or endpoint.get('conditions',{}).get('ready') is not True
            or endpoint.get('targetRef',{}).get('kind')!='Pod' or endpoint['targetRef'].get('uid')!=pod_uid
            or endpoint['targetRef'].get('namespace')!=NAMESPACE): raise ValueError()
        return address
    except Exception: raise PrerequisiteError('staging P3 publisher endpoint not qualified') from None


def probe_jwks(address,ca,expected):
    """Probe fixed HTTPS path; address/CA must already be authenticated by caller."""
    try:
        ipaddress.ip_address(address)
        if not isinstance(ca,bytes) or not isinstance(expected,bytes) or not 0<len(expected)<=131072: raise ValueError()
        context=ssl.create_default_context(cadata=ca.decode('ascii'))
        host='voice-p3-principal-jwks.voice-staging.svc'
        with socket.create_connection((address,8443),timeout=5) as raw:
            with context.wrap_socket(raw,server_hostname=host) as tls:
                deadline=time.monotonic()+5
                tls.sendall(('GET /realtime.json HTTP/1.0\r\nHost: '+host+'\r\nConnection: close\r\n\r\n').encode('ascii'))
                data=bytearray()
                while True:
                    remaining=deadline-time.monotonic()
                    if remaining<=0: raise ValueError()
                    tls.settimeout(remaining); chunk=tls.recv(8192)
                    if not chunk: break
                    data.extend(chunk)
                    if len(data)>len(expected)+16384: raise ValueError()
                head,body=bytes(data).split(b'\r\n\r\n',1)
                lines=head.split(b'\r\n')
                if lines[0] not in (b'HTTP/1.0 200 OK',b'HTTP/1.1 200 OK'): raise ValueError()
                headers={}
                for line in lines[1:]:
                    key,value=line.split(b':',1); key=key.strip().lower()
                    if not key or key in headers: raise ValueError()
                    headers[key]=value.strip()
                if (headers.get(b'content-length')!=str(len(expected)).encode() or b'location' in headers
                    or b'content-encoding' in headers or b'transfer-encoding' in headers or body!=expected): raise ValueError()
        return True
    except Exception: raise PrerequisiteError('staging P3 publisher HTTPS proof unavailable') from None


def validate_plan_shape(plan):
    """Check the private fixed-plan grammar; this does not authenticate evidence."""
    try:
        expected={'schema','operation','namespace_uid','source_capture_sha256','source_contract_sha256',
                  'issuer_authority_sha256','auxiliary_image','objects','effects','preserved'}
        if not isinstance(plan,dict) or set(plan)!=expected or plan['schema']!='voice-staging-p3-prerequisites-plan-v1': raise ValueError()
        def nonempty(x): return isinstance(x,str) and 0<len(x)<=256
        def sha(x): return isinstance(x,str) and re.fullmatch(r'[0-9a-f]{64}',x) is not None
        if not nonempty(plan['operation']) or not nonempty(plan['namespace_uid']): raise ValueError()
        if any(not sha(plan[k]) for k in ('source_capture_sha256','source_contract_sha256','issuer_authority_sha256')): raise ValueError()
        aux=plan['auxiliary_image']
        if not isinstance(aux,dict) or set(aux)!={'workload','container','source_ref','image_ref','platform','manifest_sha256',
                                                  'config_sha256','index_sha256','source_authority_sha256'}: raise ValueError()
        if (aux['workload']!='voice-p3-principal-jwks' or aux['container']!='jwks' or aux['source_ref']!='nginx:stable-alpine'
            or any(not sha(aux[k]) for k in ('manifest_sha256','config_sha256','source_authority_sha256'))
            or (aux['index_sha256'] is not None and not sha(aux['index_sha256']))): raise ValueError()
        source_plan.principal_publisher_image(aux['image_ref'])
        if aux['image_ref']!='docker.io/library/nginx@sha256:'+(aux['index_sha256'] or aux['manifest_sha256']): raise ValueError()
        platform=aux['platform']
        if (not isinstance(platform,dict) or set(platform)!={'os','architecture','variant'} or platform['os']!='linux'
            or not nonempty(platform['architecture']) or (platform['variant'] is not None and not nonempty(platform['variant']))): raise ValueError()
        ids=inventory_intent(); objects=plan['objects']; effects=plan['effects']; preserved=plan['preserved']
        if not isinstance(objects,list) or len(objects)!=len(ids) or not isinstance(effects,list) or not isinstance(preserved,dict): raise ValueError()
        by_id={}
        for ident,obj in zip(ids,objects):
            kind,name=ident.split('/')
            if (not isinstance(obj,dict) or set(obj)!={'id','kind','name','namespace','desired_sha256','payload_sha256'}
                or (obj['id'],obj['kind'],obj['name'],obj['namespace'])!=(ident,kind,name,NAMESPACE)
                or not sha(obj['desired_sha256']) or not sha(obj['payload_sha256'])): raise ValueError()
            by_id[ident]=obj
        effect_ids=[]
        for effect in effects:
            if not isinstance(effect,dict) or set(effect)!=(set(objects[0])|{'before'}): raise ValueError()
            ident=effect['id']; before=effect['before']
            if (ident not in by_id or {k:v for k,v in effect.items() if k!='before'}!=by_id[ident]
                or not isinstance(before,dict) or set(before)!={'absent','list_rv'} or before['absent'] is not True
                or not nonempty(before['list_rv'])): raise ValueError()
            effect_ids.append(ident)
        if effect_ids!=[ident for ident in ids if ident in effect_ids] or set(effect_ids)&set(preserved) or set(effect_ids)|set(preserved)!=set(ids): raise ValueError()
        for ident,item in preserved.items():
            if not isinstance(item,dict) or set(item)!={'authority','observed'}: raise ValueError()
            authority=item['authority']; observed=item['observed']; kind,name=ident.split('/')
            if (not isinstance(authority,dict) or set(authority)!={'operation','receipt_sha256'}
                or not nonempty(authority['operation']) or not sha(authority['receipt_sha256'])): raise ValueError()
            fields={'kind','name','namespace','uid','resourceVersion','semantic_sha256'}|({'generation'} if kind=='Deployment' else set())
            if (not isinstance(observed,dict) or set(observed)!=fields or (observed['kind'],observed['name'],observed['namespace'])!=(kind,name,NAMESPACE)
                or not nonempty(observed['uid']) or not nonempty(observed['resourceVersion']) or not sha(observed['semantic_sha256'])
                or (kind=='Deployment' and (type(observed['generation']) is not int or observed['generation']<1))): raise ValueError()
        return copy.deepcopy(plan)
    except Exception: raise PrerequisiteError('staging P3 private plan malformed') from None


def material_objects(values):
    """Convert private bytes to exact staging API objects; caller must validate crypto."""
    try:
        if not isinstance(values,dict) or set(values)!=set(ASSET_KEYS): raise ValueError()
        result=[]
        for name,keys in ASSET_KEYS.items():
            row=values[name]
            if not isinstance(row,dict) or set(row)!=keys: raise ValueError()
            for key,raw in row.items():
                if not isinstance(raw,bytes) or len(raw)>131072 or (not raw and name!='voice-p3-principal-replay'):
                    raise ValueError()
            meta={'name':name,'namespace':NAMESPACE}
            if name=='voice-p3-principal-public':
                data={key:raw.decode('utf-8') for key,raw in row.items()}
                result.append({'apiVersion':'v1','kind':'ConfigMap','metadata':meta,'immutable':True,'data':data})
            else:
                data={key:base64.b64encode(raw).decode('ascii') for key,raw in row.items()}
                result.append({'apiVersion':'v1','kind':'Secret','metadata':meta,
                               'type':'kubernetes.io/tls' if name in TLS_NAMES else 'Opaque','data':data})
        return result
    except Exception: raise PrerequisiteError('staging P3 material invalid') from None


def classify_inventory(cycle, current, protected_receipt):
    """Classify bounded snapshots; caller must authenticate and reread both."""
    try:
        names=inventory_intent()
        if cycle not in ('c1','c2') or not isinstance(current,dict) or set(current)!=set(names): raise ValueError()
        def valid(item):
            return (isinstance(item,dict) and set(item)=={'uid','resourceVersion','content_sha256'}
                    and all(isinstance(item[k],str) and item[k] for k in ('uid','resourceVersion'))
                    and isinstance(item['content_sha256'],str) and re.fullmatch(r'[0-9a-f]{64}',item['content_sha256']))
        if cycle=='c1':
            if protected_receipt is not None or any(current[name] is not None for name in names): raise ValueError()
            return 'create'
        if not isinstance(protected_receipt,dict) or set(protected_receipt)!=set(names): raise ValueError()
        for name in names:
            old=protected_receipt[name]; new=current[name]
            if not valid(old) or not valid(new) or any(old[k]!=new[k] for k in ('uid','content_sha256')): raise ValueError()
        return 'revalidate'
    except Exception: raise PrerequisiteError('staging P3 inventory not qualified') from None
