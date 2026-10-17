#!/usr/bin/env bash
# DEV smoke: email verification → login/refresh → DM/history + receiver WebSocket → Loki chain.
# Specs: docs/TESTING.md § Debug by request_id, docs/features/auth-and-contacts.md.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"

BASE="${VOICE_STAGING_URL:-https://${VOICE_GATEWAY_INGRESS_HOST}}"
BASE="${BASE%/}"
APP_NS="${VOICE_STAGING_NAMESPACE:-voice-staging}"
OBS_NS="${VOICE_OBSERVABILITY_NAMESPACE:-voice-observability}"
LOKI_LOCAL="${LOKI_LOCAL_URL:-http://127.0.0.1:3100}"
MAIL_PORT="${VOICE_DEV_MAIL_LOCAL_PORT:-$((20000 + $$ % 20000))}"
MAIL_LOCAL="${VOICE_DEV_MAIL_LOCAL_URL:-http://127.0.0.1:${MAIL_PORT}}"
PASSWORD="${STAGING_SMOKE_PASSWORD:-VoiceQaTest1!}"
POLL_SECONDS="${REQUEST_ID_SMOKE_POLL_SECONDS:-90}"
WS_URL="${VOICE_STAGING_WS_URL:-${BASE/https/wss}/ws}"
SESSION_FILE="${VOICE_SMOKE_SESSION_FILE:-}"
EXPECTED_BASE="https://api-dev.comrade.click"
EXPECTED_WS_URL="wss://api-dev.comrade.click/ws"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd kubectl
need_cmd python3
need_cmd node
need_cmd npm
need_cmd tar

if [ "${APP_NS}" != "voice-dev" ] || [ "${BASE}" != "${EXPECTED_BASE}" ] || [ "${WS_URL}" != "${EXPECTED_WS_URL}" ]; then
  echo "this account-lifecycle smoke is pinned to the canonical voice-dev API, namespace, and WebSocket origin" >&2
  exit 1
fi

echo "request_id smoke: gateway ${BASE}"

not_ready="$(kubectl get pods -n "${OBS_NS}" --no-headers 2>/dev/null | awk '$3!="Running" && $3!="Completed" {print}' || true)"
if [ -n "${not_ready}" ]; then
  echo "observability pods not Running/Completed:" >&2
  echo "${not_ready}" >&2
  exit 1
fi

