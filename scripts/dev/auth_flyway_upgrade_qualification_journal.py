"""Fence one Auth Flyway clone against authenticated cohort readback/restore.

Only resource metadata and receipt hashes reach the common journal/ledger.
The in-memory OwnedDocker state may contain private qualification evidence.
Any ambiguous transition poisons the common fence and requires reconciliation.
"""
import copy
import hashlib
import json
import re
import secrets
import subprocess

import cohort_readback as readback
import story_media_backup_cohort_restore as restore
from auth_user_adoption_runtime import QUALIFIED_TOOLS


class CloneJournalError(RuntimeError):
    pass


class AuthFlywayJournal:
    def __init__(self, phase, drain, binding, manifest_sha256, custody_sha256,
                 restore_sha256, plan_sha256, source_metadata):
        self.phase, self.drain, self.p = phase, drain, phase.phases
        self.state, self.saved_state = {}, {}
        self.poisoned = False
        self.started = False
        self.binding = copy.deepcopy(binding)
        self.source_metadata = copy.deepcopy(source_metadata)
        self.hashes = {'manifest_sha256': manifest_sha256, 'custody_sha256': custody_sha256,
                       'restore_sha256': restore_sha256, 'plan_sha256': plan_sha256,
                       'metadata_sha256': _digest(source_metadata)}
        try:
            if any(not isinstance(v,str) or not re.fullmatch(r'[0-9a-f]{64}',v) or v == '0'*64 for v in self.hashes.values()):
                raise ValueError()
            readback._guard(phase,drain)
            row, revision = self.p.fence.ledger.read()
            self.snapshot = readback._snapshot(phase,row)
            actual, planned, marker, _ = self.snapshot
            if actual != self.binding or marker['manifest_sha256'] != manifest_sha256 or 'auth_db' not in actual['databases']:
                raise ValueError()
            self.readback = copy.deepcopy(self.p.journal.state['steps']['cohort-readback'])
            self.restored = copy.deepcopy(self.p.journal.state['steps']['cohort-restore'])
            self.restore_resources = copy.deepcopy(row['cohort_restore_resources'])
            self._validate_sources(row,planned,marker)
            inputs = actual['restore_inputs']
            restore._source(actual,actual['databases'],source_metadata,inputs['postgres_image'],inputs['migrate_image'])
            self.source_catalog = copy.deepcopy(source_metadata['migrations']['auth_db']['files'])
            if not isinstance(self.source_catalog,dict) or not self.source_catalog: raise ValueError()
            if ('cohort-auth-flyway' in self.p.journal.state['steps'] or
                    'cohort-auth-flyway-private' in self.p.journal.state['steps'] or
                    'cohort_auth_flyway' in row): raise ValueError()
            self.record = {'status':'pending','operation_id':actual['operation_id'],
                           'snapshot_id':actual['snapshot_id'],'source_sha':actual['source_sha'],
                           'invocation_nonce':self.p.invocation,'nonce':secrets.token_hex(16),
                           **self.hashes,'stage':None,'resources':[]}
            self.started = True
            self.p.journal.state['steps']['cohort-auth-flyway'] = copy.deepcopy(self.record)
            self.p.journal.save()
            current, rev = self.p.fence.ledger.read()
            if current != row or rev != revision: raise ValueError()
            current['cohort_auth_flyway'] = copy.deepcopy(self.record)
            current['step'] = 'cohort-auth-flyway-pending'
            self.p.fence.ledger.write(rev,current)
            self._guard_common()
        except BaseException:
            if self.started: self._poison()
            raise CloneJournalError('Auth clone not admitted; reconcile any pending operation') from None

    def _validate_sources(self,row,planned,marker):
        p=self.p; rd=self.readback; rs=self.restored; binding=self.binding
        if (not isinstance(rd,dict) or set(rd) != {'status','intent','receipt'} or rd['status'] != 'done'
                or row.get('cohort_readback') != rd or _digest(rd['receipt']) != self.hashes['custody_sha256']
                or rd['intent'].get('manifest_sha256') != marker['manifest_sha256']
                or rd['intent'].get('invocation_nonce') != p.invocation
                or rd['receipt'].get('artifact_id') != rd['intent'].get('artifact_id')
                or rd['receipt'].get('operation_id') != binding['operation_id']
                or rd['receipt'].get('snapshot_id') != binding['snapshot_id']
                or rd['receipt'].get('head_sha') != binding['source_sha']
                or str(rd['receipt'].get('run_id')) != planned['run_id']
                or not isinstance(rs,dict) or set(rs) != {'status','intent','receipt'} or rs['status'] != 'done'
                or row.get('cohort_restore') != rs or _digest(rs['receipt']) != self.hashes['restore_sha256']): raise ValueError()
        artifact=rd['intent'].get('binding')
        if (set(rd['intent']) != {'status','artifact_id','binding','manifest_sha256','invocation_nonce'}
                or type(rd['intent'].get('artifact_id')) is not int or rd['intent']['artifact_id'] <= 0
                or not isinstance(artifact,dict) or set(artifact) != {'operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256','run_id','head_sha','created_at'}
                or rd['intent'].get('status') != 'pending'
                or rd['receipt'] != {'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot',
                    'artifact_id':rd['intent']['artifact_id'], **{k:v for k,v in artifact.items() if k != 'created_at'}}
                or any(artifact.get(k) != planned[k] for k in ('operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256'))
                or artifact.get('head_sha') != planned['source_sha'] or type(artifact.get('run_id')) is not int
                or str(artifact['run_id']) != planned['run_id']): raise ValueError()
        ri, rr = rs['intent'], rs['receipt']
        mixed = isinstance(ri,dict) and 'migration_roles_sha256' in ri
        if 'full_catalog' in binding:
            import auth_live_migration_phase as auth_live
            actual, provenance = auth_live.verified_sources(self.phase,self.source_metadata,26)
            if (actual != binding or provenance['custody_sha256'] != self.hashes['custody_sha256']
                    or provenance['restore_sha256'] != self.hashes['restore_sha256']): raise ValueError()
        elif mixed: raise ValueError()
        if (not isinstance(ri,dict) or set(ri) != {'status','operation_id','snapshot_id','invocation_nonce','manifest_sha256','artifact_id'} | ({'migration_roles_sha256'} if mixed else set())
                or ri.get('status') != 'pending' or not isinstance(rr,dict)
                or set(rr) != {'operation_id','snapshot_id','source_sha','manifest_sha256','databases','scope','cleanup'} | ({'migration_logins'} if mixed else set())
                or ri.get('operation_id') != binding['operation_id'] or ri.get('snapshot_id') != binding['snapshot_id']
                or ri.get('invocation_nonce') != p.invocation or ri.get('manifest_sha256') != marker['manifest_sha256']
                or ri.get('artifact_id') != rd['receipt']['artifact_id']
                or rr.get('operation_id') != binding['operation_id'] or rr.get('snapshot_id') != binding['snapshot_id']
                or rr.get('source_sha') != binding['source_sha'] or rr.get('manifest_sha256') != marker['manifest_sha256']
                or rr.get('scope') != 'selected-cohort-roles-catalog-and-ledger-only' or rr.get('cleanup') != 'verified'
                or set(rr.get('databases',{})) != set(binding['databases'])
                or rr['databases'].get('auth_db') != {'mode':'restore-only','from':26,'to':26}): raise ValueError()
        local = {k.removeprefix('cohort-restore-resource-'):v for k,v in p.journal.state['steps'].items()
                 if k.startswith('cohort-restore-resource-')}
        if (not isinstance(self.restore_resources,dict) or not self.restore_resources
                or set(self.restore_resources)-{'postgres','migrate'} or local != self.restore_resources
                or row.get('cohort_restore_resources') != self.restore_resources): raise ValueError()
        for record in self.restore_resources.values():
            if (record.get('status') != 'removed' or record.get('operation_id') != binding['operation_id']
                    or record.get('invocation_nonce') != p.invocation): raise ValueError()

    def _guard_common(self):
        if self.poisoned: raise ValueError()
        readback._guard(self.phase,self.drain)
        row, _ = self.p.fence.ledger.read()
        if readback._snapshot(self.phase,row) != self.snapshot: raise ValueError()
        self._validate_sources(row,self.snapshot[1],self.snapshot[2])
        if (self.p.journal.state['steps'].get('cohort-readback') != self.readback
                or self.p.journal.state['steps'].get('cohort-restore') != self.restored
                or self.p.journal.state['steps'].get('cohort-auth-flyway') != self.record
                or row.get('cohort_auth_flyway') != self.record): raise ValueError()
        if self.record['status'] == 'done' and _digest(self.p.journal.state['steps']['cohort-auth-flyway-private']) != self.record['receipt_sha256']:
            raise ValueError()
        return row

    def _poison(self):
        self.poisoned = True
        readback._poison(self.phase)

    def uncertain(self):
        self._poison()
        raise CloneJournalError('Auth clone outcome uncertain; retain lock and reconcile') from None

    def guard(self):
        try:
            self._guard_common()
            if self.state != self.saved_state: raise ValueError()
        except BaseException:
            self.uncertain()

    def _commit(self,target):
        try:
            self._guard_common()
            row, revision = self.p.fence.ledger.read()
            self.p.journal.state['steps']['cohort-auth-flyway'] = copy.deepcopy(target)
            self.p.journal.save()
            current, rev = self.p.fence.ledger.read()
            if current != row or rev != revision: raise ValueError()
            current['cohort_auth_flyway'] = copy.deepcopy(target)
            current['step'] = 'cohort-auth-flyway-' + target['status']
            self.p.fence.ledger.write(rev,current)
            self.record = copy.deepcopy(target)
            self._guard_common()
        except BaseException:
            self.uncertain()

    def stage(self,name,status):
        try:
            self.guard()
            if self.record['status'] != 'pending': raise ValueError()
            if name != 'flyway-migrate' or status not in ('pending','done'): raise ValueError()
            previous = self.record['stage']
            resources = self.record['resources']
            if (status == 'pending' and (previous is not None or len(resources) != 2)
                    or status == 'done' and (previous != 'pending' or len(resources) != 3)
                    or any(r['state'] != 'owned' for r in resources)): raise ValueError()
            target=copy.deepcopy(self.record); target['stage']=status; self._commit(target)
        except BaseException:
            self.uncertain()

    def save(self):
        try:
            self._guard_common()
            if self.record['status'] != 'pending': raise ValueError()
            if set(self.state) != {'flyway_clone'} or not isinstance(self.state['flyway_clone'],dict): raise ValueError()
            clone = self.state['flyway_clone']; status=clone.get('status'); resources=clone.get('resources')
            if status not in ('starting','complete') or not isinstance(resources,list): raise ValueError()
            if set(clone) != ({'status','resources'} if status == 'starting' else {'status','resources','receipt'}): raise ValueError()
            previous = self.record['resources']
            _transition(previous,resources,self.record['stage'])
            target=copy.deepcopy(self.record); target['resources']=copy.deepcopy(resources)
            if status == 'complete':
                if self.record['stage'] != 'done' or len(resources) != 4 or any(r['state'] != 'removed' for r in resources): raise ValueError()
                receipt=clone['receipt']; b=self.binding
                receipt_fields={'protocol','operation_id','snapshot_id','source_sha','target','plan_sha256',
                    'manifest_sha256','custody_sha256','restore_sha256','metadata_sha256','source_catalog',
                    'postgres_image','flyway_image','before_history','after_history','before_markers',
                    'expected_after_markers','after_markers','original_table_columns','before_catalog_sha256',
                    'after_catalog_sha256','roles_sha256','scope','cleanup','resources'}
                if (not isinstance(receipt,dict) or set(receipt) != receipt_fields or receipt.get('protocol') != 'voice-auth-flyway-cohort-clone-v1'
                        or receipt.get('operation_id') != b['operation_id'] or receipt.get('snapshot_id') != b['snapshot_id']
                        or receipt.get('source_sha') != b['source_sha'] or type(receipt.get('target')) is not int or receipt['target'] != 32
                        or any(receipt.get(k) != v for k,v in self.hashes.items())
                        or receipt.get('postgres_image') != QUALIFIED_TOOLS['postgres']
                        or receipt.get('flyway_image') != QUALIFIED_TOOLS['flyway']
                        or receipt.get('source_catalog') != self.source_catalog
                        or any(not isinstance(receipt.get(k),str) or not re.fullmatch(r'[0-9a-f]{64}',receipt[k]) for k in ('before_catalog_sha256','after_catalog_sha256','roles_sha256'))
                        or receipt.get('scope') != 'isolated-auth-clone-only' or receipt.get('cleanup') != 'confirmed'
                        or receipt.get('resources') != resources): raise ValueError()
                _assert_gone(resources)
                self._guard_common()
                # The common local Journal is mode 0600 and fsyncs before this
                # CAS. The public ledger receives only the receipt hash.
                if 'cohort-auth-flyway-private' in self.p.journal.state['steps']: raise ValueError()
                self.p.journal.state['steps']['cohort-auth-flyway-private']=copy.deepcopy(receipt)
                self.p.journal.save()
                self._guard_common()
                target['status']='done'; target['receipt_sha256']=_digest(receipt)
            self._commit(target)
            self.saved_state=copy.deepcopy(self.state)
        except BaseException:
            self.uncertain()


