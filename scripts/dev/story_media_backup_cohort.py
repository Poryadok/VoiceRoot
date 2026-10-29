"""Authenticated encrypted PostgreSQL cohort primitives."""
import hashlib
import hmac
import json
import os
import re
import subprocess

import story_media_backup as backup

ALLOWED_DATABASES = frozenset(('auth_db', 'bot_db', 'chat_db', 'file_db', 'game_integration_db', 'gateway_db',
    'matchmaking_db', 'messaging_db', 'moderation_db', 'notification_db', 'realtime_db',
    'role_db', 'search_db', 'social_db', 'space_db', 'story_db', 'subscription_db', 'user_db', 'voice_db'))
DOMAIN = b'voice-postgres-cohort-backup-v1\0'


class CohortError(Exception):
    pass


def _canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True,
                      allow_nan=False).encode('ascii')


def _members_shape(binding, databases):
    """Validate archive structure; fixed callers must separately bind authority."""
    full = binding.get('full_catalog') if isinstance(binding,dict) else None
    if full is not None:
        if (not isinstance(full,dict) or set(full)!={'receipt_sha256','properties'} or
                not isinstance(full['receipt_sha256'],str) or not re.fullmatch(r'[0-9a-f]{64}',full['receipt_sha256']) or
                not isinstance(full['properties'],dict) or not 1<=len(full['properties'])<=128 or
                any(not isinstance(name,str) or not re.fullmatch(r'[a-zA-Z_][a-zA-Z0-9_]{0,62}',name) or name=='roles' or
                    not isinstance(value,dict) or type(value.get('allowconn')) is not bool
                    for name,value in full['properties'].items()) or
                databases!=sorted(name for name,value in full['properties'].items() if value['allowconn'])):
            raise CohortError('full cohort catalog invalid')
    if (not isinstance(binding, dict) or not isinstance(databases, list) or not databases
            or any(not isinstance(db, str) or (full is None and db not in ALLOWED_DATABASES) for db in databases)
            or databases != sorted(set(databases)) or binding.get('databases') != databases
            or not isinstance(binding.get('operation_id'), str)
            or not isinstance(binding.get('snapshot_id'), str)
            or not re.fullmatch(r'capture-[0-9a-f]{32}', binding['snapshot_id'])
            or not isinstance(binding.get('source_sha'), str) or not re.fullmatch(r'[0-9a-f]{40}', binding['source_sha'])):
        raise CohortError('cohort binding invalid')
    members = frozenset(databases) | {'roles'}
    sources = binding.get('sources')
    if (not isinstance(sources, dict) or set(sources) != members
            or any(not isinstance(value, dict) or not isinstance(value.get('engine'), dict)
                   or not value['engine'] or not isinstance(value.get('server_start'), str)
                   or not value['server_start'] for value in sources.values())
            or set(sources['roles']) != {'engine', 'server_start'}
            or any(set(sources[db]) != {'engine', 'server_start', 'database_oid'}
                   or type(sources[db]['database_oid']) is not int or sources[db]['database_oid'] <= 0
                   for db in databases)
            or len({sources[db]['database_oid'] for db in databases}) != len(databases)):
        raise CohortError('cohort engine binding missing')
    engine = _canonical(sources['roles']['engine'])
    start = sources['roles']['server_start']
    if any(_canonical(value['engine']) != engine or value['server_start'] != start for value in sources.values()):
        raise CohortError('multiple engines need separate role backups')
    return members


def _members(binding, databases):
    members = _members_shape(binding, databases)
    if not re.fullmatch(r'media-c[12]-[0-9a-f]{32}', binding['operation_id']):
        raise CohortError('cohort binding invalid')
    return members


def _mac(key, manifest):
    raw = _canonical(manifest)
    return hmac.new(key, DOMAIN + raw, hashlib.sha256).hexdigest()


