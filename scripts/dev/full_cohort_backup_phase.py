"""Source-owned, fenced full-cohort backup and isolated restore consumer."""
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import stat
import time

import cohort_backup_operation
import cohort_backup_phases
import cohort_dispatch
import cohort_readback
import cohort_restore_journal
import cohort_upload_phase
import cohort_upload_runner
import frontend_runtime_executor
import full_cohort_backup_metadata
import full_cohort_pg_physical_identity
import full_cohort_supervisor
import media_backup_phase
import media_operation_journal
import publish_candidate
import resource_fencing
import story_media_backup
import writer_cut


class BackupError(RuntimeError): pass


class BackupPhase:
    """One invocation under an already-owned supervisor, writer cut and common lock."""
    def __init__(self, supervisor, cut, physical, metadata):
        if (type(supervisor) is not full_cohort_supervisor.Supervisor or type(cut) is not writer_cut.WriterCut
                or type(physical) is not full_cohort_pg_physical_identity.PhysicalPhase
                or type(metadata) is not full_cohort_backup_metadata.BackupMetadataPhase
                or metadata.package is not supervisor.package or metadata.inventory is not supervisor.inventory
                or metadata.catalog is not supervisor.catalog or metadata.cut is not cut
                or physical.catalog is not supervisor.catalog or physical.cut is not cut
                or supervisor.census is None or supervisor.intent.get('cohort_route_mode')!='full-cohort-v1'):
            raise BackupError('full cohort phase identity unavailable')
        self.supervisor,self.cut,self.physical,self.metadata=supervisor,cut,physical,metadata
        self._dependencies=(supervisor,cut,physical,metadata,supervisor.package,supervisor.inventory,supervisor.catalog,supervisor.census)
        self.journal=self.cohort=self._bound_journal=self._bound_cohort=None
        self._route=None
        self._started=False
        self._guard()

    def _guard(self):
        s=self.supervisor
        if (any(a is not b for a,b in zip((s,self.cut,self.physical,self.metadata,s.package,s.inventory,s.catalog,s.census),self._dependencies))
                or self.journal is not self._bound_journal or self.cohort is not self._bound_cohort
                or s.intent.get('cohort_route_mode')!='full-cohort-v1'):
            raise BackupError('full cohort phase identity changed')
        s._guard()
        row,revision=s.fence.ledger.read()
        for phase in (s.package,s.inventory,s.catalog,s.census,self.cut,self.physical,self.metadata):
            if (phase.fence.context!=s.fence.context or phase.lock!=s.lock or phase.namespace_uid!=s.namespace_uid
                    or phase.fence.ledger.read()!=(row,revision)):
                raise BackupError('full cohort ledger identity changed')
        if self._route is not None and (row.get('full_cohort_backup_route')!=self._route
                or self.journal.state['steps'].get('full-cohort-backup-route')!={'status':'done','route':self._route}):
            raise BackupError('full cohort route changed')
        self.cut.verify_drained()
        return row,revision

    def _source(self, token):
        self._guard()
        census=self.supervisor.census.verify_current(token,self.cut)
        physical=full_cohort_pg_physical_identity.verify_current(self.physical,token)
        metadata=full_cohort_backup_metadata.verify_current(self.metadata,token)
        admin=self.supervisor.inventory._admin_role()
        self._guard()
        return copy.deepcopy((census,physical,metadata,admin))

    @staticmethod
    def _write_key(root):
        key=secrets.token_bytes(32)
        with story_media_backup.private_directory(root) as directory:
            fd=os.open('backup.key',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=directory)
            try:
                with os.fdopen(fd,'wb',closefd=False) as stream:
                    stream.write(key);stream.flush();os.fsync(stream.fileno())
                before=os.fstat(fd)
            finally:os.close(fd)
            os.fsync(directory)
            fd=os.open('backup.key',os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=directory)
            try:
                after=os.fstat(fd);raw=os.read(fd,33)
                if (not stat.S_ISREG(after.st_mode) or after.st_uid!=os.geteuid() or stat.S_IMODE(after.st_mode)!=0o600
                        or after.st_nlink!=1 or (before.st_dev,before.st_ino)!=(after.st_dev,after.st_ino) or raw!=key):
                    raise BackupError('private key custody changed')
            finally:os.close(fd)
        return key

    def _wait(self, token, planned, hint):
        deadline=time.monotonic()+7200
        while time.monotonic()<deadline:
            self._guard();self.cohort._guard()
            self.supervisor.package.verify_current(token)
            row,_=self.supervisor.fence.ledger.read()
            if row.get('full_cohort_backup_route')!=cohort_upload_runner.route_marker(self.cohort.binding,self.supervisor.namespace_uid):
                raise BackupError('backup route changed')
            actual=row.get('cohort_upload')
            if actual==planned:
                time.sleep(5);continue
            if (not isinstance(actual,dict) or actual.get('status')!='upload-running'
                    or {k:v for k,v in actual.items() if k not in ('run_id','run_attempt')}!=dict(planned,status='upload-running')
                    or actual.get('run_attempt')!=1 or not isinstance(actual.get('run_id'),str)
                    or not actual['run_id'].isdigit() or (hint is not None and hint!=actual['run_id'])):
                raise BackupError('backup upload claim changed')
            run=cohort_readback.get_json(token,cohort_readback.API+actual['run_id'])
            if (run.get('id')!=int(actual['run_id']) or run.get('head_sha')!=planned['source_sha']
                    or run.get('head_branch')!='master' or run.get('event')!='workflow_dispatch'
                    or run.get('path')!=cohort_readback.WORKFLOW or run.get('run_attempt')!=1):
                raise BackupError('backup workflow identity changed')
            if run.get('status')=='completed':
                if run.get('conclusion')!='success':raise BackupError('backup workflow failed')
                return actual
            if run.get('status') not in ('queued','in_progress','waiting','requested','pending'):
                raise BackupError('backup workflow status unknown')
            time.sleep(5)
        raise BackupError('backup upload deadline exceeded')

    def run(self, token):
        if self._started:raise BackupError('backup attempt requires reconciliation')
        self._started=True;pending=False
        s=self.supervisor;root=cohort_backup_operation.ROOT/s.context['operation_id']
        try:
            original=self._source(token);_,physical,source,admin=original
            snapshot='capture-'+secrets.token_hex(16);context=s.context
            binding={k:copy.deepcopy(context[k]) for k in ('project','resource','actor','claim','window','generation','revision','operation_id')}
            binding.update(snapshot_id=snapshot,source_sha=context['commit'],databases=source['databases'],sources=source['sources'],
                           restore_inputs=source['restore_inputs'],full_catalog=source['full_catalog'])
            expected=physical|{'operation':context['operation_id'],'snapshot_id':snapshot}
            capture={k:expected[k] for k in ('snapshot_id','pod_uid','pvc_uid','pv_uid','container_id','restart_count')}
            intent={'resource_context':copy.deepcopy(context),'namespace_uid':s.namespace_uid,'kube_lock':copy.deepcopy(s.lock),
                    'cohort_binding':binding,'backup_capture':capture,'cohort_source':expected,
                    'cohort_admin_role':admin,'upload_workflow_sha':context['commit']}
            pending=True
            with story_media_backup.private_directory(root):
                self.journal=media_operation_journal.Journal(root/'backup-journal',intent)
            self._bound_journal=self.journal
            client=frontend_runtime_executor.KubectlTransport(self.journal)
            fence=resource_fencing.Fence(copy.deepcopy(context),s.fence.check,resource_fencing.KubeLedger(client),s.fence.on_uncertain)
            if fence.ledger.read()!=s.fence.ledger.read():raise BackupError('backup journal ledger differs')
            phase=media_backup_phase.BackupPhases(self.journal,fence,client,s.lock,s.namespace_uid)
            self.cohort=cohort_backup_phases.CohortPhases(phase,s.catalog,token)
            self._bound_cohort=self.cohort
            self.cohort.bind_catalog_cut(self.cut)
            row,revision=self._guard()
            if 'full_cohort_backup_route' in row:raise BackupError('backup route already exists')
            route=cohort_upload_runner.route_marker(binding,s.namespace_uid)
            self.journal.state['steps']['full-cohort-backup-route']={'status':'pending','route':route};self.journal.save()
            if self._guard()!=(row,revision):raise BackupError('backup route fence changed')
            row['full_cohort_backup_route']=route;row['step']='full-cohort-backup-route';fence.ledger.write(revision,row)
            self._guard();current,_=fence.ledger.read()
            if current.get('full_cohort_backup_route')!=route:raise BackupError('backup route changed')
            self.journal.state['steps']['full-cohort-backup-route']={'status':'done','route':route};self.journal.save()
            self._route=copy.deepcopy(route);self._guard()
            key=self._write_key(root)
            with story_media_backup.private_directory(root) as directory:
                os.mkdir('cohort-backup',0o700,dir_fd=directory);os.fsync(directory)
            if self._source(token)!=original:raise BackupError('backup source changed')
            bundle,final_binding,manifest=cohort_backup_operation.run(self.cohort,root/'cohort-backup',key,expected,admin,self._guard)
            if self._source(token)!=original:raise BackupError('backup source changed')
            planned=cohort_upload_phase.prepare(self.cohort,key,bundle)
            def source_authority(sha):
                if sha!=context['commit']:return False
                publish_candidate.source_snapshot(self.metadata.source_root,sha);self._guard();return True
            submitted=cohort_dispatch.submit(self.cohort,planned,self._guard,source_authority)
            running=self._wait(token,planned,submitted['run_id_hint'])
            destination=root/('cohort-readback-'+running['run_id']+'-'+planned['challenge'])
            custody,restored_manifest=cohort_readback.run(self.cohort,token,key,bundle,destination,self._guard)
            if restored_manifest!=manifest or self._source(token)!=original:raise BackupError('readback or source changed')
            receipt=cohort_restore_journal.run(self.cohort,key,destination,restored_manifest,source['metadata'],
                self.metadata.source_root,source['restore_inputs']['postgres_image'],source['restore_inputs']['migrate_image'],
                self._guard,metadata_phase=self.metadata if source['migration_roles'] else None,
                metadata_token=token if source['migration_roles'] else None)
            if self._source(token)!=original:raise BackupError('backup source changed')
            return verify_completed(self,token)
        except BaseException:
            if pending:
                if self.cohort is not None:self.cohort.phases.poisoned=True
                try:s.fence.uncertain('full cohort backup outcome uncertain')
                except BaseException:pass
            raise BackupError('full cohort backup incomplete; retain lock and reconcile' if pending else 'full cohort backup not admitted') from None


