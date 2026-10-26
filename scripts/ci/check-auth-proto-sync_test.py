"""Finite Auth mirror guard: missing and field-number drift must fail closed."""
import pathlib
import os
import shutil
import subprocess
import tempfile
import unittest


class AuthProtoSyncTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="voice-auth-proto-sync-")
        self.addCleanup(self.temporary.cleanup)
        self.root = pathlib.Path(self.temporary.name).resolve()
        self.checker = self.root / "scripts/ci/check-auth-proto-sync.sh"
        self.checker.parent.mkdir(parents=True)
        shutil.copyfile(pathlib.Path(__file__).with_name("check-auth-proto-sync.sh"), self.checker)
        self.copies = {}
        self.bash = os.environ.get("VOICE_BASH", "bash")
        if os.name == "nt" and "VOICE_BASH" not in os.environ:
            git_bash = pathlib.Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Git/bin/bash.exe"
            if git_bash.is_file():
                self.bash = str(git_bash)
        for name in ("auth.proto", "subscription_lifecycle.proto"):
            canonical = self.root / "protos/voice/auth/v1" / name
            copy = self.root / "src/backend/auth/src/main/proto/voice/auth/v1" / name
            canonical.parent.mkdir(parents=True, exist_ok=True)
            copy.parent.mkdir(parents=True, exist_ok=True)
            canonical.write_text('syntax = "proto3";\nmessage Wire { string id = 1; }\n', encoding="utf-8")
            copy.write_text('// fixture comment\nsyntax="proto3";\nmessage Wire{string id=1;}\n', encoding="utf-8")
            self.copies[name] = copy

    def run_guard(self):
        return subprocess.run([self.bash, self.checker.as_posix()], cwd=self.root,
                              capture_output=True, text=True, encoding="utf-8", timeout=10)

    def test_comments_and_spacing_preserve_both_wires(self):
        result = self.run_guard()
        self.assertEqual(0, result.returncode, result.stderr)

    def test_missing_new_mirror_is_not_ignored(self):
        self.copies["subscription_lifecycle.proto"].unlink()
        self.assertNotEqual(0, self.run_guard().returncode)

    def test_changed_field_number_is_rejected_for_each_mirror(self):
        for name in self.copies:
            with self.subTest(name=name):
                original = self.copies[name].read_text(encoding="utf-8")
                self.copies[name].write_text(original.replace("id=1", "id=2"), encoding="utf-8")
                self.assertNotEqual(0, self.run_guard().returncode)
                self.copies[name].write_text(original, encoding="utf-8")


if __name__ == "__main__":
    unittest.main()
