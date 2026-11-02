"""Two captured module caches merge only with exact matching entries."""
import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

import local_go_multi_dependencies as subject


class MultiDependenciesTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = {'head': 'a' * 40}

    def archive(self, name, entries):
        path = self.root / name
        with tarfile.open(path, 'w') as tar:
            for entry, data in entries:
                info = tarfile.TarInfo(entry)
                info.size = len(data)
                tar.addfile(info, io.BytesIO(data))
        return path

    def receipt(self, module, path):
        return {'status': 'incomplete', 'source': self.source, 'module_dir': module,
                'archive_sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
                'toolchain_sha256': 'b' * 64, 'toolchain_version': 'go version go1.26.10 windows/amd64',
                'target': {'goos': 'linux', 'goarch': 'amd64', 'cgo_enabled': False}}

    def test_union_deduplicates_identical_files_and_rejects_conflicts(self):
        a = self.archive('a.tar', [('cache/download/a/@v/v1.mod', b'one')])
        b = self.archive('b.tar', [('cache/download/a/@v/v1.mod', b'one'), ('cache/download/b/@v/v1.mod', b'two')])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        output = self.root / 'merged.tar'
        result = subject.merge(rows, self.source, output)
        self.assertEqual(result['module_dirs'], list(subject.MODULES))
        with tarfile.open(output) as tar:
            self.assertEqual(tar.getnames(), ['cache/download/a/@v/v1.mod', 'cache/download/b/@v/v1.mod'])
        b2 = self.archive('b2.tar', [('cache/download/a/@v/v1.mod', b'different')])
        rows[1] = (self.receipt(subject.MODULES[1], b2), b2)
        with self.assertRaisesRegex(ValueError, 'conflict'):
            subject.merge(rows, self.source, self.root / 'bad.tar')

    def test_rejects_wrong_source_identity_paths_duplicates_and_bounds(self):
        for entries in ([('../escape', b'x')], [('cache/download/a/@v/v1.mod', b'x')] * 2,
                        [('cache/download/a/@v/v1.mod', b'x' * 33)]):
            with self.subTest(entries=entries), tempfile.TemporaryDirectory(dir=self.root) as directory:
                nested = Path(directory)
                a = self.archive(nested.name + 'a.tar', entries)
                b = self.archive(nested.name + 'b.tar', [('cache/download/b/@v/v1.mod', b'ok')])
                rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
                with mock.patch.object(subject, 'MAX_MEMBER', 32), self.assertRaises(ValueError):
                    subject.merge(rows, self.source, nested / 'out.tar')
                self.assertFalse((nested / 'out.tar').exists())
        a = self.archive('source-a.tar', [('cache/download/a/@v/v1.mod', b'x')])
        b = self.archive('source-b.tar', [('cache/download/b/@v/v1.mod', b'y')])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        rows[1][0]['source'] = {'head': 'different'}
        with self.assertRaises(ValueError):
            subject.merge(rows, self.source, self.root / 'wrong.tar')

    def test_mutated_archive_is_rejected(self):
        a = self.archive('mutable-a.tar', [('cache/download/a/@v/v1.mod', b'x')])
        b = self.archive('mutable-b.tar', [('cache/download/b/@v/v1.mod', b'y')])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        with a.open('ab') as stream:
            stream.write(b'changed')
        with self.assertRaisesRegex(ValueError, 'changed'):
            subject.merge(rows, self.source, self.root / 'out.tar')

    def test_output_archive_cap_counts_tar_framing_and_retains_only_bounded_partial(self):
        payload = b'x' * 4600
        a = self.archive('large-a.tar', [('cache/download/a/@v/v1.mod', payload)])
        b = self.archive('large-b.tar', [('cache/download/b/@v/v1.mod', payload)])
        self.assertEqual(a.stat().st_size, 10 * 1024)
        self.assertEqual(b.stat().st_size, 10 * 1024)
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        output = self.root / 'capped-merge.tar'

        # The payload total is only 9,200 bytes, but tar headers, padding, and
        # end blocks make the actual merged archive 12 KiB.
        with mock.patch.object(subject, 'MAX_ARCHIVE', 10 * 1024):
            with self.assertRaisesRegex(ValueError, 'limit'):
                subject.merge(rows, self.source, output)

        self.assertTrue(output.exists())
        self.assertGreater(output.stat().st_size, 0)
        self.assertLessEqual(output.stat().st_size, 10 * 1024)

    def test_exact_merged_archive_cap_succeeds_at_actual_tar_size(self):
        payload = b'x' * 4600
        a = self.archive('exact-a.tar', [('cache/download/a/@v/v1.mod', payload)])
        b = self.archive('exact-b.tar', [('cache/download/b/@v/v1.mod', payload)])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        measured = self.root / 'measured-merge.tar'

        with mock.patch.object(subject, 'MAX_ARCHIVE', 64 * 1024):
            subject.merge(rows, self.source, measured)
        expected_size = measured.stat().st_size
        measured.unlink()
        output = self.root / 'exact-merge.tar'

        with mock.patch.object(subject, 'MAX_ARCHIVE', expected_size):
            result = subject.merge(rows, self.source, output)

        self.assertEqual(output.stat().st_size, expected_size)
        self.assertEqual(result['archive_sha256'], hashlib.sha256(output.read_bytes()).hexdigest())
        with tarfile.open(output) as tar:
            self.assertEqual(tar.getnames(), [
                'cache/download/a/@v/v1.mod',
                'cache/download/b/@v/v1.mod',
            ])

    def test_input_archive_cap_applies_to_each_archive_before_merge(self):
        a = self.archive('input-cap-a.tar', [('cache/download/a/@v/v1.mod', b'a')])
        b = self.archive('input-cap-b.tar', [('cache/download/b/@v/v1.mod', b'b')])
        self.assertEqual(a.stat().st_size, 10 * 1024)
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        output = self.root / 'input-cap-out.tar'

        with mock.patch.object(subject, 'MAX_ARCHIVE', 10 * 1024 - 1):
            with self.assertRaises(ValueError):
                subject.merge(rows, self.source, output)

        self.assertFalse(output.exists())

    def test_cumulative_member_and_entry_caps_count_duplicate_input_entries(self):
        path = 'cache/download/shared/@v/v1.mod'
        payload = b'x' * 5000
        a = self.archive('duplicate-cap-a.tar', [(path, payload)])
        b = self.archive('duplicate-cap-b.tar', [(path, payload)])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]

        # Duplicate cache members are deduplicated in the result, but still
        # count against cumulative input quotas to prevent duplicate-input bypass.
        with mock.patch.object(subject, 'MAX_TOTAL', 2 * len(payload) - 1):
            with self.assertRaisesRegex(ValueError, 'limit'):
                subject.merge(rows, self.source, self.root / 'total-cap.tar')
        with mock.patch.object(subject, 'MAX_ENTRIES', 1):
            with self.assertRaisesRegex(ValueError, 'limit'):
                subject.merge(rows, self.source, self.root / 'entry-cap.tar')

    def test_exact_member_total_entry_and_archive_caps_succeed(self):
        a = self.archive('quota-a.tar', [('cache/download/a/@v/v1.mod', b'a' * 32)])
        b = self.archive('quota-b.tar', [('cache/download/b/@v/v1.mod', b'b' * 32)])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        output = self.root / 'quota-exact.tar'

        with mock.patch.object(subject, 'MAX_ARCHIVE', 10 * 1024), \
             mock.patch.object(subject, 'MAX_TOTAL', 64), \
             mock.patch.object(subject, 'MAX_MEMBER', 32), \
             mock.patch.object(subject, 'MAX_ENTRIES', 2):
            result = subject.merge(rows, self.source, output)

        self.assertEqual(output.stat().st_size, 10 * 1024)
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['module_dirs'], list(subject.MODULES))

    def test_exclusive_output_rejects_existing_file_without_replacing_it(self):
        a = self.archive('exclusive-a.tar', [('cache/download/a/@v/v1.mod', b'a')])
        b = self.archive('exclusive-b.tar', [('cache/download/b/@v/v1.mod', b'b')])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        output = self.root / 'existing.tar'
        sentinel = b'keep existing output bytes'
        output.write_bytes(sentinel)

        with self.assertRaises((FileExistsError, ValueError)):
            subject.merge(rows, self.source, output)

        self.assertEqual(output.read_bytes(), sentinel)

    def test_merge_rejects_mismatched_toolchain_or_target_identity(self):
        a = self.archive('identity-a.tar', [('cache/download/a/@v/v1.mod', b'a')])
        b = self.archive('identity-b.tar', [('cache/download/b/@v/v1.mod', b'b')])
        for field, changed in (
            ('toolchain_sha256', 'c' * 64),
            ('toolchain_version', 'go version go1.26.11 windows/amd64'),
            ('target', {'goos': 'windows', 'goarch': 'amd64', 'cgo_enabled': False}),
        ):
            with self.subTest(field=field):
                rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
                rows[1][0][field] = changed
                with self.assertRaises(ValueError):
                    subject.merge(rows, self.source, self.root / f'mismatch-{field}.tar')

    def test_source_archive_mutation_during_merge_rejects_receipt_and_keeps_output(self):
        a = self.archive('mutate-during-a.tar', [('cache/download/a/@v/v1.mod', b'a')])
        b = self.archive('mutate-during-b.tar', [('cache/download/b/@v/v1.mod', b'b')])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        output = self.root / 'mutate-during-output.tar'
        writer_type = subject.local_bounded_source._CappedWriter
        original_write = writer_type.write
        mutated = False

        def mutate_after_first_write(writer, raw):
            nonlocal mutated
            result = original_write(writer, raw)
            if not mutated:
                with a.open('ab') as stream:
                    stream.write(b'changed while merge was active')
                mutated = True
            return result

        with mock.patch.object(writer_type, 'write', mutate_after_first_write):
            with self.assertRaisesRegex(ValueError, 'changed during merge'):
                subject.merge(rows, self.source, output)

        self.assertTrue(mutated)
        self.assertTrue(output.exists())
        self.assertGreater(output.stat().st_size, 0)

    def test_output_write_failure_keeps_partial_archive_and_returns_no_receipt(self):
        a = self.archive('write-fail-a.tar', [('cache/download/a/@v/v1.mod', b'a')])
        b = self.archive('write-fail-b.tar', [('cache/download/b/@v/v1.mod', b'b')])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        output = self.root / 'write-failure.tar'
        writer_type = subject.local_bounded_source._CappedWriter
        original_write = writer_type.write
        prefix_written = False

        def fail_after_first_write(writer, raw):
            nonlocal prefix_written
            if not prefix_written:
                original_write(writer, raw[:64])
                prefix_written = True
            raise OSError('injected output write failure')

        with mock.patch.object(writer_type, 'write', fail_after_first_write):
            with self.assertRaisesRegex(OSError, 'injected output write failure'):
                subject.merge(rows, self.source, output)

        self.assertTrue(prefix_written)
        self.assertTrue(output.exists())
        self.assertEqual(output.stat().st_size, 64)

    def test_corruption_during_output_write_is_rejected(self):
        a = self.archive('write-a.tar', [('cache/download/a/@v/v1.mod', b'abc')])
        b = self.archive('write-b.tar', [('cache/download/b/@v/v1.mod', b'def')])
        rows = [(self.receipt(subject.MODULES[0], a), a), (self.receipt(subject.MODULES[1], b), b)]
        original = tarfile.TarFile.addfile
        def corrupt(tar, info, source=None):
            return original(tar, info, io.BytesIO(b'z' * info.size))
        with mock.patch.object(tarfile.TarFile, 'addfile', corrupt), \
             self.assertRaisesRegex(ValueError, 'merged.*differ'):
            subject.merge(rows, self.source, self.root / 'corrupt.tar')

    def test_transfer_requires_both_modules_and_verifies_user_cache(self):
        owner = mock.Mock()
        acquisition = {'module_dirs': list(subject.MODULES)}
        with mock.patch.object(subject.local_go_execution, 'transfer_dependencies', return_value={'status': 'incomplete'}) as transfer, \
             mock.patch.object(subject.local_go_execution, '_verify_modules') as verify:
            result = subject.transfer(owner, 'client', self.root, self.source, self.root, 'digest', acquisition, self.root / 'cache.tar')
        self.assertEqual(result['status'], 'incomplete')
        transfer.assert_called_once()
        verify.assert_called_once_with(owner, 'client', 'src/backend/user')
        with mock.patch.object(subject.local_go_execution, 'transfer_dependencies') as transfer, self.assertRaises(ValueError):
            subject.transfer(owner, 'client', self.root, self.source, self.root, 'digest', {'module_dirs': [subject.MODULES[0]]}, self.root / 'cache.tar')
        transfer.assert_not_called()

    def test_notification_acquire_forwards_bounded_output_to_each_module_separately(self):
        calls = []

        def acquire(root, source, source_copy, module, output, go_binary, *, bounded_output=False):
            calls.append((module, Path(output), go_binary, bounded_output))
            output = Path(output)
            output.mkdir(parents=True)
            archive = output / 'download-cache.tar'
            with tarfile.open(archive, 'w') as tar:
                data = module.encode()
                info = tarfile.TarInfo('cache/download/' + module.rsplit('/', 1)[-1] + '/@v/v1.mod')
                info.size = len(data)
                tar.addfile(info, io.BytesIO(data))
            return self.receipt(module, archive)

        outside = tempfile.TemporaryDirectory()
        self.addCleanup(outside.cleanup)
        output = Path(outside.name) / 'notification-output'
        with mock.patch.object(subject.snapshot, 'verify_materialized'), \
             mock.patch.object(subject.snapshot, 'capture', return_value=self.source), \
             mock.patch.object(subject.local_go_dependencies, 'acquire', side_effect=acquire):
            result = subject.acquire(self.root, self.source, self.root, output, bounded_output=True)

        self.assertEqual([row[0] for row in calls], list(subject.MODULES))
        self.assertTrue(all(row[3] is True for row in calls))
        self.assertEqual(len({row[1] for row in calls}), 2)
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['module_dirs'], list(subject.MODULES))

    def test_notification_bounded_mode_has_an_independent_budget_per_module(self):
        source_temp = tempfile.TemporaryDirectory()
        output_temp = tempfile.TemporaryDirectory()
        self.addCleanup(source_temp.cleanup)
        self.addCleanup(output_temp.cleanup)
        source_copy = Path(source_temp.name)
        output = Path(output_temp.name) / 'notification-output'
        module_names = {}
        module_mod_sums = {}
        module_gomod_bytes = {}
        for module in subject.MODULES:
            short = module.rsplit('/', 1)[-1]
            name = 'example.test/' + short
            module_names[module] = name
            directory = source_copy.joinpath(*module.split('/'))
            directory.mkdir(parents=True)
            gomod = directory / 'go.mod'
            gomod.write_text('module ' + name + '\ngo 1.26.0\n', encoding='utf-8')
            module_gomod_bytes[name] = gomod.read_bytes()
            module_mod_sums[name] = subject.local_go_dependencies._gomod_sum(gomod)
            (directory / 'go.sum').write_text(
                name + ' v1.0.0 h1:zip\n' + name + ' v1.0.0/go.mod ' + module_mod_sums[name] + '\n',
                encoding='utf-8')

        commands = []

        def execute(command, *, cwd=None, env=None, input=None, timeout, output_limit=None):
            commands.append((list(command), cwd, output_limit))
            if command[-1] == 'version':
                stdout, stderr = b'go version go1.26.10 windows/amd64\n', b''
            elif command[-5:] == ['list', '-deps', '-test', '-json', './...']:
                name = module_names[next(module for module in subject.MODULES if Path(cwd).name == module.rsplit('/', 1)[-1])]
                module = {'Path': name, 'Version': 'v1.0.0', 'Sum': 'h1:zip',
                          'GoModSum': module_mod_sums[name]}
                stdout = (json.dumps({'ImportPath': name, 'Module': module}) + '\n').encode()
                stderr = b''
            elif command[1:3] == ['mod', 'download']:
                records = []
                for argument in command[4:]:
                    name, version = argument.rsplit('@', 1)
                    cache = Path(env['GOMODCACHE']) / 'cache' / 'download' / name / '@v'
                    cache.mkdir(parents=True, exist_ok=True)
                    info, mod, archive = cache / (version + '.info'), cache / (version + '.mod'), cache / (version + '.zip')
                    info.write_bytes(b'info')
                    mod.write_bytes(module_gomod_bytes[name])
                    self.assertEqual(subject.local_go_dependencies._gomod_sum(mod), module_mod_sums[name])
                    archive.write_bytes(b'zip fixture')
                    Path(str(archive) + 'hash').write_text('h1:zip\n', encoding='utf-8')
                    records.append({'Path': name, 'Version': version, 'Sum': 'h1:zip',
                                    'GoModSum': module_mod_sums[name],
                                    'Info': str(info), 'GoMod': str(mod), 'Zip': str(archive)})
                stdout = b''.join((json.dumps(row, separators=(',', ':')) + '\n').encode() for row in records)
                stdout += b' ' * (3000 - len(stdout))
                stderr = b''
            elif command[1:3] == ['mod', 'verify']:
                stdout, stderr = b'all modules verified', b''
            else:
                raise AssertionError('unexpected Go command: ' + repr(command))
            return mock.Mock(returncode=0, stdout=stdout, stderr=stderr)

        with mock.patch.object(subject.snapshot, 'verify_materialized'), \
             mock.patch.object(subject.snapshot, 'capture', return_value=self.source), \
             mock.patch.object(subject.local_go_dependencies.snapshot, 'verify_materialized'), \
             mock.patch.object(subject.local_go_dependencies.snapshot, 'capture', return_value=self.source), \
             mock.patch.object(subject.local_go_dependencies.local_current_diff, '_windows', return_value=True), \
             mock.patch.object(subject.local_go_dependencies.local_current_diff, '_execute', side_effect=execute), \
             mock.patch.object(subject.local_go_dependencies, 'MAX_COMMAND_STREAM', 4096), \
             mock.patch.object(subject.local_go_dependencies, 'MAX_ACQUISITION_OUTPUT', 4096):
            result = subject.acquire(self.root, self.source, source_copy, output, sys.executable,
                                     bounded_output=True)

        self.assertEqual(result['status'], 'incomplete')
        self.assertTrue(all(output_limit == 4096 for _, _, output_limit in commands))
        persisted_names = ('dependencies.json', 'dependencies.stderr', 'download.json', 'download.stderr',
                           'verify-before-archive.stdout', 'verify-before-archive.stderr',
                           'verify-after-archive.stdout', 'verify-after-archive.stderr')
        per_module_sizes = [sum((output / module.rsplit('/', 1)[-1] / name).stat().st_size
                                for name in persisted_names) for module in subject.MODULES]
        self.assertTrue(all(size <= 4096 for size in per_module_sizes), per_module_sizes)
        self.assertGreater(sum(per_module_sizes), 4096)


if __name__ == '__main__':
    unittest.main()
