"""Pure transaction-double tests: never opens a connection or executes PostgreSQL."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("maintenance", Path(__file__).with_name("auth-identity-maintenance.py"))
maintenance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(maintenance)


class Connection:
    autocommit = True
    def __init__(self, rows, fail_plan=False):
        self.rows, self.fail_plan, self.calls = iter(rows), fail_plan, []
    def cursor(self): return self
    def __enter__(self): return self
    def __exit__(self, *args): return False
    def execute(self, sql):
        self.calls.append(sql)
        if self.fail_plan and sql.startswith("DROP EVENT TRIGGER"):
            raise RuntimeError("injected plan failure")
    def fetchall(self): return next(self.rows)
    def commit(self): self.calls.append("COMMIT")
    def rollback(self): self.calls.append("ROLLBACK")


class MaintenanceTest(unittest.TestCase):
    def good(self, enabled="O"):
        expected, _, _ = maintenance.resources()
        sql = (maintenance.RESOURCES.parent / "db/migration/V31__auth_identity_ddl_barrier.sql").read_text()
        result = {key: True for key in ("relation_ok", "acl_ok", "function_ok", "binding_ok", "major_ok")}
        result.update({key: "trusted_owner" for key in ("barrier_owner", "function_owner", "event_owner")})
        result.update(event_enabled=enabled, event_tags=expected["tags"], function_source=sql.split("AS $$", 1)[1].split("$$;", 1)[0])
        return result

    def test_exclusive_first_single_connection_pre_and_post_commit(self):
        connection = Connection([[ [self.good("D")] ], [ [self.good()] ]])
        maintenance.maintain(connection, "trusted_owner")
        calls = connection.calls
        self.assertEqual(calls[0], "BEGIN ISOLATION LEVEL READ COMMITTED")
        self.assertTrue(calls[4].startswith("LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE"))
        self.assertEqual(calls[-1], "COMMIT")
        self.assertEqual(sum(sql.startswith("DROP EVENT TRIGGER") for sql in calls), 1)
        self.assertEqual(sum("jsonb_build_object" in sql for sql in calls), 2)

    def test_bad_preimage_never_runs_plan(self):
        for field in ("barrier_owner", "function_owner", "event_owner", "relation_ok", "acl_ok", "function_ok", "binding_ok", "major_ok", "function_source", "event_tags"):
            bad = self.good()
            bad[field] = [] if field == "event_tags" else False
            connection = Connection([[[bad]]])
            with self.assertRaises((ValueError, TypeError)):
                maintenance.maintain(connection, "trusted_owner")
            self.assertEqual(connection.calls[-1], "ROLLBACK")
            self.assertFalse(any(sql.startswith("DROP EVENT TRIGGER") for sql in connection.calls))

    def test_plan_failure_and_disabled_postimage_rollback(self):
        for connection in (Connection([[[self.good()]]], fail_plan=True), Connection([[[self.good()]], [[self.good("D")]]])):
            with self.assertRaises((ValueError, RuntimeError)):
                maintenance.maintain(connection, "trusted_owner")
            self.assertEqual(connection.calls[-1], "ROLLBACK")
            self.assertNotIn("COMMIT", connection.calls)

    def test_ambient_transaction_rejected_before_sql(self):
        connection = Connection([])
        connection.autocommit = False
        with self.assertRaises(ValueError): maintenance.maintain(connection, "trusted_owner")
        self.assertEqual(connection.calls, [])


if __name__ == "__main__":
    unittest.main()
