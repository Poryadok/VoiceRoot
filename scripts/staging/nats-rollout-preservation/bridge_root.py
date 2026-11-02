"""Installed root actions: approved source, encrypted off-node custody, restart."""
import datetime as dt
import copy
import grp
import hashlib
import json
import re
import os
import stat
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
from nats_active_contract import validate_identity as validate_active_contract_identity

INSTALLED=guard.ROOT/'installed'

def private_json(path):return root_private_json(path,limit=128<<20 if Path(path).name=='checkpoint.json' else 2<<20)
def save(path,row):root_save(path,row,limit=128<<20)


def _copy_checkpoint(state,cut,installation,space_members=None):
    import resource_claim
    installation=resource_claim.require_installation(installation)
    fence=resource_claim.historical_binding(state,installation=installation)
    scoped=fence['schema']=='voice-staging-resource-fence-v2'
    row={'schema':'nats-rollout-copy-v2' if scoped else 'nats-rollout-copy-v1',
        'operation':state['operation'],'archive_sha256':cut['manifest']['archive_sha256'],
        'manifest_sha256':cut['manifest_sha256'],'census_sha256':cut['census_sha256']}
    if scoped:row['installation_sha256']=installation.sha256
    if space_members is not None:row['space_members']=space_members
    return row


def _validate_copy_checkpoint(row,state,cut,installation,space_members=None):
    if row!=_copy_checkpoint(state,cut,installation,space_members):
        raise Blocked('rollout_copy_checkpoint_changed')

def summary(state):
    return {key:state[key] for key in ('operation','phase','status','fence_status','error') if key in state}

def status_summary(state):
    if (state.get("schema") not in ("nats-rollout-root-v1","nats-rollout-root-v2","voice-staging-cohort-preservation-checkpoint-v1")
        or state.get("schema")=="nats-rollout-root-v2" and (not isinstance(state.get("installation_sha256"),str) or not re.fullmatch(r"[a-f0-9]{64}",state["installation_sha256"]))
        or not isinstance(state.get("operation"),str) or not re.fullmatch(r"[a-f0-9]{12}",state["operation"])):
        raise Blocked("bridge_status_checkpoint_invalid")
    result=summary(state)
    for key,value in result.items():
        if not isinstance(value,str) or len(value)>128 or key!="operation" and not re.fullmatch(r"[A-Za-z0-9_.:-]+",value):
            raise Blocked("bridge_status_summary_invalid")
    if not result.get("status"):raise Blocked("bridge_status_checkpoint_invalid")
    return result

