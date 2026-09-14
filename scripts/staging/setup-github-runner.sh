#!/usr/bin/env bash
# Register a self-hosted GitHub Actions runner on the staging k3s node.
#
# Why: home staging hosts usually block inbound SSH/6443 from the internet.
# A runner on the node polls GitHub outbound; deploy uses local kubectl (127.0.0.1:6443).
#
# Run on the staging host as a normal user (e.g. pmd), not root.
#
# Env (optional):
#   GITHUB_REPO          owner/repo (default: Poryadok/VoiceRoot)
#   RUNNER_NAME          default: hostname
#   RUNNER_DIR           default: ~/actions-runner
#   RUNNER_LABELS        default: self-hosted,Linux,X64,voice-staging
#   RUNNER_VERSION       default: latest from GitHub API
#
set -euo pipefail

GITHUB_REPO="${GITHUB_REPO:-Poryadok/VoiceRoot}"
RUNNER_NAME="${RUNNER_NAME:-$(hostname -s)}"
RUNNER_DIR="${RUNNER_DIR:-${HOME}/actions-runner}"
RUNNER_LABELS="${RUNNER_LABELS:-self-hosted,Linux,X64,voice-staging}"

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "Missing dependency: $1" >&2; exit 1; }
}

need curl
need tar
need jq

require_minimum_runner_version() {
  local version="$1"
  local major minor patch

  if [[ ! "$version" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "Invalid GitHub Actions runner version: ${version}" >&2
    exit 1
  fi

  major="${BASH_REMATCH[1]}"
  minor="${BASH_REMATCH[2]}"
  patch="${BASH_REMATCH[3]}"
  if (( 10#$major < 2 || (10#$major == 2 && (10#$minor < 327 || (10#$minor == 327 && 10#$patch < 1)) ) )); then
    echo "GitHub Actions runner version 2.327.1 or newer is required; got ${version}." >&2
    exit 1
  fi
}

if [ -z "${RUNNER_TOKEN:-}" ]; then
  cat <<EOF
=== Self-hosted runner for ${GITHUB_REPO} ===

1. Open (repo admin):
   https://github.com/${GITHUB_REPO}/settings/actions/runners/new?arch=x64&os=linux

2. Copy the one-time registration token, then re-run:

   RUNNER_TOKEN=<token> bash scripts/staging/setup-github-runner.sh

3. In GitHub → Settings → Environments → staging → Variables, set:

   STAGING_RUNNER_LABELS=["self-hosted","Linux","X64","voice-staging"]

4. STAGING_KUBECONFIG: server https://127.0.0.1:6443 (prepare-kubeconfig-secret.sh)
   STAGING_DEPLOY_VIA_SSH=false

5. Close temporary WAN rules for 6443/22 if you opened them for debugging.

EOF
  exit 0
fi

ARCH="x64"
OS="linux"
VER="${RUNNER_VERSION:-$(curl -fsSL "https://api.github.com/repos/actions/runner/releases/latest" | jq -r .tag_name | sed 's/^v//')}"
require_minimum_runner_version "${VER}"
PKG="actions-runner-${OS}-${ARCH}-${VER}.tar.gz"
URL="https://github.com/actions/runner/releases/download/v${VER}/${PKG}"

mkdir -p "${RUNNER_DIR}"
cd "${RUNNER_DIR}"

if [ ! -f ./config.sh ]; then
  echo "Downloading runner v${VER}..."
  curl -fsSLO "${URL}"
  tar xzf "${PKG}"
  rm -f "${PKG}"
fi

./config.sh \
  --url "https://github.com/${GITHUB_REPO}" \
  --token "${RUNNER_TOKEN}" \
  --name "${RUNNER_NAME}" \
  --labels "${RUNNER_LABELS}" \
  --unattended \
  --replace

echo
echo "Runner configured in ${RUNNER_DIR}."
echo "Install as a service (recommended):"
echo "  cd ${RUNNER_DIR} && sudo ./svc.sh install ${USER:-$(id -un)} && sudo ./svc.sh start"
echo
echo "Or run interactively for a test:"
echo "  cd ${RUNNER_DIR} && ./run.sh"
echo
echo "Ensure kubectl works locally:"
echo "  kubectl cluster-info"
echo "  docker version   # needed for verify-staging-images.sh"
