import base64
import importlib.util
import json
import pathlib
import sys
import unittest

sys.dont_write_bytecode = True

SCRIPT = pathlib.Path(__file__).with_name("check-resend-key.py")
spec = importlib.util.spec_from_file_location("check_resend_key", SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def encoded(value):
    return base64.b64encode(value.encode()).decode()


def secret(data=None, string_data=None, version="12"):
    document = {
        "apiVersion": "v1",
        "kind": "Secret",
        "metadata": {"name": "voice-app-secrets", "namespace": "voice-staging"},
        "type": "Opaque",
    }
    if version is not None:
        document["metadata"]["resourceVersion"] = version
    if data is not None:
        document["data"] = data
    if string_data is not None:
        document["stringData"] = string_data
    return document


class MergeUploadTest(unittest.TestCase):
    def setUp(self):
        self.live_data = {key: encoded("live-" + key) for key in module.REQUIRED_KEYS}
        self.live_data["EXTRA_KEY"] = encoded("keep")

    def test_partial_upload_uses_existing_fields_and_patches_only_supplied_keys(self):
        live = secret(data=self.live_data)
        upload = secret(string_data={"AUTH_RESEND_API_KEY": "new-mail-key"}, version=None)
        missing, patch = module.plan_upload(upload, live, "voice-staging")
        self.assertEqual(missing, [])
        self.assertEqual(
            patch,
            [
                {"op": "test", "path": "/metadata/resourceVersion", "value": "12"},
                {"op": "add", "path": "/data/AUTH_RESEND_API_KEY", "value": encoded("new-mail-key")},
            ],
        )
        self.assertNotIn("live-", json.dumps(patch))

    def test_blank_upload_cannot_fall_back_to_live_value(self):
        upload = secret(string_data={"AUTH_RESEND_API_KEY": ""}, version=None)
        missing, patch = module.plan_upload(upload, secret(data=self.live_data), "voice-staging")
        self.assertIn("AUTH_RESEND_API_KEY", missing)
        self.assertIsNone(patch)

    def test_missing_live_requires_complete_upload(self):
        missing, patch = module.plan_upload(
            secret(string_data={"AUTH_RESEND_API_KEY": "new-mail-key"}, version=None),
            None,
            "voice-staging",
        )
        self.assertIn("POSTGRES_PASSWORD", missing)
        self.assertIsNone(patch)

    def test_wrong_identity_and_invalid_base64_fail_closed(self):
        upload = secret(data={"AUTH_RESEND_API_KEY": "not base64!"}, version=None)
        self.assertIsNone(module.plan_upload(upload, secret(data=self.live_data), "voice-staging"))
        upload = secret(string_data={"AUTH_RESEND_API_KEY": "new"}, version=None)
        upload["metadata"]["namespace"] = "other"
        self.assertIsNone(module.plan_upload(upload, secret(data=self.live_data), "voice-staging"))

    def test_full_upload_can_create_absent_secret(self):
        upload = secret(string_data={key: "new-" + key for key in module.REQUIRED_KEYS}, version=None)
        missing, patch = module.plan_upload(upload, None, "voice-staging")
        self.assertEqual(missing, [])
        self.assertIsNone(patch)

    def test_empty_upload_is_rejected(self):
        self.assertIsNone(module.plan_upload(secret(string_data={}, version=None), secret(data=self.live_data), "voice-staging"))

    def test_offline_upload_format_accepts_partial_but_rejects_invalid_encoding(self):
        upload = secret(string_data={"AUTH_RESEND_API_KEY": "new"}, version=None)
        self.assertEqual(module.uploaded_data(upload, "voice-staging"), {"AUTH_RESEND_API_KEY": encoded("new")})
        invalid = secret(data={"AUTH_RESEND_API_KEY": "!"}, version=None)
        self.assertIsNone(module.uploaded_data(invalid, "voice-staging"))


if __name__ == "__main__":
    unittest.main()
