import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import local_preflight as preflight


class StagingIntegrationTests(unittest.TestCase):
    def test_missing_or_wrong_kubectl_bytes_fail_before_workers(self):
        for mode in ('missing', 'wrong'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / 'source'; root.mkdir()
                binary = Path(directory) / 'kubectl'
                if mode == 'wrong': binary.write_bytes(b'wrong binary')
                plan = {'checks': ['ci-script-tests'], 'source': {'head': 'a' * 40, 'tree': 'b' * 40}}
                with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(preflight.local_ci_staging_execution, 'run') as worker, \
                     patch.object(preflight.local_ci_routing, 'run') as routing:
                    result = preflight.run(root, 'c' * 40, 'a' * 40, Path(directory) / 'out',
                                           'flutter', staging_assets={'kubectl': str(binary)})
                self.assertEqual(result['status'], 'failed')
                worker.assert_not_called(); routing.assert_not_called()

    def test_missing_policy_kubectl_fails_before_workers(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'; root.mkdir()
            plan = {'checks': ['ci-script-tests'], 'source': {'head': 'a' * 40, 'tree': 'b' * 40}}
            with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                 patch.object(preflight.local_ci_staging_execution, 'run') as worker, \
                 patch.object(preflight.local_ci_routing, 'run') as routing:
                result = preflight.run(root, 'c' * 40, 'a' * 40, Path(directory) / 'out',
                                       'flutter', staging_assets={'helper': 'test'})
            self.assertEqual(result['status'], 'failed')
            self.assertIn('kubectl asset path', result['error'])
            worker.assert_not_called(); routing.assert_not_called()

    def test_assets_for_unselected_check_fail_before_any_worker(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'
            root.mkdir()
            plan = {'checks': [], 'source': {'head': 'a' * 40, 'tree': 'b' * 40}}
            with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                 patch.object(preflight.local_ci_staging_execution, 'run') as worker, \
                 patch.object(preflight.local_ci_routing, 'run') as routing:
                result = preflight.run(root, 'c' * 40, 'a' * 40, Path(directory) / 'out',
                                       'flutter', staging_assets={'helper': 'test', 'kubectl': str(Path(directory) / 'kubectl')})
            self.assertEqual(result['status'], 'failed')
            self.assertIn('require a planner-selected ci-script-tests check', result['error'])
            worker.assert_not_called()
            routing.assert_not_called()

    def test_selected_staging_receipt_stays_incomplete(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'
            root.mkdir()
            source = {'head': 'a' * 40, 'tree': 'b' * 40}
            plan = {'checks': ['ci-script-tests'], 'source': source}
            routing = {'schema': 'voice-local-ci-routing-diagnostic-v1', 'status': 'incomplete',
                       'source': source, 'commands': [{'name': 'gate-shell', 'tests': None},
                           {'name': 'gate', 'tests': len(preflight.local_ci_routing.GATE_CASES)},
                           {'name': 'router', 'tests': len(preflight.local_ci_routing.ROUTER_CASES)}]}
            receipt = {'schema': 'voice-local-ci-staging-v1', 'status': 'incomplete',
                       'source': source['head'], 'base': 'c' * 40, 'shard': 'staging', 'recipes': 34, 'hosted_go_tests': 3, 'hosted_policy_tests': 0}
            proofs = dict(receipt, shard='proofs', recipes=15, hosted_go_tests=0)
            policy = dict(proofs, shard='policy', recipes=69, hosted_policy_tests=1)
            with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                 patch.object(preflight.local_chat_orchestration, '_verify_file'), \
                 patch.object(preflight.local_ci_routing, 'run', return_value=routing), \
                 patch.object(preflight.local_ci_staging_execution, 'run', side_effect=[receipt, proofs, policy]) as run, \
                 patch.object(preflight.local_source_snapshot, 'capture', return_value=source):
                result = preflight.run(root, 'c' * 40, source['head'], Path(directory) / 'out',
                                       'flutter', staging_assets={'helper': 'test', 'kubectl': str(Path(directory) / 'kubectl')})
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['checks']['ci-script-tests']['status'], 'not_run')
            self.assertEqual([call.args[0]['shard'] for call in run.call_args_list], ['staging', 'proofs', 'policy'])
            self.assertTrue(all(call.args[0]['commit'] == source['head'] and call.args[0]['tree'] == source['tree'] for call in run.call_args_list))

    def test_runtime_follows_staging_and_proofs_with_explicit_images(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'; root.mkdir()
            source = {'head': 'a' * 40, 'tree': 'b' * 40}
            plan = {'checks': ['ci-script-tests'], 'source': source}
            routing = {'schema': 'voice-local-ci-routing-diagnostic-v1', 'status': 'incomplete', 'source': source,
                'commands': [{'name': 'gate-shell', 'tests': None},
                             {'name': 'gate', 'tests': len(preflight.local_ci_routing.GATE_CASES)},
                             {'name': 'router', 'tests': len(preflight.local_ci_routing.ROUTER_CASES)}]}
            common = {'schema': 'voice-local-ci-staging-v1', 'status': 'incomplete', 'source': source['head'], 'base': 'c' * 40}
            receipts = [dict(common, shard='staging', recipes=34, hosted_go_tests=1, hosted_policy_tests=0),
                        dict(common, shard='proofs', recipes=15, hosted_go_tests=0, hosted_policy_tests=0),
                        dict(common, shard='policy', recipes=69, hosted_go_tests=0, hosted_policy_tests=1),
                        dict(common, shard='runtime', recipes=23, hosted_go_tests=0, hosted_policy_tests=0)]
            images = {key: {'archive': str(Path(directory) / key), 'sha256': 'd' * 64,
                            'image_id': 'sha256:' + 'e' * 64} for key in ('postgres', 'docker_cli', 'python')}
            images['postgres']['image_id'] = 'sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'
            with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                 patch.object(preflight.local_chat_orchestration, '_verify_file'), \
                 patch.object(preflight.local_ci_routing, 'run', return_value=routing), \
                 patch.object(preflight.local_ci_staging_execution, 'run', side_effect=receipts) as run, \
                 patch.object(preflight.local_source_snapshot, 'capture', return_value=source):
                result = preflight.run(root, 'c' * 40, source['head'], Path(directory) / 'out',
                    'flutter', staging_assets={'helper': 'test', 'kubectl': str(Path(directory) / 'kubectl')}, runtime_images=images)
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['checks']['ci-script-tests']['status'], 'not_run')
            self.assertEqual([call.args[0]['shard'] for call in run.call_args_list], ['staging', 'proofs', 'policy', 'runtime'])
            self.assertEqual(run.call_args_list[-1].args[0]['runtime_images'], images)

    def test_changed_source_and_worker_failure_fail_preflight(self):
        for failure in ('drift', 'worker'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / 'source'
                root.mkdir()
                source = {'head': 'a' * 40, 'tree': 'b' * 40}
                plan = {'checks': ['ci-script-tests'], 'source': source}
                routing = {'schema': 'voice-local-ci-routing-diagnostic-v1', 'status': 'incomplete',
                    'source': source, 'commands': [{'name': 'gate-shell', 'tests': None},
                    {'name': 'gate', 'tests': len(preflight.local_ci_routing.GATE_CASES)},
                    {'name': 'router', 'tests': len(preflight.local_ci_routing.ROUTER_CASES)}]}
                receipt = {'schema': 'voice-local-ci-staging-v1', 'status': 'incomplete',
                           'source': source['head'], 'base': 'c' * 40, 'shard': 'staging', 'recipes': 34, 'hosted_go_tests': 3, 'hosted_policy_tests': 0}
                side_effect = preflight.local_ci_staging_execution.ExecutionError('worker failed') if failure == 'worker' else None
                with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(preflight.local_chat_orchestration, '_verify_file'), \
                     patch.object(preflight.local_ci_routing, 'run', return_value=routing), \
                     patch.object(preflight.local_ci_staging_execution, 'run', return_value=receipt, side_effect=side_effect), \
                     patch.object(preflight.local_source_snapshot, 'capture', return_value={'head': 'different'}):
                    result = preflight.run(root, 'c' * 40, source['head'], Path(directory) / 'out',
                                           'flutter', staging_assets={'helper': 'test', 'kubectl': str(Path(directory) / 'kubectl')})
                self.assertEqual(result['status'], 'failed')
                self.assertEqual(result['checks']['ci-script-tests']['status'], 'not_run')


if __name__ == '__main__':
    unittest.main()
