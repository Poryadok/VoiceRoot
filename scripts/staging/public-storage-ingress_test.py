"""Guard the public S3 ingress and application endpoint contract."""

from pathlib import Path
import base64
import sys
import unittest


ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'scripts/staging/nats-rollout-preservation'))
import source_plan
import source_plan_test as fixtures
import nonnats_plan


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
        for path in ("scripts/prod/render-and-apply-prod.sh",):
            rollout = (ROOT / path).read_text(encoding="utf-8")
            self.assertGreaterEqual(rollout.count("apply-minio-cors.sh"), 2, path)
        cors = (ROOT / "scripts/storage/apply-minio-cors.sh").read_text(encoding="utf-8")
        self.assertIn("kubectl set env statefulset/voice-minio", cors)
        self.assertIn("MINIO_API_CORS_ALLOW_ORIGIN", cors)
        for mode in ('full', 'app-only'):
            kube, params = fixtures.SourceTests().canonical_fixture(mode)
            plan = source_plan.compile_plan(ROOT, params, fixtures.decoder, kube)
            row = next(r for r in plan['checks'] if r['id'] == 'minio-cors')
            minio = next(o for o in row['objects'] if o['name'] == 'voice-minio')
            env = minio['desired']['spec']['template']['spec']['containers'][0]['env']
            self.assertIn({'name': 'MINIO_API_CORS_ALLOW_ORIGIN', 'value': 'https://app.comrade.click'}, env)
            self.assertFalse(any(a['manifest']['kind'] in ('StatefulSet', 'PersistentVolumeClaim') for a in plan['actions']))
            params['web_origin'] = 'https://changed.example.invalid'
            with self.assertRaises(nonnats_plan.PlanError):
                source_plan.compile_plan(ROOT, params, fixtures.decoder, kube)

    def test_images_only_rollout_also_sets_public_signing_endpoint(self):
        for path in ("scripts/prod/render-and-apply-prod.sh",):
            rollout = (ROOT / path).read_text(encoding="utf-8")
            images_only = rollout.split("images-only)", 1)[1].split(";;", 1)[0]
            self.assertIn("apply-signing-env.sh", images_only)
        kube, params = fixtures.fixture()
        plan = source_plan.compile_plan(ROOT, params, fixtures.decoder, kube)
        row = next(r for r in plan['checks'] if r['id'] == 'signing-endpoint')
        for service in ('file', 'user'):
            app = next(o for o in row['objects'] if o['name'] == 'voice-' + service)
            env = app['desired']['spec']['template']['spec']['containers'][0]['env']
            self.assertIn({'name': service.upper() + '_R2_SIGNING_ENDPOINT', 'value': params['s3_signing_endpoint']}, env)
        params['s3_signing_endpoint'] = 'https://changed.example.invalid'
        with self.assertRaisesRegex(nonnats_plan.PlanError, 'signing'):
            source_plan.compile_plan(ROOT, params, fixtures.decoder, kube)

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

    def test_storage_ingress_is_ready_before_any_rollout_changes_signers(self):
        for path in ("scripts/prod/render-and-apply-prod.sh",):
            rollout = (ROOT / path).read_text(encoding="utf-8")
            for mode in ("images-only)", "app-only)", "full|*)"):
                branch = rollout.split(mode, 1)[1].split(";;", 1)[0]
                ingress = branch.index("apply-gateway-ingress.sh")
                for mutation in ("rollout-subset.sh", "deploy-changed.sh", "apply-signing-env.sh", "apply-app-manifests.sh"):
                    if mutation in branch:
                        self.assertLess(ingress, branch.index(mutation), (path, mode, mutation))
        for mode in ('full', 'app-only', 'images-only'):
            kube, params = fixtures.SourceTests().canonical_fixture(mode)
            params['storage_host'] = 'storage.example.invalid'
            params['storage_tls_secret'] = 'voice-storage-tls'
            import yaml
            raw = (ROOT / 'deploy/storage/ingress.yaml').read_text()
            for token, value in {'__STORAGE_INGRESS_HOST__': params['storage_host'],
                    '__STORAGE_TLS_SECRET__': params['storage_tls_secret'], '__FILE_BUCKET__': 'voice-staging-files',
                    '__AVATAR_BUCKET__': 'voice-staging-avatars'}.items(): raw = raw.replace(token, value)
            for obj in yaml.safe_load_all(raw):
                name = obj['metadata']['name']; obj['metadata'].update(uid=name, resourceVersion='1', namespace='voice-staging')
                kube.objects[(obj['kind'], name)] = obj
            kube.objects[('Secret', 'voice-storage-tls')] = {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'kubernetes.io/tls',
                'metadata': {'name': 'voice-storage-tls', 'namespace': 'voice-staging', 'uid': 'storage-tls', 'resourceVersion': '1'},
                'data': {key: base64.b64encode(b'fixture-private').decode() for key in ('tls.crt', 'tls.key')}}
            plan = source_plan.compile_plan(ROOT, params, fixtures.decoder, kube)
            objects = [o for row in plan['checks'] for o in row['objects']]
            self.assertTrue(any(o['kind'] == 'Ingress' and 'storage' in o['name'] for o in objects))
            self.assertTrue(any(o['kind'] == 'Secret' and o['name'] == 'voice-storage-tls' for o in objects))
            del kube.objects[('Secret', 'voice-storage-tls')]
            with self.assertRaises(nonnats_plan.PlanError):
                source_plan.compile_plan(ROOT, params, fixtures.decoder, kube)
            self.assertTrue(all(call[0] != 'run' or call[1][0] in ('get', 'exec')
                or call[1][0] == 'apply' and '--dry-run=server' in call[1] for call in kube.calls))
        for path in ("scripts/staging/apply-gateway-ingress.sh", "scripts/prod/apply-gateway-ingress.sh"):
            apply = (ROOT / path).read_text(encoding="utf-8")
            self.assertLess(apply.index("storage TLS Secret missing"), apply.index('"${ROOT}/deploy/gateway/ingress.yaml" | kubectl apply'))

    def test_production_smoke_uses_production_namespace(self):
        smoke = (ROOT / "scripts/prod/smoke-prod.sh").read_text(encoding="utf-8")
        self.assertIn('export VOICE_K8S_NAMESPACE="${VOICE_K8S_NAMESPACE:-voice-prod}"', smoke)
        self.assertLess(smoke.index('export VOICE_K8S_NAMESPACE='), smoke.index('smoke-staging.sh'))


if __name__ == "__main__":
    unittest.main()
