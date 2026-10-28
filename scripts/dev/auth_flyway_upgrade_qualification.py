"""Checks for an admitted Auth Flyway upgrade on an isolated restore.

Admission is supplied by the protected orchestrator, not established here.
This module does not authorize or perform a live migration.
"""
import re
import copy
import hashlib
import hmac
import io
import json
import os
import pathlib
import secrets
import subprocess
import tarfile
import tempfile
import time

TRANSITION_SOURCES = {
    'V12__ownership_transfer_proofs.sql': 'd8bb335db193a5ad7221fb79d563957f4c4c51ce3223c69593860df768a9e99c',
    'V26__authority_source_revision.sql': '7dc77ce6479f7713780a5cc43f211346a781618c1d8a66595c63c1ca9c2e8441',
    'V28__role_identity_owner.sql': 'd45a6d138a1856fffc7fa79834d1c3b175c6fa5ae1f8d768a5f20b19204e670e',
    'V31__auth_identity_ddl_barrier.sql': '9bc1e3474ba1a2703e661452478b22b7514df0c1322d5e285dd783b6307120b1',
    'V32__linked_oauth_transport.sql': '812d4f20cd6ea8ccc97104fc1a1bd899e5d3e71a35e8065003eb8babcfcb51cc',
}

def validate_admission(admission, source):
    if (not isinstance(admission, dict)
            or set(admission) != {'source_sha', 'target', 'flyway_enabled', 'selector_mode', 'persistence',
                                  'plan_sha256', 'operator_sha256', 'sources_sha256'}
            or not isinstance(source, str) or not re.fullmatch(r'[0-9a-f]{40}', source)
            or source == '0' * 40 or admission['source_sha'] != source
            or type(admission['target']) is not int or admission['target'] != 32
            or admission['flyway_enabled'] is not False or admission['selector_mode'] != 'absent'
            or admission['persistence'] != 'jdbc'
            or any(not re.fullmatch(r'[0-9a-f]{64}', str(admission[k]))
                   or admission[k] == '0' * 64 for k in ('plan_sha256', 'operator_sha256', 'sources_sha256'))):
        raise ValueError('unsupported source or protected migration admission')


def validate_history(before, after):
    if (not before or before[-1].get('version') != '26' or after[:len(before)] != before
            or any(row.get('success') is not True or row.get('type') != 'SQL' for row in after)
            or [row.get('version') for row in after[len(before):]] != ['28', '31', '32']):
        raise ValueError('migration history differs from the admitted transition')


def validate_markers(before, after):
    expected = {name: [dict(row) for row in rows] for name, rows in before.items()}
    if 'accounts' not in expected or 'auth_authority_revision' not in expected:
        raise ValueError('required marker missing')
    for row in expected['accounts']:
        if row['deleted_at'] is not None and row['status'] != 'deleted':
            row['status'] = 'deleted'
            row['security_revision'] += 1
    revisions = expected['auth_authority_revision']
    if len(revisions) != 1 or type(revisions[0].get('revision')) is not int:
        raise ValueError('authority revision unavailable')
    revisions[0]['revision'] += 1
    if expected != after:
        raise ValueError('old data changed outside V28 transformations')


