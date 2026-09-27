#!/usr/bin/env bash
# Verify the full deploy Secret path preserves live keys and fails before mutation.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
export TEST_STATE="$TMP"

python3 - "$ROOT" "$TMP" <<'PY'
import base64
import importlib.util
import json
import pathlib
import sys

root, directory = sys.argv[1:]
spec = importlib.util.spec_from_file_location("check", pathlib.Path(root) / "scripts/staging/check-resend-key.py")
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
data = {key: base64.b64encode(("live-" + key).encode()).decode() for key in module.REQUIRED_KEYS}
data["EXTRA_KEY"] = base64.b64encode(b"preserve-me").decode()
live = {"kind": "Secret", "metadata": {"name": "voice-app-secrets", "namespace": "voice-staging", "resourceVersion": "42"}, "data": data}
upload = {"kind": "Secret", "metadata": {"name": "voice-app-secrets", "namespace": "voice-staging"}, "stringData": {"AUTH_RESEND_API_KEY": "replacement-mail-key"}}
pathlib.Path(directory, "live.json").write_text(json.dumps(live))
pathlib.Path(directory, "upload.json").write_text(json.dumps(upload))
PY

cat >"$TMP/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[ "${TEST_FORBID_KUBECTL:-}" != 1 ] || { echo 'offline check called kubectl' >&2; exit 1; }
case "$1 $2" in
  'get secret')
    if [ -s "$TEST_STATE/live.json" ]; then cat "$TEST_STATE/live.json"; fi
    ;;
  'create --dry-run=client') cat ;;
  'create -f') cat >"$TEST_STATE/created.json" ;;
  'patch secret')
    [ "$3" = voice-app-secrets ]
    [ "$4" = -n ] && [ "$5" = voice-staging ]
    [ "$6" = --type=json ] && [ "$7" = --patch-file ]
    cp "$8" "$TEST_STATE/patch.json"
    ;;
  *) echo "unexpected kubectl call: $1 $2" >&2; exit 1 ;;
esac
EOF
chmod +x "$TMP/kubectl"
export PATH="$TMP:$PATH"
export STAGING_APP_SECRETS_YAML_B64="$(base64 -w0 "$TMP/upload.json")"

TEST_FORBID_KUBECTL=1 STAGING_SECRET_OFFLINE_PARSE=1 \
  bash "$ROOT/scripts/staging/preflight-resend-key.sh" >"$TMP/offline.out"
grep -Fq 'effective completeness requires live Secret preflight' "$TMP/offline.out" || {
  echo 'offline check claimed effective completeness' >&2
  exit 1
}

bash "$ROOT/scripts/staging/preflight-resend-key.sh" >"$TMP/preflight.out"
bash "$ROOT/scripts/staging/ensure-app-secrets.sh" >"$TMP/ensure.out"
python3 - "$TMP" <<'PY'
import base64
import json
import pathlib
import sys

directory = pathlib.Path(sys.argv[1])
patch = json.loads((directory / "patch.json").read_text())
assert patch == [
    {"op": "test", "path": "/metadata/resourceVersion", "value": "42"},
    {"op": "add", "path": "/data/AUTH_RESEND_API_KEY", "value": base64.b64encode(b"replacement-mail-key").decode()},
]
for output in ("preflight.out", "ensure.out"):
    text = (directory / output).read_text()
    assert "replacement-mail-key" not in text and "preserve-me" not in text
PY

python3 - "$TMP/upload.json" <<'PY'
import json
import pathlib
import sys
path = pathlib.Path(sys.argv[1])
upload = json.loads(path.read_text())
upload["stringData"]["AUTH_RESEND_API_KEY"] = ""
path.write_text(json.dumps(upload))
PY
export STAGING_APP_SECRETS_YAML_B64="$(base64 -w0 "$TMP/upload.json")"
rm "$TMP/patch.json"
if bash "$ROOT/scripts/staging/preflight-resend-key.sh" >"$TMP/invalid.out" 2>&1; then
  echo 'blank uploaded value unexpectedly passed preflight' >&2
  exit 1
fi
[ ! -e "$TMP/patch.json" ] || { echo 'invalid upload reached patch' >&2; exit 1; }

# A first install may create the Secret only when the upload is complete.
python3 - "$ROOT" "$TMP" <<'PY'
import importlib.util
import json
import pathlib
import sys
root, directory = sys.argv[1:]
spec = importlib.util.spec_from_file_location("check", pathlib.Path(root) / "scripts/staging/check-resend-key.py")
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
upload = {"kind": "Secret", "metadata": {"name": "voice-app-secrets", "namespace": "voice-staging"}, "stringData": {key: "first-install-" + key for key in module.REQUIRED_KEYS}}
pathlib.Path(directory, "upload.json").write_text(json.dumps(upload))
PY
: >"$TMP/live.json"
export STAGING_APP_SECRETS_YAML_B64="$(base64 -w0 "$TMP/upload.json")"
bash "$ROOT/scripts/staging/preflight-resend-key.sh" >"$TMP/first-preflight.out"
bash "$ROOT/scripts/staging/ensure-app-secrets.sh" >"$TMP/first-ensure.out"
[ -s "$TMP/created.json" ] || { echo 'complete first upload did not create Secret' >&2; exit 1; }
[ ! -e "$TMP/patch.json" ] || { echo 'first install unexpectedly patched Secret' >&2; exit 1; }

echo 'staging app Secret merge tests passed.'
