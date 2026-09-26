#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
HELPER="${ROOT}/scripts/staging/replace-minio-bucket-jobs.sh"
APPLY="${ROOT}/scripts/staging/apply-infra.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "${TMP}/bin" "${TMP}/fixtures"

fail() { echo "FAIL: $*" >&2; exit 1; }

cat >"${TMP}/bin/kubectl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
action="$1"
resource="$2"
name="$3"
printf '%s:%s\n' "$action" "$name" >>"${CALLS}"
if [[ "$action" == get ]]; then
  fixture="${FIXTURES}/${name}.json"
  if [[ -f "$fixture" ]]; then cat "$fixture"; fi
elif [[ "$action" == delete ]]; then
  exit 0
else
  echo "unexpected kubectl action" >&2
  exit 64
fi
SH
chmod +x "${TMP}/bin/kubectl"
export PATH="${TMP}/bin:${PATH}" CALLS="${TMP}/calls" FIXTURES="${TMP}/fixtures"
touch "$CALLS"

make_job() {
  local name="$1" image="$2" status="$3" unexpected="${4:-false}"
  python - "$name" "$image" "$status" "$unexpected" <<'PY'
import json, sys
name, image, status, unexpected = sys.argv[1:]
bucket = "avatars" if name.endswith("avatars-bucket") else "files"
container = {
    "name": "mc", "image": image,
    "args": ["mb", "--ignore-existing", f"local/voice-staging-{bucket}"],
    "env": [
        {"name": "MINIO_ROOT_USER", "valueFrom": {"secretKeyRef": {"name": "voice-minio-credentials", "key": "MINIO_ROOT_USER"}}},
        {"name": "MINIO_ROOT_PASSWORD", "valueFrom": {"secretKeyRef": {"name": "voice-minio-credentials", "key": "MINIO_ROOT_PASSWORD"}}},
        {"name": "MC_HOST_local", "value": "http://$(MINIO_ROOT_USER):$(MINIO_ROOT_PASSWORD)@voice-minio:9000"},
    ],
}
if unexpected == "true":
    container["command"] = ["sh", "-c", "echo unexpected"]
job = {
    "apiVersion": "batch/v1", "kind": "Job",
    "metadata": {"name": name, "namespace": "voice-staging", "uid": "fixture-uid"},
    "spec": {"backoffLimit": 10, "selector": {"matchLabels": {"batch.kubernetes.io/job-name": name, "batch.kubernetes.io/controller-uid": "fixture-uid"}}, "template": {"metadata": {"labels": {"batch.kubernetes.io/job-name": name, "batch.kubernetes.io/controller-uid": "fixture-uid"}}, "spec": {"restartPolicy": "OnFailure", "containers": [container]}}},
    "status": {"succeeded": 1, "failed": 0, "conditions": [{"type": "Complete", "status": "True"}]} if status == "complete" else ({"active": 1} if status == "active" else {}),
}
print(json.dumps(job))
PY
}

EXPECTED_IMAGE="ghcr.io/poryadok/voiceroot/minio-mc:reviewed@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
LEGACY_IMAGE="quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727"

# A same-image completed job remains untouched, and absent jobs are harmless.
make_job voice-minio-create-avatars-bucket "$EXPECTED_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
rm -f "${FIXTURES}/voice-minio-create-files-bucket.json"
bash "$HELPER" voice-staging "$EXPECTED_IMAGE"
[[ "$(cat "$CALLS")" == $'get:voice-minio-create-avatars-bucket\nget:voice-minio-create-files-bucket' ]] || fail 'same-image/absent Jobs must not be deleted'

# Only the known previous immutable mc image on a completed expected Job is replaced.
: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
make_job voice-minio-create-files-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-files-bucket.json"
bash "$HELPER" voice-staging "$EXPECTED_IMAGE"
[[ "$(cat "$CALLS")" == $'get:voice-minio-create-avatars-bucket\ndelete:voice-minio-create-avatars-bucket\nget:voice-minio-create-files-bucket\ndelete:voice-minio-create-files-bucket' ]] || fail 'completed image-drift Jobs must be replaced one by one'

# Never delete an active job or a job with an unexpected action, even if its image is old.
: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" active >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'active stale Job must fail closed'; fi
[[ "$(cat "$CALLS")" == 'get:voice-minio-create-avatars-bucket' ]] || fail 'active Job must not be deleted'

: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete true >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'unexpected Job spec must fail closed'; fi
[[ "$(cat "$CALLS")" == 'get:voice-minio-create-avatars-bucket' ]] || fail 'unexpected Job spec must not be deleted'

# The guard must run before the immutable Job manifest is applied.
guard_line="$(awk '/replace-minio-bucket-jobs\.sh/ { print NR; exit }' "$APPLY")"
manifest_line="$(awk '/deploy\/staging\/minio\.yaml/ { print NR; exit }' "$APPLY")"
[[ -n "$guard_line" && -n "$manifest_line" && "$guard_line" -lt "$manifest_line" ]] || fail 'stale Job guard must run before MinIO manifest apply'

echo 'PASS: staging MinIO bucket Jobs are replaced only for known completed image drift.'
