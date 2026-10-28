#!/usr/bin/env python3
"""Fixed, credential-free f194 node/API probe contracts.

These builders and validators grant no execution or resource authority. The
guarded controller must bind their results to its actual lock/window/journal.
"""
import argparse
import copy
import datetime
import hashlib
import json
import os
import pathlib
import re
import secrets
import stat
import sys
import time
import urllib.request
from auth_user_adoption import RecoveryRequired,Kube,Journal
from auth_user_adoption_runtime import digest

LOCK='voice-dev-image-update-lock'
NAMESPACE_UID='15950fc8-0392-4053-9156-e95e78b596f9'
PROTOCOL='voice-auth-user-node-probe-v1'
PRODUCT='f1946355a084bed4c62a602ec1dc04e2d57e52ee'


class ProbePending(RecoveryRequired):
    """Known owned server object has not reached the required terminal state."""


def pending_unless(value,message):
    if not value:raise ProbePending(message)


def probe_resource_card():
    from auth_user_adoption_runtime import resource_card
    import urllib.request
    card=resource_card()
    with urllib.request.urlopen('http://192.168.0.109:30990/api/projects/voice/tasks/A1-DEV-AUTH-USER-CREDENTIAL-ADOPTION',timeout=10) as response:
        task=json.load(response)
    artifact_review=task.get('status')=='review' and card.get('phase')=='prepare'
    require(task.get('owner')=='Dimas-16' and task.get('assignee')=='Dimas-16' and
            (artifact_review or task.get('status') in ('dev_ready_for_deploy','dev_deploying')) and not task.get('review_required'),
            'actual adoption task ownership/stage/review gate not eligible for candidate preflight')
    return card


def candidate_projection(candidate):
    # TT validates ResourceVersion material at acquisition. Bind the complete
    # typed server object (state/commit/images/deployment/evidence), not a
    # locally invented version label or a second partial schema.
    require(isinstance(candidate,dict) and candidate.get('state')=='known','actual typed known ResourceVersion candidate required')
    return digest(candidate)


def prepare_probe(directory,binding,kube,resource=None,clock=time.time):
    require(isinstance(binding,dict) and set(binding)=={'protocol','candidate_source','namespace_uid','probe_source','review','resource'},
            'probe binding fields invalid')
    require(binding['protocol']==PROTOCOL and binding['candidate_source']==PRODUCT and binding['namespace_uid']==NAMESPACE_UID,
            'fixed DEV/product protocol binding changed')
    review=binding['review'];fingerprint=probe_fingerprint()
    require(isinstance(binding['probe_source'],str) and re.fullmatch(r'[0-9a-f]{40}',binding['probe_source']),
            'reviewed exact probe source SHA required')
    require(isinstance(review,dict) and set(review)=={'reviewer','source_sha','fingerprint','evidence'} and
            review['reviewer']=='Dimas-8' and review['source_sha']==binding['probe_source'] and
            review['fingerprint']==fingerprint and isinstance(review['evidence'],str) and
            review['evidence'].startswith(('http://','https://')),
            'actual independently frozen probe review binding absent or source changed')
    expected=binding['resource']
    require(isinstance(expected,dict) and set(expected)=={'claim','generation','window','candidate_sha256'} and
            type(expected['generation']) is int and expected['generation']>0 and all(
                isinstance(expected[key],str) and 0<len(expected[key])<=200 for key in ('claim','window')) and
            isinstance(expected['candidate_sha256'],str) and re.fullmatch(r'[0-9a-f]{64}',expected['candidate_sha256']),
            'actual resource claim/generation/window/candidate binding required')
    resource=resource or probe_resource_card;card=resource()
    require(card.get('state')=='held' and card.get('owner')=='Dimas-16' and card.get('phase')=='prepare' and
            card.get('tasks')==['A1-DEV-AUTH-USER-CREDENTIAL-ADOPTION'] and
            all(card.get(key)==expected[key] for key in ('claim','generation','window')) and
            candidate_projection(card.get('candidate'))==expected['candidate_sha256'],
            'actual DEV prepare claim does not match binding')
    require(not kube.call('get','configmap',LOCK,'--ignore-not-found=true','-o','json'),'common updater lock already held')
    baseline=probe_baseline(kube);now=clock()
    intent={'protocol':PROTOCOL,'operation':secrets.token_hex(12),'resource':copy.deepcopy(expected),
        'fingerprint':fingerprint,'binding':copy.deepcopy(binding),'prepared_at':now,'expires_at':now+900,'baseline':baseline}
    journal=Journal(directory,intent)
    try:
        WindowGuard(journal,kube,resource=resource,clock=clock)()
        return {'result':'PREPARED','operation':intent['operation'],'fingerprint':fingerprint,'live_writes':False}
    finally:journal.close()


def probe_fingerprint():
    root=pathlib.Path(__file__).resolve().parent;hasher=hashlib.sha256()
    for name in ('auth_user_candidate_probe.py','auth_user_adoption.py','auth_user_adoption_runtime.py'):
        path=root/name
        require(not path.is_symlink() and stat.S_ISREG(path.stat().st_mode),'probe source alias/non-file refused')
        hasher.update(name.encode()+b'\0'+path.read_bytes()+b'\0')
    return hasher.hexdigest()


def public_health(opener=None):
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self,*args,**kwargs):
            raise RecoveryRequired('public DEV health redirect refused')
    url='https://api-dev.comrade.click/health'
    opener=opener or urllib.request.build_opener(NoRedirect()) # Default verified TLS.
    with opener.open(urllib.request.Request(url,method='GET'),timeout=10) as response:
        require(response.status==200 and response.geturl()==url,'actual public DEV health not200 on fixed route')
        require(len(response.read(4097))<=4096,'public health response exceeds bounded protocol')
    return {'url':url,'status':200} # No health-body/private data retention.


