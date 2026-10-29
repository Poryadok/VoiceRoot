"""Observe PostgreSQL sessions belonging to source-resolved resumed writers.

Pod IP is a cooperative correlation, not cryptographic workload identity.
The caller retains the writer fence and rejects ambiguous routing or sessions.
"""
import copy
import ipaddress
import json
import re

import canonical_application_package as canonical
import full_cohort_inventory_phase as inventory
import full_cohort_schema_metadata as metadata
import writer_cut
import writer_cut_pg_probe as pg

NATS_IMAGE='nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb'


def _leaf_identity(cut, receipt, controller, pod):
    """Admit only the existing source-known NATS leaf under the cooperative boundary."""
    name=controller['name']; workload=name[6:]; spec=pod['spec']
    originals=[x for x in cut.admission['controllers'] if x.get('kind')=='Deployment' and x.get('metadata',{}).get('name')==name]
    if len(originals)!=1: raise pg.ProbeError('leaf source unavailable')
    old=originals[0]['spec']['template']['spec']; target=controller['spec']['template']['spec']
    def leaf(value):
        found=[x for x in value.get('containers',[]) if x.get('name')=='nats-leaf']
        if len(found)!=1: raise pg.ProbeError('leaf identity invalid')
        return found[0]
    source=leaf(old); expected=leaf(target); actual=copy.deepcopy(leaf(spec))
    # The controller may inject only the standard service-account token mount.
    mounts=actual.get('volumeMounts',[])
    injected=[m for m in mounts if m.get('mountPath')=='/var/run/secrets/kubernetes.io/serviceaccount' and
              m.get('name','').startswith('kube-api-access-') and m.get('readOnly') is True and
              set(m)=={'name','mountPath','readOnly'}]
    if len(injected)>1: raise pg.ProbeError('leaf mount invalid')
    if injected:
        actual['volumeMounts']=[m for m in mounts if m!=injected[0]]
        if not actual['volumeMounts'] and 'volumeMounts' not in expected: actual.pop('volumeMounts')
    config=[m for m in expected.get('volumeMounts',[]) if m.get('mountPath')=='/etc/nats/leaf.conf']
    volumes=[v for v in target.get('volumes',[]) if len(config)==1 and v.get('name')==config[0].get('name')]
    old_volumes=[v for v in old.get('volumes',[]) if len(config)==1 and v.get('name')==config[0].get('name')]
    pod_volumes=[v for v in spec.get('volumes',[]) if len(config)==1 and v.get('name')==config[0].get('name')]
    if (source!=expected or actual!=expected or expected.get('image')!=NATS_IMAGE or expected.get('command') or
            expected.get('args')!=['-c','/etc/nats/leaf.conf','-a','127.0.0.1','-m','8222'] or
            len(config)!=1 or config[0].get('subPath')!='leaf.conf' or config[0].get('readOnly') is not True or
            len(volumes)!=1 or volumes[0].get('configMap')!={'name':'voice-nats-leaf-config'} or
            old_volumes!=volumes or pod_volumes!=volumes):
        raise pg.ProbeError('leaf source changed')
    statuses=[x for x in pod.get('status',{}).get('containerStatuses',[]) if x.get('name')=='nats-leaf']
    historical=[c['image_id'] for p in receipt['projection'][workload]['pods'] for c in p['containers'] if c['name']=='nats-leaf']
    if (len(statuses)!=1 or not historical or len(set(historical))!=1 or statuses[0].get('imageID')!=historical[0] or
            not statuses[0].get('containerID') or type(statuses[0].get('restartCount')) is not int or
            not isinstance(statuses[0].get('state',{}).get('running'),dict)):
        raise pg.ProbeError('leaf runtime identity invalid')
    cut._guard(); configmap=cut.client.call('get','configmap','voice-nats-leaf-config','-o','json'); cut._guard()
    m=configmap.get('metadata',{})
    if m.get('namespace')!='voice-dev' or not m.get('uid') or not m.get('resourceVersion'):
        raise pg.ProbeError('leaf config identity invalid')
    value=configmap.get('data',{}).get('leaf.conf')
    if not isinstance(value,str) or not value or len(value.encode())>65536: raise pg.ProbeError('leaf config invalid')
    args=['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
          '--request-timeout=15s','exec',pod['metadata']['name'],'-c','nats-leaf','--','cat','/etc/nats/leaf.conf']
    cut._guard(); mounted=metadata.linux_private_bytes(args); cut._guard()
    if mounted!=value.encode(): raise pg.ProbeError('leaf mounted config changed')
    return (statuses[0]['containerID'],statuses[0]['restartCount'],statuses[0]['imageID'],m['uid'],m['resourceVersion'])


