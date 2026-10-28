"""Stage the owned encrypted DEV backup for one GitHub Actions upload.

The invoking master workflow and its GitHub environment are trusted. The
operation must already own the DEV claim, common lock and permanent ledger.
This command never uploads keys or kubeconfig and never retries an effect.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys

import media_backup_upload_run
import media_operation_guard
import publish_candidate
import resource_fencing
import story_media_backup as backup
import story_media_backup_custody as custody

ROOT = Path('/home/pmd/voice-dev-media-operations')
TT = 'http://192.168.0.109:30990'
LIMIT = 64 * 1024 ** 3
CHUNK = 1024 * 1024
OPERATOR_UID = 1000


class Refused(RuntimeError):
    pass


class Client:
    def call(self, *args, body=None):
        payload = None if body is None else json.dumps(body).encode()
        if args and args[0] == 'patch' and '--patch-file=-' in args:
            if not isinstance(body, list):
                raise Refused('invalid ledger patch')
            args = tuple('--patch=' + json.dumps(body) if arg == '--patch-file=-' else arg for arg in args)
            payload = None
        cmd = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
               '--namespace', 'voice-dev', '--request-timeout=15s', *args]
        result = subprocess.run(cmd, input=payload, capture_output=True, timeout=20)
        if result.returncode:
            raise Refused('Kubernetes request failed')
        return json.loads(result.stdout) if result.stdout.strip() else {}


def private_file(directory, name, limit):
    with backup.private_directory(directory) as dirfd:
        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=dirfd)
        with os.fdopen(fd, 'rb') as stream:
            info = os.fstat(stream.fileno())
            if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or
                    stat.S_IMODE(info.st_mode) != 0o600 or info.st_nlink != 1 or
                    not 0 < info.st_size <= limit):
                raise Refused('private input custody invalid')
            raw = stream.read(limit + 1)
            if len(raw) != info.st_size or len(raw) > limit:
                raise Refused('private input changed')
            final = os.fstat(stream.fileno())
            if (final.st_dev, final.st_ino, final.st_size, final.st_mtime_ns) != (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns):
                raise Refused('private input changed')
            return raw


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise Refused('duplicate private input key')
        result[key] = value
    return result


def stage(environment, operation, challenge, repo_root):
    if (os.geteuid() != OPERATOR_UID or not isinstance(operation, str) or
            not re.fullmatch(r'media-c1-[0-9a-f]{32}', operation) or
            not isinstance(challenge, str) or not re.fullmatch(r'[0-9a-f]{32}', challenge)):
        raise Refused('fixed Linux operator identity required')
    source = environment.get('GITHUB_SHA', '')
    publish_candidate.source_snapshot(Path(repo_root), source)
    operation_dir = ROOT / operation
    raw_state = private_file(operation_dir / 'journal', 'state.json', 1048576)
    state = json.loads(raw_state, object_pairs_hook=unique_object)
    intent = state['intent']
    context = intent['resource_context']
    namespace_uid = intent['namespace_uid']
    lock = intent['kube_lock']
    planned = state['steps']['backup-upload']
    if (set(state) != {'format', 'intent', 'steps'} or state.get('format') != 1 or
            context['operation_id'] != operation or planned.get('status') != 'dispatch-pending' or
            planned['operation_id'] != operation or planned['challenge'] != challenge or
            planned['workflow_sha'] != source):
        raise Refused('operation journal differs from requested upload')
    client = Client()
    checker = resource_fencing.ResourceCheck(TT, context)
    fence = resource_fencing.Fence(context, checker, resource_fencing.KubeLedger(client), checker.mark_uncertain)
    admitted = media_backup_upload_run.claim(fence, client, lock, namespace_uid, environment, challenge)
    try:
        original = {k: v for k, v in admitted.items() if k not in ('run_id', 'run_attempt')}
        original['status'] = 'dispatch-pending'
        if original != planned:
            raise Refused('journal and run reservation differ')
        row, _ = fence.ledger.read()
        digests = {}
        for member in ('file_db', 'story_db', 'messaging_db', 'roles'):
            saved = state['steps']['dump-' + member]
            receipt = saved['receipt']
            digest = hashlib.sha256(json.dumps(receipt, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            ledger = row['backup_members'][member]
            expected_receipt = intent['backup_capture'] | {'operation_id': operation, 'member': member,
                               'exit': 0, 'scope': 'dump-terminal-only', 'attempt_nonce': saved.get('nonce')}
            expected_ledger = {'status': 'done', 'nonce': saved.get('nonce'),
                               'snapshot_id': intent['backup_capture']['snapshot_id'],
                               'operation_id': operation, 'receipt_sha256': digest}
            if (saved.get('status') != 'done' or receipt != expected_receipt or
                    type(receipt.get('exit')) is not int or type(receipt.get('restart_count')) is not int or
                    ledger != expected_ledger):
                raise Refused('dump receipt changed')
            digests[member] = digest
        binding = intent['backup_binding'] | {'dump_receipts': digests}
        key = private_file(operation_dir, 'backup.key', 32)
        if len(key) != 32:
            raise Refused('private recovery key invalid')
        original = operation_dir / 'backup' / 'media-backup.tar'
        expected = {'bytes': planned['bundle_bytes'], 'sha256': planned['bundle_sha256']}
        if custody.verify_bundle(original, key, binding) != expected:
            raise Refused('original bundle identity changed')
        scratch = operation_dir / ('upload-' + admitted['run_id'] + '-' + challenge)
        scratch.mkdir(mode=0o700)
        with backup.private_directory(operation_dir) as operation_fd:
            os.fsync(operation_fd)
        target = scratch / 'media-backup.tar'
        with backup.private_directory(original.parent) as source_dir, backup.private_directory(scratch) as target_dir:
            fd = os.open(original.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=source_dir)
            out_fd = os.open(target.name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=target_dir)
            with os.fdopen(fd, 'rb') as src, os.fdopen(out_fd, 'wb') as dst:
                before = os.fstat(src.fileno())
                if (not stat.S_ISREG(before.st_mode) or before.st_uid != os.geteuid() or
                        stat.S_IMODE(before.st_mode) != 0o600 or before.st_nlink != 1 or
                        before.st_size != planned['bundle_bytes']):
                    raise Refused('source bundle custody invalid')
                digest = hashlib.sha256(); count = 0
                for chunk in iter(lambda: src.read(CHUNK), b''):
                    count += len(chunk)
                    if count > LIMIT:
                        raise Refused('bundle too large')
                    digest.update(chunk); dst.write(chunk)
                dst.flush(); os.fsync(dst.fileno())
                after = os.fstat(src.fileno())
                if ((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) !=
                        (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) or
                        count != expected['bytes'] or digest.hexdigest() != expected['sha256']):
                    raise Refused('source bundle changed while copying')
            os.fsync(target_dir)
        if custody.verify_bundle(target, key, binding) != expected:
            raise Refused('staged bundle authentication failed')
        if private_file(operation_dir / 'journal', 'state.json', 1048576) != raw_state:
            raise Refused('operation journal changed')
        media_operation_guard.verify(fence, client, lock, namespace_uid)
        final, final_revision = fence.ledger.read()
        if (not fence.owned(final) or final.get('status') != 'active' or
                final.get('backup_invocation') != {'nonce': admitted['invocation_nonce'], 'operation_id': operation} or
                final.get('backup_upload') != admitted or not isinstance(final_revision, tuple) or
                len(final_revision) != 2 or any(not isinstance(v, str) or not v for v in final_revision)):
            raise Refused('upload run reservation changed')
        output = environment.get('GITHUB_OUTPUT')
        if not isinstance(output, str) or not os.path.isabs(output):
            raise Refused('workflow output unavailable')
        fd = os.open(output, os.O_WRONLY | os.O_APPEND | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(fd, 'w', encoding='utf-8') as stream:
            info = os.fstat(stream.fileno())
            if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or
                    info.st_mode & 0o022 or info.st_nlink != 1):
                raise Refused('workflow output custody invalid')
            stream.write('path=' + str(target) + '\n')
            stream.write('name=voice-media-backup-' + operation + '-' + challenge + '\n')
            stream.flush()
        return target
    except BaseException:
        try:
            fence.uncertain('backup upload staging outcome uncertain')
        except BaseException:
            pass
        raise Refused('upload staging uncertain; retain lock and reconcile') from None


def mark_failed(environment, operation, challenge, repo_root):
    if (os.geteuid() != OPERATOR_UID or not isinstance(operation, str) or
            not re.fullmatch(r'media-c1-[0-9a-f]{32}', operation)):
        raise Refused('fixed Linux operator identity required')
    source = environment.get('GITHUB_SHA', '')
    publish_candidate.source_snapshot(Path(repo_root), source)
    raw_state = private_file(ROOT / operation / 'journal', 'state.json', 1048576)
    state = json.loads(raw_state, object_pairs_hook=unique_object)
    intent, planned = state['intent'], state['steps']['backup-upload']
    context, lock = intent['resource_context'], intent['kube_lock']
    client = Client()
    checker = resource_fencing.ResourceCheck(TT, context)
    fence = resource_fencing.Fence(context, checker, resource_fencing.KubeLedger(client), checker.mark_uncertain)
    media_backup_upload_run.validate_environment(fence, environment, challenge)
    media_operation_guard.verify(fence, client, lock, intent['namespace_uid'])
    row, revision = fence.ledger.read()
    actual = planned | {'status': 'upload-running', 'run_id': environment['GITHUB_RUN_ID'], 'run_attempt': 1}
    if (context['operation_id'] != operation or planned.get('operation_id') != operation or
            planned.get('status') != 'dispatch-pending' or planned.get('challenge') != challenge or
            not fence.owned(row) or row.get('status') != 'active' or row.get('backup_upload') != actual or
            row.get('backup_invocation') != {'nonce': planned.get('invocation_nonce'), 'operation_id': operation} or
            not isinstance(revision, tuple) or len(revision) != 2 or
            any(not isinstance(v, str) or not v for v in revision)):
        raise Refused('failed upload identity changed')
    fence.uncertain('GitHub ciphertext upload failed or outcome uncertain')


def main():
    try:
        if sys.argv[1:] == ['--mark-failed']:
            mark_failed(dict(os.environ), os.environ.get('DEV_BACKUP_OPERATION'),
                        os.environ.get('DEV_BACKUP_CHALLENGE'), Path.cwd())
        elif not sys.argv[1:]:
            stage(dict(os.environ), os.environ.get('DEV_BACKUP_OPERATION'),
                  os.environ.get('DEV_BACKUP_CHALLENGE'), Path.cwd())
        else:
            raise Refused('unsupported mode')
        return 0
    except BaseException:
        print('DEV backup upload stage refused; retain operation and reconcile', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
