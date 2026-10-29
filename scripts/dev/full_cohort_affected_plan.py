"""Derive changed database writers from authenticated current/P1 observations.

This is a bounded first stage: unresolved routes and configuration-only
transitions are refused until their source-owned planners are available.
It does not itself establish a writer cut or execute a backup.
"""
import copy
import hashlib
import json
import re
import secrets

import frontend_runtime_executor
import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import full_cohort_schema_metadata as metadata
import media_operation_guard
import writer_cut
import writer_cut_pg_probe as pg


class AffectedError(RuntimeError): pass

PLANNED_CONFIG_UNKNOWN=frozenset(('chat','space','user','notification','realtime','gateway'))
POTENTIAL_NONOWNERS=frozenset(('analytics','web','admin','developer-portal'))


def _full_catalog(phase,expected_engine,server_start):
    """Observe all databases; exclude unavailable DBs only with zero activity."""
    snapshot=metadata.kube_engine_snapshot(metadata.linux_private_bytes,phase._guard)
    sql=("SELECT json_build_object('server_start',pg_catalog.pg_postmaster_start_time(),"
         "'can_read',pg_catalog.pg_has_role(current_user,'pg_read_all_stats','MEMBER'),"
         "'databases',COALESCE(json_agg(json_build_object('name',d.datname,'oid',d.oid::bigint,"
         "'allowconn',d.datallowconn,'template',d.datistemplate,"
         "'encoding',pg_catalog.pg_encoding_to_char(d.encoding),'collate',d.datcollate,'ctype',d.datctype,"
         "'locale_provider',d.datlocprovider,'icu_locale',d.daticulocale,'connection_limit',d.datconnlimit,"
         "'owner',pg_catalog.pg_get_userbyid(d.datdba),'acl',d.datacl::text,"
         "'connect',pg_catalog.has_database_privilege(current_user,d.oid,'CONNECT'),"
         "'sessions',(SELECT count(*) FROM pg_catalog.pg_stat_activity a WHERE a.datid=d.oid AND a.pid<>pg_catalog.pg_backend_pid()),"
         "'prepared',(SELECT count(*) FROM pg_catalog.pg_prepared_xacts x WHERE x.database=d.datname)) ORDER BY d.datname),'[]'::json)) "
         "FROM pg_catalog.pg_database d")
    args=['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
          '--request-timeout=15s','exec','voice-postgres-0','-c','postgres','--','sh','-ec',
          metadata.PSQL_SCRIPT,'--','postgres',sql]
    def read():
        phase._guard()
        if snapshot()!=expected_engine: raise ValueError()
        value=json.loads(metadata.linux_psql(args)); phase._guard()
        if snapshot()!=expected_engine or not isinstance(value,dict) or set(value)!={'server_start','can_read','databases'} or \
                value['server_start']!=server_start or value['can_read'] is not True or not isinstance(value['databases'],list) or not 1<=len(value['databases'])<=128:
            raise ValueError()
        result={}; backup={}; excluded={}; properties={}
        for item in value['databases']:
            if (not isinstance(item,dict) or set(item)!={'name','oid','connect','allowconn','template','sessions','prepared',
                    'encoding','collate','ctype','locale_provider','icu_locale','connection_limit','owner','acl'} or
                    not isinstance(item['name'],str) or not metadata.SAFE.fullmatch(item['name']) or item['name']=='roles' or
                    item['name'] in result or type(item['oid']) is not int or item['oid']<=0 or
                    any(type(item[k]) is not bool for k in ('connect','allowconn','template')) or
                    any(not isinstance(item[k],str) or not item[k] for k in ('encoding','collate','ctype','locale_provider','owner')) or
                    item['icu_locale'] is not None and not isinstance(item['icu_locale'],str) or
                    item['acl'] is not None and not isinstance(item['acl'],str) or type(item['connection_limit']) is not int or
                    any(type(item[k]) is not int or item[k]<0 for k in ('sessions','prepared'))): raise ValueError()
            result[item['name']]=item['oid']
            properties[item['name']]={k:item[k] for k in ('template','allowconn','encoding','collate','ctype',
                'locale_provider','icu_locale','connection_limit','owner','acl')}
            if item['allowconn']:
                if item['connect'] is not True: raise ValueError()
                backup[item['name']]=item['oid']
            else:
                if item['sessions'] or item['prepared']: raise ValueError()
                excluded[item['name']]={'oid':item['oid'],'template':item['template'],'reason':'connections_disabled_zero_activity'}
        if len(set(result.values()))!=len(result): raise ValueError()
        return {'all_databases':result,'databases':backup,'excluded':excluded,'properties':properties}
    first=read()
    if read()!=first: raise ValueError()
    return first


