"""Collect source-bound, private metadata for the full-cohort restore.

Plans select source-backed targets for isolated restore; they are not an
admission to mutate the live database or proof of application compatibility.
"""
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import re
import secrets
import stat
import tarfile

import canonical_application_package as canonical
import cohort_unmanaged_actual as unmanaged
import frontend_runtime_executor
import full_cohort_affected_plan as affected
import full_cohort_go_migration_plan as go_plan
import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import full_cohort_schema_metadata as schema
import media_backup_upload_phase
import media_operation_guard
import publish_candidate
import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as restore
import story_media_backup_restore as media_restore
import story_media_backup
import auth_user_adoption_runtime
import realtime_migration_job
import writer_cut


class MetadataError(RuntimeError):
    pass


STEP = 'full-cohort-backup-metadata'
LEDGER = 'full_cohort_backup_metadata'
FILENAME = 'cohort-backup-metadata.json'
LIMIT = 32 * 1024 * 1024


def _bytes(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True, allow_nan=False).encode('ascii')


def _digest(value):
    return hashlib.sha256(_bytes(value)).hexdigest()


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError()
        result[key] = value
    return result


def _query_for(database):
    if database == 'roles':
        return media_restore.role_catalog_sql()
    if database == 'auth_db':
        return restore.AUTH_CATALOG.replace('FROM flyway_schema_history)', 'FROM public.flyway_schema_history)')
    if database == 'gateway_db':
        return restore.GATEWAY_CATALOG
    if database in cohort.ALLOWED_DATABASES:
        return media_restore.CATALOG.replace('FROM schema_migrations)', 'FROM public.schema_migrations)')
    return unmanaged.CATALOG_SQL


def _sql(query):
    # The caller selects only constants above, never caller-supplied SQL.
    query = query.strip().removesuffix(';')
    return ("SELECT json_build_object('database',current_database(),"
            "'database_oid',(SELECT oid::bigint FROM pg_catalog.pg_database WHERE datname=current_database()),"
            "'role',current_user,'server_start',pg_catalog.pg_postmaster_start_time(),"
            "'read_only',current_setting('transaction_read_only'),"
            "'data_directory',current_setting('data_directory'),"
            "'pg_major',current_setting('server_version_num')::integer / 10000,'value',(" + query + '))')


def checked_session(observation, expected):
    """Reject observations from a different physical or logical PG session."""
    fields = {'database', 'database_oid', 'role', 'server_start', 'read_only', 'data_directory', 'pg_major'}
    if (not isinstance(expected, dict) or set(expected) != fields or
            not isinstance(observation, dict) or set(observation) != fields | {'value'} or
            type(observation['database_oid']) is not int or type(observation['pg_major']) is not int or
            any(observation[k] != expected[k] for k in fields)):
        raise MetadataError('database session changed')
    return copy.deepcopy(observation['value'])


def restore_plan(database, catalog, files):
    """Derive a no-migration restore plan from observed ledger and source files."""
    try:
        if not isinstance(files, dict) or any(not isinstance(k, str) or
                not isinstance(v, str) or not re.fullmatch(r'[0-9a-f]{64}', v)
                for k, v in files.items()):
            raise ValueError()
        if database not in cohort.ALLOWED_DATABASES:
            if files:
                raise ValueError()
            plan = {'engine': 'unmanaged', 'from': None, 'to': None, 'files': {}}
        elif database == 'gateway_db':
            history = catalog['history']
            if (not isinstance(history, dict) or set(history) != {'schema_migrations', 'flyway_schema_history'} or
                    any(history[k] is not False for k in history)):
                raise ValueError()
            plan = {'engine': 'unmanaged', 'from': None, 'to': None, 'files': copy.deepcopy(files)}
        elif database == 'auth_db':
            history = catalog['history']
            if not isinstance(history, list) or not history:
                raise ValueError()
            versions = []
            for row in history:
                if not isinstance(row, dict) or set(row) != {'version', 'script', 'checksum', 'success'} or row['success'] is not True:
                    raise ValueError()
                if row['version'] is not None:
                    version = row['version']
                    script = row['script']
                    match = restore.FLYWAY_SQL.fullmatch(script) if isinstance(script, str) else None
                    if not isinstance(version, str) or not re.fullmatch(r'[0-9]+', version) or not match or int(match.group(1)) != int(version) or script not in files:
                        raise ValueError()
                    versions.append(int(version))
            if not versions:
                raise ValueError()
            version = max(versions)
            plan = {'engine': 'flyway', 'from': version, 'to': version, 'files': copy.deepcopy(files)}
        else:
            history = catalog['history']
            if (not isinstance(history, list) or len(history) != 1 or not isinstance(history[0], dict) or
                    set(history[0]) != {'version', 'dirty'} or type(history[0]['version']) is not int or
                    history[0]['version'] < 1 or history[0]['dirty'] is not False):
                raise ValueError()
            version = history[0]['version']
            plan = {'engine': 'golang-migrate', 'from': version, 'to': version, 'files': copy.deepcopy(files)}
        if restore.classify_plan(database, plan, True) != 'restore-only':
            raise ValueError()
        return plan
    except (KeyError, TypeError, ValueError, restore.CohortRestoreError):
        raise MetadataError('restore-only plan unavailable') from None


