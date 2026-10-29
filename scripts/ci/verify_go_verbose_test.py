import importlib.util
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('verify_go_verbose', ROOT / 'scripts/ci/verify_go_verbose.py')
verify = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verify)


class GoVerboseProofTests(unittest.TestCase):
    def test_requires_one_run_one_pass_and_package_pass(self):
        good = '=== RUN   TestTarget\n--- PASS: TestTarget (0.12s)\nPASS\nok  example/pkg 0.12s\n'
        verify.check(good, 'TestTarget')
        for bad in ('', 'testing: warning: no tests to run\nPASS\n',
                    '=== RUN   TestTarget\n--- SKIP: TestTarget (0.00s)\nPASS\n',
                    '=== RUN   TestOther\n--- PASS: TestOther (0.01s)\nPASS\n',
                    good + '=== RUN   TestTarget\n--- PASS: TestTarget (0.01s)\n',
                    good.replace('PASS\nok', 'FAIL\nFAIL'),
                    'PASS\n=== RUN   TestTarget\n--- PASS: TestTarget (0.01s)\n',
                    good.replace('--- PASS: TestTarget', '--- SKIP: TestOther\n--- PASS: TestTarget'),
                    good.replace('--- PASS: TestTarget', '    --- SKIP: TestTarget/sub\n--- PASS: TestTarget')):
            with self.subTest(log=bad), self.assertRaises(ValueError):
                verify.check(bad, 'TestTarget')

    def test_rejects_malformed_target_and_truncated_log(self):
        with self.assertRaises(ValueError):
            verify.check('=== RUN   TestTarget\n--- PASS: TestTarget (0.01s)\nPASS\n', 'TestTarget|TestOther')
        with self.assertRaises(ValueError):
            verify.check('=== RUN   TestTarget\n--- PASS: TestTarget (0.01s)\nPASS', 'TestTarget')


if __name__ == '__main__':
    unittest.main()
