"""Fixed STAGING P3 object contract; pure helpers do not grant authority."""
import base64
import copy
import hashlib
import ipaddress
import json
import os
from pathlib import Path
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


def source_contract(approved,raw):
    """Bind publisher source bytes; caller authenticates the approved capture."""
    try:
        sha=approved['source_sha']; expected=approved['source_files']['deploy/staging/services.yaml']
        if (not isinstance(sha,str) or not re.fullmatch(r'[0-9a-f]{40}',sha)
            or not isinstance(expected,str) or not re.fullmatch(r'[0-9a-f]{64}',expected)
            or not isinstance(raw,bytes) or not 0<len(raw)<=2<<20 or hashlib.sha256(raw).hexdigest()!=expected): raise ValueError()
        return {'schema':'voice-staging-p3-source-contract-v1','source_sha':sha,'services_sha256':expected}
    except Exception: raise PrerequisiteError('staging P3 source contract invalid') from None


def source_consumers(rows):
    """Check source P3 listener/issuer wiring; Space lifecycle remains dormant."""
    try:
        names=('voice-realtime','voice-chat','voice-space')
        if not isinstance(rows,list) or len(rows)!=6: raise ValueError()
        keyed={(r['kind'],r['metadata']['name']):r for r in rows}
        if len(keyed)!=6 or set(keyed)!={(kind,name) for kind in ('Deployment','Service') for name in names}: raise ValueError()
        envs={
          'voice-realtime':{'REALTIME_PRINCIPAL_SIGNING_KEYS_DIR':'/etc/voice/p3/keys','REALTIME_PRINCIPAL_ACTIVE_KID':'current',
           'REALTIME_PRINCIPAL_JWKS_FILE':'/etc/voice/p3/public/realtime.json','REALTIME_CHAT_LIFECYCLE_ADDR':'voice-chat:9099',
           'REALTIME_CHAT_LIFECYCLE_SERVER_NAME':'voice-chat.voice-staging.svc','REALTIME_CHAT_LIFECYCLE_CA_FILE':'/etc/voice/p3/trust/chat-server-ca.crt',
           'REALTIME_CHAT_LIFECYCLE_CERT_FILE':'/etc/voice/p3/tls/tls.crt','REALTIME_CHAT_LIFECYCLE_KEY_FILE':'/etc/voice/p3/tls/tls.key',
           'REALTIME_SPACE_LIFECYCLE_GRPC_LISTEN':':9091','REALTIME_SPACE_LIFECYCLE_TLS_CERT_FILE':'/etc/voice/p3/tls/tls.crt',
           'REALTIME_SPACE_LIFECYCLE_TLS_KEY_FILE':'/etc/voice/p3/tls/tls.key','REALTIME_SPACE_LIFECYCLE_CLIENT_CA_FILE':'/etc/voice/p3/trust/space-client-ca.crt',
           'REALTIME_SPACE_PRINCIPAL_JWKS_URL':'https://voice-space.voice-staging.svc:8443/.well-known/jwks.json',
           'REALTIME_SPACE_PRINCIPAL_JWKS_CA_FILE':'/etc/voice/p3/trust/space-jwks-ca.crt'},
          'voice-chat':{'CHAT_REALTIME_LIFECYCLE_GRPC_LISTEN':':9099','CHAT_REALTIME_LIFECYCLE_TLS_CERT_FILE':'/etc/voice/realtime-p3/tls/tls.crt',
           'CHAT_REALTIME_LIFECYCLE_TLS_KEY_FILE':'/etc/voice/realtime-p3/tls/tls.key','CHAT_REALTIME_LIFECYCLE_CLIENT_CA_FILE':'/etc/voice/realtime-p3/trust/realtime-client-ca.crt',
           'CHAT_REALTIME_PRINCIPAL_JWKS_URL':'https://voice-p3-principal-jwks.voice-staging.svc:8443/realtime.json',
           'CHAT_REALTIME_PRINCIPAL_JWKS_CA_FILE':'/etc/voice/realtime-p3/trust/realtime-jwks-ca.crt'},
          'voice-space':{'SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE':'/etc/voice/media/server/tls.key',
           'SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE':'/etc/voice/media/server/tls.crt','SPACE_PRINCIPAL_JWKS_HTTPS_LISTEN':':8443',
           'SPACE_PRINCIPAL_SIGNING_KEYS_DIR':'/etc/voice/media/signing','S2S_JWKS_CLIENT_KEY_FILE':'/etc/voice/media/client/tls.key',
           'S2S_JWKS_CLIENT_CERT_FILE':'/etc/voice/media/client/tls.crt'}}
        mounts={'voice-realtime':{'p3-signing':'/etc/voice/p3/keys','p3-public':'/etc/voice/p3/public','p3-server':'/etc/voice/p3/tls','p3-trust':'/etc/voice/p3/trust'},
                'voice-chat':{'realtime-p3-server':'/etc/voice/realtime-p3/tls','realtime-p3-trust':'/etc/voice/realtime-p3/trust'},
                'voice-space':{'media-signing':'/etc/voice/media/signing','media-server':'/etc/voice/media/server','media-client':'/etc/voice/media/client'}}
        refs={'voice-realtime':{'p3-signing':('secret','voice-realtime-principal-signing'),'p3-public':('configMap','voice-p3-principal-public'),
                                'p3-server':('secret','voice-realtime-p3-tls'),'p3-trust':('secret','voice-p3-principal-trust')},
              'voice-chat':{'realtime-p3-server':('secret','voice-chat-realtime-p3-tls'),'realtime-p3-trust':('secret','voice-p3-principal-trust')},
              'voice-space':{'media-signing':('secret','voice-space-principal-signing'),'media-server':('secret','voice-space-media-server-tls'),
                             'media-client':('secret','voice-space-media-client-tls')}}
        for name in names:
            deployment=keyed['Deployment',name]; service=keyed['Service',name]
            if deployment['metadata'].get('namespace')!=NAMESPACE or service['metadata'].get('namespace')!=NAMESPACE: raise ValueError()
            spec=deployment['spec']['template']['spec']; containers=[c for c in spec['containers'] if c.get('name')==name.removeprefix('voice-')]
            if len(containers)!=1: raise ValueError()
            container=containers[0]; entries=container['env']; env={e['name']:e for e in entries}
            if len(entries)!=len(env): raise ValueError()
            for key,value in envs[name].items():
                if env.get(key)!={'name':key,'value':value}: raise ValueError()
            if name!='voice-space':
                prefix='REALTIME_SPACE' if name=='voice-realtime' else 'CHAT_REALTIME'
                key=prefix+'_PRINCIPAL_REPLAY_REDIS_PASSWORD'
                if env.get(key)!={'name':key,'valueFrom':{'secretKeyRef':{'name':'voice-p3-principal-replay','key':'password'}}}: raise ValueError()
                key=prefix+'_PRINCIPAL_REPLAY_REDIS_ADDR'
                if env.get(key)!={'name':key,'valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'REDIS_ADDR'}}}: raise ValueError()
            elif env.get('SPACE_PRINCIPAL_ACTIVE_KID')!={'name':'SPACE_PRINCIPAL_ACTIVE_KID','valueFrom':{'secretKeyRef':{'name':'voice-space-principal-signing','key':'active-kid'}}}: raise ValueError()
            entries=container['volumeMounts']; by_name={m['name']:m for m in entries}
            volumes={v['name']:v for v in spec['volumes']}
            if len(entries)!=len(by_name) or len(spec['volumes'])!=len(volumes): raise ValueError()
            for mount,path in mounts[name].items():
                if by_name.get(mount)!={'name':mount,'mountPath':path,'readOnly':True}: raise ValueError()
                for other in entries:
                    op=other.get('mountPath','')
                    if other['name']!=mount and (op==path or op.startswith(path+'/') or path.startswith(op+'/')): raise ValueError()
                typ,ref=refs[name][mount]; source=volumes[mount][typ]
                keys={('voice-realtime','p3-signing'):('current.pem','next.pem'),('voice-realtime','p3-public'):('realtime.json',),
                      ('voice-realtime','p3-trust'):('chat-server-ca.crt','space-client-ca.crt','space-jwks-ca.crt'),
                      ('voice-chat','realtime-p3-trust'):('realtime-client-ca.crt','realtime-jwks-ca.crt'),
                      ('voice-space','media-signing'):('current.pem','next.pem'),('voice-space','media-server'):('tls.crt','tls.key'),
                      ('voice-space','media-client'):('tls.crt','tls.key')}.get((name,mount))
                want={'secretName' if typ=='secret' else 'name':ref}
                if typ=='secret': want['defaultMode']=288
                if keys is not None: want['items']=[{'key':key,'path':key} for key in keys]
                if source!=want or set(volumes[mount])!={'name',typ}: raise ValueError()
            if (service['spec'].get('selector')!={'app':name}
                or deployment['spec'].get('selector')!={'matchLabels':{'app':name}}
                or deployment['spec']['template']['metadata'].get('labels',{}).get('app')!=name): raise ValueError()
            port={'voice-realtime':('realtime-lifecycle',9091),'voice-chat':('realtime-lifecycle',9099),'voice-space':('principal-jwks',8443)}[name]
            selected=[p for p in service['spec']['ports'] if p.get('name')==port[0]]
            if (len(selected)!=1 or set(selected[0])-{'name','port','protocol','targetPort'}
                or selected[0].get('port')!=port[1] or selected[0].get('targetPort',port[1])!=port[1]
                or selected[0].get('protocol','TCP')!='TCP'): raise ValueError()
        return True
    except Exception: raise PrerequisiteError('staging P3 source consumers invalid') from None


def private_semantic(obj):
    """Retain full private object semantics while removing only server identity."""
    try:
        row=copy.deepcopy(obj)
        if not isinstance(row,dict) or not isinstance(row.get('metadata'),dict): raise ValueError()
        if row['metadata'].get('deletionTimestamp') is not None: raise ValueError()
        row.pop('status',None)
        for key in ('uid','resourceVersion','generation','creationTimestamp','managedFields'):
            row['metadata'].pop(key,None)
        return row
    except Exception: raise PrerequisiteError('staging P3 object invalid') from None


def deployment_successor(desired,actual,replicasets):
    """Permit only a controller revision backed by one exact owned ReplicaSet."""
    try:
        wanted=private_semantic(desired); observed=private_semantic(actual)
        meta=actual['metadata']; uid=meta['uid']
        if (not uid or meta.get('name')!='voice-p3-principal-jwks' or meta.get('namespace')!=NAMESPACE
            or actual.get('kind')!='Deployment' or desired.get('kind')!='Deployment'): raise ValueError()
        annotations=observed['metadata'].get('annotations',{})
        revision=annotations.pop('deployment.kubernetes.io/revision',None)
        if not isinstance(revision,str) or not re.fullmatch(r'[1-9][0-9]*',revision): raise ValueError()
        if not annotations: observed['metadata'].pop('annotations',None)
        wanted_annotations=wanted['metadata'].get('annotations',{})
        wanted_revision=wanted_annotations.pop('deployment.kubernetes.io/revision',None)
        if wanted_revision is not None and wanted_revision!=revision: raise ValueError()
        if not wanted_annotations: wanted['metadata'].pop('annotations',None)
        if observed!=wanted: raise ValueError()
        owned=[]
        for row in replicasets:
            refs=[ref for ref in row.get('metadata',{}).get('ownerReferences',[]) if ref.get('controller') is True]
            if len(refs)==1 and refs[0].get('kind')=='Deployment' and refs[0].get('uid')==uid and row.get('spec',{}).get('replicas',0)!=0:
                owned.append(row)
        if len(owned)!=1: raise ValueError()
        rs=owned[0]; rm=rs['metadata']; spec=rs['spec']
        if (rm.get('namespace')!=NAMESPACE or not rm.get('uid') or rm.get('deletionTimestamp') or spec.get('replicas')!=1
            or set(spec)-{'replicas','selector','template','minReadySeconds'}
            or spec.get('minReadySeconds',0)!=desired['spec'].get('minReadySeconds',0)
            or rm.get('annotations',{}).get('deployment.kubernetes.io/revision')!=revision): raise ValueError()
        template=copy.deepcopy(spec['template']); labels=template['metadata']['labels']; token=labels.pop('pod-template-hash')
        if (not isinstance(token,str) or not token or template!=desired['spec']['template']
            or spec['selector']!={'matchLabels':{**desired['spec']['selector']['matchLabels'],'pod-template-hash':token}}): raise ValueError()
        return rm['uid']
    except Exception: raise PrerequisiteError('staging P3 Deployment successor invalid') from None


def _source_and_image(session,plan):
    """Reload captured authority, exact source bytes and fixed auxiliary proof."""
    import compiler
    import encrypted_cut
    import guard
    import resource_claim
    import staging_prerequisites as parent
    authority=parent.load_authority(session)
    if (not isinstance(authority,dict) or set(authority)!={'approved','execution','source'}
        or authority['source']!=str(guard.ROOT/'installed'/'sources'/session.operation)): raise ValueError()
    path=Path(authority['source'])/'deploy'/'staging'/'services.yaml'
    fd,info=encrypted_cut.regular(path)
    try:
        if not 0<info.st_size<=2<<20: raise ValueError()
        data=bytearray()
        while len(data)<info.st_size:
            chunk=os.read(fd,min(65536,info.st_size-len(data)))
            if not chunk: raise ValueError()
            data.extend(chunk)
        after=os.fstat(fd)
        if (os.read(fd,1) or (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)!=
            (after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns)): raise ValueError()
    finally: os.close(fd)
    raw=bytes(data); approved=authority['approved']
    if (approved['source_sha']!=session.context['commit']
        or resource_claim.digest(source_contract(approved,raw))!=plan['source_contract_sha256']): raise ValueError()
    record=parent.load_auxiliary_image(session,'p3'); aux=plan['auxiliary_image']
    fields={'schema','operation','nonce','context','helper_sha256','source_capture_sha256','source_module_sha256',
            'source_ref','repository','index_sha256','manifest_sha256','config_sha256','platform','node','image_ref'}
    if (not isinstance(record,dict) or set(record)!=fields or record['schema']!='voice-staging-p3-auxiliary-image-v1'
        or record['operation']!=session.operation or record['context']!=session.context or record['helper_sha256']!=session.helper
        or record['source_capture_sha256']!=plan['source_capture_sha256'] or resource_claim.digest(record)!=aux['source_authority_sha256']
        or record['source_module_sha256']!=approved['source_files']['scripts/deploy/staging_p3_auxiliary_image.py']
        or not isinstance(record['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',record['nonce'])
        or record['platform']!={'os':'linux','architecture':'amd64','variant':None}
        or not isinstance(record['node'],dict) or set(record['node'])!={'name','uid','os','architecture'}
        or record['node']['os']!='linux' or record['node']['architecture']!='amd64'
        or not all(isinstance(record['node'][k],str) and record['node'][k] for k in ('name','uid'))
        or record['source_ref']!=aux['source_ref'] or record['repository']!='docker.io/library/nginx'
        or any(record[k]!=aux[k] for k in ('index_sha256','manifest_sha256','config_sha256','platform','image_ref'))): raise ValueError()
    selected=[]
    with resource_claim.command_scope(session.check):
        for piece in re.split(r'^---\s*$',raw.decode('utf-8'),flags=re.M):
            if re.search(r'^  name: (?:voice-realtime|voice-chat|voice-space|voice-p3-principal-jwks|voice-p3-principal-publisher-config)\s*$',piece,re.M):
                selected.append(compiler.decode_yaml(piece.encode()))
    consumers=[r for r in selected if r.get('metadata',{}).get('name') in ('voice-realtime','voice-chat','voice-space')]
    publisher=[r for r in selected if r.get('metadata',{}).get('name') in ('voice-p3-principal-jwks','voice-p3-principal-publisher-config')]
    source_consumers(consumers)
    built=publisher_objects(publisher,aux['image_ref'],'sha256:'+aux['manifest_sha256'])
    session.check()
    if parent.load_authority(session)!=authority or parent.load_auxiliary_image(session,'p3')!=record: raise ValueError()
    return built,record


def mark_operation(objects,operation):
    """Build private payloads; caller must derive operation from the root session."""
    try:
        if not isinstance(operation,str) or not 0<len(operation)<=256 or not isinstance(objects,list): raise ValueError()
        rows={}
        for obj in copy.deepcopy(objects):
            meta=obj['metadata']; ident=obj['kind']+'/'+meta['name']
            if ident not in inventory_intent() or ident in rows or meta.get('namespace')!=NAMESPACE: raise ValueError()
            annotations=meta.setdefault('annotations',{})
            if not isinstance(annotations,dict) or 'voice.poryadok.io/prerequisites-operation' in annotations: raise ValueError()
            annotations['voice.poryadok.io/prerequisites-operation']=operation; rows[ident]=obj
        if set(rows)!=set(inventory_intent()): raise ValueError()
        return [rows[ident] for ident in inventory_intent()]
    except Exception: raise PrerequisiteError('staging P3 operation payload invalid') from None


def service_allocation(intended,actual,*,dry_run=False):
    """Compare normalized Service intent and response; not a readback or authority."""
    try:
        wanted=gis_first_install.semantic(intended); live=gis_first_install.semantic(actual)
        identity={'name':'voice-p3-principal-jwks','namespace':NAMESPACE}
        if any(wanted['metadata'].get(k)!=v or live['metadata'].get(k)!=v for k,v in identity.items()): raise ValueError()
        if (wanted['kind']!='Service' or wanted['apiVersion']!='v1' or wanted['spec'].get('type')!='ClusterIP'
            or 'clusterIP' in wanted['spec'] or 'clusterIPs' in wanted['spec']): raise ValueError()
        meta=actual['metadata']
        if type(dry_run) is not bool: raise ValueError()
        if not dry_run and not all(isinstance(meta.get(k),str) and meta[k] for k in ('uid','resourceVersion')): raise ValueError()
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


def publisher_endpoint(deployment,replicasets,pods,slices,node,node_uid,platform_digest,service):
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
            or not re.fullmatch(r'(?:(?:docker-pullable|containerd)://)?(?:docker\.io/library/)?nginx@'+re.escape(platform_digest),statuses[0].get('imageID',''))): raise ValueError()
        if (not isinstance(node_uid,str) or not node_uid or node['metadata'].get('uid')!=node_uid
            or node['metadata'].get('deletionTimestamp') or not isinstance(node['metadata'].get('name'),str)
            or not node['metadata']['name'] or pod['spec'].get('nodeName')!=node['metadata']['name']
            or node['status'].get('nodeInfo',{}).get('operatingSystem')!='linux'
            or node['status'].get('nodeInfo',{}).get('architecture')!='amd64'
            or not any(c.get('type')=='Ready' and c.get('status')=='True' for c in node['status'].get('conditions',[]))): raise ValueError()
        matching=[s for s in slices if s.get('metadata',{}).get('labels',{}).get('kubernetes.io/service-name')=='voice-p3-principal-jwks']
        service_meta=service['metadata']; service_uid=service_meta.get('uid'); ports=service['spec']['ports']
        if (service_meta.get('name')!='voice-p3-principal-jwks' or service_meta.get('namespace')!=NAMESPACE
            or service_meta.get('deletionTimestamp') or not service_uid
            or service['spec'].get('selector')!={'app':'voice-p3-principal-jwks'} or len(ports)!=1
            or ports[0].get('name')!='https' or ports[0].get('port')!=8443 or ports[0].get('targetPort',8443)!=8443
            or ports[0].get('protocol','TCP')!='TCP'): raise ValueError()
        if (len(matching)!=1 or matching[0].get('metadata',{}).get('namespace')!=NAMESPACE
            or matching[0]['metadata'].get('deletionTimestamp')
            or not controller(matching[0],'Service',service_uid)
            or matching[0].get('ports')!=[{'name':'https','port':8443,'protocol':'TCP'}]): raise ValueError()
        endpoints=matching[0]['endpoints']
        if len(endpoints)!=1: raise ValueError()
        endpoint=endpoints[0]; address=ps['podIP']; ip=ipaddress.ip_address(address)
        if (ip.is_unspecified or ip.is_multicast or ip.is_loopback or ip.is_link_local or endpoint.get('addresses')!=[address]
            or endpoint.get('conditions',{}).get('ready') is not True or endpoint['conditions'].get('terminating') is True
            or endpoint.get('targetRef',{}).get('kind')!='Pod' or endpoint['targetRef'].get('uid')!=pod_uid
            or endpoint['targetRef'].get('namespace')!=NAMESPACE): raise ValueError()
        return address
    except Exception: raise PrerequisiteError('staging P3 publisher endpoint not qualified') from None


def probe_jwks(address,ca,expected,leaf_der_sha256):
    """Probe fixed HTTPS path; inputs must already be authenticated by caller."""
    try:
        ipaddress.ip_address(address)
        if (not isinstance(ca,bytes) or not isinstance(expected,bytes) or not 0<len(expected)<=131072
            or not isinstance(leaf_der_sha256,str) or not re.fullmatch(r'[0-9a-f]{64}',leaf_der_sha256)): raise ValueError()
        context=ssl.create_default_context(cadata=ca.decode('ascii'))
        host='voice-p3-principal-jwks.voice-staging.svc'
        with socket.create_connection((address,8443),timeout=5) as raw:
            with context.wrap_socket(raw,server_hostname=host) as tls:
                if hashlib.sha256(tls.getpeercert(binary_form=True)).hexdigest()!=leaf_der_sha256: raise ValueError()
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
                  'issuer_authority_sha256','issuer_input_sha256','auxiliary_image','objects','effects','preserved'}
        if not isinstance(plan,dict) or set(plan)!=expected or plan['schema']!='voice-staging-p3-prerequisites-plan-v1': raise ValueError()
        def nonempty(x): return isinstance(x,str) and 0<len(x)<=256
        def sha(x): return isinstance(x,str) and re.fullmatch(r'[0-9a-f]{64}',x) is not None
        if not nonempty(plan['operation']) or not nonempty(plan['namespace_uid']): raise ValueError()
        if any(not sha(plan[k]) for k in ('source_capture_sha256','source_contract_sha256','issuer_authority_sha256','issuer_input_sha256')): raise ValueError()
        aux=plan['auxiliary_image']
        if not isinstance(aux,dict) or set(aux)!={'workload','container','source_ref','image_ref','platform','manifest_sha256',
                                                  'config_sha256','index_sha256','source_authority_sha256'}: raise ValueError()
        if (aux['workload']!='voice-p3-principal-jwks' or aux['container']!='jwks' or aux['source_ref']!='nginx:stable-alpine'
            or any(not sha(aux[k]) for k in ('manifest_sha256','config_sha256','source_authority_sha256'))
            or (aux['index_sha256'] is not None and not sha(aux['index_sha256']))): raise ValueError()
        source_plan.principal_publisher_image(aux['image_ref'])
        if aux['image_ref']!='docker.io/library/nginx@sha256:'+aux['manifest_sha256']: raise ValueError()
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


