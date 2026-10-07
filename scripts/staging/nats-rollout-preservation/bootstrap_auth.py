"""Actual fixed API permission/default proof in a disposable isolated realm.

Original operator/account JWT and renewed existing user are retained. The root
signer is never mounted; no live/native PVC, business records or public port.
"""
import copy,json,os
from pathlib import Path
import tempfile,uuid
from bootstrap_root import secret_bytes,ACCOUNT_SHA,hash_bytes
from nats_contract_actor import Actor
from nats_contract_plan import DECLARATIONS,digest
from controller import Blocked
from docker_runtime import DockerRuntime,NATS_IMAGE
from scenario import ready
import guard

def prove(credentials,operator,unchanged):
    unchanged()
    with tempfile.TemporaryDirectory(prefix='bootstrap-auth-',dir=guard.ROOT) as td:
        base=Path(td);base.chmod(0o750);os.chown(base,0,65532)
        inputs=base/'inputs';inputs.mkdir(mode=0o750);os.chown(inputs,0,65532)
        for name,value in [('bootstrap.creds',credentials)]:
            p=inputs/name;p.write_bytes(value);p.chmod(0o440);os.chown(p,0,65532)
        tokens={key:secret_bytes(operator,key).decode().strip() for key in ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
        if hash_bytes(tokens['account.public'].encode())!=ACCOUNT_SHA:raise Blocked('bootstrap_auth_account_changed')
        q=json.dumps
        config='host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\nmax_control_line: 32768\noperator: '+q(tokens['operator.jwt'])+'\nsystem_account: '+q(tokens['system-account.public'])+'\nresolver: MEMORY\nresolver_preload: { '+q(tokens['account.public'])+': '+q(tokens['account.jwt'])+', '+q(tokens['system-account.public'])+': '+q(tokens['system-account.jwt'])+' }\njetstream { store_dir: /data }\n'
        p=base/'server.conf';p.write_text(config);p.chmod(0o440);os.chown(p,0,65532)
        store=base/'auth-store';store.mkdir(mode=0o700);os.chown(store,65532,65532)
        runtime=DockerRuntime(base,'auth'+uuid.uuid4().hex[:12])
        try:
            broker=runtime.start_broker('proof',store);ready(runtime,broker)
            actor=Actor(runtime,broker,runtime.owned[broker]['id'],store,unchanged)
            stream_subjects={}
            for stream,declaration in DECLARATIONS:
                stream_subjects.setdefault(stream,[]).append(declaration['filter_subject'])
            for name,subjects in stream_subjects.items():
                cfg={'name':name,'subjects':subjects,'storage':'file','retention':'limits','max_age':604800000000000,'duplicate_window':120000000000}
                actor._request('$JS.API.STREAM.CREATE.'+name,cfg)
            normalized_consumers=[]
            for stream,declaration in DECLARATIONS:
                durable=declaration['durable_name']
                actor.mutate('$JS.API.CONSUMER.CREATE.'+stream+'.'+durable,{'stream_name':stream,'action':'create','config':copy.deepcopy(declaration)})
                normalized=actor.info_config(stream+'/'+durable)
                if any(normalized.get(k)!=v for k,v in declaration.items()):raise Blocked('bootstrap_auth_consumer_not_exact')
                normalized_consumers.append({'stream':stream,'durable':durable,'config':normalized})
            tree=runtime.monitor_jsz(broker)
            if tree['messages']!=0 or tree['streams']!=len(stream_subjects) or tree['consumers']!=len(DECLARATIONS):raise Blocked('bootstrap_auth_scratch_not_empty')
            if [a['id'] for a in tree['account_details'] if a.get('stream_detail')]!=[tokens['account.public']]:raise Blocked('bootstrap_auth_account_not_exact')
            runtime.stop(broker);unchanged()
            legacy=next(row['config'] for row in normalized_consumers if row['stream']=='social_events')
            return {'verified':True,'server_image':NATS_IMAGE,'account_sha256':ACCOUNT_SHA,
                'reply_prefix':'_INBOX.voice.bootstrap.reply','normalized_consumer':legacy,
                'normalized_consumer_sha256':digest(legacy),'normalized_consumers':normalized_consumers,
                'normalized_consumers_sha256':digest(normalized_consumers)}
        finally:
            for name in reversed(list(runtime.owned)):
                row=runtime.inspect(name);runtime.run(['rm','-f',row['Id']])
                if runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']):raise Blocked('bootstrap_auth_cleanup_failed')
