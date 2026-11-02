"""Source transfer into an owned Go runner must fail closed on drift."""
import hashlib
import io
import json
from pathlib import Path
import tempfile
import tarfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import local_go_execution as runner


class FakeOwner:
    def __init__(self):
        self.state = {'resources': [{'kind': 'container', 'name': 'owned', 'state': 'ready',
                                     'mounts': {'/workspace': 'owned-volume'}}]}
        self.commands = []
        self.payload = b''
        self.bad_hash = False
        self.mutate_archive = False

    def execute(self, name, command, stdout, stderr, timeout, stdin=None):
        self.commands.append(command)
        if stdin is not None:
            self.payload = stdin.read()
            if self.mutate_archive:
                Path(stdin.name).write_bytes(b'changed archive')
        if command[0] == 'sha256sum':
            digest = '0' * 64 if self.bad_hash else hashlib.sha256(self.payload).hexdigest()
            stdout.write((digest + '  /workspace/candidate.tar\n').encode())
        return 0


class TransferTests(unittest.TestCase):
    def test_dependency_transfer_checks_versions_and_archive_before_extraction(self):
        source = {'head': 'a' * 40}
        class DependencyOwner:
            def __init__(self):
                self.commands, self.payload = [], b''
            def execute(self, name, command, stdout, stderr, timeout, stdin=None):
                self.commands.append(command)
                if command == ['go', 'version']:
                    stdout.write(b'go version go1.26.10 linux/amd64\n')
                if stdin is not None:
                    self.payload = stdin.read()
                if command[0] == 'sha256sum':
                    stdout.write((hashlib.sha256(self.payload).hexdigest() + '  /workspace/dependencies.tar\n').encode())
                return 0
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            archive = Path(directory) / 'cache.tar'
            with tarfile.open(archive, 'w') as tar:
                info = tarfile.TarInfo('cache/download/example.test/pkg/@v/v1.0.0.mod')
                info.size = 3
                tar.addfile(info, io.BytesIO(b'mod'))
            acquisition = {'status': 'incomplete', 'source': source, 'module_dir': 'src/backend',
                           'target': {'goos': 'linux', 'goarch': 'amd64', 'cgo_enabled': False},
                           'toolchain_version': 'go version go1.26.10 windows/amd64',
                           'archive_sha256': hashlib.sha256(archive.read_bytes()).hexdigest()}
            owner = DependencyOwner()
            with patch.object(runner.snapshot, 'capture', return_value=source), \
                 patch.object(runner.snapshot, 'verify_materialized'), \
                 patch.object(runner, '_verify_workspace'), patch.object(runner, '_verify_modules'):
                result = runner.transfer_dependencies(owner, 'owned', root, source, root / 'copy',
                                                       'b' * 64, acquisition, archive)
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(owner.payload, archive.read_bytes())
            self.assertLess(next(i for i, cmd in enumerate(owner.commands) if cmd[0] == 'sha256sum'),
                            next(i for i, cmd in enumerate(owner.commands) if cmd[:2] == ['tar', '-xf']))
            acquisition['toolchain_version'] = 'go version go1.26.2 windows/amd64'
            owner = DependencyOwner()
            with patch.object(runner.snapshot, 'capture', return_value=source), \
                 patch.object(runner.snapshot, 'verify_materialized'), patch.object(runner, '_verify_workspace'):
                with self.assertRaisesRegex(ValueError, 'toolchains differ'):
                    runner.transfer_dependencies(owner, 'owned', root, source, root / 'copy',
                                                 'b' * 64, acquisition, archive)
            self.assertFalse(any(cmd[0] == 'tar' for cmd in owner.commands))
            with tarfile.open(archive, 'w') as tar:
                info = tarfile.TarInfo('cache/download/../../escape')
                info.size = 3
                tar.addfile(info, io.BytesIO(b'bad'))
            acquisition['archive_sha256'] = hashlib.sha256(archive.read_bytes()).hexdigest()
            acquisition['toolchain_version'] = 'go version go1.26.10 windows/amd64'
            owner = DependencyOwner()
            with patch.object(runner.snapshot, 'capture', return_value=source), \
                 patch.object(runner.snapshot, 'verify_materialized'), patch.object(runner, '_verify_workspace'):
                with self.assertRaisesRegex(ValueError, 'archive entry'):
                    runner.transfer_dependencies(owner, 'owned', root, source, root / 'copy',
                                                 'b' * 64, acquisition, archive)
            self.assertFalse(any(cmd[0] == 'tar' for cmd in owner.commands))

    def test_go_discovery_covers_packages_with_and_without_tests(self):
        source = {'head': 'a' * 40}
        module = {'Dir': '/workspace/source/src/backend', 'Path': 'example.test/module', 'Main': True}
        items = [{'Dir': '/workspace/source/src/backend', 'ImportPath': 'example.test/module',
                  'Module': module, 'TestGoFiles': ['hello_test.go']},
                 {'Dir': '/workspace/source/src/backend/empty', 'ImportPath': 'example.test/module/empty',
                  'Module': module}]
        class ListOwner:
            def execute(self, name, command, stdout, stderr, timeout):
                stdout.write(('\n'.join(json.dumps(item) for item in items) + '\n').encode())
                return 0
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            with patch.object(runner.snapshot, 'capture', return_value=source), \
                 patch.object(runner.snapshot, 'verify_materialized'), \
                 patch.object(runner, '_verify_workspace'), patch.object(runner, '_verify_modules'):
                report = runner.discover_packages(ListOwner(), 'owned', root, source, root / 'copy',
                    'a' * 64, 'src/backend', Path(directory) / 'output')
            self.assertEqual(report['packages_with_tests'], ['example.test/module'])
            self.assertEqual(report['packages_without_tests'], ['example.test/module/empty'])
            self.assertEqual(report['status'], 'incomplete')

    def test_go_discovery_rejects_foreign_module(self):
        class ListOwner:
            def execute(self, name, command, stdout, stderr, timeout):
                stdout.write(json.dumps({'Dir': '/workspace/source/other', 'ImportPath': 'example.test/other',
                    'Module': {'Dir': '/workspace/source/other', 'Path': 'example.test/other', 'Main': True}}).encode())
                return 0
        source = {'head': 'a' * 40}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            with patch.object(runner.snapshot, 'capture', return_value=source), \
                 patch.object(runner.snapshot, 'verify_materialized'), patch.object(runner, '_verify_workspace'), \
                 patch.object(runner, '_verify_modules'):
                with self.assertRaisesRegex(ValueError, 'outside'):
                    runner.discover_packages(ListOwner(), 'owned', root, source, root / 'copy',
                        'a' * 64, 'src/backend', Path(directory) / 'output')

    def test_go_diagnostic_records_skips_and_offline_command_without_pass(self):
        source = {'head': 'a' * 40}
        events = [{'Action': 'start', 'Package': 'example.test/module'},
                  {'Action': 'run', 'Package': 'example.test/module', 'Test': 'TestPass'},
                  {'Action': 'pass', 'Package': 'example.test/module', 'Test': 'TestPass'},
                  {'Action': 'run', 'Package': 'example.test/module', 'Test': 'TestSkip'},
                  {'Action': 'skip', 'Package': 'example.test/module', 'Test': 'TestSkip'},
                  {'Action': 'pass', 'Package': 'example.test/module'}]
        class GoOwner:
            def execute(self, name, command, stdout, stderr, timeout):
                self.command = command
                stdout.write(('\n'.join(json.dumps(event) for event in events) + '\n').encode())
                return 0
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            owner = GoOwner()
            with patch.object(runner.snapshot, 'capture', return_value=source), \
                 patch.object(runner.snapshot, 'verify_materialized'), \
                 patch.object(runner, '_verify_workspace') as verify_workspace, \
                 patch.object(runner, '_verify_modules') as verify_modules:
                report = runner.run_package_diagnostic(owner, 'owned', root, source, root / 'copy',
                    'a' * 64, 'src/backend', ['example.test/module'], Path(directory) / 'output')
            self.assertEqual(report['status'], 'incomplete')
            self.assertEqual(report['tests']['skipped'], [['example.test/module', 'TestSkip']])
            self.assertIn('GOPROXY=off', owner.command[2])
            self.assertIn('GOMAXPROCS=2', owner.command[2])
            self.assertIn('go test -short -json -count=1 -p 2', owner.command[2])
            self.assertIn('example.test/module', owner.command)
            self.assertEqual(verify_workspace.call_count, 2)
            self.assertEqual(verify_modules.call_count, 2)

    def test_go_diagnostic_rejects_changed_source_and_invalid_package(self):
        source = {'head': 'a' * 40}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            owner = FakeOwner()
            with patch.object(runner.snapshot, 'capture', return_value={'head': 'b' * 40}), \
                 patch.object(runner.snapshot, 'verify_materialized'):
                with self.assertRaisesRegex(ValueError, 'source changed'):
                    runner.run_package_diagnostic(owner, 'owned', root, source, root / 'copy',
                        'a' * 64, 'src/backend', ['example.test/module'], Path(directory) / 'output')
            with self.assertRaisesRegex(ValueError, 'package inventory'):
                runner.run_package_diagnostic(owner, 'owned', root, source, root / 'copy',
                    'a' * 64, 'src/backend', ['example.test/../escape'], Path(directory) / 'output')

    def test_workspace_rejects_extra_file(self):
        class WorkspaceOwner:
            def execute(self, name, command, stdout, stderr, timeout):
                if command[0] == 'sha256sum':
                    stdout.write((('a' * 64) + '  /workspace/candidate.tar\n').encode())
                elif command[0] == 'sh' and 'sort -z' in command[2]:
                    stdout.write(b'./known.go\0./unexpected.go\0')
                return 0
        with patch.object(runner.snapshot, '_snapshot_entries', return_value=[(b'known.go', ['known.go'])]):
            with self.assertRaisesRegex(ValueError, 'workspace changed'):
                runner._verify_workspace(WorkspaceOwner(), 'owned', Path('.'), {}, 'a' * 64)

    def test_go_diagnostic_rejects_source_change_during_command(self):
        source = {'head': 'a' * 40}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            with patch.object(runner.snapshot, 'capture', side_effect=[source, {'head': 'b' * 40}]), \
                 patch.object(runner.snapshot, 'verify_materialized'), \
                 patch.object(runner, '_verify_workspace'), patch.object(runner, '_verify_modules'):
                with self.assertRaisesRegex(ValueError, 'source changed during'):
                    runner.run_package_diagnostic(FakeOwner(), 'owned', root, source, root / 'copy',
                        'a' * 64, 'src/backend', ['example.test/module'], Path(directory) / 'output')

    def test_module_verifier_rejects_failure_and_unexpected_output(self):
        class VerifyOwner:
            def __init__(self, status, output):
                self.status, self.output = status, output
            def execute(self, name, command, stdout, stderr, timeout):
                stdout.write(self.output)
                return self.status
        runner._verify_modules(VerifyOwner(0, b'all modules verified\n'), 'owned', 'module')
        with self.assertRaisesRegex(ValueError, 'verification failed'):
            runner._verify_modules(VerifyOwner(1, b''), 'owned', 'module')
        with self.assertRaisesRegex(ValueError, 'verification failed'):
            runner._verify_modules(VerifyOwner(0, b''), 'owned', 'module')

    def test_replaced_archive_path_prevents_remote_extraction(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / 'candidate.tar'
            owner = FakeOwner()
            def create_archive(*args):
                archive.write_bytes(b'exact archive')
                return hashlib.sha256(b'exact archive').hexdigest()
            original_lstat = Path.lstat
            calls = 0
            def changed_lstat(path):
                nonlocal calls
                info = original_lstat(path)
                calls += 1
                if calls == 2:
                    return SimpleNamespace(st_mode=info.st_mode, st_dev=info.st_dev, st_ino=info.st_ino + 1)
                return info
            with patch.object(runner.snapshot, 'archive', side_effect=create_archive), \
                 patch.object(Path, 'lstat', changed_lstat):
                with self.assertRaisesRegex(ValueError, 'archive path changed'):
                    runner.transfer_source(owner, 'owned', root, {'head': 'a' * 40}, root / 'snapshot', archive)
            self.assertFalse(any(cmd[0] == 'tar' for cmd in owner.commands))

    def test_local_archive_mutation_prevents_remote_extraction(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / 'candidate.tar'
            owner = FakeOwner()
            owner.mutate_archive = True
            def create_archive(*args):
                archive.write_bytes(b'exact archive')
                return hashlib.sha256(b'exact archive').hexdigest()
            with patch.object(runner.snapshot, 'archive', side_effect=create_archive):
                with self.assertRaisesRegex(ValueError, 'local archive changed'):
                    runner.transfer_source(owner, 'owned', root, {'head': 'a' * 40}, root / 'snapshot', archive)
            self.assertFalse(any(cmd[0] == 'tar' for cmd in owner.commands))

    def test_remote_hash_is_checked_before_extract_and_local_source_rechecked(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / 'candidate.tar'
            owner = FakeOwner()
            source = {'head': 'a' * 40}
            def create_archive(*args):
                archive.write_bytes(b'exact archive')
                return hashlib.sha256(b'exact archive').hexdigest()
            with patch.object(runner.snapshot, 'archive', side_effect=create_archive), \
                 patch.object(runner.snapshot, 'verify_materialized') as verify, \
                 patch.object(runner.snapshot, 'capture', return_value=source):
                result = runner.transfer_source(owner, 'owned', root, source, root / 'snapshot', archive)
            self.assertEqual(result, hashlib.sha256(b'exact archive').hexdigest())
            self.assertEqual(owner.payload, b'exact archive')
            self.assertLess(next(i for i, cmd in enumerate(owner.commands) if cmd[0] == 'sha256sum'),
                            next(i for i, cmd in enumerate(owner.commands) if cmd[0] == 'tar'))
            verify.assert_called_once()

    def test_wrong_remote_hash_never_extracts(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / 'candidate.tar'
            owner = FakeOwner()
            owner.bad_hash = True
            source = {'head': 'a' * 40}
            def create_archive(*args):
                archive.write_bytes(b'exact archive')
                return hashlib.sha256(b'exact archive').hexdigest()
            with patch.object(runner.snapshot, 'archive', side_effect=create_archive):
                with self.assertRaisesRegex(ValueError, 'container archive'):
                    runner.transfer_source(owner, 'owned', root, source, root / 'snapshot', archive)
            self.assertFalse(any(cmd[0] == 'tar' for cmd in owner.commands))


if __name__ == '__main__':
    unittest.main()