def execute(session):
    """Execute only the parent-owned fixed effects after its complete preflight."""
    import resource_claim
    import staging_prerequisites as parent
    if type(session) is not resource_claim.Session: raise PrerequisiteError('staging P3 session invalid')
    plan=validate_plan_shape(parent.require_child(session,'p3'))
    if plan['operation']!=session.operation: raise PrerequisiteError('staging P3 operation changed')
    for effect in plan['effects']:
        ident=effect['id']
        session.check()
        parent.record_pending(session,'p3',ident)
        observation=parent.submit_effect(session,'p3',ident)
        parent.record_done(session,'p3',ident,observation)
        session.check()


def preflight_payloads(session):
    """Qualify all fixed private inputs before any prerequisite effect."""
    import resource_claim
    import staging_prerequisites as parent
    import staging_p3_prerequisites_crypto as crypto
    from stage_runtime import Kube
    try:
        if type(session) is not resource_claim.Session: raise ValueError()
        session.check(); plan=validate_plan_shape(parent.require_child(session,'p3'))
        if plan['operation']!=session.operation: raise ValueError()
        publisher,record=_source_and_image(session,plan)
        issuer=parent.load_issuer(session,'p3')
        effects={row['id']:row for row in plan['effects']}; objects={row['id']:row for row in plan['objects']}
        kube=Kube(); current={}; payloads={}; operations={}; provenances={}
        def check_namespace():
            with resource_claim.command_scope(session.check): namespace=kube.get('namespace',NAMESPACE)
            meta=namespace['metadata']
            if (namespace.get('kind')!='Namespace' or meta.get('name')!=NAMESPACE or meta.get('uid')!=plan['namespace_uid']
                or meta.get('deletionTimestamp')): raise ValueError()
        check_namespace()
        with resource_claim.command_scope(session.check):
            listings={kind:kube.run(['get',kind.lower(),'-o','json']) for kind in ('Secret','ConfigMap','Service','Deployment')}
        for listing in listings.values():
            if (not isinstance(listing,dict) or not isinstance(listing.get('items'),list)
                or not isinstance(listing.get('metadata',{}).get('resourceVersion'),str)
                or not listing['metadata']['resourceVersion']): raise ValueError()
        for ident in inventory_intent():
            kind,name=ident.split('/'); matches=[r for r in listings[kind]['items'] if r.get('metadata',{}).get('name')==name]
            if ident in effects:
                if matches: raise ValueError()
                raw=parent.load_payload(session,'p3',ident)
                if hashlib.sha256(raw).hexdigest()!=objects[ident]['payload_sha256']: raise ValueError()
                row=json.loads(raw,object_pairs_hook=_unique_json)
                if json.dumps(row,sort_keys=True,separators=(',',':'),allow_nan=False).encode()!=raw: raise ValueError()
                if resource_claim.digest(private_semantic(row))!=objects[ident]['desired_sha256']: raise ValueError()
                operations[ident]=session.operation; payloads[ident]=row
            else:
                if len(matches)!=1: raise ValueError()
                row=matches[0]; provenance=parent.load_preserved(session,'p3',ident)
                old=plan['preserved'][ident]; semantic=private_semantic(row); meta=row['metadata']
                if (not isinstance(provenance,dict) or set(provenance)!={'authority','semantic','uid'}
                    or provenance['authority']!=old['authority'] or provenance['uid']!=old['observed']['uid']
                    or meta.get('uid')!=provenance['uid'] or semantic!=provenance['semantic']
                    or resource_claim.digest(semantic)!=old['observed']['semantic_sha256']
                    or meta.get('annotations',{}).get('voice.poryadok.io/prerequisites-operation')!=provenance['authority']['operation']): raise ValueError()
                if kind=='Deployment':
                    if meta.get('generation')!=old['observed']['generation']: raise ValueError()
                elif meta.get('resourceVersion')!=old['observed']['resourceVersion']: raise ValueError()
                operations[ident]=provenance['authority']['operation']; current[ident]=row; payloads[ident]=row
                provenances[ident]=provenance
            if (row.get('kind')!=kind or row.get('metadata',{}).get('name')!=name
                or row['metadata'].get('namespace')!=NAMESPACE or row['metadata'].get('annotations',{}).get(
                    'voice.poryadok.io/prerequisites-operation')!=operations[ident]): raise ValueError()
        values=_material_values(payloads)
        crypto.validate_issuer_materials(values,issuer)
        requested=material_objects(values)+copy.deepcopy(publisher)
        for row in requested:
            ident=row['kind']+'/'+row['metadata']['name']
            if ident not in effects: continue
            row['metadata']['annotations']={'voice.poryadok.io/prerequisites-operation':session.operation}
            with resource_claim.command_scope(session.check):
                normalized=kube.run(['create','--dry-run=server','-f','/dev/stdin','-o','json'],body=row)
            observed=private_semantic(normalized); expected=private_semantic(payloads[ident])
            if row['kind']=='Service' and ('clusterIP' in observed.get('spec',{}) or 'clusterIPs' in observed.get('spec',{})):
                service_allocation(payloads[ident],normalized,dry_run=True)
                observed['spec'].pop('clusterIP'); observed['spec'].pop('clusterIPs')
            if observed!=expected: raise ValueError()
        session.check()
        if parent.require_child(session,'p3')!=plan or parent.load_issuer(session,'p3')!=issuer: raise ValueError()
        publisher2,record2=_source_and_image(session,plan)
        if publisher2!=publisher or record2!=record: raise ValueError()
        with resource_claim.command_scope(session.check):
            final={kind:kube.run(['get',kind.lower(),'-o','json']) for kind in ('Secret','ConfigMap','Service','Deployment')}
        for listing in final.values():
            if (not isinstance(listing,dict) or not isinstance(listing.get('items'),list)
                or not isinstance(listing.get('metadata',{}).get('resourceVersion'),str)
                or not listing['metadata']['resourceVersion']): raise ValueError()
        for ident in inventory_intent():
            kind,name=ident.split('/'); matches=[r for r in final[kind]['items'] if r.get('metadata',{}).get('name')==name]
            if ident in effects:
                if matches: raise ValueError()
            elif (len(matches)!=1 or matches[0].get('metadata',{}).get('uid')!=current[ident]['metadata']['uid']
                  or private_semantic(matches[0])!=private_semantic(current[ident])
                  or (kind!='Deployment' and matches[0]['metadata'].get('resourceVersion')!=current[ident]['metadata'].get('resourceVersion'))): raise ValueError()
        for ident,provenance in provenances.items():
            if parent.load_preserved(session,'p3',ident)!=provenance: raise ValueError()
        check_namespace()
        session.check()
        if parent.require_child(session,'p3')!=plan: raise ValueError()
        return True
    except Exception: raise PrerequisiteError('staging P3 preflight not qualified') from None


