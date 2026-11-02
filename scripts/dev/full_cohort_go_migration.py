"""Fenced generic Go migrations with source, backup and clone-role evidence.

The shared WriterCut and its source-owned Go observer must be installed by the
consumer. Guarded observations are not an atomic lock against external writers.
"""
import re
import copy
import base64
from datetime import datetime, timezone
import hashlib
import json
import os
import ipaddress
from pathlib import Path
import selectors
import subprocess
import time
from urllib.parse import parse_qsl, unquote, urlsplit

import auth_user_candidate_probe
import cohort_backup_phases
import cohort_readback as readback
import cohort_restore_journal
import frontend_runtime_executor
import full_cohort_backup_metadata as metadata
import full_cohort_go_migration_plan as source_plan
import full_cohort_schema_metadata as schema
import media_operation_guard
import media_operation_journal
import realtime_migration_job
import resource_fencing
import secrets
import story_media_backup_cohort_restore as restore
import writer_cut


class MigrationError(RuntimeError):
    pass


def _check_clone(clone, database, role, plan, source_sha):
    if (not isinstance(clone, dict) or set(clone) != {'database', 'role', 'from', 'to', 'source_sha', 'snapshot_id',
                                                      'manifest_sha256', 'migration_roles_sha256', 'scope'} or
            clone['database'] != database or clone['role'] != role or clone['from'] != plan['from'] or
            clone['to'] != plan['to'] or clone['source_sha'] != source_sha or
            clone['scope'] != 'isolated-clone-migration-login-only' or
            not isinstance(clone['snapshot_id'], str) or not re.fullmatch(r'capture-[a-f0-9]{32}', clone['snapshot_id']) or
            any(not isinstance(clone[k], str) or not re.fullmatch(r'[a-f0-9]{64}', clone[k])
                for k in ('manifest_sha256', 'migration_roles_sha256'))):
        raise MigrationError('clone migration authority unavailable')


def _postgres_password():
    """Read only the fixed admin key; stdout remains private memory."""
    try:
        template = ('{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}'
                    '{{if eq (index .data "POSTGRES_PASSWORD") nil}}M{{else}}P{{index .data "POSTGRES_PASSWORD"}}{{end}}')
        raw = schema.linux_private_bytes(['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
               '--namespace', 'voice-dev', '--request-timeout=15s', 'get', 'secret', 'voice-app-secrets',
               '-o', 'go-template=' + template])
        uid, rv, payload = raw.split(b'\n', 2)
        if not uid or not rv or payload[:1] != b'P':
            raise ValueError()
        return {'uid': uid.decode('ascii'), 'resource_version': rv.decode('ascii'),
                'value': base64.b64decode(payload[1:], validate=True).decode('utf-8')}
    except BaseException:
        raise MigrationError('migration credential unavailable') from None


def _credential(database, guard):
    """Resolve two guarded reads of exactly the Job's source-owned key(s)."""
    try:
        if database not in source_plan.DATABASES:
            raise ValueError()
        reader = schema.kube_key_reader(schema.linux_private_bytes)
        service = database.removesuffix('_db')
        def read():
            schema._fence(guard)
            if service == 'user':
                username = reader('configmap', 'voice-app-config', 'POSTGRES_USER')
                password = _postgres_password()
                if (not username or not password or not isinstance(username['value'], str) or
                        not schema.SAFE.fullmatch(username['value']) or not isinstance(password['value'], str) or
                        not re.fullmatch(r'[A-Za-z0-9._~-]{1,1024}', password['value'])):
                    raise ValueError()
                result = {'role': username['value'], 'sslmode': 'disable', 'host': 'voice-postgres', 'password': password['value'],
                          'references': {'user': {k:username[k] for k in ('uid', 'resource_version')},
                                         'password': {k:password[k] for k in ('uid', 'resource_version')}}}
            else:
                key = reader('secret', 'voice-app-secrets', service.upper() + '_DATABASE_URL')
                if not key:
                    raise ValueError()
                role, sslmode, password, host = _parse_url(database, key['value'])
                result = {'role': role, 'sslmode': sslmode, 'host': host, 'password': password,
                          'references': {'url': {k:key[k] for k in ('uid', 'resource_version')}}}
            schema._fence(guard)
            return result
        first = read()
        if read() != first:
            raise ValueError()
        return first
    except schema.GuardLost:
        raise
    except BaseException:
        raise MigrationError('migration credential unavailable') from None


_LOGIN_SCRIPT = ("IFS= read -r PGPASSWORD; export PGPASSWORD; export PGAPPNAME=voice-cohort-observe; "
                 "export PGOPTIONS='-c default_transaction_read_only=on -c statement_timeout=5000 -c lock_timeout=3000'; "
                 'export PGSSLMODE="$3"; exec psql -h "$5" -p 5432 -X -qAt -v ON_ERROR_STOP=1 -U "$1" -d "$2" -c "$4"')
_LOGIN_SQL = ("SELECT json_build_object('database',current_database(),'database_oid',"
              "(SELECT oid::bigint FROM pg_catalog.pg_database WHERE datname=current_database()),"
              "'role',current_user,'server_start',pg_catalog.pg_postmaster_start_time(),"
              "'read_only',current_setting('transaction_read_only'))")


def _same_start(left, right):
    try:
        def instant(value):
            if not isinstance(value, str): raise ValueError()
            parsed = datetime.fromisoformat(value.replace('Z', '+00:00'))
            if parsed.tzinfo is None or parsed.utcoffset() is None: raise ValueError()
            return parsed.astimezone(timezone.utc)
        return instant(left) == instant(right)
    except (TypeError, ValueError, OverflowError):
        return False


def _login(database, credential, source, guard):
    """Authenticate exact credentials via stdin, bound to the observed engine."""
    process = selector = None
    try:
        fence = lambda: schema._fence(guard)
        snapshot = schema.kube_engine_snapshot(schema.linux_private_bytes, fence)
        fence()
        if snapshot() != source['engine']:
            raise ValueError()
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default', '--namespace', 'voice-dev',
                '--request-timeout=15s', 'exec', '-i', 'voice-postgres-0', '-c', 'postgres', '--', 'sh', '-ec',
                _LOGIN_SCRIPT, '--', credential['role'], database, credential['sslmode'], _LOGIN_SQL, credential['host']]
        process = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        process.stdin.write(credential['password'].encode('utf-8') + b'\n'); process.stdin.close()
        selector = selectors.DefaultSelector(); selector.register(process.stdout, selectors.EVENT_READ)
        deadline = time.monotonic() + 20; chunks = []; size = 0
        while selector.get_map():
            remaining = deadline - time.monotonic()
            if remaining <= 0 or not selector.select(remaining):
                raise ValueError()
            chunk = os.read(process.stdout.fileno(), min(8192, 8193 - size))
            if not chunk:
                selector.unregister(process.stdout); break
            size += len(chunk)
            if size > 8192:
                raise ValueError()
            chunks.append(chunk)
        if process.wait(timeout=max(.01, deadline - time.monotonic())):
            raise ValueError()
        observed = json.loads(b''.join(chunks))
        expected = {'database': database, 'database_oid': source['database_oid'], 'role': credential['role'], 'read_only': 'on'}
        fence()
        if (not isinstance(observed, dict) or set(observed) != set(expected) | {'server_start'} or
                any(observed[k] != value for k,value in expected.items()) or
                not _same_start(observed['server_start'], source['server_start']) or snapshot() != source['engine']):
            raise ValueError()
        return {k:v for k,v in credential.items() if k != 'password'}
    except schema.GuardLost:
        raise
    except BaseException:
        raise MigrationError('migration login unavailable') from None
    finally:
        if selector is not None: selector.close()
        if process is not None:
            if process.poll() is None: process.kill(); process.wait()
            if process.stdout is not None: process.stdout.close()


