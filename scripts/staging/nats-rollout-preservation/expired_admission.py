"""Root-only source and retained-input admission for one expired observation.

No dispatcher supplies hashes, snapshots, target overrides or credentials.
Called under the bridge operation lock; it leaves the checkpoint untouched.
"""
import copy
import hashlib
import json
from pathlib import Path
import time
import compiler
import guard
import paused_recovery
import expired_recovery as recovery
import transaction
from root_main import revalidate_inputs


def capture_grant_inventory(kube,base,captured,source_sha256):
    """ROOT-only captured template -> fixed retained service credential inventory.

    This is custody input to the signed grant verifier, never a grant proof.
    Raw credentials remain private and no request supplies a role or identity.
    Unsupported mounts, incomplete references or a changed live source veto.
    """
    import re
    from bootstrap_root import secret_bytes
    from preserved_upload import digest,private_read
    from expired_transport import put
    roles=set('auth social user role space chat file gateway voice messaging matchmaking search notification realtime bot subscription moderation story analytics'.split())
    if (not isinstance(captured,dict) or not captured or len(captured)>64
        or not re.fullmatch('[a-f0-9]{64}',source_sha256)):recovery.reject('grant_inventory_source_invalid')
    destination=Path(base)/'service-grants'
    if destination.exists():recovery.reject('grant_inventory_replay')
    rows=[];values={};sources={}
    for name,row in sorted(captured.items()):
        seen_roles=[]
        if kube.get('deployment',name)!=row:recovery.reject('grant_inventory_workload_changed')
        spec=row['spec']['template']['spec']
        if spec.get('initContainers') or spec.get('ephemeralContainers'):
            recovery.reject('grant_inventory_extra_actor_unsupported')
        for container in spec['containers']:
            env=[item for item in container.get('env',[]) if item.get('name')=='NATS_CREDS']
            if not env:continue
            if len(env)!=1 or set(env[0])!={'name','value'}:recovery.reject('grant_inventory_reference_invalid')
            path=env[0]['value'];prefix='/var/run/nats/creds/'
            if not isinstance(path,str) or not path.startswith(prefix):recovery.reject('grant_inventory_reference_invalid')
            key=path[len(prefix):];role=key.removesuffix('.creds')
            if key!=role+'.creds' or role not in roles or role in values:
                recovery.reject('grant_inventory_role_invalid')
            mounts=[item for item in container.get('volumeMounts',[]) if item.get('mountPath')==path]
            if len(mounts)!=1 or mounts[0].get('readOnly') is not True or mounts[0].get('subPath')!=key:
                recovery.reject('grant_inventory_mount_invalid')
            volumes=[item for item in spec.get('volumes',[]) if item.get('name')==mounts[0]['name']]
            expected={'secretName':'voice-nats-service-credentials','items':[{'key':key,'path':key}]}
            if len(volumes)!=1:recovery.reject('grant_inventory_volume_invalid')
            secret_spec=volumes[0].get('secret',{})
            if (secret_spec.get('secretName')!=expected['secretName'] or secret_spec.get('items')!=expected['items']
                or secret_spec.get('optional',False) is not False):recovery.reject('grant_inventory_secret_invalid')
            secret=kube.get('secret',expected['secretName']);metadata=secret['metadata']
            if any(not isinstance(metadata.get(field),str) or not metadata[field] for field in ('name','uid','resourceVersion')):
                recovery.reject('grant_inventory_secret_identity_invalid')
            raw=secret_bytes(secret,key)
            if not raw or len(raw)>65536:recovery.reject('grant_inventory_credential_size_invalid')
            sources[metadata['name']]=secret;values[role]=raw
            seen_roles.append(role)
            rows.append({'workload':name,'workload_uid':row['metadata']['uid'],
                'template_sha256':digest(row['spec']['template']),'container':container['name'],
                'role':role,'key':key,'credential_sha256':hashlib.sha256(raw).hexdigest(),
                'secret':{field:metadata[field] for field in ('name','uid','resourceVersion')}})
        expected_role=name.removeprefix('voice-')
        if expected_role in roles and seen_roles!=[expected_role]:
            recovery.reject('grant_inventory_required_role_missing')
    if not values:recovery.reject('grant_inventory_empty')
    for name,row in captured.items():
        if kube.get('deployment',name)!=row:recovery.reject('grant_inventory_workload_changed')
    for name,row in sources.items():
        if kube.get('secret',name)!=row:recovery.reject('grant_inventory_secret_changed')
    destination.mkdir(mode=0o700)
    for role,raw in values.items():
        path=destination/(role+'.creds')
        import os
        fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
        try:
            with os.fdopen(fd,'wb') as output:output.write(raw);output.flush();os.fsync(output.fileno())
        except BaseException:
            # A partial private input is not an admitted inventory or capability.
            raise
        if private_read(path,65536)!=raw:recovery.reject('grant_inventory_private_bytes_changed')
    result={'schema':'voice-root-retained-grant-input-v1','source_sha256':source_sha256,
        'captured_templates_sha256':digest(grant_template_identity(captured)),'roles':sorted(values),'workloads':rows}
    for name,row in captured.items():
        if kube.get('deployment',name)!=row:recovery.reject('grant_inventory_workload_changed')
    for name,row in sources.items():
        if kube.get('secret',name)!=row:recovery.reject('grant_inventory_secret_changed')
    put(destination/'inventory.json',result)
    return result


