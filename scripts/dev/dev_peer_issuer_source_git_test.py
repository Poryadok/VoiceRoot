import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import dev_peer_issuer_source as reviewed


@unittest.skipUnless(os.name == 'posix' and shutil.which('git'), 'Linux Git fixture required')
class GitSourceTests(unittest.TestCase):
    def test_pinned_real_checkout_and_changed_root_auth_dependency(self):
        source = Path(os.environ['VOICE_TEST_SOURCE_ROOT'])
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            paths = tuple('scripts/dev/' + name for name in reviewed.FILES) + reviewed.ROOT_PATHS + (reviewed.SELF,)
            for name in paths:
                destination = root / name
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(source / name, destination)
            self_path = root / reviewed.SELF
            original = self_path.read_text()
            pinned = "REVIEWED_SOURCE='" + reviewed.REVIEWED_SOURCE + "'"
            self.assertEqual(original.count(pinned), 1)
            self_path.write_text(original.replace(pinned, 'REVIEWED_SOURCE=None'))
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.DEVNULL).decode().strip()
            git('init', '-q')
            git('config', 'user.email', 'fixture@example.invalid')
            git('config', 'user.name', 'Fixture')
            git('add', '.')
            git('commit', '-qm', 'reviewed')
            anchor = git('rev-parse', 'HEAD')
            self_path.write_text(original.replace(pinned, "REVIEWED_SOURCE='" + anchor + "'"))
            git('add', '.')
            git('commit', '-qm', 'pin')
            candidate = git('rev-parse', 'HEAD')
            with patch.object(reviewed, 'REVIEWED_SOURCE', anchor):
                reviewed.verify(str(root), candidate)
                changed = root / reviewed.ROOT_PATHS[0]
                changed.write_bytes(changed.read_bytes() + b'\n# changed\n')
                git('add', '.')
                git('commit', '-qm', 'changed auth')
                with self.assertRaises(reviewed.SourceError):
                    reviewed.verify(str(root), git('rev-parse', 'HEAD'))


if __name__ == '__main__':
    unittest.main()
