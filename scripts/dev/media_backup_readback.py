"""Verify one actual GitHub upload; caller retains the environment fence."""
import io
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import time

import story_media_backup_custody as custody
import story_media_backup as backup
import media_backup_upload_phase as upload
from media_backup_phase import MEMBERS

API = 'https://api.github.com/repos/Poryadok/VoiceRoot/actions/runs/'
PATH = '.github/workflows/dev-media-backup-upload.yml'


class ReadbackError(RuntimeError):
    pass


def get_json(token, url):
    if not isinstance(token, str) or not token or '\n' in token or '\r' in token:
        raise ReadbackError('credential unavailable')
    headers = {'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json',
               'X-GitHub-Api-Version': '2022-11-28', 'User-Agent': 'Voice-media-readback'}
    result = io.BytesIO()
    custody.transport._download(url, headers, result, 65536, time.monotonic() + 30)
    return json.loads(result.getvalue(), object_pairs_hook=custody._unique_object)


def workflow_evidence(token, planned):
    try:
        run_id = planned['run_id']
        if not isinstance(run_id, str) or not re.fullmatch(r'[1-9][0-9]{0,19}', run_id):
            raise ReadbackError('invalid run')
        run = get_json(token, API + run_id)
        if (run.get('id') != int(run_id) or run.get('head_sha') != planned['source_sha'] or
                run.get('head_branch') != 'master' or run.get('event') != 'workflow_dispatch' or
                run.get('path') != PATH or run.get('run_attempt') != 1 or
                run.get('status') != 'completed' or run.get('conclusion') != 'success' or
                not isinstance(run.get('created_at'), str)):
            raise ReadbackError('workflow is not successful qualified run')
        listing = get_json(token, API + run_id + '/artifacts?per_page=100')
        name = 'voice-media-backup-' + planned['operation_id'] + '-' + planned['challenge']
        artifacts = listing.get('artifacts')
        if (not isinstance(artifacts, list) or type(listing.get('total_count')) is not int or
                listing['total_count'] != len(artifacts) or len(artifacts) > 100):
            raise ReadbackError('incomplete artifact list')
        matches = [a for a in artifacts if isinstance(a, dict) and a.get('name') == name]
        if (len(matches) != 1 or type(matches[0].get('id')) is not int or matches[0]['id'] <= 0 or
                not isinstance(matches[0].get('created_at'), str)):
            raise ReadbackError('artifact identity unavailable')
        return matches[0]['id'], matches[0]['created_at']
    except Exception:
        raise ReadbackError('qualified workflow evidence unavailable') from None


