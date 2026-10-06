"""Root-private authority and output proof for the single hub init image."""
import copy
import hashlib
import os
from pathlib import Path
import re
import tempfile
import time
from apply import digest
from bootstrap_root import secret_bytes
from commands import capture
from controller import Blocked
from stage_runtime import HUB
from runtime_stage import revision_template
import guard
import renderer_execution

def image_identity(image):
    """Resolve one fixed renderer URI; config digests are never pull aliases."""
    import source_authority as registry
    prefix=registry.REGISTRY+'/nats-hub-config-renderer@'
    if not isinstance(image,str) or not image.startswith(prefix) or not re.fullmatch(r'sha256:[a-f0-9]{64}',image[len(prefix):]):
        raise Blocked('renderer_registry_identity_invalid')
    deadline=time.monotonic()+30;name='nats-hub-config-renderer'
    try:
        token=registry._get('https://ghcr.io/token?service=ghcr.io&scope=repository:poryadok/voiceroot/'+name+':pull',{},deadline)['token']
        if not isinstance(token,str) or not token or '\r' in token or '\n' in token:raise ValueError()
        headers={'Authorization':'Bearer '+token,'Accept':','.join(('application/vnd.oci.image.index.v1+json',
            'application/vnd.oci.image.manifest.v1+json','application/vnd.docker.distribution.manifest.list.v2+json',
            'application/vnd.docker.distribution.manifest.v2+json'))}
        digest,body,_=registry._manifest(name,image[len(prefix):],headers,deadline)
        if 'manifests' in body:
            matches=[r for r in body['manifests'] if r.get('platform',{}).get('os')=='linux' and r.get('platform',{}).get('architecture')=='amd64' and not r.get('platform',{}).get('variant')]
            if len(matches)!=1:raise ValueError()
            child=matches[0];digest,body,length=registry._manifest(name,child['digest'],headers,deadline)
            if 'manifests' in body or type(child.get('size')) is not int or child['size']!=length:raise ValueError()
        config=body['config']
        if not re.fullmatch(r'sha256:[a-f0-9]{64}',config['digest']) or type(config['size']) is not int or not 0<config['size']<=4<<20:raise ValueError()
        raw,_=registry._fetch('https://ghcr.io/v2/poryadok/voiceroot/'+name+'/blobs/'+config['digest'],headers,4<<20,deadline)
        if len(raw)!=config['size'] or 'sha256:'+hashlib.sha256(raw).hexdigest()!=config['digest']:raise ValueError()
        row=registry._json(raw)
        if row.get('os')!='linux' or row.get('architecture')!='amd64':raise ValueError()
        return {'requested_image':image,'manifest_image':prefix+digest,'config_sha256':config['digest'][7:]}
    except Exception:
        raise Blocked('renderer_registry_identity_unavailable') from None


def image_aliases(authority):
    identities=authority.get('image_identities')
    if not isinstance(identities,dict) or set(identities)!={'old','target'}:raise Blocked('renderer_registry_binding_missing')
    prefix='ghcr.io/poryadok/voiceroot/nats-hub-config-renderer@sha256:'
    for role,row in identities.items():
        if not isinstance(row,dict) or set(row)!={'requested_image','manifest_image','config_sha256'} or row['requested_image']!=authority['descriptor']['images'][role]:
            raise Blocked('renderer_registry_binding_invalid')
        if any(not isinstance(row[key],str) or not row[key].startswith(prefix) or not guard.SHA.fullmatch(row[key][len(prefix):]) for key in ('requested_image','manifest_image')) or not isinstance(row['config_sha256'],str) or not guard.SHA.fullmatch(row['config_sha256']):
            raise Blocked('renderer_registry_binding_invalid')
    target=identities['target'];old=identities['old']
    if old['manifest_image']==target['manifest_image'] and old['config_sha256']!=target['config_sha256']:
        raise Blocked('renderer_registry_binding_invalid')
    aliases={target['requested_image'],target['manifest_image']}
    if old['manifest_image']==target['manifest_image'] and old['config_sha256']==target['config_sha256']:
        aliases.add(old['requested_image'])
    return aliases

def storage(stage):
    e=stage.expected
    return {'pvc_name':e['source_claim'],'pvc_uid':e['source_claim_uid'],
        'pv_uid':e['source_pv_uid'],'generation':e['generation']}

