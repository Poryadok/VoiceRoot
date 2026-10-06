"""Finite nonreset transaction; root owns prepare/off-node checkpoint/finish.

These entrypoints accept captured immutable code and target inputs only. They
never allocate a claim or execute bootstrap against existing staging data.
"""
import copy
import datetime as dt
import json
import os
from pathlib import Path
import shutil
import guard
import migrations
import nonnats_plan
import nonnats_runtime
import encrypted_cut
import space_safeguards
import renderer_root
from controller import Blocked, file_sha
from native_store import verify_archive,preflight as native_preflight
from root_main import capture_inputs,revalidate_inputs,save as root_save,private_json
from docker_runtime import DockerRuntime
from runtime_stage import RolloutStage
from preserve import capture_cut, verify_post_apply, canonical
from normalize import normalize_target,image_only_documents
from errors import safe_error
from nats_migration import apply_contract

def save(path,row):root_save(path,row,limit=128<<20)


def rollback_services(previous):
    # Selection is not schema compatibility approval. Full admission remains a
    # separate prerequisite before any fresh prepare/fence is authorized.
    import source_authority
    catalog=set(source_authority.IMAGE_NAMES)-{'nats-hub-config-renderer'}
    target=previous.get('input_target',{})
    if previous.get('status')!='PASS':raise Blocked('rollout_rollback_successful_release_required')
    if target.get('mode')=='full':
        recorded=set(target.get('template_hashes',{}))
        if renderer_root.HUB not in recorded or previous.get('renderer_authority') is None:
            raise Blocked('renderer_rollback_authority_required')
        recorded.remove(renderer_root.HUB)
        if recorded!={'voice-'+name for name in catalog}:raise Blocked('rollout_rollback_full_catalog_incomplete')
        return sorted(catalog)
    names=target.get('changed_services')
    if target.get('mode')!='images-only' or not isinstance(names,list) or not names or any(not isinstance(n,str) for n in names) or len(set(names))!=len(names) or not set(names)<=catalog:
        raise Blocked('rollout_rollback_service_selection_invalid')
    return sorted(names)


def rollback_package(previous):
    if previous.get('status')!='PASS' or previous['input_target']['mode']!='images-only':
        raise Blocked('rollout_rollback_database_compatibility_required')
    selected=rollback_services(previous)
    backends=set(selected)-{'web','admin','developer-portal'}
    if backends:
        import actor_root
        authority=previous.get('service_actor_authority') or {}
        candidate=authority.get('compatible_story_candidate') or {}
        image=previous.get('context',{}).get('old_images',{}).get('voice-story/story')
        schema=authority.get('story_schema')
        content=(candidate.get('child_sha256'),candidate.get('config_sha256'),candidate.get('component_source_sha'))
        if (backends!={'story'} or not isinstance(image,str) or not __import__('re').fullmatch(r'ghcr\.io/poryadok/voiceroot/story@sha256:[a-f0-9]{64}',image) or content not in actor_root.STORY_IMAGES.values() or schema is None
            or 'story' not in authority.get('roles',[]) or 'story' not in authority.get('proofs',{})
            or candidate!={'schema':'voice-reviewed-story-compatible-pair-v1','image':image,
                'child_sha256':content[0],'config_sha256':content[1],
                'component_source_sha':content[2],'schema_sha256':canonical(schema),
                'actor_sha256':canonical({'binding':authority.get('binding'),'mount':authority.get('mounts',{}).get('story'),'proof':authority.get('proofs',{}).get('story')})}):
            raise Blocked('rollout_rollback_backend_candidate_unproved')
    target=copy.deepcopy(previous['input_target']);rows=[];images={}
    for name in target['template_hashes']:
        if name==renderer_root.HUB:
            if previous.get('renderer_authority') is None:raise Blocked('renderer_rollback_authority_required')
            # Root bridge captures a fresh renderer descriptor and proof. The
            # application runner never receives a hub Deployment document.
            continue
        old=previous['context']['original_snapshots'][name]
        row={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':name,'namespace':guard.NS},
             'spec':copy.deepcopy(old['spec'])}
        template=row['spec']['template']
        for c in template['spec'].get('initContainers',[])+template['spec']['containers']:
            key=name+'/'+c['name'];image=previous['context']['old_images'].get(key,'')
            if not guard.SHA.fullmatch(image.rsplit('@sha256:',1)[-1]) or '@sha256:' not in image:
                raise Blocked('rollout_rollback_image_unpinned')
            c['image']=image;images[key]=image
        rows.append(row)
    target['images']=images;target['template_hashes']={r['metadata']['name']:canonical(r['spec']['template']) for r in rows}
    target['manifest_sha256']=canonical(rows);target['migration_sha256']=canonical([])
    plan=copy.deepcopy(previous['nonnats'])
    # Only the successfully authorized image changes may update a preserved
    # signing descriptor; config/env drift remains forbidden in the fresh cut.
    for row in plan['checks']:
        for descriptor in row['objects']:
            if descriptor['kind']!='Deployment':continue
            for c in descriptor['desired'].get('spec',{}).get('template',{}).get('spec',{}).get('containers',[]):
                image=previous['target']['images'].get(descriptor['name']+'/'+c['name'])
                if image is not None:c['image']=image
    target['nonnats_sha256']=canonical(plan)
    return {'target':target,'manifests':rows,'migrations':[],'contract':copy.deepcopy(previous['contract']),'nonnats':plan}


