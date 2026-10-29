import hashlib
import base64
import io
import os
import tarfile
import zlib
import re
import unittest
from unittest.mock import patch

import dev_peer_issuer_installer as installer


class InstallerTests(unittest.TestCase):
    def test_wrong_digest_never_touches_install_paths(self):
        with patch.object(installer.os, 'geteuid', return_value=0), \
             patch.object(installer.custody, '_check_ancestors') as custody:
            with self.assertRaises(ValueError):
                installer.install(b'archive', 'a' * 64)
            custody.assert_not_called()

    def test_symlink_is_rejected_before_filesystem_mutation(self):
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode='w') as archive:
            entry = tarfile.TarInfo(installer.authority.FILES[0])
            entry.type = tarfile.SYMTYPE
            entry.linkname = '/etc/shadow'
            archive.addfile(entry)
        raw = output.getvalue()
        with patch.object(installer.os, 'geteuid', return_value=0), \
             patch.object(installer.custody, '_check_ancestors') as custody:
            with self.assertRaises(ValueError):
                installer.install(raw, hashlib.sha256(raw).hexdigest())
            custody.assert_not_called()

    def test_unreviewed_source_cannot_build_bundle(self):
        with self.assertRaises(installer.reviewed.SourceError):
            installer.build('/not/a/checkout', 'a' * 40, '/tmp/not-created')

    def test_inline_bootstrap_binds_exact_digest_and_fixed_archive(self):
        script = installer.bootstrap('a' * 64)
        self.assertEqual(len(script.splitlines()), 1)
        encoded = re.search(r'b64decode\("([A-Za-z0-9+/=]+)"\)', script).group(1)
        program = zlib.decompress(base64.b64decode(encoded, validate=True)).decode()
        compile(program, '<bootstrap>', 'exec')
        self.assertIn("EXPECTED = '" + 'a' * 64 + "'", program)
        self.assertIn(installer.BOOTSTRAP_ARCHIVE, program)
        with self.assertRaises(ValueError):
            installer.bootstrap('bad')


if __name__ == '__main__':
    unittest.main()
