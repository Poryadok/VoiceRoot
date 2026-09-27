#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
HELPER="${ROOT}/scripts/staging/replace-minio-bucket-jobs.sh"
APPLY="${ROOT}/scripts/staging/apply-infra.sh"
TMP="$(mktemp -d)"
REAL_KUBECTL="$(command -v kubectl || true)"
API_PID=""
cleanup() {
  if [[ -n "$API_PID" ]]; then kill "$API_PID" 2>/dev/null || true; wait "$API_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT
mkdir -p "${TMP}/bin" "${TMP}/fixtures"

fail() { echo "FAIL: $*" >&2; exit 1; }

cat >"${TMP}/bin/kubectl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
action="$1"
resource="$2"
if [[ "$action" == get ]]; then
  name="$3"
  printf '%s:%s:\n' "$action" "$name" >>"${CALLS}"
  fixture="${FIXTURES}/${name}.json"
  if [[ -f "$fixture" ]]; then cat "$fixture"; fi
elif [[ "$action" == delete ]]; then
  name="${resource##*/}"
  if [[ "${USE_REAL_KUBECTL:-false}" == true ]]; then exec "${REAL_KUBECTL}" "$@"; fi
  body="$(cat)"
  uid="$(printf '%s' "$body" | jq -er '.preconditions.uid')"
  printf '%s:%s:uid=%s\n' "$action" "$name" "$uid" >>"${CALLS}"
  if [[ -n "${CURRENT_UID:-}" && "$uid" != "$CURRENT_UID" ]]; then
    echo 'delete UID precondition conflict' >&2
    exit 1
  fi
  exit 0
elif [[ "$action" == wait ]]; then
  name="${3#job/}"
  printf '%s:%s:\n' "$action" "$name" >>"${CALLS}"
  exit 0
else
  echo "unexpected kubectl action" >&2
  exit 64
fi
SH
chmod +x "${TMP}/bin/kubectl"
export PATH="${TMP}/bin:${PATH}" CALLS="${TMP}/calls" FIXTURES="${TMP}/fixtures"
export REAL_KUBECTL
touch "$CALLS"

make_job() {
  local name="$1" image="$2" status="$3" unexpected="${4:-false}" selector_key="${5:-controller-uid}"
  python - "$name" "$image" "$status" "$unexpected" "$selector_key" <<'PY'
import json, sys
name, image, status, unexpected, selector_key = sys.argv[1:]
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
    "spec": {"backoffLimit": 10, "manualSelector": False, "podReplacementPolicy": "TerminatingOrFailed", "selector": {"matchLabels": {selector_key: "fixture-uid"}}, "template": {"metadata": {"labels": {"batch.kubernetes.io/job-name": name, "batch.kubernetes.io/controller-uid": "fixture-uid", "job-name": name, "controller-uid": "fixture-uid"}}, "spec": {"restartPolicy": "OnFailure", "containers": [container]}}},
    "status": {"succeeded": 1, "failed": 0, "conditions": [{"type": "Complete", "status": "True"}]} if status == "complete" else ({"active": 1} if status == "active" else {}),
}
container["resources"] = {}
print(json.dumps(job))
PY
}

EXPECTED_IMAGE="ghcr.io/poryadok/voiceroot/minio-mc:reviewed@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
LEGACY_IMAGE="quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727"

# A same-image completed job remains untouched, and absent jobs are harmless.
: >"$CALLS"
if bash "$HELPER" voice-prod "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'non-staging namespace must be rejected'; fi
[[ ! -s "$CALLS" ]] || fail 'namespace rejection must precede kubectl access'

make_job voice-minio-create-avatars-bucket "$EXPECTED_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
rm -f "${FIXTURES}/voice-minio-create-files-bucket.json"
bash "$HELPER" voice-staging "$EXPECTED_IMAGE"
[[ "$(cat "$CALLS")" == $'get:voice-minio-create-avatars-bucket:\nget:voice-minio-create-files-bucket:' ]] || fail 'same-image/absent Jobs must not be deleted'

# Only the known previous immutable mc image on a completed expected Job is replaced.
: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
make_job voice-minio-create-files-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-files-bucket.json"
bash "$HELPER" voice-staging "$EXPECTED_IMAGE"
[[ "$(cat "$CALLS")" == $'get:voice-minio-create-avatars-bucket:\ndelete:voice-minio-create-avatars-bucket:uid=fixture-uid\nwait:voice-minio-create-avatars-bucket:\nget:voice-minio-create-files-bucket:\ndelete:voice-minio-create-files-bucket:uid=fixture-uid\nwait:voice-minio-create-files-bucket:' ]] || fail 'completed image-drift Jobs must use their inspected UID preconditions'

