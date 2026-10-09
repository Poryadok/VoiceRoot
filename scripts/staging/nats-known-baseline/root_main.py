"""Captured, reviewed human-root entry point. Agent never runs it on staging."""
import base64
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import uuid
import fcntl
from contextlib import contextmanager

# Launcher captures and verifies all adjacent modules before this isolated
# interpreter starts. The directory is root-owned and never writable by pmd.
sys.path.insert(0,str(Path(__file__).parent))
from controller import Blocked, file_sha, archive_closed_store, verify_archive
from docker_runtime import DockerRuntime
from scenario import fixture, staging_baseline, recover_staging_baseline
from stage_runtime import Kube, Staging, HUB, MARKER, LEAVES, pv_storage_path

ROOT=Path('/var/lib/voice-nats-preservation')
CURRENT={'namespace_uid':'ac7abaf3-241f-483f-ac33-71086e4dfd32',
         'marker_uid':'cbafb90c-6974-4642-86d6-09b2eef1c843',
         'generation':'r20260930a4',
         'hub_uid':'441a6e17-6a01-4de2-a7ea-4c00f17a4180',
         'source_claim':'voice-nats-jsdata-r20260930a4',
         'source_claim_uid':'5fae59b0-aea2-405a-a0f7-287e94a9b6a9',
         'source_pv_uid':'4a11a528-81f9-4404-9757-f4f32c411310'}


def operator_error(error):
    # Never disclose an exception message, command output or private value.
    public={'pv_storage_identity_invalid','pv_storage_identity_changed','pv_storage_kind_unsupported',
        'pv_storage_path_unsupported','pv_storage_node_unsupported','fence_continuation_source_changed',
        'fence_continuation_ttl_invalid','mounted_credential_input_changed','bootstrap_input_changed',
        'maintenance_ownership_changed','fenced_workload_changed','fence_projection_invalid',
        'staging_fence_timeout','command_failed','command_output_limit','root_operation_phase_failed',
        'baseline_recovery_final_identity_invalid','baseline_recovery_archive_already_present',
        'existing_store_identity_invalid','existing_store_custody_invalid','operation_container_writer_present',
        'recovery_baseline_census_changed','closed_final_store_changed','kernel_failed','bootstrap_failed'}
    if isinstance(error,Blocked):return str(error) if str(error) in public else 'blocked_unclassified'
    types={'TypeError','KeyError','NameError','OSError','ValueError','TimeoutError','FileNotFoundError','FileExistsError','PermissionError'}
    return 'exception_'+type(error).__name__ if type(error).__name__ in types else 'exception_unclassified'

@contextmanager
def operation_lock():
    fd=os.open(ROOT/'known-baseline.lock',os.O_RDWR|os.O_CREAT|os.O_NOFOLLOW|os.O_NONBLOCK,0o600)
    try:
        s=os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid!=0 or s.st_mode&0o077 or s.st_nlink!=1:
            raise Blocked('operation_lock_custody_invalid')
        try:fcntl.flock(fd,fcntl.LOCK_EX|fcntl.LOCK_NB)
        except BlockingIOError:raise Blocked('operation_already_running') from None
        yield
    finally:os.close(fd)

def bind_code(code,state):
    if json.loads(public_read(code/'capture-manifest.json'))!=state['code_capture']:
        raise Blocked('checkpoint_code_capture_changed')

def verify_store(base,state,staging):
    staging.verify_final_storage()
    expected=json.loads(public_read(Path(state['staging_backup']['manifest']),2<<20))
    temporary=base/('store-verification-'+uuid.uuid4().hex+'.tar')
    try:
        current=archive_closed_store(staging.final_path,temporary)
        if current!=expected:raise Blocked('closed_final_store_changed')
    finally:
        if temporary.exists():temporary.unlink()

def observe_refence(state,staging):
    state['stage_fenced']=False;state['fence_status']='UNKNOWN'
    if staging is None or staging.marker is None or staging.marker.get('data',{}).get('knownBaselineOperation')!=staging.operation:return
    try:state['refence']=staging.refence()
    except Exception:state['refence']={'verified':False,'error':'ownership_or_fence_failed'}
    if state['refence'].get('verified') is True:
        state['stage_fenced']=True;state['fence_status']='VERIFIED'


