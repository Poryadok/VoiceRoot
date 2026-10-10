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
import tarfile
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


def native_message_files(manifest):
    """Exact closed record-file bytes, not decoded/API message semantics."""
    def message_path(path):
        parts=path.split('/')
        return len(parts)>=5 and parts[0]=='jetstream' and parts[2]=='streams' and parts[4]=='msgs'
    files=sorted((copy.deepcopy(row) for row in manifest['files'] if message_path(row['path'])),key=lambda row:row['path'])
    dirs=sorted(path for path in manifest['dirs'] if message_path(path))
    if len({row['path'] for row in files})!=len(files) or len(set(dirs))!=len(dirs):
        raise Blocked('rollout_native_store_changed')
    return {'schema':'voice-native-message-files-v1','dirs':dirs,'files':files,
        'file_count':len(files),'bytes':sum(row['size'] for row in files),
        'sha256':canonical({'dirs':dirs,'files':files})}


def _native_members(runtime,archive,manifest,paths,authority):
    from closed_archive import Archive
    reader=Archive(runtime.base,archive,manifest,authority)
    rows={f['path']:f for f in manifest['files']};result={}
    with tarfile.open(archive,'r|') as bundle:
        for member in bundle:
            if member.name not in paths:continue
            expected=rows.get(member.name)
            if (expected is None or member.name in result or not member.isreg()
                or member.size!=expected['size'] or not 0<=member.size<=256<<10):
                raise Blocked('rollout_native_layout_invalid')
            source=bundle.extractfile(member)
            with source:raw=source.read((256<<10)+1)
            if len(raw)!=member.size or hashlib.sha256(raw).hexdigest()!=expected['sha256']:
                raise Blocked('rollout_native_member_changed')
            result[member.name]=raw
    if set(result)!=set(paths):raise Blocked('rollout_native_member_missing')
    reader.verify();authority();return result


def native_catalog(runtime,archive,manifest,authority):
    """Enumerate file-backed resources before ANY restored-server startup."""
    import re
    account=runtime.account_id();prefix='jetstream/'+account+'/streams/'
    files={f['path'] for f in manifest['files']};roots=set()
    for path in [*files,*manifest.get('dirs',[])]:
        if '/streams/' not in path:continue
        if not path.startswith(prefix):raise Blocked('rollout_native_account_layout_invalid')
        name=path[len(prefix):].split('/',1)[0]
        if not name or not re.fullmatch('[A-Za-z0-9_-]{1,255}',name):
            raise Blocked('rollout_native_layout_invalid')
        roots.add(name)
    if any(not {prefix+name+'/meta.inf',prefix+name+'/meta.sum'}<=files for name in roots):
        raise Blocked('rollout_native_stream_metadata_missing')
    selected={f['path'] for f in manifest['files'] if f['path'].endswith('/meta.inf')}
    raw=_native_members(runtime,archive,manifest,selected,authority);streams=set();consumers=[]
    for path,data in raw.items():
        if not path.startswith(prefix):raise Blocked('rollout_native_account_layout_invalid')
        relative=path[len(prefix):];parts=relative.split('/')
        if (len(parts) not in (2,4) or not re.fullmatch('[A-Za-z0-9_-]{1,255}',parts[0])
            or parts[-1]!='meta.inf'):raise Blocked('rollout_native_layout_invalid')
        config=json.loads(data)
        if type(config) is not dict:raise Blocked('rollout_native_config_invalid')
        if len(parts)==2:
            if config.get('name')!=parts[0] or config.get('storage')!='file':
                raise Blocked('rollout_native_memory_stream_unsupported')
            streams.add(parts[0])
        else:
            name=parts[2]
            if (parts[1]!='obs' or not re.fullmatch('[A-Za-z0-9_-]{1,255}',name)
                or config.get('mem_storage',False) is not False
                or config.get('durable_name','') not in ('',name)):
                raise Blocked('rollout_native_memory_consumer_unsupported')
            consumers.append({'stream':parts[0],'name':name})
    if roots!=streams or any(c['stream'] not in streams for c in consumers):
        raise Blocked('rollout_native_orphan_consumer')
    row={'account':account,'streams':sorted(streams),'consumers':consumers}
    closed_native_states(runtime,archive,manifest,row,authority)
    return row