def run(phases, token, key, bundle, destination, drain):
    """Authenticate a complete run and reserve/commit one readback; no restore admission."""
    pending = False
    try:
        phases._guard()
        drain()
        row, revision = phases.fence.ledger.read()
        planned = row.get('backup_upload')
        local = phases.journal.state['steps'].get('backup-upload')
        if (not phases.fence.owned(row) or row.get('status') != 'active' or
                not phases._owned_invocation(row) or not isinstance(planned, dict) or
                planned.get('status') != 'upload-running' or planned.get('run_attempt') != 1 or
                not isinstance(local, dict) or local.get('status') != 'dispatch-pending' or
                any(planned.get(k) != v for k, v in local.items() if k != 'status') or
                'backup_readback' in row or 'backup-readback' in phases.journal.state['steps'] or
                Path(bundle) != upload.ROOT / planned['operation_id'] / 'backup' / 'media-backup.tar' or
                Path(destination) != upload.ROOT / planned['operation_id'] /
                    ('readback-' + planned['run_id'] + '-' + planned['challenge'])):
            raise ReadbackError('readback reservation invalid')
        receipts = {}
        for member in MEMBERS:
            entry = phases.journal.state['steps']['dump-' + member]
            receipt = entry['receipt']
            digest = hashlib.sha256(json.dumps(receipt, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            saved = row['backup_members'][member]
            if (entry['status'] != 'done' or saved != {'status': 'done', 'nonce': entry['nonce'],
                    'snapshot_id': planned['snapshot_id'], 'operation_id': planned['operation_id'],
                    'receipt_sha256': digest}):
                raise ReadbackError('dump receipt changed')
            receipts[member] = digest
        binding = copy.deepcopy(phases.intent['backup_binding']) | {'dump_receipts': receipts}
        artifact_id, created_at = workflow_evidence(token, planned)
        artifact_binding = {k: planned[k] for k in ('operation_id', 'snapshot_id', 'challenge',
                            'bundle_bytes', 'bundle_sha256')}
        artifact_binding.update(run_id=int(planned['run_id']), head_sha=planned['source_sha'], created_at=created_at)
        intent = {'status': 'pending', 'artifact_id': artifact_id, 'binding': artifact_binding}
        phases._guard()
        drain()
        pending = True
        phases.journal.state['steps']['backup-readback'] = copy.deepcopy(intent)
        phases.journal.save()
        current, current_revision = phases.fence.ledger.read()
        if current_revision != revision or current != row:
            raise ReadbackError('ledger changed before readback')
        current['backup_readback'] = copy.deepcopy(intent)
        current['step'] = 'backup-readback-pending'
        phases.fence.ledger.write(current_revision, current)
        phases._guard()
        drain()
        reserved, reserved_revision = phases.fence.ledger.read()
        if (not phases.fence.owned(reserved) or reserved.get('status') != 'active' or
                not phases._owned_invocation(reserved) or reserved.get('backup_upload') != planned or
                reserved.get('backup_readback') != intent or not isinstance(reserved_revision, tuple) or
                len(reserved_revision) != 2 or any(not isinstance(v, str) or not v for v in reserved_revision)):
            raise ReadbackError('readback reservation changed')
        with backup.private_directory(Path(destination).parent) as parent:
            os.mkdir(Path(destination).name, 0o700, dir_fd=parent)
            os.fsync(parent)
        receipt, manifest = custody.readback_extract(token, artifact_binding, artifact_id, bundle, key, binding, destination)
        wanted = {'schema': 'voice-media-custody-v1', 'destination': 'github:Poryadok/VoiceRoot',
                  'artifact_id': artifact_id, **{k: v for k, v in artifact_binding.items() if k != 'created_at'}}
        if receipt != wanted:
            raise ReadbackError('readback receipt mismatch')
        backup.verify(destination, key, binding, manifest)
        phases._guard()
        drain()
        current, current_revision = phases.fence.ledger.read()
        if (not phases.fence.owned(current) or current.get('status') != 'active' or
                not phases._owned_invocation(current) or current.get('backup_upload') != planned or
                current.get('backup_readback') != intent):
            raise ReadbackError('readback fence changed')
        done = {'status': 'done', 'intent': copy.deepcopy(intent), 'receipt': copy.deepcopy(receipt)}
        phases.journal.state['steps']['backup-readback'] = done
        phases.journal.save()
        current['backup_readback'] = copy.deepcopy(done)
        current['step'] = 'backup-readback-done'
        phases.fence.ledger.write(current_revision, current)
        phases._guard()
        confirmed, confirmed_revision = phases.fence.ledger.read()
        if (not phases.fence.owned(confirmed) or confirmed.get('status') != 'active' or
                not phases._owned_invocation(confirmed) or confirmed.get('backup_upload') != planned or
                confirmed.get('backup_readback') != done or not isinstance(confirmed_revision, tuple) or
                len(confirmed_revision) != 2 or any(not isinstance(v, str) or not v for v in confirmed_revision)):
            raise ReadbackError('terminal readback changed')
        return receipt, manifest
    except BaseException:
        if pending:
            phases.poisoned = True
            try:
                phases.fence.uncertain('media artifact readback outcome uncertain')
            except BaseException:
                pass
        raise ReadbackError('readback not complete; retain lock and reconcile') from None
