"""Prove the fixed create-only Realtime database baseline under a DEV fence."""
import copy
import hashlib
import json
import secrets

import frontend_runtime_executor
import full_cohort_affected_plan as affected
import full_cohort_inventory_phase as inventory
import full_cohort_schema_metadata as schema
import media_operation_guard
import realtime_db_provision

STEP='realtime-absent-baseline'


class AbsentError(RuntimeError): pass


def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=True).encode('ascii')).hexdigest()


class RealtimeAbsentPhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,inventory_phase,catalog_phase):
        self.journal,self.fence,self.client=journal,fence,client
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.inventory,self.catalog=inventory_phase,catalog_phase
        self.intent=copy.deepcopy(journal.intent); self.poisoned=False
        self._objects=(journal,fence,client,inventory_phase,catalog_phase)
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                fence.ledger.client is not client or type(inventory_phase) is not inventory.InventoryPhase or
                type(catalog_phase) is not affected.FullCatalogPhase or catalog_phase.inventory is not inventory_phase or
                self.intent.get('resource_context')!=fence.context or self.intent.get('namespace_uid')!=namespace_uid):
            raise AbsentError('Realtime absence phase identity invalid')
        for other in (inventory_phase,catalog_phase):
            if other.fence.context!=fence.context or other.lock!=self.lock or other.namespace_uid!=namespace_uid:
                raise AbsentError('Realtime absence authority differs')

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent!=self.intent or
                self.journal.state.get('intent')!=self.intent or
                self.client.journal is not self.journal or self.fence.ledger.client is not self.client or
                any(other.fence.context!=self.fence.context or other.lock!=self.lock or
                    other.namespace_uid!=self.namespace_uid for other in (self.inventory,self.catalog)) or
                any(x is not y for x,y in zip((self.journal,self.fence,self.client,self.inventory,self.catalog),self._objects))):
            raise AbsentError('Realtime absence identity changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        row,rev=self.fence.ledger.read()
        if self.inventory.fence.ledger.read()!=(row,rev) or self.catalog.fence.ledger.read()!=(row,rev):
            raise AbsentError('Realtime absence ledger differs')

    def _observe(self,token):
        self._guard(); initial=self.inventory.verify_current(token); cat=self.catalog.verify_current(token)
        if 'realtime_db' in cat['all_databases'] or initial['source_sha']!=self.fence.context['commit']:
            raise AbsentError('Realtime database is not absent')
        projection=initial['projection']['realtime']; names=initial['pods']['realtime']
        by_name={p['name']:p for p in projection['pods']}
        if not names or set(names)!=set(by_name): raise AbsentError('Realtime pod inventory differs')
        entries=[]
        for name in names:
            snapshot=schema.kube_workload_snapshot('realtime',name,schema.linux_private_bytes,self._guard)
            raw_runtime=schema.kube_runtime_reader('realtime',name,schema.linux_private_bytes)
            raw_key=schema.kube_key_reader(schema.linux_private_bytes)
            def runtime(variable):
                self._guard(); result=raw_runtime(variable); self._guard(); return result
            def key_reader(kind,refname,key):
                self._guard(); result=raw_key(kind,refname,key); self._guard(); return result
            first=snapshot(); schema._identity('realtime',first)
            expected=by_name[name]
            app=[x for x in expected['containers'] if x['name']=='realtime']
            if (len(app)!=1 or first['namespace_uid']!=self.namespace_uid or first['deployment_uid']!=projection['uid'] or
                    first['pod_uid']!=expected['uid'] or first['container_id']!=app[0]['container_id'] or
                    first['restart_count']!=app[0]['restart_count']): raise AbsentError('Realtime pod identity changed')
            value,refs=schema._env('realtime',first,'DATABASE_URL',key_reader)
            if value is not None or runtime('DATABASE_URL') is not None: raise AbsentError('Realtime database configured')
            second=snapshot()
            if first!=second or runtime('DATABASE_URL') is not None: raise AbsentError('Realtime runtime changed')
            for kind,refname,key,old in refs:
                if key_reader(kind,refname,key)!=old: raise AbsentError('Realtime reference changed')
            entries.append({'pod_uid':first['pod_uid'],'container_id':first['container_id'],
                'restart_count':first['restart_count'],
                'references':[{'kind':kind,'name':refname,'key':key,'uid':old['uid'],
                               'resource_version':old['resource_version']} for kind,refname,key,old in refs if old is not None]})
        pg=realtime_db_provision.KubePostgres(self._guard)
        if pg.snapshot()!=cat['engine'] or pg.catalog()!={'role':None,'database':None} or pg.snapshot()!=cat['engine']:
            raise AbsentError('Realtime create-only objects are not absent')
        if self.inventory.verify_current(token)!=initial or self.catalog.verify_current(token)!=cat:
            raise AbsentError('Realtime absence observation changed')
        self._guard()
        return {'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                'deployment_uid':projection['uid'],'generation':projection['generation'],
                'spec_sha256':projection['spec_sha256'],'catalog_sha256':digest(cat),'pods':entries,
                'scope':'realtime-create-only-baseline'}

    def run(self,token):
        pending=False
        try:
            self._guard(); row,rev=self.fence.ledger.read()
            if STEP in self.journal.state['steps'] or 'realtime_absent_baseline' in row: raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save()
            self._guard(); current,actual=self.fence.ledger.read()
            if (current,actual)!=(row,rev): raise ValueError()
            current['realtime_absent_baseline']=copy.deepcopy(marker); self.fence.ledger.write(actual,current)
            receipt=self._observe(token)
            if self._observe(token)!=receipt: raise ValueError()
            self._guard(); row,rev=self.fence.ledger.read()
            if row.get('realtime_absent_baseline')!=marker or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
            done=marker|{'status':'done','receipt_sha256':digest(receipt)}
            self.journal.state['steps'][STEP]=done|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            row['realtime_absent_baseline']=done; self.fence.ledger.write(rev,row)
            return self.verify_receipt(token)
        except BaseException:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('Realtime absence observation uncertain')
                except BaseException:pass
            raise AbsentError('Realtime absence unavailable; reconcile') from None

    def verify_receipt(self,token):
        self._guard(); local=copy.deepcopy(self.journal.state['steps'].get(STEP)); row,rev=self.fence.ledger.read()
        catalog=self.catalog.verify_receipt(token)
        if (not isinstance(local,dict) or not isinstance(local.get('receipt'),dict) or
                {k:v for k,v in local.items() if k!='receipt'}!=row.get('realtime_absent_baseline') or
                local.get('status')!='done' or local.get('receipt_sha256')!=digest(local['receipt']) or
                local['receipt'].get('operation_id')!=self.fence.context['operation_id'] or
                local['receipt'].get('source_sha')!=self.fence.context['commit'] or
                catalog.get('scope')!='all-databases-connectable-backup' or
                local['receipt'].get('catalog_sha256')!=digest(catalog)):
            raise AbsentError('Realtime absence receipt changed')
        self._guard()
        if self.journal.state['steps'].get(STEP)!=local or self.fence.ledger.read()!=(row,rev):
            raise AbsentError('Realtime absence receipt changed')
        return copy.deepcopy(local['receipt'])

    def verify_current(self,token):
        receipt=self.verify_receipt(token)
        if self._observe(token)!=receipt or self.verify_receipt(token)!=receipt:
            raise AbsentError('Realtime absence changed')
        return receipt
