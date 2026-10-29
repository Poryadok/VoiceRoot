"""WIP C1 continuation helpers; no complete activation entrypoint yet."""
import copy
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import selectors
import stat
import subprocess
import time

import canonical_application_package as canonical
import dev_peer_issuer_preflight as issuer
import full_cohort_supervisor as supervisor_module
import frontend_runtime_executor
import full_cohort_affected_plan
import full_cohort_inventory_phase
import full_cohort_package_phase
import full_cohort_pg_physical_identity as physical
import media_operation_guard
import media_operation_journal
import publish_candidate
import realtime_absent_phase
import realtime_migration_job as migration
import realtime_db_provision
import realtime_secret_phase
import realtime_secret_provision
import resource_fencing
import realtime_config_transition_authority as config_authority
import space_creator_runtime_profile_delegated as delegated
import space_creator_runtime_profile_target_trust as target_trust
import space_creator_runtime_profile_trust_effect as trust_effect
import space_fresh_install_phase as fresh
import space_role_peer_create as role
import space_native_tool_identity as native
import story_media_backup
import writer_cut


class ActivationError(RuntimeError): pass

REALTIME_STEP='c1-realtime-preparation'
REALTIME_LEDGER='c1_realtime_preparation'
_SEALS=fcntl.F_SEAL_WRITE|fcntl.F_SEAL_GROW|fcntl.F_SEAL_SHRINK|fcntl.F_SEAL_SEAL


