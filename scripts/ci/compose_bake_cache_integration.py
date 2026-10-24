#!/usr/bin/env python3
"""Opt-in network-free BuildKit content cache proof: python this_file.py.

Uses only a FROM scratch image, one disposable agent-prefixed tag and stopped
containers. No Compose stack, external services, or registry access is needed.
"""
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
import uuid

from compose_bake_plan import build_document, configure_plan


def docker(*args):
    result = subprocess.run(["docker", *map(str, args)], capture_output=True,
                            text=True, timeout=120)
    if result.returncode:
        raise AssertionError(f"Docker proof command failed: {result.stderr[-1000:]}")
    return result


class CacheProof(unittest.TestCase):
    def test_unchanged_copy_is_cached_but_changed_input_changes_image_content(self):
        project = "agent-compose-cache-test-" + uuid.uuid4().hex
        tag = project + "-sentinel"
        self.addCleanup(lambda: docker("image", "rm", "-f", tag))
        with tempfile.TemporaryDirectory(prefix="voice-bake-sentinel-") as temp:
            root = Path(temp)
            context = root / "context"
            context.mkdir()
            (context / "Dockerfile").write_text(
                "FROM scratch\nCOPY sentinel /sentinel\n", encoding="utf-8")
            (context / "sentinel").write_text("first", encoding="utf-8")
            source = root / "compose.yaml"
            config = {"name": project, "services": {
                "sentinel": {"build": {"context": str(context)}}}}
            # Independently ask Compose for its implicit image name.
            compose = root / "original.yaml"
            compose.write_text(json.dumps(config), encoding="utf-8")
            self.assertEqual(docker("compose", "-f", compose, "config", "--images").stdout.strip(), tag)
            document = build_document(config)
            source.write_text(json.dumps(document), encoding="utf-8")
            native = json.loads(docker("buildx", "bake", "-f", source, "--print").stdout)
            plan = configure_plan(document, native)
            # The local proof uses the builder's content cache. GitHub runtime
            # credentials are intentionally unavailable on a developer machine.
            target = plan["target"]["sentinel"]
            target.pop("cache-from")
            target.pop("cache-to")
            target["network"] = "none"
            bake = root / "bake.json"
            bake.write_text(json.dumps(plan), encoding="utf-8")

            def build_and_read():
                result = docker("buildx", "bake", "-f", bake, "--load", "--progress=plain")
                image_id = docker("image", "inspect", "--format", "{{.Id}}", tag).stdout.strip()
                container = docker("create", "--entrypoint", "/sentinel", tag).stdout.strip()
                try:
                    output = root / "output"
                    docker("cp", container + ":/sentinel", output)
                    return image_id, output.read_text(), result.stderr + result.stdout
                finally:
                    docker("rm", container)

            first, content, _ = build_and_read()
            self.assertEqual(content, "first")
            same, content, log = build_and_read()
            self.assertEqual((same, content), (first, "first"))
            self.assertIn("CACHED", log)
            (context / "sentinel").write_text("second", encoding="utf-8")
            changed, content, _ = build_and_read()
            self.assertNotEqual(changed, first)
            self.assertEqual(content, "second")


if __name__ == "__main__":
    unittest.main()
