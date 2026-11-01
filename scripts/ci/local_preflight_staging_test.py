import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import local_preflight as preflight


class StagingIntegrationTests(unittest.TestCase):
    def test_assets_for_unselected_check_fail_before_any_worker(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'
            root.mkdir()
            plan = {'checks': [], 'source': {'head': 'a' * 40, 'tree': 'b' * 40}}
            with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                 patch.object(preflight.local_ci_staging_execution, 'run') as worker, \
                 patch.object(preflight.local_ci_routing, 'run') as routing:
                result = preflight.run(root, 'c' * 40, 'a' * 40, Path(directory) / 'out',
                                       'flutter', staging_assets={'helper': 'test'})
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
                       'source': source['head'], 'base': 'c' * 40, 'shard': 'staging', 'recipes': 34, 'hosted_go_tests': 3}
            proofs = dict(receipt, shard='proofs', recipes=15, hosted_go_tests=0)
            with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                 patch.object(preflight.local_ci_routing, 'run', return_value=routing), \
                 patch.object(preflight.local_ci_staging_execution, 'run', side_effect=[receipt, proofs]) as run, \
                 patch.object(preflight.local_source_snapshot, 'capture', return_value=source):
                result = preflight.run(root, 'c' * 40, source['head'], Path(directory) / 'out',
                                       'flutter', staging_assets={'helper': 'test'})
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['checks']['ci-script-tests']['status'], 'not_run')
            self.assertEqual([call.args[0]['shard'] for call in run.call_args_list], ['staging', 'proofs'])
            self.assertTrue(all(call.args[0]['commit'] == source['head'] and call.args[0]['tree'] == source['tree'] for call in run.call_args_list))

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
                           'source': source['head'], 'base': 'c' * 40, 'shard': 'staging', 'recipes': 34, 'hosted_go_tests': 3}
                side_effect = preflight.local_ci_staging_execution.ExecutionError('worker failed') if failure == 'worker' else None
                with patch.object(preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(preflight.local_ci_routing, 'run', return_value=routing), \
                     patch.object(preflight.local_ci_staging_execution, 'run', return_value=receipt, side_effect=side_effect), \
                     patch.object(preflight.local_source_snapshot, 'capture', return_value={'head': 'different'}):
                    result = preflight.run(root, 'c' * 40, source['head'], Path(directory) / 'out',
                                           'flutter', staging_assets={'helper': 'test'})
                self.assertEqual(result['status'], 'failed')
                self.assertEqual(result['checks']['ci-script-tests']['status'], 'not_run')


if __name__ == '__main__':
    unittest.main()
