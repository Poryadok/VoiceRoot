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
from normalize import normalize_target,normalize_first_install,image_only_documents
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
    # A fresh rollback does not inherit a previous operation's bootstrap proof.
    target.pop('realtime_bootstrap_sha256',None)
    target.pop('prerequisites_sha256',None)
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
    result={k:copy.deepcopy(getattr(stage,k)) for k in ('expected','snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv')} | {'final_path':str(stage.final_path)}
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


def reconstruct(kube,state,journal,*,installation=None):
    ctx=state['context'];stage=RolloutStage(kube,state['operation'],ctx['expected'],journal)
    for name in ('snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv'):
        setattr(stage,name,copy.deepcopy(ctx[name]))
    stage.final_path=Path(ctx['final_path'])
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
        stage.renderer_post_start=lambda current:renderer_root.verify_live(current,state['renderer_authority'],installation=installation)
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


def verify_gis_before_fence(kube,state,*,paused=False):
    import gis_existing_update as existing
    existing.verify_phase(state['gis_binding'],kube,'paused-old' if paused else 'active')
    if existing.service_account_preflight(kube,state['gis_binding']['target']['spec']['template']['spec'])!=state['gis_service_account']:
        raise Blocked('gis_existing_service_account_changed')
    if private_json(guard.ROOT/'installed'/'active-release.json')!=state['gis_expected_active']:
        raise Blocked('gis_existing_authority_changed')


def prepare(kube,base,code,target,contract,operation,code_capture,migration_plan,non_nats,before_fence=None,nats_authority=None,actor_authority=None,actor_services=None,renderer_authority=None,lifecycle=None,gis_first_install=None,gis_source_root=None,gis_source_capture=None,gis_source_authority=None,gis_transition=None,resource_session=None,gis_binding=None,realtime_bootstrap_receipt=None,prerequisites_capture=None,preservation_ref=None):
    try:
        return _prepare(kube,base,code,target,contract,operation,code_capture,migration_plan,non_nats,
            before_fence=before_fence,nats_authority=nats_authority,actor_authority=actor_authority,
            actor_services=actor_services,renderer_authority=renderer_authority,lifecycle=lifecycle,
            gis_first_install=gis_first_install,gis_source_root=gis_source_root,gis_source_capture=gis_source_capture,
            gis_source_authority=gis_source_authority,gis_transition=gis_transition,resource_session=resource_session,
            gis_binding=gis_binding,realtime_bootstrap_receipt=realtime_bootstrap_receipt,prerequisites_capture=prerequisites_capture,preservation_ref=preservation_ref)
    except Exception as error:
        if realtime_bootstrap_receipt is not None or prerequisites_capture is not None:
            path=Path(base)/'checkpoint.json'
            if path.exists():
                state=private_json(path,limit=128<<20)
                if state.get('phase')=='READ_ONLY_CAPTURE':
                    state.update(status='BLOCKED',fence_status='UNKNOWN',error=safe_error(error))
                    save(path,state)
        raise

def prepare_preserved(*args,preservation_ref,resource_session,**kwargs):
    if preservation_ref is None or resource_session is None or kwargs.get('before_fence') is not None:
        raise Blocked('cohort_preserved_prepare_invalid')
    return prepare(*args,preservation_ref=preservation_ref,resource_session=resource_session,**kwargs)


