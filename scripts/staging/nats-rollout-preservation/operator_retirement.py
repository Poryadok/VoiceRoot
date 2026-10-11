"""One specific human-root waiver closure; never a deployment or rollback PASS."""
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
sys.path.insert(0,str(Path(__file__).resolve().parent))
import guard
import root_cli
from controller import Blocked,READ,file_sha
from root_main import operation_lock,save
from stage_runtime import Staging,Kube,HUB,LEAVES,MARKER,pv_storage_path

OPERATION='3340764a7d24'
SNAPSHOT_SHA='c776c76492c6266eeaed8eae736224435814f68a76e2a57c892aa9de86925672'
V15_BINDING='10b5e8f3b5b8676bdc8d4e1f5432304933b3205e7a4a820e09730459ba923d2b'
FAILED_RETIREMENT_SHA='5a0e2c2437f3d6b96b57e5793d577d58d85ff0310996cf354dc658774ee5b7b6'
ORIGINAL_RETIREMENT_BINDING='2f90bfbc649b4492c3ca997f8cbaf31932a1033b883bdda06c40a68f658e5173'
CONFIGMAPS={'voice-app-config','voice-nats-analytics-chat-bootstrap','voice-nats-hub-config',
    'voice-nats-leaf-config','voice-nats-realtime-bootstrap'}
CORE={HUB,'voice-gateway',*('voice-'+s for s in LEAVES)}
WAIVER='CURRENT_STAGING_MESSAGES_AND_CONSUMERS_MAY_BE_LOST_SETUP_RETAINED'

def fail():raise Blocked('operator_retirement_binding_invalid')
def canonical(row):return hashlib.sha256(json.dumps(row,sort_keys=True,separators=(',',':')).encode()).hexdigest()

def private_bytes(path,limit=128<<20,*,group_read=False):
    path=Path(path);before=path.lstat();fd=os.open(path,READ)
    identity=lambda s:(s.st_dev,s.st_ino,s.st_mode,s.st_uid,s.st_gid,s.st_nlink,s.st_size,s.st_mtime_ns,s.st_ctime_ns)
    try:
        opened=os.fstat(fd)
        if (not stat.S_ISREG(opened.st_mode) or opened.st_uid!=0 or opened.st_mode&(0o037 if group_read else 0o077)
            or opened.st_nlink!=1 or not 0<opened.st_size<=limit or identity(before)!=identity(opened)):fail()
        chunks=[];size=0
        while raw:=os.read(fd,min(1<<20,limit-size+1)):
            chunks.append(raw);size+=len(raw)
            if size>limit:fail()
        if size!=opened.st_size or identity(opened)!=identity(os.fstat(fd)) or identity(opened)!=identity(path.lstat()):fail()
        return b''.join(chunks)
    finally:os.close(fd)

def document(path):return json.loads(private_bytes(path))
def exclusive(path,raw):
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    try:
        with os.fdopen(fd,'wb',closefd=False) as stream:
            stream.write(raw);stream.flush();os.fsync(fd)
    finally:os.close(fd)
    fd=os.open(Path(path).parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(fd)
    finally:os.close(fd)

def terminal_state(snapshot,events,receipt_sha):
    if (snapshot.get('operation')!=OPERATION or snapshot.get('phase')!='PAUSED_APPLY'
        or snapshot.get('status')!='WAITING' or snapshot.get('fence_status')!='VERIFIED'
        or len(snapshot.get('events',[]))!=83 or snapshot.get('nats_migration',{}).get('verified') is not True
        or not isinstance(events,list) or len(events)<2
        or events[0].get('kind')!='operator_retirement_intent'
        or events[-1].get('kind')!='operator_retirement_completed'
        or any(e.get('kind') not in {'operator_retirement_intent','operator_retirement_completed',
            'workload_scaled','user_cycle_override','user_cycle_restored','operator_retirement_continuation_intent'} for e in events)
        or not re.fullmatch('[a-f0-9]{64}',receipt_sha)):fail()
    result=copy.deepcopy(snapshot)
    result.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',
        events=copy.deepcopy(snapshot['events']+events),
        operator_retirement={'schema':'voice-operator-retirement-reference-v1',
            'snapshot_sha256':SNAPSHOT_SHA,'receipt_sha256':receipt_sha})
    return result

def validate_terminal(snapshot,state,events,receipt_sha):
    if state!=terminal_state(snapshot,events,receipt_sha):fail()

def journal_binding(installed,rows):
    if not isinstance(rows,dict) or len(rows)>100:fail()
    for name,wanted in rows.items():
        if not re.fullmatch('[a-f0-9]{64}\\.json',name) or set(wanted)!={'journal_sha256','request_sha256','response_sha256'}:fail()
        raw=private_bytes(installed/'journal'/name,2<<20);row=json.loads(raw)
        if hashlib.sha256(raw).hexdigest()!=wanted['journal_sha256'] or row.get('request_sha256')!=wanted['request_sha256']:fail()
        request=row.get('request',{})
        if canonical(request)!=wanted['request_sha256']:fail()
        if not (request.get('operation')==OPERATION or request.get('action')=='prepare' and request.get('nonce','')[:12]==OPERATION):fail()
        if hashlib.sha256(private_bytes(installed/'responses'/name,2<<20,group_read=True)).hexdigest()!=wanted['response_sha256']:fail()
    return rows

