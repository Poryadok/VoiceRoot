"""Root-only target-bound database migration subphase; no NATS resources."""
import hashlib
import base64
import re
from urllib.parse import quote
import guard
from controller import Blocked

IMAGE='migrate/migrate@sha256:f21c436af23c282f4516b00ba3e93bccf5c5fe5cd52530fd5c319a936998f539'
FLYWAY_IMAGE='flyway/flyway@sha256:c8cf894f9b63e8b3a50bb7a89f65b84dbd147d3203d8bc502f7c570b7d440895'
DATABASES=('bot','story','moderation','subscription','user','search','social','chat',
           'messaging','file','space','role','notification','matchmaking','voice')
LEGACY_DSN=('bot','story','moderation','subscription','search')
CREATE_PROJECTION="jsonpath-as-json={.metadata['name','uid']}"

def validate_plan(plan,mode):
    if mode not in ('full','app-only','images-only') or not isinstance(plan,list) or len(plan)>len(DATABASES)+1:
        raise Blocked('rollout_migration_plan_invalid')
    if mode=='images-only' and plan:raise Blocked('rollout_images_only_migration_forbidden')
    seen=set();total=0
    for row in plan:
        if not isinstance(row,dict) or set(row)!={'database','image','secret_key','files','hashes'}:
            raise Blocked('rollout_migration_shape_invalid')
        database=row['database']
        auth=database=='auth'
        if database not in (*DATABASES,'auth') or database in seen or row['image']!=(FLYWAY_IMAGE if auth else IMAGE) or row['secret_key']!=('POSTGRES_PASSWORD' if auth else database.upper()+'_DATABASE_URL'):
            raise Blocked('rollout_migration_authority_invalid')
        seen.add(database);files=row['files'];hashes=row['hashes']
        if not isinstance(files,dict) or not 1<=len(files)<=256 or not isinstance(hashes,dict) or set(files)!=set(hashes):
            raise Blocked('rollout_migration_files_invalid')
        for name,text in files.items():
            pattern=r'V[0-9]{1,20}__[a-zA-Z0-9_-]{1,100}\.sql' if auth else r'[0-9]{1,20}_[a-zA-Z0-9_-]{1,100}\.(up|down)\.sql'
            if not isinstance(name,str) or not re.fullmatch(pattern,name) or not isinstance(text,str):
                raise Blocked('rollout_migration_file_invalid')
            raw=text.encode('utf-8');total+=len(raw)
            if not 1<=len(raw)<=262144 or total>1048576 or hashlib.sha256(raw).hexdigest()!=hashes[name]:
                raise Blocked('rollout_migration_digest_invalid')
    return plan