# Fresh k3s Jobs use the batch-qualified key for the selector, while legacy objects used the bare key.
: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete false batch.kubernetes.io/controller-uid >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
rm -f "${FIXTURES}/voice-minio-create-files-bucket.json"
bash "$HELPER" voice-staging "$EXPECTED_IMAGE"
[[ "$(cat "$CALLS")" == $'get:voice-minio-create-avatars-bucket:\ndelete:voice-minio-create-avatars-bucket:uid=fixture-uid\nwait:voice-minio-create-avatars-bucket:\nget:voice-minio-create-files-bucket:' ]] || fail 'fresh batch-qualified selector must be safely accepted and replaced'

# Only one exact controller UID selector key is allowed; both spellings or a wrong UID fail closed.
for selector_change in both wrong_uid arbitrary arbitrary_only; do
  : >"$CALLS"
  make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete false batch.kubernetes.io/controller-uid >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
  python - "${FIXTURES}/voice-minio-create-avatars-bucket.json" "$selector_change" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
job = json.loads(path.read_text())
labels = job["spec"]["selector"]["matchLabels"]
if sys.argv[2] == "both":
    labels["controller-uid"] = "fixture-uid"
elif sys.argv[2] == "wrong_uid":
    labels["batch.kubernetes.io/controller-uid"] = "other-uid"
elif sys.argv[2] == "arbitrary_only":
    job["spec"]["selector"]["matchLabels"] = {"attacker/controller-uid": "fixture-uid"}
else:
    labels["attacker/controller-uid"] = "fixture-uid"
path.write_text(json.dumps(job))
PY
  if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail "${selector_change} selector must remain unexpected"; fi
  [[ "$(cat "$CALLS")" == 'get:voice-minio-create-avatars-bucket:' ]] || fail "${selector_change} selector must not be deleted"
done

# If a same-name replacement appears after inspection, the UID precondition rejects deletion.
: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
CURRENT_UID=replacement-uid
export CURRENT_UID
if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'replacement Job UID must not be deleted'; fi
unset CURRENT_UID
[[ "$(cat "$CALLS")" == $'get:voice-minio-create-avatars-bucket:\ndelete:voice-minio-create-avatars-bucket:uid=fixture-uid' ]] || fail 'replacement UID conflict must be enforced at deletion'

# Never delete an active job or a job with an unexpected action, even if its image is old.
: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" active >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'active stale Job must fail closed'; fi
[[ "$(cat "$CALLS")" == 'get:voice-minio-create-avatars-bucket:' ]] || fail 'active Job must not be deleted'

# Defaulted fields are accepted only at their known harmless values.
: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
python - "${FIXTURES}/voice-minio-create-avatars-bucket.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
job = json.loads(path.read_text())
job["spec"]["manualSelector"] = True
path.write_text(json.dumps(job))
PY
if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'manual selector must not be accepted as a defaulted Job'; fi
[[ "$(cat "$CALLS")" == 'get:voice-minio-create-avatars-bucket:' ]] || fail 'non-default Job selector must not be deleted'

: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
python - "${FIXTURES}/voice-minio-create-avatars-bucket.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
job = json.loads(path.read_text())
job["spec"]["template"]["spec"]["containers"][0]["resources"]["requests"] = {"cpu": "1"}
path.write_text(json.dumps(job))
PY
if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'non-empty container resources must remain unexpected'; fi
[[ "$(cat "$CALLS")" == 'get:voice-minio-create-avatars-bucket:' ]] || fail 'unexpected resource requests must not be deleted'

: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
python - "${FIXTURES}/voice-minio-create-avatars-bucket.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
job = json.loads(path.read_text())
job["spec"]["template"]["metadata"]["labels"]["attacker/controller-uid"] = "fixture-uid"
path.write_text(json.dumps(job))
PY
if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'arbitrary label with a controller-uid suffix must remain unexpected'; fi
[[ "$(cat "$CALLS")" == 'get:voice-minio-create-avatars-bucket:' ]] || fail 'unexpected selector labels must not be deleted'

: >"$CALLS"
make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete true >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'unexpected Job spec must fail closed'; fi
[[ "$(cat "$CALLS")" == 'get:voice-minio-create-avatars-bucket:' ]] || fail 'unexpected Job spec must not be deleted'

# When kubectl is installed, exercise its real raw-delete parser and HTTP body against a loopback API.
if [[ -n "$REAL_KUBECTL" ]]; then
  cat >"${TMP}/fake_kube_api.py" <<'PY'
