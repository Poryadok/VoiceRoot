import hashlib
import multiprocessing
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import json

import story_media_backup_restore as restore


class MigrationInputsTest(unittest.TestCase):
    def test_source_metadata_must_match_authenticated_sidecar_hashes(self):
        plans = {db: {'from': 1, 'to': 2, 'files': {'000002_x.up.sql': 'a' * 64}} for db in restore.DATABASES}
        catalog = {db: {'database_owner': 'voice', 'schema_owner': 'voice', 'schema_acl': None,
                        'objects': [], 'defaults': [], 'history': [{'version': 1, 'dirty': False}]}
                   for db in restore.DATABASES}
        metadata = {'catalog': catalog, 'roles': {'roles': [], 'memberships': []}, 'migrations': plans}
        def digest(value):
            return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('ascii')).hexdigest()
        pg, tool = 'sha256:' + 'a' * 64, 'sha256:' + 'b' * 64
        binding = {'operation_id': 'media-c1-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
                   'source_sha': 'c' * 40, 'dump_receipts': {m: 'd' * 64 for m in ('file_db', 'story_db', 'messaging_db', 'roles')},
                   'restore_inputs': {'catalog_sha256': digest(catalog), 'roles_catalog_sha256': digest(metadata['roles']),
                        'migration_catalog_sha256': digest(plans), 'postgres_image': pg, 'migrate_image': tool,
                        'migrate_binary_sha256': 'e' * 64, 'postgres_major': 16}}
        restore._source_binding(binding, metadata, pg, tool)
        metadata['catalog']['file_db']['database_owner'] = 'foreign'
        with self.assertRaises(restore.RestoreError): restore._source_binding(binding, metadata, pg, tool)

    def test_fifo_migration_is_rejected_without_blocking(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); plans = {}
            for db in ('file_db', 'story_db', 'messaging_db'):
                folder = root / db; folder.mkdir()
                (folder / '000001_base.down.sql').write_bytes(b'SELECT 1;')
                os.mkfifo(folder / '000002_x.up.sql', 0o600)
                plans[db] = {'from': 1, 'to': 2, 'files': {
                    '000001_base.down.sql': hashlib.sha256(b'SELECT 1;').hexdigest(),
                    '000002_x.up.sql': 'a' * 64}}
            def attempt():
                try: restore.migration_sources(root, plans)
                except restore.RestoreError: return
                raise RuntimeError('FIFO accepted')
            process = multiprocessing.get_context('fork').Process(target=attempt)
            process.start(); process.join(2); stuck = process.is_alive()
            if stuck: process.terminate(); process.join(5)
            self.assertFalse(stuck); self.assertEqual(process.exitcode, 0)

    def test_exact_hashes_inventory_and_gaps_are_checked(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            plans = {}
            for db in ('file_db', 'story_db', 'messaging_db'):
                folder = root / db; folder.mkdir()
                files = {'000027_base.up.sql': b'SELECT 1;', '000027_base.down.sql': b'SELECT 1;',
                         '000028_first.up.sql': b'SELECT 1;', '000030_last.up.sql': b'SELECT 2;'}
                for name, data in files.items(): (folder / name).write_bytes(data)
                plans[db] = {'from': 27, 'to': 30,
                             'files': {name: hashlib.sha256(data).hexdigest() for name, data in files.items()}}
            payload = restore.migration_sources(root, plans)
            self.assertEqual(payload['file_db']['000030_last.up.sql'], b'SELECT 2;')
            (root / 'file_db' / '000028_first.up.sql').write_bytes(b'changed')
            with self.assertRaises(restore.RestoreError): restore.migration_sources(root, plans)
            (root / 'file_db' / '000028_first.up.sql').write_bytes(files['000028_first.up.sql'])
            (root / 'file_db' / '000029_unexpected.up.sql').write_bytes(b'new')
            with self.assertRaises(restore.RestoreError): restore.migration_sources(root, plans)


class CleanupTest(unittest.TestCase):
    def test_startup_wait_requires_an_authenticated_sql_probe(self):
        with patch.object(restore, '_docker', side_effect=[restore.RestoreError('starting'), b'160014']) as docker, \
             patch.object(restore.time, 'sleep'):
            self.assertEqual(restore._ready_major('fixture-id', 'admin'), 16)
        for call in docker.call_args_list:
            self.assertIn('-h 127.0.0.1', ' '.join(call.args))
            self.assertIn('POSTGRES_PASSWORD', ' '.join(call.args))

    def test_foreign_identity_never_removed_and_marks_uncertain(self):
        class Journal:
            def __init__(self): self.events = []
            def uncertain(self, kind, nonce): self.events.append((kind, nonce))
        journal = Journal(); calls = []
        def docker(*args, **kwargs):
            calls.append(args)
            return json.dumps([{'Id': 'f' * 64, 'Config': {'Labels': {'voice.test.owner': 'foreign'}}}]).encode()
        with patch.object(restore, '_docker', side_effect=docker):
            with self.assertRaises(restore.RestoreUncertain):
                restore._cleanup(journal, [('postgres', 'n', 'a' * 64, 'owned', [])])
        self.assertFalse(any(call[0] == 'rm' for call in calls))
        self.assertEqual(journal.events, [('postgres', 'n')])

    def test_symlink_migration_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); plans = {}
            for db in ('file_db', 'story_db', 'messaging_db'):
                folder = root / db; folder.mkdir()
                (folder / '000002_x.up.sql').symlink_to('/etc/passwd')
                (folder / '000001_base.down.sql').write_bytes(b'SELECT 1;')
                plans[db] = {'from': 1, 'to': 2, 'files': {'000001_base.down.sql': hashlib.sha256(b'SELECT 1;').hexdigest(),
                                                           '000002_x.up.sql': 'a' * 64}}
            with self.assertRaises(restore.RestoreError): restore.migration_sources(root, plans)


if __name__ == '__main__': unittest.main()
