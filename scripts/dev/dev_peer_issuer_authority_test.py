import base64
import hashlib
import unittest
from unittest.mock import patch

import dev_peer_issuer_authority as authority


class AuthorityTests(unittest.TestCase):
    def test_fixed_candidate_bytes_and_both_master_checks(self):
        content = b'installed source bytes\n'
        oid = hashlib.sha1(b'blob ' + str(len(content)).encode() + b'\0' + content).hexdigest()
        heads = []
        def get(url, headers, deadline):
            if '/actions/runs/' in url:
                return {'actual': 'run'}
            name = url.split('/contents/', 1)[1].split('?ref=', 1)[0]
            return {'path': name, 'type': 'file', 'sha': oid, 'size': len(content),
                    'encoding': 'base64', 'content': base64.b64encode(content).decode()}
        with patch.object(authority.github, '_head', side_effect=lambda *args: heads.append(args[1])), \
             patch.object(authority.github, '_get', side_effect=get), \
             patch.object(authority.github, '_pages', return_value=['job']), \
             patch.object(authority.canonical, 'validate_run') as run, \
             patch.object(authority.canonical, 'validate_jobs') as jobs, \
             patch.object(authority, '_installed', return_value=content):
            authority.verify('token', 'a' * 40, 5, 2)
        self.assertEqual(heads, ['a' * 40, 'a' * 40])
        run.assert_called_once_with({'actual': 'run'}, 'a' * 40, 5, 2)
        jobs.assert_called_once_with(['job'], 'a' * 40, 5, 2)

    def test_changed_installed_bytes_rejected(self):
        content = b'candidate\n'
        oid = hashlib.sha1(b'blob ' + str(len(content)).encode() + b'\0' + content).hexdigest()
        def get(url, headers, deadline):
            if '/actions/runs/' in url:
                return {}
            name = url.split('/contents/', 1)[1].split('?ref=', 1)[0]
            return {'path': name, 'type': 'file', 'sha': oid, 'size': len(content),
                    'encoding': 'base64', 'content': base64.b64encode(content).decode()}
        with patch.object(authority.github, '_head'), patch.object(authority.github, '_get', side_effect=get), \
             patch.object(authority.github, '_pages', return_value=[]), \
             patch.object(authority.canonical, 'validate_run'), patch.object(authority.canonical, 'validate_jobs'), \
             patch.object(authority, '_installed', return_value=b'changed'):
            with self.assertRaises(ValueError):
                authority.verify('token', 'a' * 40, 5, 2)

    def test_bad_run_identity_rejected_before_network(self):
        with patch.object(authority.github, '_head') as head:
            with self.assertRaises(ValueError):
                authority.verify('token', 'bad', 5, 2)
            head.assert_not_called()


if __name__ == '__main__':
    unittest.main()
