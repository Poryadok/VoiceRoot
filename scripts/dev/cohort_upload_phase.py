"""Reserve one selected-cohort ciphertext upload; no dispatch or deployment."""
import copy
import hashlib
import json
from pathlib import Path
import re
import secrets
import story_media_backup_cohort_custody as custody

ROOT = Path('/home/pmd/voice-dev-media-operations')


class CohortUploadError(RuntimeError):
    pass


def prepare(phase, key, bundle):
    pending = False
    p = phase.phases
    try:
        phase._guard()
        binding = copy.deepcopy(phase.binding)
        context = p.fence.context
        operation = context['operation_id']
        if (not p.started or not isinstance(key, bytes) or len(key) != 32 or
                p.intent.get('upload_workflow_sha') != context['commit'] or
                Path(bundle) != ROOT/operation/'cohort-backup'/'cohort-backup.tar'):
            raise CohortUploadError('upload input invalid')
        row, revision = p.fence.ledger.read()
        if (not p.fence.owned(row) or row.get('status') != 'active' or not p._owned_invocation(row) or
                'backup_members' in row or 'cohort_upload' in row or
                set(row.get('cohort_members', {})) != phase.members):
            raise CohortUploadError('owned complete cohort required')
        digests = {}
        for member in phase.members:
            entry = p.journal.state['steps']['cohort-dump-' + member]
            receipt = entry['receipt']
            digest = hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
            expected = {'status':'done','nonce':entry['nonce'],'snapshot_id':binding['snapshot_id'],
                        'operation_id':operation,'receipt_sha256':digest}
            if entry['status'] != 'done' or row['cohort_members'][member] != expected:
                raise CohortUploadError('cohort terminal receipt differs')
            digests[member] = digest
        final_binding = binding | {'dump_receipts':digests}
        verified = custody.verify_bundle(Path(bundle),key,final_binding,binding['databases'])
        if (not isinstance(verified,dict) or set(verified)!={'bytes','sha256'} or
                type(verified['bytes']) is not int or not 0 < verified['bytes'] <= 64*1024**3 or
                not isinstance(verified['sha256'],str) or not re.fullmatch(r'[0-9a-f]{64}',verified['sha256'])):
            raise CohortUploadError('cohort bundle invalid')
        marker = row.get('cohort_bundle')
        local_marker = p.journal.state['steps'].get('cohort-bundle')
        if (not isinstance(marker,dict) or set(marker) != {'status','operation_id','snapshot_id','invocation_nonce',
                'bundle_bytes','bundle_sha256','manifest_sha256','receipt_sha256'} or
                marker.get('status') != 'done' or marker.get('operation_id') != operation or
                marker.get('snapshot_id') != binding['snapshot_id'] or marker.get('invocation_nonce') != p.invocation or
                marker.get('bundle_bytes') != verified['bytes'] or marker.get('bundle_sha256') != verified['sha256'] or
                not isinstance(marker.get('manifest_sha256'),str) or not re.fullmatch(r'[0-9a-f]{64}',marker['manifest_sha256']) or
                marker.get('receipt_sha256') != digests or
                local_marker != {'status':'done','marker':marker}):
            raise CohortUploadError('same invocation bundle proof missing')
        phase._guard()
        planned = {'status':'dispatch-pending','operation_id':operation,'snapshot_id':binding['snapshot_id'],
                   'source_sha':context['commit'],'workflow_sha':context['commit'],
                   'invocation_nonce':p.invocation,'challenge':secrets.token_hex(16),
                   'bundle_bytes':verified['bytes'],'bundle_sha256':verified['sha256'],
                   'members':sorted(phase.members),'receipt_sha256':copy.deepcopy(digests),
                   'manifest_sha256':marker['manifest_sha256']}
        if 'cohort-upload' in p.journal.state['steps']:
            raise CohortUploadError('upload already attempted')
        pending = True
        p.journal.state['steps']['cohort-upload'] = copy.deepcopy(planned)
        p.journal.save()
        current, actual = p.fence.ledger.read()
        if current != row or actual != revision:
            raise CohortUploadError('ledger changed')
        current['cohort_upload'] = copy.deepcopy(planned)
        current['step'] = 'cohort-upload-dispatch-pending'
        p.fence.ledger.write(actual,current)
        phase._guard()
        checked, checked_revision = p.fence.ledger.read()
        if (not p.fence.owned(checked) or checked.get('status') != 'active' or not p._owned_invocation(checked) or
                checked.get('cohort_upload') != planned or not isinstance(checked_revision,tuple) or
                checked.get('cohort_bundle') != marker or checked.get('cohort_members') != row['cohort_members'] or
                'backup_members' in checked or
                len(checked_revision)!=2 or any(not isinstance(v,str) or not v for v in checked_revision)):
            raise CohortUploadError('upload reservation changed')
        return copy.deepcopy(planned)
    except BaseException:
        if pending:
            try: phase.uncertain('upload')
            except BaseException: pass
        raise CohortUploadError('cohort upload not admitted; reconcile if pending') from None
