"""Reserve one DEV ciphertext upload; never dispatch or upload by itself.

The caller must hold the live operation lock and the same sequential
BackupPhases instance, with separately verified drain and terminal dump
receipts and externally qualified exact P1 source. The first supported case
binds workflow source S=P1; this is not a general publisher constraint. A
planned or uncertain upload is never retried automatically.
"""
import copy
import hashlib
import json
from pathlib import Path
import re
import secrets

from media_backup_phase import MEMBERS, RecoveryRequired

ROOT = Path('/home/pmd/voice-dev-media-operations')


def prepare(phases, key, bundle):
    pending = False
    try:
        phases._guard()
        if not phases.started or not isinstance(key, bytes) or len(key) != 32:
            raise RecoveryRequired('live owned backup invocation required')
        intent = phases.intent
        binding = intent.get('backup_binding')
        capture = intent['backup_capture']
        context = phases.fence.context
        operation = phases.fence.context['operation_id']
        source = phases.fence.context['commit']
        workflow = intent.get('upload_workflow_sha')
        fixed = {'operation_id': operation, 'snapshot_id': capture['snapshot_id'], 'source_sha': source,
                 **{k: context[k] for k in ('project', 'resource', 'actor', 'claim', 'window', 'generation', 'revision')},
                 'namespace_uid': phases.namespace_uid,
                 **{k: capture[k] for k in ('pod_uid', 'pvc_uid', 'pv_uid', 'container_id', 'restart_count')}}
        restore = binding.get('restore_inputs') if isinstance(binding, dict) else None
        if (not isinstance(binding, dict) or set(binding) != set(fixed) | {'restore_inputs'} or
                any(binding.get(k) != value for k, value in fixed.items()) or
                type(binding.get('restart_count')) is not int or type(binding.get('generation')) is not int or
                type(binding.get('revision')) is not int or not isinstance(restore, dict) or
                set(restore) != {'catalog_sha256', 'roles_catalog_sha256', 'migration_catalog_sha256',
                                 'postgres_image', 'migrate_image', 'migrate_binary_sha256', 'postgres_major'} or
                any(not isinstance(restore.get(k), str) or not re.fullmatch(r'[0-9a-f]{64}', restore[k])
                    for k in ('catalog_sha256', 'roles_catalog_sha256', 'migration_catalog_sha256', 'migrate_binary_sha256')) or
                any(not isinstance(restore.get(k), str) or not re.fullmatch(r'[^\s@]+@sha256:[0-9a-f]{64}', restore[k])
                    for k in ('postgres_image', 'migrate_image')) or
                type(restore.get('postgres_major')) is not int or restore['postgres_major'] != 16 or
                not isinstance(workflow, str) or
                not re.fullmatch(r'[0-9a-f]{40}', workflow) or workflow != source or
                Path(bundle) != ROOT / operation / 'backup' / 'media-backup.tar'):
            raise RecoveryRequired('upload source, path or binding invalid')
        row, revision = phases.fence.ledger.read()
        if (not phases.fence.owned(row) or row.get('status') != 'active' or
                not phases._owned_invocation(row) or 'backup_upload' in row or
                set(row.get('backup_members', {})) != MEMBERS):
            raise RecoveryRequired('complete owned dump set required')
        receipt_digests = {}
        for member in MEMBERS:
            entry = phases.journal.state['steps'].get('dump-' + member)
            saved = row['backup_members'][member]
            if not isinstance(entry, dict) or entry.get('status') != 'done' or not isinstance(entry.get('receipt'), dict):
                raise RecoveryRequired('local terminal dump receipt missing')
            digest = hashlib.sha256(json.dumps(entry['receipt'], sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            if (saved != {'status': 'done', 'nonce': entry.get('nonce'),
                          'snapshot_id': binding['snapshot_id'], 'operation_id': operation,
                          'receipt_sha256': digest}):
                raise RecoveryRequired('ledger terminal dump receipt differs')
            receipt_digests[member] = digest
        binding = binding | {'dump_receipts': receipt_digests}
        from story_media_backup_custody import verify_bundle
        verified = verify_bundle(Path(bundle), key, binding)
        if (not isinstance(verified, dict) or set(verified) != {'bytes', 'sha256'} or
                type(verified['bytes']) is not int or not 0 < verified['bytes'] <= 64 * 1024 ** 3 or
                not isinstance(verified['sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', verified['sha256'])):
            raise RecoveryRequired('bundle verification receipt invalid')
        phases._guard()
        planned = {'status': 'dispatch-pending', 'operation_id': operation,
                   'snapshot_id': binding['snapshot_id'], 'source_sha': source,
                   'invocation_nonce': phases.invocation, 'challenge': secrets.token_hex(16),
                   'bundle_bytes': verified['bytes'], 'bundle_sha256': verified['sha256'],
                   'workflow_sha': workflow}
        if 'backup-upload' in phases.journal.state['steps']:
            raise RecoveryRequired('upload already attempted; reconcile')
        pending = True
        phases.journal.state['steps']['backup-upload'] = copy.deepcopy(planned)
        phases.journal.save()
        phases.fence.authority()
        current, actual_revision = phases.fence.ledger.read()
        if (actual_revision != revision or current != row or not phases.fence.owned(current) or
                current.get('status') != 'active' or not phases._owned_invocation(current)):
            raise RecoveryRequired('fence changed before upload intent')
        current['backup_upload'] = copy.deepcopy(planned)
        current['step'] = 'backup-upload-dispatch-pending'
        phases.fence.ledger.write(actual_revision, current)
        phases._guard()
        checked, checked_revision = phases.fence.ledger.read()
        if (not phases.fence.owned(checked) or checked.get('status') != 'active' or
                not isinstance(checked_revision, tuple) or len(checked_revision) != 2 or
                any(not isinstance(v, str) or not v for v in checked_revision) or
                not phases._owned_invocation(checked) or checked.get('backup_upload') != planned):
            raise RecoveryRequired('upload intent changed')
        return copy.deepcopy(planned)
    except BaseException:
        if pending:
            phases._uncertain('upload')
        raise RecoveryRequired('upload was not admitted; no dispatch issued') from None
