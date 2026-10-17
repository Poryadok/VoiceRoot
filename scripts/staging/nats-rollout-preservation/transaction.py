"""Finite nonreset transaction; root owns prepare/off-node checkpoint/finish.

These entrypoints accept captured immutable code and target inputs only. They
never allocate a claim or execute bootstrap against existing staging data.
"""
import copy
import datetime as dt
import json
import os
from pathlib import Path
import re
import secrets
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
from root_main import NativeCaptureReader,capture_inputs,revalidate_inputs,save as root_save,private_json
from docker_runtime import DockerRuntime
from runtime_stage import RolloutStage
from preserve import capture_cut, verify_post_apply, canonical
from normalize import normalize_target,image_only_documents
from errors import safe_error
from nats_migration import apply_contract
import rollout_census

def save(path,row):
    root_save(path,row,limit=128<<20)
    if Path(path).name=='checkpoint.json':
        import deployment_report
        deployment_report.checkpoint(row)


def rollback_services(previous,*,story_only=False):
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
        return ['story'] if story_only else sorted(catalog)
    names=target.get('changed_services')
    if target.get('mode')!='images-only' or not isinstance(names,list) or not names or any(not isinstance(n,str) for n in names) or len(set(names))!=len(names) or not set(names)<=catalog:
        raise Blocked('rollout_rollback_service_selection_invalid')
    return sorted(names)


def rollback_package(previous,*,story_only=False):
    full_story=story_only and previous.get('input_target',{}).get('mode')=='full'
    if previous.get('status')!='PASS' or (previous['input_target']['mode']!='images-only' and not full_story):
        raise Blocked('rollout_rollback_database_compatibility_required')
    selected=rollback_services(previous,story_only=story_only)
    backends=set(selected)-{'web','admin','developer-portal'}
    if backends:
        import story_witness
        authority=previous.get('service_actor_authority') or {}
        candidate=authority.get('compatible_story_candidate') or {}
        image=previous.get('context',{}).get('old_images',{}).get('voice-story/story')
        schema=authority.get('story_schema')
        content=(candidate.get('child_sha256'),candidate.get('config_sha256'),candidate.get('component_source_sha'))
        try:selected_authority=story_witness.approve_content(content,candidate.get('root_witness_authority'))
        except (Blocked,OSError):raise Blocked('rollout_rollback_backend_candidate_unproved') from None
        expected={'schema':'voice-reviewed-story-compatible-pair-v1','image':image,
            'child_sha256':content[0],'config_sha256':content[1],
            'component_source_sha':content[2],'schema_sha256':canonical(schema),
            'actor_sha256':canonical({'binding':authority.get('binding'),'mount':authority.get('mounts',{}).get('story'),'proof':authority.get('proofs',{}).get('story')})}
        if selected_authority is not None:expected['root_witness_authority']=selected_authority
        if (backends!={'story'} or not isinstance(image,str) or not __import__('re').fullmatch(r'ghcr\.io/poryadok/voiceroot/story@sha256:[a-f0-9]{64}',image) or schema is None
            or 'story' not in authority.get('roles',[]) or 'story' not in authority.get('proofs',{})
            or candidate!=expected):
            raise Blocked('rollout_rollback_backend_candidate_unproved')
    target=copy.deepcopy(previous['input_target']);rows=[];images={}
    if full_story:
        target['mode']='images-only';target['changed_services']=['story']
        target['template_hashes']={'voice-story':previous['target']['template_hashes']['voice-story']}
    for name in target['template_hashes']:
        if name==renderer_root.HUB:
            if previous.get('renderer_authority') is None:raise Blocked('renderer_rollback_authority_required')
            # Root bridge captures a fresh renderer descriptor and proof. The
            # application runner never receives a hub Deployment document.
            continue
        old=previous['context']['snapshots'][name] if full_story else previous['context']['original_snapshots'][name]
        if full_story and canonical(old['spec']['template'])!=previous['target']['template_hashes'][name]:
            raise Blocked('rollout_rollback_current_template_changed')
        row={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':name,'namespace':guard.NS},
             'spec':copy.deepcopy(old['spec'])}
        template=row['spec']['template']
        for c in template['spec'].get('initContainers',[])+template['spec']['containers']:
            key=name+'/'+c['name']
            image=(previous['context']['old_images'].get(key,'') if not full_story or key=='voice-story/story' else previous['target']['images'].get(key,''))
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


def validate_full_story_rollback_schema(previous,current):
    # Fresh actor preflight reads the current database; the historical proof
    # only identifies the reviewed predecessor, never authorizes a downgrade.
    import actor_root
    rollback_package(previous,story_only=True)
    for authority in (previous.get('service_actor_authority') or {},current):
        schema=authority.get('story_schema') or {}
        if (schema.get('observation')!={'database':'story_db','version':4,'dirty':False}
            or schema.get('migration_hashes')!=actor_root.STORY_UP):
            raise Blocked('rollout_rollback_database_compatibility_required')


