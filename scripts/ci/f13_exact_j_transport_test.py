"""Real fixture materialization regression; never a F13 guard/producer RED."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock
import zipfile

import f13_exact_j_historical_red_test as harness


class HistoricalJTransport(unittest.TestCase):
    def test_actual_materializer_installs_pinned_Git(self):
        with tempfile.TemporaryDirectory(prefix='f13-pinned-') as folder:
            root = Path(folder)
            harness.materialize(root)
            binary = root / 'historical-git/bin/git'
            self.assertTrue(binary.is_file(), 'actual materializer did not install pinned historical Git')
            self.assertEqual(hashlib.sha256(binary.read_bytes()).hexdigest(),
                             '2540879925a6881e3877ff7e3330746ba3027b04edf16a3a12dccd1644c4f32d')

    def test_real_loader_argv0_stdout_stderr_and_exit(self):
        self.assertNotEqual(os.getuid(), 0, 'component execution must be unprivileged Linux')
        with tempfile.TemporaryDirectory(prefix='f13-loader-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            env = harness.pinned_git_environment(root)
            result = subprocess.run(['git', '--version'], env=env, capture_output=True)
            self.assertEqual((result.returncode, result.stdout, result.stderr), (0, b'git version 2.39.5\n', b''))
            for name, args in [('git', ['-C', str(root), 'show', 'no-such-object']),
                               ('git-index-pack', ['--no-such-option']), ('git-rev-list', ['--no-such-option'])]:
                entry = base / ('launch' if name == 'git' else 'exec') / name
                actual = subprocess.run([str(entry), *args], env=env, capture_output=True)
                direct = subprocess.run([str(base / 'lib/ld-linux-x86-64.so.2'), '--library-path', str(base / 'lib'),
                                         '--argv0', name, str(base / 'bin/git-elf'), *args], env=env, capture_output=True)
                self.assertEqual((actual.returncode, actual.stdout, actual.stderr),
                                 (direct.returncode, direct.stdout, direct.stderr), name)
                self.assertNotEqual(actual.returncode, 0)

    def test_unknown_git_routes_cannot_fall_back_to_host(self):
        with tempfile.TemporaryDirectory(prefix='f13-fallback-') as folder:
            root = Path(folder) / 'private'
            root.mkdir()
            host = Path(folder) / 'fake-host'
            host.mkdir()
            marker = Path(folder) / 'forbidden-host-executed'
            for name in ('git', 'git-unknown-child', 'git-upload-pack'):
                file = host / name
                file.write_text('#!/bin/sh\ntouch ' + str(marker) + '\nexit 0\n')
                file.chmod(0o755)
            with mock.patch.object(harness, 'UTILITY_DIRS', (str(host), *harness.UTILITY_DIRS)):
                base = harness.materialize_pinned_git(root)
                env = harness.pinned_git_environment(root)
                self.assertFalse(any(p.name == 'git' or p.name.startswith('git-') for p in (base / 'util').iterdir()))
                for args in (['unknown-child'], ['--exec-path=' + str(host), '--version'], ['clone', 'https://invalid.example', 'bad']):
                    result = subprocess.run(['git', *args], env=env, capture_output=True)
                    self.assertEqual(result.returncode, 125)
                    self.assertIn(b'F13_PINNED_GIT_SETUP_FAILURE', result.stderr)
                for change in ({'GIT_EXEC_PATH': str(host)}, {'PATH': env['PATH'] + ':' + str(host)},
                               {'GIT_CONFIG_COUNT': '1', 'GIT_CONFIG_KEY_0': 'alias.version', 'GIT_CONFIG_VALUE_0': '!false'},
                               {'LD_LIBRARY_PATH': str(host)}):
                    result = subprocess.run([str(base / 'launch/git'), '--version'], env=dict(env, **change), capture_output=True)
                    self.assertEqual(result.returncode, 125)
                self.assertFalse(marker.exists())

    def test_same_size_binary_and_all_library_corruption_reject_before_launch(self):
        with tempfile.TemporaryDirectory(prefix='f13-forged-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            for name in harness.pinned_manifest():
                with self.subTest(name=name):
                    path = base / name
                    raw = path.read_bytes()
                    path.chmod(0o755)
                    forged = bytearray(raw)
                    forged[-1] ^= 1
                    path.write_bytes(forged)
                    path.chmod(0o555)
                    with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_bytes'):
                        harness.pinned_git_environment(root)
                    path.chmod(0o755)
                    path.write_bytes(raw)
                    path.chmod(0o555)
            harness.pinned_git_environment(root)

    def test_forged_path_mode_and_inherited_context_reject(self):
        with tempfile.TemporaryDirectory(prefix='f13-path-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            binary = base / 'bin/git'
            binary.chmod(0o755)
            with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_mode'):
                harness.pinned_git_environment(root)
            binary.chmod(0o555)
            (base / 'bin').chmod(0o755)
            binary.rename(base / 'bin/saved-git')
            binary.symlink_to(base / 'bin/saved-git')
            with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_path'):
                harness.pinned_git_environment(root)
            binary.unlink()
            (base / 'bin/saved-git').rename(binary)
            (base / 'bin').chmod(0o555)
            for change in ({'GIT_CONFIG_COUNT': '1'}, {'LD_PRELOAD': '/wrong.so'}, {'GIT_EXEC_PATH': '/host/git'}):
                with mock.patch.dict(os.environ, change):
                    with self.assertRaises(harness.SetupFailure):
                        harness.pinned_git_environment(root)

    def test_archive_manifest_forgery_rejects_without_extraction(self):
        with tempfile.TemporaryDirectory(prefix='f13-archive-') as folder:
            fixture = Path(folder) / 'fixture'
            fixture.mkdir()
            for name in ('git-2.39.5-data.tar.gz', 'git-2.39.5-data-manifest.json'):
                shutil.copyfile(harness.FIXTURE / name, fixture / name)
            for name, reason in [('git-2.39.5-data.tar.gz', 'pinned_Git_archive'),
                                 ('git-2.39.5-data-manifest.json', 'pinned_Git_manifest')]:
                original = (fixture / name).read_bytes()
                forged = bytearray(original)
                forged[-1] ^= 1
                (fixture / name).write_bytes(forged)
                root = Path(folder) / ('private-' + reason)
                root.mkdir()
                with mock.patch.object(harness, 'FIXTURE', fixture):
                    with self.assertRaisesRegex(harness.SetupFailure, reason):
                        harness.materialize_pinned_git(root)
                self.assertFalse((root / 'historical-git').exists())
                (fixture / name).write_bytes(original)

    def test_captured_bundle_clone_child_routing_and_precheckout_rejections(self):
        with tempfile.TemporaryDirectory(prefix='f13-child-') as folder:
            root = Path(folder)
            harness.materialize(root)
            env = harness.pinned_git_environment(root)
            repo = root / 'repo'
            result = subprocess.run(['git', 'clone', '--no-checkout', str(root / 'out/prelaunch/private-target.bundle'), str(repo)],
                                    env=env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            with mock.patch.dict(os.environ, env, clear=True):
                harness.verify_checkout_inputs(repo, harness.TARGET)
                parents = harness.run(['git', '-C', str(repo), 'rev-list', '--parents', '-n', '1', harness.TARGET]).decode().split()
                self.assertEqual(parents, [harness.TARGET, harness.BASE, harness.SOURCE])
                config = repo / '.git/config'
                original = config.read_bytes()
                config.write_bytes(original + b'\n[alias]\nversion = !false\n')
                with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_unsafe_config:alias.version'):
                    harness.verify_checkout_inputs(repo, harness.TARGET)
                config.write_bytes(original)
                hooks = repo / '.git/hooks'
                hooks.mkdir(exist_ok=True)
                (hooks / 'post-checkout').write_text('#!/bin/sh\nexit 0\n')
                with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_hooks_absent'):
                    harness.verify_checkout_inputs(repo, harness.TARGET)
            self.assertFalse((repo / 'scripts').exists(), 'component tests never checkout or run the historical helper')

    def test_capability_diagnostic_preserves_every_field_and_raw_inputs(self):
        captured = {'git_version': 'git version 2.39.5', 'native': {'paths': ['a'], 'admission': False}, 'ledger': {'sha': 'abc'}}
        before = json.dumps(captured, sort_keys=True)
        self.assertEqual(harness.capability_differences(captured, captured), [])
        changed = json.loads(before)
        changed['native']['paths'][0] = 'foreign'
        changed['ledger']['sha'] = 'wrong'
        changed['git_version'] = 'git version 2.55.0'
        self.assertEqual(harness.capability_differences(captured, changed), ['$.git_version', '$.ledger.sha', '$.native.paths[0]'])
        self.assertEqual(json.dumps(captured, sort_keys=True), before)

    def test_actual_archive_restores_original_J_record_path(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            harness.materialize(root)
            captured = root / 'out/actual/observed/fixed3071-J-ledger.json'
            original = root / 'tmp/F13-author-547/fixed3071-J-ledger.json'
            self.assertTrue(original.is_file(), 'original literal J-ledger consumer path was not materialized')
            raw = original.read_bytes()
            self.assertEqual(len(raw), 109549632)
            self.assertEqual(hashlib.sha256(raw).hexdigest(), harness.LEDGER_SHA)
            self.assertEqual(raw, captured.read_bytes())

    def test_record_alias_rejects_wrong_declared_binding(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            for binding in (None, ('0' * 64, 109549632), (harness.LEDGER_SHA, 0)):
                with self.subTest(binding=binding):
                    logical = {} if binding is None else {harness.J_CAPTURE: binding}
                    with self.assertRaisesRegex(harness.SetupFailure, 'J_record_declared_binding'):
                        harness.materialize_J_record_alias(root, logical)
                    self.assertEqual(list(root.iterdir()), [])

    def test_record_alias_rejects_forged_captured_bytes(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            source = root / harness.J_CAPTURE.lstrip('/')
            source.parent.mkdir(parents=True)
            with zipfile.ZipFile(harness.FIXTURE / 'historical-inputs.zip') as archive:
                forged = bytearray(archive.read('objects/' + harness.LEDGER_SHA))
            self.assertEqual(len(forged), harness.J_RECORD_BYTES)
            forged[-1] ^= 1  # Same-length actual record mutation must hit SHA, not size.
            self.assertNotEqual(hashlib.sha256(forged).hexdigest(), harness.LEDGER_SHA)
            source.write_bytes(forged)
            with self.assertRaisesRegex(harness.SetupFailure, 'J_record_raw_binding'):
                harness.materialize_J_record_alias(root, {harness.J_CAPTURE: (harness.LEDGER_SHA, 109549632)})
            self.assertEqual(source.read_bytes(), forged)
            self.assertFalse((root / harness.J_ORIGINAL_RECORD.lstrip('/')).exists())

    def test_record_alias_never_overwrites_existing_target(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            source, target = root / harness.J_CAPTURE.lstrip('/'), root / harness.J_ORIGINAL_RECORD.lstrip('/')
            source.parent.mkdir(parents=True)
            source.write_bytes(b'rejection-only source control')
            target.parent.mkdir(parents=True)
            target.write_bytes(b'prior target must stay byte-exact')
            with self.assertRaisesRegex(harness.SetupFailure, 'J_record_no_overwrite'):
                harness.materialize_J_record_alias(root, {harness.J_CAPTURE: (harness.LEDGER_SHA, 109549632)})
            self.assertEqual(target.read_bytes(), b'prior target must stay byte-exact')


if __name__ == '__main__':
    unittest.main()
