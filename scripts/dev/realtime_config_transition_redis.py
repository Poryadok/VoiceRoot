"""Guarded, fixed no-auth Redis reachability observation; not admission."""
import base64
import copy
import re

import full_cohort_schema_metadata as metadata
import frontend_runtime_executor
import media_operation_guard


class RedisObservationError(RuntimeError): pass


def observe(fence,client,lock,namespace_uid):
    """Observe one fixed DEV Redis endpoint and its empty P3 replay credential.

    PING and sampled ACL DRYRUNs do not prove all replay key permissions, peer
    connectivity, or rollout readiness. Call only under the central live fence.
    """
    try:
        if type(client) is not frontend_runtime_executor.KubectlTransport or fence.ledger.client is not client: raise ValueError()
        def guard(): media_operation_guard.verify(fence,client,lock,namespace_uid)
        def get(kind,name):
            guard(); value = client.call('get',kind,name,'-o','json'); guard(); return value
        def ident(item,kind,name):
            m = item.get('metadata',{})
            if (item.get('kind') != kind or m.get('name') != name or m.get('namespace') != 'voice-dev'
                    or m.get('deletionTimestamp') or any(not isinstance(m.get(k),str) or not m[k] for k in ('uid','resourceVersion'))): raise ValueError()
            return {'uid':m['uid'],'resourceVersion':m['resourceVersion']}
        def snapshot():
            service = get('service','voice-redis'); si = ident(service,'Service','voice-redis'); spec = service.get('spec',{})
            if (spec.get('type','ClusterIP') != 'ClusterIP' or not isinstance(spec.get('clusterIP'),str)
                    or spec['clusterIP'] in ('','None') or spec.get('selector') != {'app':'voice-redis'}
                    or len(spec.get('ports',[])) != 1 or spec['ports'][0].get('port') != 6379
                    or spec['ports'][0].get('targetPort',6379) != 6379
                    or spec['ports'][0].get('protocol','TCP') != 'TCP'): raise ValueError()
            endpoint = get('endpoints','voice-redis'); ei = ident(endpoint,'Endpoints','voice-redis')
            subsets = endpoint.get('subsets',[])
            if len(subsets) != 1 or subsets[0].get('notReadyAddresses') or len(subsets[0].get('addresses',[])) != 1: raise ValueError()
            ports = subsets[0].get('ports',[])
            if len(ports) != 1 or ports[0].get('port') != 6379 or ports[0].get('protocol','TCP') != 'TCP': raise ValueError()
            address = subsets[0]['addresses'][0]; ref = address.get('targetRef',{}); name = ref.get('name')
            if (ref.get('kind') != 'Pod' or ref.get('namespace','voice-dev') != 'voice-dev'
                    or not isinstance(name,str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9]*[a-z0-9])?',name) or len(name)>253): raise ValueError()
            pod = get('pod',name); pi = ident(pod,'Pod',name)
            statuses = [x for x in pod.get('status',{}).get('containerStatuses',[]) if x.get('name') == 'redis']
            if (ref.get('uid') != pi['uid'] or pod.get('metadata',{}).get('labels',{}).get('app') != 'voice-redis'
                    or pod.get('status',{}).get('podIP') != address.get('ip') or len(statuses) != 1
                    or statuses[0].get('ready') is not True or not isinstance(statuses[0].get('containerID'),str)
                    or not statuses[0]['containerID'] or type(statuses[0].get('restartCount')) is not int): raise ValueError()
            secret = get('secret','voice-p3-principal-replay'); ri = ident(secret,'Secret','voice-p3-principal-replay')
            if (secret.get('type') != 'Opaque' or set(secret.get('data',{})) != {'password'}
                    or base64.b64decode(secret['data']['password'],validate=True) != b''): raise ValueError()
            return {'service':si,'endpoints':ei,'pod':pi | {'name':name,'container_id':statuses[0]['containerID'],
                    'restart_count':statuses[0]['restartCount']},'replay_secret':ri}
        first = snapshot()
        base = frontend_runtime_executor.KubectlTransport.PREFIX
        commands = [('PING',), ('ACL','WHOAMI'),
            ('ACL','DRYRUN','default','SET','chat:principal:replay:'+'0'*64,'1','NX','PX','1000'),
            ('ACL','DRYRUN','default','SET','realtime:lifecycle:principal:replay:'+'0'*64,'1','NX','PX','1000')]
        for args,expected in zip(commands,(b'PONG',b'default',b'OK',b'OK')):
            guard()
            raw = metadata.linux_private_bytes(base+['exec',first['pod']['name'],'-c','redis','--',
                 'env','-u','REDISCLI_AUTH','redis-cli','--raw','-h','voice-redis','-p','6379',*args])
            guard()
            if raw.strip() != expected: raise ValueError()
        if snapshot() != first: raise ValueError()
        guard()
        return copy.deepcopy(first) | {'scope':'redis-noauth-ping-and-sampled-acl-only'}
    except Exception:
        raise RedisObservationError('Redis observation unavailable') from None
