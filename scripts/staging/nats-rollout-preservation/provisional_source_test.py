"""Exact pending-attempt admission; ordinary authority remains strict."""
import copy
import unittest
from unittest.mock import patch
import source_authority as source

SHA='a'*40

class Tests(unittest.TestCase):
    def setUp(self):
        self.repo={'id':111,'full_name':source.REPO}
        self.run={'id':123,'workflow_id':source.WORKFLOW,'head_sha':SHA,
            'head_branch':'master','repository':self.repo,'head_repository':self.repo,
            'event':'push','path':'.github/workflows/ci.yml','status':'in_progress',
            'conclusion':None,'run_attempt':2}
        self.jobs=[{'id':1,'run_id':123,'head_sha':SHA,'run_attempt':2,
            'name':'staging-stack-lock','status':'completed','conclusion':'success'},
            {'id':2,'run_id':123,'head_sha':SHA,'run_attempt':2,
            'name':'ci-gate','status':'queued','conclusion':None}]

    def call(self,provisional=True):
        with patch.object(source,'_get',return_value=self.run),patch.object(source,'_pages',return_value=self.jobs):
            return (source._provisional_ci if provisional else source._ci)({},123,SHA,self.repo,1)

    def test_exact_pending_attempt_admitted_but_ordinary_rejected(self):
        run,jobs=self.call()
        self.assertEqual(run['run_attempt'],2)
        self.assertEqual(jobs['staging-stack-lock']['id'],1)
        with self.assertRaises(source.SourceError):self.call(False)

    def test_failed_cancelled_or_skipped_gate_rejected(self):
        for conclusion in ('failure','cancelled','timed_out','skipped','neutral'):
            with self.subTest(conclusion=conclusion):
                self.jobs[1].update(status='completed',conclusion=conclusion)
                with self.assertRaises(source.SourceError):self.call()

    def test_dependency_blocked_gate_not_yet_listed_is_pending_only(self):
        self.jobs.pop()
        self.assertIsNone(self.call()[1]['ci-gate'])
        with self.assertRaises(source.SourceError):self.call(False)
        self.run.update(status='completed',conclusion='success')
        with self.assertRaises(source.SourceError):self.call()

    def test_any_observed_failed_job_rejected_even_if_gate_pending(self):
        for conclusion in ('failure','cancelled','timed_out'):
            with self.subTest(conclusion=conclusion):
                self.jobs.append(dict(self.jobs[0],id=3,name='optional',conclusion=conclusion))
                with self.assertRaises(source.SourceError):self.call()
                self.jobs.pop()

    def test_lock_pending_and_mixed_run_or_attempt_rejected(self):
        original=copy.deepcopy(self.jobs)
        for mutation in ({'status':'in_progress','conclusion':None},{'run_id':124},
                         {'run_attempt':1},{'head_sha':'b'*40}):
            with self.subTest(mutation=mutation):
                self.jobs=copy.deepcopy(original);self.jobs[0].update(mutation)
                with self.assertRaises(source.SourceError):self.call()

    def test_completed_success_also_requires_real_gate(self):
        self.run.update(status='completed',conclusion='success')
        with self.assertRaises(source.SourceError):self.call()
        self.jobs[1].update(status='completed',conclusion='success')
        self.assertEqual(self.call()[0]['id'],123)

    def test_duplicate_names_and_foreign_dispatch_rejected(self):
        self.jobs.append(dict(self.jobs[0],id=3))
        with self.assertRaises(source.SourceError):self.call()
        self.jobs.pop();self.run['event']='workflow_dispatch'
        with self.assertRaises(source.SourceError):self.call()

    def test_revalidation_never_changes_bound_attempt_and_final_is_strict(self):
        authority={'source_sha':SHA,'run_id':123,'run_attempt':2,'provisional_staging':True}
        with patch.object(source,'_head',return_value=self.repo),\
             patch.object(source,'_provisional_ci',return_value=(self.run,{})) as pending,\
             patch.object(source,'_ci',side_effect=source.SourceError('source_authority_rejected')) as strict:
            source.revalidate_ci('token',authority)
            pending.assert_called_once();strict.assert_not_called()
            with self.assertRaises(source.SourceError):source.revalidate_ci('token',authority,final=True)
            strict.assert_called_once()
            self.run['run_attempt']=3
            with self.assertRaises(source.SourceError):source.revalidate_ci('token',authority)

    def test_advanced_master_cannot_accept_old_source(self):
        authority={'source_sha':SHA,'run_id':123,'run_attempt':2,'provisional_staging':True}
        with patch.object(source,'_head',side_effect=source.SourceError('source_authority_rejected')),\
             patch.object(source,'_ci') as strict:
            with self.assertRaises(source.SourceError):source.revalidate_ci('token',authority,final=True)
        strict.assert_not_called()

if __name__=='__main__':unittest.main()
