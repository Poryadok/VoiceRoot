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
        self._guard()

    def _image_admission(self):
        if not self.context['operation_id'].startswith('media-c1-'):return
        step=copy.deepcopy(self.journal.state['steps'].get('image-only-admission'))
        capture=copy.deepcopy(self.journal.state['steps'].get(full_cohort_image_only_baseline.STEP)); row,revision=self.fence.ledger.read()
        if (not isinstance(step,dict) or set(step)!={'status','descriptor','old_revision','new_revision'} or
                step.get('status')!='done' or not isinstance(step.get('old_revision'),list) or
                not isinstance(step.get('new_revision'),list) or len(step['old_revision'])!=2 or len(step['new_revision'])!=2 or
                any(not isinstance(v,str) or not v for v in step['old_revision']+step['new_revision']) or
                step['old_revision'][0]!=step['new_revision'][0] or not isinstance(revision,tuple) or len(revision)!=2 or
                revision[0]!=step['new_revision'][0] or not self.fence.owned(row) or row.get('status')!='active' or
                row.get('image_only_baseline')!=step['descriptor'] or not isinstance(capture,dict) or
                capture.get('descriptor')!=step['descriptor'] or self.journal.dirfd is None or self.journal.lockfd is None or
                self.journal.state['steps'].get('image-only-admission')!=step or
                self.journal.state['steps'].get(full_cohort_image_only_baseline.STEP)!=capture):
            raise SupervisorError('image-only admission is not confirmed')

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
        self._image_admission()

    def _open(self, name, extra):
        go_names={'go-migration-'+db+'-journal' for db in full_cohort_go_migration_plan.DATABASES}
        if name not in ('package-journal', 'inventory-journal', 'catalog-journal', 'realtime-absence-journal', 'namespace-census-journal') and name not in go_names:
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
