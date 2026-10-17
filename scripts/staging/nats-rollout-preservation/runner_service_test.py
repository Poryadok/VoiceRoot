"""Trusted config and socket activation contract for the nonroot runner."""
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest

import runner_service as service
import tokenless_job as jobs


class RunnerServiceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name) / ('a' * 40); self.root.mkdir()
        self.binding = {'claim_name': 'voice-nats-jsdata-d20261017a',
            'claim_uid': '11111111-1111-4111-8111-111111111111', 'pv_name': 'local-pv-voice-nats',
            'pv_uid': '22222222-2222-4222-8222-222222222222', 'node_name': 'k3s-worker-1'}
        self.config = {'schema': service.CONFIG_SCHEMA, 'runner_source_sha': self.root.name,
            'runner_closure_sha256': 'c' * 64, 'api_server': 'https://10.0.0.1:6443',
            'ca_file': service.CA_FILE, 'token_file': service.TOKEN_FILE,
            'state_dir': service.STATE_DIR, 'runner_image': 'ghcr.io/poryadok/voice-nats-rollout-runner@sha256:' + 'd' * 64,
            'storage_binding': self.binding, 'verification_pvc_size_bytes': 48 << 30,
            'verification_pvc_max_bytes': 50 << 30, 'verification_storage_class': 'local-path'}
        self.path = Path(self.temp.name) / 'runner-config.json'
        self.write()

    def tearDown(self): self.temp.cleanup()

    def write(self):
        if self.path.exists(): self.path.chmod(0o644)
        self.path.write_text(json.dumps(self.config, sort_keys=True)); self.path.chmod(0o444)

    def test_loads_root_custody_and_dynamic_operation_with_bootstrap_bound_storage(self):
        loaded = service.load_config(self.path, self.root, owner_uid=os.geteuid())
        self.assertEqual(loaded['storage_binding'], jobs.validate_binding(self.binding))
        self.assertEqual(service.binding_for(loaded, 'b' * 12), loaded['storage_binding'])
        with self.assertRaisesRegex(service.ServiceError, 'operation_invalid'):
            service.binding_for(loaded, '../etc/passwd')

    def test_candidate_selected_image_api_endpoint_or_storage_is_not_configurable(self):
        for update, error in (({'runner_image': 'latest'}, 'image_invalid'),
            ({'api_server': 'http://10.0.0.1:6443'}, 'server_invalid'),
            ({'storage_binding': self.binding | {'claim_uid': 'not-a-uid'}}, 'binding_invalid'),
            ({'app_release_sha': 'f' * 40}, 'config_shape_invalid')):
            with self.subTest(update=update):
                self.config.update(update); self.write()
                with self.assertRaisesRegex(service.ServiceError, error):
                    service.load_config(self.path, self.root, owner_uid=os.geteuid())
                self.config = {'schema': service.CONFIG_SCHEMA, 'runner_source_sha': self.root.name,
                    'runner_closure_sha256': 'c' * 64, 'api_server': 'https://10.0.0.1:6443',
                    'ca_file': service.CA_FILE, 'token_file': service.TOKEN_FILE, 'state_dir': service.STATE_DIR,
                    'runner_image': 'ghcr.io/poryadok/voice-nats-rollout-runner@sha256:' + 'd' * 64,
                    'storage_binding': self.binding, 'verification_pvc_size_bytes': 48 << 30,
                    'verification_pvc_max_bytes': 50 << 30, 'verification_storage_class': 'local-path'}

    def test_verification_pvc_size_ceiling_and_storage_class_are_required(self):
        loaded = service.load_config(self.path, self.root, owner_uid=os.geteuid())
        self.assertEqual(loaded['verification_pvc_size_bytes'], 48 << 30)
        self.assertEqual(loaded['verification_pvc_max_bytes'], 50 << 30)
        self.assertEqual(loaded['verification_storage_class'], 'local-path')
        for field, value, expected in (
            ('verification_pvc_size_bytes', 0, 'verification_pvc_ceiling_invalid'),
            ('verification_pvc_size_bytes', -1, 'verification_pvc_ceiling_invalid'),
            ('verification_pvc_size_bytes', True, 'verification_pvc_ceiling_invalid'),
            ('verification_pvc_size_bytes', 1 << 63, 'verification_pvc_ceiling_invalid'),
            ('verification_pvc_size_bytes', 51 << 30, 'verification_pvc_ceiling_invalid'),
            ('verification_pvc_max_bytes', 0, 'verification_pvc_ceiling_invalid'),
            ('verification_pvc_max_bytes', False, 'verification_pvc_ceiling_invalid'),
            ('verification_pvc_max_bytes', 1 << 63, 'verification_pvc_ceiling_invalid'),
            ('verification_storage_class', '', 'verification_storage_class_invalid'),
            ('verification_storage_class', 'Local Path', 'verification_storage_class_invalid'),
            ('verification_storage_class', 'local-path/', 'verification_storage_class_invalid'),
        ):
            with self.subTest(value=value):
                self.config[field] = value
                self.write()
                with self.assertRaisesRegex(service.ServiceError, expected):
                    service.load_config(self.path, self.root, owner_uid=os.geteuid())
                self.config[field] = {'verification_pvc_size_bytes': 48 << 30,
                    'verification_pvc_max_bytes': 50 << 30, 'verification_storage_class': 'local-path'}[field]
        for field in ('verification_pvc_size_bytes', 'verification_pvc_max_bytes', 'verification_storage_class'):
            with self.subTest(missing=field):
                self.config.pop(field)
                self.write()
                with self.assertRaisesRegex(service.ServiceError, 'config_shape_invalid'):
                    service.load_config(self.path, self.root, owner_uid=os.geteuid())
                self.config[field] = {'verification_pvc_size_bytes': 48 << 30,
                    'verification_pvc_max_bytes': 50 << 30, 'verification_storage_class': 'local-path'}[field]

    def test_rejects_mutable_or_symlinked_config(self):
        self.path.chmod(0o644)
        with self.assertRaisesRegex(service.ServiceError, 'config_custody_invalid'):
            service.load_config(self.path, self.root, owner_uid=os.geteuid())
        self.path.chmod(0o444)
        link = Path(self.temp.name) / 'link'; link.symlink_to(self.path)
        with self.assertRaisesRegex(service.ServiceError, 'config_unavailable'):
            service.load_config(link, self.root, owner_uid=os.geteuid())


if __name__ == '__main__': unittest.main()
