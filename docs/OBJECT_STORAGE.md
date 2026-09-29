# Object storage operations

MinIO is the default S3-compatible store for Compose, staging, and production.
Cloudflare R2 or another managed S3-compatible provider is optional; it is not a
runtime requirement and does not change the File API.

## Image policy and registry bootstrap

The local Compose MinIO server default is a multi-platform upstream reference
pinned as `tag@sha256`:
`RELEASE.2024-12-18T13-15-44Z@sha256:1dce27c494a16bae114774f1cec295493f3613142713130c2d22dd5696be6ad3`.
CI builds both runtime images from the exact official releases: the mc client
`RELEASE.2025-08-13T08-35-41Z` and MinIO server
`RELEASE.2024-12-18T13-15-44Z`, for Linux amd64. Each build checks the
published SHA-256 and MinIO minisign signature. The server image also records
its upstream GitHub source, verified release commit, and AGPL-3.0-only license
in OCI labels. See [`docker/minio/mc.Dockerfile`](../docker/minio/mc.Dockerfile)
and [`docker/minio/minio.Dockerfile`](../docker/minio/minio.Dockerfile). The
attachment restart proof checks each binary and its required shell utilities,
then selects the local images with `VOICE_MINIO_MC_IMAGE` and
`VOICE_MINIO_IMAGE` while leaving explicit caller overrides intact.

On a master push, `minio-mc-image-publish` and `minio-server-image-publish` publish the
verified images to
`ghcr.io/<owner>/<repository>/minio-mc:<commit-sha>` and
`ghcr.io/<owner>/<repository>/minio:<commit-sha>`, respectively, and record the
registry-reported digests in the workflow summary. The repackaged server image
is Linux amd64 for the current hosted proof and staging runtime. Staging now
defaults to the reviewed immutable references published from master commit
`86b2017f06d0d471e8b43abc78031e86756defe3`:

- Server: `ghcr.io/poryadok/voiceroot/minio:86b2017f06d0d471e8b43abc78031e86756defe3@sha256:ab7687bc47a84c3aec0d9706dabd47b4719b081683cde745f8a1b84c6c7681e0`
- mc: `ghcr.io/poryadok/voiceroot/minio-mc:86b2017f06d0d471e8b43abc78031e86756defe3@sha256:66a55c322fed37a3fefa0b815d195b01e7903d1bbffccd80d5a8af3cedf343f2`

The staging defaults are in `scripts/staging/apply-infra.sh`; explicit
`VOICE_MINIO_IMAGE` and `VOICE_MINIO_MC_IMAGE` overrides remain available.
On GitHub Actions, `compose-e2e`, `a1-e2e`, and
`a1-flutter-profile-handoff` use those same immutable server and mc references.
Each job authenticates to GHCR with its read-only `GITHUB_TOKEN` permission and
checks the rendered Compose image references before starting containers. The
attachment restart proof builds and verifies the same official binaries locally
in its isolated job. Local Compose and production defaults remain unchanged.
Do not infer digests from tags or use mutable tags as substitutes.

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
the deployment/CI environment. Grant the CI repository pull access to the
packages; the GitHub Actions jobs use only `packages:read` and do not expose
registry credentials to Compose or the application services.

## k3s deployment

`scripts/staging/apply-infra.sh` and `scripts/prod/apply-infra.sh` render
`deploy/*/minio.yaml`. Each environment gets an internal `voice-minio` Service,
a one-replica StatefulSet with readiness/liveness probes, and a
ReadWriteOnce PVC. Configure `VOICE_MINIO_STORAGE_CLASS` and
`VOICE_MINIO_STORAGE_SIZE` (staging defaults: `local-path`/`20Gi`; production:
`local-path`/`100Gi`) for the cluster. The two bucket Jobs use `mc mb
--ignore-existing`, so retries are idempotent. Kubernetes Job pod templates
are immutable: staging replaces an existing bucket Job only when it matches the
expected bucket action and credential references, uses the previous pinned mc
image, and has completed successfully. The guard accepts the observed Kubernetes
defaults (`manualSelector: false`, `podReplacementPolicy: TerminatingOrFailed`,
and empty container resources) and the exact controller/job labels emitted by
the cluster; other selector fields, labels, actions, and credentials remain
unexpected. Active Jobs and unexpected specs stop infra apply without deletion;
a Job already using the selected image is kept.
The replacement guard is restricted to `voice-staging` and deletes with the
observed Job UID as a precondition, so a same-name replacement after inspection
is left untouched.

For staging, store `STAGING_MINIO_ROOT_USER` and `STAGING_MINIO_ROOT_PASSWORD`
in GitHub repository **Settings → Environments → staging → Environment secrets**.
The staging deploy creates `voice-minio-credentials` from them when absent and
never rotates an existing Secret. For production, create the Secret from the
external secret manager before infra apply. Copy the stage/prod
`secret.example.yaml` shape but never commit credentials. In staging, User and
File read their `USER_R2_*` and `FILE_R2_*` access key and secret directly from
`voice-minio-credentials` (`MINIO_ROOT_USER` and `MINIO_ROOT_PASSWORD`). The
base64-encoded `STAGING_APP_SECRETS_YAML` manifest therefore needs only their
endpoint, region, and separate avatar/file bucket settings; MinIO credentials
must not be copied into `voice-app-secrets`. For MinIO use
`http://voice-minio:9000`, region `us-east-1`, and the environment-specific
avatar/file buckets. Production keeps its own provider credentials in
`voice-app-secrets`.

