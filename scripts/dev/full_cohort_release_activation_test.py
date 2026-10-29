import unittest

import full_cohort_release_activation as activation


class GoAdmissionTest(unittest.TestCase):
    def test_untyped_backup_is_rejected_before_opening_a_phase(self):
        with self.assertRaises(activation.ActivationError):
            activation._run_go(None, 'fixture-token')


if __name__ == '__main__':
    unittest.main()