import http.server, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
class Handler(http.server.BaseHTTPRequestHandler):
    def do_DELETE(self):
        if self.headers.get("Transfer-Encoding", "").lower() == "chunked":
            chunks = []
            while True:
                size = int(self.rfile.readline().split(b";", 1)[0], 16)
                if size == 0:
                    while self.rfile.readline().strip():
                        pass
                    break
                chunks.append(self.rfile.read(size))
                self.rfile.read(2)
            raw_body = b"".join(chunks)
        else:
            raw_body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if not raw_body:
            raise ValueError(f"kubectl sent an empty DELETE body; headers={dict(self.headers)}")
        body = json.loads(raw_body)
        expected = (root / "expected-uid").read_text()
        valid_path = self.path == "/apis/batch/v1/namespaces/voice-staging/jobs/voice-minio-create-avatars-bucket"
        valid_body = body.get("apiVersion") == "meta.k8s.io/v1" and body.get("kind") == "DeleteOptions"
        valid_uid = body.get("preconditions", {}).get("uid") == expected
        status = 200 if valid_path and valid_body and valid_uid else 409
        with (root / "requests.jsonl").open("a", encoding="utf-8") as out:
            out.write(json.dumps({"path": self.path, "body": body, "status": status}) + "\n")
        data = json.dumps({"apiVersion":"v1", "kind":"Status", "status":"Success" if status == 200 else "Failure", "reason":"Conflict" if status == 409 else "Success", "code":status}).encode()
        self.send_response(status); self.send_header("Content-Type", "application/json"); self.send_header("Content-Length", str(len(data))); self.end_headers(); self.wfile.write(data)
    def log_message(self, *args): pass
server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
(root / "api-port").write_text(str(server.server_port))
server.serve_forever()
PY
  python "${TMP}/fake_kube_api.py" "${TMP}" &
  API_PID=$!
  for _ in $(seq 1 50); do [[ -s "${TMP}/api-port" ]] && break; sleep 0.1; done
  [[ -s "${TMP}/api-port" ]] || fail 'loopback Kubernetes API did not start'
  python - "${TMP}" <<'PY'
import pathlib, sys
root = pathlib.Path(sys.argv[1])
port = (root / "api-port").read_text()
(root / "kubeconfig").write_text(f'''apiVersion: v1
kind: Config
clusters:
- name: fixture
  cluster:
    server: http://127.0.0.1:{port}
contexts:
- name: fixture
  context:
    cluster: fixture
    user: fixture
    namespace: voice-staging
current-context: fixture
users:
- name: fixture
  user: {{}}
''')
PY
  KUBECONFIG="${TMP}/kubeconfig"
  if command -v cygpath >/dev/null 2>&1; then KUBECONFIG="$(cygpath -w "$KUBECONFIG")"; fi
  export KUBECONFIG USE_REAL_KUBECTL=true
  printf '%s' fixture-uid >"${TMP}/expected-uid"
  make_job voice-minio-create-avatars-bucket "$LEGACY_IMAGE" complete >"${FIXTURES}/voice-minio-create-avatars-bucket.json"
  make_job voice-minio-create-files-bucket "$EXPECTED_IMAGE" complete >"${FIXTURES}/voice-minio-create-files-bucket.json"
  bash "$HELPER" voice-staging "$EXPECTED_IMAGE"
  : >"$CALLS"
  printf '%s' replacement-uid >"${TMP}/expected-uid"
  if bash "$HELPER" voice-staging "$EXPECTED_IMAGE" >/dev/null 2>&1; then fail 'real kubectl must propagate API UID precondition conflicts'; fi
  python - "${TMP}/requests.jsonl" <<'PY'
import json, pathlib, sys
records = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text().splitlines()]
assert len(records) == 2, f"expected successful and conflicting DELETE requests, got {records!r}"
for record in records:
    assert record["path"] == "/apis/batch/v1/namespaces/voice-staging/jobs/voice-minio-create-avatars-bucket"
    assert record["body"]["apiVersion"] == "meta.k8s.io/v1"
    assert record["body"]["kind"] == "DeleteOptions"
    assert record["body"]["preconditions"]["uid"] == "fixture-uid"
assert records[0]["status"] == 200 and records[1]["status"] == 409, f"unexpected API statuses: {records!r}"
PY
  unset USE_REAL_KUBECTL KUBECONFIG
else
  echo 'SKIP: kubectl CLI transport check unavailable; fake-kubectl lifecycle cases ran.'
fi

# The guard must run before the immutable Job manifest is applied.
guard_line="$(awk '/replace-minio-bucket-jobs\.sh/ { print NR; exit }' "$APPLY")"
manifest_line="$(awk '/deploy\/staging\/minio\.yaml/ { print NR; exit }' "$APPLY")"
[[ -n "$guard_line" && -n "$manifest_line" && "$guard_line" -lt "$manifest_line" ]] || fail 'stale Job guard must run before MinIO manifest apply'

echo 'PASS: staging MinIO bucket Jobs are replaced only for known completed image drift.'
