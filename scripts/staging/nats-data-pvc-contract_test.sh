#!/usr/bin/env bash
# Networkless regression: a data reset must not rotate mounted identities.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin"
printf '#!/usr/bin/env bash\ncat "$MARKER"\n' >"$tmp/bin/kubectl"
chmod +x "$tmp/bin/kubectl"
export PATH="$tmp/bin:$PATH"
export MARKER="$tmp/marker.json"
fail() { echo "FAIL: $1" >&2; exit 1; }
marker() { printf '{"data":{"phase":"%s","generation":"%s","previousGeneration":"r20260930a3"%s}}\n' "$1" "$2" "$3" >"$MARKER"; }
cat >"$tmp/infra.yaml" <<'YAML'
name: voice-nats-jsdata
claimName: voice-nats-jsdata
secretName: voice-nats-operator
secretName: voice-nats-hub-tls
secretName: voice-nats-bootstrap-credentials
secretName: voice-nats-service-credentials
YAML
marker active r20260930a4 ',"dataPVC":"voice-nats-jsdata-d20261004proof"'
bash "$ROOT/scripts/staging/nats-generation.sh" --render "$tmp/infra.yaml" >"$tmp/rendered"
grep -Fxq 'claimName: voice-nats-jsdata-d20261004proof' "$tmp/rendered" || fail 'normal deploy reverted selected data PVC'
for secret in operator hub-tls bootstrap-credentials service-credentials; do
  grep -Fxq "secretName: voice-nats-${secret}-r20260930a4" "$tmp/rendered" || fail 'data reset changed identity references'
done
# Environment cannot override an absent marker field or poison its value.
marker active r20260930a4 ''
NATS_DATA_PVC=voice-nats-jsdata-d20261004poison bash "$ROOT/scripts/staging/nats-generation.sh" --render "$tmp/infra.yaml" >"$tmp/rendered"
grep -Fxq 'claimName: voice-nats-jsdata-r20260930a4' "$tmp/rendered" || fail 'legacy identity-bound PVC changed'
for bad in '"postgres-data"' '"voice-nats-jsdata-d20261004/../x"' '"voice-nats-jsdata-d20261004BAD"' '""' null 4 '[]'; do
  marker active r20260930a4 ",\"dataPVC\":$bad"
  if bash "$ROOT/scripts/staging/nats-generation.sh" --check >/dev/null 2>&1; then fail 'unsafe data PVC accepted'; fi
done
marker resetting r20260930a4 ',"dataPVC":"voice-nats-jsdata-d20261004proof"'
if bash "$ROOT/scripts/staging/nats-generation.sh" --render "$tmp/infra.yaml" >"$tmp/rendered" 2>/dev/null; then fail 'maintenance fence bypassed'; fi
[[ ! -s "$tmp/rendered" ]] || fail 'fenced deploy emitted manifest'
marker active legacy ',"dataPVC":"voice-nats-jsdata-d20261004proof"'
bash "$ROOT/scripts/staging/nats-generation.sh" --render "$tmp/infra.yaml" >"$tmp/rendered"
grep -Fxq 'claimName: voice-nats-jsdata-d20261004proof' "$tmp/rendered" || fail 'legacy identity data reset ignored'
grep -Fxq 'secretName: voice-nats-operator' "$tmp/rendered" || fail 'legacy identity changed'
echo 'PASS: independent NATS data PVC contract (zero skips)'