def private_json(path,*,limit=2<<20):
    if type(limit) is not int or not 0<limit<=128<<20:raise Blocked('checkpoint_limit_invalid')
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        s=os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid!=0 or s.st_mode&0o077 or not 1<=s.st_size<=limit:
            raise Blocked('checkpoint_custody_invalid')
        raw=os.read(fd,s.st_size+1)
        if len(raw)!=s.st_size: raise Blocked('checkpoint_size_changed')
        return json.loads(raw)
    finally: os.close(fd)


def save(path, row,*,limit=2<<20):
    if type(limit) is not int or not 0<limit<=128<<20:raise Blocked('journal_limit_invalid')
    raw=json.dumps(row,sort_keys=True,indent=2).encode()+b'\n'
    if len(raw)>limit: raise Blocked('journal_size_limit')
    temporary=path.with_name(path.name+'.next')
    fd=os.open(temporary,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    with os.fdopen(fd,'wb') as f:
        f.write(raw); f.flush(); os.fsync(f.fileno())
    os.replace(temporary,path)
    directory=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try: os.fsync(directory)
    finally: os.close(directory)


def public_read(path,limit=256<<10):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        s=os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or not 1<=s.st_size<=limit: raise Blocked('captured_input_invalid')
        raw=os.read(fd,s.st_size+1)
        if len(raw)!=s.st_size: raise Blocked('captured_input_changed')
        return raw
    finally: os.close(fd)


def capture_inputs(kube, base, contract, generation=None, bootstrap_enrollment=None):
    generation=CURRENT['generation'] if generation is None else generation
    if not re.fullmatch(r'r[0-9]{8}[a-z0-9]{0,8}',generation):
        raise Blocked('captured_generation_invalid')
    selected={'operator':'voice-nats-operator-'+generation,
              'bootstrap':'voice-nats-bootstrap-credentials-'+generation,
              'services':'voice-nats-service-credentials-'+generation}
    if bootstrap_enrollment is not None:
        from bootstrap_selection import select
        selected['bootstrap']=select(kube,bootstrap_enrollment,generation)
    secrets={role:kube.get('secret',name) for role,name in selected.items()}
    inputs=base/'inputs'; inputs.mkdir(mode=0o750); os.chown(inputs,0,65532); inputs.chmod(0o750)
    provenance={}
    tls='voice-nats-hub-tls-'+generation
    metadata=kube.secret_meta(tls)
    provenance['tls-metadata-only']={'resource':tls,'uid':metadata['uid'],'resource_version':metadata['resourceVersion']}
    def secret(role,key):
        row=secrets[role]
        raw=base64.b64decode(row['data'][key],validate=True)
        if not 1<=len(raw)<=256<<10: raise Blocked('secret_input_size_invalid')
        provenance[key]={'resource':selected[role],'uid':row['metadata']['uid'],
                         'resource_version':row['metadata']['resourceVersion'],
                         'sha256':hashlib.sha256(raw).hexdigest()}
        return raw
    for role in ('bootstrap','social','realtime'):
        raw=secret('bootstrap' if role=='bootstrap' else 'services',role+'.creds')
        path=inputs/(role+'.creds'); path.write_bytes(raw); path.chmod(0o440); os.chown(path,0,65532)
    tokens={key:secret('operator',key).decode().strip() for key in ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
    for key in ('account.public','system-account.public'):
        if not re.fullmatch(r'A[A-Z2-7]{55}',tokens[key]): raise Blocked('account_identity_invalid')
    path=inputs/'account.public';path.write_text(tokens['account.public']);path.chmod(0o440);os.chown(path,0,65532)
    for key in ('operator.jwt','account.jwt','system-account.jwt'):
        if not re.fullmatch(r'[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+',tokens[key]): raise Blocked('public_jwt_invalid')
    q=json.dumps
    config=('host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\nmax_control_line: 32768\n'
        'operator: '+q(tokens['operator.jwt'])+'\nsystem_account: '+q(tokens['system-account.public'])+
        '\nresolver: MEMORY\nresolver_preload: { '+q(tokens['account.public'])+': '+q(tokens['account.jwt'])+
        ', '+q(tokens['system-account.public'])+': '+q(tokens['system-account.jwt'])+' }\njetstream { store_dir: /data }\n')
    path=base/'server.conf'; path.write_text(config); path.chmod(0o440); os.chown(path,0,65532)
    for script in contract['scripts']:
        role=script['part']; name='voice-nats-'+role+'-bootstrap'
        row=kube.get('configmap',name); raw=row['data']['bootstrap.sh'].encode()
        if (row['metadata']['uid']!=script['configMapUID'] or
            row['metadata']['resourceVersion']!=script['configMapResourceVersion'] or
            hashlib.sha256(raw).hexdigest()!=script['sha256'] or len(raw)!=script['bytes']):
            raise Blocked('deployed_bootstrap_contract_changed')
        path=base/('bootstrap-'+role+'.sh'); path.write_bytes(raw); path.chmod(0o440); os.chown(path,0,65532)
    return provenance


def revalidate_inputs(kube, contract, provenance):
    for row in provenance.values():
        current=kube.secret_meta(row['resource'])
        if current['uid']!=row['uid'] or current['resourceVersion']!=row['resource_version']:
            raise Blocked('mounted_credential_input_changed')
    for script in contract['scripts']:
        current=kube.get('configmap','voice-nats-'+script['part']+'-bootstrap')
        if current['metadata']['uid']!=script['configMapUID'] or current['metadata']['resourceVersion']!=script['configMapResourceVersion'] or hashlib.sha256(current['data']['bootstrap.sh'].encode()).hexdigest()!=script['sha256']:
            raise Blocked('bootstrap_input_changed')


def prepare(code):
    operation=uuid.uuid4().hex[:12]
    base=ROOT/('known-baseline-'+operation); base.mkdir(mode=0o700)
    contract=json.loads(public_read(code/'deployed-contract.json'))
    kube=Kube(); state={'schema':'known-nats-root-checkpoint-v1','operation':operation,
        'status':'PREPARING','phase':'READ_ONLY_CAPTURE','stage_fenced':False,'fence_status':'NOT_ESTABLISHED',
        'created_at':dt.datetime.now(dt.timezone.utc).isoformat(),'journal':[]}
    state['code_capture']=json.loads(public_read(code/'capture-manifest.json'))
    staging=None
    def journal(row):
        if staging is not None:
            state['marker']=staging.marker;state['snapshots']=staging.snapshots
            if hasattr(staging,'service'):state['service']=staging.service
            for key in ('final_claim','final_pv'):
                if hasattr(staging,key):state[key]=getattr(staging,key)
            if hasattr(staging,'final_path'):state['final_path']=str(staging.final_path)
        state['journal'].append(row); save(base/'checkpoint.json',state)
    def phase(name): state['phase']=name; save(base/'checkpoint.json',state)
    runtime=DockerRuntime(base,operation)
    print('KNOWN_NATS_OPERATION='+str(base),flush=True)
    try:
        expected=dict(CURRENT)
        generation=CURRENT['generation']
        expected['secret_refs']={HUB:['voice-nats-operator-'+generation,'voice-nats-hub-tls-'+generation],
            **{'voice-'+s:['voice-nats-service-credentials-'+generation,'voice-nats-hub-tls-'+generation] for s in LEAVES}}
        expected['deployment_uids']={name:kube.get('deployment',name)['metadata']['uid'] for name in (HUB,'voice-gateway',*('voice-'+s for s in LEAVES))}
        if expected['deployment_uids'][HUB]!=CURRENT['hub_uid']: raise Blocked('hub_identity_changed')
        staging=Staging(kube,operation,expected,journal); staging.preflight()
        state['expected']=expected; state['provenance']=capture_inputs(kube,base,contract)
        binary=public_read(code/'kernel',32<<20)
        path=base/'kernel'; path.write_bytes(binary); path.chmod(0o555)
        phase('FIXTURE_BACKUP_RESTORE'); state['fixture_backup']=fixture(runtime)
        revalidate_inputs(kube,contract,state['provenance']); staging.preflight()
        phase('FENCE_STAGE')
        staging.fence()
        state['stage_fenced']=True;state['fence_status']='VERIFIED';save(base/'checkpoint.json',state)
        phase('NEW_FINAL_BASELINE')
        store=staging.new_claim(dt.datetime.now(dt.timezone.utc).strftime('%Y%m%d'))
        phase('STAGING_BACKUP_RESTORE'); state['staging_backup']=staging_baseline(runtime,store)
        staging.select_claim()
        state['marker']=staging.marker; state['snapshots']=staging.snapshots
        state['final_claim']=staging.final_claim; state['final_path']=str(staging.final_path)
        state['status']='AWAITING_OFF_NODE_COPY'
        state['expires_at']=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=4)).isoformat()
        phase('CUSTODY_CHECKPOINT')
        return base,state
    except Exception as error:
        observe_refence(state,staging)
        state['status']='BLOCKED'; state['error']=str(error) if isinstance(error,Blocked) and re.fullmatch(r'[a-z_]{1,80}',str(error)) else 'root_operation_phase_failed'
        save(base/'checkpoint.json',state)
        raise Blocked('root_operation_phase_failed') from None
    finally:
        for name in reversed(runtime.owned):
            runtime.inspect(name); runtime.run(['rm','-f',name])


