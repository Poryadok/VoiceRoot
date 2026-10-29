"""Fixed DEV Space PostgreSQL transport; caller owns durable fencing."""
import os
import selectors
import subprocess
import time

import full_cohort_schema_metadata as metadata


class SpacePostgresError(RuntimeError):
    pass


PSQL = ('export PGPASSWORD="$POSTGRES_PASSWORD"; '
        'export PGAPPNAME=voice-space-fresh-control; '
        'export PGOPTIONS="-c statement_timeout=15000 -c lock_timeout=5000 -c search_path=pg_catalog"; '
        'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres')
COMMAND = ('kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
           '--namespace', 'voice-dev', '--request-timeout=15s', 'exec', '-i',
           'voice-postgres-0', '-c', 'postgres', '--', 'sh', '-ec', PSQL)
OID_QUERY = "SELECT COALESCE((SELECT oid::bigint FROM pg_catalog.pg_database WHERE datname='space_db'),0);\n"


def drop_script(oid):
    if type(oid) is not int or not 0 < oid < 2**32:
        raise ValueError('invalid original Space database OID')
    return ("SELECT CASE WHEN pg_catalog.pg_try_advisory_lock(pg_catalog.hashtextextended('voice-space-native-installer',0)) "
            "THEN 'SELECT 1 WHERE false' ELSE 'SELECT 1/0' END\n\\gexec\n"
            "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_database "
            "WHERE datname='space_db' AND oid=" + str(oid) + "::oid) "
            "THEN 'DROP DATABASE space_db' ELSE 'SELECT 1/0' END\n\\gexec\n")


class KubeSpacePostgres:
    """Fixed transport; a guarded phase owns all admission and effect records."""
    def __init__(self, guard):
        if not callable(guard):
            raise ValueError('operation guard required')
        self._guard = guard
        self._engine = metadata.kube_engine_snapshot(metadata.linux_private_bytes, guard)

    def snapshot(self):
        return self._engine()

    def _run(self, sql):
        process = None
        selector = selectors.DefaultSelector()
        try:
            metadata._fence(self._guard)
            process = subprocess.Popen(COMMAND, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                       stderr=subprocess.DEVNULL)
            process.stdin.write(sql.encode('ascii'))
            process.stdin.close()
            deadline = time.monotonic() + 25
            selector.register(process.stdout, selectors.EVENT_READ)
            chunks, size = [], 0
            while selector.get_map():
                left = deadline - time.monotonic()
                if left <= 0 or not selector.select(left):
                    raise SpacePostgresError('postgres transport timed out')
                chunk = os.read(process.stdout.fileno(), min(4096, 8193 - size))
                if not chunk:
                    selector.unregister(process.stdout)
                    break
                size += len(chunk)
                if size > 8192:
                    raise SpacePostgresError('postgres response exceeds bound')
                chunks.append(chunk)
            if process.wait(timeout=max(0.01, deadline - time.monotonic())) != 0:
                raise SpacePostgresError('postgres command failed')
            metadata._fence(self._guard)
            return b''.join(chunks)
        except metadata.GuardLost:
            raise
        except BaseException:
            raise SpacePostgresError('postgres operation unavailable or uncertain') from None
        finally:
            selector.close()
            if process is not None:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                if process.stdout is not None:
                    process.stdout.close()
                if process.stdin is not None and not process.stdin.closed:
                    process.stdin.close()

    def space_oid(self):
        raw = self._run(OID_QUERY)
        try:
            value = raw.decode('ascii').strip()
            if not value.isdecimal() or str(int(value)) != value or not 0 <= int(value) < 2**32:
                raise ValueError()
            return int(value)
        except (ValueError, UnicodeError):
            raise SpacePostgresError('postgres database identity unavailable') from None

    def drop_exact(self, oid):
        output = self._run(drop_script(oid))
        if output not in (b'', b'DROP DATABASE\n'):
            raise SpacePostgresError('postgres drop outcome uncertain')
