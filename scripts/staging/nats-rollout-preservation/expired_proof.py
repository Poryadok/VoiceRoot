"""Fresh proof of restored private copies; never mounts the selected store."""
import base64
import hashlib
import json
import re
from datetime import datetime, timezone
from pathlib import Path
import shutil
import tempfile

MAX_SEQUENCES=262144
MAX_RECORD_BYTES=48<<30

def fail():raise ValueError('expired_recovery_full_record_proof_invalid')


def cri_time(value):
    """Exact RFC3339Nano copied-container timestamp, without live-HUB runtime."""
    if type(value) is not str:fail()
    match=re.fullmatch(r'(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?Z',value)
    if not match:fail()
    stamp=datetime.strptime(match[1],'%Y-%m-%dT%H:%M:%S').replace(tzinfo=timezone.utc)
    seconds=(stamp-datetime(1970,1,1,tzinfo=timezone.utc)).days*86400+stamp.hour*3600+stamp.minute*60+stamp.second
    if seconds<=0:fail()
    return seconds*1000000000+int((match[2] or '').ljust(9,'0'))

def record_digest(actor,row):
    """Read every selected sequence, including records not delivered or ACKed.

    Deletion holes are queried and counted; a record is never assumed present
    from consumer floors. The caller has independently captured complete JSZ.
    """
    digest=hashlib.sha256();total=count=scanned=0;names=set()
    for stream in row['streams']:
        name=stream['name'];state=stream['state']
        if not isinstance(name,str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,255}',name) or name in names:fail()
        names.add(name)
        first,last,messages=(state[key] for key in ('first_seq','last_seq','messages'))
        if any(type(value)!=int or value<0 for value in (first,last,messages)):fail()
        if not messages:continue
        if first<1 or last<first:fail()
        scanned+=last-first+1
        if scanned>MAX_SEQUENCES:fail()
        found=0
        for seq in range(first,last+1):
            record=actor.get_record(name,seq)
            if record is None:continue
            if (not isinstance(record,dict) or set(record)-{'subject','seq','data','hdrs','time'}
                or record.get('seq')!=seq or not isinstance(record.get('subject'),str)
                or not record['subject'] or not isinstance(record.get('data'),str)):fail()
            try:
                stamp=datetime.fromisoformat(record['time'].replace('Z','+00:00'))
                if stamp.tzinfo is None:fail()
                payload=base64.b64decode(record['data'],validate=True)
                headers=base64.b64decode(record.get('hdrs',''),validate=True)
            except (ValueError,TypeError,KeyError):fail()
            total+=len(payload)+len(headers)
            if total>MAX_RECORD_BYTES:fail()
            encoded=json.dumps({'stream':name,'record':record},sort_keys=True,separators=(',',':'),allow_nan=False).encode()
            digest.update(len(encoded).to_bytes(8,'big'));digest.update(encoded)
            found+=1;count+=1
        if found!=messages:fail()
    return {'schema':'voice-full-record-proof-v1','messages':count,'decoded_bytes':total,
            'sequences_observed':scanned,'sha256':digest.hexdigest()}

