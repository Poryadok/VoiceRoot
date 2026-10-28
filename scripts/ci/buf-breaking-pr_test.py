"""Real Git/Buf regressions for the immutable PR breaking-check baseline."""
from contextlib import redirect_stderr, redirect_stdout
import importlib.util
import io
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / 'scripts/ci/buf-breaking-pr.py'
SCHEMA = 'protos/example/v1/example.proto'


class ImmutableBaselineTests(unittest.TestCase):
    def setUp(self):
        scratch = ROOT / 'tmp/anton197-buf-baseline'
        scratch.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / 'candidate'
        self.repo.mkdir()

    def git(self, *args, cwd=None):
        return subprocess.check_output(
            ['git', '-C', str(cwd or self.repo), *args],
            stderr=subprocess.PIPE, text=True, encoding='utf-8').strip()

    def identity(self, repo):
        self.git('config', 'user.name', 'Buf fixture', cwd=repo)
        self.git('config', 'user.email', 'buf-fixture@example.invalid', cwd=repo)
        self.git('config', 'core.autocrlf', 'false', cwd=repo)

    def commit(self, message, repo=None):
        self.git('add', '.', cwd=repo)
        self.git('commit', '-m', message, cwd=repo)
        return self.git('rev-parse', 'HEAD', cwd=repo)

    def fixture(self, remove=False):
        self.git('init', '--initial-branch=master')
        self.identity(self.repo)
        schema = self.repo / SCHEMA
        schema.parent.mkdir(parents=True)
        schema.write_text('syntax = "proto3";\npackage example.v1;\n'
                          'message Original { string value = 1; }\n', encoding='utf-8')
        (self.repo / 'buf.yaml').write_text(
            'version: v2\nmodules:\n  - path: protos\nbreaking:\n  use: [FILE]\n',
            encoding='utf-8')
        base = self.commit('base schema')
        remote = self.root / 'remote.git'
        subprocess.run(['git', 'init', '--bare', '--initial-branch=master', str(remote)],
                       check=True, capture_output=True)
        self.git('remote', 'add', 'origin', str(remote))
        self.git('push', 'origin', 'master')
        self.git('checkout', '-b', 'feature')
        if remove:
            schema.write_text('syntax = "proto3";\npackage example.v1;\n'
                              'message Replacement { string value = 1; }\n', encoding='utf-8')
        else:
            (self.repo / 'README.md').write_text('Documentation change.\n', encoding='utf-8')
        head = self.commit('PR change')
        self.git('checkout', '-b', 'integration', base)
        self.git('merge', '--no-ff', 'feature', '-m', 'synthetic integration')
        merge = self.git('rev-parse', 'HEAD')
        self.git('checkout', '--detach', merge)
        return base, head, merge, remote

    def gate(self, base, head, merge):
        return subprocess.run(
            [sys.executable, str(HELPER), '--repo', str(self.repo),
             '--base', base, '--head', head, '--merge', merge],
            capture_output=True, text=True, encoding='utf-8')

    def assert_refused_before_buf(self, base, head, merge, diagnostic):
        # Patch only the Buf execution boundary; Git identity checks stay real.
        # The real-Buf advance/removal tests above continue through the CLI.
        spec = importlib.util.spec_from_file_location('buf_breaking_pr', HELPER)
        gate = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(gate)
        out, err = io.StringIO(), io.StringIO()
        with mock.patch.object(gate, 'run_buf',
                               side_effect=AssertionError('Buf ran before identity refusal')) as sentinel:
            with redirect_stdout(out), redirect_stderr(err):
                result = gate.main(['--repo', str(self.repo), '--base', base,
                                    '--head', head, '--merge', merge])
            sentinel.assert_not_called()
        self.assertNotEqual(result, 0)
        self.assertIn(diagnostic, err.getvalue())

    def test_advancing_remote_master_does_not_change_event_baseline(self):
        base, head, merge, remote = self.fixture()
        writer = self.root / 'writer'
        subprocess.run(['git', 'clone', str(remote), str(writer)],
                       check=True, capture_output=True)
        self.identity(writer)
        with (writer / SCHEMA).open('a', encoding='utf-8') as schema:
            schema.write('message Future { string value = 1; }\n')
        later = self.commit('later master declaration', writer)
        self.git('push', 'origin', 'master', cwd=writer)
        self.git('fetch', '--no-tags', 'origin', 'master:master')
        self.assertEqual(self.git('rev-parse', 'master'), later)
        self.assertNotEqual(later, base)
        # Reproduce the real failure with the old workflow's actual Buf command.
        legacy = subprocess.run(
            ['buf', 'breaking', 'protos', '--against', '.git#branch=master,subdir=protos'],
            cwd=self.repo, capture_output=True, text=True, encoding='utf-8')
        self.assertNotEqual(legacy.returncode, 0, legacy.stdout + legacy.stderr)
        self.assertIn('Future', legacy.stdout + legacy.stderr)
        result = self.gate(base, head, merge)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        receipt = json.loads(result.stdout.splitlines()[0])
        self.assertEqual((receipt['checkout'], receipt['base'], receipt['head']),
                         (merge, base, head))
        self.assertEqual(receipt['event_base'], base)
        self.assertEqual(receipt['parents'], [base, head])
        self.assertEqual(receipt['base_protos_tree'], self.git('rev-parse', base + ':protos'))

    def test_stale_event_base_uses_exact_current_merge_first_parent(self):
        event_base, head, _, _ = self.fixture()
        self.git('checkout', '-b', 'current-base', event_base)
        (self.repo / SCHEMA).write_text(
            'syntax = "proto3";\npackage example.v1;\n'
            'message Current { string value = 1; }\n', encoding='utf-8')
        current_base = self.commit('advance integration base')
        self.git('merge', '--no-ff', head, '-m', 'synthetic refreshed integration')
        merge = self.git('rev-parse', 'HEAD')
        self.git('checkout', '--detach', merge)

        result = self.gate(event_base, head, merge)

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        receipt = json.loads(result.stdout.splitlines()[0])
        self.assertEqual(receipt['event_base'], event_base)
        self.assertEqual(receipt['base'], current_base)
        self.assertEqual(receipt['parents'], [current_base, head])
        self.assertEqual(receipt['base_tree'], self.git('rev-parse', current_base + '^{tree}'))
        self.assertEqual(receipt['base_protos_tree'],
                         self.git('rev-parse', current_base + ':protos'))
        self.assertNotEqual(receipt['base_protos_tree'],
                            self.git('rev-parse', event_base + ':protos'))

        self.assert_refused_before_buf(head, head, merge, 'base_ancestry')

    def test_real_declaration_removal_still_fails_buf(self):
        base, head, merge, _ = self.fixture(remove=True)
        result = self.gate(base, head, merge)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Original', result.stdout + result.stderr)
        self.assertIn('deleted', result.stdout + result.stderr)

    def test_stale_event_base_uses_exact_current_merge_first_parent(self):
        event_base, head, _, _ = self.fixture()
        self.git('checkout', '-b', 'current-base', event_base)
        (self.repo / SCHEMA).write_text(
            'syntax = "proto3";\npackage example.v1;\n'
            'message Current { string value = 1; }\n', encoding='utf-8')
        current_base = self.commit('advance integration base')
        self.git('merge', '--no-ff', head, '-m', 'synthetic refreshed integration')
        merge = self.git('rev-parse', 'HEAD')
        self.git('checkout', '--detach', merge)

        result = self.gate(event_base, head, merge)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        receipt = json.loads(result.stdout.splitlines()[0])
        self.assertEqual(receipt['event_base'], event_base)
        self.assertEqual(receipt['base'], current_base)
        self.assertEqual(receipt['parents'], [current_base, head])
        self.assertEqual(receipt['base_tree'], self.git('rev-parse', current_base + '^{tree}'))
        self.assertEqual(receipt['base_protos_tree'], self.git('rev-parse', current_base + ':protos'))
        self.assertNotEqual(receipt['base_protos_tree'],
                            self.git('rev-parse', event_base + ':protos'))
        self.assert_refused_before_buf(head, head, merge, 'base_ancestry')

    def test_mismatched_event_base_fails_closed(self):
        base, head, merge, _ = self.fixture()
        self.assert_refused_before_buf(head, head, merge, 'base_ancestry')

    def test_checkout_must_be_the_event_merge(self):
        base, head, merge, _ = self.fixture()
        self.git('checkout', '--detach', head)
        self.assert_refused_before_buf(base, head, merge, 'checkout_identity')

    def test_event_sha_cannot_be_a_revision_expression(self):
        base, head, merge, _ = self.fixture()
        self.assert_refused_before_buf(base + '^', head, merge, 'base_identity')

    def test_missing_immutable_base_object_fails_closed(self):
        _, head, merge, _ = self.fixture()
        missing = 'f' * 40
        absent = subprocess.run(['git', '-C', str(self.repo), 'cat-file', '-e', missing],
                                capture_output=True)
        self.assertNotEqual(absent.returncode, 0)
        self.assert_refused_before_buf(missing, head, merge, 'base_object')

    def test_workflow_passes_immutable_event_identities_and_runs_regressions(self):
        workflow = (ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8')
        protobuf = re.search(r'(?ms)^  protobuf:\n.*?(?=^  [\w-]+:)', workflow).group()
        self.assertIn('python3 scripts/ci/buf-breaking-pr_test.py', protobuf)
        self.assertIn('python3 scripts/ci/buf-breaking-pr.py', protobuf)
        self.assertIn('github.event.pull_request.base.sha', protobuf)
        self.assertIn('github.event.pull_request.head.sha', protobuf)
        self.assertIn('github.sha', protobuf)
        self.assertNotIn('github.base_ref', protobuf)


if __name__ == '__main__':
    unittest.main()