def _parse_url(database, value):
    """Resolve privately; callers must never emit the returned password."""
    try:
        if database not in source_plan.DATABASES or not isinstance(value, str) or len(value.encode()) > 4096:
            raise ValueError()
        host, actual, role = schema._url(value, 'voice-dev')
        parsed = urlsplit(value)
        if (actual != database or not schema.SAFE.fullmatch(role) or parsed.password is None or
                parsed.username != role or parsed.netloc.count('@') != 1):
            raise ValueError()
        password = unquote(parsed.password, errors='strict')
        if not password or len(password.encode()) > 1024 or any(c in password for c in '\r\n\0'):
            raise ValueError()
        options = dict(parse_qsl(parsed.query, keep_blank_values=True, strict_parsing=True))
        if set(options) != {'sslmode'}:
            raise ValueError()
        return role, options['sslmode'], password, host
    except BaseException:
        raise MigrationError('migration credential route unavailable') from None


def _check_order(cut, observed, database):
    """Require the same Go order used by the single qualified clone restore."""
    plans = observed['metadata']['migrations']
    ordered = [db for db in observed['databases'] if db in source_plan.DATABASES and plans[db]['from'] < plans[db]['to']]
    if database not in ordered or not isinstance(cut.migration_history, list):
        raise MigrationError('migration order unavailable')
    prior = [entry for entry in cut.migration_history if isinstance(entry, dict) and entry.get('kind') == 'go']
    required = ordered[:ordered.index(database)]
    if len(prior) != len(required):
        raise MigrationError('migration order changed')
    for entry, name in zip(prior, required):
        phase = entry.get('phase')
        if (type(phase) is not Migration or phase.cut is not cut or phase.database != name or
                entry.get('finished') is not True):
            raise MigrationError('prior migration authority unavailable')
        verify_completed(phase)


def qualified_plan(metadata_phase, cohort_phase, cut, token, database):
    """Check actual backup/readback/isolated-restore authority for one target."""
    try:
        if (type(metadata_phase) is not metadata.BackupMetadataPhase or
                type(cohort_phase) is not cohort_backup_phases.CohortPhases or
                type(cut) is not writer_cut.WriterCut or database not in source_plan.DATABASES):
            raise ValueError()
        p = cohort_phase.phases
        if (metadata_phase.fence.context != p.fence.context or cut.fence.context != p.fence.context or
                metadata_phase.lock != p.lock or cut.lock != p.lock or
                metadata_phase.namespace_uid != p.namespace_uid or cut.namespace_uid != p.namespace_uid or
                metadata_phase.fence.ledger.read() != p.fence.ledger.read() or cut.fence.ledger.read() != p.fence.ledger.read()):
            raise ValueError()
        cohort_phase._guard(); cut.verify_drained()
        observed = metadata.verify_completed(metadata_phase, token)
        plans = observed['metadata']['migrations']
        if database not in plans: raise ValueError()
        _check_order(cut, observed, database)
        row, revision = p.fence.ledger.read()
        binding, _, marker, _ = readback._snapshot(cohort_phase, row)
        clone = cohort_restore_journal.verify_go_role(cohort_phase, metadata_phase, token, database)
        plan = plans[database]
        if (binding['restore_inputs'] != observed['restore_inputs'] or binding['databases'] != observed['databases'] or
                binding['sources'] != observed['sources'] or binding['full_catalog'] != observed['full_catalog'] or
                restore.classify_plan(database, plan, True) != 'migrate'):
            raise ValueError()
        _check_clone(clone, database, observed['migration_roles'][database]['role'], plan, binding['source_sha'])
        if clone['manifest_sha256'] != marker['manifest_sha256'] or clone['snapshot_id'] != binding['snapshot_id']:
            raise ValueError()
        restore._source(binding, binding['databases'], observed['metadata'],
                        binding['restore_inputs']['postgres_image'], binding['restore_inputs']['migrate_image'])
        receipt = p.journal.state['steps']['cohort-restore']['receipt']
        if receipt['databases'][database] != {'mode': 'migrate', 'from': plan['from'], 'to': plan['to']}:
            raise ValueError()
        restore.migration_inventory(metadata_phase.source_root, binding['databases'], plans, full_catalog=True)
        cut.verify_drained(); cohort_phase._guard()
        if (metadata.verify_completed(metadata_phase, token) != observed or
                p.fence.ledger.read() != (row, revision) or
                cohort_restore_journal.verify_go_role(cohort_phase, metadata_phase, token, database) != clone):
            raise ValueError()
        return {'database': database, 'plan': copy.deepcopy(plan), 'source': copy.deepcopy(binding['sources'][database]),
                'credential': copy.deepcopy(observed['migration_roles'][database]),
                'metadata_receipt_sha256': p.fence.ledger.read()[0][metadata.LEDGER]['receipt_sha256'],
                'restore_provenance': {'manifest_sha256': marker['manifest_sha256'], 'clone_receipt_sha256': _hash(clone)}}
    except BaseException:
        raise MigrationError('source-qualified migrated restore unavailable') from None


def _secret_name(operation, database):
    if (not isinstance(operation, str) or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}', operation) or
            database not in source_plan.DATABASES):
        raise MigrationError('migration secret identity invalid')
    return 'voice-go-input-' + database.removesuffix('_db') + '-' + operation[-32:]


def _secret(operation, database, credential, service_ip):
    """Construct private operation-only credentials; never log or journal body."""
    try:
        name = _secret_name(operation, database)
        if (credential['sslmode'] != 'disable' or not schema.SAFE.fullmatch(credential['role']) or
                str(ipaddress.IPv4Address(service_ip)) != service_ip or
                not isinstance(credential['password'], str) or not credential['password']):
            raise ValueError()
        return {'apiVersion': 'v1', 'kind': 'Secret', 'immutable': True, 'type': 'Opaque',
                'metadata': {'name': name, 'namespace': 'voice-dev'},
                'data': {'PGPASSWORD': base64.b64encode(credential['password'].encode('utf-8')).decode('ascii')}}
    except BaseException:
        raise MigrationError('migration credential snapshot unavailable') from None


def _check_secret(intended, actual, *, require_identity=True):
    try:
        meta = actual['metadata']
        if (not isinstance(meta, dict) or set(meta) - {'name', 'namespace', 'uid', 'resourceVersion',
                                                       'creationTimestamp', 'managedFields', 'generation'} or
                actual.get('apiVersion') != 'v1' or actual.get('kind') != 'Secret' or actual.get('immutable') is not True or
                actual.get('type') != 'Opaque' or actual.get('data') != intended['data'] or
                meta.get('name') != intended['metadata']['name'] or meta.get('namespace') != 'voice-dev' or
                require_identity and any(not isinstance(meta.get(k), str) or not meta[k]
                                         for k in ('uid', 'resourceVersion'))):
            raise ValueError()
        return {k: meta[k] for k in ('name', 'uid', 'resourceVersion')} if require_identity else None
    except BaseException:
        raise MigrationError('migration credential snapshot changed') from None


