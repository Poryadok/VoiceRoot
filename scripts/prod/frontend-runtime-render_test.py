"""Exercise production renderer without invoking its deployment side effects."""

import os
from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
BASH = r"C:\Program Files\Git\bin\bash.exe" if os.name == "nt" else "bash"
SOURCE = (ROOT / "scripts/prod/apply-app-manifests.sh").read_text(encoding="utf-8")
RENDER_FUNCTION = "render() {" + SOURCE.split("render() {", 1)[1].split("\n}", 1)[0] + "\n}"


class FrontendRuntimeRenderTest(unittest.TestCase):
    def test_gateway_placeholder_is_resolved_for_both_apps(self):
        env = dict(os.environ, REGISTRY="registry.example", TAG="0" * 40,
                   VOICE_GATEWAY_INGRESS_HOST="gateway.example",
                   S3_SIGNING_ENDPOINT="https://gateway.example")
        for name in ("admin", "developer-portal"):
            with self.subTest(name=name):
                command = RENDER_FUNCTION + f"\nrender deploy/prod/{name}.yaml"
                result = subprocess.run([BASH, "-c", command], cwd=ROOT, env=env,
                                        capture_output=True, text=True, check=True)
                self.assertIn('"api_base_url":"https://gateway.example"', result.stdout)
                self.assertNotIn("__GATEWAY_INGRESS_HOST__", result.stdout)


if __name__ == "__main__":
    unittest.main()
