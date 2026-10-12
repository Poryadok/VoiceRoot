#!/usr/bin/env python3
"""Collect bounded, sanitized Compose startup diagnostics for the T31 workflow."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import selectors
import signal
import subprocess
import time


SCHEMA = "voice-t31-startup-diagnostics-v2"
SERVICES = (
    "auth", "user", "role", "chat", "space", "messaging", "voice", "bot",
    "notification", "gameintegration", "gateway", "matchmaking", "search",
    "file", "moderation", "social", "subscription", "story", "realtime",
    "analytics", "t31-session-event-receiver-db",
)
SERVICE_SET = frozenset(SERVICES)
STATES = frozenset({"created", "running", "restarting", "paused", "exited", "dead", "removing", "unknown"})
HEALTH_STATES = frozenset({"healthy", "unhealthy", "starting", "none", "unknown"})
ERROR_CODES = frozenset({
    "CANCELED", "UNKNOWN", "INVALID_ARGUMENT", "DEADLINE_EXCEEDED", "NOT_FOUND",
    "ALREADY_EXISTS", "PERMISSION_DENIED", "RESOURCE_EXHAUSTED", "FAILED_PRECONDITION",
    "ABORTED", "OUT_OF_RANGE", "UNAUTHENTICATED", "INTERNAL", "UNAVAILABLE",
    "DATA_LOSS", "UNIMPLEMENTED",
})
EXCEPTION_CLASSES = frozenset({
    "ConnectException", "ConnectionError", "IllegalStateException", "IOException",
    "RuntimeException", "SQLException", "SQLNonTransientConnectionException",
    "TimeoutException", "UnknownHostException",
})
SQLSTATES = frozenset({"08000", "08001", "08003", "08006", "28P01", "3D000", "53300", "57P01", "57P03"})
SOURCE_BASENAMES = frozenset({
    "auth.go", "config.go", "database.go", "main.go", "postgres.go", "redis.go",
    "runtime.go", "server.go", "session_floor.go", "voice.go",
})
MAX_PS_BYTES = 32 * 1024
MAX_LOG_BYTES = 64 * 1024
MAX_LOG_LINES = 512
MAX_RECORD_BYTES = 4096
COMMAND_TIMEOUT_SECONDS = 15.0
READ_CHUNK_BYTES = 4096

_SERVICE_PREFIX = re.compile(r"^\s*([A-Za-z0-9_-]+)\s+\|\s?(.*)$")
VOICE_STARTUP_PREFIXES = (
    ("voice lifecycle database:", "voice_lifecycle_database"),
    ("voice GIS principal configuration:", "voice_gis_principal_configuration"),
    ("voice principal JWKS configuration:", "voice_principal_jwks_configuration"),
    ("voice Role grant checker configuration:", "voice_role_grant_configuration"),
    ("voice Space lifecycle principal configuration:", "voice_space_principal_configuration"),
    ("voice user-principal configuration:", "voice_user_principal_configuration"),
    ("voice Auth session-floor configuration:", "voice_auth_session_floor_configuration"),
    ("Voice user-principal listener requires the private Auth session-floor client", "voice_auth_session_floor_client_missing"),
    ("Voice Space lifecycle principal requires VOICE_DATABASE_URL", "voice_space_database_missing"),
    ("voice Space lifecycle database schema:", "voice_space_database_schema"),
    ("voice Space lifecycle principal runtime:", "voice_space_principal_runtime"),
    ("Voice managed game-session admission requires both GIS provisioning and Role grant checker configuration", "voice_game_admission_configuration"),
    ("Voice Role grants and user-principal listener require the Voice service principal signer", "voice_principal_signer_missing"),
    ("voice principal issuer:", "voice_principal_issuer"),
    ("voice Auth session-floor client:", "voice_auth_session_floor_client"),
    ("voice user-principal runtime:", "voice_user_principal_runtime"),
    ("voice Role grant checker:", "voice_role_grant_checker"),
    ("voice GIS principal runtime:", "voice_gis_principal_runtime"),
    ("VOICE_DATABASE_URL is required when GIS room provisioning is enabled", "voice_gis_database_missing"),
    ("voice GIS database:", "voice_gis_database"),
    ("voice GIS database schema:", "voice_gis_database_schema"),
    ("voice account fence schema:", "voice_account_fence_schema"),
    ("voice GIS listener:", "voice_gis_listener"),
    ("nats jetstream publisher:", "voice_jetstream_publisher"),
    ("chat grpc:", "voice_chat_grpc"),
    ("user grpc:", "voice_user_grpc"),
    ("social grpc:", "voice_social_grpc"),
    ("space grpc:", "voice_space_grpc"),
    ("subscription grpc:", "voice_subscription_grpc"),
    ("role grpc:", "voice_role_grpc"),
    ("federated media configuration invalid", "voice_federated_media_configuration"),
    ("Space media invalidation requires the durable Voice Redis call store", "space_media_redis_store_missing"),
    ("Space media invalidation requires Space and Role authority resolvers", "space_media_authority_resolvers_missing"),
    ("Space media durable event publisher is not configured", "space_media_event_publisher_missing"),
    ("grpc listen:", "voice_grpc_listen"),
    ("grpc serve:", "voice_grpc_serve"),
    ("voice Space lifecycle principal listen:", "voice_space_principal_listen"),
    ("voice user-principal listener:", "voice_user_principal_listener"),
    ("Space media admission requires the current Auth session-floor checker", "space_media_auth_checker_missing"),
    ("Space media PostgreSQL admission schema:", "space_media_postgres_admission_schema"),
    ("Space media initial reconciliation:", "space_media_initial_reconciliation"),
    ("Space media invalidation consumer startup:", "space_media_invalidation_consumer_startup"),
)
_ERROR_MARKER = re.compile(
    r"(?:\blevel\s*[=:]\s*error\b|\bERROR\b|\bFATAL\b|\bPANIC\b|\bSQLSTATE\b|\bexception\b|"
    + "|".join(re.escape(prefix) for prefix, _ in VOICE_STARTUP_PREFIXES)
    + r")",
    re.IGNORECASE,
)
_ERROR_CODE = re.compile(r"\berror_code\s*[=:]\s*([A-Z0-9_]{1,64})\b")
_SQLSTATE = re.compile(r"\bSQLSTATE\s*[=: ]\s*['\"]?([A-Z0-9]{5})\b", re.IGNORECASE)
_EXCEPTION = re.compile(r"\b([A-Za-z][A-Za-z0-9]*(?:Exception|Error))\b")
_SOURCE = re.compile(r"(?:^|[/\\])([A-Za-z0-9_.-]+\.go):(\d{1,7})\b")


def _base_record(startup_status: int) -> dict:
    return {
        "schema": SCHEMA,
        "startup_command_exit_status": startup_status if 0 <= startup_status <= 255 else 1,
        "status_collection": {"result": "unknown", "services": []},
        "log_collection": {
            "result": "unknown", "coverage": "unknown", "bytes_read": 0,
            "lines_read": 0, "services_requested": [], "services_seen": [], "truncated": False,
            "window": "first_emitted", "window_bytes_limit": MAX_LOG_BYTES,
            "window_lines_limit": MAX_LOG_LINES, "causal_order": "not_established",
        },
        "first_error": _error_record(),
    }


def _error_record(result: str = "unknown") -> dict:
    return {
        "result": result, "service": "unknown", "stage": "unknown",
        "error_code": "unknown", "startup_category": "unknown", "exception_class": "unknown",
        "sqlstate": "unknown", "source_basename": "unknown", "source_line": None,
        "selection": "unknown", "causal_order": "not_established",
    }


def _bounded_command(argv: list[str], timeout_seconds: float, max_bytes: int) -> tuple[bytes, str, int | None]:
    """Run one direct command in an owned process group with bounded output and wait."""
    try:
        proc = subprocess.Popen(
            argv,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            stdin=subprocess.DEVNULL,
            close_fds=True,
            start_new_session=(os.name == "posix"),
        )
    except (OSError, ValueError):
        return b"", "unavailable", None

    assert proc.stdout is not None
    output = bytearray()
    selector = selectors.DefaultSelector()
    selector.register(proc.stdout, selectors.EVENT_READ)
    deadline = time.monotonic() + timeout_seconds
    result = "complete"

    def stop_owned_process_group() -> None:
        try:
            if os.name == "posix":
                # start_new_session gives this invocation its own process group;
                # a plugin child retaining the pipe is still part of that group.
                os.killpg(proc.pid, signal.SIGTERM)
            elif proc.poll() is None:
                proc.terminate()
        except (OSError, ProcessLookupError):
            pass
        try:
            proc.wait(timeout=1.0)
        except subprocess.TimeoutExpired:
            try:
                if os.name == "posix":
                    os.killpg(proc.pid, signal.SIGKILL)
                elif proc.poll() is None:
                    proc.kill()
            except (OSError, ProcessLookupError):
                pass
        else:
            # The direct process may exit while a child still owns the pipe.
            # Signal only this invocation's process group to close that drain.
            if os.name == "posix":
                try:
                    os.killpg(proc.pid, signal.SIGKILL)
                except (OSError, ProcessLookupError):
                    pass
        try:
            proc.wait(timeout=1.0)
        except subprocess.TimeoutExpired:
            pass

    try:
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                result = "timeout"
                stop_owned_process_group()
                break
            ready = selector.select(min(remaining, 0.25))
            if not ready:
                if proc.poll() is not None:
                    # A descendant retaining the pipe is not followed; the direct producer is the contract.
                    result = "drain_incomplete"
                    stop_owned_process_group()
                    break
                continue
            chunk = os.read(proc.stdout.fileno(), min(READ_CHUNK_BYTES, max_bytes + 1 - len(output)))
            if not chunk:
                break
            output.extend(chunk)
            if len(output) > max_bytes:
                result = "byte_limit"
                del output[max_bytes:]
                stop_owned_process_group()
                break
        if proc.poll() is None:
            try:
                proc.wait(timeout=max(0.0, deadline - time.monotonic()))
            except subprocess.TimeoutExpired:
                result = "timeout"
                stop_owned_process_group()
        if result == "complete" and proc.returncode != 0:
            result = "command_failed"
        return bytes(output), result, proc.returncode
    finally:
        selector.close()
        proc.stdout.close()


def _decode_bounded(data: bytes) -> tuple[str | None, str]:
    try:
        text = data.decode("utf-8-sig", errors="strict")
    except UnicodeDecodeError:
        return None, "invalid_encoding"
    return text, "complete"


def _json_rows(text: str) -> list[dict] | None:
    if not text.strip():
        return []
    try:
        value = json.loads(text)
        rows = value if isinstance(value, list) else [value]
        return [row for row in rows if isinstance(row, dict)]
    except (json.JSONDecodeError, TypeError):
        rows = []
        for line in text.splitlines():
            if not line.strip():
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                return None
            if not isinstance(row, dict):
                return None
            rows.append(row)
        return rows


def sanitize_status(data: bytes, command_result: str) -> tuple[dict, tuple[str, ...]]:
    text, decode_result = _decode_bounded(data)
    if text is None:
        return {"result": decode_result, "services": [], "bytes_read": len(data), "rows_read": 0,
                "truncated": command_result == "byte_limit"}, ()
    rows = _json_rows(text)
    if rows is None:
        return {"result": "parse_error", "services": [], "bytes_read": len(data), "rows_read": 0,
                "truncated": command_result == "byte_limit"}, ()
    safe_rows = []
    failed = []
    for row in rows:
        service = row.get("Service")
        if service not in SERVICE_SET:
            continue
        state_raw = row.get("State")
        state = state_raw.lower() if isinstance(state_raw, str) else "unknown"
        if state not in STATES:
            state = "unknown"
        health_raw = row.get("Health")
        health = health_raw.lower() if isinstance(health_raw, str) else "none"
        if health not in HEALTH_STATES:
            health = "unknown"
        exit_raw = row.get("ExitCode")
        try:
            exit_code = int(exit_raw) if exit_raw is not None else None
        except (TypeError, ValueError):
            exit_code = None
        if exit_code is not None and not -255 <= exit_code <= 255:
            exit_code = None
        safe_rows.append({"service": service, "state": state, "health": health, "exit_code": exit_code})
        if state in {"exited", "dead", "restarting"} or health == "unhealthy" or (exit_code not in (None, 0)):
            failed.append(service)
    if command_result != "complete":
        result = command_result
    else:
        result = "complete" if rows is not None else "parse_error"
    safe_rows = safe_rows[:len(SERVICES)]
    safe_rows.sort(key=lambda item: SERVICES.index(item["service"]))
    return {
        "result": result, "services": safe_rows, "bytes_read": len(data),
        "rows_read": len(rows), "truncated": command_result == "byte_limit",
    }, tuple(dict.fromkeys(failed))


def _stage_for(line: str, sqlstate: str | None, exception: str | None) -> str:
    lowered = line.lower()
    if sqlstate is not None or "postgres" in lowered or "database" in lowered:
        return "database"
    if any(marker in lowered for marker in ("certificate", "tls", "jwks")):
        return "identity_dependency"
    if any(marker in lowered for marker in ("connection refused", "deadline exceeded", "dependency")):
        return "dependency"
    if exception is not None or "panic" in lowered or "fatal" in lowered:
        return "process_fault"
    if any(marker in lowered for marker in ("config", "configuration")):
        return "configuration"
    return "startup_error"


def sanitize_logs(data: bytes, command_result: str, failed_services: tuple[str, ...]) -> tuple[dict, dict]:
    text, decode_result = _decode_bounded(data)
    if text is None:
        return {
            "result": decode_result, "coverage": "unknown", "bytes_read": len(data),
            "lines_read": 0, "services_requested": list(failed_services or SERVICES),
            "services_seen": [], "truncated": command_result == "byte_limit",
            "window": "first_emitted", "window_bytes_limit": MAX_LOG_BYTES,
            "window_lines_limit": MAX_LOG_LINES, "causal_order": "not_established",
        }, _error_record()
    lines = text.splitlines()
    if command_result != "complete" and text and not text.endswith(("\n", "\r")):
        # Do not parse a potentially cut-off final line as a diagnostic marker.
        lines = lines[:-1]
    line_limited = len(lines) > MAX_LOG_LINES
    lines = lines[:MAX_LOG_LINES]
    if command_result != "complete":
        result = command_result
    elif line_limited:
        result = "line_limit"
    else:
        result = "complete"
    requested = failed_services or SERVICES
    seen = set()
    first_error = None
    for line in lines:
        match = _SERVICE_PREFIX.match(line)
        if not match:
            continue
        service, message = match.group(1), match.group(2)
        if service not in SERVICE_SET:
            # Compose v2 prefixes replicas as service-1; older/project-prefixed
            # forms may include project_service_1. Emit only the known service.
            service = next((known for known in sorted(SERVICES, key=len, reverse=True)
                            if re.fullmatch(r"(?:[A-Za-z0-9_-]+[-_])?" + re.escape(known)
                                            + r"[-_][0-9]+", service)), "unknown")
        if service not in SERVICE_SET:
            continue
        seen.add(service)
        if first_error is not None or not _ERROR_MARKER.search(message):
            continue
        code_match = _ERROR_CODE.search(message)
        code = code_match.group(1) if code_match and code_match.group(1) in ERROR_CODES else None
        startup_category = next(
            (category for prefix, category in VOICE_STARTUP_PREFIXES if prefix in message),
            "unknown",
        )
        state_match = _SQLSTATE.search(message)
        sqlstate = state_match.group(1).upper() if state_match and state_match.group(1).upper() in SQLSTATES else None
        exception_match = _EXCEPTION.search(message)
        exception = exception_match.group(1) if exception_match and exception_match.group(1) in EXCEPTION_CLASSES else None
        source_match = _SOURCE.search(message)
        source = source_match.group(1) if source_match and source_match.group(1) in SOURCE_BASENAMES else None
        source_line = int(source_match.group(2)) if source and source_match else None
        first_error = {
            "result": "observed_in_bounded_window", "service": service,
            "stage": _stage_for(message, sqlstate, exception),
            "error_code": code or "unknown", "startup_category": startup_category,
            "exception_class": exception or "unknown",
            "sqlstate": sqlstate or "unknown", "source_basename": source or "unknown",
            "source_line": source_line, "selection": "first_marker_in_bounded_head",
            "causal_order": "not_established",
        }
    if first_error is None and result != "complete":
        first_error = _error_record()
    elif first_error is None:
        first_error = _error_record("no_error_marker")
    if result != "complete":
        coverage = "bounded_head_window_only"
    elif set(requested).issubset(seen):
        coverage = "all_requested_observed_in_stream"
    elif seen:
        coverage = "partial_requested_services_observed_in_stream"
    else:
        coverage = "no_requested_service_observed_in_stream"
    return {
        "result": result, "coverage": coverage, "bytes_read": len(data),
        "lines_read": len(lines), "services_requested": list(requested),
        "services_seen": sorted(seen, key=SERVICES.index),
        "truncated": result in {"byte_limit", "line_limit"},
        "window": "first_emitted", "window_bytes_limit": MAX_LOG_BYTES,
        "window_lines_limit": MAX_LOG_LINES, "causal_order": "not_established",
    }, first_error


def _write_record(path: Path, record: dict) -> bool:
    try:
        encoded = (json.dumps(record, sort_keys=True, separators=(",", ":")) + "\n").encode("ascii")
        if len(encoded) > MAX_RECORD_BYTES:
            record = _base_record(record.get("startup_command_exit_status", 1))
            record["status_collection"]["result"] = "output_limit"
            record["log_collection"]["result"] = "output_limit"
            encoded = (json.dumps(record, sort_keys=True, separators=(",", ":")) + "\n").encode("ascii")
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("x", encoding="ascii", newline="\n") as handle:
            handle.write(encoded.decode("ascii"))
        if os.name == "posix":
            os.chmod(path, 0o600)
        return True
    except OSError:
        return False


def build_diagnostics(compose: list[str], startup_status: int, service: str | None = None) -> dict:
    record = _base_record(startup_status)
    if not compose or compose[0] != "docker" or (service is not None and service not in SERVICE_SET):
        record["status_collection"]["result"] = "invalid_invocation"
        record["log_collection"]["result"] = "invalid_invocation"
        return record

    service_args = [service] if service is not None else []
    ps_bytes, ps_result, _ = _bounded_command(compose + ["ps", "--all", "--format", "json", *service_args], COMMAND_TIMEOUT_SECONDS, MAX_PS_BYTES)
    status, failed_services = sanitize_status(ps_bytes, ps_result)
    record["status_collection"] = status

    requested = (service,) if service is not None else (failed_services or SERVICES)
    log_bytes, log_result, _ = _bounded_command(
        compose + ["logs", "--no-color", "--timestamps", *requested],
        COMMAND_TIMEOUT_SECONDS,
        MAX_LOG_BYTES,
    )
    log_summary, first_error = sanitize_logs(log_bytes, log_result, failed_services)
    record["log_collection"] = log_summary
    record["first_error"] = first_error
    return record


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("--output", required=True)
    parser.add_argument("--startup-status", type=int, required=True)
    parser.add_argument("--service", choices=SERVICES)
    parser.add_argument("--compose", nargs=argparse.REMAINDER, required=True)
    args = parser.parse_args(argv)
    compose = args.compose[1:] if args.compose[:1] == ["--"] else args.compose
    output = Path(args.output)
    try:
        record = build_diagnostics(compose, args.startup_status, args.service)
    except Exception:
        record = _base_record(args.startup_status)
        record["status_collection"]["result"] = "collector_error"
        record["log_collection"]["result"] = "collector_error"
        record["first_error"]["result"] = "unknown"
    return 0 if _write_record(output, record) else 1


if __name__ == "__main__":
    raise SystemExit(main())