def resume(code, base, hashes):
    if base.parent!=ROOT or not re.fullmatch(r'known-baseline-[a-f0-9]{12}',base.name): raise Blocked('checkpoint_path_invalid')
    state=private_json(base/'checkpoint.json')
    if state['status']!='AWAITING_OFF_NODE_COPY' or dt.datetime.now(dt.timezone.utc)>dt.datetime.fromisoformat(state['expires_at']):
        raise Blocked('checkpoint_state_or_ttl_invalid')
    expected=[state[k]['archive_sha256'] for k in ('fixture_backup','staging_backup')]
    if hashes!=expected or any(not re.fullmatch(r'[a-f0-9]{64}',h) for h in hashes): raise Blocked('off_node_receipt_hash_mismatch')
    bind_code(code,state)
    kube=Kube(); contract=json.loads(public_read(code/'deployed-contract.json'))
    revalidate_inputs(kube,contract,state['provenance'])
    for k in ('fixture_backup','staging_backup'):
        if file_sha(state[k]['archive'])!=state[k]['archive_sha256']: raise Blocked('checkpoint_archive_changed')
        state[k]['off_node_copy_verified']=True
        state[k]['off_node_custody']={'location':'C:/Users/Sergey/.codex/worktrees/a1-user-findings/Voice/.local/known-baseline-'+state['operation']+'/'+('fixture.tar' if k=='fixture_backup' else 'staging-baseline.tar'),
            'custodian':'Sergey; pmd read-only transfer; independently hash-verified by Codex',
            'receipt_authority':'human operator supplied both verified off-node SHA256 values',
            'accepted_at':dt.datetime.now(dt.timezone.utc).isoformat(),
            'retention':'retain both verified archives and manifests until explicit owner disposal'}
    def journal(row): state['journal'].append(row); save(base/'checkpoint.json',state)
    staging=Staging(kube,state['operation'],state['expected'],journal)
    staging.marker=state['marker']; staging.snapshots=state['snapshots']
    staging.final_claim=state['final_claim']; staging.final_path=Path(state['final_path'])
    staging.final_pv=state['final_pv'];staging.service=state['service']
    verify_store(base,state,staging)
    state['phase']='RESTART_STAGE'; save(base/'checkpoint.json',state)
    try:
        receipt=staging.restart()
        state['restart']=receipt; state['stage_fenced']=False;state['fence_status']='RELEASED'; state['status']='PASS'
        state['phase']='POST_RESTART'; save(base/'checkpoint.json',state)
        hub=kube.get('deployment',HUB)
        state['post_restart']={'observed_at':dt.datetime.now(dt.timezone.utc).isoformat(),
            'hub_uid':hub['metadata']['uid'],'ready_replicas':hub.get('status',{}).get('readyReplicas',0),
            'active_claim':state['final_claim']['metadata']['name'],
            'closed_backup_messages':0,'current_messages':'UNKNOWN; business clients resumed after closed capture cut',
            'scope':'backup protects closed bootstrap baseline; later records are outside this capture'}
        save(base/'checkpoint.json',state)
    except Exception:
        state['status']='BLOCKED'; state['error']='restart_failed'
        observe_refence(state,staging)
        save(base/'checkpoint.json',state)
        raise Blocked('restart_failed') from None
    return state


