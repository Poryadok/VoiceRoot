"""Normal service umask must retain group-only scratch credential access."""
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import actor_auth
import bootstrap_auth


class RuntimeBoundary(Exception):
    pass


class ScratchInputTests(unittest.TestCase):
    def test_actual_helper_setup_preserves_private_modes_under_service_umask(self):
        for mask in (0o022, 0o077):
            for module in (actor_auth, bootstrap_auth):
                with self.subTest(mask=oct(mask), helper=module.__name__):
                    self.check_setup(module, mask)

    def check_setup(self, module, mask):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            binary = root / 'dummy-helper'
            binary.write_bytes(b'not executed')
            calls = []
            observed = []

            def boundary(base, operation):
                inputs = base / 'inputs'
                name = 'actor.creds' if module is actor_auth else 'bootstrap.creds'
                credential = inputs / name
                observed.append((inputs.stat().st_mode & 0o777,
                                 credential.stat().st_mode & 0o777))
                self.assertIn((inputs, 0, 65532), calls)
                self.assertIn((credential, 0, 65532), calls)
                raise RuntimeBoundary()

            previous = os.umask(mask)
            try:
                with patch.object(module.guard, 'ROOT', root), \
                        patch.object(module, 'DockerRuntime', boundary), \
                        patch.object(module, 'secret_bytes', return_value=b'public-fixture'), \
                        patch.object(module.os, 'chown', side_effect=lambda p, u, g: calls.append((p, u, g))):
                    with self.assertRaises(RuntimeBoundary):
                        if module is actor_auth:
                            module.prove(b'harmless-fixture', {}, binary, lambda: None)
                        else:
                            with patch.object(module, 'hash_bytes', return_value=module.ACCOUNT_SHA):
                                module.prove(b'harmless-fixture', {}, lambda: None)
                self.assertEqual(observed, [(0o750, 0o440)])
                self.assertEqual([p.name for p in root.iterdir()], ['dummy-helper'])
            finally:
                os.umask(previous)


if __name__ == '__main__':
    unittest.main()
