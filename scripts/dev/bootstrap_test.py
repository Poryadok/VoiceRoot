import json
import unittest
from unittest.mock import patch
import tempfile
from pathlib import Path

import bootstrap
import environment as dev


class FreshNamespaceTests(unittest.TestCase):
    def test_command_return_and_tls_kube_paths_recheck_authority(self):
        response = type('Response', (), {'returncode': 0, 'stdout': b'{}'})()
        with patch.object(bootstrap, 'command_guard', side_effect=[600, ValueError('changed after command')]), \
                patch.object(bootstrap, 'command_started', return_value=None), \
                patch.object(bootstrap.subprocess, 'run', return_value=response) as run:
            with self.assertRaisesRegex(ValueError, 'changed after command'):
                bootstrap.run('kubectl', 'apply', '-f', '-')
            self.assertEqual(run.call_count, 1)
        with patch.object(bootstrap, 'command_guard', side_effect=ValueError('unsafe TLS path')), \
                patch.object(bootstrap.reset_bootstrap.Kube, 'command') as command:
            with self.assertRaisesRegex(ValueError, 'unsafe TLS path'):
                bootstrap.GuardedKube().command(['create', '-f', '-'], body={'kind': 'Secret'})
            command.assert_not_called()

    def test_dynamic_command_rejects_replaced_namespace_before_submission(self):
        dynamic = {'new_namespace_uid': 'expected', 'operation': 'owned', 'host': {}, 'deadline': '2099-01-01T00:00:00+00:00'}
        response = type('Response', (), {'returncode': 0, 'stdout': b'{"metadata":{"uid":"foreign"}}'})()
        with patch.object(bootstrap, 'dynamic_storage', return_value=dynamic), \
                patch.object(bootstrap.reset_bootstrap, 'observe_host', return_value={}), \
                patch.object(bootstrap.reset_bootstrap, 'quiet_window'), \
                patch.object(bootstrap.subprocess, 'run', return_value=response) as run:
            with self.assertRaisesRegex(ValueError, 'namespace identity'):
                bootstrap.run('kubectl', 'apply', '-f', '-')
            self.assertEqual(run.call_count, 1)
            self.assertIn('get', run.call_args.args[0])

    def test_dynamic_apply_pins_tool_images_and_rejects_unmapped_mutable_images(self):
        obj = {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': dev.metadata('voice-dev-mail'),
               'spec': {'template': {'spec': {'containers': [{'image': 'node:24-alpine'}]}}}}
        immutable = 'node@sha256:' + 'a' * 64
        with patch.object(bootstrap, 'dynamic_storage', return_value={'tool_images': {'node:24-alpine': immutable}, 'image_references': [immutable]}), \
                patch.object(bootstrap, 'run') as run:
            bootstrap.apply([obj])
            submitted = json.loads(run.call_args.kwargs['body'])
            self.assertEqual(submitted['items'][0]['spec']['template']['spec']['containers'][0]['image'], immutable)
            self.assertEqual(obj['spec']['template']['spec']['containers'][0]['image'], 'node:24-alpine')
        with patch.object(bootstrap, 'dynamic_storage', return_value={'tool_images': {}, 'image_references': []}), \
                patch.object(bootstrap, 'run') as run:
            with self.assertRaises(ValueError):
                bootstrap.apply([obj])
            run.assert_not_called()

    def test_dynamic_foundation_cannot_apply_old_static_pvs(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            for name in ('prepared', 'secrets'):
                (directory / (name + '.json')).write_text('{"items":[]}')
            (directory / 'foundation.json').write_text(json.dumps({'items': dev.foundation('pmdebook')}))
            with patch.object(bootstrap, 'WORK', directory), \
                    patch.object(bootstrap, 'dynamic_storage', return_value={'retained': []}), \
                    patch.object(bootstrap, 'run') as run:
                with self.assertRaises(ValueError):
                    bootstrap.start()
                run.assert_not_called()

    def test_namespace_creation_is_exclusive_and_conflict_is_not_adopted(self):
        namespace = dev.foundation('pmdebook')[0]
        with patch.object(bootstrap, 'run', side_effect=RuntimeError('AlreadyExists')) as run:
            with self.assertRaises(RuntimeError):
                bootstrap.create_namespace(namespace)
        args, kwargs = run.call_args
        self.assertEqual(args, ('kubectl', 'create', '-f', '-'))
        self.assertEqual(json.loads(kwargs['body']), namespace)
        self.assertEqual(run.call_count, 1)

    def test_foreign_namespace_is_rejected_before_write(self):
        with patch.object(bootstrap, 'run') as run:
            with self.assertRaises(ValueError):
                bootstrap.create_namespace({'metadata': {'name': 'voice-staging'}})
        run.assert_not_called()