def _unique_json(pairs):
    result={}
    for key,value in pairs:
        if key in result: raise ValueError()
        result[key]=value
    return result


def _material_values(rows):
    values={}
    for ident in inventory_intent()[:len(ASSET_KEYS)]:
        kind,name=ident.split('/'); row=rows[ident]; data=row['data']
        if not isinstance(data,dict) or set(data)!=ASSET_KEYS[name]: raise ValueError()
        if kind=='ConfigMap':
            if row.get('immutable') is not True: raise ValueError()
            values[name]={k:v.encode('utf-8') for k,v in data.items()}
        else:
            if row.get('type')!=('kubernetes.io/tls' if name in TLS_NAMES else 'Opaque'): raise ValueError()
            values[name]={k:base64.b64decode(v,validate=True) for k,v in data.items()}
            if any(base64.b64encode(values[name][k]).decode()!=v for k,v in data.items()): raise ValueError()
    return values
def verify_effect(session,effect_id,observation):
    """Freshly verify one parent-owned create response against private intent."""
    import resource_claim
    import staging_prerequisites as parent
    from stage_runtime import Kube
    try:
        if type(session) is not resource_claim.Session: raise ValueError()
        plan=validate_plan_shape(parent.require_child(session,'p3'))
        if plan['operation']!=session.operation: raise ValueError()
        effects=[row for row in plan['effects'] if row['id']==effect_id]
        if len(effects)!=1: raise ValueError()
        effect=effects[0]; kind=effect['kind']; name=effect['name']
        fields={'kind','name','namespace','uid','resourceVersion','semantic_sha256'}|({'generation'} if kind=='Deployment' else set())
        if (not isinstance(observation,dict) or set(observation)!=fields
            or (observation['kind'],observation['name'],observation['namespace'])!=(kind,name,NAMESPACE)
            or not all(isinstance(observation[k],str) and observation[k] for k in ('uid','resourceVersion'))): raise ValueError()
        raw=parent.load_payload(session,'p3',effect_id)
        if hashlib.sha256(raw).hexdigest()!=effect['payload_sha256']: raise ValueError()
        def unique(pairs):
            result={}
            for key,value in pairs:
                if key in result: raise ValueError()
                result[key]=value
            return result
        desired=json.loads(raw,object_pairs_hook=unique)
        if json.dumps(desired,sort_keys=True,separators=(',',':'),allow_nan=False).encode()!=raw: raise ValueError()
        if (desired.get('kind')!=kind or desired.get('metadata',{}).get('name')!=name
            or desired['metadata'].get('namespace')!=NAMESPACE or desired['metadata'].get('annotations',{}).get(
                'voice.poryadok.io/prerequisites-operation')!=session.operation): raise ValueError()
        expected=private_semantic(desired)
        if resource_claim.digest(expected)!=effect['desired_sha256']: raise ValueError()
        session.check()
        with resource_claim.command_scope(session.check): actual=Kube().get(kind.lower(),name)
        meta=actual['metadata']; observed=private_semantic(actual)
        original=copy.deepcopy(observed); successor_uid=None
        if (actual.get('kind')!=kind or meta.get('name')!=name or meta.get('namespace')!=NAMESPACE
            or meta.get('uid')!=observation['uid']): raise ValueError()
        if kind=='Deployment':
            if (type(observation['generation']) is not int or observation['generation']<1
                or meta.get('generation')!=observation['generation']): raise ValueError()
            if observation['semantic_sha256'] not in (resource_claim.digest(observed),resource_claim.digest(expected)): raise ValueError()
            if observed!=expected:
                with resource_claim.command_scope(session.check): rs=Kube().run(['get','replicaset','-o','json'])
                if not isinstance(rs,dict) or not isinstance(rs.get('items'),list): raise ValueError()
                successor_uid=deployment_successor(desired,actual,rs['items']); observed=expected
        elif (meta.get('resourceVersion')!=observation['resourceVersion']
              or resource_claim.digest(observed)!=observation['semantic_sha256']): raise ValueError()
        if kind=='Service':
            service_allocation(desired,actual)
            observed['spec'].pop('clusterIP'); observed['spec'].pop('clusterIPs')
        if observed!=expected: raise ValueError()
        session.check()
        with resource_claim.command_scope(session.check):
            fresh=Kube().get(kind.lower(),name)
            rs2=Kube().run(['get','replicaset','-o','json']) if successor_uid else None
        if (fresh.get('metadata',{}).get('uid')!=observation['uid'] or private_semantic(fresh)!=original
            or (kind=='Deployment' and fresh['metadata'].get('generation')!=observation['generation'])
            or (kind!='Deployment' and fresh['metadata'].get('resourceVersion')!=observation['resourceVersion'])): raise ValueError()
        if successor_uid and (not isinstance(rs2,dict) or not isinstance(rs2.get('items'),list)
                              or deployment_successor(desired,fresh,rs2['items'])!=successor_uid): raise ValueError()
        session.check()
        if parent.require_child(session,'p3')!=plan: raise ValueError()
        return copy.deepcopy(observation)
    except Exception: raise PrerequisiteError('staging P3 effect readback invalid') from None


