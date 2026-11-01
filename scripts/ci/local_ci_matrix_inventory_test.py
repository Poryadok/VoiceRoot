"""No-execution tests for the four-shard inventory."""
import tempfile
import ast
import hashlib
import unittest
from pathlib import Path

import local_ci_matrix_inventory as inventory


class MatrixInventoryTest(unittest.TestCase):
    def test_worker_dispatcher_pin_matches_actual_source(self):
        directory = Path(__file__).resolve().parent
        tree = ast.parse((directory / 'local_ci_staging_worker.py').read_bytes())
        pins = [node.value.value for node in ast.walk(tree) if isinstance(node, ast.Assign)
                and len(node.targets) == 1 and isinstance(node.targets[0], ast.Name)
                and node.targets[0].id == 'dispatch_sha' and isinstance(node.value, ast.Constant)]
        self.assertEqual(pins, [hashlib.sha256((directory / 'local_ci_staging_dispatch.py').read_bytes()).hexdigest()])

    def test_transitive_recipes_preserve_order_and_reject_missing_targets(self):
        text = 'shard: first second\n\techo shard\nfirst:\n\techo first\nsecond: first\n\techo second\n'
        rows = inventory.collect(text, 'shard')
        self.assertEqual([row['recipe'] for row in rows], ['echo first', 'echo second', 'echo shard'])
        with self.assertRaisesRegex(ValueError, 'missing'):
            inventory.collect('shard: missing\n', 'shard')

    def test_unresolved_or_cyclic_dependencies_refuse(self):
        for text in ('shard: $(UNKNOWN)\n', 'shard: next\nnext: shard\n'):
            with self.subTest(text=text), self.assertRaises(ValueError):
                inventory.collect(text, 'shard')

    def test_current_repository_inventory_contains_all_shards_and_hosted_steps(self):
        root = Path(__file__).resolve().parents[2]
        result = inventory.build(root)
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(set(result['shards']), {'policy', 'runtime', 'staging', 'proofs'})
        self.assertEqual({key: len(rows) for key, rows in result['shards'].items()},
                         inventory.SHARD_RECIPE_COUNTS)
        self.assertTrue(any('local_ci_staging_execution_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_staging_owned_tmp_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_nats_hosted_diagnostic_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_ci_matrix_inventory_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_auth_private_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_auth_full_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_auth_full_orchestration_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_go_full_integration_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_chat_full_integration_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertTrue(any('local_compose_bridge_test.py' in row['recipe'] for row in result['shards']['policy']))
        self.assertIn('Makefile, workflow and snapshot root may follow links', result['omitted'])
        self.assertIn('hosted-only input files are not hashed', result['omitted'])
        self.assertTrue(any('nats-jwt-leaf-hosted-proof.sh' in row['recipe'] for row in result['shards']['staging']))
        self.assertTrue(any('compiled_source_build_test.py' in row.get('command', '') for row in result['workflow_steps']['policy']))
        self.assertTrue(any('go test -count=1' in row.get('command', '') for row in result['workflow_steps']['staging']))
        self.assertTrue(all(row['status'] == 'not_run' for rows in result['shards'].values() for row in rows))
        for shard, rows in result['workflow_steps'].items():
            self.assertTrue(all(row['status'] == 'not_run' for row in rows))
            self.assertEqual(rows[0]['action'], 'actions/checkout@v5')
            self.assertTrue(any(row.get('action') == 'actions/setup-python@v6' for row in rows))
            self.assertTrue(any('pip install PyYAML==6.0.3' in row.get('command', '') for row in rows))
            self.assertTrue(any(row.get('action') == 'actions/setup-go@v5' for row in rows))
            self.assertEqual(len(rows), 7 if shard in ('policy', 'staging') else 6)
            base = next(row for row in rows if 'resolve-voice-r22-base.sh' in row.get('command', ''))
            self.assertIn('GITHUB_ENV', base['command'])

    def test_symlinked_recipe_source_is_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'source').mkdir()
            try:
                (root / 'source/link').symlink_to(root / 'outside')
            except OSError:
                self.skipTest('symlink creation unavailable')
            with self.assertRaisesRegex(ValueError, 'symlink'):
                inventory._recipe_source(root, 'source/link')

    def test_drift_refuses_without_execution(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / '.github/workflows').mkdir(parents=True)
            actual = Path(__file__).resolve().parents[2]
            make = (actual / 'Makefile').read_bytes()
            workflow = (actual / '.github/workflows/ci.yml').read_bytes()
            for changed_make, changed_workflow in ((make + b'\n# drift\n', workflow),
                                                   (make, workflow + b'\n# drift\n')):
                (root / 'Makefile').write_bytes(changed_make)
                (root / '.github/workflows/ci.yml').write_bytes(changed_workflow)
                with self.assertRaisesRegex(ValueError, 'inventory safety surface'):
                    inventory.build(root)
            (root / '.github/workflows/ci.yml').unlink()
            with self.assertRaisesRegex(ValueError, 'inventory safety surface'):
                inventory.build(root)


if __name__ == '__main__':
    unittest.main()