def _prepare(kube,base,code,target,contract,operation,code_capture,migration_plan,non_nats,before_fence=None,nats_authority=None,actor_authority=None,actor_services=None,renderer_authority=None,lifecycle=None,gis_first_install=None,gis_source_root=None,gis_source_capture=None,gis_source_authority=None,gis_transition=None,resource_session=None,gis_binding=None,realtime_bootstrap_receipt=None,prerequisites_capture=None,preservation_ref=None):
    # base creation/root lock/captured code custody belongs to the root launcher.
    base=Path(base);code=Path(code)
    state={'schema':'nats-rollout-root-v1','operation':operation,'phase':'READ_ONLY_CAPTURE',
           'status':'RUNNING','fence_status':'UNKNOWN','events':[],'target':copy.deepcopy(target),
           'code_capture':copy.deepcopy(code_capture),'contract':copy.deepcopy(contract),
           'input_target':copy.deepcopy(target),
           'service_actor_authority':copy.deepcopy(actor_authority),
           'renderer_authority':copy.deepcopy(renderer_authority),
           'service_actor_services':list(actor_services or [])}
    if resource_session is not None:
        import resource_claim
        installation=resource_claim.require_installation(resource_session.installation)
        if target.get('installation_sha256')!=installation.sha256:raise Blocked('staging_installation_binding_changed')
        state.update(schema='nats-rollout-root-v2',installation_sha256=installation.sha256)
    preserved=None
    if preservation_ref is not None:
        import staging_cohort_preservation
        if (resource_session is None or resource_session.operation!=operation or prerequisites_capture is None
            or realtime_bootstrap_receipt is None
            or staging_cohort_preservation.load_qualified(resource_session)!=preservation_ref
            or target.get('cohort_preservation_sha256')!=canonical(preservation_ref)):
            raise Blocked('cohort_preserved_prepare_invalid')
        preserved=staging_cohort_preservation.load_rollout_continuation(resource_session)
        if preserved['contract']!=contract:raise Blocked('cohort_preserved_contract_changed')
        state['cohort_preservation']=copy.deepcopy(preservation_ref)
    if realtime_bootstrap_receipt is None and 'realtime_bootstrap_sha256' in target:
        raise Blocked('realtime_bootstrap_receipt_missing')
    if prerequisites_capture is None and 'prerequisites_sha256' in target:
        raise Blocked('staging_prerequisites_capture_missing')
    if prerequisites_capture is not None:
        if resource_session is None or realtime_bootstrap_receipt is None:raise Blocked('staging_prerequisites_session_required')
        import staging_prerequisites
        if (staging_prerequisites.load_capture(resource_session)!=prerequisites_capture
            or target.get('prerequisites_sha256')!=canonical(prerequisites_capture)):
            raise Blocked('staging_prerequisites_capture_changed')
        state['prerequisites']=copy.deepcopy(prerequisites_capture)
        save(base/'checkpoint.json',state)
    if realtime_bootstrap_receipt is not None:
        state['realtime_bootstrap']=copy.deepcopy(realtime_bootstrap_receipt)
        save(base/'checkpoint.json',state)
        if resource_session is None:raise Blocked('realtime_bootstrap_session_required')
        import realtime_bootstrap
        bootstrap_schema=realtime_bootstrap_receipt.get('schema') if isinstance(realtime_bootstrap_receipt,dict) else None
        if bootstrap_schema in ('voice-staging-realtime-bootstrap-receipt-v1','voice-staging-realtime-bootstrap-receipt-v2'):
            live_receipt=realtime_bootstrap.load_completed(resource_session)
        elif bootstrap_schema in ('voice-staging-realtime-bootstrap-existing-v1','voice-staging-realtime-bootstrap-existing-v2','voice-staging-realtime-bootstrap-existing-v3','voice-staging-realtime-bootstrap-existing-v4'):
            live_receipt=realtime_bootstrap.load_existing(resource_session)
        else:raise Blocked('realtime_bootstrap_receipt_invalid')
        if (live_receipt!=realtime_bootstrap_receipt
            or target.get('realtime_bootstrap_sha256')!=canonical(realtime_bootstrap_receipt)):
            raise Blocked('realtime_bootstrap_receipt_changed')
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
    if gis_binding is not None:
        import source_authority
        authority=gis_source_authority
        required={'schema','repository','source_sha','workflow_id','run_id','run_attempt','tree_sha',
                  'source_files','images','lock_artifact_id','lock_sha256','image_provenance','verified'}
        if (not isinstance(authority,dict) or set(authority)!=required
            or authority.get('schema')!='voice-source-authority-v1'
            or authority.get('repository')!=source_authority.REPO
            or authority.get('workflow_id')!=source_authority.WORKFLOW
            or authority.get('verified') is not True or authority.get('source_sha')!=target.get('tag')
            or authority.get('image_provenance')!='independently resolved trusted CI tag'
            or type(authority.get('run_id')) is not int or authority['run_id']<=0
            or type(authority.get('run_attempt')) is not int or authority['run_attempt']<=0
            or type(authority.get('lock_artifact_id')) is not int or authority['lock_artifact_id']<=0
            or not re.fullmatch(r'[a-f0-9]{40}',str(authority.get('tree_sha','')))
            or not re.fullmatch(r'[a-f0-9]{64}',str(authority.get('lock_sha256','')))
            or not isinstance(authority.get('source_files'),dict)
            or not isinstance(authority.get('images'),dict)
            or set(authority['images'])!=set(source_authority.IMAGE_NAMES)
            or any(not isinstance(image,str) or not re.fullmatch(re.escape(source_authority.REGISTRY+'/'+name)+r'@sha256:[a-f0-9]{64}',image)
                   for name,image in authority['images'].items())):
            raise Blocked('gis_existing_source_authority_invalid')
        selected_gis=target.get('images',{}).get('voice-gameintegration/gameintegration')
        if selected_gis is not None and selected_gis!=authority['images']['gameintegration']:
            raise Blocked('gis_existing_source_image_changed')
        state['gis_source_authority']=copy.deepcopy(authority)
    if gis_first_install is None and target.get('mode') in ('full','app-only'):
        try:active_release=private_json(guard.ROOT/'installed'/'active-release.json')
        except FileNotFoundError:active_release={}
        if ('gis_first_install_transition' in active_release and gis_transition is None and gis_binding is None):
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
            import gis_first_install as gis
            if existing!=gis.baseline_workload_names(include_hub=False):
                raise Blocked('gis_first_install_baseline_workload_count_invalid')
            # The new GIS Deployment does not exist yet and is managed by its
            # create-only receipt. The preservation stage snapshots the 22
            # existing workloads, then verifies those same UIDs/templates
            # after the GIS-only target has been applied.
            snapshot_names=existing
        if gis_binding is not None:
            if gis_first_install is not None or gis_transition is not None:
                raise Blocked('gis_existing_admission_conflict')
            import gis_existing_update as existing
            actual_gis=(existing.load_authority(kube) if preserved is None else staging_prerequisites.load_gis_authority(resource_session))
            if actual_gis!=gis_binding:
                raise Blocked('gis_existing_authority_changed')
            state['gis_expected_active']=copy.deepcopy(private_json(guard.ROOT/'installed'/'active-release.json'))
            if state['gis_expected_active'].get('gis_first_install_transition')!={
                    'operation':gis_binding['operation'],'receipt_sha256':gis_binding['anchor_sha256']}:
                raise Blocked('gis_existing_authority_changed')
            snapshot_names.add('voice-gameintegration')
        capture_args={'gis_binding':gis_binding} if gis_binding is not None else {}
        if preserved is None:
            stage=RolloutStage.capture(kube,operation,journal,
                sorted(snapshot_names)+([renderer_root.HUB] if renderer_authority is not None else []),**capture_args)
        else:
            stage=staging_cohort_preservation.capture_final_stage(resource_session)
            stage.save=journal
            if not snapshot_names<=set(stage.snapshots):raise Blocked('cohort_preserved_workload_inventory_changed')
        if target['mode']=='images-only':
            manifests,target=image_only_documents(stage,manifests,target,renderer_authority=renderer_authority)
            save(base/'apply-manifests.json',manifests)
            target['manifest_sha256']=file_sha(base/'apply-manifests.json')
        normalized_gis=[]
        def collect_gis(deployment,service,selected):
            normalized_gis.append((deployment,service,selected))
        normalize_args=({'gis_target_sink':collect_gis,'gis_binding':gis_binding}
                        if gis_binding is not None else {})
        if gis_first_install is not None:
            state['target'],state['gis_first_install']=normalize_first_install(
                kube,stage,manifests,target,state['gis_first_install'],renderer_authority=renderer_authority)
        else:
            state['target']=normalize_target(kube,stage,manifests,target,renderer_authority=renderer_authority,**normalize_args)
        if gis_binding is not None:
            import gis_existing_update as existing
            if len(normalized_gis)!=1:raise Blocked('gis_existing_normalization_scope_invalid')
            deployment,service,selected=normalized_gis[0]
            pair={'deployment_mode':'selected-paused' if selected else 'preserved-active',
                  'deployment_uid':deployment['metadata']['uid'],'service_uid':service['metadata']['uid'],
                  'deployment':existing.gis.semantic(deployment),'service':existing.gis.semantic(service)}
            state['gis_binding']=existing.bind_normalized_target(gis_binding,deployment,service,selected,
                target['mode'],state['target']['gis_existing_normalized_sha256'])
            state['gis_normalized_pair']=copy.deepcopy(pair)
            state['gis_service_account']=existing.service_account_preflight(kube,
                state['gis_binding']['target']['spec']['template']['spec'])
            save(base/'checkpoint.json',state)
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
        claim=kube.get('pvc',stage.expected['source_claim'])
        capacity=claim['status']['capacity']['storage']
        state['native_capacity']=native_preflight(stage.final_path,base,capacity)
        state['combined_space_capacity']=space_safeguards.combined_capacity(base,state['native_capacity'],state['space_preflight'])
        if state['space_preflight'] is not None:
            state['space_preflight']['source_sha']=target['tag']
            state['space_preflight']['migration_plan_sha256']=target['migration_sha256']
        try:enrollment=private_json(guard.ROOT/'bootstrap-enrollment.json')
        except FileNotFoundError:enrollment=None
        state['provenance']=(capture_inputs(kube,base,contract,stage.expected['generation'],bootstrap_enrollment=enrollment,installation=getattr(resource_session,'installation',None))
                             if preserved is None else copy.deepcopy(preserved['provenance']))
        if enrollment is not None:state['bootstrap_enrollment']=copy.deepcopy(enrollment)
        if nats_authority is not None:
            if enrollment is None:raise Blocked('nats_migration_enrollment_missing')
            state['nats_contract']=copy.deepcopy(nats_authority['plan'])
            state['nats_contract_binding']=copy.deepcopy(nats_authority['binding'])
            state['nats_target_scripts']=copy.deepcopy(nats_authority['scripts'])
            state['nats_contract_expires_at']=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=4)).isoformat()
        if preserved is None:
            shutil.copyfile(code/'kernel',base/'kernel');os.chmod(base/'kernel',0o550);os.chown(base/'kernel',0,65532)
        elif file_sha(code/'kernel')!=file_sha(base/'kernel'):
            raise Blocked('cohort_preserved_kernel_changed')
        state['kernel_sha256']=file_sha(base/'kernel')
        if gis_binding is not None:verify_gis_before_fence(kube,state,paused=preserved is not None)
    except Exception as error:
        import deployment_report
        deployment_report.capture_failure(error)
        if realtime_bootstrap_receipt is not None or prerequisites_capture is not None:
            failed=state
            failed.update(status='BLOCKED',fence_status='UNKNOWN',error=safe_error(error))
        else:
            from operator_retirement import record_readonly_failure
            failed=record_readonly_failure(base,state,error)
        save(base/'checkpoint.json',failed)
        raise
    state['phase']='FENCE';save(base/'checkpoint.json',state)
    try:
        nonnats_plan.revalidate(kube,state['nonnats_binding'])
        if preserved is not None:
            import actor_root
            import compiler
            preflight=staging_cohort_preservation.load_preflight(resource_session)
            if (preflight['nats']!=nats_authority or preflight['services']!=list(actor_services or [])
                or preflight['contract']!=contract):raise Blocked('cohort_preserved_authority_changed')
            actor_root.revalidate(kube,guard.ROOT/'installed'/'sources'/operation,code.parent,code_capture,
                                  stage,actor_services,compiler.decode_yaml,actor_authority,installation=resource_session.installation)
            staging_cohort_preservation.revalidate_paused(resource_session)
        if before_fence is not None:before_fence()
        if gis_binding is not None:
            if resource_session is None:raise Blocked('gis_existing_resource_fence_required')
            verify_gis_before_fence(kube,state,paused=preserved is not None)
        if resource_session is not None:
            if realtime_bootstrap_receipt is not None and bootstrap_schema in ('voice-staging-realtime-bootstrap-receipt-v1','voice-staging-realtime-bootstrap-receipt-v2'):
                resource_session.advance_realtime_bootstrap(state['target'],stage.old_images)
            elif prerequisites_capture is not None:
                resource_session.advance_prerequisites_target(state['target'],stage.old_images)
            else:
                resource_session.bind_target(state['target']['tag'],state['target']['images'],stage.old_images,canonical(state['target']))
            state['resource_fence']=resource_session.binding()
            save(base/'checkpoint.json',state)
        if realtime_bootstrap_receipt is not None:
            loader=(realtime_bootstrap.load_completed if bootstrap_schema in ('voice-staging-realtime-bootstrap-receipt-v1','voice-staging-realtime-bootstrap-receipt-v2')
                    else realtime_bootstrap.load_existing)
            receipt=loader(resource_session,actual=stage.old_images,rollout_target=state['target'])
            if receipt!=realtime_bootstrap_receipt or resource_session.binding()['target_sha256']!=canonical(state['target']):
                raise Blocked('realtime_bootstrap_receipt_changed')
        if prerequisites_capture is not None:
            if staging_prerequisites.load_capture(resource_session,actual=stage.old_images,rollout_target=state['target'])!=prerequisites_capture:
                raise Blocked('staging_prerequisites_capture_changed')
        if gis_binding is not None:
            verify_gis_before_fence(kube,state,paused=preserved is not None)
        stage.runtime_base=base
        if preserved is not None:
            staging_cohort_preservation.revalidate_paused(resource_session)
            state.update(context=context(stage),fence_status='VERIFIED',events=copy.deepcopy(preserved['events']),
                         cut=copy.deepcopy(preserved['cut']),custody=copy.deepcopy(preserved['custody']),
                         phase='AWAITING_OFF_NODE',status='WAITING')
            save(base/'checkpoint.json',state)
            return state
        stage.fence();state['context']=context(stage);state['fence_status']='VERIFIED'
        save(base/'checkpoint.json',state)
        runtime=DockerRuntime(base,operation)
        state['phase']='COLD_BACKUP';save(base/'checkpoint.json',state)
        live_catalog=require_original_live_census(stage,state['events'])
        state['cut']=capture_cut(runtime,stage.final_path,stage.verify_final_storage,
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
    gis.validate_normalized_binding(state['target'],state['gis_first_install'])
    baseline=state['context']['original_snapshots']
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
        'baseline_workload_count':len(baseline),
        'baseline_snapshots_sha256':gis.digest(baseline),
        'off_node_verified':True,
    }
    gis.validate_baseline_snapshots(baseline)
    state['gis_first_install']['baseline_snapshots']=copy.deepcopy(baseline)
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