def source_catalog(root, source):
    """Resolve every Auth migration from the selected immutable Git tree."""
    if not isinstance(source, str) or not re.fullmatch(r'[0-9a-f]{40}', source) or source == '0' * 40:
        raise ValueError('invalid candidate source')
    folder = 'src/backend/auth/src/main/resources/db/migration'
    root = pathlib.Path(root)
    def git(*args):
        result = subprocess.run(['git', '-C', str(root), *args], capture_output=True, check=False)
        if result.returncode:
            raise ValueError('candidate Git source unavailable')
        return result.stdout
    if git('rev-parse', '--verify', source + '^{commit}').decode().strip() != source:
        raise ValueError('candidate source does not resolve exactly')
    entries = git('ls-tree', '-r', source, '--', folder).decode().splitlines()
    paths = []
    for entry in entries:
        metadata, separator, path = entry.partition('\t')
        if not separator or not re.fullmatch(r'100644 blob [0-9a-f]{40}', metadata):
            raise ValueError('candidate migration must be a regular Git blob')
        paths.append(path)
    if not paths or any(not re.fullmatch(re.escape(folder) + r'/V[0-9]+__[A-Za-z0-9_]+\.sql', p) for p in paths):
        raise ValueError('unrecognized candidate migration catalog')
    sources = {p.rsplit('/', 1)[1]: git('show', source + ':' + p) for p in paths}
    if len(sources) != len(paths):
        raise ValueError('duplicate source name')
    catalog = {name: hashlib.sha256(value).hexdigest() for name, value in sources.items()}
    if any(catalog.get(name) != value for name, value in TRANSITION_SOURCES.items()):
        raise ValueError('migration transition sources differ from reviewed invariants')
    return sources, catalog


def candidate_blob(root, source, path):
    entry = subprocess.run(['git', '-C', str(root), 'ls-tree', source, '--', path], capture_output=True)
    if entry.returncode or not re.fullmatch(r'100644 blob [0-9a-f]{40}\t' + re.escape(path) + r'\n', entry.stdout.decode()):
        raise ValueError('candidate file is not a regular Git blob')
    blob = subprocess.run(['git', '-C', str(root), 'show', source + ':' + path], capture_output=True)
    if blob.returncode:
        raise ValueError('candidate file unavailable')
    return blob.stdout


def validate_backup_inputs(admission, manifest, report, operation, directory):
    """Check existing encrypted backup protocol; caller authenticates admission."""
    from auth_user_adoption_backup import validate_restore
    validate_admission(admission, admission['source_sha'])
    validate_restore(manifest, report, operation)
    binding = manifest.get('binding', {})
    if (report.get('scope') != 'actual-existing-dev-backup' or manifest.get('private_fixture')
            or any(binding.get(k) != admission[k] for k in ('source_sha', 'operator_sha256', 'sources_sha256'))
            or set(manifest.get('files', {})) != {'auth_db.enc', 'user_db.enc', 'roles.enc'}):
        raise ValueError('backup is not bound to the admitted source and operation')
    directory = pathlib.Path(directory)
    if directory.is_symlink() or any(p.is_symlink() for p in directory.parents):
        raise ValueError('backup directory path must not contain symlinks')
    def read_private(name, limit):
        from auth_user_adoption import Journal
        dirfd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            Journal._private(os.fstat(dirfd), 0o700, directory=True)
            fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=dirfd)
            with os.fdopen(fd, 'rb') as stream:
                Journal._private(os.fstat(stream.fileno()), 0o600)
                data = stream.read(limit + 1)
        finally:
            os.close(dirfd)
        if len(data) > limit:
            raise ValueError('private backup member too large')
        return data
    key = read_private('backup.key', 65).decode().strip()
    if not re.fullmatch(r'[0-9a-f]{64}', key):
        raise ValueError('private backup key invalid')
    encrypted = {}
    for name, expected in manifest['files'].items():
        raw = read_private(name, 256 * 1024 * 1024)
        identity = {'sha256': hashlib.sha256(raw).hexdigest(),
                    'hmac': hmac.new(bytes.fromhex(key), raw, hashlib.sha256).hexdigest()}
        if identity != expected:
            raise ValueError('encrypted backup content changed')
        encrypted[name] = raw
    return key, encrypted


