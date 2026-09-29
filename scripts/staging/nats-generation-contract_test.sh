#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PACKAGE="${ROOT}/scripts/staging/prepare-nats-rotation-bundle.py"
GENERATION="${ROOT}/scripts/staging/nats-generation.sh"
APPLY="${ROOT}/scripts/staging/render-and-apply.sh"
ACL_PROOF_SHA="$(sha256sum "${ROOT}/deploy/nats/acl-intent.yaml" | cut -d' ' -f1)"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
expect_failure() {
  local label="$1"
  shift
  if "$@" >"${tmp}/command.out" 2>"${tmp}/command.err"; then
    fail "${label} must fail"
  fi
}

# Deliberately synthetic values: this test never reads a .local bundle or a live Secret.
python3 - "${tmp}/source.json" <<'PY'
import base64
import json
import sys

keys = {
    "voice-nats-operator": ["operator.jwt", "account.jwt", "system-account.jwt", "account.public", "system-account.public"],
    "voice-nats-hub-tls": ["tls.crt", "tls.key", "ca.crt"],
    "voice-nats-bootstrap-credentials": ["bootstrap.creds"],
    "voice-nats-service-credentials": [f"{service}.creds" for service in "analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice".split()],
}
items = []
for name, names in keys.items():
    data = {key: base64.b64encode(f"synthetic-{name}-{key}".encode()).decode() for key in names}
    items.append({"apiVersion": "v1", "kind": "Secret", "metadata": {"name": name, "namespace": "voice-staging"}, "type": "Opaque", "data": data})
with open(sys.argv[1], "w", encoding="utf-8") as output:
    json.dump({"apiVersion": "v1", "kind": "List", "items": items}, output)
PY

generation=r20260930a1
python3 "${PACKAGE}" "${generation}" "${tmp}/source.json" "${tmp}/versioned.json" || fail 'valid synthetic bundle packaging'
python3 - "${tmp}/source.json" "${tmp}/versioned.json" "${generation}" <<'PY' || fail 'versioned bundle contract'
import json
import sys

source, bundle = [json.load(open(path, encoding="utf-8")) for path in sys.argv[1:3]]
generation = sys.argv[3]
assert bundle.get("kind") == "List" and len(bundle.get("items", [])) == 4
old = {item["metadata"]["name"]: item for item in source["items"]}
new = {item["metadata"]["name"]: item for item in bundle["items"]}
assert len(new) == 4
assert set(new) == {f"{name}-{generation}" for name in old}
for name, item in old.items():
    versioned = new[f"{name}-{generation}"]
    assert versioned["metadata"]["namespace"] == "voice-staging"
    assert versioned["type"] == "Opaque"
    assert versioned.get("immutable") is True
    assert versioned.get("data") == item["data"]
    assert "stringData" not in versioned
    assert not any("seed" in key.lower() for key in versioned["data"])
    assert "seed" not in json.dumps(versioned).lower()
PY

cp "${tmp}/versioned.json" "${tmp}/collision.before"
expect_failure 'existing output collision' python3 "${PACKAGE}" "${generation}" "${tmp}/source.json" "${tmp}/versioned.json"
cmp -s "${tmp}/collision.before" "${tmp}/versioned.json" || fail 'output collision altered the existing bundle'
if [[ "$(uname -s)" == Linux ]]; then
  python3 - "${tmp}" "${tmp}/versioned.json" <<'PY' || fail 'Linux bundle permissions'
import os
import stat
import sys

parent, output = sys.argv[1:]
assert stat.S_IMODE(os.stat(parent).st_mode) == 0o700
assert stat.S_IMODE(os.stat(output).st_mode) == 0o600
PY
fi
for invalid in r2026093 r20260930A r20260930_bad r20260930abcdefghi r20260930/../x legacy; do
  expect_failure "malformed generation ${invalid}" python3 "${PACKAGE}" "${invalid}" "${tmp}/source.json" "${tmp}/invalid.json"
  [ ! -e "${tmp}/invalid.json" ] || fail 'invalid generation created an output file'
done

python3 - "${tmp}/source.json" "${tmp}" <<'PY'
import json
import pathlib
import sys

source = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
out = pathlib.Path(sys.argv[2])
variants = {}
partial = json.loads(json.dumps(source))
partial["items"].pop()
variants["partial"] = partial
duplicate = json.loads(json.dumps(source))
duplicate["items"][1]["metadata"]["name"] = duplicate["items"][0]["metadata"]["name"]
variants["duplicate"] = duplicate
wrong_namespace = json.loads(json.dumps(source))
wrong_namespace["items"][0]["metadata"]["namespace"] = "voice-prod"
variants["wrong-namespace"] = wrong_namespace
unknown_name = json.loads(json.dumps(source))
unknown_name["items"][0]["metadata"]["name"] = "voice-nats-unknown"
variants["unknown-name"] = unknown_name
wrong_type = json.loads(json.dumps(source))
wrong_type["items"][0]["type"] = "kubernetes.io/tls"
variants["wrong-type"] = wrong_type
missing_key = json.loads(json.dumps(source))
missing_key["items"][0]["data"].pop("operator.jwt")
variants["missing-key"] = missing_key
extra_key = json.loads(json.dumps(source))
extra_key["items"][0]["data"]["unapproved.jwt"] = "c3ludGhldGlj"
variants["extra-key"] = extra_key
string_data = json.loads(json.dumps(source))
string_data["items"][0]["stringData"] = {"operator.jwt": "synthetic"}
variants["string-data"] = string_data
unexpected = json.loads(json.dumps(source))
unexpected["items"][0]["data"]["operator.seed"] = "c2VlZA=="
variants["seed"] = unexpected
bad_data = json.loads(json.dumps(source))
bad_data["items"][0]["data"]["operator.jwt"] = "not base64!"
variants["malformed-data"] = bad_data
for name, value in variants.items():
    (out / f"{name}.json").write_text(json.dumps(value), encoding="utf-8")
PY
for variant in partial duplicate wrong-namespace unknown-name wrong-type missing-key extra-key string-data seed malformed-data; do
  expect_failure "${variant} source bundle" python3 "${PACKAGE}" "${generation}" "${tmp}/${variant}.json" "${tmp}/${variant}-out.json"
  [ ! -e "${tmp}/${variant}-out.json" ] || fail "${variant} source created an output file"
done

mkdir -p "${tmp}/bin"
printf '#!%s\n' "${BASH}" >"${tmp}/bin/kubectl"
cat >>"${tmp}/bin/kubectl" <<'SH'
printf 'kubectl %s\n' "$*" >>"${NATS_TEST_CALLS}"
if [[ " $* " == *' get configmap voice-nats-generation '* ]]; then
  if [ "${NATS_TEST_MARKER:-}" = absent ]; then
    echo 'Error from server (NotFound): configmaps "voice-nats-generation" not found' >&2
    exit 1
  fi
  cat "${NATS_TEST_MARKER}"
  exit 0
fi
if [ -n "${NATS_TEST_EXPECT_GENERATION:-}" ] && [[ " $* " == *' get job '* ]]; then
  printf '{"metadata":{"annotations":{"voice.io/nats-generation":"%s"}}}\n' "${NATS_TEST_EXPECT_GENERATION}"
  exit 0
fi
if [ -n "${NATS_TEST_CAPTURE_APPLY:-}" ] && [[ " $* " == *' apply -f - '* ]]; then
  cat >>"${NATS_TEST_CAPTURE_APPLY}"
  printf '\n---\n' >>"${NATS_TEST_CAPTURE_APPLY}"
fi
exit 0
SH
chmod +x "${tmp}/bin/kubectl"
printf '#!%s\n' "${BASH}" >"${tmp}/bin/jq"
cat >>"${tmp}/bin/jq" <<'SH'
# The Windows test host need not have jq; emulate only the marker reads used
# by nats-generation.sh, while parsing the actual JSON emitted by kubectl.
python3 -c '
import json
import re
import sys
query = sys.argv[-1]
match = re.fullmatch(r"\.data\.(phase|generation|previousGeneration) // empty", query)
job_query = ".metadata.annotations[\"voice.io/nats-generation\"] // empty"
if match is None and query != job_query:
    sys.exit(2)
try:
    document = json.load(sys.stdin)
    value = (document["data"][match.group(1)] if match is not None
             else document["metadata"]["annotations"]["voice.io/nats-generation"])
