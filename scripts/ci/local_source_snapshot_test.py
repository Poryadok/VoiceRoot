"""Behavioral tests for local source snapshots used by preflight."""
import subprocess
import tarfile
import hashlib
import os
import sys
import tempfile
import time
from types import SimpleNamespace
import unittest
from pathlib import Path
from unittest import mock

import affected_check_plan as planner
import local_source_snapshot as snapshot
import local_process_supervision as supervision


class LocalSourceSnapshotTests(unittest.TestCase):
    @unittest.skipUnless(os.name == 'nt', 'Windows owned process timeout')
    def test_git_timeout_uses_real_windows_broker(self):
        real_run = supervision.run_diagnostic
        def slow_git(command, **kwargs):
            return real_run([sys.executable, '-c', 'import time; time.sleep(30)'], **kwargs)
        started = time.monotonic()
        with mock.patch.object(snapshot, 'GIT_TIMEOUT_SECONDS', 0.5), \
             mock.patch.object(supervision, 'run_diagnostic', side_effect=slow_git):
            with self.assertRaises(subprocess.TimeoutExpired):
                snapshot._git(self.root, 'status')
        self.assertLess(time.monotonic() - started, 20)

    def test_git_uses_owned_windows_timeout_and_propagates_uncertainty(self):
        with mock.patch.object(snapshot, '_windows', return_value=True), \
             mock.patch.object(supervision, 'run_diagnostic', side_effect=supervision.SupervisionError('cleanup uncertain')) as run:
            with self.assertRaisesRegex(supervision.SupervisionError, 'cleanup uncertain'):
                snapshot._git(self.root, 'status')
        self.assertEqual(run.call_args.kwargs['timeout'], snapshot.GIT_TIMEOUT_SECONDS)
        self.assertTrue(run.call_args.kwargs['capture_output'])

    def test_git_failure_preserves_captured_error_and_posix_timeout(self):
        result = SimpleNamespace(returncode=7, stdout=b'out', stderr=b'failure')
        with mock.patch.object(snapshot, '_windows', return_value=True), \
             mock.patch.object(supervision, 'run_diagnostic', return_value=result):
            with self.assertRaises(subprocess.CalledProcessError) as raised:
                snapshot._git(self.root, 'status', env={'TASK_TEST': 'value'})
        self.assertEqual((raised.exception.returncode, raised.exception.output, raised.exception.stderr),
                         (7, b'out', b'failure'))
        with mock.patch.object(snapshot, '_windows', return_value=False), \
             mock.patch.object(subprocess, 'run', side_effect=subprocess.TimeoutExpired('git', 600)) as run:
            with self.assertRaises(subprocess.TimeoutExpired): snapshot._git(self.root, 'status')
        self.assertEqual(run.call_args.kwargs['timeout'], snapshot.GIT_TIMEOUT_SECONDS)

    def test_git_timeout_removes_private_index_and_leaves_real_index_unchanged(self):
        before = self.git('status', '--porcelain=v1', '-z', '--untracked-files=all')
        paths = []
        def timed_out(root, *args, env=None):
            paths.append(Path(env['GIT_INDEX_FILE']))
            if args[0] == 'add':
                raise subprocess.TimeoutExpired(args, snapshot.GIT_TIMEOUT_SECONDS)
            return b''
        with mock.patch.object(snapshot, '_git', side_effect=timed_out):
            with self.assertRaises(subprocess.TimeoutExpired):
                snapshot._capture_tree(self.root, self.head)
        self.assertTrue(paths)
        self.assertFalse(paths[0].parent.exists())
        self.assertEqual(before, self.git('status', '--porcelain=v1', '-z', '--untracked-files=all'))

    def test_capture_final_diff_uses_bounded_runner_and_propagates_timeout(self):
        tree = 'a' * 40
        commands = []
        def git(root, *args, env=None):
            commands.append(args)
            if args[0] == 'diff': raise subprocess.TimeoutExpired(args, snapshot.GIT_TIMEOUT_SECONDS)
            if args[:2] == ('rev-parse', 'HEAD'): return self.head.encode() + b'\n'
            if args[0] == 'ls-files': return b''
            raise AssertionError(args)
        with mock.patch.object(snapshot, '_capture_tree', return_value=(tree, 'digest', [])), \
             mock.patch.object(snapshot, '_git', side_effect=git):
            with self.assertRaises(subprocess.TimeoutExpired): snapshot.capture(self.root, self.head)
        self.assertEqual(commands[-1][0], 'diff')

    def test_archive_rejects_transient_same_size_restored_mutation(self):
        source = snapshot.capture(self.root, self.head)
        with tempfile.TemporaryDirectory() as external:
            copy = snapshot.materialize(self.root, source, Path(external) / 'source')
            target = copy / 'old.txt'
            original = target.read_bytes()
            info = target.stat()
            addfile = tarfile.TarFile.addfile
            def mutate_then_restore(tar, entry, fileobj=None):
                if entry.name != 'old.txt':
                    return addfile(tar, entry, fileobj)
                target.write_bytes(b'bad\n')
                try:
                    return addfile(tar, entry, fileobj)
                finally:
                    target.write_bytes(original)
                    os.utime(target, ns=(info.st_atime_ns, info.st_mtime_ns))
            with mock.patch.object(tarfile.TarFile, 'addfile', mutate_then_restore):
                with self.assertRaisesRegex(ValueError, 'archive bytes differ'):
                    snapshot.archive(self.root, source, copy, Path(external) / 'source.tar')

    def test_archive_rejects_corrupted_written_bytes(self):
        source = snapshot.capture(self.root, self.head)
        with tempfile.TemporaryDirectory() as external:
            copy = snapshot.materialize(self.root, source, Path(external) / 'source')
            real_open = tarfile.open
            def corrupt_before_read(*args, **kwargs):
                if kwargs.get('mode') == 'r:':
                    raw = kwargs['fileobj']
                    raw.seek(512)
                    raw.write(b'X')
                    raw.flush()
                    raw.seek(0)
                return real_open(*args, **kwargs)
            with mock.patch.object(tarfile, 'open', side_effect=corrupt_before_read):
                with self.assertRaisesRegex(ValueError, 'archive bytes differ'):
                    snapshot.archive(self.root, source, copy, Path(external) / 'source.tar')

    def test_archive_preserves_exact_snapshot_and_rejects_changed_copy(self):
        (self.root / 'old.txt').write_bytes(b'changed\r\n')
        source = snapshot.capture(self.root, self.head)
        with tempfile.TemporaryDirectory() as external:
            copy = snapshot.materialize(self.root, source, Path(external) / 'source')
            target = Path(external) / 'source.tar'
            digest = snapshot.archive(self.root, source, copy, target)
            self.assertEqual(digest, hashlib.sha256(target.read_bytes()).hexdigest())
            with tarfile.open(target) as archive:
                self.assertEqual(archive.extractfile('old.txt').read(), b'changed\r\n')
                self.assertEqual(set(archive.getnames()), {'.github/ci/path-filters.yml', 'old.txt', 'deleted.txt'})
            (copy / 'old.txt').write_bytes(b'tampered')
            with self.assertRaises(ValueError):
                snapshot.archive(self.root, source, copy, Path(external) / 'tampered.tar')

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

    def test_materialized_source_rejects_added_files_except_declared_outputs(self):
        source = snapshot.capture(self.root, self.head)
        with tempfile.TemporaryDirectory() as external:
            destination = Path(external) / 'source'
            snapshot.materialize(self.root, source, destination)
            generated = destination / 'generated'
            generated.mkdir()
            (generated / 'tool-output.txt').write_text('derived')
            with self.assertRaisesRegex(ValueError, 'unexpected'):
                snapshot.verify_materialized(self.root, source, destination)
            snapshot.verify_materialized(self.root, source, destination, allowed_extra=('generated/',))
            (destination / 'injected.go').write_text('package injected')
            with self.assertRaisesRegex(ValueError, 'unexpected'):
                snapshot.verify_materialized(self.root, source, destination, allowed_extra=('generated/',))
            for name in ('../generated/', '/generated/', './', 'generated/../other/'):
                with self.subTest(name=name), self.assertRaisesRegex(ValueError, 'invalid'):
                    snapshot.verify_materialized(self.root, source, destination, allowed_extra=(name,))

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
