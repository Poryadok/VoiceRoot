"""Exercise the production image gate without a registry or cluster."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
REGISTRY = "ghcr.io/fixture/voice"
TAG = "a" * 40
CATALOG = ROOT / "scripts/ci/staging-image-catalog.json"
NAMES = [row["name"] for row in json.loads(CATALOG.read_text())["images"]
         if "prod" in row.get("environments", ["dev", "staging", "prod"])]
NAMES.append("nats-hub-config-renderer")


def lock(names=None):
    return (f"version: 1\nregistry: {REGISTRY}\ntag: {TAG}\nimages:\n"
            + "".join(f"  {name}: {TAG}\n" for name in (NAMES if names is None else names)))


class ProductionImageGateTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.directory = Path(self.tmp.name)
        self.calls = self.directory / "calls"
        fake = self.directory / "docker"
        fake.write_text("#!/usr/bin/env python3\nimport os,sys\n"
                        "with open(os.environ['IMAGE_CALLS'],'a') as out: out.write(sys.argv[-1]+'\\n')\n"
                        "sys.exit(1 if os.environ.get('MISSING_IMAGE')==sys.argv[-1] else 0)\n")
        fake.chmod(0o755)
        self.env = dict(os.environ, PATH=f"{self.directory}:{os.environ['PATH']}",
                        VOICE_IMAGE_REGISTRY=REGISTRY, VOICE_IMAGE_TAG=TAG,
                        IMAGE_CALLS=str(self.calls))
        # The old no-lock branch uses jq; keep its fixture deterministic too.
        jq = self.directory / "jq"
        jq.write_text("#!/usr/bin/env python3\nprint(" + repr("\n".join(NAMES[:-1])) + ")\n")
        jq.chmod(0o755)

    def run_gate(self, content=None, **updates):
        env = dict(self.env, **updates)
        env.pop("STACK_LOCK_FILE", None)
        if content is not None:
            path = self.directory / "stack.lock.yaml"
            path.write_text(content)
            env["STACK_LOCK_FILE"] = str(path)
        return subprocess.run(["bash", str(ROOT / "scripts/prod/verify-prod-images.sh")],
                              env=env, capture_output=True, text=True, timeout=10)

    def assert_rejected_before_registry(self, content):
        self.calls.unlink(missing_ok=True)
        result = self.run_gate(content)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(self.calls.exists(), "invalid input reached docker manifest lookup")

    def test_invalid_locks_fail_before_any_registry_lookup(self):
        cases = {
            "empty": lock([]),
            "missing_one": lock(NAMES[1:]),
            "missing_renderer": lock(NAMES[:-1]),
            "duplicate": lock() + f"  gateway: {TAG}\n",
            "unknown": lock() + f"  unrecognized: {TAG}\n",
            "nonproduction": lock() + f"  gameintegration: {TAG}\n",
            "version": lock().replace("version: 1", "version: 2"),
            "registry": lock().replace(REGISTRY, "ghcr.io/other/voice"),
            "tag": lock().replace(f"tag: {TAG}", "tag: " + "b" * 40),
            "image_tag": lock().replace(f"  gateway: {TAG}", "  gateway: " + "b" * 40),
            "mutable": lock().replace(TAG, "latest"),
            "malformed": lock() + "  gateway: [unterminated\n",
            "duplicate_root": lock() + f"registry: {REGISTRY}\n",
            "yaml_alias": lock().replace(f"  gateway: {TAG}", "  gateway: *alias"),
        }
        for name, content in cases.items():
            with self.subTest(name=name):
                self.assert_rejected_before_registry(content)

    def test_complete_lock_checks_exact_applied_refs_and_outputs_tag(self):
        output = self.directory / "output"
        result = self.run_gate(lock(), GITHUB_OUTPUT=str(output))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.calls.read_text().splitlines(), [f"{REGISTRY}/{name}:{TAG}" for name in NAMES])
        self.assertEqual(output.read_text(), f"deploy_tag={TAG}\nchanged_services=\n")

    def test_missing_manifest_is_not_success(self):
        missing = f"{REGISTRY}/nats-hub-config-renderer:{TAG}"
        result = self.run_gate(lock(), MISSING_IMAGE=missing)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("nats-hub-config-renderer", result.stderr)

    def test_explicit_missing_lock_cannot_fall_back_to_tag(self):
        self.env["STACK_LOCK_FILE"] = str(self.directory / "absent.yaml")
        result = subprocess.run(["bash", str(ROOT / "scripts/prod/verify-prod-images.sh")],
                                env=self.env, capture_output=True, text=True, timeout=10)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.calls.exists())

    def test_no_lock_checks_complete_production_profile(self):
        result = self.run_gate()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.calls.read_text().splitlines(), [f"{REGISTRY}/{name}:{TAG}" for name in NAMES])

    def test_images_only_scope_cannot_be_empty_unknown_or_duplicate(self):
        for scope in ("", "unknown", "gameintegration", "gateway,gateway", "gateway,,chat"):
            with self.subTest(scope=scope):
                self.calls.unlink(missing_ok=True)
                result = self.run_gate(lock(), DEPLOY_MODE="images-only", CHANGED_SERVICES=scope)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertFalse(self.calls.exists())

    def test_valid_partial_scope_and_lock_artifact(self):
        saved = self.directory / "artifact" / "stack.lock.yaml"
        output = self.directory / "output"
        result = self.run_gate(lock(), DEPLOY_MODE="images-only", CHANGED_SERVICES="gateway,chat",
                               PROD_STACK_LOCK_OUT=str(saved), GITHUB_OUTPUT=str(output))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(saved.read_text(), lock())
        self.assertIn("changed_services=gateway,chat\n", output.read_text())
        self.assertIn(f"stack_lock_file={saved}\n", output.read_text())

    def test_json_scope_is_validated_without_interpreting_shell_text(self):
        for payload in ('{}', '["gateway", "gateway"]', '["gateway,chat"]', '["$(touch bad)"]'):
            with self.subTest(payload=payload):
                self.calls.unlink(missing_ok=True)
                result = self.run_gate(lock(), DEPLOY_MODE="images-only", CHANGED_SERVICES_JSON=payload)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(self.calls.exists())

    def test_missing_manifest_never_publishes_artifact(self):
        saved = self.directory / "must-not-exist.yaml"
        result = self.run_gate(lock(), PROD_STACK_LOCK_OUT=str(saved), MISSING_IMAGE=f"{REGISTRY}/gateway:{TAG}")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(saved.exists())

    def test_direct_apply_rejects_noop_scope_before_cluster_effects(self):
        kubectl = self.directory / "kubectl"
        kubectl.write_text("#!/usr/bin/env bash\necho cluster-effect >> \"${IMAGE_CALLS}\"\nexit 1\n")
        kubectl.chmod(0o755)
        for scope in ("", "unknown", "gameintegration"):
            with self.subTest(scope=scope):
                self.calls.unlink(missing_ok=True)
                result = subprocess.run(["bash", str(ROOT / "scripts/prod/render-and-apply-prod.sh")],
                                        env=dict(self.env, DEPLOY_MODE="images-only", CHANGED_SERVICES=scope),
                                        capture_output=True, text=True, timeout=10)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("production image plan", result.stderr)
                self.assertFalse(self.calls.exists())

    def test_semver_lock_can_supply_tag_and_json_partial_scope(self):
        result = self.run_gate(lock().replace(TAG, "v1.2.3-rc.1"), VOICE_IMAGE_TAG="",
                               DEPLOY_MODE="images-only", CHANGED_SERVICES_JSON='["gateway"]')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(all(ref.endswith(":v1.2.3-rc.1") for ref in self.calls.read_text().splitlines()))


if __name__ == "__main__":
    unittest.main()
