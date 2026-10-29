import unittest

import dev_realtime_p3_issuer_metadata as metadata


class MetadataTests(unittest.TestCase):
    def test_fixed_seven_absent_or_exact_metadata(self):
        self.assertEqual(len(metadata.MEMBERS), 7)
        for (kind, name), keys in metadata.MEMBERS.items():
            with self.subTest(name=name):
                self.assertEqual(metadata._item(b'', kind, name), {'state': 'absent'})
                raw = ('uid-1\nrv-1\n\n' + ''.join(key + '\n' for key in sorted(keys))).encode()
                self.assertEqual(metadata._item(raw, kind, name),
                                 {'state': 'present', 'uid': 'uid-1', 'resourceVersion': 'rv-1', 'keys': sorted(keys)})
                with self.assertRaises(ValueError):
                    metadata._item(raw + b'extra\n', kind, name)

    def test_deleted_and_partial_materials_refused(self):
        kind, name = next(iter(metadata.MEMBERS))
        with self.assertRaises(ValueError):
            metadata._item(b'uid\nrv\ndeleting\n', kind, name)
        with self.assertRaises(ValueError):
            metadata._item(b'uid\nrv\n\n', kind, name)


if __name__ == '__main__':
    unittest.main()
