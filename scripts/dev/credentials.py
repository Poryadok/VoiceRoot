#!/usr/bin/env python3
"""Issue independent voice-dev credentials on private Linux storage; never apply.

Requires Python3, openssl, keytool and a built nats-jwt-issuer. Inputs must be
the public PEM roots and unmodified cacerts exported from the exact Auth image.
The caller retains the entire protected output, including CA keys and NATS seeds.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import stat
import subprocess
import sys
import tempfile

NAMESPACE = "voice-dev"


def run(*args):
    result = subprocess.run([str(x) for x in args], capture_output=True, timeout=120)
    if result.returncode:
        raise ValueError("credential tool failed")
    return result.stdout


def validate_destination(destination):
    if sys.platform != "linux" or os.geteuid() == 0:
        raise ValueError("dedicated non-root Linux identity required")
    import pwd
    if pwd.getpwuid(os.geteuid()).pw_name == "pmd":
        raise ValueError("shared workload identity forbidden")
    destination = Path(destination)
    if not destination.is_absolute() or destination.parent.resolve() != destination.parent:
        raise ValueError("clean absolute destination required")
    if destination.is_symlink() or destination.exists():
        raise ValueError("destination must be fresh")
    parent = destination.parent
    if parent == Path("/home/pmd") or Path("/home/pmd") in parent.parents:
        raise ValueError("shared staging home forbidden")
    info = parent.stat()
    if not stat.S_ISDIR(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o700 or info.st_uid != os.geteuid():
        raise ValueError("parent must be owned private0700 directory")


def write_private(path, contents):
    with Path(path).open("xb") as output:
        output.write(contents if isinstance(contents, bytes) else contents.encode())
    Path(path).chmod(0o600)


class Material:
    def __init__(self, root):
        self.root = Path(root)
        self.items = []

    def add(self, name, values):
        encoded = {key: base64.b64encode(value if isinstance(value, bytes) else value.encode()).decode()
                   for key, value in values.items()}
        self.items.append({"apiVersion":"v1", "kind":"Secret", "metadata":{"name":name, "namespace":NAMESPACE},
                           "type":"Opaque", "data":encoded})

    def key(self, name):
        path = self.root / (name + ".key")
        run("openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:3072", "-out", path)
        path.chmod(0o600)
        return path

    def ca(self, name):
        key = self.key(name + "-ca")
        cert = self.root / (name + "-ca.crt")
        run("openssl", "req", "-x509", "-new", "-key", key, "-out", cert, "-days", "365",
            "-subj", "/CN=Voice dev " + name + " CA", "-addext", "basicConstraints=critical,CA:TRUE,pathlen:0",
            "-addext", "keyUsage=critical,keyCertSign,cRLSign")
        return key, cert

    def tls(self, name, ca, hostname, purpose):
        key = self.key(name)
        csr, cert, ext = [self.root / (name + suffix) for suffix in (".csr", ".crt", ".ext")]
        run("openssl", "req", "-new", "-key", key, "-out", csr, "-subj", "/CN=" + hostname)
        write_private(ext, "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\n"
                      "extendedKeyUsage=" + purpose + "\nsubjectAltName=DNS:" + hostname + "\n")
        run("openssl", "x509", "-req", "-in", csr, "-CA", ca[1], "-CAkey", ca[0],
            "-set_serial", "0x" + secrets.token_hex(16), "-days", "90", "-extfile", ext, "-out", cert)
        run("openssl", "verify", "-CAfile", ca[1], "-verify_hostname", hostname, cert)
        values = {"tls.crt":cert.read_bytes(), "tls.key":key.read_bytes()}
        self.add(name, values)
        csr.unlink()
        ext.unlink()
        return values

    def signer(self, name, active=True):
        values = {kid + ".pem": self.key(name + "-" + kid).read_bytes() for kid in ("current", "next")}
        if active:
            values["active-kid"] = "current"
        self.add(name, values)


def application_values(jwt, gateway_host):
    pg, ch = secrets.token_hex(32), secrets.token_hex(32)
    values = {"POSTGRES_PASSWORD":pg, "CLICKHOUSE_PASSWORD":ch,
              "CLICKHOUSE_DSN":f"clickhouse://default:{ch}@voice-clickhouse:9000/voice",
              "AUTH_JWT_PRIVATE_KEY":jwt, "AUTH_TOTP_ENCRYPTION_KEY":secrets.token_hex(32),
              "ACCOUNT_DELETE_TOKEN_SECRET":secrets.token_hex(32), "ANALYTICS_ID_HASH_KEY":secrets.token_hex(32),
              "LIVEKIT_API_KEY":secrets.token_hex(16), "LIVEKIT_API_SECRET":secrets.token_hex(32),
              "PADDLE_WEBHOOK_SECRET":secrets.token_hex(32), "GATEWAY_STATIC_TOKENS_JSON":"{}",
              "AUTH_RESEND_API_KEY":"", "AUTH_RESEND_FROM":"Voice dev <noreply@voice.invalid>",
              "FCM_PROJECT_ID":"", "FCM_SERVICE_ACCOUNT_JSON":"", "APNS_KEY_ID":"", "APNS_TEAM_ID":"",
              "APNS_PRIVATE_KEY":"", "APNS_BUNDLE_ID":"voice.app.dev", "APNS_VOIP_TOPIC":"voice.app.dev.voip",
              "APNS_PRODUCTION":"false"}
    for service in "social user chat space messaging file role matchmaking search notification bot story moderation subscription gateway voice".split():
        values[service.upper() + "_DATABASE_URL"] = f"postgres://voice:{pg}@voice-postgres:5432/{service}_db?sslmode=disable"
    for service, bucket in (("USER", "avatars"), ("FILE", "files")):
        values.update({service + "_R2_ENDPOINT":"http://voice-minio:9000", service + "_R2_REGION":"us-east-1",
                       service + "_R2_BUCKET":"voice-dev-" + bucket})
    values["USER_R2_PUBLIC_BASE_URL"] = "https://" + gateway_host + "/voice-dev-avatars"
    return values


def fingerprints(output):
    return set(re.findall(rb"SHA256:\s*([0-9A-F:]{95})", output))


def make_truststore(root, cacerts, media_cert):
    original = run("keytool", "-list", "-v", "-keystore", cacerts, "-storepass", "changeit", "-J-Duser.language=en")
    if b"PrivateKeyEntry" in original or b"SecretKeyEntry" in original:
        raise ValueError("Java trust input must contain public certificates only")
    before = fingerprints(original)
    if not before:
        raise ValueError("public Java trust input is empty")
    output = root / "truststore.p12"
    run("keytool", "-importkeystore", "-noprompt", "-srckeystore", cacerts, "-srcstorepass", "changeit",
        "-destkeystore", output, "-deststoretype", "PKCS12", "-deststorepass", "changeit")
    run("keytool", "-importcert", "-noprompt", "-alias", "voice-dev-media-ca", "-file", media_cert,
        "-keystore", output, "-storepass", "changeit")
    after = fingerprints(run("keytool", "-list", "-v", "-keystore", output, "-storepass", "changeit", "-J-Duser.language=en"))
    if not before < after or len(after) != len(before) + 1:
        raise ValueError("public Java trust changed")
    return output.read_bytes()


def validate_bundle(bundle):
    if bundle.get("apiVersion") != "v1" or bundle.get("kind") != "List" or not bundle.get("items"):
        raise ValueError("invalid Secret List")
    names = set()
    for item in bundle["items"]:
        meta = item["metadata"]
        if item.get("kind") != "Secret" or meta.get("namespace") != NAMESPACE or meta["name"] in names:
            raise ValueError("cross-environment or duplicate Secret")
        names.add(meta["name"])


def generate(destination, issuer, public_roots, java_cacerts, gateway_host, acl):
    validate_destination(destination)
    if not re.fullmatch(r"[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?", gateway_host) or any(x in gateway_host for x in ("staging", "voice-prod")):
        raise ValueError("independent dev gateway host required")
    for tool in ("openssl", "keytool"):
        if not shutil.which(tool):
            raise ValueError("crypto tooling unavailable")
    roots = Path(public_roots).read_bytes()
    if b"PRIVATE KEY" in roots or b"BEGIN CERTIFICATE" not in roots:
        raise ValueError("public roots PEM required")
    run("openssl", "crl2pkcs7", "-nocrl", "-certfile", public_roots)
    old_umask = os.umask(0o077)
    try:
        with tempfile.TemporaryDirectory(prefix=".voice-dev-issue-", dir=destination.parent) as scratch:
            root = Path(scratch)
            material = Material(root)
            principal, media, nats = [material.ca(name) for name in ("principal", "media", "nats")]
            material.add("voice-principal-ca", {"ca.crt":principal[1].read_bytes()})
            material.add("voice-media-ca", {"ca.crt":media[1].read_bytes(), "jwks-ca.crt":roots + b"\n" + media[1].read_bytes()})
            for service in ("social", "user", "space", "file", "search"):
                material.tls("voice-" + service + "-principal-tls", principal, "voice-" + service, "serverAuth,clientAuth")
            material.tls("voice-user-file-principal-tls", principal, "voice-user", "serverAuth,clientAuth")
            for service in ("social", "file", "search", "gateway"):
                material.signer("voice-" + service + "-principal-signing")
            material.signer("voice-voice-principal-signing", active=False)
            for service in ("auth", "role", "voice"):
                material.tls("voice-" + service + "-media-server-tls", media, "voice-" + service, "serverAuth")
            material.tls("voice-voice-media-jwks-tls", media, "voice-voice", "serverAuth")
            for service in ("voice", "gateway"):
                material.tls("voice-" + service + "-media-client-tls", media, "voice-" + service, "clientAuth")
            material.add("voice-media-jvm-trust", {"truststore.p12":make_truststore(root, java_cacerts, media[1])})
            material.add("voice-media-resolver", {"token":secrets.token_hex(32)})
            material.add("voice-user-search-projection", {"cursor-hmac-key":secrets.token_hex(32)})
            minio = {"MINIO_ROOT_USER":"dev" + secrets.token_hex(12), "MINIO_ROOT_PASSWORD":secrets.token_hex(32)}
            material.add("voice-minio-credentials", minio)
            app = application_values(material.key("auth-jwt").read_bytes(), gateway_host)
            for service in ("USER", "FILE"):
                app[service + "_R2_ACCESS_KEY_ID"] = minio["MINIO_ROOT_USER"]
                app[service + "_R2_SECRET_ACCESS_KEY"] = minio["MINIO_ROOT_PASSWORD"]
            material.add("voice-app-secrets", app)
            material.tls("nats-input", nats, "voice-nats", "serverAuth")
            material.items.pop()  # Input certificate is exported only through the reviewed issuer.
            run(issuer, "--namespace", NAMESPACE, "--tls-cert", root / "nats-input.crt", "--tls-key", root / "nats-input.key",
                "--tls-ca", nats[1], acl, root / "nats-issued")
            nats_list = json.loads((root / "nats-issued/secrets.json").read_text())
            validate_bundle(nats_list)
            bundle = {"apiVersion":"v1", "kind":"List", "items":material.items + nats_list["items"]}
            validate_bundle(bundle)
            write_private(root / "secrets.json", json.dumps(bundle, separators=(",", ":")) + "\n")
            provenance = {"namespace":NAMESPACE, "public_roots_sha256":hashlib.sha256(roots).hexdigest(),
                          "java_cacerts_sha256":hashlib.sha256(Path(java_cacerts).read_bytes()).hexdigest()}
            write_private(root / "public-inputs.json", json.dumps(provenance) + "\n")
            for path in root.rglob("*"):
                path.chmod(0o700 if path.is_dir() else 0o600)
            if destination.exists() or destination.is_symlink():
                raise ValueError("destination appeared")
            root.rename(destination)
    finally:
        os.umask(old_umask)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path)
    parser.add_argument("--nats-issuer", required=True, type=Path)
    parser.add_argument("--public-roots", required=True, type=Path)
    parser.add_argument("--java-cacerts", required=True, type=Path)
    parser.add_argument("--gateway-host", required=True)
    parser.add_argument("--acl", type=Path, default=Path(__file__).resolve().parents[2] / "deploy/nats/acl-intent.yaml")
    args = parser.parse_args()
    try:
        generate(args.destination, args.nats_issuer, args.public_roots, args.java_cacerts, args.gateway_host, args.acl)
    except (ValueError, OSError, KeyError, subprocess.TimeoutExpired):
        print("Dev credential generation failed; no values printed", file=sys.stderr)
        return 1
    print("Independent voice-dev credentials generated in private destination")
    return 0


if __name__ == "__main__":
    sys.exit(main())