def verify_completed(phase, token):
    """Verify a completed private backup before the first post-backup effect."""
    try:
        if type(phase) is not BackupPhase or type(phase.cohort) is not cohort_backup_phases.CohortPhases:
            raise ValueError()
        phase._guard();cohort=phase.cohort;cohort._guard();p=cohort.phases
        if p.journal is not phase.journal or cohort.catalog_phase is not phase.supervisor.catalog:
            raise ValueError()
        source=full_cohort_backup_metadata.verify_completed(phase.metadata,token)
        row,revision=p.fence.ledger.read();row=copy.deepcopy(row);snapshot=copy.deepcopy(cohort_readback._snapshot(cohort,row))
        binding,planned,marker,_=snapshot
        if (binding['operation_id']!=source['operation_id'] or binding['source_sha']!=source['source_sha']
                or binding['databases']!=source['databases'] or binding['sources']!=source['sources']
                or binding['restore_inputs']!=source['restore_inputs'] or binding['full_catalog']!=source['full_catalog']
                or row.get('full_cohort_backup_route')!=cohort_upload_runner.route_marker(cohort.binding,phase.supervisor.namespace_uid)):
            raise ValueError()
        readback=row.get('cohort_readback');done=row.get('cohort_restore');resources=row.get('cohort_restore_resources')
        if (not isinstance(readback,dict) or readback.get('status')!='done'
                or readback!=p.journal.state['steps'].get('cohort-readback') or not isinstance(done,dict)
                or done.get('status')!='done' or done!=p.journal.state['steps'].get('cohort-restore')):
            raise ValueError()
        receipt=done['receipt'];changed=set(source['migration_roles'])
        ri=readback['intent'];custody=readback['receipt'];intent=done['intent']
        expected_intent={'status':'pending','operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                         'invocation_nonce':p.invocation,'manifest_sha256':marker['manifest_sha256'],
                         'artifact_id':custody['artifact_id']}
        if changed:
            raw=json.dumps(source['migration_roles'],sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')
            expected_intent['migration_roles_sha256']=hashlib.sha256(raw).hexdigest()
        artifact=ri['binding']
        expected_custody={'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot',
                          'artifact_id':ri['artifact_id'],**{k:v for k,v in artifact.items() if k!='created_at'}}
        if (set(readback)!={'status','intent','receipt'} or set(done)!={'status','intent','receipt'}
                or set(ri)!={'status','artifact_id','binding','manifest_sha256','invocation_nonce'}
                or set(artifact)!={'operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256','run_id','head_sha','created_at'}
                or ri['status']!='pending' or ri['invocation_nonce']!=p.invocation
                or type(ri['artifact_id']) is not int or ri['artifact_id']<=0 or type(artifact['run_id']) is not int
                or not isinstance(artifact['created_at'],str) or not artifact['created_at']
                or ri['manifest_sha256']!=marker['manifest_sha256'] or custody!=expected_custody
                or any(artifact.get(k)!=planned[k] for k in ('operation_id','snapshot_id','challenge','bundle_bytes','bundle_sha256'))
                or artifact.get('head_sha')!=binding['source_sha'] or str(artifact.get('run_id'))!=planned['run_id']
                or intent!=expected_intent):raise ValueError()
        if (receipt.get('operation_id')!=binding['operation_id'] or receipt.get('snapshot_id')!=binding['snapshot_id']
                or receipt.get('source_sha')!=binding['source_sha'] or receipt.get('manifest_sha256')!=marker['manifest_sha256']
                or receipt.get('cleanup')!='verified' or set(receipt.get('databases',{}))!=set(binding['databases'])
                or not isinstance(resources,dict) or set(resources)!=({'postgres','migrate'} if changed else {'postgres'})):
            raise ValueError()
        expected_results={}
        for db in binding['databases']:
            plan=source['metadata']['migrations'][db]
            expected_results[db]={'mode':cohort_restore_journal.restore.classify_plan(db,plan,True),
                                  'from':plan['from'],'to':plan['to']}
        expected_receipt_keys={'operation_id','snapshot_id','source_sha','manifest_sha256','databases','scope','cleanup'}
        if changed:expected_receipt_keys.add('migration_logins')
        if (set(receipt)!=expected_receipt_keys or receipt['databases']!=expected_results
                or receipt['scope']!='selected-cohort-roles-catalog-and-ledger-only'):
            raise ValueError()
        local={name.removeprefix('cohort-restore-resource-'):value for name,value in p.journal.state['steps'].items()
               if name.startswith('cohort-restore-resource-')}
        if local!=resources:raise ValueError()
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
        if any(len({resource[key] for resource in resources.values()})!=len(resources) for key in ('nonce','name','container_id')):
            raise ValueError()
        for db in changed:cohort_restore_journal.verify_go_role(cohort,phase.metadata,token,db)
        saved=copy.deepcopy((readback,done,resources));phase._guard();cohort._guard();current,rev=p.fence.ledger.read()
        if (current!=row or rev!=revision or cohort_readback._snapshot(cohort,current)!=snapshot
                or (p.journal.state['steps'].get('cohort-readback'),p.journal.state['steps'].get('cohort-restore'),
                    {name.removeprefix('cohort-restore-resource-'):value for name,value in p.journal.state['steps'].items()
                     if name.startswith('cohort-restore-resource-')})!=saved):raise ValueError()
        return {'operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],'source_sha':binding['source_sha'],
                'manifest_sha256':marker['manifest_sha256'],'scope':'full-cohort-private-backup-and-isolated-restore'}
    except BaseException:
        raise BackupError('completed full cohort backup unavailable') from None
