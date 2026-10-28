"""One-shot GitHub run binding, with no GitHub or live cluster writes."""
import copy
import unittest
from unittest import mock

import media_backup_upload_phase as upload
import media_backup_upload_phase_test as fixture
import media_backup_upload_run as runner


class RunTest(unittest.TestCase):
    def setUp(self):
        self.test = fixture.UploadTest('test_requires_complete_dump_set_and_one_shot_reservation')
        self.test.setUp()
        self.addCleanup(self.test.doCleanups)
        self.test.complete_all()
        self.planned = upload.prepare(self.test.phases, b'x' * 32, self.test.bundle)
        self.env = {'GITHUB_REPOSITORY': runner.REPOSITORY, 'GITHUB_EVENT_NAME': 'workflow_dispatch',
            'GITHUB_REF': 'refs/heads/master', 'GITHUB_WORKFLOW_REF': runner.WORKFLOW_REF,
            'GITHUB_SHA': self.test.base.context['commit'], 'GITHUB_WORKFLOW_SHA': self.test.base.context['commit'],
            'GITHUB_RUN_ID': '123', 'GITHUB_RUN_ATTEMPT': '1'}

    def claim(self, env=None, challenge=None):
        return runner.claim(self.test.base.fence, self.test.phases.client, self.test.base.lock, 'ns',
                            self.env if env is None else env,
                            self.planned['challenge'] if challenge is None else challenge)

    def test_only_one_run_can_bind(self):
        actual = self.claim()
        self.assertEqual(actual['status'], 'upload-running')
        self.assertEqual(actual['run_id'], '123')
        with self.assertRaises(runner.RecoveryRequired): self.claim()

    def test_wrong_repo_ref_source_attempt_or_challenge_never_binds(self):
        for field, value in (('GITHUB_REPOSITORY', 'other/repo'), ('GITHUB_REF', 'refs/heads/other'),
                             ('GITHUB_WORKFLOW_REF', 'other'), ('GITHUB_SHA', 'f' * 40),
                             ('GITHUB_WORKFLOW_SHA', 'f' * 40),
                             ('GITHUB_RUN_ATTEMPT', '2'), ('GITHUB_RUN_ID', '0')):
            with self.subTest(field=field):
                env = copy.deepcopy(self.env); env[field] = value
                with self.assertRaises(runner.RecoveryRequired): self.claim(env)
        with self.assertRaises(runner.RecoveryRequired): self.claim(challenge='f' * 32)
        self.assertEqual(self.test.base.ledger.row['backup_upload']['status'], 'dispatch-pending')

    def test_lost_cas_response_never_allows_second_run(self):
        original = self.test.base.ledger.write
        def lost(revision, row):
            original(revision, row)
            if row.get('backup_upload', {}).get('status') == 'upload-running':
                raise OSError('unknown')
        with mock.patch.object(self.test.base.ledger, 'write', side_effect=lost), \
                mock.patch.object(self.test.base.fence, 'uncertain', side_effect=OSError('unknown')):
            with self.assertRaises(runner.RecoveryRequired): self.claim()
        env = self.env | {'GITHUB_RUN_ID': '124'}
        with self.assertRaises(runner.RecoveryRequired): self.claim(env)
        self.assertEqual(self.test.base.ledger.row['backup_upload']['status'], 'upload-running')

    def test_invocation_change_after_guard_refuses_run(self):
        actual_guard = runner.media_operation_guard.verify
        calls = []
        def changed(*args):
            actual_guard(*args)
            calls.append(1)
            if len(calls) == 2:
                with self.test.base.ledger.lock:
                    self.test.base.ledger.row['backup_invocation']['nonce'] = 'f' * 32
                    self.test.base.ledger.version += 1
        with mock.patch.object(runner.media_operation_guard, 'verify', side_effect=changed):
            with self.assertRaises(runner.RecoveryRequired): self.claim()


if __name__ == '__main__': unittest.main()
