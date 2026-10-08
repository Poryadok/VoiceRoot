"""Two exact failed V4 preflights; retain original bytes and forbid replay."""
import hashlib
import json
from pathlib import Path
from controller import Blocked
from encrypted_cut import directory
from root_main import private_json,public_read,save
import prebuild_disposition

V4_BINDING='426a6edf16ccb8aba30145cfde458c9b2ca5d468cdf4015ea8bdf525e049f47f'
SOURCE='86d85d361a004e2346bc7f454055147ab6c51731'
CASES={
    '65607cfc93978a0b395bf69fb98c35f51f15b7c563b433eefa1674b65aa7009e':37579773420,
    '161c282144ec3d2c10d018c5ee89818b3a06301fde5306d2494d672cfbb246b2':37620052558,
}

def fail():raise Blocked('bridge_v4_prebuild_disposition_refused')

def enroll(root,path,marker,code_sha):
    root=directory(root);installed=directory(root/'installed');path=Path(path)
    directory(installed/'journal');directory(installed/'responses');directory(installed/'sources')
    if path.parent!=installed/'journal' or path.suffix!='.json':fail()
    nonce=path.stem
    if nonce.startswith(prebuild_disposition.OPERATION):
        # This old receipt must already exist; V5 does not enroll a V3 failure.
        private_json(installed/('prebuild-disposition-'+prebuild_disposition.OPERATION+'.json'))
        return prebuild_disposition.enroll(root,path,marker,prebuild_disposition.V3_BINDING)
    if nonce not in CASES or code_sha!=V4_BINDING:fail()
    op=nonce[:12];original=private_json(path);original_bytes=public_read(path)
    wanted={'action':'prepare','nonce':nonce,'run_id':CASES[nonce],'source_sha':SOURCE,
        'mode':'images-only','changed_services':['story']}
    if (set(original)!={'schema','request_sha256','request','phase','prepare_stage','prepare_error'}
        or original['schema']!='voice-nats-bridge-request-v1' or original['phase']!='STARTED'
        or original['request']!=wanted or original['request_sha256']!=prebuild_disposition.digest(wanted)
        or original['prepare_stage']!={'name':'nats-preflight','status':'STARTED'}
        or original['prepare_error']!='guard_rejected'):fail()
    response=json.loads(public_read(installed/'responses'/path.name))
    if response!={'status':'BLOCKED','error':'exception_Blocked'}:fail()
    identity={k:marker['metadata'][k] for k in ('uid','resourceVersion')};data=marker['data']
    if (identity!=prebuild_disposition.MARKER or data.get('phase')!='active'
        or data.get('generation')!='r20260930a4'
        or data.get('dataPVC')!='voice-nats-jsdata-d202610040049430b'):fail()
    directory(installed/'sources'/op)
    prebuild_disposition.absent(installed/'sources'/(op+'-build'))
    prebuild_disposition.absent(root/('rollout-'+op))
    receipt={'schema':'voice-nats-v4-prebuild-disposition-v1','operation':op,'run_id':CASES[nonce],
        'marker':identity,'generation':data['generation'],'pvc':data['dataPVC'],'code_sha256':code_sha,
        'original_journal':original,'journal_sha256':hashlib.sha256(original_bytes).hexdigest(),
        'response_sha256':hashlib.sha256(public_read(installed/'responses'/path.name)).hexdigest(),
        'operation_absent':True,'build_absent':True,'disposition':'FAILED_BEFORE_TRANSACTION_NO_REPLAY'}
    record=installed/('prebuild-disposition-'+op+'.json')
    try:
        if private_json(record)!=receipt:fail()
    except FileNotFoundError:save(record,receipt)
    if private_json(path)!=original or public_read(path)!=original_bytes:fail()
    prebuild_disposition.absent(installed/'sources'/(op+'-build'))
    prebuild_disposition.absent(root/('rollout-'+op))
    return receipt
