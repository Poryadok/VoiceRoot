#!/usr/bin/env python3
"""Mail-only patch contains precisely the two Auth mail fields."""

import base64
import importlib.util
from pathlib import Path
import secrets
import subprocess
import sys

import yaml


path = Path(__file__).with_name("mail-only-patch.py")
spec = importlib.util.spec_from_file_location("mail_only_patch", path)
assert spec and spec.loader
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

key = secrets.token_hex(24)
sender = "Voice <sender@example.invalid>"
document = {
    "kind": "Secret",
    "metadata": {"name": "voice-app-secrets", "namespace": "voice-staging"},
    "stringData": {
        "AUTH_RESEND_API_KEY": key,
        "AUTH_RESEND_FROM": sender,
        "POSTGRES_PASSWORD": secrets.token_hex(24),
    },
}
patch = module.build_patch(document, "voice-staging")
assert set(patch) == {"data"}
assert set(patch["data"]) == {"AUTH_RESEND_API_KEY", "AUTH_RESEND_FROM"}
assert base64.b64decode(patch["data"]["AUTH_RESEND_API_KEY"]).decode() == key
assert base64.b64decode(patch["data"]["AUTH_RESEND_FROM"]).decode() == sender

for missing in ("AUTH_RESEND_API_KEY", "AUTH_RESEND_FROM"):
    broken = {**document, "stringData": {**document["stringData"], missing: ""}}
    assert module.build_patch(broken, "voice-staging") is None

for sender_format in (
    "not-an-address",
    "Voice <sender@>",
    "Voice <sender@example.invalid> extra",
    "Injected\n<sender@example.invalid>",
    "Injected\r<sender@example.invalid>",
    "Injected\x00<sender@example.invalid>",
):
    broken = {**document, "stringData": {**document["stringData"], "AUTH_RESEND_FROM": sender_format}}
    assert module.build_patch(broken, "voice-staging") is None

encoded = {
    **document,
    "stringData": {},
    "data": {
        "AUTH_RESEND_API_KEY": base64.b64encode(key.encode()).decode(),
        "AUTH_RESEND_FROM": base64.b64encode(sender.encode()).decode(),
    },
}
assert set(module.build_patch(encoded, "voice-staging")["data"]) == {
    "AUTH_RESEND_API_KEY", "AUTH_RESEND_FROM"
}
assert module.build_patch({**document, "metadata": {"name": "wrong", "namespace": "voice-staging"}}, "voice-staging") is None

checked = subprocess.run(
    [sys.executable, str(path), "voice-staging", "--yaml-check"],
    input=yaml.safe_dump(document), text=True, capture_output=True, check=False,
)
assert checked.returncode == 0 and checked.stdout.strip() == "MAIL_ONLY_INPUT=PASS"
assert key not in checked.stdout + checked.stderr
print("Mail-only patch contract passed.")
