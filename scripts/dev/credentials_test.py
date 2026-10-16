"""Run: python -m unittest discover -s scripts/dev -p credentials_test.py."""
import base64
import importlib.util
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("credentials", Path(__file__).with_name("credentials.py"))
credentials = importlib.util.module_from_spec(spec)
spec.loader.exec_module(credentials)


class CredentialsTests(unittest.TestCase):
    def test_target_and_custody_reject_unsafe_inputs_before_output(self):
        with tempfile.TemporaryDirectory() as parent:
            target = Path(parent) / "issued"
            with patch.object(credentials.sys, "platform", "win32"):
                with self.assertRaises(ValueError):
                    credentials.validate_destination(target)
            self.assertFalse(target.exists())

    @unittest.skipUnless(shutil.which("openssl") and shutil.which("keytool"), "crypto tools required")
    def test_independent_tls_and_signers_keep_ca_keys_out_of_secrets(self):
        with tempfile.TemporaryDirectory() as parent:
            root = Path(parent)
            material = credentials.Material(root)
            ca = material.ca("media")
            server = material.tls("voice-voice-media-server-tls", ca, "voice-voice", "serverAuth")
            client = material.tls("voice-voice-media-client-tls", ca, "voice", "clientAuth")
            self.assertNotEqual(server["tls.key"], client["tls.key"])
            material.signer("voice-gateway-principal-signing")
            material.signer("voice-voice-principal-signing", active=False)
            items = material.items
            self.assertTrue(all(x["metadata"]["namespace"] == "voice-dev" for x in items))
            payload = json.dumps(items)
            self.assertNotIn(base64.b64encode(ca[0].read_bytes()).decode(), payload)
            signer = next(x for x in items if x["metadata"]["name"] == "voice-gateway-principal-signing")
            self.assertNotEqual(signer["data"]["current.pem"], signer["data"]["next.pem"])
            self.assertEqual(base64.b64decode(signer["data"]["active-kid"]), b"current")
            credentials.run("openssl", "verify", "-CAfile", ca[1], "-purpose", "sslserver", root / "voice-voice-media-server-tls.crt")
            with self.assertRaises(ValueError):
                credentials.run("openssl", "verify", "-CAfile", ca[1], "-purpose", "sslserver", root / "voice-voice-media-client-tls.crt")

    def test_application_material_is_fresh_and_dev_scoped(self):
        first = credentials.application_values(b"private-key", "api.dev.example.test")
        second = credentials.application_values(b"other-private-key", "api.dev.example.test")
        self.assertNotEqual(first["POSTGRES_PASSWORD"], second["POSTGRES_PASSWORD"])
        self.assertEqual(first["AUTH_JWT_PRIVATE_KEY"], b"private-key")
        self.assertTrue(first["USER_R2_BUCKET"].startswith("voice-dev-"))
        self.assertEqual(first["AUTH_RESEND_API_KEY"], "")
        self.assertNotIn("voice-staging", json.dumps({k:str(v) for k,v in first.items()}))

    @unittest.skipUnless(shutil.which("openssl") and shutil.which("keytool"), "crypto tools required")
    def test_import_retains_existing_public_java_roots(self):
        with tempfile.TemporaryDirectory() as parent:
            root = Path(parent)
            material = credentials.Material(root)
            public = material.ca("public-test")
            media = material.ca("media-test")
            source = root / "cacerts"
            credentials.run("keytool", "-importcert", "-noprompt", "-alias", "existing-public",
                            "-file", public[1], "-keystore", source, "-storepass", "changeit")
            output = credentials.make_truststore(root, source, media[1])
            self.assertTrue(output)
            before = credentials.fingerprints(credentials.run("keytool", "-list", "-v", "-keystore", source,
                                                            "-storepass", "changeit", "-J-Duser.language=en"))
            after = credentials.fingerprints(credentials.run("keytool", "-list", "-v", "-keystore", root / "truststore.p12",
                                                           "-storepass", "changeit", "-J-Duser.language=en"))
            self.assertEqual(len(before), 1)
            self.assertEqual(len(after), 2)
            self.assertTrue(before < after)

    def test_secret_contract_refuses_cross_environment_output(self):
        with self.assertRaises(ValueError):
            credentials.validate_bundle({"kind":"List", "items":[{"metadata":{"name":"voice-app-secrets", "namespace":"voice-staging"}, "data":{}}]})


if __name__ == "__main__":
    unittest.main()