def _assert_gone(resources):
    def query(*args):
        result=subprocess.run(['docker',*args],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=30)
        if result.returncode or len(result.stdout) > 4*1024*1024: raise ValueError()
        return result.stdout.decode('ascii').splitlines()
    ids=set(query('ps','-a','--no-trunc','--format','{{.ID}}'))
    names=set(query('ps','-a','--format','{{.Names}}'))
    volumes=set(query('volume','ls','--format','{{.Name}}'))
    if any(r['id'] in ids or r['name_hint'] in names or volumes.intersection(r['volumes']) for r in resources):
        raise ValueError()


def _digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()


def _transition(previous,resources,stage):
    if len(resources) > 4: raise ValueError()
    for index, record in enumerate(resources):
        status=record.get('state') if isinstance(record,dict) else None
        fields={'name_hint','image','image_id','state'}
        if status in ('created','owned','removing','removed'): fields.add('id')
        if status in ('owned','removing','removed'): fields.add('volumes')
        wanted = QUALIFIED_TOOLS['postgres'] if index == 0 else QUALIFIED_TOOLS['flyway']
        if (status not in ('creating','created','owned','removing','removed') or set(record) != fields
                or not re.fullmatch(r'voice-auth-flyway-[0-9a-f]{32}',str(record['name_hint']))
                or record['image'] != wanted or not re.fullmatch(r'sha256:[0-9a-f]{64}',str(record['image_id']))
                or ('id' in record and not re.fullmatch(r'[0-9a-f]{64}',str(record['id'])))
                or ('volumes' in record and (not isinstance(record['volumes'],list) or len(record['volumes']) != len(set(record['volumes']))
                    or any(not isinstance(v,str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,255}',v) for v in record['volumes'])))): raise ValueError()
    if len({r['name_hint'] for r in resources}) != len(resources) or len({r['id'] for r in resources if 'id' in r}) != sum('id' in r for r in resources): raise ValueError()
    if len({r['image_id'] for r in resources[1:]}) > 1: raise ValueError()
    if previous == resources: return
    if len(resources) == len(previous)+1:
        if resources[:-1] != previous or resources[-1]['state'] != 'creating' or any(r['state'] != 'owned' for r in previous): raise ValueError()
        if (len(previous) == 0 and stage is not None or len(previous) == 1 and stage is not None
                or len(previous) == 2 and stage != 'pending' or len(previous) == 3 and stage != 'done'): raise ValueError()
        return
    if len(resources) != len(previous): raise ValueError()
    changed=[i for i in range(len(resources)) if resources[i] != previous[i]]
    if len(changed) != 1: raise ValueError()
    i=changed[0]; old=previous[i]; new=resources[i]
    allowed={'creating':'created','created':'owned','owned':'removing','removing':'removed'}
    if allowed.get(old['state']) != new['state']: raise ValueError()
    if {k:v for k,v in new.items() if k not in ('state','id','volumes')} != {k:v for k,v in old.items() if k not in ('state','id','volumes')}: raise ValueError()
    if 'id' in old and old['id'] != new.get('id') or 'volumes' in old and old['volumes'] != new.get('volumes'): raise ValueError()
    if new['state'] in ('removing','removed') and (any(r['state'] != 'owned' for r in resources[:i])
            or any(r['state'] != 'removed' for r in resources[i+1:])): raise ValueError()
