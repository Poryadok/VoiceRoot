"""Fixed Realtime PostgreSQL provisioning phases for the central DEV operator.

The caller owns the common lock, verified claim, immutable operation-owned Secret,
credential custody and a source-bound engine snapshot. Construct KubePostgres
with the same operation guard and pass it to Provision. Intent binds
`resource_context`, `namespace_uid`, `postgres`, and `realtime_credential`
({name, uid, resourceVersion, password_sha256}). Only password bytes go to SQL
stdin. No secret value is written to Journal or ledger. A pending or uncertain
attempt retains the lock and requires actual effect/session reconciliation;
there is no automatic retry, DROP or rollback.

The result proves only create-role/create-database readback on the captured
engine. Central separately proves Secret creation ownership, actual SCRAM/DSN
authentication and foreign object privileges, runs the pinned golang-migrate
Job, verifies its source and schema, and qualifies backup/restore. This helper
does not authorize rollout or claim migration or public P3 activation.
"""
import re
import base64
import copy
import hashlib
import json
import secrets
import subprocess

import full_cohort_schema_metadata as metadata
import media_operation_guard


class RecoveryRequired(RuntimeError):
    pass


def guard(*args):
    media_operation_guard.verify(*args)


class Provision:
    """One-shot role/database creation. A pending or unknown attempt is HOLD."""
    def __init__(self, journal, fence, client, lock, namespace_uid, postgres, password):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid, self.pg = copy.deepcopy(lock), namespace_uid, postgres
        role_sql(password)  # format check; no credential is stored in the journal
        self.password = password
        self.intent = copy.deepcopy(journal.intent)
        self.nonce = secrets.token_hex(16)
        self.poisoned = False

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or
                self.intent.get('resource_context') != self.fence.context or
                self.intent.get('namespace_uid') != self.namespace_uid):
            raise RecoveryRequired('operation identity changed')
        guard(self.fence, self.client, self.lock, self.namespace_uid)
        if self.pg.snapshot() != self.intent.get('postgres'):
            raise RecoveryRequired('postgres identity changed')

    def _secret(self):
        ref = self.intent.get('realtime_credential')
        if (not isinstance(ref, dict) or set(ref) != {'name', 'uid', 'resourceVersion', 'password_sha256'} or
                not isinstance(ref['name'], str) or not re.fullmatch(r'voice-realtime-db-[a-z0-9-]{1,43}', ref['name']) or
                not all(isinstance(ref[k], str) and ref[k] for k in ('uid', 'resourceVersion')) or
                ref['password_sha256'] != hashlib.sha256(self.password).hexdigest()):
            raise RecoveryRequired('credential identity invalid')
        obj = self.client.call('get', 'secret', ref['name'], '-o', 'json')
        m = obj.get('metadata', {})
        if (m.get('name') != ref['name'] or m.get('namespace') != 'voice-dev' or m.get('uid') != ref['uid'] or
                m.get('resourceVersion') != ref['resourceVersion'] or obj.get('immutable') is not True or
                obj.get('type') != 'Opaque' or set(obj.get('data', {})) != {'DATABASE_URL'}):
            raise RecoveryRequired('credential Secret changed')
        try:
            url = base64.b64decode(obj['data']['DATABASE_URL'], validate=True).decode('ascii')
            expected = 'postgres://realtime:' + self.password.decode('ascii') + '@voice-postgres:5432/realtime_db?sslmode=disable'
            if url != expected:
                raise ValueError()
        except Exception:
            raise RecoveryRequired('credential Secret binding invalid') from None

    def _uncertain(self):
        self.poisoned = True
        try:
            self.fence.uncertain('realtime database provisioning outcome uncertain')
        except BaseException:
            pass
        raise RecoveryRequired('retain lock and reconcile provisioning') from None

    def _owned(self, row):
        record = row.get('realtime_db_provision') if isinstance(row, dict) else None
        return (self.fence.owned(row) and row.get('status') == 'active' and isinstance(record, dict) and
                record.get('nonce') == self.nonce and record.get('operation_id') == self.fence.context['operation_id'])

    def _reserve(self):
        steps = self.journal.state['steps']
        if 'realtime-db-invocation' in steps:
            raise RecoveryRequired('prior provisioning requires reconciliation')
        steps['realtime-db-invocation'] = {'status': 'pending', 'nonce': self.nonce}
        self.journal.save()
        self.fence.authority()
        row, revision = self.fence.ledger.read()
        if not self.fence.owned(row) or row.get('status') != 'active' or 'realtime_db_provision' in row:
            raise RecoveryRequired('provision reservation exists')
        row['realtime_db_provision'] = {'nonce': self.nonce, 'operation_id': self.fence.context['operation_id'], 'steps': {}}
        self.fence.ledger.write(revision, row)
        self._guard()
        row, _ = self.fence.ledger.read()
        if not self._owned(row):
            raise RecoveryRequired('provision reservation changed')

    def _effect(self, name, sql, before, valid):
        self._guard()
        self._secret()
        if self.pg.catalog() != before:
            raise RecoveryRequired('catalog changed before effect')
        steps = self.journal.state['steps']
        key = 'realtime-db-' + name
        if key in steps:
            raise RecoveryRequired('previous attempt requires reconciliation')
        attempt = secrets.token_hex(16)
        steps[key] = {'status': 'pending', 'nonce': attempt}
        self.journal.save()
        row, revision = self.fence.ledger.read()
        if not self._owned(row) or row['realtime_db_provision']['steps']:
            if not (self._owned(row) and name == 'database' and row['realtime_db_provision']['steps'].get('role', {}).get('status') == 'done' and len(row['realtime_db_provision']['steps']) == 1):
                raise RecoveryRequired('provision effect already reserved')
        row['realtime_db_provision']['steps'][name] = {'status': 'pending', 'nonce': attempt}
        self.fence.ledger.write(revision, row)
        self._guard()
        self._secret()
        row, _ = self.fence.ledger.read()
        if not self._owned(row) or row['realtime_db_provision']['steps'].get(name) != {'status': 'pending', 'nonce': attempt}:
            raise RecoveryRequired('provision effect reservation changed')
        if self.pg.catalog() != before:
            raise RecoveryRequired('catalog changed at submission')
        self.pg.execute(sql)
        self._guard()
        self._secret()
        after = self.pg.catalog()
        if not valid(after):
            raise RecoveryRequired('provision readback mismatch')
        row, revision = self.fence.ledger.read()
        if not self._owned(row) or row['realtime_db_provision']['steps'].get(name) != {'status': 'pending', 'nonce': attempt}:
            raise RecoveryRequired('provision effect ownership changed')
        receipt = {'status': 'done', 'nonce': attempt, 'catalog': after}
        steps[key] = copy.deepcopy(receipt)
        self.journal.save()
        row['realtime_db_provision']['steps'][name] = copy.deepcopy(receipt)
        self.fence.ledger.write(revision, row)
        self._guard()
        return after

    def run(self):
        pending = False
        try:
            self._guard()
            self._secret()
            initial = self.pg.catalog()
            if initial != {'role': None, 'database': None}:
                raise RecoveryRequired('existing role or database is not adoptable')
            pending = True
            self._reserve()
            role = self._effect('role', role_sql(self.password), initial,
                                lambda c: _valid_role(c.get('role')) and c.get('database') is None)
            final = self._effect('database', database_sql(), role,
                                 lambda c: c.get('role') == role['role'] and isinstance(c.get('database'), dict) and
                                 set(c['database']) == {'oid', 'owner_oid'} and _oid(c['database'].get('owner_oid')) and
                                 c['database']['owner_oid'] == role['role']['oid'] and _oid(c['database'].get('oid')))
            self._guard()
            self._secret()
            return {'operation_id': self.fence.context['operation_id'], 'role_oid': role['role']['oid'],
                    'database_oid': final['database']['oid'], 'credential': copy.deepcopy(self.intent['realtime_credential']),
                    'postgres': copy.deepcopy(self.intent['postgres']), 'scope': 'create-only', 'migration_pass': False}
        except BaseException:
            if pending:
                self._uncertain()
            raise RecoveryRequired('provisioning refused before effect') from None


