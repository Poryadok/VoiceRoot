# Object storage operations

MinIO is the default S3-compatible store for Compose, staging, and production.
Cloudflare R2 or another managed S3-compatible provider is optional; it is not a
runtime requirement and does not change the File API.

## Image policy and registry bootstrap

The checked-in defaults are multi-platform upstream references pinned as
`tag@sha256`: MinIO `RELEASE.2024-12-18T13-15-44Z@sha256:1dce27c494a16bae114774f1cec295493f3613142713130c2d22dd5696be6ad3`
and mc `RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727`.
The digests are recorded from `docker buildx imagetools inspect` against Quay;
the dated tag is included for operator auditability, not as the integrity lock.

For an internal mirror, a package administrator with `packages:write` performs
the following once for each exact digest, then grants the CI repository pull
access to the resulting GHCR package:

```sh
docker pull quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z@sha256:1dce27c494a16bae114774f1cec295493f3613142713130c2d22dd5696be6ad3
docker tag quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z@sha256:1dce27c494a16bae114774f1cec295493f3613142713130c2d22dd5696be6ad3 ghcr.io/OWNER/voice-minio:RELEASE.2024-12-18T13-15-44Z
docker push ghcr.io/OWNER/voice-minio:RELEASE.2024-12-18T13-15-44Z
```

Resolve and record the mirror digest after the push, then set both
`VOICE_MINIO_IMAGE` and `VOICE_MINIO_MC_IMAGE` to mirror `tag@sha256` values in
the deployment/CI environment. Until a mirror is pre-seeded, the non-Docker-Hub
Quay defaults keep the A1 proof runnable without anonymous Docker Hub access.

## k3s deployment

`scripts/staging/apply-infra.sh` and `scripts/prod/apply-infra.sh` render
`deploy/*/minio.yaml`. Each environment gets an internal `voice-minio` Service,
a one-replica StatefulSet with readiness/liveness probes, and a
ReadWriteOnce PVC. Configure `VOICE_MINIO_STORAGE_CLASS` and
`VOICE_MINIO_STORAGE_SIZE` (staging defaults: `local-path`/`20Gi`; production:
`local-path`/`100Gi`) for the cluster. The two bucket Jobs use `mc mb
--ignore-existing`, so retries are idempotent.

For staging, store `STAGING_MINIO_ROOT_USER` and `STAGING_MINIO_ROOT_PASSWORD`
in GitHub repository **Settings → Environments → staging → Environment secrets**.
The staging deploy creates `voice-minio-credentials` from them when absent and
never rotates an existing Secret. For production, create the Secret from the
external secret manager before infra apply. Copy the stage/prod
`secret.example.yaml` shape but never commit credentials. `voice-app-secrets`
receives the existing `USER_R2_*` and `FILE_R2_*` names because those are
service configuration names; they accept
any S3-compatible endpoint. For MinIO use `http://voice-minio:9000`, region
`us-east-1`, and the environment-specific avatar/file buckets.
The staging `STAGING_APP_SECRETS_YAML` environment secret is a base64-encoded
`voice-app-secrets` manifest; its `USER_R2_*` and `FILE_R2_*` credentials must
match the MinIO Secret when those services use MinIO.

## Backup, restore, and optional provider migration

Back up the MinIO PVC with the cluster storage snapshot/backup facility and
test a restore into an isolated namespace first. Preserve the matching
`voice-minio-credentials` secret through the secret manager; changing keys
without a deliberate object migration makes existing objects inaccessible.

For MinIO to R2 (or another provider), create destination buckets, copy with a
version-aware tool, verify object counts and SHA-256 metadata/sample downloads,
then atomically update all six endpoint/region/key/bucket values for both
`USER_R2_*` and `FILE_R2_*`. Keep the old store read-only until File download
and delete-access checks pass. Do not use a destructive mirror/delete option.
Rollback is the same configuration switch while the old store and credentials
are retained. This procedure does not expose a public restore endpoint and is
not account recovery.