def verify_terminal(base,state):
    if 'operator_captured_abort' in state:return verify_captured_terminal(base,state)
    if 'operator_precut_abort' in state:return verify_precut_terminal(base,state)
    base=Path(base)
    if base.name!='rollout-'+OPERATION or state.get('operation')!=OPERATION:fail()
    raw=private_bytes(base/'operator-retirement-snapshot.json')
    if hashlib.sha256(raw).hexdigest()!=SNAPSHOT_SHA:fail()
    snapshot=json.loads(raw);receipt=document(base/'operator-retirement-receipt.json')
    reference=state.get('operator_retirement',{})
    if (reference.get('receipt_sha256')!=canonical(receipt)
        or receipt.get('schema')!='voice-operator-retirement-closure-v1'
        or receipt.get('operation')!=OPERATION or receipt.get('snapshot_sha256')!=SNAPSHOT_SHA
        or receipt.get('waiver')!=WAIVER or receipt.get('deployment_pass') is not False
        or receipt.get('rollback_eligible') is not False):fail()
    plan_raw=private_bytes(base/'operator-retirement-plan.json',2<<20);plan=json.loads(plan_raw)
    if (hashlib.sha256(plan_raw).hexdigest()!=receipt.get('plan_sha256')
        or set(plan)!={'schema','operation','checkpoint_sha256','code_binding_sha256','waiver','configmaps','journals'}
        or plan.get('schema')!='voice-specific-operator-retirement-plan-v1'
        or plan.get('waiver')!=WAIVER or plan.get('operation')!=OPERATION
        or plan.get('checkpoint_sha256')!=SNAPSHOT_SHA
        or plan.get('code_binding_sha256')!=receipt.get('code_binding_sha256')
        or plan.get('journals')!=receipt.get('journals') or set(plan.get('configmaps',{}))!=CONFIGMAPS):fail()
    validate_terminal(snapshot,state,receipt['events'],canonical(receipt))
    original=snapshot['context']['original_snapshots']
    continuation=receipt.get('continuation')
    if continuation is not None:
        verify_continuation(base,snapshot,plan_raw,receipt)
    elif any(e.get('kind')=='operator_retirement_continuation_intent' for e in receipt['events']):fail()
    setup=receipt.get('setup',{})
    if not isinstance(setup,dict) or set(setup)!=CORE:fail()
    expected={n:{'uid':r['metadata']['uid'],'template_sha256':canonical(r['spec']['template']),
        'replicas':1,'ready':setup[n].get('ready') if continuation is not None else True} for n,r in original.items()}
    if (set(original)!=CORE or setup!=expected or any(type(row.get('ready')) is not bool for row in setup.values())
        or setup[HUB]['ready'] is not True):fail()
    before=snapshot['context']['marker'];after=receipt.get('released_marker',{})
    wanted=copy.deepcopy(before['data']);wanted['phase']='active';wanted.pop('knownRolloutOperation',None)
    if (before['metadata']['resourceVersion']!='3154269' or before['data'].get('phase')!='rollout-prepared'
        or before['data'].get('knownRolloutOperation')!=OPERATION
        or after.get('uid')!=before['metadata']['uid'] or after.get('data')!=wanted
        or not isinstance(after.get('resourceVersion'),str) or after['resourceVersion']==before['metadata']['resourceVersion']
        or receipt.get('storage_verified') is not True or receipt.get('config_verified') is not True
        or receipt.get('user_cycle_restored') is not True):fail()
    journal_binding(guard.ROOT/'installed',receipt['journals'])
    return receipt

def precut_retained_setup(snapshot):
    ctx=snapshot['context'];expected=ctx['expected']
    return {'namespace_uid':expected['namespace_uid'],
        'selected_claim':{'name':expected['source_claim'],'uid':expected['source_claim_uid'],'spec_sha256':canonical(ctx['final_claim']['spec'])},
        'selected_pv':{'uid':expected['source_pv_uid'],'spec_sha256':canonical(ctx['final_pv']['spec']),'path':ctx['final_path']},
        'service':{'uid':ctx['service']['metadata']['uid'],'object_sha256':canonical(ctx['service'])}}


def precut_configmaps(original):
    names=set(CONFIGMAPS)
    for row in original.values():
        spec=row['spec']['template']['spec']
        names.update(v['configMap']['name'] for v in spec.get('volumes',[]) if 'configMap' in v)
        names.update(s['configMap']['name'] for v in spec.get('volumes',[])
            for s in v.get('projected',{}).get('sources',[]) if 'configMap' in s)
        for c in spec.get('initContainers',[])+spec.get('containers',[]):
            names.update(v['configMapRef']['name'] for v in c.get('envFrom',[]) if 'configMapRef' in v)
            names.update(v['valueFrom']['configMapKeyRef']['name'] for v in c.get('env',[]) if 'configMapKeyRef' in v.get('valueFrom',{}))
    return names