def context(stage):
    return {k:copy.deepcopy(getattr(stage,k)) for k in ('expected','snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv')} | {'final_path':str(stage.final_path)}


def reconstruct(kube,state,journal):
    ctx=state['context'];stage=RolloutStage(kube,state['operation'],ctx['expected'],journal)
    for name in ('snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv'):
        setattr(stage,name,copy.deepcopy(ctx[name]))
    stage.final_path=Path(ctx['final_path'])
    if state.get('renderer_authority') is not None:
        stage.renderer_transition=copy.deepcopy(state['renderer_authority']['descriptor'])
        stage.renderer_post_start=lambda current:renderer_root.verify_live(current,state['renderer_authority'])
    return stage


def checkpoint_journal(base,state,stage_getter):
    def journal(event):
        state['events'].append(copy.deepcopy(event))
        stage=stage_getter()
        if stage is not None:
            state['context']=context(stage)
        save(Path(base)/'checkpoint.json',state)
    return journal


def prepare(kube,base,code,target,contract,operation,code_capture,migration_plan,non_nats,before_fence=None,nats_authority=None,actor_authority=None,actor_services=None,renderer_authority=None):
    # base creation/root lock/captured code custody belongs to the root launcher.
    base=Path(base);code=Path(code)
    state={'schema':'nats-rollout-root-v1','operation':operation,'phase':'READ_ONLY_CAPTURE',
           'status':'RUNNING','fence_status':'UNKNOWN','events':[],'target':copy.deepcopy(target),
           'code_capture':copy.deepcopy(code_capture),'contract':copy.deepcopy(contract),
           'input_target':copy.deepcopy(target),
           'service_actor_authority':copy.deepcopy(actor_authority),
           'renderer_authority':copy.deepcopy(renderer_authority),
           'service_actor_services':list(actor_services or [])}
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    save(base/'checkpoint.json',state)
    manifests=json.loads((base/'apply-manifests.json').read_bytes())
    if guard.frontend_image_only(target) and (migration_plan or non_nats['actions']):
        raise Blocked('rollout_frontend_auxiliary_action_forbidden')
    stage=RolloutStage.capture(kube,operation,journal,
        [r['metadata']['name'] for r in manifests if r.get('kind')=='Deployment']+([renderer_root.HUB] if renderer_authority is not None else []))
    if target['mode']=='images-only':
        manifests,target=image_only_documents(stage,manifests,target,renderer_authority=renderer_authority)
        save(base/'apply-manifests.json',manifests)
        target['manifest_sha256']=file_sha(base/'apply-manifests.json')
    state['target']=normalize_target(kube,stage,manifests,target,renderer_authority=renderer_authority)
    if canonical(migration_plan)!=target['migration_sha256']:raise Blocked('rollout_target_migration_digest_invalid')
    state['migrations']=copy.deepcopy(migration_plan)
    state['migration_secret_metadata']=migrations.preflight(kube,migration_plan,target['mode'])
    state['space_preflight']=space_safeguards.preflight(kube,base,migration_plan)
    if canonical(non_nats)!=target['nonnats_sha256']:raise Blocked('rollout_target_nonnats_digest_invalid')
    state['nonnats']=copy.deepcopy(non_nats)
    state['nonnats_binding']=nonnats_runtime.preflight(kube,non_nats,target['mode'])
    import source_plan
    source_plan.verify_db_init(kube,non_nats)
    state['context']=context(stage)
    claim=kube.get('pvc',stage.expected['source_claim'])
    capacity=claim['status']['capacity']['storage']
    state['native_capacity']=native_preflight(stage.final_path,base,capacity)
    state['combined_space_capacity']=space_safeguards.combined_capacity(base,state['native_capacity'],state['space_preflight'])
    if state['space_preflight'] is not None:
        state['space_preflight']['source_sha']=target['tag']
        state['space_preflight']['migration_plan_sha256']=target['migration_sha256']
    try:enrollment=private_json(guard.ROOT/'bootstrap-enrollment.json')
    except FileNotFoundError:enrollment=None
    state['provenance']=capture_inputs(kube,base,contract,stage.expected['generation'],bootstrap_enrollment=enrollment)
    if enrollment is not None:state['bootstrap_enrollment']=copy.deepcopy(enrollment)
    if nats_authority is not None:
        if enrollment is None:raise Blocked('nats_migration_enrollment_missing')
        state['nats_contract']=copy.deepcopy(nats_authority['plan'])
        state['nats_contract_binding']=copy.deepcopy(nats_authority['binding'])
        state['nats_target_scripts']=copy.deepcopy(nats_authority['scripts'])
        state['nats_contract_expires_at']=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=4)).isoformat()
    shutil.copyfile(code/'kernel',base/'kernel');os.chmod(base/'kernel',0o550);os.chown(base/'kernel',0,65532)
    state['kernel_sha256']=file_sha(base/'kernel')
    state['phase']='FENCE';save(base/'checkpoint.json',state)
    try:
        nonnats_plan.revalidate(kube,state['nonnats_binding'])
        if before_fence is not None:before_fence()
        stage.fence();state['context']=context(stage);state['fence_status']='VERIFIED'
        save(base/'checkpoint.json',state)
        runtime=DockerRuntime(base,operation)
        state['phase']='COLD_BACKUP';save(base/'checkpoint.json',state)
        state['cut']=capture_cut(runtime,stage.final_path,stage.verify_final_storage)
        save(base/'rollout-before-manifest.json',state['cut']['manifest'])
        state['cut']['manifest_sha256']=file_sha(base/'rollout-before-manifest.json')
        if state['space_preflight'] is not None:
            state['phase']='SPACE_BACKUP';save(base/'checkpoint.json',state)
            state['space_gate']=space_safeguards.capture(kube,base,state['space_preflight'],operation,
                stage.verify_final_storage,save)
            if 'backup' in state['space_gate']:state['space_backup']=copy.deepcopy(state['space_gate'])
        state['phase']='AWAITING_OFF_NODE';state['status']='WAITING'
        state['context']=context(stage);save(base/'checkpoint.json',state)
        return state
    except Exception as error:
        state['context']=context(stage);state['status']='BLOCKED';state['fence_status']='UNKNOWN'
        state['error']=safe_error(error)
        try:
            if stage.refence().get('verified') is True:state['fence_status']='VERIFIED'
        except Exception:pass
        save(base/'checkpoint.json',state);raise