## Browser-facing signed URLs

`*_R2_ENDPOINT` remains the internal S3 endpoint for File object operations.
For browser PUT/GET, File and User sign against `*_R2_SIGNING_ENDPOINT` instead;
the optional setting falls back to `*_R2_ENDPOINT` for local Compose and an
externally reachable S3 provider. A signed URL's Host and path are part of
SigV4, so never rewrite the URL after signing.

Staging and default MinIO production deployments set both signing endpoints to
`https://<VOICE_GATEWAY_INGRESS_HOST>` when rendering User/File Deployments.
The Gateway host's Ingress sends only the path-style file and avatar bucket
prefixes to `voice-minio:9000`, without stripping or rewriting the path, on both
the `web` (Cloudflare Flex origin) and `websecure` entrypoints. The `/` route
continues to Gateway. The ingress apply scripts read `FILE_R2_BUCKET` and
`USER_R2_BUCKET` from the existing `voice-app-secrets` Secret and route those
exact bucket names; malformed or missing names stop deployment. The manual
PowerShell ingress helper accepts `-FileBucket` and `-AvatarBucket` when values
differ from its staging defaults. No new DNS name or credential is required.

`VOICE_S3_SIGNING_ENDPOINT` can override the rendered public endpoint for an
external S3 provider. It must be a browser-reachable HTTPS S3 API origin that
supports path-style signing for both configured buckets; do not point it to a
CDN read origin. User's `USER_R2_PUBLIC_BASE_URL` is a separate avatar display
URL and does not sign uploads. MinIO's `MINIO_API_CORS_ALLOW_ORIGIN` is rendered
from `https://<VOICE_WEB_INGRESS_HOST>` so browser PUT preflight allows the Web
client origin. The staging smoke performs read-only OPTIONS preflight against
both bucket routes after deployment; it does not upload an object or print a
signed URL. Selective `app-only` and `images-only` rollouts update only MinIO's
CORS environment setting on its existing StatefulSet and wait for readiness;
the PVC and object data stay in place. `images-only` also sets the File/User
signing endpoint on the existing Deployments so the new binary never falls back
to the internal ClusterDNS URL.

File signed GET responses request `Cache-Control: no-store` through the signed
S3 response override. Keep Cloudflare cache bypass for these bucket paths as an
edge rule as well; otherwise a previously cached old signed GET may outlive its
one-hour expiry until that cache entry ages out or is purged. Cloudflare's
proxied-host upload-size limit is independent of Voice's 200 MiB premium API
limit ([Cloudflare 413 limits](https://developers.cloudflare.com/support/troubleshooting/http-status-codes/4xx-client-error/error-413/)).
The current Gateway-host route is suitable for small A1 browser uploads.
For full-size premium uploads, configure a DNS-only public storage hostname in
`VOICE_STORAGE_INGRESS_HOST` (GitHub staging variable; production uses
`VOICE_PROD_STORAGE_INGRESS_HOST`) and a namespace-local TLS Secret with a
certificate valid for that name. `VOICE_STORAGE_TLS_SECRET` selects its name
(production: `VOICE_PROD_STORAGE_TLS_SECRET`; default `voice-storage-tls`). The
deployment then signs against that HTTPS host and applies a separate
`websecure` ingress limited to the two exact bucket prefixes. The deploy checks
that the TLS Secret exists and has type `kubernetes.io/tls`; the operator must
verify DNS-only routing and certificate SAN outside the repository. The staging
smoke probes the selected signing host with read-only CORS OPTIONS. Do not
treat the 200 MiB API limit as verified on the proxied Gateway route or until a
full-size upload passes through the direct route.

## Backup, restore, and optional provider migration

Back up the MinIO PVC with the cluster storage snapshot/backup facility and
test a restore into an isolated namespace first. Preserve the matching
`voice-minio-credentials` secret through the secret manager; changing keys
without a deliberate object migration makes existing objects inaccessible.

For MinIO to R2 (or another provider), create destination buckets, copy with a
version-aware tool, verify object counts and SHA-256 metadata/sample downloads,
then atomically update the endpoint, region, key, and bucket values for both
`USER_R2_*` and `FILE_R2_*`, plus `VOICE_S3_SIGNING_ENDPOINT` to the provider's
public HTTPS S3 API origin. On staging, first change the User/File Deployment
credential references from `voice-minio-credentials` to the new provider Secret
as part of that migration; changing only `STAGING_APP_SECRETS_YAML` cannot
switch credentials. Keep the old store read-only until File download
and delete-access checks pass. Do not use a destructive mirror/delete option.
Rollback is the same configuration switch while the old store and credentials
are retained. This procedure does not expose a public restore endpoint and is
not account recovery.
