#!/usr/bin/env bash
# An existing clean-install marker permits additive bootstrap reconciliation only
# after the PVC-backed hub is the live NATS Service target. No reset is implied.
nats_bootstrap_on_active_pvc() {
  [ "${1:-}" = clean-install ] &&
    [ "${2:-}" = voice-nats-pvc-candidate ] &&
    [ "${3:-}" = false ]
}

# Direct infra apply is the documented PVC candidate preparation step. A full
# deploy may only continue when the accepted candidate can be bootstrapped.
nats_bootstrap_action() {
  if nats_bootstrap_on_active_pvc "${1:-}" "${2:-}" "${3:-}"; then
    printf 'bootstrap\n'
  elif [ -z "${1:-}" ] && [ "${2:-}" = voice-nats ] && [ "${3:-}" = false ] && [ "${4:-false}" = false ]; then
    printf 'prepare\n'
  else
    return 1
  fi
}

nats_acl_proof_valid() {
  [ -n "${1:-}" ] && [ "${1:-}" = "${2:-}" ]
}
