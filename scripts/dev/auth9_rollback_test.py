"""Real PostgreSQL Auth9 DOWN regression; Docker required, no host ports.

Run from any directory: python scripts/dev/auth9_rollback_test.py
Only the uniquely named disposable container created here is removed.
"""

from pathlib import Path
import hashlib
import json
import os
import subprocess
import time
import unittest
import uuid


ROOT = Path(__file__).resolve().parents[2]
MIGRATIONS = ROOT / "src/backend/migrations/auth_db"
UP = (MIGRATIONS / "000009_guest_conversion_operations.up.sql").read_text(encoding="utf-8")
DOWN = (MIGRATIONS / "000009_guest_conversion_operations.down.sql").read_text(encoding="utf-8")
ARTIFACTS = os.environ.get("AUTH9_ROLLBACK_ARTIFACT_DIR")


def record(event, **values):
    if ARTIFACTS:
        if "sql" in values:
            values["sql_sha256"] = hashlib.sha256(values["sql"].encode("utf-8")).hexdigest()
        directory = Path(ARTIFACTS)
        directory.mkdir(parents=True, exist_ok=True)
        with (directory / "sql-trace.jsonl").open("a", encoding="utf-8") as trace:
            trace.write(json.dumps({"event": event, **values}) + "\n")


if ARTIFACTS:
    manifest = {}
    for path in (Path(__file__),
                 MIGRATIONS / "000009_guest_conversion_operations.up.sql",
                 MIGRATIONS / "000009_guest_conversion_operations.down.sql"):
        data = path.read_bytes()
        manifest[str(path.relative_to(ROOT))] = hashlib.sha256(data).hexdigest()
        destination = Path(ARTIFACTS) / "source" / path.relative_to(ROOT)
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(data)
    record("source-before-run", sha256=manifest)


