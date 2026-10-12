import json
import sys
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
from nats_contract_actor import Actor,INFO,WRITE
from nats_contract_plan import ContractError,DECLARATIONS
class Runtime:
    base=Path('/root-owned');owned={}
    def __init__(self):self.row={'Id':'owned-id','State':{'Running':True},'HostConfig':{'NetworkMode':'none','PortBindings':{}},'Mounts':[{'Type':'bind','Source':str(Path('/selected-pvc')),'Destination':'/data','RW':True}]};self.calls=[]
    def inspect(self,name):return self.row
class RequestRuntime(Runtime):
    def __init__(self,response):
        super().__init__();self.response=response;self.created=[]
    def inputs_directory(self):return Path('/inputs')
    def create(self,name,image,mounts,command,broker):
        self.created.append((name,command));return name
    def run(self,args,**kwargs):
        if args[0]=='wait':return '0'
        if args[0]=='logs':return json.dumps(self.response)
        return ''
    def inspect(self,name):
        if name.startswith('contract-'):
            return {'State':{'Running':False}}
        return super().inspect(name)
class Tests(unittest.TestCase):
    def actor(self,r,fence=lambda:None):return Actor(r,'owned-broker','owned-id',Path('/selected-pvc'),fence)
    def test_public_network_refused(self):
        r=Runtime();r.row['HostConfig']['NetworkMode']='host'
        with self.assertRaises(ContractError):self.actor(r).assert_closed_isolation()
    def test_changed_container_identity_refused(self):
        r=Runtime();r.row['Id']='other'
        with self.assertRaises(ContractError):self.actor(r).assert_closed_isolation()
    def test_selected_pvc_must_be_exact_only_writable_data(self):
        r=Runtime();r.row['Mounts'][0]['Source']='/other'
        with self.assertRaises(ContractError):self.actor(r).assert_closed_isolation()
    def test_fence_revalidated_every_time(self):
        r=Runtime();calls=[];a=self.actor(r,lambda:calls.append('fence'))
        a.assert_closed_isolation();a.assert_closed_isolation();self.assertEqual(calls,['fence','fence'])
    def test_arbitrary_api_never_reaches_runtime(self):
        r=Runtime()
        with self.assertRaises(ContractError):self.actor(r).mutate('$JS.API.STREAM.DELETE.message_events',{})
        self.assertEqual(r.calls,[])
    def test_info_removes_only_pinned_response_metadata(self):
        a=self.actor(Runtime()); config={'name':'chat_events','metadata':{'_nats.ver':'2.12.12','_nats.level':'3','_nats.req.level':'1','owner':'keep'}}
        a._request=lambda *args,**kwargs:{'config':config}
        self.assertEqual(a.info_config('chat_events'),{'name':'chat_events','metadata':{'_nats.req.level':'1','owner':'keep'}})
        self.assertIn('_nats.ver',config['metadata'])
    def test_info_refuses_unbound_server_metadata(self):
        for metadata in ({'_nats.ver':'2.12.11','_nats.level':'3'},{'_nats.ver':'2.12.12','_nats.level':'4'},{'_nats.ver':'2.12.12'}):
            a=self.actor(Runtime());a._request=lambda *args,**kwargs:{'config':{'name':'chat_events','metadata':metadata}}
            with self.assertRaises(ContractError):a.info_config('chat_events')
    def test_info_and_create_surfaces_match_all_exact_plan_consumers(self):
        expected_streams={stream for stream,_ in DECLARATIONS}
        expected_consumers={(stream,config['durable_name']) for stream,config in DECLARATIONS}
        self.assertEqual({key for key in INFO if '/' not in key},expected_streams)
        self.assertEqual({tuple(key.split('/',1)) for key in INFO if '/' in key},expected_consumers)
        self.assertEqual({(api.split('.INFO.',1)[1].replace('.','/',1)) for api in INFO.values() if '.CONSUMER.INFO.' in api},
            {stream+'/'+durable for stream,durable in expected_consumers})
        self.assertEqual({tuple(value.split('/',1)) for value in WRITE.values() if '/' in value},expected_consumers)
        self.assertEqual({api:value for api,value in WRITE.items() if api.startswith('$JS.API.STREAM.UPDATE.')},
            {'$JS.API.STREAM.UPDATE.chat_events':'chat_events',
             '$JS.API.STREAM.UPDATE.social_events':'social_events',
             '$JS.API.STREAM.UPDATE.role_events':'role_events'})
        for stream,config in DECLARATIONS:
            durable=config['durable_name'];api='$JS.API.CONSUMER.CREATE.'+stream+'.'+durable
            actor=self.actor(Runtime());calls=[];actor._request=lambda *args,**kwargs:calls.append(args)
            actor.mutate(api,{'stream_name':stream,'action':'create','config':config})
            self.assertEqual(len(calls),1)
            with self.subTest(stream=stream),self.assertRaises(ContractError):
                actor.mutate(api,{'stream_name':stream,'action':'create','config':dict(config,name='other')})
            self.assertEqual(len(calls),1)
    def test_consumer_info_identity_is_checked_for_each_declared_stream(self):
        for stream,config in DECLARATIONS:
            durable=config['durable_name'];actor=self.actor(Runtime())
            actor._request=lambda *args,**kwargs:{'stream_name':stream,'name':durable,'config':dict(config)}
            self.assertEqual(actor.info_config(stream+'/'+durable),config)
            actor._request=lambda *args,**kwargs:{'stream_name':'wrong','name':durable,'config':dict(config)}
            with self.subTest(stream=stream),self.assertRaises(ContractError):actor.info_config(stream+'/'+durable)
    def test_all_declared_missing_consumers_accept_only_exact_10014(self):
        with patch.dict(sys.modules,{'docker_runtime':SimpleNamespace(BOX_IMAGE='pinned-box')}):
            for stream,config in DECLARATIONS:
                durable=config['durable_name'];obj=stream+'/'+durable;api=INFO[obj]
                runtime=RequestRuntime({'error':{'err_code':10014}})
                actor=self.actor(runtime)
                self.assertIsNone(actor.info_config(obj))
                self.assertEqual(runtime.created[0][1][-2],api)
                for code in (10037,10059,99999):
                    runtime=RequestRuntime({'error':{'err_code':code}})
                    with self.subTest(stream=stream,code=code),self.assertRaises(ContractError):
                        self.actor(runtime).info_config(obj)
            # A stream INFO error cannot borrow the missing-consumer exception.
            runtime=RequestRuntime({'error':{'err_code':10014}})
            with self.assertRaises(ContractError):
                self.actor(runtime)._request(INFO['chat_events'],None,missing=True)
    def test_existing_declared_consumers_are_read_as_configs(self):
        with patch.dict(sys.modules,{'docker_runtime':SimpleNamespace(BOX_IMAGE='pinned-box')}):
            for stream,config in DECLARATIONS:
                durable=config['durable_name'];obj=stream+'/'+durable
                response={'stream_name':stream,'name':durable,'config':dict(config)}
                runtime=RequestRuntime(response)
                self.assertEqual(self.actor(runtime).info_config(obj),config)
    def test_unknown_info_object_remains_outside_exact_actor_allowlist(self):
        actor=self.actor(Runtime())
        with self.assertRaises(ContractError):actor.info_config('rollout_extra')

if __name__=='__main__':unittest.main()
