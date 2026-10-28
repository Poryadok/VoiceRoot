#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import tempfile
import unittest
import uuid

spec = importlib.util.spec_from_file_location("bot_source", Path(__file__).parents[1] / "dev/provision-bot-membership-source.py")
source = importlib.util.module_from_spec(spec)
spec.loader.exec_module(source)


class SourceProvisioningTests(unittest.TestCase):
    def test_session_epoch_schema_is_required_before_source_publication(self):
        # The source identity and counters still belong to migration 8, while
        # migration 9 adds the Bot session-epoch fence to the current schema.
        self.assertIn("version=9 AND NOT dirty", source.STATE_SQL)
        self.assertIn("count(*)=1", source.STATE_SQL)
        for table, counter in (("bot_space_membership_heads", "issued_pairs"),
                               ("bot_space_membership_operations", "issued_operations"),
                               ("bot_space_membership_calls", "issued_calls")):
            self.assertIn(f"s.{counter}=(SELECT count(*) FROM {table})", source.STATE_SQL)

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.path = Path(self.directory.name) / "source.uuid"
        self.identity = str(uuid.uuid4())
        self.tables = "0"
        self.valid = "t"
        self.empty = "t"
        self.migrations = 0
        self.version = 9

    def query(self, sql):
        if sql == source.EMPTY_SQL:
            return self.tables
        expected_version = 9 if sql == source.STATE_SQL else 8
        valid = self.valid if self.version == expected_version else "f"
        return f"{self.identity}|{valid}|{self.empty}"

    def migrate(self):
        self.migrations += 1
        self.tables = "12"
        self.version = 9

    def run_provision(self):
        return source.provision(self.path, self.query, self.migrate)

    def test_fresh_and_retained_restart_preserve_original(self):
        self.assertEqual(self.run_provision(), "OWNED_FRESH")
        original = self.path.read_bytes()
        self.empty = "f"  # Retained installations are legitimate.
        self.assertEqual(self.run_provision(), "KNOWN_RETAINED")
        self.assertEqual(self.migrations, 1)
        self.assertEqual(self.path.read_bytes(), original)

    def test_retained_missing_artifact_never_adopts(self):
        self.tables = "12"
        with self.assertRaisesRegex(ValueError, "artifact is missing"):
            self.run_provision()
        self.assertEqual(self.migrations, 0)
        self.assertFalse(self.path.exists())

    def test_retained_v8_upgrades_only_after_identity_and_counter_validation(self):
        self.run_provision()
        original = self.path.read_bytes()
        self.version = 8
        self.empty = "f"
        self.assertEqual(self.run_provision(), "KNOWN_RETAINED")
        self.assertEqual(self.migrations, 2)
        self.assertEqual(self.path.read_bytes(), original)
        self.assertEqual(self.version, 9)

    def test_retained_v8_invalid_identity_or_counters_never_migrates(self):
        self.run_provision()
        original = self.path.read_bytes()
        self.version = 8
        self.valid = "f"
        with self.assertRaisesRegex(ValueError, "not current"):
            self.run_provision()
        self.assertEqual(self.migrations, 1)
        self.valid = "t"
        self.identity = str(uuid.uuid4())
        with self.assertRaisesRegex(ValueError, "differs"):
            self.run_provision()
        self.assertEqual(self.migrations, 1)
        self.assertEqual(self.path.read_bytes(), original)

    def test_failed_or_changed_upgrade_keeps_original_artifact(self):
        self.run_provision()
        original = self.path.read_bytes()
        self.version = 8
        def failed():
            raise RuntimeError("migration failed")
        with self.assertRaisesRegex(RuntimeError, "migration failed"):
            source.provision(self.path, self.query, failed)
        self.assertEqual(self.path.read_bytes(), original)
        def changed():
            self.version = 9
            self.identity = str(uuid.uuid4())
        with self.assertRaisesRegex(ValueError, "differs"):
            source.provision(self.path, self.query, changed)
        self.assertEqual(self.path.read_bytes(), original)

    def test_source_mismatch_preserves_artifact(self):
        self.run_provision()
        original = self.path.read_bytes()
        self.identity = str(uuid.uuid4())
        with self.assertRaisesRegex(ValueError, "differs"):
            self.run_provision()
        self.assertEqual(self.path.read_bytes(), original)

    def test_empty_or_malformed_artifact_never_migrates(self):
        for content in ("", "not-a-uuid", str(uuid.UUID(int=0)), self.identity + "\nextra"):
            self.path.write_text(content)
            with self.assertRaises(ValueError):
                self.run_provision()
        self.assertEqual(self.migrations, 0)

    def test_dirty_or_incomplete_source_does_not_publish(self):
        self.valid = "f"
        with self.assertRaisesRegex(ValueError, "not current"):
            self.run_provision()
        self.assertFalse(self.path.exists())
        self.valid = "t"
        with self.assertRaisesRegex(ValueError, "artifact is missing"):
            self.run_provision()
        self.assertEqual(self.migrations, 1)

    def test_fresh_already_issued_authority_does_not_publish(self):
        self.empty = "f"
        with self.assertRaisesRegex(ValueError, "already issued"):
            self.run_provision()
        self.assertFalse(self.path.exists())

    def test_concurrent_or_interrupted_lock_is_not_stolen(self):
        lock = self.path.with_name("source.uuid.lock")
        lock.mkdir()
        with self.assertRaises(FileExistsError):
            self.run_provision()
        self.assertTrue(lock.is_dir())
        self.assertEqual(self.migrations, 0)


if __name__ == "__main__":
    unittest.main()
