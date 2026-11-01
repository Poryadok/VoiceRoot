"""Two captured module caches merge only with exact matching entries."""
import hashlib
import io
from pathlib import Path
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


if __name__ == '__main__':
    unittest.main()