def verify_precut_terminal(base,state):
    """Historical root-attested abort before any cold cut or application effect."""
    return _verify_unapplied_terminal(base,state,captured=False)


def verify_captured_terminal(base,state):
    """Abort an unapplied captured rollout; retained backup is never release PASS."""
    return _verify_unapplied_terminal(base,state,captured=True)


def _verify_unapplied_terminal(base,state,*,captured):
    kind='captured' if captured else 'precut';prefix='operator-'+kind+'-abort'
    reference_key='operator_'+kind+'_abort' 
    base=Path(base);operation=state.get('operation','')
    if not re.fullmatch('[a-f0-9]{12}',operation) or base.name!='rollout-'+operation:fail()
    raw=private_bytes(base/(prefix+'-snapshot.json'));snapshot=json.loads(raw)
    sha=hashlib.sha256(raw).hexdigest();receipt=document(base/(prefix+'-receipt.json'))
    reference={'schema':'voice-'+prefix+'-reference-v1','snapshot_sha256':sha,'receipt_sha256':canonical(receipt)}
    forbidden={'authorization','nats_migration','migration_jobs','migration_completed_metadata','space_postcondition',
        'renderer_applied','apply','restart','preservation','closed_preservation','closed_preservation_sealed',
        'operator_retirement','operator_precut_abort','operator_captured_abort'}
    if not captured:forbidden.update({'cut','custody','space_gate','space_backup'})
    absent=('authorization.json','apply-authorization.json') if captured else (
        'rollout-before.tar','rollout-before-manifest.json','authorization.json','apply-authorization.json','copy-checkpoint.json')
    if (state.get(reference_key)!=reference or snapshot.get('schema')!='nats-rollout-root-v1'
        or snapshot.get('operation')!=operation or snapshot.get('phase')!=('AWAITING_OFF_NODE' if captured else 'FENCE')
        or snapshot.get('status')!=('WAITING' if captured else 'BLOCKED') or snapshot.get('fence_status')!='VERIFIED'
        or any(k in snapshot for k in forbidden) or any((base/n).exists() for n in absent)
        or receipt.get('schema')!='voice-'+prefix+'-closure-v1' or receipt.get('operation')!=operation
        or receipt.get('snapshot_sha256')!=sha or receipt.get('waiver')!=WAIVER
        or receipt.get('deployment_pass') is not False or receipt.get('rollback_eligible') is not False):fail()
    binding=root_cli.code_binding(base/'code')
    if binding!=snapshot.get('code_capture') or canonical(binding)!=receipt.get('code_binding_sha256'):fail()
    if captured:verify_captured_evidence(base,snapshot,receipt)
    history=snapshot.get('events');events=receipt.get('events')
    allowed={'staging_preflight','rollout_original_capture','rollout_maintenance_owned','workload_scaled'}
    if captured:allowed.update({'original_hub_shutdown_intent','original_hub_orderly_exit_verified','original_hub_owned_finalizer_removed'})
    if (not isinstance(history,list) or not history or any(e.get('kind') not in allowed for e in history)
        or not isinstance(events,list) or len(events)<2 or events[0].get('kind')!='operator_'+kind+'_abort_intent'
        or events[-1].get('kind')!='operator_'+kind+'_abort_completed'):fail()
    original=snapshot['context']['original_snapshots'];names=CORE|{'voice-web','voice-admin','voice-developer-portal'}
    setup=receipt.get('setup',{})
    if set(original)!=names or not isinstance(setup,dict) or set(setup)!=names:fail()
    captured_snapshots=snapshot['context']['snapshots']
    if (set(captured_snapshots)!=names or any(r['spec'].get('replicas',1)!=1
        or captured_snapshots[n]['spec'].get('replicas',1)!=0 or captured_snapshots[n]['metadata']['uid']!=r['metadata']['uid']
        or captured_snapshots[n]['spec']['template']!=r['spec']['template'] for n,r in original.items())
        or any(e.get('kind')=='workload_scaled' and (e.get('name') not in names or e.get('replicas')!=0) for e in history)
        or {e['name'] for e in events[1:-1]}!=names or len(events)!=len(names)+2):fail()
    expected={n:{'uid':r['metadata']['uid'],'template_sha256':canonical(r['spec']['template']),
        'replicas':1,'ready':setup[n].get('ready')} for n,r in original.items()}
    if (setup!=expected or any(type(r['ready']) is not bool for r in setup.values()) or setup[HUB]['ready'] is not True
        or any(e.get('kind')!='workload_scaled' or e.get('name') not in names or e.get('replicas')!=1 for e in events[1:-1])
        or receipt.get('retained_setup')!=precut_retained_setup(snapshot)
        or receipt.get('storage_verified') is not True or receipt.get('config_verified') is not True):fail()
    configs=receipt.get('configmaps',{})
    if (not isinstance(configs,dict) or set(configs)!=precut_configmaps(original)
        or any(set(r)!={'uid','data_sha256','binary_data_sha256'} or not re.fullmatch('[a-f0-9-]{36}',r['uid'])
            or any(not re.fullmatch('[a-f0-9]{64}',r[k]) for k in ('data_sha256','binary_data_sha256')) for r in configs.values())):fail()
    marker=snapshot['context']['marker'];before={'uid':marker['metadata']['uid'],'resourceVersion':marker['metadata']['resourceVersion'],'data':marker['data']}
    after=receipt.get('released_marker',{});data=copy.deepcopy(before['data']);data['phase']='active';data.pop('knownRolloutOperation',None)
    if (receipt.get('before_marker')!=before or before['data'].get('phase')!='rollout-capturing'
        or before['data'].get('knownRolloutOperation')!=operation or set(after)!={'uid','resourceVersion','data'}
        or after.get('uid')!=before['uid'] or after.get('data')!=data or not isinstance(after.get('resourceVersion'),str)
        or after['resourceVersion']==before['resourceVersion']):fail()
    journals=receipt.get('journals',{})
    if not isinstance(journals,dict) or not 1<=len(journals)<=100:fail()
    for name,wanted in journals.items():
        if not re.fullmatch('[a-f0-9]{64}\\.json',name) or set(wanted)!={'journal_sha256','request_sha256','response_sha256'}:fail()
        installed=guard.ROOT/'installed';raw=private_bytes(installed/'journal'/name,2<<20);journal=json.loads(raw)
        request=journal.get('request',{});response_raw=private_bytes(installed/'responses'/name,2<<20,group_read=True)
        response=json.loads(response_raw);action=request.get('action')
        failed=(journal.get('phase')=='STARTED' and journal.get('prepare_error')=='guard_rejected'
            and response=={'status':'BLOCKED','error':'exception_Blocked'})
        complete=(journal.get('phase')=='COMPLETE' and journal.get('result')==response
            and response.get('operation')==operation and response.get('phase')=='AWAITING_OFF_NODE'
            and response.get('status')=='WAITING' and response.get('fence_status')=='VERIFIED')
        identity=(request.get('nonce')==name[:-5] and request.get('operation',operation)==operation)
        admitted=(action=='prepare' and request.get('mode')=='full' and name[:12]==operation
            and (complete if captured else failed))
        if captured:admitted=admitted or action=='authorize' and request.get('operation')==operation and failed or action=='status' and request.get('operation')==operation and complete
        if (hashlib.sha256(raw).hexdigest()!=wanted['journal_sha256'] or journal.get('request_sha256')!=wanted['request_sha256']
            or canonical(request)!=wanted['request_sha256'] or not identity or not admitted
            or hashlib.sha256(response_raw).hexdigest()!=wanted['response_sha256']):fail()
    if captured and not any(json.loads(private_bytes((guard.ROOT/'installed/journal'/n),2<<20))['request']['action']=='authorize' for n in journals):fail()
    terminal=copy.deepcopy(snapshot);terminal.update(phase='OPERATOR_RETIRED',status='ABORTED',fence_status='RELEASED',events=history+events)
    terminal[reference_key]=reference
    if state!=terminal:fail()
    return receipt