def _export_validated(root, key, binding, databases, producer, expected_members):
    try:
        frozen = json.loads(json.dumps({'binding': binding, 'databases': databases},
                                       sort_keys=True, separators=(',', ':'), allow_nan=False))
        binding, databases = frozen['binding'], frozen['databases']
        members = _members_shape(binding, databases)
        if members != expected_members: raise CohortError('cohort member set changed')
        if not isinstance(key, bytes) or len(key) != 32 or not callable(producer):
            raise CohortError('cohort input invalid')
        files = {}; inodes = {}
        with backup.private_directory(root) as dirfd:
            for name in sorted(members):
                filename = name + '.enc'
                fd = os.open(filename, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=dirfd)
                try:
                    read_fd, write_fd = os.pipe()
                except OSError:
                    os.close(fd)
                    raise
                try:
                    os.write(write_fd, key.hex().encode() + b'\n'); os.close(write_fd); write_fd = -1
                    with os.fdopen(fd, 'w+b') as output:
                        process = subprocess.Popen(['openssl', 'enc', '-aes-256-cbc', '-pbkdf2', '-iter', '200000',
                            '-salt', '-pass', 'file:/proc/self/fd/' + str(read_fd)], stdin=subprocess.PIPE,
                            stdout=output, stderr=subprocess.DEVNULL, pass_fds=(read_fd,))
                        try:
                            producer(name, process.stdin)
                            process.stdin.close()
                            if process.wait(timeout=20) != 0:
                                raise CohortError('cohort encryption failed')
                            output.flush(); os.fsync(output.fileno())
                            info = os.fstat(output.fileno())
                            output.seek(0); files[name] = backup.stream_identity(output, key)
                            current = os.stat(filename, dir_fd=dirfd, follow_symlinks=False)
                            if (info.st_dev, info.st_ino) != (current.st_dev, current.st_ino):
                                raise CohortError('cohort member changed')
                            inodes[name] = (info.st_dev, info.st_ino)
                        finally:
                            if process.poll() is None: process.kill(); process.wait()
                            if not process.stdin.closed:
                                try: process.stdin.close()
                                except OSError: pass
                finally:
                    os.close(read_fd)
                    if write_fd >= 0: os.close(write_fd)
            for name in sorted(members):
                info = os.stat(name + '.enc', dir_fd=dirfd, follow_symlinks=False)
                if ((info.st_dev, info.st_ino) != inodes[name]
                        or backup.identity_at(dirfd, name + '.enc', key) != files[name]):
                    raise CohortError('cohort member changed')
        manifest = {'format': 1, 'binding': binding, 'files': files}
        manifest['mac'] = _mac(key, manifest)
        return manifest
    except Exception:
        raise CohortError('cohort export rejected') from None


def export(root, key, binding, databases, producer):
    """Stream the DEV-selected set; selection completeness is external."""
    try:
        frozen=json.loads(_canonical({'binding':binding,'databases':databases}))
        binding,databases=frozen['binding'],frozen['databases']
        return _export_validated(root,key,binding,databases,producer,_members(binding,databases))
    except Exception:raise CohortError('cohort export rejected') from None


def export_staging(session, binding, databases, catalog_ref, metadata_ref):
    """Use only the fixed STAGING producer after root-loaded authority validation."""
    try:
        import staging_cohort_preservation as staging
        frozen=json.loads(_canonical({'binding':binding,'databases':databases,'catalog_ref':catalog_ref,'metadata_ref':metadata_ref}))
        binding,databases,catalog_ref,metadata_ref=(frozen[k] for k in ('binding','databases','catalog_ref','metadata_ref'))
        members=staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
        root,key=staging.backup_custody(session)
        result=_export_validated(root,key,binding,databases,
            lambda member,stream:staging.produce_dump(session,member,stream),members)
        if staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)!=members:
            raise CohortError('staging cohort authority changed')
        return result
    except Exception:raise CohortError('staging cohort export rejected') from None


def _verify_validated(root, key, binding, databases, manifest, expected_members):
    try:
        snapshot = json.loads(json.dumps({'binding': binding, 'databases': databases, 'manifest': manifest},
                                         sort_keys=True, separators=(',', ':'), allow_nan=False))
        binding, databases, manifest = snapshot['binding'], snapshot['databases'], snapshot['manifest']
        members = _members_shape(binding, databases)
        if members != expected_members: raise CohortError('cohort member set changed')
        if (not isinstance(key, bytes) or len(key) != 32 or not isinstance(manifest, dict)
                or set(manifest) != {'format', 'binding', 'files', 'mac'}
                or type(manifest['format']) is not int or manifest['format'] != 1
                or _canonical(manifest['binding']) != _canonical(binding)
                or not isinstance(manifest['files'], dict) or set(manifest['files']) != members
                or not isinstance(manifest['mac'], str)):
            raise CohortError('cohort integrity failed')
        unsigned = {name: manifest[name] for name in ('format', 'binding', 'files')}
        if not hmac.compare_digest(_mac(key, unsigned), manifest['mac']):
            raise CohortError('cohort integrity failed')
        with backup.private_directory(root) as dirfd:
            for name in sorted(members):
                if backup.identity_at(dirfd, name + '.enc', key) != manifest['files'][name]:
                    raise CohortError('cohort integrity failed')
        return set(members)
    except Exception:
        raise CohortError('cohort verification rejected') from None


