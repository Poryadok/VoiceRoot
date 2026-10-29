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
from urllib.parse import quote

import story_media_backup_cohort as cohort
import story_media_backup_restore as media_restore
import cohort_unmanaged_actual as unmanaged

GO_SQL = re.compile(r'([0-9]{6})_[a-z0-9_]+\.(up|down)\.sql')
FLYWAY_SQL = re.compile(r'V([0-9]+)__[A-Za-z0-9_]+\.sql')
MAX_FILE = 4 * 1024 * 1024
MAX_TOTAL = 64 * 1024 * 1024
GO_HISTORY = "(SELECT json_agg(json_build_object('version',version,'dirty',dirty) ORDER BY version)\n FROM public.schema_migrations)"
AUTH_HISTORY = "(SELECT coalesce(json_agg(json_build_object('version',version,'script',script,'checksum',checksum,'success',success) ORDER BY installed_rank),'[]'::json) FROM public.flyway_schema_history)"
AUTH_CATALOG = media_restore.CATALOG.replace(GO_HISTORY, AUTH_HISTORY)
GATEWAY_HISTORY = "json_build_object('schema_migrations',to_regclass('public.schema_migrations') IS NOT NULL,'flyway_schema_history',to_regclass('public.flyway_schema_history') IS NOT NULL)"
GATEWAY_CATALOG = media_restore.CATALOG.replace(GO_HISTORY, GATEWAY_HISTORY)


class CohortRestoreError(Exception):
    pass


def restore_order(databases):
    """Restore legacy service databases before changing disposable template0."""
    return sorted(databases,key=lambda name:(name not in cohort.ALLOWED_DATABASES,name))


def classify_plan(database, plan, full_catalog=False):
    if database not in cohort.ALLOWED_DATABASES and full_catalog:
        if (isinstance(plan,dict) and plan == {'engine':'unmanaged','from':None,'to':None,'files':{}}):
            return 'restore-only'
        raise CohortRestoreError('unmanaged database plan invalid')
    if database == 'gateway_db':
        if (isinstance(plan, dict) and set(plan) == {'engine', 'from', 'to', 'files'}
                and plan['engine'] == 'unmanaged' and plan['from'] is None and plan['to'] is None
                and isinstance(plan['files'], dict) and plan['files']):
            return 'restore-only'
        raise CohortRestoreError('gateway migration plan not qualified')
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


