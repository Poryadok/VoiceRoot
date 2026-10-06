"""Fixed authenticated API actor in the owned network-none broker namespace."""
import copy
import json
from pathlib import Path
from nats_contract_plan import ContractError
INFO={'chat_events':'$JS.API.STREAM.INFO.chat_events','social_events':'$JS.API.STREAM.INFO.social_events','social_events/rt_realtime1_friend_removed':'$JS.API.CONSUMER.INFO.social_events.rt_realtime1_friend_removed'}
WRITE={'$JS.API.STREAM.UPDATE.chat_events':'chat_events','$JS.API.STREAM.UPDATE.social_events':'social_events','$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed':'social_events/rt_realtime1_friend_removed'}
def fail():raise ContractError('nats_contract_actor_refused')
def persistent_config(config):
    # NATS v2.12.12 setDynamic{Stream,Consumer}Metadata adds exactly these
    # response-only keys. Static _nats.req.level and user metadata stay bound.
    result=copy.deepcopy(config);metadata=result.get('metadata')
    if metadata is not None:
        if not isinstance(metadata,dict):fail()
        dynamic={'_nats.ver','_nats.level'}&set(metadata)
        if dynamic:
            if dynamic!={'_nats.ver','_nats.level'} or metadata['_nats.ver']!='2.12.12' or metadata['_nats.level']!='3':fail()
            del metadata['_nats.ver'];del metadata['_nats.level']
            if not metadata:del result['metadata']
    return result
class Actor:
    def __init__(self,runtime,broker,broker_id,store,fence):
        self.runtime=runtime;self.broker=broker;self.broker_id=broker_id;self.store=str(Path(store));self.fence=fence;self.counter=0
    def assert_closed_isolation(self):
        self.fence();row=self.runtime.inspect(self.broker)
        if row['Id']!=self.broker_id or not row['State']['Running'] or row['HostConfig']['NetworkMode']!='none' or row['HostConfig'].get('PortBindings'):fail()
        data=[m for m in row['Mounts'] if m.get('Destination')=='/data']
        if len(data)!=1 or data[0].get('Type')!='bind' or data[0].get('Source')!=self.store or not data[0].get('RW'):fail()
        if any(m.get('RW') and m.get('Destination')!='/data' for m in row['Mounts'] if m.get('Type')=='bind'):fail()
    def _request(self,api,payload,missing=False):
        from docker_runtime import BOX_IMAGE
        self.assert_closed_isolation();self.counter+=1
        # Fixed shell text; subject and JSON are separate positional arguments.
        # Credentials stay in root-captured read-only /inputs, never argv/logs.
        command=['/bin/sh','-c','exec nats --server nats://127.0.0.1:4222 --creds /inputs/bootstrap.creds --inbox-prefix _INBOX.voice.bootstrap.reply --timeout 5s req --raw "$1" "$2"','voice-contract',api,json.dumps(payload,separators=(',',':')) if payload is not None else '']
        name=self.runtime.create('contract-'+str(len(self.runtime.owned))+'-'+str(self.counter),BOX_IMAGE,[(self.runtime.base/'inputs','/inputs',False)],command,self.broker)
        self.runtime.run(['start',name]);exitcode=self.runtime.run(['wait',name],timeout=15)
        if self.runtime.inspect(name)['State']['Running']:fail()
        self.assert_closed_isolation()
        if exitcode!='0':fail()
        raw=self.runtime.run(['logs',name],timeout=10,limit=256<<10)
        try:row=json.loads(raw)
        except Exception:fail()
        if not isinstance(row,dict):fail()
        if row.get('error'):
            if missing and row['error'].get('err_code')==10014:return None
            fail()
        return row
    def info_config(self,obj):
        if obj not in INFO:fail()
        row=self._request(INFO[obj],None,missing=obj=='social_events/rt_realtime1_friend_removed')
        if row is None:return None
        if not isinstance(row.get('config'),dict):fail()
        if obj in ('chat_events','social_events'):
            if row['config'].get('name')!=obj:fail()
        elif row.get('stream_name')!='social_events' or row.get('name')!='rt_realtime1_friend_removed':fail()
        return persistent_config(row['config'])
    def mutate(self,api,payload):
        if api not in WRITE or not isinstance(payload,dict):fail()
        if '.STREAM.UPDATE.' in api and payload.get('name')!=WRITE[api]:fail()
        if '.CONSUMER.CREATE.' in api and (payload.get('stream_name')!='social_events' or payload.get('action')!='create' or payload.get('config',{}).get('durable_name')!='rt_realtime1_friend_removed'):fail()
        self._request(api,payload)
