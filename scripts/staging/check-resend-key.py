#!/usr/bin/env python3
"""Validate a Kubernetes Secret JSON document without displaying its contents."""

import base64
import binascii
import json
import sys


def valid(document: object, namespace: str) -> bool:
    if not isinstance(document, dict):
        return False
    metadata = document.get("metadata")
    if (
        document.get("kind") != "Secret"
        or not isinstance(metadata, dict)
        or metadata.get("name") != "voice-app-secrets"
        or metadata.get("namespace") != namespace
    ):
        return False

    string_data = document.get("stringData") or {}
    data = document.get("data") or {}
    if not isinstance(string_data, dict) or not isinstance(data, dict):
        return False
    if "AUTH_RESEND_API_KEY" in string_data:
        value = string_data["AUTH_RESEND_API_KEY"]
    else:
        encoded = data.get("AUTH_RESEND_API_KEY")
        if not isinstance(encoded, str):
            return False
        try:
            value = base64.b64decode(encoded, validate=True).decode("utf-8")
        except (binascii.Error, UnicodeDecodeError):
            return False
    return isinstance(value, str) and bool(value.strip())


def main() -> int:
    try:
        document = json.load(sys.stdin)
    except (json.JSONDecodeError, UnicodeDecodeError):
        return 1
    return 0 if valid(document, sys.argv[1]) else 1


if __name__ == "__main__":
    sys.exit(main())
