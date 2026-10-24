"""Exercise the real ordered rollout against a recording kubectl, never a cluster."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
BASH = str(Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Git/bin/bash.exe") if os.name == "nt" else shutil.which("bash")


class UserSpaceRolloutTests(unittest.TestCase):
    def run_rollout(self, changed, fail_user=False):
        with tempfile.TemporaryDirectory(prefix="voice-subset-rollout-") as directory:
            root = Path(directory)
            staging = root / "scripts/staging"
            staging.mkdir(parents=True)
            for name in ("rollout-user-space-tier.sh", "rollout-helpers.sh", "deploy-changed.sh"):
                (staging / name).write_text((ROOT / "scripts/staging" / name).read_text(encoding="utf-8"), encoding="utf-8", newline="\n")
            trace = root / "trace"
            wrapper = root / "run.sh"
            wrapper.write_text('''#!/usr/bin/env bash
set -euo pipefail
kubectl() {
  printf '%s\\n' "$*" >> "$TRACE"
  if [[ "$*" == *jsonpath* ]]; then printf main; fi
  if [[ "$FAIL_USER" == 1 && "$*" == 'rollout status deployment/voice-user '* ]]; then return 1; fi
}
jq() { printf 'voice-%s\\n' "$4"; }
export -f kubectl jq
bash "$(dirname "$0")/scripts/staging/rollout-user-space-tier.sh"
''', encoding="utf-8", newline="\n")
            env = dict(os.environ, CHANGED_SERVICES=changed, VOICE_IMAGE_REGISTRY="fixture.invalid/voice", VOICE_IMAGE_TAG="exact-fixture", VOICE_K8S_NAMESPACE="fixture-only", NEEDS_USER_SPACE_ROLLOUT="false", TRACE=str(trace), FAIL_USER="1" if fail_user else "0", MSYS_NO_PATHCONV="1")
            result = subprocess.run([BASH, str(wrapper)], env=env, capture_output=True, text=True, timeout=20)
            calls = trace.read_text(encoding="utf-8").splitlines() if trace.exists() else []
            return result, calls

    def test_subset_never_scales_space_or_restarts_unchanged_space(self):
        for changed in ("user", "space", "user,space", "chat,user,space"):
            with self.subTest(changed=changed):
                result, calls = self.run_rollout(changed)
                self.assertEqual(0, result.returncode, result.stderr)
                self.assertFalse(any(call.startswith("scale ") for call in calls), calls)
                self.assertFalse(any(call.startswith("rollout restart deployment/voice-space") for call in calls), calls)
                images = [call for call in calls if call.startswith("set image ")]
                for name in changed.split(","):
                    self.assertEqual(1, sum(f"deployment/voice-{name} " in call for call in images), calls)
                if "space" not in changed.split(","):
                    self.assertFalse(any("deployment/voice-space " in call for call in images), calls)

    def test_user_readiness_precedes_changed_space_image(self):
        result, calls = self.run_rollout("user,space")
        self.assertEqual(0, result.returncode, result.stderr)
        patch = calls.index("set env deployment/voice-user -n fixture-only SPACE_GRPC_ADDR=")
        user_image = next(i for i, call in enumerate(calls) if call.startswith("set image deployment/voice-user "))
        user_ready = next(i for i, call in enumerate(calls) if call.startswith("rollout status deployment/voice-user "))
        space_image = next(i for i, call in enumerate(calls) if call.startswith("set image deployment/voice-space "))
        self.assertLess(user_image, patch)
        self.assertLess(patch, user_ready)
        self.assertLess(user_ready, space_image)
        restore = calls.index("set env deployment/voice-user -n fixture-only SPACE_GRPC_ADDR-")
        space_ready = next(i for i, call in enumerate(calls) if call.startswith("rollout status deployment/voice-space "))
        self.assertLess(space_ready, restore)

    def test_failed_user_readiness_stops_before_space_image(self):
        result, calls = self.run_rollout("user,space", fail_user=True)
        self.assertNotEqual(0, result.returncode)
        self.assertFalse(any(call.startswith("set image deployment/voice-space ") for call in calls), calls)
        self.assertNotIn("set env deployment/voice-user -n fixture-only SPACE_GRPC_ADDR-", calls)


if __name__ == "__main__":
    unittest.main()
