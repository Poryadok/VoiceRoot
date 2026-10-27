#!/usr/bin/env python3
"""Fixed, credential-free f194 node/API probe contracts.

These builders and validators grant no execution or resource authority. The
guarded controller must bind their results to its actual lock/window/journal.
"""
import copy
import datetime
import re
from auth_user_adoption import RecoveryRequired

LOCK='voice-dev-image-update-lock'

IMAGES={
    'auth':{'ref':'ghcr.io/poryadok/voiceroot/dev-auth@sha256:67de6a724618f1d7a98a95103690d315f5b0fa069071b384c98b93c712a87885',
            'path':'/auth.jar','sha256':'1c5725b380007317eaf75bf86a9cf1abc5794c801372c7a05fbb6b5c847fe0cc','bytes':69905589},
    'user':{'ref':'ghcr.io/poryadok/voiceroot/dev-user@sha256:42b84cfe7766bdb702ac73f50f21dc92bd60456220d7e04d6e27c0f4bfc14a61',
            'path':'/service','sha256':'03eb359e8e1ec6b6edd5db9428ddc51e5175d261ff9a7f87261e3f1a8307b04a','bytes':34406562}}


def require(value,message):
    if not value:raise RecoveryRequired(message)


def operation_id(operation):
    require(isinstance(operation,str) and re.fullmatch(r'[0-9a-f]{24}',operation),'probe operation identity invalid')
    return operation


def artifact_pod(operation,service):
    operation_id(operation);require(service in IMAGES,'probe service invalid')
    item=IMAGES[service];path=item['path']
    return {'apiVersion':'v1','kind':'Pod','metadata':{'namespace':'voice-dev',
        'name':'voice-db-artifact-'+operation+'-'+service,
        'labels':{'voice.test.owner':'Dimas-16','voice.probe.operation':operation}},
        'spec':{'restartPolicy':'Never','activeDeadlineSeconds':120,'terminationGracePeriodSeconds':5,
            'automountServiceAccountToken':False,'enableServiceLinks':False,
            'securityContext':{'runAsNonRoot':True,'runAsUser':65532,'seccompProfile':{'type':'RuntimeDefault'}},
            'containers':[{'name':'artifact','image':item['ref'],'imagePullPolicy':'Always',
                'command':['/bin/sh','-ec','sha256sum '+path+'; wc -c < '+path],
                'securityContext':{'readOnlyRootFilesystem':True,'allowPrivilegeEscalation':False,
                    'capabilities':{'drop':['ALL']}},
                'resources':{'requests':{'cpu':'10m','memory':'32Mi'},'limits':{'cpu':'100m','memory':'128Mi'}}}]}}


def selector_matches(labels,selector):
    require(isinstance(selector,dict),'unknown controller selector')
    require(not set(selector)-{'matchLabels','matchExpressions'},'unknown selector structure')
    values=selector.get('matchLabels',{})
    require(isinstance(values,dict),'unknown selector labels')
    matched=all(labels.get(k)==v for k,v in values.items())
    expressions=selector.get('matchExpressions',[])
    require(isinstance(expressions,list),'unknown selector expressions')
    for expression in expressions:
        require(isinstance(expression,dict) and not set(expression)-{'key','operator','values'},'unknown selector expression')
        key=expression.get('key');operator=expression.get('operator');values=expression.get('values',[])
        require(isinstance(key,str) and isinstance(values,list),'invalid selector expression')
        require(operator in ('In','NotIn','Exists','DoesNotExist'),'unknown selector operator')
        require((bool(values) if operator in ('In','NotIn') else not values),'invalid selector operand')
        selected={'In':key in labels and labels[key] in values,
                  'NotIn':key not in labels or labels[key] not in values,
                  'Exists':key in labels,'DoesNotExist':key not in labels}[operator]
        matched=matched and selected
    return matched


def exclude_selectors(labels,objects):
    for item in objects:
        kind=item.get('kind');selector=item.get('spec',{}).get('selector')
        if kind in ('Service','ReplicationController'):
            if not selector:continue # A selector-less Service does not select Pods.
            selector={'matchLabels':selector}
        else:
            require(kind in ('Deployment','ReplicaSet','StatefulSet','DaemonSet','Job'), 'unrecognized selector consumer')
            require(selector is not None,'controller selector missing')
        require(not selector_matches(labels,selector),'probe matches existing Service or controller selector')


