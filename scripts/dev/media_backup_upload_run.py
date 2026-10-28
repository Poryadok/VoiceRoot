"""Bind an already reserved DEV backup upload to one GitHub Actions run.

Only the reviewed master workflow may supply GitHub's environment; this is
not an authentication boundary against a local actor able to forge env. A
successful reservation is not an artifact upload or independent readback.
"""
import copy
import re

import media_operation_guard

REPOSITORY = 'Poryadok/VoiceRoot'
WORKFLOW_REF = REPOSITORY + '/.github/workflows/dev-media-backup-upload.yml@refs/heads/master'


class RecoveryRequired(RuntimeError):
    pass


def claim(fence, client, lock, namespace_uid, environment, challenge):
    attempted = False
    try:
        required = {'GITHUB_REPOSITORY', 'GITHUB_EVENT_NAME', 'GITHUB_REF', 'GITHUB_WORKFLOW_REF',
                    'GITHUB_SHA', 'GITHUB_WORKFLOW_SHA', 'GITHUB_RUN_ID', 'GITHUB_RUN_ATTEMPT'}
        if (not isinstance(environment, dict) or not required <= environment.keys() or
                environment['GITHUB_REPOSITORY'] != REPOSITORY or environment['GITHUB_EVENT_NAME'] != 'workflow_dispatch' or
                environment['GITHUB_REF'] != 'refs/heads/master' or environment['GITHUB_WORKFLOW_REF'] != WORKFLOW_REF or
                environment['GITHUB_SHA'] != fence.context['commit'] or
                environment['GITHUB_WORKFLOW_SHA'] != environment['GITHUB_SHA'] or
                not isinstance(environment['GITHUB_RUN_ID'], str) or
                not re.fullmatch(r'[1-9][0-9]{0,19}', environment['GITHUB_RUN_ID']) or
                environment['GITHUB_RUN_ATTEMPT'] != '1' or not isinstance(challenge, str) or
                not re.fullmatch(r'[0-9a-f]{32}', challenge)):
            raise RecoveryRequired('workflow identity invalid')
        media_operation_guard.verify(fence, client, lock, namespace_uid)
        row, revision = fence.ledger.read()
        planned = row.get('backup_upload') if isinstance(row, dict) else None
        required_plan = {'status', 'operation_id', 'snapshot_id', 'source_sha', 'invocation_nonce',
                         'challenge', 'bundle_bytes', 'bundle_sha256', 'workflow_sha'}
        invocation = row.get('backup_invocation') if isinstance(row, dict) else None
        if (not fence.owned(row) or row.get('status') != 'active' or not isinstance(planned, dict) or
                set(planned) != required_plan or planned['status'] != 'dispatch-pending' or
                planned['operation_id'] != fence.context['operation_id'] or
                not isinstance(planned['operation_id'], str) or not re.fullmatch(r'media-c1-[0-9a-f]{32}', planned['operation_id']) or
                not isinstance(planned['invocation_nonce'], str) or not re.fullmatch(r'[0-9a-f]{32}', planned['invocation_nonce']) or
                planned['source_sha'] != fence.context['commit'] or planned['workflow_sha'] != environment['GITHUB_SHA'] or
                planned['challenge'] != challenge or not isinstance(invocation, dict) or
                invocation != {'nonce': planned['invocation_nonce'], 'operation_id': planned['operation_id']} or
                not isinstance(planned['snapshot_id'], str) or not re.fullmatch(r'capture-[0-9a-f]{32}', planned['snapshot_id']) or
                not isinstance(planned['bundle_sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', planned['bundle_sha256']) or
                type(planned['bundle_bytes']) is not int or not 0 < planned['bundle_bytes'] <= 64 * 1024 ** 3):
            raise RecoveryRequired('upload reservation invalid or already used')
        actual = copy.deepcopy(planned)
        actual.update(status='upload-running', run_id=environment['GITHUB_RUN_ID'], run_attempt=1)
        row['backup_upload'] = actual
        row['step'] = 'backup-upload-running'
        fence.authority()
        attempted = True
        fence.ledger.write(revision, row)
        media_operation_guard.verify(fence, client, lock, namespace_uid)
        current, current_revision = fence.ledger.read()
        if (not fence.owned(current) or current.get('status') != 'active' or
                current.get('backup_invocation') != invocation or current.get('backup_upload') != actual or
                not isinstance(current_revision, tuple) or
                len(current_revision) != 2 or any(not isinstance(v, str) or not v for v in current_revision)):
            raise RecoveryRequired('upload run binding changed')
        return actual
    except BaseException:
        if attempted:
            try:
                fence.uncertain('backup upload run binding outcome uncertain')
            except BaseException:
                pass
        raise RecoveryRequired('upload run not admitted; retain lock and reconcile') from None