def _ready_snapshot(session,desired,record):
    """Read and check one bounded publisher snapshot; caller repeats after TLS."""
    import gis_existing_update
    import resource_claim
    from stage_runtime import Kube
    kube=Kube(); node_ref=record['node']
    with resource_claim.command_scope(session.check):
        deployment=kube.get('deployment','voice-p3-principal-jwks')
        service=kube.get('service','voice-p3-principal-jwks')
        node=kube.get('node',node_ref['name'])
        replicaset_list=kube.run(['get','replicasets','-o','json'])
        pod_list=kube.run(['get','pods','-o','json'])
        slice_list=kube.run(['get','endpointslices','-o','json'])
        account=gis_existing_update.service_account_preflight(kube,desired['spec']['template']['spec'])
    lists=(replicaset_list,pod_list,slice_list)
    for listing,limit in zip(lists,(2048,4096,2048)):
        if (not isinstance(listing,dict) or not isinstance(listing.get('items'),list) or len(listing['items'])>limit
            or not isinstance(listing.get('metadata',{}).get('resourceVersion'),str) or not listing['metadata']['resourceVersion']): raise ValueError()
    replicasets,pods,slices=[listing['items'] for listing in lists]
    rs_uid=deployment_successor(desired,deployment,replicasets)
    address=publisher_endpoint(deployment,replicasets,pods,slices,node,node_ref['uid'],'sha256:'+record['manifest_sha256'],service)
    pod=next(p for p in pods if p.get('metadata',{}).get('labels',{}).get('app')=='voice-p3-principal-jwks')
    rs=next(r for r in replicasets if r.get('metadata',{}).get('uid')==rs_uid)
    sl=next(s for s in slices if s.get('metadata',{}).get('labels',{}).get('kubernetes.io/service-name')=='voice-p3-principal-jwks')
    if not sl.get('metadata',{}).get('uid'): raise ValueError()
    gis_existing_update.verify_pod_spec(desired['spec']['template']['spec'],pod['spec'])
    status=pod['status']['containerStatuses'][0]
    proof={'schema':'voice-staging-p3-publisher-ready-v1',
           'deployment':{'uid':deployment['metadata']['uid'],'generation':deployment['metadata']['generation'],
                         'semantic_sha256':resource_claim.digest(private_semantic(deployment))},
           'replicaset':{'uid':rs_uid,'template_sha256':resource_claim.digest(rs['spec']['template'])},
           'pod':{'uid':pod['metadata']['uid'],'node':pod['spec']['nodeName'],'ip':address,
                  'spec_sha256':resource_claim.digest(pod['spec']),'image_id':status['imageID']},
           'node':{'name':node_ref['name'],'uid':node_ref['uid'],'os':node_ref['os'],'architecture':node_ref['architecture']},
           'service':{'uid':service['metadata']['uid'],'semantic_sha256':resource_claim.digest(private_semantic(service))},
           'endpoint_slice':{'uid':sl['metadata']['uid'],'semantic_sha256':resource_claim.digest(private_semantic(sl))},
           'service_account':account}
    session.check()
    return proof


