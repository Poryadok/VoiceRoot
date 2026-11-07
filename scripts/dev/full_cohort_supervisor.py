"""Source-owned beginning of a fenced cohort operation.

The operation lock and TT deploy claim must already be held.  This object
keeps every private journal open for the caller; it never releases a lock or
declares a rollout complete.  Later phases consume the verified objects, not
an externally supplied description of package or database state.
"""
import copy
import os
import re

import frontend_runtime_executor
import full_cohort_affected_plan
import full_cohort_inventory_phase
import full_cohort_image_only_baseline
import full_cohort_go_migration_plan
import full_cohort_namespace_census
import full_cohort_package_phase
import media_backup_upload_runner
import media_operation_guard
import media_operation_journal
import realtime_absent_phase
import resource_fencing
import story_media_backup
import canonical_application_package
import full_cohort_pg_physical_identity
import space_fresh_install_phase
import space_native_tool_identity
import space_creator_runtime_profile_config
import space_creator_runtime_profile_jdk
import space_creator_peer_phase
import space_p3_issuer_preflight
import writer_cut


class SupervisorError(RuntimeError):
    pass


class OperationBootstrap:
    """Own a new fixed private operation journal through its single begin attempt."""
    def __init__(self,check,lock,namespace_uid):
        if (type(check) is not resource_fencing.ResourceCheck or check.endpoint!=media_backup_upload_runner.TT or
                not isinstance(namespace_uid,str) or not namespace_uid):
            raise SupervisorError('bootstrap authority invalid')
        context=copy.deepcopy(check.context);resource_fencing.validate_context(context)
        operation=context['operation_id']
        if (context['actor']!='scheduled_devops' or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}',operation)):
            raise SupervisorError('bootstrap operation invalid')
        self.check,self.lock,self.namespace_uid=check,copy.deepcopy(lock),namespace_uid
        self.context=context;self.started=False;self.supervisor=None
        with story_media_backup.private_directory(media_backup_upload_runner.ROOT) as directory:
            os.mkdir(operation,0o700,dir_fd=directory);os.fsync(directory)
        root=media_backup_upload_runner.ROOT/operation
        with story_media_backup.private_directory(root):
            self.journal=media_operation_journal.Journal(root/'journal',{
                'resource_context':copy.deepcopy(context),'namespace_uid':namespace_uid,
                'kube_lock':copy.deepcopy(lock),'cohort_route_mode':'full-cohort-v1'})
        self.client=frontend_runtime_executor.KubectlTransport(self.journal)
        self.fence=resource_fencing.Fence(copy.deepcopy(context),check,resource_fencing.KubeLedger(self.client),check.mark_uncertain)
        self._identity=(self.check,self.journal,self.client,self.fence,self.fence.ledger)
        self._fixed=(copy.deepcopy(self.context),copy.deepcopy(self.lock),self.namespace_uid,copy.deepcopy(self.journal.intent))

    def begin(self):
        if self.started:raise SupervisorError('bootstrap already attempted; reconcile')
        if (any(a is not b for a,b in zip((self.check,self.journal,self.client,self.fence,self.fence.ledger),self._identity)) or
                (self.context,self.lock,self.namespace_uid,self.journal.intent)!=self._fixed or
                self.journal.state.get('intent')!=self._fixed[3] or self.journal.poisoned or
                self.journal.dirfd is None or self.journal.lockfd is None or self.check.context!=self.context or
                self.fence.context!=self.context or self.fence.check is not self.check or
                self.client.journal is not self.journal or self.fence.ledger.client is not self.client):
            raise SupervisorError('bootstrap identity changed')
        self.started=True
        self.supervisor=begin_operation(self.journal,self.fence,self.client,self.lock,self.namespace_uid)
        return self.supervisor