except (KeyError, ValueError):
    sys.exit(1)
if not isinstance(value, str) or not value:
    sys.exit(1)
print(value)
' "$@"
SH
chmod +x "${tmp}/bin/jq"
printf '{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"voice-nats-generation"},"data":{"phase":"active","generation":"%s","previousGeneration":"r20260929"}}\n' "${generation}" >"${tmp}/active.json"
printf '{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"voice-nats-generation"},"data":{"phase":"active","generation":"legacy","previousGeneration":"%s"}}\n' "${generation}" >"${tmp}/rollback.json"
printf '{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"voice-nats-generation"},"data":{"phase":"rotating","generation":"%s","previousGeneration":"r20260929"}}\n' "${generation}" >"${tmp}/rotating.json"
printf '{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"voice-nats-generation"},"data":{"phase":"active","generation":"../invalid","previousGeneration":"r20260929"}}\n' >"${tmp}/bad-marker.json"

export NATS_TEST_CALLS="${tmp}/calls.log"
export NATS_TEST_MARKER="${tmp}/active.json"
export PATH="${tmp}/bin:${PATH}"
: >"${NATS_TEST_CALLS}"
for manifest in deploy/staging/infra.yaml deploy/staging/services.yaml deploy/templates/nats-realtime-bootstrap.yaml deploy/templates/nats-realtime-permissions-preflight.yaml; do
  bash "${GENERATION}" --render "${ROOT}/${manifest}" >"${tmp}/rendered.yaml" || fail "active render ${manifest}"
  ! grep -Eq 'secretName: voice-nats-(operator|hub-tls|bootstrap-credentials|service-credentials)([^-]|$)' "${tmp}/rendered.yaml" || fail "fixed Secret reference in active ${manifest}"
  case "$manifest" in
    deploy/staging/infra.yaml)
      grep -Fq "secretName: voice-nats-operator-${generation}" "${tmp}/rendered.yaml" || fail 'active hub does not mount generation operator Secret'
      grep -Fq "secretName: voice-nats-hub-tls-${generation}" "${tmp}/rendered.yaml" || fail 'active hub does not mount generation TLS Secret'
      grep -Fq "claimName: voice-nats-jsdata-${generation}" "${tmp}/rendered.yaml" || fail 'active infra does not mount the generation PVC'
      grep -Fq "name: voice-nats-jsdata-${generation}" "${tmp}/rendered.yaml" || fail 'active infra does not define the generation PVC'
      ! grep -Eq 'claimName: voice-nats-jsdata([^a-z0-9-]|$)' "${tmp}/rendered.yaml" || fail 'active infra mounts the legacy PVC'
      ;;
    deploy/staging/services.yaml)
      grep -Fq "secretName: voice-nats-service-credentials-${generation}" "${tmp}/rendered.yaml" || fail 'active app does not mount generation service Secret'
      grep -Fq "secretName: voice-nats-hub-tls-${generation}" "${tmp}/rendered.yaml" || fail 'active leaf does not mount generation TLS Secret'
      ;;
    deploy/templates/nats-realtime-bootstrap.yaml)
      grep -Fq "secretName: voice-nats-bootstrap-credentials-${generation}" "${tmp}/rendered.yaml" || fail "active ${manifest} does not mount generation bootstrap Secret"
      ;;
    deploy/templates/nats-realtime-permissions-preflight.yaml)
      grep -Fq "secretName: voice-nats-service-credentials-${generation}" "${tmp}/rendered.yaml" || fail 'active preflight does not mount generation service Secret'
      grep -Fq "secretName: voice-nats-hub-tls-${generation}" "${tmp}/rendered.yaml" || fail 'active preflight does not mount generation TLS Secret'
      ;;
  esac
done

