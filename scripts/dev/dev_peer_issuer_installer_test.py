import hashlib
import io
import os
import tarfile
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


if __name__ == '__main__':
    unittest.main()