def authorize(kube,base,state,off_node_archive_sha,off_node_manifest_sha,contract,*,authorization_deadline=None,expired_authority=None,resource_session=None,cohort_request=None):
    import resource_claim
    fence=state.get('resource_fence')
    scoped=isinstance(fence,dict) and fence.get('schema')=='voice-staging-resource-fence-v2'
    if scoped:
        if (type(resource_session) is not resource_claim.Session or not resource_session.admitted
            or state.get('schema')!='nats-rollout-root-v2'
            or state.get('installation_sha256')!=resource_session.installation.sha256
            or state.get('target',{}).get('installation_sha256')!=resource_session.installation.sha256
            or fence.get('target_sha256')!=canonical(state['target'])):
            raise Blocked('staging_installation_binding_changed')
        resource_session.require_binding(fence)
    elif (state.get('schema')=='nats-rollout-root-v2' or 'installation_sha256' in state
          or 'installation_sha256' in state.get('target',{})
          or fence is not None and (not isinstance(fence,dict) or fence.get('schema')!='voice-staging-resource-fence-v1')):
        raise Blocked('staging_installation_binding_changed')
    qualified_admission=None
    if state.get("target",{}).get("mode") in ("full","app-only") or state.get("migrations"):
        if resource_session is None or cohort_request is None:
            migrations.require_preserving_admission(state.get("target",{}).get("mode"),state.get("migrations"))
        import staging_cohort_preservation as cohort
        qualified_admission=cohort.issue_final_admission(resource_session,base,state,cohort_request)
    if state.get('gis_first_install') is not None:
        import gis_first_install as gis
        gis.validate_normalized_binding(state['target'],state['gis_first_install'])
    if state.get('gis_binding') is not None and expired_authority is not None:
        raise Blocked('gis_existing_recovery_unsupported')
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
    stage=reconstruct(kube,state,journal,installation=getattr(resource_session,'installation',None))
    if state.get('gis_transition') is not None:
        import gis_first_install as gis
        gis.validate_transition_binding(state['gis_transition'],kube)
    if state.get('gis_binding') is not None:
        import gis_existing_update as existing
        existing.verify_phase(state['gis_binding'],kube,'paused-old')
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
                                                        state['migration_secret_metadata'],journal,**({
                                                            'resource_session':resource_session,'qualified_admission':qualified_admission,
                                                            'authorization_base':base,'authorization_state':state} if qualified_admission is not None else {}))
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
        if 'nonnats' in state:
            nonnats_runtime.create_frontends(kube,state['nonnats'],state['nonnats_binding'],state,stage,journal,current_deadline)
        state['space_postcondition']=space_safeguards.after_migrate(kube,state.get('space_gate'))
        save(Path(base)/'checkpoint.json',state)
        if state.get('renderer_authority') is not None:
            state['phase']='RENDERER_TEMPLATE';save(Path(base)/'checkpoint.json',state)
            renderer_root.apply_paused(stage,state['renderer_authority'],installation=getattr(resource_session,'installation',None))
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
        if scoped:
            receipt.update(schema='nats-rollout-preservation-v2',installation_sha256=resource_session.installation.sha256)
        if state.get('provisional') is not None:
            import provisional
            receipt['provisional']=copy.deepcopy(provisional.validate(state['provisional']))
        if 'resource_fence' in state:receipt['resource_fence']=copy.deepcopy(state['resource_fence'])
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
        if qualified_admission is not None:cohort.complete_authorization(qualified_admission,resource_session,base,state)
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


