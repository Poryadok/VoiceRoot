"""Finite diagnostic projection. No report grants mutation or recovery authority."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import time
import uuid

SCHEMA='voice-deployment-report-v1'
STAGES=frozenset(('UNKNOWN','request','source-capture','workload-capture','policy-validation',
    'image-selection','contract-selection','nats-preflight','target-build','READ_ONLY_CAPTURE',
    'FENCE','COLD_BACKUP','SPACE_BACKUP','AWAITING_OFF_NODE','DATABASE_MIGRATIONS',
    'NATS_CONTRACT_MIGRATION','RENDERER_TEMPLATE','PAUSED_APPLY','POST_APPLY_PROOF',
    'RESTART','POST_RESTART','ROLLBACK_TEMPLATES','ROLLBACK_PROOF','OPERATOR_RETIRING'))
STATUSES=frozenset(('UNKNOWN','RUNNING','WAITING','BLOCKED','PASS','ROLLED_BACK'))
EVENTS=frozenset(('rollout_maintenance_owned','original_hub_shutdown_intent',
    'original_hub_orderly_exit_verified','original_hub_owned_finalizer_removed','rollout_released','rollout_target_observed'))
EFFECTS=frozenset(('maintenance','fence','original_broker_exit','finalizer','cut','preservation','apply','restart','release'))
FIELDS=frozenset(('schema','request_nonce','request_action','request_status','run_id','run_attempt',
    'source_sha','operation','operation_status','operation_error','helper_binding','updated_at',
    'stage','current_action','last_completed_action','error_code','command','effects','recovery_authorized'))
ACTION_IDS=frozenset({'command'}|{tool+'.'+verb for tool in ('kubectl','docker','nats') for verb in ('get','patch','wait','scale','rollout','apply','create','delete','exec','run','inspect','stop','rm','image','info','other')})
COMMAND_FIELDS=frozenset(('exit_code','timeout','cause'))
CAUSES=frozenset(('unknown','os_permission_denied','application_authorization','timeout','nonzero_exit'))
_observer=None
_failure_observer=None

def code(value):
    # The existing closed error inventory is the sole guard-label authority.
    try:
        from errors import CODES,TYPES
        approved=CODES|{'exception_'+name for name in TYPES}
    except ImportError:approved=set()
    return value if isinstance(value,str) and value in approved|CAUSES|{'none','interrupted','unknown','source_authority_rejected','transport_rejected'} else 'unknown'

def error_code(error):
    if error is None:return 'none'
    if isinstance(error,PermissionError):return 'os_permission_denied'
    if isinstance(error,(KeyboardInterrupt,SystemExit)):return 'interrupted'
    if type(error).__name__=='SourceError':return 'source_authority_rejected'
    if type(error).__name__=='CustodyError':return 'transport_rejected'
    if type(error).__name__=='Blocked' and len(error.args)==1:return code(error.args[0])
    return code('exception_'+type(error).__name__)

def match(value,pattern):return value if isinstance(value,str) and re.fullmatch(pattern,value) else None
def number(value):return value if type(value) is int and 0<value<2**63 else None

def build(request,state,*,error=None,request_status='COMPLETE',stage=None,last=None,binding=None,current_action=None):
    state=state or {};events=state.get('events',[])
    kinds=[e.get('kind') for e in events if isinstance(e,dict) and e.get('kind') in EVENTS]
    def seen(kind):return 'RECORDED' if kind in kinds else 'UNKNOWN'
    status=state.get('status');phase=state.get('phase')
    released=status in ('PASS','ROLLED_BACK') and state.get('fence_status')=='RELEASED'
    execution=state.get('execution_authority') or {}
    command=getattr(error,'diagnostic',{})
    if isinstance(command,dict) and command.get('action') in ACTION_IDS:current_action=command['action']
    command={k:v for k,v in command.items() if k in COMMAND_FIELDS} if isinstance(command,dict) else {}
    command={'exit_code':command.get('exit_code') if type(command.get('exit_code')) is int and -255<=command['exit_code']<=255 else None,
        'timeout':command.get('timeout') is True,'cause':command.get('cause') if command.get('cause') in CAUSES else 'unknown'}
    return {'schema':SCHEMA,'request_nonce':match(request.get('nonce'),r'[a-f0-9]{64}'),
        'request_action':request.get('action') if request.get('action') in __import__('bridge').FIELDS else 'unknown',
        'request_status':'BLOCKED' if error is not None else request_status,
        'run_id':number(request.get('report_run_id',request.get('run_id',execution.get('dispatcher_run_id')))),
        'run_attempt':number(request.get('run_attempt',execution.get('run_attempt'))),
        'source_sha':match(request.get('report_source_sha',request.get('source_sha',execution.get('head_sha'))),r'[a-f0-9]{40}'),
        'operation':match(state.get('operation'),r'[a-f0-9]{12}'),
        'operation_status':status if status in STATUSES else 'UNKNOWN',
        'operation_error':code(state.get('error')),'helper_binding':match(binding,r'[a-f0-9]{64}'),
        'updated_at':int(time.time()),'stage':stage if stage in STAGES else phase if phase in STAGES else 'UNKNOWN',
        'current_action':current_action if current_action in ACTION_IDS else None,
        'last_completed_action':last if last in STAGES|EVENTS|ACTION_IDS else kinds[-1] if kinds else None,
        'error_code':error_code(error),'command':command,'recovery_authorized':False,
        'effects':{'maintenance':seen('rollout_maintenance_owned'),
            'fence':state.get('fence_status') if state.get('fence_status') in ('VERIFIED','RELEASED') else 'UNKNOWN',
            'original_broker_exit':seen('original_hub_orderly_exit_verified'),
            'finalizer':'REMOVED' if 'original_hub_owned_finalizer_removed' in kinds else 'UNKNOWN',
            'cut':'PRESENT' if isinstance(state.get('cut'),dict) else 'UNKNOWN',
            'preservation':'PRESENT' if isinstance(state.get('preservation'),dict) else 'UNKNOWN',
            'apply':seen('rollout_target_observed'),'restart':'PRESENT' if isinstance(state.get('restart'),dict) else 'UNKNOWN',
            'release':'RECORDED' if released else 'UNKNOWN'}}

def validate(row,*,nonce=None,source_sha=None,run_id=None,run_attempt=None):
    if not isinstance(row,dict) or set(row)!=FIELDS or row['schema']!=SCHEMA:raise ValueError('report_schema_invalid')
    if row['recovery_authorized'] is not False or row['request_status'] not in ('STARTED','COMPLETE','BLOCKED'):raise ValueError('report_status_invalid')
    if row['request_action'] not in set(__import__('bridge').FIELDS)|{'unknown'}:raise ValueError('report_action_invalid')
    for key,pattern in (('request_nonce',r'[a-f0-9]{64}'),('source_sha',r'[a-f0-9]{40}'),('operation',r'[a-f0-9]{12}'),('helper_binding',r'[a-f0-9]{64}')):
        if row[key] is not None and match(row[key],pattern) is None:raise ValueError('report_identity_invalid')
    for key in ('run_id','run_attempt','updated_at'):
        if row[key] is not None and number(row[key]) is None:raise ValueError('report_number_invalid')
    if row['updated_at'] is None or row['request_nonce'] is None:raise ValueError('report_identity_missing')
    for key,wanted in (('request_nonce',nonce),('source_sha',source_sha),('run_id',run_id),('run_attempt',run_attempt)):
        if wanted is not None and row[key]!=wanted:raise ValueError('report_identity_mismatch')
    if row['stage'] not in STAGES or row['operation_status'] not in STATUSES or row['last_completed_action'] not in STAGES|EVENTS|ACTION_IDS|{None} or row['current_action'] not in ACTION_IDS|{None}:raise ValueError('report_stage_invalid')
    if any(code(row[k])!=row[k] for k in ('error_code','operation_error')):raise ValueError('report_error_invalid')
    effects=row['effects']
    if not isinstance(effects,dict) or set(effects)!=EFFECTS or any(v not in ('UNKNOWN','VERIFIED','RELEASED','RECORDED','REMOVED','PRESENT') for v in effects.values()):raise ValueError('report_effects_invalid')
    command=row['command']
    if not isinstance(command,dict) or set(command)!=COMMAND_FIELDS or command['cause'] not in CAUSES or type(command['timeout']) is not bool:raise ValueError('report_command_invalid')
    if command['exit_code'] is not None and (type(command['exit_code']) is not int or not -255<=command['exit_code']<=255):raise ValueError('report_command_invalid')
    return row

def persist(path,row,gid):
    validate(row);path=Path(path);raw=json.dumps(row,sort_keys=True).encode()
    if len(raw)>8192:raise ValueError('report_size_invalid')
    parent=path.parent.lstat()
    if not stat.S_ISDIR(parent.st_mode) or parent.st_uid!=os.geteuid() or parent.st_mode&0o022:raise ValueError('report_directory_invalid')
    scratch=path.with_name(path.name+'.'+uuid.uuid4().hex+'.new')
    fd=os.open(scratch,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    try:
        with os.fdopen(fd,'wb') as stream:
            stream.write(raw);stream.flush();os.fchown(stream.fileno(),os.geteuid(),gid);os.fchmod(stream.fileno(),0o440);os.fsync(stream.fileno())
        os.replace(scratch,path)
        fd=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
        try:os.fsync(fd)
        finally:os.close(fd)
    finally:
        if scratch.exists():scratch.unlink()

def read(path,*,owner=0,now=None,max_age=None,trusted_root=None,**identity):
    path=Path(path)
    for parent in (path.parent,*path.parent.parents):
        info=parent.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid!=owner or info.st_mode&0o022:
            raise ValueError('report_directory_invalid')
        if trusted_root is not None and parent==Path(trusted_root):break
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        info=os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_uid!=owner or info.st_mode&0o022 or info.st_nlink!=1 or not 1<=info.st_size<=8192:raise ValueError('report_custody_invalid')
        raw=os.read(fd,8193)
    finally:os.close(fd)
    row=validate(json.loads(raw),**identity)
    age=(time.time() if now is None else now)-row['updated_at']
    if age< -60 or max_age is not None and age>max_age:raise ValueError('report_stale')
    return row

def checkpoint(state):
    if _observer is not None:_observer(state)

def capture_failure(error):
    """Freeze action before cleanup; retain original error independently."""
    import commands
    if not hasattr(error,'diagnostic'):
        error.diagnostic={'action':commands._current_action,'cause':'unknown','exit_code':None,'timeout':False}
    if _failure_observer is not None:_failure_observer(error)

def export_historical(installed,nonce,operation):
    """Root-only projection of an exact retained journal/checkpoint; no recovery."""
    import sys
    if os.geteuid()!=0 or sys.platform!='linux':raise ValueError('root_linux_required')
    if match(nonce,r'[a-f0-9]{64}') is None or match(operation,r'[a-f0-9]{12}') is None:raise ValueError('report_identity_invalid')
    from root_main import private_json,operation_lock
    from root_cli import operation_path,code_binding
    import grp
    installed=Path(installed)
    with operation_lock():
        journal=private_json(installed/'journal'/(nonce+'.json'))
        request=journal['request'];__import__('bridge').validate(dict(request,token='historical-placeholder') if 'token' in __import__('bridge').FIELDS.get(request.get('action'),set()) else request)
        expected=request['nonce'][:12] if request['action'] in ('prepare','prepare-rollback') else request.get('operation')
        if expected!=operation or request['nonce']!=nonce:raise ValueError('report_operation_mismatch')
        base=operation_path(str(installed.parent/('rollout-'+operation)))
        state=private_json(base/'checkpoint.json',limit=128<<20)
        if state.get('operation')!=operation:raise ValueError('report_operation_mismatch')
        binding=hashlib.sha256(json.dumps(code_binding(installed/'code'),sort_keys=True,separators=(',',':')).encode()).hexdigest()
        row=build(request,state,request_status='COMPLETE' if journal.get('phase')=='COMPLETE' else 'BLOCKED',binding=binding)
        # Historical journals do not establish the current exception.
        row['error_code']='unknown'
        persist(installed/'responses'/(nonce+'.report.json'),row,grp.getgrnam('pmd').gr_gid)
        return row

if __name__=='__main__':
    import sys
    sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
    if len(sys.argv)!=4 or sys.argv[1]!='--export-historical':raise SystemExit('usage: deployment_report.py --export-historical REQUEST_NONCE OPERATION')
    try:print(json.dumps(export_historical('/var/lib/voice-nats-preservation/installed',sys.argv[2],sys.argv[3]),sort_keys=True))
    except Exception:raise SystemExit('NATS_DEPLOYMENT_REPORT=UNAVAILABLE')
