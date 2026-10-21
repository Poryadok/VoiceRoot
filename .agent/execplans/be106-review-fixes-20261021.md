# BE-106: exact-generation Leave and safe ordinary DOWN

Author Rudeus-1, lease31 `f7ad171cd8afe35953f1f9bac816c71f`, branch
`codex/be106-review-fixes-20261021`, base
`4ea5dbc551aa0abf890d0c745bb8d341e8622808`.

Scope: PR875 Voice/MM review c1 and Astra review c6. Existing service contract
requires stale operations not to eject a replacement membership, and ordinary
DOWN to refuse retained native journal/snapshot evidence. No new API, retirement
policy, Space/Game ownership, wire event or rollout change.

Test-first acceptance:

- Pause Leave during trusted User lookup after it observes G1; another Leave
  completes G1, Join commits G2; releasing the first request must preserve G2,
  its media identity, roster/events and cleanup effects.
- Actual DOWN waits behind an uncommitted first native journal or conversion
  snapshot writer. After commit it must refuse and preserve every committed byte.
- Inherited REPEATABLE READ must be rejected; empty READ COMMITTED DOWN/UP works.

Risk: generation-authority and irreversible DDL; independent reviewer required.
Use approved isolated cached Linux Go1.26 + PG runner with private internal
network, no host ports/sockets/listeners and existing native DSN selector.
No Windows test.exe, GH/CI/DEV, CodeGraph or graphify under current user override.

## Implementation and actual evidence

Tests were authored before production changes. Linux Go 1.26.5 and native PG16
reproduced the stale Leave (nil error after G2), both concurrent first-writer
DOWN cases (nil error after commit), and the inherited RR bypass (nil error).
The empty DOWN/UP control already passed. No focused test skipped.

The handler captures the room operation and membership identity/generation/fence
before User/Auth IO, then compares them inside the journal mutation. It refuses
a changed generation before deciding, publishing or performing cleanup. Matching
generation retries still use the existing model decisions.

The actual 000012 DOWN now refuses non-READ-COMMITTED executors and takes ACCESS
EXCLUSIVE locks on every removed/altered table before its fresh retention probe.
The existing explicit-retirement refusal and DDL remain unchanged. This does not
authorize online DOWN/retirement; lock contention can still abort a migration.

Focused GREEN: 6 test events (five leaf scenarios), zero skips. Full ordinarycall
package passed against native PG, zero skips. All 8 ordinary gRPC tests passed,
zero skips. gRPC short passed 208 test events, 4 expected integration skips.
go vet passed both affected packages. The deterministic Leave regression also
passed ten consecutive runs under the Linux race detector, zero skips.

The broader non-short gRPC package stopped at pre-existing Game Testcontainers
setup (rootless Docker not found). Its helper has no native-DSN override and this
runner deliberately has no Docker socket. This is a SETUP limitation, not an
implementation failure or full-service GREEN. No fixture/authority scope added.

Artifacts live under ignored tmp/be106-review; a portable checksum manifest and
archive will accompany the TT handoff. The final race/cleanup/head receipt goes
in TT. Independent review and assembled checks remain with real integrator121;
no GH/CI/DEV, merge, deployment or acceptance is claimed by this author.