def gis_successor_checkpoint(base,state,stage):
    """Create the first successor from root-owned evidence before release."""
    import gis_existing_update as existing
    import gis_first_install as gis
    binding=state['gis_binding'];existing.validate(binding)
    evidence=existing.successor_evidence(base,state,stage)
    state['gis_ready_proof']=existing.verify_ready(binding,stage.kube,state['gis_service_account'])
    save(Path(base)/'checkpoint.json',state)
    active=private_json(guard.ROOT/'installed'/'active-release.json')
    if active!=state['gis_expected_active']:
        raise Blocked('gis_existing_predecessor_changed')
    anchor=active['gis_first_install_transition']
    if anchor!={'operation':binding['operation'],'receipt_sha256':binding['anchor_sha256']}:
        raise Blocked('gis_existing_anchor_changed')
    original=private_json(guard.ROOT/('rollout-'+anchor['operation'])/'checkpoint.json',limit=128 << 20)
    original_receipt=original.get('gis_first_install_transition_receipt')
    if original.get('status')!='PASS' or not isinstance(original_receipt,dict):
        raise Blocked('gis_existing_anchor_invalid')
    gis.validate_transition_receipt(original_receipt,original)
    if original_receipt['receipt_sha256']!=anchor['receipt_sha256']:
        raise Blocked('gis_existing_anchor_changed')
    old={'deployment_uid':binding['deployment_uid'],'service_uid':binding['service_uid'],
         'deployment':copy.deepcopy(binding['old']),'service':copy.deepcopy(binding['service'])}
    predecessor=active.get('gis_latest_transition',anchor)
    if not isinstance(predecessor,dict) or predecessor.get('operation')==state['operation']:
        raise Blocked('gis_existing_predecessor_invalid')
    if predecessor==anchor:
        first_ready=original_receipt['ready']
        if (first_ready['deployment_uid']!=old['deployment_uid'] or first_ready['service_uid']!=old['service_uid']
            or first_ready['deployment_spec_sha256']!=gis.digest(old['deployment'])
            or first_ready['service_spec_sha256']!=gis.digest(old['service'])
            or original_receipt['namespace_uid']!=binding['namespace_uid']):
            raise Blocked('gis_existing_genesis_pair_changed')
    else:
        if (not isinstance(predecessor,dict) or set(predecessor)!={'operation','receipt_sha256'}
            or not re.fullmatch(r'[a-f0-9]{12}',str(predecessor.get('operation','')))):
            raise Blocked('gis_existing_predecessor_invalid')
        previous=private_json(guard.ROOT/('rollout-'+predecessor['operation'])/'checkpoint.json',limit=128 << 20)
        prior=previous.get('gis_existing_update',{})
        prior_receipt=prior.get('receipt',{})
        if (previous.get('status')!='PASS' or prior_receipt.get('receipt_sha256')!=predecessor['receipt_sha256']
            or prior_receipt.get('ready_pair_sha256')!=gis.digest(old)
            or prior.get('ready_pair')!=old):
            raise Blocked('gis_existing_successor_predecessor_changed')
    existing.verify_phase(binding,stage.kube,'ready-target')
    rows=gis._read_pair(stage.kube,None)
    deployment,service=rows[gis.DEPLOYMENT],rows[gis.SERVICE]
    existing.verify_phase(binding,stage.kube,'ready-target',pair=rows)
    ready={'deployment_uid':deployment['metadata']['uid'],'service_uid':service['metadata']['uid'],
           'deployment':gis.semantic(deployment),'service':gis.semantic(service)}
    existing.verify_phase(binding,stage.kube,'ready-target')
    final=gis._read_pair(stage.kube,None)
    for key,row in ((gis.DEPLOYMENT,deployment),(gis.SERVICE,service)):
        current=final[key]
        if (current['metadata']['uid']!=row['metadata']['uid']
            or current['metadata'].get('resourceVersion')!=row['metadata'].get('resourceVersion')
            or gis.semantic(current)!=gis.semantic(row)):
            raise Blocked('gis_existing_ready_observation_changed')
    authority=state['gis_source_authority'];target=state['target']
    ready_proof=state['gis_ready_proof']
    if (not isinstance(ready_proof,dict)
        or ready_proof.get('deployment_uid')!=binding['deployment_uid']
        or ready_proof.get('service_uid')!=binding['service_uid']
        or ready_proof.get('target_sha256')!=gis.digest(binding['target'])
        or not isinstance(ready_proof.get('endpoints'),list) or not ready_proof['endpoints']):
        raise Blocked('gis_existing_ready_proof_changed')
    payload={'schema':'voice-gis-existing-update-receipt-v1','operation':state['operation'],
        'anchor':copy.deepcopy(anchor),'predecessor':copy.deepcopy(predecessor),
        'namespace_uid':binding['namespace_uid'],'service_uid':binding['service_uid'],
        'deployment_uid':binding['deployment_uid'],'source_sha':authority['source_sha'],
        'source_run_id':authority['run_id'],'source_run_attempt':authority['run_attempt'],
        'source_authority_sha256':gis.digest(authority),'old_pair_sha256':gis.digest(old),
        'normalized_pair_sha256':gis.digest(state['gis_normalized_pair']),
        'ready_pair_sha256':gis.digest(ready),'target_sha256':gis.digest(target),
        'resource_fence_sha256':gis.digest(state['resource_fence']),
        'custody_sha256':gis.digest(state['custody']),'preservation_sha256':gis.digest(evidence),
        'ready_proof_sha256':gis.digest(state['gis_ready_proof'])}
    payload['receipt_sha256']=gis.digest(payload)
    state['gis_existing_update']={'receipt':payload,'old_pair':old,
        'normalized_pair':copy.deepcopy(state['gis_normalized_pair']),'ready_pair':ready,
        'source_authority':copy.deepcopy(authority),'resource_fence':copy.deepcopy(state['resource_fence']),
        'custody':copy.deepcopy(state['custody']),'preservation':evidence,
        'ready_proof':copy.deepcopy(state['gis_ready_proof'])}
    save(Path(base)/'checkpoint.json',state)
    return payload