def _private_kube(args, body=None):
    """Fixed kubectl transport; Secret bodies and diagnostics stay in memory."""
    process = selector = None
    try:
        raw = b'' if body is None else json.dumps(body, separators=(',', ':')).encode()
        if len(raw) > 4096:
            raise ValueError()
        process = subprocess.Popen(frontend_runtime_executor.KubectlTransport.PREFIX + list(args),
                                   stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        if raw: process.stdin.write(raw)
        process.stdin.close()
        selector = selectors.DefaultSelector(); selector.register(process.stdout, selectors.EVENT_READ)
        deadline = time.monotonic() + 25; chunks = []; size = 0
        while selector.get_map():
            left = deadline - time.monotonic()
            if left <= 0 or not selector.select(left): raise ValueError()
            chunk = os.read(process.stdout.fileno(), min(8192, 65537 - size))
            if not chunk:
                selector.unregister(process.stdout); break
            size += len(chunk)
            if size > 65536: raise ValueError()
            chunks.append(chunk)
        if process.wait(timeout=max(.01, deadline - time.monotonic())): raise ValueError()
        raw_output = b''.join(chunks)
        if not raw_output.strip():
            if (len(args) != 6 or list(args[:2]) != ['get', 'secret'] or
                    not re.fullmatch(r'voice-go-input-[a-z]+-[a-f0-9]{32}', args[2]) or
                    list(args[3:]) != ['--ignore-not-found=true', '-o', 'json']):
                raise ValueError()
            return {}
        return json.loads(raw_output)
    except BaseException:
        raise MigrationError('private Kubernetes request unavailable') from None
    finally:
        if selector is not None: selector.close()
        if process is not None:
            if process.poll() is None: process.kill(); process.wait()
            if process.stdout is not None: process.stdout.close()


class SecretSnapshot:
    """One create-only Secret snapshot; not a migration execution admission."""
    def __init__(self, journal, fence, client, lock, namespace_uid, metadata_phase, cohort_phase, cut, token, database):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.metadata_phase, self.cohort, self.cut, self.token, self.database = metadata_phase, cohort_phase, cut, token, database
        self.intent = copy.deepcopy(journal.intent); self.poisoned = False
        self._bound = None
        self._dependencies = (journal, fence, client, metadata_phase, cohort_phase, cut)
        self._fixed = (token, database, copy.deepcopy(lock), namespace_uid, copy.deepcopy(fence.context))
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
                type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                fence.ledger.client is not client or self.intent.get('resource_context') != fence.context or
                self.intent.get('namespace_uid') != namespace_uid or database not in source_plan.DATABASES or
                not isinstance(token, str) or not token):
            raise MigrationError('migration credential phase identity invalid')

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or
                (self.token, self.database, self.lock, self.namespace_uid, self.fence.context) != self._fixed or
                any(a is not b for a, b in zip((self.journal, self.fence, self.client, self.metadata_phase,
                                                self.cohort, self.cut), self._dependencies))):
            raise MigrationError('migration credential phase changed')
        schema._fence(lambda: media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid))

    def _ledger(self):
        row, revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or not isinstance(revision, tuple) or
                len(revision) != 2 or any(not isinstance(v, str) or not v for v in revision)):
            raise MigrationError('migration credential ledger changed')
        return row, revision

    def create(self):
        """Create once. Uncertain responses retain ownership for reconciliation."""
        pending = False
        try:
            self._guard(); proof = qualified_plan(self.metadata_phase, self.cohort, self.cut, self.token, self.database)
            source = proof['source']; secret = _credential(self.database, self._guard)
            safe = _login(self.database, secret, source, self._guard)
            clone = cohort_restore_journal.verify_go_role(self.cohort, self.metadata_phase, self.token, self.database)
            _check_clone(clone, self.database, safe['role'], proof['plan'], self.fence.context['commit'])
            if (safe != proof['credential'] or _credential(self.database, self._guard) != secret or
                    clone.get('source_sha') != self.fence.context['commit']):
                raise ValueError()
            intended = _secret(self.fence.context['operation_id'], self.database, secret, source['engine']['service_ip'])
            self._guard()
            dry = _private_kube(['create', '--dry-run=server', '-f', '-', '-o', 'json'], intended)
            _check_secret(intended, dry, require_identity=False)
            self.cut.verify_drained(); self._guard(); row, revision = self._ledger()
            local_key = 'go-migration-secret-' + self.database
            if local_key in self.journal.state['steps'] or self.database in row.get('go_migration_secrets', {}):
                raise ValueError()
            marker = {'status': 'pending', 'nonce': os.urandom(16).hex(), 'operation_id': self.fence.context['operation_id'],
                      'source_sha': self.fence.context['commit'], 'database': self.database,
                      'name': intended['metadata']['name'], 'metadata_receipt_sha256': proof['metadata_receipt_sha256'],
                      'credential': copy.deepcopy(safe), 'clone_receipt_sha256': _hash(clone)}
            pending = True; self.journal.state['steps'][local_key] = copy.deepcopy(marker); self.journal.save()
            self._guard(); current, rev = self._ledger()
            if current != row or rev != revision or self.journal.state['steps'].get(local_key) != marker:
                raise ValueError()
            current.setdefault('go_migration_secrets', {})[self.database] = copy.deepcopy(marker)
            current['step'] = local_key + '-pending'; self.fence.ledger.write(rev, current)
            self._guard()
            if self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != marker or \
                    _credential(self.database, self._guard) != secret or \
                    qualified_plan(self.metadata_phase, self.cohort, self.cut, self.token, self.database) != proof or \
                    cohort_restore_journal.verify_go_role(self.cohort, self.metadata_phase, self.token, self.database) != clone:
                raise ValueError()
            self.cut.verify_drained()
            created = _private_kube(['create', '-f', '-', '-o', 'json'], intended)
            receipt = _check_secret(intended, created)
            self._guard()
            actual = _private_kube(['get', 'secret', intended['metadata']['name'], '-o', 'json'])
            if _check_secret(intended, actual) != receipt or _credential(self.database, self._guard) != secret:
                raise ValueError()
            if _login(self.database, secret, source, self._guard) != safe:
                raise ValueError()
            self.cut.verify_drained(); self._guard(); row, revision = self._ledger()
            if row.get('go_migration_secrets', {}).get(self.database) != marker or self.journal.state['steps'].get(local_key) != marker:
                raise ValueError()
            done = marker | {'status': 'done', 'receipt': receipt}
            self.journal.state['steps'][local_key] = copy.deepcopy(done); self.journal.save()
            row['go_migration_secrets'][self.database] = copy.deepcopy(done)
            row['step'] = local_key + '-done'; self.fence.ledger.write(revision, row)
            self._guard()
            if self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != done or self.journal.state['steps'].get(local_key) != done:
                raise ValueError()
            self._bound = (copy.deepcopy(done), copy.deepcopy(secret), copy.deepcopy(source))
            return copy.deepcopy(receipt)
        except schema.GuardLost:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('Go migration secret outcome uncertain')
                except BaseException: pass
            raise
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('Go migration secret outcome uncertain')
                except BaseException: pass
            raise MigrationError('migration credential snapshot uncertain' if pending else 'migration credential snapshot not admitted') from None

    def verify(self):
        """Recheck the same owned Secret and source refs; never adopt by name."""
        try:
            self._guard()
            if self._bound is None: raise ValueError()
            done, credential, source = self._bound
            local_key = 'go-migration-secret-' + self.database
            if (self.journal.state['steps'].get(local_key) != done or
                    self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != done or
                    _credential(self.database, self._guard) != credential):
                raise ValueError()
            snapshot = schema.kube_engine_snapshot(schema.linux_private_bytes, self._guard)
            if snapshot() != source['engine']: raise ValueError()
            intended = _secret(self.fence.context['operation_id'], self.database, credential, source['engine']['service_ip'])
            actual = _private_kube(['get', 'secret', intended['metadata']['name'], '-o', 'json'])
            if _check_secret(intended, actual) != done['receipt'] or snapshot() != source['engine']:
                raise ValueError()
            self._guard()
            if (self.journal.state['steps'].get(local_key) != done or
                    self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != done or
                    _credential(self.database, self._guard) != credential):
                raise ValueError()
            return copy.deepcopy(done['receipt'])
        except schema.GuardLost:
            raise
        except BaseException:
            raise MigrationError('migration credential snapshot unavailable') from None

    def remove(self):
        """Delete the exact created Secret only after the caller releases its CUT binding."""
        started = False
        try:
            receipt = self.verify()
            if getattr(self.cut, 'migration', None) is not None:
                raise ValueError()
            done, _, _ = self._bound; key = 'go-migration-secret-' + self.database
            row, revision = self._ledger()
            if row.get('go_migration_secrets', {}).get(self.database) != done or self.journal.state['steps'].get(key) != done:
                raise ValueError()
            removing = done | {'status': 'removing'}
            started = True; self.journal.state['steps'][key] = copy.deepcopy(removing); self.journal.save()
            self._guard(); current, rev = self._ledger()
            if current != row or rev != revision: raise ValueError()
            current['go_migration_secrets'][self.database] = copy.deepcopy(removing)
            current['step'] = key + '-removing'; self.fence.ledger.write(rev, current)
            self._guard()
            if self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != removing: raise ValueError()
            options = {'apiVersion': 'v1', 'kind': 'DeleteOptions', 'propagationPolicy': 'Foreground',
                       'preconditions': {'uid': receipt['uid'], 'resourceVersion': receipt['resourceVersion']}}
            _private_kube(['delete', '--raw=/api/v1/namespaces/voice-dev/secrets/' + receipt['name'], '-f', '-'], options)
            deadline = time.monotonic() + 60
            while True:
                self._guard()
                remaining = _private_kube(['get', 'secret', receipt['name'], '--ignore-not-found=true', '-o', 'json'])
                if not remaining: break
                if remaining.get('metadata', {}).get('uid') != receipt['uid'] or time.monotonic() >= deadline:
                    raise ValueError()
                time.sleep(1)
            self._guard(); row, rev = self._ledger()
            if row.get('go_migration_secrets', {}).get(self.database) != removing or self.journal.state['steps'].get(key) != removing:
                raise ValueError()
            removed = done | {'status': 'removed'}
            self.journal.state['steps'][key] = copy.deepcopy(removed); self.journal.save()
            self._guard(); current, final_rev = self._ledger()
            if current != row or final_rev != rev: raise ValueError()
            current['go_migration_secrets'][self.database] = copy.deepcopy(removed)
            current['step'] = key + '-removed'; self.fence.ledger.write(final_rev, current)
            self._guard()
            if self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != removed:
                raise ValueError()
            self._bound = None
            return copy.deepcopy(receipt)
        except BaseException:
            if started:
                self.poisoned = True
                try: self.fence.uncertain('Go migration secret cleanup uncertain')
                except BaseException: pass
            raise MigrationError('migration credential cleanup uncertain' if started else 'migration credential cleanup not admitted') from None


