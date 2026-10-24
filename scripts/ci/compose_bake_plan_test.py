"""Contracts for current-source Compose builds; no Docker needed by default."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("compose_bake_plan", ROOT / "scripts/ci/compose_bake_plan.py")
plan = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(plan)


class PlanTests(unittest.TestCase):
    def fixture(self):
        config = {"name": "voice", "services": {
            "gateway": {"build": {"context": str(ROOT / "src/backend"),
                                  "dockerfile": "gateway/Dockerfile",
                                  "args": {"SENTINEL": "one"}},
                        "platform": "linux/amd64", "environment": {"PASSWORD": "never-copy"}},
            "infra": {"image": "postgres:16"}}}
        document = plan.build_document(config)
        native = {"target": {"gateway": {**document["services"]["gateway"]["build"],
                                        "tags": ["voice-gateway"]}}}
        return config, document, native

    def test_preserves_inputs_and_implicit_image_without_mutating_config(self):
        config, document, native = self.fixture()
        before = copy.deepcopy((config, document, native))
        result = plan.configure_plan(document, native)["target"]["gateway"]
        self.assertEqual((config, document, native), before)
        self.assertEqual(result["args"], {"SENTINEL": "one"})
        self.assertEqual(result["platforms"], ["linux/amd64"])
        self.assertEqual(result["tags"], ["voice-gateway"])
        self.assertNotIn("PASSWORD", json.dumps(document))
        self.assertIn("type=gha,scope=backend-gateway", result["cache-from"])
        self.assertEqual(result["cache-to"], [
            "type=gha,scope=compose-e2e-gateway,mode=max,timeout=30s,ignore-error=true"])

    def test_missing_extra_or_changed_native_build_cannot_be_loaded(self):
        _, document, native = self.fixture()
        for field, value in (("context", "/stale"), ("dockerfile", "other"),
                             ("args", {"SENTINEL": "stale"}), ("tags", ["stale"]),
                             ("platforms", ["linux/arm64"]), ("target", "other")):
            changed = copy.deepcopy(native)
            changed["target"]["gateway"][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                plan.configure_plan(document, changed)
        for targets in ({}, {**native["target"], "unexpected": {}}):
            with self.assertRaises(ValueError):
                plan.configure_plan(document, {"target": targets})

    def test_input_changes_invalidate_old_plan_and_explicit_image_is_preserved(self):
        config, _, native = self.fixture()
        config["services"]["gateway"]["image"] = "custom/gateway:current"
        document = plan.build_document(config)
        with self.assertRaises(ValueError):
            plan.configure_plan(document, native)
        native["target"]["gateway"]["tags"] = ["custom/gateway:current"]
        self.assertEqual(plan.configure_plan(document, native)["target"]["gateway"]["tags"],
                         ["custom/gateway:current"])
        config["services"]["gateway"]["build"]["args"]["SENTINEL"] = "two"
        with self.assertRaises(ValueError):
            plan.configure_plan(plan.build_document(config), native)

    def test_unsupported_inputs_fail_closed_instead_of_silently_dropping_them(self):
        config, _, native = self.fixture()
        for key in ("secrets", "additional_contexts", "ssh", "dockerfile_inline", "cache_from"):
            changed = copy.deepcopy(config)
            changed["services"]["gateway"]["build"][key] = ["unsupported"]
            with self.subTest(key=key), self.assertRaises(ValueError):
                plan.build_document(changed)
        native["target"]["gateway"]["output"] = ["type=registry"]
        with self.assertRaises(ValueError):
            plan.configure_plan(plan.build_document(config), native)

    def test_pull_policies_cannot_replace_the_fresh_loaded_image(self):
        config, _, _ = self.fixture()
        for policy in ("always", "daily", "weekly", "every_2h"):
            config["services"]["gateway"]["pull_policy"] = policy
            with self.subTest(policy=policy), self.assertRaises(ValueError):
                plan.build_document(config)

    def test_failed_preparation_removes_old_plan_and_sanitizes_docker_failure(self):
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / "plan.json"
            output.write_text('{"stale": true}', encoding="utf-8")
            with patch("sys.argv", ["compose_bake_plan.py", "--output", str(output)]), \
                    patch.object(plan, "docker_json", side_effect=ValueError("private-password")):
                with self.assertRaises(SystemExit) as error:
                    plan.main()
            self.assertEqual(str(error.exception), "COMPOSE_BAKE_PLAN=FAIL")
            self.assertFalse(output.exists())

    def test_matching_frontend_recipes_import_only_read_scopes(self):
        recipes = {
            "web": {"VOICE_API_BASE_URL": "http://127.0.0.1:18080",
                    "VOICE_LIVEKIT_URL": "ws://127.0.0.1:7880"},
            "developer-portal": {"VITE_VOICE_API_BASE": "http://127.0.0.1:18080",
                                 "VITE_OAUTH_CLIENT_ID": "voice-developer-portal",
                                 "VITE_OAUTH_DISABLED": "false"},
        }
        for name, args in recipes.items():
            config = {"name": "voice", "services": {name: {"build": {
                "context": str(ROOT / "src" / ("frontend" if name == "web" else name)),
                "dockerfile": "Dockerfile", "args": args}}}}
            document = plan.build_document(config)
            native = {"target": {name: {**document["services"][name]["build"],
                                       "tags": [f"voice-{name}"]}}}
            target = plan.configure_plan(document, native)["target"][name]
            with self.subTest(name=name):
                self.assertIn(f"type=gha,scope={name}", target["cache-from"])
                self.assertEqual(target["cache-to"], [
                    f"type=gha,scope=compose-e2e-{name},mode=max,timeout=30s,ignore-error=true"])
                self.assertEqual(target["args"], args)
            for key, value in (("args", {"CHANGED": "true"}), ("dockerfile", "other"),
                               ("context", str(ROOT / "other")), ("platforms", ["linux/arm64"]),
                               ("target", "different-stage")):
                changed = copy.deepcopy(document)
                changed["services"][name]["build"][key] = value
                changed_native = {"target": {name: {
                    **changed["services"][name]["build"], "tags": [f"voice-{name}"]}}}
                result = plan.configure_plan(changed, changed_native)["target"][name]
                with self.subTest(name=name, changed=key):
                    self.assertNotIn(f"type=gha,scope={name}", result["cache-from"])
                    self.assertTrue(all("compose-e2e-" in ref for ref in result["cache-to"]))


class WorkflowTests(unittest.TestCase):
    def test_host_go_build_cache_covers_real_modules_and_never_reuses_test_results(self):
        workflow = (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")
        job = workflow.split("  compose-e2e:\n", 1)[1].split("\n  a1-e2e:", 1)[0]
        setup = job.split("      - uses: actions/setup-go@v5", 1)[1].split("      - ", 1)[0]
        self.assertIn("cache: true", setup)
        for module in ("gateway", "notification", "pkg"):
            path = f"src/backend/{module}/go.sum"
            self.assertIn(path, setup)
            self.assertTrue((ROOT / path).is_file())
        script = (ROOT / "scripts/ci/compose-e2e-smoke.sh").read_text(encoding="utf-8")
        commands = [line for line in script.splitlines() if "go test " in line]
        self.assertGreaterEqual(len(commands), 4)
        self.assertTrue(all("-count=1" in line for line in commands))
        self.assertIn("CGO_ENABLED=1 go test -tags=live -count=1", job)

    def test_current_checkout_is_built_and_loaded_before_no_build_start(self):
        workflow = (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")
        job = workflow.split("  compose-e2e:\n", 1)[1].split("\n  a1-e2e:", 1)[0]
        self.assertIn("uses: docker/bake-action@v7", job)
        self.assertIn("source: .", job)
        self.assertIn("load: true", job)
        self.assertIn("--profile app up -d --no-build", job)
        self.assertLess(job.index("generate-phase0-fixture.py"), job.index("compose_bake_plan.py"))
        self.assertLess(job.index("docker/bake-action"), job.index("Start app stack"))
        self.assertIn("max-parallelism = 4", job)
        self.assertIn('exit "$startup_status"', job)


if __name__ == "__main__":
    unittest.main()
