import copy
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixtures
import media_operation_journal as journal
import realtime_config_transition_deployment as deployment


class Client:
    def __init__(self,source): self.source=copy.deepcopy(source)
    def call(self,verb,kind,name,*args): return copy.deepcopy(self.source)


class Cut:
    def _guard(self): pass
    def _guards(self): pass
    def _pods_drained(self): return True
    def _owned(self,row): return self.fence.owned(row)
    def verify_drained(self): pass
    def accept_config_successor(self,journal,service): self.accepted.append(service)
    def _patch(self,kind,name,patch):
        if patch[2]['value']!=self.client.source['spec']: raise ValueError()
        obj=self.client.source; obj['spec']=copy.deepcopy(patch[3]['value'])
        obj['metadata']['resourceVersion']='8'; obj['metadata']['generation']=3
        obj['status']={'observedGeneration':3,'replicas':0,'readyReplicas':0}
        return copy.deepcopy(obj)


class DeploymentTest(unittest.TestCase):
    def setUp(self):
        self.base=fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        self.source={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-chat','namespace':'voice-dev',
            'uid':'dep','resourceVersion':'7','generation':2},'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'chat','image':'old'}]}}}}
        self.target=copy.deepcopy(self.source['spec']); self.target['template']['spec']['containers'][0]['env']=[{'name':'fixed','value':'x'}]
        self.patch=[{'op':'test','path':'/metadata/uid','value':'dep'},{'op':'test','path':'/metadata/resourceVersion','value':'7'},
                    {'op':'test','path':'/spec','value':copy.deepcopy(self.source['spec'])},{'op':'replace','path':'/spec','value':copy.deepcopy(self.target)}]
        self.cut=Cut(); self.cut.fence=self.base.fence; self.cut.lock=self.base.lock; self.cut.namespace_uid='ns'; self.cut.invocation='b'*32
        self.cut.client=Client(self.source); self.cut.journal=self.base.journal; self.cut.accepted=[]
        intent={'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns','backend_config_deployments':{'services':['chat']}}
        self.journal=journal.Journal(Path(self.base.temp.name)/'p3-config',intent); self.addCleanup(self.journal.close)
        self.cut.config_journal=self.journal; self.cut.config_snapshot={'services':[]}
        self.patches=[patch.object(deployment,'_plan',side_effect=lambda *a:(copy.deepcopy(self.source),copy.deepcopy(self.target),copy.deepcopy(self.patch))),
                      patch.object(deployment.media_operation_guard,'verify')]
        for p in self.patches: p.start(); self.addCleanup(p.stop)

    def cleanup(self,base):
        if not base.doCleanups(): raise RuntimeError('cleanup failed')

    def test_apply_and_historical_verifier(self):
        receipt=deployment.apply(self.cut,self.journal,'chat'); self.assertEqual(self.cut.accepted,['chat'])
        self.cut.verify_drained=lambda:(_ for _ in ()).throw(AssertionError())
        self.assertEqual(deployment.verify_deployment_successor(self.cut,self.journal,'chat')['receipt'],receipt)
        self.base.ledger.row['backend_config_effects']['backend-config-deployment-voice-chat']['nonce']='f'*32
        with self.assertRaises(deployment.DeploymentError): deployment.verify_deployment_successor(self.cut,self.journal,'chat')

    def test_fixed9_dependency_requires_same_cut_and_token(self):
        import realtime_jwks_publisher as publisher
        class Session: pass
        class Migration: pass
        class Publisher: pass
        bound=deployment.authority.BoundFixed9SpaceAuthority(self.cut,object(),object(),object(),object(),'token')
        with patch.object(deployment.writer_cut,'WriterCut',Cut), \
             patch.object(deployment.realtime_p3_create_only,'Session',Session), \
             patch.object(deployment.realtime_migration_job,'Migration',Migration), \
             patch.object(publisher,'PublisherPhase',Publisher):
            value=deployment.Dependencies(self.cut,self.journal,Session(),Migration(),bound,Publisher(),'/source','token')
            self.assertIs(value.space_phase,bound)
            with self.assertRaises(ValueError):deployment.Dependencies(self.cut,self.journal,Session(),Migration(),bound,Publisher(),'/source','wrong')
            foreign=deployment.authority.BoundFixed9SpaceAuthority(Cut(),object(),object(),object(),object(),'token')
            with self.assertRaises(ValueError):deployment.Dependencies(self.cut,self.journal,Session(),Migration(),foreign,Publisher(),'/source','token')

    def test_out_of_order_and_lost_response_do_not_replay(self):
        self.cut.config_snapshot['services']=['foreign']
        with self.assertRaises(deployment.DeploymentError): deployment.apply(self.cut,self.journal,'chat')
        self.assertEqual(self.client_generation(),2)
        self.cut.config_snapshot['services']=[]
        self.cut._patch=lambda *a:(_ for _ in ()).throw(OSError('private-sentinel'))
        with self.assertRaises(deployment.DeploymentError) as error: deployment.apply(self.cut,self.journal,'chat')
        self.assertNotIn('private-sentinel',str(error.exception)); self.assertEqual(self.base.ledger.row['status'],'uncertain')
        with self.assertRaises(deployment.DeploymentError): deployment.apply(self.cut,self.journal,'chat')

    def test_prepatch_authority_change_prevents_effect(self):
        calls=0
        def plan(*args):
            nonlocal calls
            calls+=1
            if calls==2: raise ValueError('source changed')
            return copy.deepcopy(self.source),copy.deepcopy(self.target),copy.deepcopy(self.patch)
        with patch.object(deployment,'_plan',side_effect=plan):
            with self.assertRaises(deployment.DeploymentError): deployment.apply(self.cut,self.journal,'chat')
        self.assertEqual(calls,2); self.assertEqual(self.client_generation(),2)
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_resumed_route_requires_source_proof_and_exact_database_ref(self):
        name='voice-realtime-db-'+'a'*32
        target={'template':{'spec':{'containers':[{'name':'realtime','env':[{'name':'DATABASE_URL',
            'valueFrom':{'secretKeyRef':{'name':name,'key':'DATABASE_URL'}}}]}]}}}
        proof={'target_spec':target,'receipt':{'uid':'dep','spec_sha256':'b'*64}}
        migration=SimpleNamespace(intent={'realtime_migration':{'credential':{'name':name}}},journal=SimpleNamespace(state={'steps':{}}))
        session=SimpleNamespace(intent={'fixed':True},journal=SimpleNamespace(state={'steps':{}}))
        self.journal._realtime_config_dependencies=SimpleNamespace(session=session,migration=migration,source_root='/source')
        self.journal.state['steps']['backend-config-deployment-voice-realtime']={'status':'done'}
        source={'migration':{'execution':{'database':'realtime_db','role':'realtime','database_oid':42}}}
        self.cut.verify_drained=lambda:(_ for _ in ()).throw(AssertionError('must not require drain'))
        with patch.object(deployment,'verify_deployment_successor',return_value=proof), \
             patch.object(deployment.inputs,'observe',return_value=source):
            route=deployment.read_realtime_route(self.cut,self.journal)
            self.assertEqual((route['database'],route['role'],route['database_oid']),('realtime_db','realtime',42))
            source['migration']['execution']['database_oid']=True
            with self.assertRaises(deployment.DeploymentError): deployment.read_realtime_route(self.cut,self.journal)
            source['migration']['execution']['database_oid']=42
            def drift(*args):
                self.journal.state['steps']['backend-config-deployment-voice-realtime']['status']='foreign'
                return source
            with patch.object(deployment.inputs,'observe',side_effect=drift):
                with self.assertRaises(deployment.DeploymentError): deployment.read_realtime_route(self.cut,self.journal)

    def client_generation(self): return self.cut.client.source['metadata']['generation']


if __name__=='__main__': unittest.main()