def refresh(code, base):
    if base.parent!=ROOT or not re.fullmatch(r'known-baseline-[a-f0-9]{12}',base.name): raise Blocked('checkpoint_path_invalid')
    state=private_json(base/'checkpoint.json')
    if state['status']!='AWAITING_OFF_NODE_COPY': raise Blocked('refresh_state_invalid')
    bind_code(code,state)
    kube=Kube(); contract=json.loads(public_read(code/'deployed-contract.json'))
    revalidate_inputs(kube,contract,state['provenance'])
    staging=Staging(kube,state['operation'],state['expected'],lambda row:None)
    staging.marker=state['marker']; staging.snapshots=state['snapshots']; staging.final_claim=state['final_claim']
    staging.final_pv=state['final_pv'];staging.final_path=Path(state['final_path']);staging.service=state['service']
    verify_store(base,state,staging)
    staging.owned_marker(); staging.no_pods(tuple(staging.snapshots))
    claim=kube.get('pvc',state['final_claim']['metadata']['name'])
    if claim['metadata']['uid']!=state['final_claim']['metadata']['uid']: raise Blocked('refresh_final_claim_changed')
    for name,snapshot in staging.snapshots.items():
        current=kube.get('deployment',name)
        if current['metadata']['uid']!=snapshot['metadata']['uid'] or current['spec']['template']!=snapshot['spec']['template'] or current['spec'].get('replicas',1)!=0:
            raise Blocked('refresh_workload_changed')
    for key in ('fixture_backup','staging_backup'):
        if file_sha(state[key]['archive'])!=state[key]['archive_sha256']: raise Blocked('refresh_archive_changed')
    state['expires_at']=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=4)).isoformat()
    state['journal'].append({'kind':'custody_ttl_refreshed','stage_fenced':True,'expires_at':state['expires_at']})
    save(base/'checkpoint.json',state)
    return state


