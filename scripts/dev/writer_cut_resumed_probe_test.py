import copy
import json
import unittest
from unittest.mock import patch

import writer_cut_resumed_probe as probe


class TestResume(unittest.TestCase):
    def setUp(self):
        self.engine={'id':'fixture'}
        self.scope={'engine':self.engine,'databases':['user_db'],'database_oids':{'user_db':5},'server_start':'start'}
        self.route={'binding':{'state':'resolved','database':'user_db','role':'voice','binding':{'pod_uid':'pod','container_id':'containerd://fixture','restart_count':0}},
                    'metadata':{'database':'user_db','engine':self.engine,'session':{'database_oid':5,'server_start':'start','role':'admin'}},'secondary':[]}
        self.receipt={'database_observations':{'workloads':{'user':[{'status':'observed','result':copy.deepcopy(self.route)}]}}}
        self.proof={'controllers':[{'name':'voice-user','kind':'Deployment','uid':'dep',
                     'spec':{'template':{'spec':{'containers':[{'name':'user','image':'repo@sha256:index'}]}}}}]}
        class Client:
            def __init__(inner): inner.calls=0; inner.fault=None
            def call(inner,*args):
                if args[1]=='replicasets':
                    return {'items':[{'metadata':{'uid':'rs','ownerReferences':[{'kind':'Deployment','uid':'dep','controller':True}]}}]}
                inner.calls+=1
                value={'metadata':{'name':'voice-user-abc','namespace':'voice-dev','uid':'pod',
                    'ownerReferences':[{'kind':'ReplicaSet','uid':'rs','controller':True}]},'spec':{'nodeName':'node','containers':[{'name':'user','image':'repo@sha256:index'}]},
                    'status':{'phase':'Running','podIP':'10.0.0.2','containerStatuses':[
                        {'name':'user','containerID':'containerd://fixture','imageID':'sha256:'+'a'*64,'restartCount':0}]}}
                if inner.fault=='sidecar': value['spec']['containers'].append({'name':'unknown'})
                if inner.fault=='init': value['spec']['initContainers']=[{'name':'restartable','restartPolicy':'Always'}]
                if inner.fault=='restart' and inner.calls>1: value['status']['containerStatuses'][0]['restartCount']=1
                if inner.fault=='app-image': value['spec']['containers'][0]['image']='other'
                if inner.fault=='runtime-image': value['status']['containerStatuses'][0]['imageID']='sha256:'+'b'*64
                if inner.fault=='node': value['spec']['nodeName']='other'
                if inner.fault=='pending': value['status']={'phase':'Pending','containerStatuses':[]}
                if inner.fault=='pending-to-running' and inner.calls==1:
                    value['status']={'phase':'Pending','containerStatuses':[]}
                if inner.fault=='pending-running':
                    value['status']={'phase':'Pending','containerStatuses':[{'name':'user','state':{'running':{}}}]}
                return {'items':[value]}
        class Cut:
            def __init__(inner):
                ledger=type('Ledger',(),{'read':lambda inner:({'status':'active'},('uid','rv'))})()
                inner.admission={'session_scope':self.scope}; inner.fence=type('Fence',(),{'context':{'commit':'a'*40},'ledger':ledger})()
                inner.lock={'uid':'lock'}; inner.namespace_uid='ns'; inner.client=Client()
            def _guard(inner): pass
            def current_resumed_proof(inner): return copy.deepcopy(self.proof)
        class Phase:
            def __init__(inner):
                inner.fence=self.cut.fence; inner.lock=self.cut.lock; inner.namespace_uid='ns'
                inner.package=type('Package',(),{'verify_current':lambda obj,token:{'images':{'voice-user/user':'repo@sha256:index'},
                    'platform_digests':{'voice-user/user':'sha256:'+'a'*64},'nodes':[{'name':'node'}]}})()
            def verify_receipt(inner,token): return copy.deepcopy(self.receipt)
            def _admin_role(inner): return 'admin'
        self.cut=Cut(); self.phase=Phase()
        self.patches=[patch.object(probe.writer_cut,'WriterCut',Cut),patch.object(probe.inventory,'InventoryPhase',Phase),
            patch.object(probe.pg,'_validate_scope',side_effect=lambda x,g:copy.deepcopy(x)),
            patch.object(probe.metadata,'catalog_from_source',return_value={}),
            patch.object(probe.metadata,'inspect',side_effect=lambda *a:copy.deepcopy(self.route)),
            patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:copy.deepcopy(self.engine))]
        for p in self.patches: p.start(); self.addCleanup(p.stop)

    def run_session(self,addr='10.0.0.2',prepared=0):
        value={'server_start':'start','can_read':True,'matched':1,'prepared':prepared,'sessions':[
            {'pid':9,'backend_start':'instant','database':'user_db','role':'voice','client_addr':addr,'transaction':True}]}
        with patch.object(probe.metadata,'linux_psql',return_value=json.dumps(value)):
            return probe.inspect_resumed(self.cut,self.phase,'token')

    def test_owned_session_and_unknown_session(self):
        self.assertEqual(self.run_session()['active_sessions'],0)
        self.assertEqual(len(self.run_session()['attributed_sessions']),1)
        self.assertEqual(self.run_session('10.0.0.3')['active_sessions'],1)
        self.assertEqual(self.run_session(prepared=1)['active_transactions'],1)

    def test_route_drift_and_auth_successor_refused(self):
        self.route['binding']['role']='other'
        with self.assertRaises(probe.pg.ProbeError): self.run_session()

    def test_shared_network_and_restart_refused(self):
        for fault in ('sidecar','init','restart'):
            self.cut.client.fault=fault; self.cut.client.calls=0
            with self.subTest(fault=fault), self.assertRaises(probe.pg.ProbeError): self.run_session()

    def test_image_node_and_package_drift_refused(self):
        for fault in ('app-image','runtime-image','node'):
            self.cut.client.fault=fault; self.cut.client.calls=0
            with self.subTest(fault=fault), self.assertRaises(probe.pg.ProbeError): self.run_session()
        self.cut.client.fault=None
        self.proof['controllers'][0]['spec']['template']['spec']['containers'][0]['image']='other'
        with self.assertRaises(probe.pg.ProbeError): self.run_session()
        self.proof['controllers'][0]['spec']['template']['spec']['containers'][0]['image']='repo@sha256:index'
        first=self.phase.package.verify_current('token')
        second=copy.deepcopy(first); second['nodes'][0]['name']='other'
        with patch.object(self.phase.package,'verify_current',side_effect=[first,second]):
            with self.assertRaises(probe.pg.ProbeError): self.run_session()

    def test_pending_pod_without_running_container_does_not_require_ip(self):
        self.cut.client.fault='pending'
        value={'server_start':'start','can_read':True,'matched':1,'prepared':0,'sessions':[]}
        with patch.object(probe.metadata,'linux_psql',return_value=json.dumps(value)):
            self.assertEqual(probe.inspect_resumed(self.cut,self.phase,'token')['active_sessions'],0)
            for fault in ('pending-to-running','pending-running'):
                self.cut.client.fault=fault; self.cut.client.calls=0
                with self.subTest(fault=fault), self.assertRaises(probe.pg.ProbeError):
                    probe.inspect_resumed(self.cut,self.phase,'token')

    def test_known_nats_leaf_only(self):
        leaf={'name':'nats-leaf','image':probe.NATS_IMAGE,'args':['-c','/etc/nats/leaf.conf','-a','127.0.0.1','-m','8222'],
              'volumeMounts':[{'name':'leaf','mountPath':'/etc/nats/leaf.conf','subPath':'leaf.conf','readOnly':True}]}
        spec={'containers':[{'name':'user'},leaf],'volumes':[{'name':'leaf','configMap':{'name':'voice-nats-leaf-config'}}]}
        self.cut.admission['controllers']=[{'kind':'Deployment','metadata':{'name':'voice-user'},'spec':{'template':{'spec':copy.deepcopy(spec)}}}]
        self.receipt['projection']={'user':{'pods':[{'containers':[{'name':'nats-leaf','image_id':'nats-runtime'}]}]}}
        controller={'name':'voice-user','spec':{'template':{'spec':copy.deepcopy(spec)}}}
        pod={'metadata':{'name':'voice-user-abc'},'spec':copy.deepcopy(spec),'status':{'containerStatuses':[{'name':'nats-leaf','imageID':'nats-runtime',
             'containerID':'nats-container','restartCount':0,'state':{'running':{}}}]}}
        self.cut.client.call=lambda *args: {'metadata':{'namespace':'voice-dev','uid':'cm','resourceVersion':'1'},'data':{'leaf.conf':'expected'}}
        with patch.object(probe.metadata,'linux_private_bytes',return_value=b'expected'):
            self.assertEqual(probe._leaf_identity(self.cut,self.receipt,controller,pod)[-2:],('cm','1'))
            pod['spec']['containers'][1]['args']=['different']
            with self.assertRaises(probe.pg.ProbeError): probe._leaf_identity(self.cut,self.receipt,controller,pod)
            pod['spec']=copy.deepcopy(spec); pod['spec']['volumes'][0]['configMap']['name']='other'
            with self.assertRaises(probe.pg.ProbeError): probe._leaf_identity(self.cut,self.receipt,controller,pod)
        pod['spec']=copy.deepcopy(spec)
        with patch.object(probe.metadata,'linux_private_bytes',return_value=b'old'):
            with self.assertRaises(probe.pg.ProbeError): probe._leaf_identity(self.cut,self.receipt,controller,pod)
        self.proof['controllers'][0]['name']='voice-auth'
        with self.assertRaises(probe.pg.ProbeError): self.run_session()


if __name__=='__main__': unittest.main()
