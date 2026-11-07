import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

import dev_peer_issuer_material as issuer
import dev_peer_issuer_packet as packet
import reset_credentials as reset


@unittest.skipUnless(os.name == 'posix' and os.geteuid() == 0, 'isolated Linux root fixture required')
class MaterialTests(unittest.TestCase):
    def test_fixed_role_and_space_leaves_use_retained_ca(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            archive = root / 'archive'
            archive.mkdir(mode=0o700)
            source = reset.ResetMaterial(root)
            key, cert = source.ca('fixture')
            shutil.copyfile(key, archive / 'media-ca.key')
            shutil.copyfile(cert, archive / 'media-ca.crt')
            (archive / 'media-ca.key').chmod(0o600)
            media = cert.read_bytes()
            with patch.object(issuer.custody, 'ARCHIVE', archive), patch.object(issuer.custody, '_check_ancestors'):
                for profile, names in [('role', ['voice-role-media-client-tls', 'voice-role-principal-signing']),
                                       ('space', ['voice-space-media-server-tls'])]:
                    directory = root / profile
                    directory.mkdir(mode=0o700)
                    receipt = issuer.generate(directory, media, profile, names, 'media-c1-' + 'a' * 32,
                                              'b' * 40, 'ns', 'c' * 64)
                    envelope = json.loads((directory / 'objects.json').read_bytes())
                    objects = packet.validate(envelope, profile, 'media-c1-' + 'a' * 32, 'b' * 40,
                                              'ns', 'c' * 64, names, media)
                    self.assertEqual(set(objects), set(names))
                    self.assertEqual(receipt['object_names'], names)
                    self.assertFalse(receipt['runtime_qualified'])
            self.assertEqual((archive / 'media-ca.key').read_bytes(), key.read_bytes())


if __name__ == '__main__':
    unittest.main()
