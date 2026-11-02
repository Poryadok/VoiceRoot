"""Tests for source-bound private Go module cache acquisition."""
import json
from pathlib import Path
import sys
import tarfile
import tempfile
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
        original_add = tarfile.TarFile.add
        def mutate_then_restore(archive, name, *args, **kwargs):
            path = Path(name)
            if path.name != 'v1.0.0.mod':
                return original_add(archive, name, *args, **kwargs)
            original = path.read_bytes()
            path.write_bytes(b'tampered')
            try:
                return original_add(archive, name, *args, **kwargs)
            finally:
                path.write_bytes(original)
        with patch.object(tarfile.TarFile, 'add', mutate_then_restore):
            with self.assertRaisesRegex(ValueError, 'archive bytes changed'):
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
