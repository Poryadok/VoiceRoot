"""Fixed pre-cut Space24 inspection under the claimed root operation."""
import base64
import copy
import hashlib
import re
import time

from controller import Blocked
from stage_runtime import Kube
import realtime_bootstrap
import staging_cohort_preservation as owner
import staging_cohort_preservation_pg as pg
import staging_cohort_preservation_space as space


IDENTITY_SQL="""SELECT json_build_object('system_identifier',(pg_control_system()).system_identifier::text,
 'server_start',pg_postmaster_start_time()::text,'database_oid',d.oid::bigint,
 'database_owner',pg_get_userbyid(d.datdba),'session_user',session_user,
 'schema_owner',pg_get_userbyid(n.nspowner)) FROM pg_database d CROSS JOIN pg_namespace n
 WHERE d.datname=current_database() AND d.datname='space_db' AND n.nspname='public';"""
PG_CREDENTIAL_SCRIPT='printf "%s\\n" "$POSTGRES_USER"; printf %s "$POSTGRES_PASSWORD" | sha256sum'


def _identity(session,kube):
    namespace,before=realtime_bootstrap.engine_snapshot(kube)
    value=pg._query(session,'space_db',IDENTITY_SQL,'space-inspect-identity')
    if realtime_bootstrap.engine_snapshot(kube)!=(namespace,before):raise Blocked('space_inspect_postgres_changed')
    if (type(value) is not dict or set(value)!={'system_identifier','server_start','database_oid','database_owner','session_user','schema_owner'}
        or not isinstance(value['system_identifier'],str) or not value['system_identifier'].isdigit()
        or type(value['database_oid']) is not int or value['database_oid']<=0
        or any(not isinstance(value[k],str) or not value[k] for k in ('server_start','database_owner','session_user','schema_owner'))):
        raise Blocked('space_inspect_identity_invalid')
    return {'namespace_uid':namespace,'engine':before,'database':value}


