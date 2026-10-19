"""Failure-path custody checks; mocks only, no Docker or listeners."""
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import fence_probe


class NetworkCustodyTest(unittest.TestCase):
    def test_inspect_failure_keeps_exact_network_for_cleanup(self):
        nid = 'a' * 64
        cases = [RuntimeError('inspect unavailable'), '{',
                 json.dumps([{'Id': 'b' * 64, 'Internal': True}]),
                 json.dumps([{'Id': nid, 'Internal': False}])]
        for failure in cases:
            with self.subTest(failure=str(failure)), tempfile.TemporaryDirectory() as temp:
                output = Path(temp) / 'observation.json'
                calls = []

                def docker(*args, **kwargs):
                    calls.append(args)
                    if args[0] == 'info':
                        return json.dumps({'OSType': 'linux'})
                    if args[:2] == ('network', 'create'):
                        return nid
                    if args[:2] == ('network', 'inspect'):
                        # Custody must already survive a supervisor interruption.
                        saved = json.loads(output.read_text())
                        self.assertEqual(saved['network_births'][0]['network_uid'], nid)
                        if isinstance(failure, Exception):
                            raise failure
                        return failure
                    if args == ('network', 'rm', nid):
                        return nid
                    self.fail(f'unexpected Docker operation {args}')

                with patch.object(fence_probe, 'docker', side_effect=docker):
                    with self.assertRaises(Exception):
                        fence_probe.main(output)
                report = json.loads(output.read_text())
                self.assertIn(('network', 'rm', nid), calls)
                self.assertEqual(report['cleanup_failures'], [])
                self.assertIn('failure', report)
                self.assertNotIn('component_fence_observed', report)

    def test_cleanup_failure_retains_exact_network_and_failure(self):
        nid = 'c' * 64
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / 'observation.json'

            def docker(*args, **kwargs):
                if args[0] == 'info':
                    return json.dumps({'OSType': 'linux'})
                if args[:2] == ('network', 'create'):
                    return nid
                if args[:2] == ('network', 'inspect'):
                    raise RuntimeError('inspect unavailable')
                if args == ('network', 'rm', nid):
                    raise RuntimeError('owned network removal unavailable')
                self.fail(f'unexpected Docker operation {args}')

            with patch.object(fence_probe, 'docker', side_effect=docker):
                with self.assertRaisesRegex(RuntimeError, 'cleanup incomplete'):
                    fence_probe.main(output)
            report = json.loads(output.read_text())
            self.assertEqual(report['network_births'][0]['network_uid'], nid)
            self.assertEqual(report['failure'], 'inspect unavailable')
            self.assertEqual(report['cleanup_failures'], ['owned network removal unavailable'])

    def test_container_uid_is_saved_before_inspection(self):
        networks = ['d' * 64, 'e' * 64]
        cid = 'f' * 64
        calls, inspect_count = [], 0
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / 'observation.json'

            def docker(*args, **kwargs):
                nonlocal inspect_count
                calls.append(args)
                if args[0] == 'info':
                    return json.dumps({'OSType': 'linux'})
                if args[:2] == ('network', 'create'):
                    return networks[sum(x[:2] == ('network', 'create') for x in calls) - 1]
                if args[:2] == ('network', 'inspect'):
                    return json.dumps([{'Id': args[2], 'Internal': True}])
                if args[0] == 'create':
                    return cid
                if args[0] == 'inspect':
                    self.assertEqual(json.loads(output.read_text())['births'][0]['runtime_uid'], cid)
                    inspect_count += 1
                    if inspect_count == 1:
                        raise RuntimeError('birth inspection unavailable')
                    return json.dumps([{'Id': cid, 'State': {'Running': False}}])
                if args == ('rm', cid) or args[:2] == ('network', 'rm'):
                    return args[-1]
                self.fail(f'unexpected Docker operation {args}')

            with patch.object(fence_probe, 'docker', side_effect=docker), \
                    patch.object(fence_probe.subprocess, 'run', return_value=type('Logs', (), {'stdout': '', 'stderr': ''})()):
                with self.assertRaisesRegex(RuntimeError, 'birth inspection unavailable'):
                    fence_probe.main(output)
            self.assertIn(('rm', cid), calls)
            self.assertEqual(json.loads(output.read_text())['cleanup_failures'], [])


if __name__ == '__main__':
    unittest.main()
