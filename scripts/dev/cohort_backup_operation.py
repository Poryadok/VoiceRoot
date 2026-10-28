"""One admitted cohort capture and authenticated private bundle; no upload or rollout."""
import copy
import hashlib
import json
from pathlib import Path

import story_media_backup_cohort as cohort
import story_media_backup_cohort_capture as capture
import story_media_backup_cohort_finalize as finalize
import story_media_backup_cohort_custody as custody

ROOT = Path('/home/pmd/voice-dev-media-operations')


class CohortOperationError(RuntimeError):
    pass


def run(phase, root, key, expected, admin_role, drain):
    """Caller owns the verified affected set, source, drain, claim and common lock."""
    p = phase.phases
    pending = False
    try:
        phase._guard()
        binding = copy.deepcopy(phase.binding)
        finalize.validate_binding(binding, binding['databases'])
        context = p.fence.context
        source = binding['sources']['roles']['engine']
        if (not isinstance(key, bytes) or len(key) != 32 or
                Path(root) != ROOT / context['operation_id'] / 'cohort-backup' or
                expected != p.intent['cohort_source'] or admin_role != p.intent['cohort_admin_role'] or
                expected.get('operation') != context['operation_id'] or
                expected.get('snapshot_id') != binding['snapshot_id'] or
                expected.get('pg_major') != binding['restore_inputs']['postgres_major'] or
                any(expected.get(k) != source.get(k) for k in
                    ('namespace_uid','pod_uid','pvc_uid','pv_uid','container_id','restart_count')) or
                any(binding.get(k) != context[k] for k in
                    ('project','resource','actor','claim','window','generation','revision'))):
            raise CohortOperationError('cohort preflight identity mismatch')
        producer = capture.Producer(binding, binding['databases'], expected, admin_role, phase._guard, drain, phase)
        drain()
        pending = True
        p._start('roles')
        phase._guard(); drain()
        manifest = cohort.export(root, key, binding, binding['databases'], producer)
        phase._guard(); drain()
        row, revision = p.fence.ledger.read()
        if (not p.fence.owned(row) or row.get('status') != 'active' or not p._owned_invocation(row) or
                set(row.get('cohort_members', {})) != phase.members):
            raise CohortOperationError('cohort receipt set incomplete')
        receipts = {}
        for member in phase.members:
            entry = p.journal.state['steps']['cohort-dump-' + member]
            receipt = entry['receipt']
            digest = hashlib.sha256(json.dumps(receipt, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            expected_record = {'status':'done','nonce':entry['nonce'],'snapshot_id':binding['snapshot_id'],
                               'operation_id':binding['operation_id'],'receipt_sha256':digest}
            if entry['status'] != 'done' or row['cohort_members'][member] != expected_record:
                raise CohortOperationError('cohort receipt changed')
            receipts[member] = copy.deepcopy(receipt)
        final_binding, final_manifest = finalize.finalize(root, key, binding, binding['databases'], manifest, receipts)
        phase._guard(); drain()
        current, current_revision = p.fence.ledger.read()
        if current != row or current_revision != revision:
            raise CohortOperationError('cohort ledger changed')
        bundle = custody.build_bundle(root, key, final_binding, binding['databases'], final_manifest)
        phase._guard(); drain()
        current, current_revision = p.fence.ledger.read()
        if current != row or current_revision != revision:
            raise CohortOperationError('cohort ledger changed')
        return bundle, final_binding, final_manifest
    except BaseException:
        if pending:
            p.poisoned = True
            try: p.fence.uncertain('cohort capture or bundle outcome uncertain')
            except BaseException: pass
        raise CohortOperationError('cohort capture failed; retain lock and reconcile' if pending
                                   else 'cohort preflight refused; no capture attempted') from None