def observe_copy(runtime,archive,manifest,label,verify_fence,validate=None,*,record_sink=None,closed_archive=None):
    """One owned copy feeds full census/config and all-record proof.

    Failure removes only newly owned container IDs and the private restored
    directory. Original archive and selected native tree are never written.
    """
    from native_store import verify_archive,archive_closed_store
    from scenario import ready
    from rollout_census import census
    from nats_contract_actor import Actor
    from docker_runtime import NATS_IMAGE
    if record_sink is not None and not callable(record_sink):fail()
    if closed_archive is not None:
        closed_archive=Path(closed_archive)
        if (closed_archive.name!='current-copy-closed.tar' or closed_archive.exists()
            or closed_archive.resolve()!=closed_archive or not closed_archive.is_relative_to(runtime.base)
            or closed_archive==Path(archive)):fail()
    verify_fence()
    if not verify_archive(archive,manifest):fail()
    baseline=set(runtime.owned)
    parent=Path(tempfile.mkdtemp(prefix=label+'-',dir=runtime.base))
    started=datetime.now(timezone.utc)
    broker=None
    try:
        runtime.restore(archive,parent/'store',manifest)
        broker=runtime.start_broker(label,parent/'store')
        owned=runtime.inspect(broker)
        if owned['Config']['Image']!=NATS_IMAGE or owned['HostConfig']['NetworkMode']!='none':fail()
        ready(runtime,broker)
        tree=runtime.monitor_jsz(broker)
        class Snapshot:
            def monitor_jsz(self,name):
                if name!=broker:fail()
                return tree
        row=census(Snapshot(),broker,runtime.account_id())
        actor=Actor(runtime,broker,owned['Id'],parent/'store',verify_fence)
        ledger=None
        if record_sink is None:
            records=record_digest(actor,row)
        else:
            streams={item['name']:{'records':{},'missing_sequences':[]} for item in row['streams']}
            class Capture:
                def get_record(self,stream,sequence):
                    verify_fence();value=actor.get_record(stream,sequence);verify_fence()
                    if value is None:streams[stream]['missing_sequences'].append(sequence)
                    else:streams[stream]['records'][str(sequence)]=record_sink(stream,value)
                    return value
            # One authenticated GET per sequence supplies both the unchanged
            # digest and the exact private subject/header/time/payload index.
            records=record_digest(Capture(),row)
            ledger={'schema':'voice-private-full-record-ledger-v1','server_id':tree.get('server_id'),
                'storage':'ROOT_IMMUTABLE_RECORD_FILES','streams':streams,'record_proof':records}
        if validate is not None:validate(runtime,broker,tree,row,manifest,records)
        verify_fence()
        result={'census':row,'records':records,'recovery':{'server_image':NATS_IMAGE,
            'container_id':owned['Id'],'started_at':started.isoformat()}}
        if ledger is not None:
            result.update(private_record_ledger=ledger,private_monitor=tree)
        if closed_archive is not None:
            # rm -f is cleanup, never an orderly durable flush certificate.
            # DockerRuntime.inspect independently binds image/isolation/mounts
            # and the owned container ID on each read.
            opening=owned['State']['StartedAt']
            if owned['State'].get('Running') is not True:fail()
            verify_fence();runtime.stop(broker);verify_fence()
            closed=runtime.inspect(broker);state=closed['State']
            stopped=datetime.now(timezone.utc)
            if (closed['Id']!=owned['Id'] or closed['Config']['Image']!=NATS_IMAGE
                or closed['HostConfig']['NetworkMode']!='none' or state.get('Running') is not False
                or type(state.get('ExitCode')) is not int or state['ExitCode']!=0
                or state.get('OOMKilled') is not False or state.get('Dead') is not False
                or state.get('Error')!='' or state.get('StartedAt')!=opening
                or not cri_time(started.isoformat().replace('+00:00','Z'))<=cri_time(opening)
                    <=cri_time(state.get('FinishedAt'))<=cri_time(stopped.isoformat().replace('+00:00','Z'))):fail()
            verify_fence()
            closed_manifest=archive_closed_store(parent/'store',closed_archive)
            # Container replacement/restart across the archive cannot be
            # hidden by a successful archive hash.
            if runtime.inspect(broker)!=closed or not verify_archive(closed_archive,closed_manifest):fail()
            verify_fence()
            result.update(closed_manifest=closed_manifest,orderly_exit={
                'schema':'voice-private-copy-orderly-exit-v1','container_id':owned['Id'],
                'server_image':NATS_IMAGE,'started_at':opening,
                'finished_at':state['FinishedAt'],'exit_code':0})
    finally:
        # Capture failure can leave created but unstarted clients. ID-bound
        # inspect is mandatory before removal and removal is verified.
        for name in reversed(list(runtime.owned)):
            if name in baseline:continue
            cid=runtime.inspect(name)['Id']
            runtime.run(['rm','-f',cid])
            if runtime.run(['ps','-a','--filter','id='+cid,'--format','{{.ID}}']):fail()
            del runtime.owned[name]
        shutil.rmtree(parent)
        verify_fence()
    result['recovery']['finished_at']=datetime.now(timezone.utc).isoformat()
    return result

def failed_startup_interval(state,awaiting):
    """Only the exact preserved pre-intent failure has this startup branch."""
    from docker_runtime import NATS_IMAGE
    from nats_contract_plan import digest
    prefix=awaiting['events'];events=state['events']
    if events[:len(prefix)]!=prefix:fail()
    delta=events[len(prefix):]
    native=[event for event in delta if str(event.get('kind','')).startswith('nats_contract_')]
    if [event['kind'] for event in native]!=['nats_contract_old_cut_verified',
        'nats_contract_broker_opened','nats_contract_broker_closed']:fail()
    proof,opened,closed=native
    if (proof.get('plan_sha256')!=digest(state['nats_contract'])
        or proof.get('old_manifest_sha256')!=state['cut']['manifest_sha256']
        or proof.get('old_census_sha256')!=state['cut']['census_sha256']
        or proof.get('proof',{}).get('native_files_verified') is not True
        or proof.get('proof',{}).get('census_verified') is not True):fail()
    if (set(opened)!={'kind','container_id','server_image','started_at'}
        or set(closed)!={'kind','container_id','server_image','started_at','finished_at'}
        or not re.fullmatch(r'[a-f0-9]{64}',opened.get('container_id',''))
        or opened['server_image']!=NATS_IMAGE
        or any(closed[key]!=opened[key] for key in ('container_id','server_image','started_at'))):fail()
    start=datetime.fromisoformat(opened['started_at'].replace('Z','+00:00'))
    end=datetime.fromisoformat(closed['finished_at'].replace('Z','+00:00'))
    if start.tzinfo is None or end.tzinfo is None or end<start:fail()
    return {'server_image':NATS_IMAGE,'started_at':opened['started_at'],'closed_at':closed['finished_at']}

