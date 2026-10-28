"""Bounded isolated restore for a caller-verified PostgreSQL cohort."""
import hashlib
import io
import json
import os
import re
import secrets
import stat
import subprocess
import tarfile
import threading

import story_media_backup_cohort as cohort
import story_media_backup_restore as media_restore

GO_SQL = re.compile(r'([0-9]{6})_[a-z0-9_]+\.(up|down)\.sql')
FLYWAY_SQL = re.compile(r'V([0-9]+)__[A-Za-z0-9_]+\.sql')
MAX_FILE = 4 * 1024 * 1024
MAX_TOTAL = 64 * 1024 * 1024
GO_HISTORY = "(SELECT json_agg(json_build_object('version',version,'dirty',dirty) ORDER BY version)\n FROM schema_migrations)"
AUTH_HISTORY = "(SELECT coalesce(json_agg(json_build_object('version',version,'script',script,'checksum',checksum,'success',success) ORDER BY installed_rank),'[]'::json) FROM flyway_schema_history)"
AUTH_CATALOG = media_restore.CATALOG.replace(GO_HISTORY, AUTH_HISTORY)


class CohortRestoreError(Exception):
    pass


def classify_plan(database, plan):
    if (database not in cohort.ALLOWED_DATABASES or not isinstance(plan, dict)
            or set(plan) != {'engine', 'from', 'to', 'files'}
            or plan['engine'] != ('flyway' if database == 'auth_db' else 'golang-migrate')
            or type(plan['from']) is not int or type(plan['to']) is not int
            or plan['from'] < 1 or plan['to'] < plan['from']
            or not isinstance(plan['files'], dict) or not plan['files']):
        raise CohortRestoreError('cohort migration plan invalid')
    if plan['to'] == plan['from']:
        return 'restore-only'
    if plan['engine'] == 'flyway':
        raise CohortRestoreError('changed Flyway migrations not qualified')
    return 'migrate'


def migration_inventory(root, databases, plans):
    try:
        if (not isinstance(databases, list) or not databases or databases != sorted(set(databases))
                or not isinstance(plans, dict) or set(plans) != set(databases)):
            raise CohortRestoreError('cohort inventory invalid')
        rootfd = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        result = {}; total = 0
        try:
            for db in databases:
                mode = classify_plan(db, plans[db])
                components = (['src', 'backend', 'auth', 'src', 'main', 'resources', 'db', 'migration']
                              if db == 'auth_db' else ['src', 'backend', 'migrations', db])
                folder = os.dup(rootfd)
                try:
                    for part in components:
                        child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=folder)
                        os.close(folder); folder = child
                    found = {}; versions = set()
                    for name in os.listdir(folder):
                        if not name.endswith('.sql'):
                            continue
                        match = (FLYWAY_SQL if db == 'auth_db' else GO_SQL).fullmatch(name)
                        if not match:
                            raise CohortRestoreError('unsupported migration filename')
                        version = int(match.group(1))
                        identity = (version, match.group(2) if db != 'auth_db' else 'flyway')
                        if identity in versions:
                            raise CohortRestoreError('ambiguous migration version')
                        versions.add(identity); found[name] = identity
                    if set(found) != set(plans[db]['files']):
                        raise CohortRestoreError('migration inventory changed')
                    current, target = plans[db]['from'], plans[db]['to']
                    if db == 'auth_db':
                        if (current, 'flyway') not in versions or (target, 'flyway') not in versions:
                            raise CohortRestoreError('Flyway baseline missing')
                    elif ((current, 'up') not in versions or (target, 'up') not in versions
                          or (mode == 'migrate' and (current, 'down') not in versions)):
                        raise CohortRestoreError('migration baseline missing')
                    result[db] = {}
                    for name in sorted(found):
                        expected = plans[db]['files'][name]
                        if not isinstance(expected, str) or not re.fullmatch(r'[0-9a-f]{64}', expected):
                            raise CohortRestoreError('migration digest invalid')
                        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=folder)
                        with os.fdopen(fd, 'rb') as source:
                            info = os.fstat(source.fileno())
                            if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= MAX_FILE:
                                raise CohortRestoreError('migration file invalid')
                            raw = source.read(MAX_FILE + 1)
                        total += len(raw)
                        if total > MAX_TOTAL or len(raw) != info.st_size or hashlib.sha256(raw).hexdigest() != expected:
                            raise CohortRestoreError('migration file changed')
                        result[db][name] = raw
                finally:
                    os.close(folder)
            return result
        finally:
            os.close(rootfd)
    except Exception:
        raise CohortRestoreError('cohort source inventory rejected') from None


