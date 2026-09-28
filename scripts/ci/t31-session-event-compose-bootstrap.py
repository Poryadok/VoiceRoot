#!/usr/bin/env python3
"""Bootstrap disposable T31 identities and a scoped session in local Compose."""

import argparse
import json
import os
from pathlib import Path
import re
import secrets
import string
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


def api(method, url, body=None, token=None, idempotency_key=None):
    headers = {"Accept": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if idempotency_key:
        headers["Idempotency-Key"] = idempotency_key
    data = None
    if body is not None:
        data = json.dumps(body, separators=(",", ":")).encode()
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            raw = response.read()
            return response.status, json.loads(raw) if raw else None
    except urllib.error.HTTPError as exc:
        raw = exc.read(4096)
        try:
            parsed = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            parsed = {}
        return exc.code, parsed


def compose(args, *command, check=True, capture=False):
    cmd = [
        "docker", "compose", "--env-file", str(args.phase0_env),
        "--env-file", str(args.compose_env), "--profile", "app",
        "-f", "docker-compose.yml", "-f", "docker-compose.phase0.yml",
        "-f", "docker-compose.t31-session-events-e2e.yml", "-p", args.project,
        *command,
    ]
    result = subprocess.run(cmd, check=False, capture_output=capture, text=True)
    if check and result.returncode:
        raise RuntimeError(f"docker compose {command[0]} failed with exit {result.returncode}")
    return result


def clear_local_auth_limits(args):
    for pattern in (
        "ratelimit:AuthLogin:*", "ratelimit:AuthRegister:*", "ratelimit:Auth:*",
        "ratelimit:OTP:*", "auth:otp:*",
    ):
        result = compose(args, "exec", "-T", "redis", "redis-cli", "--scan", "--pattern", pattern, capture=True)
        for key in result.stdout.splitlines():
            key = key.strip()
            if key:
                compose(args, "exec", "-T", "redis", "redis-cli", "DEL", key, capture=True)


def verified_account(args, label):
    clear_local_auth_limits(args)
    suffix = uuid.uuid4().hex
    email = f"t31-{label}-{suffix}@voice-qa.test"
    password = "T31!" + uuid.uuid4().hex + "aA9"
    base = args.gateway_base.rstrip("/")
    status, registration = api("POST", f"{base}/api/v1/auth/register", {
        "email": email, "password": password, "guest": False,
        "device_info_json": '{"platform":"t31-compose-e2e"}',
    })
    session = registration.get("session") if registration else None
    if status != 200 or not session or not session.get("access_token"):
        raise RuntimeError(f"Auth registration failed with HTTP {status}")

    clear_local_auth_limits(args)
    status, _ = api("POST", f"{base}/api/v1/auth/otp/send", {
        "email": email, "otp_type": "email_verify",
    }, token=session["access_token"])
    if status != 204:
        raise RuntimeError(f"Auth OTP send failed with HTTP {status}")
    mail_url = args.mail_stub_base.rstrip("/") + "/emails/latest?" + urllib.parse.urlencode({"to": email})
    code = None
    for _ in range(100):
        try:
            with urllib.request.urlopen(mail_url, timeout=5) as response:
                message = json.loads(response.read())
            if email in message.get("to", []):
                match = re.search(r"\b(\d{6})\b", message.get("text", ""))
                if match:
                    code = match.group(1)
                    break
        except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError, json.JSONDecodeError):
            pass
        time.sleep(0.1)
    if not code:
        raise RuntimeError("verification stub did not deliver an OTP for a disposable account")

    status, verified = api("POST", f"{base}/api/v1/auth/otp/verify", {
        "email": email, "code": code, "otp_type": "email_verify",
    }, token=session["access_token"])
    regular = verified.get("session") if verified else None
    if status in (202, 204, 503):
        regular = session
        for _ in range(24):
            time.sleep(2)
            _, refreshed = api("POST", f"{base}/api/v1/auth/refresh", {
                "refresh_token": regular["refresh_token"],
                "device_info_json": '{"platform":"t31-compose-e2e"}',
            })
            current = refreshed.get("session") if refreshed else None
            if current and current.get("account_type") == "regular":
                regular = current
                break
    if not regular or regular.get("account_type") != "regular" or not regular.get("access_token"):
        raise RuntimeError(f"Auth email verification did not produce a regular session (HTTP {status})")
    return regular


def sandbox_app(args, name, applicant_token, operator_token):
    base = args.gateway_base.rstrip("/")
    status, created = api("POST", f"{base}/api/v1/game-integrations/applications", {"name": name},
                          applicant_token, str(uuid.uuid4()))
    if status != 201:
        raise RuntimeError(f"sandbox application creation failed with HTTP {status}")
    app_id = created["application_id"]
    status, environment = api(
        "POST", f"{base}/api/v1/game-integrations/applications/{app_id}/admissions/sandbox", None,
        operator_token, str(uuid.uuid4()),
    )
    if status != 201:
        raise RuntimeError(f"sandbox admission failed with HTTP {status}")
    environment_id = environment["environment_id"]
    status, credential = api(
        "POST", f"{base}/api/v1/game-integrations/applications/{app_id}/environments/{environment_id}/credentials",
        {"scopes": ["game.sessions.manage"]}, applicant_token, str(uuid.uuid4()),
    )
    if status != 201 or credential.get("scopes") != ["game.sessions.manage"] or not credential.get("secret", "").startswith("vgi1_"):
        raise RuntimeError(f"game.sessions.manage credential issue failed with HTTP {status}")
    return {
        "application_id": app_id, "environment_id": environment_id,
        "credential_id": credential["credential_id"], "secret": credential["secret"],
    }


