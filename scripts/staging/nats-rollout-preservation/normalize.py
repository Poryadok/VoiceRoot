"""Pre-fence API normalization, never an application rollout or restart."""
import copy
import guard
from apply import paused_documents,digest
from controller import Blocked

def image_only_documents(stage,documents,target,renderer_authority=None):
    # Validate the compiled input before transforming it. Image-only mode
    # preserves current environment, mounts, identity refs and strategies.
    paused_documents(documents,{'target':target,'pvc':{'name':stage.expected['source_claim']}})
    rows=[];used={}
    for wanted in documents:
        name=wanted.get('metadata',{}).get('name')
        if wanted.get('kind')!='Deployment' or name not in stage.snapshots or name.startswith('voice-nats'):
            raise Blocked('rollout_images_only_scope_invalid')
        old=stage.snapshots[name]
        row={'apiVersion':'apps/v1','kind':'Deployment',
             'metadata':{'name':name,'namespace':guard.NS,'uid':old['metadata']['uid']},
             'spec':copy.deepcopy(old['spec'])}
        for c in row['spec']['template']['spec'].get('initContainers',[])+row['spec']['template']['spec']['containers']:
            key=name+'/'+c['name'];image=target['images'].get(key)
            if image is None:raise Blocked('rollout_images_only_container_changed')
            c['image']=image;used[key]=image
        rows.append(row)
    renderer_images={}
    if renderer_authority is not None:
        import renderer_root
        renderer_images=renderer_root.validate_target(target,renderer_authority,stage)
    if used|renderer_images!=target['images']:raise Blocked('rollout_images_only_container_changed')
    if guard.frontend_image_only(target) and not any(stage.old_images.get(key)!=image for key,image in used.items()):
        raise Blocked('rollout_frontend_actual_image_change_required')
    updated=copy.deepcopy(target)
    updated['template_hashes']={r['metadata']['name']:digest(r['spec']['template']) for r in rows}
    if renderer_authority is not None:updated['template_hashes'][renderer_root.HUB]=target['template_hashes'][renderer_root.HUB]
    updated['manifest_sha256']=digest(rows)
    return rows,updated

def normalize_target(kube,stage,documents,target,renderer_authority=None):
    receipt={'target':target,'pvc':{'name':stage.expected['source_claim']}}
    rows=paused_documents(documents,receipt)
    names={(r['kind'],r['metadata']['name']) for r in rows}
    if len(names)!=len(rows):raise Blocked('rollout_target_duplicate_resource')
    identities={}
    for kind,name in names:
        if name.startswith('voice-nats'):raise Blocked('rollout_target_nats_resource_forbidden')
        current=kube.get(kind.lower(),name)
        uid=current.get('metadata',{}).get('uid','')
        if not guard.UUID.fullmatch(uid):raise Blocked('rollout_target_resource_identity_invalid')
        identities[(kind,name)]=uid
        if kind=='Deployment' and (name not in stage.snapshots or uid!=stage.snapshots[name]['metadata']['uid']):
            raise Blocked('rollout_target_deployment_unenrolled')
    normalized=[]
    for row in rows:
        result=kube.run(['apply','--dry-run=server','-f','/dev/stdin','-o','json'],body=row)
        if not isinstance(result,dict):raise Blocked('rollout_target_normalization_invalid')
        normalized.append(result)
    wanted=copy.deepcopy(target);wanted['input_template_hashes']=dict(target['template_hashes'])
    templates={}
    for row in normalized:
        key=(row.get('kind'),row.get('metadata',{}).get('name'))
        if key not in identities or row.get('metadata',{}).get('uid')!=identities.pop(key):
            raise Blocked('rollout_target_normalization_identity_changed')
        if row['kind']=='Deployment':
            if row['spec'].get('replicas')!=0:raise Blocked('rollout_target_normalization_unpaused')
            template=row['spec']['template']
            for c in template['spec'].get('initContainers',[])+template['spec']['containers']:
                if target['images'].get(row['metadata']['name']+'/'+c['name'])!=c['image']:
                    raise Blocked('rollout_target_normalization_image_changed')
            templates[row['metadata']['name']]=digest(template)
    if renderer_authority is not None:
        import renderer_root
        renderer_root.validate_target(target,renderer_authority,stage)
        templates[renderer_root.HUB]=target['template_hashes'][renderer_root.HUB]
    if identities or set(templates)!=set(target['template_hashes']):raise Blocked('rollout_target_normalization_inventory_changed')
    wanted['template_hashes']=templates
    return wanted