def marker_snapshot(query, columns, *, before):
    """Hash all preexisting ordinary-table rows over their original columns.

    Query is an internal isolated-clone PostgreSQL adapter, never caller SQL.
    """
    result = {}
    for table, old_columns in sorted(columns.items()):
        if table == 'flyway_schema_history':
            continue
        if not re.fullmatch(r'[a-z][a-z0-9_]*', table):
            raise ValueError('unrecognized table identifier')
        current = query("SELECT coalesce(json_agg(attname ORDER BY attnum),'[]'::json) "
                        "FROM pg_attribute WHERE attrelid='public." + table + "'::regclass "
                        "AND attnum>0 AND NOT attisdropped")
        if not all(isinstance(x, str) and re.fullmatch(r'[a-z][a-z0-9_]*', x) for x in current):
            raise ValueError('unrecognized column identifier')
        additions = [x for x in current if x not in old_columns]
        if ([x for x in current if x in old_columns] != old_columns or
                additions != ([] if before or table != 'accounts' else [
                    'role_identity_handle', 'role_identity_selector_version',
                    'role_identity_account_selector', 'role_identity_keyset_revision'])):
            raise ValueError('original table columns changed unexpectedly')
        expr = 'to_jsonb(t)'
        if additions:
            expr += " - ARRAY[" + ','.join("'" + name + "'" for name in additions) + "]::text[]"
        if before and table == 'accounts':
            changed = "jsonb_set(jsonb_set(" + expr + ",'{status}',to_jsonb('deleted'::text)),'{security_revision}',to_jsonb(t.security_revision+1))"
            expr = "CASE WHEN t.deleted_at IS NOT NULL AND t.status <> 'deleted' THEN " + changed + " ELSE " + expr + ' END'
        if before and table == 'auth_authority_revision':
            expr = "jsonb_set(" + expr + ",'{revision}',to_jsonb(t.revision+1))"
        result[table] = query("SELECT json_build_object('count',count(*),'sha256',"
            "encode(sha256(convert_to(coalesce(string_agg(j::text,E'\\n' ORDER BY j::text),''),'UTF8')),'hex')) "
            "FROM (SELECT " + expr + ' AS j FROM public.' + table + ' AS t) rows')
    return result


def snapshot_protected(value):
    try:
        raw = json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode('utf-8')
        if len(raw) > 2 * 1024 * 1024:
            raise ValueError('protected input exceeds limit')
        return json.loads(raw)
    except (TypeError, UnicodeError, RecursionError):
        raise ValueError('protected input must be finite JSON') from None


