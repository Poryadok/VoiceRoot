"""One-shot DEV-only Space replacement under the protected maintenance cut."""
import copy
import hashlib
import json
from pathlib import Path
import re
import secrets
import time

import frontend_runtime_executor as transport
import full_cohort_affected_plan as catalogs
import full_cohort_pg_physical_identity as physical_identity
import full_cohort_schema_metadata as metadata
import media_operation_guard
import media_operation_journal
import resource_fencing
import space_fresh_install_phase_events as events
import space_fresh_install_phase_job as jobs
import space_fresh_install_phase_pg as postgres
import space_fresh_install_phase_plan as plans
import space_native_tool_identity as native_tool
import writer_cut


class SpaceFreshError(RuntimeError):
    pass


def _check_catalog(original, actual, state, old_oid, new_oid):
    """Check the exact catalog transition without attributing its authority."""
    keys = ('all_databases', 'databases', 'excluded', 'properties')
    if (type(old_oid) is not int or not 0 < old_oid < 2**32 or
            any(not isinstance(original.get(k), dict) or not isinstance(actual.get(k), dict) for k in keys) or
            original['all_databases'].get('space_db') != old_oid or
            original['databases'].get('space_db') != old_oid or 'space_db' in original['excluded']):
        raise ValueError()
    if state == 'drop_pending':
        if new_oid is not None or any(original[k] != actual[k] for k in keys): raise ValueError()
        return
    if state in ('drop_done', 'job_pending', 'job_owned'):
        if new_oid is not None or any('space_db' in actual[k] for k in keys): raise ValueError()
    elif state in ('created', 'done'):
        if (type(new_oid) is not int or not 0 < new_oid < 2**32 or new_oid == old_oid or
                new_oid in (v for k, v in original['all_databases'].items() if k != 'space_db') or
                actual['all_databases'].get('space_db') != new_oid or actual['databases'].get('space_db') != new_oid or
                'space_db' in actual['excluded'] or 'space_db' not in actual['properties']): raise ValueError()
    else:
        raise ValueError()
    for key in keys:
        before, after = copy.deepcopy(original[key]), copy.deepcopy(actual[key])
        before.pop('space_db', None); after.pop('space_db', None)
        if before != after: raise ValueError()


STEP='space-fresh-install'
LEDGER='space_fresh_install'
RESET={'mode':'dev-reset-native51','database':'space_db'}
EVIDENCE='voice-space-native-installer-evidence'


def _digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode()).hexdigest()


class SpaceFreshInstallPhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,catalog,physical,cut,tool,source_root,token):
        try:
            import space_fresh_install_phase_runtime as runtime
            if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
                type(fence.ledger) is not resource_fencing.KubeLedger or type(client) is not transport.KubectlTransport or
                type(catalog) is not catalogs.FullCatalogPhase or type(physical) is not physical_identity.PhysicalPhase or
                type(cut) is not writer_cut.WriterCut or type(tool) is not native_tool.SpaceNativeToolPhase or
                journal.intent.get(LEDGER)!=RESET or journal.intent.get('namespace_uid')!=namespace_uid or
                journal.intent.get('resource_context')!=fence.context or fence.context.get('resource')!='DEV' or
                not re.fullmatch(r'media-c1-[a-f0-9]{32}',fence.context.get('operation_id','')) or
                not isinstance(token,str) or not token):raise ValueError()
            self.journal,self.fence,self.client=journal,fence,client
            self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
            self.catalog,self.physical,self.cut,self.tool=catalog,physical,cut,tool
            self.source_root,self.token=Path(source_root).resolve(),token
            self.intent=copy.deepcopy(journal.intent);self.context=copy.deepcopy(fence.context)
            self.poisoned=False
            self._expected=copy.deepcopy(journal.state['steps'].get(STEP))
            self._refs=(journal,fence,client,catalog,physical,cut,tool)
            self._facades=tuple((obj,obj.journal,obj.fence,obj.client,obj.fence.ledger,
                                native_tool.SpaceNativeToolPhase._custody(obj.journal))
                               for obj in (self,catalog,physical,cut,tool))
            self._identity()
            self.runtime=runtime.SpaceRuntimeRolePhase(journal,fence,client,self.lock,namespace_uid,catalog,cut,self.source_root,token)
            self._runtime_ref=self.runtime
            self.pg=postgres.KubeSpacePostgres(self._guard);self._pg_ref=self.pg
        except BaseException:
            raise SpaceFreshError('Space fresh phase identity unavailable') from None

    def _identity(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent!=self.intent or
            self.journal.state.get('intent')!=self.intent or self.fence.context!=self.context or
            any(a is not b for a,b in zip((self.journal,self.fence,self.client,self.catalog,self.physical,self.cut,self.tool),self._refs))):
            raise SpaceFreshError('Space phase identity changed')
        for obj,journal,fence,client,ledger,custody in self._facades:
            if (obj.journal is not journal or obj.fence is not fence or obj.client is not client or fence.ledger is not ledger or
                client.journal is not journal or ledger.client is not client or fence.context!=self.context or
                journal.poisoned or journal.intent.get('resource_context')!=self.context or
                journal.intent.get('namespace_uid')!=self.namespace_uid or obj.namespace_uid!=self.namespace_uid or obj.lock!=self.lock or
                native_tool.SpaceNativeToolPhase._custody(journal)!=custody):raise SpaceFreshError('Space dependency changed')
        if hasattr(self,'_runtime_ref') and (self.runtime is not self._runtime_ref or self.pg is not self._pg_ref):raise ValueError()

    def _guard(self):
        self._identity()
        for obj,*_ in self._facades:
            media_operation_guard.verify(obj.fence,obj.client,obj.lock,obj.namespace_uid)
        self._identity()

    def _ledger(self):
        row,revision=self.fence.ledger.read();row=copy.deepcopy(row)
        if (not self.fence.owned(row) or row.get('status')!='active' or not isinstance(revision,tuple) or
            len(revision)!=2 or any(not isinstance(v,str) or not v for v in revision)):raise ValueError()
        for obj,*_ in self._facades[1:]:
            if obj.fence.ledger.read()!=(row,revision):raise ValueError()
        return row,revision

    def _read(self):
        self._guard();row,revision=self._ledger()
        local=copy.deepcopy(self.journal.state['steps'].get(STEP))
        if local!=self._expected or not isinstance(local,dict) or set(local)!={'marker','details'}:raise ValueError()
        marker,details=local['marker'],local['details']
        if (set(marker)!={'operation_id','source_sha','nonce','state','details_sha256'} or
            marker['operation_id']!=self.context['operation_id'] or marker['source_sha']!=self.context['commit'] or
            not re.fullmatch(r'[a-f0-9]{32}',marker['nonce']) or marker['state']!=details['state'] or
            marker['details_sha256']!=_digest(details) or row.get(LEDGER)!=marker):raise ValueError()
        self._identity()
        return details,row,revision,local

    def _stable(self,before):
        self._guard();row,revision=self._ledger()
        if (row!=before[1] or revision!=before[2] or self.journal.state['steps'].get(STEP)!=before[3] or
            self._expected!=before[3]):raise ValueError()
        self._identity()

    def _save(self,details):
        self._guard();row,revision=self._ledger()
        if self._expected is None:
            if STEP in self.journal.state['steps'] or LEDGER in row:raise ValueError()
            nonce=secrets.token_hex(16)
        else:
            if self.journal.state['steps'].get(STEP)!=self._expected or row.get(LEDGER)!=self._expected['marker']:raise ValueError()
            nonce=self._expected['marker']['nonce']
        marker={'operation_id':self.context['operation_id'],'source_sha':self.context['commit'],
                'nonce':nonce,'state':details['state'],'details_sha256':_digest(details)}
        local={'marker':marker,'details':copy.deepcopy(details)}
        self._expected=copy.deepcopy(local);self.journal.state['steps'][STEP]=copy.deepcopy(local);self.journal.save()
        self._guard()
        if self._ledger()!=(row,revision) or self.journal.state['steps'].get(STEP)!=local:raise ValueError()
        row[LEDGER]=copy.deepcopy(marker);row['step']=STEP+'-'+details['state']
        self.fence.ledger.write(revision,row)
        self._read()

    def _catalog(self,details):
        original=details['catalog']
        if self.pg.snapshot()!=original['engine']:raise ValueError()
        actual=catalogs._full_catalog(self.catalog,original['engine'],original['server_start'])
        _check_catalog(original,actual,details['state'],details['old_oid'],details['new_oid'])
        if self.pg.snapshot()!=original['engine']:raise ValueError()
        return actual

    def _get(self,kind,name):
        self._guard();value=self.client.call('get',kind,name,'--ignore-not-found=true','-o','json');self._guard()
        return value

    def _cm_identity(self,authored,actual):
        meta=actual['metadata']
        if (actual.get('kind')!='ConfigMap' or actual.get('apiVersion')!='v1' or actual.get('immutable') is not True or
            actual.get('data')!=authored['data'] or actual.get('binaryData') or
            meta.get('name')!=authored['metadata']['name'] or meta.get('namespace')!='voice-dev' or
            meta.get('annotations')!=authored['metadata'].get('annotations') or meta.get('deletionTimestamp') or
            meta.get('ownerReferences') or meta.get('finalizers') or
            any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion'))):raise ValueError()
        return {k:meta[k] for k in ('uid','resourceVersion')}

    def _create(self,details,key,authored):
        if key in details['resources']:raise ValueError()
        details['resources'][key]={'state':'pending','nonce':secrets.token_hex(16),'authored':copy.deepcopy(authored)}
        self._save(details)
        self.cut.verify_space_pending() if details['state']!='created' else self.cut.verify_space_running()
        self._read()
        actual=self.client.call('create','-f','-','-o','json',body=authored)
        if authored['kind']=='Job':jobs.job(authored,actual);identity={k:actual['metadata'][k] for k in ('uid','resourceVersion')}
        else:identity=self._cm_identity(authored,actual)
        # Save the known response identity before a generic catalog guard can see
        # a newly created database. A lost response never enters this path.
        details['resources'][key].update(state='owned',**identity)
        if key=='provision':details['state']='job_owned'
        self._save(details)
        reread=self._get(authored['kind'].lower(),authored['metadata']['name'])
        if authored['kind']=='Job':jobs.job(authored,reread)
        else:self._cm_identity(authored,reread)
        if reread['metadata']['uid']!=identity['uid']:raise ValueError()
        return details

    def _job_observation(self,details,key):
        resource=details['resources'][key];authored=resource['authored'];name=authored['metadata']['name']
        actual=self._get('job',name);jobs.job(authored,actual)
        if actual['metadata']['uid']!=resource['uid']:raise ValueError()
        status=actual.get('status',{})
        if (type(status.get('failed',0)) is not int or status.get('failed',0)!=0 or
            any(c.get('type')=='Failed' and c.get('status')=='True' for c in status.get('conditions',[]))):raise ValueError()
        self._guard();pods=self.client.call('get','pods','-l','job-name='+name,'-o','json').get('items');self._guard()
        if not isinstance(pods,list) or len(pods)>1:raise ValueError()
        if not pods:return actual,None,None
        pod=pods[0]
        if resource.get('pending_pod') and {k:pod['metadata'].get(k) for k in ('name','uid')}!=resource['pending_pod']:raise ValueError()
        if pod.get('status',{}).get('phase')=='Pending':
            pm=pod['metadata'];owners=[o for o in pm.get('ownerReferences',[]) if o.get('controller') is True]
            if (pm.get('namespace')!='voice-dev' or pm.get('deletionTimestamp') or
                any(not isinstance(pm.get(k),str) or not pm[k] for k in ('name','uid','resourceVersion')) or
                len(owners)!=1 or owners[0].get('kind')!='Job' or owners[0].get('uid')!=resource['uid'] or
                owners[0].get('name')!=name):raise ValueError()
            template=authored['spec']['template']
            intended={'apiVersion':'v1','kind':'Pod','metadata':{'name':'template','namespace':'voice-dev',
                'labels':template['metadata']['labels']},'spec':template['spec']}
            observed=copy.deepcopy(intended);observed['spec']=pod['spec']
            jobs.admission.admitted_pod(intended,observed)
            return actual,pod,None
        proof=jobs.pod(authored,actual,pod,details['tool']['platform_digest'])
        previous=resource.get('pod')
        if previous and any(previous[k]!=proof[k] for k in ('name','uid','pod_ip','container_id')):raise ValueError()
        return actual,pod,proof

    def _logs(self,name):
        if not isinstance(name,str) or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]{0,251}[a-z0-9])?',name):raise ValueError()
        self._guard()
        raw=metadata._bounded_process(self.client.PREFIX+['logs',name,'-c','migrate','--tail=-1','--limit-bytes=1048577'],1048576)
        self._guard();return raw

    def _expected_events(self,details):
        tool=details['tool']
        return {'source_revision':tool['source_sha'],'source_sha256':tool['build_source_sha256'],
            'executable_sha256':tool['executable_sha256'],'migrator_sha256':tool['migrator_sha256'],
            'inventory_sha256':tool['sql_inventory_sha256'],'database_system_id':details['physical']['physical']['system_identifier'],
            'installer_role':details['installer_role']}

    @staticmethod
    def _terminal(actual,pod):
        status=actual.get('status',{})
        if not (status.get('succeeded')==1 and type(status.get('succeeded')) is int and status.get('active',0)==0 and
                any(c.get('type')=='Complete' and c.get('status')=='True' for c in status.get('conditions',[]))):return False
        statuses=pod.get('status',{}).get('containerStatuses',[])
        return (pod.get('status',{}).get('phase')=='Succeeded' and len(statuses)==1 and
                type(statuses[0].get('state',{}).get('terminated',{}).get('exitCode')) is int and
                statuses[0]['state']['terminated']['exitCode']==0)

    def _wait_job(self,details,key):
        deadline=time.monotonic()+620
        while True:
            self._read();actual,pod,proof=self._job_observation(details,key)
            if pod and details['resources'][key].get('pending_pod') is None:
                details['resources'][key]['pending_pod']={k:pod['metadata'][k] for k in ('name','uid')}
                self._save(details)
            if proof:
                if details['resources'][key].get('pod') is None:
                    details['resources'][key]['pod']=proof;self._save(details)
                self.cut.verify_space_running();self._read()
                raw=self._logs(proof['name'])
                if not raw:
                    if self._terminal(actual,pod) or time.monotonic()>=deadline:raise ValueError()
                    time.sleep(1);continue
                if key=='provision':
                    identity=events.created(raw,self._expected_events(details),old_oid=details['old_oid'])
                    if details['new_oid'] is None:
                        if self.pg.space_oid()!=identity['new_oid']:raise ValueError()
                        details.update(state='created',**identity)
                        details['resources'][key]['pod']=proof;details['created_log_sha256']=hashlib.sha256(raw).hexdigest()
                        self._save(details)
                    elif any(details[k]!=v for k,v in identity.items()):raise ValueError()
                self.cut.verify_space_running();self._read()
                actual2,pod2,proof2=self._job_observation(details,key)
                if any(proof2[k]!=proof[k] for k in ('name','uid','pod_ip','container_id')):raise ValueError()
                if self._terminal(actual2,pod2):
                    raw=self._logs(proof['name'])
                    if key=='provision':
                        if events.completed(raw,self._expected_events(details),old_oid=details['old_oid'])!={k:details[k] for k in ('run_id','new_oid','role')}:raise ValueError()
                        records=[json.loads(line,object_pairs_hook=events._unique) for line in raw.splitlines()]
                        complete=records[-1]['receipt']
                        for prefix,key45 in (('000045_','transition45'),('000050_','transition50')):
                            candidates=[v for k,v in details['sql']['data'].items() if k.startswith(prefix) and k.endswith('.up.sql')]
                            if len(candidates)!=1 or hashlib.sha256(candidates[0].encode()).hexdigest()!=complete[key45]['migration_sha256']:raise ValueError()
                        details['native_receipt']=complete;details['native_log']=raw.decode('utf-8')
                    else:
                        record=json.loads(raw,object_pairs_hook=events._unique)
                        if record!={'event':'verified','receipt':details['native_receipt']}:raise ValueError()
                        details['verified_log']=raw.decode('utf-8')
                    details['resources'][key]['terminal']=True
                    details['resources'][key]['pod']=proof2
                    self._save(details);return details
            if time.monotonic()>=deadline:raise ValueError()
            time.sleep(1)

    def _cleanup(self,details,key):
        resource=details['resources'][key]
        if resource['state']!='owned':raise ValueError()
        authored=resource['authored'];kind=authored['kind'].lower();name=authored['metadata']['name']
        actual=self._get(kind,name)
        if kind=='job':
            jobs.job(authored,actual)
            if not resource.get('terminal'):raise ValueError()
        else:self._cm_identity(authored,actual)
        if actual['metadata']['uid']!=resource['uid']:raise ValueError()
        resource['state']='removing';self._save(details)
        self.cut.verify_space_running();self._read()
        prefix='/apis/batch/v1/namespaces/voice-dev/jobs/' if kind=='job' else '/api/v1/namespaces/voice-dev/configmaps/'
        self.client.call('delete','--raw='+prefix+name,'-f','-',body={'apiVersion':'v1','kind':'DeleteOptions',
            'propagationPolicy':'Foreground','preconditions':{k:actual['metadata'][k] for k in ('uid','resourceVersion')}})
        deadline=time.monotonic()+60
        while True:
            actual=self._get(kind,name)
            pods=[]
            if kind=='job':
                self._guard();pods=self.client.call('get','pods','-l','job-name='+name,'-o','json').get('items');self._guard()
                if not isinstance(pods,list) or any(p.get('metadata',{}).get('uid')!=resource['pod']['uid'] for p in pods):raise ValueError()
            if not actual and not pods:break
            if actual and actual.get('metadata',{}).get('uid')!=resource['uid']:raise ValueError()
            if time.monotonic()>=deadline:raise ValueError()
            time.sleep(1)
        resource['state']='removed';self._save(details)

    def prepare(self):
        """Capture typed runtime consumer authority while original Pods still exist."""
        try:
            self._guard()
            if self._expected is not None or LEDGER in self._ledger()[0]:raise ValueError()
            value=self.runtime.prepare()
            self._guard();return copy.deepcopy(value)
        except BaseException:raise SpaceFreshError('Space pre-cut runtime admission unavailable') from None

    def run(self):
        attempted=False
        try:
            import space_fresh_install_phase_runtime as runtime
            self._guard()
            if self._expected is not None or LEDGER in self._ledger()[0]:raise ValueError()
            self.cut.verify_drained()
            original=self.catalog.verify_current(self.token)
            physical=physical_identity.verify_receipt(self.physical,self.token)
            tool=native_tool.verify_completed(self.tool,self.token)
            if (original['operation_id']!=self.context['operation_id'] or original['source_sha']!=self.context['commit'] or
                tool['operation_id']!=self.context['operation_id'] or tool['source_sha']!=self.context['commit'] or
                tool['namespace_uid']!=self.namespace_uid or physical['physical']['engine']!=original['engine'] or
                physical['physical']['server_start']!=original['server_start']):raise ValueError()
            node=physical['physical']['node']
            if not any(all(n.get(k)==node[k] for k in ('name','uid')) for n in tool['nodes']):raise ValueError()
            sql,job=plans.render(self.source_root,self.context['commit'],self.context['operation_id'],tool['immutable_image'],tool['build_source_sha256'],node['name'])
            for kind,name in (('configmap',sql['metadata']['name']),('job',job['metadata']['name']),
                              ('job',job['metadata']['name']+'-verify'),('configmap',EVIDENCE)):
                if self._get(kind,name):raise ValueError()
            preflight=runtime.verify_prepared(self.runtime,self.token)
            details={'state':'drop_pending','catalog':copy.deepcopy(original),'physical':copy.deepcopy(physical),'tool':copy.deepcopy(tool),
                'old_oid':original['databases']['space_db'],'new_oid':None,'run_id':None,'role':None,
                'installer_role':self.catalog.inventory._admin_role(),'sql':sql,'job':job,'resources':{},'runtime_preflight':preflight,'job_kind':'provision'}
            if self.catalog.verify_current(self.token)!=original or native_tool.verify_completed(self.tool,self.token)!=tool:raise ValueError()
            self._catalog(details);self.cut.verify_drained()
            attempted=True;self._save(details);self.cut.bind_space_fresh(self,self.token)
            self.cut.verify_space_pending();self._read();self._catalog(details)
            self.pg.drop_exact(details['old_oid'])
            if self.pg.space_oid()!=0:raise ValueError()
            details['state']='drop_done';self._catalog(details);self._save(details)
            self.cut.verify_space_pending();self._create(details,'sql',sql)
            details['state']='job_pending';self._save(details);self._create(details,'provision',job)
            self._wait_job(details,'provision')
            evidence={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':EVIDENCE,'namespace':'voice-dev',
                'annotations':{'voice.dev/cohort-operation':self.context['operation_id'],'voice.io/source-sha':self.context['commit']}},
                'immutable':True,'data':{'prior.jsonl':details['native_log']}}
            self._create(details,'evidence',evidence)
            details['runtime']=self.runtime.run(self);self._save(details)
            verify=copy.deepcopy(job);verify['metadata']['name']+='-verify'
            env=verify['spec']['template']['spec']['containers'][0]['env']
            for entry in env:
                if entry['name']=='SPACE_NATIVE_INSTALLER_MODE':entry['value']='verify'
            details['job_kind']='verify';self._save(details);self._create(details,'verify',verify)
            self._wait_job(details,'verify')
            for key in ('provision','verify','sql'):self._cleanup(details,key)
            details['state']='done';details['final_catalog']=self._catalog(details);self._save(details)
            self.cut.finish_space_fresh()
            return verify_completed(self,self.token)
        except BaseException:
            if attempted:
                self.poisoned=True
                try:self.fence.uncertain('Space fresh install outcome uncertain')
                except BaseException:pass
            raise SpaceFreshError('Space fresh install uncertain; retain lock' if attempted else 'Space fresh install preflight refused') from None


