# Story LFP private evidence erasure

Accepted producer c40 `StoryLfpErasure-v1` (384), SHA256
`9061a5d658ae93fabd16e20c14e462a746dbd889a0dfaad2359cf408366ab8c0`,
extends169/301L13/306. Consumers291 Story,145 MM,121 integration. This continuation
does not grant Auth ownership or resume172/191/230 operations.

- [ ] Story-purpose authenticated terminal owner/cycle decision and complete
  historically authenticated User profile scope; soft deletion is insufficient.
- [ ] Authenticated complete erasure snapshot and current cut-to-open seal,
  including canceled/recoverable cycles, offline replicas and pre-delete backups.
- [ ] Approved finite negative replay-memory disposal policy, or proved complete
  retirement of operation issuance, credentials, routes and restorable copies.
  Permanent hash/HMAC, local TTL and cohort rotation alone are insufficient.
- [ ] Actual storage/key owners demonstrate finite physical erasure across DB
  dead tuples/WAL/replicas/backups/exports/diagnostics by the required deadline.
- [ ] Ordinary Story worker: inventory, durable restrictive gate, exact structured
  dependency-closure scrub and zero residual personal copies; local completion
  never implies physical completion. MM consumes the owner cut independently and
  returns terminal-erased distinctly from ordinary registration absence.

These prerequisites are presently absent. The source adapter boundary is closed
without them. See [implementation and oracles](../testing/be011-lfp-consent-producer-exec-plan.md).