def context(stage):
    result={k:copy.deepcopy(getattr(stage,k)) for k in ('expected','snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv')}
    # A native stage deliberately has no host-visible PV path. Keep the legacy
    # path only in legacy contexts; reading it here would reintroduce the
    # forbidden hostPath authority into checkpoints and journal callbacks.
    if getattr(stage,'native_api',None) is None:
        result['final_path']=str(stage.final_path)
    else:
        pv=result['final_pv']
        if (not isinstance(pv,dict) or set(pv)!={'metadata'}
            or not isinstance(pv['metadata'],dict)
            or set(pv['metadata'])-{'name','uid','resourceVersion'}):
            raise Blocked('rollout_native_pv_path_forbidden')
        if getattr(stage,'source_identity',None) is not None:
            result['source_identity']=copy.deepcopy(stage.source_identity)
    if 'original_live_census' in vars(stage):
        result['original_live_census']=rollout_census.validate_snapshot(vars(stage)['original_live_census'])
    return result


def _shutdown_census_intent(events):
    if not isinstance(events,list):raise Blocked('rollout_original_live_census_binding_invalid')
    intents=[event for event in events if isinstance(event,dict) and event.get('kind')=='original_hub_shutdown_intent']
    if len(intents)>1:raise Blocked('rollout_original_live_census_binding_invalid')
    return intents[0] if intents else None


def require_original_live_census(stage,events):
    snapshot=vars(stage).get('original_live_census')
    intent=_shutdown_census_intent(events)
    if snapshot is None or intent is None:raise Blocked('rollout_original_live_census_unavailable')
    try:validated=rollout_census.validate_snapshot(snapshot)
    except rollout_census.CensusError:raise Blocked('rollout_original_live_census_invalid') from None
    wanted=intent.get('live_census_sha256')
    if not isinstance(wanted,str) or not re.fullmatch(r'[a-f0-9]{64}',wanted) or canonical(validated)!=wanted:
        raise Blocked('rollout_original_live_census_binding_invalid')
    return validated


def reconstruct(kube,state,journal,native_api=None):
    ctx=state['context']
    if native_api is not None:
        if 'final_path' in ctx:
            raise Blocked('rollout_native_context_contains_host_path')
        import native_runtime_stage
        stage=native_runtime_stage.NativeRolloutStage(native_api,state['operation'],
            ctx['expected'],journal,history=state.get('events',[]),account_id=None)
        # Keep the authority marker explicit for context() and downstream
        # guards. The typed client is injected by the trusted bridge only.
        stage.native_api=native_api
    else:
        if 'final_path' not in ctx:
            raise Blocked('rollout_native_authority_missing')
        stage=RolloutStage(kube,state['operation'],ctx['expected'],journal)
    for name in ('snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv'):
        setattr(stage,name,copy.deepcopy(ctx[name]))
    if native_api is None:
        stage.final_path=Path(ctx['final_path'])
    elif 'source_identity' in ctx:
        stage.source_identity=copy.deepcopy(ctx['source_identity'])
    if 'native_source_binding' in state:
        if native_api is None:
            raise Blocked('rollout_native_authority_missing')
        import kube_json_api
        try:kube_json_api.resolve_source_binding(native_api,operation=state['operation'],expected=state['native_source_binding'])
        except kube_json_api.KubeAPIError:raise Blocked('rollout_native_source_binding_changed') from None
        stage.native_api=native_api
    snapshot=ctx.get('original_live_census')
    intent=_shutdown_census_intent(state.get('events',[]))
    if snapshot is not None:
        if intent is None:raise Blocked('rollout_original_live_census_binding_invalid')
        try:validated=rollout_census.validate_snapshot(snapshot)
        except rollout_census.CensusError:raise Blocked('rollout_original_live_census_invalid') from None
        wanted=intent.get('live_census_sha256')
        if not isinstance(wanted,str) or not re.fullmatch(r'[a-f0-9]{64}',wanted) or canonical(validated)!=wanted:
            raise Blocked('rollout_original_live_census_binding_invalid')
        stage.original_live_census=validated
    elif intent is not None:
        # Digest-only historical operations remain readable for already-proven
        # later phases. They fail only if a pre-cut census is required.
        pass
    if state.get('renderer_authority') is not None:
        stage.renderer_transition=copy.deepcopy(state['renderer_authority']['descriptor'])
        stage.renderer_post_start=lambda current:renderer_root.verify_live(current,state['renderer_authority'])
    if native_api is not None:
        stage.recover()
    return stage


def checkpoint_journal(base,state,stage_getter):
    def journal(event):
        stage=stage_getter()
        event=copy.deepcopy(event)
        if isinstance(event,dict) and event.get('kind')=='original_hub_shutdown_intent':
            if stage is None or 'original_live_census' not in vars(stage):
                raise Blocked('rollout_original_live_census_unavailable')
            try:snapshot=rollout_census.validate_snapshot(vars(stage)['original_live_census'])
            except rollout_census.CensusError:raise Blocked('rollout_original_live_census_invalid') from None
            wanted=event.get('live_census_sha256')
            if not isinstance(wanted,str) or not re.fullmatch(r'[a-f0-9]{64}',wanted) or canonical(snapshot)!=wanted:
                raise Blocked('rollout_original_live_census_binding_invalid')
            stage.original_live_census=snapshot
        state['events'].append(event)
        if stage is not None:
            state['context']=context(stage)
        save(Path(base)/'checkpoint.json',state)
    return journal


