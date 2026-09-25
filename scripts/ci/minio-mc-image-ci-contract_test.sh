#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/ci.yml"
DOCKERFILE="${ROOT}/docker/minio/mc.Dockerfile"
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
grep -Fq 'VOICE_MINIO_MC_IMAGE: voice-minio-mc:ci' "$JOB_FILE" || fail 'A1 proof must use its local image through the existing override'
grep -Fq 'run: make compose-file-attachment-restart-proof' "$JOB_FILE" || fail 'A1 proof must retain the attachment restart integration test'

PUBLISH_JOB="$(awk '
  /^  minio-mc-image-publish:/ { in_job = 1 }
  in_job && /^  [A-Za-z0-9_-]+:/ && $0 !~ /^  minio-mc-image-publish:/ { exit }
  in_job { print }
' "$WORKFLOW")"
[[ -n "$PUBLISH_JOB" ]] || fail 'master GHCR publication job must exist'
grep -Fq "github.event_name == 'push'" <<<"$PUBLISH_JOB" || fail 'mc publication must be push-only'
grep -Fq "github.ref == 'refs/heads/master'" <<<"$PUBLISH_JOB" || fail 'mc publication must be master-only'
grep -Fq 'packages: write' <<<"$PUBLISH_JOB" || fail 'mc publication must use scoped GHCR package permission'
grep -Fq 'tags: ${{ steps.image.outputs.base }}/minio-mc:${{ github.sha }}' <<<"$PUBLISH_JOB" || fail 'mc publication must use an immutable source-SHA tag'
grep -Fq 'steps.build.outputs.digest' <<<"$PUBLISH_JOB" || fail 'mc publication must expose its actual resulting digest'

[[ -f "$DOCKERFILE" ]] || fail 'verified mc Dockerfile must exist'
grep -Fq 'FROM alpine:3.24.2@sha256:' "$DOCKERFILE" || fail 'mc runtime must use the pinned Alpine base'
grep -Fq 'sha256sum -cs' "$DOCKERFILE" || fail 'mc release checksum must be verified during build'
grep -Fq 'minisign -Vqm' "$DOCKERFILE" || fail 'mc release signature must be verified during build'
grep -Fq 'ENTRYPOINT ["mc"]' "$DOCKERFILE" || fail 'mc default entrypoint must be preserved'

echo 'PASS: A1 proof builds and validates the pinned official mc runtime before Compose.'