def verify_complete(session):
    """Requalify all owned resources and the publisher under the root session."""
    import resource_claim
    import staging_prerequisites as parent
    import staging_p3_prerequisites_crypto as crypto
    from stage_runtime import Kube
    try:
        if type(session) is not resource_claim.Session: raise ValueError()
        session.check(); plan=validate_plan_shape(parent.require_child(session,'p3'))
        if plan['operation']!=session.operation: raise ValueError()
        publisher,record=_source_and_image(session,plan); issuer=parent.load_issuer(session,'p3')
        effects={r['id']:r for r in plan['effects']}; done=parent.load_done(session,'p3')
        if not isinstance(done,dict) or set(done)!=set(effects): raise ValueError()
        for ident in effects:
            if verify_effect(session,ident,done[ident])!=done[ident]: raise ValueError()
        kube=Kube(); current={}; provenances={}; observations={}
        with resource_claim.command_scope(session.check):
            ns=kube.get('namespace',NAMESPACE)
            for ident in inventory_intent():
                kind,name=ident.split('/'); current[ident]=kube.get(kind.lower(),name)
        if (ns.get('kind')!='Namespace' or ns.get('metadata',{}).get('name')!=NAMESPACE
            or ns['metadata'].get('uid')!=plan['namespace_uid'] or ns['metadata'].get('deletionTimestamp')): raise ValueError()
        for ident in inventory_intent():
            kind,name=ident.split('/'); row=current[ident]; meta=row['metadata']; semantic=private_semantic(row)
            if row.get('kind')!=kind or meta.get('name')!=name or meta.get('namespace')!=NAMESPACE: raise ValueError()
            if ident in effects:
                if meta.get('uid')!=done[ident]['uid']: raise ValueError()
                observations[ident]=copy.deepcopy(done[ident])
            else:
                previous=parent.load_preserved(session,'p3',ident); old=plan['preserved'][ident]
                if (not isinstance(previous,dict) or set(previous)!={'authority','semantic','uid'}
                    or previous['authority']!=old['authority'] or previous['uid']!=old['observed']['uid']
                    or meta.get('uid')!=previous['uid'] or semantic!=previous['semantic']
                    or resource_claim.digest(semantic)!=old['observed']['semantic_sha256']
                    or meta.get('annotations',{}).get('voice.poryadok.io/prerequisites-operation')!=previous['authority']['operation']): raise ValueError()
                provenances[ident]=previous
                obs={'kind':kind,'name':name,'namespace':NAMESPACE,'uid':meta['uid'],
                     'resourceVersion':meta['resourceVersion'],'semantic_sha256':resource_claim.digest(semantic)}
                if kind=='Deployment': obs['generation']=meta['generation']
                observations[ident]=obs
        values=_material_values(current); crypto.validate_issuer_materials(values,issuer)
        dep_id='Deployment/voice-p3-principal-jwks'
        if dep_id in effects:
            raw=parent.load_payload(session,'p3',dep_id); desired=json.loads(raw,object_pairs_hook=_unique_json)
        else: desired=copy.deepcopy(provenances[dep_id]['semantic'])
        proof=_ready_snapshot(session,desired,record)
        leaf=values['voice-p3-principal-jwks-tls']['tls.crt']
        leaf_hash=hashlib.sha256(ssl.PEM_cert_to_DER_cert(leaf.decode('ascii'))).hexdigest()
        body=values['voice-p3-principal-public']['realtime.json']
        probe_jwks(proof['pod']['ip'],issuer['trust']['realtime-jwks-ca.crt'].encode('ascii'),body,leaf_hash)
        after=_ready_snapshot(session,desired,record)
        if after!=proof: raise ValueError()
        proof['tls']={'leaf_der_sha256':leaf_hash,'jwks_sha256':hashlib.sha256(body).hexdigest()}
        with resource_claim.command_scope(session.check):
            ns2=kube.get('namespace',NAMESPACE)
            fresh={ident:kube.get(ident.split('/')[0].lower(),ident.split('/')[1]) for ident in inventory_intent()}
        if (ns2.get('metadata',{}).get('uid')!=plan['namespace_uid'] or ns2['metadata'].get('deletionTimestamp')): raise ValueError()
        for ident,row in current.items():
            again=fresh[ident]; kind=ident.split('/')[0]
            if (again.get('metadata',{}).get('uid')!=row['metadata'].get('uid') or private_semantic(again)!=private_semantic(row)
                or (kind!='Deployment' and again['metadata'].get('resourceVersion')!=row['metadata'].get('resourceVersion'))): raise ValueError()
        for ident in effects:
            if verify_effect(session,ident,done[ident])!=done[ident]: raise ValueError()
        for ident,previous in provenances.items():
            if parent.load_preserved(session,'p3',ident)!=previous: raise ValueError()
        if (parent.load_done(session,'p3')!=done or parent.require_child(session,'p3')!=plan
            or parent.load_issuer(session,'p3')!=issuer or _source_and_image(session,plan)!=(publisher,record)): raise ValueError()
        session.check()
        receipt={'schema':'voice-staging-p3-prerequisites-receipt-v1','operation':session.operation,
                 'plan_sha256':resource_claim.digest(plan),'objects':observations,'publisher_ready_sha256':resource_claim.digest(proof)}
        return {'receipt':receipt,'proof':proof}
    except Exception: raise PrerequisiteError('staging P3 completion not qualified') from None


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