def prepare(kube,base,code,target,contract,operation,code_capture,migration_plan,non_nats,before_fence=None,nats_authority=None,actor_authority=None,actor_services=None,renderer_authority=None,lifecycle=None,gis_first_install=None,gis_source_root=None,gis_source_capture=None,gis_source_authority=None,gis_transition=None,native_api=None,native_config=None):
    # base creation/root lock/captured code custody belongs to the root launcher.
    base=Path(base);code=Path(code)
    state={'schema':'nats-rollout-root-v1','operation':operation,'phase':'READ_ONLY_CAPTURE',
           'status':'RUNNING','fence_status':'UNKNOWN','events':[],'target':copy.deepcopy(target),
           'code_capture':copy.deepcopy(code_capture),'contract':copy.deepcopy(contract),
           'input_target':copy.deepcopy(target),
           'service_actor_authority':copy.deepcopy(actor_authority),
           'renderer_authority':copy.deepcopy(renderer_authority),
           'service_actor_services':list(actor_services or [])}
    native_source=None
    if native_api is not None:
        if not isinstance(native_config,dict) or not isinstance(native_config.get('phase_image'),str):
            raise Blocked('rollout_native_authority_missing')
        import kube_json_api
        try:native_source=kube_json_api.resolve_source_binding(native_api,operation=operation)
        except kube_json_api.KubeAPIError:raise Blocked('rollout_native_source_binding_invalid') from None
        state['native_source_binding']=copy.deepcopy(native_source)
    if gis_first_install is not None:
        import gis_first_install as gis
        if gis_source_root is None or not isinstance(gis_source_capture,dict):
            raise Blocked('gis_first_install_source_capture_missing')
        if target.get('gis_first_install_sha256') != gis_first_install.get('source_plan_sha256'):
            raise Blocked('gis_first_install_target_binding_invalid')
        state['gis_first_install']=copy.deepcopy(gis_first_install)
        authority=gis_source_authority
        if (not isinstance(authority,dict) or authority.get('verified') is not True
            or authority.get('source_sha')!=gis_first_install.get('source_sha')
            or type(authority.get('run_id')) is not int or authority['run_id']<=0
            or type(authority.get('run_attempt')) is not int or authority['run_attempt']<=0
            or type(authority.get('workflow_id')) is not int
            or not isinstance(authority.get('tree_sha'),str) or not re.fullmatch(r'[a-f0-9]{40}',authority['tree_sha'])
            or type(authority.get('lock_artifact_id')) is not int or authority['lock_artifact_id']<=0
            or not isinstance(authority.get('lock_sha256'),str) or not re.fullmatch(r'[a-f0-9]{64}',authority['lock_sha256'])):
            raise Blocked('gis_first_install_source_authority_invalid')
        state['gis_first_install']['source_ci']={key:authority[key] for key in
            ('run_id','run_attempt','workflow_id','tree_sha','lock_artifact_id','lock_sha256')}
        import gis_prerequisite
        inputs=gis_prerequisite.source_inputs(Path(gis_source_root),gis_source_capture)
        required_capture={path:gis_source_capture[path] for path in (
            'scripts/dev/migrations.py','docker/postgres/provision-gameintegration-runtime-role.sql',
            'docker/postgres/verify-gameintegration-runtime-role.sql',
            'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
            'deploy/templates/migrate-game-integration-db-job.yaml',
            'deploy/templates/verify-game-integration-db-job.yaml')}
        required_capture.update({'src/backend/migrations/game_integration_db/'+row['name']:row['sha256']
                                 for row in inputs['migrations']})
        state['gis_first_install']['inputs']=inputs
        state['gis_first_install']['inputs_sha256']=gis.digest(inputs)
        state['gis_first_install']['source_root']=str(Path(gis_source_root))
        state['gis_first_install']['source_capture']=required_capture
    if gis_transition is not None:
        if gis_first_install is not None:
            raise Blocked('gis_first_install_transition_admission_conflict')
        import gis_first_install as gis
        gis.validate_transition_binding(gis_transition,kube)
        state['gis_transition']=copy.deepcopy(gis_transition)
    if gis_first_install is None and target.get('mode') in ('full','app-only'):
        try:active_release=private_json(guard.ROOT/'installed'/'active-release.json')
        except FileNotFoundError:active_release={}
        if ('gis_first_install_transition' in active_release and gis_transition is None):
            raise Blocked('gis_first_install_transition_required')
    if lifecycle is not None:
        if set(lifecycle)-{'provisional','repair_parent','source_authority','execution_authority'}:
            raise Blocked('provisional_binding_invalid')
        state.update(copy.deepcopy(lifecycle))
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    save(base/'checkpoint.json',state)
    try:
        manifests=json.loads((base/'apply-manifests.json').read_bytes())
        if guard.frontend_image_only(target) and (migration_plan or non_nats['actions']):
            raise Blocked('rollout_frontend_auxiliary_action_forbidden')
        snapshot_names={r['metadata']['name'] for r in manifests if r.get('kind')=='Deployment'}
        if gis_first_install is not None:
            # The add-only target contains only the new GIS pair; independently
            # retain every pre-existing workload in the preservation cut.
            existing={obj.get('name') for row in non_nats.get('checks',[]) for obj in row.get('objects',[])
                if obj.get('kind')=='Deployment'}
            if len(existing)!=22 or 'voice-gameintegration' in existing:
                raise Blocked('gis_first_install_baseline_workload_count_invalid')
            # The new GIS Deployment does not exist yet and is managed by its
            # create-only receipt. The preservation stage snapshots the 22
            # existing workloads, then verifies those same UIDs/templates
            # after the GIS-only target has been applied.
            snapshot_names=existing
        try:enrollment=private_json(guard.ROOT/'bootstrap-enrollment.json')
        except FileNotFoundError:enrollment=None
        account_id=None
        if native_api is not None:
            import native_runtime_stage
            generation=native_source['proof']['generation']
            account_capture=[]
            state['provenance']=capture_inputs(
                NativeCaptureReader(native_api,generation,contract),base,contract,generation,
                bootstrap_enrollment=enrollment,public_account_callback=account_capture.append,
                native_private=True)
            if len(account_capture)!=1:
                raise Blocked('rollout_native_account_capture_invalid')
            account_id=account_capture[0]
            core={native_runtime_stage.HUB,'voice-gateway',
                *('voice-'+name for name in native_runtime_stage.LEAVES)}
            extras=sorted(snapshot_names-core)
            stage=native_runtime_stage.NativeRolloutStage.capture(native_api,operation,journal,
                extras,expected_identity=None,history=state['events'],account_id=account_id)
            stage.native_api=native_api
        else:
            stage=RolloutStage.capture(kube,operation,journal,
                sorted(snapshot_names)+([renderer_root.HUB] if renderer_authority is not None else []))
        if native_source is not None:
            source=native_source['binding']
            if (stage.expected['source_claim']!=source['source_claim']
                or stage.expected['source_claim_uid']!=source['source_uid']
                or stage.expected['source_pv_uid']!=source['source_pv_uid']
                or stage.expected['generation']!=native_source['proof']['generation']
                or stage.expected['namespace_uid']!=native_source['proof']['namespace_uid']
                or stage.expected['marker_uid']!=native_source['proof']['marker_uid']
                or stage.expected['source_node_uid']!=native_source['proof']['node_uid']):
                raise Blocked('rollout_native_source_binding_mismatch')
        if target['mode']=='images-only':
            manifests,target=image_only_documents(stage,manifests,target,renderer_authority=renderer_authority)
            save(base/'apply-manifests.json',manifests)
            target['manifest_sha256']=file_sha(base/'apply-manifests.json')
        state['target']=normalize_target(kube,stage,manifests,target,renderer_authority=renderer_authority)
        if canonical(migration_plan)!=target['migration_sha256']:raise Blocked('rollout_target_migration_digest_invalid')
        state['migrations']=copy.deepcopy(migration_plan)
        state['migration_secret_metadata']=migrations.preflight(kube,migration_plan,target['mode'])
        if state.get('gis_first_install') is not None:
            import gis_first_install as gis
            gis.preflight(kube,state['gis_first_install'])
            import gis_prerequisite
            gis_prerequisite.preflight(kube,state['gis_first_install'])
        state['space_preflight']=space_safeguards.preflight(kube,base,migration_plan)
        if canonical(non_nats)!=target['nonnats_sha256']:raise Blocked('rollout_target_nonnats_digest_invalid')
        state['nonnats']=copy.deepcopy(non_nats)
        state['nonnats_binding']=nonnats_runtime.preflight(kube,non_nats,target['mode'])
        import source_plan
        source_plan.verify_db_init(kube,non_nats)
        state['context']=context(stage)
        if native_api is not None:
            claim=native_api.get_pvc(namespace='voice-staging',name=stage.expected['source_claim'])
        else:
            claim=kube.get('pvc',stage.expected['source_claim'])
        capacity=claim['status']['capacity']['storage']
        if native_api is None:
            state['native_capacity']=native_preflight(stage.final_path,base,capacity)
        else:
            # The native helper reports an authenticated manifest over the
            # operation-bound archive stream. A PV request is not capacity,
            # and there is no host PV path to scan for used bytes.
            if not isinstance(capacity,str):raise Blocked('rollout_native_capacity_readback_invalid')
            state['native_capacity']={'reported_capacity':capacity,'max_bytes':48<<30,
                'max_files':262144,'source_pvc_uid':stage.expected['source_claim_uid'],
                'source_pv_uid':stage.expected['source_pv_uid']}
            if state['space_preflight'] is not None and state['space_preflight']['requirement']['backup_required']:
                raise Blocked('rollout_native_space_backup_capacity_adapter_required')
            state['combined_space_capacity']=None
        if native_api is None:
            state['combined_space_capacity']=space_safeguards.combined_capacity(base,state['native_capacity'],state['space_preflight'])
        if state['space_preflight'] is not None:
            state['space_preflight']['source_sha']=target['tag']
            state['space_preflight']['migration_plan_sha256']=target['migration_sha256']
        if native_api is None:
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
    except Exception as error:
        import deployment_report
        deployment_report.capture_failure(error)
        from operator_retirement import record_readonly_failure
        failed=record_readonly_failure(base,state,error)
        save(base/'checkpoint.json',failed)
        raise
    state['phase']='FENCE';save(base/'checkpoint.json',state)
    try:
        nonnats_plan.revalidate(kube,state['nonnats_binding'])
        if before_fence is not None:before_fence()
        stage.runtime_base=base
        stage.fence();state['context']=context(stage);state['fence_status']='VERIFIED'
        if native_source is not None:
            import kube_json_api
            try:kube_json_api.resolve_source_binding(native_api,operation=operation,expected=native_source)
            except kube_json_api.KubeAPIError:raise Blocked('rollout_native_source_binding_changed') from None
        save(base/'checkpoint.json',state)
        if native_api is None:
            runtime=DockerRuntime(base,operation)
        else:
            import tokenless_job
            source=native_source['binding']
            legacy_binding={'claim_name':source['source_claim'],'claim_uid':source['source_uid'],
                'pv_name':source['source_pv_name'],'pv_uid':source['source_pv_uid'],
                'node_name':source['source_node_name']}
            def persist_native_phase(event):
                import kube_json_api
                try:
                    kube_json_api.resolve_source_binding(native_api,operation=operation,expected=native_source)
                    stage.verify_final_storage()
                except (kube_json_api.KubeAPIError, Exception) as error:
                    raise Blocked('rollout_native_source_changed_before_phase') from None
                journal({'phase':'COLD_BACKUP',**event})
            runtime=tokenless_job.PvcArchiveRuntime(native_api,base,legacy_binding,
                native_config['phase_image'],operation,secrets.token_hex(32),
                persist_intent=persist_native_phase,
                persist_result=persist_native_phase,
                account_id=account_id)
        state['phase']='COLD_BACKUP';save(base/'checkpoint.json',state)
        live_catalog=require_original_live_census(stage,state['events'])
        state['cut']=capture_cut(runtime,None if native_api is not None else stage.final_path,stage.verify_final_storage,
            live_catalog=live_catalog)
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
        import deployment_report
        deployment_report.capture_failure(error)
        state['context']=context(stage);state['status']='BLOCKED';state['fence_status']='UNKNOWN'
        state['error']=safe_error(error)
        try:
            if stage.refence().get('verified') is True:state['fence_status']='VERIFIED'
        except Exception:pass
        save(base/'checkpoint.json',state);raise


