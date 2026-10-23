#!/usr/bin/env python3
"""Create short-lived local/CI Phase0 credentials; never provision a live cluster."""

import argparse
import base64
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


def generate(destination):
    destination = Path(destination).absolute()
    if destination.exists() or destination.is_symlink():
        raise ValueError("fixture destination must not exist")
    binaries = {name: shutil.which(name) for name in ("openssl", "keytool")}
    if not all(binaries.values()):
        raise ValueError("openssl and keytool are required")
    if not destination.parent.is_dir():
        raise ValueError("fixture parent directory must exist")

    def command(tool, *args):
        # Error output is deliberately withheld: tools can echo sensitive input.
        result = subprocess.run([binaries[tool], *map(str, args)],
                                capture_output=True, timeout=60)
        if result.returncode:
            raise ValueError(f"{tool} fixture generation failed")

    old_umask = os.umask(0o077)
    try:
        with tempfile.TemporaryDirectory(prefix=".phase0-", dir=destination.parent) as scratch:
            root = Path(scratch)
            for directory in ("gateway", "space", "chat", "role", "matchmaking", "gameintegration", "voice", "bot", "jwks", "tls", "ca"):
                (root / directory).mkdir(mode=0o700)
            for issuer in ("gateway", "chat", "matchmaking"):
                (root / "jwks" / issuer).mkdir(mode=0o700)
            for issuer in ("gateway", "space", "chat", "role", "matchmaking"):
                for kid in ("current", "next"):
                    command("openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt",
                            "rsa_keygen_bits:2048", "-out", root / issuer / f"{kid}.pem")

            def der_value(data, offset, expected_tag):
                if offset >= len(data) or data[offset] != expected_tag:
                    raise ValueError("invalid RSA public key encoding")
                offset += 1
                if offset >= len(data):
                    raise ValueError("truncated RSA public key encoding")
                length = data[offset]
                offset += 1
                if length & 0x80:
                    count = length & 0x7f
                    if not count or offset + count > len(data):
                        raise ValueError("invalid RSA public key length")
                    length = int.from_bytes(data[offset:offset + count], "big")
                    offset += count
                end = offset + length
                if end > len(data):
                    raise ValueError("truncated RSA public key value")
                return data[offset:end], end

            def b64url(value):
                return base64.urlsafe_b64encode(value).decode("ascii").rstrip("=")

            for issuer in ("gateway", "chat", "matchmaking"):
                issuer_keys = []
                for kid in ("current", "next"):
                    der = subprocess.run(
                        [binaries["openssl"], "pkey", "-in", str(root / issuer / f"{kid}.pem"),
                         "-pubout", "-outform", "DER"],
                        capture_output=True, timeout=60, check=False)
                    if der.returncode:
                        raise ValueError(f"{issuer} public key export failed")
                    spki, end = der_value(der.stdout, 0, 0x30)
                    if end != len(der.stdout):
                        raise ValueError("trailing RSA public key data")
                    _, offset = der_value(spki, 0, 0x30)
                    bit_string, offset = der_value(spki, offset, 0x03)
                    if offset != len(spki) or not bit_string or bit_string[0] != 0:
                        raise ValueError("invalid RSA public key bit string")
                    rsa_key, end = der_value(bit_string[1:], 0, 0x30)
                    if end != len(bit_string) - 1:
                        raise ValueError("trailing RSA key data")
                    modulus, offset = der_value(rsa_key, 0, 0x02)
                    exponent, offset = der_value(rsa_key, offset, 0x02)
                    if offset != len(rsa_key):
                        raise ValueError("trailing RSA integer data")
                    modulus = modulus.lstrip(b"\x00")
                    exponent = exponent.lstrip(b"\x00")
                    issuer_keys.append({
                        "kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
                        "n": b64url(modulus), "e": b64url(exponent),
                    })
                (root / "jwks" / issuer / "jwks.json").write_text(
                    json.dumps({"keys": issuer_keys}, separators=(",", ":")) + "\n",
                    encoding="ascii")
            for issuer in ("gameintegration", "voice", "bot"):
                for kid in ("current", "next"):
                    command("openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt",
                            "rsa_keygen_bits:2048", "-out", root / issuer / f"{kid}.pem")
            ca_key, ca_cert = root / "ca/ca.key", root / "ca/ca.crt"
            command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                    "-keyout", ca_key, "-out", ca_cert, "-days", "2",
                    "-subj", "/CN=Voice Phase0 local fixture CA",
                    "-addext", "basicConstraints=critical,CA:TRUE",
                    "-addext", "keyUsage=critical,keyCertSign,cRLSign")
            gis_ca_key, gis_ca_cert = root / "ca/gameintegration-client-ca.key", root / "ca/gameintegration-client-ca.crt"
            command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                    "-keyout", gis_ca_key, "-out", gis_ca_cert, "-days", "2",
                    "-subj", "/CN=Voice Phase0 GIS client CA",
                    "-addext", "basicConstraints=critical,CA:TRUE",
                    "-addext", "keyUsage=critical,keyCertSign,cRLSign")
            lifecycle_ca_key = root / "ca/space-lifecycle-client-ca.key"
            lifecycle_ca_cert = root / "ca/space-lifecycle-client-ca.crt"
            command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                    "-keyout", lifecycle_ca_key, "-out", lifecycle_ca_cert, "-days", "2",
                    "-subj", "/CN=Voice Phase0 Space lifecycle client CA",
                    "-addext", "basicConstraints=critical,CA:TRUE",
                    "-addext", "keyUsage=critical,keyCertSign,cRLSign")
            for index, (leaf, hostname) in enumerate(
                    (("role", "role"), ("auth", "auth"), ("proxy", "phase0-jwks"),
                     ("matchmaking-complete-grpc", "matchmaking"),
                     ("matchmaking-jwks", "voice-matchmaking"),
                     ("gameintegration-jwks", "gameintegration"), ("voice-jwks", "voice"),
                     ("chat-gis-grpc", "chat"), ("voice-game-grpc", "voice"),
                     ("chat-match-squad-grpc", "chat"), ("voice-match-squad-grpc", "voice"),
                     ("voice-match-squad-member-grpc", "voice"),
                     ("space-lifecycle", "space"), ("bot-game-event", "bot"),
                     ("notification-lifecycle", "notification"),
                     ("messaging-gameintegration-grpc", "messaging"),
                     ("chat-create-grpc", "chat"), ("chat-jwks", "chat"),
                     ("space-chat-composite-grpc", "space"), ("space-audit-grpc", "space")), 1):
                key, csr = root / f"tls/{leaf}.key", root / f"tls/{leaf}.csr"
                extensions = root / "leaf.ext"
                extensions.write_text(
                    "basicConstraints=critical,CA:FALSE\n"
                    "keyUsage=critical,digitalSignature,keyEncipherment\n"
                    "extendedKeyUsage=serverAuth\n"
                    f"subjectAltName=DNS:{hostname}\n", encoding="ascii")
                command("openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes",
                        "-keyout", key, "-out", csr, "-subj", f"/CN={hostname}")
                command("openssl", "x509", "-req", "-in", csr, "-CA", ca_cert,
                        "-CAkey", ca_key, "-set_serial", str(index), "-days", "2",
                        "-extfile", extensions, "-out", root / f"tls/{leaf}.crt")
                csr.unlink()
            extensions.unlink()
            for serial, service in enumerate(("space", "gameintegration", "voice", "matchmaking", "chat", "auth-jwks", "role", "bot"), 100):
                client_key = root / f"tls/{service}-client.key"
                client_csr = root / f"tls/{service}-client.csr"
                client_extensions = root / "client.ext"
                client_extensions.write_text(
                    "basicConstraints=critical,CA:FALSE\n"
                    "keyUsage=critical,digitalSignature\n"
                    "extendedKeyUsage=clientAuth\n"
                    + ("subjectAltName=URI:spiffe://voice.local/phase0/bot\n" if service == "bot" else ""),
                    encoding="ascii")
                command("openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes",
                        "-keyout", client_key, "-out", client_csr, "-subj", f"/CN={service}")
                command("openssl", "x509", "-req", "-in", client_csr, "-CA", ca_cert,
                        "-CAkey", ca_key, "-set_serial", str(serial), "-days", "2",
                        "-extfile", client_extensions, "-out", root / f"tls/{service}-client.crt")
                client_csr.unlink()
                client_extensions.unlink()
            scoped_clients = [
                (f"gameintegration-{service}-client", "gameintegration", gis_ca_cert, gis_ca_key)
                for service in ("chat", "voice", "messaging")
            ]
            scoped_clients.append(("gateway-client", "gateway", ca_cert, ca_key))
            scoped_clients.append(("space-lifecycle-client", "space", lifecycle_ca_cert, lifecycle_ca_key))
            # Chat's Gateway/Bot JWKS HTTP transport has its own identity. Bot's
            # existing protected JWKS endpoint trusts the GIS client CA; owner
            # RPCs use the independent root-CA Chat identity above.
            scoped_clients.append(("chat-jwks-client", "chat", gis_ca_cert, gis_ca_key))
            for serial, (leaf, service, client_ca_cert, client_ca_key) in enumerate(scoped_clients, 200):
                client_key = root / f"tls/{leaf}.key"
                client_csr = root / f"tls/{leaf}.csr"
                client_extensions = root / "client.ext"
                client_extensions.write_text(
                    "basicConstraints=critical,CA:FALSE\n"
                    "keyUsage=critical,digitalSignature\n"
                    "extendedKeyUsage=clientAuth\n", encoding="ascii")
                command("openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes",
                        "-keyout", client_key, "-out", client_csr, "-subj", f"/CN={service}")
                command("openssl", "x509", "-req", "-in", client_csr, "-CA", client_ca_cert,
                        "-CAkey", client_ca_key, "-set_serial", str(serial), "-days", "2",
                        "-extfile", client_extensions, "-out", root / f"tls/{leaf}.crt")
                client_csr.unlink()
                client_extensions.unlink()
            # Auth's default Java HttpClient uses the JSSE client keystore for
            # Chat's mTLS JWKS endpoint. A truststore alone supplies no identity.
            command("openssl", "pkcs12", "-export", "-name", "auth-jwks-client",
                    "-inkey", root / "tls/auth-jwks-client.key",
                    "-in", root / "tls/auth-jwks-client.crt", "-certfile", ca_cert,
                    "-out", root / "auth-jwks-client.p12", "-passout", "pass:phase0-fixture")
            # Bot publishes keys to both the existing GIS consumers and the
            # native Space/Role/Auth installation consumers. RPC listener CA
            # scopes remain independent; this bundle is only for JWKS HTTPS.
            (root / "ca/bot-jwks-client-ca.crt").write_bytes(ca_cert.read_bytes() + gis_ca_cert.read_bytes())
            ca_key.unlink()
            gis_ca_key.unlink()
            lifecycle_ca_key.unlink()
            command("keytool", "-importcert", "-noprompt", "-alias", "phase0-fixture-ca",
                    "-file", ca_cert, "-keystore", root / "truststore.p12",
                    "-storetype", "PKCS12", "-storepass", "phase0-fixture")
            for path in root.rglob("*"):
                path.chmod(0o700 if path.is_dir() else 0o600)
            # This JWKS is public and bind-mounted into nginx, whose worker
            # drops root privileges. Make only the public document readable
            # by that worker; keep private fixture keys owner-only.
            for issuer in ("gateway", "chat", "matchmaking"):
                (root / "jwks" / issuer / "jwks.json").chmod(0o644)
            fixture_uid = os.environ.get("PHASE0_MATCHSQUAD_FIXTURE_UID")
            if fixture_uid is not None:
                if os.name != "posix" or fixture_uid != "65532":
                    raise ValueError("unsupported MatchSquad fixture owner")
                owned_paths = [
                    root / "matchmaking",
                    root / "tls/chat-match-squad-grpc.crt",
                    root / "tls/chat-match-squad-grpc.key",
                    root / "tls/voice-match-squad-grpc.crt",
                    root / "tls/voice-match-squad-grpc.key",
                    root / "tls/voice-match-squad-member-grpc.crt",
                    root / "tls/voice-match-squad-member-grpc.key",
                    root / "tls/matchmaking-client.crt",
                    root / "tls/matchmaking-client.key",
                    root / "tls/matchmaking-complete-grpc.crt",
                    root / "tls/matchmaking-complete-grpc.key",
                    root / "tls/matchmaking-jwks.crt",
                    root / "tls/matchmaking-jwks.key",
                    root / "tls/space-lifecycle.crt",
                    root / "tls/space-lifecycle.key",
                    root / "tls/space-client.crt",
                    root / "tls/space-client.key",
                    root / "ca/ca.crt",
                ]
                # Permit path traversal to only the owner-only files below;
                # execute-only directories do not permit listing or reading
                # any of the remaining fixture material.
                for path in (root, root / "tls", root / "ca"):
                    path.chmod(0o711)
                if os.geteuid() == 0:
                    for path in owned_paths:
                        entries = (path, *path.rglob("*")) if path == root / "matchmaking" else (path,)
                        for entry in entries:
                            os.chown(entry, 65532, 65532)
                else:
                    for path in owned_paths:
                        command_args = ["sudo", "-n", "chown"]
                        if path == root / "matchmaking":
                            command_args.append("-R")
                        command_args.extend(("65532:65532", str(path)))
                        result = subprocess.run(
                            command_args,
                            capture_output=True, timeout=30, check=False)
                        if result.returncode:
                            raise ValueError("MatchSquad fixture ownership setup failed")
            # No existing output is overwritten; a failed tool leaves only the
            # temporary tree, whose context manager removes all key material.
            root.rename(destination)
    finally:
        os.umask(old_umask)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", help="new directory under an existing local temp parent")
    args = parser.parse_args()
    try:
        generate(args.destination)
    except (ValueError, OSError, subprocess.TimeoutExpired):
        print("Phase0 fixture generation failed; destination must be new and crypto tools available.",
              file=sys.stderr)
        return 1
    print("Created local Phase0 fixture (expires in two days).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
