#!/usr/bin/env bash
# Build base64 for GitHub secret STAGING_KUBECONFIG (Environment: staging).
# Replaces cluster server URL so a GitHub-hosted runner can reach k3s (not 127.0.0.1 / 0.0.0.0).
#
# Usage:
#   STAGING_KUBE_API_SERVER=https://95.31.10.177:6443 \
#     ./scripts/staging/prepare-kubeconfig-secret.sh ~/.kube/staging-config
#
# On the staging host, copy config first:
#   mkdir -p ~/.kube && sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config && sudo chown "$(id -u):$(id -g)" ~/.kube/config
#
set -euo pipefail
KCFG="${1:?path to kubeconfig file}"
SERVER="${STAGING_KUBE_API_SERVER:-https://95.31.10.177:6443}"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
awk -v srv="$SERVER" '
  {
    if ($0 ~ /^[[:space:]]*server:[[:space:]]*https:\/\//) {
      sub(/server:[[:space:]]*https:\/\/.*/, "server: " srv)
    }
    print
  }
' "$KCFG" > "$TMP"
chmod 600 "$TMP"
if command -v base64 >/dev/null 2>&1; then
  if base64 --help 2>&1 | grep -q '\-w'; then
    base64 -w0 "$TMP"
  else
    base64 "$TMP" | tr -d '\n'
  fi
  echo
else
  echo "base64 not found" >&2
  exit 1
fi
