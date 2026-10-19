"""Complete private historical units; never mutate the production authority."""
import copy
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
HELPER = 'scripts/ci/f13_historical_composition.py'
RECORD = 'docs/testing/master-historical-composition-v1.json'
SHELL = ROOT / 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh'


def rawgit(repo, *args, check=True):
    p = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), *args], capture_output=True)
    if check and p.returncode:
        raise AssertionError(p.stderr)
    return p


def entry(repo, rev, path):
    data = rawgit(repo, 'ls-tree', '-z', rev, '--', path).stdout
    if not data:
        return None
    mode, kind, oid = data.split(b'\t')[0].decode().split()
    return dict(mode=mode, type=kind, oid=oid)


class Unit:
    def __init__(self, repo):
        self.repo = repo
        repo.mkdir()
        git(repo, 'init', '-q', '-b', 'lane')
        git(repo, 'config', 'user.name', 'Historical unit fixture')
        git(repo, 'config', 'user.email', 'fixture@example.invalid')
        self.groups = [[f'src/backend/space/group{n}/file{i}.go' for i in range(count)]
                       for n, count in enumerate((3, 2, 2, 10, 5, 3, 9, 4))]
        self.write('README', 'private fixture only\n')
        for group in self.groups:
            for path in group:
                if path not in self.groups[0][:2]:
                    self.write(path, 'package p\n// base\n')
        self.base = self.commit('base')
        self.rows, self.artifacts = [], {}
        for index, group in enumerate(self.groups):
            base = git(repo, 'rev-parse', 'HEAD')
            for path in group:
                self.write(path, 'package p\n// first\n')
            if index == 0:
                self.write('src/backend/voice/main.go', 'package main\n// retained runtime taint\n')
            first = self.commit('first producer')
            git(repo, 'checkout', '-qb', f'incoming{index}', base)
            for path in group:
                self.write(path, 'package p\n// incoming\n')
            incoming = self.commit('incoming producer')
            git(repo, 'checkout', '-q', first)
            git(repo, 'merge', '--no-ff', '-m', 'conflict', incoming, check=False)
            for path in group:
                self.write(path, 'package p\n// first\n// incoming\n')
            merge = self.commit('reviewed historical result')
            mt = rawgit(repo, 'merge-tree', '--write-tree', first, incoming, check=False)
            prefix = f'historical-composition/{merge[:12]}/'
            self.artifacts[prefix + 'merge-tree.stdout'] = mt.stdout
            self.artifacts[prefix + 'merge-tree.stderr'] = mt.stderr
            row = dict(merge=merge, parents=[first, incoming], best_bases=[base],
                       whole_conflict_paths=sorted(group), guarded_resolutions=[])
            for path in group:
                stem = path.replace('/', '__')
                item = dict(path=path, category='GUARDED_ACTUAL_CONFLICT_RESOLUTION', entries={}, files={},
                            current_h3_entry=entry(repo, merge, path), test_only=False,
                            review='SOURCE_CLEAR: private fixture independent source review only')
                for label, rev in (('base', base), ('first', first), ('incoming', incoming), ('result', merge)):
                    item['entries'][label] = entry(repo, rev, path)
                    if item['entries'][label]:
                        data = rawgit(repo, 'show', rev + ':' + path).stdout
                        name = prefix + label + '__' + stem
                        self.artifacts[name] = data
                        item['files'][label] = dict(name=name, sha256=hashlib.sha256(data).hexdigest())
                for label, rev in (('first-result', first), ('incoming-result', incoming)):
                    self.artifacts[prefix + label + '__' + stem + '.diff'] = rawgit(
                        repo, 'diff', '--no-ext-diff', rev, merge, '--', path).stdout
                row['guarded_resolutions'].append(item)
            self.rows.append(row)
        self.source = git(repo, 'rev-parse', 'HEAD')
        template = json.loads((ROOT / RECORD).read_text())
        self.data = template
        template['source'] = self.source
        template['base'] = self.base
        template['rows'] = self.rows
        template['runtime_coverage']['source'] = self.source
        proposal = copy.deepcopy(template)
        for key in ('proposal_sha256', 'source_qualification', 'qualification_kind', 'activation_authority',
                    'current_or_future_path_authority', 'consumption_contract', 'runtime_coverage', 'artifacts'):
            proposal.pop(key)
        proposal['version'] = 'MASTER-HISTORICAL-COMPOSITION-PROPOSED-v1'
        proposal['qualification'] = False
        for row in proposal['rows']:
            for item in row['guarded_resolutions']:
                item['review'] = 'PENDING'
        raw = (json.dumps(proposal, indent=2) + '\n').encode()
        self.artifacts['historical-composition/composition-PROPOSAL.json'] = raw
        template['proposal_sha256'] = hashlib.sha256(raw).hexdigest()
        template['artifacts'] = [dict(name=name, sha256=hashlib.sha256(data).hexdigest(), bytes=len(data))
                                 for name, data in sorted(self.artifacts.items())]
        assert len(template['artifacts']) == 243
        self.freeze()

    def write(self, name, text):
        p = self.repo / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text)

    def commit(self, message):
        git(self.repo, 'add', '.')
        git(self.repo, 'commit', '--allow-empty', '-qm', message)
        return git(self.repo, 'rev-parse', 'HEAD')

    def freeze(self):
        raw = json.dumps(self.data, indent=2, ensure_ascii=False) + '\n'
        self.write(RECORD, raw)
        self.record = self.commit('separate immutable record')
        code = (ROOT / HELPER).read_text() if (ROOT / HELPER).exists() else '# consumer not implemented\n'
        for name, value in dict(PINNED_RECORD_COMMIT=self.record,
                                PINNED_RECORD_SHA256=hashlib.sha256(raw.encode()).hexdigest(),
                                PINNED_SOURCE=self.source, PINNED_BASE=self.base).items():
            code = re.sub(r'^' + name + r' = ".*"$', name + ' = "' + value + '"', code, flags=re.M)
        self.write(HELPER, code)
        self.target = self.commit('later carrier')

    def run(self, **overrides):
        row = self.rows[0]
        args = dict(repo=str(self.repo), target=self.target, merge=row['merge'], first=row['parents'][0],
                    source=row['parents'][1], **{'source-base': row['best_bases'][0], 'path': self.groups[0][0]})
        args.update(overrides)
        cmd = ['python3', str(self.repo / HELPER)]
        for key, value in args.items():
            cmd += ['--' + key, value]
        return subprocess.run(cmd, capture_output=True, text=True)


class HistoricalCompositionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.folder = Path(self.tmp.name)
        self.before = {p: hashlib.sha256(p.read_bytes()).hexdigest() for p in
                       (ROOT / HELPER, SHELL) if p.exists()}

    def tearDown(self):
        for path, sha in self.before.items():
            self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), sha)

    def fixture(self):
        return Unit(self.folder / str(len(list(self.folder.iterdir()))))

    def test_shell_complete_unit_preserves_sources_content_history_and_runtime_taint(self):
        h = self.fixture()
        out = self.folder / 'shell'
        out.mkdir()
        for name in ('game-checkpoint-tree', 'be255-checkpoint-tree'):
            (out / name).write_text('')
        source = SHELL.read_text()
        extra = '\n'.join(m.group(0) for m in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n', source)
            if m.group(1) == 'f13_historical_composition_qualified')
        values = dict(ROOT=str(h.repo), TMP_DIR=str(out), GITHUB_EVENT_NAME='pull_request',
                      GITHUB_REF='refs/pull/875/merge', native_p3_active='false', game_checkpoint_aggregate_active='false',
                      game_checkpoint_snapshot_base='', game_checkpoint_snapshot_head='', game_checkpoint_snapshot_record='',
                      game_checkpoint_aggregate_head='', game_checkpoint_aggregate_paths='')
        program = 'set -eu\n' + definitions() + '\n' + extra + '\n'
        program += '\n'.join(k + '=' + shlex.quote(v) for k, v in values.items()) + '\n'
        program += f'f13_collect_develop_contributions "$ROOT" {h.base} {h.target} "$TMP_DIR/map" "$TMP_DIR/category" true true\n'
        path = h.groups[0][0]
        program += f'f13_changed_content "$ROOT" "$TMP_DIR/map" {path} > "$TMP_DIR/content"\n'
        result = subprocess.run(['bash', '-c', program], capture_output=True, text=True)
        category = (out / 'category').read_text() if (out / 'category').exists() else 'no category'
        self.assertEqual(result.returncode, 0, result.stderr + category)
        rows = (out / 'map').read_text().splitlines()
        runtime = next(r.split('\t') for r in rows if r.startswith('src/backend/voice/main.go\t'))
        self.assertEqual(runtime[1], 'true')
        actual = next(r.split('\t') for r in rows if r.startswith(path + '\t'))
        first = h.rows[0]
        self.assertEqual(actual[2:7], [first['merge'], first['parents'][0], first['best_bases'][0],
                                        first['parents'][1], first['best_bases'][0]])
        self.assertIn('// first', (out / 'content').read_text())
        self.assertIn('// incoming', (out / 'content').read_text())
        self.assertEqual(len((out / 'f13-historical-composition.jsonl').read_text().splitlines()), 38)
        self.assertIn(first['parents'][1], (out / 'f13-guarded-merge-history.tsv').read_text())
        # A damaged complete authority cannot fall back through ordinary clean
        # merge or test-only Role provenance in the real shell consumer.
        previous = h.target
        h.data['rows'].pop()
        h.freeze()
        rejected = subprocess.run(['bash', '-c', program.replace(previous, h.target)],
                                  capture_output=True, text=True)
        self.assertEqual(rejected.returncode, 1)
        self.assertEqual((out / 'category').read_text().strip(), 'merge_tree_mismatch')

    def test_all_eight_exact_calls_grant_source_only(self):
        h = self.fixture()
        for row in h.rows:
            r = h.run(merge=row['merge'], first=row['parents'][0], source=row['parents'][1],
                      **{'source-base': row['best_bases'][0], 'path': row['guarded_resolutions'][0]['path']})
            self.assertEqual(r.returncode, 0, r.stderr)
            proof = json.loads(r.stdout)
            self.assertEqual((proof['merges'], proof['occurrences'], proof['artifact_bindings']), (8, 38, 243))
            self.assertFalse(proof['runtime_pass'])
            self.assertTrue(proof['retain_runtime_and_content_taint'])

    def test_partial_forged_binding_mode_source_and_authority_records_reject(self):
        for case in ('partial', 'occurrence', 'binding', 'missing-binding', 'conflicts', 'current',
                     'mode', 'oid', 'source', 'runtime', 'activation', 'future'):
            with self.subTest(case=case):
                h = self.fixture()
                if case == 'partial': h.data['rows'].pop()
                elif case == 'occurrence': h.rows[-1]['guarded_resolutions'].pop()
                elif case == 'binding': h.data['artifacts'][-1]['sha256'] = '0' * 64
                elif case == 'missing-binding': h.data['artifacts'].pop()
                elif case == 'conflicts': h.rows[-1]['whole_conflict_paths'].pop()
                elif case == 'current': h.rows[-1]['guarded_resolutions'][-1]['current_h3_entry']['oid'] = '0' * 40
                elif case in ('mode', 'oid'):
                    h.rows[-1]['guarded_resolutions'][-1]['entries']['result'][case] = '100755' if case == 'mode' else '0' * 40
                elif case == 'source': h.data['source'] = h.rows[0]['merge']
                else: h.data[{'runtime': 'runtime_pass', 'activation': 'activation_authority',
                              'future': 'current_or_future_path_authority'}[case]] = True
                h.freeze()
                self.assertEqual(h.run().returncode, 1)

    def test_wrong_historical_tuple_future_path_missing_carrier_and_no_selector(self):
        h = self.fixture()
        for case in ({'merge': h.target}, {'first': h.source}, {'source': h.base}, {'source-base': h.source},
                     {'path': 'src/backend/space/future.go'}, {'target': h.record}):
            with self.subTest(case=case):
                self.assertEqual(h.run(**case).returncode, 1)
        self.assertNotEqual(h.run(record=h.record).returncode, 0)
        p = h.repo / HELPER
        code = p.read_text()
        for name, value in (('PINNED_RECORD_COMMIT', '0' * 40), ('PINNED_RECORD_SHA256', '0' * 64)):
            p.write_text(re.sub(r'^' + name + r' = ".*"$', name + ' = "' + value + '"', code, flags=re.M))
            self.assertEqual(h.run().returncode, 1)
        p.write_text(code)


if __name__ == '__main__':
    unittest.main()
