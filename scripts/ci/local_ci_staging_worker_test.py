import hashlib
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import local_ci_staging_request as request_contract
import local_ci_staging_worker as worker
import local_ci_staging_dispatch as dispatch
import local_ci_matrix_inventory as matrix


class WorkerBindingTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        parent = Path(self.tmp.name)
        root = parent / 'source'
        root.mkdir()
        self.request = {'root': str(root), 'output': str(parent / 'output'),
                        'commit': 'a' * 40, 'tree': 'b' * 40, 'base': 'c' * 40, 'shard': 'staging',
                        'assets': {name: str(parent / name) for name in request_contract.ASSET_NAMES}}
        worker.configure(self.request)

    def test_request_sets_source_base_and_exact_asset_paths(self):
        self.assertEqual(worker.SOURCE_COMMIT, 'a' * 40)
        self.assertEqual(worker.SOURCE_TREE, 'b' * 40)
        self.assertEqual(worker.SOURCE_BASE, 'c' * 40)
        self.assertEqual(worker.SHARD, 'staging')
        self.assertEqual([row[0] for row in worker.IMAGES],
                         [Path(self.request['assets'][key]) for key in
                          ('postgres_image', 'go_image', 'python_image', 'nats_image', 'nats_box_image', 'alpine_image')])

    def test_host_identity_and_cleanliness_fail_closed(self):
        good = [b'a' * 40 + b'\n', b'b' * 40 + b'\n', b'']
        with patch.object(worker, 'host_git', side_effect=good): worker.verify_host()
        for bad in ([b'd' * 40 + b'\n'], [good[0], b'd' * 40 + b'\n'], [good[0], good[1], b'?? foreign\n']):
            with self.subTest(bad=bad), patch.object(worker, 'host_git', side_effect=bad), self.assertRaises(RuntimeError):
                worker.verify_host()

    def test_current_pinned_matrix_generates_both_complete_dispatchers(self):
        root = Path(__file__).resolve().parents[2]
        inventory = matrix.build(root)
        for shard, count in (('staging', 34), ('proofs', 15)):
            with self.subTest(shard=shard):
                recipes = dispatch.inventory(inventory['shards'][shard], shard=shard)
                self.assertEqual(len(recipes), count)
                wrappers = dispatch.wrappers(recipes, shard=shard)
                self.assertEqual(set(wrappers), {'bash', 'python'})
                if shard == 'staging':
                    self.assertEqual({kind: hashlib.sha256(data).hexdigest() for kind, data in wrappers.items()}, {
                        'bash': 'ebf2214ac2a6aaccc3d7f99903d103ed8623b2c65a7342ce3130f49a985401d8',
                        'python': '3d0cdaba229063d5a482328c75d3d5e53fb8daefab3806bcd92ff811c799d09d'})


if __name__ == '__main__':
    unittest.main()