def migration_inventory(root, databases, plans, full_catalog=False):
    try:
        if (not isinstance(databases, list) or not databases or databases != sorted(set(databases))
                or not isinstance(plans, dict) or set(plans) != set(databases)):
            raise CohortRestoreError('cohort inventory invalid')
        rootfd = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        result = {}; total = 0
        try:
            for db in databases:
                mode = classify_plan(db, plans[db],full_catalog)
                if full_catalog and db not in cohort.ALLOWED_DATABASES:
                    result[db] = {}
                    continue
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
                    elif db != 'gateway_db' and ((current, 'up') not in versions or (target, 'up') not in versions
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


def _source_shape(binding, databases, metadata, postgres_image, migrate_image):
    cohort._members_shape(binding, databases)
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
    if AUTH_CATALOG == media_restore.CATALOG or GATEWAY_CATALOG == media_restore.CATALOG:
        raise CohortRestoreError('cohort catalog query unavailable')
    for db in databases:
        full_catalog='full_catalog' in binding
        mode = classify_plan(db, metadata['migrations'][db],full_catalog)
        catalog = metadata['catalog'][db]
        if full_catalog and db not in cohort.ALLOWED_DATABASES:
            try: unmanaged.validate_source_catalog(catalog,binding['full_catalog']['properties'][db])
            except unmanaged.UnmanagedError: raise CohortRestoreError('unmanaged source catalog invalid') from None
            continue
        if not isinstance(catalog, dict) or set(catalog) != {'database_owner', 'schema_owner', 'schema_acl',
                                                             'objects', 'defaults', 'history'}:
            raise CohortRestoreError('cohort catalog invalid')
        media_restore._quote_identifier(catalog['database_owner'])
        version = metadata['migrations'][db]['from']
        history = catalog['history']
        if db == 'gateway_db':
            if (not isinstance(history, dict) or set(history) != {'schema_migrations', 'flyway_schema_history'}
                    or any(history[key] is not False for key in history)):
                raise CohortRestoreError('gateway ledger observation differs')
        elif db != 'auth_db':
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


def _source(binding, databases, metadata, postgres_image, migrate_image):
    cohort._members(binding,databases)
    return _source_shape(binding,databases,metadata,postgres_image,migrate_image)


def _decrypt_command(root, key, binding, databases, manifest, member, command):
    session=media_restore._STAGING_SESSION.get()
    if session is not None:
        import staging_cohort_preservation
        return staging_cohort_preservation.restore_member(session,member,command)
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


def _qualified_roles(phase, token, catalog_phase, binding, databases, metadata, changed):
    # Imported here because metadata itself imports this module.
    import full_cohort_backup_metadata as source
    if (type(phase) is not source.BackupMetadataPhase or phase.catalog is not catalog_phase
            or phase.fence.context != catalog_phase.fence.context or phase.lock != catalog_phase.lock
            or phase.namespace_uid != catalog_phase.namespace_uid
            or phase.fence.ledger.read() != catalog_phase.fence.ledger.read()):
        raise CohortRestoreError('migration source authority mismatch')
    result = source.verify_completed(phase, token)
    if (result['operation_id'] != binding['operation_id'] or result['source_sha'] != binding['source_sha']
            or result['databases'] != databases or result['sources'] != binding['sources']
            or result['full_catalog'] != binding['full_catalog'] or result['restore_inputs'] != binding['restore_inputs']
            or result['metadata'] != metadata or set(result['migration_roles']) != set(changed)):
        raise CohortRestoreError('migration source receipt mismatch')
    roles = result['migration_roles']
    for db in changed:
        role = roles[db]['role']
        matches = [r for r in metadata['roles']['roles'] if r['name'] == role]
        if len(matches) != 1 or matches[0]['login'] is not True:
            raise CohortRestoreError('migration login absent from source')
    if phase.fence.ledger.read() != catalog_phase.fence.ledger.read():
        raise CohortRestoreError('migration source ledger changed')
    return roles


def _qualified_staging_roles(session, binding, databases, metadata, changed, catalog_ref, metadata_ref):
    import staging_cohort_preservation as staging
    staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
    record=staging.load_metadata(session,metadata_ref)
    if record.get('source')!=metadata or set(record.get('migration_roles',{}))!=set(changed):
        raise CohortRestoreError('staging migration source receipt mismatch')
    roles=record['migration_roles']
    for db in changed:
        role=roles[db]['role']
        matches=[row for row in metadata['roles']['roles'] if row['name']==role]
        if len(matches)!=1 or matches[0]['login'] is not True:
            raise CohortRestoreError('migration login absent from source')
    staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
    return roles


def _run_go_as_role(cid, admin, db, role):
    """Run the pinned migration binary as a restored login, with a clone-only password."""
    password = secrets.token_hex(32)
    statement = 'ALTER ROLE ' + media_restore._quote_identifier(role) + " PASSWORD '" + password + "'"
    media_restore._docker('exec', '-i', cid, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1',
                         '-U', admin, '-d', db, input=statement.encode('utf8'))
    wrapper = 'IFS= read -r PGPASSWORD; export PGPASSWORD; exec "$@"'
    def run(*command):
        return media_restore._docker('exec', '-i', cid, 'sh', '-ec', wrapper, 'role-login', *command,
                                    input=(password + '\n').encode('ascii'))
    identity_sql = "SELECT json_build_array(session_user,current_user)"
    def identity():
        actual = json.loads(run('psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-h', '127.0.0.1',
                                '-U', role, '-d', db, '-c', identity_sql))
        if actual != [role, role]: raise CohortRestoreError('clone login identity mismatch')
    identity()
    url = 'postgres://' + quote(role, safe='') + '@127.0.0.1:5432/' + quote(db, safe='') + '?sslmode=disable'
    run('/tmp/voice-migrate', '-path', '/tmp/voice-cohort-migrations/' + db, '-database', url, 'up')
    identity()


def _qualify(root, key, binding, databases, manifest, source_metadata, source_root, postgres_image, migrate_image, journal,
             catalog_phase=None, catalog_token=None, metadata_phase=None, metadata_token=None, *, _staging=None):
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
        full_catalog='full_catalog' in binding
        if _staging is not None:
            import staging_cohort_preservation as staging
            session,catalog_ref,metadata_ref=_staging
            if not full_catalog:raise CohortRestoreError('staging full catalog required')
            staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
            if staging.load_metadata(session,metadata_ref).get('source')!=source_metadata:
                raise CohortRestoreError('staging metadata changed')
        elif full_catalog: unmanaged.verify_catalog_phase(catalog_phase,catalog_token,binding)
        elif catalog_phase is not None or catalog_token is not None: raise CohortRestoreError('unexpected catalog authority')
        if _staging is None:
            _source(binding, databases, source_metadata, postgres_image, migrate_image)
            cohort.verify(root, key, binding, databases, manifest)
        else:
            _source_shape(binding, databases, source_metadata, postgres_image, migrate_image)
            cohort.verify_staging(session,binding,databases,manifest,catalog_ref,metadata_ref)
        sources = migration_inventory(source_root, databases, source_metadata['migrations'],full_catalog)
        changed = [db for db in databases if classify_plan(db, source_metadata['migrations'][db],full_catalog) == 'migrate']
        if _staging is not None and 'space_db' in changed:
            raise CohortRestoreError('staging Space native migration qualification required')
        if media_restore._docker('info', '--format', '{{.OSType}}').strip() != b'linux':
            raise CohortRestoreError('Linux Docker required')
        postgres_id = media_restore._check_image(postgres_image)
        migration_roles = ((_qualified_staging_roles(session,binding,databases,source_metadata,changed,catalog_ref,metadata_ref)
                            if _staging is not None else _qualified_roles(metadata_phase, metadata_token, catalog_phase, binding, databases,
                                            source_metadata, changed)) if full_catalog and changed else {})
        if _staging is None and not (full_catalog and changed) and (metadata_phase is not None or metadata_token is not None):
            raise CohortRestoreError('unexpected migration authority')
        migrate_id = media_restore._check_image(migrate_image) if changed else None
        admin = 'voice_cohort_' + secrets.token_hex(12); password = secrets.token_hex(24)
        password_name = 'voice-cohort-password-' + secrets.token_hex(16)
        postgres = media_restore._create(journal, resources, 'postgres', postgres_id,
                                          ['-e', 'POSTGRES_USER=' + admin, '-e', 'POSTGRES_PASSWORD_FILE=/tmp/' + password_name])
        payload = io.BytesIO()
        with tarfile.open(fileobj=payload,mode='w',format=tarfile.USTAR_FORMAT) as archive:
            entry=tarfile.TarInfo(password_name);entry.mode=0o600;entry.size=len(password)
            archive.addfile(entry,io.BytesIO(password.encode('ascii')))
        media_restore._docker('cp','-',postgres+':/tmp/',input=payload.getvalue())
        if changed:
            tool_container = media_restore._create(journal, resources, 'migrate', migrate_id, ['--entrypoint', '/bin/true'])
            tool = media_restore._tool_bytes(tool_container, binding['restore_inputs']['migrate_binary_sha256'])
        media_restore._docker('start', postgres)
        if media_restore._ready_major(postgres, admin) != 16:
            raise CohortRestoreError('postgres major mismatch')
        media_restore._docker('exec',postgres,'rm','-f','--','/tmp/'+password_name)
        _decrypt_command(root, key, binding, databases, manifest, 'roles',
                         [postgres, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-U', admin, '-d', 'postgres'])
        if json.loads(media_restore._psql(postgres, admin, 'postgres', media_restore.role_catalog_sql(admin))) != source_metadata['roles']:
            raise CohortRestoreError('restored role catalog differs')
        maintenance=None
        if full_catalog:
            maintenance='voice_cohort_private_'+secrets.token_hex(16)
            if maintenance in binding['full_catalog']['properties']:
                raise CohortRestoreError('maintenance database collides with source')
            media_restore._psql(postgres,admin,'postgres','CREATE DATABASE '+media_restore._quote_identifier(maintenance)+' TEMPLATE template0')
        for db in restore_order(databases):
            if full_catalog and db not in cohort.ALLOWED_DATABASES:
                sql=lambda name,statement:media_restore._psql(postgres,admin,name,statement)
                unmanaged.prepare_disposable_template(sql,source_metadata['catalog'][db],maintenance)
                if db in ('postgres','template1'): unmanaged.prepare_disposable_database(sql,db,maintenance)
                _decrypt_command(root,key,binding,databases,manifest,db,
                    [postgres,'pg_restore','-U',admin,'--exit-on-error','--create','-d',maintenance])
                if binding['full_catalog']['properties'][db]['template']:
                    sql(maintenance,'ALTER DATABASE '+media_restore._quote_identifier(db)+' IS_TEMPLATE true')
                actual=json.loads(sql(db,unmanaged.CATALOG_SQL))
                unmanaged.restore_result(source_metadata['catalog'][db],actual)
                continue
            if full_catalog:
                _decrypt_command(root,key,binding,databases,manifest,db,
                    [postgres,'pg_restore','-U',admin,'--exit-on-error','--create','-d',maintenance])
                unmanaged.verify_restored_properties(
                    lambda name,sql:media_restore._psql(postgres,admin,name,sql),db,
                    binding['full_catalog']['properties'][db])
            else:
                owner = media_restore._quote_identifier(source_metadata['catalog'][db]['database_owner'])
                media_restore._psql(postgres, admin, 'postgres', 'CREATE DATABASE ' + db + ' OWNER ' + owner)
                _decrypt_command(root, key, binding, databases, manifest, db,
                                 [postgres, 'pg_restore', '-U', admin, '--exit-on-error', '-d', db])
            query = AUTH_CATALOG if db == 'auth_db' else GATEWAY_CATALOG if db == 'gateway_db' else media_restore.CATALOG
            if json.loads(media_restore._psql(postgres, admin, db, query)) != source_metadata['catalog'][db]:
                raise CohortRestoreError('restored database catalog differs')
        if changed:
            media_restore._docker('exec', '-i', postgres, 'sh', '-ec',
                                  'cat > /tmp/voice-migrate && chmod 0555 /tmp/voice-migrate', input=tool)
            _install_go(postgres, sources, source_metadata['migrations'], changed)
        results = {}
        for db in databases:
            plan = source_metadata['migrations'][db]
            mode = classify_plan(db, plan,full_catalog)
            if full_catalog and db not in cohort.ALLOWED_DATABASES:
                actual=json.loads(media_restore._psql(postgres,admin,db,unmanaged.CATALOG_SQL))
                unmanaged.restore_result(source_metadata['catalog'][db],actual)
                results[db]={'mode':'restore-only','from':None,'to':None}
                continue
            if mode == 'migrate':
                if full_catalog:
                    _run_go_as_role(postgres, admin, db, migration_roles[db]['role'])
                else:
                    _run_go_as_role(postgres, admin, db, admin)
            query = AUTH_CATALOG if db == 'auth_db' else GATEWAY_CATALOG if db == 'gateway_db' else media_restore.CATALOG
            actual = json.loads(media_restore._psql(postgres, admin, db, query))
            if full_catalog:
                unmanaged.verify_restored_properties(
                    lambda name,sql:media_restore._psql(postgres,admin,name,sql),db,
                    binding['full_catalog']['properties'][db])
            if mode == 'restore-only':
                if actual != source_metadata['catalog'][db]:
                    raise CohortRestoreError('unchanged catalog or ledger drift')
            elif actual['history'] != [{'version': plan['to'], 'dirty': False}]:
                raise CohortRestoreError('migrated ledger differs')
            results[db] = {'mode': mode, 'from': plan['from'], 'to': plan['to']}
        if full_catalog:
            if _staging is None:
                unmanaged.verify_catalog_phase(catalog_phase,catalog_token,binding)
                final_roles=(_qualified_roles(metadata_phase,metadata_token,catalog_phase,binding,databases,source_metadata,changed) if changed else {})
            else:
                staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
                final_roles=(_qualified_staging_roles(session,binding,databases,source_metadata,changed,catalog_ref,metadata_ref) if changed else {})
            if final_roles != migration_roles:
                raise CohortRestoreError('migration source authority changed')
        receipt = {'operation_id': binding['operation_id'], 'snapshot_id': binding['snapshot_id'], 'source_sha': binding['source_sha'],
                   'manifest_sha256': hashlib.sha256(json.dumps(manifest, sort_keys=True, separators=(',', ':'),
                       ensure_ascii=True, allow_nan=False).encode('ascii')).hexdigest(),
                   'databases': results, 'scope': 'selected-cohort-roles-catalog-and-ledger-only'}
        if migration_roles:
            receipt['migration_logins'] = {db: {'role': migration_roles[db]['role'], 'from': results[db]['from'],
                                                'to': results[db]['to']} for db in changed}
    except media_restore.RestoreUncertain:
        raise CohortRestoreError('cohort resources uncertain; retain operation') from None
    except Exception:
        raise CohortRestoreError('cohort restore incomplete') from None
    finally:
        media_restore._cleanup(journal, resources)
    receipt['cleanup'] = 'verified'
    return receipt


def qualify(root, key, binding, databases, manifest, source_metadata, source_root, postgres_image, migrate_image, journal,
            catalog_phase=None, catalog_token=None, metadata_phase=None, metadata_token=None):
    return _qualify(root,key,binding,databases,manifest,source_metadata,source_root,postgres_image,migrate_image,journal,
                    catalog_phase,catalog_token,metadata_phase,metadata_token)


def qualify_staging(session,binding,databases,manifest,catalog_ref,metadata_ref):
    """Restore only the root-loaded STAGING cohort through fixed guarded transport."""
    import staging_cohort_preservation as staging
    staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
    context=staging.restore_context(session,catalog_ref,metadata_ref)
    if type(context) is not dict or set(context)!={'root','key','source_root','postgres_image','migrate_image','journal','source_metadata'}:
        raise CohortRestoreError('staging restore context invalid')
    token=media_restore._STAGING_SESSION.set(session)
    try:
        return _qualify(context['root'],context['key'],binding,databases,manifest,context['source_metadata'],
                        context['source_root'],context['postgres_image'],context['migrate_image'],context['journal'],
                        _staging=(session,catalog_ref,metadata_ref))
    finally:
        media_restore._STAGING_SESSION.reset(token)