def _routes(value):
    if not isinstance(value,dict) or value.get('binding',{}).get('state')!='resolved':
        raise pg.ProbeError('writer route unresolved')
    entries=[(value['binding'],value.get('metadata'))]
    entries.extend((item.get('binding'),item.get('metadata')) for item in value.get('secondary',[]))
    result=set()
    for binding, observed in entries:
        if not isinstance(binding,dict) or not isinstance(observed,dict): raise pg.ProbeError('writer route unresolved')
        db,role=binding.get('database'),binding.get('role')
        session=observed.get('session',{})
        if (not isinstance(db,str) or db not in pg.ALLOWED or not isinstance(role,str) or
                not metadata.SAFE.fullmatch(role) or observed.get('database')!=db or
                type(session.get('database_oid')) is not int):
            raise pg.ProbeError('writer route invalid')
        result.add((db,role,session['database_oid']))
    return result


def inspect_resumed(cut, phase, token):
    """Attribute only resumed, owned Pods with source-resolved unchanged routes.

    Auth and Realtime transitions require their own typed successor proofs and
    remain closed here until those proofs are integrated.
    """
    try:
        if type(cut) is not writer_cut.WriterCut or type(phase) is not inventory.InventoryPhase:
            raise pg.ProbeError('writer authority invalid')
        scope=pg._validate_scope(cut.admission['session_scope'],cut._guard)
        if (phase.fence.context!=cut.fence.context or phase.lock!=cut.lock or
                phase.namespace_uid!=cut.namespace_uid): raise pg.ProbeError('writer authority changed')
        if cut.fence.ledger.read()!=phase.fence.ledger.read(): raise pg.ProbeError('physical ledger differs')
        receipt=phase.verify_receipt(token); package=phase.package.verify_current(token); proof=cut.current_resumed_proof()
        if not proof['controllers']:
            result=pg.inspect(scope,cut._guard)
            return result | {'attributed_sessions':[]}
        def authority(sha):
            return sha==cut.fence.context['commit'] and phase.verify_receipt(token)==receipt
        catalog=metadata.catalog_from_source(canonical.ROOT,cut.fence.context['commit'],authority)
        expected={entry['name']:entry for entry in proof['controllers']}
        if len(expected)!=len(proof['controllers']): raise pg.ProbeError('writer proof ambiguous')
        for name,entry in expected.items():
            if entry['kind']!='Deployment' or not name.startswith('voice-') or name[6:] not in metadata.CONTAINERS:
                raise pg.ProbeError('writer kind unsupported')
            if name[6:] in ('auth','realtime'): raise pg.ProbeError('writer successor not qualified')
        def sample():
            cut._guard()
            replica_sets=cut.client.call('get','replicasets','-o','json')['items']
            pods=cut.client.call('get','pods','-o','json')['items']
            owners={v['uid']:name for name,v in expected.items()}
            rs={}
            for item in replica_sets:
                owner=inventory._owner(item,'Deployment'); m=item.get('metadata',{})
                if owner in owners:
                    if not m.get('uid') or m['uid'] in rs: raise pg.ProbeError('replicaset identity invalid')
                    rs[m['uid']]=owners[owner]
            current=[]; ips=set()
            for pod in pods:
                owner=inventory._owner(pod,'ReplicaSet')
                if owner not in rs or pod.get('status',{}).get('phase') in ('Succeeded','Failed'): continue
                m=pod.get('metadata',{}); status=pod.get('status',{}); spec=pod.get('spec',{})
                uid,ip=m.get('uid'),status.get('podIP')
                if (not isinstance(uid,str) or not uid or m.get('namespace')!='voice-dev' or
                        m.get('deletionTimestamp') or spec.get('hostNetwork',False) is not False or
                        not isinstance(ip,str) or str(ipaddress.ip_address(ip))!=ip or ip in ips):
                    raise pg.ProbeError('pod network identity invalid')
                appname=metadata.CONTAINERS[rs[owner][6:]]
                containers=spec.get('containers',[])
                names=[c.get('name') for c in containers] if isinstance(containers,list) else []
                if (len(names)!=len(set(names)) or set(names) not in ({appname},{appname,'nats-leaf'}) or
                        spec.get('ephemeralContainers',[]) or spec.get('initContainers',[])):
                    raise pg.ProbeError('shared pod network not qualified')
                leaf=_leaf_identity(cut,receipt,expected[rs[owner]],pod) if 'nats-leaf' in names else None
                statuses=[s for s in status.get('containerStatuses',[]) if s.get('name')==appname]
                if status.get('phase')=='Running' and (len(statuses)!=1 or not statuses[0].get('containerID') or
                        not statuses[0].get('imageID') or
                        type(statuses[0].get('restartCount')) is not int): raise pg.ProbeError('container identity invalid')
                if status.get('phase')=='Running':
                    desired=expected[rs[owner]]['spec']['template']['spec']['containers']
                    image_key=rs[owner]+'/'+appname
                    actual_app=[c for c in containers if c.get('name')==appname]
                    intended=[c for c in desired if c.get('name')==appname]
                    image_match=re.search(r'(sha256:[a-f0-9]{64})$',statuses[0]['imageID'])
                    if (len(actual_app)!=1 or len(intended)!=1 or actual_app[0].get('image')!=intended[0].get('image') or
                            intended[0].get('image')!=package['images'].get(image_key) or image_match is None or
                            image_match.group(1)!=package['platform_digests'].get(image_key) or
                            spec.get('nodeName') not in {n['name'] for n in package['nodes']}):
                        raise pg.ProbeError('application image identity invalid')
                identity=(statuses[0]['containerID'],statuses[0]['restartCount'],statuses[0]['imageID']) if len(statuses)==1 else (None,None,None)
                ips.add(ip); current.append((rs[owner],m['name'],uid,ip,status.get('phase'),*identity,leaf))
            cut._guard()
            return sorted(current)
        before=sample(); allowed=set(); admin=phase._admin_role()
        old=receipt['database_observations']['workloads']
        for name,pod,uid,ip,state,container_id,restarts,image_id,_leaf in before:
            workload=name[6:]
            if state!='Running': continue
            if workload not in metadata.OWNERS and workload!='gateway': continue
            entries=old.get(workload)
            if not entries or any(x.get('status')!='observed' for x in entries): raise pg.ProbeError('baseline route unavailable')
            prior=[_routes(x['result']) for x in entries]
            if any(routes!=prior[0] for routes in prior): raise pg.ProbeError('baseline routes differ')
            actual=metadata.inspect(catalog,workload,pod,admin,cut._guard)
            binding_identity=actual.get('binding',{}).get('binding',{})
            if (binding_identity.get('pod_uid')!=uid or binding_identity.get('container_id')!=container_id or
                    binding_identity.get('restart_count')!=restarts): raise pg.ProbeError('pod identity changed')
            routes=_routes(actual)
            if routes!=prior[0]: raise pg.ProbeError('writer route changed')
            for db,role,oid in routes:
                observed=actual['metadata'] if actual['metadata']['database']==db else next(
                    x['metadata'] for x in actual['secondary'] if x['metadata']['database']==db)
                if (observed['engine']!=scope['engine'] or observed['session']['server_start']!=scope['server_start'] or
                        db in scope['database_oids'] and scope['database_oids'][db]!=oid):
                    raise pg.ProbeError('writer engine changed')
                if db in scope['databases']: allowed.add((db,role,ip))
        # Query and final checks are deliberately separate from route resolution:
        # no process or Pod is accepted solely because it claims an application name.
        snapshot=metadata.kube_engine_snapshot(metadata.linux_private_bytes,cut._guard)
        if snapshot()!=scope['engine']: raise pg.ProbeError('engine changed')
        names=scope['databases']; array=','.join("'"+x+"'" for x in names)
        pairs=','.join("('"+x+"',"+str(scope['database_oids'][x])+"::oid)" for x in names)
        sql=("SELECT json_build_object('server_start',pg_catalog.pg_postmaster_start_time(),"
             "'can_read',pg_catalog.pg_has_role(current_user,'pg_read_all_stats','MEMBER'),"
             "'matched',(SELECT count(*) FROM pg_catalog.pg_database d JOIN (VALUES "+pairs+
             ") e(name,oid) ON d.datname=e.name AND d.oid=e.oid),"
             "'prepared',(SELECT count(*) FROM pg_catalog.pg_prepared_xacts WHERE database=ANY(ARRAY["+array+
             "])), 'sessions',COALESCE(json_agg(json_build_object('pid',pid,'backend_start',backend_start,"
             "'database',datname,'role',usename,'client_addr',pg_catalog.host(client_addr),"
             "'transaction',xact_start IS NOT NULL)),'[]'::json)) FROM pg_catalog.pg_stat_activity "
             "WHERE datname=ANY(ARRAY["+array+"]) AND pid<>pg_catalog.pg_backend_pid()")
        args=['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
              '--request-timeout=15s','exec','voice-postgres-0','-c','postgres','--','sh','-ec',
              metadata.PSQL_SCRIPT,'--','postgres',sql]
        cut._guard(); response=json.loads(metadata.linux_psql(args)); cut._guard()
        if (set(response)!={'server_start','can_read','matched','prepared','sessions'} or
                response['server_start']!=scope['server_start'] or response['can_read'] is not True or
                type(response['matched']) is not int or response['matched']!=len(names) or
                type(response['prepared']) is not int or response['prepared']<0 or
                not isinstance(response['sessions'],list) or len(response['sessions'])>1024): raise pg.ProbeError('session observation invalid')
        matched=[]; other=0; transactions=response['prepared']; seen=set()
        for s in response['sessions']:
            if (not isinstance(s,dict) or set(s)!={'pid','backend_start','database','role','client_addr','transaction'} or
                    type(s['pid']) is not int or s['pid']<=0 or s['pid'] in seen or
                    not isinstance(s['backend_start'],str) or not s['backend_start'] or type(s['transaction']) is not bool):
                raise pg.ProbeError('session identity invalid')
            seen.add(s['pid'])
            if (s['database'],s['role'],s['client_addr']) in allowed: matched.append(s)
            else: other+=1; transactions+=int(s['transaction'])
        if (sample()!=before or snapshot()!=scope['engine'] or cut.current_resumed_proof()!=proof or
                phase.verify_receipt(token)!=receipt or phase.package.verify_current(token)!=package or
                cut.fence.ledger.read()!=phase.fence.ledger.read()):
            raise pg.ProbeError('observation changed')
        return {'scope':copy.deepcopy(scope),'active_sessions':other,'active_transactions':transactions,'attributed_sessions':matched}
    except BaseException:
        raise pg.ProbeError('resumed session observation unavailable') from None