def _oid(value):
    return type(value) is int and value > 0


def _valid_role(role):
    return (isinstance(role, dict) and set(role) == {'oid', 'login', 'superuser', 'createdb', 'createrole', 'inherit', 'replication', 'bypassrls', 'memberships', 'connection_limit', 'valid_until', 'config'} and
            _oid(role['oid']) and role['login'] is True and all(role[k] is False for k in ('superuser', 'createdb', 'createrole', 'inherit', 'replication', 'bypassrls')) and type(role['memberships']) is int and role['memberships'] == 0 and
            type(role['connection_limit']) is int and role['connection_limit'] == -1 and role['valid_until'] is None and role['config'] is None)


def verify_completed(phase):
    """Revalidate the owned create-only result before admitting its database to a cut."""
    if type(phase) is not Provision or type(phase.pg) is not KubePostgres:
        raise RecoveryRequired('provision phase identity invalid')
    phase._guard()
    phase._secret()
    steps = copy.deepcopy(phase.journal.state.get('steps', {}))
    row, _ = phase.fence.ledger.read()
    record = row.get('realtime_db_provision', {})
    if (not phase._owned(row) or steps.get('realtime-db-invocation') !=
            {'status': 'pending', 'nonce': phase.nonce} or
            set(record.get('steps', {})) != {'role', 'database'}):
        raise RecoveryRequired('provision ownership changed')
    for name in ('role', 'database'):
        item = steps.get('realtime-db-' + name)
        if (not isinstance(item, dict) or set(item) != {'status', 'nonce', 'catalog'} or
                item['status'] != 'done' or not isinstance(item['nonce'], str) or
                not re.fullmatch(r'[0-9a-f]{32}', item['nonce']) or
                item != record['steps'][name]):
            raise RecoveryRequired('provision receipt changed')
    role = steps['realtime-db-role']['catalog']
    final = steps['realtime-db-database']['catalog']
    if (not isinstance(role, dict) or set(role) != {'role', 'database'} or
            not _valid_role(role['role']) or role['database'] is not None or
            not isinstance(final, dict) or set(final) != {'role', 'database'} or final['role'] != role['role'] or
            not isinstance(final['database'], dict) or set(final['database']) != {'oid', 'owner_oid'} or
            not _oid(final['database']['oid']) or final['database']['owner_oid'] != role['role']['oid'] or
            phase.pg.catalog() != final):
        raise RecoveryRequired('provision catalog changed')
    phase._guard()
    phase._secret()
    observed, _ = phase.fence.ledger.read()
    if (not phase._owned(observed) or observed.get('realtime_db_provision') != record or
            phase.journal.state.get('steps') != steps or phase.pg.catalog() != final):
        raise RecoveryRequired('provision evidence changed')
    phase._secret()
    phase._guard()
    last, _ = phase.fence.ledger.read()
    if (not phase._owned(last) or last.get('realtime_db_provision') != record or
            phase.journal.state.get('steps') != steps):
        raise RecoveryRequired('provision evidence changed')
    return {'operation_id': phase.fence.context['operation_id'], 'postgres': copy.deepcopy(phase.intent['postgres']),
            'database': 'realtime_db', 'database_oid': final['database']['oid'],
            'role': 'realtime', 'role_oid': role['role']['oid'], 'scope': 'create-only'}


