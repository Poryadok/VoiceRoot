#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
restore="${root}/scripts/staging/restore-nats-generation.sh"
workflow="${root}/.github/workflows/staging-deploy.yml"
real_jq="$(command -v jq || true)"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }

generation=r20260930a1
mkdir -p "${work}/bin" "${work}/present"
python3 - "${work}" "${generation}" <<'PY'
import base64
import json
from pathlib import Path
import sys

out = Path(sys.argv[1])
generation = sys.argv[2]
keys = {
    "voice-nats-operator": ["operator.jwt", "account.jwt", "system-account.jwt", "account.public", "system-account.public"],
    "voice-nats-hub-tls": ["tls.crt", "tls.key", "ca.crt"],
    "voice-nats-bootstrap-credentials": ["bootstrap.creds"],
    "voice-nats-service-credentials": [f"{name}.creds" for name in "analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice".split()],
}
items = []
for base, names in keys.items():
    data = {name: base64.b64encode(f"synthetic-credential-do-not-print-{base}-{name}".encode()).decode() for name in names}
    items.append({"apiVersion": "v1", "kind": "Secret", "metadata": {"name": f"{base}-{generation}", "namespace": "voice-staging"}, "type": "Opaque", "immutable": True, "data": data})
valid = {"apiVersion": "v1", "kind": "List", "items": items}
variants = {"valid": valid}
for label in ("wrong-namespace", "wrong-name", "missing-key", "extra-key", "not-immutable", "wrong-type", "string-data"):
    variant = json.loads(json.dumps(valid))
    first = variant["items"][0]
    if label == "wrong-namespace":
        first["metadata"]["namespace"] = "voice-prod"
    elif label == "wrong-name":
        first["metadata"]["name"] = "voice-nats-operator-r20260929"
    elif label == "missing-key":
        del first["data"]["operator.jwt"]
    elif label == "extra-key":
        first["data"]["operator.seed"] = "c2VlZA=="
    elif label == "not-immutable":
        first["immutable"] = False
    elif label == "wrong-type":
        first["type"] = "kubernetes.io/tls"
    elif label == "string-data":
        first["stringData"] = {"operator.jwt": "synthetic"}
    variants[label] = variant
for label, value in variants.items():
    (out / f"{label}.json").write_text(json.dumps(value), encoding="utf-8")
PY

# Test doubles expose only presence and operation kind. They never print a
# Secret or a restore bundle, including when validation fails.
printf '#!%s\n' "${BASH}" >"${work}/bin/kubectl"
cat >>"${work}/bin/kubectl" <<'SH'
if [[ " $* " == *' get configmap voice-nats-generation '* ]]; then
  case "${NATS_TEST_MARKER}" in
    absent) echo 'Error from server (NotFound): configmaps "voice-nats-generation" not found' >&2; exit 1 ;;
    active) printf '{"data":{"phase":"active","generation":"%s","previousGeneration":"r20260929"}}\n' "${NATS_TEST_GENERATION}"; exit 0 ;;
    rotating) printf '{"data":{"phase":"rotating","generation":"%s","previousGeneration":"r20260929"}}\n' "${NATS_TEST_GENERATION}"; exit 0 ;;
  esac
fi
if [[ " $* " == *' get secret '* ]]; then
  for arg in "$@"; do
    if [[ "$arg" == voice-nats-* ]]; then
      if [[ "${NATS_TEST_FORBID_SECRET:-}" == "$arg" ]]; then
        printf 'Error from server (Forbidden): secrets "%s" is forbidden\n' "$arg" >&2
        exit 1
      fi
      if [[ -f "${NATS_TEST_PRESENT}/${arg}" ]]; then exit 0; fi
      printf 'Error from server (NotFound): secrets "%s" not found\n' "$arg" >&2
      exit 1
    fi
  done
  exit 1
fi
if [[ " $* " == *' create '* ]]; then
  if [[ " $* " == *' --dry-run=server '* ]]; then
    printf 'create-dry-run\n' >>"${NATS_TEST_CALLS}"
    cat >"${NATS_TEST_DRY_RUN}"
  else
    printf 'create-live\n' >>"${NATS_TEST_CALLS}"
    cat >"${NATS_TEST_CREATED}"
  fi
  exit 0
