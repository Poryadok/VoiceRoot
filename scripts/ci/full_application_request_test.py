import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('request', Path(__file__).with_name('full_application_request.py'))
request = importlib.util.module_from_spec(spec)
spec.loader.exec_module(request)


def row(cycle, identifier='a' * 32):
    return json.dumps({'schema': 'voice-full-application-build-request-v1',
                       'cycle': cycle, 'request_id': identifier}).encode()


class RequestTest(unittest.TestCase):
    def test_initial_and_retry_transitions(self):
        request.validate_transition(None, row(1))
        request.validate_transition(row(1), row(1, 'b' * 32))
        request.validate_transition(row(1), row(2, 'b' * 32))
        request.validate_transition(row(2), row(2, 'b' * 32))

    def test_invalid_transitions(self):
        for previous, current in ((None, row(2)), (row(1), row(1)),
                                  (row(1), row(2)), (row(2), row(1, 'b' * 32))):
            with self.subTest(previous=previous, current=current), self.assertRaises(ValueError):
                request.validate_transition(previous, current)

    def test_rejects_bad_schema_and_duplicate_fields(self):
        for raw in (b'{}', row(True), row(3), row(1, 'A' * 32),
                    b'{"cycle":1,"cycle":2}', row(1)[:-1] + b',"extra":1}'):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                request.validate_transition(None, raw)

    def test_real_first_parent_and_commit_binding(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            def git(*args):
                return subprocess.check_output(['git', '-C', temp, *args], stderr=subprocess.DEVNULL).decode().strip()
            git('init', '-q')
            (root / 'base').write_text('base')
            git('add', 'base')
            git('-c', 'user.name=Test', '-c', 'user.email=test@example.invalid', 'commit', '-qm', 'base')
            target = root / request.REQUEST_PATH
            target.parent.mkdir(parents=True)
            target.write_bytes(row(1))
            git('add', request.REQUEST_PATH)
            git('-c', 'user.name=Test', '-c', 'user.email=test@example.invalid', 'commit', '-qm', 'first')
            sha = git('rev-parse', 'HEAD')
            output = root / 'output'
            env = dict(os.environ, GITHUB_SHA=sha, GITHUB_OUTPUT=str(output))
            result = subprocess.run([sys.executable, str(Path(request.__file__))], cwd=temp, env=env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(output.read_text(), 'requested=true\n')
            env['GITHUB_SHA'] = 'f' * 40
            self.assertNotEqual(subprocess.run([sys.executable, str(Path(request.__file__))], cwd=temp, env=env, capture_output=True).returncode, 0)
            target.write_bytes(row(2, 'b' * 32))
            git('add', request.REQUEST_PATH)
            git('-c', 'user.name=Test', '-c', 'user.email=test@example.invalid', 'commit', '-qm', 'second')
            env['GITHUB_SHA'] = git('rev-parse', 'HEAD')
            self.assertEqual(subprocess.run([sys.executable, str(Path(request.__file__))], cwd=temp, env=env, capture_output=True).returncode, 0)

    def test_request_wiring_keeps_publication_and_deployment_distinct(self):
        repo = Path(__file__).resolve().parents[2]
        workflow = (repo / '.github/workflows/ci.yml').read_text(encoding='utf-8')
        changes = workflow.split('  changes:\n', 1)[1].split('  ci-skip-gate:\n', 1)[0]
        deploy = workflow.split('  deploy-staging:\n', 1)[1]
        self.assertIn("github.event_name == 'push' && github.ref == 'refs/heads/master' && steps.filter.outputs.full_application_request == 'true'", changes)
        for output in ('code:', 'FORCE_FULL:', 'FILTER_CODE:'):
            lines = [line for line in changes.splitlines() if line.strip().startswith(output)]
            self.assertEqual(len(lines), 3 if output == 'FORCE_FULL:' else 1)
            for line in lines:
                self.assertIn("steps.fullrequest.outputs.requested == 'true'", line)
        self.assertIn("needs.changes.outputs.full_application_request != 'true'", deploy)
        self.assertIn("vars.STAGING_DEPLOY_ENABLED == 'true'", deploy)


if __name__ == '__main__':
    unittest.main()
