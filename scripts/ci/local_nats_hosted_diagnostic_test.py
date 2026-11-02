"""Fail-closed result accounting for the two hosted NATS diagnostics."""
import tempfile
import unittest
import json
import io
import subprocess
import shutil
import os
import sys
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch

import local_nats_hosted_diagnostic as diagnostic


class NatsHostedDiagnosticTests(unittest.TestCase):
    def test_exact_inventory_and_success_without_skips(self):
        self.assertEqual(set(diagnostic.SCRIPTS), {
            'nats-jwt-leaf-hosted-proof.sh', 'nats-canonical-acl-hosted-proof.sh'})
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for script, marker in diagnostic.SCRIPTS.items():
                out, err = root / 'out', root / 'err'
                out.write_bytes((marker + '\n').encode())
                err.write_bytes(b'')
                result = diagnostic.validate_script_result(script, 0, out, err)
                self.assertEqual(result['status'], 'incomplete')
                self.assertEqual(result['script'], script)
                self.assertEqual(result['exit_code'], 0)

    def test_rejects_failure_wrong_marker_skips_and_unknown_script(self):
        with tempfile.TemporaryDirectory() as directory:
            out, err = Path(directory) / 'out', Path(directory) / 'err'
            script, marker = next(iter(diagnostic.SCRIPTS.items()))
            for code, stdout, stderr in ((1, marker, ''), (0, 'unrelated', ''),
                    (0, marker + '\nSKIP: missing', ''), (0, marker, 'SKIP: missing')):
                out.write_text(stdout, encoding='utf-8'); err.write_text(stderr, encoding='utf-8')
                with self.subTest(code=code, stdout=stdout, stderr=stderr), self.assertRaises(ValueError):
                    diagnostic.validate_script_result(script, code, out, err)
            with self.assertRaises(ValueError):
                diagnostic.validate_script_result('unknown.sh', 0, out, err)

    def test_rejects_oversized_logs_before_reading(self):
        with tempfile.TemporaryDirectory() as directory:
            out, err = Path(directory) / 'out', Path(directory) / 'err'
            out.write_bytes(b'x' * (diagnostic.MAX_LOG_BYTES + 1)); err.write_bytes(b'')
            with self.assertRaises(ValueError):
                diagnostic.validate_script_result(next(iter(diagnostic.SCRIPTS)), 0, out, err)

    def test_image_inventory_is_exact_and_refuses_duplicates_or_invalid_hashes(self):
        images = [(Path('/private') / str(i), str(i) * 64, 'sha256:' + str(i) * 64, tag)
                  for i, tag in enumerate(diagnostic.IMAGE_TAGS, 1)]
        self.assertEqual(len(diagnostic.validate_images(images)), 3)
        for value in (images[:-1], list(reversed(images)),
                      [images[0], images[0], images[2]],
                      [(images[0][0], 'bad', images[0][2], images[0][3]), *images[1:]],
                      [(images[0][0], images[0][1], images[1][2], images[0][3]), *images[1:]]):
            with self.subTest(value=value), self.assertRaises(ValueError):
                diagnostic.validate_images(value)

    def test_linux_refuses_before_creating_resources(self):
        with patch.object(diagnostic, '_windows', return_value=False), patch.object(diagnostic, 'DockerCLI') as engine:
            with self.assertRaisesRegex(ValueError, 'Windows'):
                diagnostic._run_owned(None, None, None, None, None, None, None, None, None, None, None)
            engine.assert_not_called()

    def test_setup_failure_closes_once_and_does_not_publish_report(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            copied = Path(directory) / 'copied'; copied.mkdir()
            output = Path(directory) / 'output'
            images = [(Path(directory) / str(i), str(i)*64, 'sha256:' + str(i)*64, tag)
                      for i, tag in enumerate(diagnostic.IMAGE_TAGS, 1)]
            source = {'head': 'a'*40}
            with patch.object(diagnostic, '_windows', return_value=True), \
                 patch.object(diagnostic.chat, '_verify_file'), \
                 patch.object(diagnostic.snapshot, 'verify_materialized'), \
                 patch.object(diagnostic.snapshot, 'capture', return_value=source), \
                 patch.object(diagnostic, 'DockerCLI') as engine, \
                 patch.object(diagnostic.ScheduledRun, 'start_host') as start:
                engine.return_value.daemon.return_value = 'daemon'
                engine.return_value._call.side_effect = lambda *a: a[2]
                start.return_value.slot = 0
                owner = start.return_value.run
                owner.directory = output / 'owner'
                owner.create_resource.side_effect = RuntimeError('setup failed')
                with self.assertRaisesRegex(RuntimeError, 'setup failed'):
                    diagnostic._run_owned(root, source, copied, {}, Path(directory)/'deps', output,
                                          Path(directory)/'helper', 'a'*64, Path(directory)/'jq', 'b'*64, images)
                start.return_value.close.assert_called_once_with()
                self.assertFalse((output / 'diagnostic.json').exists())

    def test_supervised_launcher_refuses_crash_timeout_and_partial_result(self):
        request = {key: None for key in diagnostic.REQUEST_KEYS}
        request['source'] = {'head': 'a'*40}
        complete = {'status': 'incomplete', 'source': request['source'],
                    'scripts': [{'status': 'incomplete', 'script': s, 'exit_code': 0} for s in diagnostic.SCRIPTS]}
        with patch.object(diagnostic.supervision, 'run_diagnostic') as broker:
            broker.return_value = subprocess.CompletedProcess([], 0, json.dumps(complete).encode(), b'')
            self.assertEqual(diagnostic.run(request, timeout=123), complete)
            self.assertEqual(broker.call_args.kwargs['timeout'], 123)
            self.assertEqual(json.loads(broker.call_args.kwargs['input']), request)
            for result in (subprocess.CompletedProcess([], 1, b'', b'crash'),
                           subprocess.CompletedProcess([], 0, b'{"status":"incomplete"}', b'')):
                broker.return_value = result
                with self.assertRaises(diagnostic.DiagnosticError):
                    diagnostic.run(request, timeout=123)
            broker.return_value = subprocess.CompletedProcess([], 7, b'', b'worker failure')
            with self.assertRaises(diagnostic.DiagnosticError) as failure:
                diagnostic.run(request, timeout=123)
            self.assertEqual(failure.exception.worker_returncode, 7)
            self.assertEqual(failure.exception.worker_stderr, b'worker failure')
            broker.side_effect = TimeoutError('timed out')
            with self.assertRaises(TimeoutError):
                diagnostic.run(request, timeout=123)

    def test_worker_ignores_python_environment_and_adjacent_stdlib_shadows(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            marker = root / 'injected'
            for name in ('hashlib.py', 'json.py', 'sitecustomize.py'):
                (root / name).write_text('open(' + repr(str(marker)) + ', "w").write("bad")\n', encoding='utf-8')
            worker = root / 'worker.py'
            worker.write_text('import hashlib, json\nprint(hashlib.sha256(b"ok").hexdigest())\n', encoding='utf-8')
            env = dict(os.environ, PYTHONPATH=str(root), PYTHONHOME=str(root))
            result = subprocess.run(diagnostic._worker_command(worker), cwd=root, env=env,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30, check=False)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse(marker.exists())

    def test_isolated_worker_imports_real_module_without_site_packages(self):
        result = subprocess.run(diagnostic._worker_command(Path(diagnostic.__file__)), input=b'{}',
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30, check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b'invalid diagnostic request fields', result.stderr)

    @unittest.skipUnless(os.name == 'nt', 'Windows optional stdlib import acceptance')
    def test_real_diagnostic_import_cannot_load_adjacent_pwd_or_grp(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = Path(diagnostic.__file__).resolve().parent
            for name in diagnostic.PROJECT_MODULES | {'local_nats_hosted_diagnostic'}:
                shutil.copyfile(source / (name + '.py'), root / (name + '.py'))
            marker = root / 'injected'
            for name in ('pwd.py', 'grp.py'):
                (root / name).write_text('open(' + repr(str(marker)) + ', "w").write("bad")\n', encoding='utf-8')
            env = dict(os.environ, PYTHONPATH=str(root), PYTHONHOME=str(root))
            result = subprocess.run(diagnostic._worker_command(root / 'local_nats_hosted_diagnostic.py'),
                                    cwd=root, env=env, input=b'{}', stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=30, check=False)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'invalid diagnostic request fields', result.stderr)
            self.assertFalse(marker.exists())

    @unittest.skipUnless(os.name == 'nt', 'Windows owned worker crash acceptance')
    def test_real_worker_crash_never_returns_a_report(self):
        request = {key: None for key in diagnostic.REQUEST_KEYS}
        broker = diagnostic.supervision.run_diagnostic
        def crash(_command, **kwargs):
            return broker([sys.executable, '-B', '-c', 'raise SystemExit(7)'], **kwargs)
        with patch.object(diagnostic.supervision, 'run_diagnostic', side_effect=crash):
            with self.assertRaises(diagnostic.DiagnosticError):
                diagnostic.run(request, timeout=30)

    def test_cli_accepts_exact_request_and_rejects_unknown_fields(self):
        request = {key: None for key in diagnostic.REQUEST_KEYS}
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'request.json'
            path.write_text(json.dumps(request), encoding='utf-8')
            with patch.object(diagnostic, 'run', return_value={'status': 'incomplete'}) as run, redirect_stdout(io.StringIO()):
                self.assertEqual(diagnostic.main([str(path)]), 0)
                run.assert_called_once_with(request)
            request['unexpected'] = True
            path.write_text(json.dumps(request), encoding='utf-8')
            with patch.object(diagnostic, 'run') as run, self.assertRaises(ValueError):
                diagnostic.main([str(path)])
            run.assert_not_called()
            del request['unexpected']
            path.write_text('{"root":null,' + json.dumps(request)[1:], encoding='utf-8')
            with patch.object(diagnostic, 'run') as run, self.assertRaises(ValueError):
                diagnostic.main([str(path)])
            run.assert_not_called()


if __name__ == '__main__':
    unittest.main()
