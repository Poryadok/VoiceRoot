"""Bind authenticated cohort members to externally verified dump receipts."""
import hashlib
import json
import re
import uuid

import story_media_backup_cohort as cohort
from story_media_backup_finalize import IMAGE, RESTORE_KEYS

BINDING_KEYS = {'operation_id', 'snapshot_id', 'source_sha', 'project', 'resource', 'actor',
                'claim', 'window', 'generation', 'revision', 'databases', 'sources', 'restore_inputs'}
RECEIPT_KEYS = {'operation_id', 'snapshot_id', 'member', 'pod_uid', 'pvc_uid', 'pv_uid',
                'container_id', 'restart_count', 'server_start', 'database_oid', 'attempt_nonce', 'exit', 'scope'}


class CohortFinalizeError(Exception):
    pass


def _validate_binding_shape(binding, databases):
    """Shared structural checks; callers must separately establish authority."""
    members = cohort._members_shape(binding, databases)
    expected_keys = BINDING_KEYS | ({'full_catalog'} if 'full_catalog' in binding else set())
    if (set(binding) != expected_keys or type(binding['generation']) is not int
            or binding['generation'] <= 0 or type(binding['revision']) is not int or binding['revision'] <= 0):
        raise CohortFinalizeError('cohort binding invalid')
    for name in ('claim', 'window'):
        if not isinstance(binding[name], str) or str(uuid.UUID(binding[name])) != binding[name]:
            raise CohortFinalizeError('cohort binding invalid')
    engine = binding['sources']['roles']['engine']
    for name in ('namespace_uid', 'pod_uid', 'pvc_uid', 'pv_uid'):
        if not isinstance(engine.get(name), str) or str(uuid.UUID(engine[name])) != engine[name]:
            raise CohortFinalizeError('cohort engine invalid')
    if (not isinstance(engine.get('container_id'), str)
            or not re.fullmatch(r'containerd://[0-9a-f]{64}', engine['container_id'])
            or type(engine.get('restart_count')) is not int or engine['restart_count'] < 0):
        raise CohortFinalizeError('cohort engine invalid')
    inputs = binding['restore_inputs']
    if not isinstance(inputs, dict) or set(inputs) != RESTORE_KEYS:
        raise CohortFinalizeError('cohort restore binding invalid')
    for name in ('catalog_sha256', 'roles_catalog_sha256', 'migration_catalog_sha256', 'migrate_binary_sha256'):
        if not isinstance(inputs[name], str) or not re.fullmatch(r'[0-9a-f]{64}', inputs[name]):
            raise CohortFinalizeError('cohort restore binding invalid')
    if (not isinstance(inputs['postgres_image'], str) or not IMAGE.fullmatch(inputs['postgres_image'])
            or not isinstance(inputs['migrate_image'], str) or not IMAGE.fullmatch(inputs['migrate_image'])
            or type(inputs['postgres_major']) is not int or inputs['postgres_major'] != 16):
        raise CohortFinalizeError('cohort restore binding invalid')
    return members


def validate_binding(binding, databases):
    """Reject malformed or non-DEV authority before any dump."""
    members = _validate_binding_shape(binding, databases)
    if cohort._members(binding, databases)!=members:raise CohortFinalizeError('cohort binding invalid')
    if (binding['project'], binding['resource'], binding['actor']) != ('voice', 'DEV', 'scheduled_devops'):
        raise CohortFinalizeError('cohort binding invalid')
    return members


