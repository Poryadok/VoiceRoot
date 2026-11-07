import unittest
import json
from pathlib import Path
from unittest.mock import patch
import cohort_upload_phase_test as fixture
import cohort_upload_phase as upload
import cohort_dispatch as dispatch
import cohort_upload_run as run
import cohort_upload_runner as runner
import media_operation_journal as journal


class DispatchTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.UploadPhaseTest('test_reservation_is_one_shot_and_receipt_bound')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.base.install_marker()
        with patch.object(upload, 'ROOT', Path('/fixed')), patch.object(upload.custody, 'verify_bundle', return_value={'bytes':123,'sha256':'d'*64}):
            self.plan = upload.prepare(self.base.base.phase, b'x'*32, self.base.path)
        self.phase = self.base.base.phase
        self.calls = []
        p = self.phase.phases
        root = Path(self.base.base.base.temp.name) / 'operations'
        root.mkdir(mode=0o700)
        operation = root / p.fence.context['operation_id']
        operation.mkdir(mode=0o700)
        self.root_journal = journal.Journal(operation / 'journal', {
            'resource_context': p.fence.context, 'kube_lock': p.lock,
            'namespace_uid': p.namespace_uid,
        })
        self.addCleanup(self.root_journal.close)
        route_root = patch.object(runner, 'ROOT', root)
        route_root.start(); self.addCleanup(route_root.stop)

    def transport(self, command, payload):
        self.calls.append((command, payload))
        return b''

    def test_one_submission_and_no_replay(self):
        result = dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True, self.transport)
        self.assertEqual(result['status'], 'accepted-for-dispatch')
        self.assertEqual(len(self.calls), 1)
        with self.assertRaises(dispatch.DispatchError):
            dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True, self.transport)
        self.assertEqual(len(self.calls), 1)

    def test_timeout_is_uncertain_and_never_replayed(self):
        def lost(command, payload):
            self.calls.append(command)
            raise TimeoutError()
        with self.assertRaises(dispatch.DispatchError):
            dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True, lost)
        self.assertEqual(self.base.base.base.ledger.row['status'], 'uncertain')
        with self.assertRaises(dispatch.DispatchError):
            dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True, self.transport)
        self.assertEqual(len(self.calls), 1)

    def test_changed_reservation_refused_before_submit(self):
        self.base.base.base.ledger.row['cohort_upload']['challenge'] = '0'*32
        with self.assertRaises(dispatch.DispatchError):
            dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True, self.transport)
        self.assertEqual(self.calls, [])

    def test_runner_can_claim_during_dispatch(self):
        def racing(command, payload):
            p = self.phase.phases
            env = {'GITHUB_REPOSITORY':run.REPOSITORY, 'GITHUB_EVENT_NAME':'workflow_dispatch',
                   'GITHUB_REF':'refs/heads/master', 'GITHUB_WORKFLOW_REF':run.WORKFLOW_REF,
                   'GITHUB_SHA':p.fence.context['commit'], 'GITHUB_WORKFLOW_SHA':p.fence.context['commit'],
                   'GITHUB_RUN_ID':'123', 'GITHUB_RUN_ATTEMPT':'1'}
            run.claim(p.fence,p.client,p.lock,p.namespace_uid,env,self.plan['challenge'],['roles','voice_db'])
            return b''
        result = dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True, racing)
        self.assertEqual(result['status'], 'accepted-for-dispatch')
        self.assertEqual(self.base.base.base.ledger.row['cohort_upload']['run_id'], '123')

    def test_committed_dispatch_loses_reply_after_runner_claim(self):
        def racing(command, payload):
            p = self.phase.phases
            env = {'GITHUB_REPOSITORY':run.REPOSITORY, 'GITHUB_EVENT_NAME':'workflow_dispatch',
                   'GITHUB_REF':'refs/heads/master', 'GITHUB_WORKFLOW_REF':run.WORKFLOW_REF,
                   'GITHUB_SHA':p.fence.context['commit'], 'GITHUB_WORKFLOW_SHA':p.fence.context['commit'],
                   'GITHUB_RUN_ID':'123', 'GITHUB_RUN_ATTEMPT':'1'}
            run.claim(p.fence,p.client,p.lock,p.namespace_uid,env,self.plan['challenge'],['roles','voice_db'])
            raise TimeoutError()
        with self.assertRaises(dispatch.DispatchError):
            dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True, racing)
        row = self.base.base.base.ledger.row
        self.assertEqual(row['cohort_upload']['run_id'], '123')
        self.assertEqual(row['status'], 'uncertain')

    def test_wrong_source_refused_without_submission(self):
        with self.assertRaises(dispatch.DispatchError):
            dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: False, self.transport)
        self.assertEqual(self.calls, [])

    def test_false_drain_refused_without_submission(self):
        with self.assertRaises(dispatch.DispatchError):
            dispatch.submit(self.phase, self.plan, lambda: False, lambda sha: True, self.transport)
        self.assertEqual(self.calls, [])

    def test_documented_details_response(self):
        def transport(command, payload):
            self.assertIs(json.loads(payload)['return_run_details'], True)
            return json.dumps({'workflow_run_id':123,
                'run_url':'https://api.github.com/repos/Poryadok/VoiceRoot/actions/runs/123',
                'html_url':'https://github.com/Poryadok/VoiceRoot/actions/runs/123'}).encode()
        result = dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True, transport)
        self.assertEqual(result['run_id_hint'], '123')

    def test_bad_response_is_uncertain(self):
        with self.assertRaises(dispatch.DispatchError):
            dispatch.submit(self.phase, self.plan, lambda: None, lambda sha: True,
                            lambda command, payload: b'{"workflow_run_id":true}')
        self.assertEqual(self.base.base.base.ledger.row['status'], 'uncertain')


if __name__ == '__main__': unittest.main()
