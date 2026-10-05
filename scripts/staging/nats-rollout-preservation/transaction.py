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
from controller import Blocked, file_sha
from native_store import verify_archive,preflight as native_preflight
from root_main import capture_inputs,revalidate_inputs,save as root_save
from docker_runtime import DockerRuntime
from runtime_stage import RolloutStage
from preserve import capture_cut, verify_post_apply, canonical
from normalize import normalize_target,image_only_documents
from errors import safe_error

def save(path,row):root_save(path,row,limit=128<<20)


def rollback_package(previous):
    if previous.get('status')!='PASS' or previous['input_target']['mode']!='images-only':
        raise Blocked('rollout_rollback_database_compatibility_required')
    target=copy.deepcopy(previous['input_target']);rows=[];images={}
    for name in target['template_hashes']:
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
    return stage


def checkpoint_journal(base,state,stage_getter):
    def journal(event):
        state['events'].append(copy.deepcopy(event))
        stage=stage_getter()
        if stage is not None:
            state['context']=context(stage)
        save(Path(base)/'checkpoint.json',state)
    return journal


def prepare(kube,base,code,target,contract,operation,code_capture,migration_plan,non_nats,before_fence=None):
    # base creation/root lock/captured code custody belongs to the root launcher.
    base=Path(base);code=Path(code)
    state={'schema':'nats-rollout-root-v1','operation':operation,'phase':'READ_ONLY_CAPTURE',
           'status':'RUNNING','fence_status':'UNKNOWN','events':[],'target':copy.deepcopy(target),
           'code_capture':copy.deepcopy(code_capture),'contract':copy.deepcopy(contract),
           'input_target':copy.deepcopy(target)}
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    save(base/'checkpoint.json',state)
    manifests=json.loads((base/'apply-manifests.json').read_bytes())
    if guard.frontend_image_only(target) and (migration_plan or non_nats['actions']):
        raise Blocked('rollout_frontend_auxiliary_action_forbidden')
    stage=RolloutStage.capture(kube,operation,journal,
        [r['metadata']['name'] for r in manifests if r.get('kind')=='Deployment'])
    if target['mode']=='images-only':
        manifests,target=image_only_documents(stage,manifests,target)
        save(base/'apply-manifests.json',manifests)
        target['manifest_sha256']=file_sha(base/'apply-manifests.json')
    state['target']=normalize_target(kube,stage,manifests,target)
    if canonical(migration_plan)!=target['migration_sha256']:raise Blocked('rollout_target_migration_digest_invalid')
    state['migrations']=copy.deepcopy(migration_plan)
    state['migration_secret_metadata']=migrations.preflight(kube,migration_plan,target['mode'])
    if canonical(non_nats)!=target['nonnats_sha256']:raise Blocked('rollout_target_nonnats_digest_invalid')
    state['nonnats']=copy.deepcopy(non_nats)
    state['nonnats_binding']=nonnats_runtime.preflight(kube,non_nats,target['mode'])
    import source_plan
    source_plan.verify_db_init(kube,non_nats)
    state['context']=context(stage)
    claim=kube.get('pvc',stage.expected['source_claim'])
    capacity=claim['status']['capacity']['storage']
    state['native_capacity']=native_preflight(stage.final_path,base,capacity)
    state['provenance']=capture_inputs(kube,base,contract,stage.expected['generation'])
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
    if state['phase']!='AWAITING_OFF_NODE' or state['status']!='WAITING':
        raise Blocked('rollout_offnode_phase_invalid')
    cut=state['cut'];manifest=cut['manifest']
    if (off_node_archive_sha!=manifest['archive_sha256'] or
        off_node_manifest_sha!=cut['manifest_sha256'] or
        file_sha(Path(base)/'rollout-before-manifest.json')!=cut['manifest_sha256'] or
        not verify_archive(Path(base)/'rollout-before.tar',manifest)):
        raise Blocked('rollout_offnode_copy_mismatch')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal)
    revalidate_inputs(kube,contract,state['provenance'])
    stage.verify_final_storage()
    if 'nonnats' in state:nonnats_runtime.verify(kube,state['nonnats'],state['nonnats_binding'],stage)
    if canonical(state['migrations'])!=state['target']['migration_sha256']:raise Blocked('rollout_target_migration_digest_invalid')
    state['phase']='DATABASE_MIGRATIONS';save(Path(base)/'checkpoint.json',state)
    try:
        state['migration_jobs']=migrations.execute(kube,stage,state['migrations'],state['target']['mode'],
                                                    state['migration_secret_metadata'],journal)
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
        state['preservation']=verify_post_apply(runtime,stage.final_path,state['cut'],stage.verify_final_storage)
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
    if state['phase'] not in ('PAUSED_APPLY','POST_APPLY_PROOF') or state['target']['mode']!='images-only':
        raise Blocked('rollout_rollback_fresh_transaction_required')
    if state.get('status') not in ('WAITING','BLOCKED') or receipt!=state.get('authorization'):
        raise Blocked('rollout_rollback_authorization_changed')
    stage=None
    journal=checkpoint_journal(base,state,lambda:stage)
    stage=reconstruct(kube,state,journal)
    revalidate_inputs(kube,contract,state['provenance'])
    try:
        stage.adopt_rollback_target(receipt,claim_rv)
        state['phase']='ROLLBACK_TEMPLATES';save(Path(base)/'checkpoint.json',state)
        stage.restore_original_templates()
        state['phase']='ROLLBACK_PROOF';save(Path(base)/'checkpoint.json',state)
        state['preservation']=verify_post_apply(DockerRuntime(base,state['operation']),stage.final_path,state['cut'],stage.verify_final_storage)
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
