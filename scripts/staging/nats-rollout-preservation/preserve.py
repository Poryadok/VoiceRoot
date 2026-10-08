"""Nonreset cold-cut proof: the selected staging store is never broker-mounted.

The caller owns the UID-bound physical fence and captured runtime. All INFO
queries run against restored private copies; no bootstrap, publish or ACK runs
against the selected staging claim. The release controller must retain the
fence until post-apply proof and its target-template checks have succeeded.
"""
import hashlib
import copy
import datetime as dt
import json
from pathlib import Path
import tempfile
import guard
from controller import Blocked
from native_store import archive_closed_store,verify_archive
from scenario import ready
from rollout_census import census, semantic
from docker_runtime import NATS_IMAGE


def canonical(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


def native_inventory(manifest):
    # Deterministic archive timestamps are absent; compare every native byte,
    # file path and directory, including records not delivered by a consumer.
    return {key:manifest[key] for key in ('mechanism','file_count','bytes','dirs','files')}


def isolated_census(runtime, archive, manifest, label, recovery=None,validate=None):
    if validate is not None:return closed_copy_census(runtime,archive,manifest,label,validate)
    if not verify_archive(archive,manifest):raise Blocked('rollout_archive_changed')
    parent=Path(tempfile.mkdtemp(prefix=label+'-',dir=runtime.base))
    target=parent/'store'
    runtime.restore(archive,target,manifest)
    started=dt.datetime.now(dt.timezone.utc)
    broker=runtime.start_broker(label,target)
    try:
        owned=runtime.inspect(broker)
        if owned['Config']['Image']!=NATS_IMAGE or owned['HostConfig']['NetworkMode']!='none':raise Blocked('rollout_recovery_broker_unbound')
        ready(runtime,broker)
        if validate is None:return census(runtime,broker,runtime.account_id())
        # One actual monitoring snapshot feeds both census and root plan proof.
        tree=runtime.monitor_jsz(broker)
        class Snapshot:
            def monitor_jsz(self,name):
                if name!=broker:raise Blocked('rollout_recovery_snapshot_unbound')
                return tree
        row=census(Snapshot(),broker,runtime.account_id())
        validate(runtime,broker,tree,row,manifest)
        return row
    finally:
        runtime.stop(broker)
        cid=runtime.owned[broker]['id']
        if recovery is not None:recovery.update({'server_image':NATS_IMAGE,'container_id':cid,
            'started_at':started.isoformat(),'finished_at':dt.datetime.now(dt.timezone.utc).isoformat()})
        runtime.inspect(broker)
        runtime.run(['rm','-f',cid])
        if runtime.run(['ps','-a','--filter','id='+cid,'--format','{{.ID}}']):raise Blocked('rollout_recovery_copy_cleanup_failed')
        del runtime.owned[broker]
        if validate is not None:
            import shutil
            shutil.rmtree(parent)

def closed_copy_census(runtime,archive,manifest,label,validate):
    """Rejecting proof/startup still removes only the exact owned copy."""
    import shutil
    if not verify_archive(archive,manifest):raise Blocked('rollout_archive_changed')
    parent=Path(tempfile.mkdtemp(prefix=label+'-',dir=runtime.base))
    broker='voice-known-'+runtime.operation+'-'+label
    try:
        runtime.restore(archive,parent/'store',manifest)
        actual=runtime.start_broker(label,parent/'store')
        if actual!=broker:raise Blocked('rollout_recovery_broker_unbound')
        owned=runtime.inspect(broker)
        if owned['Config']['Image']!=NATS_IMAGE or owned['HostConfig']['NetworkMode']!='none':
            raise Blocked('rollout_recovery_broker_unbound')
        ready(runtime,broker)
        tree=runtime.monitor_jsz(broker)
        class Snapshot:
            def monitor_jsz(self,name):
                if name!=broker:raise Blocked('rollout_recovery_snapshot_unbound')
                return tree
        row=census(Snapshot(),broker,runtime.account_id())
        validate(runtime,broker,tree,row,manifest)
        return row
    finally:
        # A create/start error may already have entered the owned ledger.
        # Inspect immutable identity before removal; never remove by name.
        if broker in runtime.owned:
            cid=runtime.inspect(broker)['Id']
            runtime.run(['rm','-f',cid])
            if runtime.run(['ps','-a','--filter','id='+cid,'--format','{{.ID}}']):
                raise Blocked('rollout_recovery_copy_cleanup_failed')
            del runtime.owned[broker]
        else:
            runtime.no_operation_containers(running_only=False)
        shutil.rmtree(parent)

def recovered_census(before,after,recovery,manifest):
    """Normalize only a proved pinned ephemeral recovery timestamp rewrite.

    This never changes semantic() or waives missing/config/ACK/state differences.
    Native consumer state bytes must already match this original manifest.
    """
    adjusted=copy.deepcopy(after);observations=[]
    current={(row['stream'],row['name']):row for row in adjusted['consumers']}
    for old in before['consumers']:
        key=old['stream'],old['name'];new=current.get(key)
        if new is None or new.get('created')==old.get('created'):continue
        if old.get('durable') or new.get('durable'):raise Blocked('rollout_durable_creation_changed')
        if recovery.get('server_image')!=NATS_IMAGE or not recovery.get('container_id'):raise Blocked('rollout_ephemeral_recovery_unbound')
        start=dt.datetime.fromisoformat(recovery['started_at']);end=dt.datetime.fromisoformat(recovery['finished_at'])
        observed_created=new['created'];created=dt.datetime.fromisoformat(observed_created.replace('Z','+00:00'))
        original_created=dt.datetime.fromisoformat(old['created'].replace('Z','+00:00'))
        if (start.tzinfo is None or end.tzinfo is None or created.tzinfo is None or not start<=created<=end
            or original_created.tzinfo is None or created<original_created
            or not 0<old.get('inactive_threshold',0) or (end-start).total_seconds()*1e9>=old['inactive_threshold']):
            raise Blocked('rollout_ephemeral_recovery_interval_invalid')
        suffix='/streams/'+old['stream']+'/obs/'+old['name']+'/o.dat'
        if sum(row['path'].endswith(suffix) for row in manifest['files'])!=1:raise Blocked('rollout_ephemeral_state_file_missing')
        new['created']=old['created']
        if semantic({'consumers':[new]})!=semantic({'consumers':[old]}):raise Blocked('rollout_ephemeral_state_changed')
        observations.append({'stream':old['stream'],'name':old['name'],'old_created':old['created'],
            'recovered_created':observed_created,
            'recovery':copy.deepcopy(recovery),'native_state_file_verified':True})
    streams={row['name']:row for row in adjusted['streams']}
    for old in before['streams']:
        new=streams.get(old['name'])
        if new is None or new.get('created')==old.get('created'):continue
        evidence=recovery.get('stream_updates',{}).get(old['name'])
        if old['name'] not in ('chat_events','social_events') or not evidence:raise Blocked('rollout_stream_creation_changed')
        attempt=evidence['attempt'];start=dt.datetime.fromisoformat(attempt['started_at']);end=dt.datetime.fromisoformat(attempt['finished_at'])
        created=dt.datetime.fromisoformat(new['created'].replace('Z','+00:00'))
        if (attempt.get('server_image')!=NATS_IMAGE or not attempt.get('container_id') or start.tzinfo is None or end.tzinfo is None
            or created.tzinfo is None or not start<=created<=end or evidence['native_created']!=new['created']
            or evidence.get('target_config_sha256')!=new['config_sha256']):
            raise Blocked('rollout_stream_update_recovery_unbound')
        observations.append({'stream':old['name'],'old_created':old['created'],'recovered_created':new['created'],
            'native_created':evidence['native_created'],'migration_attempt':copy.deepcopy(attempt),'fixed_update_verified':True})
        new['created']=old['created']
    return adjusted,observations


def capture_cut(runtime, source, verify_fence,validate=None):
    verify_fence()
    runtime.no_operation_containers(running_only=True)
    archive=runtime.base/'rollout-before.tar'
    manifest=archive_closed_store(source,archive)
    verify_fence()
    row=isolated_census(runtime,archive,manifest,'rollout-before',validate=validate)
    if validate is not None:verify_fence()
    return {'manifest':manifest,'census':row,'census_sha256':canonical(row)}


def verify_post_apply(runtime, source, cut, verify_fence):
    # No cold cut may be replaced by an empty/new claim or silently reseeded.
    verify_fence()
    runtime.no_operation_containers(running_only=True)
    attempt=Path(tempfile.mkdtemp(prefix='post-apply-',dir=runtime.base))
    archive=attempt/'rollout-after.tar'
    manifest=archive_closed_store(source,archive)
    if native_inventory(manifest)!=native_inventory(cut['manifest']):
        raise Blocked('rollout_native_store_changed')
    verify_fence()
    recovery={};row=isolated_census(runtime,archive,manifest,'rollout-after',recovery)
    compared,observations=recovered_census(cut['census'],row,recovery,cut['manifest'])
    if canonical(cut['census'])!=cut['census_sha256'] or semantic(compared)!=semantic(cut['census']):
        raise Blocked('rollout_populated_census_changed')
    verify_fence()
    return {'archive':str(archive.relative_to(runtime.base)),'archive_sha256':manifest['archive_sha256'],
            'native_files_verified':True,'census_verified':True,
            'messages':sum(s['state']['messages'] for s in row['streams']),
            'census_sha256':cut['census_sha256'],'ephemeral_recovery_observations':observations}
