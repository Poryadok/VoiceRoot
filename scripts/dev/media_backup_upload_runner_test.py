"""Linux end-to-end private ciphertext staging, with synthetic DEV/GitHub."""
import hashlib
import json
import os
from pathlib import Path
import unittest
from unittest import mock

import media_backup_phase as phase
import media_backup_phase_test as fixture
import media_backup_upload_phase as upload
import media_backup_upload_run as run
import media_backup_upload_runner as runner
import media_operation_journal as journal
import story_media_backup as backup
import story_media_backup_custody as custody


class RunnerTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        ctx, capture = self.base.context, self.base.capture
        root = Path(self.base.temp.name)
        self.opdir = root / ctx['operation_id']; self.opdir.mkdir(mode=0o700)
        self.backupdir = self.opdir / 'backup'; self.backupdir.mkdir(mode=0o700)
        restore = {'catalog_sha256': '1' * 64, 'roles_catalog_sha256': '2' * 64,
            'migration_catalog_sha256': '3' * 64, 'migrate_binary_sha256': '4' * 64,
            'postgres_image': 'example/postgres@sha256:' + '5' * 64,
            'migrate_image': 'example/migrate@sha256:' + '6' * 64, 'postgres_major': 16}
        binding = {'operation_id': ctx['operation_id'], 'snapshot_id': capture['snapshot_id'],
            'source_sha': ctx['commit'], **{k: ctx[k] for k in
                ('project', 'resource', 'actor', 'claim', 'window', 'generation', 'revision')},
            'namespace_uid': 'ns', **{k: capture[k] for k in
                ('pod_uid', 'pvc_uid', 'pv_uid', 'container_id', 'restart_count')}, 'restore_inputs': restore}
        intent = self.base.intent | {'backup_binding': binding, 'upload_workflow_sha': ctx['commit'],
                                     'kube_lock': self.base.lock}
        self.journal = journal.Journal(self.opdir / 'journal', intent)
        self.addCleanup(self.journal.close)
        self.phases = phase.BackupPhases(self.journal, self.base.fence, fixture.Client(self.base.lock), self.base.lock, 'ns')
        for member in sorted(phase.MEMBERS):
            nonce = self.phases.begin(member)
            receipt = capture | {'operation_id': ctx['operation_id'], 'member': member,
                                  'exit': 0, 'scope': 'dump-terminal-only', 'attempt_nonce': nonce}
            self.phases.complete(member, receipt)
        receipts = {m: hashlib.sha256(json.dumps(self.journal.state['steps']['dump-' + m]['receipt'],
                    sort_keys=True, separators=(',', ':')).encode()).hexdigest() for m in phase.MEMBERS}
        final_binding = binding | {'dump_receipts': receipts}
        key = bytes(range(32))
        keypath = self.opdir / 'backup.key'; keypath.write_bytes(key); keypath.chmod(0o600)
        manifest = backup.export(self.backupdir, key, final_binding,
                                 lambda member, out: out.write(('synthetic-' + member).encode()))
        self.bundle = custody.build_bundle(self.backupdir, key, final_binding, manifest)
        self.binding, self.key = final_binding, key
        self.patches = [mock.patch.object(upload, 'ROOT', root), mock.patch.object(runner, 'ROOT', root),
                        mock.patch.object(runner, 'OPERATOR_UID', os.geteuid()),
                        mock.patch.object(runner.publish_candidate, 'source_snapshot', return_value={}),
                        mock.patch.object(runner, 'Client', return_value=fixture.Client(self.base.lock)),
                        mock.patch.object(runner.resource_fencing, 'ResourceCheck', return_value=mock.Mock()),
                        mock.patch.object(runner.resource_fencing, 'Fence', return_value=self.base.fence)]
        for patch in self.patches: patch.start(); self.addCleanup(patch.stop)
        self.planned = upload.prepare(self.phases, key, self.bundle)
        output = self.opdir / 'github-output'; output.touch(mode=0o600)
        self.env = {'GITHUB_REPOSITORY': run.REPOSITORY, 'GITHUB_EVENT_NAME': 'workflow_dispatch',
                    'GITHUB_REF': 'refs/heads/master', 'GITHUB_WORKFLOW_REF': run.WORKFLOW_REF,
                    'GITHUB_SHA': ctx['commit'], 'GITHUB_WORKFLOW_SHA': ctx['commit'],
                    'GITHUB_RUN_ID': '123', 'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_OUTPUT': str(output)}

    def test_verified_ciphertext_staged_once(self):
        target = runner.stage(self.env, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())
        self.assertEqual(custody.verify_bundle(target, self.key, self.binding),
                         {'bytes': self.planned['bundle_bytes'], 'sha256': self.planned['bundle_sha256']})
        with self.assertRaises(run.RecoveryRequired):
            runner.stage(self.env, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())

    def test_changed_source_bundle_marks_uncertain(self):
        with self.bundle.open('r+b') as stream:
            stream.write(b'X')
        with self.assertRaises(runner.Refused):
            runner.stage(self.env, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_output_failure_marks_uncertain(self):
        self.env['GITHUB_OUTPUT'] = str(self.opdir / 'missing-output')
        with self.assertRaises(runner.Refused):
            runner.stage(self.env, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_upload_failure_marks_the_exact_run_uncertain(self):
        runner.stage(self.env, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())
        runner.mark_failed(self.env, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_other_run_cannot_mark_owned_upload(self):
        runner.stage(self.env, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())
        wrong = self.env | {'GITHUB_RUN_ID': '124'}
        with self.assertRaises(runner.Refused):
            runner.mark_failed(wrong, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())
        self.assertEqual(self.base.ledger.row['status'], 'active')

    def test_invocation_drift_after_guard_marks_uncertain(self):
        actual_guard = runner.media_operation_guard.verify
        calls = []
        def changed(*args):
            actual_guard(*args)
            calls.append(1)
            if len(calls) == 3:
                with self.base.ledger.lock:
                    self.base.ledger.row['backup_invocation']['nonce'] = 'f' * 32
                    self.base.ledger.version += 1
        with mock.patch.object(runner.media_operation_guard, 'verify', side_effect=changed):
            with self.assertRaises(runner.Refused):
                runner.stage(self.env, self.base.context['operation_id'], self.planned['challenge'], Path.cwd())
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')


if __name__ == '__main__': unittest.main()
