"""Private C1 tool evidence carried through the fenced C2 predecessor."""
import copy
import hashlib
import json
import os
import re
import stat

import media_backup_upload_phase
import story_media_backup
import space_native_tool_identity as tool

FILENAME = 'space-native-tool-completion.json'
MARKER = 'space_native_tool_completion'
LIMIT = 1048576

class CustodyError(RuntimeError):
    pass

def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode()).hexdigest()

def _equivalent(contract,source):
    files=source['files']
    if (set(contract)!={'schema','packages','files','workspace','raw_sha256'} or
        contract['schema']!='voice-space-native-local-closure-v1' or not isinstance(contract['packages'],dict) or
        not contract['packages'] or not re.fullmatch(r'[a-f0-9]{64}',contract['raw_sha256']) or
        not isinstance(contract['files'],dict) or any(not isinstance(k,str) or not isinstance(v,str) or
            not re.fullmatch(r'[a-f0-9]{64}',v) or files.get(k)!=v for k,v in contract['files'].items())): raise ValueError()
    if contract['workspace']!=[name for name in ('src/backend/go.work','src/backend/go.work.sum') if name in files]: raise ValueError()
    module_dirs={name.removesuffix('/go.mod') for name in contract['files'] if name.endswith('/go.mod')}
    for directory in module_dirs:
        if any((directory+'/'+name in files)!=(directory+'/'+name in contract['files']) for name in ('go.mod','go.sum')): raise ValueError()
    for directory,members in contract['packages'].items():
        if (not isinstance(directory,str) or not directory.startswith('src/backend/') or
            not isinstance(members,list) or members!=sorted(set(members)) or any(name not in contract['files'] for name in members)):
            raise ValueError()
        actual=sorted(name for name in files if name.startswith(directory+'/') and '/' not in name.removeprefix(directory+'/')
                      and name.endswith(('.go','.s','.S','.c','.cc','.cpp','.cxx','.h','.hh','.hpp','.hxx','.f','.F','.m','.mm','.syso','.swig','.swigcxx')) and not name.endswith('_test.go'))
        if actual!=members: raise ValueError()

def _root(operation):
    if not isinstance(operation,str) or not re.fullmatch(r'media-c1-[a-f0-9]{32}',operation):
        raise CustodyError('tool predecessor operation invalid')
    return media_backup_upload_phase.ROOT / operation

def _read(operation):
    try:
        with story_media_backup.private_directory(_root(operation)) as directory:
            fd=os.open(FILENAME,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=directory)
            try:
                before=os.fstat(fd)
                if (not stat.S_ISREG(before.st_mode) or before.st_uid!=os.geteuid() or
                    stat.S_IMODE(before.st_mode)!=0o400 or before.st_nlink!=1 or not 0<before.st_size<=LIMIT): raise ValueError()
                data=bytearray()
                while len(data)<before.st_size:
                    part=os.read(fd,min(65536,before.st_size-len(data)))
                    if not part: raise ValueError()
                    data.extend(part)
                def identity(s): return (s.st_dev,s.st_ino,s.st_size,s.st_mtime_ns,s.st_ctime_ns,s.st_uid,s.st_mode,s.st_nlink)
                if identity(before)!=identity(os.fstat(fd)) or identity(before)!=identity(os.stat(FILENAME,dir_fd=directory,follow_symlinks=False)):
                    raise ValueError()
                return bytes(data)
            finally: os.close(fd)
    except (OSError,ValueError): raise CustodyError('private tool snapshot unavailable') from None

def _unique(pairs):
    out={}
    for key,value in pairs:
        if key in out: raise ValueError()
        out[key]=value
    return out

