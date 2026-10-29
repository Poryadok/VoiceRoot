"""Pre-fence API normalization, never an application rollout or restart."""
import copy
import guard
from apply import paused_documents,digest
from controller import Blocked

def _contains_requested(requested,actual):
    if isinstance(requested,dict):
        return isinstance(actual,dict) and all(key in actual and _contains_requested(value,actual[key]) for key,value in requested.items())
    if isinstance(requested,list):
        return isinstance(actual,list) and len(requested)==len(actual) and all(_contains_requested(a,b) for a,b in zip(requested,actual))
    return type(requested) is type(actual) and requested==actual

def normalize_first_install(kube,stage,documents,target,plan,renderer_authority=None):
    """Normalize the isolated create-only GIS pair without generic apply."""
    import gis_first_install as gis
    if (documents!=[] or target.get('manifest_sha256')!=digest([])
        or plan.get('operation')!=stage.operation
        or target.get('gis_first_install_sha256')!=plan.get('source_plan_sha256')):
        raise Blocked('gis_first_install_normalization_binding_invalid')
    objects=plan.get('objects')
    if not isinstance(objects,list) or len(objects)!=2 or {gis._identity(o) for o in objects}!=set(gis.IDENTITIES):
        raise Blocked('gis_first_install_source_pair_invalid')
    deployment=next(o for o in objects if gis._identity(o)==gis.DEPLOYMENT)
    original_template=deployment['spec']['template']
    original_hash=target.get('template_hashes',{}).get('voice-gameintegration')
    if original_hash!=digest(original_template):raise Blocked('gis_first_install_template_changed')
    expected_images={}
    for container in original_template['spec'].get('initContainers',[])+original_template['spec'].get('containers',[]):
        key='voice-gameintegration/'+container['name'];expected_images[key]=container['image']
    renderer_images={}
    if renderer_authority is not None:
        import renderer_root
        renderer_images=renderer_root.validate_target(target,renderer_authority,stage)
    if expected_images|renderer_images!=target.get('images'):
        raise Blocked('gis_first_install_image_changed')
    expected_names={'voice-gameintegration'}|({renderer_root.HUB} if renderer_authority is not None else set())
    if set(target['template_hashes'])!=expected_names:raise Blocked('gis_first_install_template_inventory_invalid')
    gis.preflight(kube,plan)
    normalized=[]
    for obj in objects:
        result=kube.run(['create','--dry-run=server','-f','/dev/stdin','-o','json'],body=obj)
        if (not isinstance(result,dict) or gis._identity(result)!=gis._identity(obj)
            or result.get('metadata',{}).get('namespace')!=guard.NS
            or result.get('metadata',{}).get('annotations',{}).get(gis.MARKER)!=plan['operation']):
            raise Blocked('gis_first_install_normalization_identity_changed')
        actual=gis.semantic(result)
        if not _contains_requested(gis.semantic(obj),actual):raise Blocked('gis_first_install_source_fields_changed')
        normalized.append(actual)
    gis.preflight(kube,plan)
    service=next(o for o in normalized if gis._identity(o)==gis.SERVICE)
    requested_service=next(o for o in objects if gis._identity(o)==gis.SERVICE)
    if 'clusterIP' not in requested_service['spec'] and 'clusterIPs' not in requested_service['spec']:
        service['spec'].pop('clusterIP',None);service['spec'].pop('clusterIPs',None)
    new_deployment=next(o for o in normalized if gis._identity(o)==gis.DEPLOYMENT)
    if new_deployment['spec'].get('replicas')!=0:raise Blocked('gis_first_install_normalization_unpaused')
    spec=new_deployment['spec']['template']['spec']
    normalized_images={'voice-gameintegration/'+c['name']:c['image'] for c in spec.get('initContainers',[])+spec.get('containers',[])}
    if normalized_images!=expected_images:raise Blocked('gis_first_install_image_changed')
    updated_plan=copy.deepcopy(plan);updated_plan['objects']=normalized
    if 'clusterIP' not in requested_service['spec'] and 'clusterIPs' not in requested_service['spec']:
        updated_plan['service_allocation']='server'
    updated_plan['spec_sha256']={f'{k}/{n}':digest(gis.semantic(o)) for o in normalized for k,n in [gis._identity(o)]}
    wanted=copy.deepcopy(target);wanted['input_template_hashes']=dict(target['template_hashes'])
    wanted['template_hashes']['voice-gameintegration']=digest(new_deployment['spec']['template'])
    wanted['gis_first_install_normalized_sha256']=gis.normalized_binding(updated_plan)
    return wanted,updated_plan

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

