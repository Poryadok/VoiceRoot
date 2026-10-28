"""Identity-bound GIS lifecycle primitives for a protected existing deployment."""
import copy
import re

import gis_first_install as gis
import guard
from root_main import private_json


def load_authority(kube):
    """Resolve only the root-owned active reference and its original PASS."""
    try:
        active = private_json(guard.ROOT / 'installed' / 'active-release.json')
    except FileNotFoundError:
        active = {}
    if not isinstance(active, dict):
        raise gis.Blocked('gis_existing_active_reference_invalid')
    pair = gis._read_pair(kube, None)
    reference = active.get('gis_first_install_transition')
    if reference is None:
        if any(pair.values()):
            raise gis.Blocked('gis_existing_anchor_missing')
        return None
    if (not isinstance(reference, dict)
            or set(reference) != {'operation', 'receipt_sha256'}
            or not isinstance(reference.get('operation'), str)
            or not re.fullmatch('[a-f0-9]{12}', reference['operation'])
            or not isinstance(reference.get('receipt_sha256'), str)
            or not re.fullmatch('[a-f0-9]{64}', reference['receipt_sha256'])):
        raise gis.Blocked('gis_existing_active_reference_invalid')
    try:
        state = private_json(guard.ROOT / ('rollout-' + reference['operation']) / 'checkpoint.json', limit=128 << 20)
    except FileNotFoundError:
        raise gis.Blocked('gis_existing_anchor_missing') from None
    receipt = state.get('gis_first_install_transition_receipt') if isinstance(state, dict) else None
    if not isinstance(receipt, dict) or receipt.get('receipt_sha256') != reference['receipt_sha256']:
        raise gis.Blocked('gis_existing_anchor_changed')
    if 'gis_latest_transition' in active:
        return load_successors(kube, reference, receipt, state, active['gis_latest_transition'])
    return enroll_first_install(receipt, state, kube)


