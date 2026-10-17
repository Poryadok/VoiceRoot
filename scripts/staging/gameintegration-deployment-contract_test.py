#!/usr/bin/env python3
"""Source contract for internal-only GIS workload, image and migration wiring."""
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[2]


def documents(path):
    return re.split(r"(?m)^---\s*$", (ROOT / path).read_text(encoding="utf-8"))


def resource(path, kind, name):
    matches = [doc for doc in documents(path)
               if re.search(rf"(?m)^kind: {re.escape(kind)}$", doc)
               and re.search(rf"(?m)^  name: {re.escape(name)}$", doc)]
    assert len(matches) == 1, f"expected exactly one {kind} {name} in {path}"
    return matches[0]


catalog = json.loads((ROOT / "scripts/ci/staging-image-catalog.json").read_text(encoding="utf-8"))
entries = [row for row in catalog["images"] if row["name"] == "gameintegration"]
assert len(entries) == 1, "catalog must contain exactly one GIS image"
entry = entries[0]
assert entry["deployment"] == "voice-gameintegration"
assert entry["k8s_manifest"] == "deploy/staging/services.yaml"
assert entry["language"] == "go"
assert entry["build_context"] == "./src/backend"
assert entry["dockerfile"] == "./src/backend/gameintegration/Dockerfile"
assert entry["environments"] == ["dev", "staging"], "GIS production image/deploy remains deferred"

manifest = "deploy/staging/services.yaml"
deployment = resource(manifest, "Deployment", "voice-gameintegration")
service = resource(manifest, "Service", "voice-gameintegration")
for contract in (
    "image: __IMAGE_REGISTRY__/gameintegration:__IMAGE_TAG__",
    "name: DATABASE_URL",
    "key: GAME_INTEGRATION_DATABASE_URL",
    "name: GAME_INTEGRATION_REDIS_ADDR",
    "name: GAME_INTEGRATION_JWKS_URL",
    "name: GAME_INTEGRATION_JWT_ISSUER",
    "name: GAME_INTEGRATION_JWT_AUDIENCE",
    "name: GAME_INTEGRATION_PRINCIPAL_PRIVATE_KEY_FILE",
    "name: GAME_INTEGRATION_PRINCIPAL_NEXT_PRIVATE_KEY_FILE",
    "name: GAME_INTEGRATION_PRINCIPAL_JWKS_TLS_CERT_FILE",
    "name: GAME_INTEGRATION_PRINCIPAL_JWKS_TLS_KEY_FILE",
    "secretName: voice-gameintegration-principal-signing",
    "secretName: voice-gameintegration-jwks-tls",
):
    assert contract in deployment, f"GIS Deployment misses {contract}"
assert "port: 8443" in service and "name: principal-jwks" in service
assert "GAME_INTEGRATION_SESSION_" not in deployment, "session API remains disabled without its full operator tuple"
assert "BOT_INTERNAL_URL" not in deployment, "optional Bot authority must not be activated implicitly"
assert "FCM_" not in deployment and "APNS_" not in deployment
assert "secretName: voice-media-ca" not in deployment, "GIS does not need the media CA projection for its internal HTTP Auth JWKS"

role = resource(manifest, "Deployment", "voice-role")
assert '"gameintegration":"https://voice-gameintegration:8443/internal/v1/principal/jwks.json"' in role
assert "value: \"http://voice-auth:8080/api/v1/auth/.well-known/jwks.json\"" in deployment
assert "name: GAME_INTEGRATION_JWT_ISSUER, value: \"voice-auth\"" in deployment
assert "name: GAME_INTEGRATION_JWT_AUDIENCE, value: \"voice-client\"" in deployment

notification = resource(manifest, "Deployment", "voice-notification")
realtime = resource(manifest, "Deployment", "voice-realtime")
for doc, env, key in (
    (notification, "GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64", "GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64"),
    (realtime, "GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64", "GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64"),
):
    assert "name: GAME_INTEGRATION_CONSENT_URL" in doc
    assert "http://voice-gameintegration:8080" in doc
    assert f"name: {env}" in doc and f"key: {key}" in doc
    assert "voice-gameintegration-workload-credentials" in doc
assert "GAME_INTEGRATION_SESSION_" not in deployment