def probe_baseline(kube,state=None):
    from auth_user_adoption_runtime import capture
    snapshot,_,pods=capture(kube)
    require(snapshot['namespace']['uid']==NAMESPACE_UID,'actual DEV namespace identity changed')
    require(snapshot['ready'] and all(snapshot['ready'].values()),'original DEV workloads not all Ready')
    state=state or {};known=set()
    if state:ProbeController(type('View',(),{'state':state})(),kube,lambda:None)
    for record in state.get('probe',{}).values():
        for pod in pods:
            if pod.get('metadata',{}).get('name')!=record['body']['metadata']['name']:continue
            expected_uid=record.get('uid') or record.get('returned_uid_hint')
            require(not expected_uid or pod['metadata'].get('uid')==expected_uid,'owned scratch name replaced')
            checked=copy.deepcopy(pod);checked.get('spec',{}).pop('nodeName',None)
            admitted_pod(record['body'],checked)
            require(not pod['metadata'].get('ownerReferences'),'scratch Pod adopted by controller')
            require(pod['metadata'].get('uid'),'scratch Pod UID absent')
            known.add(pod['metadata']['uid'])
    jobs=kube.call('get','jobs','-o','json')
    require(isinstance(jobs.get('items'),list),'current Job inventory unavailable')
    deadline=state.get('probe_deadline');owned_job=None;existing_jobs={}
    for job in jobs['items']:
        if deadline and job['metadata']['name']==deadline['body']['metadata']['name']:
            admitted_job(deadline['body'],job)
            expected_uid=deadline.get('uid') or deadline.get('returned_uid_hint')
            require(not expected_uid or job['metadata'].get('uid')==expected_uid,'deadline Job replaced')
            if deadline.get('spec'):require(job['spec']==deadline['spec'],'deadline admitted spec changed')
            owned_job=job;continue
        status=job.get('status',{})
        require(not status.get('active') and any(c.get('type') in ('Complete','Failed') and c.get('status')=='True'
                for c in status.get('conditions',[])),'unrelated active or pending Job observed')
        existing_jobs[job['metadata']['name']]={'uid':job['metadata']['uid'],'spec_sha256':digest(job['spec'])}
    if deadline and (owned_job or deadline.get('uid')):
        effective=copy.deepcopy(deadline)
        if owned_job:effective.update(uid=owned_job['metadata']['uid'],spec=owned_job['spec'])
        controller=ProbeController(type('View',(),{'state':state})(),kube,lambda:None)
        known.update(p['metadata']['uid'] for p in controller._deadline_children(effective))
    original_pods={}
    for pod in pods:
        metadata=pod['metadata']
        if metadata['uid'] in known:continue
        status=pod.get('status',{})
        original_pods[metadata['name']]={'uid':metadata['uid'],'owners':metadata.get('ownerReferences',[]),
            'spec_sha256':digest(pod['spec']),'phase':status.get('phase'),
            'deleting':bool(metadata.get('deletionTimestamp')),
            'ready':any(c.get('type')=='Ready' and c.get('status')=='True' for c in status.get('conditions',[])),
            'containers':{c['name']:{k:c.get(k) for k in ('imageID','containerID','restartCount','ready')}
                          for c in status.get('containerStatuses',[])}}
    supplemental=kube.call('get','services,daemonsets,cronjobs,replicationcontrollers,configmaps,secrets,serviceaccounts','-o','json')
    require(isinstance(supplemental.get('items'),list),'original namespace metadata inventory unavailable')
    namespace_objects={}
    for item in supplemental['items']:
        kind=item.get('kind');metadata=item.get('metadata',{})
        require(kind in ('Service','DaemonSet','CronJob','ReplicationController','ConfigMap','Secret','ServiceAccount'),
                'unexpected namespace inventory kind')
        if kind=='ConfigMap' and metadata.get('name')==LOCK and state.get('probe_lock'):continue
        require(metadata.get('uid') and metadata.get('resourceVersion'),'namespace object UID/version missing')
        row={'uid':metadata['uid'],'rv':metadata['resourceVersion']}
        if kind in ('Service','DaemonSet','CronJob','ReplicationController'):row['spec_sha256']=digest(item['spec'])
        if kind in ('Secret','ConfigMap'):
            row.update(type=item.get('type'),keys=sorted(set(item.get('data',{}))|set(item.get('binaryData',{}))))
        namespace_objects[kind+'/'+metadata['name']]=row # Never retain Secret/ConfigMap values.
    return {'snapshot_sha256':digest(snapshot),'namespace_uid':NAMESPACE_UID,
            'ready_workloads':len(snapshot['ready']),'original_pods':original_pods,'terminal_jobs':existing_jobs,
            'namespace_objects':namespace_objects,'public_health':public_health()}


class WindowGuard:
    def __init__(self,journal,kube,resource=None,clock=time.time):
        self.journal=journal;self.kube=kube;self.resource=resource or probe_resource_card;self.clock=clock

    def __call__(self):
        intent=self.journal.state['intent'];now=self.clock()
        extensions=self.journal.state.get('probe_recovery_windows',[])
        period=extensions[-1] if extensions else intent
        require(period['prepared_at']-30<=now<period['expires_at']<=period['prepared_at']+900,
                'probe window expired or clock changed; retain custody for fresh guarded reconciliation')
        self._current()

    def _current(self):
        intent=self.journal.state['intent']
        require(probe_fingerprint()==intent['fingerprint'],'approved probe source changed')
        card=self.resource();expected=intent['resource']
        require(card.get('state')=='held' and card.get('owner')=='Dimas-16' and card.get('phase')=='prepare' and
                card.get('tasks')==['A1-DEV-AUTH-USER-CREDENTIAL-ADOPTION'],'actual DEV prepare custody absent')
        for key in ('claim','generation','window'):
            require(card.get(key)==expected.get(key),'DEV claim/window/candidate changed')
        require(candidate_projection(card.get('candidate'))==expected.get('candidate_sha256'),'actual typed DEV candidate changed')
        lock=self.kube.call('get','configmap',LOCK,'--ignore-not-found=true','-o','json')
        record=self.journal.state.get('probe_lock')
        releasing=bool(self.journal.state.get('probe_finish_pending') or self.journal.state.get('probe_abort_pending'))
        if lock:
            require(record,'unattributed common updater lock observed')
            controller=ProbeController(self.journal,self.kube,self)
            controller._check_lock(lock,record if record['state']=='done' and not releasing else None)
            expected_uid=record.get('uid') or record.get('returned_uid_hint')
            if expected_uid:require(lock['metadata']['uid']==expected_uid,'common lock UID replaced')
        elif record and record['state']=='done':
            require(releasing or self.journal.state.get('probe_finished') or self.journal.state.get('probe_aborted'),
                    'common lock disappeared outside owned release intent')
        require(probe_baseline(self.kube,self.journal.state)==intent['baseline'],'original DEV baseline changed')
        return card

    def refresh(self,reason):
        require(isinstance(reason,str) and 20<=len(reason)<=1000,'explicit reconciliation reason required')
        # This does not change or adopt a different resource fence. The same
        # actual held claim must still pass TT check and the original baseline.
        card=self._current();now=self.clock()
        require(now>=self.journal.state['intent']['prepared_at']-30,'clock moved behind original operation')
        record=self.journal.state.get('probe_lock')
        if record:
            current=self.kube.call('get','configmap',LOCK,'--ignore-not-found=true','-o','json')
            controller=ProbeController(self.journal,self.kube,self)
            releasing=self.journal.state.get('probe_finish_pending') or self.journal.state.get('probe_abort_pending')
            if current:controller._check_lock(current,record if record.get('state')=='done' and not releasing else None)
            else:require(releasing,'owned common lock no longer observed')
        self.journal.state.setdefault('probe_recovery_windows',[]).append({
            'prepared_at':now,'expires_at':now+900,'reason':reason,'resource_revision':card['revision']})
        self.journal.save()


