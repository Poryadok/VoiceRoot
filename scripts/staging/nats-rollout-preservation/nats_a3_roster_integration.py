"""Pinned native A3 fixture; selected stores are never pulled or ACKed.

Synthetic signed policy credentials prove only this disposable realm. Actual
installed rights and MM/PG commit-before-ACK remain separate activation gates.
"""
import base64
import copy
from datetime import datetime,timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import tempfile
import time
import uuid

import nats_contract_integration as legacy
from nats_contract_plan import V6_SCHEMA,V6_MIGRATION_ID,V6_DECLARATIONS,compile_plan,digest


def build(bundle):
    bundle=Path(bundle)
    legacy.build_fixture_records(bundle,a3_roster=True)
    source=Path(__file__).parent/'testdata/a3-roster-source'
    shutil.copytree(source,bundle/'a3-source')


def _durable_save(path,state):
    raw=json.dumps(state,sort_keys=True,separators=(',',':')).encode()
    temporary=path.with_suffix('.new')
    fd=os.open(temporary,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,'wb') as out:out.write(raw);out.flush();os.fsync(out.fileno())
    os.replace(temporary,path)
    fd=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY)
    try:os.fsync(fd)
    finally:os.close(fd)


def prepare(base):
    import bootstrap_auth,guard
    from bootstrap_renewal import invoke
    legacy.fixture.call(['/bundle/helper','prepare',str(base)])
    shutil.copyfile('/bundle/nats-9ae-predecessor.json',base/'inputs/predecessor.json')
    (base/'inputs/predecessor.json').chmod(0o440)
    for name in ('kernel','helper','a3-client'):
        shutil.copyfile('/bundle/'+name,base/name);(base/name).chmod(0o550);os.chown(base/name,0,65532)
    for role in ('realtime','notification','analytics-chat','search'):
        p=base/('bootstrap-'+role+'.sh')
        shutil.copyfile('/source/scripts/staging/nats-known-baseline/testdata/deployed-bootstrap-20261004/'+role+'-bootstrap.sh',p)
        p.chmod(0o440);os.chown(p,0,65532)
    for p in [base/'inputs',base/'server.conf',*list((base/'inputs').iterdir())]:os.chown(p,0,65532)
    old=(base/'inputs/bootstrap.creds').read_text()
    claims=json.loads(base64.urlsafe_b64decode(old.split('-----BEGIN NATS USER JWT-----\n',1)[1].split('\n',1)[0].split('.')[1]+'=='))
    account=(base/'inputs/account.public').read_text().strip()
    request={'schema':'voice-nats-existing-bootstrap-renewal-v1','old_creds':old,
        'account_jwt':(base/'inputs/account.jwt').read_text().strip(),
        'operator_jwt':(base/'inputs/operator.jwt').read_text().strip(),
        'signer_seed':(base/'inputs/synthetic-account.seed').read_text(),
        'actor_sha256':hashlib.sha256(claims['sub'].encode()).hexdigest(),
        'account_sha256':hashlib.sha256(account.encode()).hexdigest()}
    binary=base/'bootstrap-renewer';shutil.copyfile('/bundle/bootstrap-renewer',binary);binary.chmod(0o550)
    renewed=invoke(binary,request);(base/'inputs/bootstrap.creds').write_bytes(renewed)
    operator={'data':{k:base64.b64encode((base/'inputs'/k).read_bytes()).decode() for k in (
        'operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}}
    guard.ROOT=base/'root-custody';guard.ROOT.mkdir(mode=0o700)
    installation=legacy.fixture_installation(request['account_sha256'],request['actor_sha256'],operator)
    enrolled=bootstrap_auth.prove(renewed,operator,lambda:None,installation=installation)
    fresh=bootstrap_auth.prove(renewed,operator,lambda:None,version='voice-bootstrap-auth-v5',installation=installation)
    from nats_root_plan import _validated_normalized_consumers
    normalized=_validated_normalized_consumers(fresh,enrolled,version=V6_SCHEMA)
    return enrolled,fresh,normalized


def _case_base(base,mode):
    folder=base/mode;folder.mkdir(mode=0o750);os.chown(folder,0,65532)
    shutil.copytree(base/'inputs',folder/'inputs')
    for p in [folder/'inputs',*list((folder/'inputs').iterdir())]:os.chown(p,0,65532)
    for name in ('kernel','helper','a3-client','server.conf',*[f'bootstrap-{p}.sh' for p in ('realtime','notification','analytics-chat','search')]):
        shutil.copyfile(base/name,folder/name);os.chown(folder/name,0,65532)
        (folder/name).chmod(0o550 if name in ('kernel','helper','a3-client') else 0o440)
    return folder


def _client(runtime,broker,*args):
    from docker_runtime import NATS_IMAGE
    name=runtime.create('a3-client-'+str(len(runtime.owned)),NATS_IMAGE,
        [(runtime.base/'a3-client','/kernel',False),(runtime.inputs_directory(),'/inputs',False)],['/kernel',*args],broker)
    runtime.run(['start',name])
    code=runtime.run(['wait',name],timeout=50)
    logs=runtime.run(['logs',name],limit=64<<10)
    if code!='0':raise RuntimeError('a3_fixture_client_failed '+logs[-2000:])
    return logs


def _inputs(runtime,broker,store,fence):
    from nats_contract_actor import Actor
    actor=Actor(runtime,broker,runtime.owned[broker]['id'],store,fence,
        plan={'schema':V6_SCHEMA,'migration_id':V6_MIGRATION_ID})
    tree=runtime.monitor_jsz(broker)
    detail=next(a for a in tree['account_details'] if a['id']==runtime.account_id())['stream_detail']
    streams={s['name']:{'config':s['config'],'state':s['state']} for s in detail}
    consumers={}
    for stream,cfg in V6_DECLARATIONS:
        value=actor.info_config(stream+'/'+cfg['durable_name'])
        if value is not None:consumers[(stream,cfg['durable_name'])]={'config':value}
    return streams,consumers


def run_case(base,mode,enrolled,fresh,normalized):
    from docker_runtime import DockerRuntime,NATS_IMAGE
    from scenario import ready
    from preserve import capture_cut,closed_copy_census,closed_native_states,native_message_files
    from native_store import archive_closed_store
    from rollout_census import census
    import nats_migration
    folder=_case_base(base,mode)
    operation='all'+uuid.uuid4().hex[:12]
    runtime=DockerRuntime(folder,operation)
    store=folder/'store';store.mkdir(mode=0o700);os.chown(store,65532,65532)
    original=None
    try:
        original=runtime.start_broker('original',store);ready(runtime,original)
        runtime.bootstrap(original)
        runtime.kernel(original,'seed')  # Existing pending social payload/state.
        existing=mode in ('existing-unacked','existing-acked')
        _client(runtime,original,'seed','populated' if existing else mode)
        if existing:
            # Controlled application progress BEFORE this operation's original
            # cut. It must become preserved old state, never CREATE recovery.
            from nats_contract_actor import Actor
            actor=Actor(runtime,original,runtime.owned[original]['id'],store,lambda:None,
                plan={'schema':V6_SCHEMA,'migration_id':V6_MIGRATION_ID})
            actor.mutate('$JS.API.CONSUMER.CREATE.voice_events.matchmaking_voice_roster_v1',
                {'stream_name':'voice_events','action':'create','config':copy.deepcopy(V6_DECLARATIONS[-1][1])})
            _client(runtime,original,'deliver','lost-ack' if mode=='existing-unacked' else 'all')
        streams,consumers=_inputs(runtime,original,store,lambda:None)
        p=compile_plan('/bundle/a3-source',streams,consumers,normalized,version=V6_SCHEMA)
        runtime.stop(original)
        def fence():
            if runtime.inspect(original)['State']['Running'] or store.resolve()!=store:raise RuntimeError('a3_original_fence_lost')
        cut=capture_cut(runtime,store,fence);cut['manifest_sha256']=digest(cut['manifest'])
        before_messages=sum(s['state']['messages'] for s in cut['census']['streams'])
        state={'operation':operation,'events':[],'custody':{'verified':True,'scope':'owned-synthetic-native-fixture'},
            'bootstrap_enrollment':{'auth_proof':enrolled},'nats_contract':p,'cut':cut}
        state['nats_contract_binding']={'plan_sha256':digest(p),'server_image':NATS_IMAGE,
            'enrollment_sha256':digest(state['bootstrap_enrollment']),'live_auth_sha256':digest(fresh),'sources':p['sources']}
        class Stage:
            final_path=store;final_claim={};final_pv={}
            def verify_final_storage(self):fence()
            def selected_store_descriptor(self):return legacy._private_fixture_store_descriptor(folder,store,fence)
        stage=Stage()
        def journal(event):state['events'].append(copy.deepcopy(event));_durable_save(folder/'checkpoint.json',state)
        create_api='$JS.API.CONSUMER.CREATE.voice_events.matchmaking_voice_roster_v1'
        crash_targets=[] if existing else [(create_api,'response-lost')]
        if mode=='empty13':crash_targets.insert(0,(create_api,'before-mutation'))
        if mode=='empty12':crash_targets.insert(0,(create_api,'before-intent'))
        if mode in ('empty10','empty12'):crash_targets.insert(0,('$JS.API.STREAM.UPDATE.voice_events','response-lost'))
        real_execute=nats_migration.execute;crashed=[]
        for target,phase in crash_targets:
            def interrupted(plan,actor,writer):
                mutate=actor.mutate
                def interrupted_writer(event):
                    if phase=='before-intent' and event.get('api')==target and event.get('kind')=='nats_contract_intent':
                        raise RuntimeError('a3_owned_response_lost')
                    writer(event)
                def lost(api,payload):
                    if api==target and phase=='before-mutation':raise RuntimeError('a3_owned_response_lost')
                    mutate(api,payload)
                    if api==target and phase=='response-lost':raise RuntimeError('a3_owned_response_lost')
                actor.mutate=lost
                return real_execute(plan,actor,interrupted_writer)
            nats_migration.execute=interrupted
            try:nats_migration.apply_contract(folder,state,stage,journal)
            except RuntimeError as error:
                if str(error)!='a3_owned_response_lost':raise
                crashed.append({'api':target,'phase':phase})
            else:raise RuntimeError('a3_expected_actual_mutation_interruption_missing')
            finally:nats_migration.execute=real_execute
        result=nats_migration.apply_contract(folder,state,stage,journal)
        proof=result['proof'];descriptor=state['nats_deliver_all_cut']
        expected=2 if mode=='populated' or existing else 0
        if descriptor['matching_messages']!=expected or len(proof['created_consumer_evidence'])!=(0 if existing else 1):raise RuntimeError('a3_backlog_proof_mismatch')
        if proof['messages']!=before_messages:raise RuntimeError('a3_original_records_changed')
        # New operation/cut for a genuine subsequent no-op; never reuse a
        # progressed application state inside the interrupted prior operation.
        inputs={}
        # Only fixed INFO calls use this callback; actor's exact store binding
        # is obtained below from the /data mount, not mount ordering.
        def read(r,broker,tree,row,manifest):
            mounts=[m for m in r.inspect(broker)['Mounts'] if m.get('Destination')=='/data']
            if len(mounts)!=1:raise RuntimeError('a3_copy_store_invalid')
            inputs['value']=_inputs(r,broker,Path(mounts[0]['Source']),fence)
        closed_copy_census(runtime,Path(result['cut']['archive']),result['cut']['manifest'],'all-noop-inputs',read)
        noop=compile_plan('/bundle/a3-source',*inputs['value'],normalized,version=V6_SCHEMA)
        if noop['actions']:raise RuntimeError('a3_second_compile_not_noop')
        next_state={'operation':operation,'events':[],'custody':state['custody'],'bootstrap_enrollment':state['bootstrap_enrollment'],
            'nats_contract':noop,'cut':result['cut'],
            'nats_contract_binding':{**state['nats_contract_binding'],'plan_sha256':digest(noop)}}
        def next_journal(e):next_state['events'].append(copy.deepcopy(e));_durable_save(folder/'noop-checkpoint.json',next_state)
        no_op=nats_migration.apply_contract(folder,next_state,stage,next_journal)
        if no_op['applied'] or no_op['proof']['created_consumer_evidence']!=[]:raise RuntimeError('a3_noop_created_consumer')
        delivery=[]
        if mode=='populated':
            # All delivery/ACK activity is confined to disposable POST-proof
            # clones, never fed back into the selected store/cold cut.
            for kind in ('all','lost-ack'):
                target_store=folder/('delivery-'+kind)
                runtime.restore(Path(result['cut']['archive']),target_store,result['cut']['manifest'])
                broker=runtime.start_broker('delivery-'+kind,target_store);ready(runtime,broker)
                delivery.append(_client(runtime,broker,'deliver',kind))
                if kind=='lost-ack':
                    snapshot=census(runtime,broker,runtime.account_id());runtime.stop(broker)
                    archive=folder/'lost-ack-closed.tar';manifest=archive_closed_store(target_store,archive)
                    states=closed_native_states(runtime,archive,manifest,snapshot,fence)
                    own=[v for k,v in states.items() if '/obs/matchmaking_voice_roster_v1/' in k]
                    if len(own)!=1 or set(own[0]['pending'])!={'4'}:raise RuntimeError('a3_lost_ack_native_pending_missing')
                    time.sleep(31)  # Owned bounded AckWait; no peer/resource wait.
                    runtime.restart(broker);ready(runtime,broker)
                    delivery.append(_client(runtime,broker,'deliver','finish'))
                info=next(c for c in census(runtime,broker,runtime.account_id())['consumers'] if c['stream']=='voice_events' and c['name']=='matchmaking_voice_roster_v1')
                if info['num_pending'] or info['num_ack_pending']:raise RuntimeError('a3_delivery_backlog_not_settled')
                original_info=next(c for c in result['cut']['census']['consumers'] if c['stream']=='voice_events' and c['name']=='matchmaking_voice_roster_v1')
                if any(info[k]!=original_info[k] for k in ('created','config_sha256')):raise RuntimeError('a3_delivery_identity_changed')
                runtime.stop(broker)
                settled=archive_closed_store(target_store,folder/('delivery-'+kind+'-settled.tar'))
                if native_message_files(settled)!=native_message_files(result['cut']['manifest']):raise RuntimeError('a3_delivery_records_changed')
        _durable_save(folder/'result.json',{'schema':'voice-a3-native-case-v1','mode':mode,'source_hashes':p['sources'],
            'plan_sha256':digest(p),'proof':proof,'noop_proof':no_op['proof'],'crash_targets':crashed,
            'delivery':delivery,'selected_store_pulled':False,'installed_rights_proved':False,
            'product_mm_pg_ack_ordering':'OPEN owner147 C4/C5'})
        return {'mode':mode,'original_messages':before_messages,'matching_messages':expected,
            'created_consumers':proof['created_consumers'],'old_consumers_preserved':proof['old_consumers_preserved'],
            'crash_targets':crashed,'noop':True,'delivery_clones':len(delivery),'proof_sha256':digest(proof)}
    finally:
        for name in reversed(list(runtime.owned)):
            row=runtime.inspect(name)
            if row['State']['Running']:runtime.stop(name)
            runtime.run(['rm',row['Id']])
            if runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']):raise RuntimeError('a3_fixture_cleanup_failed')


def inner(base):
    base=Path(base);legacy._set_archive_base_custody(base)
    enrolled,fresh,normalized=prepare(base)
    rights=qualify_rights(base)
    results=[]
    for mode in ('empty10','empty12','empty13','other','advanced-empty','populated','existing-unacked','existing-acked'):
        result=run_case(base,mode,enrolled,fresh,normalized)
        results.append(result)
        print(json.dumps({'completed_case':result},sort_keys=True),flush=True)
    print(json.dumps({'schema':'voice-a3-nats-native-fixture-v1','cases':results,'signed_fixture_rights':rights,
        'installed_rights_proved':False,'product_mm_pg_ack_ordering':'OPEN owner147 C4/C5'},sort_keys=True),flush=True)


def inner_existing(base):
    """Bounded supplemental cases for stronger delivery and original progress."""
    base=Path(base);legacy._set_archive_base_custody(base)
    enrolled,fresh,normalized=prepare(base)
    results=[]
    for mode in ('populated','existing-unacked','existing-acked'):
        result=run_case(base,mode,enrolled,fresh,normalized)
        results.append(result)
        print(json.dumps({'completed_case':result},sort_keys=True),flush=True)
    print(json.dumps({'schema':'voice-a3-nats-existing-fixture-v1','cases':results,
        'installed_rights_proved':False,'product_mm_pg_ack_ordering':'OPEN owner147 C4/C5'},sort_keys=True),flush=True)


def qualify_rights(base):
    from docker_runtime import DockerRuntime
    from scenario import ready
    from nats_contract_actor import Actor
    folder=_case_base(base,'signed-rights')
    runtime=DockerRuntime(folder,'rights'+uuid.uuid4().hex[:12])
    store=folder/'store';store.mkdir(mode=0o700);os.chown(store,65532,65532)
    try:
        broker=runtime.start_broker('rights',store);ready(runtime,broker);runtime.bootstrap(broker)
        _client(runtime,broker,'seed','empty13')
        actor=Actor(runtime,broker,runtime.owned[broker]['id'],store,lambda:None,
            plan={'schema':V6_SCHEMA,'migration_id':V6_MIGRATION_ID})
        actor.mutate('$JS.API.CONSUMER.CREATE.voice_events.matchmaking_voice_roster_v1',
            {'stream_name':'voice_events','action':'create','config':copy.deepcopy(V6_DECLARATIONS[-1][1])})
        result=_client(runtime,broker,'rights');runtime.stop(broker)
        return json.loads(result)
    finally:
        for name in reversed(list(runtime.owned)):
            row=runtime.inspect(name)
            if row['State']['Running']:runtime.stop(name)
            runtime.run(['rm',row['Id']])
