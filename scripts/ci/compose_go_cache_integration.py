#!/usr/bin/env python3
"""Network-free proof that a warm Go build cache never replaces count=1 tests."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class HostGoCacheProof(unittest.TestCase):
    def test_warm_compilation_still_runs_tests_and_changed_source_fails(self):
        with tempfile.TemporaryDirectory(prefix="voice-go-cache-proof-") as temp:
            root = Path(temp)
            (root / "go.mod").write_text("module cacheproof\n\ngo 1.26\n", encoding="utf-8")
            source = root / "value.go"
            source.write_text('package cacheproof\nfunc Value() string { return "current" }\n',
                              encoding="utf-8")
            (root / "value_test.go").write_text(
                'package cacheproof\nimport "testing"\n'
                'func TestCurrent(t *testing.T) { t.Log("executed-current-test"); '
                'if Value() != "current" { t.Fatal("source-changed") } }\n', encoding="utf-8")
            env = dict(os.environ, GOTOOLCHAIN="local", GOPROXY="off", GOSUMDB="off",
                       GOWORK="off", GOCACHE=str(root / "build-cache"))

            def run():
                return subprocess.run(["go", "test", "-v", "-count=1", "."], cwd=root,
                                      env=env, capture_output=True, text=True, timeout=180)

            for _ in range(2):
                result = run()
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("executed-current-test", result.stdout)
                self.assertNotIn("(cached)", result.stdout)
            source.write_text('package cacheproof\nfunc Value() string { return "changed" }\n',
                              encoding="utf-8")
            result = run()
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("source-changed", result.stdout)


if __name__ == "__main__":
    unittest.main()
