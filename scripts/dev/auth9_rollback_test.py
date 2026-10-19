"""Real PostgreSQL Auth9 DOWN regression; Docker required, no host ports.

Run from any directory: python scripts/dev/auth9_rollback_test.py
Only the uniquely named disposable container created here is removed.
"""

from pathlib import Path
import subprocess
import time
import unittest
import uuid


ROOT = Path(__file__).resolve().parents[2]
MIGRATIONS = ROOT / "src/backend/migrations/auth_db"
UP = (MIGRATIONS / "000009_guest_conversion_operations.up.sql").read_text(encoding="utf-8")
DOWN = (MIGRATIONS / "000009_guest_conversion_operations.down.sql").read_text(encoding="utf-8")


class Auth9RollbackTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.container = "voice-auth9-rollback-" + uuid.uuid4().hex
        subprocess.run(
            ["docker", "run", "--detach", "--network", "none", "--name", cls.container,
             "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "postgres:16-alpine"],
            check=True, capture_output=True, text=True, timeout=60)
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
        return subprocess.run(self.command(), input=sql, text=True, capture_output=True,
                              check=check, timeout=20)

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

    def first_writer(self, state):
        writer = subprocess.Popen(self.command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.PIPE, text=True)
        rollback = None
        try:
            writer.stdin.write("BEGIN; SET LOCAL statement_timeout='15s';" + self.insert(state)
                               + "SELECT 'WRITER_READY';\n")
            writer.stdin.flush()
            self.assertEqual(writer.stdout.readline().strip(), "WRITER_READY")
            rollback = subprocess.Popen(self.command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, text=True)
            rollback.stdin.write("SET application_name='auth9_down'; SET statement_timeout='12s';"
                                 + DOWN + "\n")
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
            _, writer_error = writer.communicate(timeout=15)
            self.assertEqual(writer.returncode, 0, writer_error)
            _, rollback_error = rollback.communicate(timeout=15)
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
