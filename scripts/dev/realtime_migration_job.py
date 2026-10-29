"""Fixed Realtime migration inputs. Rendering is not authority or execution."""
import hashlib
import copy
import json
from pathlib import Path
import re
import secrets
import time
import auth_user_candidate_probe
import media_operation_guard
import publish_candidate
import realtime_db_provision
import realtime_migration_verify
import realtime_secret_phase

MIGRATE_IMAGE = 'migrate/migrate@sha256:f21c436af23c282f4516b00ba3e93bccf5c5fe5cd52530fd5c319a936998f539'
PLATFORMS = {'amd64':'sha256:a7a70ebfa0be59cc9b80e405f08c25e8ab746b1c9555c58fc2369abc2c84acab',
             'arm64':'sha256:88330a26116ba4bfa4855901b5d0920112e60fa2242ec8aabba5af995ed47d8f'}

class RecoveryRequired(RuntimeError):
    pass


def render(source_root, source_sha, operation, secret, migrate_image, node):
    try:
        match = re.fullmatch(r'media-c[12]-([0-9a-f]{32})',operation) if isinstance(operation,str) else None
        if (not match or not isinstance(source_sha,str) or not re.fullmatch(r'[0-9a-f]{40}',source_sha) or
                not isinstance(secret,dict) or set(secret)!={'name','uid','resourceVersion'} or
                secret['name']!='voice-realtime-db-'+match.group(1) or
                any(not isinstance(secret[k],str) or not secret[k] for k in ('uid','resourceVersion')) or
                migrate_image!=MIGRATE_IMAGE or not isinstance(node,dict) or
                set(node)!={'name','uid','hostname','architecture'} or node.get('architecture') not in PLATFORMS or
                any(not isinstance(node.get(k),str) or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?',node[k])
                    for k in ('name','hostname')) or not isinstance(node.get('uid'),str) or not node['uid']):
            raise RecoveryRequired('migration identity invalid')
        snapshot = publish_candidate.source_snapshot(Path(source_root),source_sha)
        prefix = 'src/backend/migrations/realtime_db/'
        names = {'000001_admission.up.sql','000001_admission.down.sql'}
        files = snapshot['files']
        if snapshot['source_sha']!=source_sha or {k[len(prefix):] for k in files if k.startswith(prefix)} != names:
            raise RecoveryRequired('migration source inventory changed')
        data = {}
        for name in sorted(names):
            path = Path(source_root)/prefix/name
            raw = path.read_bytes()
            if hashlib.sha256(raw).hexdigest()!=files[prefix+name]:
                raise RecoveryRequired('migration source changed')
            data[name] = raw.decode('utf-8')
        suffix = match.group(1)
        config_name = 'voice-realtime-migrations-'+suffix
        name = 'voice-realtime-migrate-'+suffix
        config = {'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':config_name,'namespace':'voice-dev',
                  'annotations':{'voice.io/source-sha':source_sha}},'immutable':True,'data':data}
        planned = {'apiVersion':'batch/v1','kind':'Job','metadata':{'name':name,'namespace':'voice-dev',
                   'annotations':{'voice.io/source-sha':source_sha}},'spec':{
            'backoffLimit':0,'parallelism':1,'completions':1,'activeDeadlineSeconds':600,'template':{'metadata':{'labels':{'app':name}},'spec':{
                'restartPolicy':'Never','automountServiceAccountToken':False,'terminationGracePeriodSeconds':30,
                'nodeSelector':{'kubernetes.io/os':'linux','kubernetes.io/arch':node['architecture'],
                                'kubernetes.io/hostname':node['hostname']},
                'enableServiceLinks':True,'securityContext':{},'containers':[{'name':'migrate',
                'image':migrate_image,'imagePullPolicy':'IfNotPresent','command':['/bin/sh','-ec','exec migrate -path=/migrations -database "$DATABASE_URL" up'],
                'env':[{'name':'DATABASE_URL','valueFrom':{'secretKeyRef':{'name':secret['name'],'key':'DATABASE_URL'}}}],
                'volumeMounts':[{'name':'migrations','mountPath':'/migrations','readOnly':True}],
                'resources':{'requests':{'cpu':'20m','memory':'64Mi'},'limits':{'cpu':'500m','memory':'512Mi'}}}],
                'volumes':[{'name':'migrations','configMap':{'name':config_name,'optional':False,'defaultMode':420}}]}}}}
        return config, planned
    except BaseException:
        raise RecoveryRequired('Realtime migration plan refused') from None


