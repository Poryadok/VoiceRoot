import subprocess
import tempfile
from pathlib import Path
import unittest

import check_go_modules_clean as guard


class GoModuleCleanTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)
        for name in ('pkg', 'chat'):
            directory = self.root / 'src/backend' / name
            directory.mkdir(parents=True)
            (directory / 'go.mod').write_text('module example/' + name + '\n', encoding='utf-8')
        (self.root / '.gitignore').write_text('src/backend/chat/go.sum\n', encoding='utf-8')
        subprocess.run(['git', '-C', str(self.root), 'add', '.'], check=True)
        subprocess.run(['git', '-C', str(self.root), '-c', 'user.name=Fixture',
                        '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'fixture'], check=True)

    def test_clean_committed_modules_pass(self):
        guard.check(self.root, ['chat'])

    def test_tracked_modification_and_ignored_new_sum_fail(self):
        path = self.root / 'src/backend/chat/go.mod'
        path.write_text('module modified\n', encoding='utf-8')
        with self.assertRaises(ValueError):
            guard.check(self.root, ['chat'])
        path.write_text('module example/chat\n', encoding='utf-8')
        (path.parent / 'go.sum').write_text('new checksum\n', encoding='utf-8')
        with self.assertRaises(ValueError):
            guard.check(self.root, ['chat'])

    def test_invalid_module_inventory_fails(self):
        for services in ([], ['../escape'], ['chat', 'chat'], ['pkg']):
            with self.subTest(services=services), self.assertRaises(ValueError):
                guard.check(self.root, services)


if __name__ == '__main__':
    unittest.main()