def documents(plan,operation):
    if not re.fullmatch(r'[a-f0-9]{12}',operation):raise Blocked('rollout_migration_operation_invalid')
    result=[]
    for row in plan:
        name='voice-rollout-'+operation+'-'+row['database']
        metadata={'name':name,'namespace':guard.NS,
                  'labels':{'voice-rollout-operation':operation,'voice-rollout-database':row['database']}}
        result.append({'apiVersion':'v1','kind':'ConfigMap','metadata':metadata,
                       'immutable':True,'data':dict(row['files'])})
        result.append({'apiVersion':'batch/v1','kind':'Job','metadata':metadata,
            'spec':{'backoffLimit':0,'activeDeadlineSeconds':300,
                'template':{'metadata':{'labels':metadata['labels']},'spec':{
                    'restartPolicy':'Never','automountServiceAccountToken':False,
                    'securityContext':{'runAsNonRoot':True,'runAsUser':65532,'runAsGroup':65532},
                    'containers':[{'name':'migrate','image':IMAGE,
                        'args':['-path=/migrations','-database=$(DATABASE_URL)','up'],
                        'env':[{'name':'DATABASE_URL','valueFrom':{'secretKeyRef':{
                            'name':'voice-app-secrets','key':row['secret_key']}}}],
                        'securityContext':{'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,
                                           'capabilities':{'drop':['ALL']}},
                        'volumeMounts':[{'name':'migrations','mountPath':'/migrations','readOnly':True}]}],
                    'volumes':[{'name':'migrations','configMap':{'name':name,'optional':False}}]
                }}}})
        if row['database']=='auth':
            container=result[-1]['spec']['template']['spec']['containers'][0]
            container['name']='flyway';container['image']=FLYWAY_IMAGE;container['args']=['repair','migrate']
            container['env']=[{'name':'FLYWAY_URL','value':'jdbc:postgresql://voice-postgres:5432/auth_db'},
                {'name':'FLYWAY_USER','valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
                {'name':'FLYWAY_PASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}},
                {'name':'FLYWAY_LOCATIONS','value':'filesystem:/flyway/sql'}]
            container['volumeMounts'][0]['mountPath']='/flyway/sql'
        elif row['database'] in LEGACY_DSN:
            result[-1]['spec']['template']['spec']['containers'][0]['env'][0]['valueFrom']['secretKeyRef']['name']='voice-rollout-'+operation+'-database-urls'
    return result

def preflight(kube,plan,mode):
    validate_plan(plan,mode)
    if not plan:return None
    metadata=kube.secret_meta('voice-app-secrets')
    for row in plan:
        # Projection emits only a boolean, never a Secret value.
        key='POSTGRES_PASSWORD' if row['database'] in LEGACY_DSN else row['secret_key']
        template='{{if index .data "'+key+'"}}true{{else}}false{{end}}'
        if kube.run(['get','secret','voice-app-secrets','-o','go-template='+template]) is not True:
            raise Blocked('rollout_database_url_missing')
    if kube.secret_meta('voice-app-secrets')!=metadata:raise Blocked('rollout_database_secret_changed')
    if any(r['database'] in (*LEGACY_DSN,'auth') for r in plan):
        config=kube.get('configmap','voice-app-config')
        user=config.get('data',{}).get('POSTGRES_USER','')
        if not isinstance(user,str) or not re.fullmatch(r'[a-z_][a-z0-9_]{0,31}',user):
            raise Blocked('rollout_database_authority_invalid')
        metadata=dict(metadata,app_config={k:config['metadata'][k] for k in ('uid','resourceVersion')})
    return metadata

def legacy_credentials(kube,plan,operation,secret_metadata):
    legacy=[r for r in plan if r['database'] in LEGACY_DSN]
    if not legacy:return None
    row=kube.get('secret','voice-app-secrets')
    if {k:row['metadata'].get(k) for k in ('uid','resourceVersion')}!={k:secret_metadata[k] for k in ('uid','resourceVersion')}:
        raise Blocked('rollout_database_secret_changed')
    try:
        password=base64.b64decode(row['data']['POSTGRES_PASSWORD'],validate=True).decode('utf-8')
        if not password or len(password)>4096:raise ValueError()
        config=kube.get('configmap','voice-app-config')
        if {k:config['metadata'].get(k) for k in ('uid','resourceVersion')}!=secret_metadata['app_config']:
            raise Blocked('rollout_database_config_changed')
        user=config['data']['POSTGRES_USER']
        if not isinstance(user,str) or not re.fullmatch(r'[a-z_][a-z0-9_]{0,31}',user):raise ValueError()
    except (KeyError,ValueError,UnicodeError):raise Blocked('rollout_database_authority_invalid') from None
    data={r['secret_key']:base64.b64encode(('postgres://'+quote(user,safe='')+':'+quote(password,safe='')+
         '@voice-postgres:5432/'+r['database']+'_db?sslmode=disable').encode()).decode('ascii') for r in legacy}
    return {'apiVersion':'v1','kind':'Secret','type':'Opaque','immutable':True,
            'metadata':{'name':'voice-rollout-'+operation+'-database-urls','namespace':guard.NS,
                        'labels':{'voice-rollout-operation':operation}},'data':data}

def execute(kube,stage,plan,mode,secret_metadata,journal):
    if preflight(kube,plan,mode)!=secret_metadata:raise Blocked('rollout_database_secret_changed')
    completed=[]
    copied=legacy_credentials(kube,plan,stage.operation,secret_metadata)
    if copied is not None:
        stage.verify_final_storage()
        # Copy existing DB authority only inside the claimed migration phase.
        # It is neither printed nor included in NATS data archives/receipts.
        result=kube.run(['create','-f','/dev/stdin','-o',CREATE_PROJECTION],body=copied)
        if not isinstance(result,list) or len(result)!=2 or result[0]!=copied['metadata']['name'] or not isinstance(result[1],str) or not guard.UUID.fullmatch(result[1]):
            raise Blocked('rollout_migration_secret_create_invalid')
        journal({'kind':'migration_authority_copy_created','name':result[0],'uid':result[1]})
    compiled=documents(plan,stage.operation)
    for cm,job in zip(compiled[::2],compiled[1::2]):
        stage.verify_final_storage()
        if kube.secret_meta('voice-app-secrets')!={k:secret_metadata[k] for k in ('uid','resourceVersion')}:raise Blocked('rollout_database_secret_changed')
        # create, never apply/delete: another operation's same-name resources
        # cannot be adopted or overwritten, including a failed old Job.
        identities=[]
        for wanted in (cm,job):
            stage.verify_final_storage()
            result=kube.run(['create','-f','/dev/stdin','-o',CREATE_PROJECTION],body=wanted)
            if not isinstance(result,list) or len(result)!=2 or result[0]!=wanted['metadata']['name'] or not isinstance(result[1],str) or not guard.UUID.fullmatch(result[1]):
                raise Blocked('rollout_migration_create_invalid')
            identities.append(result[1])
            journal({'kind':'migration_resource_created','resource_kind':wanted['kind'],
                     'name':result[0],'uid':result[1]})
        stage.verify_final_storage()
        result=kube.run(['wait','--for=condition=complete','job/'+job['metadata']['name'],
                         '--timeout=300s','-o','json'],timeout=310)
        if result.get('metadata',{}).get('uid')!=identities[1] or result.get('status',{}).get('succeeded')!=1:
            raise Blocked('rollout_migration_not_complete')
        stage.verify_final_storage()
        completed.append({'name':job['metadata']['name'],'uid':identities[1],
                          'configmap_uid':identities[0]})
        journal({'kind':'migration_complete',**completed[-1]})
    return completed