def validate_snapshot(snapshot,marker,previous,namespace_uid):
    try:
        if (set(snapshot)!={'schema','resource_context','namespace_uid','receipt','inventory','contract','tool_marker'} or
            snapshot['schema']!='voice-space-native-tool-completion-v1' or snapshot['namespace_uid']!=namespace_uid or
            set(marker)!={'schema','status','operation_id','source_sha','namespace_uid','snapshot_sha256'} or
            marker['schema']!='voice-space-native-tool-completion-marker-v1' or marker['status']!='done' or
            marker['operation_id']!=previous['operation_id'] or marker['source_sha']!=previous['commit'] or
            marker['namespace_uid']!=namespace_uid or marker['snapshot_sha256']!=digest(snapshot)):
            raise ValueError()
        context=snapshot['resource_context']; receipt=snapshot['receipt']; inventory=snapshot['inventory']; done=snapshot['tool_marker']
        keys={'project','resource','actor','claim','generation','window','revision','operation_id','commit','images'}
        if context!={k:previous[k] for k in keys} or done!=previous['space_native_tool'] or done.get('status')!='done': raise ValueError()
        if (receipt['operation_id']!=previous['operation_id'] or receipt['namespace_uid']!=namespace_uid or
            receipt['source_sha']!=previous['commit'] or done['receipt_sha256']!=digest(receipt) or
            inventory['source_sha']!=previous['commit'] or
            any(receipt[k]!=inventory[k] for k in ('build_source_sha256','sql_inventory_sha256','dockerfile_sha256','source_manifest_sha256')) or
            receipt['runtime_receipt_verified'] is not False or receipt['compiled_gate_probe']!='native51-source-gates-reached-v1'):
            raise ValueError()
        if not isinstance(snapshot['contract'],dict) or receipt['closure_sha256']!=digest(snapshot['contract']): raise ValueError()
        return copy.deepcopy(snapshot)
    except (KeyError,TypeError,ValueError,AttributeError): raise CustodyError('tool snapshot binding invalid') from None

def read_completed(previous,namespace_uid):
    try:
        if previous.get('status')!='complete': raise ValueError()
        raw=_read(previous['operation_id']); snapshot=json.loads(raw,object_pairs_hook=_unique)
        if digest(snapshot)!=hashlib.sha256(raw).hexdigest(): raise ValueError()
        return validate_snapshot(snapshot,previous[MARKER],previous,namespace_uid)
    except (KeyError,TypeError,ValueError,AttributeError): raise CustodyError('completed tool custody unavailable') from None