def _migrate_manifest(digest):
    if digest not in (migration.MIGRATE_IMAGE.rsplit('@',1)[1],*migration.PLATFORMS.values()): raise ValueError()
    process=subprocess.Popen(['docker','buildx','imagetools','inspect','--raw','migrate/migrate@'+digest],
                             stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    selector=selectors.DefaultSelector(); data=bytearray(); deadline=time.monotonic()+30
    try:
        selector.register(process.stdout,selectors.EVENT_READ)
        while selector.get_map():
            remaining=deadline-time.monotonic()
            if remaining<=0 or not selector.select(remaining): raise ValueError()
            chunk=os.read(process.stdout.fileno(),min(65536,(2<<20)+1-len(data)))
            if not chunk:selector.unregister(process.stdout); break
            data.extend(chunk)
            if len(data)>2<<20:raise ValueError()
        if process.wait(timeout=max(.01,deadline-time.monotonic())):raise ValueError()
        return bytes(data)
    finally:
        selector.close()
        if process.poll() is None:process.kill();process.wait()
        process.stdout.close()


def _qualified_migrate(architecture):
    if architecture not in migration.PLATFORMS:raise ValueError()
    index_digest=migration.MIGRATE_IMAGE.rsplit('@',1)[1]; child=migration.PLATFORMS[architecture]
    index=publish_candidate.decode_manifest(_migrate_manifest(index_digest),index_digest)
    entries=[item for item in index.get('manifests',[]) if
             item.get('platform',{}).get('os')=='linux' and item.get('platform',{}).get('architecture')==architecture and
             not item.get('platform',{}).get('variant')]
    if len(entries)!=1 or entries[0].get('digest')!=child or type(entries[0].get('size')) is not int or entries[0]['size']<=0:raise ValueError()
    raw=_migrate_manifest(child)
    if len(raw)!=entries[0]['size'] or publish_candidate.decode_manifest(raw,child).get('schemaVersion')!=2:raise ValueError()
    return child


def _dev(supervisor):
    return (type(supervisor) is supervisor_module.Supervisor and supervisor.context.get('project')=='voice' and
            supervisor.context.get('resource')=='DEV' and bool(re.fullmatch(r'media-c1-[a-f0-9]{32}',supervisor.context.get('operation_id',''))))


class RealtimePreparation:
    """Pre-drain fixed source, absent baseline, node and immutable tool admission."""
    def __init__(self,journal,fence,client,lock,namespace_uid,supervisor):
        if (not _dev(supervisor) or type(journal) is not media_operation_journal.Journal or
                type(fence) is not resource_fencing.Fence or type(client) is not frontend_runtime_executor.KubectlTransport or
                client.journal is not journal or fence.ledger.client is not client or
                type(supervisor.package) is not full_cohort_package_phase.PackagePhase or
                type(supervisor.inventory) is not full_cohort_inventory_phase.InventoryPhase or
                type(supervisor.catalog) is not full_cohort_affected_plan.FullCatalogPhase or type(supervisor.cut) is not writer_cut.WriterCut or
                type(supervisor.realtime_absent) is not realtime_absent_phase.RealtimeAbsentPhase or
                supervisor.realtime_absent.inventory is not supervisor.inventory or supervisor.realtime_absent.catalog is not supervisor.catalog or
                fence.context!=supervisor.context or lock!=supervisor.lock or namespace_uid!=supervisor.namespace_uid or
                journal.intent.get('resource_context')!=fence.context or journal.intent.get('namespace_uid')!=namespace_uid):
            raise ActivationError('Realtime preflight identity unavailable')
        self.journal,self.fence,self.client,self.supervisor=journal,fence,client,supervisor
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid; self.intent=copy.deepcopy(journal.intent)
        self.refs=(journal,fence,client,fence.ledger,supervisor,supervisor.package,supervisor.inventory,supervisor.catalog,supervisor.cut,supervisor.realtime_absent)
        self.context=copy.deepcopy(fence.context); self.custody=native.SpaceNativeToolPhase._custody(journal)
        self.dependencies=(supervisor.package,supervisor.inventory,supervisor.catalog,supervisor.cut,supervisor.realtime_absent)
        self.dependency_refs=tuple((p,p.journal,p.fence,p.client,p.fence.ledger) for p in self.dependencies)
        self.dependency_custody=tuple(native.SpaceNativeToolPhase._custody(p.journal) for p in self.dependencies)
        self.dependency_intents=tuple(copy.deepcopy(p.journal.intent) for p in self.dependencies)
        self._bound_dependencies,self._bound_refs,self._bound_custody,self._bound_intents=(self.dependencies,self.dependency_refs,self.dependency_custody,self.dependency_intents)
        self._fixed=copy.deepcopy((self.context,self.lock,self.namespace_uid,self.intent,self.dependency_intents,self.dependency_custody))
        self.record=None; self.poisoned=False

    def _local(self):
        s=self.supervisor
        if (self.poisoned or (self.context,self.lock,self.namespace_uid,self.intent,self.dependency_intents,self.dependency_custody)!=self._fixed or
                self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                self.fence.context!=self.context or self.lock!=s.lock or self.namespace_uid!=s.namespace_uid or
                self.client.journal is not self.journal or self.fence.ledger.client is not self.client or
                any(a is not b for a,b in zip(self.refs,(self.journal,self.fence,self.client,self.fence.ledger,s,s.package,s.inventory,s.catalog,s.cut,s.realtime_absent))) or
                self.dependencies is not self._bound_dependencies or self.dependency_refs is not self._bound_refs or
                self.dependency_custody is not self._bound_custody or self.dependency_intents is not self._bound_intents or
                any(p.journal is not j or p.fence is not f or p.client is not c or f.ledger is not ledger or
                    c.journal is not j or ledger.client is not c or f.context!=self.context or p.lock!=self.lock or
                    p.namespace_uid!=self.namespace_uid or j.poisoned or j.intent!=intent or j.state.get('intent')!=intent or
                    native.SpaceNativeToolPhase._custody(j)!=custody
                    for (p,j,f,c,ledger),custody,intent in zip(self.dependency_refs,self.dependency_custody,self.dependency_intents)) or
                native.SpaceNativeToolPhase._custody(self.journal)!=self.custody):raise ValueError()
        if self.record is not None:
            local=self.journal.state['steps'].get(REALTIME_STEP)
            if not isinstance(local,dict) or {k:v for k,v in local.items() if k!='receipt'}!=self.record:raise ValueError()

    def _guard(self):
        self._local(); self.supervisor._guard(); media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        row,rev=self.fence.ledger.read()
        if (self.supervisor.fence.ledger.read()!=(row,rev) or any(p.fence.ledger.read()!=(row,rev) for p in self.dependencies) or
                not self.fence.owned(row) or row.get('status')!='active' or
                (self.record is not None and row.get(REALTIME_LEDGER)!=self.record)):raise ValueError()
        self._local(); return row,rev

    def _dependency_state(self):
        return tuple(copy.deepcopy((p.journal.intent,p.journal.state)) for p in self.dependencies)

    def _observe(self,token):
        s=self.supervisor; self._guard()
        if s.cut.started or s.cut.drained:raise ValueError()
        s._creator_guard(token)
        package=s.package.verify_current(token); absent=s.realtime_absent.verify_current(token); catalog=s.catalog.verify_current(token)
        dependency_state=self._dependency_state()
        if 'realtime_db' in catalog['all_databases']:raise ValueError()
        snapshot=publish_candidate.source_snapshot(canonical.ROOT,self.context['commit']); prefix='src/backend/migrations/realtime_db/'
        names={'000001_admission.up.sql','000001_admission.down.sql'}; files=snapshot['files']
        if (snapshot['source_sha']!=self.context['commit'] or {k[len(prefix):] for k in files if k.startswith(prefix)}!=names or
                files.get('src/backend/realtime/internal/lifecycle/store.go')!='1e34ba05a4831031409bb90e37b541b8adfbe233cb307618cbb6ba471fb5fdc9'):raise ValueError()
        for name in names:
            data=(canonical.ROOT/prefix/name).read_bytes()
            if len(data)>1048576 or hashlib.sha256(data).hexdigest()!=files[prefix+name]:raise ValueError()
        self._guard(); pod=self.client.call('get','pod','voice-postgres-0','-o','json'); self._guard()
        meta=pod.get('metadata',{}); node_name=pod.get('spec',{}).get('nodeName')
        if (meta.get('name')!='voice-postgres-0' or meta.get('namespace')!='voice-dev' or meta.get('uid')!=catalog['engine']['pod_uid'] or
                meta.get('deletionTimestamp') or not isinstance(node_name,str) or not node_name):raise ValueError()
        nodes=self.client.call('get','nodes','-o','json').get('items'); self._guard()
        if not isinstance(nodes,list):raise ValueError()
        matches=[item for item in nodes if item.get('metadata',{}).get('name')==node_name]
        if len(matches)!=1:raise ValueError()
        item=matches[0]; nm=item['metadata']; labels=nm.get('labels',{}); info=item.get('status',{}).get('nodeInfo',{})
        node={'name':node_name,'uid':nm.get('uid'),'hostname':labels.get('kubernetes.io/hostname'),'architecture':labels.get('kubernetes.io/arch')}
        if (any(not isinstance(node[k],str) or not node[k] for k in node) or node['architecture'] not in migration.PLATFORMS or
                any(not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?',node[k]) for k in ('name','hostname')) or
                labels.get('kubernetes.io/os')!='linux' or info.get('operatingSystem')!='linux' or info.get('architecture')!=node['architecture'] or
                nm.get('deletionTimestamp') or item.get('spec',{}).get('unschedulable',False) is not False or
                not any(c.get('type')=='Ready' and c.get('status')=='True' for c in item.get('status',{}).get('conditions',[])) or
                sum(x.get('metadata',{}).get('labels',{}).get('kubernetes.io/hostname')==node['hostname'] for x in nodes)!=1 or
                package.get('nodes',[]).count(node)!=1):raise ValueError()
        platform=_qualified_migrate(node['architecture']); s._creator_guard(token); self._guard(); self._local()
        if self._dependency_state()!=dependency_state:raise ValueError()
        return {'operation_id':self.context['operation_id'],'source_sha':self.context['commit'],'namespace_uid':self.namespace_uid,
                'absent_sha256':writer_cut.digest(absent),'catalog_sha256':writer_cut.digest(catalog),
                'node':node,'image':migration.MIGRATE_IMAGE,'platform_digest':platform,
                'migration_files':{name:files[prefix+name] for name in sorted(names)},'scope':'realtime-create-only-predrain-plan'}

    def run(self,token):
        pending=False
        try:
            first=self._observe(token); row,rev=self._guard()
            if REALTIME_STEP in self.journal.state['steps'] or REALTIME_LEDGER in row:raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.context['operation_id']}
            pending=True; self.journal.state['steps'][REALTIME_STEP]=copy.deepcopy(marker); self.journal.save()
            if self._guard()!=(row,rev):raise ValueError()
            row[REALTIME_LEDGER]=copy.deepcopy(marker); self.fence.ledger.write(rev,row); self.record=copy.deepcopy(marker)
            if self._observe(token)!=first:raise ValueError()
            row,rev=self._guard(); done=marker|{'status':'done','receipt_sha256':writer_cut.digest(first)}
            self.journal.state['steps'][REALTIME_STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(first)}; self.journal.save()
            self.supervisor._guard(); media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
            if (native.SpaceNativeToolPhase._custody(self.journal)!=self.custody or self.journal.intent!=self.intent or
                    self.journal.state.get('intent')!=self.intent or self.journal.state['steps'].get(REALTIME_STEP)!=done|{'receipt':first}):raise ValueError()
            current,actual=self.fence.ledger.read()
            if (current,actual)!=(row,rev):raise ValueError()
            current[REALTIME_LEDGER]=copy.deepcopy(done); self.fence.ledger.write(rev,current); self.record=copy.deepcopy(done)
            return verify_realtime_current(self,token)
        except BaseException:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('Realtime preparation uncertain')
                except BaseException:pass
            raise ActivationError('Realtime preparation unavailable; reconcile if reserved') from None


def verify_realtime_current(phase,token):
    try:
        if type(phase) is not RealtimePreparation or phase.record is None or phase.record.get('status')!='done':raise ValueError()
        row,rev=phase._guard(); local=copy.deepcopy(phase.journal.state['steps'][REALTIME_STEP]); receipt=local['receipt']
        if phase.record.get('receipt_sha256')!=writer_cut.digest(receipt) or phase._observe(token)!=receipt:raise ValueError()
        dependencies=phase._dependency_state()
        if phase._guard()!=(row,rev) or phase.journal.state['steps'].get(REALTIME_STEP)!=local:raise ValueError()
        phase._local()
        if phase._dependency_state()!=dependencies:raise ValueError()
        return copy.deepcopy(receipt)
    except BaseException:raise ActivationError('Realtime preparation unavailable') from None


def verify_realtime_receipt(phase,token):
    """Reopen the protected pre-drain plan without requiring old Pods to exist."""
    try:
        if type(phase) is not RealtimePreparation or phase.record is None or phase.record.get('status')!='done':raise ValueError()
        row,rev=phase._guard(); local=copy.deepcopy(phase.journal.state['steps'][REALTIME_STEP]); receipt=local['receipt']
        if phase.record.get('receipt_sha256')!=writer_cut.digest(receipt):raise ValueError()
        package=phase.supervisor.package.verify_current(token); absent=phase.supervisor.realtime_absent.verify_receipt(token)
        catalog=phase.supervisor.catalog.verify_receipt(token); node=receipt['node']
        dependencies=phase._dependency_state()
        if (receipt['operation_id']!=phase.context['operation_id'] or receipt['source_sha']!=phase.context['commit'] or
                receipt['namespace_uid']!=phase.namespace_uid or receipt['scope']!='realtime-create-only-predrain-plan' or
                receipt['absent_sha256']!=writer_cut.digest(absent) or receipt['catalog_sha256']!=writer_cut.digest(catalog) or
                receipt['image']!=migration.MIGRATE_IMAGE or receipt['platform_digest']!=migration.PLATFORMS.get(node.get('architecture')) or
                package.get('nodes',[]).count(node)!=1):raise ValueError()
        if phase._guard()!=(row,rev) or phase.journal.state['steps'].get(REALTIME_STEP)!=local:raise ValueError()
        phase._local()
        if phase._dependency_state()!=dependencies:raise ValueError()
        return copy.deepcopy(receipt)
    except BaseException:raise ActivationError('Realtime preparation receipt unavailable') from None


class _RealtimeCredential:
    """One operation-local sealed credential; never serialized into a receipt."""
    def __init__(self):
        value=secrets.token_urlsafe(32).encode('ascii'); self.fd=os.memfd_create('realtime-operation',os.MFD_CLOEXEC|os.MFD_ALLOW_SEALING)
        try:
            os.fchmod(self.fd,0o600); view=memoryview(value)
            while view:
                count=os.write(self.fd,view)
                if count<=0:raise ValueError()
                view=view[count:]
            fcntl.fcntl(self.fd,fcntl.F_ADD_SEALS,_SEALS); info=os.fstat(self.fd)
            self.identity=(info.st_dev,info.st_ino,info.st_size,info.st_uid)
            if self.read()!=value:raise ValueError()
        except BaseException:
            os.close(self.fd); self.fd=None; raise

    def read(self):
        if type(self.fd) is not int:raise ValueError()
        info=os.fstat(self.fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink!=0 or info.st_uid!=os.geteuid() or
                stat.S_IMODE(info.st_mode)!=0o600 or not 32<=info.st_size<=128 or
                (info.st_dev,info.st_ino,info.st_size,info.st_uid)!=self.identity or
                fcntl.fcntl(self.fd,fcntl.F_GET_SEALS)&_SEALS!=_SEALS):raise ValueError()
        value=os.pread(self.fd,info.st_size,0)
        if len(value)!=info.st_size or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}',value):raise ValueError()
        return value

    def close(self):
        if self.fd is not None:os.close(self.fd); self.fd=None


