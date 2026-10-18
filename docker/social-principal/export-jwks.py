"""Local bootstrap only: expose public rotation keys independently of app startup."""
import base64
import json
import os
import re
from pathlib import Path
import subprocess
import sys


def document(directory):
    keys = []
    for kid in ("current", "next"):
        modulus = subprocess.check_output(
            ["openssl", "rsa", "-in", f"{directory}/{kid}.pem", "-noout", "-modulus"],
            text=True,
        ).strip().removeprefix("Modulus=")
        encoded = base64.urlsafe_b64encode(bytes.fromhex(modulus)).decode().rstrip("=")
        public = subprocess.check_output(
            ["openssl", "pkey", "-in", f"{directory}/{kid}.pem", "-pubout", "-text_pub", "-noout"],
            text=True,
        )
        exponent = int(re.search(r"^Exponent: ([0-9]+)", public, re.MULTILINE)[1])
        encoded_exponent = base64.urlsafe_b64encode(
            exponent.to_bytes((exponent.bit_length() + 7) // 8, "big")
        ).decode().rstrip("=")
        keys.append(dict(kty="RSA", kid=kid, use="sig", alg="RS256", n=encoded, e=encoded_exponent))
    if keys[0]["n"] == keys[1]["n"]:
        raise ValueError("duplicate rotation keys")
    return json.dumps({"keys": keys}, separators=(",", ":")) + "\n"


for service in ("space", "realtime"):
    destination = Path(f"/jwks-public/{service}.json")
    expected = document(f"/{service}-signing")
    if "--check" in sys.argv:
        if destination.read_text() != expected:
            raise ValueError(f"incomplete public JWKS bootstrap: {service}")
    else:
        with destination.open("x") as output:
            output.write(expected)
        os.chmod(destination, 0o644)
os.chmod("/jwks-public", 0o755)
