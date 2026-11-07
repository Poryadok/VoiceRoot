#!/usr/bin/env python3
"""Collect only bounded, allowlisted Space media admission failure classes."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import time


SCHEMA = "voice-space-media-admission-diagnostics-v2"
EVENT = "space_media_admission_commit_failed"
RECONCILIATION_EVENT = "Space media authority reconciliation failed"
FAILURE_CLASSES = frozenset({
    "room_head_pending", "admission_conflict", "admission_unavailable", "store_error",
})
MAX_LOG_BYTES = 64 * 1024
MAX_LOG_LINES = 256
MAX_FAILURE_RECORDS = 8
MAX_RECORD_BYTES = 4096
COMMAND_TIMEOUT_SECONDS = 15


def sanitize_log_bytes(data: bytes, command_result: str) -> dict:
    """Emit only class labels; never copy service log text or arbitrary fields."""
    lines = data.splitlines()
    if command_result != "complete" and data and not data.endswith((b"\n", b"\r")):
        lines = lines[:-1]
    line_limited = len(lines) > MAX_LOG_LINES
    lines = lines[-MAX_LOG_LINES:]
    result = command_result if command_result != "complete" else ("line_limit" if line_limited else "complete")
    classes: list[str] = []
    omitted = 0
    for line in lines:
        start = line.find(b"{")
        if start < 0:
            continue
        try:
            event = json.loads(line[start:])
        except (UnicodeDecodeError, json.JSONDecodeError):
            continue
        if not isinstance(event, dict):
            continue
        if event.get("msg") == EVENT:
            failure_class = event.get("failure_class")
            if not isinstance(failure_class, str) or failure_class not in FAILURE_CLASSES:
                continue
        elif event.get("msg") == RECONCILIATION_EVENT:
            # Do not retain the error text: it can include authority or storage data.
            failure_class = "authority_reconciliation_failed"
        else:
            continue
        if len(classes) < MAX_FAILURE_RECORDS:
            classes.append(failure_class)
        else:
            omitted += 1
    return {
        "schema": SCHEMA,
        "result": "matched" if classes else ("no_match" if result == "complete" else result),
        "collection_result": result,
        "bytes_read": min(len(data), MAX_LOG_BYTES),
        "lines_read": len(lines),
        "failure_classes": classes,
        "omitted_failure_records": omitted,
    }


def _terminate(proc: subprocess.Popen) -> None:
    try:
        if os.name == "posix":
            # The leader may have exited while a child still holds stdout.
            # Signal before poll/wait can reap it and release the unique PGID.
            os.killpg(proc.pid, signal.SIGKILL)
        elif proc.poll() is None:
            proc.kill()
    except OSError:
        pass
    if proc.poll() is None:
        try:
            proc.wait(timeout=2)
        except subprocess.TimeoutExpired:
            pass


def bounded_command(argv: list[str]) -> tuple[bytes, str]:
    """Read Compose output with strict byte/time bounds, without persisting it."""
    try:
        proc = subprocess.Popen(
            argv,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            stdin=subprocess.DEVNULL,
            close_fds=True,
            start_new_session=(os.name == "posix"),
        )
    except (OSError, ValueError):
        return b"", "unavailable"

    assert proc.stdout is not None
    output = bytearray()
    deadline = time.monotonic() + COMMAND_TIMEOUT_SECONDS
    result = "complete"
    leader_waited = False
    selector = None
    try:
        selector = selectors.DefaultSelector()
        selector.register(proc.stdout, selectors.EVENT_READ)
        while selector.get_map():
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                result = "timeout"
                break
            if not selector.select(min(remaining, 0.1)):
                continue
            room = MAX_LOG_BYTES - len(output)
            raw = os.read(proc.stdout.fileno(), min(4096, room + 1))
            if not raw:
                selector.unregister(proc.stdout)
                proc.stdout.close()
                continue
            output.extend(raw[:room])
            if len(raw) > room:
                result = "byte_limit"
                break
        if result == "complete":
            try:
                proc.wait(timeout=max(0.01, deadline - time.monotonic()))
                leader_waited = True
            except subprocess.TimeoutExpired:
                result = "timeout"
            if result == "complete" and proc.returncode != 0:
                result = "command_failed"
        return bytes(output), result
    finally:
        try:
            if selector is not None:
                selector.close()
        finally:
            if not leader_waited:
                _terminate(proc)
            if not proc.stdout.closed:
                proc.stdout.close()


def _write_record(path: Path, record: dict) -> bool:
    try:
        encoded = (json.dumps(record, sort_keys=True, separators=(",", ":")) + "\n").encode("ascii")
        if len(encoded) > MAX_RECORD_BYTES:
            return False
        # The parent is a unique per-attempt directory under RUNNER_TEMP.
        # Creating it exclusively prevents following a pre-planted directory
        # symlink; descriptor-based creation/mode setting avoids a leaf race.
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


def collect(compose: list[str]) -> dict:
    if not compose or compose[0] != "docker":
        return sanitize_log_bytes(b"", "invalid_invocation")
    argv = compose + [
        "logs", "--no-color", "--no-log-prefix", "--timestamps",
        "--since=15m", f"--tail={MAX_LOG_LINES}", "voice",
    ]
    data, command_result = bounded_command(argv)
    return sanitize_log_bytes(data, command_result)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    parser.add_argument("--compose", nargs=argparse.REMAINDER, required=True)
    args = parser.parse_args(argv)
    compose = args.compose[1:] if args.compose[:1] == ["--"] else args.compose
    record = collect(compose)
    output = Path(args.output)
    return 0 if _write_record(output, record) else 1


if __name__ == "__main__":
    raise SystemExit(main())