def admitted_pod(intended,actual):
    require(actual.get('kind')=='Pod' and actual.get('apiVersion')=='v1','admitted probe kind changed')
    for key in ('namespace','name','labels'):
        require(actual.get('metadata',{}).get(key)==intended['metadata'].get(key),'admitted probe identity changed')
    spec=copy.deepcopy(actual.get('spec',{}))
    defaults={'dnsPolicy':'ClusterFirst','schedulerName':'default-scheduler','serviceAccountName':'default',
              'serviceAccount':'default','priority':0,'preemptionPolicy':'PreemptLowerPriority'}
    for key,value in defaults.items():
        if key in spec and key not in intended['spec']:
            require(spec.pop(key)==value,'unapproved Pod default')
    if 'tolerations' in spec and 'tolerations' not in intended['spec']:
        allowed=[{'key':'node.kubernetes.io/'+key,'operator':'Exists','effect':'NoExecute','tolerationSeconds':300}
                 for key in ('not-ready','unreachable')]
        tolerations=spec.pop('tolerations')
        require(isinstance(tolerations,list) and all(t in allowed for t in tolerations),'unapproved probe toleration')
    for container in spec.get('containers',[]):
        for key,value in (('terminationMessagePath','/dev/termination-log'),('terminationMessagePolicy','File')):
            if key in container:require(container.pop(key)==value,'unapproved container default')
    require(spec==intended['spec'],'admission changed credential-free probe spec')
    return copy.deepcopy(actual['spec'])


def delete_options(obj,operation,uid):
    operation_id(operation)
    metadata=obj.get('metadata',{});labels=metadata.get('labels',{})
    require(isinstance(uid,str) and uid and metadata.get('uid')==uid,'probe cleanup UID changed')
    require(labels.get('voice.test.owner')=='Dimas-16' and labels.get('voice.probe.operation')==operation,
            'probe cleanup owner changed')
    require(isinstance(metadata.get('resourceVersion'),str) and metadata['resourceVersion'],'probe cleanup version missing')
    return {'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
        'preconditions':{'uid':uid,'resourceVersion':metadata['resourceVersion']}}


def deadline_receipt(job,pods,uid):
    require(job.get('metadata',{}).get('uid')==uid,'deadline Job UID changed')
    status=job.get('status',{})
    require(not status.get('active') and status.get('failed',0)>=1,'deadline Job remains active')
    conditions=status.get('conditions',[])
    require(any(c.get('type')=='Failed' and c.get('status')=='True' and c.get('reason')=='DeadlineExceeded'
                for c in conditions),'actual deadline failure absent')
    require(not any(c.get('type')=='Complete' and c.get('status')=='True' for c in conditions),'deadline Job completed instead')
    require(len(pods)==1,'deadline owned Pod set unexpected')
    pod=pods[0];owners=[o for o in pod.get('metadata',{}).get('ownerReferences',[]) if o.get('controller')]
    require(len(owners)==1 and owners[0].get('uid')==uid and owners[0].get('kind')=='Job','deadline child owner changed')
    require(pod.get('metadata',{}).get('uid') and pod.get('status',{}).get('phase')=='Failed','deadline child not terminal')
    containers=pod['status'].get('containerStatuses',[])
    require(len(containers)==1 and containers[0].get('name')=='artifact' and containers[0].get('containerID'),
            'actual deadline container identity absent')
    terminal=containers[0].get('state',{}).get('terminated',{})
    try:
        started=datetime.datetime.fromisoformat(terminal['startedAt'].replace('Z','+00:00'))
        finished=datetime.datetime.fromisoformat(terminal['finishedAt'].replace('Z','+00:00'))
        require(started.tzinfo and finished.tzinfo and (finished-started).total_seconds()>=1,
                'deadline container did not execute sleep')
        require(terminal['exitCode']!=0,'deadline command exited successfully')
    except (KeyError,TypeError,ValueError):
        raise RecoveryRequired('actual deadline container start/finish absent') from None
    return {'job_uid':uid,'pod_uid':pod['metadata']['uid'],'container_id':containers[0]['containerID'],
            'started_at':terminal['startedAt'],'finished_at':terminal['finishedAt'],'reason':'DeadlineExceeded'}


