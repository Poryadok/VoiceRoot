import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import f13_current_source as current
import f13_production_source as production


class SourceClosureTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name)
        self.git('init', '-q', '-b', 'master')
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')
        for path, data in {'src/backend/role/pkg/a.go': 'package pkg\n',
                           'src/backend/role/pkg/config.txt': 'configuration\n',
                           'src/backend/role/go.mod': 'module example\n',
                           'protos/example.proto': 'syntax = "proto3";\n'}.items():
            target = self.repo / path
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(data, encoding='utf-8')
        self.git('add', '.')
        self.git('commit', '-qm', 'source')
        self.source = self.git('rev-parse', 'HEAD')
        self.raw = json.dumps({'Dir': '/work/src/backend/role/pkg', 'GoFiles': ['a.go'],
                               'Module': {'Dir': '/work/src/backend/role'}}).encode()

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], stderr=subprocess.PIPE).decode().strip()

    def test_closure_derives_files_and_complete_directory(self):
        result = current.closure(self.repo, self.source, self.raw, [], [], ['protos/example.proto'])
        self.assertEqual(result['compiled_paths'], ['src/backend/role/pkg/a.go'])
        self.assertEqual(result['module_manifests'], ['src/backend/role/go.mod'])
        self.assertEqual([x['path'] for x in result['entries']], [
            'protos/example.proto', 'src/backend/role/go.mod',
            'src/backend/role/pkg/a.go', 'src/backend/role/pkg/config.txt'])
        self.assertEqual(result['directories'], ['src/backend/role/pkg'])

    def test_missing_compiled_input_or_context_fails_closed(self):
        bad = self.raw.replace(b'a.go', b'missing.go')
        with self.assertRaises(ValueError):
            current.closure(self.repo, self.source, bad, [], [], [])
        with self.assertRaises(ValueError):
            current.closure(self.repo, self.source, self.raw, [], [], ['protos/missing.proto'])

    def test_open_record_and_missing_pinned_record_are_rejected(self):
        draft = dict(version=current.VERSION, status='OPEN', source=current.SOURCE, source_tree='',
                     predecessor={}, dag={}, native={}, role={}, runtime_evidence={}, source_review={},
                     runtime_pass=False, activation_authority=False, current_or_future_path_authority=False)
        with self.assertRaisesRegex(ValueError, 'current_record_not_reviewed'):
            current.validate_header(draft)
        with self.assertRaises(ValueError):
            current.load_record(self.repo)
        self.assertFalse(current.applicable(self.repo))
        with self.assertRaisesRegex(ValueError, 'current_source_lookup'):
            current.applicable(self.repo / 'missing')

    def test_collection_cli_routes_v6_and_rejects_missing_record(self):
        binding = self.repo / 'binding.json'
        binding.write_text(json.dumps({'version': current.NATIVE_VERSION}), encoding='utf-8')
        script = Path(__file__).with_name('f13_release_source.py')
        result = subprocess.run([sys.executable, str(script), 'collection-base', str(self.repo), str(binding)],
                                capture_output=True, text=True, check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('git_rev-list', result.stderr)

    def test_intermediate_change_and_restore_is_rejected(self):
        archive = self.repo / production.ARCHIVE_PATH
        archive.parent.mkdir(parents=True, exist_ok=True)
        archive.write_bytes(b'historical archive')
        historical_path = 'docs/testing/role-source-v3-evidence/source-review.md'
        historical = self.repo / historical_path
        historical.parent.mkdir(parents=True, exist_ok=True)
        historical.write_text('review', encoding='utf-8')
        self.git('add', '.')
        self.git('commit', '-qm', 'base with archive')
        base = self.git('rev-parse', 'HEAD')
        record = self.repo / 'docs/testing/current-record.json'
        record.parent.mkdir(parents=True, exist_ok=True)
        record.write_text('record', encoding='utf-8')
        self.git('add', '.')
        self.git('commit', '-qm', 'record')
        record_commit = self.git('rev-parse', 'HEAD')
        self.git('commit', '--allow-empty', '-qm', 'carrier')
        carrier = self.git('rev-parse', 'HEAD')
        tree = current.release.documentation.tree_entries(self.repo, base)
        dirs = current.release.documentation.immediate_directory_entries(tree)
        path = 'src/backend/role/pkg/a.go'
        unit = {'entries': [{'path': path}], 'directory_inventories': [
            {'directory': 'src/backend/role/pkg', 'entries': dirs['src/backend/role/pkg']}]}
        foundation = {'source': carrier, 'target': carrier}
        current._check_continuation(self.repo, foundation, carrier, {'role': unit}, base, record_commit,
                                    {historical_path})
        (self.repo / path).write_text('changed\n', encoding='utf-8')
        self.git('add', path)
        self.git('commit', '-qm', 'change')
        (self.repo / path).write_text('package pkg\n', encoding='utf-8')
        self.git('add', path)
        self.git('commit', '-qm', 'restore')
        with self.assertRaisesRegex(ValueError, 'current_continuation_entry'):
            current._check_continuation(self.repo, foundation, self.git('rev-parse', 'HEAD'),
                                        {'role': unit}, base, record_commit, {historical_path})
        self.git('checkout', '-qb', 'precarrier-test', record_commit)
        (self.repo / path).write_text('changed before carrier\n', encoding='utf-8')
        self.git('add', path)
        self.git('commit', '-qm', 'precarrier change')
        (self.repo / path).write_text('package pkg\n', encoding='utf-8')
        self.git('add', path)
        self.git('commit', '-qm', 'precarrier restore')
        restored = self.git('rev-parse', 'HEAD')
        with self.assertRaisesRegex(ValueError, 'current_precarrier_entry'):
            current._check_continuation(self.repo, {'source': restored, 'target': restored}, restored,
                                        {'role': unit}, base, record_commit, {historical_path})
        self.git('checkout', '-qb', 'artifact-test', carrier)
        historical.write_text('changed', encoding='utf-8')
        self.git('add', historical_path)
        self.git('commit', '-qm', 'change historical evidence')
        historical.write_text('review', encoding='utf-8')
        self.git('add', historical_path)
        self.git('commit', '-qm', 'restore historical evidence')
        with self.assertRaisesRegex(ValueError, 'current_continuation_entry'):
            current._check_continuation(self.repo, foundation, self.git('rev-parse', 'HEAD'),
                                        {'role': unit}, base, record_commit, {historical_path})


if __name__ == '__main__':
    unittest.main()
