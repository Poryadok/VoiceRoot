import copy
from pathlib import Path
import unittest
from unittest import mock

import media_backup_phase_test as fixture
import media_operation_journal
import realtime_absent_phase as absent


class AbsentTest(unittest.TestCase):
    def setUp(self):
        self.base=fixture.PhaseTest()
        self.addCleanup(lambda:self.assertTrue(self.base.doCleanups()))
        self.base.setUp()
        self.journal=media_operation_journal.Journal(Path(self.base.temp.name)/'absent',
            {'resource_context':self.base.context,'namespace_uid':'ns'})
        self.addCleanup(self.journal.close)
        class Client(fixture.Client): pass
        self.client=Client(self.base.lock); self.client.journal=self.journal; self.base.ledger.client=self.client
        self.pg_state={'role':None,'database':None}; self.runtime=None
        self.snapshot={'namespace':'voice-dev','namespace_uid':'ns','deployment':'voice-realtime',
            'deployment_uid':'deploy','deployment_rv':'2','pod_name':'voice-realtime-abc','pod_uid':'pod',
            'pod_rv':'4','container_id':'cid','restart_count':0,'container_name':'realtime','env':[],'envFrom':[]}
        projection={'uid':'deploy','generation':1,'spec_sha256':'a'*64,'pods':[{'name':'voice-realtime-abc',
            'uid':'pod','containers':[{'name':'realtime','container_id':'cid','restart_count':0}]}]}
        observed={'source_sha':self.base.context['commit'],'pods':{'realtime':['voice-realtime-abc']},
                  'projection':{'realtime':projection}}
        catalog={'engine':{'id':'pg'},'all_databases':{'postgres':1},'scope':'all-databases-connectable-backup'}
        base=self.base
        class Inventory:
            fence,lock,namespace_uid=base.fence,base.lock,'ns'
            def verify_current(inner,token):return copy.deepcopy(observed)
        class Catalog:
            fence,lock,namespace_uid=base.fence,base.lock,'ns'
            def __init__(inner,inv):inner.inventory=inv
            def verify_current(inner,token):return copy.deepcopy(catalog)
            def verify_receipt(inner,token):return copy.deepcopy(catalog)
        subject=self
        class PG:
            def __init__(inner,guard):inner.guard=guard
            def snapshot(inner):return {'id':'pg'}
            def catalog(inner):return copy.deepcopy(subject.pg_state)
        inv=Inventory(); cat=Catalog(inv)
        for patcher in (mock.patch.object(absent.frontend_runtime_executor,'KubectlTransport',Client),
                mock.patch.object(absent.inventory,'InventoryPhase',Inventory),
                mock.patch.object(absent.affected,'FullCatalogPhase',Catalog),
                mock.patch.object(absent.realtime_db_provision,'KubePostgres',PG),
                mock.patch.object(absent.schema,'kube_workload_snapshot',return_value=lambda:copy.deepcopy(subject.snapshot)),
                mock.patch.object(absent.schema,'kube_runtime_reader',return_value=lambda variable:subject.runtime),
                mock.patch.object(absent.schema,'kube_key_reader',return_value=lambda *args:None)):
            patcher.start(); self.addCleanup(patcher.stop)
        self.phase=absent.RealtimeAbsentPhase(self.journal,base.fence,self.client,base.lock,'ns',inv,cat)

    def test_owned_absence_and_runtime_or_database_drift(self):
        receipt=self.phase.run('token')
        self.assertEqual(receipt['scope'],'realtime-create-only-baseline')
        self.assertEqual(self.phase.verify_current('token'),receipt)
        self.runtime='postgres://unexpected'
        with self.assertRaises(absent.AbsentError): self.phase.verify_current('token')
        self.runtime=None; self.pg_state['database']={'oid':3}
        with self.assertRaises(absent.AbsentError): self.phase.verify_current('token')

    def test_ledger_drift_rejected(self):
        self.phase.run('token')
        self.base.ledger.row['realtime_absent_baseline']['nonce']='changed'
        with self.assertRaises(absent.AbsentError): self.phase.verify_receipt('token')

    def test_guard_loss_after_reservation_never_returns_absence(self):
        guard=self.phase._guard; calls=0
        def fail():
            nonlocal calls
            calls+=1
            if calls>=3: raise absent.schema.GuardLost('operation_guard_failed')
            guard()
        with mock.patch.object(self.phase,'_guard',side_effect=fail):
            with self.assertRaises(absent.AbsentError): self.phase.run('token')
        self.assertEqual(self.base.ledger.row['status'],'uncertain')
        self.assertNotEqual(self.journal.state['steps'][absent.STEP]['status'],'done')

if __name__=='__main__':unittest.main()
