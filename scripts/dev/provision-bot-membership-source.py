#!/usr/bin/env python3
"""Owned Compose migration: create original Bot source artifact, or verify it.

The caller owns the migration window (Bot must not be running). A missing artifact
is accepted only before any user table exists. Migration success followed by lost
artifact publication deliberately requires operator reconciliation, never adoption.
"""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import uuid

EMPTY_SQL = "SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema')"
STATE_SQL = """SELECT s.source_generation::text,
 (SELECT count(*)=1 AND bool_and(version=9 AND NOT dirty) FROM schema_migrations)
 AND s.issued_pairs=(SELECT count(*) FROM bot_space_membership_heads)
 AND s.issued_operations=(SELECT count(*) FROM bot_space_membership_operations)
 AND s.issued_calls=(SELECT count(*) FROM bot_space_membership_calls),
 s.issued_pairs=0 AND s.issued_operations=0 AND s.issued_calls=0
 FROM bot_space_membership_source s WHERE singleton"""
LEGACY_STATE_SQL = STATE_SQL.replace("version=9", "version=8")


def canonical(raw):
    value = raw.removesuffix("\n")
    if len(value) != 36 or str(uuid.UUID(value)) != value or uuid.UUID(value).int == 0:
        raise ValueError("invalid retained Bot source identity")
    return value


def provision(path, query, migrate):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    lock = path.with_name(path.name + ".lock")
    lock.mkdir()  # Refuse concurrent or interrupted provisioning; never steal.
    try:
        if path.is_symlink():
            raise ValueError("source artifact must not be a symbolic link")
        retained = path.exists()
        if retained:
            with path.open("r", encoding="ascii", newline="") as stream:
                expected = canonical(stream.read(38))
        else:
            if query(EMPTY_SQL) != "0":
                raise ValueError("Bot database is retained but original source artifact is missing")
            migrate()
            expected = None
        parts = query(STATE_SQL).split("|")
        if retained and (len(parts) != 3 or parts[1] != "t"):
            legacy = query(LEGACY_STATE_SQL).split("|")
            if len(legacy) != 3 or legacy[1] != "t":
                raise ValueError("Bot migration/source counters are not current")
            if canonical(legacy[0]) != expected:
                raise ValueError("Bot database differs from retained source artifact")
            migrate()
            parts = query(STATE_SQL).split("|")
        if len(parts) != 3 or parts[1] != "t":
            raise ValueError("Bot migration/source counters are not current")
        source = canonical(parts[0])
        if retained:
            if source != expected:
                raise ValueError("Bot database differs from retained source artifact")
            return "KNOWN_RETAINED"
        if parts[2] != "t":
            raise ValueError("fresh Bot source already issued authority")
        descriptor, temporary = tempfile.mkstemp(prefix=".source-", dir=path.parent)
        try:
            with os.fdopen(descriptor, "w", encoding="ascii", newline="\n") as stream:
                stream.write(source + "\n")
                stream.flush()
                os.fsync(stream.fileno())
            # Identity is not a credential; read permission permits the nonroot
            # service UID to consume the single read-only bind-mounted file.
            os.chmod(temporary, 0o644)
            os.link(temporary, path)  # Atomic publication; never overwrite.
            if os.name != "nt":
                directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
                try:
                    os.fsync(directory)
                finally:
                    os.close(directory)
        finally:
            os.unlink(temporary)
        return "OWNED_FRESH"
    finally:
        lock.rmdir()


def main():
    if len(sys.argv) < 4 or sys.argv[2] != "--":
        raise ValueError("usage: provision-bot-membership-source.py SOURCE_FILE -- MIGRATE_COMMAND...")
    env = os.environ.copy()
    env["PGPASSWORD"] = env.get("PGPASSWORD", env.get("POSTGRES_PASSWORD", "voice"))
    command = ["psql", "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1",
               "-h", env.get("PGHOST", "postgres"), "-p", env.get("PGPORT", "5432"),
               "-U", env.get("PGUSER", "voice"), "-d", "bot_db"]
    def query(sql):
        return subprocess.check_output(command + ["-c", sql], env=env, text=True).strip()
    result = provision(sys.argv[1], query, lambda: subprocess.run(sys.argv[3:], check=True))
    print("Bot membership source " + result + "; original artifact preserved")


if __name__ == "__main__":
    try:
        main()
    except subprocess.SubprocessError:
        # The migrate invocation contains a DSN; never echo argv on failure.
        print("Bot source provisioning command failed; original artifact was not replaced", file=sys.stderr)
        sys.exit(1)
    except (ValueError, OSError) as error:
        print("Bot source provisioning refused: " + str(error), file=sys.stderr)
        sys.exit(1)
