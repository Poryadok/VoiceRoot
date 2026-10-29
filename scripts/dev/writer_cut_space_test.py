"""Actual phase/helper/CUT journal effects; controller and PG observers synthetic.

Temporary wrapper wiring represents the concurrently owned WriterCut integration;
this fixture does not qualify actual writer drain or SQL session attribution.
"""
import copy
import unittest
from unittest import mock

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
        admission={'controllers':[cut_fixture.workload('Deployment','voice-space','space-controller',1,'source-image')],
            'guards':{'jobs':[],'cronjobs':[],'hpas':[]},'linked':{'jobs':[],'cronjobs':[]},'session_scope':scope}
        f.journal.intent['writer_cut']=copy.deepcopy(admission);f.journal.state['intent']=copy.deepcopy(f.journal.intent)
        cut=writer_cut.WriterCut(f.journal,f.fence,f.client,f.lock,'ns',admission,
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
        original_map=cut._effect_map
        def effect_map(exclude=None):
            result=original_map(exclude)
            for name in (space.BIND,space.FINISH):
                if name==exclude or name not in cut.journal.state['steps']:continue
                entry=cut.journal.state['steps'][name]
                if entry['details_sha256']!=writer_cut.digest(entry['details']):raise ValueError()
                result[name]={k:entry[k] for k in ('status','nonce','details_sha256')}
                if entry['status']=='done':result[name]['receipt_sha256']=writer_cut.digest(entry['receipt'])
            return result
        cut._effect_map=effect_map
        cut.bind_space_fresh=lambda phase,token:space.bind(cut,phase,token)
        cut.verify_space_pending=lambda:space.pending(cut)
        cut.verify_space_running=lambda:space.running(cut)
        cut.finish_space_fresh=lambda:space.finish(cut)
        self.probes=[]
        def inspect(scope,identity,guard):
            guard();self.probes.append(copy.deepcopy(identity))
            return {'scope':copy.deepcopy(scope),'active_sessions':0,'active_transactions':0,'attributed_sessions':[]}
        patch=mock.patch.object(space.pg,'inspect_space',side_effect=inspect,create=True)
        patch.start();self.addCleanup(patch.stop)

    def test_two_jobs_and_durable_finished_binding(self):
        phase=self.f.phase();receipt=phase.run()
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

    def test_binding_cannot_be_replaced_or_terminal_local_deleted(self):
        phase=self.f.phase();phase.run();original=self.cut.space_fresh
        self.cut.space_fresh=dict(original)
        with self.assertRaises(ValueError):space.verify_binding(self.cut,True)
        self.cut.space_fresh=original
        self.cut.journal.state['steps'].pop(space.FINISH)
        with self.assertRaises(Exception):space.verify_binding(self.cut,True)


if __name__=='__main__':unittest.main()
