"""Fixed development-profile transitions for drained User/Notification."""
import copy
import re
import secrets
import time

import backend_config_transition_probe as probe
import full_cohort_schema_metadata as metadata
import media_operation_guard
import writer_cut

SERVICES = ('notification','user')
POLICY = ('VOICE_RELEASE_PROFILE','VOICE_RELEASE_POLICY_FILE','VOICE_RELEASE_POLICY_SHA256','VOICE_RELEASE_SNAPSHOT_FILE')
BASE = probe.BASE


class DeploymentError(RuntimeError): pass


def _plan(cut, journal, service):
    intent = journal.intent; context = cut.fence.context
    plan = intent['backend_config_deployments']; observation = plan['observation']; prior = plan['observation_plan']
    if (journal.poisoned or journal.state.get('intent') != intent or intent.get('resource_context') != context
            or intent.get('namespace_uid') != cut.namespace_uid or set(plan) != {'source_sha','cut_invocation','services','sources','observation','observation_plan'}
            or plan['source_sha'] != context['commit'] or plan['cut_invocation'] != cut.invocation
            or not isinstance(plan['services'],list) or not plan['services']
            or plan['services'] != sorted(set(plan['services'])) or any(s not in SERVICES for s in plan['services'])
            or service not in plan['services'] or set(plan['sources']) != set(plan['services'])
            or not isinstance(observation,dict) or set(observation) != {'operation_id','source_sha','nonce','observations'}
            or observation['operation_id'] != context['operation_id'] or observation['source_sha'] != context['commit']
            or not isinstance(observation['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',observation['nonce'])
            or not isinstance(prior,dict) or set(prior) != {'source_sha','targets'} or prior['source_sha'] != context['commit']
            or not isinstance(prior['targets'],list) or [x.get('workload') for x in prior['targets']] != list(SERVICES)
            or not isinstance(observation['observations'],dict) or set(observation['observations']) != set(SERVICES)): raise ValueError()
    row, _ = cut.fence.ledger.read(); saved = row.get('backend_config_observation')
    wanted = {'status':'done','nonce':observation['nonce'],'operation_id':context['operation_id'],
              'source_sha':context['commit'],'targets_sha256':writer_cut.digest(prior['targets']),
              'receipt_sha256':writer_cut.digest(observation)}
    if not cut.fence.owned(row) or row.get('status') != 'active' or saved != wanted: raise ValueError()
    bound = cut.journal.state['steps']['writer-cut-config-plan']
    expected_details = {'intent':intent,'steps':{},'effects':{},'services':[]}
    expected_receipt = {'operation_id':context['operation_id'],'cut_invocation':cut.invocation,
                        'services':plan['services'],'config_intent_sha256':writer_cut.digest(intent)}
    bound_record = {'status':'done','nonce':bound['nonce'],'details_sha256':writer_cut.digest(expected_details),
                    'receipt_sha256':writer_cut.digest(expected_receipt)}
    if (set(bound) != {'status','nonce','details_sha256','details','receipt'} or bound['status'] != 'done'
            or not isinstance(bound['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',bound['nonce'])
            or bound['details'] != expected_details or bound['receipt'] != expected_receipt
            or bound['details_sha256'] != bound_record['details_sha256']
            or row['writer_cut_effects'].get('writer-cut-config-plan') != bound_record): raise ValueError()
    result = observation['observations'][service]; identity = result['identity']; target = next(x for x in prior['targets'] if x['workload'] == service)
    if (not isinstance(result,dict) or set(result) != {'status','identity','fields','env_sources','scope','template_binding'}
            or result.get('status') != 'observed' or result.get('scope') != 'runtime-config-metadata-only-no-provider-validation'
            or result.get('template_binding') != 'pod-env-and-deployment-template-v1'
            or set(result['fields']) != set(probe.FIELDS[service]) or not isinstance(result.get('env_sources'),list)
            or identity.get('namespace_uid') != cut.namespace_uid
            or any(identity.get(k) != target.get(k) for k in ('pod_name','pod_uid','deployment_uid','container_id','restart_count'))
            or any(result['fields'][name] != {'state':'absent','origin':None} for name in POLICY)): raise ValueError()
    source = copy.deepcopy(plan['sources'][service]); m = source['metadata']; spec = source['spec']
    matches = [x for x in cut.admission['controllers'] if x['kind'] == 'Deployment' and x['metadata']['name'] == 'voice-'+service]
    if len(matches) != 1: raise ValueError()
    original = matches[0]; name = 'writer-drain-deployment-voice-'+service
    drained = cut.journal.state['steps'][name]; saved_cut = row['writer_cut_effects'][name]
    if (drained.get('status') != 'done' or saved_cut.get('status') != 'done'
            or saved_cut.get('receipt_sha256') != writer_cut.digest(drained['receipt'])
            or m.get('uid') != original['metadata']['uid'] or m.get('uid') != identity.get('deployment_uid')
            or original['metadata'].get('resourceVersion') != identity.get('deployment_rv')
            or m.get('namespace') != 'voice-dev' or m.get('name') != 'voice-'+service
            or m.get('generation') != drained['receipt']['generation'] or type(m.get('generation')) is not int
            or not isinstance(m.get('resourceVersion'),str) or not m['resourceVersion']
            or source.get('kind') != 'Deployment' or source.get('apiVersion') != 'apps/v1'
            or writer_cut.digest(source.get('spec')) != writer_cut.digest(drained['details']['target_spec'])
            or type(spec.get('replicas')) is not int or spec['replicas'] != 0): raise ValueError()
    target_spec = copy.deepcopy(spec)
    containers = [c for c in target_spec['template']['spec']['containers'] if c.get('name') == service]
    if len(containers) != 1: raise ValueError()
    env = containers[0].setdefault('env',[])
    if (not isinstance(env,list) or any(not isinstance(e,dict) or not isinstance(e.get('name'),str) for e in env)
            or len({e['name'] for e in env}) != len(env) or any(e['name'] in POLICY for e in env)): raise ValueError()
    env.append({'name':'VOICE_RELEASE_PROFILE','value':'development'})
    return source, target_spec, result


def _refs(cut, result):
    refs = {}
    for value in result['env_sources']:
        if not isinstance(value,dict) or set(value) != {'kind','name','uid','resource_version','prefix','fields'}: raise ValueError()
        key = (value['kind'],value['name']); identity = (value['uid'],value['resource_version'])
        if key in refs and refs[key] != identity: raise ValueError()
        refs[key] = identity
    for field in result['fields'].values():
        origin = field['origin']
        if origin is not None and origin.get('source') != 'literal':
            key = (origin['kind'],origin['name']); identity = (origin['uid'],origin['resource_version'])
            if key in refs and refs[key] != identity: raise ValueError()
            refs[key] = identity
    for (kind,name), (uid,rv) in refs.items():
        if (kind not in ('secret','configmap') or not isinstance(name,str)
                or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',name)): raise ValueError()
        media_operation_guard.verify(cut.fence,cut.client,cut.lock,cut.namespace_uid)
        template = '{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}'
        raw = metadata.linux_private_bytes(BASE+['get',kind,name,'-o','go-template='+template])
        media_operation_guard.verify(cut.fence,cut.client,cut.lock,cut.namespace_uid)
        if raw != (uid+'\n'+rv).encode(): raise ValueError()


def verify_deployment_successor(cut, config_journal, service):
    """Reconstruct a completed fixed effect for WriterCut; no recursive cut check."""
    try:
        media_operation_guard.verify(cut.fence,cut.client,cut.lock,cut.namespace_uid)
        source, target, observation = _plan(cut,config_journal,service); _refs(cut,observation)
        step = 'backend-config-deployment-voice-'+service
        entry = config_journal.state['steps'][step]; row, _ = cut.fence.ledger.read(); record = row['backend_config_effects'][step]
        patch = [{'op':'test','path':'/metadata/uid','value':source['metadata']['uid']},
                 {'op':'test','path':'/metadata/resourceVersion','value':source['metadata']['resourceVersion']},
                 {'op':'test','path':'/spec','value':source['spec']}, {'op':'replace','path':'/spec','value':target}]
        details = {'operation_id':cut.fence.context['operation_id'],'invocation_nonce':cut.invocation,
                   'source':source,'target_spec':target,'patch':patch}
        receipt = entry['receipt']
        if (set(entry) != {'status','nonce','details_sha256','details','receipt'} or entry['status'] != 'done'
                or not isinstance(entry['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',entry['nonce'])
                or entry['details'] != details or entry['details_sha256'] != writer_cut.digest(details)
                or record != {'status':'done','nonce':entry['nonce'],'details_sha256':entry['details_sha256'],
                              'receipt_sha256':writer_cut.digest(receipt)}
                or set(receipt) != {'uid','spec_sha256','responseResourceVersion','resourceVersion','generation'}
                or receipt['uid'] != source['metadata']['uid'] or receipt['spec_sha256'] != writer_cut.digest(target)
                or type(receipt['generation']) is not int or receipt['generation'] != source['metadata']['generation']+1
                or any(not isinstance(receipt[k],str) or not receipt[k] for k in ('responseResourceVersion','resourceVersion'))): raise ValueError()
        return {'source':source,'target_spec':target,'receipt':copy.deepcopy(receipt),'ledger_record':copy.deepcopy(record)}
    except Exception:
        raise DeploymentError('configuration successor unavailable') from None


def _effect_map(cut, journal, exclude=None):
    result = {}
    for source in (cut.journal,journal):
        for name, entry in source.state['steps'].items():
            if not name.startswith('backend-config-') or name == 'backend-config-observation' or name == exclude: continue
            if (name in result or not isinstance(entry,dict) or entry.get('status') not in ('pending','done')
                    or not isinstance(entry.get('nonce'),str) or not re.fullmatch(r'[0-9a-f]{32}',entry['nonce'])
                    or entry.get('details_sha256') != writer_cut.digest(entry.get('details'))): raise ValueError()
            record = {k:entry[k] for k in ('status','nonce','details_sha256')}
            if entry['status'] == 'done': record['receipt_sha256'] = writer_cut.digest(entry['receipt'])
            result[name] = record
    return result


def apply_development(cut, config_journal, service):
    """Apply exactly one development profile and immediately accept its cut successor."""
    pending = False
    try:
        if type(cut) is not writer_cut.WriterCut or config_journal is cut.journal: raise ValueError()
        cut.verify_drained()
        source, target, observation = _plan(cut,config_journal,service); _refs(cut,observation)
        plan = config_journal.intent['backend_config_deployments']['services']
        previous = cut.config_snapshot['services'] if cut.config_journal is config_journal and isinstance(cut.config_snapshot,dict) else None
        if (not isinstance(previous,list) or previous != plan[:len(previous)] or len(previous) >= len(plan)
                or service != plan[len(previous)]): raise ValueError()
        step = 'backend-config-deployment-voice-'+service
        if step in config_journal.state['steps']: raise ValueError()
        actual = cut.client.call('get','deployment','voice-'+service,'-o','json')
        m = source['metadata']; am = actual['metadata']
        if (writer_cut.digest(actual.get('spec')) != writer_cut.digest(source['spec']) or any(am.get(k) != m.get(k) for k in
                ('uid','resourceVersion','generation','name','namespace')) or am.get('deletionTimestamp')): raise ValueError()
        patch = [{'op':'test','path':'/metadata/uid','value':m['uid']},
                 {'op':'test','path':'/metadata/resourceVersion','value':m['resourceVersion']},
                 {'op':'test','path':'/spec','value':source['spec']}, {'op':'replace','path':'/spec','value':target}]
        details = {'operation_id':cut.fence.context['operation_id'],'invocation_nonce':cut.invocation,
                   'source':source,'target_spec':target,'patch':patch}
        record = {'status':'pending','nonce':secrets.token_hex(16),'details_sha256':writer_cut.digest(details)}
        row, rev = cut.fence.ledger.read()
        if not cut._owned(row) or row.get('backend_config_effects',{}) != _effect_map(cut,config_journal): raise ValueError()
        pending = True; expected = record | {'details':copy.deepcopy(details)}
        config_journal.state['steps'][step] = copy.deepcopy(expected); config_journal.save()
        current, actual_rev = cut.fence.ledger.read()
        if current != row or actual_rev != rev: raise ValueError()
        current.setdefault('backend_config_effects',{})[step] = copy.deepcopy(record); current['step'] = step
        cut.fence.ledger.write(rev,current); cut.verify_drained()
        if config_journal.state['steps'].get(step) != expected: raise ValueError()
        current, _ = cut.fence.ledger.read()
        if current.get('backend_config_effects',{}) != _effect_map(cut,config_journal): raise ValueError()
        response = cut._patch('Deployment','voice-'+service,patch)
        def check_item(item):
            meta = item['metadata']
            if (item.get('apiVersion') != 'apps/v1' or item.get('kind') != 'Deployment' or writer_cut.digest(item.get('spec')) != writer_cut.digest(target)
                    or meta.get('uid') != m['uid'] or meta.get('name') != m['name'] or meta.get('namespace') != 'voice-dev'
                    or meta.get('deletionTimestamp') or type(meta.get('generation')) is not int
                    or meta['generation'] != m['generation']+1 or not isinstance(meta.get('resourceVersion'),str)
                    or not meta['resourceVersion']): raise ValueError()
        check_item(response)
        deadline = time.monotonic()+60
        while True:
            media_operation_guard.verify(cut.fence,cut.client,cut.lock,cut.namespace_uid)
            cut._guards()
            if not cut._pods_drained(): raise ValueError()
            _refs(cut,observation)
            live, _ = cut.fence.ledger.read()
            if (not cut._owned(live) or live.get('backend_config_effects',{}) != _effect_map(cut,config_journal)
                    or config_journal.state['steps'].get(step) != expected): raise ValueError()
            observed = cut.client.call('get','deployment','voice-'+service,'-o','json'); check_item(observed)
            status = observed.get('status',{}); generation = status.get('observedGeneration')
            if (type(generation) is int and generation >= m['generation']+1
                    and status.get('replicas',0) == 0 and status.get('readyReplicas',0) == 0): break
            if time.monotonic() >= deadline: raise ValueError()
            time.sleep(min(1, max(0,deadline-time.monotonic())))
        _refs(cut,observation); media_operation_guard.verify(cut.fence,cut.client,cut.lock,cut.namespace_uid)
        current, rev = cut.fence.ledger.read()
        if (not cut._owned(current) or current.get('backend_config_effects',{}) != _effect_map(cut,config_journal)
                or config_journal.state['steps'].get(step) != expected): raise ValueError()
        receipt = {'uid':m['uid'],'spec_sha256':writer_cut.digest(target),
                   'responseResourceVersion':response['metadata']['resourceVersion'],
                   'resourceVersion':observed['metadata']['resourceVersion'],'generation':observed['metadata']['generation']}
        done = record | {'status':'done','receipt_sha256':writer_cut.digest(receipt)}
        config_journal.state['steps'][step] = expected | {'status':'done','receipt':copy.deepcopy(receipt)}
        local_done = copy.deepcopy(config_journal.state['steps'][step]); config_journal.save()
        current['backend_config_effects'][step] = copy.deepcopy(done); current['step'] = step+'-done'
        cut.fence.ledger.write(rev,current)
        media_operation_guard.verify(cut.fence,cut.client,cut.lock,cut.namespace_uid)
        after, _ = cut.fence.ledger.read()
        if (config_journal.state['steps'].get(step) != local_done or not cut._owned(after)
                or after.get('backend_config_effects',{}) != _effect_map(cut,config_journal)): raise ValueError()
        cut.accept_config_successor(config_journal,service); cut.verify_drained()
        return receipt
    except BaseException:
        if pending:
            cut.poisoned = True
            try: cut.fence.uncertain('backend deployment configuration outcome uncertain')
            except BaseException: pass
        raise DeploymentError('configuration uncertain; retain lock and reconcile' if pending else 'configuration not admitted') from None