def verify_writer_job(authored, actual, *, dry=False):
    """Apply the existing exact Kubernetes Job admission checks."""
    try:
        meta = actual['metadata']
        allowed = {'name', 'namespace', 'uid', 'resourceVersion', 'creationTimestamp', 'managedFields',
                   'generation', 'annotations', 'labels'}
        if (not isinstance(meta, dict) or set(meta) - allowed or
                meta.get('annotations') != authored['metadata']['annotations']):
            raise ValueError()
        if dry:
            if (meta.get('name') != authored['metadata']['name'] or meta.get('namespace') != 'voice-dev' or
                    not isinstance(meta.get('uid'), str) or not meta['uid']):
                raise ValueError()
            return realtime_migration_job.admitted_job(authored, actual)
        return realtime_migration_job.verify_writer_job(authored, actual)
    except BaseException:
        raise MigrationError('migration Job identity unavailable') from None


def verify_writer_configmap(authored, actual):
    receipt = _config_receipt(authored, actual)
    return {'uid': receipt['uid'], 'resourceVersion': receipt['resourceVersion']}


def verify_writer_pod(authored, job, pod, execution):
    """Bind the Job-owned Pod, platform image and IP to the CUT observer."""
    try:
        accepted = verify_writer_job(authored, job)
        fields = {'database', 'database_oid', 'role', 'application_name', 'container_name',
                  'node_name', 'node_uid', 'platform_digest'}
        if (not isinstance(execution, dict) or set(execution) != fields or
                execution['database'] not in source_plan.DATABASES or
                type(execution['database_oid']) is not int or execution['database_oid'] <= 0 or
                not isinstance(execution['role'], str) or not schema.SAFE.fullmatch(execution['role']) or
                execution['application_name'] != authored['metadata']['name'] or execution['container_name'] != 'migrate' or
                not isinstance(execution['node_uid'], str) or not execution['node_uid'] or
                execution['platform_digest'] != realtime_migration_job.PLATFORMS['amd64']):
            raise ValueError()
        meta, spec, status = pod['metadata'], copy.deepcopy(pod['spec']), pod['status']
        owners = [o for o in meta.get('ownerReferences', []) if o.get('controller') is True]
        phase = status.get('phase'); node_name = spec.pop('nodeName', None)
        if (pod.get('apiVersion') != 'v1' or pod.get('kind') != 'Pod' or meta.get('namespace') != 'voice-dev' or
                any(not isinstance(meta.get(k), str) or not meta[k] for k in ('uid', 'resourceVersion')) or
                meta.get('deletionTimestamp') or len(owners) != 1 or owners[0].get('kind') != 'Job' or
                owners[0].get('uid') != job['metadata']['uid'] or
                (node_name not in (None, execution['node_name']) if phase == 'Pending' else node_name != execution['node_name']) or
                spec.get('hostNetwork', False) is not False):
            raise ValueError()
        name, uid = authored['metadata']['name'], job['metadata']['uid']
        labels = copy.deepcopy(meta.get('labels', {}))
        if labels != accepted['template']['metadata']['labels']: raise ValueError()
        generated = {'batch.kubernetes.io/controller-uid': uid, 'controller-uid': uid,
                     'batch.kubernetes.io/job-name': name, 'job-name': name}
        for key, value in generated.items():
            if key in labels and labels.pop(key) != value: raise ValueError()
        template = authored['spec']['template']
        if labels != template['metadata']['labels']: raise ValueError()
        intended = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': 'template', 'namespace': 'voice-dev',
                    'labels': template['metadata']['labels']}, 'spec': template['spec']}
        observed = copy.deepcopy(intended); observed['spec'] = spec
        auth_user_candidate_probe.admitted_pod(intended, observed)
        ip, containers = status.get('podIP'), status.get('containerStatuses', [])
        if phase == 'Pending':
            if (not isinstance(containers, list) or len(containers) > 1 or
                    containers and (containers[0].get('name') != 'migrate' or
                                    type(containers[0].get('restartCount')) is not int or containers[0]['restartCount'] != 0 or
                                    containers[0].get('containerID') or containers[0].get('state', {}).get('running') or
                                    containers[0].get('state', {}).get('terminated'))):
                raise ValueError()
            return {'uid': meta['uid'], 'ip': None}
        image = containers[0].get('imageID', '') if len(containers) == 1 else ''
        image_match = re.fullmatch(r'(?:docker-pullable://[^@]+@|containerd://|docker://)?(sha256:[0-9a-f]{64})', image)
        if (status.get('phase') not in ('Running', 'Succeeded') or not isinstance(ip, str) or
                str(ipaddress.ip_address(ip)) != ip or len(containers) != 1 or containers[0].get('name') != 'migrate' or
                not containers[0].get('containerID') or type(containers[0].get('restartCount')) is not int or
                containers[0]['restartCount'] != 0 or not image_match or image_match.group(1) != execution['platform_digest']):
            raise ValueError()
        return {'uid': meta['uid'], 'ip': ip}
    except BaseException:
        raise MigrationError('migration Pod identity unavailable') from None