def _migration_roles(phase, plans, catalog, roles):
    """Bind actual fixed migration logins to the source role catalog privately."""
    import full_cohort_go_migration as migration
    result = {}
    for db, plan in plans.items():
        if db not in go_plan.DATABASES or plan['from'] == plan['to']:
            continue
        credential = migration._credential(db, phase._guard)
        source = {'engine': catalog['engine'], 'server_start': catalog['server_start'],
                  'database_oid': catalog['databases'][db]}
        safe = migration._login(db, credential, source, phase._guard)
        if migration._credential(db, phase._guard) != credential:
            raise MetadataError('migration credential changed')
        matching = [role for role in roles['roles'] if role.get('name') == safe['role']]
        if len(matching) != 1 or matching[0].get('login') is not True:
            raise MetadataError('migration role catalog unavailable')
        result[db] = safe
    return result


class BackupMetadataPhase:
    """One fenced pre-backup observation; never an application rollout gate."""
    def __init__(self, journal, fence, client, lock, namespace_uid, package_phase,
                 inventory_phase, catalog_phase, cut, source_root):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.package, self.inventory, self.catalog, self.cut = package_phase, inventory_phase, catalog_phase, cut
        self.source_root = Path(source_root)
        self.intent = copy.deepcopy(journal.intent)
        self.poisoned = False
        self._dependencies = (journal, fence, client, package_phase, inventory_phase, catalog_phase, cut)
        self._source_root = self.source_root
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                getattr(fence.ledger, 'client', None) is not client or
                type(package_phase) is not package.PackagePhase or type(inventory_phase) is not inventory.InventoryPhase or
                type(catalog_phase) is not affected.FullCatalogPhase or type(cut) is not writer_cut.WriterCut or
                inventory_phase.package is not package_phase or catalog_phase.inventory is not inventory_phase or
                self.intent.get('resource_context') != fence.context or self.intent.get('namespace_uid') != namespace_uid or
                self.source_root.resolve() != canonical.ROOT.resolve()):
            raise MetadataError('metadata phase identity invalid')
        for other in (package_phase, inventory_phase, catalog_phase, cut):
            if other.fence.context != fence.context or other.lock != self.lock or other.namespace_uid != namespace_uid:
                raise MetadataError('metadata phase identity invalid')

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or self.source_root != self._source_root or
                any(a is not b for a, b in zip((self.journal, self.fence, self.client, self.package,
                                                self.inventory, self.catalog, self.cut), self._dependencies)) or
                any(other.fence.context != self.fence.context or other.lock != self.lock or
                    other.namespace_uid != self.namespace_uid for other in (self.package, self.inventory, self.catalog, self.cut))):
            raise MetadataError('metadata identity changed')
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)

    def _ledger(self):
        row, revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or
                not isinstance(revision, tuple) or len(revision) != 2 or
                any(not isinstance(v, str) or not v for v in revision)):
            raise MetadataError('metadata ledger changed')
        for other in (self.package, self.inventory, self.catalog, self.cut):
            if other.fence.ledger.read() != (row, revision):
                raise MetadataError('metadata ledger differs')
        return row, revision

    def _read(self, database, oid, engine, start, role, snapshot):
        selected = 'postgres' if database == 'roles' else database
        if not schema.SAFE.fullmatch(selected):
            raise MetadataError('database identity invalid')
        if schema.PSQL_SCRIPT.count('-c lock_timeout=3000') != 1:
            raise MetadataError('database observer contract changed')
        expected = {'database': selected, 'database_oid': oid, 'role': role, 'server_start': start,
                    'read_only': 'on', 'data_directory': '/var/lib/postgresql/data', 'pg_major': 16}
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                '--namespace', 'voice-dev', '--request-timeout=15s', 'exec', 'voice-postgres-0',
                '-c', 'postgres', '--', 'sh', '-ec', schema.PSQL_SCRIPT.replace(
                    '-c lock_timeout=3000', '-c lock_timeout=3000 -c search_path=pg_catalog,public'),
                '--', selected, _sql(_query_for(database))]
        self._guard()
        if snapshot() != engine:
            raise MetadataError('database engine changed')
        data = schema._bounded_process(args, LIMIT)
        observed = json.loads(data, object_pairs_hook=_unique)
        self._guard()
        if snapshot() != engine:
            raise MetadataError('database engine changed')
        return checked_session(observed, expected)

    def _source_files(self, receipt, token):
        self._guard()
        if self.package.verify_current(token) != receipt:
            raise MetadataError('package changed')
        snapshot = publish_candidate.source_snapshot(self.source_root, self.fence.context['commit'])
        # Parse the full canonical inventory, including duplicate/version checks.
        schema.catalog(snapshot)
        if self.package.verify_current(token) != receipt:
            raise MetadataError('package changed')
        return snapshot

    @staticmethod
    def _files(database, source):
        if database not in cohort.ALLOWED_DATABASES:
            return {}
        prefix = ('src/backend/auth/src/main/resources/db/migration/' if database == 'auth_db' else
                  'src/backend/migrations/' + database + '/')
        return {name[len(prefix):]: digest for name, digest in source.items()
                if name.startswith(prefix) and name.endswith('.sql')}

    def _record(self, marker, old=None):
        self._guard(); row, revision = self._ledger()
        if row.get(LEDGER) != old:
            raise MetadataError('metadata reservation changed')
        row[LEDGER] = copy.deepcopy(marker)
        row['step'] = STEP + '-' + marker['status']
        self.fence.ledger.write(revision, row)
        self._guard(); current, _ = self._ledger()
        if current.get(LEDGER) != marker or self.journal.state['steps'].get(STEP) != marker:
            raise MetadataError('metadata reservation changed')

    def _tool_state(self, value):
        self._guard(); row, revision = self._ledger()
        marker = row.get(LEDGER)
        if (not isinstance(marker, dict) or marker.get('status') != 'pending' or
                self.journal.state['steps'].get(STEP) != marker):
            raise MetadataError('tool reservation missing')
        new = copy.deepcopy(marker); new['tool'] = copy.deepcopy(value)
        self.journal.state['steps'][STEP] = copy.deepcopy(new); self.journal.save()
        self._guard(); current, current_revision = self._ledger()
        if current != row or current_revision != revision:
            raise MetadataError('tool reservation changed')
        current[LEDGER] = copy.deepcopy(new); self.fence.ledger.write(current_revision, current)
        self._guard()
        if self._ledger()[0].get(LEDGER) != new or self.journal.state['steps'].get(STEP) != new:
            raise MetadataError('tool reservation changed')

    def _tool_sha(self):
        image = realtime_migration_job.MIGRATE_IMAGE
        self._guard()
        info = json.loads(schema._bounded_process(['docker', 'image', 'inspect', image], 1048576))
        self._guard()
        if (not isinstance(info, list) or len(info) != 1 or info[0].get('Os') != 'linux' or info[0].get('Architecture') != 'amd64' or
                image not in (info[0].get('RepoDigests') or []) or
                not re.fullmatch(r'sha256:[0-9a-f]{64}', info[0].get('Id', ''))):
            raise MetadataError('tool image not qualified')
        image_id = info[0]['Id']
        name = 'voice-media-restore-' + secrets.token_hex(16)
        self._tool_state({'status': 'create-pending', 'name': name, 'image_id': image_id})
        result = schema._bounded_process(['docker', 'create', '--name', name, '--network', 'none',
                                         '--label', 'voice.test.owner=' + name, '--entrypoint', '/bin/true', image_id], 256)
        self._guard()
        cid = result.decode('ascii').strip()
        if not re.fullmatch(r'[0-9a-f]{64}', cid):
            raise MetadataError('tool container identity unknown')
        details = json.loads(schema._bounded_process(['docker', 'inspect', cid], 1048576))
        self._guard()
        if (not isinstance(details, list) or len(details) != 1 or details[0].get('Id') != cid or
                details[0].get('Image') != image_id or details[0].get('Config', {}).get('Labels', {}).get('voice.test.owner') != name or
                details[0].get('HostConfig', {}).get('NetworkMode') != 'none' or
                details[0].get('HostConfig', {}).get('PortBindings') or details[0].get('Mounts')):
            raise MetadataError('tool container identity changed')
        self._tool_state({'status': 'created', 'name': name, 'image_id': image_id, 'id': cid})
        raw = schema._bounded_process(['docker', 'cp', cid + ':/usr/local/bin/migrate', '-'], media_restore.MAX_TOOL_BYTES + 10240)
        self._guard()
        with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
            members = archive.getmembers()
            if (len(members) != 1 or members[0].name != 'migrate' or not members[0].isfile() or
                    not 0 < members[0].size <= media_restore.MAX_TOOL_BYTES):
                raise MetadataError('tool archive invalid')
            contents = archive.extractfile(members[0]).read(media_restore.MAX_TOOL_BYTES + 1)
        if len(contents) != members[0].size:
            raise MetadataError('tool archive invalid')
        digest = hashlib.sha256(contents).hexdigest()
        self._tool_state({'status': 'remove-pending', 'name': name, 'image_id': image_id, 'id': cid})
        self._guard()
        schema._bounded_process(['docker', 'rm', '-f', '-v', cid], 256)
        self._guard()
        remaining = schema._bounded_process(['docker', 'ps', '-a', '--no-trunc', '--format', '{{.ID}}'], 1048576).decode('ascii').splitlines()
        if cid in remaining:
            raise MetadataError('tool cleanup incomplete')
        self._tool_state({'status': 'removed', 'name': name, 'image_id': image_id, 'id': cid})
        return digest

    def run(self, token):
        pending = False
        try:
            self._guard(); candidate = self.package.verify_current(token)
            self.inventory.verify_receipt(token); self.cut.verify_drained()
            catalog = self.catalog.verify_current(token)
            source_snapshot = self._source_files(candidate, token)
            source = source_snapshot['files']
            databases = sorted(catalog['databases'])
            if 'postgres' not in databases or not databases:
                raise MetadataError('roles observation database unavailable')
            row, _ = self._ledger()
            catalog_marker = row['full_cohort_db_catalog']
            if catalog_marker.get('receipt_sha256') != _digest(catalog):
                raise MetadataError('catalog receipt changed')
            if STEP in self.journal.state['steps'] or LEDGER in row:
                raise MetadataError('metadata attempt already exists')
            marker = {'status': 'pending', 'nonce': secrets.token_hex(16),
                      'operation_id': self.fence.context['operation_id'], 'source_sha': self.fence.context['commit']}
            pending = True
            self.journal.state['steps'][STEP] = copy.deepcopy(marker); self.journal.save()
            self._record(marker)
            tool_sha = self._tool_sha()
            engine, start = catalog['engine'], catalog['server_start']
            snapshot = schema.kube_engine_snapshot(schema.linux_private_bytes, self._guard)
            if snapshot() != engine:
                raise MetadataError('database engine changed')
            role = self.inventory._admin_role()
            def sample():
                self.cut.verify_drained()
                roles = self._read('roles', catalog['databases']['postgres'], engine, start, role, snapshot)
                result = {db: self._read(db, catalog['databases'][db], engine, start, role, snapshot)
                          for db in databases}
                self.cut.verify_drained()
                return roles, result
            roles, catalogs = sample()
            if sample() != (roles, catalogs):
                raise MetadataError('metadata changed during observation')
            if not isinstance(roles, dict) or set(roles) != {'roles', 'memberships'}:
                raise MetadataError('role catalog invalid')
            plans = {}
            for db in databases:
                if db not in cohort.ALLOWED_DATABASES:
                    unmanaged.validate_source_catalog(catalogs[db], catalog['properties'][db])
                baseline = restore_plan(db, catalogs[db], self._files(db, source))
                plans[db] = go_plan.for_restore(db, baseline, source_snapshot)
            migration_roles = _migration_roles(self, plans, catalog, roles)
            restore.migration_inventory(self.source_root, databases, plans, full_catalog=True)
            metadata = {'catalog': catalogs, 'roles': roles, 'migrations': plans}
            sources = {'roles': {'engine': engine, 'server_start': start}}
            sources.update({db: {'engine': engine, 'server_start': start, 'database_oid': catalog['databases'][db]}
                            for db in databases})
            restore_inputs = {'catalog_sha256': _digest(catalogs), 'roles_catalog_sha256': _digest(roles),
                              'migration_catalog_sha256': _digest(plans),
                              'postgres_image': auth_user_adoption_runtime.QUALIFIED_TOOLS['postgres'],
                              'migrate_image': realtime_migration_job.MIGRATE_IMAGE,
                              'migrate_binary_sha256': tool_sha, 'postgres_major': 16}
            result = {'operation_id': self.fence.context['operation_id'], 'source_sha': self.fence.context['commit'],
                      'catalog_receipt_sha256': catalog_marker['receipt_sha256'], 'databases': databases,
                      'sources': sources, 'metadata': metadata, 'restore_inputs': restore_inputs,
                      'full_catalog': {'receipt_sha256': catalog_marker['receipt_sha256'], 'properties': catalog['properties']},
                      'migration_roles': migration_roles,
                      'migration_holds': ({'space_db': 'native_installer_provenance_required'}
                                          if 'space_db' in databases else {}),
                      'scope': 'source-metadata-with-isolated-restore-targets'}
            if (self.catalog.verify_current(token) != catalog or self.package.verify_current(token) != candidate or
                    self._source_files(candidate, token) != source_snapshot):
                raise MetadataError('source or catalog changed')
            self.cut.verify_drained(); self._guard()
            raw = _bytes(result)
            if not 0 < len(raw) <= LIMIT:
                raise MetadataError('metadata size invalid')
            root = media_backup_upload_phase.ROOT / self.fence.context['operation_id']
            with story_media_backup.private_directory(root) as directory:
                fd = os.open(FILENAME, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=directory)
                with os.fdopen(fd, 'wb') as stream:
                    stream.write(raw); stream.flush(); os.fsync(stream.fileno())
                os.fsync(directory)
            current, _ = self._ledger(); old = current[LEDGER]
            if (old.get('status') != 'pending' or old.get('tool', {}).get('status') != 'removed' or
                    self.journal.state['steps'].get(STEP) != old):
                raise MetadataError('metadata reservation changed')
            done = {k: old[k] for k in ('nonce', 'operation_id', 'source_sha')}
            done.update(status='done', receipt_sha256=hashlib.sha256(raw).hexdigest())
            self.journal.state['steps'][STEP] = copy.deepcopy(done); self.journal.save()
            self._record(done, old)
            return verify_completed(self, token)
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('backup metadata outcome uncertain')
                except BaseException: pass
            raise MetadataError('backup metadata unavailable; reconcile' if pending else 'backup metadata preflight refused') from None


