import json
import tempfile
from pathlib import Path
import unittest

import local_ci_staging_request as staging


class RequestTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        parent = Path(self.tmp.name)
        self.root = parent / 'source'
        self.root.mkdir()
        self.request = {
            'root': str(self.root), 'output': str(parent / 'output'),
            'commit': 'a' * 40, 'tree': 'b' * 40, 'base': 'c' * 40, 'shard': 'staging',
            'assets': {name: str(parent / name) for name in staging.ASSET_NAMES},
        }

    def test_accepts_explicit_full_request(self):
        self.assertEqual(staging.validate(self.request), self.request)

    def test_rejects_missing_extra_and_invalid_identity(self):
        for change in ({'commit': 'a' * 39}, {'base': 'X' * 40}, {'extra': 1}, {'tree': None}, {'shard': 'unknown'}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                staging.validate(dict(self.request, **change))
        with self.assertRaises(ValueError):
            staging.validate({k: v for k, v in self.request.items() if k != 'base'})

    def test_rejects_incomplete_assets_and_unsafe_output(self):
        bad = dict(self.request, assets={'helper': str(self.root / 'file')})
        with self.assertRaises(ValueError): staging.validate(bad)
        with self.assertRaises(ValueError): staging.validate(dict(self.request, output=str(self.root / 'output')))
        with self.assertRaises(ValueError): staging.validate(dict(self.request, output=str(self.root.parent / 'existing')) | {'assets': {**self.request['assets'], 'helper': 'relative'}})

    def test_parse_rejects_duplicate_keys_and_oversized_request(self):
        self.assertEqual(staging.parse(json.dumps(self.request).encode()), self.request)
        with self.assertRaises(ValueError): staging.parse(b'{"root":1,"root":2}')
        with self.assertRaises(ValueError): staging.parse(b' ' * (1024 * 1024 + 1))

    def test_assets_file_requires_exact_absolute_paths(self):
        assets = self.request['assets']
        self.assertEqual(staging.parse_assets(json.dumps(assets).encode()), assets)
        with self.assertRaises(ValueError): staging.parse_assets(b'{"helper":"x","helper":"y"}')
        with self.assertRaises(ValueError): staging.parse_assets(b'{}')
        with self.assertRaises(ValueError): staging.parse_assets(json.dumps(dict(assets, helper='relative')).encode())

    def test_runtime_requires_exact_pinned_image_records(self):
        parent = self.root.parent
        images = {name: {'archive': str(parent / name), 'sha256': 'd' * 64, 'image_id': 'sha256:' + 'e' * 64}
                  for name in ('postgres', 'docker_cli', 'python')}
        images['postgres']['image_id'] = 'sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'
        request = dict(self.request, shard='runtime', runtime_images=images)
        self.assertEqual(staging.validate(request), request)
        with self.assertRaises(ValueError): staging.validate(dict(self.request, shard='runtime'))
        for bad in (dict(images, postgres=dict(images['postgres'], image_id='sha256:' + 'f' * 64)),
                    dict(images, python=dict(images['python'], archive='relative')),
                    dict(images, docker_cli=dict(images['docker_cli'], sha256='x' * 64))):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                staging.validate(dict(request, runtime_images=bad))


if __name__ == '__main__':
    unittest.main()
