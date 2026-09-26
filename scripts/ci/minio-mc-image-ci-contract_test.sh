#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/ci.yml"
DOCKERFILE="${ROOT}/docker/minio/mc.Dockerfile"
SERVER_DOCKERFILE="${ROOT}/docker/minio/minio.Dockerfile"
STAGING_APPLY="${ROOT}/scripts/staging/apply-infra.sh"
PROD_APPLY="${ROOT}/scripts/prod/apply-infra.sh"
JOB_FILE="$(mktemp)"
trap 'rm -f "$JOB_FILE"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

awk '
  /^  a1-attachment-restart-proof:/ { in_job = 1 }
  in_job && /^  [A-Za-z0-9_-]+:/ && $0 !~ /^  a1-attachment-restart-proof:/ { exit }
  in_job { print }
' "$WORKFLOW" >"$JOB_FILE"

grep -Fq 'docker build -f docker/minio/mc.Dockerfile -t voice-minio-mc:ci .' "$JOB_FILE" || fail 'A1 proof must build the verified mc image locally'
grep -Fq 'docker run --rm --entrypoint /bin/sh voice-minio-mc:ci -c' "$JOB_FILE" || fail 'A1 proof must check runtime tools and mc version'
grep -Fq 'docker build -f docker/minio/minio.Dockerfile -t voice-minio:ci .' "$JOB_FILE" || fail 'A1 proof must build the verified MinIO server image locally'
grep -Fq 'docker run --rm --entrypoint /bin/sh voice-minio:ci -c' "$JOB_FILE" || fail 'A1 proof must check server, curl, shell, and sleep'
grep -Fq 'VOICE_MINIO_MC_IMAGE: voice-minio-mc:ci' "$JOB_FILE" || fail 'A1 proof must use its local image through the existing override'
grep -Fq 'VOICE_MINIO_IMAGE: voice-minio:ci' "$JOB_FILE" || fail 'A1 proof must use its local server image through the existing override'
grep -Fq 'run: make compose-file-attachment-restart-proof' "$JOB_FILE" || fail 'A1 proof must retain the attachment restart integration test'

PUBLISH_JOB="$(awk '
  /^  minio-mc-image-publish:/ { in_job = 1 }
  in_job && /^  [A-Za-z0-9_-]+:/ && $0 !~ /^  minio-mc-image-publish:/ { exit }
  in_job { print }
' "$WORKFLOW")"
[[ -n "$PUBLISH_JOB" ]] || fail 'master GHCR publication job must exist'
grep -Fq "github.event_name == 'push'" <<<"$PUBLISH_JOB" || fail 'mc publication must be push-only'
grep -Fq "github.ref == 'refs/heads/master'" <<<"$PUBLISH_JOB" || fail 'mc publication must be master-only'
grep -Fq "needs.changes.outputs.a1_e2e == 'true'" <<<"$PUBLISH_JOB" || fail 'mc publication must run for A1 E2E changes'
grep -Fq 'packages: write' <<<"$PUBLISH_JOB" || fail 'mc publication must use scoped GHCR package permission'
grep -Fq 'tags: ${{ steps.image.outputs.base }}/minio-mc:${{ github.sha }}' <<<"$PUBLISH_JOB" || fail 'mc publication must use an immutable source-SHA tag'
grep -Fq 'steps.build.outputs.digest' <<<"$PUBLISH_JOB" || fail 'mc publication must expose its actual resulting digest'

[[ -f "$DOCKERFILE" ]] || fail 'verified mc Dockerfile must exist'
grep -Fq 'FROM alpine:3.24.2@sha256:' "$DOCKERFILE" || fail 'mc runtime must use the pinned Alpine base'
grep -Fq 'sha256sum -cs' "$DOCKERFILE" || fail 'mc release checksum must be verified during build'
grep -Fq 'minisign -Vqm' "$DOCKERFILE" || fail 'mc release signature must be verified during build'
grep -Fq 'ENTRYPOINT ["mc"]' "$DOCKERFILE" || fail 'mc default entrypoint must be preserved'
grep -Fq '7394ce0dd2a80935aded936b09fa12cbb3cb8096/LICENSE' "$DOCKERFILE" || fail 'mc license must come from the pinned upstream source revision'
grep -Fq '7394ce0dd2a80935aded936b09fa12cbb3cb8096/CREDITS' "$DOCKERFILE" || fail 'mc credits must come from the pinned upstream source revision'
grep -Fq 'COPY --from=verify /out/licenses/ /licenses/' "$DOCKERFILE" || fail 'mc image must install upstream license notices'

