#!/usr/bin/env python3
"""Fixed gate-control v1, one connection and transaction; never accepts arbitrary SQL.

Trusted maintenance credentials are supplied via AUTH_DB_MAINTENANCE_DSN.
This source does not provision roles, enable a runtime or authorize execution.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path

RESOURCES = Path(__file__).resolve().parents[2] / "src/backend/auth/src/main/resources/roleidentityauthority"


def resources():
    expected = json.loads((RESOURCES / "ddl-barrier-v1.json").read_text(encoding="utf-8"))
    query = (RESOURCES / "gate-catalog.sql").read_text(encoding="utf-8")
    plan = (RESOURCES / "maintenance/gate-control-v1.sql").read_text(encoding="utf-8")
    # The fixed complete operation is reconstructible from the accepted closed
    # metadata inventory. Neither a CLI path nor database content supplies SQL.
    fixed = "DROP EVENT TRIGGER auth_identity_ddl_gate;\nCREATE EVENT TRIGGER auth_identity_ddl_gate ON ddl_command_start\n  WHEN TAG IN (" + ", ".join("'" + tag + "'" for tag in expected["tags"]) + ")\n  EXECUTE FUNCTION public.auth_identity_ddl_gate();\n"
    if plan != fixed:
        raise ValueError("fixed maintenance source mismatch")
    return expected, query, plan


def validate(metadata, owner, expected, *, preimage):
    if isinstance(metadata, str):
        metadata = json.loads(metadata)
    if not isinstance(metadata, dict) or not owner or any(metadata.get(k) != owner for k in ("barrier_owner", "function_owner", "event_owner")):
        raise ValueError("maintenance owner mismatch")
    if any(metadata.get(k) is not True for k in ("relation_ok", "acl_ok", "function_ok", "binding_ok", "major_ok")):
        raise ValueError("maintenance catalog unavailable")
    if metadata.get("event_enabled") not in (("O", "D") if preimage else ("O",)):
        raise ValueError("maintenance gate disabled")
    tags = metadata.get("event_tags")
    if not isinstance(tags, list) or len(tags) != len(set(tags)) or set(tags) != set(expected["tags"]):
        raise ValueError("maintenance tag mismatch")
    source = metadata.get("function_source")
    if not isinstance(source, str) or hashlib.sha256(source.replace("\r\n", "\n").encode()).hexdigest() != expected["sourceSha256"]:
        raise ValueError("maintenance function mismatch")


def maintain(connection, owner):
    expected, query, plan = resources()  # Passive reads precede transaction/locks.
    if not connection.autocommit:
        raise ValueError("maintenance requires a fresh autocommit connection")
    try:
        with connection.cursor() as cursor:
            cursor.execute("BEGIN ISOLATION LEVEL READ COMMITTED")
            cursor.execute("SET LOCAL lock_timeout='2s'")
            cursor.execute("SET LOCAL statement_timeout='2s'")
            cursor.execute("SET LOCAL idle_in_transaction_session_timeout='3s'")
            cursor.execute("LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE")
            cursor.execute(query)
            rows = cursor.fetchall()
            if len(rows) != 1 or len(rows[0]) != 1:
                raise ValueError("maintenance preimage unavailable")
            validate(rows[0][0], owner, expected, preimage=True)
            cursor.execute(plan)
            cursor.execute(query)
            rows = cursor.fetchall()
            if len(rows) != 1 or len(rows[0]) != 1:
                raise ValueError("maintenance postimage unavailable")
            validate(rows[0][0], owner, expected, preimage=False)
            connection.commit()
    except BaseException:
        connection.rollback()
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--schema-owner", required=True)
    args = parser.parse_args()
    import psycopg  # Existing trusted operator environment; no runtime app grant.
    with psycopg.connect(os.environ["AUTH_DB_MAINTENANCE_DSN"], autocommit=True, connect_timeout=3) as connection:
        maintain(connection, args.schema_owner)


if __name__ == "__main__":
    main()
