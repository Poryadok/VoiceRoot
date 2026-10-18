"""Fixed authenticated API actor in the owned network-none broker namespace."""
import copy
import json
from pathlib import Path
from nats_contract_plan import ContractError,DECLARATIONS,V4_SCHEMA,V4_MIGRATION_ID,stream_updates_for,declarations_for
_STREAMS=tuple(dict.fromkeys(stream for stream,_ in DECLARATIONS))
_CONSUMERS={(stream,config['durable_name']):config for stream,config in DECLARATIONS}
_CONSUMER_OBJECTS=frozenset(stream+'/'+durable for stream,durable in _CONSUMERS)
INFO={name:'$JS.API.STREAM.INFO.'+name for name in _STREAMS}
INFO.update({stream+'/'+durable:'$JS.API.CONSUMER.INFO.'+stream+'.'+durable for stream,durable in _CONSUMERS})
_CONSUMER_INFO_APIS=frozenset(INFO[obj] for obj in _CONSUMER_OBJECTS)
WRITE={'$JS.API.STREAM.UPDATE.'+name:name for name in ('chat_events','social_events','role_events')}
WRITE.update({'$JS.API.CONSUMER.CREATE.'+stream+'.'+durable:stream+'/'+durable for stream,durable in _CONSUMERS})
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
    def __init__(self,runtime,broker,broker_id,store,fence,*,mutation_guard=None,plan=None):
        self.runtime=runtime;self.broker=broker;self.broker_id=broker_id;self.store=str(Path(store));self.fence=fence;self.counter=0;self.mutation_guard=mutation_guard
        selected=plan or {'schema':V4_SCHEMA,'migration_id':V4_MIGRATION_ID}
        self.streams=stream_updates_for(selected)
        self.consumers={(stream,config['durable_name']):config for stream,config in declarations_for(selected)}
        self.write={'$JS.API.STREAM.UPDATE.'+name:name for name in self.streams}
        self.write.update({'$JS.API.CONSUMER.CREATE.'+s+'.'+d:s+'/'+d for s,d in self.consumers})
        names=set(self.streams)|{s for s,_ in self.consumers}
        self.info={name:'$JS.API.STREAM.INFO.'+name for name in names}
        self.info.update({s+'/'+d:'$JS.API.CONSUMER.INFO.'+s+'.'+d for s,d in self.consumers})
    def assert_closed_isolation(self):
        self.fence();row=self.runtime.inspect(self.broker)
        if row['Id']!=self.broker_id or not row['State']['Running'] or row['HostConfig']['NetworkMode']!='none' or row['HostConfig'].get('PortBindings'):fail()
        data=[m for m in row['Mounts'] if m.get('Destination')=='/data']
        if len(data)!=1 or data[0].get('Type')!='bind' or data[0].get('Source')!=self.store or not data[0].get('RW'):fail()
        if any(m.get('RW') and m.get('Destination')!='/data' for m in row['Mounts'] if m.get('Type')=='bind'):fail()
    def _request(self,api,payload,missing=False,permission_probe=False):
        from docker_runtime import BOX_IMAGE
        self.assert_closed_isolation();self.counter+=1
        # Fixed shell text; subject and JSON are separate positional arguments.
        # Credentials stay in root-captured read-only /inputs, never argv/logs.
        command=['/bin/sh','-c','exec nats --server nats://127.0.0.1:4222 --creds /inputs/bootstrap.creds --inbox-prefix _INBOX.voice.bootstrap.reply --timeout 5s req --raw "$1" "$2"','voice-contract',api,json.dumps(payload,separators=(',',':')) if payload is not None else '']
        name=self.runtime.create('contract-'+str(len(self.runtime.owned))+'-'+str(self.counter),BOX_IMAGE,[(self.runtime.inputs_directory(),'/inputs',False)],command,self.broker)
        # This is the mutating CLI's use boundary, after durable issued intent
        # and client creation; read-only INFO/GET requests remain unchanged.
        if api in self.write and self.mutation_guard is not None:self.mutation_guard()
        self.runtime.run(['start',name]);exitcode=self.runtime.run(['wait',name],timeout=15)
        if self.runtime.inspect(name)['State']['Running']:fail()
        self.assert_closed_isolation()
        if exitcode!='0':fail()
        raw=self.runtime.run(['logs',name],timeout=10,limit=256<<10)
        try:row=json.loads(raw)
        except Exception:fail()
        if not isinstance(row,dict):fail()
        if row.get('error'):
            if permission_probe and api.startswith('$JS.API.STREAM.MSG.GET.'):
                error=row['error']
                if (set(row)=={'type','error'} and row['type']=='io.nats.jetstream.api.v1.stream_msg_get_response'
                    and isinstance(error,dict) and set(error)=={'code','err_code','description'}
                    and type(error['code']) is int and error['code']==404
                    and type(error['err_code']) is int and error['err_code'] in (10037,10059)
                    and isinstance(error['description'],str) and 0<len(error['description'])<=256):return None
                fail()
            if missing and api in _CONSUMER_INFO_APIS and row['error'].get('err_code')==10014:return None
            if missing and api.startswith('$JS.API.STREAM.MSG.GET.') and row['error'].get('err_code')==10037:return None
            fail()
        return row
    def prove_get_permissions(self,streams):
        """Only for an independently proved empty disposable auth realm.

        Every exact GET subject must yield the pinned missing-message/stream
        response. Permission errors, timeouts, records and unknown shapes veto.
        Neither credentials nor error descriptions are returned.
        """
        import re
        if (not isinstance(streams,list) or not streams or len(streams)>256
            or len(set(streams))!=len(streams)
            or any(not isinstance(name,str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,255}',name) for name in streams)):fail()
        for name in sorted(streams):
            if self._request('$JS.API.STREAM.MSG.GET.'+name,{'seq':1},permission_probe=True) is not None:fail()
        return {'schema':'voice-retained-get-permission-proof-v1',
            'subjects_sha256':__import__('hashlib').sha256(json.dumps(sorted(streams),separators=(',',':')).encode()).hexdigest(),
            'streams':len(streams),'verified':True}
    def get_record(self,stream,sequence):
        import re
        if (not isinstance(stream,str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,255}',stream)
            or type(sequence)!=int or not 1<=sequence<=2**64-1):fail()
        row=self._request('$JS.API.STREAM.MSG.GET.'+stream,{'seq':sequence},missing=True)
        if row is None:return None
        if (set(row)!={'type','message'}
            or row['type']!='io.nats.jetstream.api.v1.stream_msg_get_response'
            or not isinstance(row.get('message'),dict) or row['message'].get('seq')!=sequence):fail()
        return row['message']
    def info_config(self,obj):
        if obj not in self.info:fail()
        row=self._request(self.info[obj],None,missing=obj in _CONSUMER_OBJECTS)
        if row is None:return None
        if not isinstance(row.get('config'),dict):fail()
        if '/' not in obj:
            if row['config'].get('name')!=obj:fail()
        else:
            stream,durable=obj.split('/',1)
            if row.get('stream_name')!=stream or row.get('name')!=durable:fail()
        return persistent_config(row['config'])
    def mutate(self,api,payload):
        if api not in self.write or not isinstance(payload,dict):fail()
        if '.STREAM.UPDATE.' in api:
            if payload.get('name')!=self.write[api]:fail()
        else:
            stream_durable=WRITE[api]
            stream,durable=stream_durable.split('/',1)
            if (set(payload)!={'stream_name','action','config'} or payload.get('stream_name')!=stream
                or payload.get('action')!='create' or payload.get('config')!=self.consumers[(stream,durable)]):fail()
        self._request(api,payload)