def verify_catalog_successor(phase,cut,token):
    """Authenticate a Space-only catalog transition, without calling CUT guards."""
    try:
        if type(phase) is not SpaceFreshInstallPhase or cut is not phase.cut or token!=phase.token:raise ValueError()
        before=phase._read();details=before[0]
        if phase.catalog.verify_receipt(token)!=details['catalog'] or physical_identity.verify_receipt(phase.physical,token)!=details['physical']:raise ValueError()
        phase._catalog(details)
        resource=details['resources'].get(details['job_kind'],{})
        pod=resource.get('pod',{})
        result={'state':details['state'],'old_oid':details['old_oid'],'new_oid':details['new_oid'],
            'run_id':details['run_id'],'role':details['role'],'job_kind':details['job_kind'],
            'job_uid':resource.get('uid'),'pod_uid':pod.get('uid'),'pod_ip':pod.get('pod_ip'),
            'receipt_sha256':before[3]['marker']['details_sha256']}
        phase._stable(before);return result
    except BaseException:raise SpaceFreshError('Space catalog successor unavailable') from None


def verify_writer_successor(phase,token):
    """Owned Job window; observed pre-created OID is not a durable successor."""
    try:
        if type(phase) is not SpaceFreshInstallPhase or token!=phase.token:raise ValueError()
        before=phase._read();details=before[0];kind=details['job_kind']
        resource=details['resources'].get(kind)
        if kind not in ('provision','verify') or not isinstance(resource,dict):raise ValueError()
        if phase.catalog.verify_receipt(token)!=details['catalog']:raise ValueError()
        actual=catalogs._full_catalog(phase.catalog,details['catalog']['engine'],details['catalog']['server_start'])
        if details['state']=='job_owned' and kind=='provision':
            others=copy.deepcopy(actual)
            for name in ('all_databases','databases','excluded','properties'):others[name].pop('space_db',None)
            _check_catalog(details['catalog'],others,'drop_done',details['old_oid'],None)
            observed=actual['all_databases'].get('space_db')
            if observed==details['old_oid']:raise ValueError()
        else:_check_catalog(details['catalog'],actual,details['state'],details['old_oid'],details['new_oid'])
        pod=None
        if resource['state'] in ('owned','removing'):
            _,_,pod=phase._job_observation(details,kind)
        elif resource['state']=='removed':
            if not resource.get('terminal') or phase._get('job',resource['authored']['metadata']['name']):raise ValueError()
        elif resource['state']!='pending':raise ValueError()
        result={'state':details['state'],'job_kind':kind,'record':copy.deepcopy(resource),'job':copy.deepcopy(resource['authored']),
            'old_oid':details['old_oid'],'new_oid':details['new_oid'],'run_id':details['run_id'],
            'role':details['installer_role'],'owned_pod':pod,'engine':copy.deepcopy(details['catalog']['engine']),
            'server_start':details['catalog']['server_start'],'catalog_original':copy.deepcopy(details['catalog']),
            'catalog_actual':actual,'receipt_sha256':before[3]['marker']['details_sha256']}
        phase._stable(before);return result
    except BaseException:raise SpaceFreshError('Space owned Job window unavailable') from None


