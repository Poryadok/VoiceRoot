#!/usr/bin/env python3
"""Offline tests for the T31 Compose bootstrap environment writer."""

import importlib.util
from pathlib import Path
import re
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
BOOTSTRAP = ROOT / "scripts/ci/t31-session-event-compose-bootstrap.py"
WORKFLOW = ROOT / ".github/workflows/t31-session-events-e2e.yml"
SPEC = importlib.util.spec_from_file_location("t31_session_event_bootstrap", BOOTSTRAP)
assert SPEC and SPEC.loader
bootstrap = importlib.util.module_from_spec(SPEC)
sys.dont_write_bytecode = True
SPEC.loader.exec_module(bootstrap)


class ComposeBootstrapEnvironmentTests(unittest.TestCase):
    def test_member_profile_ids_are_sorted_for_gis_validation(self):
        self.assertEqual(
            bootstrap.canonical_member_profile_ids(
                "a2691bc7-2703-4040-bfae-f5580a8d0fb0",
                "9afcddc9-082c-4f17-b08c-3d1f18a5da10",
            ),
            [
                "9afcddc9-082c-4f17-b08c-3d1f18a5da10",
                "a2691bc7-2703-4040-bfae-f5580a8d0fb0",
            ],
        )

    def test_sanitized_error_summary_keeps_only_error_code(self):
        secret = "must-not-appear-in-failure-artifact"
        summary = bootstrap.safe_error_code({
            "error_code": "INVALID_ARGUMENT",
            "access_token": secret,
            "password": secret,
        })
        self.assertEqual(summary, "INVALID_ARGUMENT")
        self.assertNotIn(secret, summary)
        self.assertEqual(bootstrap.safe_error_code({"error_code": secret}), "UNAVAILABLE")

    def test_workflow_failure_artifact_uses_sanitized_allowlist(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")
        start = workflow.index("      - name: Save sanitized T31 acceptance evidence")
        end = workflow.index("      - name: Remove this run's isolated Compose project and fixtures", start)
        artifact_step = workflow[start:end]

        paths = re.findall(r"^\s+(tmp/[^\s]+)$", artifact_step, flags=re.MULTILINE)
        self.assertEqual(
            paths,
            [
                "tmp/t31-session-event-e2e/bootstrap-result.json",
                "tmp/t31-session-event-e2e/acceptance.log",
            ],
        )
        self.assertNotIn("compose.env", artifact_step)
        self.assertNotIn("/state/", artifact_step)
        self.assertNotIn("/tls/", artifact_step)

    def test_rewrites_preserve_static_acceptance_inputs(self):
        with tempfile.TemporaryDirectory(prefix="voice-t31-compose-env-") as directory:
            root = Path(directory)
            tls_dir = root / "tls"
            state_dir = root / "state"
            tls_dir.mkdir()
            state_dir.mkdir()
            env_file = root / "compose.env"
            app = {"application_id": "app", "environment_id": "env", "secret": "target-secret"}
            other = {"application_id": "other-app", "environment_id": "other-env", "secret": "other-secret"}

            bootstrap.write_compose_env(env_file, tls_dir, state_dir, "operator-id", app, other)
            values = dict(line.split("=", 1) for line in env_file.read_text(encoding="ascii").splitlines())

            self.assertEqual(values["T31_E2E_TLS_DIR"], str(tls_dir.resolve()))
            self.assertEqual(values["T31_E2E_STATE_DIR"], str(state_dir.resolve()))
            self.assertEqual(values["T31_ACCEPTANCE_PHASE"], "unselected")
            self.assertEqual(values["T31_E2E_OPERATOR_ACCOUNT_ID"], "operator-id")
            self.assertEqual(values["T31_GAME_SERVER_CREDENTIAL"], "target-secret")
            self.assertEqual(values["T31_OTHER_GAME_SERVER_CREDENTIAL"], "other-secret")


if __name__ == "__main__":
    unittest.main()
