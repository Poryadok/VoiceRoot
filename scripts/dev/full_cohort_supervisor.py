"""Source-owned beginning of a fenced cohort operation.

The operation lock and TT deploy claim must already be held.  This object
keeps every private journal open for the caller; it never releases a lock or
declares a rollout complete.  Later phases consume the verified objects, not
an externally supplied description of package or database state.
"""
import copy
import re

import frontend_runtime_executor
import full_cohort_affected_plan
import full_cohort_inventory_phase
import full_cohort_package_phase
import media_backup_upload_runner
import media_operation_guard
import media_operation_journal
import realtime_absent_phase
import resource_fencing
import story_media_backup


class SupervisorError(RuntimeError):
    pass


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
        self.package = self.inventory = self.catalog = self.realtime_absent = None
        self._guard()

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

    def _open(self, name, extra):
        if name not in ('package-journal', 'inventory-journal', 'catalog-journal', 'realtime-absence-journal'):
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

    def prepare(self, token, actor, run_id, run_attempt, nodes):
        """Capture the canonical package, live inventory and full DB catalog."""
        if any(x is not None for x in (self.package, self.inventory, self.catalog)):
            raise SupervisorError('prepare requires reconciliation')
        journal, fence, client = self._open('package-journal', {'full_cohort_package':
            {'run_id': run_id, 'run_attempt': run_attempt, 'nodes': copy.deepcopy(nodes)}})
        self.package = full_cohort_package_phase.PackagePhase(journal, fence, client, self.lock, self.namespace_uid)
        self.package.run(token, actor)
        journal, fence, client = self._open('inventory-journal', {})
        self.inventory = full_cohort_inventory_phase.InventoryPhase(journal, fence, client, self.lock,
                                                                     self.namespace_uid, self.package)
        self.inventory.run(token)
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
