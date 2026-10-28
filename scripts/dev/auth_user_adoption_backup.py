"""Private encrypted backup and off-node restore qualification for adoption."""
import hashlib
import hmac
import json
import os
import re
import secrets
import socket
import subprocess
from auth_user_adoption import RecoveryRequired


def validate_restore(manifest, report, operation):
    if (manifest.get('format') != 1 or manifest.get('operation') != operation
            or report.get('protocol') != 'voice-auth-user-backup-v1'
            or report.get('operation') != operation or report.get('result') != 'PASS'
            or report.get('files') != manifest.get('files')
            or report.get('binding') != manifest.get('binding')
            or report.get('restored_catalog') != manifest.get('catalog')
            or report.get('off_node') is not True or report.get('fixture_cleanup') is not True
            or report.get('restore_exit') != {'auth_db':0,'user_db':0,'roles':0}
            or report.get('postgres_major') != 16
            or not re.fullmatch(r'[0-9a-f]{64}', str(report.get('verifier_sha256', '')))
            or not re.fullmatch(r'sha256:[0-9a-f]{64}', str(report.get('tools_image', '')))):
        raise RecoveryRequired('backup restore qualification absent or changed')
    return True


def backup_binding(intent):
    return {'resource':intent['resource'],'namespace_uid':intent['baseline']['namespace']['uid'],
            'source_sha':intent['manifest']['source_sha'],'operator_sha256':intent['manifest']['operator_sha256'],
            'products':intent['products'],'sources_sha256':intent['sources_sha256']}


def file_identity(path, key):
    sha, mac = hashlib.sha256(), hmac.new(key, digestmod=hashlib.sha256)
    with open(path, 'rb') as stream:
        for chunk in iter(lambda: stream.read(1024*1024), b''):
            sha.update(chunk); mac.update(chunk)
    return {'sha256':sha.hexdigest(),'hmac':mac.hexdigest()}


def export_backup(journal, kube, catalog):
    """One export attempt; uncertainty is never replayed automatically.

    Each database has its own consistent pg_dump snapshot. This does not claim
    a cross-service snapshot or authorize automatic restore over newer writes.
    """
    directory = journal.directory
    key = secrets.token_hex(32)
    keyfd = os.open('backup.key', os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,
                    0o600, dir_fd=journal.dirfd)
    with os.fdopen(keyfd, 'w') as stream:
        stream.write(key + '\n'); stream.flush(); os.fsync(stream.fileno())
    files = {}
    for database in ('auth_db','user_db','roles'):
        name = database + '.enc'
        fd = os.open(name, os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,
                     0o600, dir_fd=journal.dirfd)
        with os.fdopen(fd, 'wb') as output:
            encrypt = subprocess.Popen(['openssl','enc','-aes-256-cbc','-pbkdf2','-iter','200000',
                '-salt','-pass','file:'+str(directory/'backup.key')], stdin=subprocess.PIPE,
                stdout=output, stderr=subprocess.DEVNULL)
            try:
                kube.dump(database, encrypt.stdin)
                encrypt.stdin.close()
                if encrypt.wait(timeout=20) != 0:
                    raise RecoveryRequired('backup encryption failed')
                output.flush(); os.fsync(output.fileno())
            finally:
                if encrypt.poll() is None:
                    encrypt.kill(); encrypt.wait()
        files[name] = file_identity(directory/name, bytes.fromhex(key))
    manifest = {'format':1,'operation':journal.state['intent']['operation'],
                'files':files,'catalog':catalog,'origin_host':socket.gethostname(),
                'binding':backup_binding(journal.state['intent'])}
    journal.write_private('backup-manifest.json', manifest)
    return manifest


def qualified_backup(journal):
    manifest = journal.read_private('backup-manifest.json', optional=True)
    if manifest is None:
        return None
    report = journal.read_private('backup-qualified.json', optional=True)
    if report is None:
        return None
    if (manifest.get('binding')!=backup_binding(journal.state['intent'])
            or manifest.get('catalog')!=journal.state['intent']['backup_catalog']
            or manifest.get('private_fixture') or report.get('scope')!='actual-existing-dev-backup'):
        raise RecoveryRequired('backup not bound to this original DEV operation')
    keyfd = os.open('backup.key', os.O_RDONLY|os.O_NOFOLLOW, dir_fd=journal.dirfd)
    with os.fdopen(keyfd) as stream:
        journal._private(os.fstat(stream.fileno()),0o600)
        key = bytes.fromhex(stream.read().strip())
    for name, expected in manifest['files'].items():
        if name not in ('auth_db.enc','user_db.enc','roles.enc'):
            raise RecoveryRequired('unexpected backup member')
        fd = os.open(name, os.O_RDONLY|os.O_NOFOLLOW, dir_fd=journal.dirfd)
        with os.fdopen(fd, 'rb') as stream:
            journal._private(os.fstat(stream.fileno()),0o600)
        if file_identity(journal.directory/name,key) != expected:
            raise RecoveryRequired('encrypted backup integrity changed')
    validate_restore(manifest,report,journal.state['intent']['operation'])
    return {'manifest_sha256':hashlib.sha256(json.dumps(manifest,sort_keys=True).encode()).hexdigest(),
            'report_sha256':hashlib.sha256(json.dumps(report,sort_keys=True).encode()).hexdigest()}
