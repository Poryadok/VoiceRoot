"""Authenticate a receipt-bound media backup manifest after export."""
import hashlib
import json
import re
import uuid

import story_media_backup as backup

BINDING_KEYS = {'operation_id', 'snapshot_id', 'source_sha', 'project', 'resource', 'actor',
                'claim', 'window', 'generation', 'revision', 'namespace_uid', 'pod_uid',
                'pvc_uid', 'pv_uid', 'container_id', 'restart_count', 'restore_inputs'}
RESTORE_KEYS = {'catalog_sha256', 'roles_catalog_sha256', 'migration_catalog_sha256',
                'postgres_image', 'migrate_image', 'migrate_binary_sha256', 'postgres_major'}
RECEIPT_KEYS = {'operation_id', 'snapshot_id', 'member', 'pod_uid', 'pvc_uid', 'pv_uid',
                'container_id', 'restart_count', 'attempt_nonce', 'exit', 'scope'}
IMAGE = re.compile(r'(?:sha256:[0-9a-f]{64}|[a-z0-9][a-z0-9._/:\-]*@sha256:[0-9a-f]{64})')


class FinalizeError(Exception):
    pass


def finalize(root, key, binding, manifest, receipts):
    """Bind verified receipts; caller separately verifies real ledger authority."""
    try:
        snapshot = json.loads(json.dumps({'binding': binding, 'manifest': manifest, 'receipts': receipts},
                                         sort_keys=True, separators=(',', ':'), allow_nan=False))
        binding, manifest, receipts = snapshot['binding'], snapshot['manifest'], snapshot['receipts']
        if not isinstance(binding, dict) or set(binding) != BINDING_KEYS:
            raise FinalizeError('binding invalid')
        if (not re.fullmatch(r'media-c[12]-[0-9a-f]{32}', binding['operation_id'])
                or not re.fullmatch(r'capture-[0-9a-f]{32}', binding['snapshot_id'])
                or not re.fullmatch(r'[0-9a-f]{40}', binding['source_sha'])
                or (binding['project'], binding['resource'], binding['actor']) != ('voice', 'DEV', 'scheduled_devops')
                or not re.fullmatch(r'containerd://[0-9a-f]{64}', binding['container_id'])
                or type(binding['generation']) is not int or binding['generation'] <= 0
                or type(binding['revision']) is not int or binding['revision'] <= 0
                or type(binding['restart_count']) is not int or binding['restart_count'] < 0):
            raise FinalizeError('binding invalid')
        for name in ('claim', 'window', 'namespace_uid', 'pod_uid', 'pvc_uid', 'pv_uid'):
            if not isinstance(binding[name], str) or str(uuid.UUID(binding[name])) != binding[name]:
                raise FinalizeError('binding invalid')
        inputs = binding['restore_inputs']
        if not isinstance(inputs, dict) or set(inputs) != RESTORE_KEYS:
            raise FinalizeError('restore binding invalid')
        for name in ('catalog_sha256', 'roles_catalog_sha256', 'migration_catalog_sha256', 'migrate_binary_sha256'):
            if not isinstance(inputs[name], str) or not re.fullmatch(r'[0-9a-f]{64}', inputs[name]):
                raise FinalizeError('restore binding invalid')
        if (not isinstance(inputs['postgres_image'], str) or not IMAGE.fullmatch(inputs['postgres_image'])
                or not isinstance(inputs['migrate_image'], str) or not IMAGE.fullmatch(inputs['migrate_image'])
                or type(inputs['postgres_major']) is not int or inputs['postgres_major'] != 16):
            raise FinalizeError('restore binding invalid')
        if not isinstance(receipts, dict) or set(receipts) != backup.MEMBERS:
            raise FinalizeError('receipt set invalid')
        hashes = {}; nonces = set()
        for member in sorted(backup.MEMBERS):
            receipt = receipts[member]
            if (not isinstance(receipt, dict) or set(receipt) != RECEIPT_KEYS
                    or receipt['member'] != member or receipt['scope'] != 'dump-terminal-only'
                    or type(receipt['exit']) is not int or receipt['exit'] != 0
                    or type(receipt['restart_count']) is not int
                    or any(receipt[name] != binding[name] for name in ('operation_id', 'snapshot_id', 'pod_uid',
                        'pvc_uid', 'pv_uid', 'container_id', 'restart_count'))
                    or not isinstance(receipt['attempt_nonce'], str)
                    or not re.fullmatch(r'[0-9a-f]{32}', receipt['attempt_nonce'])
                    or receipt['attempt_nonce'] in nonces):
                raise FinalizeError('receipt invalid')
            nonces.add(receipt['attempt_nonce'])
            raw = json.dumps(receipt, sort_keys=True, separators=(',', ':'), ensure_ascii=True,
                             allow_nan=False).encode('ascii')
            hashes[member] = hashlib.sha256(raw).hexdigest()
        backup.verify(root, key, binding, manifest)
        final_binding = json.loads(json.dumps(binding, sort_keys=True, separators=(',', ':'), allow_nan=False))
        final_binding['dump_receipts'] = hashes
        final_manifest = {'format': 1, 'binding': final_binding,
                          'files': json.loads(json.dumps(manifest['files'], sort_keys=True))}
        final_manifest['mac'] = backup.manifest_mac(key, final_manifest)
        backup.verify(root, key, final_binding, final_manifest)
        return final_binding, final_manifest
    except Exception:
        raise FinalizeError('receipt finalization rejected') from None
