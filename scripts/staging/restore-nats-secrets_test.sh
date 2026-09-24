#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/present"

cat >"$work/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "$1" in
  get) [[ -f "$TEST_STATE/present/$3" ]] ;;
  create)
    [[ "$*" == *"-n voice-staging "* ]]
    jq -e '.kind == "List" and (.items | length == 4)' > /dev/null
    [[ "$*" == *"--dry-run=server"* ]] || touch "$TEST_STATE/created"
    ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$work/bin/kubectl"
export PATH="$work/bin:$PATH" TEST_STATE="$work" VOICE_K8S_NAMESPACE=voice-staging

bundle="$(jq -nc '
  {kind:"List",items:[
    {apiVersion:"v1",kind:"Secret",metadata:{name:"voice-nats-operator",namespace:"voice-staging"},type:"Opaque",data:{"operator.jwt":"eA==","account.jwt":"eA==","system-account.jwt":"eA==","account.public":"eA==","system-account.public":"eA=="}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:"voice-nats-hub-tls",namespace:"voice-staging"},type:"Opaque",data:{"tls.crt":"eA==","tls.key":"eA==","ca.crt":"eA=="}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:"voice-nats-bootstrap-credentials",namespace:"voice-staging"},type:"Opaque",data:{"bootstrap.creds":"eA=="}},
    {apiVersion:"v1",kind:"Secret",metadata:{name:"voice-nats-service-credentials",namespace:"voice-staging"},type:"Opaque",data:(["analytics","auth","bot","chat","file","gateway","matchmaking","messaging","moderation","notification","realtime","role","search","social","space","story","subscription","user","voice"] | map({key:(.+".creds"),value:"eA=="}) | from_entries)}
  ]}')"
export STAGING_NATS_SECRETS_B64="$(printf '%s' "$bundle" | gzip | base64 | tr -d '\r\n')"

bash "$root/scripts/staging/restore-nats-secrets.sh" >"$work/output" 2>&1
[[ -f "$work/created" ]]
! grep -Fq 'eA==' "$work/output"

rm "$work/created"
for name in voice-nats-operator voice-nats-hub-tls voice-nats-bootstrap-credentials voice-nats-service-credentials; do
  touch "$work/present/$name"
done
unset STAGING_NATS_SECRETS_B64
bash "$root/scripts/staging/restore-nats-secrets.sh" >"$work/output" 2>&1
[[ ! -f "$work/created" ]]

rm "$work/present/voice-nats-operator"
if bash "$root/scripts/staging/restore-nats-secrets.sh" >"$work/output" 2>&1; then
  echo 'partial set unexpectedly accepted' >&2
  exit 1
fi
grep -Fq 'partial NATS secret set' "$work/output"

rm "$work/present/voice-nats-hub-tls" "$work/present/voice-nats-bootstrap-credentials" "$work/present/voice-nats-service-credentials"
export STAGING_NATS_SECRETS_B64="$(printf '%s' "${bundle/voice-staging/voice-prod}" | gzip | base64 | tr -d '\r\n')"
if bash "$root/scripts/staging/restore-nats-secrets.sh" >"$work/output" 2>&1; then
  echo 'wrong namespace unexpectedly accepted' >&2
  exit 1
fi
[[ ! -f "$work/created" ]]

export STAGING_NATS_SECRETS_B64="$(printf '%s' "$bundle" | jq 'del(.items[] | select(.metadata.name == "voice-nats-service-credentials") | .data["chat.creds"])' | gzip | base64 | tr -d '\r\n')"
if bash "$root/scripts/staging/restore-nats-secrets.sh" >"$work/output" 2>&1; then
  echo 'missing credential key unexpectedly accepted' >&2
  exit 1
fi
[[ ! -f "$work/created" ]]

export STAGING_NATS_SECRETS_B64="$(printf '%s' "$bundle" | jq '(.items[] | select(.metadata.name == "voice-nats-service-credentials") | .data["chat.creds"]) = "not base64"' | gzip | base64 | tr -d '\r\n')"
if bash "$root/scripts/staging/restore-nats-secrets.sh" >"$work/output" 2>&1; then
  echo 'invalid credential encoding unexpectedly accepted' >&2
  exit 1
fi
[[ ! -f "$work/created" ]]

echo 'restore-nats-secrets tests passed'