def _source(binding, databases, metadata, postgres_image, migrate_image):
    cohort._members(binding, databases)
    inputs = binding.get('restore_inputs')
    keys = {'catalog_sha256', 'roles_catalog_sha256', 'migration_catalog_sha256',
            'postgres_image', 'migrate_image', 'migrate_binary_sha256', 'postgres_major'}
    if (not isinstance(inputs, dict) or set(inputs) != keys
            or inputs['postgres_image'] != postgres_image or inputs['migrate_image'] != migrate_image
            or type(inputs['postgres_major']) is not int or inputs['postgres_major'] != 16
            or not isinstance(inputs['migrate_binary_sha256'], str)
            or not re.fullmatch(r'[0-9a-f]{64}', inputs['migrate_binary_sha256'])
            or not isinstance(metadata, dict) or set(metadata) != {'catalog', 'roles', 'migrations'}
            or not isinstance(metadata['catalog'], dict) or set(metadata['catalog']) != set(databases)
            or not isinstance(metadata['migrations'], dict) or set(metadata['migrations']) != set(databases)
            or not isinstance(metadata['roles'], dict) or set(metadata['roles']) != {'roles', 'memberships'}
            or not isinstance(binding.get('dump_receipts'), dict)
            or set(binding['dump_receipts']) != set(databases) | {'roles'}
            or any(not isinstance(v, str) or not re.fullmatch(r'[0-9a-f]{64}', v) for v in binding['dump_receipts'].values())):
        raise CohortRestoreError('cohort source binding invalid')
    for field, name in (('catalog_sha256', 'catalog'), ('roles_catalog_sha256', 'roles'),
                        ('migration_catalog_sha256', 'migrations')):
        raw = json.dumps(metadata[name], sort_keys=True, separators=(',', ':'), ensure_ascii=True,
                         allow_nan=False).encode('ascii')
        if not isinstance(inputs[field], str) or hashlib.sha256(raw).hexdigest() != inputs[field]:
            raise CohortRestoreError('cohort metadata mismatch')
    if AUTH_CATALOG == media_restore.CATALOG:
        raise CohortRestoreError('auth catalog query unavailable')
    for db in databases:
        mode = classify_plan(db, metadata['migrations'][db])
        catalog = metadata['catalog'][db]
        if not isinstance(catalog, dict) or set(catalog) != {'database_owner', 'schema_owner', 'schema_acl',
                                                             'objects', 'defaults', 'history'}:
            raise CohortRestoreError('cohort catalog invalid')
        media_restore._quote_identifier(catalog['database_owner'])
        version = metadata['migrations'][db]['from']
        history = catalog['history']
        if db != 'auth_db':
            if (not isinstance(history, list) or len(history) != 1 or not isinstance(history[0], dict)
                    or set(history[0]) != {'version', 'dirty'} or type(history[0]['version']) is not int
                    or history[0]['version'] != version or history[0]['dirty'] is not False):
                raise CohortRestoreError('cohort ledger invalid')
        else:
            if (not isinstance(history, list) or not history
                    or any(not isinstance(row, dict) or set(row) != {'version', 'script', 'checksum', 'success'}
                           or row['success'] is not True for row in history)):
                raise CohortRestoreError('Flyway history invalid')
            versions = [row['version'] for row in history if row['version'] is not None]
            if (not versions or any(not isinstance(v, str) or not re.fullmatch(r'[0-9]+', v) for v in versions)
                    or max(int(v) for v in versions) != version or mode != 'restore-only'):
                raise CohortRestoreError('Flyway baseline invalid')
            for row in history:
                if row['version'] is not None:
                    script = row['script']
                    match = FLYWAY_SQL.fullmatch(script) if isinstance(script, str) else None
                    if (not match or int(match.group(1)) != int(row['version'])
                            or script not in metadata['migrations'][db]['files']):
                        raise CohortRestoreError('Flyway source inventory differs')