def load_successors(kube, anchor, original_receipt, original_state, latest):
    """Resolve the root-private PASS chain back to the first-install anchor."""
    import resource_claim
    if original_state.get('status') != 'PASS':
        raise gis.Blocked('gis_existing_anchor_invalid')
    gis.validate_transition_receipt(original_receipt, original_state)
    if original_receipt['receipt_sha256'] != anchor['receipt_sha256']:
        raise gis.Blocked('gis_existing_anchor_changed')
    fields = {'schema','operation','anchor','predecessor','namespace_uid','service_uid','deployment_uid',
              'source_sha','source_run_id','source_run_attempt','source_authority_sha256','old_pair_sha256',
              'normalized_pair_sha256','ready_pair_sha256','target_sha256','resource_fence_sha256',
              'custody_sha256','preservation_sha256','ready_proof_sha256','receipt_sha256'}
    section_fields = {'receipt','old_pair','normalized_pair','ready_pair','source_authority','resource_fence',
                      'custody','preservation','ready_proof'}
    chain=[];seen=set();ref=latest
    for _ in range(64):
        if (not isinstance(ref,dict) or set(ref)!={'operation','receipt_sha256'}
                or not re.fullmatch('[a-f0-9]{12}',str(ref.get('operation','')))
                or not re.fullmatch('[a-f0-9]{64}',str(ref.get('receipt_sha256','')))
                or ref['operation'] in seen or ref['operation']==anchor['operation']):
            raise gis.Blocked('gis_existing_successor_chain_invalid')
        seen.add(ref['operation'])
        try:state=private_json(guard.ROOT/('rollout-'+ref['operation'])/'checkpoint.json',limit=128 << 20)
        except FileNotFoundError:raise gis.Blocked('gis_existing_successor_missing') from None
        section=state.get('gis_existing_update') if isinstance(state,dict) else None
        if state.get('status')!='PASS' or not isinstance(section,dict) or set(section)!=section_fields:
            raise gis.Blocked('gis_existing_successor_invalid')
        receipt=section['receipt']
        if (not isinstance(receipt,dict) or set(receipt)!=fields or receipt['schema']!='voice-gis-existing-update-receipt-v1'
                or receipt['operation']!=ref['operation'] or state.get('operation')!=ref['operation']
                or receipt['receipt_sha256']!=ref['receipt_sha256']
                or receipt['receipt_sha256']!=gis.digest({k:v for k,v in receipt.items() if k!='receipt_sha256'})
                or receipt['anchor']!=anchor or receipt['predecessor']==ref
                or not isinstance(state.get('gis_expected_active'),dict)
                or state['gis_expected_active'].get('gis_first_install_transition')!=anchor
                or receipt['predecessor']!=state['gis_expected_active'].get('gis_latest_transition',anchor)
                or receipt['namespace_uid']!=original_receipt['namespace_uid']
                or section['source_authority']!=state.get('gis_source_authority')
                or section['resource_fence']!=state.get('resource_fence')
                or section['custody']!=state.get('custody')
                or section['ready_proof']!=state.get('gis_ready_proof')
                or section['normalized_pair']!=state.get('gis_normalized_pair')
                or receipt['source_authority_sha256']!=gis.digest(section['source_authority'])
                or receipt['resource_fence_sha256']!=gis.digest(section['resource_fence'])
                or receipt['custody_sha256']!=gis.digest(section['custody'])
                or receipt['preservation_sha256']!=gis.digest(section['preservation'])
                or receipt['ready_proof_sha256']!=gis.digest(section['ready_proof'])
                or receipt['old_pair_sha256']!=gis.digest(section['old_pair'])
                or receipt['normalized_pair_sha256']!=gis.digest(section['normalized_pair'])
                or receipt['ready_pair_sha256']!=gis.digest(section['ready_pair'])
                or receipt['target_sha256']!=gis.digest(state['target'])
                or state['target'].get('gis_existing_normalized_sha256')!=receipt['normalized_pair_sha256']
                or receipt['source_sha']!=section['source_authority'].get('source_sha')
                or receipt['source_run_id']!=section['source_authority'].get('run_id')
                or receipt['source_run_attempt']!=section['source_authority'].get('run_attempt')):
            raise gis.Blocked('gis_existing_successor_invalid')
        resource_claim.historical_binding(state)
        validate_source_authority(section['source_authority'],state['target'])
        if successor_evidence(guard.ROOT/('rollout-'+ref['operation']),state)!=section['preservation']:
            raise gis.Blocked('gis_existing_successor_evidence_invalid')
        bound=state.get('gis_binding');validate(bound)
        pair=section['normalized_pair']
        if (not isinstance(pair,dict) or set(pair)!={'deployment_mode','deployment_uid','service_uid','deployment','service'}
                or pair['deployment_mode'] not in ('selected-paused','preserved-active')):
            raise gis.Blocked('gis_existing_successor_binding_changed')
        deployment=copy.deepcopy(pair['deployment']);service=copy.deepcopy(pair['service'])
        deployment['metadata']['uid']=pair['deployment_uid'];service['metadata']['uid']=pair['service_uid']
        target=state['target'];template=deployment['spec']['template']
        gis_images={key:value for key,value in target['images'].items() if key.startswith('voice-gameintegration/')}
        if pair['deployment_mode']=='selected-paused':
            containers=template['spec'].get('initContainers',[])+template['spec']['containers']
            expected_images={'voice-gameintegration/'+row['name']:row['image'] for row in containers}
            if (len(expected_images)!=len(containers) or gis_images!=expected_images
                    or target['template_hashes'].get('voice-gameintegration')!=gis.digest(template)):
                raise gis.Blocked('gis_existing_successor_binding_changed')
        elif (gis_images or 'voice-gameintegration' in target['template_hashes']
              or pair['deployment']!=bound['old'] or pair['service']!=bound['service']):
            raise gis.Blocked('gis_existing_successor_binding_changed')
        initial=copy.deepcopy(bound);initial['target']=None;initial['target_service']=None;initial=seal(initial)
        reconstructed=bind_normalized_target(initial,deployment,service,pair['deployment_mode']=='selected-paused',
                                             state['target']['mode'],receipt['normalized_pair_sha256'])
        if reconstructed!=bound:raise gis.Blocked('gis_existing_successor_binding_changed')
        ready_proof=section['ready_proof']
        if (bound['anchor_sha256']!=anchor['receipt_sha256'] or bound['operation']!=anchor['operation']
                or bound['namespace_uid']!=receipt['namespace_uid']
                or bound['deployment_uid']!=receipt['deployment_uid'] or bound['service_uid']!=receipt['service_uid']
                or section['old_pair']!={'deployment_uid':bound['deployment_uid'],'service_uid':bound['service_uid'],
                                         'deployment':bound['old'],'service':bound['service']}
                or section['ready_pair']['service']!=bound['target_service']
                or not isinstance(ready_proof,dict)
                or ready_proof.get('deployment_uid')!=bound['deployment_uid']
                or ready_proof.get('service_uid')!=bound['service_uid']
                or ready_proof.get('target_sha256')!=gis.digest(bound['target'])
                or not isinstance(ready_proof.get('endpoints'),list) or not ready_proof['endpoints']):
            raise gis.Blocked('gis_existing_successor_binding_changed')
        actual=copy.deepcopy(section['ready_pair']['deployment']);wanted=copy.deepcopy(bound['target'])
        revision='deployment.kubernetes.io/revision'
        actual_revision=actual['metadata']['annotations'].pop(revision,None)
        wanted_revision=wanted['metadata']['annotations'].pop(revision,None)
        if (actual!=wanted or actual_revision!=wanted_revision and
                (not isinstance(actual_revision,str) or not re.fullmatch('[1-9][0-9]{0,18}',actual_revision)
                 or wanted_revision is not None and (not isinstance(wanted_revision,str)
                     or not re.fullmatch('[1-9][0-9]{0,18}',wanted_revision)
                     or int(actual_revision)<int(wanted_revision)))):
            raise gis.Blocked('gis_existing_successor_binding_changed')
        chain.append((receipt,section,state))
        ref=receipt['predecessor']
        if ref==anchor:break
    else:raise gis.Blocked('gis_existing_successor_chain_too_long')
    prior=None
    for receipt,section,state in reversed(chain):
        old,ready=section['old_pair'],section['ready_pair']
        if (set(old)!={'deployment_uid','service_uid','deployment','service'} or set(ready)!=set(old)
                or old['deployment_uid']!=original_receipt['ready']['deployment_uid']
                or old['service_uid']!=original_receipt['ready']['service_uid']
                or ready['deployment_uid']!=old['deployment_uid'] or ready['service_uid']!=old['service_uid']
                or receipt['deployment_uid']!=old['deployment_uid'] or receipt['service_uid']!=old['service_uid']):
            raise gis.Blocked('gis_existing_successor_identity_changed')
        if prior is None:
            if (gis.digest(old['deployment'])!=original_receipt['ready']['deployment_spec_sha256']
                    or gis.digest(old['service'])!=original_receipt['ready']['service_spec_sha256']):
                raise gis.Blocked('gis_existing_genesis_pair_changed')
        elif receipt['old_pair_sha256']!=prior:
            raise gis.Blocked('gis_existing_successor_predecessor_changed')
        prior=receipt['ready_pair_sha256']
    receipt,section,_=chain[0]
    result=seal({'schema':'voice-gis-existing-update-v1','anchor_sha256':anchor['receipt_sha256'],
        'namespace_uid':original_receipt['namespace_uid'],'operation':anchor['operation'],
        'service_uid':receipt['service_uid'],'deployment_uid':receipt['deployment_uid'],
        'service':copy.deepcopy(section['ready_pair']['service']),
        'old':copy.deepcopy(section['ready_pair']['deployment']),'target':None,'target_service':None})
    verify_phase(result,kube,'active')
    return result


