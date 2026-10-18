"""Negative and positive checks for the immutable helper image source closure."""
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

import verify_phase_image_manifest as verifier


class PhaseImageManifestTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.dockerfile = self.root / verifier.DOCKERFILE
        self.dockerfile.parent.mkdir(parents=True)
        self.dockerfile.write_text(
            'FROM golang@sha256:' + '1' * 64 + ' AS build\n'
            'COPY scripts/staging/nats-known-baseline/go.mod scripts/staging/nats-known-baseline/go.sum ./\n'
            'COPY scripts/staging/nats-known-baseline/*.go ./\n'
            'FROM nats@sha256:' + '2' * 64 + '\n'
            'COPY --from=build /out/kernel /voice-nats-runner/kernel\n'
            'COPY scripts/staging/nats-rollout-preservation/nats-phase /voice-nats-runner/phase\n'
            'COPY scripts/staging/nats-rollout-preservation/native_store.py /voice-nats-runner/native_store.py\n'
            'COPY scripts/staging/nats-known-baseline/controller.py /voice-nats-runner/controller.py\n', encoding='utf-8')
        files = {
            'scripts/staging/nats-known-baseline/go.mod': 'module fixture\n',
            'scripts/staging/nats-known-baseline/go.sum': 'sum fixture\n',
            'scripts/staging/nats-known-baseline/main.go': 'package main\n',
            'scripts/staging/nats-rollout-preservation/nats-phase': '#!/usr/bin/python3\n',
            'scripts/staging/nats-rollout-preservation/native_store.py': '# fixed fixture\n',
            'scripts/staging/nats-known-baseline/controller.py': '# fixed fixture\n',
        }
        for name, content in files.items():
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding='utf-8')
        self.manifest = self.root / verifier.MANIFEST
        self.manifest.parent.mkdir(parents=True, exist_ok=True)
        self.write_manifest()

    def tearDown(self):
        self.temp.cleanup()

    def write_manifest(self):
        inputs = {}
        paths = [self.dockerfile, *(self.root / name for name in (
            'scripts/staging/nats-known-baseline/go.mod',
            'scripts/staging/nats-known-baseline/go.sum',
            'scripts/staging/nats-known-baseline/main.go',
            'scripts/staging/nats-rollout-preservation/nats-phase',
            'scripts/staging/nats-rollout-preservation/native_store.py',
            'scripts/staging/nats-known-baseline/controller.py'))]
        for path in paths:
            relative = path.relative_to(self.root).as_posix()
            inputs[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
        value = {'schema': verifier.SCHEMA,
            'image': {'localImageId': 'sha256:' + 'a' * 64, 'registryPullDigest': None,
                'platform': 'linux/amd64', 'user': '10000:10000',
                'entrypoint': ['/usr/bin/python3', '/voice-nats-runner/phase']},
            'baseImages': ['golang@sha256:' + '1' * 64, 'nats@sha256:' + '2' * 64],
            'inputs': inputs, 'qualification': 'private fixture only'}
        self.manifest.write_text(json.dumps(value), encoding='utf-8')

    def test_complete_copy_closure_is_accepted(self):
        receipt = verifier.verify(self.root)
        self.assertEqual(receipt['input_count'], 7)
        self.assertRegex(receipt['source_closure_sha256'], r'^[a-f0-9]{64}$')
        self.assertIsNone(receipt['registry_pull_digest'])

    def test_changed_source_bytes_are_rejected(self):
        source = self.root / 'scripts/staging/nats-known-baseline/main.go'
        source.write_text('package main\n// drift\n', encoding='utf-8')
        with self.assertRaisesRegex(verifier.ManifestError, 'input_hash_mismatch'):
            verifier.verify(self.root)

    def test_new_docker_copy_input_must_be_added_to_manifest(self):
        extra = self.root / 'scripts/staging/nats-known-baseline/new.go'
        extra.parent.mkdir(parents=True, exist_ok=True)
        extra.write_text('package main\n', encoding='utf-8')
        with self.assertRaisesRegex(verifier.ManifestError, 'input_closure_mismatch'):
            verifier.verify(self.root)

    def test_source_closure_can_be_checked_before_a_new_image_is_built(self):
        value = json.loads(self.manifest.read_text(encoding='utf-8'))
        value['image']['localImageId'] = None
        self.manifest.write_text(json.dumps(value), encoding='utf-8')
        receipt = verifier.verify(self.root)
        self.assertIsNone(receipt['image_id'])


if __name__ == '__main__':
    unittest.main()
