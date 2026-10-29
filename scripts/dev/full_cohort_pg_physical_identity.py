"""Capture source-owned DEV PostgreSQL physical identity under the writer cut."""
import copy
import hashlib
import json
import re
import secrets
from types import SimpleNamespace

import frontend_runtime_executor
import full_cohort_affected_plan as affected
import full_cohort_schema_metadata as schema
import media_operation_guard
import media_operation_journal
import resource_fencing
import story_media_backup_capture as capture
import writer_cut


class PhysicalError(RuntimeError):
    pass


STEP='full-cohort-pg-physical-identity'
LEDGER='full_cohort_pg_physical_identity'
MOUNT='/var/lib/postgresql/data'
TABLESPACE_SCRIPT=('test -d "$1/pg_wal"; test ! -L "$1/pg_wal"; '
                   'test -d "$1/pg_tblspc"; test ! -L "$1/pg_tblspc"; '
                   'listing=$(ls -A -- "$1/pg_tblspc") || exit 1; test -z "$listing"; printf ok')
SQL=("SELECT json_build_object('database',pg_catalog.current_database(),"
     "'database_oid',(SELECT oid::bigint FROM pg_catalog.pg_database WHERE datname=pg_catalog.current_database()),"
     "'role',current_user,'server_start',pg_catalog.pg_postmaster_start_time(),"
     "'read_only',pg_catalog.current_setting('transaction_read_only'),"
     "'data_directory',pg_catalog.current_setting('data_directory'),"
     "'pg_major',pg_catalog.current_setting('server_version_num')::integer / 10000,"
     "'system_identifier',(pg_catalog.pg_control_system()).system_identifier::text)")


def _digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode()).hexdigest()


class PhysicalPhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,catalog,cut):
        self.journal,self.fence,self.client,self.catalog,self.cut=journal,fence,client,catalog,cut
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.intent=copy.deepcopy(journal.intent);self.poisoned=False
        self._dependencies=(journal,fence,client,catalog,cut)
        self._facades=tuple((other.journal,other.fence,other.client) for other in (catalog,cut))
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence
            or type(fence.ledger) is not resource_fencing.KubeLedger
            or type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
            or fence.ledger.client is not client or type(catalog) is not affected.FullCatalogPhase
            or type(cut) is not writer_cut.WriterCut
            or self.intent.get('resource_context')!=fence.context or self.intent.get('namespace_uid')!=namespace_uid):
            raise PhysicalError('physical identity context invalid')
        self._authority()

    def _authority(self):
        for other,facade in zip((self.catalog,self.cut),self._facades):
            if (type(other.journal) is not media_operation_journal.Journal or
                type(other.fence) is not resource_fencing.Fence or type(other.fence.ledger) is not resource_fencing.KubeLedger or
                type(other.client) is not frontend_runtime_executor.KubectlTransport or
                other.client.journal is not other.journal or other.fence.ledger.client is not other.client or
                other.fence.context!=self.fence.context or other.lock!=self.lock or other.namespace_uid!=self.namespace_uid or
                other.journal.intent.get('resource_context')!=self.fence.context or
                other.journal.intent.get('namespace_uid')!=self.namespace_uid or
                any(actual is not original for actual,original in zip((other.journal,other.fence,other.client),facade))):
                raise PhysicalError('physical identity context changed')
            media_operation_guard.verify(other.fence,other.client,other.lock,other.namespace_uid)
        self._ledger()

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.dirfd is None or self.journal.lockfd is None or self.journal.intent!=self.intent
            or self.journal.state.get('intent')!=self.intent
            or any(a is not b for a,b in zip((self.journal,self.fence,self.client,self.catalog,self.cut),self._dependencies))):
            raise PhysicalError('physical identity context changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        self._authority()

    def _ledger(self):
        row,revision=self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status')!='active' or
            not isinstance(revision,tuple) or len(revision)!=2 or any(not isinstance(v,str) or not v for v in revision)):
            raise PhysicalError('physical identity fence changed')
        for other in (self.catalog,self.cut):
            if other.fence.ledger.read()!=(row,revision):raise PhysicalError('physical identity ledger differs')
        return row,revision

    def _get(self,kind,name):
        self._guard();self.cut.verify_drained()
        value=self.client.call('get',kind,name,'-o','json')
        self._guard();self.cut.verify_drained()
        return value

    def _physical_command(self,args,**_kwargs):
        self._guard();self.cut.verify_drained()
        value=schema._bounded_process(args,1024*1024)
        self._guard();self.cut.verify_drained()
        return SimpleNamespace(returncode=0,stdout=value)

    def _mount_proof(self,expected):
        # Reuse the dump producer's mount-namespace verification with only its
        # fixed read-only commands and this phase's authority around each call.
        outputs=[]
        def run(args,**kwargs):
            result=self._physical_command(args,**kwargs);outputs.append(result.stdout);return result
        probe=SimpleNamespace(expected=expected,runner=run)
        capture.Producer._physical_source(probe)
        if len(outputs)!=2:raise capture.CaptureError('physical mount proof invalid')
        lines=outputs[1].decode().split('\n',2)
        if len(lines)!=3:raise capture.CaptureError('physical mount proof invalid')
        for line in lines[2].splitlines():
            fields=line.split(' - ',1)[0].split()
            if len(fields)<6 or fields[4].startswith(expected['data_directory'].rstrip('/')+'/'):
                raise capture.CaptureError('postgres data includes a nested mount')
        args=capture.BASE+['exec',capture.POD,'-c','postgres','--','sh','-ec',TABLESPACE_SCRIPT,'--',expected['data_directory']]
        if self._physical_command(args).stdout!=b'ok':raise capture.CaptureError('postgres external storage present')

    def _observe(self,catalog):
        self._guard();self.cut.verify_drained()
        snap=schema.kube_engine_snapshot(schema.linux_private_bytes,self._guard)
        engine=snap()
        if engine!=catalog['engine'] or engine.get('namespace_uid')!=self.namespace_uid:
            raise PhysicalError('physical engine changed')
        sts=self._get('statefulset','voice-postgres');pod=self._get('pod','voice-postgres-0')
        pvc=self._get('persistentvolumeclaim','pgdata-voice-postgres-0')
        sm=sts['metadata'];pm=pod['metadata'];pvc_meta=pvc['metadata'];name=pvc['spec']['volumeName']
        if not isinstance(name,str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',name):raise ValueError()
        pv=self._get('persistentvolume',name);pvm=pv['metadata'];claim=pv['spec']['claimRef']
        node_name=pod['spec']['nodeName']
        if not isinstance(node_name,str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',node_name):raise ValueError()
        node=self._get('node',node_name);nm=node['metadata']
        owners=[row for row in pm.get('ownerReferences',[]) if row.get('controller') is True]
        containers=[row for row in pod['spec']['containers'] if row.get('name')=='postgres']
        statuses=[row for row in pod['status']['containerStatuses'] if row.get('name')=='postgres']
        volumes=[row for row in pod['spec']['volumes'] if row.get('persistentVolumeClaim',{}).get('claimName')=='pgdata-voice-postgres-0']
        if (sm.get('name')!='voice-postgres' or sm.get('namespace')!='voice-dev' or not sm.get('uid') or sm.get('deletionTimestamp')
            or pm.get('name')!='voice-postgres-0' or pm.get('namespace')!='voice-dev' or pm.get('uid')!=engine['pod_uid']
            or pm.get('deletionTimestamp') or len(owners)!=1 or owners[0].get('kind')!='StatefulSet' or owners[0].get('uid')!=sm['uid']
            or len(containers)!=1 or len(statuses)!=1 or len(volumes)!=1 or statuses[0].get('ready') is not True
            or not isinstance(statuses[0].get('state',{}).get('running'),dict)
            or statuses[0].get('containerID')!=engine['container_id'] or statuses[0].get('restartCount')!=engine['restart_count']
            or pvc_meta.get('uid')!=engine['pvc_uid'] or pvc_meta.get('namespace')!='voice-dev' or pvc_meta.get('deletionTimestamp')
            or pvc.get('status',{}).get('phase')!='Bound' or pvm.get('name')!=name or pvm.get('uid')!=engine['pv_uid']
            or pvm.get('deletionTimestamp') or pv.get('status',{}).get('phase')!='Bound'
            or any(claim.get(k)!=v for k,v in {'uid':pvc_meta['uid'],'name':'pgdata-voice-postgres-0','namespace':'voice-dev'}.items())
            or nm.get('name')!=node_name or not nm.get('uid') or nm.get('deletionTimestamp')
            or node.get('status',{}).get('nodeInfo',{}).get('operatingSystem')!='linux'
            or not any(row.get('type')=='Ready' and row.get('status')=='True' for row in node.get('status',{}).get('conditions',[]))):
            raise PhysicalError('physical objects changed')
        mounts=[row for row in containers[0].get('volumeMounts',[]) if row.get('name')==volumes[0].get('name')]
        if (len(mounts)!=1 or mounts[0].get('mountPath')!=MOUNT or mounts[0].get('subPath') or mounts[0].get('subPathExpr')
            or mounts[0].get('readOnly') is True):raise PhysicalError('physical mount changed')
        role=self.catalog.inventory._admin_role()
        args=['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
              '--request-timeout=15s','exec','voice-postgres-0','-c','postgres','--','sh','-ec',schema.PSQL_SCRIPT,'--','postgres',SQL]
        self._guard();self.cut.verify_drained();raw=schema.linux_psql(args);self._guard();self.cut.verify_drained()
        observation=json.loads(raw)
        if (type(observation) is not dict or set(observation)!={'database','database_oid','role','server_start','read_only',
                'data_directory','pg_major','system_identifier'} or observation['database']!='postgres'
            or observation['database_oid']!=catalog['databases']['postgres'] or type(observation['database_oid']) is not int
            or observation['role']!=role or observation['server_start']!=catalog['server_start']
            or observation['read_only']!='on' or observation['data_directory']!=MOUNT or observation['pg_major']!=16
            or type(observation['pg_major']) is not int or not isinstance(observation['system_identifier'],str)
            or not re.fullmatch(r'[1-9][0-9]{0,19}',observation['system_identifier'])):
            raise PhysicalError('physical postgres changed')
        expected={'namespace_uid':self.namespace_uid,'pod_uid':pm['uid'],'statefulset_uid':sm['uid'],
            'pvc_uid':pvc_meta['uid'],'pv_name':name,'pv_uid':pvm['uid'],'container':'postgres',
            'container_id':statuses[0]['containerID'],'restart_count':statuses[0]['restartCount'],
            'mount_path':mounts[0]['mountPath'],'data_directory':observation['data_directory'],'pg_major':observation['pg_major']}
        self._mount_proof(expected)
        if snap()!=engine:raise PhysicalError('physical engine changed')
        self._guard();self.cut.verify_drained()
        return {'expected':expected,'engine':copy.deepcopy(engine),'server_start':observation['server_start'],
                'database_oid':observation['database_oid'],'system_identifier':observation['system_identifier'],
                'node':{'name':node_name,'uid':nm['uid']}}

    def run(self,token):
        pending=False
        try:
            self._guard();self.cut.verify_drained();catalog=self.catalog.verify_current(token)
            row,revision=self._ledger();catalog_marker=row['full_cohort_db_catalog']
            if (catalog_marker.get('receipt_sha256')!=_digest(catalog) or STEP in self.journal.state['steps'] or LEDGER in row):raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id']}
            pending=True;self.journal.state['steps'][STEP]=copy.deepcopy(marker);self.journal.save()
            self._guard();current,actual=self._ledger()
            if current!=row or actual!=revision or self.journal.state['steps'][STEP]!=marker:raise ValueError()
            current[LEDGER]=copy.deepcopy(marker);current['step']=STEP+'-pending'
            self.fence.ledger.write(actual,current);self._guard()
            first=self._observe(catalog)
            if self._observe(catalog)!=first or self.catalog.verify_current(token)!=catalog:raise ValueError()
            self.cut.verify_drained();self._guard();current,actual=self._ledger()
            if current.get(LEDGER)!=marker or self.journal.state['steps'].get(STEP)!=marker:raise ValueError()
            receipt={'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                'catalog_receipt_sha256':_digest(catalog),'physical':first}
            done=marker|{'status':'done','receipt_sha256':_digest(receipt)}
            local=done|{'receipt':copy.deepcopy(receipt)};self.journal.state['steps'][STEP]=copy.deepcopy(local);self.journal.save()
            current[LEDGER]=copy.deepcopy(done);current['step']=STEP+'-done'
            self.fence.ledger.write(actual,current);self._guard()
            return verify_current(self,token)
        except BaseException:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('physical identity outcome uncertain')
                except BaseException:pass
            raise PhysicalError('physical identity unavailable; reconcile' if pending else 'physical identity preflight refused') from None


def verify_receipt(phase,token):
    """Authenticate historical physical evidence; this does not reobserve PostgreSQL."""
    try:
        if type(phase) is not PhysicalPhase:raise ValueError()
        phase._guard();local=copy.deepcopy(phase.journal.state['steps'][STEP])
        receipt=local['receipt'];marker={key:value for key,value in local.items() if key!='receipt'}
        row,revision=phase._ledger();row=copy.deepcopy(row)
        if (set(marker)!={'status','nonce','operation_id','receipt_sha256'} or marker['status']!='done'
            or marker['operation_id']!=phase.fence.context['operation_id'] or row.get(LEDGER)!=marker
            or marker['receipt_sha256']!=_digest(receipt) or receipt['operation_id']!=marker['operation_id']
            or receipt['source_sha']!=phase.fence.context['commit']):raise ValueError()
        catalog=phase.catalog.verify_receipt(token)
        if (receipt['catalog_receipt_sha256']!=_digest(catalog) or
                receipt['physical']['engine']!=catalog['engine'] or
                receipt['physical']['server_start']!=catalog['server_start'] or
                receipt['physical']['database_oid']!=catalog['databases']['postgres']):
            raise ValueError()
        if phase.catalog.verify_receipt(token)!=catalog:raise ValueError()
        phase._guard();final,final_revision=phase._ledger()
        if (final!=row or final_revision!=revision or phase.journal.dirfd is None or phase.journal.lockfd is None or
                phase.journal.state['steps'].get(STEP)!=local):raise ValueError()
        return copy.deepcopy(receipt)
    except BaseException:
        raise PhysicalError('physical identity receipt unavailable') from None


def verify_current(phase,token):
    try:
        receipt=verify_receipt(phase,token);phase.cut.verify_drained()
        catalog=phase.catalog.verify_current(token)
        if receipt['catalog_receipt_sha256']!=_digest(catalog) or phase._observe(catalog)!=receipt['physical']:
            raise ValueError()
        phase.cut.verify_drained()
        if verify_receipt(phase,token)!=receipt:raise ValueError()
        return copy.deepcopy(receipt['physical']['expected'])
    except BaseException:
        raise PhysicalError('physical identity no longer current') from None