def finish(kube,base,state,receipt,claim_rv,contract,*,finish_authority=None,publication=None,installation=None):
    if state.get('gis_binding') is not None and state.get('expired_recovery_authority') is not None:
        raise Blocked('gis_existing_recovery_unsupported')
    import provisional
    provisional.receipt_binding(state,receipt)
    if finish_authority is not None:
        return finish_only(kube,base,state,receipt,contract,finish_authority,publication=publication,installation=installation)
    if state['phase']!='PAUSED_APPLY' or state['status']!='WAITING':
        raise Blocked('rollout_finish_phase_invalid')
    if receipt!=state.get('authorization') or receipt.get('operation')!=state['operation'] or receipt.get('target')!=state['target']:
        raise Blocked('rollout_finish_authorization_changed')
    expires=dt.datetime.fromisoformat(receipt['expires_at'])
    if expires.tzinfo is None or dt.datetime.now(dt.timezone.utc)>=expires:
        raise Blocked('rollout_finish_authorization_expired')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal,installation=installation)
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
    if state.get('gis_binding') is not None:
        import gis_existing_update as existing
        existing.verify_phase(state['gis_binding'],kube,'paused-target')
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
        if state.get('gis_binding') is not None:
            import gis_existing_update as existing
            if existing.service_account_preflight(kube,state['gis_binding']['target']['spec']['template']['spec'])!=state['gis_service_account']:
                raise Blocked('gis_existing_service_account_changed')
        attach_closed_preservation(base,state,stage,resume_authority)
        if state.get('gis_binding') is not None:
            import gis_existing_update as existing
            def gis_start_guard(resumed):
                existing.verify_phase(state['gis_binding'],resumed.kube,'paused-target')
                if existing.service_account_preflight(resumed.kube,state['gis_binding']['target']['spec']['template']['spec'])!=state['gis_service_account']:
                    raise Blocked('gis_existing_service_account_changed')
            stage.gis_start_guard=gis_start_guard
            def gis_ready_proof(resumed):
                state['gis_ready_proof']=existing.verify_ready(state['gis_binding'],resumed.kube,
                    state['gis_service_account'])
                save(Path(base)/'checkpoint.json',state)
                gis_successor_checkpoint(base,state,resumed)
            stage.gis_ready_proof=gis_ready_proof
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
        save(Path(base)/'checkpoint.json',state)
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


