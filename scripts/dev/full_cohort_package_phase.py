"""Fenced admission of a real canonical full-application build package.

The exact GitHub capture and OCI reads are source-owned. The receipt is package
and platform metadata, not application runtime, rollout or node-set completeness.
"""
import copy
import hashlib
import json
from pathlib import Path
import re
import secrets
import time

import canonical_application_package as package
import frontend_runtime_executor
import full_cohort_oci as oci
import media_backup_upload_runner as private
import media_operation_guard
import publish_candidate as source
import story_media_backup as backup
import update_images

ROOT = Path('/home/pmd/voice-dev-media-operations')
STEP = 'full-cohort-package'


class PackageError(RuntimeError): pass


class PackagePhase:
    def __init__(self, journal, fence, client, lock, namespace_uid):
        self.journal,self.fence,self.client = journal,fence,client
        self.lock,self.namespace_uid = copy.deepcopy(lock),namespace_uid
        self.intent = copy.deepcopy(journal.intent); self.poisoned = False
        self.plan = copy.deepcopy(self.intent.get('full_cohort_package'))
        operation = fence.context.get('operation_id')
        match = re.fullmatch(r'media-c([12])-[0-9a-f]{32}',operation) if isinstance(operation,str) else None
        if (not match or self.intent.get('resource_context') != fence.context or
                self.intent.get('namespace_uid') != namespace_uid or
                type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                getattr(fence.ledger,'client',None) is not client or not isinstance(self.plan,dict) or
                set(self.plan) != {'run_id','run_attempt','nodes'} or
                type(self.plan['run_id']) is not int or self.plan['run_id'] <= 0 or
                type(self.plan['run_attempt']) is not int or self.plan['run_attempt'] <= 0 or
                not isinstance(self.plan['nodes'],list) or not self.plan['nodes']):
            raise PackageError('package phase identity invalid')
        self.cycle = int(match.group(1))
        names = set()
        for node in self.plan['nodes']:
            if (not isinstance(node,dict) or set(node)!={'name','uid','hostname','architecture'} or
                    node.get('architecture')!='amd64' or
                    any(not isinstance(node.get(k),str) or not node[k] for k in ('name','uid','hostname')) or
                    node['name'] in names): raise PackageError('node plan invalid')
            names.add(node['name'])

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent): raise PackageError('package identity changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)

    def _nodes(self):
        for expected in self.plan['nodes']:
            self._guard(); actual = self.client.call('get','node',expected['name'],'-o','json'); self._guard()
            meta,status = actual.get('metadata',{}),actual.get('status',{})
            info = status.get('nodeInfo',{}); labels = meta.get('labels',{})
            if (meta.get('name') != expected['name'] or meta.get('uid') != expected['uid'] or
                    meta.get('deletionTimestamp') or labels.get('kubernetes.io/hostname') != expected['hostname'] or
                    labels.get('kubernetes.io/os') != 'linux' or labels.get('kubernetes.io/arch') != 'amd64' or
                    info.get('operatingSystem') != 'linux' or info.get('architecture') != 'amd64' or
                    actual.get('spec',{}).get('unschedulable',False) is not False or
                    not any(c.get('type')=='Ready' and c.get('status')=='True' for c in status.get('conditions',[]))):
                raise PackageError('node identity changed')

    def _record(self, row, revision, expected):
        if (not self.fence.owned(row) or row.get('status') != 'active' or
                row.get('full_cohort_package') != expected or not isinstance(revision,tuple) or
                len(revision) != 2 or any(not isinstance(v,str) or not v for v in revision)):
            raise PackageError('package record changed')

    def verify_current(self, token):
        """Reopen the same private receipt and exact completed ledger record."""
        try:
            if not isinstance(token,str) or not token: raise PackageError('package token unavailable')
            self._guard(); self._nodes(); row, revision = self.fence.ledger.read()
            self._guard()
            local = copy.deepcopy(self.journal.state['steps'].get(STEP))
            if not isinstance(local,dict) or not isinstance(local.get('receipt'),dict): raise PackageError('package receipt missing')
            receipt = local['receipt']; done = {k:v for k,v in local.items() if k!='receipt'}
            self._record(row,revision,done)
            digest = hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
            if (done.get('status')!='done' or done.get('receipt_sha256')!=digest or
                    receipt.get('operation_id')!=self.fence.context['operation_id'] or
                    receipt.get('source_sha')!=self.fence.context['commit'] or
                    receipt.get('images')!=self.fence.context['images'] or receipt.get('nodes')!=self.plan['nodes'] or
                    receipt.get('run_id')!=self.plan['run_id'] or receipt.get('run_attempt')!=self.plan['run_attempt']):
                raise PackageError('package receipt invalid')
            oci.validate_provenance(receipt['images'],receipt['platform_digests'],receipt['oci_provenance'])
            root = ROOT / self.fence.context['operation_id']
            raw = private.private_file(root,'canonical-application-package.json',1048576)
            if hashlib.sha256(raw).hexdigest()!=receipt.get('package_sha256'):
                raise PackageError('package file changed')
            self._guard(); self._nodes(); current, final_revision = self.fence.ledger.read()
            self._record(current,final_revision,done)
            if self.journal.state['steps'].get(STEP)!=local: raise PackageError('package local receipt changed')
            return copy.deepcopy(receipt)
        except BaseException:
            raise PackageError('completed package authority unavailable') from None

    def run(self, token, actor):
        pending = False
        try:
            self._guard(); self._nodes(); row, revision = self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status')!='active' or STEP in self.journal.state['steps'] or
                    'full_cohort_package' in row or not isinstance(token,str) or not token or
                    not isinstance(actor,str) or not actor): raise PackageError('package preflight refused')
            operation = self.fence.context['operation_id']; root = ROOT / operation
            with backup.private_directory(root): pass
            output = root / 'canonical-application-package.json'
            marker = {'status':'pending','nonce':secrets.token_hex(16),'operation_id':operation,
                      'source_sha':self.fence.context['commit'],'run_id':self.plan['run_id'],
                      'run_attempt':self.plan['run_attempt']}
            pending = True; self.journal.state['steps'][STEP] = copy.deepcopy(marker); self.journal.save()
            self._guard(); current, actual = self.fence.ledger.read()
            if current != row or actual != revision: raise PackageError('package ledger changed')
            current['full_cohort_package'] = copy.deepcopy(marker); current['step'] = STEP+'-pending'
            self.fence.ledger.write(actual,current); self._guard()
            row, reserved_revision = self.fence.ledger.read()
            self._record(row,reserved_revision,marker)
            if self.journal.state['steps'].get(STEP) != marker: raise PackageError('package local reservation changed')
            captured = package.capture(token,self.fence.context['commit'],self.plan['run_id'],self.plan['run_attempt'],output)
            self._guard(); raw = private.private_file(root,output.name,1048576)
            decoded = json.loads(raw,object_pairs_hook=private.unique_object)
            if decoded != captured or not isinstance(captured,dict): raise PackageError('package receipt changed')
            package.validate_images(captured['images'])
            mapped = {'voice-'+name+'/'+container:captured['images'][name] for name,container in update_images.SERVICES.items()}
            request = captured.get('request',{})
            if (captured.get('schema') != 'voice-canonical-application-package-v1' or
                    captured.get('source_sha') != self.fence.context['commit'] or captured.get('verifier_sha') != self.fence.context['commit'] or
                    type(captured.get('run_id')) is not int or captured['run_id'] != self.plan['run_id'] or
                    type(captured.get('run_attempt')) is not int or captured['run_attempt'] != self.plan['run_attempt'] or
                    type(request.get('cycle')) is not int or request['cycle'] != self.cycle or
                    'source_equivalence' in captured or
                    captured.get('runtime_qualified') is not False or captured.get('deployment_performed') is not False or
                    mapped != self.fence.context['images']): raise PackageError('canonical package differs')
            platform,provenance = oci.resolve_provenance(captured['images'],source.Registry(actor,token),self.fence.context['commit'])
            if set(platform) != set(mapped): raise PackageError('platform inventory differs')
            oci.validate_provenance(mapped,platform,provenance)
            package.source._head(package.source._headers(token),self.fence.context['commit'],time.monotonic()+30)
            self._guard(); self._nodes(); current, rev = self.fence.ledger.read()
            self._record(current,rev,marker)
            if (self.journal.state['steps'].get(STEP) != marker or private.private_file(root,output.name,1048576)!=raw):
                raise PackageError('package authority changed')
            receipt = {'operation_id':operation,'source_sha':self.fence.context['commit'],
                       'run_id':self.plan['run_id'],'run_attempt':self.plan['run_attempt'],
                       'package_sha256':hashlib.sha256(raw).hexdigest(), 'images':mapped,
                       'platform_digests':platform, 'oci_provenance':provenance,'nodes':copy.deepcopy(self.plan['nodes'])}
            digest = hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
            done = marker | {'status':'done','receipt_sha256':digest}
            self.journal.state['steps'][STEP] = copy.deepcopy(done) | {'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current['full_cohort_package'] = done; current['step'] = STEP+'-done'
            self.fence.ledger.write(rev,current); self._guard()
            confirmed, final_revision = self.fence.ledger.read()
            self._record(confirmed,final_revision,done)
            if self.journal.state['steps'].get(STEP) != done | {'receipt':receipt}:
                raise PackageError('package local terminal record changed')
            self._nodes(); self._guard(); final,version=self.fence.ledger.read()
            if final!=confirmed or version!=final_revision or self.journal.state['steps'].get(STEP)!=done | {'receipt':receipt}:
                raise PackageError('package terminal authority changed')
            return receipt
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('full application package admission uncertain')
                except BaseException: pass
            raise PackageError('package admission uncertain; reconcile' if pending else 'package admission refused') from None
