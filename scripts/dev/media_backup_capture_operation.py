"""Single fenced capture-to-upload reservation, not a complete rollout.

The caller must establish and continuously maintain actual writer drain, a
qualified source/binding, operation ownership and recovery custody. This does
not dispatch, read back an artifact, restore data, or admit a deployment.
Any failure retains the operation fence for manual reconciliation.
"""
import copy
from pathlib import Path

import media_backup_upload_phase as upload
import story_media_backup as backup
import story_media_backup_capture as capture
import story_media_backup_custody as custody
import story_media_backup_finalize as finalize

MEMBERS = frozenset(('file_db', 'story_db', 'messaging_db', 'roles'))


class CaptureOperationError(RuntimeError):
    pass


def run(root, key, expected, phases, drain):
    try:
        phases._guard()
        intent = phases.intent
        source = intent['backup_capture']
        binding = intent['backup_binding']
        operation = phases.fence.context['operation_id']
        if (not isinstance(key, bytes) or len(key) != 32 or
                Path(root) != upload.ROOT / operation / 'backup' or
                expected != intent['backup_source'] or
                expected.get('operation') != operation or
                expected.get('namespace_uid') != phases.namespace_uid or
                binding.get('operation_id') != operation or
                binding.get('namespace_uid') != phases.namespace_uid or
                binding.get('source_sha') != phases.fence.context['commit'] or
                any(binding.get(k) != phases.fence.context[k] for k in
                    ('project', 'resource', 'actor', 'claim', 'window', 'generation', 'revision')) or
                expected.get('pg_major') != binding['restore_inputs']['postgres_major'] or
                any(expected.get(k) != source[k] or binding.get(k) != source[k] for k in source)):
            raise CaptureOperationError('capture source or root mismatch')
        drain()
        binding = copy.deepcopy(binding)
        phases._start('file_db')
        phases._guard()
        drain()
        producer = capture.Producer(expected, phases._guard, drain, phases)
        manifest = backup.export(root, key, binding, producer)
        phases._guard()
        drain()
        receipts = {}
        for member in MEMBERS:
            entry = phases.journal.state['steps']['dump-' + member]
            if entry.get('status') != 'done':
                raise CaptureOperationError('capture incomplete')
            receipts[member] = copy.deepcopy(entry['receipt'])
        final_binding, final_manifest = finalize.finalize(root, key, binding, manifest, receipts)
        phases._guard()
        drain()
        bundle = custody.build_bundle(root, key, final_binding, final_manifest)
        phases._guard()
        drain()
        return upload.prepare(phases, key, bundle)
    except BaseException:
        phases.poisoned = True
        try:
            phases.fence.uncertain('media capture or upload preparation failed')
        except BaseException:
            pass
        raise CaptureOperationError('capture failed; retain lock and reconcile') from None