def continuation_preflight(code, base, *, baseline=False):
    # Recovery is for the observed, already-fenced operation, not a generic
    # migration/override of arbitrary checkpoints or a fresh prepare retry.
    if base!=ROOT/'known-baseline-0049430b0dbb':raise Blocked('fence_continuation_path_invalid')
    for path,directory in ((base,True),(base/'inputs',True),(base/'server.conf',False),(base/'kernel',False)):
        s=path.lstat()
        if s.st_uid!=0 or s.st_mode&0o022 or not (stat.S_ISDIR(s.st_mode) if directory else stat.S_ISREG(s.st_mode)):
            raise Blocked('fence_continuation_custody_invalid')
    state=private_json(base/'checkpoint.json')
    if (state.get('schema')!='known-nats-root-checkpoint-v1' or state.get('operation')!='0049430b0dbb' or
        state.get('status')!='BLOCKED' or
        (not baseline and (state.get('phase')!='FENCE_STAGE' or state.get('error')!='command_output_limit' or any(k in state for k in ('final_claim','final_pv','staging_backup')))) or
        (baseline and (state.get('phase')!='STAGING_BACKUP_RESTORE' or state.get('error')!='root_operation_phase_failed' or state.get('fence_status')!='VERIFIED' or 'staging_backup' in state))):
        raise Blocked('fence_continuation_state_invalid')
    age=dt.datetime.now(dt.timezone.utc)-dt.datetime.fromisoformat(state['created_at'])
    # A separately reviewed post-allocation recovery renews custody only after
    # complete live revalidation; it never rewrites the original creation cut.
    if not dt.timedelta(minutes=-5)<=age<=dt.timedelta(hours=24 if baseline else 4):raise Blocked('fence_continuation_ttl_invalid')
    current=json.loads(public_read(code/'capture-manifest.json'))
    expected=dict(current['files'])
    if baseline:
        expected.update({'root_main.py':'28ee5551af7d4a2d9037977f1d6866cf64c682dcbe48932459724d65b507e12f',
                         'docker_runtime.py':'c5bd8a4adf22b3dabb7d8d2b7a1c2f88185974e2b532804fb8131cdd10d4e18b',
                         'scenario.py':'169a67428404cfb9021415bc4b6c9f9b2da39330a097562b92cf76acec7b75d9'})
    else:
        expected.update({'root_main.py':'5938a72653b973c41ebe0bd138fe4f15f1671ff6dc0895af18f0132c794f3cb0',
                         'stage_runtime.py':'0a79beea09cbfe78679054c4b7b661e823b7bea25b0970641d9ef14bed1fbfa3'})
    if state['code_capture'].get('schema')!='known-nats-code-capture-v1' or state['code_capture'].get('files')!=expected:
        raise Blocked('fence_continuation_code_changed')
    if file_sha(base/'kernel')!=current['files']['kernel']:raise Blocked('fence_continuation_kernel_changed')
    names=(HUB,'voice-gateway',*('voice-'+s for s in LEAVES))
    if (any(state['expected'].get(k)!=v for k,v in CURRENT.items()) or set(state['snapshots'])!=set(names) or
        set(state['expected']['deployment_uids'])!=set(names) or
        not any(r.get('kind')=='staging_preflight' and r.get('replicas')==dict.fromkeys(names,1) for r in state['journal'])):
        raise Blocked('fence_continuation_original_ledger_invalid')
    for name,row in state['snapshots'].items():
        if row['metadata']['uid']!=state['expected']['deployment_uids'][name] or row['spec'].get('replicas',1)!=0:
            raise Blocked('fence_continuation_snapshot_invalid')
    receipt=state['fixture_backup']
    if (any(receipt.get(k)!=v for k,v in {'messages':3,'streams':15,'consumers':42,'restore_verified':True,'node_copy_verified':True}.items()) or
        receipt.get('archive')!=str(base/'fixture.tar') or receipt.get('manifest')!=str(base/'fixture-manifest.json')):
        raise Blocked('fence_continuation_fixture_invalid')
    manifest=private_json(base/'fixture-manifest.json')
    if receipt['archive_sha256']!=manifest['archive_sha256'] or not verify_archive(base/'fixture.tar',manifest):
        raise Blocked('fence_continuation_fixture_changed')
    for role in ('bootstrap','social','realtime'):
        path=base/'inputs'/(role+'.creds');s=path.lstat()
        if not stat.S_ISREG(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022 or file_sha(path)!=state['provenance'][role+'.creds']['sha256']:
            raise Blocked('fence_continuation_private_input_changed')
    kube=Kube();contract=json.loads(public_read(code/'deployed-contract.json'))
    revalidate_inputs(kube,contract,state['provenance'])
    staging=None
    def journal(row):
        state['marker']=staging.marker;state['snapshots']=staging.snapshots
        for key in ('final_claim','final_pv'):
            if hasattr(staging,key):state[key]=getattr(staging,key)
        if hasattr(staging,'final_path'):state['final_path']=str(staging.final_path)
        state['journal'].append(row);save(base/'checkpoint.json',state)
    staging=Staging(kube,state['operation'],state['expected'],journal)
    staging.marker=state['marker'];staging.snapshots=state['snapshots'];staging.service=state['service']
    claim=kube.get('pvc',CURRENT['source_claim']);pv=kube.get('pv',claim['spec']['volumeName'])
    if claim['metadata']['uid']!=CURRENT['source_claim_uid']:
        raise Blocked('fence_continuation_source_changed')
    pv_storage_path(pv,CURRENT['source_claim_uid'],CURRENT['source_claim'],CURRENT['source_pv_uid'])
    staging.source_claim=claim
    if baseline:
        final_name='voice-nats-jsdata-d202610040049430b'
        final_uid='e319328f-1fb4-4a02-9c95-1dfa1997a668'
        final_pv_uid='01ef090e-a221-4aa4-9a6d-7f74304cf4c6'
        final_path='/var/lib/rancher/k3s/storage/pvc-'+final_uid+'_voice-staging_'+final_name
        final=state.get('final_claim',{});pv=state.get('final_pv',{})
        if (final.get('metadata',{}).get('uid')!=final_uid or final.get('metadata',{}).get('name')!=final_name or
            pv.get('metadata',{}).get('uid')!=final_pv_uid or state.get('final_path')!=final_path or
            not any(r.get('kind')=='final_claim_bound' and r.get('uid')==final_uid and r.get('pv_uid')==final_pv_uid and r.get('path')==final_path for r in state['journal'])):
            raise Blocked('baseline_recovery_final_identity_invalid')
        if any((base/n).exists() for n in ('staging-baseline.tar','staging-baseline-manifest.json','staging-baseline-node-copy.tar','staging-baseline-restored')):
            raise Blocked('baseline_recovery_archive_already_present')
        staging.final_claim=final;staging.final_pv=pv;staging.final_path=Path(final_path)
        staging.verify_final_storage()
    staging.verify_closed() # Read-only ownership/template/zero/mount checks.
    return state,current,staging


def continue_fence(code, base):
    state,current,staging=continuation_preflight(code,base)
    def journal(row):
        state['marker']=staging.marker;state['snapshots']=staging.snapshots
        for key in ('final_claim','final_pv'):
            if hasattr(staging,key):state[key]=getattr(staging,key)
        if hasattr(staging,'final_path'):state['final_path']=str(staging.final_path)
        state['journal'].append(row);save(base/'checkpoint.json',state)
    staging.save=journal
    state['previous_code_capture']=state['code_capture'];state['code_capture']=current
    state['status']='CONTINUING';state['stage_fenced']=True;state['fence_status']='VERIFIED'
    state['previous_error']=state.pop('error');journal({'kind':'owned_fence_continuation_verified'})
    runtime=DockerRuntime(base,state['operation'])
    def phase(name):state['phase']=name;save(base/'checkpoint.json',state)
    try:
        phase('NEW_FINAL_BASELINE')
        store=staging.new_claim(dt.datetime.now(dt.timezone.utc).strftime('%Y%m%d'))
        phase('STAGING_BACKUP_RESTORE');state['staging_backup']=staging_baseline(runtime,store)
        staging.select_claim()
        state['status']='AWAITING_OFF_NODE_COPY'
        state['expires_at']=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=4)).isoformat()
        phase('CUSTODY_CHECKPOINT')
        return base,state
    except Exception as error:
        observe_refence(state,staging);state['status']='BLOCKED'
        state['error']=str(error) if isinstance(error,Blocked) and re.fullmatch(r'[a-z_]{1,80}',str(error)) else 'root_operation_phase_failed'
        save(base/'checkpoint.json',state);raise Blocked('root_operation_phase_failed') from None
    finally:
        for name in reversed(runtime.owned):runtime.inspect(name);runtime.run(['rm','-f',name])


