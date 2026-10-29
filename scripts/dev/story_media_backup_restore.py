"""Isolated media backup restore and exact migration qualification."""
import hashlib
import contextvars
import io
import json
import os
import re
import secrets
import stat
import subprocess
import tarfile
import threading
import time
import uuid

import story_media_backup as backup

_STAGING_SESSION=contextvars.ContextVar('voice_staging_restore_session',default=None)

DATABASES = ('file_db', 'story_db', 'messaging_db')
SQL_NAME = re.compile(r'([0-9]{6})_[a-z0-9_]+\.(up|down)\.sql')
MAX_SQL_BYTES = 4 * 1024 * 1024
MAX_TOOL_BYTES = 64 * 1024 * 1024
CATALOG = """SELECT json_build_object(
 'database_owner',(SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname=current_database()),
 'schema_owner',(SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname='public'),
 'schema_acl',(SELECT nspacl::text FROM pg_namespace WHERE nspname='public'),
 'objects',(SELECT coalesce(json_agg(json_build_object('name',c.relname,'kind',c.relkind,
 'owner',pg_get_userbyid(c.relowner),'acl',c.relacl::text) ORDER BY c.relname),'[]'::json)
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relkind IN('r','p','S','v','m')),
 'defaults',(SELECT coalesce(json_agg(json_build_object('owner',pg_get_userbyid(d.defaclrole),
 'schema',n.nspname,'type',d.defaclobjtype,'acl',d.defaclacl::text)
 ORDER BY pg_get_userbyid(d.defaclrole),n.nspname,d.defaclobjtype),'[]'::json)
 FROM pg_default_acl d LEFT JOIN pg_namespace n ON n.oid=d.defaclnamespace),
 'history',(SELECT json_agg(json_build_object('version',version,'dirty',dirty) ORDER BY version)
 FROM public.schema_migrations));"""


def role_catalog_sql(excluded=''):
    if not isinstance(excluded, str) or '\0' in excluded:
        raise RestoreError('role catalog selector invalid')
    literal = "'" + excluded.replace("'", "''") + "'"
    return """SELECT json_build_object(
      'roles',(SELECT coalesce(json_agg(json_build_object('name',rolname,'super',rolsuper,
       'inherit',rolinherit,'create_role',rolcreaterole,'create_db',rolcreatedb,
       'login',rolcanlogin,'replication',rolreplication,'bypass_rls',rolbypassrls,
       'connection_limit',rolconnlimit,'valid_until',rolvaliduntil) ORDER BY rolname),'[]'::json)
       FROM pg_roles WHERE left(rolname,3)<>'pg_' AND rolname<>EXCLUDED),
      'memberships',(SELECT coalesce(json_agg(json_build_object('role',r.rolname,'member',m.rolname,
       'admin',a.admin_option,'inherit',a.inherit_option,'set',a.set_option)
       ORDER BY r.rolname,m.rolname),'[]'::json)
       FROM pg_auth_members a JOIN pg_roles r ON r.oid=a.roleid JOIN pg_roles m ON m.oid=a.member
       WHERE m.rolname<>EXCLUDED));""".replace('EXCLUDED', literal)


class RestoreError(Exception):
    pass