def grant_template_identity(captured):
    """Credential source identity excludes owned scale/controller observations."""
    return {name:{'uid':row['metadata']['uid'],'template':row['spec']['template']}
        for name,row in captured.items()}


def verify_grant_inventory(kube,base,captured,source_sha256,operator,account,account_public,verify):
    """Consume private fixed-role inventory at the signed-verifier use boundary.

    ``verify`` is the lexical ROOT current-helper invocation, never request data.
    Its output is still only signed effective grants, not retention eligibility.
    """
    from preserved_upload import private_json,private_read,digest
    folder=Path(base)/'service-grants';inventory=private_json(folder/'inventory.json',2<<20)
    if (inventory.get('schema')!='voice-root-retained-grant-input-v1'
        or inventory.get('source_sha256')!=source_sha256
        or inventory.get('captured_templates_sha256')!=digest(grant_template_identity(captured))):
        recovery.reject('grant_inventory_source_changed')
    credentials={};observed_workloads={}
    def unchanged():
        if private_json(folder/'inventory.json',2<<20)!=inventory:recovery.reject('grant_inventory_private_changed')
        for name,row in captured.items():
            current=kube.get('deployment',name)
            if (grant_template_identity({name:current})!=grant_template_identity({name:row})
                or current['spec'].get('replicas',1)!=row['spec'].get('replicas',1)
                or name in observed_workloads and observed_workloads[name]!=current):
                recovery.reject('grant_inventory_workload_changed')
            observed_workloads[name]=copy.deepcopy(current)
        from bootstrap_root import secret_bytes
        for row in inventory['workloads']:
            secret=kube.get('secret',row['secret']['name'])
            if any(secret['metadata'].get(key)!=value for key,value in row['secret'].items()):
                recovery.reject('grant_inventory_secret_changed')
            raw=private_read(folder/row['key'],65536)
            if (hashlib.sha256(raw).hexdigest()!=row['credential_sha256']
                or secret_bytes(secret,row['key'])!=raw):recovery.reject('grant_inventory_credential_changed')
            credentials[row['role']]=raw.decode('ascii')
        if sorted(credentials)!=inventory['roles']:recovery.reject('grant_inventory_roles_changed')
    unchanged()
    result=verify({'operator':operator,'account':account,'account_public':account_public,'credentials':credentials})
    unchanged()
    if (result.get('schema')!='voice-signed-retained-grants-v1'
        or result.get('account_public')!=account_public
        or result.get('operator_sha256')!=hashlib.sha256(operator.encode()).hexdigest()
        or result.get('account_sha256')!=hashlib.sha256(account.encode()).hexdigest()
        or sorted(result.get('roles',{}))!=inventory['roles']):recovery.reject('signed_grant_inventory_binding_invalid')
    for row in inventory['workloads']:
        if result['roles'][row['role']].get('credential_sha256')!=row['credential_sha256']:
            recovery.reject('signed_grant_credential_binding_invalid')
    return {'schema':'voice-root-signed-grant-inventory-v1','inventory_sha256':digest(inventory),
        'source_sha256':source_sha256,'signed_grants':result}


