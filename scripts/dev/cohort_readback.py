"""Fenced GitHub cohort readback and isolated restore, with no replay."""
import copy
import hashlib
import json
import os
from pathlib import Path
import re

import cohort_upload_phase as upload
import media_backup_readback as media_readback
import story_media_backup as backup
import story_media_backup_cohort as cohort
import story_media_backup_cohort_custody as custody
import story_media_backup_cohort_finalize as finalize

WORKFLOW = '.github/workflows/dev-cohort-backup-upload.yml'
API = 'https://api.github.com/repos/Poryadok/VoiceRoot/actions/runs/'
get_json = media_readback.get_json


class ReadbackError(RuntimeError): pass


def workflow_evidence(token, planned):
    try:
        run_id = planned['run_id']
        if not isinstance(run_id, str) or not re.fullmatch(r'[1-9][0-9]{0,19}', run_id): raise ValueError()
        run = get_json(token, API + run_id)
        if (type(run.get('id')) is not int or run['id'] != int(run_id) or run.get('head_sha') != planned['source_sha']
                or run.get('head_branch') != 'master' or run.get('event') != 'workflow_dispatch'
                or run.get('path') != WORKFLOW or type(run.get('run_attempt')) is not int or run['run_attempt'] != 1
                or run.get('status') != 'completed' or run.get('conclusion') != 'success'): raise ValueError()
        listing = get_json(token, API + run_id + '/artifacts?per_page=100')
        items = listing.get('artifacts'); name = 'voice-cohort-backup-' + planned['operation_id'] + '-' + planned['challenge']
        if (not isinstance(items, list) or type(listing.get('total_count')) is not int
                or listing['total_count'] != len(items) or len(items) > 100): raise ValueError()
        found = [x for x in items if isinstance(x, dict) and x.get('name') == name]
        if (len(found) != 1 or type(found[0].get('id')) is not int or found[0]['id'] <= 0
                or not isinstance(found[0].get('created_at'), str)): raise ValueError()
        return found[0]['id'], found[0]['created_at']
    except Exception:
        raise ReadbackError('qualified workflow evidence unavailable') from None