def write_compose_env(path, tls_dir, operator_id, app, other, event_id="bootstrap-pending"):
    values = {
        "T31_E2E_TLS_DIR": str(tls_dir.resolve()),
        "T31_E2E_OPERATOR_ACCOUNT_ID": operator_id,
        "T31_APPLICATION_ID": app["application_id"],
        "T31_ENVIRONMENT_ID": app["environment_id"],
        "T31_GAME_SERVER_CREDENTIAL": app["secret"],
        "T31_OTHER_APPLICATION_ID": other["application_id"],
        "T31_OTHER_ENVIRONMENT_ID": other["environment_id"],
        "T31_OTHER_GAME_SERVER_CREDENTIAL": other["secret"],
        "T31_EXPECTED_EVENT_ID": event_id,
    }
    path.write_text("".join(f"{key}={value}\n" for key, value in values.items()), encoding="ascii")
    os.chmod(path, 0o600)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", required=True)
    parser.add_argument("--phase0-env", type=Path, required=True)
    parser.add_argument("--compose-env", type=Path, required=True)
    parser.add_argument("--tls-dir", type=Path, required=True)
    parser.add_argument("--result", type=Path, required=True)
    parser.add_argument("--gateway-base", default="http://127.0.0.1:18080")
    parser.add_argument("--mail-stub-base", default="http://127.0.0.1:14180")
    args = parser.parse_args()

    applicant = verified_account(args, "applicant")
    operator = verified_account(args, "operator")
    if applicant["account_id"] == operator["account_id"] or applicant["profile_id"] == operator["profile_id"]:
        raise RuntimeError("applicant and operator identities must be distinct")

    # Compose changes only GIS and rebuilds it from this checkout so the operator
    # allowlist and T31 routes are present without recreating its PostgreSQL DB.
    write_compose_env(args.compose_env, args.tls_dir, operator["account_id"],
                      {"application_id": "pending", "environment_id": "pending", "secret": "pending"},
                      {"application_id": "pending", "environment_id": "pending", "secret": "pending"})
    compose(args, "up", "-d", "--no-deps", "--build", "--force-recreate", "gameintegration")
    for _ in range(60):
        health = subprocess.run(
            ["docker", "inspect", "--format", "{{.State.Health.Status}}", f"{args.project}-gameintegration-1"],
            capture_output=True, text=True, check=False,
        )
        if health.returncode == 0 and health.stdout.strip() == "healthy":
            break
        time.sleep(1)
    else:
        raise RuntimeError("GIS did not become healthy after the targeted rebuild")

    app = sandbox_app(args, f"T31 HTTPS receiver {uuid.uuid4().hex[:8]}", applicant["access_token"], operator["access_token"])
    other = sandbox_app(args, f"T31 HTTPS isolation {uuid.uuid4().hex[:8]}", applicant["access_token"], operator["access_token"])
    write_compose_env(args.compose_env, args.tls_dir, operator["account_id"], app, other)
    result = {
        "applicant_account_id": applicant["account_id"], "applicant_profile_id": applicant["profile_id"],
        "operator_account_id": operator["account_id"], "operator_profile_id": operator["profile_id"],
        "application_id": app["application_id"], "environment_id": app["environment_id"],
        "credential_id": app["credential_id"], "other_application_id": other["application_id"],
        "other_environment_id": other["environment_id"], "other_credential_id": other["credential_id"],
    }
    args.result.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    os.chmod(args.result, 0o600)

    operation_id = str(uuid.uuid4())
    payload = {
        "operation_id": operation_id, "kind": "party", "external_key": f"t31-e2e-{operation_id}",
        "display_name": "T31 local HTTPS receiver", "roster_revision": 1,
        "roster_complete": True, "members": [applicant["profile_id"], operator["profile_id"]],
    }
    status, _ = api("POST", args.gateway_base.rstrip("/") + "/api/v1/sessions", payload, app["secret"])
    if status != 202:
        raise RuntimeError(f"GIS session create failed with HTTP {status}")
    operation = None
    for _ in range(90):
        status, operation = api("GET", args.gateway_base.rstrip("/") + f"/api/v1/operations/{operation_id}", token=app["secret"])
        if status != 200:
            raise RuntimeError(f"GIS operation read failed with HTTP {status}")
        if operation.get("session_status") == "active" and operation.get("active_event_id"):
            break
        time.sleep(1)
    if not operation or operation.get("session_status") != "active" or not operation.get("active_event_id"):
        stage = operation.get("stage") if operation else "unavailable"
        raise RuntimeError(f"GIS session did not become active; stage={stage}")

    write_compose_env(args.compose_env, args.tls_dir, operator["account_id"], app, other, operation["active_event_id"])
    result.update({
        "operation_id": operation_id, "session_id": operation["session_id"],
        "active_event_id": operation["active_event_id"], "session_status": operation["session_status"],
        "operation_status": operation["status"], "stage": operation["stage"],
    })
    args.result.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    os.chmod(args.result, 0o600)
    print("T31 bootstrap complete: " + " ".join(f"{key}={result[key]}" for key in (
        "applicant_account_id", "applicant_profile_id", "operator_account_id", "operator_profile_id",
        "application_id", "environment_id", "credential_id", "other_application_id", "other_environment_id",
        "operation_id", "session_id", "active_event_id", "session_status", "stage",
    )))


if __name__ == "__main__":
    main()
