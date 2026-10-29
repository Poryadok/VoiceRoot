import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch
import sys

import story_media_backup_cohort as cohort
import story_media_backup_cohort_finalize as finalize
import story_media_backup_cohort_custody as custody
import story_media_backup_cohort_custody as custody


class FinalizeCohortTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name); self.key = bytes(range(32)); self.databases = ['auth_db', 'voice_db']
        engine = {'namespace_uid': '33333333-3333-3333-3333-333333333333',
                  'pod_uid': '44444444-4444-4444-4444-444444444444', 'pvc_uid': '55555555-5555-5555-5555-555555555555',
                  'pv_uid': '66666666-6666-6666-6666-666666666666', 'container_id': 'containerd://' + 'd' * 64,
                  'restart_count': 7}
        self.binding = {'operation_id': 'media-c1-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
            'source_sha': 'c' * 40, 'project': 'voice', 'resource': 'DEV', 'actor': 'scheduled_devops',
            'claim': '11111111-1111-1111-1111-111111111111', 'window': '22222222-2222-2222-2222-222222222222',
            'generation': 3, 'revision': 4, 'databases': self.databases,
            'sources': {'auth_db': {'engine': engine, 'server_start': 'fixture', 'database_oid': 1},
                        'voice_db': {'engine': engine, 'server_start': 'fixture', 'database_oid': 2},
                        'roles': {'engine': engine, 'server_start': 'fixture'}},
            'restore_inputs': {'catalog_sha256': '1' * 64, 'roles_catalog_sha256': '2' * 64,
                'migration_catalog_sha256': '3' * 64, 'postgres_image': 'sha256:' + '4' * 64,
                'migrate_image': 'sha256:' + '5' * 64, 'migrate_binary_sha256': '6' * 64, 'postgres_major': 16}}
        self.manifest = cohort.export(self.root, self.key, self.binding, self.databases,
                                      lambda member, out: out.write(b'fixture'))
        self.receipts = {}
        for index, member in enumerate(sorted((*self.databases, 'roles'))):
            source = self.binding['sources'][member]
            self.receipts[member] = {'operation_id': self.binding['operation_id'], 'snapshot_id': self.binding['snapshot_id'],
                'member': member, 'pod_uid': engine['pod_uid'], 'pvc_uid': engine['pvc_uid'], 'pv_uid': engine['pv_uid'],
                'container_id': engine['container_id'], 'restart_count': engine['restart_count'],
                'server_start': source['server_start'], 'database_oid': source.get('database_oid'),
                'attempt_nonce': '%032x' % (index+1), 'exit': 0, 'scope': 'dump-terminal-only'}

    def test_dev_validator_refuses_staging_binding_with_same_contents(self):
        binding=copy.deepcopy(self.binding);binding['resource']='STAGING'
        with self.assertRaises(finalize.CohortFinalizeError):
            finalize.validate_binding(binding,self.databases)

    def test_staging_wrappers_do_not_access_custody_or_dump_after_authority_failure(self):
        binding=copy.deepcopy(self.binding);binding['resource']='STAGING'
        module=Mock();module.validate_backup_binding.side_effect=RuntimeError('untrusted session')
        with patch.dict(sys.modules,{'staging_cohort_preservation':module}):
            with self.assertRaises(cohort.CohortError):
                cohort.export_staging(object(),binding,self.databases,{}, {})
            with self.assertRaises(cohort.CohortError):
                cohort.verify_staging(object(),binding,self.databases,self.manifest,{}, {})
            with self.assertRaises(finalize.CohortFinalizeError):
                finalize.finalize_staging(object(),binding,self.databases,self.manifest,self.receipts,{}, {})
        module.backup_custody.assert_not_called();module.produce_dump.assert_not_called()

    def test_exact_receipts_are_bound_without_mutating_inputs(self):
        original = copy.deepcopy((self.binding, self.manifest, self.receipts))
        binding, manifest = finalize.finalize(self.root, self.key, self.binding, self.databases, self.manifest, self.receipts)
        self.assertEqual((self.binding, self.manifest, self.receipts), original)
        for member, receipt in self.receipts.items():
            raw = json.dumps(receipt, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('ascii')
            self.assertEqual(binding['dump_receipts'][member], hashlib.sha256(raw).hexdigest())
        self.assertEqual(cohort.verify(self.root, self.key, binding, self.databases, manifest),
                         {'auth_db', 'voice_db', 'roles'})
        path = custody.build_bundle(self.root, self.key, binding, self.databases, manifest)
        self.assertEqual(custody.verify_bundle(path, self.key, binding, self.databases)['bytes'], path.stat().st_size)

    def test_missing_or_foreign_receipts_cannot_finalize(self):
        for fault in ('missing', 'extra', 'wrong_oid', 'wrong_start', 'wrong_pv', 'duplicate_nonce', 'bool_exit'):
            with self.subTest(fault=fault):
                receipts = copy.deepcopy(self.receipts)
                if fault == 'missing': del receipts['roles']
                if fault == 'extra': receipts['other'] = copy.deepcopy(receipts['roles'])
                if fault == 'wrong_oid': receipts['voice_db']['database_oid'] = 1
                if fault == 'wrong_start': receipts['voice_db']['server_start'] = 'other'
                if fault == 'wrong_pv': receipts['voice_db']['pv_uid'] = 'other'
                if fault == 'duplicate_nonce': receipts['voice_db']['attempt_nonce'] = receipts['roles']['attempt_nonce']
                if fault == 'bool_exit': receipts['voice_db']['exit'] = False
                with self.assertRaises(finalize.CohortFinalizeError):
                    finalize.finalize(self.root, self.key, self.binding, self.databases, self.manifest, receipts)

    def test_second_cycle_finalizes_and_builds_only_its_own_bundle(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); binding = copy.deepcopy(self.binding)
            binding['operation_id'] = 'media-c2-' + 'a' * 32
            receipts = copy.deepcopy(self.receipts)
            for receipt in receipts.values(): receipt['operation_id'] = binding['operation_id']
            manifest = cohort.export(root, self.key, binding, self.databases, lambda name, out: out.write(b'fixture'))
            final_binding, final_manifest = finalize.finalize(root, self.key, binding, self.databases, manifest, receipts)
            path = custody.build_bundle(root, self.key, final_binding, self.databases, final_manifest)
            self.assertEqual(custody.verify_bundle(path, self.key, final_binding, self.databases)['bytes'], path.stat().st_size)
            foreign = copy.deepcopy(final_binding); foreign['operation_id'] = 'media-c3-' + 'a' * 32
            with self.assertRaises(custody.CohortCustodyError):
                custody.verify_bundle(path, self.key, foreign, self.databases)


if __name__ == '__main__': unittest.main()
