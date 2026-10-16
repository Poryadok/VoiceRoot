import importlib.util
from contextlib import closing
from pathlib import Path
import sqlite3
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('storage_migration', Path(__file__).with_name('container-storage-home.py'))
migration = importlib.util.module_from_spec(spec)
spec.loader.exec_module(migration)


class PreservationTests(unittest.TestCase):
    def test_added_volumes_are_allowed_but_original_replacement_or_loss_is_not(self):
        original = {'metadata': {'name': 'old', 'uid': 'stable'}}
        added = {'metadata': {'name': 'dev', 'uid': 'new'}}
        migration.require_preserved_volumes([original], [original, added])
        for live in ([added], [{'metadata': {'name': 'old', 'uid': 'replaced'}}]):
            with self.assertRaises(RuntimeError):
                migration.require_preserved_volumes([original], live)

    def test_completed_migration_accepts_retained_home_rollback(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            with patch.object(migration, 'BASE', base):
                self.assertFalse(migration.rollback_present('/var/lib/docker'))
                (base / 'rollback/lib/docker').mkdir(parents=True)
                self.assertTrue(migration.rollback_present('/var/lib/docker'))

    def test_content_change_with_same_row_count_is_detected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'tracker.db'
            with closing(sqlite3.connect(path)) as db, db:
                db.execute('CREATE TABLE projects (id INTEGER PRIMARY KEY, payload TEXT)')
                db.execute('INSERT INTO projects VALUES (1, ?)', ('original',))
            before = migration.sqlite_facts(path)
            with closing(sqlite3.connect(path)) as db, db:
                db.execute('UPDATE projects SET payload=? WHERE id=1', ('modified',))
            after = migration.sqlite_facts(path)
            self.assertEqual(before['projects']['rows'], after['projects']['rows'])
            self.assertNotEqual(before['projects']['sha256'], after['projects']['sha256'])

    def test_only_exact_sources_can_be_relocated(self):
        for path in ('/', '/var', '/var/lib', '/var/lib/docker/../other', '/home'):
            with self.assertRaises(ValueError):
                migration.target(path)
        for path in migration.PATHS:
            self.assertTrue(migration.target(path).as_posix().startswith('/home/container-data/active/'))


if __name__ == '__main__':
    unittest.main()
