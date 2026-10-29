"""Actual phase/helper/CUT with separate Journal/Fence facades; observers synthetic.

This fixture exercises the actual WriterCut dispatch and effect map, but does
not qualify actual writer drain or SQL session attribution.
"""
import copy
import unittest
from pathlib import Path
from unittest import mock

import frontend_runtime_executor as transport
import media_operation_journal as journals
import resource_fencing
import space_fresh_install_phase as fresh
import space_fresh_install_phase_test as fixture
import writer_cut
import writer_cut_space as space
import writer_cut_test as cut_fixture


class SpaceCutTest(unittest.TestCase):
    def setUp(self):
        f=fixture.PhaseTest('test_one_shot_success_retains_original_and_checked_cleanup');self.f=f
        def cleanup():
            if not f.doCleanups():raise RuntimeError('nested phase cleanup failed')
        self.addCleanup(cleanup);f.setUp()
        scope={'engine':copy.deepcopy(f.engine),'server_start':'start',
            'databases':sorted(f.original['all_databases']),'database_oids':copy.deepcopy(f.original['all_databases'])}
        if self._testMethodName=='test_incomplete_original_scope_refuses_before_bind_and_drop':
            scope['database_oids'].pop('voice_db');scope['databases'].remove('voice_db')
        admission={'controllers':[cut_fixture.workload('Deployment','voice-space','space-controller',1,'source-image')],
            'guards':{'jobs':[],'cronjobs':[],'hpas':[]},'linked':{'jobs':[],'cronjobs':[]},'session_scope':scope}
        cut_journal=journals.Journal(Path(f.temp.name)/'cut-journal',{
            'resource_context':copy.deepcopy(f.context),'namespace_uid':'ns','writer_cut':copy.deepcopy(admission)})
        self.addCleanup(cut_journal.close)
        cut_client=transport.KubectlTransport(cut_journal);cut_client.call=f.call
        cut_ledger=resource_fencing.KubeLedger(cut_client)
        cut_ledger.read=f.backing.read;cut_ledger.write=f.backing.write
        cut_fence=resource_fencing.Fence(copy.deepcopy(f.context),lambda:copy.deepcopy(f.card),cut_ledger)
        cut=writer_cut.WriterCut(cut_journal,cut_fence,cut_client,f.lock,'ns',admission,
            lambda scope:{'scope':scope,'active_sessions':0,'active_transactions':0})
        f.cut=self.cut=cut
        cut._start();cut.drained=True
        cut.verify_drained=mock.Mock(return_value={})
        cut._retired=mock.Mock(return_value=True);cut._recheck_config_successors=mock.Mock()
        def guards():
            actual=writer_cut.inventory([obj for (kind,name),obj in f.objects.items() if kind=='job'])
            if getattr(cut,'space_fresh',None) is not None:space.guard_jobs(cut,actual)
            elif actual:raise ValueError()
        cut._guards=guards
        self.probes=[]
        def inspect(scope,identity,guard):
            guard();self.probes.append(copy.deepcopy(identity))
            return {'scope':copy.deepcopy(scope),'active_sessions':0,'active_transactions':0,'attributed_sessions':[]}
        patch=mock.patch.object(space.pg,'inspect_space',side_effect=inspect,create=True)
        patch.start();self.addCleanup(patch.stop)

    def test_two_jobs_and_durable_finished_binding(self):
        phase=self.f.phase();receipt=phase.run()
        self.assertIsNot(phase.journal,self.cut.journal)
        self.assertIsNot(phase.fence,self.cut.fence)
        self.assertIsNot(phase.client,self.cut.client)
        binding=space.verify_binding(self.cut,require_finished=True)
        self.assertIs(binding,self.cut.space_fresh);self.assertIs(binding['phase'],phase)
        self.assertEqual(binding['receipt_sha256'],receipt['receipt_sha256'])
        self.assertEqual({p['job_kind'] for p in self.probes},{'provision','verify'})
        self.assertEqual(self.f.drops,[11])
        self.assertEqual(self.cut.journal.state['steps'][space.FINISH]['status'],'done')
        with self.assertRaises(writer_cut.RecoveryRequired):space.finish(self.cut)
        self.assertEqual(self.f.drops,[11])

    def test_foreign_session_before_drop_poisoned_without_drop(self):
        original=space.pg.inspect_space.side_effect
        def foreign(scope,identity,guard):
            result=original(scope,identity,guard);result['active_sessions']=1;return result
        space.pg.inspect_space.side_effect=foreign
        with self.assertRaises(fresh.SpaceFreshError):self.f.phase().run()
        self.assertEqual(self.f.drops,[]);self.assertEqual(self.f.backing.row['status'],'uncertain')

    def test_incomplete_original_scope_refuses_before_bind_and_drop(self):
        with self.assertRaises(fresh.SpaceFreshError):self.f.phase().run()
        self.assertEqual(self.f.drops,[])
        self.assertNotIn(space.BIND,self.cut.journal.state['steps'])
        self.assertEqual(self.f.backing.row['status'],'uncertain')

    def test_binding_cannot_be_replaced_or_terminal_local_deleted(self):
        phase=self.f.phase();phase.run();original=self.cut.space_fresh
        self.cut.space_fresh=dict(original)
        with self.assertRaises(ValueError):space.verify_binding(self.cut,True)
        self.cut.space_fresh=original
        self.cut.journal.state['steps'].pop(space.FINISH)
        with self.assertRaises(Exception):space.verify_binding(self.cut,True)

    def test_final_ledger_read_cannot_close_journal(self):
        self.f.phase().run()
        ledger=self.cut.fence.ledger;read=ledger.read;calls=[]
        def count():
            calls.append(None);return read()
        with mock.patch.object(ledger,'read',side_effect=count):
            self.assertEqual(space.guard_jobs(self.cut,[]),[])
        final=len(calls);self.assertGreater(final,0);calls.clear()
        def close_at_final():
            result=read();calls.append(None)
            if len(calls)==final:self.cut.journal.close()
            return result
        with mock.patch.object(ledger,'read',side_effect=close_at_final):
            with self.assertRaises(Exception):space.guard_jobs(self.cut,[])
        self.assertEqual(len(calls),final)


if __name__=='__main__':unittest.main()