def verify_captured_evidence(base,snapshot,receipt):
    names={'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json',
        'rollout-backup.cms','decrypted-restore.json','apply-manifests.json'}
    files=receipt.get('retained_files',{})
    if not isinstance(files,dict) or set(files)!=names:fail()
    raw={}
    for name,row in files.items():
        if (set(row)!={'sha256','bytes'} or not re.fullmatch('[a-f0-9]{64}',row['sha256'])
            or type(row['bytes']) is not int or not 0<row['bytes']<=128<<20):fail()
        raw[name]=private_bytes(Path(base)/name,group_read=True)
        if len(raw[name])!=row['bytes'] or hashlib.sha256(raw[name]).hexdigest()!=row['sha256']:fail()
    cut=snapshot['cut'];cipher=snapshot['cipher_binding'];custody=snapshot['custody']
    hashes={'rollout-before.tar':cut['manifest']['archive_sha256'],
        'rollout-before-manifest.json':cut['manifest_sha256'],'rollout-backup.cms':cipher['cipher_sha256'],
        'apply-manifests.json':snapshot['target']['manifest_sha256']}
    if (any(files[n]['sha256']!=h for n,h in hashes.items())
        or json.loads(raw['rollout-before-manifest.json'])!=cut['manifest']
        or files['rollout-backup.cms']['bytes']!=cipher['cipher_bytes'] or cipher['operation']!=snapshot['operation']
        or custody.get('verified') is not True or any(custody.get(k)!=cipher[k] for k in (
            'operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes'))):fail()
    restored=json.loads(raw['decrypted-restore.json'])
    members={n:files[n]['sha256'] for n in ('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json')}
    members.update(snapshot.get('cipher_space_members',{}));members.update(cipher.get('space_members',{}));members.update(cipher.get('postseal_members',{}))
    if (restored.get('schema')!='voice-nats-decrypted-restore-v1' or restored.get('decrypted_members_verified') is not True
        or restored.get('remote_equivalence')!='FULL_CIPHERTEXT_SHA256_AND_SIZE'
        or restored.get('cipher_sha256')!=cipher['cipher_sha256'] or restored.get('cipher_bytes')!=cipher['cipher_bytes']
        or restored.get('members')!=members or restored.get('readback',{}).get('native_archive_verified') is not True):fail()

