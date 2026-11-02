"""Real metadata/clone verifiers with synthetic Kubernetes and PG observations."""
import copy
import hashlib
import json
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest import mock

import cohort_backup_phases
import cohort_restore_journal
import full_cohort_backup_metadata as metadata
import full_cohort_backup_metadata_test as fixture
import full_cohort_go_migration as go
import media_operation_journal


class JointTest(unittest.TestCase):
    def test_protected_metadata_and_clone_receipt_feed_go_preflight(self):
        base = fixture.JournalBoundaryTest('test_actual_journal_and_fence_reservation_rejects_duplicate')
        base.setUp(); self.addCleanup(base.doCleanups)
        client = object.__new__(metadata.frontend_runtime_executor.KubectlTransport)
        client.journal = base.journal; base.ledger.client = client
        package = object.__new__(metadata.package.PackagePhase)
        inventory = object.__new__(metadata.inventory.InventoryPhase)
        cut = object.__new__(metadata.writer_cut.WriterCut)
        for obj in (package, inventory, cut):
            obj.fence = base.fence; obj.lock = {}; obj.namespace_uid = 'ns'
        inventory.package = package; cut.verify_drained = lambda: None; cut.migration_history = []
        package.verify_current = lambda token: {'package_sha256': 'f' * 64}
        engine = {'namespace_uid': 'ns', 'service_ip': '10.0.0.1'}
        report = {'database_observations': {'workloads': {'chat': [{'status': 'observed', 'result': {
            'metadata': {'engine': engine, 'session': {'server_start': 'start'}}}}]}}}
        inventory.verify_current = inventory.verify_receipt = lambda token: report
        inventory._admin_role = lambda: 'postgres'
        dbs = {'postgres': 1, 'chat_db': 2}
        props = {db: {'allowconn': True} for db in dbs}
        catalog = metadata.affected.FullCatalogPhase(base.journal, base.fence, client, {}, 'ns', inventory)
        capture = metadata.BackupMetadataPhase(base.journal, base.fence, client, {}, 'ns', package, inventory,
                                               catalog, cut, metadata.canonical.ROOT)
        operation = Path(base.tmp.name) / base.context['operation_id']; operation.mkdir(mode=0o700)
        sql = {'000001_initial.up.sql': 'a' * 64, '000001_initial.down.sql': 'b' * 64,
               '000002_second.up.sql': 'c' * 64}
        snapshot = {'files': {'scripts/dev/migrations.py': go.source_plan.MIGRATIONS_SOURCE_SHA256} |
                    {'src/backend/migrations/chat_db/' + k:v for k,v in sql.items()}}
        roles = {'roles': [{'name': 'chat_login', 'login': True}], 'memberships': []}
        schema = {'database_owner': 'chat_login', 'schema_owner': 'chat_login', 'schema_acl': None,
                  'objects': [], 'defaults': [], 'history': [{'version': 1, 'dirty': False}]}
        def read(db, *args):
            return copy.deepcopy(roles if db == 'roles' else schema if db == 'chat_db' else {'database_owner': 'postgres'})
        credential = {'role': 'chat_login', 'host': 'voice-postgres', 'sslmode': 'disable', 'password': 'fixture-only',
                      'references': {'url': {'uid': 'source-secret', 'resource_version': '1'}}}
        safe = {k:v for k,v in credential.items() if k != 'password'}
        def tool(): capture._tool_state({'status': 'removed'}); return 'd' * 64
        projected = {'all_databases': dbs, 'databases': dbs, 'excluded': {}, 'properties': props}
        with mock.patch.object(metadata.media_operation_guard, 'verify'), \
             mock.patch.object(metadata.affected, '_full_catalog', return_value=projected), \
             mock.patch.object(metadata.media_backup_upload_phase, 'ROOT', Path(base.tmp.name)), \
             mock.patch.object(metadata.schema, 'kube_engine_snapshot', return_value=lambda: engine), \
             mock.patch.object(metadata.unmanaged, 'validate_source_catalog'), \
             mock.patch.object(metadata.restore, 'migration_inventory'), mock.patch.object(metadata.restore, '_source'), \
             mock.patch.object(capture, '_source_files', return_value=snapshot), \
             mock.patch.object(capture, '_tool_sha', side_effect=tool), mock.patch.object(capture, '_read', side_effect=read), \
             mock.patch.object(go, '_credential', return_value=credential), mock.patch.object(go, '_login', return_value=safe):
            catalog.run('token'); captured = capture.run('token')
            self.assertEqual(captured['metadata']['migrations']['chat_db']['to'], 2)
            self.assertEqual(captured['migration_roles']['chat_db'], safe)
            binding = {k: captured[k] for k in ('operation_id', 'source_sha', 'databases', 'sources', 'restore_inputs', 'full_catalog')}
            binding['snapshot_id'] = 'capture-' + 'a' * 32
            manifest = 'b' * 64; marker = {'manifest_sha256': manifest}
            role_digest = hashlib.sha256(json.dumps(captured['migration_roles'], sort_keys=True,
                                  separators=(',', ':'), ensure_ascii=True, allow_nan=False).encode('ascii')).hexdigest()
            intent = {'status': 'pending', 'operation_id': binding['operation_id'], 'snapshot_id': binding['snapshot_id'],
                      'invocation_nonce': 'invocation', 'manifest_sha256': manifest, 'artifact_id': 1,
                      'migration_roles_sha256': role_digest}
            receipt = {'operation_id': binding['operation_id'], 'snapshot_id': binding['snapshot_id'],
                       'source_sha': binding['source_sha'], 'manifest_sha256': manifest, 'cleanup': 'verified',
                       'databases': {'chat_db': {'mode': 'migrate', 'from': 1, 'to': 2}},
                       'migration_logins': {'chat_db': {'role': 'chat_login', 'from': 1, 'to': 2}}}
            done = {'status': 'done', 'intent': intent, 'receipt': receipt}
            readback = {'status': 'done', 'intent': {'manifest_sha256': manifest}, 'receipt': {'artifact_id': 1}}
            resources = {}
            for index, kind in enumerate(('postgres', 'migrate'), 1):
                name = 'voice-media-restore-' + str(index) * 32
                resources[kind] = {'status': 'removed', 'nonce': str(index) * 32, 'name': name,
                                   'labels': {'voice.test.owner': name}, 'operation_id': binding['operation_id'],
                                   'invocation_nonce': 'invocation', 'container_id': str(index) * 64}
            base.ledger.row.update(cohort_restore=copy.deepcopy(done), cohort_readback=copy.deepcopy(readback),
                                   cohort_restore_resources=copy.deepcopy(resources))
            j = media_operation_journal.Journal(Path(base.tmp.name) / 'cohort-journal', base.journal.intent)
            self.addCleanup(j.close)
            j.state['steps'].update({'cohort-restore': copy.deepcopy(done), 'cohort-readback': copy.deepcopy(readback),
                                     'cohort-restore-resource-postgres': resources['postgres'],
                                     'cohort-restore-resource-migrate': resources['migrate']}); j.save()
            cohort = object.__new__(cohort_backup_phases.CohortPhases)
            cohort.phases = SimpleNamespace(fence=base.fence, lock={}, namespace_uid='ns', invocation='invocation', journal=j)
            cohort.catalog_phase = catalog
            with mock.patch.object(cohort_backup_phases.CohortPhases, '_guard'), \
                 mock.patch.object(cohort_restore_journal.readback, '_snapshot', return_value=(binding, {}, marker, {})):
                clone = cohort_restore_journal.verify_go_role(cohort, capture, 'token', 'chat_db')
                self.assertEqual((clone['role'], clone['from'], clone['to']), ('chat_login', 1, 2))
                admitted = go.qualified_plan(capture, cohort, cut, 'token', 'chat_db')
                self.assertEqual(admitted['credential'], safe)
                base.ledger.row['cohort_restore']['receipt']['migration_logins']['chat_db']['role'] = 'forged'
                with self.assertRaises(go.MigrationError): go.qualified_plan(capture, cohort, cut, 'token', 'chat_db')
