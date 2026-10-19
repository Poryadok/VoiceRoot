"""Real producer regressions, using only private Git fixture authority."""
import hashlib
import json
from pathlib import Path
import re
import shlex
import subprocess
import tempfile
import unittest

import f13_native_p3_source as native

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh'
VERIFIER = ROOT / native.VERIFIER_PATH
MAIN = 'src/backend/voice/main.go'


def git(repo, *args, check=True):
    r = subprocess.run(['git', '-C', str(repo), *args], capture_output=True, text=True)
    if check and r.returncode:
        raise AssertionError(r.stderr)
    return r.stdout.strip()


class NativeHistory(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.folder = Path(self.tmp.name)
        self.repo = self.folder / 'repo'
        self.repo.mkdir()
        git(self.repo, 'init', '-q', '-b', 'develop')
        git(self.repo, 'config', 'user.name', 'Native fixture')
        git(self.repo, 'config', 'user.email', 'fixture@example.invalid')
        self.write(MAIN, 'base\n')
        self.write('build.cfg', 'context\n')
        self.base = self.commit('base')
        git(self.repo, 'checkout', '-qb', 'source-a')
        self.write(MAIN, 'source A\n')
        self.a = self.commit('source A')
        git(self.repo, 'checkout', '-q', 'develop')
        git(self.repo, 'merge', '--no-ff', '-qm', 'import A', self.a)
        self.owner_a = git(self.repo, 'rev-parse', 'HEAD')
        git(self.repo, 'checkout', '-qb', 'source-b', self.base)
        self.write(MAIN, 'source B\n')
        self.b = self.commit('source B')
        git(self.repo, 'checkout', '-q', 'develop')
        git(self.repo, 'merge', '--no-ff', '-m', 'import conflicting B', self.b, check=False)
        self.write(MAIN, 'temporary restrictive result\n')
        self.integration = self.commit('real pre H conflict resolution')
        self.write(MAIN, 'reviewed dormant restrictive H\n')
        self.head = self.commit('real exact H restoration')
        self.freeze()
        self.before = {p: hashlib.sha256(p.read_bytes()).hexdigest() for p in (SCRIPT, VERIFIER)}

    def tearDown(self):
        for p, value in self.before.items():
            self.assertEqual(hashlib.sha256(p.read_bytes()).hexdigest(), value, 'fixture changed production source')

    def write(self, name, data):
        p = self.repo / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(data)

    def commit(self, message):
        git(self.repo, 'add', '.')
        git(self.repo, 'commit', '--allow-empty', '-qm', message)
        return git(self.repo, 'rev-parse', 'HEAD')

    def freeze(self):
        record = {
            'version': 'NATIVE-P3-SOURCE-v1', 'base': self.base, 'source': self.head,
            'classification': 'DORMANT-A3-RESTRICTIVE-P3-SOURCE',
            'consumers': ['DEVELOP-SOURCE-v1', 'MASTER-SOURCE-v1'],
            'projection': [{'path': MAIN, 'before': native.entry(str(self.repo), self.base, MAIN),
                            'after': native.entry(str(self.repo), self.head, MAIN)}],
            'context': [{'path': 'build.cfg', 'entry': native.entry(str(self.repo), self.head, 'build.cfg')}],
            'context_only_changed_generated': [], 'closure_review': 'https://example.invalid/closure',
            'down_review': 'https://example.invalid/down', 'source_proposal_sha256': '1' * 64,
            'evidence': [{'name': 'synthetic-fixture-only', 'sha256': '2' * 64}], 'excluded_authority': ['activation'],
            'evidence_matrix': {k: {'result': 'PASS', 'evidence': 'fixture ' + k, 'review': 'fixture'} for k in native.EVIDENCE_CASES},
        }
        raw = json.dumps(record, indent=2) + '\n'
        self.write(native.RECORD_PATH, raw)
        self.record = self.commit('immutable private fixture record')
        code = VERIFIER.read_text()
        pins = {'PINNED_BASE': self.base, 'PINNED_SOURCE': self.head,
                'PINNED_RECORD_COMMIT': self.record, 'PINNED_RECORD_SHA256': hashlib.sha256(raw.encode()).hexdigest()}
        for key, value in pins.items():
            code = re.sub(r'^' + key + r' = ".*".*$', key + ' = "' + value + '"', code, flags=re.M)
        self.write(native.VERIFIER_PATH, code)
        self.write(native.SHELL_PATH, SCRIPT.read_text())
        self.target = self.commit('later private verifier carrier')

    def collect(self, source=None, target=None, event='push', ref='refs/heads/develop', before=None):
        out = self.folder / ('run-' + str(len(list(self.folder.iterdir()))))
        out.mkdir()
        source = source or self.target
        target = target or source
        git(self.repo, 'checkout', '-q', target)
        # Only function definitions are imported; production authority constants
        # and files remain unchanged. Fixture R/K pins live solely in its repo.
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
            'f13_collect_master_release_pr_contributions', 'f13_collect_master_push_contributions',
            'f13_master_merge_push_enabled', 'f13_master_push_before_matches', 'game_checkpoint_aggregate_authorized',
            'f13_migration_alias_reconciles_guarded_path', 'f13_migration_alias_record_load',
        }
        source_functions = SCRIPT.read_text() + '\n' + (ROOT / 'scripts/ci/voice-r22-runtime-scope.sh').read_text()
        funcs = '\n'.join(m.group(0) for m in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n', source_functions)
            if m.group(1) in selected or m.group(1).startswith('native_p3_'))
        program = 'set -eu\n' + funcs + '\n'
        values = {'ROOT': str(self.repo), 'TMP_DIR': str(out), 'base_sha': self.base,
                  'GITHUB_EVENT_NAME': event, 'GITHUB_REF': ref, 'GITHUB_SHA': target,
                  'VOICE_CI_PUSH_BEFORE_SHA': before or self.base, 'GITHUB_BASE_REF': 'master',
                  'native_p3_active': 'false', 'native_p3_source': '', 'native_p3_target': '',
                  'native_p3_paths': '', 'native_p3_binding': '', 'native_p3_paths_hash': '', 'native_p3_binding_hash': '',
                  'native_p3_snapshot': '', 'game_checkpoint_aggregate_active': 'false',
                  'game_checkpoint_snapshot_base': '', 'game_checkpoint_snapshot_head': '', 'game_checkpoint_snapshot_record': '',
                  'game_checkpoint_aggregate_head': '', 'game_checkpoint_aggregate_paths': '', 'game_checkpoint_aggregate_record': '',
                  'game_checkpoint_aggregate_map': '', 'game_checkpoint_aggregate_alias_map': ''}
        program += '\n'.join(k + '=' + shlex.quote(v) for k, v in values.items()) + '\n'
        program += 'native_p3_prepare\n'
        if ref == 'refs/heads/develop':
            program += f'f13_collect_develop_contributions "$ROOT" {self.base} {target} "$TMP_DIR/f13-contributions" "$TMP_DIR/category"\n'
        elif event == 'push':
            program += f'f13_collect_master_push_contributions "$ROOT" push refs/heads/master {before or self.base} {self.base} {target} "$TMP_DIR/f13-contributions" "$TMP_DIR/category"\n'
        else:
            program += f'f13_collect_master_release_pr_contributions "$ROOT" {self.base} {target} "$TMP_DIR/f13-contributions" "$TMP_DIR/category"\n'
        program += 'native_p3_audit_map\nnative_p3_path_allowed_for_provenance "$ROOT" "$TMP_DIR/f13-contributions" ' + MAIN + '\n'
        harness = out / 'harness.sh'
        harness.write_text(program)
        r = subprocess.run(['bash', str(harness)], capture_output=True, text=True)
        rows = (out / 'f13-contributions').read_text().splitlines() if (out / 'f13-contributions').exists() else []
        return r, rows, out

    def test_real_conflict_then_direct_h_restoration_preserves_history_and_trigger(self):
        r, rows, out = self.collect()
        self.assertEqual(r.returncode, 0, r.stderr)
        row = next(line.split('\t') for line in rows if line.split('\t')[0] == MAIN)
        self.assertEqual(row[1], 'true')
        self.assertEqual(row[2], self.head)
        self.assertEqual(row[5], self.head)
        self.assertEqual(row[6], self.base)
        history = (out / 'f13-native-p3-history.tsv').read_text()
        self.assertIn(self.owner_a, history)
        self.assertIn(self.b, history)
        self.assertIn(self.integration, history)
        self.assertIn('guarded_conflict\t2', history)
        self.assertIn('direct\t0', history)

    def test_real_master_source_route_preserves_before_two_parents_and_result(self):
        target = git(self.repo, 'commit-tree', self.target + '^{tree}', '-p', self.base, '-p', self.target, '-m', 'release')
        for event, ref in (('pull_request', 'refs/pull/875/merge'), ('push', 'refs/heads/master')):
            with self.subTest(event=event):
                r, rows, _ = self.collect(target=target, event=event, ref=ref)
                self.assertEqual(r.returncode, 0, r.stderr)
                self.assertTrue(any(line.startswith(MAIN + '\ttrue\t') for line in rows))

    def test_missing_h_restoration_wrong_before_and_post_h_delta_are_rejected(self):
        r, _, _ = self.collect(target=self.integration)
        self.assertNotEqual(r.returncode, 0)
        git(self.repo, 'checkout', '-q', self.target)
        self.write(MAIN, 'post H activation\n')
        changed = self.commit('unreviewed post H content')
        r, _, _ = self.collect(target=changed)
        self.assertNotEqual(r.returncode, 0)
        target = git(self.repo, 'commit-tree', self.target + '^{tree}', '-p', self.base, '-p', self.target, '-m', 'release')
        r, _, _ = self.collect(target=target, event='push', ref='refs/heads/master', before=self.head)
        self.assertNotEqual(r.returncode, 0)

    def test_foreign_native_like_path_does_not_gain_projection_authority(self):
        self.write('src/backend/voice/foreign.go', 'unreviewed NewLifecycleWorker()\n')
        self.commit('foreign native source')
        result, _, _ = self.collect(target=git(self.repo, 'rev-parse', 'HEAD'))
        self.assertNotEqual(result.returncode, 0)

    def test_actual_producer_map_missing_duplicate_forged_and_cleared_taint_reject(self):
        result, rows, out = self.collect()
        self.assertEqual(result.returncode, 0, result.stderr)
        binding = json.loads((out / 'f13-native-p3-approval.json').read_text())['binding']
        original = (out / 'f13-contributions').read_text()
        history = (out / 'f13-native-p3-history.tsv').read_text()
        selected = next(row for row in rows if row.startswith(MAIN + '\t'))
        command = ['python3', str(self.repo / native.VERIFIER_PATH), '--repo', str(self.repo),
                   '--base', self.base, '--source', binding['source'], '--target', binding['target'],
                   '--event', 'push', '--ref', 'refs/heads/develop', '--event-sha', binding['target'],
                   '--before', self.base, '--base-ref', '', '--map', str(out / 'f13-contributions'),
                   '--history', str(out / 'f13-native-p3-history.tsv'), '--output', str(out / 'tamper-result.json')]
        for case in ('missing', 'duplicate', 'trigger', 'owner', 'source', 'content', 'map-and-history-trigger'):
            with self.subTest(case=case):
                fields = selected.split('\t')
                if case == 'missing':
                    value = ''
                elif case == 'duplicate':
                    value = original + selected + '\n'
                else:
                    index = {'trigger': 1, 'owner': 2, 'source': 5, 'content': 6, 'map-and-history-trigger': 1}[case]
                    fields[index] = 'false' if index == 1 else self.head if case == 'content' else self.base
                    value = '\t'.join(fields) + '\n'
                (out / 'f13-contributions').write_text(value)
                (out / 'f13-native-p3-history.tsv').write_text(
                    history.replace('\ttrue\t', '\tfalse\t') if case == 'map-and-history-trigger' else history)
                result = subprocess.run(command, capture_output=True, text=True)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
