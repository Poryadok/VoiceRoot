"""Component checks; real pinned Docker compilation is a separate acceptance run."""
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import staging_story_target8_helper as helper


class SourceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.inputs = self.work / 'inputs'
        self.inputs.mkdir()
        self.variant = self.work / 'variant.go'
        self.variant.write_bytes(b'variant\n')
        data = io.BytesIO()
        with tarfile.open(fileobj=data, mode='w') as archive:
            member = tarfile.TarInfo(helper.MAIN)
            member.size = len(b'original\n')
            archive.addfile(member, io.BytesIO(b'original\n'))
        self.raw = data.getvalue()
        self.fetches = []

    def archive(self, root, commit, subtree, destination):
        self.fetches.append((root, commit, subtree))
        destination.write_bytes(self.raw)
        return hashlib.sha256(self.raw).hexdigest()

    def prepare(self, **pins):
        defaults = {'ARCHIVE_SHA': hashlib.sha256(self.raw).hexdigest(),
                    'ORIGINAL_SHA': hashlib.sha256(b'original\n').hexdigest(),
                    'VARIANT_SHA': hashlib.sha256(b'variant\n').hexdigest()}
        defaults.update(pins)
        with patch.multiple(helper, **defaults), patch.object(helper.legacy, 'archive', self.archive):
            return helper.prepare_sources(self.work, self.inputs, self.variant)

    def test_variant_mismatch_stops_before_archive(self):
        with self.assertRaisesRegex(ValueError, 'variant'):
            self.prepare(VARIANT_SHA='0' * 64)
        self.assertEqual([], self.fetches)

    def test_archive_mismatch_stops_before_overlay(self):
        with self.assertRaisesRegex(ValueError, 'archive'):
            self.prepare(ARCHIVE_SHA='0' * 64)
        self.assertFalse((self.inputs / 'main.go').exists())

    def test_original_mismatch_stops_before_overlay(self):
        with self.assertRaisesRegex(ValueError, 'original'):
            self.prepare(ORIGINAL_SHA='0' * 64)
        self.assertFalse((self.inputs / 'main.go').exists())

    def test_exact_archive_api_and_only_variant_overlay(self):
        record = self.prepare()
        self.assertEqual([(self.work, helper.SOURCE, 'src/backend')], self.fetches)
        self.assertEqual({'source.tar', 'main.go'}, {p.name for p in self.inputs.iterdir()})
        self.assertEqual(self.raw, (self.inputs / 'source.tar').read_bytes())
        self.assertEqual(b'variant\n', (self.inputs / 'main.go').read_bytes())
        self.assertEqual(helper.SOURCE, record['commit'])

    def test_variant_symlink_rejected_before_archive(self):
        linked = self.work / 'linked.go'
        linked.symlink_to(self.variant)
        with self.assertRaisesRegex(ValueError, 'regular file'):
            helper.prepare_sources(self.work, self.inputs, linked)
        self.assertEqual([], self.fetches)

    def test_source_archive_failure_does_not_overlay(self):
        with patch.object(helper, 'VARIANT_SHA', helper.sha(b'variant\n')):
            with patch.object(helper.legacy, 'archive', side_effect=RuntimeError('fetch failed')):
                with self.assertRaisesRegex(RuntimeError, 'fetch failed'):
                    helper.prepare_sources(self.work, self.inputs, self.variant)
        self.assertFalse((self.inputs / 'main.go').exists())


class ImageAndCleanupTests(unittest.TestCase):
    def image(self):
        return [{'Id': helper.GO_CONFIG, 'Os': 'linux', 'Architecture': 'amd64',
                 'RepoDigests': ['golang@' + helper.GO_IMAGE.split('@')[1]],
                 'Config': {'Env': ['GOLANG_VERSION=1.26.8']}}]

    def test_image_identity_dimensions_fail_closed(self):
        for key, wrong in [('Id', 'sha256:' + '0' * 64), ('Os', 'windows'),
                           ('Architecture', 'arm64'), ('RepoDigests', []),
                           ('Config', {'Env': ['GOLANG_VERSION=1.26.7']})]:
            with self.subTest(key=key):
                image = self.image()
                image[0][key] = wrong
                result = subprocess.CompletedProcess([], 0, json.dumps(image).encode(), b'')
                with patch.object(helper, 'command', return_value=result):
                    with self.assertRaisesRegex(ValueError, 'image identity'):
                        helper.authenticate_image([])

    def test_valid_image_identity(self):
        result = subprocess.CompletedProcess([], 0, json.dumps(self.image()).encode(), b'')
        with patch.object(helper, 'command', return_value=result):
            self.assertEqual(helper.GO_CONFIG, helper.authenticate_image([])['config_id'])

    def cleanup(self, label='owned', name='/exact', remaining=b''):
        calls = []
        identity = {'Id': 'a' * 64, 'Name': name, 'Config': {'Labels': {helper.LABEL: label}}}
        responses = [b'aaaaaaaaaaaa\n', json.dumps([identity]).encode(), b'removed\n', remaining]

        def run(argv, records):
            calls.append(argv)
            return subprocess.CompletedProcess(argv, 0, responses.pop(0), b'')

        return calls, run

    def test_foreign_label_or_name_is_never_removed(self):
        for wrong in [{'label': 'foreign'}, {'name': '/foreign'}]:
            calls, run = self.cleanup(**wrong)
            with patch.object(helper, 'command', run):
                with self.assertRaisesRegex(RuntimeError, 'ownership mismatch'):
                    helper.cleanup_container('exact', 'owned', [])
            self.assertFalse(any('rm' in call for call in calls))

    def test_owned_cleanup_uses_inspected_exact_id_and_proves_absence(self):
        calls, run = self.cleanup()
        with patch.object(helper, 'command', run):
            self.assertTrue(helper.cleanup_container('exact', 'owned', [])['absent'])
        self.assertEqual(['docker', 'container', 'rm', '--force', 'a' * 64], calls[2])

    def test_cleanup_remaining_container_is_failure(self):
        _, run = self.cleanup(remaining=b'aaaaaaaaaaaa\n')
        with patch.object(helper, 'command', run):
            with self.assertRaisesRegex(RuntimeError, 'absence unproven'):
                helper.cleanup_container('exact', 'owned', [])


class PublicationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.built = self.work / 'built'
        self.built.mkdir()
        self.raw = b'\x7fELFsynthetic-publication-test'
        (self.built / helper.NAME).write_bytes(self.raw)
        self.output = self.work / 'result'

    def publish(self):
        with patch.object(helper, 'BINARY_SHA', helper.sha(self.raw)):
            helper.publish(self.built, self.work, self.output, {'test': True})

    def test_complete_host_owned_output_with_private_modes(self):
        self.publish()
        self.assertEqual({helper.NAME, 'provenance.json'}, {p.name for p in self.output.iterdir()})
        self.assertEqual(0o700, self.output.stat().st_mode & 0o777)
        for item in self.output.iterdir():
            self.assertEqual(os.getuid(), item.stat().st_uid)
            self.assertEqual(0o400, item.stat().st_mode & 0o777)

    def test_bad_binary_is_never_published(self):
        with self.assertRaisesRegex(ValueError, 'binary pin'):
            helper.publish(self.built, self.work, self.output, {})
        self.assertFalse(self.output.exists())

    def test_failed_chmod_provenance_or_rename_never_publishes(self):
        for method in ['chmod', 'write_text', 'rename']:
            with self.subTest(method=method):
                stage = self.work / 'publish'
                if stage.exists():
                    for item in stage.iterdir():
                        item.chmod(0o600)
                        item.unlink()
                    stage.rmdir()
                with patch.object(Path, method, side_effect=OSError('injected publication failure')):
                    with self.assertRaisesRegex(OSError, 'publication failure'):
                        self.publish()
                self.assertFalse(self.output.exists())

    def test_existing_output_and_dangling_symlink_stop_before_source_or_docker(self):
        for kind in ['directory', 'symlink']:
            if kind == 'directory':
                self.output.mkdir()
            else:
                self.output.symlink_to(self.work / 'absent')
            with patch.object(helper, 'prepare_sources') as prepare, patch.object(helper, 'command') as command:
                with self.assertRaisesRegex(ValueError, 'output must be new'):
                    helper.build(self.work, self.output)
            prepare.assert_not_called()
            command.assert_not_called()
            if kind == 'directory':
                self.output.rmdir()
            else:
                self.output.unlink()

    def test_failed_or_interrupted_build_always_cleans_and_never_publishes(self):
        for failure in [RuntimeError('build failed'), KeyboardInterrupt()]:
            with self.subTest(failure=type(failure).__name__):
                with patch.object(helper, 'prepare_sources', return_value={}), \
                        patch.object(helper, 'authenticate_image', return_value={}), \
                        patch.object(helper, 'command', side_effect=failure), \
                        patch.object(helper, 'cleanup_container', return_value={'absent': True}) as cleanup:
                    with self.assertRaises(type(failure)):
                        helper.build(self.work, self.output)
                cleanup.assert_called_once()
                self.assertFalse(self.output.exists())
                self.assertFalse(list(self.work.glob('voice-target8-build-*')))

    def test_root_owned_elf_publishes_under_ordinary_uid(self):
        # The system utility supplies a root-owned ELF independent of the
        # hosted runner's user-owned Python toolchain. It is never executed.
        original = Path('/usr/bin/env').resolve()
        uid = os.getuid()
        self.assertNotEqual(0, uid)
        self.assertEqual(0, original.stat().st_uid)
        self.assertEqual(0, original.stat().st_mode & 0o022)
        self.assertEqual(b'\x7fELF', original.read_bytes()[:4])
        before = original.stat()
        (self.built / helper.NAME).unlink()
        (self.built / helper.NAME).symlink_to(original)
        self.raw = original.read_bytes()
        self.publish()
        for item in self.output.iterdir():
            self.assertEqual(uid, item.stat().st_uid)
            self.assertEqual(0o400, item.stat().st_mode & 0o777)
        self.assertEqual(before, original.stat())


if __name__ == '__main__':
    unittest.main()