def execute_gis_first_install(kube,base,state,stage,journal):
    """Collect prerequisites and create only the paused GIS pair before standard apply."""
    import gis_prerequisite
    import gis_first_install as gis
    custody=state.get('custody')
    if not isinstance(custody,dict) or custody.get('verified') is not True:
        raise Blocked('gis_first_install_preservation_custody_missing')
    custody_binding={
        'operation':state['operation'],
        'namespace_uid':stage.expected['namespace_uid'],
        'marker_uid':stage.expected['marker_uid'],
        'claim_uid':stage.expected['source_claim_uid'],
        'pv_uid':stage.expected['source_pv_uid'],
        'target_sha':state['target']['tag'],
        'archive_sha256':state['cut']['manifest']['archive_sha256'],
        'manifest_sha256':state['cut']['manifest_sha256'],
        'census_sha256':state['cut']['census_sha256'],
        'baseline_workload_count':len(state['context']['snapshots']),
        'baseline_snapshots_sha256':gis.digest(state['context']['snapshots']),
        'off_node_verified':True,
    }
    if custody_binding['baseline_workload_count'] != 22:
        raise Blocked('gis_first_install_baseline_workload_count_invalid')
    state['gis_first_install']['custody']=copy.deepcopy(custody_binding)
    state['gis_first_install']['custody_binding_sha256']=gis.digest(custody_binding)
    def save_gis_plan(value):
        state['gis_first_install']=copy.deepcopy(value)
        save(Path(base)/'checkpoint.json',state)
        journal({'kind':'gis_first_install_checkpoint','phase':value.get('phase')})
    stage.verify_final_storage()
    state['gis_first_install']['prerequisite_receipt']=gis_prerequisite.execute(
        kube,state['gis_first_install'],Path(state['gis_first_install']['source_root']),
        state['gis_first_install']['source_capture'],save_gis_plan,
        verify_storage=stage.verify_final_storage)
    save(Path(base)/'checkpoint.json',state)
    stage.verify_final_storage()
    state['gis_first_install']['resource_receipt']=gis.execute(
        kube,state['gis_first_install'],save_gis_plan,start=False)
    state['gis_first_install_receipt']=copy.deepcopy(state['gis_first_install']['resource_receipt'])
    save(Path(base)/'checkpoint.json',state)
    return state['gis_first_install_receipt']


