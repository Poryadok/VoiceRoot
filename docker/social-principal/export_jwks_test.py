"""Run only in an isolated Linux container: LOCAL_JWKS_BOOTSTRAP_TEST=1."""
import base64
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


@unittest.skipUnless(
    sys.platform == "linux" and os.environ.get("LOCAL_JWKS_BOOTSTRAP_TEST") == "1",
    "requires the isolated Linux bootstrap runner",
)
class ExportJWKSKeyValidationTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fixture = tempfile.TemporaryDirectory(prefix="jwks-key-validation-")
        cls.keys = Path(cls.fixture.name)
        for service in ("space", "realtime"):
            for kid in ("current", "next"):
                cls.generate(cls.keys / f"{service}-{kid}.pem", 2048)
        for kid in ("current", "next"):
            cls.generate(cls.keys / f"weak-{kid}.pem", 1024)
        cls.generate(cls.keys / "exponent-three.pem", 2048, 3)

    @classmethod
    def tearDownClass(cls):
        cls.fixture.cleanup()

    @staticmethod
    def generate(path, bits, exponent=65537):
        subprocess.run(
            ["openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt",
             f"rsa_keygen_bits:{bits}", "-pkeyopt",
             f"rsa_keygen_pubexp:{exponent}", "-out", str(path)],
            check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )

    def setUp(self):
        self.reset()

    def reset(self):
        for directory in ("/space-signing", "/realtime-signing", "/jwks-public"):
            path = Path(directory)
            if path.exists():
                shutil.rmtree(path)
            path.mkdir()
        for service in ("space", "realtime"):
            for kid in ("current", "next"):
                shutil.copyfile(self.keys / f"{service}-{kid}.pem",
                                Path(f"/{service}-signing/{kid}.pem"))

    def export(self, check=False):
        return subprocess.run(
            [sys.executable, str(Path(__file__).with_name("export-jwks.py"))]
            + (["--check"] if check else []),
            capture_output=True, text=True,
        )

    def weak(self, service):
        for kid in ("current", "next"):
            shutil.copyfile(self.keys / f"weak-{kid}.pem",
                            Path(f"/{service}-signing/{kid}.pem"))

    def matching_public_documents(self):
        # These fixture keys explicitly use 65537. Construct the public docs even
        # when the publisher correctly refuses their weak/broken private pair.
        for service in ("space", "realtime"):
            keys = []
            for kid in ("current", "next"):
                raw = subprocess.check_output(
                    ["openssl", "rsa", "-in", f"/{service}-signing/{kid}.pem",
                     "-noout", "-modulus"], text=True,
                ).strip().removeprefix("Modulus=")
                n = base64.urlsafe_b64encode(bytes.fromhex(raw)).decode().rstrip("=")
                keys.append(dict(kty="RSA", kid=kid, use="sig", alg="RS256", n=n, e="AQAB"))
            Path(f"/jwks-public/{service}.json").write_text(
                json.dumps({"keys": keys}, separators=(",", ":")) + "\n",
                encoding="utf-8",
            )

    @staticmethod
    def break_private_exponent(service):
        path = Path(f"/{service}-signing/current.pem")
        raw = bytearray(base64.b64decode("".join(path.read_text().splitlines()[1:-1])))

        def value(offset):
            length = raw[offset + 1]
            start = offset + 2
            if length & 128:
                width = length & 127
                length = int.from_bytes(raw[start:start + width], "big")
                start += width
            return start, start + length

        outer, _ = value(0)
        _, offset = value(outer)  # PKCS#8 version
        _, offset = value(offset)  # algorithm identifier
        inner, _ = value(offset)  # wrapped PKCS#1
        offset, _ = value(inner)
        for _ in range(4):  # version, n, e, d
            _, end = value(offset)
            offset = end
        raw[end - 1] ^= 1  # Public n/e remain unchanged; private d becomes invalid.
        encoded = base64.b64encode(raw).decode()
        path.write_text("-----BEGIN PRIVATE KEY-----\n" +
                        "\n".join(encoded[i:i + 64] for i in range(0, len(encoded), 64)) +
                        "\n-----END PRIVATE KEY-----\n", encoding="utf-8")

    def test_valid_distinct_pairs_publish_and_reuse(self):
        self.assertEqual(self.export().returncode, 0)
        self.assertEqual(self.export(check=True).returncode, 0)
        for service in ("space", "realtime"):
            keys = json.loads(Path(f"/jwks-public/{service}.json").read_text())["keys"]
            self.assertEqual([key["kid"] for key in keys], ["current", "next"])
            self.assertNotEqual(keys[0]["n"], keys[1]["n"])
            for key in keys:
                self.assertEqual(set(key), {"kty", "kid", "use", "alg", "n", "e"})
                n = int.from_bytes(base64.urlsafe_b64decode(key["n"] + "=="), "big")
                self.assertGreaterEqual(n.bit_length(), 2048)

    def test_both_issuers_reject_weak_keys_before_any_publication(self):
        for service in ("space", "realtime"):
            with self.subTest(service=service):
                self.reset()
                self.weak(service)
                self.assertNotEqual(self.export().returncode, 0)
                self.assertEqual(list(Path("/jwks-public").iterdir()), [])

    def test_both_issuers_reject_matching_weak_reuse(self):
        for service in ("space", "realtime"):
            with self.subTest(service=service):
                self.reset()
                self.weak(service)
                self.matching_public_documents()
                self.assertNotEqual(self.export(check=True).returncode, 0)

    def test_both_issuers_reject_unusable_private_pair_before_publication(self):
        for service in ("space", "realtime"):
            with self.subTest(service=service):
                self.reset()
                self.break_private_exponent(service)
                self.assertNotEqual(self.export().returncode, 0)
                self.assertEqual(list(Path("/jwks-public").iterdir()), [])

    def test_both_issuers_reject_matching_unusable_private_pair_reuse(self):
        for service in ("space", "realtime"):
            with self.subTest(service=service):
                self.reset()
                self.break_private_exponent(service)
                self.matching_public_documents()
                self.assertNotEqual(self.export(check=True).returncode, 0)

    def test_both_issuers_preserve_usable_nondefault_exponent(self):
        for service in ("space", "realtime"):
            with self.subTest(service=service):
                self.reset()
                shutil.copyfile(self.keys / "exponent-three.pem",
                                Path(f"/{service}-signing/current.pem"))
                result = self.export()
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(self.export(check=True).returncode, 0)
                keys = json.loads(Path(f"/jwks-public/{service}.json").read_text())["keys"]
                self.assertEqual(keys[0]["e"], "Aw")


if __name__ == "__main__":
    unittest.main(verbosity=2)