class ProbeController:
    """Write-ahead scratch creation. Uncertain effects require reconciliation.

    guard is the real current-window/baseline verifier, not an approval flag.
    Journal supplies the existing private fsync/lock/immutable-intent contract.
    """
    def __init__(self,journal,kube,guard):
        self.journal=journal;self.kube=kube;self.guard=guard
        self.intent=journal.state['intent'];self.operation=operation_id(self.intent['operation'])

    def _get(self,kind,name):
        return self.kube.call('get',kind,name,'--ignore-not-found=true','-o','json')

    def _lock_body(self):
        resource=self.intent['resource']
        return {'apiVersion':'v1','kind':'ConfigMap','metadata':{'namespace':'voice-dev','name':LOCK,
                'labels':{'voice.test.owner':'Dimas-16','voice.probe.operation':self.operation}},
                'data':{'owner':'Dimas-16','operation':self.operation,'purpose':'auth-user-candidate-probe',
                    'claim':resource['claim'],'generation':str(resource['generation']),'window':resource['window']}}

    def _check_lock(self,current,record=None):
        expected=self._lock_body();metadata=current.get('metadata',{})
        require(current.get('data')==expected['data'] and metadata.get('labels')==expected['metadata']['labels'],
                'common updater lock is foreign or changed')
        require(metadata.get('uid') and metadata.get('resourceVersion'),'common lock identity absent')
        if record:
            require(metadata['uid']==record['uid'] and metadata['resourceVersion']==record['resource_version'],
                    'common lock UID/version changed')

    def acquire_lock(self):
        self.guard()
        current=self._get('configmap',LOCK)
        record=self.journal.state.get('probe_lock')
        if record:
            require(record['state']=='done','uncertain lock creation; reconcile without repeating')
            self._check_lock(current,record);return record
        require(not current,'common updater lock already held')
        body=self._lock_body()
        self.journal.state['probe_lock']={'state':'pending','body':copy.deepcopy(body)};self.journal.save()
        # Failure or lost response leaves durable pending; no blind second CREATE.
        current=self.kube.call('create','-f','-','-o','json',body=body)
        self._check_lock(current)
        record={'state':'done','uid':current['metadata']['uid'],
                'resource_version':current['metadata']['resourceVersion']}
        self.journal.state['probe_lock']=record;self.journal.save()
        return record

    def _guard(self):
        self.guard()
        record=self.journal.state.get('probe_lock',{})
        require(record.get('state')=='done','actual common updater lock required before scratch writes')
        self._check_lock(self._get('configmap',LOCK),record)

    def create_artifact(self,service,selectors):
        self._guard()
        records=self.journal.state.setdefault('probe',{})
        require(service not in records,'artifact creation already intended; reconcile without repeating')
        body=artifact_pod(self.operation,service)
        exclude_selectors(body['metadata']['labels'],selectors)
        # Re-read the live selector consumers immediately before the effect;
        # baseline observations supplied by a caller cannot waive this check.
        current=self.kube.call('get','services,deployments,replicasets,statefulsets,daemonsets,replicationcontrollers,jobs','-o','json')
        require(isinstance(current.get('items'),list),'current selector inventory absent')
        exclude_selectors(body['metadata']['labels'],current['items'])
        admitted=self.kube.call('create','--dry-run=server','-f','-','-o','json',body=body)
        spec=admitted_pod(body,admitted)
        self._guard()
        records[service]={'state':'pending','body':body,'admitted_spec':spec};self.journal.save()
        actual=self.kube.call('create','-f','-','-o','json',body=body)
        admitted_pod(body,actual)
        require(actual['spec']==spec,'created probe differs from dry-run spec')
        metadata=actual.get('metadata',{})
        require(metadata.get('uid') and metadata.get('resourceVersion'),'actual probe creation UID absent')
        records[service].update(state='created',uid=metadata['uid'],resource_version=metadata['resourceVersion'])
        self.journal.save()
        return copy.deepcopy(records[service])