def _recheck_realtime_node(supervisor,node):
    supervisor._guard(); supervisor.cut.verify_drained()
    items=supervisor.client.call('get','nodes','-o','json').get('items'); supervisor._guard()
    if not isinstance(items,list):raise ValueError()
    matches=[item for item in items if item.get('metadata',{}).get('labels',{}).get('kubernetes.io/hostname')==node['hostname']]
    if len(matches)!=1:raise ValueError()
    item=matches[0]; meta=item.get('metadata',{}); labels=meta.get('labels',{}); info=item.get('status',{}).get('nodeInfo',{})
    if (meta.get('name')!=node['name'] or meta.get('uid')!=node['uid'] or meta.get('deletionTimestamp') or
            labels.get('kubernetes.io/arch')!=node['architecture'] or labels.get('kubernetes.io/os')!='linux' or
            info.get('architecture')!=node['architecture'] or info.get('operatingSystem')!='linux' or
            item.get('spec',{}).get('unschedulable',False) is not False or
            not any(c.get('type')=='Ready' and c.get('status')=='True' for c in item.get('status',{}).get('conditions',[]))):raise ValueError()
    supervisor.cut.verify_drained(); supervisor._guard()


def _realtime_database(driver,preparation,token):
    """Create only an independently proven absent Realtime role/database and migrate it."""
    import full_cohort_c1_driver as driver_module
    if (type(driver) is not driver_module.C1Driver or not _dev(driver.supervisor) or
            type(preparation) is not RealtimePreparation or preparation.supervisor is not driver.supervisor):
        raise ActivationError('Realtime admission unavailable')
    s=driver.supervisor; credential=None; attempted=False
    try:
        plan=verify_realtime_receipt(preparation,token); s.cut.verify_drained(); _recheck_realtime_node(s,plan['node'])
        data=physical.verify_receipt(s.physical,token)['physical']; engine=data['engine']
        if data['node']!={k:plan['node'][k] for k in ('name','uid')}:raise ValueError()
        pg=realtime_db_provision.KubePostgres(lambda:(s._guard(),s.cut.verify_drained()))
        if pg.snapshot()!=engine or pg.catalog()!={'role':None,'database':None} or pg.snapshot()!=engine:raise ValueError()
        credential=_RealtimeCredential(); password=credential.read(); attempted=True
        j,f,c=s._open('realtime-secret-journal',{})
        secret=realtime_secret_phase.SecretPhase(j,f,c,s.lock,s.namespace_uid); secret.bind_cut(s.cut); secret_receipt=secret.run(password)
        if credential.read()!=password:raise ValueError()
        j,f,c=s._open('realtime-provision-journal',{'postgres':copy.deepcopy(engine),'realtime_credential':copy.deepcopy(secret_receipt)})
        provision,provision_receipt=realtime_secret_provision.run_owned(secret,j,c,f,s.lock,s.namespace_uid,password,cut=s.cut)
        s.cut.bind_provision_scope(provision); _recheck_realtime_node(s,plan['node'])
        public={key:secret_receipt[key] for key in ('name','uid','resourceVersion')}
        spec={'source_sha':s.context['commit'],'image':plan['image'],'platform_digest':plan['platform_digest'],
              'engine':copy.deepcopy(engine),'database_oid':provision_receipt['database_oid'],
              'credential':public,'node':copy.deepcopy(plan['node'])}
        mj,mf,mc=s._open('realtime-migration-journal',{'realtime_migration':spec})
        phase=migration.Migration(secret,j,mj,canonical.ROOT,password,mc,mf); phase.run(s.cut)
        if credential.read()!=password:raise ValueError()
        s.cut.bind_realtime_scope(); s._guard()
        return credential,secret,provision,phase
    except BaseException:
        if attempted:
            try:s.fence.uncertain('Realtime creation or migration uncertain')
            except BaseException:pass
        if credential is not None:credential.close()
        raise ActivationError('Realtime creation unavailable; retain claim if attempted') from None


