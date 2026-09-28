#!/usr/bin/env python3
"""Offline tests for the T31 Compose bootstrap environment writer."""

import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
BOOTSTRAP = ROOT / "scripts/ci/t31-session-event-compose-bootstrap.py"
SPEC = importlib.util.spec_from_file_location("t31_session_event_bootstrap", BOOTSTRAP)
assert SPEC and SPEC.loader
bootstrap = importlib.util.module_from_spec(SPEC)
sys.dont_write_bytecode = True
SPEC.loader.exec_module(bootstrap)


class ComposeBootstrapEnvironmentTests(unittest.TestCase):
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
