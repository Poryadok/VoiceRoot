import copy
from pathlib import Path
import unittest
from unittest import mock

import full_cohort_supervisor as supervisor
import media_backup_phase_test as fixtures
import media_operation_journal
import resource_fencing


class SupervisorTest(unittest.TestCase):
    def setUp(self):
        self.base=fixtures.PhaseTest(); self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.root=Path(self.base.temp.name)/'operations'; self.root.mkdir(mode=0o700)
        self.operation=self.root/self.base.context['operation_id']; self.operation.mkdir(mode=0o700)
        self.root_journal=media_operation_journal.Journal(self.operation/'journal',
            {'resource_context':self.base.context,'namespace_uid':'ns'})
        self.addCleanup(self.root_journal.close)
        base=self.base
        class Client:
            def __init__(inner,journal): inner.journal=journal
        class Ledger:
            def __init__(inner,client): inner.client=client
            def read(inner): return base.ledger.read()
            def write(inner,revision,row): return base.ledger.write(revision,row)
        class Check:
            endpoint='http://fixture'
            def __init__(inner,context): inner.context=context
            def __call__(inner): return copy.deepcopy(base.card)
            def mark_uncertain(inner,reason): pass
        self.client=Client(self.root_journal); check=Check(base.context)
        self.fence=resource_fencing.Fence(base.context,check,Ledger(self.client),check.mark_uncertain)
        for patcher in (mock.patch.object(supervisor.frontend_runtime_executor,'KubectlTransport',Client),
                        mock.patch.object(resource_fencing,'KubeLedger',Ledger),
                        mock.patch.object(resource_fencing,'ResourceCheck',Check),
                        mock.patch.object(supervisor.media_backup_upload_runner,'ROOT',self.root),
                        mock.patch.object(supervisor.media_backup_upload_runner,'TT','http://fixture'),
                        mock.patch.object(supervisor.media_operation_guard,'verify')):
            patcher.start(); self.addCleanup(patcher.stop)

    def test_prepare_calls_owned_phases_and_never_replays(self):
        calls=[]
        nodes=[{'name':'node','uid':'uid','hostname':'node','architecture':'amd64'}]
        with mock.patch.object(supervisor.full_cohort_package_phase.PackagePhase,'run',side_effect=lambda *a:calls.append('package')), \
             mock.patch.object(supervisor.full_cohort_inventory_phase.InventoryPhase,'run',side_effect=lambda *a:calls.append('inventory')), \
             mock.patch.object(supervisor.full_cohort_affected_plan.FullCatalogPhase,'run',side_effect=lambda *a:calls.append('catalog')), \
             mock.patch.object(supervisor.full_cohort_affected_plan,'build',return_value={'plan':'fixture'}):
            subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
            self.assertEqual(subject.prepare('token','actor',1,1,nodes),{'plan':'fixture'})
            self.assertEqual(calls,['package','inventory','catalog'])
            self.assertEqual(len(subject.journals),3)
            for journal in subject.journals: self.addCleanup(journal.close)
            self.assertIs(subject.package.journal,subject.journals[0])
            self.assertIs(subject.inventory.package,subject.package)
            self.assertIs(subject.catalog.inventory,subject.inventory)
            with self.assertRaises(supervisor.SupervisorError): subject.prepare('token','actor',1,1,nodes)

    def test_replaced_ledger_client_refuses_before_phase(self):
        self.fence.ledger.client=object()
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')

    def test_wrong_journal_path_and_replaced_check_are_refused(self):
        other=media_operation_journal.Journal(self.operation/'other',self.root_journal.intent)
        self.addCleanup(other.close)
        self.client.journal=other
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(other,self.fence,self.client,self.base.lock,'ns')
        self.client.journal=self.root_journal
        subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
        self.fence.check=type(self.fence.check)(self.base.context)
        with self.assertRaises(supervisor.SupervisorError): subject._guard()

    def test_mutated_root_journal_state_is_refused(self):
        subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
        self.root_journal.state['intent']['namespace_uid']='foreign'
        with self.assertRaises(supervisor.SupervisorError): subject._guard()


if __name__=='__main__': unittest.main()