def decode_material_payloads(payloads,operations):
    """Decode private payloads; operations must come from authenticated provenance."""
    try:
        ids=inventory_intent()[:len(ASSET_KEYS)]
        if (not isinstance(payloads,dict) or set(payloads)!=set(ids) or not isinstance(operations,dict)
            or set(operations)!=set(ids) or any(not isinstance(x,str) or not x for x in operations.values())): raise ValueError()
        def unique(pairs):
            result={}
            for k,v in pairs:
                if k in result: raise ValueError()
                result[k]=v
            return result
        values={}; decoded=[]
        for ident in ids:
            raw=payloads[ident]
            if not isinstance(raw,bytes) or not 0<len(raw)<=1048576: raise ValueError()
            obj=json.loads(raw,object_pairs_hook=unique)
            if json.dumps(obj,sort_keys=True,separators=(',',':'),allow_nan=False).encode()!=raw: raise ValueError()
            kind,name=ident.split('/'); data=obj['data']
            if not isinstance(data,dict) or set(data)!=ASSET_KEYS[name]: raise ValueError()
            values[name]={key:(value.encode('utf-8') if kind=='ConfigMap' else base64.b64decode(value,validate=True))
                          for key,value in data.items()}
            decoded.append(obj)
        expected=material_objects(values)
        for obj in expected:
            ident=obj['kind']+'/'+obj['metadata']['name']
            obj['metadata']['annotations']={'voice.poryadok.io/prerequisites-operation':operations[ident]}
        if decoded!=expected: raise ValueError()
        return values
    except Exception: raise PrerequisiteError('staging P3 private payload invalid') from None


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
