import unittest
from errors import safe_error
from controller import Blocked

class SafeErrorTest(unittest.TestCase):
    def test_literal_guard_visible_but_arbitrary_message_and_key_private(self):
        self.assertEqual(safe_error(Blocked('rollout_ownership_changed')),'rollout_ownership_changed')
        for error in (Blocked('private_sentinel_never_render'),KeyError('private_sentinel_never_render'),
                      ValueError('private_sentinel_never_render'),RuntimeError('private_sentinel_never_render')):
            with self.subTest(kind=type(error).__name__):
                self.assertNotIn('private_sentinel',safe_error(error))

if __name__=='__main__':unittest.main()
