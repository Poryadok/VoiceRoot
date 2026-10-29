"""C1 config completion custody for the C2 predecessor transition.

This captures only source-owned backend-config effects from the actual CUT and
configuration journals. Other phase histories are not represented here.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import stat

import media_backup_upload_phase
import story_media_backup


class CustodyError(RuntimeError):
    pass


FILENAME = 'config-completion.json'
MARKER = 'full_cohort_config_completion'
LIMIT = 1048576


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()).hexdigest()


def checked_records(operation, invocation, services, local, ledger):
    """Check exact local/ledger effect pairing; caller establishes authority."""
    try:
        if (not isinstance(operation, str) or not re.fullmatch(r'media-c1-[a-f0-9]{32}', operation) or
                not isinstance(invocation, str) or not re.fullmatch(r'[a-f0-9]{32}', invocation) or
                not isinstance(services, list) or not services or services != sorted(set(services)) or
                any(name not in {'chat','notification','realtime','space','user'} for name in services) or
                not isinstance(local, dict) or not isinstance(ledger, dict)):
            raise ValueError()
        expected = {'backend-config-deployment-voice-' + name for name in services}
        allowed_service = {'backend-config-service-voice-chat', 'backend-config-service-voice-realtime'}
        names = set(local)
        if not expected <= names or names - expected - allowed_service or names != set(ledger): raise ValueError()
        for name in names:
            entry, remote = local[name], ledger[name]
            if (not isinstance(entry, dict) or set(entry) != {'status','nonce','details_sha256','details','receipt'} or
                    entry['status'] != 'done' or not re.fullmatch(r'[a-f0-9]{32}', entry['nonce']) or
                    entry['details_sha256'] != digest(entry['details']) or
                    remote != {'status':'done','nonce':entry['nonce'], 'details_sha256':entry['details_sha256'],
                               'receipt_sha256':digest(entry['receipt'])}): raise ValueError()
            details, receipt = entry['details'], entry['receipt']
            if (not isinstance(details, dict) or set(details) != {'operation_id','invocation_nonce','source','target_spec','patch'} or
                    details['operation_id'] != operation or details['invocation_nonce'] != invocation or
                    not isinstance(details['source'], dict) or not isinstance(details['target_spec'], dict) or
                    not isinstance(details['patch'], list) or not isinstance(receipt, dict) or
                    receipt.get('uid') != details['source'].get('metadata',{}).get('uid') or
                    receipt.get('spec_sha256') != digest(details['target_spec']) or
                    any(not isinstance(receipt.get(k), str) or not receipt[k] for k in ('uid','resourceVersion','responseResourceVersion'))):
                raise ValueError()
            if name in expected:
                if (set(receipt) != {'uid','spec_sha256','responseResourceVersion','resourceVersion','generation'} or
                        type(receipt['generation']) is not int or receipt['generation'] < 1 or
                        details['source'].get('apiVersion') != 'apps/v1' or details['source'].get('kind') != 'Deployment' or
                        details['source'].get('metadata',{}).get('namespace') != 'voice-dev' or
                        details['source'].get('metadata',{}).get('name') != name.removeprefix('backend-config-deployment-')):
                    raise ValueError()
            elif (set(receipt) not in ({'uid','spec_sha256','responseResourceVersion','resourceVersion'},
                                       {'uid','spec_sha256','responseResourceVersion','resourceVersion','generation'}) or
                  details['source'].get('apiVersion') != 'v1' or details['source'].get('kind') != 'Service' or
                  details['source'].get('metadata',{}).get('namespace') != 'voice-dev' or
                  details['source'].get('metadata',{}).get('name') != name.removeprefix('backend-config-service-')):
                raise ValueError()
            if name in allowed_service and 'generation' in receipt and (type(receipt['generation']) is not int or receipt['generation'] < 0):
                raise ValueError()
        return copy.deepcopy(local)
    except (TypeError, KeyError, ValueError, AttributeError):
        raise CustodyError('config completion records invalid') from None


def _operation_root(operation):
    if not isinstance(operation, str) or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}', operation):
        raise CustodyError('config operation invalid')
    return media_backup_upload_phase.ROOT / operation


def open_config_journal(fence, client, lock, namespace_uid, intent):
    """Open the sole config journal under the already-owned operation root."""
    import media_operation_guard
    import media_operation_journal
    try:
        if intent.get('resource_context') != fence.context or intent.get('namespace_uid') != namespace_uid:
            raise ValueError()
        media_operation_guard.verify(fence, client, lock, namespace_uid)
        root = _operation_root(fence.context['operation_id'])
        with story_media_backup.private_directory(root) as parent:
            journal = media_operation_journal.Journal(root / 'config-journal', intent)
            try:
                info = os.stat('config-journal', dir_fd=parent, follow_symlinks=False)
                held = os.fstat(journal.dirfd)
                if (info.st_dev, info.st_ino) != (held.st_dev, held.st_ino): raise ValueError()
                media_operation_guard.verify(fence, client, lock, namespace_uid)
                return journal
            except BaseException:
                journal.close(); raise
    except Exception:
        raise CustodyError('fixed config journal unavailable') from None


def _read_file(operation):
    root = _operation_root(operation)
    try:
        with story_media_backup.private_directory(root) as directory:
            fd = os.open(FILENAME, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
            try:
                before = os.fstat(fd)
                if (not stat.S_ISREG(before.st_mode) or before.st_uid != os.geteuid() or
                        stat.S_IMODE(before.st_mode) != 0o400 or before.st_nlink != 1 or
                        not 0 < before.st_size <= LIMIT): raise ValueError()
                pieces = []; remaining = before.st_size
                while remaining:
                    piece = os.read(fd, min(65536,remaining))
                    if not piece: raise ValueError()
                    pieces.append(piece); remaining -= len(piece)
                raw = b''.join(pieces)
                after = os.fstat(fd)
                current = os.stat(FILENAME, dir_fd=directory, follow_symlinks=False)
                def identity(value):
                    return (value.st_dev,value.st_ino,value.st_size,value.st_mtime_ns,value.st_ctime_ns,
                            value.st_uid,value.st_mode,value.st_nlink)
                if len(raw) != before.st_size or identity(before) != identity(after) or identity(current) != identity(before):
                    raise ValueError()
                return raw
            finally: os.close(fd)
    except (OSError, ValueError):
        raise CustodyError('private config snapshot unavailable') from None


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result: raise ValueError()
        result[key] = value
    return result


def validate_snapshot(snapshot, marker, previous, namespace_uid):
    """Validate bytes against the actual completed ledger marker and effects."""
    try:
        context_keys = {'project','resource','actor','claim','generation','window','revision','operation_id','commit','images'}
        context = {key:previous[key] for key in context_keys}
        if (not isinstance(marker,dict) or set(marker) != {'schema','status','operation_id','source_sha','namespace_uid',
                'cut_invocation','snapshot_sha256','effects_sha256'} or marker['schema'] != 'voice-full-cohort-config-marker-v1' or
                marker['status'] != 'done' or marker['operation_id'] != context['operation_id'] or
                marker['source_sha'] != context['commit'] or marker['namespace_uid'] != namespace_uid or
                not isinstance(snapshot,dict) or set(snapshot) != {'schema','resource_context','namespace_uid','cut_invocation',
                    'services','effects'} or snapshot['schema'] != 'voice-full-cohort-config-snapshot-v1' or
                snapshot['resource_context'] != context or snapshot['namespace_uid'] != namespace_uid or
                snapshot['cut_invocation'] != marker['cut_invocation'] or marker['snapshot_sha256'] != digest(snapshot) or
                marker['effects_sha256'] != digest(previous['backend_config_effects'])): raise ValueError()
        checked_records(context['operation_id'], snapshot['cut_invocation'], snapshot['services'],
                        snapshot['effects'], previous['backend_config_effects'])
        return copy.deepcopy(snapshot)
    except (KeyError, TypeError, ValueError, AttributeError):
        raise CustodyError('config snapshot binding invalid') from None


def read_completed(previous, namespace_uid):
    """Read C1 only from its ledger-selected fixed path; no caller path."""
    try:
        if not isinstance(previous,dict) or previous.get('status') != 'complete': raise ValueError()
        raw = _read_file(previous['operation_id'])
        snapshot = json.loads(raw, object_pairs_hook=_unique)
        if digest(snapshot) != hashlib.sha256(raw).hexdigest(): raise ValueError()
        return validate_snapshot(snapshot, previous[MARKER], previous, namespace_uid)
    except (KeyError, TypeError, ValueError, AttributeError):
        raise CustodyError('completed config custody unavailable') from None


def seal(fence, client, lock, namespace_uid, config_journal, cut):
    """Durably seal the C1 config effects into the active ledger before complete."""
    import backend_config_transition_deployment as backend
    import frontend_runtime_executor
    import media_operation_guard
    import media_operation_journal
    import resource_fencing
    import writer_cut
    attempted = False
    try:
        if (type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
                type(client) is not frontend_runtime_executor.KubectlTransport or fence.ledger.client is not client or
                type(cut) is not writer_cut.WriterCut or type(config_journal) is not media_operation_journal.Journal or
                cut.journal is config_journal or cut.fence.context != fence.context or cut.lock != lock or
                cut.namespace_uid != namespace_uid or cut.fence.ledger.client is not cut.client or
                cut.config_journal is not config_journal or config_journal.poisoned or
                config_journal.dirfd is None or config_journal.lockfd is None or
                config_journal.intent.get('resource_context') != fence.context or
                config_journal.intent.get('namespace_uid') != namespace_uid or
                config_journal.state.get('intent') != config_journal.intent or
                config_journal.path != _operation_root(fence.context['operation_id']) / 'config-journal' or
                not re.fullmatch(r'media-c1-[a-f0-9]{32}', fence.context['operation_id'])):
            raise ValueError()
        def guard():
            media_operation_guard.verify(fence,client,lock,namespace_uid); cut._guard()
            if cut.fence.ledger.read() != fence.ledger.read(): raise ValueError()
        guard(); cut._recheck_config_successors()
        plan = config_journal.intent['backend_config_deployments']
        if (plan['source_sha'] != fence.context['commit'] or plan['cut_invocation'] != cut.invocation or
                not isinstance(cut.config_snapshot,dict) or cut.config_snapshot.get('services') != plan['services']):
            raise ValueError()
        row, revision = fence.ledger.read()
        if not fence.owned(row) or row.get('status') != 'active' or MARKER in row: raise ValueError()
        local = {}
        for source in (cut.journal,config_journal):
            if source.poisoned or source.dirfd is None or source.lockfd is None or source.state.get('intent') != source.intent:
                raise ValueError()
            for name, entry in source.state['steps'].items():
                if not name.startswith('backend-config-') or name == 'backend-config-observation': continue
                if name in local: raise ValueError()
                local[name] = copy.deepcopy(entry)
        effects = row['backend_config_effects']
        if backend._effect_map(cut, config_journal) != effects: raise ValueError()
        records = checked_records(fence.context['operation_id'], cut.invocation, plan['services'], local, effects)
        snapshot = {'schema':'voice-full-cohort-config-snapshot-v1', 'resource_context':copy.deepcopy(fence.context),
                    'namespace_uid':namespace_uid, 'cut_invocation':cut.invocation,
                    'services':copy.deepcopy(plan['services']), 'effects':records}
        raw = json.dumps(snapshot, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()
        if not 0 < len(raw) <= LIMIT: raise ValueError()
        marker = {'schema':'voice-full-cohort-config-marker-v1','status':'done',
                  'operation_id':fence.context['operation_id'], 'source_sha':fence.context['commit'],
                  'namespace_uid':namespace_uid, 'cut_invocation':cut.invocation,
                  'snapshot_sha256':hashlib.sha256(raw).hexdigest(), 'effects_sha256':digest(effects)}
        guard(); cut._recheck_config_successors()
        if fence.ledger.read() != (row,revision) or backend._effect_map(cut,config_journal) != effects:
            raise ValueError()
        root = _operation_root(fence.context['operation_id'])
        with story_media_backup.private_directory(root) as directory:
            entry = os.stat('config-journal', dir_fd=directory, follow_symlinks=False); held = os.fstat(config_journal.dirfd)
            if (entry.st_dev,entry.st_ino) != (held.st_dev,held.st_ino): raise ValueError()
            attempted = True
            fd = os.open(FILENAME, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=directory)
            try:
                data = memoryview(raw)
                while data:
                    count = os.write(fd,data)
                    if count <= 0: raise ValueError()
                    data = data[count:]
                os.fchmod(fd,0o400); os.fsync(fd)
            finally: os.close(fd)
            os.fsync(directory)
        if _read_file(fence.context['operation_id']) != raw: raise ValueError()
        guard(); cut._recheck_config_successors()
        if (fence.ledger.read() != (row,revision) or backend._effect_map(cut,config_journal) != effects or
                checked_records(fence.context['operation_id'],cut.invocation,plan['services'],local,effects) != records):
            raise ValueError()
        current = copy.deepcopy(row); current[MARKER] = marker
        fence.ledger.write(revision,current)
        guard(); actual, _ = fence.ledger.read()
        if actual != current or _read_file(fence.context['operation_id']) != raw: raise ValueError()
        return copy.deepcopy(marker)
    except BaseException:
        if attempted:
            try: fence.uncertain('config completion custody uncertain')
            except BaseException: pass
        raise CustodyError('config completion unavailable; reconcile if attempted') from None


def load_predecessor(fence, client, lock, namespace_uid, predecessor_journal):
    """Return a detached private proof only through the active C2 predecessor."""
    import full_cohort_predecessor_phase as predecessor
    try:
        result = predecessor.load(fence,client,lock,namespace_uid,predecessor_journal)
        config = result['config_completion']['snapshot']
        return {'schema':'voice-full-cohort-config-predecessor-v1',
                'operation_id':result['previous_operation_id'], 'source_sha':result['previous_commit'],
                'namespace_uid':namespace_uid, 'cut_invocation':config['cut_invocation'],
                'services':copy.deepcopy(config['services']), 'effects':copy.deepcopy(config['effects'])}
    except Exception:
        raise CustodyError('config predecessor unavailable') from None