fi
printf 'unexpected-kubectl\n' >>"${NATS_TEST_CALLS}"
exit 1
SH
chmod +x "${work}/bin/kubectl"

# jq is absent on some Windows development hosts. This test double parses the
# real JSON input and independently enforces the restore List contract.
printf '#!%s\n' "${BASH}" >"${work}/bin/jq"
cat >>"${work}/bin/jq" <<'SH'
exec python3 "${NATS_TEST_JQ_IMPL}" "$@"
SH
cat >"${work}/jq_impl.py" <<'PY'
import base64
import binascii
import json
from pathlib import Path
import re
import sys

args = sys.argv[1:]
query = next((value for value in args if value.startswith(".data.")), None)
if query is not None:
    field = re.fullmatch(r"\.data\.(phase|generation|previousGeneration) // empty", query)
    if field is None:
        sys.exit(2)
    try:
        value = json.load(sys.stdin)["data"][field.group(1)]
    except (KeyError, ValueError):
        sys.exit(1)
    if not isinstance(value, str) or not value:
        sys.exit(1)
    print(value)
    sys.exit(0)
path = Path(args[-1]) if args else None
if path is None or not path.is_file():
    sys.exit(2)
try:
    bundle = json.loads(path.read_text(encoding="utf-8"))
    generation = __import__("os").environ["NATS_TEST_GENERATION"]
    expected = {
        "voice-nats-operator": {"operator.jwt", "account.jwt", "system-account.jwt", "account.public", "system-account.public"},
        "voice-nats-hub-tls": {"tls.crt", "tls.key", "ca.crt"},
        "voice-nats-bootstrap-credentials": {"bootstrap.creds"},
        "voice-nats-service-credentials": {f"{name}.creds" for name in "analytics auth bot chat file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice".split()},
    }
    assert bundle["apiVersion"] == "v1" and bundle["kind"] == "List" and len(bundle["items"]) == 4
    found = set()
    for item in bundle["items"]:
        base = item["metadata"]["name"].removesuffix("-" + generation)
        assert base in expected and base not in found
        assert item["metadata"] == {"name": base + "-" + generation, "namespace": "voice-staging"}
        assert item["apiVersion"] == "v1" and item["kind"] == "Secret" and item["type"] == "Opaque"
        assert item.get("immutable") is True and "stringData" not in item
        assert set(item["data"]) == expected[base]
        for value in item["data"].values():
            assert value and base64.b64encode(base64.b64decode(value, validate=True)).decode() == value
        found.add(base)
    assert found == set(expected)
except (AssertionError, KeyError, ValueError, TypeError, binascii.Error, OSError):
    sys.exit(1)
sys.exit(0)
PY
chmod +x "${work}/bin/jq"

export PATH="${work}/bin:${PATH}"
export NATS_TEST_GENERATION="${generation}"
export NATS_TEST_PRESENT="${work}/present"
export NATS_TEST_CALLS="${work}/calls"
export NATS_TEST_DRY_RUN="${work}/dry-run.json"
export NATS_TEST_CREATED="${work}/created.json"
export NATS_TEST_JQ_IMPL="${work}/jq_impl.py"

bundle_env() {
  python3 - "$1" <<'PY'
import base64
import gzip
from pathlib import Path
import sys
print(base64.b64encode(gzip.compress(Path(sys.argv[1]).read_bytes())).decode())
PY
}
run_restore() {
  : >"${NATS_TEST_CALLS}"
  rm -f "${NATS_TEST_DRY_RUN}" "${NATS_TEST_CREATED}"
  "${BASH}" "${restore}" >"${work}/output" 2>&1
}
no_create() {
  ! grep -Eq '^(create-(dry-run|live)|unexpected-kubectl)$' "${NATS_TEST_CALLS}" || fail "$1 invoked a kubectl mutation"
  [[ ! -e "${NATS_TEST_CREATED}" ]] || fail "$1 created Secrets"
}
no_leak() {
  ! grep -Eq 'synthetic-credential-do-not-print|c3ludGhldGlj' "${work}/output" || fail "$1 leaked credential data"
  if [[ -n "${STAGING_NATS_ROTATION_SECRETS_B64:-}" ]]; then
    ! grep -Fq "${STAGING_NATS_ROTATION_SECRETS_B64}" "${work}/output" || fail "$1 leaked encoded bundle"
  fi
}
reject() {
  local label="$1"
  if run_restore; then fail "$label unexpectedly succeeded"; fi
  no_create "$label"
  no_leak "$label"
}

