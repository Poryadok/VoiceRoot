"""Closed-store migration phase in the normal root-owned transaction.

The original cut remains the independently verified recovery artifact. A new
post-config ledger becomes the unchanged-data baseline for paused app apply.
"""
import copy,tempfile,json,os,stat,tarfile,hashlib
import datetime as dt
from pathlib import Path
from controller import Blocked,file_sha
from docker_runtime import DockerRuntime,NATS_IMAGE
from native_store import archive_closed_store
from nats_contract_actor import Actor
from nats_contract_plan import digest,execute,verify_census
from preserve import verify_post_apply,isolated_census,canonical,recovered_census
from scenario import ready

def stream_updates(state,archive,manifest):
    """Owned UPDATE attempt evidence; full current INFO must still match target."""
    opened={event['container_id']:event for event in state['events'] if event.get('kind')=='nats_contract_broker_opened'}
    closed={event['container_id']:event for event in state['events'] if event.get('kind')=='nats_contract_broker_closed'}
    updates={}
    for event in state['events']:
        if event.get('kind')!='nats_contract_mutation_issued':continue
        obj=event.get('object');attempt=event.get('migration_attempt')
        action=next((a for a in state['nats_contract']['actions'] if a['api']=='$JS.API.STREAM.UPDATE.'+str(obj)),None)
        if (obj not in ('chat_events','social_events') or action is None or event.get('api')!=action['api']
            or event.get('target_sha256')!=digest(action['after'])):continue
        if attempt not in opened or attempt not in closed:raise Blocked('nats_stream_update_attempt_incomplete')
        start=opened[attempt];end=closed[attempt]
        if start['server_image']!=NATS_IMAGE or end['started_at']!=start['started_at']:raise Blocked('nats_stream_update_attempt_changed')
        updates[obj]={'attempt':{**start,'finished_at':end['finished_at']},'target_config_sha256':digest(action['after'])}
    if not updates:return {}
    wanted={}
    for obj in updates:
        matches=[row for row in manifest['files'] if row['path'].endswith('/streams/'+obj+'/meta.inf')]
        if len(matches)!=1 or matches[0]['size']>1<<20:raise Blocked('nats_stream_update_native_metadata_invalid')
        wanted[matches[0]['path']]=(obj,matches[0])
    fd=os.open(archive,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        info=os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_nlink!=1 or info.st_mode&0o077:raise Blocked('nats_stream_update_archive_untrusted')
        with os.fdopen(fd,'rb',closefd=False) as stream,tarfile.open(fileobj=stream,mode='r|') as source:
            for member in source:
                if member.name not in wanted:continue
                obj,row=wanted.pop(member.name)
                if not member.isreg() or member.size!=row['size']:raise Blocked('nats_stream_update_native_metadata_invalid')
                raw=source.extractfile(member).read((1<<20)+1)
                if hashlib.sha256(raw).hexdigest()!=row['sha256']:raise Blocked('nats_stream_update_native_metadata_changed')
                updates[obj]['native_created']=json.loads(raw)['Created']
        if wanted:raise Blocked('nats_stream_update_native_metadata_missing')
    finally:os.close(fd)
    return updates

def apply_contract(base,state,stage,journal):
    plan=state['nats_contract'];binding=state.get('nats_contract_binding',{})
    enrollment=state.get('bootstrap_enrollment',{})
    if (state.get('custody',{}).get('verified') is not True
        or binding.get('plan_sha256')!=digest(plan) or binding.get('server_image')!=NATS_IMAGE
        or binding.get('enrollment_sha256')!=digest(enrollment)
        or binding.get('sources')!=plan.get('sources')):
        raise Blocked('nats_migration_authority_unbound')
    runtime=DockerRuntime(Path(base),state['operation'])
    def records(row):return {f['path']:f for f in row['files'] if '/msgs/' in f['path']}
    def old_consumer_states_unchanged(manifest):
        old={f['path']:f for f in state['cut']['manifest']['files'] if '/obs/' in f['path'] and Path(f['path']).name=='o.dat'}
        current={f['path']:f for f in manifest['files']}
        if any(current.get(path)!=row for path,row in old.items()):raise Blocked('nats_migration_consumer_state_bytes_changed')
        return len(old)
    owned_proof={'kind':'nats_contract_old_cut_verified','plan_sha256':digest(plan),
        'old_manifest_sha256':state['cut']['manifest_sha256'],'old_census_sha256':state['cut']['census_sha256']}
    if any(all(event.get(k)==v for k,v in owned_proof.items()) for event in state['events']):
        # After an interrupted owned UPDATE, original config bytes differ.
        # Validate every original record byte and only the already-approved
        # subset of exact config changes before continuing the fixed plan.
        stage.verify_final_storage();runtime.no_operation_containers(running_only=True)
        attempt=Path(tempfile.mkdtemp(prefix='contract-retry-',dir=base));archive=attempt/'current.tar'
        manifest=archive_closed_store(stage.final_path,archive)
        if records(manifest)!=records(state['cut']['manifest']):raise Blocked('nats_migration_record_bytes_changed')
        old_consumer_states_unchanged(manifest)
        recovery={'stream_updates':stream_updates(state,archive,manifest)};current=isolated_census(runtime,archive,manifest,'contract-retry',recovery)
        current,observations=recovered_census(state['cut']['census'],current,recovery,state['cut']['manifest'])
        partial=copy.deepcopy(plan);partial['actions']=[]
        streams={s['name']:s for s in current['streams']};consumers={(c['stream'],c['name']):c for c in current['consumers']}
        for action in plan['actions']:
            if '.STREAM.UPDATE.' in action['api']:
                value=streams[action['object']]['config_sha256']
                if value==digest(action['after']):partial['actions'].append(action)
                elif value!=digest(action['before']):raise Blocked('nats_migration_retry_config_drift')
            elif ('social_events','rt_realtime1_friend_removed') in consumers:partial['actions'].append(action)
        verify_census(partial,state['cut']['census'],current)
    else:
        # Exact unchanged native bytes + full old census before any config write.
        old_proof=verify_post_apply(runtime,stage.final_path,state['cut'],stage.verify_final_storage)
        journal(owned_proof|{'proof':old_proof})
    if not plan['actions']:
        return {'verified':True,'applied':[],'proof':verify_census(plan,state['cut']['census'],state['cut']['census']),
            'old_record_files_verified':True,'cut':copy.deepcopy(state['cut'])}
    runtime.no_operation_containers(running_only=True);stage.verify_final_storage()
    runtime.allow_bound_store(stage.final_path,stage.final_claim,stage.final_pv,stage.verify_final_storage,
                             descriptor=stage.selected_store_descriptor())
    broker=None;attempt=None;closed=False
    try:
        started=dt.datetime.now(dt.timezone.utc).isoformat()
        broker=runtime.start_broker('contract-migration',stage.final_path);ready(runtime,broker)
        attempt={'container_id':runtime.owned[broker]['id'],'server_image':NATS_IMAGE,'started_at':started}
        journal({'kind':'nats_contract_broker_opened',**attempt})
        actor=Actor(runtime,broker,runtime.owned[broker]['id'],stage.final_path,stage.verify_final_storage)
        applied=execute(plan,actor,lambda event:journal({**event,'migration_attempt':attempt['container_id']}))
        runtime.stop(broker);stage.verify_final_storage()
        journal({'kind':'nats_contract_broker_closed',**attempt,'finished_at':dt.datetime.now(dt.timezone.utc).isoformat()});closed=True
        attempt=Path(tempfile.mkdtemp(prefix='contract-post-',dir=base));archive=attempt/'nats-post-migration.tar'
        manifest=archive_closed_store(stage.final_path,archive)
        if records(manifest)!=records(state['cut']['manifest']):raise Blocked('nats_migration_record_bytes_changed')
        old_state_count=old_consumer_states_unchanged(manifest)
        recovery={'stream_updates':stream_updates(state,archive,manifest)};row=isolated_census(runtime,archive,manifest,'contract-after',recovery)
        compared,observations=recovered_census(state['cut']['census'],row,recovery,state['cut']['manifest'])
        proof=verify_census(plan,state['cut']['census'],compared)
        proof['ephemeral_recovery_observations']=observations
        stage.verify_final_storage()
        return {'verified':True,'applied':applied,'proof':proof,'old_record_files_verified':True,'old_consumer_state_files_verified':old_state_count,
            'cut':{'manifest':manifest,'census':row,'census_sha256':canonical(row),'archive':str(archive),
                'manifest_sha256':digest(manifest),'archive_sha256':file_sha(archive)}}
    finally:
        if broker is not None and attempt is not None and not closed:
            runtime.stop(broker)
            journal({'kind':'nats_contract_broker_closed',**attempt,'finished_at':dt.datetime.now(dt.timezone.utc).isoformat()})
        for name in reversed(list(runtime.owned)):
            runtime.inspect(name);runtime.run(['rm','-f',runtime.owned[name]['id']])
            if runtime.run(['ps','-a','--filter','id='+runtime.owned[name]['id'],'--format','{{.ID}}']):raise Blocked('nats_migration_cleanup_failed')
