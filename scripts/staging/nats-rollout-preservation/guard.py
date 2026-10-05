"""Read a root-established target-bound rollout authorization before mutation.

This module cannot produce an authorization or choose a data claim. The root
preservation controller owns backup/custody; the runner may only claim its
single-use paused-apply phase with the exact observed marker resourceVersion.
"""
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
from commands import capture
from controller import Blocked
from stage_runtime import pv_storage_path

ROOT=Path('/var/lib/voice-nats-preservation')
NS='voice-staging'
MARKER='voice-nats-generation'
UUID=re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
SHA=re.compile(r'[a-f0-9]{64}')

def frontend_image_only(target):
    if target.get('mode')!='images-only':return False
    changed=set(target.get('changed_services',[]))
    if not changed or not changed<={'web','admin','developer-portal'}:return False
    names={'voice-'+name for name in changed}
    return set(target.get('template_hashes',{}))==names and bool(target.get('images')) and {key.split('/',1)[0] for key in target['images']}==names

def object_pairs(rows):
    value={}
    for key,item in rows:
        if key in value: raise Blocked('rollout_receipt_duplicate_key')
        value[key]=item
    return value

def source_root():
    # Captured runner lives outside the target workspace, including rollback.
    # Its immutable target file hashes are checked regardless of workspace name.
    return Path(os.environ.get('VOICE_NATS_TARGET_SOURCE',str(Path(__file__).resolve().parents[3]))).resolve(strict=True)

def protected_receipt(path, root=ROOT):
    path=Path(path)
    if path.parent.parent!=root or not re.fullmatch(r'rollout-[a-f0-9]{12}',path.parent.name) or path.name!='apply-authorization.json':
        raise Blocked('rollout_receipt_path_invalid')
    for directory in (*root.parents[::-1],root,path.parent):
        s=directory.lstat()
        if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022:
            raise Blocked('rollout_receipt_parent_untrusted')
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        s=os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022 or s.st_nlink!=1 or not 1<=s.st_size<=65536:
            raise Blocked('rollout_receipt_custody_invalid')
        raw=os.read(fd,s.st_size+1)
        if len(raw)!=s.st_size:raise Blocked('rollout_receipt_changed')
        return json.loads(raw,object_pairs_hook=object_pairs)
    finally:os.close(fd)

def validate(receipt, registry, tag, mode, changed, source_root):
    if set(receipt)!={'schema','operation','namespace_uid','marker','pvc','pv','target','backup','expires_at'} or receipt['schema']!='nats-rollout-preservation-v1':
        raise Blocked('rollout_receipt_schema_invalid')
    if not re.fullmatch(r'[a-f0-9]{12}',receipt['operation']) or not UUID.fullmatch(receipt['namespace_uid']):
        raise Blocked('rollout_receipt_identity_invalid')
    target=receipt['target']
    if set(target)!={'registry','tag','mode','changed_services','source_hashes','template_hashes','input_template_hashes','images','manifest_sha256','migration_sha256','nonnats_sha256'} or target['registry']!=registry or target['tag']!=tag or target['mode']!=mode or target['changed_services']!=sorted(set(changed)):
        raise Blocked('rollout_target_changed')
    if mode not in ('full','app-only','images-only') or not isinstance(target['source_hashes'],dict) or not 1<=len(target['source_hashes'])<=2048:
        raise Blocked('rollout_target_invalid')
    for name,digest in target['source_hashes'].items():
        p=Path(name)
        if p.is_absolute() or '..' in p.parts or not SHA.fullmatch(digest) or not str(p).startswith(('scripts/','deploy/','docker/clickhouse/init/','src/backend/migrations/','src/backend/auth/src/main/resources/db/migration/')):
            raise Blocked('rollout_source_path_invalid')
        actual=Path(source_root)/p
        if actual.is_symlink() or not actual.is_file() or actual.stat().st_size>2<<20 or hashlib.sha256(actual.read_bytes()).hexdigest()!=digest:
            raise Blocked('rollout_source_changed')
    if not SHA.fullmatch(target['manifest_sha256']) or not SHA.fullmatch(target['migration_sha256']) or not SHA.fullmatch(target['nonnats_sha256']) or not target['template_hashes'] or not target['images'] or any(not SHA.fullmatch(v) for v in target['template_hashes'].values()) or any(not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}@sha256:[a-f0-9]{64}',v) for v in target['images'].values()):
        raise Blocked('rollout_target_image_invalid')
    marker=receipt['marker'];pvc=receipt['pvc'];pv=receipt['pv'];backup=receipt['backup']
    if not isinstance(target['input_template_hashes'],dict) or set(target['input_template_hashes'])!=set(target['template_hashes']) or any(not SHA.fullmatch(v) for v in target['input_template_hashes'].values()):
        raise Blocked('rollout_input_templates_invalid')
    if set(marker)!={'uid','resourceVersion','generation','previousGeneration','dataPVC'} or not UUID.fullmatch(marker['uid']) or not re.fullmatch(r'[0-9]{1,20}',marker['resourceVersion']):
        raise Blocked('rollout_marker_invalid')
    if set(pvc)!={'name','uid','pv_name'} or not UUID.fullmatch(pvc['uid']) or pvc['name']!=marker['dataPVC'] or not re.fullmatch(r'voice-nats-jsdata-d[0-9]{8}[a-z0-9]{0,8}',pvc['name']) or pvc['pv_name']!='pvc-'+pvc['uid']:
        raise Blocked('rollout_pvc_invalid')
    expected='/var/lib/rancher/k3s/storage/'+pvc['pv_name']+'_'+NS+'_'+pvc['name']
    if set(pv)!={'uid','kind','path','node'} or not UUID.fullmatch(pv['uid']) or pv['kind'] not in ('local','hostPath') or pv['path']!=expected or pv['node']!='pmdebook':
        raise Blocked('rollout_pv_invalid')
    if set(backup)!={'archive_sha256','manifest_sha256','census_sha256','off_node_verified'} or backup['off_node_verified'] is not True or any(not SHA.fullmatch(backup[k]) for k in ('archive_sha256','manifest_sha256','census_sha256')):
        raise Blocked('rollout_backup_unverified')
    expires=dt.datetime.fromisoformat(receipt['expires_at'])
    now=dt.datetime.now(dt.timezone.utc)
    if expires.tzinfo is None or not now<expires<=now+dt.timedelta(hours=4):raise Blocked('rollout_receipt_expired')
    return receipt