def verify_completed(phase, token):
    """Load the private snapshot linked to the actual completed active ledger."""
    try:
        if type(phase) is not BackupMetadataPhase:
            raise ValueError()
        phase._guard(); phase.package.verify_current(token); phase.inventory.verify_receipt(token)
        catalog = phase.catalog.verify_receipt(token)
        row, _ = phase._ledger(); marker = row[LEDGER]
        if (not isinstance(marker, dict) or set(marker) != {'status', 'nonce', 'operation_id', 'source_sha', 'receipt_sha256'} or
                marker['status'] != 'done' or marker['operation_id'] != phase.fence.context['operation_id'] or
                marker['source_sha'] != phase.fence.context['commit'] or
                phase.journal.state['steps'].get(STEP) != marker):
            raise ValueError()
        root = media_backup_upload_phase.ROOT / marker['operation_id']
        with story_media_backup.private_directory(root) as directory:
            fd = os.open(FILENAME, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
            try:
                before = os.fstat(fd)
                if (not stat.S_ISREG(before.st_mode) or before.st_uid != os.geteuid() or
                        stat.S_IMODE(before.st_mode) != 0o600 or before.st_nlink != 1 or not 0 < before.st_size <= LIMIT):
                    raise ValueError()
                with os.fdopen(fd, 'rb', closefd=False) as stream:
                    raw = stream.read(LIMIT + 1)
                after = os.fstat(fd); current = os.stat(FILENAME, dir_fd=directory, follow_symlinks=False)
                identity = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns, s.st_uid, s.st_mode, s.st_nlink)
                if len(raw) != before.st_size or identity(before) != identity(after) or identity(current) != identity(before):
                    raise ValueError()
            finally: os.close(fd)
        result = json.loads(raw, object_pairs_hook=_unique)
        if (hashlib.sha256(raw).hexdigest() != marker['receipt_sha256'] or result['operation_id'] != marker['operation_id'] or
                result['source_sha'] != marker['source_sha'] or result['catalog_receipt_sha256'] != _digest(catalog) or
                result['full_catalog'] != {'receipt_sha256': _digest(catalog), 'properties': catalog['properties']} or
                result['databases'] != sorted(catalog['databases']) or
                not isinstance(result.get('migration_roles'), dict) or
                set(result['migration_roles']) != {db for db, plan in result['metadata']['migrations'].items()
                                                   if db in go_plan.DATABASES and plan['from'] != plan['to']} or
                result['migration_holds'] != ({'space_db': 'native_installer_provenance_required'}
                                              if 'space_db' in result['databases'] else {}) or
                result['scope'] != 'source-metadata-with-isolated-restore-targets'):
            raise ValueError()
        phase._guard()
        if phase._ledger()[0].get(LEDGER) != marker or phase.journal.state['steps'].get(STEP) != marker:
            raise ValueError()
        return copy.deepcopy(result)
    except BaseException:
        raise MetadataError('completed backup metadata unavailable') from None


