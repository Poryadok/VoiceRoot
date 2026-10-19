"""Fixed historical conflict authority; all synthetic pins stay in private Git."""
import hashlib
import json
from pathlib import Path
import re
import shlex
import subprocess
import tempfile
import unittest

import f13_reviewed_test_merge as reviewed
from f13_guarded_merge_test import definitions

VERIFIER = Path(reviewed.__file__)
TEST = 'src/backend/role/permissions/permissions_test.go'


def git(repo, *args, check=True):
    r = subprocess.run(['git', '-C', str(repo), *args], capture_output=True, text=True)
    if check and r.returncode:
        raise AssertionError(r.stderr)
    return r.stdout.strip()


class Conflict:
    def __init__(self, root, path=TEST, extra_conflict=False):
        self.repo = root
        self.path = path
        self.repo.mkdir()
        git(root, 'init', '-q', '-b', 'first')
        git(root, 'config', 'user.name', 'Reviewed conflict fixture')
        git(root, 'config', 'user.email', 'fixture@example.invalid')
        self.write(path, 'package p\n\nfunc TestBase() {}\n')
        if extra_conflict:
            self.write('src/backend/role/runtime.go', 'package p\n//base\n')
        self.base = self.commit('base')
        self.write(path, 'package p\n\nfunc TestFirst() {}\nfunc TestBase() {}\n')
        if extra_conflict:
            self.write('src/backend/role/runtime.go', 'package p\n// first\n')
        self.first = self.commit('first source')
        git(root, 'checkout', '-qb', 'source', self.base)
        self.write(path, 'package p\n\nfunc TestSource() {}\nfunc TestBase() {}\n')
        if extra_conflict:
            self.write('src/backend/role/runtime.go', 'package p\n// second\n')
        self.source = self.commit('second source')
        git(root, 'checkout', '-q', self.first)
        git(root, 'merge', '--no-ff', '-m', 'conflict', self.source, check=False)
        self.write(path, 'package p\n\nfunc TestFirst() {}\nfunc TestSource() {}\nfunc TestBase() {}\n')
        if extra_conflict:
            self.write('src/backend/role/runtime.go', 'package p\n// invented conflict resolution\n')
        self.merge = self.commit('actual historical conflict result')
        self.data = {'version': 'F13-REVIEWED-TEST-MERGE-v1', 'consumer': 'HISTORICAL-TEST-MERGE-ONLY',
                     'merge': self.merge, 'parents': [self.first, self.source], 'best_bases': [self.base],
                     'conflicts': [{'path': path, 'base': self.entry(self.base, path), 'first': self.entry(self.first, path),
                                    'source': self.entry(self.source, path), 'result': self.entry(self.merge, path)}],
                     'review': 'https://example.invalid/independent-resolution-review',
                     'evidence': [{'name': 'synthetic-fixture-only', 'sha256': '1' * 64}],
                     'exclusions': ['No current or future path exemption']}
        self.freeze()

    def write(self, name, text):
        p = self.repo / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text)

    def commit(self, message):
        git(self.repo, 'add', '.')
        git(self.repo, 'commit', '--allow-empty', '-qm', message)
        return git(self.repo, 'rev-parse', 'HEAD')

    def entry(self, revision, path):
        return reviewed.entry(self.repo, revision, path)

    def freeze(self):
        raw = json.dumps(self.data, indent=2) + '\n'
        self.write(reviewed.RECORD_PATH, raw)
        self.record = self.commit('immutable reviewed record')
        code = VERIFIER.read_text()
        for key, value in {'PINNED_RECORD_COMMIT': self.record,
                           'PINNED_RECORD_SHA256': hashlib.sha256(raw.encode()).hexdigest()}.items():
            code = re.sub(r'^' + key + r' = ".*"$', key + ' = "' + value + '"', code, flags=re.M)
        self.write(reviewed.VERIFIER_PATH, code)
        self.target = self.commit('later fixed pin carrier')

    def run(self, **overrides):
        values = {'repo': str(self.repo), 'target': self.target, 'merge': self.merge, 'first': self.first,
                  'source-base': self.base, 'source': self.source, 'path': self.path}
        values.update(overrides)
        args = ['python3', str(self.repo / reviewed.VERIFIER_PATH)]
        for key, value in values.items():
            args.extend(['--' + key, value])
        return subprocess.run(args, capture_output=True, text=True)


class ReviewedMergeTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.before = hashlib.sha256(VERIFIER.read_bytes()).hexdigest()

    def tearDown(self):
        self.assertEqual(hashlib.sha256(VERIFIER.read_bytes()).hexdigest(), self.before)

    def fixture(self, **kwargs):
        return Conflict(self.root / str(len(list(self.root.iterdir()))), **kwargs)

    def test_exact_historical_tuple_qualifies_provenance_only(self):
        h = self.fixture()
        r = h.run()
        self.assertEqual(r.returncode, 0, r.stderr)
        proof = json.loads(r.stdout)
        self.assertTrue(proof['historical_only'])
        self.assertTrue(proof['retain_runtime_and_content_taint'])
        self.assertEqual(proof['source'], h.source)

    def test_real_shell_consumer_preserves_historical_source_and_full_content(self):
        h = self.fixture()
        out = self.root / 'shell-run'
        out.mkdir()
        (out / 'game-checkpoint-tree').write_text('')
        (out / 'be255-checkpoint-tree').write_text('')
        values = {'ROOT': str(h.repo), 'TMP_DIR': str(out), 'GITHUB_EVENT_NAME': 'pull_request',
                  'GITHUB_REF': 'refs/pull/875/merge', 'game_checkpoint_aggregate_active': 'false',
                  'game_checkpoint_snapshot_base': '', 'game_checkpoint_snapshot_head': '',
                  'game_checkpoint_snapshot_record': '', 'game_checkpoint_aggregate_head': '',
                  'game_checkpoint_aggregate_paths': '', 'native_p3_active': 'false'}
        program = 'set -eu\n' + definitions() + '\n'
        program += '\n'.join(k + '=' + shlex.quote(v) for k, v in values.items()) + '\n'
        program += f'f13_collect_develop_contributions "$ROOT" {h.base} {h.target} "$TMP_DIR/map" "$TMP_DIR/category" true true\n'
        program += 'f13_changed_content "$ROOT" "$TMP_DIR/map" ' + TEST + ' >"$TMP_DIR/content"\n'
        harness = out / 'consumer.sh'
        harness.write_text(program)
        result = subprocess.run(['bash', str(harness)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        row = next(line.split('\t') for line in (out / 'map').read_text().splitlines() if line.startswith(TEST + '\t'))
        self.assertEqual(row[2:7], [h.merge, h.first, h.base, h.source, h.base])
        content = (out / 'content').read_text()
        self.assertIn('TestFirst', content)
        self.assertIn('TestSource', content)
        proof = json.loads((out / 'f13-reviewed-test-merges.jsonl').read_text())
        self.assertEqual(proof['record'], h.record)
        self.assertTrue(proof['retain_runtime_and_content_taint'])

    def test_wrong_path_parent_source_base_future_merge_and_missing_carrier_reject(self):
        h = self.fixture()
        cases = [{'path': 'src/backend/role/other_test.go'}, {'first': h.source}, {'source': h.first},
                 {'source-base': h.first}, {'merge': h.target}, {'target': h.record}, {'target': h.merge}]
        for case in cases:
            with self.subTest(case=case):
                self.assertEqual(h.run(**case).returncode, 1)

    def test_wrong_missing_record_hash_and_caller_record_override_reject(self):
        h = self.fixture()
        p = h.repo / reviewed.VERIFIER_PATH
        original = p.read_text()
        for key, value in (('PINNED_RECORD_COMMIT', '0' * 40), ('PINNED_RECORD_SHA256', '0' * 64)):
            with self.subTest(key=key):
                p.write_text(re.sub(r'^' + key + r' = ".*"$', key + ' = "' + value + '"', original, flags=re.M))
                self.assertEqual(h.run().returncode, 1)
        p.write_text(original)
        command = ['python3', str(p), '--record', h.record]
        self.assertNotEqual(subprocess.run(command, capture_output=True).returncode, 0)

    def test_record_entry_mode_oid_extra_scope_and_unreviewed_status_reject(self):
        for case in ('mode', 'oid', 'extra', 'review'):
            with self.subTest(case=case):
                h = self.fixture()
                if case in ('mode', 'oid'):
                    h.data['conflicts'][0]['result'][case] = '100755' if case == 'mode' else '0' * 40
                elif case == 'extra':
                    h.data['conflicts'].append(dict(h.data['conflicts'][0]))
                else:
                    h.data['review'] = 'PENDING'
                h.freeze()
                self.assertEqual(h.run().returncode, 1)

    def test_non_test_resolution_and_unrecorded_runtime_conflict_reject(self):
        for h in (self.fixture(path='src/backend/role/runtime.go'), self.fixture(extra_conflict=True)):
            with self.subTest(path=h.path, merge=h.merge):
                self.assertEqual(h.run().returncode, 1)


if __name__ == '__main__':
    unittest.main()
