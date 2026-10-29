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
import cohort_readback as readback
import story_media_backup_cohort_restore as restore
import auth_live_migration_phase_eligibility as eligibility
import auth_live_migration_phase_probe as probe
from auth_flyway_upgrade_qualification import source_catalog, candidate_blob
import auth_user_adoption_runtime as runtime
import cohort_backup_phases
import media_backup_phase
import media_operation_journal
import resource_fencing
import full_cohort_package_phase as package_phase
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


def verify_writer_successor(phase, source_root):
    """Reconstruct the planned C1 objects from protected state; never create them."""
    try:
        p=phase.phases; _phase_identity(phase); _package_authority(phase)
        if phase._guard() is False or p.journal.intent!=p.intent: raise ValueError()
        steps=p.journal.state['steps']; record=copy.deepcopy(steps['auth-live-c1'])
        inputs=copy.deepcopy(steps['auth-live-c1-inputs']); row,revision=p.fence.ledger.read()
        if (not isinstance(record,dict) or set(record)!={'status','operation_id','snapshot_id','source_sha','nonce','input_sha256','terminal_sha256','configmap','job'}
                or record!=row.get('auth_live_c1') or record['status'] not in ('active','done')
                or not isinstance(inputs,dict) or set(inputs)!={'candidate','metadata','history','provenance','node','credential','clone_receipt_sha256'}
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
                or not isinstance(inputs['credential'],dict) or set(inputs['credential'])!={'uid','resourceVersion'}
                or any(not isinstance(v,str) or not v for v in inputs['credential'].values())): raise ValueError()
        sources,catalog=source_catalog(source_root,record['source_sha'])
        if catalog!=inputs['metadata']['migrations']['auth_db']['files']: raise ValueError()
        clone=steps['cohort-auth-flyway-private']; clone_record=steps['cohort-auth-flyway']
        if _digest(clone)!=inputs['clone_receipt_sha256']: raise ValueError()
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
                'pod_uid','pod_ip','node_uid','image_id','before_history','after_history','clone_receipt_sha256','sessions'}
                or record['terminal_sha256']!=_digest(terminal) or record['job']['state'] not in ('owned','removing','removed')
                or terminal['operation_id']!=record['operation_id'] or terminal['snapshot_id']!=record['snapshot_id']
                or terminal['source_sha']!=record['source_sha'] or terminal['job_uid']!=record['job']['uid']
                or not isinstance(terminal['pod_uid'],str) or not terminal['pod_uid']
                or terminal['node_uid']!=successor['execution']['node_uid']
                or terminal['image_id']!=successor['execution']['platform_digest']
                or terminal['sessions']!=[] or terminal['before_history']!=inputs['history']
                or terminal['clone_receipt_sha256']!=_digest(clone)): raise ValueError()
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


def verified_sources(phase, metadata, version):
    """Verify current cohort custody and restore records from the common fence."""
    p=phase.phases; row,_=p.fence.ledger.read()
    binding,planned,marker,_=readback._snapshot(phase,row)
    rd=p.journal.state['steps']['cohort-readback']; rs=p.journal.state['steps']['cohort-restore']
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
    if (set(intent)!={'status','operation_id','snapshot_id','invocation_nonce','manifest_sha256','artifact_id'}
            or intent!={'status':'pending','operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                       'invocation_nonce':p.invocation,'manifest_sha256':marker['manifest_sha256'],'artifact_id':ri['artifact_id']}
            or not isinstance(receipt,dict) or set(receipt)!={'operation_id','snapshot_id','source_sha','manifest_sha256','databases','scope','cleanup'}
            or receipt.get('operation_id')!=binding['operation_id'] or receipt.get('snapshot_id')!=binding['snapshot_id']
            or receipt.get('source_sha')!=binding['source_sha'] or receipt.get('manifest_sha256')!=marker['manifest_sha256']
            or receipt.get('scope')!='selected-cohort-roles-catalog-and-ledger-only' or receipt.get('cleanup')!='verified'
            or set(receipt.get('databases',{}))!=set(binding['databases'])
            or version not in (26,32)
            or receipt['databases'].get('auth_db')!={'mode':'restore-only','from':version,'to':version}): raise ValueError()
    resources=row.get('cohort_restore_resources')
    local={k.removeprefix('cohort-restore-resource-'):v for k,v in p.journal.state['steps'].items()
           if k.startswith('cohort-restore-resource-')}
    if (not isinstance(resources,dict) or not resources or set(resources)-{'postgres','migrate'} or local!=resources
            or any(r.get('status')!='removed' or r.get('operation_id')!=binding['operation_id']
                   or r.get('invocation_nonce')!=p.invocation for r in resources.values())): raise ValueError()
    inputs=binding['restore_inputs']
    restore._source(binding,binding['databases'],metadata,inputs['postgres_image'],inputs['migrate_image'])
    return binding,{'manifest_sha256':marker['manifest_sha256'],'custody_sha256':_digest(rr),
                    'restore_sha256':_digest(receipt),'metadata_sha256':_digest(metadata)}


def c2_sources(phase, metadata):
    return verified_sources(phase,metadata,32)


def init_c1(phase, drain, source_root, actor, token, metadata):
    """Capture and reserve an authenticated C1 plan; creates no Kubernetes objects."""
    p=phase.phases; started=False
    try:
        _phase_identity(phase); _package_authority(phase); readback._guard(phase,drain)
        binding,provenance=verified_sources(phase,metadata,26)
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
        validate_clone_receipt(clone,steps['cohort-auth-flyway'],row.get('cohort_auth_flyway'),operation,
                               binding['snapshot_id'],binding['source_sha'],catalog,observed['history'])
        tool=copy.deepcopy(p.intent['auth_live_tool'])
        readback._guard(phase,drain)
        if flyway_platform(runtime.QUALIFIED_TOOLS['flyway'].rsplit('@',1)[1])!=tool['flyway_platform_digest']: raise ValueError()
        readback._guard(phase,drain)
        node=observe_node(p.client,auth['engine'],tool,lambda:readback._guard(phase,drain))
        credential=probe.verify_login(p.client,{'engine':auth['engine'],'database_oid':auth['database_oid'],
            'server_start':auth['server_start']},lambda:readback._guard(phase,drain))
        if (verified_sources(phase,metadata)!=(binding,provenance)
                or probe.inspect(scope,lambda:readback._guard(phase,drain))!=observed): raise ValueError()
        candidate_again=eligibility.observe_candidate(phase,drain,source_root,actor,token)
        if not _same_candidate(candidate,candidate_again): raise ValueError()
        candidate=candidate_again
        cm,job=c1_objects(operation,sources,tool['node_name'])
        def resource(obj):
            return {'state':'absent','name':obj['metadata']['name'],'spec_sha256':_digest(obj),'uid':None,'resourceVersion':None}
        inputs={'candidate':candidate,'metadata':copy.deepcopy(metadata),'history':copy.deepcopy(observed['history']),
                'provenance':provenance,'node':node,'credential':credential,'clone_receipt_sha256':_digest(clone)}
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


def no_op_c2(phase, drain, source_root, actor, token, metadata):
    """Record a fresh clean32 C2 observation. No Kubernetes objects are created."""
    p=phase.phases; started=False
    try:
        _phase_identity(phase); _package_authority(phase)
        readback._guard(phase,drain)
        binding,provenance=c2_sources(phase,metadata)
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
                 'source_catalog':catalog,'history':history,'engine':scope}
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
