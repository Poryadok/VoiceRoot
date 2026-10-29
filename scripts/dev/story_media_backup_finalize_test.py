import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import story_media_backup as backup
import story_media_backup_custody as custody
import story_media_backup_finalize as finalize


class FinalizeTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name); self.key = bytes(range(32))
        self.binding = {'operation_id': 'media-c1-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
            'source_sha': 'c' * 40, 'project': 'voice', 'resource': 'DEV', 'actor': 'scheduled_devops',
            'claim': '11111111-1111-1111-1111-111111111111', 'window': '22222222-2222-2222-2222-222222222222',
            'generation': 3, 'revision': 4, 'namespace_uid': '33333333-3333-3333-3333-333333333333',
            'pod_uid': '44444444-4444-4444-4444-444444444444', 'pvc_uid': '55555555-5555-5555-5555-555555555555',
            'pv_uid': '66666666-6666-6666-6666-666666666666', 'container_id': 'containerd://' + 'd' * 64,
            'restart_count': 7, 'restore_inputs': {'catalog_sha256': '1' * 64, 'roles_catalog_sha256': '2' * 64,
                'migration_catalog_sha256': '3' * 64, 'postgres_image': 'sha256:' + '4' * 64,
                'migrate_image': 'sha256:' + '5' * 64, 'migrate_binary_sha256': '6' * 64, 'postgres_major': 16}}
        self.manifest = backup.export(self.root, self.key, self.binding, lambda member, output: output.write(b'fixture'))
        self.receipts = {}
        for index, member in enumerate(sorted(backup.MEMBERS)):
            self.receipts[member] = {'operation_id': self.binding['operation_id'], 'snapshot_id': self.binding['snapshot_id'],
                'member': member, 'pod_uid': self.binding['pod_uid'], 'pvc_uid': self.binding['pvc_uid'],
                'pv_uid': self.binding['pv_uid'], 'container_id': self.binding['container_id'],
                'restart_count': 7, 'attempt_nonce': '%032x' % (index + 1), 'exit': 0, 'scope': 'dump-terminal-only'}

    def test_final_manifest_binds_exact_receipts_without_mutating_inputs(self):
        initial = copy.deepcopy((self.binding, self.manifest, self.receipts))
        binding, manifest = finalize.finalize(self.root, self.key, self.binding, self.manifest, self.receipts)
        self.assertEqual((self.binding, self.manifest, self.receipts), initial)
        for member, receipt in self.receipts.items():
            raw = json.dumps(receipt, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('ascii')
            self.assertEqual(binding['dump_receipts'][member], hashlib.sha256(raw).hexdigest())
        self.assertEqual(backup.verify(self.root, self.key, binding, manifest), backup.MEMBERS)
        self.assertEqual(manifest['files'], self.manifest['files'])
        path = custody.build_bundle(self.root, self.key, binding, manifest)
        self.assertEqual(custody.verify_bundle(path, self.key, binding)['bytes'], path.stat().st_size)

    def test_second_cycle_is_distinct_and_third_cycle_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            binding = copy.deepcopy(self.binding)
            binding['operation_id'] = 'media-c2-' + 'a'*32
            receipts = copy.deepcopy(self.receipts)
            for receipt in receipts.values(): receipt['operation_id'] = binding['operation_id']
            manifest = backup.export(root, self.key, binding, lambda member, output: output.write(member.encode()))
            final, signed = finalize.finalize(root, self.key, binding, manifest, receipts)
            self.assertEqual(final['operation_id'], binding['operation_id'])
            backup.verify(root, self.key, final, signed)
            binding['operation_id'] = 'media-c3-' + 'a'*32
            with self.assertRaises(finalize.FinalizeError):
                finalize.finalize(root, self.key, binding, manifest, receipts)

    def test_stale_foreign_partial_or_duplicated_receipts_fail(self):
        for fault in ('missing', 'extra', 'member', 'operation', 'snapshot', 'pod', 'nonce', 'exit', 'bool_exit', 'unknown_key', 'scope'):
            with self.subTest(fault=fault):
                receipts = copy.deepcopy(self.receipts)
                if fault == 'missing': del receipts['roles']
                if fault == 'extra': receipts['other'] = copy.deepcopy(receipts['roles'])
                if fault == 'member': receipts['file_db']['member'] = 'roles'
                if fault == 'operation': receipts['file_db']['operation_id'] = 'foreign'
                if fault == 'snapshot': receipts['file_db']['snapshot_id'] = 'foreign'
                if fault == 'pod': receipts['file_db']['pod_uid'] = self.binding['pv_uid']
                if fault == 'nonce': receipts['file_db']['attempt_nonce'] = receipts['roles']['attempt_nonce']
                if fault == 'exit': receipts['file_db']['exit'] = 1
                if fault == 'bool_exit': receipts['file_db']['exit'] = False
                if fault == 'unknown_key': receipts['file_db']['extra'] = 'unknown'
                if fault == 'scope': receipts['file_db']['scope'] = 'verified'
                with self.assertRaises(finalize.FinalizeError):
                    finalize.finalize(self.root, self.key, self.binding, self.manifest, receipts)

    def test_bad_binding_or_ciphertext_cannot_be_finalized(self):
        for fault in ('extra_binding', 'bad_uuid', 'bool_revision', 'bad_restore', 'ciphertext'):
            with self.subTest(fault=fault):
                binding = copy.deepcopy(self.binding)
                if fault == 'extra_binding': binding['unreviewed'] = 'x'
                if fault == 'bad_uuid': binding['claim'] = 'not-a-uuid'
                if fault == 'bool_revision': binding['revision'] = True
                if fault == 'bad_restore': binding['restore_inputs']['postgres_major'] = 15
                path = self.root / 'file_db.enc'; original = path.read_bytes()
                if fault == 'ciphertext': path.write_bytes(original[:-1])
                try:
                    with self.assertRaises(finalize.FinalizeError):
                        finalize.finalize(self.root, self.key, binding, self.manifest, self.receipts)
                finally:
                    path.write_bytes(original)

    def test_caller_mutation_during_verification_cannot_rebind_manifest(self):
        original_source = self.binding['source_sha']
        verify = backup.verify
        def mutate(root, key, binding, manifest):
            self.binding['source_sha'] = 'e' * 40
            return verify(root, key, binding, manifest)
        with patch.object(backup, 'verify', side_effect=mutate):
            final_binding, manifest = finalize.finalize(self.root, self.key, self.binding, self.manifest, self.receipts)
        self.assertEqual(final_binding['source_sha'], original_source)
        self.assertEqual(manifest['binding']['source_sha'], original_source)


if __name__ == '__main__': unittest.main()
