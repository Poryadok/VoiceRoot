"""Independent finish-only capability; never grants migration or application.

All evidence below is written by the locked root proof producer. Request JSON
supplies only an exact reference, not a replacement proof or restart ledger.
"""
import copy
from datetime import datetime,timedelta,timezone
from pathlib import Path
import re
import guard
import expired_recovery as recovery
import expired_transport as transport
import preserved_upload as upload
from stage_runtime import HUB


def stable(state):
    return {key:copy.deepcopy(state.get(key)) for key in ('operation','target','input_target',
        'authorization','cut','cipher_binding','execution_authority','source_authority',
        'nats_contract_expires_at','nats_migration','migration_jobs','migration_completed_metadata',
        'space_postcondition','active_contract','renderer_authority','expired_recovery_authority')}


def issue(state,execution,observation,readbacks,*,now):
    """Called only after root proved application, closure and both full readbacks."""
    try:
        receipt=state['authorization'];target=state['target']
        if (state['operation']!=upload.OPERATION or target['tag']!=upload.ORIGINAL_TARGET
            or state['phase'] not in ('PAUSED_APPLY','POST_APPLY_PROOF','RESTART')
            or state['status'] not in ('WAITING','BLOCKED') or state['fence_status']!='VERIFIED'
            or receipt['operation']!=state['operation'] or receipt['target']!=target
            or state['nats_migration'].get('verified') is not True
            or receipt.get('expired_recovery_authority_sha256')!=upload.digest(state['expired_recovery_authority'])
            or set(execution)!=upload.EXECUTION_FIELDS or execution['repository']!='Poryadok/VoiceRoot'
            or execution['workflow_id']!=263689731 or execution['event']!='workflow_dispatch'
            or execution['path']!='.github/workflows/staging-deploy.yml'
            or type(execution['dispatcher_run_id']) is not int or execution['dispatcher_run_id']<=37917461677
            or type(execution['run_attempt']) is not int or execution['run_attempt']<=0
            or not re.fullmatch('[a-f0-9]{40}',execution['head_sha'])
            or not re.fullmatch('[a-f0-9]{64}',execution['nonce'])
            or execution['nonce'] in (upload.ORIGINAL_NONCE,state['expired_recovery_authority']['execution']['nonce'])):
            recovery.reject('finish_applied_identity_invalid')
        fields={'schema','operation','started_at','completed_at','checkpoint_sha256',
            'applied_receipt_sha256','post_apply_cut_sha256','selected_inventory_sha256',
            'census_sha256','native_messages_sha256','applied_workloads','original_cipher_binding',
            'observation_cipher_binding','artifact_bindings','source_admission_sha256'}
        postseal=observation.get('schema')=='voice-expired-finish-postseal-proof-v1'
        if postseal:fields.add('postseal_binding')
        if (set(observation)!=fields or observation['schema'] not in ('voice-expired-finish-proof-v1','voice-expired-finish-postseal-proof-v1')
            or observation['operation']!=state['operation']
            or observation['checkpoint_sha256']!=upload.digest(state)
            or observation['applied_receipt_sha256']!=upload.digest(receipt)
            or observation['post_apply_cut_sha256']!=upload.digest(state['nats_migration']['cut'])
            or observation['original_cipher_binding']!=state['cipher_binding']
            or any(not isinstance(observation[key],str) or not re.fullmatch('[a-f0-9]{64}',observation[key])
                for key in ('selected_inventory_sha256','census_sha256','native_messages_sha256','source_admission_sha256'))):
            recovery.reject('finish_proof_binding_invalid')
        if postseal:
            bound=observation['postseal_binding']
            if (type(bound) is not dict or set(bound)!={'observation','current_cut','postseal_members','source_admission'}
                or bound['observation']['schema']!='voice-expired-finish-current-observation-v1'
                or bound['observation']['operation']!=state['operation']
                or bound['current_cut']['postseal_observation_sha256']!=upload.digest(bound['observation'])
                or bound['current_cut']['manifest']!=bound['observation']['manifest']
                or bound['current_cut']['manifest_sha256']!=upload.digest(bound['current_cut']['manifest'])
                or bound['current_cut']['census_sha256']!=upload.digest(bound['current_cut']['census'])
                or bound['postseal_members']!=observation['observation_cipher_binding'].get('postseal_members')
                or upload.digest(bound['observation']['native_messages'])!=observation['native_messages_sha256']
                or bound['observation']['selected_inventory_sha256']!=observation['selected_inventory_sha256']):
                recovery.reject('finish_postseal_proof_invalid')
        workloads=observation['applied_workloads']
        if not isinstance(workloads,dict) or set(workloads)!=set(state['context']['snapshots']) or HUB not in workloads:
            recovery.reject('finish_workload_inventory_invalid')
        for name,row in workloads.items():
            captured=state['context']['snapshots'][name]
            wanted=target['template_hashes'].get(name,upload.digest(captured['spec']['template']))
            if (set(row)!={'uid','template_sha256','replicas','generation','pause_proof_sha256'}
                or row['uid']!=captured['metadata']['uid'] or row['template_sha256']!=wanted
                or row['replicas']!=0 or type(row['generation']) is not int or row['generation']<=0
                or not re.fullmatch('[a-f0-9]{64}',row['pause_proof_sha256'])):
                recovery.reject('finish_owned_pause_invalid')
        if set(readbacks)!={'original','observation'}:recovery.reject('finish_readbacks_missing')
        ids=[]
        for role in ('original','observation'):
            readback=readbacks[role];binding=observation['artifact_bindings'][role]
            cipher=observation[role+'_cipher_binding']
            if (readback.get('schema')!='voice-nats-custody-v1' or readback.get('verified') is not True
                or readback.get('destination')!='github:Poryadok/VoiceRoot'
                or binding['run_id']!=execution['dispatcher_run_id'] or binding['head_sha']!=execution['head_sha']
                or any(binding[key]!=cipher[key] for key in ('operation','cipher_sha256','cipher_bytes'))
                or any(readback.get(key)!=binding[key] for key in
                    ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes'))
                or type(readback.get('artifact_id')) is not int or readback['artifact_id']<=0):
                recovery.reject('finish_readback_unbound')
            ids.append(readback['artifact_id'])
        if len(set(ids))!=2:recovery.reject('finish_readbacks_not_distinct')
        start=upload.timestamp(observation['started_at']);complete=upload.timestamp(observation['completed_at'])
        deadline=start+timedelta(seconds=600)
        if not upload.timestamp(receipt['expires_at'])<=start<=complete<=now<deadline:
            recovery.reject('finish_proof_time_invalid')
        return {'schema':'voice-expired-finish-authority-v1','operation':state['operation'],
            'purpose':'finish-only','execution':copy.deepcopy(execution),
            'snapshot_sha256':upload.digest(state),'stable_sha256':upload.digest(stable(state)),
            'observation_sha256':upload.digest(observation),
            'readbacks_sha256':upload.digest(readbacks),'issued_at':now.isoformat(),'expires_at':deadline.isoformat()}
    except (KeyError,TypeError,AttributeError):recovery.reject('finish_evidence_shape_invalid')


def commit_authority(base,state,slot,execution,*,clock=None,stage=None,verify_source=None):
    """Mint resume-only permission from immutable ROOT producer and readbacks.

    No request may supply a substitute census, pause obligation or proof hash.
    The original checkpoint and expired apply receipt remain historical facts.
    """
    import hashlib
    clock=clock or (lambda:datetime.now(timezone.utc));base=Path(base);slot=Path(slot)
    if (slot.parent!=base/'expired-finish-proofs' or slot.name!=execution['nonce']
        or base.name!='rollout-'+upload.OPERATION):recovery.reject('finish_commit_path_invalid')
    checkpoint=upload.private_read(base/'checkpoint.json')
    snapshot_raw=upload.private_read(slot/'checkpoint.json')
    snapshot=upload.private_json(slot/'checkpoint.json',128<<20)
    if snapshot!=state or upload.private_json(base/'checkpoint.json',128<<20)!=snapshot:
        recovery.reject('finish_commit_checkpoint_changed')
    names=('producer.json','upload.json','rollout-before-manifest.json','pause.json','verified-pause.json',
        'paused-checkpoint.json','source.json',
        'original-readback.json','observation-readback.json')
    raw={name:upload.private_read(slot/name) for name in names}
    rows={name:upload.private_json(slot/name,128<<20) for name in names}
    producer=rows['producer.json'];record=rows['upload.json'];observed=rows['rollout-before-manifest.json']
    pause=rows['verified-pause.json'];source=rows['source.json'];paused=rows['paused-checkpoint.json']
    try:
        if (producer['schema']!='voice-expired-finish-observation-producer-v1'
            or producer['operation']!=state['operation'] or record['execution']!=execution
            or source['execution']!=execution
            or producer['original_checkpoint_sha256']!=hashlib.sha256(checkpoint).hexdigest()
            or record['checkpoint_sha256']!=producer['original_checkpoint_sha256']
            or record['producer_sha256']!=upload.digest(producer)
            or producer['observation_sha256']!=upload.digest(observed)
            or producer['pause_sha256']!=upload.digest(pause)
            or producer['paused_record_sha256']!=upload.digest(rows['pause.json'])
            or producer['paused_checkpoint_sha256']!=upload.digest(paused)
            or rows['pause.json']['checkpoint_sha256']!=upload.digest(paused)
            or {key:value for key,value in pause.items() if key!='checkpoint_sha256'}!=
                {key:value for key,value in rows['pause.json'].items() if key!='checkpoint_sha256'}
            or stable(paused)!=stable(state) or state['events'][:len(paused['events'])]!=paused['events']
            or producer['source_sha256']!=upload.digest(source)
            or record['original_cipher_binding']!=state['cipher_binding']
            or record['observation_cipher_binding']!=producer['cipher']
            or observed['schema']!='voice-expired-finish-current-observation-v1'
            or observed['operation']!=state['operation']
            or observed['original_cut_sha256']!=upload.digest(state['cut'])
            or observed['post_apply_cut_sha256']!=upload.digest(state['nats_migration']['cut'])
            or pause['schema']!='voice-expired-finish-owned-pause-v1'
            or pause['operation']!=state['operation'] or pause['checkpoint_sha256']!=upload.digest(state)):
            recovery.reject('finish_commit_producer_changed')
        readbacks={role:rows[role+'-readback.json'] for role in ('original','observation')}
        proof={'schema':'voice-expired-finish-proof-v1','operation':state['operation'],
            'started_at':producer['started_at'],'completed_at':clock().isoformat(),
            'checkpoint_sha256':upload.digest(state),'applied_receipt_sha256':upload.digest(state['authorization']),
            'post_apply_cut_sha256':upload.digest(state['nats_migration']['cut']),
            'selected_inventory_sha256':observed['selected_inventory_sha256'],
            'census_sha256':upload.digest(observed['census']),'native_messages_sha256':upload.digest(observed['native_messages']),
            'applied_workloads':pause['workloads'],'original_cipher_binding':record['original_cipher_binding'],
            'observation_cipher_binding':record['observation_cipher_binding'],
            'artifact_bindings':record['artifact_bindings'],'source_admission_sha256':upload.digest(source)}
        if 'postseal' in observed:
            recovery.reject('finish_legacy_postseal_unsupported')
        authority=issue(state,execution,proof,readbacks,now=clock())
    except (KeyError,TypeError,AttributeError):recovery.reject('finish_commit_shape_invalid')
    if (upload.private_read(base/'checkpoint.json')!=checkpoint
        or upload.private_read(slot/'checkpoint.json')!=snapshot_raw
        or any(upload.private_read(slot/name)!=value for name,value in raw.items())):
        recovery.reject('finish_commit_final_drift')
    transport.put(slot/'proof.json',proof);transport.put(slot/'authority.json',authority)
    return authority


def legacy_finish_guard(state):
    """A cold attempt nonce is ordinary history, never a legacy seal grant."""
    if any(event.get('kind') in ('closed_preservation_sealed','closed_seal_journal_head')
           for event in state['events']):
        recovery.reject('finish_legacy_postseal_unsupported')


class Engine:
    def __init__(self,base,state,reference,stage,journal):
        import root_cli
        self.base=Path(base);self.state=state;self.reference=reference;self.stage=stage;self.journal=journal
        if self.base!=guard.ROOT/('rollout-'+upload.OPERATION):recovery.reject('finish_root_path_invalid')
        recovery.verify_adopted_binding(self.base,state,root_cli.code_binding(self.base/'code'))
        try:nonce=reference['execution']['nonce']
        except (KeyError,TypeError):recovery.reject('finish_reference_invalid')
        if not isinstance(nonce,str) or not re.fullmatch('[a-f0-9]{64}',nonce):recovery.reject('finish_nonce_invalid')
        self.slot=self.base/'expired-finish-proofs'/nonce
        self.snapshot=upload.private_json(self.slot/'checkpoint.json',128<<20)
        self.observation=upload.private_json(self.slot/'proof.json',128<<20)
        self.readbacks={role:upload.private_json(self.slot/(role+'-readback.json'),65536)
            for role in ('original','observation')}
        self.record=upload.private_json(self.slot/'authority.json',65536)
        self.verify()
        ledger_path=self.slot/'resume-ledger.json'
        if ledger_path.exists():ledger=upload.private_json(ledger_path,128<<20)
        else:
            ledger={'schema':'voice-expired-finish-ledger-v1','operation':state['operation'],
                'authority_sha256':upload.digest(self.record),'pending':copy.deepcopy(self.observation['applied_workloads']),
                'completed':{},'nats_started':False}
            transport.put(ledger_path,ledger)
        if ledger['authority_sha256']!=upload.digest(self.record):recovery.reject('finish_ledger_authority_changed')
        self.ledger=recovery.FinishLedger(ledger,self.verify,self.closed,self.running,self.persist)

    def verify_core(self):
        expected=issue(self.snapshot,self.record['execution'],self.observation,self.readbacks,
            now=upload.timestamp(self.record['issued_at']))
        now=datetime.now(timezone.utc)
        if (self.record!=expected or self.record!=self.reference
            or not upload.timestamp(self.record['issued_at'])<=now<upload.timestamp(self.record['expires_at'])
            or stable(self.state)!=stable(self.snapshot)
            or self.state['events'][:len(self.snapshot['events'])]!=self.snapshot['events']
            or upload.private_json(self.slot/'authority.json',65536)!=self.record
            or upload.private_json(self.slot/'proof.json',128<<20)!=self.observation
            or upload.private_json(self.slot/'checkpoint.json',128<<20)!=self.snapshot
            or any(upload.private_json(self.slot/(role+'-readback.json'),65536)!=self.readbacks[role]
                for role in self.readbacks)):
            recovery.reject('finish_authority_expired_or_changed')
        if (self.slot/'safety-pause.json').exists():recovery.reject('finish_new_pause_requires_fresh_proof')
        if self.stage.marker.get('data',{}).get('phase')=='active':
            current=self.stage.kube.get('configmap','voice-nats-generation')
            if (current!=self.stage.marker or current['data'].get('knownRolloutOperation') is not None
                or not any(event.get('kind')=='rollout_released' and event.get('marker')==current
                    for event in self.state['events'][len(self.snapshot['events']):])):
                recovery.reject('finish_released_marker_unbound')
        else:self.stage.owned_marker()
        self.stage.verify_selected_storage_identity()

    def verify(self):
        self.verify_core()
        if self.observation['schema']=='voice-expired-finish-postseal-proof-v1':
            self.postseal_runtime()
        self.verify_core()

    def postseal_runtime(self):
        # Preserved legacy evidence is never admitted by the cold-route package.
        recovery.reject('finish_legacy_postseal_unsupported')

    def persist(self,row):
        from root_main import save
        save(self.slot/'resume-ledger.json',row,limit=128<<20)
        self.journal({'kind':'expired_finish_resume_progress','authority_sha256':upload.digest(self.record),
            'ledger_sha256':upload.digest(row)})

    def closed(self):
        import expired_proof
        from docker_runtime import DockerRuntime
        self.stage.verify_final_storage()
        sealed=self.current_seal_manifest()
        if sealed is not None:
            from preserve import native_inventory
            if expired_proof.current_inventory(DockerRuntime(self.base,self.state['operation']),self.stage)!=upload.digest(native_inventory(sealed)):
                recovery.reject('finish_new_sealed_store_changed')
            self.verify()
            return
        if expired_proof.current_inventory(DockerRuntime(self.base,self.state['operation']),self.stage)!=self.observation['selected_inventory_sha256']:
            recovery.reject('finish_selected_store_changed')
        if self.observation['schema']=='voice-expired-finish-postseal-proof-v1':
            selected=self.postseal_runtime().selected_postseal_cut(self.stage,self.slot,
                self.observation['postseal_binding']['observation'])
            if selected!=self.observation['postseal_binding']['current_cut']:
                recovery.reject('finish_postseal_selected_cut_changed')
        self.verify()

    def current_seal_manifest(self):
        if self.observation['schema']=='voice-expired-finish-postseal-proof-v1':
            recovery.reject('finish_legacy_postseal_unsupported')
        return None

    def running(self,name):
        captured=self.stage.snapshots[name];current=self.stage.kube.get('deployment',name)
        expected=self.observation['applied_workloads'][name]
        template=copy.deepcopy(current['spec']['template'])
        if name=='voice-user' and template.get('metadata',{}).get('annotations',{}).get('voice.io/nats-user-space-bootstrap')==self.state['operation']:
            del template['metadata']['annotations']['voice.io/nats-user-space-bootstrap']
            containers=[row for row in template['spec']['containers'] if row['name']=='user']
            if len(containers)!=1:recovery.reject('finish_cycle_shape_changed')
            env=containers[0].get('env',[])
            if [row for row in env if row.get('name')=='SPACE_GRPC_ADDR']!=[{'name':'SPACE_GRPC_ADDR','value':''}]:
                recovery.reject('finish_cycle_override_changed')
            containers[0]['env']=[row for row in env if row.get('name')!='SPACE_GRPC_ADDR']
        if (current['metadata']['uid']!=expected['uid'] or current['spec']['template']!=captured['spec']['template']
            or upload.digest(template)!=expected['template_sha256']
            or current['spec'].get('replicas',1)!=1
            or current.get('status',{}).get('observedGeneration',0)<current['metadata']['generation']
            or any(current.get('status',{}).get(key)!=1 for key in ('readyReplicas','updatedReplicas','availableReplicas'))):
            recovery.reject('finish_running_workload_changed')
        self.stage.verify_selected_storage_identity();self.verify()

    def restart(self):
        self.stage.finish_guard=self.ledger.guard
        self.stage.finish_authority_guard=self.verify
        self.stage.finish_ledger=self.ledger
        import transaction
        kwargs={'nonce':self.record['execution']['nonce']}
        if self.observation['schema']=='voice-expired-finish-postseal-proof-v1':
            # ROOT derives a new seal attempt from this exact capability; the
            # immutable observation/grant slot is never reused as a seal slot.
            self.closed()
            kwargs={'nonce':upload.digest({'purpose':'postseal-finish-seal','authority':self.record}),
                'admitted_cut':copy.deepcopy(self.observation['postseal_binding']['current_cut']),
                'finish_authority_sha256':upload.digest(self.record)}
        transaction.attach_closed_preservation(self.base,self.state,self.stage,self.verify,
            startup_deadline=upload.timestamp(self.record['expires_at']),**kwargs)
        return self.stage.restart_missing(self.ledger)

    def final(self):
        self.ledger.guard('release')
        for name,row in self.observation['applied_workloads'].items():
            self.running(name)
            if upload.digest(self.stage.kube.get('deployment',name)['spec']['template'])!=row['template_sha256']:
                recovery.reject('finish_final_template_changed')
        self.verify()

    def safety_paused(self):
        """Verified zero replicas create new obligations, never old permission.

        This runs after safety refence even when its permission has expired.
        A temporary owned startup template is recorded truthfully; a future
        proof must resolve it before claiming the exact final applied target.
        """
        paused={}
        for name,expected in self.observation['applied_workloads'].items():
            current=self.stage.kube.get('deployment',name)
            if (current['metadata']['uid']!=expected['uid'] or current['spec'].get('replicas',1)!=0
                or current['spec']['template']!=self.stage.snapshots[name]['spec']['template']):
                recovery.reject('finish_safety_pause_unverified')
            paused[name]={'uid':current['metadata']['uid'],'generation':current['metadata']['generation'],
                'template_sha256':upload.digest(current['spec']['template']),'replicas':0,
                'expected_target_template_sha256':expected['template_sha256']}
        record={'schema':'voice-expired-finish-safety-pause-v1','operation':self.state['operation'],
            'authority_sha256':upload.digest(self.record),'marker':copy.deepcopy(self.stage.marker),
            'workloads':paused,'requires_new_complete_proof':True}
        transport.put(self.slot/'safety-pause.json',record)
        proof_sha=upload.digest(record)
        for name,row in paused.items():
            self.ledger.record['completed'].pop(name,None)
            self.ledger.record['pending'][name]={**row,'safety_pause_proof_sha256':proof_sha,
                'requires_new_complete_proof':True}
        self.ledger.record['nats_started']=False
        self.persist(self.ledger.record)


def bind(base,state,reference,stage,journal):return Engine(base,state,reference,stage,journal)


def pause_owned(stage,base,state,slot,journal,verify_source,*,clock=None):
    """Explicit safety pause of proved applied targets, not application replay.

    Current source admission and the existing helper chain precede this call.
    The durable prospective event is saved before the first scale/restore CAS.
    Failures retain that progress; the caller refences and saves truthful state.
    """
    import transaction
    clock=clock or (lambda:datetime.now(timezone.utc));start=clock();deadline=start+timedelta(seconds=600)
    base=Path(base);slot=Path(slot)
    if (base.name!='rollout-'+upload.OPERATION or slot.parent!=base/'expired-finish-proofs'
        or state['operation']!=upload.OPERATION or state['target']['tag']!=upload.ORIGINAL_TARGET
        or state['authorization']['operation']!=state['operation']
        or state['authorization']['target']!=state['target']
        or state['phase'] not in ('PAUSED_APPLY','POST_APPLY_PROOF','RESTART')
        or state['status'] not in ('WAITING','BLOCKED')):recovery.reject('finish_pause_state_invalid')
    before=upload.private_read(base/'checkpoint.json')
    if upload.private_json(base/'checkpoint.json')!=state:recovery.reject('finish_pause_checkpoint_changed')
    restore={};current={}
    # Validate EVERY workload before any scale; no partial unknown ownership.
    for name,captured in stage.snapshots.items():
        row=stage.kube.get('deployment',name);template=copy.deepcopy(row['spec']['template'])
        wanted=state['target']['template_hashes'].get(name,upload.digest(captured['spec']['template']))
        if (row['metadata']['uid']!=captured['metadata']['uid']
            or type(row['spec'].get('replicas',1)) is not int or row['spec'].get('replicas',1) not in (0,1)):
            recovery.reject('finish_pause_ownership_changed')
        if upload.digest(template)!=wanted:
            annotations=template.get('metadata',{}).get('annotations',{})
            if (name!='voice-user' or annotations.get('voice.io/nats-user-space-bootstrap')!=state['operation']
                or template!=captured['spec']['template']
                or not any(event.get('kind')=='user_cycle_override' and event.get('owner')==state['operation'] for event in state['events'])):
                recovery.reject('finish_pause_target_changed')
            del annotations['voice.io/nats-user-space-bootstrap']
            containers=[value for value in template['spec']['containers'] if value['name']=='user']
            if len(containers)!=1:recovery.reject('finish_pause_cycle_shape_invalid')
            env=containers[0].get('env',[])
            if [value for value in env if value.get('name')=='SPACE_GRPC_ADDR']!=[{'name':'SPACE_GRPC_ADDR','value':''}]:
                recovery.reject('finish_pause_cycle_override_invalid')
            containers[0]['env']=[value for value in env if value.get('name')!='SPACE_GRPC_ADDR']
            if upload.digest(template)!=wanted:recovery.reject('finish_pause_cycle_target_changed')
            restore[name]=template
        current[name]=row
    stage.owned_marker();verify_source()
    if clock()>=deadline:recovery.reject('finish_pause_expired')
    admission={'schema':'voice-expired-finish-pause-admission-v1','operation':state['operation'],
        'checkpoint_sha256':__import__('hashlib').sha256(before).hexdigest(),
        'started_at':start.isoformat(),'workloads':copy.deepcopy(current),'target_sha256':upload.digest(state['target'])}
    transport.put(slot/'pause-admission.json',admission)
    stage.snapshots=copy.deepcopy(current)
    def emit(event):
        state['context']=transaction.context(stage);journal(event)
        transaction.save(base/'checkpoint.json',state)
    state['fence_status']='UNKNOWN'
    emit({'kind':'expired_finish_owned_pause_entered','admission_sha256':upload.digest(admission)})
    stage.save=emit
    for name in tuple(n for n in current if n!=HUB)+(HUB,):
        verify_source();stage.owned_marker()
        if clock()>=deadline:recovery.reject('finish_pause_expired')
        if current[name]['spec'].get('replicas',1)==1:stage.scale(name,0)
    stage.no_pods(tuple(current));stage.verify_final_storage()
    for name,template in restore.items():
        verify_source();stage.owned_marker()
        if clock()>=deadline:recovery.reject('finish_pause_expired')
        row=stage.kube.get('deployment',name)
        stage.snapshots[name]=stage.kube.cas('deployment',row,[
            {'op':'test','path':'/spec/replicas','value':0},
            {'op':'test','path':'/spec/template','value':stage.snapshots[name]['spec']['template']},
            {'op':'replace','path':'/spec/template','value':template}])
        emit({'kind':'expired_finish_owned_cycle_restored','name':name,'template_sha256':upload.digest(template)})
    verify_source();stage.verify_final_storage()
    if clock()>=deadline:recovery.reject('finish_pause_expired')
    workloads={}
    for name,captured in stage.snapshots.items():
        row=stage.kube.get('deployment',name)
        if (row['metadata']['uid']!=captured['metadata']['uid'] or row['spec'].get('replicas',1)!=0
            or row['spec']['template']!=captured['spec']['template']):recovery.reject('finish_pause_final_changed')
        workloads[name]={'uid':row['metadata']['uid'],'generation':row['metadata']['generation'],
            'template_sha256':upload.digest(row['spec']['template']),'replicas':0,
            'pause_proof_sha256':upload.digest({'admission':admission,'observed':row})}
    state['fence_status']='VERIFIED'
    emit({'kind':'expired_finish_owned_pause_verified','workloads_sha256':upload.digest(workloads)})
    record={'schema':'voice-expired-finish-owned-pause-v1','operation':state['operation'],
        'started_at':start.isoformat(),'completed_at':clock().isoformat(),'admission_sha256':upload.digest(admission),
        'workloads':workloads,'checkpoint_sha256':upload.digest(state)}
    transport.put(slot/'pause.json',record)
    return record


def observe_post_apply(runtime,stage,state,slot,validate,checksum,startup_interval):
    """Distinct CLOSED finish observation, never a replacement original cut.

    Three private copies prove exact native message-file bytes, the authorized post-apply
    contract/full consumer state, and the current observation. A changed native
    inventory needs a separately source-bound selected startup interval and the
    same strict pinned metadata classifier; unknown changes always veto.
    Caller holds the root lock and has proved the exact applied target/pause.
    """
    from native_store import verify_archive,archive_closed_store
    from preserve import recovered_census,native_inventory,canonical
    from rollout_census import semantic
    from expired_proof import observe_copy,current_inventory,verify_native_continuity
    import tarfile
    slot=Path(slot);base=Path(runtime.base)
    if (slot.parent!=base/'expired-finish-proofs' or not callable(validate)
        or state['nats_migration'].get('verified') is not True):recovery.reject('finish_observation_path_invalid')
    original=base/'rollout-before.tar';old=state['cut'];post=state['nats_migration']['cut']
    post_archive=Path(post.get('archive',str(original)))
    try:post_archive.relative_to(base)
    except ValueError:recovery.reject('finish_post_archive_path_invalid')
    if '..' in post_archive.parts:recovery.reject('finish_post_archive_path_invalid')
    for archive,cut in ((original,old),(post_archive,post)):
        if (canonical(cut['census'])!=cut['census_sha256']
            or not verify_archive(archive,cut['manifest'])):recovery.reject('finish_baseline_changed')
    stage.verify_final_storage();runtime.no_operation_containers(running_only=False)
    current=slot/'rollout-before.tar';manifest=archive_closed_store(stage.final_path,current)
    originals=observe_copy(runtime,original,old['manifest'],'finish-original',stage.verify_final_storage,native_only=True)
    original_row,original_observations=recovered_census(old['census'],originals['census'],originals['recovery'],old['manifest'])
    if semantic(original_row)!=semantic(old['census']):recovery.reject('finish_original_census_changed')
    applied=observe_copy(runtime,post_archive,post['manifest'],'finish-applied',stage.verify_final_storage,validate,native_only=True)
    applied_row,applied_observations=recovered_census(post['census'],applied['census'],applied['recovery'],post['manifest'])
    observed=observe_copy(runtime,current,manifest,'finish-current',stage.verify_final_storage,validate,native_only=True)
    current_row,current_observations=recovered_census(post['census'],observed['census'],observed['recovery'],post['manifest'])
    if (semantic(applied_row)!=semantic(post['census']) or semantic(current_row)!=semantic(post['census'])
        or originals['native_messages']!=applied['native_messages']):
        recovery.reject('finish_records_or_consumer_state_changed')
    verify_native_continuity(applied,observed)
    if native_inventory(manifest)==native_inventory(post['manifest']):
        disposition={'schema':'voice-native-startup-disposition-v1','branch':'EXACT','changed_paths':[]}
    else:
        if startup_interval is None:recovery.reject('finish_selected_startup_evidence_missing')
        def read(side,path):
            archive,wanted=(post_archive,post['manifest']) if side=='before' else (current,manifest)
            if not verify_archive(archive,wanted):recovery.reject('finish_archive_changed')
            with tarfile.open(archive,'r:') as bundle:
                member=bundle.getmember(path)
                if not member.isfile() or not 0<=member.size<=256<<10:recovery.reject('finish_native_field_bound')
                source=bundle.extractfile(member)
                if source is None:recovery.reject('finish_native_field_missing')
                with source:return source.read((256<<10)+1)
        disposition=recovery.classify_startup(post['manifest'],manifest,post['census'],current_row,
            applied['native_messages'],observed['native_messages'],read,checksum,startup_interval)
    for archive,cut in ((original,old),(post_archive,post)):
        if not verify_archive(archive,cut['manifest']):recovery.reject('finish_final_baseline_changed')
    stage.verify_final_storage();runtime.no_operation_containers(running_only=False)
    inventory=upload.digest(native_inventory(manifest))
    if current_inventory(runtime,stage)!=inventory:recovery.reject('finish_selected_changed_during_proof')
    result={'schema':'voice-expired-finish-current-observation-v1','operation':state['operation'],
        'original_cut_sha256':upload.digest(old),'post_apply_cut_sha256':upload.digest(post),
        'manifest':manifest,'census':current_row,'native_messages':observed['native_messages'],
        'closed_durable_states':observed['closed_durable_states'],
        'native_disposition':disposition,'selected_inventory_sha256':inventory,
        'original_private_recovery':originals['recovery'],'applied_private_recovery':applied['recovery'],
        'current_private_recovery':observed['recovery'],'ephemeral_observations':{
            'original':original_observations,'applied':applied_observations,'current':current_observations}}
    transport.put(slot/'rollout-before-manifest.json',result)
    transport.put(slot/'copy-checkpoint.json',{'schema':'voice-expired-finish-copy-v1',
        'operation':state['operation'],'observation_sha256':upload.digest(result),
        'archive_sha256':manifest['archive_sha256']})
    return result