def continue_staging_baseline(code, base):
    state,current,staging=continuation_preflight(code,base,baseline=True)
    runtime=DockerRuntime(base,state['operation'])
    runtime.no_operation_containers() # Read-only, before any checkpoint rewrite.
    runtime.allow_existing_store(staging.final_path)
    original_created=state['created_at']
    state['previous_code_capture']=state['code_capture'];state['code_capture']=current
    state['previous_error']=state.pop('error');state['status']='RECOVERING_BASELINE'
    state['recovery_cut_at']=dt.datetime.now(dt.timezone.utc).isoformat()
    state['journal'].append({'kind':'baseline_recovery_custody_revalidated',
                             'original_created_at':original_created,'recovery_cut_at':state['recovery_cut_at']})
    save(base/'checkpoint.json',state)
    def journal(row):
        state['marker']=staging.marker;state['snapshots']=staging.snapshots
        state['journal'].append(row);save(base/'checkpoint.json',state)
    staging.save=journal
    try:
        # This is a newly observed paused-store cut, never claimed as an old
        # historical inventory. EXCL archives preserve all earlier proof files.
        archive=base/('recovery-before-'+uuid.uuid4().hex+'.tar')
        manifest=archive_closed_store(staging.final_path,archive)
        manifest_path=archive.with_suffix('.json');save(manifest_path,manifest)
        state['recovery_paused_store']={'archive':str(archive),'manifest':str(manifest_path),
                                      'archive_sha256':manifest['archive_sha256']}
        journal({'kind':'paused_store_cut_captured','archive_sha256':manifest['archive_sha256']})
        def before_start():
            runtime.no_operation_containers(running_only=True)
            revalidate_inputs(staging.kube,json.loads(public_read(code/'deployed-contract.json')),state['provenance'])
            verify_store(base,{'staging_backup':{'manifest':str(manifest_path)}},staging)
        state['staging_backup']=recover_staging_baseline(runtime,staging.final_path,
                                                        private_json(base/'fixture-manifest.json'),before_start)
        staging.select_claim()
        state['status']='AWAITING_OFF_NODE_COPY';state['phase']='CUSTODY_CHECKPOINT'
        state['stage_fenced']=True;state['fence_status']='VERIFIED'
        state['expires_at']=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=4)).isoformat()
        save(base/'checkpoint.json',state)
        return base,state
    except Exception as error:
        observe_refence(state,staging);state['status']='BLOCKED'
        state['error']=operator_error(error)
        save(base/'checkpoint.json',state)
        raise Blocked(state['error']) from None
    finally:
        for name in reversed(runtime.owned):runtime.inspect(name);runtime.run(['rm','-f',name])


