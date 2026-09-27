#!/usr/bin/env python3
"""Validate a Kubernetes Secret JSON document without displaying its contents."""

import base64
import binascii
import json
import pathlib
import sys

REQUIRED_KEYS = frozenset(
    """ACCOUNT_DELETE_TOKEN_SECRET ANALYTICS_ID_HASH_KEY AUTH_JWT_PRIVATE_KEY
    AUTH_RESEND_API_KEY AUTH_TOTP_ENCRYPTION_KEY BOT_DATABASE_URL CHAT_DATABASE_URL
    CLICKHOUSE_DSN CLICKHOUSE_PASSWORD FILE_DATABASE_URL FILE_R2_BUCKET
    FILE_R2_ENDPOINT FILE_R2_REGION GATEWAY_DATABASE_URL LIVEKIT_API_KEY
    LIVEKIT_API_SECRET MATCHMAKING_DATABASE_URL MESSAGING_DATABASE_URL
    MODERATION_DATABASE_URL NOTIFICATION_DATABASE_URL POSTGRES_PASSWORD
    ROLE_DATABASE_URL SEARCH_DATABASE_URL SOCIAL_DATABASE_URL SPACE_DATABASE_URL
    STORY_DATABASE_URL SUBSCRIPTION_DATABASE_URL USER_DATABASE_URL USER_R2_BUCKET
    USER_R2_ENDPOINT USER_R2_PUBLIC_BASE_URL USER_R2_REGION VOICE_DATABASE_URL""".split()
)
# Endpoint and public URL can intentionally be empty; their Secret keys must exist.
ALLOW_EMPTY = frozenset({"USER_R2_ENDPOINT", "FILE_R2_ENDPOINT", "USER_R2_PUBLIC_BASE_URL"})


def missing_keys(document: object, namespace: str) -> list[str] | None:
    if not isinstance(document, dict):
        return None
    metadata = document.get("metadata")
    if (
        document.get("kind") != "Secret"
        or not isinstance(metadata, dict)
        or metadata.get("name") != "voice-app-secrets"
        or metadata.get("namespace") != namespace
    ):
        return None

    string_data = document.get("stringData") or {}
    data = document.get("data") or {}
    if not isinstance(string_data, dict) or not isinstance(data, dict):
        return None
    missing = []
    for key in sorted(REQUIRED_KEYS):
        if key in string_data:
            value = string_data[key]
        elif key in data:
            encoded = data[key]
            if not isinstance(encoded, str):
                missing.append(key)
                continue
            try:
                value = base64.b64decode(encoded, validate=True).decode("utf-8")
            except (binascii.Error, UnicodeDecodeError):
                missing.append(key)
                continue
        else:
            missing.append(key)
            continue
        if not isinstance(value, str) or (key not in ALLOW_EMPTY and not value.strip()):
            missing.append(key)
    return missing


def valid(document: object, namespace: str) -> bool:
    return missing_keys(document, namespace) == []


def plan_upload(upload: object, live: object | None, namespace: str) -> tuple[list[str], list[dict] | None] | None:
    """Validate the effective Secret and plan a guarded patch of uploaded keys only."""
    if missing_keys(upload, namespace) is None:
        return None
    if live is not None and missing_keys(live, namespace) is None:
        return None
    upload_data = upload.get("data") or {}
    string_data = upload.get("stringData") or {}
    if set(upload_data).intersection(string_data):
        return None
    encoded_upload = dict(upload_data)
    for key, value in encoded_upload.items():
        if not isinstance(key, str) or not isinstance(value, str):
            return None
        try:
            base64.b64decode(value, validate=True)
        except binascii.Error:
            return None
    for key, value in string_data.items():
        if not isinstance(key, str) or not isinstance(value, str):
            return None
        encoded_upload[key] = base64.b64encode(value.encode("utf-8")).decode("ascii")
    if not encoded_upload:
        return None
    effective = {
        "kind": "Secret",
        "metadata": {"name": "voice-app-secrets", "namespace": namespace},
        "data": dict((live or {}).get("data") or {}),
    }
    effective["data"].update(encoded_upload)
    missing = missing_keys(effective, namespace)
    if missing:
        return missing, None
    if live is None:
        return [], None
    version = live["metadata"].get("resourceVersion")
    if not isinstance(version, str) or not version:
        return None
    patch = [{"op": "test", "path": "/metadata/resourceVersion", "value": version}]
    for key, value in sorted(encoded_upload.items()):
        pointer = key.replace("~", "~0").replace("/", "~1")
        patch.append({"op": "add", "path": "/data/" + pointer, "value": value})
    return [], patch


def main() -> int:
    if len(sys.argv) == 4 and sys.argv[2] in {"--merge-check", "--patch"}:
        try:
            upload = json.load(sys.stdin)
            live_text = pathlib.Path(sys.argv[3]).read_text(encoding="utf-8")
            live = json.loads(live_text) if live_text.strip() else None
        except (json.JSONDecodeError, UnicodeDecodeError, OSError):
            print("invalid Secret document")
            return 1
        result = plan_upload(upload, live, sys.argv[1])
        if result is None:
            print("invalid Secret identity or data")
            return 1
        missing, patch = result
        if missing:
            print("missing or blank required keys: " + ", ".join(missing))
            return 1
        if sys.argv[2] == "--patch":
            if patch is None:
                print("live Secret is required for patch")
                return 1
            json.dump(patch, sys.stdout, separators=(",", ":"))
        return 0
    if sys.argv[2:] == ["--yaml"]:
        import yaml

        try:
            documents = list(yaml.safe_load_all(sys.stdin))
        except (yaml.YAMLError, UnicodeDecodeError):
            print("invalid Secret document")
            return 1
        document = documents[0] if len(documents) == 1 else None
    else:
        try:
            document = json.load(sys.stdin)
        except (json.JSONDecodeError, UnicodeDecodeError):
            print("invalid Secret document")
            return 1
    missing = missing_keys(document, sys.argv[1])
    if missing is None:
        print("invalid Secret identity or data")
        return 1
    if missing:
        print("missing or blank required keys: " + ", ".join(missing))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
