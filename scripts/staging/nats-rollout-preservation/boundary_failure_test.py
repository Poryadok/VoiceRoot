import contextlib
import io
import json
import os
from pathlib import Path
import runpy
import sys
import tempfile
import unittest
from unittest.mock import patch

import bridge_client
import workflow_entry
from controller import Blocked


class BoundaryFailureTests(unittest.TestCase):
    def environment(self, directory):
        return {'RUNNER_TEMP':directory,'GITHUB_RUN_ID':'17','GITHUB_RUN_ATTEMPT':'2',
                'GITHUB_SHA':'b'*40,'GITHUB_TOKEN':'SECRET_TOKEN',
                'GITHUB_STEP_SUMMARY':directory+'/summary','GITHUB_OUTPUT':directory+'/output'}

    def test_source_mismatch_is_visible_and_collectable_without_submission(self):
        with tempfile.TemporaryDirectory() as d:
            env=self.environment(d);stdout=io.StringIO();stderr=io.StringIO()
            with patch.dict(os.environ,env,clear=True),patch.object(sys,'argv',['bridge_client.py','prepare','images-only','voice','a'*40,'17']),patch.object(bridge_client,'submit',side_effect=AssertionError('submission forbidden')),contextlib.redirect_stdout(stdout),contextlib.redirect_stderr(stderr):
                with self.assertRaises(SystemExit) as exit:
                    runpy.run_path(bridge_client.__file__,run_name='__main__')
            self.assertEqual(exit.exception.code,1)
            self.assertIn('bridge_workflow_source_changed',stderr.getvalue())
            self.assertIn('bridge_workflow_source_changed',Path(env['GITHUB_STEP_SUMMARY']).read_text())
            bridge_client.collect_all(env)
            target=Path(Path(env['GITHUB_OUTPUT']).read_text().split('report_directory=')[-1].strip())
            rows=[json.loads(p.read_text()) for p in target.glob('*.deployment-report.json')]
            self.assertEqual(len(rows),1)
            self.assertEqual(rows[0]['schema'],'voice-deployment-boundary-failure-v1')
            self.assertEqual(rows[0]['error_code'],'bridge_workflow_source_changed')
            self.assertEqual(rows[0]['boundary'],'bridge-client')
            self.assertEqual(rows[0]['operation_status'],'UNKNOWN')
            self.assertFalse(rows[0]['recovery_authorized'])
            self.assertNotIn('SECRET',json.dumps(rows)+stdout.getvalue()+stderr.getvalue())

    def test_workflow_failure_preserves_allowlisted_code_and_failure_exit(self):
        with tempfile.TemporaryDirectory() as d:
            env=self.environment(d);stderr=io.StringIO()
            env['VOICE_NATS_ROLLOUT_OPERATION']='c'*12
            with patch.object(workflow_entry,'main',side_effect=Blocked('rollout_acl_proof_missing')),patch.dict(os.environ,env,clear=True),contextlib.redirect_stderr(stderr):
                self.assertEqual(workflow_entry.cli(['--preflight']),1)
            self.assertIn('rollout_acl_proof_missing',stderr.getvalue())
            bridge_client.collect_all(env)
            target=Path(Path(env['GITHUB_OUTPUT']).read_text().split('report_directory=')[-1].strip())
            row=json.loads(next(target.glob('*.deployment-report.json')).read_text())
            self.assertEqual(row['action'],'preflight')
            self.assertEqual(row['operation'],'c'*12)
            self.assertEqual(row['operation_status'],'UNKNOWN')

    def test_secret_unknown_errors_and_export_failure_never_replace_failure(self):
        with tempfile.TemporaryDirectory() as d:
            env=self.environment(d);stderr=io.StringIO()
            with patch.dict(os.environ,env,clear=True),patch.object(workflow_entry,'main',side_effect=Blocked('SECRET=token')),patch.object(bridge_client,'record_failure',side_effect=OSError('SECRET=path')),contextlib.redirect_stderr(stderr):
                self.assertEqual(workflow_entry.cli(['--apply']),1)
            self.assertNotIn('SECRET',stderr.getvalue())
            self.assertIn('exception_Blocked',stderr.getvalue())

    def test_failure_collection_rejects_foreign_run_and_hostile_fields(self):
        with tempfile.TemporaryDirectory() as d:
            env=self.environment(d)
            bridge_client.record_failure(Blocked('rollout_acl_proof_missing'),'workflow','preflight',env)
            index=next(Path(d).glob('*.failures'))
            row=json.loads(index.read_text());row['run_id']=18
            index.write_text(json.dumps(row)+'\n')
            with self.assertRaises(ValueError):bridge_client.collect_all(env)
            row['run_id']=17;row['stderr']='SECRET'
            index.write_text(json.dumps(row)+'\n')
            with self.assertRaises(ValueError):bridge_client.collect_all(env)

    def test_symlink_index_cannot_write_or_export_arbitrary_content(self):
        with tempfile.TemporaryDirectory() as d:
            env=self.environment(d);victim=Path(d)/'victim';victim.write_text('SECRET')
            index=Path(d)/'voice-deployment-17-2.failures';index.symlink_to(victim)
            with self.assertRaises(OSError):bridge_client.record_failure(Blocked('rollout_acl_proof_missing'),'workflow','preflight',env)
            self.assertEqual(victim.read_text(),'SECRET')
            with self.assertRaises((OSError,ValueError)):bridge_client.collect_all(env)

    def test_collection_is_read_only_replayable_and_success_emits_no_failure(self):
        with tempfile.TemporaryDirectory() as d:
            env=self.environment(d)
            with patch.dict(os.environ,env,clear=True),patch.object(workflow_entry,'main',return_value=None):
                self.assertEqual(workflow_entry.cli(['--preflight']),0)
            self.assertFalse(list(Path(d).glob('*.failures')))
            bridge_client.record_failure(Blocked('rollout_nonnats_live_object_identity_changed'),'workflow','apply',env)
            index=next(Path(d).glob('*.failures'));before=index.read_bytes()
            bridge_client.collect_all(env);bridge_client.collect_all(env)
            self.assertEqual(index.read_bytes(),before)
            self.assertEqual(len(list(Path(d).glob('voice-deployment-reports-*'))),2)