def share_checkpoint(base,state):
    # Same existing sanitized off-node intake protocol; never shares inputs.
    gid=__import__('grp').getgrnam('pmd').gr_gid
    for key in ('fixture_backup','staging_backup'):
        for field in ('archive','manifest'):
            path=Path(state[key][field]);os.chown(path,0,gid);path.chmod(0o440)
    public={'schema':'known-nats-copy-checkpoint-v1','status':state['status'],
        'stage_fenced':state['stage_fenced'],'fence_status':state['fence_status'],'operation':state['operation'],'expires_at':state['expires_at'],
        'fixture_backup':{k:state['fixture_backup'][k] for k in ('archive','manifest','archive_sha256','messages','streams','consumers','restore_verified')},
        'staging_backup':{k:state['staging_backup'][k] for k in ('archive','manifest','archive_sha256','messages','streams','consumers','restore_verified')}}
    path=base/'copy-checkpoint.json';save(path,public);os.chown(path,0,gid);path.chmod(0o440)
    os.chown(base,0,gid);base.chmod(0o750);print(str(path))


def main_unlocked(args):
    if os.geteuid()!=0 or sys.platform!='linux': raise Blocked('human_root_linux_required')
    s=os.lstat(ROOT)
    if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022: raise Blocked('preservation_root_custody_invalid')
    code=Path(__file__).parent
    if args==['--prepare']:
        base,state=prepare(code)
        # Share only controlled archives and sanitized custody metadata. No
        # credentials, JWTs, rendered config, workload specs, or private journal.
        share_checkpoint(base,state);return
    if len(args)==2 and args[0]=='--continue-fence':
        base,state=continue_fence(code,Path(args[1]));share_checkpoint(base,state);return
    if len(args)==2 and args[0]=='--continue-staging-baseline':
        base,state=continue_staging_baseline(code,Path(args[1]));share_checkpoint(base,state);return
    if len(args)==4 and args[0]=='--resume':
        state=resume(code,Path(args[1]),args[2:])
        print('KNOWN_NATS_BASELINE='+state['status']); return
    if len(args)==2 and args[0]=='--refresh':
        state=refresh(code,Path(args[1])); print('KNOWN_NATS_CUSTODY_EXPIRES='+state['expires_at']);return
    if len(args)==2 and args[0]=='--status':
        base=Path(args[1])
        if base.parent!=ROOT or not re.fullmatch(r'known-baseline-[a-f0-9]{12}',base.name):raise Blocked('status_path_invalid')
        state=private_json(base/'checkpoint.json')
        print(json.dumps({k:state[k] for k in ('schema','operation','status','phase','stage_fenced','fence_status','error','expires_at','restart','post_restart') if k in state},sort_keys=True));return
    raise Blocked('root_arguments_invalid')

def main(args):
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('human_root_linux_required')
    s=os.lstat(ROOT)
    if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022:raise Blocked('preservation_root_custody_invalid')
    with operation_lock():main_unlocked(args)


if __name__=='__main__':
    try: main(sys.argv[1:])
    except Exception as error:
        print('KNOWN_NATS_BASELINE=BLOCKED',file=sys.stderr)
        print('KNOWN_NATS_ERROR='+operator_error(error),file=sys.stderr);sys.exit(1)
