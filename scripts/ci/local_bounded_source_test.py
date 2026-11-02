"""Behavioral tests for bounded, read-only local Go source acquisition."""
import hashlib
import os
from pathlib import Path
import stat
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

import local_bounded_source as bounded_source
import local_source_snapshot as source_snapshot


class BoundedSourceArchiveTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'repo'
        self.root.mkdir()
        self.git('init', '-q')
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')
        self.git('config', 'core.autocrlf', 'false')
        (self.root / '.gitignore').write_bytes(b'ignored.bin\n')
        (self.root / 'dir').mkdir()
        self.files = {
            '.gitignore': b'ignored.bin\n',
            'dir/space name.txt': b'line one\nline two\n',
            'bytes.dat': b'\x00\x01bounded-source\xff',
        }
        for name, data in self.files.items():
            (self.root / name).write_bytes(data)
        self.git('add', '.')
        self.git('commit', '-qm', 'fixture source')
        self.head = self.git('rev-parse', 'HEAD').decode().strip()
        self.source = source_snapshot.capture(self.root, self.head)
        self.output_dir = Path(self.temp.name) / 'archives'
        self.output_dir.mkdir()

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE)

    def tree_state(self):
        index = (self.root / '.git/index').read_bytes()
        objects = {}
        for path in sorted((self.root / '.git/objects').rglob('*')):
            if path.is_file():
                relative = path.relative_to(self.root / '.git/objects').as_posix()
                objects[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
        refs = self.git('for-each-ref', '--format=%(refname) %(objectname)')
        status = self.git('status', '--porcelain=v1', '-z', '--untracked-files=all')
        return index, objects, refs, status

    def capture(self, name='source.tar', **limits):
        values = {'max_files': 10, 'max_source_bytes': 1024,
                  'max_archive_bytes': 1024 * 1024, 'timeout': 30}
        values.update(limits)
        return bounded_source.capture_archive(
            self.root, self.head, self.output_dir / name, **values)

    def test_archives_exact_committed_files_and_matches_source_identity(self):
        before = self.tree_state()
        receipt = self.capture()
        archive_path = self.output_dir / 'source.tar'

        self.assertEqual(set(receipt), {'schema', 'source', 'archive_sha256', 'archive_bytes'})
        self.assertEqual(receipt['schema'], 'voice-bounded-source-archive-v1')
        self.assertEqual(receipt['source'], self.source)
        self.assertEqual(receipt['archive_sha256'], hashlib.sha256(archive_path.read_bytes()).hexdigest())
        self.assertEqual(receipt['archive_bytes'], archive_path.stat().st_size)
        with tarfile.open(archive_path, mode='r:') as archive:
            members = archive.getmembers()
            names = [member.name for member in members]
            self.assertEqual(names, sorted(self.files, key=os.fsencode))
            for member in members:
                self.assertTrue(member.isfile())
                self.assertEqual(member.uid, 0)
                self.assertEqual(member.gid, 0)
                self.assertEqual(member.mtime, 0)
                self.assertEqual(member.uname, '')
                self.assertEqual(member.gname, '')
                self.assertEqual(archive.extractfile(member).read(), self.files[member.name])
                self.assertEqual(member.mode, stat.S_IMODE((self.root / member.name).stat().st_mode))
        self.assertEqual(self.tree_state(), before,
                         'source acquisition must not mutate the index, object database, refs, or worktree')

    def test_same_source_has_deterministic_archive_bytes(self):
        first = self.capture('first.tar')
        second = self.capture('second.tar')
        self.assertEqual(first['source'], second['source'])
        self.assertEqual(first['archive_sha256'], second['archive_sha256'])
        self.assertEqual((self.output_dir / 'first.tar').read_bytes(),
                         (self.output_dir / 'second.tar').read_bytes())

    def test_git_commands_disable_optional_repository_locks(self):
        real_execute = bounded_source.local_current_diff._execute
        observed = []

        def observe_environment(*args, **kwargs):
            for value in (*args, *kwargs.values()):
                if isinstance(value, dict) and 'GIT_OPTIONAL_LOCKS' in value:
                    observed.append(value['GIT_OPTIONAL_LOCKS'])
            return real_execute(*args, **kwargs)

        with mock.patch.object(bounded_source.local_current_diff, '_execute',
                               side_effect=observe_environment):
            self.capture('no-locks.tar')
        self.assertTrue(observed, 'producer must pass an explicit Git environment to its runner')
        self.assertTrue(all(value == '0' for value in observed))

    def test_source_validation_never_executes_configured_git_clean_filters(self):
        attributes = self.root / '.gitattributes'
        attributes.write_bytes(b'*.txt filter=probe\n')
        self.git('add', '.gitattributes')
        self.git('commit', '-qm', 'declare fixture clean filter')
        self.head = self.git('rev-parse', 'HEAD').decode().strip()
        self.source = source_snapshot.capture(self.root, self.head)

        marker = Path(self.temp.name) / 'clean-filter-invoked'
        filter_script = Path(self.temp.name) / 'clean_filter.py'
        filter_script.write_text(
            'import pathlib,sys\n'
            'pathlib.Path(sys.argv[1]).write_text("invoked", encoding="utf-8")\n'
            'sys.stdout.buffer.write(sys.stdin.buffer.read())\n', encoding='utf-8')
        command = f'"{sys.executable}" "{filter_script}" "{marker}"'
        self.git('config', 'filter.probe.clean', command)
        marker.unlink(missing_ok=True)

        receipt = self.capture('clean-filter.tar')
        self.assertEqual(receipt['source'], self.source)
        self.assertFalse(marker.exists(), 'Git source inspection must not execute user-configured clean filters')

    def test_git_runner_refuses_stdout_above_caller_limit(self):
        command = [sys.executable, '-c', 'import sys; sys.stdout.buffer.write(b"x" * 129)']
        result = bounded_source.local_current_diff._execute(
            command, timeout=10, output_limit=128)
        self.assertNotEqual(result.returncode, 0, 'overflow must become a failed subprocess result')
        self.assertLessEqual(len(result.stdout), 128, 'overflow output must stay capped in memory')

    def test_exact_file_and_source_byte_limits_are_accepted(self):
        source_bytes = sum(len(data) for data in self.files.values())
        receipt = self.capture(max_files=len(self.files), max_source_bytes=source_bytes)
        self.assertEqual(receipt['source'], self.source)

    def test_count_and_source_byte_overflow_reject_before_output_creation(self):
        cases = (
            ('too-many-files.tar', {'max_files': len(self.files) - 1}),
            ('too-many-bytes.tar', {'max_source_bytes': sum(map(len, self.files.values())) - 1}),
        )
        for name, limits in cases:
            with self.subTest(limits=limits):
                output = self.output_dir / name
                with self.assertRaises((ValueError, OSError)):
                    self.capture(name, **limits)
                self.assertFalse(output.exists())

    def test_archive_write_limit_keeps_only_bounded_partial_output_without_receipt(self):
        good = self.capture('sizing.tar')
        cap = good['archive_bytes'] - 1
        output = self.output_dir / 'capped.tar'
        with self.assertRaises((ValueError, OSError)):
            self.capture('capped.tar', max_archive_bytes=cap)
        self.assertTrue(output.is_file(), 'failed capture preserves its owned partial archive for reconciliation')
        self.assertLessEqual(output.stat().st_size, cap)
        self.assertFalse((self.output_dir / 'capped.receipt.json').exists())

    def test_existing_output_and_missing_parent_are_rejected_without_replacement(self):
        existing = self.output_dir / 'existing.tar'
        existing.write_bytes(b'preserve-existing-output')
        with self.assertRaises((ValueError, FileExistsError)):
            bounded_source.capture_archive(
                self.root, self.head, existing, max_files=10,
                max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
        self.assertEqual(existing.read_bytes(), b'preserve-existing-output')
        missing_parent = self.output_dir / 'missing' / 'source.tar'
        with self.assertRaises((ValueError, FileNotFoundError, OSError)):
            bounded_source.capture_archive(
                self.root, self.head, missing_parent, max_files=10,
                max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
        self.assertFalse(missing_parent.parent.exists())

    def test_invalid_or_unbounded_limits_reject_without_creating_output(self):
        invalid = (
            {'max_files': True}, {'max_files': 0}, {'max_files': -1},
            {'max_files': bounded_source.MAX_FILES + 1},
            {'max_source_bytes': False}, {'max_source_bytes': 0}, {'max_source_bytes': -1},
            {'max_source_bytes': bounded_source.MAX_SOURCE_BYTES + 1},
            {'max_archive_bytes': False}, {'max_archive_bytes': 0}, {'max_archive_bytes': -1},
            {'max_archive_bytes': bounded_source.MAX_ARCHIVE_BYTES + 1},
            {'max_archive_bytes': 1.5},
            {'timeout': False}, {'timeout': 0}, {'timeout': -1}, {'timeout': 1.5},
            {'timeout': bounded_source.MAX_SECONDS + 1},
        )
        for index, limits in enumerate(invalid):
            output = self.output_dir / f'invalid-{index}.tar'
            with self.subTest(limits=limits), self.assertRaises((ValueError, TypeError)):
                values = {'max_files': 10, 'max_source_bytes': 1024,
                          'max_archive_bytes': 1024 * 1024, 'timeout': 30}
                values.update(limits)
                bounded_source.capture_archive(self.root, self.head, output, **values)
            self.assertFalse(output.exists())

    def test_wrong_head_and_untracked_source_reject_before_output_but_dirty_bytes_never_get_receipt(self):
        with self.assertRaises(ValueError):
            bounded_source.capture_archive(
                self.root, '0' * 40, self.output_dir / 'wrong-head.tar', max_files=10,
                max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
        self.assertFalse((self.output_dir / 'wrong-head.tar').exists())

        source_file = self.root / 'bytes.dat'
        original = source_file.read_bytes()
        source_file.write_bytes(original + b'dirty')
        with self.assertRaises((ValueError, OSError)):
            self.capture('dirty.tar')
        dirty_archive = self.output_dir / 'dirty.tar'
        self.assertTrue(dirty_archive.is_file(), 'post-open source drift preserves its owned partial archive')
        self.assertLessEqual(dirty_archive.stat().st_size, 1024 * 1024)
        self.assertFalse((self.output_dir / 'dirty.receipt.json').exists())
        source_file.write_bytes(original)

        (self.root / 'untracked.txt').write_bytes(b'uncommitted')
        with self.assertRaises((ValueError, OSError)):
            self.capture('untracked.tar')
        self.assertFalse((self.output_dir / 'untracked.tar').exists())

    def test_ignored_files_are_not_silently_added_to_archive(self):
        ignored = self.root / 'ignored.bin'
        ignored.write_bytes(b'ignored build output')
        receipt = self.capture('ignored.tar')
        with tarfile.open(self.output_dir / 'ignored.tar', mode='r:') as archive:
            names = archive.getnames()
        self.assertNotIn('ignored.bin', names)
        self.assertEqual(receipt['source'], self.source)

    def test_link_and_gitlink_tree_entries_are_rejected_without_writing_output(self):
        link_target = self.root / 'link-target-fixture'
        link_target.write_bytes(b'target')
        link_blob = self.git('hash-object', '-w', str(link_target)).decode().strip()
        for index, mode, oid in ((0, '120000', link_blob), (1, '160000', self.head)):
            with self.subTest(mode=mode):
                self.git('update-index', '--add', '--cacheinfo', f'{mode},{oid},unsupported-{index}')
                self.git('commit', '-qm', 'unsupported source tree entry')
                self.head = self.git('rev-parse', 'HEAD').decode().strip()
                output = self.output_dir / f'unsupported-{mode}.tar'
                with self.assertRaises((ValueError, OSError)):
                    bounded_source.capture_archive(
                        self.root, self.head, output, max_files=10,
                        max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
                self.assertFalse(output.exists())

    def test_git_errors_and_timeouts_fail_closed_without_success_receipt(self):
        output = self.output_dir / 'git-failure.tar'
        with mock.patch.object(bounded_source, '_git', side_effect=subprocess.CalledProcessError(1, ['git'])):
            with self.assertRaises(subprocess.CalledProcessError):
                bounded_source.capture_archive(
                    self.root, self.head, output, max_files=10,
                    max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
        self.assertFalse(output.exists())
        self.assertFalse((self.output_dir / 'git-failure.receipt.json').exists())

        timeout_output = self.output_dir / 'git-timeout.tar'
        with mock.patch.object(bounded_source, '_git', side_effect=subprocess.TimeoutExpired(['git'], 1)):
            with self.assertRaises(subprocess.TimeoutExpired):
                bounded_source.capture_archive(
                    self.root, self.head, timeout_output, max_files=10,
                    max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=1)
        self.assertFalse(timeout_output.exists())

    def test_sink_write_failure_preserves_partial_output_and_returns_no_receipt(self):
        output = self.output_dir / 'sink-failure.tar'
        with mock.patch.object(bounded_source._CappedWriter, 'write', side_effect=OSError('simulated disk full')):
            with self.assertRaises(OSError):
                bounded_source.capture_archive(
                    self.root, self.head, output, max_files=10,
                    max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
        self.assertTrue(output.exists())
        self.assertFalse((self.output_dir / 'sink-failure.receipt.json').exists())

    def test_stream_read_failure_preserves_partial_output_and_returns_no_receipt(self):
        output = self.output_dir / 'stream-failure.tar'
        original_visit = bounded_source._visit

        def fail_during_stream(root, entries, deadline, tar=None):
            if tar is not None:
                raise OSError('simulated source read failure')
            return original_visit(root, entries, deadline, tar=tar)

        with mock.patch.object(bounded_source, '_visit', side_effect=fail_during_stream):
            with self.assertRaisesRegex(OSError, 'simulated source read failure'):
                bounded_source.capture_archive(
                    self.root, self.head, output, max_files=10,
                    max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
        self.assertTrue(output.exists())
        self.assertFalse((self.output_dir / 'stream-failure.receipt.json').exists())

    def test_archive_readback_corruption_fails_without_returning_receipt(self):
        output = self.output_dir / 'readback-corruption.tar'
        original_verify = bounded_source._verify_archive

        def corrupt_then_verify(raw, entries, expected_hash, expected_worktree, deadline):
            raw.seek(512)
            raw.write(b'X')
            raw.flush()
            raw.seek(0)
            return original_verify(raw, entries, expected_hash, expected_worktree, deadline)

        with mock.patch.object(bounded_source, '_verify_archive', side_effect=corrupt_then_verify):
            with self.assertRaises((ValueError, OSError)):
                bounded_source.capture_archive(
                    self.root, self.head, output, max_files=10,
                    max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
        self.assertTrue(output.exists())
        self.assertFalse((self.output_dir / 'readback-corruption.receipt.json').exists())

    def test_source_mutation_after_streaming_revalidation_fails_and_retains_partial_archive(self):
        output = self.output_dir / 'mutated.tar'
        original_visit = bounded_source._visit
        changed = False

        def mutate_after_visit(root, entries, deadline, tar=None):
            nonlocal changed
            result = original_visit(root, entries, deadline, tar=tar)
            if tar is not None and not changed:
                changed = True
                (self.root / 'bytes.dat').write_bytes(b'changed after archive read')
            return result

        with mock.patch.object(bounded_source, '_visit', side_effect=mutate_after_visit):
            with self.assertRaises((ValueError, OSError)):
                bounded_source.capture_archive(
                    self.root, self.head, output, max_files=10,
                    max_source_bytes=1024, max_archive_bytes=1024 * 1024, timeout=30)
        self.assertTrue(changed, 'mutation injection must occur after the actual stream pass')
        self.assertTrue(output.exists())
        self.assertFalse((self.output_dir / 'mutated.receipt.json').exists())


if __name__ == '__main__':
    unittest.main()