CATALOG_SQL = """SELECT pg_catalog.json_build_object(
 'role', (SELECT pg_catalog.json_build_object('oid', r.oid::bigint, 'login', r.rolcanlogin,
   'superuser', r.rolsuper, 'createdb', r.rolcreatedb, 'createrole', r.rolcreaterole,
   'inherit', r.rolinherit, 'replication', r.rolreplication, 'bypassrls', r.rolbypassrls,
   'connection_limit', r.rolconnlimit, 'valid_until', r.rolvaliduntil, 'config', r.rolconfig,
   'memberships', (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid OR m.roleid=r.oid))
   FROM pg_catalog.pg_roles r WHERE r.rolname='realtime'),
 'database', (SELECT pg_catalog.json_build_object('oid', d.oid::bigint, 'owner_oid', d.datdba::bigint)
   FROM pg_catalog.pg_database d WHERE d.datname='realtime_db'));
"""


class KubePostgres:
    """Fixed DEV transport. SQL and credentials travel exclusively via stdin."""
    def __init__(self, operation_guard):
        self.operation_guard = operation_guard
        self._snapshot = metadata.kube_engine_snapshot(metadata.linux_private_bytes, operation_guard)

    def snapshot(self):
        return self._snapshot()

    def _psql(self, sql):
        script = ('export PGPASSWORD="$POSTGRES_PASSWORD"; export PGAPPNAME=voice-realtime-provision; '
                  'export PGOPTIONS="-c statement_timeout=15000 -c lock_timeout=5000 -c search_path=pg_catalog,public"; '
                  'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres')
        command = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                   '--namespace', 'voice-dev', '--request-timeout=15s', 'exec', '-i', 'voice-postgres-0',
                   '-c', 'postgres', '--', 'sh', '-ec', script]
        try:
            self.operation_guard()
            result = subprocess.run(command, input=sql.encode('ascii'), stdout=subprocess.PIPE,
                                    stderr=subprocess.DEVNULL, timeout=25)
            if result.returncode or len(result.stdout) > 65536:
                raise RecoveryRequired('postgres transport failed')
            self.operation_guard()
            return result.stdout
        except BaseException:
            raise RecoveryRequired('postgres transport failed or uncertain') from None

    def catalog(self):
        try:
            return json.loads(self._psql(CATALOG_SQL))
        except BaseException:
            raise RecoveryRequired('postgres catalog unavailable') from None

    def execute(self, sql):
        allowed_role = (r"SET log_statement = 'none';\nSET log_min_error_statement = 'panic';\n"
                        r"SET password_encryption = 'scram-sha-256';\n"
                        r"CREATE ROLE realtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD '[A-Za-z0-9_-]{32,128}';\n")
        if sql != database_sql() and (not isinstance(sql, str) or not re.fullmatch(allowed_role, sql)):
            raise RecoveryRequired('SQL outside fixed provisioning scope')
        self._psql(sql)


def role_sql(password):
    if not isinstance(password, bytes) or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}', password):
        raise ValueError('credential format invalid')
    # Only this fixed alphabet enters SQL; contents remain in private stdin.
    return ("SET log_statement = 'none';\nSET log_min_error_statement = 'panic';\n"
            "SET password_encryption = 'scram-sha-256';\n"
            "CREATE ROLE realtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD '"
            + password.decode('ascii') + "';\n")


def database_sql():
    return 'CREATE DATABASE realtime_db OWNER realtime;\n'
