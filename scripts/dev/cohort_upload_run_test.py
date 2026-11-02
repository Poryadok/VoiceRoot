import unittest
import threading
from concurrent.futures import ThreadPoolExecutor
from unittest.mock import patch
from pathlib import Path
import cohort_upload_phase_test as fixture
import cohort_upload_phase as phase
import cohort_upload_run as run


class RunTest(unittest.TestCase):
    def setUp(self):
        route=patch('cohort_upload_runner.route_mode',return_value=None);route.start();self.addCleanup(route.stop)
        self.base = fixture.UploadPhaseTest('test_reservation_is_one_shot_and_receipt_bound')
        self.base.setUp(); self.addCleanup(self.base.doCleanups); self.base.install_marker()
        with patch.object(phase, 'ROOT', Path('/fixed')), \
             patch.object(phase.custody, 'verify_bundle', return_value={'bytes':123,'sha256':'d'*64}):
            self.plan = phase.prepare(self.base.base.phase,b'x'*32,self.base.path)
        self.p = self.base.base.media
        self.base.base.base.ledger.row['cohort_dispatch'] = run.dispatch_marker(self.plan)
        self.env = {'GITHUB_REPOSITORY':'Poryadok/VoiceRoot','GITHUB_EVENT_NAME':'workflow_dispatch',
            'GITHUB_REF':'refs/heads/master','GITHUB_WORKFLOW_REF':run.WORKFLOW_REF,
            'GITHUB_SHA':self.p.fence.context['commit'],'GITHUB_WORKFLOW_SHA':self.p.fence.context['commit'],
            'GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'1'}

    def test_exact_run_one_shot(self):
        actual = run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env,self.plan['challenge'],['roles','voice_db'])
        self.assertEqual(actual['run_id'],'123')
        with self.assertRaises(run.CohortRunError):
            run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env,self.plan['challenge'],['roles','voice_db'])

    def test_missing_dispatch_is_refused(self):
        del self.base.base.base.ledger.row['cohort_dispatch']
        with self.assertRaises(run.CohortRunError):
            run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env,self.plan['challenge'],['roles','voice_db'])
        self.assertEqual(self.base.base.base.ledger.row['cohort_upload']['status'], 'dispatch-pending')

    def test_dispatch_drift_after_claim_is_uncertain(self):
        original = run.media_operation_guard.verify
        calls = 0
        def guard(*args):
            nonlocal calls
            original(*args); calls += 1
            if calls == 2:
                row, rev = self.base.base.base.ledger.read()
                row['cohort_dispatch']['challenge'] = '0'*32
                self.base.base.base.ledger.write(rev, row)
        with patch.object(run.media_operation_guard, 'verify', side_effect=guard):
            with self.assertRaises(run.CohortRunError):
                run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env,self.plan['challenge'],['roles','voice_db'])
        self.assertEqual(self.base.base.base.ledger.row['status'], 'uncertain')

    def test_wrong_source_or_attempt_never_binds(self):
        for field,value in [('GITHUB_SHA','f'*40),('GITHUB_RUN_ATTEMPT','2'),('GITHUB_WORKFLOW_REF','wrong')]:
            with self.subTest(field=field):
                with self.assertRaises(run.CohortRunError):
                    run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env|{field:value},self.plan['challenge'],['roles','voice_db'])
        self.assertEqual(self.base.base.base.ledger.row['cohort_upload']['status'],'dispatch-pending')

    def test_coherent_member_removal_or_marker_drift_refused(self):
        row = self.base.base.base.ledger.row
        del row['cohort_members']['roles']
        del row['cohort_bundle']['receipt_sha256']['roles']
        with self.assertRaises(run.CohortRunError):
            run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env,self.plan['challenge'],['roles','voice_db'])

    def test_lost_cas_response_never_allows_retry(self):
        original = self.base.base.base.ledger.write
        def lost(revision,row):
            original(revision,row)
            if row.get('cohort_upload',{}).get('status') == 'upload-running': raise OSError('lost')
        with patch.object(self.base.base.base.ledger,'write',side_effect=lost), \
             patch.object(self.p.fence,'uncertain',side_effect=OSError('unknown')):
            with self.assertRaises(run.CohortRunError):
                run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env,self.plan['challenge'],['roles','voice_db'])
        with self.assertRaises(run.CohortRunError):
            run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env|{'GITHUB_RUN_ID':'124'},self.plan['challenge'],['roles','voice_db'])

    def test_competing_claims_never_both_succeed(self):
        barrier = threading.Barrier(2)
        original = self.base.base.base.ledger.write
        def simultaneous(revision,row):
            if row.get('cohort_upload',{}).get('status') == 'upload-running': barrier.wait(timeout=5)
            original(revision,row)
        def attempt(number):
            try:
                run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,
                          self.env|{'GITHUB_RUN_ID':str(number)},self.plan['challenge'],['roles','voice_db'])
                return True
            except run.CohortRunError: return False
        with patch.object(self.base.base.base.ledger,'write',side_effect=simultaneous):
            with ThreadPoolExecutor(max_workers=2) as pool:
                results = list(pool.map(attempt, (123,124)))
        self.assertLessEqual(sum(results),1)

    def test_marker_drift_after_cas_is_uncertain(self):
        original = run.media_operation_guard.verify
        calls = 0
        def guard(*args):
            nonlocal calls
            original(*args); calls += 1
            if calls == 2:
                row, rev = self.base.base.base.ledger.read()
                row['cohort_bundle']['manifest_sha256'] = '0'*64
                self.base.base.base.ledger.write(rev,row)
        with patch.object(run.media_operation_guard,'verify',side_effect=guard):
            with self.assertRaises(run.CohortRunError):
                run.claim(self.p.fence,self.p.client,self.p.lock,self.p.namespace_uid,self.env,self.plan['challenge'],['roles','voice_db'])
        self.assertEqual(self.base.base.base.ledger.row['status'],'uncertain')


if __name__ == '__main__': unittest.main()
