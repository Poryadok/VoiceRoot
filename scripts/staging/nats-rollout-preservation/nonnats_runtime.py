"""Verify the canonical non-NATS plan against root's exact owned workload ledger."""
import copy
import nonnats_plan as plan_api
from controller import Blocked

def preflight(kube,plan,mode):
    try:return plan_api.preflight(kube,plan,mode)
    except plan_api.PlanError as error:raise Blocked('rollout_nonnats_'+str(error)) from None

def owned_deployment_semantic(obj):
    result=plan_api.semantic(obj)
    annotations=result['metadata'].get('annotations',{})
    # The Deployment controller advances this after our target-template CAS.
    annotations.pop('deployment.kubernetes.io/revision',None)
    if not annotations:result['metadata'].pop('annotations',None)
    return result

def create_frontends(kube,plan,binding,state,stage,journal,mutation_guard):
    """Create only missing source-bound frontend configs under the owned fence."""
    rows=[row for row in binding.get('objects',[]) if row.get('disposition')=='create']
    wanted={(o['kind'],o['name']):o['desired'] for r in plan.get('checks',[]) for o in r['objects']
            if o.get('disposition',r['disposition'])=='create'}
    if not rows and not wanted:return
    if plan_api.digest(plan)!=binding['plan_sha256']:raise Blocked('rollout_nonnats_plan_changed')
    if len(rows)!=len(wanted) or any(wanted.get((r['kind'],r['name']))!=r['desired'] for r in rows):
        raise Blocked('frontend_bootstrap_target_changed')
    if state.get('gis_first_install') is not None:raise Blocked('frontend_bootstrap_gis_first_install_forbidden')
    records=state.setdefault('frontend_bootstrap',{})
    if records:raise Blocked('frontend_bootstrap_reconciliation_required')
    for row in rows:
        name=row['name']
        if row.get('kind')!='ConfigMap' or name not in plan_api.FRONTEND_BOOTSTRAP:
            raise Blocked('frontend_bootstrap_scope_invalid')
        stage.verify_final_storage()
        if plan_api.frontend_live(kube,name) is not None:raise Blocked('frontend_bootstrap_already_present')
        records[name]={'status':'pending','desired_sha256':plan_api.digest(row['desired'])}
        journal({'kind':'frontend_bootstrap_pending','name':name})
        stage.verify_final_storage()
        if plan_api.frontend_live(kube,name) is not None:raise Blocked('frontend_bootstrap_already_present')
        stage.verify_final_storage()
        mutation_guard()
        manifest=copy.deepcopy(row['desired'])
        manifest.setdefault('metadata',{}).update(name=name,namespace='voice-staging')
        try:created=kube.run(['create','-f','/dev/stdin','-o','json'],body=manifest)
        except Exception:raise Blocked('frontend_bootstrap_outcome_unknown') from None
        meta=created.get('metadata',{}) if isinstance(created,dict) else {}
        if (created.get('kind')!='ConfigMap' or meta.get('name')!=name or meta.get('namespace')!='voice-staging'
            or not all(isinstance(meta.get(k),str) and meta[k] for k in ('uid','resourceVersion'))
            or plan_api.semantic(created)!=row['desired']):
            raise Blocked('frontend_bootstrap_outcome_unknown')
        stage.verify_final_storage()
        live=plan_api.frontend_live(kube,name)
        if (live is None or any(live['metadata'].get(k)!=meta[k] for k in ('uid','resourceVersion'))
            or plan_api.semantic(live)!=row['desired']):raise Blocked('frontend_bootstrap_outcome_unknown')
        row.update(uid=meta['uid'],resourceVersion=meta['resourceVersion'])
        records[name]={'status':'created','uid':meta['uid'],'resourceVersion':meta['resourceVersion'],
                       'desired_sha256':plan_api.digest(row['desired'])}
        journal({'kind':'frontend_bootstrap_created','name':name,'uid':meta['uid']})

def verify(kube,plan,binding,stage,applied=False):
    if plan_api.digest(plan)!=binding['plan_sha256']:raise Blocked('rollout_nonnats_plan_changed')
    desired={(o['kind'],o['name']):o['desired'] for r in plan['checks'] for o in r['objects']}
    strict=copy.deepcopy(binding);strict['objects']=[];after=[]
    for descriptor in binding['objects']:
        key=(descriptor['kind'],descriptor['name'])
        if descriptor['disposition']=='create':
            if desired.get(key)!=descriptor['desired']:raise Blocked('frontend_bootstrap_target_changed')
            try:plan_api.revalidate(kube,{**binding,'objects':[descriptor]})
            except plan_api.PlanError as error:raise Blocked('rollout_nonnats_'+str(error)) from None
            if applied and 'uid' not in descriptor:raise Blocked('frontend_bootstrap_not_created')
            continue
        if descriptor['kind']=='Deployment' and descriptor['name'] in stage.snapshots:
            owned=stage.snapshots[descriptor['name']];current=kube.get(*key)
            # Controller status reconciliation may advance resourceVersion
            # after our scale CAS; identity and complete semantics stay strict.
            if owned['metadata']['uid']!=descriptor['uid'] or current['metadata']['uid']!=owned['metadata']['uid'] or owned_deployment_semantic(current)!=owned_deployment_semantic(owned):
                raise Blocked('rollout_nonnats_owned_workload_changed')
        elif applied and descriptor['kind']=='Service' and descriptor['disposition']=='preserve':
            # Full apply can update kubectl bookkeeping on an unchanged Service.
            # UID and complete semantics remain bound; pre-apply RVs stay strict.
            try:
                current=plan_api._read(kube,descriptor)
                if (current['metadata']['uid']!=descriptor['uid']
                    or plan_api.semantic(current)!=descriptor['desired']
                    or plan_api.semantic(current)!=desired[key]):
                    raise plan_api.PlanError('preserved_service_changed')
            except plan_api.PlanError as error:raise Blocked('rollout_nonnats_'+str(error)) from None
        elif applied and descriptor['disposition']=='action':
            current=kube.get(*key)
            if current['metadata']['uid']!=descriptor['uid'] or plan_api.semantic(current)!=desired[key]:
                raise Blocked('rollout_nonnats_applied_resource_changed')
        else:
            strict['objects'].append(descriptor);continue
        after.append({'kind':key[0],'name':key[1],'uid':current['metadata']['uid'],
                      'resourceVersion':current['metadata']['resourceVersion'],
                      'semantic_sha256':plan_api.digest(plan_api.semantic(current))})
    if strict['objects']:
        try:plan_api.revalidate(kube,strict)
        except plan_api.PlanError as error:raise Blocked('rollout_nonnats_'+str(error)) from None
    return {'verified':True,'owned_resources':after}