def idle_state(base,state):
    if state.get('status') in ('PASS','ROLLED_BACK'):return True
    verify_terminal(base,state)
    return True


PREBUILD_RECEIPTS={
    '161c282144ec':'ded7b5fb7a1342653b2067157b0b2410b4d888f5a4fc23cd6050d8fb8ebd3db3',
    '65607cfc9397':'707e8b70d15f0ead23da0f87ed1aac519b40435ff2774266fc7a0eb553a63594',
    'c8862dbc23a8':'ce3b0e55845c8bc898d02577c9e850f61de0063c435af65cfe4e8ccc1add4110'}

def pending_request_accepted(installed,path,closure):
    installed=Path(installed);path=Path(path)
    if path.parent!=installed/'journal' or not re.fullmatch('[a-f0-9]{64}\\.json',path.name):fail()
    row=document(path)
    if row.get('phase')=='COMPLETE':return
    if path.name in closure['journals']:
        journal_binding(installed,{path.name:closure['journals'][path.name]})
        return
    operation=path.stem[:12];wanted=PREBUILD_RECEIPTS.get(operation)
    if wanted is None:raise Blocked('bridge_upgrade_interrupted_request')
    raw=private_bytes(installed/('prebuild-disposition-'+operation+'.json'),2<<20)
    if hashlib.sha256(raw).hexdigest()!=wanted:fail()
    receipt=json.loads(raw)
    if (receipt.get('operation')!=operation or receipt.get('original_journal')!=row
        or receipt.get('journal_sha256')!=hashlib.sha256(private_bytes(path,2<<20)).hexdigest()
        or receipt.get('response_sha256')!=hashlib.sha256(private_bytes(installed/'responses'/path.name,2<<20,group_read=True)).hexdigest()
        or receipt.get('operation_absent') is not True or receipt.get('build_absent') is not True):fail()
    for candidate in (guard.ROOT/('rollout-'+operation),installed/'sources'/(operation+'-build')):
        try:candidate.lstat()
        except FileNotFoundError:continue
        fail()

class RetirementStage(Staging):
    def __init__(self,kube,snapshot,plan,journal):
        ctx=snapshot['context'];super().__init__(kube,OPERATION,copy.deepcopy(ctx['expected']),journal)
        for key in ('snapshots','marker','service','final_claim','final_pv'):
            setattr(self,key,copy.deepcopy(ctx[key]))
        self.final_path=Path(ctx['final_path']);self.original=ctx['original_snapshots'];self.plan=plan
        self.released=None;self.setup=None

    def owned_marker(self):
        marker=self.kube.get('configmap',MARKER)
        if (marker['metadata']['uid']!=self.marker['metadata']['uid']
            or marker['metadata']['resourceVersion']!=self.marker['metadata']['resourceVersion']
            or marker['data']!=self.marker['data']
            or marker['data'].get('phase')!='rollout-prepared'
            or marker['data'].get('knownRolloutOperation')!=OPERATION):fail()
        self.storage_config()
        return marker

    def storage_config(self):
        namespace=self.kube.get('namespace',guard.NS)
        claim=self.kube.get('pvc',self.expected['source_claim']);pv=self.kube.get('pv',claim['spec']['volumeName'])
        if (namespace['metadata']['uid']!=self.expected['namespace_uid']
            or claim['metadata']['uid']!=self.expected['source_claim_uid'] or claim['status']['phase']!='Bound'
            or pv['metadata']['uid']!=self.expected['source_pv_uid']
            or pv_storage_path(pv,claim['metadata']['uid'],claim['metadata']['name'],pv['metadata']['uid'])!=str(self.final_path)
            or claim['spec']!=self.final_claim['spec'] or pv['spec']!=self.final_pv['spec']
            or self.kube.get('service','voice-nats')!=self.service):fail()
        for name,wanted in self.plan['configmaps'].items():
            row=self.kube.get('configmap',name)
            if row['metadata']['uid']!=wanted['uid'] or canonical(row.get('data',{}))!=wanted['data_sha256']:fail()

    def release_marker(self):
        self.owned_marker();setup={}
        for name,original in self.original.items():
            if not getattr(self,'waived_continuation',False):self.wait_ready(name)
            row=self.kube.get('deployment',name)
            if (row['metadata']['uid']!=original['metadata']['uid'] or row['spec']['template']!=original['spec']['template']
                or row['spec'].get('replicas',1)!=1):fail()
            setup[name]={'uid':row['metadata']['uid'],'template_sha256':canonical(row['spec']['template']),
                'replicas':1,'ready':row.get('status',{}).get('observedGeneration',0)>=row['metadata']['generation']
                    and all(row.get('status',{}).get(k,0)==1 for k in ('readyReplicas','updatedReplicas','availableReplicas'))}
        if setup[HUB]['ready'] is not True:fail()
        marker=self.owned_marker()
        self.released=self.kube.cas('configmap',marker,[
            {'op':'test','path':'/data/phase','value':'rollout-prepared'},
            {'op':'test','path':'/data/knownRolloutOperation','value':OPERATION},
            {'op':'replace','path':'/data/phase','value':'active'},
            {'op':'remove','path':'/data/knownRolloutOperation'}])
        wanted=copy.deepcopy(marker['data']);wanted['phase']='active';wanted.pop('knownRolloutOperation')
        if self.released['metadata']['uid']!=marker['metadata']['uid'] or self.released['data']!=wanted:fail()
        self.setup=setup

    def restore_owned_cycle(self):
        row=self.kube.get('deployment','voice-user');original=self.original['voice-user']['spec']['template']
        if row['spec']['template']==original:return
        owned=self.snapshots['voice-user']['spec']['template']
        if (row['metadata']['uid']!=self.original['voice-user']['metadata']['uid']
            or row['spec']['template']!=owned
            or owned.get('metadata',{}).get('annotations',{}).get('voice.io/nats-user-space-bootstrap')!=OPERATION):fail()
        self.snapshots['voice-user']=self.kube.cas('deployment',row,[
            {'op':'test','path':'/spec/template','value':owned},
            {'op':'replace','path':'/spec/template','value':original}])
        self.save({'kind':'user_cycle_restored','owner':OPERATION})