secret_example = (ROOT / "deploy/staging/secret.example.yaml").read_text(encoding="utf-8")
assert "GAME_INTEGRATION_DB_PASSWORD: \"REPLACE_WITH" in secret_example
assert "GAME_INTEGRATION_DATABASE_URL: \"postgres://gameintegration_runtime:" in secret_example
assert "/game_integration_db?sslmode=disable\"" in secret_example
assert "REPLACE_WITH_64_RANDOM_LOWERCASE_HEX_CHARACTERS" in secret_example

role_template = (ROOT / "deploy/templates/provision-gameintegration-runtime-role-job.yaml").read_text(encoding="utf-8")
assert "name: PGPASSWORD" in role_template and "key: POSTGRES_PASSWORD" in role_template
assert "name: GAME_INTEGRATION_DB_PASSWORD" in role_template
assert "configMapKeyRef: {name: voice-app-config, key: POSTGRES_USER}" in role_template
assert "pg_isready" in role_template
assert "valueFrom:" in role_template
assert "voice-compose-gameintegration-runtime" not in role_template
assert "REPLACE_WITH" not in role_template
gis_secrets_patch = (ROOT / "scripts/staging/patch-app-secrets-database-urls.sh").read_text(encoding="utf-8")
assert "GAME_INTEGRATION_DATABASE_URL" not in gis_secrets_patch, "admin password sync must not overwrite the restricted GIS URL"
staging_migrations = (ROOT / "scripts/staging/apply-migrate-jobs.sh").read_text(encoding="utf-8")
assert "postgres://gameintegration_runtime:${role_password}@voice-postgres:5432/game_integration_db?sslmode=disable" in staging_migrations
assert "64-character random hex password" in staging_migrations
migration_template = (ROOT / "deploy/templates/migrate-game-integration-db-job.yaml").read_text(encoding="utf-8")
assert "key: POSTGRES_PASSWORD" in migration_template
assert "value: game_integration_db" in migration_template
assert "rfc3986_encode_component" in migration_template
assert "__DATABASE_URL__" not in migration_template
assert "GAME_INTEGRATION_DATABASE_URL" not in migration_template

assert not any("voice-gameintegration" in p.read_text(encoding="utf-8")
               for p in (ROOT / "deploy/prod").rglob("*.yaml")), "GIS must remain absent from production"

prod_verifier = (ROOT / "scripts/prod/verify-prod-images.sh").read_text(encoding="utf-8")
assert 'select((.environments // ["dev", "staging", "prod"]) | index("prod"))' in prod_verifier

database_initializer = (ROOT / "scripts/staging/init-postgres-databases.sh").read_text(encoding="utf-8")
assert 'VOICE_ENABLE_GAME_INTEGRATION_DB:-false' in database_initializer
assert 'databases+=(game_integration_db)' in database_initializer
assert 'game_integration_db' not in database_initializer.split('databases=(')[1].split(')', 1)[0]
assert "VOICE_ENABLE_GAME_INTEGRATION_DB must be true or false" in database_initializer
dev_bootstrap = (ROOT / "scripts/dev/bootstrap.py").read_text(encoding="utf-8")
stage_infra = (ROOT / "scripts/staging/apply-infra.sh").read_text(encoding="utf-8")
prod_infra = (ROOT / "scripts/prod/apply-infra.sh").read_text(encoding="utf-8")
prod_renderer = (ROOT / "scripts/prod/render-and-apply-prod.sh").read_text(encoding="utf-8")
stage_migration_runner = (ROOT / "scripts/staging/apply-migrate-jobs.sh").read_text(encoding="utf-8")
assert "VOICE_ENABLE_GAME_INTEGRATION_DB='true'" in dev_bootstrap
assert 'VOICE_ENABLE_GAME_INTEGRATION_DB=true bash' in stage_infra
assert 'VOICE_ENABLE_GAME_INTEGRATION_DB=false bash' in prod_infra
assert 'VOICE_ENABLE_GAME_INTEGRATION_DB=false bash' in prod_renderer
assert 'VOICE_ENABLE_GAME_INTEGRATION_DB:-false' in stage_migration_runner
assert 'if [ "${GIS_MIGRATIONS_ENABLED}" = true ]; then' in stage_migration_runner
assert "Skipping internal Game Integration migrations" in stage_migration_runner

workload_example = (ROOT / "deploy/staging/gameintegration-workload-credentials.example.yaml").read_text(encoding="utf-8")
assert "REPLACE_WITH_NOTIFICATION_KEY_B64" in workload_example
assert "REPLACE_WITH_REALTIME_KEY_B64" in workload_example
assert "voice-gameintegration-workload-credentials" in workload_example

print("Game Integration internal deployment contract passed.")