class ProbeKube(Kube):
    def logs(self,name):
        require(re.fullmatch(r'voice-db-artifact-[0-9a-f]{24}-(auth|user)',name),'artifact log target invalid')
        command=['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default',
            '--namespace','voice-dev','--request-timeout=15s','logs',name,'--container=artifact','--limit-bytes=1024','--tail=2']
        try:
            result=self.runner(command,capture_output=True,text=True,timeout=20)
            require(result.returncode==0 and len(result.stdout)<=1024,'artifact log read incomplete')
            return result.stdout
        except BaseException:
            raise RecoveryRequired('artifact log read incomplete') from None

IMAGES={
    'auth':{'ref':'ghcr.io/poryadok/voiceroot/dev-auth@sha256:67de6a724618f1d7a98a95103690d315f5b0fa069071b384c98b93c712a87885',
            'config':'sha256:637142d44eac253dedbc8f64a8d168b02b44d2e0ab93743e1143d44117418a35',
            'path':'/auth.jar','sha256':'1c5725b380007317eaf75bf86a9cf1abc5794c801372c7a05fbb6b5c847fe0cc','bytes':69905589},
    'user':{'ref':'ghcr.io/poryadok/voiceroot/dev-user@sha256:42b84cfe7766bdb702ac73f50f21dc92bd60456220d7e04d6e27c0f4bfc14a61',
            'config':'sha256:82c487aff83c170a888add5ba464680a6d342cb321f2c02e43cf2fd2f35ebf11',
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


def deadline_job(operation):
    pod=artifact_pod(operation,'user')
    pod['spec']['containers'][0]['command']=['/bin/sh','-ec','sleep 60']
    pod['spec']['terminationGracePeriodSeconds']=1
    return {'apiVersion':'batch/v1','kind':'Job','metadata':{
        'namespace':'voice-dev','name':'voice-db-deadline-'+operation,'labels':pod['metadata']['labels']},
        'spec':{'activeDeadlineSeconds':15,'backoffLimit':0,'parallelism':1,'completions':1,
                'template':{'metadata':{'labels':copy.deepcopy(pod['metadata']['labels'])},'spec':pod['spec']}}}


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
    require(isinstance(spec.get('containers'),list) and len(spec['containers'])==len(intended['spec']['containers']),
            'admitted probe container set changed')
    for index,container in enumerate(spec['containers']):
        for key,value in (('terminationMessagePath','/dev/termination-log'),('terminationMessagePolicy','File')):
            if key in container and key not in intended['spec']['containers'][index]:
                require(container.pop(key)==value,'unapproved container default')
    require(spec==intended['spec'],'admission changed credential-free probe spec')
    return copy.deepcopy(actual['spec'])


def admitted_job(intended,actual):
    require(actual.get('kind')=='Job' and actual.get('apiVersion')=='batch/v1','admitted deadline kind changed')
    metadata=actual.get('metadata',{});uid=metadata.get('uid')
    require(uid,'admitted deadline UID absent')
    for key in ('namespace','name','labels'):
        require(metadata.get(key)==intended['metadata'].get(key),'deadline identity changed')
    spec=copy.deepcopy(actual.get('spec',{}))
    generated={'batch.kubernetes.io/controller-uid':uid,'controller-uid':uid,
               'batch.kubernetes.io/job-name':metadata['name'],'job-name':metadata['name']}
    selector=spec.pop('selector',{})
    require(set(selector)=={'matchLabels'} and selector['matchLabels'] and
            all(key in ('batch.kubernetes.io/controller-uid','controller-uid') and value==uid
                for key,value in selector['matchLabels'].items()),'deadline selector is not actual UID')
    template=spec.get('template',{});labels=template.get('metadata',{}).get('labels',{})
    require(isinstance(labels,dict),'deadline labels absent')
    for key,value in generated.items():
        if key in labels:require(labels.pop(key)==value,'deadline controller label changed')
    expected_template=intended['spec']['template']
    if 'creationTimestamp' in template.get('metadata',{}):
        require(template['metadata'].pop('creationTimestamp') is None,'non-null deadline template creation timestamp injected')
    require(template.get('metadata')==expected_template['metadata'],'deadline template metadata injected')
    pod={'apiVersion':'v1','kind':'Pod','metadata':{'namespace':'voice-dev','name':'template',
         'labels':expected_template['metadata']['labels']},'spec':expected_template['spec']}
    admitted=copy.deepcopy(pod);admitted['spec']=template.get('spec',{})
    admitted_pod(pod,admitted)
    spec['template']=copy.deepcopy(expected_template)
    for key,value in (('completionMode','NonIndexed'),('suspend',False),('manualSelector',False),
                      ('podReplacementPolicy','TerminatingOrFailed')):
        if key in spec and key not in intended['spec']:require(spec.pop(key)==value,'unapproved Job default')
    require(spec==intended['spec'],'admitted deadline Job changed')
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


def artifact_receipt(pod,service,uid,logs):
    require(service in IMAGES,'unknown artifact service')
    item=IMAGES[service];metadata=pod.get('metadata',{});spec=pod.get('spec',{})
    require(metadata.get('uid')==uid and uid,'artifact Pod UID changed')
    require(not metadata.get('ownerReferences'),'scratch Pod adopted by a controller')
    require(spec.get('nodeName'),'actual artifact node absent')
    statuses=pod.get('status',{}).get('containerStatuses',[])
    require(pod.get('status',{}).get('phase')=='Succeeded' and len(statuses)==1,'artifact container not terminal success')
    status=statuses[0];terminal=status.get('state',{}).get('terminated',{})
    require(status.get('name')=='artifact' and status.get('containerID') and terminal.get('exitCode')==0,
            'actual artifact container identity/exit absent')
    require(terminal.get('startedAt') and terminal.get('finishedAt'),'actual artifact start/finish absent')
    allowed={item['ref'],item['ref'].split('@')[1],item['config'],
             'docker-pullable://'+item['ref'],'docker://'+item['config'],'containerd://'+item['config']}
    require(status.get('imageID') in allowed,'actual node image is not admitted immutable candidate')
    require(isinstance(logs,str) and len(logs)<=1024,'artifact log bound exceeded')
    lines=logs.strip().splitlines()
    require(len(lines)==2 and lines[0].split()==[item['sha256'],item['path']] and lines[1].strip()==str(item['bytes']),
            'actual node artifact bytes/hash mismatch')
    return {'pod_uid':uid,'node':spec['nodeName'],'container_id':status['containerID'],
            'image_id':status['imageID'],'sha256':item['sha256'],'bytes':item['bytes']}


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


def deadline_start(pod,uid):
    """Actual container identity, also comparable after its terminal transition."""
    metadata=pod.get('metadata',{});owners=[o for o in metadata.get('ownerReferences',[]) if o.get('controller')]
    require(len(owners)==1 and owners[0].get('uid')==uid and owners[0].get('kind')=='Job','deadline start owner changed')
    statuses=pod.get('status',{}).get('containerStatuses',[])
    require(len(statuses)==1 and statuses[0].get('name')=='artifact','deadline start container missing')
    status=statuses[0];state=status.get('state',{})
    require(pod.get('status',{}).get('phase') in ('Running','Failed') and
            (state.get('running') or state.get('terminated')) and not status.get('restartCount',0),
            'deadline container has not actually started or restarted')
    item=IMAGES['user'];allowed={item['ref'],item['ref'].split('@')[1],item['config'],
        'docker-pullable://'+item['ref'],'docker://'+item['config'],'containerd://'+item['config']}
    require(status.get('imageID') in allowed,'deadline start image is not admitted candidate')
    result={'job_uid':uid,'pod_uid':metadata.get('uid'),'pod_name':metadata.get('name'),
        'node':pod.get('spec',{}).get('nodeName'),'container_id':status.get('containerID'),
        'image_id':status['imageID'],'started_at':(state.get('running') or state.get('terminated')).get('startedAt')}
    require(all(isinstance(v,str) and 0<len(v)<=512 for v in result.values()),'deadline start identity incomplete')
    return result


def absent_deadline_receipt(job,started,child,uid):
    # A Job condition alone never proves that an admitted sleep container ran.
    keys={'job_uid','pod_uid','pod_name','node','container_id','image_id','started_at'}
    require(isinstance(started,dict) and set(started)==keys and started['job_uid']==uid and
            child=={'uid':started['pod_uid'],'name':started['pod_name']} and
            all(isinstance(v,str) and 0<len(v)<=512 for v in started.values()),'durable deadline start identity absent')
    status=job.get('status',{});conditions=status.get('conditions',[])
    failed=[c for c in conditions if c.get('type')=='Failed' and c.get('status')=='True']
    require(job.get('metadata',{}).get('uid')==uid and len(failed)==1 and
            failed[0].get('reason')=='DeadlineExceeded' and not status.get('active') and status.get('failed',0)>=1 and
            not any(c.get('type')=='Complete' and c.get('status')=='True' for c in conditions),
            'same actual terminal deadline Job required after child absence')
    try:
        began=datetime.datetime.fromisoformat(started['started_at'].replace('Z','+00:00'))
        ended=datetime.datetime.fromisoformat(failed[0]['lastTransitionTime'].replace('Z','+00:00'))
        require(began.tzinfo and ended.tzinfo and (ended-began).total_seconds()>=1,'deadline start/failure order invalid')
    except (KeyError,TypeError,ValueError):
        raise RecoveryRequired('actual deadline failure timestamp absent') from None
    return {**started,'deadline_at':failed[0]['lastTransitionTime'],'reason':'DeadlineExceeded',
            'child_terminal_evidence':'absent_after_deadline'} # No invented container finishedAt.


class ProbeController:
    """Write-ahead scratch creation. Uncertain effects require reconciliation.

    guard is the real current-window/baseline verifier, not an approval flag.
    Journal supplies the existing private fsync/lock/immutable-intent contract.
    """
    def __init__(self,journal,kube,guard):
        self.journal=journal;self.kube=kube;self.guard=guard
        self.intent=journal.state['intent'];self.operation=operation_id(self.intent['operation'])
        self._validate_records()

    def _validate_records(self):
        for service,record in self.journal.state.get('probe',{}).items():
            require(service in IMAGES and record.get('body')==artifact_pod(self.operation,service),
                    'saved scratch body differs from fixed source contract')
            if record.get('admitted_spec'):
                actual=copy.deepcopy(record['body']);actual['spec']=record['admitted_spec'];admitted_pod(record['body'],actual)
        record=self.journal.state.get('probe_deadline')
        if record:
            require(record.get('body')==deadline_job(self.operation),'saved deadline body differs from fixed source contract')
            if record.get('spec'):
                actual=copy.deepcopy(record['body']);actual['metadata']['uid']=record.get('uid');actual['spec']=record['spec']
                admitted_job(record['body'],actual)

    def _get(self,kind,name):
        return self.kube.call('get',kind,name,'--ignore-not-found=true','-o','json')

    def _note_created(self,record,current):
        # Hints aid recovery if admission/ownership validation fails. They do
        # not authorize cleanup or replace the expected immutable source body.
        metadata=current.get('metadata',{})
        for key,target in (('uid','returned_uid_hint'),('resourceVersion','returned_version_hint')):
            value=metadata.get(key)
            if isinstance(value,str) and value and len(value)<=200:record[target]=value
        record['returned_object_sha256']=digest(current);self.journal.save()

    def _lock_body(self):
        resource=self.intent['resource']
        return {'apiVersion':'v1','kind':'ConfigMap','metadata':{'namespace':'voice-dev','name':LOCK,
                'labels':{'voice.test.owner':'Dimas-16','voice.probe.operation':self.operation}},
                'data':{'owner':'Dimas-16','operation':self.operation,'purpose':'auth-user-candidate-probe',
                    'claim':resource['claim'],'generation':str(resource['generation']),'window':resource['window']}}

    def _check_lock(self,current,record=None):
        expected=self._lock_body();metadata=current.get('metadata',{})
        require(current.get('kind')=='ConfigMap' and metadata.get('name')==LOCK and metadata.get('namespace')=='voice-dev' and
                not metadata.get('ownerReferences') and not current.get('binaryData'),'common lock kind/owner/projection changed')
        finalizers=metadata.get('finalizers',[])
        require(not finalizers or (finalizers==['foregroundDeletion'] and metadata.get('deletionTimestamp') and
                (self.journal.state.get('probe_finish_pending') or self.journal.state.get('probe_abort_pending'))),
                'unapproved common lock finalizer')
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
        self._note_created(self.journal.state['probe_lock'],current)
        self._check_lock(current)
        record={'state':'done','uid':current['metadata']['uid'],
                'resource_version':current['metadata']['resourceVersion']}
        self.journal.state['probe_lock']=record;self.journal.save()
        return record

    def _guard(self):
        self._validate_records()
        self.guard()
        record=self.journal.state.get('probe_lock',{})
        require(record.get('state')=='done','actual common updater lock required before scratch writes')
        self._check_lock(self._get('configmap',LOCK),record)

    def reconcile_delete_cas(self,record,current):
        if record['state']!='deleting':return
        metadata=current['metadata'];old=record.get('delete_options',{}).get('preconditions',{})
        require(old.get('uid')==record.get('uid')==metadata['uid'] and old.get('resourceVersion'),
                'pending DELETE precondition identity missing')
        # An exact fresh GET with a different version and no deletion marker
        # proves the old RV-fenced request can no longer affect this object.
        # Same-version/marked-deleting state remains pending, never repeated.
        pending_unless(not metadata.get('deletionTimestamp') and metadata['resourceVersion']!=old['resourceVersion'],
                       'old exact DELETE still pending; do not repeat')
        record.setdefault('delete_cas_reconciliations',[]).append({
            'old_version':old['resourceVersion'],'observed_version':metadata['resourceVersion']})
        record['state']=record.get('delete_from_state','observed' if record.get('receipt') else 'created')
        self.journal.save()

    def reconcile_lock(self):
        self.guard()
        record=self.journal.state.get('probe_lock',{})
        require(record.get('state')=='pending','no uncertain lock creation intent')
        current=self._get('configmap',LOCK)
        require(current,'lock creation remains uncertain/absent; do not repeat or delete by name')
        self._check_lock(current)
        require(not record.get('returned_uid_hint') or current['metadata']['uid']==record['returned_uid_hint'],
                'common lock changed from returned creation hint')
        self.journal.state['probe_lock']={'state':'done','uid':current['metadata']['uid'],
            'resource_version':current['metadata']['resourceVersion'],'reconciled_lost_response':True}
        self.journal.save()

    def reconcile_creation(self,target):
        self._guard()
        require(target in ('auth','user','deadline'),'unknown creation reconciliation target')
        record=(self.journal.state.get('probe_deadline',{}) if target=='deadline' else
                self.journal.state.get('probe',{}).get(target,{}))
        require(record.get('state')=='pending','no uncertain scratch creation intent')
        kind='job' if target=='deadline' else 'pod'
        current=self._get(kind,record['body']['metadata']['name'])
        require(current,'creation remains uncertain/absent; no automatic repeat or name cleanup')
        metadata=current.get('metadata',{})
        expected_uid=record.get('uid') or record.get('returned_uid_hint')
        require(metadata.get('uid') and metadata.get('resourceVersion') and
                (not expected_uid or expected_uid==metadata['uid']),'reconciled creation identity changed')
        try:
            created=datetime.datetime.fromisoformat(metadata['creationTimestamp'].replace('Z','+00:00')).timestamp()
            require(created>=self.intent['prepared_at']-1 and created<=time.time()+30,'creation predates operation or has future clock')
        except (KeyError,TypeError,ValueError):raise RecoveryRequired('actual creation timestamp absent') from None
        if target=='deadline':
            admitted_job(record['body'],current);record['spec']=current['spec']
        else:
            checked=copy.deepcopy(current);checked.get('spec',{}).pop('nodeName',None)
            admitted_pod(record['body'],checked)
            require(checked['spec']==record['admitted_spec'] and not metadata.get('ownerReferences'),
                    'reconciled scratch spec/owner differs from admitted creation')
        record.update(state='created',uid=metadata['uid'],resource_version=metadata['resourceVersion'],reconciled_lost_response=True)
        self.journal.save()

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
        self._note_created(records[service],actual)
        admitted_pod(body,actual)
        require(actual['spec']==spec,'created probe differs from dry-run spec')
        metadata=actual.get('metadata',{})
        require(metadata.get('uid') and metadata.get('resourceVersion'),'actual probe creation UID absent')
        records[service].update(state='created',uid=metadata['uid'],resource_version=metadata['resourceVersion'])
        self.journal.save()
        return copy.deepcopy(records[service])

    def cleanup_artifact(self,service):
        self._guard()
        record=self.journal.state.get('probe',{}).get(service,{})
        require(record.get('state') in ('created','observed','deleting','absent') and record.get('uid'),
                'artifact identity uncertain; no automatic cleanup')
        name=record['body']['metadata']['name'];actual=self._get('pod',name)
        if not actual:
            record['state']='absent';self.journal.save();return
        options=delete_options(actual,self.operation,record['uid'])
        require(not actual.get('metadata',{}).get('ownerReferences'),'scratch Pod adopted by a controller')
        checked=copy.deepcopy(actual);checked.get('spec',{}).pop('nodeName',None)
        admitted_pod(record['body'],checked)
        require(record['state']!='absent','scratch replacement observed after recorded absence')
        self.reconcile_delete_cas(record,actual)
        record['delete_from_state']=record['state'];record['state']='deleting';record['delete_options']=options;self.journal.save()
        self._guard()
        self.kube.call('delete','--raw=/api/v1/namespaces/voice-dev/pods/'+name,'-f','-',body=options)
        pending_unless(not self._get('pod',name),'scratch deletion acknowledged but absence not observed')
        record['state']='absent';self.journal.save()

    def observe_artifact(self,service):
        self._guard()
        record=self.journal.state.get('probe',{}).get(service,{})
        require(record.get('state') in ('created','observed') and record.get('uid'),'artifact creation uncertain')
        name=record['body']['metadata']['name'];actual=self._get('pod',name)
        require(actual.get('metadata',{}).get('uid')==record['uid'],'artifact Pod replaced or absent')
        checked=copy.deepcopy(actual);checked.get('spec',{}).pop('nodeName',None)
        admitted_pod(record['body'],checked)
        require(not actual.get('metadata',{}).get('ownerReferences'),'scratch Pod adopted by controller')
        phase=actual.get('status',{}).get('phase')
        require(phase not in ('Failed','Unknown'),'artifact Pod failed; retain observed identity for cleanup')
        if phase!='Succeeded':return None
        logs=self.kube.logs(name)
        after=self._get('pod',name)
        require(after.get('metadata',{}).get('uid')==record['uid'],'artifact identity changed during log read')
        checked=copy.deepcopy(after);checked.get('spec',{}).pop('nodeName',None);admitted_pod(record['body'],checked)
        receipt=artifact_receipt(after,service,record['uid'],logs)
        record.update(state='observed',receipt=receipt);self.journal.save();return copy.deepcopy(receipt)

    def create_deadline(self,selectors):
        self._guard()
        require('probe_deadline' not in self.journal.state,'deadline creation already intended; reconcile without repeating')
        body=deadline_job(self.operation)
        exclude_selectors(body['spec']['template']['metadata']['labels'],selectors)
        current=self.kube.call('get','services,deployments,replicasets,statefulsets,daemonsets,replicationcontrollers,jobs','-o','json')
        require(isinstance(current.get('items'),list),'current selector inventory absent')
        exclude_selectors(body['spec']['template']['metadata']['labels'],current['items'])
        admitted=self.kube.call('create','--dry-run=server','-f','-','-o','json',body=body)
        admitted_job(body,admitted)
        self._guard()
        record={'state':'pending','body':body};self.journal.state['probe_deadline']=record;self.journal.save()
        actual=self.kube.call('create','-f','-','-o','json',body=body)
        self._note_created(record,actual)
        admitted_job(body,actual)
        require(actual.get('metadata',{}).get('resourceVersion'),'actual deadline Job version absent')
        record.update(state='created',uid=actual['metadata']['uid'],spec=actual['spec'])
        self.journal.save();return copy.deepcopy(record)

    def _deadline_children(self,record):
        result=self.kube.call('get','pods','-o','json')
        require(isinstance(result.get('items'),list),'actual deadline Pod inventory absent')
        children=[]
        for pod in result['items']:
            if not any(o.get('uid')==record['uid'] for o in pod.get('metadata',{}).get('ownerReferences',[])):continue
            owners=[o for o in pod['metadata']['ownerReferences'] if o.get('controller')]
            require(len(owners)==1 and owners[0].get('uid')==record['uid'] and owners[0].get('kind')=='Job',
                    'deadline child controller changed')
            metadata=pod['metadata'];name=metadata.get('name','')
            require(name.startswith(record['body']['metadata']['name']+'-') and metadata.get('uid'),
                    'deadline child creation identity invalid')
            expected={'apiVersion':'v1','kind':'Pod','metadata':{'namespace':'voice-dev','name':name,
                'labels':record['spec']['template']['metadata']['labels']},'spec':record['spec']['template']['spec']}
            checked=copy.deepcopy(pod);checked.get('spec',{}).pop('nodeName',None)
            admitted_pod(expected,checked)
            children.append(pod)
        return children

    def observe_deadline(self):
        self._guard()
        record=self.journal.state.get('probe_deadline',{})
        require(record.get('state') in ('created','observed') and record.get('uid'),'deadline creation uncertain')
        actual=self._get('job',record['body']['metadata']['name'])
        require(actual.get('metadata',{}).get('uid')==record['uid'],'deadline Job identity changed')
        admitted_job(record['body'],actual)
        require(actual['spec']==record['spec'],'actual admitted Job spec drifted')
        conditions=actual.get('status',{}).get('conditions',[])
        require(not any(c.get('type')=='Complete' and c.get('status')=='True' for c in conditions),
                'deadline Job completed without enforcing deadline')
        failed=any(c.get('type')=='Failed' and c.get('status')=='True' for c in conditions)
        children=self._deadline_children(record)
        require(len(children)<=1,'deadline owned Pod set unexpected')
        if children:
            child=children[0];identity={k:child['metadata'][k] for k in ('uid','name')}
            require(not record.get('child_identity') or record['child_identity']==identity,'deadline child replaced')
            if not record.get('child_identity'):
                record['child_identity']=identity;self.journal.save()
            if record.get('started_observation'):
                require(deadline_start(child,record['uid'])==record['started_observation'],'deadline started container changed')
            elif child.get('status',{}).get('phase')=='Running':
                require(not failed,'deadline start was not observed before terminal Job')
                record['started_observation']=deadline_start(child,record['uid']);self.journal.save()
        elif record.get('child_identity'):
            require(not self._get('pod',record['child_identity']['name']),'saved deadline child still present or replaced')
        if not failed:return None
        if children:receipt=deadline_receipt(actual,children,record['uid'])
        else:receipt=absent_deadline_receipt(actual,record.get('started_observation'),record.get('child_identity'),record['uid'])
        record.update(state='observed',receipt=receipt);self.journal.save();return copy.deepcopy(receipt)

    def cleanup_deadline(self):
        self._guard()
        record=self.journal.state.get('probe_deadline',{})
        require(record.get('state') in ('observed','deleting','absent') and record.get('receipt'),
                'actual sleep/deadline/terminal proof required before qualified cleanup')
        name=record['body']['metadata']['name'];actual=self._get('job',name)
        children=self._deadline_children(record)
        if not actual:
            pending_unless(not children,'deadline child garbage collection not complete')
            record['state']='absent';self.journal.save();return
        options=delete_options(actual,self.operation,record['uid'])
        admitted_job(record['body'],actual)
        require(actual['spec']==record['spec'],'deadline cleanup spec drift')
        for child in children:
            statuses=child.get('status',{}).get('containerStatuses',[])
            require(child['metadata']['uid']==record['receipt']['pod_uid'] and child.get('status',{}).get('phase')=='Failed' and
                len(statuses)==1 and statuses[0].get('containerID')==record['receipt']['container_id'] and all(
                c.get('state',{}).get('terminated') for c in statuses),
                'deadline child not terminal before cleanup')
        require(record['state']!='absent','deadline replacement observed after recorded absence')
        self.reconcile_delete_cas(record,actual)
        require(record['state']=='observed','qualified deadline receipt state changed')
        record.update(delete_from_state='observed',state='deleting',delete_options=options);self.journal.save();self._guard()
        self.kube.call('delete','--raw=/apis/batch/v1/namespaces/voice-dev/jobs/'+name,'-f','-',body=options)
        pending_unless(not self._get('job',name) and not self._deadline_children(record),
                'deadline Job/children absence not observed')
        record['state']='absent';self.journal.save()

    def cancel_deadline(self):
        """Abort only the admitted credential-free sleep Job, never claim PASS."""
        self._guard()
        record=self.journal.state.get('probe_deadline',{})
        require(record.get('uid') and record.get('state') in ('created','observed','deleting','absent'),
                'deadline identity uncertain; explicit reconciliation required')
        name=record['body']['metadata']['name'];actual=self._get('job',name)
        children=self._deadline_children(record) # Validates exact owner/spec even when running.
        if not actual:
            pending_unless(not children,'cancelled deadline children not absent')
            record.update(state='absent',cancelled=True);self.journal.save();return
        options=delete_options(actual,self.operation,record['uid']);admitted_job(record['body'],actual)
        require(actual['spec']==record['spec'],'deadline cancellation spec changed')
        require(record['state']!='absent','deadline reappeared after absence')
        self.reconcile_delete_cas(record,actual)
        record.update(delete_from_state=record['state'],state='deleting',cancelled=True,delete_options=options);self.journal.save();self._guard()
        self.kube.call('delete','--raw=/apis/batch/v1/namespaces/voice-dev/jobs/'+name,'-f','-',body=options)
        pending_unless(not self._get('job',name) and not self._deadline_children(record),'cancelled deadline Job/children not absent')
        record['state']='absent';self.journal.save()

    def finish(self):
        # Plain guard permits reconciliation of a lost exact-lock DELETE
        # response after its actual absence; all other proofs remain mandatory.
        require(not self.journal.state.get('probe_abort_requested') and not self.journal.state.get('probe_abort_pending') and not self.journal.state.get('probe_aborted'),
                'aborted probe cannot publish qualified artifact/controller PASS')
        self.guard()
        artifacts=self.journal.state.get('probe',{})
        require(set(artifacts)=={'auth','user'} and all(
            record.get('state')=='absent' and record.get('receipt') for record in artifacts.values()),
            'both actual artifact receipts and absence required')
        for record in artifacts.values():
            require(not self._get('pod',record['body']['metadata']['name']),'scratch Pod still present or replaced')
        deadline=self.journal.state.get('probe_deadline',{})
        require(deadline.get('state')=='absent' and deadline.get('receipt'),'deadline receipt and cleanup required')
        require(not self._get('job',deadline['body']['metadata']['name']) and not self._deadline_children(deadline),
                'deadline Job/children still present')
        self.release_lock('finish')
        self.guard()
        receipt={'protocol':'voice-auth-user-node-probe-v1','operation':self.operation,'result':'PASS',
            'candidate_source':PRODUCT,'probe_source':self.intent['binding']['probe_source'],
            'probe_fingerprint':self.intent['fingerprint'],'resource':self.intent['resource'],
            'original_baseline_sha256':digest(self.intent['baseline']),
            'finished_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),
            'artifacts':{name:record['receipt'] for name,record in artifacts.items()},
            'deadline':deadline['receipt'],'owned_objects_absent':True,
            'scope':'actual node artifact/controller proof only; no application rollout or DB adoption'}
        self.journal.state['probe_finished']=receipt;self.journal.save();return copy.deepcopy(receipt)

    def release_lock(self,phase):
        require(phase in ('finish','abort'),'unknown lock release purpose')
        self.guard();key='probe_'+phase+'_pending';current=self._get('configmap',LOCK)
        record=self.journal.state.get('probe_lock',{})
        if not current:
            require(self.journal.state.get(key),'common lock disappeared without own release intent');return
        if self.journal.state.get(key):
            self._check_lock(current)
            require(current['metadata']['uid']==record.get('uid'),'common lock replaced while releasing')
            if not current['metadata'].get('deletionTimestamp'):
                require(current['metadata']['resourceVersion']==record['resource_version'],
                        'common lock version changed outside observed own deletion')
            raise ProbePending('owned common lock release has not reached absence; do not repeat DELETE')
        self._check_lock(current,record);options=delete_options(current,self.operation,record['uid'])
        self.journal.state[key]=True;self.journal.state['probe_'+phase+'_delete_options']=options
        self.journal.save();self.guard()
        self.kube.call('delete','--raw=/api/v1/namespaces/voice-dev/configmaps/'+LOCK,'-f','-',body=options)
        pending_unless(not self._get('configmap',LOCK),'owned common lock release not yet absent')


def wait_for(action,seconds,receipt=False,sleep=time.sleep,clock=time.monotonic):
    deadline=clock()+seconds
    while True:
        try:
            result=action()
            if not receipt or result is not None:return result
        except ProbePending:
            pass # Only known owned pending state is polled; drift/transport stop.
        require(clock()<deadline,'bounded probe wait exhausted; reconcile exact owned server state')
        sleep(2)


def run_probe(journal,kube,guard):
    require(journal.state['intent'].get('protocol')==PROTOCOL and not journal.state.get('probe_abort_requested') and
            not journal.state.get('probe_abort_pending') and not journal.state.get('probe_aborted'),
            'existing live probe journal required')
    if journal.state.get('probe_finished'):
        guard();return copy.deepcopy(journal.state['probe_finished'])
    controller=ProbeController(journal,kube,guard)
    if journal.state.get('probe_finish_pending'):
        return wait_for(controller.finish,60,receipt=True)
    controller.acquire_lock()
    for service in ('auth','user'):
        if service not in journal.state.get('probe',{}):controller.create_artifact(service,[])
        record=journal.state['probe'][service]
        require(record['state']!='pending','creation response uncertain; explicit reconcile before continuation')
        if record['state']=='created':wait_for(lambda:controller.observe_artifact(service),120,receipt=True)
        if record['state'] in ('observed','deleting'):wait_for(lambda:controller.cleanup_artifact(service),60)
        require(record['state']=='absent' and record.get('receipt'),'artifact proof/absence incomplete')
    if 'probe_deadline' not in journal.state:controller.create_deadline([])
    record=journal.state['probe_deadline']
    require(record['state']!='pending','deadline creation uncertain; explicit reconcile required')
    if record['state']=='created':wait_for(controller.observe_deadline,90,receipt=True)
    if record['state'] in ('observed','deleting'):wait_for(controller.cleanup_deadline,60)
    require(record['state']=='absent' and record.get('receipt'),'actual deadline/child absence incomplete')
    return wait_for(controller.finish,60,receipt=True)


def abort_probe(journal,kube,guard):
    require(journal.state['intent'].get('protocol')==PROTOCOL and not journal.state.get('probe_finished'),
            'unfinished original probe journal required for abort')
    guard();controller=ProbeController(journal,kube,guard)
    if journal.state.get('probe_aborted'):return copy.deepcopy(journal.state['probe_aborted'])
    journal.state['probe_abort_requested']=True;journal.save()
    lock_record=journal.state.get('probe_lock')
    if lock_record:
        if lock_record['state']=='pending':controller.reconcile_lock()
        for service,record in journal.state.get('probe',{}).items():
            if record['state']=='pending':controller.reconcile_creation(service)
            wait_for(lambda:controller.cleanup_artifact(service),60)
        deadline=journal.state.get('probe_deadline')
        if deadline:
            if deadline['state']=='pending':controller.reconcile_creation('deadline')
            wait_for(controller.cancel_deadline,60)
        wait_for(lambda:controller.release_lock('abort'),60)
    else:
        require(not journal.state.get('probe') and not journal.state.get('probe_deadline'),'scratch records exist without lock custody')
        require(not controller._get('configmap',LOCK),'foreign common lock prevents unchanged release proof')
    guard()
    result={'protocol':PROTOCOL,'operation':controller.operation,'result':'ABORTED','owned_objects_absent':True,
            'scope':'no candidate/application/database adoption or deadline PASS'}
    journal.state['probe_aborted']=result;journal.save();return result


def read_binding(path):
    path=pathlib.Path(path).absolute()
    require(not any(parent.is_symlink() for parent in (path,*path.parents)),'binding alias refused')
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    with os.fdopen(fd,'rb') as stream:
        info=os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_size<=16384,'binding is not a bounded regular file')
        return json.loads(stream.read(16385).decode('utf-8-sig'))


def main(argv=None):
    parser=argparse.ArgumentParser(description=__doc__)
    commands=parser.add_subparsers(dest='command',required=True)
    commands.add_parser('fingerprint',help='Print exact source fingerprint; no live access')
    prepare=commands.add_parser('prepare',help='Read-only current DEV/resource baseline and private journal')
    prepare.add_argument('--binding',required=True);prepare.add_argument('--journal',required=True)
    for name in ('run','abort','inspect','reconcile','refresh'):
        command=commands.add_parser(name);command.add_argument('--journal',required=True)
        if name=='reconcile':command.add_argument('--target',choices=('lock','auth','user','deadline'),required=True)
        if name=='refresh':command.add_argument('--reason',required=True)
    args=parser.parse_args(argv);journal=None
    try:
        require(sys.platform.startswith('linux'),'probe controller must run on Linux')
        if args.command=='fingerprint':
            result={'protocol':PROTOCOL,'fingerprint':probe_fingerprint(),'candidate_source':PRODUCT,
                    'images':IMAGES,'live_access':False}
        elif args.command=='prepare':
            result=prepare_probe(args.journal,read_binding(args.binding),ProbeKube())
        else:
            journal=Journal(args.journal,None)
            intent=journal.state['intent']
            require(intent.get('protocol')==PROTOCOL,'wrong operation journal protocol')
            if args.command=='inspect':
                result={'result':'HISTORICAL_STATE','operation':intent['operation'],
                    'fingerprint':intent['fingerprint'],'source_unchanged':intent['fingerprint']==probe_fingerprint(),
                    'lock_state':journal.state.get('probe_lock',{}).get('state'),
                    'artifacts':{key:value['state'] for key,value in journal.state.get('probe',{}).items()},
                    'deadline_state':journal.state.get('probe_deadline',{}).get('state'),
                    'finished':bool(journal.state.get('probe_finished')),'aborted':bool(journal.state.get('probe_aborted'))}
            else:
                kube=ProbeKube();guard=WindowGuard(journal,kube)
                if args.command=='run':result=run_probe(journal,kube,guard)
                elif args.command=='abort':result=abort_probe(journal,kube,guard)
                elif args.command=='refresh':
                    guard.refresh(args.reason);result={'result':'REFRESHED_SAME_CLAIM','operation':intent['operation']}
                else:
                    controller=ProbeController(journal,kube,guard)
                    if args.target=='lock':controller.reconcile_lock()
                    else:controller.reconcile_creation(args.target)
                    result={'result':'RECONCILED_EXISTING_IDENTITY','operation':intent['operation'],'target':args.target}
        print(json.dumps(result,sort_keys=True));return 0
    except BaseException as error:
        print(json.dumps({'result':'INCOMPLETE','reason':str(error) if isinstance(error,RecoveryRequired) else 'probe controller input/transport failed',
            'action':'Retain private journal and actual resource/owned lock custody; reconcile exact identities before retry or release'}))
        return 1
    finally:
        if journal:journal.close()


if __name__=='__main__':raise SystemExit(main())
