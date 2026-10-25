"""One fixed historical duplicate requires its correction and actual two rows."""
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
HELPER = 'scripts/ci/f13_role_conditional.py'
RECORD = 'docs/testing/role-checkpermission-historical-source-v1.json'
SHELL = ROOT / 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh'
ROLE = 'src/backend/role/internal/principalruntime/runtime.go'


def entry(repo, rev, path):
    mode, kind, oid = git(repo, 'ls-tree', rev, '--', path).split('\t')[0].split()
    return dict(mode=mode, type=kind, oid=oid)


class History:
    def __init__(self, repo):
        self.repo = repo
        repo.mkdir()
        git(repo, 'init', '-q', '-b', 'lane')
        git(repo, 'config', 'user.name', 'Conditional historical fixture')
        git(repo, 'config', 'user.email', 'fixture@example.invalid')
        self.data = json.loads((ROOT / RECORD).read_text())
        for item in self.data['required_current_source']:
            self.write(item['path'], 'fixture base\n')
        self.base = self.commit('base')
        self.write('README', 'earlier first parent\n')
        self.previous_first = self.commit('earlier parent')
        git(repo, 'checkout', '-qb', 'old-source', self.base)
        self.write(ROLE, 'reviewed historical bytes\n')
        self.write('src/backend/voice/main.go', 'package main\n// original runtime taint\n')
        self.previous_source = self.commit('original producer with runtime taint')
        git(repo, 'checkout', '-q', self.previous_first)
        git(repo, 'merge', '--no-ff', '-qm', 'previous actual integration', self.previous_source)
        self.previous_owner = git(repo, 'rev-parse', 'HEAD')
        self.write('README', 'later first parent\n')
        self.first = self.commit('later parent')
        git(repo, 'checkout', '-qb', 'incoming', self.base)
        self.write(ROLE, 'reviewed historical bytes\n')
        self.incoming = self.commit('independent identical producer')
        git(repo, 'checkout', '-q', self.first)
        git(repo, 'merge', '--no-ff', '-qm', 'actual historical duplicate', self.incoming)
        self.merge = git(repo, 'rev-parse', 'HEAD')
        self.write('src/backend/role/internal/principalruntime/listener.go', 'mandatory current correction\n')
        self.source = self.commit('mandatory corrected current listener')
        r = self.data
        r['source'], r['base'] = self.source, self.base
        r['current_correction']['commit'] = self.source
        r['historical_tuple'] = dict(merge=self.merge, parents=[self.first, self.incoming], best_bases=[self.base],
            previous_owner=self.previous_owner, previous_owner_parents=[self.previous_first, self.previous_source],
            previous_source=self.previous_source, previous_source_base=self.base,
            entries={label: entry(repo, rev, ROLE) for label, rev in (
                ('base', self.base), ('first', self.first), ('incoming', self.incoming), ('result', self.merge),
                ('previous_owner', self.previous_owner), ('previous_source', self.previous_source))})
        self.prior = [ROLE, 'true', self.previous_owner, self.previous_first, self.base,
                      self.previous_source, self.previous_first, self.base]
        r['actual_prior_row'] = self.prior.copy()
        r['incoming_metadata_at_failure'] = dict(owner=self.merge, parent=self.first, source_base=self.base,
            source_head=self.incoming, content_base=self.previous_first, best_bases=[self.base])
        for item in r['required_current_source']:
            item['entry'] = entry(repo, self.source, item['path'])
            raw = subprocess.check_output(['git', '-C', str(repo), 'show', self.source + ':' + item['path']])
            item['sha256'] = hashlib.sha256(raw).hexdigest()
        self.freeze()

    def write(self, path, text):
        p = self.repo / path
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text)

    def commit(self, message):
        git(self.repo, 'add', '.')
        git(self.repo, 'commit', '--allow-empty', '-qm', message)
        return git(self.repo, 'rev-parse', 'HEAD')

    def freeze(self):
        raw = json.dumps(self.data, indent=2) + '\n'
        self.write(RECORD, raw)
        self.record = self.commit('immutable conditional source record')
        code = (ROOT / HELPER).read_text() if (ROOT / HELPER).exists() else '# not implemented\n'
        for name, value in dict(PINNED_RECORD_COMMIT=self.record, PINNED_RECORD_SHA256=hashlib.sha256(raw.encode()).hexdigest(),
                                PINNED_SOURCE=self.source, PINNED_BASE=self.base).items():
            code = re.sub(r'^' + name + r' = ".*"$', name + ' = "' + value + '"', code, flags=re.M)
        self.write(HELPER, code)
        self.target = self.commit('later conditional carrier')

    def row(self, trigger='false'):
        return [ROLE, trigger, self.merge, self.first, self.base, self.incoming, self.previous_first, self.base]

    def run(self, prior=None, incoming=None, **overrides):
        args = dict(repo=str(self.repo), target=self.target,
                    **{'prior-row': '\t'.join(prior or self.prior), 'incoming-row': '\t'.join(incoming or self.row())})
        args.update(overrides)
        cmd = ['python3', str(self.repo / HELPER)]
        for key, value in args.items(): cmd += ['--' + key, value]
        return subprocess.run(cmd, capture_output=True, text=True)


class ConditionalRoleTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.folder = Path(self.tmp.name)
        self.before = {p: hashlib.sha256(p.read_bytes()).hexdigest() for p in (SHELL, ROOT / HELPER) if p.exists()}

    def tearDown(self):
        for path, before in self.before.items(): self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), before)

    def fixture(self):
        return History(self.folder / str(len(list(self.folder.iterdir()))))

    def test_real_shell_keeps_both_actual_rows_and_runtime_taint_before_replacement(self):
        h = self.fixture()
        out = self.folder / 'shell'
        out.mkdir()
        for name in ('game-checkpoint-tree', 'be255-checkpoint-tree'): (out / name).write_text('')
        extra = '\n'.join(m.group(0) for m in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n', SHELL.read_text())
            if m.group(1) == 'f13_role_conditional_qualified')
        values = dict(ROOT=str(h.repo), TMP_DIR=str(out), GITHUB_EVENT_NAME='pull_request',
                      GITHUB_REF='refs/pull/875/merge', native_p3_active='false', game_checkpoint_aggregate_active='false',
                      game_checkpoint_snapshot_base='', game_checkpoint_snapshot_head='', game_checkpoint_snapshot_record='',
                      game_checkpoint_aggregate_head='', game_checkpoint_aggregate_paths='')
        program = 'set -eu\n' + definitions() + '\n' + extra + '\n'
        program += '\n'.join(k + '=' + shlex.quote(v) for k, v in values.items()) + '\n'
        program += f'f13_collect_develop_contributions "$ROOT" {h.base} {h.target} "$TMP_DIR/map" "$TMP_DIR/category" true true\n'
        p = subprocess.run(['bash', '-c', program], capture_output=True, text=True)
        category = (out / 'category').read_text() if (out / 'category').exists() else ''
        self.assertEqual(p.returncode, 0, p.stderr + category)
        retained = (out / 'f13-role-conditional-history.tsv').read_text().splitlines()
        self.assertEqual(retained, ['prior\t' + '\t'.join(h.prior), 'incoming\t' + '\t'.join(h.row())])
        current = next(v.split('\t') for v in (out / 'map').read_text().splitlines() if v.startswith(ROLE + '\t'))
        self.assertEqual(current, h.row(trigger='true'))
        proof = json.loads((out / 'f13-role-conditional.jsonl').read_text())
        self.assertEqual(proof['prior_row'], h.prior)
        self.assertEqual(proof['incoming_row'], h.row())
        self.assertFalse(proof['runtime_pass'])
        # Invalid fixed authority cannot borrow generic identical-blob lineage.
        previous = h.target
        h.data['status'] = 'PENDING'
        h.freeze()
        bad = subprocess.run(['bash', '-c', program.replace(previous, h.target)], capture_output=True, text=True)
        self.assertEqual(bad.returncode, 1)
        self.assertEqual((out / 'category').read_text().strip(), 'duplicate_guarded_owner')

    def test_exact_current_correction_and_two_runtime_flags_are_source_only(self):
        h = self.fixture()
        for flag in ('false', 'true'):
            p = h.run(incoming=h.row(trigger=flag))
            self.assertEqual(p.returncode, 0, p.stderr)
            proof = json.loads(p.stdout)
            self.assertEqual(proof['prior_row'][1], 'true')
            self.assertEqual(proof['incoming_row'][1], flag)
            self.assertTrue(proof['retain_runtime_and_content_taint'])
            self.assertFalse(proof['runtime_pass'])

    def test_wrong_actual_prior_or_incoming_metadata_and_future_duplicate_reject(self):
        h = self.fixture()
        for which in ('prior', 'incoming'):
            for column in range(8):
                row = (h.prior if which == 'prior' else h.row()).copy()
                row[column] = ('false' if which == 'prior' else 'invalid') if column == 1 else 'wrong'
                with self.subTest(which=which, column=column):
                    self.assertEqual(h.run(**{which: row}).returncode, 1)
        future = h.row(); future[2] = h.target
        self.assertEqual(h.run(incoming=future).returncode, 1)
        self.assertEqual(h.run(target=h.record).returncode, 1)
        self.assertEqual(h.run(target=h.merge).returncode, 1)
        self.assertNotEqual(h.run(record=h.record).returncode, 0)

    def test_missing_current_binding_mode_source_correction_and_runtime_authority_reject(self):
        for case in ('missing', 'mode', 'oid', 'sha', 'source', 'correction', 'runtime', 'generic', 'future', 'activation'):
            with self.subTest(case=case):
                h = self.fixture()
                if case == 'missing': h.data['required_current_source'].pop()
                elif case in ('mode', 'oid'): h.data['required_current_source'][0]['entry'][case] = '100755' if case == 'mode' else '0' * 40
                elif case == 'sha': h.data['required_current_source'][0]['sha256'] = '0' * 64
                elif case == 'source': h.data['source'] = h.merge
                elif case == 'correction': h.data['current_correction']['commit'] = h.merge
                else: h.data[dict(runtime='runtime_pass', generic='generic_identical_blob_authority',
                                  future='current_or_future_path_authority', activation='activation_authority')[case]] = True
                h.freeze()
                self.assertEqual(h.run().returncode, 1)

    def test_regressed_listener_wrong_pin_and_record_drift_reject(self):
        h = self.fixture()
        h.write('src/backend/role/internal/principalruntime/listener.go', 'regressed current correction\n')
        h.target = h.commit('unqualified current listener regression')
        self.assertEqual(h.run().returncode, 1)
        h = self.fixture()
        p = h.repo / HELPER
        code = p.read_text()
        for name, value in (('PINNED_RECORD_COMMIT', '0' * 40), ('PINNED_RECORD_SHA256', '0' * 64)):
            p.write_text(re.sub(r'^' + name + r' = ".*"$', name + ' = "' + value + '"', code, flags=re.M))
            self.assertEqual(h.run().returncode, 1)
        p.write_text(code)
        h.write(RECORD, '{}\n'); h.target = h.commit('current record drift')
        self.assertEqual(h.run().returncode, 1)


if __name__ == '__main__': unittest.main()
