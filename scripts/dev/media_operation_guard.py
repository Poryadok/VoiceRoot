"""Read-only shared authority check, not packet or workload admission.

The caller must separately validate the fixed candidate, full live inventory,
step-specific expected state, durable local journal and all effect receipts.
This check cannot revoke a request already sent or prevent an external writer.
"""
import json

import resource_fencing
import update_images

LABELS = update_images.LABELS
LOCK = update_images.LOCK


def verify(fence, client, expected_lock, namespace_uid):
    context = fence.context
    resource_fencing.validate_context(context)
    if context['actor'] != 'scheduled_devops' or not isinstance(namespace_uid, str) or not namespace_uid:
        raise ValueError('fixed DEV operator and namespace identity required')
    lock_meta = expected_lock.get('metadata', {}) if isinstance(expected_lock, dict) else {}
    lock_data = expected_lock.get('data', {}) if isinstance(expected_lock, dict) else {}
    expected_context = json.dumps(context, sort_keys=True, separators=(',', ':'))
    if (lock_meta.get('name') != LOCK or lock_meta.get('namespace') != 'voice-dev' or
            any(lock_meta.get('labels', {}).get(k) != v for k, v in LABELS.items()) or
            not lock_meta.get('uid') or not lock_meta.get('resourceVersion') or
            lock_data.get('resource_operation') != context['operation_id'] or
            lock_data.get('resource_context') != expected_context or not lock_data.get('operation')):
        raise ValueError('exact operation-owned lock required')
    fence.authority()
    first, revision = fence.ledger.read()
    if (not fence.owned(first) or first.get('status') != 'active' or
            not isinstance(revision, tuple) or len(revision) != 2 or
            any(not isinstance(value, str) or not value for value in revision)):
        raise ValueError('active owned permanent fence required')
    namespace = client.call('get', 'namespace', 'voice-dev', '-o', 'json')
    meta = namespace.get('metadata', {})
    if (meta.get('name') != 'voice-dev' or meta.get('uid') != namespace_uid or
            any(meta.get('labels', {}).get(k) != v for k, v in LABELS.items())):
        raise ValueError('namespace identity changed')
    actual = client.call('get', 'configmap', LOCK, '-o', 'json')
    actual_meta = actual.get('metadata', {})
    if (actual_meta.get('uid') != lock_meta['uid'] or
            actual_meta.get('resourceVersion') != lock_meta['resourceVersion'] or
            actual_meta.get('name') != LOCK or actual_meta.get('namespace') != 'voice-dev' or
            any(actual_meta.get('labels', {}).get(k) != v for k, v in LABELS.items()) or
            actual.get('data') != lock_data):
        raise ValueError('common operation lock changed')
    fence.authority()
    second, final_revision = fence.ledger.read()
    if second != first or final_revision != revision or not fence.owned(second) or second.get('status') != 'active':
        raise ValueError('permanent fence changed during check')
