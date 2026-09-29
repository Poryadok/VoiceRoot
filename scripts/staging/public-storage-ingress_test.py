"""Guard the public S3 ingress and application endpoint contract."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


class PublicStorageIngressTest(unittest.TestCase):
    def test_gateway_ingress_routes_signed_bucket_prefixes_to_minio(self):
        manifest = (ROOT / "deploy/gateway/ingress.yaml").read_text(encoding="utf-8")
        for bucket in ("__FILE_BUCKET__", "__AVATAR_BUCKET__"):
            self.assertEqual(2, manifest.count(f"path: /{bucket}"))
        self.assertEqual(4, manifest.count("name: voice-minio"))
        self.assertEqual(4, manifest.count("number: 9000"))

    def test_staging_services_sign_for_https_gateway_host(self):
        manifest = (ROOT / "deploy/staging/services.yaml").read_text(encoding="utf-8")
        for prefix in ("FILE", "USER"):
            self.assertIn(f"name: {prefix}_R2_SIGNING_ENDPOINT", manifest)
        render = (ROOT / "scripts/staging/apply-app-manifests.sh").read_text(encoding="utf-8")
        self.assertIn("__S3_SIGNING_ENDPOINT__", render)
        self.assertIn("VOICE_GATEWAY_INGRESS_HOST", render)

    def test_ingress_routes_exact_buckets_from_app_secret(self):
        for path in ("scripts/staging/apply-gateway-ingress.sh", "scripts/prod/apply-gateway-ingress.sh"):
            apply = (ROOT / path).read_text(encoding="utf-8")
            self.assertIn("voice-app-secrets", apply)
            self.assertIn("FILE_R2_BUCKET", apply)
            self.assertIn("USER_R2_BUCKET", apply)
            self.assertNotIn("VOICE_FILE_R2_BUCKET", apply)

    def test_minio_cors_restricts_browser_origin(self):
        manifest = (ROOT / "deploy/staging/minio.yaml").read_text(encoding="utf-8")
        self.assertIn("MINIO_API_CORS_ALLOW_ORIGIN", manifest)
        self.assertIn("__WEB_ORIGIN__", manifest)
        render = (ROOT / "scripts/staging/apply-infra.sh").read_text(encoding="utf-8")
        self.assertIn("__WEB_ORIGIN__", render)
        self.assertIn("VOICE_WEB_INGRESS_HOST", render)

    def test_staging_smoke_probes_public_browser_preflight_without_uploading(self):
        smoke = (ROOT / "scripts/staging/smoke-staging.sh").read_text(encoding="utf-8")
        self.assertIn("Access-Control-Request-Method: PUT", smoke)
        self.assertIn("Access-Control-Allow-Origin", smoke)
        self.assertIn("-X OPTIONS", smoke)

    def test_selective_rollouts_apply_minio_cors_without_replacing_storage(self):
        for path in ("scripts/staging/render-and-apply.sh", "scripts/prod/render-and-apply-prod.sh"):
            rollout = (ROOT / path).read_text(encoding="utf-8")
            self.assertGreaterEqual(rollout.count("apply-minio-cors.sh"), 2, path)
        cors = (ROOT / "scripts/storage/apply-minio-cors.sh").read_text(encoding="utf-8")
        self.assertIn("kubectl set env statefulset/voice-minio", cors)
        self.assertIn("MINIO_API_CORS_ALLOW_ORIGIN", cors)

    def test_images_only_rollout_also_sets_public_signing_endpoint(self):
        for path in ("scripts/staging/render-and-apply.sh", "scripts/prod/render-and-apply-prod.sh"):
            rollout = (ROOT / path).read_text(encoding="utf-8")
            images_only = rollout.split("images-only)", 1)[1].split(";;", 1)[0]
            self.assertIn("apply-signing-env.sh", images_only)

    def test_optional_direct_https_storage_route_for_large_uploads(self):
        manifest = (ROOT / "deploy/storage/ingress.yaml").read_text(encoding="utf-8")
        self.assertIn("__STORAGE_INGRESS_HOST__", manifest)
        self.assertIn("__STORAGE_TLS_SECRET__", manifest)
        self.assertIn("name: voice-minio", manifest)
        for path in ("scripts/staging/apply-gateway-ingress.sh", "scripts/prod/apply-gateway-ingress.sh"):
            apply = (ROOT / path).read_text(encoding="utf-8")
            self.assertIn("VOICE_STORAGE_INGRESS_HOST", apply)
            self.assertIn("VOICE_STORAGE_TLS_SECRET", apply)
        for path in ("scripts/staging/apply-app-manifests.sh", "scripts/prod/apply-app-manifests.sh"):
            render = (ROOT / path).read_text(encoding="utf-8")
            self.assertIn("VOICE_STORAGE_INGRESS_HOST", render)


if __name__ == "__main__":
    unittest.main()
