#!/usr/bin/env bash
# Generate Secrets locally on legacy and modern kubectl before applying them.

kubectl_secret_dry_run_flag() {
  if kubectl create secret generic --help 2>&1 | grep -Eq -- '--dry-run(=|[[:space:]]).*client'; then
    printf '%s\n' '--dry-run=client'
  else
    printf '%s\n' '--dry-run'
  fi
}

kubectl_apply_secret() (
  local name="$1"
  local namespace="$2"
  shift 2

  # The subshell keeps cleanup and restrictive permissions local to this Secret.
  umask 077
  local manifest
  manifest="$(mktemp)" || return 1
  trap 'rm -f "${manifest}"' EXIT
  if ! kubectl create secret generic "${name}" --namespace="${namespace}" "$@" \
    "$(kubectl_secret_dry_run_flag)" -o yaml >"${manifest}"; then
    return 1
  fi
  if [ ! -s "${manifest}" ]; then
    echo "ERROR: kubectl generated an empty Secret manifest for ${name}" >&2
    return 1
  fi

  kubectl apply -f "${manifest}"
)
