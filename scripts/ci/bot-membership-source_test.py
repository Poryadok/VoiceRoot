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
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.path = Path(self.directory.name) / "source.uuid"
        self.identity = str(uuid.uuid4())
        self.tables = "0"
        self.valid = "t"
        self.empty = "t"
        self.migrations = 0

    def query(self, sql):
        if sql == source.EMPTY_SQL:
            return self.tables
        return f"{self.identity}|{self.valid}|{self.empty}"

    def migrate(self):
        self.migrations += 1
        self.tables = "12"

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