def _hash(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()).hexdigest()


def _config_receipt(intended, actual, *, require_identity=True):
    try:
        meta = actual['metadata']
        allowed = {'name', 'namespace', 'uid', 'resourceVersion', 'creationTimestamp', 'managedFields', 'generation', 'annotations'}
        if (not isinstance(meta, dict) or set(meta) - allowed or actual.get('apiVersion') != 'v1' or
                actual.get('kind') != 'ConfigMap' or actual.get('immutable') is not True or
                actual.get('data') != intended['data'] or actual.get('binaryData') not in (None, {}) or
                meta.get('name') != intended['metadata']['name'] or
                meta.get('namespace') != 'voice-dev' or meta.get('annotations') != intended['metadata']['annotations'] or
                require_identity and any(not isinstance(meta.get(k), str) or not meta[k] for k in ('uid', 'resourceVersion'))):
            raise ValueError()
        if not require_identity: return None
        return {'name': meta['name'], 'uid': meta['uid'], 'resourceVersion': meta['resourceVersion'],
                'data_sha256': _hash(intended['data'])}
    except BaseException:
        raise MigrationError('migration ConfigMap identity unavailable') from None


class Migration:
    """One exact Go migration under the established shared WriterCut."""
    def __init__(self, secret):
        if type(secret) is not SecretSnapshot:
            raise MigrationError('migration credential phase invalid')
        self.secret = secret
        self.journal, self.fence, self.client = secret.journal, secret.fence, secret.client
        self.lock, self.namespace_uid = copy.deepcopy(secret.lock), secret.namespace_uid
        self.metadata_phase, self.cohort, self.cut = secret.metadata_phase, secret.cohort, secret.cut
        self.token, self.database, self.root = secret.token, secret.database, secret.metadata_phase.source_root
        self.invocation = secrets.token_hex(16)
        self.marker = self.input = None
        self.poisoned = False

    def _guard(self):
        if (self.poisoned or self.secret.poisoned or self.secret.journal is not self.journal or
                self.secret.fence is not self.fence or self.secret.client is not self.client or
                self.secret.cut is not self.cut or self.secret.metadata_phase is not self.metadata_phase or
                self.secret.cohort is not self.cohort or self.secret.token != self.token or
                self.secret.database != self.database or self.secret.lock != self.lock or
                self.secret.namespace_uid != self.namespace_uid or self.metadata_phase.source_root != self.root):
            raise MigrationError('migration phase identity changed')
        if self.secret._bound is None:
            verify_completed(self)
        else:
            self.secret.verify()
        if self.marker is not None:
            row, _ = self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status') != 'active' or
                    self.marker.get('nonce') != self.invocation or self.marker.get('input_sha256') != self._input_hash() or
                    row.get('go_migrations', {}).get(self.database) != self.marker or
                    self.journal.state['steps'].get('go-migration-' + self.database) != self.marker):
                raise MigrationError('migration journal changed')

    def _update(self, old, new):
        self._guard(); row, revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or
                row.get('go_migrations', {}).get(self.database) != old):
            raise MigrationError('migration reservation changed')
        key = 'go-migration-' + self.database
        if self.journal.state['steps'].get(key) != old: raise MigrationError('migration journal changed')
        self.journal.state['steps'][key] = copy.deepcopy(new); self.journal.save()
        self.secret._guard()
        current, rev = self.fence.ledger.read()
        if current != row or rev != revision: raise MigrationError('migration ledger changed')
        current.setdefault('go_migrations', {})[self.database] = copy.deepcopy(new)
        current['step'] = key + '-' + new['status']; self.fence.ledger.write(rev, current)
        self.marker = copy.deepcopy(new); self._guard()

    def _step(self, name, state, receipt=None):
        old = copy.deepcopy(self.marker); new = copy.deepcopy(old)
        new['steps'][name] = copy.deepcopy(state)
        if receipt is not None:
            new['steps'][name]['receipt'] = copy.deepcopy(receipt)
        self._update(old, new)

    def _objects(self):
        if self.input is None: raise MigrationError('migration source unavailable')
        return render(self.fence.context['operation_id'], self.fence.context['commit'], self.database,
                      self.input['files'], self.input['node'], self.input['route'])

    def _input_hash(self):
        if self.input is None: raise MigrationError('migration source unavailable')
        value = {k:copy.deepcopy(v) for k,v in self.input.items() if k != 'files'}
        value['files'] = {name:hashlib.sha256(raw).hexdigest() for name,raw in self.input['files'].items()}
        return _hash(value)

    def run(self):
        """Run exactly once; any uncertain effect retains the shared fence."""
        pending = False
        try:
            self._guard()
            proof = qualified_plan(self.metadata_phase, self.cohort, self.cut, self.token, self.database)
            clone = cohort_restore_journal.verify_go_role(self.cohort, self.metadata_phase, self.token, self.database)
            safe = proof['credential']
            _check_clone(clone, self.database, safe['role'], proof['plan'], self.fence.context['commit'])
            secret_done, credential, source = self.secret._bound
            if (secret_done['clone_receipt_sha256'] != _hash(clone) or source != proof['source'] or
                    {k:v for k,v in credential.items() if k != 'password'} != safe or
                    _credential(self.database, self._guard) != credential or _login(self.database, credential, source, self._guard) != safe):
                raise ValueError()
            observed = metadata.verify_completed(self.metadata_phase, self.token)
            plans = observed['metadata']['migrations']
            files = restore.migration_inventory(self.root, observed['databases'], plans, full_catalog=True)[self.database]
            original = observed['metadata']['catalog'][self.database]
            engine_snapshot = schema.kube_engine_snapshot(schema.linux_private_bytes, self._guard)
            role = self.metadata_phase.inventory._admin_role()
            actual = self.metadata_phase._read(self.database, source['database_oid'], source['engine'], source['server_start'], role, engine_snapshot)
            if actual != original or actual.get('history') != [{'version': proof['plan']['from'], 'dirty': False}]:
                raise ValueError()
            package = self.metadata_phase.package.verify_current(self.token)
            nodes = sorted((n for n in package['nodes'] if n.get('architecture') == 'amd64'), key=lambda n:n['name'])
            if not nodes: raise ValueError()
            node = nodes[0]
            route = {'host': safe['host'], 'service_ip': source['engine']['service_ip'],
                     'role': safe['role'], 'sslmode': safe['sslmode']}
            execution = {'database': self.database, 'database_oid': source['database_oid'], 'role': safe['role'],
                         'application_name': 'voice-go-migrate-' + self.database.removesuffix('_db') + '-' + self.fence.context['operation_id'][-32:],
                         'container_name': 'migrate', 'node_name': node['name'], 'node_uid': node['uid'],
                         'platform_digest': realtime_migration_job.PLATFORMS['amd64']}
            self.input = {'files': files, 'node': node, 'route': route, 'execution': execution,
                          'proof': proof, 'clone': clone, 'secret': secret_done['receipt']}
            config, job = self._objects()
            self.cut.verify_drained(); self._guard()
            for kind, obj in (('configmaps', config), ('jobs', job)):
                entries = self.client.call('get', kind, '-o', 'json')['items']
                if any(item.get('metadata', {}).get('name') == obj['metadata']['name'] for item in entries):
                    raise ValueError()
            row, _ = self.fence.ledger.read()
            key = 'go-migration-' + self.database
            if key in self.journal.state['steps'] or self.database in row.get('go_migrations', {}):
                raise ValueError()
            marker = {'status': 'active', 'operation_id': self.fence.context['operation_id'],
                      'source_sha': self.fence.context['commit'], 'nonce': self.invocation,
                      'input_sha256': self._input_hash(), 'steps': {}}
            pending = True; self._update(None, marker)
            config_step = {'status': 'pending', 'nonce': secrets.token_hex(16), 'details_sha256': _hash(config)}
            self._guard(); dry_config = self.client.call('create', '--dry-run=server', '-f', '-', '-o', 'json', body=config)
            _config_receipt(config, dry_config, require_identity=False)
            self._step('config', config_step); self._guard(); self.cut.verify_drained()
            created = self.client.call('create', '-f', '-', '-o', 'json', body=config)
            config_receipt = _config_receipt(config, created)
            fetched = self.client.call('get', 'configmap', config_receipt['name'], '-o', 'json')
            if _config_receipt(config, fetched) != config_receipt: raise ValueError()
            self._step('config', config_step | {'status': 'done'}, config_receipt)
            self._guard()
            dry = self.client.call('create', '--dry-run=server', '-f', '-', '-o', 'json', body=job)
            dry_spec = verify_writer_job(job, dry, dry=True)
            template = dry_spec['template']
            dry_pod = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': 'voice-go-check-' + self.fence.context['operation_id'][-32:],
                       'namespace': 'voice-dev', 'labels': copy.deepcopy(template['metadata']['labels']),
                       'ownerReferences': [{'apiVersion': 'batch/v1', 'kind': 'Job', 'name': job['metadata']['name'],
                                             'uid': dry['metadata']['uid'], 'controller': True, 'blockOwnerDeletion': False}]},
                       'spec': copy.deepcopy(template['spec'])}
            self._guard(); admitted = self.client.call('create', '--dry-run=server', '-f', '-', '-o', 'json', body=dry_pod)
            auth_user_candidate_probe.admitted_pod(dry_pod, admitted)
            if admitted.get('metadata', {}).get('ownerReferences') != dry_pod['metadata']['ownerReferences']:
                raise ValueError()
            self._guard(); self.cut.bind_migration('go', self, self.root)
            job_step = {'status': 'pending', 'nonce': secrets.token_hex(16), 'details_sha256': _hash(job)}
            self._step('job', job_step)
            self._guard(); self.cut.verify_migration_pending()
            if (cohort_restore_journal.verify_go_role(self.cohort, self.metadata_phase, self.token, self.database) != clone or
                    self.secret.verify() != secret_done['receipt']):
                raise ValueError()
            self.cut.verify_migration_pending(); self._guard()
            submitted = self.client.call('create', '-f', '-', '-o', 'json', body=job)
            accepted = verify_writer_job(job, submitted)
            jm = submitted['metadata']; uid, rv = jm.get('uid'), jm.get('resourceVersion')
            if any(not isinstance(x, str) or not x for x in (uid, rv)) or jm.get('annotations', {}).get('voice.io/source-sha') != self.fence.context['commit']:
                raise ValueError()
            job_step = job_step | {'status': 'owned', 'uid': uid, 'resourceVersion': rv}
            self._step('job', job_step); deadline = time.monotonic() + 650
            while time.monotonic() < deadline:
                self._guard(); self.cut.verify_migration_running()
                current = self.client.call('get', 'job', jm['name'], '-o', 'json')
                if current.get('metadata', {}).get('uid') != uid or verify_writer_job(job, current) != accepted:
                    raise ValueError()
                conditions = current.get('status', {}).get('conditions', [])
                if any(c.get('type') == 'Failed' and c.get('status') == 'True' for c in conditions):
                    raise ValueError()
                if any(c.get('type') == 'Complete' and c.get('status') == 'True' for c in conditions):
                    if current.get('status', {}).get('active', 0) or current.get('status', {}).get('failed', 0): raise ValueError()
                    pods = self.client.call('get', 'pods', '-o', 'json')['items']
                    owned = [p for p in pods if any(o.get('controller') is True and o.get('kind') == 'Job' and o.get('uid') == uid
                                                  for o in p.get('metadata', {}).get('ownerReferences', []))]
                    if len(owned) != 1: raise ValueError()
                    pod = owned[0]; identity = verify_writer_pod(job, current, pod, execution)
                    statuses = pod['status']['containerStatuses']
                    if pod['status'].get('phase') != 'Succeeded' or statuses[0].get('state', {}).get('terminated', {}).get('exitCode') != 0:
                        raise ValueError()
                    break
                time.sleep(2)
            else:
                raise ValueError()
            self._guard(); actual = self.metadata_phase._read(self.database, source['database_oid'], source['engine'],
                                                               source['server_start'], role, engine_snapshot)
            if actual.get('history') != [{'version': proof['plan']['to'], 'dirty': False}]: raise ValueError()
            self.cut.verify_migration_running(); self._guard()
            final_job = self.client.call('get', 'job', jm['name'], '-o', 'json')
            final_pods = self.client.call('get', 'pods', '-o', 'json')['items']
            final_owned = [p for p in final_pods if any(o.get('controller') is True and o.get('kind') == 'Job' and o.get('uid') == uid
                                                       for o in p.get('metadata', {}).get('ownerReferences', []))]
            if final_job != current or final_owned != [pod]: raise ValueError()
            receipt = {'job_uid': uid, 'pod_uid': identity['uid'], 'container_id': statuses[0]['containerID'],
                       'image_id': statuses[0]['imageID'], 'database_oid': source['database_oid'],
                       'from': proof['plan']['from'], 'to': proof['plan']['to'], 'dirty': False}
            self._step('job', job_step | {'status': 'done'}, receipt)
            self.cut.finish_migration(); self._guard()
            old = copy.deepcopy(self.marker); self._update(old, old | {'status': 'done'})
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('Go migration outcome uncertain')
                except BaseException: pass
            raise MigrationError('Go migration uncertain; retain fence' if pending else 'Go migration not admitted') from None

    def cleanup(self):
        """Remove only terminal owned objects, with UID/RV preconditions."""
        started = False
        try:
            verify_writer_terminal(self, self.root)
            for kind in ('job', 'configmap'):
                self.cut.verify_migration_running(); self._guard()
                config, job = self._objects(); intended = job if kind == 'job' else config
                step_name = 'job' if kind == 'job' else 'config'
                previous = copy.deepcopy(self.marker['steps'][step_name])
                if previous['status'] != 'done': raise ValueError()
                actual = self.client.call('get', kind, intended['metadata']['name'], '-o', 'json')
                if kind == 'job':
                    verify_writer_job(intended, actual)
                    uid, rv = actual['metadata']['uid'], actual['metadata']['resourceVersion']
                    if uid != previous['uid']: raise ValueError()
                else:
                    identity = _config_receipt(intended, actual)
                    uid, rv = identity['uid'], identity['resourceVersion']
                    if identity != previous['receipt']: raise ValueError()
                started = True
                self._step(step_name, previous | {'status': 'removing'})
                self._guard()
                prefix = '/apis/batch/v1/namespaces/voice-dev/jobs/' if kind == 'job' else '/api/v1/namespaces/voice-dev/configmaps/'
                options = {'apiVersion': 'v1', 'kind': 'DeleteOptions', 'propagationPolicy': 'Foreground',
                           'preconditions': {'uid': uid, 'resourceVersion': rv}}
                self.client.call('delete', '--raw=' + prefix + intended['metadata']['name'], '-f', '-', body=options)
                deadline = time.monotonic() + 60
                while True:
                    self._guard()
                    remaining = self.client.call('get', kind, intended['metadata']['name'], '--ignore-not-found=true', '-o', 'json')
                    pods = []
                    if kind == 'job':
                        rows = self.client.call('get', 'pods', '-o', 'json')['items']
                        pods = [p for p in rows if any(o.get('controller') is True and o.get('kind') == 'Job' and o.get('uid') == uid
                                                       for o in p.get('metadata', {}).get('ownerReferences', []))]
                    if not remaining and not pods: break
                    if remaining and remaining.get('metadata', {}).get('uid') != uid or time.monotonic() >= deadline:
                        raise ValueError()
                    time.sleep(1)
                self._step(step_name, previous | {'status': 'removed'})
            verify_writer_cleanup(self, self.root)
            self.cut.accept_migration_cleanup()
            return self.secret.remove()
        except BaseException:
            if started:
                self.poisoned = True
                try: self.fence.uncertain('Go migration cleanup outcome uncertain')
                except BaseException: pass
            raise MigrationError('Go cleanup uncertain; retain fence' if started else 'Go cleanup not admitted') from None


