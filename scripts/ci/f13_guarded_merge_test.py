"""Real guarded three-way merge provenance, with no checkpoint exemption."""
import hashlib
from pathlib import Path
import re
import shlex
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh'
PATH = 'src/backend/role/internal/store/roles_custom_integration_test.go'
RUNTIME = 'src/backend/voice/main.go'


def git(repo, *args, check=True):
    r = subprocess.run(['git', '-C', str(repo), *args], capture_output=True, text=True)
    if check and r.returncode:
        raise AssertionError(r.stderr)
    return r.stdout.strip()


def definitions():
    selected = {
        'f13_collect_develop_contributions', 'f13_path_is_guarded', 'f13_resolve_source_base',
        'f13_collect_failure', 'f13_emit_guarded_failure_diagnostic', 'f13_tree_entry_identity',
        'f13_guarded_entry_matches_final_target', 'game_checkpoint_contribution_allowed',
        'game_checkpoint_aggregate_path_allowed', 'game_checkpoint_aggregate_binding_valid',
        'game_checkpoint_path_authorized_in_delta', 'be255_checkpoint_path_authorized_in_delta',
        'game_checkpoint_domain_path', 'voice_r22_runtime_changed', 'f13_develop_aggregate_enabled',
        'f13_guarded_correction_qualified', 'f13_validate_sync_merge_resolution',
        'f13_sync_boundary_reconciles_guarded_path', 'f13_checkpoint_merge_tree_entry_matches',
        'f13_tree_entry_tuple_for_path', 'f13_tree_entry_from_records', 'f13_tree_entry_raw_for_path',
        'f13_git_tree_entry_record', 'f13_git_tree_entry', 'game_checkpoint_duplicate_owner_supersession_allowed',
        'f13_changed_content', 'f13_migration_alias_reconciles_guarded_path',
        'f13_content_base_for_path', 'f13_source_head_for_path',
        'f13_reviewed_test_merge_qualified',
    }
    source = SCRIPT.read_text() + '\n' + (ROOT / 'scripts/ci/voice-r22-runtime-scope.sh').read_text()
    return '\n'.join(m.group(0) for m in re.finditer(
        r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n', source)
        if m.group(1) in selected)


class GuardedMerge(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.folder = Path(self.tmp.name)
        self.repo = self.folder / 'repo'
        self.repo.mkdir()
        git(self.repo, 'init', '-q', '-b', 'lane')
        git(self.repo, 'config', 'user.name', 'Merge fixture')
        git(self.repo, 'config', 'user.email', 'fixture@example.invalid')
        self.text = ['left base'] + ['stable ' + str(i) for i in range(20)] + ['right base']
        self.write(PATH, '\n'.join(self.text) + '\n')
        self.base = self.commit('base')
        left = self.text.copy()
        left[0] = 'retained first-parent source'
        self.write(PATH, '\n'.join(left) + '\n')
        self.write(RUNTIME, 'package main\n// retained runtime source\n')
        self.left = self.commit('first-parent source with runtime taint')
        git(self.repo, 'checkout', '-qb', 'source', self.base)
        right = self.text.copy()
        right[-1] = 'incoming reviewed source'
        self.write(PATH, '\n'.join(right) + '\n')
        self.source = self.commit('independent source')
        git(self.repo, 'checkout', '-q', 'lane')
        git(self.repo, 'merge', '--no-ff', '-qm', 'clean actual union', self.source)
        self.merged = git(self.repo, 'rev-parse', 'HEAD')
        self.initial_hash = hashlib.sha256(SCRIPT.read_bytes()).hexdigest()

    def tearDown(self):
        self.assertEqual(hashlib.sha256(SCRIPT.read_bytes()).hexdigest(), self.initial_hash)

    def write(self, name, text):
        p = self.repo / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text)

    def commit(self, message):
        git(self.repo, 'add', '.')
        git(self.repo, 'commit', '-qm', message)
        return git(self.repo, 'rev-parse', 'HEAD')

    def collect(self, target=None):
        out = self.folder / ('run-' + str(len(list(self.folder.iterdir()))))
        out.mkdir()
        (out / 'game-checkpoint-tree').write_text('')
        (out / 'be255-checkpoint-tree').write_text('')
        target = target or self.merged
        values = {'ROOT': str(self.repo), 'TMP_DIR': str(out), 'GITHUB_EVENT_NAME': 'pull_request',
                  'GITHUB_REF': 'refs/pull/875/merge', 'game_checkpoint_aggregate_active': 'false',
                  'game_checkpoint_snapshot_base': '', 'game_checkpoint_snapshot_head': '',
                  'game_checkpoint_snapshot_record': '', 'game_checkpoint_aggregate_head': '',
                  'game_checkpoint_aggregate_paths': '', 'native_p3_active': 'false'}
        program = 'set -eu\n' + definitions() + '\n'
        program += '\n'.join(k + '=' + shlex.quote(v) for k, v in values.items()) + '\n'
        program += f'f13_collect_develop_contributions "$ROOT" {self.base} {target} "$TMP_DIR/map" "$TMP_DIR/category" true true\n'
        program += 'f13_changed_content "$ROOT" "$TMP_DIR/map" ' + PATH + ' >"$TMP_DIR/content"\n'
        harness = out / 'collector.sh'
        harness.write_text(program)
        r = subprocess.run(['bash', str(harness)], capture_output=True, text=True)
        return r, out

    def test_clean_union_has_actual_result_owner_retains_both_sources_and_runtime(self):
        result, out = self.collect()
        category = (out / 'category').read_text() if (out / 'category').exists() else 'no category'
        self.assertEqual(result.returncode, 0, result.stderr + category)
        row = next(line.split('\t') for line in (out / 'map').read_text().splitlines() if line.startswith(PATH + '\t'))
        self.assertEqual(row[1], 'true')
        self.assertEqual(row[2:7], [self.merged, self.left, self.base, self.source, self.base])
        content = (out / 'content').read_text()
        self.assertIn('retained first-parent source', content)
        self.assertIn('incoming reviewed source', content)
        ledger = (out / 'f13-guarded-merge-history.tsv').read_text()
        self.assertIn(self.source, ledger)
        self.assertIn(self.left, ledger)
        self.assertIn(self.base, ledger)
        self.assertIn(self.merged, ledger)

    def test_manual_non_git_result_and_mode_change_reject(self):
        for case in ('bytes', 'mode'):
            with self.subTest(case=case):
                git(self.repo, 'checkout', '-q', self.merged)
                if case == 'bytes':
                    self.write(PATH, 'invented merge-only content\n')
                else:
                    (self.repo / PATH).chmod(0o755)
                git(self.repo, 'add', PATH)
                tree = git(self.repo, 'write-tree')
                target = git(self.repo, 'commit-tree', tree, '-p', self.left, '-p', self.source, '-m', 'unreviewed result')
                result, out = self.collect(target)
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertEqual((out / 'category').read_text().strip(), 'merge_tree_mismatch')

    def test_genuine_conflict_does_not_gain_clean_merge_authority(self):
        git(self.repo, 'checkout', '-qb', 'conflicting', self.base)
        self.write(PATH, 'conflicting incoming source\n')
        conflict = self.commit('conflict source')
        git(self.repo, 'checkout', '-q', self.left)
        git(self.repo, 'merge', '--no-ff', '-m', 'conflict', conflict, check=False)
        self.write(PATH, 'manual conflict resolution\n')
        resolved = self.commit('unqualified conflict resolution')
        result, out = self.collect(resolved)
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertEqual((out / 'category').read_text().strip(), 'merge_tree_mismatch')


if __name__ == '__main__':
    unittest.main()
