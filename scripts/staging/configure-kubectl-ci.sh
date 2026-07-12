#!/usr/bin/env bash
# Configure kubectl for GitHub Actions staging deploy.
#
# Modes (pick one for your network):
#   1. Self-hosted runner on the k3s node (recommended for home staging) — local API, no inbound SSH/6443.
#   2. SSH tunnel from github-hosted runner — needs inbound SSH from the internet (or GitHub IP allowlist).
#   3. Direct API — only if kube-apiserver is reachable from the runner (datacenter / allowlisted IPs).
#
# Env:
#   STAGING_KUBECONFIG_B64   (required) base64 kubeconfig
#   STAGING_SSH_PRIVATE_KEY  (optional) PEM private key for SSH
#   STAGING_SSH_HOST         (default: 95.31.10.177)
#   STAGING_SSH_USER         (default: pmd)
#   STAGING_SSH_PORT         (default: 22)
#   STAGING_DEPLOY_VIA_SSH   true|false|auto (default: auto)
#
set -euo pipefail

: "${STAGING_KUBECONFIG_B64:?STAGING_KUBECONFIG_B64 is required}"

mkdir -p ~/.kube
echo "${STAGING_KUBECONFIG_B64}" | base64 -d > ~/.kube/config
chmod 600 ~/.kube/config

CLUSTER_NAME="$(kubectl config view -o jsonpath='{.clusters[0].name}')"
API_SERVER="$(kubectl config view -o jsonpath="{.clusters[?(@.name==\"${CLUSTER_NAME}\")].cluster.server}")"

is_local_api_server() {
  case "${API_SERVER}" in
    *127.0.0.1*|*localhost*) return 0 ;;
    *) return 1 ;;
  esac
}

is_github_hosted_runner() {
  [ "${RUNNER_ENVIRONMENT:-}" = "github-hosted" ]
}

ssh_tunnel_enabled() {
  local mode="${STAGING_DEPLOY_VIA_SSH:-auto}"
  case "${mode}" in
    true|yes|1) return 0 ;;
    false|no|0) return 1 ;;
    auto)
      if is_github_hosted_runner && is_local_api_server; then
        # github-hosted cannot reach 127.0.0.1 on the staging node; need SSH if key present.
        [ -n "${STAGING_SSH_PRIVATE_KEY:-}" ]
      else
        return 1
      fi
      ;;
    *)
      echo "WARN: unknown STAGING_DEPLOY_VIA_SSH=${mode}; treating as auto" >&2
      if is_github_hosted_runner && is_local_api_server; then
        [ -n "${STAGING_SSH_PRIVATE_KEY:-}" ]
      else
        return 1
      fi
      ;;
  esac
}

import_local_k3s_kubeconfig() {
  local src="${STAGING_K3S_KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
  [ -r "${src}" ] || return 1
  cp "${src}" ~/.kube/config
  chmod 600 ~/.kube/config
  CLUSTER_NAME="$(kubectl config view -o jsonpath='{.clusters[0].name}')"
  kubectl config set-cluster "${CLUSTER_NAME}" --server="https://127.0.0.1:6443" >/dev/null
  API_SERVER="https://127.0.0.1:6443"
}

start_ssh_tunnel() {
  local host user port key_file
  host="${STAGING_SSH_HOST:-95.31.10.177}"
  user="${STAGING_SSH_USER:-pmd}"
  port="${STAGING_SSH_PORT:-22}"
  key_file="$(mktemp)"
  trap 'rm -f "${key_file}"' RETURN

  printf '%s\n' "${STAGING_SSH_PRIVATE_KEY}" | tr -d '\r' > "${key_file}"
  chmod 600 "${key_file}"

  mkdir -p ~/.ssh
  ssh-keyscan -p "${port}" -H "${host}" >> ~/.ssh/known_hosts 2>/dev/null || true

  echo "Starting SSH tunnel to ${user}@${host}:${port} -> 127.0.0.1:6443"
  if ! ssh -i "${key_file}" \
    -p "${port}" \
    -o BatchMode=yes \
    -o ConnectTimeout=20 \
    -o ExitOnForwardFailure=yes \
    -o ServerAliveInterval=30 \
    -o StrictHostKeyChecking=accept-new \
    -f -N \
    -L "127.0.0.1:6443:127.0.0.1:6443" \
    "${user}@${host}"; then
    print_ssh_tunnel_help "${host}" "${port}"
    return 1
  fi

  kubectl config set-cluster "${CLUSTER_NAME}" --server="https://127.0.0.1:6443"
  API_SERVER="https://127.0.0.1:6443"
}

