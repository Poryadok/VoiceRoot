#!/usr/bin/env python3
"""Build a two-key Kubernetes Secret merge patch without printing source data."""

import base64
import binascii
import json
import sys


MAIL_KEYS = ("AUTH_RESEND_API_KEY", "AUTH_RESEND_FROM")


def build_patch(document: object, namespace: str) -> dict | None:
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
    encoded_patch = {}
    for key in MAIL_KEYS:
        if key in string_data:
            value = string_data[key]
        elif key in data and isinstance(data[key], str):
            try:
                value = base64.b64decode(data[key], validate=True).decode("utf-8")
            except (binascii.Error, UnicodeDecodeError):
                return None
        else:
            return None
        if not isinstance(value, str) or not value.strip():
            return None
        encoded_patch[key] = base64.b64encode(value.encode("utf-8")).decode("ascii")
    return {"data": encoded_patch}


def main() -> int:
    try:
        document = json.load(sys.stdin)
    except (json.JSONDecodeError, UnicodeDecodeError):
        print("invalid mail Secret document", file=sys.stderr)
        return 1
    patch = build_patch(document, sys.argv[1])
    if patch is None:
        print("mail Secret must contain nonblank AUTH_RESEND_API_KEY and AUTH_RESEND_FROM", file=sys.stderr)
        return 1
    json.dump(patch, sys.stdout, separators=(",", ":"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