def retire(code,plan_path,plan_sha,kube=None):
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('operator_retirement_human_root_required')
    code=Path(code);binding=root_cli.code_binding(code)
    plan_raw=private_bytes(plan_path,2<<20)
    if hashlib.sha256(plan_raw).hexdigest()!=plan_sha:fail()
    plan=json.loads(plan_raw)
    if (set(plan)!={'schema','operation','checkpoint_sha256','code_binding_sha256','waiver','configmaps','journals'}
        or plan['schema']!='voice-specific-operator-retirement-plan-v1' or plan['operation']!=OPERATION
        or plan['checkpoint_sha256']!=SNAPSHOT_SHA or plan['code_binding_sha256']!=canonical(binding)
        or plan['waiver']!=WAIVER or set(plan['configmaps'])!=CONFIGMAPS):fail()
    installed=guard.ROOT/'installed';base=root_cli.operation_path(str(guard.ROOT/('rollout-'+OPERATION)))
    with operation_lock():
        if canonical(root_cli.code_binding(installed/'code'))!=V15_BINDING:fail()
        inbox=installed/'inbox';old_mode=stat.S_IMODE(inbox.lstat().st_mode)
        if old_mode!=0o1730:fail()
        inbox.chmod(0o700)
        try:
            if any(inbox.iterdir()) or any((installed/'processing').iterdir()):fail()
            raw=private_bytes(base/'checkpoint.json')
            if hashlib.sha256(raw).hexdigest()!=SNAPSHOT_SHA:fail()
            snapshot=json.loads(raw)
            # Exact checkpoint bytes pin all private authority/contract inputs.
            terminal_state(snapshot,[{'kind':'operator_retirement_intent'},{'kind':'operator_retirement_completed'}],'a'*64)
            ctx=snapshot['context'];original=ctx['original_snapshots']
            if set(original)!=CORE or set(ctx['snapshots'])!=CORE:fail()
            for name,row in original.items():
                current=ctx['snapshots'][name]
                if (row['spec'].get('replicas',1)!=1 or current['spec'].get('replicas',1)!=0
                    or current['metadata']['uid']!=row['metadata']['uid'] or current['spec']['template']!=row['spec']['template']):fail()
            journal_binding(installed,plan['journals'])
            stage=None;events=[{'kind':'operator_retirement_intent','waiver':WAIVER,'plan_sha256':plan_sha}]
            progress=copy.deepcopy(snapshot)
            def journal(event):
                events.append(copy.deepcopy(event))
                progress.update(phase='OPERATOR_RETIRING',status='RUNNING',
                    events=copy.deepcopy(snapshot['events']+events))
                save(base/'checkpoint.json',progress,limit=128<<20)
            stage=RetirementStage(kube or Kube(),snapshot,plan,journal)
            stage.verify_closed()
            if stage.marker['metadata']['resourceVersion']!='3154269':fail()
            exclusive(base/'operator-retirement-snapshot.json',raw)
            exclusive(base/'operator-retirement-plan.json',plan_raw)
            progress.update(phase='OPERATOR_RETIRING',status='RUNNING',events=snapshot['events']+events)
            save(base/'checkpoint.json',progress,limit=128<<20)
            try:
                stage.restart()
                if stage.released is None or stage.setup is None:fail()
                events.append({'kind':'operator_retirement_completed'})
                receipt={'schema':'voice-operator-retirement-closure-v1','operation':OPERATION,
                    'snapshot_sha256':SNAPSHOT_SHA,'waiver':WAIVER,'plan_sha256':plan_sha,
                    'code_binding_sha256':canonical(binding),'deployment_pass':False,'rollback_eligible':False,
                    'events':events,'setup':stage.setup,'journals':plan['journals'],
                    'released_marker':{'uid':stage.released['metadata']['uid'],
                        'resourceVersion':stage.released['metadata']['resourceVersion'],'data':stage.released['data']},
                    'storage_verified':True,'config_verified':True,'user_cycle_restored':True}
                exclusive(base/'operator-retirement-receipt.json',
                    json.dumps(receipt,sort_keys=True,separators=(',',':')).encode())
                state=terminal_state(snapshot,events,canonical(receipt))
                verify_terminal(base,state);save(base/'checkpoint.json',state,limit=128<<20)
                return {'operation':OPERATION,'status':'ABORTED','phase':'OPERATOR_RETIRED',
                    'receipt_sha256':canonical(receipt),'setup_ready':True}
            except Exception:
                progress.update(phase='OPERATOR_RETIRING',status='BLOCKED',fence_status='UNKNOWN')
                try:
                    if stage.released is None:
                        stage.restore_owned_cycle()
                        if stage.refence().get('verified') is True:progress['fence_status']='VERIFIED'
                finally:
                    progress.update(phase='OPERATOR_RETIRING',status='BLOCKED')
                    save(base/'checkpoint.json',progress,limit=128<<20)
                raise
        finally:inbox.chmod(0o1730)


