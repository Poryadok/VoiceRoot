import unittest
from unittest.mock import patch
from pathlib import Path
from datetime import datetime, timedelta, timezone
import copy
import hashlib
import io
import json
import zipfile
import cohort_readback as readback
import cohort_upload_phase as upload
import cohort_upload_run as upload_run
import cohort_backup_phases as phases
import media_backup_phase as media
import media_backup_phase_test as phase_fixture
import media_operation_journal as journal
import story_media_backup_cohort_finalize_test as fixtures
import story_media_backup_cohort_finalize as finalize
import story_media_backup_cohort as cohort
import story_media_backup_cohort_custody as custody


class WorkflowTest(unittest.TestCase):
    def test_exact_run_and_artifact_required(self):
        planned = {'run_id':'123', 'source_sha':'a'*40, 'operation_id':'media-c2-'+'b'*32, 'challenge':'c'*32}
        run = {'id':123, 'head_sha':'a'*40, 'head_branch':'master', 'event':'workflow_dispatch',
               'path':readback.WORKFLOW, 'run_attempt':1, 'status':'completed', 'conclusion':'success',
               'created_at':'2026-10-28T21:00:00Z'}
        artifacts = {'total_count':1, 'artifacts':[{'id':456,
            'name':'voice-cohort-backup-'+planned['operation_id']+'-'+planned['challenge'],
            'created_at':'2026-10-28T21:00:00Z'}]}
        with patch.object(readback, 'get_json', side_effect=[run, artifacts]):
            self.assertEqual(readback.workflow_evidence('token', planned), (456, '2026-10-28T21:00:00Z'))
        for field, value in [('head_sha','f'*40), ('run_attempt',2), ('path','other'), ('conclusion','failure')]:
            with self.subTest(field=field), patch.object(readback, 'get_json', side_effect=[run | {field:value}, artifacts]):
                with self.assertRaises(readback.ReadbackError): readback.workflow_evidence('token', planned)


def cleanup(fixture):
    if not fixture.doCleanups(): raise RuntimeError('nested fixture cleanup failed')


