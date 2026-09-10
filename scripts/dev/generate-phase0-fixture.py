#!/usr/bin/env python3
"""Create short-lived local/CI Phase0 credentials; never provision a live cluster."""

import argparse
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
            for directory in ("gateway", "space", "tls", "ca"):
                (root / directory).mkdir(mode=0o700)
            for issuer in ("gateway", "space"):
                for kid in ("current", "next"):
                    command("openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt",
                            "rsa_keygen_bits:2048", "-out", root / issuer / f"{kid}.pem")
            ca_key, ca_cert = root / "ca/ca.key", root / "ca/ca.crt"
            command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                    "-keyout", ca_key, "-out", ca_cert, "-days", "2",
                    "-subj", "/CN=Voice Phase0 local fixture CA",
                    "-addext", "basicConstraints=critical,CA:TRUE",
                    "-addext", "keyUsage=critical,keyCertSign,cRLSign")
            for index, (leaf, hostname) in enumerate(
                    (("role", "role"), ("auth", "auth"), ("proxy", "phase0-jwks")), 1):
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
            ca_key.unlink()
            command("keytool", "-importcert", "-noprompt", "-alias", "phase0-fixture-ca",
                    "-file", ca_cert, "-keystore", root / "truststore.p12",
                    "-storetype", "PKCS12", "-storepass", "phase0-fixture")
            for path in root.rglob("*"):
                path.chmod(0o700 if path.is_dir() else 0o600)
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
