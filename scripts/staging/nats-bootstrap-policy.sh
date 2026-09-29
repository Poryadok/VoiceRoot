#!/usr/bin/env bash
# An existing clean-install marker permits additive bootstrap reconciliation only
# after the PVC-backed hub is the live NATS Service target. No reset is implied.
nats_bootstrap_on_active_pvc() {
  [ "${1:-}" = clean-install ] &&
    [ "${2:-}" = voice-nats-pvc-candidate ] &&
    [ "${3:-}" = false ]
}
