#!/usr/bin/env python3
"""Collect bounded, sanitized Auth session-floor failure classes from Compose."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

from space_media_admission_diagnostics import bounded_command


SCHEMA = "voice-auth-session-floor-diagnostics-v1"
METHOD = "voice.auth.v1.AuthService/GetVoiceSessionEpochFloor"
FAILURE_MARKERS = {
    b"principal rejected": "principal_rejected",
    b"UNAUTHENTICATED": "unauthenticated",
    b"PERMISSION_DENIED": "permission_denied",
    b"UNAVAILABLE": "unavailable",
    b"DEADLINE_EXCEEDED": "deadline_exceeded",
}
MAX_LOG_BYTES = 64 * 1024
MAX_LOG_LINES = 256
MAX_FAILURE_RECORDS = 8
MAX_RECORD_BYTES = 4096


def sanitize_log_bytes(data: bytes, command_result: str) -> dict:
    lines = data.splitlines()
    if command_result != "complete" and data and not data.endswith((b"\n", b"\r")):
        lines = lines[:-1]
    line_limited = len(lines) > MAX_LOG_LINES
    lines = lines[-MAX_LOG_LINES:]
    collection = command_result if command_result != "complete" else (
        "line_limit" if line_limited else "complete"
    )
    failures: list[str] = []
    omitted = 0
    method = METHOD.encode("ascii")
    for line in lines:
        if method not in line:
            continue
        category = next((label for marker, label in FAILURE_MARKERS.items() if marker in line), None)
        if category is None:
            continue
        if len(failures) < MAX_FAILURE_RECORDS:
            failures.append(category)
        else:
            omitted += 1
    return {
        "schema": SCHEMA,
        "result": "matched" if failures else ("no_match" if collection == "complete" else collection),
        "collection_result": collection,
        "bytes_read": min(len(data), MAX_LOG_BYTES),
        "lines_read": len(lines),
        "failure_classes": failures,
        "omitted_failure_records": omitted,
    }


def collect(compose: list[str]) -> dict:
    if not compose or compose[0] != "docker":
        return sanitize_log_bytes(b"", "invalid_invocation")
    argv = compose + [
        "logs", "--no-color", "--no-log-prefix", "--timestamps",
        "--since=15m", f"--tail={MAX_LOG_LINES}", "auth",
    ]
    data, result = bounded_command(argv)
    return sanitize_log_bytes(data, result)


def write_record(path: Path, record: dict) -> bool:
    try:
        encoded = (json.dumps(record, sort_keys=True, separators=(",", ":")) + "\n").encode("ascii")
        if len(encoded) > MAX_RECORD_BYTES:
            return False
        path.parent.mkdir(mode=0o700)
        flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
        if hasattr(os, "O_NOFOLLOW"):
            flags |= os.O_NOFOLLOW
        fd = os.open(path, flags, 0o600)
        try:
            if os.name == "posix":
                os.fchmod(fd, 0o600)
            with os.fdopen(fd, "wb", closefd=False) as handle:
                handle.write(encoded)
                handle.flush()
        finally:
            os.close(fd)
        return True
    except OSError:
        return False


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    parser.add_argument("--compose", nargs=argparse.REMAINDER, required=True)
    args = parser.parse_args(argv)
    compose = args.compose[1:] if args.compose[:1] == ["--"] else args.compose
    return 0 if write_record(Path(args.output), collect(compose)) else 1


if __name__ == "__main__":
    raise SystemExit(main())
