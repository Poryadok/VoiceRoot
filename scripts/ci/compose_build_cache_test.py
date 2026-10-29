#!/usr/bin/env python3
"""Content cache orchestration must preserve the current Compose build inputs."""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import sys

sys.dont_write_bytecode = True
import unittest

PATH = Path(__file__).with_name("compose_build_cache.py")


class ComposeBuildCacheTest(unittest.TestCase):
    def load(self):
        spec = importlib.util.spec_from_file_location("cache", PATH)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_override_only_changes_build_cache_and_separates_services(self):
        module = self.load()
        config = {"services": {
            "gateway": {"build": {"context": "/current", "args": {"SOURCE": "new"}},
                        "environment": {"SECRET": "must-not-cache"}, "volumes": ["state:/data"]},
            "file": {"build": {"context": "/current/file"}},
            "postgres": {"image": "postgres:18"}}}
        result = module.cache_override(config, "voice-ci-v1")
        self.assertEqual(set(result), {"services"})
        self.assertEqual(set(result["services"]), {"gateway", "file"})
        for service, item in result["services"].items():
            self.assertEqual(set(item), {"build"})
            self.assertEqual(set(item["build"]), {"cache_from", "cache_to"})
            self.assertEqual(item["build"]["cache_from"],
                             [f"type=gha,version=2,scope=voice-ci-v1-{service}"])
            self.assertEqual(item["build"]["cache_to"],
                             [f"type=gha,version=2,scope=voice-ci-v1-{service},mode=max,ignore-error=true"])
        self.assertNotIn("SECRET", json.dumps(result))

    def test_current_config_and_explicit_builder_always_execute(self):
        module = self.load()
        calls = []
        compose = ["docker", "compose", "--env-file", "/private/run.env", "-p", "owned",
                   "-f", "/current/compose.yml", "--profile", "app"]
        def run(command, **kwargs):
            calls.append(command)
            if command[-3:] == ["config", "--format", "json"]:
                return subprocess.CompletedProcess(command, 0,
                    json.dumps({"services": {"file": {"build": {"context": "/current"}}}}))
            override = Path(command[command.index("-f", len(compose)) + 1])
            self.assertEqual(set(json.loads(override.read_text())["services"]), {"file"})
            return subprocess.CompletedProcess(command, 37)
        with tempfile.TemporaryDirectory() as temp:
            self.assertEqual(module.build(compose, "voice-ci-v1", "owned-builder",
                                          temp, run=run), 37)
            self.assertEqual(list(Path(temp).iterdir()), [])
        self.assertEqual(calls[0], compose + ["config", "--format", "json"])
        self.assertEqual(calls[1][:len(compose)], compose)
        self.assertEqual(calls[1][-3:], ["build", "--builder", "owned-builder"])
        self.assertNotIn("up", calls[1])

    def test_config_failure_stops_before_build(self):
        module = self.load()
        calls = []
        def run(command, **kwargs):
            calls.append(command)
            return subprocess.CompletedProcess(command, 19, "")
        self.assertEqual(module.build(["docker", "compose"], "voice-ci", "builder",
                                      None, run=run), 19)
        self.assertEqual(len(calls), 1)

    def test_scope_rejects_option_injection(self):
        module = self.load()
        for scope in ("", "safe,mode=min", "../other", "secret token"):
            with self.assertRaises(ValueError):
                module.cache_override({"services": {}}, scope)


if __name__ == "__main__":
    unittest.main()
