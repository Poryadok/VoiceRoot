"""Behavioral tests for local source snapshots used by preflight."""
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import affected_check_plan as planner
import local_source_snapshot as snapshot


class LocalSourceSnapshotTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.git('init', '-q')
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')
        (self.root / '.github/ci').mkdir(parents=True)
        (self.root / '.github/ci/path-filters.yml').write_bytes(
            b'code:\n  - "**"\nsvc_chat:\n  - src/backend/chat/**\n')
        (self.root / 'old.txt').write_bytes(b'old\n')
        (self.root / 'deleted.txt').write_bytes(b'delete\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'base')
        self.head = self.git('rev-parse', 'HEAD').strip().decode()

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE)

    def test_tracks_actual_worktree_and_leaves_real_index_unchanged(self):
        (self.root / 'old.txt').rename(self.root / 'new.txt')
        (self.root / 'deleted.txt').unlink()
        self.git('add', '-A')
        (self.root / 'new.txt').write_text('updated after staging\n')
        (self.root / 'untracked.txt').write_text('new file\n')
        before = self.git('status', '--porcelain=v1', '-z', '--untracked-files=all')
        source = snapshot.capture(self.root, self.head)
        self.assertEqual(source, snapshot.capture(self.root, self.head))
        self.assertEqual(source['head'], self.head)
        self.assertEqual(set(source['changed_paths']),
                         {'old.txt', 'new.txt', 'deleted.txt', 'untracked.txt'})
        self.assertEqual(before, self.git('status', '--porcelain=v1', '-z', '--untracked-files=all'))
        (self.root / 'new.txt').write_text('different contents\n')
        self.assertNotEqual(source['tree'], snapshot.capture(self.root, self.head)['tree'])

    def test_clean_tree_matches_head_and_wrong_head_is_rejected(self):
        source = snapshot.capture(self.root, self.head)
        self.assertEqual(source['tree'], self.git('rev-parse', 'HEAD^{tree}').strip().decode())
        self.assertEqual(source['changed_paths'], [])
        with self.assertRaises(ValueError):
            snapshot.capture(self.root, '0' * 40)

    def test_materialized_source_preserves_dirty_raw_bytes_and_real_index(self):
        (self.root / 'old.txt').write_bytes(b'changed\r\n')
        (self.root / 'untracked.txt').write_bytes(b'new\n')
        before = self.git('status', '--porcelain=v1', '-z', '--untracked-files=all')
        source = snapshot.capture(self.root, self.head)
        with tempfile.TemporaryDirectory() as external:
            destination = Path(external) / 'source'
            snapshot.materialize(self.root, source, destination)
            self.assertEqual((destination / 'old.txt').read_bytes(), b'changed\r\n')
            self.assertEqual((destination / 'untracked.txt').read_bytes(), b'new\n')
            self.assertFalse((destination / '.git').exists())
            with self.assertRaises(ValueError):
                snapshot.materialize(self.root, source, destination)
        self.assertEqual(before, self.git('status', '--porcelain=v1', '-z', '--untracked-files=all'))

    def test_materialization_rejects_changed_source(self):
        source = snapshot.capture(self.root, self.head)
        (self.root / 'old.txt').write_bytes(b'changed after snapshot\n')
        with tempfile.TemporaryDirectory() as external:
            with self.assertRaisesRegex(ValueError, 'source changed'):
                snapshot.materialize(self.root, source, Path(external) / 'source')

    def test_materialization_rejects_corrupt_copy_even_if_source_is_unchanged(self):
        source = snapshot.capture(self.root, self.head)
        copy = snapshot.shutil.copyfile
        def corrupt(original, target):
            copy(original, target)
            if original.name == 'old.txt':
                target.write_bytes(b'corrupt\n')
        with tempfile.TemporaryDirectory() as external:
            with mock.patch.object(snapshot.shutil, 'copyfile', side_effect=corrupt):
                with self.assertRaisesRegex(ValueError, 'source changed'):
                    snapshot.materialize(self.root, source, Path(external) / 'source')

    def test_materialization_refuses_destination_in_source_tree(self):
        source = snapshot.capture(self.root, self.head)
        with self.assertRaisesRegex(ValueError, 'outside'):
            snapshot.materialize(self.root, source, self.root / 'snapshot')

    def test_materialized_source_can_be_rechecked_after_a_command(self):
        source = snapshot.capture(self.root, self.head)
        with tempfile.TemporaryDirectory() as external:
            destination = Path(external) / 'source'
            snapshot.materialize(self.root, source, destination)
            snapshot.verify_materialized(self.root, source, destination)
            (destination / 'old.txt').write_bytes(b'changed by command\n')
            with self.assertRaisesRegex(ValueError, 'snapshot changed'):
                snapshot.verify_materialized(self.root, source, destination)

    def test_tracked_gitlinks_are_rejected(self):
        self.git('update-index', '--add', '--cacheinfo', '160000,' + self.head + ',submodule')
        with self.assertRaisesRegex(ValueError, 'gitlink'):
            snapshot.capture(self.root, self.head)

    def test_staged_symbolic_link_is_rejected(self):
        oid = self.git('hash-object', 'old.txt').strip().decode()
        self.git('update-index', '--add', '--cacheinfo', '120000,' + oid + ',link')
        with self.assertRaisesRegex(ValueError, 'symbolic link'):
            snapshot.capture(self.root, self.head)

    def test_raw_file_changes_are_visible_even_when_git_normalizes_them(self):
        (self.root / '.gitattributes').write_text('*.txt text\n')
        (self.root / 'old.txt').write_bytes(b'old\n')
        first = snapshot.capture(self.root, self.head)
        (self.root / 'old.txt').write_bytes(b'old\r\n')
        second = snapshot.capture(self.root, self.head)
        self.assertEqual(first['tree'], second['tree'])
        self.assertNotEqual(first['worktree_sha256'], second['worktree_sha256'])
        self.assertIn('old.txt', second['raw_mismatch_paths'])
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        with mock.patch.object(planner, 'resolve_go_matrix', return_value=matrix):
            plan = planner.build_local_plan(self.root, self.head, self.head)
        self.assertIn('old.txt', plan['changed_paths'])

    def test_local_plan_matches_hosted_selection_for_clean_source_and_adds_dirty_changes(self):
        base = self.head
        (self.root / 'src/backend/chat').mkdir(parents=True)
        (self.root / 'src/backend/chat/main.go').write_bytes(b'package main\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'head')
        head = self.git('rev-parse', 'HEAD').strip().decode()
        matrix = {'go_services': ['chat'], 'docker_go_services': ['chat'],
                  'integration_go_services': ['chat'], 'run_go': True, 'run_pkg': True}
        with mock.patch.object(planner, 'resolve_go_matrix', return_value=matrix):
            hosted = planner.build_plan(self.root, base, head, head)
            local = planner.build_local_plan(self.root, base, head)
            for key in ('components', 'checks', 'filters', 'changed_paths', 'filter_reasons'):
                self.assertEqual(local[key], hosted[key], key)
            (self.root / 'new-area').mkdir()
            (self.root / 'new-area/tool.py').write_text('print(1)\n')
            dirty = planner.build_local_plan(self.root, base, head)
            self.assertIn('new-area/tool.py', dirty['changed_paths'])
            self.assertTrue(dirty['filters']['ci_global'])
            self.assertNotEqual(local['source']['tree'], dirty['source']['tree'])

    def test_changed_source_during_planning_is_rejected(self):
        matrix = {'go_services': [], 'docker_go_services': [],
                  'integration_go_services': [], 'run_go': False, 'run_pkg': False}
        def mutate(*_):
            (self.root / 'old.txt').write_text('changed during planning\n')
            return matrix
        with mock.patch.object(planner, 'resolve_go_matrix', side_effect=mutate):
            with self.assertRaisesRegex(ValueError, 'source changed'):
                planner.build_local_plan(self.root, self.head, self.head)


if __name__ == '__main__':
    unittest.main()
