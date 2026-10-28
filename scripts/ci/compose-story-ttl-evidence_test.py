import importlib.util
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock


HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("story_ttl_win", HERE / "compose-story-ttl-delete-proof-win.py")
win = importlib.util.module_from_spec(spec)
spec.loader.exec_module(win)


class PrivateEvidenceTest(unittest.TestCase):
    def test_windows_runner_overrides_foreign_evidence_and_retains_on_failure(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            foreign = root / "foreign"
            foreign.mkdir()
            prior = foreign / "prior.jsonl"
            prior.write_text("foreign evidence")
            runs = root / "runs"
            fake_os = mock.Mock(wraps=os)
            fake_os.name = "nt"
            fake_os.environ = {"SPACE_NATIVE_EVIDENCE_DIR": str(foreign)}
            seen = []

            def call(args, **kwargs):
                if "--services" in args:
                    env = kwargs["env"]
                    evidence = Path(env["SPACE_NATIVE_EVIDENCE_DIR"])
                    self.assertEqual(evidence.parent.parent, runs)
                    self.assertTrue(evidence.is_dir())
                    self.assertEqual(list(evidence.iterdir()), [])
                    seen.append(evidence)
                    return "postgres"
                if "up" in args:
                    raise RuntimeError("injected startup failure")
                if "down" in args:
                    raise RuntimeError("injected cleanup failure")
                return ""

            # Compilation writes its output inside the new run directory.
            def compile_docker(*args, **kwargs):
                if args[0] == "run":
                    (next(runs.iterdir()) / "gateway.test").write_bytes(b"test")
                return ""

            with mock.patch.object(win, "RUNS", runs), mock.patch.object(win, "os", fake_os), \
                 mock.patch.object(win, "clean_head", return_value=("a" * 40, "b" * 40)), \
                 mock.patch.object(win, "require_frozen"), mock.patch.object(win, "free_ports"), \
                 mock.patch.object(win, "docker", side_effect=compile_docker), \
                 mock.patch.object(win, "call", side_effect=call):
                with self.assertRaisesRegex(RuntimeError, "injected cleanup failure"):
                    win.main()
            self.assertEqual(prior.read_text(), "foreign evidence")
            self.assertEqual(len(seen), 1)
            self.assertTrue(seen[0].exists())

    def test_bash_runner_uses_private_directory_and_retains_failure(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "scripts/ci").mkdir(parents=True)
            (root / "src/backend/gateway").mkdir(parents=True)
            shutil.copy(HERE / "compose-story-ttl-delete-proof.sh", root / "scripts/ci/run.sh")
            foreign = root / "foreign"
            foreign.mkdir()
            prior = foreign / "prior.jsonl"
            prior.write_text("foreign evidence")
            binaries = root / "bin"
            binaries.mkdir()
            scripts = {
                "git": "case \"$*\" in *rev-parse*) echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa;; esac\n",
                "go": "while [ $# -gt 0 ]; do if [ \"$1\" = -o ]; then shift; printf test > \"$1\"; break; fi; shift; done\n",
                "python3": "exit 0\n",
                "docker": "case \"$*\" in *'config --services'*) echo postgres; exit 0;; *'config --quiet'*) test -d \"$SPACE_NATIVE_EVIDENCE_DIR\" || exit 91; test -z \"$(ls -A \"$SPACE_NATIVE_EVIDENCE_DIR\")\" || exit 92; printf '%s' \"$SPACE_NATIVE_EVIDENCE_DIR\" > \"$TEST_RECEIPT\";; *'up -d --build'*) exit 70;; *'down --remove-orphans --volumes'*) exit 72;; esac\n",
            }
            for name, body in scripts.items():
                path = binaries / name
                path.write_text("#!/bin/sh\n" + body)
                path.chmod(0o755)
            receipt = root / "receipt"
            env = os.environ.copy()
            env.update(PATH=str(binaries) + os.pathsep + env["PATH"], TMPDIR=str(root),
                       SPACE_NATIVE_EVIDENCE_DIR=str(foreign), TEST_RECEIPT=str(receipt))
            env.pop("COMPOSE_PROJECT_NAME", None)
            result = subprocess.run(["bash", str(root / "scripts/ci/run.sh")], env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 1, result.stderr)
            evidence = Path(receipt.read_text())
            self.assertNotEqual(evidence, foreign)
            self.assertTrue(evidence.is_dir(), "failed run retains its private evidence directory")
            self.assertEqual(prior.read_text(), "foreign evidence")


if __name__ == "__main__":
    unittest.main()
