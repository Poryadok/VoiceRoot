#!/usr/bin/env bash
# Configure kubectl for GitHub Actions staging deploy.
#
# Direct mode: STAGING_KUBECONFIG must point at an API server reachable from the runner.
# SSH tunnel mode (recommended for home/single-node k3s): set STAGING_SSH_PRIVATE_KEY;
# kubectl traffic goes through SSH to 127.0.0.1:6443 on the staging host.
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

ssh_tunnel_enabled() {
  local mode="${STAGING_DEPLOY_VIA_SSH:-auto}"
  case "${mode}" in
    true|yes|1) return 0 ;;
    false|no|0) return 1 ;;
    auto)
      [ -n "${STAGING_SSH_PRIVATE_KEY:-}" ]
      ;;
    *)
      echo "WARN: unknown STAGING_DEPLOY_VIA_SSH=${mode}; treating as auto" >&2
      [ -n "${STAGING_SSH_PRIVATE_KEY:-}" ]
      ;;
  esac
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
  ssh -i "${key_file}" \
    -p "${port}" \
    -o BatchMode=yes \
    -o ExitOnForwardFailure=yes \
    -o ServerAliveInterval=30 \
    -o StrictHostKeyChecking=accept-new \
    -f -N \
    -L "127.0.0.1:6443:127.0.0.1:6443" \
    "${user}@${host}"

  kubectl config set-cluster "${CLUSTER_NAME}" --server="https://127.0.0.1:6443"
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

print_connectivity_help() {
  cat >&2 <<EOF
ERROR: Cannot reach Kubernetes API (${API_SERVER}) from this runner.

If the staging API is not exposed on the public internet (typical for k3s on a home router),
use SSH tunnel mode:

  1. Add Environment secret STAGING_SSH_PRIVATE_KEY (PEM key for SSH user on staging host).
  2. Optional Variables: STAGING_SSH_HOST (default 95.31.10.177), STAGING_SSH_USER (default pmd).
  3. Set STAGING_DEPLOY_VIA_SSH=true (or leave auto when the SSH key secret is set).

STAGING_KUBECONFIG can use server https://127.0.0.1:6443 (from /etc/rancher/k3s/k3s.yaml).

Alternative: open TCP 6443 to GitHub Actions IP ranges or run a self-hosted runner on the staging node.
See docs/STAGING_SERVER.md and docs/DEPLOYMENT.md.
EOF
}

if ssh_tunnel_enabled; then
  : "${STAGING_SSH_PRIVATE_KEY:?STAGING_DEPLOY_VIA_SSH requires STAGING_SSH_PRIVATE_KEY}"
  start_ssh_tunnel
  if ! wait_for_api; then
    echo "ERROR: SSH tunnel up but Kubernetes API did not respond on 127.0.0.1:6443" >&2
    exit 1
  fi
else
  if ! wait_for_api; then
    if [ -n "${STAGING_SSH_PRIVATE_KEY:-}" ]; then
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
