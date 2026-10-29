import unittest
import os
from pathlib import Path
import tempfile
from unittest.mock import patch

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

    @unittest.skipUnless(os.name == 'posix', 'Linux FD fixture required')
    def test_capability_snapshot_never_reads_future_space_leaf(self):
        with tempfile.TemporaryDirectory() as temp:
            binary, config = Path(temp) / 'k3s', Path(temp) / 'config'
            binary.write_bytes(b'binary')
            config.write_bytes(b'config')
            def opened(path, limit):
                fd = os.open(path, os.O_RDONLY)
                item = os.fstat(fd)
                return fd, (item.st_dev, item.st_ino, item.st_size, item.st_mtime_ns, item.st_ctime_ns)
            with patch.object(metadata.base, 'K3S', binary), patch.object(metadata.base, 'KUBECONFIG', config), \
                 patch.object(metadata.base, '_open', side_effect=opened), \
                 patch.object(metadata.base, '_digest', return_value=metadata.base.K3S_SHA256), \
                 patch.object(metadata.base, 'observe'), patch.object(metadata, 'check_principal'), \
                 patch.object(metadata.base, '_read', return_value=b''), \
                 patch.object(metadata, '_cert', side_effect=lambda binary, config, name, key, expected: name.encode()) as cert:
                seven, roots = metadata.observe('ns', {}, {}, {}, {})
            self.assertEqual(len(seven), 7)
            self.assertTrue(all(item == {'state': 'absent'} for item in seven.values()))
            self.assertEqual(set(roots), {'principal', 'media'})
            self.assertEqual([call.args[2] for call in cert.call_args_list],
                             ['voice-principal-ca', 'voice-media-ca'] * 2)


if __name__ == '__main__':
    unittest.main()