class ReadbackTest(unittest.TestCase):
    def setUp(self):
        self.base = phase_fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(cleanup, self.base); self.base.setUp()
        template = fixtures.FinalizeCohortTest('test_exact_receipts_are_bound_without_mutating_inputs')
        self.addCleanup(cleanup, template); template.setUp()
        binding = copy.deepcopy(template.binding)
        binding.update(operation_id=self.base.context['operation_id'], source_sha=self.base.context['commit'],
                       snapshot_id=self.base.capture['snapshot_id'])
        for k in ('project','resource','actor','claim','window','generation','revision'): binding[k] = self.base.context[k]
        engine = binding['sources']['roles']['engine']
        capture = {'snapshot_id':binding['snapshot_id'], **{k:engine[k] for k in ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')}}
        intent = self.base.intent | {'namespace_uid':engine['namespace_uid'], 'backup_capture':capture,
                                    'cohort_binding':binding, 'upload_workflow_sha':binding['source_sha'], 'kube_lock':self.base.lock}
        self.root = Path(self.base.temp.name); op = self.root/binding['operation_id']; op.mkdir(mode=0o700)
        self.backup = op/'cohort-backup'; self.backup.mkdir(mode=0o700)
        self.journal = journal.Journal(op/'journal', intent); self.addCleanup(self.journal.close)
        client = phase_fixture.Client(self.base.lock); call = client.call
        def guarded_call(*args):
            result = call(*args)
            if args[1] == 'namespace': result['metadata']['uid'] = engine['namespace_uid']
            return result
        client.call = guarded_call
        self.media = media.BackupPhases(self.journal,self.base.fence,client,self.base.lock,engine['namespace_uid'])
        self.phase = phases.CohortPhases(self.media); receipts = {}
        for member in sorted(self.phase.members):
            nonce = self.phase.begin(member); source = binding['sources'][member]
            receipt = {'operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],'member':member,
                **{k:engine[k] for k in ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')},
                'server_start':source['server_start'],'database_oid':source.get('database_oid'),
                'attempt_nonce':nonce,'exit':0,'scope':'dump-terminal-only'}
            self.phase.complete(member,receipt); receipts[member] = receipt
        self.key = b'x'*32
        manifest = cohort.export(self.backup,self.key,binding,binding['databases'],lambda name,out:out.write(('synthetic-'+name).encode()))
        self.binding, self.manifest = finalize.finalize(self.backup,self.key,binding,binding['databases'],manifest,receipts)
        self.bundle = custody.build_bundle(self.backup,self.key,self.binding,binding['databases'],self.manifest)
        verified = custody.verify_bundle(self.bundle,self.key,self.binding,binding['databases'])
        marker = {'status':'done','operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
            'invocation_nonce':self.media.invocation,'bundle_bytes':verified['bytes'],'bundle_sha256':verified['sha256'],
            'manifest_sha256':hashlib.sha256(json.dumps(self.manifest,sort_keys=True,separators=(',',':')).encode()).hexdigest(),
            'receipt_sha256':self.binding['dump_receipts']}
        self.base.ledger.row['cohort_bundle'] = copy.deepcopy(marker)
        self.journal.state['steps']['cohort-bundle'] = {'status':'done','marker':copy.deepcopy(marker)}; self.journal.save()
        with patch.object(upload,'ROOT',self.root): self.plan = upload.prepare(self.phase,self.key,self.bundle)
        dispatch = upload_run.dispatch_marker(self.plan)
        self.journal.state['steps']['cohort-dispatch'] = copy.deepcopy(dispatch); self.journal.save()
        row, rev = self.base.ledger.read(); row['cohort_dispatch'] = dispatch
        row['cohort_upload'] = self.plan | {'status':'upload-running','run_id':'123','run_attempt':1}
        self.base.ledger.write(rev,row)
        self.destination = op/('cohort-readback-123-'+self.plan['challenge'])

    def extract(self):
        now = datetime.now(timezone.utc); created = (now-timedelta(seconds=1)).isoformat()
        wire = io.BytesIO()
        with zipfile.ZipFile(wire,'w',compression=zipfile.ZIP_STORED) as zipped: zipped.writestr('cohort-backup.tar',self.bundle.read_bytes())
        raw = wire.getvalue()
        meta = {'id':456,'url':custody.API+'456','expired':False,
            'name':'voice-cohort-backup-'+self.binding['operation_id']+'-'+self.plan['challenge'],
            'created_at':now.isoformat(),'expires_at':(now+timedelta(days=1)).isoformat(),
            'workflow_run':{'id':123,'head_sha':self.binding['source_sha']},'size_in_bytes':len(raw)}
        def download(url, headers, target, limit, deadline, redirects=False):
            data = raw if redirects else json.dumps(meta).encode(); target.write(data); return len(data)
        with patch.object(upload,'ROOT',self.root), patch.object(readback,'workflow_evidence',return_value=(456,created)), \
             patch.object(custody.transport,'_download',side_effect=download):
            return readback.run(self.phase,'fixture-token',self.key,self.bundle,self.destination,lambda:None)

    def test_real_encrypted_readback_and_no_replay(self):
        receipt, manifest = self.extract(); self.assertEqual(receipt['artifact_id'],456)
        for member in self.phase.members:
            output = io.BytesIO(); cohort.decrypt(self.destination,self.key,self.binding,self.binding['databases'],manifest,member,output)
            self.assertEqual(output.getvalue(),('synthetic-'+member).encode())
        with self.assertRaises(readback.ReadbackError): self.extract()

    def test_member_drift_rejected_before_network(self):
        self.base.ledger.row['cohort_members']['voice_db']['receipt_sha256'] = 'f'*64
        with patch.object(upload,'ROOT',self.root), patch.object(readback,'workflow_evidence') as network:
            with self.assertRaises(readback.ReadbackError):
                readback.run(self.phase,'fixture-token',self.key,self.bundle,self.destination,lambda:None)
        network.assert_not_called(); self.assertFalse(self.destination.exists())

    def test_dispatch_drift_rejected_before_network(self):
        self.base.ledger.row['cohort_dispatch']['challenge'] = 'f'*32
        with patch.object(upload,'ROOT',self.root), patch.object(readback,'workflow_evidence') as network:
            with self.assertRaises(readback.ReadbackError):
                readback.run(self.phase,'fixture-token',self.key,self.bundle,self.destination,lambda:None)
        network.assert_not_called(); self.assertFalse(self.destination.exists())

    def test_foreign_claim_rejected_before_network(self):
        value = '99999999-9999-9999-9999-999999999999'
        self.phase.binding['claim'] = value; self.media.intent['cohort_binding']['claim'] = value
        row, _ = self.base.ledger.read()
        with self.assertRaises(ValueError): readback._snapshot(self.phase, row)
        with patch.object(upload,'ROOT',self.root), patch.object(readback,'workflow_evidence') as network:
            with self.assertRaises(readback.ReadbackError):
                readback.run(self.phase,'fixture-token',self.key,self.bundle,self.destination,lambda:None)
        network.assert_not_called(); self.assertFalse(self.destination.exists())

    def test_lost_remote_response_poisoned_and_no_retry(self):
        with patch.object(upload,'ROOT',self.root), patch.object(readback,'workflow_evidence',return_value=(456,datetime.now(timezone.utc).isoformat())), \
             patch.object(custody,'readback_extract',side_effect=OSError('secret diagnostic')):
            with self.assertRaises(readback.ReadbackError):
                readback.run(self.phase,'fixture-token',self.key,self.bundle,self.destination,lambda:None)
        self.assertTrue(self.media.poisoned); self.assertEqual(self.base.ledger.row['status'],'uncertain')
        with self.assertRaises(readback.ReadbackError): self.extract()


if __name__ == '__main__': unittest.main()