def finish_only(kube,base,state,receipt,contract,reference,*,publication=None,installation=None):
    # Expired application permission is preserved history. A separately loaded
    # root proof permits missing resumes only; never authorize/apply/migrate.
    if state.get('gis_binding') is not None:
        raise Blocked('gis_existing_finish_only_unsupported')
    if receipt!=state.get('authorization') or receipt.get('target')!=state['target']:
        raise Blocked('rollout_finish_authorization_changed')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal,installation=installation)
    import expired_finish
    engine=expired_finish.bind(base,state,reference,stage,journal)
    revalidate_inputs(kube,contract,state['provenance'])
    try:
        state['phase']='RESTART';save(Path(base)/'checkpoint.json',state)
        state['restart']=engine.restart()
        engine.final()
        state['context']=context(stage)
        save(Path(base)/'checkpoint.json',state)
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


def rollback(kube,base,state,receipt,claim_rv,contract,*,installation=None):
    # An application which has resumed may have newer NATS/DB state. It needs
    # a fresh preservation transaction; this recovery never replays a backup.
    if state['phase'] not in ('PAUSED_APPLY','POST_APPLY_PROOF','ROLLBACK_TEMPLATES') or state['target']['mode']!='images-only':
        raise Blocked('rollout_rollback_fresh_transaction_required')
    if state.get('status') not in ('WAITING','BLOCKED') or receipt!=state.get('authorization'):
        raise Blocked('rollout_rollback_authorization_changed')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal,installation=installation)
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
        if state.get('gis_binding') is not None:
            import gis_existing_update as existing
            try:existing.verify_phase(state['gis_binding'],kube,'paused-old')
            except existing.gis.Blocked:
                try:existing.verify_phase(state['gis_binding'],kube,'paused-target')
                except existing.gis.Blocked:raise Blocked('gis_existing_rollback_pair_changed') from None
        if state['phase']!='ROLLBACK_TEMPLATES':stage.adopt_rollback_target(receipt,claim_rv)
        state['phase']='ROLLBACK_TEMPLATES';save(Path(base)/'checkpoint.json',state)
        if state.get('renderer_authority') is not None:
            if state.get('renderer_recovery_authority') is None:
                reverse=renderer_root.recovery_authority(stage,state['renderer_authority'],installation=installation)
                state['renderer_recovery_authority']=copy.deepcopy(reverse)
                state['renderer_authority']=copy.deepcopy(reverse)
                save(Path(base)/'checkpoint.json',state)
            reverse=state['renderer_recovery_authority']
            stage.renderer_transition=copy.deepcopy(reverse['descriptor'])
            stage.renderer_post_start=lambda current:renderer_root.verify_live(current,reverse,installation=installation)
            renderer_root.apply_paused(stage,reverse,installation=installation)
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