def migration_sources(root, plans):
    try:
        if not isinstance(plans, dict) or set(plans) != set(DATABASES):
            raise RestoreError('migration plan invalid')
        rootfd = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            result = {}
            for db in DATABASES:
                plan = plans[db]
                if (set(plan) != {'from', 'to', 'files'} or type(plan['from']) is not int
                        or type(plan['to']) is not int or plan['from'] < 0 or plan['to'] <= plan['from']
                        or not isinstance(plan['files'], dict) or not plan['files']):
                    raise RestoreError('migration plan invalid')
                folder = os.open(db, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=rootfd)
                try:
                    selected = {}
                    versions = set()
                    for name in os.listdir(folder):
                        match = SQL_NAME.fullmatch(name)
                        if not match:
                            continue
                        version = int(match.group(1))
                        if not plan['from'] <= version <= plan['to']:
                            continue
                        identity = (version, match.group(2))
                        if identity in versions:
                            raise RestoreError('migration inventory ambiguous')
                        versions.add(identity)
                        selected[name] = version
                    if (set(selected) != set(plan['files']) or (plan['from'], 'down') not in versions
                            or (plan['to'], 'up') not in versions):
                        raise RestoreError('migration inventory changed')
                    result[db] = {}
                    for name in sorted(selected):
                        expected = plan['files'][name]
                        if not isinstance(expected, str) or not re.fullmatch(r'[0-9a-f]{64}', expected):
                            raise RestoreError('migration digest invalid')
                        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=folder)
                        with os.fdopen(fd, 'rb') as source:
                            info = os.fstat(source.fileno())
                            if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= MAX_SQL_BYTES:
                                raise RestoreError('migration file invalid')
                            raw = source.read(MAX_SQL_BYTES + 1)
                        if len(raw) != info.st_size or hashlib.sha256(raw).hexdigest() != expected:
                            raise RestoreError('migration file changed')
                        result[db][name] = raw
                finally:
                    os.close(folder)
            return result
        finally:
            os.close(rootfd)
    except Exception:
        raise RestoreError('migration inputs invalid') from None


class RestoreUncertain(RestoreError):
    pass


def _quote_identifier(value):
    if not isinstance(value, str) or not value or '\0' in value or len(value.encode('utf-8')) > 63:
        raise RestoreError('database owner invalid')
    return '"' + value.replace('"', '""') + '"'


def _docker(*args, input=None, timeout=120):
    session=_STAGING_SESSION.get()
    if session is not None:
        import staging_cohort_preservation
        return staging_cohort_preservation.restore_command(session,['docker',*args],body=b'' if input is None else input,timeout=timeout)
    result = subprocess.run(['docker', *args], input=input, stdout=subprocess.PIPE,
                            stderr=subprocess.DEVNULL, timeout=timeout)
    if result.returncode:
        raise RestoreError('isolated restore operation failed')
    return result.stdout


def _uncertain(journal, kind, nonce):
    try:
        journal.uncertain(kind, nonce)
    except BaseException:
        pass


def _create(journal, resources, kind, image, arguments):
    name = 'voice-media-restore-' + uuid.uuid4().hex
    label = 'voice.test.owner=' + name
    nonce = None
    try:
        nonce = journal.begin(kind, name, {'voice.test.owner': name})
        if not isinstance(nonce, str) or not re.fullmatch(r'[0-9a-f]{32}', nonce):
            raise RestoreError('restore attempt identity invalid')
        cid = _docker('create', '--name', name, '--network', 'none', '--label', label,
                      *arguments, image).decode('ascii').strip()
        if not re.fullmatch(r'[0-9a-f]{64}', cid):
            raise RestoreError('restore fixture identity unknown')
        info = json.loads(_docker('inspect', cid))[0]
        if (info.get('Id') != cid or info.get('Image') != image
                or info.get('Config', {}).get('Labels', {}).get('voice.test.owner') != name
                or info.get('HostConfig', {}).get('NetworkMode') != 'none'
                or info.get('HostConfig', {}).get('PortBindings')
                or any(m.get('Type') != 'volume' for m in info.get('Mounts', []))):
            raise RestoreError('restore fixture ownership unverified')
        volumes = [m['Name'] for m in info.get('Mounts', [])]
        if kind == 'postgres' and (len(volumes) != 1 or info['Mounts'][0].get('Destination') != '/var/lib/postgresql/data'):
            raise RestoreError('restore volume isolation invalid')
        if kind == 'migrate' and volumes:
            raise RestoreError('restore tool isolation invalid')
        journal.created(kind, nonce, cid, {'voice.test.owner': name})
        resources.append((kind, nonce, cid, name, volumes))
        return cid
    except BaseException:
        _uncertain(journal, kind, nonce)
        raise RestoreUncertain('restore resource ownership uncertain') from None


