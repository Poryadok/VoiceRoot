import copy
from pathlib import Path
import unittest
from unittest.mock import patch

import full_cohort_inventory_phase as inventory
import full_cohort_schema_metadata as metadata
import media_backup_phase_test as fixture
import media_operation_journal as journal


class Client:
    def __init__(self):
        self.deployments=[]; self.replicasets=[]; self.pods=[]
        for name, container in metadata.CONTAINERS.items():
            deployment='voice-'+name; uid='dep-'+name; rs='rs-'+name
            self.deployments.append({'kind':'Deployment','metadata':{'name':deployment,'namespace':'voice-dev',
                'uid':uid,'resourceVersion':'1','generation':1},'spec':{'replicas':1,'selector':{'matchLabels':{'app':name}},'template':{'spec':{'containers':[{'name':container}]}}},
                'status':{'observedGeneration':1,'replicas':1,'updatedReplicas':1,'readyReplicas':1}})
            self.replicasets.append({'metadata':{'name':'rs-'+name,'namespace':'voice-dev','uid':rs,
                'ownerReferences':[{'kind':'Deployment','uid':uid,'controller':True}]}})
            self.pods.append({'metadata':{'name':'pod-'+name,'namespace':'voice-dev','uid':'pod-'+name,'labels':{'app':name},
                'ownerReferences':[{'kind':'ReplicaSet','uid':rs,'controller':True}]},
                'spec':{'containers':[{'name':container}]},
                'status':{'phase':'Running','containerStatuses':[{'name':container,'ready':True,
                    'state':{'running':{}},
                    'containerID':'containerd://'+('a'*64),'imageID':'containerd://sha256:'+('b'*64),'restartCount':0}]}})

    def call(self,*args):
        return {'items':copy.deepcopy(getattr(self,args[1]))}


class InventoryTest(unittest.TestCase):
    def test_exact_complete_owner_chain(self):
        pods, controllers, projection = inventory.inventory(Client())
        self.assertEqual(set(pods),set(metadata.CONTAINERS))
        self.assertEqual(len(controllers),23)
        self.assertEqual(pods['auth'],['pod-auth'])
        self.assertEqual(len(projection),23)

    def test_missing_or_foreign_or_duplicate_pod_refused(self):
        for fault in ('missing','foreign','duplicate','unready','sidecar','labels'):
            client=Client()
            if fault=='missing': client.pods.pop()
            if fault=='foreign': client.pods[-1]['metadata']['ownerReferences'][0]['uid']='foreign'
            if fault=='duplicate': client.pods.append(copy.deepcopy(client.pods[-1]))
            if fault=='unready': client.pods[-1]['status']['containerStatuses'][0]['ready']=False
            if fault=='sidecar': client.pods[-1]['spec']['containers'].append({'name':'sidecar'})
            if fault=='labels': client.pods[-1]['metadata']['labels']={}
            with self.subTest(fault=fault), self.assertRaises(inventory.InventoryError): inventory.inventory(client)


class PhaseTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.journal = journal.Journal(Path(self.base.temp.name)/'inventory-journal',
            {'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns'})
        self.addCleanup(self.journal.close)
        class Kube(fixture.Client):
            def __init__(self, lock): super().__init__(lock); self.fixture=Client()
            def call(self,*args):
                if args[1] in ('deployments','replicasets','pods'): return self.fixture.call(*args)
                return super().call(*args)
        self.client=Kube(self.base.lock); self.client.journal=self.journal; self.base.ledger.client=self.client
        class Package:
            def __init__(other):
                other.fence=self.base.fence; other.lock=self.base.lock; other.namespace_uid='ns'
            def verify_current(other, token): return {'package_sha256':'a'*64,'source_sha':self.base.context['commit']}
        self.package=Package()
        self.client_patch=patch.object(inventory.frontend_runtime_executor,'KubectlTransport',Kube)
        self.package_patch=patch.object(inventory.package_phase,'PackagePhase',Package)
        self.client_patch.start(); self.package_patch.start()
        self.addCleanup(self.client_patch.stop); self.addCleanup(self.package_patch.stop)
        self.phase=inventory.InventoryPhase(self.journal,self.base.fence,self.client,self.base.lock,'ns',self.package)

    def test_holds_are_recorded_honestly_and_no_replay(self):
        self.client.fixture.deployments[0]['spec']['template']['spec']['containers'][0]['env']=[{'name':'TOKEN','value':'PRIVATE_SENTINEL'}]
        report={'source_sha':self.base.context['commit'],'workloads':{'auth':[{'status':'hold','reason':'auth_unknown'}]},
                'status':'HOLD','startup_qualified':False}
        with patch.object(metadata,'catalog_from_source',return_value={}), \
             patch.object(metadata,'inspect_all',return_value=report), patch.object(self.phase,'_admin_role',return_value='voice'):
            result=self.phase.run('fixture-token')
            self.assertEqual(result['database_observations'],report)
            self.assertNotIn('PRIVATE_SENTINEL',str(result))
            self.assertNotIn('PRIVATE_SENTINEL',str(self.journal.state))
            self.assertEqual(self.base.ledger.row['full_cohort_inventory']['status'],'done')
            self.assertEqual(self.phase.verify_current('fixture-token'),result)
            self.client.fixture.pods[0]['status']['containerStatuses'][0]['restartCount']=1
            self.assertEqual(self.phase.verify_receipt('fixture-token'),result)
            with self.assertRaises(inventory.InventoryError): self.phase.verify_current('fixture-token')
            with self.assertRaises(inventory.InventoryError): self.phase.run('fixture-token')

    def test_inventory_drift_poisoned_before_terminal(self):
        def inspect(*args):
            self.client.fixture.deployments[0]['spec']['replicas']=2
            return {'status':'HOLD'}
        with patch.object(metadata,'catalog_from_source',return_value={}), \
             patch.object(metadata,'inspect_all',side_effect=inspect), patch.object(self.phase,'_admin_role',return_value='voice'):
            with self.assertRaises(inventory.InventoryError): self.phase.run('fixture-token')
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_verify_refuses_in_place_receipt_mutation(self):
        report={'status':'HOLD'}
        with patch.object(metadata,'catalog_from_source',return_value={}), \
             patch.object(metadata,'inspect_all',return_value=report), patch.object(self.phase,'_admin_role',return_value='voice'):
            self.phase.run('fixture-token')
            def mutate(*args):
                self.journal.state['steps'][inventory.STEP]['receipt']['scope']='tampered'
                return report
            with patch.object(metadata,'inspect_all',side_effect=mutate):
                with self.assertRaises(inventory.InventoryError): self.phase.verify_current('fixture-token')


if __name__=='__main__': unittest.main()
