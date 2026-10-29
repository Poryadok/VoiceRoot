"""Fenced current 23-app inventory and source-bound database observations.

This records actual workload identities and each observer result, including
HOLD. It does not declare startup compatibility or select a complete writer
cut. The central executor must resolve concrete held routes before using them.
"""
import copy
import hashlib
import json
import re
import secrets

import canonical_application_package as canonical
import frontend_runtime_executor
import full_cohort_package_phase as package_phase
import full_cohort_schema_metadata as metadata
import media_operation_guard

STEP = 'full-cohort-inventory'


class InventoryError(RuntimeError): pass


def _owner(obj, kind):
    refs = [x for x in obj.get('metadata',{}).get('ownerReferences',[]) if x.get('controller') is True]
    if len(refs)!=1 or refs[0].get('kind')!=kind or not isinstance(refs[0].get('uid'),str) or not refs[0]['uid']:
        return None
    return refs[0]['uid']


def inventory(client):
    """Return private current specs and stable owner-chain projection."""
    try:
        wanted = {'voice-'+name:name for name in metadata.CONTAINERS}
        deployments = client.call('get','deployments','-o','json')['items']
        replicasets = client.call('get','replicasets','-o','json')['items']
        pods = client.call('get','pods','-o','json')['items']
        if any(not isinstance(items,list) for items in (deployments,replicasets,pods)): raise ValueError()
        selected = {}
        for item in deployments:
            meta = item.get('metadata',{}); name = meta.get('name')
            if name not in wanted: continue
            spec,status = item.get('spec',{}),item.get('status',{})
            selector = spec.get('selector',{})
            if (name in selected or item.get('kind')!='Deployment' or meta.get('namespace')!='voice-dev' or
                    not meta.get('uid') or not meta.get('resourceVersion') or meta.get('deletionTimestamp') or
                    type(meta.get('generation')) is not int or type(spec.get('replicas')) is not int or spec['replicas']<1 or
                    status.get('observedGeneration')!=meta['generation'] or
                    any(status.get(k,0)!=spec['replicas'] for k in ('replicas','updatedReplicas','readyReplicas')) or
                    not isinstance(selector,dict) or not isinstance(selector.get('matchLabels'),dict) or
                    not selector['matchLabels'] or selector.get('matchExpressions',[])!=[]):
                raise ValueError()
            selected[name] = copy.deepcopy(item)
        if set(selected)!=set(wanted) or len({x['metadata']['uid'] for x in selected.values()})!=len(wanted): raise ValueError()
        by_uid = {value['metadata']['uid']:wanted[name] for name,value in selected.items()}
        rs_map = {}
        for item in replicasets:
            meta = item.get('metadata',{}); owner = _owner(item,'Deployment')
            if owner not in by_uid: continue
            uid = meta.get('uid')
            if (meta.get('namespace')!='voice-dev' or not isinstance(uid,str) or not uid or uid in rs_map or
                    meta.get('deletionTimestamp')): raise ValueError()
            rs_map[uid] = by_uid[owner]
        names = {key:[] for key in metadata.CONTAINERS}; details = {key:[] for key in names}; seen = set()
        for item in pods:
            owner = _owner(item,'ReplicaSet')
            meta,status = item.get('metadata',{}),item.get('status',{})
            labels = meta.get('labels',{})
            matching = [wanted[name] for name,obj in selected.items() if isinstance(labels,dict) and
                        all(labels.get(k)==v for k,v in obj['spec']['selector']['matchLabels'].items())]
            if status.get('phase') not in ('Succeeded','Failed') and matching:
                if len(matching)!=1 or owner not in rs_map or rs_map[owner]!=matching[0]: raise ValueError()
            if owner not in rs_map: continue
            workload = rs_map[owner]
            if matching != [workload]: raise ValueError()
            uid,name = meta.get('uid'),meta.get('name'); states = status.get('containerStatuses',[])
            podspec = item.get('spec',{}); containers = podspec.get('containers'); init = podspec.get('initContainers',[])
            if (meta.get('namespace')!='voice-dev' or not isinstance(uid,str) or not uid or uid in seen or
                    not isinstance(name,str) or not name or meta.get('deletionTimestamp') or status.get('phase')!='Running' or
                    not isinstance(containers,list) or not containers or not isinstance(init,list) or
                    podspec.get('ephemeralContainers',[]) or not isinstance(states,list) or not states or
                    any(s.get('ready') is not True or not isinstance(s.get('state',{}).get('running'),dict) for s in states)):
                raise ValueError()
            container_names = [c.get('name') for c in containers]
            status_names = [s.get('name') for s in states]
            if (any(not isinstance(n,str) or not n for n in container_names+status_names) or
                    len(set(container_names))!=len(container_names) or len(set(status_names))!=len(status_names) or
                    set(container_names)!=set(status_names)): raise ValueError()
            init_states = status.get('initContainerStatuses',[])
            if (not isinstance(init_states,list) or len(init_states)!=len(init) or
                    len({x.get('name') for x in init})!=len(init) or
                    len({x.get('name') for x in init_states})!=len(init_states) or
                    {x.get('name') for x in init_states}!={x.get('name') for x in init} or
                    any(type(x.get('state',{}).get('terminated',{}).get('exitCode')) is not int or
                        x['state']['terminated']['exitCode']!=0 for x in init_states)):
                raise ValueError()
            app = [s for s in states if s.get('name')==metadata.CONTAINERS[workload]]
            all_states = []
            for state in states:
                if (not isinstance(state.get('name'),str) or not state['name'] or
                        not isinstance(state.get('containerID'),str) or not state['containerID'] or
                        not isinstance(state.get('imageID'),str) or not state['imageID'] or
                        type(state.get('restartCount')) is not int or state['restartCount']<0): raise ValueError()
                all_states.append({'name':state['name'],'container_id':state['containerID'],
                    'image_id':state['imageID'],'restart_count':state['restartCount']})
            if len({x['name'] for x in all_states})!=len(all_states): raise ValueError()
            if (len(app)!=1 or not isinstance(app[0].get('containerID'),str) or not app[0]['containerID'] or
                    type(app[0].get('restartCount')) is not int or app[0]['restartCount']<0 or
                    not isinstance(app[0].get('imageID'),str) or not app[0]['imageID']): raise ValueError()
            seen.add(uid); names[workload].append(name)
            details[workload].append({'name':name,'uid':uid,'replicaset_uid':owner,
                'containers':sorted(all_states,key=lambda x:x['name'])})
        projection = {}
        for name, workload in wanted.items():
            obj = selected[name]; meta=obj['metadata']
            if len(names[workload]) != obj['spec']['replicas'] or len(set(names[workload])) != len(names[workload]): raise ValueError()
            names[workload].sort(); details[workload].sort(key=lambda x:x['name'])
            spec_hash = hashlib.sha256(json.dumps(obj['spec'],sort_keys=True,separators=(',',':')).encode()).hexdigest()
            projection[workload] = {'uid':meta['uid'],'generation':meta['generation'],
                                    'spec_sha256':spec_hash,'replicas':obj['spec']['replicas'],'pods':details[workload]}
        return names, selected, projection
    except Exception:
        raise InventoryError('current application inventory unavailable') from None


