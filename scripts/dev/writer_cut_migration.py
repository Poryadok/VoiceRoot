"""Source-bound migration Job observation under an existing writer cut.

The Kubernetes POST authorizes execution under the existing cooperative fence;
the subsequent Job/Pod/session observation is evidence, not an atomic revoke
or cryptographic authentication of a network peer. Unknown outcomes hold.
"""
import copy
import ipaddress

import auth_live_migration_phase as auth
import cohort_backup_phases
import realtime_migration_job as realtime
import writer_cut_pg_probe


def normalized(cut, kind, phase, root):
    if kind == 'auth' and type(phase) is cohort_backup_phases.CohortPhases:
        owner = phase.phases
        module = auth
    elif kind == 'realtime' and type(phase) is realtime.Migration:
        owner = phase
        module = realtime
    elif kind == 'go':
        import full_cohort_go_migration as go
        if type(phase) is not go.Migration or phase.cut is not cut: raise ValueError('migration producer type invalid')
        owner = phase
        module = go
    else:
        raise ValueError('migration producer type invalid')
    namespace = getattr(owner, 'namespace_uid', getattr(owner, 'namespace', None))
    if (owner.fence.context != cut.fence.context or owner.lock != cut.lock or namespace != cut.namespace_uid
            or owner.fence.ledger.client is not owner.client):
        raise ValueError('migration authority differs')
    left, lrev = cut.fence.ledger.read()
    right, rrev = owner.fence.ledger.read()
    if left != right or lrev != rrev or not cut.fence.owned(left):
        raise ValueError('migration ledger differs')
    result = module.verify_writer_successor(phase, root)
    if set(result) != {'configmap', 'job', 'record', 'execution'}:
        raise ValueError('migration successor invalid')
    if kind == 'auth':
        record = result['record']
        job = {'authored': result['job'], **{k: record['job'][k] for k in ('state', 'uid', 'resourceVersion')}}
        config = {'authored': result['configmap'], **{k: record['configmap'][k] for k in ('state', 'uid', 'resourceVersion')}}
        stable = {k: record[k] for k in ('operation_id', 'source_sha', 'nonce', 'snapshot_id', 'input_sha256')}
        terminal = record['terminal_sha256']
    elif kind == 'realtime':
        record = result['record']
        job = result['job']
        config = {'receipt': result['configmap'], 'state': 'owned', 'uid': result['configmap']['uid'],
                  'resourceVersion': result['configmap']['resourceVersion']}
        stable = {k: record[k] for k in ('operation_id', 'source_sha', 'nonce')}
        terminal = record['terminal_sha256']
    else:
        record = result['record']; job = result['job']; config = result['configmap']
        stable = {k: record[k] for k in ('operation_id', 'source_sha', 'nonce')}
        terminal = record['terminal_sha256']
    if stable['operation_id'] != cut.fence.context['operation_id'] or stable['source_sha'] != cut.fence.context['commit']:
        raise ValueError('migration operation differs')
    return module, {'job': copy.deepcopy(job), 'configmap': copy.deepcopy(config), 'stable': stable,
                    'execution': copy.deepcopy(result['execution']), 'terminal': terminal}