def authorize(kube,base,state,off_node_archive_sha,off_node_manifest_sha,contract):
    migration_recovery=(state['phase']=='NATS_CONTRACT_MIGRATION' and state['status']=='BLOCKED'
        and state.get('custody',{}).get('verified') is True and 'nats_contract' in state
        and state['target']['mode']=='images-only'
        and state['context']['marker']['data'].get('phase')=='rollout-capturing')
    renderer_recovery=(state['phase']=='RENDERER_TEMPLATE' and state['status']=='BLOCKED'
        and state.get('custody',{}).get('verified') is True and state.get('renderer_authority') is not None
        and 'migration_jobs' in state and 'space_postcondition' in state
        and state['context']['marker']['data'].get('phase')=='rollout-capturing')
    recovery=migration_recovery or renderer_recovery
    if not recovery and (state['phase']!='AWAITING_OFF_NODE' or state['status']!='WAITING'):
        raise Blocked('rollout_offnode_phase_invalid')
    if 'nats_contract' in state:
        expires=dt.datetime.fromisoformat(state['nats_contract_expires_at'])
        if expires.tzinfo is None or dt.datetime.now(dt.timezone.utc)>=expires:raise Blocked('nats_migration_authority_expired')
    cut=state['cut'];manifest=cut['manifest']
    if (off_node_archive_sha!=manifest['archive_sha256'] or
        off_node_manifest_sha!=cut['manifest_sha256'] or
        file_sha(Path(base)/'rollout-before-manifest.json')!=cut['manifest_sha256'] or
        not verify_archive(Path(base)/'rollout-before.tar',manifest)):
        raise Blocked('rollout_offnode_copy_mismatch')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal)
    # An interrupted owned CM CAS is checked by advance's exact UID/hash/
    # operation annotation. Credential provenance remains strict on retries.
    revalidate_inputs(kube,{'scripts':[]} if migration_recovery else contract,state['provenance'])
    stage.verify_final_storage()
    if 'nonnats' in state:nonnats_runtime.verify(kube,state['nonnats'],state['nonnats_binding'],stage)
    if canonical(state['migrations'])!=state['target']['migration_sha256']:raise Blocked('rollout_target_migration_digest_invalid')
    # A restored DB dump alone is insufficient: the root bridge must first
    # read every byte of the enclosing encrypted artifact from off-node custody.
    if 'space_backup' in state:
        state['space_offnode_backup']=encrypted_cut.authorize_space(base,state)
    if not renderer_recovery:space_safeguards.before_migrate(kube,state.get('space_gate'),state.get('space_offnode_backup'))
    if migrations.bot_migration.required(state['migrations']):
        # Root-owned closed writer fence and off-node custody already hold.
        # Recheck the forward constraint before either NATS UPDATE/CREATE or
        # any DB Job; pre-fence observation alone cannot authorize migration.
        expected=state.get('migration_completed_metadata') if renderer_recovery else state['migration_secret_metadata']
        if expected is None or migrations.preflight(kube,state['migrations'],state['target']['mode'])!=expected:
            raise Blocked('rollout_database_secret_changed')
    state['phase']='DATABASE_MIGRATIONS';save(Path(base)/'checkpoint.json',state)
    try:
        if renderer_recovery:
            # The renderer CAS never starts a broker or a DB job. On retry,
            # prove the recorded post-migration native baseline again rather
            # than executing any previously completed migration a second time.
            stage.verify_closed()
            baseline=state.get('nats_migration',{}).get('cut',state['cut'])
            verify_post_apply(DockerRuntime(base,state['operation']),stage.final_path,baseline,stage.verify_final_storage)
        elif 'nats_contract' in state:
            state['phase']='NATS_CONTRACT_MIGRATION';save(Path(base)/'checkpoint.json',state)
            state['nats_migration']=apply_contract(base,state,stage,journal)
            save(Path(base)/'checkpoint.json',state)
            from nats_contract_custody import advance
            active=advance(kube,state,stage,journal)
            save(guard.ROOT/'installed'/'active-contract.json',active)
            state['active_contract']=active;save(Path(base)/'checkpoint.json',state)
        if not renderer_recovery:
            state['migration_jobs']=migrations.execute(kube,stage,state['migrations'],state['target']['mode'],
                                                        state['migration_secret_metadata'],journal)
            if migrations.bot_migration.required(state['migrations']):
                after=migrations.preflight(kube,state['migrations'],state['target']['mode'])
                expected=copy.deepcopy(state['migration_secret_metadata'])
                row=next(r for r in state['migrations'] if r['database']=='bot')
                version=max(int(n.split('_',1)[0]) for n in row['files'] if n.endswith('.up.sql'))
                expected['bot_prerequisite']['observation']['version']=version
                if after!=expected:raise Blocked('rollout_completed_database_authority_changed')
                state['migration_completed_metadata']=after
                save(Path(base)/'checkpoint.json',state)
        state['space_postcondition']=space_safeguards.after_migrate(kube,state.get('space_gate'))
        save(Path(base)/'checkpoint.json',state)
        if state.get('renderer_authority') is not None:
            state['phase']='RENDERER_TEMPLATE';save(Path(base)/'checkpoint.json',state)
            renderer_root.apply_paused(stage,state['renderer_authority'])
            state['renderer_applied']=True;save(Path(base)/'checkpoint.json',state)
        stage.prepared()
    except Exception as error:
        state['status']='BLOCKED';state['fence_status']='UNKNOWN'
        state['error']=safe_error(error)
        try:
            if stage.refence().get('verified') is True:state['fence_status']='VERIFIED'
        except Exception:pass
        state['context']=context(stage);save(Path(base)/'checkpoint.json',state);raise
    row=stage.marker;claim=stage.final_claim;pv=stage.final_pv
    receipt={'schema':'nats-rollout-preservation-v1','operation':state['operation'],
        'namespace_uid':stage.expected['namespace_uid'],
        'marker':{'uid':row['metadata']['uid'],'resourceVersion':row['metadata']['resourceVersion'],
                  **{k:row['data'][k] for k in ('generation','previousGeneration','dataPVC')}},
        'pvc':{'name':claim['metadata']['name'],'uid':claim['metadata']['uid'],'pv_name':claim['spec']['volumeName']},
        'pv':{'uid':pv['metadata']['uid'],'kind':next(k for k in ('local','hostPath') if k in pv['spec']),
              'path':str(stage.final_path),'node':'pmdebook'},
        'target':state['target'],
        'backup':{'archive_sha256':manifest['archive_sha256'],'manifest_sha256':cut['manifest_sha256'],
                  'census_sha256':cut['census_sha256'],'off_node_verified':True},
        'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=4)).isoformat()}
    save(Path(base)/'apply-authorization.json',receipt)
    state['authorization']=copy.deepcopy(receipt)
    state['context']=context(stage);state['phase']='PAUSED_APPLY';state['status']='WAITING'
    save(Path(base)/'checkpoint.json',state)
    return receipt


def finish(kube,base,state,receipt,claim_rv,contract):
    if state['phase']!='PAUSED_APPLY' or state['status']!='WAITING':
        raise Blocked('rollout_finish_phase_invalid')
    if receipt!=state.get('authorization') or receipt.get('operation')!=state['operation'] or receipt.get('target')!=state['target']:
        raise Blocked('rollout_finish_authorization_changed')
    expires=dt.datetime.fromisoformat(receipt['expires_at'])
    if expires.tzinfo is None or dt.datetime.now(dt.timezone.utc)>=expires:
        raise Blocked('rollout_finish_authorization_expired')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal)
    # No replica can restart until exact target templates and unchanged data
    # are proved. Original templates remain separately bound for rollback.
    revalidate_inputs(kube,contract,state['provenance'])
    stage.adopt_applied_target(receipt,claim_rv)
    if 'nonnats' in state:state['nonnats_applied']=nonnats_runtime.verify(kube,state['nonnats'],state['nonnats_binding'],stage,applied=True)
    state['context']=context(stage);state['phase']='POST_APPLY_PROOF';save(Path(base)/'checkpoint.json',state)
    try:
        runtime=DockerRuntime(base,state['operation'])
        baseline=state.get('nats_migration',{}).get('cut',state['cut'])
        state['preservation']=verify_post_apply(runtime,stage.final_path,baseline,stage.verify_final_storage)
        stage.verified();state['context']=context(stage);save(Path(base)/'checkpoint.json',state)
        state['phase']='RESTART';save(Path(base)/'checkpoint.json',state)
        state['restart']=stage.restart()
        state['context']=context(stage);state['phase']='POST_RESTART';state['status']='PASS';state['fence_status']='RELEASED'
        save(Path(base)/'checkpoint.json',state)
        return state
    except Exception as error:
        state['status']='BLOCKED';state['fence_status']='UNKNOWN'
        state['error']=safe_error(error)
        # refence itself rechecks exact marker ownership before every mutation.
        # Never claim VERIFIED solely because the preceding phase was paused.
        try:
            observed=stage.refence()
            if observed.get('verified') is True:state['fence_status']='VERIFIED'
        except Exception:pass
        state['context']=context(stage);save(Path(base)/'checkpoint.json',state)
        raise


