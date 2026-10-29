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
            {'resource_context':self.base.context,'namespace_uid':'ns','cohort_route_mode':'full-cohort-v1'})
        self.addCleanup(self.root_journal.close)
        descriptor={'snapshot_sha256':'d'*64}
        self.root_journal.state['steps']['full-cohort-image-only-baseline']={'descriptor':descriptor}
        self.root_journal.state['steps']['image-only-admission']={'status':'done','descriptor':descriptor,
            'old_revision':['ledger','0'],'new_revision':['ledger','1']}
        self.base.ledger.row['image_only_baseline']=copy.deepcopy(descriptor)
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
             mock.patch.object(supervisor.full_cohort_image_only_baseline,'ImageOnlyPhase') as baseline, \
             mock.patch.object(supervisor.full_cohort_inventory_phase.InventoryPhase,'run',side_effect=lambda *a:calls.append('inventory')), \
             mock.patch.object(supervisor.full_cohort_inventory_phase.InventoryPhase,'verify_receipt',return_value={
                 'database_observations':{'workloads':{'realtime':[{'status':'observed'}]}}}), \
             mock.patch.object(supervisor.full_cohort_namespace_census.NamespaceCensus,'run',side_effect=lambda *a:calls.append('census')), \
             mock.patch.object(supervisor.full_cohort_affected_plan.FullCatalogPhase,'run',side_effect=lambda *a:calls.append('catalog')), \
             mock.patch.object(supervisor.full_cohort_affected_plan,'build',return_value={'plan':'fixture'}):
            subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
            self.assertEqual(subject.prepare('token','actor',1,1,nodes),{'plan':'fixture'})
            baseline.return_value.run.assert_called_once_with('token')
            self.assertEqual(calls,['package','inventory','census','catalog'])
            self.assertEqual(len(subject.journals),4)
            for journal in subject.journals: self.addCleanup(journal.close)
            self.assertIs(subject.package.journal,subject.journals[0])
            self.assertIs(subject.inventory.package,subject.package)
            self.assertIs(subject.census.inventory,subject.inventory)
            self.assertIs(subject.catalog.inventory,subject.inventory)
            with self.assertRaises(supervisor.SupervisorError): subject.prepare('token','actor',1,1,nodes)

    def test_replaced_ledger_client_refuses_before_phase(self):
        self.fence.ledger.client=object()
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')

    def test_go_journal_is_fixed_to_source_database_set(self):
        subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
        journal,_,_=subject._open_go('chat_db');self.addCleanup(journal.close)
        self.assertEqual(journal.path,self.operation/'go-migration-chat_db-journal')
        with self.assertRaises(supervisor.SupervisorError):subject._open_go('../foreign')

    def test_bootstrap_creates_fixed_immutable_intent_and_never_replays(self):
        other=copy.deepcopy(self.base.context);other['operation_id']='media-c1-'+'f'*32
        check=type(self.fence.check)(other)
        bootstrap=supervisor.OperationBootstrap(check,self.base.lock,'ns')
        self.addCleanup(bootstrap.journal.close)
        self.assertEqual(bootstrap.journal.path,self.root/other['operation_id']/'journal')
        self.assertEqual(bootstrap.journal.intent['cohort_route_mode'],'full-cohort-v1')
        self.assertEqual(bootstrap.journal.intent['kube_lock'],self.base.lock)
        with mock.patch.object(supervisor,'begin_operation',side_effect=RuntimeError('unknown')) as begin:
            with self.assertRaises(RuntimeError):bootstrap.begin()
            with self.assertRaises(supervisor.SupervisorError):bootstrap.begin()
            begin.assert_called_once()

    def test_bootstrap_refuses_changed_lock_before_begin(self):
        other=copy.deepcopy(self.base.context);other['operation_id']='media-c1-'+'e'*32
        bootstrap=supervisor.OperationBootstrap(type(self.fence.check)(other),self.base.lock,'ns')
        self.addCleanup(bootstrap.journal.close)
        bootstrap.lock['metadata']['resourceVersion']='changed'
        with mock.patch.object(supervisor,'begin_operation') as begin:
            with self.assertRaises(supervisor.SupervisorError):bootstrap.begin()
            begin.assert_not_called()

    def test_begin_routes_only_fixed_operation_and_requires_immutable_mode(self):
        with mock.patch.object(self.fence,'begin_image_only') as c1, \
             mock.patch.object(self.fence,'begin_with_predecessor') as c2:
            supervisor.begin_operation(self.root_journal,self.fence,self.client,self.base.lock,'ns')
            c1.assert_called_once();c2.assert_not_called()
            self.root_journal.intent['cohort_route_mode']='legacy'
            with self.assertRaises(supervisor.SupervisorError):
                supervisor.begin_operation(self.root_journal,self.fence,self.client,self.base.lock,'ns')
            self.assertEqual(c1.call_count,1)

    def test_pending_or_missing_admission_cannot_reconstruct_supervisor(self):
        step=self.root_journal.state['steps']['image-only-admission']
        step['status']='pending'
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
        step['status']='done';self.base.ledger.row.pop('image_only_baseline')
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