def input_hashes(kube,state):
    """Reconstruct fixed captured inputs from the unchanged enrolled identities."""
    from bootstrap_root import secret_bytes
    provenance=state['provenance'];values={}
    revalidate_inputs(kube,state['contract'],provenance)
    for key in ('bootstrap.creds','social.creds','realtime.creds','operator.jwt','account.jwt',
                'system-account.jwt','account.public','system-account.public'):
        descriptor=provenance[key]
        raw=secret_bytes(kube.get('secret',descriptor['resource']),key)
        if hashlib.sha256(raw).hexdigest()!=descriptor['sha256']:
            recovery.reject('input_source_bytes_changed')
        values[key]=raw
    tokens={key:values[key].decode().strip() for key in
        ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
    q=json.dumps
    config=('host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\nmax_control_line: 32768\n'
        'operator: '+q(tokens['operator.jwt'])+'\nsystem_account: '+q(tokens['system-account.public'])+
        '\nresolver: MEMORY\nresolver_preload: { '+q(tokens['account.public'])+': '+q(tokens['account.jwt'])+
        ', '+q(tokens['system-account.public'])+': '+q(tokens['system-account.jwt'])+' }\njetstream { store_dir: /data }\n')
    result={'inputs/'+key:hashlib.sha256(values[key]).hexdigest()
        for key in ('bootstrap.creds','social.creds','realtime.creds')}
    result['inputs/account.public']=hashlib.sha256(tokens['account.public'].encode()).hexdigest()
    result['server.conf']=hashlib.sha256(config.encode()).hexdigest()
    revalidate_inputs(kube,state['contract'],provenance)
    return result


def get_permission_proof(kube,state,input_bindings,unchanged,cached=None):
    """Same retained credential, every captured exact GET subject, before pause.

    The optional cache is solely the lexical ROOT admission result. Reuse still
    binds its exact credential bytes and subject inventory to current inputs;
    it cannot be supplied through a request or establish a new actor identity.
    """
    from bootstrap_root import secret_bytes,hash_bytes,ACCOUNT_SHA
    from bootstrap_auth import prove
    from preserved_upload import digest
    from docker_runtime import NATS_IMAGE
    names=[row['name'] for row in state['cut']['census']['streams']]
    if (state.get('nats_migration') or {}).get('verified'):
        names+= [row['name'] for row in state['nats_migration']['cut']['census']['streams']]
    if any(not isinstance(name,str) for name in names):recovery.reject('get_permission_subjects_invalid')
    names=sorted(set(names))
    expected={'schema':'voice-retained-get-capability-v1','operation':state['operation'],
        'server_image':NATS_IMAGE,'account_sha256':ACCOUNT_SHA,
        'credential_sha256':input_bindings['inputs/bootstrap.creds'],
        'subjects_sha256':hash_bytes(json.dumps(names,separators=(',',':')).encode()),'streams':len(names),'verified':True}
    if cached is not None:
        if cached!=expected:recovery.reject('get_permission_cached_binding_changed')
        unchanged();return cached
    enrollment=state['bootstrap_enrollment'];generation=enrollment['binding']['generation']
    credentials=secret_bytes(kube.get('secret',enrollment['secret']['name']),'bootstrap.creds')
    if hash_bytes(credentials)!=expected['credential_sha256']:recovery.reject('get_permission_credential_changed')
    actual=prove(credentials,kube.get('secret','voice-nats-operator-'+generation),unchanged,names)
    required={'schema':'voice-retained-get-permission-proof-v1',
        'subjects_sha256':expected['subjects_sha256'],'streams':len(names),'verified':True}
    if (actual['get_permissions']!=required or actual['server_image']!=NATS_IMAGE
        or actual['account_sha256']!=ACCOUNT_SHA):recovery.reject('get_permission_actual_proof_changed')
    unchanged();return expected


def admit(kube,base,state,code,binding,historical_source,verify_current_source,get_proof=None):
    """Every returned predicate has an actual production consumer, no cache waiver."""
    import actor_root
    import migrations
    import nonnats_runtime
    from preserved_upload import digest,private_read
    checkpoint=private_read(Path(base)/'checkpoint.json')
    recovery.verify_adopted_binding(base,state,binding)
    adopted=json.loads(private_read(Path(base)/'expired-recovery-adopted-checkpoint.json'))
    if state!=adopted:recovery.reject('admission_progress_present')
    stage=transaction.reconstruct(kube,state,lambda event:None)
    stage.owned_marker();stage.verify_final_storage()
    verify_current_source()
    workspace=guard.ROOT/'installed'/'sources'/state['operation']
    target=paused_recovery.recompile_target(base,state,stage,historical_source)
    if transaction.file_sha(Path(base)/'kernel')!=state['kernel_sha256']:
        recovery.reject('admission_kernel_changed')
    hashes=input_hashes(kube,state)
    get_proof=get_permission_proof(kube,state,hashes,verify_current_source,get_proof)
    actor_root.revalidate(kube,workspace,code,binding,stage,state['service_actor_services'],
        compiler.decode_yaml,state['service_actor_authority'])
    database=migrations.preflight(kube,state['migrations'],state['target']['mode'])
    if database!=state['migration_secret_metadata']:recovery.reject('admission_database_changed')
    nonnats_runtime.verify(kube,state['nonnats'],state['nonnats_binding'],stage)
    verify_current_source();stage.verify_final_storage()
    if private_read(Path(base)/'checkpoint.json')!=checkpoint:recovery.reject('admission_checkpoint_changed')
    return stage,{'schema':'voice-expired-native-admission-v1','operation':state['operation'],
        'checkpoint_sha256':hashlib.sha256(checkpoint).hexdigest(),'helper_binding_sha256':digest(binding),
        'target':target,'input_hashes':hashes,'database_sha256':digest(database),'get_permission_proof':get_proof}


def admit_finish(kube,base,state,code,binding,verify_current_source,get_proof=None):
    """Applied target observation has no recompile/apply/migration capability.

    The expired native authority is verified only at its immutable issuance
    timestamp as HISTORY. Only the separate completed finish proof/readback may
    subsequently issue a new short resume-only permission.
    """
    import actor_root
    import migrations
    import nonnats_runtime
    from preserved_upload import digest,private_read,timestamp
    base=Path(base);checkpoint=private_read(base/'checkpoint.json')
    recovery.verify_adopted_binding(base,state,binding);verify_current_source()
    if (state['operation']!='3340764a7d24' or state['phase'] not in ('PAUSED_APPLY','POST_APPLY_PROOF','RESTART')
        or state['status'] not in ('WAITING','BLOCKED')
        or state.get('nats_migration',{}).get('verified') is not True):recovery.reject('finish_admission_state_invalid')
    receipt=guard.protected_receipt(base/'apply-authorization.json')
    if (receipt!=state['authorization'] or receipt['operation']!=state['operation']
        or receipt['target']!=state['target']
        or transaction.file_sha(base/'apply-manifests.json')!=state['target']['manifest_sha256']):
        recovery.reject('finish_admission_applied_receipt_changed')
    authority=state['expired_recovery_authority']
    recovery.runtime_authority(base,state,authority,now=timestamp(authority['issued_at']))
    if receipt.get('expired_recovery_authority_sha256')!=digest(authority):recovery.reject('finish_native_history_changed')
    stage=transaction.reconstruct(kube,state,lambda event:None)
    actual_marker=kube.get('configmap','voice-nats-generation')
    prepared=stage.marker['data']['phase']=='rollout-prepared'
    if prepared:
        # A NEW proof observes the completed target. It does not reuse the old
        # finish RV gate or authorize application under its expired receipt.
        # Root binds the applying marker and ALL target UIDs/templates below.
        if (stage.marker['metadata']['uid']!=receipt['marker']['uid']
            or stage.marker['metadata']['resourceVersion']!=receipt['marker']['resourceVersion']
            or actual_marker['metadata']['uid']!=receipt['marker']['uid']
            or actual_marker['data'].get('phase')!='rollout-applying'
            or actual_marker['data'].get('knownRolloutOperation')!=state['operation']
            or any(actual_marker['data'].get(key)!=receipt['marker'][key]
                for key in ('generation','previousGeneration','dataPVC'))):recovery.reject('finish_applied_marker_changed')
        observed={}
        for name,captured in stage.snapshots.items():
            row=kube.get('deployment',name)
            wanted=state['target']['template_hashes'].get(name,digest(captured['spec']['template']))
            if (row['metadata']['uid']!=captured['metadata']['uid']
                or type(row['spec'].get('replicas',1)) is not int or row['spec'].get('replicas',1) not in (0,1)
                or digest(row['spec']['template'])!=wanted):recovery.reject('finish_applied_workload_changed')
            observed[name]=row
        stage.marker=actual_marker;stage.snapshots=observed
    elif actual_marker!=stage.marker:recovery.reject('finish_applied_marker_changed')
    stage.owned_marker();stage.verify_selected_storage_identity()
    if stage.marker['data']['phase'] not in ('rollout-applying','rollout-verified'):
        recovery.reject('finish_applied_marker_invalid')
    if transaction.file_sha(base/'kernel')!=state['kernel_sha256']:recovery.reject('finish_original_kernel_changed')
    hashes=input_hashes(kube,state)
    get_proof=get_permission_proof(kube,state,hashes,verify_current_source,get_proof)
    workspace=guard.ROOT/'installed'/'sources'/state['operation']
    actor_root.revalidate(kube,workspace,code,binding,stage,state['service_actor_services'],compiler.decode_yaml,state['service_actor_authority'])
    database=migrations.preflight(kube,state['migrations'],state['target']['mode'])
    expected=state.get('migration_completed_metadata',state['migration_secret_metadata'])
    if database!=expected:recovery.reject('finish_database_changed')
    nonnats_runtime.verify(kube,state['nonnats'],state['nonnats_binding'],stage,applied=True)
    verify_current_source();stage.owned_marker();stage.verify_selected_storage_identity()
    if private_read(base/'checkpoint.json')!=checkpoint:recovery.reject('finish_admission_checkpoint_changed')
    return stage,{'schema':'voice-expired-finish-admission-v1','operation':state['operation'],
        'checkpoint_sha256':hashlib.sha256(checkpoint).hexdigest(),'helper_binding_sha256':digest(binding),
        'applied_receipt_sha256':digest(receipt),'target_sha256':digest(state['target']),
        'native_history_sha256':digest(authority),'input_hashes':hashes,'database_sha256':digest(database),
        'get_permission_proof':get_proof}