export NATS_TEST_MARKER=active
export STAGING_NATS_ROTATION_SECRETS_B64="$(bundle_env "${work}/valid.json")"
run_restore || fail 'active generation restore'
printf 'create-dry-run\ncreate-live\n' | cmp -s - "${NATS_TEST_CALLS}" || fail 'active restore must dry-run before its only live create'
python3 - "${work}/valid.json" "${NATS_TEST_CREATED}" <<'PY' || fail 'created Secret set differs from validated bundle'
import json
import sys
assert json.load(open(sys.argv[1], encoding="utf-8")) == json.load(open(sys.argv[2], encoding="utf-8"))
PY
no_leak 'active restore'

for name in voice-nats-operator voice-nats-hub-tls voice-nats-bootstrap-credentials voice-nats-service-credentials; do
  touch "${NATS_TEST_PRESENT}/${name}-${generation}"
done
unset STAGING_NATS_ROTATION_SECRETS_B64
run_restore || fail 'complete generation must skip without bundle env'
no_create 'complete generation'

rm "${NATS_TEST_PRESENT}/voice-nats-operator-${generation}"
export STAGING_NATS_ROTATION_SECRETS_B64="$(bundle_env "${work}/valid.json")"
reject 'partial generation'
rm "${NATS_TEST_PRESENT}"/*

export NATS_TEST_FORBID_SECRET="voice-nats-operator-${generation}"
reject 'Forbidden Secret read'
unset NATS_TEST_FORBID_SECRET

for variant in wrong-namespace wrong-name missing-key extra-key not-immutable wrong-type string-data; do
  export STAGING_NATS_ROTATION_SECRETS_B64="$(bundle_env "${work}/${variant}.json")"
  reject "$variant"
done
export STAGING_NATS_ROTATION_SECRETS_B64='not-gzip-base64'
reject 'malformed encoding'
unset STAGING_NATS_ROTATION_SECRETS_B64
reject 'missing bundle environment'

NATS_TEST_MARKER=rotating reject 'rotating marker'
NATS_TEST_MARKER=absent run_restore || fail 'legacy marker must skip versioned restore'
no_create 'legacy marker'

# Explicit restore helper positives above remain intact. Ordinary versions
# retain captured Secret authority instead of replaying fixed/versioned restore.
! grep -Eq 'run: bash scripts/staging/restore-nats-(secrets|generation).sh' "${workflow}" || fail 'ordinary workflow must not replace retained NATS Secret authority'
grep -Fq -- 'ROLLOUT_ACTION: --bridge-prepare' "${workflow}" || fail 'ordinary workflow must capture current generation authority'
grep -Eq 'state.*provenance.*capture_inputs' "${root}/scripts/staging/nats-rollout-preservation/transaction.py" || fail 'root prepare must capture current Secret/config authority'
grep -Fq 'revalidate_inputs' "${root}/scripts/staging/nats-rollout-preservation/transaction.py" || fail 'root operation must revalidate captured Secret/config authority'

if [[ -n "$real_jq" ]]; then
  # Linux CI also exercises the production jq filter itself; the portable
  # Python parser above keeps the Windows contract test runnable without jq.
  export NATS_TEST_REAL_JQ="$real_jq"
  printf '#!%s\n' "${BASH}" >"${work}/bin/jq"
  cat >>"${work}/bin/jq" <<'SH'
exec "${NATS_TEST_REAL_JQ}" "$@"
SH
  NATS_TEST_MARKER=active
  export STAGING_NATS_ROTATION_SECRETS_B64="$(bundle_env "${work}/valid.json")"
  run_restore || fail 'real jq rejected the valid generation bundle'
  printf 'create-dry-run\ncreate-live\n' | cmp -s - "${NATS_TEST_CALLS}" || fail 'real jq path did not dry-run before create'
  no_leak 'real jq valid bundle'
  for variant in wrong-namespace wrong-name missing-key extra-key not-immutable wrong-type string-data; do
    export STAGING_NATS_ROTATION_SECRETS_B64="$(bundle_env "${work}/${variant}.json")"
    reject "real jq ${variant}"
  done
fi

echo 'staging NATS generation restore contract: OK'
