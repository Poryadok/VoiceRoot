#!/usr/bin/env bash
# Staging smoke: DM send → X-Request-Id → Loki chain Gateway→…→ws_fanout.
# Spec: docs/TESTING.md § Debug by request_id, docs/features/observability.md OBS-02.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"

BASE="${VOICE_STAGING_URL:-https://${VOICE_GATEWAY_INGRESS_HOST}}"
BASE="${BASE%/}"
APP_NS="${VOICE_STAGING_NAMESPACE:-voice-staging}"
OBS_NS="${VOICE_OBSERVABILITY_NAMESPACE:-voice-observability}"
LOKI_LOCAL="${LOKI_LOCAL_URL:-http://127.0.0.1:3100}"
PASSWORD="${STAGING_SMOKE_PASSWORD:-VoiceQaTest1!}"
POLL_SECONDS="${REQUEST_ID_SMOKE_POLL_SECONDS:-90}"
WS_URL="${VOICE_STAGING_WS_URL:-${BASE/https/wss}/ws}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd kubectl
need_cmd python3
need_cmd websocat

echo "request_id smoke: gateway ${BASE}"

not_ready="$(kubectl get pods -n "${OBS_NS}" --no-headers 2>/dev/null | awk '$3!="Running" && $3!="Completed" {print}' || true)"
if [ -n "${not_ready}" ]; then
  echo "observability pods not Running/Completed:" >&2
  echo "${not_ready}" >&2
  exit 1
fi

kubectl port-forward -n "${OBS_NS}" "svc/loki" 3100:3100 >/tmp/voice-loki-pf.log 2>&1 &
PF_LOKI=$!
cleanup() {
  kill "${PF_LOKI}" 2>/dev/null || true
  if [ -n "${WS_PID:-}" ]; then
    kill "${WS_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

for attempt in 1 2 3 4 5 6 7 8 9 10; do
  if curl -sS -o /dev/null "${LOKI_LOCAL}/ready" 2>/dev/null; then
    break
  fi
  sleep 2
done
if ! curl -sS -o /dev/null "${LOKI_LOCAL}/ready" 2>/dev/null; then
  echo "Loki not ready at ${LOKI_LOCAL}" >&2
  exit 1
fi

n="$(date +%s%N)"
email_a="reqid-a-${n}@voice-qa.test"
email_b="reqid-b-${n}@voice-qa.test"

register() {
  local email="$1"
  local body code
  body="$(curl -sS -w '\n%{http_code}' -X POST "${BASE}/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${email}\",\"password\":\"${PASSWORD}\",\"device_info_json\":\"{}\"}")"
  code="$(echo "${body}" | tail -n1)"
  body="$(echo "${body}" | sed '$d')"
  if [ "${code}" != "200" ]; then
    echo "register ${email} failed: HTTP ${code} body=${body}" >&2
    exit 1
  fi
  echo "${body}"
}

echo "registering ${email_a} / ${email_b}"
sess_a="$(register "${email_a}")"
sess_b="$(register "${email_b}")"

token_a="$(echo "${sess_a}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session"]["access_token"])')"
token_b="$(echo "${sess_b}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session"]["access_token"])')"
profile_b="$(echo "${sess_b}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session"]["profile_id"])')"

# Keep a WS open for B so realtime can emit ws_fanout for the DM.
websocat -k -H "Authorization: Bearer ${token_b}" "${WS_URL}" >/tmp/voice-reqid-ws.log 2>&1 &
WS_PID=$!
sleep 2

dm_body="$(curl -sS -w '\n%{http_code}' -X POST "${BASE}/api/v1/chats/dm" \
  -H "Authorization: Bearer ${token_a}" \
  -H 'Content-Type: application/json' \
  -d "{\"other_profile_id\":\"${profile_b}\"}")"
dm_code="$(echo "${dm_body}" | tail -n1)"
dm_json="$(echo "${dm_body}" | sed '$d')"
if [ "${dm_code}" != "200" ]; then
  echo "create DM failed: HTTP ${dm_code} body=${dm_json}" >&2
  exit 1
fi
chat_id="$(echo "${dm_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["chat"]["id"])')"

send_tmp="$(mktemp)"
send_headers="$(mktemp)"
send_code="$(curl -sS -D "${send_headers}" -o "${send_tmp}" -w "%{http_code}" \
  -X POST "${BASE}/api/v1/messages/send" \
  -H "Authorization: Bearer ${token_a}" \
  -H 'Content-Type: application/json' \
  -d "{\"chat\":{\"id\":\"${chat_id}\"},\"content\":\"request-id-smoke-${n}\"}" || echo "000")"
send_body="$(tr -d '\r' < "${send_tmp}")"
rm -f "${send_tmp}"
if [ "${send_code}" != "200" ]; then
  echo "send message failed: HTTP ${send_code} body=${send_body}" >&2
  exit 1
fi

request_id="$(
  tr -d '\r' < "${send_headers}" \
    | awk -F': ' 'tolower($1)=="x-request-id" {print $2; exit}'
)"
rm -f "${send_headers}"
if [ -z "${request_id}" ]; then
  echo "missing X-Request-Id on send response" >&2
  exit 1
fi
echo "captured request_id=${request_id}"

# LogQL query_range: expect http_access → grpc_call → nats_publish → nats_consume → ws_fanout
query="{namespace=\"${APP_NS}\"} | json | request_id=\"${request_id}\""
deadline=$((SECONDS + POLL_SECONDS))
found_events=""
while [ "${SECONDS}" -lt "${deadline}" ]; do
  start_ns="$(python3 -c 'import time; print(int((time.time()-300)*1e9))')"
  end_ns="$(python3 -c 'import time; print(int(time.time()*1e9))')"
  resp="$(curl -sS -G "${LOKI_LOCAL}/loki/api/v1/query_range" \
    --data-urlencode "query=${query}" \
    --data-urlencode "start=${start_ns}" \
    --data-urlencode "end=${end_ns}" \
    --data-urlencode "limit=200" || true)"
  found_events="$(
    echo "${resp}" | python3 -c '
import json,sys
need={"http_access","grpc_call","nats_publish","nats_consume","ws_fanout"}
got=set()
try:
  data=json.load(sys.stdin)
except Exception:
  print("")
  raise SystemExit(0)
for stream in data.get("data",{}).get("result",[]):
  for _, line in stream.get("values",[]):
    try:
      obj=json.loads(line)
    except Exception:
      continue
    ev=obj.get("event")
    if ev in need:
      got.add(ev)
print(",".join(sorted(got)))
'
  )"
  missing=0
  for ev in http_access grpc_call nats_publish nats_consume ws_fanout; do
    case ",${found_events}," in
      *",${ev},"*) ;;
      *) missing=1; break ;;
    esac
  done
  if [ "${missing}" = "0" ]; then
    echo "Loki chain ok for request_id=${request_id}: ${found_events}"
    echo "request_id smoke passed."
    exit 0
  fi
  sleep 3
done

echo "Loki chain incomplete for request_id=${request_id} within ${POLL_SECONDS}s; found=[${found_events}]" >&2
echo "LogQL: ${query}" >&2
exit 1