def verify_current(phase, token):
    """Recheck twice around a drained pre-provision backup, not an atomic snapshot."""
    try:
        result = verify_completed(phase, token)
        phase.cut.verify_drained()
        catalog = phase.catalog.verify_current(token)
        if _digest(catalog) != result['catalog_receipt_sha256']:
            raise ValueError()
        candidate = phase.package.verify_current(token)
        source = phase._source_files(candidate, token)
        restore.migration_inventory(phase.source_root, result['databases'], result['metadata']['migrations'], full_catalog=True)
        engine, start = catalog['engine'], catalog['server_start']
        snapshot = schema.kube_engine_snapshot(schema.linux_private_bytes, phase._guard)
        role = phase.inventory._admin_role()
        for _ in range(2):
            if phase._read('roles', catalog['databases']['postgres'], engine, start, role, snapshot) != result['metadata']['roles']:
                raise ValueError()
            for db in result['databases']:
                if phase._read(db, catalog['databases'][db], engine, start, role, snapshot) != result['metadata']['catalog'][db]:
                    raise ValueError()
        if _migration_roles(phase, result['metadata']['migrations'], catalog, result['metadata']['roles']) != result['migration_roles']:
            raise ValueError()
        phase.cut.verify_drained()
        if (phase.catalog.verify_current(token) != catalog or phase._source_files(candidate, token) != source or
                verify_completed(phase, token) != result):
            raise ValueError()
        return result
    except BaseException:
        raise MetadataError('current backup metadata unavailable') from None
