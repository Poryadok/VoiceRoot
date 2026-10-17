"""Unprivileged fixed inbox client; credentials only enter a private request."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import time
import tempfile
sys.path.insert(0,str(Path(__file__).resolve().parent))
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
import bridge
import deployment_report
from errors import safe_error

INSTALLED=Path('/var/lib/voice-nats-preservation/installed')

def request(args,environment):
    action=args[0] if args else ''
    parent=None
    if action=='prepare-repair' and len(args) in (6,7):
        parent=args[1];args=['prepare',*args[2:]];action='prepare'
    if action=='prepare' and len(args) in (5,6,7):
        row={'action':action,'mode':args[1],'changed_services':[s for s in args[2].split(',') if s],'source_sha':args[3],'run_id':int(args[4]),'token':environment['GITHUB_TOKEN']}
        if len(args)>=6:
            if args[5] not in ('true','false'):raise bridge.BridgeError('bridge_client_arguments_invalid')
            if args[5]=='true':row['provisional_staging']=True
        if len(args)==7:
            if args[6] not in ('true','false'):raise bridge.BridgeError('bridge_client_arguments_invalid')
            if args[6]=='true':row['gis_first_install']=True
        if parent is not None:row.update(action='prepare-repair',operation=parent)
    elif action in ('prepare-rollback','resume-cold-backup','resume-cipher-upload','prepare-expired-native','prepare-expired-finish','accept-provisional') and len(args)==3:
        row={'action':action,'operation':args[1],'run_id':int(args[2]),'token':environment['GITHUB_TOKEN']}
    elif action=='authorize' and len(args)==3:
        row={'action':action,'operation':args[1],'artifact_id':int(args[2]),'token':environment['GITHUB_TOKEN']}
    elif action=='authorize-preserved-upload' and len(args)==5:
        row={'action':action,'operation':args[1],'artifact_id':int(args[2]),'upload_nonce':args[3],
            'run_id':int(args[4]),'token':environment['GITHUB_TOKEN']}
    elif action=='finish' and len(args)==3:row={'action':action,'operation':args[1],'claim_rv':args[2]}
    elif action in ('authorize-expired-native','authorize-expired-finish') and len(args)==6:
        row={'action':action,'operation':args[1],'original_artifact_id':int(args[2]),
            'observation_artifact_id':int(args[3]),'upload_nonce':args[4],'run_id':int(args[5]),'token':environment['GITHUB_TOKEN']}
    elif action=='finish-expired-native' and len(args)==4:
        row={'action':action,'operation':args[1],'claim_rv':args[2],'run_id':int(args[3]),'token':environment['GITHUB_TOKEN']}
    elif action=='status' and len(args)==2:row={'action':action,'operation':args[1]}
    else:raise bridge.BridgeError('bridge_client_arguments_invalid')
    row.update(run_attempt=int(environment['GITHUB_RUN_ATTEMPT']),report_run_id=int(environment['GITHUB_RUN_ID']),report_source_sha=environment['GITHUB_SHA'])
    # Same job/attempt/action payload always resumes the same request. Status
    # is observational, so each poll has a distinct nonce and fresh result.
    public={k:v for k,v in row.items() if k!='token'}
    identity=[environment['GITHUB_RUN_ID'],environment['GITHUB_RUN_ATTEMPT'],environment.get('GITHUB_JOB',''),public]
    if action=='status':identity.append(time.time_ns())
    row['nonce']=hashlib.sha256(json.dumps(identity,sort_keys=True).encode()).hexdigest()
    if 'run_id' in row and row['run_id']!=int(environment['GITHUB_RUN_ID']):raise bridge.BridgeError('bridge_workflow_run_changed')
    if action=='prepare' and row['source_sha']!=environment['GITHUB_SHA']:raise bridge.BridgeError('bridge_workflow_source_changed')
    return bridge.validate(row)

def submit(row,installed=INSTALLED,timeout=2400):
    installed=Path(installed)
    for path in (installed,installed/'inbox',installed/'responses'):
        info=path.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o002:raise bridge.BridgeError('bridge_client_directory_invalid')
    path=installed/'inbox'/(row['nonce']+'.json')
    response=installed/'responses'/path.name
    if not response.exists() and not path.exists():
        scratch=path.with_suffix('.pending')
        fd=os.open(scratch,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
        try:
            with os.fdopen(fd,'w') as stream:json.dump(row,stream);stream.flush();os.fsync(stream.fileno())
            os.rename(scratch,path)
        finally:
            if scratch.exists():scratch.unlink()
    deadline=time.monotonic()+timeout
    while time.monotonic()<deadline:
        try:fd=os.open(response,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
        except FileNotFoundError:time.sleep(1);continue
        try:
            info=os.fstat(fd)
            if not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_mode&0o022 or info.st_nlink!=1 or not 1<=info.st_size<=65536:raise bridge.BridgeError('bridge_client_response_invalid')
            result=json.loads(os.read(fd,65537))
        finally:os.close(fd)
        if result.get('status')=='BLOCKED':
            error=bridge.BridgeError('bridge_root_request_blocked')
            error.safe_result={'status':'BLOCKED','error':deployment_report.code(result.get('error'))}
            raise error
        return result
    raise bridge.BridgeError('bridge_response_timeout_operation_requires_status')

def output_directory(environment):
    base=Path(environment['RUNNER_TEMP'])
    info=base.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid!=os.geteuid() or info.st_mode&0o022:raise ValueError('report_output_directory_invalid')
    return Path(tempfile.mkdtemp(prefix='voice-deployment-reports-',dir=base))

BOUNDARY_ACTIONS=frozenset(bridge.FIELDS)|frozenset(('preflight','apply','configure','unknown'))
BOUNDARY_FIELDS={'schema','boundary','action','error_code','run_id','run_attempt','source_sha',
    'operation','operation_status','recovery_authorized','updated_at'}

def failure_index(environment):
    # Runner-owned observations never masquerade as a root response/nonce.
    run=environment['GITHUB_RUN_ID'];attempt=environment['GITHUB_RUN_ATTEMPT']
    if not re.fullmatch('[1-9][0-9]{0,19}',run) or not re.fullmatch('[1-9][0-9]{0,9}',attempt):raise ValueError('report_identity_invalid')
    base=Path(environment['RUNNER_TEMP']);info=base.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid!=os.geteuid() or info.st_mode&0o022:raise ValueError('report_output_directory_invalid')
    return base/('voice-deployment-'+run+'-'+attempt+'.failures')

def validate_failure(row,environment):
    if (not isinstance(row,dict) or set(row)!=BOUNDARY_FIELDS
        or row['schema']!='voice-deployment-boundary-failure-v1'
        or row['boundary'] not in ('workflow','bridge-client') or row['action'] not in BOUNDARY_ACTIONS
        or row['operation_status']!='UNKNOWN' or row['recovery_authorized'] is not False
        or type(row['run_id']) is not int or row['run_id']!=int(environment['GITHUB_RUN_ID'])
        or type(row['run_attempt']) is not int or row['run_attempt']!=int(environment['GITHUB_RUN_ATTEMPT'])
        or row['source_sha']!=environment['GITHUB_SHA'] or not re.fullmatch('[a-f0-9]{40}',row['source_sha'])
        or type(row['updated_at']) is not int or not 0<row['updated_at']<=time.time()+60
        or row['operation'] is not None and (not isinstance(row['operation'],str) or not re.fullmatch('[a-f0-9]{12}',row['operation']))
        or deployment_report.code(row['error_code'])!=row['error_code']):raise ValueError('boundary_report_invalid')
    return row

def failure_file(fd):
    info=os.fstat(fd)
    if not stat.S_ISREG(info.st_mode) or info.st_uid!=os.geteuid() or info.st_mode&0o077 or info.st_nlink!=1 or info.st_size>65536:raise ValueError('report_index_invalid')
    return info

def record_failure(error,boundary,action,environment):
    row={'schema':'voice-deployment-boundary-failure-v1','boundary':boundary,
        'action':action if action in BOUNDARY_ACTIONS else 'unknown','error_code':deployment_report.code(safe_error(error)),
        'run_id':int(environment['GITHUB_RUN_ID']),'run_attempt':int(environment['GITHUB_RUN_ATTEMPT']),
        'source_sha':environment['GITHUB_SHA'],'operation':None,'operation_status':'UNKNOWN',
        'recovery_authorized':False,'updated_at':int(time.time())}
    operation=environment.get('VOICE_NATS_ROLLOUT_OPERATION','')
    if re.fullmatch('[a-f0-9]{12}',operation):row['operation']=operation
    validate_failure(row,environment)
    raw=(json.dumps(row,sort_keys=True)+'\n').encode()
    fd=os.open(failure_index(environment),os.O_WRONLY|os.O_CREAT|os.O_APPEND|os.O_NOFOLLOW|os.O_NONBLOCK,0o600)
    try:
        if failure_file(fd).st_size+len(raw)>65536:raise ValueError('report_index_invalid')
        if os.write(fd,raw)!=len(raw):raise OSError('report_write_incomplete')
        os.fsync(fd)
    finally:os.close(fd)
    if environment.get('GITHUB_STEP_SUMMARY'):
        with open(environment['GITHUB_STEP_SUMMARY'],'a') as output:
            output.write('Deployment boundary failure: '+boundary+'/'+row['action']+' error='+row['error_code']+'; operation state UNKNOWN.\n\n')

def emit_failure(error,boundary,action,environment):
    # Emit the safe reason even when local evidence cannot be persisted.
    print('NATS_DEPLOYMENT_FAILURE boundary='+boundary+' error='+safe_error(error),file=sys.stderr)
    try:record_failure(error,boundary,action,environment)
    except Exception:print('NATS_DEPLOYMENT_REPORT=UNAVAILABLE',file=sys.stderr)

def collect(nonce,environment,installed=INSTALLED,*,target=None):
    if not re.fullmatch('[a-f0-9]{64}',nonce):raise ValueError('report_nonce_invalid')
    row=deployment_report.read(Path(installed)/'responses'/(nonce+'.report.json'),nonce=nonce,
        run_id=int(environment['GITHUB_RUN_ID']),run_attempt=int(environment['GITHUB_RUN_ATTEMPT']),source_sha=environment['GITHUB_SHA'])
    target=output_directory(environment) if target is None else Path(target)
    info=target.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid!=os.geteuid() or info.st_mode&0o077:raise ValueError('report_output_directory_invalid')
    destination=target/(nonce+'.deployment-report.json')
    # Never follow runner workspace links or retain a previous attempt's file.
    fd=os.open(destination,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    with os.fdopen(fd,'w') as output:json.dump(row,output,sort_keys=True)
    line='NATS_DEPLOYMENT_REPORT request='+row['request_status']+' stage='+row['stage']+' error='+row['error_code']+' operation='+str(row['operation'])+' state='+row['operation_status']+' fence='+row['effects']['fence']+' updated_at='+str(row['updated_at'])
    print(line)
    if environment.get('GITHUB_STEP_SUMMARY'):
        with open(environment['GITHUB_STEP_SUMMARY'],'a') as output:output.write(line+'\n\n')
    return row

def collect_all(environment):
    index=Path(environment['RUNNER_TEMP'])/('voice-deployment-'+environment['GITHUB_RUN_ID']+'-'+environment['GITHUB_RUN_ATTEMPT']+'.requests')
    nonces=[]
    if index.exists() or index.is_symlink():
        if index.is_symlink() or index.stat().st_size>8192:raise ValueError('report_index_invalid')
        nonces=index.read_text().splitlines()
        if not 1<=len(nonces)<=64:raise ValueError('report_index_invalid')
    failed=False
    target=output_directory(environment)
    failures=failure_index(environment)
    if failures.exists() or failures.is_symlink():
        fd=os.open(failures,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
        try:
            info=failure_file(fd);raw=os.read(fd,65537)
            if len(raw)!=info.st_size:raise ValueError('report_index_invalid')
        finally:os.close(fd)
        rows=[validate_failure(json.loads(line),environment) for line in raw.splitlines()]
        if not 1<=len(rows)<=64:raise ValueError('report_index_invalid')
        for number,row in enumerate(rows):
            with open(target/('boundary-'+str(number)+'.deployment-report.json'),'x') as output:json.dump(row,output,sort_keys=True)
    elif not nonces:raise ValueError('report_index_missing')
    for nonce in dict.fromkeys(nonces):
        try:collect(nonce,environment,target=target)
        except Exception:
            failed=True;print('NATS_DEPLOYMENT_REPORT=UNAVAILABLE')
    if environment.get('GITHUB_OUTPUT'):
        with open(environment['GITHUB_OUTPUT'],'a') as output:output.write('report_directory='+str(target)+'\n')
    if failed:raise ValueError('report_collection_incomplete')

if __name__=='__main__':
    try:
        if sys.argv[1:]==['collect-reports']:
            collect_all(os.environ);sys.exit(0)
        if len(sys.argv)==3 and sys.argv[1]=='report':
            if not re.fullmatch('[a-f0-9]{64}',sys.argv[2]):raise ValueError('report_nonce_invalid')
            print(json.dumps(deployment_report.read(INSTALLED/'responses'/(sys.argv[2]+'.report.json'),nonce=sys.argv[2]),sort_keys=True));sys.exit(0)
        row=request(sys.argv[1:],os.environ)
        if os.environ.get('RUNNER_TEMP'):
            index=Path(os.environ['RUNNER_TEMP'])/('voice-deployment-'+os.environ['GITHUB_RUN_ID']+'-'+os.environ['GITHUB_RUN_ATTEMPT']+'.requests')
            fd=os.open(index,os.O_WRONLY|os.O_CREAT|os.O_APPEND|os.O_NOFOLLOW,0o600)
            with os.fdopen(fd,'w') as output:output.write(row['nonce']+'\n')
        try:result=submit(row)
        finally:
            try:collect(row['nonce'],os.environ)
            except Exception:print('NATS_DEPLOYMENT_REPORT=UNAVAILABLE',file=sys.stderr)
        if os.environ.get('GITHUB_OUTPUT'):
            with open(os.environ['GITHUB_OUTPUT'],'a') as output:
                for key in ('operation','cipher_path','artifact_name','challenge','authorization','changed_services','deploy_mode','source_sha','upload_nonce',
                    'original_cipher_path','observation_cipher_path','original_artifact_name','observation_artifact_name'):
                    if key in result:
                        value=result[key]
                        if not isinstance(value,str) or '\n' in value or '\r' in value:raise bridge.BridgeError('bridge_output_invalid')
                        output.write(key+'='+value+'\n')
        print(json.dumps(result,sort_keys=True))
    except Exception as error:
        if hasattr(error,'safe_result'):print(json.dumps(error.safe_result,sort_keys=True))
        emit_failure(error,'bridge-client',sys.argv[1] if len(sys.argv)>1 else 'unknown',os.environ)
        print('NATS_ROLLOUT_BRIDGE=BLOCKED',file=sys.stderr);sys.exit(1)