for manifest in deploy/staging/infra.yaml deploy/staging/services.yaml deploy/templates/nats-realtime-bootstrap.yaml; do
  NATS_TEST_MARKER=absent bash "${GENERATION}" --render "${ROOT}/${manifest}" >"${tmp}/legacy.yaml" || fail "legacy render ${manifest}"
  cmp -s "${ROOT}/${manifest}" "${tmp}/legacy.yaml" || fail "absent marker changed legacy ${manifest} references"
  NATS_TEST_MARKER="${tmp}/rollback.json" bash "${GENERATION}" --render "${ROOT}/${manifest}" >"${tmp}/rollback.yaml" || fail "rollback legacy render ${manifest}"
  cmp -s "${ROOT}/${manifest}" "${tmp}/rollback.yaml" || fail "rollback marker changed legacy ${manifest} references"
done
NATS_TEST_MARKER="${tmp}/rollback.json" bash "${GENERATION}" --check || fail 'active legacy rollback marker must pass generation preflight'

# A normal full deploy after rollback must keep the already selected PVC hub
# Service selector. The real filter is exercised on the rollback render, and
# apply-infra must select its preserve mode before piping infra into it.
NATS_TEST_MARKER="${tmp}/rollback.json" bash "${GENERATION}" --render "${ROOT}/deploy/staging/infra.yaml" | \
  VOICE_NATS_PRESERVE_SERVICE_SELECTOR=true "${BASH}" "${ROOT}/scripts/staging/filter-staging-infra-source-nats.sh" >"${tmp}/rollback-filtered.yaml" || fail 'rollback infra filter'
python3 - "${tmp}/rollback-filtered.yaml" <<'PY' || fail 'rollback full infra must omit the NATS Service while retaining its PVC hub'
from pathlib import Path
import re
import sys

documents = re.split(r"(?=^apiVersion:)", Path(sys.argv[1]).read_text(encoding="utf-8"), flags=re.M)
assert not any(re.search(r"^kind: Service$", document, re.M) and re.search(r"^  name: voice-nats$", document, re.M) for document in documents)
assert any(re.search(r"^kind: Deployment$", document, re.M) and re.search(r"^  name: voice-nats-pvc-candidate$", document, re.M) for document in documents)
PY
python3 - "${ROOT}/scripts/staging/apply-infra.sh" <<'PY' || fail 'full infra does not wire rollback marker to Service preservation before apply'
from pathlib import Path
import sys

source = Path(sys.argv[1]).read_text(encoding="utf-8")
marker = source.index('if [ "${NATS_MARKER_PRESENT}" = true ]; then')
preserve = source.index('VOICE_NATS_PRESERVE_SERVICE_SELECTOR=true', marker)
filtered = source.index('filter-staging-infra-source-nats.sh', preserve)
assert marker < preserve < filtered
PY
NATS_TEST_MARKER="${tmp}/rotating.json" expect_failure 'rotating marker render' bash "${GENERATION}" --render "${ROOT}/deploy/staging/infra.yaml"
NATS_TEST_MARKER="${tmp}/bad-marker.json" expect_failure 'malformed active marker' bash "${GENERATION}" --check

grep -Fq 'nats_generation_render' "${ROOT}/scripts/staging/apply-infra.sh" || fail 'full infra path bypasses generation rendering'
grep -Fq 'nats_generation_render' "${ROOT}/scripts/staging/apply-app-manifests.sh" || fail 'app-only path bypasses generation rendering'

# The full and app-only entrypoints must evaluate the marker before calling
# any child apply/preflight script or kubectl mutation.
printf '#!%s\n' "${BASH}" >"${tmp}/bin/bash"
cat >>"${tmp}/bin/bash" <<'SH'
if [[ "${1:-}" == */nats-generation.sh ]]; then
  if [[ "${2:-}" == --check ]]; then
    printf 'generation-check\n' >>"${NATS_TEST_CALLS}"
  fi
  exec "${NATS_TEST_REAL_BASH}" "$@"
fi
printf 'child-script %s\n' "$*" >>"${NATS_TEST_CALLS}"
exit 0
SH
chmod +x "${tmp}/bin/bash"
export NATS_TEST_REAL_BASH="${BASH}"

