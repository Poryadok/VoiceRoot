"""Stage one exact encrypted cohort bundle on the trusted DEV runner.

No cooperating writer or cleanup may alter the private copy before upload and
authenticated readback complete. Unknown effects retain the common lock.
"""
import copy
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

import cohort_upload_run as run
import media_backup_upload_runner as media
import media_operation_guard
import publish_candidate
import resource_fencing
import story_media_backup as backup
import story_media_backup_cohort as cohort
import story_media_backup_cohort_custody as custody
import story_media_backup_cohort_finalize as finalize

ROOT = Path('/home/pmd/voice-dev-media-operations')
OPERATOR_UID = 1000


class RunnerError(RuntimeError):
    pass


def route_marker(binding, namespace_uid):
    raw=json.dumps(binding,sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')
    return {'version':'full-cohort-backup-route-v1','operation_id':binding['operation_id'],
            'source_sha':binding['source_sha'],'namespace_uid':namespace_uid,
            'binding_sha256':hashlib.sha256(raw).hexdigest()}


def _open_journal(directory):
    with backup.private_directory(directory) as fd:
        lock=os.open('lock',os.O_RDWR|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=fd)
        try:
            info=os.fstat(lock)
            if not stat.S_ISREG(info.st_mode) or info.st_uid!=os.geteuid() or stat.S_IMODE(info.st_mode)!=0o600 or info.st_nlink!=1:
                raise RunnerError('backup journal lock invalid')
            try:
                fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
            except BlockingIOError:
                return
            fcntl.flock(lock,fcntl.LOCK_UN)
            raise RunnerError('backup journal is not held')
        finally: os.close(lock)


def full_members(fence, lock, namespace_uid, row):
    """Derive the full member set from the fixed, held source-owned journals."""
    operation=fence.context['operation_id'];directory=ROOT/operation
    _open_journal(directory/'journal');_open_journal(directory/'backup-journal')
    root=json.loads(media.private_file(directory/'journal','state.json',1048576),object_pairs_hook=media.unique_object)
    selected=json.loads(media.private_file(directory/'backup-journal','state.json',1048576),object_pairs_hook=media.unique_object)
    root_intent=root['intent'];intent=selected['intent'];binding=intent['cohort_binding']
    planned=row['cohort_upload']
    local_planned={k:('dispatch-pending' if k=='status' else v) for k,v in planned.items()
                   if k not in ('run_id','run_attempt')}
    if (root_intent.get('cohort_route_mode')!='full-cohort-v1' or root_intent.get('resource_context')!=fence.context
            or root_intent.get('kube_lock')!=lock or root_intent.get('namespace_uid')!=namespace_uid
            or intent.get('resource_context')!=fence.context or intent.get('kube_lock')!=lock
            or intent.get('namespace_uid')!=namespace_uid or 'full_catalog' not in binding
            or binding.get('operation_id')!=operation or binding.get('source_sha')!=fence.context['commit']
            or binding.get('snapshot_id')!=planned.get('snapshot_id') or binding.get('operation_id')!=planned.get('operation_id')
            or binding.get('source_sha')!=planned.get('source_sha')
            or any(binding.get(k)!=fence.context.get(k) for k in ('project','resource','actor','claim','window','generation','revision'))
            or binding.get('sources',{}).get('roles',{}).get('engine',{}).get('namespace_uid')!=namespace_uid
            or selected['steps'].get('cohort-upload')!=local_planned
            or selected['steps'].get('cohort-dispatch')!=run.dispatch_marker(local_planned)
            or selected['steps'].get('cohort-bundle')!={'status':'done','marker':row.get('cohort_bundle')}
            or row.get('full_cohort_backup_route')!=route_marker(binding,namespace_uid)):
        raise RunnerError('full cohort source route unavailable')
    return sorted(finalize.validate_binding(binding,binding['databases']))


def route_mode(fence, lock, namespace_uid):
    state=json.loads(media.private_file(ROOT/fence.context['operation_id']/'journal','state.json',1048576),
                     object_pairs_hook=media.unique_object)
    intent=state['intent']
    if (intent.get('resource_context')!=fence.context or intent.get('kube_lock')!=lock
            or intent.get('namespace_uid')!=namespace_uid):
        raise RunnerError('root journal context changed')
    if 'cohort_route_mode' not in intent:return None
    if intent['cohort_route_mode']!='full-cohort-v1':raise RunnerError('root journal route invalid')
    return intent['cohort_route_mode']


def load(environment, operation, challenge, repo_root):
    if (os.geteuid()!=OPERATOR_UID or not isinstance(operation,str) or
            not re.fullmatch(r'media-c[12]-[0-9a-f]{32}',operation) or
            not isinstance(challenge,str) or not re.fullmatch(r'[0-9a-f]{32}',challenge)):
        raise RunnerError('fixed operator identity required')
    source = environment.get('GITHUB_SHA','')
    publish_candidate.source_snapshot(Path(repo_root),source)
    operation_dir = ROOT/operation
    root_raw = media.private_file(operation_dir/'journal','state.json',1048576)
    root_state = json.loads(root_raw,object_pairs_hook=media.unique_object)
    root_intent=root_state['intent']; context=root_intent['resource_context']
    client = media.Client()
    checker = resource_fencing.ResourceCheck(media.TT,context)
    fence = resource_fencing.Fence(context,checker,resource_fencing.KubeLedger(client),checker.mark_uncertain)
    media_operation_guard.verify(fence,client,root_intent['kube_lock'],root_intent['namespace_uid'])
    row, revision=fence.ledger.read()
    if not fence.owned(row) or row.get('status')!='active' or not isinstance(revision,tuple) or len(revision)!=2:
        raise RunnerError('backup route authority unavailable')
    route=row.get('full_cohort_backup_route')
    mode=root_intent.get('cohort_route_mode')
    if ((mode=='full-cohort-v1' and not isinstance(route,dict)) or
            (mode is None and 'cohort_route_mode' not in root_intent and 'full_cohort_backup_route' in row) or
            (mode!='full-cohort-v1' and ('cohort_route_mode' in root_intent or 'full_cohort_backup_route' in row))):
        raise RunnerError('full cohort route invalid')
    journal_dir=operation_dir/('backup-journal' if route is not None else 'journal')
    if route is not None:
        _open_journal(operation_dir/'journal')
        _open_journal(journal_dir)
        raw=media.private_file(journal_dir,'state.json',1048576)
        state=json.loads(raw,object_pairs_hook=media.unique_object)
    else:
        raw=root_raw;state=root_state
    intent = state['intent']; context = intent['resource_context']; planned = state['steps']['cohort-upload']
    binding = intent['cohort_binding']; databases = binding['databases']
    if route is not None and (route != route_marker(binding,intent['namespace_uid']) or
            context!=root_intent['resource_context'] or intent['kube_lock']!=root_intent['kube_lock'] or
            intent['namespace_uid']!=root_intent['namespace_uid'] or 'full_catalog' not in binding):
        raise RunnerError('full cohort journal route invalid')
    members = sorted(finalize.validate_binding(binding,databases))
    local_marker = state['steps']['cohort-bundle']
    marker = local_marker.get('marker', {})
    if (set(state)!={'format','intent','steps'} or state.get('format')!=1 or
            context['operation_id']!=operation or binding['operation_id']!=operation or
            binding['source_sha']!=source or context['commit']!=source or
            any(binding.get(k)!=context.get(k) for k in
                ('project','resource','actor','claim','window','generation','revision')) or
            binding['sources']['roles']['engine']['namespace_uid']!=intent['namespace_uid'] or
            planned.get('status')!='dispatch-pending' or
            planned.get('operation_id')!=operation or planned.get('challenge')!=challenge or
            planned.get('workflow_sha')!=source or planned.get('members')!=members or
            state['steps'].get('cohort-dispatch')!=run.dispatch_marker(planned) or
            local_marker!={'status':'done','marker':marker} or
            marker!={'status':'done','operation_id':operation,'snapshot_id':binding['snapshot_id'],
                     'invocation_nonce':planned.get('invocation_nonce'),
                     'bundle_bytes':planned.get('bundle_bytes'),'bundle_sha256':planned.get('bundle_sha256'),
                     'manifest_sha256':planned.get('manifest_sha256'),'receipt_sha256':planned.get('receipt_sha256')}):
        raise RunnerError('journal identity invalid')
    if fence.ledger.read()!=(row,revision) or media.private_file(operation_dir/'journal','state.json',1048576)!=root_raw:
        raise RunnerError('backup route changed')
    return operation_dir, raw, state, intent, planned, binding, databases, members, client, fence, journal_dir, route, root_raw


def stage(environment, operation, challenge, repo_root):
    data = load(environment,operation,challenge,repo_root)
    operation_dir, raw, state, intent, planned, binding, databases, members, client, fence, journal_dir, route, root_raw = data
    lock = intent['kube_lock']; namespace_uid = intent['namespace_uid']
    admitted = run.claim(fence,client,lock,namespace_uid,environment,challenge,members)
    try:
        original = {k:v for k,v in admitted.items() if k not in ('run_id','run_attempt')}
        original['status'] = 'dispatch-pending'
        if original != planned: raise RunnerError('run reservation differs')
        row, _ = fence.ledger.read()
        if row.get('full_cohort_backup_route')!=route: raise RunnerError('backup route changed')
        digests = {}
        for member in members:
            saved = state['steps']['cohort-dump-'+member]
            receipt = saved['receipt']; source = binding['sources'][member]
            engine = source['engine']
            expected_receipt = {'operation_id':operation,'snapshot_id':binding['snapshot_id'],'member':member,
                **{k:engine[k] for k in ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')},
                'server_start':source['server_start'],'database_oid':source.get('database_oid'),
                'attempt_nonce':saved.get('nonce'),'exit':0,'scope':'dump-terminal-only'}
            digest = hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
            if (saved.get('status')!='done' or receipt!=expected_receipt or type(receipt.get('exit')) is not int or
                    type(receipt.get('restart_count')) is not int or
                    (member!='roles' and type(receipt.get('database_oid')) is not int) or
                    row['cohort_members'][member] != {'status':'done','nonce':saved.get('nonce'),
                        'snapshot_id':binding['snapshot_id'],'operation_id':operation,'receipt_sha256':digest}):
                raise RunnerError('cohort receipt changed')
            digests[member] = digest
        if (digests != planned['receipt_sha256'] or state['steps']['cohort-bundle'] !=
                {'status':'done','marker':row.get('cohort_bundle')}):
            raise RunnerError('cohort bundle marker changed')
        media_operation_guard.verify(fence,client,lock,namespace_uid)
        checked, revision = fence.ledger.read()
        if (not fence.owned(checked) or checked.get('status')!='active' or
                checked.get('cohort_upload')!=admitted or checked.get('cohort_dispatch')!=run.dispatch_marker(planned) or checked.get('cohort_bundle')!=row.get('cohort_bundle') or
                checked.get('cohort_members')!=row.get('cohort_members') or
                checked.get('full_cohort_backup_route')!=route or
                checked.get('backup_invocation')!={'nonce':admitted['invocation_nonce'],'operation_id':operation} or
                not isinstance(revision,tuple) or len(revision)!=2 or
                any(not isinstance(v,str) or not v for v in revision)):
            raise RunnerError('cohort authority changed before private read')
        final_binding = binding | {'dump_receipts':digests}
        key = media.private_file(operation_dir,'backup.key',32)
        if len(key)!=32: raise RunnerError('private key invalid')
        original = operation_dir/'cohort-backup'/'cohort-backup.tar'
        expected = {'bytes':planned['bundle_bytes'],'sha256':planned['bundle_sha256']}
        if custody.verify_bundle(original,key,final_binding,databases)!=expected:
            raise RunnerError('cohort bundle changed')
        scratch = operation_dir/('cohort-upload-'+admitted['run_id']+'-'+challenge)
        scratch.mkdir(mode=0o700)
        with backup.private_directory(operation_dir) as operation_fd: os.fsync(operation_fd)
        target = scratch/'cohort-backup.tar'
        with backup.private_directory(original.parent) as source_dir, backup.private_directory(scratch) as target_dir:
            fd = os.open(original.name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=source_dir)
            out_fd = os.open(target.name,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=target_dir)
            with os.fdopen(fd,'rb') as src, os.fdopen(out_fd,'wb') as dst:
                before = os.fstat(src.fileno())
                if (not stat.S_ISREG(before.st_mode) or before.st_uid!=os.geteuid() or
                        stat.S_IMODE(before.st_mode)!=0o600 or before.st_nlink!=1 or before.st_size!=expected['bytes']):
                    raise RunnerError('cohort source custody invalid')
                digest = hashlib.sha256(); count = 0
                for chunk in iter(lambda:src.read(media.CHUNK),b''):
                    count += len(chunk)
                    if count>media.LIMIT: raise RunnerError('cohort bundle too large')
                    digest.update(chunk); dst.write(chunk)
                dst.flush(); os.fsync(dst.fileno()); after = os.fstat(src.fileno())
                if ((before.st_dev,before.st_ino,before.st_size,before.st_mtime_ns) !=
                        (after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns) or
                        count!=expected['bytes'] or digest.hexdigest()!=expected['sha256']):
                    raise RunnerError('cohort source changed')
            os.fsync(target_dir)
        if (custody.verify_bundle(target,key,final_binding,databases)!=expected or
                media.private_file(journal_dir,'state.json',1048576)!=raw or
                media.private_file(operation_dir/'journal','state.json',1048576)!=root_raw):
            raise RunnerError('cohort staged custody changed')
        if route is not None: _open_journal(journal_dir)
        media_operation_guard.verify(fence,client,lock,namespace_uid)
        final, revision = fence.ledger.read()
        if (not fence.owned(final) or final.get('status')!='active' or final.get('cohort_upload')!=admitted or final.get('cohort_dispatch')!=run.dispatch_marker(planned) or
                final.get('cohort_bundle')!=row.get('cohort_bundle') or final.get('cohort_members')!=row.get('cohort_members') or
                final.get('full_cohort_backup_route')!=route or
                final.get('backup_invocation')!={'nonce':admitted['invocation_nonce'],'operation_id':operation} or
                not isinstance(revision,tuple) or len(revision)!=2 or any(not isinstance(v,str) or not v for v in revision)):
            raise RunnerError('cohort upload authority changed')
        if (media.private_file(journal_dir,'state.json',1048576)!=raw or
                media.private_file(operation_dir/'journal','state.json',1048576)!=root_raw):
            raise RunnerError('cohort journal changed after guard')
        if route is not None:
            _open_journal(operation_dir/'journal');_open_journal(journal_dir)
        output = environment.get('GITHUB_OUTPUT')
        if not isinstance(output,str) or not os.path.isabs(output): raise RunnerError('workflow output unavailable')
        fd = os.open(output,os.O_WRONLY|os.O_APPEND|os.O_NOFOLLOW|os.O_NONBLOCK)
        with os.fdopen(fd,'w',encoding='utf-8') as stream:
            info = os.fstat(stream.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_uid!=os.geteuid() or info.st_mode&0o022 or info.st_nlink!=1:
                raise RunnerError('workflow output custody invalid')
            stream.write('path='+str(target)+'\nname=voice-cohort-backup-'+operation+'-'+challenge+'\n')
            stream.flush()
        return target
    except BaseException:
        try: fence.uncertain('cohort upload staging outcome uncertain')
        except BaseException: pass
        raise RunnerError('cohort upload staging uncertain; retain lock') from None


def mark_failed(environment, operation, challenge, repo_root):
    operation_dir, raw, state, intent, planned, binding, databases, members, client, fence, journal_dir, route, root_raw = load(environment,operation,challenge,repo_root)
    run.validate_environment(fence,environment,challenge)
    media_operation_guard.verify(fence,client,intent['kube_lock'],intent['namespace_uid'])
    row, revision = fence.ledger.read()
    actual = planned | {'status':'upload-running','run_id':environment['GITHUB_RUN_ID'],'run_attempt':1}
    if (not fence.owned(row) or row.get('status')!='active' or row.get('cohort_upload')!=actual or row.get('cohort_dispatch')!=run.dispatch_marker(planned) or
            row.get('full_cohort_backup_route')!=route or
            row.get('backup_invocation')!={'nonce':planned['invocation_nonce'],'operation_id':operation} or
            not isinstance(revision,tuple) or len(revision)!=2 or any(not isinstance(v,str) or not v for v in revision)):
        raise RunnerError('failed cohort upload identity changed')
    fence.uncertain('GitHub cohort upload failed or outcome uncertain')


def main():
    try:
        action = sys.argv[1:]
        if action == ['--mark-failed']: mark_failed(dict(os.environ),os.environ.get('DEV_BACKUP_OPERATION'),os.environ.get('DEV_BACKUP_CHALLENGE'),Path.cwd())
        elif not action: stage(dict(os.environ),os.environ.get('DEV_BACKUP_OPERATION'),os.environ.get('DEV_BACKUP_CHALLENGE'),Path.cwd())
        else: raise RunnerError('unsupported mode')
        return 0
    except BaseException:
        print('DEV cohort upload refused; retain operation and reconcile',file=sys.stderr)
        return 1


if __name__ == '__main__': sys.exit(main())
