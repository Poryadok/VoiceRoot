"""Fault-inject the diagnostic shell with fake commands, never PostgreSQL."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("space_profile50_diagnostic.sh").resolve()

PSQL = r'''#!/usr/bin/python3
import os, pathlib, sys
p = pathlib.Path(os.environ['MOCK_STATE'])
case = os.environ['MOCK_CASE']
args = ' '.join(sys.argv)
after = (p / 'after').exists()
if 'coalesce(sum(jit_functions),0)::bigint' in args:
    count = p / 'jit_count'
    n = int(count.read_text()) + 1 if count.exists() else 0
    count.write_text(str(n))
    print(0 if case == 'jit_canary_zero' else n)
elif 'pg_control_system' in args and 'current_database' not in args:
    print('222' if after and case == 'wrong_cluster' else '111')
elif 'COALESCE' in args:
    if after and case == 'post_drop_failure' and (p / 'dropped').exists(): sys.exit(1)
    print('99' if after and case == 'wrong_oid' else ('42' if (p / 'created').exists() and not (p / 'dropped').exists() else '0'))
elif 'CREATE DATABASE' in args:
    (p / 'created').touch()
elif 'DROP DATABASE' in args:
    (p / 'drop_called').touch()
    if case == 'drop_fail': sys.exit(1)
    (p / 'dropped').touch()
elif 'current_database' in args:
    print('111:42')
elif 'server_version_num' in args:
    print('postgres_server_version_num=160000')
'''

MIGRATE = r'''#!/usr/bin/python3
import os, pathlib, sys
p = pathlib.Path(os.environ['MOCK_STATE'])
(p / 'after').touch()
if os.environ['MOCK_CASE'] in ('normal', 'wrong_cluster', 'wrong_oid', 'drop_fail', 'post_drop_failure'):
    sys.exit(1)
'''

GO = r'''#!/usr/bin/python3
import os, pathlib, sys
if '-o' in sys.argv:
    pathlib.Path(sys.argv[sys.argv.index('-o')+1]).write_bytes(b'diagnostic binary')
if 'test2json' in sys.argv:
    (pathlib.Path(os.environ['MOCK_STATE']) / 'test_executed').touch()
    if os.environ['MOCK_CASE'] in ('test_failure', 'test_failure_source_drift'):
        sys.exit(7)
'''

GIT = r'''#!/usr/bin/python3
import os, pathlib, sys
if 'rev-parse' in sys.argv:
    (pathlib.Path(os.environ['MOCK_STATE']) / 'source_checked').touch()
    if os.environ['MOCK_CASE'] == 'source_unavailable': sys.exit(1)
    print('other' if os.environ['MOCK_CASE'] in ('source_drift', 'test_failure_source_drift') else 'candidate')
elif 'status' in sys.argv:
    if os.environ['MOCK_CASE'] == 'tracked_unavailable' and '--untracked-files=no' in sys.argv: sys.exit(1)
    if os.environ['MOCK_CASE'] == 'tracked_drift': print(' M tracked.go')
    if os.environ['MOCK_CASE'] == 'untracked_drift' and '--untracked-files=no' not in sys.argv: print('?? generated.pyc')
elif 'ls-files' in sys.argv:
    if os.environ['MOCK_CASE'] == 'untracked_unavailable': sys.exit(1)
    if os.environ['MOCK_CASE'] == 'untracked_drift': print('generated.pyc')
'''

SHA = r'''#!/usr/bin/python3
import os, pathlib, sys
p = pathlib.Path(os.environ['MOCK_STATE'])
name = 'binary' if sys.argv[-1].endswith('space-profile50.test') else 'driver'
counter = p / ('sha_' + name)
n = int(counter.read_text()) + 1 if counter.exists() else 1
counter.write_text(str(n))
if os.environ['MOCK_CASE'] == name + '_unavailable' and n >= (3 if name == 'binary' else 2): sys.exit(1)
drift = os.environ['MOCK_CASE'] == name + '_drift' and n >= (3 if name == 'binary' else 2)
print(('b' if drift else 'a') * 64 + '  ' + sys.argv[-1])
'''


class Lifecycle(unittest.TestCase):
    def test_receipt_import_bytecode_would_dirty_checkout_without_B(self):
        for disable_bytecode in (False, True):
            with self.subTest(disable_bytecode=disable_bytecode):
                root = Path(tempfile.mkdtemp(prefix='voice-m50-bytecode-test-'))
                scripts = root / 'scripts' / 'ci'
                scripts.mkdir(parents=True)
                for source in SCRIPT.parent.glob('*.py'):
                    shutil.copyfile(source, scripts / source.name)
                env = dict(os.environ)
                env.pop('PYTHONDONTWRITEBYTECODE', None)
                env.pop('PYTHONPYCACHEPREFIX', None)
                env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull)
                subprocess.run(['git', 'init', '-q', str(root)], env=env, check=True, capture_output=True)
                subprocess.run(['git', '-C', str(root), 'add', 'scripts'], env=env, check=True, capture_output=True)
                cmd = [sys.executable] + (['-B'] if disable_bytecode else []) + [str(scripts / 'go_integration_matrix.py'), '--help']
                subprocess.run(cmd, env=env, check=True, capture_output=True, timeout=30)
                files = subprocess.run(['git', '-C', str(root), 'ls-files', '--others', '--exclude-standard'],
                                       env=env, check=True, capture_output=True, text=True).stdout.splitlines()
                if disable_bytecode:
                    self.assertEqual(files, [])
                else:
                    self.assertTrue(any('__pycache__/' in name and name.endswith('.pyc') for name in files), files)

    def run_case(self, case):
        # The test leaves its own isolated temp directory for runner cleanup.
        root = Path(tempfile.mkdtemp(prefix='voice-m50-profile-test-'))
        bin_dir = root / 'bin'
        bin_dir.mkdir()
        for name, body in {'psql': PSQL, 'go': GO, 'git': GIT, 'sha256sum': SHA,
                           'python3': '#!/bin/sh\nif [ "$1" = -B ]; then shift; else touch "$MOCK_STATE/python_without_B"; fi\ncase "$1" in *space_profile50_validate.py) touch "$MOCK_STATE/validator_called";; esac\nexit 0\n'}.items():
            path = bin_dir / name
            path.write_text(body, encoding='utf-8')
            path.chmod(0o755)
        driver = root / 'native-migrate-driver'
        driver.write_text(MIGRATE, encoding='utf-8')
        driver.chmod(0o755)
        env = dict(os.environ, PATH=str(bin_dir) + os.pathsep + os.environ['PATH'],
                   RUNNER_TEMP=str(root), MOCK_STATE=str(root), MOCK_CASE=case,
                   SPACE_NATIVE_ADMIN_URL='fake-admin', SPACE_BAN_TEST_DATABASE_URL='fake-target',
                   CANDIDATE_SHA='candidate', GITHUB_RUN_ID='1', GITHUB_RUN_ATTEMPT='1')
        result = subprocess.run(['bash', str(SCRIPT)], cwd=SCRIPT.parents[2] / 'src/backend/space',
                                env=env, capture_output=True, timeout=30, check=False)
        recovery = (root / 'space-profile50-cleanup.txt').read_text(encoding='utf-8')
        return result, recovery, root

    def test_cleanup_refuses_unowned_or_unverified_targets(self):
        for case, reason in [('wrong_cluster', 'cluster_identity_unverified'), ('wrong_oid', 'identity_unverified'),
                             ('drop_fail', 'drop_failed'), ('post_drop_failure', 'post_drop_unverified')]:
            with self.subTest(case=case):
                result, recovery, root = self.run_case(case)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('state=uncertain', recovery)
                self.assertIn('reason=' + reason, recovery)
                self.assertEqual((root / 'drop_called').exists(), case in ('drop_fail', 'post_drop_failure'))

    def test_owned_cleanup_and_post_use_identity_drift(self):
        for case in ('normal', 'source_drift', 'driver_drift', 'binary_drift'):
            with self.subTest(case=case):
                result, recovery, root = self.run_case(case)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('state=removed', recovery)
                self.assertTrue((root / 'drop_called').exists())
                if case != 'normal':
                    self.assertTrue((root / 'test_executed').exists())
                    self.assertTrue((root / 'source_checked').exists())
                if case == 'driver_drift':
                    self.assertEqual((root / 'sha_driver').read_text(), '2')
                if case == 'binary_drift':
                    self.assertEqual((root / 'sha_binary').read_text(), '3')

    def test_complete_fake_run_reaches_post_use_verification(self):
        result, recovery, root = self.run_case('complete')
        self.assertEqual(result.returncode, 0, result.stderr.decode(errors='replace'))
        self.assertIn('state=removed', recovery)
        self.assertTrue((root / 'test_executed').exists())
        self.assertTrue((root / 'source_checked').exists())
        self.assertEqual((root / 'sha_driver').read_text(), '2')
        self.assertEqual((root / 'sha_binary').read_text(), '3')
        self.assertIn('post_run_source_and_binaries=verified', (root / 'space-profile50-identities.txt').read_text())
        self.assertFalse((root / 'python_without_B').exists())

    def test_jit_canary_requires_a_counter_increase(self):
        result, recovery, root = self.run_case('jit_canary_zero')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('state=removed', recovery)
        self.assertFalse((root / 'test_executed').exists())

    def test_failed_profile_still_records_every_post_use_check(self):
        for case, source in [('test_failure', 'verified'), ('test_failure_source_drift', 'mismatch')]:
            with self.subTest(case=case):
                result, recovery, root = self.run_case(case)
                self.assertEqual(result.returncode, 7)
                self.assertIn('state=removed', recovery)
                identities = (root / 'space-profile50-identities.txt').read_text()
                self.assertIn('profile_exit_code=7', identities)
                self.assertIn('post_run_source=' + source, identities)
                self.assertIn('post_run_driver=verified', identities)
                self.assertIn('post_run_binary=verified', identities)
                self.assertTrue((root / 'validator_called').exists())
                self.assertEqual((root / 'sha_driver').read_text(), '2')
                self.assertEqual((root / 'sha_binary').read_text(), '3')

    def test_source_mismatch_reports_component_without_weakening_gate(self):
        for case, field in [('source_drift', 'head'), ('tracked_drift', 'tracked'), ('untracked_drift', 'untracked')]:
            with self.subTest(case=case):
                result, recovery, root = self.run_case(case)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('state=removed', recovery)
                identities = (root / 'space-profile50-identities.txt').read_text()
                self.assertIn('post_run_' + field + '=mismatch', identities)
                self.assertIn('post_run_source=mismatch', identities)
                self.assertNotIn('post_run_source_and_binaries=verified', identities)

    def test_post_use_command_failure_is_not_verified_and_checks_continue(self):
        for case, field in [('source_unavailable', 'source'), ('tracked_unavailable', 'tracked'), ('untracked_unavailable', 'untracked'), ('driver_unavailable', 'driver'), ('binary_unavailable', 'binary')]:
            with self.subTest(case=case):
                result, recovery, root = self.run_case(case)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('state=removed', recovery)
                identities = (root / 'space-profile50-identities.txt').read_text()
                self.assertIn('post_run_' + field + '=unavailable', identities)
                self.assertNotIn('post_run_source_and_binaries=verified', identities)
                self.assertTrue((root / 'validator_called').exists())
                self.assertEqual((root / 'sha_driver').read_text(), '2')
                self.assertEqual((root / 'sha_binary').read_text(), '3')


if __name__ == '__main__':
    unittest.main()