def begin_operation(journal,fence,client,lock,namespace_uid):
    """Start the fixed C1/C2 admission route while retaining its private journal."""
    if journal.intent.get('cohort_route_mode')!='full-cohort-v1':
        raise SupervisorError('full cohort route intent required')
    operation=fence.context.get('operation_id','')
    if re.fullmatch(r'media-c1-[a-f0-9]{32}',operation):
        fence.begin_image_only(client,lock,namespace_uid,journal)
    elif re.fullmatch(r'media-c2-[a-f0-9]{32}',operation):
        fence.begin_with_predecessor(client,lock,namespace_uid,journal)
    else: raise SupervisorError('operation route invalid')
    return Supervisor(journal,fence,client,lock,namespace_uid)


class Supervisor:
    def __init__(self, journal, fence, client, lock, namespace_uid):
        if (type(journal) is not media_operation_journal.Journal or
                type(client) is not frontend_runtime_executor.KubectlTransport or
                type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
                type(fence.check) is not resource_fencing.ResourceCheck or
                fence.check.endpoint != media_backup_upload_runner.TT or
                getattr(fence.on_uncertain, '__self__', None) is not fence.check or
                getattr(fence.on_uncertain, '__func__', None) is not resource_fencing.ResourceCheck.mark_uncertain or
                client.journal is not journal or fence.ledger.client is not client or
                journal.intent.get('resource_context') != fence.context or
                journal.intent.get('namespace_uid') != namespace_uid or
                journal.intent.get('cohort_route_mode') != 'full-cohort-v1' or
                not re.fullmatch(r'media-c[12]-[a-f0-9]{32}', fence.context.get('operation_id', '')) or
                journal.path != media_backup_upload_runner.ROOT / fence.context['operation_id'] / 'journal' or
                fence.check.context != fence.context):
            raise SupervisorError('supervisor authority invalid')
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.context = copy.deepcopy(fence.context)
        self.intent = copy.deepcopy(journal.intent)
        self._journal, self._fence, self._client = journal, fence, client
        self._check = fence.check
        self.journals = []
        self.package = self.inventory = self.catalog = self.realtime_absent = self.census = self.baseline = None
        self.cut = self.physical = self.tool = self.space = None
        self.creator_config = self.target_jdk = self.issuer_preflight = None
        self.peer_target = None
        self._creator_preparation_started = False
        self._peer_preparation_started = False
        self._execution_preparation_started = False
        self._guard()

    def _confirmed_admission(self):
        c1=self.context['operation_id'].startswith('media-c1-')
        step_name='image-only-admission' if c1 else 'predecessor-admission'
        capture_name=full_cohort_image_only_baseline.STEP if c1 else 'full-cohort-predecessor'
        marker='image_only_baseline' if c1 else 'predecessor'
        step=copy.deepcopy(self.journal.state['steps'].get(step_name))
        capture=copy.deepcopy(self.journal.state['steps'].get(capture_name)); row,revision=self.fence.ledger.read()
        if (not isinstance(step,dict) or set(step)!={'status','descriptor','old_revision','new_revision'} or
                step.get('status')!='done' or not isinstance(step.get('old_revision'),list) or
                not isinstance(step.get('new_revision'),list) or len(step['old_revision'])!=2 or len(step['new_revision'])!=2 or
                any(not isinstance(v,str) or not v for v in step['old_revision']+step['new_revision']) or
                step['old_revision'][0]!=step['new_revision'][0] or not isinstance(revision,tuple) or len(revision)!=2 or
                revision[0]!=step['new_revision'][0] or not self.fence.owned(row) or row.get('status')!='active' or
                row.get(marker)!=step['descriptor'] or not isinstance(capture,dict) or
                capture.get('descriptor')!=step['descriptor'] or self.journal.dirfd is None or self.journal.lockfd is None or
                self.journal.state['steps'].get(step_name)!=step or
                self.journal.state['steps'].get(capture_name)!=capture):
            raise SupervisorError('operation admission is not confirmed')
        if not c1:
            import full_cohort_predecessor_phase as predecessor
            if (capture.get('status')!='captured' or not isinstance(capture.get('snapshot'),dict) or
                    predecessor.digest(capture['snapshot'])!=step['descriptor'].get('snapshot_sha256')):
                raise SupervisorError('predecessor snapshot changed')

    def _guard(self):
        if (self.journal is not self._journal or self.fence is not self._fence or self.client is not self._client or
                self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or
                self.journal.intent.get('resource_context') != self.context or
                self.journal.intent.get('namespace_uid') != self.namespace_uid or
                self.fence.context != self.context or self.client.journal is not self.journal or
                self.fence.ledger.client is not self.client or self.fence.check is not self._check or
                self._check.context != self.context or self._check.endpoint != media_backup_upload_runner.TT or
                getattr(self.fence.on_uncertain, '__self__', None) is not self._check or
                getattr(self.fence.on_uncertain, '__func__', None) is not resource_fencing.ResourceCheck.mark_uncertain):
            raise SupervisorError('supervisor authority changed')
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        self._confirmed_admission()

    def _open(self, name, extra):
        go_names={'go-migration-'+db+'-journal' for db in full_cohort_go_migration_plan.DATABASES}
        if name not in ('package-journal', 'inventory-journal', 'catalog-journal', 'realtime-absence-journal', 'namespace-census-journal',
                        'writer-cut-journal', 'physical-journal', 'space-native-tool-journal', 'space-fresh-install-journal',
                        'creator-config-journal', 'target-jdk-journal', 'space-issuer-preflight-journal',
                        'creator-peer-target-journal', 'creator-delegated-preflight-journal',
                        'creator-delegated-receiver-journal', 'creator-trust-effect-journal',
                        'creator-target-plan-journal', 'creator-target-trust-journal',
                        'space-delegated-preflight-journal', 'space-delegated-receiver-journal',
                        'role-delegated-preflight-journal', 'role-delegated-receiver-journal',
                        'realtime-p3-journal', 'release-journal', 'core-smoke-journal',
                        'realtime-secret-journal', 'realtime-provision-journal', 'realtime-migration-journal') and name not in go_names:
            raise SupervisorError('phase journal name invalid')
        self._guard()
        operation = self.context['operation_id']
        root = media_backup_upload_runner.ROOT / operation
        with story_media_backup.private_directory(root):
            journal = media_operation_journal.Journal(root / name,
                {'resource_context': copy.deepcopy(self.context), 'namespace_uid': self.namespace_uid, **extra})
        self.journals.append(journal)
        client = frontend_runtime_executor.KubectlTransport(journal)
        fence = resource_fencing.Fence(copy.deepcopy(self.context), self.fence.check,
            resource_fencing.KubeLedger(client), self.fence.on_uncertain)
        self._guard()
        if fence.ledger.read() != self.fence.ledger.read():
            raise SupervisorError('phase ledger differs')
        return journal, fence, client

    def _open_go(self,database):
        if database not in full_cohort_go_migration_plan.DATABASES:
            raise SupervisorError('Go migration database invalid')
        return self._open('go-migration-'+database+'-journal',{})

    def prepare(self, token, actor, run_id, run_attempt, nodes):
        """Capture the canonical package, live inventory and full DB catalog."""
        if any(x is not None for x in (self.package, self.inventory, self.catalog)):
            raise SupervisorError('prepare requires reconciliation')
        journal, fence, client = self._open('package-journal', {'full_cohort_package':
            {'run_id': run_id, 'run_attempt': run_attempt, 'nodes': copy.deepcopy(nodes)}})
        self.package = full_cohort_package_phase.PackagePhase(journal, fence, client, self.lock, self.namespace_uid)
        self.package.run(token, actor)
        if not self.context['operation_id'].startswith('media-c1-'):
            raise SupervisorError('C2 fresh baseline authority unavailable')
        self.baseline=full_cohort_image_only_baseline.ImageOnlyPhase(self.journal,self.fence,self.client,
            self.lock,self.namespace_uid,self.package)
        self.baseline.run(token)
        journal, fence, client = self._open('inventory-journal', {})
        self.inventory = full_cohort_inventory_phase.InventoryPhase(journal, fence, client, self.lock,
                                                                     self.namespace_uid, self.package)
        self.inventory.run(token)
        journal, fence, client = self._open('namespace-census-journal', {})
        self.census = full_cohort_namespace_census.NamespaceCensus(journal,fence,client,self.lock,
                                                                    self.namespace_uid,self.package,self.inventory)
        self.census.run(token)
        journal, fence, client = self._open('catalog-journal', {})
        self.catalog = full_cohort_affected_plan.FullCatalogPhase(journal, fence, client, self.lock,
                                                                  self.namespace_uid, self.inventory)
        self.catalog.run(token)
        observations=self.inventory.verify_receipt(token)['database_observations']['workloads']['realtime']
        if (observations and all(item.get('status')=='hold' and item.get('reason')=='database_configuration_missing'
                                 for item in observations)):
            journal,fence,client=self._open('realtime-absence-journal',{})
            self.realtime_absent=realtime_absent_phase.RealtimeAbsentPhase(journal,fence,client,self.lock,
                self.namespace_uid,self.inventory,self.catalog)
            self.realtime_absent.run(token)
        self._guard()
        return full_cohort_affected_plan.build(self.package,self.inventory,token,self.catalog,self.realtime_absent)

    def prepare_execution(self,token,actor):
        """Construct source-bound pre-drain phases once; no writer is stopped here."""
        if (self._execution_preparation_started or any(x is None for x in
                (self.package,self.inventory,self.catalog,self.census,self.baseline)) or
                not self.context['operation_id'].startswith('media-c1-')):
            raise SupervisorError('execution preparation unavailable or already attempted')
        self._execution_preparation_started=True
        try:
            self._guard();self.baseline.verify_current(token)
            self.census.verify_current(token);self.inventory.verify_current(token)
            plan=full_cohort_affected_plan.build(self.package,self.inventory,token,self.catalog,self.realtime_absent)
            receipt=self.catalog.verify_current(token)
            _,deployments,_=full_cohort_inventory_phase.inventory(self.client)
            if set(plan['writers'])-set(deployments):raise ValueError()
            controllers=[copy.deepcopy(deployments[name]) for name in sorted(plan['writers'])]
            for source in controllers:
                name=source['metadata']['name'];expected=plan['writers'][name]
                if (source.get('kind')!='Deployment' or source['metadata'].get('uid')!=expected['uid'] or
                        source['metadata'].get('generation')!=expected['generation'] or
                        writer_cut.digest(source['spec'])!=expected['spec_sha256']):raise ValueError()
            guards={kind:writer_cut.inventory(self.client.call('get',resource,'-o','json')['items'])
                    for kind,resource in (('jobs','jobs'),('cronjobs','cronjobs'),('hpas','horizontalpodautoscalers'))}
            linked={kind:[item['metadata']['uid'] for item in guards[kind]] for kind in ('jobs','cronjobs')}
            scope={'engine':copy.deepcopy(receipt['engine']),'server_start':receipt['server_start'],
                   'databases':sorted(receipt['all_databases']),'database_oids':copy.deepcopy(receipt['all_databases'])}
            if scope['database_oids']!=plan['global_database_oids']:raise ValueError()
            admission={'controllers':controllers,'guards':guards,'linked':linked,'session_scope':scope}
            self.census.verify_current(token);self.inventory.verify_current(token)
            if self.catalog.verify_current(token)!=receipt:raise ValueError()
            j,f,c=self._open('writer-cut-journal',{'writer_cut':copy.deepcopy(admission)})
            self.cut=writer_cut.WriterCut(j,f,c,self.lock,self.namespace_uid,admission,
                full_cohort_affected_plan.CatalogScopedProbe(self.catalog,token))
            j,f,c=self._open('physical-journal',{})
            self.physical=full_cohort_pg_physical_identity.PhysicalPhase(j,f,c,self.lock,self.namespace_uid,self.catalog,self.cut)
            j,f,c=self._open('space-native-tool-journal',{})
            self.tool=space_native_tool_identity.SpaceNativeToolPhase(j,f,c,self.lock,self.namespace_uid,self.package,actor)
            self.tool.run(token)
            j,f,c=self._open('space-fresh-install-journal',{'space_fresh_install':copy.deepcopy(space_fresh_install_phase.RESET)})
            self.space=space_fresh_install_phase.SpaceFreshInstallPhase(j,f,c,self.lock,self.namespace_uid,
                self.catalog,self.physical,self.cut,self.tool,canonical_application_package.ROOT,token)
            self.space.prepare()
            refs=(self.package,self.inventory,self.catalog,self.census,self.baseline,self.cut,self.physical,self.tool,self.space)
            root_custody=space_native_tool_identity.SpaceNativeToolPhase._custody(self.journal)
            fixed=(copy.deepcopy(self.context),copy.deepcopy(self.lock),self.namespace_uid,copy.deepcopy(self.intent))
            phases=(self.package,self.inventory,self.catalog,self.census,self.cut,self.physical,self.tool,self.space)
            facades=tuple((p,p.journal,p.fence,p.client,p.fence.ledger,copy.deepcopy(p.journal.intent),
                           space_native_tool_identity.SpaceNativeToolPhase._custody(p.journal)) for p in phases)
            self._guard()
            self.census.verify_current(token);self.inventory.verify_current(token)
            if self.catalog.verify_current(token)!=receipt:raise ValueError()
            self.space._identity();self.cut._identity()
            if (any(a is not b for a,b in zip(refs,(self.package,self.inventory,self.catalog,self.census,
                    self.baseline,self.cut,self.physical,self.tool,self.space))) or
                    space_native_tool_identity.SpaceNativeToolPhase._custody(self.journal)!=root_custody or
                    self.journal is not self._journal or self.fence is not self._fence or self.client is not self._client or
                    (self.context,self.lock,self.namespace_uid,self.intent)!=fixed or
                    self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                    self.fence.context!=self.context or self.client.journal is not self.journal or
                    self.fence.ledger.client is not self.client or self.fence.check is not self._check or
                    self._check.context!=self.context or self._check.endpoint!=media_backup_upload_runner.TT or
                    getattr(self.fence.on_uncertain,'__self__',None) is not self._check or
                    getattr(self.fence.on_uncertain,'__func__',None) is not resource_fencing.ResourceCheck.mark_uncertain or
                    self.baseline.journal is not self.journal or self.baseline.fence is not self.fence or
                    self.baseline.client is not self.client or self.baseline.package_phase is not self.package or
                    any(p.journal is not j or p.fence is not f or p.client is not c or f.ledger is not ledger or
                        c.journal is not j or ledger.client is not c or f.context!=self.context or
                        p.lock!=self.lock or p.namespace_uid!=self.namespace_uid or j.poisoned or
                        j.intent!=intent or j.state.get('intent')!=intent or
                        space_native_tool_identity.SpaceNativeToolPhase._custody(j)!=custody
                        for p,j,f,c,ledger,intent,custody in facades) or
                    self.space.cut is not self.cut or self.space.physical is not self.physical or
                    self.space.tool is not self.tool or self.physical.cut is not self.cut or
                    self.cut.admission!=admission or self.cut.started or self.cut.drained):raise ValueError()
            self._execution_refs=refs;self._execution_token=token;self._execution_actor=actor
            self._execution_fixed=(fixed,copy.deepcopy(actor),root_custody,facades)
            return {'operation_id':self.context['operation_id'],'scope':'pre-drain-phases-only'}
        except BaseException:
            raise SupervisorError('execution preparation incomplete; reconcile before retry') from None

    def _creator_guard(self, token):
        refs=(self.package,self.inventory,self.catalog,self.census,self.baseline,self.cut,self.physical,self.tool,self.space)
        if (not hasattr(self,'_execution_refs') or token!=self._execution_token or
                any(a is not b for a,b in zip(refs,self._execution_refs)) or
                self.cut.started or self.cut.drained):
            raise SupervisorError('creator pre-drain authority unavailable')
        self._guard(); self.cut._identity(); self.space._identity()

    def _creator_authority(self):
        fixed,actor,root_custody,facades=self._execution_fixed
        return (self._execution_fixed,self._execution_refs,copy.deepcopy((fixed,actor,root_custody)),
                tuple((p,j,f,c,ledger,copy.deepcopy(intent),custody) for p,j,f,c,ledger,intent,custody in facades))

    def _execution_final(self,token,authority):
        saved,saved_refs,values,saved_facades=authority
        if self._execution_fixed is not saved or self._execution_refs is not saved_refs:raise ValueError()
        refs=(self.package,self.inventory,self.catalog,self.census,self.baseline,self.cut,self.physical,self.tool,self.space)
        fixed,actor,root_custody,facades=self._execution_fixed
        if ((fixed,actor,root_custody)!=values or len(facades)!=len(saved_facades) or
                any(any(a is not b for a,b in zip(current[:5],original[:5])) or current[5:]!=original[5:]
                    for current,original in zip(facades,saved_facades))):raise ValueError()
        if (token!=self._execution_token or any(a is not b for a,b in zip(refs,self._execution_refs)) or
                (self.context,self.lock,self.namespace_uid,self.intent)!=fixed or self._execution_actor!=actor or
                self.cut.started or self.cut.drained or self.journal is not self._journal or
                self.fence is not self._fence or self.client is not self._client or
                self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                self.fence.context!=self.context or self.client.journal is not self.journal or
                self.fence.ledger.client is not self.client or self.fence.check is not self._check or
                self._check.context!=self.context or self._check.endpoint!=media_backup_upload_runner.TT or
                getattr(self.fence.on_uncertain,'__self__',None) is not self._check or
                getattr(self.fence.on_uncertain,'__func__',None) is not resource_fencing.ResourceCheck.mark_uncertain or
                space_native_tool_identity.SpaceNativeToolPhase._custody(self.journal)!=root_custody or
                any(p.journal is not j or p.fence is not f or p.client is not c or f.ledger is not ledger or
                    c.journal is not j or ledger.client is not c or f.context!=self.context or
                    p.lock!=self.lock or p.namespace_uid!=self.namespace_uid or j.poisoned or
                    j.intent!=intent or j.state.get('intent')!=intent or
                    space_native_tool_identity.SpaceNativeToolPhase._custody(j)!=custody
                    for p,j,f,c,ledger,intent,custody in facades)):raise ValueError()
        self.cut._identity();self.space._identity()

    def _creator_final(self,token,phases,locals_,authority):
        self._execution_final(token,authority)
        if (any(a is not b for a,b in zip(phases,(self.creator_config,self.target_jdk,self.issuer_preflight))) or
                any((p.journal.state['steps'],p.record)!=local for p,local in zip(phases,locals_))):raise ValueError()
        for phase in phases:phase._local()

    def _rootless_final(self,token,phases,locals_,authority,predecessors):
        self._execution_final(token,authority)
        if (any(a is not b for a,b in zip(phases,(self.creator_config,self.target_jdk))) or
                any((p.journal.state['steps'],p.record)!=local or p.fence.context!=self.context or
                    p.lock!=self.lock or p.namespace_uid!=self.namespace_uid or p.client.journal is not p.journal or
                    p.fence.ledger.client is not p.client for p,local in zip(phases,locals_)) or
                (self.package.journal.state['steps'],self.inventory.journal.state['steps'])!=predecessors):raise ValueError()
        for phase in phases:phase._local()

    def prepare_rootless_creator_base(self,token):
        """Capture current config and selected JDK without starting an issuer."""
        if self._creator_preparation_started:
            raise SupervisorError('creator preparation already attempted; reconcile')
        self._creator_preparation_started=True
        try:
            authority=self._creator_authority();self._creator_guard(token)
            j,f,c=self._open('creator-config-journal',{})
            self.creator_config=space_creator_runtime_profile_config.CreatorConfigPhase(
                j,f,c,self.lock,self.namespace_uid,self.package,self.inventory)
            self.creator_config.run(token)
            j,f,c=self._open('target-jdk-journal',{})
            self.target_jdk=space_creator_runtime_profile_jdk.TargetJdkPhase(
                j,f,c,self.lock,self.namespace_uid,self.package,self.inventory)
            self.target_jdk.run(token,self._execution_actor)
            phases=(self.creator_config,self.target_jdk)
            space_creator_runtime_profile_config.verify_completed(self.creator_config,token)
            config_local=copy.deepcopy((self.creator_config.journal.state['steps'],self.creator_config.record))
            predecessors=copy.deepcopy((self.package.journal.state['steps'],self.inventory.journal.state['steps']))
            space_creator_runtime_profile_jdk.verify_completed(self.target_jdk,token)
            jdk_local=copy.deepcopy((self.target_jdk.journal.state['steps'],self.target_jdk.record))
            self._creator_guard(token);self._rootless_final(token,phases,(config_local,jdk_local),authority,predecessors)
            self._rootless_base_refs=phases
            return {'operation_id':self.context['operation_id'],'scope':'rootless-creator-base-only'}
        except BaseException:
            raise SupervisorError('rootless creator preparation incomplete; reconcile before retry') from None

    def prepare_peer_targets(self,token):
        """Capture static peer targets before drain; this does not admit effects."""
        if self._peer_preparation_started:
            raise SupervisorError('peer preparation already attempted; reconcile')
        self._peer_preparation_started=True
        try:
            authority=self._creator_authority();phases=self._rootless_base_refs
            entry_predecessors=copy.deepcopy((self.package.journal.state['steps'],self.inventory.journal.state['steps']))
            self._creator_guard(token)
            initial=tuple(copy.deepcopy((p.journal.state['steps'],p.record)) for p in phases)
            if self._rootless_base_refs is not phases:raise ValueError()
            self._rootless_final(token,phases,initial,authority,entry_predecessors)
            j,f,c=self._open('creator-peer-target-journal',{})
            self.peer_target=space_creator_peer_phase.PeerTargetPhase(
                j,f,c,self.lock,self.namespace_uid,self.package,self.inventory,self.creator_config)
            target=self.peer_target;receipt=target.run(token)
            target_local=copy.deepcopy((target.journal.state['steps'],target.record))
            space_creator_runtime_profile_config.verify_completed(self.creator_config,token)
            config_local=copy.deepcopy((self.creator_config.journal.state['steps'],self.creator_config.record))
            predecessors=copy.deepcopy((self.package.journal.state['steps'],self.inventory.journal.state['steps']))
            space_creator_runtime_profile_jdk.verify_completed(self.target_jdk,token)
            jdk_local=copy.deepcopy((self.target_jdk.journal.state['steps'],self.target_jdk.record))
            if space_creator_peer_phase.verify_current(target,token)!=receipt:raise ValueError()
            self._creator_guard(token)
            if self._rootless_base_refs is not phases or self.peer_target is not target:raise ValueError()
            self._rootless_final(token,phases,(config_local,jdk_local),authority,predecessors);target._peer_local()
            if ((target.journal.state['steps'],target.record)!=target_local or target.fence.context!=self.context or
                    target.lock!=self.lock or target.namespace_uid!=self.namespace_uid or
                    target.client.journal is not target.journal or target.fence.ledger.client is not target.client):raise ValueError()
            self._peer_target_ref=target
            return {'operation_id':self.context['operation_id'],'scope':'static-peer-targets-only'}
        except BaseException:
            raise SupervisorError('peer target preparation incomplete; reconcile before retry') from None

    def prepare_creator_inputs(self,token):
        """Start fixed pre-drain observations and the root custody handshake.

        This is not a complete release admission and does not stop writers.
        """
        if self._creator_preparation_started:
            raise SupervisorError('creator preparation already attempted; reconcile')
        self._creator_preparation_started=True
        try:
            authority=self._creator_authority()
            self._creator_guard(token)
            j,f,c=self._open('creator-config-journal',{})
            self.creator_config=space_creator_runtime_profile_config.CreatorConfigPhase(
                j,f,c,self.lock,self.namespace_uid,self.package,self.inventory)
            self.creator_config.run(token)
            j,f,c=self._open('target-jdk-journal',{})
            self.target_jdk=space_creator_runtime_profile_jdk.TargetJdkPhase(
                j,f,c,self.lock,self.namespace_uid,self.package,self.inventory)
            self.target_jdk.run(token,self._execution_actor)
            j,f,c=self._open('space-issuer-preflight-journal',{'space_issuer_preflight':{
                'source_root':str(canonical_application_package.ROOT),'source_sha':self.context['commit']}})
            self.issuer_preflight=space_p3_issuer_preflight.IssuerPreflightPhase(
                j,f,c,self.lock,self.namespace_uid,self.package,self.cut,str(canonical_application_package.ROOT),token)
            phases=(self.creator_config,self.target_jdk,self.issuer_preflight)
            ready=self.issuer_preflight.prepare()
            issuer_local=copy.deepcopy((self.issuer_preflight.journal.state['steps'],self.issuer_preflight.record))
            self._creator_guard(token)
            space_creator_runtime_profile_config.verify_completed(self.creator_config,token)
            config_local=copy.deepcopy((self.creator_config.journal.state['steps'],self.creator_config.record))
            space_creator_runtime_profile_jdk.verify_completed(self.target_jdk,token)
            jdk_local=copy.deepcopy((self.target_jdk.journal.state['steps'],self.target_jdk.record))
            self._creator_guard(token)
            self._creator_final(token,phases,(config_local,jdk_local,issuer_local),authority)
            self._creator_refs=phases
            return ready
        except BaseException:
            if self.issuer_preflight is not None:
                if self.issuer_preflight.pending:self.issuer_preflight._uncertain()
                try:self.issuer_preflight.close()
                except BaseException:pass
            raise SupervisorError('creator preparation incomplete; reconcile before retry') from None

    def finish_creator_inputs(self,token):
        """Finish the root handshake; further typed release gates still apply."""
        try:
            authority=self._creator_authority(); phases=self._creator_refs
            self._creator_guard(token)
            if any(x is None for x in (self.creator_config,self.target_jdk,self.issuer_preflight)):raise ValueError()
            initial=tuple(copy.deepcopy((p.journal.state['steps'],p.record)) for p in phases)
            if self._creator_refs is not phases:raise ValueError()
            self._creator_final(token,phases,initial,authority)
            receipt=self.issuer_preflight.finish()
            space_creator_runtime_profile_config.verify_completed(self.creator_config,token)
            config_local=copy.deepcopy((self.creator_config.journal.state['steps'],self.creator_config.record))
            space_creator_runtime_profile_jdk.verify_completed(self.target_jdk,token)
            jdk_local=copy.deepcopy((self.target_jdk.journal.state['steps'],self.target_jdk.record))
            if space_p3_issuer_preflight.verify_current(self.issuer_preflight,token)!=receipt:raise ValueError()
            issuer_local=copy.deepcopy((self.issuer_preflight.journal.state['steps'],self.issuer_preflight.record))
            self._creator_guard(token)
            if self._creator_refs is not phases:raise ValueError()
            self._creator_final(token,phases,(config_local,jdk_local,issuer_local),authority)
            return {'operation_id':self.context['operation_id'],'scope':'creator-inputs-only'}
        except BaseException:
            if self.issuer_preflight is not None and self.issuer_preflight.pending:
                self.issuer_preflight._uncertain()
                try:self.issuer_preflight.close()
                except BaseException:pass
            raise SupervisorError('creator custody incomplete; reconcile before retry') from None