def _decrypt_command(root, key, binding, databases, manifest, member, command):
    process = subprocess.Popen(['docker', 'exec', '-i', *command], stdin=subprocess.PIPE,
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    def expire():
        try: process.kill()
        except OSError: pass
    timer = threading.Timer(120, expire); timer.start()
    try:
        cohort.decrypt(root, key, binding, databases, manifest, member, process.stdin)
        process.stdin.close()
        if process.wait(timeout=120) != 0:
            raise CohortRestoreError('cohort restore failed')
    finally:
        timer.cancel()
        if process.poll() is None: process.kill(); process.wait()
        if not process.stdin.closed:
            try: process.stdin.close()
            except OSError: pass


def _install_go(cid, sources, plans, changed):
    payload = io.BytesIO()
    with tarfile.open(fileobj=payload, mode='w', format=tarfile.USTAR_FORMAT) as archive:
        for db in changed:
            plan = plans[db]
            for name, raw in sorted(sources[db].items()):
                version = int(GO_SQL.fullmatch(name).group(1))
                if plan['from'] <= version <= plan['to']:
                    entry = tarfile.TarInfo(db + '/' + name); entry.mode = 0o444; entry.size = len(raw)
                    archive.addfile(entry, io.BytesIO(raw))
    media_restore._docker('exec', '-i', cid, 'sh', '-ec',
        'mkdir -p /tmp/voice-cohort-migrations && tar -xf - -C /tmp/voice-cohort-migrations', input=payload.getvalue())


def qualify(root, key, binding, databases, manifest, source_metadata, source_root, postgres_image, migrate_image, journal):
    """Restore a caller-authorized exact set; this does not prove its completeness."""
    if any(not callable(getattr(journal, name, None)) for name in ('begin', 'created', 'begin_remove', 'removed', 'uncertain')):
        raise CohortRestoreError('cohort resource journal required')
    resources = []
    try:
        raw = json.dumps({'binding': binding, 'databases': databases, 'manifest': manifest,
                          'metadata': source_metadata}, sort_keys=True, separators=(',', ':'),
                         ensure_ascii=True, allow_nan=False).encode('ascii')
        if len(raw) > 16 * 1024 * 1024: raise CohortRestoreError('cohort metadata exceeds limit')
        snapshot = json.loads(raw)
        binding, databases, manifest, source_metadata = (snapshot['binding'], snapshot['databases'],
                                                         snapshot['manifest'], snapshot['metadata'])
        _source(binding, databases, source_metadata, postgres_image, migrate_image)
        cohort.verify(root, key, binding, databases, manifest)
        sources = migration_inventory(source_root, databases, source_metadata['migrations'])
        if media_restore._docker('info', '--format', '{{.OSType}}').strip() != b'linux':
            raise CohortRestoreError('Linux Docker required')
        postgres_id = media_restore._check_image(postgres_image)
        changed = [db for db in databases if classify_plan(db, source_metadata['migrations'][db]) == 'migrate']
        migrate_id = media_restore._check_image(migrate_image) if changed else None
        admin = 'voice_cohort_' + secrets.token_hex(12); password = secrets.token_hex(24)
        postgres = media_restore._create(journal, resources, 'postgres', postgres_id,
                                          ['-e', 'POSTGRES_USER=' + admin, '-e', 'POSTGRES_PASSWORD=' + password])
        if changed:
            tool_container = media_restore._create(journal, resources, 'migrate', migrate_id, ['--entrypoint', '/bin/true'])
            tool = media_restore._tool_bytes(tool_container, binding['restore_inputs']['migrate_binary_sha256'])
        media_restore._docker('start', postgres)
        if media_restore._ready_major(postgres, admin) != 16:
            raise CohortRestoreError('postgres major mismatch')
        _decrypt_command(root, key, binding, databases, manifest, 'roles',
                         [postgres, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-U', admin, '-d', 'postgres'])
        if json.loads(media_restore._psql(postgres, admin, 'postgres', media_restore.role_catalog_sql(admin))) != source_metadata['roles']:
            raise CohortRestoreError('restored role catalog differs')
        for db in databases:
            owner = media_restore._quote_identifier(source_metadata['catalog'][db]['database_owner'])
            media_restore._psql(postgres, admin, 'postgres', 'CREATE DATABASE ' + db + ' OWNER ' + owner)
            _decrypt_command(root, key, binding, databases, manifest, db,
                             [postgres, 'pg_restore', '-U', admin, '--exit-on-error', '-d', db])
            query = AUTH_CATALOG if db == 'auth_db' else media_restore.CATALOG
            if json.loads(media_restore._psql(postgres, admin, db, query)) != source_metadata['catalog'][db]:
                raise CohortRestoreError('restored database catalog differs')
        if changed:
            media_restore._docker('exec', '-i', postgres, 'sh', '-ec',
                                  'cat > /tmp/voice-migrate && chmod 0555 /tmp/voice-migrate', input=tool)
            _install_go(postgres, sources, source_metadata['migrations'], changed)
        results = {}
        for db in databases:
            plan = source_metadata['migrations'][db]
            mode = classify_plan(db, plan)
            if mode == 'migrate':
                url = 'postgres://' + admin + ':' + password + '@127.0.0.1:5432/' + db + '?sslmode=disable'
                command = 'exec /tmp/voice-migrate -path /tmp/voice-cohort-migrations/' + db + ' -database "$VOICE_RESTORE_URL" up'
                media_restore._docker('exec', '-e', 'VOICE_RESTORE_URL=' + url, postgres, 'sh', '-ec', command)
            query = AUTH_CATALOG if db == 'auth_db' else media_restore.CATALOG
            actual = json.loads(media_restore._psql(postgres, admin, db, query))
            if mode == 'restore-only':
                if actual != source_metadata['catalog'][db]:
                    raise CohortRestoreError('unchanged catalog or ledger drift')
            elif actual['history'] != [{'version': plan['to'], 'dirty': False}]:
                raise CohortRestoreError('migrated ledger differs')
            results[db] = {'mode': mode, 'from': plan['from'], 'to': plan['to']}
        receipt = {'operation_id': binding['operation_id'], 'snapshot_id': binding['snapshot_id'], 'source_sha': binding['source_sha'],
                   'manifest_sha256': hashlib.sha256(json.dumps(manifest, sort_keys=True, separators=(',', ':'),
                       ensure_ascii=True, allow_nan=False).encode('ascii')).hexdigest(),
                   'databases': results, 'scope': 'selected-cohort-roles-catalog-and-ledger-only'}
    except media_restore.RestoreUncertain:
        raise CohortRestoreError('cohort resources uncertain; retain operation') from None
    except Exception:
        raise CohortRestoreError('cohort restore incomplete') from None
    finally:
        media_restore._cleanup(journal, resources)
    receipt['cleanup'] = 'verified'
    return receipt