class FullCatalogPhase:
    """Bind a fresh complete logical PostgreSQL database inventory to the fence."""
    def __init__(self,journal,fence,client,lock,namespace_uid,inventory_phase):
        self.journal,self.fence,self.client=journal,fence,client
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.inventory=inventory_phase; self.intent=copy.deepcopy(journal.intent); self.poisoned=False
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                getattr(fence.ledger,'client',None) is not client or type(inventory_phase) is not inventory.InventoryPhase or
                self.intent.get('resource_context')!=fence.context or self.intent.get('namespace_uid')!=namespace_uid or
                inventory_phase.fence.context!=fence.context or inventory_phase.lock!=self.lock or
                inventory_phase.namespace_uid!=namespace_uid): raise AffectedError('catalog identity invalid')

    def _guard(self):
        if self.poisoned or self.journal.poisoned or self.journal.intent!=self.intent or \
                self.journal.state.get('intent')!=self.intent: raise AffectedError('catalog identity changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)

    def _ledger(self):
        row,rev=self.fence.ledger.read()
        if self.inventory.fence.ledger.read()!=(row,rev) or not self.fence.owned(row) or row.get('status')!='active':
            raise AffectedError('catalog ledger changed')
        return row,rev

    def run(self,token):
        pending=False
        try:
            self._guard(); observed=self.inventory.verify_current(token); row,rev=self._ledger()
            if 'full-cohort-db-catalog' in self.journal.state['steps'] or 'full_cohort_db_catalog' in row: raise ValueError()
            references=[e['result']['metadata'] for values in observed['database_observations']['workloads'].values()
                        for e in values if e.get('status')=='observed' and isinstance(e.get('result',{}).get('metadata'),dict)]
            if not references: raise ValueError()
            engine=references[0]['engine']; start=references[0]['session']['server_start']
            if engine.get('namespace_uid')!=self.namespace_uid or any(v['engine']!=engine or v['session']['server_start']!=start for v in references): raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id']}
            pending=True; self.journal.state['steps']['full-cohort-db-catalog']=copy.deepcopy(marker); self.journal.save()
            self._guard(); current,actual=self._ledger()
            if current!=row or actual!=rev or self.journal.state['steps']['full-cohort-db-catalog']!=marker: raise ValueError()
            current['full_cohort_db_catalog']=copy.deepcopy(marker); current['step']='full-cohort-db-catalog-pending'
            self.fence.ledger.write(actual,current); self._guard()
            catalog=_full_catalog(self,engine,start)
            if self.inventory.verify_receipt(token)!=observed: raise ValueError()
            receipt={'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                     'namespace_uid':self.namespace_uid,'engine':engine,'server_start':start,**catalog,
                     'scope':'all-databases-connectable-backup'}
            current,actual=self._ledger()
            if current.get('full_cohort_db_catalog')!=marker or self.journal.state['steps']['full-cohort-db-catalog']!=marker: raise ValueError()
            done=marker|{'status':'done','receipt_sha256':hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()}
            local=done|{'receipt':copy.deepcopy(receipt)}; self.journal.state['steps']['full-cohort-db-catalog']=local; self.journal.save()
            current['full_cohort_db_catalog']=copy.deepcopy(done); current['step']='full-cohort-db-catalog-done'
            self.fence.ledger.write(actual,current); self._guard()
            if self._ledger()[0].get('full_cohort_db_catalog')!=done or self.journal.state['steps']['full-cohort-db-catalog']!=local: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                self.poisoned=True
                try: self.fence.uncertain('database catalog outcome uncertain')
                except BaseException: pass
            raise AffectedError('database catalog unavailable') from None

    def verify_receipt(self,token):
        try:
            self._guard(); local=copy.deepcopy(self.journal.state['steps']['full-cohort-db-catalog']); receipt=local['receipt']
            marker={k:v for k,v in local.items() if k!='receipt'}; row,_=self._ledger()
            if (set(marker)!={'status','nonce','operation_id','receipt_sha256'} or marker['status']!='done' or
                    marker['operation_id']!=self.fence.context['operation_id'] or row.get('full_cohort_db_catalog')!=marker or
                    marker['receipt_sha256']!=hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest() or
                    receipt['operation_id']!=self.fence.context['operation_id'] or receipt['source_sha']!=self.fence.context['commit'] or
                    receipt['namespace_uid']!=self.namespace_uid or receipt['scope']!='all-databases-connectable-backup'):
                raise ValueError()
            self.inventory.verify_receipt(token); self._guard(); final,_=self._ledger()
            if final.get('full_cohort_db_catalog')!=marker or self.journal.state['steps']['full-cohort-db-catalog']!=local: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            raise AffectedError('database catalog receipt unavailable') from None

    def verify_current(self,token):
        receipt=self.verify_receipt(token)
        if _full_catalog(self,receipt['engine'],receipt['server_start'])!={k:receipt[k] for k in ('all_databases','databases','excluded','properties')} or self.verify_receipt(token)!=receipt:
            raise AffectedError('database catalog changed')
        return receipt


class CatalogScopedProbe:
    """Keep the complete catalog bound around every pre-migration zero sample."""
    def __init__(self,catalog_phase,token):
        if type(catalog_phase) is not FullCatalogPhase or not isinstance(token,str) or not token:
            raise AffectedError('catalog probe identity invalid')
        self.catalog,self.token=catalog_phase,token
        self._bound_catalog,self._bound_token=catalog_phase,token

    def __call__(self,scope):
        try:
            if self.catalog is not self._bound_catalog or self.token!=self._bound_token: raise ValueError()
            before=self.catalog.verify_current(self.token)
            expected={'engine':before['engine'],'server_start':before['server_start'],
                      'databases':sorted(before['all_databases']), 'database_oids':before['all_databases']}
            if scope!=expected: raise ValueError()
            result=pg.inspect(scope,self.catalog._guard)
            if (self.catalog.verify_current(self.token)!=before or self.catalog is not self._bound_catalog or
                    self.token!=self._bound_token): raise ValueError()
            return result
        except BaseException:
            raise AffectedError('catalog-scoped session observation unavailable') from None


def _verify_catalog(probe,cut,scope):
    if (type(probe) is not CatalogScopedProbe or type(cut) is not writer_cut.WriterCut or cut.probe is not probe or
            probe.catalog is not probe._bound_catalog or probe.token!=probe._bound_token):
        raise AffectedError('catalog probe identity changed')
    before=probe.catalog.verify_receipt(probe.token)
    current=_full_catalog(probe.catalog,before['engine'],before['server_start'])
    expected=copy.deepcopy(before['all_databases'])
    properties=copy.deepcopy(before['properties']); excluded=copy.deepcopy(before['excluded'])
    if getattr(cut,'space_fresh',None) is not None:
        import writer_cut_space
        import space_fresh_install_phase
        binding=writer_cut_space.verify_binding(cut,require_finished=True)
        proof=space_fresh_install_phase.verify_historical(binding['phase'],binding['token'])
        if (proof['operation_id']!=cut.fence.context['operation_id'] or proof['source_sha']!=cut.fence.context['commit'] or
                proof['namespace_uid']!=cut.namespace_uid or proof['receipt_sha256']!=binding['receipt_sha256'] or
                proof['original_catalog_sha256']!=writer_cut.digest(before) or expected.get('space_db')!=proof['old_oid']):
            raise AffectedError('Space catalog proof changed')
        post=proof['catalog']
        if (not isinstance(post,dict) or set(post)!={'databases','all_databases','excluded','properties'} or
                post['all_databases'].get('space_db')!=proof['new_oid']):raise AffectedError('Space catalog proof invalid')
        expected=copy.deepcopy(post['all_databases']);properties=copy.deepcopy(post['properties']);excluded=copy.deepcopy(post['excluded'])
        if writer_cut_space.verify_binding(cut,require_finished=True)!=binding:raise AffectedError('Space binding changed')
    if 'realtime_db' not in expected and 'realtime_db' in scope['database_oids']:
        expected['realtime_db']=scope['database_oids']['realtime_db']
    if (scope['engine']!=before['engine'] or scope['server_start']!=before['server_start'] or
            scope['database_oids']!=expected or scope['databases']!=sorted(expected) or
            current['all_databases']!=expected or current['excluded']!=excluded or
            any(current['properties'].get(name)!=value for name,value in properties.items()) or
            ('realtime_db' not in before['all_databases'] and 'realtime_db' in expected and
             current['databases'].get('realtime_db')!=expected['realtime_db']) or
            probe.catalog.verify_receipt(probe.token)!=before):
        raise AffectedError('effective catalog changed')
    return copy.deepcopy(current)


def verify_effective_catalog(probe,cut):
    """Recheck the original catalog plus only a source-bound Realtime successor."""
    scope=cut.current_effective_session_scope(); result=_verify_catalog(probe,cut,scope)
    if cut.current_effective_session_scope()!=scope: raise AffectedError('effective scope changed')
    return result


def verify_provisioned_catalog(probe,cut):
    """Verify the typed provision successor without sampling sessions."""
    scope=cut.current_provisioned_session_scope()
    current=_verify_catalog(probe,cut,scope)
    if cut.current_provisioned_session_scope()!=scope:
        raise AffectedError('provisioned scope changed')
    return current


def inspect_provisioned(probe,cut):
    """Sample global zero against the protected, freshly provisioned scope."""
    try:
        scope=cut.current_provisioned_session_scope()
        before=_verify_catalog(probe,cut,scope)
        result=pg.inspect(scope,probe.catalog._guard)
        if (not isinstance(result,dict) or set(result)!={'scope','active_sessions','active_transactions'} or
                result['scope']!=scope or type(result['active_sessions']) is not int or result['active_sessions']!=0 or
                type(result['active_transactions']) is not int or result['active_transactions']!=0 or
                cut.current_provisioned_session_scope()!=scope or _verify_catalog(probe,cut,scope)!=before or
                cut.current_provisioned_session_scope()!=scope):
            raise ValueError()
        return copy.deepcopy(result)
    except BaseException:
        raise AffectedError('provisioned scope observation unavailable') from None


def verify_space_catalog(probe,cut):
    """Verify the completed Space successor without asserting zero sessions."""
    try:
        scope=cut.current_base_session_scope();result=_verify_catalog(probe,cut,scope)
        if cut.current_base_session_scope()!=scope:raise ValueError()
        return copy.deepcopy(result)
    except BaseException:raise AffectedError('Space catalog unavailable') from None


def inspect_space_completed(probe,cut):
    """Sample zero against only the protected completed Space replacement."""
    try:
        scope=cut.current_base_session_scope();before=_verify_catalog(probe,cut,scope)
        result=pg.inspect(scope,probe.catalog._guard)
        if (not isinstance(result,dict) or set(result)!={'scope','active_sessions','active_transactions'} or
                result['scope']!=scope or type(result['active_sessions']) is not int or result['active_sessions']!=0 or
                type(result['active_transactions']) is not int or result['active_transactions']!=0 or
                cut.current_base_session_scope()!=scope or _verify_catalog(probe,cut,scope)!=before or
                cut.current_base_session_scope()!=scope):raise ValueError()
        return copy.deepcopy(result)
    except BaseException:raise AffectedError('Space scope observation unavailable') from None


def verify_migration_catalog(probe,cut):
    """Bind an active source-owned migration DB before effective-scope completion."""
    import writer_cut_migration
    binding=cut.migration
    if not isinstance(binding,dict) or binding.get('kind') not in ('auth','realtime','go'): raise AffectedError('migration unavailable')
    _,state=writer_cut_migration.normalized(cut,binding['kind'],binding['phase'],binding['root'])
    if state['execution']!=binding['plan']['execution'] or state['stable']!=binding['plan']['stable']:
        raise AffectedError('migration identity changed')
    scope=cut.current_base_session_scope(); execution=state['execution']
    db,oid=execution['database'],execution['database_oid']
    if db in scope['database_oids'] and scope['database_oids'][db]!=oid: raise AffectedError('migration database changed')
    if db not in scope['database_oids']:
        if binding['kind']!='realtime' or db!='realtime_db': raise AffectedError('unexpected migration database')
        scope['database_oids'][db]=oid; scope['databases']=sorted(scope['database_oids'])
    result=_verify_catalog(probe,cut,scope)
    _,final=writer_cut_migration.normalized(cut,binding['kind'],binding['phase'],binding['root'])
    if final!=state or cut.migration is not binding: raise AffectedError('migration changed')
    return {'scope':scope,'catalog':result}

def build(package_phase, inventory_phase, token, catalog_phase=None, realtime_absent=None):
    """Return only source-observed routes of actual image-changed DB writers."""
    try:
        if type(package_phase) is not package.PackagePhase or type(inventory_phase) is not inventory.InventoryPhase:
            raise ValueError()
        if (inventory_phase.package is not package_phase or inventory_phase.fence.context!=package_phase.fence.context or
                inventory_phase.lock!=package_phase.lock or inventory_phase.namespace_uid!=package_phase.namespace_uid): raise ValueError()
        observed=inventory_phase.verify_current(token); candidate=package_phase.verify_current(token)
        if (inventory_phase.fence.ledger.read()!=package_phase.fence.ledger.read() or
                observed['source_sha']!=candidate['source_sha'] or observed['package_sha256']!=candidate['package_sha256']): raise ValueError()
        projection=observed['projection']; workloads=observed['database_observations']['workloads']
        if set(projection)!=set(metadata.CONTAINERS) or set(workloads)!=set(metadata.CONTAINERS): raise ValueError()
        all_writers={}; potential={}; database_catalog={}; changed_names=set(); unchanged=[]; nonowners=[]
        for name,app in sorted(metadata.CONTAINERS.items()):
            pods=projection[name]['pods']; image_key='voice-'+name+'/'+app
            target=candidate['platform_digests'][image_key]
            if not isinstance(target,str) or not re.fullmatch(r'sha256:[a-f0-9]{64}',target): raise ValueError()
            ids=[]
            for pod in pods:
                found=[c for c in pod['containers'] if c.get('name')==app]
                if len(found)!=1 or not isinstance(found[0].get('image_id'),str): raise ValueError()
                match=re.search(r'(sha256:[a-f0-9]{64})$',found[0]['image_id'])
                if match is None: raise ValueError()
                ids.append(match.group(1))
            if not ids: raise ValueError()
            changed=any(value!=target for value in ids)
            # Until a protected per-workload config plan proves otherwise, the
            # four legacy-unclassified workloads conservatively affect scope.
            if name in POTENTIAL_NONOWNERS: changed=True
            # These fixed services have separately planned configuration changes.
            # Until a protected source-owned config plan exists, an equal image
            # cannot establish that the service is unchanged.
            if not changed and name in PLANNED_CONFIG_UNKNOWN and not (name=='realtime' and realtime_absent is not None):
                raise ValueError()
            if not changed:
                unchanged.append(name)
            else:
                changed_names.add('voice-'+name)
            entries=workloads[name]
            if not isinstance(entries,list) or len(entries)!=len(pods): raise ValueError()
            if name=='realtime' and realtime_absent is not None:
                import realtime_absent_phase
                if (type(realtime_absent) is not realtime_absent_phase.RealtimeAbsentPhase or
                        realtime_absent.inventory is not inventory_phase or realtime_absent.catalog is not catalog_phase or
                        any(e.get('status')!='hold' or e.get('reason')!='database_configuration_missing' for e in entries)):
                    raise ValueError()
                absent=realtime_absent.verify_current(token)
                if (absent['source_sha']!=candidate['source_sha'] or absent['deployment_uid']!=projection[name]['uid'] or
                        absent['generation']!=projection[name]['generation'] or
                        absent['spec_sha256']!=projection[name]['spec_sha256']): raise ValueError()
                changed_names.add('voice-realtime')
                potential['voice-realtime']={'uid':projection[name]['uid'],'generation':projection[name]['generation'],
                    'spec_sha256':projection[name]['spec_sha256'],'routes':[], 'reason':'qualified_create_only_realtime'}
                continue
            if name not in metadata.OWNERS and name!='gateway':
                if name not in POTENTIAL_NONOWNERS or any(e.get('status')!='observed' or
                        e.get('result',{}).get('binding',{}).get('state')!='not_observed' for e in entries):
                    raise ValueError()
                # Their legacy processes are unclassified, so stop all four
                # across the selected PostgreSQL scope; no non-writer claim.
                nonowners.append(name)
                potential['voice-'+name]={'uid':projection[name]['uid'],'generation':projection[name]['generation'],
                    'spec_sha256':projection[name]['spec_sha256'],'routes':[], 'reason':'unclassified_potential_writer'}
                continue
            if any(e.get('status')!='observed' for e in entries): raise ValueError()
            routes=[]
            for entry in entries:
                value=entry['result']; binding=value['binding']; data=value['metadata']
                all_routes=[(binding,data)]
                for secondary in value.get('secondary',[]):
                    if secondary.get('owner_consistency')!='matched': raise ValueError()
                    all_routes.append((secondary['binding'],secondary['metadata']))
                current=[]
                for bound,record in all_routes:
                    db,role=bound.get('database'),bound.get('role'); session=record.get('session',{})
                    if (bound.get('state')!='resolved' or db not in set(metadata.OWNERS.values())|{'gateway_db'} or
                            not isinstance(role,str) or not metadata.SAFE.fullmatch(role) or record.get('database')!=db or
                            type(session.get('database_oid')) is not int or session['database_oid']<=0 or
                            not isinstance(record.get('engine'),dict) or not isinstance(session.get('server_start'),str)):
                        raise ValueError()
                    source={'engine':record['engine'],'server_start':session['server_start'],'database_oid':session['database_oid']}
                    if db in database_catalog and database_catalog[db]!=source: raise ValueError()
                    database_catalog[db]=copy.deepcopy(source); current.append({'database':db,'role':role})
                routes.append(sorted(current,key=lambda x:(x['database'],x['role'])))
            if any(r!=routes[0] for r in routes): raise ValueError()
            all_writers['voice-'+name]={'uid':projection[name]['uid'],'generation':projection[name]['generation'],
                'spec_sha256':projection[name]['spec_sha256'],'routes':routes[0]}
        affected={route['database'] for name,entry in all_writers.items() if name in changed_names for route in entry['routes']}
        full_catalog=bool(changed_names.intersection(potential))
        if full_catalog:
            if not database_catalog or type(catalog_phase) is not FullCatalogPhase or catalog_phase.inventory is not inventory_phase:
                raise ValueError()
            reference=next(iter(database_catalog.values()))
            catalog_receipt=catalog_phase.verify_current(token)
            if catalog_receipt['engine']!=reference['engine'] or catalog_receipt['server_start']!=reference['server_start']:
                raise ValueError()
            catalog=catalog_receipt['databases']
            for db,source in database_catalog.items():
                if catalog.get(db)!=source['database_oid'] or source['engine']!=reference['engine'] or \
                        source['server_start']!=reference['server_start']: raise ValueError()
            for db,oid in catalog.items():
                database_catalog.setdefault(db,{'engine':copy.deepcopy(reference['engine']),
                    'server_start':reference['server_start'],'database_oid':oid})
            affected.update(catalog)
        databases={db:database_catalog[db] for db in sorted(affected)}
        writers={name:entry|{'reason':'candidate_image_changed' if name in changed_names else 'shares_affected_database'}
                 for name,entry in all_writers.items() if any(route['database'] in affected for route in entry['routes'])}
        writers.update(potential)
        if not writers or not databases: raise ValueError()
        engines={(json.dumps(v['engine'],sort_keys=True,separators=(',',':')),v['server_start']) for v in databases.values()}
        if len(engines)!=1 or len({v['database_oid'] for v in databases.values()})!=len(databases): raise ValueError()
        if inventory_phase.verify_current(token)!=observed or package_phase.verify_current(token)!=candidate or \
                inventory_phase.fence.ledger.read()!=package_phase.fence.ledger.read() or \
                (full_catalog and catalog_phase.verify_current(token)!=catalog_receipt) or \
                (realtime_absent is not None and realtime_absent.verify_current(token)!=absent): raise ValueError()
        return {'operation_id':candidate['operation_id'],'source_sha':candidate['source_sha'],
                'package_sha256':candidate['package_sha256'],'writers':writers,'databases':databases,
                'unchanged_images':unchanged,'unobserved_nonowners':nonowners,
                'full_catalog':full_catalog,'unmanaged_databases':sorted(set(databases)-set(metadata.OWNERS.values())-{'gateway_db'}),
                'global_database_oids':copy.deepcopy(catalog_receipt['all_databases']) if full_catalog else {db:databases[db]['database_oid'] for db in databases},
                'catalog_properties':copy.deepcopy(catalog_receipt['properties']) if full_catalog else None,
                'excluded_databases':copy.deepcopy(catalog_receipt['excluded']) if full_catalog else {},
                'catalog_receipt_sha256':hashlib.sha256(json.dumps(catalog_receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest() if full_catalog else None,
                'scope':'all-connectable-databases' if full_catalog else 'image-changed-source-observed-routes-only'}
    except BaseException:
        raise AffectedError('affected writer routes unavailable') from None


def drain(package_phase, inventory_phase, token, cut, catalog_phase=None, realtime_absent=None):
    """Re-derive immediately, then invoke only an exactly matching writer cut."""
    try:
        if type(cut) is not writer_cut.WriterCut or cut.fence.context!=package_phase.fence.context or \
                cut.lock!=package_phase.lock or cut.namespace_uid!=package_phase.namespace_uid: raise ValueError()
        plan=build(package_phase,inventory_phase,token,catalog_phase,realtime_absent)
        sources=cut.admission['controllers']; names=[x['metadata']['name'] for x in sources]
        if len(names)!=len(set(names)) or set(names)!=set(plan['writers']): raise ValueError()
        for source in sources:
            name=source['metadata']['name']; actual=plan['writers'][name]; meta=source['metadata']
            spec_hash=hashlib.sha256(json.dumps(source['spec'],sort_keys=True,separators=(',',':')).encode()).hexdigest()
            if (source['kind']!='Deployment' or meta.get('uid')!=actual['uid'] or
                    meta.get('generation')!=actual['generation'] or spec_hash!=actual['spec_sha256']): raise ValueError()
        dbs=plan['databases']; first=next(iter(dbs.values()))
        scope={'engine':first['engine'],'server_start':first['server_start'],'databases':sorted(plan['global_database_oids']),
               'database_oids':{db:plan['global_database_oids'][db] for db in sorted(plan['global_database_oids'])}}
        if cut.admission['session_scope']!=scope: raise ValueError()
        if plan['full_catalog'] and (type(cut.probe) is not CatalogScopedProbe or cut.probe.catalog is not catalog_phase or
                cut.probe.token!=token): raise ValueError()
        for kind,resource in (('jobs','jobs'),('cronjobs','cronjobs')):
            actual=writer_cut.inventory(cut.client.call('get',resource,'-o','json')['items'])
            if actual!=cut.admission['guards'][kind]: raise ValueError()
            uids=[item['metadata']['uid'] for item in actual]
            if len(set(uids))!=len(uids) or set(cut.admission['linked'][kind])!=set(uids): raise ValueError()
        if cut.fence.ledger.read()!=package_phase.fence.ledger.read(): raise ValueError()
        result=cut.drain()
        return {'plan':plan,'cut':result}
    except BaseException:
        raise AffectedError('affected writer cut unavailable') from None
