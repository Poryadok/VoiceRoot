import unittest
from pathlib import Path

import yaml


WORKFLOW = Path(__file__).resolve().parents[2] / ".github/workflows/dev-matchmaking-source-authority.yml"


class WorkflowTests(unittest.TestCase):
    def test_collector_is_manual_master_only_and_read_only(self):
        workflow = yaml.load(WORKFLOW.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)
        self.assertEqual(set(workflow["on"]), {"workflow_dispatch"})
        self.assertEqual(set(workflow["permissions"]), set())
        self.assertEqual(set(workflow["jobs"]), {"source-authority"})

        job = workflow["jobs"]["source-authority"]
        self.assertIn("github.repository == 'Poryadok/VoiceRoot'", job["if"])
        self.assertIn("github.ref == 'refs/heads/master'", job["if"])
        self.assertLessEqual(int(job["timeout-minutes"]), 15)
        self.assertEqual(job["permissions"], {
            "actions": "read", "contents": "read", "packages": "read"})

        checkout = next(step for step in job["steps"]
            if step.get("uses", "").startswith("actions/checkout@"))
        self.assertEqual(checkout["with"]["ref"], "refs/heads/master")
        self.assertEqual(checkout["with"]["persist-credentials"], "false")

        emit = next(step for step in job["steps"] if step.get("name") ==
            "Emit verified Matchmaking source authority")
        self.assertIn("${{ secrets.GITHUB_TOKEN }}", emit["env"]["GITHUB_TOKEN"])
        self.assertIn("${{ inputs.source_run_id }}", emit["env"]["SOURCE_RUN_ID"])
        self.assertIn("matchmaking_source_authority.py", emit["run"])
        self.assertNotIn("set-output", emit["run"])

        upload = next(step for step in job["steps"]
            if step.get("uses", "").startswith("actions/upload-artifact@"))
        self.assertEqual(upload["with"]["name"], "matchmaking-source-authority")
        self.assertEqual(upload["with"]["if-no-files-found"], "error")


if __name__ == "__main__":
    unittest.main()
