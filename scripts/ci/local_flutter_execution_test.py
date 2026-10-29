"""A Flutter execution must account for its command results and source drift."""
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import local_flutter_execution as execution


class LocalFlutterExecutionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'repo'
        self.root.mkdir()
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')
        (self.root / 'src/frontend/test').mkdir(parents=True)
        (self.root / 'src/frontend/test/example_test.dart').write_text('test source')
        self.git('add', '.')
        self.git('commit', '-qm', 'test')
        self.head = self.git('rev-parse', 'HEAD').stdout.decode().strip()
        self.output = Path(self.tmp.name) / 'run'

    def git(self, *args):
        return subprocess.run(['git', '-C', str(self.root), *args], check=True, capture_output=True)

    def fake_flutter(self, command, cwd=None, stdout=None, stderr=None, **kwargs):
        if '--version' in command:
            return subprocess.CompletedProcess(command, 0, stdout=b'{"frameworkRevision":"test"}')
        if 'test' in command:
            path = str(Path(cwd) / 'test/example_test.dart')
            rows = [{'type':'start'}, {'type':'suite','suite':{'id':1,'path':path}},
                    {'type':'allSuites','count':1},
                    {'type':'testStart','test':{'id':2,'suiteID':1,'name':'example','metadata':{'skip':False}}},
                    {'type':'testDone','testID':2,'result':'success','skipped':False,'hidden':False},
                    {'type':'done','success':True}]
            stdout.write(('\n'.join(json.dumps(row) for row in rows) + '\n').encode())
        return subprocess.CompletedProcess(command, 0)

    def test_full_test_inventory_and_command_results_are_recorded(self):
        with mock.patch.object(execution, '_invoke', side_effect=self.fake_flutter):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['tests']['passed'], 1)
        self.assertEqual(result['tests']['skipped'], [])
        self.assertEqual(len(result['commands']), 4)
        self.assertEqual(result['status'], 'incomplete')
        self.assertTrue(result['unbound_inputs'])

    def test_failed_command_and_source_mutation_cannot_report_success(self):
        def fail(command, **kwargs):
            if 'analyze' in command:
                return subprocess.CompletedProcess(command, 2)
            return self.fake_flutter(command, **kwargs)
        with mock.patch.object(execution, '_invoke', side_effect=fail):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(result['commands'][-1]['exit_code'], 2)

    def test_source_change_during_command_is_rejected(self):
        def mutate(command, **kwargs):
            result = self.fake_flutter(command, **kwargs)
            if 'analyze' in command:
                (Path(kwargs['cwd']) / 'test/example_test.dart').write_text('changed')
            return result
        with mock.patch.object(execution, '_invoke', side_effect=mutate):
            result = execution.run(self.root, self.head, self.output, 'flutter')
        self.assertEqual(result['status'], 'failed')
        self.assertIn('snapshot changed', result['error'])


if __name__ == '__main__':
    unittest.main()
