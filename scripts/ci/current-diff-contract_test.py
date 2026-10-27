"""Current-input gate acceptance cases, using real disposable Git histories."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

CHECKER = Path(__file__).with_name('current-diff-contract.py')


REVIEWED_FIXTURES = [('src/backend/voice/logger.go', None, '44ff46d26c9e4f9d7b067e84d4e7771a797290f3'),
 ('src/backend/voice/main.go',
  'b5434cfd21b94bd69ff25f912858f12f0aec6813',
  '540a91513cfd19d1499183302b3880a578951d56'),
 ('src/backend/voice/ordinary_runtime.go', None, 'b51436bec28c17c8ecb93cad68176564026802e3'),
 ('src/backend/voice/party_outbox_runtime.go', None, 'e7c5eab30a054008d7d16862f622f5d92f5351f6'),
 ('src/backend/voice/space_media_runtime.go', None, 'beff70aecd2d4dd42cbc21ba88691193caf4ac56'),
 ('src/backend/migrations/auth_db/000009_guest_conversion_operations.down.sql',
  'df7ab6deeb0e77de9a6b68a1803fcd660fc084c1',
  '967c11f2f4a57ddc84e188d453f6da9db7a252f9'),
 ('src/backend/migrations/messaging_db/000025_space_file_producer.down.sql',
  'b2211787546381e4725ae65192e2afef8adeb3b2',
  'f2be8b9e29165a15f791f62d7e27980ba6e0c5f4'),
 ('src/backend/migrations/messaging_db/000026_attachment_send_intents.down.sql',
  '338db36c8b247ffa8d6db7392633bc0e552fd0d8',
  '01181fad12cbb8f202bbec16a3a8fece4ffd3a91'),
 ('src/backend/migrations/role_db/000013_game_session_grants.down.sql',
  'cec72a64a43e369be4199fb3b9056ee664c50ba7',
  '3ca06b04b93ba22c830f99387d135fe17aeb179d')]


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

    def merge_commit(self, tree=None, base=None, source=None):
        source = source or self.git('rev-parse', 'HEAD')
        tree = tree or self.git('rev-parse', 'HEAD^{tree}')
        return self.git('commit-tree', tree, '-p', base or self.base, '-p', source, '-m', 'PR merge')

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

    def test_stale_event_base_uses_exact_current_merge_first_parent(self):
        event_base = self.base
        self.write('README.md', 'current base\n')
        current_base = self.commit('advance integration base')
        self.git('checkout', '-q', event_base)
        self.write('docs/pr.md', 'PR source\n')
        head = self.commit('PR source')
        self.git('checkout', '-q', current_base)
        tree = self.git('merge-tree', '--write-tree', current_base, head).splitlines()[0]
        merge = self.merge_commit(tree, base=current_base, source=head)
        result = self.check(merge, head, base=event_base)
        self.assertEqual(result.returncode, 0, result.stderr)
        evidence = json.loads(result.stdout)
        self.assertEqual(evidence['event_base'], event_base)
        self.assertEqual(evidence['base'], current_base)
        self.assertEqual(evidence['pr_head'], head)

    def test_event_base_reachable_only_through_pr_head_is_rejected(self):
        initial = self.base
        self.write('README.md', 'event base\n')
        event_base = self.commit('event base on source branch')
        self.write('docs/extra.md', 'PR source\n')
        head = self.commit('PR source')
        self.git('checkout', '-q', initial)
        tree = self.git('merge-tree', '--write-tree', initial, head).splitlines()[0]
        merge = self.merge_commit(tree, base=initial, source=head)
        result = self.check(merge, head, base=event_base)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('base_ancestry', result.stderr)

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



    def reviewed_fixture(self):
        # Actual immutable reviewed blobs, copied into a disposable Git history.
        for path, before, after in REVIEWED_FIXTURES:
            if before:
                self.write(path, subprocess.check_output(
                    ["git", "-C", str(CHECKER.parents[2]), "cat-file", "blob", before]).decode())
        self.base = self.commit("original reviewed transition inputs")
        for path, before, after in REVIEWED_FIXTURES:
            self.write(path, subprocess.check_output(
                ["git", "-C", str(CHECKER.parents[2]), "cat-file", "blob", after]).decode())

    def test_exact_reviewed_native_and_rollback_transitions_pass(self):
        self.reviewed_fixture()
        head = self.commit("reviewed Native and rollback repairs")
        result = self.check(self.merge_commit(), head)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_reviewed_transition_does_not_allow_content_mode_or_path_drift(self):
        self.reviewed_fixture()
        good = self.commit("reviewed repairs")
        for path, before, after in REVIEWED_FIXTURES:
            for mutation in ("content", "mode", "rename", "delete"):
                with self.subTest(path=path, mutation=mutation):
                    self.git("checkout", "-q", good)
                    target = self.repo / path
                    if mutation == "content":
                        self.write(path, target.read_text() + "\n// altered reviewed source\n")
                    elif mutation == "mode":
                        self.git("update-index", "--chmod=+x", path)
                        target.chmod(0o755)
                    elif mutation == "rename":
                        self.git("mv", path, path + ".renamed")
                    else:
                        self.git("rm", path)
                    if mutation != "mode": self.git("add", ".")
                    self.git("commit", "-qm", mutation)
                    result = self.check()
                    self.assertNotEqual(result.returncode, 0, result.stdout)
                    self.assertIn("reviewed_transition:", result.stderr)
        self.git("checkout", "-q", good)

    def test_reviewed_transitions_do_not_allow_wrong_before_or_sibling_activation(self):
        self.reviewed_fixture()
        good = self.commit("reviewed repairs")
        for path, before, after in REVIEWED_FIXTURES:
            with self.subTest(path=path):
                self.git("checkout", "-q", self.base)
                prior = (self.repo / path).read_text() if before else "package main\n"
                self.write(path, prior + "\n// other input\n")
                wrong = self.commit("unreviewed baseline")
                for candidate, old_blob, new_blob in REVIEWED_FIXTURES:
                    self.write(candidate, subprocess.check_output(
                        ["git", "-C", str(CHECKER.parents[2]), "cat-file", "blob", new_blob]).decode())
                self.commit("same output from wrong baseline")
                result = self.check(base=wrong)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("reviewed_transition:", result.stderr)
        self.git("checkout", "-q", good)
        self.write("src/backend/voice/internal/roomlifecycle/worker.go",
                   "package roomlifecycle\nfunc boot() { NewLifecycleWorker() }\n")
        self.commit("sibling activation")
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("source_disabled_surface:", result.stderr)

if __name__ == '__main__':
    unittest.main()
