#!/usr/bin/env python3
"""Contract checks for the dedicated DEV replay store template."""
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
TEMPLATE = ROOT / "deploy/templates/story-media-replay-redis.yaml"


class ReplayManifestTest(unittest.TestCase):
    def test_manifest_fences_storage_and_uses_durable_single_primary(self):
        self.assertTrue(TEMPLATE.is_file(), "replay fixture manifest is required")
        doc = json.loads(TEMPLATE.read_text(encoding="utf-8"))
        objects = {x["kind"]: x for x in doc["items"]}
        config = objects["ConfigMap"]
        self.assertTrue(config["immutable"])
        for directive in ("appendonly yes", "appendfsync always", "aof-load-truncated no", "no-appendfsync-on-rewrite no", "maxmemory-policy noeviction"):
            self.assertIn(directive, config["data"]["redis.conf"].splitlines())
        start = config["data"]["start.sh"]
        self.assertIn("/data/.voice-replay-binding", start)
        self.assertIn("REPLAY_BINDING", start)
        self.assertIn("requirepass", start)
        deployment = objects["Deployment"]["spec"]
        self.assertEqual(deployment["replicas"], 1)
        self.assertEqual(deployment["strategy"]["type"], "Recreate")
        pod = deployment["template"]["spec"]
        self.assertFalse(pod["automountServiceAccountToken"])
        container = pod["containers"][0]
        self.assertIn("@sha256:", container["image"])
        self.assertTrue(container["securityContext"]["readOnlyRootFilesystem"])
        self.assertFalse(container["securityContext"]["allowPrivilegeEscalation"])
        self.assertTrue(any("persistentVolumeClaim" in v for v in pod["volumes"]))
        self.assertNotIn("Secret", objects)
        self.assertEqual(objects["Service"]["spec"]["type"], "ClusterIP")


if __name__ == "__main__":
    unittest.main()
