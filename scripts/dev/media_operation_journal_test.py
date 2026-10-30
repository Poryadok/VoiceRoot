"""Linux custody and crash boundaries for the media operation journal."""
import json
import os
import subprocess
import sys
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import media_operation_journal as journal


class JournalTest(unittest.TestCase):
    def test_fifo_state_is_rejected_without_blocking(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            with journal.Journal(root, {'operation': 'example'}):
                pass
            (root / 'state.json').unlink()
            os.mkfifo(root / 'state.json', 0o600)
            code = ("import sys; sys.path.insert(0, sys.argv[2]); import media_operation_journal as j\n"
                    "try: j.Journal(sys.argv[1], {'operation':'example'})\n"
                    "except j.RecoveryRequired: sys.exit(0)\n"
                    "sys.exit(1)\n")
            result = subprocess.run([sys.executable, '-c', code, str(root), str(Path(__file__).parent)], timeout=3,
                                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            self.assertEqual(result.returncode, 0)

    def test_exclusive_lock_and_exact_intent(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            with journal.Journal(root, {'operation': 'example'}) as first:
                first.state['steps']['one'] = {'status': 'pending'}
                first.save()
                with self.assertRaises(journal.RecoveryRequired):
                    journal.Journal(root, {'operation': 'example'})
            with journal.Journal(root, {'operation': 'example'}) as resumed:
                self.assertEqual(resumed.state['steps']['one']['status'], 'pending')
            with self.assertRaises(journal.RecoveryRequired):
                journal.Journal(root, {'operation': 'different'})

    def test_failed_save_poison_blocks_same_instance(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            with journal.Journal(root, {'operation': 'example'}) as active:
                active.state['steps']['one'] = {'status': 'done'}
                with mock.patch.object(journal.os, 'fsync', side_effect=OSError('injected')):
                    with self.assertRaises(journal.RecoveryRequired):
                        active.save()
                with self.assertRaises(journal.RecoveryRequired):
                    active.save()

    def test_unsafe_existing_state_and_symlink_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            with journal.Journal(root, {'operation': 'example'}):
                pass
            path = root / 'state.json'
            path.chmod(0o644)
            with self.assertRaises(journal.RecoveryRequired):
                journal.Journal(root, {'operation': 'example'})
            path.chmod(0o600)
            link = Path(tmp) / 'link'
            link.symlink_to(root, target_is_directory=True)
            with self.assertRaises(journal.RecoveryRequired):
                journal.Journal(link, {'operation': 'example'})

    def test_interrupted_temporary_state_is_preserved_and_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            with journal.Journal(root, {'operation': 'example'}):
                pass
            pending = root / '.state-retained'
            pending.write_text('retained')
            pending.chmod(0o600)
            with self.assertRaises(journal.RecoveryRequired):
                journal.Journal(root, {'operation': 'example'})
            self.assertEqual(pending.read_text(), 'retained')

    def test_existing_lock_without_state_is_never_initialized(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            root.mkdir(mode=0o700)
            (root / 'lock').touch(mode=0o600)
            with self.assertRaises(journal.RecoveryRequired):
                journal.Journal(root, {'operation': 'example'})
            self.assertFalse((root / 'state.json').exists())

    def test_cleanup_error_is_sanitized_and_poisons_journal(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            with journal.Journal(root, {'operation': 'example'}) as active:
                with mock.patch.object(journal.os, 'unlink', side_effect=OSError('private path')):
                    with self.assertRaisesRegex(journal.RecoveryRequired, 'cleanup uncertain'):
                        active.save()
                with self.assertRaises(journal.RecoveryRequired):
                    active.save()

    def test_collision_does_not_remove_foreign_temporary_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            with journal.Journal(root, {'operation': 'example'}) as active:
                collision = root / ('.state-' + 'a' * 32)
                collision.write_text('foreign')
                with mock.patch.object(journal.secrets, 'token_hex', return_value='a' * 32):
                    with self.assertRaises(journal.RecoveryRequired):
                        active.save()
                self.assertEqual(collision.read_text(), 'foreign')

    def test_replace_committed_then_error_requires_external_reconciliation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'operation'
            replace = os.replace
            with journal.Journal(root, {'operation': 'example'}) as active:
                active.state['steps']['one'] = {'status': 'done'}
                def committed_then_lost(*args, **kwargs):
                    replace(*args, **kwargs)
                    raise OSError('lost response')
                with mock.patch.object(journal.os, 'replace', side_effect=committed_then_lost):
                    with self.assertRaises(journal.RecoveryRequired):
                        active.save()
                with self.assertRaises(journal.RecoveryRequired):
                    active.save()
            # Local disk may contain the new state despite an error. The caller
            # must consult the permanent fence before taking any further action.
            with journal.Journal(root, {'operation': 'example'}) as reopened:
                self.assertEqual(reopened.state['steps']['one']['status'], 'done')


if __name__ == '__main__':
    unittest.main()
