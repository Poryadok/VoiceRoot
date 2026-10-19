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
        # A matching public document does not establish private-key usability.
        # Validate imported/reused pairs as strictly as native signer startup.
        subprocess.run(
            ["openssl", "pkey", "-in", f"{directory}/{kid}.pem", "-check", "-noout"],
            check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            timeout=10,
        )
        modulus = subprocess.check_output(
            ["openssl", "rsa", "-in", f"{directory}/{kid}.pem", "-noout", "-modulus"],
            text=True,
        ).strip().removeprefix("Modulus=")
        if int(modulus, 16).bit_length() < 2048:
            raise ValueError("RSA signing key must have at least 2048 bits")
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


# Validate both issuers before publishing any part of this static generation.
documents = {service: document(f"/{service}-signing") for service in ("space", "realtime")}
for service, expected in documents.items():
    destination = Path(f"/jwks-public/{service}.json")
    if "--check" in sys.argv:
        if destination.read_text() != expected:
            raise ValueError(f"incomplete public JWKS bootstrap: {service}")
    else:
        with destination.open("x") as output:
            output.write(expected)
        os.chmod(destination, 0o644)
os.chmod("/jwks-public", 0o755)
