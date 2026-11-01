import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import local_ci_staging_request as request_contract
import local_ci_staging_worker as worker


class WorkerBindingTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        parent = Path(self.tmp.name)
        root = parent / 'source'
        root.mkdir()
        self.request = {'root': str(root), 'output': str(parent / 'output'),
                        'commit': 'a' * 40, 'tree': 'b' * 40, 'base': 'c' * 40,
                        'assets': {name: str(parent / name) for name in request_contract.ASSET_NAMES}}
        worker.configure(self.request)

    def test_request_sets_source_base_and_exact_asset_paths(self):
        self.assertEqual(worker.SOURCE_COMMIT, 'a' * 40)
        self.assertEqual(worker.SOURCE_TREE, 'b' * 40)
        self.assertEqual(worker.SOURCE_BASE, 'c' * 40)
        self.assertEqual([row[0] for row in worker.IMAGES],
                         [Path(self.request['assets'][key]) for key in
                          ('postgres_image', 'go_image', 'python_image', 'nats_image', 'nats_box_image', 'alpine_image')])

    def test_host_identity_and_cleanliness_fail_closed(self):
        good = [b'a' * 40 + b'\n', b'b' * 40 + b'\n', b'']
        with patch.object(worker, 'host_git', side_effect=good): worker.verify_host()
        for bad in ([b'd' * 40 + b'\n'], [good[0], b'd' * 40 + b'\n'], [good[0], good[1], b'?? foreign\n']):
            with self.subTest(bad=bad), patch.object(worker, 'host_git', side_effect=bad), self.assertRaises(RuntimeError):
                worker.verify_host()


if __name__ == '__main__':
    unittest.main()
