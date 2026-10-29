import os
from pathlib import Path
import unittest
from unittest import mock

import full_cohort_c1_activation as activation


@unittest.skipUnless(os.name=='posix','Linux private directory fixture')
class ProfileDirectoryTest(unittest.TestCase):
    def setUp(self):
        self.root=Path('/home/pmd/voice-dev-peer-issuer/preflight'); self.suffix='a'*32
        self.root.mkdir(parents=True,mode=0o700,exist_ok=True); (self.root/self.suffix).mkdir(mode=0o700)
        self.addCleanup(self.cleanup_dirs)
        self.supervisor=activation.supervisor_module.Supervisor.__new__(activation.supervisor_module.Supervisor)
        self.supervisor.context={'operation_id':'media-c1-'+self.suffix,'project':'voice','resource':'DEV'}; self.supervisor._guard=mock.Mock()

    def cleanup_dirs(self):
        parent=self.root/self.suffix
        for name in ('space','role'):
            path=parent/name
            if path.is_symlink():path.unlink()
            elif path.exists():path.rmdir()
        parent.rmdir()

    def test_two_fixed_directories_are_exclusive(self):
        activation._profile_directory(self.supervisor,'space'); activation._profile_directory(self.supervisor,'role')
        self.assertEqual((self.root/self.suffix/'space').stat().st_mode & 0o777,0o700)
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'space')
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'creator')
        self.supervisor.context['resource']='STAGING'
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'role')

    def test_existing_symlink_is_never_adopted(self):
        (self.root/self.suffix/'role').symlink_to('/tmp',target_is_directory=True)
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'role')
        self.assertTrue((self.root/self.suffix/'role').is_symlink())

    def test_lost_guard_retains_reserved_directory_for_reconciliation(self):
        self.supervisor._guard.side_effect=[None,RuntimeError('guard lost')]
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'space')
        self.assertTrue((self.root/self.suffix/'space').is_dir())


if __name__=='__main__': unittest.main()