def _cleanup(journal, resources):
    errors = False
    for kind, nonce, cid, name, volumes in reversed(resources):
        try:
            info = json.loads(_docker('inspect', cid))[0]
            if info.get('Id') != cid or info.get('Config', {}).get('Labels', {}).get('voice.test.owner') != name:
                raise RestoreError('restore cleanup ownership changed')
            journal.begin_remove(kind, cid)
            _docker('rm', '-f', '-v', cid)
            remaining = set(_docker('ps', '-a', '--no-trunc', '--format', '{{.ID}}').decode().splitlines())
            volume_names = set(_docker('volume', 'ls', '--format', '{{.Name}}').decode().splitlines())
            if cid in remaining or set(volumes).intersection(volume_names):
                raise RestoreError('restore cleanup incomplete')
            journal.removed(kind, cid, True)
        except BaseException:
            errors = True
            _uncertain(journal, kind, nonce)
    if errors:
        raise RestoreUncertain('restore cleanup uncertain')


def _psql(cid, admin, database, sql, timeout=120):
    return _docker('exec', '-i', '-e', 'PGOPTIONS=-c search_path=pg_catalog,public', cid, 'psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1',
                   '-U', admin, '-d', database, input=sql.encode('utf-8'), timeout=timeout)


def _ready_major(cid, admin):
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        try:
            script = ('export PGPASSWORD="$POSTGRES_PASSWORD"; '
                      'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$1" -d postgres -c "SHOW server_version_num"')
            raw = _docker('exec', cid, 'sh', '-ec', script, '--', admin, timeout=3).strip()
            if not re.fullmatch(rb'[0-9]{6}', raw):
                raise RestoreError('restore version invalid')
            return int(raw) // 10000
        except (RestoreError, subprocess.TimeoutExpired):
            time.sleep(.25)
    raise RestoreError('restore database unavailable')


def _decrypt_to_command(root, key, binding, manifest, member, command):
    process = subprocess.Popen(['docker', 'exec', '-i', *command], stdin=subprocess.PIPE,
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    def expire():
        try:
            process.kill()
        except OSError:
            pass
    timer = threading.Timer(120, expire)
    timer.start()
    try:
        backup.decrypt(root, key, binding, manifest, member, process.stdin)
        process.stdin.close()
        if process.wait(timeout=120) != 0:
            raise RestoreError('isolated database restore failed')
    finally:
        timer.cancel()
        if process.poll() is None:
            process.kill(); process.wait()
        if not process.stdin.closed:
            try:
                process.stdin.close()
            except OSError:
                pass


def _source_binding(binding, metadata, postgres_image, migrate_image):
    inputs = binding.get('restore_inputs', {}) if isinstance(binding, dict) else {}
    image_pattern = r'(?:sha256:[0-9a-f]{64}|[a-z0-9][a-z0-9._/:\-]*@sha256:[0-9a-f]{64})'
    if (not isinstance(binding, dict) or not re.fullmatch(r'[0-9a-f]{40}', str(binding.get('source_sha', '')))
            or not re.fullmatch(r'media-c[12]-[0-9a-f]{32}', str(binding.get('operation_id', '')))
            or not re.fullmatch(r'capture-[0-9a-f]{32}', str(binding.get('snapshot_id', '')))
            or not re.fullmatch(image_pattern, str(postgres_image)) or not re.fullmatch(image_pattern, str(migrate_image))
            or not isinstance(inputs, dict) or set(inputs) != {'catalog_sha256', 'roles_catalog_sha256',
                'migration_catalog_sha256', 'postgres_image', 'migrate_image', 'migrate_binary_sha256', 'postgres_major'}
            or inputs['postgres_image'] != postgres_image or inputs['migrate_image'] != migrate_image
            or not re.fullmatch(r'[0-9a-f]{64}', str(inputs['migrate_binary_sha256']))
            or type(inputs['postgres_major']) is not int or inputs['postgres_major'] != 16
            or not isinstance(binding.get('dump_receipts'), dict) or set(binding['dump_receipts']) != backup.MEMBERS
            or any(not isinstance(x, str) or not re.fullmatch(r'[0-9a-f]{64}', x) for x in binding['dump_receipts'].values())
            or not isinstance(metadata, dict) or set(metadata) != {'catalog', 'roles', 'migrations'}
            or not isinstance(metadata['roles'], dict) or set(metadata['roles']) != {'roles', 'memberships'}
            or not isinstance(metadata['catalog'], dict) or set(metadata['catalog']) != set(DATABASES)
            or not isinstance(metadata['migrations'], dict) or set(metadata['migrations']) != set(DATABASES)):
        raise RestoreError('authenticated restore binding invalid')
    for field, metadata_key in (('catalog_sha256', 'catalog'), ('roles_catalog_sha256', 'roles'),
                                ('migration_catalog_sha256', 'migrations')):
        encoded = json.dumps(metadata[metadata_key], sort_keys=True, separators=(',', ':'),
                             ensure_ascii=True, allow_nan=False).encode('ascii')
        if (not isinstance(inputs[field], str) or not re.fullmatch(r'[0-9a-f]{64}', inputs[field])
                or hashlib.sha256(encoded).hexdigest() != inputs[field]):
            raise RestoreError('authenticated source metadata changed')
    for db in DATABASES:
        catalog = metadata['catalog'][db]
        plan = metadata['migrations'][db]
        if (not isinstance(catalog, dict) or set(catalog) != {'database_owner', 'schema_owner', 'schema_acl',
                                                              'objects', 'defaults', 'history'}
                or not isinstance(plan, dict) or set(plan) != {'from', 'to', 'files'}
                or catalog['history'] != [{'version': plan['from'], 'dirty': False}]):
            raise RestoreError('authenticated source catalog invalid')
        _quote_identifier(catalog['database_owner'])


def _check_image(image):
    info = json.loads(_docker('image', 'inspect', image))[0]
    image_id = info.get('Id', '')
    if (not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id) or info.get('Os') != 'linux'
            or (image.startswith('sha256:') and image_id != image)
            or (not image.startswith('sha256:') and image not in (info.get('RepoDigests') or []))):
        raise RestoreError('restore image identity changed')
    return image_id


