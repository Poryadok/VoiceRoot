"""Disposable Git tests for the local current-diff diagnostic."""
import json
import subprocess
import tempfile
import unittest
from pathlib import Path

import local_current_diff
import local_source_snapshot


class CurrentDiffDiagnosticTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'repo'
        self.root.mkdir()
        self.output = Path(self.tmp.name) / 'output'
        self.git('init', '-q')
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')
        script = self.root / 'scripts/ci/current-diff-contract.py'
        script.parent.mkdir(parents=True)
        script.write_bytes((Path(__file__).parent / 'current-diff-contract.py').read_bytes())
        self.migration = self.root / 'src/backend/migrations/chat/001_init.sql'
        self.migration.parent.mkdir(parents=True)
        self.migration.write_text('SELECT 1;\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'base')
        self.head = self.git('rev-parse', 'HEAD').decode().strip()

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE)

    def test_source_bound_snapshot_runs_canonical_validator(self):
        (self.root / 'new.txt').write_text('new\n')
        source = local_source_snapshot.capture(self.root, self.head)
        report = local_current_diff.run(self.root, source, self.head, self.output)
        self.assertEqual(report['status'], 'diagnostic')
        self.assertEqual(report['source'], source)
        self.assertIn('new.txt', report['contract']['changed_paths'])
        self.assertEqual(self.git('rev-parse', 'HEAD').decode().strip(), self.head)
        self.assertEqual(json.loads((self.output / 'diagnostic.json').read_text()), report)

    def test_modified_existing_migration_fails(self):
        self.migration.write_text('SELECT 2;\n')
        source = local_source_snapshot.capture(self.root, self.head)
        with self.assertRaisesRegex(ValueError, 'current-diff-contract'):
            local_current_diff.run(self.root, source, self.head, self.output)

    def test_changed_source_after_capture_is_rejected(self):
        source = local_source_snapshot.capture(self.root, self.head)
        (self.root / 'later.txt').write_text('later\n')
        with self.assertRaisesRegex(ValueError, 'source changed'):
            local_current_diff.run(self.root, source, self.head, self.output)


if __name__ == '__main__':
    unittest.main()
