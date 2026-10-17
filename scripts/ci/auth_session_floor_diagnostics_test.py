#!/usr/bin/env python3
"""Tests for the sanitized Auth session-floor diagnostic collector."""

import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import auth_session_floor_diagnostics as diagnostics  # noqa: E402


class AuthSessionFloorDiagnosticsTests(unittest.TestCase):
    def test_only_exact_method_and_fixed_categories_are_emitted(self):
        raw = (
            'auth | {"method":"voice.auth.v1.AuthService/GetVoiceSessionEpochFloor",'
            '"code":"UNAUTHENTICATED","message":"principal rejected","token":"secret"}\n'
            'auth | voice.auth.v1.AuthService/GetVoiceSessionEpochFloor UNAVAILABLE secret-payload\n'
            'auth | voice.auth.v1.AuthService/Register UNAUTHENTICATED private\n'
        ).encode()

        result = diagnostics.sanitize_log_bytes(raw, "complete")

        self.assertEqual(result["result"], "matched")
        self.assertEqual(result["failure_classes"], ["principal_rejected", "unavailable"])
        self.assertNotIn("secret", json.dumps(result))
        self.assertNotIn("private", json.dumps(result))
        self.assertNotIn("token", json.dumps(result))

    def test_incomplete_trailing_record_is_discarded(self):
        raw = b"voice.auth.v1.AuthService/GetVoiceSessionEpochFloor UNAUTHENTICATED"

        result = diagnostics.sanitize_log_bytes(raw, "byte_limit")

        self.assertEqual(result["result"], "byte_limit")
        self.assertEqual(result["failure_classes"], [])

    def test_caps_failure_records(self):
        line = b"voice.auth.v1.AuthService/GetVoiceSessionEpochFloor UNAUTHENTICATED\n"

        result = diagnostics.sanitize_log_bytes(line * (diagnostics.MAX_FAILURE_RECORDS + 2), "complete")

        self.assertEqual(len(result["failure_classes"]), diagnostics.MAX_FAILURE_RECORDS)
        self.assertEqual(result["omitted_failure_records"], 2)

    def test_rejects_non_docker_collection_command(self):
        result = diagnostics.collect(["sh", "-c", "echo secret"])

        self.assertEqual(result["collection_result"], "invalid_invocation")
        self.assertNotIn("secret", json.dumps(result))


if __name__ == "__main__":
    unittest.main()
