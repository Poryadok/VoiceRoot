"""Unprivileged fixed inbox client; credentials only enter a private request."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import time
sys.path.insert(0,str(Path(__file__).resolve().parent))
import bridge

INSTALLED=Path('/var/lib/voice-nats-preservation/installed')

def request(args,environment):
    action=args[0] if args else ''
    if action=='prepare' and len(args)==5:
        row={'action':action,'mode':args[1],'changed_services':[s for s in args[2].split(',') if s],'source_sha':args[3],'run_id':int(args[4]),'token':environment['GITHUB_TOKEN']}
    elif action in ('prepare-rollback','resume-cold-backup','resume-cipher-upload','prepare-expired-native','prepare-expired-finish') and len(args)==3:
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
        if result.get('status')=='BLOCKED':raise bridge.BridgeError('bridge_root_request_blocked')
        return result
    raise bridge.BridgeError('bridge_response_timeout_operation_requires_status')

if __name__=='__main__':
    try:
        result=submit(request(sys.argv[1:],os.environ))
        if os.environ.get('GITHUB_OUTPUT'):
            with open(os.environ['GITHUB_OUTPUT'],'a') as output:
                for key in ('operation','cipher_path','artifact_name','challenge','authorization','changed_services','deploy_mode','source_sha','upload_nonce',
                    'original_cipher_path','observation_cipher_path','original_artifact_name','observation_artifact_name'):
                    if key in result:
                        value=result[key]
                        if not isinstance(value,str) or '\n' in value or '\r' in value:raise bridge.BridgeError('bridge_output_invalid')
                        output.write(key+'='+value+'\n')
        print(json.dumps(result,sort_keys=True))
    except Exception:print('NATS_ROLLOUT_BRIDGE=BLOCKED',file=sys.stderr);sys.exit(1)
