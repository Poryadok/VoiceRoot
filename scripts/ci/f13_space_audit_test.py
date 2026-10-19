"""One observed grpc tuple; unobserved store must remain fail closed."""
import hashlib
import json
from pathlib import Path
import re
import shlex
import subprocess
import tempfile
import unittest

from f13_guarded_merge_test import definitions
from f13_reviewed_test_merge_test import git

ROOT = Path(__file__).resolve().parents[2]
HELPER = 'scripts/ci/f13_space_audit.py'
RECORD = 'docs/testing/space-audit-grpc-observed-v1.json'
SHELL = ROOT / 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh'
PATHS = ['src/backend/space/internal/grpcsvc/audit_ingestion_test.go',
         'src/backend/space/internal/store/audit_ingestion_test.go']


def raw(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args])


def entry(repo, rev, path):
    mode, kind, value = git(repo, 'ls-tree', rev, '--', path).split('\t')[0].split()
    return dict(mode=mode, type=kind, oid=value)


class History:
    def __init__(self, repo):
        self.repo = repo; repo.mkdir()
        git(repo, 'init', '-q', '-b', 'lane')
        git(repo, 'config', 'user.name', 'Fixed Space audit fixture')
        git(repo, 'config', 'user.email', 'fixture@example.invalid')
        for path in PATHS: self.write(path, 'reviewed first fixture\n')
        self.shared = self.commit('older incoming shared base')
        for path in PATHS: self.write(path, 'initial fixture\n')
        self.base = self.commit('later event traversal base')
        self.write('README', 'earlier first parent\n')
        self.old_first = self.commit('earlier parent')
        git(repo, 'checkout', '-qb', 'old-source', self.base)
        for path in PATHS: self.write(path, 'reviewed first fixture\n')
        self.old_source = self.commit('original incoming runtime producer')
        git(repo, 'checkout', '-q', self.old_first)
        git(repo, 'merge', '--no-ff', '-qm', 'prior integration', self.old_source)
        self.first = git(repo, 'rev-parse', 'HEAD')
        git(repo, 'checkout', '-qb', 'incoming', self.shared)
        for path in PATHS: self.write(path, 'reviewed incoming fixture29or31\n')
        self.incoming = self.commit('later reviewed test fixture edit')
        git(repo, 'checkout', '-q', self.first)
        git(repo, 'merge', '--no-ff', '-qm', 'actual duplicate integration', self.incoming)
        self.merge = git(repo, 'rev-parse', 'HEAD')
        self.write('README', 'current fixed source boundary\n')
        self.source = self.commit('current source')
        self.data = json.loads((ROOT / RECORD).read_text())
        r = self.data; r['source'], r['base'] = self.source, self.base
        for index, case in enumerate(r['cases']):
            path = PATHS[index]
            case.update(merge=self.merge, parents=[self.first, self.incoming], best_bases=[self.shared])
            case['actual_prior_row'] = [path, 'false', self.first, self.old_first, self.base,
                                        self.old_source, self.old_first, self.base]
            # This private incoming source has no earlier runtime history;
            # its nested producer emits false independently for both paths.
            case['actual_incoming_row'] = [path, 'false', self.merge,
                                           self.first, self.shared, self.incoming, self.old_first, self.shared]
            for label, rev in dict(base=self.shared, first=self.first, incoming=self.incoming, result=self.merge,
                                   prior_owner=self.first, prior_source=self.old_source, current=self.source).items():
                data = raw(repo, 'show', rev + ':' + path)
                case['entries'][label] = dict(revision=rev, entry=entry(repo, rev, path),
                    sha256=hashlib.sha256(data).hexdigest(), artifact=path.replace('/', '__') + '__' + label, bytes=len(data))
        r['observation']['source'] = self.source
        self.original_case = json.loads(json.dumps(r['cases'][0]))
        self.freeze()

    def write(self, path, text):
        p = self.repo / path; p.parent.mkdir(parents=True, exist_ok=True); p.write_text(text)

    def commit(self, message):
        git(self.repo, 'add', '.'); git(self.repo, 'commit', '--allow-empty', '-qm', message)
        return git(self.repo, 'rev-parse', 'HEAD')

    def freeze(self):
        data = json.dumps(self.data, indent=2) + '\n'; self.write(RECORD, data)
        self.record = self.commit('immutable reviewed observed grpc record')
        code = (ROOT / HELPER).read_text() if (ROOT / HELPER).exists() else '# not implemented\n'
        for name, value in dict(PINNED_RECORD_COMMIT=self.record, PINNED_RECORD_SHA256=hashlib.sha256(data.encode()).hexdigest(),
                                PINNED_SOURCE=self.source, PINNED_BASE=self.base).items():
            code = re.sub(r'^' + name + r' = ".*"$', name + ' = "' + value + '"', code, flags=re.M)
        self.write(HELPER, code); self.target = self.commit('later fixed pin carrier')

    def run(self, index=0, **overrides):
        c = self.data['cases'][index] if self.data['cases'] else self.original_case
        args = dict(repo=str(self.repo), target=self.target,
                    **{'prior-row': '\t'.join(c['actual_prior_row']), 'incoming-row': '\t'.join(c['actual_incoming_row'])})
        args.update(overrides); command = ['python3', str(self.repo / HELPER)]
        for key, value in args.items(): command += ['--' + key, value]
        return subprocess.run(command, capture_output=True, text=True)


class SpaceAuditTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup); self.folder = Path(self.tmp.name)
        self.before = {p: hashlib.sha256(p.read_bytes()).hexdigest() for p in (SHELL, ROOT / HELPER) if p.exists()}

    def tearDown(self):
        for p, sha in self.before.items(): self.assertEqual(hashlib.sha256(p.read_bytes()).hexdigest(), sha)

    def fixture(self): return History(self.folder / str(len(list(self.folder.iterdir()))))

    def test_original_shell_keeps_both_grpc_rows_or_false_then_rejects_store(self):
        h = self.fixture(); out = self.folder / 'shell'; out.mkdir()
        for name in ('game-checkpoint-tree', 'be255-checkpoint-tree'): (out / name).write_text('')
        extra = '\n'.join(m.group(0) for m in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n', SHELL.read_text())
            if m.group(1) == 'f13_space_audit_qualified')
        values = dict(ROOT=str(h.repo), TMP_DIR=str(out), GITHUB_EVENT_NAME='pull_request', GITHUB_REF='refs/pull/875/merge',
            native_p3_active='false', game_checkpoint_aggregate_active='false', game_checkpoint_snapshot_base='',
            game_checkpoint_snapshot_head='', game_checkpoint_snapshot_record='', game_checkpoint_aggregate_head='', game_checkpoint_aggregate_paths='')
        program = 'set -eu\n' + definitions() + '\n' + extra + '\n'
        program += '\n'.join(k + '=' + shlex.quote(v) for k, v in values.items()) + '\n'
        program += f'f13_collect_develop_contributions "$ROOT" {h.base} {h.target} "$TMP_DIR/map" "$TMP_DIR/category" true true\n'
        result = subprocess.run(['bash', '-c', program], capture_output=True, text=True)
        category = (out / 'category').read_text() if (out / 'category').exists() else ''
        self.assertEqual(result.returncode, 1, result.stderr + category)
        self.assertEqual(category.strip(), 'duplicate_guarded_owner')
        self.assertIn(PATHS[1], result.stderr)
        expected = []
        for case in h.data['cases']:
            expected += ['prior\t' + '\t'.join(case['actual_prior_row']), 'incoming\t' + '\t'.join(case['actual_incoming_row'])]
        self.assertEqual((out / 'f13-space-audit-history.tsv').read_text().splitlines(), expected)
        rows = [v.split('\t') for v in (out / 'map').read_text().splitlines() if v.split('\t')[0] in PATHS]
        self.assertEqual([v[1] for v in rows], ['false', 'false'])
        self.assertEqual(next(v for v in rows if v[0] == PATHS[0])[2], h.merge)
        self.assertEqual(next(v for v in rows if v[0] == PATHS[1])[2], h.first)
        receipts = [json.loads(v) for v in (out / 'f13-space-audit.jsonl').read_text().splitlines()]
        self.assertEqual(len(receipts), 1)
        self.assertEqual([v['incoming_row'][1] for v in receipts], ['false'])
        # A partial fixed record cannot become a generic duplicate allowance.
        previous = h.target; h.data['cases'].pop(); h.freeze()
        bad = subprocess.run(['bash', '-c', program.replace(previous, h.target)], capture_output=True, text=True)
        self.assertEqual(bad.returncode, 1)
        self.assertEqual((out / 'category').read_text().strip(), 'duplicate_guarded_owner')

    def test_one_grpc_loads_seven_raw_bindings_and_ten_provenance_bindings_source_only(self):
        h = self.fixture()
        p = h.run(); self.assertEqual(p.returncode, 0, p.stderr)
        proof = json.loads(p.stdout)
        self.assertEqual(proof['raw_bindings'], 7); self.assertEqual(proof['cases'], 1)
        self.assertEqual(proof['artifact_bindings'], 10)
        self.assertEqual((proof['prior_row'][1], proof['incoming_row'][1]), ('false', 'false'))
        self.assertFalse(proof['runtime_pass']); self.assertTrue(proof['retain_runtime_and_content_taint'])

    def test_unobserved_store_and_forged_true_flags_cannot_gain_authority(self):
        h = self.fixture()
        case = h.data['cases'][0]
        for which in ('prior-row', 'incoming-row'):
            values = case['actual_' + which.replace('-', '_')].copy(); values[0] = PATHS[1]
            self.assertEqual(h.run(**{which: '\t'.join(values)}).returncode, 1)
        for key in ('actual_prior_row', 'actual_incoming_row'):
            h = self.fixture(); h.data['cases'][0][key][1] = 'true'; h.freeze()
            self.assertEqual(h.run().returncode, 1)
        h = self.fixture(); c = h.data['cases'][0]
        c['path'] = c['actual_prior_row'][0] = c['actual_incoming_row'][0] = PATHS[1]
        h.freeze(); self.assertEqual(h.run().returncode, 1)

    def test_wrong_actual_rows_flags_source_metadata_and_future_call_reject(self):
        h = self.fixture()
        for index in (0,):
            for which in ('prior-row', 'incoming-row'):
                key = 'actual_prior_row' if which == 'prior-row' else 'actual_incoming_row'
                for col in range(8):
                    values = h.data['cases'][index][key].copy()
                    values[col] = ('false' if values[col] == 'true' else 'true') if col == 1 else 'wrong'
                    self.assertEqual(h.run(index, **{which: '\t'.join(values)}).returncode, 1)
        self.assertEqual(h.run(target=h.record).returncode, 1)
        self.assertNotEqual(h.run(record=h.record).returncode, 0)

    def test_partial_mode_oid_hash_revision_non_test_and_authority_records_reject(self):
        for case in ('partial', 'extra', 'mode', 'oid', 'sha', 'bytes', 'revision', 'path', 'parent', 'source', 'runtime', 'generic', 'activation', 'observation', 'receipt', 'artifact', 'store_hold'):
            with self.subTest(case=case):
                h = self.fixture(); item = h.data['cases'][-1]
                if case == 'partial': h.data['cases'].pop()
                elif case == 'extra': h.data['cases'].append(json.loads(json.dumps(item)))
                elif case == 'observation': h.data['observation']['actual_exit'] = 0
                elif case == 'receipt': h.data['observation']['receipt_sha256'] = '0' * 64
                elif case == 'artifact': h.data['artifacts'].pop('space-audit-proof/actual-call.stderr')
                elif case == 'store_hold': h.data['unobserved_store_status'] = 'CLEAR'
                elif case in ('mode', 'oid'): item['entries']['result']['entry'][case] = '100755' if case == 'mode' else '0' * 40
                elif case in ('sha', 'bytes', 'revision'): item['entries']['current'][{'sha':'sha256','bytes':'bytes','revision':'revision'}[case]] = -1 if case == 'bytes' else '0' * (64 if case == 'sha' else 40)
                elif case == 'path': item['path'] = 'src/backend/space/runtime.go'
                elif case == 'parent': item['parents'].reverse()
                elif case == 'source': h.data['source'] = h.merge
                else: h.data[{'runtime':'runtime_pass','generic':'generic_path_authority','activation':'activation_authority'}[case]] = True
                h.freeze(); self.assertEqual(h.run().returncode, 1)

    def test_current_test_regression_record_drift_missing_pin_and_future_path_reject(self):
        h = self.fixture(); h.write(PATHS[0], 'unreviewed current fixture\n'); h.target = h.commit('current drift')
        self.assertEqual(h.run().returncode, 1)
        h = self.fixture(); p = h.repo / HELPER; original = p.read_text()
        for key, value in (('PINNED_RECORD_COMMIT', '0' * 40), ('PINNED_RECORD_SHA256', '0' * 64)):
            p.write_text(re.sub(r'^' + key + r' = ".*"$', key + ' = "' + value + '"', original, flags=re.M))
            self.assertEqual(h.run().returncode, 1)
        p.write_text(original); h.write(RECORD, '{}\n'); h.target = h.commit('record drift')
        self.assertEqual(h.run().returncode, 1)


if __name__ == '__main__': unittest.main()