def rollback_gis_first_install(kube,base,state,journal):
    """Remove only journaled GIS objects; retain completed additive DB prerequisites."""
    plan=state.get('gis_first_install')
    if not isinstance(plan,dict):return False
    import gis_first_install as gis
    def save_plan(value):
        state['gis_first_install']=copy.deepcopy(value)
        save(Path(base)/'checkpoint.json',state)
        journal({'kind':'gis_first_install_rollback_checkpoint','phase':value.get('phase')})
    gis.rollback(kube,plan,save_plan)
    state['gis_first_install_rollback']={'schema':'voice-gis-first-install-rollback-v1',
        'operation':state['operation'],'phase':'ROLLED_BACK',
        'remaining_database_prerequisite':bool(plan.get('prerequisite_receipt'))}
    save(Path(base)/'checkpoint.json',state)
    return True


def authorize(kube,base,state,off_node_archive_sha,off_node_manifest_sha,contract,*,authorization_deadline=None,expired_authority=None):
    def current_deadline():
        if expired_authority is not None:
            import expired_recovery
            actual,_=expired_recovery.runtime_authority(base,state,expired_authority,now=dt.datetime.now(dt.timezone.utc))
            if authorization_deadline!=dt.datetime.fromisoformat(actual['expires_at']):
                raise Blocked('expired_recovery_deadline_changed')
            return
        if authorization_deadline is None:return
        now=dt.datetime.now(dt.timezone.utc)
        if (not isinstance(authorization_deadline,dt.datetime) or authorization_deadline.tzinfo is None
            or not now<authorization_deadline<=now+dt.timedelta(seconds=600)
            or 'nats_contract_expires_at' not in state
            or authorization_deadline>dt.datetime.fromisoformat(state['nats_contract_expires_at'])):
            raise Blocked('preserved_upload_authorization_expired')
    current_deadline()
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
    if 'nats_contract' in state and expired_authority is None:
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
    if state.get('gis_transition') is not None:
        import gis_first_install as gis
        gis.validate_transition_binding(state['gis_transition'],kube)
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
    current_deadline()
    if expired_authority is not None:
        import expired_recovery,expired_proof
        actual,proof=expired_recovery.runtime_authority(base,state,expired_authority,now=dt.datetime.now(dt.timezone.utc))
        adopted=copy.deepcopy(state)
        def save_entry(event):
            state['expired_recovery_authority']=copy.deepcopy(actual)
            journal(event)
        expired_recovery.enter(actual,adopted,proof,actual['execution'],
            lambda:expired_proof.current_inventory(DockerRuntime(base,state['operation']),stage),
            save_entry,lambda:None,now=dt.datetime.now(dt.timezone.utc),
            clock=lambda:dt.datetime.now(dt.timezone.utc))
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
            if expired_authority is None:
                state['nats_migration']=apply_contract(base,state,stage,journal)
            else:
                def before_selected_start():
                    current_deadline()
                    _,proof=expired_recovery.runtime_authority(base,state,expired_authority,now=dt.datetime.now(dt.timezone.utc))
                    if expired_proof.current_inventory(DockerRuntime(base,state['operation']),stage)!=proof['selected_inventory_sha256']:
                        raise Blocked('expired_recovery_selected_store_changed_before_start')
                    current_deadline()
                state['nats_migration']=apply_contract(base,state,stage,journal,before_selected_start=before_selected_start,mutation_guard=current_deadline)
            save(Path(base)/'checkpoint.json',state)
            from nats_contract_custody import advance
            active=(advance(kube,state,stage,journal,mutation_guard=current_deadline) if expired_authority is not None
                else advance(kube,state,stage,journal))
            current_deadline()
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
        if state.get('gis_first_install') is not None:
            execute_gis_first_install(kube,base,state,stage,journal)
        state['space_postcondition']=space_safeguards.after_migrate(kube,state.get('space_gate'))
        save(Path(base)/'checkpoint.json',state)
        if state.get('renderer_authority') is not None:
            state['phase']='RENDERER_TEMPLATE';save(Path(base)/'checkpoint.json',state)
            renderer_root.apply_paused(stage,state['renderer_authority'])
            state['renderer_applied']=True;save(Path(base)/'checkpoint.json',state)
        current_deadline()
        stage.prepared()
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
        if state.get('provisional') is not None:
            import provisional
            receipt['provisional']=copy.deepcopy(provisional.validate(state['provisional']))
        if authorization_deadline is not None:
            current_deadline()
            receipt['expires_at']=(authorization_deadline if expired_authority is not None else
                min(authorization_deadline,dt.datetime.fromisoformat(state['nats_contract_expires_at']))).isoformat()
        if expired_authority is not None:
            receipt['expired_recovery_authority_sha256']=canonical(expired_authority)
            expired_recovery.publish_apply_authority(base,state,expired_authority,receipt,now=dt.datetime.now(dt.timezone.utc))
            current_deadline()
        save(Path(base)/'apply-authorization.json',receipt)
        state['authorization']=copy.deepcopy(receipt)
        state['context']=context(stage);state['phase']='PAUSED_APPLY';state['status']='WAITING'
        save(Path(base)/'checkpoint.json',state)
        return receipt
    except Exception as error:
        import deployment_report
        deployment_report.capture_failure(error)
        state['status']='BLOCKED';state['fence_status']='UNKNOWN'
        state['error']=safe_error(error)
        try:
            if stage.refence().get('verified') is True:state['fence_status']='VERIFIED'
        except Exception:pass
        if state.get('gis_first_install') is not None and state['fence_status']=='VERIFIED':
            try:
                if rollback_gis_first_install(kube,base,state,journal):
                    state['status']='ROLLED_BACK';state['phase']='GIS_FIRST_INSTALL_ROLLED_BACK'
            except Exception:pass
        state['context']=context(stage);save(Path(base)/'checkpoint.json',state);raise