def seal(phase,token):
    """Seal authenticated C1 evidence before completion; unknown write means HOLD."""
    import canonical_application_package as canonical
    attempted=False
    try:
        if type(phase) is not tool.SpaceNativeToolPhase: raise ValueError()
        receipt=tool.verify_completed(phase,token); package=phase._package(token)
        artifact=tool.authenticated_publication(token,phase.actor,package,canonical.ROOT)
        if phase._receipt(package,artifact)!=receipt: raise ValueError()
        inventory=copy.deepcopy(artifact['publication']['source_inventory'])
        contract=copy.deepcopy(artifact['publication']['content']['closure'])
        phase._guard(); row,revision=phase.fence.ledger.read()
        if not phase.fence.owned(row) or row.get('status')!='active' or MARKER in row: raise ValueError()
        snapshot={'schema':'voice-space-native-tool-completion-v1','resource_context':copy.deepcopy(phase.fence.context),
                  'namespace_uid':phase.namespace_uid,'receipt':receipt,'inventory':inventory,'contract':contract,
                  'tool_marker':copy.deepcopy(row['space_native_tool'])}
        raw=json.dumps(snapshot,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
        if not 0<len(raw)<=LIMIT: raise ValueError()
        marker={'schema':'voice-space-native-tool-completion-marker-v1','status':'done',
                'operation_id':phase.fence.context['operation_id'],'source_sha':receipt['source_sha'],
                'namespace_uid':phase.namespace_uid,'snapshot_sha256':hashlib.sha256(raw).hexdigest()}
        validate_snapshot(snapshot,marker,row,phase.namespace_uid)
        phase._guard()
        if phase.fence.ledger.read()!=(row,revision) or tool.verify_completed(phase,token)!=receipt: raise ValueError()
        with story_media_backup.private_directory(_root(row['operation_id'])) as directory:
            attempted=True
            fd=os.open(FILENAME,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=directory)
            try:
                data=memoryview(raw)
                while data:
                    count=os.write(fd,data)
                    if count<=0: raise ValueError()
                    data=data[count:]
                os.fchmod(fd,0o400); os.fsync(fd)
            finally: os.close(fd)
            os.fsync(directory)
        if _read(row['operation_id'])!=raw: raise ValueError()
        phase._guard()
        if phase.fence.ledger.read()!=(row,revision) or tool.verify_completed(phase,token)!=receipt: raise ValueError()
        current=copy.deepcopy(row); current[MARKER]=marker
        phase.fence.ledger.write(revision,current); phase._guard()
        if phase.fence.ledger.read()[0]!=current or _read(row['operation_id'])!=raw: raise ValueError()
        return copy.deepcopy(marker)
    except BaseException:
        if attempted:
            try: phase.fence.uncertain('Space native tool completion custody uncertain')
            except BaseException: pass
        raise CustodyError('tool completion unavailable; reconcile if attempted') from None

def reuse(fence,client,lock,namespace_uid,predecessor_journal,package,token):
    """Qualify the original C1 artifact against a separately admitted C2 source."""
    import canonical_application_package as canonical
    import full_cohort_package_phase
    import full_cohort_predecessor_phase as predecessor
    import publish_candidate
    import resource_fencing
    import frontend_runtime_executor
    import media_operation_journal
    try:
        if (type(package) is not full_cohort_package_phase.PackagePhase or
            type(package.fence) is not resource_fencing.Fence or
            type(package.client) is not frontend_runtime_executor.KubectlTransport or
            type(package.journal) is not media_operation_journal.Journal or package.client.journal is not package.journal or
            package.journal is predecessor_journal or
            type(package.fence.ledger) is not resource_fencing.KubeLedger or
            package.fence.ledger.client is not package.client or package.fence.context!=fence.context or
            package.lock!=lock or package.namespace_uid!=namespace_uid or
            not re.fullmatch(r'media-c2-[a-f0-9]{32}',fence.context['operation_id'])): raise ValueError()
        package_fence=package.fence; package_client=package.client; package_ledger=package_fence.ledger
        package_journal=package.journal
        custody=(tool.SpaceNativeToolPhase._custody(predecessor_journal),tool.SpaceNativeToolPhase._custody(package_journal))
        snapshot=predecessor.load(fence,client,lock,namespace_uid,predecessor_journal)
        first,revision=fence.ledger.read()
        if package.fence.ledger.read()!=(first,revision): raise ValueError()
        approved=package.verify_current(token)
        source=publish_candidate.source_snapshot(canonical.ROOT,approved['source_sha'])
        candidate=tool.source_inventory(canonical.ROOT,source)
        original=snapshot['tool_completion']['snapshot']
        a=copy.deepcopy(original['inventory']); b=copy.deepcopy(candidate)
        for item in (a,b):
            del item['source_sha']; del item['build_source_sha256']
        if a!=b: raise ValueError()
        _equivalent(original['contract'],source)
        if package.verify_current(token)!=approved or predecessor.load(fence,client,lock,namespace_uid,predecessor_journal)!=snapshot:
            raise ValueError()
        if (package.fence is not package_fence or package.client is not package_client or package.journal is not package_journal or
            package_fence.ledger is not package_ledger or package_client.journal is not package_journal or
            custody!=(tool.SpaceNativeToolPhase._custody(predecessor_journal),tool.SpaceNativeToolPhase._custody(package_journal)) or
            fence.ledger.read()!=(first,revision) or package.fence.ledger.read()!=(first,revision)): raise ValueError()
        return copy.deepcopy(original['receipt'])
    except BaseException: raise CustodyError('C2 original tool authority unavailable') from None