def observe(cut, binding, allow_removed=False):
    import writer_cut
    module, state = normalized(cut, binding['kind'], binding['phase'], binding['root'])
    config_plan = state['configmap'].get('authored', state['configmap'].get('receipt'))
    plan = {'authored': state['job']['authored'], 'config': config_plan,
            'execution': state['execution'], 'stable': state['stable']}
    if plan != binding['plan']:
        raise ValueError('migration plan changed')
    job, config, execution = state['job'], state['configmap'], state['execution']
    if ((binding['job_uid'] is not None and job.get('uid') != binding['job_uid']) or
            (binding['config_uid'] is not None and config.get('uid') != binding['config_uid']) or
            (binding.get('terminal') is not None and state['terminal'] != binding['terminal'])):
        raise ValueError('migration receipt identity changed')
    jobs = cut.client.call('get', 'jobs', '-o', 'json')['items']
    status = job['state']
    if status in ('absent', 'pending'):
        uid = None
    elif status in ('owned', 'done', 'removing'):
        uid = job['uid']
    elif status == 'removed' and allow_removed:
        uid = None
    else:
        raise ValueError('migration Job state unavailable')
    writer_cut.migration_job_inventory(cut.admission['guards']['jobs'], jobs, uid)
    if uid:
        actual = next(x for x in jobs if x.get('metadata', {}).get('uid') == uid)
        module.verify_writer_job(job['authored'], actual)
        if actual['metadata']['uid'] != uid:
            raise ValueError('migration Job UID changed')
    else:
        actual = None
        if any(x.get('metadata', {}).get('name') == job['authored']['metadata']['name'] for x in jobs):
            raise ValueError('unowned migration Job present')
    if binding['kind'] in ('auth','go'):
        cmstate = config['state']
        cms = cut.client.call('get', 'configmaps', '-o', 'json')['items']
        matching = [x for x in cms if x.get('metadata', {}).get('name') == config['authored']['metadata']['name']]
        if cmstate in ('owned', 'removing'):
            if len(matching) != 1 or module.verify_writer_configmap(config['authored'], matching[0])['uid'] != config['uid']:
                raise ValueError('migration ConfigMap changed')
            if matching[0]['metadata']['resourceVersion'] != config['resourceVersion']:
                raise ValueError('migration ConfigMap revision changed')
        elif cmstate in ('absent', 'pending') or (cmstate == 'removed' and allow_removed):
            if matching: raise ValueError('unowned migration ConfigMap present')
        else: raise ValueError('migration ConfigMap state unavailable')
    pods = cut.client.call('get', 'pods', '-o', 'json')['items']
    owned = [p for p in pods if any(o.get('controller') is True and o.get('kind') == 'Job' and o.get('uid') == job.get('uid')
                                    for o in p.get('metadata', {}).get('ownerReferences', []))] if job.get('uid') else []
    if len(owned) > 1 or (status == 'removed' and owned):
        raise ValueError('migration Pod set changed')
    pod = module.verify_writer_pod(job['authored'], actual, owned[0], execution) if owned else None
    if actual and state['terminal']:
        status = actual.get('status', {})
        if (status.get('active', 0) or status.get('failed', 0) or not any(
                c.get('type') == 'Complete' and c.get('status') == 'True' for c in status.get('conditions', []))):
            raise ValueError('terminal migration Job changed')
    if pod and state['terminal']:
        status = owned[0].get('status', {})
        containers = status.get('containerStatuses', [])
        if (status.get('phase') != 'Succeeded' or len(containers) != 1 or
                containers[0].get('state', {}).get('terminated', {}).get('exitCode') != 0):
            raise ValueError('terminal migration Pod changed')
    if pod and binding['kind'] == 'realtime':
        pod = {'uid': pod['pod_uid'], 'ip': pod['pod_ip']}
    if binding.get('pod_uid') and (not pod or pod['uid'] != binding['pod_uid']):
        if not (allow_removed and status == 'removed' and pod is None):
            raise ValueError('migration Pod UID changed')
    if pod and pod.get('ip'):
        ipaddress.ip_address(pod['ip'])
        if sum(p.get('status', {}).get('podIP') == pod['ip'] for p in pods) != 1:
            raise ValueError('migration Pod address ambiguous')
    nodes = cut.client.call('get', 'nodes', '-o', 'json')['items']
    matches = [n for n in nodes if n.get('metadata', {}).get('name') == execution['node_name']]
    if (len(matches) != 1 or matches[0].get('metadata', {}).get('uid') != execution['node_uid'] or
            matches[0].get('metadata', {}).get('deletionTimestamp') or
            not any(c.get('type') == 'Ready' and c.get('status') == 'True'
                    for c in matches[0].get('status', {}).get('conditions', []))):
        raise ValueError('migration node changed')
    return state, pod


def sessions(cut, state, pod):
    import full_cohort_affected_plan
    execution = state['execution']
    identity = {k: execution[k] for k in ('database', 'database_oid', 'role', 'application_name')}
    identity.update(pod_uid=pod['uid'] if pod and pod.get('ip') else None, pod_ip=pod['ip'] if pod else None)
    before = full_cohort_affected_plan.verify_migration_catalog(cut.probe, cut)
    scope = copy.deepcopy(before['scope'])
    receipt = writer_cut_pg_probe.inspect_migration(scope, identity, cut._guard)
    after = full_cohort_affected_plan.verify_migration_catalog(cut.probe, cut)
    if before != after or receipt.get('scope') != scope:
        raise ValueError('migration database catalog changed during observation')
    if (type(receipt['active_sessions']) is not int or type(receipt['active_transactions']) is not int or
            receipt['active_sessions'] != 0 or receipt['active_transactions'] != 0):
        raise ValueError('other sessions or prepared transactions remain')
    return receipt


def verify_cleaned(cut, binding):
    module, state = normalized(cut, binding['kind'], binding['phase'], binding['root'])
    module.verify_writer_cleanup(binding['phase'], binding['root'])
    if (state['job']['state'] != 'removed' or state['configmap']['state'] != 'removed' or
            state['job']['uid'] != binding['job_uid'] or state['configmap']['uid'] != binding['config_uid'] or
            state['terminal'] != binding['terminal']):
        raise ValueError('cleaned migration receipt changed')
    for resource, item in (('jobs', state['job']), ('configmaps', state['configmap'])):
        name = item['authored']['metadata']['name']
        rows = cut.client.call('get', resource, '-o', 'json')['items']
        if any(x.get('metadata', {}).get('name') == name or x.get('metadata', {}).get('uid') == item['uid'] for x in rows):
            raise ValueError('cleaned migration object present')
    pods = cut.client.call('get', 'pods', '-o', 'json')['items']
    if any(any(o.get('controller') is True and o.get('kind') == 'Job' and o.get('uid') == binding['job_uid']
               for o in p.get('metadata', {}).get('ownerReferences', [])) for p in pods):
        raise ValueError('cleaned migration Pod present')
