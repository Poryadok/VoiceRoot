#!/usr/bin/env python3
"""Offline Phase0 fixture contracts. Never starts containers or contacts Docker."""

import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import ssl
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
import yaml


ROOT = Path(__file__).resolve().parents[2]
GENERATOR = ROOT / "scripts/dev/generate-phase0-fixture.py"
OVERLAY = ROOT / "docker-compose.phase0.yml"
MEDIA_OVERLAY = ROOT / "docker-compose.voice-space-media.yml"
CI_WORKFLOW = ROOT / ".github/workflows/ci.yml"
PROXY = "phase0-jwks"
URLS = {name: f"https://{PROXY}:8443/{name}/jwks.json"
        for name in ("gateway", "space")}
MATCHMAKING_JWKS_URL = "https://voice-matchmaking:8443/internal/v1/principal/jwks.json"
STARTUP_DIAGNOSTICS = ROOT / "scripts/ci/t31-startup-diagnostics.py"
FIXTURE_RESTORER = ROOT / "scripts/dev/restore-phase0-fixture-owner.py"
CAN_SET_MATCHSQUAD_UID = os.name == "posix" and (os.geteuid() == 0 or shutil.which("sudo"))
sys.dont_write_bytecode = True
RESTORER_SPEC = importlib.util.spec_from_file_location("phase0_fixture_restorer", FIXTURE_RESTORER)
assert RESTORER_SPEC and RESTORER_SPEC.loader
fixture_restorer = importlib.util.module_from_spec(RESTORER_SPEC)
RESTORER_SPEC.loader.exec_module(fixture_restorer)
STARTUP_SPEC = importlib.util.spec_from_file_location("t31_startup_diagnostics", STARTUP_DIAGNOSTICS)
assert STARTUP_SPEC and STARTUP_SPEC.loader
startup_diagnostics = importlib.util.module_from_spec(STARTUP_SPEC)
STARTUP_SPEC.loader.exec_module(startup_diagnostics)


def run(*args, env=None, input=None):
    return subprocess.run([str(arg) for arg in args], cwd=ROOT, env=env,
                          input=input, capture_output=True, text=True, timeout=90)


def require_success(result):
    # Do not echo child output: a faulty generator could leak private material.
    if result.returncode:
        raise AssertionError(f"command failed with exit code {result.returncode}")
    return result.stdout


def safe_service_uid_failure_category(stderr):
    """Classify the UID-drop check without exposing child output or paths."""
    message = (stderr or "").lower()
    if "unknown user" in message or "unknown uid" in message:
        return "target_uid_unavailable"
    if "not in the sudoers" in message or "a password is required" in message:
        return "sudo_authorization_unavailable"
    if "permission denied" in message or "permissionerror" in message:
        return "service_uid_access_denied"
    if "assertionerror" in message:
        return "service_uid_metadata_or_read_assertion"
    if "traceback" in message:
        return "service_uid_child_failed"
    return "service_uid_command_failed"


def require_binary(name):
    if not shutil.which(name):
        raise AssertionError(f"required offline test prerequisite missing: {name}")


def restore_fixture_owner(destination):
    if os.name != "posix" or not Path(destination).exists():
        return
    uid, gid = os.getuid(), os.getgid()
    if os.geteuid() == 0:
        for path in (Path(destination), *Path(destination).rglob("*")):
            os.chown(path, uid, gid)
    else:
        require_success(run("sudo", "-n", "chown", "-R", f"{uid}:{gid}", destination))
    for path in (Path(destination), Path(destination) / "tls", Path(destination) / "ca",
                 Path(destination) / "matchmaking"):
        if path.is_dir():
            path.chmod(0o700)


class FixtureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not GENERATOR.is_file():
            raise AssertionError("Phase0 fixture generator does not exist")
        for binary in ("openssl", "keytool"):
            require_binary(binary)
        cls.temp = tempfile.TemporaryDirectory(prefix="voice-phase0-test-")
        cls.addClassCleanup(cls.temp.cleanup)
        cls.dest = Path(cls.temp.name) / "fixture"
        generator_env = os.environ.copy()
        generator_env.pop("PHASE0_MATCHSQUAD_FIXTURE_UID", None)
        cls.generated = run(sys.executable, GENERATOR, cls.dest, env=generator_env)
        require_success(cls.generated)

    @unittest.skipUnless(os.name == "posix" and os.geteuid() != 0 and shutil.which("sudo"),
                         "covers non-root CI runners that can provision the service UID")
    def test_general_fixture_inputs_remain_readable_by_the_test_user(self):
        path = self.dest / "matchmaking/current.pem"
        self.assertEqual(path.stat().st_uid, os.getuid())
        self.assertTrue(path.read_bytes().startswith(b"-----BEGIN PRIVATE KEY-----"))

    def openssl(self, *args, input=None):
        return require_success(run("openssl", *args, input=input))

    def test_separate_current_next_rsa_pkcs8_keys(self):
        public_keys = set()
        for issuer in ("gateway", "space", "matchmaking"):
            self.assertEqual({p.name for p in (self.dest / issuer).iterdir()},
                             {"current.pem", "next.pem"})
            for kid in ("current", "next"):
                path = self.dest / issuer / f"{kid}.pem"
                self.assertTrue(path.read_text().startswith("-----BEGIN PRIVATE KEY-----"),
                                "issuer must contain unencrypted PKCS8")
                public = self.openssl("pkey", "-in", path, "-pubout")
                description = self.openssl("rsa", "-pubin", "-text", "-noout", input=public)
                bits = re.search(r"Public-Key:\s*\((\d+) bit\)", description)
                self.assertIsNotNone(bits)
                self.assertGreaterEqual(int(bits.group(1)), 2048)
                public_keys.add(public)
        for issuer in ("gameintegration", "voice", "bot"):
            for kid in ("current", "next"):
                path = self.dest / issuer / f"{kid}.pem"
                self.assertTrue(path.read_text().startswith("-----BEGIN PRIVATE KEY-----"),
                                "T31 service signer must contain unencrypted PKCS8")
                public = self.openssl("pkey", "-in", path, "-pubout")
                description = self.openssl("rsa", "-pubin", "-text", "-noout", input=public)
                bits = re.search(r"Public-Key:\s*\((\d+) bit\)", description)
                self.assertIsNotNone(bits)
                self.assertGreaterEqual(int(bits.group(1)), 2048)
                public_keys.add(public)
        self.assertEqual(len(public_keys), 12, "issuer and service signing keys must be independent")

    def test_matchmaking_public_jwks_matches_both_private_rotation_keys(self):
        for issuer in ("gateway", "matchmaking"):
            with self.subTest(issuer=issuer):
                document = json.loads((self.dest / f"jwks/{issuer}/jwks.json").read_text(encoding="ascii"))
                self.assertEqual({key["kid"] for key in document["keys"]}, {"current", "next"})
                self.assertEqual(len(document["keys"]), 2)
                for key in document["keys"]:
                    with self.subTest(issuer=issuer, kid=key["kid"]):
                        self.assertEqual(set(key), {"kty", "kid", "use", "alg", "n", "e"})
                        self.assertEqual((key["kty"], key["use"], key["alg"]), ("RSA", "sig", "RS256"))
                        modulus = self.openssl("rsa", "-in", self.dest / f"{issuer}/{key['kid']}.pem",
                                               "-noout", "-modulus").strip().split("=", 1)[1]
                        decode = lambda value: base64.urlsafe_b64decode(value + "=" * (-len(value) % 4))
                        self.assertEqual(decode(key["n"]), bytes.fromhex(modulus))
                        self.assertEqual(int.from_bytes(decode(key["e"]), "big"), 65537)

    def test_ca_signed_leaf_keys_hostname_and_short_lifetime(self):
        ca = self.dest / "ca/ca.crt"
        self.assertEqual({p.name for p in ca.parent.iterdir()},
                         {"ca.crt", "gameintegration-client-ca.crt", "space-lifecycle-client-ca.crt"},
                         "CA signing key must not survive issuance")
        for leaf, hostname in (("role", "role"), ("auth", "auth"), ("proxy", PROXY),
                               ("matchmaking-complete-grpc", "matchmaking"),
                               ("gameintegration-jwks", "gameintegration"),
                                ("voice-jwks", "voice"), ("chat-gis-grpc", "chat"),
                                ("voice-game-grpc", "voice"),
                                ("chat-match-squad-grpc", "chat"), ("voice-match-squad-grpc", "voice"),
                                 ("voice-match-squad-member-grpc", "voice"),
                                ("space-lifecycle", "space"),
                                ("bot-game-event", "bot"), ("notification-lifecycle", "notification"),
                                ("messaging-gameintegration-grpc", "messaging")):
            with self.subTest(leaf=leaf):
                cert = self.dest / "tls" / f"{leaf}.crt"
                key = self.dest / "tls" / f"{leaf}.key"
                self.openssl("verify", "-CAfile", ca, "-purpose", "sslserver",
                             "-verify_hostname", hostname, cert)
                san = self.openssl("x509", "-in", cert, "-noout", "-ext", "subjectAltName")
                self.assertIn(hostname, re.findall(r"DNS:([^,\s]+)", san),
                              "leaf must have an explicit DNS SAN, not only a matching CN")
                bad = run("openssl", "verify", "-CAfile", ca,
                          "-verify_hostname", "wrong.invalid", cert)
                self.assertNotEqual(bad.returncode, 0)
                self.assertEqual(self.openssl("x509", "-in", cert, "-pubkey", "-noout"),
                                 self.openssl("pkey", "-in", key, "-pubout"))
                dates = self.openssl("x509", "-in", cert, "-dates", "-noout")
                values = dict(line.split("=", 1) for line in dates.splitlines())
                lifetime = ssl.cert_time_to_seconds(values["notAfter"]) - ssl.cert_time_to_seconds(values["notBefore"])
                self.assertGreater(lifetime, 0)
                self.assertLessEqual(lifetime, 2 * 86400 + 60)

    def test_service_client_leaves_are_ca_signed_and_client_auth_only(self):
        for service in ("space", "gameintegration", "voice", "matchmaking"):
            with self.subTest(service=service):
                cert = self.dest / f"tls/{service}-client.crt"
                self.openssl("verify", "-CAfile", self.dest / "ca/ca.crt", "-purpose", "sslclient", cert)
                eku = self.openssl("x509", "-in", cert, "-noout", "-ext", "extendedKeyUsage")
                self.assertIn("TLS Web Client Authentication", eku)
                self.assertNotIn("TLS Web Server Authentication", eku)
                subject = self.openssl("x509", "-in", cert, "-noout", "-subject")
                # OpenSSL 3.4 formats this as ``subject=CN = service`` while
                # older releases commonly emit ``subject=CN=service``.
                normalized_subject = re.sub(r"\s+", "", subject).removeprefix("subject=")
                self.assertRegex(
                    normalized_subject,
                    rf"(?:^|[,/])CN={re.escape(service)}(?:$|[,/])",
                )

    def test_gateway_voice_media_client_leaf_is_root_ca_client_only(self):
        cert = self.dest / "tls/gateway-client.crt"
        self.openssl("verify", "-CAfile", self.dest / "ca/ca.crt", "-purpose", "sslclient", cert)
        eku = self.openssl("x509", "-in", cert, "-noout", "-ext", "extendedKeyUsage")
        self.assertIn("TLS Web Client Authentication", eku)
        self.assertNotIn("TLS Web Server Authentication", eku)
        subject = self.openssl("x509", "-in", cert, "-noout", "-subject")
        normalized_subject = re.sub(r"\s+", "", subject).removeprefix("subject=")
        self.assertRegex(normalized_subject, r"(?:^|[,/])CN=gateway(?:$|[,/])")

    def test_gis_client_leaves_use_dedicated_ca_for_chat_and_voice(self):
        ca = self.dest / "ca/gameintegration-client-ca.crt"
        for leaf in ("gameintegration-chat-client", "gameintegration-voice-client", "gameintegration-messaging-client"):
            with self.subTest(leaf=leaf):
                cert = self.dest / f"tls/{leaf}.crt"
                self.openssl("verify", "-CAfile", ca, "-purpose", "sslclient", cert)
                eku = self.openssl("x509", "-in", cert, "-noout", "-ext", "extendedKeyUsage")
                self.assertIn("TLS Web Client Authentication", eku)
                self.assertNotIn("TLS Web Server Authentication", eku)
                self.assertNotEqual(run("openssl", "verify", "-CAfile", self.dest / "ca/ca.crt",
                                        "-purpose", "sslclient", cert).returncode, 0)

    def test_space_lifecycle_client_uses_only_its_dedicated_ca(self):
        cert = self.dest / "tls/space-lifecycle-client.crt"
        self.openssl("verify", "-CAfile", self.dest / "ca/space-lifecycle-client-ca.crt",
                     "-purpose", "sslclient", cert)
        for foreign_ca in ("ca.crt", "gameintegration-client-ca.crt"):
            self.assertNotEqual(run("openssl", "verify", "-CAfile", self.dest / "ca" / foreign_ca,
                                    "-purpose", "sslclient", cert).returncode, 0)
        subject = self.openssl("x509", "-in", cert, "-noout", "-subject")
        self.assertEqual(re.sub(r"\s+", "", subject).removeprefix("subject="), "CN=space")
        eku = self.openssl("x509", "-in", cert, "-noout", "-ext", "extendedKeyUsage")
        self.assertIn("TLS Web Client Authentication", eku)
        self.assertNotIn("TLS Web Server Authentication", eku)

    def test_certificate_serials_are_distinct_within_each_issuer(self):
        issued = set()
        for cert in (self.dest / "tls").glob("*.crt"):
            identity = self.openssl("x509", "-in", cert, "-noout", "-issuer", "-serial")
            self.assertNotIn(identity, issued, "one CA must not issue duplicate serials")
            issued.add(identity)

    def test_jvm_truststore_contains_fixture_ca_and_no_private_entry(self):
        listing = require_success(run("keytool", "-list", "-rfc", "-keystore",
                                      self.dest / "truststore.p12", "-storetype", "PKCS12",
                                      "-storepass", "phase0-fixture",
                                      "-J-Duser.language=en"))
        self.assertNotIn("PrivateKeyEntry", listing)
        certs = re.findall(r"-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----",
                           listing, re.S)
        self.assertTrue(certs, "truststore contains no certificates")
        ca = self.openssl("x509", "-in", self.dest / "ca/ca.crt", "-fingerprint", "-sha256", "-noout")
        fingerprints = {self.openssl("x509", "-fingerprint", "-sha256", "-noout", input=cert)
                        for cert in certs}
        self.assertIn(ca, fingerprints)

    def test_existing_destination_is_refused_without_mutation(self):
        def snapshot():
            return {p.relative_to(self.dest).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
                    for p in self.dest.rglob("*") if p.is_file()}
        before = snapshot()
        result = run(sys.executable, GENERATOR, self.dest)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(snapshot(), before)
        empty = Path(self.temp.name) / "existing-empty"
        empty.mkdir(exist_ok=True)
        self.assertNotEqual(run(sys.executable, GENERATOR, empty).returncode, 0)
        self.assertEqual(list(empty.iterdir()), [])

    def test_private_material_never_appears_in_process_output(self):
        output = self.generated.stdout + self.generated.stderr
        self.assertFalse("PRIVATE KEY" in output, "generator leaked a private PEM marker")
        for path in self.dest.rglob("*"):
            if path.is_file() and path.suffix in (".key", ".pem"):
                for line in path.read_text().splitlines():
                    if len(line) >= 40 and not line.startswith("-----"):
                        self.assertTrue(line not in output, "generator leaked private key data")

    def test_missing_tools_fail_without_partial_destination(self):
        dest = Path(self.temp.name) / "missing-tools"
        env = os.environ.copy()
        env["PATH"] = ""
        before = set(Path(self.temp.name).iterdir())
        result = run(sys.executable, GENERATOR, dest, env=env)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(dest.exists())
        self.assertEqual(set(Path(self.temp.name).iterdir()), before)

    def test_tool_failure_after_private_creation_removes_partial_fixture(self):
        # Run the real generator and real crypto tools until private material is
        # on disk. Replace one subsequent tool process with a real nonzero child.
        # Popen interception works on Windows and POSIX without shell wrappers.
        harness = r'''
import contextlib
import io
from pathlib import Path
import runpy
import subprocess
import sys
from unittest.mock import patch

generator, destination, failed_tool = sys.argv[1:]
scratch = Path(destination).parent
original_popen = subprocess.Popen
private_lines = set()
injected = False

def failing_popen(command, *args, **kwargs):
    global injected
    name = Path(str(command[0])).stem.lower() if isinstance(command, (list, tuple)) else ""
    if name == failed_tool and not injected:
        for path in scratch.rglob("*"):
            if path.is_file():
                data = path.read_bytes()
                if b"PRIVATE KEY-----" in data:
                    private_lines.update(line for line in data.decode("ascii").splitlines()
                                         if len(line) >= 40 and not line.startswith("-----"))
        if private_lines:
            injected = True
            command = [sys.executable, "-c", "import sys; sys.exit(73)"]
    return original_popen(command, *args, **kwargs)

sys.argv = [generator, destination]
stdout, stderr = io.StringIO(), io.StringIO()
code = 0
with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
    try:
        with patch("subprocess.Popen", side_effect=failing_popen):
            runpy.run_path(generator, run_name="__main__")
    except SystemExit as error:
        code = error.code if isinstance(error.code, int) else 1
    except Exception:
        code = 1
output = stdout.getvalue() + stderr.getvalue()
print("injected-after-private" if injected else "injection-not-reached")
leaked = "PRIVATE KEY" in output or any(line in output for line in private_lines)
print("private-output-detected" if leaked else "no-private-output")
sys.exit(code)
'''
        for tool in ("openssl", "keytool"):
            with self.subTest(tool=tool), tempfile.TemporaryDirectory(prefix="voice-phase0-fail-") as tmp:
                scratch = Path(tmp)
                dest = scratch / "fixture"
                result = run(sys.executable, "-c", harness, GENERATOR, dest, tool)
                output = result.stdout + result.stderr
                self.assertTrue("injected-after-private" in output,
                                "failure injection must occur after actual private material creation")
                self.assertNotEqual(result.returncode, 0, "tool failure must fail generation")
                self.assertTrue("no-private-output" in output and "private-output-detected" not in output,
                                "failed generation leaked private data")
                self.assertFalse("PRIVATE KEY" in output)
                self.assertFalse(dest.exists(), "failed generation published its destination")
                self.assertEqual(list(scratch.iterdir()), [], "failed generation left sibling temp material")

    @unittest.skipUnless(os.name == "posix", "POSIX modes are verified in Linux CI")
    def test_private_permissions(self):
        paths = [self.dest]
        for directory, dirs, files in os.walk(self.dest, topdown=True):
            current = Path(directory)
            paths.extend(current / name for name in (*dirs, *files))
            if current == self.dest:
                dirs[:] = [name for name in dirs if name != "matchmaking"]
        for path in paths:
            if path.is_dir():
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o700)
            elif path.suffix in (".pem", ".key", ".p12"):
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)

    @unittest.skipUnless(CAN_SET_MATCHSQUAD_UID, "UID ownership is verified in Linux CI")
    def test_match_squad_fixture_is_private_and_readable_by_service_uid(self):
        service_temp = tempfile.TemporaryDirectory(prefix="voice-phase0-service-test-")
        self.addCleanup(service_temp.cleanup)
        service_root = Path(service_temp.name)
        service_root.chmod(0o711)
        service_dest = service_root / "fixture"
        service_env = os.environ.copy()
        service_env["PHASE0_MATCHSQUAD_FIXTURE_UID"] = "65532"
        require_success(run(sys.executable, GENERATOR, service_dest, env=service_env))
        self.addCleanup(restore_fixture_owner, service_dest)
        self.assertEqual(stat.S_IMODE(service_root.stat().st_mode), 0o711)
        owned_directory = service_dest / "matchmaking"
        self.assertEqual((owned_directory.stat().st_uid, owned_directory.stat().st_gid), (65532, 65532))
        self.assertEqual(stat.S_IMODE(owned_directory.stat().st_mode), 0o700)
        owned = [owned_directory,
                 service_dest / "matchmaking/current.pem", service_dest / "matchmaking/next.pem",
                 service_dest / "tls/chat-match-squad-grpc.crt",
                 service_dest / "tls/chat-match-squad-grpc.key",
                 service_dest / "tls/voice-match-squad-grpc.crt",
                 service_dest / "tls/voice-match-squad-grpc.key",
                 service_dest / "tls/voice-match-squad-member-grpc.crt",
                 service_dest / "tls/voice-match-squad-member-grpc.key",
                 service_dest / "tls/matchmaking-client.crt",
                 service_dest / "tls/matchmaking-client.key",
                 service_dest / "tls/matchmaking-complete-grpc.crt",
                 service_dest / "tls/matchmaking-complete-grpc.key",
                 service_dest / "tls/space-lifecycle.crt",
                 service_dest / "tls/space-lifecycle.key",
                 service_dest / "tls/space-client.crt",
                 service_dest / "tls/space-client.key",
                 service_dest / "ca/ca.crt"]
        read_check = (
            "import pathlib,stat,sys\n"
            "import os\n"
            "assert (os.geteuid(),os.getegid())==(65532,65532)\n"
            "for raw in sys.argv[1:]:\n"
            " p=pathlib.Path(raw); s=p.stat(); expected=0o700 if p.is_dir() else 0o600\n"
            " assert (s.st_uid,s.st_gid)==(65532,65532) and stat.S_IMODE(s.st_mode)==expected\n"
            " if p.is_dir(): assert {x.name for x in p.iterdir()}=={'current.pem','next.pem'}\n"
            " else: p.read_bytes()"
        )
        paths = [str(path) for path in owned]
        if os.geteuid() == 0:
            def drop_to_service_uid():
                os.setgroups([])
                os.setgid(65532)
                os.setuid(65532)

            result = subprocess.run([sys.executable, "-c", read_check, *paths],
                                    cwd=ROOT, capture_output=True, text=True,
                                    timeout=90, preexec_fn=drop_to_service_uid)
        else:
            require_binary("sudo")
            drop_to_service_uid = (
                "import os,sys\n"
                "read_check=sys.argv.pop(1)\n"
                "os.setgroups([]); os.setgid(65532); os.setuid(65532)\n"
                "exec(read_check, {'__name__':'__main__'})"
            )
            command = ("sudo", "-n", "-u", "root", sys.executable, "-c",
                       drop_to_service_uid, read_check, *paths)
            result = run(*command)
        if result.returncode:
            category = safe_service_uid_failure_category(result.stderr)
            raise AssertionError(
                f"service UID read check failed with exit code {result.returncode} "
                f"(category={category})")


class ComposeTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not OVERLAY.is_file():
            raise AssertionError("Phase0 Compose overlay does not exist")
        if not MEDIA_OVERLAY.is_file():
            raise AssertionError("Voice space-media Compose overlay does not exist")
        require_binary("docker")
        cls.temp = tempfile.TemporaryDirectory(prefix="voice-phase0-compose-")
        cls.addClassCleanup(cls.temp.cleanup)
        cls.fixture = Path(cls.temp.name) / "fixture"
        cls.empty_env = Path(cls.temp.name) / "empty.env"
        cls.empty_env.write_text("")
        for binary in ("openssl", "keytool"):
            require_binary(binary)
        generator_env = os.environ.copy()
        if CAN_SET_MATCHSQUAD_UID:
            Path(cls.temp.name).chmod(0o711)
            generator_env["PHASE0_MATCHSQUAD_FIXTURE_UID"] = "65532"
        require_success(run(sys.executable, GENERATOR, cls.fixture, env=generator_env))
        if CAN_SET_MATCHSQUAD_UID:
            cls.addClassCleanup(restore_fixture_owner, cls.fixture)
        # Whitelist execution prerequisites, never inherit developer Compose secrets.
        cls.env = {k: v for k, v in os.environ.items()
                   if k.upper() in {"PATH", "SYSTEMROOT", "WINDIR", "HOME", "USERPROFILE",
                                    "TEMP", "TMP", "DOCKER_CONFIG"}}
        cls.env["PHASE0_FIXTURE_DIR"] = str(cls.fixture.resolve())
        cls.base = cls.render(False)
        cls.merged = cls.render(True)
        cls.media_merged = cls.render_media()
        cls.ci_merged = cls.render_ci_stack()

    @classmethod
    def command(cls, overlay):
        command = ["docker", "compose", "--env-file", cls.empty_env,
                   "--profile", "*", "-f", ROOT / "docker-compose.yml"]
        if overlay:
            command += ["-f", OVERLAY]
        return command + ["config", "--format", "json"]

    @classmethod
    def render(cls, overlay):
        return json.loads(require_success(run(*cls.command(overlay), env=cls.env)))["services"]

    @classmethod
    def render_media(cls):
        command = ["docker", "compose", "--env-file", cls.empty_env,
                   "--profile", "*", "-f", ROOT / "docker-compose.yml",
                   "-f", MEDIA_OVERLAY, "config", "--format", "json"]
        return json.loads(require_success(run(*command, env=cls.env)))["services"]

    @classmethod
    def render_ci_stack(cls):
        command = ["docker", "compose", "--env-file", cls.empty_env,
                   "--profile", "*", "-f", ROOT / "docker-compose.yml",
                   "-f", MEDIA_OVERLAY, "-f", OVERLAY, "config", "--format", "json"]
        return json.loads(require_success(run(*command, env=cls.env)))["services"]

    def fixture_mounts(self, service):
        mounts = []
        for volume in self.merged[service].get("volumes", []):
            if volume.get("type") == "bind":
                source = Path(volume["source"])
                if source.is_relative_to(self.fixture):
                    mounts.append((source.relative_to(self.fixture).as_posix(), volume))
        return mounts

    def source_at(self, service, target):
        for source, volume in self.fixture_mounts(service):
            if volume["target"] == target:
                return source
        self.fail(f"{service} config path has no fixture mount: {target}")

    def test_matchmaking_trusts_replaced_space_jwks_certificate(self):
        environment = self.merged["matchmaking"]["environment"]
        self.assertEqual(json.loads(environment["S2S_JWKS_URLS_JSON"])["space"],
                         "https://space:8443/.well-known/jwks.json")
        self.assertEqual(self.source_at("matchmaking", environment["S2S_JWKS_CA_FILE"]),
                         "ca/ca.crt")
        self.assertNotIn("user", self.merged["matchmaking"],
                         "Matchmaking must retain its Dockerfile nonroot identity")
        self.assertIn("USER nonroot:nonroot", (ROOT / "src/backend/matchmaking/Dockerfile").read_text())

    def test_matchmaking_complete_match_uses_gateway_principal_and_scoped_mtls(self):
        gateway = self.merged["gateway"]
        matchmaking = self.merged["matchmaking"]
        gateway_env = gateway["environment"]
        matchmaking_env = matchmaking["environment"]
        self.assertEqual(gateway_env["GATEWAY_MATCHMAKING_COMPLETE_GRPC_ADDR"], "matchmaking:9092")
        self.assertEqual(gateway_env["GATEWAY_MATCHMAKING_COMPLETE_TLS_SERVER_NAME"], "matchmaking")
        for key, leaf in (("GATEWAY_MATCHMAKING_COMPLETE_TLS_CA_FILE", "ca/ca.crt"),
                          ("GATEWAY_MATCHMAKING_COMPLETE_CLIENT_CERT_FILE", "tls/gateway-client.crt"),
                          ("GATEWAY_MATCHMAKING_COMPLETE_CLIENT_KEY_FILE", "tls/gateway-client.key")):
            self.assertEqual(self.source_at("gateway", gateway_env[key]), leaf)
        self.assertEqual(json.loads(matchmaking_env["S2S_JWKS_URLS_JSON"])["gateway"],
                         "https://phase0-jwks:8443/gateway-bootstrap/jwks.json")
        self.assertEqual(json.loads(self.merged["auth"]["environment"]["S2S_JWKS_URLS_JSON"])["gateway"],
                         "https://phase0-jwks:8443/gateway/jwks.json")
        self.assertEqual(self.source_at("phase0-jwks", "/run/phase0/gateway/jwks.json"),
                         "jwks/gateway/jwks.json")
        self.assertEqual(json.loads(matchmaking_env["S2S_JWKS_URLS_JSON"])["space"],
                         "https://space:8443/.well-known/jwks.json")
        self.assertEqual(self.source_at("matchmaking", matchmaking_env["S2S_JWKS_CA_FILE"]), "ca/ca.crt")
        self.assertEqual(self.source_at("matchmaking", matchmaking_env["MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE"]),
                         "ca/ca.crt")
        self.assertEqual(self.source_at("matchmaking", matchmaking_env["MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE"]),
                         "tls/matchmaking-complete-grpc.crt")
        self.assertEqual(self.source_at("matchmaking", matchmaking_env["MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE"]),
                         "tls/matchmaking-complete-grpc.key")
        self.assertEqual(matchmaking["depends_on"]["phase0-jwks"]["condition"], "service_started")

    def test_compose_e2e_starts_with_generated_complete_match_fixture(self):
        workflow = CI_WORKFLOW.read_text(encoding="utf-8")
        fixture_generation = workflow.index("python scripts/dev/generate-phase0-fixture.py")
        app_start = workflow.index("      - name: Start app stack")
        app_section = workflow[app_start:workflow.index("      - name:", app_start + 1)]

        self.assertLess(fixture_generation, app_start)
        self.assertIn(
            "-f docker-compose.yml -f docker-compose.voice-space-media.yml -f docker-compose.phase0.yml --profile app",
            app_section,
        )
        self.assertIn("docker-compose.phase0.yml", app_section)

    def test_complete_match_ci_stack_keeps_matchmaking_nonroot_and_reads_owned_tls(self):
        matchmaking = self.ci_merged["matchmaking"]
        environment = matchmaking["environment"]
        self.assertNotIn("user", matchmaking)
        self.assertIn("USER nonroot:nonroot", (ROOT / "src/backend/matchmaking/Dockerfile").read_text())
        for key, leaf in (("MATCHMAKING_SPACE_PRINCIPAL_TLS_CERT_FILE", "tls/matchmaking-complete-grpc.crt"),
                          ("MATCHMAKING_SPACE_PRINCIPAL_TLS_KEY_FILE", "tls/matchmaking-complete-grpc.key"),
                          ("MATCHMAKING_SPACE_PRINCIPAL_CLIENT_CA_FILE", "ca/ca.crt")):
            self.assertEqual(self.media_source_at("matchmaking", environment[key], merged=self.ci_merged), leaf)
        if CAN_SET_MATCHSQUAD_UID:
            for directory in (self.fixture, self.fixture / "tls", self.fixture / "ca"):
                info = directory.stat()
                self.assertEqual(stat.S_IMODE(info.st_mode), 0o711)
            matchmaking_dir = self.fixture / "matchmaking"
            matchmaking_info = matchmaking_dir.stat()
            self.assertEqual((matchmaking_info.st_uid, matchmaking_info.st_gid), (65532, 65532))
            self.assertEqual(stat.S_IMODE(matchmaking_info.st_mode), 0o700)
            for leaf in ("tls/matchmaking-complete-grpc.crt", "tls/matchmaking-complete-grpc.key", "ca/ca.crt"):
                path = self.fixture / leaf
                info = path.stat()
                self.assertEqual((info.st_uid, info.st_gid), (65532, 65532))
                self.assertEqual(stat.S_IMODE(info.st_mode), 0o600)

    def test_media_compose_preserves_social_and_protected_voice_space_authority(self):
        space = self.media_merged["space"]
        provider = space["environment"]
        voice = self.media_merged["voice"]["environment"]
        self.assertEqual(json.loads(provider["S2S_JWKS_URLS_JSON"]), {
            "social": "https://social:8443/.well-known/jwks.json",
            "voice": "https://voice:8443/internal/v1/principal/jwks.json",
        })
        self.assertEqual(provider["S2S_JWKS_CA_FILE"], "/etc/voice/principal/ca/ca.crt")
        prefix = "SPACE_VOICE_COMMUNICATION_"
        self.assertEqual(provider[prefix + "GRPC_LISTEN"], ":9094")
        self.assertEqual(provider[prefix + "REPLAY_REDIS_ADDR"], "redis:6379")
        for suffix, leaf in (("TLS_CERT_FILE", "tls/space-lifecycle.crt"),
                             ("TLS_KEY_FILE", "tls/space-lifecycle.key"),
                             ("CLIENT_CA_FILE", "ca/ca.crt"),
                             ("JWKS_CA_FILE", "ca/ca.crt"),
                             ("JWKS_CLIENT_CERT_FILE", "tls/space-client.crt"),
                             ("JWKS_CLIENT_KEY_FILE", "tls/space-client.key")):
            self.assertEqual(self.media_source_at("space", provider[prefix + suffix]), leaf)
        self.assertIn("9094", space["expose"])
        self.assertNotIn("user", space, "Space keeps its existing nonroot image identity")
        self.assertNotIn("voice", space.get("depends_on", {}), "startup graph must stay acyclic")
        self.assertFalse(any(k.startswith("SPACE_MESSAGING_COMMUNICATION_") for k in provider))
        self.assertEqual(voice["VOICE_SPACE_COMMUNICATION_GRPC_ADDR"], "space:9094")
        for suffix, leaf in (("TLS_CA_FILE", "ca/ca.crt"),
                             ("CLIENT_CERT_FILE", "tls/voice-client.crt"),
                             ("CLIENT_KEY_FILE", "tls/voice-client.key")):
            self.assertEqual(self.media_source_at("voice", voice["VOICE_SPACE_COMMUNICATION_" + suffix]), leaf)
        space_sources = {Path(v["source"]).name for v in space.get("volumes", []) if v.get("type") == "bind"}
        self.assertNotIn("voice-client.key", space_sources)
        self.assertNotIn("current.pem", space_sources)

    def test_media_compose_uses_protected_match_squad_provider_fixture(self):
        matchmaking = self.media_merged["matchmaking"]
        chat = self.media_merged["chat"]
        voice = self.media_merged["voice"]
        environment = matchmaking["environment"]
        self.assertEqual(environment["MATCHMAKING_MATCH_SQUAD_CHAT_GRPC_ADDR"], "chat:9092")
        self.assertEqual(environment["MATCHMAKING_MATCH_SQUAD_VOICE_GRPC_ADDR"], "voice:9094")
        self.assertEqual(environment["MATCHMAKING_MATCH_SQUAD_CHAT_SERVER_NAME"], "chat")
        self.assertEqual(environment["MATCHMAKING_MATCH_SQUAD_VOICE_SERVER_NAME"], "voice")
        self.assertEqual(environment["MATCHMAKING_PRINCIPAL_ACTIVE_KID"], "current")
        for key in ("MATCHMAKING_MATCH_SQUAD_CHAT_CA_FILE", "MATCHMAKING_MATCH_SQUAD_VOICE_CA_FILE"):
            self.assertEqual(self.media_source_at("matchmaking", environment[key]), "ca/ca.crt")
        self.assertEqual(self.media_source_at("matchmaking", environment["MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR"]),
                         "matchmaking")
        self.assertEqual(environment["MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR"], ":8443")
        self.assertEqual(self.media_source_at("matchmaking", environment["MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE"]),
                         "tls/matchmaking-jwks.crt")
        self.assertEqual(self.media_source_at("matchmaking", environment["MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE"]),
                         "tls/matchmaking-jwks.key")
        self.assertEqual(self.media_source_at("matchmaking", environment["MATCHMAKING_MATCH_SQUAD_CLIENT_CERT_FILE"]),
                         "tls/matchmaking-client.crt")
        self.assertEqual(self.media_source_at("matchmaking", environment["MATCHMAKING_MATCH_SQUAD_CLIENT_KEY_FILE"]),
                         "tls/matchmaking-client.key")
        self.assertNotIn("user", matchmaking, "Matchmaking must retain its Dockerfile nonroot identity")
        self.assertIn("USER nonroot:nonroot", (ROOT / "src/backend/matchmaking/Dockerfile").read_text())
        self.assertEqual(chat["environment"]["CHAT_MATCH_SQUAD_PRINCIPAL_GRPC_LISTEN"], ":9092")
        self.assertEqual(voice["environment"]["VOICE_MATCH_SQUAD_PRINCIPAL_GRPC_LISTEN"], ":9094")
        self.assertEqual(voice["environment"]["VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_GRPC_LISTEN"], ":9095")
        self.assertEqual(self.media_merged["voice"].get("user"), "0:0")
        self.assertEqual(voice["environment"]["VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_JWKS_URL"],
                         "https://phase0-jwks:8443/gateway/jwks.json")
        self.assertEqual(voice["environment"]["VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_REDIS_ADDR"], "redis:6379")
        self.assertEqual(voice["environment"]["VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_CLIENT_CA_FILE"],
                         "/run/phase0/ca.crt")
        voice_main = (ROOT / "src/backend/voice/main.go").read_text(encoding="utf-8")
        self.assertIn("if matchSquadEnabled != matchSquadMemberEnabled", voice_main)
        self.assertIn("Voice MatchSquad provisioning and delegated member listeners must be enabled together",
                      voice_main)
        self.assertEqual(self.media_merged["gateway"]["environment"]["GATEWAY_MATCH_SQUAD_MEMBER_GRPC_ADDR"],
                         "voice:9095")
        self.assertEqual(self.media_merged["gateway"]["environment"]["GATEWAY_MATCH_SQUAD_MEMBER_TLS_SERVER_NAME"],
                         "voice")
        self.assertEqual(self.media_source_at("gateway", "/run/phase0/gateway-client.crt"),
                         "tls/gateway-client.crt")
        self.assertEqual(self.media_source_at("gateway", "/run/phase0/gateway-client.key"),
                         "tls/gateway-client.key")
        for service, env, leaf in (("chat", chat["environment"], "chat-match-squad-grpc"),
                                   ("voice", voice["environment"], "voice-match-squad-grpc")):
            self.assertEqual(env[f"{service.upper()}_MATCH_SQUAD_PRINCIPAL_JWKS_URL"], MATCHMAKING_JWKS_URL)
            self.assertEqual(env[f"{service.upper()}_MATCH_SQUAD_PRINCIPAL_JWKS_CA_FILE"], "/run/phase0/ca.crt")
            self.assertEqual(env[f"{service.upper()}_MATCH_SQUAD_PRINCIPAL_REPLAY_REDIS_ADDR"], "redis:6379")
            self.assertEqual(env[f"{service.upper()}_MATCH_SQUAD_PRINCIPAL_REPLAY_REDIS_PASSWORD"], "")
            if service == "chat":
                self.assertNotIn("user", self.media_merged[service],
                                 "Chat must retain its Dockerfile nonroot identity")
                self.assertIn("USER nonroot:nonroot", (ROOT / "src/backend/chat/Dockerfile").read_text())
            self.assertEqual(self.media_source_at(service, env[f"{service.upper()}_MATCH_SQUAD_PRINCIPAL_TLS_CERT_FILE"],
                                                  merged=self.media_merged), f"tls/{leaf}.crt")
            self.assertEqual(self.media_source_at(service, env[f"{service.upper()}_MATCH_SQUAD_PRINCIPAL_TLS_KEY_FILE"],
                                                  merged=self.media_merged), f"tls/{leaf}.key")
            self.assertEqual(self.media_source_at(service, env[f"{service.upper()}_MATCH_SQUAD_PRINCIPAL_CLIENT_CA_FILE"],
                                                  merged=self.media_merged), "ca/ca.crt")
        self.assertEqual(self.media_source_at(
            "voice", voice["environment"]["VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_TLS_CERT_FILE"],
            merged=self.media_merged), "tls/voice-match-squad-member-grpc.crt")
        self.assertEqual(self.media_source_at(
            "voice", voice["environment"]["VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_TLS_KEY_FILE"],
            merged=self.media_merged), "tls/voice-match-squad-member-grpc.key")
        self.assertEqual(self.media_source_at(
            "voice", voice["environment"]["VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_JWKS_CA_FILE"],
            merged=self.media_merged), "ca/ca.crt")
        self.assertEqual(self.media_merged["gateway"]["environment"][
            "GATEWAY_MATCH_SQUAD_MEMBER_TLS_CA_FILE"], "/run/phase0/ca.crt")
        self.assertEqual(self.media_merged["gateway"]["environment"][
            "GATEWAY_MATCH_SQUAD_MEMBER_CLIENT_CERT_FILE"], "/run/phase0/gateway-client.crt")
        self.assertEqual(self.media_source_at("phase0-jwks", "/run/phase0/matchmaking/jwks.json",
                                              merged=self.media_merged), "jwks/matchmaking/jwks.json")

    def test_match_squad_provider_fixture_is_opt_in_to_media_compose(self):
        for service in ("chat", "voice", "matchmaking"):
            with self.subTest(service=service):
                environment = self.base[service].get("environment", {})
                self.assertFalse(any("MATCH_SQUAD" in key for key in environment),
                                 "base Compose must retain its existing provider policy")
        proxy = (ROOT / "docker/phase0/nginx.conf").read_text(encoding="utf-8")
        self.assertIn("location = /matchmaking/jwks.json", proxy)
        self.assertIn("if ($request_method != GET) { return 405; }", proxy)
        self.assertIn("alias /run/phase0/matchmaking/jwks.json;", proxy)
        self.assertIn("location = /gateway-bootstrap/jwks.json", proxy)
        self.assertIn("alias /run/phase0/gateway/jwks.json;", proxy)
        self.assertIn("location = /gateway/jwks.json", proxy)
        self.assertIn("proxy_pass http://$gateway_issuer/.well-known/voice-principal-jwks.json;", proxy)
        self.assertIn("location / { return 404; }", proxy)

    def media_source_at(self, service, target, merged=None):
        selected = merged or self.media_merged
        for volume in selected[service].get("volumes", []):
            if volume.get("type") == "bind" and volume.get("target") == target:
                source = Path(volume["source"])
                if source.is_relative_to(self.fixture):
                    self.assertTrue(volume.get("read_only"), f"{service} fixture mount must stay read-only: {target}")
                    return source.relative_to(self.fixture).as_posix()
        self.fail(f"{service} config path has no Phase0 fixture mount: {target}")

    def test_all_media_overlay_fixture_producers_enable_nonroot_material(self):
        wrappers = (
            "scripts/ci/compose-a1-flutter-profile-handoff.sh",
            "scripts/ci/compose-a1-multi-account-proof.sh",
            "scripts/ci/compose-file-attachment-restart-proof.sh",
        )
        for relative in wrappers:
            source = (ROOT / relative).read_text(encoding="utf-8")
            self.assertIn("docker-compose.voice-space-media.yml", source, relative)
            self.assertIn("export PHASE0_MATCHSQUAD_FIXTURE_UID=65532", source, relative)
            self.assertIn("restore-phase0-fixture-owner.py", source, relative)
            self.assertIn("generate-phase0-fixture.py", source, relative)

        workflows = (
            (".github/workflows/ci.yml", "compose-e2e", "Generate disposable media principal and TLS fixture"),
            (".github/workflows/t31-session-events-e2e.yml", "compose-https-claim-inbox-ack",
             "Generate isolated Phase0 and HTTPS certificates"),
        )
        for relative, job, name in workflows:
            workflow = yaml.safe_load((ROOT / relative).read_text(encoding="utf-8"))
            producer = next(step for step in workflow["jobs"][job]["steps"]
                            if step.get("name") == name)
            self.assertEqual(producer.get("env", {}).get("PHASE0_MATCHSQUAD_FIXTURE_UID"), "65532",
                             f"{relative}:{job}:{name}")
        t31 = (ROOT / ".github/workflows/t31-session-events-e2e.yml").read_text(encoding="utf-8")
        ci = (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")
        self.assertIn("restore-phase0-fixture-owner.py", t31)
        self.assertIn("restore-phase0-fixture-owner.py", ci)
        restorer = (ROOT / "scripts/dev/restore-phase0-fixture-owner.py").read_text(encoding="utf-8")
        for root_name in ("phase0-fixture", "credentials", "voice-space-media-phase0"):
            self.assertIn(f'root.name == "{root_name}"', restorer)
        self.assertIn("def supported_root(", restorer)
        self.assertIn("if not supported_root(root, runner_temp, declared_root):", restorer)
        self.assertIn("for path in paths:", restorer)
        self.assertNotIn("os.chown(root", restorer)
        self.assertIn('os.environ.get("PHASE0_FIXTURE_DIR")', restorer)
        self.assertIn("declared_root != root", restorer)
        self.assertNotIn('"-R"', restorer, "cleanup must only chown enumerated fixture paths")

        self.assertIn("docker-compose.voice-space-media.yml",
                      (ROOT / "scripts/ci/t31-session-event-compose-bootstrap.py").read_text(encoding="utf-8"))

    def test_delivery_ack_has_authoritative_messaging_reader(self):
        for name, services in (("base", self.base), ("phase0", self.merged),
                               ("media", self.media_merged)):
            with self.subTest(compose=name):
                realtime = services["realtime"]
                self.assertEqual(realtime["environment"]["REALTIME_MESSAGING_GRPC_ADDR"],
                                 "messaging:9090")
                self.assertEqual(realtime["depends_on"]["messaging"]["condition"],
                                 "service_healthy")

    def test_bounded_startup_diagnostics_only_name_declared_services(self):
        path = ROOT / "scripts/ci/compose-file-attachment-restart-proof.sh"
        text = path.read_text()
        batches = re.findall(r"logs --no-color --timestamps --tail=100 ([^\n]+)", text)
        self.assertEqual(len(batches), 1)
        names = shlex.split(re.split(r">&2|\|\|", batches[0])[0])
        self.assertIn("matchmaking", names)
        for name in names:
            self.assertTrue(name in self.merged, f"undeclared diagnostic service {name} in {path.name}")

        helper = STARTUP_DIAGNOSTICS.read_text()
        workflow = (ROOT / ".github/workflows/t31-session-events-e2e.yml").read_text()
        self.assertIn("scripts/ci/t31-startup-diagnostics.py", workflow)
        self.assertIn('compose + ["logs", "--no-color", "--timestamps", *log_services]', helper)
        self.assertIn("log_services = requested or failed_services or SERVICES", helper)
        self.assertNotIn("--tail", helper)

        service_block = re.search(r"SERVICES = \((.*?)\n\)", helper, flags=re.DOTALL)
        self.assertIsNotNone(service_block)
        names = re.findall(r'"([a-z0-9-]+)"', service_block.group(1))
        t31_overlay = (ROOT / "docker-compose.t31-session-events-e2e.yml").read_text()
        for name in names:
            declared = (name in self.media_merged or name in self.merged
                        or bool(re.search(rf"^\s*{re.escape(name)}:\s*$", t31_overlay, flags=re.MULTILINE)))
            self.assertTrue(declared, f"undeclared diagnostic service {name} in T31 helper")
        self.assertNotIn("container_id", helper)

        start = workflow.index("      - name: Save sanitized T31 acceptance evidence")
        end = workflow.index("      - name: Remove this run's isolated Compose project and fixtures", start)
        artifact_step = workflow[start:end]
        artifacts = re.findall(r"^\s+(tmp/[^\s]+)$", artifact_step, flags=re.MULTILINE)
        self.assertEqual(artifacts, [
            "tmp/t31-session-event-e2e/bootstrap-result.json",
            "tmp/t31-session-event-e2e/acceptance.log",
            "tmp/t31-session-event-e2e/startup-diagnostics.json",
        ])
        self.assertNotIn("compose.env", artifact_step)
        self.assertNotIn("/state/", artifact_step)
        self.assertNotIn("/tls/", artifact_step)
        self.assertIn('startup_status=$?', workflow)
        self.assertIn('python3 scripts/ci/t31-startup-diagnostics.py', workflow)
        self.assertIn('--service voice', workflow)
        self.assertIn('--include-matchmaking-startup', workflow)
        self.assertIn('|| true', workflow)
        self.assertIn('exit "$startup_status"', workflow)
        self.assertNotIn("logs --no-color --timestamps", workflow)

    def test_livekit_uses_single_published_udp_mux_port(self):
        config = (ROOT / "deploy/livekit/livekit.yaml").read_text(encoding="utf-8")
        self.assertRegex(config, r"(?m)^rtc:\s*\n(?:^[ \t]+[^\n]*\n)*?^[ \t]+udp_port:\s*7882\s*$")
        self.assertNotRegex(config, r"(?m)^\s+port_range_(?:start|end):")
        livekit_ports = self.base["livekit"]["ports"]
        port_contract = [{key: port[key] for key in ("target", "published", "protocol")}
                         for port in livekit_ports]
        self.assertIn({"target": 7882, "published": "7882", "protocol": "udp"}, port_contract)
        self.assertIn({"target": 7881, "published": "7881", "protocol": "tcp"}, port_contract)

    def test_t31_log_summary_only_emits_allowlisted_diagnostic_fields(self):
        secret = "private-dsn-and-password-sentinel"
        raw = (
            "voice  | level=error error_code=UNAVAILABLE SQLSTATE=08006 "
            "SQLNonTransientConnectionException /app/main.go:42 password=" + secret + "\n"
        ).encode("utf-8")
        summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
        self.assertEqual(summary["result"], "complete")
        self.assertEqual(first["result"], "observed_in_bounded_window")
        self.assertEqual(first["service"], "voice")
        self.assertEqual(first["stage"], "database")
        self.assertEqual(first["error_code"], "UNAVAILABLE")
        self.assertEqual(first["exception_class"], "SQLNonTransientConnectionException")
        self.assertEqual(first["sqlstate"], "08006")
        self.assertEqual(first["source_basename"], "main.go")
        self.assertEqual(first["source_line"], 42)
        self.assertEqual(first["startup_category"], "unknown")
        self.assertEqual(first["startup_detail"], "unknown")
        self.assertEqual(first["selection"], "first_marker_in_bounded_head")
        self.assertEqual(first["causal_order"], "not_established")
        self.assertNotIn(secret, json.dumps({"summary": summary, "first": first}))
        self.assertEqual(set(first), {
            "result", "service", "stage", "error_code", "startup_category", "startup_detail",
            "exception_class", "sqlstate",
            "source_basename", "source_line", "selection", "causal_order",
        })

    def test_t31_voice_startup_prefix_emits_only_allowlisted_category(self):
        secret = "private-dsn-sentinel"
        for container in ("voice", "voice-1", "private-project-voice-1", "private_project_voice_1"):
            with self.subTest(container=container):
                raw = (
                    container + " | 2026-10-12T12:34:56.123456789Z 2026/10/12 12:34:56 "
                    "Space media initial reconciliation: " + secret + "\n"
                ).encode("utf-8")
                summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
                self.assertEqual(summary["services_seen"], ["voice"])
                self.assertEqual(first["result"], "observed_in_bounded_window")
                self.assertEqual(first["startup_category"], "space_media_initial_reconciliation")
                serialized = json.dumps({"summary": summary, "first": first})
                self.assertNotIn(secret, serialized)
                self.assertNotIn("private", serialized)

        raw = b"private-service-1 | ERROR private-dsn-sentinel\n"
        summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
        self.assertEqual(summary["services_seen"], [])
        self.assertEqual(first["result"], "no_error_marker")

    def test_t31_matchmaking_startup_marker_is_service_scoped_and_redacted(self):
        secret = "private-jwks-url-and-password-sentinel"
        raw = ("matchmaking-1 | level=error error_code=UNAVAILABLE "
               "ConnectionError Matchmaking Space principal runtime: gateway JWKS /app/runtime.go:128 endpoint="
               + secret + "\n").encode("utf-8")
        requested = ("voice", "matchmaking")
        summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("matchmaking",), requested)
        self.assertEqual(summary["services_requested"], ["voice", "matchmaking"])
        self.assertEqual(summary["services_seen"], ["matchmaking"])
        self.assertEqual(first["service"], "matchmaking")
        self.assertEqual(first["startup_category"], "matchmaking_space_principal_runtime")
        self.assertEqual(first["stage"], "identity_dependency")
        self.assertEqual(first["error_code"], "UNAVAILABLE")
        self.assertEqual(first["exception_class"], "ConnectionError")
        self.assertNotIn(secret, json.dumps({"summary": summary, "first": first}))

    def test_t31_startup_collection_requests_only_voice_and_matchmaking(self):
        commands = []
        status = json.dumps([
            {"Service": "voice", "State": "running", "Health": "healthy", "ExitCode": 0},
            {"Service": "matchmaking", "State": "exited", "Health": "none", "ExitCode": 1},
        ]).encode("ascii")
        logs = b"matchmaking-1 | level=error Matchmaking Space principal runtime: gateway JWKS unavailable\n"

        def bounded(command, timeout, limit):
            commands.append(command)
            if "ps" in command:
                return status, "complete", None
            return logs, "complete", None

        with patch.object(startup_diagnostics, "_bounded_command", side_effect=bounded):
            record = startup_diagnostics.build_diagnostics(
                ["docker", "compose"], 1, "voice", include_matchmaking_startup=True)
        self.assertEqual(commands[0][-2:], ["voice", "matchmaking"])
        self.assertEqual(commands[1][-2:], ["voice", "matchmaking"])
        self.assertEqual(record["first_error"]["service"], "matchmaking")
        self.assertEqual(record["first_error"]["startup_category"],
                         "matchmaking_space_principal_runtime")

    def test_voice_startup_categories_cover_static_fatal_sites_without_dynamic_text(self):
        source = "\n".join((ROOT / path).read_text() for path in (
            "src/backend/voice/main.go", "src/backend/voice/space_media_runtime.go"))
        prefixes = dict(startup_diagnostics.VOICE_STARTUP_PREFIXES)
        for literal in re.findall(r'log\.Fatalf?\("([^"\n]+)"', source):
            prefix = literal.split("%", 1)[0].rstrip()
            self.assertIn(prefix, prefixes)
            raw = ("voice-1 | 2026-10-12T12:34:56Z " + prefix + " private-sentinel\n").encode()
            summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
            self.assertEqual(first["startup_category"], prefixes[prefix])
            self.assertNotIn("private-sentinel", json.dumps({"summary": summary, "first": first}))

    def test_voice_p3_startup_categories_are_finite_and_redacted(self):
        cases = {
            "P3 requires durable lifecycle-fenced Redis": "voice_p3_durable_redis_required",
            "Space requires an incarnation-fenced projection store": "voice_space_incarnation_projection_required",
            "P3 requires lifecycle-fenced Redis projection": "voice_p3_projection_fence_required",
        }
        for message, expected in cases.items():
            with self.subTest(category=expected):
                raw = ("voice | " + message + " private-sentinel\n").encode()
                summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
                self.assertEqual(first["startup_category"], expected)
                self.assertNotIn("private-sentinel", json.dumps({"summary": summary, "first": first}))
        summary, first = startup_diagnostics.sanitize_logs(
            b"voice | P3 requires unknown-private-sentinel\n", "complete", ("voice",))
        self.assertEqual(first["startup_category"], "unknown")
        self.assertNotIn("private-sentinel", json.dumps({"summary": summary, "first": first}))

    def test_t31_match_squad_runtime_detail_is_allowlisted_and_redacted(self):
        secret = "private-certificate-or-endpoint-detail"
        cases = {
            "voice MatchSquad TLS certificate/key unavailable": "voice_match_squad_tls_server_identity",
            "voice MatchSquad client CA unavailable": "voice_match_squad_tls_client_ca_unavailable",
            "voice MatchSquad JWKS unavailable": "voice_match_squad_jwks_refresh_unavailable",
            "voice MatchSquad JWKS has no usable key": "voice_match_squad_jwks_document_invalid",
        }
        for message, expected in cases.items():
            with self.subTest(expected=expected):
                raw = ("voice-1 | voice MatchSquad principal runtime: " + message + " " + secret + "\n").encode()
                summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
                self.assertEqual(summary["lines_read"], 1)
                self.assertEqual(first["startup_category"], "voice_match_squad_principal_runtime")
                self.assertEqual(first["startup_detail"], expected)
                self.assertNotIn(secret, json.dumps({"summary": summary, "first": first}))

        line = "voice | voice MatchSquad principal runtime: voice MatchSquad JWKS unavailable".ljust(131) + "\n"
        raw = line.encode("utf-8")
        self.assertEqual(len(raw), 132)
        summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
        self.assertEqual(summary["lines_read"], 1)
        self.assertEqual(first["startup_category"], "voice_match_squad_principal_runtime")
        self.assertEqual(first["startup_detail"], "voice_match_squad_jwks_refresh_unavailable")

    def test_t31_readiness_failure_is_allowlisted_and_redacted(self):
        secret = "private-endpoint-or-certificate-value"
        readiness = "phase0-matchmaking-jwks-readiness"
        status = json.dumps([{
            "Service": readiness, "State": "exited", "Health": "none", "ExitCode": 1,
            "Name": secret, "ID": secret,
        }]).encode("utf-8")
        status_summary, failed = startup_diagnostics.sanitize_status(status, "complete")
        self.assertEqual(failed, (readiness,))
        self.assertEqual(status_summary["services"], [
            {"service": readiness, "state": "exited", "health": "none", "exit_code": 1},
        ])

        raw = (readiness + "-1 | 2026-10-16T12:58:00Z "
               "phase0_matchmaking_jwks_readiness_failed category=jwks_tls_verification_failed\n").encode("utf-8")
        log_summary, first = startup_diagnostics.sanitize_logs(raw, "complete", failed)
        self.assertEqual(log_summary["services_seen"], [readiness])
        self.assertEqual(first["service"], readiness)
        self.assertEqual(first["stage"], "identity_dependency")
        self.assertEqual(first["startup_category"], "phase0_matchmaking_jwks_readiness")
        self.assertEqual(first["startup_detail"], "jwks_tls_verification_failed")
        self.assertNotIn(secret, json.dumps({"status": status_summary, "logs": log_summary, "first": first}))

        unknown = (readiness + " | phase0_matchmaking_jwks_readiness_failed "
                   "category=private_value\n").encode("utf-8")
        _, first = startup_diagnostics.sanitize_logs(unknown, "complete", (readiness,))
        self.assertEqual(first["startup_category"], "phase0_matchmaking_jwks_readiness")
        self.assertEqual(first["startup_detail"], "unknown")
        self.assertNotIn("private_value", json.dumps(first))

        calls = []

        def bounded(command, _timeout, _limit):
            calls.append(command)
            return (status if "ps" in command else raw, "complete", 0)

        with patch.object(startup_diagnostics, "_bounded_command", side_effect=bounded):
            record = startup_diagnostics.build_diagnostics(["docker", "compose"], 1)
        self.assertIn(readiness, calls[1])
        self.assertEqual(record["first_error"]["startup_category"], "phase0_matchmaking_jwks_readiness")
        self.assertEqual(record["first_error"]["startup_detail"], "jwks_tls_verification_failed")
        self.assertNotIn(secret, json.dumps(record))

    def test_t31_voice_match_squad_member_startup_details_are_allowlisted_and_redacted(self):
        secret = "private-redis-password-or-jwks-url"
        cases = {
            "voice MatchSquad member TLS identity unavailable": "voice_match_squad_member_tls_identity_unavailable",
            "voice MatchSquad member client CA unavailable": "voice_match_squad_member_client_ca_unavailable",
            "voice MatchSquad member client CA is invalid": "voice_match_squad_member_client_ca_invalid",
            "system CA roots unavailable for delegated user JWKS": "voice_match_squad_member_jwks_system_trust_unavailable",
            "voice MatchSquad member JWKS CA is invalid": "voice_match_squad_member_jwks_ca_invalid",
            "voice MatchSquad member Redis unavailable": "voice_match_squad_member_redis_unavailable",
            "voice delegated user JWKS unavailable": "voice_match_squad_member_jwks_refresh_unavailable",
        }
        for message, expected in cases.items():
            with self.subTest(expected=expected):
                raw = ("voice-1 | voice MatchSquad member principal runtime: " + message + " " + secret + "\n").encode()
                summary, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
                self.assertEqual(summary["lines_read"], 1)
                self.assertEqual(first["startup_category"], "voice_match_squad_member_principal_runtime")
                self.assertEqual(first["startup_detail"], expected)
                self.assertNotIn(secret, json.dumps({"summary": summary, "first": first}))

    def test_media_voice_waits_for_verified_public_match_squad_jwks(self):
        readiness = self.media_merged["phase0-matchmaking-jwks-readiness"]
        self.assertEqual(readiness["image"], "node:24-alpine")
        self.assertEqual(readiness["user"], "65532:65532")
        self.assertTrue(readiness["read_only"])
        self.assertEqual(readiness["cap_drop"], ["ALL"])
        self.assertEqual(readiness["security_opt"], ["no-new-privileges:true"])
        self.assertEqual(readiness["command"], ["node", "/run/phase0/readiness.js"])
        self.assertIn("app", readiness.get("profiles", []))
        self.assertEqual(self.media_source_at("phase0-matchmaking-jwks-readiness", "/run/phase0/ca.crt"),
                         "ca/ca.crt")
        ca_mount = next(volume for volume in readiness["volumes"]
                        if isinstance(volume, dict) and volume.get("target") == "/run/phase0/ca.crt")
        self.assertTrue(ca_mount["read_only"])
        if CAN_SET_MATCHSQUAD_UID:
            ca_stat = self.fixture.joinpath("ca/ca.crt").stat()
            self.assertEqual((ca_stat.st_uid, ca_stat.st_gid), (65532, 65532))
            self.assertEqual(stat.S_IMODE(ca_stat.st_mode), 0o600)
        if os.name == "posix":
            for issuer in ("gateway", "matchmaking"):
                jwks_mode = stat.S_IMODE(self.fixture.joinpath(f"jwks/{issuer}/jwks.json").stat().st_mode)
                self.assertEqual(jwks_mode, 0o644, "nginx worker must be able to read the public JWKS")
        readiness_script = next(volume for volume in readiness["volumes"]
                                 if isinstance(volume, dict)
                                 and volume.get("target") == "/run/phase0/readiness.js")
        self.assertEqual(Path(readiness_script["source"]).resolve(),
                         (ROOT / "scripts/dev/phase0-matchmaking-jwks-readiness.js").resolve())
        self.assertTrue(readiness_script["read_only"])
        self.assertEqual(readiness["depends_on"]["matchmaking"]["condition"], "service_started")
        self.assertNotIn("phase0-jwks", readiness["depends_on"],
                         "Matchmaking must publish its own issuer keys, not use the Phase0 proxy")
        for consumer in ("chat", "voice"):
            self.assertEqual(self.media_merged[consumer]["depends_on"][
                "phase0-matchmaking-jwks-readiness"]["condition"], "service_completed_successfully",
                f"{consumer} must wait for verified Matchmaking JWKS over HTTPS")
        self.assertEqual(self.media_merged["voice"]["depends_on"][
            "phase0-matchmaking-jwks-readiness"]["condition"], "service_completed_successfully")
        self.assertEqual(self.media_merged["voice"]["environment"]["VOICE_MATCH_SQUAD_PRINCIPAL_JWKS_URL"],
                         MATCHMAKING_JWKS_URL)
        probe = (ROOT / "scripts/dev/phase0-matchmaking-jwks-readiness.js").read_text(encoding="utf-8")
        for marker in ("https.get", "ca,", "servername: endpoint.hostname", "response.statusCode !== 200",
                       '"application/jwk-set+json", "application/json"',
                       "document.keys", 'key.kty === "RSA"', 'key.alg === "RS256"',
                       'Object.hasOwn(key, privateField)', '"dp", "dq", "qi", "oth"',
                       "phase0_matchmaking_jwks_readiness_failed category=", "ca_file_unavailable",
                       "jwks_tls_verification_failed", "jwks_endpoint_unavailable",
                       "jwks_http_response_invalid", "jwks_document_invalid", "jwks_response_too_large",
                       "jwks_request_timeout", "jwks_transport_failed", "const ATTEMPT_TIMEOUT_MS = 2000",
                       "deadline = setTimeout(", "clearTimeout(deadline)", "request.destroy()"):
            self.assertIn(marker, probe)
        self.assertNotIn("timeout: 2000", probe)
        self.assertIn("phase0-matchmaking-jwks-readiness", startup_diagnostics.SERVICES)

    def test_startup_diagnostics_can_scope_status_and_logs_to_voice(self):
        status = json.dumps([{
            "Service": "voice", "State": "exited", "Health": "none", "ExitCode": 1,
        }]).encode("utf-8")
        logs = b"voice | level=error error_code=UNAVAILABLE\n"
        calls = []

        def bounded(command, _timeout, _limit):
            calls.append(command)
            return (status if "ps" in command else logs, "complete", 0)

        with patch.object(startup_diagnostics, "_bounded_command", side_effect=bounded):
            record = startup_diagnostics.build_diagnostics(
                ["docker", "compose", "-f", "docker-compose.yml"], 1, "voice"
            )

        self.assertEqual(calls[0][-1], "voice")
        self.assertEqual(calls[1][-1], "voice")
        self.assertEqual(record["status_collection"]["services"], [
            {"service": "voice", "state": "exited", "health": "none", "exit_code": 1},
        ])
        self.assertEqual(record["log_collection"]["services_requested"], ["voice"])
        self.assertEqual(record["first_error"]["service"], "voice")

    def test_t31_first_emitted_window_does_not_claim_later_tail_cause(self):
        early = b"voice | level=info service_starting\n"
        early += b"chat | level=info service_starting\n"
        head = (early * ((startup_diagnostics.MAX_LOG_BYTES // len(early)) + 1))[:startup_diagnostics.MAX_LOG_BYTES]
        later = b"voice | level=error error_code=UNAVAILABLE\n"
        full_stream = head + later
        bounded = full_stream[:startup_diagnostics.MAX_LOG_BYTES]
        self.assertNotIn(later, bounded)
        summary, first = startup_diagnostics.sanitize_logs(bounded, "byte_limit", ("voice",))
        self.assertEqual(summary["window"], "first_emitted")
        self.assertEqual(summary["coverage"], "bounded_head_window_only")
        self.assertTrue(summary["truncated"])
        self.assertEqual(summary["causal_order"], "not_established")
        self.assertEqual(first["result"], "unknown")
        self.assertEqual(first["causal_order"], "not_established")

    def test_t31_unrecognized_diagnostic_values_remain_unknown(self):
        secret = "must-not-leak"
        raw = (
            "voice | level=error error_code=" + secret + " SQLSTATE=99999 "
            "UnknownPrivateException /tmp/secret-path/private.go:700 password=" + secret + "\n"
        ).encode("utf-8")
        _, first = startup_diagnostics.sanitize_logs(raw, "complete", ("voice",))
        self.assertEqual(first["error_code"], "unknown")
        self.assertEqual(first["exception_class"], "unknown")
        self.assertEqual(first["sqlstate"], "unknown")
        self.assertEqual(first["source_basename"], "unknown")
        self.assertNotIn(secret, json.dumps(first))
        self.assertNotIn("private.go", json.dumps(first))

    def test_t31_status_discards_unrecognized_services_and_identifiers(self):
        raw = json.dumps([
            {"Service": "voice", "State": "exited", "Health": "none", "ExitCode": 1,
             "Name": "container-id-secret", "ID": "opaque-container-id"},
            {"Service": "unknown-service", "State": "exited", "ExitCode": 77},
        ]).encode("utf-8")
        summary, failed = startup_diagnostics.sanitize_status(raw, "complete")
        self.assertEqual(failed, ("voice",))
        self.assertEqual(summary["services"], [
            {"service": "voice", "state": "exited", "health": "none", "exit_code": 1},
        ])
        self.assertNotIn("container-id-secret", json.dumps(summary))
        self.assertNotIn("opaque-container-id", json.dumps(summary))

    def test_t31_truncated_logs_do_not_claim_an_error(self):
        raw = b"voice | level=error " + b"x" * (startup_diagnostics.MAX_LOG_BYTES + 1)
        bounded = raw[:startup_diagnostics.MAX_LOG_BYTES]
        summary, first = startup_diagnostics.sanitize_logs(bounded, "byte_limit", ("voice",))
        self.assertEqual(summary["result"], "byte_limit")
        self.assertTrue(summary["truncated"])
        self.assertEqual(first["result"], "unknown")
        self.assertLessEqual(summary["bytes_read"], startup_diagnostics.MAX_LOG_BYTES)

    @unittest.skipUnless(os.name == "posix", "process-group bounds are exercised on Linux CI")
    def test_t31_direct_compose_process_wait_and_output_are_bounded(self):
        ok = startup_diagnostics._bounded_command(
            [sys.executable, "-c", "print('bounded-stub')"], 2.0, 128
        )
        self.assertEqual(ok, (b"bounded-stub\n", "complete", 0))
        too_much = startup_diagnostics._bounded_command(
            [sys.executable, "-c", "print('x' * 10000)"], 2.0, 128
        )
        self.assertEqual(too_much[1], "byte_limit")
        self.assertLessEqual(len(too_much[0]), 128)
        timed_out = startup_diagnostics._bounded_command(
            [sys.executable, "-c", "import time; time.sleep(10)"], 0.05, 128
        )
        self.assertEqual(timed_out[1], "timeout")
        self.assertIsNotNone(timed_out[2])

    def test_fixture_directory_is_required(self):
        env = dict(self.env)
        env.pop("PHASE0_FIXTURE_DIR")
        self.assertNotEqual(run(*self.command(True), env=env).returncode, 0)

    def test_every_fixture_bind_source_is_generated(self):
        for service in self.merged:
            for source, volume in self.fixture_mounts(service):
                with self.subTest(service=service, source=source):
                    path = self.fixture / source
                    self.assertTrue(path.exists(), "Compose bind source is absent from generated fixture")
                    if path.is_dir():
                        self.assertTrue(any(path.iterdir()), "signer directory is empty")
                    else:
                        self.assertGreater(path.stat().st_size, 0)
                    self.assertTrue(volume.get("read_only"), "fixture credentials must be mounted read-only")

    def test_frozen_environment_and_mounted_paths(self):
        for issuer in ("gateway", "space"):
            env = self.merged[issuer]["environment"]
            prefix = issuer.upper() + "_PRINCIPAL_"
            self.assertEqual(env[prefix + "ACTIVE_KID"], "current")
            self.assertEqual(self.source_at(issuer, env[prefix + "SIGNING_KEYS_DIR"]), issuer)
        space = self.merged["space"]["environment"]
        self.assertEqual(space["ROLE_PRINCIPAL_GRPC_ADDR"], "role:9091")
        self.assertEqual(self.source_at("space", space["ROLE_PRINCIPAL_TLS_CA_FILE"]), "ca/ca.crt")
        self.assertEqual(self.source_at("space", space["SPACE_ROLE_CLIENT_CERT_FILE"]), "tls/space-client.crt")
        self.assertEqual(self.source_at("space", space["SPACE_ROLE_CLIENT_KEY_FILE"]), "tls/space-client.key")
        gis = self.merged["gameintegration"]["environment"]
        self.assertEqual(gis["GAME_INTEGRATION_PRINCIPAL_KID"], "current")
        self.assertEqual(self.source_at("gameintegration", gis["GAME_INTEGRATION_PRINCIPAL_PRIVATE_KEY_FILE"]), "gameintegration/current.pem")
        self.assertEqual(gis["GAME_INTEGRATION_PRINCIPAL_NEXT_KID"], "next")
        self.assertEqual(self.source_at("gameintegration", gis["GAME_INTEGRATION_PRINCIPAL_NEXT_PRIVATE_KEY_FILE"]), "gameintegration/next.pem")
        self.assertEqual(self.source_at("gameintegration", gis["GAME_INTEGRATION_PRINCIPAL_JWKS_TLS_CERT_FILE"]), "tls/gameintegration-jwks.crt")
        self.assertEqual(self.source_at("gameintegration", gis["GAME_INTEGRATION_PRINCIPAL_JWKS_TLS_KEY_FILE"]), "tls/gameintegration-jwks.key")
        self.assertEqual(gis["GAME_INTEGRATION_PRINCIPAL_JWKS_LISTEN_ADDR"], ":8443")
        self.assertEqual(gis["GIS_ROLE_GRPC_ADDR"], "role:9091")
        self.assertEqual(self.source_at("gameintegration", gis["GIS_ROLE_TLS_CA_FILE"]), "ca/ca.crt")
        self.assertEqual(self.source_at("gameintegration", gis["GIS_ROLE_CLIENT_CERT_FILE"]), "tls/gameintegration-client.crt")
        self.assertEqual(self.source_at("gameintegration", gis["GIS_ROLE_CLIENT_KEY_FILE"]), "tls/gameintegration-client.key")
        for prefix, destination, cert, key in (
                ("GIS_CHAT", "chat:9091", "tls/gameintegration-chat-client.crt", "tls/gameintegration-chat-client.key"),
                ("GIS_VOICE", "voice:9091", "tls/gameintegration-voice-client.crt", "tls/gameintegration-voice-client.key")):
            self.assertEqual(gis[prefix + "_GRPC_ADDR"], destination)
            self.assertEqual(self.source_at("gameintegration", gis[prefix + "_TLS_CA_FILE"]), "ca/ca.crt")
            self.assertEqual(self.source_at("gameintegration", gis[prefix + "_CLIENT_CERT_FILE"]), cert)
            self.assertEqual(self.source_at("gameintegration", gis[prefix + "_CLIENT_KEY_FILE"]), key)
        voice = self.merged["voice"]["environment"]
        self.assertEqual(voice["VOICE_PRINCIPAL_KID"], "current")
        self.assertEqual(self.source_at("voice", voice["VOICE_PRINCIPAL_PRIVATE_KEY_FILE"]), "voice/current.pem")
        self.assertEqual(voice["VOICE_PRINCIPAL_NEXT_KID"], "next")
        self.assertEqual(self.source_at("voice", voice["VOICE_PRINCIPAL_NEXT_PRIVATE_KEY_FILE"]), "voice/next.pem")
        self.assertEqual(self.source_at("voice", voice["VOICE_PRINCIPAL_JWKS_TLS_CERT_FILE"]), "tls/voice-jwks.crt")
        self.assertEqual(self.source_at("voice", voice["VOICE_PRINCIPAL_JWKS_TLS_KEY_FILE"]), "tls/voice-jwks.key")
        self.assertEqual(voice["VOICE_PRINCIPAL_JWKS_LISTEN_ADDR"], ":8443")
        self.assertEqual(voice["VOICE_GAME_PRINCIPAL_JWKS_URL"], "https://gameintegration:8443/internal/v1/principal/jwks.json")
        self.assertEqual(self.source_at("voice", voice["VOICE_GAME_PRINCIPAL_JWKS_CA_FILE"]), "ca/ca.crt")
        self.assertEqual(voice["VOICE_GAME_PRINCIPAL_GRPC_LISTEN"], ":9091")
        self.assertEqual(self.source_at("voice", voice["VOICE_GAME_PRINCIPAL_TLS_CERT_FILE"]), "tls/voice-game-grpc.crt")
        self.assertEqual(self.source_at("voice", voice["VOICE_GAME_PRINCIPAL_TLS_KEY_FILE"]), "tls/voice-game-grpc.key")
        self.assertEqual(self.source_at("voice", voice["VOICE_GAME_PRINCIPAL_CLIENT_CA_FILE"]), "ca/gameintegration-client-ca.crt")
        self.assertEqual(voice["VOICE_GAME_PRINCIPAL_REPLAY_REDIS_ADDR"], "redis:6379")
        self.assertEqual(voice["VOICE_ROLE_GRPC_ADDR"], "role:9091")
        self.assertEqual(self.source_at("voice", voice["VOICE_ROLE_TLS_CA_FILE"]), "ca/ca.crt")
        self.assertEqual(self.source_at("voice", voice["VOICE_ROLE_CLIENT_CERT_FILE"]), "tls/voice-client.crt")
        self.assertEqual(self.source_at("voice", voice["VOICE_ROLE_CLIENT_KEY_FILE"]), "tls/voice-client.key")
        role = self.merged["role"]["environment"]
        self.assertEqual(role["ROLE_PRINCIPAL_GRPC_LISTEN"], ":9091")
        self.assertEqual(role["ROLE_PRINCIPAL_REPLAY_REDIS_ADDR"], "redis:6379")
        self.assertEqual(json.loads(role["S2S_JWKS_URLS_JSON"]), {
            "space": URLS["space"],
            "gameintegration": "https://gameintegration:8443/internal/v1/principal/jwks.json",
            "voice": "https://voice:8443/internal/v1/principal/jwks.json",
        })
        self.assertEqual(self.source_at("role", role["S2S_JWKS_CA_FILE"]), "ca/ca.crt")
        self.assertEqual(self.source_at("role", role["ROLE_PRINCIPAL_CLIENT_CA_FILE"]), "ca/ca.crt")
        chat = self.merged["chat"]["environment"]
        self.assertEqual(chat["CHAT_GIS_GRPC_LISTEN"], ":9091")
        self.assertEqual(self.source_at("chat", chat["CHAT_GIS_TLS_CERT_FILE"]), "tls/chat-gis-grpc.crt")
        self.assertEqual(self.source_at("chat", chat["CHAT_GIS_TLS_KEY_FILE"]), "tls/chat-gis-grpc.key")
        self.assertEqual(self.source_at("chat", chat["CHAT_GIS_CLIENT_CA_FILE"]), "ca/gameintegration-client-ca.crt")
        self.assertEqual(chat["GAME_INTEGRATION_PRINCIPAL_JWKS_URL"], "https://gameintegration:8443/internal/v1/principal/jwks.json")
        self.assertEqual(self.source_at("chat", chat["GAME_INTEGRATION_PRINCIPAL_JWKS_CA_FILE"]), "ca/ca.crt")
        self.assertEqual(chat["GAME_INTEGRATION_PRINCIPAL_REPLAY_REDIS_ADDR"], "redis:6379")
        for key, cert, path in (("GIS_CHAT", "chat:9091", "chat-client"),
                                ("GIS_VOICE", "voice:9091", "voice-client")):
            self.assertEqual(gis[key + "_GRPC_ADDR"], cert)
            self.assertEqual(self.source_at("gameintegration", gis[key + "_TLS_CA_FILE"]), "ca/ca.crt")
            self.assertEqual(self.source_at("gameintegration", gis[key + "_CLIENT_CERT_FILE"]),
                             f"tls/gameintegration-{path}.crt")
            self.assertEqual(self.source_at("gameintegration", gis[key + "_CLIENT_KEY_FILE"]),
                             f"tls/gameintegration-{path}.key")
        auth = self.merged["auth"]["environment"]
        self.assertEqual(auth["AUTH_PRINCIPAL_GRPC_PORT"], "9091")
        self.assertEqual(json.loads(auth["S2S_JWKS_URLS_JSON"]), {
            **URLS, "voice": "https://voice:8443/internal/v1/principal/jwks.json",
        })
        self.assertNotIn("S2S_JWKS_CA_FILE", auth)
        options = auth.get("JAVA_TOOL_OPTIONS", "")
        trust = re.search(r"-Djavax\.net\.ssl\.trustStore=([^\s]+)", options)
        self.assertIsNotNone(trust)
        self.assertEqual(self.source_at("auth", trust.group(1)), "truststore.p12")
        self.assertIn("-Djavax.net.ssl.trustStorePassword=phase0-fixture", options)
        for service, prefix in (("role", "ROLE_PRINCIPAL_TLS_"), ("auth", "AUTH_GRPC_TLS_")):
            env = self.merged[service]["environment"]
            for suffix, ext in (("CERT_FILE", "crt"), ("KEY_FILE", "key")):
                self.assertEqual(self.source_at(service, env[prefix + suffix]), f"tls/{service}.{ext}")

    def test_base_environments_and_legacy_endpoints_are_preserved(self):
        declared_overrides = {}
        for overlay in (OVERLAY,):
            overlay_services = yaml.safe_load(overlay.read_text(encoding="utf-8")).get("services", {})
            for service, config in overlay_services.items():
                declared_overrides.setdefault(service, {}).update(config.get("environment", {}))
        for service, base in self.base.items():
            for name, value in base.get("environment", {}).items():
                overlay_value = declared_overrides.get(service, {}).get(name, ...)
                if overlay_value is not ...:
                    actual = self.merged[service]["environment"].get(name)
                    if overlay_value is None:
                        self.assertIsNone(actual, f"overlay did not clear {service}/{name}")
                    elif isinstance(overlay_value, str) and "$" in overlay_value:
                        match = re.fullmatch(r"\$\{([A-Z_][A-Z0-9_]*):-([^}]*)\}", overlay_value)
                        if match:
                            expected = self.env.get(match.group(1)) or match.group(2)
                            self.assertEqual(actual, expected, f"Compose interpolation changed: {service}/{name}")
                        else:
                            self.assertTrue(actual, f"Compose did not resolve {service}/{name}")
                    else:
                        self.assertEqual(actual, str(overlay_value),
                                         f"overlay value was not applied: {service}/{name}")
                    continue
                if service == "gameintegration" and name == "GAME_INTEGRATION_CREDENTIAL_KEY_B64":
                    self.assertEqual(len(base64.b64decode(self.merged[service]["environment"][name])), 32)
                    continue
                self.assertEqual(self.merged[service]["environment"].get(name), value,
                                 f"base environment changed: {service}/{name}")
            self.assertEqual(self.merged[service].get("ports", []), base.get("ports", []))
        self.assertEqual(self.merged["role"]["environment"]["ROLE_GRPC_LISTEN"], ":9090")
        self.assertEqual(self.merged["space"]["environment"]["ROLE_GRPC_ADDR"], "role:9090")

    def test_private_mounts_are_readonly_and_service_isolated(self):
        allowed = {"gateway": {"gateway", "ca/ca.crt", "tls/gateway-client.crt", "tls/gateway-client.key"}, "space": {"space", "ca/ca.crt", "tls/space-client.crt", "tls/space-client.key"},
                   "role": {"tls/role.crt", "tls/role.key", "ca/ca.crt"},
                    "gameintegration": {"gameintegration/current.pem", "gameintegration/next.pem", "tls/gameintegration-jwks.crt", "tls/gameintegration-jwks.key", "tls/gameintegration-client.crt", "tls/gameintegration-client.key", "tls/gameintegration-chat-client.crt", "tls/gameintegration-chat-client.key", "tls/gameintegration-voice-client.crt", "tls/gameintegration-voice-client.key", "ca/ca.crt"},
                    "voice": {"voice/current.pem", "voice/next.pem", "tls/voice-jwks.crt", "tls/voice-jwks.key", "tls/voice-game-grpc.crt", "tls/voice-game-grpc.key", "tls/voice-client.crt", "tls/voice-client.key", "ca/ca.crt", "ca/gameintegration-client-ca.crt"},
                    "chat": {"tls/chat-gis-grpc.crt", "tls/chat-gis-grpc.key", "ca/ca.crt", "ca/gameintegration-client-ca.crt"},
                   "auth": {"tls/auth.crt", "tls/auth.key", "truststore.p12", "ca/ca.crt"},
                   PROXY: {"tls/proxy.crt", "tls/proxy.key", "ca/ca.crt"}}
        allowed["space"].update({"tls/space-lifecycle.crt", "tls/space-lifecycle.key",
                                 "tls/space-lifecycle-client.crt", "tls/space-lifecycle-client.key"})
        allowed["gameintegration"].update({"tls/gameintegration-messaging-client.crt",
                                           "tls/gameintegration-messaging-client.key"})
        allowed["voice"].add("ca/space-lifecycle-client-ca.crt")
        allowed["realtime"] = {"ca/ca.crt"}
        allowed["bot"] = {"bot", "tls/bot-game-event.crt", "tls/bot-game-event.key",
                          "tls/gameintegration-messaging-client.crt", "tls/gameintegration-messaging-client.key",
                          "ca/ca.crt", "ca/gameintegration-client-ca.crt", "ca/space-lifecycle-client-ca.crt"}
        allowed["notification"] = {"tls/notification-lifecycle.crt", "tls/notification-lifecycle.key",
                                   "ca/ca.crt", "ca/space-lifecycle-client-ca.crt"}
        allowed["matchmaking"] = {"ca/ca.crt", "tls/matchmaking-complete-grpc.crt",
                                   "tls/matchmaking-complete-grpc.key", "tls/matchmaking-jwks.crt",
                                   "tls/matchmaking-jwks.key"}
        allowed["phase0-jwks"] = {"jwks/matchmaking/jwks.json", "jwks/gateway/jwks.json",
                                   "tls/proxy.crt", "tls/proxy.key", "ca/ca.crt"}
        allowed["messaging"] = {"tls/gameintegration-messaging-client.crt", "tls/gameintegration-messaging-client.key",
                                "tls/messaging-gameintegration-grpc.crt", "tls/messaging-gameintegration-grpc.key",
                                "tls/gameintegration-client.crt", "tls/gameintegration-client.key",
                                "ca/ca.crt", "ca/gameintegration-client-ca.crt"}
        for service in self.merged:
            for source, mount in self.fixture_mounts(service):
                self.assertIn(source, allowed.get(service, set()), f"fixture exposed to {service}")
                self.assertTrue(mount.get("read_only"), f"writable fixture: {service}/{source}")
        self.assertEqual(self.merged[PROXY].get("user"), "0:0")
        for service in ("role", "auth", PROXY):
            self.assertFalse(self.merged[service].get("ports"), f"private port published: {service}")
        for service in ("role", "auth"):
            self.assertIn("9091", [str(p) for p in self.merged[service].get("expose", [])])
        self.assertIn("8443", [str(p) for p in self.merged[PROXY].get("expose", [])])
        self.assertNotIn("PRIVATE KEY", json.dumps(self.merged))

    def test_proxy_exact_public_routes_tls_and_docker_dns(self):
        service = self.merged[PROXY]
        command = service.get("command") or ["nginx", "-g", "daemon off;"]
        command = shlex.split(command) if isinstance(command, str) else command
        self.assertEqual(Path(command[0]).name, "nginx", "proxy must actually execute nginx")
        self.assertFalse(service.get("entrypoint"), "custom entrypoint must be reviewed for config reachability")
        loaded_path = "/etc/nginx/nginx.conf"
        if "-c" in command:
            index = command.index("-c")
            self.assertLess(index + 1, len(command))
            loaded_path = command[index + 1]
        self.assertTrue(loaded_path.startswith("/"), "nginx config path must be absolute")
        mounted = [volume for volume in service.get("volumes", [])
                   if volume.get("target") == loaded_path and volume.get("type") == "bind"]
        self.assertEqual(len(mounted), 1, "loaded nginx config must have exactly one bind mount")
        expected = (ROOT / "docker/phase0/nginx.conf").resolve()
        self.assertEqual(Path(mounted[0]["source"]).resolve(), expected,
                         "Compose must load the exact nginx config checked below")
        self.assertTrue(mounted[0].get("read_only"), "nginx config must be read-only")
        self.assertRegex(service.get("image", ""), r"^nginx(?::|@)", "default command requires nginx image")
        config = expected.read_text()
        config = re.sub(r"(?m)#.*$", "", config)
        self.assertRegex(config, r"listen\s+8443\s+ssl\s*;")
        self.assertRegex(config, r"resolver\s+127\.0\.0\.11\b")
        for issuer, upstream in (("space", "/.well-known/jwks.json"),
                                 ("gateway", "/.well-known/voice-principal-jwks.json")):
            location = re.search(rf"location\s+=\s+/{issuer}/jwks\.json\s*\{{", config)
            self.assertIsNotNone(location)
            end, depth = location.end(), 1
            while end < len(config) and depth:
                depth += (config[end] == "{") - (config[end] == "}")
                end += 1
            self.assertEqual(depth, 0, "unclosed proxy location")
            body = config[location.end():end - 1]
            forward = re.search(r"proxy_pass\s+([^;]+);", body)
            self.assertIsNotNone(forward)
            target = forward.group(1).strip()
            # Variables keep nginx startup independent of issuer DNS/readiness.
            self.assertIn("$", target, "proxy must resolve issuer DNS lazily")
            for variable, value in re.findall(r"set\s+(\$\w+)\s+([^;]+);", config):
                target = target.replace(variable, value.strip().strip('\"'))
            if issuer == "space":
                self.assertEqual(target, f"https://space:8443{upstream}")
                for directive, value in (("proxy_ssl_server_name", "on"),
                                         ("proxy_ssl_name", "space"),
                                         ("proxy_ssl_verify", "on")):
                    self.assertEqual(re.findall(rf"\b{directive}\s+([^;\s]+)\s*;", body), [value])
                trusted_ca = re.search(r"proxy_ssl_trusted_certificate\s+([^;\s]+)\s*;", body)
                self.assertIsNotNone(trusted_ca)
                self.assertEqual(self.source_at(PROXY, trusted_ca.group(1)), "ca/ca.crt")
            else:
                self.assertEqual(target, f"http://{issuer}:8080{upstream}")
            method_guard = re.search(
                r"if\s*\(\s*\$request_method\s*!=\s*[\"']?GET[\"']?\s*\)"
                r"\s*\{\s*return\s+(?:403|405)\s*;\s*\}", body)
            self.assertIsNotNone(method_guard, "exact JWKS route must reject non-GET methods")

    def test_voice_space_media_overlay_is_limited_to_existing_services_and_tls_helper(self):
        added = set(self.media_merged) - set(self.base)
        self.assertEqual(added, {PROXY, "phase0-matchmaking-jwks-readiness"})
        def common_proxy(services):
            proxy = dict(services[PROXY])
            proxy.pop("depends_on", None)
            proxy["volumes"] = [v for v in proxy["volumes"]
                                if v["target"] != "/run/phase0/matchmaking/jwks.json"
                                and v["target"] != "/run/phase0/gateway/jwks.json"
                                and not v["target"].startswith("/run/phase0/space-ca")]
            return proxy
        self.assertEqual(common_proxy(self.media_merged), common_proxy(self.merged),
                         "proxy differs only in fixture JWKS inputs, Space CA and initialization dependency")
        expected_environment = {
            "space": {"SPACE_VOICE_S2S_TOKEN", "SPACE_VOICE_COMMUNICATION_GRPC_LISTEN",
                      "SPACE_VOICE_COMMUNICATION_REPLAY_REDIS_ADDR", "SPACE_VOICE_COMMUNICATION_TLS_CERT_FILE",
                      "SPACE_VOICE_COMMUNICATION_TLS_KEY_FILE", "SPACE_VOICE_COMMUNICATION_CLIENT_CA_FILE",
                      "SPACE_VOICE_COMMUNICATION_JWKS_CA_FILE", "SPACE_VOICE_COMMUNICATION_JWKS_CLIENT_CERT_FILE",
                      "SPACE_VOICE_COMMUNICATION_JWKS_CLIENT_KEY_FILE"},
            "auth": {
                "AUTH_PRINCIPAL_GRPC_PORT", "AUTH_SESSION_FLOOR_GRPC_PORT",
                "AUTH_GRPC_TLS_CERT_FILE", "AUTH_GRPC_TLS_KEY_FILE",
                "AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE",
                "S2S_JWKS_URLS_JSON", "JAVA_TOOL_OPTIONS",
            },
            "gateway": {
                "GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR", "GATEWAY_PRINCIPAL_ACTIVE_KID",
                "GATEWAY_VOICE_USER_GRPC_ADDR", "GATEWAY_VOICE_USER_TLS_CA_FILE",
                "GATEWAY_VOICE_USER_TLS_SERVER_NAME", "GATEWAY_VOICE_USER_CLIENT_CERT_FILE",
                "GATEWAY_VOICE_USER_CLIENT_KEY_FILE",
                "GATEWAY_MATCH_SQUAD_MEMBER_GRPC_ADDR", "GATEWAY_MATCH_SQUAD_MEMBER_TLS_CA_FILE",
                "GATEWAY_MATCH_SQUAD_MEMBER_TLS_SERVER_NAME",
                "GATEWAY_MATCH_SQUAD_MEMBER_CLIENT_CERT_FILE",
                "GATEWAY_MATCH_SQUAD_MEMBER_CLIENT_KEY_FILE",
            },
            "voice": {
                "SPACE_VOICE_S2S_TOKEN",
                "VOICE_MATCH_SQUAD_PRINCIPAL_GRPC_LISTEN",
                "VOICE_MATCH_SQUAD_PRINCIPAL_TLS_CERT_FILE",
                "VOICE_MATCH_SQUAD_PRINCIPAL_TLS_KEY_FILE",
                "VOICE_MATCH_SQUAD_PRINCIPAL_CLIENT_CA_FILE",
                "VOICE_MATCH_SQUAD_PRINCIPAL_JWKS_URL",
                "VOICE_MATCH_SQUAD_PRINCIPAL_JWKS_CA_FILE",
                "VOICE_MATCH_SQUAD_PRINCIPAL_REPLAY_REDIS_ADDR",
                "VOICE_MATCH_SQUAD_PRINCIPAL_REPLAY_REDIS_PASSWORD",
                "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_GRPC_LISTEN",
                "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_TLS_CERT_FILE",
                "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_TLS_KEY_FILE",
                "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_CLIENT_CA_FILE",
                "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_JWKS_URL",
                "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_JWKS_CA_FILE",
                "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_REDIS_ADDR",
                "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_REDIS_PASSWORD",
                "VOICE_ROLE_GRPC_ADDR", "VOICE_ROLE_TLS_CA_FILE",
                "VOICE_ROLE_CLIENT_CERT_FILE", "VOICE_ROLE_CLIENT_KEY_FILE",
                "VOICE_SPACE_COMMUNICATION_GRPC_ADDR", "VOICE_SPACE_COMMUNICATION_TLS_CA_FILE",
                "VOICE_SPACE_COMMUNICATION_CLIENT_CERT_FILE", "VOICE_SPACE_COMMUNICATION_CLIENT_KEY_FILE",
                "VOICE_SPACE_LIFECYCLE_PRINCIPAL_GRPC_LISTEN",
                "VOICE_SPACE_LIFECYCLE_PRINCIPAL_TLS_CERT_FILE",
                "VOICE_SPACE_LIFECYCLE_PRINCIPAL_TLS_KEY_FILE",
                "VOICE_SPACE_LIFECYCLE_PRINCIPAL_CLIENT_CA_FILE",
                "VOICE_SPACE_LIFECYCLE_PRINCIPAL_REPLAY_REDIS_ADDR",
                "VOICE_PRINCIPAL_PRIVATE_KEY_FILE", "VOICE_PRINCIPAL_KID",
                "VOICE_PRINCIPAL_NEXT_PRIVATE_KEY_FILE", "VOICE_PRINCIPAL_NEXT_KID",
                "VOICE_PRINCIPAL_JWKS_TLS_CERT_FILE", "VOICE_PRINCIPAL_JWKS_TLS_KEY_FILE",
                "VOICE_PRINCIPAL_JWKS_LISTEN_ADDR", "VOICE_USER_PRINCIPAL_REPLAY_REDIS_ADDR",
                "VOICE_USER_PRINCIPAL_TLS_CERT_FILE", "VOICE_USER_PRINCIPAL_TLS_KEY_FILE",
                "VOICE_USER_PRINCIPAL_CLIENT_CA_FILE", "VOICE_USER_PRINCIPAL_GRPC_LISTEN",
                "VOICE_AUTH_SESSION_FLOOR_GRPC_ADDR", "VOICE_AUTH_SESSION_FLOOR_TLS_CA_FILE",
                "VOICE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME", "VOICE_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE",
                "VOICE_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE",
                "S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE",
                "S2S_JWKS_CLIENT_CERT_FILE", "S2S_JWKS_CLIENT_KEY_FILE",
            },
            "role": {
                "ROLE_PRINCIPAL_GRPC_LISTEN", "ROLE_PRINCIPAL_TLS_CERT_FILE",
                "ROLE_PRINCIPAL_TLS_KEY_FILE", "ROLE_PRINCIPAL_CLIENT_CA_FILE",
                "ROLE_PRINCIPAL_REPLAY_REDIS_ADDR", "S2S_JWKS_URLS_JSON",
                "S2S_JWKS_CA_FILE",
            },
        }
        for service, expected in expected_environment.items():
            baseline = set(self.base[service].get("environment", {}))
            actual = set(self.media_merged[service].get("environment", {}))
            self.assertEqual(actual - baseline, expected,
                             f"unexpected {service} media-overlay environment additions")
        auth_env = self.media_merged["auth"]["environment"]
        self.assertEqual(auth_env["S2S_JWKS_URLS_JSON"],
                         '{"gateway":"https://phase0-jwks:8443/gateway/jwks.json","space":"https://phase0-jwks:8443/space/jwks.json","voice":"https://voice:8443/internal/v1/principal/jwks.json"}')
        self.assertEqual(auth_env["AUTH_SESSION_FLOOR_GRPC_PORT"], "9092")
        self.assertEqual(auth_env["AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE"], "/run/phase0/ca.crt")
        gateway_env = self.media_merged["gateway"]["environment"]
        self.assertEqual(gateway_env["GATEWAY_VOICE_USER_GRPC_ADDR"], "voice:9093")
        self.assertEqual(gateway_env["GATEWAY_VOICE_USER_TLS_SERVER_NAME"], "voice")
        voice_env = self.media_merged["voice"]["environment"]
        self.assertEqual(json.loads(voice_env["S2S_JWKS_URLS_JSON"]), {
            "space": "https://phase0-jwks:8443/space/jwks.json",
            "gateway": "https://phase0-jwks:8443/gateway/jwks.json",
        })
        self.assertEqual(voice_env["VOICE_AUTH_SESSION_FLOOR_GRPC_ADDR"], "auth:9092")
        self.assertEqual(voice_env["VOICE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME"], "auth")
        self.assertEqual(voice_env["VOICE_ROLE_GRPC_ADDR"], "role:9091")
        self.assertEqual(voice_env["VOICE_ROLE_TLS_CA_FILE"], "/run/phase0/ca.crt")
        self.assertEqual(voice_env["VOICE_ROLE_CLIENT_CERT_FILE"], "/run/phase0/voice-client.crt")
        self.assertEqual(voice_env["VOICE_ROLE_CLIENT_KEY_FILE"], "/run/phase0/voice-client.key")
        self.assertEqual(voice_env["VOICE_USER_PRINCIPAL_GRPC_LISTEN"], ":9093")
        self.assertEqual(voice_env["S2S_JWKS_CA_FILE"], "/run/phase0/ca.crt")
        self.assertEqual(voice_env["S2S_JWKS_CLIENT_CERT_FILE"], "/run/phase0/voice-client.crt")
        self.assertEqual(voice_env["S2S_JWKS_CLIENT_KEY_FILE"], "/run/phase0/voice-client.key")
        self.assertEqual(self.media_merged["gateway"]["depends_on"]["voice"]["condition"], "service_started",
                         "Gateway must serve its JWKS while Voice bootstraps the TLS JWKS proxy")
        self.assertIn("phase0-jwks", self.media_merged["voice"].get("depends_on", {}))
        for service in self.base:
            for name, value in self.base[service].get("environment", {}).items():
                if service == "space" and name == "S2S_JWKS_URLS_JSON":
                    self.assertEqual(json.loads(self.media_merged[service]["environment"][name])["social"],
                                     json.loads(value)["social"])
                    continue
                if service in {"auth", "gateway", "voice"} and name in {
                    "AUTH_PRINCIPAL_GRPC_PORT", "AUTH_GRPC_TLS_CERT_FILE", "AUTH_GRPC_TLS_KEY_FILE",
                    "GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR", "GATEWAY_PRINCIPAL_ACTIVE_KID",
                    "GATEWAY_VOICE_USER_GRPC_ADDR", "GATEWAY_VOICE_USER_TLS_CA_FILE",
                    "GATEWAY_VOICE_USER_TLS_SERVER_NAME", "GATEWAY_VOICE_USER_CLIENT_CERT_FILE",
                    "GATEWAY_VOICE_USER_CLIENT_KEY_FILE", "VOICE_USER_PRINCIPAL_GRPC_LISTEN",
                    "VOICE_USER_PRINCIPAL_TLS_CERT_FILE", "VOICE_USER_PRINCIPAL_TLS_KEY_FILE",
                    "VOICE_USER_PRINCIPAL_CLIENT_CA_FILE", "VOICE_AUTH_SESSION_FLOOR_GRPC_ADDR",
                    "VOICE_AUTH_SESSION_FLOOR_TLS_CA_FILE", "VOICE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME",
                    "VOICE_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE", "VOICE_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE",
                    "VOICE_PRINCIPAL_PRIVATE_KEY_FILE", "VOICE_PRINCIPAL_KID",
                    "VOICE_PRINCIPAL_NEXT_PRIVATE_KEY_FILE", "VOICE_PRINCIPAL_NEXT_KID",
                    "VOICE_PRINCIPAL_JWKS_TLS_CERT_FILE", "VOICE_PRINCIPAL_JWKS_TLS_KEY_FILE",
                    "VOICE_PRINCIPAL_JWKS_LISTEN_ADDR", "VOICE_USER_PRINCIPAL_REPLAY_REDIS_ADDR",
                    "S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE", "S2S_JWKS_CLIENT_CERT_FILE",
                    "S2S_JWKS_CLIENT_KEY_FILE", "JAVA_TOOL_OPTIONS", "LIVEKIT_URL",
                }:
                    continue
                self.assertEqual(self.media_merged[service].get("environment", {}).get(name), value,
                                 f"media overlay changed unrelated environment: {service}/{name}")
        role_env = self.media_merged["role"]["environment"]
        self.assertEqual(self.media_merged["role"].get("user"), "0:0",
                         "Role must read the private 0600 key in the disposable Phase0 fixture")
        self.assertEqual(role_env["ROLE_PRINCIPAL_GRPC_LISTEN"], ":9091")
        self.assertEqual(role_env["ROLE_PRINCIPAL_TLS_CERT_FILE"], "/run/phase0/role.crt")
        self.assertEqual(role_env["ROLE_PRINCIPAL_TLS_KEY_FILE"], "/run/phase0/role.key")
        self.assertEqual(role_env["ROLE_PRINCIPAL_CLIENT_CA_FILE"], "/run/phase0/ca.crt")
        self.assertEqual(role_env["ROLE_PRINCIPAL_REPLAY_REDIS_ADDR"], "redis:6379")
        self.assertEqual(json.loads(role_env["S2S_JWKS_URLS_JSON"]), {
            "space": "https://phase0-jwks:8443/space/jwks.json",
            "gameintegration": "https://gameintegration:8443/internal/v1/principal/jwks.json",
            "voice": "https://voice:8443/internal/v1/principal/jwks.json",
        })
        self.assertEqual(role_env["S2S_JWKS_CA_FILE"], "/run/phase0/ca.crt")
        self.assertEqual(self.media_merged["role"]["depends_on"]["redis"]["condition"], "service_healthy")
        self.assertEqual(self.media_merged["role"]["depends_on"]["phase0-jwks"]["condition"], "service_started")
        self.assertNotIn("voice", self.media_merged["role"]["depends_on"],
                         "Role JWKS refresh is lazy; a Voice dependency would cycle with Voice's Role health wait")
        role_mounts = self.media_merged["role"]["volumes"]
        for target, fixture_source in (("/run/phase0/role.crt", "tls/role.crt"),
                                       ("/run/phase0/role.key", "tls/role.key"),
                                       ("/run/phase0/ca.crt", "ca/ca.crt")):
            mount = next(volume for volume in role_mounts if volume["target"] == target)
            self.assertTrue(mount["read_only"], f"Role private mount must be read-only: {target}")
            self.assertEqual(self.source_at("role", target), fixture_source)
        for service in ("auth", "role", "voice", PROXY):
            self.assertFalse(self.media_merged[service].get("ports"), f"private port published: {service}")
        self.assertIn("9091", [str(port) for port in self.media_merged["auth"].get("expose", [])])
        self.assertIn("9091", [str(port) for port in self.media_merged["role"].get("expose", [])])
        self.assertIn("9093", [str(port) for port in self.media_merged["voice"].get("expose", [])])
        self.assertNotIn("PRIVATE KEY", json.dumps(self.media_merged))

    def test_media_canonical_admission_dependencies_are_wired(self):
        voice = self.media_merged["voice"]["environment"]
        space = self.media_merged["space"]["environment"]
        token = voice.get("SPACE_VOICE_S2S_TOKEN", "")
        self.assertTrue(token.strip(), "canonical Space resolver must have a fixture credential")
        self.assertEqual(token, space.get("SPACE_VOICE_S2S_TOKEN"))
        prefix = "VOICE_SPACE_LIFECYCLE_PRINCIPAL_"
        for field in ("TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR"):
            self.assertTrue(voice.get(prefix + field), f"missing lifecycle dependency: {field}")
        self.assertEqual(voice[prefix + "GRPC_LISTEN"], ":9092")
        self.assertNotEqual(voice[prefix + "GRPC_LISTEN"], voice["VOICE_USER_PRINCIPAL_GRPC_LISTEN"])
        self.assertEqual(voice["LIVEKIT_URL"], "ws://livekit:7880")
        self.assertEqual(self.media_merged["voice"]["depends_on"]["space"]["condition"], "service_healthy")
        self.assertEqual(self.media_merged[PROXY]["depends_on"]["social-principal-init"]["condition"],
                         "service_completed_successfully")

    def test_proxy_trusts_the_ca_of_each_overlay_space_certificate(self):
        ca_directory = "/run/phase0/space-ca"
        ca_file = ca_directory + "/ca.crt"
        config = (ROOT / "docker/phase0/nginx.conf").read_text(encoding="utf-8")
        self.assertIn(f"proxy_ssl_trusted_certificate {ca_file};", config)
        self.assertIn("proxy_ssl_verify on;", config)
        self.assertIn("proxy_ssl_name space;", config)
        media_ca = next(v for v in self.media_merged[PROXY]["volumes"] if v["target"] == ca_directory)
        space_ca = next(v for v in self.base["space"]["volumes"]
                        if v["target"] == "/etc/voice/principal/ca")
        self.assertEqual(media_ca["type"], "volume")
        self.assertEqual(media_ca["source"], space_ca["source"])
        self.assertTrue(media_ca["read_only"])
        for name in ("SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE", "SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE"):
            self.assertEqual(self.media_merged["space"]["environment"][name], self.base["space"]["environment"][name])
        self.assertEqual(self.source_at(PROXY, ca_file), "ca/ca.crt")
        command = self.command(True)
        command[-3:-3] = ["-f", MEDIA_OVERLAY]
        combined = json.loads(require_success(run(*command, env=self.env)))["services"]
        # The full Phase0 overlay replaces Space's leaf. Its more specific file
        # mount retains the fixture CA over the media-only directory mount.
        combined_ca = next(v for v in combined[PROXY]["volumes"] if v["target"] == ca_file)
        self.assertEqual(Path(combined_ca["source"]), self.fixture / "ca/ca.crt")
        self.assertTrue(combined_ca["read_only"])
        self.assertEqual(combined["space"]["environment"]["SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE"],
                         self.merged["space"]["environment"]["SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE"])

    def test_combined_phase0_media_listeners_have_distinct_ports_and_matching_caller(self):
        # Match the CI stack's effective order: the complete Phase0 overlay
        # must override the media-only Gateway JWKS bootstrap URL.
        combined = self.ci_merged
        voice = combined["voice"]["environment"]
        self.assertEqual(json.loads(voice["S2S_JWKS_URLS_JSON"]), {
            "gateway": "https://phase0-jwks:8443/gateway-bootstrap/jwks.json",
            "space": "https://phase0-jwks:8443/space/jwks.json",
        })
        matchmaking = combined["matchmaking"]["environment"]
        self.assertEqual(json.loads(matchmaking["S2S_JWKS_URLS_JSON"]), {
            "gateway": "https://phase0-jwks:8443/gateway-bootstrap/jwks.json",
            "space": "https://space:8443/.well-known/jwks.json",
        })
        auth = combined["auth"]["environment"]
        self.assertEqual(json.loads(auth["S2S_JWKS_URLS_JSON"]), {
            "gateway": "https://phase0-jwks:8443/gateway/jwks.json",
            "space": "https://phase0-jwks:8443/space/jwks.json",
            "voice": "https://voice:8443/internal/v1/principal/jwks.json",
        })
        self.assertEqual(auth["AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE"], "/run/phase0/ca.crt")
        listeners = [voice[name] for name in (
            "VOICE_GAME_PRINCIPAL_GRPC_LISTEN", "VOICE_SPACE_LIFECYCLE_PRINCIPAL_GRPC_LISTEN",
            "VOICE_USER_PRINCIPAL_GRPC_LISTEN")]
        self.assertEqual(len(set(listeners)), len(listeners))
        self.assertEqual(combined["gateway"]["environment"]["GATEWAY_VOICE_USER_GRPC_ADDR"],
                         "voice" + voice["VOICE_USER_PRINCIPAL_GRPC_LISTEN"])
        self.assertFalse(combined["voice"].get("ports"))

    def test_media_overlay_mounts_only_ephemeral_chain_credentials_readonly(self):
        allowed = {
            "space": {"tls/space-lifecycle.crt", "tls/space-lifecycle.key", "tls/space-client.crt",
                      "tls/space-client.key", "ca/ca.crt"},
            "auth": {"tls/auth.crt", "tls/auth.key", "truststore.p12", "ca/ca.crt"},
            "gateway": {"gateway/current.pem", "gateway/next.pem", "tls/gateway-client.crt", "tls/gateway-client.key", "ca/ca.crt"},
            "voice": {"voice/current.pem", "voice/next.pem", "tls/voice-jwks.crt", "tls/voice-jwks.key",
                      "tls/voice-game-grpc.crt", "tls/voice-game-grpc.key", "tls/voice-client.crt",
                      "tls/voice-client.key", "tls/voice-match-squad-grpc.crt",
                      "tls/voice-match-squad-grpc.key", "tls/voice-match-squad-member-grpc.crt",
                      "tls/voice-match-squad-member-grpc.key", "ca/ca.crt",
                      "ca/space-lifecycle-client-ca.crt"},
            PROXY: {"tls/proxy.crt", "tls/proxy.key", "ca/ca.crt"},
            "phase0-jwks": {"jwks/matchmaking/jwks.json", "tls/proxy.crt",
                            "tls/proxy.key", "ca/ca.crt"},
        }
        for service, sources in allowed.items():
            for volume in self.media_merged[service].get("volumes", []):
                if volume.get("type") != "bind":
                    continue
                source = Path(volume["source"])
                if source.is_relative_to(self.fixture):
                    relative = source.relative_to(self.fixture).as_posix()
                    if service == "gateway" and relative == "gateway":
                        self.assertTrue(source.is_dir() and not source.is_symlink())
                        entries = list(source.iterdir())
                        self.assertEqual({entry.name for entry in entries}, {"current.pem", "next.pem"})
                        self.assertTrue(all(entry.is_file() and not entry.is_symlink() for entry in entries))
                    else:
                        self.assertIn(relative, sources,
                                      f"unexpected fixture material mounted into {service}")
                    self.assertTrue(volume.get("read_only"), f"writable fixture mounted into {service}")
        config = re.sub(r"(?m)#.*$", "", (ROOT / "docker/phase0/nginx.conf").read_text(encoding="utf-8"))
        for directive, source in (("ssl_certificate", "tls/proxy.crt"),
                                  ("ssl_certificate_key", "tls/proxy.key")):
            match = re.search(rf"\b{directive}\s+([^;\s]+)\s*;", config)
            self.assertIsNotNone(match)
            self.assertEqual(self.source_at(PROXY, match.group(1)), source)


class RestorerPathTests(unittest.TestCase):
    def test_owner_restorer_accepts_only_declared_generated_fixture_locations(self):
        with tempfile.TemporaryDirectory(prefix="phase0-restorer-paths-") as tmp:
            base = Path(tmp)
            runner_temp = base / "runner-temp"
            runner_temp.mkdir()
            accepted = (
                base / "voice-a1-flutter-profile-handoff.abcdefgh" / "phase0-fixture",
                base / "voice-a1-multi-account-proof.abcdefgh" / "phase0-fixture",
                base / "voice-file-restart-proof.abcdefgh" / "phase0-fixture",
                base / "phase0-t31-ci-fixture" / "credentials",
                runner_temp / "voice-space-media-phase0",
            )
            for path in accepted:
                path.mkdir(parents=True)
            self.assertTrue(all(fixture_restorer.supported_root(path, runner_temp, path) for path in accepted))
            self.assertFalse(fixture_restorer.supported_root(accepted[0], runner_temp, accepted[1]))
            self.assertFalse(fixture_restorer.supported_root(base / "unrelated" / "credentials",
                                                             runner_temp, None))
            self.assertFalse(fixture_restorer.supported_root(base / "unrelated" / "phase0-fixture",
                                                             runner_temp, None))
            wrong_temp = base / "wrong-temp" / "voice-space-media-phase0"
            self.assertFalse(fixture_restorer.supported_root(wrong_temp, runner_temp, wrong_temp))


if __name__ == "__main__":
    unittest.main(verbosity=2)
