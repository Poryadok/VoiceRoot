"""Fenced one-shot Auth migration phase; no standalone live invocation."""
import hashlib
import json
import re
import copy
import secrets
import subprocess
import selectors
import os
import time
import ipaddress
import pathlib
import tempfile
import cohort_readback as readback
import story_media_backup_cohort_restore as restore
import auth_live_migration_phase_eligibility as eligibility
import auth_live_migration_phase_probe as probe
from auth_flyway_upgrade_qualification import source_catalog, candidate_blob, marker_snapshot, snapshot_protected
import auth_user_adoption_runtime as runtime
import story_media_backup_restore as media_restore
import cohort_backup_phases
import media_backup_phase
import media_operation_journal
import resource_fencing
import full_cohort_package_phase as package_phase
import writer_cut
from frontend_runtime_executor import KubectlTransport


class PhaseError(RuntimeError): pass


def _phase_identity(phase):
    if type(phase) is not cohort_backup_phases.CohortPhases: raise ValueError()
    p=phase.phases
    if (type(p) is not media_backup_phase.BackupPhases or type(p.journal) is not media_operation_journal.Journal
            or type(p.fence) is not resource_fencing.Fence or type(p.fence.ledger) is not resource_fencing.KubeLedger
            or type(p.client) is not KubectlTransport or p.client.journal is not p.journal
            or p.fence.ledger.client is not p.client): raise ValueError()


def bind_package(phase, package, token):
    """Bind a source-owned package phase for repeated guarded verification."""
    try:
        _phase_identity(phase)
        if (type(package) is not package_phase.PackagePhase or not isinstance(token,str) or not token
                or hasattr(phase,'_auth_package_binding')): raise ValueError()
        phase._auth_package_binding=(package,token)
        try: _package_authority(phase)
        except BaseException:
            del phase._auth_package_binding
            raise
    except BaseException: raise PhaseError('protected candidate package authority unavailable') from None


def bind_metadata(phase, metadata_phase, token):
    """Bind the source-owned metadata receipt used by mixed cohort restores."""
    import full_cohort_backup_metadata as source
    try:
        _phase_identity(phase); p=phase.phases
        if (type(metadata_phase) is not source.BackupMetadataPhase or not isinstance(token,str) or not token
                or hasattr(phase,'_auth_metadata_binding') or metadata_phase.catalog is not phase.catalog_phase
                or metadata_phase.fence.context!=p.fence.context or metadata_phase.lock!=p.lock
                or metadata_phase.namespace_uid!=p.namespace_uid
                or metadata_phase.fence.ledger.read()!=p.fence.ledger.read()): raise ValueError()
        source.verify_completed(metadata_phase,token)
        if metadata_phase.fence.ledger.read()!=p.fence.ledger.read(): raise ValueError()
        phase._auth_metadata_binding=(metadata_phase,token)
    except BaseException: raise PhaseError('protected cohort metadata authority unavailable') from None


def _package_authority(phase):
    try:
        _phase_identity(phase); p=phase.phases
        package,token=phase._auth_package_binding
        if (type(package) is not package_phase.PackagePhase or not isinstance(token,str) or not token
                or package.fence.context!=p.fence.context or package.namespace_uid!=p.namespace_uid
                or package.lock['metadata']['uid']!=p.lock['metadata']['uid']
                or package.lock['data']['resource_operation']!=p.fence.context['operation_id']
                or p.lock['data']['resource_operation']!=p.fence.context['operation_id']): raise ValueError()
        _,left=p.fence.ledger.read(); _,right=package.fence.ledger.read()
        if (not isinstance(left,tuple) or not isinstance(right,tuple) or len(left)!=2 or len(right)!=2
                or left[0]!=right[0] or not left[0]): raise ValueError()
        receipt=package.verify_current(token)
        if (receipt.get('source_sha')!=p.fence.context['commit'] or receipt.get('operation_id')!=p.fence.context['operation_id']
                or receipt.get('images')!=p.fence.context['images']
                or receipt.get('platform_digests',{}).get('voice-auth/auth')!=p.intent['auth_live_candidate']['platform_digest']):
            raise ValueError()
        if 'auth_live_tool' in p.intent:
            tool=p.intent['auth_live_tool']; nodes=receipt.get('nodes',[])
            if not any(n.get('name')==tool['node_name'] and n.get('uid')==tool['node_uid']
                       and n.get('architecture')=='amd64' for n in nodes): raise ValueError()
        return receipt
    except BaseException: raise PhaseError('protected candidate package authority unavailable') from None