kubectl port-forward -n "${OBS_NS}" "svc/loki" 3100:3100 >/tmp/voice-loki-pf.log 2>&1 &
PF_LOKI=$!
kubectl port-forward --address 127.0.0.1 -n voice-dev svc/voice-dev-mail "${MAIL_PORT}:4180" >/tmp/voice-dev-mail-pf.log 2>&1 &
PF_MAIL=$!
PRIVATE_TMP="$(mktemp -d "${TMPDIR:-/tmp}/voice-request-id.XXXXXX")"
chmod 700 "${PRIVATE_TMP}"
cleanup() {
  local status=$?
  trap - EXIT
  set +e
  kill "${PF_LOKI}" 2>/dev/null || true
  kill "${PF_MAIL}" 2>/dev/null || true
  if [ -n "${WS_PID:-}" ]; then
    kill "${WS_PID}" 2>/dev/null || true
  fi
  exec 3>&- 2>/dev/null || true
  if [ -d "${PRIVATE_TMP}/node_modules/ws" ] && [ ! -L "${PRIVATE_TMP}/node_modules/ws" ]; then
    rm -rf -- "${PRIVATE_TMP}/node_modules/ws"
  fi
  rmdir "${PRIVATE_TMP}/node_modules" 2>/dev/null || true
  rm -f -- "${PRIVATE_TMP}"/*
  rmdir "${PRIVATE_TMP}" 2>/dev/null || true
  exit "${status}"
}
trap cleanup EXIT

load_session_fixture() {
  [ -n "${SESSION_FILE}" ] || return 0
  case "${SESSION_FILE}" in /*) ;; *) echo "VOICE_SMOKE_SESSION_FILE must be absolute" >&2; exit 1 ;; esac
  python3 - "${SESSION_FILE}" "${ROOT}" "${BASE}" "${APP_NS}" "${PRIVATE_TMP}/session.tsv" <<'PY'
import json,os,re,stat,sys
from pathlib import Path
p=Path(sys.argv[1]); root=Path(sys.argv[2]).resolve(); origin=sys.argv[3]; namespace=sys.argv[4]; out=Path(sys.argv[5])
resolved=p.resolve(strict=False)
if resolved==root or root in resolved.parents: raise SystemExit("session fixture must be outside the repository")
if p.is_symlink(): raise SystemExit("session fixture must not be a symlink")
if not p.exists(): out.write_text(""); raise SystemExit(0)
st=p.stat()
if not stat.S_ISREG(st.st_mode) or st.st_uid!=os.getuid() or stat.S_IMODE(st.st_mode)!=0o600:
    raise SystemExit("session fixture must be an owned regular file with mode 0600")
if st.st_size > 32768: raise SystemExit("session fixture exceeds the size limit")
def reject_duplicates(pairs):
    out={}
    for key,value in pairs:
        if key in out: raise ValueError("duplicate session fixture key")
        out[key]=value
    return out
try: d=json.loads(p.read_text(encoding="utf-8"),object_pairs_hook=reject_duplicates)
except (UnicodeError,ValueError,json.JSONDecodeError): raise SystemExit("session fixture JSON is invalid")
if not isinstance(d,dict) or set(d)!={"schema","api_origin","namespace","email_a","email_b","refresh_a","refresh_b"} or d.get("schema")!="voice-dev-smoke-session-v2":
    raise SystemExit("session fixture schema is invalid")
if d["api_origin"]!=origin or d["namespace"]!=namespace:
    raise SystemExit("session fixture environment does not match this API origin and namespace")
if not all(isinstance(d[k],str) for k in ("email_a","email_b")) or not re.fullmatch(r"reqid-a-[0-9]+@voice-qa\.test",d["email_a"]) or not re.fullmatch(r"reqid-b-[0-9]+@voice-qa\.test",d["email_b"]):
    raise SystemExit("session fixture emails are invalid")
if not all(isinstance(d[k],str) for k in ("refresh_a","refresh_b")) or bool(d["refresh_a"])!=bool(d["refresh_b"]):
    raise SystemExit("session fixture must contain both refresh tokens or neither")
if any(len(d[k])>16384 or re.search(r"[\s\x00-\x1f\x7f]",d[k]) for k in ("refresh_a","refresh_b")):
    raise SystemExit("session fixture refresh token format is invalid")
fd=os.open(out,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
with os.fdopen(fd,"w") as f: f.write("\n".join(d[k] or "-" for k in ("email_a","email_b","refresh_a","refresh_b")))
PY
  mapfile -t SESSION_VALUES < "${PRIVATE_TMP}/session.tsv"
  if [ "${#SESSION_VALUES[@]}" -gt 0 ]; then
    email_a="${SESSION_VALUES[0]}"; email_b="${SESSION_VALUES[1]}"
    refresh_a="${SESSION_VALUES[2]:--}"; refresh_b="${SESSION_VALUES[3]:--}"
    [ "${refresh_a}" != "-" ] || refresh_a=""
    [ "${refresh_b}" != "-" ] || refresh_b=""
  fi
}

save_session_fixture() {
  [ -n "${SESSION_FILE}" ] || return 0
  printf '%s\0' "${email_a}" "${email_b}" "${refresh_a}" "${refresh_b}" | python3 -c 'import json,os,secrets,stat,sys
from pathlib import Path
p=Path(sys.argv[1]); root=Path(sys.argv[2]).resolve(); origin=sys.argv[3]; namespace=sys.argv[4]; resolved=p.resolve(strict=False)
if not p.is_absolute() or resolved==root or root in resolved.parents: raise SystemExit("session fixture path must be absolute and outside the repository")
if not p.parent.is_dir() or p.is_symlink(): raise SystemExit("session fixture parent must exist and the fixture must not be a symlink")
if p.exists() and (not stat.S_ISREG(p.stat().st_mode) or p.stat().st_uid!=os.getuid() or stat.S_IMODE(p.stat().st_mode)!=0o600): raise SystemExit("refusing to replace unsafe session fixture")
values=sys.stdin.buffer.read().split(b"\0")
if values and values[-1]==b"": values.pop()
if len(values)!=4: raise SystemExit("session fixture input is invalid")
email_a,email_b,refresh_a,refresh_b=(v.decode("utf-8") for v in values)
if not all((email_a,email_b,refresh_a,refresh_b)): raise SystemExit("session fixture requires both emails and rotated refresh tokens")
if any(len(value)>16384 or any(ch.isspace() or ord(ch)<32 or ord(ch)==127 for ch in value) for value in (refresh_a,refresh_b)): raise SystemExit("session fixture refresh token format is invalid")
d={"schema":"voice-dev-smoke-session-v2","api_origin":origin,"namespace":namespace,"email_a":email_a,"email_b":email_b,"refresh_a":refresh_a,"refresh_b":refresh_b}
tmp=p.with_name(p.name+"."+secrets.token_hex(8)+".tmp")
try:
    fd=os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,"w") as f: json.dump(d,f,separators=(",",":")); f.flush(); os.fsync(f.fileno())
    os.replace(tmp,p)
finally:
    try: tmp.unlink()
    except FileNotFoundError: pass' "${SESSION_FILE}" "${ROOT}" "${BASE}" "${APP_NS}"
}

write_auth_header() {
  local output_file="$1" token="$2"
  (umask 077; printf 'Authorization: Bearer %s\n' "${token}" > "${output_file}")
}

write_request_body() {
  local output_file="$1" kind="$2"
  shift 2
  printf '%s\0' "$@" | python3 -c 'import json,os,sys
path,kind=sys.argv[1:]; values=sys.stdin.buffer.read().split(b"\0")
if values and values[-1]==b"": values.pop()
values=[v.decode("utf-8") for v in values]
if kind=="login" and len(values)==2: data={"email":values[0],"password":values[1],"device_info_json":"{}"}
elif kind=="refresh" and len(values)==1: data={"refresh_token":values[0],"device_info_json":"{}"}
elif kind=="verify" and len(values)==1: data={"code":values[0],"otp_type":"email_verify"}
elif kind=="dm" and len(values)==1: data={"other_profile_id":values[0]}
elif kind=="message" and len(values)==2: data={"chat":{"id":values[0]},"content":values[1]}
else: raise SystemExit("invalid private request input")
fd=os.open(path,os.O_WRONLY|os.O_TRUNC,0o600)
with os.fdopen(fd,"w") as f: json.dump(data,f,separators=(",",":"))' "${output_file}" "${kind}"
}

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
for attempt in 1 2 3 4 5 6 7 8 9 10; do
  if curl -sS -o /dev/null "${MAIL_LOCAL}/health" 2>/dev/null; then
    break
  fi
  sleep 1
done
if ! curl -sS -o /dev/null "${MAIL_LOCAL}/health" 2>/dev/null; then
  echo "DEV verification mail fixture not ready at ${MAIL_LOCAL}" >&2
  exit 1
fi

email_a=""
email_b=""
refresh_a=""
refresh_b=""
SESSION_VALUES=()
load_session_fixture

WS_PACKAGE_TARBALL="$(npm pack ws@8.21.0 --silent --ignore-scripts \
  --pack-destination "${PRIVATE_TMP}" --registry=https://registry.npmjs.org/)"
python3 - "${PRIVATE_TMP}/${WS_PACKAGE_TARBALL}" <<'PY'
import base64,hashlib,sys
expected="Vsp28b7DRcimFQvrqu2Wek3z1iYxDCWqHYB8Qsnk/S4RfaCQzPGPyBNuVjJV3cd6UiKtUtp6sNM77gWvzcCH+g=="
actual=base64.b64encode(hashlib.sha512(open(sys.argv[1],"rb").read()).digest()).decode()
if actual!=expected: raise SystemExit("WebSocket client package integrity mismatch")
PY
mkdir -p "${PRIVATE_TMP}/node_modules/ws"
tar -xzf "${PRIVATE_TMP}/${WS_PACKAGE_TARBALL}" --strip-components=1 -C "${PRIVATE_TMP}/node_modules/ws"
rm -f "${PRIVATE_TMP}/${WS_PACKAGE_TARBALL}"

if [ -z "${email_a}" ]; then
  n="$(date +%s%N)"
  email_a="reqid-a-${n}@voice-qa.test"
  email_b="reqid-b-${n}@voice-qa.test"
fi
n="${n:-$(date +%s%N)}"

register() {
  local email="$1"
  local body code payload_file
  payload_file="$(mktemp "${PRIVATE_TMP}/register.XXXXXX")"
  write_request_body "${payload_file}" login "${email}" "${PASSWORD}"
  body="$(curl -sS -w '\n%{http_code}' -X POST "${BASE}/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    --data-binary "@${payload_file}")"
  rm -f "${payload_file}"
  code="$(echo "${body}" | tail -n1)"
  body="$(echo "${body}" | sed '$d')"
  if [ "${code}" != "200" ]; then
    echo "register failed: HTTP ${code}" >&2
    exit 1
  fi
  echo "${body}"
}

verified_a_file="$(mktemp "${PRIVATE_TMP}/verified-a.XXXXXX")"
verified_b_file="$(mktemp "${PRIVATE_TMP}/verified-b.XXXXXX")"

login_to_file() {
  local email="$1" output_file="$2" status retry_after payload_file
  payload_file="$(mktemp "${PRIVATE_TMP}/login.XXXXXX")"
  write_request_body "${payload_file}" login "${email}" "${PASSWORD}"
  status="$(curl -sS -D "${output_file}.headers" -o "${output_file}" -w '%{http_code}' -X POST "${BASE}/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    --data-binary "@${payload_file}")"
  rm -f "${payload_file}"
  if [ "${status}" != "200" ]; then
    retry_after="$(awk 'tolower($1)=="retry-after:" {gsub("\r", "", $2); print $2; exit}' "${output_file}.headers")"
    if [ "${status}" = "429" ] && [ -n "${retry_after}" ]; then
      echo "ordinary login rate limited: HTTP 429 Retry-After=${retry_after}s" >&2
    else
      echo "ordinary login failed: HTTP ${status}" >&2
    fi
    exit 1
  fi
}

if [ -z "${SESSION_VALUES[0]:-}" ]; then
  echo "registering two disposable DEV accounts"
  sess_a="$(register "${email_a}")"
  sess_b="$(register "${email_b}")"
  pending_a="$(echo "${sess_a}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session"]["access_token"])')"
  pending_b="$(echo "${sess_b}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session"]["access_token"])')"

verify_email() {
  local email="$1" pending_token="$2" output_file="$3" code status mail_json verify_status status_file verified_token
  local pending_header verify_payload verified_header
  pending_header="$(mktemp "${PRIVATE_TMP}/auth-header.XXXXXX")"
  write_auth_header "${pending_header}" "${pending_token}"
  status_file="$(mktemp "${PRIVATE_TMP}/status.XXXXXX")"
  status="$(curl -sS -o "${status_file}" -w '%{http_code}' \
    -H "@${pending_header}" "${BASE}/api/v1/auth/verification-status")"
  if [ "${status}" != "200" ] || ! python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if d.get("state")=="EMAIL_PENDING" else 1)' "${status_file}"; then
    echo "expected EMAIL_PENDING verification state (HTTP ${status})" >&2
    exit 1
  fi

  code=""
  for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    mail_json="$(curl -sS -G "${MAIL_LOCAL}/emails/latest" --data-urlencode "to=${email}" 2>/dev/null || true)"
    code="$(printf '%s' "${mail_json}" | python3 -c 'import json,re,sys; d=json.load(sys.stdin); m=re.search(r"\b([0-9]{6})\b", d.get("text", "")); print(m.group(1) if m else "")' 2>/dev/null || true)"
    if [ -n "${code}" ]; then break; fi
    sleep 1
  done
  if [ -z "${code}" ]; then
    echo "verification email did not contain a six-digit code" >&2
    exit 1
  fi

  verify_payload="$(mktemp "${PRIVATE_TMP}/verify.XXXXXX")"
  write_request_body "${verify_payload}" verify "${code}"
  verify_status="$(curl -sS -o "${output_file}" -w '%{http_code}' \
    -X POST "${BASE}/api/v1/auth/otp/verify" \
    -H "@${pending_header}" -H 'Content-Type: application/json' \
    --data-binary "@${verify_payload}")"
  rm -f "${verify_payload}" "${pending_header}"
  if [ "${verify_status}" != "200" ] || ! python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); s=d.get("session",{}); sys.exit(0 if s.get("access_token") and s.get("refresh_token") else 1)' "${output_file}"; then
    echo "email verification did not return a replacement session (HTTP ${verify_status})" >&2
    exit 1
  fi
  verified_token="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"]["access_token"])' "${output_file}")"
  verified_header="$(mktemp "${PRIVATE_TMP}/auth-header.XXXXXX")"
  write_auth_header "${verified_header}" "${verified_token}"
  status="$(curl -sS -o "${status_file}" -w '%{http_code}' \
    -H "@${verified_header}" "${BASE}/api/v1/auth/verification-status")"
  rm -f "${verified_header}"
  if [ "${status}" != "200" ] || ! python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if d.get("state")=="REGULAR" else 1)' "${status_file}"; then
    echo "verification did not promote the account to REGULAR (HTTP ${status})" >&2
    exit 1
  fi
}

  verify_email "${email_a}" "${pending_a}" "${verified_a_file}"
  verify_email "${email_b}" "${pending_b}" "${verified_b_file}"
  # A login is exercised after the successful public email-verification flow.
  login_to_file "${email_a}" "${PRIVATE_TMP}/login-a.json"
  login_to_file "${email_b}" "${PRIVATE_TMP}/login-b.json"
  refresh_a="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"]["refresh_token"])' "${PRIVATE_TMP}/login-a.json")"
  refresh_b="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"]["refresh_token"])' "${PRIVATE_TMP}/login-b.json")"
elif [ -z "${refresh_a}" ]; then
  # An email-only fixture is a deliberate one-time seed for already verified accounts.
  login_to_file "${email_a}" "${PRIVATE_TMP}/login-a.json"
  login_to_file "${email_b}" "${PRIVATE_TMP}/login-b.json"
  refresh_a="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"]["refresh_token"])' "${PRIVATE_TMP}/login-a.json")"
  refresh_b="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"]["refresh_token"])' "${PRIVATE_TMP}/login-b.json")"
else
  echo "reusing private verified-account session fixture"
fi

refresh_session() {
  local old_refresh="$1" output_file="$2" status payload_file
  payload_file="$(mktemp "${PRIVATE_TMP}/refresh.XXXXXX")"
  write_request_body "${payload_file}" refresh "${old_refresh}"
  status="$(curl -sS -o "${output_file}" -w '%{http_code}' -X POST "${BASE}/api/v1/auth/refresh" \
    -H 'Content-Type: application/json' \
    --data-binary "@${payload_file}")"
  rm -f "${payload_file}"
  if [ "${status}" != "200" ]; then echo "session refresh failed: HTTP ${status}" >&2; exit 1; fi
  refreshed_access="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"]["access_token"])' "${output_file}")"
  rotated_refresh="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"]["refresh_token"])' "${output_file}")"
}

refresh_session "${refresh_a}" "${PRIVATE_TMP}/refresh-a.json"
token_a="${refreshed_access}"
refresh_a="${rotated_refresh}"
save_session_fixture
refresh_session "${refresh_b}" "${PRIVATE_TMP}/refresh-b.json"
token_b="${refreshed_access}"
refresh_b="${rotated_refresh}"
save_session_fixture
verify_regular() {
  local token="$1" status auth_file
  auth_file="$(mktemp "${PRIVATE_TMP}/auth-header.XXXXXX")"
  write_auth_header "${auth_file}" "${token}"
  status="$(curl -sS -o "${PRIVATE_TMP}/regular.json" -w '%{http_code}' \
    -H "@${auth_file}" "${BASE}/api/v1/auth/verification-status")"
  rm -f "${auth_file}"
  if [ "${status}" != "200" ] || ! python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if d.get("state")=="REGULAR" else 1)' "${PRIVATE_TMP}/regular.json"; then
    echo "reused account is not a regular verified session (HTTP ${status})" >&2
    exit 1
  fi
}
verify_regular "${token_a}"
verify_regular "${token_b}"
profile_b="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"]["profile_id"])' "${PRIVATE_TMP}/refresh-b.json")"

auth_a="$(mktemp "${PRIVATE_TMP}/auth-header.XXXXXX")"
auth_b="$(mktemp "${PRIVATE_TMP}/auth-header.XXXXXX")"
write_auth_header "${auth_a}" "${token_a}"
write_auth_header "${auth_b}" "${token_b}"
dm_payload="$(mktemp "${PRIVATE_TMP}/dm.XXXXXX")"
write_request_body "${dm_payload}" dm "${profile_b}"
dm_body="$(curl -sS -w '\n%{http_code}' -X POST "${BASE}/api/v1/chats/dm" \
  -H "@${auth_a}" \
  -H 'Content-Type: application/json' \
  --data-binary "@${dm_payload}")"
rm -f "${dm_payload}"
dm_code="$(echo "${dm_body}" | tail -n1)"
dm_json="$(echo "${dm_body}" | sed '$d')"
if [ "${dm_code}" != "200" ]; then
  echo "create DM failed: HTTP ${dm_code}" >&2
  exit 1
fi
chat_id="$(echo "${dm_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["chat"]["id"])')"

# Keep the access token in a private file; the Node client reads it in-process,
# so neither curl nor the WebSocket process receives a credential in argv.
WS_LOG="${PRIVATE_TMP}/ws.log"
WS_FIFO="${PRIVATE_TMP}/ws.in"
WS_TOKEN_FILE="${PRIVATE_TMP}/ws-token"
printf '%s' "${token_b}" > "${WS_TOKEN_FILE}"
chmod 600 "${WS_TOKEN_FILE}"
mkfifo "${WS_FIFO}"
NODE_PATH="${PRIVATE_TMP}/node_modules" node - "${WS_URL}" "${WS_TOKEN_FILE}" "${WS_LOG}" "${WS_FIFO}" >"${WS_LOG}.stderr" 2>&1 <<'NODE' &
const fs = require('node:fs');
const readline = require('node:readline');
const WebSocket = require('ws');
const [url, tokenPath, logPath, inputPath] = process.argv.slice(2);
const token = fs.readFileSync(tokenPath, 'utf8');
const socket = new WebSocket(url, {
  headers: { Authorization: `Bearer ${token}` },
  maxPayload: 1024 * 1024,
});
let queued = [];
const input = readline.createInterface({ input: fs.createReadStream(inputPath) });
input.on('line', (line) => {
  if (socket.readyState === WebSocket.OPEN) socket.send(line);
  else queued.push(line);
});
socket.on('open', () => {
  for (const line of queued) socket.send(line);
  queued = [];
});
socket.on('message', (data) => fs.appendFileSync(logPath, `${data.toString()}\n`, { mode: 0o600 }));
socket.on('error', () => { process.stderr.write('WebSocket client error\n'); process.exitCode = 1; });
input.on('close', () => socket.close());
NODE
WS_PID=$!
exec 3>"${WS_FIFO}"
printf '{"op":"subscribe","d":{"chat_id":"%s"}}\n' "${chat_id}" >&3

wait_ws_frame() {
  local op="$1" expected_chat="$2" expected_message="${3:-}"
  python3 - "${WS_LOG}" "${op}" "${expected_chat}" "${expected_message}" <<'PY'
import json,sys,time
from pathlib import Path
path, op, chat, message = sys.argv[1:]
deadline=time.monotonic()+20
while time.monotonic()<deadline:
    try:
        lines=Path(path).read_text(errors="replace").splitlines()
    except FileNotFoundError:
        lines=[]
    for line in lines:
        try: frame=json.loads(line)
        except Exception: continue
        data=frame.get("d", {})
        if frame.get("op") != op or data.get("chat_id") != chat:
            continue
        if message and data.get("message_id") != message:
            continue
        raise SystemExit(0)
    time.sleep(.25)
raise SystemExit(1)
PY
}
if ! wait_ws_frame subscribe_ack "${chat_id}"; then
  echo "receiver WebSocket did not acknowledge the chat subscription" >&2
  exit 1
fi

send_tmp="$(mktemp)"
send_headers="$(mktemp)"
send_payload="$(mktemp "${PRIVATE_TMP}/send.XXXXXX")"
write_request_body "${send_payload}" message "${chat_id}" "request-id-smoke-${n}"
send_code="$(curl -sS -D "${send_headers}" -o "${send_tmp}" -w "%{http_code}" \
  -X POST "${BASE}/api/v1/messages/send" \
  -H "@${auth_a}" \
  -H 'Content-Type: application/json' \
  --data-binary "@${send_payload}" || echo "000")"
rm -f "${send_tmp}" "${send_payload}"
if [ "${send_code}" != "200" ]; then
  echo "send message failed: HTTP ${send_code}" >&2
  exit 1
fi

history_file="$(mktemp "${PRIVATE_TMP}/history.XXXXXX")"
history_code="$(curl -sS -o "${history_file}" -w '%{http_code}' \
  -G "${BASE}/api/v1/messages" -H "@${auth_a}" \
  --data-urlencode "chat_id=${chat_id}")"
if [ "${history_code}" != "200" ] || ! python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); xs=d.get("message_list",{}).get("messages",[]); sys.exit(0 if any(x.get("content")==sys.argv[2] for x in xs) else 1)' "${history_file}" "request-id-smoke-${n}"; then
  echo "sent message was not readable from chat history (HTTP ${history_code})" >&2
  exit 1
fi
message_id="$(python3 - "${history_file}" "request-id-smoke-${n}" <<'PY'
import json,sys
data=json.load(open(sys.argv[1]))
for item in data.get("message_list",{}).get("messages",[]):
  if item.get("content")==sys.argv[2]:
    print(item.get("id") or item.get("message_id") or "")
    break
PY
)"
rm -f "${history_file}"
if [ -z "${message_id}" ] || ! wait_ws_frame message_create "${chat_id}" "${message_id}"; then
  echo "receiver WebSocket did not observe the sent message_create event" >&2
  exit 1
fi
echo "receiver WebSocket message delivery verified."

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
