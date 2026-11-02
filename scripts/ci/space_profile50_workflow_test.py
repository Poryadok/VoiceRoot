"""Keep the opt-in profiling route separate from normal audit proof runs."""
from pathlib import Path
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[2]


def workflow(name):
    return yaml.load((ROOT / ".github" / "workflows" / name).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


class ProfileRouteTest(unittest.TestCase):
    def test_default_audit_and_profile_are_separate(self):
        w = workflow("space-audit-ingestion-proof.yml")
        opt = w["on"]["workflow_dispatch"]["inputs"]["m50_profile"]
        self.assertEqual(opt["type"], "boolean")
        self.assertEqual(opt["default"], "false")
        self.assertEqual(opt["required"], "false")
        self.assertEqual(w["jobs"]["ingestion-proof"]["if"], "github.event_name != 'workflow_dispatch' || !inputs.m50_profile")
        group = w["concurrency"]["group"]
        self.assertIn("format('space-audit-ingestion-{0}', github.ref)", group)
        self.assertIn("format('space-m50-profile-{0}-{1}', github.run_id, github.run_attempt)", group)
        self.assertEqual(w["concurrency"]["cancel-in-progress"], "true")

    def test_profile_requires_same_source_and_readonly_permission(self):
        w = workflow("space-audit-ingestion-proof.yml")
        source, profile = w["jobs"]["m50-source"], w["jobs"]["m50-profile"]
        condition = "github.event_name == 'workflow_dispatch' && inputs.m50_profile"
        self.assertEqual(source["if"], condition)
        self.assertEqual(profile["if"], condition)
        self.assertEqual(profile["needs"], "m50-source")
        self.assertEqual(profile["uses"], "./.github/workflows/space-m50-profile.yml")
        self.assertEqual(profile["permissions"], {"contents": "read"})
        step = source["steps"][0]
        self.assertEqual(step["env"]["EXPECTED_SHA"], "${{ inputs.source_sha }}")
        self.assertEqual(step["env"]["CALLER_SHA"], "${{ github.sha }}")
        self.assertIn('test "$EXPECTED_SHA" = "$CALLER_SHA"', step["run"])
        self.assertIn('[[ "$EXPECTED_SHA" =~ ^[0-9a-f]{40}$ ]]', step["run"])
        reusable = workflow("space-m50-profile.yml")
        self.assertIn("workflow_call", reusable["on"])
        self.assertEqual(reusable["permissions"], {"contents": "read"})
        binding = next(s for s in reusable["jobs"]["profile"]["steps"] if s.get("name") == "Bind source and PostgreSQL image")
        self.assertIn('test "$(git rev-parse HEAD)" = "$GITHUB_SHA"', binding["run"])


if __name__ == "__main__":
    unittest.main()
