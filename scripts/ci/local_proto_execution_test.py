import json
import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

import local_proto_execution as proto


class LocalProtoExecutionTests(unittest.TestCase):
    def test_component_evidence_rejects_tampered_logs_and_skips(self):
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp); base, head, candidate = 'a'*40, 'b'*40, 'c'*40
            source = {'head':head}
            package = 'voice/r23-contract-test'; test = 'TestR23GeneratedRuntimeConsumers/Java'
            events = [{'Action':'start','Package':package},{'Action':'run','Package':package,'Test':test},
                      {'Action':'pass','Package':package,'Test':test},{'Action':'pass','Package':package}]
            receipt = {'event_base':base,'base':base,'head':head,'merge':candidate,
                       'checkout':candidate,'parents':[base,head]}
            payloads = {'r23-contract.stdout':''.join(json.dumps(e)+'\n' for e in events).encode(),
                        'buf-pr-regressions.stderr':b'Ran 1 test in 1s\n\nOK\n',
                        'buf-pr-candidate.stdout':json.dumps(receipt).encode()}
            commands=[]
            for name in proto.COMPONENT_COMMANDS:
                row={'name':name,'exit_code':0}
                for suffix in ('stdout','stderr'):
                    raw=payloads.get(name+'.'+suffix,b''); (out/(name+'.'+suffix)).write_bytes(raw)
                    row[suffix+'_sha256']=hashlib.sha256(raw).hexdigest()
                commands.append(row)
            (out/'base-protos.tar').write_bytes(b'archive')
            report={'schema':'voice-local-protobuf-diagnostic-v1','status':'incomplete','source':source,
                    'base':base,'candidate':candidate,'commands':commands,
                    'base_archive_sha256':hashlib.sha256(b'archive').hexdigest(),
                    'r23':{'packages':[package],'passed':1,'skipped':[]},'pr_regressions':1,'pr_candidate':receipt}
            proto.verify_component_evidence(out, source, base, report)
            (out/'buf-pr-regressions.stderr').write_bytes(b'Ran 1 test in 1s\nOK (skipped=1)\n')
            with self.assertRaises(ValueError): proto.verify_component_evidence(out, source, base, report)
            commands[-2]['stderr_sha256'] = hashlib.sha256((out/'buf-pr-regressions.stderr').read_bytes()).hexdigest()
            with self.assertRaises(ValueError): proto.verify_component_evidence(out, source, base, report)
    def test_candidate_git_is_visible_to_buf_and_restored_on_failure(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp); snapshot = root/'snapshot'; snapshot.mkdir()
            private = root/'fixture.git'; private.mkdir(); (private/'marker').write_text('owned')
            env = {'GIT_DIR':str(private),'GIT_WORK_TREE':str(snapshot)}
            (root/'other.git').mkdir()
            with self.assertRaises(ValueError):
                proto.run_pr_candidate(['buf'], snapshot, dict(env, GIT_DIR=str(root/'other.git')))
            original = Path.is_symlink
            with mock.patch.object(Path, 'is_symlink', lambda path: True if path == private else original(path)):
                with self.assertRaises(ValueError): proto.run_pr_candidate(['buf'], snapshot, env)
            self.assertTrue((private/'marker').is_file())
            def fail(command, **kwargs):
                self.assertNotIn('GIT_DIR', kwargs['env'])
                self.assertNotIn('GIT_WORK_TREE', kwargs['env'])
                self.assertTrue((snapshot/'.git/marker').is_file())
                raise subprocess.TimeoutExpired(command, 1)
            with self.assertRaises(subprocess.TimeoutExpired):
                proto.run_pr_candidate(['buf'], snapshot, env, execute=fail)
            self.assertTrue((private/'marker').is_file())
            self.assertFalse((snapshot/'.git').exists())
            def uncertain(command, **kwargs):
                raise proto.supervision.SupervisionError('cleanup unverified')
            with self.assertRaises(proto.supervision.SupervisionError):
                proto.run_pr_candidate(['buf'], snapshot, env, execute=uncertain)
            self.assertTrue((snapshot/'.git/marker').is_file())
            self.assertFalse(private.exists())

    def test_pr_regressions_require_executed_non_skipped_tests(self):
        self.assertEqual(proto.pr_regression_count(b'....\n----------------------------------------------------------------------\nRan 4 tests in 1.0s\n\nOK\n', 0), 4)
        for raw, code in ((b'Ran 0 tests in 0s\nOK\n', 0), (b'Ran 4 tests in 1s\nOK (skipped=1)\n', 0), (b'Ran 4 tests in 1s\nOK\n', 1)):
            with self.assertRaises(ValueError): proto.pr_regression_count(raw, code)

    def test_pr_receipt_binds_candidate_and_its_ordered_parents(self):
        base, head, candidate = 'a'*40, 'b'*40, 'c'*40
        receipt = {'event_base':base, 'base':base, 'head':head, 'merge':candidate,
                   'checkout':candidate, 'parents':[base,head]}
        proto.validate_pr_receipt(json.dumps(receipt).encode(), base, head, candidate)
        for key, value in (('base','d'*40),('head','e'*40),('parents',[head,base]),('checkout',head)):
            with self.assertRaises(ValueError):
                proto.validate_pr_receipt(json.dumps(dict(receipt, **{key:value})).encode(), base, head, candidate)

    def test_command_uses_job_broker_and_propagates_timeout(self):
        result = subprocess.CompletedProcess([], 0, b'\x00\xff', b'')
        with mock.patch.object(proto, '_windows', return_value=True), \
             mock.patch.object(proto.supervision, 'run_diagnostic', return_value=result) as broker:
            actual = proto._execute(['git', 'archive'], timeout=120)
        self.assertEqual(actual.stdout, b'\x00\xff')
        self.assertEqual(broker.call_args.kwargs['timeout'], 120)
        self.assertIn('--limit=16777216', broker.call_args.args[0])
        with mock.patch.object(proto, '_windows', return_value=True), \
             mock.patch.object(proto.supervision, 'run_diagnostic', side_effect=subprocess.TimeoutExpired('worker', 1)):
            with self.assertRaises(subprocess.TimeoutExpired):
                proto._execute(['git', 'archive'], timeout=1)

    def test_binary_archive_output_is_preserved_and_oversize_rejected(self):
        binary = 'import sys; sys.stdout.buffer.write(b"\\x00\\xff" * 700000)'
        result = proto._execute([sys.executable, '-c', binary], timeout=30)
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b'\x00\xff' * 700000)
        excess = 'import sys; sys.stdout.buffer.write(b"x" * 17000000)'
        result = proto._execute([sys.executable, '-c', excess], timeout=30)
        self.assertEqual(result.returncode, 125)
        self.assertLessEqual(len(result.stdout), 16777216)

    def test_private_maven_options_allow_spaces_but_reject_unsafe_characters(self):
        options = proto.maven_options(Path('directory with spaces') / 'repository')
        self.assertIn('directory with spaces', options)
        self.assertTrue(options.startswith('-Dmaven.repo.local="'))
        self.assertTrue(options.endswith('"'))
        with self.assertRaises(ValueError):
            proto.maven_options('invalid"path')
        with self.assertRaises(ValueError):
            proto.maven_options('invalid\npath')

    def test_java_consumer_requires_named_pass_and_no_skips(self):
        package = 'voice/r23-contract-test'
        name = 'TestR23GeneratedRuntimeConsumers/Java'
        def stream(action):
            events = [{'Action': 'start', 'Package': package},
                      {'Action': 'run', 'Package': package, 'Test': name},
                      {'Action': action, 'Package': package, 'Test': name},
                      {'Action': 'pass', 'Package': package}]
            return ''.join(json.dumps(item) + '\n' for item in events).encode()
        self.assertEqual(proto.r23_java_summary(stream('pass'), 0)['skipped'], [])
        with self.assertRaises(ValueError):
            proto.r23_java_summary(stream('skip'), 0)
        with self.assertRaises(ValueError):
            proto.r23_java_summary(stream('pass').replace(name.encode(), b'Other'), 0)

    def test_r23_event_validation_records_skip_and_rejects_partial_or_failure(self):
        package = 'voice/r23-contract-test'
        events = [
            {'Action': 'start', 'Package': package},
            {'Action': 'run', 'Package': package, 'Test': 'TestGo'},
            {'Action': 'pass', 'Package': package, 'Test': 'TestGo'},
            {'Action': 'run', 'Package': package, 'Test': 'TestJava'},
            {'Action': 'skip', 'Package': package, 'Test': 'TestJava'},
            {'Action': 'pass', 'Package': package},
        ]
        raw = ''.join(json.dumps(item) + '\n' for item in events).encode()
        result = proto.r23_summary(raw, 0)
        self.assertEqual(result['passed'], 1)
        self.assertEqual(result['skipped'], [[package, 'TestJava']])
        with self.assertRaises(ValueError):
            proto.r23_summary(raw, 1)
        with self.assertRaises(ValueError):
            proto.r23_summary(raw.splitlines(keepends=True)[0], 0)
        with self.assertRaises(ValueError):
            proto.r23_summary(raw + b'{', 0)

    def test_baseline_archive_uses_the_immutable_commit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'repo'
            root.mkdir()
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()
            git('init', '-q')
            (root / 'protos').mkdir()
            file = root / 'protos' / 'buf.yaml'
            file.write_text('old\n', encoding='utf-8')
            git('add', 'protos')
            git('-c', 'user.name=CI', '-c', 'user.email=ci@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture')
            base = git('rev-parse', 'HEAD')
            file.write_text('new\n', encoding='utf-8')
            out = Path(directory) / 'baseline'
            proto.archive_baseline(root, base, out)
            self.assertEqual((out / 'protos/buf.yaml').read_text(encoding='utf-8'), 'old\n')
            with self.assertRaises(ValueError):
                proto.archive_baseline(root, 'not-a-commit', Path(directory) / 'invalid')


if __name__ == '__main__':
    unittest.main()