class InventoryPhase:
    def __init__(self, journal, fence, client, lock, namespace_uid, package):
        self.journal,self.fence,self.client,self.package = journal,fence,client,package
        self.lock,self.namespace_uid = copy.deepcopy(lock),namespace_uid
        self.intent = copy.deepcopy(journal.intent); self.poisoned=False; self.expected_marker=None
        if (type(package) is not package_phase.PackagePhase or type(client) is not frontend_runtime_executor.KubectlTransport or
                client.journal is not journal or getattr(fence.ledger,'client',None) is not client or
                self.intent.get('resource_context') != fence.context or self.intent.get('namespace_uid') != namespace_uid or
                package.fence.context != fence.context or package.lock != self.lock or package.namespace_uid != namespace_uid):
            raise InventoryError('inventory phase identity invalid')

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent!=self.intent or
                self.journal.state.get('intent')!=self.intent): raise InventoryError('inventory identity changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        if self.expected_marker is not None:
            row, revision = self.fence.ledger.read()
            local = self.journal.state['steps'].get(STEP)
            if (not self.fence.owned(row) or row.get('status')!='active' or
                    row.get('full_cohort_inventory')!=self.expected_marker or
                    not isinstance(local,dict) or {k:v for k,v in local.items() if k!='receipt'}!=self.expected_marker or
                    not isinstance(revision,tuple) or len(revision)!=2 or
                    any(not isinstance(v,str) or not v for v in revision)):
                raise InventoryError('inventory reservation changed')

    def _package(self,token):
        receipt = self.package.verify_current(token)
        own, own_revision = self.fence.ledger.read(); other, other_revision = self.package.fence.ledger.read()
        if own!=other or own_revision!=other_revision or not self.fence.owned(own) or own.get('status')!='active':
            raise InventoryError('package ledger differs')
        return receipt

    def _admin_role(self):
        snapshot = metadata.kube_engine_snapshot(metadata.linux_private_bytes,self._guard)
        before = snapshot(); self._guard()
        if before.get('namespace_uid') != self.namespace_uid: raise InventoryError('database namespace changed')
        args = ['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
                '--request-timeout=15s','exec','voice-postgres-0','-c','postgres','--','sh','-ec',
                metadata.PSQL_SCRIPT,'--','postgres',"SELECT json_build_object('role',current_user)"]
        value = json.loads(metadata.linux_psql(args)); self._guard()
        if (not isinstance(value,dict) or set(value)!={'role'} or not isinstance(value['role'],str) or
                not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]{0,62}',value['role']) or snapshot()!=before):
            raise InventoryError('database observer identity changed')
        return value['role']

    def run(self, token):
        pending = False
        try:
            self._guard(); package_receipt = self._package(token)
            row, revision = self.fence.ledger.read()
            if STEP in self.journal.state['steps'] or 'full_cohort_inventory' in row: raise ValueError()
            marker = {'status':'pending','nonce':secrets.token_hex(16),
                      'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save()
            self._guard(); current, actual = self.fence.ledger.read()
            if current!=row or actual!=revision or not self.fence.owned(current) or current.get('status')!='active': raise ValueError()
            current['full_cohort_inventory']=copy.deepcopy(marker); current['step']=STEP+'-pending'
            self.fence.ledger.write(actual,current); self.expected_marker=copy.deepcopy(marker); self._guard()
            def authority(sha):
                return sha==self.fence.context['commit'] and self._package(token)==package_receipt
            catalog = metadata.catalog_from_source(canonical.ROOT,self.fence.context['commit'],authority)
            self._guard(); pods, _, projection = inventory(self.client); self._guard()
            role = self._admin_role()
            observed = metadata.inspect_all(catalog,pods,role,self._guard)
            self._guard(); final_pods, controllers, final_projection = inventory(self.client); self._guard()
            if final_pods!=pods or final_projection!=projection or self._package(token)!=package_receipt: raise ValueError()
            current, rev = self.fence.ledger.read()
            if (not self.fence.owned(current) or current.get('status')!='active' or
                    current.get('full_cohort_inventory')!=marker or self.journal.state['steps'].get(STEP)!=marker): raise ValueError()
            identities = {name:{k:value['metadata'][k] for k in ('uid','resourceVersion','generation')}
                          for name,value in controllers.items()}
            receipt = {'operation_id':marker['operation_id'],'source_sha':marker['source_sha'],
                       'package_sha256':package_receipt['package_sha256'],'pods':pods,'controllers':identities,
                       'projection':final_projection,'database_observations':observed,
                       'scope':'current-inventory-and-db-metadata-only',
                       'limits':['other_writers_not_excluded','runtime_source_not_proven','startup_not_qualified']}
            digest = hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
            done = marker | {'status':'done','receipt_sha256':digest}
            local = done | {'receipt':copy.deepcopy(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(local); self.journal.save()
            current['full_cohort_inventory']=copy.deepcopy(done); current['step']=STEP+'-done'
            self.fence.ledger.write(rev,current); self.expected_marker=copy.deepcopy(done); self._guard()
            confirmed, final_revision = self.fence.ledger.read()
            if (not self.fence.owned(confirmed) or confirmed.get('status')!='active' or
                    confirmed.get('full_cohort_inventory')!=done or self.journal.state['steps'].get(STEP)!=local or
                    not isinstance(final_revision,tuple) or len(final_revision)!=2 or
                    any(not isinstance(x,str) or not x for x in final_revision)): raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                self.poisoned=True
                try: self.fence.uncertain('full cohort inventory outcome uncertain')
                except BaseException: pass
            raise InventoryError('inventory observation uncertain; reconcile' if pending else 'inventory not admitted') from None

    def verify_current(self, token):
        """Revalidate the protected receipt against current source, routes and inventory."""
        try:
            local = copy.deepcopy(self.journal.state['steps'][STEP])
            receipt = local['receipt']
            marker = {k:v for k,v in local.items() if k!='receipt'}
            digest = hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
            if (set(marker)!={'status','nonce','operation_id','source_sha','receipt_sha256'} or
                    marker['status']!='done' or marker['receipt_sha256']!=digest or
                    marker['operation_id']!=self.fence.context['operation_id'] or
                    marker['source_sha']!=self.fence.context['commit']): raise ValueError()
            self.expected_marker=copy.deepcopy(marker); self._guard()
            package_receipt=self._package(token)
            if receipt['package_sha256']!=package_receipt['package_sha256']: raise ValueError()
            def authority(sha):
                return sha==self.fence.context['commit'] and self._package(token)==package_receipt
            catalog=metadata.catalog_from_source(canonical.ROOT,self.fence.context['commit'],authority)
            self._guard(); pods,_,projection=inventory(self.client); self._guard()
            if pods!=receipt['pods'] or projection!=receipt['projection']: raise ValueError()
            observed=metadata.inspect_all(catalog,pods,self._admin_role(),self._guard)
            self._guard(); final_pods,_,final_projection=inventory(self.client); self._guard()
            if (final_pods!=pods or final_projection!=projection or observed!=receipt['database_observations'] or
                    self._package(token)!=package_receipt): raise ValueError()
            self._guard()
            if self.journal.state['steps'].get(STEP)!=local: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            raise InventoryError('inventory receipt no longer current') from None
