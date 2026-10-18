"""Trusted config and socket activation contract for the nonroot runner."""
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest

import runner_service as service


class RunnerServiceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name) / ('a' * 40); self.root.mkdir()
        self.config = {'schema': service.CONFIG_SCHEMA, 'runner_source_sha': self.root.name,
            'runner_closure_sha256': 'c' * 64, 'api_server': 'https://10.0.0.1:6443',
            'ca_file': service.CA_FILE, 'token_file': service.TOKEN_FILE,
            'state_dir': service.STATE_DIR, 'runner_image': 'ghcr.io/poryadok/voice-nats-rollout-runner@sha256:' + 'd' * 64,
            'phase_image': 'ghcr.io/poryadok/voice-nats-phase@sha256:' + 'e' * 64,
            'verification_pvc_size_bytes': 48 << 30,
            'verification_pvc_max_bytes': 50 << 30, 'verification_storage_class': 'local-path'}
        self.path = Path(self.temp.name) / 'runner-config.json'
        self.write()

    def tearDown(self): self.temp.cleanup()

    def write(self):
        if self.path.exists(): self.path.chmod(0o644)
        self.path.write_text(json.dumps(self.config, sort_keys=True)); self.path.chmod(0o444)

    def test_loads_root_custody_without_static_source_identity(self):
        loaded = service.load_config(self.path, self.root, owner_uid=os.geteuid())
        self.assertNotIn('storage_binding', loaded)

    def test_installed_phase_image_pin_is_distinct_from_runner_package_pin(self):
        config = self.config
        installed = {'closure_sha256': config['runner_closure_sha256'],
            'images': {'nats_phase': config['phase_image']}}
        self.assertNotEqual(config['runner_image'], config['phase_image'])
        service.validate_installed_pins(config, installed)
        installed['images']['nats_phase'] = config['runner_image']
        with self.assertRaisesRegex(service.ServiceError, 'installed_pin_mismatch'):
            service.validate_installed_pins(config, installed)

    def test_candidate_selected_image_api_endpoint_or_storage_is_not_configurable(self):
        for update, error in (({'runner_image': 'latest'}, 'image_invalid'),
            ({'phase_image': 'latest'}, 'phase_image_invalid'),
            ({'api_server': 'http://10.0.0.1:6443'}, 'server_invalid'),
            ({'storage_binding': {'claim_uid': 'not-a-uid'}}, 'config_shape_invalid'),
            ({'app_release_sha': 'f' * 40}, 'config_shape_invalid')):
            with self.subTest(update=update):
                self.config.update(update); self.write()
                with self.assertRaisesRegex(service.ServiceError, error):
                    service.load_config(self.path, self.root, owner_uid=os.geteuid())
                self.config = {'schema': service.CONFIG_SCHEMA, 'runner_source_sha': self.root.name,
                    'runner_closure_sha256': 'c' * 64, 'api_server': 'https://10.0.0.1:6443',
                    'ca_file': service.CA_FILE, 'token_file': service.TOKEN_FILE, 'state_dir': service.STATE_DIR,
                    'runner_image': 'ghcr.io/poryadok/voice-nats-rollout-runner@sha256:' + 'd' * 64,
                    'phase_image': 'ghcr.io/poryadok/voice-nats-phase@sha256:' + 'e' * 64,
                    'verification_pvc_size_bytes': 48 << 30,
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
            ('verification_storage_class', 'local..path', 'verification_storage_class_invalid'),
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

    def test_obsolete_replacement_pvc_settings_are_not_part_of_trusted_config(self):
        loaded = service.load_config(self.path, self.root, owner_uid=os.geteuid())
        self.assertNotIn('replacement_pvc_size_bytes', loaded)
        self.assertNotIn('replacement_pvc_max_bytes', loaded)
        self.assertNotIn('replacement_storage_class', loaded)
        for field, value in (('replacement_pvc_size_bytes', 48 << 30),
                             ('replacement_pvc_max_bytes', 50 << 30),
                             ('replacement_storage_class', 'local-path')):
            with self.subTest(field=field):
                self.config[field] = value; self.write()
                with self.assertRaisesRegex(service.ServiceError, 'config_shape_invalid'):
                    service.load_config(self.path, self.root, owner_uid=os.geteuid())
                del self.config[field]

    def test_rejects_mutable_or_symlinked_config(self):
        self.path.chmod(0o644)
        with self.assertRaisesRegex(service.ServiceError, 'config_custody_invalid'):
            service.load_config(self.path, self.root, owner_uid=os.geteuid())
        self.path.chmod(0o444)
        link = Path(self.temp.name) / 'link'; link.symlink_to(self.path)
        with self.assertRaisesRegex(service.ServiceError, 'config_unavailable'):
            service.load_config(link, self.root, owner_uid=os.geteuid())


if __name__ == '__main__': unittest.main()
