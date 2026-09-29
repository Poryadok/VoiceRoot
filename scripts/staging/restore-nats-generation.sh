#!/usr/bin/env bash
# Restore only the immutable Secret set named by the active staging generation.
set +x
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=nats-generation.sh
source "${script_dir}/nats-generation.sh"
nats_generation_load
[[ "$NATS_GENERATION" != legacy ]] || exit 0

secrets=("$NATS_OPERATOR_SECRET" "$NATS_TLS_SECRET" "$NATS_BOOTSTRAP_SECRET" "$NATS_SERVICE_SECRET")
present=0
for name in "${secrets[@]}"; do
  if read_result="$(kubectl get secret "$name" -n voice-staging -o name 2>&1)"; then
    present=$((present + 1))
  elif [[ "$read_result" != *'NotFound'* && "$read_result" != *"secrets \"${name}\" not found"* ]]; then
    echo 'ERROR: cannot inspect active NATS generation Secret set' >&2
    exit 1
  fi
done
if ((present == ${#secrets[@]})); then
  echo 'Active staging NATS generation Secrets already present'
  exit 0
fi
if ((present != 0)); then
  echo 'ERROR: partial active NATS generation Secret set; stop and recover manually' >&2
  exit 1
fi

[[ -n "${STAGING_NATS_ROTATION_SECRETS_B64:-}" ]] || {
  echo 'ERROR: active NATS generation Secret bundle is unavailable' >&2
  exit 1
}
umask 077
bundle="$(mktemp)"
trap 'rm -f "$bundle"' EXIT
if ! printf '%s' "$STAGING_NATS_ROTATION_SECRETS_B64" | base64 -d 2>/dev/null | gzip -d >"$bundle" 2>/dev/null; then
  echo 'ERROR: cannot decode active NATS generation Secret bundle' >&2
  exit 1
fi

# Validate the entire namespace-bound bundle before any Kubernetes write. Do
# not print the file or jq diagnostics: it contains credentials and TLS keys.
if ! jq -e --arg generation "$NATS_GENERATION" '
  def encoded: type == "string" and length > 0 and test("^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$");
  def exact_keys($want): (.data | type == "object" and (keys | sort) == ($want | sort) and all(.[]; encoded));
  .apiVersion == "v1" and .kind == "List" and (keys | sort) == ["apiVersion","items","kind"] and (.items | type == "array" and length == 4) and
  ([.items[].metadata.name] | sort) == (["voice-nats-operator-", "voice-nats-hub-tls-", "voice-nats-bootstrap-credentials-", "voice-nats-service-credentials-"] | map(. + $generation) | sort) and
  all(.items[]; .apiVersion == "v1" and .kind == "Secret" and .type == "Opaque" and .immutable == true and (keys | sort) == ["apiVersion","data","immutable","kind","metadata","type"] and .metadata.namespace == "voice-staging" and (.metadata | keys | sort) == ["name","namespace"] and (has("stringData") | not)) and
  (.items[] | select(.metadata.name == ("voice-nats-operator-" + $generation)) | exact_keys(["operator.jwt","account.jwt","system-account.jwt","account.public","system-account.public"])) and
  (.items[] | select(.metadata.name == ("voice-nats-hub-tls-" + $generation)) | exact_keys(["tls.crt","tls.key","ca.crt"])) and
  (.items[] | select(.metadata.name == ("voice-nats-bootstrap-credentials-" + $generation)) | exact_keys(["bootstrap.creds"])) and
  (.items[] | select(.metadata.name == ("voice-nats-service-credentials-" + $generation)) | exact_keys(["analytics.creds","auth.creds","bot.creds","chat.creds","file.creds","gateway.creds","matchmaking.creds","messaging.creds","moderation.creds","notification.creds","realtime.creds","role.creds","search.creds","social.creds","space.creds","story.creds","subscription.creds","user.creds","voice.creds"]))
' "$bundle" >/dev/null 2>&1; then
  echo 'ERROR: invalid active NATS generation Secret bundle contract' >&2
  exit 1
fi

if ! kubectl create -n voice-staging --dry-run=server -o name -f - <"$bundle" >/dev/null 2>&1; then
  echo 'ERROR: Kubernetes rejected active NATS generation Secret bundle preflight' >&2
  exit 1
fi
if ! kubectl create -n voice-staging -f - <"$bundle" >/dev/null 2>&1; then
  echo 'ERROR: active NATS generation Secret restore failed; inspect Secret set before retry' >&2
  exit 1
fi
echo 'Active staging NATS generation Secrets restored'
