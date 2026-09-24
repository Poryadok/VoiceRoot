#!/usr/bin/env bash
# Restore the complete staging NATS secret set from the environment secret manager.
set +x
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
if [[ "$NS" != voice-staging ]]; then
  echo "ERROR: staging NATS bundle cannot be applied to $NS" >&2
  exit 1
fi

secrets=(voice-nats-operator voice-nats-hub-tls voice-nats-bootstrap-credentials voice-nats-service-credentials)
present=0
for name in "${secrets[@]}"; do
  if kubectl get secret "$name" -n "$NS" >/dev/null 2>&1; then
    present=$((present + 1))
  fi
done
if ((present == ${#secrets[@]})); then
  echo "NATS secrets already present in $NS"
  exit 0
fi
if ((present != 0)); then
  echo "ERROR: partial NATS secret set in $NS; restore only the missing Secrets from the staging secret manager" >&2
  exit 1
fi

: "${STAGING_NATS_SECRETS_B64:?Set the staging environment secret STAGING_NATS_SECRETS_B64}"
umask 077
bundle="$(mktemp)"
trap 'rm -f "$bundle"' EXIT
printf '%s' "$STAGING_NATS_SECRETS_B64" | base64 -d | gzip -d > "$bundle"

# Validate the full, namespace-bound contract before the first write. Only
# Kubernetes data is accepted; plaintext stringData cannot enter CI logs.
jq -e '
  def encoded: type == "string" and length > 0 and test("^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$");
  def exact_keys($want): (.data | type == "object" and (keys | sort) == ($want | sort) and all(.[]; encoded));
  .kind == "List" and (.items | type == "array" and length == 4) and
  ([.items[].metadata.name] | sort) == (["voice-nats-operator","voice-nats-hub-tls","voice-nats-bootstrap-credentials","voice-nats-service-credentials"] | sort) and
  all(.items[]; .apiVersion == "v1" and .kind == "Secret" and .type == "Opaque" and .metadata.namespace == "voice-staging" and (.metadata | keys | sort) == ["name","namespace"] and (has("stringData") | not)) and
  (.items[] | select(.metadata.name == "voice-nats-operator") | exact_keys(["operator.jwt","account.jwt","system-account.jwt","account.public","system-account.public"])) and
  (.items[] | select(.metadata.name == "voice-nats-hub-tls") | exact_keys(["tls.crt","tls.key","ca.crt"])) and
  (.items[] | select(.metadata.name == "voice-nats-bootstrap-credentials") | exact_keys(["bootstrap.creds"])) and
  (.items[] | select(.metadata.name == "voice-nats-service-credentials") | exact_keys(["analytics.creds","auth.creds","bot.creds","chat.creds","file.creds","gateway.creds","matchmaking.creds","messaging.creds","moderation.creds","notification.creds","realtime.creds","role.creds","search.creds","social.creds","space.creds","story.creds","subscription.creds","user.creds","voice.creds"]))
' "$bundle" >/dev/null || { echo 'ERROR: invalid staging NATS secret bundle contract' >&2; exit 1; }

# The API server must accept every item before any persistent create request.
kubectl create -n "$NS" --dry-run=server -o name -f - < "$bundle" >/dev/null
kubectl create -n "$NS" -f - < "$bundle"
