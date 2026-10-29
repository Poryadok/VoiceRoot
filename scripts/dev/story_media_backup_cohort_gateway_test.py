import tempfile
import unittest
from pathlib import Path
import hashlib
import json
import copy

import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as restore


class GatewayPlanTest(unittest.TestCase):
    def test_gateway_is_explicit_restore_only_with_no_invented_ledger(self):
        raw = b'SELECT 1;'
        plan = {'engine': 'unmanaged', 'from': None, 'to': None,
                'files': {'000001_base.up.sql': hashlib.sha256(raw).hexdigest()}}
        self.assertIn('gateway_db', cohort.ALLOWED_DATABASES)
        self.assertEqual(restore.classify_plan('gateway_db', plan), 'restore-only')
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'src/backend/migrations/gateway_db'; path.mkdir(parents=True)
            (path / '000001_base.up.sql').write_bytes(raw)
            self.assertEqual(restore.migration_inventory(directory, ['gateway_db'], {'gateway_db': plan}),
                             {'gateway_db': {'000001_base.up.sql': raw}})
            (path / '000001_base.up.sql').write_bytes(b'SELECT 2;')
            with self.assertRaises(restore.CohortRestoreError):
                restore.migration_inventory(directory, ['gateway_db'], {'gateway_db': plan})

    def test_gateway_migration_claims_are_rejected(self):
        for plan in ({'engine': 'golang-migrate', 'from': 1, 'to': 2, 'files': {'x': 'a' * 64}},
                     {'engine': 'unmanaged', 'from': 1, 'to': 1, 'files': {}},
                     {'engine': 'unmanaged', 'from': None, 'to': None, 'files': {}}):
            with self.subTest(plan=plan), self.assertRaises(restore.CohortRestoreError):
                restore.classify_plan('gateway_db', plan)

    def test_observed_ledger_must_be_absent_and_boolean(self):
        source = {'engine': {'same': 'fixture'}, 'server_start': 'fixture'}
        binding = {'operation_id': 'media-c1-' + 'a'*32, 'snapshot_id': 'capture-' + 'b'*32,
                   'source_sha': 'c'*40, 'databases': ['gateway_db'],
                   'sources': {'gateway_db': {**source, 'database_oid': 1}, 'roles': source},
                   'dump_receipts': {'gateway_db': 'd'*64, 'roles': 'e'*64}}
        metadata = {'catalog': {'gateway_db': {'database_owner': 'admin', 'schema_owner': 'admin',
            'schema_acl': None, 'objects': [], 'defaults': [],
            'history': {'schema_migrations': False, 'flyway_schema_history': False}}},
            'roles': {'roles': [], 'memberships': []},
            'migrations': {'gateway_db': {'engine': 'unmanaged', 'from': None, 'to': None,
                                          'files': {'000001_base.up.sql': 'a'*64}}}}
        def validate(value):
            digest = lambda obj: hashlib.sha256(json.dumps(obj, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode()).hexdigest()
            binding['restore_inputs'] = {'catalog_sha256': digest(value['catalog']), 'roles_catalog_sha256': digest(value['roles']),
                'migration_catalog_sha256': digest(value['migrations']), 'postgres_image': 'pg', 'migrate_image': 'migrate',
                'migrate_binary_sha256': 'a'*64, 'postgres_major': 16}
            return restore._source(binding, ['gateway_db'], value, 'pg', 'migrate')
        validate(metadata)
        for bad in (True, 0, None):
            changed = copy.deepcopy(metadata); changed['catalog']['gateway_db']['history']['schema_migrations'] = bad
            with self.subTest(value=bad), self.assertRaises(restore.CohortRestoreError): validate(changed)


if __name__ == '__main__': unittest.main()
