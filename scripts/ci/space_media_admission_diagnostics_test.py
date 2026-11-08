#!/usr/bin/env python3
"""Tests for the bounded Compose Space-admission diagnostic collector."""

import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import space_media_admission_diagnostics as diagnostics  # noqa: E402


class SpaceMediaAdmissionDiagnosticsTests(unittest.TestCase):
    def test_classifies_authority_reconciliation_without_exposing_error_text(self):
        raw = b'{"msg":"Space media authority reconciliation failed","error":"private-token-and-dsn"}\n'
        result = diagnostics.sanitize_log_bytes(raw, "complete")
        self.assertEqual(result['result'], 'matched')
        self.assertEqual(result['failure_classes'], ['authority_reconciliation_failed'])
        self.assertNotIn('private-token-and-dsn', json.dumps(result))

    def test_keeps_only_exact_event_and_allowlisted_failure_class(self):
        lines = [
            'voice-1 | {"msg":"space_media_admission_commit_failed","failure_class":"room_head_pending","profile_id":"private"}',
            'voice-1 | {"msg":"space_media_admission_commit_failed","failure_class":"raw-postgres-error"}',
            'voice-1 | {"msg":"space_media_admission_commit_failed","failure_class":["store_error","private"]}',
            'voice-1 | {"msg":"other_event","failure_class":"store_error","secret":"private"}',
        ]

        result = diagnostics.sanitize_log_bytes(("\n".join(lines) + "\n").encode(), "complete")

        self.assertEqual(result["result"], "matched")
        self.assertEqual(result["failure_classes"], ["room_head_pending"])
        self.assertNotIn("private", json.dumps(result))
        self.assertNotIn("postgres", json.dumps(result))

    def test_drops_an_incomplete_record_after_bounded_collection(self):
        raw = b'{"msg":"space_media_admission_commit_failed","failure_class":"store_error"}'

        result = diagnostics.sanitize_log_bytes(raw, "byte_limit")

        self.assertEqual(result["result"], "byte_limit")
        self.assertEqual(result["failure_classes"], [])

    def test_caps_emitted_failure_classes(self):
        line = '{"msg":"space_media_admission_commit_failed","failure_class":"store_error"}'
        raw = ((line + "\n") * (diagnostics.MAX_FAILURE_RECORDS + 3)).encode()

        result = diagnostics.sanitize_log_bytes(raw, "complete")

        self.assertEqual(len(result["failure_classes"]), diagnostics.MAX_FAILURE_RECORDS)
        self.assertEqual(result["omitted_failure_records"], 3)


if __name__ == "__main__":
    unittest.main()