[[ -f "$SERVER_DOCKERFILE" ]] || fail 'verified MinIO server Dockerfile must exist'
grep -Fq 'FROM alpine:3.24.2@sha256:' "$SERVER_DOCKERFILE" || fail 'MinIO server must use the pinned Alpine base'
grep -Fq 'sha256sum -cs' "$SERVER_DOCKERFILE" || fail 'MinIO server release checksum must be verified during build'
grep -Fq 'minisign -Vqm' "$SERVER_DOCKERFILE" || fail 'MinIO server release signature must be verified during build'
grep -Fq 'ENTRYPOINT ["minio"]' "$SERVER_DOCKERFILE" || fail 'MinIO server entrypoint must accept Compose and Kubernetes server args'
grep -Fq 'org.opencontainers.image.licenses="AGPL-3.0-only"' "$SERVER_DOCKERFILE" || fail 'server image must record its AGPL license'
grep -Fq 'org.opencontainers.image.source="https://github.com/minio/minio"' "$SERVER_DOCKERFILE" || fail 'server image must identify upstream source'
grep -Fq '16f8cf1c52f0a77eeb8f7565aaf7f7df12454583/LICENSE' "$SERVER_DOCKERFILE" || fail 'server license must come from the pinned upstream source revision'
grep -Fq '16f8cf1c52f0a77eeb8f7565aaf7f7df12454583/CREDITS' "$SERVER_DOCKERFILE" || fail 'server credits must come from the pinned upstream source revision'
grep -Fq 'COPY --from=verify /out/licenses/ /licenses/' "$SERVER_DOCKERFILE" || fail 'server image must install upstream license notices'

SERVER_PUBLISH_JOB="$(awk '
  /^  minio-server-image-publish:/ { in_job = 1 }
  in_job && /^  [A-Za-z0-9_-]+:/ && $0 !~ /^  minio-server-image-publish:/ { exit }
  in_job { print }
' "$WORKFLOW")"
[[ -n "$SERVER_PUBLISH_JOB" ]] || fail 'master GHCR server publication job must exist'
grep -Fq "github.event_name == 'push'" <<<"$SERVER_PUBLISH_JOB" || fail 'server publication must be push-only'
grep -Fq "github.ref == 'refs/heads/master'" <<<"$SERVER_PUBLISH_JOB" || fail 'server publication must be master-only'
grep -Fq "needs.changes.outputs.a1_e2e == 'true'" <<<"$SERVER_PUBLISH_JOB" || fail 'server publication must run for A1 E2E changes'
grep -Fq 'packages: write' <<<"$SERVER_PUBLISH_JOB" || fail 'server publication must use scoped GHCR package permission'
grep -Fq 'platforms: linux/amd64' <<<"$SERVER_PUBLISH_JOB" || fail 'server publication must declare its verified platform scope'
grep -Fq 'tags: ${{ steps.image.outputs.base }}/minio:${{ github.sha }}' <<<"$SERVER_PUBLISH_JOB" || fail 'server publication must use an immutable source-SHA tag'
grep -Fq 'steps.build.outputs.digest' <<<"$SERVER_PUBLISH_JOB" || fail 'server publication must expose its actual resulting digest'

[[ -f "$STAGING_APPLY" ]] || fail 'staging infra apply script must exist'
grep -Fq 'MINIO_IMAGE="${VOICE_MINIO_IMAGE:-ghcr.io/poryadok/voiceroot/minio:86b2017f06d0d471e8b43abc78031e86756defe3@sha256:ab7687bc47a84c3aec0d9706dabd47b4719b081683cde745f8a1b84c6c7681e0}"' "$STAGING_APPLY" || fail 'staging MinIO server must use the verified immutable GHCR reference by default'
grep -Fq 'MINIO_MC_IMAGE="${VOICE_MINIO_MC_IMAGE:-ghcr.io/poryadok/voiceroot/minio-mc:86b2017f06d0d471e8b43abc78031e86756defe3@sha256:66a55c322fed37a3fefa0b815d195b01e7903d1bbffccd80d5a8af3cedf343f2}"' "$STAGING_APPLY" || fail 'staging mc must use the verified immutable GHCR reference by default'
grep -Fq 'MINIO_IMAGE="${VOICE_MINIO_IMAGE:-quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z@sha256:1dce27c494a16bae114774f1cec295493f3613142713130c2d22dd5696be6ad3}"' "$PROD_APPLY" || fail 'production MinIO default must remain unchanged'
grep -Fq 'MINIO_MC_IMAGE="${VOICE_MINIO_MC_IMAGE:-quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727}"' "$PROD_APPLY" || fail 'production mc default must remain unchanged'

echo 'PASS: A1 proof builds and validates the pinned official mc runtime before Compose.'