def qualify(root, admission, backup_directory, manifest, report, operation, tools_image, journal):
    """Run one isolated qualification attempt; the protected caller authenticates inputs.

    The receipt is not release admission or permission to migrate any live database.
    """
    from auth_user_adoption_runtime import (QUALIFIED_TOOLS, backup_catalog_queries, validate_ddl_gate,
                                            digest, operator_fingerprint, source_config)
    from auth_flyway_upgrade_qualification_docker import OwnedDocker
    admission, manifest, report = (snapshot_protected(x) for x in (admission, manifest, report))
    source = admission.get('source_sha')
    validate_admission(admission, source)
    if not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,30}', operation):
        raise ValueError('operation identity invalid')
    if (not re.fullmatch(r'sha256:[0-9a-f]{64}', tools_image)
            or report.get('tools_image') != tools_image):
        raise ValueError('exact qualified tools image required')
    sources, catalog = source_catalog(root, source)
    gate_path = 'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql'
    gate_bytes = candidate_blob(root, source, gate_path)
    config = source_config(root, operation)
    if (operator_fingerprint(root) != admission['operator_sha256']
            or digest(config) != admission['sources_sha256']
            or any(config['data'].get('auth-' + name, '').encode() != raw for name, raw in sources.items())):
        raise ValueError('candidate operator/source bundle changed')
    key, encrypted = validate_backup_inputs(admission, manifest, report, operation, backup_directory)
    docker = OwnedDocker(journal)
    call = docker.call
    password = secrets.token_hex(24)
    container = None
    receipt = None
    try:
        def decrypt(name):
            cid = docker.create(tools_image, ['-i', '--network', 'none', '-e', 'VOICE_BACKUP_KEY=' + key,
                         '--entrypoint', 'openssl'], ['enc', '-d', '-aes-256-cbc', '-pbkdf2', '-iter',
                                                    '200000', '-pass', 'env:VOICE_BACKUP_KEY'])
            return call('start', '-a', '-i', cid, data=encrypted[name])
        role_sql = decrypt('roles.enc').decode('utf-8')
        if re.search(r'\bPASSWORD\s', role_sql, re.I) or role_sql.count('CREATE ROLE voice;') != 1:
            raise ValueError('role archive is not admitted')
        role_sql = role_sql.replace('CREATE ROLE voice;', '-- disposable migration owner exists')
        container = docker.create(QUALIFIED_TOOLS['postgres'], ['--network', 'none', '--memory', '1g',
                  '--cpus', '2', '-e', 'POSTGRES_USER=voice', '-e', 'POSTGRES_PASSWORD=' + password],
                  ['-c', 'cluster_name=voice-auth-flyway-qualification'])
        call('start', container)
        def psql(db, sql):
            raw = call('exec', '-i', '-e', 'PGPASSWORD=' + password, container, 'psql', '-h', '127.0.0.1',
                       '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-U', 'voice', '-d', db, data=sql.encode())
            return raw
        for attempt in range(100):
            try:
                if psql('postgres', 'SELECT 1').strip() == b'1':
                    break
            except ValueError:
                time.sleep(.2)
        else:
            raise ValueError('private PostgreSQL not ready')
        if psql('postgres', 'SHOW cluster_name').strip() != b'voice-auth-flyway-qualification':
            raise ValueError('private PostgreSQL identity changed')
        psql('postgres', role_sql)
        for db in ('auth_db', 'user_db'):
            psql('postgres', 'CREATE DATABASE ' + db + ' OWNER voice;')
            call('exec', '-i', '-e', 'PGPASSWORD=' + password, container, 'pg_restore', '-h', '127.0.0.1',
                 '-U', 'voice', '--exit-on-error', '--dbname=' + db, data=decrypt(db + '.enc'))
        def query(db, sql):
            return json.loads(psql(db, sql))
        original_catalog = {db: query(db, sql) for db, sql in backup_catalog_queries().items()}
        if original_catalog != manifest['catalog']:
            raise ValueError('restored ownership, ACL or history differs from backup')
        history_sql = "SELECT coalesce(json_agg(row_to_json(h) ORDER BY installed_rank),'[]'::json) FROM public.flyway_schema_history h"
        before_history = query('auth_db', history_sql)
        if not before_history or before_history[-1].get('version') != '26':
            raise ValueError('original Auth history is not clean Flyway 26')
        cols = query('auth_db', "SELECT coalesce(json_object_agg(c.relname,(SELECT json_agg(a.attname ORDER BY a.attnum) "
            "FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped)),'{}'::json) "
            "FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='r'")
        q = lambda sql: query('auth_db', sql)
        before_markers = marker_snapshot(q, cols, before=True)
        role_sql_query = "SELECT coalesce(json_agg(row_to_json(r) ORDER BY r.rolname),'[]'::json) FROM (SELECT rolname,rolsuper,rolinherit,rolcreaterole,rolcreatedb,rolcanlogin,rolreplication,rolbypassrls FROM pg_roles WHERE rolname !~ '^pg_' ORDER BY rolname) r"
        before_roles = query('postgres', role_sql_query)
        memberships_sql = "SELECT coalesce(json_agg(row_to_json(m) ORDER BY m.role_name,m.member_name),'[]'::json) FROM (SELECT pg_get_userbyid(roleid) role_name,pg_get_userbyid(member) member_name,admin_option,inherit_option,set_option FROM pg_auth_members) m"
        before_memberships = query('postgres', memberships_sql)
        archive = io.BytesIO()
        with tarfile.open(fileobj=archive, mode='w') as tar:
            for name, raw in sorted(sources.items()):
                info = tarfile.TarInfo('sql/' + name); info.size = len(raw); info.mode = 0o644
                tar.addfile(info, io.BytesIO(raw))
        def flyway(action, ignore_pending=False):
            args = ['-connectRetries=0', '-validateMigrationNaming=true', '-baselineOnMigrate=false',
                    '-outOfOrder=false', '-locations=filesystem:/flyway/sql', '-target=32']
            if ignore_pending:
                args.append('-ignoreMigrationPatterns=*:pending')
            cid = docker.create(QUALIFIED_TOOLS['flyway'], ['--network', 'container:' + container,
                 '-e', 'FLYWAY_USER=voice', '-e', 'FLYWAY_PASSWORD=' + password,
                 '-e', 'FLYWAY_URL=jdbc:postgresql://127.0.0.1:5432/auth_db'], args + [action])
            call('cp', '-', cid + ':/flyway', data=archive.getvalue())
            call('start', '-a', cid, timeout=240)
        flyway('validate', ignore_pending=True)
        flyway('migrate')
        flyway('validate')
        after_history = query('auth_db', history_sql)
        validate_history(before_history, after_history)
        with tempfile.TemporaryDirectory(prefix='auth-flyway-v31-') as temp:
            pinned_root = pathlib.Path(temp)
            pinned_v31 = pinned_root / 'src/backend/auth/src/main/resources/db/migration/V31__auth_identity_ddl_barrier.sql'
            pinned_v31.parent.mkdir(parents=True)
            pinned_v31.write_bytes(sources['V31__auth_identity_ddl_barrier.sql'])
            validate_ddl_gate(q(gate_bytes.decode('utf-8')), pinned_root)
        after_markers = marker_snapshot(q, cols, before=False)
        if before_markers != after_markers:
            changed = sorted(k for k in before_markers if before_markers[k] != after_markers.get(k))
            raise ValueError('original data changed outside admitted transformations: ' + ','.join(changed))
        if (before_roles != query('postgres', role_sql_query)
                or before_memberships != query('postgres', memberships_sql)):
            raise ValueError('original roles changed outside admitted transformations')
        after_catalog = {db: query(db, sql) for db, sql in backup_catalog_queries().items()}
        for db in ('auth_db', 'user_db'):
            for field in ('database_owner', 'schema_owner', 'schema_acl', 'defaults'):
                if original_catalog[db][field] != after_catalog[db][field]:
                    raise ValueError('catalog ownership or defaults changed')
            old = {o['name']: o for o in original_catalog[db]['objects']}
            new = {o['name']: o for o in after_catalog[db]['objects']}
            if any(new.get(name) != obj for name, obj in old.items()):
                raise ValueError('original relation ownership or ACL changed')
        receipt = {'protocol': 'voice-auth-flyway-clone-v1', 'source_sha': source,
            'target': 32, 'plan_sha256': admission['plan_sha256'], 'source_catalog': catalog,
            'admission_sha256': digest(admission), 'backup_binding': manifest['binding'],
            'backup_manifest_sha256': digest(manifest), 'restore_report_sha256': digest(report),
            'tools_image': tools_image, 'flyway_image': QUALIFIED_TOOLS['flyway'],
            'postgres_image': QUALIFIED_TOOLS['postgres'], 'before_history': before_history,
            'after_history': after_history, 'before_markers': before_markers, 'after_markers': after_markers,
            'original_table_columns': cols,
            'before_catalog_sha256': digest(original_catalog), 'after_catalog_sha256': digest(after_catalog),
            'roles_sha256': digest({'roles': before_roles, 'memberships': before_memberships}),
            'scope': 'isolated-clone-only',
            'coverage': 'original public ordinary-table rows/columns except Flyway ledger; V28 status/security revision and V26 authority revision normalized; selected role and catalog metadata'}
    finally:
        docker.cleanup()
    receipt['cleanup'] = 'confirmed'
    receipt['resources'] = copy.deepcopy(docker.records)
    journal.state['flyway_clone']['status'] = 'complete'
    journal.state['flyway_clone']['receipt'] = receipt
    journal.save()
    return receipt
