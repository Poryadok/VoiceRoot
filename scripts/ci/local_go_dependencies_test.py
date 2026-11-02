"""Tests for source-bound private Go module cache acquisition."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import local_go_dependencies as dependencies


class GoAcquisitionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'root'
        self.copy = self.base / 'source'
        self.root.mkdir()
        (self.copy / 'module').mkdir(parents=True)
        (self.copy / 'module/go.mod').write_text('module example.test/module\n', encoding='utf-8')
        self.sums = self.copy / 'module/go.sum'
        fixture_mod = self.base / 'fixture.mod'
        fixture_mod.write_bytes(b'fixture')
        self.mod_sum = dependencies._gomod_sum(fixture_mod)
        self.sums.write_text('example.test/dependency v1.0.0 h1:zip\n'
                             'example.test/dependency v1.0.0/go.mod ' + self.mod_sum + '\n'
                             'example.test/Extra v1.0.0/go.mod ' + self.mod_sum + '\n', encoding='utf-8')
        self.source = {'head': 'a' * 40}
        self.verify_status = 0
        self.verify_calls = 0
        self.fail_second_verify = False
        self.bounded_calls = []
        self.bounded_required = None
        self.bounded_download_sizes = []
        self.bounded_download_index = 0
        self.bounded_timeout = None
        self.bounded_returncodes = {}
        self.bounded_list_stdout = None
        self.bounded_list_stderr = b'list diagnostic\n'
        self.bounded_download_stderr = b'download diagnostic\n'
        self.bounded_verify_stdout = b'all modules verified\n'
        self.bounded_verify_stderr = b''
        self.bounded_outputs = []

    def run_go(self, command, **kwargs):
        if command[-1] == 'version':
            return SimpleNamespace(returncode=0, stdout=b'go version go1.26.2 test/amd64\n')
        if command[-1] == 'verify':
            self.verify_calls += 1
            code = 1 if self.fail_second_verify and self.verify_calls == 2 else self.verify_status
            return SimpleNamespace(returncode=code, stdout=b'all modules verified\n', stderr=b'')
        if command[-5:] == ['list', '-deps', '-test', '-json', './...']:
            self.assertEqual(kwargs['env']['GOMAXPROCS'], '2')
            graph = Path(kwargs['env']['GOMODCACHE']) / 'cache/download/example.test/!extra/@v'
            graph.mkdir(parents=True)
            (graph / 'v1.0.0.mod').write_bytes(b'fixture')
            (graph / 'v1.0.0.info').write_bytes(b'info')
            (graph / 'v2.0.0.mod').write_bytes(b'unbound')
            module = {'Path': 'example.test/dependency', 'Version': 'v1.0.0',
                      'Sum': 'h1:zip', 'GoModSum': self.mod_sum}
            kwargs['stdout'].write((json.dumps({'ImportPath': 'example.test/dependency', 'Module': module}) + '\n').encode())
            return SimpleNamespace(returncode=0)
        self.assertEqual(command[-3:], ['download', '-json', 'example.test/dependency@v1.0.0'])
        cache = Path(kwargs['env']['GOMODCACHE']) / 'cache/download/example.test/dependency/@v'
        cache.mkdir(parents=True)
        info, mod, zipfile = (cache / 'v1.0.0.info', cache / 'v1.0.0.mod', cache / 'v1.0.0.zip')
        for file in (info, mod, zipfile):
            file.write_bytes(b'fixture')
        Path(str(zipfile) + 'hash').write_text('h1:zip\n', encoding='utf-8')
        record = {'Path': 'example.test/dependency', 'Version': 'v1.0.0', 'Sum': 'h1:zip',
                  'GoModSum': self.mod_sum, 'Info': str(info), 'GoMod': str(mod), 'Zip': str(zipfile)}
        kwargs['stdout'].write((json.dumps(record) + '\n').encode())
        return SimpleNamespace(returncode=0)

    def acquire(self):
        with patch.object(dependencies.snapshot, 'capture', return_value=self.source), \
             patch.object(dependencies.snapshot, 'verify_materialized'), \
             patch.object(dependencies.subprocess, 'run', side_effect=self.run_go):
            return dependencies.acquire(self.root, self.source, self.copy, 'module', self.base / 'output', sys.executable)

    def bounded_execute(self, command, *, cwd=None, env=None, timeout, output_limit=None):
        self.bounded_calls.append((list(command), cwd, timeout, output_limit))
        self.assertEqual(output_limit, dependencies.MAX_COMMAND_STREAM)
        if self.bounded_timeout is not None and self.bounded_timeout(command):
            raise subprocess.TimeoutExpired(command, timeout, output=b'partial stdout', stderr=b'partial stderr')
        if command[-1] == 'version':
            stage = 'version'
            stdout, stderr = b'go version go1.26.2 test/amd64\n', b'version diagnostic\n'
        elif command[-5:] == ['list', '-deps', '-test', '-json', './...']:
            stage = 'list'
            graph = Path(env['GOMODCACHE']) / 'cache/download/example.test/!extra/@v'
            graph.mkdir(parents=True)
            (graph / 'v1.0.0.mod').write_bytes(b'fixture')
            (graph / 'v1.0.0.info').write_bytes(b'info')
            (graph / 'v2.0.0.mod').write_bytes(b'unbound')
            if self.bounded_list_stdout is None:
                module = {'Path': 'example.test/dependency', 'Version': 'v1.0.0',
                          'Sum': 'h1:zip', 'GoModSum': self.mod_sum}
                stdout = (json.dumps({'ImportPath': 'example.test/dependency', 'Module': module}) + '\n').encode()
            else:
                stdout = self.bounded_list_stdout
            stderr = self.bounded_list_stderr
        elif command[1:3] == ['mod', 'download']:
            stage = 'download'
            records = []
            for argument in command[4:]:
                module_name, version = argument.rsplit('@', 1)
                module_sum, mod_sum = ((self.bounded_required or {}).get((module_name, version),
                                    ('h1:zip', self.mod_sum)))
                cache = Path(env['GOMODCACHE']) / ('cache/download/' + module_name + '/@v')
                cache.mkdir(parents=True, exist_ok=True)
                info, mod, zipfile = (cache / (version + '.info'), cache / (version + '.mod'),
                                      cache / (version + '.zip'))
                info.write_bytes(b'info')
                mod.write_bytes(b'fixture')
                zipfile.write_bytes(b'fixture')
                Path(str(zipfile) + 'hash').write_text(module_sum + '\n', encoding='utf-8')
                records.append({'Path': module_name, 'Version': version, 'Sum': module_sum,
                                'GoModSum': mod_sum, 'Info': str(info), 'GoMod': str(mod), 'Zip': str(zipfile)})
            stdout = b''.join((json.dumps(record, separators=(',', ':')) + '\n').encode() for record in records)
            if self.bounded_download_index < len(self.bounded_download_sizes):
                target_size = self.bounded_download_sizes[self.bounded_download_index]
                if len(stdout) > target_size:
                    raise AssertionError('download fixture exceeds requested padded size')
                stdout += b' ' * (target_size - len(stdout))
            self.bounded_download_index += 1
            stderr = self.bounded_download_stderr
        elif command[1:3] == ['mod', 'verify']:
            stage = 'verify'
            self.verify_calls += 1
            code = self.bounded_returncodes.get(('verify', self.verify_calls), self.verify_status)
            result = SimpleNamespace(returncode=code, stdout=self.bounded_verify_stdout,
                                     stderr=self.bounded_verify_stderr)
            self.bounded_outputs.append((stage, result.stdout, result.stderr))
            return result
        elif command[-4:] == ['list', '-m', '-json', 'voice/backend/pkg']:
            stage = 'local'
            stdout, stderr = b'{"Path":"voice/backend/pkg"}\n', b'local stderr not persisted\n'
        else:
            raise AssertionError('unexpected bounded Go command: ' + repr(command))
        result = SimpleNamespace(returncode=self.bounded_returncodes.get((stage, len(self.bounded_calls)), 0),
                                 stdout=stdout, stderr=stderr)
        self.bounded_outputs.append((stage, stdout, stderr))
        return result

    def bounded_acquire(self, output=None, *, runner=None):
        output = output or (self.base / 'bounded-output')
        self.bounded_download_index = 0
        with patch.object(dependencies.snapshot, 'capture', return_value=self.source), \
             patch.object(dependencies.snapshot, 'verify_materialized'), \
             patch.object(dependencies.local_current_diff, '_windows', return_value=True), \
             patch.object(dependencies.local_current_diff, '_execute', side_effect=runner or self.bounded_execute):
            return dependencies.acquire(self.root, self.source, self.copy, 'module', output, sys.executable,
                                         bounded_output=True)

    def many_required_modules(self, count):
        modules = {}
        sums = []
        for index in range(count):
            name, version, module_sum = f'example.test/dependency{index:03d}', 'v1.0.0', 'h1:zip'
            modules[(name, version)] = (module_sum, self.mod_sum)
            sums.extend((f'{name} {version} {module_sum}', f'{name} {version}/go.mod {self.mod_sum}'))
        self.sums.write_text('\n'.join(sums) + '\n', encoding='utf-8')
        self.bounded_required = modules
        self.bounded_list_stdout = b''.join(
            (json.dumps({'ImportPath': name, 'Module': {
                'Path': name, 'Version': version, 'Sum': module_sum, 'GoModSum': mod_sum}}) + '\n').encode()
            for (name, version), (module_sum, mod_sum) in modules.items())
        return modules

    def test_bounded_capture_persists_original_json_streams_without_a_receipt(self):
        result = self.bounded_acquire()
        output = self.base / 'bounded-output'
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual((output / 'dependencies.json').read_bytes(), self.bounded_outputs[1][1])
        self.assertEqual((output / 'dependencies.stderr').read_bytes(), self.bounded_outputs[1][2])
        self.assertEqual((output / 'download.json').read_bytes(), self.bounded_outputs[2][1])
        self.assertEqual((output / 'download.stderr').read_bytes(), self.bounded_outputs[2][2])
        self.assertEqual((output / 'verify-before-archive.stdout').read_bytes(), self.bounded_outputs[3][1])
        self.assertEqual((output / 'verify-before-archive.stderr').read_bytes(), self.bounded_outputs[3][2])
        self.assertEqual((output / 'verify-after-archive.stdout').read_bytes(), self.bounded_outputs[4][1])
        self.assertEqual((output / 'verify-after-archive.stderr').read_bytes(), self.bounded_outputs[4][2])
        self.assertTrue(all(call[3] == dependencies.MAX_COMMAND_STREAM for call in self.bounded_calls))
        self.assertFalse(any(path.name in {'receipt.json', 'complete.json'} for path in output.iterdir()))
        self.assertNotIn('version diagnostic', ''.join(path.read_text(errors='ignore') for path in output.iterdir()
                                                        if path.is_file()))

    def test_bounded_pipe_wrapper_accepts_exact_limit_and_rejects_each_stream_overflow(self):
        limit = 256

        def execute(stdout_bytes, stderr_bytes):
            code = ('import sys;sys.stdout.buffer.write(b"x"*%d);'
                    'sys.stderr.buffer.write(b"y"*%d)' % (stdout_bytes, stderr_bytes))
            return dependencies.local_current_diff._execute(
                [sys.executable, '-B', '-c', code], timeout=10, output_limit=limit)

        with patch.object(dependencies.local_current_diff, '_windows', return_value=False):
            exact = execute(limit, limit)
            self.assertEqual(exact.returncode, 0)
            self.assertEqual(exact.stdout, b'x' * limit)
            self.assertEqual(exact.stderr, b'y' * limit)
            stdout_overflow = execute(limit + 1, 0)
            self.assertEqual(stdout_overflow.returncode, 125)
            self.assertEqual(stdout_overflow.stdout, b'x' * limit)
            self.assertLessEqual(len(stdout_overflow.stderr), limit)
            stderr_overflow = execute(0, limit + 1)
            self.assertEqual(stderr_overflow.returncode, 125)
            self.assertEqual(stderr_overflow.stderr, b'y' * limit)
            self.assertLessEqual(len(stderr_overflow.stdout), limit)

    @unittest.skipUnless(os.name == 'nt', 'Windows Job descendant cleanup is required')
    def test_bounded_pipe_overflow_cleans_owned_descendants(self):
        limit = 256
        marker = self.base / 'descendant-started'
        descendant = ('import time\np=%r\nwhile True:\n open(p,"ab").write(b"x")\n time.sleep(.02)\n' % str(marker))
        parent = ('import subprocess,sys,time,os\n'
                  'subprocess.Popen([sys.executable,"-I","-S","-c",%r])\n'
                  'deadline=time.monotonic()+5\n'
                  'while not os.path.exists(%r) and time.monotonic()<deadline:\n time.sleep(.01)\n'
                  'sys.stdout.buffer.write(b"x"*(%d+1))\n'
                  'sys.stdout.flush()\n'
                  'time.sleep(20)\n' % (descendant, str(marker), limit))
        with patch.object(dependencies.local_current_diff, '_windows', return_value=True):
            result = dependencies.local_current_diff._execute(
                [sys.executable, '-I', '-S', '-c', parent], timeout=10, output_limit=limit)
        self.assertTrue(marker.exists())
        self.assertIn(result.returncode, (124, 125))
        self.assertLessEqual(len(result.stdout), limit)
        marker_size = marker.stat().st_size
        time.sleep(.2)
        self.assertEqual(marker.stat().st_size, marker_size)

    def test_bounded_mode_refuses_without_windows_job_before_output_creation(self):
        output = self.base / 'refused-output'
        with patch.object(dependencies.snapshot, 'capture', return_value=self.source), \
             patch.object(dependencies.local_current_diff, '_windows', return_value=False), \
             patch.object(dependencies.local_current_diff, '_execute') as execute:
            with self.assertRaisesRegex(ValueError, 'Windows Job'):
                dependencies.acquire(self.root, self.source, self.copy, 'module', output, sys.executable,
                                     bounded_output=True)
        execute.assert_not_called()
        self.assertFalse(output.exists())

    def test_bounded_wrapper_sentinel_codes_fail_without_persisting_or_receipt(self):
        for sentinel in (124, 125, 126):
            with self.subTest(sentinel=sentinel):
                output = self.base / ('sentinel-' + str(sentinel))
                self.bounded_calls.clear()
                self.bounded_outputs.clear()
                self.bounded_returncodes = {('list', 2): sentinel}
                with self.assertRaisesRegex(ValueError, 'supervision failed'):
                    self.bounded_acquire(output)
                self.assertFalse((output / 'dependencies.json').exists())
                self.assertFalse((output / 'receipt.json').exists())
                self.assertFalse((output / 'cache.tar').exists())
        self.bounded_returncodes = {}

    def test_bounded_nonzero_command_preserves_raw_output_without_success_receipt(self):
        self.bounded_returncodes = {('list', 2): 7}
        output = self.base / 'nonzero-output'
        with self.assertRaisesRegex(ValueError, 'package discovery failed'):
            self.bounded_acquire(output)
        self.assertEqual((output / 'dependencies.json').read_bytes(), self.bounded_outputs[1][1])
        self.assertEqual((output / 'dependencies.stderr').read_bytes(), self.bounded_outputs[1][2])
        self.assertFalse((output / 'receipt.json').exists())
        self.assertFalse((output / 'download-cache.tar').exists())

    def test_bounded_malformed_utf8_is_preserved_then_rejected_without_receipt(self):
        self.bounded_list_stdout = b'\xff\n'
        output = self.base / 'invalid-utf8-output'
        with self.assertRaises(UnicodeDecodeError):
            self.bounded_acquire(output)
        self.assertEqual((output / 'dependencies.json').read_bytes(), b'\xff\n')
        self.assertTrue((output / 'dependencies.stderr').is_file())
        self.assertFalse((output / 'receipt.json').exists())
        self.assertFalse((output / 'download-cache.tar').exists())

    def test_bounded_command_timeout_preserves_prior_batch_without_receipt(self):
        self.many_required_modules(51)
        self.bounded_download_sizes = [40000, 20000]
        self.bounded_timeout = lambda command: (
            command[1:3] == ['mod', 'download'] and command[-1].endswith('dependency050@v1.0.0'))
        output = self.base / 'timeout-output'
        with patch.object(dependencies, 'MAX_COMMAND_STREAM', 45000):
            with self.assertRaises(subprocess.TimeoutExpired):
                self.bounded_acquire(output)
        self.assertGreater((output / 'download.json').stat().st_size, 0)
        self.assertGreater((output / 'download.stderr').stat().st_size, 0)
        self.assertFalse((output / 'verify-before-archive.stdout').exists())
        self.assertFalse((output / 'receipt.json').exists())
        self.assertFalse((output / 'cache.tar').exists())

    def test_bounded_cumulative_budget_counts_batches_and_verification_streams(self):
        self.many_required_modules(51)
        self.bounded_download_sizes = [40000, 20000]
        self.bounded_download_stderr = b'd' * 17
        self.bounded_verify_stdout = b'all modules verified'
        self.bounded_verify_stderr = b' ' * 23
        output_names = ('budget-a', 'budget-b')
        measured = self.base / output_names[0]
        with patch.object(dependencies, 'MAX_COMMAND_STREAM', 45000), \
             patch.object(dependencies, 'MAX_ACQUISITION_OUTPUT', 2 * 1024 * 1024):
            self.bounded_acquire(measured)
        persisted = [measured / name for name in (
            'dependencies.json', 'dependencies.stderr', 'download.json', 'download.stderr',
            'verify-before-archive.stdout', 'verify-before-archive.stderr',
            'verify-after-archive.stdout', 'verify-after-archive.stderr')]
        exact_budget = sum(path.stat().st_size for path in persisted)

        self.bounded_calls.clear()
        self.bounded_outputs.clear()
        exact = self.base / output_names[1]
        with patch.object(dependencies, 'MAX_COMMAND_STREAM', 45000), \
             patch.object(dependencies, 'MAX_ACQUISITION_OUTPUT', exact_budget):
            self.bounded_acquire(exact)
        self.assertEqual(sum((exact / path.name).stat().st_size for path in persisted), exact_budget)
        self.assertEqual(len([call for call in self.bounded_calls if call[0][1:3] == ['mod', 'download']]), 2)

        self.bounded_calls.clear()
        self.bounded_outputs.clear()
        overflow = self.base / 'budget-c'
        with patch.object(dependencies, 'MAX_COMMAND_STREAM', 45000), \
             patch.object(dependencies, 'MAX_ACQUISITION_OUTPUT', exact_budget - 1):
            with self.assertRaisesRegex(ValueError, 'cumulative limit'):
                self.bounded_acquire(overflow)
        for name in ('dependencies.json', 'dependencies.stderr', 'download.json', 'download.stderr',
                     'verify-before-archive.stdout', 'verify-before-archive.stderr'):
            self.assertTrue((overflow / name).is_file(), name)
        self.assertFalse((overflow / 'verify-after-archive.stdout').exists())
        self.assertFalse((overflow / 'verify-after-archive.stderr').exists())
        self.assertFalse((overflow / 'receipt.json').exists())
        self.assertFalse((overflow / 'cache.tar').exists())

    def test_bounded_download_batch_budget_rejects_next_batch_before_append(self):
        self.many_required_modules(51)
        self.bounded_download_sizes = [40000, 20000]
        self.bounded_download_stderr = b'd' * 17
        output = self.base / 'batch-budget'

        def runner(command, *, cwd=None, env=None, timeout, output_limit=None):
            result = self.bounded_execute(command, cwd=cwd, env=env, timeout=timeout, output_limit=output_limit)
            if command[1:3] == ['mod', 'download'] and command[-1].endswith('dependency050@v1.0.0'):
                prior = sum(len(stdout) + len(stderr) for stage, stdout, stderr in self.bounded_outputs[:-1]
                            if stage in ('list', 'download'))
                dependencies.MAX_ACQUISITION_OUTPUT = prior + len(result.stdout) + len(result.stderr) - 1
            return result

        with patch.object(dependencies, 'MAX_COMMAND_STREAM', 45000), \
             patch.object(dependencies, 'MAX_ACQUISITION_OUTPUT', 2 * 1024 * 1024):
            with self.assertRaisesRegex(ValueError, 'cumulative limit'):
                self.bounded_acquire(output, runner=runner)
        self.assertEqual((output / 'download.json').read_bytes(), self.bounded_outputs[2][1])
        self.assertEqual((output / 'download.stderr').read_bytes(), self.bounded_outputs[2][2])
        self.assertFalse((output / 'verify-before-archive.stdout').exists())
        self.assertFalse((output / 'receipt.json').exists())
        self.assertFalse((output / 'download-cache.tar').exists())

    def test_bounded_cumulative_budget_includes_local_replacement_json(self):
        self.bounded_returncodes = {('verify', 1): 1}
        # A local replacement inspection is only captured after verify reports failure.
        output = self.base / 'local-budget'
        def runner(command, *, cwd=None, env=None, timeout, output_limit=None):
            result = self.bounded_execute(command, cwd=cwd, env=env, timeout=timeout, output_limit=output_limit)
            if command[-4:] == ['list', '-m', '-json', 'voice/backend/pkg']:
                persisted = sum(len(stdout) + len(stderr) for stage, stdout, stderr in self.bounded_outputs
                                if stage in ('list', 'download', 'verify'))
                dependencies.MAX_ACQUISITION_OUTPUT = persisted + len(result.stdout) - 1
            return result
        with patch.object(dependencies.local_go_verify, 'is_local_failure', return_value=True), \
             patch.object(dependencies, 'MAX_ACQUISITION_OUTPUT', 1024 * 1024):
            with self.assertRaisesRegex(ValueError, 'cumulative limit'):
                self.bounded_acquire(output, runner=runner)
        self.assertTrue((output / 'dependencies.json').is_file())
        self.assertTrue((output / 'download.json').is_file())
        self.assertTrue((output / 'verify-before-archive.stdout').is_file())
        self.assertFalse((output / 'local-module-before-archive.json').exists())
        self.assertFalse((output / 'receipt.json').exists())
        self.assertFalse((output / 'cache.tar').exists())
        self.bounded_returncodes = {}

    def test_legacy_acquisition_default_keeps_unbounded_subprocess_path(self):
        with patch.object(dependencies.local_current_diff, '_execute') as execute:
            self.acquire()
        execute.assert_not_called()

    def test_archive_contains_only_sum_bound_download_artifacts(self):
        result = self.acquire()
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['module_count'], 1)
        self.assertEqual(self.verify_calls, 2)
        with tarfile.open(self.base / 'output/download-cache.tar') as archive:
            self.assertEqual(len(archive.getmembers()), 6)
            self.assertTrue(all(member.name.startswith('cache/download/') for member in archive.getmembers()))
            self.assertIn('cache/download/example.test/!extra/@v/v1.0.0.mod', archive.getnames())
            self.assertNotIn('cache/download/example.test/!extra/@v/v2.0.0.mod', archive.getnames())

    def test_rejects_download_missing_from_captured_go_sum(self):
        self.sums.write_text('example.test/dependency v1.0.0 h1:wrong\n'
                             'example.test/dependency v1.0.0/go.mod ' + self.mod_sum + '\n', encoding='utf-8')
        with self.assertRaisesRegex(ValueError, 'not bound to go.sum'):
            self.acquire()

    def test_rejects_failed_module_verification(self):
        self.verify_status = 1
        with self.assertRaisesRegex(ValueError, 'verification failed'):
            self.acquire()

    def test_rejects_failed_post_archive_verification(self):
        self.fail_second_verify = True
        with self.assertRaisesRegex(ValueError, 'verification failed'):
            self.acquire()
        self.assertEqual(self.verify_calls, 2)

    def test_rejects_transient_cache_change_during_archival(self):
        original_addfile = tarfile.TarFile.addfile
        def mutate_then_restore(archive, info, stream=None):
            path = Path(stream.name)
            if path.name != 'v1.0.0.mod':
                return original_addfile(archive, info, stream)
            original = path.read_bytes()
            path.write_bytes(b'tampered')
            try:
                return original_addfile(archive, info, stream)
            finally:
                path.write_bytes(original)
        with patch.object(tarfile.TarFile, 'addfile', mutate_then_restore):
            with self.assertRaisesRegex(ValueError, 'changed|truncated'):
                self.acquire()

    def test_rejects_workspace_input(self):
        (self.copy / 'go.work').write_text('go 1.26\n', encoding='utf-8')
        with self.assertRaisesRegex(ValueError, 'workspace'):
            self.acquire()

    def test_rejects_local_replacement_outside_snapshot(self):
        item = {'ImportPath': 'example.test/foreign',
                'Module': {'Path': 'example.test/foreign', 'Version': 'v1.0.0',
                           'Replace': {'Path': '../foreign', 'Dir': str(self.base / 'foreign')}}}
        with self.assertRaisesRegex(ValueError, 'outside captured source'):
            dependencies._required_modules((json.dumps(item) + '\n').encode(), self.copy, set())


if __name__ == '__main__':
    unittest.main()
