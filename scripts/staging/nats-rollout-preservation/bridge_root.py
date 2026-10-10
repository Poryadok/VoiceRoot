"""Installed root actions: approved source, encrypted off-node custody, restart."""
import datetime as dt
import grp
import json
import os
from pathlib import Path
import secrets
import shutil
import time
import compiler
import encrypted_cut
import github_custody
import guard
import root_cli
import source_authority
import transaction
from root_main import private_json as root_private_json,public_read,save as root_save
from runtime_stage import RolloutStage
from stage_runtime import Kube, HUB
from controller import Blocked

INSTALLED=guard.ROOT/'installed'

def private_json(path):return root_private_json(path,limit=128<<20 if Path(path).name=='checkpoint.json' else 2<<20)
def save(path,row):root_save(path,row,limit=128<<20)

def summary(state):
    return {key:state[key] for key in ('operation','phase','status','fence_status','error') if key in state}

class Actions:
    def __init__(self,code):
        self.code=Path(code);self.binding=root_cli.code_binding(self.code)

    def _report(self,name,status):
        observer=getattr(self,'_observer',None)
        if observer is not None:observer(name,status)

    def execute_observed(self,request,observe):
        if getattr(self,'_observer',None) is not None:raise Blocked('bridge_operation_already_observed')
        self._observer=observe
        try:return self.execute(request)
        finally:self._observer=None

    def state(self,operation):
        base=root_cli.operation_path(str(guard.ROOT/('rollout-'+operation)))
        state=private_json(base/'checkpoint.json')
        if state.get('operation')!=operation:raise Blocked('bridge_operation_custody_invalid')
        import paused_recovery
        paused_recovery.verify_adopted_binding(base,state,self.binding)
        return base,state

    def recover(self,request):
        if request['action']=='status':return self.execute(request)
        operation=request['nonce'][:12] if request['action'] in ('prepare','prepare-rollback') else request.get('operation')
        try:base,state=self.state(operation)
        except (OSError,Blocked):return None
        if request['action']=='resume-cipher-upload':
            import preserved_upload
            _,continuity=preserved_upload.verify_helper_continuity(base,state,self.binding)
            record,_,_=preserved_upload.load(base,request['nonce'],state,continuity)
            if record['upload_execution']['dispatcher_run_id']!=request['run_id']:
                raise Blocked('preserved_upload_replay_run_changed')
            return preserved_upload.result(base,state,record)
        if request['action']=='finish' and state.get('status')=='PASS':
            self._active_release(state)
            return summary(state)
        if request['action']=='authorize' and state.get('phase')=='PAUSED_APPLY' and state.get('custody',{}).get('artifact_id')==request['artifact_id']:
            return summary(state)|{'authorization':str(base/'apply-authorization.json')}
        if request['action'] in ('prepare','prepare-rollback','resume-cold-backup') and state.get('phase')=='AWAITING_OFF_NODE' and state.get('cipher_binding'):
            if request['action']=='resume-cold-backup':
                execution=state.get('execution_authority',{})
                cipher=state['cipher_binding']
                if (execution.get('nonce')!=request['nonce'] or execution.get('dispatcher_run_id')!=request['run_id']
                    or cipher.get('operation')!=operation or cipher.get('run_id')!=request['run_id']
                    or cipher.get('head_sha')!=execution.get('head_sha')):
                    raise Blocked('paused_recovery_cipher_execution_changed')
            return self.copy_result(base,state)
        return None

    def copy_result(self,base,state):
        binding=state['cipher_binding']
        return summary(state)|{'cipher_path':str(base/'export'/'rollout-backup.cms'),
            'artifact_name':'voice-nats-rollout-'+state['operation']+'-'+binding['challenge'],
            'challenge':binding['challenge'],'changed_services':','.join(state['target']['changed_services']),
            'deploy_mode':state['target']['mode'],'source_sha':state['target']['tag']}

    def _idle(self):
        rows=list(guard.ROOT.glob('rollout-*'))
        if len(rows)>1000:raise Blocked('bridge_operation_inventory_bound')
        for base in rows:
            if root_cli.rollout_directory_kind(base)=='capture':continue
            if not (base/'checkpoint.json').exists():raise Blocked('bridge_incomplete_operation_present')
            row=private_json(base/'checkpoint.json')
            if row.get('status') not in ('PASS','ROLLED_BACK'):raise Blocked('bridge_active_operation_present')

    def _dispatcher(self,request):
        headers=source_authority._headers(request['token']);deadline=time.monotonic()+30
        run=source_authority._get(source_authority.API+'/actions/runs/'+str(request['run_id']),headers,deadline)
        if run['id']!=request['run_id'] or run['repository']['full_name']!=source_authority.REPO or run['head_repository']['full_name']!=source_authority.REPO or run['head_branch']!='master' or run['status']!='in_progress':raise Blocked('bridge_dispatcher_authority_invalid')
        workflow=run['workflow_id'];path=run['path'].split('@',1)[0]
        if workflow==source_authority.WORKFLOW and path=='.github/workflows/ci.yml' and run['event']=='push':source_run=run['id']
        elif workflow==263689731 and path=='.github/workflows/staging-deploy.yml' and run['event']=='workflow_dispatch':
            result=source_authority._get(source_authority.API+'/actions/workflows/'+str(source_authority.WORKFLOW)+'/runs?branch=master&event=push&per_page=100',headers,deadline)
            candidates=[r for r in result['workflow_runs'] if r['head_sha']==run['head_sha'] and r['event']=='push' and r['head_branch']=='master' and r['workflow_id']==source_authority.WORKFLOW and r['status'] in ('in_progress','completed')]
            if not candidates:raise Blocked('bridge_approved_ci_source_missing')
            source_run=max(candidates,key=lambda r:r['id'])['id']
        else:raise Blocked('bridge_dispatcher_authority_invalid')
        if 'source_sha' in request and request['source_sha']!=run['head_sha']:raise Blocked('bridge_dispatcher_source_changed')
        return source_run,{'dispatcher_run_id':run['id'],'run_attempt':run['run_attempt'],'head_sha':run['head_sha'],'event':run['event'],'workflow_id':workflow,'path':path,'repository':source_authority.REPO}

    def _active_release(self,state,*,authority_guard=None):
        if authority_guard is not None:authority_guard()
        save(INSTALLED/'active-release.json',{'operation':state['operation'],'namespace_uid':state['context']['expected']['namespace_uid'],
            'pvc_uid':state['context']['expected']['source_claim_uid'],'target_sha256':state['target']['manifest_sha256']})

    def _prepare(self,request,previous=None):
        self._idle();operation=request['nonce'][:12]
        source_run,execution=self._dispatcher(request)
        workspace=INSTALLED/'sources'/operation;workspace.mkdir(mode=0o700)
        self._report('source-capture','STARTED')
        approved=source_authority.capture_source(request['token'],source_run,execution['head_sha'],workspace)
        self._report('source-capture','COMPLETE')
        self._report('workload-capture','STARTED')
        kube=Kube();marker=kube.get('configmap','voice-nats-generation')
        frontends=[s for s in request['changed_services'] if s in ('web','admin','developer-portal')] if request['mode']=='images-only' else ['web','admin','developer-portal']
        renderer_selected=request['mode']=='full' or previous is not None and previous.get('renderer_authority') is not None
        stage=RolloutStage.capture(kube,operation,lambda event:None,['voice-'+s for s in frontends]+([HUB] if renderer_selected else []))
        self._report('workload-capture','COMPLETE')
        self._report('policy-validation','STARTED')
        policy=private_json(INSTALLED/'policy.json')
        optional={'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host','developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability','minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin'}
        if not set(policy)<=optional or 's3_signing_endpoint' not in policy:raise Blocked('bridge_policy_invalid')
        self._report('policy-validation','COMPLETE')
        self._report('image-selection','STARTED')
        names=request['changed_services'] if request['mode']=='images-only' else [n.removeprefix('voice-') for n in stage.snapshots if n!=HUB]
        images={};image_deadline=time.monotonic()+600
        for service in names:
            if service not in approved['images'] or service=='nats-hub-config-renderer':raise Blocked('bridge_target_service_invalid')
            name='voice-'+service;row=stage.snapshots[name]
            for container in row['spec']['template']['spec'].get('initContainers',[])+row['spec']['template']['spec']['containers']:
                key=name+'/'+container['name'];image=stage.old_images[key]
                # Only the matching app repository receives a new approved
                # image; sidecar/init images retain the captured running pins.
                if image.split('@',1)[0]==source_authority.REGISTRY+'/'+service:image=approved['images'][service]
                if previous is not None:
                    if key not in previous['target']['images']:raise Blocked('bridge_rollback_current_image_changed')
                    expected_image=previous['target']['images'][key]
                    if stage.old_images[key]!=expected_image and not source_authority.same_image_content(stage.old_images[key],expected_image,image_deadline):
                        raise Blocked('bridge_rollback_current_image_changed')
                    image=previous['context']['old_images'][key]
                images[key]=image
        self._report('image-selection','COMPLETE')
        self._report('contract-selection','STARTED')
        contract=json.loads(public_read(self.code/'nats-known-baseline'/'deployed-contract.json'))
        try:active_contract=private_json(INSTALLED/'active-contract.json')
        except FileNotFoundError:active_contract=None
        if active_contract is not None:
            if (active_contract.get('schema')!='voice-nats-active-contract-v1' or active_contract.get('migration_id')!='space-chat-social-24h-friend-removed-v1'
                or active_contract.get('generation')!=stage.expected['generation'] or active_contract.get('namespace_uid')!=stage.expected['namespace_uid']
                or active_contract.get('pvc_uid')!=stage.expected['source_claim_uid']):raise Blocked('nats_active_contract_identity_changed')
            contract=active_contract['contract']
        parameters={**policy,'registry':source_authority.REGISTRY,'tag':approved['source_sha'],
            'mode':request['mode'],'changed_services':request['changed_services'],'images':images,
            'contract':contract,
            'enrolled':list(stage.snapshots),'generation':marker['data']['generation'],'dataPVC':marker['data']['dataPVC']}
        self._report('contract-selection','COMPLETE')
        self._report('nats-preflight','STARTED')
        renderer_authority=None
        if renderer_selected:
            import renderer_root
            renderer_image=approved['images']['nats-hub-config-renderer']
            if previous is not None:
                current_renderer=stage.old_images[HUB+'/nats-config-renderer']
                expected_renderer=previous['renderer_authority']['descriptor']['images']['target']
                if current_renderer!=expected_renderer and not source_authority.same_image_content(current_renderer,expected_renderer,image_deadline):
                    raise Blocked('renderer_rollback_current_image_changed')
                renderer_image=previous['renderer_authority']['descriptor']['images']['old']
            renderer_authority=renderer_root.prove(kube,stage,approved['source_sha'],renderer_image)
        import actor_root
        actor_authority=actor_root.preflight(kube,workspace,self.code,self.binding,stage,names,compiler.decode_yaml)
        nats_authority=None
        if not guard.frontend_image_only({**parameters,'template_hashes':{'voice-'+s:None for s in request['changed_services']}}):
            try:enrollment=private_json(guard.ROOT/'bootstrap-enrollment.json')
            except FileNotFoundError:enrollment=None
            if enrollment is not None:
                import nats_root_plan
                nats_authority=nats_root_plan.preflight(kube,workspace,enrollment,parameters['contract'],compiler.decode_yaml)
                parameters['bootstrap_target_scripts']={part:row['sha256'] for part,row in nats_authority['scripts'].items()}
        self._report('nats-preflight','COMPLETE')
        self._report('target-build','STARTED')
        build=workspace.parent/(operation+'-build');build.mkdir(mode=0o700)
        parameter_path=build/'parameters.json';save(parameter_path,parameters)
        output=build/('target-'+approved['source_sha']+'.json')
        compiler.main([str(workspace),str(parameter_path),str(output)])
        output.chmod(0o600)
        value=private_json(output)
        if renderer_authority is not None:value['target']=renderer_root.bind_target(value['target'],renderer_authority)
        self._report('target-build','COMPLETE')
        def authority_before_fence():
            source_authority._head(source_authority._headers(request['token']),approved['source_sha'],time.monotonic()+30)
            actor_root.revalidate(kube,workspace,self.code,self.binding,stage,names,compiler.decode_yaml,actor_authority)
            if renderer_authority is not None:renderer_root.revalidate(kube,stage,renderer_authority)
            if nats_authority is not None:
                current=nats_root_plan.preflight(kube,workspace,enrollment,parameters['contract'],compiler.decode_yaml)
                # Business records may advance, but config/consumer intent and
                # dynamic subject ownership cannot change between plan/fence.
                if current['plan']!=nats_authority['plan'] or current['binding']!=nats_authority['binding']:raise Blocked('nats_migration_live_contract_changed')
        state=root_cli.prepare_value(value,self.code,self.binding,operation,before_fence=authority_before_fence,nats_authority=nats_authority,actor_authority=actor_authority,actor_services=names,renderer_authority=renderer_authority)
        base=guard.ROOT/('rollout-'+operation)
        state['service_actor_authority']=actor_authority
        state['service_actor_services']=names
        state['source_authority']={key:value for key,value in approved.items() if key!='source_files'}
        state['execution_authority']=execution
        if previous is not None:state['rollback_from']={'operation':previous['operation'],'original_release_source':previous['target']['tag'],'images':images}
        save(base/'checkpoint.json',state)
        return self._encrypt(base,state,execution)

    def _verify_decrypted_restore(self,base,state,stage,*,payload_base=None,cut=None,binding=None):
        """Full remote-byte equality binds the actual authenticated restore."""
        from preserve import closed_native_states,recovered_census,canonical,native_catalog,verify_catalog
        from rollout_census import semantic
        from expired_proof import observe_copy
        from native_store import verify_archive
        from docker_runtime import NATS_IMAGE,DockerRuntime
        payload_base=Path(payload_base or base);cut=cut or state['cut'];binding=binding or state['cipher_binding']
        runtime=DockerRuntime(base,state['operation'])
        def readback(archive):
            stage.verify_final_storage()
            if not verify_archive(archive,cut['manifest']):raise Blocked('decrypted_restore_native_changed')
            states=closed_native_states(runtime,archive,cut['manifest'],cut['census'],stage.verify_final_storage)
            catalog=native_catalog(runtime,archive,cut['manifest'],stage.verify_final_storage)
            observed=observe_copy(runtime,archive,cut['manifest'],'decrypted-readback',stage.verify_final_storage)
            verify_catalog(catalog,observed['census'])
            compared,ephemeral=recovered_census(cut['census'],observed['census'],observed['recovery'],cut['manifest'])
            if (canonical(cut['census'])!=cut['census_sha256'] or semantic(compared)!=semantic(cut['census'])):
                raise Blocked('decrypted_restore_census_changed')
            stage.verify_final_storage()
            return {'native_archive_verified':True,'complete_durable_sha256':canonical(states),
                'census_sha256':cut['census_sha256'],'full_records':observed['records'],
                'ephemeral_recovery_observations':ephemeral,'server_image':NATS_IMAGE}
        members={'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json'}
        if payload_base==Path(base):members.update(state.get('cipher_space_members',{}))
        members.update(binding.get('space_members',{}));members.update(binding.get('postseal_members',{}))
        receipt=encrypted_cut.decrypt_verify(payload_base,INSTALLED/'recovery',binding,readback,expected_members=members)
        save(payload_base/'decrypted-restore.json',receipt)
        return receipt

    def _encrypt(self,base,state,execution):
        cut=state['cut']
        copy_binding={'schema':'nats-rollout-copy-v1','operation':state['operation'],
            'archive_sha256':cut['manifest']['archive_sha256'],'manifest_sha256':cut['manifest_sha256'],
            'census_sha256':cut['census_sha256']}
        if 'space_backup' in state:
            _,members=encrypted_cut.space_members(base,state['space_backup'])
            copy_binding['space_members']=members
        save(base/'copy-checkpoint.json',copy_binding)
        cipher=encrypted_cut.encrypt_cut(base,INSTALLED/'recovery',space=state.get('space_backup'))
        if 'space_members' in cipher:state['cipher_space_members']=cipher.pop('space_members')
        state['cipher_binding']={**cipher,'operation':state['operation'],'challenge':secrets.token_hex(16),
            'run_id':execution['dispatcher_run_id'],'head_sha':execution['head_sha'],'created_at':dt.datetime.now(dt.timezone.utc).isoformat()}
        gid=grp.getgrnam('pmd').gr_gid;export=base/'export';export.mkdir(mode=0o750)
        shutil.copyfile(base/'rollout-backup.cms',export/'rollout-backup.cms')
        os.chown(export,0,gid);export.chmod(0o750)
        os.chown(export/'rollout-backup.cms',0,gid);(export/'rollout-backup.cms').chmod(0o440)
        os.chown(base,0,gid);base.chmod(0o750)
        save(base/'checkpoint.json',state)
        return self.copy_result(base,state)

    def _resume_cold_backup(self,request):
        import paused_recovery
        if (request['operation']!=paused_recovery.OPERATION or request['nonce']==paused_recovery.NONCE
            or request['nonce'][:12]==paused_recovery.OPERATION):
            raise Blocked('paused_recovery_execution_identity_invalid')
        base,state=self.state(request['operation'])
        rows=list(guard.ROOT.glob('rollout-*'))
        if len(rows)>1000:raise Blocked('bridge_operation_inventory_bound')
        for path in rows:
            if root_cli.rollout_directory_kind(path)=='capture' or path==base:continue
            if private_json(path/'checkpoint.json').get('status') not in ('PASS','ROLLED_BACK'):
                raise Blocked('paused_recovery_other_operation_present')
        source_run,execution=self._dispatcher(request)
        workspace=INSTALLED/'sources'/('recovery-'+request['nonce'])
        workspace.mkdir(mode=0o700)
        current=source_authority.capture_source(request['token'],source_run,execution['head_sha'],workspace)
        for relative,wanted in self.binding.items():
            if Path(relative).name in ('kernel','bootstrap-renewer'):continue
            if current['source_files'].get('scripts/staging/'+relative)!=wanted:
                raise Blocked('paused_recovery_current_helper_source_changed')
        approved=paused_recovery.reconstruct_source(request['token'],base,state,self.binding)
        source_authority._head(source_authority._headers(request['token']),execution['head_sha'],time.monotonic()+30)
        execution['helper_source_authority']={k:v for k,v in current.items() if k!='source_files'}
        execution['nonce']=request['nonce']
        def verify_execution():
            source_authority._head(source_authority._headers(request['token']),execution['head_sha'],time.monotonic()+30)
        state=paused_recovery.resume_capture(Kube(),base,state,self.code,self.binding,approved,execution,verify_execution=verify_execution)
        verify_execution()
        return self._encrypt(base,state,execution)

    def _resume_cipher_upload(self,request):
        import preserved_upload
        if request['operation']!=preserved_upload.OPERATION:raise Blocked('preserved_upload_operation_invalid')
        base,state=self.state(request['operation'])
        _,continuity=preserved_upload.verify_helper_continuity(base,state,self.binding)
        rows=list(guard.ROOT.glob('rollout-*'))
        if len(rows)>1000:raise Blocked('bridge_operation_inventory_bound')
        for path in rows:
            if path==base or root_cli.rollout_directory_kind(path)=='capture':continue
            if private_json(path/'checkpoint.json').get('status') not in ('PASS','ROLLED_BACK'):
                raise Blocked('preserved_upload_other_operation_active')
        source_run,execution=self._dispatcher(request);execution['nonce']=request['nonce']
        workspace=INSTALLED/'sources'/('upload-'+request['nonce']);workspace.mkdir(mode=0o700)
        current=source_authority.capture_source(request['token'],source_run,execution['head_sha'],workspace)
        for relative,wanted in self.binding.items():
            if Path(relative).name in ('kernel','bootstrap-renewer'):continue
            if current['source_files'].get('scripts/staging/'+relative)!=wanted:
                raise Blocked('preserved_upload_current_helper_source_changed')
        source_authority._head(source_authority._headers(request['token']),execution['head_sha'],time.monotonic()+30)
        stage=transaction.reconstruct(Kube(),state,lambda event:None)
        stage.owned_marker();stage.verify_final_storage()
        marker=Kube().get('configmap','voice-nats-generation')
        if marker['data'].get('phase')!='rollout-capturing' or marker['metadata']['resourceVersion']!='3073024':
            raise Blocked('preserved_upload_fence_changed')
        checkpoint_before=preserved_upload.private_read(base/'checkpoint.json')
        record=preserved_upload.prepare(base,state,execution,INSTALLED,continuity,
            {k:v for k,v in current.items() if k!='source_files'})
        stage.owned_marker();stage.verify_final_storage()
        source_authority._head(source_authority._headers(request['token']),execution['head_sha'],time.monotonic()+30)
        if (preserved_upload.private_read(base/'checkpoint.json')!=checkpoint_before
            or Kube().get('configmap','voice-nats-generation')!=marker):
            raise Blocked('preserved_upload_final_integrity_changed')
        preserved_upload.verify_deadline(record,dt.datetime.now(dt.timezone.utc))
        return preserved_upload.result(base,state,record)

    def execute(self,request):
        expired_actions={'prepare-expired-native':self._prepare_expired_native,
            'authorize-expired-native':self._authorize_expired_native,'finish-expired-native':self._finish_expired_native,
            'prepare-expired-finish':self._prepare_expired_finish,'authorize-expired-finish':self._authorize_expired_finish}
        if request['action'] in expired_actions:return expired_actions[request['action']](request)
        if request['action']=='prepare':return self._prepare(request)
        if request['action']=='resume-cold-backup':return self._resume_cold_backup(request)
        if request['action']=='resume-cipher-upload':return self._resume_cipher_upload(request)
        if request['action']=='prepare-rollback':
            base,previous=self.state(request['operation'])
            active=private_json(INSTALLED/'active-release.json')
            if active.get('operation')!=previous['operation'] or active.get('target_sha256')!=previous['target']['manifest_sha256']:raise Blocked('bridge_rollback_latest_release_required')
            transaction.rollback_package(previous) # successful images-only/immutable old pins gate
            changed=transaction.rollback_services(previous)
            return self._prepare({**request,'mode':'images-only','changed_services':changed},previous)
        base,state=self.state(request['operation'])
        if request['action']=='status':return summary(state)
        if request['action'] in ('authorize','authorize-preserved-upload'):
            upload_record=None
            if request['action']=='authorize-preserved-upload':
                import preserved_upload
                _,continuity=preserved_upload.verify_helper_continuity(base,state,self.binding)
                _,execution=self._dispatcher(request);execution['nonce']=request['upload_nonce']
                source_authority._head(source_authority._headers(request['token']),execution['head_sha'],time.monotonic()+30)
                receipt=preserved_upload.verify_artifact(request['token'],base,state,continuity,
                    request['upload_nonce'],execution,request['artifact_id'])
                upload_record,_,_=preserved_upload.load(base,request['upload_nonce'],state,continuity,execution=execution)
            else:
                # A revised helper cannot route preserved output through the old binding.
                if (base/'cipher-upload').exists():raise Blocked('preserved_upload_explicit_authorization_required')
                receipt=github_custody.verify_artifact(request['token'],state['cipher_binding'],base/'rollout-backup.cms',request['artifact_id'])
            # Dedicated readback/actor/deadline rejection leaves the captured
            # checkpoint byte-for-byte intact, so a failed proof is not custody.
            if upload_record is None:
                state['custody']=receipt;save(base/'checkpoint.json',state)
            self._verify_decrypted_restore(base,state,transaction.reconstruct(Kube(),state,lambda event:None))
            import actor_root
            actor_root.revalidate(Kube(),INSTALLED/'sources'/state['operation'],self.code,self.binding,
                transaction.reconstruct(Kube(),state,lambda event:None),state['service_actor_services'],compiler.decode_yaml,state['service_actor_authority'])
            if upload_record is not None:
                preserved_upload.verify_deadline(upload_record,dt.datetime.now(dt.timezone.utc))
                state['custody']=receipt
                transaction.authorize(Kube(),base,state,state['cut']['manifest']['archive_sha256'],state['cut']['manifest_sha256'],state['contract'],
                    authorization_deadline=dt.datetime.fromisoformat(upload_record['deadline']))
            else:
                transaction.authorize(Kube(),base,state,state['cut']['manifest']['archive_sha256'],state['cut']['manifest_sha256'],state['contract'])
            gid=grp.getgrnam('pmd').gr_gid
            for name in ('apply-authorization.json','apply-manifests.json'):os.chown(base/name,0,gid);(base/name).chmod(0o440)
            return summary(state)|{'authorization':str(base/'apply-authorization.json')}
        if request['action']=='finish':
            if state.get('expired_recovery_authority') is not None:
                raise Blocked('expired_native_explicit_finish_required')
            receipt=guard.protected_receipt(base/'apply-authorization.json')
            result=transaction.finish(Kube(),base,state,receipt,request['claim_rv'],state['contract'])
            self._active_release(result)
            return summary(result)
        raise Blocked('bridge_action_invalid')

    def _expired_source_admission(self,request):
        """Fresh helper authority and historical target data remain separate.

        Internal producer entry only; no request route admits an incomplete
        proof/upload/finish protocol. The final consumer wires this result.
        """
        import expired_admission
        import expired_recovery
        import paused_recovery
        base,state=self.state(request['operation'])
        if request['operation']!=paused_recovery.OPERATION:
            raise Blocked('expired_recovery_operation_changed')
        source_run,execution=self._dispatcher(request)
        execution['nonce']=request['nonce']
        workspace=INSTALLED/'sources'/('expired-'+request['nonce'])
        workspace.mkdir(mode=0o700)
        current=source_authority.capture_source(request['token'],source_run,execution['head_sha'],workspace)
        for relative,wanted in self.binding.items():
            if Path(relative).name in ('kernel','bootstrap-renewer'):continue
            if current['source_files'].get('scripts/staging/'+relative)!=wanted:
                raise Blocked('expired_recovery_current_helper_source_changed')
        historical=paused_recovery.reconstruct_source(request['token'],base,state,self.binding)
        def fresh():
            source_authority._head(source_authority._headers(request['token']),execution['head_sha'],time.monotonic()+30)
            expired_recovery.verify_adopted_binding(base,state,self.binding)
        stage,admission=expired_admission.admit(Kube(),base,state,self.code,self.binding,historical,fresh)
        def all_fresh():
            _,again=expired_admission.admit(Kube(),base,state,self.code,self.binding,historical,fresh,
                admission['get_permission_proof'])
            if again!=admission:raise Blocked('expired_recovery_admission_changed')
        return base,state,stage,execution,current,admission,all_fresh

    def _finish_source_admission(self,request):
        """Current helper admission plus completed historical application facts.

        Never recompiles the saved target or invokes apply. Owned pause events
        may advance context; immutable input/target/DB facts remain checked.
        """
        import expired_admission
        import expired_recovery
        base,state=self.state(request['operation'])
        source_run,execution=self._dispatcher(request)
        execution['nonce']=request['nonce']
        workspace=INSTALLED/'sources'/('finish-'+request['nonce'])
        workspace.mkdir(mode=0o700)
        current=source_authority.capture_source(request['token'],source_run,execution['head_sha'],workspace)
        for relative,wanted in self.binding.items():
            if Path(relative).name in ('kernel','bootstrap-renewer'):continue
            if current['source_files'].get('scripts/staging/'+relative)!=wanted:
                raise Blocked('expired_finish_current_helper_source_changed')
        def source_fresh():
            source_authority._head(source_authority._headers(request['token']),execution['head_sha'],time.monotonic()+30)
            expired_recovery.verify_adopted_binding(base,state,self.binding)
        stage,admission=expired_admission.admit_finish(Kube(),base,state,self.code,self.binding,source_fresh)
        def all_fresh():
            _,again=expired_admission.admit_finish(Kube(),base,state,self.code,self.binding,source_fresh,
                admission['get_permission_proof'])
            stable=lambda row:{key:value for key,value in row.items() if key!='checkpoint_sha256'}
            if stable(again)!=stable(admission):raise Blocked('expired_finish_admission_changed')
        return base,state,stage,execution,current,admission,all_fresh

    def _produce_finish_observation(self,request):
        """Owned target pause and a distinct full CLOSED post-apply proof.

        A failed pause is truthful progress, never restored to its old snapshot.
        Public routing remains disabled until both consumers are complete.
        """
        import copy
        import hashlib
        import subprocess
        import expired_finish
        import expired_transport
        import preserved_upload
        from docker_runtime import DockerRuntime
        base,state,stage,execution,current,admission,fresh=self._finish_source_admission(request)
        parent=base/'expired-finish-proofs'
        if not parent.exists():parent.mkdir(mode=0o700)
        fd,_=preserved_upload.directory(parent,modes={0o700});os.close(fd)
        slot=parent/request['nonce'];slot.mkdir(mode=0o700)
        expired_transport.put(slot/'admission.json',admission)
        source={'schema':'voice-expired-finish-source-admission-v1',
            'current_helper':{key:value for key,value in current.items() if key!='source_files'},
            'target':state['target']['tag'],'execution':execution,'helper_binding':self.binding,
            'admission_sha256':preserved_upload.digest(admission)}
        expired_transport.put(slot/'source.json',source)
        def journal(event):state['events'].append({**copy.deepcopy(event),'at':dt.datetime.now(dt.timezone.utc).isoformat()})
        try:
            expired_finish.legacy_finish_guard(state)
            pause=expired_finish.pause_owned(stage,base,state,slot,journal,fresh)
            state['context']=transaction.context(stage);save(base/'checkpoint.json',state)
            expired_transport.put(slot/'paused-checkpoint.json',state)
            kernel=self.code/'nats-known-baseline'/'kernel'
            def checksum(stream,consumer,raw):
                if root_cli.code_binding(self.code)!=self.binding:raise Blocked('expired_finish_helper_changed')
                result=subprocess.run([str(kernel),'--metadata-checksum',stream,consumer],input=raw,
                    stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=5,
                    env={'PATH':'/usr/bin:/bin','HOME':'/nonexistent','LC_ALL':'C'})
                if result.returncode!=0 or len(result.stdout)!=16:raise Blocked('expired_finish_checksum_failed')
                return result.stdout
            def validate(runtime,broker,tree,row,manifest,records):
                # observe_post_apply validates the entire post-apply census,
                # record digest and native branch against immutable root cuts.
                fresh();stage.verify_final_storage()
            expired_finish.legacy_finish_guard(state)
            observation=expired_finish.observe_post_apply(DockerRuntime(base,state['operation']),stage,state,
                slot,validate,checksum,None)
            fresh();stage.verify_final_storage();stage.verified()
            state['context']=transaction.context(stage);save(base/'checkpoint.json',state)
            # The proof owns this verified marker transition as well as pause.
            # Its immutable snapshot is captured only after both are durable.
            pause={**pause,'checkpoint_sha256':preserved_upload.digest(state)}
            expired_transport.put(slot/'verified-pause.json',pause)
            checkpoint=preserved_upload.private_read(base/'checkpoint.json')
            expired_transport.put(slot/'checkpoint.json',state)
            if 'postseal' in observation:
                cipher=encrypted_cut.encrypt_cut(slot,INSTALLED/'recovery',postseal=observation)
            else:cipher=encrypted_cut.encrypt_cut(slot,INSTALLED/'recovery')
            cipher={**cipher,'operation':state['operation'],'run_id':execution['dispatcher_run_id'],
                'head_sha':execution['head_sha'],'challenge':secrets.token_hex(16),
                'created_at':dt.datetime.now(dt.timezone.utc).isoformat()}
            expired_transport.put(slot/'observation-cipher.json',cipher)
            fresh();stage.verify_final_storage()
            if preserved_upload.private_read(base/'checkpoint.json')!=checkpoint:
                raise Blocked('expired_finish_producer_checkpoint_changed')
            expired_transport.put(slot/'producer.json',{'schema':'voice-expired-finish-observation-producer-v1',
                'operation':state['operation'],'started_at':pause['started_at'],
                'completed_at':dt.datetime.now(dt.timezone.utc).isoformat(),'cipher':cipher,
                'original_checkpoint_sha256':hashlib.sha256(checkpoint).hexdigest(),
                'observation_sha256':preserved_upload.digest(observation),
                'pause_sha256':preserved_upload.digest(pause),
                'paused_record_sha256':preserved_upload.digest(preserved_upload.private_json(slot/'pause.json')),
                'paused_checkpoint_sha256':preserved_upload.digest(preserved_upload.private_json(slot/'paused-checkpoint.json',128<<20)),
                'source_sha256':preserved_upload.digest(source)})
            return slot,state,cipher
        except Exception as error:
            state['status']='BLOCKED';state['error']=transaction.safe_error(error)
            try:
                state['fence_status']='VERIFIED' if stage.refence().get('verified') else 'UNKNOWN'
            except Exception:state['fence_status']='UNKNOWN'
            state['context']=transaction.context(stage);save(base/'checkpoint.json',state)
            raise

    def _prepare_expired_finish(self,request):
        import expired_transport
        slot,state,cipher=self._produce_finish_observation(request)
        base=slot.parent.parent;execution=expired_transport.upload.private_json(slot/'source.json')['execution']
        record,paths=expired_transport.export(base,state,slot,execution,cipher)
        # Request success is separate from the still truthful BLOCKED operation.
        # The client can upload these proved artifacts without resetting history.
        return summary(state)|{'status':'WAITING','operation_status':state['status'],
            'operation_error':state.get('error'),'upload_nonce':execution['nonce'],
            'original_cipher_path':paths['original'],'observation_cipher_path':paths['observation'],
            'original_artifact_name':'voice-finish-original-'+state['operation']+'-'+record['artifact_bindings']['original']['challenge'],
            'observation_artifact_name':'voice-finish-observation-'+state['operation']+'-'+record['artifact_bindings']['observation']['challenge'],
            'source_sha':state['target']['tag']}

    def _authorize_expired_finish(self,request):
        import expired_finish
        import expired_transport
        base,state,stage,execution,current,admission,fresh=self._finish_source_admission(request)
        execution['nonce']=request['upload_nonce']
        slot=base/'expired-finish-proofs'/request['upload_nonce']
        record=expired_transport.upload.private_json(slot/'upload.json')
        if record['execution']!=execution:raise Blocked('expired_finish_upload_execution_changed')
        expired_transport.readback(request['token'],base,state,slot,execution,
            {'original':request['original_artifact_id'],'observation':request['observation_artifact_id']})
        self._verify_decrypted_restore(base,state,stage)
        observation=expired_transport.upload.private_json(slot/'rollout-before-manifest.json',128<<20)
        self._verify_decrypted_restore(base,state,stage,payload_base=slot,
            cut={'manifest':observation['manifest'],'census':observation['census'],
                'census_sha256':transaction.canonical(observation['census'])},
            binding=expired_transport.upload.private_json(slot/'observation-cipher.json'))
        fresh()
        authority=expired_finish.commit_authority(base,state,slot,execution,stage=stage,verify_source=fresh)
        result=transaction.finish(Kube(),base,state,guard.protected_receipt(base/'apply-authorization.json'),
            stage.marker['metadata']['resourceVersion'],state['contract'],finish_authority=authority,publication=self._active_release)
        return summary(state)

    def _finish_expired_native(self,request):
        import preserved_upload
        base,state,stage,execution,current,admission,fresh=self._finish_source_admission(request)
        original=state['expired_recovery_authority']['execution']
        if {key:value for key,value in execution.items() if key!='nonce'}!={key:value for key,value in original.items() if key!='nonce'}:
            raise Blocked('expired_native_finish_execution_changed')
        fresh()
        result=transaction.finish(Kube(),base,state,guard.protected_receipt(base/'apply-authorization.json'),
            request['claim_rv'],state['contract'],publication=self._active_release)
        return summary(result)

    def _produce_expired_observation(self,request):
        """Internal root producer; public routing waits for the whole protocol."""
        import copy
        import hashlib
        import subprocess
        import expired_proof
        import expired_recovery
        import nats_root_plan
        import paused_recovery
        import preserved_upload
        from docker_runtime import DockerRuntime
        base,state,stage,execution,current,admission,fresh=self._expired_source_admission(request)
        checkpoint=preserved_upload.private_read(base/'checkpoint.json')
        parent=base/'expired-recovery-proofs'
        if not parent.exists():parent.mkdir(mode=0o700)
        parent_fd,_=preserved_upload.directory(parent,modes={0o700});os.close(parent_fd)
        slot=parent/request['nonce'];slot.mkdir(mode=0o700)
        # Never adopt partial slots, overwrite a record or replay this nonce.
        def immutable(name,row):
            preserved_upload.exclusive(slot/name,json.dumps(row,sort_keys=True,separators=(',',':')).encode())
        immutable('admission.json',admission)
        repair=expired_recovery.repair_inputs(base,admission['input_hashes'],preserved_upload.digest(admission))
        immutable('repair.json',repair)
        immutable('source.json',{'schema':'voice-expired-source-admission-v1',
            'current_helper':{key:value for key,value in current.items() if key!='source_files'},
            'historical_target':state['target']['tag'],'execution':execution,
            'helper_binding':self.binding,'retained_target':admission['target']})
        kernel=self.code/'nats-known-baseline'/'kernel'
        def checksum(stream,consumer,raw):
            if root_cli.code_binding(self.code)!=self.binding:raise Blocked('expired_recovery_helper_changed')
            result=subprocess.run([str(kernel),'--metadata-checksum',stream,consumer],input=raw,
                stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=5,
                env={'PATH':'/usr/bin:/bin','HOME':'/nonexistent','LC_ALL':'C'})
            if result.returncode!=0 or len(result.stdout)!=16:raise Blocked('expired_recovery_checksum_failed')
            return result.stdout
        workspace=INSTALLED/'sources'/state['operation']
        def validate(runtime,broker,tree,row,manifest,records):
            actual=nats_root_plan.closed_copy_preflight(Kube(),workspace,state['bootstrap_enrollment'],
                state['contract'],compiler.decode_yaml,runtime,broker,tree)
            if (actual['plan']!=state['nats_contract'] or actual['binding']!=state['nats_contract_binding']
                or actual['scripts']!=state['nats_target_scripts']):
                raise Blocked('expired_recovery_original_nats_authority_changed')
            # Full records were read using actual retained bootstrap credentials
            # before this callback; failed GET permission rejects the proof.
            fresh();stage.verify_final_storage()
        awaiting=preserved_upload.private_json(base/'recovery-v8-awaiting-checkpoint.json',128<<20)
        started=dt.datetime.now(dt.timezone.utc)
        observation=expired_proof.produce(DockerRuntime(base,state['operation']),stage,state,awaiting,
            slot,checksum,validate)
        cipher=encrypted_cut.encrypt_cut(slot,INSTALLED/'recovery')
        cipher={**cipher,'operation':state['operation'],'run_id':execution['dispatcher_run_id'],
            'head_sha':execution['head_sha'],'challenge':secrets.token_hex(16),
            'created_at':dt.datetime.now(dt.timezone.utc).isoformat()}
        immutable('observation-cipher.json',cipher)
        fresh();stage.verify_final_storage()
        if preserved_upload.private_read(base/'checkpoint.json')!=checkpoint:
            raise Blocked('expired_recovery_producer_checkpoint_changed')
        immutable('producer.json',{'schema':'voice-expired-observation-producer-v1',
            'operation':state['operation'],'started_at':started.isoformat(),
            'completed_at':dt.datetime.now(dt.timezone.utc).isoformat(),
            'observation_sha256':preserved_upload.digest(observation),'cipher':cipher,
            'original_checkpoint_sha256':hashlib.sha256(checkpoint).hexdigest()})
        return slot,observation,cipher

    def _prepare_expired_native(self,request):
        """Complete producer/export response, still private until finish routing."""
        import expired_transport
        base,state=self.state(request['operation'])
        slot,observation,cipher=self._produce_expired_observation(request)
        producer=expired_transport.upload.private_json(slot/'source.json')
        execution=producer['execution']
        record,paths=expired_transport.export(base,state,slot,execution,cipher)
        return {'operation':state['operation'],'phase':'EXPIRED_NATIVE_OBSERVATION',
            'status':'WAITING','fence_status':state['fence_status'],'upload_nonce':execution['nonce'],
            'original_cipher_path':paths['original'],'observation_cipher_path':paths['observation'],
            'original_artifact_name':'voice-expired-original-'+state['operation']+'-'+record['artifact_bindings']['original']['challenge'],
            'observation_artifact_name':'voice-expired-observation-'+state['operation']+'-'+record['artifact_bindings']['observation']['challenge'],
            'deploy_mode':state['target']['mode'],'changed_services':','.join(state['target']['changed_services']),
            'source_sha':state['target']['tag']}

    def _authorize_expired_native(self,request):
        """Actual dual readback and fresh guards precede durable transaction entry."""
        import expired_transport
        import expired_recovery
        # Current master/source/helper admission is repeated for this separate
        # authorize request, while upload proof stays bound to its own nonce.
        base,state,stage,execution,current,admission,all_fresh=self._expired_source_admission(request)
        execution['nonce']=request['upload_nonce']
        slot=base/'expired-recovery-proofs'/request['upload_nonce']
        record=expired_transport.upload.private_json(slot/'upload.json')
        if record['execution']!=execution:raise Blocked('expired_recovery_upload_execution_changed')
        expired_transport.readback(request['token'],base,state,slot,execution,
            {'original':request['original_artifact_id'],'observation':request['observation_artifact_id']})
        self._verify_decrypted_restore(base,state,stage)
        observation=expired_transport.upload.private_json(slot/'rollout-before-manifest.json',128<<20)
        self._verify_decrypted_restore(base,state,stage,payload_base=slot,
            cut={'manifest':observation['manifest'],'census':observation['census'],
                'census_sha256':transaction.canonical(observation['census'])},
            binding=expired_transport.upload.private_json(slot/'observation-cipher.json'))
        all_fresh()
        authority=expired_transport.commit_authority(base,state,slot,execution)
        actual,proof=expired_recovery.runtime_authority(base,state,authority,now=dt.datetime.now(dt.timezone.utc))
        transaction.authorize(Kube(),base,state,state['cut']['manifest']['archive_sha256'],
            state['cut']['manifest_sha256'],state['contract'],
            authorization_deadline=dt.datetime.fromisoformat(actual['expires_at']),expired_authority=actual)
        gid=grp.getgrnam('pmd').gr_gid
        for name in ('apply-authorization.json','apply-manifests.json'):
            os.chown(base/name,0,gid);(base/name).chmod(0o440)
        return summary(state)|{'authorization':str(base/'apply-authorization.json')}