def verify_writer_successor(phase, source_root):
    """Reconstruct the owned ConfigMap and Job for the shared CUT observer."""
    try:
        if type(phase) is not Migration or Path(source_root) != phase.root:
            raise ValueError()
        phase._guard(); marker = copy.deepcopy(phase.marker)
        if (not isinstance(marker, dict) or marker.get('status') not in ('active', 'done') or
                marker.get('operation_id') != phase.fence.context['operation_id'] or
                marker.get('source_sha') != phase.fence.context['commit'] or marker.get('nonce') != phase.invocation or
                marker.get('input_sha256') != phase._input_hash() or not isinstance(marker.get('steps'), dict)):
            raise ValueError()
        config, job = phase._objects()
        cm_step = marker['steps']['config']
        if (set(cm_step) != {'status', 'nonce', 'details_sha256', 'receipt'} or cm_step['status'] not in ('done', 'removing', 'removed') or
                not isinstance(cm_step['nonce'], str) or not re.fullmatch(r'[a-f0-9]{32}', cm_step['nonce']) or
                cm_step['details_sha256'] != _hash(config)):
            raise ValueError()
        cm_receipt = cm_step['receipt']
        if cm_step['status'] == 'done':
            actual = phase.client.call('get', 'configmap', config['metadata']['name'], '-o', 'json')
            if _config_receipt(config, actual) != cm_receipt: raise ValueError()
        item = marker['steps'].get('job')
        state = 'absent' if item is None else item.get('status')
        if state not in ('absent', 'pending', 'owned', 'done', 'removing', 'removed'): raise ValueError()
        uid = rv = job_nonce = terminal = None
        if item is not None:
            expected = {'status', 'nonce', 'details_sha256'}
            if state in ('owned', 'done', 'removing', 'removed'): expected |= {'uid', 'resourceVersion'}
            if state in ('done', 'removing', 'removed'): expected.add('receipt')
            if (set(item) != expected or item['details_sha256'] != _hash(job) or
                    not re.fullmatch(r'[a-f0-9]{32}', item['nonce'])):
                raise ValueError()
            job_nonce = item['nonce']
            if state in ('owned', 'done', 'removing', 'removed'):
                uid, rv = item['uid'], item['resourceVersion']
                if any(not isinstance(x, str) or not x for x in (uid, rv)): raise ValueError()
            if state in ('done', 'removing', 'removed'):
                if item['receipt'].get('job_uid') != uid: raise ValueError()
                terminal = _hash(item['receipt'])
        execution = phase.input['execution']
        phase._guard()
        if phase.marker != marker: raise ValueError()
        return {'configmap': {'authored': config, 'state': 'owned' if cm_step['status'] == 'done' else cm_step['status'],
                              'uid': cm_receipt['uid'], 'resourceVersion': cm_receipt['resourceVersion']},
                'job': {'authored': job, 'state': state, 'uid': uid, 'resourceVersion': rv},
                'record': {'operation_id': marker['operation_id'], 'source_sha': marker['source_sha'],
                           'nonce': marker['nonce'], 'job_nonce': job_nonce, 'terminal_sha256': terminal},
                'execution': copy.deepcopy(execution)}
    except BaseException:
        raise MigrationError('writer migration successor unavailable') from None