class Auth9RollbackTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.container = "voice-auth9-rollback-" + uuid.uuid4().hex
        subprocess.run(
            ["docker", "run", "--detach", "--network", "none", "--name", cls.container,
             "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "postgres:16-alpine"],
            check=True, capture_output=True, text=True, timeout=60)
        record("owned-container", name=cls.container, network="none", published_ports=[])
        cls.addClassCleanup(cls.remove_container)
        deadline = time.monotonic() + 40
        while time.monotonic() < deadline:
            ready = subprocess.run(
                ["docker", "exec", cls.container, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"],
                capture_output=True, timeout=5)
            if ready.returncode == 0:
                return
            time.sleep(0.1)
        raise AssertionError("owned PostgreSQL did not become ready")

    @classmethod
    def remove_container(cls):
        subprocess.run(["docker", "rm", "--force", cls.container],
                       check=True, capture_output=True, timeout=20)

    @classmethod
    def command(cls):
        return ["docker", "exec", "-i", cls.container, "psql", "-X", "-qAt",
                "-v", "ON_ERROR_STOP=1", "-h", "127.0.0.1", "-U", "postgres"]

    def sql(self, sql, *, check=True):
        result = subprocess.run(self.command(), input=sql, text=True, capture_output=True,
                                check=False, timeout=20)
        record("sql", sql=sql, stdout=result.stdout, stderr=result.stderr,
               exit=result.returncode)
        if check:
            result.check_returncode()
        return result

    def setUp(self):
        self.sql("DROP TABLE IF EXISTS guest_conversion_operations;" + UP)

    def insert(self, state):
        return ("INSERT INTO guest_conversion_operations(operation_id,account_id,otp_code_id,state) "
                f"VALUES ('{uuid.uuid4()}','{uuid.uuid4()}','{uuid.uuid4()}','{state}');")

    def test_empty_down_up(self):
        self.sql(DOWN)
        self.assertEqual(self.sql("SELECT to_regclass('guest_conversion_operations') IS NULL;").stdout.strip(), "t")
        self.sql(UP)
        self.assertEqual(self.sql("SELECT count(*) FROM guest_conversion_operations;").stdout.strip(), "0")

    def test_completed_down_up(self):
        self.sql(self.insert("COMPLETED"))
        self.sql(DOWN)
        self.assertEqual(self.sql("SELECT to_regclass('guest_conversion_operations') IS NULL;").stdout.strip(), "t")
        self.sql(UP)

    def test_populated_pending_refused(self):
        for state in ("PENDING_USER", "PENDING_EVENT"):
            with self.subTest(state=state):
                self.setUp()
                self.sql(self.insert(state))
                self.assert_refused(self.sql(DOWN, check=False), state)

    def assert_refused(self, result, state):
        self.assertNotEqual(result.returncode, 0, "DOWN discarded recoverable conversion")
        self.assertIn("non-completed work exists", result.stderr)
        self.assertEqual(self.sql("SELECT state FROM guest_conversion_operations;").stdout.strip(), state)

    def test_first_pending_writer_survives_down(self):
        for state in ("PENDING_USER", "PENDING_EVENT"):
            with self.subTest(state=state):
                self.setUp()
                self.first_writer(state)

    def test_inherited_repeatable_read_snapshot_refuses_down(self):
        for state in ("PENDING_USER", "PENDING_EVENT"):
            with self.subTest(state=state):
                self.setUp()
                rollback = subprocess.Popen(
                    self.command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE, text=True)
                try:
                    snapshot_sql = ("BEGIN ISOLATION LEVEL REPEATABLE READ;"
                                    "SET LOCAL statement_timeout='15s';"
                                    "SELECT 'RR_SNAPSHOT|' || count(*) "
                                    "FROM guest_conversion_operations;\n")
                    rollback.stdin.write(snapshot_sql)
                    rollback.stdin.flush()
                    snapshot = rollback.stdout.readline()
                    record("inherited-rr-snapshot", sql=snapshot_sql, stdout=snapshot)
                    self.assertEqual(snapshot.strip(), "RR_SNAPSHOT|0")
                    # Independent committed insert after the rollback transaction pins empty.
                    self.sql(self.insert(state))
                    binding_sql = ("SELECT operation_id,account_id,otp_code_id,state "
                                   "FROM guest_conversion_operations;")
                    admitted_binding = self.sql(binding_sql).stdout
                    rollback.stdin.write(DOWN + "\nCOMMIT;\n")
                    rollback.stdin.close()
                    rollback.stdin = None
                    stdout, stderr = rollback.communicate(timeout=20)
                    record("inherited-rr-down", sql=DOWN + "\nCOMMIT;\n",
                           stdout=stdout, stderr=stderr, exit=rollback.returncode)
                    self.assertNotEqual(rollback.returncode, 0,
                                        "old empty RR snapshot allowed DOWN after pending commit")
                    self.assertIn("READ COMMITTED", stderr)
                    self.assertEqual(self.sql(binding_sql).stdout, admitted_binding)
                finally:
                    if rollback.poll() is None:
                        rollback.kill()
                        rollback.communicate(timeout=10)

    def first_writer(self, state):
        writer = subprocess.Popen(self.command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.PIPE, text=True)
        rollback = None
        try:
            writer_sql = ("BEGIN; SET LOCAL statement_timeout='15s';" + self.insert(state)
                          + "SELECT 'WRITER_READY';\n")
            writer.stdin.write(writer_sql)
            writer.stdin.flush()
            writer_ready = writer.stdout.readline()
            record("first-writer-input", sql=writer_sql, stdout=writer_ready)
            self.assertEqual(writer_ready.strip(), "WRITER_READY")
            rollback = subprocess.Popen(self.command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, text=True)
            rollback_sql = ("SET application_name='auth9_down'; SET statement_timeout='12s';"
                            + DOWN + "\n")
            rollback.stdin.write(rollback_sql)
            rollback.stdin.close()
            rollback.stdin = None
            # Observe a real relation-lock wait, not a sleep-based ordering guess.
            deadline = time.monotonic() + 8
            while time.monotonic() < deadline:
                waiting = self.sql("""
                    SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid
                    WHERE a.application_name='auth9_down' AND NOT l.granted
                      AND l.relation='guest_conversion_operations'::regclass
                      AND l.mode='AccessExclusiveLock';
                """).stdout.strip()
                if waiting == "1":
                    break
                self.assertIsNone(rollback.poll(), "DOWN exited before waiting on writer")
                time.sleep(0.05)
            else:
                self.fail("DOWN did not wait on the first writer's relation lock")
            writer.stdin.write("COMMIT;\n")
            writer.stdin.close()
            writer.stdin = None
            writer_stdout, writer_error = writer.communicate(timeout=15)
            record("first-writer-commit", sql="COMMIT;\n", stdout=writer_stdout, stderr=writer_error,
                   exit=writer.returncode)
            self.assertEqual(writer.returncode, 0, writer_error)
            rollback_stdout, rollback_error = rollback.communicate(timeout=15)
            record("first-writer-down", sql=rollback_sql, stdout=rollback_stdout,
                   stderr=rollback_error, exit=rollback.returncode)
            self.assert_refused(subprocess.CompletedProcess([], rollback.returncode,
                                                           stderr=rollback_error), state)
        finally:
            # Close only our clients; PostgreSQL rolls back any unfinished transaction.
            for process in (writer, rollback):
                if process is not None and process.poll() is None:
                    process.kill()
                    process.communicate(timeout=10)


if __name__ == "__main__":
    unittest.main(verbosity=2)