def rollback(kube,base,state,receipt,claim_rv,contract):
    # An application which has resumed may have newer NATS/DB state. It needs
    # a fresh preservation transaction; this recovery never replays a backup.
    if state['phase'] not in ('PAUSED_APPLY','POST_APPLY_PROOF','ROLLBACK_TEMPLATES') or state['target']['mode']!='images-only':
        raise Blocked('rollout_rollback_fresh_transaction_required')
    if state.get('status') not in ('WAITING','BLOCKED') or receipt!=state.get('authorization'):
        raise Blocked('rollout_rollback_authorization_changed')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal)
    revalidate_inputs(kube,contract,state['provenance'])
    try:
        if state['phase']!='ROLLBACK_TEMPLATES':stage.adopt_rollback_target(receipt,claim_rv)
        state['phase']='ROLLBACK_TEMPLATES';save(Path(base)/'checkpoint.json',state)
        if state.get('renderer_authority') is not None:
            if state.get('renderer_recovery_authority') is None:
                reverse=renderer_root.recovery_authority(stage,state['renderer_authority'])
                state['renderer_recovery_authority']=copy.deepcopy(reverse)
                state['renderer_authority']=copy.deepcopy(reverse)
                save(Path(base)/'checkpoint.json',state)
            reverse=state['renderer_recovery_authority']
            stage.renderer_transition=copy.deepcopy(reverse['descriptor'])
            stage.renderer_post_start=lambda current:renderer_root.verify_live(current,reverse)
            renderer_root.apply_paused(stage,reverse)
        stage.restore_original_templates()
        state['phase']='ROLLBACK_PROOF';save(Path(base)/'checkpoint.json',state)
        baseline=state.get('nats_migration',{}).get('cut',state['cut'])
        state['preservation']=verify_post_apply(DockerRuntime(base,state['operation']),stage.final_path,baseline,stage.verify_final_storage)
        stage.verified()
        state['phase']='RESTART';save(Path(base)/'checkpoint.json',state)
        state['restart']=stage.restart()
        state['context']=context(stage);state['phase']='POST_RESTART';state['status']='ROLLED_BACK';state['fence_status']='RELEASED'
        save(Path(base)/'checkpoint.json',state)
        return state
    except Exception as error:
        state['status']='BLOCKED';state['fence_status']='UNKNOWN';state['error']=safe_error(error)
        try:
            if stage.refence().get('verified') is True:state['fence_status']='VERIFIED'
        except Exception:pass
        state['context']=context(stage);save(Path(base)/'checkpoint.json',state)
        raise