def verify_catalog(catalog,row):
    if (catalog['account']!=row['account'] or set(catalog['streams'])!={s['name'] for s in row['streams']}
        or {(c['stream'],c['name']) for c in catalog['consumers']}
            !={(c['stream'],c['name']) for c in row['consumers']}):
        raise Blocked('rollout_restored_native_inventory_changed')


def closed_native_states(runtime,archive,manifest,row,authority):
    """Decode every CLOSED consumer file, including complete pending maps.

    Restored INFO cannot define this inventory: unsupported/missing native
    consumers veto instead of being silently omitted by server startup.
    """
    from closed_archive import Archive
    from closed_durable import state_shape
    from commands import capture
    expected={'jetstream/'+row['account']+'/streams/'+c['stream']+'/obs/'+c['name']+'/o.dat'
        for c in row['consumers']}
    actual={f['path'] for f in manifest['files'] if f['path'].endswith('/o.dat')}
    roots={path.rsplit('/',1)[0] for path in expected}
    files={f['path'] for f in manifest['files']}
    # Enumerate ANY native consumer path, not only supported o.dat files.
    # Server startup can otherwise omit an incomplete/unknown consumer from
    # INFO, falsely making two independently incomplete inventories equal.
    observed=set()
    for path in [*files,*manifest.get('dirs',[])]:
        if '/obs/' not in path:continue
        prefix,suffix=path.split('/obs/',1);name=suffix.split('/',1)[0]
        if not name or not prefix.startswith('jetstream/'+row['account']+'/streams/'):
            raise Blocked('rollout_native_consumer_inventory_invalid')
        observed.add(prefix+'/obs/'+name)
    required={root+'/'+name for root in roots for name in ('o.dat','meta.inf','meta.sum')}
    consumer_files={path for path in files if '/obs/' in path}
    if (len(expected)!=len(row['consumers']) or actual!=expected
        or observed!=roots or consumer_files!=required):
        raise Blocked('rollout_native_consumer_inventory_invalid')
    raw_states=_native_members(runtime,archive,manifest,expected,authority);decoded={}
    for path in sorted(expected):
        authority();raw=raw_states[path]
        result=capture([str(Path(runtime.base)/'kernel'),'--consumer-state'],body=raw,
            timeout=10,limit=2<<20,env={'PATH':'/usr/bin:/bin','HOME':'/nonexistent'})
        def pairs(items):
            value={}
            for key,item in items:
                if key in value:raise Blocked('rollout_native_consumer_decode_invalid')
                value[key]=item
            return value
        value=json.loads(result,object_pairs_hook=pairs);state_shape(value)
        decoded[path]=value
    authority();return decoded