# Exercise the real app manifest entrypoint with harmless kubectl calls. The
# captured apply stream must carry the same active generation as direct render.
: >"${NATS_TEST_CALLS}"
export NATS_TEST_CAPTURE_APPLY="${tmp}/app-applies.yaml"
export NATS_TEST_EXPECT_GENERATION="${generation}"
: >"${NATS_TEST_CAPTURE_APPLY}"
NATS_TEST_MARKER="${tmp}/active.json" VOICE_IMAGE_REGISTRY=example.invalid/voice VOICE_IMAGE_TAG=test \
  VOICE_NATS_ACL_PROOF_SHA="${ACL_PROOF_SHA}" VOICE_NATS_ACL_PROOF_GENERATION="${generation}" \
  "${BASH}" "${ROOT}/scripts/staging/apply-app-manifests.sh" >"${tmp}/app.out" 2>"${tmp}/app.err" || {
    sed -n '1,12p' "${tmp}/app.err" >&2
    fail 'app-only manifest entrypoint with active generation'
  }
grep -Fq "secretName: voice-nats-service-credentials-${generation}" "${NATS_TEST_CAPTURE_APPLY}" || fail 'app-only apply stream reverted service credentials'
grep -Fq "secretName: voice-nats-hub-tls-${generation}" "${NATS_TEST_CAPTURE_APPLY}" || fail 'app-only apply stream reverted hub TLS'
! grep -Eq 'secretName: voice-nats-(service-credentials|hub-tls)([^-]|$)' "${NATS_TEST_CAPTURE_APPLY}" || fail 'app-only apply stream contains fixed NATS Secret references'

: >"${NATS_TEST_CAPTURE_APPLY}"
NATS_TEST_MARKER="${tmp}/rollback.json" NATS_TEST_EXPECT_GENERATION=legacy VOICE_IMAGE_REGISTRY=example.invalid/voice VOICE_IMAGE_TAG=test \
  VOICE_NATS_ACL_PROOF_SHA="${ACL_PROOF_SHA}" VOICE_NATS_ACL_PROOF_GENERATION=legacy \
  "${BASH}" "${ROOT}/scripts/staging/apply-app-manifests.sh" >"${tmp}/rollback-app.out" 2>"${tmp}/rollback-app.err" || {
    sed -n '1,12p' "${tmp}/rollback-app.err" >&2
    fail 'app-only entrypoint after rollback to legacy generation'
  }
grep -Fq 'secretName: voice-nats-service-credentials,' "${NATS_TEST_CAPTURE_APPLY}" || fail 'rollback app-only apply did not retain legacy credentials'
grep -Fq 'secretName: voice-nats-hub-tls,' "${NATS_TEST_CAPTURE_APPLY}" || fail 'rollback app-only apply did not retain legacy TLS'
! grep -Fq "voice-nats-service-credentials-${generation}" "${NATS_TEST_CAPTURE_APPLY}" || fail 'rollback app-only apply selected superseded credentials'
unset NATS_TEST_CAPTURE_APPLY
unset NATS_TEST_EXPECT_GENERATION

for mode in full app-only; do
  : >"${NATS_TEST_CALLS}"
  NATS_TEST_MARKER="${tmp}/rotating.json" DEPLOY_MODE="$mode" VOICE_IMAGE_TAG=test "${BASH}" "${APPLY}" >"${tmp}/apply.out" 2>"${tmp}/apply.err" && fail "${mode} deploy accepted rotating marker"
  grep -Fqx generation-check "${NATS_TEST_CALLS}" || fail "${mode} deploy omitted generation preflight"
  ! grep -Eq '^child-script |^kubectl( [^ ]+)*( apply| create| delete| patch| scale)( |$)' "${NATS_TEST_CALLS}" || fail "${mode} deploy mutated resources while rotating"
done

! grep -Rq 'nats-generation.sh\|voice-nats-generation' "${ROOT}/scripts/prod" "${ROOT}/deploy/prod" || fail 'staging generation selection leaked into production'
python3 - "${ROOT}/.github/workflows/staging-deploy.yml" <<'PY' || fail 'main staging deploy job must use the trusted staging runner labels'
from pathlib import Path
import re
import sys

workflow = Path(sys.argv[1]).read_text(encoding="utf-8")
match = re.search(r"(?ms)^  deploy:\n(.*?)(?=^  [a-z][a-z0-9-]*:\n|\Z)", workflow)
assert match is not None
assert re.search(r"(?m)^    runs-on: \[self-hosted, Linux, X64, voice-staging\]$", match.group(1))
PY
echo 'staging NATS generation contract: OK'
