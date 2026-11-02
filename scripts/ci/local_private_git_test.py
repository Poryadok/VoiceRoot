"""Private Git fixtures must never alter the source repository or index."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import local_private_git
import local_source_snapshot


class PrivateGitTests(unittest.TestCase):
    def test_private_index_tracks_captured_tree_and_detects_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            root = base / 'checkout'
            root.mkdir()
            def git(*args, env=None):
                return subprocess.check_output(['git', '-C', str(root), *args], env=env, text=True).strip()
            git('init', '-q')
            tracked = root / 'src/backend/chat/pb/test.go'
            tracked.parent.mkdir(parents=True)
            tracked.write_text('old\n', encoding='utf-8')
            git('add', '.')
            git('-c', 'user.name=CI', '-c', 'user.email=ci@example.invalid', 'commit', '-qm', 'fixture')
            head = git('rev-parse', 'HEAD')
            tracked.write_text('captured\n', encoding='utf-8')
            source = local_source_snapshot.capture(root, head)
            snapshot = local_source_snapshot.materialize(root, source, base / 'snapshot')
            original = git('status', '--porcelain')
            calls = []
            def runner(command, **kwargs):
                calls.append(command)
                return subprocess.run(command, capture_output=True, check=False, **kwargs)
            env = local_private_git.prepare(root, source, snapshot, base / 'fixture.git', runner=runner)
            self.assertEqual(len(calls), 9)
            self.assertTrue(all(command[0] == 'git' for command in calls))
            self.assertEqual(subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], cwd=snapshot, env=env, text=True).strip(), source['tree'])
            self.assertEqual(subprocess.run(['git', 'diff', '--quiet', 'HEAD'], cwd=snapshot, env=env).returncode, 0)
            new = snapshot / 'src/backend/chat/pb/new.go'
            new.write_text('new\n', encoding='utf-8')
            untracked = subprocess.check_output(['git', 'ls-files', '--others', '--exclude-standard', '--',
                                                 'src/backend/chat/pb'], cwd=snapshot, env=env, text=True)
            self.assertIn('src/backend/chat/pb/new.go', untracked)
            (snapshot / 'src/backend/chat/pb/test.go').write_text('changed\n', encoding='utf-8')
            self.assertEqual(subprocess.run(['git', 'diff', '--quiet', 'HEAD'], cwd=snapshot, env=env).returncode, 1)
            self.assertEqual(git('status', '--porcelain'), original)
            self.assertEqual(tracked.read_text(encoding='utf-8'), 'captured\n')

    def test_rejects_private_git_inside_checkout(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'checkout'
            root.mkdir()
            with self.assertRaises(ValueError):
                local_private_git.prepare(root, {'tree': 'a' * 40}, Path(directory) / 'snapshot', root / 'private.git')


if __name__ == '__main__':
    unittest.main()