def seal(value):
    result = copy.deepcopy(value)
    result.pop('sha256', None)
    result['sha256'] = gis.digest(result)
    return result


def validate_source_authority(authority, target):
    """Validate the root-captured ordinary canonical source authority shape."""
    import source_authority
    required={'schema','repository','source_sha','workflow_id','run_id','run_attempt','tree_sha',
              'source_files','images','lock_artifact_id','lock_sha256','image_provenance','verified'}
    if (not isinstance(authority,dict) or set(authority)!=required
        or authority.get('schema')!='voice-source-authority-v1'
        or authority.get('repository')!=source_authority.REPO
        or authority.get('workflow_id')!=source_authority.WORKFLOW
        or authority.get('verified') is not True or authority.get('source_sha')!=target.get('tag')
        or authority.get('image_provenance')!='independently resolved trusted CI tag'
        or type(authority.get('run_id')) is not int or authority['run_id']<=0
        or type(authority.get('run_attempt')) is not int or authority['run_attempt']<=0
        or type(authority.get('lock_artifact_id')) is not int or authority['lock_artifact_id']<=0
        or not re.fullmatch(r'[a-f0-9]{40}',str(authority.get('tree_sha','')))
        or not re.fullmatch(r'[a-f0-9]{64}',str(authority.get('lock_sha256','')))
        or not isinstance(authority.get('source_files'),dict)
        or not isinstance(authority.get('images'),dict)
        or set(authority['images'])!=set(source_authority.IMAGE_NAMES)
        or any(not isinstance(image,str) or not re.fullmatch(re.escape(source_authority.REGISTRY+'/'+name)+r'@sha256:[a-f0-9]{64}',image)
               for name,image in authority['images'].items())):
        raise gis.Blocked('gis_existing_source_authority_invalid')
    selected=target.get('images',{}).get('voice-gameintegration/gameintegration')
    if selected is not None and selected!=authority['images']['gameintegration']:
        raise gis.Blocked('gis_existing_source_image_changed')


def validate(binding):
    fields = {'schema', 'anchor_sha256', 'namespace_uid', 'operation', 'service_uid',
              'deployment_uid', 'service', 'old', 'target', 'target_service', 'sha256'}
    uid = r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}'
    if (type(binding) is not dict or set(binding) != fields
            or binding.get('schema') != 'voice-gis-existing-update-v1'
            or any(not isinstance(binding.get(key), str) or not re.fullmatch(uid, binding[key])
                   for key in ('namespace_uid', 'service_uid', 'deployment_uid'))
            or not isinstance(binding.get('operation'), str) or not re.fullmatch('[a-f0-9]{12}', binding['operation'])
            or not isinstance(binding.get('anchor_sha256'), str) or not re.fullmatch('[a-f0-9]{64}', binding['anchor_sha256'])
            or not isinstance(binding.get('service'), dict) or not isinstance(binding.get('old'), dict)
            or binding.get('target') is not None and not isinstance(binding['target'], dict)
            or binding.get('target_service') is not None and not isinstance(binding['target_service'], dict)
            or binding.get('sha256') != gis.digest({key: value for key, value in binding.items() if key != 'sha256'})):
        raise gis.Blocked('gis_existing_binding_invalid')