def finish(kube,base,state,receipt,claim_rv,contract,*,finish_authority=None,publication=None):
    import provisional
    provisional.receipt_binding(state,receipt)
    if finish_authority is not None:
        return finish_only(kube,base,state,receipt,contract,finish_authority,publication=publication)
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
    active_recovery=state.get('expired_recovery_authority')
    def resume_authority():
        if dt.datetime.now(dt.timezone.utc)>=expires:raise Blocked('rollout_finish_authorization_expired')
        if active_recovery is None:return
        import expired_recovery
        authority,_=expired_recovery.runtime_authority(base,state,active_recovery,now=dt.datetime.now(dt.timezone.utc))
        if (receipt.get('expired_recovery_authority_sha256')!=canonical(active_recovery)
            or dt.datetime.fromisoformat(authority['expires_at'])!=expires):
            raise Blocked('rollout_finish_recovery_authority_changed')
        if dt.datetime.now(dt.timezone.utc)>=expires:raise Blocked('rollout_finish_authorization_expired')
    if active_recovery is not None:
        resume_authority()
        stage.finish_guard=lambda action,name=None,replicas=None:resume_authority()
        stage.finish_authority_guard=resume_authority
    # No replica can restart until exact target templates and unchanged data
    # are proved. Original templates remain separately bound for rollback.
    revalidate_inputs(kube,contract,state['provenance'])
    stage.adopt_applied_target(receipt,claim_rv)
    if state.get('gis_transition') is not None:
        import gis_first_install as gis
        gis.validate_transition_binding(state['gis_transition'],kube)
    if 'nonnats' in state:state['nonnats_applied']=nonnats_runtime.verify(kube,state['nonnats'],state['nonnats_binding'],stage,applied=True)
    state['context']=context(stage);state['phase']='POST_APPLY_PROOF';save(Path(base)/'checkpoint.json',state)
    try:
        runtime=DockerRuntime(base,state['operation'])
        baseline=state.get('nats_migration',{}).get('cut',state['cut'])
        state['preservation']=verify_post_apply(runtime,stage.final_path,baseline,stage.verify_final_storage)
        stage.verified();state['context']=context(stage);save(Path(base)/'checkpoint.json',state)
        state['phase']='RESTART';save(Path(base)/'checkpoint.json',state)
        attach_closed_preservation(base,state,stage,resume_authority)
        state['restart']=stage.restart()
        resume_authority()
        if state.get('gis_transition') is not None:
            import gis_first_install as gis
            gis.validate_transition_binding(state['gis_transition'],kube)
        if state.get('gis_first_install') is not None:
            import gis_first_install as gis
            gis_plan=state['gis_first_install']
            def persist_gis_plan(value):
                state['gis_first_install']=copy.deepcopy(value)
                state['gis_first_install_receipt']=copy.deepcopy(value.get('receipt'))
                save(Path(base)/'checkpoint.json',state)
                journal({'kind':'gis_first_install_checkpoint','phase':value.get('phase')})
            gis.start_owned(kube,gis_plan,persist_gis_plan)
            state['gis_first_install_receipt']=copy.deepcopy(gis_plan['receipt'])
            save(Path(base)/'checkpoint.json',state)
            ready=gis.verify_ready(kube,gis_plan)
            transition=gis.transition_receipt(gis_plan,ready,
                source_ci=gis_plan.get('source_ci'),
                custody=gis_plan.get('custody'),
                target_mode=state['target']['mode'])
            gis.validate_transition_receipt(transition,state)
            state['gis_first_install_ready']=copy.deepcopy(ready)
            state['gis_first_install_transition_receipt']=copy.deepcopy(transition)
            save(Path(base)/'checkpoint.json',state)
            resume_authority()
        state['context']=context(stage)
        if publication is not None:publication(state,authority_guard=resume_authority)
        resume_authority()
        state['context']=context(stage);state['phase']='POST_RESTART';state['status']='PROVISIONAL_READY' if state.get('provisional') is not None else 'PASS';state['fence_status']='RELEASED'
        save(Path(base)/'checkpoint.json',state)
        return state
    except Exception as error:
        import deployment_report
        deployment_report.capture_failure(error)
        state['status']='BLOCKED';state['fence_status']='UNKNOWN'
        state['error']=safe_error(error)
        # refence itself rechecks exact marker ownership before every mutation.
        # Never claim VERIFIED solely because the preceding phase was paused.
        try:
            observed=stage.refence()
            if observed.get('verified') is True:state['fence_status']='VERIFIED'
        except Exception:pass
        if state.get('gis_first_install') is not None and state['fence_status']=='VERIFIED':
            try:
                if rollback_gis_first_install(kube,base,state,journal):
                    state['status']='ROLLED_BACK';state['phase']='GIS_FIRST_INSTALL_ROLLED_BACK'
            except Exception:pass
        state['context']=context(stage);save(Path(base)/'checkpoint.json',state)
        raise