def verify(root, key, binding, databases, manifest):
    try:
        frozen=json.loads(_canonical({'binding':binding,'databases':databases,'manifest':manifest}))
        binding,databases,manifest=(frozen[k] for k in ('binding','databases','manifest'))
        return _verify_validated(root,key,binding,databases,manifest,_members(binding,databases))
    except Exception:raise CohortError('cohort verification rejected') from None


def verify_staging(session, binding, databases, manifest, catalog_ref, metadata_ref):
    try:
        import staging_cohort_preservation as staging
        frozen=json.loads(_canonical({'binding':binding,'databases':databases,'catalog_ref':catalog_ref,'metadata_ref':metadata_ref,'manifest':manifest}))
        binding,databases,catalog_ref,metadata_ref,manifest=(frozen[k] for k in ('binding','databases','catalog_ref','metadata_ref','manifest'))
        members=staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
        root,key=staging.backup_custody(session)
        result=_verify_validated(root,key,binding,databases,manifest,members)
        if staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)!=members:
            raise CohortError('staging cohort authority changed')
        return result
    except Exception:raise CohortError('staging cohort verification rejected') from None


def decrypt(root, key, binding, databases, manifest, member, destination):
    try:
        snapshot = json.loads(json.dumps({'binding': binding, 'databases': databases, 'manifest': manifest},
                                         sort_keys=True, separators=(',', ':'), allow_nan=False))
        binding, databases, manifest = snapshot['binding'], snapshot['databases'], snapshot['manifest']
        members = verify(root, key, binding, databases, manifest)
        if member not in members:
            raise CohortError('cohort member missing')
        with backup.private_directory(root) as dirfd:
            fd = os.open(member + '.enc', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=dirfd)
        backup._decrypt_snapshot(fd, key, manifest['files'][member], destination)
    except Exception:
        raise CohortError('cohort decryption rejected') from None


def decrypt_staging(session, member):
    """Decrypt one root-selected member to a private, single-attempt path."""
    try:
        import guard
        import staging_cohort_preservation as staging
        record=staging.load_backup_archive(session)
        if type(record) is not dict or set(record)!={'binding','databases','manifest','catalog_ref','metadata_ref'}:
            raise CohortError('staging archive invalid')
        binding,databases,manifest,catalog_ref,metadata_ref=(record[k] for k in
            ('binding','databases','manifest','catalog_ref','metadata_ref'))
        members=staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
        if member not in members:raise CohortError('staging member invalid')
        root,key=staging.backup_custody(session)
        _verify_validated(root,key,binding,databases,manifest,members)
        nonce=staging.restore_attempt(session)
        if not isinstance(nonce,str) or not re.fullmatch(r'[a-f0-9]{32}',nonce):raise CohortError('staging restore attempt invalid')
        directory=guard.ROOT/'installed'/'cohort-preservation'/session.operation/'restore-input'
        filename=member+'-'+nonce+'.plain'
        with backup.private_directory(directory) as dirfd:
            output_fd=os.open(filename,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=dirfd)
            try:
                with os.fdopen(output_fd,'wb') as output, backup.private_directory(root) as archivefd:
                    input_fd=os.open(member+'.enc',os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=archivefd)
                    backup._decrypt_snapshot(input_fd,key,manifest['files'][member],output)
                    output.flush();os.fsync(output.fileno())
                if staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)!=members:
                    raise CohortError('staging authority changed')
            except BaseException:
                os.unlink(filename,dir_fd=dirfd)
                raise
        return directory/filename
    except Exception:raise CohortError('staging cohort decryption rejected') from None
