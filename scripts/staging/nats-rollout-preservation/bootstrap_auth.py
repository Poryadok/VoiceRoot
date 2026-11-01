"""Actual fixed API permission/default proof in a disposable isolated realm.

Original operator/account JWT and renewed existing user are retained. The root
signer is never mounted; no live/native PVC, business records or public port.
"""
import copy,json,os
from pathlib import Path
import tempfile,uuid
from bootstrap_root import secret_bytes,ACCOUNT_SHA,hash_bytes
from nats_contract_actor import Actor
from nats_contract_plan import DECLARATIONS,V6_DECLARATIONS,V6_SCHEMA,V6_MIGRATION_ID,digest
from controller import Blocked
from docker_runtime import DockerRuntime,NATS_IMAGE
from scenario import ready
import guard
from actor_auth import AuthScratch

def prove(credentials,operator,unchanged,required_get_streams=None,*,version='voice-bootstrap-auth-v4',installation=None):
    from resource_claim import require_installation
    installation=require_installation(installation)
    if version not in ('voice-bootstrap-auth-v4','voice-bootstrap-auth-v5'):raise Blocked('bootstrap_auth_version_invalid')
    declarations=V6_DECLARATIONS if version=='voice-bootstrap-auth-v5' else DECLARATIONS
    unchanged()
    with AuthScratch('bootstrap-auth-',guard.ROOT) as scratch:
        base=scratch.path;base.chmod(0o750);os.chown(base,0,65532)
        inputs=base/'inputs';inputs.mkdir(mode=0o750);os.chown(inputs,0,65532);inputs.chmod(0o750)
        for name,value in [('bootstrap.creds',credentials)]:
            p=inputs/name;p.write_bytes(value);p.chmod(0o440);os.chown(p,0,65532)
        tokens={key:secret_bytes(operator,key).decode().strip() for key in ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
        if hash_bytes(tokens['account.public'].encode())!=installation.account_sha256:raise Blocked('bootstrap_auth_account_changed')
        q=json.dumps
        config='host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\nmax_control_line: 32768\noperator: '+q(tokens['operator.jwt'])+'\nsystem_account: '+q(tokens['system-account.public'])+'\nresolver: MEMORY\nresolver_preload: { '+q(tokens['account.public'])+': '+q(tokens['account.jwt'])+', '+q(tokens['system-account.public'])+': '+q(tokens['system-account.jwt'])+' }\njetstream { store_dir: /data }\n'
        p=base/'server.conf';p.write_text(config);p.chmod(0o440);os.chown(p,0,65532)
        store=base/'auth-store';store.mkdir(mode=0o700);os.chown(store,65532,65532)
        import commands,resource_claim
        session=getattr(commands.authority_guard,'__self__',None)
        cohort_runtime=isinstance(session,resource_claim.Session) and session.check().get('cohort_preservation_sha256') is not None
        if cohort_runtime:
            scratch.hold() # Constructor may persist a journal before it returns.
            import staging_cohort_preservation
            runtime=staging_cohort_preservation.auth_runtime(session,base,'bootstrap')
        else:runtime=DockerRuntime(base,'auth'+uuid.uuid4().hex[:12])
        scratch.hold()
        completed=False
        try:
            broker=runtime.start_broker('proof',store)
            ready(runtime,broker)
            actor=Actor(runtime,broker,runtime.owned[broker]['id'],store,unchanged,
                **({'plan':{'schema':V6_SCHEMA,'migration_id':V6_MIGRATION_ID}} if version=='voice-bootstrap-auth-v5' else {}))
            stream_subjects={}
            for stream,declaration in declarations:
                stream_subjects.setdefault(stream,[]).append(declaration['filter_subject'])
            for name,subjects in stream_subjects.items():
                cfg={'name':name,'subjects':subjects,'storage':'file','retention':'limits','max_age':604800000000000,'duplicate_window':120000000000}
                actor._request('$JS.API.STREAM.CREATE.'+name,cfg)
            if version=='voice-bootstrap-auth-v5':
                # Qualify actual retained bootstrap UPDATE/INFO rights on an
                # empty disposable realm before opening a populated archive.
                cfg=actor.info_config('voice_events')
                actor.mutate('$JS.API.STREAM.UPDATE.voice_events',copy.deepcopy(cfg))
                if actor.info_config('voice_events')!=cfg:raise Blocked('bootstrap_auth_stream_not_exact')
            normalized_consumers=[]
            for stream,declaration in declarations:
                durable=declaration['durable_name']
                actor.mutate('$JS.API.CONSUMER.CREATE.'+stream+'.'+durable,{'stream_name':stream,'action':'create','config':copy.deepcopy(declaration)})
                normalized=actor.info_config(stream+'/'+durable)
                if any(normalized.get(k)!=v for k,v in declaration.items()):raise Blocked('bootstrap_auth_consumer_not_exact')
                if declaration['durable_name']=='matchmaking_voice_roster_v1' and (
                    normalized.get('deliver_subject','')!='' or normalized.get('deliver_group','')!=''
                    or normalized.get('filter_subjects',[])!=[]):raise Blocked('bootstrap_auth_consumer_not_exact')
                normalized_consumers.append({'stream':stream,'durable':durable,'config':normalized})
            tree=runtime.monitor_jsz(broker)
            if tree['messages']!=0 or tree['streams']!=len(stream_subjects) or tree['consumers']!=len(declarations):raise Blocked('bootstrap_auth_scratch_not_empty')
            if [a['id'] for a in tree['account_details'] if a.get('stream_detail')]!=[tokens['account.public']]:raise Blocked('bootstrap_auth_account_not_exact')
            get_proof=None
            if required_get_streams is not None:
                from rollout_census import census,semantic
                class Captured:
                    def __init__(self,row):self.row=row
                    def monitor_jsz(self,broker):return self.row
                before_get=census(Captured(tree),'scratch',tokens['account.public'])
                get_proof=actor.prove_get_permissions(required_get_streams)
                after=runtime.monitor_jsz(broker)
                if semantic(census(Captured(after),'scratch',tokens['account.public']))!=semantic(before_get):
                    raise Blocked('bootstrap_auth_get_scratch_changed')
            runtime.stop(broker);unchanged()
            legacy=next(row['config'] for row in normalized_consumers if row['stream']=='social_events')
            result={'schema':version,'verified':True,'server_image':NATS_IMAGE,'account_sha256':installation.account_sha256,
                'reply_prefix':'_INBOX.voice.bootstrap.reply','normalized_consumer':legacy,
                'normalized_consumer_sha256':digest(legacy),'normalized_consumers':normalized_consumers,
                'normalized_consumers_sha256':digest(normalized_consumers)}
            if get_proof is not None:result['get_permissions']=get_proof
            completed=True
            return result
        finally:
            try:
                for name in reversed(list(runtime.owned)):
                    row=runtime.inspect(name);runtime.run(['rm','-f',row['Id']])
                    if runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']):raise Blocked('bootstrap_auth_cleanup_failed')
            finally:
                if cohort_runtime:runtime.finish_auth()
            if not cohort_runtime:runtime.no_operation_containers()
            if completed:scratch.release()