def enroll_first_install(receipt, state, kube):
    """Caller must load receipt/state from the root-owned active reference."""
    gis.validate_next_operation(receipt, state, kube)
    pair = gis._read_pair(kube, None)
    service, deployment = pair[gis.SERVICE], pair[gis.DEPLOYMENT]
    namespace = kube.get('namespace', gis.NS)
    if (namespace.get('metadata', {}).get('deletionTimestamp')
            or namespace.get('metadata', {}).get('uid') != receipt['namespace_uid']):
        raise gis.Blocked('gis_existing_identity_changed')
    for key, row in (('Service/voice-gameintegration', service),
                     ('Deployment/voice-gameintegration', deployment)):
        expected = receipt['resource_receipt']['objects'][key]
        if (row is None or row.get('metadata', {}).get('deletionTimestamp')
                or row.get('metadata', {}).get('uid') != expected['uid']
                or gis.digest(gis.semantic(row)) != expected['spec_sha256']
                or row.get('metadata', {}).get('annotations', {}).get(gis.MARKER) != receipt['operation']):
            raise gis.Blocked('gis_existing_anchor_changed')
    return seal({'schema': 'voice-gis-existing-update-v1',
                 'anchor_sha256': receipt['receipt_sha256'],
                 'namespace_uid': receipt['namespace_uid'],
                 'operation': receipt['operation'],
                 'service_uid': service['metadata']['uid'],
                 'deployment_uid': deployment['metadata']['uid'],
                 'service': gis.semantic(service), 'old': gis.semantic(deployment),
                 'target': None, 'target_service': None})


def bind_target(binding, target):
    validate(binding)
    expected = gis.semantic(target)
    if (target.get('metadata', {}).get('uid') != binding['deployment_uid']
            or expected['metadata'] != binding['old']['metadata']
            or expected['spec'].get('replicas', 1) != 1):
        raise gis.Blocked('gis_existing_target_invalid')
    candidate = copy.deepcopy(expected)
    old = binding['old']
    current = candidate['spec']['template']['spec']['containers']
    prior = old['spec']['template']['spec']['containers']
    if len(current) != len(prior):
        raise gis.Blocked('gis_existing_target_invalid')
    selected = [i for i, container in enumerate(current) if container.get('name') == 'gameintegration']
    if (len(selected) != 1 or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/gameintegration@sha256:[a-f0-9]{64}',
                                                current[selected[0]].get('image', ''))):
        raise gis.Blocked('gis_existing_target_invalid')
    current[selected[0]]['image'] = prior[selected[0]]['image']
    if candidate != old:
        raise gis.Blocked('gis_existing_target_invalid')
    value = copy.deepcopy(binding)
    value['target'] = expected
    value['target_service'] = copy.deepcopy(binding['service'])
    return seal(value)


def bind_normalized_target(binding, deployment, service, selected, mode, expected_hash):
    """Bind the root-normalized, TT-target-hashed pair, never caller documents."""
    validate(binding)
    if (type(selected) is not bool or mode not in ('images-only', 'full', 'app-only')
            or not isinstance(expected_hash, str) or not re.fullmatch('[a-f0-9]{64}', expected_hash)
            or not isinstance(deployment, dict) or not isinstance(service, dict)):
        raise gis.Blocked('gis_existing_target_invalid')
    pair = {'deployment_mode': 'selected-paused' if selected else 'preserved-active',
            'deployment_uid': deployment.get('metadata', {}).get('uid'),
            'service_uid': service.get('metadata', {}).get('uid'),
            'deployment': gis.semantic(deployment), 'service': gis.semantic(service)}
    if (gis.digest(pair) != expected_hash or pair['deployment_uid'] != binding['deployment_uid']
            or pair['service_uid'] != binding['service_uid']
            or gis._identity(deployment) != gis.DEPLOYMENT or gis._identity(service) != gis.SERVICE
            or any(row.get('metadata', {}).get('namespace') != gis.NS
                   or row.get('metadata', {}).get('annotations', {}).get(gis.MARKER) != binding['operation']
                   or row.get('metadata', {}).get('deletionTimestamp') for row in (deployment, service))
            or deployment.get('spec', {}).get('replicas', 1) != (0 if selected else 1)):
        raise gis.Blocked('gis_existing_target_invalid')
    ready = copy.deepcopy(pair['deployment'])
    ready['spec']['replicas'] = 1
    if mode == 'images-only':
        if pair['service'] != binding['service']:
            raise gis.Blocked('gis_existing_target_invalid')
        candidate = copy.deepcopy(ready)
        old = copy.deepcopy(binding['old'])
        managed = 'kubectl.kubernetes.io/last-applied-configuration'
        candidate['metadata']['annotations'].pop(managed, None)
        old['metadata']['annotations'].pop(managed, None)
        current = candidate['spec']['template']['spec']['containers']
        prior = old['spec']['template']['spec']['containers']
        indexes = [i for i, c in enumerate(current) if c.get('name') == 'gameintegration']
        if len(current) != len(prior) or len(indexes) != 1:
            raise gis.Blocked('gis_existing_target_invalid')
        index = indexes[0]
        if not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/gameintegration@sha256:[a-f0-9]{64}', current[index].get('image', '')):
            raise gis.Blocked('gis_existing_target_invalid')
        current[index]['image'] = prior[index]['image']
        if candidate != old or not selected and ready != binding['old']:
            raise gis.Blocked('gis_existing_target_invalid')
    value = copy.deepcopy(binding)
    value['target'] = ready
    value['target_service'] = pair['service']
    return seal(value)


