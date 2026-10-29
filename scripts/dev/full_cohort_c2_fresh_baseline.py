"""Source-qualified fresh C2 baseline; historical C1 is not current identity."""
import copy
import hashlib
import json
import re
import secrets
import time

import canonical_application_package as canonical
import frontend_runtime_executor
import full_cohort_affected_plan
import full_cohort_image_only_baseline as image_only
import full_cohort_inventory_phase
import full_cohort_namespace_census
import full_cohort_package_phase
import full_cohort_predecessor_phase as predecessor_module
import full_cohort_config_custody as config_custody
import full_cohort_schema_metadata as metadata
import media_operation_guard
import media_operation_journal
import resource_fencing
import space_native_tool_identity_custody as tool_custody
import space_native_tool_identity as tool_identity

STEP = 'full-cohort-c2-fresh-baseline'
MARKER = 'full_cohort_c2_fresh_baseline'


class C2BaselineError(RuntimeError): pass


def _observation_floors(context, card, actual, history, predecessor):
    """Pure comparison only; callers must first obtain all inputs under authority."""
    try:
        floors = set(image_only.history_floors(history))
        tip = history['events'][-1]['after']
        fields = ('code','environment','state','revision','generation','owner','claim',
                  'window','operation','phase','candidate','observed')
        if (history.get('revision') != card.get('revision') or
                any(tip.get(key) != card.get(key) for key in fields)):
            raise ValueError()
        observed = card['observed']
        if (observed.get('state') != 'known' or observed.get('actor') != context['actor'] or
                observed.get('claim') != context['claim'] or observed.get('window') != context['window'] or
                type(card.get('operation')) is not int or observed.get('operation') != card['operation']-1 or
                not isinstance(actual.get('images'),dict) or not actual['images'] or
                observed.get('images') != actual['images']):
            raise ValueError()
        deployments=[event for event in history['events'] if event.get('kind')=='deployment']
        if not deployments: raise ValueError()
        latest=deployments[-1]['after']['observed']['images']
        app_keys={'voice-'+name+'/'+container for name,container in metadata.CONTAINERS.items()}
        if (not isinstance(latest,dict) or set(latest)!=app_keys or
                any(actual['images'].get(key)!=value for key,value in latest.items())):
            raise ValueError()
        previous = predecessor['previous_commit']
        if not isinstance(previous,str) or not re.fullmatch(r'[a-f0-9]{40}',previous) or previous not in floors:
            raise ValueError()
        if 'commit' in observed:
            commit = observed['commit']
            if not isinstance(commit,str) or not re.fullmatch(r'[a-f0-9]{40}',commit): raise ValueError()
            floors.add(commit)
        return sorted(floors)
    except (KeyError, TypeError, ValueError, AttributeError, image_only.BaselineError):
        raise C2BaselineError('C2 current observation or historical floors unavailable') from None


def _join_current(namespace_uid, actual, inventory, catalog):
    """Cross-check fresh independent observers; no historical DB IDs enter."""
    try:
        projection = inventory['projection']
        if (actual['namespace_uid'] != namespace_uid or catalog['namespace_uid'] != namespace_uid or
                set(projection) != set(metadata.CONTAINERS)):
            raise ValueError()
        for name, item in projection.items():
            key = 'Deployment/voice-' + name
            if (item['uid'] != actual['workload_uids'][key] or
                    item['spec_sha256'] != actual['workload_spec_sha256'][key]): raise ValueError()
        engine, start = catalog['engine'], catalog['server_start']
        if engine['namespace_uid'] != namespace_uid or not isinstance(start,str) or not start: raise ValueError()
        all_databases, databases = catalog['all_databases'], catalog['databases']
        if (not isinstance(all_databases,dict) or not all_databases or not isinstance(databases,dict) or
                not databases or any(not isinstance(name,str) or not name or type(oid) is not int or oid < 1
                                     for name,oid in all_databases.items()) or
                len(set(all_databases.values())) != len(all_databases) or
                any(all_databases.get(name) != oid for name,oid in databases.items())):
            raise ValueError()
        workloads=inventory['database_observations']['workloads']
        if not isinstance(workloads,dict) or set(workloads)!=set(metadata.CONTAINERS): raise ValueError()
        observations=[]
        for name,entries in workloads.items():
            if not isinstance(entries,list) or not entries: raise ValueError()
            for item in entries:
                if not isinstance(item,dict) or item.get('status')!='observed': raise ValueError()
                result=item['result']; binding=result['binding']
                expected='resolved' if name in metadata.OWNERS or name=='gateway' else 'not_observed'
                if (binding.get('workload')!=name or binding.get('state')!=expected or
                        (expected=='not_observed' and binding.get('database') is not None)):
                    raise ValueError()
                if expected=='resolved':
                    primary=result['metadata']; database=binding['database']
                    if (database!=metadata.OWNERS.get(name,metadata.OPTIONAL_DATABASES.get(name)) or
                            primary['database']!=database or database not in databases or
                            primary['session']['database_oid']!=databases[database]): raise ValueError()
                    observations.append(primary)
                    for entry in result.get('secondary',[]):
                        secondary=entry['metadata']; secondary_db=secondary['database']
                        if (entry.get('owner_consistency')!='matched' or secondary_db not in databases or
                                secondary['session']['database_oid']!=databases[secondary_db]): raise ValueError()
                        observations.append(secondary)
        if not observations or any(item['engine'] != engine or item['session']['server_start'] != start
                                   for item in observations): raise ValueError()
    except (KeyError, TypeError, ValueError, AttributeError):
        raise C2BaselineError('C2 fresh inventory and database catalog differ') from None


