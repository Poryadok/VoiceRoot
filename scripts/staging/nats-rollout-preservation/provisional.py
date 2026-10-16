"""Explicit staging admission, acceptance and named repair-forward disposition."""
import copy
from pathlib import Path
import time
import guard
import nonnats_runtime
import source_authority
from controller import Blocked
from preserve import canonical
from root_main import private_json,save
from runtime_stage import RolloutStage,HUB

FIELDS={'schema','source_sha','run_id','run_attempt','helper_binding_sha256'}

def admission(source,execution,binding):
    if (source.get('verified') is not True or source.get('provisional_staging') is not True
        or execution.get('event')!='workflow_dispatch'
        or execution.get('workflow_id')!=263689731
        or execution.get('path')!='.github/workflows/staging-deploy.yml'
        or execution.get('head_sha')!=source.get('source_sha')):
        raise Blocked('provisional_dispatch_required')
    return {'schema':'voice-provisional-staging-v1','source_sha':source['source_sha'],
        'run_id':source['run_id'],'run_attempt':source['run_attempt'],
        'helper_binding_sha256':canonical(binding)}

def validate(value):
    if (not isinstance(value,dict) or set(value)!=FIELDS
        or value['schema']!='voice-provisional-staging-v1'
        or not isinstance(value['source_sha'],str) or len(value['source_sha'])!=40
        or any(c not in '0123456789abcdef' for c in value['source_sha'])
        or any(type(value[k]) is not int or value[k]<=0 for k in ('run_id','run_attempt'))
        or not isinstance(value['helper_binding_sha256'],str)
        or not guard.SHA.fullmatch(value['helper_binding_sha256'])):
        raise Blocked('provisional_binding_invalid')
    return value

def receipt_binding(state,receipt):
    expected=state.get('provisional');actual=receipt.get('provisional')
    if expected is None and actual is None:return
    if expected is None or actual is None or expected!=actual or receipt.get('operation')!=state['operation']:
        raise Blocked('provisional_binding_invalid')
    validate(actual)

def require_ready(state,binding):
    value=validate(state.get('provisional'))
    if (state.get('status')!='PROVISIONAL_READY' or state.get('phase')!='POST_RESTART'
        or state.get('fence_status')!='RELEASED'
        or value['helper_binding_sha256']!=canonical(binding)
        or state.get('code_capture')!=binding or state['target']['mode']!='images-only'
        or value['source_sha']!=state['target']['tag']
        or any(state['source_authority'].get(k)!=value[k] for k in ('source_sha','run_id','run_attempt'))):
        raise Blocked('provisional_released_parent_required')

def verify_runtime(state,kube,binding,active,*,require_healthy=True):
    """Read-only current runtime proof; never scale, apply, claim or restart."""
    require_ready(state,binding)
    expected=state['context']['expected'];operation=state['operation']
    if active!={'operation':operation,'namespace_uid':expected['namespace_uid'],
        'pvc_uid':expected['source_claim_uid'],'target_sha256':state['target']['manifest_sha256']}:
        raise Blocked('provisional_active_release_changed')
    marker=kube.get('configmap','voice-nats-generation')
    if (marker!=state['context']['marker'] or marker['data'].get('phase')!='active'
        or 'knownRolloutOperation' in marker['data']
        or not any(e.get('kind')=='rollout_released' and e.get('marker')==marker for e in state['events'])):
        raise Blocked('provisional_released_marker_changed')
    captured=RolloutStage.capture(kube,operation,lambda event:None,
        [name for name in state['context']['snapshots'] if name!=HUB])
    if captured.expected!=expected:raise Blocked('provisional_runtime_changed')
    captured.verify_selected_storage_identity()
    for name,before in state['context']['snapshots'].items():
        row=kube.get('deployment',name);status=row.get('status',{})
        if (row['metadata']['uid']!=before['metadata']['uid']
            or row['spec']!=before['spec'] or row['spec'].get('replicas',1)!=1
            or require_healthy and (status.get('observedGeneration',0)<row['metadata']['generation']
                or any(status.get(k,0)!=1 for k in ('readyReplicas','updatedReplicas','availableReplicas')))):
            raise Blocked('provisional_runtime_changed')
    # Compare owned resource metadata against the recorded post-restart state,
    # not against the fresh capture itself. Preserve all non-owned bindings too.
    captured.snapshots=copy.deepcopy(state['context']['snapshots'])
    nonnats_runtime.verify(kube,state['nonnats'],state['nonnats_binding'],captured,applied=True)
    wanted={**state['context']['old_images'],**state['target']['images']}
    deadline=time.monotonic()+120
    for key,image in captured.old_images.items():
        if key not in wanted or image!=wanted[key] and not source_authority.same_image_content(image,wanted[key],deadline):
            raise Blocked('provisional_runtime_changed')
    captured.verify_selected_storage_identity()
    if kube.get('configmap','voice-nats-generation')!=marker:raise Blocked('provisional_released_marker_changed')

def accept(base,state,token,kube,binding,active):
    require_ready(state,binding)
    if (Path(base)/'provisional-child.json').exists():raise Blocked('provisional_child_already_bound')
    source_authority.revalidate_ci(token,state['source_authority'],final=True)
    verify_runtime(state,kube,binding,active)
    # No checkpoint or workload mutation occurs before all acceptance proofs.
    accepted=copy.deepcopy(state);accepted['status']='PASS'
    accepted['ci_acceptance']={'run_id':state['provisional']['run_id'],
        'run_attempt':state['provisional']['run_attempt'],'verified':True}
    save(Path(base)/'checkpoint.json',accepted,limit=128<<20)
    return accepted

def parent_reference(state):
    return {'operation':state['operation'],'checkpoint_sha256':canonical(state)}

def link_parent(base,state,child):
    """A separate root-owned link preserves the unaccepted checkpoint bytes."""
    path=Path(base)/'provisional-child.json'
    value={'schema':'voice-provisional-repair-v1','parent':parent_reference(state),'child':child}
    if path.exists():
        if private_json(path)!=value:raise Blocked('provisional_child_already_bound')
    else:save(path,value)
    return value['parent']

def covered(base,state,*,admitted=None,seen=None):
    """Idle exception only for a verified chain ending at PASS or named parent."""
    seen=set() if seen is None else seen
    operation=state['operation']
    if operation in seen or len(seen)>=32:raise Blocked('provisional_chain_invalid')
    seen.add(operation)
    if admitted is not None and operation==admitted:return True
    path=Path(base)/'provisional-child.json'
    if not path.exists():return False
    link=private_json(path)
    if (set(link)!={'schema','parent','child'} or link['schema']!='voice-provisional-repair-v1'
        or link['parent']!=parent_reference(state) or not isinstance(link['child'],str)
        or len(link['child'])!=12 or any(c not in '0123456789abcdef' for c in link['child'])):
        raise Blocked('provisional_chain_invalid')
    child_base=Path(base).parent/('rollout-'+link['child'])
    child=private_json(child_base/'checkpoint.json',limit=128<<20)
    if child.get('operation')!=link['child'] or child.get('repair_parent')!=link['parent']:
        raise Blocked('provisional_chain_invalid')
    if child.get('status')=='PASS' and child.get('phase')=='POST_RESTART' and child.get('fence_status')=='RELEASED':return True
    return child.get('status')=='PROVISIONAL_READY' and covered(child_base,child,admitted=admitted,seen=seen)
