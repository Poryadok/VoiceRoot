#!/usr/bin/env python3
"""Create new short-lived LOCAL/CI report-target PKI; never provision a stand."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def generate(destination):
    destination = Path(destination).absolute()
    if destination.exists() or destination.is_symlink():
        raise ValueError("fixture destination must not exist")
    openssl = shutil.which("openssl")
    if not openssl or not destination.parent.is_dir():
        raise ValueError("openssl and an existing parent directory are required")

    def run(*args):
        result = subprocess.run([openssl, *map(str, args)], capture_output=True, timeout=60)
        if result.returncode:
            raise ValueError("report-target fixture generation failed")

    old_umask = os.umask(0o077)
    try:
        with tempfile.TemporaryDirectory(prefix=".report-targets-", dir=destination.parent) as scratch:
            root = Path(scratch)
            (root / "moderation").mkdir()
            for kid in ("current", "next"):
                run("genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048",
                    "-out", root / "moderation" / f"{kid}.pem")
            run("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2",
                "-keyout", root / "ca.key", "-out", root / "ca.crt",
                "-subj", "/CN=Voice local report targets CA",
                "-addext", "basicConstraints=critical,CA:TRUE",
                "-addext", "keyUsage=critical,keyCertSign,cRLSign")
            for serial, (leaf, hostname, usage) in enumerate([
                (owner, owner, "serverAuth") for owner in ("user", "messaging", "space", "story", "moderation")
            ] + [("moderation-client", "moderation", "clientAuth"),
                 ("gateway-intake-client", "gateway", "clientAuth")], 1):
                extension = root / "leaf.ext"
                extension.write_text("basicConstraints=critical,CA:FALSE\n"
                                     "keyUsage=critical,digitalSignature,keyEncipherment\n"
                                     f"extendedKeyUsage={usage}\nsubjectAltName=DNS:{hostname}\n", encoding="ascii")
                run("req", "-new", "-newkey", "rsa:2048", "-nodes",
                    "-keyout", root / f"{leaf}.key", "-out", root / "leaf.csr", "-subj", f"/CN={hostname}")
                run("x509", "-req", "-in", root / "leaf.csr", "-CA", root / "ca.crt",
                    "-CAkey", root / "ca.key", "-set_serial", serial, "-days", "2",
                    "-extfile", extension, "-out", root / f"{leaf}.crt")
            (root / "leaf.ext").unlink()
            (root / "leaf.csr").unlink()
            root.rename(destination)
    finally:
        os.umask(old_umask)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination")
    generate(parser.parse_args().destination)