def current_inventory(runtime,stage):
    """Inspect actual CLOSED selected bytes immediately before entry/start."""
    from native_store import archive_closed_store
    from preserve import native_inventory
    from preserved_upload import digest
    stage.verify_final_storage();runtime.no_operation_containers(running_only=True)
    attempt=Path(tempfile.mkdtemp(prefix='expired-freshness-',dir=runtime.base))
    try:
        manifest=archive_closed_store(stage.final_path,attempt/'current.tar')
        stage.verify_final_storage();runtime.no_operation_containers(running_only=True)
        return digest(native_inventory(manifest))
    finally:shutil.rmtree(attempt)

def produce(runtime,stage,state,awaiting,slot,checksum,validate):
    """Root-owned fresh CURRENT observation, distinct from the original cut.

    Called under the existing global/root operation lock after source admission.
    validate executes the source-bound plan/actor predicates against this one
    copied monitoring tree; accepting hashes alone never bypasses them.
    """
    from native_store import archive_closed_store,verify_archive
    from preserve import recovered_census,native_inventory,canonical
    from rollout_census import semantic
    from expired_recovery import classify_startup
    from root_main import save
    from preserved_upload import digest
    import tarfile
    if not callable(validate):fail()
    slot=Path(slot)
    if slot.parent.parent!=runtime.base or slot.parent.name!='expired-recovery-proofs':fail()
    stage.verify_final_storage();runtime.no_operation_containers(running_only=False)
    original=runtime.base/'rollout-before.tar';cut=state['cut']
    from preserved_upload import private_read
    raw_manifest=private_read(runtime.base/'rollout-before-manifest.json',128<<20)
    import json,hashlib
    if (canonical(cut['census'])!=cut['census_sha256']
        or hashlib.sha256(raw_manifest).hexdigest()!=cut['manifest_sha256']
        or json.loads(raw_manifest)!=cut['manifest']):fail()
    if not verify_archive(original,cut['manifest']):fail()
    interval=failed_startup_interval(state,awaiting)
    current=slot/'rollout-before.tar'
    manifest=archive_closed_store(stage.final_path,current)
    stage.verify_final_storage()
    old=observe_copy(runtime,original,cut['manifest'],'expired-original',stage.verify_final_storage)
    old_row,old_observations=recovered_census(cut['census'],old['census'],old['recovery'],cut['manifest'])
    if semantic(old_row)!=semantic(cut['census']):fail()
    observed=observe_copy(runtime,current,manifest,'expired-current',stage.verify_final_storage,validate)
    row,observations=recovered_census(cut['census'],observed['census'],observed['recovery'],cut['manifest'])
    if semantic(row)!=semantic(cut['census']) or old['records']!=observed['records']:fail()
    def read_bytes(side,path):
        archive,wanted=(original,cut['manifest']) if side=='before' else (current,manifest)
        if not verify_archive(archive,wanted):fail()
        with tarfile.open(archive,'r:') as bundle:
            member=bundle.getmember(path)
            if not member.isfile() or member.size>256*1024:fail()
            stream=bundle.extractfile(member)
            if stream is None:fail()
            with stream:return stream.read(256*1024+1)
    disposition=classify_startup(cut['manifest'],manifest,cut['census'],row,
        old['records'],observed['records'],read_bytes,checksum,interval)
    if not verify_archive(original,cut['manifest']) or not verify_archive(current,manifest):fail()
    stage.verify_final_storage();runtime.no_operation_containers(running_only=False)
    result={'schema':'voice-expired-current-observation-v1','operation':state['operation'],
        'original_cut_sha256':digest(cut),'manifest':manifest,'census':row,
        'full_records':observed['records'],'native_disposition':disposition,
        'private_recovery':observed['recovery'],'ephemeral_observations':observations,
        'original_private_recovery':old['recovery'],'original_ephemeral_observations':old_observations,
        'selected_inventory_sha256':digest(native_inventory(manifest))}
    save(slot/'rollout-before-manifest.json',result)
    save(slot/'copy-checkpoint.json',{'schema':'voice-expired-observation-copy-v1',
        'operation':state['operation'],'original_cut_sha256':digest(cut),
        'observation_sha256':digest(result),'archive_sha256':manifest['archive_sha256']})
    return result