def record_source_authority(session,approved,execution,*,code=None,binding=None):
    """Record only digests produced by this root-owned source capture."""
    session.check()
    if not re.fullmatch(r'[a-f0-9]{12}',session.operation):raise Blocked('bridge_source_capture_invalid')
    import resource_claim
    resource_claim.require_session(session,installation=session.installation)
    if code is None or not isinstance(binding,dict) or transaction.canonical(binding)!=session.helper:
        raise Blocked('bridge_source_helper_custody_required')
    code=Path(code)
    if root_cli.code_binding(code)!=binding:raise Blocked('bridge_source_helper_changed')
    code_info=code.lstat()
    row={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':session.operation,
         'context':copy.deepcopy(session.context),'helper_sha256':session.helper,
         'approved_sha256':transaction.canonical(approved),'execution_sha256':transaction.canonical(execution)}
    raw=json.dumps(row,sort_keys=True,separators=(',',':')).encode()
    parent=os.open(INSTALLED,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:
        pinfo=os.fstat(parent)
        if pinfo.st_uid!=0 or pinfo.st_mode&0o022:raise Blocked('bridge_source_capture_directory_invalid')
        try:os.mkdir('source-authority',0o700,dir_fd=parent);os.fsync(parent)
        except FileExistsError:pass
        dfd=os.open('source-authority',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=parent)
        try:
            info=os.fstat(dfd)
            if not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o077:
                raise Blocked('bridge_source_capture_directory_invalid')
            # Capture the actual already-approved data before publishing its
            # create-only receipt. Early root readers cannot reconstruct it
            # from a target, a Secret or a later optional checkpoint field.
            os.mkdir(session.operation,0o700,dir_fd=dfd)
            os.fsync(dfd)
            custody=INSTALLED/'source-authority'/session.operation
            save(custody/'approved.json',approved)
            save(custody/'execution.json',execution)
            save(custody/'code-custody.json',{'root':str(code),'manifest':copy.deepcopy(binding),
                'directory':{'dev':code_info.st_dev,'ino':code_info.st_ino}})
            fd=os.open(session.operation+'.json',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=dfd)
            with os.fdopen(fd,'wb') as output:
                output.write(raw);output.flush();os.fsync(output.fileno())
            os.fsync(dfd)
        finally:os.close(dfd)
    finally:os.close(parent)
    session.check()
    return row

def actor_after_realtime_bootstrap(before,after,old_secret,new_secret):
    """Allow only the bootstrap's proved app-secret RV change in actor proof."""
    if before is None or before.get('story_schema') is None:
        if before==after:return after
        raise Blocked('realtime_bootstrap_actor_changed')
    try:
        expected=copy.deepcopy(before)
        profile=expected['story_schema']
        schema=profile['baseline'] if profile.get('schema')=='voice-story-baseline4-target8-composition-v1' else profile
        first=schema['database_secret'];second=schema['authority']['secret']
        if (first['name']!='voice-app-secrets' or first['uid']!=old_secret['uid']
            or second['uid']!=old_secret['uid'] or new_secret['uid']!=old_secret['uid']
            or first['resourceVersion']!=old_secret['resourceVersion']
            or second['resourceVersion']!=old_secret['resourceVersion']
            or not isinstance(new_secret['resourceVersion'],str) or not new_secret['resourceVersion']):
            raise ValueError()
        first['resourceVersion']=second['resourceVersion']=new_secret['resourceVersion']
        candidate=expected.get('compatible_story_candidate')
        if candidate is not None:candidate['schema_sha256']=transaction.canonical(profile)
        if expected!=after:raise ValueError()
        return after
    except (KeyError,TypeError,ValueError):raise Blocked('realtime_bootstrap_actor_changed') from None

def verify_bootstrap_candidate_images(session,images,renderer_authority,first_install,ordinary_target=None):
    """Reject candidate pins outside the independently selected source images."""
    allowed=dict(images);renderer={}
    if renderer_authority is not None:
        from stage_runtime import HUB
        try:
            spec=renderer_authority['descriptor']['target_template']['spec']
            containers=spec.get('initContainers',[])+spec['containers']
            renderer={HUB+'/'+c['name']:c['image'] for c in containers}
            if len(renderer)!=len(containers):raise ValueError()
        except (KeyError,TypeError,ValueError):raise Blocked('realtime_bootstrap_candidate_invalid') from None
        allowed.update(renderer)
    candidate=session.context.get('images')
    if (not isinstance(candidate,dict) or not candidate
        or any(key not in allowed or allowed[key]!=value for key,value in candidate.items())):
        raise Blocked('realtime_bootstrap_candidate_changed')
    if first_install:
        expected={key:value for key,value in allowed.items() if key.startswith('voice-gameintegration/')}
        expected.update(renderer)
        if not any(key.startswith('voice-gameintegration/') for key in expected) or candidate!=expected:
            raise Blocked('realtime_bootstrap_candidate_changed')
    elif ordinary_target is not None:
        if candidate!={**ordinary_target,**renderer}:raise Blocked('realtime_bootstrap_candidate_changed')

def capture_cohort_authorities(actions,request,approved,workspace,stage,contract):
    """Capture the existing actor and live NATS authorities before the cut."""
    import actor_root
    import nats_root_plan
    import resource_claim
    import commands
    session=actions.tt_session
    if (type(actions) is not Actions or type(session) is not resource_claim.Session
        or commands.authority_guard!=session.check or session.check().get('phase')!='prepare'
        or request.get('mode') not in ('full','app-only') or stage.operation!=session.operation
        or Path(workspace)!=INSTALLED/'sources'/session.operation or approved.get('source_sha')!=session.context['commit']):
        raise Blocked('cohort_preflight_authority_invalid')
    baseline=contract.get('consumer_pairs')
    try:active=private_json(INSTALLED/'active-contract.json')
    except FileNotFoundError:active=None
    if active is not None:
        validate_active_contract_identity(active,stage.expected,baseline);contract=active['contract']
    try:enrollment=private_json(guard.ROOT/'bootstrap-enrollment-v4.json')
    except FileNotFoundError:raise Blocked('enrolled_bootstrap_selection_refused')
    previous=private_json(guard.ROOT/'bootstrap-enrollment.json')
    from nats_contract_plan import digest as enrollment_digest
    if (not isinstance(enrollment,dict) or enrollment.get('schema') not in ('voice-nats-bootstrap-enrollment-v4','voice-nats-bootstrap-enrollment-scoped-v4')
        or enrollment.get('previous_enrollment')!=previous
        or enrollment.get('previous_enrollment_sha256')!=enrollment_digest(previous)):
        raise Blocked('enrolled_bootstrap_selection_refused')
    nats=nats_root_plan.preflight(stage.kube,workspace,enrollment,contract,compiler.decode_yaml,installation=actions.installation)
    names=[name.removeprefix('voice-') for name in stage.snapshots if name!=HUB]
    if 'story' in names:
        composition=_cohort_story_composition(actions,request,approved,workspace)
        actor=actor_root.preflight(stage.kube,workspace,actions.code,actions.binding,stage,names,compiler.decode_yaml,
            story_composition=composition,installation=actions.installation)
    else:actor=actor_root.preflight(stage.kube,workspace,actions.code,actions.binding,stage,names,compiler.decode_yaml,installation=actions.installation)
    session.check()
    return {'schema':'voice-staging-cohort-preflight-v1','operation':session.operation,
            'services':names,'actor':actor,'nats':nats,'enrollment':enrollment,'contract':contract}


def _cohort_story_composition(actions,request,approved,workspace):
    """Bind pre-cut Story4/source8 descriptors to actual protected source inputs."""
    session=actions.tt_session
    parent=INSTALLED/'prerequisites'/session.operation
    prepared=private_json(parent/'prepared.json');parent_plan=prepared.get('plan') if type(prepared) is dict else None
    execution=private_json(parent/'execution.json')
    receipt=private_json(INSTALLED/'source-authority'/(session.operation+'.json'))
    from apply import digest
    import staging_prerequisites
    if (type(prepared) is not dict or set(prepared)!={'schema','plan','request','gis_authority_sha256'}
        or prepared.get('schema')!='voice-staging-prerequisites-prepared-v1'
        or prepared.get('request')!={'mode':request.get('mode'),'changed_services':request.get('changed_services'),
                                     'gis_first_install':request.get('gis_first_install') is True}
        or not isinstance(prepared.get('gis_authority_sha256'),str)
        or not re.fullmatch(r'[a-f0-9]{64}',prepared['gis_authority_sha256'])
        or type(parent_plan) is not dict or actions.installation!=session.installation
        or receipt!={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':session.operation,
        'context':session.context,'helper_sha256':session.helper,'approved_sha256':digest(approved),
        'execution_sha256':digest(execution)}
        or parent_plan.get('schema')!='voice-staging-prerequisites-plan-v3'
        or parent_plan.get('installation_sha256')!=session.installation.sha256
        or parent_plan.get('operation')!=session.operation or parent_plan.get('context')!=session.context
        or parent_plan.get('helper_sha256')!=session.helper
        or parent_plan.get('source_capture_sha256')!=digest(receipt)
        or parent_plan.get('approved_sha256')!=digest(approved) or parent_plan.get('execution_sha256')!=digest(execution)):
        raise Blocked('cohort_preflight_source_changed')
    staging_prerequisites.validate_plan(parent_plan,session)
    def source_read(relative):
        name=str(relative).replace('\\','/')
        path=Path(workspace)/relative
        if Path(relative).is_absolute() or '..' in Path(relative).parts:raise Blocked('cohort_preflight_source_changed')
        encrypted_cut.directory(path.parent)
        raw=public_read(path)
        if hashlib.sha256(raw).hexdigest()!=approved['source_files'].get(name):raise Blocked('cohort_preflight_source_changed')
        return raw
    migration_plan=compiler.compile_migrations(workspace,request['mode'],source_read)
    story_binding={'operation':session.operation,'source_sha':approved['source_sha'],
        'source_capture_sha256':digest(receipt),'helper_sha256':session.helper,
        'context_sha256':digest(session.context),'migration_plan_sha256':digest(migration_plan)}
    session.check()
    return migration_plan,story_binding


class Actions:
    def resource_scope(self,request):
        import resource_claim
        return resource_claim.root_scope(self,request)

    def __init__(self,code,*,installation=None):
        import resource_claim
        self._installation=resource_claim.require_installation(installation)
        self.code=Path(code);self.binding=root_cli.code_binding(self.code)

    @property
    def installation(self):return self._installation

    def _report(self,name,status):
        observer=getattr(self,'_observer',None)
        if observer is not None:observer(name,status)

    def _require_authorization_operation(self,operation):
        session=getattr(self,'tt_session',None)
        if session is not None and operation!=session.operation:
            raise Blocked('staging_authorization_operation_changed')
        return session

    def _require_authorization_binding(self,state):
        import resource_claim
        session=self._require_authorization_operation(state.get('operation'))
        fence=state.get('resource_fence')
        if isinstance(fence,dict) and fence.get('schema')=='voice-staging-resource-fence-v2':
            if type(session) is not resource_claim.Session or not session.admitted:
                raise Blocked('staging_installation_binding_changed')
            resource_claim.historical_binding(state,installation=session.installation)
            session.require_binding(fence)
        return session

    def execute_observed(self,request,observe):
        if getattr(self,'_observer',None) is not None:raise Blocked('bridge_operation_already_observed')
        self._observer=observe
        try:return self.execute(request)
        finally:self._observer=None

    def state(self,operation):
        base=root_cli.operation_path(str(guard.ROOT/('rollout-'+operation)))
        state=private_json(base/'checkpoint.json')
        if state.get('operation')!=operation:raise Blocked('bridge_operation_custody_invalid')
        session=getattr(self,'tt_session',None)
        if state.get('schema')=='voice-staging-cohort-preservation-checkpoint-v1':
            import resource_claim
            import staging_cohort_preservation as cohort
            plan=state.get('plan')
            if (type(plan) is not dict or set(plan)!=cohort.PLAN_FIELDS or plan.get('schema')!=cohort.SCHEMA
                or plan.get('operation')!=operation or plan.get('helper_sha256')!=resource_claim.digest(self.binding)):
                raise Blocked('cohort_status_checkpoint_changed')
            resource_claim.validate(plan.get('context'))
            if session is not None:
                cohort.validate_plan(plan,session)
                cohort.load_plan(session)
            return base,state
        if session is not None:
            if operation==session.operation:session.require_binding(state.get('resource_fence'))
            else:
                import resource_claim
                resource_claim.historical_binding(state,installation=self.installation)
        if state.get('operator_retirement') is not None:
            from operator_retirement import verify_terminal
            verify_terminal(base,state)
            raise Blocked('bridge_operation_operator_retired')
        import paused_recovery
        paused_recovery.verify_adopted_binding(base,state,self.binding)
        return base,state

    def validate_cached(self,request,result):
        session=getattr(self,"tt_session",None)
        protected=request.get("action")=="prepare" and request.get("mode") in ("full","app-only")
        if session is None:
            if protected:raise Blocked("cohort_cache_session_required")
            return
        row=session.journal.read()
        if not protected and not (request.get("action")=="authorize" and row.get("cohort_preservation_sha256") is not None):return
        import staging_cohort_preservation as cohort
        expected=(cohort.prepared_replay_result(session,request) if protected
            else cohort.final_authorization_result(session,request))
        if result!=expected:raise Blocked("cohort_cache_result_changed")

    def recover(self,request):
        if request['action']=='status':return self.execute(request)
        session=getattr(self,"tt_session",None)
        if session is not None:
            row=session.journal.read()
            protected=request.get("action")=="prepare" and request.get("mode") in ("full","app-only")
            if protected or request.get("action")=="authorize" and row.get("cohort_preservation_sha256") is not None:
                import staging_cohort_preservation as cohort
                return (cohort.prepared_replay_result(session,request) if protected
                    else cohort.final_authorization_result(session,request))
        operation=request['nonce'][:12] if request['action'] in ('prepare','prepare-rollback','prepare-repair') else request.get('operation')
        try:base,state=self.state(operation)
        except (OSError,Blocked):return None
        if request['action']=='authorize':
            import migrations
            migrations.require_preserving_admission(state.get('target',{}).get('mode'),state.get('migrations'))
        if request['action']=='resume-cipher-upload':
            import preserved_upload
            _,continuity=preserved_upload.verify_helper_continuity(base,state,self.binding)
            record,_,_=preserved_upload.load(base,request['nonce'],state,continuity)
            if record['upload_execution']['dispatcher_run_id']!=request['run_id']:
                raise Blocked('preserved_upload_replay_run_changed')
            return preserved_upload.result(base,state,record)
        if request['action']=='finish' and state.get('status') in ('PASS','PROVISIONAL_READY'):
            if state.get('status')=='PROVISIONAL_READY' and (base/'provisional-child.json').exists():
                raise Blocked('provisional_child_already_bound')
            self._active_release(state)
            return summary(state)
        if request['action']=='accept-provisional' and state.get('status')=='PASS' and state.get('ci_acceptance',{}).get('verified') is True:
            return summary(state)
        if request['action']=='authorize' and state.get('phase')=='PAUSED_APPLY' and state.get('custody',{}).get('artifact_id')==request['artifact_id']:
            return summary(state)|{'authorization':str(base/'apply-authorization.json')}
        if request['action'] in ('prepare','prepare-rollback','prepare-repair','resume-cold-backup') and state.get('phase')=='AWAITING_OFF_NODE' and state.get('cipher_binding'):
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

    def _idle(self,*,admitted_parent=None):
        rows=list(guard.ROOT.glob('rollout-*'))
        if len(rows)>1000:raise Blocked('bridge_operation_inventory_bound')
        for base in rows:
            if root_cli.rollout_directory_kind(base)=='capture':continue
            if not (base/'checkpoint.json').exists():raise Blocked('bridge_incomplete_operation_present')
            row=private_json(base/'checkpoint.json')
            if row.get('status') not in ('PASS','ROLLED_BACK'):
                if row.get('status')=='PROVISIONAL_READY':
                    import provisional
                    if provisional.covered(base,row,admitted=admitted_parent):continue
                from operator_retirement import verify_terminal
                verify_terminal(base,row)

    def _dispatcher(self,request):
        headers=source_authority._headers(request['token']);deadline=time.monotonic()+30
        run=source_authority._get(source_authority.API+'/actions/runs/'+str(request['run_id']),headers,deadline)
        if run['id']!=request['run_id'] or run['repository']['full_name']!=source_authority.REPO or run['head_repository']['full_name']!=source_authority.REPO or run['head_branch']!='master' or run['status']!='in_progress':raise Blocked('bridge_dispatcher_authority_invalid')
        workflow=run['workflow_id'];path=run['path'].split('@',1)[0]
        if workflow==source_authority.WORKFLOW and path=='.github/workflows/ci.yml' and run['event']=='push':source_run=run['id']
        elif workflow==263689731 and path=='.github/workflows/staging-deploy.yml' and run['event']=='workflow_dispatch':
            if not re.fullmatch(r'[a-f0-9]{40}',run['head_sha']):raise Blocked('bridge_dispatcher_authority_invalid')
            result=source_authority._get(source_authority.API+'/actions/workflows/'+str(source_authority.WORKFLOW)+'/runs?branch=master&event=push&per_page=100&head_sha='+run['head_sha'],headers,deadline)
            candidates=[r for r in result['workflow_runs'] if r['head_sha']==run['head_sha'] and r['event']=='push' and r['head_branch']=='master' and r['workflow_id']==source_authority.WORKFLOW and r['status'] in ('in_progress','completed')]
            if not candidates:raise Blocked('bridge_approved_ci_source_missing')
            source_run=max(candidates,key=lambda r:r['id'])['id']
        else:raise Blocked('bridge_dispatcher_authority_invalid')
        if 'source_sha' in request and request['source_sha']!=run['head_sha']:raise Blocked('bridge_dispatcher_source_changed')
        return source_run,{'dispatcher_run_id':run['id'],'run_attempt':run['run_attempt'],'head_sha':run['head_sha'],'event':run['event'],'workflow_id':workflow,'path':path,'repository':source_authority.REPO}

    def _active_release(self,state,*,authority_guard=None):
        import resource_claim
        if authority_guard is not None:authority_guard()
        row={'operation':state['operation'],'namespace_uid':state['context']['expected']['namespace_uid'],
            'pvc_uid':state['context']['expected']['source_claim_uid'],'target_sha256':state['target']['manifest_sha256']}
        if state.get('schema')=='nats-rollout-root-v2':
            if (state.get('installation_sha256')!=self.installation.sha256
                or state['target'].get('installation_sha256')!=self.installation.sha256
                or not isinstance(state.get('operation'),str) or not re.fullmatch(r'[a-f0-9]{12}',state['operation'])):
                raise Blocked('staging_installation_binding_changed')
            import commands
            session=getattr(self,'tt_session',None)
            if (type(session) is not resource_claim.Session or not session.admitted
                or session.installation!=self.installation or session.operation!=state['operation']
                or commands.authority_guard!=session.check):raise Blocked('staging_installation_binding_changed')
            session.require_binding(state.get('resource_fence'))
            resource_claim.historical_binding(state,installation=self.installation)
            if private_json(guard.ROOT/('rollout-'+state['operation'])/'checkpoint.json')!=state:
                raise Blocked('staging_active_release_checkpoint_changed')
            row.update(schema='voice-active-release-v1',installation_sha256=self.installation.sha256)
        elif state.get('schema')=='nats-rollout-root-v1':
            resource_claim.require_production(self.installation)
        else:raise Blocked('staging_installation_binding_changed')
        try:previous=private_json(INSTALLED/'active-release.json')
        except FileNotFoundError:previous=None
        if previous is not None:resource_claim.validate_active_release(previous,installation=self.installation)
        successor=state.get('gis_existing_update')
        if successor is not None:
            import gis_existing_update as existing
            if state.get('status')!='PASS':raise Blocked('gis_existing_publication_not_pass')
            saved=private_json(guard.ROOT/('rollout-'+state['operation'])/'checkpoint.json')
            if saved!=state:raise Blocked('gis_existing_publication_checkpoint_changed')
            receipt=successor['receipt'];anchor=receipt['anchor']
            reference={'operation':state['operation'],'receipt_sha256':receipt['receipt_sha256']}
            if previous is None:raise Blocked('gis_existing_publication_stale')
            if previous.get('operation')==state['operation']:
                if (previous.get('gis_first_install_transition')!=anchor
                    or previous.get('gis_latest_transition')!=reference
                    or any(previous.get(key)!=value for key,value in row.items())):
                    raise Blocked('gis_existing_publication_stale')
                return
            if previous!=state.get('gis_expected_active'):
                raise Blocked('gis_existing_publication_stale')
            if (previous.get('gis_first_install_transition')!=anchor
                    or receipt.get('predecessor')!=previous.get('gis_latest_transition',anchor)):
                raise Blocked('gis_existing_publication_stale')
            original=private_json(guard.ROOT/('rollout-'+anchor['operation'])/'checkpoint.json')
            existing.load_successors(Kube(),anchor,original['gis_first_install_transition_receipt'],original,reference)
            row['gis_first_install_transition']=copy.deepcopy(anchor)
            row['gis_latest_transition']=reference
            save(INSTALLED/'active-release.json',row)
            return
        transition=state.get('gis_first_install_transition_receipt')
        if transition is not None:
            reference={'operation':state['operation'],
                'receipt_sha256':transition['receipt_sha256']}
            previous=previous or {}
            existing=previous.get('gis_first_install_transition')
            if existing is not None:
                if existing!=reference or any(previous.get(key)!=value for key,value in row.items()):
                    raise Blocked('gis_first_install_publication_stale')
                return
            row['gis_first_install_transition']=reference
        else:
            previous=previous or {}
            if 'gis_first_install_transition' in previous:
                if all(previous.get(key)==value for key,value in row.items()):return
                raise Blocked('gis_existing_publication_stale')
        save(INSTALLED/'active-release.json',row)

    def _previous_gis_transition(self,kube):
        try:active=private_json(INSTALLED/'active-release.json')
        except FileNotFoundError:return None
        reference=active.get('gis_first_install_transition')
        if reference is None:return None
        if (not isinstance(reference,dict) or set(reference)!={'operation','receipt_sha256'}
            or not re.fullmatch(r'[a-f0-9]{12}',str(reference.get('operation','')))
            or not re.fullmatch(r'[a-f0-9]{64}',str(reference.get('receipt_sha256','')))):
            raise Blocked('gis_first_install_transition_reference_invalid')
        prior=private_json(guard.ROOT/('rollout-'+reference['operation'])/'checkpoint.json')
        receipt=prior.get('gis_first_install_transition_receipt')
        if not isinstance(receipt,dict) or receipt.get('receipt_sha256')!=reference['receipt_sha256']:
            raise Blocked('gis_first_install_transition_reference_changed')
        import gis_first_install as gis
        binding=gis.validate_next_operation(receipt,prior,kube)
        return {**binding,'receipt':copy.deepcopy(receipt)}

    def _capture_source(self,request,previous=None,repair=None):
        """Capture source receipts after admission; performs no rollout or migration."""
        operation=request['nonce'][:12]
        source_run,execution=self._dispatcher(request)
        provisional_mode=request.get('provisional_staging') is True
        if provisional_mode or repair is not None:
            if (execution['event']!='workflow_dispatch' or execution['workflow_id']!=263689731
                or request['mode']!='images-only' or previous is not None):raise Blocked('provisional_dispatch_required')
        workspace=INSTALLED/'sources'/operation;workspace.mkdir(mode=0o700)
        self._report('source-capture','STARTED')
        approved=(source_authority.capture_source(request['token'],source_run,execution['head_sha'],workspace,provisional_staging=True)
            if provisional_mode else source_authority.capture_source(request['token'],source_run,execution['head_sha'],workspace))
        return source_run,execution,provisional_mode,workspace,approved

    def _prepare_cohort(self,request):
        """Protected source capture and real cohort producer."""
        import commands
        import resource_claim
        import staging_cohort_preservation
        import staging_prerequisites
        session=getattr(self,"tt_session",None)
        if (type(self) is not Actions or type(session) is not resource_claim.Session
            or commands.authority_guard!=session.check or not session.admitted
            or request.get("action")!="prepare" or request.get("mode") not in ("full","app-only")
            or request.get("provisional_staging") is True or request.get("operation") is not None
            or request.get("nonce","")[:12]!=session.operation
            or request.get("resource_context")!=session.context
            or request.get("source_sha")!=session.context.get("commit")
            or resource_claim.digest(self.binding)!=session.helper):
            raise Blocked("cohort_prepare_session_invalid")
        if session.installation!=self.installation:
            raise RuntimeError('staging_installation_binding_changed')
        row=session.check()
        if row.get("phase")!="prepare" or row.get("target_sha256") is not None:
            raise Blocked("cohort_prepare_session_invalid")
        staging_cohort_preservation.record_prepare_request(session,request)
        self._idle()
        source_run,execution=self._dispatcher(request)
        encrypted_cut.directory(INSTALLED/"sources")
        workspace=INSTALLED/"sources"/session.operation
        workspace.mkdir(mode=0o700)
        self._report("source-capture","STARTED")
        approved=source_authority.capture_source(request["token"],source_run,execution["head_sha"],workspace)
        code=Path(self.code);installed=guard.ROOT/"installed"/"code"
        if code!=installed and (code.parent!=guard.ROOT or not re.fullmatch(r"rollout-code-[a-f0-9]{32}",code.name)
                                or root_cli.rollout_directory_kind(code)!="capture"):
            raise Blocked("cohort_helper_source_changed")
        if (type(approved) is not dict or type(approved.get("source_files")) is not dict
            or approved.get("source_sha")!=request["source_sha"]
            or execution.get("head_sha")!=request["source_sha"]
            or root_cli.code_binding(self.code)!=self.binding
            or not staging_prerequisites.REQUIRED_HELPERS.issubset(self.binding)
            or not set(staging_prerequisites.P2_BINARY_PINS).issubset(self.binding)):
            raise Blocked("cohort_source_identity_changed")
        encrypted_cut.directory(workspace)
        if stat.S_IMODE(workspace.lstat().st_mode)!=0o700:raise Blocked("cohort_source_workspace_invalid")
        for relative,wanted in self.binding.items():
            if relative in staging_prerequisites.P2_BINARY_PINS:
                if wanted!=staging_prerequisites.P2_BINARY_PINS[relative]:
                    raise Blocked("cohort_helper_source_changed")
                continue
            path=workspace/"scripts"/"staging"/relative
            if approved["source_files"].get("scripts/staging/"+relative)!=wanted:
                raise Blocked("cohort_helper_source_changed")
            for parent in path.parents:
                if parent==workspace:break
                info=parent.lstat()
                if not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o022:
                    raise Blocked("cohort_helper_source_changed")
            if root_cli.public_code_sha(path)!=wanted:raise Blocked("cohort_helper_source_changed")
        session.check()
        record_source_authority(session,approved,execution,code=self.code,binding=self.binding)
        self._report("source-capture","COMPLETE")
        return staging_cohort_preservation.prepare(self,request,approved,execution,workspace)
    def _prepare(self,request,previous=None,repair=None,*,cohort_ref=None):
        import migrations
        if cohort_ref is None:
            migrations.require_preserving_admission(request.get('mode'))
            if repair is None:self._idle()
            else:self._idle(admitted_parent=repair['operation'])
            operation=request['nonce'][:12]
            source_run,execution,provisional_mode,workspace,approved=self._capture_source(request,previous,repair)
            if request['mode'] in ('full','app-only') and not provisional_mode and repair is None and previous is None:
                if getattr(self,'tt_session',None) is None:raise Blocked('realtime_bootstrap_session_required')
                record_source_authority(self.tt_session,approved,execution,code=self.code,binding=self.binding)
        else:
            import staging_cohort_preservation
            if previous is not None or repair is not None or request.get('provisional_staging') is True:
                raise Blocked('cohort_preserved_prepare_invalid')
            session=self.tt_session;loaded=staging_cohort_preservation.load_prepared(session)
            if (staging_cohort_preservation.load_qualified(session)!=cohort_ref
                or any(request.get(key)!=loaded['request'][key] for key in ('mode','changed_services'))
                or (request.get('gis_first_install') is True)!=loaded['request']['gis_first_install']):
                raise Blocked('cohort_preserved_prepare_invalid')
            staging_cohort_preservation.require_continuation(session)
            operation=session.operation;approved=loaded['approved'];execution=loaded['execution'];workspace=Path(loaded['workspace'])
            provisional_mode=False
        lifecycle=None
        if provisional_mode or repair is not None:
            import provisional
            for relative,wanted in self.binding.items():
                if relative in ('nats-known-baseline/kernel','nats-rollout-preservation/bootstrap-renewer',
                    'nats-rollout-preservation/story-current4-probe','nats-rollout-preservation/story-restored8-probe'):continue
                if approved['source_files'].get('scripts/staging/'+relative)!=wanted:
                    raise Blocked('provisional_helper_source_changed')
            lifecycle={'source_authority':{k:v for k,v in approved.items() if k!='source_files'},'execution_authority':execution}
            if provisional_mode:lifecycle['provisional']=provisional.admission(approved,execution,self.binding)
            if repair is not None:lifecycle['repair_parent']=provisional.parent_reference(repair)
        self._report('source-capture','COMPLETE')
        prerequisites=None
        if request['mode'] in ('full','app-only') and not provisional_mode and repair is None and previous is None:
            import staging_prerequisites
            staging_prerequisites.run_prepare(self,request,approved,execution,workspace,preservation_ref=cohort_ref)
        self._report('workload-capture','STARTED')
        kube=Kube();marker=kube.get('configmap','voice-nats-generation')
        frontends=[s for s in request['changed_services'] if s in ('web','admin','developer-portal')] if request['mode']=='images-only' else ['web','admin','developer-portal']
        full_story=previous is not None and previous['input_target']['mode']=='full'
        renderer_selected=request['mode']=='full' or previous is not None and not full_story and previous.get('renderer_authority') is not None
        import source_plan
        publisher_selected=request['mode'] in ('full','app-only') and source_plan.principal_publisher_source(workspace,compiler.decode_yaml)
        gis_binding=None
        if request.get('gis_first_install') is not True:
            import gis_existing_update
            gis_binding=(gis_existing_update.load_authority(kube) if cohort_ref is None else staging_prerequisites.load_gis_authority(self.tt_session))
            if gis_binding is not None and (provisional_mode or repair is not None or previous is not None):
                raise Blocked('gis_existing_lifecycle_path_unsupported')
        gis_extra=['voice-gameintegration'] if gis_binding is not None else []
        gis_capture_args={'gis_binding':gis_binding} if gis_binding is not None else {}
        stage=(RolloutStage.capture(kube,operation,lambda event:None,['voice-'+s for s in frontends]+([HUB] if renderer_selected else [])+
            ([source_plan.PRINCIPAL_PUBLISHER] if publisher_selected else [])+gis_extra,**gis_capture_args) if cohort_ref is None
            else staging_cohort_preservation.capture_final_stage(self.tt_session))
        if request['mode'] in ('full','app-only') and not provisional_mode and repair is None and previous is None:
            self.tt_session.capture_prerequisites(stage)
            prerequisites=staging_prerequisites.load_capture(self.tt_session,actual=stage.old_images)
        self._report('workload-capture','COMPLETE')
        gis_transition=None
        if (gis_binding is None and request.get('gis_first_install') is not True and request['mode'] in ('full','app-only')
            and not provisional_mode and repair is None and previous is None):
            gis_transition=self._previous_gis_transition(kube)
        self._report('policy-validation','STARTED')
        policy=private_json(INSTALLED/'policy.json')
        optional={'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host','developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability','minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin','release_policy'}
        if not set(policy)<=optional or 's3_signing_endpoint' not in policy:raise Blocked('bridge_policy_invalid')
        if 'release_policy' in policy:
            import release_policy_wiring
            try:release_policy_wiring.validate(policy['release_policy'],approved['source_sha'],guard.NS)
            except ValueError as exc:raise Blocked('bridge_release_policy_invalid') from exc
        self._report('policy-validation','COMPLETE')
        self._report('image-selection','STARTED')
        names=request['changed_services'] if request['mode']=='images-only' else [n.removeprefix('voice-') for n in stage.snapshots if n!=HUB]
        images={};image_deadline=time.monotonic()+600
        for service in names:
            if service=='p3-principal-jwks':
                if not publisher_selected:raise Blocked('bridge_principal_publisher_not_selected')
                name=source_plan.PRINCIPAL_PUBLISHER;row=stage.snapshots[name]
                spec=row['spec']['template']['spec']
                if spec.get('initContainers') or [c['name'] for c in spec['containers']]!=['jwks']:
                    raise Blocked('bridge_principal_publisher_container_changed')
                key=name+'/jwks';image=source_plan.principal_publisher_image(stage.old_images[key])
                if previous is not None:
                    if previous['target']['images'].get(key)!=image or previous['context']['old_images'].get(key)!=image:
                        raise Blocked('bridge_principal_publisher_rollback_image_changed')
                images[key]=image
                continue
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
                    image=(previous['context']['old_images'][key] if not full_story or key=='voice-story/story' else stage.old_images[key])
                images[key]=image
        if request.get('gis_first_install') is True:
            image=approved['images'].get('gameintegration')
            if not isinstance(image,str) or '@sha256:' not in image:
                raise Blocked('gis_first_install_source_image_invalid')
            images['voice-gameintegration/gameintegration']=image
        self._report('image-selection','COMPLETE')
        self._report('contract-selection','STARTED')
        contract=json.loads(public_read(self.code/'nats-known-baseline'/'deployed-contract.json'))
        baseline_pairs=contract.get('consumer_pairs')
        try:active_contract=private_json(INSTALLED/'active-contract.json')
        except FileNotFoundError:active_contract=None
        if active_contract is not None:
            validate_active_contract_identity(active_contract,stage.expected,baseline_pairs)
            contract=active_contract['contract']
        parameters={**policy,'registry':source_authority.REGISTRY,'tag':approved['source_sha'],
            'mode':request['mode'],'changed_services':request['changed_services'],'images':images,
            'contract':contract,
            'enrolled':list(stage.snapshots),'generation':marker['data']['generation'],'dataPVC':marker['data']['dataPVC']}
        if request.get('gis_first_install') is True:
            if request['mode'] not in ('full','app-only') or 'gameintegration' not in approved['images']:
                raise Blocked('gis_first_install_admission_invalid')
            if 'voice-gameintegration' in parameters['enrolled']:
                raise Blocked('gis_first_install_identity_present')
            # The separately validated add-only pair is the only not-yet-live
            # deployment admitted to the source compiler.
            parameters['enrolled'].append('voice-gameintegration')
            parameters['gis_first_install']=True
        self._report('contract-selection','COMPLETE')
        self._report('nats-preflight','STARTED')
        renderer_authority=None
        import actor_root
        if prerequisites is not None and staging_prerequisites.load_capture(self.tt_session,actual=stage.old_images)!=prerequisites:
            raise Blocked('staging_prerequisites_capture_changed')
        if cohort_ref is not None:
            preserved_preflight=staging_cohort_preservation.load_preflight(self.tt_session)
        nats_authority=None
        if cohort_ref is not None:
            enrollment=preserved_preflight['enrollment'];nats_authority=preserved_preflight['nats']
            if preserved_preflight['contract']!=parameters['contract'] or nats_authority is None:
                raise Blocked('cohort_preserved_authority_changed')
            import nats_root_plan
            current=staging_cohort_preservation.successor_nats_authority(self.tt_session)
            if any(current[key]!=nats_authority[key] for key in ('plan','binding','scripts')):
                raise Blocked('cohort_preserved_authority_changed')
            parameters['bootstrap_target_scripts']={part:row['sha256'] for part,row in nats_authority['scripts'].items()}
        elif not guard.frontend_image_only({**parameters,'template_hashes':{'voice-'+s:None for s in request['changed_services']}}):
            try:enrollment=private_json(guard.ROOT/'bootstrap-enrollment-v4.json')
            except FileNotFoundError:raise Blocked('enrolled_bootstrap_selection_refused')
            if not isinstance(enrollment,dict):raise Blocked('enrolled_bootstrap_selection_refused')
            if enrollment is not None:
                from nats_contract_plan import digest as enrollment_digest
                previous_enrollment=private_json(guard.ROOT/'bootstrap-enrollment.json')
                if (enrollment.get('schema') not in ('voice-nats-bootstrap-enrollment-v4','voice-nats-bootstrap-enrollment-scoped-v4')
                    or enrollment.get('previous_enrollment')!=previous_enrollment
                    or enrollment.get('previous_enrollment_sha256')!=enrollment_digest(previous_enrollment)):
                    raise Blocked('enrolled_bootstrap_selection_refused')
                import nats_root_plan
                nats_authority=nats_root_plan.preflight(kube,workspace,enrollment,parameters['contract'],compiler.decode_yaml,installation=self.installation)
                parameters['bootstrap_target_scripts']={part:row['sha256'] for part,row in nats_authority['scripts'].items()}
        if renderer_selected:
            import renderer_root
            renderer_image=approved['images']['nats-hub-config-renderer']
            if previous is not None:
                current_renderer=stage.old_images[HUB+'/nats-config-renderer']
                expected_renderer=previous['renderer_authority']['descriptor']['images']['target']
                if current_renderer!=expected_renderer and not source_authority.same_image_content(current_renderer,expected_renderer,image_deadline):
                    raise Blocked('renderer_rollback_current_image_changed')
                renderer_image=previous['renderer_authority']['descriptor']['images']['old']
            if prerequisites is not None:
                renderer_authority=staging_prerequisites.load_renderer(self.tt_session,stage)
                if renderer_authority['descriptor']['images']['target']!=renderer_image:
                    raise Blocked('staging_prerequisites_renderer_changed')
            else:
                renderer_authority=renderer_root.prove(kube,stage,approved['source_sha'],renderer_image,installation=self.installation)
        if cohort_ref is None:
            actor_authority=actor_root.preflight(kube,workspace,self.code,self.binding,stage,names,compiler.decode_yaml,installation=self.installation)
        else:
            names=preserved_preflight['services'];actor_authority=preserved_preflight['actor']
            composition=_cohort_story_composition(self,request,approved,workspace)
            refreshed=actor_root.preflight(kube,workspace,self.code,self.binding,stage,names,compiler.decode_yaml,story_composition=composition,installation=self.installation)
            stores=staging_cohort_preservation._read(staging_cohort_preservation._base(self.tt_session)/'stores.json')
            current_secret=kube.get('secret','voice-app-secrets')
            staging_cohort_preservation._app_successor(self.tt_session,stores['app_before'],current_secret)
            actor_authority=actor_after_realtime_bootstrap(actor_authority,refreshed,
                stores['app_before']['metadata'],current_secret['metadata'])
            staging_cohort_preservation.revalidate_paused(self.tt_session)
        if full_story:transaction.validate_full_story_rollback_schema(previous,actor_authority)
        self._report('nats-preflight','COMPLETE')
        realtime_receipt=None
        if request['mode'] in ('full','app-only') and not provisional_mode and repair is None and previous is None:
            import realtime_bootstrap
            if getattr(self,'tt_session',None) is None:raise Blocked('realtime_bootstrap_session_required')
            first_install=request.get('gis_first_install') is True
            ordinary_target=None if first_install else compiler.prebootstrap_image_inventory(workspace,parameters,kube)
            verify_bootstrap_candidate_images(self.tt_session,images,renderer_authority,first_install,ordinary_target)
            realtime_receipt=realtime_bootstrap.run_prepare(self,request,approved,execution,workspace,stage)
            if realtime_receipt is None:raise Blocked('realtime_bootstrap_receipt_missing')
            if realtime_receipt is not None:
                schema=realtime_receipt.get('schema') if isinstance(realtime_receipt,dict) else None
                if schema in ('voice-staging-realtime-bootstrap-receipt-v1','voice-staging-realtime-bootstrap-receipt-v2'):
                    if realtime_bootstrap.load_completed(self.tt_session)!=realtime_receipt:raise Blocked('realtime_bootstrap_receipt_changed')
                    bootstrap_plan=realtime_bootstrap.load_plan(self.tt_session)
                    refreshed=actor_root.preflight(kube,workspace,self.code,self.binding,stage,names,compiler.decode_yaml,
                        installation=self.installation,**({'story_composition':_cohort_story_composition(self,request,approved,workspace)} if cohort_ref is not None else {}))
                    if realtime_bootstrap.load_completed(self.tt_session)!=realtime_receipt:raise Blocked('realtime_bootstrap_receipt_changed')
                    actor_authority=actor_after_realtime_bootstrap(actor_authority,refreshed,bootstrap_plan['secret'],realtime_receipt['secret'])
                elif schema in ('voice-staging-realtime-bootstrap-existing-v1','voice-staging-realtime-bootstrap-existing-v2','voice-staging-realtime-bootstrap-existing-v3','voice-staging-realtime-bootstrap-existing-v4'):
                    if realtime_bootstrap.load_existing(self.tt_session)!=realtime_receipt:raise Blocked('realtime_bootstrap_receipt_changed')
                    refreshed=actor_root.preflight(kube,workspace,self.code,self.binding,stage,names,compiler.decode_yaml,
                        installation=self.installation,**({'story_composition':_cohort_story_composition(self,request,approved,workspace)} if cohort_ref is not None else {}))
                    if refreshed!=actor_authority or realtime_bootstrap.load_existing(self.tt_session)!=realtime_receipt:
                        raise Blocked('realtime_bootstrap_actor_changed')
                else:raise Blocked('realtime_bootstrap_receipt_invalid')
        if cohort_ref is not None:
            staging_cohort_preservation.record_actor_successor(self.tt_session,actor_authority,stage,realtime_receipt)
        self._report('target-build','STARTED')
        build=workspace.parent/(operation+'-build');build.mkdir(mode=0o700)
        parameter_path=build/'parameters.json';save(parameter_path,parameters)
        output=build/('target-'+approved['source_sha']+'.json')
        compiler.main([str(workspace),str(parameter_path),str(output)])
        output.chmod(0o600)
        value=private_json(output)
        if getattr(self,'tt_session',None) is not None:value['target']['installation_sha256']=self.installation.sha256
        if realtime_receipt is not None:
            value['target']['realtime_bootstrap_sha256']=transaction.canonical(realtime_receipt)
        if prerequisites is not None:value['target']['prerequisites_sha256']=transaction.canonical(prerequisites)
        if cohort_ref is not None:value['target']['cohort_preservation_sha256']=transaction.canonical(cohort_ref)
        if lifecycle is not None and (value['migrations'] or value['nonnats']['actions']
            or nats_authority is not None and nats_authority['plan']['actions']):
            raise Blocked('provisional_migration_forbidden')
        if renderer_authority is not None:value['target']=renderer_root.bind_target(value['target'],renderer_authority)
        self._report('target-build','COMPLETE')
        def authority_before_fence():
            source_authority._head(source_authority._headers(request['token']),approved['source_sha'],time.monotonic()+30)
            if lifecycle is not None:source_authority.revalidate_ci(request['token'],approved)
            if repair is not None:
                provisional.verify_runtime(repair,kube,self.binding,private_json(INSTALLED/'active-release.json'),require_healthy=False)
            actor_root.revalidate(kube,workspace,self.code,self.binding,stage,names,compiler.decode_yaml,actor_authority,installation=self.installation)
            if renderer_authority is not None:renderer_root.revalidate(kube,stage,renderer_authority,installation=self.installation)
            if nats_authority is not None:
                current=(staging_cohort_preservation.successor_nats_authority(self.tt_session) if cohort_ref is not None
                    else nats_root_plan.preflight(kube,workspace,enrollment,parameters['contract'],compiler.decode_yaml,installation=self.installation))
                # Business records may advance, but config/consumer intent and
                # dynamic subject ownership cannot change between plan/fence.
                if current['plan']!=nats_authority['plan'] or current['binding']!=nats_authority['binding']:raise Blocked('nats_migration_live_contract_changed')
            if repair is not None:
                provisional.link_parent(guard.ROOT/('rollout-'+repair['operation']),repair,operation)
        lifecycle_args={'lifecycle':lifecycle} if lifecycle is not None else {}
        if getattr(self,'tt_session',None) is not None:lifecycle_args['resource_session']=self.tt_session
        if realtime_receipt is not None:lifecycle_args['realtime_bootstrap_receipt']=realtime_receipt
        if prerequisites is not None:lifecycle_args['prerequisites_capture']=prerequisites
        if cohort_ref is None:
            state=root_cli.prepare_value(value,self.code,self.binding,operation,before_fence=authority_before_fence,nats_authority=nats_authority,actor_authority=actor_authority,actor_services=names,renderer_authority=renderer_authority,gis_source_authority=(approved if request.get('gis_first_install') is True or gis_binding is not None else None),gis_source_root=(workspace if request.get('gis_first_install') is True else None),gis_source_capture=(approved['source_files'] if request.get('gis_first_install') is True else None),gis_transition=gis_transition,gis_binding=gis_binding,**lifecycle_args)
        else:
            source_authority._head(source_authority._headers(request['token']),approved['source_sha'],time.monotonic()+30)
            state=root_cli.prepare_preserved_value(value,self.code,self.binding,operation,resource_session=self.tt_session,preservation_ref=cohort_ref,
                nats_authority=nats_authority,actor_authority=actor_authority,actor_services=names,renderer_authority=renderer_authority,
                gis_source_authority=(approved if request.get('gis_first_install') is True or gis_binding is not None else None),
                gis_source_root=(workspace if request.get('gis_first_install') is True else None),
                gis_source_capture=(approved['source_files'] if request.get('gis_first_install') is True else None),gis_transition=gis_transition,
                gis_binding=gis_binding,realtime_bootstrap_receipt=realtime_receipt,prerequisites_capture=prerequisites)
        base=guard.ROOT/('rollout-'+operation)
        state['service_actor_authority']=actor_authority
        state['service_actor_services']=names
        state['source_authority']={key:value for key,value in approved.items() if key!='source_files'}
        state['execution_authority']=execution
        if previous is not None:state['rollback_from']={'operation':previous['operation'],'original_release_source':previous['target']['tag'],'images':images}
        save(base/'checkpoint.json',state)
        return self._encrypt(base,state,execution) if cohort_ref is None else state

    def _verify_decrypted_restore(self,base,state,stage,*,payload_base=None,cut=None,binding=None):
        """Full remote-byte equality binds the actual authenticated restore."""
        from preserve import closed_native_states,recovered_census,canonical,native_catalog,verify_catalog
        from rollout_census import semantic
        from expired_proof import observe_copy
        from native_store import verify_archive
        from docker_runtime import NATS_IMAGE,DockerRuntime
        payload_base=Path(payload_base or base);cut=cut or state['cut'];binding=binding or state['cipher_binding']
        if payload_base==Path(base):
            _validate_copy_checkpoint(private_json(payload_base/'copy-checkpoint.json'),state,cut,self.installation,
                state.get('cipher_space_members'))
        runtime=DockerRuntime(base,state['operation'])
        def readback(archive):
            stage.verify_final_storage()
            if not verify_archive(archive,cut['manifest']):raise Blocked('decrypted_restore_native_changed')
            states=closed_native_states(runtime,archive,cut['manifest'],cut['census'],stage.verify_final_storage)
            catalog=native_catalog(runtime,archive,cut['manifest'],stage.verify_final_storage)
            observed=observe_copy(runtime,archive,cut['manifest'],'decrypted-readback',stage.verify_final_storage,native_only=True)
            verify_catalog(catalog,observed['census'])
            compared,ephemeral=recovered_census(cut['census'],observed['census'],observed['recovery'],cut['manifest'])
            if (canonical(cut['census'])!=cut['census_sha256'] or semantic(compared)!=semantic(cut['census'])):
                raise Blocked('decrypted_restore_census_changed')
            stage.verify_final_storage()
            return {'native_archive_verified':True,'complete_durable_sha256':canonical(states),
                'census_sha256':cut['census_sha256'],'native_messages':observed['native_messages'],
                'ephemeral_recovery_observations':ephemeral,'server_image':NATS_IMAGE}
        members={'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json'}
        if payload_base==Path(base):members.update(state.get('cipher_space_members',{}))
        members.update(binding.get('space_members',{}));members.update(binding.get('postseal_members',{}))
        receipt=encrypted_cut.decrypt_verify(payload_base,INSTALLED/'recovery',binding,readback,expected_members=members)
        save(payload_base/'decrypted-restore.json',receipt)
        return receipt

    def _encrypt(self,base,state,execution):
        cut=state['cut']
        members=None
        if 'space_backup' in state:
            _,members=encrypted_cut.space_members(base,state['space_backup'])
        copy_binding=_copy_checkpoint(state,cut,self.installation,members)
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
        stage=transaction.reconstruct(Kube(),state,lambda event:None,installation=self.installation)
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
        if request['action']=='authorize-expired-native':
            import migrations
            _,state=self.state(request['operation'])
            migrations.require_preserving_admission(state.get('target',{}).get('mode'),state.get('migrations'))
        expired_actions={'prepare-expired-native':self._prepare_expired_native,
            'authorize-expired-native':self._authorize_expired_native,'finish-expired-native':self._finish_expired_native,
            'prepare-expired-finish':self._prepare_expired_finish,'authorize-expired-finish':self._authorize_expired_finish}
        if request['action'] in expired_actions:return expired_actions[request['action']](request)
        if request['action']=='prepare':
            if request.get('mode') in ('full','app-only') and request.get('provisional_staging') is not True:
                return self._prepare_cohort(request)
            return self._prepare(request)
        if request['action']=='prepare-repair':
            import provisional
            base,parent=self.state(request['operation'])
            provisional.require_ready(parent,self.binding)
            if not set(parent['target']['changed_services'])<=set(request['changed_services']):
                raise Blocked('provisional_repair_scope_missing')
            if (base/'provisional-child.json').exists():raise Blocked('provisional_child_already_bound')
            provisional.verify_runtime(parent,Kube(),self.binding,private_json(INSTALLED/'active-release.json'),require_healthy=False)
            return self._prepare(request,repair=parent)
        if request['action']=='accept-provisional':
            import provisional
            _,execution=self._dispatcher(request)
            if execution['event']!='workflow_dispatch' or execution['workflow_id']!=263689731:
                raise Blocked('provisional_dispatch_required')
            base,state=self.state(request['operation'])
            return summary(provisional.accept(base,state,request['token'],Kube(),self.binding,private_json(INSTALLED/'active-release.json')))
        if request['action']=='resume-cold-backup':return self._resume_cold_backup(request)
        if request['action']=='resume-cipher-upload':return self._resume_cipher_upload(request)
        if request['action']=='prepare-rollback':
            base,previous=self.state(request['operation'])
            active=private_json(INSTALLED/'active-release.json')
            if active.get('operation')!=previous['operation'] or active.get('target_sha256')!=previous['target']['manifest_sha256']:raise Blocked('bridge_rollback_latest_release_required')
            story_only=previous['input_target']['mode']=='full'
            transaction.rollback_package(previous,story_only=story_only)
            changed=transaction.rollback_services(previous,story_only=story_only)
            return self._prepare({**request,'mode':'images-only','changed_services':changed},previous)
        session=getattr(self,"tt_session",None)
        if request["action"]=="authorize" and session is not None:
            row=session.journal.read()
            if row.get("cohort_preservation_sha256") is not None:
                return self._authorize_cohort(request)
        base,state=self.state(request['operation'])
        if request['action']=='status':return status_summary(state)
        if request['action'] in ('authorize','authorize-preserved-upload'):
            session=self._require_authorization_binding(state)
            import migrations
            migrations.require_preserving_admission(state.get('target',{}).get('mode'),state.get('migrations'))
            _validate_copy_checkpoint(private_json(base/'copy-checkpoint.json'),state,state['cut'],self.installation,
                state.get('cipher_space_members'))
            if state.get('provisional') is not None:
                source_authority.revalidate_ci(request['token'],state['source_authority'])
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
            self._verify_decrypted_restore(base,state,transaction.reconstruct(Kube(),state,lambda event:None,installation=self.installation))
            import actor_root
            actor_root.revalidate(Kube(),INSTALLED/'sources'/state['operation'],self.code,self.binding,
                transaction.reconstruct(Kube(),state,lambda event:None,installation=self.installation),state['service_actor_services'],compiler.decode_yaml,state['service_actor_authority'],installation=self.installation)
            if state.get('provisional') is not None:
                source_authority.revalidate_ci(request['token'],state['source_authority'])
            if upload_record is not None:
                preserved_upload.verify_deadline(upload_record,dt.datetime.now(dt.timezone.utc))
                state['custody']=receipt
                transaction.authorize(Kube(),base,state,state['cut']['manifest']['archive_sha256'],state['cut']['manifest_sha256'],state['contract'],
                    authorization_deadline=dt.datetime.fromisoformat(upload_record['deadline']),resource_session=session)
            else:
                transaction.authorize(Kube(),base,state,state['cut']['manifest']['archive_sha256'],state['cut']['manifest_sha256'],state['contract'],resource_session=session)
            gid=grp.getgrnam('pmd').gr_gid
            for name in ('apply-authorization.json','apply-manifests.json'):os.chown(base/name,0,gid);(base/name).chmod(0o440)
            return summary(state)|{'authorization':str(base/'apply-authorization.json')}
        if request['action']=='finish':
            if state.get('expired_recovery_authority') is not None:
                raise Blocked('expired_native_explicit_finish_required')
            receipt=guard.protected_receipt(base/'apply-authorization.json')
            result=transaction.finish(Kube(),base,state,receipt,request['claim_rv'],state['contract'],installation=self.installation)
            self._active_release(result)
            return summary(result)
        raise Blocked('bridge_action_invalid')

    def _authorize_cohort(self,request):
        import staging_cohort_preservation as cohort
        session=self.tt_session;row=session.check()
        if row.get("phase")!="cohort-preservation":
            # Read-only recovery can prove a completed final result. No child,
            # target construction or authorization is retried after an intent.
            return cohort.final_authorization_result(session,request)
        cohort.qualify_offnode(self,request)
        qualified,loaded=cohort.begin_continuation(session,request)
        prepare={"action":"prepare","nonce":cohort._read(cohort._request_record(session))["nonce"],
            "mode":loaded["request"]["mode"],"changed_services":loaded["request"]["changed_services"],
            "source_sha":session.context["commit"],"resource_context":session.context,"token":request["token"]}
        if loaded["request"]["gis_first_install"]:prepare["gis_first_install"]=True
        self._prepare(prepare,cohort_ref=qualified)
        base,state=self.state(session.operation)
        transaction.authorize(Kube(),base,state,state["cut"]["manifest"]["archive_sha256"],
            state["cut"]["manifest_sha256"],state["contract"],resource_session=session,cohort_request=request)
        gid=grp.getgrnam("pmd").gr_gid
        for name in ("apply-authorization.json","apply-manifests.json"):
            os.chown(base/name,0,gid);(base/name).chmod(0o440)
        return cohort.final_authorization_result(session,request)

    def _expired_source_admission(self,request):
        """Fresh helper authority and historical target data remain separate.

        Internal producer entry only; no request route admits an incomplete
        proof/upload/finish protocol. The final consumer wires this result.
        """
        import expired_admission
        import expired_recovery
        import paused_recovery
        base,state=self.state(request['operation'])
        if state.get('gis_binding') is not None:
            raise Blocked('gis_existing_recovery_unsupported')
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
            _,again=expired_admission.admit(Kube(),base,state,self.code,self.binding,historical,fresh)
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
        if state.get('gis_binding') is not None:
            raise Blocked('gis_existing_recovery_unsupported')
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
            _,again=expired_admission.admit_finish(Kube(),base,state,self.code,self.binding,source_fresh)
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
            import deployment_report
            deployment_report.capture_failure(error)
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
            'original_artifact_name':'voice-nats-rollout-'+state['operation']+'-'+record['artifact_bindings']['original']['challenge'],
            'observation_artifact_name':'voice-nats-rollout-'+state['operation']+'-'+record['artifact_bindings']['observation']['challenge'],
            'source_sha':state['target']['tag']}

    def _authorize_expired_finish(self,request):
        import expired_finish
        import expired_transport
        base,state,stage,execution,current,admission,fresh=self._finish_source_admission(request)
        execution['nonce']=request['upload_nonce']
        slot=base/'expired-finish-proofs'/request['upload_nonce']
        record=expired_transport.upload.private_json(slot/'upload.json')
        if record['execution']!=execution:raise Blocked('expired_finish_upload_execution_changed')
        _validate_copy_checkpoint(private_json(base/'copy-checkpoint.json'),state,state['cut'],self.installation,
            state.get('cipher_space_members'))
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
        repair=expired_recovery.repair_inputs(base,admission['input_hashes'],
            expired_recovery.repair_admission_digest(base,admission))
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
                state['contract'],compiler.decode_yaml,runtime,broker,tree,installation=self.installation)
            if (actual['plan']!=state['nats_contract'] or actual['binding']!=state['nats_contract_binding']
                or actual['scripts']!=state['nats_target_scripts']):
                raise Blocked('expired_recovery_original_nats_authority_changed')
            # The authoritative archive's exact message-file bytes and complete
            # durable state are verified; this makes no API record-decoding claim.
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
            'original_artifact_name':'voice-nats-rollout-'+state['operation']+'-'+record['artifact_bindings']['original']['challenge'],
            'observation_artifact_name':'voice-nats-rollout-'+state['operation']+'-'+record['artifact_bindings']['observation']['challenge'],
            'deploy_mode':state['target']['mode'],'changed_services':','.join(state['target']['changed_services']),
            'source_sha':state['target']['tag']}

    def _authorize_expired_native(self,request):
        """Actual dual readback and fresh guards precede durable transaction entry."""
        import expired_transport
        import expired_recovery
        self._require_authorization_operation(request.get('operation'))
        _,current_state=self.state(request['operation'])
        self._require_authorization_binding(current_state)
        # Current master/source/helper admission is repeated for this separate
        # authorize request, while upload proof stays bound to its own nonce.
        base,state,stage,execution,current,admission,all_fresh=self._expired_source_admission(request)
        self._require_authorization_binding(state)
        execution['nonce']=request['upload_nonce']
        slot=base/'expired-recovery-proofs'/request['upload_nonce']
        record=expired_transport.upload.private_json(slot/'upload.json')
        if record['execution']!=execution:raise Blocked('expired_recovery_upload_execution_changed')
        _validate_copy_checkpoint(private_json(base/'copy-checkpoint.json'),state,state['cut'],self.installation,
            state.get('cipher_space_members'))
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
            authorization_deadline=dt.datetime.fromisoformat(actual['expires_at']),expired_authority=actual,
            resource_session=getattr(self,'tt_session',None))
        gid=grp.getgrnam('pmd').gr_gid
        for name in ('apply-authorization.json','apply-manifests.json'):
            os.chown(base/name,0,gid);(base/name).chmod(0o440)
        return summary(state)|{'authorization':str(base/'apply-authorization.json')}
