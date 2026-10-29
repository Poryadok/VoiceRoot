import unittest

import writer_cut


def job(name, uid, version='1'):
    return {'metadata': {'name': name, 'namespace': 'voice-dev', 'uid': uid,
                         'resourceVersion': version}, 'spec': {'parallelism': 1}, 'status': {}}


class MigrationInventoryTest(unittest.TestCase):
    def test_exact_owned_job_is_the_only_addition(self):
        initial = writer_cut.inventory([job('existing', 'old')])
        current = [job('existing', 'old'), job('migration', 'owned')]
        result = writer_cut.migration_job_inventory(initial, current, 'owned')
        self.assertEqual(result['metadata']['uid'], 'owned')
        for bad in (current + [job('foreign', 'other')], [job('existing', 'old', '2'), current[1]],
                    [current[0]], [current[0], job('migration', 'replacement')]):
            with self.subTest(bad=bad):
                with self.assertRaises(writer_cut.RecoveryRequired):
                    writer_cut.migration_job_inventory(initial, bad, 'owned')

    def test_absent_pending_never_adopts_by_name(self):
        initial = writer_cut.inventory([job('existing', 'old')])
        self.assertIsNone(writer_cut.migration_job_inventory(initial, [job('existing', 'old')], None))
        with self.assertRaises(writer_cut.RecoveryRequired):
            writer_cut.migration_job_inventory(initial, [job('existing', 'old'), job('migration', 'unknown')], None)


if __name__ == '__main__':
    unittest.main()