def _terminal_receipt(phase, marker):
        step = marker['steps']['job']; receipt = step['receipt']; proof = phase.input['proof']
        if (step['status'] not in ('done', 'removing', 'removed') or not isinstance(receipt, dict) or
                set(receipt) != {'job_uid', 'pod_uid', 'container_id', 'image_id', 'database_oid', 'from', 'to', 'dirty'} or
                receipt['job_uid'] != step['uid'] or receipt['database_oid'] != proof['source']['database_oid'] or
                receipt['from'] != proof['plan']['from'] or receipt['to'] != proof['plan']['to'] or receipt['dirty'] is not False or
                any(not isinstance(receipt[k], str) or not receipt[k] for k in ('pod_uid', 'container_id', 'image_id')) or
                not receipt['image_id'].endswith(phase.input['execution']['platform_digest'])):
            raise ValueError()
        return copy.deepcopy(receipt)


def verify_writer_terminal(phase, source_root):
    try:
        successor = verify_writer_successor(phase, source_root)
        receipt = _terminal_receipt(phase, phase.marker)
        if successor['record']['terminal_sha256'] != _hash(receipt): raise ValueError()
        return receipt
    except BaseException:
        raise MigrationError('migration terminal receipt unavailable') from None


def verify_writer_cleanup(phase, source_root):
    try:
        verify_writer_terminal(phase, source_root)
        successor = verify_writer_successor(phase, source_root)
        if successor['job']['state'] != 'removed' or successor['configmap']['state'] != 'removed':
            raise ValueError()
        return {'job': successor['job'], 'configmap': successor['configmap']}
    except BaseException:
        raise MigrationError('migration cleanup receipt unavailable') from None


