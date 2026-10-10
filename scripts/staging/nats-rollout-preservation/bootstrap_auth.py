"""Actual fixed API permission/default proof in a disposable isolated realm.

Original operator/account JWT and renewed existing user are retained. The root
signer is never mounted; no live/native PVC, business records or public port.
"""
import copy,json,os
from pathlib import Path
import tempfile,uuid
from bootstrap_root import secret_bytes,ACCOUNT_SHA,hash_bytes
from nats_contract_actor import Actor
from nats_contract_plan import DECLARATION,digest
from controller import Blocked
from docker_runtime import DockerRuntime,NATS_IMAGE
from scenario import ready
import guard

def prove(credentials,operator,unchanged,required_get_streams=None):
    unchanged()
    with tempfile.TemporaryDirectory(prefix='bootstrap-auth-',dir=guard.ROOT) as td:
        base=Path(td);base.chmod(0o750);os.chown(base,0,65532)
        inputs=base/'inputs';inputs.mkdir(mode=0o750);os.chown(inputs,0,65532);inputs.chmod(0o750)
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
            for name,subjects in [('chat_events',['space.created']),('social_events',['social.friend_removed'])]:
                cfg={'name':name,'subjects':subjects,'storage':'file','retention':'limits','max_age':604800000000000,'duplicate_window':120000000000}
                actor._request('$JS.API.STREAM.CREATE.'+name,cfg)
                before=actor.info_config(name);target=copy.deepcopy(before)
                if name=='chat_events':target['subjects']+=['space.deletion_scheduled','space.restored']
                else:target['duplicate_window']=86400000000000
                actor.mutate('$JS.API.STREAM.UPDATE.'+name,target)
                if actor.info_config(name)!=target:raise Blocked('bootstrap_auth_update_not_exact')
            actor.mutate('$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed',{'stream_name':'social_events','action':'create','config':copy.deepcopy(DECLARATION)})
            normalized=actor.info_config('social_events/rt_realtime1_friend_removed')
            if any(normalized.get(k)!=v for k,v in DECLARATION.items()):raise Blocked('bootstrap_auth_consumer_not_exact')
            tree=runtime.monitor_jsz(broker)
            if tree['messages']!=0 or tree['streams']!=2 or tree['consumers']!=1:raise Blocked('bootstrap_auth_scratch_not_empty')
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
            result={'verified':True,'server_image':NATS_IMAGE,'account_sha256':ACCOUNT_SHA,
                'reply_prefix':'_INBOX.voice.bootstrap.reply','normalized_consumer':normalized,
                'normalized_consumer_sha256':digest(normalized)}
            if get_proof is not None:result['get_permissions']=get_proof
            return result
        finally:
            for name in reversed(list(runtime.owned)):
                row=runtime.inspect(name);runtime.run(['rm','-f',row['Id']])
                if runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']):raise Blocked('bootstrap_auth_cleanup_failed')