def verify_native_completed_before_runtime(phase,token):
    try:
        if type(phase) is not SpaceFreshInstallPhase or token!=phase.token:raise ValueError()
        before=phase._read();details=before[0]
        resource=details['resources']['provision']
        if details['state']!='created' or resource['state']!='owned' or resource.get('terminal') is not True:raise ValueError()
        actual,pod,proof=phase._job_observation(details,'provision')
        if not phase._terminal(actual,pod) or phase._logs(proof['name']).decode()!=details['native_log']:raise ValueError()
        if events.completed(details['native_log'].encode(),phase._expected_events(details),old_oid=details['old_oid'])!={k:details[k] for k in ('run_id','new_oid','role')}:raise ValueError()
        phase._catalog(details)
        result={'operation_id':phase.context['operation_id'],'source_sha':phase.context['commit'],'namespace_uid':phase.namespace_uid,
                'new_oid':details['new_oid'],'run_id':details['run_id'],'installer_role':details['role'],
                'native_receipt_sha256':_digest(details['native_receipt'])}
        phase._stable(before);return result
    except BaseException:raise SpaceFreshError('Space native completion unavailable') from None


def verify_historical(phase,token):
    """Authenticate completed Space evidence without observing current PG or CUT.

    The catalog is the recorded post-install catalog, not a current observation.
    The central typed successor composes it with later qualified transitions.
    """
    try:
        import space_fresh_install_phase_runtime as runtime
        if type(phase) is not SpaceFreshInstallPhase or token!=phase.token:raise ValueError()
        before=phase._read();details=before[0]
        if details['state']!='done' or set(details['resources'])!={'sql','provision','evidence','verify'}:raise ValueError()
        for key in ('sql','provision','verify'):
            record=details['resources'][key]
            if record['state']!='removed' or not record.get('uid'):raise ValueError()
            if key!='sql' and (record.get('terminal') is not True or not record.get('pod')):raise ValueError()
        evidence=details['resources']['evidence']
        if (evidence['state']!='owned' or not evidence.get('uid') or
            evidence['authored']['data']!={'prior.jsonl':details['native_log']}):raise ValueError()
        if (native_tool.verify_completed(phase.tool,token)!=details['tool'] or
            phase.catalog.verify_receipt(token)!=details['catalog'] or
            physical_identity.verify_receipt(phase.physical,token)!=details['physical'] or
            runtime.verify_receipt(phase.runtime,token)!=details['runtime']):raise ValueError()
        identity=events.completed(details['native_log'].encode(),phase._expected_events(details),old_oid=details['old_oid'])
        records=[json.loads(line,object_pairs_hook=events._unique) for line in details['native_log'].splitlines()]
        if (identity!={k:details[k] for k in ('run_id','new_oid','role')} or
            records[-1]['receipt']!=details['native_receipt'] or
            json.loads(details['verified_log'],object_pairs_hook=events._unique)!=
                {'event':'verified','receipt':details['native_receipt']}):raise ValueError()
        _check_catalog(details['catalog'],details['final_catalog'],'done',details['old_oid'],details['new_oid'])
        result={'operation_id':phase.context['operation_id'],'source_sha':phase.context['commit'],
            'namespace_uid':phase.namespace_uid,'old_oid':details['old_oid'],'new_oid':details['new_oid'],
            'catalog':copy.deepcopy(details['final_catalog']),'original_catalog_sha256':_digest(details['catalog']),
            'receipt_sha256':before[3]['marker']['details_sha256']}
        phase._stable(before);return result
    except BaseException:raise SpaceFreshError('Space historical evidence unavailable') from None


