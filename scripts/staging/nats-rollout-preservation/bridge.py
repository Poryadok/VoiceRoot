"""Durable fixed-action root bridge protocol. Caller must hold operation_lock."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import uuid
from contextlib import nullcontext

class BridgeError(RuntimeError):pass

FIELDS={
    'prepare':{'source_sha','run_id','mode','changed_services','token'},
    'prepare-repair':{'source_sha','run_id','mode','changed_services','token','operation'},
    'accept-provisional':{'operation','run_id','token'},
    'prepare-rollback':{'operation','run_id','token'},
    'resume-cold-backup':{'operation','run_id','token'},
    'resume-cipher-upload':{'operation','run_id','token'},
    'authorize-preserved-upload':{'operation','artifact_id','upload_nonce','run_id','token'},
    'prepare-expired-native':{'operation','run_id','token'},
    'prepare-expired-finish':{'operation','run_id','token'},
    'authorize-expired-native':{'operation','original_artifact_id','observation_artifact_id','upload_nonce','run_id','token'},
    'authorize-expired-finish':{'operation','original_artifact_id','observation_artifact_id','upload_nonce','run_id','token'},
    'finish-expired-native':{'operation','claim_rv','run_id','token'},
    'authorize':{'operation','artifact_id','token'},
    'finish':{'operation','claim_rv'},
    'status':{'operation'},
}
PREPARE_STAGES=frozenset(('source-capture','workload-capture','policy-validation',
    'image-selection','contract-selection','nats-preflight','target-build'))

def prepare_error(error):
    # Class labels only: no exception text, arguments or remote response body.
    name=type(error).__name__
    if name=='SourceError':return 'source_authority_rejected'
    if name=='CustodyError':return 'transport_rejected'
    if name=='Blocked':return 'guard_rejected'
    if name in ('KeyError','ValueError','TypeError','FileExistsError','PermissionError','TimeoutError'):
        return 'exception_'+name
    if isinstance(error,(KeyboardInterrupt,SystemExit)):return 'interrupted'
    return 'unexpected'

def validate(row):
    optional={'run_attempt','report_run_id','report_source_sha','resource_context'}
    if isinstance(row,dict) and row.get('action') in ('prepare','prepare-repair'):
        optional.update(('provisional_staging','gis_first_install'))
    if not isinstance(row,dict) or row.get('action') not in FIELDS or set(row)-optional!={'action','nonce'}|FIELDS[row['action']]:raise BridgeError('bridge_request_invalid')
    if 'resource_context' in row:
        import resource_claim
        try:resource_claim.validate(row['resource_context'])
        except (ValueError,TypeError,AttributeError):raise BridgeError('bridge_resource_context_invalid') from None
    if 'provisional_staging' in row and (type(row['provisional_staging']) is not bool or row['mode']!='images-only'):raise BridgeError('bridge_target_invalid')
    if 'gis_first_install' in row and (type(row['gis_first_install']) is not bool or row['gis_first_install'] is not True
        or row['action']!='prepare' or row['mode'] not in ('full','app-only')):
        raise BridgeError('bridge_gis_first_install_invalid')
    for key in ('run_attempt','report_run_id'):
        if key in row and (type(row[key]) is not int or not 0<row[key]<2**63):raise BridgeError('bridge_identity_invalid')
    if 'report_source_sha' in row and (not isinstance(row['report_source_sha'],str) or not re.fullmatch('[a-f0-9]{40}',row['report_source_sha'])):raise BridgeError('bridge_identity_invalid')
    if not isinstance(row['nonce'],str) or not re.fullmatch('[a-f0-9]{64}',row['nonce']):raise BridgeError('bridge_nonce_invalid')
    for key,pattern in (('operation','[a-f0-9]{12}'),('source_sha','[a-f0-9]{40}'),('claim_rv','[0-9]{1,20}')):
        if key in row and (not isinstance(row[key],str) or not re.fullmatch(pattern,row[key])):raise BridgeError('bridge_identity_invalid')
    if 'upload_nonce' in row and (not isinstance(row['upload_nonce'],str) or not re.fullmatch('[a-f0-9]{64}',row['upload_nonce'])):
        raise BridgeError('bridge_upload_nonce_invalid')
    for key in ('run_id','artifact_id','original_artifact_id','observation_artifact_id'):
        if key in row and (type(row[key]) is not int or not 0<row[key]<2**63):raise BridgeError('bridge_identity_invalid')
    if 'original_artifact_id' in row and row['original_artifact_id']==row['observation_artifact_id']:
        raise BridgeError('bridge_artifact_identity_not_distinct')
    if 'token' in row and (not isinstance(row['token'],str) or not 1<=len(row['token'])<=4096 or any(ord(c)<33 or ord(c)>126 for c in row['token'])):raise BridgeError('bridge_token_invalid')
    if row['action'] in ('prepare','prepare-repair'):
        if row['mode'] not in ('full','app-only','images-only') or not isinstance(row['changed_services'],list) or not 0<=len(row['changed_services'])<=23 or len(set(row['changed_services']))!=len(row['changed_services']) or row['mode']=='images-only' and not row['changed_services']:raise BridgeError('bridge_target_invalid')
        if any(not isinstance(s,str) or not re.fullmatch('[a-z][a-z0-9-]{0,50}',s) for s in row['changed_services']):raise BridgeError('bridge_target_invalid')
    return row

def _save(path,row):
    raw=json.dumps(row,sort_keys=True,separators=(',',':')).encode()
    scratch=path.with_name(path.name+'.'+uuid.uuid4().hex+'.new')
    fd=os.open(scratch,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    try:
        with os.fdopen(fd,'wb') as stream:stream.write(raw);stream.flush();os.fsync(stream.fileno())
        os.replace(scratch,path)
        fd=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY)
        try:os.fsync(fd)
        finally:os.close(fd)
    finally:
        if scratch.exists():scratch.unlink()

def dispatch(directory,request,execute,recover,execute_observed=None,reporter=None,admit=None,validate_cached=None):
    """Never rerun an interrupted mutation unless checkpoint proves completion.

    Tokens are excluded from the durable journal. The request hash includes
    all noncredential fields, so refreshed short-lived credentials may retry.
    """
    validate(request);directory=Path(directory)
    if admit is not None:admit(request)
    import deployment_report
    def publish(**values):
        if reporter is not None:
            try:reporter(**values)
            except Exception:print('NATS_DEPLOYMENT_REPORT=UNAVAILABLE',file=sys.stderr)
    public={k:v for k,v in request.items() if k!='token'}
    wanted=hashlib.sha256(json.dumps(public,sort_keys=True,separators=(',',':')).encode()).hexdigest()
    path=directory/(request['nonce']+'.json')
    if path.exists():
        fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW)
        try:
            info=os.fstat(fd)
            if not stat.S_ISREG(info.st_mode) or info.st_uid!=os.geteuid() or info.st_mode&0o077 or info.st_nlink!=1 or info.st_size>65536:raise BridgeError('bridge_journal_untrusted')
            state=json.loads(os.read(fd,65537))
        finally:os.close(fd)
        if state['request_sha256']!=wanted:raise BridgeError('bridge_nonce_reused')
        if state['phase']=='COMPLETE':
            if validate_cached is not None:validate_cached(public,state['result'])
            return state['result']
        result=recover(public)
        if result is None:raise BridgeError('bridge_interrupted_requires_recovery')
    else:
        state={'schema':'voice-nats-bridge-request-v1','request_sha256':wanted,'request':public,'phase':'STARTED'}
        _save(path,state)
        publish(request_status='STARTED')
        def observe(name,status):
            if request['action'] not in ('prepare','prepare-rollback','prepare-repair') or name not in PREPARE_STAGES or status not in ('STARTED','COMPLETE'):
                raise BridgeError('bridge_prepare_stage_invalid')
            state['prepare_stage']={'name':name,'status':status}
            _save(path,state)
            publish(stage=name,last=name if status=='COMPLETE' else None,request_status='STARTED')
        try:
            deployment_report._observer=lambda checkpoint:publish(checkpoint=checkpoint,request_status='STARTED')
            deployment_report._failure_observer=lambda error:publish(error=error)
            import commands
            commands.diagnostic_observer=lambda action,status:publish(current_action=action,last=action if status=='COMPLETE' else None,request_status='STARTED')
            result=execute(request) if execute_observed is None else execute_observed(request,observe)
        except BaseException as error:
            if execute_observed is not None:
                state['prepare_error']=prepare_error(error);_save(path,state)
            publish(error=error)
            raise
        finally:
            deployment_report._observer=None
            deployment_report._failure_observer=None
            commands.diagnostic_observer=None
    if not isinstance(result,dict):raise BridgeError('bridge_result_invalid')
    state['phase']='COMPLETE';state['result']=result;_save(path,state)
    publish(request_status='COMPLETE')
    return result


def drain(installed):
    """Root claims at most eight bounded pmd requests under the global lock."""
    sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
    sys.path.insert(0,str(Path(__file__).resolve().parent))
    import guard
    from root_main import operation_lock
    from bridge_root import Actions
    from errors import safe_error
    installed=Path(installed)
    for directory in (installed,installed/'inbox',installed/'processing',installed/'journal',installed/'responses'):
        row=directory.lstat()
        if not stat.S_ISDIR(row.st_mode) or row.st_uid!=0 or row.st_mode&0o002 or directory.name!='inbox' and row.st_mode&0o020:raise BridgeError('bridge_directory_untrusted')
    with operation_lock():
        actions=Actions(installed/'code')
        inbox=installed/'inbox';names=sorted(inbox.iterdir())
        if len(names)>128:raise BridgeError('bridge_inbox_bound')
        pending=list((installed/'processing').iterdir())
        if len(pending)>8:raise BridgeError('bridge_processing_bound')
        for source in pending+names[:max(0,8-len(pending))]:
            if not re.fullmatch(r'[a-f0-9]{64}\.json',source.name):continue
            claimed=installed/'processing'/source.name
            if source.parent==inbox:
                if claimed.exists():continue
                os.rename(source,claimed)
            response=installed/'responses'/source.name
            try:
                fd=os.open(claimed,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
                try:
                    before=os.fstat(fd)
                    if not stat.S_ISREG(before.st_mode) or before.st_uid not in ((0,1000) if source.parent==installed/'processing' else (1000,)) or before.st_nlink!=1 or before.st_mode&0o077 or not 1<=before.st_size<=16384:raise BridgeError('bridge_request_custody_invalid')
                    raw=os.read(fd,16385);after=os.fstat(fd)
                    if (before.st_dev,before.st_ino,before.st_size,before.st_mtime_ns,before.st_ctime_ns)!=(after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns) or len(raw)!=before.st_size:raise BridgeError('bridge_request_changed')
                finally:os.close(fd)
                def pairs(values):
                    row={}
                    for key,value in values:
                        if key in row:raise BridgeError('bridge_duplicate_field')
                        row[key]=value
                    return row
                request=json.loads(raw,object_pairs_hook=pairs)
                if request.get('nonce')+'.json'!=source.name:raise BridgeError('bridge_filename_nonce_invalid')
                # Replace the runner inode with a root-created copy. A runner
                # descriptor opened before claim cannot alter the replay input.
                _save(claimed,request)
                import deployment_report
                import grp
                latest={}
                def reporter(**values):
                    checkpoint=values.pop('checkpoint',None)
                    if checkpoint is not None:
                        latest['checkpoint']=checkpoint
                        latest.pop('stage',None)
                    latest.update({k:v for k,v in values.items() if v is not None})
                    snapshot=latest.get('checkpoint',{})
                    operation=request['nonce'][:12] if request['action'] in ('prepare','prepare-rollback','prepare-repair') else request.get('operation')
                    try:
                        from bridge_root import private_json
                        snapshot=private_json(guard.ROOT/('rollout-'+operation)/'checkpoint.json')
                    except (FileNotFoundError,PermissionError):pass
                    binding=getattr(actions,'binding',None)
                    digest=hashlib.sha256(json.dumps(binding,sort_keys=True,separators=(',',':')).encode()).hexdigest() if binding is not None else None
                    row=deployment_report.build(request,snapshot,binding=digest,**{k:v for k,v in latest.items() if k!='checkpoint'})
                    deployment_report.persist(installed/'responses'/(request['nonce']+'.report.json'),row,grp.getgrnam('pmd').gr_gid)
                # Admission covers cached COMPLETE and recover as well as execute.
                scope=nullcontext(None) if request['action']=='status' else actions.resource_scope(request)
                with scope as resource:
                    result=dispatch(installed/'journal',request,actions.execute,actions.recover,
                        execute_observed=actions.execute_observed,reporter=reporter,
                        admit=None if resource is None else resource.admit_request, validate_cached=actions.validate_cached)
                    if resource is not None:
                        import resource_claim
                        resource_claim.finish_request(resource,actions,request,result)
            except Exception as error:result={'status':'BLOCKED','error':safe_error(error)}
            _save(response,result)
            import grp
            os.chown(response,0,grp.getgrnam('pmd').gr_gid);response.chmod(0o440)
            claimed.unlink()


if __name__=='__main__':
    if os.geteuid()!=0 or sys.platform!='linux':raise SystemExit('root Linux required')
    if len(sys.argv)!=1:raise SystemExit('fixed installed bridge only')
    drain(Path('/var/lib/voice-nats-preservation/installed'))
