#!/usr/bin/env python3
"""Offline Phase0 fixture contracts. Never starts containers or contacts Docker."""

import hashlib
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


ROOT = Path(__file__).resolve().parents[2]
GENERATOR = ROOT / "scripts/dev/generate-phase0-fixture.py"
OVERLAY = ROOT / "docker-compose.phase0.yml"
PROXY = "phase0-jwks"
URLS = {name: f"https://{PROXY}:8443/{name}/jwks.json"
        for name in ("gateway", "space")}


def run(*args, env=None, input=None):
    return subprocess.run([str(arg) for arg in args], cwd=ROOT, env=env,
                          input=input, capture_output=True, text=True, timeout=90)


def require_success(result):
    # Do not echo child output: a faulty generator could leak private material.
    if result.returncode:
        raise AssertionError(f"command failed with exit code {result.returncode}")
    return result.stdout


def require_binary(name):
    if not shutil.which(name):
        raise AssertionError(f"required offline test prerequisite missing: {name}")


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
        cls.generated = run(sys.executable, GENERATOR, cls.dest)
        require_success(cls.generated)

    def openssl(self, *args, input=None):
        return require_success(run("openssl", *args, input=input))

    def test_separate_current_next_rsa_pkcs8_keys(self):
        public_keys = set()
        for issuer in ("gateway", "space"):
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
        self.assertEqual(len(public_keys), 4, "issuers and rotation keys must be independent")

    def test_ca_signed_leaf_keys_hostname_and_short_lifetime(self):
        ca = self.dest / "ca/ca.crt"
        self.assertEqual({p.name for p in ca.parent.iterdir()}, {"ca.crt"},
                         "CA signing key must not survive issuance")
        for leaf, hostname in (("role", "role"), ("auth", "auth"), ("proxy", PROXY)):
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
        for path in (self.dest, *self.dest.rglob("*")):
            if path.is_dir():
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o700)
            elif path.suffix in (".pem", ".key", ".p12"):
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)


class ComposeTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not OVERLAY.is_file():
            raise AssertionError("Phase0 Compose overlay does not exist")
        require_binary("docker")
        cls.temp = tempfile.TemporaryDirectory(prefix="voice-phase0-compose-")
        cls.addClassCleanup(cls.temp.cleanup)
        cls.fixture = Path(cls.temp.name) / "fixture"
        cls.empty_env = Path(cls.temp.name) / "empty.env"
        cls.empty_env.write_text("")
        # Whitelist execution prerequisites, never inherit developer Compose secrets.
        cls.env = {k: v for k, v in os.environ.items()
                   if k.upper() in {"PATH", "SYSTEMROOT", "WINDIR", "HOME", "USERPROFILE",
                                    "TEMP", "TMP", "DOCKER_CONFIG"}}
        cls.env["PHASE0_FIXTURE_DIR"] = str(cls.fixture.resolve())
        cls.base = cls.render(False)
        cls.merged = cls.render(True)

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

    def test_fixture_directory_is_required(self):
        env = dict(self.env)
        env.pop("PHASE0_FIXTURE_DIR")
        self.assertNotEqual(run(*self.command(True), env=env).returncode, 0)

    def test_frozen_environment_and_mounted_paths(self):
        for issuer in ("gateway", "space"):
            env = self.merged[issuer]["environment"]
            prefix = issuer.upper() + "_PRINCIPAL_"
            self.assertEqual(env[prefix + "ACTIVE_KID"], "current")
            self.assertEqual(self.source_at(issuer, env[prefix + "SIGNING_KEYS_DIR"]), issuer)
        space = self.merged["space"]["environment"]
        self.assertEqual(space["ROLE_PRINCIPAL_GRPC_ADDR"], "role:9091")
        self.assertEqual(self.source_at("space", space["ROLE_PRINCIPAL_TLS_CA_FILE"]), "ca/ca.crt")
        role = self.merged["role"]["environment"]
        self.assertEqual(role["ROLE_PRINCIPAL_GRPC_LISTEN"], ":9091")
        self.assertEqual(role["ROLE_PRINCIPAL_REPLAY_REDIS_ADDR"], "redis:6379")
        self.assertEqual(json.loads(role["S2S_JWKS_URLS_JSON"]), {"space": URLS["space"]})
        self.assertEqual(self.source_at("role", role["S2S_JWKS_CA_FILE"]), "ca/ca.crt")
        auth = self.merged["auth"]["environment"]
        self.assertEqual(auth["AUTH_PRINCIPAL_GRPC_PORT"], "9091")
        self.assertEqual(json.loads(auth["S2S_JWKS_URLS_JSON"]), URLS)
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
        for service, base in self.base.items():
            for name, value in base.get("environment", {}).items():
                self.assertEqual(self.merged[service]["environment"].get(name), value,
                                 f"base environment changed: {service}/{name}")
            self.assertEqual(self.merged[service].get("ports", []), base.get("ports", []))
        self.assertEqual(self.merged["role"]["environment"]["ROLE_GRPC_LISTEN"], ":9090")
        self.assertEqual(self.merged["space"]["environment"]["ROLE_GRPC_ADDR"], "role:9090")

    def test_private_mounts_are_readonly_and_service_isolated(self):
        allowed = {"gateway": {"gateway"}, "space": {"space", "ca/ca.crt"},
                   "role": {"tls/role.crt", "tls/role.key", "ca/ca.crt"},
                   "auth": {"tls/auth.crt", "tls/auth.key", "truststore.p12", "ca/ca.crt"},
                   PROXY: {"tls/proxy.crt", "tls/proxy.key", "ca/ca.crt"}}
        for service in self.merged:
            for source, mount in self.fixture_mounts(service):
                self.assertIn(source, allowed.get(service, set()), f"fixture exposed to {service}")
                self.assertTrue(mount.get("read_only"), f"writable fixture: {service}/{source}")
            if service in allowed:
                self.assertIn(str(self.merged[service].get("user")), ("0", "0:0", "root"))
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
            self.assertEqual(target, f"http://{issuer}:8080{upstream}")
            method_guard = re.search(
                r"if\s*\(\s*\$request_method\s*!=\s*[\"']?GET[\"']?\s*\)"
                r"\s*\{\s*return\s+(?:403|405)\s*;\s*\}", body)
            self.assertIsNotNone(method_guard, "exact JWKS route must reject non-GET methods")
        for directive, source in (("ssl_certificate", "tls/proxy.crt"),
                                  ("ssl_certificate_key", "tls/proxy.key")):
            match = re.search(rf"\b{directive}\s+([^;\s]+)\s*;", config)
            self.assertIsNotNone(match)
            self.assertEqual(self.source_at(PROXY, match.group(1)), source)


if __name__ == "__main__":
    unittest.main(verbosity=2)
