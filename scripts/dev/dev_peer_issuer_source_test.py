import unittest
from unittest.mock import patch

import dev_peer_issuer_source as reviewed


class SourceTests(unittest.TestCase):
    def run_case(self, drift=None):
        anchor, candidate = 'a' * 40, 'b' * 40
        def blob(repo, commit, path):
            if path == reviewed.SELF:
                return b'REVIEWED_SOURCE=None\n' if commit == anchor else ("REVIEWED_SOURCE='" + anchor + "'\n").encode()
            if path == drift and commit == candidate:
                return b'changed'
            return b'original'
        def disk(repo, path):
            return blob(repo, candidate, path)
        with patch.object(reviewed, 'REVIEWED_SOURCE', anchor), \
             patch.object(reviewed.source, '_blob', side_effect=blob), \
             patch.object(reviewed.source, '_disk', side_effect=disk):
            return reviewed.verify('/fixed/repo', candidate)

    def test_pin_only_self_difference_allowed(self):
        self.assertIsNone(self.run_case())

    def test_changed_root_auth_dependencies_are_rejected(self):
        for path in reviewed.ROOT_PATHS:
            with self.subTest(path=path), self.assertRaises(reviewed.SourceError):
                self.run_case(path)


if __name__ == '__main__':
    unittest.main()