def inputs(kube,stage):
    spec=stage.snapshots[HUB]['spec']['template']['spec']
    volumes={v['name']:v for v in spec['volumes']}
    renderers=[c for c in spec.get('initContainers',[]) if c['name']=='nats-config-renderer']
    if len(volumes)!=len(spec['volumes']) or len(renderers)!=1 or renderers[0].get('args')!=list(renderer_execution.ARGS) or renderers[0].get('command',[]) not in ([],['/nats-hub-config-renderer']):
        raise Blocked('renderer_input_projection_unsupported')
    expected_mounts=[{'name':name,'mountPath':dest,**({'readOnly':True} if name!='nats-rendered-config' else {})} for name,dest in (
        ('nats-resolver-input','/run/nats/input'),('nats-config-template','/run/nats/template'),
        ('nats-rendered-config','/run/nats-rendered'),('nats-operator-jwt','/etc/nats/jwt'),('nats-hub-tls','/etc/nats/tls'))]
    if renderers[0].get('volumeMounts')!=expected_mounts:
        raise Blocked('renderer_input_projection_unsupported')
    generation=stage.expected['generation']
    if (volumes['nats-resolver-input']['secret']['secretName']!='voice-nats-operator-'+generation
        or volumes['nats-operator-jwt']['secret']['secretName']!='voice-nats-operator-'+generation
        or volumes['nats-hub-tls']['secret']['secretName']!='voice-nats-hub-tls-'+generation
        or volumes['nats-config-template']['configMap']['name']!='voice-nats-hub-config'):
        raise Blocked('renderer_input_generation_changed')
    for name,keys in (('nats-resolver-input',('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')),
                      ('nats-operator-jwt',('operator.jwt',)),('nats-hub-tls',('tls.crt','tls.key','ca.crt'))):
        if volumes[name]['secret'].get('items')!=[{'key':key,'path':key} for key in keys]:
            raise Blocked('renderer_input_projection_unsupported')
    if 'items' in volumes['nats-config-template']['configMap']:
        raise Blocked('renderer_input_projection_unsupported')
    rows={
        'operator':kube.get('secret','voice-nats-operator-'+generation),
        'tls':kube.get('secret','voice-nats-hub-tls-'+generation),
        'template':kube.get('configmap','voice-nats-hub-config')}
    binding={name:{'uid':row['metadata']['uid'],'resourceVersion':row['metadata']['resourceVersion'],
        'sha256':digest(row)} for name,row in rows.items()}
    return rows,binding

def revalidate(kube,stage,authority):
    stage.verify_selected_storage_identity()
    _,binding=inputs(kube,stage)
    if binding!=authority['input_binding'] or storage(stage)!=authority['descriptor']['storage_binding']:
        raise Blocked('renderer_input_authority_changed')

def bind_target(target,authority):
    """The hub is root-CAS-only, never a runner manifest resource."""
    result=copy.deepcopy(target);descriptor=authority['descriptor']
    if descriptor['source_sha']!=target['tag'] or HUB in target['template_hashes'] or any(k.startswith(HUB+'/') for k in target['images']):
        raise Blocked('renderer_target_binding_invalid')
    spec=descriptor['target_template']['spec']
    if len(spec['initContainers'])!=1 or len(spec['containers'])!=1 or spec['containers'][0]['name']!='nats':
        raise Blocked('renderer_target_inventory_unsupported')
    for c in spec['initContainers']+spec['containers']:
        if '@sha256:' not in c['image'] or not guard.SHA.fullmatch(c['image'].rsplit('@sha256:',1)[-1]):
            raise Blocked('renderer_target_image_unpinned')
        result['images'][HUB+'/'+c['name']]=c['image']
    result['template_hashes'][HUB]=digest(descriptor['target_template'])
    return result