def failed_snapshot(snapshot,failed):
    expected=copy.deepcopy(snapshot)
    expected.update(phase='OPERATOR_RETIRING',status='BLOCKED',fence_status='VERIFIED',
        events=failed.get('events'))
    if (failed!=expected or len(failed.get('events',[]))!=125
        or failed['events'][:83]!=snapshot['events']
        or failed['events'][83].get('kind')!='operator_retirement_intent'
        or any(e.get('kind') not in {'operator_retirement_intent','workload_scaled',
            'user_cycle_override','user_cycle_restored'} for e in failed['events'][83:])):fail()

def continuation_document(row,plan_sha):
    if (not isinstance(row,dict) or set(row)!={'schema','operation','failed_checkpoint_sha256',
        'original_plan_sha256','code_binding_sha256','waiver'}
        or row['schema']!='voice-specific-retirement-continuation-v1'
        or row['operation']!=OPERATION or row['failed_checkpoint_sha256']!=FAILED_RETIREMENT_SHA
        or row['original_plan_sha256']!=plan_sha or row['waiver']!=WAIVER
        or not re.fullmatch('[a-f0-9]{64}',row['code_binding_sha256'])):fail()

def verify_continuation(base,snapshot,plan_raw,receipt):
    failed_raw=private_bytes(base/'operator-retirement-failed-attempt.json')
    if hashlib.sha256(failed_raw).hexdigest()!=FAILED_RETIREMENT_SHA:fail()
    failed=json.loads(failed_raw);failed_snapshot(snapshot,failed)
    raw=private_bytes(base/'operator-retirement-continuation.json',2<<20)
    row=json.loads(raw);continuation_document(row,hashlib.sha256(plan_raw).hexdigest())
    if (json.loads(plan_raw)['code_binding_sha256']!=ORIGINAL_RETIREMENT_BINDING
        or receipt.get('continuation')!={'attestation_sha256':hashlib.sha256(raw).hexdigest(),
            'code_binding_sha256':row['code_binding_sha256']}
        or receipt['events'][:42]!=failed['events'][83:]
        or receipt['events'][42]!={'kind':'operator_retirement_continuation_intent',
            'attestation_sha256':hashlib.sha256(raw).hexdigest(),
            'code_binding_sha256':row['code_binding_sha256']}):fail()
    return row

