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

    def state(self,operation):
        base=root_cli.operation_path(str(guard.ROOT/('rollout-'+operation)))
        state=private_json(base/'checkpoint.json')
        if state.get('operation')!=operation or state.get('code_capture')!=self.binding:raise Blocked('bridge_operation_custody_invalid')
        return base,state

    def recover(self,request):
        if request['action']=='status':return self.execute(request)
        operation=request['nonce'][:12] if request['action'] in ('prepare','prepare-rollback') else request.get('operation')
        try:base,state=self.state(operation)
        except (OSError,Blocked):return None
        if request['action']=='finish' and state.get('status')=='PASS':
            self._active_release(state)
            return summary(state)
        if request['action']=='authorize' and state.get('phase')=='PAUSED_APPLY' and state.get('custody',{}).get('artifact_id')==request['artifact_id']:
            return summary(state)|{'authorization':str(base/'apply-authorization.json')}
        if request['action'] in ('prepare','prepare-rollback') and state.get('phase')=='AWAITING_OFF_NODE' and state.get('cipher_binding'):
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

    def _active_release(self,state):
        save(INSTALLED/'active-release.json',{'operation':state['operation'],'namespace_uid':state['context']['expected']['namespace_uid'],
            'pvc_uid':state['context']['expected']['source_claim_uid'],'target_sha256':state['target']['manifest_sha256']})

    def _prepare(self,request,previous=None):
        self._idle();operation=request['nonce'][:12]
        source_run,execution=self._dispatcher(request)
        workspace=INSTALLED/'sources'/operation;workspace.mkdir(mode=0o700)
        approved=source_authority.capture_source(request['token'],source_run,execution['head_sha'],workspace)
        kube=Kube();marker=kube.get('configmap','voice-nats-generation')
        frontends=[s for s in request['changed_services'] if s in ('web','admin','developer-portal')] if request['mode']=='images-only' else ['web','admin','developer-portal']
        stage=RolloutStage.capture(kube,operation,lambda event:None,['voice-'+s for s in frontends])
        policy=private_json(INSTALLED/'policy.json')
        optional={'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host','developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability','minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin'}
        if not set(policy)<=optional or 's3_signing_endpoint' not in policy:raise Blocked('bridge_policy_invalid')
        names=request['changed_services'] if request['mode']=='images-only' else [n.removeprefix('voice-') for n in stage.snapshots if n!=HUB]
        images={}
        for service in names:
            if service not in approved['images'] or service=='nats-hub-config-renderer':raise Blocked('bridge_target_service_invalid')
            name='voice-'+service;row=stage.snapshots[name]
            for container in row['spec']['template']['spec'].get('initContainers',[])+row['spec']['template']['spec']['containers']:
                key=name+'/'+container['name'];image=stage.old_images[key]
                # Only the matching app repository receives a new approved
                # image; sidecar/init images retain the captured running pins.
                if image.split('@',1)[0]==source_authority.REGISTRY+'/'+service:image=approved['images'][service]
                if previous is not None:
                    if key not in previous['target']['images'] or stage.old_images[key]!=previous['target']['images'][key]:raise Blocked('bridge_rollback_current_image_changed')
                    image=previous['context']['old_images'][key]
                images[key]=image
        parameters={**policy,'registry':source_authority.REGISTRY,'tag':approved['source_sha'],
            'mode':request['mode'],'changed_services':request['changed_services'],'images':images,
            'contract':json.loads(public_read(self.code/'nats-known-baseline'/'deployed-contract.json')),
            'enrolled':list(stage.snapshots),'generation':marker['data']['generation'],'dataPVC':marker['data']['dataPVC']}
        build=workspace.parent/(operation+'-build');build.mkdir(mode=0o700)
        parameter_path=build/'parameters.json';save(parameter_path,parameters)
        output=build/('target-'+approved['source_sha']+'.json')
        compiler.main([str(workspace),str(parameter_path),str(output)])
        output.chmod(0o600)
        value=private_json(output)
        def authority_before_fence():source_authority._head(source_authority._headers(request['token']),approved['source_sha'],time.monotonic()+30)
        state=root_cli.prepare_value(value,self.code,self.binding,operation,before_fence=authority_before_fence)
        base=guard.ROOT/('rollout-'+operation)
        state['source_authority']={key:value for key,value in approved.items() if key!='source_files'}
        state['execution_authority']=execution
        if previous is not None:state['rollback_from']={'operation':previous['operation'],'original_release_source':previous['target']['tag'],'images':images}
        save(base/'checkpoint.json',state)
        return self._encrypt(base,state,execution)

    def _encrypt(self,base,state,execution):
        cut=state['cut']
        save(base/'copy-checkpoint.json',{'schema':'nats-rollout-copy-v1','operation':state['operation'],
            'archive_sha256':cut['manifest']['archive_sha256'],'manifest_sha256':cut['manifest_sha256'],
            'census_sha256':cut['census_sha256']})
        cipher=encrypted_cut.encrypt_cut(base,INSTALLED/'recovery')
        state['cipher_binding']={**cipher,'operation':state['operation'],'challenge':secrets.token_hex(16),
            'run_id':execution['dispatcher_run_id'],'head_sha':execution['head_sha'],'created_at':dt.datetime.now(dt.timezone.utc).isoformat()}
        gid=grp.getgrnam('pmd').gr_gid;export=base/'export';export.mkdir(mode=0o750)
        shutil.copyfile(base/'rollout-backup.cms',export/'rollout-backup.cms')
        os.chown(export,0,gid);os.chown(export/'rollout-backup.cms',0,gid);(export/'rollout-backup.cms').chmod(0o440)
        os.chown(base,0,gid);base.chmod(0o750)
        save(base/'checkpoint.json',state)
        return self.copy_result(base,state)

    def execute(self,request):
        if request['action']=='prepare':return self._prepare(request)
        if request['action']=='prepare-rollback':
            base,previous=self.state(request['operation'])
            active=private_json(INSTALLED/'active-release.json')
            if active.get('operation')!=previous['operation'] or active.get('target_sha256')!=previous['target']['manifest_sha256']:raise Blocked('bridge_rollback_latest_release_required')
            transaction.rollback_package(previous) # successful images-only/immutable old pins gate
            changed=previous['input_target']['changed_services']
            return self._prepare({**request,'mode':'images-only','changed_services':changed},previous)
        base,state=self.state(request['operation'])
        if request['action']=='status':return summary(state)
        if request['action']=='authorize':
            receipt=github_custody.verify_artifact(request['token'],state['cipher_binding'],base/'rollout-backup.cms',request['artifact_id'])
            state['custody']=receipt;save(base/'checkpoint.json',state)
            transaction.authorize(Kube(),base,state,state['cut']['manifest']['archive_sha256'],state['cut']['manifest_sha256'],state['contract'])
            gid=grp.getgrnam('pmd').gr_gid
            for name in ('apply-authorization.json','apply-manifests.json'):os.chown(base/name,0,gid);(base/name).chmod(0o440)
            return summary(state)|{'authorization':str(base/'apply-authorization.json')}
        if request['action']=='finish':
            receipt=guard.protected_receipt(base/'apply-authorization.json')
            result=transaction.finish(Kube(),base,state,receipt,request['claim_rv'],state['contract'])
            self._active_release(result)
            return summary(result)
        raise Blocked('bridge_action_invalid')