def _snapshot(phase, row):
    p = phase.phases; binding = copy.deepcopy(phase.binding); databases = binding['databases']
    finalize.validate_binding(binding, databases)
    if (any(binding.get(k) != p.fence.context.get(k) for k in
            ('project','resource','actor','claim','window','generation','revision','operation_id'))
            or binding['source_sha'] != p.fence.context.get('commit')): raise ValueError()
    if (not p.fence.owned(row) or row.get('status') != 'active' or not p._owned_invocation(row)
            or set(row.get('cohort_members', {})) != phase.members or 'backup_members' in row): raise ValueError()
    receipts = {}
    for member in phase.members:
        entry = p.journal.state['steps']['cohort-dump-' + member]; receipt = entry['receipt']
        source = binding['sources'][member]; engine = source['engine']
        wanted_receipt = {'operation_id':binding['operation_id'], 'snapshot_id':binding['snapshot_id'],
            'member':member, **{k:engine[k] for k in ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')},
            'server_start':source['server_start'], 'database_oid':source.get('database_oid'),
            'attempt_nonce':entry['nonce'], 'exit':0, 'scope':'dump-terminal-only'}
        if (receipt != wanted_receipt or type(receipt.get('exit')) is not int
                or type(receipt.get('restart_count')) is not int
                or (member != 'roles' and type(receipt.get('database_oid')) is not int)
                or not isinstance(entry['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',entry['nonce'])): raise ValueError()
        digest = hashlib.sha256(json.dumps(receipt, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        expected = {'status':'done', 'nonce':entry['nonce'], 'snapshot_id':binding['snapshot_id'],
                    'operation_id':binding['operation_id'], 'receipt_sha256':digest}
        if entry['status'] != 'done' or row['cohort_members'][member] != expected: raise ValueError()
        receipts[member] = digest
    if len({p.journal.state['steps']['cohort-dump-'+m]['nonce'] for m in phase.members}) != len(phase.members): raise ValueError()
    marker = row['cohort_bundle']; planned = row['cohort_upload']
    marker_keys = {'status','operation_id','snapshot_id','invocation_nonce','bundle_bytes','bundle_sha256','manifest_sha256','receipt_sha256'}
    upload_keys = {'status','operation_id','snapshot_id','source_sha','workflow_sha','invocation_nonce','challenge',
                   'bundle_bytes','bundle_sha256','members','receipt_sha256','manifest_sha256','run_id','run_attempt'}
    if (not isinstance(marker,dict) or set(marker) != marker_keys or marker.get('status') != 'done'
            or p.journal.state['steps'].get('cohort-bundle') != {'status':'done','marker':marker}
            or not isinstance(planned,dict) or set(planned) != upload_keys or planned['status'] != 'upload-running'
            or type(planned['run_attempt']) is not int or planned['run_attempt'] != 1
            or not isinstance(planned['run_id'],str) or not re.fullmatch(r'[1-9][0-9]{0,19}',planned['run_id'])
            or not isinstance(planned['challenge'],str) or not re.fullmatch(r'[0-9a-f]{32}',planned['challenge'])
            or type(planned['bundle_bytes']) is not int or not 0 < planned['bundle_bytes'] <= 64*1024**3
            or any(not isinstance(planned[k],str) or not re.fullmatch(r'[0-9a-f]{64}',planned[k]) for k in ('bundle_sha256','manifest_sha256'))
            or planned['members'] != sorted(phase.members) or planned['receipt_sha256'] != receipts
            or planned['operation_id'] != binding['operation_id'] or planned['snapshot_id'] != binding['snapshot_id']
            or planned['source_sha'] != binding['source_sha'] or planned['workflow_sha'] != binding['source_sha']
            or planned['invocation_nonce'] != p.invocation
            or any(marker[k] != planned[k] for k in ('operation_id','snapshot_id','invocation_nonce','bundle_bytes','bundle_sha256','manifest_sha256','receipt_sha256'))): raise ValueError()
    local = p.journal.state['steps'].get('cohort-upload')
    if local != {k: ('dispatch-pending' if k == 'status' else v) for k,v in planned.items() if k not in ('run_id','run_attempt')}: raise ValueError()
    return binding | {'dump_receipts':receipts}, copy.deepcopy(planned), copy.deepcopy(marker), copy.deepcopy(row['cohort_members'])


def _guard(phase, drain):
    if phase._guard() is False or drain() is False: raise ValueError('authority or drain unavailable')


def _poison(phase):
    phase.phases.poisoned = True
    try: phase.phases.fence.uncertain('cohort readback or restore outcome uncertain')
    except BaseException: pass


def run(phase, token, key, bundle, destination, drain):
    """Read every remote ciphertext byte and persist its fenced receipt once."""
    p = phase.phases; pending = False
    try:
        _guard(phase, drain); row, revision = p.fence.ledger.read()
        binding, planned, marker, members = _snapshot(phase, row)
        if (not isinstance(key,bytes) or len(key) != 32 or 'cohort_readback' in row
                or 'cohort-readback' in p.journal.state['steps']
                or Path(bundle) != upload.ROOT/planned['operation_id']/'cohort-backup'/'cohort-backup.tar'
                or Path(destination) != upload.ROOT/planned['operation_id']/('cohort-readback-'+planned['run_id']+'-'+planned['challenge'])): raise ValueError()
        artifact_id, created = workflow_evidence(token, planned)
        artifact = {k:planned[k] for k in ('operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256')}
        artifact.update(run_id=int(planned['run_id']), head_sha=planned['source_sha'], created_at=created)
        intent = {'status':'pending','artifact_id':artifact_id,'binding':artifact,
                  'manifest_sha256':marker['manifest_sha256'],'invocation_nonce':p.invocation}
        _guard(phase, drain); pending = True
        p.journal.state['steps']['cohort-readback'] = copy.deepcopy(intent); p.journal.save()
        current, rev = p.fence.ledger.read()
        if current != row or rev != revision: raise ValueError()
        current['cohort_readback'] = copy.deepcopy(intent); current['step'] = 'cohort-readback-pending'
        p.fence.ledger.write(rev, current)
        _guard(phase, drain); reserved, _ = p.fence.ledger.read()
        if _snapshot(phase, reserved) != (binding,planned,marker,members) or reserved.get('cohort_readback') != intent: raise ValueError()
        with backup.private_directory(Path(destination).parent) as parent:
            os.mkdir(Path(destination).name, 0o700, dir_fd=parent); os.fsync(parent)
        receipt, manifest = custody.readback_extract(token, artifact, artifact_id, bundle, key, binding, binding['databases'], destination)
        wanted = {'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot',
                  'artifact_id':artifact_id, **{k:v for k,v in artifact.items() if k != 'created_at'}}
        digest = hashlib.sha256(json.dumps(manifest,sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()
        if receipt != wanted or digest != marker['manifest_sha256']: raise ValueError()
        cohort.verify(destination, key, binding, binding['databases'], manifest)
        _guard(phase, drain); current, rev = p.fence.ledger.read()
        if _snapshot(phase,current) != (binding,planned,marker,members) or current.get('cohort_readback') != intent: raise ValueError()
        done = {'status':'done','intent':copy.deepcopy(intent),'receipt':copy.deepcopy(receipt)}
        p.journal.state['steps']['cohort-readback'] = copy.deepcopy(done); p.journal.save()
        current['cohort_readback'] = copy.deepcopy(done); current['step'] = 'cohort-readback-done'
        p.fence.ledger.write(rev,current)
        _guard(phase, drain); confirmed, _ = p.fence.ledger.read()
        if _snapshot(phase,confirmed) != (binding,planned,marker,members) or confirmed.get('cohort_readback') != done: raise ValueError()
        return receipt, manifest
    except BaseException:
        if pending: _poison(phase)
        raise ReadbackError('cohort readback not complete; retain lock and reconcile' if pending else 'cohort readback not admitted') from None