def verify_completed(phase):
    """Verify the historical terminal record after all owned objects were removed."""
    try:
        if type(phase) is not Migration or phase.poisoned or phase.secret.poisoned or phase.secret._bound is not None:
            raise ValueError()
        phase.secret._guard(); row, _ = phase.secret._ledger(); marker = phase.marker
        key = 'go-migration-' + phase.database; secret_key = 'go-migration-secret-' + phase.database
        secret = phase.journal.state['steps'][secret_key]
        if (not isinstance(marker, dict) or marker.get('status') != 'done' or marker.get('nonce') != phase.invocation or
                marker.get('input_sha256') != phase._input_hash() or row.get('go_migrations', {}).get(phase.database) != marker or
                phase.journal.state['steps'].get(key) != marker or marker['steps']['job']['status'] != 'removed' or
                marker['steps']['config']['status'] != 'removed' or secret.get('status') != 'removed' or
                row.get('go_migration_secrets', {}).get(phase.database) != secret or secret['receipt'] != phase.input['secret'] or
                secret['clone_receipt_sha256'] != _hash(phase.input['clone'])):
            raise ValueError()
        receipt = _terminal_receipt(phase, marker)
        phase.secret._guard()
        if phase.secret._ledger()[0].get('go_migrations', {}).get(phase.database) != marker:
            raise ValueError()
        return receipt
    except BaseException:
        raise MigrationError('completed Go migration unavailable') from None


def render(operation, source_sha, database, files, node, route):
    """Render closed fixed objects from inputs authenticated by the phase."""
    try:
        if (not isinstance(operation, str) or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}', operation) or
                not isinstance(source_sha, str) or not re.fullmatch(r'[a-f0-9]{40}', source_sha) or
                database not in source_plan.DATABASES or not isinstance(node, dict) or
                set(node) != {'name', 'uid', 'hostname', 'architecture'} or node['architecture'] != 'amd64' or
                any(not isinstance(node[k], str) or not node[k] for k in ('name', 'uid', 'hostname')) or
                not isinstance(files, dict) or not files):
            raise ValueError()
        service = database.removesuffix('_db')
        if (not isinstance(route, dict) or set(route) != {'host', 'service_ip', 'role', 'sslmode'} or
                route['host'] not in ('voice-postgres', 'voice-postgres.voice-dev', 'voice-postgres.voice-dev.svc',
                                      'voice-postgres.voice-dev.svc.cluster.local') or
                not isinstance(route['service_ip'], str) or
                str(ipaddress.IPv4Address(route['service_ip'])) != route['service_ip'] or
                not isinstance(route['role'], str) or not schema.SAFE.fullmatch(route['role']) or
                route['sslmode'] != 'disable' or service == 'user' and route['host'] != 'voice-postgres'):
            raise ValueError()
        data = {}
        for name, raw in sorted(files.items()):
            if (not isinstance(name, str) or not re.fullmatch(r'[0-9]{6}_[a-z0-9_]+\.(up|down)\.sql', name) or
                    not isinstance(raw, bytes) or not 0 < len(raw) <= 4 * 1024 * 1024):
                raise ValueError()
            data[name] = raw.decode('utf-8')
        if sum(len(raw) for raw in files.values()) > 900000:
            raise ValueError()
        suffix = operation[-32:]
        name = 'voice-go-migrate-' + service + '-' + suffix
        config_name = 'voice-go-migrations-' + service + '-' + suffix
        annotation = {'voice.io/source-sha': source_sha}
        config = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
                  'metadata': {'name': config_name, 'namespace': 'voice-dev', 'annotations': annotation}, 'data': data}
        url = 'postgres://' + route['role'] + '@' + route['service_ip'] + ':5432/' + database + '?sslmode=disable'
        env = [{'name': 'PGPASSWORD', 'valueFrom': {'secretKeyRef': {'name': _secret_name(operation, database), 'key': 'PGPASSWORD'}}},
               {'name': 'DATABASE_URL', 'value': url},
               {'name': 'PGAPPNAME', 'value': name}, {'name': 'VOICE_DB_SERVICE_IP', 'value': route['service_ip']},
               {'name': 'VOICE_DB_ROLE', 'value': route['role']}, {'name': 'VOICE_DATABASE', 'value': database}]
        command = ('[ "$DATABASE_URL" = "postgres://$VOICE_DB_ROLE@$VOICE_DB_SERVICE_IP:5432/$VOICE_DATABASE?sslmode=disable" ] || exit 1; '
                   'exec migrate -path=/migrations -database "$DATABASE_URL" up')
        pod = {'restartPolicy': 'Never', 'automountServiceAccountToken': False,
               'nodeSelector': {'kubernetes.io/hostname': node['hostname']},
               'containers': [{'name': 'migrate', 'image': realtime_migration_job.MIGRATE_IMAGE,
                               'imagePullPolicy': 'IfNotPresent', 'command': ['/bin/sh', '-ec', command], 'env': env,
                               'volumeMounts': [{'name': 'migrations', 'mountPath': '/migrations', 'readOnly': True}],
                               'resources': {'requests': {'cpu': '20m', 'memory': '64Mi'},
                                             'limits': {'cpu': '500m', 'memory': '512Mi'}}}],
               'volumes': [{'name': 'migrations', 'configMap': {'name': config_name, 'optional': False, 'defaultMode': 420}}]}
        job = {'apiVersion': 'batch/v1', 'kind': 'Job',
               'metadata': {'name': name, 'namespace': 'voice-dev', 'annotations': annotation},
               'spec': {'backoffLimit': 0, 'activeDeadlineSeconds': 600,
                        'template': {'metadata': {'labels': {'app': name}}, 'spec': pod}}}
        return config, job
    except (UnicodeError, TypeError, KeyError, ValueError):
        raise MigrationError('generic migration render refused') from None