def attach_closed_preservation(base,state,stage,authority,*,nonce=None,admitted_cut=None,finish_authority_sha256=None,startup_deadline=None):
    """ROOT-owned attempt identity shared by ordinary/native finish consumers.

    This is a seal attempt, not an upload/application attempt or a replacement
    authority. Its identity is saved before any isolated HUB start. Retry must
    retain the private journal head and cannot adopt an unknown partial effect.
    """
    import secrets
    from preserve import ColdRelease
    authority()
    attempts=state.setdefault('closed_preservation_attempts',{})
    if type(attempts) is not dict or len(attempts)>64:
        raise Blocked('closed_preservation_attempt_changed')
    chosen=nonce if nonce is not None else state.get('closed_preservation_attempt_nonce')
    if chosen is None:chosen=secrets.token_hex(32)
    if type(chosen) is not str or not guard.SHA.fullmatch(chosen):
        raise Blocked('closed_preservation_attempt_changed')
    saved=attempts.get(chosen)
    if saved is None:
        if len(attempts)==64:raise Blocked('closed_preservation_attempt_changed')
        saved={'nonce':chosen,'authorization_sha256':canonical(state['authorization'])}
        if admitted_cut is not None:
            if not guard.SHA.fullmatch(finish_authority_sha256 or ''):
                raise Blocked('closed_preservation_current_authority_missing')
            saved.update(admitted_cut_sha256=canonical(admitted_cut),finish_authority_sha256=finish_authority_sha256)
        attempts[chosen]=saved
        state['closed_preservation_attempt_nonce']=chosen
        save(Path(base)/'checkpoint.json',state)
    fields={'nonce','authorization_sha256'}
    if admitted_cut is not None:fields|={'admitted_cut_sha256','finish_authority_sha256'}
    if (type(saved) is not dict or set(saved)!=fields
        or not guard.SHA.fullmatch(saved['nonce'])
        or saved['authorization_sha256']!=canonical(state['authorization'])
        or nonce is not None and saved['nonce']!=nonce
        or admitted_cut is not None and (saved['admitted_cut_sha256']!=canonical(admitted_cut)
            or saved['finish_authority_sha256']!=finish_authority_sha256)):
        raise Blocked('closed_preservation_attempt_changed')
    if startup_deadline is None:
        startup_deadline=dt.datetime.fromisoformat(state['authorization']['expires_at'])
    if startup_deadline.tzinfo is None:raise Blocked('closed_preservation_deadline_invalid')
    authority()
    baseline=admitted_cut if admitted_cut is not None else state.get('nats_migration',{}).get('cut',state['cut'])
    # ColdRelease retains the exact saved nonce/authorization/current-cut
    # registry and source-derived short startup deadline at every release use.
    runtime=ColdRelease(base,state,stage,authority,baseline,saved['nonce'],startup_deadline)
    stage.closed_preservation_producer=runtime
    stage.closed_preservation_runtime=runtime
    authority()
    return runtime


