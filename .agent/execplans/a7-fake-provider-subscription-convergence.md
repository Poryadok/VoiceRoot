# ExecPlan: A7 fake-provider subscription convergence

## Purpose

Prove the provider-independent Subscription lifecycle is safe for the A7
benefit-matrix E2E consumer: a deterministic fake provider drives the same
revisioned entitlement state machine, and duplicates, stale facts, conflicts,
and future-effective facts cannot incorrectly change paid benefits.

## Context

- Docs: `docs/PLAN.md` (Accepted A7 subscription lifecycle contract),
  `docs/features/subscription.md` (provider-independent lifecycle),
  `docs/testing/subscription-lifecycle-convergence-exec-plan.md` (S2),
  `docs/TESTING.md`, and `docs/CONTRIBUTING.md`.
- Code: `src/backend/subscription/internal/providerlifecycle/` owns the
  source-disabled deterministic provider command reducer and PostgreSQL replay
  journal; `internal/entitlementoutbox/` owns stored snapshot publication.
- Consumer: the A7 benefit-matrix E2E may rely only on canonical revisioned
  `subscription.entitlement_changed` outcomes.
- Constraints: no live provider listener, signature/payment activation, or
  soft-delete/attachment work. Adapter event version is a provider fact, not
  transport order. Database time gates future-effective facts.

## Scope

- In: a focused direct fake-provider convergence test/implementation adjustment
  within `internal/providerlifecycle`, and any narrowly necessary contract doc
  clarification.
- Out: provider adapters, webhook listener/signatures, checkout/payment,
  consumer authority cutover, migrations unless tests identify a documented
  storage contract gap.
- Documentation gap: none currently identified; S2 is explicitly specified.

## Milestones

- [ ] Review existing S2 test coverage and record a concrete missing observable.
- [x] RED/GREEN assessment: existing S2 implementation already satisfies the
      newly added regression. A real RED could not be reproduced without
      breaking documented behavior; full database execution is externally
      blocked by rootless Docker on Windows.
- [x] GREEN: no production change was warranted; the test locks existing
      source-disabled behavior for the benefit-matrix consumer.
- [ ] Run focused, service-wide short tests, vet, lint, and required contract
      checks; submit and merge through a green PR.

## Detailed Steps

1. Compare `providerlifecycle` reducer/store behavior to S2 acceptance:
   saved duplicate outcome, stale-version non-regression, conflict quarantine,
   and future-effective no-early-benefit behavior.
2. Add a direct test first, using deterministic `fake` commands and existing
   fixture helpers. Do not enable any live adapter path.
3. Run the narrow test to establish RED. If all documented assertions already
   pass, revise the plan to a missing test-only convergence proof rather than
   changing production behavior.
4. Make the smallest package-local implementation change needed for GREEN;
   rerun narrow test, then refactor only under green tests.
5. Reconcile the exact fact shape with the entitlement event contract. Update a
   doc only if code would otherwise leave the accepted contract misleading.
6. Commit/push, open PR, await exact-head CI, resolve failures, merge with a
   merge commit, refresh master, and return this Treehouse lease.

## Validation

- [ ] `go test ./internal/providerlifecycle -count=1`
- [ ] `go test -short ./... -count=1` from `src/backend/subscription`
- [ ] `go vet ./...` from `src/backend/subscription`
- [ ] `golangci-lint run ./...` from `src/backend/subscription`
- [ ] Hosted PR CI is green on the submitted head SHA.

## Progress

- [x] Clean Treehouse lease and branch from `c46a5bc8`.
- [x] Documentation read and state-machine owner located.
- [x] Direct test added locally: personal and Space fake-provider grace,
      recovery, stale failure and exact replay converge to documented outcomes.
- [x] `go test -short ./internal/providerlifecycle -count=1` (7 packages).
- [x] `go test -short ./... -count=1` (89 packages) and `go vet ./...`.
- [ ] Full providerlifecycle integration test needs hosted CI: local run panics
      before test setup because rootless Docker is unsupported on Windows.

## Decisions

- Use the existing `providerlifecycle` S2 seam: it is explicitly source-disabled
  and uses the authoritative outbox path, matching the named E2E consumer.
- Keep this PR strictly fake-provider: the accepted docs reserve live adapter
  activation for later work.

## Risks And Follow-Ups

- The existing S2 tests may already cover every specified transition. In that
  case, add only a genuinely missing direct convergence assertion; do not create
  duplicate production code or widen the lifecycle.
- PostgreSQL evidence requires Docker/testcontainers. A skipped local database
  check is not evidence and remains a hosted-CI gate. The local full run failed
  before test setup with `rootless Docker is not supported on Windows`.