def verify_phase(binding, kube, phase, *, pair=None):
    validate(binding)
    if phase not in ('active', 'paused-old', 'paused-target', 'ready-target'):
        raise gis.Blocked('gis_existing_phase_invalid')
    namespace = kube.get('namespace', gis.NS)
    service = gis._read_one(kube, *gis.SERVICE) if pair is None else pair[gis.SERVICE]
    deployment = gis._read_one(kube, *gis.DEPLOYMENT) if pair is None else pair[gis.DEPLOYMENT]
    if (namespace['metadata']['uid'] != binding['namespace_uid']
            or any(row.get('metadata', {}).get('deletionTimestamp') for row in (namespace, service, deployment))
            or service['metadata']['uid'] != binding['service_uid']
            or deployment['metadata']['uid'] != binding['deployment_uid']
            or gis.semantic(service) != (binding['target_service'] if phase in ('paused-target','ready-target') else binding['service'])):
        raise gis.Blocked('gis_existing_identity_changed')
    expected = copy.deepcopy(binding['target'] if phase in ('paused-target', 'ready-target') else binding['old'])
    if not isinstance(expected, dict):
        raise gis.Blocked('gis_existing_target_invalid')
    if phase in ('paused-old', 'paused-target'):
        expected['spec']['replicas'] = 0
    actual = gis.semantic(deployment)
    if actual != expected:
        # The Deployment controller alone advances this one annotation after
        # creating the owned ReplicaSet; never discard other metadata drift.
        if phase not in ('paused-target', 'ready-target'):
            raise gis.Blocked('gis_existing_deployment_changed')
        revision_key = 'deployment.kubernetes.io/revision'
        actual_copy, expected_copy = copy.deepcopy(actual), copy.deepcopy(expected)
        actual_revision = actual_copy['metadata']['annotations'].pop(revision_key, None)
        previous_revision = expected_copy['metadata']['annotations'].pop(revision_key, None)
        if (actual_copy != expected_copy or not isinstance(actual_revision, str)
                or not re.fullmatch(r'[1-9][0-9]{0,18}', actual_revision)
                or previous_revision is not None and (not isinstance(previous_revision, str)
                    or not re.fullmatch(r'[1-9][0-9]{0,18}', previous_revision)
                    or int(actual_revision) < int(previous_revision))):
            raise gis.Blocked('gis_existing_deployment_changed')
        rows = kube.run(['get', 'replicasets', '-o', 'json']).get('items')
        if not isinstance(rows, list) or len(rows) > 2048:
            raise gis.Blocked('gis_existing_ready_inventory_invalid')
        from runtime_stage import revision_template
        matching = [row for row in rows if row.get('metadata', {}).get('annotations', {}).get(revision_key) == actual_revision
                    and not row.get('metadata', {}).get('deletionTimestamp')
                    and any(owner.get('kind') == 'Deployment' and owner.get('uid') == binding['deployment_uid']
                            and owner.get('controller') is True for owner in row.get('metadata', {}).get('ownerReferences', []))
                    and revision_template(row.get('spec', {}).get('template', {})) == revision_template(expected['spec']['template'])]
        if len(matching) != 1:
            raise gis.Blocked('gis_existing_revision_unproved')
    return True


def service_account_preflight(kube, template):
    """Reject unsupported effective Pod admission before any fence."""
    if not isinstance(template, dict) or (template.get('automountServiceAccountToken') is not None
                                          and template['automountServiceAccountToken'] is not True):
        raise gis.Blocked('gis_existing_service_account_unsupported')
    name = template.get('serviceAccountName') or 'default'
    if not isinstance(name, str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]{0,251}[a-z0-9])?', name):
        raise gis.Blocked('gis_existing_service_account_invalid')
    account = kube.get('serviceaccount', name)
    metadata = account.get('metadata', {}) if isinstance(account, dict) else {}
    uid = metadata.get('uid')
    rv = metadata.get('resourceVersion')
    if (metadata.get('name') != name or metadata.get('namespace') != gis.NS
            or metadata.get('deletionTimestamp') or not isinstance(uid, str)
            or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', uid)
            or not isinstance(rv, str) or not re.fullmatch('[0-9]+', rv)
            or account.get('automountServiceAccountToken') is not None and account['automountServiceAccountToken'] is not True
            or not template.get('imagePullSecrets') and account.get('imagePullSecrets')):
        raise gis.Blocked('gis_existing_service_account_unsupported')
    return {'name': name, 'uid': uid, 'resourceVersion': rv}


