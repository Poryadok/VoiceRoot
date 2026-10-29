"""Fixed pre-cut Space24 inspection under the claimed root operation."""
import base64
import contextvars
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
_HISTORY_STACK=contextvars.ContextVar('space_native_history_stack',default=())
LIVE_UPGRADE_SCRIPT='''umask 077
set -C
/usr/local/bin/native-migrate -mode upgrade-legacy24 -migrations-dir /migrations -migrate /usr/local/bin/migrate -operation-id "$1" -snapshot-id "$2" -origin-evidence /evidence/origin.json > /evidence/completed.jsonl
sync -f /evidence/completed.jsonl
sync -f /evidence
sha256sum /evidence/origin.json /evidence/completed.jsonl
'''
LIVE_VERIFY_SCRIPT='''umask 077
/usr/local/bin/native-migrate -mode verify-legacy24 -migrations-dir /migrations -migrate /usr/local/bin/migrate -prior-evidence /evidence/completed.jsonl -origin-evidence /evidence/origin.json > /dev/null
sha256sum /evidence/origin.json /evidence/completed.jsonl
printf 'verification=ok\\n'
'''
LIVE_BIND_SCRIPT='test -w /evidence; exec sleep 300'


def _live_documents(session,tool,node,storage_class,snapshot):
    """Build only the operation-owned PVC and the two fixed Tool jobs."""
    operation=session.operation;owner_id=session.context['operation_id']
    if (not re.fullmatch(r'[a-f0-9]{12}',operation)
        or not isinstance(node,str) or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}',node)
        or not isinstance(storage_class,str) or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}',storage_class)
        or any(not isinstance(value,str) or not re.fullmatch(r'[A-Za-z0-9._:-]{1,256}',value) for value in (owner_id,snapshot))):
        raise Blocked('space_live_binding_invalid')
    image=tool['immutable_image'];build=tool['source_inventory']['build_source_sha256']
    if (not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/space-native-installer@sha256:[a-f0-9]{64}',image)
        or not re.fullmatch(r'[a-f0-9]{64}',build)):raise Blocked('space_live_tool_invalid')
    claim='voice-space-native-'+operation;labels={'voice-space-native-operation':operation}
    pvc={'apiVersion':'v1','kind':'PersistentVolumeClaim','metadata':{'name':claim,'namespace':'voice-staging','labels':labels},
        'spec':{'accessModes':['ReadWriteOnce'],'storageClassName':storage_class,'resources':{'requests':{'storage':'16Mi'}}}}
    env=[{'name':'PGHOST','value':'voice-postgres'},{'name':'PGPORT','value':'5432'},
        {'name':'PGUSER','valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
        {'name':'PGPASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}},
        {'name':'PGSSLMODE','value':'disable'},{'name':'SPACE_NATIVE_REQUIRED_SOURCE_SHA256','value':build}]
    def job(mode):
        verify=mode=='verify';probe=mode=='bind';name=claim+'-'+mode
        return {'apiVersion':'batch/v1','kind':'Job','metadata':{'name':name,'namespace':'voice-staging','labels':labels},
            'spec':{'backoffLimit':0,'activeDeadlineSeconds':360,'template':{'metadata':{'labels':labels},'spec':{
                'affinity':{'nodeAffinity':{'requiredDuringSchedulingIgnoredDuringExecution':{'nodeSelectorTerms':[
                    {'matchFields':[{'key':'metadata.name','operator':'In','values':[node]}]}]}}},
                'restartPolicy':'Never','automountServiceAccountToken':False,
                'securityContext':{'runAsNonRoot':True,'runAsUser':65532,'runAsGroup':65532},
                'volumes':[{'name':'evidence','persistentVolumeClaim':{'claimName':claim,'readOnly':verify}}],
                'containers':[{'name':'native','image':image,'imagePullPolicy':'IfNotPresent',
                    'command':['/bin/sh','-ceu',LIVE_BIND_SCRIPT if probe else LIVE_VERIFY_SCRIPT if verify else LIVE_UPGRADE_SCRIPT,'native-space',owner_id,snapshot],
                    'env':[] if probe else copy.deepcopy(env),'volumeMounts':[{'name':'evidence','mountPath':'/evidence','readOnly':verify}],
                    'securityContext':{'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,'capabilities':{'drop':['ALL']}}}]}}}}
    return {'pvc':pvc,'bind':job('bind'),'upgrade':job('upgrade'),'verify':job('verify')}


def _live_context(session):
    """Revalidate protected live authority and derive all execution inputs."""
    return _live_context_core(session,False)


def _live_running_context(session):
    """Use the fixed postgres-only authority while the native Job may run."""
    return _live_context_core(session,True)


def _live_context_core(session,running):
    row=session.check()
    if row.get('phase')!='prepare' or not row.get('target_sha256'):raise Blocked('space_live_phase_invalid')
    authority=(space.load_native_authority_during_job(session) if running else space.load_native_authority(session))
    qualified=owner.load_qualified(session)
    archive=owner._read(owner._base(session)/'archive-record.json')
    if owner.digest(archive)!=qualified.get('archive_sha256'):raise Blocked('space_live_source_changed')
    owner.revalidate_paused(session);kube=Kube()
    namespace,engine=realtime_bootstrap.engine_snapshot(kube)
    original=authority['original'];pod=kube.get('pod','voice-postgres-0')
    pvc=kube.get('persistentvolumeclaim','pgdata-voice-postgres-0')
    config=kube.get('configmap','voice-app-config');secret=kube.get('secret','voice-app-secrets')
    node=pod.get('spec',{}).get('nodeName');storage_class=pvc.get('spec',{}).get('storageClassName')
    if not isinstance(storage_class,str) or not storage_class:raise Blocked('space_live_source_changed')
    storage=kube.get('storageclass',storage_class);storage_meta=storage.get('metadata',{})
    if (namespace!=original['engine']['namespace_uid'] or engine['pod']['uid']!=original['engine']['pod_uid']
        or engine['pvc']['uid']!=original['engine']['pvc_uid'] or engine['pv']['uid']!=original['engine']['pv_uid']
        or pvc.get('metadata',{}).get('uid')!=original['engine']['pvc_uid']
        or storage_meta.get('name')!=storage_class or not storage_meta.get('uid') or not storage_meta.get('resourceVersion')
        or storage_meta.get('deletionTimestamp') or not isinstance(storage.get('provisioner'),str) or not storage['provisioner']
        or storage.get('volumeBindingMode','Immediate') not in ('Immediate','WaitForFirstConsumer')
        or config.get('data',{}).get('POSTGRES_USER')!=original['installer_role']
        or not secret.get('data',{}).get('POSTGRES_PASSWORD')):
        raise Blocked('space_live_source_changed')
    containers=[item for item in pod.get('spec',{}).get('containers',[]) if item.get('name')=='postgres']
    if len(containers)!=1:raise Blocked('space_live_source_changed')
    for name,ref in (('POSTGRES_USER',{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}),
                     ('POSTGRES_PASSWORD',{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}})):
        if [item for item in containers[0].get('env',[]) if item.get('name')==name]!=[{'name':name,'valueFrom':ref}]:
            raise Blocked('space_live_source_changed')
    docs=_live_documents(session,authority['tool'],node,storage_class,archive['binding']['snapshot_id'])
    if realtime_bootstrap.engine_snapshot(kube)!=(namespace,engine):raise Blocked('space_live_source_changed')
    again=(space.load_native_authority_during_job(session) if running else space.load_native_authority(session))
    if (again!=authority or owner.load_qualified(session)!=qualified
        or owner._read(owner._base(session)/'archive-record.json')!=archive):raise Blocked('space_live_source_changed')
    session.check()
    return {'schema':'voice-staging-space-live-plan-v1','operation':session.operation,'binding':session.binding(),
        'authority_sha256':owner.digest(authority),'archive_sha256':owner.digest(archive),
        'namespace_uid':namespace,'node':node,'storage_class_uid':storage_meta['uid'],
        'storage_class_sha256':owner.digest(storage),'storage_provisioner':storage['provisioner'],
        'postgres_pvc_uid':engine['pvc']['uid'],'postgres_pv_uid':engine['pv']['uid'],
        'tool_platform_digest':authority['tool']['platform_digest'],'tool_config_digest':authority['tool']['config_digest'],
        'config_sha256':owner.digest(config),'secret_sha256':owner.digest(secret),'documents':docs}


def _live_base(session):
    return owner._base(session)/'space-live'


def prepare_live(session):
    """Persist the source-derived private plan; no Kubernetes effect occurs."""
    plan=_live_context(session);base=_live_base(session)
    owner.private._private_directory(base)
    owner._write(base/'plan.json',plan)
    if _live_context(session)!=plan:raise Blocked('space_live_source_changed')
    return copy.deepcopy(plan)


def _live_checked_plan(session):
    plan=owner._read(_live_base(session)/'plan.json')
    if _live_context(session)!=plan:raise Blocked('space_live_source_changed')
    return plan


def _live_checked_running_plan(session):
    plan=owner._read(_live_base(session)/'plan.json')
    if _live_running_context(session)!=plan:raise Blocked('space_live_source_changed')
    return plan


def _check_live_job_spec(spec,desired):
    allowed={'backoffLimit','activeDeadlineSeconds','template','selector','manualSelector','parallelism',
             'completions','completionMode','suspend','podReplacementPolicy'}
    if (not isinstance(spec,dict) or set(spec)-allowed or spec.get('backoffLimit')!=0
        or spec.get('activeDeadlineSeconds')!=360 or spec.get('parallelism',1)!=1 or spec.get('completions',1)!=1
        or spec.get('manualSelector',False) is not False or spec.get('suspend',False) is not False
        or spec.get('completionMode','NonIndexed')!='NonIndexed'
        or spec.get('podReplacementPolicy','TerminatingOrFailed')!='TerminatingOrFailed'):
        raise Blocked('space_live_admission_changed')
    _check_template(spec.get('template',{}).get('spec',{}),desired['spec']['template']['spec'])


def _live_volume(session,plan):
    base=_live_base(session);created=owner._read(base/'effects'/'pvc-create.json')
    name=plan['documents']['pvc']['metadata']['name'];kube=Kube();pvc=kube.get('persistentvolumeclaim',name)
    meta=pvc.get('metadata',{});spec=pvc.get('spec',{});volume_name=spec.get('volumeName')
    if (created.get('status')!='DONE' or meta.get('uid')!=created.get('response',{}).get('uid')
        or meta.get('name')!=name or meta.get('namespace')!='voice-staging' or meta.get('deletionTimestamp')
        or not meta.get('resourceVersion') or pvc.get('status',{}).get('phase')!='Bound'
        or spec.get('storageClassName')!=plan['documents']['pvc']['spec']['storageClassName']
        or spec.get('accessModes')!=['ReadWriteOnce'] or spec.get('dataSource') or spec.get('dataSourceRef')
        or not isinstance(volume_name,str) or not volume_name):raise Blocked('space_live_volume_changed')
    pv=kube.get('persistentvolume',volume_name);pm=pv.get('metadata',{});ps=pv.get('spec',{});claim=ps.get('claimRef',{})
    import datetime
    try:
        pv_created=datetime.datetime.fromisoformat(pm['creationTimestamp'].replace('Z','+00:00'))
        pvc_created=datetime.datetime.fromisoformat(created['response']['creationTimestamp'].replace('Z','+00:00'))
        if pv_created.tzinfo is None or pvc_created.tzinfo is None or pv_created<pvc_created:raise ValueError()
    except (KeyError,TypeError,ValueError):raise Blocked('space_live_volume_changed') from None
    if (pm.get('name')!=volume_name or not owner.guard.UUID.fullmatch(str(pm.get('uid','')))
        or not pm.get('resourceVersion') or pm.get('deletionTimestamp') or pv.get('status',{}).get('phase')!='Bound'
        or pm['uid']==plan['postgres_pv_uid'] or meta['uid']==plan['postgres_pvc_uid']
        or pm.get('annotations',{}).get('pv.kubernetes.io/provisioned-by')!=plan['storage_provisioner']
        or ps.get('storageClassName')!=spec['storageClassName'] or claim.get('uid')!=meta['uid']
        or claim.get('name')!=name or claim.get('namespace')!='voice-staging'):
        raise Blocked('space_live_volume_changed')
    return {'pvc_uid':meta['uid'],'pvc_rv':meta['resourceVersion'],'pv_name':volume_name,'pv_uid':pm['uid'],'pv_rv':pm['resourceVersion']}


def _live_closed(session,plan,kind):
    base=_live_base(session);closed=owner._read(base/(kind+'-closed.json'))
    created=owner._read(base/'effects'/(kind+'-create.json'));deleted=owner._read(base/'effects'/(kind+'-delete.json'))
    fields={'schema','status','operation','kind','plan_sha256','job_uid','pod_uid','volume'}
    if (type(closed) is not dict or set(closed)!=fields or closed['schema']!='voice-staging-space-live-job-closed-v1'
        or closed['status']!='DONE' or closed['operation']!=session.operation or closed['kind']!=kind
        or closed['plan_sha256']!=owner.digest(plan) or created.get('status')!='DONE' or deleted.get('status')!='DONE'
        or created.get('response',{}).get('uid')!=closed['job_uid'] or deleted.get('response',{}).get('uid')!=closed['job_uid']
        or not owner.guard.UUID.fullmatch(str(closed['pod_uid']))):raise Blocked('space_live_predecessor_changed')
    return closed


def _live_unmounted(plan):
    rows=Kube().run(['get','pods','-o','json']);meta=rows.get('metadata',{})
    if (rows.get('apiVersion')!='v1' or rows.get('kind')!='PodList' or not meta.get('resourceVersion')
        or meta.get('continue') or not isinstance(rows.get('items'),list)):raise Blocked('space_live_inventory_invalid')
    name=plan['documents']['pvc']['metadata']['name']
    for pod in rows['items']:
        if any(volume.get('persistentVolumeClaim',{}).get('claimName')==name for volume in pod.get('spec',{}).get('volumes',[])):
            raise Blocked('space_live_volume_in_use')


def _live_create(session,kind):
    """Create one fixed planned object with durable PENDING and no adoption."""
    if kind not in ('pvc','bind','upgrade','verify'):raise Blocked('space_live_effect_invalid')
    plan=_live_checked_plan(session);base=_live_base(session);folder=base/'effects'
    owner.private._private_directory(folder)
    for path in folder.iterdir():
        if path.suffix!='.json' or owner._read(path).get('status')!='DONE':raise Blocked('space_live_reconciliation_required')
    if kind=='bind' and owner._read(folder/'pvc-create.json').get('status')!='DONE':raise Blocked('space_live_predecessor_missing')
    if kind in ('upgrade','verify'):_live_closed(session,plan,'bind')
    if kind=='verify':_live_closed(session,plan,'upgrade')
    desired=plan['documents'][kind];kube=Kube()
    if kind=='bind':
        pvc=kube.get('persistentvolumeclaim',plan['documents']['pvc']['metadata']['name'])
        created=owner._read(folder/'pvc-create.json')
        if (pvc.get('metadata',{}).get('uid')!=created.get('response',{}).get('uid')
            or pvc.get('metadata',{}).get('deletionTimestamp')):raise Blocked('space_live_volume_changed')
        _live_unmounted(plan)
    if kind in ('upgrade','verify'):
        volume=_live_volume(session,plan);prior=_live_closed(session,plan,'bind')['volume']
        if any(volume[key]!=prior[key] for key in ('pvc_uid','pv_name','pv_uid')):raise Blocked('space_live_volume_changed')
        _live_unmounted(plan)
    dry=kube.run(['create','--dry-run=server','-f','/dev/stdin','-o','json'],body=desired)
    if (dry.get('kind')!=desired['kind'] or dry.get('metadata',{}).get('name')!=desired['metadata']['name']
        or dry.get('metadata',{}).get('namespace')!='voice-staging'):
        raise Blocked('space_live_admission_changed')
    if kind=='pvc':
        spec=dry.get('spec',{})
        if (spec.get('storageClassName')!=desired['spec']['storageClassName'] or spec.get('accessModes')!=['ReadWriteOnce']
            or spec.get('resources',{}).get('requests',{}).get('storage')!='16Mi'
            or spec.get('volumeName') or spec.get('selector') or spec.get('dataSource') or spec.get('dataSourceRef')):
            raise Blocked('space_live_admission_changed')
    else:_check_live_job_spec(dry.get('spec',{}),desired)
    if _live_checked_plan(session)!=plan:raise Blocked('space_live_source_changed')
    argv=owner.PREFIX+['create','-f','/dev/stdin','-o','json'];body=owner.private._bytes(desired)
    intent={'schema':'voice-staging-space-live-effect-v1','operation':session.operation,'effect':kind+'-create',
        'plan_sha256':owner.digest(plan),'binding':session.binding(),'argv':argv,
        'body_sha256':hashlib.sha256(body).hexdigest(),'status':'PENDING'}
    path=folder/(kind+'-create.json');owner._write(path,intent)
    import commands
    raw=commands.capture(argv,body=body,limit=4<<20,timeout=60,
        env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
    session.check();response=owner.private._json(raw);meta=response.get('metadata',{})
    actual_spec=response.get('spec',{});dry_spec=dry.get('spec',{})
    if kind=='pvc':
        spec_ok=all(actual_spec.get(key)==dry_spec.get(key) for key in ('accessModes','storageClassName','resources','volumeMode'))
    else:
        _check_live_job_spec(actual_spec,desired)
        spec_ok=(actual_spec.get('backoffLimit')==dry_spec.get('backoffLimit')
            and actual_spec.get('activeDeadlineSeconds')==dry_spec.get('activeDeadlineSeconds')
            and actual_spec.get('template',{}).get('spec')==dry_spec.get('template',{}).get('spec'))
    if (response.get('apiVersion')!=desired['apiVersion'] or response.get('kind')!=desired['kind']
        or meta.get('name')!=desired['metadata']['name'] or meta.get('namespace')!='voice-staging'
        or meta.get('deletionTimestamp') or any(meta.get('labels',{}).get(key)!=value for key,value in desired['metadata']['labels'].items())
        or not owner.guard.UUID.fullmatch(str(meta.get('uid',''))) or not meta.get('resourceVersion')
        or not isinstance(meta.get('creationTimestamp'),str) or not meta['creationTimestamp']
        or not spec_ok or owner._read(path)!=intent):
        raise Blocked('space_live_create_uncertain')
    owner.save(path,{**intent,'status':'DONE','response':{'uid':meta['uid'],'resourceVersion':meta['resourceVersion'],'creationTimestamp':meta['creationTimestamp'],
        'spec_sha256':owner.digest(response['spec'])}})
    if (_live_checked_running_plan(session) if kind in ('upgrade','verify') else _live_checked_plan(session))!=plan:
        raise Blocked('space_live_source_changed')
    return copy.deepcopy(meta)


def _live_job_observation(session,plan,kind,*,running):
    """Read back only the response-owned Job and its single Tool Pod."""
    base=_live_base(session);record=owner._read(base/'effects'/(kind+'-create.json'))
    expected=plan['documents'][kind];name=expected['metadata']['name'];kube=Kube();job=kube.get('job',name)
    meta=job.get('metadata',{});spec=job.get('spec',{})
    if (record.get('status')!='DONE' or meta.get('uid')!=record.get('response',{}).get('uid')
        or meta.get('name')!=name or meta.get('namespace')!='voice-staging' or meta.get('deletionTimestamp')
        or not meta.get('resourceVersion') or owner.digest(spec)!=record['response']['spec_sha256']):
        raise Blocked('space_live_job_changed')
    listing=kube.run(['get','pods','-l','job-name='+name,'-o','json']);lm=listing.get('metadata',{})
    if (listing.get('apiVersion')!='v1' or listing.get('kind')!='PodList' or not lm.get('resourceVersion')
        or lm.get('continue') or not isinstance(listing.get('items'),list) or len(listing['items'])!=1):
        raise Blocked('space_live_pod_changed')
    pod=listing['items'][0];pm=pod.get('metadata',{});owners=[item for item in pm.get('ownerReferences',[]) if item.get('controller') is True]
    statuses=[item for item in pod.get('status',{}).get('containerStatuses',[]) if item.get('name')=='native']
    template=spec.get('template',{}).get('spec',{});ps=pod.get('spec',{})
    if (pm.get('namespace')!='voice-staging' or not isinstance(pm.get('name'),str)
        or not re.fullmatch(re.escape(name)+r'-[a-z0-9]{1,32}',pm['name'])
        or not owner.guard.UUID.fullmatch(str(pm.get('uid','')))
        or not pm.get('resourceVersion') or pm.get('deletionTimestamp') or len(owners)!=1
        or owners[0].get('kind')!='Job' or owners[0].get('uid')!=meta['uid']
        or ps.get('nodeName')!=plan['node'] or ps.get('affinity')!=template.get('affinity') or ps.get('containers')!=template.get('containers')
        or ps.get('volumes')!=template.get('volumes') or ps.get('automountServiceAccountToken') is not False
        or ps.get('securityContext')!=template.get('securityContext') or ps.get('initContainers') or ps.get('ephemeralContainers')
        or len(statuses)!=1 or len(pod.get('status',{}).get('containerStatuses',[]))!=1
        or not any(statuses[0].get('imageID','').endswith(plan[key]) for key in ('tool_platform_digest','tool_config_digest'))):
        raise Blocked('space_live_pod_changed')
    state=statuses[0].get('state',{})
    if running:
        if 'running' not in state or job.get('status',{}).get('failed',0):raise Blocked('space_live_pod_changed')
    elif (state.get('terminated',{}).get('exitCode')!=0 or job.get('status',{}).get('succeeded')!=1
          or job.get('status',{}).get('active',0)):
        raise Blocked('space_live_job_failed')
    if _live_checked_plan(session)!=plan:raise Blocked('space_live_source_changed')
    return job,pod


def _live_hashes(raw,kind):
    try:lines=raw.decode('ascii').splitlines()
    except (UnicodeError,AttributeError):raise Blocked('space_live_output_invalid') from None
    paths=['/evidence/origin.json','/evidence/completed.jsonl']
    if kind=='verify':
        if len(lines)!=3 or lines.pop()!='verification=ok':raise Blocked('space_live_output_invalid')
    if len(lines)!=len(paths):raise Blocked('space_live_output_invalid')
    result={}
    for line,path in zip(lines,paths):
        match=re.fullmatch(r'([a-f0-9]{64})  '+re.escape(path),line)
        if not match:raise Blocked('space_live_output_invalid')
        result[path]=match.group(1)
    return result


def _live_complete_job(session,plan,kind):
    if kind not in ('upgrade','verify'):raise Blocked('space_live_effect_invalid')
    job,pod=_live_job_observation(session,plan,kind,running=False)
    volume=_live_volume(session,plan);base=_live_base(session)
    import commands
    argv=owner.PREFIX+['logs',pod['metadata']['name'],'-c','native']
    raw=commands.capture(argv,limit=4096,timeout=30,env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
    hashes=_live_hashes(raw,kind);after_job,after_pod=_live_job_observation(session,plan,kind,running=False)
    if (after_job['metadata']['uid']!=job['metadata']['uid'] or after_pod['metadata']['uid']!=pod['metadata']['uid']
        or after_pod['spec']!=pod['spec'] or after_pod['status']['containerStatuses'][0]['imageID']!=pod['status']['containerStatuses'][0]['imageID']
        or _live_volume(session,plan)!=volume):raise Blocked('space_live_job_changed')
    if kind=='verify':
        original=owner._read(base/'upgrade-result.json')['hashes']
        if any(hashes[path]!=original[path] for path in ('/evidence/origin.json','/evidence/completed.jsonl')):
            raise Blocked('space_live_evidence_changed')
    result={'schema':'voice-staging-space-live-job-result-v1','operation':session.operation,'kind':kind,
        'plan_sha256':owner.digest(plan),'job_uid':job['metadata']['uid'],'pod_uid':pod['metadata']['uid'],
        'pod_before_rv':pod['metadata']['resourceVersion'],'pod_after_rv':after_pod['metadata']['resourceVersion'],
        'volume':volume,'hashes':hashes}
    owner._write(base/(kind+'-result.json'),result)
    return result


def _live_delete_job(session,plan,kind):
    if kind not in ('bind','upgrade','verify'):raise Blocked('space_live_effect_invalid')
    base=_live_base(session);job,pod=_live_job_observation(session,plan,kind,running=kind=='bind')
    volume=_live_volume(session,plan);meta=job['metadata'];name=meta['name'];uid=meta['uid']
    result=owner._read(base/(kind+'-result.json'))
    if result.get('job_uid')!=uid or result.get('pod_uid')!=pod['metadata']['uid'] or result.get('plan_sha256')!=owner.digest(plan):
        raise Blocked('space_live_job_changed')
    options={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
        'preconditions':{'uid':uid,'resourceVersion':meta['resourceVersion']}}
    argv=owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/'+name,'-f','/dev/stdin']
    body=owner.private._bytes(options);path=base/'effects'/(kind+'-delete.json')
    intent={'schema':'voice-staging-space-live-effect-v1','operation':session.operation,'effect':kind+'-delete',
        'plan_sha256':owner.digest(plan),'binding':session.binding(),'argv':argv,'body_sha256':hashlib.sha256(body).hexdigest(),'status':'PENDING'}
    if _live_checked_plan(session)!=plan:raise Blocked('space_live_source_changed')
    owner._write(path,intent)
    import commands
    raw=commands.capture(argv,body=body,limit=4<<20,timeout=60,env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
    session.check();_check_delete_response(owner.private._json(raw),uid)
    if owner._read(path)!=intent or _live_checked_plan(session)!=plan:raise Blocked('space_live_delete_uncertain')
    owner.save(path,{**intent,'status':'DONE','response':{'uid':uid,'resourceVersion':meta['resourceVersion']}})
    kube=Kube();deadline=time.monotonic()+60
    while True:
        path_arg='/apis/batch/v1/namespaces/voice-staging/jobs?fieldSelector=metadata.name%3D'+name+'&limit=2'
        jobs=kube.run(['get','--raw',path_arg]);pods=kube.run(['get','pods','-l','job-name='+name,'-o','json'])
        for value,api,expected_kind in ((jobs,'batch/v1','JobList'),(pods,'v1','PodList')):
            lm=value.get('metadata',{})
            if (value.get('apiVersion')!=api or value.get('kind')!=expected_kind or not lm.get('resourceVersion')
                or lm.get('continue') or not isinstance(value.get('items'),list)):raise Blocked('space_live_absence_invalid')
        if jobs['items'] and (len(jobs['items'])!=1 or jobs['items'][0].get('metadata',{}).get('uid')!=uid):
            raise Blocked('space_live_job_changed')
        if not jobs['items'] and not pods['items']:break
        if time.monotonic()>=deadline:raise Blocked('space_live_cleanup_timeout')
        time.sleep(1)
    current=_live_volume(session,plan)
    if any(current[key]!=volume[key] for key in ('pvc_uid','pv_uid','pv_name')) or _live_checked_plan(session)!=plan:
        raise Blocked('space_live_volume_changed')
    closed={'schema':'voice-staging-space-live-job-closed-v1','status':'DONE','operation':session.operation,
        'kind':kind,'plan_sha256':owner.digest(plan),'job_uid':uid,'pod_uid':pod['metadata']['uid'],'volume':current}
    owner._write(base/(kind+'-closed.json'),closed)
    return closed


def _live_wait_job(session,plan,kind):
    name=plan['documents'][kind]['metadata']['name'];record=owner._read(_live_base(session)/'effects'/(kind+'-create.json'))
    deadline=time.monotonic()+370;kube=Kube()
    while True:
        job=kube.get('job',name);meta=job.get('metadata',{});status=job.get('status',{})
        if (meta.get('uid')!=record.get('response',{}).get('uid') or meta.get('deletionTimestamp')
            or owner.digest(job.get('spec',{}))!=record['response']['spec_sha256'] or status.get('failed',0)):
            raise Blocked('space_live_job_changed')
        listing=kube.run(['get','pods','-l','job-name='+name,'-o','json']);lm=listing.get('metadata',{})
        if (listing.get('apiVersion')!='v1' or listing.get('kind')!='PodList' or not lm.get('resourceVersion')
            or lm.get('continue') or not isinstance(listing.get('items'),list) or len(listing['items'])>1):
            raise Blocked('space_live_pod_changed')
        if listing['items']:
            phase=listing['items'][0].get('status',{}).get('phase')
            if phase=='Failed':raise Blocked('space_live_job_failed')
            if kind=='bind' and phase=='Running' or kind!='bind' and status.get('succeeded')==1:
                return _live_job_observation(session,plan,kind,running=kind=='bind')
        if (_live_checked_running_plan(session) if kind in ('upgrade','verify') else _live_checked_plan(session))!=plan:
            raise Blocked('space_live_source_changed')
        if time.monotonic()>=deadline:raise Blocked('space_live_job_timeout')
        time.sleep(1)


def execute_live(session):
    """Run the fixed source-qualified native upgrade once; retain its PVC."""
    plan=prepare_live(session);base=_live_base(session)
    _live_create(session,'pvc')
    _live_create(session,'bind');bind_job,bind_pod=_live_wait_job(session,plan,'bind');volume=_live_volume(session,plan)
    owner._write(base/'bind-result.json',{'schema':'voice-staging-space-live-bind-result-v1','operation':session.operation,
        'plan_sha256':owner.digest(plan),'job_uid':bind_job['metadata']['uid'],'pod_uid':bind_pod['metadata']['uid'],'volume':volume})
    _live_delete_job(session,plan,'bind')
    _live_create(session,'upgrade');_live_wait_job(session,plan,'upgrade')
    upgrade=_live_complete_job(session,plan,'upgrade');_live_delete_job(session,plan,'upgrade')
    _live_create(session,'verify');_live_wait_job(session,plan,'verify')
    verification=_live_complete_job(session,plan,'verify');closed=_live_delete_job(session,plan,'verify')
    if _live_checked_plan(session)!=plan:raise Blocked('space_live_source_changed')
    receipt={'schema':'voice-staging-space-live-complete-v1','status':'COMPLETE','operation':session.operation,
        'plan_sha256':owner.digest(plan),'authority_sha256':plan['authority_sha256'],'volume':closed['volume'],
        'upgrade_result_sha256':owner.digest(upgrade),'verify_result_sha256':owner.digest(verification),
        'upgrade_closed_sha256':owner.digest(owner._read(base/'upgrade-closed.json')),
        'verify_closed_sha256':owner.digest(closed),'evidence_hashes':copy.deepcopy(verification['hashes'])}
    owner._write(base/'complete.json',receipt)
    return load_live_completed(session)


def load_live_completed(session):
    """Require the protected completed jobs and still-owned retained volume."""
    plan=_live_checked_plan(session);base=_live_base(session);receipt=owner._read(base/'complete.json')
    effects=base/'effects'
    expected={'pvc-create.json','bind-create.json','bind-delete.json','upgrade-create.json',
        'upgrade-delete.json','verify-create.json','verify-delete.json'}
    if ({path.name for path in effects.iterdir()}!=expected
        or any(owner._read(effects/name).get('status')!='DONE' for name in expected)):
        raise Blocked('space_live_reconciliation_required')
    fields={'schema','status','operation','plan_sha256','authority_sha256','volume','upgrade_result_sha256',
        'verify_result_sha256','upgrade_closed_sha256','verify_closed_sha256','evidence_hashes'}
    upgrade=owner._read(base/'upgrade-result.json');verification=owner._read(base/'verify-result.json')
    bind=_live_closed(session,plan,'bind');upgrade_closed=_live_closed(session,plan,'upgrade');verify_closed=_live_closed(session,plan,'verify')
    if (type(receipt) is not dict or set(receipt)!=fields or receipt['schema']!='voice-staging-space-live-complete-v1'
        or receipt['status']!='COMPLETE' or receipt['operation']!=session.operation or receipt['plan_sha256']!=owner.digest(plan)
        or receipt['authority_sha256']!=plan['authority_sha256'] or receipt['upgrade_result_sha256']!=owner.digest(upgrade)
        or receipt['verify_result_sha256']!=owner.digest(verification) or receipt['upgrade_closed_sha256']!=owner.digest(upgrade_closed)
        or receipt['verify_closed_sha256']!=owner.digest(verify_closed) or receipt['volume']!=verify_closed['volume']
        or receipt['evidence_hashes']!=verification['hashes']
        or any(upgrade['hashes'].get(path)!=verification['hashes'].get(path) for path in ('/evidence/origin.json','/evidence/completed.jsonl'))
        or upgrade.get('job_uid')!=upgrade_closed['job_uid'] or verification.get('job_uid')!=verify_closed['job_uid']):
        raise Blocked('space_live_receipt_changed')
    current=_live_volume(session,plan)
    for evidence in (bind['volume'],upgrade_closed['volume'],verify_closed['volume']):
        if any(current[key]!=evidence[key] for key in ('pvc_uid','pv_name','pv_uid')):raise Blocked('space_live_volume_changed')
    _live_unmounted(plan)
    if _live_checked_plan(session)!=plan:raise Blocked('space_live_source_changed')
    return copy.deepcopy(receipt)


def validate_historical_live_completed(operation):
    """Validate the original protected PASS and native effects without a paused read."""
    import resource_claim
    if not isinstance(operation,str) or not re.fullmatch('[a-f0-9]{12}',operation):
        raise Blocked('space_live_history_invalid')
    stack=_HISTORY_STACK.get()
    if operation in stack or len(stack)>=32:raise Blocked('space_live_history_cycle')
    token=_HISTORY_STACK.set((*stack,operation))
    try:
        qualified=owner.validate_historical_capture(operation)
        rollout=owner._read(owner.guard.ROOT/('rollout-'+operation)/'checkpoint.json',limit=128<<20)
        binding=resource_claim.historical_binding(rollout)
        native_receipt=rollout.get('native_space_migration')
        if type(native_receipt) is dict and native_receipt.get('schema')=='voice-staging-space-native-successor-v1':
            return _validate_historical_native_successor(operation,qualified,rollout,binding)
        return _validate_historical_live_completed(operation,qualified,rollout,binding)
    finally:_HISTORY_STACK.reset(token)


def _validate_historical_live_completed(operation,qualified,rollout,binding):
    """Internal record validation after the fixed historical authority checks."""
    from types import SimpleNamespace
    root=owner.guard.ROOT/'installed'/'cohort-preservation'/operation
    base=root/'space-live';plan=owner._read(base/'plan.json');receipt=owner._read(base/'complete.json')
    archive=owner._read(root/'archive-record.json');catalog=owner._read(root/'catalog.json');metadata=owner._read(root/'metadata.json')
    tool_record=owner._read(root/'space-tool.json');tool=tool_record['authenticated_publication']['publication']
    offnode=owner._read(root/'offnode-restore.json');rehearsal=offnode['proof']['native_space']
    role=metadata['source']['catalog']['space_db']['database_owner']
    original={'engine':catalog['engine'],'server_start':catalog['server_start'],'database_name':'space_db',
        'database_oid':catalog['databases']['space_db'],'database_owner':role,'schema_owner':role,'installer_role':role}
    authority={'schema':'voice-staging-space-native-authority-v1','operation':operation,
        'preservation_sha256':owner.digest(qualified),'tool':tool,'rehearsal':rehearsal,'original':original}
    data_session=SimpleNamespace(operation=operation,context=binding['context'])
    docs=_live_documents(data_session,tool,plan['node'],plan['documents']['pvc']['spec']['storageClassName'],archive['binding']['snapshot_id'])
    if (plan.get('schema')!='voice-staging-space-live-plan-v1' or plan.get('operation')!=operation
        or plan.get('binding')!=binding or plan.get('authority_sha256')!=owner.digest(authority)
        or plan.get('archive_sha256')!=qualified['archive_sha256'] or plan.get('namespace_uid')!=qualified['namespace_uid']
        or plan.get('documents')!=docs or plan.get('tool_platform_digest')!=tool['platform_digest']
        or plan.get('tool_config_digest')!=tool['config_digest'] or tool.get('source_sha')!=binding['context']['commit']
        or owner.digest(catalog)!=archive['catalog_ref']['record_sha256']
        or owner.digest(metadata)!=archive['metadata_ref']['record_sha256']
        or metadata.get('catalog_sha256')!=owner.digest(catalog)
        or metadata['source']['catalog']['space_db']['schema_owner']!=role
        or catalog['properties']['space_db']['owner']!=role
        or offnode['proof'].get('space_native_rehearsal_sha256')!=owner.digest(rehearsal)):
        raise Blocked('space_live_history_changed')
    effects=base/'effects';names={'pvc-create.json','bind-create.json','bind-delete.json','upgrade-create.json',
        'upgrade-delete.json','verify-create.json','verify-delete.json'}
    if {path.name for path in effects.iterdir()}!=names:raise Blocked('space_live_history_changed')
    for name in names:
        effect=owner._read(effects/name);kind,verb=name[:-5].split('-')
        fields={'schema','operation','effect','plan_sha256','binding','argv','body_sha256','status','response'}
        if (set(effect)!=fields or effect.get('schema')!='voice-staging-space-live-effect-v1'
            or effect.get('operation')!=operation or effect.get('effect')!=kind+'-'+verb
            or effect.get('plan_sha256')!=owner.digest(plan) or effect.get('binding')!=binding
            or effect.get('status')!='DONE' or not owner.guard.UUID.fullmatch(str(effect.get('response',{}).get('uid','')))):
            raise Blocked('space_live_history_changed')
        if verb=='create':
            if (effect['argv']!=owner.PREFIX+['create','-f','/dev/stdin','-o','json']
                or effect['body_sha256']!=hashlib.sha256(owner.private._bytes(docs[kind])).hexdigest()
                or set(effect['response'])!={'uid','resourceVersion','creationTimestamp','spec_sha256'}
                or not effect['response']['resourceVersion'] or not effect['response']['creationTimestamp']
                or not re.fullmatch('[a-f0-9]{64}',effect['response']['spec_sha256'])):
                raise Blocked('space_live_history_changed')
        else:
            response=effect['response'];name=docs[kind]['metadata']['name']
            if set(response)!={'uid','resourceVersion'} or not response['resourceVersion']:
                raise Blocked('space_live_history_changed')
            options={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
                'preconditions':{'uid':response['uid'],'resourceVersion':response['resourceVersion']}}
            if (effect['argv']!=owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/'+name,'-f','/dev/stdin']
                or effect['body_sha256']!=hashlib.sha256(owner.private._bytes(options)).hexdigest()):
                raise Blocked('space_live_history_changed')
    closed={kind:_live_closed(data_session,plan,kind) for kind in ('bind','upgrade','verify')}
    bind=owner._read(base/'bind-result.json')
    if (set(bind)!={'schema','operation','plan_sha256','job_uid','pod_uid','volume'}
        or bind.get('schema')!='voice-staging-space-live-bind-result-v1' or bind.get('operation')!=operation
        or bind.get('plan_sha256')!=owner.digest(plan) or bind.get('job_uid')!=closed['bind']['job_uid']
        or bind.get('pod_uid')!=closed['bind']['pod_uid']):raise Blocked('space_live_history_changed')
    upgrade=owner._read(base/'upgrade-result.json');verify=owner._read(base/'verify-result.json')
    for kind,result in (('upgrade',upgrade),('verify',verify)):
        if (set(result)!={'schema','operation','kind','plan_sha256','job_uid','pod_uid','pod_before_rv','pod_after_rv','volume','hashes'}
            or result.get('schema')!='voice-staging-space-live-job-result-v1' or result.get('operation')!=operation
            or result.get('kind')!=kind or result.get('plan_sha256')!=owner.digest(plan)
            or not result.get('pod_before_rv') or not result.get('pod_after_rv')
            or set(result.get('hashes',{}))!={'/evidence/origin.json','/evidence/completed.jsonl'}
            or any(not isinstance(value,str) or not re.fullmatch('[a-f0-9]{64}',value) for value in result['hashes'].values())):
            raise Blocked('space_live_history_changed')
    expected={'schema':'voice-staging-space-live-complete-v1','status':'COMPLETE','operation':operation,
        'plan_sha256':owner.digest(plan),'authority_sha256':plan['authority_sha256'],'volume':closed['verify']['volume'],
        'upgrade_result_sha256':owner.digest(upgrade),'verify_result_sha256':owner.digest(verify),
        'upgrade_closed_sha256':owner.digest(closed['upgrade']),'verify_closed_sha256':owner.digest(closed['verify']),
        'evidence_hashes':copy.deepcopy(verify['hashes'])}
    if (receipt!=expected or rollout.get('native_space_migration')!=receipt
        or [event for event in rollout.get('events',[]) if event.get('kind')=='native_space_migration_complete']
            !=[{'kind':'native_space_migration_complete','receipt_sha256':owner.digest(receipt)}]
        or upgrade.get('job_uid')!=closed['upgrade']['job_uid'] or upgrade.get('pod_uid')!=closed['upgrade']['pod_uid']
        or verify.get('job_uid')!=closed['verify']['job_uid'] or verify.get('pod_uid')!=closed['verify']['pod_uid']
        or upgrade.get('hashes')!=verify.get('hashes')
        or receipt['volume'].get('pvc_uid')!=owner._read(effects/'pvc-create.json')['response']['uid']
        or receipt['volume'].get('pvc_uid')==plan['postgres_pvc_uid']
        or receipt['volume'].get('pv_uid')==plan['postgres_pv_uid']
        or any(any(result['volume'][key]!=receipt['volume'][key] for key in ('pvc_uid','pv_uid','pv_name'))
               for result in (bind,upgrade,verify))
        or any(any(closed[kind]['volume'][key]!=receipt['volume'][key] for key in ('pvc_uid','pv_uid','pv_name'))
               for kind in ('bind','upgrade','verify'))):raise Blocked('space_live_history_changed')
    return copy.deepcopy(receipt)


def historical_native_predecessor(session):
    """Derive the only possible native successor from protected active history."""
    plan=owner._guard(session);owner._no_pending(session)
    _,_,prepared=owner._source(session,plan)
    expected=prepared['plan'].get('previous_active_sha256')
    active_path=owner.guard.ROOT/'installed'/'active-release.json'
    if expected is None:
        # No protected native predecessor is admitted in this branch. The
        # actual legacy24 inspector still decides whether fresh migration is
        # eligible; an unrelated older active release is not such a proof.
        return None
    active=owner._read(active_path)
    if owner.digest(active)!=expected:raise Blocked('space_native_predecessor_changed')
    operation=active.get('operation')
    if (not isinstance(operation,str) or not re.fullmatch('[a-f0-9]{12}',operation)
        or operation==session.operation or active.get('namespace_uid')!=plan['namespace_uid']):
        raise Blocked('space_native_predecessor_changed')
    rollout=owner._read(owner.guard.ROOT/('rollout-'+operation)/'checkpoint.json',limit=128<<20)
    if (rollout.get('operation')!=operation or rollout.get('status')!='PASS' or rollout.get('phase')!='POST_RESTART'
        or rollout.get('target',{}).get('manifest_sha256')!=active.get('target_sha256')):
        raise Blocked('space_native_predecessor_changed')
    previous=owner.guard.ROOT/'installed'/'cohort-preservation'/operation/'space-live'
    if 'native_space_migration' not in rollout:
        if previous.exists() or previous.is_symlink():raise Blocked('space_native_predecessor_changed')
        return None
    receipt=validate_historical_live_completed(operation)
    if receipt!=rollout['native_space_migration']:raise Blocked('space_native_predecessor_changed')
    if owner._guard(session)!=plan or owner._source(session,plan)[2]!=prepared or owner._read(active_path)!=active:
        raise Blocked('space_native_predecessor_changed')
    return {'schema':'voice-staging-space-native-predecessor-v1','operation':session.operation,
        'plan_sha256':owner.digest(plan),'previous_operation':operation,'active_sha256':owner.digest(active),
        'native_sha256':owner.digest(receipt)}


def _successor_job(operation,oldplan,node):
    if not isinstance(operation,str) or not re.fullmatch('[a-f0-9]{12}',operation):raise Blocked('space_native_successor_changed')
    if not isinstance(node,str) or not re.fullmatch('[a-z0-9][a-z0-9.-]{0,252}',node):raise Blocked('space_native_successor_node_changed')
    wanted=copy.deepcopy(oldplan['documents']['verify']);name='voice-space-native-verify-'+operation
    labels={'voice-space-native-successor':operation}
    wanted['metadata'].update(name=name,labels=labels);template=wanted['spec']['template']
    template['metadata']['labels']=labels
    template['spec']['affinity']['nodeAffinity']['requiredDuringSchedulingIgnoredDuringExecution']['nodeSelectorTerms'][0]['matchFields'][0]['values']=[node]
    return wanted


def _successor_context(session):
    """Root-derived C2 input, including the original Tool and retained PVC."""
    predecessor=historical_native_predecessor(session)
    if predecessor is None:raise Blocked('space_native_successor_missing')
    old=predecessor['previous_operation'];root=owner.guard.ROOT/'installed'/'cohort-preservation'/old
    prior=validate_historical_live_completed(old);oldplan=owner._read(root/'space-live'/'plan.json')
    oldcatalog=owner._read(root/'catalog.json');oldmetadata=owner._read(root/'metadata.json')
    kube=Kube();identity=_identity(session,kube);database=identity['database']
    role=oldmetadata['source']['catalog']['space_db']['database_owner']
    if (identity['namespace_uid']!=oldcatalog['engine']['namespace_uid']
        or database['system_identifier']!=oldcatalog['engine']['system_identifier']
        or database['database_oid']!=oldcatalog['databases']['space_db']
        or database['database_owner']!=role or database['schema_owner']!=role or database['session_user']!=role):
        raise Blocked('space_native_successor_database_changed')
    claim=oldplan['documents']['pvc']['metadata']['name'];pvc=kube.get('persistentvolumeclaim',claim)
    volume=prior['volume'];meta=pvc.get('metadata',{});spec=pvc.get('spec',{})
    if (meta.get('name')!=claim or meta.get('namespace')!=owner.guard.NS or meta.get('uid')!=volume['pvc_uid']
        or not meta.get('resourceVersion') or meta.get('deletionTimestamp') or pvc.get('status',{}).get('phase')!='Bound'
        or spec.get('volumeName')!=volume['pv_name'] or spec.get('storageClassName')!=oldplan['documents']['pvc']['spec']['storageClassName']
        or spec.get('accessModes')!=['ReadWriteOnce']):raise Blocked('space_native_successor_volume_changed')
    pv=kube.get('persistentvolume',volume['pv_name']);pm=pv.get('metadata',{});ps=pv.get('spec',{});ref=ps.get('claimRef',{})
    if (pm.get('uid')!=volume['pv_uid'] or not pm.get('resourceVersion') or pm.get('deletionTimestamp')
        or pv.get('status',{}).get('phase')!='Bound' or ref.get('uid')!=volume['pvc_uid']
        or ref.get('name')!=claim or ref.get('namespace')!=owner.guard.NS
        or ps.get('storageClassName')!=spec['storageClassName']):raise Blocked('space_native_successor_volume_changed')
    config=kube.get('configmap','voice-app-config');secret=kube.get('secret','voice-app-secrets')
    if config.get('data',{}).get('POSTGRES_USER')!=role or not secret.get('data',{}).get('POSTGRES_PASSWORD'):
        raise Blocked('space_native_successor_credentials_changed')
    node=kube.get('pod','voice-postgres-0').get('spec',{}).get('nodeName')
    if not isinstance(node,str) or not re.fullmatch('[a-z0-9][a-z0-9.-]{0,252}',node):raise Blocked('space_native_successor_node_changed')
    wanted=_successor_job(session.operation,oldplan,node)
    if (historical_native_predecessor(session)!=predecessor or _identity(session,kube)!=identity
        or kube.get('configmap','voice-app-config')!=config or kube.get('secret','voice-app-secrets')!=secret):
        raise Blocked('space_native_successor_changed')
    return {'schema':'voice-staging-space-native-successor-plan-v1','operation':session.operation,
        'predecessor':predecessor,'prior_native_sha256':owner.digest(prior),'identity':identity,
        'pvc_uid':volume['pvc_uid'],'pv_uid':volume['pv_uid'],'pv_name':volume['pv_name'],
        'pvc_sha256':owner.digest(pvc),'pv_sha256':owner.digest(pv),
        'config_sha256':owner.digest(config),'secret_sha256':owner.digest(secret),
        'expected_hashes':prior['evidence_hashes'],'tool_platform_digest':oldplan['tool_platform_digest'],
        'tool_config_digest':oldplan['tool_config_digest'],'node':node,'job':wanted}


def _successor_no_sql(session,plan):
    """Recheck physical custody without opening Space during Tool execution."""
    owner._guard(session);kube=Kube();namespace,engine=realtime_bootstrap.engine_snapshot(kube)
    if namespace!=plan['identity']['namespace_uid'] or engine!=plan['identity']['engine']:
        raise Blocked('space_native_successor_database_changed')
    claim=plan['job']['spec']['template']['spec']['volumes'][0]['persistentVolumeClaim']['claimName']
    pvc=kube.get('persistentvolumeclaim',claim);pv=kube.get('persistentvolume',plan['pv_name'])
    if (pvc.get('metadata',{}).get('uid')!=plan['pvc_uid'] or pvc.get('metadata',{}).get('deletionTimestamp')
        or pvc.get('status',{}).get('phase')!='Bound' or pvc.get('spec',{}).get('volumeName')!=plan['pv_name']
        or pv.get('metadata',{}).get('uid')!=plan['pv_uid'] or pv.get('metadata',{}).get('deletionTimestamp')
        or pv.get('status',{}).get('phase')!='Bound' or pv.get('spec',{}).get('claimRef',{}).get('uid')!=plan['pvc_uid']
        or owner.digest(pvc)!=plan['pvc_sha256'] or owner.digest(pv)!=plan['pv_sha256']):
        raise Blocked('space_native_successor_volume_changed')
    if (owner.digest(kube.get('configmap','voice-app-config'))!=plan['config_sha256']
        or owner.digest(kube.get('secret','voice-app-secrets'))!=plan['secret_sha256']):
        raise Blocked('space_native_successor_credentials_changed')
    session.check()


def _successor_effect(session,plan,kind,uid=None,resource_version=None):
    """One fixed C2 Job create/delete; unknown replies are never adopted."""
    if kind not in ('create','delete'):raise Blocked('space_native_successor_effect_invalid')
    owner._guard(session);owner._no_pending(session);_successor_no_sql(session,plan)
    base=owner._base(session);stored=owner._read(base/'space-native-successor-plan.json')
    if stored!=plan:raise Blocked('space_native_successor_changed')
    ident='space-native-successor-'+kind;name=plan['job']['metadata']['name']
    if kind=='create':
        argv=owner.PREFIX+['create','-f','/dev/stdin','-o','json'];body=owner.private._bytes(plan['job'])
    else:
        created=owner._read(base/'effects'/'space-native-successor-create.json')
        if (created.get('status')!='DONE' or created.get('response',{}).get('uid')!=uid
            or not owner.guard.UUID.fullmatch(str(uid)) or not isinstance(resource_version,str) or not resource_version):
            raise Blocked('space_native_successor_effect_invalid')
        argv=owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/'+name,'-f','/dev/stdin']
        body=owner.private._bytes({'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
            'preconditions':{'uid':uid,'resourceVersion':resource_version}})
    folder=base/'effects';owner.private._private_directory(folder)
    intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':ident,
        'binding':session.binding(),'argv':argv,'body_sha256':hashlib.sha256(body).hexdigest(),'status':'PENDING'}
    path=folder/(ident+'.json');owner._write(path,intent)
    raw=owner._send(session,ident,argv,body,timeout=60)
    if owner._read(path)!=intent:raise Blocked('space_native_successor_effect_changed')
    return path,intent,owner.private._json(raw)


def _successor_pods(session,name):
    raw=owner._observation(session,'space-native-successor-pods',owner.PREFIX+['get','pods','-l','job-name='+name,'-o','json'])
    row=owner.private._json(raw);meta=row.get('metadata',{})
    if (row.get('apiVersion')!='v1' or row.get('kind')!='PodList' or not meta.get('resourceVersion')
        or meta.get('continue') or not isinstance(row.get('items'),list)):raise Blocked('space_native_successor_inventory_invalid')
    return row['items']


def _successor_unmounted(plan):
    claim=plan['job']['spec']['template']['spec']['volumes'][0]['persistentVolumeClaim']['claimName']
    _live_unmounted({'documents':{'pvc':{'metadata':{'name':claim}}}})


def verify_native_successor(session):
    """Verify C1 evidence on the original Tool and retained PVC, without DDL."""
    owner._guard(session);owner._no_pending(session)
    plan=_successor_context(session);base=owner._base(session);name=plan['job']['metadata']['name'];kube=Kube()
    _successor_unmounted(plan);owner._write(base/'space-native-successor-plan.json',plan)
    dry=kube.run(['create','--dry-run=server','-f','/dev/stdin','-o','json'],body=plan['job'])
    if (dry.get('kind')!='Job' or dry.get('metadata',{}).get('name')!=name
        or dry.get('metadata',{}).get('namespace')!=owner.guard.NS):raise Blocked('space_native_successor_job_invalid')
    _check_live_job_spec(dry.get('spec',{}),plan['job'])
    if _successor_context(session)!=plan:raise Blocked('space_native_successor_changed')
    path,intent,created=_successor_effect(session,plan,'create');meta=created.get('metadata',{});uid=meta.get('uid')
    if (created.get('apiVersion')!='batch/v1' or created.get('kind')!='Job' or meta.get('name')!=name or meta.get('namespace')!=owner.guard.NS
        or any(meta.get('labels',{}).get(key)!=value for key,value in plan['job']['metadata']['labels'].items())
        or not owner.guard.UUID.fullmatch(str(uid)) or not meta.get('resourceVersion')
        or created.get('spec',{}).get('template',{}).get('spec',{})!=dry.get('spec',{}).get('template',{}).get('spec',{})):
        raise Blocked('space_native_successor_create_invalid')
    _check_live_job_spec(created.get('spec',{}),plan['job'])
    _done(path,intent,{'uid':uid,'resourceVersion':meta['resourceVersion'],'spec_sha256':owner.digest(created['spec'])})
    deadline=time.monotonic()+310
    while True:
        _successor_no_sql(session,plan);job=kube.get('job',name);jm=job.get('metadata',{})
        if (jm.get('uid')!=uid or jm.get('deletionTimestamp') or owner.digest(job.get('spec',{}))!=owner.digest(created['spec'])
            or job.get('status',{}).get('failed',0)):raise Blocked('space_native_successor_job_changed')
        if job.get('status',{}).get('succeeded')==1 and not job.get('status',{}).get('active',0):break
        if time.monotonic()>=deadline:raise Blocked('space_native_successor_job_timeout')
        time.sleep(1)
    pods=_successor_pods(session,name)
    if len(pods)!=1:raise Blocked('space_native_successor_pod_changed')
    pod=pods[0];pm=pod.get('metadata',{});ps=pod.get('spec',{});owners=[x for x in pm.get('ownerReferences',[]) if x.get('controller') is True]
    statuses=[x for x in pod.get('status',{}).get('containerStatuses',[]) if x.get('name')=='native']
    if (len(owners)!=1 or owners[0].get('kind')!='Job' or owners[0].get('uid')!=uid
        or pm.get('namespace')!=owner.guard.NS or not owner.guard.UUID.fullmatch(str(pm.get('uid','')))
        or not isinstance(pm.get('name'),str) or not re.fullmatch(re.escape(name)+r'-[a-z0-9]{1,32}',pm['name'])
        or not pm.get('resourceVersion') or pm.get('deletionTimestamp') or ps.get('nodeName')!=plan['node']
        or ps.get('containers')!=created['spec']['template']['spec']['containers']
        or ps.get('volumes')!=created['spec']['template']['spec']['volumes']
        or ps.get('securityContext')!=created['spec']['template']['spec']['securityContext']
        or ps.get('automountServiceAccountToken') is not False or ps.get('initContainers') or ps.get('ephemeralContainers')
        or len(statuses)!=1 or len(pod.get('status',{}).get('containerStatuses',[]))!=1
        or statuses[0].get('state',{}).get('terminated',{}).get('exitCode')!=0
        or not any(statuses[0].get('imageID','').endswith(plan[key]) for key in ('tool_platform_digest','tool_config_digest'))):
        raise Blocked('space_native_successor_pod_changed')
    pod_template=copy.deepcopy(ps);pod_template.pop('nodeName',None)
    _check_template(pod_template,created['spec']['template']['spec'])
    raw=owner._observation(session,'space-native-successor-log',owner.PREFIX+['logs',pm['name'],'-c','native'],limit=4096)
    hashes=_live_hashes(raw,'verify');fresh=kube.get('pod',pm['name'])
    if (hashes!=plan['expected_hashes'] or fresh.get('metadata',{}).get('uid')!=pm['uid']
        or not fresh.get('metadata',{}).get('resourceVersion') or fresh.get('spec')!=pod['spec']
        or fresh.get('status',{}).get('containerStatuses')!=pod['status']['containerStatuses']):
        raise Blocked('space_native_successor_proof_changed')
    if _successor_context(session)!=plan:raise Blocked('space_native_successor_changed')
    job=kube.get('job',name);jm=job.get('metadata',{})
    if (jm.get('uid')!=uid or not jm.get('resourceVersion') or jm.get('deletionTimestamp')
        or owner.digest(job.get('spec',{}))!=owner.digest(created['spec'])
        or job.get('status',{}).get('succeeded')!=1 or job.get('status',{}).get('active',0)):
        raise Blocked('space_native_successor_job_changed')
    path,intent,response=_successor_effect(session,plan,'delete',uid,jm['resourceVersion'])
    _check_delete_response(response,uid);_done(path,intent,{'uid':uid,'resourceVersion':jm['resourceVersion']})
    deadline=time.monotonic()+60
    while True:
        rows=owner.private._json(owner._observation(session,'space-native-successor-absence',owner.PREFIX+['get','--raw',
            '/apis/batch/v1/namespaces/voice-staging/jobs?fieldSelector=metadata.name%3D'+name+'&limit=2']))
        rm=rows.get('metadata',{})
        if (rows.get('apiVersion')!='batch/v1' or rows.get('kind')!='JobList' or not rm.get('resourceVersion')
            or rm.get('continue') or not isinstance(rows.get('items'),list)):raise Blocked('space_native_successor_inventory_invalid')
        if not rows['items'] and not _successor_pods(session,name):break
        if rows['items'] and (len(rows['items'])!=1 or rows['items'][0].get('metadata',{}).get('uid')!=uid):
            raise Blocked('space_native_successor_job_changed')
        if time.monotonic()>=deadline:raise Blocked('space_native_successor_cleanup_timeout')
        time.sleep(1)
    if _successor_context(session)!=plan:raise Blocked('space_native_successor_changed')
    _successor_unmounted(plan)
    receipt={'schema':'voice-staging-space-native-successor-v1','operation':session.operation,
        'plan_sha256':owner.digest(plan),'predecessor':plan['predecessor'],'job_uid':uid,'pod_uid':pm['uid'],
        'pod_before_rv':pm['resourceVersion'],'pod_after_rv':fresh['metadata']['resourceVersion'],
        'hashes':hashes,'cleanup':'verified'}
    owner._write(base/'space-native-successor.json',receipt)
    return load_native_successor(session)


def _successor_records(operation,binding):
    """Verify durable C2 records using an already authenticated operation binding."""
    if not isinstance(operation,str) or not re.fullmatch('[a-f0-9]{12}',operation):
        raise Blocked('space_native_successor_changed')
    base=owner.guard.ROOT/'installed'/'cohort-preservation'/operation
    plan=owner._read(base/'space-native-successor-plan.json')
    receipt=owner._read(base/'space-native-successor.json')
    create=owner._read(base/'effects'/'space-native-successor-create.json')
    delete=owner._read(base/'effects'/'space-native-successor-delete.json')
    name=plan['job']['metadata']['name']
    for kind,effect in (('create',create),('delete',delete)):
        if (set(effect)!={'schema','operation','effect_id','binding','argv','body_sha256','status','response'}
            or effect['schema']!='voice-staging-cohort-effect-v1' or effect['operation']!=operation
            or effect['effect_id']!='space-native-successor-'+kind or effect['binding']!=binding
            or effect['status']!='DONE'):raise Blocked('space_native_successor_changed')
    response=create['response'];deleted=delete['response']
    if (set(response)!={'uid','resourceVersion','spec_sha256'} or set(deleted)!={'uid','resourceVersion'}
        or not owner.guard.UUID.fullmatch(str(response['uid'])) or not response['resourceVersion']
        or not re.fullmatch('[a-f0-9]{64}',str(response['spec_sha256'])) or deleted['uid']!=response['uid']
        or not deleted['resourceVersion']):raise Blocked('space_native_successor_changed')
    options={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
        'preconditions':{'uid':deleted['uid'],'resourceVersion':deleted['resourceVersion']}}
    if (create['argv']!=owner.PREFIX+['create','-f','/dev/stdin','-o','json']
        or create['body_sha256']!=hashlib.sha256(owner.private._bytes(plan['job'])).hexdigest()
        or delete['argv']!=owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/'+name,'-f','/dev/stdin']
        or delete['body_sha256']!=hashlib.sha256(owner.private._bytes(options)).hexdigest()):
        raise Blocked('space_native_successor_changed')
    fields={'schema','operation','plan_sha256','predecessor','job_uid','pod_uid','pod_before_rv','pod_after_rv','hashes','cleanup'}
    if (set(receipt)!=fields or receipt['schema']!='voice-staging-space-native-successor-v1'
        or receipt['operation']!=operation or receipt['plan_sha256']!=owner.digest(plan)
        or receipt['predecessor']!=plan['predecessor'] or receipt['job_uid']!=response['uid']
        or not owner.guard.UUID.fullmatch(str(receipt['pod_uid'])) or not receipt['pod_before_rv'] or not receipt['pod_after_rv']
        or receipt['hashes']!=plan['expected_hashes'] or receipt['cleanup']!='verified'):
        raise Blocked('space_native_successor_changed')
    return plan,receipt


def load_native_successor(session):
    """Reload the exact C2 verification while the pre-cut Session is active."""
    owner._guard(session);owner._no_pending(session)
    plan,receipt=_successor_records(session.operation,session.binding())
    if _successor_context(session)!=plan:raise Blocked('space_native_successor_changed')
    _successor_unmounted(plan);owner._no_pending(session);session.check()
    return copy.deepcopy(receipt)


def _successor_history(operation,cohort_plan,prepared,binding):
    """Validate protected C2 proof without opening the current database."""
    plan,receipt=_successor_records(operation,binding)
    fields={'schema','operation','predecessor','prior_native_sha256','identity','pvc_uid','pv_uid','pv_name',
        'pvc_sha256','pv_sha256','config_sha256','secret_sha256','expected_hashes','tool_platform_digest',
        'tool_config_digest','node','job'}
    predecessor=plan.get('predecessor',{});old=predecessor.get('previous_operation')
    prior_active=owner._read(owner.guard.ROOT/'installed'/'prerequisites'/operation/'previous-active.json')
    if (set(plan)!=fields or plan.get('schema')!='voice-staging-space-native-successor-plan-v1'
        or plan.get('operation')!=operation or type(predecessor) is not dict
        or set(predecessor)!={'schema','operation','plan_sha256','previous_operation','active_sha256','native_sha256'}
        or predecessor.get('schema')!='voice-staging-space-native-predecessor-v1'
        or predecessor.get('operation')!=operation or predecessor.get('plan_sha256')!=owner.digest(cohort_plan)
        or not isinstance(old,str) or not re.fullmatch('[a-f0-9]{12}',old) or old==operation
        or predecessor.get('active_sha256')!=owner.digest(prior_active)
        or prepared['plan'].get('previous_active_sha256')!=owner.digest(prior_active)
        or prior_active.get('operation')!=old or prior_active.get('namespace_uid')!=cohort_plan['namespace_uid']):
        raise Blocked('space_native_successor_changed')
    previous=validate_historical_live_completed(old)
    root=owner.guard.ROOT/'installed'/'cohort-preservation'/old
    oldplan=owner._read(root/'space-live'/'plan.json');catalog=owner._read(root/'catalog.json');metadata=owner._read(root/'metadata.json')
    role=metadata['source']['catalog']['space_db']['database_owner'];database=plan['identity']['database']
    if (predecessor['native_sha256']!=owner.digest(previous) or plan['prior_native_sha256']!=owner.digest(previous)
        or plan['identity']['namespace_uid']!=catalog['engine']['namespace_uid']
        or database['system_identifier']!=catalog['engine']['system_identifier']
        or database['database_oid']!=catalog['databases']['space_db'] or database['database_owner']!=role
        or database['schema_owner']!=role or database['session_user']!=role
        or any(plan[key]!=previous['volume'][key] for key in ('pvc_uid','pv_uid','pv_name'))
        or plan['expected_hashes']!=previous['evidence_hashes']
        or plan['tool_platform_digest']!=oldplan['tool_platform_digest']
        or plan['tool_config_digest']!=oldplan['tool_config_digest']
        or plan['job']!=_successor_job(operation,oldplan,plan['node'])
        or any(not isinstance(plan[key],str) or not re.fullmatch('[a-f0-9]{64}',plan[key])
               for key in ('pvc_sha256','pv_sha256','config_sha256','secret_sha256'))):
        raise Blocked('space_native_successor_changed')
    return copy.deepcopy(receipt)


def load_retained_native_successor(session):
    """Revalidate the pre-cut C2 proof across typed successor phases, without SQL."""
    cohort_plan=owner.load_plan(session);owner._phase_continuity(session,cohort_plan);owner._no_pending(session)
    _,_,prepared=owner._source(session,cohort_plan)
    binding={'schema':'voice-staging-resource-fence-v1','context':session.context,'operation':session.operation,
        'helper_sha256':session.helper,'target_sha256':owner.digest(cohort_plan)}
    receipt=_successor_history(session.operation,cohort_plan,prepared,binding)
    owner._phase_continuity(session,cohort_plan);owner._no_pending(session);session.check()
    return receipt


def _validate_historical_native_successor(operation,qualified,rollout,binding):
    """Internal C2 history check after the caller authenticates PASS ancestry."""
    root=owner.guard.ROOT/'installed'/'cohort-preservation'/operation
    cohort=owner._read(root/'checkpoint.json')['plan']
    prepared=owner._read(owner.guard.ROOT/'installed'/'prerequisites'/operation/'prepared.json')
    initial={'schema':'voice-staging-resource-fence-v1','context':binding['context'],'operation':operation,
        'helper_sha256':binding['helper_sha256'],'target_sha256':owner.digest(cohort)}
    receipt=_successor_history(operation,cohort,prepared,initial)
    if (qualified.get('space_native_successor_sha256')!=owner.digest(receipt)
        or rollout.get('native_space_migration')!=receipt
        or rollout.get('target',{}).get('cohort_preservation_sha256')!=owner.digest(qualified)
        or [event for event in rollout.get('events',[]) if event.get('kind')=='native_space_migration_complete']
            !=[{'kind':'native_space_migration_complete','receipt_sha256':owner.digest(receipt)}]):
        raise Blocked('space_native_successor_changed')
    return receipt


def prepare_native_transition(session):
    """Choose legacy inspection or retained native verification from history."""
    predecessor=historical_native_predecessor(session)
    if predecessor is None:return inspect_source(session)
    return verify_native_successor(session)


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


def _check_fresh_pod(original,fresh,status):
    meta=fresh.get('metadata',{})
    rows=[item for item in fresh.get('status',{}).get('containerStatuses',[]) if item.get('name')=='inspect']
    if (meta.get('uid')!=original['metadata']['uid'] or meta.get('namespace')!='voice-staging'
        or not meta.get('resourceVersion') or fresh.get('spec')!=original['spec']
        or len(rows)!=1 or rows[0].get('imageID')!=status.get('imageID')
        or rows[0].get('state',{}).get('terminated',{}).get('exitCode')!=0):
        raise Blocked('space_inspect_pod_changed')


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
    if identity['namespace_uid']!=plan['namespace_uid']:raise Blocked('space_inspect_namespace_changed')
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
            or row['metadata'].get('namespace')!='voice-staging' or not row['metadata'].get('resourceVersion')
            or row['spec'].get('containers')!=created['spec']['template']['spec']['containers']
            or row['spec'].get('automountServiceAccountToken') is not False
            or statuses[0].get('state',{}).get('terminated',{}).get('exitCode')!=0
            or not any(statuses[0].get('imageID','').endswith(tool[key]) for key in ('platform_digest','config_digest'))):
            raise Blocked('space_inspect_pod_invalid')
        raw=owner._observation(session,'space-inspect-log',owner.PREFIX+['logs',row['metadata']['name'],'-c','inspect'],limit=1<<20)
        fresh=kube.get('pod',row['metadata']['name']);_check_fresh_pod(row,fresh,statuses[0])
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
             'pod_observation':{'uid':row['metadata']['uid'],'before_rv':row['metadata']['resourceVersion'],
                 'after_rv':fresh['metadata']['resourceVersion'],'image_id':statuses[0]['imageID']},
             'credential_binding':{'config_uid':config['metadata']['uid'],'config_rv':config['metadata']['resourceVersion'],
                 'config_sha256':owner.digest(config),'secret_uid':secret['metadata']['uid'],
                 'secret_rv':secret['metadata']['resourceVersion'],'secret_sha256':owner.digest(secret)}}
    owner._write(owner._base(session)/'space-inspection.json',receipt)
    return copy.deepcopy(receipt)


def _retained_record(base,operation,plan,tool):
    receipt=owner._read(base/'space-inspection.json')
    fields={'schema','operation','plan_sha256','tool_sha256','identity','job_uid','result','cleanup','credential_binding','pod_observation'}
    if (type(receipt) is not dict or set(receipt)!=fields or receipt['schema']!='voice-staging-space-inspection-v1'
        or receipt['operation']!=operation or receipt['plan_sha256']!=owner.digest(plan)
        or receipt['tool_sha256']!=owner.digest(tool) or receipt['cleanup']!='verified'
        or receipt.get('identity',{}).get('namespace_uid')!=plan.get('namespace_uid')):
        raise Blocked('space_inspect_receipt_changed')
    created=owner._read(base/'effects'/'space-inspect-create.json')
    deleted=owner._read(base/'effects'/'space-inspect-delete.json')
    name='voice-space-inspect-'+operation
    if (created.get('schema')!='voice-staging-cohort-effect-v1' or deleted.get('schema')!='voice-staging-cohort-effect-v1'
        or created.get('operation')!=operation or deleted.get('operation')!=operation
        or created.get('effect_id')!='space-inspect-create' or deleted.get('effect_id')!='space-inspect-delete'
        or created.get('argv')!=owner.PREFIX+['create','-f','/dev/stdin','-o','json']
        or deleted.get('argv')!=owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/'+name,'-f','/dev/stdin']
        or created.get('binding')!=deleted.get('binding') or not isinstance(created.get('binding'),dict)
        or created.get('status')!='DONE' or deleted.get('status')!='DONE'
        or created.get('response',{}).get('uid')!=receipt['job_uid']
        or deleted.get('response',{}).get('uid')!=receipt['job_uid']):raise Blocked('space_inspect_receipt_changed')
    return receipt


def load_retained_inspection(session):
    """Reload original evidence through the current protected successor lineage."""
    plan=owner.load_plan(session);owner._phase_continuity(session,plan);owner._no_pending(session)
    receipt=_retained_record(owner._base(session),session.operation,plan,space.load_tool(session))
    owner._phase_continuity(session,owner.load_plan(session))
    return copy.deepcopy(receipt)


def validate_historical_inspection(operation):
    """Read only root-owned inspection linked to an already released rollout."""
    if not isinstance(operation,str) or not re.fullmatch(r'[a-f0-9]{12}',operation):
        raise Blocked('space_inspect_history_invalid')
    base=owner.guard.ROOT/'installed'/'cohort-preservation'/operation
    checkpoint=owner._read(base/'checkpoint.json');plan=checkpoint['plan']
    qualified=owner._read(base/'qualified.json');tool_record=owner._read(base/'space-tool.json')
    rollout=owner._read(owner.guard.ROOT/('rollout-'+operation)/'checkpoint.json',limit=128<<20)
    binding=owner.resource_claim.historical_binding(rollout)
    source=owner._read(owner.guard.ROOT/'installed'/'source-authority'/(operation+'.json'))
    if (rollout.get('status')!='PASS' or rollout.get('phase')!='POST_RESTART'
        or rollout.get('fence_status')!='RELEASED' or rollout.get('operation')!=operation
        or checkpoint.get('status')!='QUALIFIED' or plan.get('operation')!=operation
        or plan.get('context')!=binding['context'] or plan.get('helper_sha256')!=binding['helper_sha256']
        or plan.get('source_capture_sha256')!=owner.digest(source)
        or plan.get('space_tool_sha256')!=owner.digest(tool_record)
        or tool_record.get('operation')!=operation or tool_record.get('selected') is not True
        or rollout.get('target',{}).get('cohort_preservation_sha256')!=owner.digest(qualified)
        or qualified.get('operation')!=operation or qualified.get('plan_sha256')!=owner.digest(plan)):
        raise Blocked('space_inspect_history_invalid')
    tool=tool_record['authenticated_publication']['publication']
    receipt=_retained_record(base,operation,plan,tool)
    if qualified.get('space_inspection_sha256')!=owner.digest(receipt):raise Blocked('space_inspect_history_invalid')
    return copy.deepcopy(receipt)


def load_inspection(session):
    """Reload the protected completed inspection and recheck its original inputs."""
    owner._guard(session);receipt=load_retained_inspection(session);kube=Kube()
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
