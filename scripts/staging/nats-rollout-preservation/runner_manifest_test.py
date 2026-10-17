"""Independent trusted package closure checks; candidate trees are never imported."""
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest

import runner_manifest as manifest


class RunnerManifestTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.base = Path(self.temp.name)
        self.source_sha = 'a' * 40
        self.root = self.base / self.source_sha
        self.root.mkdir()
        self.files = {
            'runner_service.py': b'fixed runner entrypoint\n',
            'runner_rpc.py': b'fixed protocol module\n',
            'bridge_state.py': b'fixed status state reader\n',
            'tokenless_job.py': b'fixed Job producer\n',
            'kube_json_api.py': b'fixed JSON adapter\n',
            'runner_manifest.py': b'fixed verifier\n',
            'protocol-schema.json': b'{"schema":"voice-staging-bridge-runner-request-v1"}\n',
            'runner.service': b'[Service]\nUser=voice-nats-runner\n',
            'runner.socket': b'[Socket]\nSocketMode=0660\n',
            'nats-phase': b'fixed trusted helper ELF\n',
        }
        for name, raw in self.files.items():
            path = self.root / name; path.write_bytes(raw)
            path.chmod(0o555 if name in ('runner_service.py', 'nats-phase') else 0o444)
        self.root.chmod(0o555)
        self.row = {'schema': manifest.SCHEMA, 'runner_source_sha': self.source_sha,
            'protocol_schema': manifest.PROTOCOL_SCHEMA,
            'files': {name: hashlib.sha256(raw).hexdigest() for name, raw in self.files.items()},
            'images': {'nats_phase': 'ghcr.io/poryadok/voice-nats-rollout-runner@sha256:' + 'b' * 64}}
        self.row['closure_sha256'] = manifest.closure_digest(self.row)

    def tearDown(self):
        self.root.chmod(0o755)
        self.temp.cleanup()

    def test_verifies_installed_package_hashes_without_loading_candidate_code(self):
        candidate = self.base / 'candidate'; candidate.mkdir()
        sentinel = self.base / 'candidate-imported'
        (candidate / 'runner_rpc.py').write_text("open(%r,'w').write('executed')\n" % str(sentinel))
        self.assertEqual(manifest.verify(self.root, self.row, owner_uid=os.geteuid())['runner_source_sha'], self.source_sha)
        self.assertFalse(sentinel.exists())
        candidate.joinpath('runner_rpc.py').write_text('app release code changed\n')
        self.assertEqual(manifest.verify(self.root, self.row, owner_uid=os.geteuid())['runner_source_sha'], self.source_sha)

    def test_rejects_missing_or_changed_trusted_closure_member(self):
        changed = self.root / 'tokenless_job.py'
        changed.chmod(0o644); changed.write_text('candidate data'); changed.chmod(0o444)
        with self.assertRaisesRegex(manifest.ManifestError, 'file_hash_mismatch'):
            manifest.verify(self.root, self.row, owner_uid=os.geteuid())
        changed.chmod(0o644); changed.write_bytes(self.files['tokenless_job.py']); changed.chmod(0o444)
        self.row['files'].pop('protocol-schema.json')
        self.row['closure_sha256'] = manifest.closure_digest(self.row)
        with self.assertRaisesRegex(manifest.ManifestError, 'closure_incomplete'):
            manifest.verify(self.root, self.row, owner_uid=os.geteuid())

    def test_rejects_symlinked_package_member_and_unpinned_image(self):
        target = self.base / 'external.py'; target.write_text('outside')
        entry = self.root / 'runner_rpc.py'; self.root.chmod(0o755); entry.chmod(0o644); entry.unlink(); entry.symlink_to(target)
        self.root.chmod(0o555)
        with self.assertRaisesRegex(manifest.ManifestError, 'file_unavailable'):
            manifest.verify(self.root, self.row, owner_uid=os.geteuid())
        self.root.chmod(0o755); entry.unlink(); entry.write_bytes(self.files['runner_rpc.py']); entry.chmod(0o444)
        self.root.chmod(0o555)
        self.row['images']['nats_phase'] = 'latest'
        self.row['closure_sha256'] = manifest.closure_digest(self.row)
        with self.assertRaisesRegex(manifest.ManifestError, 'image_unpinned'):
            manifest.verify(self.root, self.row, owner_uid=os.geteuid())


if __name__ == '__main__': unittest.main()