def verify_pod_spec(template, pod):
    """Allow only the standard ServiceAccount injection and fixed Pod defaults."""
    wanted = copy.deepcopy(template)
    actual = copy.deepcopy(pod)
    if not isinstance(wanted, dict) or not isinstance(actual, dict):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    volumes = actual.get('volumes', [])
    expected_volumes = wanted.get('volumes', [])
    if not isinstance(volumes, list) or not isinstance(expected_volumes, list):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    extra = [v for v in volumes if v not in expected_volumes]
    if len(extra) != 1 or not isinstance(extra[0], dict):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    volume = extra[0]
    name = volume.get('name', '')
    if not isinstance(name, str) or not re.fullmatch(r'kube-api-access-[a-z0-9]{5}', name):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    projected = volume.get('projected', {})
    sources = projected.get('sources') if isinstance(projected, dict) else None
    if (set(volume) != {'name', 'projected'} or not isinstance(projected, dict)
            or set(projected) != {'defaultMode', 'sources'}
            or projected['defaultMode'] != 420 or not isinstance(sources, list) or len(sources) != 3):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    token = sources[0].get('serviceAccountToken', {}) if isinstance(sources[0], dict) else {}
    if (not isinstance(sources[0], dict) or set(sources[0]) != {'serviceAccountToken'}
            or not isinstance(token, dict) or set(token) != {'path', 'expirationSeconds'}
            or token['path'] != 'token' or type(token['expirationSeconds']) is not int
            or not 600 <= token['expirationSeconds'] <= 86400
            or sources[1] != {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}}
            or sources[2] != {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}):
        raise gis.Blocked('gis_existing_ready_pod_changed')
    volumes.remove(volume)
    if volumes != expected_volumes:
        raise gis.Blocked('gis_existing_ready_pod_changed')
    if 'volumes' not in wanted:
        actual.pop('volumes', None)
    for field in ('containers', 'initContainers'):
        rows = actual.get(field, [])
        if not isinstance(rows, list): raise gis.Blocked('gis_existing_ready_pod_changed')
        for row in rows:
            mounts = row.get('volumeMounts', [])
            mount = {'name': name, 'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True}
            if not isinstance(mounts, list) or mounts.count(mount) != 1:
                raise gis.Blocked('gis_existing_ready_pod_changed')
            mounts.remove(mount)
            if not mounts: row.pop('volumeMounts', None)
    defaults = {'serviceAccountName': wanted.get('serviceAccountName') or 'default',
                'serviceAccount': wanted.get('serviceAccountName') or 'default',
                'schedulerName': 'default-scheduler', 'restartPolicy': 'Always',
                'dnsPolicy': 'ClusterFirst', 'enableServiceLinks': True,
                'priority': 0, 'preemptionPolicy': 'PreemptLowerPriority',
                'terminationGracePeriodSeconds': 30, 'hostNetwork': False,
                'hostPID': False, 'hostIPC': False, 'securityContext': {},
                'tolerations': [
                    {'key': 'node.kubernetes.io/not-ready', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300},
                    {'key': 'node.kubernetes.io/unreachable', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300}]}
    for key in set(actual) - set(wanted):
        if key == 'nodeName' and isinstance(actual[key], str) and re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', actual[key]):
            continue
        if key not in defaults or type(actual[key]) is not type(defaults[key]) or actual[key] != defaults[key]:
            raise gis.Blocked('gis_existing_ready_pod_changed')
    if any(actual.get(key) != value for key, value in wanted.items()):
        raise gis.Blocked('gis_existing_ready_pod_changed')


def verify_ready(binding, kube, service_account):
    """Prove the endpoint belongs to the captured UID and exact target image."""
    verify_phase(binding, kube, 'ready-target')
    if service_account_preflight(kube, binding['target']['spec']['template']['spec']) != service_account:
        raise gis.Blocked('gis_existing_service_account_changed')
    pair = gis._read_pair(kube, None)
    service, deployment = pair[gis.SERVICE], pair[gis.DEPLOYMENT]
    status = deployment.get('status', {})
    generation = deployment.get('metadata', {}).get('generation')
    if (type(generation) is not int or status.get('observedGeneration', 0) < generation
            or any(status.get(key) != 1 for key in ('readyReplicas', 'updatedReplicas', 'availableReplicas'))):
        raise gis.Blocked('gis_existing_not_ready')
    endpoints = gis._ready_service_endpoints(kube, service, binding['deployment_uid'])
    if not endpoints:
        raise gis.Blocked('gis_existing_endpoint_missing')
    slices = kube.run(['get', 'endpointslices', '-l',
                       'kubernetes.io/service-name=voice-gameintegration', '-o', 'json']).get('items')
    replicasets = kube.run(['get', 'replicasets', '-o', 'json']).get('items')
    if not isinstance(slices, list) or len(slices) > 64 or not isinstance(replicasets, list) or len(replicasets) > 2048:
        raise gis.Blocked('gis_existing_ready_inventory_invalid')
    from runtime_stage import revision_template
    template = binding['target']['spec']['template']
    observed = []
    for endpoint in endpoints:
        matches = [rs for rs in replicasets if rs.get('metadata', {}).get('uid') == endpoint['replicaset_uid']]
        if (len(matches) != 1 or not any(owner.get('kind') == 'Deployment' and owner.get('uid') == binding['deployment_uid']
                and owner.get('controller') is True for owner in matches[0].get('metadata', {}).get('ownerReferences', []))
                or revision_template(matches[0].get('spec', {}).get('template', {})) != revision_template(template)):
            raise gis.Blocked('gis_existing_ready_revision_changed')
        refs = [item.get('targetRef', {}) for row in slices if row.get('metadata', {}).get('uid') == endpoint['slice_uid']
                for item in row.get('endpoints', []) if item.get('conditions', {}).get('ready') is True
                and item.get('targetRef', {}).get('uid') == endpoint['pod_uid']]
        if len(refs) != 1:
            raise gis.Blocked('gis_existing_ready_endpoint_changed')
        pod = kube.get('pod', refs[0]['name'])
        if (pod.get('metadata', {}).get('uid') != endpoint['pod_uid']
                or pod.get('metadata', {}).get('deletionTimestamp')
                or pod.get('status', {}).get('phase') != 'Running'
                or not any(owner.get('kind') == 'ReplicaSet' and owner.get('uid') == endpoint['replicaset_uid']
                           and owner.get('controller') is True for owner in pod.get('metadata', {}).get('ownerReferences', []))):
            raise gis.Blocked('gis_existing_ready_pod_changed')
        slice_rows = [row for row in slices if row.get('metadata', {}).get('uid') == endpoint['slice_uid']]
        versions = [pod.get('metadata', {}).get('resourceVersion'), matches[0].get('metadata', {}).get('resourceVersion'),
                    slice_rows[0].get('metadata', {}).get('resourceVersion') if len(slice_rows) == 1 else None]
        if any(not isinstance(value, str) or not re.fullmatch(r'[0-9]+', value) for value in versions):
            raise gis.Blocked('gis_existing_ready_version_missing')
        observed.append((refs[0]['name'], endpoint, versions))
        pod_spec = pod.get('spec', {})
        verify_pod_spec(template['spec'], pod_spec)
        for spec_key, status_key in (('containers', 'containerStatuses'), ('initContainers', 'initContainerStatuses')):
            expected = template['spec'].get(spec_key, [])
            actual = pod.get('spec', {}).get(spec_key, [])
            states = pod.get('status', {}).get(status_key, [])
            if ({c.get('name'): c.get('image') for c in actual} != {c.get('name'): c.get('image') for c in expected}
                    or len(actual) != len(expected) or len(states) != len(expected)):
                raise gis.Blocked('gis_existing_ready_pod_changed')
            for container in expected:
                matched = [row for row in states if row.get('name') == container['name']]
                if (len(matched) != 1 or spec_key == 'containers' and matched[0].get('ready') is not True
                        or matched[0].get('imageID', '').removeprefix('docker-pullable://') != container['image']):
                    raise gis.Blocked('gis_existing_ready_image_changed')
    verify_phase(binding, kube, 'ready-target')
    if service_account_preflight(kube, binding['target']['spec']['template']['spec']) != service_account:
        raise gis.Blocked('gis_existing_service_account_changed')
    if gis._ready_service_endpoints(kube, service, binding['deployment_uid']) != endpoints:
        raise gis.Blocked('gis_existing_ready_endpoint_changed')
    current_slices = kube.run(['get', 'endpointslices', '-l',
                               'kubernetes.io/service-name=voice-gameintegration', '-o', 'json']).get('items')
    current_rs = kube.run(['get', 'replicasets', '-o', 'json']).get('items')
    if not isinstance(current_slices, list) or len(current_slices) > 64 or not isinstance(current_rs, list) or len(current_rs) > 2048:
        raise gis.Blocked('gis_existing_ready_inventory_invalid')
    for pod_name, endpoint, versions in observed:
        pod = kube.get('pod', pod_name)
        rs = [row for row in current_rs if row.get('metadata', {}).get('uid') == endpoint['replicaset_uid']]
        slices = [row for row in current_slices if row.get('metadata', {}).get('uid') == endpoint['slice_uid']]
        if (pod.get('metadata', {}).get('uid') != endpoint['pod_uid'] or len(rs) != 1 or len(slices) != 1
                or [pod.get('metadata', {}).get('resourceVersion'), rs[0]['metadata'].get('resourceVersion'),
                    slices[0]['metadata'].get('resourceVersion')] != versions):
            raise gis.Blocked('gis_existing_ready_observation_changed')
    return {'deployment_uid': binding['deployment_uid'], 'service_uid': binding['service_uid'],
            'target_sha256': gis.digest(binding['target']), 'endpoints': endpoints}


def successor_evidence(base, state, stage=None):
    """Bind a successor to the current root-owned cold-release evidence."""
    from pathlib import Path
    from preserve import ColdRelease
    from native_store import verify_archive
    from controller import file_sha
    import resource_claim

    try:
        cut = state['cut']
        effective = state.get('nats_migration', {}).get('cut', cut)
        if stage is not None:
            runtime = stage.closed_preservation_runtime
            if (type(runtime) is not ColdRelease or runtime.stage is not stage
                    or runtime.state is not state or runtime.base != Path(base) or runtime.cut != effective
                    or getattr(stage, 'closed_preservation_release', None) != runtime.release_guard):
                raise gis.Blocked('gis_existing_cold_release_invalid')
            runtime.guard()
            intent = runtime.intent
        else:
            if (state.get('status') != 'PASS' or state.get('phase') != 'POST_RESTART'
                    or state.get('fence_status') != 'RELEASED'):
                raise gis.Blocked('gis_existing_successor_not_complete')
            intent = state['gis_existing_update']['preservation']['cold_intent']
        resource_claim.historical_binding(state)
        binding, custody = state['cipher_binding'], state['custody']
        restore = private_json(Path(base) / 'decrypted-restore.json')
        authorization, target = state['authorization'], state['target']
        proof, fence = state['preservation'], state['resource_fence']
        keys = ('operation', 'challenge', 'run_id', 'head_sha', 'cipher_sha256', 'cipher_bytes')
        backup = {'archive_sha256': cut['manifest']['archive_sha256'],
                  'manifest_sha256': cut['manifest_sha256'],
                  'census_sha256': cut['census_sha256'], 'off_node_verified': True}
        if (custody.get('schema') != 'voice-nats-custody-v1'
                or custody.get('destination') != 'github:Poryadok/VoiceRoot'
                or custody.get('verified') is not True
                or any(key not in binding or custody.get(key) != binding[key] for key in keys)
                or binding.get('operation') != state['operation']
                or restore.get('schema') != 'voice-nats-decrypted-restore-v1'
                or restore.get('cipher_sha256') != binding['cipher_sha256']
                or restore.get('cipher_bytes') != binding['cipher_bytes']
                or restore.get('decrypted_members_verified') is not True
                or restore.get('readback', {}).get('native_archive_verified') is not True
                or restore.get('readback', {}).get('census_sha256') != cut['census_sha256']
                or restore.get('members', {}).get('rollout-before.tar') != backup['archive_sha256']
                or restore.get('members', {}).get('rollout-before-manifest.json') != backup['manifest_sha256']
                or gis.digest(cut['census']) != cut['census_sha256']
                or authorization.get('operation') != state['operation']
                or authorization.get('target') != target or authorization.get('backup') != backup
                or authorization.get('namespace_uid') != state['gis_binding']['namespace_uid']
                or state['context']['expected']['namespace_uid'] != state['gis_binding']['namespace_uid']
                or file_sha(Path(base) / 'rollout-before-manifest.json') != backup['manifest_sha256']
                or not verify_archive(Path(base) / 'rollout-before.tar', cut['manifest'])
                or proof.get('native_files_verified') is not True or proof.get('census_verified') is not True
                or proof.get('census_sha256') != effective['census_sha256']
                or gis.digest(effective['census']) != effective['census_sha256']
                or fence.get('target_sha256') != gis.digest(target)
                or target.get('gis_existing_normalized_sha256') != gis.digest(state['gis_normalized_pair'])
                or not isinstance(intent, dict) or intent not in state['events']
                or set(intent)!={'kind','operation','cut_sha256','native_proof_sha256','complete_durable_sha256',
                                'decrypted_restore_sha256','cipher_sha256','custody_sha256',
                                'selected_hub_stopped','application_ack_observed'}
                or intent.get('kind') != 'cold_preservation_verified_release_intent'
                or intent.get('operation') != state['operation']
                or any(not isinstance(intent.get(key),str) or not re.fullmatch('[a-f0-9]{64}',intent[key])
                       for key in ('native_proof_sha256','complete_durable_sha256'))
                or intent.get('cut_sha256') != gis.digest(effective)
                or intent.get('decrypted_restore_sha256') != gis.digest(restore)
                or intent.get('cipher_sha256') != binding['cipher_sha256']
                or intent.get('custody_sha256') != gis.digest(custody)
                or intent.get('selected_hub_stopped') is not True
                or intent.get('application_ack_observed') is not False):
            raise gis.Blocked('gis_existing_successor_evidence_invalid')
        if stage is not None:runtime.guard()
        return {'custody': copy.deepcopy(custody), 'preservation': copy.deepcopy(proof),
                'cold_intent': copy.deepcopy(intent), 'restore': copy.deepcopy(restore)}
    except (KeyError, TypeError, AttributeError, FileNotFoundError):
        raise gis.Blocked('gis_existing_successor_evidence_invalid') from None
