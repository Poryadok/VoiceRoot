"""Fence isolated cohort restore and each owned Docker resource attempt."""
import copy
import hashlib
import json
from pathlib import Path
import re
import secrets

import cohort_readback as readback
import cohort_upload_phase as upload
import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as restore


class RestoreError(RuntimeError): pass


class RestoreJournal:
    def __init__(self, phase, drain, snapshot, readback_done, intent):
        self.phase, self.drain, self.snapshot, self.readback_done, self.intent = phase, drain, snapshot, readback_done, intent
        self.records = {}

    def _check(self, row):
        if (readback._snapshot(self.phase, row) != self.snapshot or row.get('cohort_readback') != self.readback_done
                or row.get('cohort_restore') != self.intent
                or self.phase.phases.journal.state['steps'].get('cohort-readback') != self.readback_done
                or self.phase.phases.journal.state['steps'].get('cohort-restore') != self.intent): raise ValueError()
        local = {name.removeprefix('cohort-restore-resource-'):value
                 for name,value in self.phase.phases.journal.state['steps'].items()
                 if name.startswith('cohort-restore-resource-')}
        if row.get('cohort_restore_resources', {}) != self.records or local != self.records: raise ValueError()

    def _transition(self, kind, expected, target):
        started = False; p = self.phase.phases
        try:
            if kind not in ('postgres','migrate'): raise ValueError()
            readback._guard(self.phase, self.drain); row, revision = p.fence.ledger.read(); self._check(row)
            key = 'cohort-restore-resource-' + kind; records = row.get('cohort_restore_resources', {})
            if (not isinstance(records,dict) or set(records)-{'postgres','migrate'}
                    or records.get(kind) != expected or p.journal.state['steps'].get(key) != expected): raise ValueError()
            started = True; p.journal.state['steps'][key] = copy.deepcopy(target); p.journal.save()
            current, rev = p.fence.ledger.read()
            if current != row or rev != revision: raise ValueError()
            current.setdefault('cohort_restore_resources', {})[kind] = copy.deepcopy(target)
            current['step'] = key + '-' + target['status']; p.fence.ledger.write(rev,current)
            self.records[kind] = copy.deepcopy(target)
            readback._guard(self.phase,self.drain); checked, _ = p.fence.ledger.read(); self._check(checked)
            if checked.get('cohort_restore_resources',{}).get(kind) != target: raise ValueError()
        except BaseException:
            if started: readback._poison(self.phase)
            raise RestoreError('restore resource not admitted; reconcile if pending') from None

    def begin(self, kind, name, labels):
        if not isinstance(name,str) or not re.fullmatch(r'voice-media-restore-[0-9a-f]{32}',name) or labels != {'voice.test.owner':name}:
            raise RestoreError('restore resource identity invalid')
        nonce = secrets.token_hex(16)
        target = {'status':'creating','nonce':nonce,'name':name,'labels':copy.deepcopy(labels),
                  'operation_id':self.intent['operation_id'],'invocation_nonce':self.intent['invocation_nonce']}
        self._transition(kind,None,target); return nonce

    def created(self, kind, nonce, cid, labels):
        current = self.phase.phases.journal.state['steps'].get('cohort-restore-resource-'+kind)
        if (not isinstance(current,dict) or current.get('status') != 'creating' or current.get('nonce') != nonce
                or current.get('labels') != labels or not isinstance(cid,str) or not re.fullmatch(r'[0-9a-f]{64}',cid)):
            readback._poison(self.phase); raise RestoreError('restore resource uncertain')
        self._transition(kind,current,current | {'status':'created','container_id':cid})

    def begin_remove(self, kind, cid):
        current = self.phase.phases.journal.state['steps'].get('cohort-restore-resource-'+kind)
        if not isinstance(current,dict) or current.get('status') != 'created' or current.get('container_id') != cid:
            readback._poison(self.phase); raise RestoreError('restore resource uncertain')
        self._transition(kind,current,current | {'status':'removing'})

    def removed(self, kind, cid, absent):
        current = self.phase.phases.journal.state['steps'].get('cohort-restore-resource-'+kind)
        if not isinstance(current,dict) or current.get('status') != 'removing' or current.get('container_id') != cid or absent is not True:
            readback._poison(self.phase); raise RestoreError('restore resource uncertain')
        self._transition(kind,current,current | {'status':'removed'})

    def uncertain(self, kind, nonce):
        readback._poison(self.phase); raise RestoreError('restore resource uncertain')