def validate_target(target,authority,stage):
    import renderer_transition
    if authority['descriptor']['source_sha']!=target['tag']:
        raise Blocked('renderer_target_source_changed')
    snapshot=stage.snapshots[HUB];binding=authority['descriptor']['hub']
    if snapshot['metadata']['uid']!=binding['uid'] or snapshot['metadata']['resourceVersion']!=binding['resourceVersion']:
        raise Blocked('renderer_target_capture_changed')
    renderer_transition.validate(authority['descriptor'],snapshot,storage(stage),expected_hub=snapshot,paused=False)
    stripped=copy.deepcopy(target)
    stripped['template_hashes'].pop(HUB,None)
    stripped['images']={k:v for k,v in target['images'].items() if not k.startswith(HUB+'/')}
    expected=bind_target(stripped,authority)
    if expected['template_hashes']!=target['template_hashes'] or expected['images']!=target['images']:
        raise Blocked('renderer_target_binding_invalid')
    proof=authority.get('output',{})
    if set(proof)!={'sha256','bytes','verified'} or proof['verified'] is not True or not guard.SHA.fullmatch(proof['sha256']) or type(proof['bytes']) is not int or not 1<=proof['bytes']<=2<<20:
        raise Blocked('renderer_private_output_proof_missing')
    return {k:v for k,v in target['images'].items() if k.startswith(HUB+'/')}

def apply_paused(stage,authority):
    import renderer_transition
    stage.verify_closed();revalidate(stage.kube,stage,authority)
    current=stage.kube.get('deployment',HUB)
    renderer_transition.validate(authority['descriptor'],current,storage(stage),expected_hub=stage.snapshots[HUB])
    before=authority['descriptor']['original_template'];after=authority['descriptor']['target_template']
    if current['spec']['template']==after:return
    if current['spec']['template']!=before:raise Blocked('renderer_paused_template_changed')
    index=next(i for i,c in enumerate(before['spec']['initContainers']) if c['name']=='nats-config-renderer')
    stage.save({'kind':'rollout_renderer_template_intent','uid':current['metadata']['uid'],
        'resourceVersion':current['metadata']['resourceVersion'],'before_sha256':digest(before),'after_sha256':digest(after)})
    stage.snapshots[HUB]=stage.kube.cas('deployment',current,[
        {'op':'test','path':'/spec/replicas','value':0},
        {'op':'test','path':'/spec/template','value':before},
        {'op':'replace','path':'/spec/template/spec/initContainers/'+str(index)+'/image','value':authority['descriptor']['images']['target']}])
    if stage.snapshots[HUB]['spec']['template']!=after:raise Blocked('renderer_cas_target_changed')
    stage.save({'kind':'rollout_renderer_template_applied'})
    stage.verify_closed()

def recovery_authority(stage,authority):
    """Reverse only this operation's proven image delta on its paused ledger."""
    import renderer_transition
    stage.verify_closed();revalidate(stage.kube,stage,authority)
    current=stage.kube.get('deployment',HUB)
    renderer_transition.validate(authority['descriptor'],current,storage(stage),
        expected_hub=stage.snapshots[HUB])
    original=stage.original_snapshots[HUB]
    if original['metadata']['uid']!=authority['descriptor']['hub']['uid'] or original['spec']['template']!=authority['descriptor']['original_template']:
        raise Blocked('renderer_recovery_original_changed')
    key=HUB+'/nats-config-renderer'
    if stage.old_images.get(key)!=authority['descriptor']['images']['old']:
        raise Blocked('renderer_recovery_old_identity_changed')
    result=copy.deepcopy(authority)
    result['descriptor']=renderer_transition.plan(current,
        authority['descriptor']['images']['target'],authority['descriptor']['images']['old'],
        authority['descriptor']['source_sha'],storage(stage))
    image_aliases(authority)
    result['image_identities']={'old':copy.deepcopy(authority['image_identities']['target']),
        'target':copy.deepcopy(authority['image_identities']['old'])}
    # The original two-image equality proof applies symmetrically; input custody
    # and the unchanged broker binding are revalidated before the reverse CAS.
    return result