def _tool_bytes(cid, expected_sha):
    raw = _docker('cp', cid + ':/usr/local/bin/migrate', '-', timeout=120)
    if len(raw) > MAX_TOOL_BYTES + 10240:
        raise RestoreError('migration tool exceeds limit')
    with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
        items = archive.getmembers()
        if len(items) != 1 or items[0].name != 'migrate' or not items[0].isfile() or not 0 < items[0].size <= MAX_TOOL_BYTES:
            raise RestoreError('migration tool archive invalid')
        tool = archive.extractfile(items[0]).read(MAX_TOOL_BYTES + 1)
    if len(tool) != items[0].size or hashlib.sha256(tool).hexdigest() != expected_sha:
        raise RestoreError('migration tool identity changed')
    return tool


def _install_migrations(cid, sources):
    payload = io.BytesIO()
    with tarfile.open(fileobj=payload, mode='w', format=tarfile.USTAR_FORMAT) as archive:
        for db in DATABASES:
            for name, raw in sorted(sources[db].items()):
                item = tarfile.TarInfo(db + '/' + name)
                item.mode = 0o444; item.size = len(raw)
                archive.addfile(item, io.BytesIO(raw))
    _docker('exec', '-i', cid, 'sh', '-ec',
            'mkdir -p /tmp/voice-migrations && tar -xf - -C /tmp/voice-migrations', input=payload.getvalue())


