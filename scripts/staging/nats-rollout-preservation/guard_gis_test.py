import copy
import datetime as dt
import hashlib
from pathlib import Path
import tempfile
import unittest

import guard
import guard_test
import resource_claim
import resource_claim_test


class GISBoundTargetTest(unittest.TestCase):
    def test_normalized_pair_requires_exact_resource_fence_target_binding(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory)
            file = source / 'deploy' / 'fixture.yaml'
            file.parent.mkdir()
            file.write_text('fixture')
            target = {'registry': 'example.invalid', 'tag': 'a' * 40, 'mode': 'images-only',
                'changed_services': ['gameintegration'], 'source_hashes': {'deploy/fixture.yaml': hashlib.sha256(file.read_bytes()).hexdigest()},
                'template_hashes': {'voice-gameintegration': 'a' * 64}, 'input_template_hashes': {'voice-gameintegration': 'a' * 64},
                'images': {'voice-gameintegration/gameintegration': 'example.invalid/gis@sha256:' + 'b' * 64},
                'manifest_sha256': 'c' * 64, 'migration_sha256': 'd' * 64, 'nonnats_sha256': 'e' * 64,
                'gis_existing_normalized_sha256': 'f' * 64}
            receipt = guard_test.receipt()
            receipt.update(schema='nats-rollout-preservation-v1', target=target,
                backup={'archive_sha256': '1' * 64, 'manifest_sha256': '2' * 64,
                        'census_sha256': '3' * 64, 'off_node_verified': True},
                expires_at=(dt.datetime.now(dt.timezone.utc) + dt.timedelta(hours=1)).isoformat())
            def check(value): return guard.validate(value, 'example.invalid', 'a' * 40, 'images-only', ['gameintegration'], source)
            with self.assertRaises(guard.Blocked): check(receipt)
            receipt['resource_fence'] = {'schema': 'voice-staging-resource-fence-v1',
                'context': resource_claim_test.context(), 'operation': receipt['operation'],
                'helper_sha256': 'b' * 64, 'target_sha256': resource_claim.digest(target)}
            self.assertEqual(check(receipt), receipt)
            changed = copy.deepcopy(receipt)
            changed['target']['gis_existing_normalized_sha256'] = '0' * 64
            with self.assertRaises(guard.Blocked): check(changed)
            changed['target']['gis_existing_normalized_sha256'] = 'invalid'
            changed['resource_fence']['target_sha256'] = resource_claim.digest(changed['target'])
            with self.assertRaises(guard.Blocked): check(changed)


if __name__ == '__main__':
    unittest.main()