def continue_retirement(code,attestation_path,attestation_sha,kube=None):
    """One exact failed-attempt continuation under the explicit current-data waiver."""
    if os.geteuid()!=0 or sys.platform!='linux':raise Blocked('operator_retirement_human_root_required')
    code=Path(code);binding=root_cli.code_binding(code)
    installed=guard.ROOT/'installed';base=root_cli.operation_path(str(guard.ROOT/('rollout-'+OPERATION)))
    with operation_lock():
        if canonical(root_cli.code_binding(installed/'code'))!=V15_BINDING:fail()
        inbox=installed/'inbox'
        if stat.S_IMODE(inbox.lstat().st_mode)!=0o1730:fail()
        inbox.chmod(0o700)
        try:
            if any(inbox.iterdir()) or any((installed/'processing').iterdir()):fail()
            raw=private_bytes(base/'checkpoint.json')
            if hashlib.sha256(raw).hexdigest()!=FAILED_RETIREMENT_SHA:fail()
            failed=json.loads(raw)
            snapshot_raw=private_bytes(base/'operator-retirement-snapshot.json')
            if hashlib.sha256(snapshot_raw).hexdigest()!=SNAPSHOT_SHA:fail()
            snapshot=json.loads(snapshot_raw);failed_snapshot(snapshot,failed)
            plan_raw=private_bytes(base/'operator-retirement-plan.json',2<<20);plan=json.loads(plan_raw)
            if (plan.get('schema')!='voice-specific-operator-retirement-plan-v1'
                or plan.get('operation')!=OPERATION or plan.get('checkpoint_sha256')!=SNAPSHOT_SHA
                or plan.get('code_binding_sha256')!=ORIGINAL_RETIREMENT_BINDING
                or plan.get('waiver')!=WAIVER or set(plan.get('configmaps',{}))!=CONFIGMAPS):fail()
            att_raw=private_bytes(attestation_path,2<<20)
            if hashlib.sha256(att_raw).hexdigest()!=attestation_sha:fail()
            attestation=json.loads(att_raw);continuation_document(attestation,hashlib.sha256(plan_raw).hexdigest())
            if attestation['code_binding_sha256']!=canonical(binding):fail()
            journal_binding(installed,plan['journals'])
            events=copy.deepcopy(failed['events'][83:])
            events.append({'kind':'operator_retirement_continuation_intent',
                'attestation_sha256':attestation_sha,'code_binding_sha256':canonical(binding)})
            progress=copy.deepcopy(failed)
            def journal(event):
                events.append(copy.deepcopy(event))
                progress.update(phase='OPERATOR_RETIRING',status='RUNNING',
                    events=copy.deepcopy(snapshot['events']+events))
                save(base/'checkpoint.json',progress,limit=128<<20)
            stage=RetirementStage(kube or Kube(),snapshot,plan,journal)
            stage.waived_continuation=True
            stage.verify_closed()
            # Authenticate every unchanged template before the first startup effect.
            for name,original in stage.original.items():
                row=stage.kube.get('deployment',name)
                if (row['metadata']['uid']!=original['metadata']['uid']
                    or row['spec']['template']!=original['spec']['template']
                    or row['spec'].get('replicas',1)!=0):fail()
            exclusive(base/'operator-retirement-failed-attempt.json',raw)
            exclusive(base/'operator-retirement-continuation.json',att_raw)
            progress.update(phase='OPERATOR_RETIRING',status='RUNNING',
                events=copy.deepcopy(snapshot['events']+events))
            save(base/'checkpoint.json',progress,limit=128<<20)
            try:
                stage.scale(HUB,1);stage.wait_ready(HUB)
                for name in sorted(CORE-{HUB}):stage.scale(name,1)
                stage.release_marker()
                events.append({'kind':'operator_retirement_completed'})
                receipt={'schema':'voice-operator-retirement-closure-v1','operation':OPERATION,
                    'snapshot_sha256':SNAPSHOT_SHA,'waiver':WAIVER,
                    'plan_sha256':hashlib.sha256(plan_raw).hexdigest(),
                    'code_binding_sha256':plan['code_binding_sha256'],
                    'continuation':{'attestation_sha256':attestation_sha,'code_binding_sha256':canonical(binding)},
                    'deployment_pass':False,'rollback_eligible':False,
                    'events':events,'setup':stage.setup,'journals':plan['journals'],
                    'released_marker':{'uid':stage.released['metadata']['uid'],
                        'resourceVersion':stage.released['metadata']['resourceVersion'],'data':stage.released['data']},
                    'storage_verified':True,'config_verified':True,'user_cycle_restored':True}
                exclusive(base/'operator-retirement-receipt.json',
                    json.dumps(receipt,sort_keys=True,separators=(',',':')).encode())
                state=terminal_state(snapshot,events,canonical(receipt))
                verify_terminal(base,state);save(base/'checkpoint.json',state,limit=128<<20)
                return {'operation':OPERATION,'status':'ABORTED','phase':'OPERATOR_RETIRED',
                    'receipt_sha256':canonical(receipt),'setup_ready':all(r['ready'] for r in stage.setup.values())}
            except Exception:
                progress['fence_status']='UNKNOWN'
                try:
                    if stage.released is None and stage.refence().get('verified') is True:
                        progress['fence_status']='VERIFIED'
                finally:
                    progress.update(phase='OPERATOR_RETIRING',status='BLOCKED')
                    save(base/'checkpoint.json',progress,limit=128<<20)
                raise
        finally:inbox.chmod(0o1730)

if __name__=='__main__':
    sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
    continuation=len(sys.argv)==4 and sys.argv[1]=='--continue'
    if not continuation and len(sys.argv)!=3:raise SystemExit('usage: operator_retirement.py [--continue] REVIEWED_PRIVATE_DOCUMENT SHA256')
    try:
        code=Path(__file__).resolve().parents[1]
        result=(continue_retirement(code,sys.argv[2],sys.argv[3]) if continuation else retire(code,sys.argv[1],sys.argv[2]))
        print(json.dumps(result,sort_keys=True))
    except Exception:
        print('NATS_OPERATOR_RETIREMENT=BLOCKED',file=sys.stderr);sys.exit(1)