def qualify_restored_set(root, key, binding, manifest, source_metadata, migration_root, postgres_image, migrate_image, journal):
    """Restore and migrate in owned Linux Docker containers, without live writes.

    Binding, catalog and SQL hashes must have been captured under the real
    operation authority and authenticated by the backup manifest. This result
    does not attest off-node custody, cross-service invariants or app readiness.
    """
    required = ('begin', 'created', 'begin_remove', 'removed', 'uncertain')
    if any(not callable(getattr(journal, name, None)) for name in required):
        raise RestoreError('restore resource journal required')
    resources = []
    try:
        encoded = json.dumps({'binding': binding, 'manifest': manifest, 'metadata': source_metadata},
                             sort_keys=True, separators=(',', ':'), ensure_ascii=True, allow_nan=False).encode('ascii')
        if len(encoded) > 16 * 1024 * 1024:
            raise RestoreError('source metadata exceeds limit')
        snapshot = json.loads(encoded)
        binding, manifest, source_metadata = snapshot['binding'], snapshot['manifest'], snapshot['metadata']
        _source_binding(binding, source_metadata, postgres_image, migrate_image)
        backup.verify(root, key, binding, manifest)
        sources = migration_sources(migration_root, source_metadata['migrations'])
        if _docker('info', '--format', '{{.OSType}}').strip() != b'linux':
            raise RestoreError('Linux Docker required')
        postgres_id = _check_image(postgres_image); migrate_id = _check_image(migrate_image)
        admin = 'voice_restore_' + secrets.token_hex(12)
        password = secrets.token_hex(24)
        postgres = _create(journal, resources, 'postgres', postgres_id,
                           ['-e', 'POSTGRES_USER=' + admin, '-e', 'POSTGRES_PASSWORD=' + password])
        tool_container = _create(journal, resources, 'migrate', migrate_id, ['--entrypoint', '/bin/true'])
        tool = _tool_bytes(tool_container, binding['restore_inputs']['migrate_binary_sha256'])
        _docker('start', postgres)
        major = _ready_major(postgres, admin)
        if major != binding['restore_inputs']['postgres_major']:
            raise RestoreError('restore postgres major mismatch')
        _decrypt_to_command(root, key, binding, manifest, 'roles',
                            [postgres, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-U', admin, '-d', 'postgres'])
        if json.loads(_psql(postgres, admin, 'postgres', role_catalog_sql(admin))) != source_metadata['roles']:
            raise RestoreError('restored roles differ')
        for db in DATABASES:
            owner = _quote_identifier(source_metadata['catalog'][db]['database_owner'])
            _psql(postgres, admin, 'postgres', 'CREATE DATABASE ' + db + ' OWNER ' + owner)
            _decrypt_to_command(root, key, binding, manifest, db,
                                [postgres, 'pg_restore', '-U', admin, '--exit-on-error', '-d', db])
            actual = json.loads(_psql(postgres, admin, db, CATALOG))
            if actual != source_metadata['catalog'][db]:
                raise RestoreError('restored catalog differs')
        _docker('exec', '-i', postgres, 'sh', '-ec', 'cat > /tmp/voice-migrate && chmod 0555 /tmp/voice-migrate', input=tool)
        _install_migrations(postgres, sources)
        versions = {}
        for db in DATABASES:
            url = 'postgres://' + admin + ':' + password + '@127.0.0.1:5432/' + db + '?sslmode=disable'
            command = 'exec /tmp/voice-migrate -path /tmp/voice-migrations/' + db + ' -database "$VOICE_RESTORE_URL" up'
            _docker('exec', '-e', 'VOICE_RESTORE_URL=' + url, postgres, 'sh', '-ec', command)
            history = json.loads(_psql(postgres, admin, db,
                "SELECT json_agg(json_build_object('version',version,'dirty',dirty) ORDER BY version) FROM schema_migrations"))
            if history != [{'version': source_metadata['migrations'][db]['to'], 'dirty': False}]:
                raise RestoreError('postmigration ledger differs')
            versions[db] = history[0]['version']
        receipt = {'operation_id': binding['operation_id'], 'snapshot_id': binding['snapshot_id'],
                   'source_sha': binding['source_sha'], 'postgres_image': postgres_image, 'migrate_image': migrate_image,
                   'postgres_image_id': postgres_id, 'migrate_image_id': migrate_id,
                   'manifest_sha256': hashlib.sha256(json.dumps(manifest, sort_keys=True, separators=(',', ':'),
                                                                ensure_ascii=True, allow_nan=False).encode('ascii')).hexdigest(),
                   'migrate_binary_sha256': binding['restore_inputs']['migrate_binary_sha256'], 'versions': versions,
                   'scope': 'isolated-roles-catalog-and-migrations-only'}
    except RestoreUncertain:
        raise
    except Exception:
        raise RestoreError('isolated restore incomplete') from None
    finally:
        _cleanup(journal, resources)
    receipt['cleanup'] = 'verified'
    return receipt
