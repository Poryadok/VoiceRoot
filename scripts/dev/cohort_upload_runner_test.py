import unittest
import os
from pathlib import Path
from unittest.mock import patch
import cohort_upload_runner as runner
import cohort_backup_operation_test as operation_fixture
import cohort_upload_run as run
import media_backup_phase_test as phase_fixture
import story_media_backup_cohort_custody as custody


class RunnerTest(unittest.TestCase):
    def test_unrecognised_operation_is_refused_before_source_or_ledger(self):
        with patch.object(runner.os, 'geteuid', return_value=1000), \
             patch.object(runner.publish_candidate, 'source_snapshot') as source:
            with self.assertRaises(runner.RunnerError):
                runner.stage({}, 'media-c3-'+'a'*32, 'b'*32, Path.cwd())
        source.assert_not_called()

    def test_real_private_cohort_bundle_staged_once(self):
        self.run_fixture(None)

    def test_tamper_or_output_failure_is_uncertain(self):
        for fault in ('tamper','output'):
            with self.subTest(fault=fault): self.run_fixture(fault)

    def test_failed_upload_marks_only_its_run_uncertain(self):
        self.run_fixture('mark_failed')

    def test_preflight_marker_and_post_claim_drift_fail_closed(self):
        for fault in ('preflight_marker', 'post_claim_drift'):
            with self.subTest(fault=fault): self.run_fixture(fault)

    def run_fixture(self, fault):
        fixture = operation_fixture.OperationTest('test_real_guarded_producer_encrypts_distinct_members')
        data = fixture.run_real(False); self.addCleanup(fixture.doCleanups)
        base = data['base']; operation = data['binding']['operation_id']; opdir = data['root'].parent
        marker = run.dispatch_marker(data['planned'])
        base.ledger.row['cohort_dispatch'] = marker.copy()
        data['phase'].phases.journal.state['steps']['cohort-dispatch'] = marker.copy()
        data['phase'].phases.journal.save()
        key_path = opdir/'backup.key'; key_path.write_bytes(data['key']); key_path.chmod(0o600)
        output = opdir/'github-output'; output.touch(mode=0o600)
        env = {'GITHUB_REPOSITORY':run.REPOSITORY,'GITHUB_EVENT_NAME':'workflow_dispatch',
            'GITHUB_REF':'refs/heads/master','GITHUB_WORKFLOW_REF':run.WORKFLOW_REF,
            'GITHUB_SHA':base.context['commit'],'GITHUB_WORKFLOW_SHA':base.context['commit'],
            'GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'1','GITHUB_OUTPUT':str(output)}
        if fault == 'tamper':
            with data['bundle'].open('r+b') as stream: stream.write(b'X')
        if fault == 'output': env['GITHUB_OUTPUT'] = str(opdir/'missing-output')
        client = data['phase'].phases.client
        if fault == 'preflight_marker':
            data['phase'].phases.journal.state['steps']['cohort-bundle']['marker']['bundle_sha256'] = 'f'*64
            data['phase'].phases.journal.save()
        with patch.object(runner,'ROOT',Path(base.temp.name)), patch.object(runner,'OPERATOR_UID',os.geteuid()), \
             patch.object(runner.publish_candidate,'source_snapshot',return_value={}), \
             patch.object(runner.media,'Client',return_value=client), \
             patch.object(runner.resource_fencing,'ResourceCheck'), \
             patch.object(runner.resource_fencing,'Fence',return_value=base.fence):
            if fault == 'preflight_marker':
                with patch.object(run, 'claim') as claim:
                    with self.assertRaises(runner.RunnerError):
                        runner.stage(env,operation,data['planned']['challenge'],Path.cwd())
                    claim.assert_not_called()
                return
            if fault == 'post_claim_drift':
                actual_claim = run.claim
                def drift(*args):
                    result = actual_claim(*args)
                    base.ledger.row['cohort_upload']['bundle_sha256'] = 'f'*64
                    return result
                actual_private = runner.media.private_file
                def private(directory, name, limit):
                    if name == 'backup.key': self.fail('key read after authority drift')
                    return actual_private(directory, name, limit)
                with patch.object(run,'claim',side_effect=drift), patch.object(runner.media,'private_file',side_effect=private):
                    with self.assertRaises(runner.RunnerError):
                        runner.stage(env,operation,data['planned']['challenge'],Path.cwd())
                self.assertEqual(base.ledger.row['status'],'uncertain')
                return
            if fault in ('tamper','output'):
                with self.assertRaises(runner.RunnerError):
                    runner.stage(env,operation,data['planned']['challenge'],Path.cwd())
                self.assertEqual(base.ledger.row['status'],'uncertain')
                return
            target = runner.stage(env,operation,data['planned']['challenge'],Path.cwd())
            self.assertEqual(custody.verify_bundle(target,data['key'],data['binding'],['auth_db','voice_db'])['sha256'],
                             data['planned']['bundle_sha256'])
            if fault == 'mark_failed':
                with self.assertRaises(runner.RunnerError):
                    runner.mark_failed(env|{'GITHUB_RUN_ID':'124'},operation,data['planned']['challenge'],Path.cwd())
                self.assertEqual(base.ledger.row['status'],'active')
                runner.mark_failed(env,operation,data['planned']['challenge'],Path.cwd())
                self.assertEqual(base.ledger.row['status'],'uncertain')
                return
            with self.assertRaises(run.CohortRunError):
                runner.stage(env,operation,data['planned']['challenge'],Path.cwd())


if __name__ == '__main__': unittest.main()
