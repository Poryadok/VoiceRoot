"""CAS public script custody only after the fixed semantic/native proof.

Script bytes are never executed. Existing bootstrap Jobs and credentials are
untouched. An interrupted CAS is recognized only with its owned annotation.
"""
import copy,hashlib,re
from controller import Blocked
from nats_contract_plan import DECLARATIONS,SCHEMA,MIGRATION_ID,digest

ANNOTATION='voice-nats-preservation/contract-operation'
def _validated_target_pairs(plan):
    if not isinstance(plan,dict) or plan.get('schema')!=SCHEMA or plan.get('migration_id')!=MIGRATION_ID:raise Blocked('nats_contract_targets_unbound')
    targets=plan.get('consumer_targets')
    if not isinstance(targets,list) or len(targets)!=len(DECLARATIONS):raise Blocked('nats_contract_targets_unbound')
    pairs=[]
    for row,(stream,declaration) in zip(targets,DECLARATIONS):
        if (not isinstance(row,dict) or set(row)!={'stream','name','config_sha256'}
            or row.get('stream')!=stream or row.get('name')!=declaration['durable_name']
            or not isinstance(row.get('config_sha256'),str) or not re.fullmatch('[a-f0-9]{64}',row['config_sha256'])):
            raise Blocked('nats_contract_targets_unbound')
        pairs.append([stream,declaration['durable_name']])
    return pairs

def advance(kube,state,stage,journal,*,mutation_guard=None):
    migration=state.get('nats_migration',{})
    if migration.get('verified') is not True or migration.get('old_record_files_verified') is not True:raise Blocked('nats_contract_custody_before_proof')
    contract=state['contract'];wanted=state['nats_target_scripts']
    target_pairs=_validated_target_pairs(state.get('nats_contract'))
    existing_pairs=contract.get('consumer_pairs')
    if (not isinstance(existing_pairs,list) or any(not isinstance(pair,list) or len(pair)!=2
        or any(not isinstance(value,str) or not value for value in pair) for pair in existing_pairs)
        or len({tuple(pair) for pair in existing_pairs})!=len(existing_pairs)):
        raise Blocked('nats_contract_existing_pairs_invalid')
    complete_pairs=sorted({tuple(pair) for pair in existing_pairs}|{tuple(pair) for pair in target_pairs})
    for part,target in wanted.items():
        if part not in ('realtime','analytics-chat','search') or hashlib.sha256(target['script'].encode()).hexdigest()!=target['sha256']:raise Blocked('nats_contract_script_unbound')
        expected=next(r for r in contract['scripts'] if r['part']==part)
        stage.verify_final_storage();name='voice-nats-'+part+'-bootstrap';row=kube.get('configmap',name)
        raw=row['data']['bootstrap.sh'].encode();sha=hashlib.sha256(raw).hexdigest()
        if row['metadata']['uid']!=expected['configMapUID']:raise Blocked('nats_contract_cm_identity_changed')
        if sha==target['sha256']:
            if row['metadata']['resourceVersion']!=expected['configMapResourceVersion'] and row['metadata'].get('annotations',{}).get(ANNOTATION)!=state['operation']:raise Blocked('nats_contract_cm_unowned_target')
        else:
            if row['metadata']['resourceVersion']!=expected['configMapResourceVersion'] or sha!=expected['sha256']:raise Blocked('nats_contract_cm_drift')
            journal({'kind':'nats_contract_cm_intent','part':part,'uid':row['metadata']['uid'],'resourceVersion':row['metadata']['resourceVersion'],'target_sha256':target['sha256']})
            stage.verify_final_storage()
            annotations=copy.deepcopy(row['metadata'].get('annotations',{}));annotations[ANNOTATION]=state['operation']
            if mutation_guard is not None:mutation_guard()
            row=kube.cas('configmap',row,[{'op':'replace','path':'/data/bootstrap.sh','value':target['script']},{'op':'add','path':'/metadata/annotations','value':annotations}])
        if row['data']['bootstrap.sh']!=target['script']:raise Blocked('nats_contract_cm_target_changed')
        expected.update({'configMapResourceVersion':row['metadata']['resourceVersion'],'sha256':target['sha256'],'bytes':len(target['script'].encode())})
        journal({'kind':'nats_contract_cm_applied','part':part,'uid':row['metadata']['uid'],'resourceVersion':row['metadata']['resourceVersion'],'target_sha256':target['sha256']})
    contract['consumer_pairs']=[list(pair) for pair in complete_pairs]
    return {'schema':'voice-nats-active-contract-v1','operation':state['operation'],
        'generation':stage.expected['generation'],'namespace_uid':stage.expected['namespace_uid'],
        'pvc_uid':stage.expected['source_claim_uid'],'contract':copy.deepcopy(contract),
        'migration_id':state['nats_contract']['migration_id'],'plan_sha256':digest(state['nats_contract']),
        'sources':state['nats_contract']['sources'],'proof':copy.deepcopy(migration['proof'])}
