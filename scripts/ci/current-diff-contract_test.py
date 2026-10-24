"""Current-input gate acceptance cases, using real disposable Git histories."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

CHECKER = Path(__file__).with_name('current-diff-contract.py')


class CurrentDiffTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name)
        self.git('init', '-q')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('config', 'user.name', 'CI fixture')
        self.write('README.md', 'base\n')
        self.write('src/backend/migrations/voice_db/000001_base.up.sql', 'SELECT 1;\n')
        self.write('src/backend/voice/internal/roomlifecycle/postgres_store.go',
                   'package roomlifecycle\n')
        self.base = self.commit('base')

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], text=True).strip()

    def write(self, path, content):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding='utf-8')

    def commit(self, message):
        self.git('add', '.')
        self.git('commit', '-qm', message)
        return self.git('rev-parse', 'HEAD')

    def merge_commit(self, tree=None):
        source = self.git('rev-parse', 'HEAD')
        tree = tree or self.git('rev-parse', 'HEAD^{tree}')
        return self.git('commit-tree', tree, '-p', self.base, '-p', source, '-m', 'PR merge')

    def check(self, target='HEAD', head=None, base=None, extra_env=None):
        command = ['python3' if os.name != 'nt' else 'python', str(CHECKER),
                   '--repo', str(self.repo), '--base', base or self.base, '--target', target]
        if head:
            command += ['--pr-head', head]
        return subprocess.run(command, capture_output=True, text=True,
                              env={**os.environ, **(extra_env or {})}, timeout=15)

    def test_clean_merge_and_additive_migration_pass(self):
        self.write('src/backend/migrations/voice_db/000002_new.up.sql', 'SELECT 2;\n')
        head = self.commit('add migration')
        result = self.check(self.merge_commit(), head)
        self.assertEqual(result.returncode, 0, result.stderr)
        evidence = json.loads(result.stdout)
        self.assertEqual(evidence['base'], self.base)
        self.assertEqual(evidence['pr_head'], head)
        self.assertFalse(evidence['cached'])

    def test_forged_merge_tree_is_rejected(self):
        self.write('README.md', 'source\n')
        head = self.commit('source')
        result = self.check(self.merge_commit(self.git('rev-parse', self.base + '^{tree}')), head)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('merge_tree', result.stderr)

    def test_wrong_event_head_or_nonmerge_is_rejected(self):
        self.write('README.md', 'source\n')
        head = self.commit('source')
        for target, event_head in [(self.merge_commit(), self.base), (head, head)]:
            with self.subTest(target=target):
                result = self.check(target, event_head)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('merge_parents', result.stderr)

    def test_conflicting_event_parents_are_rejected(self):
        initial = self.base
        self.write('README.md', 'source conflict\n')
        source = self.commit('source')
        self.git('checkout', '-q', initial)
        self.write('README.md', 'target conflict\n')
        self.base = self.commit('target')
        self.git('checkout', '-q', source)
        result = self.check(self.merge_commit(), source)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('merge_tree_conflict', result.stderr)

    def test_missing_or_nonancestor_base_is_rejected(self):
        result = self.check(base='0' * 40)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('base', result.stderr)
        self.write('README.md', 'new\n')
        head = self.commit('new')
        result = self.check(self.base, base=head)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('base_ancestry', result.stderr)

    def test_existing_migration_edit_delete_rename_and_mode_drift_rejected(self):
        path = 'src/backend/migrations/voice_db/000001_base.up.sql'
        for mutation in ('edit', 'delete', 'rename', 'mode'):
            with self.subTest(mutation=mutation):
                if mutation == 'edit':
                    self.write(path, 'SELECT 999;\n')
                elif mutation == 'delete':
                    self.git('rm', path)
                elif mutation == 'rename':
                    self.git('mv', path, path.replace('base', 'renamed'))
                else:
                    self.git('update-index', '--chmod=+x', path)
                    # Match the index entry on POSIX too, so this tests a
                    # committed mode change rather than a dirty worktree.
                    (self.repo / path).chmod(0o755)
                # Preserve index-only mode change.
                if mutation != 'mode':
                    self.git('add', '.')
                self.git('commit', '-qm', mutation)
                result = self.check()
                self.git('checkout', '-q', self.base)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('immutable_migration', result.stderr)

    def test_worktree_changes_and_stale_result_do_not_bypass_validation(self):
        self.assertEqual(self.check().returncode, 0)
        self.write('src/backend/migrations/voice_db/000001_base.up.sql', 'SELECT 999;\n')
        result = self.check(extra_env={'VOICE_CI_CACHED_RESULT': 'success'})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('dirty_worktree', result.stderr)

    def test_source_disabled_activation_and_unknown_production_surface_rejected(self):
        for path, content in [
            ('src/backend/voice/internal/roomlifecycle/postgres_store.go',
             'package roomlifecycle\nfunc boot() { NewLifecycleWorker() }\n'),
            ('src/backend/voice/internal/roomlifecycle/worker.go', 'package roomlifecycle\n'),
            ('src/backend/voice/unapproved.go', 'package main\n'),
        ]:
            with self.subTest(path=path):
                self.write(path, content)
                self.commit('activation')
                result = self.check()
                self.git('checkout', '-q', self.base)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('source_disabled', result.stderr)


if __name__ == '__main__':
    unittest.main()
