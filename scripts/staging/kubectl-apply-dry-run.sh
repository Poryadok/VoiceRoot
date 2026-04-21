#!/usr/bin/env bash
# Same kubectl apply as .github/workflows/staging-deploy.yml, with --dry-run=server.
# Requires: kubectl, KUBECONFIG pointing at a kubeconfig whose server URL is reachable from this machine.
#
# Usage:
#   export KUBECONFIG=/path/to/fixed-config
#   export GATEWAY_IMAGE=ghcr.io/owner/voice/gateway:sha
#   ./scripts/staging/kubectl-apply-dry-run.sh
#
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
: "${GATEWAY_IMAGE:?set GATEWAY_IMAGE e.g. ghcr.io/org/voice/gateway:latest}"
kubectl apply --dry-run=server -f deploy/staging/namespace.yaml
sed "s|IMAGE_PLACEHOLDER|${GATEWAY_IMAGE}|g" deploy/staging/gateway-deployment.yaml | kubectl apply --dry-run=server -f -