def _job(session,tool,node):
    name='voice-space-inspect-'+session.operation
    if not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}',node):raise Blocked('space_inspect_node_invalid')
    image=tool['immutable_image'];build=tool['source_inventory']['build_source_sha256']
    if (not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/space-native-installer@sha256:[a-f0-9]{64}',image)
        or not re.fullmatch(r'[a-f0-9]{64}',build)):raise Blocked('space_inspect_tool_invalid')
    labels={'voice-space-inspection':session.operation}
    return {'apiVersion':'batch/v1','kind':'Job','metadata':{'name':name,'namespace':'voice-staging','labels':labels},
        'spec':{'backoffLimit':0,'activeDeadlineSeconds':300,'template':{'metadata':{'labels':labels},'spec':{
            'nodeName':node,'restartPolicy':'Never','automountServiceAccountToken':False,
            'securityContext':{'runAsNonRoot':True,'runAsUser':65532,'runAsGroup':65532},
            'containers':[{'name':'inspect','image':image,'imagePullPolicy':'IfNotPresent',
                'args':['-mode','inspect-legacy24','-migrations-dir','/migrations','-migrate','/usr/local/bin/migrate'],
                'env':[{'name':'PGHOST','value':'voice-postgres'},{'name':'PGPORT','value':'5432'},
                    {'name':'PGUSER','valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
                    {'name':'PGPASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}},
                    {'name':'PGSSLMODE','value':'disable'},{'name':'SPACE_NATIVE_REQUIRED_SOURCE_SHA256','value':build}],
                'securityContext':{'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,'capabilities':{'drop':['ALL']}}}]}}}}


def _mutate(session,ident,argv,body):
    owner._guard(session);owner._no_pending(session)
    name='voice-space-inspect-'+session.operation
    if ident=='space-inspect-create':
        expected_argv=owner.PREFIX+['create','-f','/dev/stdin','-o','json']
        node=Kube().get('pod','voice-postgres-0').get('spec',{}).get('nodeName')
        expected=_job(session,space.load_tool(session),node)
        if argv!=expected_argv or body!=owner.private._bytes(expected):raise Blocked('space_inspect_effect_invalid')
    elif ident=='space-inspect-delete':
        expected_argv=owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/'+name,'-f','/dev/stdin']
        value=owner.private._json(body);current=Kube().get('job',name).get('metadata',{})
        created=owner._read(owner._base(session)/'effects'/'space-inspect-create.json')
        if (argv!=expected_argv or value!={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
                'preconditions':{'uid':current.get('uid'),'resourceVersion':current.get('resourceVersion')}}
            or created.get('status')!='DONE' or created.get('response',{}).get('uid')!=current.get('uid')
            or not owner.guard.UUID.fullmatch(str(current.get('uid',''))) or not current.get('resourceVersion')):
            raise Blocked('space_inspect_effect_invalid')
    else:raise Blocked('space_inspect_effect_invalid')
    folder=owner._base(session)/'effects';owner.private._private_directory(folder)
    intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':ident,
        'binding':session.binding(),'argv':argv,'body_sha256':hashlib.sha256(body).hexdigest(),'status':'PENDING'}
    path=folder/(ident+'.json');owner._write(path,intent)
    raw=owner._send(session,ident,argv,body,timeout=60)
    if owner._read(path)!=intent:raise Blocked('space_inspect_effect_changed')
    return path,intent,raw


def _done(path,intent,response):
    owner.save(path,{**intent,'status':'DONE','response':response})


def _check_template(actual,wanted):
    """Reject admission changes to the inspector's execution and credential scope."""
    actual=copy.deepcopy(actual)
    if type(actual) is not dict:raise Blocked('space_inspect_job_invalid')
    for container in actual.get('containers',[]):
        if container.get('terminationMessagePath')=='/dev/termination-log':container.pop('terminationMessagePath')
        if container.get('terminationMessagePolicy')=='File':container.pop('terminationMessagePolicy')
        if container.get('resources')=={}:container.pop('resources')
    if any(actual.get(k)!=v for k,v in wanted.items()):
        raise Blocked('space_inspect_job_invalid')
    defaults={'dnsPolicy':'ClusterFirst','schedulerName':'default-scheduler','terminationGracePeriodSeconds':30,
              'serviceAccountName':'default','serviceAccount':'default','enableServiceLinks':True,
              'priority':0,'preemptionPolicy':'PreemptLowerPriority'}
    if any(key not in wanted and (key not in defaults or value!=defaults[key]) for key,value in actual.items()):
        raise Blocked('space_inspect_job_invalid')


def _job_list(session,name):
    raw=owner._observation(session,'space-inspect-absence',owner.PREFIX+['get','--raw',
        '/apis/batch/v1/namespaces/voice-staging/jobs?fieldSelector=metadata.name%3D'+name+'&limit=2'])
    result=owner.private._json(raw)
    if (result.get('apiVersion')!='batch/v1' or result.get('kind')!='JobList' or not isinstance(result.get('metadata',{}).get('resourceVersion'),str)
        or not result['metadata']['resourceVersion']
        or result['metadata'].get('continue') or not isinstance(result.get('items'),list)):
        raise Blocked('space_inspect_absence_invalid')
    return result['items']


def _pod_list(session,name):
    raw=owner._observation(session,'space-inspect-pod-absence',owner.PREFIX+['get','pods','-l','job-name='+name,'-o','json'])
    result=owner.private._json(raw)
    if (result.get('apiVersion')!='v1' or result.get('kind')!='PodList'
        or not isinstance(result.get('metadata',{}).get('resourceVersion'),str)
        or not result['metadata']['resourceVersion'] or result['metadata'].get('continue')
        or not isinstance(result.get('items'),list)):
        raise Blocked('space_inspect_absence_invalid')
    return result['items']


def _check_delete_response(response,uid):
    status=(type(response) is dict and response.get('apiVersion')=='v1'
            and response.get('kind')=='Status' and response.get('status')=='Success')
    job=(type(response) is dict and response.get('apiVersion')=='batch/v1'
         and response.get('kind')=='Job' and response.get('metadata',{}).get('uid')==uid)
    if not (status or job):raise Blocked('space_inspect_delete_invalid')


def _cleanup(session,kube,name,uid):
    job=kube.get('job',name);meta=job['metadata']
    if meta.get('uid')!=uid or meta.get('deletionTimestamp'):raise Blocked('space_inspect_job_changed')
    options={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
             'preconditions':{'uid':uid,'resourceVersion':meta['resourceVersion']}}
    path,intent,raw=_mutate(session,'space-inspect-delete',owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/'+name,'-f','/dev/stdin'],owner.private._bytes(options))
    response=owner.private._json(raw)
    _check_delete_response(response,uid)
    _done(path,intent,{'uid':uid,'resourceVersion':meta['resourceVersion']})
    deadline=time.monotonic()+60
    while True:
        items=_job_list(session,name)
        if not items and not _pod_list(session,name):return
        if items and (len(items)!=1 or items[0].get('metadata',{}).get('uid')!=uid
            or items[0]['metadata'].get('name')!=name or items[0]['metadata'].get('namespace')!='voice-staging'):
            raise Blocked('space_inspect_job_changed')
        if time.monotonic()>=deadline:raise Blocked('space_inspect_cleanup_timeout')
        time.sleep(1)


def inspect_source(session):
    """Run the pinned read-only inspector and remove only its response-bound Job."""
    plan=owner._guard(session);owner._no_pending(session);tool=space.load_tool(session);kube=Kube()
    identity=_identity(session,kube);config=kube.get('configmap','voice-app-config');secret=kube.get('secret','voice-app-secrets')
    user=config.get('data',{}).get('POSTGRES_USER')
    if (not isinstance(user,str) or not re.fullmatch('[a-z_][a-z0-9_]{0,62}',user)
        or not secret.get('data',{}).get('POSTGRES_PASSWORD') or identity['database']['session_user']!=user):
        raise Blocked('space_inspect_credentials_changed')
    try:password=base64.b64decode(secret['data']['POSTGRES_PASSWORD'],validate=True)
    except (ValueError,TypeError):raise Blocked('space_inspect_credentials_changed') from None
    observed=owner._observation(session,'space-inspect-credentials',owner.PREFIX+['exec','voice-postgres-0','-c','postgres',
        '--','sh','-ceu',PG_CREDENTIAL_SCRIPT],limit=1024).decode('ascii').splitlines()
    if len(observed)!=2 or observed[0]!=user or observed[1].split()!=[hashlib.sha256(password).hexdigest(),'-']:
        raise Blocked('space_inspect_credentials_changed')
    pod=kube.get('pod','voice-postgres-0');node=pod.get('spec',{}).get('nodeName')
    pg_containers=[item for item in pod.get('spec',{}).get('containers',[]) if item.get('name')=='postgres']
    if len(pg_containers)!=1:raise Blocked('space_inspect_credentials_changed')
    env=pg_containers[0].get('env',[])
    for name,ref in (('POSTGRES_USER',{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}),
                     ('POSTGRES_PASSWORD',{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}})):
        rows=[item for item in env if item.get('name')==name]
        if rows!=[{'name':name,'valueFrom':ref}]:raise Blocked('space_inspect_credentials_changed')
    wanted=_job(session,tool,node);name=wanted['metadata']['name']
    # Server dry-run qualifies the exact requested Job before the create intent.
    dry=kube.run(['create','--dry-run=server','-f','/dev/stdin','-o','json'],body=wanted)
    if dry.get('kind')!='Job' or dry.get('metadata',{}).get('name')!=name:
        raise Blocked('space_inspect_job_invalid')
    _check_template(dry.get('spec',{}).get('template',{}).get('spec',{}),wanted['spec']['template']['spec'])
    path,intent,raw=_mutate(session,'space-inspect-create',owner.PREFIX+['create','-f','/dev/stdin','-o','json'],owner.private._bytes(wanted))
    created=owner.private._json(raw);meta=created.get('metadata',{});uid=meta.get('uid')
    if (created.get('kind')!='Job' or meta.get('name')!=name or meta.get('namespace')!='voice-staging'
        or not isinstance(uid,str) or not owner.guard.UUID.fullmatch(uid) or not meta.get('resourceVersion')
        or created.get('spec',{}).get('template',{}).get('spec',{})!=dry.get('spec',{}).get('template',{}).get('spec',{})):
        raise Blocked('space_inspect_create_invalid')
    _done(path,intent,{'uid':uid,'resourceVersion':meta['resourceVersion'],'spec_sha256':owner.digest(created['spec'])})
    try:
        deadline=time.monotonic()+310
        while True:
            job=kube.get('job',name)
            if job['metadata'].get('uid')!=uid or job['spec']!=created['spec']:raise Blocked('space_inspect_job_changed')
            if job.get('status',{}).get('succeeded')==1 and not job.get('status',{}).get('active',0):break
            if job.get('status',{}).get('failed',0)>0 or time.monotonic()>=deadline:raise Blocked('space_inspect_job_failed')
            time.sleep(1)
        rows=_pod_list(session,name)
        if len(rows)!=1:raise Blocked('space_inspect_pod_invalid')
        row=rows[0];owners=[item for item in row['metadata'].get('ownerReferences',[]) if item.get('controller') is True]
        statuses=[item for item in row.get('status',{}).get('containerStatuses',[]) if item.get('name')=='inspect']
        if (len(owners)!=1 or owners[0].get('kind')!='Job' or owners[0].get('uid')!=uid
            or row['spec'].get('nodeName')!=node or len(statuses)!=1
            or not owner.guard.UUID.fullmatch(str(row['metadata'].get('uid','')))
            or row['spec'].get('containers')!=created['spec']['template']['spec']['containers']
            or row['spec'].get('automountServiceAccountToken') is not False
            or statuses[0].get('state',{}).get('terminated',{}).get('exitCode')!=0
            or not any(statuses[0].get('imageID','').endswith(tool[key]) for key in ('platform_digest','config_digest'))):
            raise Blocked('space_inspect_pod_invalid')
        raw=owner._observation(session,'space-inspect-log',owner.PREFIX+['logs',row['metadata']['name'],'-c','inspect'],limit=1<<20)
        result=owner.private._json(raw);target=result.get('target',{});content=tool['content']
        if (set(result)!={'event','schema','source_revision','build_sha256','executable_sha256','migrator_sha256','inventory_sha256','target'}
            or result['event']!='inspected-legacy24' or result['schema']!='voice.space.native-legacy24-inspection.v1'
            or result['source_revision']!=session.context['commit'] or result['build_sha256']!=tool['source_inventory']['build_source_sha256']
            or result['executable_sha256']!=content['executable_sha256'] or result['migrator_sha256']!=content['migrator_sha256']
            or result['inventory_sha256']!=content['sql_inventory_sha256'] or set(target)!={'database_name','database_oid','database_system_id','database_owner','installer_role','version'}
            or target!={'database_name':'space_db','database_oid':identity['database']['database_oid'],
                'database_system_id':identity['database']['system_identifier'],'database_owner':identity['database']['database_owner'],
                'installer_role':user,'version':24}):
            raise Blocked('space_inspect_result_changed')
        if _identity(session,kube)!=identity or kube.get('configmap','voice-app-config')!=config or kube.get('secret','voice-app-secrets')!=secret:
            raise Blocked('space_inspect_source_changed')
    except BaseException:
        _cleanup(session,kube,name,uid)
        raise
    _cleanup(session,kube,name,uid)
    owner._guard(session)
    receipt={'schema':'voice-staging-space-inspection-v1','operation':session.operation,'plan_sha256':owner.digest(plan),
             'tool_sha256':owner.digest(tool),'identity':identity,'job_uid':uid,'result':result,'cleanup':'verified',
             'credential_binding':{'config_uid':config['metadata']['uid'],'config_rv':config['metadata']['resourceVersion'],
                 'config_sha256':owner.digest(config),'secret_uid':secret['metadata']['uid'],
                 'secret_rv':secret['metadata']['resourceVersion'],'secret_sha256':owner.digest(secret)}}
    owner._write(owner._base(session)/'space-inspection.json',receipt)
    return copy.deepcopy(receipt)


def load_inspection(session):
    """Reload the protected completed inspection and recheck its original inputs."""
    plan=owner._guard(session);owner._no_pending(session);tool=space.load_tool(session)
    receipt=owner._read(owner._base(session)/'space-inspection.json');kube=Kube()
    fields={'schema','operation','plan_sha256','tool_sha256','identity','job_uid','result','cleanup','credential_binding'}
    if (type(receipt) is not dict or set(receipt)!=fields or receipt['schema']!='voice-staging-space-inspection-v1'
        or receipt['operation']!=session.operation or receipt['plan_sha256']!=owner.digest(plan)
        or receipt['tool_sha256']!=owner.digest(tool) or receipt['cleanup']!='verified'):
        raise Blocked('space_inspect_receipt_changed')
    created=owner._read(owner._base(session)/'effects'/'space-inspect-create.json')
    deleted=owner._read(owner._base(session)/'effects'/'space-inspect-delete.json')
    if (created.get('status')!='DONE' or deleted.get('status')!='DONE'
        or created.get('response',{}).get('uid')!=receipt['job_uid']
        or deleted.get('response',{}).get('uid')!=receipt['job_uid']):raise Blocked('space_inspect_receipt_changed')
    if _identity(session,kube)!=receipt['identity']:raise Blocked('space_inspect_postgres_changed')
    config=kube.get('configmap','voice-app-config');secret=kube.get('secret','voice-app-secrets')
    actual={'config_uid':config['metadata']['uid'],'config_rv':config['metadata']['resourceVersion'],
        'config_sha256':owner.digest(config),'secret_uid':secret['metadata']['uid'],
        'secret_rv':secret['metadata']['resourceVersion'],'secret_sha256':owner.digest(secret)}
    if actual!=receipt['credential_binding']:raise Blocked('space_inspect_credentials_changed')
    name='voice-space-inspect-'+session.operation
    if _job_list(session,name) or _pod_list(session,name):raise Blocked('space_inspect_cleanup_changed')
    owner._guard(session)
    return copy.deepcopy(receipt)
