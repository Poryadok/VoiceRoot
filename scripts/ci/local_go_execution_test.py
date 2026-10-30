"""Source transfer into an owned Go runner must fail closed on drift."""
import hashlib
from pathlib import Path
import tempfile
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
