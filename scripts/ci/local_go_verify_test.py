import json
import unittest

import local_go_verify


class GoVerifyTests(unittest.TestCase):
    def setUp(self):
        self.path = '/workspace/source/src/backend/pkg'
        self.module = {'Path': 'voice/backend/pkg', 'Version': 'v0.0.0', 'Dir': self.path,
                       'Replace': {'Path': '../pkg', 'Dir': self.path, 'GoMod': self.path + '/go.mod'}}
        self.error = b'voice/backend/pkg v0.0.0: missing ziphash: open hash: no such file or directory\n'

    def verify(self, code, out, error, module=None):
        return local_go_verify.verify_result(code, out, error, json.dumps(module or self.module), self.path)

    def test_normal_success_and_exact_local_replacement(self):
        self.verify(0, b'all modules verified\n', b'')
        self.verify(1, b'', self.error)

    def test_other_errors_and_additional_errors_are_rejected(self):
        for out, error in ((b'', self.error + b'external module modified\n'),
                           (b'', self.error.replace(b'voice/backend/pkg', b'example.com/other')),
                           (b'other output', self.error), (b'', b'')):
            with self.subTest(out=out, error=error), self.assertRaises(ValueError):
                self.verify(1, out, error)
        with self.assertRaises(ValueError):
            self.verify(0, b'all modules verified\n', b'unexpected error')

    def test_local_replacement_must_be_the_captured_pkg(self):
        for field, value in (('Path', 'other'), ('Dir', '/tmp/other'), ('GoMod', '/tmp/go.mod'), ('Version', 'v1.0.0')):
            module = json.loads(json.dumps(self.module))
            module['Replace'][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.verify(1, b'', self.error, module)

    def test_windows_exact_diagnostic_and_path(self):
        path = r'D:\source\src\backend\pkg'
        module = {'Path': 'voice/backend/pkg', 'Version': 'v0.0.0', 'Dir': path,
                  'Replace': {'Path': '../pkg', 'Dir': path, 'GoMod': path + r'\go.mod'}}
        error = b'voice/backend/pkg v0.0.0: missing ziphash: open hash: The system cannot find the file specified.\r\n'
        local_go_verify.verify_result(1, b'', error, json.dumps(module), path, windows=True)


if __name__ == '__main__':
    unittest.main()
