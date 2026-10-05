"""Verify the canonical non-NATS plan against root's exact owned workload ledger."""
import copy
import nonnats_plan as plan_api
from controller import Blocked

def preflight(kube,plan,mode):
    try:return plan_api.preflight(kube,plan,mode)
    except plan_api.PlanError as error:raise Blocked('rollout_nonnats_'+str(error)) from None

def verify(kube,plan,binding,stage,applied=False):
    if plan_api.digest(plan)!=binding['plan_sha256']:raise Blocked('rollout_nonnats_plan_changed')
    desired={(o['kind'],o['name']):o['desired'] for r in plan['checks'] for o in r['objects']}
    strict=copy.deepcopy(binding);strict['objects']=[];after=[]
    for descriptor in binding['objects']:
        key=(descriptor['kind'],descriptor['name'])
        if descriptor['kind']=='Deployment' and descriptor['name'] in stage.snapshots:
            owned=stage.snapshots[descriptor['name']];current=kube.get(*key)
            if owned['metadata']['uid']!=descriptor['uid'] or any(current['metadata'][k]!=owned['metadata'][k] for k in ('uid','resourceVersion')) or plan_api.semantic(current)!=plan_api.semantic(owned):
                raise Blocked('rollout_nonnats_owned_workload_changed')
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