def _digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode()).hexdigest()


class C2FreshBaseline:
    """Bind current C2 observers to confirmed admission and historical C1."""
    def __init__(self, supervisor, journal, fence, client):
        import full_cohort_supervisor
        if (type(supervisor) is not full_cohort_supervisor.Supervisor or
                type(journal) is not media_operation_journal.Journal or
                type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
                type(fence.check) is not resource_fencing.ResourceCheck or
                type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                fence.ledger.client is not client or fence.check is not supervisor.fence.check or
                fence.context != supervisor.context or not re.fullmatch(r'media-c2-[a-f0-9]{32}',
                                                                       fence.context.get('operation_id','')) or
                journal.intent.get('resource_context') != supervisor.context or
                journal.intent.get('namespace_uid') != supervisor.namespace_uid or
                type(supervisor.package) is not full_cohort_package_phase.PackagePhase or
                type(supervisor.inventory) is not full_cohort_inventory_phase.InventoryPhase or
                type(supervisor.catalog) is not full_cohort_affected_plan.FullCatalogPhase or
                type(supervisor.census) is not full_cohort_namespace_census.NamespaceCensus or
                supervisor.inventory.package is not supervisor.package or
                supervisor.catalog.inventory is not supervisor.inventory or
                supervisor.census.package is not supervisor.package or
                supervisor.census.inventory is not supervisor.inventory):
            raise C2BaselineError('C2 phase identity invalid')
        self.supervisor,self.journal,self.fence,self.client = supervisor,journal,fence,client
        self.package,self.inventory,self.catalog,self.census = supervisor.package,supervisor.inventory,supervisor.catalog,supervisor.census
        self.context,self.intent = copy.deepcopy(fence.context),copy.deepcopy(journal.intent)
        self.lock,self.namespace_uid = copy.deepcopy(supervisor.lock),supervisor.namespace_uid
        self.root_refs=(supervisor.journal,supervisor.fence,supervisor.client)
        self.own_refs=(journal,fence,client)
        self.phase_refs=tuple((p.journal,p.fence,p.client) for p in (self.package,self.inventory,self.catalog,self.census))
        self.links=tuple((f.ledger,f.check) for _,f,_ in (self.root_refs,self.own_refs,*self.phase_refs))
        if len({id(j) for j,_,_ in (self.root_refs,self.own_refs,*self.phase_refs)}) != 6:
            raise C2BaselineError('C2 journals must have independent custody')
        self.custody=tuple(tool_identity.SpaceNativeToolPhase._custody(j)
                           for j,_,_ in (self.root_refs,self.own_refs,*self.phase_refs))
        self.poisoned=False

    def _local(self):
        s=self.supervisor
        if (self.poisoned or (s.journal,s.fence,s.client)!=self.root_refs or
                (self.journal,self.fence,self.client)!=self.own_refs or
                (s.package,s.inventory,s.catalog,s.census)!=(self.package,self.inventory,self.catalog,self.census) or
                tuple((p.journal,p.fence,p.client) for p in (self.package,self.inventory,self.catalog,self.census))!=self.phase_refs or
                s.context!=self.context or s.lock!=self.lock or s.namespace_uid!=self.namespace_uid or
                self.journal.intent!=self.intent or self.inventory.package is not self.package or
                self.catalog.inventory is not self.inventory or self.census.package is not self.package or
                self.census.inventory is not self.inventory or any(
                    p.lock!=self.lock or p.namespace_uid!=self.namespace_uid or
                    p.journal.intent!=p.intent for p in (self.package,self.inventory,self.catalog,self.census))):
            raise C2BaselineError('C2 phase identity changed')
        for (journal,fence,client),links in zip((self.root_refs,self.own_refs,*self.phase_refs),self.links):
            if (type(journal) is not media_operation_journal.Journal or journal.poisoned or
                    journal.dirfd is None or journal.lockfd is None or journal.state.get('intent')!=journal.intent or
                    journal.intent.get('resource_context')!=self.context or
                    journal.intent.get('namespace_uid')!=self.namespace_uid or
                    type(fence) is not resource_fencing.Fence or (fence.ledger,fence.check)!=links or
                    type(fence.ledger) is not resource_fencing.KubeLedger or
                    type(fence.check) is not resource_fencing.ResourceCheck or fence.context!=self.context or
                    fence.check is not s.fence.check or fence.check.context!=self.context or
                    fence.check.endpoint!=predecessor_module.TT_ENDPOINT or fence.ledger.client is not client or
                    type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal):
                raise C2BaselineError('C2 phase custody changed')
        if tuple(tool_identity.SpaceNativeToolPhase._custody(j)
                 for j,_,_ in (self.root_refs,self.own_refs,*self.phase_refs))!=self.custody:
            raise C2BaselineError('C2 journal physical custody changed')

    def _guard(self):
        self._local()
        # This fixed Supervisor method is supplied by the reviewed C2 begin
        # implementation. On older source it is absent and C2 remains closed.
        if not callable(getattr(type(self.supervisor),'_confirmed_admission',None)):
            raise C2BaselineError('confirmed C2 admission unavailable')
        self.supervisor._guard()
        type(self.supervisor)._confirmed_admission(self.supervisor)
        predecessor_module._no_tt_proxy()
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        row,revision=self.fence.ledger.read()
        for _,fence,_ in (self.root_refs,*self.phase_refs):
            if fence.ledger.read()!=(row,revision): raise C2BaselineError('C2 physical ledger differs')
        self._local()
        return row,revision

    def _snapshot(self,token):
        row,revision=self._guard()
        journals=[journal for journal,_,_ in (self.root_refs,self.own_refs,*self.phase_refs)]
        steps=[copy.deepcopy(journal.state['steps']) for journal in journals]
        old=predecessor_module.load(self.supervisor.fence,self.supervisor.client,self.lock,
                                    self.namespace_uid,self.supervisor.journal)
        config=config_custody.load_predecessor(self.supervisor.fence,self.supervisor.client,self.lock,
                                               self.namespace_uid,self.supervisor.journal)
        tool=tool_custody.reuse(self.supervisor.fence,self.supervisor.client,self.lock,self.namespace_uid,
                                self.supervisor.journal,self.package,token)
        package=self.package.verify_current(token)
        inventory=self.inventory.verify_current(token)
        catalog=self.catalog.verify_current(token)
        census=self.census.verify_current(token)
        actual=census['baseline']
        history=image_only._read_history(self.fence.check)
        card=self.fence.authority()
        floors=_observation_floors(self.context,card,actual,history,old)
        _join_current(self.namespace_uid,actual,inventory,catalog)
        if (self.package.verify_current(token)!=package or self.inventory.verify_current(token)!=inventory or
                self.catalog.verify_current(token)!=catalog or
                self.census.verify_current(token)!=census or
                predecessor_module.load(self.supervisor.fence,self.supervisor.client,self.lock,
                                        self.namespace_uid,self.supervisor.journal)!=old or
                config_custody.load_predecessor(self.supervisor.fence,self.supervisor.client,self.lock,
                                                self.namespace_uid,self.supervisor.journal)!=config or
                tool_custody.reuse(self.supervisor.fence,self.supervisor.client,self.lock,self.namespace_uid,
                                   self.supervisor.journal,self.package,token)!=tool or self._guard()!=(row,revision) or
                any(journal.state['steps']!=saved for journal,saved in zip(journals,steps))):
            raise C2BaselineError('C2 observations changed')
        return {'schema':'voice-full-cohort-c2-fresh-baseline-v1',
                'operation_id':self.context['operation_id'],'namespace_uid':self.namespace_uid,
                'candidate':self.context['commit'],'floors':floors,'history_sha256':_digest(history),
                'predecessor_sha256':_digest(old),'config_sha256':_digest(config),'tool_sha256':_digest(tool),
                'package_sha256':_digest(package),'inventory_sha256':_digest(inventory),'census_sha256':_digest(census),
                'actual':actual,'catalog':catalog,
                'limits':['nonowner_database_routes_unqualified','startup_unqualified']}, row, revision

    def run(self,token):
        proof,row,revision=self._snapshot(token)
        if MARKER in row or STEP in self.journal.state['steps']: raise C2BaselineError('C2 baseline already attempted')
        headers=canonical.source._headers(token); deadline=time.monotonic()+30
        canonical.source._head(headers,self.context['commit'],deadline)
        for floor in proof['floors']:
            candidate=self.context['commit']
            comparison=canonical.source._get(canonical.source.API+'/compare/'+floor+'...'+candidate,headers,deadline)
            if (comparison.get('base_commit',{}).get('sha')!=floor or
                    comparison.get('merge_base_commit',{}).get('sha')!=floor or
                    type(comparison.get('ahead_by')) is not int or
                    (floor==candidate and (comparison.get('status')!='identical' or comparison['ahead_by']!=0)) or
                    (floor!=candidate and (comparison.get('status')!='ahead' or comparison['ahead_by']<=0))):
                raise C2BaselineError('C2 candidate is not a source descendant')
        again,current,actual_revision=self._snapshot(token)
        if again!=proof or current!=row or actual_revision!=revision: raise C2BaselineError('C2 baseline changed')
        pending={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.context['operation_id'],
                 'proof_sha256':_digest(proof)}
        done=pending | {'status':'done'}
        try:
            self.journal.state['steps'][STEP]={'marker':copy.deepcopy(pending),'proof':copy.deepcopy(proof)}
            self.journal.save()
            if self._guard()!=(row,revision): raise C2BaselineError('C2 reservation changed')
            row[MARKER]=copy.deepcopy(pending); self.fence.ledger.write(revision,row)
            current,rev=self._guard()
            if current.get(MARKER)!=pending or self.journal.state['steps'][STEP]!={'marker':pending,'proof':proof}:
                raise C2BaselineError('C2 pending receipt changed')
            current[MARKER]=copy.deepcopy(done); self.fence.ledger.write(rev,current)
            current,_=self._guard()
            if current.get(MARKER)!=done or self.journal.state['steps'][STEP]!={'marker':pending,'proof':proof}:
                raise C2BaselineError('C2 completion receipt changed')
            self.journal.state['steps'][STEP]={'marker':copy.deepcopy(done),'proof':copy.deepcopy(proof)}
            self.journal.save()
            self.verify_current(token)
            return copy.deepcopy(proof)
        except BaseException:
            self.poisoned=True
            try: self.fence.uncertain('C2 fresh baseline outcome uncertain')
            except BaseException: pass
            raise

    def verify_current(self,token):
        current,row,_=self._snapshot(token)
        entry=copy.deepcopy(self.journal.state['steps'].get(STEP))
        if (not isinstance(entry,dict) or set(entry)!={'marker','proof'} or
                not isinstance(entry['marker'],dict) or set(entry['marker'])!={'status','nonce','operation_id','proof_sha256'} or
                entry['marker'].get('status')!='done' or entry['marker'].get('operation_id')!=self.context['operation_id'] or
                not isinstance(entry['marker'].get('nonce'),str) or
                not re.fullmatch(r'[a-f0-9]{32}',entry['marker']['nonce']) or row.get(MARKER)!=entry['marker'] or
                entry['marker'].get('proof_sha256')!=_digest(entry['proof']) or entry['proof']!=current or
                self.journal.state['steps'].get(STEP)!=entry):
            raise C2BaselineError('C2 baseline proof changed')
        self._local()
        return copy.deepcopy(entry['proof'])
