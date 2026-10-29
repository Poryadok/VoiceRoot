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