def verify_completed(phase,token):
    try:
        import space_fresh_install_phase_runtime as runtime
        if type(phase) is not SpaceFreshInstallPhase or token!=phase.token:raise ValueError()
        before=phase._read();details=before[0]
        if details['state']!='done' or set(details['resources'])!={'sql','provision','evidence','verify'}:raise ValueError()
        for key in ('sql','provision','verify'):
            resource=details['resources'][key]
            if resource['state']!='removed':raise ValueError()
            if phase._get(resource['authored']['kind'].lower(),resource['authored']['metadata']['name']):raise ValueError()
        evidence=details['resources']['evidence']
        actual=phase._get('configmap',EVIDENCE);phase._cm_identity(evidence['authored'],actual)
        if actual['metadata']['uid']!=evidence['uid'] or evidence['state']!='owned':raise ValueError()
        if (native_tool.verify_completed(phase.tool,token)!=details['tool'] or
            phase.catalog.verify_receipt(token)!=details['catalog'] or
            physical_identity.verify_receipt(phase.physical,token)!=details['physical'] or
            runtime.verify_completed(phase.runtime,token)!=details['runtime'] or
            phase._catalog(details)!=details['final_catalog']):raise ValueError()
        if json.loads(details['verified_log'],object_pairs_hook=events._unique)!={'event':'verified','receipt':details['native_receipt']}:raise ValueError()
        result={'operation_id':phase.context['operation_id'],'source_sha':phase.context['commit'],'namespace_uid':phase.namespace_uid,
            'old_oid':details['old_oid'],'new_oid':details['new_oid'],'native_receipt':copy.deepcopy(details['native_receipt']),
            'runtime':copy.deepcopy(details['runtime']),'catalog':copy.deepcopy(details['final_catalog']),
            'evidence_uid':evidence['uid'],'receipt_sha256':before[3]['marker']['details_sha256']}
        phase._stable(before);return result
    except BaseException:raise SpaceFreshError('Space fresh completed evidence unavailable') from None
