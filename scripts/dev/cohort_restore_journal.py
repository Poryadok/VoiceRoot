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
import cohort_backup_phases as phases


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


def verify_go_role(phase, metadata_phase, token, database):
    """Verify the completed source-bound clone migration for one Go login."""
    try:
        import full_cohort_backup_metadata as metadata_source
        if type(phase) is not phases.CohortPhases or type(metadata_phase) is not metadata_source.BackupMetadataPhase:
            raise ValueError()
        p=phase.phases
        phase._guard(); row, revision=p.fence.ledger.read(); row=copy.deepcopy(row)
        snapshot=copy.deepcopy(readback._snapshot(phase,row))
        binding, _, marker, _ = snapshot
        if 'full_catalog' not in binding: raise ValueError()
        source=metadata_source.verify_completed(metadata_phase,token)
        changed=[db for db in binding['databases'] if restore.classify_plan(db,source['metadata']['migrations'][db],True)=='migrate']
        roles=restore._qualified_roles(metadata_phase,token,phase.catalog_phase,binding,binding['databases'],source['metadata'],changed)
        if (database not in roles or metadata_phase.fence.context!=p.fence.context or metadata_phase.lock!=p.lock
                or metadata_phase.namespace_uid!=p.namespace_uid or metadata_phase.fence.ledger.read()!=(row,revision)):
            raise ValueError()
        done=row.get('cohort_restore');local=p.journal.state['steps'].get('cohort-restore')
        if not isinstance(done,dict) or done!=local or set(done)!={'status','intent','receipt'} or done['status']!='done':
            raise ValueError()
        readback_done=row.get('cohort_readback')
        if (not isinstance(readback_done,dict) or readback_done!=p.journal.state['steps'].get('cohort-readback')
                or readback_done.get('status')!='done' or readback_done.get('intent',{}).get('manifest_sha256')!=marker['manifest_sha256']
                or readback_done.get('receipt',{}).get('artifact_id')!=done['intent'].get('artifact_id')):
            raise ValueError()
        intent=done['intent'];receipt=done['receipt']
        role_digest=hashlib.sha256(json.dumps(roles,sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()
        expected={db:{'role':roles[db]['role'],'from':source['metadata']['migrations'][db]['from'],
                      'to':source['metadata']['migrations'][db]['to']} for db in changed}
        resources=row.get('cohort_restore_resources')
        local_resources={name.removeprefix('cohort-restore-resource-'):value for name,value in p.journal.state['steps'].items()
                         if name.startswith('cohort-restore-resource-')}
        if (set(intent)!={'status','operation_id','snapshot_id','invocation_nonce','manifest_sha256','artifact_id','migration_roles_sha256'}
                or intent.get('status')!='pending'
                or intent.get('operation_id')!=binding['operation_id'] or intent.get('snapshot_id')!=binding['snapshot_id']
                or intent.get('invocation_nonce')!=p.invocation or intent.get('manifest_sha256')!=marker['manifest_sha256']
                or intent.get('migration_roles_sha256')!=role_digest or receipt.get('operation_id')!=binding['operation_id']
                or receipt.get('snapshot_id')!=binding['snapshot_id'] or receipt.get('source_sha')!=binding['source_sha']
                or receipt.get('manifest_sha256')!=marker['manifest_sha256'] or receipt.get('cleanup')!='verified'
                or receipt.get('databases',{}).get(database)!={'mode':'migrate','from':expected[database]['from'],'to':expected[database]['to']}
                or receipt.get('migration_logins')!=expected or not isinstance(resources,dict)
                or set(resources)!={'postgres','migrate'} or resources!=local_resources):
            raise ValueError()
        for resource in resources.values():
            name=resource.get('name')
            if (set(resource)!={'status','nonce','name','labels','operation_id','invocation_nonce','container_id'}
                    or resource['status']!='removed' or resource['operation_id']!=binding['operation_id']
                    or resource['invocation_nonce']!=p.invocation or not isinstance(name,str)
                    or not re.fullmatch(r'voice-media-restore-[0-9a-f]{32}',name)
                    or resource['labels']!={'voice.test.owner':name}
                    or not isinstance(resource['nonce'],str) or not re.fullmatch(r'[0-9a-f]{32}',resource['nonce'])
                    or not isinstance(resource['container_id'],str) or not re.fullmatch(r'[0-9a-f]{64}',resource['container_id'])):
                raise ValueError()
        if any(len({resource[key] for resource in resources.values()})!=2 for key in ('nonce','name','container_id')):
            raise ValueError()
        verified_done=copy.deepcopy(done);verified_readback=copy.deepcopy(readback_done);verified_resources=copy.deepcopy(local_resources)
        phase._guard()
        if (p.fence.ledger.read()!=(row,revision) or metadata_phase.fence.ledger.read()!=(row,revision)
                or readback._snapshot(phase,row)!=snapshot
                or p.journal.state['steps'].get('cohort-restore')!=verified_done
                or p.journal.state['steps'].get('cohort-readback')!=verified_readback
                or {name.removeprefix('cohort-restore-resource-'):value for name,value in p.journal.state['steps'].items()
                    if name.startswith('cohort-restore-resource-')}!=verified_resources): raise ValueError()
        return {'database':database,**expected[database],'source_sha':binding['source_sha'],
                'snapshot_id':binding['snapshot_id'],'manifest_sha256':marker['manifest_sha256'],
                'migration_roles_sha256':role_digest,'scope':'isolated-clone-migration-login-only'}
    except Exception:
        raise RestoreError('clone migration login qualification unavailable') from None


def run(phase, key, destination, manifest, metadata, source_root, postgres_image, migrate_image, drain,
        metadata_phase=None, metadata_token=None):
    """One actual isolated restore; caller owns source authority and drain."""
    pending = False; p = phase.phases
    try:
        if 'full_catalog' in phase.binding and type(phase) is not phases.CohortPhases: raise ValueError()
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
        changed = ([db for db in binding['databases']
                    if restore.classify_plan(db,metadata['migrations'][db], True) == 'migrate']
                   if 'full_catalog' in binding else [])
        qualified = None
        if 'full_catalog' in binding and changed:
            qualified = restore._qualified_roles(metadata_phase,metadata_token,phase.catalog_phase,binding,
                                                  binding['databases'],metadata,changed)
            if (metadata_phase.fence.context != p.fence.context or metadata_phase.lock != p.lock
                    or metadata_phase.namespace_uid != p.namespace_uid
                    or metadata_phase.fence.ledger.read() != p.fence.ledger.read()): raise ValueError()
        elif metadata_phase is not None or metadata_token is not None: raise ValueError()
        intent = {'status':'pending','operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                  'invocation_nonce':p.invocation,'manifest_sha256':digest,'artifact_id':done['receipt']['artifact_id']}
        if qualified is not None:
            intent['migration_roles_sha256'] = hashlib.sha256(json.dumps(qualified,sort_keys=True,separators=(',', ':'),
                                                       ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()
        readback._guard(phase,drain); pending = True
        p.journal.state['steps']['cohort-restore'] = copy.deepcopy(intent); p.journal.save()
        current, rev = p.fence.ledger.read()
        if current != row or rev != revision: raise ValueError()
        current['cohort_restore'] = copy.deepcopy(intent); current['step'] = 'cohort-restore-pending'; p.fence.ledger.write(rev,current)
        readback._guard(phase,drain); reserved, _ = p.fence.ledger.read()
        journal = RestoreJournal(phase,drain,snapshot,done,intent); journal._check(reserved)
        catalog_args = ({'catalog_phase':phase.catalog_phase,'catalog_token':phase.catalog_token}
                        if 'full_catalog' in binding else {})
        if qualified is not None:
            catalog_args.update(metadata_phase=metadata_phase, metadata_token=metadata_token)
        receipt = restore.qualify(destination,key,binding,binding['databases'],manifest,metadata,source_root,
                                  postgres_image,migrate_image,journal,**catalog_args)
        if (not isinstance(receipt,dict) or receipt.get('operation_id') != binding['operation_id']
                or receipt.get('snapshot_id') != binding['snapshot_id'] or receipt.get('source_sha') != binding['source_sha']
                or receipt.get('manifest_sha256') != digest or receipt.get('cleanup') != 'verified'
                or set(receipt.get('databases',{})) != set(binding['databases'])): raise ValueError()
        if qualified is not None and receipt.get('migration_logins') != {
                db:{'role':qualified[db]['role'],'from':metadata['migrations'][db]['from'],
                    'to':metadata['migrations'][db]['to']} for db in changed}: raise ValueError()
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