def finish_only(kube,base,state,receipt,contract,reference,*,publication=None):
    # Expired application permission is preserved history. A separately loaded
    # root proof permits missing resumes only; never authorize/apply/migrate.
    if receipt!=state.get('authorization') or receipt.get('target')!=state['target']:
        raise Blocked('rollout_finish_authorization_changed')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal)
    import expired_finish
    engine=expired_finish.bind(base,state,reference,stage,journal)
    revalidate_inputs(kube,contract,state['provenance'])
    try:
        state['phase']='RESTART';save(Path(base)/'checkpoint.json',state)
        state['restart']=engine.restart()
        engine.final()
        state['context']=context(stage)
        if publication is not None:publication(state,authority_guard=engine.verify)
        engine.verify()
        state['context']=context(stage);state['phase']='POST_RESTART'
        state['status']='PASS';state['fence_status']='RELEASED'
        save(Path(base)/'checkpoint.json',state)
        return state
    except Exception as error:
        import deployment_report
        deployment_report.capture_failure(error)
        state['status']='BLOCKED';state['fence_status']='UNKNOWN';state['error']=safe_error(error)
        try:
            if stage.refence().get('verified') is True:
                state['fence_status']='VERIFIED'
                engine.safety_paused()
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
    def rollback_resume_authority():
        if receipt!=state.get('authorization') or receipt.get('target')!=state['target']:
            raise Blocked('rollout_rollback_authorization_changed')
        try:expires=dt.datetime.fromisoformat(receipt['expires_at'])
        except (KeyError,TypeError,ValueError):raise Blocked('rollout_rollback_authorization_invalid') from None
        if expires.tzinfo is None or dt.datetime.now(dt.timezone.utc)>=expires:
            raise Blocked('rollout_rollback_authorization_expired')
        revalidate_inputs(kube,contract,state['provenance'])
        released=getattr(stage,'finish_released_marker',None)
        if released is None:stage.owned_marker()
        elif stage.kube.get('configmap','voice-nats-generation')!=released:
            raise Blocked('rollout_rollback_released_marker_changed')
        stage.verify_selected_storage_identity()
    stage.finish_authority_guard=rollback_resume_authority
    stage.finish_guard=lambda action,name=None,replicas=None:rollback_resume_authority()
    try:
        rollback_resume_authority()
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
        attach_closed_preservation(base,state,stage,rollback_resume_authority)
        state['restart']=stage.restart()
        rollback_resume_authority()
        for name,captured in stage.snapshots.items():
            current=stage.kube.get('deployment',name);status=current.get('status',{})
            if (current['metadata']['uid']!=captured['metadata']['uid']
                or current['spec'].get('replicas',1)!=1
                or current['spec']['template']!=captured['spec']['template']
                or status.get('observedGeneration',0)<current['metadata']['generation']
                or any(status.get(field,0)!=1 for field in ('readyReplicas','updatedReplicas','availableReplicas'))):
                raise Blocked('rollout_rollback_final_workload_changed')
            rollback_resume_authority()
        state['context']=context(stage);state['phase']='POST_RESTART';state['status']='ROLLED_BACK';state['fence_status']='RELEASED'
        save(Path(base)/'checkpoint.json',state)
        return state
    except Exception as error:
        import deployment_report
        deployment_report.capture_failure(error)
        state['status']='BLOCKED';state['fence_status']='UNKNOWN';state['error']=safe_error(error)
        try:
            if stage.refence().get('verified') is True:state['fence_status']='VERIFIED'
        except Exception:pass
        state['context']=context(stage);save(Path(base)/'checkpoint.json',state)
        raise
