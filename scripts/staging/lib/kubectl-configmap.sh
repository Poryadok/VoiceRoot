#!/usr/bin/env bash
# Create a ConfigMap manifest locally, then apply it on both legacy and modern kubectl.
#
# Kubernetes clients before the dry-run enum accepted a boolean `--dry-run`, while
# modern clients require `--dry-run=client`. Keep the generated manifest in a
# temporary file so a rejected flag can never feed an empty stream to apply.

kubectl_configmap_dry_run_flag() {
  if kubectl create configmap --help 2>&1 | grep -Eq -- '--dry-run(=|[[:space:]]).*client'; then
    printf '%s\n' '--dry-run=client'
  else
    printf '%s\n' '--dry-run'
  fi
}

kubectl_apply_configmap() {
  local name="$1"
  local namespace="$2"
  shift 2

  local manifest
  manifest="$(mktemp)"
  if ! kubectl create configmap "${name}" --namespace="${namespace}" "$@" \
    "$(kubectl_configmap_dry_run_flag)" -o yaml >"${manifest}"; then
    rm -f "${manifest}"
    return 1
  fi

  if [ ! -s "${manifest}" ]; then
    echo "ERROR: kubectl generated an empty ConfigMap manifest for ${name}" >&2
    rm -f "${manifest}"
    return 1
  fi

  if kubectl apply -f "${manifest}"; then
    rm -f "${manifest}"
    return 0
  else
    local status=$?
    rm -f "${manifest}"
    return "${status}"
  fi
}