def observe_node(client, engine, intent, guard):
    try:
        if (not isinstance(intent,dict) or set(intent)!={'node_name','node_uid','flyway_platform_digest'}
                or not isinstance(intent['node_name'],str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',intent['node_name'])
                or not isinstance(intent['node_uid'],str) or not intent['node_uid']
                or not isinstance(intent['flyway_platform_digest'],str)
                or not re.fullmatch(r'sha256:[0-9a-f]{64}',intent['flyway_platform_digest'])): raise ValueError()
        def capture():
            if guard() is False: raise ValueError()
            pod=client.call('get','pod','voice-postgres-0','-o','json')
            if guard() is False: raise ValueError()
            if (pod['metadata'].get('uid')!=engine['pod_uid'] or pod['metadata'].get('name')!='voice-postgres-0'
                    or pod['metadata'].get('namespace')!='voice-dev' or pod['spec'].get('nodeName')!=intent['node_name']): raise ValueError()
            node=client.call('get','node',intent['node_name'],'-o','json')
            if guard() is False: raise ValueError()
            meta=node['metadata']; info=node['status']['nodeInfo']; ready=[x for x in node['status']['conditions'] if x.get('type')=='Ready']
            if (meta.get('name')!=intent['node_name'] or meta.get('uid')!=intent['node_uid'] or not meta.get('resourceVersion')
                    or info.get('operatingSystem')!='linux' or info.get('architecture')!='amd64'
                    or node.get('spec',{}).get('unschedulable') is True or len(ready)!=1 or ready[0].get('status')!='True'):
                raise ValueError()
            return {'name':meta['name'],'uid':meta['uid'],'resourceVersion':meta['resourceVersion'],'os':'linux','architecture':'amd64'}
        first=capture(); second=capture()
        if any(first[k]!=second[k] for k in ('name','uid','os','architecture')): raise ValueError()
        return second
    except BaseException: raise PhaseError('Flyway node identity unavailable') from None


def _docker_raw(reference):
    if not re.fullmatch(r'flyway/flyway@sha256:[0-9a-f]{64}',reference): raise ValueError()
    process=subprocess.Popen(['docker','buildx','imagetools','inspect','--raw',reference],
                             stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    selector=selectors.DefaultSelector(); chunks=[]; total=0; deadline=time.monotonic()+30
    try:
        selector.register(process.stdout,selectors.EVENT_READ)
        while selector.get_map():
            left=deadline-time.monotonic()
            if left<=0 or not selector.select(left): raise ValueError()
            chunk=os.read(process.stdout.fileno(),min(8192,(2<<20)+1-total))
            if not chunk: selector.unregister(process.stdout); break
            total+=len(chunk)
            if total>2<<20: raise ValueError()
            chunks.append(chunk)
        if process.wait(timeout=max(.01,deadline-time.monotonic())): raise ValueError()
        return b''.join(chunks)
    finally:
        selector.close()
        if process.poll() is None: process.kill(); process.wait()
        process.stdout.close()


def flyway_platform(index_digest):
    try:
        if not isinstance(index_digest,str) or not re.fullmatch(r'sha256:[0-9a-f]{64}',index_digest): raise ValueError()
        index=eligibility.publish_candidate.decode_manifest(_docker_raw('flyway/flyway@'+index_digest),index_digest)
        child_digest=eligibility.publish_candidate.platform_manifest(index)
        descriptors=[d for d in index['manifests'] if d.get('digest')==child_digest
                     and d.get('platform',{}).get('os')=='linux' and d.get('platform',{}).get('architecture')=='amd64'
                     and not d.get('platform',{}).get('variant')]
        if len(descriptors)!=1 or type(descriptors[0].get('size')) is not int: raise ValueError()
        raw=_docker_raw('flyway/flyway@'+child_digest)
        if len(raw)!=descriptors[0]['size']: raise ValueError()
        manifest=eligibility.publish_candidate.decode_manifest(raw,child_digest)
        if manifest.get('schemaVersion')!=2: raise ValueError()
        return child_digest
    except BaseException: raise PhaseError('Flyway platform identity unavailable') from None


def c1_objects(operation, sources, node):
    """Build the fixed canonical Job and immutable SQL ConfigMap from Git blobs."""
    short=operation_name(operation)
    if (not operation.startswith('media-c1-') or not isinstance(sources,dict) or not sources
            or not isinstance(node,str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',node)
            or len(node)>253): raise ValueError()
    data={}
    for name,raw in sorted(sources.items()):
        if not re.fullmatch(r'V[0-9]+__[A-Za-z0-9_]+\.sql',name) or not isinstance(raw,bytes): raise ValueError()
        data['auth-'+name]=raw.decode('utf-8')
    if len(json.dumps(data).encode())>900000: raise ValueError()
    cm={'apiVersion':'v1','kind':'ConfigMap','immutable':True,
        'metadata':runtime.metadata('voice-auth-user-sql-'+short,short),'data':data}
    cm['metadata']['annotations']['voice.dev/cohort-operation']=operation
    job=runtime.job_plan('migrate_auth',short,runtime.QUALIFIED_TOOLS,'voice-app-secrets',cm)
    job['metadata']['annotations']['voice.dev/cohort-operation']=operation
    job['spec']['template']['spec']['nodeName']=node
    return cm,job


def _digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


def _same_candidate(left,right):
    a=copy.deepcopy(left); b=copy.deepcopy(right)
    for item in (a,b):
        item.get('deployment',{}).pop('resourceVersion',None)
        item.pop('initial_resourceVersion',None)
    return a==b


def _expected_c1_data(clone, before):
    return {'markers_sha256':_digest(clone['before_markers' if before else 'expected_after_markers']),
            'roles_sha256':clone['roles_sha256'],
            'catalog_sha256':clone['before_catalog_sha256' if before else 'after_catalog_sha256']}


def _bind_clone_provenance(clone, provenance):
    keys=('manifest_sha256','custody_sha256','restore_sha256','metadata_sha256')
    if (not isinstance(clone,dict) or not isinstance(provenance,dict) or set(provenance)!=set(keys)
            or any(not isinstance(provenance[k],str) or not re.fullmatch(r'[0-9a-f]{64}',provenance[k])
                   or clone.get(k)!=provenance[k] for k in keys)):
        raise ValueError('clone and current cohort provenance differ')


def verify_writer_successor(phase, source_root):
    """Reconstruct the planned C1 objects from protected state; never create them."""
    try:
        p=phase.phases; _phase_identity(phase); _package_authority(phase)
        if phase._guard() is False or p.journal.intent!=p.intent: raise ValueError()
        steps=p.journal.state['steps']; record=copy.deepcopy(steps['auth-live-c1'])
        inputs=copy.deepcopy(steps['auth-live-c1-inputs']); row,revision=p.fence.ledger.read()
        if (not isinstance(record,dict) or set(record)!={'status','operation_id','snapshot_id','source_sha','nonce','input_sha256','terminal_sha256','configmap','job'}
                or record!=row.get('auth_live_c1') or record['status'] not in ('active','done')
                or not isinstance(inputs,dict) or set(inputs)!={'candidate','metadata','history','provenance','node','credential','before_data','clone_receipt_sha256'}
                or record['input_sha256']!=_digest(inputs) or not isinstance(record['nonce'],str)
                or not re.fullmatch(r'[0-9a-f]{32}',record['nonce'])): raise ValueError()
        if record['terminal_sha256'] is not None:
            if (not isinstance(record['terminal_sha256'],str) or not re.fullmatch(r'[0-9a-f]{64}',record['terminal_sha256'])
                    or _digest(steps['auth-live-c1-terminal'])!=record['terminal_sha256']): raise ValueError()
        binding,provenance=verified_sources(phase,inputs['metadata'],26)
        if (inputs['provenance']!=provenance or record['operation_id']!=binding['operation_id']
                or record['snapshot_id']!=binding['snapshot_id'] or record['source_sha']!=binding['source_sha']
                or record['source_sha']!=p.fence.context['commit'] or record['operation_id']!=p.fence.context['operation_id']): raise ValueError()
        tool=p.intent['auth_live_tool']; node=inputs['node']; candidate=inputs['candidate']
        if (set(tool)!={'node_name','node_uid','flyway_platform_digest'} or node.get('name')!=tool['node_name']
                or node.get('uid')!=tool['node_uid'] or candidate.get('source_sha')!=record['source_sha']
                or candidate.get('image')!=p.fence.context['images']['voice-auth/auth']
                or candidate.get('policy')!={'flyway':'disabled','persistence':'jdbc','selector':'absent'}
                or candidate.get('route') not in ({'host':h,'database':'auth_db','role':'voice'} for h in
                    ('voice-postgres','voice-postgres.voice-dev','voice-postgres.voice-dev.svc','voice-postgres.voice-dev.svc.cluster.local'))
                or not isinstance(inputs['credential'],dict) or set(inputs['credential'])!={'uid','resourceVersion'}
                or any(not isinstance(v,str) or not v for v in inputs['credential'].values())): raise ValueError()
        sources,catalog=source_catalog(source_root,record['source_sha'])
        if catalog!=inputs['metadata']['migrations']['auth_db']['files']: raise ValueError()
        clone=steps['cohort-auth-flyway-private']; clone_record=steps['cohort-auth-flyway']
        if _digest(clone)!=inputs['clone_receipt_sha256']: raise ValueError()
        _bind_clone_provenance(clone,provenance)
        if inputs['before_data']!=_expected_c1_data(clone,True): raise ValueError()
        validate_clone_receipt(clone,clone_record,row.get('cohort_auth_flyway'),record['operation_id'],
                               record['snapshot_id'],record['source_sha'],catalog,inputs['history'])
        cm,job=c1_objects(record['operation_id'],sources,tool['node_name'])
        for key,obj in (('configmap',cm),('job',job)):
            resource=record[key]
            if (not isinstance(resource,dict) or set(resource)!={'state','name','spec_sha256','uid','resourceVersion'}
                    or resource['name']!=obj['metadata']['name'] or resource['spec_sha256']!=_digest(obj)
                    or resource['state'] not in ('absent','pending','owned','removing','removed')): raise ValueError()
            if resource['state'] in ('absent','pending'):
                if resource['uid'] is not None or resource['resourceVersion'] is not None: raise ValueError()
            elif any(not isinstance(resource[k],str) or not resource[k] for k in ('uid','resourceVersion')): raise ValueError()
        cm_state=record['configmap']['state']; job_state=record['job']['state']
        if job_state!='absent' and cm_state not in ('owned','removing','removed'): raise ValueError()
        if cm_state in ('removing','removed') and job_state!='removed': raise ValueError()
        if (cm_state in ('removing','removed') or job_state in ('removing','removed')) and record['terminal_sha256'] is None: raise ValueError()
        if record['status']=='done' and (cm_state!='removed' or job_state!='removed' or record['terminal_sha256'] is None): raise ValueError()
        final,final_revision=p.fence.ledger.read()
        if (phase._guard() is False or p.journal.state['steps']['auth-live-c1']!=record
                or p.journal.state['steps']['auth-live-c1-inputs']!=inputs
                or final!=row or final_revision!=revision): raise ValueError()
        execution={'database':'auth_db','database_oid':binding['sources']['auth_db']['database_oid'],
                   'role':'voice','application_name':job['metadata']['name'],
                   'container_name':'operation','node_name':tool['node_name'],'node_uid':tool['node_uid'],
                   'platform_digest':tool['flyway_platform_digest']}
        return {'configmap':cm,'job':job,'record':record,'execution':execution}
    except BaseException: raise PhaseError('Auth migration successor unavailable') from None


def verify_writer_terminal(phase, source_root):
    """Validate the protected terminal receipt; cut independently checks live objects."""
    try:
        successor=verify_writer_successor(phase,source_root); record=successor['record']
        steps=phase.phases.journal.state['steps']; terminal=copy.deepcopy(steps['auth-live-c1-terminal'])
        inputs=steps['auth-live-c1-inputs']; clone=steps['cohort-auth-flyway-private']
        if (not isinstance(terminal,dict) or set(terminal)!={'operation_id','snapshot_id','source_sha','job_uid',
                'pod_uid','pod_ip','node_uid','image_id','before_history','after_history','clone_receipt_sha256','sessions',
                'gate_sha256','credential','before_data','after_data'}
                or record['terminal_sha256']!=_digest(terminal) or record['job']['state'] not in ('owned','removing','removed')
                or terminal['operation_id']!=record['operation_id'] or terminal['snapshot_id']!=record['snapshot_id']
                or terminal['source_sha']!=record['source_sha'] or terminal['job_uid']!=record['job']['uid']
                or not isinstance(terminal['pod_uid'],str) or not terminal['pod_uid']
                or terminal['node_uid']!=successor['execution']['node_uid']
                or terminal['image_id']!=successor['execution']['platform_digest']
                or terminal['sessions']!=[] or terminal['before_history']!=inputs['history']
                or terminal['clone_receipt_sha256']!=_digest(clone) or terminal['credential']!=inputs['credential']
                or terminal['before_data']!=_expected_c1_data(clone,True)
                or terminal['after_data']!=_expected_c1_data(clone,False)
                or not isinstance(terminal['gate_sha256'],str) or not re.fullmatch(r'[0-9a-f]{64}',terminal['gate_sha256'])): raise ValueError()
        ipaddress.ip_address(terminal['pod_ip'])
        verify_transition(terminal['before_history'],terminal['after_history'],clone['after_history'])
        return terminal
    except BaseException: raise PhaseError('Auth migration terminal receipt unavailable') from None


def verify_writer_cleanup(phase, source_root):
    """Return recorded removed identities; the cut must independently verify absence."""
    try:
        verify_writer_terminal(phase,source_root)
        record=verify_writer_successor(phase,source_root)['record']
        if (record['status']!='done' or record['job']['state']!='removed'
                or record['configmap']['state']!='removed'): raise ValueError()
        return {'job':copy.deepcopy(record['job']),'configmap':copy.deepcopy(record['configmap'])}
    except BaseException: raise PhaseError('Auth migration cleanup receipt unavailable') from None


def _check_configmap(authored, actual):
    expected=authored['metadata']; meta=actual['metadata']
    if (authored.get('kind')!='ConfigMap' or actual.get('kind')!='ConfigMap'
            or actual.get('apiVersion')!='v1' or actual.get('immutable') is not True
            or actual.get('data')!=authored['data'] or actual.get('binaryData')
            or meta.get('name')!=expected['name'] or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
            or any(meta.get('annotations',{}).get(k)!=v for k,v in expected['annotations'].items())
            or any(meta.get('labels',{}).get(k)!=v for k,v in expected['labels'].items())): raise ValueError()


def verify_writer_configmap(authored, actual):
    try:
        _check_configmap(authored,actual); meta=actual['metadata']
        if any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion')): raise ValueError()
        return {'uid':meta['uid'],'resourceVersion':meta['resourceVersion']}
    except BaseException: raise PhaseError('Auth SQL ConfigMap identity or contents differ') from None


def _check_job(authored, actual):
    if authored.get('kind')!='Job' or actual.get('kind')!='Job' or actual.get('apiVersion')!='batch/v1': raise ValueError()
    expected=authored['metadata']; meta=actual['metadata']
    if (meta.get('name')!=expected['name'] or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
            or any(meta.get('annotations',{}).get(k)!=v for k,v in expected['annotations'].items())
            or any(meta.get('labels',{}).get(k)!=v for k,v in expected['labels'].items())): raise ValueError()
    runtime.validate_defaulted_job(authored,actual)


def verify_writer_job(authored, actual):
    """Check an API-returned Job against the canonical authored specification."""
    try:
        _check_job(authored,actual); meta=actual['metadata']
        if any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion')): raise ValueError()
        return {'uid':meta['uid'],'resourceVersion':meta['resourceVersion']}
    except BaseException: raise PhaseError('Auth Job identity or specification differs') from None


def verify_writer_pod(authored, job, pod, execution):
    """Check the one owned Pod; execution must come from verify_writer_successor."""
    try:
        identity=verify_writer_job(authored,job); meta=pod['metadata']; status=pod['status']
        owners=meta.get('ownerReferences')
        if (meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
                or any(not isinstance(meta.get(k),str) or not meta[k] for k in ('name','uid'))
                or not isinstance(owners,list) or len(owners)!=1
                or owners[0].get('apiVersion')!='batch/v1' or owners[0].get('kind')!='Job'
                or owners[0].get('name')!=authored['metadata']['name'] or owners[0].get('uid')!=identity['uid']
                or owners[0].get('controller') is not True
                or any(meta.get('labels',{}).get(k)!=v for k,v in authored['spec']['template']['metadata']['labels'].items())
                or pod['spec'].get('nodeName')!=execution['node_name']
                or authored['spec']['template']['spec']['nodeName']!=execution['node_name']
                or execution['container_name']!='operation'): raise ValueError()
        comparison=copy.deepcopy(authored)
        comparison['spec']['template']['spec']=pod['spec']
        runtime.validate_defaulted_job(authored,comparison)
        phase=status.get('phase'); ip=status.get('podIP'); statuses=status.get('containerStatuses',[])
        if phase not in ('Pending','Running','Succeeded') or not isinstance(statuses,list) or len(statuses)>1: raise ValueError()
        image_id=None
        if statuses:
            item=statuses[0]
            if item.get('name')!='operation' or type(item.get('restartCount')) is not int or item['restartCount']!=0: raise ValueError()
            raw=item.get('imageID',''); expected=execution['platform_digest']
            if raw==expected or raw=='containerd://'+expected or raw.endswith('@'+expected): image_id=expected
            elif raw or phase!='Pending': raise ValueError()
        if phase in ('Running','Succeeded') and (not ip or image_id is None): raise ValueError()
        if ip is not None: ipaddress.ip_address(ip)
        return {'uid':meta['uid'],'ip':ip,'image_id':image_id,'phase':phase}
    except BaseException: raise PhaseError('Auth Job Pod identity or specification differs') from None


def _c1_resource_transition(phase, source_root, kind, target_state, identity=None):
    """Persist one one-way effect transition; caller performs the fixed effect."""
    started=False
    try:
        if kind not in ('configmap','job'): raise ValueError()
        p=phase.phases; record=verify_writer_successor(phase,source_root)['record']
        resource=record[kind]; previous=resource['state']
        allowed={('absent','pending'),('pending','owned'),('owned','removing'),('removing','removed')}
        if (previous,target_state) not in allowed: raise ValueError()
        if target_state=='pending' and kind=='job' and record['configmap']['state']!='owned': raise ValueError()
        if target_state=='owned':
            if (not isinstance(identity,tuple) or len(identity)!=2
                    or any(not isinstance(v,str) or not v for v in identity)): raise ValueError()
        elif identity is not None: raise ValueError()
        if target_state=='removing':
            verify_writer_terminal(phase,source_root)
            if kind=='configmap' and record['job']['state']!='removed': raise ValueError()
        target=copy.deepcopy(record); target[kind]['state']=target_state
        if target_state=='owned': target[kind]['uid'],target[kind]['resourceVersion']=identity
        if target['job']['state']=='removed' and target['configmap']['state']=='removed': target['status']='done'
        row,revision=p.fence.ledger.read()
        if row.get('auth_live_c1')!=record or p.journal.state['steps'].get('auth-live-c1')!=record: raise ValueError()
        started=True
        p.journal.state['steps']['auth-live-c1']=copy.deepcopy(target); p.journal.save()
        current,rev=p.fence.ledger.read()
        if current!=row or rev!=revision: raise ValueError()
        current['auth_live_c1']=copy.deepcopy(target); current['step']='auth-live-c1-'+kind+'-'+target_state
        p.fence.ledger.write(revision,current)
        if verify_writer_successor(phase,source_root)['record']!=target: raise ValueError()
        return target
    except BaseException:
        if started: readback._poison(phase)
        raise PhaseError('Auth resource transition unavailable; reconcile uncertain effects') from None


def _commit_c1_terminal(phase, source_root, receipt):
    started=False
    try:
        p=phase.phases; record=verify_writer_successor(phase,source_root)['record']
        if (record['status']!='active' or record['terminal_sha256'] is not None
                or record['job']['state']!='owned' or record['configmap']['state']!='owned'
                or 'auth-live-c1-terminal' in p.journal.state['steps']): raise ValueError()
        row,revision=p.fence.ledger.read()
        if row.get('auth_live_c1')!=record: raise ValueError()
        target=copy.deepcopy(record); target['terminal_sha256']=_digest(receipt)
        started=True
        p.journal.state['steps']['auth-live-c1-terminal']=copy.deepcopy(receipt)
        p.journal.state['steps']['auth-live-c1']=copy.deepcopy(target); p.journal.save()
        current,rev=p.fence.ledger.read()
        if current!=row or rev!=revision: raise ValueError()
        current['auth_live_c1']=copy.deepcopy(target); current['step']='auth-live-c1-terminal'
        p.fence.ledger.write(revision,current)
        if verify_writer_terminal(phase,source_root)!=receipt: raise ValueError()
        return target
    except BaseException:
        if started: readback._poison(phase)
        raise PhaseError('Auth terminal outcome uncertain; retain lock and reconcile') from None


def _cut_identity(phase, cut):
    _phase_identity(phase); p=phase.phases
    if (type(cut) is not writer_cut.WriterCut or cut.fence.context!=p.fence.context
            or cut.namespace_uid!=p.namespace_uid or cut.lock['metadata']['uid']!=p.lock['metadata']['uid']):
        raise ValueError()
    _,a=p.fence.ledger.read(); _,b=cut.fence.ledger.read()
    if not isinstance(a,tuple) or not isinstance(b,tuple) or len(a)!=2 or len(b)!=2 or not a[0] or a[0]!=b[0]: raise ValueError()


def _bind_cut_scope(cut, binding):
    scope=cut.admission['session_scope']; auth=binding['sources']['auth_db']
    if (scope.get('engine')!=auth['engine'] or scope.get('server_start')!=auth['server_start']
            or not isinstance(scope.get('databases'),list) or not isinstance(scope.get('database_oids'),dict)):
        raise ValueError()
    for db in set(scope['databases']) & set(binding['sources']):
        if db!='roles' and scope['database_oids'].get(db)!=binding['sources'][db]['database_oid']: raise ValueError()


def create_c1_objects(phase, cut, source_root, actor, token):
    """Create the fixed CM and Job once after the writer cut binds this phase."""
    started=False
    try:
        _cut_identity(phase,cut); p=phase.phases; drain=lambda:cut.verify_migration_running()
        if cut.verify_migration_running() is False: raise ValueError()
        readback._guard(phase,drain)
        planned=verify_writer_successor(phase,source_root); cm,job,record=planned['configmap'],planned['job'],planned['record']
        if record['configmap']['state']!='absent' or record['job']['state']!='absent': raise ValueError()
        for kind,obj in (('configmap',cm),('job',job)):
            actual=p.client.call('get',kind,obj['metadata']['name'],'--ignore-not-found=true','-o','json')
            if actual: raise ValueError()
        dry=p.client.call('create','--dry-run=server','-f','-','-o','json',body=cm)
        _check_configmap(cm,dry)
        if cut.verify_migration_running() is False: raise ValueError()
        readback._guard(phase,drain)
        _c1_resource_transition(phase,source_root,'configmap','pending'); started=True
        if cut.verify_migration_running() is False: raise ValueError()
        actual=p.client.call('create','-f','-','-o','json',body=cm)
        identity=verify_writer_configmap(cm,actual)
        _c1_resource_transition(phase,source_root,'configmap','owned',(identity['uid'],identity['resourceVersion']))
        if cut.verify_migration_running() is False: raise ValueError()
        inputs=p.journal.state['steps']['auth-live-c1-inputs']; binding,_=verified_sources(phase,inputs['metadata'],26)
        _bind_cut_scope(cut,binding)
        auth=binding['sources']['auth_db']; guard=lambda:readback._guard(phase,drain)
        candidate=eligibility.observe_candidate(phase,drain,source_root,actor,token)
        if not _same_candidate(candidate,inputs['candidate']): raise ValueError()
        secret=probe.verify_login(p.client,{'engine':auth['engine'],'database_oid':auth['database_oid'],
                                  'server_start':auth['server_start']},guard)
        if secret!=inputs['credential']: raise ValueError()
        node=observe_node(p.client,auth['engine'],p.intent['auth_live_tool'],guard)
        if node['name']!=inputs['node']['name'] or node['uid']!=inputs['node']['uid']: raise ValueError()
        scope={'engine':auth['engine'],'server_start':auth['server_start'],'databases':binding['databases'],
               'database_oids':{db:binding['sources'][db]['database_oid'] for db in binding['databases']}}
        database=probe.inspect(scope,guard)
        if database['history']!=inputs['history'] or database['sessions'] or database['prepared']!=0: raise ValueError()
        dry=p.client.call('create','--dry-run=server','-f','-','-o','json',body=job)
        _check_job(job,dry)
        if cut.verify_migration_running() is False: raise ValueError()
        readback._guard(phase,drain)
        pending=_c1_resource_transition(phase,source_root,'job','pending')
        if cut.verify_migration_running() is False: raise ValueError()
        readback._guard(phase,drain)
        if cut.verify_migration_pending() is False: raise ValueError()
        checked=verify_writer_successor(phase,source_root)
        if checked['record']!=pending or checked['job']!=job: raise ValueError()
        actual=p.client.call('create','-f','-','-o','json',body=job)
        identity=verify_writer_job(job,actual)
        result=_c1_resource_transition(phase,source_root,'job','owned',(identity['uid'],identity['resourceVersion']))
        if cut.verify_migration_running() is False: raise ValueError()
        return result
    except BaseException:
        if started: readback._poison(phase)
        raise PhaseError('Auth Job creation unavailable; retain fence and reconcile any pending effect') from None


def _inspect_c1_gate(phase, drain, source_root, source, engine):
    """Query only the candidate-owned gate SQL and validate its pinned V31 bytes."""
    gate=candidate_blob(source_root,source,'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql')
    v31=candidate_blob(source_root,source,'src/backend/auth/src/main/resources/db/migration/V31__auth_identity_ddl_barrier.sql')
    if not isinstance(gate,bytes) or not isinstance(v31,bytes) or len(gate)>131072 or len(v31)>131072: raise ValueError()
    guard=lambda:readback._guard(phase,drain)
    snapshot=probe.metadata.kube_engine_snapshot(probe.metadata.linux_private_bytes,guard)
    guard()
    if snapshot()!=engine: raise ValueError()
    result=probe._query(gate.decode('utf-8'))
    guard()
    if snapshot()!=engine: raise ValueError()
    with tempfile.TemporaryDirectory() as tmp:
        root=pathlib.Path(tmp); path=root/'src/backend/auth/src/main/resources/db/migration/V31__auth_identity_ddl_barrier.sql'
        path.parent.mkdir(parents=True); path.write_bytes(v31)
        runtime.validate_ddl_gate(result,root)
    guard()
    return result


def _c1_data_snapshot(engine, clone, before, guard):
    """Verify fixed pre-existing rows, role catalog and Auth catalog against clone."""
    if type(before) is not bool or not isinstance(clone,dict): raise ValueError()
    snapshot=probe.metadata.kube_engine_snapshot(probe.metadata.linux_private_bytes,guard)
    if guard() is False or snapshot()!=engine: raise ValueError()
    def query(sql):
        if guard() is False: raise ValueError()
        value=probe._query(sql)
        if guard() is False: raise ValueError()
        return value
    columns=clone['original_table_columns']
    marks=marker_snapshot(query,columns,before=before)
    if before:
        normalized=marker_snapshot(query,columns,before=True,normalize=True)
        if marks!=clone['before_markers'] or normalized!=clone['expected_after_markers']: raise ValueError()
    elif marks!=clone['expected_after_markers']: raise ValueError()
    roles=query(media_restore.role_catalog_sql())
    catalog=query(restore.AUTH_CATALOG)
    roles_hash=_digest(roles); catalog_hash=_digest(catalog)
    if (roles_hash!=clone['roles_sha256'] or catalog_hash!=clone['before_catalog_sha256' if before else 'after_catalog_sha256']
            or snapshot()!=engine or guard() is False): raise ValueError()
    return {'markers_sha256':_digest(marks),'roles_sha256':roles_hash,'catalog_sha256':catalog_hash}


def _running_guard(phase, cut):
    if phase._guard() is False or cut.verify_migration_running() is False: raise ValueError()


def finish_c1(phase, cut, source_root, actor, token):
    """Observe the owned Job and persist exact terminal DB evidence under cut."""
    try:
        _cut_identity(phase,cut); p=phase.phases
        planned=verify_writer_successor(phase,source_root); record=planned['record']; authored=planned['job']
        if record['job']['state']!='owned' or record['terminal_sha256'] is not None: raise ValueError()
        deadline=time.monotonic()+180; guard=lambda:_running_guard(phase,cut)
        while True:
            guard()
            actual=p.client.call('get','job',authored['metadata']['name'],'-o','json')
            identity=verify_writer_job(authored,actual)
            if identity['uid']!=record['job']['uid']: raise ValueError()
            status=actual.get('status',{})
            if (type(status.get('failed',0)) is not int or status.get('failed',0)!=0
                    or type(status.get('active',0)) is not int or type(status.get('succeeded',0)) is not int
                    or any(c.get('type')=='Failed' and c.get('status')=='True' for c in status.get('conditions',[]))):
                raise ValueError()
            pods=p.client.call('get','pods','-l','voice.dev/adoption-job='+authored['metadata']['name'],'-o','json').get('items')
            if not isinstance(pods,list) or len(pods)>1: raise ValueError()
            pod=verify_writer_pod(authored,actual,pods[0],planned['execution']) if pods else None
            complete=any(c.get('type')=='Complete' and c.get('status')=='True' for c in status.get('conditions',[]))
            if complete and status.get('succeeded')==1 and status.get('active',0)==0 and pod and pod['phase']=='Succeeded': break
            if time.monotonic()>=deadline: raise ValueError()
            time.sleep(1)
        guard(); inputs=p.journal.state['steps']['auth-live-c1-inputs']
        binding,provenance=verified_sources(phase,inputs['metadata'],26); _bind_cut_scope(cut,binding)
        if provenance!=inputs['provenance']: raise ValueError()
        auth=binding['sources']['auth_db']
        scope={'engine':auth['engine'],'server_start':auth['server_start'],'databases':binding['databases'],
               'database_oids':{db:binding['sources'][db]['database_oid'] for db in binding['databases']}}
        observed=probe.inspect(scope,guard)
        if observed['sessions'] or observed['prepared']!=0: raise ValueError()
        clone=p.journal.state['steps']['cohort-auth-flyway-private']
        verify_transition(inputs['history'],observed['history'],clone['after_history'])
        gate=_inspect_c1_gate(phase,guard,source_root,binding['source_sha'],auth['engine'])
        after_data=_c1_data_snapshot(auth['engine'],clone,False,guard)
        secret=probe.verify_login(p.client,{'engine':auth['engine'],'database_oid':auth['database_oid'],
                                           'server_start':auth['server_start']},guard)
        if secret!=inputs['credential']: raise ValueError()
        node=observe_node(p.client,auth['engine'],p.intent['auth_live_tool'],guard)
        if node['name']!=inputs['node']['name'] or node['uid']!=inputs['node']['uid']: raise ValueError()
        candidate=eligibility.observe_candidate(phase,guard,source_root,actor,token)
        if not _same_candidate(candidate,inputs['candidate']): raise ValueError()
        guard()
        if probe.inspect(scope,guard)!=observed: raise ValueError()
        receipt={'operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                 'source_sha':binding['source_sha'],'job_uid':identity['uid'],'pod_uid':pod['uid'],'pod_ip':pod['ip'],
                 'node_uid':planned['execution']['node_uid'],'image_id':pod['image_id'],
                 'before_history':copy.deepcopy(inputs['history']),'after_history':copy.deepcopy(observed['history']),
                 'clone_receipt_sha256':_digest(clone),'sessions':[],'gate_sha256':_digest(gate),'credential':secret,
                 'before_data':inputs['before_data'],'after_data':after_data}
        _commit_c1_terminal(phase,source_root,receipt)
        if cut.finish_migration() is False: raise ValueError()
        return receipt
    except BaseException:
        readback._poison(phase)
        raise PhaseError('Auth migration terminal outcome uncertain; retain lock and reconcile') from None


def cleanup_c1(phase, cut, source_root):
    """Delete only terminal owned Job and ConfigMap with UID/RV preconditions."""
    try:
        _cut_identity(phase,cut); p=phase.phases
        verify_writer_terminal(phase,source_root)
        for kind in ('job','configmap'):
            _running_guard(phase,cut)
            planned=verify_writer_successor(phase,source_root); record=planned['record']; resource=record[kind]
            if resource['state']!='owned': raise ValueError()
            authored=planned[kind]; name=authored['metadata']['name']
            actual=p.client.call('get',kind,name,'-o','json')
            identity=verify_writer_job(authored,actual) if kind=='job' else verify_writer_configmap(authored,actual)
            if identity['uid']!=resource['uid']: raise ValueError()
            _c1_resource_transition(phase,source_root,kind,'removing')
            _running_guard(phase,cut)
            prefix='/apis/batch/v1/namespaces/voice-dev/jobs/' if kind=='job' else '/api/v1/namespaces/voice-dev/configmaps/'
            options={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
                     'preconditions':{'uid':identity['uid'],'resourceVersion':identity['resourceVersion']}}
            p.client.call('delete','--raw='+prefix+name,'-f','-',body=options)
            deadline=time.monotonic()+60
            while True:
                if phase._guard() is False: raise ValueError()
                remaining=p.client.call('get',kind,name,'--ignore-not-found=true','-o','json')
                pods=[]
                if kind=='job':
                    pods=p.client.call('get','pods','-l','voice.dev/adoption-job='+name,'-o','json').get('items')
                    if not isinstance(pods,list): raise ValueError()
                if not remaining and not pods: break
                if remaining and remaining.get('metadata',{}).get('uid')!=identity['uid']: raise ValueError()
                if time.monotonic()>=deadline: raise ValueError()
                time.sleep(1)
            _c1_resource_transition(phase,source_root,kind,'removed')
        verify_writer_cleanup(phase,source_root)
        if cut.accept_migration_cleanup() is False: raise ValueError()
        return True
    except BaseException:
        readback._poison(phase)
        raise PhaseError('Auth cleanup outcome uncertain; retain lock and reconcile') from None


def verified_sources(phase, metadata, version):
    """Verify current cohort custody and restore records from the common fence."""
    original_metadata=metadata; metadata=snapshot_protected(metadata)
    p=phase.phases; row,revision=p.fence.ledger.read(); row=copy.deepcopy(row)
    snapshot=copy.deepcopy(readback._snapshot(phase,row)); binding,planned,marker,_=snapshot
    rd=copy.deepcopy(p.journal.state['steps']['cohort-readback']); rs=copy.deepcopy(p.journal.state['steps']['cohort-restore'])
    if (not isinstance(rd,dict) or set(rd)!={'status','intent','receipt'}
            or not isinstance(rs,dict) or set(rs)!={'status','intent','receipt'}
            or rd!=row.get('cohort_readback') or rs!=row.get('cohort_restore')
            or rd.get('status')!='done' or rs.get('status')!='done'): raise ValueError()
    ri,rr=rd['intent'],rd['receipt']; artifact=ri['binding']
    if (set(ri)!={'status','artifact_id','binding','manifest_sha256','invocation_nonce'}
            or set(artifact)!={'operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256','run_id','head_sha','created_at'}
            or ri['status']!='pending' or ri['invocation_nonce']!=p.invocation
            or type(ri['artifact_id']) is not int or ri['artifact_id']<=0
            or type(artifact['run_id']) is not int or not isinstance(artifact['created_at'],str) or not artifact['created_at']
            or ri['manifest_sha256']!=marker['manifest_sha256']
            or any(artifact[k]!=planned[k] for k in ('operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256'))
            or artifact['head_sha']!=binding['source_sha'] or str(artifact['run_id'])!=planned['run_id']
            or rr!={'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot',
                    'artifact_id':ri['artifact_id'],**{k:v for k,v in artifact.items() if k!='created_at'}}): raise ValueError()
    intent,receipt=rs['intent'],rs['receipt']
    changed=([db for db in binding['databases']
              if restore.classify_plan(db,metadata['migrations'][db],True)=='migrate']
             if 'full_catalog' in binding else [])
    extra_intent={}; extra_receipt={}
    if changed:
        import cohort_restore_journal
        metadata_phase,token=phase._auth_metadata_binding
        roles=restore._qualified_roles(metadata_phase,token,phase.catalog_phase,binding,binding['databases'],metadata,changed)
        extra_intent={'migration_roles_sha256':_digest(roles)}
        extra_receipt={'migration_logins':{db:{'role':roles[db]['role'],
                         'from':metadata['migrations'][db]['from'],'to':metadata['migrations'][db]['to']} for db in changed}}
        for db in changed:
            proof=cohort_restore_journal.verify_go_role(phase,metadata_phase,token,db)
            if (proof['migration_roles_sha256']!=extra_intent['migration_roles_sha256']
                    or proof['source_sha']!=binding['source_sha'] or proof['snapshot_id']!=binding['snapshot_id']
                    or proof['manifest_sha256']!=marker['manifest_sha256']
                    or any(proof[k]!=extra_receipt['migration_logins'][db][k] for k in ('role','from','to'))): raise ValueError()
        if p.fence.ledger.read() != metadata_phase.fence.ledger.read(): raise ValueError()
    if (intent!={'status':'pending','operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                       'invocation_nonce':p.invocation,'manifest_sha256':marker['manifest_sha256'],'artifact_id':ri['artifact_id'],**extra_intent}
            or not isinstance(receipt,dict) or set(receipt)!={'operation_id','snapshot_id','source_sha','manifest_sha256','databases','scope','cleanup'}|set(extra_receipt)
            or any(receipt.get(k)!=v for k,v in extra_receipt.items())
            or receipt.get('operation_id')!=binding['operation_id'] or receipt.get('snapshot_id')!=binding['snapshot_id']
            or receipt.get('source_sha')!=binding['source_sha'] or receipt.get('manifest_sha256')!=marker['manifest_sha256']
            or receipt.get('scope')!='selected-cohort-roles-catalog-and-ledger-only' or receipt.get('cleanup')!='verified'
            or set(receipt.get('databases',{}))!=set(binding['databases'])
            or version not in (26,32)
            or receipt['databases'].get('auth_db')!={'mode':'restore-only','from':version,'to':version}): raise ValueError()
    resources=row.get('cohort_restore_resources')
    local=copy.deepcopy({k.removeprefix('cohort-restore-resource-'):v for k,v in p.journal.state['steps'].items()
                         if k.startswith('cohort-restore-resource-')})
    if (not isinstance(resources,dict) or not resources or set(resources)-{'postgres','migrate'} or local!=resources
            or any(r.get('status')!='removed' or r.get('operation_id')!=binding['operation_id']
                   or r.get('invocation_nonce')!=p.invocation for r in resources.values())): raise ValueError()
    inputs=binding['restore_inputs']
    restore._source(binding,binding['databases'],metadata,inputs['postgres_image'],inputs['migrate_image'])
    if (p.fence.ledger.read()!=(row,revision) or readback._snapshot(phase,row)!=snapshot
            or p.journal.state['steps'].get('cohort-readback')!=rd or p.journal.state['steps'].get('cohort-restore')!=rs
            or {k.removeprefix('cohort-restore-resource-'):v for k,v in p.journal.state['steps'].items()
                if k.startswith('cohort-restore-resource-')}!=local or original_metadata!=metadata): raise ValueError()
    return binding,{'manifest_sha256':marker['manifest_sha256'],'custody_sha256':_digest(rr),
                    'restore_sha256':_digest(receipt),'metadata_sha256':_digest(metadata)}


def c2_sources(phase, metadata):
    return verified_sources(phase,metadata,32)


def init_c1(phase, cut, source_root, actor, token, metadata):
    """Capture and reserve an authenticated C1 plan; creates no Kubernetes objects."""
    p=phase.phases; started=False
    try:
        metadata=snapshot_protected(metadata)
        _cut_identity(phase,cut); drain=lambda:cut.verify_drained()
        _package_authority(phase); readback._guard(phase,drain)
        binding,provenance=verified_sources(phase,metadata,26); _bind_cut_scope(cut,binding)
        operation= binding['operation_id']; operation_name(operation)
        if not operation.startswith('media-c1-'): raise ValueError()
        sources,catalog=source_catalog(source_root,binding['source_sha'])
        if catalog!=metadata['migrations']['auth_db']['files']: raise ValueError()
        profile=candidate_blob(source_root,binding['source_sha'],
            'src/backend/auth/src/main/resources/roleidentityauthority/migration-history-linked-oauth-v1.json')
        expected=expected_history(catalog,profile,26)
        candidate=eligibility.observe_candidate(phase,drain,source_root,actor,token)
        auth=binding['sources']['auth_db']; databases=binding['databases']
        scope={'engine':auth['engine'],'server_start':auth['server_start'],'databases':databases,
               'database_oids':{db:binding['sources'][db]['database_oid'] for db in databases}}
        observed=probe.inspect(scope,lambda:readback._guard(phase,drain))
        projected=[{k:r[k] for k in ('version','script','checksum','success')} for r in observed['history']]
        if (not matches_expected_history(observed['history'],expected) or observed['sessions'] or observed['prepared']!=0
                or projected!=metadata['catalog']['auth_db']['history']): raise ValueError()
        row,_=p.fence.ledger.read(); steps=p.journal.state['steps']
        clone=steps['cohort-auth-flyway-private']
        _bind_clone_provenance(clone,provenance)
        validate_clone_receipt(clone,steps['cohort-auth-flyway'],row.get('cohort_auth_flyway'),operation,
                               binding['snapshot_id'],binding['source_sha'],catalog,observed['history'])
        before_data=_c1_data_snapshot(auth['engine'],clone,True,lambda:readback._guard(phase,drain))
        tool=copy.deepcopy(p.intent['auth_live_tool'])
        readback._guard(phase,drain)
        if flyway_platform(runtime.QUALIFIED_TOOLS['flyway'].rsplit('@',1)[1])!=tool['flyway_platform_digest']: raise ValueError()
        readback._guard(phase,drain)
        node=observe_node(p.client,auth['engine'],tool,lambda:readback._guard(phase,drain))
        credential=probe.verify_login(p.client,{'engine':auth['engine'],'database_oid':auth['database_oid'],
            'server_start':auth['server_start']},lambda:readback._guard(phase,drain))
        if (verified_sources(phase,metadata,26)!=(binding,provenance)
                or probe.inspect(scope,lambda:readback._guard(phase,drain))!=observed): raise ValueError()
        candidate_again=eligibility.observe_candidate(phase,drain,source_root,actor,token)
        if not _same_candidate(candidate,candidate_again): raise ValueError()
        candidate=candidate_again
        cm,job=c1_objects(operation,sources,tool['node_name'])
        def resource(obj):
            return {'state':'absent','name':obj['metadata']['name'],'spec_sha256':_digest(obj),'uid':None,'resourceVersion':None}
        inputs={'candidate':candidate,'metadata':copy.deepcopy(metadata),'history':copy.deepcopy(observed['history']),
                'provenance':provenance,'node':node,'credential':credential,'before_data':before_data,
                'clone_receipt_sha256':_digest(clone)}
        record={'status':'active','operation_id':operation,'snapshot_id':binding['snapshot_id'],
                'source_sha':binding['source_sha'],'nonce':secrets.token_hex(16),'input_sha256':_digest(inputs),
                'terminal_sha256':None,'configmap':resource(cm),'job':resource(job)}
        row,revision=p.fence.ledger.read()
        if ('auth_live_c1' in row or 'auth-live-c1' in steps or 'auth-live-c1-inputs' in steps): raise ValueError()
        started=True
        steps['auth-live-c1-inputs']=copy.deepcopy(inputs); steps['auth-live-c1']=copy.deepcopy(record); p.journal.save()
        current,rev=p.fence.ledger.read()
        if current!=row or rev!=revision: raise ValueError()
        current['auth_live_c1']=copy.deepcopy(record); current['step']='auth-live-c1-init'
        p.fence.ledger.write(revision,current)
        readback._guard(phase,drain); verify_writer_successor(phase,source_root)
        return record
    except BaseException:
        if started: readback._poison(phase)
        raise PhaseError('C1 Auth plan unavailable; retain fence if outcome is uncertain') from None


def no_op_c2(phase, cut, source_root, actor, token, metadata):
    """Record a fresh clean32 C2 observation. No Kubernetes objects are created."""
    p=phase.phases; started=False
    try:
        metadata=snapshot_protected(metadata)
        _cut_identity(phase,cut); drain=lambda:cut.verify_drained()
        _package_authority(phase)
        readback._guard(phase,drain)
        binding,provenance=c2_sources(phase,metadata); _bind_cut_scope(cut,binding)
        operation_name(binding['operation_id'])
        if not binding['operation_id'].startswith('media-c2-'): raise ValueError()
        _,catalog=source_catalog(source_root,binding['source_sha'])
        if catalog!=metadata['migrations']['auth_db']['files']: raise ValueError()
        profile=candidate_blob(source_root,binding['source_sha'],
            'src/backend/auth/src/main/resources/roleidentityauthority/migration-history-linked-oauth-v1.json')
        expected=expected_history(catalog,profile,32)
        candidate=eligibility.observe_candidate(phase,drain,source_root,actor,token)
        sources=binding['sources']; databases=binding['databases']; auth=sources['auth_db']
        scope={'engine':auth['engine'],'server_start':auth['server_start'],'databases':databases,
               'database_oids':{db:sources[db]['database_oid'] for db in databases}}
        observed=probe.inspect(scope,lambda:readback._guard(phase,drain))
        history=observed['history']
        projected=[{k:r[k] for k in ('version','script','checksum','success')} for r in history]
        if (not matches_expected_history(history,expected) or projected!=metadata['catalog']['auth_db']['history']
                or observed['sessions'] or observed['prepared']!=0): raise ValueError()
        readback._guard(phase,drain)
        if c2_sources(phase,metadata)!=(binding,provenance): raise ValueError()
        again=probe.inspect(scope,lambda:readback._guard(phase,drain))
        if again!=observed: raise ValueError()
        candidate_again=eligibility.observe_candidate(phase,drain,source_root,actor,token)
        if not _same_candidate(candidate,candidate_again): raise ValueError()
        candidate=candidate_again
        row,rev=p.fence.ledger.read()
        if ('auth_live_c2' in row or 'auth-live-c2' in p.journal.state['steps']
                or 'auth-live-c2-private' in p.journal.state['steps']): raise ValueError()
        receipt={'status':'no-op','operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                 'source_sha':binding['source_sha'],'candidate':candidate,'provenance':provenance,
                 'source_catalog':catalog,'history':history,'engine':scope,'metadata':metadata}
        record={'status':'done','nonce':secrets.token_hex(16),'operation_id':binding['operation_id'],
                'snapshot_id':binding['snapshot_id'],'receipt_sha256':_digest(receipt)}
        started=True
        p.journal.state['steps']['auth-live-c2-private']=copy.deepcopy(receipt)
        p.journal.state['steps']['auth-live-c2']=copy.deepcopy(record)
        p.journal.save()
        current,current_rev=p.fence.ledger.read()
        if current!=row or current_rev!=rev: raise ValueError()
        current['auth_live_c2']=copy.deepcopy(record); current['step']='auth-live-c2-done'
        p.fence.ledger.write(rev,current)
        readback._guard(phase,drain)
        current,_=p.fence.ledger.read()
        if (current.get('auth_live_c2')!=record or p.journal.state['steps'].get('auth-live-c2')!=record
                or _digest(p.journal.state['steps'].get('auth-live-c2-private'))!=record['receipt_sha256']
                or c2_sources(phase,metadata)!=(binding,provenance)
                or probe.inspect(scope,lambda:readback._guard(phase,drain))!=observed): raise ValueError()
        return receipt
    except BaseException:
        if started: readback._poison(phase)
        raise PhaseError('C2 Auth no-op unavailable; retain fence if outcome is uncertain') from None


def verify_completed(phase, cut, source_root):
    """Read protected terminal evidence without requiring writers to stay drained."""
    try:
        _cut_identity(phase,cut); p=phase.phases
        if phase._guard() is False or cut._guard() is False: raise ValueError()
        _package_authority(phase)
        context=copy.deepcopy(p.fence.context); intent=copy.deepcopy(p.intent)
        steps=copy.deepcopy(p.journal.state['steps']); cut_steps=copy.deepcopy(cut.journal.state['steps'])
        phase_binding=copy.deepcopy(phase.binding); row,_=p.fence.ledger.read()
        operation=context['operation_id']; operation_name(operation)
        bound_snapshot=None
        if operation.startswith('media-c1-'):
            inputs=steps['auth-live-c1-inputs']
            metadata=snapshot_protected(inputs['metadata'])
            verified_sources(phase,metadata,26)
            terminal=verify_writer_terminal(phase,source_root); cleanup=verify_writer_cleanup(phase,source_root)
            record=steps['auth-live-c1']; candidates=[b for b in cut.migration_history
                if b.get('kind')=='auth' and b.get('phase') is phase and b.get('root')==source_root
                and b.get('terminal')==record['terminal_sha256']]
            if len(candidates)!=1: raise ValueError()
            binding=candidates[0]
            bound_snapshot=copy.deepcopy({k:v for k,v in binding.items() if k!='phase'})
            import writer_cut_migration
            writer_cut_migration.verify_cleaned(cut,binding)
            step='writer-cut-migration-cleanup-'+record['nonce']
            entry=cut.journal.state['steps'][step]
            details={'plan_sha256':writer_cut.digest(binding['plan']),'job_uid':cleanup['job']['uid'],
                     'config_uid':cleanup['configmap']['uid'],'terminal_sha256':record['terminal_sha256']}
            if (entry.get('status')!='done' or entry.get('details')!=details
                    or entry.get('receipt')!={'operation_id':operation,**details}): raise ValueError()
            candidate=inputs['candidate']; receipt_sha=record['terminal_sha256']; snapshot=record['snapshot_id']
            status='migrated'
        else:
            metadata=snapshot_protected(steps['auth-live-c2-private']['metadata'])
            binding,provenance=c2_sources(phase,metadata); _bind_cut_scope(cut,binding)
            record=steps['auth-live-c2']; receipt=steps['auth-live-c2-private']
            if (not isinstance(record,dict) or set(record)!={'status','nonce','operation_id','snapshot_id','receipt_sha256'}
                    or record!=row.get('auth_live_c2') or record['status']!='done'
                    or record['operation_id']!=operation or record['snapshot_id']!=binding['snapshot_id']
                    or not isinstance(record['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',record['nonce'])
                    or record['receipt_sha256']!=_digest(receipt) or receipt.get('status')!='no-op'
                    or receipt.get('operation_id')!=operation or receipt.get('snapshot_id')!=binding['snapshot_id']
                    or receipt.get('source_sha')!=context['commit'] or receipt.get('provenance')!=provenance): raise ValueError()
            _,catalog=source_catalog(source_root,context['commit'])
            profile=candidate_blob(source_root,context['commit'],
                'src/backend/auth/src/main/resources/roleidentityauthority/migration-history-linked-oauth-v1.json')
            auth=binding['sources']['auth_db']; databases=binding['databases']
            scope={'engine':auth['engine'],'server_start':auth['server_start'],'databases':databases,
                   'database_oids':{db:binding['sources'][db]['database_oid'] for db in databases}}
            projected=[{k:r[k] for k in ('version','script','checksum','success')} for r in receipt['history']]
            if (receipt.get('source_catalog')!=catalog or catalog!=metadata['migrations']['auth_db']['files']
                    or receipt.get('engine')!=scope or projected!=metadata['catalog']['auth_db']['history']
                    or not matches_expected_history(receipt['history'],expected_history(catalog,profile,32))): raise ValueError()
            candidate=receipt['candidate']; receipt_sha=record['receipt_sha256']; snapshot=record['snapshot_id']
            status='no-op'
        plan=intent['auth_live_candidate']; target=eligibility.candidate_target(plan['deployment']['spec'],context['images']['voice-auth/auth'])
        if (not isinstance(candidate,dict) or candidate.get('source_sha')!=context['commit']
                or candidate.get('image')!=context['images']['voice-auth/auth']
                or candidate.get('auth_tree')!=eligibility._source_tree(source_root,context['commit'])
                or candidate.get('target_spec_sha256')!=_digest(target)
                or candidate.get('platform_digest')!=plan['platform_digest']
                or candidate.get('deployment',{}).get('uid')!=plan['deployment']['uid']
                or candidate.get('deployment',{}).get('generation')!=plan['deployment']['generation']
                or candidate.get('policy')!={'flyway':'disabled','persistence':'jdbc','selector':'absent'}
                or candidate.get('route') not in ({'host':h,'database':'auth_db','role':'voice'} for h in
                    ('voice-postgres','voice-postgres.voice-dev','voice-postgres.voice-dev.svc','voice-postgres.voice-dev.svc.cluster.local'))):
            raise ValueError()
        if phase._guard() is False or cut._guard() is False: raise ValueError()
        final_row,_=p.fence.ledger.read()
        if (final_row!=row or p.journal.state['steps']!=steps or cut.journal.state['steps']!=cut_steps
                or p.fence.context!=context or p.intent!=intent or p.journal.intent!=intent
                or p.journal.state.get('intent')!=intent or phase.binding!=phase_binding): raise ValueError()
        if bound_snapshot is not None:
            current=[b for b in cut.migration_history if b.get('phase') is phase and
                     {k:v for k,v in b.items() if k!='phase'}==bound_snapshot]
            if len(current)!=1: raise ValueError()
        return {'status':status,'operation_id':operation,'snapshot_id':snapshot,'source_sha':context['commit'],
                'receipt_sha256':receipt_sha,'candidate':{k:copy.deepcopy(candidate[k]) for k in
                    ('image','platform_digest','target_spec_sha256','auth_tree','policy','route')}}
    except BaseException: raise PhaseError('completed Auth authority unavailable') from None


def operation_name(operation):
    if not isinstance(operation,str) or not re.fullmatch(r'media-c[12]-[0-9a-f]{32}',operation):
        raise ValueError('invalid cohort operation')
    return 'au-' + hashlib.sha256(operation.encode('ascii')).hexdigest()[:28]


def classify_history(rows):
    if not isinstance(rows,list) or not rows:
        raise ValueError('missing Flyway history')
    versions=[]
    for row in rows:
        if (not isinstance(row,dict) or row.get('type') != 'SQL' or row.get('success') is not True
                or not isinstance(row.get('version'),str) or not re.fullmatch(r'[1-9][0-9]*',row['version'])
                or not isinstance(row.get('script'),str) or not row['script']
                or type(row.get('checksum')) is not int):
            raise ValueError('invalid Flyway history')
        versions.append(int(row['version']))
    if versions != sorted(set(versions)):
        raise ValueError('Flyway history reordered or duplicated')
    if versions[-1] == 26 and all(v<=26 for v in versions): return 26
    if versions[-3:] == [28,31,32] and len(versions)>3 and versions[-4] == 26: return 32
    raise ValueError('unsupported Flyway history')


def verify_transition(before, after, clone_after):
    if classify_history(before) != 26 or classify_history(after) != 32 or classify_history(clone_after) != 32:
        raise ValueError('unsupported migration transition')
    if after[:len(before)] != before or len(after) != len(before)+3:
        raise ValueError('prior Flyway history changed')
    def migration_identity(row):
        return {k:row[k] for k in ('version','script','checksum','type','success')}
    if [migration_identity(r) for r in after] != [migration_identity(r) for r in clone_after]:
        raise ValueError('actual Flyway history differs from qualified clone')


def matches_expected_history(actual, expected):
    keys=('version','script','checksum','type','success')
    try:
        return [{k:row[k] for k in keys} for row in actual] == expected
    except (TypeError,KeyError):
        return False


def validate_clone_receipt(receipt, local_record, ledger_record, operation, snapshot, source, catalog, actual_history):
    """Verify private evidence against its protected record and the actual prehistory."""
    if not isinstance(receipt,dict) or not isinstance(local_record,dict) or local_record != ledger_record:
        raise ValueError('clone record unavailable')
    digest=hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
    record=local_record
    if (record.get('status')!='done' or record.get('stage')!='done' or record.get('receipt_sha256')!=digest
            or receipt.get('protocol')!='voice-auth-flyway-cohort-clone-v1' or receipt.get('target')!=32
            or receipt.get('scope')!='isolated-auth-clone-only' or receipt.get('cleanup')!='confirmed'
            or receipt.get('source_catalog')!=catalog or receipt.get('operation_id')!=operation
            or receipt.get('snapshot_id')!=snapshot or receipt.get('source_sha')!=source
            or any(record.get(k)!=receipt.get(k) for k in ('operation_id','snapshot_id','source_sha',
                'plan_sha256','manifest_sha256','custody_sha256','restore_sha256','metadata_sha256'))
            or not isinstance(record.get('resources'),list) or len(record['resources'])!=4
            or any(not isinstance(r,dict) or r.get('state')!='removed' for r in record['resources'])):
        raise ValueError('clone receipt differs from authority')
    clone_before=receipt.get('before_history'); clone_after=receipt.get('after_history')
    if actual_history!=clone_before: raise ValueError('clone source history differs')
    verify_transition(actual_history,clone_after,clone_after)
    return clone_after


def expected_history(source_catalog, profile_bytes, target):
    """Bind reviewed Flyway checksum profile to every candidate SQL blob."""
    if target not in (26,32): raise ValueError('unsupported target')
    profile=json.loads(profile_bytes)
    if not isinstance(profile,dict) or profile.get('profile')!='linked-oauth-transport-v1':
        raise ValueError('unsupported Flyway profile')
    rows=profile.get('flyway')
    if not isinstance(rows,list) or not rows: raise ValueError('missing Flyway profile')
    expected=[]; names=set()
    for item in rows:
        if (not isinstance(item,dict) or set(item)!={'version','script','checksum','sha256'}
                or not isinstance(item['version'],str) or not re.fullmatch(r'[1-9][0-9]*',item['version'])
                or type(item['checksum']) is not int or item['script'] in names
                or source_catalog.get(item['script'])!=item['sha256']):
            raise ValueError('source/checked Flyway profile differs')
        names.add(item['script'])
        if int(item['version'])<=target:
            expected.append({'version':item['version'],'script':item['script'],
                             'checksum':item['checksum'],'type':'SQL','success':True})
    if names!=set(source_catalog) or classify_history(expected)!=target:
        raise ValueError('candidate migration inventory unsupported')
    return expected