def _profile_directory(supervisor,profile):
    """Reserve one fixed later-profile socket directory without adoption."""
    if not _dev(supervisor) or profile not in ('space','role'):
        raise ActivationError('profile authority unavailable')
    context=supervisor.context
    match=re.fullmatch(r'media-c1-([a-f0-9]{32})',context.get('operation_id',''))
    if not match or context.get('project')!='voice' or context.get('resource')!='DEV' or issuer.SOCKET_ROOT!=Path('/home/pmd/voice-dev-peer-issuer/preflight'):
        raise ActivationError('profile directory unavailable')
    try:
        supervisor._guard(); parent=issuer.SOCKET_ROOT/match[1]
        with story_media_backup.private_directory(parent) as fd:
            before=os.fstat(fd); visible=os.stat(parent,follow_symlinks=False)
            if ((before.st_dev,before.st_ino)!=(visible.st_dev,visible.st_ino) or
                    before.st_uid!=os.geteuid() or stat.S_IMODE(before.st_mode)!=0o700): raise ValueError()
            os.mkdir(profile,0o700,dir_fd=fd); os.fsync(fd)
            child=os.open(profile,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            try:
                current=os.fstat(child); named=os.stat(profile,dir_fd=fd,follow_symlinks=False)
                after=os.fstat(fd); parent_visible=os.stat(parent,follow_symlinks=False)
                if (not stat.S_ISDIR(current.st_mode) or current.st_uid!=os.geteuid() or
                        stat.S_IMODE(current.st_mode)!=0o700 or (current.st_dev,current.st_ino)!=(named.st_dev,named.st_ino) or
                        (before.st_dev,before.st_ino)!=(after.st_dev,after.st_ino) or
                        (before.st_dev,before.st_ino)!=(parent_visible.st_dev,parent_visible.st_ino)): raise ValueError()
            finally: os.close(child)
        supervisor._guard()
    except BaseException:
        raise ActivationError('profile directory outcome unavailable; reconcile') from None


def _later_receiver(driver,token,profile):
    """Construct the fixed fresh post-effect preflight and its matching receiver."""
    import full_cohort_c1_driver as driver_module
    if type(driver) is not driver_module.C1Driver or profile not in ('space','role'):
        raise ActivationError('profile receiver authority unavailable')
    s=driver.supervisor; source=str(canonical.ROOT)
    if not _dev(s) or not isinstance(token,str) or not token:
        raise ActivationError('profile receiver authority unavailable')
    try:
        s._guard()
        j,f,c=s._open(profile+'-delegated-preflight-journal',{'dev_peer_issuer_preflight':{
            'source_root':source,'source_sha':s.context['commit'],'profile':profile}})
        phase=issuer.PeerIssuerPreflightPhase(j,f,c,s.lock,s.namespace_uid,s.package,s.cut,source,token,profile)
        driver_module._handshake(phase,token)
        j,f,c=s._open(profile+'-delegated-receiver-journal',{'peer_receiver':{
            'profile':profile,'source_sha':s.context['commit']}})
        if profile=='space':
            receiver=delegated.SpaceReceiver(j,f,c,s.lock,s.namespace_uid,s.package,s.cut,phase,source,token)
            result=receiver.run()
            if delegated.verify_completed(receiver,token)!=result: raise ValueError()
        else:
            receiver=role.Session(j,f,c,s.lock,s.namespace_uid,s.package,phase,canonical.ROOT,token)
            receiver.prepare(); result=receiver.run()
            if role.verify_completed(receiver,token)!=result: raise ValueError()
        s._guard()
        return receiver
    except BaseException:
        raise ActivationError('profile receiver outcome unavailable; reconcile') from None


def _target_trust(driver,token):
    """Materialize and verify the exact target trust before Creator effects."""
    import full_cohort_c1_driver as driver_module
    if type(driver) is not driver_module.C1Driver or type(driver.trust) is not trust_effect.TrustEffectPhase:
        raise ActivationError('target trust authority unavailable')
    s=driver.supervisor
    if (not _dev(s) or driver.trust.preflight is not driver.preflight or
            driver.trust.package is not s.package or driver.trust.inventory is not s.inventory or
            driver.trust.jdk is not s.target_jdk or driver.trust.fence.context!=s.context or
            driver.trust.lock!=s.lock or driver.trust.namespace_uid!=s.namespace_uid):
        raise ActivationError('target trust authority unavailable')
    try:
        s._guard(); driver.trust.run(token); trust_effect.verify_current(driver.trust,token)
        j,f,c=s._open('creator-target-trust-journal',{})
        phase=target_trust.TargetTrustPhase(j,f,c,s.lock,s.namespace_uid,s.package,s.inventory,s.target_jdk,driver.preflight)
        receipt=phase.run(token)
        if target_trust.verify_current(phase,token)!=receipt: raise ValueError()
        s._guard()
        return phase
    except BaseException:
        raise ActivationError('target trust outcome unavailable; reconcile') from None


def _creator_and_space(driver,token):
    """Run the fixed C1 peer order, followed by the authorized DEV Space install."""
    import full_cohort_c1_driver as driver_module
    if (type(driver) is not driver_module.C1Driver or not _dev(driver.supervisor) or type(driver.receiver) is not delegated.CreatorReceiver or
            driver.receiver.preflight is not driver.preflight or type(driver.supervisor.space) is not fresh.SpaceFreshInstallPhase):
        raise ActivationError('Creator and Space authority unavailable')
    try:
        trust=_target_trust(driver,token)
        creator_receipt=driver.receiver.run()
        if delegated.verify_completed(driver.receiver,token)!=creator_receipt: raise ValueError()
        space_receiver=_later_receiver(driver,token,'space')
        role_receiver=_later_receiver(driver,token,'role')
        space_authority=config_authority.bind_delegated_space(driver.supervisor.cut,trust,driver.receiver,space_receiver,role_receiver,token)
        space_receipt=driver.supervisor.space.run()
        if fresh.verify_completed(driver.supervisor.space,token)!=space_receipt: raise ValueError()
        return trust,space_receiver,role_receiver,space_authority
    except BaseException:
        raise ActivationError('Creator and Space outcome unavailable; reconcile') from None


def _realtime_p3(driver,token):
    """Run the fixed delegated seven-object handoff after Space is installed."""
    import full_cohort_c1_driver as driver_module
    import dev_realtime_p3_issuer_capability as capability_module
    import realtime_p3_create_only as creation
    import realtime_p3_create_only_source as source
    if type(driver) is not driver_module.C1Driver or not _dev(driver.supervisor):
        raise ActivationError('Realtime P3 authority unavailable')
    s=driver.supervisor; capability=driver.p3_capability
    if (type(capability) is not capability_module.RealtimeP3Capability or
            capability.creator_preflight is not driver.preflight or capability.token!=token or
            driver.preflight.cut is not s.cut or driver.preflight.package is not s.package):
        raise ActivationError('Realtime P3 authority unavailable')
    phase=None; attempted=False
    try:
        s._guard(); s.cut.verify_drained()
        directory=driver_module.verify_p3_directory(driver,token)
        if directory!={'operation_id':s.context['operation_id'],'scope':'owned-p3-socket-directory'}:raise ValueError()
        capability_module.verify_current(capability,token)
        match=re.fullmatch(r'media-c1-([0-9a-f]{32})',s.context['operation_id'])
        if match is None:raise ValueError()
        plan={'issuer_source_sha':source.REVIEWED_ISSUER_SHA,'source_repo':str(canonical.ROOT),'issuer_suffix':match[1]}
        j,f,c=s._open('realtime-p3-journal',{'realtime_p3_create':plan})
        phase=creation.Session(j,f,c,s.lock,s.namespace_uid)
        if (phase.fence.context!=s.context or phase.lock!=s.lock or phase.namespace_uid!=s.namespace_uid or
                phase.fence.check is not s.fence.check or phase.fence.ledger.read()!=s.fence.ledger.read()):raise ValueError()
        s.cut.verify_drained(); driver_module.verify_p3_directory(driver,token)
        attempted=True; phase.prepare(); s.cut.verify_drained(); driver_module.verify_p3_directory(driver,token)
        receipt=phase.finish_delegated(capability,token)
        if (not phase.completed or phase.handoff.get('status')!='done' or phase.handoff.get('receipt')!=receipt or
                phase.journal.state['steps'].get('realtime-p3-handoff')!=phase.handoff):raise ValueError()
        phase._guard(); s._guard()
        return phase
    except BaseException:
        if attempted:
            try:s.fence.uncertain('Realtime P3 handoff outcome uncertain')
            except BaseException:pass
        if phase is not None:
            try:phase.close()
            except BaseException:pass
        raise ActivationError('Realtime P3 handoff unavailable; reconcile if attempted') from None