def prove(kube,stage,source_sha,target_image):
    import renderer_transition
    hub=stage.snapshots[HUB];key=HUB+'/nats-config-renderer'
    if key not in stage.old_images or HUB+'/nats' not in stage.old_images:
        raise Blocked('renderer_actual_old_image_missing')
    descriptor=renderer_transition.plan(hub,stage.old_images[key],target_image,source_sha,storage(stage))
    rows,binding=inputs(kube,stage)
    authority={'descriptor':descriptor,'input_binding':binding,
        'broker_image':stage.old_images[HUB+'/nats'],
        'image_identities':{'old':image_identity(descriptor['images']['old']),
            'target':image_identity(descriptor['images']['target'])}}
    def unchanged():
        # Original capture is still active at this pre-fence proof.
        current=kube.get('deployment',HUB)
        if current['metadata']['uid']!=hub['metadata']['uid'] or current['metadata']['resourceVersion']!=hub['metadata']['resourceVersion'] or current['spec']!=hub['spec']:
            raise Blocked('renderer_hub_capture_changed')
        revalidate(kube,stage,authority)
    with tempfile.TemporaryDirectory(prefix='renderer-proof-',dir=guard.ROOT) as td:
        base=Path(td);base.chmod(0o750);os.chown(base,0,65532)
        dirs={}
        for name in ('input','jwt','tls','template'):
            p=base/name;p.mkdir(mode=0o750);os.chown(p,0,65532);dirs[name]=p
        def write(path,raw):
            fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o440)
            with os.fdopen(fd,'wb') as f:f.write(raw)
            os.chown(path,0,65532)
        for key in ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public'):
            write(dirs['input']/key,secret_bytes(rows['operator'],key))
        write(dirs['jwt']/'operator.jwt',secret_bytes(rows['operator'],'operator.jwt'))
        for key in ('tls.crt','tls.key','ca.crt'):write(dirs['tls']/key,secret_bytes(rows['tls'],key))
        template=rows['template'].get('data',{}).get('nats.conf')
        if not isinstance(template,str) or not 1<=len(template.encode())<=2<<20:
            raise Blocked('renderer_template_invalid')
        write(dirs['template']/'nats.conf',template.encode())
        authority['output']=renderer_execution.prove(base,descriptor,dirs,unchanged)
    unchanged()
    return authority

def verify_live(stage,authority):
    """Called after HUB readiness and before the first application scale."""
    import renderer_transition
    revalidate(stage.kube,stage,authority)
    row=stage.kube.get('deployment',HUB)
    renderer_transition.validate(authority['descriptor'],row,storage(stage),
        expected_hub=stage.snapshots[HUB],paused=False)
    if row['spec'].get('replicas')!=1:raise Blocked('renderer_live_hub_replica_changed')
    for name in stage.snapshots:
        if name!=HUB and stage.kube.get('deployment',name)['spec'].get('replicas',1)!=0:
            raise Blocked('renderer_apps_released_before_proof')
    sets=stage.kube.run(['get','replicasets','-o','json'])['items']
    owners={r['metadata']['uid'] for r in sets if any(o.get('kind')=='Deployment' and o.get('uid')==row['metadata']['uid'] for o in r['metadata'].get('ownerReferences',[])) and revision_template(r['spec']['template'])==revision_template(row['spec']['template'])}
    pods=[p for p in stage.kube.run(['get','pods','-o','json'])['items'] if p.get('status',{}).get('phase')=='Running' and any(o.get('kind')=='ReplicaSet' and o.get('uid') in owners for o in p['metadata'].get('ownerReferences',[]))]
    if len(pods)!=1:raise Blocked('renderer_live_pod_ambiguous')
    pod=pods[0];status=pod['status']
    def actual(values,name):
        matches=[v for v in values if v['name']==name]
        if len(matches)!=1:raise Blocked('renderer_live_container_ambiguous')
        return matches[0]
    init=actual(status.get('initContainerStatuses',[]),'nats-config-renderer')
    broker=actual(status.get('containerStatuses',[]),'nats')
    if init.get('state',{}).get('terminated',{}).get('exitCode')!=0 or init.get('imageID','').removeprefix('docker-pullable://') not in image_aliases(authority) or broker.get('ready') is not True or broker.get('imageID','').removeprefix('docker-pullable://')!=authority['broker_image']:
        raise Blocked('renderer_live_image_or_completion_changed')
    name=pod['metadata']['name']
    if not re.fullmatch(r'voice-nats-pvc-candidate-[a-z0-9-]{1,100}',name):raise Blocked('renderer_live_pod_name_invalid')
    raw=capture(['/usr/local/bin/k3s','kubectl','--namespace',guard.NS,'exec',name,'-c','nats','--','/bin/busybox','sha256sum','/etc/nats/nats.conf'],limit=256,timeout=15)
    if raw.decode().strip()!=authority['output']['sha256']+'  /etc/nats/nats.conf':
        raise Blocked('renderer_live_output_changed')
    if stage.kube.get('pod',name)['metadata']['uid']!=pod['metadata']['uid']:
        raise Blocked('renderer_live_pod_identity_changed')
    return {'verified':True,'pod_uid':pod['metadata']['uid'],'renderer_image':authority['descriptor']['images']['target'],
        'broker_image':authority['broker_image'],'config_sha256':authority['output']['sha256']}
