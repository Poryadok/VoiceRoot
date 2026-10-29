import json
import os
import subprocess
import sys
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import staging_core_credentials_transfer as transfer


def sample():
    return {'schema': 'voice-staging-test-accounts-v1', 'provenance': 'fixture', 'accounts': [
        {'email': 'owned+2@example.test', 'password': 'fixture-only-2'},
        {'email': 'owned+3@example.test', 'password': 'fixture-only-3'}]}


class TransferTest(unittest.TestCase):
    def test_fixed_ssh_uses_stdin_only_and_exclusive_remote_file(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / 'source.json'
            raw = json.dumps(sample()).encode(); source.write_bytes(raw)
            calls = []
            class Result:
                stdout = b'hostname 192.168.0.109\nuser pmd\nport 22\n'
            def run(argv, **kwargs):
                calls.append((argv, kwargs))
                return Result()
            with patch.object(transfer.sys, 'platform', 'win32'), patch.object(transfer, '_read_authorized_source', return_value=raw), \
                    patch.object(transfer.subprocess, 'run', side_effect=run):
                transfer.transfer()
            self.assertEqual(len(calls), 2)
            argv, options = calls[-1]
            self.assertIn('StrictHostKeyChecking=yes', argv)
            self.assertIn('HostName=192.168.0.109', argv)
            self.assertIn('voice-staging', argv)
            self.assertIn('O_EXCL', argv[-1])
            self.assertEqual(options['input'], raw)
            self.assertNotIn('fixture-only-2', repr(argv))

    def test_bad_host_refused_before_transfer(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / 'source.json'; source.write_text(json.dumps(sample()))
            calls = []
            class Result: stdout = b'hostname foreign.invalid\nuser pmd\nport 22\n'
            def run(argv, **kwargs): calls.append(argv); return Result()
            with patch.object(transfer.sys, 'platform', 'win32'), patch.object(transfer, '_read_authorized_source', return_value=source.read_bytes()), \
                    patch.object(transfer.subprocess, 'run', side_effect=run):
                with self.assertRaises(RuntimeError): transfer.transfer()
            self.assertEqual(len(calls), 1)

    def test_mismatched_source_identity_never_calls_ssh(self):
        with patch.object(transfer.sys, 'platform', 'win32'), \
                patch.object(transfer, '_read_authorized_source', side_effect=ValueError('identity mismatch')), \
                patch.object(transfer.subprocess, 'run') as run:
            with self.assertRaises(RuntimeError): transfer.transfer()
            run.assert_not_called()

    @unittest.skipUnless(os.name == 'posix', 'POSIX remote custody fixture')
    def test_remote_script_creates_private_file_once_without_output(self):
        with tempfile.TemporaryDirectory() as directory:
            raw = json.dumps(sample()).encode()
            script = transfer._REMOTE.replace('/home/pmd', directory)
            first = subprocess.run([sys.executable, '-c', script], input=raw, capture_output=True)
            self.assertEqual(first.returncode, 0)
            self.assertEqual(first.stdout, b'')
            target = Path(directory) / 'voice-staging-core-accounts' / 'accounts.json'
            self.assertEqual(target.stat().st_mode & 0o777, 0o600)
            self.assertEqual(target.read_bytes(), raw)
            second = subprocess.run([sys.executable, '-c', script], input=b'foreign', capture_output=True)
            self.assertNotEqual(second.returncode, 0)
            self.assertEqual(second.stdout, b'')
            self.assertEqual(target.read_bytes(), raw)

    @unittest.skipUnless(os.name == 'nt', 'Windows handle custody fixture')
    def test_locked_source_refuses_path_replacement_during_acl_check(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / 'source.json'; other = Path(directory) / 'other.json'
            original = json.dumps(sample()).encode(); source.write_bytes(original); other.write_bytes(b'foreign')
            attempts = []
            def check_acl(*args, **kwargs):
                if not attempts:
                    with self.assertRaises(PermissionError): os.replace(other, source)
                attempts.append(True)
            with patch.object(transfer, 'SOURCE', str(source)), patch.object(transfer.subprocess, 'run', side_effect=check_acl):
                self.assertEqual(transfer._read_authorized_source(), original)
            self.assertEqual(len(attempts), 2)


if __name__ == '__main__': unittest.main()