def _finalize_receipts(root, key, binding, databases, manifest, receipts, members):
    try:
        raw = json.dumps({'binding': binding, 'databases': databases, 'manifest': manifest, 'receipts': receipts},
                         sort_keys=True, separators=(',', ':'), ensure_ascii=True, allow_nan=False).encode('ascii')
        if len(raw) > 16 * 1024 * 1024: raise CohortFinalizeError('cohort inputs too large')
        snapshot = json.loads(raw)
        binding, databases, manifest, receipts = (snapshot['binding'], snapshot['databases'],
                                                  snapshot['manifest'], snapshot['receipts'])
        if cohort._members_shape(binding,databases)!=members:raise CohortFinalizeError('cohort members changed')
        engine = binding['sources']['roles']['engine']
        if not isinstance(receipts, dict) or set(receipts) != members:
            raise CohortFinalizeError('cohort receipt set invalid')
        hashes = {}; nonces = set()
        for member in sorted(members):
            receipt = receipts[member]; source = binding['sources'][member]
            if (not isinstance(receipt, dict) or set(receipt) != RECEIPT_KEYS
                    or receipt['operation_id'] != binding['operation_id']
                    or receipt['snapshot_id'] != binding['snapshot_id'] or receipt['member'] != member
                    or receipt['scope'] != 'dump-terminal-only' or type(receipt['exit']) is not int or receipt['exit'] != 0
                    or type(receipt['restart_count']) is not int or receipt['restart_count'] != engine['restart_count']
                    or any(receipt[name] != engine[name] for name in ('pod_uid', 'pvc_uid', 'pv_uid', 'container_id'))
                    or receipt['server_start'] != source['server_start']
                    or receipt['database_oid'] != source.get('database_oid')
                    or (member != 'roles' and type(receipt['database_oid']) is not int)
                    or not isinstance(receipt['attempt_nonce'], str)
                    or not re.fullmatch(r'[0-9a-f]{32}', receipt['attempt_nonce'])
                    or receipt['attempt_nonce'] in nonces):
                raise CohortFinalizeError('cohort receipt invalid')
            nonces.add(receipt['attempt_nonce'])
            encoded = json.dumps(receipt, sort_keys=True, separators=(',', ':'), ensure_ascii=True,
                                 allow_nan=False).encode('ascii')
            hashes[member] = hashlib.sha256(encoded).hexdigest()
        cohort._verify_validated(root, key, binding, databases, manifest, members)
        final_binding = dict(binding); final_binding['dump_receipts'] = hashes
        final_manifest = {'format': 1, 'binding': final_binding, 'files': manifest['files']}
        final_manifest['mac'] = cohort._mac(key, final_manifest)
        cohort._verify_validated(root, key, final_binding, databases, final_manifest, members)
        return final_binding, final_manifest
    except Exception:
        raise CohortFinalizeError('cohort finalization rejected') from None


def finalize(root, key, binding, databases, manifest, receipts):
    """Authenticate DEV receipt bindings; caller verifies real ledger authority."""
    try:
        snapshot=json.loads(json.dumps({'binding':binding,'databases':databases,'manifest':manifest,'receipts':receipts},
                                      sort_keys=True,separators=(',',':'),ensure_ascii=True,allow_nan=False))
        binding,databases,manifest,receipts=(snapshot[k] for k in ('binding','databases','manifest','receipts'))
        return _finalize_receipts(root,key,binding,databases,manifest,receipts,validate_binding(binding,databases))
    except Exception:raise CohortFinalizeError('cohort finalization rejected') from None


def finalize_staging(session,binding,databases,manifest,receipts,catalog_ref,metadata_ref):
    """Finalize only the root-owned STAGING operation and its fixed custody."""
    try:
        import staging_cohort_preservation as staging
        snapshot=json.loads(json.dumps({'binding':binding,'databases':databases,'manifest':manifest,'receipts':receipts,
            'catalog_ref':catalog_ref,'metadata_ref':metadata_ref},sort_keys=True,separators=(',',':'),ensure_ascii=True,allow_nan=False))
        binding,databases,manifest,receipts,catalog_ref,metadata_ref=(snapshot[k] for k in ('binding','databases','manifest','receipts','catalog_ref','metadata_ref'))
        members=staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)
        _validate_binding_shape(binding,databases)
        root,key=staging.backup_custody(session)
        result=_finalize_receipts(root,key,binding,databases,manifest,receipts,members)
        if staging.validate_backup_binding(session,binding,databases,catalog_ref,metadata_ref)!=members:
            raise CohortFinalizeError('staging authority changed')
        return result
    except Exception:raise CohortFinalizeError('staging cohort finalization rejected') from None
