"""Real typed preflight and distinct Journals; external Kube/TT are synthetic.

This is not a successful full backup or a live environment qualification.
"""
import copy
from contextlib import contextmanager
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest import mock

import frontend_runtime_executor
import full_cohort_backup_metadata as metadata
import full_cohort_backup_phase as backup
import full_cohort_inventory_phase as inventory
import full_cohort_namespace_census as census
import full_cohort_package_phase as package
import full_cohort_pg_physical_identity as physical
import full_cohort_affected_plan as affected
import full_cohort_supervisor as supervisor
import media_operation_journal
import resource_fencing
import writer_cut
import writer_cut_test


class RealPreflightTest(unittest.TestCase):
    @contextmanager
    def phases(self):
        base=writer_cut_test.WriterCutTest('test_drain_and_resume_use_current_template_not_old')
        base.setUp();self.addCleanup(base.doCleanups)
        root=Path(base.temp.name)/'operations';root.mkdir(mode=0o700)
        op=root/base.context['operation_id'];op.mkdir(mode=0o700)
        admission=copy.deepcopy(base.admission)
        shared_workloads=base.client.workloads
        class Client(writer_cut_test.Client):
            def __init__(self,journal):
                super().__init__(base.lock,journal);self.workloads=shared_workloads
        class Ledger:
            def __init__(self,client):self.client=client
            def read(self):return base.ledger.read()
            def write(self,revision,row):return base.ledger.write(revision,row)
        class Check:
            endpoint='http://fixture'
            def __init__(self,context):self.context=context
            def __call__(self):return copy.deepcopy(base.card)
            def mark_uncertain(self,reason):pass
        check=Check(base.context)
        def facade(name,extra):
            intent={'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns',**extra}
            journal=media_operation_journal.Journal(op/name,intent);self.addCleanup(journal.close)
            client=Client(journal);fence=resource_fencing.Fence(copy.deepcopy(base.context),check,Ledger(client),check.mark_uncertain)
            return journal,client,fence
        with mock.patch.object(frontend_runtime_executor,'KubectlTransport',Client), \
             mock.patch.object(resource_fencing,'KubeLedger',Ledger),mock.patch.object(resource_fencing,'ResourceCheck',Check), \
             mock.patch.object(supervisor.media_backup_upload_runner,'ROOT',root), \
             mock.patch.object(supervisor.media_backup_upload_runner,'TT','http://fixture'), \
             mock.patch.object(backup.cohort_backup_operation,'ROOT',root), \
             mock.patch.object(supervisor.media_operation_guard,'verify'):
            j,c,f=facade('journal',{'cohort_route_mode':'full-cohort-v1','kube_lock':base.lock})
            owner=supervisor.Supervisor(j,f,c,base.lock,'ns')
            pj,pc,pf=facade('package-journal',{'full_cohort_package':{'run_id':123,'run_attempt':1,
                'nodes':[{'name':'node','uid':'uid','hostname':'node','architecture':'amd64'}]}})
            owner.package=package.PackagePhase(pj,pf,pc,base.lock,'ns')
            ij,ic,inf=facade('inventory-journal',{})
            owner.inventory=inventory.InventoryPhase(ij,inf,ic,base.lock,'ns',owner.package)
            cj,cc,cf=facade('catalog-journal',{})
            owner.catalog=affected.FullCatalogPhase(cj,cf,cc,base.lock,'ns',owner.inventory)
            nj,nc,nf=facade('census-journal',{})
            owner.census=census.NamespaceCensus(nj,nf,nc,base.lock,'ns',owner.package,owner.inventory)
            wj,wc,wf=facade('cut-journal',{'writer_cut':admission})
            cut=writer_cut.WriterCut(wj,wf,wc,base.lock,'ns',admission,
                lambda scope:{'scope':scope,'active_sessions':0,'active_transactions':0})
            with mock.patch.object(cut,'_patch',side_effect=wc.patch):cut.drain()
            phj,phc,phf=facade('physical-journal',{})
            storage=physical.PhysicalPhase(phj,phf,phc,base.lock,'ns',owner.catalog,cut)
            mj,mc,mf=facade('metadata-journal',{})
            source=metadata.BackupMetadataPhase(mj,mf,mc,base.lock,'ns',owner.package,owner.inventory,owner.catalog,cut,metadata.canonical.ROOT)
            phase=backup.BackupPhase(owner,cut,storage,source)
            yield SimpleNamespace(base=base,root=root,operation=op,owner=owner,cut=cut,storage=storage,source=source,phase=phase)

    def test_missing_census_refuses_before_backup_journal_or_key(self):
        with self.phases() as work:
            phase=work.phase;op=work.operation;base=work.base
            with self.assertRaises(backup.BackupError):phase.run('fixture-token')
            self.assertFalse((op/'backup-journal').exists())
            self.assertFalse((op/'backup.key').exists())
            self.assertNotIn('full_cohort_backup_route',base.ledger.row)
            self.assertEqual(base.ledger.row['status'],'active')


if __name__=='__main__':unittest.main()
