"""Exact failed V3 prepare disposition; original journal is never rewritten."""
import hashlib
import json
import os
from pathlib import Path
import re
from controller import Blocked
from encrypted_cut import directory
from root_main import private_json,public_read,save

OPERATION='c8862dbc23a8'
RUN=37412334089
SOURCE='e0d635a8357bc9fbfa8b29202c33ac0e81b44644'
V3_BINDING='b0742fec4732b769944e5b849e39229d5f614989c4a439050ec48dd2ed20ab26'
MARKER={'uid':'cbafb90c-6974-4642-86d6-09b2eef1c843','resourceVersion':'3004975'}
def fail():raise Blocked('bridge_known_prebuild_disposition_refused')
def digest(row):return hashlib.sha256(json.dumps(row,sort_keys=True,separators=(',',':')).encode()).hexdigest()
def absent(path):
    try:path.lstat()
    except FileNotFoundError:return
    fail()

def enroll(root,path,marker,code_sha):
    root=directory(root);installed=directory(root/'installed');path=Path(path)
    directory(installed/'journal');directory(installed/'responses');directory(installed/'sources')
    if path.parent!=installed/'journal' or not re.fullmatch(OPERATION+r'[a-f0-9]{52}\.json',path.name):fail()
    original=private_json(path)
    request=original.get('request',{})
    expected={'action':'prepare','nonce':path.stem,'run_id':RUN,'source_sha':SOURCE,
        'mode':'images-only','changed_services':['user']}
    if (set(original)!={'schema','request_sha256','request','phase'}
        or original['schema']!='voice-nats-bridge-request-v1' or original['phase']!='STARTED'
        or request!=expected or original['request_sha256']!=digest(expected)):fail()
    response=json.loads(public_read(installed/'responses'/path.name))
    if response!={'status':'BLOCKED','error':'blocked_unclassified'}:fail()
    record=installed/('prebuild-disposition-'+OPERATION+'.json')
    identity={k:marker['metadata'][k] for k in ('uid','resourceVersion')}
    data=marker['data']
    if (identity!=MARKER or data.get('phase')!='active' or data.get('generation')!='r20260930a4'
        or data.get('dataPVC')!='voice-nats-jsdata-d202610040049430b' or code_sha!=V3_BINDING):fail()
    directory(installed/'sources'/OPERATION)
    absent(installed/'sources'/(OPERATION+'-build'));absent(root/('rollout-'+OPERATION))
    receipt={'schema':'voice-nats-known-prebuild-disposition-v1','operation':OPERATION,'run_id':RUN,
        'marker':identity,'generation':data['generation'],'pvc':data['dataPVC'],'code_sha256':code_sha,
        'original_journal':original,'journal_sha256':digest(original),'response_sha256':digest(response),
        'operation_absent':True,'build_absent':True,'disposition':'FAILED_BEFORE_TRANSACTION_NO_REPLAY'}
    try:
        if private_json(record)!=receipt:fail()
    except FileNotFoundError:save(record,receipt)
    # Preserve STARTED bytes for evidence and refuse replay; this receipt is
    # consumed only by the explicit fixed predecessor upgrade under its lock.
    if private_json(path)!=original:fail()
    absent(installed/'sources'/(OPERATION+'-build'));absent(root/('rollout-'+OPERATION))
    return receipt