wait_for_api() {
  local attempt
  for attempt in $(seq 1 30); do
    if kubectl cluster-info --request-timeout=5s >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

print_ssh_tunnel_help() {
  local host="${1:-95.31.10.177}"
  local port="${2:-22}"
  cat >&2 <<EOF
ERROR: SSH tunnel to ${host}:${port} failed.

Typical causes on a home staging host:
  - UFW allows SSH only from LAN (e.g. 22/tcp ALLOW 192.168.0.0/24)
  - Router forwards SSH but the host firewall still blocks WAN SSH

Opening SSH or kube-apiserver (6443) to the whole internet is usually a bad idea.

Recommended: run deploy jobs on a self-hosted GitHub Actions runner on the staging node
(no inbound admin ports). See scripts/staging/setup-github-runner.sh and docs/STAGING_SERVER.md

Set GitHub Variable STAGING_RUNNER_LABELS to:
  ["self-hosted", "Linux", "X64", "voice-staging"]

Use STAGING_KUBECONFIG with server https://127.0.0.1:6443 and STAGING_DEPLOY_VIA_SSH=false.
EOF
}

print_connectivity_help() {
  cat >&2 <<EOF
ERROR: Cannot reach Kubernetes API (${API_SERVER}) from this runner.

If the staging API is not exposed on the public internet (typical for k3s on a home router),
use a self-hosted runner on the staging node (recommended) or SSH tunnel mode.

Self-hosted runner (recommended):
  1. scripts/staging/setup-github-runner.sh on the staging host
  2. GitHub Variable STAGING_RUNNER_LABELS=["self-hosted","Linux","X64","voice-staging"]
  3. STAGING_KUBECONFIG with server https://127.0.0.1:6443
  4. STAGING_DEPLOY_VIA_SSH=false

SSH tunnel (github-hosted runner only, needs inbound SSH):
  1. Environment secret STAGING_SSH_PRIVATE_KEY
  2. Allow SSH from GitHub Actions IP ranges (not 0.0.0.0/0) or use a bastion
  3. STAGING_DEPLOY_VIA_SSH=true

See docs/STAGING_SERVER.md and docs/DEPLOYMENT.md.
EOF
}

reject_public_api_from_github_hosted() {
  if ! is_github_hosted_runner; then
    return 0
  fi
  if is_local_api_server; then
    return 0
  fi
  cat >&2 <<EOF
ERROR: STAGING_KUBECONFIG points at ${API_SERVER}, but this is a github-hosted runner.

Cloud runners cannot reach a home staging kube-apiserver on a public IP reliably, and exposing
6443 to the internet is discouraged. Use a self-hosted runner on the staging node instead.

Fix:
  - STAGING_KUBECONFIG server: https://127.0.0.1:6443 (prepare-kubeconfig-secret.sh)
  - STAGING_RUNNER_LABELS=["self-hosted","Linux","X64","voice-staging"]
  - scripts/staging/setup-github-runner.sh on pmdebook
EOF
  return 1
}

# Self-hosted runner on the k3s node: local API, no tunnel.
if ! is_github_hosted_runner; then
  if is_local_api_server && wait_for_api; then
    echo "Kubernetes API reachable locally (cluster: ${CLUSTER_NAME}, server: ${API_SERVER})"
    kubectl cluster-info --request-timeout=15s
    exit 0
  fi
  if import_local_k3s_kubeconfig && wait_for_api; then
    echo "Kubernetes API reachable via local k3s.yaml (cluster: ${CLUSTER_NAME}, server: ${API_SERVER})"
    kubectl cluster-info --request-timeout=15s
    exit 0
  fi
fi

reject_public_api_from_github_hosted

if ssh_tunnel_enabled; then
  : "${STAGING_SSH_PRIVATE_KEY:?STAGING_SSH_PRIVATE_KEY is required for SSH tunnel mode}"
  start_ssh_tunnel
  if ! wait_for_api; then
    echo "ERROR: SSH tunnel up but Kubernetes API did not respond on 127.0.0.1:6443" >&2
    exit 1
  fi
else
  if ! wait_for_api; then
    if [ -n "${STAGING_SSH_PRIVATE_KEY:-}" ] && is_github_hosted_runner; then
      echo "Direct API unreachable; retrying via SSH tunnel..."
      start_ssh_tunnel
      if ! wait_for_api; then
        echo "ERROR: SSH tunnel mode also failed to reach Kubernetes API" >&2
        exit 1
      fi
    else
      print_connectivity_help
      exit 1
    fi
  fi
fi

echo "Kubernetes API reachable (cluster: ${CLUSTER_NAME}, server: $(kubectl config view -o jsonpath="{.clusters[?(@.name==\"${CLUSTER_NAME}\")].cluster.server}") )"
kubectl cluster-info --request-timeout=15s