def _verify_gis_normalization(kube,stage,binding):
    import commands,resource_claim,gis_existing_update
    session=getattr(commands.authority_guard,'__self__',None)
    if isinstance(session,resource_claim.Session) and session.check().get('cohort_preservation_sha256') is not None:
        import realtime_bootstrap,staging_cohort_preservation,staging_prerequisites
        protected=staging_cohort_preservation.capture_final_stage(session)
        if (staging_prerequisites.load_gis_authority(session)!=binding
            or realtime_bootstrap._stage_view(stage)!=realtime_bootstrap._stage_view(protected)
            or stage.original_snapshots!=protected.original_snapshots):
            raise Blocked('gis_existing_normalization_binding_changed')
        gis_existing_update.verify_phase(binding,kube,'paused-old')
        return True
    gis_existing_update.verify_phase(binding,kube,'active')
    return False


def normalize_target(kube,stage,documents,target,renderer_authority=None,*,gis_target_sink=None,gis_binding=None):
    if gis_target_sink is not None:
        if gis_binding is None:raise Blocked('gis_existing_normalization_binding_required')
        gis_paused=_verify_gis_normalization(kube,stage,gis_binding)
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
    if gis_target_sink is not None:
        if not callable(gis_target_sink) or 'voice-gameintegration' not in stage.snapshots:
            raise Blocked('gis_existing_normalization_scope_invalid')
        import gis_first_install as gis
        dep_rows=[row for row in normalized if row.get('kind')=='Deployment' and row.get('metadata',{}).get('name')=='voice-gameintegration']
        service_rows=[row for row in normalized if row.get('kind')=='Service' and row.get('metadata',{}).get('name')=='voice-gameintegration']
        if len(dep_rows)>1 or len(service_rows)>1:raise Blocked('gis_existing_normalization_scope_invalid')
        deployment=dep_rows[0] if dep_rows else copy.deepcopy(
            stage.original_snapshots['voice-gameintegration'] if gis_paused else stage.snapshots['voice-gameintegration'])
        service_live=kube.get('service','voice-gameintegration')
        service=service_rows[0] if service_rows else service_live
        if (deployment.get('metadata',{}).get('uid')!=stage.snapshots['voice-gameintegration']['metadata']['uid']
                or not guard.UUID.fullmatch(str(service.get('metadata',{}).get('uid','')))
                or service.get('metadata',{}).get('uid')!=service_live.get('metadata',{}).get('uid')
                or service.get('metadata',{}).get('namespace')!=guard.NS
                or service_live.get('metadata',{}).get('uid')!=gis_binding['service_uid']
                or gis.semantic(service_live)!=gis_binding['service']):
            raise Blocked('gis_existing_normalization_identity_changed')
        _verify_gis_normalization(kube,stage,gis_binding)
        pair={'deployment_mode':'selected-paused' if dep_rows else 'preserved-active',
              'deployment_uid':deployment['metadata']['uid'],'service_uid':service['metadata']['uid'],
              'deployment':gis.semantic(deployment),'service':gis.semantic(service)}
        wanted['gis_existing_normalized_sha256']=digest(pair)
        gis_target_sink(copy.deepcopy(deployment),copy.deepcopy(service),bool(dep_rows))
    return wanted