def run(phase, key, destination, manifest, metadata, source_root, postgres_image, migrate_image, drain):
    """One actual isolated restore; caller owns source authority and drain."""
    pending = False; p = phase.phases
    try:
        readback._guard(phase,drain); row, revision = p.fence.ledger.read()
        snapshot = readback._snapshot(phase,row); binding, planned, marker, _ = snapshot
        done = p.journal.state['steps'].get('cohort-readback')
        if (not isinstance(done,dict) or set(done) != {'status','intent','receipt'} or done['status'] != 'done'
                or row.get('cohort_readback') != done or 'cohort_restore' in row
                or 'cohort-restore' in p.journal.state['steps'] or 'cohort_restore_resources' in row
                or Path(destination) != upload.ROOT/planned['operation_id']/('cohort-readback-'+planned['run_id']+'-'+planned['challenge'])): raise ValueError()
        expected_receipt = {'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot',
                            'artifact_id':done['intent']['artifact_id'],
                            **{k:v for k,v in done['intent']['binding'].items() if k != 'created_at'}}
        if (done['receipt'] != expected_receipt or done['intent']['status'] != 'pending'
                or done['intent']['manifest_sha256'] != marker['manifest_sha256']
                or done['intent']['invocation_nonce'] != p.invocation
                or any(done['intent']['binding'].get(k) != planned[k] for k in
                       ('operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256'))
                or done['intent']['binding'].get('head_sha') != planned['source_sha']
                or type(done['intent']['binding'].get('run_id')) is not int
                or str(done['intent']['binding']['run_id']) != planned['run_id']): raise ValueError()
        digest = hashlib.sha256(json.dumps(manifest,sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()
        if digest != marker['manifest_sha256']: raise ValueError()
        cohort.verify(destination,key,binding,binding['databases'],manifest)
        intent = {'status':'pending','operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                  'invocation_nonce':p.invocation,'manifest_sha256':digest,'artifact_id':done['receipt']['artifact_id']}
        readback._guard(phase,drain); pending = True
        p.journal.state['steps']['cohort-restore'] = copy.deepcopy(intent); p.journal.save()
        current, rev = p.fence.ledger.read()
        if current != row or rev != revision: raise ValueError()
        current['cohort_restore'] = copy.deepcopy(intent); current['step'] = 'cohort-restore-pending'; p.fence.ledger.write(rev,current)
        readback._guard(phase,drain); reserved, _ = p.fence.ledger.read()
        journal = RestoreJournal(phase,drain,snapshot,done,intent); journal._check(reserved)
        receipt = restore.qualify(destination,key,binding,binding['databases'],manifest,metadata,source_root,
                                  postgres_image,migrate_image,journal)
        if (not isinstance(receipt,dict) or receipt.get('operation_id') != binding['operation_id']
                or receipt.get('snapshot_id') != binding['snapshot_id'] or receipt.get('source_sha') != binding['source_sha']
                or receipt.get('manifest_sha256') != digest or receipt.get('cleanup') != 'verified'
                or set(receipt.get('databases',{})) != set(binding['databases'])): raise ValueError()
        readback._guard(phase,drain); current, rev = p.fence.ledger.read(); journal._check(current)
        records = current.get('cohort_restore_resources',{})
        if not records or any(v.get('status') != 'removed' for v in records.values()): raise ValueError()
        result = {'status':'done','intent':copy.deepcopy(intent),'receipt':copy.deepcopy(receipt)}
        p.journal.state['steps']['cohort-restore'] = copy.deepcopy(result); p.journal.save()
        current['cohort_restore'] = copy.deepcopy(result); current['step'] = 'cohort-restore-done'; p.fence.ledger.write(rev,current)
        readback._guard(phase,drain); final, _ = p.fence.ledger.read()
        local_resources = {name.removeprefix('cohort-restore-resource-'):value for name,value in p.journal.state['steps'].items()
                           if name.startswith('cohort-restore-resource-')}
        if (readback._snapshot(phase,final) != snapshot or final.get('cohort_readback') != done
                or final.get('cohort_restore') != result or final.get('cohort_restore_resources') != journal.records
                or local_resources != journal.records): raise ValueError()
        return receipt
    except BaseException:
        if pending: readback._poison(phase)
        raise RestoreError('cohort restore incomplete; retain lock and reconcile' if pending else 'cohort restore not admitted') from None
