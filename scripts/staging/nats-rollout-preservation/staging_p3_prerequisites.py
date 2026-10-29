"""Fixed STAGING P3 object contract; pure helpers do not grant authority."""
import base64
import copy
import re
import source_plan


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
