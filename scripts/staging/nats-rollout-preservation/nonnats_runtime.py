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

def verify(kube,plan,binding,stage,applied=False):
    if plan_api.digest(plan)!=binding['plan_sha256']:raise Blocked('rollout_nonnats_plan_changed')
    desired={(o['kind'],o['name']):o['desired'] for r in plan['checks'] for o in r['objects']}
    strict=copy.deepcopy(binding);strict['objects']=[];after=[]
    for descriptor in binding['objects']:
        key=(descriptor['kind'],descriptor['name'])
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