def kubectl(args, body=None):
    return capture(['kubectl',*args],body=b'' if body is None else json.dumps(body).encode(),limit=65536,timeout=30)

def anchor(receipt, phase, resource_version):
    # Full objects stay inside this bounded guard and are never printed.
    namespace=json.loads(kubectl(['get','namespace',NS,'-o','json']),object_pairs_hook=object_pairs)
    pvc=json.loads(kubectl(['get','pvc',receipt['pvc']['name'],'-n',NS,'-o','json']),object_pairs_hook=object_pairs)
    pv=json.loads(kubectl(['get','pv',receipt['pvc']['pv_name'],'-o','json']),object_pairs_hook=object_pairs)
    hub=json.loads(kubectl(['get','deployment','voice-nats-pvc-candidate','-n',NS,'-o','json']),object_pairs_hook=object_pairs)
    if namespace['metadata']['uid']!=receipt['namespace_uid'] or pvc['metadata']['uid']!=receipt['pvc']['uid'] or pvc.get('status',{}).get('phase')!='Bound' or pvc['spec']['volumeName']!=receipt['pvc']['pv_name']:
        raise Blocked('rollout_storage_identity_changed')
    path=pv_storage_path(pv,receipt['pvc']['uid'],receipt['pvc']['name'],receipt['pv']['uid'])
    if path!=receipt['pv']['path'] or receipt['pv']['kind'] not in pv['spec']:
        raise Blocked('rollout_storage_kind_changed')
    claims=[v.get('persistentVolumeClaim',{}).get('claimName') for v in hub['spec']['template']['spec']['volumes'] if v['name']=='jsdata']
    if claims!=[receipt['pvc']['name']] or hub['spec'].get('replicas',1)!=0:
        raise Blocked('rollout_hub_not_paused_on_source')
    current=json.loads(kubectl(['get','configmap',MARKER,'-n',NS,'-o','json']),object_pairs_hook=object_pairs)
    expected=receipt['marker'];data=current.get('data',{})
    if current['metadata']['uid']!=expected['uid'] or current['metadata']['resourceVersion']!=resource_version or data.get('phase')!=phase or data.get('knownRolloutOperation')!=receipt['operation'] or any(data.get(k)!=expected[k] for k in ('generation','previousGeneration','dataPVC')):
        raise Blocked('rollout_ownership_changed')
    return current

def claim(receipt):
    expected=receipt['marker']
    anchor(receipt,'rollout-prepared',expected['resourceVersion'])
    patch=[{'op':'test','path':'/metadata/uid','value':expected['uid']},
           {'op':'test','path':'/metadata/resourceVersion','value':expected['resourceVersion']},
           {'op':'test','path':'/data/knownRolloutOperation','value':receipt['operation']},
           {'op':'test','path':'/data/dataPVC','value':expected['dataPVC']},
           {'op':'replace','path':'/data/phase','value':'rollout-applying'}]
    result=json.loads(kubectl(['patch','configmap',MARKER,'-n',NS,'--type=json','--patch-file=/dev/stdin','-o','json'],patch),object_pairs_hook=object_pairs)
    rv=result['metadata']['resourceVersion']
    if result['metadata']['uid']!=expected['uid'] or result.get('data',{}).get('phase')!='rollout-applying' or result['data'].get('knownRolloutOperation')!=receipt['operation'] or not re.fullmatch(r'[0-9]{1,20}',rv) or rv==expected['resourceVersion']:
        raise Blocked('rollout_claim_response_invalid')
    return rv

def main():
    path=os.environ.get('VOICE_NATS_PRESERVATION_RECEIPT','')
    receipt=protected_receipt(path)
    if Path(path).parent.name!='rollout-'+receipt['operation']:raise Blocked('rollout_operation_path_changed')
    validate(receipt,os.environ['VOICE_IMAGE_REGISTRY'],os.environ['VOICE_IMAGE_TAG'],os.environ['DEPLOY_MODE'],[s for s in os.environ.get('CHANGED_SERVICES','').split(',') if s],source_root())
    if sys.argv[1:]==['--precheck']:
        anchor(receipt,'rollout-prepared',receipt['marker']['resourceVersion'])
        print('NATS_ROLLOUT_AUTHORIZATION=PREPARED')
        return
    if sys.argv[1:]==['--check']:
        rv=os.environ.get('VOICE_NATS_ROLLOUT_CLAIM_RV','')
        if not re.fullmatch(r'[0-9]{1,20}',rv):raise Blocked('rollout_claim_rv_missing')
        anchor(receipt,'rollout-applying',rv)
        print('NATS_ROLLOUT_AUTHORIZATION=VALID')
    elif not sys.argv[1:]:print('NATS_ROLLOUT_CLAIM_RV='+claim(receipt))
    else:raise Blocked('rollout_guard_arguments_invalid')

if __name__=='__main__':
    try:main()
    except Exception:
        print('NATS_ROLLOUT_AUTHORIZATION=BLOCKED',file=sys.stderr);sys.exit(1)