class ColdRelease:
    """Source-owned complete cold verification and durable release boundary.

    Its inputs originate only from the root transaction's admitted checkpoint;
    public callers cannot substitute a callback or a preservation boolean.
    """
    def __init__(self,base,state,stage,authority,cut,nonce,startup_deadline):
        self.base=Path(base);self.state=state;self.stage=stage;self.authority=authority
        self.cut=copy.deepcopy(cut);self.nonce=nonce;self.startup_deadline=startup_deadline
        self.attempt=copy.deepcopy(state.get('closed_preservation_attempts',{}).get(nonce))
        if (not callable(authority) or self.base.name!='rollout-'+state['operation']
            or not guard.SHA.fullmatch(nonce or '') or type(self.attempt) is not dict
            or self.attempt.get('nonce')!=nonce
            or self.attempt.get('authorization_sha256')!=canonical(state['authorization'])
            or not isinstance(startup_deadline,dt.datetime) or startup_deadline.tzinfo is None):
            raise Blocked('cold_release_authority_invalid')
        self.guard()

    def guard(self):
        self.authority()
        if (dt.datetime.now(dt.timezone.utc)>=self.startup_deadline
            or self.state.get('closed_preservation_attempt_nonce')!=self.nonce
            or self.state.get('closed_preservation_attempts',{}).get(self.nonce)!=self.attempt
            or self.attempt['authorization_sha256']!=canonical(self.state['authorization'])
            or self.attempt.get('admitted_cut_sha256',canonical(self.cut))!=canonical(self.cut)):
            raise Blocked('cold_release_authority_expired_or_changed')

    def verify_and_journal(self,stage):
        from root_main import private_json
        if stage is not self.stage:raise Blocked('cold_release_stage_changed')
        self.guard();stage.verify_final_storage()
        state=self.state;binding=state.get('cipher_binding',{});custody=state.get('custody',{})
        restore=private_json(self.base/'decrypted-restore.json')
        keys=('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')
        ordinary=(custody.get('schema')=='voice-nats-custody-v1' and custody.get('verified') is True
            and custody.get('destination')=='github:Poryadok/VoiceRoot'
            and all(key in binding and custody.get(key)==binding[key] for key in keys))
        historical=False
        recovery=state.get('expired_recovery_authority')
        if recovery is not None:
            import expired_recovery
            current,_=expired_recovery.runtime_authority(self.base,state,recovery,now=dt.datetime.now(dt.timezone.utc))
            slot=self.base/'expired-recovery-proofs'/current['execution']['nonce']
            current_cipher=private_json(slot/'observation-cipher.json')
            current_restore=private_json(slot/'decrypted-restore.json')
            historical=(state['operation']=='3340764a7d24'
                and custody.get('schema')=='voice-preserved-cipher-custody-v1'
                and custody.get('verified') is True
                and custody.get('original_cipher_binding_sha256')==canonical(binding)
                and custody.get('cipher_sha256')==binding.get('cipher_sha256')
                and custody.get('cipher_bytes')==binding.get('cipher_bytes')
                and current_restore.get('cipher_sha256')==current_cipher['cipher_sha256']
                and current_restore.get('cipher_bytes')==current_cipher['cipher_bytes']
                and current_restore.get('decrypted_members_verified') is True
                and current_restore.get('readback',{}).get('native_archive_verified') is True)
        if (not (ordinary or historical)
            or binding.get('operation')!=state['operation']
            or restore.get('schema')!='voice-nats-decrypted-restore-v1'
            or restore.get('cipher_sha256')!=binding['cipher_sha256']
            or restore.get('cipher_bytes')!=binding['cipher_bytes']
            or restore.get('decrypted_members_verified') is not True
            or restore.get('readback',{}).get('native_archive_verified') is not True
            or restore.get('readback',{}).get('census_sha256')!=state['cut']['census_sha256']
            or restore.get('members',{}).get('rollout-before.tar')!=state['cut']['manifest']['archive_sha256']
            or restore.get('members',{}).get('rollout-before-manifest.json')!=state['cut']['manifest_sha256']):
            raise Blocked('cold_release_offnode_restore_invalid')
        runtime=__import__('docker_runtime').DockerRuntime(self.base,state['operation'])
        proof=verify_post_apply(runtime,stage.final_path,self.cut,stage.verify_final_storage)
        archive=self.base/proof['archive']
        current_manifest=copy.deepcopy(self.cut['manifest'])
        current_manifest['archive_sha256']=proof['archive_sha256']
        states=closed_native_states(runtime,archive,current_manifest,self.cut['census'],self.authority)
        baseline_path=Path(self.cut.get('archive',self.base/'rollout-before.tar'))
        if not baseline_path.is_absolute():baseline_path=self.base/baseline_path
        original=closed_native_states(runtime,baseline_path,self.cut['manifest'],self.cut['census'],self.authority)
        if states!=original:raise Blocked('cold_release_complete_durable_state_changed')
        self.guard();stage.verify_final_storage()
        intent={'kind':'cold_preservation_verified_release_intent','operation':state['operation'],
            'cut_sha256':canonical(self.cut),'native_proof_sha256':canonical(proof),
            'complete_durable_sha256':canonical(states),'decrypted_restore_sha256':canonical(restore),
            'cipher_sha256':binding['cipher_sha256'],'custody_sha256':canonical(custody),
            'selected_hub_stopped':True,'application_ack_observed':False}
        # stage.save is the root checkpoint fsync writer; this must complete
        # before the inherited normal restart can scale ANY workload.
        stage.save(intent);self.guard();self.intent=copy.deepcopy(intent);return intent

    def seal_before_resume(self,stage):
        # Retain the existing source-owned producer/release-arm interface.
        # A cold seal has no selected-HUB startup or network-policy effects.
        return self.verify_and_journal(stage)

    def arm_release(self,stage,result):
        self.guard()
        if (stage is not self.stage or result!=getattr(self,'intent',None)
            or not any(event==result for event in self.state['events'])
            or result['operation']!=self.state['operation']
            or result['cut_sha256']!=canonical(self.cut)):
            raise Blocked('cold_release_intent_uncommitted')
        stage.verify_final_storage();self.started=False
        stage.closed_preservation_release=self.release_guard
        self.guard()

    def release_guard(self,stage,action,name=None):
        from stage_runtime import HUB
        if stage is not self.stage or action not in ('scale-intent','scale-result','ready','release-intent','release-result'):
            raise Blocked('cold_release_progress_invalid')
        self.guard();stage.verify_selected_storage_identity()
        if not any(event==self.intent for event in self.state['events']):
            raise Blocked('cold_release_intent_changed')
        if action=='release-result':
            if stage.marker['data'].get('phase')!='active':raise Blocked('cold_release_marker_invalid')
        else:stage.owned_marker()
        if action=='scale-intent' and not self.started:
            if name!=HUB:raise Blocked('cold_release_hub_first_required')
            stage.verify_final_storage()
        if action=='scale-result' and name==HUB:self.started=True
        if name is not None and action in ('scale-result','ready'):
            row=stage.kube.get('deployment',name);captured=stage.snapshots[name]
            if (row['metadata']['uid']!=captured['metadata']['uid']
                or row['spec']['template']!=captured['spec']['template'] or row['spec'].get('replicas',1)!=1):
                raise Blocked('cold_release_workload_changed')
        self.guard()


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


