import json
import tempfile
import unittest
from pathlib import Path
import deployment_report as report
import bridge
import commands
import sys


class ReportTests(unittest.TestCase):
    def test_collect_cli_selects_read_only_collection_without_submit(self):
        import ast
        import os
        import bridge_client
        from unittest.mock import Mock,patch
        tree=ast.parse(Path(bridge_client.__file__).read_text())
        main=next(node for node in tree.body if isinstance(node,ast.If) and ast.unparse(node.test)=="__name__ == '__main__'")
        collect=next(node for node in main.body[0].body if isinstance(node,ast.If) and ast.unparse(node.test)=="sys.argv[1:] == ['collect-reports']")
        collector=Mock();submit=Mock(side_effect=AssertionError('collection submitted mutation'))
        with patch.object(sys,'argv',['bridge_client.py','collect-reports']),self.assertRaises(SystemExit) as stopped:
            exec(compile(ast.Module(body=[collect],type_ignores=[]),'<actual-collect-cli>','exec'),{'sys':sys,'os':os,'collect_all':collector,'submit':submit})
        self.assertEqual(stopped.exception.code,0)
        collector.assert_called_once_with(os.environ);submit.assert_not_called()

    def test_historical_export_keeps_private_evidence_bytes_and_unknown_cause(self):
        import contextlib
        import root_main
        import root_cli
        from unittest.mock import patch
        import os
        nonce='a'*64;operation='a'*12
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);installed=root/'installed';installed.mkdir(mode=0o700)
            for name in ('journal','responses'):(installed/name).mkdir(mode=0o700)
            base=root/('rollout-'+operation);base.mkdir(mode=0o700)
            request={'nonce':nonce,'action':'prepare','source_sha':'b'*40,'run_id':17,'mode':'full','changed_services':[]}
            journal=installed/'journal'/(nonce+'.json');journal.write_text(json.dumps({'request':request,'phase':'STARTED'}));journal.chmod(0o600)
            checkpoint=base/'checkpoint.json';checkpoint.write_text(json.dumps({'operation':operation,'status':'BLOCKED','phase':'FENCE','fence_status':'VERIFIED','error':'command_failed'}));checkpoint.chmod(0o600)
            before={path:path.read_bytes() for path in (journal,checkpoint)}
            with patch.object(root_main,'operation_lock',contextlib.nullcontext),patch.object(root_cli,'ROOT',root),patch.object(root_cli,'code_binding',return_value={}),patch('grp.getgrnam',return_value=type('G',(),{'gr_gid':os.getgid()})()):
                row=report.export_historical(installed,nonce,operation)
                replay=report.export_historical(installed,nonce,operation)
                with self.assertRaises(ValueError):report.export_historical(installed,nonce,'c'*12)
            self.assertEqual({path:path.read_bytes() for path in before},before)
            self.assertEqual(row['operation_error'],'command_failed');self.assertEqual(row['error_code'],'unknown')
            self.assertEqual(replay['effects'],row['effects'])
            self.assertEqual((installed/'responses'/(nonce+'.report.json')).stat().st_mode&0o777,0o440)

    def test_noisy_stderr_does_not_change_success(self):
        self.assertEqual(commands.capture([sys.executable,'-c','import sys;sys.stderr.write("x"*100000);print("ok")']),b'ok\n')
        self.assertEqual(commands.capture([sys.executable,'-c','import sys;sys.stderr.write("permissions violation");print("ok")']),b'ok\n')

    def test_artifact_directory_rejects_symlink_and_existing_files(self):
        import bridge_client
        from unittest.mock import patch
        row=report.build({'nonce':'a'*64,'action':'prepare','run_id':17,'run_attempt':1,'source_sha':'b'*40},{})
        env={'RUNNER_TEMP':'','GITHUB_RUN_ID':'17','GITHUB_RUN_ATTEMPT':'1','GITHUB_SHA':'b'*40}
        with tempfile.TemporaryDirectory() as d,patch.object(report,'read',return_value=row):
            env['RUNNER_TEMP']=d
            target=bridge_client.output_directory(env)
            link=Path(d)/'link';link.symlink_to(target,target_is_directory=True)
            with self.assertRaises(ValueError):bridge_client.collect('a'*64,env,target=link)
            dest=target/('a'*64+'.deployment-report.json');dest.write_text('SECRET')
            with self.assertRaises(FileExistsError):bridge_client.collect('a'*64,env,target=target)
            fresh=bridge_client.output_directory(env)
            bridge_client.collect('a'*64,env,target=fresh)
            self.assertNotIn('SECRET',next(fresh.iterdir()).read_text())
    def test_command_failure_survives_cleanup_and_reports_exit_code(self):
        try:commands.capture([sys.executable,'-c','import sys;sys.stderr.write("SECRET");sys.exit(23)'])
        except Exception as error:
            commands.capture([sys.executable,'-c','pass'])
            row=report.build({'nonce':'a'*64,'action':'prepare'},{},error=error,current_action='kubectl.get')
        self.assertEqual(row['command']['exit_code'],23)
        self.assertEqual(row['current_action'],'command')
        self.assertNotIn('SECRET',json.dumps(row))

    def test_authorization_failure_and_timeout_are_distinct(self):
        cases=[('import sys;sys.stderr.write("Permissions Violation SECRET");sys.exit(1)',2,'application_authorization',False),
               ('import time;time.sleep(5)',0.05,'timeout',True)]
        for script,timeout,cause,timed_out in cases:
            with self.assertRaises(Exception) as caught:commands.capture([sys.executable,'-c',script],timeout=timeout)
            row=report.build({'nonce':'a'*64,'action':'prepare'},{},error=caught.exception)
            self.assertEqual(row['command']['cause'],cause)
            self.assertEqual(row['command']['timeout'],timed_out)

    def test_os_permission_failure_is_not_broker_authorization(self):
        from unittest.mock import patch
        with patch.object(commands.subprocess,'Popen',side_effect=PermissionError('SECRET')):
            with self.assertRaises(Exception) as caught:commands.capture(['kubectl','get','pods'])
        row=report.build({'nonce':'a'*64,'action':'prepare'},{},error=caught.exception)
        self.assertEqual(row['command']['cause'],'os_permission_denied')
        self.assertEqual(row['current_action'],'kubectl.get')

    def test_client_blocked_response_preserves_only_safe_details(self):
        import bridge_client
        import os
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)
            for name in ('inbox','responses'):(root/name).mkdir(mode=0o700)
            nonce='a'*64;path=root/'responses'/(nonce+'.json')
            path.write_text(json.dumps({'status':'BLOCKED','error':'command_failed','stdout':'SECRET'}));path.chmod(0o440)
            with self.assertRaises(bridge.BridgeError) as caught:bridge_client.submit({'nonce':nonce},root,timeout=1)
            self.assertEqual(caught.exception.safe_result,{'status':'BLOCKED','error':'command_failed'})

    def test_workflow_collection_is_always_and_does_not_mask_result(self):
        import yaml
        root=Path(__file__).resolve().parents[3]
        workflow=yaml.safe_load((root/'.github/workflows/staging-deploy.yml').read_text())
        steps=next(job['steps'] for job in workflow['jobs'].values() if any(step.get('id')=='preservation' for step in job.get('steps',[])))
        collection=next(step for step in steps if step.get('env',{}).get('ROLLOUT_ACTION')=='--bridge-collect')
        artifact=next(step for step in steps if step.get('with',{}).get('name','').startswith('deployment-report-'))
        for step in (collection,artifact):
            self.assertTrue(step['if'].startswith('always()'));self.assertIs(step['continue-on-error'],True)
        self.assertNotIn('GITHUB_TOKEN',collection.get('env',{}))

    def test_report_failure_does_not_replace_original_failure_or_result(self):
        from unittest.mock import Mock
        with tempfile.TemporaryDirectory() as d:
            request={'nonce':'a'*64,'action':'finish','operation':'b'*12,'claim_rv':'1'}
            failing=Mock(side_effect=OSError('SECRET'))
            def execute(row):raise ValueError('original')
            with self.assertRaisesRegex(ValueError,'original'):
                bridge.dispatch(d,request,execute,lambda r:None,reporter=failing)
        with tempfile.TemporaryDirectory() as d:
            self.assertEqual(bridge.dispatch(d,request,lambda r:{'status':'PASS'},lambda r:None,reporter=failing),{'status':'PASS'})

    def test_failure_before_operation_is_unknown_not_unchanged(self):
        row=report.build({'nonce':'a'*64,'action':'prepare','run_id':17,'run_attempt':2,'source_sha':'b'*40}, {}, error=PermissionError('SECRET'))
        self.assertIsNone(row['operation'])
        self.assertEqual(row['error_code'],'os_permission_denied')
        self.assertEqual(row['effects']['fence'],'UNKNOWN')
        self.assertNotIn('SECRET',json.dumps(row))

    def test_fenced_failure_keeps_request_and_operation_separate(self):
        state={'operation':'a'*12,'phase':'FENCE','status':'BLOCKED','fence_status':'VERIFIED',
               'events':[{'kind':'rollout_maintenance_owned','secret':'SECRET'}],'error':'SECRET'}
        row=report.build({'nonce':'b'*64,'action':'status'},state)
        self.assertEqual(row['request_status'],'COMPLETE')
        self.assertEqual(row['operation_status'],'BLOCKED')
        self.assertEqual(row['effects']['maintenance'],'RECORDED')
        self.assertEqual(row['operation_error'],'unknown')
        self.assertFalse(row['recovery_authorized'])
        self.assertNotIn('SECRET',json.dumps(row))

    def test_release_requires_explicit_checkpoint(self):
        state={'operation':'a'*12,'phase':'POST_RESTART','status':'PASS','fence_status':'RELEASED'}
        row=report.build({'nonce':'b'*64,'action':'finish'},state)
        self.assertEqual(row['effects']['release'],'RECORDED')
        self.assertEqual(row['operation_status'],'PASS')

    def test_schema_rejects_hostile_fields_and_identity_mismatch(self):
        row=report.build({'nonce':'a'*64,'action':'prepare','run_id':17,'source_sha':'b'*40},{})
        report.validate(row)
        with self.assertRaises(ValueError):report.validate(dict(row,stderr='SECRET'))
        with self.assertRaises(ValueError):report.validate(row,nonce='c'*64)
        with self.assertRaises(ValueError):report.validate(row,source_sha='c'*40)
        with self.assertRaises(ValueError):report.validate(row,run_id=18)

    def test_read_is_replayable_and_rejects_symlink_corruption_staleness(self):
        import os
        if os.name!='posix':self.skipTest('Linux custody fixture')
        row=report.build({'nonce':'a'*64,'action':'prepare','run_id':17,'source_sha':'b'*40},{})
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'report.json'
            report.persist(path,row,os.getgid())
            before=path.read_bytes()
            self.assertEqual(report.read(path,owner=os.geteuid(),trusted_root=d),row)
            self.assertEqual(report.read(path,owner=os.geteuid(),trusted_root=d),row)
            self.assertEqual(path.read_bytes(),before)
            with self.assertRaises(ValueError):report.read(path,owner=os.geteuid(),trusted_root=d,now=row['updated_at']+3601,max_age=3600)
            link=Path(d)/'link';link.symlink_to(path)
            with self.assertRaises(OSError):report.read(link,owner=os.geteuid(),trusted_root=d)
            path.chmod(0o600);path.write_text('{}');path.chmod(0o440)
            with self.assertRaises(ValueError):report.read(path,owner=os.geteuid(),trusted_root=d)
