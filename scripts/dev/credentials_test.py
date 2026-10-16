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
        self.assertNotEqual(first["GAME_INTEGRATION_DB_PASSWORD"], second["GAME_INTEGRATION_DB_PASSWORD"])
        self.assertIn("postgres://gameintegration_runtime:", first["GAME_INTEGRATION_DATABASE_URL"])
        self.assertIn("/game_integration_db?sslmode=disable", first["GAME_INTEGRATION_DATABASE_URL"])
        self.assertNotIn(first["POSTGRES_PASSWORD"], first["GAME_INTEGRATION_DATABASE_URL"])
        self.assertNotIn("voice-staging", json.dumps({k:str(v) for k,v in first.items()}))

    @unittest.skipUnless(shutil.which("openssl"), "crypto tools required")
    def test_gameintegration_issuer_material_is_distinct_and_uses_media_ca(self):
        with tempfile.TemporaryDirectory() as parent:
            root = Path(parent)
            material = credentials.Material(root)
            principal = material.ca("principal")
            media = material.ca("media")
            credentials.add_gameintegration_issuer_material(material, media)
            items = {item["metadata"]["name"]: item for item in material.items}
            signer = items["voice-gameintegration-principal-signing"]
            tls = items["voice-gameintegration-jwks-tls"]
            workload = items["voice-gameintegration-workload-credentials"]
            self.assertEqual(set(signer["data"]), {"current.pem", "next.pem"})
            self.assertNotEqual(signer["data"]["current.pem"], signer["data"]["next.pem"])
            self.assertEqual(set(tls["data"]), {"tls.crt", "tls.key"})
            # Kubernetes Secret.data encodes the environment value; that value is
            # itself base64 because the service config consumes *_KEY_B64.
            notification_key_b64 = base64.b64decode(
                workload["data"]["GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64"], validate=True
            )
            realtime_key_b64 = base64.b64decode(
                workload["data"]["GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64"], validate=True
            )
            notification_key = base64.b64decode(notification_key_b64, validate=True)
            realtime_key = base64.b64decode(realtime_key_b64, validate=True)
            self.assertEqual(len(notification_key), 32)
            self.assertEqual(len(realtime_key), 32)
            self.assertNotEqual(notification_key, realtime_key)
            self.assertNotIn(base64.b64encode(principal[0].read_bytes()).decode(), json.dumps(items))
            self.assertNotIn(base64.b64encode(media[0].read_bytes()).decode(), json.dumps(items))
            credentials.run("openssl", "verify", "-CAfile", media[1], "-purpose", "sslserver",
                            root / "voice-gameintegration-jwks-tls.crt")

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