def capture_cut(runtime, source, verify_fence,validate=None,*,live_catalog=None):
    verify_fence()
    runtime.no_operation_containers(running_only=True)
    archive=runtime.base/'rollout-before.tar'
    manifest=archive_closed_store(source,archive)
    verify_fence()
    catalog=native_catalog(runtime,archive,manifest,verify_fence)
    if live_catalog is not None:
        live_streams={s['name'] for s in live_catalog['streams']}
        live_consumers={(c['stream'],c['name']) for c in live_catalog['consumers']}
        if (live_catalog['account']!=catalog['account'] or live_streams!=set(catalog['streams'])
            or live_consumers!={(c['stream'],c['name']) for c in catalog['consumers']}):
            raise Blocked('rollout_native_live_inventory_changed')
    row=isolated_census(runtime,archive,manifest,'rollout-before',validate=validate)
    verify_catalog(catalog,row)
    states=closed_native_states(runtime,archive,manifest,row,verify_fence)
    if validate is not None:verify_fence()
    return {'manifest':manifest,'census':row,'census_sha256':canonical(row),
        'closed_durable_states':states,'native_catalog_sha256':canonical(catalog)}


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
    catalog=native_catalog(runtime,archive,manifest,verify_fence)
    recovery={};row=isolated_census(runtime,archive,manifest,'rollout-after',recovery)
    verify_catalog(catalog,row)
    compared,observations=recovered_census(cut['census'],row,recovery,cut['manifest'])
    if canonical(cut['census'])!=cut['census_sha256'] or semantic(compared)!=semantic(cut['census']):
        raise Blocked('rollout_populated_census_changed')
    verify_fence()
    return {'archive':str(archive.relative_to(runtime.base)),'archive_sha256':manifest['archive_sha256'],
            'native_files_verified':True,'census_verified':True,
            'messages':sum(s['state']['messages'] for s in row['streams']),
            'census_sha256':cut['census_sha256'],'ephemeral_recovery_observations':observations}
