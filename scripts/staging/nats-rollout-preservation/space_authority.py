"""Fixed existing Space database authority; private values never enter receipts."""
import base64
import hashlib
import re
from urllib.parse import urlsplit,parse_qs

class AuthorityError(ValueError):pass
def fail():raise AuthorityError('space_database_authority_rejected')

def identity(row):
    metadata=row['metadata'];uid=metadata['uid'];rv=metadata['resourceVersion']
    if not isinstance(uid,str) or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}',uid):fail()
    if not isinstance(rv,str) or not re.fullmatch(r'[0-9]{1,20}',rv):fail()
    return {'uid':uid,'resourceVersion':rv}

def capture(kube):
    try:
        namespace=kube.get('namespace','voice-staging')
        controller=kube.get('statefulset','voice-postgres')
        pod=kube.get('pod','voice-postgres-0')
        claim=kube.get('persistentvolumeclaim','pgdata-voice-postgres-0')
        if controller['spec'].get('replicas',1)!=1 or pod['status']['phase']!='Running' or claim['status']['phase']!='Bound':fail()
        owner=[o for o in pod['metadata'].get('ownerReferences',[]) if o.get('controller') is True]
        if len(owner)!=1 or owner[0].get('kind')!='StatefulSet' or owner[0].get('name')!='voice-postgres' or owner[0].get('uid')!=controller['metadata']['uid']:fail()
        claims=[v['persistentVolumeClaim']['claimName'] for v in pod['spec']['volumes'] if 'persistentVolumeClaim' in v]
        if claims!=['pgdata-voice-postgres-0']:fail()
        statuses=[r for r in pod['status']['containerStatuses'] if r['name']=='postgres']
        if len(statuses)!=1 or statuses[0].get('ready') is not True:fail()
        image=statuses[0]['imageID'].removeprefix('docker-pullable://')
        if not re.fullmatch(r'docker.io/library/postgres@sha256:[a-f0-9]{64}',image):fail()
        pv_name=claim['spec']['volumeName']
        if not isinstance(pv_name,str) or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}',pv_name):fail()
        pv=kube.get('persistentvolume',pv_name)
        ref=pv['spec']['claimRef']
        if ref.get('uid')!=claim['metadata']['uid'] or ref.get('name')!='pgdata-voice-postgres-0' or ref.get('namespace')!='voice-staging':fail()
        secret=kube.get('secret','voice-app-secrets');data=secret['data']
        password=base64.b64decode(data['POSTGRES_PASSWORD'],validate=True)
        raw=base64.b64decode(data['SPACE_DATABASE_URL'],validate=True)
        if not password or len(password)>4096 or not 1<=len(raw)<=8192:fail()
        url=urlsplit(raw.decode('utf-8'));query=parse_qs(url.query,strict_parsing=True)
        hosts=('voice-postgres','voice-postgres.voice-staging.svc','voice-postgres.voice-staging.svc.cluster.local')
        if url.scheme not in ('postgres','postgresql') or url.hostname not in hosts or url.port not in (None,5432) or url.path!='/space_db' or not url.username or not url.password or url.fragment:fail()
        if query!={'sslmode':['disable']}:fail()
        wanted=hashlib.sha256(password).hexdigest()
        observed=kube.run(['exec','voice-postgres-0','--','sh','-ceu',
            'printf "{\\"password_sha256\\":\\"%s\\"}" "$(printf %s "$POSTGRES_PASSWORD" | sha256sum | cut -d" " -f1)"'])
        if observed!={'password_sha256':wanted}:fail()
        result={'schema':'voice-space-authority-v1','namespace':identity(namespace),'controller':identity(controller),
            'pod':identity(pod),'claim':identity(claim),'pv':identity(pv),'pv_name':pv_name,'image':image,
            'secret':identity(secret),'space_url_sha256':hashlib.sha256(raw).hexdigest(),'postgres_password_sha256':wanted}
        if kube.secret_meta('voice-app-secrets')!=result['secret']:fail()
        return result
    except AuthorityError:raise
    except Exception:fail()

def revalidate(kube,binding):
    if capture(kube)!=binding:fail()