def _hash(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


def admitted_job(intended, actual):
    """Accept only the observed Kubernetes Job labels tied to its own UID."""
    value = copy.deepcopy(actual)
    meta = value.get('metadata',{})
    uid, name = meta.get('uid'), intended['metadata']['name']
    expected = {'app':name, 'batch.kubernetes.io/controller-uid':uid, 'controller-uid':uid,
                'batch.kubernetes.io/job-name':name, 'job-name':name}
    if not isinstance(uid,str) or not uid or meta.get('labels')!=expected:
        raise RecoveryRequired('Job generated metadata labels invalid')
    meta.pop('labels')
    return auth_user_candidate_probe.admitted_job(intended,value)


class Migration:
    """One-shot guarded create and verification; uncertain outcomes retain lock.

    Caller must authenticate the candidate/source and OCI platform association
    in the immutable migration journal before constructing this phase. Secret
    and provision journals belong to the same fenced operation. No cleanup or
    retry is attempted after any pending write.
    """
    def __init__(self, secret, provision_journal, journal, source_root, password):
        self.secret, self.provision_journal, self.journal = secret, provision_journal, journal
        self.provision_intent = copy.deepcopy(provision_journal.intent)
        self.root, self.password = Path(source_root), password
        self.intent = copy.deepcopy(journal.intent)
        self.fence, self.client = secret.fence, secret.client
        self.lock, self.namespace = secret.lock, secret.namespace_uid
        self.invocation = secrets.token_hex(16)
        self.poisoned = False
        self.bound_secret = self.bound_provision = None
        self.migration_steps = None
        self.invocation_status = 'pending'

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent!=self.intent or
                self.journal.state.get('intent')!=self.intent or
                self.intent.get('resource_context')!=self.fence.context or
                self.intent.get('namespace_uid')!=self.namespace):
            raise RecoveryRequired('operation identity changed')
        self.secret._guard()
        if self.bound_secret is not None:
            row, revision = self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status')!='active' or
                    row.get('realtime_secret')!=self.bound_secret or
                    row.get('realtime_db_provision')!=self.bound_provision or
                    self.provision_journal.poisoned or self.provision_journal.intent!=self.provision_intent or
                    self.provision_journal.state.get('intent')!=self.provision_intent or
                    self.provision_journal.state['steps'].get('realtime-db-invocation') !=
                        {'status':'pending','nonce':self.bound_provision['nonce']} or
                    self.provision_journal.state['steps'].get('realtime-db-role') !=
                        self.bound_provision['steps']['role'] or
                    self.secret.journal.state['steps'].get('realtime-secret') !=
                        self.bound_secret | {'password_sha256':hashlib.sha256(self.password).hexdigest()} or
                    self.provision_journal.state['steps'].get('realtime-db-database') !=
                        self.bound_provision['steps']['database'] or
                    not realtime_secret_phase.valid_revision(revision)):
                raise RecoveryRequired('credential or provision authority changed')
            if self.migration_steps is not None:
                marker = {'operation_id':self.fence.context['operation_id'], 'nonce':self.invocation,
                          'status':self.invocation_status, 'steps':self.migration_steps}
                if (row.get('realtime_migration')!=marker or
                        self.journal.state['steps'].get('realtime-migration') !=
                            {'status':self.invocation_status,'nonce':self.invocation}):
                    raise RecoveryRequired('migration invocation changed')
                for name, record in self.migration_steps.items():
                    local = self.journal.state['steps'].get(name)
                    if (not isinstance(local,dict) or {k:v for k,v in local.items() if k!='receipt'}!=record or
                            (record.get('status')=='done' and _hash(local.get('receipt'))!=record.get('receipt_sha256'))):
                        raise RecoveryRequired('migration local receipt changed')

    def _owned(self,row):
        marker = row.get('realtime_migration',{})
        return (self.fence.owned(row) and row.get('status')=='active' and
                marker.get('operation_id')==self.fence.context['operation_id'] and
                marker.get('nonce')==self.invocation)

    def _check_step(self,name,expected,receipt=None):
        self._guard()
        row, revision = self.fence.ledger.read()
        local = self.journal.state['steps'].get(name)
        wanted_local = expected if receipt is None else expected | {'receipt':receipt}
        if (not self._owned(row) or row['realtime_migration']['steps'].get(name)!=expected or
                local!=wanted_local or not realtime_secret_phase.valid_revision(revision)):
            raise RecoveryRequired('migration step authority changed')

    def _reserve(self,name,details):
        self._guard()
        steps = self.journal.state['steps']
        if name in steps: raise RecoveryRequired('effect already attempted')
        record = {'status':'pending','nonce':secrets.token_hex(16),'details_sha256':_hash(details)}
        steps[name] = copy.deepcopy(record)
        self.journal.save()
        row, revision = self.fence.ledger.read()
        if not self._owned(row) or name in row['realtime_migration']['steps']:
            raise RecoveryRequired('migration ownership changed')
        row['realtime_migration']['steps'][name] = copy.deepcopy(record)
        self.fence.ledger.write(revision,row)
        self.migration_steps[name] = copy.deepcopy(record)
        self._guard()
        current, revision = self.fence.ledger.read()
        if (not self._owned(current) or current['realtime_migration']['steps'].get(name)!=record or
                not realtime_secret_phase.valid_revision(revision)):
            raise RecoveryRequired('effect reservation changed')
        return record

    def _finish(self,name,record,receipt):
        self._guard()
        row, revision = self.fence.ledger.read()
        if not self._owned(row) or row['realtime_migration']['steps'].get(name)!=record:
            raise RecoveryRequired('effect ownership changed')
        done = record | {'status':'done','receipt_sha256':_hash(receipt)}
        self.journal.state['steps'][name] = done | {'receipt':copy.deepcopy(receipt)}
        self.journal.save()
        row['realtime_migration']['steps'][name] = done
        self.fence.ledger.write(revision,row)
        self.migration_steps[name] = copy.deepcopy(done)
        self._guard()
        current, revision = self.fence.ledger.read()
        if (not self._owned(current) or current['realtime_migration']['steps'].get(name)!=done or
                not realtime_secret_phase.valid_revision(revision)):
            raise RecoveryRequired('terminal effect changed')

    def run(self):
        pending = False
        try:
            self._guard()
            operation = self.fence.context['operation_id']
            spec = self.intent['realtime_migration']
            if (set(spec)!={'source_sha','image','platform_digest','engine','database_oid','credential','node'} or
                    spec['source_sha']!=self.fence.context['commit'] or
                    type(spec['database_oid']) is not int or spec['database_oid']<=0 or
                    not isinstance(spec['node'],dict) or spec['platform_digest']!=PLATFORMS.get(spec['node'].get('architecture')) or
                    self.provision_journal.intent.get('resource_context')!=self.fence.context or
                    self.provision_journal.intent.get('postgres')!=spec['engine'] or
                    {k:self.provision_journal.intent['realtime_credential'][k] for k in ('name','uid','resourceVersion')}!=spec['credential']):
                raise RecoveryRequired('migration admission invalid')
            row, revision = self.fence.ledger.read()
            private = self.secret.journal.state['steps']['realtime-secret']
            secret_record = row.get('realtime_secret')
            provision_record = row.get('realtime_db_provision',{})
            database = provision_record.get('steps',{}).get('database',{})
            role = provision_record.get('steps',{}).get('role',{})
            if (not self.fence.owned(row) or row.get('status')!='active' or
                    private != secret_record | {'password_sha256':hashlib.sha256(self.password).hexdigest()} or
                    secret_record.get('status')!='done' or secret_record.get('receipt')!=spec['credential'] or
                    provision_record.get('operation_id')!=operation or
                    not isinstance(provision_record.get('nonce'),str) or
                    not re.fullmatch(r'[0-9a-f]{32}',provision_record['nonce']) or
                    set(provision_record.get('steps',{}))!={'role','database'} or role.get('status')!='done' or
                    self.provision_journal.state['steps'].get('realtime-db-invocation') !=
                        {'status':'pending','nonce':provision_record['nonce']} or
                    not realtime_db_provision._valid_role(role.get('catalog',{}).get('role')) or
                    self.provision_journal.state['steps'].get('realtime-db-role')!=role or
                    database.get('status')!='done' or database.get('catalog',{}).get('database',{}).get('oid')!=spec['database_oid'] or
                    database.get('catalog',{}).get('database',{}).get('owner_oid')!=role.get('catalog',{}).get('role',{}).get('oid') or
                    database.get('catalog',{}).get('role')!=role.get('catalog',{}).get('role') or
                    self.provision_journal.state['steps'].get('realtime-db-database')!=database or
                    'realtime_migration' in row or 'realtime-migration' in self.journal.state['steps']):
                raise RecoveryRequired('provision receipt invalid')
            self.bound_secret, self.bound_provision = copy.deepcopy(secret_record), copy.deepcopy(provision_record)
            pg = realtime_db_provision.KubePostgres(self._guard)
            worker = realtime_db_provision.Provision(self.provision_journal,self.fence,self.client,
                        self.lock,self.namespace,pg,self.password)
            worker._secret()
            if pg.snapshot()!=spec['engine']:
                raise RecoveryRequired('engine changed')
            config, planned = render(self.root,spec['source_sha'],operation,spec['credential'],spec['image'],spec['node'])
            def node_check():
                nodes = self.client.call('get','nodes','-o','json')['items']
                node = spec['node']
                matching = [n for n in nodes if n.get('metadata',{}).get('labels',{}).get('kubernetes.io/hostname')==node['hostname']]
                if len(matching)!=1: raise RecoveryRequired('node selection ambiguous')
                actual = matching[0]; meta = actual.get('metadata',{}); info = actual.get('status',{}).get('nodeInfo',{})
                if (meta.get('name')!=node['name'] or meta.get('uid')!=node['uid'] or meta.get('deletionTimestamp') or
                        meta.get('labels',{}).get('kubernetes.io/arch')!=node['architecture'] or
                        meta.get('labels',{}).get('kubernetes.io/os')!='linux' or
                        info.get('architecture')!=node['architecture'] or info.get('operatingSystem')!='linux' or
                        actual.get('spec',{}).get('unschedulable',False) is not False or
                        not any(c.get('type')=='Ready' and c.get('status')=='True' for c in actual.get('status',{}).get('conditions',[]))):
                    raise RecoveryRequired('node identity or readiness changed')
            self._guard(); node_check(); self._guard()
            snapshot = publish_candidate.source_snapshot(self.root,spec['source_sha'])
            if snapshot['files'].get('src/backend/realtime/internal/lifecycle/store.go') != \
                    '1e34ba05a4831031409bb90e37b541b8adfbe233cb307618cbb6ba471fb5fdc9':
                raise RecoveryRequired('readiness source changed')
            pending = True
            marker = {'operation_id':operation,'nonce':self.invocation,'status':'pending','steps':{}}
            self.journal.state['steps']['realtime-migration'] = {'status':'pending','nonce':self.invocation}
            self.journal.save()
            row['realtime_migration'] = copy.deepcopy(marker)
            self.fence.ledger.write(revision,row)
            self.migration_steps = {}
            record = self._reserve('config',config)
            self._check_step('config',record)
            created = self.client.call('create','-f','-','-o','json',body=config)
            cm = created.get('metadata',{})
            if (created.get('apiVersion')!='v1' or created.get('kind')!='ConfigMap' or cm.get('name')!=config['metadata']['name'] or
                    cm.get('namespace')!='voice-dev' or not cm.get('uid') or not cm.get('resourceVersion') or
                    cm.get('annotations',{}).get('voice.io/source-sha')!=spec['source_sha'] or cm.get('deletionTimestamp') or
                    created.get('immutable') is not True or created.get('data')!=config['data']):
                raise RecoveryRequired('created config differs')
            config_receipt = {'name':cm['name'],'uid':cm['uid'],'resourceVersion':cm['resourceVersion'],
                              'data_sha256':_hash(config['data'])}
            self._finish('config',record,config_receipt)
            def inputs():
                self._guard(); worker._secret(); node_check()
                actual = self.client.call('get','configmap',cm['name'],'-o','json')
                if (actual.get('apiVersion')!='v1' or actual.get('kind')!='ConfigMap' or
                        actual.get('metadata',{}).get('name')!=cm['name'] or
                        actual.get('metadata',{}).get('namespace')!='voice-dev' or
                        actual.get('metadata',{}).get('deletionTimestamp') or
                        actual.get('metadata',{}).get('annotations',{}).get('voice.io/source-sha')!=spec['source_sha'] or
                        actual.get('metadata',{}).get('uid')!=cm['uid'] or
                        actual.get('metadata',{}).get('resourceVersion')!=cm['resourceVersion'] or
                        actual.get('immutable') is not True or actual.get('data')!=config['data'] or pg.snapshot()!=spec['engine']):
                    raise RecoveryRequired('migration inputs changed')
            inputs()
            dryrun = self.client.call('create','--dry-run=server','-f','-','-o','json',body=planned)
            admitted_job(planned,dryrun)
            inputs()
            record = self._reserve('job',planned)
            inputs()
            self._check_step('job',record)
            created = self.client.call('create','-f','-','-o','json',body=planned)
            accepted_spec = admitted_job(planned,created)
            jm = created['metadata']; uid = jm['uid']
            if (not jm.get('resourceVersion') or jm.get('annotations',{}).get('voice.io/source-sha')!=spec['source_sha']):
                raise RecoveryRequired('created Job metadata changed')
            deadline = time.monotonic()+650
            while time.monotonic()<deadline:
                inputs()
                current = self.client.call('get','job',jm['name'],'-o','json')
                if (current.get('apiVersion')!='batch/v1' or current.get('kind')!='Job' or
                        current.get('metadata',{}).get('name')!=jm['name'] or
                        current.get('metadata',{}).get('namespace')!='voice-dev' or
                        current.get('metadata',{}).get('deletionTimestamp') or
                        current.get('metadata',{}).get('annotations',{}).get('voice.io/source-sha')!=spec['source_sha'] or
                        current.get('metadata',{}).get('uid')!=uid or current.get('spec')!=accepted_spec):
                    raise RecoveryRequired('Job identity changed')
                if admitted_job(planned,current)!=accepted_spec:
                    raise RecoveryRequired('Job admission changed')
                conditions = current.get('status',{}).get('conditions',[])
                if any(c.get('type')=='Failed' and c.get('status')=='True' for c in conditions):
                    raise RecoveryRequired('migration Job failed')
                if any(c.get('type')=='Complete' and c.get('status')=='True' for c in conditions):
                    if current.get('status',{}).get('active',0) or current.get('status',{}).get('failed',0):
                        raise RecoveryRequired('migration Job is not clean terminal')
                    pods = self.client.call('get','pods','-o','json')['items']
                    owned = [p for p in pods if any(o.get('controller') is True and o.get('kind')=='Job' and o.get('uid')==uid
                              for o in p.get('metadata',{}).get('ownerReferences',[]))]
                    if len(owned)!=1: raise RecoveryRequired('migration Pod set invalid')
                    pod = owned[0]; statuses = pod.get('status',{}).get('containerStatuses',[])
                    controllers = [o for o in pod.get('metadata',{}).get('ownerReferences',[]) if o.get('controller') is True]
                    if (len(controllers)!=1 or controllers[0].get('kind')!='Job' or controllers[0].get('uid')!=uid or
                            pod.get('metadata',{}).get('namespace')!='voice-dev' or pod.get('metadata',{}).get('deletionTimestamp')):
                        raise RecoveryRequired('migration Pod ownership changed')
                    observed_pod = copy.deepcopy(pod['spec'])
                    if observed_pod.pop('nodeName',None)!=spec['node']['name']:
                        raise RecoveryRequired('migration Pod node changed')
                    template = planned['spec']['template']
                    intended_pod = {'apiVersion':'v1','kind':'Pod','metadata':{'name':'template','namespace':'voice-dev',
                                    'labels':template['metadata']['labels']},'spec':template['spec']}
                    actual_pod = copy.deepcopy(intended_pod); actual_pod['spec'] = observed_pod
                    auth_user_candidate_probe.admitted_pod(intended_pod,actual_pod)
                    if (pod.get('status',{}).get('phase')!='Succeeded' or len(statuses)!=1 or
                            statuses[0].get('name')!='migrate' or not statuses[0].get('containerID') or
                            type(statuses[0].get('restartCount')) is not int or statuses[0]['restartCount']!=0 or
                            not statuses[0].get('imageID','').endswith(spec['platform_digest']) or
                            statuses[0].get('state',{}).get('terminated',{}).get('exitCode')!=0 or
                            not pod.get('metadata',{}).get('uid')):
                        raise RecoveryRequired('migration Pod terminal proof invalid')
                    receipt = {'job_uid':uid,'pod_uid':pod['metadata']['uid'],
                               'container_id':statuses[0]['containerID'],'image_id':statuses[0]['imageID']}
                    break
                time.sleep(2)
            else: raise RecoveryRequired('migration Job not terminal')
            inputs()
            schema = realtime_migration_verify.check(self.password,spec['database_oid'],spec['engine'],inputs,pg.snapshot)
            receipt['schema'] = schema
            inputs()
            final_job = self.client.call('get','job',jm['name'],'-o','json')
            final_pods = self.client.call('get','pods','-o','json')['items']
            final_owned = [p for p in final_pods if any(o.get('controller') is True and o.get('kind')=='Job' and o.get('uid')==uid
                           for o in p.get('metadata',{}).get('ownerReferences',[]))]
            if final_job != current or final_owned != [pod]: raise RecoveryRequired('terminal Job or Pod changed')
            self._finish('job',record,receipt)
            inputs()
            final_pods = self.client.call('get','pods','-o','json')['items']
            final_owned = [p for p in final_pods if any(o.get('controller') is True and o.get('kind')=='Job' and o.get('uid')==uid
                           for o in p.get('metadata',{}).get('ownerReferences',[]))]
            if self.client.call('get','job',jm['name'],'-o','json') != final_job or final_owned != [pod]:
                raise RecoveryRequired('terminal Job changed after receipt')
            done = record | {'status':'done','receipt_sha256':_hash(receipt)}
            self._check_step('job',done,receipt)
            row, revision = self.fence.ledger.read()
            if not self._owned(row) or row['realtime_migration'].get('status')!='pending':
                raise RecoveryRequired('terminal invocation changed')
            self.journal.state['steps']['realtime-migration'] = {'status':'done','nonce':self.invocation}
            self.journal.save()
            row['realtime_migration']['status'] = 'done'
            self.fence.ledger.write(revision,row)
            self.invocation_status = 'done'
            self._guard()
            self._check_step('job',done,receipt)
            return receipt
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('Realtime migration outcome uncertain')
                except BaseException: pass
            raise RecoveryRequired('Realtime migration not complete; retain lock and reconcile') from None
