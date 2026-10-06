import json
import unittest
from pathlib import Path
from nats_contract_actor import Actor
from nats_contract_plan import ContractError
class Runtime:
    base=Path('/root-owned');owned={}
    def __init__(self):self.row={'Id':'owned-id','State':{'Running':True},'HostConfig':{'NetworkMode':'none','PortBindings':{}},'Mounts':[{'Type':'bind','Source':str(Path('/selected-pvc')),'Destination':'/data','RW':True}]};self.calls=[]
    def inspect(self,name):return self.row
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
if __name__=='__main__':unittest.main()
