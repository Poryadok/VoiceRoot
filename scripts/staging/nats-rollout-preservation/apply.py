"""Apply only the root-frozen target while every application remains paused."""
import copy
import hashlib
import json
import os
from pathlib import Path
import stat
import sys
sys.path.insert(0,str(Path(__file__).resolve().parent))
import guard
from controller import Blocked

def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()

def paused_documents(documents,receipt):
    result=copy.deepcopy(documents)
    for row in result:
        kind=row.get('kind');meta=row.get('metadata',{});name=meta.get('name','')
        if meta.get('namespace')!=guard.NS or kind not in ('Deployment','ConfigMap','Service','NetworkPolicy','Ingress'):
            raise Blocked('rollout_resource_scope_invalid')
        if kind=='Deployment':
            spec=row['spec'];template=spec['template']
            if receipt['target'].get('input_template_hashes',receipt['target']['template_hashes']).get(name)!=digest(template):
                raise Blocked('rollout_target_template_changed')
            for c in template['spec'].get('initContainers',[])+template['spec']['containers']:
                if receipt['target']['images'].get(name+'/'+c['name'])!=c['image']:
                    raise Blocked('rollout_target_image_changed')
            if name=='voice-nats-pvc-candidate':
                claims=[v.get('persistentVolumeClaim',{}).get('claimName') for v in template['spec'].get('volumes',[]) if v['name']=='jsdata']
                if claims!=[receipt['pvc']['name']]:raise Blocked('rollout_rendered_claim_changed')
            spec['replicas']=0
        if kind=='Service' and name=='voice-nats' and row['spec'].get('selector')!={'app':'voice-nats-pvc-candidate'}:
            raise Blocked('rollout_hub_service_changed')
    return result

def main():
    receipt=guard.protected_receipt(os.environ['VOICE_NATS_PRESERVATION_RECEIPT'])
    guard.validate(receipt,os.environ['VOICE_IMAGE_REGISTRY'],os.environ['VOICE_IMAGE_TAG'],os.environ['DEPLOY_MODE'],[s for s in os.environ.get('CHANGED_SERVICES','').split(',') if s],guard.source_root())
    rv=os.environ['VOICE_NATS_ROLLOUT_CLAIM_RV'];guard.anchor(receipt,'rollout-applying',rv)
    path=Path(os.environ['VOICE_NATS_PRESERVATION_RECEIPT']).parent/'apply-manifests.json'
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        s=os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022 or s.st_nlink!=1 or not 1<=s.st_size<=2<<20:
            raise Blocked('rollout_manifest_custody_invalid')
        raw=os.read(fd,s.st_size+1)
    finally:os.close(fd)
    if len(raw)!=s.st_size or hashlib.sha256(raw).hexdigest()!=receipt['target']['manifest_sha256']:
        raise Blocked('rollout_manifest_changed')
    rows=json.loads(raw,object_pairs_hook=guard.object_pairs)
    if not isinstance(rows,list) or not 1<=len(rows)<=128:raise Blocked('rollout_manifest_shape_invalid')
    paused=paused_documents(rows,receipt)
    guard.anchor(receipt,'rollout-applying',rv)
    guard.kubectl(['apply','-f','/dev/stdin','-o','name'],{'apiVersion':'v1','kind':'List','items':paused})
    guard.anchor(receipt,'rollout-applying',rv)
    print('NATS_ROLLOUT_APPLY=PAUSED_COMPLETE')

if __name__=='__main__':
    try:main()
    except Exception:
        print('NATS_ROLLOUT_APPLY=BLOCKED',file=sys.stderr);sys.exit(1)
